# zoro-launcher UI Enhancements

This document describes the new UI features added to zoro-launcher.

## 1. Configurable Theme System

The launcher now supports three visual themes that can be configured via `~/.zoro/launcher.toml`:

### Available Themes

- **light-glass** (default): Light translucent glassmorphism with white backgrounds
- **dark-glass**: Dark translucent glassmorphism with dark backgrounds and light text
- **minimal**: Flat design without blur effects, solid backgrounds

### Configuration

Create or edit `~/.zoro/launcher.toml`:

```toml
# zoro-launcher UI configuration
# theme: "light-glass" (default) | "dark-glass" | "minimal"
theme = "dark-glass"
```

The theme is applied on startup and persists across sessions.

### Implementation Details

Themes are implemented using CSS custom properties (variables) scoped to `[data-theme]` attributes. The frontend requests the theme from the Go backend via `GetTheme()` bridge method on initialization.

## 2. Popout Result Window

When you press **Enter** on a selected result, the content opens in your default browser as an independent HTML window. This allows you to:

- Keep search results visible while working in other applications
- Have multiple results open simultaneously for comparison
- Use browser features like zoom, print, or copy-paste

### How It Works

Since Wails v2 doesn't support native multi-window, `PopoutResult()` renders the Markdown to HTML, writes it to a temporary file, and opens it in your system's default browser. The temp file persists until manually cleaned up.

### Keyboard Shortcut

- **Enter**: Popout selected result in browser
- **⌘+Enter** (or **Ctrl+Enter**): Open source Markdown file in default editor
- **⌘+C** (or **Ctrl+C**): Copy raw Markdown to clipboard
- **Esc**: Hide launcher window

## 3. Search Result Summary

A summary line appears below the search box showing the number of matching results:

```
搜索框
5 条命中
─────────────
结果列表...
```

The summary automatically hides when there are no results or the query is empty, keeping the interface clean.

## 4. Updated Keyboard Shortcuts

| Key Combination | Action |
|----------------|--------|
| **Enter** | Popout result in browser |
| **⌘+Enter** / **Ctrl+Enter** | Open source file |
| **⌘+C** / **Ctrl+C** | Copy to clipboard |
| **↑** / **↓** | Navigate results |
| **Esc** | Hide launcher |

## File Changes

### Backend (Go)

- `app.go`: Added `LauncherConfig`, `loadConfig()`, `GetTheme()`, `PopoutResult()` methods
- Created `~/.zoro/launcher.toml` for UI preferences

### Frontend (HTML/CSS/JS)

- `index.html`: Added `<div id="summary">` element, updated status bar hints
- `styles.css`: Added theme presets (`[data-theme]`), `.summary` styles, `--bg-blur` variable
- `app.js`: Added theme loading, `popoutActive()`, `setSummary()`, updated keyboard handlers

## Future Enhancements

Potential improvements for future iterations:

1. **True multi-window**: Upgrade to Wails v3 when stable for native secondary windows
2. **Theme customization**: Allow users to customize colors, blur intensity, fonts
3. **Window management**: Track opened popout windows, limit concurrent windows
4. **Persistent popouts**: Save popout content to user-selected location instead of temp files
5. **Rich preview**: Add syntax highlighting, mermaid diagrams, math rendering to popout view
