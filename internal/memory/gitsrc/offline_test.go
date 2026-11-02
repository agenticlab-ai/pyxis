package gitsrc

import (
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestOfflineReadsDoNotHydratePartialClones(t *testing.T) {
	// The control uses only a local file transport. It proves these exact
	// missing objects are fetchable, without needing a network service.
	t.Setenv("GIT_ALLOW_PROTOCOL", "file")
	t.Setenv("GIT_NO_LAZY_FETCH", "0")
	source := newRepo(t)
	git(t, source, "config", "uploadpack.allowFilter", "true")
	clone := filepath.Join(t.TempDir(), "partial")
	remote := (&url.URL{Scheme: "file", Path: source}).String()
	git(t, source, "clone", "-q", "--filter=blob:none", "--no-checkout", remote, clone)
	blob := git(t, source, "rev-parse", "HEAD:memory/policy.json")
	stored := func() bool {
		objects := git(t, clone, "cat-file", "--batch-all-objects", "--batch-check=%(objectname)")
		return slices.Contains(strings.Fields(objects), blob)
	}
	if stored() {
		t.Fatal("partial-clone fixture already has the blob")
	}
	r, err := Open(t.Context(), clone)
	if err != nil {
		t.Fatal(err)
	}
	files, _ := r.Files(t.Context(), "HEAD", []string{"memory/policy.json"}) // Missing content may be returned as absent or as a Git error.
	if len(files) != 0 || stored() {
		t.Error("offline read fetched an unavailable blob")
	}
	if _, _, err := (TreeSource{Repo: r, Ref: "HEAD", Prefix: "memory"}).Files(t.Context()); err == nil {
		t.Error("a tree with unavailable blobs was accepted as complete")
	}
	if stored() {
		t.Error("tree read fetched unavailable memory")
	}
	if got := git(t, clone, "cat-file", "blob", blob); got != `{"schema":1}` || !stored() {
		t.Fatalf("ordinary Git did not fetch the control blob: %q", got)
	}
}
