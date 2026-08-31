use crate::model::Entry;

/// P0：返回条目原文（`@index` 起步到边界的整段），供前端直接展示。
///
/// P1 起替换为 Markdown→HTML/ANSI 管线；本函数保留为"不渲染"的降级路径。
pub fn render_raw(entry: &Entry) -> &str {
    &entry.raw
}

/// P1 占位：Markdown → HTML（待接入 pulldown-cmark/syntect）。
pub fn render_markdown_html(_md: &str) -> String {
    // 先退回原文 escape，不引入依赖；P1 实现真正的渲染。
    let mut s = String::new();
    for line in _md.lines() {
        s.push_str(&html_escape(line));
        s.push('\n');
    }
    s
}

fn html_escape(s: &str) -> String {
    s.replace('&', "&amp;")
        .replace('<', "&lt;")
        .replace('>', "&gt;")
        .replace('"', "&quot;")
}
