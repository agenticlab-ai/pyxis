package ghcodehost

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/issueentry"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func TestIssueSummaryPreservesConcurrentDescriptionAndAttempts(t *testing.T) {
	for _, edit := range []string{"description", "newer attempts"} {
		t.Run(edit, func(t *testing.T) {
			body := "Authored body"
			reads := 0
			collector := &issueentry.Collector{RunID: "first/"}
			collector.Launched("codex", port.LaunchPurposeImplementation, port.LaunchCapture{NativeModel: "model"})(port.RunResult{Tokens: 10, TokensObserved: true}, nil)
			summary := collector.Summary(issueentry.Contract{ID: "sha256:contract"}, "https://github.com/o/r/issues/17", 77, "sha", "review_pending", 123)
			other := &issueentry.Collector{RunID: "second/"}
			other.Launched("codex", port.LaunchPurposeReview, port.LaunchCapture{NativeModel: "review-model"})(port.RunResult{Tokens: 20, TokensObserved: true}, nil)
			newer := other.Summary(issueentry.Contract{ID: summary.ContractID}, summary.Source, 77, "sha", "review_pending", 234)
			c := &ghClient{repo: "o/r", run: func(context.Context, ...string) ([]byte, error) {
				reads++
				result, err := json.Marshal(struct {
					Number int    `json:"number"`
					Body   string `json:"body"`
				}{77, body})
				if reads == 1 {
					if edit == "description" {
						body += "\nConcurrent authored edit"
					} else {
						body, err = issueentry.MergeSummary(body, newer)
					}
				}
				return result, err
			}, runInput: func(_ context.Context, b []byte, _ ...string) ([]byte, error) {
				var input struct {
					Body string `json:"body"`
				}
				if err := json.Unmarshal(b, &input); err != nil {
					return nil, err
				}
				body = input.Body
				return nil, nil
			}}
			if err := newHost(c, "main").PublishIssueSummary(t.Context(), port.Proposal{Number: 77}, summary); err != nil {
				t.Fatal(err)
			}
			want := "Concurrent authored edit"
			if edit == "newer attempts" {
				want = newer.Attempts[0].ID
			}
			if !strings.Contains(body, want) || !strings.Contains(body, summary.Attempts[0].ID) {
				t.Fatalf("concurrent %s was overwritten: %s", edit, body)
			}
		})
	}
}

func TestIssueSummaryPreservesBodyAndChecksNativeReadback(t *testing.T) {
	for _, badReadback := range []bool{false, true} {
		t.Run(map[bool]string{false: "round-trip", true: "wrong readback"}[badReadback], func(t *testing.T) {
			body := "Authored body\n<!-- task association -->"
			writes := 0
			c := &ghClient{repo: "o/r", run: func(_ context.Context, args ...string) ([]byte, error) {
				if args[len(args)-1] != "repos/o/r/pulls/77" {
					return nil, errors.New("wrong proposal")
				}
				return json.Marshal(struct {
					Number int    `json:"number"`
					Body   string `json:"body"`
				}{77, body})
			}, runInput: func(_ context.Context, b []byte, args ...string) ([]byte, error) {
				var input struct {
					Body string `json:"body"`
				}
				if err := json.Unmarshal(b, &input); err != nil {
					return nil, err
				}
				writes++
				if !badReadback {
					body = input.Body
				}
				return []byte(`{}`), nil
			}}
			s := (&issueentry.Collector{}).Summary(issueentry.Contract{ID: "sha256:contract"}, "https://github.com/o/r/issues/17", 77, "sha", "review_pending", 123)
			h := newHost(c, "main")
			err := h.PublishIssueSummary(t.Context(), port.Proposal{Number: 77}, s)
			if (err != nil) != badReadback {
				t.Fatalf("readback err=%v", err)
			}
			if !badReadback {
				if err := h.PublishIssueSummary(t.Context(), port.Proposal{Number: 77}, s); err != nil {
					t.Fatal(err)
				}
				if writes != 1 {
					t.Fatal("repeat publication wrote twice")
				}
			}
		})
	}
}

func TestIssueSummaryRefusesNativeTransportAndIdentityFailures(t *testing.T) {
	summary := (&issueentry.Collector{}).Summary(issueentry.Contract{ID: "sha256:contract"}, "https://github.com/o/r/issues/17", 77, "sha", "review_pending", 123)
	for _, failure := range []string{"read denied", "invalid read JSON", "wrong proposal", "wrong summary proposal", "invalid summary", "malformed section", "budget unavailable", "patch denied", "readback denied", "invalid readback JSON", "wrong readback identity"} {
		t.Run(failure, func(t *testing.T) {
			reads, writes := 0, 0
			c := &ghClient{repo: "o/r"}
			c.run = func(context.Context, ...string) ([]byte, error) {
				reads++
				if (reads == 1 && failure == "read denied") || (reads == 3 && failure == "readback denied") {
					return nil, errors.New("denied")
				}
				if (reads == 1 && failure == "invalid read JSON") || (reads == 3 && failure == "invalid readback JSON") {
					return []byte("broken"), nil
				}
				number := 77
				if failure == "wrong proposal" || (reads == 3 && failure == "wrong readback identity") {
					number++
				}
				body := "Authored body"
				if failure == "malformed section" {
					body += "<!-- pyxis:issue-execution-summary/v1 -->"
				}
				return json.Marshal(struct {
					Number int    "json:\"number\""
					Body   string "json:\"body\""
				}{number, body})
			}
			c.runInput = func(context.Context, []byte, ...string) ([]byte, error) {
				writes++
				if failure == "patch denied" {
					return nil, errors.New("denied")
				}
				return nil, nil
			}
			if failure == "budget unavailable" {
				c.budgetErr = errors.New("unavailable")
			}
			s := summary
			if failure == "wrong summary proposal" {
				s.Proposal++
			}
			if failure == "invalid summary" {
				s.Schema = "unknown"
			}
			if err := newHost(c, "main").PublishIssueSummary(t.Context(), port.Proposal{Number: 77}, s); err == nil {
				t.Fatal("native summary boundary accepted an unverified write/read")
			}
			if writes > 1 {
				t.Fatal("unexpected publication retries")
			}
		})
	}
	if err := newHost(&fakeClient{}, "main").PublishIssueSummary(t.Context(), port.Proposal{Number: 77}, summary); err == nil {
		t.Fatal("invented a native summary capability")
	}
}
