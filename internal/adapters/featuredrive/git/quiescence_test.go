package gitdrive

import (
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
)

func TestSignedStoreRejectsMalformedQuiescenceOnWriteAndColdRead(t *testing.T) {
	t.Parallel()
	for _, format := range []string{"sha1", "sha256"} {
		for _, operation := range []string{"handoff", "cancel"} {
			for _, tc := range []struct {
				name string
				ref  featuredrive.EvidenceRef
			}{
				{"zero", featuredrive.EvidenceRef{}},
				{"missing kind", featuredrive.EvidenceRef{ID: "attempt-one"}},
				{"missing id", featuredrive.EvidenceRef{Kind: "checkpoint"}},
				{"blank", featuredrive.EvidenceRef{Kind: " ", ID: "\t"}},
			} {
				for _, route := range []string{"write", "cold replay"} {
					t.Run(format+"/"+operation+"/"+tc.name+"/"+route, func(t *testing.T) {
						t.Parallel()
						f := newFixture(t, format)
						kind := "handoff-quiescing"
						if operation == "cancel" {
							kind = "cancel-pending"
						}
						prior := f.revokeRouting(t, f.assign(t), kind)
						next := step(prior, "ownership-changed")
						next.Ownership = featuredrive.Ownership{State: featuredrive.Active, Domain: "remote", OwnerID: "worker-1", Epoch: prior.Ownership.Epoch + 1}
						if operation == "cancel" {
							next.Phase = featuredrive.Cancelled
							next.Ownership = featuredrive.Ownership{State: featuredrive.Released, Epoch: prior.Ownership.Epoch + 1}
						}
						next.Last.NewEpoch = next.Ownership.Epoch
						next.Evidence = append(append([]featuredrive.EvidenceRef(nil), prior.Evidence...), tc.ref)
						before := git(t, f.origin, "show-ref")
						if route == "write" {
							if err := f.store.CAS(t.Context(), prior.FeatureID, prior.Version, next); !errors.Is(err, featuredrive.ErrMalformed) {
								t.Errorf("malformed checkpoint authorized %s: %v", operation, err)
							}
							if git(t, f.origin, "show-ref") != before {
								t.Error("malformed checkpoint changed origin refs")
							}
							f.unchangedSource(t)
							return
						}
						// Bypass the writer with a correctly signed canonical record.
						// Cold replay must apply the same checkpoint-content predicate.
						record, parent, body := f.currentRecord(t)
						record.Parent, record.PreviousDigest, record.Drive = parent, contentDigest(body), next
						f.resign(t, &record, f.operator.ID)
						encoded, err := record.canonical()
						if err != nil {
							t.Fatal(err)
						}
						f.appendFiles(t, map[string][]byte{drivePath: encoded})
						if _, err := f.newStore(t, f.repo, Signer{}).Load(t.Context(), prior.FeatureID); !errors.Is(err, featuredrive.ErrMalformed) {
							t.Errorf("signed malformed %s checkpoint survived cold replay: %v", operation, err)
						}
						f.unchangedSource(t)
					})
				}
			}
		}
	}
}
