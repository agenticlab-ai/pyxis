package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/converge"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
)

func localGitCLIFixture(t *testing.T, push bool) (root, origin string) {
	t.Helper()
	dir := t.TempDir()
	root, origin = filepath.Join(dir, "operand"), filepath.Join(dir, "origin.git")
	localGitCLI(t, dir, "init", "--initial-branch=trunk", root)
	localGitCLI(t, dir, "init", "--bare", "--initial-branch=trunk", origin)
	localGitCLI(t, root, "config", "user.name", "CLI fixture")
	localGitCLI(t, root, "config", "user.email", "local@example.test")
	localGitCLI(t, root, "remote", "add", "upstream", origin)
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0755); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf("schema_version: pyxis-config/v2\nsystem:\n  mode: source\nbranch: trunk\nremote: upstream\nknobs:\n  tracker: carina\n  ci_mode: local\nintegrations:\n  schema_version: pyxis-integrations/v1\n  bindings:\n    code_host:\n      provider: local-git\n      root: %s\n    tasks:\n      provider: carina\n  ci:\n    mode: local\n", origin)
	if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	localGitCLI(t, root, "add", "--", ".pyxis/config.yaml")
	localGitCLI(t, root, "commit", "-m", "configured local origin")
	if push {
		localGitCLI(t, root, "push", "upstream", "trunk")
	}
	return root, origin
}

func localGitCLI(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestLocalGitDraftRefusesBeforeHostedCommandsOrWrites(t *testing.T) {
	for _, state := range []string{"valid origin", "missing mainline", "mismatched origin"} {
		t.Run(state, func(t *testing.T) {
			root, _ := localGitCLIFixture(t, state != "missing mainline")
			if state == "mismatched origin" {
				other := filepath.Join(t.TempDir(), "other.git")
				localGitCLI(t, filepath.Dir(other), "init", "--bare", other)
				localGitCLI(t, root, "remote", "set-url", "--push", "upstream", other)
			}
			swap(t, &draftRepoRoot, func() (string, error) { return root, nil })
			swap(t, &draftCommand, func(context.Context, string, []string, string, ...string) ([]byte, error) {
				t.Fatal("local mode entered legacy hosted/Git workflow")
				return nil, nil
			})
			before := localGitCLI(t, root, "show-ref")
			verbs := [][]string{{"status"}, {"open", "widget"}, {"prepare-lock", "widget"}, {"round", "widget"}, {"reopen", "widget"}, {"abandon", "widget", "--reason", "test"}}
			if state == "valid origin" {
				verbs = [][]string{{"checkpoint"}, {"notice", "widget"}}
			}
			for _, args := range verbs {
				var out, diagnostic bytes.Buffer
				exit := draftVerb(args, &out, &diagnostic)
				want := "not available"
				if state == "missing mainline" {
					want = "first configured-mainline push"
				}
				if state == "mismatched origin" {
					want = "differs from configured origin"
				}
				if exit != 1 || !strings.Contains(diagnostic.String(), want) {
					t.Fatalf("%v: exit=%d err=%s", args, exit, &diagnostic)
				}
			}
			if localGitCLI(t, root, "show-ref") != before || localGitCLI(t, root, "status", "--porcelain") != "" {
				t.Fatal("refusal changed checkout")
			}
			if _, err := os.Stat(filepath.Join(root, "drafts")); !os.IsNotExist(err) {
				t.Fatal("refusal created draft files")
			}
			var watchOut, watchDiagnostic bytes.Buffer
			if exit := watchVerb([]string{"--repo", root, "--plan", "docs/plans/not-created"}, &watchOut, &watchDiagnostic); exit != 2 || !strings.Contains(watchDiagnostic.String(), "code host:") {
				t.Fatalf("watch passed early admission: exit=%d out=%s err=%s", exit, &watchOut, &watchDiagnostic)
			}
			if _, err := os.Stat(filepath.Join(root, ".pyxis/logs")); !os.IsNotExist(err) {
				t.Fatal("local watch refusal created an execution log before origin admission")
			}
			cfg, defects := config.Load(root)
			if len(defects) != 0 {
				t.Fatal(defects)
			}
			_, _, provider, selected, err := configuredWatch(root, "ignored/hosted", "carina", root, cfg, planMap{}, false)
			if provider != "local-git" || !selected || err == nil {
				t.Fatalf("local watch fell through: %s, %v, %v", provider, selected, err)
			}
		})
	}
}

func TestLocalGitSelectionRejectsInvalidBindingAndPreservesHostedSelection(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		wantErr    bool
	}{
		{"legacy", "", false},
		{"hosted", "integrations:\n  schema_version: pyxis-integrations/v1\n  bindings:\n    code_host:\n      provider: github\n      repository: o/r\n", false},
		{"invalid schema", "integrations:\n  schema_version: unknown\n", true},
		{"invalid direct binding", "integrations:\n  schema_version: pyxis-integrations/v1\n  bindings:\n    code_host:\n      provider: local-git\n      root: /configured/origin.git\n      repository: o/r\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, ".pyxis"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte(tc.body), 0644); err != nil {
				t.Fatal(err)
			}
			probe, err := localCodeHostDiagnostics(root, config.Defaults())
			if (err != nil) != tc.wantErr || probe != nil {
				t.Fatalf("selection: probe=%T, err=%v", probe, err)
			}
		})
	}
	if err := codehost.RequireWorkflow(context.Background(), nil); err == nil {
		t.Fatal("uncomposed local path admitted")
	}
}

func TestLocalGitDoctorComposesSameOriginAndTrust(t *testing.T) {
	for _, push := range []bool{true, false} {
		t.Run(fmt.Sprint(push), func(t *testing.T) {
			root, origin := localGitCLIFixture(t, push)
			t.Chdir(root)
			called := false
			swap(t, &doctorRun, func(ctx context.Context, cfg resolve.Config) (converge.Health, error) {
				called = true
				if cfg.CodeHostDiagnostics == nil {
					t.Fatal("local adapter not composed")
				}
				r, err := cfg.CodeHostDiagnostics.Inspect(ctx)
				canonical, pathErr := filepath.EvalSymlinks(origin)
				if err != nil || pathErr != nil || r.Origin != canonical || r.Remote != "upstream" || r.MainlineRef != "refs/heads/trunk" || (r.MainlineOID != "") != push {
					t.Fatalf("wrong selected origin: %+v, %v, %v", r, err, pathErr)
				}
				if !r.Trust.SharedAccount || r.ProposalReady || r.Trust.ApproverAuthenticated || r.Trust.IndependentOriginIdentity || r.Trust.MainlineProtected {
					t.Fatalf("false readiness/assurance: %+v", r)
				}
				return converge.Doctor(ctx, cfg)
			})
			var out, diagnostic bytes.Buffer
			exit := doctorVerb(nil, &out, &diagnostic)
			if !called || !strings.Contains(out.String(), "code-host:") || !strings.Contains(out.String(), "unauthenticated") || !strings.Contains(out.String(), "copied origins") || !strings.Contains(out.String(), "execution and watch integration remain pending") {
				t.Fatalf("exit=%d out=%s err=%s", exit, &out, &diagnostic)
			}
			if !push && (exit != 1 || !strings.Contains(out.String(), "first configured-mainline push")) {
				t.Fatalf("missing mainline doctor exit=%d out=%s", exit, &out)
			}
		})
	}
}

func TestDoctorRendersPartialHealthAfterIntegrationCompositionFailure(t *testing.T) {
	root, _ := localGitCLIFixture(t, true)
	body, err := os.ReadFile(filepath.Join(root, ".pyxis/config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	body = []byte(strings.Replace(string(body), "provider: carina", "provider: invalid-tracker", 1))
	if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), body, 0644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	called := false
	swap(t, &doctorRun, func(ctx context.Context, cfg resolve.Config) (converge.Health, error) {
		called = true
		if cfg.CodeHostDiagnostics != nil {
			t.Fatal("malformed binding composed a local adapter")
		}
		return converge.Doctor(ctx, cfg)
	})
	var out, diagnostic bytes.Buffer
	if exit := doctorVerb(nil, &out, &diagnostic); exit != 1 || !called || !strings.Contains(out.String(), "integrations") || !strings.Contains(out.String(), "prerequisite:") {
		t.Fatalf("lost partial report: called=%t exit=%d out=%s err=%s", called, exit, &out, &diagnostic)
	}
}
