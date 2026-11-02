package github

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/draft/approval"
)

func TestDriverReadFailuresNeverPublish(t *testing.T) {
	for _, tc := range []struct {
		name, path, body string
	}{
		{"mainline unavailable", "repos/example/repo", ""},
		{"mainline malformed", "repos/example/repo", "{"},
		{"mainline absent", "repos/example/repo", `{}`},
		{"target unavailable", "/pulls/17", ""},
		{"target malformed", "/pulls/17", "{"},
		{"target incomplete", "/pulls/17", `{}`},
		{"policy unavailable", "/contents/.pyxis/config.yaml", ""},
		{"policy malformed", "/contents/.pyxis/config.yaml", "{"},
		{"policy directory", "/contents/.pyxis/config.yaml", `{"type":"dir","encoding":"base64","size":0,"content":""}`},
		{"policy oversized", "/contents/.pyxis/config.yaml", `{"type":"file","encoding":"base64","size":1048577,"content":""}`},
		{"policy corrupt encoding", "/contents/.pyxis/config.yaml", `{"type":"file","encoding":"base64","size":1,"content":"?"}`},
		{"policy truncated", "/contents/.pyxis/config.yaml", `{"type":"file","encoding":"base64","size":1,"content":""}`},
		{"policy invalid schema", "/contents/.pyxis/config.yaml", `{"type":"file","encoding":"base64","size":7,"content":"bm9wZTogMQ=="}`},
		{"grant unavailable", "/contents/.pyxis/skills/argus/authority-contract.md", ""},
		{"identity unavailable", "user", ""},
		{"identity malformed", "user", "{"},
		{"prepared commit unavailable", "/commits/", ""},
		{"prepared commit malformed", "/commits/", "{"},
		{"unprepared draft", "/commits/", `{"commit":{"message":"implementation work"}}`},
		{"duplicate digest", "/commits/", `{"commit":{"message":"docs(draft): prepare lock widget\nDraft-Digest: sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc\nDraft-Digest: sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}}`},
		{"reviews unavailable", "/reviews?", ""},
		{"reviews malformed", "/reviews?", "{"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, host, d := approvalFixture(t, "driver")
			if strings.Contains(tc.path, "/commits/") {
				host.pull.Head.Ref, d.Digest = "draft/widget", host.digest
			}
			prior := r.run
			r.run = func(ctx context.Context, args ...string) ([]byte, error) {
				if strings.Contains(args[len(args)-1], tc.path) {
					if tc.body == "" {
						return nil, errors.New("host unavailable")
					}
					return []byte(tc.body), nil
				}
				return prior(ctx, args...)
			}
			if _, err := r.Approve(t.Context(), d); err == nil || host.writes != 0 {
				t.Fatalf("host failure published approval: error=%v writes=%d", err, host.writes)
			}
		})
	}
}

func TestAmbiguousPublicationNeverRetriesOrReportsApproval(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*submittedReview)
	}{
		{"different receipt", func(r *submittedReview) { r.ID++ }},
		{"different account", func(r *submittedReview) { r.User.Login = "impostor" }},
		{"different head", func(r *submittedReview) { r.CommitID = strings.Repeat("d", 40) }},
		{"different decision", func(r *submittedReview) { r.Body += "\nmodified" }},
		{"dismissed decision", func(r *submittedReview) { r.State = "DISMISSED" }},
		{"receipt unavailable", nil},
		{"receipt malformed", nil},
		{"publication unavailable", nil},
		{"publication malformed", nil},
		{"publication no identity", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, host, d := approvalFixture(t, "driver")
			priorRead, priorWrite := r.run, r.write
			r.run = func(ctx context.Context, args ...string) ([]byte, error) {
				if strings.Contains(args[len(args)-1], "/reviews/27") {
					if tc.name == "receipt unavailable" {
						return nil, errors.New("receipt unavailable")
					}
					if tc.name == "receipt malformed" {
						return []byte("{"), nil
					}
					if tc.edit != nil {
						changed := host.published
						tc.edit(&changed)
						return json.Marshal(changed)
					}
				}
				return priorRead(ctx, args...)
			}
			r.write = func(ctx context.Context, body []byte, args ...string) ([]byte, error) {
				out, err := priorWrite(ctx, body, args...)
				if err != nil {
					return out, err
				}
				switch tc.name {
				case "publication unavailable":
					return nil, errors.New("publication acknowledgement lost")
				case "publication malformed":
					return []byte("{"), nil
				case "publication no identity":
					return []byte(`{"id":0}`), nil
				default:
					return out, nil
				}
			}
			if _, err := r.Approve(t.Context(), d); err == nil || host.writes != 1 {
				t.Fatalf("ambiguous receipt accepted or retried: error=%v writes=%d", err, host.writes)
			}
		})
	}
}

// A repository whose mainline names no operator fails closed when an approval is
// read or published, and the refusal names the key that fixes it. Config load
// succeeded: the repository is usable until something needs approving.
func TestApprovalWithNoConfiguredOperatorFailsClosed(t *testing.T) {
	for name, config := range map[string]string{
		"nothing configured": "schema_version: pyxis-config/v2\n",
		"driver only":        "approval:\n  argus_login: driver\n",
	} {
		t.Run(name, func(t *testing.T) {
			r, host, d := approvalFixture(t, "driver")
			host.config = config
			host.reviews = []map[string]any{{"id": 19, "user": map[string]string{"login": "driver"}, "state": "APPROVED", "commit_id": d.Head}}
			if got, err := r.ApprovalFor(t.Context(), 17, d.Head, host.pull.Base.SHA, ""); err == nil || !strings.Contains(err.Error(), "approval.operator_login") {
				t.Fatalf("read approval = %+v, %v; want the named refusal", got, err)
			}
			if got, err := r.Approve(t.Context(), d); err == nil || !strings.Contains(err.Error(), "approval.operator_login") {
				t.Fatalf("published approval = %+v, %v; want the named refusal", got, err)
			}
		})
	}
	reader := NewPolicyReader("example/repo", "GH_TOKEN")
	if _, err := reader.Approval(t.Context(), 17, strings.Repeat("a", 40)); err == nil {
		t.Fatal("the operator-less policy reader accepted the identity-free Port")
	}
}

func TestBoundReaderPreservesHumanApprovalAndRefusesObservationFailures(t *testing.T) {
	r, host, d := approvalFixture(t, "driver")
	host.config = "approval:\n  operator_login: operator\n"
	host.reviews = []map[string]any{{"id": 19, "user": map[string]string{"login": "operator"}, "state": "APPROVED", "commit_id": d.Head}}
	if got, err := r.ApprovalFor(t.Context(), 17, d.Head, host.pull.Base.SHA, ""); err != nil || got.Operator != "operator" || got.Kind != "" {
		t.Fatalf("legacy operator approval changed: %+v %v", got, err)
	}
	for _, tc := range []struct{ name, path string }{{"first head", "/pulls/17"}, {"reviews", "/reviews?"}, {"policy", "/contents/.pyxis/config.yaml"}, {"second head", "/pulls/17"}} {
		t.Run(tc.name, func(t *testing.T) {
			r, host, d := approvalFixture(t, "driver")
			prior := r.run
			pullReads := 0
			r.run = func(ctx context.Context, args ...string) ([]byte, error) {
				path := args[len(args)-1]
				if path == "repos/example/repo/pulls/17" {
					pullReads++
				}
				if strings.Contains(path, tc.path) && (tc.name != "second head" || pullReads == 2) {
					return nil, errors.New("host observation failed")
				}
				return prior(ctx, args...)
			}
			if _, err := r.ApprovalFor(t.Context(), 17, d.Head, host.pull.Base.SHA, ""); err == nil || host.writes != 0 {
				t.Fatalf("failed observation authorized approval: %v", err)
			}
		})
	}
	r, host, d = approvalFixture(t, "driver")
	host.mutate = func(h *approvalHost) {
		if h.pullReads == 2 {
			h.pull.Head.SHA = strings.Repeat("d", 40)
		}
	}
	if _, err := r.ApprovalFor(t.Context(), 17, d.Head, host.pull.Base.SHA, ""); !errors.Is(err, approval.ErrNoApproval) {
		t.Fatalf("moving head accepted: %v", err)
	}
}

func TestDriverAdapterRequiresExplicitBindingsAndCredential(t *testing.T) {
	r, host, d := approvalFixture(t, "driver")
	for _, reader := range []*Reader{nil, {}, {Repo: "invalid", run: r.run, write: r.write}} {
		if _, err := reader.Approve(t.Context(), d); err == nil {
			t.Fatal("unconfigured writer accepted")
		}
		if _, err := reader.ApprovalFor(t.Context(), 17, d.Head, host.pull.Base.SHA, ""); err == nil {
			t.Fatal("unconfigured reader accepted")
		}
	}
	d.Digest = host.digest
	if _, err := r.Approve(t.Context(), d); err == nil || host.writes != 0 {
		t.Fatal("contract digest on implementation PR accepted")
	}
	const credential = "PYXIS_APPROVAL_TEST_MISSING_CREDENTIAL"
	t.Setenv(credential, "")
	configured := NewPolicyReader("example/repo", credential)
	if _, err := configured.run(t.Context(), "api", "user"); err == nil {
		t.Fatal("missing explicit read credential fell back to another account")
	}
	if _, err := configured.write(t.Context(), nil, "api", "--method", "POST"); err == nil {
		t.Fatal("missing explicit write credential fell back to another account")
	}
}
