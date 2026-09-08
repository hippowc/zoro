package main

import (
	"context"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"zoro/core"
)

// App is the Wails-bound bridge between the launcher UI and zoro core.
// It only exposes the Launcher v1 action surface: Query + Preview/Render + Copy.
type App struct {
	ctx context.Context
	ws  *core.Workspace
}

// NewApp creates the bound app.
func NewApp() *App { return &App{} }

// startup loads the zoro workspace (cold-start first, auto rebuild when stale).
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	ws, err := core.LoadWorkspace(launcherWorkspacePath())
	if err != nil {
		// Workspace errors surface on every query instead of crashing the launcher.
		a.ws = nil
		return
	}
	a.ws = ws
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
		return "", fmt.Errorf("workspace not loaded")
	}
	return a.ws.LoadRaw(library, path, start)
}

// RenderHTML renders Markdown to HTML for the preview pane.
func (a *App) RenderHTML(raw string) (string, error) {
	return core.RenderMarkdownHTML(raw)
}

// Copy puts text on the system clipboard (Copy action; no special permission).
func (a *App) Copy(text string) error {
	if a.ctx == nil {
		return fmt.Errorf("app not started")
	}
	return runtime.ClipboardSetText(a.ctx, text)
}

func launcherWorkspacePath() string {
	if p := os.Getenv("ZORO_WORKSPACE"); p != "" {
		return p
	}
	return "zoro.toml"
}
