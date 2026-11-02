package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

func TestLocalGitRunRefusesBeforeClaimsAgentsAndHostedCalls(t *testing.T) {
	for _, state := range []string{"valid origin", "missing mainline", "mismatched origin", "invalid repository config"} {
		t.Run(state, func(t *testing.T) {
			f := newImplementNowE2EFixture(t, e2eFixtureOptions{})
			path := filepath.Join(f.repo, ".pyxis/config.yaml")
			body := fmt.Sprintf("schema_version: pyxis-config/v2\nsystem:\n  mode: source\nbranch: main\nremote: origin\nintegrations:\n  schema_version: pyxis-integrations/v1\n  bindings:\n    code_host:\n      provider: local-git\n      root: %s\n    tasks:\n      provider: carina\n  ci:\n    mode: local\n", f.origin)
			if state == "invalid repository config" {
				body += "paths:\n  unknown_repo_path: refused\n"
			}
			if err := os.WriteFile(path, []byte(body), 0644); err != nil {
				t.Fatal(err)
			}
			if state == "missing mainline" {
				f.gitBare("update-ref", "-d", "refs/heads/main")
			}
			if state == "mismatched origin" {
				other := filepath.Join(f.root, "other.git")
				f.gitAt(f.root, "init", "--bare", other)
				f.gitAt(f.repo, "remote", "set-url", "--push", "origin", other)
			}
			before := f.gitAt(f.repo, "show-ref")
			var out, diagnostic bytes.Buffer
			if exit := run([]string{"--repo", f.repo}, &out, &diagnostic); exit != 2 {
				t.Fatalf("local run admitted: exit=%d out=%s err=%s", exit, &out, &diagnostic)
			}
			want := "not available"
			switch state {
			case "missing mainline":
				want = "first configured-mainline push"
			case "mismatched origin":
				want = "differs from configured origin"
			case "invalid repository config":
				want = "unknown_repo_path"
			}
			if !strings.Contains(diagnostic.String(), want) {
				t.Fatalf("wrong pre-write refusal: %s", &diagnostic)
			}
			if f.gitAt(f.repo, "show-ref") != before {
				t.Fatal("local refusal changed refs")
			}
			for _, path := range []string{f.journal, filepath.Join(f.repo, "logs[1]"), filepath.Join(f.stateDir, "calls.jsonl")} {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("local refusal started a log/claim/host/agent: %s, %v", path, err)
				}
			}
			if state == "valid origin" {
				if _, err := reviewHostAdapter(f.repo, "ignored/hosted", nil); !errors.Is(err, codehost.ErrWorkflowUnavailable) {
					t.Fatalf("local review fell through to GitHub: %v", err)
				}
			}
		})
	}
}
