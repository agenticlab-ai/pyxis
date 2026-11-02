package localgit

import (
	"context"
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
)

type candidateLandingReader struct {
	result codehost.CandidateLandingResult
	err    error
}

func (r candidateLandingReader) VerifyCandidateLanding(context.Context, string) (codehost.CandidateLandingResult, error) {
	return r.result, r.err
}

func TestCandidateDeliveryRefusesMissingReaderAndUnreadableProof(t *testing.T) {
	for _, reader := range []codehost.CandidateLandingReader{nil, candidateLandingReader{err: errors.New("retained delivery unavailable")}, candidateLandingReader{result: codehost.CandidateLandingResult{Landing: codehost.CandidateLanding{Evidence: codehost.ReviewArtifact{Body: []byte("malformed evidence")}}}}} {
		proof, err := NewCandidateDeliveryVerifier(reader).VerifyDelivery(t.Context(), featuredrive.AcceptedCandidate{}, featuredrive.LandedRevision{}, featuredrive.CurrentMain{})
		if !errors.Is(err, featuredrive.ErrDeliveryUnverified) || proof != (featuredrive.DeliveryProof{}) {
			t.Fatalf("missing whole proof certified delivery: %+v %v", proof, err)
		}
	}
}
