package converge_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/converge"
	"github.com/agenticlab-ai/pyxis/internal/payload"
)

func TestAuthorityHealthRefusesPolicyAndParentLinks(t *testing.T) {
	for _, rel := range []string{".pyxis", ".pyxis/skills", ".pyxis/skills/argus", payload.AuthorityContractTarget} {
		t.Run(rel, func(t *testing.T) {
			r := newRelease(t, stdFiles())
			root, cfg := r.newOperand(t)
			cfg.Repo.Knobs.Argus = true
			linkPath := at(root, rel)
			if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.RemoveAll(linkPath); err != nil {
				t.Fatal(err)
			}
			outside := t.TempDir()
			target := outside
			if rel == payload.AuthorityContractTarget {
				target = filepath.Join(outside, "policy.md")
				writeFile(t, target, operatorPolicy("tier-2"), 0o600)
			} else {
				suffix := strings.TrimPrefix(payload.AuthorityContractTarget, rel+"/")
				writeFile(t, at(outside, suffix), operatorPolicy("tier-2"), 0o600)
			}
			if err := os.Symlink(target, linkPath); err != nil {
				t.Fatal(err)
			}
			before, externalBefore := snap(t, root), snap(t, outside)
			health, err := converge.Doctor(context.Background(), cfg)
			if err != nil || health.Healthy || !slices.ContainsFunc(health.Findings, func(f converge.HealthFinding) bool {
				return f.Name == payload.AuthorityContractTarget && strings.Contains(f.Detail, "never a symlink")
			}) {
				t.Fatalf("valid external policy made authority healthy: %+v, %v", health, err)
			}
			sameSnap(t, before, snap(t, root), "policy link diagnostic")
			sameSnap(t, externalBefore, snap(t, outside), "external policy diagnostic")
		})
	}
}

func TestDisabledArgusDiagnosesRetainedAuthorityWithoutDeleting(t *testing.T) {
	for _, kind := range []string{"file", "dangling link", "directory"} {
		t.Run(kind, func(t *testing.T) {
			r := newRelease(t, stdFiles())
			root, cfg := r.newOperand(t)
			if _, err := converge.Run(context.Background(), cfg, converge.Options{}); err != nil {
				t.Fatal(err)
			}
			policyPath := at(root, payload.AuthorityContractTarget)
			if err := os.MkdirAll(filepath.Dir(policyPath), 0o755); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "file":
				writeFile(t, policyPath, operatorPolicy("tier-2"), 0o600)
			case "dangling link":
				if err := os.Symlink(filepath.Join(t.TempDir(), "missing"), policyPath); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(policyPath, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			before := snap(t, root)
			health, err := converge.Doctor(context.Background(), cfg)
			if err != nil || health.Healthy || !slices.ContainsFunc(health.Findings, func(f converge.HealthFinding) bool {
				return f.Name == payload.AuthorityContractTarget && strings.Contains(f.Detail, "FR-16")
			}) {
				t.Fatalf("disabled Argus silently accepted retained policy: %+v, %v", health, err)
			}
			preview, err := converge.Run(context.Background(), cfg, converge.Options{Preview: true})
			if err != nil || !slices.ContainsFunc(preview.RefusedByName, func(f converge.Refusal) bool { return f.Path == payload.AuthorityContractTarget }) {
				t.Fatalf("disabled policy omitted from preview: %+v, %v", preview, err)
			}
			rep, err := converge.Run(context.Background(), cfg, converge.Options{Repair: true})
			if err == nil || !slices.ContainsFunc(rep.RefusedByName, func(f converge.Refusal) bool { return f.Path == payload.AuthorityContractTarget }) {
				t.Fatalf("disabled policy was not refused: %+v, %v", rep, err)
			}
			sameSnap(t, before, snap(t, root), "retained authority is operator-owned")
		})
	}
}
