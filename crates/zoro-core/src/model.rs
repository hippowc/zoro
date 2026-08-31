use std::path::PathBuf;

/// `@` 标签的语义分类（由 `registry` 统一分类）。
///
/// zoro 的统一内容模型：每个标签都产出一个 [`Block`]——
/// 标签名 = 维度（`kind`），标签值 = 搜索词（`terms`），其后是负载（`raw`）。
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum TagKind {
    /// `@index`：正文叙述块（负载 = 到下一个标签之间的 Markdown）。
    Index,
    /// `@shell`：在终端执行的 shell 命令块（负载 = 紧随的 fenced code block）。
    Shell,
    /// `@video`：视频块（v1 注册表占位；负载先按正文保存）。
    Video,
    /// `@image`：图片块（v1 注册表占位；负载先按正文保存）。
    Image,
    /// 未注册标签：安全降级为普通正文块（保留原始标签名）。
    Unknown(String),
}

impl Default for TagKind {
    fn default() -> Self {
        TagKind::Index
    }
}

impl TagKind {
    /// manifest / 展示用的稳定标识。
    pub fn as_str(&self) -> String {
        match self {
            TagKind::Index => "index".to_string(),
            TagKind::Shell => "shell".to_string(),
            TagKind::Video => "video".to_string(),
            TagKind::Image => "image".to_string(),
            TagKind::Unknown(name) => format!("unknown:{name}"),
        }
    }

    pub fn from_str(s: &str) -> Self {
        match s {
            "index" => TagKind::Index,
            "shell" => TagKind::Shell,
            "video" => TagKind::Video,
            "image" => TagKind::Image,
            other => other
                .strip_prefix("unknown:")
                .map(|n| TagKind::Unknown(n.to_string()))
                .unwrap_or_else(|| TagKind::Unknown(other.to_string())),
        }
    }

    pub fn is_shell(&self) -> bool {
        matches!(self, TagKind::Shell)
    }
}

/// `@shell` 块的负载元数据。
#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct ShellBlock {
    pub lang: Option<String>,
    /// fence 内容每行在源文件中的 1-based 绝对行号（供精确截取 / 执行）。
    pub lines: Vec<usize>,
}

/// 统一内容块：zoro 的最小检索 / 展示单元。
///
/// 身份 = `(library, path, start)`。
#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct Block {
    pub library: String,
    pub title: String,
    pub kind: TagKind,
    /// 标签行后的搜索词（检索面）。
    pub terms: Vec<String>,
    pub path: PathBuf,
    /// 标签行的 1-based 行号。
    pub start: usize,
    /// 负载原文（从标签行到边界；运行期独有，manifest 不存）。
    pub raw: String,
    /// 仅 `@shell`：fence 元数据。
    pub shell: Option<ShellBlock>,
}

impl Block {
    /// 检索面文本。
    pub fn index_text(&self) -> String {
        self.terms.join(" ")
    }
}
