use std::collections::BTreeSet;
use std::path::{Path, PathBuf};

use crate::{library::Library, query, Entry};

pub type Result<T> = std::result::Result<T, Box<dyn std::error::Error>>;

/// 多库工作区：搜索 / 浏览的默认作用域。
///
/// 条目全局身份 = `(库名, path, start)`；聚合查询时库名作为命名空间前缀。
#[derive(Debug, Clone, Default)]
pub struct Workspace {
    pub libraries: Vec<Library>,
}

impl Workspace {
    /// 由 `name=path` 声明列表构建工作区（冷启动优先，过期自动重建）。
    pub fn from_libs(specs: &[(String, PathBuf)]) -> Result<Self> {
        let mut seen = BTreeSet::new();
        for (name, _) in specs {
            if !seen.insert(name.clone()) {
                return Err(format!("duplicate library name: {name}").into());
            }
        }
        let mut libraries = Vec::with_capacity(specs.len());
        for (name, path) in specs {
            libraries.push(Library::open_cached(name, path)?);
        }
        Ok(Self { libraries })
    }

    /// 单库便捷构造（`ZORO_ROOT` 场景）。
    pub fn single(name: &str, root: &Path) -> Result<Self> {
        Self::from_libs(&[(name.to_string(), root.to_path_buf())])
    }

    pub fn library_names(&self) -> Vec<String> {
        self.libraries
            .iter()
            .map(|l| l.name.clone())
            .collect()
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

    /// 按三元组定位库并现场截取条目原文。
    pub fn load_raw(&self, library: &str, path: &Path, start: usize) -> Result<String> {
        let lib = self
            .libraries
            .iter()
            .find(|l| l.name == library)
            .ok_or_else(|| format!("library not found: {library}"))?;
        let entry = Entry {
            library: library.to_string(),
            path: path.to_path_buf(),
            start,
            ..Default::default()
        };
        lib.load_raw(&entry)
    }
}
