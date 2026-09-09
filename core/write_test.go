package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 写 API 直接改用户的内容文件，所以这里的测试密度要高于其它文件。守四件事：
//	① 写进去的字节形状固定，且能被扫描器**原样读回**（否则等于静默丢块）；
//	② 改/删只动块范围内的字节：别的块、CRLF、尾部空行、**下一个块的 ## 标题**都不受牵连
//	   （块自己的 ## 标题：改写不碰，删除刻意一起摘掉，否则会留下孤儿标题）；
//	③ 前置校验失败时文件**一个字节都不变**；
//	④ 落盘成功但索引刷新失败 ≠ 写失败（可降级，原因必须可观测）。

func newWriteLib(t *testing.T, cfg LibraryConfig) *Library {
	t.Helper()
	lib, err := OpenLibraryCachedWithConfig("notes", t.TempDir(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	return lib
}

func rel2abs(root, rel string) string { return filepath.Join(root, filepath.FromSlash(rel)) }

func mustRead(t *testing.T, lib *Library, rel string) string {
	t.Helper()
	data, err := os.ReadFile(rel2abs(lib.Root, rel))
	if err != nil {
		t.Fatalf("读取 %s: %v", rel, err)
	}
	return string(data)
}

func writeSeed(t *testing.T, lib *Library, rel, content string) {
	t.Helper()
	abs := rel2abs(lib.Root, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

/* ------------------------------------------------------------
   ① 追加：文本形状
   ------------------------------------------------------------ */

func TestAppendBlockCreatesFileAndMakesItSearchable(t *testing.T) {
	lib := newWriteLib(t, LibraryConfig{})

	res, err := lib.AppendBlock(BlockDraft{
		Title: "定投策略",
		Terms: []string{"定投", "指数"},
		Body:  "每周定投沪深300。",
	})
	if err != nil {
		t.Fatalf("AppendBlock: %v", err)
	}
	if res.Path != "inbox.md" {
		t.Errorf("Path = %q, 期望默认 capture 文件 inbox.md", res.Path)
	}
	if !res.Analyzed {
		t.Errorf("Analyzed = false, RefreshErr = %v", res.RefreshErr)
	}
	if res.Start != 3 {
		t.Errorf("Start = %d, 期望 3（## 行 / 空行 / @ 行）", res.Start)
	}
	want := "## 定投策略\n\n@index 定投 指数\n每周定投沪深300。\n"
	if got := mustRead(t, lib, res.Path); got != want {
		t.Errorf("文件内容 = %q\n期望     = %q", got, want)
	}

	// 返回的三元组必须立刻可用（LoadRaw + Query 都走它）。
	raw, err := lib.LoadRaw(&Block{Path: res.Path, Start: res.Start})
	if err != nil {
		t.Fatalf("LoadRaw: %v", err)
	}
	if wantRaw := "@index 定投 指数\n每周定投沪深300。"; raw != wantRaw {
		t.Errorf("LoadRaw = %q, 期望 %q", raw, wantRaw)
	}
	hits := lib.Query("定投")
	if len(hits) != 1 {
		t.Fatalf("写后查询命中 %d 条，期望 1（写后必须已刷新索引）", len(hits))
	}
	if hits[0].Start != res.Start || hits[0].Path != res.Path {
		t.Errorf("命中 = %+v", hits[0])
	}
}

func TestAppendBlockShapeWithoutTitle(t *testing.T) {
	lib := newWriteLib(t, LibraryConfig{})
	res, err := lib.AppendBlock(BlockDraft{Terms: []string{"无标题"}, Body: "正文"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "@index 无标题\n正文\n"; mustRead(t, lib, res.Path) != want {
		t.Errorf("got %q want %q", mustRead(t, lib, res.Path), want)
	}
	if res.Start != 1 {
		t.Errorf("Start = %d, 期望 1", res.Start)
	}
}

func TestAppendBlockSeparatesFromExistingContent(t *testing.T) {
	cases := []struct {
		name      string
		seed      string
		wantText  string
		wantStart int
	}{
		{
			name:      "已以换行结尾 → 空一行",
			seed:      "@index 旧\n旧正文\n",
			wantText:  "@index 旧\n旧正文\n\n## 新\n\n@index 新\n新正文\n",
			wantStart: 6,
		},
		{
			name:      "最后一行没有收尾换行 → 先收尾再空一行",
			seed:      "@index 旧\n旧正文",
			wantText:  "@index 旧\n旧正文\n\n## 新\n\n@index 新\n新正文\n",
			wantStart: 6,
		},
		{
			name:      "已有标题的块 → 新块自带标题",
			seed:      "## 旧\n\n@index 旧\n旧正文\n",
			wantText:  "## 旧\n\n@index 旧\n旧正文\n\n## 新\n\n@index 新\n新正文\n",
			wantStart: 8,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			lib := newWriteLib(t, LibraryConfig{})
			writeSeed(t, lib, "inbox.md", c.seed)
			// 让内存指纹与磁盘一致，模拟「库已经加载过」。
			if err := lib.Refresh(); err != nil {
				t.Fatal(err)
			}

			res, err := lib.AppendBlock(BlockDraft{Path: "inbox.md", Title: "新", Terms: []string{"新"}, Body: "新正文"})
			if err != nil {
				t.Fatal(err)
			}
			if got := mustRead(t, lib, "inbox.md"); got != c.wantText {
				t.Errorf("文件 = %q\n期望 = %q", got, c.wantText)
			}
			if res.Start != c.wantStart {
				t.Errorf("Start = %d, 期望 %d", res.Start, c.wantStart)
			}
			if raw, err := lib.LoadRaw(&Block{Path: res.Path, Start: res.Start}); err != nil {
				t.Fatal(err)
			} else if raw != "@index 新\n新正文" {
				t.Errorf("LoadRaw = %q", raw)
			}
			if len(lib.Query("新")) != 1 {
				t.Errorf("查询「新」应命中 1 条，实际 %d", len(lib.Query("新")))
			}
			if len(lib.Query("旧")) != 1 {
				t.Errorf("旧块不应被影响，查询「旧」命中 %d 条", len(lib.Query("旧")))
			}
		})
	}
}

func TestAppendBlockUsesCaptureFileFromConfig(t *testing.T) {
	lib := newWriteLib(t, LibraryConfig{Extra: map[string]any{"capture_file": "收集箱/inbox.md"}})
	if got := lib.CaptureFile(); got != "收集箱/inbox.md" {
		t.Fatalf("CaptureFile = %q", got)
	}
	res, err := lib.AppendBlock(BlockDraft{Terms: []string{"子目录"}, Body: "正文"})
	if err != nil {
		t.Fatalf("AppendBlock: %v", err)
	}
	if res.Path != "收集箱/inbox.md" {
		t.Errorf("Path = %q", res.Path)
	}
	if mustRead(t, lib, res.Path) == "" {
		t.Error("父目录应被创建、文件应被写入")
	}
}

// 未登记的 kind 必须能被原样写回并读回：TagKind 是开放字符串（脑图这类扩展零 core 改动）。
func TestAppendBlockUnknownKindRoundTrips(t *testing.T) {
	lib := newWriteLib(t, LibraryConfig{})
	res, err := lib.AppendBlock(BlockDraft{
		Kind:  KindUnknown("mindmap"),
		Terms: []string{"架构脑图"},
		Body:  "- 根\n  - 子节点",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := mustRead(t, lib, res.Path)
	if !strings.Contains(text, "@mindmap 架构脑图") {
		t.Fatalf("应写成 @mindmap 而不是 @unknown:mindmap:\n%s", text)
	}
	if strings.Contains(text, "unknown:") {
		t.Fatalf("unknown: 前缀泄漏进了文件（这一行扫描器认不出来 = 静默丢块）:\n%s", text)
	}
	hits := lib.Query("架构脑图")
	if len(hits) != 1 {
		t.Fatalf("命中 %d 条", len(hits))
	}
	blocks := ScanText(text, res.Path)
	if len(blocks) != 1 || blocks[0].Kind != KindUnknown("mindmap") {
		t.Errorf("扫描回来的 kind = %+v", blocks)
	}
}

func TestAppendBlockRejects(t *testing.T) {
	cases := []struct {
		name string
		d    BlockDraft
	}{
		{"没有搜索词", BlockDraft{Body: "正文"}},
		{"搜索词全是空白", BlockDraft{Terms: []string{" ", "\t"}, Body: "正文"}},
		{"搜索词含空格", BlockDraft{Terms: []string{"两个 词"}, Body: "正文"}},
		{"正文含标签行", BlockDraft{Terms: []string{"a"}, Body: "第一行\n@index b\n第三行"}},
		{"正文含缩进标签行", BlockDraft{Terms: []string{"a"}, Body: "第一行\n  @index b"}},
		{"标题换行", BlockDraft{Terms: []string{"a"}, Title: "两\n行"}},
		{"标题带井号", BlockDraft{Terms: []string{"a"}, Title: "## 已经有了"}},
		{"kind 含非法字符", BlockDraft{Kind: TagKind("a:b"), Terms: []string{"a"}}},
		{"路径逃出根目录", BlockDraft{Path: "../evil.md", Terms: []string{"a"}}},
		{"路径是绝对路径", BlockDraft{Path: "/tmp/evil.md", Terms: []string{"a"}}},
		{"路径绕过子目录逃逸", BlockDraft{Path: "sub/../../evil.md", Terms: []string{"a"}}},
		{"扩展名不会被索引", BlockDraft{Path: "notes.txt", Terms: []string{"a"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			lib := newWriteLib(t, LibraryConfig{})
			res, err := lib.AppendBlock(c.d)
			if err == nil {
				t.Fatalf("期望报错，却写成了 %+v:\n%s", res, mustRead(t, lib, res.Path))
			}
			// 被拒绝时不得留下任何文件。
			entries, err := os.ReadDir(lib.Root)
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range entries {
				if e.Name() != ".zoro" {
					t.Errorf("校验失败却产生了 %s", e.Name())
				}
			}
		})
	}
}

func TestResolveContentPath(t *testing.T) {
	ok := []struct{ in, want string }{
		{"inbox.md", "inbox.md"},
		{"a/b/c.md", "a/b/c.md"},
		{"./a.md", "a.md"},
		{"a/../b.md", "b.md"},
		{"中文目录/笔记.mdx", "中文目录/笔记.mdx"},
		{"A.MD", "A.MD"},
	}
	for _, c := range ok {
		abs, rel, err := resolveContentPath("/root/lib", c.in)
		if err != nil {
			t.Errorf("resolveContentPath(%q): %v", c.in, err)
			continue
		}
		if rel != c.want {
			t.Errorf("resolveContentPath(%q) rel = %q, want %q", c.in, rel, c.want)
		}
		if abs != filepath.Join("/root/lib", filepath.FromSlash(c.want)) {
			t.Errorf("resolveContentPath(%q) abs = %q", c.in, abs)
		}
	}
	bad := []string{"", "  ", "/abs.md", "../x.md", "a/../../x.md", "..", "x.txt", "x", "x.md/../../y.md"}
	for _, in := range bad {
		if abs, rel, err := resolveContentPath("/root/lib", in); err == nil {
			t.Errorf("resolveContentPath(%q) 应报错，得到 (%q,%q)", in, abs, rel)
		}
	}
}

/* ------------------------------------------------------------
   ② 改 / 删：范围与前置校验
   ------------------------------------------------------------ */

const twoBlocks = "## 第一块\n\n@index alpha 第一\n第一块正文\n\n## 第二块\n\n@shell git\n```bash\necho hi\n```\n"

func seedTwoBlocks(t *testing.T) (*Library, []Block) {
	t.Helper()
	lib := newWriteLib(t, LibraryConfig{})
	writeSeed(t, lib, "notes.md", twoBlocks)
	if err := lib.Refresh(); err != nil {
		t.Fatal(err)
	}
	if len(lib.Blocks) != 2 {
		t.Fatalf("种子文件应扫出 2 个块，实际 %d: %+v", len(lib.Blocks), lib.Blocks)
	}
	return lib, lib.Blocks
}

func TestUpdateBlockReplacesOnlyTheBlock(t *testing.T) {
	lib, blocks := seedTwoBlocks(t)
	target := blocks[1] // @shell 那块
	expect, err := lib.LoadRaw(&Block{Path: target.Path, Start: target.Start})
	if err != nil {
		t.Fatal(err)
	}

	res, err := lib.UpdateBlock(target.Path, target.Start, expect, BlockPatch{
		Terms: []string{"git", "提交"},
		Body:  "```bash\ngit commit -m x\n```",
	})
	if err != nil {
		t.Fatalf("UpdateBlock: %v", err)
	}
	if !res.Analyzed {
		t.Errorf("Analyzed = false, RefreshErr = %v", res.RefreshErr)
	}
	if res.Start != target.Start {
		t.Errorf("Start = %d, 期望原地不变 %d", res.Start, target.Start)
	}

	got := mustRead(t, lib, "notes.md")
	want := "## 第一块\n\n@index alpha 第一\n第一块正文\n\n## 第二块\n\n@shell git 提交\n```bash\ngit commit -m x\n```\n"
	if got != want {
		t.Errorf("文件 = %q\n期望 = %q", got, want)
	}
	// kind 从现有行读出后原样保留（@shell 不会退化成 @index）。
	if raw, err := lib.LoadRaw(&Block{Path: res.Path, Start: res.Start}); err != nil {
		t.Fatal(err)
	} else if !strings.HasPrefix(raw, "@shell git 提交") {
		t.Errorf("LoadRaw = %q", raw)
	}
	if len(lib.Query("提交")) != 1 {
		t.Errorf("新搜索词应能命中")
	}
	if len(lib.Query("echo")) != 0 {
		t.Errorf("旧正文不该再命中（RawFace 是占位，这里只确认没多出块）")
	}
}

func TestUpdateBlockRejectsStaleExpectAndLeavesFileUntouched(t *testing.T) {
	lib, blocks := seedTwoBlocks(t)
	target := blocks[0]

	// 调用方手里的副本过期了（别人改了正文）。
	stale := "@index alpha 第一\n过期的正文"
	before := mustRead(t, lib, "notes.md")
	_, err := lib.UpdateBlock(target.Path, target.Start, stale, BlockPatch{Terms: []string{"x"}, Body: "y"})
	if !errors.Is(err, ErrBlockChanged) {
		t.Fatalf("err = %v, 期望 ErrBlockChanged", err)
	}
	if after := mustRead(t, lib, "notes.md"); after != before {
		t.Errorf("校验失败却改动了文件:\n%s", after)
	}
}

func TestUpdateBlockRejectsMissingExpect(t *testing.T) {
	lib, blocks := seedTwoBlocks(t)
	if _, err := lib.UpdateBlock(blocks[0].Path, blocks[0].Start, "", BlockPatch{Terms: []string{"x"}, Body: "y"}); err == nil {
		t.Error("缺少 expect 应报错（防盲写行号）")
	}
	if _, err := lib.UpdateBlock(blocks[0].Path, blocks[0].Start, "   ", BlockPatch{Terms: []string{"x"}, Body: "y"}); err == nil {
		t.Error("空白 expect 应报错")
	}
}

func TestMutateRejectsNonDirectiveLineAndOutOfRange(t *testing.T) {
	lib, _ := seedTwoBlocks(t)
	// 第 1 行是 `## 第一块`，不是标签行 → 行号漂移的典型症状。
	_, err := lib.DeleteBlock("notes.md", 1, "## 第一块")
	if !errors.Is(err, ErrBlockChanged) {
		t.Errorf("指向标题行应报 ErrBlockChanged, got %v", err)
	}
	if _, err := lib.DeleteBlock("notes.md", 999, "x"); err == nil {
		t.Error("越界行号应报错")
	}
	if _, err := lib.DeleteBlock("notes.md", 0, "x"); err == nil {
		t.Error("行号 0 应报错")
	}
	if _, err := lib.DeleteBlock("../outside.md", 1, "x"); err == nil {
		t.Error("逃逸路径应报错")
	}
	if _, err := lib.DeleteBlock("missing.md", 1, "x"); err == nil {
		t.Error("文件不存在应报错")
	}
}

func TestDeleteBlockRemovesOnlyTheBlockRange(t *testing.T) {
	lib, blocks := seedTwoBlocks(t)
	target := blocks[0] // @index alpha 第一
	expect, err := lib.LoadRaw(&Block{Path: target.Path, Start: target.Start})
	if err != nil {
		t.Fatal(err)
	}
	// L2：下一个块的 `## 第二块` **不在**范围内（扫描器不把标题行放进 Raw），
	// 所以删除也绝不会碰它；尾部空行同样原样保留。
	if expect != "@index alpha 第一\n第一块正文\n\n" {
		t.Fatalf("LoadRaw = %q（边界规则变了？见 write.go L2）", expect)
	}
	// 删除前先把「将删掉的原文」摆给用户看 —— 与真删走同一套范围计算。
	preview, err := lib.PreviewDelete(target.Path, target.Start)
	if err != nil {
		t.Fatalf("PreviewDelete: %v", err)
	}
	if want := "## 第一块\n\n@index alpha 第一\n第一块正文"; preview != want {
		t.Errorf("PreviewDelete = %q\n期望 = %q", preview, want)
	}

	res, err := lib.DeleteBlock(target.Path, target.Start, expect)
	if err != nil {
		t.Fatalf("DeleteBlock: %v", err)
	}
	if !res.Analyzed {
		t.Errorf("Analyzed = false, RefreshErr = %v", res.RefreshErr)
	}
	got := mustRead(t, lib, "notes.md")
	// 块自己的 `## 第一块` 一起摘掉（否则会留下孤儿标题）；
	// 分隔空行、`## 第二块` 与第二个块的每一个字节原样存活。
	if want := "\n## 第二块\n\n@shell git\n```bash\necho hi\n```\n"; got != want {
		t.Errorf("文件 = %q\n期望 = %q", got, want)
	}
	if len(lib.Blocks) != 1 {
		t.Fatalf("删除后应剩 1 个块，实际 %d: %+v", len(lib.Blocks), lib.Blocks)
	}
	if lib.Blocks[0].Kind != KindShell {
		t.Errorf("剩下的块 = %+v", lib.Blocks[0])
	}
	// 行号漂移了（8 → 4）→ 这正是 L1 要求前端整体重新查询、禁止局部 patch 的原因。
	if lib.Blocks[0].Start != 4 {
		t.Errorf("幸存块 Start = %d，期望 4（原来是 %d）", lib.Blocks[0].Start, blocks[1].Start)
	}
	if len(lib.Query("alpha")) != 0 {
		t.Error("被删的块不该再命中")
	}
}

// 标题行夹在负载中间时，它在 expect（预览）里**看不见** → 改写会静默删掉用户的标题。
// 宁可拒绝，不可猜。
func TestMutateRefusesHeadingInsidePayload(t *testing.T) {
	lib := newWriteLib(t, LibraryConfig{})
	// 第 2 块的负载里有一个 `## 夹在中间`，之后还有正文。
	text := "@index alpha\n正文\n@index beta\n前半\n## 夹在中间\n后半\n"
	writeSeed(t, lib, "notes.md", text)
	if err := lib.Refresh(); err != nil {
		t.Fatal(err)
	}
	blocks := ScanText(text, "notes.md")
	if len(blocks) != 2 {
		t.Fatalf("扫出 %d 个块: %+v", len(blocks), blocks)
	}
	target := blocks[1]
	before := mustRead(t, lib, "notes.md")

	if _, err := lib.UpdateBlock(target.Path, target.Start, target.Raw, BlockPatch{Terms: []string{"beta"}, Body: "新正文"}); err == nil {
		t.Error("负载中间夹标题行应拒绝改写")
	}
	if _, err := lib.DeleteBlock(target.Path, target.Start, target.Raw); err == nil {
		t.Error("负载中间夹标题行应拒绝删除")
	}
	if _, err := lib.PreviewDelete(target.Path, target.Start); err == nil {
		t.Error("负载中间夹标题行应拒绝预览删除")
	}
	if after := mustRead(t, lib, "notes.md"); after != before {
		t.Errorf("拒绝写入却改动了文件:\n%s", after)
	}
}

// 正文里写 `## x` 会被扫描器从 Raw 里剔除 → expect 永远对不上、块再也改不动，
// 所以 AppendBlock / UpdateBlock 都必须在落盘前就拒绝。
func TestValidateBodyRejectsHeadingLine(t *testing.T) {
	lib := newWriteLib(t, LibraryConfig{})
	if _, err := lib.AppendBlock(BlockDraft{Terms: []string{"x"}, Body: "正文\n## 偷来的标题\n更多正文"}); err == nil {
		t.Error("正文含标题行应被拒绝")
	}
	if _, err := lib.AppendBlock(BlockDraft{Terms: []string{"x"}, Body: "##"}); err == nil {
		t.Error("整行 ## 也应被拒绝")
	}
	entries, err := os.ReadDir(lib.Root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != ".zoro" {
			t.Errorf("拒绝写入却创建了 %s", e.Name())
		}
	}
}

// @shell 块的代码围栏里有 `@echo off` 这类第 0 列 @ 行时，块**不能**被从中间劈开 ——
// 否则预览只显示半截，删除会留下不闭合的围栏，文件直接坏掉。
func TestShellFenceProtectsAtLinesFromBoundary(t *testing.T) {
	text := "@shell bat\n```bat\n@echo off\necho hi\n```\n@index next\n下一块\n"
	blocks := ScanText(text, "a.md")
	if len(blocks) != 2 {
		t.Fatalf("扫描出 %d 个块，期望 2: %+v", len(blocks), blocks)
	}
	// 预览范围 == 扫描器的 Raw == 改删范围（L2 的三向一致）。
	if got := SliceBlock(text, blocks[0].Start); got != blocks[0].Raw {
		t.Errorf("SliceBlock = %q\nBlock.Raw = %q", got, blocks[0].Raw)
	}
	if !strings.HasSuffix(blocks[0].Raw, "```") {
		t.Errorf("shell 块的 Raw 应包含闭合围栏: %q", blocks[0].Raw)
	}

	lib := newWriteLib(t, LibraryConfig{})
	writeSeed(t, lib, "a.md", text)
	if err := lib.Refresh(); err != nil {
		t.Fatal(err)
	}
	res, err := lib.DeleteBlock("a.md", blocks[0].Start, blocks[0].Raw)
	if err != nil {
		t.Fatalf("DeleteBlock: %v", err)
	}
	got := mustRead(t, lib, "a.md")
	if want := "@index next\n下一块\n"; got != want {
		t.Errorf("文件 = %q\n期望 = %q", got, want)
	}
	if res.Start != blocks[0].Start {
		t.Errorf("Start = %d", res.Start)
	}
	if len(lib.Blocks) != 1 || lib.Blocks[0].Kind != KindIndex {
		t.Errorf("剩下的块 = %+v", lib.Blocks)
	}
}

// 缩进的 @ 行不是边界（parseDirective 要求第 0 列），所以普通块里的缩进列表不会被劈开。
func TestIndentedAtLineIsNotABoundary(t *testing.T) {
	text := "@index 列表\n- 项目\n  @提及某人\n@index 下一块\n正文\n"
	blocks := ScanText(text, "a.md")
	if len(blocks) != 2 {
		t.Fatalf("扫描出 %d 个块: %+v", len(blocks), blocks)
	}
	if got, want := SliceBlock(text, 1), "@index 列表\n- 项目\n  @提及某人"; got != want {
		t.Errorf("SliceBlock = %q, want %q", got, want)
	}
}

/* ------------------------------------------------------------
   ③ 不牵连无关字节：CRLF
   ------------------------------------------------------------ */

func TestUpdateBlockKeepsCRLF(t *testing.T) {
	lib := newWriteLib(t, LibraryConfig{})
	crlf := "## 标题\r\n\r\n@index alpha\r\n旧正文\r\n\r\n## 第二块\r\n\r\n@index beta\r\n第二块正文\r\n"
	writeSeed(t, lib, "notes.md", crlf)
	if err := lib.Refresh(); err != nil {
		t.Fatal(err)
	}
	if len(lib.Blocks) != 2 {
		t.Fatalf("扫出 %d 个块", len(lib.Blocks))
	}
	target := lib.Blocks[0]
	// expect 是 LoadRaw 的返回值：splitLines 会把 \r 去掉，所以比对的是归一化后的文本。
	expect, err := lib.LoadRaw(&Block{Path: target.Path, Start: target.Start})
	if err != nil {
		t.Fatal(err)
	}

	res, err := lib.UpdateBlock(target.Path, target.Start, expect, BlockPatch{Terms: []string{"alpha", "新词"}, Body: "新正文"})
	if err != nil {
		t.Fatalf("UpdateBlock: %v", err)
	}
	got := mustRead(t, lib, "notes.md")
	want := "## 标题\r\n\r\n@index alpha 新词\r\n新正文\r\n\r\n## 第二块\r\n\r\n@index beta\r\n第二块正文\r\n"
	if got != want {
		t.Errorf("文件 = %q\n期望 = %q", got, want)
	}
	if strings.Contains(got, "\n\n") && !strings.Contains(got, "\r\n\r\n") {
		t.Error("换行被归一化成 LF 了")
	}
	if res.Start != target.Start {
		t.Errorf("Start = %d", res.Start)
	}
}

/* ------------------------------------------------------------
   ④ 落盘成功但索引刷新失败 ≠ 写失败
   ------------------------------------------------------------ */

func TestAppendBlockSucceedsWhenStoreIsLocked(t *testing.T) {
	lib := newWriteLib(t, LibraryConfig{})
	held, err := OpenStore(lib.DBPath())
	if err != nil {
		t.Fatal(err)
	}

	res, err := lib.AppendBlock(BlockDraft{Terms: []string{"锁测试"}, Body: "正文"})
	if err != nil {
		t.Fatalf("store 被占用不该让写入失败: %v", err)
	}
	if res.Analyzed {
		t.Error("Analyzed 应为 false")
	}
	if !errors.Is(res.RefreshErr, ErrStoreLocked) {
		t.Errorf("RefreshErr = %v, 期望 ErrStoreLocked", res.RefreshErr)
	}
	if got := mustRead(t, lib, res.Path); !strings.Contains(got, "@index 锁测试") {
		t.Errorf("内容应已落盘:\n%s", got)
	}

	// 解锁后显式刷新 → 立刻可搜（内容从来没丢）。
	held.Close()
	if err := lib.Refresh(); err != nil {
		t.Fatalf("解锁后 Refresh: %v", err)
	}
	if len(lib.Query("锁测试")) != 1 {
		t.Errorf("解锁后应能搜到刚写的块")
	}
}

/* ------------------------------------------------------------
   Workspace 分派
   ------------------------------------------------------------ */

func TestWorkspaceWriteDispatch(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	if err := os.MkdirAll(a, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(b, 0o755); err != nil {
		t.Fatal(err)
	}
	ws, err := NewWorkspace([]LibrarySpec{{Name: "lib-a", Root: a}, {Name: "lib-b", Root: b}})
	if err != nil {
		t.Fatal(err)
	}

	res, err := ws.AppendBlock("lib-b", BlockDraft{Terms: []string{"乙"}, Body: "正文"})
	if err != nil {
		t.Fatalf("AppendBlock: %v", err)
	}
	if res.Library != "lib-b" {
		t.Errorf("Library = %q", res.Library)
	}
	if _, err := os.Stat(filepath.Join(b, "inbox.md")); err != nil {
		t.Errorf("文件应写在 lib-b 的根下: %v", err)
	}
	if _, err := os.ReadDir(filepath.Join(a, "inbox.md")); err == nil {
		t.Error("不该写到 lib-a")
	}

	_, err = ws.AppendBlock("不存在", BlockDraft{Terms: []string{"x"}})
	if err == nil || !strings.Contains(err.Error(), "lib-a") || !strings.Contains(err.Error(), "lib-b") {
		t.Errorf("未知库的报错应列出已声明的库，got %v", err)
	}
}
