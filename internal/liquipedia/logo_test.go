package liquipedia

import "testing"

// TestCommonsFileURL checks the hash-bucket construction against real files
// whose URLs were confirmed to return HTTP 200.
func TestCommonsFileURL(t *testing.T) {
	cases := []struct{ file, want string }{
		{"G2 Esports 2020 full lightmode.png",
			"https://liquipedia.net/commons/images/2/27/G2_Esports_2020_full_lightmode.png"},
		{"Astralis 2020 full allmode.png",
			"https://liquipedia.net/commons/images/b/b5/Astralis_2020_full_allmode.png"},
		{"File:Astralis 2020 full allmode.png",
			"https://liquipedia.net/commons/images/b/b5/Astralis_2020_full_allmode.png"},
	}
	for _, c := range cases {
		if got := CommonsFileURL(c.file); got != c.want {
			t.Errorf("CommonsFileURL(%q)\n got %s\nwant %s", c.file, got, c.want)
		}
	}
	if CommonsFileURL("") != "" {
		t.Error("empty file should give an empty URL")
	}
}

func TestInfoboxFile(t *testing.T) {
	text := "{{Infobox team\n|name=Team Liquid\n|image=Team Liquid 2024 full lightmode.png\n" +
		"|imagedark=Team Liquid 2024 full darkmode.png\n}}"
	if got := infoboxFile(imageFieldRe, text); got != "Team Liquid 2024 full lightmode.png" {
		t.Errorf("light = %q", got)
	}
	if got := infoboxFile(imageDarkFieldRe, text); got != "Team Liquid 2024 full darkmode.png" {
		t.Errorf("dark = %q", got)
	}
	// Legacy infoboxes append an inline size after a second pipe.
	legacy := "{{Infobox team\n|image=aspera_logo.png|10px\n}}"
	if got := infoboxFile(imageFieldRe, legacy); got != "aspera_logo.png" {
		t.Errorf("legacy size spec not stripped: %q", got)
	}
}
