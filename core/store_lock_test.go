package core_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"zoro/core"
)

// P-13 回归：bbolt 的默认 Timeout=0 是「无限重试、永不报错」，
// 于是「索引被占用」表现成进程永久卡死。这几个测试守住三件事：
// ① 抢不到锁要**快速失败**并给出可判定的 ErrStoreLocked；
// ② 一个已加载且磁盘无变化的库，查询/刷新**完全不需要锁**；
// ③ 因此同一进程/多进程可以对同一个库并存（Launcher + CLI）。

func TestOpenStoreFailsFastWhenLocked(t *testing.T) {
	root := repoTestDir(t, "fixtures")
	lib, err := core.OpenLibraryCached("fixtures", root)
	if err != nil {
		t.Fatal(err)
	}

	held, err := core.OpenStore(lib.DBPath())
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()

	start := time.Now()
	_, err = core.OpenStore(lib.DBPath())
	elapsed := time.Since(start)

	if !errors.Is(err, core.ErrStoreLocked) {
		t.Fatalf("OpenStore error = %v, want ErrStoreLocked", err)
	}
	// 超时是 300ms；给足调度余量，但必须远小于「永久挂起」。
	if elapsed > 5*time.Second {
		t.Fatalf("OpenStore took %v, want a fast failure", elapsed)
	}
}

func TestFreshLibraryNeedsNoLock(t *testing.T) {
	root := repoTestDir(t, "fixtures")
	lib, err := core.OpenLibraryCached("fixtures", root)
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Query("定投")) == 0 {
		t.Fatal("baseline query returned no hits")
	}

	held, err := core.OpenStore(lib.DBPath())
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()

	// 磁盘没变 → 内存指纹命中快路径 → 刷新与查询都不碰 store，
	// 所以即使锁被别人拿着也必须成功且不阻塞。
	done := make(chan error, 1)
	go func() {
		if err := lib.Refresh(); err != nil {
			done <- err
			return
		}
		if len(lib.Query("定投")) == 0 {
			done <- errors.New("query returned no hits while the store was locked")
			return
		}
		done <- nil
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Refresh/Query blocked on a locked store; the in-memory fingerprint fast path did not kick in")
	}
}

func TestRefreshReportsLockWhenContentChanged(t *testing.T) {
	root := repoTestDir(t, "fixtures")
	lib, err := core.OpenLibraryCached("fixtures", root)
	if err != nil {
		t.Fatal(err)
	}
	before := len(lib.Blocks)

	held, err := core.OpenStore(lib.DBPath())
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()

	// 改动内容 → 快路径失效 → 必须重建 → 必须拿锁 → 拿不到就要报错而不是挂死。
	note := filepath.Join(root, "locked-note.md")
	if err := os.WriteFile(note, []byte("# 新片段\n\n@index 锁测试\n\n正文\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 保证 mtime 与旧指纹不同（同 size 同 mtime 是不可见的，见 architecture.md）。
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(note, future, future); err != nil {
		t.Fatal(err)
	}

	if err := lib.Refresh(); !errors.Is(err, core.ErrStoreLocked) {
		t.Fatalf("Refresh error = %v, want ErrStoreLocked", err)
	}
	// 降级行为：Query 不失败，继续用内存里的旧块。
	if len(lib.Query("定投")) == 0 {
		t.Fatal("Query should fall back to the in-memory blocks")
	}
	if len(lib.Blocks) != before {
		t.Fatalf("Blocks changed (%d -> %d) although the rebuild could not run", before, len(lib.Blocks))
	}

	// 锁释放后同一次刷新就能成功，且新片段可见。
	held.Close()
	if err := lib.Refresh(); err != nil {
		t.Fatalf("Refresh after unlocking: %v", err)
	}
	if len(lib.Query("锁测试")) == 0 {
		t.Fatal("the new block is missing after a successful refresh")
	}
}

func TestTwoWorkspacesShareOneLibrary(t *testing.T) {
	root := repoTestDir(t, "fixtures")
	wsPath := filepath.Join(t.TempDir(), "zoro.toml")
	cfg := core.WorkspaceConfig{
		Default:   "fixtures",
		Libraries: []core.LibrarySpec{{Name: "fixtures", Root: root}},
	}
	if err := core.WriteWorkspaceConfig(wsPath, cfg); err != nil {
		t.Fatal(err)
	}

	// 模拟「常驻 Launcher」与「一次 CLI 命令」同时打开同一个库。
	// 修复前第二次 LoadWorkspace 会在 flock 上永久挂起（测试直接超时）。
	resident, err := core.LoadWorkspace(wsPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(resident.Query("定投")) == 0 {
		t.Fatal("resident workspace returned no hits")
	}

	cli, err := core.LoadWorkspace(wsPath)
	if err != nil {
		t.Fatalf("second workspace over the same library: %v", err)
	}
	if len(cli.Query("定投")) == 0 {
		t.Fatal("second workspace returned no hits")
	}

	// 两边交替强制重建（都要拿写锁）也必须都能成功。
	if err := resident.AnalyzeAll(true); err != nil {
		t.Fatalf("resident AnalyzeAll: %v", err)
	}
	if err := cli.AnalyzeAll(true); err != nil {
		t.Fatalf("cli AnalyzeAll: %v", err)
	}
}
