package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/converge"
	"github.com/agenticlab-ai/pyxis/internal/gitcredential"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/upgrade"
)

func TestUpgradeSelectsArtifactBindingBeforeMachineRecord(t *testing.T) {
	for _, mode := range []string{"repository", "machine", "channel override", "invalid config", "invalid record"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			t.Chdir(root)
			configDir := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", configDir)
			if err := os.MkdirAll(filepath.Join(configDir, "pyxis"), 0700); err != nil {
				t.Fatal(err)
			}
			record := "PYXIS_RELEASE_PROVIDER=forgejo\nPYXIS_RELEASE_BASE_URL=https://machine.test/base\nPYXIS_RELEASE_REPOSITORY=owner/pyxis\nPYXIS_RELEASE_TOKEN_ENV=MACHINE_TOKEN\n"
			if mode == "invalid record" {
				record = strings.ReplaceAll(record, "https://machine.test/base", "bad")
			}
			if err := os.WriteFile(filepath.Join(configDir, "pyxis", "release.env"), []byte(record), 0600); err != nil {
				t.Fatal(err)
			}
			if mode == "repository" || mode == "channel override" || mode == "invalid config" {
				if err := os.Mkdir(filepath.Join(root, ".pyxis"), 0700); err != nil {
					t.Fatal(err)
				}
				body := "integrations:\n  schema_version: pyxis-integrations/v1\n  connections:\n    release:\n      provider: forgejo\n      base_url: https://repo.test/base\n      token_env: REPO_TOKEN\n  bindings:\n    artifacts:\n      connection: release\n      repository: owner/pyxis\n"
				if mode == "invalid config" {
					body = "integrations:\n  schema_version: broken\n"
				}
				if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			var got upgrade.Options
			called := false
			swap(t, &upgradeRun, func(_ context.Context, o upgrade.Options) (upgrade.Report, error) {
				got = o
				called = true
				return upgrade.Report{Preview: true, Current: "v1.0.0", Target: "v1.0.0"}, nil
			})
			args := []string{"--preview"}
			if mode == "channel override" {
				args = append(args, "--channel", "https://explicit.test")
			}
			var out, diagnostic bytes.Buffer
			exit := upgradeVerb(args, &out, &diagnostic)
			if strings.HasPrefix(mode, "invalid") {
				if exit == 0 || called {
					t.Fatalf("invalid source admitted exit=%d called=%v", exit, called)
				}
				return
			}
			if exit != 0 || !called {
				t.Fatalf("exit=%d called=%v err=%s", exit, called, &diagnostic)
			}
			if mode == "channel override" {
				if got.Release != nil || got.Channel != "https://explicit.test" {
					t.Fatalf("override=%+v", got)
				}
				return
			}
			wantURL, wantEnv := "https://machine.test/base", "MACHINE_TOKEN"
			if mode == "repository" {
				wantURL, wantEnv = "https://repo.test/base", "REPO_TOKEN"
			}
			if got.Release == nil || got.Release.Provider != integrations.Forgejo || got.Release.BaseURL != wantURL || got.Release.TokenEnv != wantEnv || got.Channel != "" {
				t.Fatalf("selected source=%+v", got)
			}
		})
	}
}

func TestConvergeReportsCredentialKeysWithoutValues(t *testing.T) {
	for _, mode := range []string{converge.ModePreview, converge.ModeRepair} {
		var out bytes.Buffer
		printConvergeReport(&out, converge.Report{Mode: mode, Credentials: []gitcredential.Change{{Key: "credential.https://forgejo.test/owner/repo.helper", Before: []string{"private-before"}, After: []string{"private-after"}}}})
		if !strings.Contains(out.String(), "repository Git key credential.https://forgejo.test/owner/repo.helper") || strings.Contains(out.String(), "private-") || strings.Contains(out.String(), "nothing —") {
			t.Fatalf("report=%s", &out)
		}
	}
}
