// Package notify sends desktop notifications through omarchy's notification
// helper, falling back to notify-send elsewhere.
package notify

import (
	"os/exec"
	"strings"
)

// Notification is one desktop message.
type Notification struct {
	Title string
	Body  string
	// Glyph is a Nerd Font icon shown by omarchy's notifier.
	Glyph string
	// Image is an optional path or URI, used for team artwork.
	Image string
	// Urgency is "low", "normal" or "critical".
	Urgency string
	// Exec is a shell command run when the notification is clicked, used to
	// open a stream or VOD straight from the popup.
	Exec string
	// AppName groups notifications in the notification centre.
	AppName string
}

// Sender delivers notifications.
type Sender struct {
	// helper is the omarchy notifier path; empty means fall back to notify-send.
	helper string
	// DryRun prints instead of sending, for `--dry-run` daemon runs.
	DryRun bool
	// Log receives a line per notification, for observability.
	Log func(string)
}

// NewSender picks the best available notification backend.
func NewSender() *Sender {
	s := &Sender{}
	if p, err := exec.LookPath("omarchy-notification-send"); err == nil {
		s.helper = p
	}
	return s
}

// Send delivers n. Failures are returned but are never fatal to the caller:
// a missing notifier should not stop the daemon from tracking matches.
func (s *Sender) Send(n Notification) error {
	if n.Urgency == "" {
		n.Urgency = "normal"
	}
	if n.AppName == "" {
		n.AppName = "omarchy-esports"
	}
	if s.Log != nil {
		s.Log("notify: " + n.Title + " — " + n.Body)
	}
	if s.DryRun {
		return nil
	}

	var cmd *exec.Cmd
	if s.helper != "" {
		args := []string{"--app-name", n.AppName, "-u", n.Urgency}
		if n.Glyph != "" {
			args = append(args, "-g", n.Glyph)
		}
		if n.Image != "" {
			args = append(args, "--image", n.Image)
		}
		if n.Exec != "" {
			args = append(args, "--exec", n.Exec)
		}
		args = append(args, n.Title)
		if n.Body != "" {
			args = append(args, n.Body)
		}
		cmd = exec.Command(s.helper, args...)
	} else {
		args := []string{"-a", n.AppName, "-u", n.Urgency, n.Title}
		if n.Body != "" {
			args = append(args, n.Body)
		}
		cmd = exec.Command("notify-send", args...)
	}
	return cmd.Run()
}

// OpenCommand builds a shell command that opens a URL in the user's browser,
// preferring omarchy's launcher so the URL lands in the configured browser.
func OpenCommand(url string) string {
	if url == "" {
		return ""
	}
	if _, err := exec.LookPath("omarchy-launch-browser"); err == nil {
		return "omarchy-launch-browser " + shellQuote(url)
	}
	return "xdg-open " + shellQuote(url)
}

// shellQuote single-quotes a value for safe interpolation into a command.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
