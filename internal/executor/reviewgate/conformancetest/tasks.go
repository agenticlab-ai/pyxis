// Package conformancetest fixes task-proof recovery requirements independently
// of the code-host medium. Factories supply production ports and full host
// review fixtures; terminal summaries cannot stand in for complete snapshots.
package conformancetest

import (
	"reflect"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewgate"
)

type TaskFixture struct {
	ID             string
	Proposals      codehost.ScopedProposals
	Artifacts      codehost.ReviewArtifacts
	Requests       *reviewgate.TaskRequests
	Receipts       codehost.Receipts
	Clone          func(*testing.T) TaskFixture
	RemoveCheckout func(*testing.T)
	Forget         func(*testing.T, codehost.ArtifactRef)
	State          func(*testing.T) string
	ChangePolicy   func(*testing.T)
}

func RunTasks(t *testing.T, factory func(*testing.T) TaskFixture) {
	t.Helper()
	t.Run("identical retained proof and checkout-free recovery", func(t *testing.T) {
		f := factory(t)
		packet, err := f.Requests.Prepare(t.Context(), f.ID)
		expect(t, err == nil, "prepare complete review: %v", err)
		g, err := f.Proposals.Read(t.Context(), f.ID)
		expect(t, err == nil, "read prepared generation: %v", err)
		before := f.State(t)
		for _, artifact := range []codehost.ReviewArtifact{packet.Aggregate, packet.Request} {
			again, err := f.Artifacts.Retain(t.Context(), g, artifact.Kind, artifact.Body)
			expect(t, err == nil && reflect.DeepEqual(again, artifact), "idempotent retention changed complete bytes or identity: %v", err)
			read, err := f.Artifacts.ReadRetained(t.Context(), g, artifact.Kind, artifact.Identity)
			expect(t, err == nil && reflect.DeepEqual(read, artifact), "retained round trip changed complete proof: %v", err)
		}
		resumed, err := f.Requests.Prepare(t.Context(), f.ID)
		expect(t, err == nil && reflect.DeepEqual(resumed, packet) && f.State(t) == before, "resume replaced a request or generation: %v", err)
		receipt, err := f.Receipts.Record(t.Context(), packet.Binding, "declared operator", "")
		expect(t, err == nil, "record exact declared approval: %v", err)
		clone := f.Clone(t)
		f.RemoveCheckout(t)
		before = f.State(t)
		observed, err := clone.Requests.Observe(t.Context(), f.ID)
		expect(t, err == nil && reflect.DeepEqual(observed, packet), "clone lost complete request and proof: %v", err)
		verified, err := clone.Receipts.Verify(t.Context(), packet.Binding)
		expect(t, err == nil && verified == receipt, "clone lost exact receipt: %v", err)
		expect(t, f.State(t) == before, "recovery wrote shared state")
	})
	for _, kind := range []string{codehost.TaskAggregate, codehost.TaskRequest} {
		t.Run("missing "+kind+" refuses progression", func(t *testing.T) {
			f := factory(t)
			packet, err := f.Requests.Prepare(t.Context(), f.ID)
			expect(t, err == nil, "prepare complete review: %v", err)
			_, err = f.Receipts.Record(t.Context(), packet.Binding, "declared operator", "")
			expect(t, err == nil, "record exact declared approval: %v", err)
			ref := packet.Aggregate.Identity
			if kind == codehost.TaskRequest {
				ref = packet.Request.Identity
			}
			f.Forget(t, ref)
			before := f.State(t)
			_, err = f.Requests.Observe(t.Context(), f.ID)
			expect(t, err != nil, "missing proof was presented for review")
			_, err = f.Receipts.Verify(t.Context(), packet.Binding)
			expect(t, err != nil && f.State(t) == before, "missing proof retained approval or caused repair writes")
		})
	}
	t.Run("changed policy invalidates declared approval", func(t *testing.T) {
		f := factory(t)
		packet, err := f.Requests.Prepare(t.Context(), f.ID)
		expect(t, err == nil, "prepare complete review: %v", err)
		_, err = f.Receipts.Record(t.Context(), packet.Binding, "declared operator", "")
		expect(t, err == nil, "record original approval: %v", err)
		f.ChangePolicy(t)
		before := f.State(t)
		_, err = f.Receipts.Record(t.Context(), packet.Binding, "declared operator", "")
		expect(t, err != nil, "changed policy reused an approval")
		_, err = f.Receipts.Verify(t.Context(), packet.Binding)
		expect(t, err != nil && f.State(t) == before, "stale approval remained current or caused repair")
	})
}

func expect(t *testing.T, condition bool, format string, args ...any) {
	t.Helper()
	if !condition {
		t.Fatalf(format, args...)
	}
}
