package core

import "testing"

func TestLibraryPreviewTarget(t *testing.T) {
	cases := []struct {
		preview string
		want    RenderTarget
	}{
		{"", ""},
		{"html", RenderTargetHTML},
		{"HTML", RenderTargetHTML},
		{"text", RenderTargetText},
		{"plain", RenderTargetText},
		{"markdown", RenderTargetText},
		{"md", RenderTargetText},
		{"unknown", ""},
	}
	for _, c := range cases {
		lib := &Library{Config: LibraryConfig{Preview: c.preview}}
		if got := lib.PreviewTarget(); got != c.want {
			t.Errorf("PreviewTarget(%q) = %q, want %q", c.preview, got, c.want)
		}
	}
}

func TestLibraryAllowExec(t *testing.T) {
	falseVal := false
	trueVal := true
	cases := []struct {
		name     string
		allow    *bool
		expected bool
	}{
		{"unset", nil, false},
		{"false", &falseVal, false},
		{"true", &trueVal, true},
	}
	for _, c := range cases {
		lib := &Library{Config: LibraryConfig{AllowExec: c.allow}}
		if got := lib.AllowExec(); got != c.expected {
			t.Errorf("AllowExec(%s) = %v, want %v", c.name, got, c.expected)
		}
	}
}
