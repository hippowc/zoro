package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureDefaultWorkspaceCreatesEverything(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	wsPath, err := EnsureDefaultWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	if wsPath != filepath.Join(home, ".zoro", "zoro.toml") {
		t.Fatalf("wsPath = %q", wsPath)
	}

	data, err := os.ReadFile(wsPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "default = 'default'") {
		t.Errorf("missing default library declaration:\n%s", text)
	}
	if !strings.Contains(text, "root = '"+filepath.Join(home, ".zoro", "kb")+"'") {
		t.Errorf("library root is not absolute:\n%s", text)
	}
	if !strings.Contains(text, "data_dir = '"+filepath.Join(home, ".zoro", "index")+"'") {
		t.Errorf("data_dir is not recorded:\n%s", text)
	}

	kbNote := filepath.Join(home, ".zoro", "kb", "默认知识库.md")
	if _, err := os.Stat(kbNote); err != nil {
		t.Fatalf("default kb note missing: %v", err)
	}
}

func TestEnsureDefaultWorkspaceIsIdempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	wsPath, err := EnsureDefaultWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	custom := "default = 'mine'\n"
	if err := os.WriteFile(wsPath, []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := EnsureDefaultWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	if got != wsPath {
		t.Fatalf("path changed: %q", got)
	}
	data, err := os.ReadFile(wsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != custom {
		t.Fatalf("existing workspace was overwritten:\n%s", data)
	}
}

func TestDefaultWorkspaceDerivedFilesRedirectToDataDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	wsPath, err := EnsureDefaultWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	ws, err := LoadWorkspace(wsPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(ws.Libraries) != 1 {
		t.Fatalf("libraries = %d", len(ws.Libraries))
	}
	lib := ws.Libraries[0]

	wantDB := filepath.Join(home, ".zoro", "index", "default", "zoro.db")
	if lib.MetaPath() != wantDB {
		t.Fatalf("MetaPath = %q, want %q", lib.MetaPath(), wantDB)
	}

	if _, err := lib.Analyze(true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(wantDB); err != nil {
		t.Fatalf("store not written to data dir: %v", err)
	}
	for _, b := range lib.Blocks {
		if filepath.IsAbs(b.Path) {
			t.Fatalf("block path should be library-relative, got %q", b.Path)
		}
	}
}
