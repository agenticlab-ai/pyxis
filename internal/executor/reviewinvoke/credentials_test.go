package reviewinvoke

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestReviewFetchUsesParentCredentialsWithoutCopyingThem(t *testing.T) {
	// Keep the ambient Git identity empty. Only the original repository has
	// the URL-scoped credential helper needed by this private HTTP remote.
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "absent"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GIT_ASKPASS", "")
	upstream := t.TempDir()
	runGit(t, upstream, "init", "-q", "--initial-branch=main")
	runGit(t, upstream, "config", "user.email", "t@pyxis.invalid")
	runGit(t, upstream, "config", "user.name", "t")
	writeFile(t, filepath.Join(upstream, "subject.txt"), "base\n")
	runGit(t, upstream, "add", "subject.txt")
	runGit(t, upstream, "commit", "-q", "-m", "base")
	parent := filepath.Join(t.TempDir(), "parent")
	if out, err := exec.Command("git", "clone", "-q", upstream, parent).CombinedOutput(); err != nil {
		t.Fatalf("clone parent: %v: %s", err, out)
	}
	before := strings.TrimSpace(string(mustGit(t, parent, "rev-parse", "HEAD")))
	writeFile(t, filepath.Join(upstream, "subject.txt"), "reviewed\n")
	runGit(t, upstream, "commit", "-qam", "reviewed revision")
	revision := strings.TrimSpace(string(mustGit(t, upstream, "rev-parse", "HEAD")))
	runGit(t, upstream, "update-server-info")
	var authenticated atomic.Int32
	files := http.FileServer(http.Dir(filepath.Join(upstream, ".git")))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok || user != "selected" || password != "test-only-token" {
			w.Header().Set("WWW-Authenticate", `Basic realm="private-test"`)
			http.Error(w, "selected repository credential required", http.StatusUnauthorized)
			return
		}
		authenticated.Add(1)
		files.ServeHTTP(w, r)
	}))
	defer server.Close()
	runGit(t, parent, "remote", "set-url", "origin", server.URL)
	// An inline shell command is Git's native credential-helper protocol.
	// The fixture contains only synthetic credentials and is never copied.
	helper := "!f() { test \"$1\" = get && printf 'username=selected\\npassword=test-only-token\\n\\n'; }; f"
	runGit(t, parent, "config", "credential."+server.URL+".helper", helper)
	fetchHead := filepath.Join(parent, ".git", "FETCH_HEAD")
	writeFile(t, fetchHead, "retain unrelated fetch marker\n")
	inv := &Invoker{repoDir: parent, remote: "origin"}
	workdir, cleanup, err := inv.prepareReviewWorkspace(t.Context(), revision)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if authenticated.Load() == 0 {
		t.Fatal("review did not authenticate to private remote")
	}
	if got := strings.TrimSpace(string(mustGit(t, workdir, "rev-parse", "HEAD"))); got != revision {
		t.Fatalf("review HEAD=%s want %s", got, revision)
	}
	if got := strings.TrimSpace(string(mustGit(t, parent, "rev-parse", "HEAD"))); got != before {
		t.Fatal("review fetch moved parent branch")
	}
	if data, err := os.ReadFile(fetchHead); err != nil || string(data) != "retain unrelated fetch marker\n" {
		t.Fatalf("review fetch changed parent FETCH_HEAD: %q %v", data, err)
	}
	config, err := os.ReadFile(filepath.Join(workdir, ".git", "config"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(config), "credential") || strings.Contains(string(config), server.URL) || strings.Contains(string(config), "test-only-token") {
		t.Fatal("review clone retained remote credentials or URL")
	}
	if err := verifyReviewWorkspace(t.Context(), workdir, revision); err != nil {
		t.Fatal(err)
	}
	// A later revision unavailable locally still requires the selected remote's
	// credentials; refusal must stop before a reviewer receives a workspace.
	writeFile(t, filepath.Join(upstream, "subject.txt"), "later revision\n")
	runGit(t, upstream, "commit", "-qam", "later revision")
	later := strings.TrimSpace(string(mustGit(t, upstream, "rev-parse", "HEAD")))
	runGit(t, upstream, "update-server-info")
	runGit(t, parent, "config", "credential."+server.URL+".helper", "!f() { test \"$1\" = get && printf 'username=rejected\\npassword=wrong\\n\\n'; }; f")
	inv.remote = "" // Exercise the configured default remote too.
	if failedDir, failedCleanup, err := inv.prepareReviewWorkspace(t.Context(), later); err == nil {
		failedCleanup()
		t.Fatalf("unauthorized review fetch produced workspace %s", failedDir)
	} else if !strings.Contains(err.Error(), "fetch exact review revision") {
		t.Fatalf("unexpected authentication failure: %v", err)
	}
}
