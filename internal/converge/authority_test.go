package converge_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/converge"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
	"github.com/agenticlab-ai/pyxis/internal/payload"
	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
)

func legacyAuthorityRelease(t *testing.T) release {
	t.Helper()
	template, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(payload.AuthorityContractTemplate)))
	if err != nil {
		t.Fatal(err)
	}
	policy, err := payload.InitialAuthorityContract(template, "2026-10-01")
	if err != nil {
		t.Fatal(err)
	}
	// A valid earlier-release shape: its manifest explicitly pins the live
	// authority as engine-verbatim and its verified archive carries those bytes.
	return newRelease(t, append(stdFiles(), pf{path: payload.AuthorityContractTarget, b: policy, class: relmanifest.EngineVerbatim}))
}

func operatorPolicy(tier string) []byte {
	policy := fmt.Appendf(nil, "# Operand authority\n\n| Field | Value |\n|---|---|\n| **Current tier** | %s |\n| **Template version** | authority-contract/v1 |\n\n## The ladder\n\n| Tier | Bundle |\n|---|---|\n| revoked | ∅ |\n| floor | dispose-advisory-finding |\n| tier-2 | + dispose-blocking-finding · approve-merge |\n\n## Change history\n\n| Date | Move | Tier | Evidence |\n|---|---|---|---|\n| 2026-10-05 | none | floor | initialized at floor |\n| 2026-10-06 | promotion | tier-2 | operator-directed grant |\n", tier)
	if tier == "revoked" {
		policy = append(policy, "| 2026-10-07 | revocation | revoked | operator withdrew all delegated authority |\n"...)
	}
	return policy
}

func TestConvergePreservesAuthorityFromLegacyManifest(t *testing.T) {
	for _, kind := range []string{"promoted", "revoked", "missing", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			r := legacyAuthorityRelease(t)
			root, cfg := r.newOperand(t)
			cfg.Repo.Knobs.Argus = true
			ctx := context.Background()
			manifest, err := resolve.EnsureManifest(ctx, cfg)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.ContainsFunc(manifest.Payload.Files, func(file relmanifest.PayloadFile) bool {
				return file.Path == payload.AuthorityContractTarget && file.Class == relmanifest.EngineVerbatim && file.SHA256 != nil
			}) {
				t.Fatal("fixture does not reproduce the legacy engine-verbatim authority entry")
			}
			policy := operatorPolicy("tier-2")
			if kind == "revoked" {
				policy = operatorPolicy("revoked")
			}
			policyPath := at(root, payload.AuthorityContractTarget)
			if err := os.Remove(policyPath); err != nil {
				t.Fatal(err)
			}
			var linkTarget string
			switch kind {
			case "missing":
			case "symlink":
				linkTarget = filepath.Join(t.TempDir(), "policy.md")
				writeFile(t, linkTarget, policy, 0o600)
				if err := os.Symlink(linkTarget, policyPath); err != nil {
					t.Fatal(err)
				}
			default:
				writeFile(t, policyPath, policy, 0o600)
				trackAuthority(t, root)
			}
			if kind != "missing" {
				if defects := gate.CheckAuthorityContract(root); len(defects) != 0 {
					t.Fatalf("fixture operator policy is invalid: %v", defects)
				}
			}
			assertPreserved := func() {
				t.Helper()
				if kind == "missing" {
					if _, err := os.Lstat(policyPath); !os.IsNotExist(err) {
						t.Fatalf("convergence created authority from legacy metadata: %v", err)
					}
					return
				}
				if kind == "symlink" {
					if target, err := os.Readlink(policyPath); err != nil || target != linkTarget {
						t.Fatalf("convergence replaced the operator symlink: %q, %v", target, err)
					}
				}
				actual, err := os.ReadFile(policyPath)
				if err != nil || !bytes.Equal(actual, policy) {
					t.Fatalf("convergence replaced grants, revocations or history: %q, %v", actual, err)
				}
				info, err := os.Stat(policyPath)
				if err != nil || info.Mode().Perm() != 0o600 {
					t.Fatalf("convergence changed operator policy mode: %v, %v", info, err)
				}
			}
			writeFile(t, at(root, ".pyxis/templates/prd-template.md"), []byte("drift\n"), 0o644)
			preview, err := converge.Run(ctx, cfg, converge.Options{Preview: true})
			if err != nil || slices.Contains(actionPaths(preview.WouldWrite), payload.AuthorityContractTarget) {
				t.Fatalf("preview proposed authority replacement: %+v, %v", preview, err)
			}
			assertPreserved()
			for range 2 {
				writeFile(t, at(root, ".pyxis/templates/prd-template.md"), []byte("drift\n"), 0o644)
				repaired, err := converge.Run(ctx, cfg, converge.Options{Repair: true})
				wantRefusal := kind == "missing" || kind == "symlink"
				if (err != nil) != wantRefusal || slices.Contains(actionPaths(repaired.Repaired), payload.AuthorityContractTarget) || !slices.Contains(actionPaths(repaired.Repaired), ".pyxis/templates/prd-template.md") {
					t.Fatalf("repair did not preserve authority while repairing engine drift: %+v, %v", repaired, err)
				}
				assertPreserved()
				classified, err := converge.Drift(ctx, cfg)
				found := slices.ContainsFunc(classified.Findings, func(finding converge.Finding) bool {
					return finding.Path == payload.AuthorityContractTarget && finding.Class == relmanifest.RepoOwned
				})
				if err != nil || found != wantRefusal {
					t.Fatalf("shared drift predicate classified operator authority as engine drift: %+v, %v", classified, err)
				}
			}
		})
	}
}

func TestRepairPrimitivesRefuseAuthorityFromLegacyFindings(t *testing.T) {
	for _, class := range []relmanifest.Class{relmanifest.EngineVerbatim, relmanifest.Generated} {
		for _, present := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/present=%t", class, present), func(t *testing.T) {
				r := legacyAuthorityRelease(t)
				root, cfg := r.newOperand(t)
				ctx := context.Background()
				archive := verifiedArchive(t, ctx, cfg)
				policyPath := at(root, payload.AuthorityContractTarget)
				if err := os.Remove(policyPath); err != nil {
					t.Fatal(err)
				}
				policy := operatorPolicy("revoked")
				if present {
					writeFile(t, policyPath, policy, 0o600)
				}
				finding := []converge.Finding{{Path: payload.AuthorityContractTarget, Class: class, State: converge.StateModified, WantSHA256: hexOf(r.find(t, payload.AuthorityContractTarget).b)}}
				var actions []converge.Action
				var refusals []converge.Refusal
				var err error
				if class == relmanifest.EngineVerbatim {
					actions, refusals, err = converge.RepairEngineVerbatim(ctx, root, finding, archive)
				} else {
					actions, refusals, err = converge.WriteGenerated(ctx, root, finding, map[string][]byte{payload.AuthorityContractTarget: r.find(t, payload.AuthorityContractTarget).b})
				}
				if err != nil || len(actions) != 0 || len(refusals) != 1 || refusals[0].Path != payload.AuthorityContractTarget {
					t.Fatalf("explicit authority repair was not refused: %v, %v, %v", actions, refusals, err)
				}
				if present {
					actual, err := os.ReadFile(policyPath)
					if err != nil || !bytes.Equal(actual, policy) {
						t.Fatalf("repair primitive replaced revoked authority: %q, %v", actual, err)
					}
					info, err := os.Stat(policyPath)
					if err != nil || info.Mode().Perm() != 0o600 {
						t.Fatalf("repair primitive changed authority mode: %v, %v", info, err)
					}
				} else if _, err := os.Lstat(policyPath); !os.IsNotExist(err) {
					t.Fatalf("repair primitive created authority: %v", err)
				}
			})
		}
	}
}
