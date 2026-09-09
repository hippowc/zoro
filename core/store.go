package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	StoreSchema        = 1
	bucketMeta         = "meta"
	bucketBlocks       = "blocks"
	bucketFingerprints = "fingerprints"
	keySchemaVersion   = "schema"
	keyLibraryMeta     = "library"

	// storeOpenTimeout 限定抢文件锁的等待时间。bbolt 的默认值 0 是「每 50ms
	// 重试、永不放弃、永不报错」，于是「另一个进程占着这个库」会表现成
	// 命令行永久卡死（见 kb/facts/pitfalls.md P-13）。
	storeOpenTimeout = 300 * time.Millisecond
)

// ErrStoreLocked 表示这个库的索引正被另一个 zoro 进程独占。
// bbolt 取的是 flock 独占锁，读写模式之间也互斥，所以常驻型前端必须
// 「用完即关」（见 Library.withStore），否则 CLI 完全无法使用同一个库。
var ErrStoreLocked = errors.New("知识库索引正被另一个 zoro 进程占用（请先退出它）")

// Store wraps a bbolt database for one library.
type Store struct {
	db   *bolt.DB
	path string
}

// OpenStore opens (or creates) the bbolt store at path.
func OpenStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: storeOpenTimeout})
	if err != nil {
		if errors.Is(err, bolt.ErrTimeout) {
			return nil, fmt.Errorf("%w: %s", ErrStoreLocked, path)
		}
		return nil, fmt.Errorf("open store %s: %w", path, err)
	}
	s := &Store{db: db, path: path}
	if err := s.initBuckets(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) initBuckets() error {
	return s.db.Update(func(tx *bolt.Tx) error {
		for _, name := range []string{bucketMeta, bucketBlocks, bucketFingerprints} {
			if _, err := tx.CreateBucketIfNotExists([]byte(name)); err != nil {
				return err
			}
		}
		meta := tx.Bucket([]byte(bucketMeta))
		if meta.Get([]byte(keySchemaVersion)) == nil {
			return meta.Put([]byte(keySchemaVersion), []byte(fmt.Sprintf("%d", StoreSchema)))
		}
		return nil
	})
}

func (s *Store) Close() error { return s.db.Close() }
func (s *Store) Path() string { return s.path }

// blockKey produces a sort-friendly key from (path, start).
func blockKey(path string, start int) []byte {
	return []byte(fmt.Sprintf("%s\x00%010d", path, start))
}

// WriteBlocks replaces all blocks in the store.
func (s *Store) WriteBlocks(blocks []Block) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket([]byte(bucketBlocks))
		// Clear existing blocks by deleting the bucket and recreating it.
		if err := tx.DeleteBucket([]byte(bucketBlocks)); err != nil {
			return err
		}
		bkt, err := tx.CreateBucket([]byte(bucketBlocks))
		if err != nil {
			return err
		}
		for _, b := range blocks {
			mb := blockToPersist(b)
			data, err := json.Marshal(mb)
			if err != nil {
				return err
			}
			if err := bkt.Put(blockKey(b.Path, b.Start), data); err != nil {
				return err
			}
		}
		return nil
	})
}

// ReadBlocks loads all blocks from the store.
func (s *Store) ReadBlocks(library string) ([]Block, error) {
	var blocks []Block
	err := s.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket([]byte(bucketBlocks))
		return bkt.ForEach(func(k, v []byte) error {
			var mb ManifestBlock
			if err := json.Unmarshal(v, &mb); err != nil {
				return err
			}
			blocks = append(blocks, manifestBlockToBlock(mb, library))
			return nil
		})
	})
	return blocks, err
}

func blockToPersist(b Block) ManifestBlock {
	mb := ManifestBlock{
		Title: b.Title,
		Kind:  b.Kind.String(),
		Index: b.IndexText(),
		Path:  b.Path,
		Start: b.Start,
	}
	if b.Shell != nil {
		mb.Shell = &ShellCap{Lang: b.Shell.Lang, Lines: b.Shell.Lines}
	}
	return mb
}

func manifestBlockToBlock(mb ManifestBlock, library string) Block {
	b := Block{
		Library: library,
		Title:   mb.Title,
		Kind:    TagKindFromString(mb.Kind),
		Terms:   fieldsPreserveEmpty(mb.Index),
		Path:    mb.Path,
		Start:   mb.Start,
	}
	if mb.Shell != nil {
		b.Shell = &ShellBlock{Lang: mb.Shell.Lang, Lines: mb.Shell.Lines}
	}
	return b
}

// WriteFingerprints replaces all fingerprints in the store.
func (s *Store) WriteFingerprints(fps map[string]FileFingerprint) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket([]byte(bucketFingerprints)); err != nil {
			return err
		}
		bkt, err := tx.CreateBucket([]byte(bucketFingerprints))
		if err != nil {
			return err
		}
		for path, fp := range fps {
			data, _ := json.Marshal(fp)
			if err := bkt.Put([]byte(path), data); err != nil {
				return err
			}
		}
		return nil
	})
}

// ReadFingerprints loads all fingerprints from the store.
func (s *Store) ReadFingerprints() (map[string]FileFingerprint, error) {
	fps := map[string]FileFingerprint{}
	err := s.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket([]byte(bucketFingerprints))
		return bkt.ForEach(func(k, v []byte) error {
			var fp FileFingerprint
			if err := json.Unmarshal(v, &fp); err != nil {
				return err
			}
			fps[string(k)] = fp
			return nil
		})
	})
	return fps, err
}

// WriteLibraryMeta writes library metadata.
func (s *Store) WriteLibraryMeta(name, root string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket([]byte(bucketMeta))
		data, _ := json.Marshal(LibraryMeta{Name: name, Root: root})
		return bkt.Put([]byte(keyLibraryMeta), data)
	})
}

// ReadLibraryMeta reads library metadata.
func (s *Store) ReadLibraryMeta() (LibraryMeta, error) {
	var meta LibraryMeta
	err := s.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket([]byte(bucketMeta))
		v := bkt.Get([]byte(keyLibraryMeta))
		if v == nil {
			return fmt.Errorf("no library meta")
		}
		return json.Unmarshal(v, &meta)
	})
	return meta, err
}

// MigrateFromMetaJSON reads meta.json and writes its contents into the store.
// Returns true if migration happened.
func (s *Store) MigrateFromMetaJSON(metaPath, library string) (bool, error) {
	data, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return false, err
	}
	if m.Schema != Schema {
		return false, nil // let caller rebuild
	}
	if m.Library.Name != library {
		return false, nil
	}

	blocks := make([]Block, 0, len(m.Blocks))
	for _, mb := range m.Blocks {
		blocks = append(blocks, manifestBlockToBlock(mb, library))
	}
	if err := s.WriteBlocks(blocks); err != nil {
		return false, err
	}
	if err := s.WriteFingerprints(m.Files); err != nil {
		return false, err
	}
	if err := s.WriteLibraryMeta(m.Library.Name, m.Library.Root); err != nil {
		return false, err
	}
	// Remove old meta.json after successful migration.
	os.Remove(metaPath)
	return true, nil
}
