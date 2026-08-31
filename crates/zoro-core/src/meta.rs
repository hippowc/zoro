//! 元数据 manifest：analyze 的产物，运行期查询 / 预览 / 执行的唯一入口。
//!
//! 关键约束（见 agents.md v10 §5）：
//! - manifest 是**派生物**：可删、可重建、幂等，不存边界、不存正文副本；
//! - manifest 只记录"内容声明了什么"（kind / terms / shell 元数据），
//!   具体怎么做交给运行时配置 / 注册表。

use std::path::PathBuf;

use serde::{Deserialize, Serialize};

use crate::model::{Block, ShellBlock, TagKind};

pub const SCHEMA: u32 = 2;

/// `@shell` 块的负载元数据（body 不存盘，运行期现场截取）。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ShellCap {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub lang: Option<String>,
    /// fence 内容每行在源文件中的 1-based 绝对行号。
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub lines: Vec<usize>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct LibraryMeta {
    pub name: String,
    /// 分析时的输入根（通常 `.` 或绝对路径；仅作记录，不作为定位依据）。
    pub root: String,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ManifestBlock {
    pub title: String,
    /// 标签的语义分类稳定标识（`index` / `shell` / `video` / `unknown:<name>` …）。
    pub kind: String,
    /// 检索面（标签行后的搜索词原文）。
    pub index: String,
    /// 相对库根的路径。
    pub path: String,
    /// 标签行号（1-based）。
    pub start: usize,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub shell: Option<ShellCap>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Manifest {
    pub schema: u32,
    pub library: LibraryMeta,
    /// UNIX 秒时间戳（生成时间，用于调试，不参与内容比较）。
    pub generated_at: String,
    pub blocks: Vec<ManifestBlock>,
}

impl Manifest {
    /// 从运行期块集编译 manifest（丢弃 raw，能力交由 `shell` 元数据承载）。
    pub fn from_blocks(name: &str, root: &PathBuf, blocks: &[Block]) -> Self {
        Manifest {
            schema: SCHEMA,
            library: LibraryMeta {
                name: name.to_string(),
                root: root.to_string_lossy().to_string(),
            },
            generated_at: now_unix_secs(),
            blocks: blocks
                .iter()
                .map(|b| ManifestBlock {
                    title: b.title.clone(),
                    kind: b.kind.as_str(),
                    index: b.index_text(),
                    path: b.path.to_string_lossy().to_string(),
                    start: b.start,
                    shell: b.shell.as_ref().map(|s| ShellCap {
                        lang: s.lang.clone(),
                        lines: s.lines.clone(),
                    }),
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

    /// 把 manifest 块还原为运行期 `Block`（raw 为空，正文现场截取）。
    pub fn into_blocks(&self, library: &str) -> Vec<Block> {
        self.blocks
            .iter()
            .map(|b| Block {
                library: library.to_string(),
                title: b.title.clone(),
                kind: TagKind::from_str(&b.kind),
                terms: b.index.split_whitespace().map(str::to_string).collect(),
                path: PathBuf::from(&b.path),
                start: b.start,
                raw: String::new(),
                shell: b.shell.as_ref().map(|s| ShellBlock {
                    lang: s.lang.clone(),
                    lines: s.lines.clone(),
                }),
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
    use crate::model::ShellBlock;

    fn sample_blocks() -> Vec<Block> {
        vec![Block {
            library: String::new(),
            title: "Hello".into(),
            kind: TagKind::Shell,
            terms: vec!["alpha".into(), "beta".into()],
            path: PathBuf::from("a/b.md"),
            start: 3,
            raw: "ignored".into(),
            shell: Some(ShellBlock {
                lang: Some("bash".into()),
                lines: vec![5],
            }),
        }]
    }

    #[test]
    fn roundtrip_blocks_and_shell_cap() {
        let m = Manifest::from_blocks("t", &PathBuf::from("."), &sample_blocks());
        assert_eq!(m.schema, SCHEMA);
        assert_eq!(m.blocks[0].kind, "shell");
        assert_eq!(m.blocks[0].index, "alpha beta");
        assert_eq!(m.blocks[0].shell.as_ref().unwrap().lines, vec![5]);

        let json = m.to_json().unwrap();
        let m2 = Manifest::from_json(&json).unwrap();

        let blocks = m2.into_blocks("t");
        assert_eq!(blocks[0].title, "Hello");
        assert_eq!(blocks[0].kind, TagKind::Shell);
        assert_eq!(blocks[0].terms, vec!["alpha", "beta"]);
        assert_eq!(blocks[0].path, PathBuf::from("a/b.md"));
        assert!(blocks[0].raw.is_empty());
        let shell = blocks[0].shell.as_ref().unwrap();
        assert_eq!(shell.lang.as_deref(), Some("bash"));
        assert_eq!(shell.lines, vec![5]);
    }

    #[test]
    fn unknown_kind_roundtrips() {
        let blocks = vec![Block {
            kind: TagKind::Unknown("foo".into()),
            terms: vec!["bar".into()],
            ..Default::default()
        }];
        let m = Manifest::from_blocks("t", &PathBuf::from("."), &blocks);
        assert_eq!(m.blocks[0].kind, "unknown:foo");
        let out = Manifest::from_json(&m.to_json().unwrap()).unwrap();
        assert_eq!(out.into_blocks("t")[0].kind, TagKind::Unknown("foo".into()));
    }
}
