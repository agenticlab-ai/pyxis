package forgejopersist

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	implreport "github.com/agenticlab-ai/pyxis/internal/implreview/report"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

type harvestTransport func(*http.Request) (*http.Response, error)

func (f harvestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func nativeHarvest(t *testing.T, route func(string) (any, int)) *ForgejoAdapter {
	t.Helper()
	api, err := forgejo.NewWithHTTP("https://forgejo.test/base", "secret", &http.Client{Transport: harvestTransport(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "token secret" || r.Method != "GET" {
			t.Fatalf("unexpected authenticated read: %s %s", r.Method, r.URL)
		}
		value, code := route(strings.TrimPrefix(r.URL.Path, "/base/api/v1/repos/o/r"))
		b, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		header := http.Header{}
		var rows []json.RawMessage
		if json.Unmarshal(b, &rows) == nil && strings.HasPrefix(string(b), "[") {
			header.Set("X-Total-Count", strconv.Itoa(len(rows)))
		}
		return &http.Response{StatusCode: code, Header: header, Body: io.NopCloser(strings.NewReader(string(b))), Request: r}, nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	return &ForgejoAdapter{client: &Client{API: api, Repository: integrations.ResolvedRepository{Provider: integrations.Forgejo, BaseURL: "https://forgejo.test/base", Repository: "o/r"}}}
}

func harvestReceipt() persist.Ref {
	return persist.Ref{Medium: persist.MediumForgejo, Locator: "forgejo-comment:" + base64.RawURLEncoding.EncodeToString([]byte("https://forgejo.test/base/repositories/42")) + ":71"}
}

func TestForgejoHarvestEnumeratesAndExactReads(t *testing.T) {
	ref := "forgejo:https://forgejo.test/base:o/r#1"
	body, err := persist.ComposeArtifact(&implreport.Report{SchemaVersion: implreport.SchemaVersion, ProposalRef: ref, ReviewID: "review-1", Band: implreport.BandStandard, Verdict: implreport.VerdictApprove, Rounds: []implreport.Round{{ReviewedRevision: "abc", ContextID: "context-1", Findings: []implreport.Finding{}}}})
	if err != nil {
		t.Fatal(err)
	}
	comment := map[string]any{"id": 71, "body": strings.ReplaceAll(body, "\n", "\r\n"), "user": map[string]any{"id": 5, "login": "operator"}}
	absence := "cause: backend unavailable\n```impl-review-absent\n{\"absent\":true,\"proposal_ref\":\"forgejo:https://forgejo.test/base:o/r#2\"}\n```\n"
	a := nativeHarvest(t, func(path string) (any, int) {
		switch path {
		case "":
			return map[string]int{"id": 42}, 200
		case "/pulls":
			return []map[string]int{{"number": 1}, {"number": 2}, {"number": 3}}, 200
		case "/issues/1/comments":
			return []any{map[string]any{"id": 72, "body": "> ```impl-review\n> {}\n> ```"}, comment, map[string]any{"id": 70, "body": body}}, 200
		case "/issues/2/comments":
			return []any{map[string]any{"id": 81, "body": absence}}, 200
		case "/issues/3/comments":
			return []any{map[string]any{"id": 91, "body": "```impl-review\n{}\n```\n"}}, 200
		case "/issues/comments/71":
			return comment, 200
		default:
			t.Fatalf("unexpected path %s", path)
			return nil, 500
		}
	})
	rows, err := a.Enumerate(t.Context())
	if err != nil || len(rows) != 3 {
		t.Fatalf("rows=%+v err=%v", rows, err)
	}
	if rows[0].Ref != ref || rows[0].Review == nil || *rows[0].Review != harvestReceipt() || !rows[1].Absent || rows[1].AbsentReason != "backend unavailable" || rows[2].Review == nil {
		t.Fatalf("rows=%+v", rows)
	}
	review, err := a.Read(t.Context(), *rows[0].Review)
	if err != nil || review.ReviewID != "review-1" {
		t.Fatalf("review=%+v %v", review, err)
	}
	author, found, err := a.AuthorOf(t.Context(), rows[0].Review.Locator)
	if err != nil || !found || author != "https://forgejo.test/base/operator" {
		t.Fatalf("author=%s found=%v err=%v", author, found, err)
	}
}

func TestForgejoHarvestEnumerationFailsWithoutPartialResults(t *testing.T) {
	for _, tc := range []struct {
		name, fail                  string
		missingPull, missingComment bool
	}{
		{name: "pull outage", fail: "/pulls"}, {name: "repository outage", fail: ""}, {name: "comments outage", fail: "/issues/1/comments"}, {name: "bad pull", missingPull: true}, {name: "bad comment", missingComment: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := nativeHarvest(t, func(path string) (any, int) {
				if !tc.missingPull && !tc.missingComment && path == tc.fail {
					return nil, 503
				}
				switch path {
				case "":
					return map[string]int{"id": 42}, 200
				case "/pulls":
					if tc.missingPull {
						return []map[string]int{{"number": 0}}, 200
					}
					return []map[string]int{{"number": 1}}, 200
				default:
					return []map[string]int{{"id": 0}}, 200
				}
			})
			if rows, err := a.Enumerate(t.Context()); err == nil || rows != nil {
				t.Fatalf("partial=%+v %v", rows, err)
			}
		})
	}
}

func TestForgejoHarvestTrustAndReceiptBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name             string
		code, id, userID int
		author, login    string
		found, wantErr   bool
	}{
		{"native", 200, 71, 5, "", "operator", true, false}, {"imported", 200, 71, 5, "other", "operator", false, false}, {"missing user", 200, 71, 0, "", "operator", false, false}, {"missing login", 200, 71, 5, "", "", false, false}, {"deleted", 404, 71, 5, "", "operator", false, false}, {"outage", 503, 71, 5, "", "operator", false, true}, {"wrong ID", 200, 72, 5, "", "operator", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := nativeHarvest(t, func(path string) (any, int) {
				if path == "" {
					return map[string]int{"id": 42}, 200
				}
				return map[string]any{"id": tc.id, "original_author": tc.author, "user": map[string]any{"id": tc.userID, "login": tc.login}}, tc.code
			})
			_, found, err := a.AuthorOf(t.Context(), harvestReceipt().Locator)
			if found != tc.found || (err != nil) != tc.wantErr {
				t.Fatalf("found=%v err=%v", found, err)
			}
			if _, err := a.Read(t.Context(), persist.Ref{Medium: persist.MediumGitHub}); err == nil {
				t.Fatal("foreign medium accepted")
			}
			if _, _, err := a.AuthorOf(t.Context(), "forgejo-comment:other:71"); err == nil {
				t.Fatal("foreign repository accepted")
			}
			if tc.wantErr {
				if _, err := a.Read(t.Context(), harvestReceipt()); err == nil {
					t.Fatal("bad read accepted")
				}
			}
		})
	}
	a := nativeHarvest(t, func(string) (any, int) { return nil, 503 })
	if _, _, err := a.AuthorOf(t.Context(), harvestReceipt().Locator); err == nil {
		t.Fatal("missing repository accepted")
	}
}

func TestConfiguredHarvestSelectsForgejo(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0700); err != nil {
		t.Fatal(err)
	}
	config := "integrations:\n  schema_version: pyxis-integrations/v1\n  connections:\n    local:\n      provider: forgejo\n      base_url: https://forgejo.test/base\n      token_env: HARVEST_TOKEN\n  bindings:\n    code_host:\n      connection: local\n      repository: o/r\n"
	if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HARVEST_TOKEN", "secret")
	cfg, err := integrations.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := cfg.CodeHost("o/r")
	if err != nil {
		t.Fatal(err)
	}
	a, err := NewForgejoAdapter(binding)
	if err != nil {
		t.Fatal(err)
	}
	if a == nil {
		t.Fatal("expected forgejo harvest adapter")
	}
	for _, binding := range []integrations.ResolvedRepository{{Provider: "github"}, {Provider: "forgejo", BaseURL: "bad"}} {
		if _, err := NewForgejoAdapter(binding); err == nil {
			t.Fatal("invalid binding accepted")
		}
	}
}
