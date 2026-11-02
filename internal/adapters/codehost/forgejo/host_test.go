package forgejocodehost

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/changesize"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/prdescription"
)

// testDescription is a complete description, as the Worker hands it over.
var testDescription = prdescription.Description{
	LinesOfCode: "3 (+3/-0)", Complexity: "low", BlastRadius: "low", Decision: "two-way", ReviewNotes: "None.",
	What: "- Change the thing.", Why: "It was needed.", How: "- Directly.", Testing: "- [x] go test: passed.",
}

type roundTrip func(*http.Request) (*http.Response, error)
type rawBody []byte

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func testHost(t *testing.T, route func(*http.Request) (any, int)) *Host {
	t.Helper()
	client, err := forgejo.NewWithHTTP("https://forgejo.test", "test-token", &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "token test-token" {
			t.Errorf("authorization=%q", r.Header.Get("Authorization"))
		}
		body, code := route(r)
		if code == 0 {
			code = 200
		}
		b, err := json.Marshal(body)
		if raw, ok := body.(rawBody); ok {
			b = []byte(raw)
			err = nil
		}
		if err != nil {
			t.Fatal(err)
		}
		header := http.Header{"Content-Type": []string{"application/json"}}
		var list []json.RawMessage
		if json.Unmarshal(b, &list) == nil && strings.HasPrefix(string(b), "[") {
			header.Set("X-Total-Count", strconv.Itoa(len(list)))
		}
		return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(string(b))), Header: header, Request: r}, nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	return New(client, "https://forgejo.test", "owner/repo", "main")
}

func pullFixture() Pull {
	p := Pull{Number: 7, State: "open", Draft: true}
	p.Head.Ref = "feat/scope/task-3"
	p.Head.SHA = "abc"
	p.Head.Repo.FullName = "owner/repo"
	p.Head.Repo.ID = 1
	p.Base.Ref = "main"
	return p
}

func TestProposalNativeWritesAndAssociation(t *testing.T) {
	for _, sameHost := range []bool{true, false} {
		t.Run(strconv.FormatBool(sameHost), func(t *testing.T) {
			p := pullFixture()
			posted := false
			h := testHost(t, func(r *http.Request) (any, int) {
				switch {
				case r.Method == "GET" && r.URL.Path == "/api/v1/repos/owner/repo/pulls":
					return []Pull{}, 200
				case strings.Contains(r.URL.Path, "/compare/"):
					return map[string]int{"total_commits": 1}, 200
				case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/pulls"):
					var body struct{ Title, Head, Base, Body string }
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Fatal(err)
					}
					if body.Title != "WIP: Ship feature" || body.Head != p.Head.Ref || body.Base != "main" {
						t.Fatalf("request=%+v", body)
					}
					if !strings.Contains(body.Body, "pyxis-task-proposal:") || strings.Contains(body.Body, "- Closes #3") != sameHost {
						t.Fatalf("association=%s", body.Body)
					}
					if _, problems := prdescription.Parse(body.Body); len(problems) != 0 || !strings.Contains(body.Body, "- Implements task T1 (") {
						t.Fatalf("description problems %v in %s", problems, body.Body)
					}
					posted = true
					return p, 201
				case strings.HasSuffix(r.URL.Path, "/pulls/7"):
					return p, 200
				default:
					t.Fatalf("unexpected %s %s", r.Method, r.URL)
					return nil, 500
				}
			})
			source := "https://forgejo.test/owner/repo/issues/3"
			if !sameHost {
				source = "https://other.test/owner/repo/issues/3"
			}
			got, err := h.OpenProposal(t.Context(), port.Task{ID: "T1", Number: 3, Title: "Ship feature", HeadBranch: p.Head.Ref, SourceRef: source}, nil, testDescription)
			if err != nil || got.Number != 7 || !posted {
				t.Fatalf("OpenProposal=%+v %v", got, err)
			}
		})
	}
}

func TestOpenProposalBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name       string
		list       any
		comparison any
		created    Pull
		read       Pull
		failPath   string
		wantErr    bool
		hollow     bool
	}{
		{name: "existing", list: []Pull{pullFixture()}, wantErr: false},
		{name: "nothing", list: []Pull{}, comparison: map[string]int{"total_commits": 0}, wantErr: true, hollow: true},
		{name: "missing comparison", list: []Pull{}, comparison: struct{}{}, wantErr: true},
		{name: "bad acknowledgement", list: []Pull{}, comparison: map[string]int{"total_commits": 1}, wantErr: true},
		{name: "not draft", list: []Pull{}, comparison: map[string]int{"total_commits": 1}, created: Pull{Number: 7}, read: func() Pull { p := pullFixture(); p.Draft = false; return p }(), wantErr: true},
		{name: "list failure", failPath: "/pulls", wantErr: true},
		{name: "compare failure", list: []Pull{}, failPath: "compare", wantErr: true},
		{name: "write failure", list: []Pull{}, comparison: map[string]int{"total_commits": 1}, failPath: "POST", wantErr: true},
		{name: "read failure", list: []Pull{}, comparison: map[string]int{"total_commits": 1}, created: Pull{Number: 7}, failPath: "/pulls/7", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := testHost(t, func(r *http.Request) (any, int) {
				if tc.failPath != "" && (strings.Contains(r.URL.Path, tc.failPath) || r.Method == tc.failPath) {
					return nil, 503
				}
				switch {
				case strings.HasSuffix(r.URL.Path, "/pulls/7"):
					return tc.read, 200
				case r.Method == "POST":
					return tc.created, 201
				case strings.Contains(r.URL.Path, "compare"):
					return tc.comparison, 200
				default:
					return tc.list, 200
				}
			})
			h.DraftPrefix = ""
			_, err := h.OpenProposal(t.Context(), port.Task{ID: "T1", HeadBranch: pullFixture().Head.Ref}, nil, testDescription)
			if (err != nil) != tc.wantErr || tc.hollow && !errors.Is(err, port.ErrNoProposal) {
				t.Fatalf("error=%v", err)
			}
		})
	}
	h := testHost(t, func(*http.Request) (any, int) { return nil, 500 })
	if _, err := h.OpenProposal(t.Context(), port.Task{}, nil, testDescription); err == nil {
		t.Fatal("missing branch accepted")
	}
	if _, err := h.Pull(t.Context(), 0); err == nil {
		t.Fatal("missing number accepted")
	}
}

func TestRequiredChecksUseEffectiveRuleAndLatestStatuses(t *testing.T) {
	for _, tc := range []struct {
		name     string
		patterns []string
		rows     []status
		want     port.CheckState
		missing  int
	}{
		{"newest wins", []string{"build"}, []status{{1, "build", "failure"}, {2, "build", "success"}, {3, "optional", "failure"}}, port.CheckGreen, 0},
		{"missing", []string{"build", "test"}, []status{{1, "build", "success"}}, port.CheckPending, 1},
		{"native glob", []string{"ci/{linux,mac}/*"}, []status{{1, "ci/linux/test", "success"}, {2, "ci/mac/build", "failure"}}, port.CheckRed, 0},
		{"pending", nil, []status{{1, "build", "pending"}}, port.CheckPending, 0},
		{"required skipped", []string{"build"}, []status{{1, "build", "skipped"}}, port.CheckGreen, 0},
		{"required skip and missing", []string{"build", "test"}, []status{{1, "build", "skipped"}}, port.CheckPending, 1},
		{"required skipped excludes optional failure", []string{"build"}, []status{{1, "build", "skipped"}, {2, "optional", "failure"}}, port.CheckGreen, 0},
		{"glob skip and success", []string{"ci/*"}, []status{{1, "ci/linux", "skipped"}, {2, "ci/mac", "success"}}, port.CheckGreen, 0},
		{"glob skip and failure", []string{"ci/*"}, []status{{1, "ci/linux", "skipped"}, {2, "ci/mac", "failure"}}, port.CheckRed, 0},
		{"no required all skipped", nil, []status{{1, "build", "skipped"}, {2, "test", "skipped"}}, port.CheckPending, 0},
		{"no required skip and success", nil, []status{{1, "build", "skipped"}, {2, "test", "success"}}, port.CheckGreen, 0},
		{"no required skip and pending", nil, []status{{1, "build", "skipped"}, {2, "test", "pending"}}, port.CheckPending, 0},
		{"no required skip and failure", nil, []status{{1, "build", "skipped"}, {2, "test", "failure"}}, port.CheckRed, 0},
		{"empty", nil, nil, port.CheckPending, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := pullFixture()
			h := testHost(t, func(r *http.Request) (any, int) {
				switch {
				case strings.Contains(r.URL.Path, "/branches/"):
					return map[string]any{"name": "main", "protected": true, "effective_branch_protection_name": "m*"}, 200
				case strings.Contains(r.URL.Path, "branch_protections/"):
					return protection{RuleName: "m*", EnableStatusCheck: true, StatusCheckContexts: tc.patterns}, 200
				case strings.HasSuffix(r.URL.Path, "/statuses"):
					return append([]status{}, tc.rows...), 200
				default:
					return p, 200
				}
			})
			got, err := h.Checks(t.Context(), port.Proposal{Number: 7})
			if err != nil || got != tc.want {
				t.Fatalf("checks=%v %v", got, err)
			}
			checks, missing, _, err := h.checks(t.Context(), p)
			if err != nil || len(missing) != tc.missing || aggregate(checks, missing) != tc.want {
				t.Fatalf("checks=%v missing=%v err=%v", checks, missing, err)
			}
		})
	}
}

func TestChecksFailClosedOnMalformedOrMovingState(t *testing.T) {
	for _, failure := range []string{"pull", "malformed pull", "branch", "branch identity", "missing effective", "rule", "rule identity", "pattern", "statuses", "status identity", "head moved"} {
		t.Run(failure, func(t *testing.T) {
			reads := 0
			p := pullFixture()
			h := testHost(t, func(r *http.Request) (any, int) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/pulls/7"):
					reads++
					if failure == "pull" {
						return nil, 503
					}
					if failure == "malformed pull" {
						p.Head.SHA = ""
					}
					if failure == "head moved" && reads > 1 {
						p.Head.SHA = "def"
					}
					return p, 200
				case strings.Contains(r.URL.Path, "/branches/"):
					if failure == "branch" {
						return nil, 503
					}
					name, effective := "main", "main"
					if failure == "branch identity" {
						name = "other"
					}
					if failure == "missing effective" {
						effective = ""
					}
					return map[string]any{"name": name, "protected": true, "effective_branch_protection_name": effective}, 200
				case strings.Contains(r.URL.Path, "/branch_protections/"):
					if failure == "rule" {
						return nil, 503
					}
					rule := protection{RuleName: "main", EnableStatusCheck: true, StatusCheckContexts: []string{"build"}}
					if failure == "rule identity" {
						rule.RuleName = "other"
					}
					if failure == "pattern" {
						rule.StatusCheckContexts = []string{"["}
					}
					return rule, 200
				default:
					if failure == "statuses" {
						return nil, 503
					}
					if failure == "status identity" {
						return []status{{0, "", "success"}}, 200
					}
					return []status{{1, "build", "success"}}, 200
				}
			})
			if got, err := h.Checks(t.Context(), port.Proposal{Number: 7}); err == nil || got != port.CheckPending {
				t.Fatalf("checks=%v err=%v", got, err)
			}
		})
	}
}

func TestStateReadsEveryFeedbackSurfaceAndTrust(t *testing.T) {
	at := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	p := pullFixture()
	author := forgejo.User{ID: 5, Login: "reviewer"}
	h := testHost(t, func(r *http.Request) (any, int) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/pulls/7"):
			return p, 200
		case strings.Contains(r.URL.Path, "/branches/"):
			return map[string]any{"name": "main", "protected": true, "effective_branch_protection_name": "main"}, 200
		case strings.Contains(r.URL.Path, "/branch_protections/"):
			return protection{RuleName: "main", RequiredApprovals: 1, BlockOnRejectedReviews: true, IgnoreStaleApprovals: true}, 200
		case strings.HasSuffix(r.URL.Path, "/statuses"):
			return []status{{1, "build", "success"}}, 200
		case strings.HasSuffix(r.URL.Path, "/permission"):
			return map[string]string{"permission": "write"}, 200
		case strings.HasSuffix(r.URL.Path, "/reviews"):
			return []nativeReview{{ID: 20, User: author, State: "REQUEST_CHANGES", Official: true, SubmittedAt: at, Body: "fix"}, {ID: 21, State: "PENDING"}}, 200
		case strings.Contains(r.URL.Path, "/reviews/"):
			return []nativeComment{{ID: 30, User: author, Body: "inline", CreatedAt: at}}, 200
		default:
			return []nativeComment{{ID: 10, User: author, Body: "discussion", CreatedAt: at}, {ID: 11, User: author, Body: "imported", CreatedAt: at, OriginalAuthor: "foreign"}}, 200
		}
	})
	ref := port.ProposalRef{Repo: h.Key(), Number: 7}
	states, err := h.States(t.Context(), []port.ProposalRef{ref}, port.Cursor{})
	if err != nil || states[0].Unknown != nil || len(states[0].Comments) != 4 || states[0].ReviewDecision != port.ReviewChangesRequested {
		t.Fatalf("states=%+v err=%v", states, err)
	}
	if !states[0].Comments[1].Untrusted || states[0].Comments[0].AuthorAssociation != "COLLABORATOR" {
		t.Fatalf("trust=%+v", states[0].Comments)
	}
	positions := map[port.CommentSurface]time.Time{port.SurfaceDiscussion: at, port.SurfaceInlineReview: at, port.SurfaceReviewSummary: at}
	states, err = h.States(t.Context(), []port.ProposalRef{ref}, port.Cursor{Positions: positions, NativeIDs: map[string]bool{states[0].Comments[0].NativeID: true}})
	if err != nil || len(states[0].Comments) != 3 {
		t.Fatalf("equal timestamp cursor=%+v %v", states, err)
	}
	states, err = h.States(t.Context(), []port.ProposalRef{ref}, port.Cursor{Positions: positions})
	if err != nil || len(states[0].Comments) != 0 {
		t.Fatalf("legacy cursor=%+v %v", states, err)
	}
}

func TestStateUnknownClassesAndCancellation(t *testing.T) {
	for _, code := range []int{429, 503, 403} {
		h := testHost(t, func(*http.Request) (any, int) { return nil, code })
		states, err := h.States(t.Context(), []port.ProposalRef{{Repo: h.Key(), Number: 7}}, port.Cursor{})
		if err != nil || states[0].Unknown == nil {
			t.Fatalf("%d: %+v %v", code, states, err)
		}
	}
	h := testHost(t, func(*http.Request) (any, int) { return pullFixture(), 200 })
	states, err := h.States(t.Context(), []port.ProposalRef{{Repo: "forgejo:https://foreign:owner/repo", Number: 7}}, port.Cursor{})
	if err != nil || states[0].Unknown == nil {
		t.Fatal("cross-host reference accepted")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := h.States(ctx, []port.ProposalRef{{Number: 7}}, port.Cursor{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel=%v", err)
	}
}

func TestNativeStatusCommentApprovalAndSHAGuardedMerge(t *testing.T) {
	var paths []string
	h := testHost(t, func(r *http.Request) (any, int) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		if strings.HasSuffix(r.URL.Path, "/merge") {
			var body struct {
				SHA   string `json:"head_commit_id"`
				Force bool   `json:"force_merge"`
				Do    string `json:"Do"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.SHA != "abc" || body.Force || body.Do != "squash" {
				t.Fatalf("merge=%+v", body)
			}
		}
		return map[string]int{"id": 42}, 201
	})
	if err := h.SetStatus(t.Context(), "abc", "build", "success", "", "passed"); err != nil {
		t.Fatal(err)
	}
	if err := h.Approve(t.Context(), 7, "abc"); err != nil {
		t.Fatal(err)
	}
	if err := h.Merge(t.Context(), 7, "abc"); err != nil {
		t.Fatal(err)
	}
	wantReceipt := "forgejo-comment:" + base64.RawURLEncoding.EncodeToString([]byte("https://forgejo.test/repositories/42")) + ":42"
	if ref, err := h.PublishReview(t.Context(), port.Proposal{Number: 7}, "artifact"); err != nil || ref != wantReceipt {
		t.Fatalf("publish=%s %v", ref, err)
	}
	if len(paths) != 5 {
		t.Fatalf("writes=%v", paths)
	}
	if err := h.SetStatus(t.Context(), "", "build", "success", "", ""); err == nil {
		t.Fatal("empty status revision accepted")
	}
	if _, err := h.PublishReview(t.Context(), port.Proposal{Number: 7}, ""); err == nil {
		t.Fatal("empty artifact accepted")
	}
}

func TestPublishReviewRefusesUnidentifiedRepositoryAndWriteFailure(t *testing.T) {
	for _, fault := range []string{"repository outage", "missing identity", "write failed"} {
		t.Run(fault, func(t *testing.T) {
			writes := 0
			h := testHost(t, func(r *http.Request) (any, int) {
				if r.Method == "POST" {
					writes++
					return nil, 503
				}
				if fault == "repository outage" {
					return nil, 503
				}
				if fault == "missing identity" {
					return map[string]int{"id": 0}, 200
				}
				return map[string]int{"id": 42}, 200
			})
			if _, err := h.PublishReview(t.Context(), port.Proposal{Number: 7}, "artifact"); err == nil {
				t.Fatal("unverified receipt accepted")
			}
			if fault != "write failed" && writes != 0 {
				t.Fatal("write preceded repository identity")
			}
		})
	}
}

func TestDescribeProposalReplacesOnlyTheDescription(t *testing.T) {
	p := pullFixture()
	stale := testDescription
	stale.LinesOfCode = "1 (+1/-0)"
	p.Body = stale.Render() + "\n<!-- pyxis-task-proposal:abc -->\n\n## Pyxis execution\nStatus: running.\n"
	patched := ""
	h := testHost(t, func(r *http.Request) (any, int) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/v1/repos/owner/repo/pulls":
			return []Pull{p}, 200
		case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/pulls/7"):
			return p, 200
		case r.Method == "PATCH" && strings.HasSuffix(r.URL.Path, "/pulls/7"):
			var body struct{ Body string }
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			patched, p.Body = body.Body, body.Body
			return p, 200
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL)
			return nil, 500
		}
	})
	task := port.Task{ID: "T1", HeadBranch: p.Head.Ref, SourceRef: "ref"}
	found, err := h.DescribeProposal(t.Context(), task, nil, testDescription)
	if err != nil || !found {
		t.Fatalf("DescribeProposal = %v, %v", found, err)
	}
	if !strings.Contains(patched, "- Lines of code: 3 (+3/-0)") || !strings.Contains(patched, "<!-- pyxis-task-proposal:abc -->") || !strings.HasSuffix(patched, "## Pyxis execution\nStatus: running.\n") {
		t.Fatalf("patched body:\n%s", patched)
	}
	if found, err := h.DescribeProposal(t.Context(), port.Task{ID: "T2", HeadBranch: "feat/other"}, nil, testDescription); err != nil || found {
		t.Fatalf("no open proposal: %v, %v", found, err)
	}
	if _, err := h.DescribeProposal(t.Context(), port.Task{}, nil, testDescription); err == nil {
		t.Fatal("a task without a head branch was described")
	}
	if found, err := h.DescribeProposal(t.Context(), port.Task{ID: "T1"}, branchWorkdir(p.Head.Ref), testDescription); err != nil || !found {
		t.Fatalf("the workdir's branch names the head when the task has none: %v, %v", found, err)
	}
	failing := testHost(t, func(r *http.Request) (any, int) {
		if r.Method == "PATCH" {
			return nil, 503
		}
		if strings.HasSuffix(r.URL.Path, "/pulls/7") {
			return p, 200
		}
		return []Pull{p}, 200
	})
	if found, err := failing.DescribeProposal(t.Context(), task, nil, stale); !found || err == nil || !strings.Contains(err.Error(), "describe proposal #7") {
		t.Fatalf("a failed write: %v, %v", found, err)
	}
	listFailure := testHost(t, func(*http.Request) (any, int) { return nil, 503 })
	if _, err := listFailure.DescribeProposal(t.Context(), task, nil, testDescription); err == nil {
		t.Fatal("a failed listing was not reported")
	}
}

// branchWorkdir is a port.Workdir that only names its branch.
type branchWorkdir string

func (b branchWorkdir) Dir() string                            { return "" }
func (b branchWorkdir) Branch() string                         { return string(b) }
func (branchWorkdir) Checkpoint(context.Context) (bool, error) { return false, nil }
func (branchWorkdir) Push(context.Context) (bool, error)       { return false, nil }
func (branchWorkdir) ChangeSize(context.Context) (changesize.Size, error) {
	return changesize.Size{}, nil
}
func (branchWorkdir) Cleanup(context.Context) error { return nil }
