// Command fetchteam saves a rendered Liquipedia page for use as a test fixture.
//
// Team page templates vary by wiki and by team — some use generic headings
// ("Player Roster" then "Active"), others name the team ("Players of Team
// Spirit" then "Active Roster") — so the parser is only as good as the pages
// it has been tested against. This captures one.
//
//	go run ./tools/fetchteam dota2 Team_Falcons internal/liquipedia/testdata/x.html
//	gzip -9 internal/liquipedia/testdata/x.html
//
// It goes through the normal client, so it obeys the one-parse-per-30-seconds
// limit and writes to the same on-disk page cache. Stop the daemon before
// using it: two processes pacing themselves independently can still exceed the
// limit between them, and exceeding it has previously got the whole IP blocked.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/contra/omarchy-esports/internal/liquipedia"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: fetchteam <wiki> <Page_Title> <out.html>")
		os.Exit(2)
	}
	wiki, page, out := os.Args[1], os.Args[2], os.Args[3]

	c := liquipedia.New("0.1.0", "contra13@pm.me", os.Getenv("HOME")+"/.cache/omarchy-esports")
	html, err := c.ParsePage(context.Background(), wiki, page, 24*time.Hour)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fetch:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(out, []byte(html), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write:", err)
		os.Exit(1)
	}
	fmt.Printf("%s/%s -> %s (%d bytes)\n", wiki, page, out, len(html))
}
