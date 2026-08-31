use std::fs;
use std::path::{Path, PathBuf};
use zoro_core::{Library, Workspace};

const FIXTURES: &str = "fixtures";
const FIXTURES2: &str = "fixtures2";

fn fixture(tag: &str) -> PathBuf {
    Path::new(concat!(env!("CARGO_MANIFEST_DIR"), "/../../tests"))
        .join(tag)
}

/// 一个唯一的临时目录，避免测试在仓库内写入 .zoro / zoro-index.tsv。
fn unique_tmp(tag: &str) -> PathBuf {
    let uniq = format!(
        "zoro-test-{}-{}-{:?}",
        tag,
        std::process::id(),
        std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap()
            .as_nanos()
    );
    std::env::temp_dir().join(uniq)
}

fn copy_fixture(tag: &str) -> PathBuf {
    let dst = unique_tmp(tag);
    copy_dir(&fixture(tag), &dst).unwrap();
    dst
}

fn copy_dir(src: &Path, dst: &Path) -> std::io::Result<()> {
    fs::create_dir_all(dst)?;
    for entry in fs::read_dir(src)? {
        let entry = entry?;
        let path = entry.path();
        if entry.file_type()?.is_dir() {
            copy_dir(&path, &dst.join(entry.file_name()))?;
        } else {
            fs::copy(&path, dst.join(entry.file_name()))?;
        }
    }
    Ok(())
}

#[test]
fn scans_fixture_dir() {
    let root = copy_fixture(FIXTURES);
    let lib = Library::open("fixtures", &root).unwrap();

    let hits: Vec<String> = lib
        .query("定投")
        .into_iter()
        .map(|c| c.title.clone())
        .collect();
    assert_eq!(hits, vec!["资产配置"]);
}

#[test]
fn multi_library_does_not_cross_contaminate() {
    let a = copy_fixture(FIXTURES);
    let b = copy_fixture(FIXTURES2);
    let ws = Workspace::from_libs(&[("lib-a".to_string(), a), ("lib-b".to_string(), b)]).unwrap();

    let hits = ws.query("定投");
    assert_eq!(hits.len(), 1);
    assert_eq!(hits[0].library, "lib-a");
    assert_eq!(hits[0].title, "资产配置");

    let raw = ws
        .load_raw(&hits[0].library, &hits[0].path, hits[0].start)
        .unwrap();
    assert!(raw.contains("@index 定投 止盈 资产配置"));
}

#[test]
fn from_toml_file_with_relative_roots() {
    let base = unique_tmp("workspace");
    copy_dir(&fixture(FIXTURES), &base.join("lib-a")).unwrap();
    copy_dir(&fixture(FIXTURES2), &base.join("lib-b")).unwrap();
    fs::write(
        base.join("zoro.toml"),
        r#"
default = "lib-a"

[[libraries]]
name = "lib-a"
root = "lib-a"

[[libraries]]
name = "lib-b"
root = "lib-b"
"#,
    )
    .unwrap();

    let ws = Workspace::from_toml_file(&base.join("zoro.toml")).unwrap();
    assert_eq!(ws.library_names(), vec!["lib-a", "lib-b"]);

    let hits = ws.query("定投");
    assert_eq!(hits.len(), 1);
    assert_eq!(hits[0].library, "lib-a");
}

#[test]
fn analyze_writes_manifest_and_tsv_view() {
    let root = copy_fixture(FIXTURES);
    let mut lib = Library::open("fixtures", &root).unwrap();
    let meta_path = lib.analyze(true).unwrap();

    assert!(meta_path.exists());
    let json = fs::read_to_string(&meta_path).unwrap();
    let manifest = zoro_core::meta::Manifest::from_json(&json).unwrap();
    assert_eq!(manifest.schema, zoro_core::meta::SCHEMA);
    assert_eq!(manifest.library.name, "fixtures");
    assert_eq!(manifest.entries.len(), 2);

    let tsv = fs::read_to_string(lib.index_view_path()).unwrap();
    assert!(tsv.lines().next().unwrap().starts_with("title\tindex\tstart\tpath"));
}
