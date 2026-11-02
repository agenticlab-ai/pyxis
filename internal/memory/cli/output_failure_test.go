package cli

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
)

type outputFailure struct{ at, calls int }

func (w *outputFailure) Write(b []byte) (int, error) {
	w.calls++
	if w.calls == w.at {
		return 0, errors.New("output interrupted")
	}
	return len(b), nil
}

func assertEveryOutputFailure(t *testing.T, emit func(io.Writer) error) {
	t.Helper()
	counter := &outputFailure{}
	if err := emit(counter); err != nil {
		t.Fatal(err)
	}
	if counter.calls == 0 {
		t.Fatal("output fixture produced no writes")
	}
	for failAt := 1; failAt <= counter.calls; failAt++ {
		if err := emit(&outputFailure{at: failAt}); err == nil || !strings.Contains(err.Error(), "output interrupted") {
			t.Fatalf("write %d/%d reported success after lost output: %v", failAt, counter.calls, err)
		}
	}
}

func TestReconciliationReportsNeverCertifyLostOutput(t *testing.T) {
	t.Parallel()
	report := reconcile.Report{SourceCommit: strings.Repeat("a", 40), Findings: []reconcile.Finding{{Code: "gap", Scope: "scope", Blocking: true, Writable: true}}, Counts: reconcile.Counts{Blocking: 1, Findings: 1, Actions: 1}}
	assertEveryOutputFailure(t, func(w io.Writer) error { return writeReconciliationOutput(Env{Stdout: w}, false, 1, report) })
	assertEveryOutputFailure(t, func(w io.Writer) error {
		return writeReconciliationOutput(Env{Stdout: w}, false, 0, reconcile.Report{})
	})
	assertEveryOutputFailure(t, func(w io.Writer) error {
		return writeCloseoutOutput(Env{Stdout: w}, false, "sealed", "record", reconcile.CloseoutReceipt{Feature: "x"})
	})
	for _, blocking := range []bool{false, true} {
		for _, bootstrap := range []bool{false, true} {
			result := reconcile.BaselineResult{Improvements: []reconcile.BaselineDelta{{Code: "reduced", Scope: "one"}}}
			if blocking {
				result.Regressions = []reconcile.BaselineDelta{{Code: "new", Scope: "two"}}
			}
			base := &reconciliationBaselineRefOutput{Ref: "main", Commit: strings.Repeat("b", 40), Status: "compared", ReplayCommit: strings.Repeat("c", 40), Result: &result}
			if bootstrap {
				base.Status = "bootstrap"
			}
			assertEveryOutputFailure(t, func(w io.Writer) error {
				return writeReconciliationRatchetOutput(Env{Stdout: w}, false, reconcile.PolicyPath, "digest", false, base, result, report, policyEvidence{Schema: reconcile.PolicySchema})
			})
		}
	}
	assertEveryOutputFailure(t, func(w io.Writer) error {
		return writeReconciliationRatchetOutput(Env{Stdout: w}, false, reconcile.PolicyPath, "digest", true, nil, reconcile.BaselineResult{}, report, policyEvidence{})
	})
	assertEveryOutputFailure(t, func(w io.Writer) error {
		return writePolicySyncOutput(Env{Stdout: w}, false, policySyncOutput{Planned: []policyChange{{Path: reconcile.PolicyPath}}, Status: "incomplete"})
	})
}

func TestMemoryReadCommandsRefuseEveryLostOutputChunk(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	ids := contract(h)
	h.git("checkout", "-q", "-b", "output-check")
	h.git("commit", "--allow-empty", "-q", "-m", "candidate")
	for _, args := range [][]string{
		{"init"}, {"view", "prd/stacked-prs"}, {"view", "design/stacked-prs"}, {"why", ids["FR-1"]}, {"delivery", "prd/stacked-prs"}, {"status"},
		{"search", "executor", "--tag", "review"}, {"search", "qzxqzxnomatch"},
		{"collide", "--subject", "prd/stacked-prs", "--family", "FR-1", "--statement", "When a task produces more than one reviewable slice, the executor shall open one PR per slice, each based on the previous."},
		{"collide", "--statement", "qzxqzxnomatch"},
		{"packet", "--base", "main"}, {"packet", "--base", "main", "--body"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			counter := &outputFailure{}
			env, _, stderr := h.env("")
			env.Stdout = counter
			if code := Run(t.Context(), args, env); code != exitOK {
				t.Fatalf("fixture: %d %s", code, stderr.String())
			}
			for failAt := 1; failAt <= counter.calls; failAt++ {
				env, _, _ = h.env("")
				env.Stdout = &outputFailure{at: failAt}
				if code := Run(t.Context(), args, env); code != exitRefused {
					t.Fatalf("lost output %d/%d: exit %d", failAt, counter.calls, code)
				}
			}
		})
	}
	for _, args := range [][]string{{"help"}, {"not-a-command"}, {"add", "--unknown"}, {"why", "not-a-record"}} {
		env, _, _ := h.env("")
		env.Stderr = rejectingWriter{err: errors.New("output interrupted")}
		if code := Run(t.Context(), args, env); code != exitRefused {
			t.Fatalf("lost diagnostic output for %v: %d", args, code)
		}
	}
}

func TestMemoryWritersReportLostReceipts(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"init"},
		{"add", "--kind", "Learning", "--subject", "meta/x", "--statement", "durable result"},
		{"ask", "--subject", "prd/x", "--text", "Which behavior?"},
	} {
		t.Run(args[0], func(t *testing.T) {
			h := initRepo(t)
			env, _, stderr := h.env("")
			env.Stdout = &outputFailure{at: 1}
			if code := Run(t.Context(), args, env); code != exitRefused || !strings.Contains(stderr.String(), "output interrupted") {
				t.Fatalf("lost write receipt: %d %s", code, stderr.String())
			}
		})
	}
	h := initRepo(t)
	code, _, errout := h.runStdin(`{"kind":"Document","subject":"prd/x","family":"prd","statement":"body","body":"# Body\n"}`, "add", "--json")
	if code != exitOK {
		t.Fatal(errout)
	}
	env, _, _ := h.env("")
	env.Stdout = &outputFailure{at: 1}
	if code := Run(t.Context(), []string{"render", "prd/x"}, env); code != exitRefused {
		t.Fatalf("render output lost: %d", code)
	}
}
