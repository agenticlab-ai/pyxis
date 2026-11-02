package routingevidence

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
)

const ReviewProjectionSchema = "routing-evidence-review-projection/v2"

// ReviewProjection is the bounded identity-only face embedded by proposal and
// report adapters. It contains no candidates, finding prose, prompts, code, or
// model output; those media exact-read canonical events by these identities.
type ReviewProjection struct {
	Schema              string     `json:"schema"`
	Canonical           Projection `json:"canonical"`
	ReviewProcessID     string     `json:"review_process_id"`
	ReviewedRevision    string     `json:"reviewed_revision"`
	EvidenceRef         string     `json:"evidence_ref"`
	EvidenceCommit      string     `json:"evidence_commit"`
	PassEventIDs        []EventID  `json:"pass_event_ids"`
	FindingEventIDs     []EventID  `json:"finding_event_ids"`
	DispositionEventIDs []EventID  `json:"disposition_event_ids"`
	ProcessEventID      EventID    `json:"process_event_id,omitempty"`
}

func ProjectReview(exact ExactRead, processID string) (ReviewProjection, error) {
	if processID == "" || exact.ObservedHead.EvidenceRef == "" || exact.ObservedHead.Commit == "" {
		return ReviewProjection{}, fmt.Errorf("review projection: process and exact evidence head are required")
	}
	views, err := viewsFromExact(exact, SubjectID{Kind: SubjectProcess, ID: processID})
	if err != nil {
		return ReviewProjection{}, err
	}
	canonical, err := Project(views, MediumProposal)
	if err != nil {
		return ReviewProjection{}, err
	}
	projection := ReviewProjection{Schema: ReviewProjectionSchema, Canonical: canonical, ReviewProcessID: processID,
		EvidenceRef: exact.ObservedHead.EvidenceRef, EvidenceCommit: exact.ObservedHead.Commit,
		PassEventIDs: []EventID{}, FindingEventIDs: []EventID{}, DispositionEventIDs: []EventID{}}
	findings := map[string]bool{}
	for _, event := range exact.Events {
		if event.Kind != AssurancePassCompleted || event.Payload.AssurancePassCompleted.ReviewProcessID != processID {
			continue
		}
		p := event.Payload.AssurancePassCompleted
		if projection.ReviewedRevision == "" {
			projection.ReviewedRevision = p.ReviewedRevision
		} else if projection.ReviewedRevision != p.ReviewedRevision {
			return ReviewProjection{}, fmt.Errorf("review projection: pass revisions conflict")
		}
		projection.PassEventIDs = append(projection.PassEventIDs, event.EventID)
		for _, finding := range p.FindingRecords {
			findings[finding.FindingID] = true
		}
	}
	for _, event := range exact.Events {
		switch event.Kind {
		case FindingPersisted:
			if event.Payload.FindingPersisted.ReviewProcessID == processID {
				projection.FindingEventIDs = append(projection.FindingEventIDs, event.EventID)
			}
		case DispositionRecorded:
			if findings[event.Payload.DispositionRecorded.FindingID] {
				projection.DispositionEventIDs = append(projection.DispositionEventIDs, event.EventID)
			}
		case ReviewProcessCompleted:
			p := event.Payload.ReviewProcessCompleted
			if p.ReviewProcessID == processID {
				if projection.ProcessEventID != "" {
					return ReviewProjection{}, fmt.Errorf("review projection: duplicate process terminal")
				}
				projection.ProcessEventID = event.EventID
				if projection.ReviewedRevision == "" {
					projection.ReviewedRevision = p.ReviewedRevision
				} else if projection.ReviewedRevision != p.ReviewedRevision {
					return ReviewProjection{}, fmt.Errorf("review projection: terminal revision differs")
				}
			}
		}
	}
	if projection.ReviewedRevision == "" || len(projection.PassEventIDs) == 0 {
		return ReviewProjection{}, fmt.Errorf("review projection: canonical process is absent")
	}
	sortEventIDs(projection.PassEventIDs)
	sortEventIDs(projection.FindingEventIDs)
	sortEventIDs(projection.DispositionEventIDs)
	return projection, nil
}

func VerifyReviewProjection(projection ReviewProjection, exact ExactRead) error {
	if projection.Schema != ReviewProjectionSchema {
		return fmt.Errorf("review projection: unsupported schema %q", projection.Schema)
	}
	want, err := ProjectReview(exact, projection.ReviewProcessID)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(projection, want) {
		return fmt.Errorf("review projection: identity set does not match canonical evidence")
	}
	views, err := viewsFromExact(exact, SubjectID{Kind: SubjectProcess, ID: projection.ReviewProcessID})
	if err != nil {
		return err
	}
	return VerifyProjection(projection.Canonical, views)
}

func EncodeReviewProjection(projection ReviewProjection) ([]byte, error) {
	if err := validateReviewProjectionShape(projection); err != nil {
		return nil, err
	}
	return canonicalJSON(projection)
}

func DecodeReviewProjection(data []byte) (ReviewProjection, error) {
	var projection ReviewProjection
	if err := decodeClosed(data, &projection); err != nil {
		return ReviewProjection{}, fmt.Errorf("review projection: %w", err)
	}
	canonical, err := canonicalJSON(projection)
	if err != nil || !bytes.Equal(data, canonical) {
		return ReviewProjection{}, fmt.Errorf("review projection: non-canonical input")
	}
	if err := validateReviewProjectionShape(projection); err != nil {
		return ReviewProjection{}, err
	}
	return projection, nil
}

func validateReviewProjectionShape(projection ReviewProjection) error {
	if projection.Schema != ReviewProjectionSchema || projection.Canonical.Schema != ProjectionSchema || projection.ReviewProcessID == "" || projection.ReviewedRevision == "" ||
		projection.EvidenceRef == "" || projection.EvidenceCommit == "" || len(projection.PassEventIDs) == 0 ||
		projection.FindingEventIDs == nil || projection.DispositionEventIDs == nil {
		return fmt.Errorf("review projection: incomplete identity set")
	}
	sets := []struct {
		name string
		ids  []EventID
	}{{"pass", projection.PassEventIDs}, {"finding", projection.FindingEventIDs}, {"disposition", projection.DispositionEventIDs}}
	for _, set := range sets {
		name, ids := set.name, set.ids
		for i, id := range ids {
			if _, err := EventPath(id); err != nil || (i > 0 && ids[i-1] >= id) {
				return fmt.Errorf("review projection: %s event identities are invalid or unordered", name)
			}
		}
	}
	if projection.ProcessEventID != "" {
		if _, err := EventPath(projection.ProcessEventID); err != nil {
			return fmt.Errorf("review projection: process event identity is invalid")
		}
	}
	return nil
}

func sortEventIDs(ids []EventID) {
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
}
