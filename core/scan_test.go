package core

import "testing"

func TestScanTextSplitsTwoIndexBlocks(t *testing.T) {
	text := "## Head\n@index alpha beta\nbody one\n@index gamma\nbody two\n"
	blocks := ScanText(text, "a.md")
	if len(blocks) != 2 {
		t.Fatalf("blocks = %d, want 2", len(blocks))
	}
	if blocks[0].Title != "Head" {
		t.Errorf("title = %q, want Head", blocks[0].Title)
	}
	if want := []string{"alpha", "beta"}; !equalStrings(blocks[0].Terms, want) {
		t.Errorf("terms = %v, want %v", blocks[0].Terms, want)
	}
	if blocks[0].Start != 2 || blocks[1].Start != 4 {
		t.Errorf("starts = %d,%d want 2,4", blocks[0].Start, blocks[1].Start)
	}
	if blocks[1].Title != "gamma" {
		t.Errorf("title2 = %q, want gamma", blocks[1].Title)
	}
}

func TestScanTextSharedHeadingWhenAdjacent(t *testing.T) {
	text := "## Head\n@index alpha\nbody\n@index beta\n"
	blocks := ScanText(text, "a.md")
	if len(blocks) != 2 {
		t.Fatalf("blocks = %d", len(blocks))
	}
	if blocks[0].Title != "Head" {
		t.Errorf("title0 = %q", blocks[0].Title)
	}
	if blocks[1].Title != "beta" {
		t.Errorf("title1 = %q", blocks[1].Title)
	}
}

func TestScanTextBindsShellFence(t *testing.T) {
	text := "@index run stuff\n@shell\n```bash\necho hi\necho bye\n```\ntail\n"
	blocks := ScanText(text, "a.md")
	if len(blocks) != 2 {
		t.Fatalf("blocks = %d", len(blocks))
	}
	b := blocks[1]
	if b.Kind != KindShell {
		t.Errorf("kind = %v", b.Kind)
	}
	if b.Shell == nil {
		t.Fatal("shell payload is nil")
	}
	if b.Shell.Lang != "bash" {
		t.Errorf("lang = %q", b.Shell.Lang)
	}
	if !equalInts(b.Shell.Lines, []int{4, 5}) {
		t.Errorf("lines = %v", b.Shell.Lines)
	}
	if !contains(b.Raw, "echo hi") {
		t.Errorf("raw missing body: %q", b.Raw)
	}
}

func TestScanTextAnyTagEndsPreviousBlock(t *testing.T) {
	text := "@index alpha\nindex body\n@shell git 丢弃\n```bash\ngit checkout -- .\n```\nafter\n"
	blocks := ScanText(text, "a.md")
	if len(blocks) != 2 {
		t.Fatalf("blocks = %d", len(blocks))
	}
	if blocks[0].Kind != KindIndex || !contains(blocks[0].Raw, "index body") {
		t.Errorf("block0 wrong: %+v", blocks[0])
	}
	if contains(blocks[0].Raw, "@shell") {
		t.Errorf("block0 contains next tag: %q", blocks[0].Raw)
	}
	if blocks[1].Kind != KindShell || !equalStrings(blocks[1].Terms, []string{"git", "丢弃"}) {
		t.Errorf("block1 wrong: %+v", blocks[1])
	}
	if !contains(blocks[1].Raw, "after") {
		t.Errorf("block1 raw missing after: %q", blocks[1].Raw)
	}
}

func TestScanTextUnknownTagDegradesToNote(t *testing.T) {
	text := "@index alpha\nbody\n@foo bar baz\npayload\n"
	blocks := ScanText(text, "a.md")
	if len(blocks) != 2 {
		t.Fatalf("blocks = %d", len(blocks))
	}
	if blocks[1].Kind != KindUnknown("foo") {
		t.Errorf("kind = %v", blocks[1].Kind)
	}
	if !equalStrings(blocks[1].Terms, []string{"bar", "baz"}) {
		t.Errorf("terms = %v", blocks[1].Terms)
	}
	if !contains(blocks[1].Raw, "payload") {
		t.Errorf("raw missing payload: %q", blocks[1].Raw)
	}
}

func TestScanTextBlockEndsAtEOF(t *testing.T) {
	text := "@index only\nline one\nline two"
	blocks := ScanText(text, "a.md")
	if len(blocks) != 1 {
		t.Fatalf("blocks = %d", len(blocks))
	}
	if got := len(splitLines(blocks[0].Raw)); got != 3 {
		t.Errorf("raw lines = %d, want 3", got)
	}
}

func TestSliceBlockMatchesScannerBoundary(t *testing.T) {
	text := "## Head\n@index alpha beta\nline one\n@shell git\n```bash\necho hi\n```\n"
	if got := SliceBlock(text, 2); got != "@index alpha beta\nline one" {
		t.Errorf("slice(2) = %q", got)
	}
	sliced := SliceBlock(text, 4)
	if !hasPrefix(sliced, "@shell git") || !hasSuffix(sliced, "```") {
		t.Errorf("slice(4) bad: %q", sliced)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func hasPrefix(s, p string) bool { return len(s) >= len(p) && s[:len(p)] == p }
func hasSuffix(s, p string) bool { return len(s) >= len(p) && s[len(s)-len(p):] == p }
