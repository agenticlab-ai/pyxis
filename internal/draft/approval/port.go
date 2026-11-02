// Package approval defines version-bound authorized reviews read by a draft lock.
package approval

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var ErrNoApproval = errors.New("no authorized approval of the draft head")

// Approval is an authorized review of the exact head being locked. An empty
// capture line is a valid, explicitly lineless approval.
type Approval struct {
	Operator     string
	Head         string
	CaptureLine  string
	ReviewID     int64
	Kind         string // empty for the historical human review; "argus" for a delegated driver decision
	PolicyCommit string
	Evidence     string
}

// BoundPort verifies a delegated approval against the mainline that preceded
// the merge and the prepared contract digest. Candidate policy cannot grant it.
type BoundPort interface {
	ApprovalFor(context.Context, int, string, string, string) (Approval, error)
}

// Port reads the approval of a pull request's current head. A stale approval
// returns ErrNoApproval; transport and malformed-response failures remain errors.
type Port interface {
	Approval(ctx context.Context, pull int, head string) (Approval, error)
}

// Review is the small code-host observation needed to select an approval.
type Review struct {
	ID       int64  `json:"id"`
	Author   string `json:"author"`
	State    string `json:"state"`
	CommitID string `json:"commit_id"`
	Body     string `json:"body"`
}

// Select is the operator-only selector. Delegated driver decisions use
// SelectDriver with trusted mainline policy; neither selector accepts a stale
// head or a self-declared operator identity.
func Select(operator, head string, reviews []Review) (Approval, error) {
	if operator == "" || head == "" {
		return Approval{}, fmt.Errorf("select draft approval: operator and head are required")
	}
	var latest *Review
	for i := range reviews {
		r := &reviews[i]
		if !strings.EqualFold(r.Author, operator) || r.ID < 1 {
			continue
		}
		// Comments and unsubmitted drafts do not replace a submitted review.
		if strings.EqualFold(r.State, "COMMENTED") || strings.EqualFold(r.State, "PENDING") {
			continue
		}
		if latest == nil || r.ID > latest.ID {
			latest = r
		}
	}
	if latest == nil || !strings.EqualFold(latest.State, "APPROVED") || latest.CommitID != head {
		return Approval{}, ErrNoApproval
	}
	capture := latest.Body
	if strings.TrimSpace(capture) == "" {
		capture = ""
	}
	return Approval{Operator: latest.Author, Head: head, CaptureLine: capture, ReviewID: latest.ID}, nil
}
