package wake

import (
	"context"
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/argusledger"
)

func TestSplitScheme(t *testing.T) {
	cases := []struct {
		in         string
		wantScheme string
		wantRef    string
		wantOK     bool
	}{
		{"pr:214", "pr", "214", true},
		{"item:argus/epic#7", "item", "argus/epic#7", true},
		{"item:a:b:c", "item", "a:b:c", true}, // split on the FIRST colon
		{"  pr:9  ", "pr", "9", true},
		{"noscheme", "", "", false},
		{":ref", "", "", false},
		{"pr:", "", "", false},
		{"", "", "", false},
	}
	for _, c := range cases {
		s, r, ok := splitScheme(c.in)
		if ok != c.wantOK || s != c.wantScheme || r != c.wantRef {
			t.Errorf("splitScheme(%q) = (%q,%q,%v), want (%q,%q,%v)", c.in, s, r, ok, c.wantScheme, c.wantRef, c.wantOK)
		}
	}
}

func TestProbeAwait_Dispatch(t *testing.T) {
	ctx := context.Background()
	prober := stubProber{
		ancestor: map[string]bool{"214": true, "215": false},
		exists:   map[string]bool{"argus/epic#7": true, "9": false},
	}
	cases := []struct {
		name       string
		goal       string
		wantDone   bool
		wantDefect bool
	}{
		{"merged pr → ancestry true", "pr:214", true, false},
		{"unmerged pr → ancestry false", "pr:215", false, false},
		{"landed item → record exists", "item:argus/epic#7", true, false},
		{"absent item → record missing", "item:9", false, false},
		{"unknown scheme → defect", "branch:main", false, true},
		{"unschemed → defect", "just-text", false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			done, def, err := probeAwait(ctx, prober, argusledger.Await{GoalObject: c.goal})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if done != c.wantDone {
				t.Errorf("done = %v, want %v", done, c.wantDone)
			}
			if (def != nil) != c.wantDefect {
				t.Errorf("defect = %v, want defect=%v", def, c.wantDefect)
			}
		})
	}
}

// A probe read that faults is an infrastructure error, not a false completion.
func TestProbeAwait_ReadErrorSurfaces(t *testing.T) {
	prober := stubProber{err: errors.New("git unreachable")}
	done, def, err := probeAwait(context.Background(), prober, argusledger.Await{GoalObject: "pr:1"})
	if err == nil {
		t.Fatal("want an error from a faulting probe")
	}
	if done || def != nil {
		t.Fatalf("a faulting probe must not report done or a grammar defect: done=%v def=%v", done, def)
	}
}
