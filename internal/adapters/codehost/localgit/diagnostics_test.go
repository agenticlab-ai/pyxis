package localgit

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

func gitFixture(t *testing.T, push bool) Options {
	t.Helper()
	dir := t.TempDir()
	var err error
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	root, origin := filepath.Join(dir, "working tree"), filepath.Join(dir, "origin.git")
	fixtureGit(t, dir, "init", "--initial-branch=trunk", root)
	fixtureGit(t, dir, "init", "--bare", "--initial-branch=trunk", origin)
	fixtureGit(t, root, "config", "user.name", "Local fixture")
	fixtureGit(t, root, "config", "user.email", "local@example.test")
	for _, repo := range []string{root, origin} {
		fixtureGit(t, repo, "config", "gc.auto", "0")
		fixtureGit(t, repo, "config", "maintenance.auto", "false")
	}
	fixtureGit(t, root, "commit", "--allow-empty", "-m", "mainline fixture")
	fixtureGit(t, root, "remote", "add", "upstream", origin)
	if push {
		fixtureGit(t, root, "push", "upstream", "trunk")
	}
	return Options{Root: root, Origin: origin, Remote: "upstream", Mainline: "trunk"}
}

func fixtureGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestDiagnosticsUsesOriginAndNeverWritesOrContactsAHost(t *testing.T) {
	t.Parallel()
	o := gitFixture(t, true)
	old := fixtureGit(t, o.Root, "rev-parse", "HEAD")
	fixtureGit(t, o.Root, "commit", "--allow-empty", "-m", "advanced origin")
	fixtureGit(t, o.Root, "push", "upstream", "trunk")
	head := fixtureGit(t, o.Root, "rev-parse", "HEAD")
	fixtureGit(t, o.Root, "update-ref", "refs/remotes/upstream/trunk", old)
	before := fixtureGit(t, o.Root, "show-ref")
	originBefore := fixtureGit(t, o.Origin, "show-ref")
	var calls [][]string
	o.Run = func(ctx context.Context, root string, args ...string) ([]byte, error) {
		calls = append(calls, slices.Clone(args))
		if len(args) < 3 || args[0] != "-c" || args[1] != "protocol.allow=never" {
			t.Fatalf("transport restriction missing: %v", args)
		}
		switch args[2] {
		case "check-ref-format", "rev-parse", "for-each-ref":
		case "remote":
			if len(args) < 4 || args[3] != "get-url" {
				t.Fatalf("mutating remote operation: %v", args)
			}
		default:
			t.Fatalf("unapproved command: git %v", args)
		}
		return runGit(ctx, root, args...)
	}
	r, err := NewDiagnostics(o).Inspect(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if r.Origin != o.Origin || r.Remote != "upstream" || r.MainlineRef != "refs/heads/trunk" || r.MainlineOID != head {
		t.Fatalf("wrong configured origin/mainline: %+v", r)
	}
	if r.ProposalReady || !strings.Contains(r.ProposalRemedy, "operator lock landing are available") || !strings.Contains(r.ProposalRemedy, "execution and watch integration remain pending") || !r.Trust.SharedAccount || r.Trust.ApproverAuthenticated || r.Trust.MainlineProtected || r.Trust.IndependentOriginIdentity {
		t.Fatalf("unsupported assurance: %+v", r)
	}
	if r.SchemaVersion != "code-host-readiness/v1" || r.Provider != "local-git" || len(calls) != 6 {
		t.Fatalf("unexpected diagnostics/calls: %+v, %v", r, calls)
	}
	if fixtureGit(t, o.Root, "show-ref") != before || fixtureGit(t, o.Origin, "show-ref") != originBefore || fixtureGit(t, o.Root, "status", "--porcelain") != "" {
		t.Fatal("read-only diagnosis changed repository")
	}
	if err := codehost.RequireWorkflow(t.Context(), NewDiagnostics(o)); !errors.Is(err, codehost.ErrWorkflowUnavailable) {
		t.Fatalf("incomplete local workflow admitted: %v", err)
	}
}

func TestDiagnosticsEmptyOriginNamesSeparateFirstPush(t *testing.T) {
	t.Parallel()
	o := gitFixture(t, false)
	// A similarly prefixed ref must not stand in for configured mainline.
	head := fixtureGit(t, o.Root, "rev-parse", "HEAD")
	fixtureGit(t, o.Root, "push", "upstream", head+":refs/heads/trunk-other")
	r, err := NewDiagnostics(o).Inspect(t.Context())
	if err != nil || r.MainlineOID != "" || !strings.Contains(r.MainlineRemedy, "first configured-mainline push") || !strings.Contains(r.MainlineRemedy, "trunk") {
		t.Fatalf("wrong bootstrap diagnosis: %+v, %v", r, err)
	}
	if err := codehost.RequireWorkflow(t.Context(), NewDiagnostics(o)); !errors.Is(err, codehost.ErrOrigin) {
		t.Fatalf("missing mainline admitted: %v", err)
	}
}

func TestDiagnosticsCanonicalLocalPathsAndGitEnvironment(t *testing.T) {
	o := gitFixture(t, true)
	link := filepath.Join(filepath.Dir(o.Origin), "origin-link")
	if err := os.Symlink(o.Origin, link); err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, o.Root, "remote", "set-url", "upstream", (&url.URL{Scheme: "file", Host: "localhost", Path: link}).String())
	o.Origin = link
	// The process-global override must not redirect origin inspection.
	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "not-a-repository"))
	t.Setenv("GIT_WORK_TREE", t.TempDir())
	r, err := NewDiagnostics(o).Inspect(t.Context())
	if err != nil || r.Origin == link || r.MainlineOID == "" {
		t.Fatalf("canonical origin/environment: %+v, %v", r, err)
	}
	for _, value := range []string{o.Origin, "./origin-link", (&url.URL{Scheme: "file", Path: link}).String()} {
		p, err := localPath(filepath.Dir(o.Origin), value)
		if err != nil || p != r.Origin {
			t.Fatalf("local path %q: %q, %v", value, p, err)
		}
	}
}

func TestDiagnosticsAcceptsColonInsideExplicitLocalPaths(t *testing.T) {
	t.Parallel()
	o := gitFixture(t, true)
	origin := filepath.Join(filepath.Dir(o.Origin), "team:project.git")
	if err := os.Rename(o.Origin, origin); err != nil {
		t.Fatal(err)
	}
	o.Origin = origin
	for _, value := range []string{origin, "../team:project.git", (&url.URL{Scheme: "file", Path: origin}).String()} {
		fixtureGit(t, o.Root, "remote", "set-url", o.Remote, value)
		r, err := NewDiagnostics(o).Inspect(t.Context())
		if err != nil || r.Origin != origin || r.MainlineOID == "" {
			t.Fatalf("local colon path %q: %+v, %v", value, r, err)
		}
	}
	if _, err := localPath(filepath.Dir(origin), "team:project.git"); err == nil {
		t.Fatal("ambiguous scp-style remote accepted without an explicit local path")
	}
}

func TestDiagnosticsRefusesBadOriginsBeforeWorkflow(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"no root", "no origin", "no remote", "option remote", "control remote", "bad branch", "not working tree", "missing origin", "missing remote", "fetch mismatch", "push mismatch", "multiple fetch", "multiple push", "HTTPS", "SSH", "URL rewrite", "not bare"} {
		t.Run(name, func(t *testing.T) {
			o := gitFixture(t, true)
			other := filepath.Join(t.TempDir(), "other.git")
			fixtureGit(t, filepath.Dir(other), "init", "--bare", other)
			switch name {
			case "no root":
				o.Root = ""
			case "no origin":
				o.Origin = ""
			case "no remote":
				o.Remote = ""
			case "option remote":
				o.Remote = "--help"
			case "control remote":
				o.Remote = "bad\nremote"
			case "bad branch":
				o.Mainline = "bad..branch"
			case "not working tree":
				o.Root = o.Origin
			case "missing origin":
				o.Origin = filepath.Join(t.TempDir(), "absent.git")
			case "missing remote":
				o.Remote = "absent"
			case "fetch mismatch":
				fixtureGit(t, o.Root, "remote", "set-url", "upstream", other)
			case "push mismatch":
				fixtureGit(t, o.Root, "remote", "set-url", "--push", "upstream", other)
			case "multiple fetch":
				fixtureGit(t, o.Root, "remote", "set-url", "--add", "upstream", other)
			case "multiple push":
				fixtureGit(t, o.Root, "remote", "set-url", "--add", "--push", "upstream", o.Origin)
				fixtureGit(t, o.Root, "remote", "set-url", "--add", "--push", "upstream", other)
			case "HTTPS":
				fixtureGit(t, o.Root, "remote", "set-url", "upstream", "https://example.test/o/r.git")
			case "SSH":
				fixtureGit(t, o.Root, "remote", "set-url", "upstream", "git@example.test:o/r.git")
			case "URL rewrite":
				fixtureGit(t, o.Root, "config", "url.https://example.test/.insteadOf", o.Origin)
			case "not bare":
				o.Origin = o.Root
				fixtureGit(t, o.Root, "remote", "set-url", "upstream", o.Root)
			}
			_, err := NewDiagnostics(o).Inspect(t.Context())
			if !errors.Is(err, codehost.ErrOrigin) {
				t.Fatalf("bad origin admitted: %v", err)
			}
		})
	}
}

func TestDiagnosticsCancellationAndProbeFailures(t *testing.T) {
	t.Parallel()
	o := gitFixture(t, true)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	o.Run = func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("cancelled probe executed")
		return nil, nil
	}
	if _, err := NewDiagnostics(o).Inspect(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled probe: %v", err)
	}
	for _, name := range []string{"cancel during probe", "read ref failure", "bad response", "tag mainline", "short OID", "bare probe failure"} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			o.Run = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
				if name == "cancel during probe" {
					cancel()
					return nil, ctx.Err()
				}
				if args[2] == "for-each-ref" {
					switch name {
					case "read ref failure":
						return nil, fmt.Errorf("fixture read failure")
					case "bad response":
						return []byte("malformed"), nil
					case "tag mainline":
						return []byte("refs/heads/trunk " + strings.Repeat("a", 40) + " tag"), nil
					case "short OID":
						return []byte("refs/heads/trunk abc commit"), nil
					}
				}
				if name == "bare probe failure" && dir == o.Origin {
					return nil, fmt.Errorf("fixture bare failure")
				}
				return runGit(ctx, dir, args...)
			}
			_, err := NewDiagnostics(o).Inspect(ctx)
			if name == "cancel during probe" {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("lost cancellation: %v", err)
				}
			} else if !errors.Is(err, codehost.ErrOrigin) {
				t.Fatalf("failed probe admitted: %v", err)
			}
		})
	}
}

func TestLocalPathAndObjectValidation(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"", "bad\npath", "https://example.test/repo", "git@host:repo", "file:relative", "file://otherhost/tmp/repo", "file://user@localhost/tmp/repo", "file:///tmp/repo?query", "file:///tmp/repo#fragment", "file:///tmp/repo%00", "file://%bad/tmp/repo"} {
		if _, err := localPath(t.TempDir(), value); err == nil {
			t.Errorf("nonlocal path admitted: %q", value)
		}
	}
	for value, want := range map[string]bool{strings.Repeat("a", 40): true, strings.Repeat("0", 64): true, "abc": false, strings.Repeat("A", 40): false, strings.Repeat("g", 64): false} {
		if objectID(value) != want {
			t.Errorf("objectID %q: want %v", value, want)
		}
	}
}
