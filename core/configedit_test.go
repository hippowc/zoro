package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 这些测试守的是一条律：程序化修改 zoro.toml 时，**除了预期的那几行，其它字节一个都不动**。
// 注释和用户手写的未知键 ParseWorkspaceConfig 根本看不见，所以必须在**文本层面**断言 ——
// 只断言解析结果会让「注释被吃掉」这类回归静默通过。

// commentedTOML 故意包含所有「整文件重写会丢掉」的东西：顶部注释块、行尾注释、
// 顶层未知键、库级 config 子表、相对路径 root、段间注释。
const commentedTOML = `# zoro 工作区声明
# 这两行注释必须在任何程序化修改之后原样存在

default = "notes"  # 默认库
future_key = "还没实现的键"

[[libraries]]
name = "notes"
root = "/abs/notes"

# 第二个库
[[libraries]]
name = "work"
root = "relative/work"
[libraries.config]
preview = "html"
`

func TestAppendLibrarySpecKeepsOriginalBytesAsPrefix(t *testing.T) {
	out, err := AppendLibrarySpec(commentedTOML, "ideas", "/abs/ideas/")
	if err != nil {
		t.Fatalf("AppendLibrarySpec: %v", err)
	}
	if !strings.HasPrefix(out, commentedTOML) {
		t.Fatalf("原文本没有被原样保留为前缀:\n%s", out)
	}
	// root 被 Clean 掉尾斜杠；段前留一个空行分隔。
	want := "\n[[libraries]]\nname = \"ideas\"\nroot = \"/abs/ideas\"\n"
	if got := out[len(commentedTOML):]; got != want {
		t.Errorf("追加的字节 = %q\n期望 = %q", got, want)
	}

	cfg, err := ParseWorkspaceConfig(out)
	if err != nil {
		t.Fatalf("追加后解析失败: %v", err)
	}
	if cfg.Default != "notes" {
		t.Errorf("default = %q, 期望 notes", cfg.Default)
	}
	if len(cfg.Libraries) != 3 {
		t.Fatalf("库数量 = %d, 期望 3", len(cfg.Libraries))
	}
	last := cfg.Libraries[2]
	if last.Name != "ideas" || last.Root != "/abs/ideas" {
		t.Errorf("新库 = %+v", last)
	}
	// 已有库的 config 子表必须在（DeepEqual 自校验只看解析结果，这里显式再钉一次）。
	if got := cfg.Libraries[1].Config.Preview; got != "html" {
		t.Errorf("已有库的 preview = %q, 期望 html", got)
	}
}

func TestAppendLibrarySpecAddsMissingTrailingNewline(t *testing.T) {
	in := "[[libraries]]\nname = \"a\"\nroot = \"/a\"" // 末尾没有换行
	out, err := AppendLibrarySpec(in, "b", "/b")
	if err != nil {
		t.Fatalf("AppendLibrarySpec: %v", err)
	}
	if !strings.HasPrefix(out, in+"\n") {
		t.Errorf("原文本应先被补上换行再追加:\n%s", out)
	}
	cfg, err := ParseWorkspaceConfig(out)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(cfg.Libraries) != 2 || cfg.Libraries[1].Name != "b" {
		t.Errorf("解析结果 = %+v", cfg.Libraries)
	}
}

func TestAppendLibrarySpecOnEmptyText(t *testing.T) {
	out, err := AppendLibrarySpec("", "solo", "/abs/solo")
	if err != nil {
		t.Fatalf("AppendLibrarySpec: %v", err)
	}
	if want := "[[libraries]]\nname = \"solo\"\nroot = \"/abs/solo\"\n"; out != want {
		t.Errorf("got %q\nwant %q", out, want)
	}
}

func TestAppendLibrarySpecRejects(t *testing.T) {
	cases := []struct {
		name string
		text string
		lib  string
		root string
	}{
		{"重名", commentedTOML, "notes", "/abs/other"},
		{"相对路径 root", commentedTOML, "ideas", "relative/ideas"},
		{"空 root", commentedTOML, "ideas", ""},
		{"无法解析的现有文件", "default = \n[[libraries\n", "ideas", "/abs/ideas"},
		{"库名含分隔符", commentedTOML, "a/b", "/abs/ideas"},
		{"库名含空白", commentedTOML, "a b", "/abs/ideas"},
		{"库名为空", commentedTOML, "", "/abs/ideas"},
		{"库名含冒号", commentedTOML, "a:b", "/abs/ideas"},
		{"库名不可打印", commentedTOML, "a\x7fb", "/abs/ideas"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, err := AppendLibrarySpec(c.text, c.lib, c.root)
			if err == nil {
				t.Fatalf("期望报错，却得到:\n%s", out)
			}
			if out != "" {
				t.Errorf("报错时不应返回文本，得到 %q", out)
			}
		})
	}
}

func TestSetDefaultLibraryRewritesInPlaceKeepingComment(t *testing.T) {
	out, err := SetDefaultLibrary(commentedTOML, "work")
	if err != nil {
		t.Fatalf("SetDefaultLibrary: %v", err)
	}

	before, after := strings.Split(commentedTOML, "\n"), strings.Split(out, "\n")
	if len(before) != len(after) {
		t.Fatalf("行数从 %d 变成 %d，期望原地替换:\n%s", len(before), len(after), out)
	}
	var diffs []int
	for i := range before {
		if before[i] != after[i] {
			diffs = append(diffs, i)
		}
	}
	if len(diffs) != 1 {
		t.Fatalf("改动了 %d 行（%v），期望只有 default 那一行:\n%s", len(diffs), diffs, out)
	}
	if want := `default = "work"  # 默认库`; after[diffs[0]] != want {
		t.Errorf("default 行 = %q\n期望 = %q", after[diffs[0]], want)
	}

	cfg, err := ParseWorkspaceConfig(out)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if cfg.Default != "work" {
		t.Errorf("default = %q, 期望 work", cfg.Default)
	}
	if len(cfg.Libraries) != 2 {
		t.Errorf("库数量 = %d, 期望 2", len(cfg.Libraries))
	}
}

func TestSetDefaultLibraryInsertsBeforeFirstTable(t *testing.T) {
	in := `# 顶部注释

[[libraries]]
name = "notes"
root = "/abs/notes"
`
	out, err := SetDefaultLibrary(in, "notes")
	if err != nil {
		t.Fatalf("SetDefaultLibrary: %v", err)
	}
	want := `# 顶部注释

default = "notes"
[[libraries]]
name = "notes"
root = "/abs/notes"
`
	if out != want {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestSetDefaultLibraryInsertsAfterTopLevelKeys(t *testing.T) {
	in := `data_dir = "/abs/data"

[[libraries]]
name = "notes"
root = "/abs/notes"
`
	out, err := SetDefaultLibrary(in, "notes")
	if err != nil {
		t.Fatalf("SetDefaultLibrary: %v", err)
	}
	if !strings.HasPrefix(out, "data_dir = \"/abs/data\"\n\ndefault = \"notes\"\n") {
		t.Errorf("got:\n%s", out)
	}
	cfg, err := ParseWorkspaceConfig(out)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if cfg.DataDir != "/abs/data" || cfg.Default != "notes" {
		t.Errorf("cfg = %+v", cfg)
	}
}

func TestSetDefaultLibraryRejectsUnknownNameAndKeepsInput(t *testing.T) {
	// SetDefaultLibrary 不校验「这个名字是否存在」——它只是文本编辑，
	// 但非法库名必须被挡住。
	if _, err := SetDefaultLibrary(commentedTOML, "a/b"); err == nil {
		t.Error("含分隔符的库名应被拒绝")
	}
	if _, err := SetDefaultLibrary("default = \n", "ok"); err == nil {
		t.Error("无法解析的现有文件应被拒绝")
	}
}

func TestTrailingComment(t *testing.T) {
	cases := []struct {
		line string
		want string
	}{
		{`default = "notes"`, ""},
		{`default = "notes"  # 默认库`, "  # 默认库"},
		{"default = \"notes\"\t# tab", "\t# tab"},
		{`default = "a#b"`, ""}, // '#' 在引号里，不是注释
		{`default = "a#b" # 真注释`, " # 真注释"},
		{`# 整行注释`, ""}, // 没有 '='，不是键值行
		{`default = 'notes' # 单引号`, " # 单引号"},
	}
	for _, c := range cases {
		if got := trailingComment(c.line); got != c.want {
			t.Errorf("trailingComment(%q) = %q, want %q", c.line, got, c.want)
		}
	}
}

func TestValidateLibraryName(t *testing.T) {
	ok := []string{"notes", "work-2", "a_b.c", "中文库", "笔记2026"}
	for _, name := range ok {
		if err := ValidateLibraryName(name); err != nil {
			t.Errorf("ValidateLibraryName(%q) 意外报错: %v", name, err)
		}
	}
	bad := []string{"", ".", "..", "a/b", `a\b`, "a b", " a", "a ", "a\tb", "a\nb",
		"a:b", "a*b", `a"b`, "a<b", "a>b", "a?b", "a|b", "a\x7fb"}
	for _, name := range bad {
		if err := ValidateLibraryName(name); err == nil {
			t.Errorf("ValidateLibraryName(%q) 应当报错", name)
		}
	}
}

func TestAddLibraryToWorkspaceSeedsMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ws", "zoro.toml") // 父目录也不存在
	root := filepath.Join(dir, "notes")

	if err := AddLibraryToWorkspace(path, "notes", root, true); err != nil {
		t.Fatalf("AddLibraryToWorkspace: %v", err)
	}
	cfg, err := WorkspaceConfigFromPath(path)
	if err != nil {
		t.Fatalf("读取新建文件失败: %v", err)
	}
	if cfg.Default != "notes" {
		t.Errorf("default = %q, 期望 notes", cfg.Default)
	}
	if len(cfg.Libraries) != 1 || cfg.Libraries[0].Root != root {
		t.Fatalf("libraries = %+v", cfg.Libraries)
	}
}

// setDefault=false：原有字节必须**原样成为结果的前缀**，末尾只多出新段。
func TestAddLibraryToWorkspaceAppendOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "zoro.toml")
	if err := os.WriteFile(path, []byte(commentedTOML), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := AddLibraryToWorkspace(path, "ideas", "/abs/ideas", false); err != nil {
		t.Fatalf("AddLibraryToWorkspace: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), commentedTOML) {
		t.Fatalf("原有字节（含注释）应原样保留:\n%s", data)
	}
	cfg, err := ParseWorkspaceConfig(string(data))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if cfg.Default != "notes" {
		t.Errorf("default = %q, 期望保持 notes", cfg.Default)
	}
	if len(cfg.Libraries) != 3 || cfg.Libraries[2].Name != "ideas" {
		t.Errorf("libraries = %+v", cfg.Libraries)
	}
}

// setDefault=true：允许改动**一行**（default），其余每一行都必须逐字节相同。
func TestAddLibraryToWorkspaceWithDefaultChangesOneLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "zoro.toml")
	if err := os.WriteFile(path, []byte(commentedTOML), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := AddLibraryToWorkspace(path, "ideas", "/abs/ideas", true); err != nil {
		t.Fatalf("AddLibraryToWorkspace: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)

	appended := "\n[[libraries]]\nname = \"ideas\"\nroot = \"/abs/ideas\"\n"
	if !strings.HasSuffix(got, appended) {
		t.Fatalf("结尾应是新追加的段:\n%s", got)
	}
	before := strings.Split(commentedTOML, "\n")
	after := strings.Split(strings.TrimSuffix(got, appended), "\n")
	if len(before) != len(after) {
		t.Fatalf("行数 %d → %d，期望除 default 外原地不变:\n%s", len(before), len(after), got)
	}
	var diffs []int
	for i := range before {
		if before[i] != after[i] {
			diffs = append(diffs, i)
		}
	}
	if len(diffs) != 1 {
		t.Fatalf("改动了 %d 行（%v），期望只有 default 一行:\n%s", len(diffs), diffs, got)
	}
	if want := `default = "ideas"  # 默认库`; after[diffs[0]] != want {
		t.Errorf("default 行 = %q\n期望 = %q", after[diffs[0]], want)
	}
	for _, want := range []string{"# 这两行注释必须在任何程序化修改之后原样存在", `future_key = "还没实现的键"`, "# 第二个库"} {
		if !strings.Contains(got, want) {
			t.Errorf("缺少 %q:\n%s", want, got)
		}
	}

	cfg, err := ParseWorkspaceConfig(got)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if cfg.Default != "ideas" {
		t.Errorf("default = %q, 期望 ideas", cfg.Default)
	}
	if len(cfg.Libraries) != 3 || cfg.Libraries[2].Name != "ideas" {
		t.Errorf("libraries = %+v", cfg.Libraries)
	}

	// 权限位保留 + 没有残留临时文件。
	if info, err := os.Stat(path); err != nil {
		t.Fatal(err)
	} else if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("权限 = %v, 期望 0600", perm)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("残留了临时文件 %s.tmp", path)
	}

	// 再加一个不设 default 的库：default 必须保持 ideas。
	if err := AddLibraryToWorkspace(path, "later", "/abs/later", false); err != nil {
		t.Fatalf("第二次 AddLibraryToWorkspace: %v", err)
	}
	cfg2, err := WorkspaceConfigFromPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg2.Default != "ideas" {
		t.Errorf("default = %q, 期望仍是 ideas", cfg2.Default)
	}
	if len(cfg2.Libraries) != 4 || cfg2.Libraries[3].Name != "later" {
		t.Errorf("libraries = %+v", cfg2.Libraries)
	}
}

func TestAddLibraryToWorkspaceRejectsDuplicate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "zoro.toml")
	if err := os.WriteFile(path, []byte(commentedTOML), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AddLibraryToWorkspace(path, "notes", "/abs/elsewhere", false); err == nil {
		t.Fatal("重名应报错")
	}
	// 失败路径不得改动文件。
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != commentedTOML {
		t.Errorf("报错后文件被改动了:\n%s", data)
	}
}

/* ------------------------------------------------------------
   AddLibrary：CLI `zoro add` 与 Launcher `/lib add` 共用的整条链路
   ------------------------------------------------------------ */

// 链路里最容易被漏掉的三件事：相对 root 的解析基准、库目录的创建、首个库自动设默认。
func TestAddLibraryFirstLibraryResolvesRootAndSetsDefault(t *testing.T) {
	dir := t.TempDir()
	wsPath := filepath.Join(dir, "ws", "zoro.toml") // 声明文件还不存在

	added, err := AddLibrary(wsPath, "notes", "content/notes", false)
	if err != nil {
		t.Fatalf("AddLibrary: %v", err)
	}
	// 相对路径以 zoro.toml 所在目录为基准，不是进程 cwd（Launcher 的 cwd 是 /）。
	if want := filepath.Join(dir, "ws", "content", "notes"); added.Root != want {
		t.Errorf("Root = %q, 期望 %q", added.Root, want)
	}
	if !added.CreatedDir {
		t.Error("CreatedDir 应为 true")
	}
	if info, err := os.Stat(added.Root); err != nil || !info.IsDir() {
		t.Errorf("库目录应已创建: %v", err)
	}
	if !added.First || !added.SetDefault {
		t.Errorf("首个库应自动设为默认: %+v", added)
	}

	cfg, err := WorkspaceConfigFromPath(wsPath)
	if err != nil {
		t.Fatalf("读取新建声明失败: %v", err)
	}
	if cfg.Default != "notes" || len(cfg.Libraries) != 1 || cfg.Libraries[0].Root != added.Root {
		t.Errorf("声明 = %+v", cfg)
	}
}

// 第二次添加：既不抢 default，也不动用户手写的注释与未知键。
func TestAddLibraryKeepsCommentsAndExistingDefault(t *testing.T) {
	dir := t.TempDir()
	wsPath := filepath.Join(dir, "zoro.toml")
	if err := os.WriteFile(wsPath, []byte(commentedTOML), 0o644); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "ideas")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}

	added, err := AddLibrary(wsPath, "ideas", root, false)
	if err != nil {
		t.Fatalf("AddLibrary: %v", err)
	}
	if added.CreatedDir || added.First || added.SetDefault {
		t.Errorf("已存在的目录 + 非首个库: %+v", added)
	}
	data, err := os.ReadFile(wsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), commentedTOML) {
		t.Fatalf("原有字节（含注释）应原样保留:\n%s", data)
	}
	cfg, err := ParseWorkspaceConfig(string(data))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Default != "notes" {
		t.Errorf("default = %q, 期望仍是 notes", cfg.Default)
	}
	if len(cfg.Libraries) != 3 || cfg.Libraries[2].Name != "ideas" {
		t.Errorf("libraries = %+v", cfg.Libraries)
	}
}

// 重名 / 非法库名 / 空 root 一律拒绝，且**不建目录、不改声明文件**。
func TestAddLibraryRejects(t *testing.T) {
	dir := t.TempDir()
	wsPath := filepath.Join(dir, "zoro.toml")
	if err := os.WriteFile(wsPath, []byte(commentedTOML), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ name, root string }{
		{"notes", filepath.Join(dir, "elsewhere")}, // 重名
		{"../evil", filepath.Join(dir, "x")},       // 非法库名
		{"", filepath.Join(dir, "x")},              // 空库名
		{"ok", "   "},                              // 空 root
	} {
		if _, err := AddLibrary(wsPath, c.name, c.root, false); err == nil {
			t.Errorf("AddLibrary(%q, %q) 应报错", c.name, c.root)
		}
	}
	data, err := os.ReadFile(wsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != commentedTOML {
		t.Errorf("报错后声明被改动了:\n%s", data)
	}
	if _, err := os.Stat(filepath.Join(dir, "elsewhere")); !os.IsNotExist(err) {
		t.Error("失败路径不该创建目录")
	}
}
