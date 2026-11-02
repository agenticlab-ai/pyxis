package contractgate

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

// Determinism + hermeticity (plan T8; prd:contract-gate/NFR-2, FR-12; design
// contract-gate-v1 §3.11, DINV-5). NFR-2 is two claims: (1) the gate produces a
// byte-identical VERDICT across repeated runs over the same contract inputs, and
// (2) the gate makes no network call ("a network attempt by the gate fails the
// determinism suite"). The pre-existing TestDeterministic asserts the verdict's
// class + evaluated identities are stable; this file strengthens (1) to the WHOLE
// verdict (every field, serialized) across all fixtures, and proves (2) as a
// compile-time hermeticity assertion — the gate's transitive import closure links
// NO networking package, so a network attempt is not merely absent but impossible.

// marshalVerdict serializes a verdict deterministically (indented JSON, fixed
// field order) so two runs' bytes can be compared exactly — the strongest form of
// "byte-identical verdict" (NFR-2).
func marshalVerdict(t *testing.T, v Verdict) []byte {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal verdict: %v", err)
	}
	return b
}

// TestVerdictByteIdenticalAcrossRuns proves the ENTIRE verdict — not just its
// class and evaluated set — is byte-identical across repeated runs over the same
// contract inputs, for the clean fixture and every defect class (NFR-2, DINV-5).
// A run that leaked a clock, a random value, or a map-iteration-ordered field into
// the verdict would diverge here. Ten runs of each fixture, all bytes equal.
func TestVerdictByteIdenticalAcrossRuns(t *testing.T) {
	for _, tc := range fixtureSuite {
		t.Run(tc.class, func(t *testing.T) {
			root := filepath.Join("testdata", tc.fixture)
			first := marshalVerdict(t, RunWithConfig(root, tc.feature, config.Defaults()))
			for i := 0; i < 10; i++ {
				got := marshalVerdict(t, RunWithConfig(root, tc.feature, config.Defaults()))
				if !bytes.Equal(first, got) {
					t.Fatalf("verdict for %s not byte-identical on run %d:\n--- first ---\n%s\n--- run %d ---\n%s",
						tc.fixture, i, first, i, got)
				}
			}

			// The trigger path (RunTrigger over both closed triggers) is equally
			// stable — the split does not introduce nondeterminism.
			for _, trig := range []Trigger{TriggerPlanningEntry, TriggerContractDocPR} {
				a := marshalVerdict(t, RunWithConfigTrigger(root, tc.feature, trig, config.Defaults(), defaultSeverityMap()))
				b := marshalVerdict(t, RunWithConfigTrigger(root, tc.feature, trig, config.Defaults(), defaultSeverityMap()))
				if !bytes.Equal(a, b) {
					t.Errorf("verdict for %s under trigger %q not byte-stable", tc.fixture, trig)
				}
			}
		})
	}
}

// TestSweepByteIdenticalAcrossRuns proves the dogfood sweep result is itself
// deterministic (plan T8: "the sweep itself must stay deterministic"): repeated
// sweeps over the same fixture repo serialize byte-identically. This guarantees a
// checked-in disposition record derived from the sweep is reproducible.
func TestSweepByteIdenticalAcrossRuns(t *testing.T) {
	// The wellformed fixture repo carries one live contract (demo); the malformed
	// and missing repos have their own shapes. Sweep each and assert stability.
	for _, fixture := range []string{"wellformed", "uncovered-must", "stale-serve"} {
		t.Run(fixture, func(t *testing.T) {
			root := filepath.Join("testdata", fixture)
			a, err := json.MarshalIndent(Sweep(root), "", "  ")
			if err != nil {
				t.Fatalf("marshal sweep: %v", err)
			}
			for i := 0; i < 5; i++ {
				b, err := json.MarshalIndent(Sweep(root), "", "  ")
				if err != nil {
					t.Fatalf("marshal sweep: %v", err)
				}
				if !bytes.Equal(a, b) {
					t.Fatalf("sweep of %s not byte-identical on run %d", fixture, i)
				}
			}
		})
	}
}

// TestGateIsHermeticNoNetworkPackageLinked proves the gate makes no network call
// by construction (NFR-2 / FR-12: "a network attempt by the gate fails the
// determinism suite"; NFR-6 network-free-by-construction). Rather than injecting a
// dial-blocking transport (the gate has no true-external dependency — §3.11: "a
// single production path, exercised directly"), it asserts the STRONGER property:
// the contractgate package's entire transitive import closure links NO networking
// package (net, net/http, net/rpc, crypto/tls, …). A network attempt is therefore
// not merely unobserved — it is impossible; the instant a future edit pulls in a
// net package, this test fails, exactly as the design requires.
//
// It shells to `go list -deps` (the toolchain's own dependency resolver) — the
// authoritative closure, not a hand-maintained allowlist. If the go tool is
// unavailable the test skips (the CI environment always has it).
func TestGateIsHermeticNoNetworkPackageLinked(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("go toolchain unavailable: %v", err)
	}
	out, err := exec.Command(goBin, "list", "-deps", "github.com/agenticlab-ai/pyxis/internal/docgate/contractgate").Output()
	if err != nil {
		t.Fatalf("go list -deps: %v", err)
	}

	// The networking packages a gate must never link. A dependency whose path is
	// exactly one of these, or nested under one (e.g. net/http, net/url), is a
	// network surface. `crypto/tls` and `net/rpc` are called out explicitly.
	forbidden := []string{"net", "net/http", "net/rpc", "net/url", "crypto/tls"}
	isForbidden := func(pkg string) bool {
		for _, f := range forbidden {
			if pkg == f {
				return true
			}
		}
		// `net/...` is the whole networking tree; `net/url` is pure parsing but is
		// listed above as a deliberate boundary — the gate reads committed files, it
		// resolves no URLs.
		return strings.HasPrefix(pkg, "net/")
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		pkg := strings.TrimSpace(line)
		if pkg == "" {
			continue
		}
		if isForbidden(pkg) {
			t.Errorf("contractgate links networking package %q — the gate must be network-free by construction (NFR-2/NFR-6); a network attempt must be impossible", pkg)
		}
	}
}
