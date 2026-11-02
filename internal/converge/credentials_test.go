package converge_test

import (
	"github.com/agenticlab-ai/pyxis/internal/converge"
	"github.com/agenticlab-ai/pyxis/internal/gitcredential"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestConvergePlansAndRecordsColdProfileGitCredentials(t *testing.T) {
	r := newRelease(t, stdFiles())
	root, cfg := r.newOperand(t)
	if out, err := exec.CommandContext(t.Context(), "git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	before, err := os.ReadFile(filepath.Join(root, ".git/config"))
	if err != nil {
		t.Fatal(err)
	}
	profile := integrations.Config{Connections: map[string]integrations.Connection{"local": {Provider: integrations.Forgejo, BaseURL: "http://127.0.0.1:3000"}}, Bindings: integrations.Bindings{CodeHost: integrations.Binding{Connection: "local", Repository: "o/r"}}, CI: integrations.CI{Mode: "local"}}
	cfg.Integrations = &profile
	preview, err := converge.Run(t.Context(), cfg, converge.Options{Preview: true, CredentialExecutable: "/usr/bin/false"})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Credentials) != 4 {
		t.Fatalf("planned credential changes: %v", preview.Credentials)
	}
	after, err := os.ReadFile(filepath.Join(root, ".git/config"))
	if err != nil || string(after) != string(before) {
		t.Fatal("preview wrote Git config")
	}
	rep, err := converge.Run(t.Context(), cfg, converge.Options{CredentialExecutable: "/usr/bin/false"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Credentials) != 4 || rep.Mode == converge.ModeNoop {
		t.Fatalf("credential mutation unreported: %+v", rep)
	}
	repeat, err := converge.Run(t.Context(), cfg, converge.Options{CredentialExecutable: "/usr/bin/false"})
	if err != nil || len(repeat.Credentials) != 0 {
		t.Fatalf("rerun: %+v %v", repeat, err)
	}
	if err := gitcredential.Restore(t.Context(), root, rep.Credentials); err != nil {
		t.Fatal(err)
	}
	for _, change := range rep.Credentials {
		out, err := exec.CommandContext(t.Context(), "git", "-C", root, "config", "--local", "--get-all", change.Key).CombinedOutput()
		if err == nil || len(out) > 0 {
			t.Fatal("credential rollback did not restore missing keys")
		}
	}
}
