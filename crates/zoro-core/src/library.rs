use std::fs;
use std::io::Write;
use std::path::{Path, PathBuf};

use crate::{index, meta, query, scan, Entry};

pub type Result<T> = std::result::Result<T, Box<dyn std::error::Error>>;

/// 一个知识库：名称 + 内容根 + 条目（三元组身份的 `library` 分量）。
#[derive(Debug, Clone)]
pub struct Library {
    pub name: String,
    pub root: PathBuf,
    pub entries: Vec<Entry>,
}

impl Library {
    /// 全量扫描一个内容根（在线模式，`entry.raw` 有值）。
    pub fn open(name: &str, root: &Path) -> Result<Self> {
        let mut lib = Self {
            name: name.to_string(),
            root: root.to_path_buf(),
            entries: Vec::new(),
        };
        lib.rescan()?;
        Ok(lib)
    }

    /// 冷启动优先：manifest 存在且未过期则直接读元数据（`entry.raw` 为空）；
    /// 否则全量扫描并落盘。
    pub fn open_cached(name: &str, root: &Path) -> Result<Self> {
        let mut lib = Self {
            name: name.to_string(),
            root: root.to_path_buf(),
            entries: Vec::new(),
        };
        lib.analyze(false)?;
        Ok(lib)
    }

    pub fn meta_path(&self) -> PathBuf {
        self.root.join(".zoro").join("meta.json")
    }

    pub fn index_view_path(&self) -> PathBuf {
        self.root.join("zoro-index.tsv")
    }

    /// 是否存在比 manifest 更晚修改的 markdown 文件（含 manifest 缺失）。
    pub fn is_stale(&self) -> Result<bool> {
        let p = self.meta_path();
        if !p.exists() {
            return Ok(true);
        }
        let meta_mtime = fs::metadata(&p)?.modified()?;
        Ok(has_newer_md(&self.root, meta_mtime)?)
    }

    /// 分析（analyze）：必要时重扫，并写 manifest + 索引视图。
    ///
    /// - `force=true`：无条件重建；
    /// - `force=false`：仅在 stale 时重建，否则直接由 manifest 冷启动。
    pub fn analyze(&mut self, force: bool) -> Result<PathBuf> {
        if force || self.is_stale()? {
            self.rescan()?;
            self.write_manifest()?;
            self.write_index_view()?;
        } else {
            self.load_manifest()?;
        }
        Ok(self.meta_path())
    }

    pub fn rescan(&mut self) -> Result<()> {
        self.entries = scan::scan_dir_named(&self.root, &self.name)?;
        Ok(())
    }

    fn write_manifest(&self) -> Result<()> {
        let manifest = meta::Manifest::from_entries(&self.name, &self.root, &self.entries);
        let json = manifest.to_json()?;
        let dst = self.meta_path();
        if let Some(dir) = dst.parent() {
            fs::create_dir_all(dir)?;
        }
        atomic_write(&dst, json.as_bytes())?;
        Ok(())
    }

    fn write_index_view(&self) -> Result<()> {
        atomic_write(&self.index_view_path(), index::to_tsv(&self.entries).as_bytes())?;
        Ok(())
    }

    fn load_manifest(&mut self) -> Result<()> {
        let json = fs::read_to_string(self.meta_path())?;
        let manifest = meta::Manifest::from_json(&json)?;
        if manifest.schema != meta::SCHEMA {
            return Err(format!(
                "unsupported manifest schema {} (expected {})",
                manifest.schema,
                meta::SCHEMA
            )
            .into());
        }
        self.entries = manifest.into_entries(&self.name);
        Ok(())
    }

    /// 现场截取条目原文：按 `(path, start)` 重读源文件并推导边界。
    ///
    /// 冷启动后 `entry.raw` 为空，展示/渲染前调用此方法补齐。
    pub fn load_raw(&self, entry: &Entry) -> Result<String> {
        let text = fs::read_to_string(self.root.join(&entry.path))?;
        Ok(scan::slice_entry(&text, entry.start))
    }

    pub fn query(&self, q: &str) -> Vec<query::Candidate> {
        query::search(&self.entries, &self.name, q)
    }
}

/// 内容根内是否存在比 `since` 更新的 markdown 文件。
fn has_newer_md(root: &Path, since: std::time::SystemTime) -> Result<bool> {
    fn walk(root: &Path, since: std::time::SystemTime) -> Result<bool> {
        if !root.is_dir() {
            return Ok(false);
        }
        for entry in fs::read_dir(root)? {
            let entry = entry?;
            let path = entry.path();
            if entry.file_type()?.is_dir() {
                let name = entry.file_name();
                if name.to_string_lossy().starts_with('.') {
                    continue;
                }
                if walk(&path, since)? {
                    return Ok(true);
                }
            } else if let Some(ext) = path.extension().and_then(|e| e.to_str()) {
                if (ext.eq_ignore_ascii_case("md") || ext.eq_ignore_ascii_case("mdx"))
                    && entry.metadata()?.modified()? > since
                {
                    return Ok(true);
                }
            }
        }
        Ok(false)
    }
    walk(root, since)
}

/// 原子写：先写同目录临时文件，再 rename 覆盖。
fn atomic_write(path: &Path, bytes: &[u8]) -> std::io::Result<()> {
    let tmp = path.with_extension(format!("{}.tmp", path.extension().unwrap_or_default().to_string_lossy()));
    let mut f = fs::File::create(&tmp)?;
    f.write_all(bytes)?;
    f.sync_all()?;
    fs::rename(&tmp, path)?;
    Ok(())
}
