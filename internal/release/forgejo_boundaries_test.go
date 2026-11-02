package release

import (
	"fmt"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/wire"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestForgejoReleaseRoutingAndFailureBoundaries(t *testing.T) {
	bad := integrations.ResolvedRepository{Provider: integrations.Forgejo, BaseURL: "invalid", Repository: "o/r"}
	if err := downloadForgejoSet(t.Context(), bad, "v1", t.TempDir()); err == nil {
		t.Fatal("invalid download target accepted")
	}
	if _, err := refreshForgejoInstall(t.Context(), bad, "pins", "v1"); err == nil {
		t.Fatal("invalid write target accepted")
	}
	if _, err := cutTagCommit(t.Context(), CutOptions{SourceTarget: &bad}); err == nil {
		t.Fatal("invalid tag target accepted")
	}
	if err := cutCreateTag(t.Context(), CutOptions{SourceTarget: &bad}, "commit"); err == nil {
		t.Fatal("invalid tag create target accepted")
	}
	github := integrations.ResolvedRepository{Provider: integrations.GitHub, BaseURL: "https://github.com", Repository: "o/r", TokenEnv: "SOURCE_TOKEN"}
	scoped := githubRunner(wire.GHRunner{}, &github).(wire.GHRunner)
	if scoped.TokenEnv != "SOURCE_TOKEN" || scoped.Host != "github.com" {
		t.Fatal("GitHub runner not scoped")
	}
	fake := &coverageGHRunner{}
	if githubRunner(fake, &github) != fake {
		t.Fatal("overrode injected transport")
	}
	for _, tc := range []struct {
		source, dist *integrations.ResolvedRepository
		same         bool
	}{{&bad, nil, false}, {nil, &bad, false}, {&github, &github, true}, {&bad, &github, false}} {
		if got := sameMirrorTarget(MirrorOptions{SourceRepo: "o/r", DistRepo: "o/r", SourceTarget: tc.source, DistTarget: tc.dist}); got != tc.same {
			t.Fatalf("source identity comparison %v", got)
		}
	}
	status := 404
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status != 200 {
			w.WriteHeader(status)
			return
		}
		switch {
		case r.Method == "POST":
			w.WriteHeader(201)
		case r.URL.Path == "/api/v1/repos/o/r/tags/v1":
			if _, err := fmt.Fprint(w, `{"commit":{"sha":"abc"}}`); err != nil {
				t.Error(err)
			}
		case r.URL.Path == "/api/v1/repos/o/r/releases/tags/v1":
			if _, err := fmt.Fprint(w, `{"id":1}`); err != nil {
				t.Error(err)
			}
		case strings.HasSuffix(r.URL.Path, "/assets"):
			if _, err := fmt.Fprintf(w, `[{"name":"asset","browser_download_url":%q}]`, "http://"+r.Host+"/asset"); err != nil {
				t.Error(err)
			}
		default:
			if _, err := fmt.Fprint(w, `[]`); err != nil {
				t.Error(err)
			}
		}
	}))
	defer s.Close()
	target := integrations.ResolvedRepository{Provider: integrations.Forgejo, BaseURL: s.URL, Repository: "o/r"}
	opts := CutOptions{SourceTarget: &target, Version: "v1"}
	if sha, err := cutTagCommit(t.Context(), opts); err != nil || sha != "" {
		t.Fatalf("missing tag %s %v", sha, err)
	}
	if err := downloadForgejoSet(t.Context(), target, "v1", t.TempDir()); err == nil {
		t.Fatal("missing release accepted")
	}
	status = 403
	if _, err := cutTagCommit(t.Context(), opts); err == nil {
		t.Fatal("denied tag accepted")
	}
	if err := downloadForgejoSet(t.Context(), target, "v1", t.TempDir()); err == nil {
		t.Fatal("denied release accepted")
	}
	if _, err := pinsFromForgejo(t.Context(), target, "v1"); err == nil {
		t.Fatal("derived denied pins")
	}
	status = 200
	if sha, err := cutTagCommit(t.Context(), opts); err != nil || sha != "abc" {
		t.Fatalf("tag %s %v", sha, err)
	}
	if err := cutCreateTag(t.Context(), opts, "abc"); err != nil {
		t.Fatal(err)
	}
	if err := downloadForgejoSet(t.Context(), target, "v1", filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("download to missing dir accepted")
	}
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	if _, err := pinsFromForgejo(t.Context(), target, "v1"); err == nil {
		t.Fatal("staged pins without tempdir")
	}
}

func TestLegacyClosedSetRejectsMissingAndForeignArtifacts(t *testing.T) {
	dir := t.TempDir()
	if err := verifyLegacy(dir, "docgate", "docgate/v1.2.3"); err == nil {
		t.Fatal("missing sums accepted")
	}
	if err := verifyLegacy(dir, "other", "other/v1.2.3"); err == nil {
		t.Fatal("foreign tool accepted")
	}
	if err := os.WriteFile(filepath.Join(dir, SumsName), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifyLegacy(dir, "docgate", "docgate/v1.2.3"); err == nil {
		t.Fatal("empty artifact matrix accepted")
	}
	for _, p := range Platforms {
		if err := os.WriteFile(filepath.Join(dir, "docgate-v1.2.3-"+p), []byte(p), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := writeSums(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(dir, "docgate-v1.2.3-"+Platforms[0]), filepath.Join(dir, "foreign")); err != nil {
		t.Fatal(err)
	}
	if err := verifyLegacy(dir, "docgate", "docgate/v1.2.3"); err == nil {
		t.Fatal("foreign matrix accepted")
	}
}
