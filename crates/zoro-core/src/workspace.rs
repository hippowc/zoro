use std::collections::BTreeSet;
use std::path::{Path, PathBuf};

use crate::config::WorkspaceConfig;
use crate::library::Library;
use crate::{query, Block};

pub type Result<T> = std::result::Result<T, Box<dyn std::error::Error>>;

/// 多库工作区：搜索 / 浏览的默认作用域。
///
/// 块全局身份 = `(库名, path, start)`；聚合查询时库名作为命名空间前缀。
#[derive(Debug, Clone, Default)]
pub struct Workspace {
    pub libraries: Vec<Library>,
}

impl Workspace {
    /// 由工作区声明构建（冷启动优先，过期自动重建）。
    pub fn from_config(config: &WorkspaceConfig) -> Result<Self> {
        let mut seen = BTreeSet::new();
        for lib in &config.libraries {
            if !seen.insert(lib.name.as_str()) {
                return Err(format!("duplicate library name: {}", lib.name).into());
            }
        }
        let mut libraries = Vec::with_capacity(config.libraries.len());
        for lib in &config.libraries {
            libraries.push(Library::open_cached_with_config(
                &lib.name,
                &lib.root,
                lib.config.clone(),
            )?);
        }
        Ok(Self { libraries })
    }

    /// 读 `zoro.toml` 并构建工作区（`root` 相对路径按文件所在目录解析）。
    pub fn from_toml_file(path: &Path) -> Result<Self> {
        let config = WorkspaceConfig::from_path(path)?;
        Self::from_config(&config)
    }

    /// 以默认配置声明若干库（编程 API / 测试便捷）。
    pub fn from_libs(specs: &[(String, PathBuf)]) -> Result<Self> {
        let config = WorkspaceConfig {
            default: None,
            libraries: specs
                .iter()
                .map(|(name, root)| crate::config::LibrarySpec {
                    name: name.clone(),
                    root: root.clone(),
                    config: Default::default(),
                })
                .collect(),
        };
        Self::from_config(&config)
    }

    pub fn library_names(&self) -> Vec<String> {
        self.libraries.iter().map(|l| l.name.clone()).collect()
    }

    /// 对全部库执行 analyze。`force=true` 无条件重扫。
    pub fn analyze_all(&mut self, force: bool) -> Result<()> {
        for lib in &mut self.libraries {
            lib.analyze(force)?;
        }
        Ok(())
    }

    /// 聚合查询，结果按分数降序（跨库）。
    pub fn query(&self, q: &str) -> Vec<query::Candidate> {
        let mut out = Vec::new();
        for lib in &self.libraries {
            out.extend(lib.query(q));
        }
        out.sort_by(|a, b| b.score.cmp(&a.score));
        out
    }

    /// 按三元组定位库并现场截取块原文。
    pub fn load_raw(&self, library: &str, path: &Path, start: usize) -> Result<String> {
        let lib = self
            .libraries
            .iter()
            .find(|l| l.name == library)
            .ok_or_else(|| format!("library not found: {library}"))?;
        let block = Block {
            library: library.to_string(),
            path: path.to_path_buf(),
            start,
            ..Default::default()
        };
        lib.load_raw(&block)
    }
}
