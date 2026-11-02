package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/draft/approval"
)

type approvalHost struct {
	pull            pullBinding
	defaultBranch   string
	mainlineSHA     string
	candidateConfig string
	login           string
	config          string
	authority       string
	reviews         []map[string]any
	published       submittedReview
	event           string
	writes          int
	pullReads       int
	mutate          func(*approvalHost)
	digest          string
}

func approvalFixture(t *testing.T, author string) (*Reader, *approvalHost, approval.DriverDecision) {
	t.Helper()
	body, err := os.ReadFile("../../../../.pyxis/skills/argus/authority-contract.md")
	if err != nil {
		t.Fatal(err)
	}
	authority := strings.Replace(string(body), "**Current tier** | tier-2", "**Current tier** | tier-4", 1) +
		"| 2026-10-05 | promotion | tier-4 | operator-directed test grant |\n"
	h := &approvalHost{login: "driver", config: "approval:\n  operator_login: operator\n  argus_login: driver\n", authority: authority, digest: "sha256:" + strings.Repeat("c", 64)}
	h.defaultBranch = "main"
	h.pull.State = "open"
	h.pull.User.Login = author
	h.pull.Head.SHA, h.pull.Head.Ref = strings.Repeat("a", 40), "feat/widget"
	h.pull.Base.SHA, h.pull.Base.Ref = strings.Repeat("b", 40), "main"
	h.mainlineSHA = h.pull.Base.SHA
	r := &Reader{Repo: "example/repo", Operator: "operator"}
	r.run = h.read
	r.write = h.write
	d := approval.DriverDecision{Schema: approval.DriverSchema, Principal: "argus", Repository: r.Repo, PullRequest: 17,
		Head: h.pull.Head.SHA, Drive: "widget", Evidence: "Independent review of exact revision; report review:widget-001 approves."}
	return r, h, d
}

func (h *approvalHost) read(_ context.Context, args ...string) ([]byte, error) {
	path := args[len(args)-1]
	switch {
	case path == "repos/example/repo":
		return json.Marshal(map[string]string{"default_branch": h.defaultBranch})
	case strings.HasPrefix(path, "repos/example/repo/branches/"):
		return json.Marshal(map[string]any{"name": h.defaultBranch, "commit": map[string]string{"sha": h.mainlineSHA}})
	case path == "user":
		return json.Marshal(map[string]string{"login": h.login})
	case path == "repos/example/repo/pulls/17":
		h.pullReads++
		if h.mutate != nil {
			h.mutate(h)
		}
		return json.Marshal(h.pull)
	case strings.HasPrefix(path, "repos/example/repo/contents/"):
		if !strings.HasSuffix(path, "?ref="+h.mainlineSHA) {
			if h.candidateConfig != "" && strings.HasSuffix(path, "?ref="+h.pull.Base.SHA) {
				return json.Marshal(map[string]any{"type": "file", "encoding": "base64", "size": len(h.candidateConfig), "content": base64.StdEncoding.EncodeToString([]byte(h.candidateConfig))})
			}
			return nil, fmt.Errorf("policy was not read from mainline: %s", path)
		}
		body := h.config
		if strings.Contains(path, "authority-contract.md") {
			body = h.authority
		}
		return json.Marshal(map[string]any{"type": "file", "encoding": "base64", "size": len(body), "content": base64.StdEncoding.EncodeToString([]byte(body))})
	case strings.Contains(path, "/reviews?per_page=100&page="):
		if h.reviews == nil {
			return []byte("[]"), nil
		}
		return json.Marshal(h.reviews)
	case path == "repos/example/repo/pulls/17/reviews/"+strconv.FormatInt(h.published.ID, 10):
		return json.Marshal(h.published)
	case strings.HasPrefix(path, "repos/example/repo/commits/"):
		return json.Marshal(map[string]any{"commit": map[string]string{"message": "docs(draft): prepare lock widget\n\nDraft-Digest: " + h.digest}})
	default:
		return nil, fmt.Errorf("unexpected read %v", args)
	}
}

func (h *approvalHost) write(_ context.Context, body []byte, args ...string) ([]byte, error) {
	if strings.Join(args, " ") != "api --method POST repos/example/repo/pulls/17/reviews --input -" {
		return nil, fmt.Errorf("unexpected mutation %v", args)
	}
	var request struct {
		CommitID string `json:"commit_id"`
		Event    string `json:"event"`
		Body     string `json:"body"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		return nil, err
	}
	h.writes++
	h.event = request.Event
	if h.published.ID == 0 {
		h.published.ID = 27
	} else {
		h.published.ID++
	}
	h.published.User.Login = h.login
	h.published.State = "APPROVED"
	if request.Event == "COMMENT" {
		h.published.State = "COMMENTED"
	}
	h.published.CommitID, h.published.Body = request.CommitID, request.Body
	h.reviews = append(h.reviews, map[string]any{"id": h.published.ID, "user": map[string]string{"login": h.login}, "state": h.published.State, "commit_id": request.CommitID, "body": request.Body})
	return json.Marshal(h.published)
}

func TestPublishDriverApprovalHandlesAuthorAndDistinctReviewer(t *testing.T) {
	for _, tc := range []struct{ author, event string }{{"driver", "COMMENT"}, {"another-author", "APPROVE"}} {
		t.Run(tc.author, func(t *testing.T) {
			r, host, d := approvalFixture(t, tc.author)
			got, err := r.Approve(t.Context(), d)
			if err != nil || got.Kind != "argus" || got.Operator != "driver" || got.Head != d.Head || got.PolicyCommit != host.pull.Base.SHA || host.writes != 1 || host.event != tc.event {
				t.Fatalf("approval=%+v writes=%d event=%s err=%v", got, host.writes, host.event, err)
			}
			read, err := r.ApprovalFor(t.Context(), 17, d.Head, host.pull.Base.SHA, "")
			if err != nil || read != got {
				t.Fatalf("observed approval=%+v %v", read, err)
			}
		})
	}
}

func TestPublishDriverApprovalTargetsFeatureIntegrationBranch(t *testing.T) {
	r, host, d := approvalFixture(t, "driver")
	host.pull.Base.Ref = "feature/widget"
	host.pull.Base.SHA = strings.Repeat("e", 40)
	got, err := r.Approve(t.Context(), d)
	if err != nil || got.PolicyCommit != host.mainlineSHA || host.writes != 1 {
		t.Fatalf("implementation-task approval=%+v writes=%d err=%v", got, host.writes, err)
	}
	read, err := r.ApprovalFor(t.Context(), 17, d.Head, host.mainlineSHA, "")
	if err != nil || read != got {
		t.Fatalf("observed integration approval=%+v %v", read, err)
	}
}

func TestPublishDriverApprovalRefusesBeforeWrite(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*approvalHost, *approval.DriverDecision)
	}{
		{"wrong identity", func(h *approvalHost, _ *approval.DriverDecision) { h.login = "impostor" }},
		{"no configured driver", func(h *approvalHost, _ *approval.DriverDecision) {
			h.config = "approval:\n  operator_login: operator\n"
		}},
		{"ungranted authority", func(h *approvalHost, _ *approval.DriverDecision) {
			h.authority = strings.ReplaceAll(h.authority, "approve-merge", "rule-acceptance")
		}},
		{"bare tier promotion", func(h *approvalHost, _ *approval.DriverDecision) {
			h.authority = strings.Replace(h.authority, "| 2026-10-05 | promotion | tier-4 | operator-directed test grant |\n", "", 1)
		}},
		{"wrong policy", func(_ *approvalHost, d *approval.DriverDecision) { d.PolicyCommit = strings.Repeat("d", 40) }},
		{"prepared draft targets integration", func(h *approvalHost, d *approval.DriverDecision) {
			h.pull.Head.Ref = "draft/widget"
			h.pull.Base.Ref, h.pull.Base.SHA = "feature/widget", strings.Repeat("e", 40)
			d.Digest = h.digest
		}},
		{"stale head", func(_ *approvalHost, d *approval.DriverDecision) { d.Head = strings.Repeat("d", 40) }},
		{"draft PR", func(h *approvalHost, _ *approval.DriverDecision) { h.pull.Draft = true }},
		{"closed PR", func(h *approvalHost, _ *approval.DriverDecision) { h.pull.State = "closed" }},
		{"no review evidence", func(_ *approvalHost, d *approval.DriverDecision) { d.Evidence = " " }},
		{"oversized encoded evidence", func(_ *approvalHost, d *approval.DriverDecision) {
			d.Evidence = strings.Repeat("\x01", 6000)
		}},
		{"prepared draft without digest", func(h *approvalHost, _ *approval.DriverDecision) { h.pull.Head.Ref = "draft/widget" }},
		{"prepared draft wrong digest", func(h *approvalHost, d *approval.DriverDecision) {
			h.pull.Head.Ref = "draft/widget"
			d.Digest = "sha256:" + strings.Repeat("d", 64)
		}},
		{"operator requests changes", func(h *approvalHost, d *approval.DriverDecision) {
			h.reviews = []map[string]any{{"id": 20, "user": map[string]string{"login": "operator"}, "state": "CHANGES_REQUESTED", "commit_id": d.Head}}
		}},
		{"push before publication", func(h *approvalHost, _ *approval.DriverDecision) {
			h.mutate = func(h *approvalHost) {
				if h.pullReads == 2 {
					h.pull.Head.SHA = strings.Repeat("d", 40)
				}
			}
		}},
		{"mainline moves before publication", func(h *approvalHost, _ *approval.DriverDecision) {
			h.mutate = func(h *approvalHost) {
				if h.pullReads == 2 {
					h.pull.Base.SHA = strings.Repeat("d", 40)
				}
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, host, d := approvalFixture(t, "driver")
			tc.edit(host, &d)
			if _, err := r.Approve(t.Context(), d); err == nil || host.writes != 0 {
				t.Fatalf("refusal error=%v writes=%d", err, host.writes)
			}
		})
	}
}

func TestPreparedDraftDriverApprovalUsesExplicitReviewedDigest(t *testing.T) {
	r, host, d := approvalFixture(t, "driver")
	host.pull.Head.Ref = "draft/widget"
	d.Digest = "sha256:" + strings.Repeat("c", 64)
	if _, err := r.Approve(t.Context(), d); err != nil || host.writes != 1 {
		t.Fatalf("prepared approval: %v writes=%d", err, host.writes)
	}
	if _, err := r.ApprovalFor(t.Context(), 17, d.Head, host.pull.Base.SHA, "sha256:"+strings.Repeat("d", 64)); !errors.Is(err, approval.ErrNoApproval) {
		t.Fatalf("different compiled digest authorized: %v", err)
	}
}

func TestDriverApprovalMovingAfterPublicationIsRetainedButNotReportedCurrent(t *testing.T) {
	r, host, d := approvalFixture(t, "driver")
	host.mutate = func(h *approvalHost) {
		if h.pullReads == 3 {
			h.pull.Head.SHA = strings.Repeat("d", 40)
		}
	}
	if _, err := r.Approve(t.Context(), d); err == nil || host.writes != 1 {
		t.Fatalf("post-publication race err=%v writes=%d", err, host.writes)
	}
	if _, err := r.ApprovalFor(t.Context(), 17, d.Head, host.pull.Base.SHA, ""); !errors.Is(err, approval.ErrNoApproval) {
		t.Fatalf("stale approval accepted: %v", err)
	}
}

func TestOperatorChangesRequestPreventsDriverLock(t *testing.T) {
	r, host, d := approvalFixture(t, "driver")
	if _, err := r.Approve(t.Context(), d); err != nil {
		t.Fatal(err)
	}
	host.reviews = append(host.reviews, map[string]any{"id": 28, "user": map[string]string{"login": "operator"}, "state": "CHANGES_REQUESTED", "commit_id": d.Head})
	if _, err := r.ApprovalFor(t.Context(), 17, d.Head, host.pull.Base.SHA, ""); !errors.Is(err, approval.ErrNoApproval) {
		t.Fatalf("operator decision ignored: %v", err)
	}
}

func TestRevisedPreparedContractNeedsFreshDriverApproval(t *testing.T) {
	r, host, first := approvalFixture(t, "driver")
	host.pull.Head.Ref = "draft/widget"
	first.Digest = host.digest
	if _, err := r.Approve(t.Context(), first); err != nil {
		t.Fatal(err)
	}
	host.pull.Head.SHA = strings.Repeat("d", 40)
	host.digest = "sha256:" + strings.Repeat("e", 64)
	if _, err := r.ApprovalFor(t.Context(), 17, host.pull.Head.SHA, host.pull.Base.SHA, host.digest); !errors.Is(err, approval.ErrNoApproval) {
		t.Fatalf("old review authorized changed contract: %v", err)
	}
	renewed := first
	renewed.Head, renewed.Digest = host.pull.Head.SHA, host.digest
	renewed.Evidence = "Both independent PRD and design H2 reviews approve the new digest; current readiness and native checks pass."
	got, err := r.Approve(t.Context(), renewed)
	if err != nil || got.Operator != "driver" || got.Kind != "argus" || got.ReviewID != 28 || host.writes != 2 {
		t.Fatalf("fresh driver approval=%+v writes=%d error=%v", got, host.writes, err)
	}
	observed, err := r.ApprovalFor(t.Context(), 17, renewed.Head, host.pull.Base.SHA, renewed.Digest)
	if err != nil || observed != got {
		t.Fatalf("new contract decision=%+v error=%v", observed, err)
	}
	if _, err := r.ApprovalFor(t.Context(), 17, renewed.Head, host.pull.Base.SHA, first.Digest); !errors.Is(err, approval.ErrNoApproval) {
		t.Fatalf("prior digest accepted on new head: %v", err)
	}
}

func TestMovementBetweenFinalObservationAndSubmissionCannotAuthorizeNewVersion(t *testing.T) {
	for _, field := range []string{"head", "mainline"} {
		t.Run(field, func(t *testing.T) {
			r, host, d := approvalFixture(t, "driver")
			oldHead, oldPolicy := host.pull.Head.SHA, host.pull.Base.SHA
			prior := r.write
			r.write = func(ctx context.Context, body []byte, args ...string) ([]byte, error) {
				if field == "head" {
					host.pull.Head.SHA = strings.Repeat("d", 40)
				} else {
					host.pull.Base.SHA = strings.Repeat("d", 40)
					host.mainlineSHA = host.pull.Base.SHA
				}
				return prior(ctx, body, args...)
			}
			if _, err := r.Approve(t.Context(), d); err == nil || !strings.Contains(err.Error(), "changed") || host.writes != 1 {
				t.Fatalf("submission race accepted or retried: %v, writes=%d", err, host.writes)
			}
			retained, err := approval.ParseDriver(host.published.Body)
			if err != nil || retained.Head != oldHead || retained.PolicyCommit != oldPolicy || host.published.CommitID != oldHead {
				t.Fatalf("old decision lost its exact binding: %+v %v", retained, err)
			}
			if _, err := r.ApprovalFor(t.Context(), 17, host.pull.Head.SHA, host.pull.Base.SHA, ""); !errors.Is(err, approval.ErrNoApproval) {
				t.Fatalf("raced decision authorized changed %s: %v", field, err)
			}
		})
	}
}

func TestCandidateBaseCannotDeclareItselfTrustedMainline(t *testing.T) {
	r, host, d := approvalFixture(t, "driver")
	host.pull.Base.Ref = "candidate-policy"
	host.config = "schema_version: pyxis-config/v2\nbranch: candidate-policy\napproval:\n  operator_login: operator\n  argus_login: driver\n"
	if _, err := r.Approve(t.Context(), d); err == nil || host.writes != 0 {
		t.Fatalf("candidate-selected base granted approval: %v, writes=%d", err, host.writes)
	}
}

func TestIntegrationBranchCannotSupplyApprovalPolicy(t *testing.T) {
	r, host, d := approvalFixture(t, "driver")
	host.pull.Base.Ref, host.pull.Base.SHA = "feature/widget", strings.Repeat("e", 40)
	host.candidateConfig = host.config
	host.config = "approval:\n  operator_login: operator\n"
	if _, err := r.Approve(t.Context(), d); err == nil || host.writes != 0 {
		t.Fatalf("candidate policy granted approval: %v, writes=%d", err, host.writes)
	}
}

func TestInvalidMainlineObservationRefusesBeforeWrite(t *testing.T) {
	for _, payload := range []string{"transport-error", "not-json", `{"name":"candidate","commit":{"sha":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}}`, `{"name":"main","commit":{"sha":"invalid"}}`} {
		t.Run(payload, func(t *testing.T) {
			r, host, d := approvalFixture(t, "driver")
			prior := r.run
			r.run = func(ctx context.Context, args ...string) ([]byte, error) {
				if strings.HasPrefix(args[len(args)-1], "repos/example/repo/branches/") {
					if payload == "transport-error" {
						return nil, errors.New("mainline unavailable")
					}
					return []byte(payload), nil
				}
				return prior(ctx, args...)
			}
			if _, err := r.Approve(t.Context(), d); err == nil || host.writes != 0 {
				t.Fatalf("invalid mainline accepted: %v writes=%d", err, host.writes)
			}
		})
	}
}

func TestIntegrationApprovalReobservesMainlinePolicy(t *testing.T) {
	for _, phase := range []string{"preflight", "postpublication"} {
		t.Run(phase, func(t *testing.T) {
			r, host, d := approvalFixture(t, "driver")
			host.pull.Base.Ref, host.pull.Base.SHA = "feature/widget", strings.Repeat("e", 40)
			oldPolicy := host.mainlineSHA
			wantWrites := 0
			if phase == "preflight" {
				host.mutate = func(h *approvalHost) {
					if h.pullReads == 2 {
						h.mainlineSHA = strings.Repeat("d", 40)
					}
				}
			} else {
				wantWrites = 1
				prior := r.write
				r.write = func(ctx context.Context, body []byte, args ...string) ([]byte, error) {
					host.mainlineSHA = strings.Repeat("d", 40)
					return prior(ctx, body, args...)
				}
			}
			if _, err := r.Approve(t.Context(), d); err == nil || host.writes != wantWrites {
				t.Fatalf("policy move accepted: %v writes=%d", err, host.writes)
			}
			if wantWrites == 1 {
				retained, err := approval.ParseDriver(host.published.Body)
				if err != nil || retained.PolicyCommit != oldPolicy {
					t.Fatalf("old policy binding lost: %+v %v", retained, err)
				}
			}
		})
	}
}

func TestTrustedMainlineUsesHostDefaultBranchIncludingNonDefaultName(t *testing.T) {
	r, host, d := approvalFixture(t, "driver")
	host.defaultBranch, host.pull.Base.Ref = "trunk", "trunk"
	host.config = "schema_version: pyxis-config/v2\nbranch: trunk\napproval:\n  operator_login: operator\n  argus_login: driver\n"
	if _, err := r.Approve(t.Context(), d); err != nil || host.writes != 1 {
		t.Fatalf("configured host mainline refused: %v, writes=%d", err, host.writes)
	}
}

func TestTrustedMainlineChangeRefusesCurrentDecision(t *testing.T) {
	for _, phase := range []string{"preflight", "postpublication"} {
		t.Run(phase, func(t *testing.T) {
			r, host, d := approvalFixture(t, "driver")
			reads := 0
			prior := r.run
			r.run = func(ctx context.Context, args ...string) ([]byte, error) {
				if args[len(args)-1] == "repos/example/repo" {
					reads++
					if phase == "preflight" && reads == 2 || phase == "postpublication" && reads == 3 {
						host.defaultBranch = "changed-mainline"
					}
				}
				return prior(ctx, args...)
			}
			_, err := r.Approve(t.Context(), d)
			wantWrites := 0
			if phase == "postpublication" {
				wantWrites = 1
			}
			if err == nil || host.writes != wantWrites {
				t.Fatalf("changed host mainline accepted: %v, writes=%d", err, host.writes)
			}
		})
	}
}
