package core

import (
	"encoding/json"
	"strconv"
	"time"
)

// Schema is the current manifest schema version.
const Schema = 2

// ShellCap is the persisted payload metadata of an `@shell` block
// (command bodies are never stored; they are re-read from source on demand).
type ShellCap struct {
	Lang  string `json:"lang,omitempty"`
	Lines []int  `json:"lines,omitempty"`
}

// LibraryMeta records the analyzed input root (for debugging; not used for location).
type LibraryMeta struct {
	Name string `json:"name"`
	Root string `json:"root"`
}

// ManifestBlock is one persisted block (raw body is dropped).
type ManifestBlock struct {
	Title string `json:"title"`
	// Kind is the stable semantic identifier (`index` / `shell` / `unknown:<name>` …).
	Kind  string    `json:"kind"`
	Index string    `json:"index"`
	Path  string    `json:"path"`
	Start int       `json:"start"`
	Shell *ShellCap `json:"shell,omitempty"`
}

// FileFingerprint is a per-file identity used for incremental rebuilds.
type FileFingerprint struct {
	Size  int64 `json:"size"`
	MTime int64 `json:"mtime"` // unix nanoseconds
}

// Manifest is the analyzed result of a single library.
type Manifest struct {
	Schema      int                        `json:"schema"`
	Library     LibraryMeta                `json:"library"`
	GeneratedAt string                     `json:"generated_at"`
	Blocks      []ManifestBlock            `json:"blocks"`
	Files       map[string]FileFingerprint `json:"files,omitempty"`
}

// NewManifest compiles runtime blocks into a manifest (drops Raw).
func NewManifest(name, root string, blocks []Block) Manifest {
	m := Manifest{
		Schema:      Schema,
		Library:     LibraryMeta{Name: name, Root: root},
		GeneratedAt: strconv.FormatInt(time.Now().Unix(), 10),
		Blocks:      make([]ManifestBlock, 0, len(blocks)),
	}
	for _, b := range blocks {
		mb := ManifestBlock{
			Title: b.Title,
			Kind:  b.Kind.String(),
			Index: b.IndexText(),
			Path:  b.Path,
			Start: b.Start,
		}
		if b.Shell != nil {
			mb.Shell = &ShellCap{Lang: b.Shell.Lang, Lines: b.Shell.Lines}
		}
		m.Blocks = append(m.Blocks, mb)
	}
	return m
}

// ToJSON serializes the manifest with indentation.
func (m Manifest) ToJSON() (string, error) {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ManifestFromJSON parses a manifest.
func ManifestFromJSON(s string) (Manifest, error) {
	var m Manifest
	err := json.Unmarshal([]byte(s), &m)
	return m, err
}

// IntoBlocks restores runtime Blocks from the manifest (Raw empty; read on demand).
func (m Manifest) IntoBlocks(library string) []Block {
	blocks := make([]Block, 0, len(m.Blocks))
	for _, mb := range m.Blocks {
		b := Block{
			Library: library,
			Title:   mb.Title,
			Kind:    TagKindFromString(mb.Kind),
			Terms:   fieldsPreserveEmpty(mb.Index),
			Path:    mb.Path,
			Start:   mb.Start,
		}
		if mb.Shell != nil {
			b.Shell = &ShellBlock{Lang: mb.Shell.Lang, Lines: mb.Shell.Lines}
		}
		blocks = append(blocks, b)
	}
	return blocks
}

func fieldsPreserveEmpty(s string) []string {
	var out []string
	var cur []rune
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if len(cur) > 0 {
				out = append(out, string(cur))
				cur = nil
			}
			continue
		}
		cur = append(cur, r)
	}
	if len(cur) > 0 {
		out = append(out, string(cur))
	}
	return out
}
