package core

// ClassifyTag maps a registered `@<name>` tag to a TagKind.
// Unregistered tags degrade safely to KindUnknown(name).
func ClassifyTag(name string) TagKind {
	switch name {
	case "index":
		return KindIndex
	case "shell":
		return KindShell
	case "video":
		return KindVideo
	case "image":
		return KindImage
	default:
		return KindUnknown(name)
	}
}

// PayloadStyle describes how a tag's payload is extracted.
type PayloadStyle int

const (
	// PayloadText: plain body text until the next tag line / EOF.
	PayloadText PayloadStyle = iota
	// PayloadFence: the fenced code block immediately following the tag line.
	PayloadFence
)

// PayloadStyleOf returns the extraction strategy for a tag kind.
func PayloadStyleOf(kind TagKind) PayloadStyle {
	if kind == KindShell {
		return PayloadFence
	}
	return PayloadText
}
