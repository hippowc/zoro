use std::env;
use std::path::PathBuf;
use zoro_core::{render, Workspace, WorkspaceConfig};

fn main() {
    let args: Vec<String> = env::args().skip(1).collect();

    let ws_path = workspace_path();
    let config = match WorkspaceConfig::from_path(&ws_path) {
        Ok(c) => c,
        Err(e) => {
            eprintln!("error: 读取工作区声明失败 {}: {e}", ws_path.display());
            std::process::exit(2);
        }
    };

    let mut ws = match Workspace::from_config(&config) {
        Ok(w) => w,
        Err(e) => {
            eprintln!("error: 打开工作区失败: {e}");
            std::process::exit(1);
        }
    };

    match args.first().map(|s| s.as_str()) {
        Some("index") => match ws.analyze_all(true) {
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
        },
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
            eprintln!("workspace: 由 {} 声明（或 ZORO_WORKSPACE 指定）", ws_path.display());
            std::process::exit(0);
        }
    }
}

/// 工作区声明文件查找顺序：`ZORO_WORKSPACE` → `./zoro.toml`。
fn workspace_path() -> PathBuf {
    if let Ok(p) = env::var("ZORO_WORKSPACE") {
        let p = p.trim();
        if !p.is_empty() {
            return PathBuf::from(p);
        }
    }
    PathBuf::from("zoro.toml")
}
