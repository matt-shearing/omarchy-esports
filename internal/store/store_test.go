package store

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/contra/omarchy-esports/internal/config"
	"github.com/contra/omarchy-esports/internal/match"
	"github.com/contra/omarchy-esports/internal/spoiler"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// TestPublicFileNeverContainsResults is the load-bearing test for the whole
// spoiler design. It asserts on the raw bytes on disk rather than on the
// decoded struct, because the guarantee we actually make to the user is about
// the file: anything reading it — the UI, a log, `jq`, a backup — must not be
// able to find a result they have not revealed.
func TestPublicFileNeverContainsResults(t *testing.T) {
	st := newTestStore(t)

	finished := match.Match{
		ID:        "m1",
		State:     match.StateFinished,
		StartsAt:  time.Now().Add(-3 * time.Hour),
		Opponents: [2]match.Opponent{{Name: "Team Spirit"}, {Name: "Team Falcons"}},
		BestOf:    3,
		Score:     [2]int{2, 1},
		Winner:    1,
		VOD: &match.VOD{
			VideoID:   "abc123",
			Title:     "Team Spirit 2-1 Team Falcons | GRAND FINAL",
			Thumbnail: "https://i.ytimg.com/vi/abc123/hq.jpg",
			URL:       "https://www.youtube.com/watch?v=abc123",
		},
	}

	redacted := spoiler.RedactAll([]match.Match{finished}, config.SpoilerStrict, map[string]bool{})
	if err := st.SavePublic(Public{Matches: redacted, Spoilers: string(config.SpoilerStrict)}); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(st.PublicPath())
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)

	// Every fragment that would give the result away.
	for _, leak := range []string{"2-1", "GRAND FINAL", "hq.jpg", `"winner": 1`} {
		if strings.Contains(text, leak) {
			t.Errorf("public state leaked %q:\n%s", leak, text)
		}
	}
	// The fixture itself must survive, or the file is useless.
	for _, want := range []string{"Team Spirit", "Team Falcons", "abc123"} {
		if !strings.Contains(text, want) {
			t.Errorf("public state is missing %q, which is not a spoiler", want)
		}
	}
}

// TestPrivateFileKeepsResultsAndIsOwnerOnly confirms the daemon still knows
// the result, and that only the user can read it.
func TestPrivateFileKeepsResultsAndIsOwnerOnly(t *testing.T) {
	st := newTestStore(t)
	m := match.Match{ID: "m1", State: match.StateFinished, Score: [2]int{2, 1}, Winner: 1}
	if err := st.SavePrivate(Private{Matches: []match.Match{m}}); err != nil {
		t.Fatal(err)
	}

	fi, err := os.Stat(st.PrivatePath())
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("private state permissions are %o, want 600", perm)
	}

	got, err := st.LoadPrivate()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Matches) != 1 || got.Matches[0].Score != [2]int{2, 1} {
		t.Errorf("private state lost the result: %+v", got.Matches)
	}
}

// TestRevealRoundTrip covers the reveal set surviving a save/load cycle.
func TestRevealRoundTrip(t *testing.T) {
	st := newTestStore(t)
	if err := st.SetRevealed("m1", true); err != nil {
		t.Fatal(err)
	}
	p, err := st.LoadPrivate()
	if err != nil {
		t.Fatal(err)
	}
	if !p.Revealed["m1"] {
		t.Fatal("reveal was not persisted")
	}
	if err := st.SetRevealed("m1", false); err != nil {
		t.Fatal(err)
	}
	p, _ = st.LoadPrivate()
	if p.Revealed["m1"] {
		t.Error("un-reveal was not persisted")
	}
}

// TestCorruptPrivateStateRecovers ensures a truncated write does not brick the
// daemon on next start.
func TestCorruptPrivateStateRecovers(t *testing.T) {
	st := newTestStore(t)
	if err := os.WriteFile(st.PrivatePath(), []byte(`{"matches": [{"id"`), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := st.LoadPrivate()
	if err != nil {
		t.Fatalf("corrupt state should not be a fatal error: %v", err)
	}
	if p.Revealed == nil || p.Notified == nil || p.TournamentStreams == nil {
		t.Error("recovered state has nil maps, which would panic on write")
	}
}

// TestPublicIsSortedAndAttributed covers the ordering the UI relies on and the
// attribution Liquipedia's licence requires.
func TestPublicIsSortedAndAttributed(t *testing.T) {
	st := newTestStore(t)
	now := time.Now()
	in := []match.Match{
		{ID: "late", StartsAt: now.Add(3 * time.Hour)},
		{ID: "early", StartsAt: now.Add(time.Hour)},
		{ID: "mid", StartsAt: now.Add(2 * time.Hour)},
	}
	if err := st.SavePublic(Public{Matches: in}); err != nil {
		t.Fatal(err)
	}
	var got Public
	raw, _ := os.ReadFile(st.PublicPath())
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	want := []string{"early", "mid", "late"}
	for i, id := range want {
		if got.Matches[i].ID != id {
			t.Errorf("position %d is %q, want %q", i, got.Matches[i].ID, id)
		}
	}
	if got.Attribution == "" {
		t.Error("public state is missing the required Liquipedia attribution")
	}
	if got.Version != CurrentVersion {
		t.Errorf("version = %d, want %d", got.Version, CurrentVersion)
	}
}
