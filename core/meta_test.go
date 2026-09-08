package core

import "testing"

func TestManifestRoundtripBlocksAndShellCap(t *testing.T) {
	blocks := []Block{{
		Library: "",
		Title:   "Hello",
		Kind:    KindShell,
		Terms:   []string{"alpha", "beta"},
		Path:    "a/b.md",
		Start:   3,
		Raw:     "ignored",
		Shell:   &ShellBlock{Lang: "bash", Lines: []int{5}},
	}}
	m := NewManifest("t", ".", blocks)
	if m.Schema != Schema {
		t.Errorf("schema = %d", m.Schema)
	}
	if m.Blocks[0].Kind != "shell" || m.Blocks[0].Index != "alpha beta" {
		t.Errorf("block wrong: %+v", m.Blocks[0])
	}
	if len(m.Blocks[0].Shell.Lines) != 1 || m.Blocks[0].Shell.Lines[0] != 5 {
		t.Errorf("shell wrong: %+v", m.Blocks[0].Shell)
	}

	jsonText, err := m.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	m2, err := ManifestFromJSON(jsonText)
	if err != nil {
		t.Fatal(err)
	}
	got := m2.IntoBlocks("t")
	if len(got) != 1 {
		t.Fatalf("blocks = %d", len(got))
	}
	if got[0].Title != "Hello" || got[0].Kind != KindShell {
		t.Errorf("block wrong: %+v", got[0])
	}
	if !equalStrings(got[0].Terms, []string{"alpha", "beta"}) {
		t.Errorf("terms = %v", got[0].Terms)
	}
	if got[0].Path != "a/b.md" || got[0].Start != 3 {
		t.Errorf("path/start wrong: %+v", got[0])
	}
	if got[0].Raw != "" {
		t.Errorf("raw should be empty after manifest roundtrip: %q", got[0].Raw)
	}
	if got[0].Shell == nil || got[0].Shell.Lang != "bash" || !equalInts(got[0].Shell.Lines, []int{5}) {
		t.Errorf("shell wrong: %+v", got[0].Shell)
	}
}

func TestManifestUnknownKindRoundtrips(t *testing.T) {
	blocks := []Block{{
		Kind:  KindUnknown("foo"),
		Terms: []string{"bar"},
	}}
	m := NewManifest("t", ".", blocks)
	if m.Blocks[0].Kind != "unknown:foo" {
		t.Errorf("kind = %q", m.Blocks[0].Kind)
	}
	m2, err := ManifestFromJSON(mustJSON(t, m))
	if err != nil {
		t.Fatal(err)
	}
	if got := m2.IntoBlocks("t")[0].Kind; got != KindUnknown("foo") {
		t.Errorf("kind = %v", got)
	}
}

func mustJSON(t *testing.T, m Manifest) string {
	t.Helper()
	s, err := m.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	return s
}
