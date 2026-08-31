use std::path::PathBuf;

/// 一个被 `@index` 锚定的知识条目。
#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct Entry {
    /// 可读标题：`##` 文本，或 index 首词兜底。
    pub title: String,
    /// `@index` 的关键词列表（命中面）。
    pub index_terms: Vec<String>,
    /// 预留：tag（当前恒为空）。
    pub tags: Vec<String>,
    /// 源文件相对内容根的路径。
    pub path: PathBuf,
    /// `@index` 行号（1-based）。展示时从此行截取到边界。
    pub start: usize,
    /// 条目原文（从 `@index` 行到边界：下一个 `@index` / EOF）。
    pub raw: String,
    /// 条目内解析出的指令（不包含 `@index` 本身）。
    pub directives: Vec<Directive>,
}

impl Entry {
    /// 用于 fzf 候选行的命中面。
    pub fn index_text(&self) -> String {
        self.index_terms.join(" ")
    }
}

/// `@` 指令。`@index` 是锚点，单独存于 `Entry::index_terms`。
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Directive {
    Cmd {
        lang: Option<String>,
        body: String,
    },
    Unknown {
        name: String,
        value: String,
    },
}
