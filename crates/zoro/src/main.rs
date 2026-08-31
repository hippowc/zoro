use std::env;
use std::path::PathBuf;
use zoro_core::{render, Workspace};

fn main() {
    let args: Vec<String> = env::args().skip(1).collect();

    let specs = match parse_libs() {
        Ok(s) => s,
        Err(e) => {
            eprintln!("error: {e}");
            std::process::exit(2);
        }
    };

    let mut ws = match Workspace::from_libs(&specs) {
        Ok(w) => w,
        Err(e) => {
            eprintln!("error: 打开知识库失败: {e}");
            std::process::exit(1);
        }
    };

    match args.first().map(|s| s.as_str()) {
        Some("index") => {
            match ws.analyze_all(true) {
                Ok(()) => {
                    for lib in &ws.libraries {
                        println!(
                            "{}  {}  ({} entries)",
                            lib.meta_path().display(),
                            lib.name,
                            lib.entries.len()
                        );
                    }
                }
                Err(e) => {
                    eprintln!("error: {e}");
                    std::process::exit(1);
                }
            }
        }
        Some("html") => {
            let q = args.get(1).map(|s| s.as_str()).unwrap_or("");
            let hits = ws.query(q);
            match hits.first() {
                Some(c) => match ws.load_raw(&c.library, &c.path, c.start) {
                    Ok(raw) => print!("{}", render::render_markdown_html(&raw)),
                    Err(e) => {
                        eprintln!("error: {e}");
                        std::process::exit(1);
                    }
                },
                None => {
                    eprintln!("no match for query: {q}");
                    std::process::exit(1);
                }
            }
        }
        Some(q) => {
            let hits = ws.query(q);
            for c in hits.iter().take(20) {
                println!(
                    "{}\t{}\t{}\t{}\t{}",
                    c.library,
                    c.title,
                    c.index,
                    c.start,
                    c.path.display()
                );
            }
            if hits.is_empty() {
                eprintln!("no match for query: {q}");
                std::process::exit(1);
            }
        }
        None => {
            eprintln!("usage: zoro <query|html <query>|index>");
            eprintln!("  单库: export ZORO_ROOT=/path/to/kb");
            eprintln!("  多库: export ZORO_LIBS=sanji=/path/a,robin=/path/b");
            std::process::exit(0);
        }
    }
}

/// 解析知识库声明：
/// - 优先 `ZORO_LIBS=name=path,name=path`（多库）；
/// - 否则 `ZORO_ROOT`（单库，库名取目录 basename，fallback `default`）。
fn parse_libs() -> Result<Vec<(String, PathBuf)>, String> {
    if let Ok(spec) = env::var("ZORO_LIBS") {
        if !spec.trim().is_empty() {
            let mut out = Vec::new();
            for part in spec.split(',') {
                let part = part.trim();
                if part.is_empty() {
                    continue;
                }
                let (name, path) = part
                    .split_once('=')
                    .ok_or_else(|| format!("invalid ZORO_LIBS item (expected name=path): {part}"))?;
                let name = name.trim();
                let path = path.trim();
                if name.is_empty() || path.is_empty() {
                    return Err(format!("invalid ZORO_LIBS item (empty name/path): {part}"));
                }
                out.push((name.to_string(), PathBuf::from(path)));
            }
            return Ok(out);
        }
    }

    let root = env::var("ZORO_ROOT")
        .map_err(|_| "未设置 ZORO_ROOT 或 ZORO_LIBS（指向你的 Markdown 内容库根目录）".to_string())?;
    let root = root.trim().to_string();
    if root.is_empty() {
        return Err("ZORO_ROOT 为空".to_string());
    }
    let name = std::path::Path::new(&root)
        .file_name()
        .and_then(|s| s.to_str())
        .filter(|s| !s.is_empty())
        .unwrap_or("default")
        .to_string();
    Ok(vec![(name, PathBuf::from(root))])
}
