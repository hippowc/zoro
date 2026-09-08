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
// It exposes the Launcher v1 action surface: Query + Preview/Render + Copy + Open.
type App struct {
	ctx    context.Context
	ws     *core.Workspace
	wsErr  error
	wsPath string

	config LauncherConfig

	windowVisible    bool
	unregisterHotkey func()
}

// NewApp creates the bound app.
func NewApp() *App { return &App{} }

// startup loads the zoro workspace (cold-start first, auto rebuild when stale).
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.wsPath = launcherWorkspacePath()
	// 全局默认工作区（~/.zoro）在首次启动时自动创建：配置目录、默认知识库与
	// 索引目录都就位，避免用户手动维护 zoro.toml。
	if p, err := core.DefaultWorkspacePath(); err == nil && a.wsPath == p {
		if _, err := core.EnsureDefaultWorkspace(); err != nil {
			a.wsErr = err
			return
		}
	}
	ws, err := core.LoadWorkspace(a.wsPath)
	if err != nil {
		// Workspace errors surface in the UI instead of crashing the launcher.
		a.ws = nil
		a.wsErr = err
		return
	}
	a.ws = ws

	// Load launcher UI config
	a.loadConfig()
}

// loadConfig reads launcher preferences from ~/.zoro/launcher.toml
func (a *App) loadConfig() {
	home, err := os.UserHomeDir()
	if err != nil {
		return // silently ignore, use defaults
	}
	cfgPath := filepath.Join(home, ".zoro", "launcher.toml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return // file doesn't exist yet, use defaults
	}

	// Parse manually to avoid adding another dependency
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

	// Validate theme value
	switch a.config.Theme {
	case "light-glass", "dark-glass", "minimal":
		// valid
	default:
		a.config.Theme = "light-glass"
	}
}

// Status returns a short human-readable line about the loaded workspace.
func (a *App) Status() string {
	if a.wsErr != nil {
		return fmt.Sprintf("读取工作区失败（%s）：%v", a.wsPath, a.wsErr)
	}
	if a.ws == nil {
		return fmt.Sprintf("未加载工作区（%s）", a.wsPath)
	}
	return fmt.Sprintf("%s · %d 个知识库", a.wsPath, len(a.ws.Libraries))
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
	for _, lib := range a.ws.Libraries {
		if lib.Name == library {
			return filepath.Join(lib.Root, filepath.FromSlash(path)), nil
		}
	}
	return "", fmt.Errorf("library not found: %s", library)
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

// PopoutResult renders a candidate's content to HTML and opens it in a temporary
// browser window for side-by-side reference. This is a workaround for Wails v2's
// lack of native multi-window support.
func (a *App) PopoutResult(library, path string, start int) error {
	raw, err := a.LoadRaw(library, path, start)
	if err != nil {
		return err
	}

	html, err := a.RenderHTML(raw)
	if err != nil {
		return err
	}

	// Write to temp file and open in browser
	tmpDir, err := os.MkdirTemp("", "zoro-popout-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}

	tmpFile := filepath.Join(tmpDir, "result.html")
	fullHTML := fmt.Sprintf(`<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>zoro - Result</title>
  <style>
    body {
      margin: 0;
      padding: 20px;
      font-family: system-ui, -apple-system, sans-serif;
      line-height: 1.6;
      color: #1c1e24;
      background: #fff;
    }
    .content { max-width: 800px; margin: 0 auto; }
    pre { background: #f5f6f8; padding: 12px; border-radius: 6px; overflow: auto; }
    code { font-family: "SF Mono", Monaco, monospace; }
    a { color: #3157d1; }
  </style>
</head>
<body>
  <div class="content">%s</div>
</body>
</html>`, html)

	if err := os.WriteFile(tmpFile, []byte(fullHTML), 0644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	// Open in default browser (stays on screen independently)
	return launchFile(tmpFile)
}

// Copy puts text on the system clipboard (Copy action; no special permission).
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

// Hide hides the launcher window while the process keeps running.
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
	// 保留项目级工作区习惯：cwd 存在 zoro.toml 时使用它；否则落到全局默认
	// ~/.zoro/zoro.toml（由 startup 自动创建）。
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
