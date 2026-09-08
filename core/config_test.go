package core

import (
	"path/filepath"
	"testing"
)

func TestParseMultiLibraryAndUnknownKeys(t *testing.T) {
	text := `
default = "sanji"

[[libraries]]
name = "sanji"
root = "kb/sanji"

[libraries.config]
preview = "html"
future_option = "kept"

[[libraries]]
name = "robin"
root = "/abs/robin"
`
	cfg, err := ParseWorkspaceConfig(text)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ResolvePaths("/ws")

	if cfg.Default != "sanji" {
		t.Errorf("default = %q", cfg.Default)
	}
	if len(cfg.Libraries) != 2 {
		t.Fatalf("libraries = %d", len(cfg.Libraries))
	}
	if cfg.Libraries[0].Root != filepath.Clean("/ws/kb/sanji") {
		t.Errorf("root0 = %q", cfg.Libraries[0].Root)
	}
	if cfg.Libraries[0].Config.Preview != "html" {
		t.Errorf("preview = %q", cfg.Libraries[0].Config.Preview)
	}
	if v, ok := cfg.Libraries[0].Config.Extra["future_option"]; !ok || v != "kept" {
		t.Errorf("extra = %v", cfg.Libraries[0].Config.Extra)
	}
	if cfg.Libraries[1].Root != filepath.Clean("/abs/robin") {
		t.Errorf("root1 = %q", cfg.Libraries[1].Root)
	}
}

func TestMarshalWorkspaceConfigRoundTrip(t *testing.T) {
	src := `
default = "sanji"

[[libraries]]
name = "sanji"
root = "kb/sanji"

[libraries.config]
preview = "html"
future_option = "kept"

[[libraries]]
name = "robin"
root = "/abs/robin"

[libraries.config]
allow_exec = false
`
	cfg, err := ParseWorkspaceConfig(src)
	if err != nil {
		t.Fatal(err)
	}

	out, err := MarshalWorkspaceConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	back, err := ParseWorkspaceConfig(out)
	if err != nil {
		t.Fatalf("reparse failed: %v\n%s", err, out)
	}
	if back.Default != cfg.Default {
		t.Errorf("default = %q, want %q", back.Default, cfg.Default)
	}
	if len(back.Libraries) != len(cfg.Libraries) {
		t.Fatalf("libraries = %d, want %d\n%s", len(back.Libraries), len(cfg.Libraries), out)
	}
	for i := range cfg.Libraries {
		if back.Libraries[i].Name != cfg.Libraries[i].Name {
			t.Errorf("lib %d name = %q, want %q", i, back.Libraries[i].Name, cfg.Libraries[i].Name)
		}
		if back.Libraries[i].Root != cfg.Libraries[i].Root {
			t.Errorf("lib %d root = %q, want %q", i, back.Libraries[i].Root, cfg.Libraries[i].Root)
		}
		if back.Libraries[i].Config.Preview != cfg.Libraries[i].Config.Preview {
			t.Errorf("lib %d preview = %q, want %q", i, back.Libraries[i].Config.Preview, cfg.Libraries[i].Config.Preview)
		}
		if v, ok := cfg.Libraries[i].Config.Extra["future_option"]; ok {
			got, ok := back.Libraries[i].Config.Extra["future_option"]
			if !ok || got != v {
				t.Errorf("lib %d extra future_option = %v, want %v", i, got, v)
			}
		}
	}
	// Explicit false must survive as false, not be dropped.
	if back.Libraries[1].Config.AllowExec == nil || *back.Libraries[1].Config.AllowExec != false {
		t.Errorf("allow_exec roundtrip = %v, want false", valOf(back.Libraries[1].Config.AllowExec))
	}
}

func TestMarshalWorkspaceConfigEmptyConfigOmitsTable(t *testing.T) {
	cfg := WorkspaceConfig{
		Default:   "demo",
		Libraries: []LibrarySpec{{Name: "demo", Root: "."}},
	}
	out, err := MarshalWorkspaceConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Fatal("empty output")
	}
	back, err := ParseWorkspaceConfig(out)
	if err != nil {
		t.Fatalf("reparse failed: %v\n%s", err, out)
	}
	if back.Default != "demo" || len(back.Libraries) != 1 || back.Libraries[0].Root != "." {
		t.Errorf("roundtrip mismatch:\n%s", out)
	}
}

func valOf(b *bool) any {
	if b == nil {
		return nil
	}
	return *b
}
