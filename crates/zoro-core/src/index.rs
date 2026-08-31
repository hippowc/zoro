use crate::model::Block;

/// 四列：title / index / start / path。
pub const COLUMNS: [&str; 4] = ["title", "index", "start", "path"];

/// 把条目集序列化为四列 TSV（含表头，便于自检）。
pub fn to_tsv(blocks: &[Block]) -> String {
    let mut out = String::new();
    out.push_str(&COLUMNS.join("\t"));
    out.push('\n');
    for e in blocks {
        out.push_str(&tsv_escape(&e.title));
        out.push('\t');
        out.push_str(&tsv_escape(&e.index_text()));
        out.push('\t');
        out.push_str(&e.start.to_string());
        out.push('\t');
        out.push_str(&tsv_escape(&e.path.to_string_lossy()));
        out.push('\n');
    }
    out
}

/// 转义制表符 / 换行 / 反斜杠，保证仍是单行字段。
fn tsv_escape(s: &str) -> String {
    s.replace('\\', "\\\\").replace('\t', "\\t").replace('\n', "\\n")
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::model::Block;
    use std::path::PathBuf;

    #[test]
    fn renders_header_and_rows() {
        let e = Block {
            title: "Hello".into(),
            terms: vec!["alpha".into(), "beta".into()],
            path: PathBuf::from("sub/a.md"),
            start: 3,
            raw: String::new(),
            ..Default::default()
        };
        let tsv = to_tsv(&[e]);
        let mut lines = tsv.lines();
        assert_eq!(lines.next(), Some("title\tindex\tstart\tpath"));
        assert_eq!(lines.next(), Some("Hello\talpha beta\t3\tsub/a.md"));
    }
}
