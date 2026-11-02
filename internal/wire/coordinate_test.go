package wire

import "testing"

func TestParseCoordinate(t *testing.T) {
	cases := []struct {
		name, url, want string
		ok              bool
	}{
		{"https with .git", "https://github.com/octo/widget.git", "octo/widget", true},
		{"https without .git", "https://github.com/octo/widget", "octo/widget", true},
		{"ssh shorthand with .git", "git@github.com:octo/widget.git", "octo/widget", true},
		{"ssh shorthand without .git", "git@github.com:octo/widget", "octo/widget", true},
		{"ssh full url", "ssh://git@github.com/octo/widget.git", "octo/widget", true},
		{"non-github host", "https://gitlab.com/octo/widget.git", "", false},
		{"owner only, no name", "https://github.com/octo", "", false},
		{"empty", "", "", false},
		{"trailing slash truncates to empty name", "https://github.com/octo/", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseCoordinate(tc.url)
			if got != tc.want || ok != tc.ok {
				t.Errorf("ParseCoordinate(%q) = (%q, %v), want (%q, %v)", tc.url, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestValidCoordinate(t *testing.T) {
	valid := []string{
		"octo/widget",
		"octo-cat/widget_2.go",
		"a/b",
		"O.c-t_o/W.i-d_get",
	}
	for _, s := range valid {
		if !ValidCoordinate(s) {
			t.Errorf("ValidCoordinate(%q) = false, want true", s)
		}
	}
	invalid := []string{
		"",
		"noslash",
		"owner/name/extra",
		"/name",
		"owner/",
		"-owner/name",
		"owner/-name",
		"owner name/x",
		"owner/na me",
		"--upload-pack=/bin/sh/x",
		"owner/$(rm -rf /)",
		"owner;name/x",
	}
	for _, s := range invalid {
		if ValidCoordinate(s) {
			t.Errorf("ValidCoordinate(%q) = true, want false", s)
		}
	}
}

// TestRemoteURLRoundTripsThroughParseCoordinate proves RemoteURL and
// ParseCoordinate stay the one matched pair (AP-11): whatever RemoteURL
// materializes as a git remote, ParseCoordinate must resolve back to the
// exact same coordinate — the invariant plan init-hardening/001 T5's
// materialize-then-later-re-derive flow (stageRemote writes it, stageWiring's
// originSlug reads it back) depends on.
func TestRemoteURLRoundTripsThroughParseCoordinate(t *testing.T) {
	for _, slug := range []string{"octo/widget", "octo-cat/widget_2.go", "a/b"} {
		url := RemoteURL(slug)
		got, ok := ParseCoordinate(url)
		if !ok || got != slug {
			t.Errorf("ParseCoordinate(RemoteURL(%q)) = (%q, %v), want (%q, true)", slug, got, ok, slug)
		}
	}
}
