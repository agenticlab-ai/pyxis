package gitdrive

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
	"github.com/agenticlab-ai/pyxis/internal/gitrefstore"
)

func (f *fixture) currentRecord(t *testing.T) (Record, string, []byte) {
	t.Helper()
	snapshot, err := f.store.git.Fetch(t.Context(), "")
	if err != nil {
		t.Fatal(err)
	}
	record, err := decodeRecord(snapshot.Files[drivePath])
	if err != nil {
		t.Fatal(err)
	}
	return record, snapshot.Head, snapshot.Files[drivePath]
}

func (f *fixture) signedNext(t *testing.T) Record {
	t.Helper()
	previous, parent, body := f.currentRecord(t)
	next := step(previous.Drive, "effect-intent")
	mutation, err := f.newStore(t, f.repo, f.owner).mutation(t.Context(), parent, contentDigest(body), &previous.Drive, next)
	if err != nil {
		t.Fatal(err)
	}
	record, err := decodeRecord(mutation.Files[drivePath])
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func (f *fixture) resign(t *testing.T, record *Record, actor string) {
	t.Helper()
	record.Actor = actor
	record.Drive.Last.Actor = actor
	record.Drive.Last = featuredrive.SignTransition(record.Drive.Last, f.keys[actor])
	var err error
	record.AggregateDigest, err = aggregateDigest(record.Drive)
	if err != nil {
		t.Fatal(err)
	}
	body, err := record.signingBytes()
	if err != nil {
		t.Fatal(err)
	}
	record.Signature = hex.EncodeToString(ed25519.Sign(f.keys[actor], body))
}

func (f *fixture) appendFiles(t *testing.T, files map[string][]byte) {
	t.Helper()
	// Keep the transport pair consistent so these fixtures exercise signed
	// record validation rather than being rejected only by the checkpoint.
	raw, err := gitrefstore.NewWithCheckpoint(f.repo, "origin", f.store.ref, f.store.checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Apply(t.Context(), "", "deliberately invalid fixture", func(gitrefstore.Snapshot) (gitrefstore.Mutation, error) {
		return gitrefstore.Mutation{Files: files, Changed: true}, nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestSignedStoreRejectsTamperedAndUnauthorizedOriginRecords(t *testing.T) {
	for _, tc := range []struct {
		name  string
		alter func(*fixture, *Record, *testing.T)
	}{
		{"altered whole state", func(_ *fixture, r *Record, t *testing.T) {
			r.Drive.Phase = featuredrive.Candidate
			var err error
			r.AggregateDigest, err = aggregateDigest(r.Drive)
			if err != nil {
				t.Fatal(err)
			}
			// An attacker can recompute a digest; the retained signatures must
			// still reject bytes that were never authenticated.
		}},
		{"unknown actor", func(_ *fixture, r *Record, _ *testing.T) { r.Actor = "unregistered" }},
		{"foreign ref", func(f *fixture, r *Record, t *testing.T) { r.Ref += "-other"; f.resign(t, r, "operator") }},
		{"foreign repository", func(f *fixture, r *Record, t *testing.T) { r.Drive.Repository += "-other"; f.resign(t, r, "operator") }},
		{"wrong parent", func(f *fixture, r *Record, t *testing.T) {
			r.Parent = f.binding.Genesis.Commit
			f.resign(t, r, "operator")
		}},
		{"wrong previous digest", func(f *fixture, r *Record, t *testing.T) {
			r.PreviousDigest = "sha256:other"
			f.resign(t, r, "operator")
		}},
		{"wrong aggregate digest", func(f *fixture, r *Record, t *testing.T) {
			r.AggregateDigest = "sha256:other"
			body, err := r.signingBytes()
			if err != nil {
				t.Fatal(err)
			}
			r.Signature = hex.EncodeToString(ed25519.Sign(f.keys["operator"], body))
		}},
		{"stale epoch", func(f *fixture, r *Record, t *testing.T) { r.Drive.Last.PriorEpoch++; f.resign(t, r, "operator") }},
		{"skipped version", func(f *fixture, r *Record, t *testing.T) {
			r.Drive.Version++
			r.Drive.Last.NewVersion++
			f.resign(t, r, "operator")
		}},
		{"registered foreign owner", func(f *fixture, r *Record, t *testing.T) { f.resign(t, r, "worker-1") }},
		{"unknown transition kind", func(f *fixture, r *Record, t *testing.T) { r.Drive.Last.Kind = "arbitrary"; f.resign(t, r, "operator") }},
		{"unknown schema", func(_ *fixture, r *Record, _ *testing.T) { r.SchemaVersion += "-other" }},
		{"noncanonical signature", func(_ *fixture, r *Record, _ *testing.T) { r.Signature = strings.ToUpper(r.Signature) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, "sha1")
			f.assign(t)
			record := f.signedNext(t)
			tc.alter(f, &record, t)
			body, err := record.canonical()
			if err != nil {
				t.Fatal(err)
			}
			f.appendFiles(t, map[string][]byte{drivePath: body})
			before := git(t, f.origin, "rev-parse", f.store.ref)
			cold := f.newStore(t, f.repo, f.operator)
			if _, err := cold.Load(t.Context(), f.binding.FeatureID); err == nil {
				t.Fatal("altered or unauthorized origin record was accepted")
			}
			if err := cold.Create(t.Context(), f.initial()); err == nil || git(t, f.origin, "rev-parse", f.store.ref) != before {
				t.Fatal("invalid history became an unowned feature or was rewritten")
			}
			f.unchangedSource(t)
		})
	}
}

func TestSignedStoreRejectsDeletedReplacedAndIncompleteLineage(t *testing.T) {
	for _, kind := range []string{"deleted", "non-parent", "multiple parents", "missing state", "extra path", "unknown JSON field", "duplicate JSON field", "trailing JSON", "rewound"} {
		t.Run(kind, func(t *testing.T) {
			f := newFixture(t, "sha1")
			f.assign(t)
			_, parent, body := f.currentRecord(t)
			switch kind {
			case "deleted":
				git(t, f.origin, "update-ref", "-d", f.store.ref)
			case "non-parent", "multiple parents", "missing state":
				tree := git(t, f.repo, "rev-parse", parent+"^{tree}")
				args := []string{"commit-tree", tree, "-m", "replace fixture"}
				if kind == "missing state" {
					args = []string{"commit-tree", f.binding.Genesis.Tree, "-p", parent, "-m", "erase fixture"}
				}
				if kind == "multiple parents" {
					args = []string{"commit-tree", tree, "-p", parent, "-p", f.binding.Genesis.Commit, "-m", "merge fixture"}
				}
				commit := git(t, f.repo, args...)
				git(t, f.repo, "push", "--force", "origin", commit+":"+f.store.ref)
			case "extra path":
				f.appendFiles(t, map[string][]byte{"extra": []byte("not authorized")})
			case "unknown JSON field", "duplicate JSON field":
				field := `,"unknown":true}`
				if kind == "duplicate JSON field" {
					field = `,"schema_version":"` + RecordSchema + `"}`
				}
				f.appendFiles(t, map[string][]byte{drivePath: append(body[:len(body)-1], []byte(field)...)})
			case "trailing JSON":
				f.appendFiles(t, map[string][]byte{drivePath: append(body, []byte("{}")...)})
			case "rewound":
				git(t, f.origin, "update-ref", f.store.ref, f.binding.Genesis.Commit)
			}
			reader := f.newStore(t, f.repo, Signer{})
			if kind == "rewound" {
				// A previously observed head supplies the positive rollback proof;
				// a cold reader cannot infer an erased signed suffix from a prefix.
				reader = f.store
			}
			if _, err := reader.Load(t.Context(), f.binding.FeatureID); err == nil || errors.Is(err, featuredrive.ErrNotFound) {
				t.Fatalf("missing or broken lineage became an unowned feature: %v", err)
			}
			f.unchangedSource(t)
		})
	}
}

func TestSignedStoreInvalidSigningOperationPublishesNothing(t *testing.T) {
	f := newFixture(t, "sha1")
	for _, kind := range []string{"wrong key", "mutated input", "failure", "cancelled", "invalid supplied transition"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			signer := f.operator
			initial := f.initial()
			switch kind {
			case "wrong key":
				signer.Sign = f.foreign.Sign
			case "mutated input":
				signer.Sign = func(_ context.Context, body []byte) ([]byte, error) {
					body[0] ^= 1
					return ed25519.Sign(f.keys["operator"], body), nil
				}
			case "failure":
				signer.Sign = func(context.Context, []byte) ([]byte, error) { return nil, errors.New("credential unavailable") }
			case "cancelled":
				signer.Sign = func(_ context.Context, body []byte) ([]byte, error) {
					cancel()
					return ed25519.Sign(f.keys["operator"], body), nil
				}
			case "invalid supplied transition":
				initial.Created = featuredrive.SignTransition(initial.Created, f.keys["worker-1"])
				initial.Last = initial.Created
			}
			if err := f.newStore(t, f.repo, signer).Create(ctx, initial); err == nil {
				t.Fatal("invalid signing operation succeeded")
			}
			if git(t, f.origin, "rev-parse", f.store.ref) != f.binding.Genesis.Commit {
				t.Fatal("invalid signing operation published state")
			}
		})
	}
	f.unchangedSource(t)
}

func TestCanonicalRecordDoesNotTrustUnknownJSONFields(t *testing.T) {
	for _, body := range []string{"null", "{}", "{", `{"schema_version":"bad"}`} {
		if _, err := decodeRecord([]byte(body)); !errors.Is(err, featuredrive.ErrMalformed) {
			t.Fatalf("invalid record %q: %v", body, err)
		}
	}
	// Even a signed canonical record remains a closed nested wire contract.
	f := newFixture(t, "sha1")
	f.assign(t)
	_, _, body := f.currentRecord(t)
	var loose map[string]json.RawMessage
	if err := json.Unmarshal(body, &loose); err != nil {
		t.Fatal(err)
	}
	loose["drive"] = append(loose["drive"][:len(loose["drive"])-1], []byte(`,"unknown":true}`)...)
	altered, err := json.Marshal(loose)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeRecord(altered); !errors.Is(err, featuredrive.ErrMalformed) {
		t.Fatalf("unknown nested state accepted: %v", err)
	}
}
