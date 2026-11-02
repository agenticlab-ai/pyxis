package conformancetest

import (
	"context"
	"reflect"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/featuredrive/candidatereview"
)

// LandingFixture exposes only consumer predicates. Object shape and origin
// mutation are supplied by the production medium's test fixture.
type LandingFixture struct {
	Packet         candidatereview.Packet
	Landings       codehost.CandidateLandings
	Clone          func(*testing.T) codehost.CandidateLandings
	RemoveCheckout func(*testing.T)
	AssertDelivery func(*testing.T, codehost.CandidateLandingResult)
	SetProof       func(*testing.T, codehost.ArtifactRef, bool)
	State          func(*testing.T) string
	ChangePolicy   func(*testing.T)
}

func RunLandings(t *testing.T, factory func(*testing.T) LandingFixture) {
	t.Helper()
	f := factory(t)
	b := f.Packet.Binding
	t.Run("cancelled and different approval refuse without writes", func(t *testing.T) {
		before := f.State(t)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err := f.Landings.LandCandidate(ctx, b)
		expect(t, err != nil && f.State(t) == before, "cancelled landing wrote origin")
		wrong := b
		wrong.Sequence++
		_, err = f.Landings.LandCandidate(t.Context(), wrong)
		expect(t, err != nil && f.State(t) == before, "different generation reused prior approval")
	})
	landed, err := f.Landings.LandCandidate(t.Context(), b)
	expect(t, err == nil && landed.Created && landed.Landing.Validate() == nil, "operator landing refused: %v", err)
	expect(t, landed.Prepared.OID == b.ProposalOID && landed.Landing.Review.Receipt.Binding == b &&
		reflect.DeepEqual(landed.Landing.Evidence, f.Packet.Evidence) && reflect.DeepEqual(landed.Landing.Request, f.Packet.Request), "landing changed the complete reviewed proof")
	f.AssertDelivery(t, landed)
	clone := f.Clone(t)
	f.RemoveCheckout(t)
	t.Run("checkout-free recovery retains complete proof and does not repeat delivery", func(t *testing.T) {
		before := f.State(t)
		for _, operation := range []func() (codehost.CandidateLandingResult, error){
			func() (codehost.CandidateLandingResult, error) {
				return clone.VerifyCandidateLanding(t.Context(), b.ProposalID)
			},
			func() (codehost.CandidateLandingResult, error) { return clone.LandCandidate(t.Context(), b) },
		} {
			result, err := operation()
			expect(t, err == nil && !result.Created && reflect.DeepEqual(result.Landing, landed.Landing) && result.Prepared.OID == b.ProposalOID && result.MainHead == landed.MainHead && result.FeatureHead == landed.FeatureHead && f.State(t) == before, "retained recovery changed delivery or origin: %v", err)
		}
	})
	for _, artifact := range []codehost.ReviewArtifact{f.Packet.Evidence, f.Packet.Request} {
		t.Run("missing "+artifact.Kind+" refuses recovered delivery", func(t *testing.T) {
			f.SetProof(t, artifact.Identity, false)
			defer f.SetProof(t, artifact.Identity, true)
			before := f.State(t)
			_, err := clone.VerifyCandidateLanding(t.Context(), b.ProposalID)
			expect(t, err != nil, "missing retained proof accepted")
			_, err = clone.LandCandidate(t.Context(), b)
			expect(t, err != nil && f.State(t) == before, "retry repaired missing proof or repeated delivery")
		})
	}
	t.Run("current scenario policy remains required after landing", func(t *testing.T) {
		f.ChangePolicy(t)
		before := f.State(t)
		_, err := clone.VerifyCandidateLanding(t.Context(), b.ProposalID)
		expect(t, err != nil, "changed scenario policy accepted retained delivery")
		_, err = clone.LandCandidate(t.Context(), b)
		expect(t, err != nil && f.State(t) == before, "stale policy caused a second landing")
	})
}
