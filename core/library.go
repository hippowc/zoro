package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Library is one knowledge base: name + content root + config + content blocks.
// The library component of a block's identity is Name.
type Library struct {
	Name   string
	Root   string
	Config LibraryConfig
	// DataDir, when non-empty, redirects derived files (manifest + readable
	// TSV) into DataDir/<Name>/ instead of the content root.
	DataDir string
	Blocks  []Block
	store   *Store
}

// OpenLibrary fully scans a content root (online mode; block.Raw is populated).
func OpenLibrary(name, root string) (*Library, error) {
	return OpenLibraryWithConfig(name, root, LibraryConfig{})
}

// OpenLibraryCached prefers the manifest when it exists and is fresh;
// otherwise it scans and writes the manifest.
func OpenLibraryCached(name, root string) (*Library, error) {
	return OpenLibraryCachedWithConfig(name, root, LibraryConfig{})
}

// OpenLibraryWithConfig fully scans with a per-library config.
func OpenLibraryWithConfig(name, root string, config LibraryConfig) (*Library, error) {
	lib := &Library{Name: name, Root: root, Config: config}
	if err := lib.Rescan(); err != nil {
		return nil, err
	}
	return lib, nil
}

// OpenLibraryCachedWithConfig is the cold-start-first path with config.
func OpenLibraryCachedWithConfig(name, root string, config LibraryConfig) (*Library, error) {
	return OpenLibraryCachedWithConfigAndData(name, root, config, "")
}

// OpenLibraryCachedWithConfigAndData is the cached open path for workspaces
// that redirect derived files into a per-workspace data directory.
func OpenLibraryCachedWithConfigAndData(name, root string, config LibraryConfig, dataDir string) (*Library, error) {
	lib := &Library{Name: name, Root: root, Config: config, DataDir: dataDir}

	// Open store and try migration from meta.json.
	store, err := OpenStore(lib.DBPath())
	if err != nil {
		return nil, err
	}
	lib.store = store

	// Check if store has data.
	blocks, _ := store.ReadBlocks(lib.Name)
	if len(blocks) > 0 {
		lib.Blocks = blocks
		return lib, nil
	}

	// Try migrating from meta.json.
	oldMetaPath := lib.legacyMetaPath()
	migrated, err := store.MigrateFromMetaJSON(oldMetaPath, lib.Name)
	if err != nil {
		return nil, err
	}
	if migrated {
		blocks, _ = store.ReadBlocks(lib.Name)
		lib.Blocks = blocks
		return lib, nil
	}

	// No store data, no migration possible: full rebuild.
	if _, err := lib.Analyze(true); err != nil {
		return nil, err
	}
	return lib, nil
}

// MetaPath returns the library manifest path (now the bbolt store).
func (l *Library) MetaPath() string {
	return l.DBPath()
}

// DBPath returns the bbolt store path for this library.
func (l *Library) DBPath() string {
	if l.DataDir != "" {
		return filepath.Join(l.DataDir, l.Name, "zoro.db")
	}
	return filepath.Join(l.Root, ".zoro", "zoro.db")
}

// legacyMetaPath returns the old meta.json path (before DBPath rename).
func (l *Library) legacyMetaPath() string {
	if l.DataDir != "" {
		return filepath.Join(l.DataDir, l.Name, "meta.json")
	}
	return filepath.Join(l.Root, ".zoro", "meta.json")
}

// IndexViewPath returns the readable TSV view path.
func (l *Library) IndexViewPath() string {
	if l.DataDir != "" {
		return filepath.Join(l.DataDir, l.Name, "zoro-index.tsv")
	}
	return filepath.Join(l.Root, "zoro-index.tsv")
}

// IsStale reports whether some markdown file is newer than the manifest
// (or the manifest is missing). Kept as a coarse check for callers.
func (l *Library) IsStale() (bool, error) {
	p := l.MetaPath()
	info, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, err
	}
	return hasNewerMD(l.Root, info.ModTime())
}

// Analyze ensures the store is fresh, then returns its path.
//
// Freshness is file-level: `(path, mtime, size)` fingerprints are compared so
// only added / changed / removed markdown files trigger rescans.
func (l *Library) Analyze(force bool) (string, error) {
	// Ensure store is open.
	if l.store == nil {
		s, err := OpenStore(l.DBPath())
		if err != nil {
			return "", err
		}
		l.store = s
	}

	files, err := listMDFiles(l.Root)
	if err != nil {
		return "", err
	}
	now := fingerprintFiles(files)

	if force {
		return l.DBPath(), l.rebuildFull(now)
	}

	// Try loading fingerprints from store.
	prevFPs, err := l.store.ReadFingerprints()
	if err != nil || len(prevFPs) == 0 {
		return l.DBPath(), l.rebuildFull(now)
	}

	dirty, removed := diffFingerprints(prevFPs, now)
	if len(dirty) == 0 && len(removed) == 0 {
		// Load blocks from store.
		blocks, err := l.store.ReadBlocks(l.Name)
		if err != nil {
			return "", err
		}
		l.Blocks = blocks
		return l.DBPath(), nil
	}

	// Load existing blocks for incremental rebuild.
	prevBlocks, _ := l.store.ReadBlocks(l.Name)
	return l.DBPath(), l.rebuildIncrementalStore(prevBlocks, files, now, dirty, removed)
}

// rebuildFull rescans every file and rewrites store.
func (l *Library) rebuildFull(files map[string]FileFingerprint) error {
	if err := l.Rescan(); err != nil {
		return err
	}
	return l.writeAll(files)
}

// rebuildIncrementalStore keeps unchanged blocks from the previous store and
// rescans only dirty files; removed files drop their blocks.
func (l *Library) rebuildIncrementalStore(prevBlocks []Block, files []mdFile, now map[string]FileFingerprint, dirty, removed map[string]bool) error {
	byRel := make(map[string]mdFile, len(files))
	for _, f := range files {
		byRel[f.Rel] = f
	}

	out := make([]Block, 0, len(prevBlocks))
	for _, b := range prevBlocks {
		if dirty[b.Path] || removed[b.Path] {
			continue
		}
		out = append(out, b)
	}

	var dirtyRels []string
	for rel := range dirty {
		dirtyRels = append(dirtyRels, rel)
	}
	sort.Strings(dirtyRels)
	for _, rel := range dirtyRels {
		file, ok := byRel[rel]
		if !ok {
			continue // deleted between listing and rescan
		}
		bs, err := ScanFile(file.Abs)
		if err != nil {
			return err
		}
		for i := range bs {
			bs[i].Path = filepath.ToSlash(rel)
			bs[i].Library = l.Name
			out = append(out, bs[i])
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Start < out[j].Start
	})

	l.Blocks = out
	return l.writeAll(now)
}

// writeAll writes the store (blocks + fingerprints + meta).
func (l *Library) writeAll(files map[string]FileFingerprint) error {
	if err := l.store.WriteBlocks(l.Blocks); err != nil {
		return err
	}
	if err := l.store.WriteFingerprints(files); err != nil {
		return err
	}
	return l.store.WriteLibraryMeta(l.Name, l.Root)
}

// Rescan reloads all blocks from the content root.
func (l *Library) Rescan() error {
	blocks, err := ScanDirNamed(l.Root, l.Name)
	if err != nil {
		return err
	}
	l.Blocks = blocks
	return nil
}

func (l *Library) writeIndexView() error {
	return atomicWrite(l.IndexViewPath(), []byte(ToTSV(l.Blocks)))
}

// readManifest loads and validates a manifest, but does not mutate Blocks.
func (l *Library) readManifest() (Manifest, error) {
	data, err := os.ReadFile(l.MetaPath())
	if err != nil {
		return Manifest{}, err
	}
	m, err := ManifestFromJSON(string(data))
	if err != nil {
		return Manifest{}, err
	}
	if m.Schema != Schema {
		return Manifest{}, fmt.Errorf("unsupported manifest schema %d (expected %d)", m.Schema, Schema)
	}
	if m.Library.Name != l.Name {
		return Manifest{}, fmt.Errorf("manifest library name %q does not match workspace declaration %q", m.Library.Name, l.Name)
	}
	return m, nil
}

// loadManifest reads the manifest into Blocks.
func (l *Library) loadManifest() error {
	m, err := l.readManifest()
	if err != nil {
		return err
	}
	l.Blocks = m.IntoBlocks(l.Name)
	return nil
}

// PreviewTarget maps the `preview` config preference onto a render target.
// Returns "" when unset so callers can apply terminal-aware defaults.
func (l *Library) PreviewTarget() RenderTarget {
	switch strings.ToLower(strings.TrimSpace(l.Config.Preview)) {
	case "html":
		return RenderTargetHTML
	case "text", "plain", "markdown", "md":
		return RenderTargetText
	default:
		return ""
	}
}

// AllowExec reports the configured `allow_exec` preference (default: deny).
// It is consumed by the future Action registry for `@shell` execution (P3).
func (l *Library) AllowExec() bool {
	return l.Config.AllowExec != nil && *l.Config.AllowExec
}

// LoadRaw cuts a block's raw text on demand by (Path, Start).
func (l *Library) LoadRaw(block *Block) (string, error) {
	data, err := os.ReadFile(filepath.Join(l.Root, block.Path))
	if err != nil {
		return "", err
	}
	return SliceBlock(string(data), block.Start), nil
}

// Query searches this library's blocks, auto-refreshing if files changed.
func (l *Library) Query(q string) []Candidate {
	// Auto-refresh: check fingerprints and rebuild incrementally if needed.
	if err := l.ensureFresh(); err != nil {
		// Non-fatal: continue with stale blocks rather than failing the query.
	}

	faces := l.ActiveFaces()
	if len(faces) == 0 {
		faces = []Face{IndexFace{W: DefaultWeightIndex}}
	}
	return SearchMultiFace(faces, l.Blocks, l.Name, q)
}

// ensureFresh checks file fingerprints and triggers incremental rebuild if needed.
func (l *Library) ensureFresh() error {
	// Ensure store is open.
	if l.store == nil {
		s, err := OpenStore(l.DBPath())
		if err != nil {
			return err
		}
		l.store = s
	}

	files, err := listMDFiles(l.Root)
	if err != nil {
		return err
	}
	now := fingerprintFiles(files)

	prevFPs, err := l.store.ReadFingerprints()
	if err != nil || len(prevFPs) == 0 {
		// No fingerprints yet: if we already have blocks in memory (from Rescan),
		// just write them to the store. Otherwise do a full rebuild.
		if len(l.Blocks) > 0 {
			return l.writeAll(now)
		}
		return l.rebuildFull(now)
	}

	dirty, removed := diffFingerprints(prevFPs, now)
	if len(dirty) == 0 && len(removed) == 0 {
		return nil // Fresh, no changes.
	}

	// Incremental rebuild.
	prevBlocks, _ := l.store.ReadBlocks(l.Name)
	return l.rebuildIncrementalStore(prevBlocks, files, now, dirty, removed)
}

// ActiveFaces resolves the configured face instances.
func (l *Library) ActiveFaces() []Face {
	names := l.Config.Faces
	if len(names) == 0 {
		names = DefaultFaces()
	}
	return BuildFaces(names, l.Config.FaceWeights)
}

// Close releases the underlying store resources.
func (l *Library) Close() error {
	if l.store != nil {
		return l.store.Close()
	}
	return nil
}

// fingerprintFiles builds the `(path, mtime, size)` fingerprints for a file list.
func fingerprintFiles(files []mdFile) map[string]FileFingerprint {
	out := make(map[string]FileFingerprint, len(files))
	for _, f := range files {
		out[f.Rel] = FileFingerprint{Size: f.Size, MTime: f.MTime.UnixNano()}
	}
	return out
}

// diffFingerprints returns changed/added paths (dirty) and removed paths.
func diffFingerprints(prev, now map[string]FileFingerprint) (dirty, removed map[string]bool) {
	dirty = map[string]bool{}
	removed = map[string]bool{}
	for p, f := range now {
		old, ok := prev[p]
		if !ok || old != f {
			dirty[p] = true
		}
	}
	for p := range prev {
		if _, ok := now[p]; !ok {
			removed[p] = true
		}
	}
	return dirty, removed
}

// hasNewerMD reports whether the root has a markdown file newer than `since`.
func hasNewerMD(root string, since time.Time) (bool, error) {
	files, err := listMDFiles(root)
	if err != nil {
		return false, err
	}
	for _, f := range files {
		if f.MTime.After(since) {
			return true, nil
		}
	}
	return false, nil
}

// atomicWrite writes via a temp file and renames it over the destination.
func atomicWrite(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
