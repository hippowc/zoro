package core

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

// ScanFile scans a single Markdown file, splitting it into content blocks.
// Returned blocks have an empty Library field, filled by ScanDirNamed / Library.Open.
func ScanFile(path string) ([]Block, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ScanText(string(data), path), nil
}

// ScanText scans text content into blocks (convenient for testing with inline strings).
func ScanText(text, path string) []Block {
	s := newScanner(path)
	s.consume(text)
	return s.blocks
}

// SliceBlock cuts a block's raw text starting at the 1-based `start` line.
// Boundary = start line -> next `@<name>` tag line / EOF (derived on the fly).
func SliceBlock(text string, start int) string {
	out := make([]string, 0, 8)
	for no, line := range splitLines(text) {
		n := no + 1
		if n < start {
			continue
		}
		if n > start {
			if _, _, ok := parseDirective(line); ok {
				break
			}
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

type scanner struct {
	path         string
	blocks       []Block
	lastHeading  string
	headingFresh bool
	cur          *curBlock
	pendingShell bool
}

type curBlock struct {
	start int
	title string
	kind  TagKind
	terms []string
	lines []string
	shell *ShellBlock
}

func newScanner(path string) *scanner {
	return &scanner{path: path}
}

func (s *scanner) consume(text string) {
	var (
		fenceKind  rune
		shellLang  string
		shellLines []int
	)

	for idx, line := range splitLines(text) {
		lineNo := idx + 1

		// 0) Inside a shell fence: `@` lines are code, never tags.
		if fenceKind != 0 {
			if isFenceClose(line, fenceKind) {
				if s.cur != nil {
					s.cur.lines = append(s.cur.lines, line)
					s.cur.shell = &ShellBlock{Lang: shellLang, Lines: shellLines}
				}
				s.pendingShell = false
				fenceKind = 0
			} else {
				shellLines = append(shellLines, lineNo)
				if s.cur != nil {
					s.cur.lines = append(s.cur.lines, line)
				}
			}
			continue
		}

		// 1) Heading: record the latest heading only; it is never a boundary.
		if rest, ok := strings.CutPrefix(line, "## "); ok {
			s.lastHeading = strings.TrimSpace(rest)
			s.headingFresh = true
			continue
		}
		if strings.TrimSpace(line) == "##" {
			s.lastHeading = ""
			s.headingFresh = true
			continue
		}

		// 2) Any `@<name>` tag line ends the previous block and starts a new one.
		if name, value, ok := parseDirective(line); ok {
			s.flushCurrent()
			kind := ClassifyTag(name)
			title := ""
			if s.headingFresh {
				title = s.lastHeading
			}
			s.pendingShell = kind.IsShell()
			s.cur = &curBlock{
				start: lineNo,
				title: title,
				kind:  kind,
				terms: strings.Fields(value),
				lines: []string{line},
			}
			// A tag line does not break heading freshness.
			continue
		}

		// 3) Immediately after `@shell`: start collecting the following fence.
		if s.pendingShell {
			if k := fenceKindOf(line); k != 0 {
				fenceKind = k
				shellLang = fenceLang(line)
				shellLines = nil
				if s.cur != nil {
					s.cur.lines = append(s.cur.lines, line)
				}
				s.headingFresh = false
				continue
			}
		}

		// 4) Everything else is body text.
		if strings.TrimSpace(line) != "" {
			s.headingFresh = false
		}
		if s.cur != nil {
			s.cur.lines = append(s.cur.lines, line)
		}
	}

	// Unclosed shell fence at EOF is still bound to the block.
	if s.pendingShell {
		if s.cur != nil && s.cur.shell == nil && len(shellLines) > 0 {
			s.cur.shell = &ShellBlock{Lang: shellLang, Lines: shellLines}
		}
	}

	s.flushCurrent()
}

func (s *scanner) flushCurrent() {
	if s.cur == nil {
		return
	}
	cur := s.cur
	s.cur = nil

	title := cur.title
	if title == "" {
		if len(cur.terms) > 0 {
			title = cur.terms[0]
		} else {
			title = cur.kind.String()
		}
	}
	s.blocks = append(s.blocks, Block{
		Library: "",
		Title:   title,
		Kind:    cur.kind,
		Terms:   cur.terms,
		Path:    s.path,
		Start:   cur.start,
		Raw:     strings.Join(cur.lines, "\n"),
		Shell:   cur.shell,
	})
}

// parseDirective parses a top-level `@name value` line; returns (name, value, ok).
func parseDirective(line string) (string, string, bool) {
	if !strings.HasPrefix(line, "@") {
		return "", "", false
	}
	rest := line[1:]
	i := 0
	for i < len(rest) {
		c := rest[i]
		if isASCIIName(c) {
			i++
			continue
		}
		break
	}
	if i == 0 {
		return "", "", false
	}
	name := rest[:i]
	after := rest[i:]
	if after == "" || after[0] == ' ' || after[0] == '\t' {
		return name, strings.TrimSpace(after), true
	}
	return "", "", false
}

func isASCIIName(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_'
}

func fenceKindOf(line string) rune {
	t := trimLeftSpace(line)
	if strings.HasPrefix(t, "```") {
		return '`'
	}
	if strings.HasPrefix(t, "~~~") {
		return '~'
	}
	return 0
}

func fenceLang(line string) string {
	t := trimLeftSpace(line)
	rest := ""
	for _, marker := range []string{"```", "~~~"} {
		if strings.HasPrefix(t, marker) {
			rest = strings.TrimPrefix(t, marker)
			break
		}
	}
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func isFenceClose(line string, kind rune) bool {
	t := trimLeftSpace(line)
	switch kind {
	case '`':
		return strings.HasPrefix(t, "```")
	case '~':
		return strings.HasPrefix(t, "~~~")
	}
	return false
}

func trimLeftSpace(s string) string {
	return strings.TrimLeftFunc(s, unicode.IsSpace)
}

// mdFile is one content file candidate for scanning / fingerprinting.
type mdFile struct {
	Rel   string // library-relative slash path
	Abs   string // absolute filesystem path
	Size  int64
	MTime time.Time
}

// ScanDir scans all `*.md` / `*.mdx` under a content root, sorted by path.
func ScanDir(root string) ([]Block, error) {
	return scanDirInner(root, "")
}

// ScanDirNamed is like ScanDir but sets each block's Library to `library`.
func ScanDirNamed(root, library string) ([]Block, error) {
	return scanDirInner(root, library)
}

func scanDirInner(root, library string) ([]Block, error) {
	files, err := listMDFiles(root)
	if err != nil {
		return nil, err
	}

	blocks := make([]Block, 0, len(files)*4)
	for _, file := range files {
		bs, err := ScanFile(file.Abs)
		if err != nil {
			return nil, err
		}
		for i := range bs {
			bs[i].Path = filepath.ToSlash(file.Rel)
			bs[i].Library = library
			blocks = append(blocks, bs[i])
		}
	}
	return blocks, nil
}

// listMDFiles walks the content root and returns all markdown files sorted by
// relative path, skipping hidden directories (`.git / .zoro / ...`).
func listMDFiles(root string) ([]mdFile, error) {
	var files []mdFile
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if ext != ".md" && ext != ".mdx" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			rel = p
		}
		files = append(files, mdFile{
			Rel:   filepath.ToSlash(rel),
			Abs:   p,
			Size:  info.Size(),
			MTime: info.ModTime(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Rel < files[j].Rel })
	return files, nil
}

// splitLines mimics Rust str::lines(): \r\n -> \n, and no trailing empty line.
func splitLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	for i := range lines {
		lines[i] = strings.TrimSuffix(lines[i], "\r")
	}
	return lines
}
