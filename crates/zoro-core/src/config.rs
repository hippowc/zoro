//! 工作区声明（zoro.toml）与库级配置。
//!
//! 注意与内容元数据（manifest）的边界：
//! - `zoro.toml` 是作者维护的**配置/来源声明**，进 git；
//! - `<库根>/.zoro/meta.json` 是 analyze 的**派生内容快照**，不进 git。
//! 库级偏好（preview / allow_exec …）只属于这里，永不进 manifest。

use std::collections::BTreeMap;
use std::fs;
use std::path::{Component, Path, PathBuf};

use serde::{Deserialize, Serialize};

pub type Result<T> = std::result::Result<T, Box<dyn std::error::Error>>;

/// `zoro.toml` 顶层结构。
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct WorkspaceConfig {
    /// 无参进入时打开的默认库（按 name 指定）。
    #[serde(default)]
    pub default: Option<String>,
    #[serde(default)]
    pub libraries: Vec<LibrarySpec>,
}

impl WorkspaceConfig {
    pub fn parse(s: &str) -> Result<Self> {
        Ok(toml::from_str(s)?)
    }

    /// 读文件后的路径解析入口：`root` 支持相对 `zoro.toml` 所在目录。
    pub fn from_path(path: &Path) -> Result<Self> {
        let text = fs::read_to_string(path)?;
        let mut cfg: Self = toml::from_str(&text)?;
        cfg.resolve_paths(path.parent().unwrap_or_else(|| Path::new(".")));
        Ok(cfg)
    }

    /// 把相对 `root` 基于 `base` 目录转为绝对路径。
    pub fn resolve_paths(&mut self, base: &Path) {
        for lib in &mut self.libraries {
            if lib.root.is_relative() {
                lib.root = normalize(base.join(&lib.root));
            }
        }
    }
}

/// `[[libraries]]` 中的一项。
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LibrarySpec {
    pub name: String,
    pub root: PathBuf,
    #[serde(default)]
    pub config: LibraryConfig,
}

/// 库级配置：只承载"该库怎么被消费"的偏好。
///
/// 未知键通过 `extra` 保留，保证前向兼容（配置可先行、实现可后补）。
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct LibraryConfig {
    /// 候选保留：默认预览 target（如 `markdown` / `html`），尚未启用。
    #[serde(default)]
    pub preview: Option<String>,
    /// 候选保留：是否允许执行 `@cmd`（安全的默认策略是拒绝）。
    #[serde(default)]
    pub allow_exec: Option<bool>,
    #[serde(default, flatten)]
    pub extra: BTreeMap<String, toml::Value>,
}

/// 词法规范化 `a/b/./c`、`a/b/../c`，不要求路径存在、不解析符号链接。
fn normalize(path: PathBuf) -> PathBuf {
    let mut out = PathBuf::new();
    for comp in path.components() {
        match comp {
            Component::CurDir => {}
            Component::ParentDir => {
                out.pop();
            }
            other => out.push(other.as_os_str()),
        }
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_multi_library_and_unknown_keys() {
        let text = r#"
default = "sanji"

[[libraries]]
name = "sanji"
root = "kb/sanji"

[libraries.config]
preview = "html"
future_option = "kept"

[[libraries]]
name = "robin"
root = "/abs/robin"
"#;
        let mut cfg = WorkspaceConfig::parse(text).unwrap();
        cfg.resolve_paths(Path::new("/ws"));
        assert_eq!(cfg.default.as_deref(), Some("sanji"));
        assert_eq!(cfg.libraries.len(), 2);
        assert_eq!(cfg.libraries[0].root, Path::new("/ws/kb/sanji"));
        assert_eq!(cfg.libraries[0].config.preview.as_deref(), Some("html"));
        assert!(cfg.libraries[0].config.extra.contains_key("future_option"));
        assert_eq!(cfg.libraries[1].root, Path::new("/abs/robin"));
    }
}
