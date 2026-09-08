// Package core implements the content layer of zoro: parse / analyze / query / render.
//
// 核心只做内容层，不依赖任何 UI（无 fzf、无网络）。
package core

import "strings"

// TagKind is the semantic class of an `@<name>` tag (classified by registry).
//
// zoro 的统一内容模型：每个标签都产出一个 Block——
// 标签名 = 维度（kind），标签值 = 搜索词（terms），其后是负载（raw）。
type TagKind string

// Registered tag kinds. Unknown tags degrade safely to Unknown.
const (
	KindIndex TagKind = "index"
	KindShell TagKind = "shell"
	KindVideo TagKind = "video"
	KindImage TagKind = "image"
)

// KindUnknown builds the stable identifier for an unregistered tag.
func KindUnknown(name string) TagKind { return TagKind("unknown:" + name) }

// String returns the stable manifest/display identifier.
func (k TagKind) String() string { return string(k) }

// TagKindFromString parses a stable identifier back into a TagKind.
func TagKindFromString(s string) TagKind {
	switch s {
	case "index", "shell", "video", "image", "":
		return TagKind(s)
	default:
		if name, ok := strings.CutPrefix(s, "unknown:"); ok {
			return KindUnknown(name)
		}
		return KindUnknown(s)
	}
}

// IsShell reports whether the block is a terminal-execution block.
func (k TagKind) IsShell() bool { return k == KindShell }

// ShellBlock is the payload metadata of an `@shell` block.
type ShellBlock struct {
	// Lang is the fence info language, e.g. "bash".
	Lang string
	// Lines are the 1-based absolute source line numbers of each fence body line.
	Lines []int
}

// Block is zoro's smallest search / display unit.
//
// Identity = (Library, Path, Start).
type Block struct {
	Library string
	Title   string
	Kind    TagKind
	// Terms is the search surface (whitespace-split words after the tag line).
	Terms []string
	// Path is the library-relative content path.
	Path string
	// Start is the 1-based line number of the tag line.
	Start int
	// Raw is the payload text (runtime only; the manifest never stores it).
	Raw string
	// Shell is non-nil only for `@shell` blocks.
	Shell *ShellBlock
}

// IndexText returns the search-surface text (terms joined by spaces).
func (b *Block) IndexText() string { return strings.Join(b.Terms, " ") }
