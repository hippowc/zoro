package main

import (
	"context"
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"zoro-launcher/hotkey"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title: "zoro",
		Width: 780,
		// 初值 = 「只有搜索框」时的高度；首帧就会被前端 ResizeObserver 纠正。
		// 用小初值而不是 580，是为了让 JS 万一没跑起来时，窗口是一个
		// 小小的搜索框，而不是一整块透明空框。
		Height:   76,
		MinWidth: 560,
		// MinHeight 必须 <= 前端「仅搜索框」时的内容高度（约 76px），否则
		// NSWindow 的 userMinSize 会把 window.runtime.WindowSetSize 的结果
		// 夹回 380，窗口再也收不下去。
		// 该值必须与 frontend/dist/app.js 的 MIN_WINDOW_HEIGHT 保持一致，
		// 且只能在这里改：从 JS 调 WindowSetMinSize 会触发 darwin 的
		// adjustWindowSize()，它不重新锚定顶边，窗口会 visibly 跳一下。
		MinHeight:         60,
		Frameless:         true,
		AlwaysOnTop:       true,
		HideWindowOnClose: true,
		// Background alpha 0 keeps the translucent/rounded shell in full control.
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 0},
		AssetServer:      &assetserver.Options{Assets: assets},
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarHiddenInset(),
			Appearance:           mac.NSAppearanceNameAqua,
			WebviewIsTransparent: true,
			// WindowIsTranslucent 会让 macOS 在 WebView 后插入 NSVisualEffectView，
			// 在某些版本上渲染成一块不透明的深色背景；关闭后配合透明 BackgroundColour
			// 才能得到真正透明的窗口，玻璃质感由前端 CSS 控制。
			WindowIsTranslucent: false,
		},
		OnStartup:  app.startup,
		OnDomReady: app.domReady,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		println("zoro-launcher error:", err.Error())
	}
}

// domReady registers the global summon hotkey and starts the toggle loop.
func (a *App) domReady(ctx context.Context) {
	a.ctx = ctx

	events, unregister, err := hotkey.Listen()
	if err != nil {
		runtime.LogWarningf(ctx, "global hotkey disabled: %v", err)
		return
	}
	a.unregisterHotkey = unregister

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-events:
				a.toggle()
			}
		}
	}()
}

// shutdown unregisters the hotkey when the process exits.
func (a *App) shutdown(ctx context.Context) {
	if a.unregisterHotkey != nil {
		a.unregisterHotkey()
	}
	if a.popoutManager != nil {
		a.popoutManager.CloseAll()
	}
}

// toggle alternates the frameless summon window between visible and hidden.
func (a *App) toggle() {
	if a.ctx == nil {
		return
	}
	if a.windowVisible {
		runtime.WindowHide(a.ctx)
		a.windowVisible = false
		return
	}
	a.windowVisible = true
	runtime.WindowShow(a.ctx)
	runtime.WindowUnminimise(a.ctx)
}
