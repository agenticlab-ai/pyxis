package lease

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

const compatibilitySource = "github:https://github.com:o/r"

func compatibleLease(t *testing.T, remote string) *Lease {
	t.Helper()
	l, err := New(Config{WorkDir: newClone(t, remote), Remote: "origin", LegacySourceKey: compatibilitySource})
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestCompatibilityClaimContendsWithBothWorkerGenerations(t *testing.T) {
	for _, generation := range []string{"numeric", "scoped"} {
		for _, order := range []string{"compatible-first", "older-first", "concurrent"} {
			t.Run(generation+"/"+order, func(t *testing.T) {
				t.Parallel()
				remote := newRemote(t)
				modern := compatibleLease(t, remote)
				older := newLease(t, remote, "older")
				qualified := port.Task{ID: "T7", Number: 7, SourceKey: compatibilitySource}
				oldTask := qualified
				if generation == "numeric" {
					oldTask.SourceKey = ""
				}
				type result struct {
					claim port.Claim
					err   error
				}
				results := make(chan result, 2)
				acquire := func(l *Lease, task port.Task) {
					claim, err := l.Acquire(t.Context(), task)
					results <- result{claim, err}
				}
				switch order {
				case "compatible-first":
					acquire(modern, qualified)
					acquire(older, oldTask)
				case "older-first":
					acquire(older, oldTask)
					acquire(modern, qualified)
				case "concurrent":
					start := make(chan struct{})
					go func() { <-start; acquire(modern, qualified) }()
					go func() { <-start; acquire(older, oldTask) }()
					close(start)
				}
				var winner port.Claim
				for range 2 {
					outcome := <-results
					if outcome.err == nil {
						if winner != nil {
							t.Fatal("two worker generations acquired the same task")
						}
						winner = outcome.claim
					} else if !errors.Is(outcome.err, port.ErrClaimLost) {
						t.Fatal(outcome.err)
					}
				}
				if winner == nil {
					t.Fatal("neither worker won")
				}
				if err := winner.Release(t.Context()); err != nil {
					t.Fatal(err)
				}
				if refs := claimRefs(t, remote); len(refs) != 0 {
					t.Fatalf("losing transaction left a partial claim: %v", refs)
				}
				// A lost CAS must also clear its in-process reservations.
				retry, err := modern.Acquire(t.Context(), qualified)
				if err != nil {
					t.Fatal(err)
				}
				if refs := claimRefs(t, remote); len(refs) != 2 {
					t.Fatalf("compatibility claim must hold both namespaces: %v", refs)
				}
				if err := retry.Release(t.Context()); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestCompatibilityClaimsAllowUnrelatedIssuesAndPreserveForeignScope(t *testing.T) {
	remote := newRemote(t)
	l := compatibleLease(t, remote)
	older := newLease(t, remote, "legacy")
	legacy, err := older.Acquire(t.Context(), port.Task{ID: "old", Number: 1582})
	if err != nil {
		t.Fatal(err)
	}
	var claims []port.Claim
	for _, source := range []string{compatibilitySource, "github:https://elsewhere.example:o/r"} {
		claim, err := l.Acquire(t.Context(), port.Task{ID: "new", Number: 1492, SourceKey: source})
		if err != nil {
			t.Fatal(err)
		}
		claims = append(claims, claim)
	}
	if refs := claimRefs(t, remote); len(refs) != 4 {
		t.Fatalf("unexpected compatibility scope: %v", refs)
	}
	for _, claim := range claims {
		if err := claim.Release(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	if refs := claimRefs(t, remote); len(refs) != 1 || refs[0] != "refs/claim/1582" {
		t.Fatalf("unrelated legacy claim changed: %v", refs)
	}
	if err := legacy.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestCompatibilityClaimRequiresAtomicRemote(t *testing.T) {
	remote := newRemote(t)
	mustGit(t, remote, "config", "receive.advertiseAtomic", "false")
	l := compatibleLease(t, remote)
	task := port.Task{ID: "T7", Number: 7, SourceKey: compatibilitySource}
	if _, err := l.Acquire(t.Context(), task); err == nil || errors.Is(err, port.ErrClaimLost) || !strings.Contains(err.Error(), "atomic") {
		t.Fatalf("unsupported atomic push must fail closed: %v", err)
	}
	if refs := claimRefs(t, remote); len(refs) != 0 {
		t.Fatalf("non-atomic fallback created claims: %v", refs)
	}
	mustGit(t, remote, "config", "receive.advertiseAtomic", "true")
	claim, err := l.Acquire(t.Context(), task)
	if err != nil {
		t.Fatalf("failed push leaked local reservations: %v", err)
	}
	if err := claim.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestCompatibilityReleasePreservesReplacementHolder(t *testing.T) {
	for _, replaceNumeric := range []bool{true, false} {
		t.Run(strconv.FormatBool(replaceNumeric), func(t *testing.T) {
			t.Parallel()
			remote := newRemote(t)
			l := compatibleLease(t, remote)
			task := port.Task{ID: "T7", Number: 7, SourceKey: compatibilitySource}
			claim, err := l.Acquire(t.Context(), task)
			if err != nil {
				t.Fatal(err)
			}
			ref := "refs/claim/" + task.ScopeToken() + "/7"
			if replaceNumeric {
				ref = "refs/claim/7"
			}
			replacement, err := l.mintClaim(t.Context(), task)
			if err != nil {
				t.Fatal(err)
			}
			mustGit(t, l.workDir, "push", "--force", remote, replacement+":"+ref)
			if err := claim.Release(t.Context()); err != nil {
				t.Fatal(err)
			}
			if refs := claimRefs(t, remote); len(refs) != 1 || refs[0] != ref || mustGit(t, remote, "rev-parse", ref) != replacement {
				t.Fatalf("release changed replacement holder: %v", refs)
			}
		})
	}
}

func TestCompatibilityLocalReservationIsAllOrNothing(t *testing.T) {
	l := compatibleLease(t, newRemote(t))
	task := port.Task{ID: "T7", Number: 7}
	legacy, err := l.Acquire(t.Context(), task)
	if err != nil {
		t.Fatal(err)
	}
	task.SourceKey = compatibilitySource
	if _, err := l.Acquire(t.Context(), task); !errors.Is(err, port.ErrClaimLost) {
		t.Fatalf("local legacy claim did not block compatibility acquisition: %v", err)
	}
	if err := legacy.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
	claim, err := l.Acquire(t.Context(), task)
	if err != nil {
		t.Fatalf("lost local acquisition left a partial reservation: %v", err)
	}
	if err := claim.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
}
