package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"zoro/core"
)

// LauncherConfig holds UI preferences for the launcher.
type LauncherConfig struct {
	Theme string `toml:"theme"` // "light-glass" | "dark-glass" | "minimal"
}

// App is the Wails-bound bridge between the launcher UI and zoro core.
type App struct {
	ctx    context.Context
	ws     *core.Workspace
	wsErr  error
	wsPath string
	config LauncherConfig

	windowVisible    bool
	unregisterHotkey func()
	popoutManager    *PopoutManager
}

// NewApp creates the bound app.
func NewApp() *App {
	return &App{
		popoutManager: NewPopoutManager(),
	}
}

// startup loads the zoro workspace (cold-start first, auto rebuild when stale).
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.wsPath = launcherWorkspacePath()
	if p, err := core.DefaultWorkspacePath(); err == nil && a.wsPath == p {
		if _, err := core.EnsureDefaultWorkspace(); err != nil {
			a.wsErr = err
			return
		}
	}
	ws, err := core.LoadWorkspace(a.wsPath)
	if err != nil {
		a.ws = nil
		a.wsErr = err
		return
	}
	a.ws = ws
	a.loadConfig()
}

// loadConfig reads launcher preferences from ~/.zoro/launcher.toml
func (a *App) loadConfig() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	cfgPath := filepath.Join(home, ".zoro", "launcher.toml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
		if key == "theme" {
			a.config.Theme = val
		}
	}

	switch a.config.Theme {
	case "light-glass", "dark-glass", "minimal":
		// valid
	default:
		a.config.Theme = "light-glass"
	}
}

// Status returns a short human-readable line about the loaded workspace.
//
// ⚠️ 它有副作用：会显式 RefreshAll 一次。原因是 Library.Query 是「尽力刷新」——
// store 被 CLI 占用（ErrStoreLocked）或库目录不可读时，它**静默**用内存里的旧块继续搜。
// 用户看不见这件事，就会把「索引没刷新」误读成「我的内容没了」。
// 前端在启动时和每次改动工作区之后各调一次，所以这里的开销是一次指纹快路径。
func (a *App) Status() string {
	if a.wsErr != nil {
		return fmt.Sprintf("读取工作区失败（%s）：%v", a.wsPath, a.wsErr)
	}
	if a.ws == nil {
		return fmt.Sprintf("未加载工作区（%s）", a.wsPath)
	}
	if err := a.ws.RefreshAll(); err != nil {
		return fmt.Sprintf("%s · %d 个知识库 · ⚠️ 索引未刷新（结果可能是旧的）：%v",
			a.wsPath, len(a.ws.Libraries), err)
	}
	return fmt.Sprintf("%s · %d 个知识库", a.wsPath, len(a.ws.Libraries))
}

/* ------------------------------------------------------------
   知识库管理（前端命令面 /lib 的后端）
   ------------------------------------------------------------ */

// eventProgress 是进度事件名。前端在 init 里 EventsOn 一次，之后所有长操作
// （建索引、重建索引）都往状态栏推一行字，窗口不至于看起来卡死。
const eventProgress = "zoro:progress"

// LibraryInfo 是 `/lib` 的一行。前端直接渲染它，不需要再回访 core。
type LibraryInfo struct {
	Name      string `json:"name"`
	Root      string `json:"root"`
	Blocks    int    `json:"blocks"`
	IsDefault bool   `json:"isDefault"`
}

// LibraryAddReport 是 `/lib add` 成功后回给前端的结果。
type LibraryAddReport struct {
	Name       string `json:"name"`
	Root       string `json:"root"`
	Blocks     int    `json:"blocks"`
	CreatedDir bool   `json:"createdDir"`
	SetDefault bool   `json:"setDefault"`
}

// ListLibraries 返回已声明的库（含块数与默认标记）。
//
// 块数取自内存里的 Blocks（打开工作区时已加载），**不触发刷新**：
// `/lib` 只是个概览，为它去抢 bbolt 的独占锁不值得（见 pitfalls P-13）。
func (a *App) ListLibraries() ([]LibraryInfo, error) {
	if a.ws == nil {
		return nil, a.workspaceErr()
	}
	// ⚠️ a.ws 不知道 zoro.toml 里的 default 是哪个库，所以要单独读一次声明。
	def := ""
	if cfg, err := core.WorkspaceConfigFromPath(a.wsPath); err == nil {
		def = cfg.Default
	}
	out := make([]LibraryInfo, 0, len(a.ws.Libraries))
	for _, lib := range a.ws.Libraries {
		out = append(out, LibraryInfo{
			Name:      lib.Name,
			Root:      lib.Root,
			Blocks:    len(lib.Blocks),
			IsDefault: lib.Name == def,
		})
	}
	return out, nil
}

// PickDirectory 弹原生目录选择框（macOS 上是 NSOpenPanel）。
//
// 用户取消时返回**空串而不是错误** —— 取消是一次正常操作，前端静默回到搜索即可。
// 目录选择走原生控件而不是让用户在 780px 的输入框里打路径：含空格的路径、
// 中文目录名、Tab 补全都是原生框已经解决好的事（方案 §4.3「难输入怎么解决」）。
func (a *App) PickDirectory() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("app not started")
	}
	return wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "选择知识库目录",
	})
}

// AddLibrary 添加一个知识库：写声明 → 重载工作区 → 给新库建索引。
//
// 「校验库名 / 查重 / root 绝对化 / 建目录 / 外科式写回 zoro.toml」整条链路在
// core.AddLibrary 里，CLI `zoro add` 走的是**同一个函数**，两边语义不可能漂移。
// 这里只多做常驻进程必须做的两步（方案 §4.4 第 6、7 步，漏掉的症状写在注释里）：
//
//  6. 重载工作区 —— a.ws 是启动时的快照，不重载就「加了库但搜不到」。
//     P-13 之后 store 从不跨调用持有，所以旧工作区没有需要 Close 的东西。
//  7. 对新库 Analyze(true) —— 新目录可能已有几百个 md，也可能带着一份陈旧的 .zoro/zoro.db。
func (a *App) AddLibrary(name, root string) (LibraryAddReport, error) {
	added, err := core.AddLibrary(a.wsPath, name, root, false)
	if err != nil {
		return LibraryAddReport{}, err
	}

	ws, err := core.LoadWorkspace(a.wsPath)
	if err != nil {
		return LibraryAddReport{}, fmt.Errorf("已写入 %s，但重新加载工作区失败（重启 Launcher 可恢复）: %w", a.wsPath, err)
	}
	a.ws, a.wsErr = ws, nil

	lib, ok := ws.Library(added.Name)
	if !ok {
		return LibraryAddReport{}, fmt.Errorf("已写入 %s，但重载后找不到库 %s", a.wsPath, added.Name)
	}
	a.emitProgress("正在建立索引：" + added.Name)
	if _, err := lib.Analyze(true); err != nil {
		// 声明已经写好了，索引失败是可恢复的（/reindex 或下次启动会重建），
		// 但必须让用户知道 —— 否则他会以为「加了却搜不到」是 bug。
		return LibraryAddReport{}, fmt.Errorf("已添加 %s，但建立索引失败（可用 /reindex 重试）: %w", added.Name, err)
	}
	return LibraryAddReport{
		Name:       added.Name,
		Root:       added.Root,
		Blocks:     len(lib.Blocks),
		CreatedDir: added.CreatedDir,
		SetDefault: added.SetDefault,
	}, nil
}

// Reindex 强制重建所有库的索引（等价于 CLI `zoro index`）。
// 大库可能要几秒，所以每个库开始前都推一次进度事件。
func (a *App) Reindex() (string, error) {
	if a.ws == nil {
		return "", a.workspaceErr()
	}
	total := len(a.ws.Libraries)
	blocks := 0
	for i, lib := range a.ws.Libraries {
		a.emitProgress(fmt.Sprintf("正在重建索引（%d/%d）：%s", i+1, total, lib.Name))
		if _, err := lib.Analyze(true); err != nil {
			return "", fmt.Errorf("重建 %s 的索引失败: %w", lib.Name, err)
		}
		blocks += len(lib.Blocks)
	}
	return fmt.Sprintf("已重建 %d 个知识库的索引，共 %d 个块", total, blocks), nil
}

// RevealLibrary 在 Finder 里打开某个库的根目录（`/lib` 列表行按 Enter 的动作）。
//
// 打开目录走原生文件管理器，不要在 Launcher 里自造文件树 —— 用户的笔记目录
// 本来就有一整套成熟工具（Finder / 编辑器 / 终端），我们只需要把入口接到那里。
func (a *App) RevealLibrary(name string) (string, error) {
	if a.ws == nil {
		return "", a.workspaceErr()
	}
	lib, ok := a.ws.Library(name)
	if !ok {
		return "", fmt.Errorf("库不存在: %s（可能刚被删掉，重启 Launcher 后重试）", name)
	}
	if err := launchFile(lib.Root); err != nil {
		return "", err
	}
	return lib.Root, nil
}

func (a *App) emitProgress(msg string) {
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, eventProgress, msg)
	}
}

// Query returns ranked candidates from the workspace (empty query = browse all).
func (a *App) Query(q string) []core.Candidate {
	if a.ws == nil {
		return nil
	}
	return a.ws.Query(q)
}

// LoadRaw cuts a block's raw Markdown on demand by (library, path, start).
func (a *App) LoadRaw(library, path string, start int) (string, error) {
	if a.ws == nil {
		return "", a.workspaceErr()
	}
	return a.ws.LoadRaw(library, path, start)
}

// RenderHTML renders Markdown to HTML for the preview pane.
func (a *App) RenderHTML(raw string) (string, error) {
	return core.RenderMarkdownHTML(raw)
}

// ResolveSource maps a candidate's (library, path) to a source file path.
func (a *App) ResolveSource(library, path string) (string, error) {
	if a.ws == nil {
		return "", a.workspaceErr()
	}
	lib, ok := a.ws.Library(library)
	if !ok {
		return "", fmt.Errorf("library not found: %s", library)
	}
	return filepath.Join(lib.Root, filepath.FromSlash(path)), nil
}

// OpenSource opens the candidate's source Markdown file with the system app.
func (a *App) OpenSource(library, path string) (string, error) {
	p, err := a.ResolveSource(library, path)
	if err != nil {
		return "", err
	}
	if err := launchFile(p); err != nil {
		return "", err
	}
	return p, nil
}

// PopoutResult creates a floating window with the rendered result using Preview.app.
func (a *App) PopoutResult(library, path string, start int) error {
	raw, err := a.LoadRaw(library, path, start)
	if err != nil {
		return err
	}

	html, err := a.RenderHTML(raw)
	if err != nil {
		return err
	}

	// Write to temp file
	tmpDir, err := os.MkdirTemp("", "zoro-popout-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}

	tmpFile := filepath.Join(tmpDir, "result.html")
	fullHTML := generatePopoutHTML(html)
	if err := os.WriteFile(tmpFile, []byte(fullHTML), 0644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	// Open with Preview.app on macOS (lightweight, stays on screen)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", "-a", "Preview", tmpFile)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", tmpFile)
	default:
		cmd = exec.Command("xdg-open", tmpFile)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open %q: %w", tmpFile, err)
	}
	return nil
}

// Copy puts text on the system clipboard.
func (a *App) Copy(text string) error {
	if a.ctx == nil {
		return fmt.Errorf("app not started")
	}
	return wailsruntime.ClipboardSetText(a.ctx, text)
}

// GetTheme returns the configured UI theme name.
func (a *App) GetTheme() string {
	return a.config.Theme
}

// Hide hides the launcher window.
func (a *App) Hide() {
	if a.ctx != nil {
		wailsruntime.WindowHide(a.ctx)
	}
	a.windowVisible = false
}

func (a *App) workspaceErr() error {
	if a.wsErr != nil {
		return a.wsErr
	}
	return fmt.Errorf("workspace not loaded")
}

func launcherWorkspacePath() string {
	if p := os.Getenv("ZORO_WORKSPACE"); p != "" {
		return p
	}
	if _, err := os.Stat("zoro.toml"); err == nil {
		return "zoro.toml"
	}
	if p, err := core.DefaultWorkspacePath(); err == nil {
		return p
	}
	return "zoro.toml"
}

func launchFile(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open %q: %w", path, err)
	}
	return nil
}

func generatePopoutHTML(content string) string {
	return `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>zoro result</title>
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      font-family: system-ui, -apple-system, sans-serif;
      line-height: 1.6;
      color: #1c1e24;
      background: rgba(255, 255, 255, 0.95);
      padding: 20px;
      overflow-y: auto;
    }
    .content {
      max-width: 800px;
      margin: 0 auto;
    }
    h1, h2, h3 { margin-top: 1em; margin-bottom: 0.5em; }
    p { margin: 0.5em 0; }
    pre {
      background: #f5f6f8;
      padding: 12px;
      border-radius: 6px;
      overflow-x: auto;
      margin: 1em 0;
    }
    code {
      font-family: "SF Mono", Monaco, monospace;
      font-size: 0.9em;
    }
    a { color: #3157d1; }
    blockquote {
      border-left: 4px solid #ddd;
      padding-left: 16px;
      margin: 1em 0;
      color: #666;
    }
    ul, ol { margin: 0.5em 0 0.5em 1.5em; }
    li { margin: 0.25em 0; }
    table {
      border-collapse: collapse;
      width: 100%;
      margin: 1em 0;
    }
    th, td {
      border: 1px solid #ddd;
      padding: 8px;
      text-align: left;
    }
    th { background: #f5f6f8; }
  </style>
</head>
<body>
  <div class="content">` + content + `</div>
</body>
</html>`
}
