use std::fs;
use std::path::{Path, PathBuf};

use crate::model::{Directive, Entry};

/// 扫描单个 Markdown 文件，按 `@index` 切分成多个条目。
pub fn scan_file(path: &Path) -> Result<Vec<Entry>, ScanError> {
    let text = fs::read_to_string(path).map_err(ScanError::Io)?;
    Ok(scan_text(&text, path))
}

/// 对文本内容执行分块扫描（便于测试直接喂字符串）。
pub fn scan_text(text: &str, path: &Path) -> Vec<Entry> {
    let mut scanner = Scanner::new(path);
    scanner.consume(text);
    scanner.entries
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
    entries: Vec<Entry>,
    last_heading: Option<String>,
    /// `last_heading` 是否仍然"紧贴"下一个 `@index`（中间只允许空行 / 其他 `@` 行）。
    heading_fresh: bool,
    cur: Option<CurEntry>,
    pending_cmd: bool,
}

#[derive(Default)]
struct CurEntry {
    start: usize,
    title: Option<String>,
    index_terms: Vec<String>,
    lines: Vec<String>,
    directives: Vec<Directive>,
}

impl<'a> Scanner<'a> {
    fn new(path: &'a Path) -> Self {
        Scanner {
            path,
            entries: Vec::new(),
            last_heading: None,
            heading_fresh: false,
            cur: None,
            pending_cmd: false,
        }
    }

    fn consume(&mut self, text: &str) {
        // fence_kind: Some('`') 或 Some('~')，表示当前正在收集一个属于 @cmd 的 fence。
        let mut fence_kind: Option<char> = None;
        let mut cmd_body: Vec<String> = Vec::new();
        let mut cmd_lang: Option<String> = None;

        for (idx, line) in text.lines().enumerate() {
            let line_no = idx + 1;

            // 1) ## 标题：只记录最近标题，不参与边界。
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

            // 2) @index：结束上一条，开新条目。@ 行不破坏标题紧贴性。
            let index_val = directive_value(line, "index");
            if let Some(val) = index_val {
                self.flush_current();
                let title = if self.heading_fresh {
                    self.last_heading.clone().filter(|h| !h.is_empty())
                } else {
                    None
                };
                self.cur = Some(CurEntry {
                    start: line_no,
                    title,
                    index_terms: split_ws(val),
                    lines: Vec::new(),
                    directives: Vec::new(),
                });
                if let Some(cur) = self.cur.as_mut() {
                    cur.lines.push(line.to_string());
                }
                continue;
            }

            // 3) @cmd：标记待绑定，不破坏标题紧贴性。
            if directive_value(line, "cmd").is_some() || is_directive(line, "cmd") {
                self.pending_cmd = true;
                if let Some(cur) = self.cur.as_mut() {
                    cur.lines.push(line.to_string());
                }
                continue;
            }

            // 4) fence 起始：若存在待绑定 @cmd，开始收集其内容。
            if fence_kind.is_none() {
                if let Some(kind) = fence_kind_of(line) {
                    if self.pending_cmd {
                        fence_kind = Some(kind);
                        cmd_lang = fence_lang(line);
                        cmd_body.clear();
                        if let Some(cur) = self.cur.as_mut() {
                            cur.lines.push(line.to_string());
                        }
                        // fence 正文会破坏标题紧贴性。
                        self.heading_fresh = false;
                        continue;
                    }
                }
            }

            // 5) 正在收集 cmd fence 内容。
            if let Some(kind) = fence_kind {
                if is_fence_close(line, kind) {
                    if let Some(cur) = self.cur.as_mut() {
                        cur.lines.push(line.to_string());
                        cur.directives.push(Directive::Cmd {
                            lang: cmd_lang.take(),
                            body: cmd_body.join("\n"),
                        });
                    }
                    self.pending_cmd = false;
                    fence_kind = None;
                    cmd_body.clear();
                    continue;
                }
                cmd_body.push(line.to_string());
                if let Some(cur) = self.cur.as_mut() {
                    cur.lines.push(line.to_string());
                }
                continue;
            }

            // 6) 其余：正文。
            if !line.trim().is_empty() {
                self.heading_fresh = false; // 正文行破坏标题紧贴性
            }
            if let Some(cur) = self.cur.as_mut() {
                cur.lines.push(line.to_string());
            }
        }

        // 文件结尾：未闭合的 cmd fence 也照常绑定。
        if let Some(cur) = self.cur.as_mut() {
            if self.pending_cmd && !cmd_body.is_empty() {
                cur.directives.push(Directive::Cmd {
                    lang: cmd_lang.take(),
                    body: cmd_body.join("\n"),
                });
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
                .unwrap_or_else(|| cur.index_terms.first().cloned().unwrap_or_default());
            self.entries.push(Entry {
                title,
                index_terms: std::mem::take(&mut cur.index_terms),
                tags: Vec::new(),
                path: self.path.to_path_buf(),
                start: cur.start,
                raw: cur.lines.join("\n"),
                directives: std::mem::take(&mut cur.directives),
            });
        }
    }
}

/// 返回 `@name` 行的值（`@index xxx` → `xxx`），或 None。
fn directive_value<'a>(line: &'a str, name: &str) -> Option<&'a str> {
    let prefix = format!("@{name}");
    let rest = line.strip_prefix(&prefix)?;
    if rest.starts_with(' ') || rest.starts_with('\t') {
        Some(rest.trim())
    } else if rest.is_empty() {
        Some("")
    } else {
        None
    }
}

fn is_directive(line: &str, name: &str) -> bool {
    directive_value(line, name).is_some()
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

/// 扫描一个内容根目录下所有 `*.md` / `*.mdx`，返回按路径稳定排序的条目。
pub fn scan_dir(root: &Path) -> Result<Vec<Entry>, ScanError> {
    let mut files = Vec::new();
    collect_md(root, &mut files).map_err(ScanError::Io)?;
    files.sort();

    let mut entries = Vec::new();
    for file in files {
        let rel = file.strip_prefix(root).unwrap_or(&file).to_path_buf();
        for mut e in scan_file(&file)? {
            e.path = rel.clone();
            entries.push(e);
        }
    }
    Ok(entries)
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
    fn splits_two_entries() {
        let text = "\
## Head
@index alpha beta
body one
@index gamma
body two
";
        let entries = scan_text(text, Path::new("a.md"));
        assert_eq!(entries.len(), 2);
        assert_eq!(entries[0].title, "Head");
        assert_eq!(entries[0].index_terms, vec!["alpha", "beta"]);
        assert_eq!(entries[0].start, 2);
        assert_eq!(entries[1].title, "gamma");
        assert_eq!(entries[1].start, 4);
    }

    #[test]
    fn shared_heading_when_adjacent() {
        let text = "\
## Head
@index alpha
body
@index beta
";
        let entries = scan_text(text, Path::new("a.md"));
        assert_eq!(entries.len(), 2);
        assert_eq!(entries[0].title, "Head");
        // 第二个 @index 上方隔了正文，标题不再继承，退回 index 首词
        assert_eq!(entries[1].title, "beta");
    }

    #[test]
    fn binds_cmd_fence() {
        let text = "\
@index run stuff
@cmd
```bash
echo hi
echo bye
```
tail
";
        let entries = scan_text(text, Path::new("a.md"));
        assert_eq!(entries.len(), 1);
        let e = &entries[0];
        assert_eq!(e.directives.len(), 1);
        match &e.directives[0] {
            Directive::Cmd { lang, body } => {
                assert_eq!(lang.as_deref(), Some("bash"));
                assert_eq!(body, "echo hi\necho bye");
            }
            _ => panic!("expected Cmd"),
        }
        assert!(e.raw.contains("echo hi"));
    }

    #[test]
    fn entry_ends_at_eof() {
        let text = "@index only\nline one\nline two";
        let entries = scan_text(text, Path::new("a.md"));
        assert_eq!(entries.len(), 1);
        assert_eq!(entries[0].raw.lines().count(), 3);
    }
}
