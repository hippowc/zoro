package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// 这个文件是 zoro 的**内容写入层**。在此之前 core 只读不写（Block.Raw 明确是只读
// 运行时字段），所有 .md 都由用户的编辑器产生。这里开出块级增删改，让前端能做
// 「精准的增删改查」。
//
// 三条律（违反任何一条都会破坏用户的内容，照抄，不要自行发挥）：
//
//	L1 块身份 = (Library, Path, Start)，而 **Start 是行号 → 任何写操作都会让它漂移**。
//	   删/插一个块之后，同文件里后续块的 Start 全部失效，而前端手里还握着旧的候选列表。
//	   所以：① 写前必须重新定位 + 校验（expect 参数）；② 写成功后前端必须整体重新查询，
//	   **禁止局部 patch** 手里的候选列表。
//	L2 「用户看到的」==「被改 / 删的」。LoadRaw / Block.Raw 的范围是 blockEnd 定的
//	   「@ 行 → 下一个 @ 行 / EOF」**再剔除标题行**；改 / 删的范围由同一批函数算出
//	   （payloadEnd / ownHeadingStart），落盘前再用 expect 自校验一次，对不上就拒绝写。
//	     · UpdateBlock 只替换 [@ 行, 负载末尾)：不碰块自己上方的 `## 标题`（块身份从 @ 行开始）。
//	     · DeleteBlock 连块自己的 `## 标题` 一起摘掉（只留标题会变成孤儿标题），
//	       但**绝不**碰下一个块的 `## 标题`——它在 expect 里看不见，删了就是静默丢内容。
//	   ⚠️ 尾部空行与下一个块的标题都不在 expect 里，因此它们永远不会被改写。
//	L3 追加用 O_APPEND 单次写，**绝不 tmp+rename**（换 inode、丢权限、破坏用户的 git
//	   假设、把编辑器里打开的文件变成孤儿）。改/删是原地重写，失败时用刚读到的旧字节回滚。
//
// 每个写操作在落盘后都会**读回来逐字节自校验**，与预期不符就回滚 —— 行号/偏移算错的
// 后果是前端拿着一个指向别处的三元组，比写失败更糟（同 core/configedit.go 的纪律）。

// ErrBlockChanged 是乐观并发校验失败：文件在调用方上次读到它之后被改动过。
// 绝不盲写行号区间；调用方应重新查询后再试。
var ErrBlockChanged = errors.New("内容已变化，请重新搜索后再试")

// WriteResult 是一次写操作的结果，含**写入后**的块身份。
type WriteResult struct {
	Library string
	// Path 是库内相对 slash 路径（可直接回传给 LoadRaw）。
	Path string
	// Start 是该块 @ 行的 1-based 行号；删除时是被删块原来的行号。
	Start int
	// Analyzed 报告写后索引刷新是否成功。内容已落盘但刷新失败（例如 store 正被
	// CLI 占用，见 ErrStoreLocked）时它是 false：**这是可降级失败，不算写失败**，
	// 但 UI 必须把 RefreshErr 告诉用户，否则用户搜不到自己刚写的东西还以为是 bug。
	Analyzed bool
	// RefreshErr 是刷新失败的原因（Analyzed=false 时非 nil）。
	// json:"-" —— Wails 会把 bridge 返回值序列化成 JSON，error 接口会变成一个空对象。
	RefreshErr error `json:"-"`
}

// BlockDraft 是 AppendBlock 的输入：一个新块的全部内容。
type BlockDraft struct {
	// Path 是库内相对路径；空 → 库配置 capture_file，兜底 "inbox.md"。
	Path string
	// Kind 空 → KindIndex。未登记的 kind 也能写（TagKind 是开放字符串，会被存成
	// unknown:<name> 并照常检索）。
	Kind TagKind
	// Title 写成块上方的 `## <title>` 行；空 → 不写标题行（扫描器会用 terms[0] 兜底）。
	Title string
	// Terms 是 @ 行上的搜索词：至少一个，且每个都不能含空白。
	Terms []string
	// Body 是负载正文，可多行；不能含标签行（会多出块）、`## ` 标题行（扫描器会把它
	// 从 Raw 里剔除，于是这个块再也改不动）或以 @ 开头的行。见 validateBody。
	Body string
}

// BlockPatch 是 UpdateBlock 的输入：只改 @ 行上的搜索词与负载正文。
// 标题在块范围之外的 `## ` 行上（L2），改标题请用编辑器，或删了重加。
type BlockPatch struct {
	Terms []string
	Body  string
}

// CaptureFile 返回追加新块时的默认落盘文件（库内相对路径）。
// 取自库配置 capture_file（zoro.toml 的 [libraries.config]，未知键由 Extra 兜住），
// 兜底 "inbox.md"。
func (l *Library) CaptureFile() string {
	if v, ok := l.Config.Extra["capture_file"].(string); ok {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return "inbox.md"
}

// AppendBlock 在库内追加一个新块，**永不改写既有字节**（append-only）。
// 改 / 删是独立 API，且必须带 expect（调用方上次读到的原文）。
func (l *Library) AppendBlock(d BlockDraft) (WriteResult, error) {
	rel := strings.TrimSpace(d.Path)
	if rel == "" {
		rel = l.CaptureFile()
	}
	abs, rel, err := resolveContentPath(l.Root, rel)
	if err != nil {
		return WriteResult{}, err
	}
	name, err := normalizeTagName(d.Kind)
	if err != nil {
		return WriteResult{}, err
	}
	terms, err := validateTerms(d.Terms)
	if err != nil {
		return WriteResult{}, err
	}
	title, err := validateTitle(d.Title)
	if err != nil {
		return WriteResult{}, err
	}
	body, err := validateBody(d.Body)
	if err != nil {
		return WriteResult{}, err
	}

	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return WriteResult{}, fmt.Errorf("创建目录失败: %w", err)
	}
	old, err := os.ReadFile(abs)
	if err != nil && !os.IsNotExist(err) {
		return WriteResult{}, err
	}

	text := string(old)
	chunk, tagOffset := renderAppendChunk(text, name, title, terms, body)
	start := len(splitLines(text)) + tagOffset + 1

	// 追加：单次 write + 失败截断回滚（L3）。
	if err := appendBytes(abs, []byte(chunk), int64(len(old))); err != nil {
		return WriteResult{}, err
	}
	updated := text + chunk
	if err := verifyWritten(abs, text, updated); err != nil {
		return WriteResult{}, err
	}
	// 行号自校验：算错 Start 的后果是前端拿着一个指向别处的三元组，比写失败更糟，
	// 所以宁可把文件截回原长度。
	tagLine := "@" + name + " " + strings.Join(terms, " ")
	if got := SliceBlock(updated, start); !strings.HasPrefix(got, tagLine) {
		if rberr := os.Truncate(abs, int64(len(old))); rberr != nil {
			return WriteResult{}, fmt.Errorf("行号自校验失败（第 %d 行不是新块），且回滚也失败: %v", start, rberr)
		}
		return WriteResult{}, fmt.Errorf("行号自校验失败（第 %d 行不是新块），已回滚", start)
	}

	return l.afterWrite(WriteResult{Library: l.Name, Path: rel, Start: start}), nil
}

// UpdateBlock 原地替换一个块的 @ 行与负载正文，其余字节原样不动。
//
// expect 必须是调用方上次读到的块原文（LoadRaw 的返回值）：文件里的块与它不一致
// 就返回 ErrBlockChanged，绝不盲写行号区间。kind 从现有行读出后原样保留。
func (l *Library) UpdateBlock(path string, start int, expect string, patch BlockPatch) (WriteResult, error) {
	terms, err := validateTerms(patch.Terms)
	if err != nil {
		return WriteResult{}, err
	}
	body, err := validateBody(patch.Body)
	if err != nil {
		return WriteResult{}, err
	}
	return l.mutateBlock(path, start, expect, func(tagName string) ([]string, error) {
		return append([]string{"@" + tagName + " " + strings.Join(terms, " ")}, splitLines(body)...), nil
	})
}

// DeleteBlock 删除一个块（块自己的 `## 标题` + @ 行 + 负载，范围见 L2），其余字节原样不动。
// 与 UpdateBlock 同样强制 expect 前置校验。
//
// 删除是**不可逆**的：撤销只能靠用户自己的 git（知识库通常在 git 里）。
// 前端必须先用 PreviewDelete 取出将删的原文、二次确认后再调这里（见 kb/journal/2026-09-09 方案 §9-D2）。
func (l *Library) DeleteBlock(path string, start int, expect string) (WriteResult, error) {
	return l.mutateBlock(path, start, expect, nil)
}

// PreviewDelete 返回 DeleteBlock 将会删掉的行（含块自己的 `## 标题`，不含尾部空行）。
//
// 它与 DeleteBlock 走**同一套范围计算**（blockWriteRange），所以「给用户看的」永远等于「删掉的」。
// 只读，不校验 expect：调用时机在用户确认之前，此刻还没有「上次读到的原文」。
func (l *Library) PreviewDelete(path string, start int) (string, error) {
	abs, _, err := resolveContentPath(l.Root, path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	text := string(data)
	lines := splitLines(text)
	from := start - 1
	if from < 0 || from >= len(lines) {
		return "", fmt.Errorf("行号 %d 超出文件范围（共 %d 行）: %s", start, len(lines), path)
	}
	if _, _, ok := parseDirective(lines[from]); !ok {
		return "", fmt.Errorf("%w（第 %d 行不是标签行）", ErrBlockChanged, start)
	}
	head, end, err := blockWriteRange(lines, from)
	if err != nil {
		return "", err
	}
	return strings.Join(lines[head:end], dominantEOL(text)), nil
}

// mutateBlock 是改 / 删共用的骨架：定位 → 校验 → 拼接 → 原地写 → 自校验 → 刷索引。
// replacement 给出替换行；**nil 表示删除**，此时范围会连块自己的 `## 标题` 一起摘掉。
func (l *Library) mutateBlock(path string, start int, expect string, replacement func(tagName string) ([]string, error)) (WriteResult, error) {
	if strings.TrimSpace(expect) == "" {
		return WriteResult{}, errors.New("必须提供块的当前原文（LoadRaw 的返回值），以防盲写行号")
	}
	abs, rel, err := resolveContentPath(l.Root, path)
	if err != nil {
		return WriteResult{}, err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return WriteResult{}, err
	}
	text := string(data)
	lines := splitLines(text)

	from := start - 1
	if from < 0 || from >= len(lines) {
		return WriteResult{}, fmt.Errorf("行号 %d 超出文件范围（共 %d 行）: %s", start, len(lines), rel)
	}
	tagName, _, ok := parseDirective(lines[from])
	if !ok {
		return WriteResult{}, fmt.Errorf("%w（第 %d 行不是标签行）", ErrBlockChanged, start)
	}
	// expect 就是 LoadRaw 的返回值，所以这里必须用同一个函数比对（L2）。
	if got := SliceBlock(text, start); got != expect {
		return WriteResult{}, fmt.Errorf("%w（第 %d 行的块与调用方手里的副本不一致）", ErrBlockChanged, start)
	}

	deleting := replacement == nil
	// 改 / 删共用同一条负载末尾（end）；差别只在 head：
	// 删除连块自己的 `## 标题` 一起摘掉，改写不动它（L2）。
	head, end, err := blockWriteRange(lines, from)
	if err != nil {
		return WriteResult{}, err
	}
	if !deleting {
		head = from
	}
	// 范围自校验：[from,end) 必须正好是 expect 去掉尾部空行的部分。
	// 不成立 = payloadEnd 与 SliceBlock 对「块到哪里结束」的理解出现了分歧（本文件的 bug）。
	// 此时写下去会删掉用户在预览里**没看到**的内容，所以直接拒绝 —— 同 configedit.go 的纪律。
	if got := strings.Join(lines[from:end], "\n"); got != strings.TrimRight(expect, "\n") {
		return WriteResult{}, fmt.Errorf("内部错误：改写范围（%d 行）与预览不一致，已拒绝写入 %s:%d", end-from, rel, start)
	}

	var newLines []string
	if !deleting {
		newLines, err = replacement(tagName)
		if err != nil {
			return WriteResult{}, err
		}
	}
	out, err := spliceLines(text, head, end, newLines)
	if err != nil {
		return WriteResult{}, err
	}
	if err := rewriteInPlace(abs, text, out); err != nil {
		return WriteResult{}, err
	}
	if err := verifyWritten(abs, text, out); err != nil {
		return WriteResult{}, err
	}
	return l.afterWrite(WriteResult{Library: l.Name, Path: rel, Start: start}), nil
}

// payloadEnd 返回块**负载末尾**的排他行号：从 blockEnd 往回跳过尾部空行与标题行。
//
// ⚠️ 尾部那个 `## 标题` 属于**下一个**块（SliceBlock 剔除标题行，所以它在 expect 里
// 根本看不见），改写范围若含它就会静默删掉用户没看到的东西。
// 标题行夹在负载**中间**时返回错误：那同样在 expect 里不可见，无法安全改写，
// 只能请用户去编辑器里处理（宁可拒绝，不可猜）。
func payloadEnd(lines []string, from, to int) (int, error) {
	end := to
	for end > from+1 && (strings.TrimSpace(lines[end-1]) == "" || isHeadingLine(lines[end-1])) {
		end--
	}
	for i := from + 1; i < end; i++ {
		if isHeadingLine(lines[i]) {
			return 0, fmt.Errorf("第 %d 行的块中间夹着 ## 标题行（在预览里看不见），无法安全改写；请在编辑器里手动处理", from+1)
		}
	}
	return end, nil
}

// ownHeadingStart 返回块**自己的标题行**行号；没有则原样返回 from。
//
// 只有紧邻上方、中间**只隔空行**的标题才算 —— 与扫描器的 headingFresh 一致：
// 中间夹了一行正文，那个标题就不是这个块的标题（扫描器也不会拿它当 Title），删掉就是误伤。
// 只有 DeleteBlock 用它（见 L2）。
func ownHeadingStart(lines []string, from int) int {
	i := from - 1
	for i >= 0 && strings.TrimSpace(lines[i]) == "" {
		i--
	}
	if i >= 0 && isHeadingLine(lines[i]) {
		return i
	}
	return from
}

// blockWriteRange 是**改写范围的唯一定义处**：PreviewDelete（给用户看）、DeleteBlock（真删）
// 与 UpdateBlock（替换）共用，保证「看到的」==「动的」。
// 返回 head = 块自己的标题行（没有则是 @ 行），end = 负载末尾（不含尾部空行与下一块的标题）。
// UpdateBlock 只取 end，把 head 收回 @ 行 —— 块身份从 @ 行开始。
func blockWriteRange(lines []string, from int) (int, int, error) {
	end, err := payloadEnd(lines, from, blockEnd(lines, from))
	if err != nil {
		return 0, 0, err
	}
	return ownHeadingStart(lines, from), end, nil
}

// afterWrite 显式刷新索引（不要依赖隐式的指纹刷新：同 size+mtime 的改动不可见）。
// 刷新失败不是写失败：内容已经落盘，把原因放进 RefreshErr 让 UI 说出来。
func (l *Library) afterWrite(res WriteResult) WriteResult {
	if _, err := l.Analyze(false); err != nil {
		res.RefreshErr = err
		return res
	}
	res.Analyzed = true
	return res
}

/* ------------------------------------------------------------
   文本形状
   ------------------------------------------------------------ */

// renderAppendChunk 生成追加到 text 末尾的字节，并返回 @ 行在 chunk 内的 0-based 行偏移。
// 形状固定：
//
//	[分隔]## <title>\n\n@<kind> <terms…>\n<body>\n
//
// [分隔] 是与既有内容分开所需的最小换行：空文件 → 无；已以换行结尾 → 一个空行；
// 最后一行没有收尾换行 → 先收尾再空一行。
func renderAppendChunk(text, tagName, title string, terms []string, body string) (chunk string, tagOffset int) {
	eol := dominantEOL(text)
	prefix := ""
	switch {
	case text == "":
	case strings.HasSuffix(text, eol):
		prefix = eol
	default:
		prefix = eol + eol
	}

	var lines []string
	if title != "" {
		lines = append(lines, "## "+title, "")
	}
	tagIdx := len(lines)
	lines = append(lines, "@"+tagName+" "+strings.Join(terms, " "))
	lines = append(lines, splitLines(body)...)

	// prefixLines = prefix 在文件里**新增**的行数。原文件最后一行没有收尾换行时，
	// prefix 的第一个换行只是把它收尾，不算新行 —— 否则 Start 会指到负载行上去。
	prefixLines := strings.Count(prefix, "\n")
	if text != "" && !strings.HasSuffix(text, eol) {
		prefixLines--
	}
	return prefix + strings.Join(lines, eol) + eol, prefixLines + tagIdx
}

// dominantEOL 报告文件用的换行符。改一个块时，新写的行要跟着文件走 ——
// CRLF 的文件被塞进 LF 行会变成混合换行（用户 git diff 里一片噪音）。
func dominantEOL(text string) string {
	if strings.Contains(text, "\r\n") {
		return "\r\n"
	}
	return "\n"
}

// spliceLines 用 newLines 替换 text 中 [from,to) 行（0-based，to 不含），
// 其余字节**原样保留**：CRLF、行尾空格、文件末尾的空行都是用户的内容，不归我们管。
func spliceLines(text string, from, to int, newLines []string) (string, error) {
	sb, err := lineByteOffset(text, from)
	if err != nil {
		return "", err
	}
	eb, err := lineByteOffset(text, to)
	if err != nil {
		return "", err
	}
	repl := ""
	if len(newLines) > 0 {
		repl = strings.Join(newLines, dominantEOL(text)) + dominantEOL(text)
	}
	return text[:sb] + repl + text[eb:], nil
}

// lineByteOffset 返回第 line 行（0-based）第一个字节的偏移；line == 行数时返回 len(text)。
func lineByteOffset(text string, line int) (int, error) {
	n := len(splitLines(text))
	if line < 0 || line > n {
		return 0, fmt.Errorf("行号 %d 超出范围（共 %d 行）", line+1, n)
	}
	off, idx := 0, 0
	for idx < line {
		i := strings.IndexByte(text[off:], '\n')
		if i < 0 {
			break
		}
		off += i + 1
		idx++
	}
	if idx != line {
		return 0, fmt.Errorf("行号 %d 超出范围（共 %d 行）", line+1, n)
	}
	return off, nil
}

/* ------------------------------------------------------------
   校验（写进去的东西必须能被扫描器原样读回来，否则等于静默丢块）
   ------------------------------------------------------------ */

// normalizeTagName 把 TagKind 变成能写进 @ 行的标签名。
//
// ⚠️ KindUnknown("mindmap").String() 是 "unknown:mindmap"，直接写进文件会变成
// `@unknown:mindmap` —— ':' 不是合法标签字符，这一行**再也不会被扫描器识别**，
// 等于静默丢块。所以这里剥掉前缀，写回用户本来的名字。
func normalizeTagName(kind TagKind) (string, error) {
	name := strings.TrimPrefix(strings.TrimSpace(string(kind)), "unknown:")
	if name == "" {
		name = string(KindIndex)
	}
	for i := 0; i < len(name); i++ {
		if !isASCIIName(name[i]) {
			return "", fmt.Errorf("标签名只能是 ASCII 字母/数字/-/_（否则扫描器认不出这一行）: %q", name)
		}
	}
	return name, nil
}

// validateTerms 保证写出的 @ 行能被 strings.Fields 原样切回同样的词。
func validateTerms(terms []string) ([]string, error) {
	out := make([]string, 0, len(terms))
	for _, t := range terms {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if strings.ContainsAny(t, " \t\n\r") {
			return nil, fmt.Errorf("搜索词不能包含空白（会被拆成两个词）: %q", t)
		}
		out = append(out, t)
	}
	if len(out) == 0 {
		return nil, errors.New("至少要有一个搜索词，否则这个块永远搜不到")
	}
	return out, nil
}

func validateTitle(title string) (string, error) {
	t := strings.TrimSpace(strings.ReplaceAll(title, "\r\n", "\n"))
	if strings.ContainsAny(t, "\n\r") {
		return "", fmt.Errorf("标题不能换行: %q", title)
	}
	if strings.HasPrefix(t, "#") {
		return "", fmt.Errorf("标题不要带 markdown 井号（会自动写成 ## 行）: %q", t)
	}
	return t, nil
}

// validateBody 拒绝任何会被扫描器当成标签行、或被扫描器从正文里剔除的行：
// 前者会静默多出一个块，后者会让写进去的内容在预览 / Raw 里凭空消失
// （于是 expect 永远对不上，这个块再也改不动）。
func validateBody(body string) (string, error) {
	b := strings.ReplaceAll(body, "\r\n", "\n")
	b = strings.ReplaceAll(b, "\r", "\n")
	for _, line := range strings.Split(b, "\n") {
		if _, _, ok := parseDirective(line); ok {
			return "", fmt.Errorf("正文不能包含标签行 %q（会被当成一个新块的开始）", line)
		}
		if isDirectiveLine(trimLeftSpace(line)) {
			return "", fmt.Errorf("正文不能包含以 @ 开头的行 %q（预览会把它当成指令吃掉；缩进也一样）", line)
		}
		if isHeadingLine(line) {
			return "", fmt.Errorf("正文不能包含标题行 %q（## 行属于块的标题，扫描器不会把它放进正文）", line)
		}
	}
	return strings.Trim(b, "\n"), nil
}

/* ------------------------------------------------------------
   路径与落盘
   ------------------------------------------------------------ */

// resolveContentPath 把库内相对路径解析成绝对路径，返回 (绝对路径, 规范化的相对 slash 路径)。
//
// ⚠️ 这是安全边界：路径来自用户输入（Launcher 的捕获面板 / CLI 参数）。
// 绝对路径、`..` 逃逸、非 .md/.mdx 扩展名一律拒绝 —— 最后一条是因为扫描器只索引
// .md/.mdx，写进 notes.txt 的内容**永远搜不到**，那是最难查的静默失败。
func resolveContentPath(root, rel string) (string, string, error) {
	rel = strings.TrimSpace(filepath.ToSlash(rel))
	if rel == "" {
		return "", "", errors.New("路径不能为空")
	}
	if strings.HasPrefix(rel, "/") || filepath.IsAbs(rel) {
		return "", "", fmt.Errorf("路径必须是库内相对路径: %q", rel)
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("路径不能逃出库根: %q", rel)
	}
	if ext := strings.ToLower(filepath.Ext(clean)); ext != ".md" && ext != ".mdx" {
		return "", "", fmt.Errorf("只能写 .md / .mdx（其它扩展名不会被索引）: %q", rel)
	}
	rootClean := filepath.Clean(root)
	abs := filepath.Join(rootClean, clean)
	// 双重保险：Join 之后再确认结果仍在库根之内。
	if abs != rootClean && !strings.HasPrefix(abs, rootClean+string(filepath.Separator)) {
		return "", "", fmt.Errorf("路径不能逃出库根: %q", rel)
	}
	return abs, filepath.ToSlash(clean), nil
}

// appendBytes 用 O_APPEND 单次追加（L3）。写入前确认文件长度仍等于 expectSize ——
// 否则 Start 会算错（有别的进程也在写这个文件），此时拒绝写入而不是写歪。
func appendBytes(path string, chunk []byte, expectSize int64) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Size() != expectSize {
		return fmt.Errorf("%w（追加前文件长度从 %d 变成了 %d）", ErrBlockChanged, expectSize, info.Size())
	}
	if _, err := f.Write(chunk); err != nil {
		// 追加失败不留半个块：截回原长度。
		if terr := f.Truncate(expectSize); terr != nil {
			return fmt.Errorf("追加失败: %w（回滚也失败: %v）", err, terr)
		}
		return fmt.Errorf("追加失败: %w（已回滚）", err)
	}
	return nil
}

// rewriteInPlace 原地重写整个文件内容：不换 inode、不改权限（tmp+rename 两者都会破坏，
// 而且用户可能正在编辑器里开着这个文件）。失败时用刚读到的旧字节回滚。
func rewriteInPlace(path, old, updated string) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	if info.Size() != int64(len(old)) {
		f.Close()
		return fmt.Errorf("%w（文件在读取之后被改动了：长度 %d，期望 %d）", ErrBlockChanged, info.Size(), len(old))
	}

	werr := func() error {
		if _, err := f.Write([]byte(updated)); err != nil {
			return err
		}
		if int64(len(updated)) < info.Size() {
			return f.Truncate(int64(len(updated)))
		}
		return nil
	}()
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		if rberr := os.WriteFile(path, []byte(old), 0o600); rberr != nil {
			return fmt.Errorf("写入失败 %s: %w（回滚也失败: %v）", path, werr, rberr)
		}
		return fmt.Errorf("写入失败 %s: %w（已回滚到原内容）", path, werr)
	}
	return nil
}

// verifyWritten 读回文件并确认逐字节等于 want，否则用 old 回滚。
func verifyWritten(path, old, want string) error {
	got, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("写入后无法读回校验 %s: %w", path, err)
	}
	if string(got) == want {
		return nil
	}
	if rberr := os.WriteFile(path, []byte(old), 0o600); rberr != nil {
		return fmt.Errorf("写入后自校验失败，且回滚也失败 %s: %v", path, rberr)
	}
	return errors.New("写入后自校验失败（文件内容与预期不符），已回滚")
}

/* ------------------------------------------------------------
   Workspace 层：按库名分派
   ------------------------------------------------------------ */

// ⚠️ Workspace 只持有 Libraries，**不知道 zoro.toml 里的 default 是哪个库**，
// 所以 library 参数必须显式给出。解析 default 是调用方的事（Launcher 手里有 config，
// CLI 手里有 WorkspaceConfig）。

func (w *Workspace) withLibrary(name string, fn func(*Library) (WriteResult, error)) (WriteResult, error) {
	lib, ok := w.Library(name)
	if !ok {
		return WriteResult{}, fmt.Errorf("库不存在: %s（已声明的库: %s）", name, strings.Join(w.LibraryNames(), ", "))
	}
	return fn(lib)
}

// AppendBlock 在指定库里追加一个新块。d.Path 为空时落到该库的 CaptureFile()。
func (w *Workspace) AppendBlock(library string, d BlockDraft) (WriteResult, error) {
	return w.withLibrary(library, func(l *Library) (WriteResult, error) { return l.AppendBlock(d) })
}

// UpdateBlock 原地替换指定块的搜索词与正文（详见 Library.UpdateBlock）。
func (w *Workspace) UpdateBlock(library, path string, start int, expect string, patch BlockPatch) (WriteResult, error) {
	return w.withLibrary(library, func(l *Library) (WriteResult, error) {
		return l.UpdateBlock(path, start, expect, patch)
	})
}

// PreviewDelete 返回在指定库中删除该块将会删掉的原文（详见 Library.PreviewDelete）。
func (w *Workspace) PreviewDelete(library, path string, start int) (string, error) {
	lib, ok := w.Library(library)
	if !ok {
		return "", fmt.Errorf("库不存在: %s（已声明的库: %s）", library, strings.Join(w.LibraryNames(), ", "))
	}
	return lib.PreviewDelete(path, start)
}

// DeleteBlock 删除指定块（详见 Library.DeleteBlock）。
func (w *Workspace) DeleteBlock(library, path string, start int, expect string) (WriteResult, error) {
	return w.withLibrary(library, func(l *Library) (WriteResult, error) {
		return l.DeleteBlock(path, start, expect)
	})
}
