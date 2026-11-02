package drivetogreen

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestCILogKeepsMiddleFailureAfterSecretScrubbing(t *testing.T) {
	const secret = "ghp_abcdefghijklmnopqrstuv1234567890"
	failure := "--- FAIL: TestKillReRun\n--- FAIL: TestKillReRun/SIGKILL\nkill_test.go:92: .pyxis/.pyxis-converge-2314382340 appeared\nGITHUB_TOKEN=" + secret + "\n"
	raw := "invocation head\n" + strings.Repeat("ok successful package\n", 2000) + failure + strings.Repeat("ok later package\n", 2000) + "terminal tail"
	result := NewRedactor(0, nil).Redact(raw)
	for _, want := range []string{"invocation head", "terminal tail", "TestKillReRun/SIGKILL", "kill_test.go:92", redactionMark, "clamped"} {
		if !strings.Contains(result, want) {
			t.Errorf("diagnostic omitted %q", want)
		}
	}
	if strings.Contains(result, secret) || strings.Count(result, "kill_test.go:92") != 1 || len(result) > DefaultMaxLogBytes+100 {
		t.Fatalf("unsafe or duplicated diagnostic: bytes=%d", len(result))
	}
}

func TestFailureExcerptsRespectSmallUTF8Budgets(t *testing.T) {
	log := "panic: " + strings.Repeat("€", 100) + "\nnearby diagnostic"
	for _, budget := range []int{0, 1, 2, 31, 128, 256} {
		got := failureExcerpts(log, budget)
		if len(got) > budget || !utf8.ValidString(got) {
			t.Fatalf("budget %d yielded %d invalid or oversized bytes", budget, len(got))
		}
	}
}

func TestCILogKeepsActualFailureAfterCompetingEarlyErrors(t *testing.T) {
	const secret = "ghp_abcdefghijklmnopqrstuv1234567890"
	for _, anchor := range []string{"##[error]early warning", "source.go: error: early warning"} {
		t.Run(anchor, func(t *testing.T) {
			raw := "invocation head\n" + strings.Repeat(anchor+"\n"+strings.Repeat("early details\n", 10), 200) +
				strings.Repeat("successful package\n", 1000) +
				"--- FAIL: TestActualFailure\nactual_test.go:42: broken invariant\nGITHUB_TOKEN=" + secret + "\n" +
				strings.Repeat("later successful package\n", 1000) + "terminal tail"
			r := NewRedactor(0, nil)
			got := r.Redact(raw)
			for _, want := range []string{"invocation head", "TestActualFailure", "actual_test.go:42", redactionMark, "terminal tail"} {
				if !strings.Contains(got, want) {
					t.Errorf("competing errors hid %q", want)
				}
			}
			if strings.Contains(got, secret) || len(got) > DefaultMaxLogBytes+100 || got != r.Redact(raw) {
				t.Fatal("diagnostic must be scrubbed, bounded and deterministic")
			}
		})
	}
}

func TestCILogSamplesEarlyAndLateFailuresWithinBudget(t *testing.T) {
	var raw strings.Builder
	for i := range 50 {
		fmt.Fprintf(&raw, "--- FAIL: TestFailure%d\ncase_test.go:%d: actual diagnostic\n", i, i+1)
		raw.WriteString(strings.Repeat("ordinary output\n", 100))
	}
	got := NewRedactor(2048, nil).Redact(raw.String())
	for _, want := range []string{"TestFailure0", "TestFailure49", "case_test.go:50"} {
		if !strings.Contains(got, want) {
			t.Errorf("bounded sample omitted %q", want)
		}
	}
	if len(got) > 2148 {
		t.Fatalf("unbounded diagnostic: %d", len(got))
	}
}

func TestFailureExcerptsRetainDiagnosticKindsAndBound(t *testing.T) {
	for _, anchor := range []string{"--- FAIL: case", "FAIL\tpackage", "panic: failure", "fatal error: failure", "WARNING: DATA RACE", "##[error]failure", "source.go: error: failure"} {
		t.Run(anchor, func(t *testing.T) {
			raw := "head\n" + strings.Repeat("ordinary output\n", 100) + anchor + "\n" + strings.Repeat("large diagnostic detail\n", 30) + "tail"
			result := NewRedactor(256, nil).Redact(raw)
			if !strings.Contains(result, anchor) || !strings.Contains(result, "tail") || len(result) > 356 {
				t.Fatalf("bounded failure missing: %q", result)
			}
		})
	}
	if got := failureExcerpts("--- FAIL: first\n--- FAIL: last", 256); strings.Count(got, "first") != 1 || strings.Count(got, "last") != 1 {
		t.Fatalf("overlap or edge lost: %q", got)
	}
}
