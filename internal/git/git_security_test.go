package git

import (
	"strings"
	"testing"
)

func TestSanitizeGitURL(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"https://github.com/o/r.git", "https://github.com/o/r.git"},
		{"https://token@github.com/o/r.git", "https://github.com/o/r.git"},
		{"https://user:secret@github.com/o/r.git", "https://github.com/o/r.git"},
		{"", ""},
		{"not a url", "not a url"},
	}
	for _, c := range cases {
		got := sanitizeGitURL(c.in)
		if got != c.want {
			t.Errorf("sanitizeGitURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestScrubGitOutput_MasksURLAuth(t *testing.T) {
	in := "fatal: could not read Username for 'https://x-access-token:ghp_abcDEF123@github.com'"
	out := scrubGitOutput(in)
	if strings.Contains(out, "ghp_abcDEF123") {
		t.Errorf("token not redacted: %q", out)
	}
	if !strings.Contains(out, "***:***@github.com") {
		t.Errorf("expected masked userinfo, got: %q", out)
	}
}

func TestScrubGitOutput_MasksTokenKeywords(t *testing.T) {
	cases := []string{
		"failure: token: ghp_secretValue123",
		"failure: TOKEN=ghp_secretValue123",
		"failure: password: hunter2",
		"failure: bearer abc.def.ghi",
	}
	for _, in := range cases {
		out := scrubGitOutput(in)
		if strings.Contains(out, "ghp_secretValue123") || strings.Contains(out, "hunter2") || strings.Contains(out, "abc.def.ghi") {
			t.Errorf("secret survived scrub: in=%q out=%q", in, out)
		}
	}
}

func TestScrubGitOutput_PreservesBenign(t *testing.T) {
	in := "Cloning into '/tmp/repo-foo'..."
	out := scrubGitOutput(in)
	if out != in {
		t.Errorf("benign output mutated: %q -> %q", in, out)
	}
}
