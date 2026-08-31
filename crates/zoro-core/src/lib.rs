//! zoro-core: parse / index / query / render for zoro knowledge base.
//!
//! 核心只做内容层，不依赖任何 UI（无 fzf、无网络）。

pub mod index;
pub mod library;
pub mod model;
pub mod query;
pub mod render;
pub mod scan;

pub use library::Library;
pub use model::{Directive, Entry};
pub use scan::{scan_dir, scan_file};
