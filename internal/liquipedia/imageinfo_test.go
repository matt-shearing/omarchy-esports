package liquipedia

import (
	"encoding/json"
	"os"
	"testing"
)

// FlyQuest's 2021 logo was renamed; the old name is now a redirect, and the
// URL built from its hash 404s. Resolution must land on the live file.
func TestResolveImageInfoFollowsRenames(t *testing.T) {
	raw, err := os.ReadFile("testdata/imageinfo-flyquest.json")
	if err != nil {
		t.Fatal(err)
	}
	var resp imageInfoResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatal(err)
	}
	got := ResolveImageInfo(resp, "FlyQuest_2021_full_allmode.png", "FlyQuest 2021 allmode.png", "")
	old := got["FlyQuest_2021_full_allmode.png"]
	if old == "" || old == CommonsFileURL("FlyQuest_2021_full_allmode.png") {
		t.Fatalf("renamed file not followed: %q", old)
	}
	if got["FlyQuest 2021 allmode.png"] == "" {
		t.Fatalf("plain file not resolved: %v", got)
	}
	if _, ok := got[""]; ok {
		t.Fatal("empty name should be skipped")
	}
}
