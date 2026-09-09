package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// 这个文件解决一个问题：**程序化修改用户手写的 zoro.toml，且除了新增的那几行，
// 其它字节一个都不动。**
//
// 为什么不用现成的 ParseWorkspaceConfig → 改 → MarshalWorkspaceConfig：
//   - Marshal 走 toml.Marshal，**注释全部丢失**；
//   - workspaceConfigOut 只有 default / data_dir / libraries 三个顶层键，
//     用户手写的其它顶层键会被静默删除。
// zoro.toml 是用户会手写、会加注释的文件，所以「只增不改」的操作一律走下面的
// 外科式文本编辑；每个函数写完都会**重新解析并逐字段自校验**，不符合预期就放弃。
//
// ⚠️ 需要携带 LibraryConfig 的写入仍然走 WriteWorkspaceConfig（整文件重写），
//    因为在这里手写任意 TOML 值等于重新实现一遍编码器。

var reTopLevelDefault = regexp.MustCompile(`^default\s*=`)

// AppendLibrarySpec adds one `[[libraries]]` stanza (name + root) to existing
// `zoro.toml` text. Comments, unknown keys, ordering and the other libraries are
// preserved byte-for-byte; the only change is the appended stanza (plus a
// trailing newline when the input lacked one).
//
// root must already be absolute and clean — callers resolve relative paths
// against the directory of zoro.toml, exactly like `zoro add` does.
func AppendLibrarySpec(text, name, root string) (string, error) {
	if err := ValidateLibraryName(name); err != nil {
		return "", err
	}
	if root == "" {
		return "", errors.New("库根目录不能为空")
	}
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("库根目录必须是绝对路径（相对路径请先按 zoro.toml 所在目录解析）: %q", root)
	}
	root = filepath.Clean(root)

	before, err := ParseWorkspaceConfig(text)
	if err != nil {
		return "", fmt.Errorf("现有 zoro.toml 无法解析，拒绝做增量修改: %w", err)
	}
	for _, lib := range before.Libraries {
		if lib.Name == name {
			return "", fmt.Errorf("库名已存在: %s", name)
		}
	}

	qName, err := quoteTomlString(name)
	if err != nil {
		return "", err
	}
	qRoot, err := quoteTomlString(root)
	if err != nil {
		return "", err
	}

	out := text
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	if strings.TrimSpace(out) != "" {
		out += "\n"
	}
	out += "[[libraries]]\nname = " + qName + "\nroot = " + qRoot + "\n"

	after, err := ParseWorkspaceConfig(out)
	if err != nil {
		return "", fmt.Errorf("追加后的 zoro.toml 解析失败，已放弃: %w", err)
	}
	if err := checkUnchanged(before, after); err != nil {
		return "", err
	}
	if len(after.Libraries) != len(before.Libraries)+1 {
		return "", fmt.Errorf("追加后库数量 = %d，期望 %d，已放弃", len(after.Libraries), len(before.Libraries)+1)
	}
	last := after.Libraries[len(after.Libraries)-1]
	if last.Name != name || last.Root != root {
		return "", fmt.Errorf("追加结果与预期不符（name=%q root=%q），已放弃", last.Name, last.Root)
	}
	return out, nil
}

// SetDefaultLibrary rewrites the top-level `default` key in existing `zoro.toml`
// text, or inserts it before the first table header when absent. A trailing
// comment on the rewritten line is kept.
func SetDefaultLibrary(text, name string) (string, error) {
	if err := ValidateLibraryName(name); err != nil {
		return "", err
	}
	quoted, err := quoteTomlString(name)
	if err != nil {
		return "", err
	}

	before, err := ParseWorkspaceConfig(text)
	if err != nil {
		return "", fmt.Errorf("现有 zoro.toml 无法解析，拒绝做增量修改: %w", err)
	}

	lines := strings.SplitAfter(text, "\n") // 保留行尾换行符
	defaultIdx, firstTable := -1, len(lines)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			firstTable = i
			break
		}
		if defaultIdx < 0 && reTopLevelDefault.MatchString(trimmed) {
			defaultIdx = i
		}
	}

	if defaultIdx >= 0 {
		lines[defaultIdx] = "default = " + quoted + trailingComment(lines[defaultIdx]) + "\n"
	} else {
		// 插到第一个表头之前：这样文件顶部的注释块仍然在最上面。
		insert := "default = " + quoted + "\n"
		if prev := firstTable - 1; prev >= 0 && lines[prev] != "" && !strings.HasSuffix(lines[prev], "\n") {
			lines[prev] += "\n"
		}
		lines = append(lines[:firstTable], append([]string{insert}, lines[firstTable:]...)...)
	}
	out := strings.Join(lines, "")

	after, err := ParseWorkspaceConfig(out)
	if err != nil {
		return "", fmt.Errorf("修改 default 后的 zoro.toml 解析失败，已放弃: %w", err)
	}
	if after.Default != name {
		return "", fmt.Errorf("default 写入后读到 %q，期望 %q，已放弃", after.Default, name)
	}
	if err := checkStableParts(before, after); err != nil {
		return "", err
	}
	if len(after.Libraries) != len(before.Libraries) {
		return "", fmt.Errorf("修改 default 改变了库数量（%d → %d），已放弃", len(before.Libraries), len(after.Libraries))
	}
	return out, nil
}

// AddLibraryToWorkspace appends a library to the `zoro.toml` at path and writes
// it back atomically. A missing file is seeded from scratch (nothing to lose).
//
// 这是 CLI `zoro add` 与 Launcher `/lib add` 共用的唯一实现。
// 它只管声明文件；创建库目录是调用方的事（两边给用户的提示不同）。
func AddLibraryToWorkspace(path, name, root string, setDefault bool) error {
	text, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("读取工作区声明失败 %s: %w", path, err)
		}
		cfg := WorkspaceConfig{Libraries: []LibrarySpec{{Name: name, Root: root}}}
		if setDefault {
			cfg.Default = name
		}
		return WriteWorkspaceConfig(path, cfg)
	}

	out, err := AppendLibrarySpec(string(text), name, root)
	if err != nil {
		return err
	}
	if setDefault {
		if out, err = SetDefaultLibrary(out, name); err != nil {
			return err
		}
	}
	if err := atomicWriteFile(path, []byte(out)); err != nil {
		return fmt.Errorf("写回工作区失败 %s: %w", path, err)
	}
	return nil
}

// checkUnchanged asserts that nothing but the intended addition changed.
// 用于 AppendLibrarySpec：追加一个库不该动 default。
func checkUnchanged(before, after WorkspaceConfig) error {
	if before.Default != after.Default {
		return fmt.Errorf("default 从 %q 变成了 %q，已放弃", before.Default, after.Default)
	}
	return checkStableParts(before, after)
}

// checkStableParts asserts everything except `default` is untouched.
// SetDefaultLibrary 用它——改 default 正是它存在的意义。
func checkStableParts(before, after WorkspaceConfig) error {
	if before.DataDir != after.DataDir {
		return fmt.Errorf("data_dir 从 %q 变成了 %q，已放弃", before.DataDir, after.DataDir)
	}
	n := len(before.Libraries)
	if len(after.Libraries) < n {
		return fmt.Errorf("库数量从 %d 减少到 %d，已放弃", n, len(after.Libraries))
	}
	if !reflect.DeepEqual(after.Libraries[:n], before.Libraries) {
		return errors.New("已有库的声明被改动了，已放弃")
	}
	return nil
}

// ValidateLibraryName guards the one place a library name becomes a path
// segment (DataDir/<name>/zoro.db), so separators and traversal are refused
// while non-ASCII names (e.g. 中文库名) stay allowed.
//
// CLI `zoro add` 和 Launcher `/lib add` 共用它：库名的合法性只有一个定义处。
func ValidateLibraryName(name string) error {
	if name == "" {
		return errors.New("库名不能为空")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("库名不能是 %q", name)
	}
	if strings.ContainsAny(name, " \t\n\r") {
		return fmt.Errorf("库名不能包含空白字符: %q", name)
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("库名不能包含路径分隔符: %q", name)
	}
	for _, r := range name {
		// Windows 文件名非法字符；macOS/Linux 上合法但会阻碍跨平台同步。
		if strings.ContainsRune(`:*?"<>|`, r) {
			return fmt.Errorf("库名不能包含 %c: %q", r, name)
		}
		if !unicode.IsPrint(r) {
			return fmt.Errorf("库名不能包含不可打印字符: %q", name)
		}
	}
	return nil
}

// quoteTomlString renders s as a TOML basic string. strconv.Quote only emits
// escapes TOML understands as long as every rune is printable (control runes
// would produce \xNN / \a / \v, which TOML rejects), so that is the precondition.
func quoteTomlString(s string) (string, error) {
	for _, r := range s {
		if !unicode.IsPrint(r) {
			return "", fmt.Errorf("含不可打印字符，无法安全写入 TOML: %q", s)
		}
	}
	return strconv.Quote(s), nil
}

// trailingComment returns the trailing comment of a `key = "value"  # note` line,
// **including the whitespace that separates it** ("  # note"), so the caller can
// re-attach it verbatim: TOML requires at least one space before an inline `#`,
// and `"x"# note` is a syntax error. Returns "" when the line has no comment.
// The scan skips the quoted value so a '#' inside it is not mistaken for a comment.
func trailingComment(line string) string {
	body := strings.TrimRight(line, "\r\n")
	eq := strings.Index(body, "=")
	if eq < 0 {
		return ""
	}
	rest := body[eq+1:]
	if q := strings.IndexAny(rest, `"'`); q >= 0 {
		quote := rest[q : q+1]
		if end := strings.Index(rest[q+1:], quote); end >= 0 {
			rest = rest[q+1+end+1:]
		}
	}
	h := strings.Index(rest, "#")
	if h < 0 {
		return ""
	}
	cut := len(body) - len(rest) + h
	for cut > 0 && (body[cut-1] == ' ' || body[cut-1] == '\t') {
		cut--
	}
	return body[cut:]
}

// atomicWriteFile writes via a temp file in the same directory and renames it
// over the destination, keeping the destination's permission bits.
func atomicWriteFile(path string, data []byte) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
