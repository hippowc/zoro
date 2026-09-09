package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zoro/core"
)

// App 的桥方法里有两步是常驻进程独有的（重载工作区 + 给新库建索引），
// core 的测试覆盖不到，而漏掉的症状是用户视角最贵的那种：「加了库但搜不到」。
// 这里不启动 Wails：ctx 为 nil 时 emitProgress 静默、PickDirectory 明确报错，
// 其余方法都能在纯 Go 里真跑。

func writeLibFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newTestApp 造一个已经加载好工作区的 App，工作区里有一个库 notes（1 个块）。
func newTestApp(t *testing.T) (*App, string) {
	t.Helper()
	base := t.TempDir()
	notes := filepath.Join(base, "notes")
	writeLibFile(t, notes, "invest.md", "# 投资\n\n@index 定投 止盈\n长期定投策略\n")

	wsPath := filepath.Join(base, "zoro.toml")
	decl := "# 我的工作区声明（注释必须活下来）\n\ndefault = \"notes\"\n\n[[libraries]]\nname = \"notes\"\nroot = \"notes\"\n"
	if err := os.WriteFile(wsPath, []byte(decl), 0o644); err != nil {
		t.Fatal(err)
	}

	a := NewApp()
	a.wsPath = wsPath
	ws, err := core.LoadWorkspace(wsPath)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	a.ws = ws
	if err := ws.AnalyzeAll(true); err != nil {
		t.Fatalf("AnalyzeAll: %v", err)
	}
	return a, base
}

func TestListLibrariesReportsBlocksAndDefault(t *testing.T) {
	a, base := newTestApp(t)

	list, err := a.ListLibraries()
	if err != nil {
		t.Fatalf("ListLibraries: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("list = %+v", list)
	}
	got := list[0]
	if got.Name != "notes" {
		t.Errorf("Name = %q", got.Name)
	}
	if want := filepath.Join(base, "notes"); got.Root != want {
		t.Errorf("Root = %q, want %q", got.Root, want)
	}
	if got.Blocks != 1 {
		t.Errorf("Blocks = %d, want 1", got.Blocks)
	}
	if !got.IsDefault {
		t.Errorf("IsDefault = false，声明里 default = \"notes\"")
	}
}

func TestAddLibraryIndexesNewLibraryImmediately(t *testing.T) {
	a, base := newTestApp(t)
	more := filepath.Join(base, "more")
	writeLibFile(t, more, "deploy.md", "@shell deploy\n```bash\necho hi\n```\n")

	rep, err := a.AddLibrary("more", more)
	if err != nil {
		t.Fatalf("AddLibrary: %v", err)
	}
	if rep.Name != "more" || rep.Root != more {
		t.Errorf("report = %+v", rep)
	}
	if rep.Blocks != 1 {
		t.Errorf("Blocks = %d, want 1（新库必须当场建好索引）", rep.Blocks)
	}
	if rep.CreatedDir {
		t.Errorf("CreatedDir = true，目录本来就存在")
	}

	// ⭐ 这条断言是整个测试文件的重点：a.ws 是启动时的快照，
	//    AddLibrary 不重载工作区的话，这里就是 0 命中。
	hits := a.Query("deploy")
	if len(hits) != 1 || hits[0].Library != "more" {
		t.Fatalf("新库加完立刻搜不到：hits = %+v", hits)
	}

	list, err := a.ListLibraries()
	if err != nil {
		t.Fatalf("ListLibraries: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("list = %+v", list)
	}

	data, err := os.ReadFile(a.wsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "# 我的工作区声明（注释必须活下来）") {
		t.Errorf("注释被 TOML 序列化吃掉了:\n%s", data)
	}
	if !strings.Contains(string(data), "name = \"more\"") {
		t.Errorf("新库没写进声明:\n%s", data)
	}
	if !strings.Contains(string(data), "default = \"notes\"") {
		t.Errorf("已有的 default 被改动了:\n%s", data)
	}
}

func TestAddLibraryCreatesMissingDirectory(t *testing.T) {
	a, base := newTestApp(t)
	fresh := filepath.Join(base, "fresh")

	rep, err := a.AddLibrary("fresh", fresh)
	if err != nil {
		t.Fatalf("AddLibrary: %v", err)
	}
	if !rep.CreatedDir {
		t.Errorf("CreatedDir = false")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("目录没建出来: %v", err)
	}
	if rep.Blocks != 0 {
		t.Errorf("空目录 Blocks = %d, want 0", rep.Blocks)
	}
}

func TestAddLibraryDuplicateNameIsRejectedAndLeavesWorkspaceUsable(t *testing.T) {
	a, base := newTestApp(t)
	before, err := os.ReadFile(a.wsPath)
	if err != nil {
		t.Fatal(err)
	}

	other := filepath.Join(base, "other")
	if _, err := a.AddLibrary("notes", other); err == nil {
		t.Fatal("重名应当被拒绝")
	}

	after, err := os.ReadFile(a.wsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("被拒绝的添加改动了声明文件:\n%s", after)
	}
	if _, err := os.Stat(other); !os.IsNotExist(err) {
		t.Errorf("被拒绝的添加不应该建目录（err = %v）", err)
	}
	// 失败之后进程还得能用：a.ws 不能已经被换成了一个半成品。
	if got := a.Status(); !strings.Contains(got, "1 个知识库") {
		t.Errorf("Status = %q", got)
	}
	if hits := a.Query("定投"); len(hits) != 1 {
		t.Errorf("失败后搜索坏了：hits = %+v", hits)
	}
}

func TestReindexPicksUpFilesAddedAfterLoad(t *testing.T) {
	a, base := newTestApp(t)
	writeLibFile(t, filepath.Join(base, "notes"), "late.md", "@index 晚到的块\n正文\n")

	msg, err := a.Reindex()
	if err != nil {
		t.Fatalf("Reindex: %v", err)
	}
	if !strings.Contains(msg, "共 2 个块") {
		t.Errorf("msg = %q", msg)
	}
	list, err := a.ListLibraries()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Blocks != 2 {
		t.Errorf("list = %+v", list)
	}
}

func TestStatusReportsRefreshFailure(t *testing.T) {
	a, _ := newTestApp(t)
	if got := a.Status(); !strings.Contains(got, "1 个知识库") || strings.Contains(got, "⚠️") {
		t.Errorf("Status = %q", got)
	}
}

func TestRevealLibraryRejectsUnknownName(t *testing.T) {
	a, _ := newTestApp(t)
	// ⚠️ 只测失败路径：成功路径会 exec xdg-open / open，测试里不能真开窗口。
	if _, err := a.RevealLibrary("nope"); err == nil {
		t.Fatal("未知库名应当报错")
	}
}

func TestBridgeMethodsFailReadablyWithoutWorkspace(t *testing.T) {
	a := NewApp()
	a.wsPath = filepath.Join(t.TempDir(), "zoro.toml")

	if _, err := a.ListLibraries(); err == nil {
		t.Error("ListLibraries 应当报错")
	}
	if _, err := a.Reindex(); err == nil {
		t.Error("Reindex 应当报错")
	}
	if _, err := a.RevealLibrary("x"); err == nil {
		t.Error("RevealLibrary 应当报错")
	}
	if _, err := a.LoadRaw("x", "y.md", 1); err == nil {
		t.Error("LoadRaw 应当报错")
	}
	if _, err := a.PickDirectory(); err == nil {
		t.Error("PickDirectory 在 ctx 为 nil 时应当报错")
	}
	if got := a.Query("x"); got != nil {
		t.Errorf("Query = %+v, want nil", got)
	}
	if got := a.Status(); !strings.Contains(got, "未加载工作区") {
		t.Errorf("Status = %q", got)
	}
}
