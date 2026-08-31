//! zoro-core: parse / analyze / query / render for zoro knowledge base.
//!
//! 核心只做内容层，不依赖任何 UI（无 fzf、无网络）。

pub mod config;
pub mod index;
pub mod library;
pub mod meta;
pub mod model;
pub mod query;
pub mod registry;
pub mod render;
pub mod scan;
pub mod workspace;

pub use config::{LibraryConfig, LibrarySpec, WorkspaceConfig};
pub use library::Library;
pub use model::{Block, ShellBlock, TagKind};
pub use scan::{scan_dir, scan_dir_named, scan_file, slice_block};
pub use workspace::Workspace;
