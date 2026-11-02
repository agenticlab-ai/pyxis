package converge_test

import (
	"bytes"
	"context"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/converge"
	"github.com/agenticlab-ai/pyxis/internal/payload"
	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
)

func TestArgusAuthorityHealthIsIndependentOfPayloadOwnership(t *testing.T) {
	for _, kind := range []string{"missing", "malformed", "directory", "promoted", "revoked", "disabled"} {
		t.Run(kind, func(t *testing.T) {
			r := newRelease(t, stdFiles()) // no live authority entry in this release
			root, cfg := r.newOperand(t)
			if _, err := converge.Run(context.Background(), cfg, converge.Options{}); err != nil {
				t.Fatal(err)
			}
			cfg.Repo.Knobs.Argus = kind != "disabled"
			policyPath := at(root, payload.AuthorityContractTarget)
			switch kind {
			case "malformed":
				writeFile(t, policyPath, []byte("invalid contract\n"), 0o600)
			case "directory":
				if err := os.MkdirAll(policyPath, 0o755); err != nil {
					t.Fatal(err)
				}
			case "promoted", "revoked":
				tier := "tier-2"
				if kind == "revoked" {
					tier = "revoked"
				}
				writeFile(t, policyPath, operatorPolicy(tier), 0o600)
				trackAuthority(t, root)
			}
			wantRefusal := kind == "missing" || kind == "malformed" || kind == "directory"
			before := snap(t, root)
			ctx := context.Background()
			health, err := converge.Doctor(ctx, cfg)
			found := slices.ContainsFunc(health.Findings, func(f converge.HealthFinding) bool {
				return f.Name == payload.AuthorityContractTarget && f.Severity == converge.SeverityFinding
			})
			if err != nil || found != wantRefusal || (wantRefusal && health.Healthy) {
				t.Fatalf("doctor authority health: %+v, %v", health, err)
			}
			preview, err := converge.Run(ctx, cfg, converge.Options{Preview: true})
			found = slices.ContainsFunc(preview.RefusedByName, func(f converge.Refusal) bool {
				return f.Path == payload.AuthorityContractTarget && f.Class == relmanifest.RepoOwned
			})
			if err != nil || found != wantRefusal || slices.Contains(actionPaths(preview.WouldWrite), payload.AuthorityContractTarget) {
				t.Fatalf("preview authority health: %+v, %v", preview, err)
			}
			sameSnap(t, before, snap(t, root), "doctor and preview authority health")
			writeFile(t, at(root, ".pyxis/templates/prd-template.md"), []byte("drift\n"), 0o644)
			repaired, err := converge.Run(ctx, cfg, converge.Options{Repair: true})
			found = slices.ContainsFunc(repaired.RefusedByName, func(f converge.Refusal) bool { return f.Path == payload.AuthorityContractTarget })
			if (err != nil) != wantRefusal || found != wantRefusal || !slices.Contains(actionPaths(repaired.Repaired), ".pyxis/templates/prd-template.md") {
				t.Fatalf("ordinary repair authority health: %+v, %v", repaired, err)
			}
			// Repair restores only the deliberately drifted template. Policy
			// content, mode and absence remain exactly as before.
			sameSnap(t, before, snap(t, root), "repair preserves repository authority")
		})
	}
}

func TestArgusAuthorityHealthWithoutCachedManifest(t *testing.T) {
	for _, source := range []bool{false, true} {
		t.Run(map[bool]string{false: "cold release", true: "source"}[source], func(t *testing.T) {
			r := newRelease(t, stdFiles())
			root, cfg := r.newOperand(t)
			cfg.Repo.Knobs.Argus = true
			if source {
				cfg.Repo.System.Mode = "source"
			} else if err := os.RemoveAll(at(root, ".pyxis/bin")); err != nil {
				t.Fatal(err)
			}
			before := snap(t, root)
			health, err := converge.Doctor(context.Background(), cfg)
			if err != nil || health.Healthy || !slices.ContainsFunc(health.Findings, func(f converge.HealthFinding) bool {
				return f.Name == payload.AuthorityContractTarget && f.Severity == converge.SeverityFinding
			}) {
				t.Fatalf("missing enabled authority passed doctor: %+v, %v", health, err)
			}
			sameSnap(t, before, snap(t, root), "authority health without a manifest")
		})
	}
}

func TestLegacyPayloadAuthorityMatchRefusesWithoutMigration(t *testing.T) {
	r := legacyAuthorityRelease(t)
	root, cfg := r.newOperand(t)
	verifiedArchive(t, context.Background(), cfg)
	if err := resolve.PreWarm(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	cfg.Repo.Knobs.Argus = true
	before := snap(t, root)
	ctx := context.Background()
	classified, err := converge.Drift(ctx, cfg)
	if err != nil || !slices.ContainsFunc(classified.Findings, func(f converge.Finding) bool {
		return f.Path == payload.AuthorityContractTarget && f.Class == relmanifest.RepoOwned && f.State == converge.StateInvalid && strings.Contains(f.Detail, "legacy release")
	}) {
		t.Fatalf("legacy payload authority was silently accepted: %+v, %v", classified, err)
	}
	health, err := converge.Doctor(ctx, cfg)
	if err != nil || health.Healthy || !slices.ContainsFunc(health.Findings, func(f converge.HealthFinding) bool { return f.Name == payload.AuthorityContractTarget }) {
		t.Fatalf("doctor accepted legacy payload authority: %+v, %v", health, err)
	}
	preview, err := converge.Run(ctx, cfg, converge.Options{Preview: true})
	if err != nil || !slices.ContainsFunc(preview.RefusedByName, func(f converge.Refusal) bool { return f.Path == payload.AuthorityContractTarget }) || slices.Contains(actionPaths(preview.WouldWrite), payload.AuthorityContractTarget) {
		t.Fatalf("preview did not refuse legacy payload authority: %+v, %v", preview, err)
	}
	for range 2 {
		repaired, err := converge.Run(ctx, cfg, converge.Options{Repair: true})
		if err == nil || !slices.ContainsFunc(repaired.RefusedByName, func(f converge.Refusal) bool { return f.Path == payload.AuthorityContractTarget }) {
			t.Fatalf("legacy policy was not refused: %+v, %v", repaired, err)
		}
		sameSnap(t, before, snap(t, root), "legacy authority refusal preserves policy")
	}
}

func TestColdDoctorRefusesUnverifiedLegacyAuthorityProvenance(t *testing.T) {
	for _, source := range []bool{false, true} {
		t.Run(map[bool]string{false: "fresh clone", true: "Source owns its policy"}[source], func(t *testing.T) {
			// A promoted, structurally valid payload policy reproduces the
			// unintended grant, not merely a malformed or absent policy.
			r := newRelease(t, append(stdFiles(), pf{path: payload.AuthorityContractTarget, b: operatorPolicy("tier-2"), class: relmanifest.EngineVerbatim}))
			root, cfg := r.newOperand(t)
			cfg.Repo.Knobs.Argus = true
			if source {
				cfg.Repo.System.Mode = "source"
			}
			trackAuthority(t, root)
			if err := os.RemoveAll(at(root, ".pyxis/bin")); err != nil {
				t.Fatal(err)
			}
			before := snap(t, root)
			health, err := converge.Doctor(context.Background(), cfg)
			found := slices.ContainsFunc(health.Findings, func(f converge.HealthFinding) bool {
				return f.Name == payload.AuthorityContractTarget && f.Severity == converge.SeverityFinding && strings.Contains(f.Detail, "provenance is unverified")
			})
			if err != nil || found == source || (!source && health.Healthy) {
				t.Fatalf("cold doctor falsely certified policy provenance: %+v, %v", health, err)
			}
			sameSnap(t, before, snap(t, root), "cold doctor provenance refusal")
		})
	}
}

func TestFixedReleaseRejectsKnownLeakedPolicyWithoutLegacyManifest(t *testing.T) {
	policy, err := os.ReadFile("testdata/legacy-authority.md")
	if err != nil {
		t.Fatal(err)
	}
	if hexOf(policy) != "2a656133db3a8f91bba81d912b479c332fb98c536f45849f1806b6acf55e0fab" {
		t.Fatal("historical fixture differs from the verified v0.1.5–v0.1.8 authority pin")
	}
	for _, cold := range []bool{false, true} {
		t.Run(map[bool]string{false: "upgraded warm cache", true: "upgraded fresh clone"}[cold], func(t *testing.T) {
			r := newRelease(t, stdFiles())
			root, cfg := r.newOperand(t)
			ctx := context.Background()
			if _, err := converge.Run(ctx, cfg, converge.Options{}); err != nil {
				t.Fatal(err)
			}
			m, err := resolve.EnsureManifest(ctx, cfg)
			if err != nil || slices.ContainsFunc(m.Payload.Files, func(f relmanifest.PayloadFile) bool { return f.Path == payload.AuthorityContractTarget }) {
				t.Fatalf("fixed-release fixture still lists authority: %v", err)
			}
			policyPath := at(root, payload.AuthorityContractTarget)
			writeFile(t, policyPath, policy, 0o600)
			cfg.Repo.Knobs.Argus = true
			if cold {
				if err := os.RemoveAll(at(root, ".pyxis/bin")); err != nil {
					t.Fatal(err)
				}
			}
			before := snap(t, root)
			health, err := converge.Doctor(ctx, cfg)
			if err != nil || health.Healthy || !slices.ContainsFunc(health.Findings, func(f converge.HealthFinding) bool {
				return f.Name == payload.AuthorityContractTarget && strings.Contains(f.Detail, "v0.1.5–v0.1.8")
			}) {
				t.Fatalf("upgraded doctor accepted the leaked policy: %+v, %v", health, err)
			}
			preview, err := converge.Run(ctx, cfg, converge.Options{Preview: true})
			if err != nil || !slices.ContainsFunc(preview.RefusedByName, func(f converge.Refusal) bool {
				return f.Path == payload.AuthorityContractTarget && strings.Contains(f.Reason, "v0.1.5–v0.1.8")
			}) {
				t.Fatalf("upgraded preview accepted the leaked policy: %+v, %v", preview, err)
			}
			sameSnap(t, before, snap(t, root), "upgraded policy diagnostics")
			for range 2 {
				repaired, err := converge.Run(ctx, cfg, converge.Options{Repair: true})
				if err == nil || !slices.ContainsFunc(repaired.RefusedByName, func(f converge.Refusal) bool { return f.Path == payload.AuthorityContractTarget }) {
					t.Fatalf("upgraded converge accepted leaked policy: %+v, %v", repaired, err)
				}
				actual, err := os.ReadFile(policyPath)
				if err != nil || !bytes.Equal(actual, policy) {
					t.Fatalf("leaked policy was migrated or revoked automatically: %q, %v", actual, err)
				}
				info, err := os.Stat(policyPath)
				if err != nil || info.Mode().Perm() != 0o600 {
					t.Fatalf("leaked policy mode changed: %v, %v", info, err)
				}
			}
			cfg.Repo.System.Mode = "source"
			trackAuthority(t, root)
			health, err = converge.Doctor(ctx, cfg)
			if err != nil || slices.ContainsFunc(health.Findings, func(f converge.HealthFinding) bool { return f.Name == payload.AuthorityContractTarget }) {
				t.Fatalf("Source's own policy was mistaken for a consumer leak: %+v, %v", health, err)
			}
		})
	}
}
