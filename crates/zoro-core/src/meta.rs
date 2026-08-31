//! 元数据 manifest：analyze 的产物，运行期查询/预览/执行的唯一入口。
//!
//! 关键约束（见 agents.md v6 §5）：
//! - manifest 是**派生物**：可删、可重建、幂等，不存 end、不存正文副本；
//! - manifest 只记录"内容声明了什么"，具体怎么做交给运行时配置/注册表。

use std::path::PathBuf;

use serde::{Deserialize, Serialize};

use crate::model::{Directive, Entry};

pub const SCHEMA: u32 = 1;

/// 单条能力声明：由 `@` 标签编译而来。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
pub struct Capabilities {
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub actions: Vec<ActionCap>,
}

impl Capabilities {
    pub fn is_empty(&self) -> bool {
        self.actions.is_empty()
    }
}

/// 一个动作：例如 `@cmd` 绑定的可执行代码块。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ActionCap {
    pub kind: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub lang: Option<String>,
    /// 代码块每行在源文件中的 1-based 绝对行号。
    pub lines: Vec<usize>,
}

impl ActionCap {
    fn from_directive(d: &Directive) -> Option<Self> {
        match d {
            Directive::Cmd { lang, lines, .. } => Some(ActionCap {
                kind: "cmd".to_string(),
                lang: lang.clone(),
                lines: lines.clone(),
            }),
            Directive::Unknown { .. } => None,
        }
    }

    /// 由 action 能力还原运行期指令（body 不存盘，冷启动后按需截取）。
    fn to_directive(&self) -> Option<Directive> {
        if self.kind != "cmd" {
            return None;
        }
        Some(Directive::Cmd {
            lang: self.lang.clone(),
            body: String::new(),
            lines: self.lines.clone(),
        })
    }
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct LibraryMeta {
    pub name: String,
    /// 分析时的输入根（通常 `.` 或绝对路径；仅作记录，不作为定位依据）。
    pub root: String,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ManifestEntry {
    pub title: String,
    /// `@index` 检索面（按空白拆词的原文）。
    pub index: String,
    /// 相对库根的路径。
    pub path: String,
    /// `@index` 行号（1-based）。
    pub start: usize,
    #[serde(default, skip_serializing_if = "Capabilities::is_empty")]
    pub caps: Capabilities,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Manifest {
    pub schema: u32,
    pub library: LibraryMeta,
    /// UNIX 秒时间戳（生成时间，用于调试，不参与内容比较）。
    pub generated_at: String,
    pub entries: Vec<ManifestEntry>,
}

impl Manifest {
    /// 从运行期条目集编译 manifest（丢弃 raw，能力交由 caps 承载）。
    pub fn from_entries(name: &str, root: &PathBuf, entries: &[Entry]) -> Self {
        Manifest {
            schema: SCHEMA,
            library: LibraryMeta {
                name: name.to_string(),
                root: root.to_string_lossy().to_string(),
            },
            generated_at: now_unix_secs(),
            entries: entries
                .iter()
                .map(|e| ManifestEntry {
                    title: e.title.clone(),
                    index: e.index_text(),
                    path: e.path.to_string_lossy().to_string(),
                    start: e.start,
                    caps: Capabilities {
                        actions: e
                            .directives
                            .iter()
                            .filter_map(ActionCap::from_directive)
                            .collect(),
                    },
                })
                .collect(),
        }
    }

    pub fn to_json(&self) -> Result<String, serde_json::Error> {
        serde_json::to_string_pretty(self)
    }

    pub fn from_json(s: &str) -> Result<Self, serde_json::Error> {
        serde_json::from_str(s)
    }

    /// 把 manifest 条目还原为运行期 `Entry`（raw 为空，正文现场截取）。
    pub fn into_entries(&self, library: &str) -> Vec<Entry> {
        self.entries
            .iter()
            .map(|e| Entry {
                library: library.to_string(),
                title: e.title.clone(),
                index_terms: e.index.split_whitespace().map(str::to_string).collect(),
                tags: Vec::new(),
                path: PathBuf::from(&e.path),
                start: e.start,
                raw: String::new(),
                directives: e
                    .caps
                    .actions
                    .iter()
                    .filter_map(ActionCap::to_directive)
                    .collect(),
            })
            .collect()
    }
}

fn now_unix_secs() -> String {
    // 不引入时间库，取 UNIX 秒转成可读字符串；格式不参与内容比较。
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map(|d| format!("{}", d.as_secs()))
        .unwrap_or_else(|_| "0".to_string())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::model::{Directive, Entry};

    fn sample_entries() -> Vec<Entry> {
        vec![Entry {
            library: String::new(),
            title: "Hello".into(),
            index_terms: vec!["alpha".into(), "beta".into()],
            path: PathBuf::from("a/b.md"),
            start: 3,
            raw: "ignored".into(),
            directives: vec![Directive::Cmd {
                lang: Some("bash".into()),
                body: "echo hi".into(),
                lines: vec![5],
            }],
            ..Default::default()
        }]
    }

    #[test]
    fn roundtrip_entries_and_caps() {
        let m = Manifest::from_entries("t", &PathBuf::from("."), &sample_entries());
        assert_eq!(m.schema, SCHEMA);
        assert_eq!(m.entries[0].caps.actions.len(), 1);
        assert_eq!(m.entries[0].caps.actions[0].kind, "cmd");
        assert_eq!(m.entries[0].caps.actions[0].lines, vec![5]);

        let json = m.to_json().unwrap();
        let m2 = Manifest::from_json(&json).unwrap();

        let entries = m2.into_entries("t");
        assert_eq!(entries[0].title, "Hello");
        assert_eq!(entries[0].index_terms, vec!["alpha", "beta"]);
        assert_eq!(entries[0].path, PathBuf::from("a/b.md"));
        assert!(entries[0].raw.is_empty());
        match &entries[0].directives[0] {
            Directive::Cmd { lang, lines, body } => {
                assert_eq!(lang.as_deref(), Some("bash"));
                assert_eq!(lines, &vec![5]);
                assert_eq!(body, "");
            }
            _ => panic!("expected cmd"),
        }
    }
}
