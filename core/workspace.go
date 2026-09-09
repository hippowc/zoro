package core

import (
	"fmt"
	"sort"
)

// Workspace is the default scope for search/browse across libraries.
//
// A block's global identity is (Library, Path, Start).
type Workspace struct {
	Libraries []*Library
}

// BuildWorkspace opens all declared libraries (cold-start first, auto rebuild).
func BuildWorkspace(cfg *WorkspaceConfig) (*Workspace, error) {
	if cfg == nil {
		return nil, fmt.Errorf("nil workspace config")
	}
	seen := map[string]bool{}
	for _, spec := range cfg.Libraries {
		if seen[spec.Name] {
			return nil, fmt.Errorf("duplicate library name: %s", spec.Name)
		}
		seen[spec.Name] = true
	}
	ws := &Workspace{make([]*Library, 0, len(cfg.Libraries))}
	for _, spec := range cfg.Libraries {
		lib, err := OpenLibraryCachedWithConfigAndData(spec.Name, spec.Root, spec.Config, cfg.DataDir)
		if err != nil {
			return nil, err
		}
		ws.Libraries = append(ws.Libraries, lib)
	}
	return ws, nil
}

// LoadWorkspace reads a `zoro.toml` file and builds a Workspace.
func LoadWorkspace(path string) (*Workspace, error) {
	cfg, err := WorkspaceConfigFromPath(path)
	if err != nil {
		return nil, err
	}
	return BuildWorkspace(&cfg)
}

// NewWorkspace builds a Workspace from simple name/root specs (tests / programmatic use).
func NewWorkspace(specs []LibrarySpec) (*Workspace, error) {
	return BuildWorkspace(&WorkspaceConfig{Libraries: specs})
}

// LibraryNames returns the declared library names in order.
func (w *Workspace) LibraryNames() []string {
	names := make([]string, 0, len(w.Libraries))
	for _, l := range w.Libraries {
		names = append(names, l.Name)
	}
	return names
}

// AnalyzeAll runs analyze on every library; force=true unconditionally rescans.
// It stops at the first failure.
func (w *Workspace) AnalyzeAll(force bool) error {
	for _, lib := range w.Libraries {
		if _, err := lib.Analyze(force); err != nil {
			return err
		}
	}
	return nil
}

// RefreshAll syncs every library with its content root and returns the first
// failure. Unlike Query, it does not swallow errors — frontends use it to tell
// the user why an index is unusable (e.g. core.ErrStoreLocked).
func (w *Workspace) RefreshAll() error {
	for _, lib := range w.Libraries {
		if err := lib.Refresh(); err != nil {
			return fmt.Errorf("刷新库 %s: %w", lib.Name, err)
		}
	}
	return nil
}

// Query aggregates cross-library candidates ranked by score desc.
func (w *Workspace) Query(q string) []Candidate {
	var out []Candidate
	for _, lib := range w.Libraries {
		out = append(out, lib.Query(q)...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Score > out[j].Score
	})
	return out
}

// LoadRaw locates a library by name and cuts a block's raw text by (Path, Start).
func (w *Workspace) LoadRaw(library, path string, start int) (string, error) {
	for _, lib := range w.Libraries {
		if lib.Name == library {
			return lib.LoadRaw(&Block{Library: library, Path: path, Start: start})
		}
	}
	return "", fmt.Errorf("library not found: %s", library)
}
