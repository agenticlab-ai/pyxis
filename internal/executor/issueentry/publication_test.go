package issueentry

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func TestSummaryPublicationRoundTripAndNativeFailures(t *testing.T) {
	for _, failure := range []string{"", "concurrent edit", "read failed", "write failed", "readback failed"} {
		t.Run(failure, func(t *testing.T) {
			s := (&Collector{}).Summary(Contract{ID: "sha256:contract"}, "carina:opaque#17", 77, "sha", "review_pending", 1)
			body, reads, writes := "Authored body", 0, 0
			read := func(context.Context) (string, error) {
				reads++
				if (failure == "read failed" && reads == 1) || (failure == "readback failed" && writes > 0) {
					return "", fmt.Errorf("native read failed")
				}
				snapshot := body
				if failure == "concurrent edit" && reads == 1 {
					body += "\nConcurrent authored edit"
				}
				return snapshot, nil
			}
			write := func(_ context.Context, merged string) error {
				writes++
				if failure == "write failed" {
					return fmt.Errorf("native write failed")
				}
				body = merged
				return nil
			}
			err := PublishSummary(t.Context(), s, read, write)
			wantError := failure != "" && failure != "concurrent edit"
			if (err != nil) != wantError {
				t.Fatalf("publication result: %v", err)
			}
			if !wantError {
				if err := PublishSummary(t.Context(), s, read, write); err != nil || writes != 1 {
					t.Fatalf("publication was not idempotent: %v, %d writes", err, writes)
				}
				if !strings.HasPrefix(body, "Authored body") || (failure == "concurrent edit" && !strings.Contains(body, "Concurrent authored edit")) {
					t.Fatal("authored body was overwritten")
				}
			}
		})
	}
}

func TestSummaryPublicationBoundsConflictsAndRefusesCorruptReadback(t *testing.T) {
	for _, failure := range []string{"continual edits", "corrupt readback", "fresh read unavailable"} {
		t.Run(failure, func(t *testing.T) {
			s := (&Collector{}).Summary(Contract{ID: "sha256:contract"}, "carina:opaque#17", 77, "sha", "review_pending", 1)
			reads, writes := 0, 0
			body := "Authored body"
			read := func(context.Context) (string, error) {
				reads++
				if failure == "fresh read unavailable" && reads == 2 {
					return "", fmt.Errorf("native read unavailable")
				}
				if failure == "continual edits" {
					return fmt.Sprintf("Concurrent edit %d", reads), nil
				}
				return body, nil
			}
			write := func(context.Context, string) error {
				writes++
				body += "\n" + summaryStart + "\ncorrupt payload\n" + summaryEnd
				return nil
			}
			if err := PublishSummary(t.Context(), s, read, write); err == nil {
				t.Fatal("unstable or corrupt publication reported success")
			}
			if failure == "continual edits" && (writes != 0 || reads > 6) {
				t.Fatal("conflicting native body was written or retried without a bound")
			}
			if failure == "corrupt readback" && writes != 1 {
				t.Fatal("corrupt owned section was overwritten instead of refused")
			}
		})
	}
}

func TestSummaryMergeRetainsTerminalUsageAndRefusesForeignOrCorruptPayload(t *testing.T) {
	c := &Collector{RunID: "one/"}
	finish := c.Launched("codex", port.LaunchPurposeImplementation, port.LaunchCapture{NativeModel: "model"})
	provisional := c.Summary(Contract{ID: "sha256:contract"}, "carina:opaque#17", 77, "sha", "running", 1)
	finish(port.RunResult{Tokens: 25, TokensObserved: true}, nil)
	completed := c.Summary(Contract{ID: provisional.ContractID}, provisional.Source, 77, "sha", "review_pending", 2)
	body, err := MergeSummary("Authored body", completed)
	if err != nil {
		t.Fatal(err)
	}
	merged, err := MergeSummary(body, provisional)
	if err != nil || !strings.Contains(merged, `"status": "completed"`) || !strings.Contains(merged, `"value": 25`) || !strings.Contains(merged, `"terminal": "review_pending"`) {
		t.Fatalf("stale publisher erased terminal observations: %v %s", err, merged)
	}
	for _, change := range []string{"foreign contract", "corrupt JSON", "conflicting launch"} {
		t.Run(change, func(t *testing.T) {
			current := completed
			current.Attempts = append([]Attempt{}, completed.Attempts...)
			prior := body
			switch change {
			case "foreign contract":
				current.ContractID = "sha256:other"
			case "corrupt JSON":
				prior = strings.Replace(body, `"schema_version":`, `"schema_version"!`, 1)
			case "conflicting launch":
				current.Attempts[0].Model = "different-model"
			}
			if _, err := MergeSummary(prior, current); err == nil {
				t.Fatal("conflicting/corrupt public summary accepted")
			}
		})
	}
}
