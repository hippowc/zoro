package core

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
)

// RenderRaw returns a block's raw payload (P0 degraded path).
func RenderRaw(b *Block) string { return b.Raw }

// mdHTML is the shared goldmark renderer with GFM extensions and code highlighting.
var mdHTML = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
		highlighting.NewHighlighting(highlighting.WithStyle("github")),
	),
)

// RenderMarkdownHTML renders Markdown to HTML (P1 target).
//
// - top-level (or indented) `@name` directive lines are stripped first;
// - fenced code blocks are highlighted by chroma via goldmark-highlighting.
func RenderMarkdownHTML(md string) (string, error) {
	var buf bytes.Buffer
	if err := mdHTML.Convert([]byte(StripDirectives(md)), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// StripDirectives removes `@name` directive lines (fence-internal lines are code, kept).
func StripDirectives(md string) string {
	var out strings.Builder
	inFence := false
	for _, line := range splitLines(md) {
		t := trimLeftSpace(line)
		if inFence {
			out.WriteString(line)
			out.WriteByte('\n')
			if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
				inFence = false
			}
			continue
		}
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			inFence = true
			out.WriteString(line)
			out.WriteByte('\n')
			continue
		}
		if isDirectiveLine(t) {
			continue
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	return out.String()
}

func isDirectiveLine(t string) bool {
	if !strings.HasPrefix(t, "@") {
		return false
	}
	rest := t[1:]
	i := 0
	for i < len(rest) {
		c := rest[i]
		if (c >= 'a' && c <= 'z') || c == '-' {
			i++
			continue
		}
		break
	}
	if i == 0 {
		return false
	}
	after := rest[i:]
	return after == "" || after[0] == ' ' || after[0] == '\t'
}

// RenderTarget selects an output surface for the render pipeline (§8):
// HTML (first-class), ANSI (terminal color) and Text (no ANSI codes).
type RenderTarget string

const (
	RenderTargetHTML RenderTarget = "html"
	RenderTargetANSI RenderTarget = "ansi"
	RenderTargetText RenderTarget = "text"
)

// RenderMarkdown dispatches Markdown to the requested render target.
func RenderMarkdown(md string, target RenderTarget) (string, error) {
	switch target {
	case RenderTargetHTML, "":
		return RenderMarkdownHTML(md)
	case RenderTargetANSI:
		return RenderMarkdownANSI(md)
	case RenderTargetText:
		return RenderMarkdownText(md)
	default:
		return "", fmt.Errorf("unknown render target: %q", target)
	}
}
