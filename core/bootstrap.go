package core

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Default workspace layout: the launcher (and CLI when no local zoro.toml
// exists) keeps every config/index file under ~/.zoro so users never have to
// create a workspace by hand.
const (
	defaultPackageDirName = ".zoro"
	defaultWorkspaceName  = "zoro.toml"
	defaultLibraryName    = "default"
	defaultLibraryDirName = "kb"
	defaultIndexDirName   = "index"
	defaultKBNoteName     = "默认知识库.md"
)

// DefaultConfigDir returns the global config directory (~/.zoro).
func DefaultConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, defaultPackageDirName), nil
}

// DefaultWorkspacePath returns the global workspace file (~/.zoro/zoro.toml).
func DefaultWorkspacePath() (string, error) {
	dir, err := DefaultConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, defaultWorkspaceName), nil
}

// DefaultLibraryRoot returns the bundled knowledge base root (~/.zoro/kb).
func DefaultLibraryRoot() (string, error) {
	dir, err := DefaultConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, defaultLibraryDirName), nil
}

// DefaultIndexDir returns the directory where per-library derived files live
// for the bundled workspace (~/.zoro/index).
func DefaultIndexDir() (string, error) {
	dir, err := DefaultConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, defaultIndexDirName), nil
}

// EnsureDefaultWorkspace idempotently creates the global workspace:
//
//   - config dir     ~/.zoro
//   - workspace      ~/.zoro/zoro.toml (only when missing; never overwrites)
//   - default KB     ~/.zoro/kb with one starter note (only when it has no .md)
//   - index dir      ~/.zoro/index
//
// It returns the absolute workspace path. Library roots are always written as
// absolute paths, and the index dir is explicitly recorded so manifest + TSV
// stay inside the config directory instead of polluting the content root.
func EnsureDefaultWorkspace() (string, error) {
	cfgDir, err := DefaultConfigDir()
	if err != nil {
		return "", err
	}
	wsPath := filepath.Join(cfgDir, defaultWorkspaceName)
	kbRoot := filepath.Join(cfgDir, defaultLibraryDirName)
	indexDir := filepath.Join(cfgDir, defaultIndexDirName)

	for _, dir := range []string{cfgDir, kbRoot, indexDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", fmt.Errorf("创建配置目录 %s: %w", dir, err)
		}
	}
	if err := ensureDefaultKBFile(kbRoot); err != nil {
		return "", err
	}

	if _, err := os.Stat(wsPath); err == nil {
		return wsPath, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("检查工作区文件 %s: %w", wsPath, err)
	}

	cfg := WorkspaceConfig{
		Default: defaultLibraryName,
		DataDir: indexDir,
		Libraries: []LibrarySpec{{
			Name: defaultLibraryName,
			Root: kbRoot,
		}},
	}
	if err := WriteWorkspaceConfig(wsPath, cfg); err != nil {
		return "", fmt.Errorf("写入默认工作区 %s: %w", wsPath, err)
	}
	return wsPath, nil
}

func ensureDefaultKBFile(kbRoot string) error {
	hasMarkdown := false
	err := filepath.WalkDir(kbRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(d.Name())) {
		case ".md", ".mdx":
			hasMarkdown = true
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("扫描默认知识库 %s: %w", kbRoot, err)
	}
	if hasMarkdown {
		return nil
	}

	note := "# 默认知识库\n\n这是 zoro 自动创建的默认知识库，可以先当作临时保存与默认搜索的地方。\n\n@index 临时 待办 默认 笔记\n\n- 临时想法\n- 待办事项\n- 默认笔记\n"
	return os.WriteFile(filepath.Join(kbRoot, defaultKBNoteName), []byte(note), 0o644)
}
