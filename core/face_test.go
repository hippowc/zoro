package core

import (
	"testing"
)

func blockWithTermsAndTitle(title string, terms ...string) Block {
	return Block{Title: title, Terms: terms, Path: "test.md", Start: 1}
}

func TestIndexFaceMatchesTerms(t *testing.T) {
	face := IndexFace{W: DefaultWeightIndex}
	b := blockWithTermsAndTitle("Git pull", "git", "pull")
	text := face.Text(&b)
	if text != "git pull" {
		t.Errorf("text = %q, want %q", text, "git pull")
	}
	m, ok := face.Matcher().Match("gp", text)
	if !ok {
		t.Fatal("no match")
	}
	if m.Score <= 0 {
		t.Errorf("score = %f, want > 0", m.Score)
	}
}

func TestTitleFaceMatchesTitle(t *testing.T) {
	face := TitleFace{W: DefaultWeightTitle}
	b := blockWithTermsAndTitle("定投策略", "投资")
	text := face.Text(&b)
	if text != "定投策略" {
		t.Errorf("text = %q, want %q", text, "定投策略")
	}
	m, ok := face.Matcher().Match("定投", text)
	if !ok {
		t.Fatal("no match")
	}
	if m.Score <= 0 {
		t.Errorf("score = %f, want > 0", m.Score)
	}
}

func TestPathFaceMatchesPath(t *testing.T) {
	face := PathFace{W: DefaultWeightPath}
	b := Block{Title: "t", Terms: []string{"x"}, Path: "git/常用操作.md", Start: 1}
	text := face.Text(&b)
	if text != "git/常用操作.md" {
		t.Errorf("text = %q, want %q", text, "git/常用操作.md")
	}
	m, ok := face.Matcher().Match("git", text)
	if !ok {
		t.Fatal("no match")
	}
	if m.Score <= 0 {
		t.Errorf("score = %f, want > 0", m.Score)
	}
}

func TestRawFaceDoesNotMatch(t *testing.T) {
	face := RawFace{W: DefaultWeightRaw}
	b := blockWithTermsAndTitle("t", "x")
	_, ok := face.Matcher().Match("anything", face.Text(&b))
	if ok {
		t.Error("raw face should not match")
	}
}

func TestSearchMultiFaceAccumulatesScores(t *testing.T) {
	bs := []Block{
		blockWithTermsAndTitle("定投策略", "定投", "止盈"),
	}
	faces := []Face{
		IndexFace{W: DefaultWeightIndex},
		TitleFace{W: DefaultWeightTitle},
	}
	hits := SearchMultiFace(faces, bs, "lib", "定投")
	if len(hits) != 1 {
		t.Fatalf("hits = %d, want 1", len(hits))
	}
	if len(hits[0].MatchedFaces) != 2 {
		t.Errorf("matched_faces = %v, want [index, title]", hits[0].MatchedFaces)
	}
	// Score should be accumulated from both faces.
	if hits[0].Score <= 0 {
		t.Errorf("score = %f, want > 0", hits[0].Score)
	}
}

func TestSearchMultiFaceDeduplicates(t *testing.T) {
	bs := []Block{
		blockWithTermsAndTitle("定投策略", "定投", "止盈"),
	}
	faces := []Face{
		IndexFace{W: DefaultWeightIndex},
		TitleFace{W: DefaultWeightTitle},
		PathFace{W: DefaultWeightPath},
	}
	hits := SearchMultiFace(faces, bs, "lib", "定投")
	if len(hits) != 1 {
		t.Errorf("hits = %d, want 1 (deduplicated)", len(hits))
	}
}

func TestSearchMultiFaceEmptyQuery(t *testing.T) {
	bs := []Block{
		{Title: "a", Terms: []string{"x"}, Path: "a.md", Start: 1},
		{Title: "b", Terms: []string{"y"}, Path: "b.md", Start: 1},
	}
	faces := []Face{IndexFace{W: DefaultWeightIndex}}
	hits := SearchMultiFace(faces, bs, "lib", "")
	if len(hits) != 2 {
		t.Errorf("hits = %d, want 2", len(hits))
	}
	for _, h := range hits {
		if h.Score != 0 {
			t.Errorf("score = %f, want 0 for empty query", h.Score)
		}
	}
}

func TestSearchMultiFaceWeightsAffectRanking(t *testing.T) {
	bs := []Block{
		// Block A: matches on index only
		blockWithTermsAndTitle("Other", "定投"),
		// Block B: matches on title only (with higher weight than path)
		Block{Title: "定投笔记", Terms: []string{"其他"}, Path: "notes.md", Start: 1},
	}
	faces := []Face{
		IndexFace{W: 1.0}, // low weight for index
		TitleFace{W: 10.0}, // high weight for title
	}
	hits := SearchMultiFace(faces, bs, "lib", "定投")
	if len(hits) != 2 {
		t.Fatalf("hits = %d, want 2", len(hits))
	}
	// Block B (title match with high weight) should rank higher than Block A (index match with low weight).
	if hits[0].Title != "定投笔记" {
		t.Errorf("top hit = %q, want %q", hits[0].Title, "定投笔记")
	}
}

func TestBuildFacesDefaults(t *testing.T) {
	faces := BuildFaces(DefaultFaces(), nil)
	if len(faces) != 3 {
		t.Fatalf("faces = %d, want 3", len(faces))
	}
	names := make([]string, len(faces))
	for i, f := range faces {
		names[i] = f.Name()
	}
	expected := []string{"index", "title", "path"}
	if !equalStrings(names, expected) {
		t.Errorf("names = %v, want %v", names, expected)
	}
}

func TestBuildFacesCustomWeights(t *testing.T) {
	weights := map[string]float64{"index": 20.0, "title": 15.0}
	faces := BuildFaces([]string{"index", "title"}, weights)
	if len(faces) != 2 {
		t.Fatalf("faces = %d, want 2", len(faces))
	}
	if faces[0].Weight() != 20.0 {
		t.Errorf("index weight = %f, want 20.0", faces[0].Weight())
	}
	if faces[1].Weight() != 15.0 {
		t.Errorf("title weight = %f, want 15.0", faces[1].Weight())
	}
}

func TestBuildFacesUnknownNameSkipped(t *testing.T) {
	faces := BuildFaces([]string{"index", "unknown_face", "title"}, nil)
	if len(faces) != 2 {
		t.Errorf("faces = %d, want 2 (unknown skipped)", len(faces))
	}
}
