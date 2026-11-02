package band_test

import (
	"context"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/band"
	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// fakeVerifier is an injected second-instance runner (a runtime.LensRunner). It
// CONFIRMS a finding whose RuleID is in confirm, and does NOT confirm (returns no
// candidates) otherwise. It records every call so a test can assert it ran — or
// asserts it NEVER runs when the flag is off (spy==true fails the test on any
// call).
type fakeVerifier struct {
	confirm map[string]bool
	spy     bool
	t       *testing.T
	calls   int
}

func (f *fakeVerifier) Run(_ context.Context, lens string, _ []byte, instruction string) ([]report.Finding, error) {
	if f.spy {
		f.t.Fatalf("second-instance verifier ran while the flag was OFF (lens=%q, instr=%q)", lens, instruction)
	}
	f.calls++
	// The instruction names the finding's rule id — the second instance re-judges
	// THAT claim. It confirms iff the rule id is in the confirm set.
	for id := range f.confirm {
		if strings.Contains(instruction, id) {
			return []report.Finding{{
				ID:      "SECOND-INSTANCE-CONFIRM",
				Message: "the second instance independently agrees this is a real blocking defect",
				RuleID:  id,
				Lens:    lens,
			}}, nil
		}
	}
	// Not in the confirm set — the second instance does not confirm (downgrade).
	return nil, nil
}

// deepFindings is a seeded round finding set: one blocking finding the second
// instance WILL confirm, one blocking finding it will NOT confirm, and one
// advisory finding (never re-judged).
func deepFindings() []report.Finding {
	return []report.Finding{
		{
			ID: "FND-1", Message: "confirmed blocking defect", RuleID: "correctness-critic/real-race",
			Lens: report.LensCorrectnessCritic, Severity: report.SeverityBlocking, CalRow: "CAL-11", Acceptance: "A-1",
		},
		{
			ID: "FND-2", Message: "a plausible-but-unconfirmed blocking claim", RuleID: "correctness-critic/maybe-edge",
			Lens: report.LensCorrectnessCritic, Severity: report.SeverityBlocking, CalRow: "CAL-11", Acceptance: "A-2",
		},
		{
			ID: "FND-3", Message: "an advisory latent risk", RuleID: "correctness-critic/latent",
			Lens: report.LensCorrectnessCritic, Severity: report.SeverityAdvisory, CalRow: "CAL-12", Acceptance: "A-3",
		},
	}
}

func verifyReq(cfg band.VerifyConfig, bnd band.Band, v band.Verifier) band.VerifyRequest {
	return band.VerifyRequest{
		Config:             cfg,
		Band:               bnd,
		SecondInstanceRoot: "second-instance-root",
		ImplementerRoot:    "implementer-root",
		FirstReviewerRoot:  "first-reviewer-root",
		Pack:               []byte("the shared pack"),
		Verifier:           v,
	}
}

// TestVerifyOffByDefaultNeverRuns — A-8: the seam is dormant. The zero
// VerifyConfig (Enabled==false) is OFF: Verify is a no-op, the verifier NEVER
// runs, and the findings are returned verbatim.
func TestVerifyOffByDefaultNeverRuns(t *testing.T) {
	t.Parallel()

	spy := &fakeVerifier{spy: true, t: t}
	in := deepFindings()

	// The zero config is the dormant default.
	res, err := band.Verify(context.Background(), in, verifyReq(band.VerifyConfig{}, report.BandDeep, spy))
	if err != nil {
		t.Fatalf("Verify (off): %v", err)
	}
	if res.Ran {
		t.Fatal("Verify ran while the flag was off")
	}
	if len(res.Downgraded) != 0 {
		t.Fatalf("Verify downgraded findings while off: %v", res.Downgraded)
	}
	// The findings are unchanged (verbatim).
	if len(res.Findings) != len(in) {
		t.Fatalf("Verify changed the finding count while off")
	}
	for i := range in {
		if res.Findings[i].Severity != in[i].Severity {
			t.Fatalf("Verify changed a finding's severity while off: %q", res.Findings[i].ID)
		}
	}
	if spy.calls != 0 {
		t.Fatalf("verifier was called %d times while off", spy.calls)
	}
}

// TestVerifyOnDowngradesUnconfirmed — A-8: with the flag ON in the deep band, the
// second instance re-judges each blocking finding; the one it does not confirm is
// downgraded blocking→advisory, the one it confirms stays blocking, and the
// advisory finding is never re-judged.
func TestVerifyOnDowngradesUnconfirmed(t *testing.T) {
	t.Parallel()

	v := &fakeVerifier{
		confirm: map[string]bool{"correctness-critic/real-race": true}, // FND-1 confirmed; FND-2 not
		t:       t,
	}
	in := deepFindings()

	res, err := band.Verify(context.Background(), in, verifyReq(band.VerifyConfig{Enabled: true}, report.BandDeep, v))
	if err != nil {
		t.Fatalf("Verify (on): %v", err)
	}
	if !res.Ran {
		t.Fatal("Verify did not run while enabled in the deep band")
	}

	byID := map[string]report.Finding{}
	for _, f := range res.Findings {
		byID[f.ID] = f
	}
	// FND-1 confirmed → stays blocking.
	if byID["FND-1"].Severity != report.SeverityBlocking {
		t.Errorf("confirmed FND-1 = %q, want blocking (not downgraded)", byID["FND-1"].Severity)
	}
	// FND-2 not confirmed → downgraded to advisory.
	if byID["FND-2"].Severity != report.SeverityAdvisory {
		t.Errorf("unconfirmed FND-2 = %q, want advisory (downgraded)", byID["FND-2"].Severity)
	}
	// FND-3 advisory → untouched (only blocking findings are re-judged; the
	// verifier is called twice, once per blocking finding).
	if byID["FND-3"].Severity != report.SeverityAdvisory {
		t.Errorf("advisory FND-3 = %q, want advisory (unchanged)", byID["FND-3"].Severity)
	}
	if v.calls != 2 {
		t.Errorf("verifier called %d times, want 2 (one per blocking finding)", v.calls)
	}

	// The downgraded set names exactly the unconfirmed finding.
	if len(res.Downgraded) != 1 || res.Downgraded[0] != "FND-2" {
		t.Errorf("downgraded = %v, want [FND-2]", res.Downgraded)
	}

	// It NEVER upgraded a finding — the advisory one stayed advisory.
	if byID["FND-3"].Severity == report.SeverityBlocking {
		t.Error("the second instance upgraded an advisory finding (must be one-directional)")
	}

	// The INPUT slice is not mutated (Verify returns a copy).
	if in[1].Severity != report.SeverityBlocking {
		t.Error("Verify mutated the caller's input finding slice")
	}
}

// TestVerifyOnStandardBandNeverRuns — A-8 is deep-band only: even ENABLED, the
// verify never runs outside the deep band.
func TestVerifyOnStandardBandNeverRuns(t *testing.T) {
	t.Parallel()
	spy := &fakeVerifier{spy: true, t: t}
	res, err := band.Verify(context.Background(), deepFindings(), verifyReq(band.VerifyConfig{Enabled: true}, report.BandStandard, spy))
	if err != nil {
		t.Fatalf("Verify (standard, enabled): %v", err)
	}
	if res.Ran {
		t.Fatal("Verify ran in the standard band")
	}
	if spy.calls != 0 {
		t.Fatalf("verifier ran %d times in the standard band", spy.calls)
	}
}

// TestVerifyOnNoBlockingIsNoop — enabled in the deep band but with no blocking
// finding to re-judge, Verify is still a no-op (nothing to downgrade), and the
// verifier is never called.
func TestVerifyOnNoBlockingIsNoop(t *testing.T) {
	t.Parallel()
	spy := &fakeVerifier{spy: true, t: t}
	advisoryOnly := []report.Finding{
		{ID: "FND-A", Message: "advisory", RuleID: "correctness-critic/latent", Lens: report.LensCorrectnessCritic, Severity: report.SeverityAdvisory, CalRow: "CAL-12"},
	}
	res, err := band.Verify(context.Background(), advisoryOnly, verifyReq(band.VerifyConfig{Enabled: true}, report.BandDeep, spy))
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if res.Ran {
		t.Fatal("Verify ran with no blocking finding to re-judge")
	}
}

// TestVerifyEnabledNilVerifierFailsClosed — enabled with a blocking finding but
// no verifier is a misconfiguration: Verify fails closed rather than silently
// skip the re-judge it was told to perform.
func TestVerifyEnabledNilVerifierFailsClosed(t *testing.T) {
	t.Parallel()
	req := verifyReq(band.VerifyConfig{Enabled: true}, report.BandDeep, nil)
	_, err := band.Verify(context.Background(), deepFindings(), req)
	if err == nil {
		t.Fatal("enabled verify with a nil verifier should fail closed")
	}
}

// TestVerifyEnabledInheritedSecondInstanceFailsClosed — the second instance must
// be INDEPENDENT: if its root is inherited from the implementer (or the first
// reviewer), Verify fails closed (reusing the runtime's provenance check), never
// re-judging over a non-independent second instance.
func TestVerifyEnabledInheritedSecondInstanceFailsClosed(t *testing.T) {
	t.Parallel()
	v := &fakeVerifier{confirm: map[string]bool{}, t: t}

	// Second instance root == implementer root: not independent.
	req := verifyReq(band.VerifyConfig{Enabled: true}, report.BandDeep, v)
	req.SecondInstanceRoot = "shared-root"
	req.ImplementerRoot = "shared-root"
	if _, err := band.Verify(context.Background(), deepFindings(), req); err == nil {
		t.Fatal("second instance inheriting the implementer's context should fail closed")
	}

	// Second instance root == first reviewer root: also not independent.
	req2 := verifyReq(band.VerifyConfig{Enabled: true}, report.BandDeep, v)
	req2.SecondInstanceRoot = "same-as-first"
	req2.FirstReviewerRoot = "same-as-first"
	if _, err := band.Verify(context.Background(), deepFindings(), req2); err == nil {
		t.Fatal("second instance reusing the first reviewer's context should fail closed")
	}
}
