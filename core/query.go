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
	Library string  `json:"library"`
	Title   string  `json:"title"`
	Index   string  `json:"index"`
	Path    string  `json:"path"`
	Start   int     `json:"start"`
	Score   float64 `json:"score"`
	// Matches holds highlight ranges into Index (byte offsets, half-open).
	// Frontends may highlight themselves; this is the core-computed default.
	Matches []MatchRange `json:"matches,omitempty"`
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
	out := make([]Candidate, 0, len(blocks))
	for i := range blocks {
		b := &blocks[i]
		text := b.IndexText()
		match, ok := matcher.Match(query, text)
		if !ok {
			continue
		}
		out = append(out, Candidate{
			Library: library,
			Title:   b.Title,
			Index:   text,
			Path:    b.Path,
			Start:   b.Start,
			Score:   match.Score,
			Matches: match.Ranges,
		})
	}
	// Deterministic: higher score first, then earlier block start.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Start < out[j].Start
	})
	return out
}
