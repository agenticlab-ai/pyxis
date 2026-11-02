package trackeridentity

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

func TestMigrationGuardRejectsActiveLegacyWithoutMutation(t *testing.T) {
	old := migrationOutput
	t.Cleanup(func() { migrationOutput = old })
	claims, pulls := "", "[]"
	var gitErr, ghErr error
	migrationOutput = func(cmd *exec.Cmd) ([]byte, error) {
		if cmd.Args[0] == "git" {
			return []byte(claims), gitErr
		}
		if cmd.Args[0] != "gh" || !strings.Contains(strings.Join(cmd.Args, " "), "--hostname github.com") {
			t.Fatalf("unexpected command: %v", cmd.Args)
		}
		if strings.Contains(strings.Join(cmd.Env, "\n"), "ambient-wrong") || !strings.Contains(strings.Join(cmd.Env, "\n"), "GH_TOKEN=test-token") {
			t.Fatal("migration read did not isolate configured GitHub credentials")
		}
		return []byte(pulls), ghErr
	}
	binding := integrations.ResolvedRepository{Provider: "github", BaseURL: "https://github.com", Repository: "o/r", TokenEnv: "TEST_MIGRATION_TOKEN"}
	t.Setenv("TEST_MIGRATION_TOKEN", "test-token")
	t.Setenv("GH_TOKEN", "ambient-wrong")
	t.Setenv("GITHUB_TOKEN", "ambient-wrong")
	ctx := context.Background()
	if err := CheckLegacy(ctx, "repo", "origin", binding); err != nil {
		t.Fatal(err)
	}
	claims = "sha\trefs/claim/1\n"
	if err := CheckLegacy(ctx, "repo", "origin", binding); err == nil || !strings.Contains(err.Error(), "refs/claim/1") {
		t.Fatal("active legacy claim not refused")
	}
	claims = "sha\trefs/claim/scoped/1\nnoise\n"
	pulls = `[{"number":7,"head":{"ref":"feat/t1-old-work-1"}}]`
	if err := CheckLegacy(ctx, "repo", "origin", binding); err == nil || !strings.Contains(err.Error(), "proposal #7") {
		t.Fatal("active legacy branch not refused")
	}
	pulls = `[{"number":7,"head":{"ref":"feat/scoped/task-1"}},{"head":{"ref":"feature/user"}},{"head":{"ref":"feat/description"}},{"head":{"ref":"feat/description-no"}}][]`
	if err := CheckLegacy(ctx, "repo", "origin", binding); err != nil {
		t.Fatal(err)
	}
	pulls = "{"
	if err := CheckLegacy(ctx, "repo", "origin", binding); err == nil {
		t.Fatal("malformed read became empty")
	}
	ghErr = errors.New("offline")
	if err := CheckLegacy(ctx, "repo", "origin", binding); !errors.Is(err, ghErr) {
		t.Fatal("host failure swallowed")
	}
	gitErr = errors.New("missing remote")
	if err := CheckLegacy(ctx, "repo", "origin", binding); !errors.Is(err, gitErr) {
		t.Fatal("claim failure swallowed")
	}
	gitErr = nil
	for _, invalid := range []integrations.ResolvedRepository{{Provider: "carina"}, {Provider: "github", BaseURL: "invalid"}, {Provider: "forgejo", BaseURL: "invalid"}} {
		if err := CheckLegacy(ctx, "repo", "origin", invalid); err == nil {
			t.Fatal("invalid binding accepted")
		}
	}
	guarded := WrapWithGuard(&fakeTracker{}, binding, func(context.Context) error { return errors.New("legacy active") })
	if _, err := guarded.Frontier(ctx); err == nil {
		t.Fatal("guard did not precede dispatch")
	}
	t.Setenv("TEST_MIGRATION_TOKEN", "")
	if _, err := legacyProposals(ctx, "repo", binding); err == nil {
		t.Fatal("missing selected credential fell back to ambient GitHub auth")
	}
}

func TestMigrationGuardReadsForgejoNativeAPI(t *testing.T) {
	old := migrationOutput
	t.Cleanup(func() { migrationOutput = old })
	migrationOutput = func(cmd *exec.Cmd) ([]byte, error) {
		if cmd.Args[0] != "git" {
			t.Fatalf("unexpected CLI dependency %v", cmd.Args)
		}
		return nil, nil
	}
	status := http.StatusOK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/base/api/v1/repos/o/r/pulls" || r.URL.Query().Get("state") != "open" {
			t.Errorf("wrong endpoint %s", r.URL)
		}
		w.WriteHeader(status)
		body := `[]`
		if r.URL.Query().Get("page") == "1" {
			body = `[{"number":1,"head":{"ref":"feat/scoped/t1-1"}}]`
		}
		if _, err := fmt.Fprint(w, body); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	binding := integrations.ResolvedRepository{Provider: "forgejo", BaseURL: server.URL + "/base", Repository: "o/r"}
	if err := CheckLegacy(context.Background(), "repo", "origin", binding); err != nil {
		t.Fatal(err)
	}
	if key, err := CheckIssueMigration(t.Context(), "repo", "origin", binding, binding, 1492); err != nil || key != binding.Key() {
		t.Fatalf("Forgejo issue compatibility = %q, %v", key, err)
	}
	status = http.StatusForbidden
	if err := CheckLegacy(context.Background(), "repo", "origin", binding); err == nil {
		t.Fatal("unreadable proposals became safe migration")
	}
}
