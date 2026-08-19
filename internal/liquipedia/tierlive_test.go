package liquipedia

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestFetchTierWalksToParent hits the live API. It covers the case the ticker
// actually produces: a stage sub-page with no infobox of its own.
func TestFetchTierWalksToParent(t *testing.T) {
	if os.Getenv("LIVE") == "" {
		t.Skip("set LIVE=1 to run")
	}
	c := New("test", "matt@oneqode.com", t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// This is the page a ticker entry links to for TI matches.
	got, err := c.FetchTier(ctx, "dota2", "/dota2/The_International/2026/Main_Event")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Main_Event sub-page resolved to tier=%d type=%q", got.Tier, got.Type)
	if got.Tier != TierS {
		t.Errorf("tier = %d, want %d (should walk up to the parent tournament)", got.Tier, TierS)
	}
}
