package forgejocodehost

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func snapshotRoute(p Pull, rule protection, reviews []nativeReview, comments []nativeComment) func(*http.Request) (any, int) {
	return func(r *http.Request) (any, int) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/pulls/7"):
			return p, 200
		case strings.Contains(r.URL.Path, "/branches/"):
			return map[string]any{"name": "main", "protected": rule.RuleName != "", "effective_branch_protection_name": rule.RuleName}, 200
		case strings.Contains(r.URL.Path, "/branch_protections/"):
			return rule, 200
		case strings.HasSuffix(r.URL.Path, "/statuses"):
			return []status{{1, "build", "success"}}, 200
		case strings.HasSuffix(r.URL.Path, "/permission"):
			return map[string]string{"permission": "read"}, 200
		case strings.HasSuffix(r.URL.Path, "/reviews"):
			return append([]nativeReview{}, reviews...), 200
		case strings.Contains(r.URL.Path, "/reviews/"):
			return []nativeComment{}, 200
		default:
			return append([]nativeComment{}, comments...), 200
		}
	}
}

func TestForgejoStateReaderConformsToPort(t *testing.T) {
	p := pullFixture()
	base := snapshotRoute(p, protection{}, nil, nil)
	h := testHost(t, func(r *http.Request) (any, int) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/pulls/8"):
			landed := p
			landed.Number, landed.State, landed.Merged, landed.MergeCommitSHA = 8, "closed", true, "landed-revision"
			return landed, 200
		case strings.HasSuffix(r.URL.Path, "/pulls/9"):
			return nil, 503
		default:
			return base(r)
		}
	})
	refs := []port.ProposalRef{{Repo: h.Key(), Number: 7}, {Repo: h.Key(), Number: 8}, {Repo: h.Key(), Number: 9}}
	if err := port.CheckReaderConformance(t.Context(), h, port.ReaderConformanceCase{Refs: refs, Landed: refs[1], LandedRevision: "landed-revision", Unknown: refs[2]}); err != nil {
		t.Fatal(err)
	}
}

func TestReviewVerdictsRespectProtectionAndReviewLifecycle(t *testing.T) {
	at := time.Now().UTC()
	actor := forgejo.User{ID: 1, Login: "owner"}
	for _, tc := range []struct {
		name, state, want                           string
		dismissed, stale, ignore, official, request bool
	}{
		{"approval", "APPROVED", port.ReviewApproved, false, false, true, true, false},
		{"stale approval", "APPROVED", "REVIEW_REQUIRED", false, true, true, true, false},
		{"stale allowed", "APPROVED", port.ReviewApproved, false, true, false, true, false},
		{"dismissed", "APPROVED", "REVIEW_REQUIRED", true, false, true, true, false},
		{"unofficial", "APPROVED", "REVIEW_REQUIRED", false, false, true, false, false},
		{"review requested", "REQUEST_REVIEW", "REVIEW_REQUIRED", false, false, true, true, true},
		{"comment", "COMMENT", "REVIEW_REQUIRED", false, false, true, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := nativeReview{ID: 1, User: actor, State: tc.state, CommitID: "abc", Official: tc.official, Dismissed: tc.dismissed, Stale: tc.stale, SubmittedAt: at}
			rule := protection{RuleName: "main", RequiredApprovals: 1, IgnoreStaleApprovals: tc.ignore, BlockOnOfficialReviewRequests: tc.request}
			h := testHost(t, snapshotRoute(pullFixture(), rule, []nativeReview{r}, nil))
			result, err := h.state(t.Context(), port.ProposalRef{Repo: h.Key(), Number: 7}, port.Cursor{})
			if err != nil || result.ReviewDecision != tc.want {
				t.Fatalf("state=%+v err=%v", result, err)
			}
		})
	}
}

func TestCommentTrustHandlesOwnersAllowlistAndUnreadableMembership(t *testing.T) {
	for _, tc := range []struct {
		login      string
		code       int
		permission string
		trusted    []string
		want       string
		wantErr    bool
	}{
		{"owner", 200, "", nil, "OWNER", false},
		{"alice", 200, "write", nil, "COLLABORATOR", false},
		{"alice", 200, "read", nil, "", false},
		{"alice", 200, "", []string{"https://forgejo.test/alice"}, "COLLABORATOR", false},
		{"alice", 403, "", nil, "", false},
		{"alice", 404, "", nil, "", false},
		{"alice", 503, "", nil, "", true},
		{"", 200, "", nil, "", false},
	} {
		t.Run(tc.login+strconv.Itoa(tc.code)+tc.permission, func(t *testing.T) {
			h := testHost(t, func(*http.Request) (any, int) { return map[string]string{"permission": tc.permission}, tc.code })
			h.TrustedActors = tc.trusted
			got, err := h.association(t.Context(), tc.login, false, map[string]string{})
			if got != tc.want || (err != nil) != tc.wantErr {
				t.Fatalf("trust=%s err=%v", got, err)
			}
		})
	}
}

func TestStateFailsClosedOnIncompleteNativeSurfaces(t *testing.T) {
	at := time.Now().UTC()
	actor := forgejo.User{ID: 1, Login: "owner"}
	review := nativeReview{ID: 1, User: actor, State: "COMMENT", SubmittedAt: at}
	comment := nativeComment{ID: 1, User: actor, CreatedAt: at}
	for _, fault := range []string{"checks", "comments", "malformed comment", "reviews", "malformed review", "inline", "malformed inline", "head", "read head", "landing", "permission"} {
		t.Run(fault, func(t *testing.T) {
			p := pullFixture()
			reads := 0
			base := snapshotRoute(p, protection{}, []nativeReview{review}, []nativeComment{comment})
			h := testHost(t, func(r *http.Request) (any, int) {
				if strings.HasSuffix(r.URL.Path, "/pulls/7") {
					reads++
					if fault == "head" && reads > 1 {
						p.Head.SHA = "def"
					}
					if fault == "read head" && reads > 1 {
						return nil, 503
					}
					if fault == "landing" {
						p.State = "unknown"
					}
					return p, 200
				}
				if strings.HasSuffix(r.URL.Path, "/statuses") && fault == "checks" {
					return nil, 503
				}
				if strings.HasSuffix(r.URL.Path, "/reviews") {
					if fault == "reviews" {
						return nil, 503
					}
					if fault == "malformed review" {
						return []nativeReview{{ID: 1, State: "COMMENT"}}, 200
					}
				}
				if strings.Contains(r.URL.Path, "/reviews/") {
					if fault == "inline" {
						return nil, 503
					}
					if fault == "malformed inline" {
						return []nativeComment{{ID: 1}}, 200
					}
				}
				if strings.HasSuffix(r.URL.Path, "/issues/7/comments") {
					if fault == "comments" {
						return nil, 503
					}
					if fault == "malformed comment" {
						return []nativeComment{{ID: 1}}, 200
					}
					if fault == "permission" {
						c := comment
						c.User.Login = "other"
						return []nativeComment{c}, 200
					}
				}
				if strings.HasSuffix(r.URL.Path, "/permission") && fault == "permission" {
					return nil, 503
				}
				return base(r)
			})
			if _, err := h.state(t.Context(), port.ProposalRef{Number: 7}, port.Cursor{}); err == nil {
				t.Fatal("incomplete snapshot accepted")
			}
		})
	}
}

func TestProposalInspectionAndProbeRemainRevisionBound(t *testing.T) {
	for _, tc := range []struct {
		name, state string
		merged      bool
		mergeSHA    string
		want        port.Landing
		wantErr     bool
	}{
		{"open", "open", false, "", port.LandingOpen, false},
		{"closed", "closed", false, "", port.LandingClosedUnlanded, false},
		{"landed", "closed", true, "merged", port.LandingLanded, false},
		{"missing merge revision", "closed", true, "", port.LandingUnknown, true},
		{"unknown", "other", false, "", port.LandingUnknown, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := pullFixture()
			p.State = tc.state
			p.Merged = tc.merged
			p.MergeCommitSHA = tc.mergeSHA
			h := testHost(t, snapshotRoute(p, protection{}, nil, nil))
			landing, sha, _, err := h.InspectProposal(t.Context(), port.Proposal{Number: 7, Branch: p.Head.Ref})
			if landing != tc.want || sha != p.Head.SHA || (err != nil) != tc.wantErr {
				t.Fatalf("inspection=%v %s %v", landing, sha, err)
			}
		})
	}
	for _, fault := range []string{"pull", "branch", "checks", "head", "moved"} {
		t.Run(fault, func(t *testing.T) {
			p := pullFixture()
			reads := 0
			base := snapshotRoute(p, protection{}, nil, nil)
			h := testHost(t, func(r *http.Request) (any, int) {
				if strings.HasSuffix(r.URL.Path, "/pulls/7") {
					reads++
					if fault == "pull" || fault == "head" && reads > 1 {
						return nil, 503
					}
					if fault == "moved" && reads > 1 {
						p.State = "closed"
					}
					return p, 200
				}
				if fault == "checks" && strings.HasSuffix(r.URL.Path, "/statuses") {
					return nil, 503
				}
				return base(r)
			})
			branch := p.Head.Ref
			if fault == "branch" {
				branch = "foreign"
			}
			if _, _, _, err := h.InspectProposal(t.Context(), port.Proposal{Number: 7, Branch: branch}); err == nil {
				t.Fatal("inconsistent inspection accepted")
			}
		})
	}
	p := pullFixture()
	foreign := p
	foreign.Head.Repo.FullName = "foreign/repo"
	foreign.Head.Ref = "feat/foreign"
	h := testHost(t, func(*http.Request) (any, int) { return []Pull{p, foreign}, 200 })
	got, err := h.OpenProposals(t.Context(), []port.Task{{ID: "T1", HeadBranch: p.Head.Ref}, {ID: "T2", HeadBranch: foreign.Head.Ref}, {ID: "T3"}})
	if err != nil || len(got) != 1 || got["T1"].Number != 7 {
		t.Fatalf("probe=%v %v", got, err)
	}
	if h.Cadence() <= 0 {
		t.Fatal("missing cadence")
	}
	h = testHost(t, func(*http.Request) (any, int) { return nil, 503 })
	if _, err := h.OpenProposals(t.Context(), nil); err == nil {
		t.Fatal("probe swallowed outage")
	}
	if _, err := h.PostComment(t.Context(), 7, "body"); err == nil {
		t.Fatal("comment swallowed outage")
	}
	h = testHost(t, func(*http.Request) (any, int) { return map[string]int{"id": 0}, 200 })
	if _, err := h.PostComment(t.Context(), 7, "body"); err == nil {
		t.Fatal("missing receipt accepted")
	}
	if err := h.Merge(t.Context(), 7, ""); err == nil {
		t.Fatal("merge without SHA accepted")
	}
}
