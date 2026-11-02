package host

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

func TestLocalGitWakeRefusesHostedProposalAwaitsWithRetainedConnections(t *testing.T) {
	var outbound atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { outbound.Add(1) }))
	defer server.Close()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0755); err != nil {
		t.Fatal(err)
	}
	profile := fmt.Sprintf(`integrations:
  schema_version: pyxis-integrations/v1
  connections:
    github-release:
      provider: github
    forgejo-release:
      provider: forgejo
      base_url: %s
      token_env: UNUSED_RELEASE_TOKEN
  bindings:
    code_host:
      provider: local-git
      root: .local-origin.git
    tasks:
      provider: carina
`, server.URL)
	if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte(profile), 0644); err != nil {
		t.Fatal(err)
	}
	p := Prober{Root: root, GHRepo: "legacy/repo", Run: func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("local proposal await invoked a hosted command or ancestry fetch")
		return nil, nil
	}}
	for _, ref := range []string{"https://github.com/o/r/pull/7", server.URL + "/o/r/pulls/7", "7"} {
		if ready, err := p.AncestorOfMainline(t.Context(), ref); ready || !errors.Is(err, codehost.ErrWorkflowUnavailable) {
			t.Fatalf("local await %q escaped: ready=%t err=%v", ref, ready, err)
		}
	}
	if outbound.Load() != 0 {
		t.Fatal("retained release connection produced an outbound code-host call")
	}
}
