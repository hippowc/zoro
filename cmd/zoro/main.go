// Command zoro is the minimal CLI frontend of the zoro knowledge base.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"zoro/core"
)

const usage = `usage: zoro <command> [args]

commands:
  add <name> <root> [--default]  添加知识库（写入/创建 zoro.toml；首个库自动设为 default）
  search <query>                 搜索并打印候选（省略 query 浏览默认库或全部）
  preview <query>                取第一条命中，按库级 preview 配置渲染（默认 HTML）
  open <query> [--print]         打开第一条命中所在源文件（--print 仅打印路径）
  html <query>                   取第一条命中，渲染 Markdown → HTML
  text <query>                   取第一条命中，渲染为终端文本（TTY 彩色 ANSI，否则纯文本）
  index                          强制重建所有库的 manifest + zoro-index.tsv
  serve                          启动本地 Web（P2，尚未实现）

无子命令：若 zoro.toml 声明了 default，则浏览该库全部条目。
workspace: 由 ZORO_WORKSPACE 或 ./zoro.toml 声明
`

const addUsage = `usage: zoro add [--default] <name> <root>

  name        库名（须唯一）
  root        库根目录（相对路径相对 zoro.toml 所在目录解析）
  --default   将新库设为 default

首个库且当前未设置 default 时，自动设为 default。
`

// cli holds the resolved workspace plus its declaration, so commands can
// consult per-library config (e.g. default / preview).
type cli struct {
	cfg core.WorkspaceConfig
	ws  *core.Workspace
}

func main() {
	args := os.Args[1:]

	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(0)
	}

	// add 读写 zoro.toml，不要求工作区当前可打开（新 root 可能尚不存在）。
	if len(args) > 0 && args[0] == "add" {
		runAdd(args[1:])
		return
	}

	c := openCLI()

	// 无参：打开 default 库（浏览全部条目）。fzf 交互是 P3 目标，当前先落
	// 非交互降级：打印默认库全部候选。
	if len(args) == 0 {
		lib, err := c.defaultLibrary()
		if err != nil {
			fatalf("error: %v\n", err)
		}
		if lib == nil {
			fmt.Fprint(os.Stderr, usage)
			os.Exit(0)
		}
		printCandidates(lib.Query(""))
		return
	}

	switch args[0] {
	case "index":
		if err := c.ws.AnalyzeAll(true); err != nil {
			fatalf("error: %v\n", err)
		}
		for _, lib := range c.ws.Libraries {
			fmt.Printf("%s  %s  (%d blocks)\n", lib.MetaPath(), lib.Name, len(lib.Blocks))
		}

	case "search":
		q := strings.Join(args[1:], " ")
		if strings.TrimSpace(q) == "" {
			// 空查询 = 浏览：default 声明的库优先，否则聚合全部库。
			if lib, err := c.defaultLibrary(); err != nil {
				fatalf("error: %v\n", err)
			} else if lib != nil {
				printCandidates(lib.Query(""))
				return
			}
		}
		printCandidates(c.ws.Query(q))

	case "preview":
		q := requireQuery(args)
		cand, lib := c.pick(q)
		raw, err := c.ws.LoadRaw(cand.Library, cand.Path, cand.Start)
		if err != nil {
			fatalf("error: %v\n", err)
		}
		target := lib.PreviewTarget()
		if target == "" {
			target = core.RenderTargetHTML
		}
		out, err := core.RenderMarkdown(raw, target)
		if err != nil {
			fatalf("error: %v\n", err)
		}
		if target == core.RenderTargetHTML {
			fmt.Print(out)
		} else {
			fmt.Println(out)
		}

	case "open":
		runOpen(c, args[1:])

	case "html":
		q := requireQuery(args)
		cand, _ := c.pick(q)
		raw, err := c.ws.LoadRaw(cand.Library, cand.Path, cand.Start)
		if err != nil {
			fatalf("error: %v\n", err)
		}
		html, err := core.RenderMarkdownHTML(raw)
		if err != nil {
			fatalf("error: %v\n", err)
		}
		fmt.Print(html)

	case "text":
		q := requireQuery(args)
		cand, lib := c.pick(q)
		raw, err := c.ws.LoadRaw(cand.Library, cand.Path, cand.Start)
		if err != nil {
			fatalf("error: %v\n", err)
		}
		target := terminalTarget(lib, os.Stdout)
		out, err := core.RenderMarkdown(raw, target)
		if err != nil {
			fatalf("error: %v\n", err)
		}
		fmt.Println(out)

	case "serve":
		fmt.Fprintln(os.Stderr, "zoro serve: 本地 Web 前端（P2）尚未实现")
		os.Exit(1)

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n%s", args[0], usage)
		os.Exit(2)
	}
}

// runAdd adds one [[libraries]] entry and rewrites the workspace file.
// Existing relative roots are preserved: this path parses raw text rather than
// the resolved WorkspaceConfig used at runtime.
func runAdd(args []string) {
	setDefault := false
	var pos []string
	for _, a := range args {
		switch a {
		case "--default", "-d":
			setDefault = true
		case "-h", "--help":
			fmt.Fprint(os.Stderr, addUsage)
			os.Exit(0)
		default:
			if strings.HasPrefix(a, "-") {
				fatalf("error: unknown add flag: %s\n", a)
			}
			pos = append(pos, a)
		}
	}
	if len(pos) != 2 {
		fmt.Fprint(os.Stderr, addUsage)
		os.Exit(2)
	}
	name, rootArg := pos[0], pos[1]
	if strings.TrimSpace(name) == "" {
		fmt.Fprintln(os.Stderr, "error: library name must not be empty")
		os.Exit(2)
	}
	if name != strings.TrimSpace(name) || strings.ContainsAny(name, " \t") {
		fmt.Fprintln(os.Stderr, "error: library name must not contain whitespace (spaces/tabs)")
		os.Exit(2)
	}
	if strings.TrimSpace(rootArg) == "" {
		fmt.Fprintln(os.Stderr, "error: library root must not be empty")
		os.Exit(2)
	}

	wsPath := workspacePath()
	cfg := core.WorkspaceConfig{}
	if data, err := os.ReadFile(wsPath); err == nil {
		cfg, err = core.ParseWorkspaceConfig(string(data))
		if err != nil {
			fatalf("error: 读取工作区声明失败 %s: %v\n", wsPath, err)
		}
	} else if !os.IsNotExist(err) {
		fatalf("error: 读取工作区声明失败 %s: %v\n", wsPath, err)
	}

	for _, l := range cfg.Libraries {
		if l.Name == name {
			fatalf("error: library %q already exists\n", name)
		}
	}

	cfg.Libraries = append(cfg.Libraries, core.LibrarySpec{Name: name, Root: rootArg})
	if setDefault || (len(cfg.Libraries) == 1 && cfg.Default == "") {
		cfg.Default = name
	}

	// 新库根目录不存在则创建，保证随后 index / search 立即可用。
	rootAbs := rootArg
	if !filepath.IsAbs(rootAbs) {
		rootAbs = filepath.Join(filepath.Dir(wsPath), rootAbs)
	}
	if _, err := os.Stat(rootAbs); os.IsNotExist(err) {
		if err := os.MkdirAll(rootAbs, 0o755); err != nil {
			fatalf("error: 创建库目录失败 %s: %v\n", rootAbs, err)
		}
		fmt.Fprintf(os.Stderr, "created: %s\n", rootAbs)
	} else if err != nil {
		fatalf("error: 检查库目录失败 %s: %v\n", rootAbs, err)
	}

	if err := core.WriteWorkspaceConfig(wsPath, cfg); err != nil {
		fatalf("error: 写回工作区失败 %s: %v\n", wsPath, err)
	}

	fmt.Printf("added\t%s\t%s\n", name, rootArg)
	if cfg.Default == name {
		fmt.Printf("default\t%s\n", name)
	}
}

func runOpen(c *cli, args []string) {
	q, printOnly := parseOpenArgs(args)
	cand, lib := c.pick(q)
	abs := filepath.Join(lib.Root, filepath.FromSlash(cand.Path))
	fmt.Printf("%s:%d\n", abs, cand.Start)
	if printOnly {
		return
	}
	if err := launchFile(abs); err != nil {
		fatalf("error: %v\n", err)
	}
}

func parseOpenArgs(args []string) (string, bool) {
	var query []string
	printOnly := false
	for _, a := range args {
		switch a {
		case "--print", "-p":
			printOnly = true
		case "-h", "--help":
			fmt.Fprintln(os.Stderr, "usage: zoro open [--print] <query>")
			os.Exit(0)
		default:
			query = append(query, a)
		}
	}
	q := strings.Join(query, " ")
	if strings.TrimSpace(q) == "" {
		fmt.Fprintln(os.Stderr, "usage: zoro open [--print] <query>")
		os.Exit(2)
	}
	return q, printOnly
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

func openCLI() *cli {
	wsPath := workspacePath()
	cfg, err := core.WorkspaceConfigFromPath(wsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: 读取工作区声明失败 %s: %v\n", wsPath, err)
		os.Exit(2)
	}
	ws, err := core.BuildWorkspace(&cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: 打开工作区失败: %v\n", err)
		os.Exit(1)
	}
	return &cli{cfg: cfg, ws: ws}
}

// defaultLibrary resolves the configured default library; nil means no default.
func (c *cli) defaultLibrary() (*core.Library, error) {
	if c.cfg.Default == "" {
		return nil, nil
	}
	for _, lib := range c.ws.Libraries {
		if lib.Name == c.cfg.Default {
			return lib, nil
		}
	}
	return nil, fmt.Errorf("workspace default library %q is not declared", c.cfg.Default)
}

// pick returns the first hit and the library it belongs to.
func (c *cli) pick(q string) (core.Candidate, *core.Library) {
	cand, err := firstHit(c.ws, q)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
	for _, lib := range c.ws.Libraries {
		if lib.Name == cand.Library {
			return cand, lib
		}
	}
	fmt.Fprintf(os.Stderr, "error: library %q not found in workspace\n", cand.Library)
	os.Exit(1)
	return core.Candidate{}, nil // unreachable
}

// terminalTarget chooses how terminal text is produced: explicit plain-text
// preview wins, then a TTY gets ANSI color, otherwise plain text.
func terminalTarget(lib *core.Library, stdout *os.File) core.RenderTarget {
	if lib != nil && lib.PreviewTarget() == core.RenderTargetText {
		return core.RenderTargetText
	}
	if isTerminal(stdout) {
		return core.RenderTargetANSI
	}
	return core.RenderTargetText
}

// firstHit returns the top-ranked candidate or an error when no match.
func firstHit(ws *core.Workspace, q string) (core.Candidate, error) {
	hits := ws.Query(q)
	if len(hits) == 0 {
		return core.Candidate{}, fmt.Errorf("no match for query: %s", q)
	}
	return hits[0], nil
}

func requireQuery(args []string) string {
	q := strings.Join(args[1:], " ")
	if strings.TrimSpace(q) == "" {
		fmt.Fprintf(os.Stderr, "usage: zoro %s <query>\n", args[0])
		os.Exit(2)
	}
	return q
}

func printCandidates(hits []core.Candidate) {
	for i, c := range hits {
		if i == 20 {
			break
		}
		fmt.Printf("%s\t%s\t%s\t%d\t%s\n", c.Library, c.Title, c.Index, c.Start, c.Path)
	}
	if len(hits) == 0 {
		fmt.Fprintln(os.Stderr, "no match")
		os.Exit(1)
	}
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func fatalf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format, a...)
	os.Exit(1)
}

// workspacePath follows: ZORO_WORKSPACE -> ./zoro.toml.
func workspacePath() string {
	if p := os.Getenv("ZORO_WORKSPACE"); p != "" {
		return p
	}
	return "zoro.toml"
}
