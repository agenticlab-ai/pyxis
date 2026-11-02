package gitsrc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGitAuthorshipKeepsConfiguredIdentityWithoutChangingEvidence(t *testing.T) {
	dir := newRepo(t)
	pin := git(t, dir, "rev-parse", "HEAD")
	configDir := t.TempDir()
	global := filepath.Join(configDir, "global")
	system := filepath.Join(configDir, "system")
	marker := filepath.Join(configDir, "executed")
	write(t, configDir, "global", "[user]\n\temail = global@example.invalid\n[core]\n\tfsmonitor = !touch "+marker+"\n")
	write(t, configDir, "system", "[user]\n\temail = system@example.invalid\n")
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_SYSTEM", system)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "0")
	r, err := Open(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, want string }{
		{"local", "m@example.com"},
		{"global", "global@example.invalid"},
		{"system", "system@example.invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "global" {
				git(t, dir, "config", "--local", "--unset", "user.email")
			}
			if tc.name == "system" {
				t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
			}
			if got := r.UserEmail(t.Context()); got != tc.want {
				t.Fatalf("identity = %q; want %q", got, tc.want)
			}
			files, err := r.Files(t.Context(), pin, []string{"README.md"})
			if err != nil || string(files["README.md"]) != "hi" {
				t.Fatalf("pinned evidence: %q, %v", files["README.md"], err)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("configuration executed an extension: %v", err)
			}
		})
	}
}
