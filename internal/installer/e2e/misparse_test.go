package e2e

// Leg 5 — the misparse suite (UB-9/A-7; design §3.4, DINV-5). Every pyxis verb
// with a flag-like positional, an unknown flag, or a missing `--` refuses
// PRE-ACTION with exit 2 (usage — distinct from the 1 of a named runtime
// refusal), a named stderr, and ZERO filesystem mutation. Driven through the
// real verb dispatch on a converged scratch repo (so the guard fires on the
// deployed binary, AP-14) and snapshotted around every case to prove no write
// slipped past the guard (the I3 fail-closed discipline).
//
// Overlaps cmd/pyxis's in-process TestMisparseRefusesPreActionOnEveryVerb — the
// load-bearing addition here is driving the shipped binary end to end on a real
// converged repo AND snapshotting the tree around each subprocess invocation.

import (
	"strings"
	"testing"
)

func TestMisparseSuite(t *testing.T) {
	requireHarness(t)
	r := buildFixtureRelease(t)
	repo, env := scratchRepo(t, r)

	// Every case is a grammar refusal: exit 2, named stderr, zero mutation. The
	// converge/doctor/exec verbs run on the converged repo (so the guard is the
	// only thing that fires); the init cases refuse before any probe too, but
	// init would mutate a bare dir — so they run in a config-carrying repo where
	// UB-11 would refuse anyway; the point is the GUARD fires first (exit 2, not
	// the exit-1 UB-11 refusal).
	cases := []struct {
		name string
		args []string
	}{
		{"no verb", nil},
		{"unknown verb", []string{"bogus"}},

		{"converge unknown flag", []string{"converge", "--force"}},
		{"converge stray positional", []string{"converge", "extra"}},
		{"converge flag after positional", []string{"converge", "extra", "--preview"}},
		{"converge malformed flag value", []string{"converge", "--preview=maybe"}},

		{"repair unknown flag", []string{"repair", "--force"}},
		{"repair does not take --repair", []string{"repair", "--repair"}},
		{"repair stray positional", []string{"repair", "extra"}},

		{"doctor unknown flag", []string{"doctor", "--json"}},
		{"doctor stray positional", []string{"doctor", "extra"}},
		{"doctor flag-like positional", []string{"doctor", "-v"}},

		{"version unknown flag", []string{"version", "--verbose"}},
		{"version stray positional", []string{"version", "extra"}},

		{"init unknown flag", []string{"init", "--bogus", "--system-version", "v1", "--manifest-sha256", "ab"}},
		{"init stray positional", []string{"init", "extra"}},
		{"init flag-like positional", []string{"init", "-x"}},
		{"init missing pin pair", []string{"init", "--tracker", "github"}},
		{"init missing manifest sha", []string{"init", "--system-version", "v1.0.0"}},
		{"init malformed wiring", []string{"init", "--wiring", "repo", "--system-version", "v1", "--manifest-sha256", "ab"}},

		{"exec missing separator", []string{"exec", "docgate", "gate"}},
		{"exec bare", []string{"exec"}},
		{"exec no tool", []string{"exec", "--", "gate"}},
		{"exec two tools", []string{"exec", "docgate", "codegate", "--"}},
		{"exec flag-like tool slot", []string{"exec", "-docgate", "--"}},
		{"exec unknown flag", []string{"exec", "--tool", "docgate", "--", "gate"}},
		{"exec flag after tool", []string{"exec", "docgate", "-v", "--", "gate"}},
		{"exec uppercase tool", []string{"exec", "DocGate", "--"}},
		{"exec empty tool", []string{"exec", "", "--"}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			before := snap(t, repo)
			res := pyxis(t, repo, env, tt.args...)
			if res.code != 2 {
				t.Fatalf("pyxis %v = %d, want 2 (usage refusal before any action)\nstdout: %s\nstderr: %s", tt.args, res.code, res.stdout, res.stderr)
			}
			if strings.TrimSpace(res.stderr) == "" {
				t.Fatalf("pyxis %v refused silently — the refusal must be named", tt.args)
			}
			sameSnap(t, before, snap(t, repo), "misparse "+tt.name)
		})
	}
}
