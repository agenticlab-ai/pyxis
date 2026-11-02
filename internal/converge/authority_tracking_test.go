package converge_test

import (
	"context"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/converge"
	"github.com/agenticlab-ai/pyxis/internal/payload"
)

func authorityGit(t *testing.T, root string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v (%s)", args, err, out)
	}
}

func trackAuthority(t *testing.T, root string) {
	t.Helper()
	gitInitRepo(t, root)
	gitHooksPathSet(t, root, ".githooks")
	authorityGit(t, root, "add", "--", payload.AuthorityContractTarget)
}

func TestAuthorityRemovedFromIndexIsUnhealthyWithoutPolicyWrites(t *testing.T) {
	r := newRelease(t, stdFiles())
	root, cfg := r.newOperand(t)
	writeFile(t, at(root, payload.AuthorityContractTarget), operatorPolicy("tier-2"), 0o600)
	trackAuthority(t, root)
	cfg.Repo.Knobs.Argus = true
	if _, err := converge.Run(context.Background(), cfg, converge.Options{}); err != nil {
		t.Fatal(err)
	}
	authorityGit(t, root, "rm", "--cached", "--", payload.AuthorityContractTarget)
	before := snap(t, root)
	health, err := converge.Doctor(context.Background(), cfg)
	if err != nil || health.Healthy || !slices.ContainsFunc(health.Findings, func(f converge.HealthFinding) bool {
		return f.Name == payload.AuthorityContractTarget && strings.Contains(f.Detail, "Git-tracked")
	}) {
		t.Fatalf("untracked authority was certified healthy: %+v, %v", health, err)
	}
	preview, err := converge.Run(context.Background(), cfg, converge.Options{Preview: true})
	if err != nil || !slices.ContainsFunc(preview.RefusedByName, func(f converge.Refusal) bool { return f.Path == payload.AuthorityContractTarget }) {
		t.Fatalf("preview accepted untracked authority: %+v, %v", preview, err)
	}
	refused, err := converge.Run(context.Background(), cfg, converge.Options{Repair: true})
	if err == nil || !slices.ContainsFunc(refused.RefusedByName, func(f converge.Refusal) bool { return f.Path == payload.AuthorityContractTarget }) {
		t.Fatalf("repair accepted untracked authority: %+v, %v", refused, err)
	}
	sameSnap(t, before, snap(t, root), "untracked-policy diagnostics and refusal")
}
