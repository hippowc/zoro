use std::fs;
use std::path::{Path, PathBuf};

use crate::model::{Block, ShellBlock, TagKind};
use crate::registry;

/// 扫描单个 Markdown 文件，按任意 `@<name>` 标签切分成内容块。
///
/// 返回块的 `library` 为空串，由 `scan_dir_named` / `Library::open` 填充。
pub fn scan_file(path: &Path) -> Result<Vec<Block>, ScanError> {
    let text = fs::read_to_string(path).map_err(ScanError::Io)?;
    Ok(scan_text(&text, path))
}

/// 对文本内容执行分块扫描（便于测试直接喂字符串）。
pub fn scan_text(text: &str, path: &Path) -> Vec<Block> {
    let mut scanner = Scanner::new(path);
    scanner.consume(text);
    scanner.blocks
}

/// 从 `start` 行开始，按统一边界现场截取一个块的原文。
///
/// 边界 = `start` 行 → 下一个任意 `@<name>` 标签行 / 文件末尾。
pub fn slice_block(text: &str, start: usize) -> String {
    let mut out: Vec<&str> = Vec::new();
    for (idx, line) in text.lines().enumerate() {
        let no = idx + 1;
        if no < start {
            continue;
        }
        if no > start && parse_directive(line).is_some() {
            break;
        }
        out.push(line);
    }
    out.join("\n")
}

#[derive(Debug)]
pub enum ScanError {
    Io(std::io::Error),
}

impl std::fmt::Display for ScanError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            ScanError::Io(e) => write!(f, "io error: {e}"),
        }
    }
}

impl std::error::Error for ScanError {}

struct Scanner<'a> {
    path: &'a Path,
    blocks: Vec<Block>,
    last_heading: Option<String>,
    /// `last_heading` 是否仍然"紧贴"下一个标签（中间只允许空行 / 其他 `@` 行）。
    heading_fresh: bool,
    cur: Option<CurBlock>,
    /// 是否在等待 `@shell` 的紧随 fence。
    pending_shell: bool,
}

#[derive(Default)]
struct CurBlock {
    start: usize,
    title: Option<String>,
    kind: TagKind,
    terms: Vec<String>,
    lines: Vec<String>,
    shell: Option<ShellBlock>,
}

impl<'a> Scanner<'a> {
    fn new(path: &'a Path) -> Self {
        Scanner {
            path,
            blocks: Vec::new(),
            last_heading: None,
            heading_fresh: false,
            cur: None,
            pending_shell: false,
        }
    }

    fn consume(&mut self, text: &str) {
        // 正在收集的 `@shell` fence 状态。
        let mut fence_kind: Option<char> = None;
        let mut shell_lang: Option<String> = None;
        let mut shell_lines: Vec<usize> = Vec::new();

        for (idx, line) in text.lines().enumerate() {
            let line_no = idx + 1;

            // 0) 正在收集 shell fence：fence 内不解析标签。
            if let Some(kind) = fence_kind {
                if is_fence_close(line, kind) {
                    if let Some(cur) = self.cur.as_mut() {
                        cur.lines.push(line.to_string());
                        cur.shell = Some(ShellBlock {
                            lang: shell_lang.take(),
                            lines: std::mem::take(&mut shell_lines),
                        });
                    }
                    self.pending_shell = false;
                    fence_kind = None;
                } else {
                    shell_lines.push(line_no);
                    if let Some(cur) = self.cur.as_mut() {
                        cur.lines.push(line.to_string());
                    }
                }
                continue;
            }

            // 1) 标题：只记录最近标题，不参与边界。
            if let Some(rest) = line.strip_prefix("## ") {
                self.last_heading = Some(rest.trim().to_string());
                self.heading_fresh = true;
                continue;
            }
            if line.trim() == "##" {
                self.last_heading = Some(String::new());
                self.heading_fresh = true;
                continue;
            }

            // 2) 任意 `@<name>` 标签行：结束上一块，开新块。
            if let Some((name, value)) = parse_directive(line) {
                self.flush_current();
                let kind = registry::classify_tag(&name);
                let title = if self.heading_fresh {
                    self.last_heading.clone().filter(|h| !h.is_empty())
                } else {
                    None
                };
                self.pending_shell = kind.is_shell();
                self.cur = Some(CurBlock {
                    start: line_no,
                    title,
                    kind,
                    terms: split_ws(&value),
                    lines: vec![line.to_string()],
                    shell: None,
                });
                // `@` 行不破坏标题紧贴性。
                continue;
            }

            // 3) `@shell` 之后：开始收集紧随的 fence。
            if self.pending_shell {
                if let Some(kind) = fence_kind_of(line) {
                    fence_kind = Some(kind);
                    shell_lang = fence_lang(line);
                    shell_lines.clear();
                    if let Some(cur) = self.cur.as_mut() {
                        cur.lines.push(line.to_string());
                    }
                    // fence 正文会破坏标题紧贴性。
                    self.heading_fresh = false;
                    continue;
                }
            }

            // 4) 其余：正文。
            if !line.trim().is_empty() {
                self.heading_fresh = false;
            }
            if let Some(cur) = self.cur.as_mut() {
                cur.lines.push(line.to_string());
            }
        }

        // 文件结尾：未闭合的 shell fence 也照常绑定。
        if self.pending_shell {
            if let Some(cur) = self.cur.as_mut() {
                if cur.shell.is_none() && !shell_lines.is_empty() {
                    cur.shell = Some(ShellBlock {
                        lang: shell_lang.take(),
                        lines: std::mem::take(&mut shell_lines),
                    });
                }
            }
        }

        self.flush_current();
    }

    fn flush_current(&mut self) {
        if let Some(mut cur) = self.cur.take() {
            let title = cur
                .title
                .clone()
                .filter(|h| !h.is_empty())
                .unwrap_or_else(|| {
                    cur.terms
                        .first()
                        .cloned()
                        .unwrap_or_else(|| cur.kind.as_str())
                });
            self.blocks.push(Block {
                library: String::new(),
                title,
                kind: cur.kind,
                terms: std::mem::take(&mut cur.terms),
                path: self.path.to_path_buf(),
                start: cur.start,
                raw: cur.lines.join("\n"),
                shell: cur.shell,
            });
        }
    }
}

/// 解析顶格 `@<name> value`，返回 `(name, value)`；不匹配返回 None。
fn parse_directive(line: &str) -> Option<(String, String)> {
    let rest = line.strip_prefix('@')?;
    let name: String = rest
        .chars()
        .take_while(|c| c.is_ascii_alphanumeric() || *c == '-' || *c == '_')
        .collect();
    if name.is_empty() {
        return None;
    }
    let after = &rest[name.len()..];
    if after.is_empty() || after.starts_with(' ') || after.starts_with('\t') {
        Some((name, after.trim().to_string()))
    } else {
        None
    }
}

fn split_ws(s: &str) -> Vec<String> {
    s.split_whitespace().map(|t| t.to_string()).collect()
}

/// fence 起始判定，返回定界符：'`' 或 '~'。
fn fence_kind_of(line: &str) -> Option<char> {
    let t = line.trim_start();
    if t.starts_with("```") {
        Some('`')
    } else if t.starts_with("~~~") {
        Some('~')
    } else {
        None
    }
}

fn fence_lang(line: &str) -> Option<String> {
    let t = line.trim_start();
    let rest = t.strip_prefix("```").or_else(|| t.strip_prefix("~~~"))?;
    let lang = rest.trim().split_whitespace().next().unwrap_or("");
    if lang.is_empty() {
        None
    } else {
        Some(lang.to_string())
    }
}

fn is_fence_close(line: &str, kind: char) -> bool {
    let t = line.trim_start();
    match kind {
        '`' => t.starts_with("```"),
        '~' => t.starts_with("~~~"),
        _ => false,
    }
}

/// 扫描一个内容根目录下所有 `*.md` / `*.mdx`，返回按路径稳定排序的块。
pub fn scan_dir(root: &Path) -> Result<Vec<Block>, ScanError> {
    scan_dir_inner(root, "")
}

/// 同 [`scan_dir`]，但把每个块的 `library` 置为给定库名。
pub fn scan_dir_named(root: &Path, library: &str) -> Result<Vec<Block>, ScanError> {
    scan_dir_inner(root, library)
}

fn scan_dir_inner(root: &Path, library: &str) -> Result<Vec<Block>, ScanError> {
    let mut files = Vec::new();
    collect_md(root, &mut files).map_err(ScanError::Io)?;
    files.sort();

    let mut blocks = Vec::new();
    for file in files {
        let rel = file.strip_prefix(root).unwrap_or(&file).to_path_buf();
        for mut b in scan_file(&file)? {
            b.path = rel.clone();
            b.library = library.to_string();
            blocks.push(b);
        }
    }
    Ok(blocks)
}

fn collect_md(root: &Path, out: &mut Vec<PathBuf>) -> std::io::Result<()> {
    if !root.is_dir() {
        return Ok(());
    }
    for entry in fs::read_dir(root)? {
        let entry = entry?;
        let path = entry.path();
        if entry.file_type()?.is_dir() {
            let name = entry.file_name();
            let name = name.to_string_lossy().to_string();
            if name.starts_with('.') {
                continue;
            }
            collect_md(&path, out)?;
        } else if let Some(ext) = path.extension().and_then(|e| e.to_str()) {
            if ext.eq_ignore_ascii_case("md") || ext.eq_ignore_ascii_case("mdx") {
                out.push(path);
            }
        }
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn splits_two_index_blocks() {
        let text = "\
## Head
@index alpha beta
body one
@index gamma
body two
";
        let blocks = scan_text(text, Path::new("a.md"));
        assert_eq!(blocks.len(), 2);
        assert_eq!(blocks[0].title, "Head");
        assert_eq!(blocks[0].terms, vec!["alpha", "beta"]);
        assert_eq!(blocks[0].start, 2);
        assert_eq!(blocks[1].title, "gamma");
        assert_eq!(blocks[1].start, 4);
    }

    #[test]
    fn shared_heading_when_adjacent() {
        let text = "\
## Head
@index alpha
body
@index beta
";
        let blocks = scan_text(text, Path::new("a.md"));
        assert_eq!(blocks.len(), 2);
        assert_eq!(blocks[0].title, "Head");
        // 第二个 @index 上方隔了正文，标题不再继承，退回搜索词首词
        assert_eq!(blocks[1].title, "beta");
    }

    #[test]
    fn binds_shell_fence() {
        let text = "\
@index run stuff
@shell
```bash
echo hi
echo bye
```
tail
";
        let blocks = scan_text(text, Path::new("a.md"));
        assert_eq!(blocks.len(), 2);
        let b = &blocks[1];
        assert_eq!(b.kind, TagKind::Shell);
        let shell = b.shell.as_ref().expect("shell payload");
        assert_eq!(shell.lang.as_deref(), Some("bash"));
        assert_eq!(shell.lines, vec![4, 5]);
        assert!(b.raw.contains("echo hi"));
    }

    #[test]
    fn any_tag_ends_previous_block() {
        let text = "\
@index alpha
index body
@shell git 丢弃
```bash
git checkout -- .
```
after
";
        let blocks = scan_text(text, Path::new("a.md"));
        assert_eq!(blocks.len(), 2);
        assert_eq!(blocks[0].kind, TagKind::Index);
        assert!(blocks[0].raw.contains("index body"));
        assert!(!blocks[0].raw.contains("@shell"));
        assert_eq!(blocks[1].kind, TagKind::Shell);
        assert_eq!(blocks[1].terms, vec!["git", "丢弃"]);
        assert!(blocks[1].raw.contains("after"));
    }

    #[test]
    fn unknown_tag_degrades_to_note() {
        let text = "\
@index alpha
body
@foo bar baz
payload
";
        let blocks = scan_text(text, Path::new("a.md"));
        assert_eq!(blocks.len(), 2);
        assert_eq!(blocks[1].kind, TagKind::Unknown("foo".to_string()));
        assert_eq!(blocks[1].terms, vec!["bar", "baz"]);
        assert!(blocks[1].raw.contains("payload"));
    }

    #[test]
    fn block_ends_at_eof() {
        let text = "@index only\nline one\nline two";
        let blocks = scan_text(text, Path::new("a.md"));
        assert_eq!(blocks.len(), 1);
        assert_eq!(blocks[0].raw.lines().count(), 3);
    }

    #[test]
    fn slice_block_matches_scanner_boundary() {
        let text = "\
## Head
@index alpha beta
line one
@shell git
```bash
echo hi
```
";
        // start = 2（@index alpha beta 行），到下一个标签（第 4 行）前结束
        assert_eq!(slice_block(text, 2), "@index alpha beta\nline one");
        // shell 块到 EOF
        let sliced = slice_block(text, 4);
        assert!(sliced.starts_with("@shell git"));
        assert!(sliced.ends_with("```"));
    }
}
