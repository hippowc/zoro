package core

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Library is one knowledge base: name + content root + config + content blocks.
// The library component of a block's identity is Name.
type Library struct {
	Name   string
	Root   string
	Config LibraryConfig
	// DataDir, when non-empty, redirects derived files (the bbolt store) into
	// DataDir/<Name>/ instead of the content root.
	DataDir string
	Blocks  []Block

	// mu serializes refresh/rebuild: a resident frontend can call Query from
	// several goroutines, and two concurrent rebuilds would fight over the lock.
	mu sync.Mutex
	// fps mirrors the store's fingerprint bucket. When it matches what is on
	// disk, refreshing needs no store access at all — which is the only reason
	// a CLI command and a resident Launcher can share one library, since bbolt
	// takes an exclusive lock (see withStore and pitfalls P-13).
	fps map[string]FileFingerprint
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

	var (
		blocks []Block
		stored map[string]FileFingerprint
	)
	// 一次作用域内读完就关锁：冷启动是唯一需要读 store 的地方。
	if err := lib.withStore(func(s *Store) error {
		var err error
		blocks, err = s.ReadBlocks(lib.Name)
		if err != nil {
			return err
		}
		if len(blocks) == 0 {
			// Try migrating from the legacy meta.json.
			migrated, mErr := s.MigrateFromMetaJSON(lib.legacyMetaPath(), lib.Name)
			if mErr != nil {
				return mErr
			}
			if migrated {
				if blocks, err = s.ReadBlocks(lib.Name); err != nil {
					return err
				}
			}
		}
		stored, err = s.ReadFingerprints()
		return err
	}); err != nil {
		return nil, err
	}

	if len(blocks) == 0 {
		// No store data and nothing to migrate: full rebuild.
		if _, err := lib.Analyze(true); err != nil {
			return nil, err
		}
		return lib, nil
	}
	lib.Blocks = blocks
	lib.fps = stored
	return lib, nil
}

// withStore opens the store, runs fn, and always closes it before returning.
//
// ⚠️ 这是唯一的 store 访问入口，也是「绝不跨调用持有 store」这条律的执行点：
// bbolt 拿的是 flock 独占锁（读写互斥），一旦常驻持有，同库的 CLI 命令就会
// 在 OpenStore 上等到超时并失败 —— 常驻型前端（Launcher）尤其不能持有。
// 代价是每次冷刷新多一次 open/mmap（约 1ms），换来的是零跨进程争用。
func (l *Library) withStore(fn func(*Store) error) error {
	s, err := OpenStore(l.DBPath())
	if err != nil {
		return err
	}
	defer s.Close()
	return fn(s)
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

// Analyze ensures the store is fresh, then returns its path.
//
// Freshness is file-level: `(path, mtime, size)` fingerprints are compared so
// only added / changed / removed markdown files trigger rescans.
func (l *Library) Analyze(force bool) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.refresh(force); err != nil {
		return "", err
	}
	return l.DBPath(), nil
}

// Refresh syncs Blocks with the content root, returning any failure instead of
// swallowing it. Query() refreshes best-effort; frontends that want to tell the
// user「索引被占用 / 库目录不可读」should call this explicitly (e.g. on show).
func (l *Library) Refresh() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.refresh(false)
}

// refresh is the single sync path shared by Analyze / Refresh / Query.
// It must be called with l.mu held.
func (l *Library) refresh(force bool) error {
	files, err := listMDFiles(l.Root)
	if err != nil {
		return err
	}
	now := fingerprintFiles(files)

	// 快路径：内存指纹与磁盘一致 → 本次刷新**完全不打开 store**。
	// 常驻 Launcher 的绝大多数 Query 都走这里，因此不会与 CLI 抢锁。
	// ⚠️ 判据是 `fps != nil` 而不是 `len(fps) > 0`：**空库的镜像也是有效镜像**。
	// 用 len 判会把「已加载的空库」误当成「还没加载过」，从而走进下面的首次接触分支。
	if !force && l.fps != nil {
		dirty, removed := diffFingerprints(l.fps, now)
		if len(dirty) == 0 && len(removed) == 0 {
			return nil
		}
	}

	return l.withStore(func(s *Store) error {
		if force {
			if err := l.Rescan(); err != nil {
				return err
			}
			return l.writeAll(s, now)
		}

		prev := l.fps
		if prev == nil {
			// ReadFingerprints 对未初始化的 store 返回**空 map**，而空 map 与「磁盘上
			// 确实一个 md 都没有」无法区分，所以只在非空时采纳。
			stored, readErr := s.ReadFingerprints()
			if readErr != nil {
				return readErr
			}
			if len(stored) > 0 {
				prev = stored
			}
		}
		if prev == nil {
			// 内存和 store 都没有指纹 = 真正的第一次接触：全量扫描。
			// ⚠️ 这里不要加「Blocks 非 nil 就不重复扫」的捷径：Blocks 可能是**空库**
			// 的扫描结果（非 nil 但为空），而磁盘上刚多了文件。那样会把 now 采纳成
			// 镜像却从不索引新文件 —— 新增文件永久搜不到，直到它再次被修改。
			// （这个 bug 由 write_test.go 的 capture 用例抓到。）
			if err := l.Rescan(); err != nil {
				return err
			}
			return l.writeAll(s, now)
		}

		dirty, removed := diffFingerprints(prev, now)
		if len(dirty) == 0 && len(removed) == 0 {
			// store 是新鲜的，只是本进程的内存副本还没加载。
			blocks, err := s.ReadBlocks(l.Name)
			if err != nil {
				return err
			}
			l.Blocks = blocks
			l.fps = now
			return nil
		}

		prevBlocks, err := s.ReadBlocks(l.Name)
		if err != nil {
			return err
		}
		return l.rebuildIncremental(s, prevBlocks, files, now, dirty, removed)
	})
}

// rebuildIncremental keeps unchanged blocks from the previous store and
// rescans only dirty files; removed files drop their blocks.
func (l *Library) rebuildIncremental(s *Store, prevBlocks []Block, files []mdFile, now map[string]FileFingerprint, dirty, removed map[string]bool) error {
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
	return l.writeAll(s, now)
}

// writeAll writes the store (blocks + fingerprints + meta) and adopts `files`
// as the in-memory fingerprint mirror, so the next refresh can take the
// no-store fast path.
func (l *Library) writeAll(s *Store, files map[string]FileFingerprint) error {
	if err := s.WriteBlocks(l.Blocks); err != nil {
		return err
	}
	if err := s.WriteFingerprints(files); err != nil {
		return err
	}
	l.fps = files
	return s.WriteLibraryMeta(l.Name, l.Root)
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
	// 尽力刷新：store 被别的进程占用（ErrStoreLocked）或库目录不可读时，
	// 不该让搜索失败 —— 内存里的 Blocks 仍然可搜。需要把原因告诉用户的
	// 前端应显式调用 Refresh()（见 Launcher 的 Status）。
	_ = l.Refresh()

	faces := l.ActiveFaces()
	if len(faces) == 0 {
		faces = []Face{IndexFace{W: DefaultWeightIndex}}
	}
	return SearchMultiFace(faces, l.Blocks, l.Name, q)
}

// ActiveFaces resolves the configured face instances.
func (l *Library) ActiveFaces() []Face {
	names := l.Config.Faces
	if len(names) == 0 {
		names = DefaultFaces()
	}
	return BuildFaces(names, l.Config.FaceWeights)
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
