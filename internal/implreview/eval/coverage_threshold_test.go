package eval

import "testing"

func TestMedianIntEmptyOddAndEven(t *testing.T) {
	for _, tc := range []struct {
		in   []int
		want int
	}{
		{want: 0},
		{in: []int{9, 1, 5}, want: 5},
		{in: []int{9, 1, 5, 3}, want: 3},
	} {
		if got := medianInt(tc.in); got != tc.want {
			t.Fatalf("medianInt(%v) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestHarnessPackRoundTripAndSyntheticDiffBounds(t *testing.T) {
	pack := buildPack("standard", "charter", "diff")
	if got := parsePack(pack); got.Band != "standard" || got.Charter != "charter" || got.Diff != "diff" {
		t.Fatalf("parsePack = %+v", got)
	}
	if got := parsePack([]byte("not json")); got != (packEnvelope{}) {
		t.Fatalf("malformed parsePack = %+v", got)
	}
	if got := syntheticDiff(0); got != nil {
		t.Fatalf("syntheticDiff(0) = %q", got)
	}
	for _, n := range []int{3, 200} {
		if got := syntheticDiff(n); len(got) != n {
			t.Fatalf("syntheticDiff(%d) length = %d", n, len(got))
		}
	}
}

func TestLensOfRuleIDAcceptsBareIdentity(t *testing.T) {
	if got := lensOfRuleID("correctness-critic"); got != "correctness-critic" {
		t.Fatalf("lensOfRuleID() = %q", got)
	}
}
