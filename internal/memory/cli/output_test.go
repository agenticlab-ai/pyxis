package cli

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
)

// A report may be piped to a process that exits or to a full filesystem.
// Every truncated prefix must remain an error, including a lost final verdict.
func TestIntegrityReportsRefuseTruncatedOutput(t *testing.T) {
	t.Parallel()
	blocked := reconcile.Report{
		SourceCommit: strings.Repeat("a", 40),
		Counts:       reconcile.Counts{Blocking: 1, Actions: 1},
		Findings: []reconcile.Finding{
			{Code: "missing-document", Scope: "README.md", Blocking: true, Writable: true},
		},
	}
	delta := reconcile.BaselineDelta{Code: "missing-document", Scope: "README.md", Reason: "new finding"}
	regression := reconcile.BaselineResult{Regressions: []reconcile.BaselineDelta{delta}}
	improvement := reconcile.BaselineResult{Improvements: []reconcile.BaselineDelta{delta}}
	for _, asJSON := range []bool{false, true} {
		tests := []struct {
			name string
			run  func(io.Writer) error
		}{
			{"blocked", func(w io.Writer) error { return writeReconciliationOutput(Env{Stdout: w}, asJSON, 1, blocked) }},
			{"clear", func(w io.Writer) error {
				return writeReconciliationOutput(Env{Stdout: w}, asJSON, 0, reconcile.Report{})
			}},
			{"receipt", func(w io.Writer) error {
				return writeCloseoutOutput(Env{Stdout: w}, asJSON, "sealed", "Document-receipt", reconcile.CloseoutReceipt{Feature: "widget"})
			}},
		}
		for _, tc := range []struct {
			name   string
			exact  bool
			ref    *reconciliationBaselineRefOutput
			result reconcile.BaselineResult
		}{
			{"regression", true, &reconciliationBaselineRefOutput{Status: "compared", ReplayCommit: strings.Repeat("b", 40), Result: &regression}, regression},
			{"improvement", false, &reconciliationBaselineRefOutput{Status: "compared", Result: &improvement}, improvement},
			{"bootstrap", true, &reconciliationBaselineRefOutput{Status: "bootstrap"}, reconcile.BaselineResult{}},
		} {
			tests = append(tests, struct {
				name string
				run  func(io.Writer) error
			}{tc.name, func(w io.Writer) error {
				return writeReconciliationRatchetOutput(Env{Stdout: w}, asJSON, reconciliationBaselinePath, "digest", tc.exact, tc.ref, tc.result, blocked, policyEvidence{})
			}})
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				var full bytes.Buffer
				if err := tc.run(&full); err != nil {
					t.Fatal(err)
				}
				if full.Len() == 0 {
					t.Fatal("empty report")
				}
				for limit := 0; limit < full.Len(); limit++ {
					w := &limitedOutput{remaining: limit}
					if err := tc.run(w); !errors.Is(err, io.ErrClosedPipe) {
						t.Fatalf("accepted truncated report at byte %d/%d: %v", limit, full.Len(), err)
					}
				}
			})
		}
	}
}

type limitedOutput struct{ remaining int }

func (w *limitedOutput) Write(p []byte) (int, error) {
	if len(p) > w.remaining {
		n := w.remaining
		w.remaining = 0
		return n, io.ErrClosedPipe
	}
	w.remaining -= len(p)
	return len(p), nil
}

func TestReadCommandsRefuseClosedOutput(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	ids := contract(h)
	doc := h.add("--kind", "Document", "--subject", "design/stacked-prs", "--family", "design", "--statement", "Stack design", "--body-file", "README.md", "--literal-body")
	h.commit("narrative")
	h.git("checkout", "-qb", "topic")
	h.add("--kind", "Learning", "--subject", "learning/proposal", "--statement", "A proposal has a reviewable change.")
	for _, args := range [][]string{
		{"init"}, {"render", "design/stacked-prs"}, {"view", "prd/stacked-prs"}, {"view", "design/stacked-prs"},
		{"why", doc}, {"delivery", "prd/stacked-prs"}, {"status"},
		{"search", "executor"}, {"search", "absentzzzz"},
		{"collide", "--statement", "executor manifest", "--subject", "design/stacked-prs"},
		{"collide", "--statement", "absentzzzz"},
		{"pack", "--subject", "prd/stacked-prs"},
		{"packet", "--base", "main"}, {"packet", "--base", "main", "--body"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			// First prove this command reaches a successful output boundary.
			h.ok(args...)
			env, _, stderr := h.env("")
			env.Stdout = &limitedOutput{}
			if code := Run(t.Context(), args, env); code != exitRefused || !strings.Contains(stderr.String(), io.ErrClosedPipe.Error()) {
				t.Fatalf("output failure = %d, %s", code, stderr.String())
			}
		})
	}
	// Usage and failure diagnostics must also propagate a broken stderr.
	for _, args := range [][]string{nil, {"unknown"}, {"why"}, {"why", "missing"}} {
		env, _, _ := h.env("")
		env.Stderr = &limitedOutput{}
		if code := Run(t.Context(), args, env); code != exitRefused {
			t.Fatalf("diagnostic output failure = %d for %v", code, args)
		}
	}
	// A failed acknowledgement does not erase a successfully stored record.
	env, _, stderr := h.env("")
	env.Stdout = &limitedOutput{}
	if code := Run(t.Context(), []string{"add", "--kind", "Learning", "--subject", "learning/output", "--statement", "Keep the durable write when its acknowledgement fails."}, env); code != exitRefused || !strings.Contains(stderr.String(), io.ErrClosedPipe.Error()) {
		t.Fatalf("write acknowledgement = %d, %s", code, stderr.String())
	}
	want(t, h.ok("view", "learning/output"), "Keep the durable write")
	question := h.add("--kind", "Question", "--subject", "prd/output", "--statement", "Which output is durable?")
	for _, answered := range []bool{false, true} {
		if answered {
			h.ok("answer", question, "--text", "The stored record.")
		}
		env, _, stderr := h.env("")
		env.Stdout = &limitedOutput{}
		if code := Run(t.Context(), []string{"ask", "--subject", "prd/output", "--text", "Which output is durable?"}, env); code != exitRefused || !strings.Contains(stderr.String(), io.ErrClosedPipe.Error()) {
			t.Fatalf("existing question acknowledgement = %d, %s", code, stderr.String())
		}
	}
	// Keep the seeded contract live so read failures cannot be false positives
	// caused by a projection or admission failure.
	want(t, h.ok("why", ids["FR-1"]), "executor")
}
