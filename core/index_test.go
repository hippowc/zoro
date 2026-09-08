package core

import "testing"

func TestToTSVRendersHeaderAndRows(t *testing.T) {
	e := Block{
		Title: "Hello",
		Terms: []string{"alpha", "beta"},
		Path:  "sub/a.md",
		Start: 3,
	}
	tsv := ToTSV([]Block{e})
	lines := splitLines(tsv)
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2", len(lines))
	}
	if lines[0] != "title\tindex\tstart\tpath" {
		t.Errorf("header = %q", lines[0])
	}
	if lines[1] != "Hello\talpha beta\t3\tsub/a.md" {
		t.Errorf("row = %q", lines[1])
	}
}
