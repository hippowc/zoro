package core

import "testing"

func blockWithTerms(terms ...string) Block {
	return Block{Title: "t", Terms: terms}
}

func queryIndexes(t *testing.T, blocks []Block, q string) []string {
	t.Helper()
	hits := SearchWith(DefaultMatcher, blocks, "lib", q)
	out := make([]string, 0, len(hits))
	for _, c := range hits {
		out = append(out, c.Index)
	}
	return out
}

func TestQueryTermOrderInsensitive(t *testing.T) {
	bs := []Block{blockWithTerms("python", "安装")}
	got := queryIndexes(t, bs, "安装 python")
	if !equalStrings(got, []string{"python 安装"}) {
		t.Errorf("hits = %v", got)
	}
}

func TestQueryTermFuzzySubsequence(t *testing.T) {
	bs := []Block{blockWithTerms("python", "安装")}
	if got := queryIndexes(t, bs, "py"); !equalStrings(got, []string{"python 安装"}) {
		t.Errorf("hits = %v", got)
	}
}

func TestQueryReversedCharsWithinTermDoNotMatch(t *testing.T) {
	bs := []Block{blockWithTerms("python", "安装")}
	if got := queryIndexes(t, bs, "npy"); len(got) != 0 {
		t.Errorf("hits = %v, want none", got)
	}
}

func TestQueryCaseInsensitive(t *testing.T) {
	bs := []Block{blockWithTerms("git", "pull")}
	if got := queryIndexes(t, bs, "GIT"); !equalStrings(got, []string{"git pull"}) {
		t.Errorf("hits = %v", got)
	}
}

func TestQueryCandidateCarriesLibraryAndTitle(t *testing.T) {
	bs := []Block{{Title: "Git pull", Terms: []string{"git", "pull"}}}
	found := Search(bs, "my-lib", "gp")
	if len(found) != 1 {
		t.Fatalf("hits = %d", len(found))
	}
	if found[0].Library != "my-lib" || found[0].Title != "Git pull" {
		t.Errorf("candidate = %+v", found[0])
	}
}

func TestQueryEmptyBrowsesAll(t *testing.T) {
	bs := []Block{
		{Title: "a", Terms: []string{"x"}},
		{Title: "b", Terms: []string{"y"}},
	}
	found := Search(bs, "lib", "")
	if len(found) != 2 {
		t.Fatalf("hits = %d", len(found))
	}
}

func TestQueryCandidateCarriesMatches(t *testing.T) {
	bs := []Block{blockWithTerms("python", "安装")}
	found := Search(bs, "lib", "py")
	if len(found) != 1 {
		t.Fatalf("hits = %d", len(found))
	}
	if len(found[0].Matches) != 1 {
		t.Fatalf("matches = %+v", found[0].Matches)
	}
	r := found[0].Matches[0]
	idx := found[0].Index
	if r.Start != 0 || r.End != len("py") || idx[r.Start:r.End] != "py" {
		t.Errorf("match range = %+v over %q", r, idx)
	}
}

func TestQueryCandidateMatchesCaseInsensitive(t *testing.T) {
	bs := []Block{blockWithTerms("git", "pull")}
	found := Search(bs, "lib", "GIT")
	if len(found) != 1 || len(found[0].Matches) != 1 {
		t.Fatalf("matches = %+v", found[0].Matches)
	}
	if got := found[0].Index[found[0].Matches[0].Start:found[0].Matches[0].End]; got != "git" {
		t.Errorf("matched span = %q, want %q", got, "git")
	}
}

func TestQueryCandidateMatchesMultiTermCJK(t *testing.T) {
	bs := []Block{blockWithTerms("定投", "止盈", "资产配置")}
	found := Search(bs, "lib", "定 盈")
	if len(found) != 1 {
		t.Fatalf("hits = %d", len(found))
	}
	if len(found[0].Matches) == 0 {
		t.Fatalf("matches empty")
	}
	var highlighted []string
	for _, r := range found[0].Matches {
		highlighted = append(highlighted, found[0].Index[r.Start:r.End])
	}
	if !equalStrings(highlighted, []string{"定", "盈"}) {
		t.Errorf("highlighted = %v", highlighted)
	}
}

func TestQueryEmptyHasNoMatches(t *testing.T) {
	bs := []Block{blockWithTerms("anything")}
	found := Search(bs, "lib", "")
	if len(found) != 1 || len(found[0].Matches) != 0 {
		t.Errorf("candidate = %+v", found[0])
	}
}
