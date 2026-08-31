use std::env;
use std::path::Path;
use zoro_core::{render, Library};

fn main() {
    let args: Vec<String> = env::args().skip(1).collect();
    let root = match env::var("ZORO_ROOT") {
        Ok(r) if !r.is_empty() => r,
        _ => {
            eprintln!("error: ZORO_ROOT 未设置（指向你的 Markdown 内容库根目录）");
            std::process::exit(2);
        }
    };

    let mut lib = match Library::open(Path::new(&root)) {
        Ok(l) => l,
        Err(e) => {
            eprintln!("error: 打开内容根失败: {e}");
            std::process::exit(1);
        }
    };

    match args.first().map(|s| s.as_str()) {
        Some("index") => {
            match lib.ensure_index(true) {
                Ok(p) => println!("{}  ({} entries)", p.display(), lib.entries.len()),
                Err(e) => {
                    eprintln!("error: {e}");
                    std::process::exit(1);
                }
            }
        }
        Some("html") => {
            let q = args.get(1).map(|s| s.as_str()).unwrap_or("");
            let hits = lib.query(q);
            match hits.first() {
                Some(c) => print!("{}", render::render_markdown_html(&c.entry.raw)),
                None => {
                    eprintln!("no match for query: {q}");
                    std::process::exit(1);
                }
            }
        }
        Some(q) => {
            let hits = lib.query(q);
            for c in hits.iter().take(20) {
                println!(
                    "{}\t{}\t{}\t{}",
                    c.entry.title,
                    c.entry.index_text(),
                    c.entry.start,
                    c.entry.path.display()
                );
            }
        }
        None => {
            eprintln!("usage: zoro <query|html <query>|index>");
            std::process::exit(0);
        }
    }
}
