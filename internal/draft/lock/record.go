package lock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/draft/approval"
	"github.com/agenticlab-ai/pyxis/internal/draft/lifecycle"
)

const lockSchema = "draft-lock/v1"
const driverLockSchema = "draft-lock/v2"

var fullSHA = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)
var draftDigest = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// LockRecord is an annotated tag's closed, durable lock event.
type LockRecord struct {
	Schema               string `json:"schema"`
	Feature              string `json:"feature"`
	Version              int    `json:"version"`
	MergeCommit          string `json:"merge_commit"`
	PullRequest          int    `json:"pull_request"`
	ApprovedHead         string `json:"approved_head"`
	ReadinessDigest      string `json:"readiness_digest"`
	ProductApprover      string `json:"product_approver"`
	EngineeringApprover  string `json:"engineering_approver"`
	CaptureLine          string `json:"capture_line"`
	Lineless             bool   `json:"lineless"`
	ReviewID             int64  `json:"review_id"`
	ProposalsSinceOpened int    `json:"proposals_since_opened"`
	ApprovalKind         string `json:"approval_kind,omitempty"`
	PolicyCommit         string `json:"policy_commit,omitempty"`
	ApprovalEvidence     string `json:"approval_evidence,omitempty"`
}

type RecordRequest struct {
	Feature              string
	Remote               string
	ExpectedVersion      int
	MergeCommit          string
	PullRequest          int
	ApprovedHead         string
	ReadinessDigest      string
	ProposalsSinceOpened int
}

type RecordOutcome struct {
	Lock    *LockRecord // nil for a non-lock merge
	Created bool
}

// RecordMerge creates the immutable tag when the merge adds the first version's
// two Documents, or at least one changed Document for a later version. The
// approval port must certify the PR's exact head.
// The returned record is identical on a repeated run; conflicting tags fail.
func RecordMerge(ctx context.Context, root string, request RecordRequest, reader approval.Port, run lifecycle.RunCommand) (RecordOutcome, error) {
	if run == nil || !featureName.MatchString(request.Feature) || request.Remote == "" || !fullSHA.MatchString(request.MergeCommit) {
		return RecordOutcome{}, fmt.Errorf("record lock: process adapter, feature, remote, and full merge commit are required")
	}
	changed, err := command(ctx, run, root, nil, "git", "diff-tree", "--no-commit-id", "--name-status", "-r", request.MergeCommit+"^", request.MergeCommit)
	if err != nil {
		return RecordOutcome{}, fmt.Errorf("inspect lock merge: %w", err)
	}
	prd, design := false, false
	for line := range strings.SplitSeq(changed, "\n") {
		status, path, ok := strings.Cut(line, "\t")
		if !ok || status != "A" {
			continue
		}
		if strings.HasPrefix(path, "memory/prd/"+request.Feature+"/Document-") && strings.HasSuffix(path, ".json") {
			prd = true
		}
		if strings.HasPrefix(path, "memory/design/"+request.Feature+"/Document-") && strings.HasSuffix(path, ".json") {
			design = true
		}
	}
	if !prd && !design {
		return RecordOutcome{}, nil
	}
	if request.ExpectedVersion == 1 && (!prd || !design) {
		return RecordOutcome{}, fmt.Errorf("lock merge has only one of the two contract Documents")
	}
	if reader == nil || request.ExpectedVersion < 1 || request.PullRequest < 1 || !fullSHA.MatchString(request.ApprovedHead) || !draftDigest.MatchString(request.ReadinessDigest) || request.ProposalsSinceOpened < 1 {
		return RecordOutcome{}, fmt.Errorf("record lock: approval reader, pull request, approved head, digest, and proposal count are required")
	}
	var approved approval.Approval
	var policyBase string
	if bound, ok := reader.(approval.BoundPort); ok {
		base, baseErr := command(ctx, run, root, nil, "git", "rev-parse", request.MergeCommit+"^")
		if baseErr != nil {
			return RecordOutcome{}, fmt.Errorf("resolve pre-merge approval policy: %w", baseErr)
		}
		if !fullSHA.MatchString(base) {
			return RecordOutcome{}, fmt.Errorf("pre-merge approval policy has no full commit")
		}
		policyBase = base
		approved, err = bound.ApprovalFor(ctx, request.PullRequest, request.ApprovedHead, base, request.ReadinessDigest)
	} else {
		approved, err = reader.Approval(ctx, request.PullRequest, request.ApprovedHead)
	}
	if err != nil {
		if errors.Is(err, approval.ErrNoApproval) {
			return RecordOutcome{}, fmt.Errorf("lock %s at head %s has no authorized approval of the exact head; request operator approval or a granted Argus approval of this head and retry: %w", request.Feature, request.ApprovedHead, err)
		}
		return RecordOutcome{}, fmt.Errorf("read exact-head lock approval: %w", err)
	}
	if approved.Head != request.ApprovedHead || approved.Operator == "" || approved.ReviewID < 1 {
		return RecordOutcome{}, fmt.Errorf("approval port returned an incomplete or mismatched review")
	}
	if approved.Kind != "" && (approved.Kind != "argus" || !fullSHA.MatchString(approved.PolicyCommit) || approved.PolicyCommit != policyBase || strings.TrimSpace(approved.Evidence) == "") {
		return RecordOutcome{}, fmt.Errorf("approval port returned invalid delegated approval evidence")
	}
	remote, err := command(ctx, run, root, nil, "git", "ls-remote", "--tags", request.Remote, "refs/tags/lock/"+request.Feature+"/v*")
	if err != nil {
		return RecordOutcome{}, fmt.Errorf("list remote lock tags: %w", err)
	}
	tags := map[int]string{}
	maxVersion := 0
	for line := range strings.SplitSeq(remote, "\n") {
		_, ref, ok := strings.Cut(line, "\t")
		if !ok || strings.HasSuffix(ref, "^{}") {
			continue
		}
		prefix := "refs/tags/lock/" + request.Feature + "/v"
		raw, ok := strings.CutPrefix(ref, prefix)
		if !ok {
			continue
		}
		version, err := strconv.Atoi(raw)
		if err != nil || version < 1 {
			return RecordOutcome{}, fmt.Errorf("invalid remote lock tag %s", ref)
		}
		if _, exists := tags[version]; exists {
			return RecordOutcome{}, fmt.Errorf("duplicate remote lock version %d", version)
		}
		tags[version] = ref
		maxVersion = max(maxVersion, version)
	}
	for version := 1; version <= maxVersion; version++ {
		ref := tags[version]
		if ref == "" {
			return RecordOutcome{}, fmt.Errorf("lock tags for %s skip version %d", request.Feature, version)
		}
		if _, err := run(ctx, root, nil, "git", "fetch", request.Remote, ref+":"+ref); err != nil {
			return RecordOutcome{}, fmt.Errorf("fetch lock tag %s: %w", ref, err)
		}
		past, err := ReadLock(ctx, root, request.Feature, version, run)
		if err != nil {
			return RecordOutcome{}, err
		}
		if past.MergeCommit != request.MergeCommit {
			continue
		}
		if version != request.ExpectedVersion {
			return RecordOutcome{}, fmt.Errorf("merged draft expected version %d but existing lock is version %d", request.ExpectedVersion, version)
		}
		expected := lockRecord(request, version, approved)
		if *past != expected {
			return RecordOutcome{}, fmt.Errorf("existing lock tag %s differs from this approval", ref)
		}
		return RecordOutcome{Lock: past}, nil
	}
	if maxVersion+1 != request.ExpectedVersion {
		return RecordOutcome{}, fmt.Errorf("merged draft expected version %d but next lock version is %d", request.ExpectedVersion, maxVersion+1)
	}
	record := lockRecord(request, maxVersion+1, approved)
	body, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return RecordOutcome{}, fmt.Errorf("encode lock record: %w", err)
	}
	tag := fmt.Sprintf("lock/%s/v%d", request.Feature, record.Version)
	local, err := command(ctx, run, root, nil, "git", "tag", "--list", tag)
	if err != nil {
		return RecordOutcome{}, fmt.Errorf("inspect local lock tag %s: %w", tag, err)
	}
	if local != "" {
		past, err := ReadLock(ctx, root, request.Feature, record.Version, run)
		if err != nil {
			return RecordOutcome{}, fmt.Errorf("read local lock tag %s: %w", tag, err)
		}
		if *past != record {
			return RecordOutcome{}, fmt.Errorf("local lock tag %s conflicts with this approval", tag)
		}
	} else if _, err := run(ctx, root, nil, "git", "-c", "user.name=Pyxis Lock Recorder", "-c", "user.email=lock@pyxis.invalid", "tag", "-a", tag, request.MergeCommit, "-m", string(body)); err != nil {
		return RecordOutcome{}, fmt.Errorf("create lock tag %s: %w", tag, err)
	}
	if _, err := run(ctx, root, nil, "git", "push", request.Remote, "refs/tags/"+tag); err != nil {
		return RecordOutcome{}, fmt.Errorf("publish lock tag %s: %w", tag, err)
	}
	return RecordOutcome{Lock: &record, Created: true}, nil
}

func lockRecord(request RecordRequest, version int, approved approval.Approval) LockRecord {
	record := LockRecord{
		Schema: lockSchema, Feature: request.Feature, Version: version,
		MergeCommit: request.MergeCommit, PullRequest: request.PullRequest,
		ApprovedHead: request.ApprovedHead, ReadinessDigest: request.ReadinessDigest,
		ProductApprover: approved.Operator, EngineeringApprover: approved.Operator,
		CaptureLine: approved.CaptureLine, Lineless: approved.CaptureLine == "",
		ReviewID: approved.ReviewID, ProposalsSinceOpened: request.ProposalsSinceOpened,
	}
	if approved.Kind == "argus" {
		record.Schema = driverLockSchema
		record.ApprovalKind = approved.Kind
		record.PolicyCommit = approved.PolicyCommit
		record.ApprovalEvidence = approved.Evidence
	}
	return record
}

// ReadLock validates an annotated lock tag from a local or fresh clone.
func ReadLock(ctx context.Context, root, feature string, version int, run lifecycle.RunCommand) (*LockRecord, error) {
	if run == nil || !featureName.MatchString(feature) || version < 1 {
		return nil, fmt.Errorf("read lock: process adapter, feature, and version are required")
	}
	tag := fmt.Sprintf("lock/%s/v%d", feature, version)
	contents, err := command(ctx, run, root, nil, "git", "cat-file", "-p", "refs/tags/"+tag)
	if err != nil {
		return nil, fmt.Errorf("read lock tag %s: %w", tag, err)
	}
	header, body, ok := strings.Cut(contents, "\n\n")
	if !ok || !strings.Contains(header, "\ntype commit\n") || !strings.Contains(header, "\ntag "+tag+"\n") {
		return nil, fmt.Errorf("lock tag %s is not an annotated commit tag", tag)
	}
	var record LockRecord
	dec := json.NewDecoder(strings.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&record); err != nil {
		return nil, fmt.Errorf("decode lock tag %s: %w", tag, err)
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("lock tag %s has trailing content", tag)
	}
	commit, err := command(ctx, run, root, nil, "git", "rev-parse", "refs/tags/"+tag+"^{commit}")
	if err != nil {
		return nil, fmt.Errorf("resolve lock tag %s: %w", tag, err)
	}
	validApproval := record.Schema == lockSchema && record.ApprovalKind == "" && record.PolicyCommit == "" && record.ApprovalEvidence == "" ||
		record.Schema == driverLockSchema && record.ApprovalKind == "argus" && fullSHA.MatchString(record.PolicyCommit) && strings.TrimSpace(record.ApprovalEvidence) != ""
	if !validApproval || record.Feature != feature || record.Version != version || record.MergeCommit != commit || !fullSHA.MatchString(record.ApprovedHead) || !draftDigest.MatchString(record.ReadinessDigest) || record.PullRequest < 1 || record.ProposalsSinceOpened < 1 || record.ProductApprover == "" || record.EngineeringApprover != record.ProductApprover || record.ReviewID < 1 || record.Lineless != (record.CaptureLine == "") {
		return nil, fmt.Errorf("lock tag %s has invalid or mismatched fields", tag)
	}
	if record.Schema == driverLockSchema {
		parent, err := command(ctx, run, root, nil, "git", "rev-parse", record.MergeCommit+"^")
		if err != nil {
			return nil, fmt.Errorf("resolve pre-merge policy for lock tag %s: %w", tag, err)
		}
		if record.PolicyCommit != parent {
			return nil, fmt.Errorf("lock tag %s policy commit does not match pre-merge parent", tag)
		}
	}
	return &record, nil
}
