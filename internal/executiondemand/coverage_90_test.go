package executiondemand

import "testing"

func TestCoverage90MinimumRejectsEveryInvalidAxis(t *testing.T) {
	for _, tc := range []struct {
		name string
		min  ProfileMinimum
	}{
		{name: "capability", min: ProfileMinimum{Capability: "invalid"}},
		{name: "deliberation", min: ProfileMinimum{Deliberation: "invalid"}},
		{name: "context", min: ProfileMinimum{Context: "invalid"}},
		{name: "assurance", min: ProfileMinimum{Assurance: "invalid"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateMinimum(tc.min, "profile"); err == nil {
				t.Fatal("invalid minimum was accepted")
			}
		})
	}
}

func TestCoverage90CitationFailures(t *testing.T) {
	if err := validateCitations(EvidenceSet{}, nil, "citations"); err == nil {
		t.Fatal("empty citations were accepted")
	}
	if err := validateCitations(EvidenceSet{Resolved: map[string]struct{}{}}, []Citation{{ID: "missing"}}, "citations"); err == nil {
		t.Fatal("unresolved citation was accepted")
	}
}

func TestCoverage90WithAuthorityAcceptsPointerMismatch(t *testing.T) {
	mismatch := newCapsuleMismatch(BoundarySelf, "digest", CodeDigestMismatch, "mismatch")
	err := withAuthority(&mismatch, Capsule{})
	if _, ok := err.(CapsuleMismatch); !ok {
		t.Fatalf("withAuthority returned %T", err)
	}
}
