package issueentry

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/taskrouting"
)

const SummarySchema = "issue-execution-summary/v1"
const summaryStart = "<!-- pyxis:issue-execution-summary/v1 -->"
const summaryEnd = "<!-- /pyxis:issue-execution-summary/v1 -->"

type Counter struct {
	State  string `json:"state"`
	Value  *int64 `json:"value,omitempty"`
	Reason string `json:"reason,omitempty"`
}

func Unavailable(reason string) Counter { return Counter{State: "unavailable", Reason: reason} }
func observed(value int) Counter        { v := int64(value); return Counter{State: "observed", Value: &v} }

type Attempt struct {
	ID        string         `json:"attempt_id"`
	Role      string         `json:"role"`
	Harness   entity.Harness `json:"harness"`
	Model     string         `json:"model"`
	Effort    string         `json:"effort"`
	Status    string         `json:"status"`
	Tokens    Counter        `json:"tokens"`
	Turns     Counter        `json:"turns"`
	ToolCalls Counter        `json:"tool_calls"`
}
type Summary struct {
	Schema             string    `json:"schema_version"`
	ContractID         string    `json:"contract_id"`
	Source             string    `json:"source"`
	Proposal           int       `json:"proposal_number"`
	Revision           string    `json:"revision,omitempty"`
	Terminal           string    `json:"terminal"`
	ExecutionElapsedMS int64     `json:"execution_elapsed_ms"`
	TimingBoundary     string    `json:"timing_boundary"`
	UsageScope         string    `json:"usage_scope"`
	Attempts           []Attempt `json:"attempts"`
	TotalTokens        Counter   `json:"total_tokens"`
	IntakeTokens       Counter   `json:"intake_tokens"`
	IntakeElapsedMS    Counter   `json:"intake_elapsed_ms"`
}
type SummaryPublisher interface {
	PublishIssueSummary(context.Context, port.Proposal, Summary) error
}

// Collector observes only processes launched by this invocation. It never sums
// review artifacts again, which would count the same native usage twice.
type Collector struct {
	mu       sync.Mutex
	attempts []Attempt
	RunID    string
}

func (c *Collector) Launched(harness entity.Harness, purpose port.LaunchPurpose, capture port.LaunchCapture) func(port.RunResult, error) {
	c.mu.Lock()
	role := "implementation"
	if purpose == port.LaunchPurposeReview {
		role = "review"
	}
	index := len(c.attempts)
	c.attempts = append(c.attempts, Attempt{ID: fmt.Sprintf("%sinvocation-%d", c.RunID, index+1), Role: role, Harness: harness, Model: capture.NativeModel, Effort: capture.NativeEffort, Status: "running", Tokens: Unavailable("terminal usage not observed"), Turns: Unavailable("terminal usage not observed"), ToolCalls: Unavailable("adapter does not report tool counts")})
	c.mu.Unlock()
	var once sync.Once
	return func(result port.RunResult, err error) {
		once.Do(func() {
			c.mu.Lock()
			defer c.mu.Unlock()
			a := &c.attempts[index]
			a.Status = "completed"
			if err != nil || result.Incomplete {
				a.Status = "failed"
			}
			if result.Tokens >= 0 && (result.TokensObserved || result.Tokens > 0) {
				a.Tokens = observed(result.Tokens)
				if !result.TokensObserved {
					a.Tokens.State = "partial"
					a.Tokens.Reason = "adapter did not confirm complete token accounting"
				}
			} else {
				a.Tokens = Unavailable("adapter did not report token usage")
			}
			if result.Turns >= 0 && (result.TurnsObserved || result.Turns > 0) {
				a.Turns = observed(result.Turns)
				if !result.TurnsObserved {
					a.Turns.State = "partial"
					a.Turns.Reason = "adapter did not confirm complete turn accounting"
				}
			} else {
				a.Turns = Unavailable("adapter did not report turn count")
			}
			if result.ToolCalls >= 0 && (result.ToolCallsObserved || result.ToolCalls > 0) {
				a.ToolCalls = observed(result.ToolCalls)
				if !result.ToolCallsObserved {
					a.ToolCalls.State = "partial"
					a.ToolCalls.Reason = "adapter did not confirm complete tool accounting"
				}
			}
		})
	}
}
func (c *Collector) Summary(contract Contract, source string, proposal int, revision, terminal string, elapsed int64) Summary {
	c.mu.Lock()
	defer c.mu.Unlock()
	attempts := append([]Attempt{}, c.attempts...)
	total := int64(0)
	known := 0
	partial := false
	for _, a := range attempts {
		if a.Tokens.Value != nil {
			total += *a.Tokens.Value
			known++
			partial = partial || a.Tokens.State == "partial"
		}
	}
	counter := Unavailable("no token observations in this invocation")
	if known > 0 {
		counter = Counter{State: "observed", Value: &total}
		if known < len(attempts) || partial {
			counter.State = "partial"
			counter.Reason = "some invocation usage is unavailable"
		}
	}
	return Summary{Schema: SummarySchema, ContractID: contract.ID, Source: source, Proposal: proposal, Revision: revision, Terminal: terminal, ExecutionElapsedMS: elapsed, TimingBoundary: "command_start_to_terminal_excludes_final_summary_write_and_intake", UsageScope: "current_invocation", Attempts: attempts, TotalTokens: counter, IntakeTokens: Unavailable("invoking session is not a bounded executor process"), IntakeElapsedMS: Unavailable("intake occurs before command entry")}
}
func validateCounter(c Counter) error {
	switch c.State {
	case "observed":
		if c.Value == nil || *c.Value < 0 || c.Reason != "" {
			return fmt.Errorf("invalid observed counter")
		}
	case "partial":
		if c.Value == nil || *c.Value < 0 || c.Reason == "" {
			return fmt.Errorf("invalid partial counter")
		}
	case "unavailable":
		if c.Value != nil || c.Reason == "" {
			return fmt.Errorf("invalid unavailable counter")
		}
	default:
		return fmt.Errorf("unknown counter state %q", c.State)
	}
	return nil
}
func DecodeSummary(b []byte) (Summary, error) {
	var s Summary
	if err := taskrouting.Decode(b, &s); err != nil {
		return s, err
	}
	return s, validateSummary(s)
}
func validateSummary(s Summary) error {
	if s.Schema != SummarySchema || s.ContractID == "" || s.Source == "" || s.Proposal <= 0 || s.ExecutionElapsedMS < 0 || s.TimingBoundary != "command_start_to_terminal_excludes_final_summary_write_and_intake" || (s.UsageScope != "current_invocation" && s.UsageScope != "retained_invocations") {
		return fmt.Errorf("invalid execution summary identity")
	}
	switch s.Terminal {
	case "running", "verified_pr", "review_pending", "verification_pending", "implementation_failed":
	default:
		return fmt.Errorf("unknown execution terminal %q", s.Terminal)
	}
	for _, counter := range []Counter{s.TotalTokens, s.IntakeTokens, s.IntakeElapsedMS} {
		if err := validateCounter(counter); err != nil {
			return err
		}
	}
	seen := make(map[string]bool)
	for _, a := range s.Attempts {
		if a.ID == "" || seen[a.ID] || (a.Role != "implementation" && a.Role != "review") || a.Harness == "" {
			return fmt.Errorf("invalid attempt identity/observed route")
		}
		seen[a.ID] = true
		if a.Status != "running" && a.Status != "completed" && a.Status != "failed" {
			return fmt.Errorf("unknown attempt status")
		}
		for _, counter := range []Counter{a.Tokens, a.Turns, a.ToolCalls} {
			if err := validateCounter(counter); err != nil {
				return err
			}
		}
	}
	return nil
}
func MergeSummary(body string, s Summary) (string, error) {
	if err := validateSummary(s); err != nil {
		return "", err
	}
	if strings.Count(body, summaryStart) > 1 || strings.Count(body, summaryEnd) > 1 {
		return "", fmt.Errorf("ambiguous execution summary markers")
	}
	start := strings.Index(body, summaryStart)
	end := strings.Index(body, summaryEnd)
	if (start < 0) != (end < 0) || (start >= 0 && end < start) {
		return "", fmt.Errorf("malformed execution summary markers")
	}
	if start >= 0 {
		section := body[start:end]
		opening := "\n```json\n"
		begin, finish := strings.Index(section, opening), strings.LastIndex(section, "\n```")
		if begin < 0 || finish < begin+len(opening) {
			return "", fmt.Errorf("malformed execution summary payload")
		}
		prior, err := DecodeSummary([]byte(section[begin+len(opening) : finish]))
		if err != nil {
			return "", err
		}
		if prior.ContractID != s.ContractID || prior.Source != s.Source || prior.Proposal != s.Proposal {
			return "", fmt.Errorf("conflicting issue summary identity")
		}
		if !reflect.DeepEqual(prior.Attempts, s.Attempts) {
			s, err = mergeObservations(prior, s)
			if err != nil {
				return "", err
			}
		}
	}
	wire, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}
	var section strings.Builder
	fmt.Fprintf(&section, "%s\n## Pyxis execution\n\nStatus: %s. Execution: %d ms (command start to terminal; excludes intake and final metadata write).\n\nContract: %s\n\n| Role | Harness | Model | Effort | Tokens | Tool calls |\n|---|---|---|---|---|---|\n", summaryStart, s.Terminal, s.ExecutionElapsedMS, s.ContractID)
	for _, a := range s.Attempts {
		counterText := func(c Counter) string {
			if c.Value == nil {
				return "unavailable"
			}
			value := fmt.Sprint(*c.Value)
			if c.State == "partial" {
				return value + " (partial)"
			}
			return value
		}
		clean := func(v string) string {
			if v == "" {
				return "unavailable"
			}
			return strings.NewReplacer("|", "\\|", "\n", " ").Replace(v)
		}
		fmt.Fprintf(&section, "| %s | %s | %s | %s | %s | %s |\n", a.Role, clean(a.Harness.String()), clean(a.Model), clean(a.Effort), counterText(a.Tokens), counterText(a.ToolCalls))
	}
	fmt.Fprintf(&section, "\nUsage scope: %s. Only retained native launch observations are counted; missing role data is unavailable. Duration covers the latest command invocation. Tool counts and intake usage are unavailable unless observed by an adapter.\n\n\x60\x60\x60json\n%s\n\x60\x60\x60\n%s", s.UsageScope, wire, summaryEnd)
	if start < 0 && end < 0 {
		return strings.TrimRight(body, "\n") + "\n\n" + section.String() + "\n", nil
	}
	return body[:start] + section.String() + body[end+len(summaryEnd):], nil
}
