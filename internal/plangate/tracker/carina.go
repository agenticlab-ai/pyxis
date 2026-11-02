// The Carina adapter — the second Tracker implementation (design:
// plangate-carina-adapter). It projects a plan onto the local (Carina)
// WorkTracker by driving the `carina` CLI, exactly as the gh adapter drives
// `gh`: every Carina-ism lives here, behind the same interface. Items are
// addressed by Carina's own sequential id (DINV-4); plangate labels map to
// Carina signals with an auto-define-as-classification fallback (DINV-3);
// reads return item bodies so the marker probe converges (DINV-5). No
// GitHub-ism and no engine-vocabulary widening leaks out.
package tracker

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Carina is the local-WorkTracker-backed Tracker for one store.
type Carina struct {
	Root string // carina --root: the operand root holding the store
	Bin  string // resolved carina binary (main.go resolves it; R1/Q1)
}

// NewCarina binds the adapter to a store root and a resolved carina binary.
func NewCarina(root, bin string) *Carina { return &Carina{Root: root, Bin: bin} }

// carinaLabelMap maps a plangate label to its engine-recognized Carina signal.
// Everything not here (and not kind:*) lands on the operand-extensible
// classification axis via signal def — never widening the closed set (DINV-3).
var carinaLabelMap = map[string]string{
	"local-agent": "agent",    // the authorization analog
	"operator":    "operator", // routing (seeded)
}

// carinaSignal returns the Carina signal for a plangate label, or "" when the
// label is elided because the Carina type already carries it (epic).
func carinaSignal(label string) string {
	if label == "epic" {
		return "" // the --type carries the hierarchy role (DINV-3)
	}
	if s, ok := carinaLabelMap[label]; ok {
		return s
	}
	if strings.HasPrefix(label, "kind:") || strings.HasPrefix(label, "area:") {
		return label // already on the classification axis — passthrough
	}
	return "kind:" + label // unmapped → namespaced classification signal (DINV-3)
}

// carinaType maps a plangate issueType onto Carina's closed type set; an
// unknown type falls back to task rather than failing (legibility-additive).
func carinaType(issueType string) string {
	// The dialect passes native type names (e.g. "Feature", "Task"); match
	// case-insensitively onto Carina's closed type set. An unknown native type
	// falls back to task rather than failing (legibility-additive).
	switch t := strings.ToLower(strings.TrimSpace(issueType)); t {
	case "feature", "milestone", "epic", "task":
		return t
	default:
		return "task"
	}
}

// itemView mirrors carina's schema-versioned ItemView (--json), reading only
// the fields the projection consumes (AP-6 compactness).
type carinaItemView struct {
	ID        int      `json:"id"`
	Type      string   `json:"type"`
	State     string   `json:"state"`
	Title     string   `json:"title"`
	Signals   []string `json:"signals"`
	Parent    string   `json:"parent"`
	BlockedBy []string `json:"blocked_by"`
	Body      string   `json:"body"`
	Log       []struct {
		Kind    string `json:"kind"`
		Payload string `json:"payload"`
	} `json:"log"`
}

func (v carinaItemView) issue() Issue {
	labels := v.Signals
	if labels == nil {
		labels = []string{}
	}
	return Issue{Number: v.ID, Title: v.Title, Body: v.Body, Labels: labels, State: v.State, Type: v.Type}
}

type carinaListView struct {
	Items []carinaItemView `json:"items"`
}

// run execs `carina <verb> --root <root> [args…]` and returns stdout. A
// non-zero exit becomes an error carrying carina's stderr (the typed Defect).
func (c *Carina) run(args ...string) ([]byte, error) {
	full := append([]string{args[0], "--root", c.Root}, args[1:]...)
	cmd := exec.Command(c.Bin, full...)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return nil, fmt.Errorf("carina %s: %s", strings.Join(args, " "), strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("carina %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

// vocabulary returns the set of currently-defined signal names.
func (c *Carina) vocabulary() (map[string]bool, error) {
	out, err := c.run("signal", "--json", "ls")
	if err != nil {
		return nil, err
	}
	var v struct {
		Signals []struct {
			Name string `json:"name"`
		} `json:"signals"`
	}
	if err := json.Unmarshal(out, &v); err != nil {
		return nil, fmt.Errorf("parse signal vocabulary: %w", err)
	}
	set := make(map[string]bool, len(v.Signals))
	for _, s := range v.Signals {
		set[s.Name] = true
	}
	return set, nil
}

// ensureSignal defines sig on the classification axis iff it is absent —
// idempotent, and never touches an engine-recognized signal (DINV-3).
func (c *Carina) ensureSignal(sig, color, description string) error {
	vocab, err := c.vocabulary()
	if err != nil {
		return err
	}
	if vocab[sig] {
		return nil // already defined (seeded or previously projected)
	}
	_, err = c.run("signal", "def", sig, "classification", "--color", color, "--description", description)
	return err
}

// EnsureLabel maps the label to a Carina signal and, when it is an unmapped
// (operand) label, defines it on the classification axis; a mapped or elided
// label is a no-op (DINV-3). Never fails on an unknown label.
func (c *Carina) EnsureLabel(name, color, description string) error {
	sig := carinaSignal(name)
	if sig == "" {
		return nil // epic elided (the type carries it)
	}
	if _, mapped := carinaLabelMap[name]; mapped {
		return nil // maps to a seeded engine-recognized signal — never widen the set
	}
	// sig is classification-namespaced (kind:* / area:*): define iff absent — a
	// seeded kind:* is a no-op, a new one is materialized (DINV-3).
	return c.ensureSignal(sig, color, description)
}

// CreateIssue creates a work item and returns its Carina id (DINV-4). Labels
// are applied after creation; issueType maps onto Carina's type.
func (c *Carina) CreateIssue(title, body string, labels []string, issueType string) (int, error) {
	out, err := c.run("create", "--json", "--type", carinaType(issueType), "--title", title, "--body", body)
	if err != nil {
		return 0, err
	}
	var v carinaItemView
	if err := json.Unmarshal(out, &v); err != nil {
		return 0, fmt.Errorf("parse created item: %w", err)
	}
	if err := c.AddLabels(v.ID, labels); err != nil {
		return v.ID, err
	}
	return v.ID, nil
}

// AddLabels applies each label as its mapped Carina signal (defining unmapped
// ones on the classification axis first); additive and idempotent.
func (c *Carina) AddLabels(issue int, labels []string) error {
	for _, l := range labels {
		sig := carinaSignal(l)
		if sig == "" {
			continue // elided (epic)
		}
		if err := c.EnsureLabel(l, "ededed", "projected by plangate"); err != nil {
			return err
		}
		if _, err := c.run("signal", "add", strconv.Itoa(issue), sig); err != nil {
			return err
		}
	}
	return nil
}

// RemoveLabel withdraws the label's mapped Carina signal.
func (c *Carina) RemoveLabel(issue int, label string) error {
	sig := carinaSignal(label)
	if sig == "" {
		return nil // elided (epic)
	}
	_, err := c.run("signal", "rm", strconv.Itoa(issue), sig)
	return err
}

// LinkSubIssue sets child's parent to parent (additive; re-setting converges).
func (c *Carina) LinkSubIssue(parent, child int) error {
	_, err := c.run("parent", "set", strconv.Itoa(child), strconv.Itoa(parent))
	return err
}

// AddBlockedBy wires issue ← blocker (additive; re-adding converges).
func (c *Carina) AddBlockedBy(issue, blocker int) error {
	_, err := c.run("dep", "add", strconv.Itoa(issue), strconv.Itoa(blocker))
	return err
}

// RemoveBlockedBy drops the edge issue ← blocker.
func (c *Carina) RemoveBlockedBy(issue, blocker int) error {
	_, err := c.run("dep", "rm", strconv.Itoa(issue), strconv.Itoa(blocker))
	return err
}

// Comment appends a comment to the item's log.
func (c *Carina) Comment(issue int, body string) error {
	_, err := c.run("comment", strconv.Itoa(issue), "--body", body)
	return err
}

// OpenIssues enumerates open items (bodies included) — the stray-scan half of
// the marker probe.
func (c *Carina) OpenIssues() ([]Issue, error) {
	out, err := c.run("list", "--json", "--state", "open")
	if err != nil {
		return nil, err
	}
	return c.itemsToIssues(out)
}

// SubIssues enumerates a parent's direct children (bodies included). Interim:
// a full list filtered by parent, until FR-19's list --parent ships (Q2).
func (c *Carina) SubIssues(parent int) ([]Issue, error) {
	out, err := c.run("list", "--json")
	if err != nil {
		return nil, err
	}
	var lv carinaListView
	if err := json.Unmarshal(out, &lv); err != nil {
		return nil, fmt.Errorf("parse list: %w", err)
	}
	want := strconv.Itoa(parent)
	var issues []Issue
	for _, v := range lv.Items {
		if v.Parent == want {
			issues = append(issues, v.issue())
		}
	}
	return issues, nil
}

func (c *Carina) itemsToIssues(out []byte) ([]Issue, error) {
	var lv carinaListView
	if err := json.Unmarshal(out, &lv); err != nil {
		return nil, fmt.Errorf("parse list: %w", err)
	}
	issues := make([]Issue, 0, len(lv.Items))
	for _, v := range lv.Items {
		issues = append(issues, v.issue())
	}
	return issues, nil
}

// BlockedBy lists the item's existing blocked-by edge targets (Carina ids).
func (c *Carina) BlockedBy(issue int) ([]int, error) {
	v, err := c.show(issue)
	if err != nil {
		return nil, err
	}
	nums := make([]int, 0, len(v.BlockedBy))
	for _, b := range v.BlockedBy {
		n, err := strconv.Atoi(strings.TrimSpace(b))
		if err != nil {
			continue // a cross-repo reference form (owner/repo#id) is not a local edge
		}
		nums = append(nums, n)
	}
	return nums, nil
}

// Comments lists the item's comment bodies, in order.
func (c *Carina) Comments(issue int) ([]string, error) {
	v, err := c.show(issue)
	if err != nil {
		return nil, err
	}
	var bodies []string
	for _, e := range v.Log {
		if e.Kind == "comment" {
			bodies = append(bodies, e.Payload)
		}
	}
	return bodies, nil
}

func (c *Carina) show(issue int) (carinaItemView, error) {
	out, err := c.run("show", "--json", strconv.Itoa(issue))
	if err != nil {
		return carinaItemView{}, err
	}
	var v carinaItemView
	if err := json.Unmarshal(out, &v); err != nil {
		return carinaItemView{}, fmt.Errorf("parse item: %w", err)
	}
	return v, nil
}

// EditIssue rewrites the item's title and body.
func (c *Carina) EditIssue(issue int, title, body string) error {
	_, err := c.run("edit", strconv.Itoa(issue), "--title", title, "--body", body)
	return err
}

// ReplaceComment is unsupported: a Carina item's log is append-only, so a
// plan changed in place cannot rewrite a projected spec here.
func (c *Carina) ReplaceComment(int, string, string) error {
	return ErrInPlaceUnsupported
}
