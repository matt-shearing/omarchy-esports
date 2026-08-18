// Package fuzzy provides the ranking used by team search.
//
// The same algorithm is mirrored in shared/Model.js for the app, which does
// its matching in-process so results appear as you type rather than after a
// round trip.
package fuzzy

import "strings"

// NoMatch is returned by Score when the query does not match at all.
const NoMatch = -1

// Score ranks how well target matches query. Higher is better.
//
// The ordering it produces, best first:
//
//	exact match            "navi"      -> "NAVI"
//	prefix                 "team sp"   -> "Team Spirit"
//	word prefix            "spirit"    -> "Team Spirit"
//	substring              "pirit"     -> "Team Spirit"
//	subsequence            "tsp"       -> "Team Spirit"
//
// Subsequence matching is what makes short abbreviations useful, but it also
// matches loosely, so it scores far below the others and is penalised by how
// spread out the matched characters are.
func Score(query, target string) int {
	q := strings.ToLower(strings.TrimSpace(query))
	t := strings.ToLower(strings.TrimSpace(target))
	if q == "" || t == "" {
		return NoMatch
	}
	switch {
	case q == t:
		return 1000
	case strings.HasPrefix(t, q):
		// Shorter targets are better: "G2" beats "G2 Academy" for "g2".
		return 800 - min(len(t)-len(q), 99)
	}
	if wordPrefix(t, q) {
		return 600 - min(len(t)-len(q), 99)
	}
	if strings.Contains(t, q) {
		return 400 - min(len(t)-len(q), 99)
	}
	if spread := subsequenceSpread(t, q); spread >= 0 {
		return 200 - min(spread, 199)
	}
	return NoMatch
}

// wordPrefix reports whether any word in target starts with query.
func wordPrefix(target, query string) bool {
	for _, w := range strings.FieldsFunc(target, func(r rune) bool {
		return r == ' ' || r == '.' || r == '-' || r == '_'
	}) {
		if strings.HasPrefix(w, query) {
			return true
		}
	}
	return false
}

// subsequenceSpread returns how many characters the match spans beyond the
// query length, or -1 when query is not a subsequence of target.
func subsequenceSpread(target, query string) int {
	if len(query) == 0 {
		return -1
	}
	ti, first, last := 0, -1, -1
	for qi := 0; qi < len(query); qi++ {
		found := false
		for ; ti < len(target); ti++ {
			if target[ti] == query[qi] {
				if first < 0 {
					first = ti
				}
				last = ti
				ti++
				found = true
				break
			}
		}
		if !found {
			return -1
		}
	}
	return (last - first + 1) - len(query)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Best returns the highest score across several candidate strings, which lets
// a team be found by either its canonical name or its ticker abbreviation.
func Best(query string, candidates ...string) int {
	best := NoMatch
	for _, c := range candidates {
		if s := Score(query, c); s > best {
			best = s
		}
	}
	return best
}
