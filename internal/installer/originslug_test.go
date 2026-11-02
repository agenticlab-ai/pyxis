package installer

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

// TestOriginSlug_ResolvesConfiguredRemote proves originSlug derives owner/name
// from the CONFIGURED remote's URL, not a baked "origin" (config-driven-
// portability lift, FR-2, DINV-1). A repo whose sole remote is named "upstream"
// (no "origin") yields the slug only when the configured remote is threaded in; a
// blank remote falls back to the config schema's default "origin" (byte-identical
// to the old literal). It runs a real git repo, no network.
func TestOriginSlug_ResolvesConfiguredRemote(t *testing.T) {
	if config.Defaults().Remote != "origin" {
		t.Fatalf("config default remote is no longer byte-identical to the old literal (%q)", config.Defaults().Remote)
	}
	repo := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = os.Environ()
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	git("init", "-q")
	git("remote", "add", "upstream", "https://github.com/octo/widget.git")

	ctx := context.Background()
	cases := []struct {
		name, remote, want string
	}{
		{"configured non-default remote resolves its URL", "upstream", "octo/widget"},
		// A blank remote resolves the config default "origin", which this repo does
		// NOT have — so the slug is empty, proving the fallback targets "origin".
		{"blank falls back to origin (absent here)", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := originSlug(ctx, repo, tc.remote); got != tc.want {
				t.Errorf("originSlug(remote=%q) = %q, want %q", tc.remote, got, tc.want)
			}
		})
	}

	// And the default-remote path is honored end-to-end when the remote IS "origin"
	// (byte-identical to the pre-config behavior): a blank remote resolves it.
	git("remote", "add", "origin", "git@github.com:octo/other.git")
	if got := originSlug(ctx, repo, ""); got != "octo/other" {
		t.Errorf("blank remote with an origin present = %q, want octo/other (default resolution)", got)
	}
}
