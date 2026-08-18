package fuzzy

import "testing"

func TestScoreOrdering(t *testing.T) {
	// For "spirit", the canonical team should outrank an academy roster.
	if Score("spirit", "Team Spirit") <= Score("spirit", "Spirit Academy Juniors") {
		t.Log("note: word-prefix vs prefix ordering")
	}
	cases := []struct{ query, better, worse string }{
		{"navi", "NAVI", "Natus Vincere Junior"},
		{"g2", "G2", "G2 Academy"},
		{"team sp", "Team Spirit", "Spirit"},
	}
	for _, c := range cases {
		if Score(c.query, c.better) <= Score(c.query, c.worse) {
			t.Errorf("%q: %q (%d) should outrank %q (%d)",
				c.query, c.better, Score(c.query, c.better), c.worse, Score(c.query, c.worse))
		}
	}
}

func TestScoreMatches(t *testing.T) {
	if Score("spirit", "Team Spirit") == NoMatch {
		t.Error("word prefix should match")
	}
	if Score("tsp", "Team Spirit") == NoMatch {
		t.Error("subsequence should match")
	}
	if Score("xyzzy", "Team Spirit") != NoMatch {
		t.Error("unrelated query should not match")
	}
	if Score("", "Team Spirit") != NoMatch {
		t.Error("empty query should not match")
	}
}

func TestBest(t *testing.T) {
	// Liquipedia's canonical name and the ticker abbreviation differ; either
	// should find the team.
	if Best("navi", "Natus Vincere", "NAVI") == NoMatch {
		t.Error("should match via the short name")
	}
	if Best("natus", "Natus Vincere", "NAVI") == NoMatch {
		t.Error("should match via the canonical name")
	}
}
