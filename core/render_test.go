package core

import "testing"

func TestStripDirectivesKeepsFence(t *testing.T) {
	md := "@index a b\n@shell\n```bash\necho @not_a_directive\n```\ntext\n"
	out := StripDirectives(md)
	for _, banned := range []string{"@index", "@shell"} {
		if contains(out, banned) {
			t.Errorf("output contains %q: %q", banned, out)
		}
	}
	for _, want := range []string{"echo @not_a_directive", "```bash", "text"} {
		if !contains(out, want) {
			t.Errorf("output missing %q: %q", want, out)
		}
	}
}

func TestRenderMarkdownHTMLRendersHeading(t *testing.T) {
	html, err := RenderMarkdownHTML("# Title\n\nhello\n")
	if err != nil {
		t.Fatal(err)
	}
	if !contains(html, "<h1") {
		t.Errorf("missing h1: %s", html)
	}
	if !contains(html, "hello") {
		t.Errorf("missing body: %s", html)
	}
}

func TestRenderMarkdownANSIColorsHeadingAndCode(t *testing.T) {
	out, err := RenderMarkdownANSI("## 标题\n\n**加粗** and `code`\n\n```bash\necho hi\n```\n")
	if err != nil {
		t.Fatal(err)
	}
	if !contains(out, "标题") {
		t.Errorf("missing heading text: %q", out)
	}
	if !contains(out, "\x1b[") {
		t.Errorf("missing ANSI codes: %q", out)
	}
	if !contains(out, "echo hi") {
		t.Errorf("missing code body: %q", out)
	}
}

func TestRenderMarkdownTextHasNoANSI(t *testing.T) {
	out, err := RenderMarkdownText("## 标题\n\n**加粗**\n")
	if err != nil {
		t.Fatal(err)
	}
	if contains(out, "\x1b[") {
		t.Errorf("plain target contains ANSI: %q", out)
	}
	if !contains(out, "标题") || !contains(out, "加粗") {
		t.Errorf("missing text: %q", out)
	}
}

func TestRenderMarkdownDispatch(t *testing.T) {
	for _, tt := range []struct {
		target   RenderTarget
		wantANSI bool
	}{
		{RenderTargetHTML, false},
		{RenderTargetANSI, true},
		{RenderTargetText, false},
	} {
		out, err := RenderMarkdown("## T\n\nbody", tt.target)
		if err != nil {
			t.Fatalf("%s: %v", tt.target, err)
		}
		if contains(out, "\x1b[") != tt.wantANSI {
			t.Errorf("%s: ansi=%v, out=%q", tt.target, tt.wantANSI, out)
		}
	}
}
