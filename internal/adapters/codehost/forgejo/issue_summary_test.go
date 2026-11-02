package forgejocodehost

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/issueentry"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func TestIssueSummaryPreservesConcurrentDescriptionAndAttempts(t *testing.T) {
	for _, edit := range []string{"description", "newer attempts"} {
		t.Run(edit, func(t *testing.T) {
			p := pullFixture()
			p.Body = "Authored body"
			reads := 0
			collector := &issueentry.Collector{RunID: "first/"}
			collector.Launched("codex", port.LaunchPurposeImplementation, port.LaunchCapture{NativeModel: "model"})(port.RunResult{Tokens: 10, TokensObserved: true}, nil)
			summary := collector.Summary(issueentry.Contract{ID: "sha256:contract"}, "carina:opaque#17", 7, "sha", "review_pending", 123)
			other := &issueentry.Collector{RunID: "second/"}
			other.Launched("codex", port.LaunchPurposeReview, port.LaunchCapture{NativeModel: "review-model"})(port.RunResult{Tokens: 20, TokensObserved: true}, nil)
			newer := other.Summary(issueentry.Contract{ID: summary.ContractID}, summary.Source, 7, "sha", "review_pending", 234)
			h := testHost(t, func(r *http.Request) (any, int) {
				if r.Method == "PATCH" {
					var input struct {
						Body string `json:"body"`
					}
					if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
						t.Fatal(err)
					}
					p.Body = input.Body
					return p, 200
				}
				reads++
				before := p
				if reads == 1 {
					if edit == "description" {
						p.Body += "\nConcurrent authored edit"
					} else {
						var err error
						p.Body, err = issueentry.MergeSummary(p.Body, newer)
						if err != nil {
							t.Fatal(err)
						}
					}
				}
				return before, 200
			})
			if err := h.PublishIssueSummary(t.Context(), port.Proposal{Number: 7}, summary); err != nil {
				t.Fatal(err)
			}
			want := "Concurrent authored edit"
			if edit == "newer attempts" {
				want = newer.Attempts[0].ID
			}
			if !strings.Contains(p.Body, want) || !strings.Contains(p.Body, summary.Attempts[0].ID) {
				t.Fatalf("concurrent %s was overwritten: %s", edit, p.Body)
			}
		})
	}
}

func TestIssueSummaryUpdatesOnlyOwnedSectionWithReadback(t *testing.T) {
	for _, bad := range []bool{false, true} {
		t.Run(map[bool]string{false: "round-trip", true: "bad readback"}[bad], func(t *testing.T) {
			p := pullFixture()
			p.Body = "Authored explanation\n<!-- task association -->"
			writes := 0
			h := testHost(t, func(r *http.Request) (any, int) {
				if r.Method == "PATCH" {
					var input struct {
						Body string `json:"body"`
					}
					if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
						t.Fatal(err)
					}
					writes++
					if !bad {
						p.Body = input.Body
					}
					return p, 200
				}
				return p, 200
			})
			s := (&issueentry.Collector{}).Summary(issueentry.Contract{ID: "sha256:contract"}, "carina:opaque#17", 7, "abc", "review_pending", 123)
			err := h.PublishIssueSummary(t.Context(), port.Proposal{Number: 7}, s)
			if (err != nil) != bad {
				t.Fatalf("readback %v", err)
			}
			if !bad {
				if err := h.PublishIssueSummary(t.Context(), port.Proposal{Number: 7}, s); err != nil {
					t.Fatal(err)
				}
				if writes != 1 {
					t.Fatal("repeat publication wrote twice")
				}
			}
		})
	}
}

func TestIssueSummaryRefusesNativeReadWriteAndMalformedSection(t *testing.T) {
	for _, failure := range []string{"read denied", "wrong proposal", "invalid summary", "malformed section", "patch denied", "readback denied"} {
		t.Run(failure, func(t *testing.T) {
			p := pullFixture()
			reads := 0
			h := testHost(t, func(r *http.Request) (any, int) {
				if r.Method == "PATCH" {
					if failure == "patch denied" {
						return nil, http.StatusForbidden
					}
					return p, 200
				}
				reads++
				if (reads == 1 && failure == "read denied") || (reads == 3 && failure == "readback denied") {
					return nil, http.StatusForbidden
				}
				if failure == "malformed section" {
					p.Body = "<!-- pyxis:issue-execution-summary/v1 -->"
				}
				return p, 200
			})
			s := (&issueentry.Collector{}).Summary(issueentry.Contract{ID: "sha256:contract"}, "carina:opaque#17", 7, "sha", "review_pending", 123)
			if failure == "wrong proposal" {
				s.Proposal++
			}
			if failure == "invalid summary" {
				s.Schema = "unknown"
			}
			if err := h.PublishIssueSummary(t.Context(), port.Proposal{Number: 7}, s); err == nil {
				t.Fatal("unverified native summary accepted")
			}
		})
	}
}
