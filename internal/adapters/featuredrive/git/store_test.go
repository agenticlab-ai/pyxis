package gitdrive

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
	"github.com/agenticlab-ai/pyxis/internal/featuredrive/conformancetest"
)

type fixture struct {
	repo, origin, sourceHead string
	binding                  Binding
	operator, owner, foreign Signer
	keys                     map[string]ed25519.PrivateKey
	store                    *Store
}

func git(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "core.hooksPath=/dev/null", "-C", repo}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_NO_REPLACE_OBJECTS=1", "GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@invalid", "GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@invalid")
	body, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, body)
	}
	return strings.TrimSpace(string(body))
}

func newFixture(t *testing.T, format string) *fixture {
	t.Helper()
	root := t.TempDir()
	f := &fixture{repo: filepath.Join(root, "work"), origin: filepath.Join(root, "origin.git"), keys: make(map[string]ed25519.PrivateKey)}
	template := templateFor(t, format)
	copyTemplate(t, template, root)
	f.sourceHead = template.sourceHead
	f.binding = Binding{Repository: "local-git:" + f.origin, FeatureID: "sample-feature", Genesis: template.genesis}
	for i, principal := range []Principal{{ID: "operator", Operator: true}, {ID: "machine-1", Owner: featuredrive.Owner{Domain: "local", ID: "machine-1"}}, {ID: "worker-1", Owner: featuredrive.Owner{Domain: "remote", ID: "worker-1"}}} {
		key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{byte(i + 1)}, ed25519.SeedSize))
		f.keys[principal.ID] = key
		principal.PublicKey = key.Public().(ed25519.PublicKey)
		f.binding.Principals = append(f.binding.Principals, principal)
		signer := Signer{ID: principal.ID, Sign: func(ctx context.Context, body []byte) ([]byte, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return ed25519.Sign(key, body), nil
		}}
		switch i {
		case 0:
			f.operator = signer
		case 1:
			f.owner = signer
		case 2:
			f.foreign = signer
		}
	}
	f.store = f.newStore(t, f.repo, f.operator)
	return f
}

func (f *fixture) newStore(t *testing.T, repo string, signer Signer) *Store {
	t.Helper()
	store, err := NewStore(repo, "origin", f.binding, signer)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func (f *fixture) initial() featuredrive.FeatureDrive {
	t := featuredrive.Transition{Kind: "create", NewVersion: 1}
	return featuredrive.FeatureDrive{Schema: featuredrive.Schema, Repository: f.binding.Repository, FeatureID: f.binding.FeatureID, PlanFingerprint: "sha256:frozen-plan", TargetMainRef: "refs/heads/main", IntegrationRef: "refs/heads/feature/sample-feature", Version: 1, Created: t, Last: t, Ownership: featuredrive.Ownership{State: featuredrive.Unowned}, Phase: featuredrive.Integrating}
}

func step(prior featuredrive.FeatureDrive, kind string) featuredrive.FeatureDrive {
	next := prior
	next.Version++
	next.Last = featuredrive.Transition{Kind: kind, PriorVersion: prior.Version, NewVersion: next.Version, PriorEpoch: prior.Ownership.Epoch, NewEpoch: next.Ownership.Epoch}
	if kind == "effect-intent" {
		next.Effects = append(append([]featuredrive.EffectIntent(nil), prior.Effects...), featuredrive.EffectIntent{ID: "effect-" + strconv.FormatUint(next.Version, 10), Kind: "dispatch", RequiredEpoch: prior.Ownership.Epoch, Status: featuredrive.Pending})
	}
	return next
}

func (f *fixture) assign(t *testing.T) featuredrive.FeatureDrive {
	t.Helper()
	if err := f.store.Create(t.Context(), f.initial()); err != nil {
		t.Fatal(err)
	}
	prior, err := f.store.Load(t.Context(), f.binding.FeatureID)
	if err != nil {
		t.Fatal(err)
	}
	next := step(prior, "ownership-assigned")
	next.Ownership = featuredrive.Ownership{State: featuredrive.Active, Domain: "local", OwnerID: "machine-1", Epoch: 1}
	next.Last.NewEpoch = 1
	if err := f.store.CAS(t.Context(), next.FeatureID, prior.Version, next); err != nil {
		t.Fatal(err)
	}
	got, err := f.store.Load(t.Context(), next.FeatureID)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func (f *fixture) unchangedSource(t *testing.T) {
	t.Helper()
	if git(t, f.repo, "rev-parse", "HEAD") != f.sourceHead || git(t, f.origin, "rev-parse", "refs/heads/main") != f.sourceHead || git(t, f.repo, "status", "--porcelain") != "" {
		t.Fatal("metadata operation changed source, mainline, or worktree")
	}
}

func TestGitStoreConsumerContract(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			conformancetest.RunStore(t, func(t *testing.T) conformancetest.StoreFixture {
				f := newFixture(t, format)
				t.Cleanup(func() { f.unchangedSource(t) })
				cold := func(t *testing.T, signer Signer) featuredrive.FeatureDriveStore {
					git(t, f.origin, "gc", "--prune=now")
					clone := filepath.Join(t.TempDir(), "cold")
					git(t, filepath.Dir(clone), "clone", "--quiet", f.origin, clone)
					return f.newStore(t, clone, signer)
				}
				return conformancetest.StoreFixture{Store: f.store, ExecutionStore: f.newStore(t, f.repo, f.owner), Initial: f.initial(), Owner: featuredrive.Owner{Domain: "local", ID: "machine-1"},
					Reopen:          func(t *testing.T) featuredrive.FeatureDriveStore { return cold(t, Signer{}) },
					ReopenExecution: func(t *testing.T) featuredrive.FeatureDriveStore { return cold(t, f.owner) }}
			})
		})
	}
}

func TestSignedStoreRecoversWholeDriveFromCloneAfterOriginGC(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			f := newFixture(t, format)
			prior := f.assign(t)
			owner := f.newStore(t, f.repo, f.owner)
			intent := step(prior, "effect-intent")
			intent.Effects = []featuredrive.EffectIntent{{ID: "effect-one", Kind: "request-task-proposal", Subject: "T1", InputDigest: "sha256:input", RequiredEpoch: 1, Status: featuredrive.Pending}}
			if err := owner.CAS(t.Context(), prior.FeatureID, prior.Version, intent); err != nil {
				t.Fatal(err)
			}
			retained, err := owner.Load(t.Context(), prior.FeatureID)
			if err != nil {
				t.Fatal(err)
			}
			f.unchangedSource(t)
			clone := filepath.Join(t.TempDir(), "clone")
			git(t, filepath.Dir(clone), "clone", "--quiet", f.origin, clone)
			if err := os.RemoveAll(f.repo); err != nil {
				t.Fatal(err)
			}
			git(t, f.origin, "gc", "--prune=now")
			cold := f.newStore(t, clone, f.owner)
			got, err := cold.Load(t.Context(), retained.FeatureID)
			if err != nil || !reflect.DeepEqual(got, retained) || got.Last.Actor != "machine-1" || !featuredrive.VerifyTransition(got.Last, f.keys["machine-1"].Public().(ed25519.PublicKey)) {
				t.Fatalf("cold reader lost authenticated whole state: %+v, %v", got, err)
			}
			completed := step(got, "effect-observed")
			completed.Effects[0].Status = featuredrive.Observed
			completed.Effects[0].Observation = &featuredrive.EvidenceRef{Kind: "local-proposal", ID: "retained-ref", Digest: "sha256:result"}
			if err := cold.CAS(t.Context(), got.FeatureID, got.Version, completed); err != nil {
				t.Fatal(err)
			}
			if git(t, f.origin, "rev-parse", "refs/heads/main") != f.sourceHead {
				t.Fatal("recovery altered mainline")
			}
		})
	}
}

func TestSignedStoreConcurrentCreatesAndCASHaveOneWinner(t *testing.T) {
	f := newFixture(t, "sha1")
	stores := []*Store{f.store, f.newStore(t, f.repo, f.operator)}
	compete := func(write func(*Store) error) {
		t.Helper()
		start := make(chan struct{})
		results := make(chan error, len(stores))
		var workers sync.WaitGroup
		for _, store := range stores {
			workers.Go(func() { <-start; results <- write(store) })
		}
		close(start)
		workers.Wait()
		close(results)
		won, lost := 0, 0
		for err := range results {
			if err == nil {
				won++
			} else if errors.Is(err, featuredrive.ErrConflict) {
				lost++
			} else {
				t.Fatalf("race returned a non-CAS failure: %v", err)
			}
		}
		if won != 1 || lost != 1 {
			t.Fatalf("race had %d winners and %d losers", won, lost)
		}
	}
	compete(func(store *Store) error { return store.Create(t.Context(), f.initial()) })
	prior, err := f.store.Load(t.Context(), f.binding.FeatureID)
	if err != nil {
		t.Fatal(err)
	}
	next := step(prior, "ownership-assigned")
	next.Ownership = featuredrive.Ownership{State: featuredrive.Active, Domain: "local", OwnerID: "machine-1", Epoch: 1}
	next.Last.NewEpoch = 1
	compete(func(store *Store) error { return store.CAS(t.Context(), prior.FeatureID, prior.Version, next) })
	f.unchangedSource(t)
}

func TestSignedStoreRefusesForeignOwnerReadOnlyAndChangedFrozenInputs(t *testing.T) {
	f := newFixture(t, "sha1")
	prior := f.assign(t)
	before := git(t, f.origin, "rev-parse", f.store.ref)
	for _, signer := range []Signer{f.foreign, {}} {
		if err := f.newStore(t, f.repo, signer).CAS(t.Context(), prior.FeatureID, prior.Version, step(prior, "effect-intent")); !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("unowned or read-only writer: %v", err)
		}
	}
	for _, alter := range []func(*featuredrive.FeatureDrive){
		func(d *featuredrive.FeatureDrive) { d.PlanFingerprint = "other-plan" },
		func(d *featuredrive.FeatureDrive) { d.TargetMainRef = "refs/heads/other" },
		func(d *featuredrive.FeatureDrive) { d.IntegrationRef = "refs/heads/other" },
		func(d *featuredrive.FeatureDrive) { d.Created.Signature = "forged" },
		func(d *featuredrive.FeatureDrive) { d.Version++ },
		func(d *featuredrive.FeatureDrive) { d.Last.Kind = "unknown-transition" },
		func(d *featuredrive.FeatureDrive) { d.Ownership.Epoch = 3; d.Last.NewEpoch = 3 },
	} {
		next := step(prior, "effect-intent")
		alter(&next)
		if err := f.store.CAS(t.Context(), prior.FeatureID, prior.Version, next); !errors.Is(err, featuredrive.ErrMalformed) {
			t.Fatalf("invalid frozen input or transition accepted: %v", err)
		}
	}
	if _, err := f.store.Load(t.Context(), "other-feature"); !errors.Is(err, featuredrive.ErrNotFound) {
		t.Fatalf("foreign feature lookup: %v", err)
	}
	if git(t, f.origin, "rev-parse", f.store.ref) != before {
		t.Fatal("refused writes changed the authoritative state")
	}
	f.unchangedSource(t)
}

func TestSignedStoreHumanHandoffFencesOldSigningOwner(t *testing.T) {
	f := newFixture(t, "sha1")
	prior := f.assign(t)
	quiescing := step(prior, "handoff-quiescing")
	quiescing.Ownership.State = featuredrive.Quiescing
	if err := f.newStore(t, f.repo, f.owner).CAS(t.Context(), prior.FeatureID, prior.Version, quiescing); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("owner bypassed human handoff: %v", err)
	}
	if err := f.store.CAS(t.Context(), prior.FeatureID, prior.Version, quiescing); err != nil {
		t.Fatal(err)
	}
	prior, err := f.store.Load(t.Context(), prior.FeatureID)
	if err != nil {
		t.Fatal(err)
	}
	next := step(prior, "ownership-changed")
	next.Ownership = featuredrive.Ownership{State: featuredrive.Active, Domain: "remote", OwnerID: "worker-1", Epoch: 2}
	next.Last.NewEpoch = 2
	next.Evidence = []featuredrive.EvidenceRef{{Kind: "checkpoint", ID: "attempt-one", Digest: "sha256:checkpoint"}}
	if err := f.store.CAS(t.Context(), prior.FeatureID, prior.Version, next); err != nil {
		t.Fatal(err)
	}
	current, err := f.store.Load(t.Context(), prior.FeatureID)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.newStore(t, f.repo, f.owner).CAS(t.Context(), current.FeatureID, current.Version, step(current, "effect-intent")); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("old registered owner kept execution authority: %v", err)
	}
	if err := f.newStore(t, f.repo, f.foreign).CAS(t.Context(), current.FeatureID, current.Version, step(current, "effect-intent")); err != nil {
		t.Fatal(err)
	}
	f.unchangedSource(t)
}
