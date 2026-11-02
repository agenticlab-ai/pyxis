package installer

import (
	"context"
	"strings"
	"testing"
)

// The interactive forgiveness contract (#935): case, surrounding space, and
// underscore/hyphen near-misses resolve; a fragment resolves only when
// exactly one allowed value contains it; everything else stays invalid. The
// protection enum is the table's spine because it is the one a real first
// run tripped on.
func TestCanonicalizeEnumInput(t *testing.T) {
	protection := []string{"none", "required-checks", "checks+review"}
	cases := []struct {
		name    string
		input   string
		allowed []string
		want    string
		ok      bool
	}{
		{"exact", "required-checks", protection, "required-checks", true},
		{"underscore for hyphen", "required_checks", protection, "required-checks", true},
		{"case folded", "NONE", protection, "none", true},
		{"surrounding space", "  checks+review ", protection, "checks+review", true},
		{"unique fragment", "req", protection, "required-checks", true},
		{"single letter unique", "n", protection, "none", true},
		{"fragment in two values stays ambiguous", "checks", protection, "", false},
		{"prefix of one but inside another", "c", protection, "", false},
		{"contained once but not a prefix", "review", protection, "", false},
		{"garbage", "strict", protection, "", false},
		{"empty", "", protection, "", false},
		{"bool case folded", "True", []string{"true", "false"}, "true", true},
		{"bool fragment", "f", []string{"true", "false"}, "false", true},
		// "gh" is neither a prefix nor a substring of "github" (which opens
		// "gi") — aliases are beyond narrow forgiveness, so it stays invalid.
		{"alias is not a fragment", "gh", []string{"github", "carina"}, "", false},
		{"tracker fragment", "git", []string{"github", "carina"}, "github", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := canonicalizeEnumInput(tc.input, tc.allowed)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("canonicalizeEnumInput(%q, %v) = (%q, %t), want (%q, %t)",
					tc.input, tc.allowed, got, ok, tc.want, tc.ok)
			}
		})
	}
}

// A near-miss answer must not burn one of FR-13's three attempts: the typo'd
// hyphenation from the real transcript resolves on the first ask, the
// acceptance is echoed, and no re-ask fires.
func TestAskValidatedForgivesNearMissWithoutARetry(t *testing.T) {
	var out strings.Builder
	pr := newPrompter(context.Background(), strings.NewReader("required_checks\n"), &out)
	var collected collectedAnswers

	got, err := askValidated(pr, wiringProtectionQuestion("both"),
		func(s string) (string, bool) {
			return canonicalizeEnumInput(s, []string{"none", "required-checks", "checks+review"})
		},
		&collected)
	if err != nil {
		t.Fatalf("askValidated: %v", err)
	}
	if got != "required-checks" {
		t.Fatalf("resolved %q, want required-checks", got)
	}
	if strings.Contains(out.String(), "try again") {
		t.Errorf("a forgivable near-miss burned a retry:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "→ required-checks") {
		t.Errorf("the canonicalized acceptance was not echoed:\n%s", out.String())
	}
}

// An ambiguous fragment still re-asks (FR-13 unchanged), and the follow-up
// answer resolves.
func TestAskValidatedStillReasksOnAmbiguity(t *testing.T) {
	var out strings.Builder
	pr := newPrompter(context.Background(), strings.NewReader("checks\nchecks+review\n"), &out)
	var collected collectedAnswers

	got, err := askValidated(pr, wiringProtectionQuestion("both"),
		func(s string) (string, bool) {
			return canonicalizeEnumInput(s, []string{"none", "required-checks", "checks+review"})
		},
		&collected)
	if err != nil {
		t.Fatalf("askValidated: %v", err)
	}
	if got != "checks+review" {
		t.Fatalf("resolved %q, want checks+review", got)
	}
	if !strings.Contains(out.String(), "try again") {
		t.Errorf("the ambiguous fragment should have re-asked:\n%s", out.String())
	}
}

// The forgiveness is INTERACTIVE-ONLY (#935's other half): the promptless
// lanes (flags, answers file) arrive as provided values and stay
// byte-strict — a near-miss the prompt would resolve refuses by name here,
// because the machine surface's enums are closed.
func TestProvidedEnumValuesStayByteStrict(t *testing.T) {
	var collected collectedAnswers
	if _, err := resolveEnum("required_checks", wiringProtectionQuestion("both"), nil, &collected); err == nil {
		t.Fatal("promptless \"required_checks\" resolved — the machine lane must stay byte-strict")
	}
	if _, err := resolveEnum("GitHub", trackerQuestion(), nil, &collected); err == nil {
		t.Fatal("promptless \"GitHub\" resolved — the machine lane must stay byte-strict")
	}
}
