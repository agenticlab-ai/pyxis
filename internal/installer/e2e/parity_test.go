package e2e

// Leg 2 — the source-vs-installed parity suite (A-6/NR-2; design §3.11). THE
// proof the T13 reporoot fix delivers NR-2 parity: identical docgate-gate
// fixtures, run through the SOURCE path and the OPERAND path, must produce
// byte-identical verdicts — zero divergence.
//
// The two paths, over the SAME converged operand tree and the SAME docgate
// bytes (TestMain built docgate; the fixture manifest pins exactly it):
//
//   - OPERAND path: `pyxis exec docgate -- gate --defects-json -` — resolve →
//     reporoot (the nearest .pyxis/config.yaml ancestor, then the git root) →
//     re-hash the cached binary → execve. This is the shipped installed-repo
//     invocation; before the reporoot fix it dead-ended on go.mod in a
//     source-less operand (findings_test.go's regression proof). This leg is
//     what makes NR-2 falsifiable end to end: the operand path now REACHES the
//     gate and must agree with the source path.
//   - SOURCE path: the docgate binary invoked directly with the operand as its
//     working directory — exactly what the source repo's resolver delegates to
//     in source mode (locate-and-delegate; the retiring `docgate-resolve.sh`'s
//     `exec` contract), minus the acquisition indirection. Same bytes, same
//     tree, so any verdict difference is a parity break in the INVOCATION path,
//     not the tool.
//
// Run over two fixtures — a clean tree (→ green, exit 0, empty defect stream)
// and one carrying an injected, named violation (→ exit 1, a non-empty defect
// stream) — and diff the full verdict (exit code + the JSON defect lines) both
// ways. Both fixtures are the SAME converged scratch operand; the defect
// fixture injects one corruption a fresh gate names deterministically.
//
// `--defects-json -` gives a machine-stable verdict: one JSON object per defect
// on stdout, so the diff is exact rather than prose-sensitive. Base-free (a
// fresh single-commit operand has no origin merge base), so both paths run the
// identical history-free check set.

import (
	"os/exec"
	"sort"
	"strings"
	"testing"
)

// gateVerdict is one docgate-gate invocation's full, diffable outcome: the exit
// code and the sorted JSON defect lines (`--defects-json -` on stdout).
type gateVerdict struct {
	code      int
	defects   []string // one JSON object per defect, sorted for order-independence
	rawJSON   string   // the unsorted stdout, for failure output
	rawStderr string   // the human defect lines, for failure output
}

func TestSourceVsInstalledParity(t *testing.T) {
	requireHarness(t)
	r := buildFixtureRelease(t)

	t.Run("clean tree — identical green verdict", func(t *testing.T) {
		repo, env := scratchRepo(t, r)
		operand := runGateOperand(t, repo, env)
		source := runGateSource(t, repo, env)
		assertSameVerdict(t, source, operand)
		// A clean converged operand gates green both ways (the T8 green proof's
		// tree, re-gated through both invocation paths).
		if operand.code != 0 {
			t.Fatalf("clean operand gate exit = %d, want 0 (green)\nstdout: %s\nstderr: %s", operand.code, operand.rawJSON, operand.rawStderr)
		}
		if len(operand.defects) != 0 {
			t.Fatalf("clean operand gate reported %d defect(s):\n%s", len(operand.defects), strings.Join(operand.defects, "\n"))
		}
	})

	t.Run("injected violation — identical named-defect verdict", func(t *testing.T) {
		repo, env := scratchRepo(t, r)
		injectGateViolation(t, repo)
		operand := runGateOperand(t, repo, env)
		source := runGateSource(t, repo, env)
		assertSameVerdict(t, source, operand)
		// The injected violation reds the gate both ways with a non-empty,
		// identical defect stream — the parity proof at a non-trivial verdict.
		if operand.code != 1 {
			t.Fatalf("violated operand gate exit = %d, want 1 (named defect)\nstdout: %s\nstderr: %s", operand.code, operand.rawJSON, operand.rawStderr)
		}
		if len(operand.defects) == 0 {
			t.Fatalf("injected violation produced no defects — the fixture no longer reds the gate")
		}
	})
}

// runGateOperand runs the OPERAND path: `pyxis exec docgate -- gate
// --defects-json -` in the converged operand, via resolve+reporoot+execve.
func runGateOperand(t *testing.T, repo string, env []string) gateVerdict {
	t.Helper()
	res := pyxis(t, repo, env, "exec", "docgate", "--", "gate", "--defects-json", "-")
	return gateVerdict{
		code:      res.code,
		defects:   sortedJSONLines(res.stdout),
		rawJSON:   res.stdout,
		rawStderr: res.stderr,
	}
}

// runGateSource runs the SOURCE path: the docgate binary invoked directly with
// the operand as its working directory — the source-mode delegate's contract
// (locate-and-delegate), the same bytes without the acquisition indirection.
func runGateSource(t *testing.T, repo string, env []string) gateVerdict {
	t.Helper()
	requireHarness(t)
	cmd := exec.Command(builtBins["docgate"], "gate", "--defects-json", "-")
	cmd.Dir = repo
	cmd.Env = env
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	code := 0
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if asExit(err, &ee) {
			code = ee.ExitCode()
		} else {
			t.Fatalf("source-path docgate gate: launch failed: %v\n%s", err, stderr.String())
		}
	}
	return gateVerdict{
		code:      code,
		defects:   sortedJSONLines(stdout.String()),
		rawJSON:   stdout.String(),
		rawStderr: stderr.String(),
	}
}

// assertSameVerdict is the zero-divergence assertion: the two paths agree on
// the exit code AND on the exact set of JSON defect lines (order-independent,
// since the two invocation paths need not fix a defect ordering — the SET is
// the verdict).
func assertSameVerdict(t *testing.T, source, operand gateVerdict) {
	t.Helper()
	if source.code != operand.code {
		t.Fatalf("verdict divergence: source exit %d vs operand exit %d (NR-2)\n source stderr: %s\n operand stderr: %s",
			source.code, operand.code, source.rawStderr, operand.rawStderr)
	}
	if len(source.defects) != len(operand.defects) {
		t.Fatalf("verdict divergence: source has %d defect(s), operand has %d (NR-2)\n source: %s\n operand: %s",
			len(source.defects), len(operand.defects), strings.Join(source.defects, "\n"), strings.Join(operand.defects, "\n"))
	}
	for i := range source.defects {
		if source.defects[i] != operand.defects[i] {
			t.Fatalf("verdict divergence at defect[%d] (NR-2):\n source:  %s\n operand: %s", i, source.defects[i], operand.defects[i])
		}
	}
}

// injectGateViolation plants one corruption a fresh gate names deterministically
// through BOTH paths: it rewrites the generated design index (index.json) with
// bytes the generator would never emit, so the gate's verify-generated /
// verify-index leg reports a named divergence. The generated class is the
// honest choice — the operand path and the source path derive the SAME expected
// bytes from the SAME (empty) corpus, so the defect is identical either way.
func injectGateViolation(t *testing.T, repo string) {
	t.Helper()
	// A structurally-parseable but content-wrong index: not the empty-corpus
	// output the generator produces, so verify-index names the drift.
	writeFile(t, at(repo, "docs/tech-design/index.json"),
		[]byte(`{"schema_version":2,"designs":[{"slug":"ghost","status":"Approved"}]}`+"\n"), 0o644)
}

// sortedJSONLines splits a `--defects-json -` stdout into its non-empty JSON
// lines and sorts them — the defect SET, order-independent, so the two
// invocation paths are compared on content not on any incidental ordering.
func sortedJSONLines(stdout string) []string {
	var lines []string
	for _, ln := range strings.Split(stdout, "\n") {
		if strings.TrimSpace(ln) != "" {
			lines = append(lines, ln)
		}
	}
	sort.Strings(lines)
	return lines
}
