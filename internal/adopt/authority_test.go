package adopt_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/adopt"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/payload"
)

const floorAuthority = `# Operand authority

| Field | Value |
|---|---|
| **Current tier** | floor |
| **Template version** | authority-contract/v1 |

## The ladder

| Tier | Bundle |
|---|---|
| revoked | ∅ |
| floor | dispose-advisory-finding |

## Change history

| Date | Move | Tier | Evidence |
|---|---|---|---|
| 2026-10-05 | none | floor | initialized at floor |
`

func writeArgusConfig(t *testing.T, root string, enabled bool) {
	t.Helper()
	writeFile(t, filepath.Join(root, ".pyxis/config.yaml"), fmt.Appendf(nil, "schema_version: pyxis-config/v2\nknobs:\n  argus: %t\n", enabled))
	if _, defects := config.Load(root); len(defects) != 0 {
		t.Fatalf("invalid fixture config: %v", defects)
	}
}

func TestInspectRefusesMissingArgusAuthorityBeforePlanWrite(t *testing.T) {
	r := newReleaseWithArchive(t, adoptFiles())
	root := nonEmptyAdopterRepo(t)
	writeArgusConfig(t, root, true)
	before := snapshot(t, root)
	_, err := adopt.Inspect(context.Background(), applyOpts(root, r))
	if err == nil || !errors.Is(err, adopt.ErrAuthoritySurfaceInvalid) || !strings.Contains(err.Error(), payload.AuthorityContractTarget) {
		t.Fatalf("inspection did not refuse missing authority by name: %v", err)
	}
	if !mapsEqual(before, snapshot(t, root)) {
		t.Fatal("refused inspection changed repository content")
	}
}

func TestApplyRechecksAuthorityBeforeAnyRepositoryWrite(t *testing.T) {
	for _, kind := range []string{"missing", "malformed", "legacy Source", "symlink", "disabled", "untracked"} {
		t.Run(kind, func(t *testing.T) {
			r := newReleaseWithArchive(t, adoptFiles())
			root := nonEmptyAdopterRepo(t)
			writeArgusConfig(t, root, true)
			policy := filepath.Join(root, filepath.FromSlash(payload.AuthorityContractTarget))
			writeFile(t, policy, []byte(floorAuthority))
			git(t, root, "add", "--", payload.AuthorityContractTarget)
			draftPlan(t, root, r)
			switch kind {
			case "missing":
				if err := os.Remove(policy); err != nil {
					t.Fatal(err)
				}
			case "malformed":
				writeFile(t, policy, []byte("invalid policy\n"))
			case "legacy Source":
				b, err := os.ReadFile("../converge/testdata/legacy-authority.md")
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, policy, b)
			case "symlink":
				outside := filepath.Join(t.TempDir(), "policy.md")
				writeFile(t, outside, []byte(floorAuthority))
				if err := os.Remove(policy); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, policy); err != nil {
					t.Fatal(err)
				}
			case "disabled":
				writeArgusConfig(t, root, false)
			case "untracked":
				git(t, root, "rm", "--cached", "--", payload.AuthorityContractTarget)
			}
			before := snapshot(t, root)
			hooksBefore, err := os.ReadFile(filepath.Join(root, ".git/config"))
			if err != nil {
				t.Fatal(err)
			}
			_, applyErr := adopt.Apply(context.Background(), applyOpts(root, r))
			if !mapsEqual(before, snapshot(t, root)) {
				t.Error("authority refusal happened after repository writes")
			}
			hooksAfter, err := os.ReadFile(filepath.Join(root, ".git/config"))
			if err != nil || !bytes.Equal(hooksBefore, hooksAfter) {
				t.Errorf("refused adoption changed Git hook configuration: %v", err)
			}
			if applyErr == nil || !errors.Is(applyErr, adopt.ErrAuthoritySurfaceInvalid) || !strings.Contains(applyErr.Error(), payload.AuthorityContractTarget) {
				t.Fatalf("apply did not refuse authority by name: %v", applyErr)
			}
		})
	}
}

func TestAdoptPreservesHealthyOperatorAuthority(t *testing.T) {
	r := newReleaseWithArchive(t, adoptFiles())
	root := nonEmptyAdopterRepo(t)
	writeArgusConfig(t, root, true)
	policy := filepath.Join(root, filepath.FromSlash(payload.AuthorityContractTarget))
	writeFile(t, policy, []byte(floorAuthority))
	git(t, root, "add", "--", payload.AuthorityContractTarget)
	draftPlan(t, root, r)
	if _, err := adopt.Apply(context.Background(), applyOpts(root, r)); err != nil {
		t.Fatal(err)
	}
	if _, err := adopt.Undo(context.Background(), applyOpts(root, r)); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(policy)
	if err != nil || string(b) != floorAuthority {
		t.Fatalf("adopt/undo changed operator policy: %q, %v", b, err)
	}
}

func TestUndoPreservesAuthorityFromLegacyPlan(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(fmt.Sprintf("missing=%t", missing), func(t *testing.T) {
			r := newReleaseWithArchive(t, adoptFiles())
			root := nonEmptyAdopterRepo(t)
			writeArgusConfig(t, root, true)
			policy := filepath.Join(root, filepath.FromSlash(payload.AuthorityContractTarget))
			writeFile(t, policy, []byte(floorAuthority))
			git(t, root, "add", "--", payload.AuthorityContractTarget)
			draftPlan(t, root, r)
			applied, err := adopt.Apply(context.Background(), applyOpts(root, r))
			if err != nil {
				t.Fatal(err)
			}
			// Older releases recorded live authority as engine-verbatim.
			legacy := applied.Plan
			legacy.Entries = append(legacy.Entries, adopt.Entry{Path: payload.AuthorityContractTarget, Class: adopt.ClassEngineVerbatim})
			b, err := legacy.Encode()
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, filepath.FromSlash(adopt.PlanPath)), b)
			if missing {
				if err := os.Remove(policy); err != nil {
					t.Fatal(err)
				}
			}
			rep, err := adopt.Undo(context.Background(), applyOpts(root, r))
			if err != nil || slices.Contains(rep.Removed, payload.AuthorityContractTarget) {
				t.Fatalf("undo treated live authority as engine-owned: %+v, %v", rep, err)
			}
			b, readErr := os.ReadFile(policy)
			if missing {
				if !os.IsNotExist(readErr) || slices.Contains(rep.Preserved, payload.AuthorityContractTarget) {
					t.Fatalf("undo changed missing-policy state: %+v, %v", rep, readErr)
				}
			} else if readErr != nil || string(b) != floorAuthority || !slices.Contains(rep.Preserved, payload.AuthorityContractTarget) {
				t.Fatalf("undo deleted or rewrote operator policy: %+v, %v", rep, readErr)
			}
		})
	}
}
