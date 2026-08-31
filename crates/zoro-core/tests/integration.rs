use std::path::Path;
use zoro_core::{query, scan, Library};

#[test]
fn scans_fixture_dir() {
    let root = Path::new(concat!(env!("CARGO_MANIFEST_DIR"), "/../../tests/fixtures"));
    let entries = scan::scan_dir(root).unwrap();
    assert_eq!(entries.len(), 2);

    let lib = Library::open(root).unwrap();
    let hits: Vec<String> = query::search(&lib.entries, "定投")
        .into_iter()
        .map(|c| c.entry.title.clone())
        .collect();
    assert_eq!(hits, vec!["资产配置"]);
}
