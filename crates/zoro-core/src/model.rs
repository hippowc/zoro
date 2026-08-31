use std::path::PathBuf;

/// 一个被 `@index` 锚定的知识条目。
///
/// 条目全局身份 = `(library, path, start)`；`path` 是相对库根的路径，
/// `start` 是 `@index` 行的 1-based 行号。
#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct Entry {
    /// 所属知识库名（工作区多库时的命名空间）。
    pub library: String,
    /// 可读标题：`##` 文本，或 index 首词兜底。
    pub title: String,
    /// `@index` 的关键词列表（命中面）。
    pub index_terms: Vec<String>,
    /// 预留：tag（当前恒为空）。
    pub tags: Vec<String>,
    /// 源文件相对库根的路径。
    pub path: PathBuf,
    /// `@index` 行号（1-based）。展示时从此行截取到边界。
    pub start: usize,
    /// 条目原文（从 `@index` 行到边界：下一个 `@index` / EOF）。
    ///
    /// 注意：元数据（manifest）不含此字段——它是运行期对象，冷启动时为
    /// 空串，正文由 `Library::load_raw` 按 `(path, start)` 现场截取。
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
        /// 代码正文（行尾换行保留在行内部，与 fence 内一致）。
        body: String,
        /// 代码块内每行在源文件中的 1-based 绝对行号。
        ///
        /// 供 manifest 的 `caps.actions[].lines` 使用；执行/复制时按行号
        /// 精确截取，不必重解析。
        lines: Vec<usize>,
    },
    Unknown {
        name: String,
        value: String,
    },
}

impl Directive {
    /// 该指令进 manifest 时的能力种类；对 manifest 不关心的指令返回 None。
    pub fn capability_kind(&self) -> Option<&'static str> {
        match self {
            Directive::Cmd { .. } => Some("cmd"),
            Directive::Unknown { .. } => None,
        }
    }
}
