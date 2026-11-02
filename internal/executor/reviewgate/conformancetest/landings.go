package conformancetest

import (
	"context"
	"reflect"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewgate"
)

// LandingFixture supplies an approved complete request and production operator
// ports. Git shape checks belong to the medium fixture; proof and recovery
// assertions below are shared by every task-landing adapter.
type LandingFixture struct {
	Packet         reviewgate.Packet
	Landings       codehost.TaskLandings
	Clone          func(*testing.T) codehost.TaskLandings
	RemoveCheckout func(*testing.T)
	AssertSquash   func(*testing.T, codehost.TaskLandingResult)
	Forget         func(*testing.T, codehost.ArtifactRef)
	State          func(*testing.T) string
}

func RunLandings(t *testing.T, factory func(*testing.T) LandingFixture) {
	t.Helper()
	t.Run("exact proof and checkout-free recovery", func(t *testing.T) {
		f := factory(t)
		b := f.Packet.Binding
		landed, err := f.Landings.LandTask(t.Context(), b)
		expect(t, err == nil && landed.Created, "operator landing failed: %v", err)
		expect(t, landed.Landing.Validate() == nil && landed.Landing.Review.Receipt.Binding == b &&
			landed.Prepared.OID == b.ProposalOID && landed.FeatureHead == landed.Landing.MergeCommit &&
			reflect.DeepEqual(landed.Landing.Aggregate, f.Packet.Aggregate) && reflect.DeepEqual(landed.Landing.Request, f.Packet.Request),
			"landing changed the reviewed generation or complete proof")
		f.AssertSquash(t, landed)
		clone := f.Clone(t)
		f.RemoveCheckout(t)
		before := f.State(t)
		for _, operation := range []func() (codehost.TaskLandingResult, error){
			func() (codehost.TaskLandingResult, error) { return clone.VerifyTaskLanding(t.Context(), b.ProposalID) },
			func() (codehost.TaskLandingResult, error) { return clone.LandTask(t.Context(), b) },
		} {
			recovered, err := operation()
			expect(t, err == nil && !recovered.Created && reflect.DeepEqual(recovered.Landing, landed.Landing) &&
				recovered.Prepared.OID == b.ProposalOID && recovered.FeatureHead == landed.FeatureHead && f.State(t) == before,
				"recovery lost proof or wrote shared state: %v", err)
		}
	})
	t.Run("cancelled and different approval refuse before mutation", func(t *testing.T) {
		f := factory(t)
		before := f.State(t)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err := f.Landings.LandTask(ctx, f.Packet.Binding)
		expect(t, err != nil && f.State(t) == before, "cancelled landing wrote shared state")
		binding := f.Packet.Binding
		binding.Sequence++
		_, err = f.Landings.LandTask(t.Context(), binding)
		expect(t, err != nil && f.State(t) == before, "a different generation used the prior approval")
	})
	for _, kind := range []string{codehost.TaskAggregate, codehost.TaskRequest} {
		t.Run("missing "+kind+" prevents landed recovery", func(t *testing.T) {
			f := factory(t)
			_, err := f.Landings.LandTask(t.Context(), f.Packet.Binding)
			expect(t, err == nil, "operator landing failed: %v", err)
			ref := f.Packet.Aggregate.Identity
			if kind == codehost.TaskRequest {
				ref = f.Packet.Request.Identity
			}
			f.Forget(t, ref)
			before := f.State(t)
			_, err = f.Landings.VerifyTaskLanding(t.Context(), f.Packet.Binding.ProposalID)
			expect(t, err != nil, "landed observation accepted missing proof")
			_, err = f.Landings.LandTask(t.Context(), f.Packet.Binding)
			expect(t, err != nil && f.State(t) == before, "landed retry repaired missing proof")
		})
	}
}
