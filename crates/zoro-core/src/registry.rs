//! 标签注册表：`@<name>` → 语义分类 + 负载提取策略。
//!
//! 这是 zoro 扩展新标签的唯一入口。新增一个标签，只需在这里登记
//! 它的分类与负载提取方式；扫描 / 检索 / 渲染自动获得该标签。
use crate::model::TagKind;

/// 已知标签名 → 语义分类；未注册标签安全降级为 `Unknown`。
pub fn classify_tag(name: &str) -> TagKind {
    match name {
        "index" => TagKind::Index,
        "shell" => TagKind::Shell,
        "video" => TagKind::Video,
        "image" => TagKind::Image,
        other => TagKind::Unknown(other.to_string()),
    }
}

/// 负载提取策略。
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum PayloadStyle {
    /// 到下一个标签行 / EOF 之间的正文。
    Text,
    /// 紧随标签行的 fenced code block。
    Fence,
}

/// 标签分类决定负载怎么取。
pub fn payload_style(kind: &TagKind) -> PayloadStyle {
    match kind {
        TagKind::Shell => PayloadStyle::Fence,
        _ => PayloadStyle::Text,
    }
}
