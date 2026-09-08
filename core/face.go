package core

// Face is one search dimension over a block.
type Face interface {
	// Name is the unique identifier (e.g. "index", "title", "path", "raw").
	Name() string
	// Weight scales this face's contribution to the accumulated score.
	Weight() float64
	// Text extracts the searchable text from a block for this face.
	Text(b *Block) string
	// Matcher returns the matcher used by this face.
	Matcher() Matcher
}

// DefaultWeights for built-in faces.
const (
	DefaultWeightIndex = 10.0
	DefaultWeightTitle = 5.0
	DefaultWeightPath  = 3.0
	DefaultWeightRaw   = 1.0
)

// IndexFace matches over block.Terms (@tag search terms). Uses FuzzyMatcher.
type IndexFace struct{ W float64 }

func (f IndexFace) Name() string         { return "index" }
func (f IndexFace) Weight() float64      { return f.W }
func (f IndexFace) Text(b *Block) string { return b.IndexText() }
func (f IndexFace) Matcher() Matcher     { return DefaultMatcher }

// TitleFace matches over block.Title. Uses FuzzyMatcher.
type TitleFace struct{ W float64 }

func (f TitleFace) Name() string         { return "title" }
func (f TitleFace) Weight() float64      { return f.W }
func (f TitleFace) Text(b *Block) string { return b.Title }
func (f TitleFace) Matcher() Matcher     { return DefaultMatcher }

// PathFace matches over block.Path. Uses FuzzyMatcher.
type PathFace struct{ W float64 }

func (f PathFace) Name() string         { return "path" }
func (f PathFace) Weight() float64      { return f.W }
func (f PathFace) Text(b *Block) string { return b.Path }
func (f PathFace) Matcher() Matcher     { return DefaultMatcher }

// RawFace is a stub for P4 (bleve full-text search over block.Raw).
// It must not be activated until bleve is implemented; Name() is registered
// but the Matcher returns no match.
type RawFace struct{ W float64 }

func (f RawFace) Name() string         { return "raw" }
func (f RawFace) Weight() float64      { return f.W }
func (f RawFace) Text(b *Block) string { return b.Raw }
func (f RawFace) Matcher() Matcher     { return rawMatcher{} }

type rawMatcher struct{}

func (rawMatcher) Match(_, _ string) (Match, bool) { return Match{}, false }

// faceRegistry maps face names to factory functions.
var faceRegistry = map[string]func(weight float64) Face{
	"index": func(w float64) Face { return IndexFace{W: w} },
	"title": func(w float64) Face { return TitleFace{W: w} },
	"path":  func(w float64) Face { return PathFace{W: w} },
	"raw":   func(w float64) Face { return RawFace{W: w} },
}

// DefaultFaces returns the names of faces enabled by default.
func DefaultFaces() []string { return []string{"index", "title", "path"} }

// BuildFaces creates face instances from names and weights.
// Unknown names are silently skipped (forward compatibility).
// Missing weights fall back to defaults.
func BuildFaces(names []string, weights map[string]float64) []Face {
	defaults := map[string]float64{
		"index": DefaultWeightIndex,
		"title": DefaultWeightTitle,
		"path":  DefaultWeightPath,
		"raw":   DefaultWeightRaw,
	}
	var out []Face
	for _, name := range names {
		factory, ok := faceRegistry[name]
		if !ok {
			continue
		}
		w := defaults[name]
		if w2, ok := weights[name]; ok && w2 > 0 {
			w = w2
		}
		out = append(out, factory(w))
	}
	return out
}
