package lock

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/draft/approval"
)

type boundApproval struct {
	result approval.Approval
	base   string
	digest string
}

func (r *boundApproval) Approval(context.Context, int, string) (approval.Approval, error) {
	return approval.Approval{}, approval.ErrNoApproval
}

func (r *boundApproval) ApprovalFor(_ context.Context, _ int, _ string, base, digest string) (approval.Approval, error) {
	r.base, r.digest = base, digest
	return r.result, nil
}

func TestDriverLockReadsPolicyBeforeMergeAndSurvivesFreshClone(t *testing.T) {
	work, request, host := preparedMerge(t)
	base := gitFixture(t, work, "rev-parse", request.MergeCommit+"^")
	reader := &boundApproval{result: approval.Approval{Operator: "driver", Head: request.ApprovedHead, ReviewID: 27,
		Kind: "argus", PolicyCommit: base, Evidence: "Independent current-digest reviews 003 and 004 approve."}}
	out, err := RecordMerge(t.Context(), work, request, reader, host.run)
	if err != nil || out.Lock == nil || out.Lock.Schema != driverLockSchema || out.Lock.ApprovalKind != "argus" || out.Lock.PolicyCommit != base || reader.base != base || reader.digest != request.ReadinessDigest {
		t.Fatalf("driver lock=%+v base=%s digest=%s error=%v", out.Lock, reader.base, reader.digest, err)
	}
	bare := filepath.Join(filepath.Dir(work), "remote.git")
	gitFixture(t, bare, "symbolic-ref", "HEAD", "refs/heads/main")
	fresh := filepath.Join(t.TempDir(), "fresh")
	gitFixture(t, filepath.Dir(fresh), "clone", "--no-local", bare, fresh)
	read, err := ReadLock(t.Context(), fresh, "widget", 1, host.run)
	if err != nil || *read != *out.Lock {
		t.Fatalf("fresh driver lock=%+v error=%v", read, err)
	}
	if _, err := LoadContract(t.Context(), fresh, "origin", "widget", 1, host.run); err != nil {
		t.Fatalf("fresh driver-approved contract unavailable: %v", err)
	}
}

func TestDriverLockRejectsPolicyFromTheCandidateMerge(t *testing.T) {
	work, request, host := preparedMerge(t)
	reader := &boundApproval{result: approval.Approval{Operator: "driver", Head: request.ApprovedHead, ReviewID: 27,
		Kind: "argus", PolicyCommit: request.MergeCommit, Evidence: "self-granted candidate approval"}}
	if _, err := RecordMerge(t.Context(), work, request, reader, host.run); err == nil {
		t.Fatal("candidate policy authorized the same merge")
	}
	if tags := gitFixture(t, work, "tag", "--list", "lock/widget/v*"); tags != "" {
		t.Fatalf("refused approval published a tag: %s", tags)
	}
}

func TestReadDriverLockRejectsNonParentPolicyInFreshClone(t *testing.T) {
	for _, policySource := range []string{"candidate", "later", "unrelated"} {
		t.Run(policySource, func(t *testing.T) {
			work, request, host := preparedMerge(t)
			policy := request.MergeCommit
			if policySource == "unrelated" {
				base := gitFixture(t, work, "rev-parse", request.MergeCommit+"^")
				gitFixture(t, work, "switch", "-c", "unrelated", base)
			}
			if policySource != "candidate" {
				gitFixture(t, work, "commit", "--allow-empty", "-m", policySource+" policy")
				policy = gitFixture(t, work, "rev-parse", "HEAD")
				gitFixture(t, work, "push", "origin", "HEAD")
			}
			record := lockRecord(request, 1, approval.Approval{
				Operator: "driver", Head: request.ApprovedHead, ReviewID: 27,
				Kind: "argus", PolicyCommit: policy, Evidence: "Reviewed the prepared contract.",
			})
			body, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			// Simulate a corrupted remote record without going through the
			// recorder, whose pre-merge policy check already refuses this.
			gitFixture(t, work, "tag", "-a", "lock/widget/v1", request.MergeCommit, "-m", string(body))
			gitFixture(t, work, "push", "origin", "refs/tags/lock/widget/v1")
			bare := filepath.Join(filepath.Dir(work), "remote.git")
			gitFixture(t, bare, "symbolic-ref", "HEAD", "refs/heads/main")
			fresh := filepath.Join(t.TempDir(), "fresh")
			gitFixture(t, filepath.Dir(fresh), "clone", "--no-local", bare, fresh)
			gitFixture(t, fresh, "cat-file", "-e", policy+"^{commit}")
			if record, err := ReadLock(t.Context(), fresh, "widget", 1, host.run); err == nil || !strings.Contains(err.Error(), "policy commit does not match pre-merge parent") {
				t.Fatalf("non-parent policy accepted: record=%+v error=%v", record, err)
			}
			if _, err := LoadContract(t.Context(), fresh, "origin", "widget", 1, host.run); err == nil || !strings.Contains(err.Error(), "policy commit does not match pre-merge parent") {
				t.Fatalf("contract loaded with non-parent policy: %v", err)
			}
		})
	}
}

func TestReadDriverLockRefusesUnavailableMergeParent(t *testing.T) {
	work, request, host := preparedMerge(t)
	base := gitFixture(t, work, "rev-parse", request.MergeCommit+"^")
	reader := &boundApproval{result: approval.Approval{Operator: "driver", Head: request.ApprovedHead, ReviewID: 27,
		Kind: "argus", PolicyCommit: base, Evidence: "Reviewed the prepared contract."}}
	if _, err := RecordMerge(t.Context(), work, request, reader, host.run); err != nil {
		t.Fatal(err)
	}
	bare := filepath.Join(filepath.Dir(work), "remote.git")
	gitFixture(t, bare, "symbolic-ref", "HEAD", "refs/heads/main")
	shallow := filepath.Join(t.TempDir(), "shallow")
	gitFixture(t, filepath.Dir(shallow), "clone", "--no-local", "--depth", "1", bare, shallow)
	if gitFixture(t, shallow, "rev-parse", "--is-shallow-repository") != "true" {
		t.Fatal("fixture is not shallow")
	}
	if _, err := ReadLock(t.Context(), shallow, "widget", 1, host.run); err == nil || !strings.Contains(err.Error(), "resolve pre-merge policy") {
		t.Fatalf("unavailable pre-merge policy accepted: %v", err)
	}
}
