use std::fs;
use std::io::Write;
use std::path::{Path, PathBuf};

use crate::{index, query, scan, Entry};

/// 内容库：一个内容根 + 它的条目 + 索引缓存。
#[derive(Debug, Clone)]
pub struct Library {
    pub root: PathBuf,
    pub entries: Vec<Entry>,
}

pub type Result<T> = std::result::Result<T, Box<dyn std::error::Error>>;

impl Library {
    /// 全量扫描一个内容根。
    pub fn open(root: &Path) -> Result<Self> {
        Ok(Self {
            root: root.to_path_buf(),
            entries: scan::scan_dir(root)?,
        })
    }

    pub fn index_path(&self) -> PathBuf {
        self.root.join("zoro-index.tsv")
    }

    /// 索引是否过期（不存在，或内容根里有更晚修改的 `*.md`/`*.mdx`）。
    pub fn is_stale(&self) -> Result<bool> {
        let p = self.index_path();
        if !p.exists() {
            return Ok(true);
        }
        let idx_mtime = fs::metadata(&p)?.modified()?;
        Ok(has_newer_md(&self.root, idx_mtime)?)
    }

    /// 保证索引新鲜。过期则重扫并原子写入；否则不重建。
    pub fn ensure_index(&mut self, force: bool) -> Result<PathBuf> {
        let p = self.index_path();
        let stale = force || self.is_stale()?;
        if stale {
            self.entries = scan::scan_dir(&self.root)?;
            atomic_write(&p, index::to_tsv(&self.entries).as_bytes())?;
        }
        Ok(p)
    }

    /// 对当前条目集执行查询。
    pub fn query(&self, q: &str) -> Vec<query::Candidate> {
        query::search(&self.entries, q)
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
    let tmp = path.with_extension("tsv.tmp");
    let mut f = fs::File::create(&tmp)?;
    f.write_all(bytes)?;
    f.sync_all()?;
    fs::rename(&tmp, path)?;
    Ok(())
}
