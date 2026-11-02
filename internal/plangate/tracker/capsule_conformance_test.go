package tracker

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executiondemand/conformancetest"
)

// capsuleGitHub records the body passed through the production GH adapter's
// CreateIssue argv and serves it back through the production OpenIssues JSON
// decoder. The fake replaces only the network/gh process; body construction,
// argument transport, response decoding, and Issue mapping are the real
// adapter boundary exercised in production.
type capsuleGitHub struct {
	body  string
	title string
}

func (f *capsuleGitHub) run(args ...string) ([]byte, error) {
	endpoint := ""
	for _, arg := range args {
		if strings.HasPrefix(arg, "repos/") {
			endpoint = arg
			break
		}
	}
	switch {
	case endpoint == "repos/o/r/issues":
		f.title = formValue(args, "title")
		f.body = formValue(args, "body")
		return json.Marshal(map[string]any{
			"number": 41,
			"title":  f.title,
			"body":   f.body,
			"state":  "open",
		})
	case strings.HasPrefix(endpoint, "repos/o/r/issues?state=open"):
		return json.Marshal([]map[string]any{{
			"number": 41,
			"title":  f.title,
			"body":   f.body,
			"state":  "open",
			"labels": []map[string]string{{"name": "local-agent"}},
		}})
	default:
		return nil, fmt.Errorf("unexpected GitHub adapter call: %v", args)
	}
}

func formValue(args []string, key string) string {
	prefix := key + "="
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "-f" && strings.HasPrefix(args[i+1], prefix) {
			return strings.TrimPrefix(args[i+1], prefix)
		}
	}
	return ""
}

func githubAdapterRoundTrip(t *testing.T, body string) string {
	t.Helper()
	server := &capsuleGitHub{}
	adapter := &GH{Repo: "o/r", run: server.run}
	if _, err := adapter.CreateIssue("[T1] capsule conformance", body, []string{"local-agent"}, ""); err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	issues, err := adapter.OpenIssues()
	if err != nil {
		t.Fatalf("OpenIssues: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("OpenIssues returned %d issues, want 1", len(issues))
	}
	return issues[0].Body
}

// TestGitHubAdapterRunsSharedCapsuleConformance supplies the actual GitHub
// CreateIssue/OpenIssues boundary to the same closed harness the local
// production adapter runs. Future origin and tamper cases are added once in
// conformancetest and automatically bind both media.
func TestGitHubAdapterRunsSharedCapsuleConformance(t *testing.T) {
	conformancetest.Run(t, "github", githubAdapterRoundTrip)
}
