package core

import (
	"strings"

	"github.com/yuin/goldmark/ast"
	gast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// ANSI escape sequences used by the terminal target. Reset closes any
// previously opened style; codes are never emitted for the plain-text target.
const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiDim    = "\x1b[2m"
	ansiItalic = "\x1b[3m"
	ansiRed    = "\x1b[31m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiBlue   = "\x1b[34m"
	ansiCyan   = "\x1b[36m"
)

// terminalRenderer walks a goldmark AST and emits either ANSI-colored text
// (color=true) or plain text (color=false) for terminal consumption.
type terminalRenderer struct {
	src   []byte
	color bool
	buf   strings.Builder
}

// RenderMarkdownANSI renders Markdown to ANSI-colored terminal text.
// Bold/headings/code get color treatment; everything else is plain.
func RenderMarkdownANSI(md string) (string, error) {
	return renderTerminal(md, true)
}

// RenderMarkdownText renders Markdown to plain terminal text (no ANSI codes).
func RenderMarkdownText(md string) (string, error) {
	return renderTerminal(md, false)
}

func renderTerminal(md string, color bool) (string, error) {
	src := []byte(StripDirectives(md))
	r := &terminalRenderer{src: src, color: color}
	reader := text.NewReader(src)
	doc := mdHTML.Parser().Parse(reader)
	r.render(doc)
	return strings.TrimRight(r.buf.String(), "\n"), nil
}

func (r *terminalRenderer) render(n ast.Node) {
	if n == nil {
		return
	}
	child := n.FirstChild()
	switch n.Kind() {
	case ast.KindDocument:
		for ; child != nil; child = child.NextSibling() {
			r.render(child)
		}

	case ast.KindHeading:
		h := n.(*ast.Heading)
		color := ansiCyan
		if h.Level > 2 {
			color = ansiBlue
		}
		r.style(ansiBold, color)
		r.inlines(n)
		r.style(ansiReset)
		r.buf.WriteByte('\n')
		if h.Level <= 3 {
			r.buf.WriteByte('\n')
		}

	case ast.KindParagraph:
		r.inlines(n)
		r.buf.WriteString("\n\n")

	case ast.KindFencedCodeBlock:
		r.codeLines(n.(*ast.FencedCodeBlock).Lines())
		r.buf.WriteByte('\n')

	case ast.KindCodeBlock:
		r.codeLines(n.(*ast.CodeBlock).Lines())
		r.buf.WriteByte('\n')

	case gast.KindTable, gast.KindTableHeader:
		for ; child != nil; child = child.NextSibling() {
			r.render(child)
		}
		r.buf.WriteByte('\n')

	case gast.KindTableRow:
		r.renderTableRow(n)

	case gast.KindTableCell:
		r.renderTableCell(n)

	case ast.KindTextBlock:
		r.buf.WriteString(strings.TrimSpace(string(n.Text(r.src))))
		r.buf.WriteByte('\n')

	case ast.KindBlockquote:
		r.style(ansiDim)
		// Blockquote children start on fresh lines; emit a minimal marker.
		r.buf.WriteString("> ")
		r.childrenWithPrefix(n, "> ")
		r.style(ansiReset)
		r.buf.WriteByte('\n')

	case ast.KindList:
		for ; child != nil; child = child.NextSibling() {
			r.render(child)
		}
		r.buf.WriteByte('\n')

	case ast.KindListItem:
		r.buf.WriteString("- ")
		r.renderListChildren(n)
		r.buf.WriteByte('\n')

	case ast.KindThematicBreak:
		r.style(ansiDim)
		r.buf.WriteString("────────\n")
		r.style(ansiReset)
		r.buf.WriteByte('\n')

	default:
		for ; child != nil; child = child.NextSibling() {
			r.render(child)
		}
	}
}

// renderListChildren emits the inline content of a list item (paragraphs /
// nested blocks are laid out on following lines).
func (r *terminalRenderer) renderListChildren(n ast.Node) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if c.Kind() == ast.KindParagraph {
			r.inlines(c)
		} else if c.Kind() == ast.KindTextBlock {
			r.buf.Write(c.Text(r.src))
		} else {
			r.render(c)
		}
	}
}

func (r *terminalRenderer) childrenWithPrefix(n ast.Node, prefix string) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		r.buf.WriteString(prefix)
		if c.Kind() == ast.KindParagraph {
			r.inlines(c)
		} else {
			r.render(c)
		}
		r.buf.WriteByte('\n')
	}
}

func (r *terminalRenderer) renderTableRow(n ast.Node) {
	first := true
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if c.Kind() != gast.KindTableCell {
			continue
		}
		if !first {
			r.buf.WriteString(" │ ")
		}
		first = false
		r.renderTableCell(c)
	}
	r.buf.WriteByte('\n')
}

func (r *terminalRenderer) renderTableCell(n ast.Node) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch c.Kind() {
		case ast.KindTextBlock:
			r.buf.WriteString(strings.TrimSpace(string(c.Text(r.src))))
		case ast.KindParagraph:
			r.inlines(c)
		default:
			r.inline(c)
		}
	}
}

func (r *terminalRenderer) codeLines(lineSegments *text.Segments) {
	r.style(ansiDim)
	if lineSegments == nil {
		r.style(ansiReset)
		return
	}
	for i := 0; i < lineSegments.Len(); i++ {
		seg := lineSegments.At(i)
		r.buf.Write(seg.Value(r.src))
		if len(seg.Value(r.src)) == 0 {
			r.buf.WriteByte('\n')
		}
	}
	r.style(ansiReset)
}

// inlines renders inline children of n.
func (r *terminalRenderer) inlines(n ast.Node) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		r.inline(c)
	}
}

func (r *terminalRenderer) inline(n ast.Node) {
	switch n.Kind() {
	case ast.KindText, ast.KindString, ast.KindRawHTML:
		r.buf.Write(n.Text(r.src))

	case ast.KindCodeSpan:
		r.style(ansiGreen)
		r.buf.Write(n.Text(r.src))
		r.style(ansiReset)

	case ast.KindEmphasis:
		e := n.(*ast.Emphasis)
		style := ansiItalic
		if e.Level >= 2 {
			style = ansiBold
		}
		r.style(style)
		r.inlines(n)
		r.style(ansiReset)

	case ast.KindLink, ast.KindImage:
		// Link text / image alt text only; URLs are handled by rendered targets.
		r.inlines(n)

	default:
		r.inlines(n)
	}
}

// style writes codes for the color target, or skips them for plain text.
func (r *terminalRenderer) style(codes ...string) {
	if !r.color {
		return
	}
	for _, c := range codes {
		r.buf.WriteString(c)
	}
}
