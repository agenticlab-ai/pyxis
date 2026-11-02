package codehost

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
)

const TaskLandingSchema = "local-task-landing/v1"

// TaskLanding embeds the identical complete origin proof in the finalized
// proposal. Its prepared predecessor remains reachable through proposal lineage.
type TaskLanding struct {
	SchemaVersion string         `json:"schema_version"`
	Feature       string         `json:"feature"`
	Task          string         `json:"task"`
	MergeCommit   string         `json:"merge_commit"`
	Aggregate     ReviewArtifact `json:"aggregate"`
	Request       ReviewArtifact `json:"request"`
	Review        ReceiptRecord  `json:"review"`
}

func (l TaskLanding) Validate() error {
	b := l.Review.Receipt.Binding
	if l.SchemaVersion != TaskLandingSchema || l.Feature == "" || l.Task == "" || strings.ContainsAny(l.Feature+l.Task, "\r\n\x00") ||
		b.Validate() != nil || b.Kind != "implementation" || b.Operation != TaskMerge || !lowerHex(l.MergeCommit, len(b.Head)) ||
		l.Review.Receipt.SchemaVersion != ReceiptSchema || !lowerHex(l.Review.OID, len(b.Head)) ||
		l.Aggregate.Kind != TaskAggregate || l.Aggregate.Identity != b.Evidence || l.Request.Kind != TaskRequest || l.Request.Identity != b.Request {
		return ErrReviewInvalid
	}
	for _, artifact := range []ReviewArtifact{l.Aggregate, l.Request} {
		if len(artifact.Body) == 0 || len(artifact.Body) > MaxReviewArtifactBytes || artifact.Identity.Digest != fmt.Sprintf("sha256:%x", sha256.Sum256(artifact.Body)) {
			return ErrReviewInvalid
		}
	}
	request, err := ParseTaskRequest(l.Request.Body)
	if err != nil {
		return err
	}
	request.Binding.Request = l.Request.Identity
	if request.Binding != b {
		return ErrReviewInvalid
	}
	body, err := json.Marshal(l.Review.Receipt)
	if err != nil {
		return err
	}
	if l.Review.Digest != fmt.Sprintf("sha256:%x", sha256.Sum256(append(body, '\n'))) {
		return ErrReviewInvalid
	}
	return nil
}

type TaskLandingResult struct {
	Landing     TaskLanding        `json:"landing"`
	Prepared    ProposalGeneration `json:"prepared"`
	FeatureHead string             `json:"feature_head"`
	Created     bool               `json:"created"`
}

// The drive receives only the reader. The operator route receives LandTask
// after rechecking the complete current review and code-gate policy.
type TaskLandingReader interface {
	VerifyTaskLanding(context.Context, string) (TaskLandingResult, error)
}

type TaskLandings interface {
	TaskLandingReader
	LandTask(context.Context, ReviewBinding) (TaskLandingResult, error)
}
