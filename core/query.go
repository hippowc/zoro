package core

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Candidate is a unified hit model across libraries.
//
// Three-way separation (aligned with fzf):
// match field (Index) != display field (Title) != payload ((Library, Path)+Start).
type Candidate struct {
	Library      string       `json:"library"`
	Title        string       `json:"title"`
	Index        string       `json:"index"`
	Path         string       `json:"path"`
	Start        int          `json:"start"`
	Score        float64      `json:"score"`
	Matches      []MatchRange `json:"matches,omitempty"`
	MatchedFaces []string     `json:"matched_faces,omitempty"`
}

// MatchRange is a highlighted span into the target text.
// Start is inclusive, End is exclusive (byte offsets into Candidate.Index).
type MatchRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// Match is the result of a Matcher.
type Match struct {
	Score float64
	// Ranges holds highlight intervals in target (byte offsets, half-open).
	Ranges []MatchRange
}

// Matcher scores a query against an index target.
//
// The default implementation keeps fzf-aligned semantics: whitespace-split
// terms, AND across terms, order-insensitive, in-term character subsequence,
// case-insensitive. It also returns highlight ranges for frontend display.
type Matcher interface {
	Match(query, target string) (Match, bool)
}

// DefaultMatcher is the core's built-in fuzzy matcher.
var DefaultMatcher Matcher = FuzzyMatcher{}

// FuzzyMatcher is a small, dependency-free fuzzy matcher.
type FuzzyMatcher struct{}

// Match implements Matcher. An empty query matches everything with score 0
// (browse-all). Each term must be a subsequence of target.
func (FuzzyMatcher) Match(query, target string) (Match, bool) {
	query = strings.TrimSpace(query)
	if query == "" {
		return Match{Score: 0}, true
	}

	terms := strings.Fields(query)
	runes := []rune(target)
	folded := foldRunes(runes)

	total := 0.0
	matched := make([]int, 0, len(runes))
	for _, term := range terms {
		termRunes := foldRunes([]rune(term))
		score, positions, ok := subsequenceMatch(termRunes, folded)
		if !ok {
			return Match{}, false
		}
		total += score
		matched = append(matched, positions...)
	}
	// Light preference for a more focused (shorter) index.
	total -= float64(len(folded)) / 4
	return Match{Score: total, Ranges: mergeMatchedRanges(matched, target)}, true
}

// foldRunes lowercases rune-by-rune, preserving the rune count.
func foldRunes(in []rune) []rune {
	out := make([]rune, len(in))
	for i, r := range in {
		out[i] = unicode.ToLower(r)
	}
	return out
}

// subsequenceMatch finds the highest-value subsequence of term in haystack
// and returns its score plus the matched haystack rune positions.
func subsequenceMatch(term, haystack []rune) (float64, []int, bool) {
	if len(term) == 0 {
		return 0, nil, true
	}
	var (
		ti      int
		last    = -1
		score   float64
		matched = make([]int, 0, len(term))
	)
	for hi, hc := range haystack {
		if hc != term[ti] {
			continue
		}
		if ti == 0 {
			// Earlier first match is better.
			score += 100 - float64(hi)
		}
		if last != -1 {
			if hi == last+1 {
				score += 50 // contiguous bonus
			} else {
				score -= float64(hi - last - 1) // gap penalty
			}
		}
		last = hi
		matched = append(matched, hi)
		ti++
		if ti == len(term) {
			return score, matched, true
		}
	}
	return 0, nil, false
}

// mergeMatchedRanges converts matched rune positions into coalesced byte ranges.
func mergeMatchedRanges(positions []int, target string) []MatchRange {
	if len(positions) == 0 {
		return nil
	}
	sort.Ints(positions)

	runes := []rune(target)
	bounds := make([]int, len(runes)+1)
	b := 0
	for i, r := range runes {
		bounds[i] = b
		b += utf8.RuneLen(r)
	}
	bounds[len(runes)] = b

	ranges := make([]MatchRange, 0, len(positions))
	for _, p := range positions {
		if p < 0 || p >= len(runes) {
			continue
		}
		start, end := bounds[p], bounds[p+1]
		if n := len(ranges); n > 0 && start <= ranges[n-1].End {
			if end > ranges[n-1].End {
				ranges[n-1].End = end
			}
			continue
		}
		ranges = append(ranges, MatchRange{Start: start, End: end})
	}
	return ranges
}

// Search runs the default matcher over a library's blocks.
func Search(blocks []Block, library, query string) []Candidate {
	return SearchWith(DefaultMatcher, blocks, library, query)
}

// SearchWith runs any Matcher, ranked by score desc then start asc.
func SearchWith(matcher Matcher, blocks []Block, library, query string) []Candidate {
	if matcher == nil {
		matcher = DefaultMatcher
	}
	// Use a single face with the given matcher, weight 1 (no scaling for old API).
	faces := []Face{&singleMatcherFace{m: matcher}}
	return SearchMultiFace(faces, blocks, library, query)
}

// singleMatcherFace wraps a Matcher for SearchWith backward compat.
type singleMatcherFace struct{ m Matcher }

func (f *singleMatcherFace) Name() string         { return "index" }
func (f *singleMatcherFace) Weight() float64      { return 1.0 }
func (f *singleMatcherFace) Text(b *Block) string { return b.IndexText() }
func (f *singleMatcherFace) Matcher() Matcher     { return f.m }

// SearchMultiFace runs all faces and merges results.
// Same block hit by multiple faces → one candidate with accumulated scores.
func SearchMultiFace(faces []Face, blocks []Block, library, query string) []Candidate {
	type blockKey struct {
		path  string
		start int
	}

	// Accumulate hits per block.
	acc := map[blockKey]*candidateAcc{}

	for i := range blocks {
		b := &blocks[i]
		for _, face := range faces {
			text := face.Text(b)
			m, ok := face.Matcher().Match(query, text)
			if !ok {
				continue
			}
			key := blockKey{b.Path, b.Start}
			a, exists := acc[key]
			if !exists {
				a = &candidateAcc{block: b}
				acc[key] = a
			}
			a.addHit(face.Name(), m.Score*face.Weight(), m.Ranges)
		}
	}

	out := make([]Candidate, 0, len(acc))
	for _, a := range acc {
		c := a.toCandidate(library)
		out = append(out, c)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Start < out[j].Start
	})
	return out
}

type candidateAcc struct {
	block     *Block
	score     float64
	faces     []string
	ranges    []MatchRange // from highest-scoring face
	bestScore float64
}

func (a *candidateAcc) addHit(face string, score float64, ranges []MatchRange) {
	a.score += score
	a.faces = append(a.faces, face)
	if score > a.bestScore {
		a.bestScore = score
		a.ranges = ranges
	}
}

func (a *candidateAcc) toCandidate(library string) Candidate {
	return Candidate{
		Library:      library,
		Title:        a.block.Title,
		Index:        a.block.IndexText(),
		Path:         a.block.Path,
		Start:        a.block.Start,
		Score:        a.score,
		Matches:      a.ranges,
		MatchedFaces: a.faces,
	}
}
