package core_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"zoro/core"
)

func repoTestDir(t *testing.T, tag string) string {
	t.Helper()
	src := filepath.Join("..", "tests", tag)
	dst := t.TempDir()
	copyDir(t, src, dst)
	return dst
}

func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestScansFixtureDir(t *testing.T) {
	root := repoTestDir(t, "fixtures")
	lib, err := core.OpenLibrary("fixtures", root)
	if err != nil {
		t.Fatal(err)
	}

	hits := lib.Query("定投")
	titles := make([]string, 0, len(hits))
	for _, c := range hits {
		titles = append(titles, c.Title)
	}
	if len(titles) != 1 || titles[0] != "资产配置" {
		t.Fatalf("titles = %v, want [资产配置]", titles)
	}
}

func TestMultiLibraryDoesNotCrossContaminate(t *testing.T) {
	a := repoTestDir(t, "fixtures")
	b := repoTestDir(t, "fixtures2")
	ws, err := core.NewWorkspace([]core.LibrarySpec{{Name: "lib-a", Root: a}, {Name: "lib-b", Root: b}})
	if err != nil {
		t.Fatal(err)
	}

	hits := ws.Query("定投")
	if len(hits) != 1 {
		t.Fatalf("hits = %d, want 1", len(hits))
	}
	if hits[0].Library != "lib-a" || hits[0].Title != "资产配置" {
		t.Errorf("hit = %+v", hits[0])
	}
	raw, err := ws.LoadRaw(hits[0].Library, hits[0].Path, hits[0].Start)
	if err != nil {
		t.Fatal(err)
	}
	if raw == "" {
		t.Errorf("raw is empty")
	}
	if !stringContains(raw, "@index 定投 止盈 资产配置") {
		t.Errorf("raw missing tag line: %q", raw)
	}
}

func TestLoadWorkspaceWithRelativeRoots(t *testing.T) {
	base := t.TempDir()
	copyDir(t, filepath.Join("..", "tests", "fixtures"), filepath.Join(base, "lib-a"))
	copyDir(t, filepath.Join("..", "tests", "fixtures2"), filepath.Join(base, "lib-b"))
	err := os.WriteFile(filepath.Join(base, "zoro.toml"), []byte(`
default = "lib-a"

[[libraries]]
name = "lib-a"
root = "lib-a"

[[libraries]]
name = "lib-b"
root = "lib-b"
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	ws, err := core.LoadWorkspace(filepath.Join(base, "zoro.toml"))
	if err != nil {
		t.Fatal(err)
	}
	names := ws.LibraryNames()
	if len(names) != 2 || names[0] != "lib-a" || names[1] != "lib-b" {
		t.Fatalf("names = %v", names)
	}

	hits := ws.Query("定投")
	if len(hits) != 1 || hits[0].Library != "lib-a" {
		t.Fatalf("hits = %+v", hits)
	}
}

func TestAnalyzeWritesStore(t *testing.T) {
	root := repoTestDir(t, "fixtures")
	lib, err := core.OpenLibrary("fixtures", root)
	if err != nil {
		t.Fatal(err)
	}
	dbPath, err := lib.Analyze(true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatal(err)
	}
	// Verify block count.
	if len(lib.Blocks) != 4 {
		t.Errorf("blocks = %d, want 4", len(lib.Blocks))
	}
}

func stringContains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func findByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func TestAnalyzeIncrementalOnDirtyFile(t *testing.T) {
	root := repoTestDir(t, "fixtures")
	lib, err := core.OpenLibraryCached("fixtures", root)
	if err != nil {
		t.Fatal(err)
	}
	before := len(lib.Blocks)

	p := filepath.Join(root, "example.md")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, []byte("\n## 新条目\n@index 新词 内容\n新正文。\n")...)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := lib.Analyze(false); err != nil {
		t.Fatal(err)
	}
	if len(lib.Blocks) != before+1 {
		t.Fatalf("blocks = %d, want %d", len(lib.Blocks), before+1)
	}
	found := false
	for _, b := range lib.Blocks {
		if b.Title == "新条目" || containsTerm(b.Terms, "新词") {
			found = true
		}
	}
	if !found {
		t.Fatalf("dirty file's new block not found in %+v", lib.Blocks)
	}
}

func TestAnalyzeIncrementalRemovesDeletedFile(t *testing.T) {
	root := repoTestDir(t, "fixtures")
	extra := filepath.Join(root, "extra.md")
	if err := os.WriteFile(extra, []byte("@index extra 条目\n正文。\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lib, err := core.OpenLibraryCached("fixtures", root)
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Blocks) != 5 {
		t.Fatalf("blocks = %d, want 5", len(lib.Blocks))
	}
	if err := os.Remove(extra); err != nil {
		t.Fatal(err)
	}
	if _, err := lib.Analyze(false); err != nil {
		t.Fatal(err)
	}
	if len(lib.Blocks) != 4 {
		t.Fatalf("blocks = %d, want 4", len(lib.Blocks))
	}
	for _, b := range lib.Blocks {
		if b.Path == "extra.md" {
			t.Fatalf("removed file still has blocks: %+v", b)
		}
	}
}

func containsTerm(terms []string, want string) bool {
	for _, term := range terms {
		if term == want {
			return true
		}
	}
	return false
}
