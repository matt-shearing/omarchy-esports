package store

import (
	"fmt"
	"os"
	"testing"
)

func TestZZProfileJSON(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveProfiles([]TeamProfile{{Name: "Empty Org", Wiki: "dota2"}}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(s.ProfilesPath())
	fmt.Printf("%s\n", b)
	fi, _ := os.Stat(s.ProfilesPath())
	fmt.Println("mode:", fi.Mode())
}
