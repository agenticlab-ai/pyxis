package lock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/draft/approval"
)

func TestLoadContractRefusesMissingTagDocumentsAndDigestMismatch(t *testing.T) {
	work, request, host := preparedMerge(t)
	reader := &recordApproval{result: approval.Approval{Operator: "mdarabi", Head: request.ApprovedHead, ReviewID: 27}}
	if _, err := RecordMerge(t.Context(), work, request, reader, host.run); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadContract(t.Context(), work, "", "widget", 1, host.run); err == nil {
		t.Fatal("empty remote accepted")
	}
	if _, err := LoadContract(t.Context(), work, "origin", "widget", 1, nil); err == nil {
		t.Fatal("missing adapter accepted")
	}
	failFetch := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
		if name == "git" && args[0] == "fetch" {
			return nil, errors.New("remote unavailable")
		}
		return host.run(ctx, dir, env, name, args...)
	}
	if _, err := LoadContract(t.Context(), work, "origin", "widget", 1, failFetch); err == nil || !strings.Contains(err.Error(), "fetch binding lock") {
		t.Fatalf("fetch error=%v", err)
	}
	badDigest := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
		out, err := host.run(ctx, dir, env, name, args...)
		if err != nil || name != "git" || args[0] != "cat-file" {
			return out, err
		}
		header, body, ok := strings.Cut(string(out), "\n\n")
		if !ok {
			return nil, fmt.Errorf("invalid fixture tag")
		}
		var r LockRecord
		if err := json.Unmarshal([]byte(body), &r); err != nil {
			return nil, err
		}
		r.ReadinessDigest = "sha256:" + strings.Repeat("f", 64)
		changed, err := json.Marshal(r)
		if err != nil {
			return nil, err
		}
		return []byte(header + "\n\n" + string(changed)), nil
	}
	if _, err := LoadContract(t.Context(), work, "origin", "widget", 1, badDigest); err == nil || !strings.Contains(err.Error(), "digest differs") {
		t.Fatalf("digest error=%v", err)
	}
	base := gitFixture(t, work, "rev-parse", request.MergeCommit+"^")
	empty := LockRecord{Schema: lockSchema, Feature: "empty", Version: 1, MergeCommit: base, PullRequest: 17, ApprovedHead: request.ApprovedHead, ReadinessDigest: request.ReadinessDigest, ProductApprover: "mdarabi", EngineeringApprover: "mdarabi", Lineless: true, ReviewID: 27, ProposalsSinceOpened: 1}
	body, err := json.Marshal(empty)
	if err != nil {
		t.Fatal(err)
	}
	gitFixture(t, work, "tag", "-a", "lock/empty/v1", base, "-m", string(body))
	gitFixture(t, work, "push", "origin", "refs/tags/lock/empty/v1")
	if _, err := LoadContract(t.Context(), work, "origin", "empty", 1, host.run); err == nil || !strings.Contains(err.Error(), "lacks PRD") {
		t.Fatalf("empty memory error=%v", err)
	}
	badRead := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
		if name == "git" && args[0] == "cat-file" {
			return nil, errors.New("bad tag object")
		}
		return host.run(ctx, dir, env, name, args...)
	}
	if _, err := LoadContract(t.Context(), work, "origin", "widget", 1, badRead); err == nil || !strings.Contains(err.Error(), "read lock tag") {
		t.Fatalf("tag error=%v", err)
	}
	gitFixture(t, work, "switch", "-c", "corrupt-memory", request.MergeCommit)
	badPath := filepath.Join(work, "memory/prd/corrupt/Need-00000000000000000000000000.json")
	if err := os.MkdirAll(filepath.Dir(badPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(badPath, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitFixture(t, work, "add", "--", "memory/prd/corrupt/Need-00000000000000000000000000.json")
	gitFixture(t, work, "commit", "-m", "corrupt memory fixture")
	corruptCommit := gitFixture(t, work, "rev-parse", "HEAD")
	corrupt := empty
	corrupt.Feature, corrupt.MergeCommit = "corrupt", corruptCommit
	body, err = json.Marshal(corrupt)
	if err != nil {
		t.Fatal(err)
	}
	gitFixture(t, work, "tag", "-a", "lock/corrupt/v1", corruptCommit, "-m", string(body))
	gitFixture(t, work, "push", "origin", "refs/tags/lock/corrupt/v1")
	if _, err := LoadContract(t.Context(), work, "origin", "corrupt", 1, host.run); err == nil || !strings.Contains(err.Error(), "invalid memory") {
		t.Fatalf("corrupt memory error=%v", err)
	}
}

func TestLoadContractRefusesUnusableRepositoryAfterTagCheck(t *testing.T) {
	sha := strings.Repeat("a", 40)
	record := LockRecord{Schema: lockSchema, Feature: "widget", Version: 1, MergeCommit: sha, PullRequest: 17, ApprovedHead: strings.Repeat("b", 40), ReadinessDigest: "sha256:" + strings.Repeat("c", 64), ProductApprover: "operator", EngineeringApprover: "operator", Lineless: true, ReviewID: 27, ProposalsSinceOpened: 1}
	body, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	run := func(_ context.Context, _ string, _ []string, name string, args ...string) ([]byte, error) {
		if name != "git" {
			return nil, errors.New("unexpected command")
		}
		switch args[0] {
		case "fetch":
			return nil, nil
		case "cat-file":
			return []byte("object " + sha + "\ntype commit\ntag lock/widget/v1\ntagger operator <o@x> 1 +0000\n\n" + string(body)), nil
		case "rev-parse":
			return []byte(sha), nil
		default:
			return nil, errors.New("unexpected git command")
		}
	}
	if _, err := LoadContract(t.Context(), t.TempDir(), "origin", "widget", 1, run); err == nil || !strings.Contains(err.Error(), "open locked repository") {
		t.Fatalf("repo error=%v", err)
	}
}
