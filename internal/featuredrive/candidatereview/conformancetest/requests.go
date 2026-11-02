// Package conformancetest defines the final-proof recovery contract for
// production adapters independently of their storage medium.
package conformancetest

import (
	"reflect"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/featuredrive/candidatereview"
)

type Fixture struct {
	ID             string
	Candidate      codehost.FinalCandidate
	Scenarios      string
	Proposals      codehost.ScopedProposals
	Artifacts      codehost.ReviewArtifacts
	Requests       *candidatereview.Requests
	Receipts       codehost.Receipts
	Clone          func(*testing.T) Fixture
	RemoveCheckout func(*testing.T)
	Forget         func(*testing.T, codehost.ArtifactRef)
	State          func(*testing.T) string
	ChangePolicy   func(*testing.T)
}

func Run(t *testing.T, factory func(*testing.T) Fixture) {
	t.Helper()
	t.Run("complete proof and checkout-free recovery", func(t *testing.T) {
		f := factory(t)
		packet, err := f.Requests.Prepare(t.Context(), f.ID, f.Candidate, f.Scenarios)
		expect(t, err == nil, "prepare complete final review: %v", err)
		g, err := f.Proposals.Read(t.Context(), f.ID)
		expect(t, err == nil, "read prepared final generation: %v", err)
		before := f.State(t)
		for _, artifact := range []codehost.ReviewArtifact{packet.Evidence, packet.Request} {
			again, err := f.Artifacts.Retain(t.Context(), g, artifact.Kind, artifact.Body)
			expect(t, err == nil && reflect.DeepEqual(again, artifact), "idempotent retention changed whole proof: %v", err)
			read, err := f.Artifacts.ReadRetained(t.Context(), g, artifact.Kind, artifact.Identity)
			expect(t, err == nil && reflect.DeepEqual(read, artifact), "origin round trip changed whole proof: %v", err)
		}
		resumed, err := f.Requests.Prepare(t.Context(), f.ID, f.Candidate, f.Scenarios)
		expect(t, err == nil && reflect.DeepEqual(resumed, packet) && f.State(t) == before, "resume changed request or generation: %v", err)
		receipt, err := f.Receipts.Record(t.Context(), packet.Binding, "declared operator", "")
		expect(t, err == nil, "record exact final approval: %v", err)
		clone := f.Clone(t)
		f.RemoveCheckout(t)
		before = f.State(t)
		observed, err := clone.Requests.Observe(t.Context(), f.ID)
		expect(t, err == nil && reflect.DeepEqual(observed, packet), "clone lost complete candidate request: %v", err)
		verified, err := clone.Receipts.Verify(t.Context(), packet.Binding)
		expect(t, err == nil && verified == receipt, "clone lost exact final receipt: %v", err)
		expect(t, f.State(t) == before, "read-only recovery changed shared state")
	})
	for _, kind := range []string{codehost.CandidateEvidence, codehost.CandidateRequest} {
		t.Run("missing "+kind+" refuses progression", func(t *testing.T) {
			f := factory(t)
			packet, err := f.Requests.Prepare(t.Context(), f.ID, f.Candidate, f.Scenarios)
			expect(t, err == nil, "prepare final proof: %v", err)
			_, err = f.Receipts.Record(t.Context(), packet.Binding, "declared operator", "")
			expect(t, err == nil, "record final receipt: %v", err)
			ref := packet.Evidence.Identity
			if kind == codehost.CandidateRequest {
				ref = packet.Request.Identity
			}
			f.Forget(t, ref)
			before := f.State(t)
			_, err = f.Requests.Observe(t.Context(), f.ID)
			expect(t, err != nil, "missing proof was presented")
			_, err = f.Receipts.Verify(t.Context(), packet.Binding)
			expect(t, err != nil && f.State(t) == before, "missing proof retained approval or caused repair")
		})
	}
	t.Run("changed scenario policy invalidates declared approval", func(t *testing.T) {
		f := factory(t)
		packet, err := f.Requests.Prepare(t.Context(), f.ID, f.Candidate, f.Scenarios)
		expect(t, err == nil, "prepare final proof: %v", err)
		_, err = f.Receipts.Record(t.Context(), packet.Binding, "declared operator", "")
		expect(t, err == nil, "record original final receipt: %v", err)
		f.ChangePolicy(t)
		before := f.State(t)
		_, err = f.Receipts.Record(t.Context(), packet.Binding, "declared operator", "")
		expect(t, err != nil, "changed scenario set reused an approval")
		_, err = f.Receipts.Verify(t.Context(), packet.Binding)
		expect(t, err != nil && f.State(t) == before, "stale final approval remained current or caused repair")
	})
}

func expect(t *testing.T, condition bool, format string, args ...any) {
	t.Helper()
	if !condition {
		t.Fatalf(format, args...)
	}
}
