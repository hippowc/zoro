use crate::model::Block;
use pulldown_cmark::{html, CodeBlockKind, CowStr, Event, Options, Parser, Tag, TagEnd};
use syntect::highlighting::{Theme, ThemeSet};
use syntect::html::highlighted_html_for_string;
use syntect::parsing::SyntaxSet;

/// P0 降级路径：返回条目原文。
pub fn render_raw(block: &Block) -> &str {
    &block.raw
}

/// 把 Markdown 渲染为 HTML（P1）。
///
/// - 顶格 `@` 指令行先被剥离（不进 AST）；
/// - fenced code block 使用 syntect 按 fence 语言高亮；
/// - 表格 / 删除线 / 任务列表等 GFM 扩展开启。
pub fn render_markdown_html(md: &str) -> String {
    let clean = strip_directives(md);

    let mut opts = Options::empty();
    opts.insert(Options::ENABLE_TABLES);
    opts.insert(Options::ENABLE_STRIKETHROUGH);
    opts.insert(Options::ENABLE_TASKLISTS);

    let parser = Parser::new_ext(&clean, opts).map(|e| e.into_static());
    let ss = SyntaxSet::load_defaults_newlines();
    let ts = ThemeSet::load_defaults();
    let theme = ts.themes.get("InspiredGitHub").expect("InspiredGitHub theme");

    let events: Vec<Event> = parser.collect();
    let mut out: Vec<Event> = Vec::with_capacity(events.len());

    let mut i = 0usize;
    while i < events.len() {
        match &events[i] {
            Event::Start(Tag::CodeBlock(kind)) => {
                let lang = match kind {
                    CodeBlockKind::Fenced(info) => info
                        .split_whitespace()
                        .next()
                        .unwrap_or("")
                        .to_string(),
                    CodeBlockKind::Indented => String::new(),
                };
                let mut code = String::new();
                i += 1;
                while i < events.len() {
                    if let Event::End(TagEnd::CodeBlock) = events[i] {
                        break;
                    }
                    if let Event::Text(t) = &events[i] {
                        code.push_str(t.as_ref());
                    }
                    i += 1;
                }
                let highlighted = highlight_code(&code, &lang, &ss, theme);
                out.push(Event::Html(CowStr::from(highlighted)));
            }
            other => out.push(other.clone()),
        }
        i += 1;
    }

    let mut buf = String::new();
    html::push_html(&mut buf, out.into_iter());
    buf
}

/// 剥离顶格 `@name` 指令行；fence 内部的 `@` 行是代码，不剥离。
pub fn strip_directives(md: &str) -> String {
    let mut out = String::with_capacity(md.len());
    let mut in_fence = false;
    for line in md.lines() {
        let t = line.trim_start();
        if in_fence {
            out.push_str(line);
            out.push('\n');
            if t.starts_with("```") || t.starts_with("~~~") {
                in_fence = false;
            }
            continue;
        }
        if t.starts_with("```") || t.starts_with("~~~") {
            in_fence = true;
            out.push_str(line);
            out.push('\n');
            continue;
        }
        if is_directive_line(t) {
            continue;
        }
        out.push_str(line);
        out.push('\n');
    }
    out
}

fn is_directive_line(t: &str) -> bool {
    if let Some(rest) = t.strip_prefix('@') {
        let name: String = rest
            .chars()
            .take_while(|c| c.is_ascii_lowercase() || *c == '-')
            .collect();
        if name.is_empty() {
            return false;
        }
        let after = &rest[name.chars().count()..];
        after.is_empty() || after.starts_with(' ') || after.starts_with('\t')
    } else {
        false
    }
}

fn highlight_code(code: &str, lang: &str, ss: &SyntaxSet, theme: &Theme) -> String {
    let syntax = if lang.is_empty() {
        ss.find_syntax_plain_text()
    } else {
        ss.find_syntax_by_token(lang).unwrap_or_else(|| ss.find_syntax_plain_text())
    };
    highlighted_html_for_string(code, ss, syntax, theme)
        .unwrap_or_else(|e| {
            eprintln!("zoro-core: highlight failed for '{lang}': {e}");
            let escaped = code
                .replace('&', "&amp;")
                .replace('<', "&lt;")
                .replace('>', "&gt;");
            format!("<pre><code>{escaped}</code></pre>\n")
        })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn strips_directives_keeps_fence() {
        let md = "@index a b\n@shell\n```bash\necho @not_a_directive\n```\ntext\n";
        let out = strip_directives(md);
        assert!(!out.contains("@index"));
        assert!(!out.contains("@shell"));
        assert!(out.contains("echo @not_a_directive"));
        assert!(out.contains("```bash"));
        assert!(out.contains("text"));
    }

    #[test]
    fn renders_heading() {
        let html = render_markdown_html("# Title\n\nhello\n");
        assert!(html.contains("<h1"));
        assert!(html.contains("hello"));
    }
}
