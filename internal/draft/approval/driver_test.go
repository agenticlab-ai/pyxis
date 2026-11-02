package approval

import (
	"errors"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

func driverFixture(t *testing.T) (DriverDecision, Review) {
	t.Helper()
	d := DriverDecision{Schema: DriverSchema, Principal: "argus", Repository: "example/repo", PullRequest: 17,
		Head: strings.Repeat("a", 40), PolicyCommit: strings.Repeat("b", 40), Drive: "widget",
		Digest: "sha256:" + strings.Repeat("c", 64), Evidence: "Independent PRD and design reviews approve this digest; reports 003 and 004.", CaptureLine: "Reviewed exact contract"}
	body, err := d.Body()
	if err != nil {
		t.Fatal(err)
	}
	return d, Review{ID: 27, Author: "driver", State: "COMMENTED", CommitID: d.Head, Body: body}
}

func TestDriverApprovalRequiresHostIdentityGrantAndWholeBinding(t *testing.T) {
	d, review := driverFixture(t)
	policy := config.ApprovalSettings{OperatorLogin: "operator", ArgusLogin: "driver"}
	got, err := SelectDriver(policy, d.Repository, d.PullRequest, d.Head, d.PolicyCommit, d.Digest, true, []Review{review})
	if err != nil || got.Kind != "argus" || got.Operator != "driver" || got.ReviewID != 27 || got.PolicyCommit != d.PolicyCommit || got.Evidence != d.Evidence || got.CaptureLine != d.CaptureLine {
		t.Fatalf("approval=%+v error=%v", got, err)
	}
	for _, change := range []struct {
		name string
		edit func(*DriverDecision, *Review)
	}{
		{"different repository", func(d *DriverDecision, _ *Review) { d.Repository = "other/repo" }},
		{"different proposal", func(d *DriverDecision, _ *Review) { d.PullRequest++ }},
		{"old head", func(d *DriverDecision, _ *Review) { d.Head = strings.Repeat("d", 40) }},
		{"old mainline policy", func(d *DriverDecision, _ *Review) { d.PolicyCommit = strings.Repeat("d", 40) }},
		{"old contract", func(d *DriverDecision, _ *Review) { d.Digest = "sha256:" + strings.Repeat("d", 64) }},
		{"forged account", func(_ *DriverDecision, r *Review) { r.Author = "impostor" }},
		{"reviewed other commit", func(_ *DriverDecision, r *Review) { r.CommitID = strings.Repeat("d", 40) }},
		{"dismissed", func(_ *DriverDecision, r *Review) { r.State = "DISMISSED" }},
		{"unsubmitted", func(_ *DriverDecision, r *Review) { r.State = "PENDING" }},
	} {
		t.Run(change.name, func(t *testing.T) {
			changed, submitted := d, review
			change.edit(&changed, &submitted)
			var err error
			submitted.Body, err = changed.Body()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := SelectDriver(policy, d.Repository, d.PullRequest, d.Head, d.PolicyCommit, d.Digest, true, []Review{submitted}); !errors.Is(err, ErrNoApproval) {
				t.Fatalf("changed decision accepted: %v", err)
			}
		})
	}
	for _, configured := range []config.ApprovalSettings{{OperatorLogin: "operator"}, {OperatorLogin: "driver", ArgusLogin: "driver"}, {ArgusLogin: "driver"}} {
		if _, err := SelectDriver(configured, d.Repository, d.PullRequest, d.Head, d.PolicyCommit, d.Digest, true, []Review{review}); !errors.Is(err, ErrNoApproval) {
			t.Fatal("missing or shared operator identity granted driver approval")
		}
	}
	if _, err := SelectDriver(policy, d.Repository, d.PullRequest, d.Head, d.PolicyCommit, d.Digest, false, []Review{review}); !errors.Is(err, ErrNoApproval) {
		t.Fatal("ungranted driver approval accepted")
	}
}

func TestLaterDriverReviewRevokesEarlierApproval(t *testing.T) {
	d, review := driverFixture(t)
	policy := config.ApprovalSettings{OperatorLogin: "operator", ArgusLogin: "driver"}
	for _, state := range []string{"CHANGES_REQUESTED", "DISMISSED", "APPROVED"} {
		later := review
		later.ID++
		later.State, later.Body = state, "An ordinary review has no typed driver decision"
		if _, err := SelectDriver(policy, d.Repository, d.PullRequest, d.Head, d.PolicyCommit, d.Digest, true, []Review{review, later}); !errors.Is(err, ErrNoApproval) {
			t.Fatalf("later %s did not revoke the decision", state)
		}
	}
	later := review
	later.ID++
	later.Body = "An ordinary discussion comment"
	if _, err := SelectDriver(policy, d.Repository, d.PullRequest, d.Head, d.PolicyCommit, d.Digest, true, []Review{review, later}); err != nil {
		t.Fatalf("discussion replaced submitted decision: %v", err)
	}
}

func TestDriverDecisionClosedCanonicalProtocol(t *testing.T) {
	d, review := driverFixture(t)
	parsed, err := ParseDriver(review.Body)
	if err != nil || parsed != d {
		t.Fatalf("round trip=%+v %v", parsed, err)
	}
	for _, body := range []string{
		"Argus approves", review.Body + "\n" + review.Body, review.Body + " trailing",
		strings.Replace(review.Body, `"principal": "argus",`, `"principal": "argus", "extra": true,`, 1),
		strings.Replace(review.Body, `"principal": "argus",`, `"principal": "operator", "principal": "argus",`, 1),
		strings.Replace(review.Body, `"principal": "argus"`, `"principal": "operator"`, 1),
	} {
		if _, err := ParseDriver(body); err == nil {
			t.Fatalf("malformed decision accepted: %q", body)
		}
	}
}

func TestDriverDecisionRejectsOversizedEncodedEvidence(t *testing.T) {
	d, _ := driverFixture(t)
	d.Evidence = strings.Repeat("\x01", 6000)
	if err := d.Validate(); err != nil {
		t.Fatalf("raw evidence is within its field bound: %v", err)
	}
	if _, err := d.Body(); err == nil {
		t.Fatal("writer emitted escaped evidence that its reader cannot accept")
	}
	d.Evidence = strings.Repeat("\x01", 4096)
	body, err := d.Body()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseDriver(body)
	if err != nil || parsed != d {
		t.Fatalf("bounded escaped evidence did not round trip: %v", err)
	}
}

func TestDriverGrantUsesAuthorityHistoryAndRevocation(t *testing.T) {
	const body = `# Argus authority contract
| Field | Value |
|---|---|
| **Current tier** | tier-4 |
| **Template version** | authority-contract/v1 |
## The ladder
| Tier (ordered) | Bundle (action classes) |
|---|---|
| revoked | ∅ |
| floor | dispose-advisory-finding |
| tier-2 | + sign-gate-verdict |
| tier-3 | + rule-acceptance |
| tier-4 | + approve-merge |
## Change history
| Date | Move | Tier | Evidence |
|---|---|---|---|
| 2026-10-01 | none | floor | born |
| 2026-10-05 | promotion | tier-4 | operator-directed grant |
`
	if !DriverGranted([]byte(body)) {
		t.Fatal("recorded driver grant refused")
	}
	for _, changed := range []string{
		strings.Replace(body, "**Current tier** | tier-4", "**Current tier** | tier-2", 1),
		strings.Replace(strings.Replace(body, "**Current tier** | tier-4", "**Current tier** | tier-2", 1), "promotion | tier-4", "promotion | tier-2", 1),
		strings.Replace(body, "| 2026-10-01 | none | floor | born |\n", "", 1),
		strings.Replace(body, "authority-contract/v1", "authority-contract/v2", 1),
		strings.Replace(strings.Replace(body, "**Current tier** | tier-4", "**Current tier** | revoked", 1), "promotion | tier-4", "revocation | revoked", 1),
	} {
		if DriverGranted([]byte(changed)) {
			t.Fatal("ungranted, malformed, or revoked contract granted approval")
		}
	}
}
