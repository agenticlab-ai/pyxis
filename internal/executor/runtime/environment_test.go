package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func TestLaunchEnvironmentReviewDropsRepositoryRedirections(t *testing.T) {
	t.Setenv("PWD", "/operator/clone")
	t.Setenv("OLDPWD", "/implementation/worktree")
	t.Setenv("GIT_DIR", "/operator/clone/.git")
	t.Setenv("GIT_WORK_TREE", "/operator/clone")
	t.Setenv("GH_TOKEN", "operator-secret")
	t.Setenv("GITHUB_TOKEN", "operator-secret-2")
	t.Setenv("SSH_AUTH_SOCK", "/private/operator-agent.sock")
	t.Setenv("HOME", "/Users/operator")
	t.Setenv("PYXIS_REVIEW_EVIDENCE_REF", "refs/pyxis/private")
	t.Setenv("OPENAI_API_KEY", "model-only")
	env := LaunchEnvironment(port.LaunchSpec{Purpose: port.LaunchPurposeReview, Workdir: "/private/review", RuntimeStateDir: "/private/runtime"}, "OPENAI_API_KEY", "CODEX_HOME")
	joined := "\n" + strings.Join(env, "\n") + "\n"
	for _, forbidden := range []string{"\nOLDPWD=", "\nGIT_DIR=", "\nGIT_WORK_TREE=", "\nGH_TOKEN=", "\nGITHUB_TOKEN=", "\nSSH_AUTH_SOCK=", "\nPYXIS_REVIEW_EVIDENCE_REF=", "\nHOME=/Users/operator\n"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("review environment retained %q: %s", forbidden, joined)
		}
	}
	if !strings.Contains(joined, "\nPWD=/private/review\n") {
		t.Fatalf("review environment did not bind PWD to workspace: %s", joined)
	}
	for _, required := range []string{
		"\nHOME=/private/runtime/home\n",
		"\nOPENAI_API_KEY=model-only\n",
		"\nGH_CONFIG_DIR=/private/runtime/no-codehost-credentials\n",
		"\nCODEX_HOME=/private/runtime/model-auth/codex\n",
		"\nGIT_CONFIG_GLOBAL=/dev/null\n",
		"\nGIT_CONFIG_SYSTEM=/dev/null\n",
	} {
		if !strings.Contains(joined, required) {
			t.Fatalf("review environment omitted %q: %s", required, joined)
		}
	}
}

func TestLaunchEnvironmentCursorKeepsSocketStateInsideLongRuntime(t *testing.T) {
	t.Setenv("CURSOR_DATA_DIR", "/operator/cursor-data")
	root := filepath.Join(t.TempDir(), strings.Repeat("long-", 20))
	workdir, state := filepath.Join(root, "review"), filepath.Join(root, "state")
	env := LaunchEnvironment(port.LaunchSpec{Purpose: port.LaunchPurposeReview, Workdir: workdir, RuntimeStateDir: state}, "CURSOR_API_KEY")
	var dataDir string
	for _, item := range env {
		if value, ok := strings.CutPrefix(item, "CURSOR_DATA_DIR="); ok {
			dataDir = value
		}
	}
	if dataDir == "" || filepath.IsAbs(dataDir) || len(dataDir) > 84 {
		t.Fatalf("Cursor socket state would fall back outside the runtime: %q", dataDir)
	}
	if resolved := filepath.Clean(filepath.Join(workdir, dataDir)); resolved != filepath.Join(state, "cursor-data") {
		t.Fatalf("Cursor data directory escaped runtime state: %q", resolved)
	}
}

func TestPrepareReviewAuthenticationCopiesOnlyBackendCredential(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")
	credentialDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(filepath.Join(credentialDir, "projects"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(credentialDir, "auth.json"), []byte(`{"token":"model-only"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(credentialDir, "settings.json"), []byte(`{"hooks":"operator-code"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	state := t.TempDir()
	if err := PrepareReviewAuthentication(t.Context(), state, "codex"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(state, "model-auth", "codex", "auth.json"))
	if err != nil || string(got) != `{"token":"model-only"}` {
		t.Fatalf("credential snapshot = %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(state, "model-auth", "codex", "settings.json")); !os.IsNotExist(err) {
		t.Fatalf("review snapshot exposed settings/hooks: %v", err)
	}
}

func TestPrepareReviewAuthenticationCoversDefaultLoginFiles(t *testing.T) {
	for _, tc := range []struct {
		backend string
		source  string
		target  string
	}{
		{"claude", ".claude/.credentials.json", "model-auth/claude/.credentials.json"},
		{"opencode", ".local/share/opencode/auth.json", "model-auth/xdg-data/opencode/auth.json"},
	} {
		t.Run(tc.backend, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("CLAUDE_CONFIG_DIR", "")
			t.Setenv("XDG_DATA_HOME", "")
			source := filepath.Join(home, filepath.FromSlash(tc.source))
			if err := os.MkdirAll(filepath.Dir(source), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(source, []byte("credential"), 0o600); err != nil {
				t.Fatal(err)
			}
			state := t.TempDir()
			if err := PrepareReviewAuthentication(t.Context(), state, entity.Harness(tc.backend)); err != nil {
				t.Fatal(err)
			}
			if got, err := os.ReadFile(filepath.Join(state, filepath.FromSlash(tc.target))); err != nil || string(got) != "credential" {
				t.Fatalf("credential snapshot = %q, %v", got, err)
			}
		})
	}
}

func TestCursorReviewAuthenticationUsesCredentialStoreWithoutSettings(t *testing.T) {
	for _, tc := range []struct{ goos, source, target string }{
		{"darwin", ".cursor/auth.json", "home/.cursor/auth.json"},
		{"linux", "xdg-config/cursor/auth.json", "xdg/config/cursor/auth.json"},
		{"windows", "appdata/Cursor/auth.json", "home/AppData/Roaming/Cursor/auth.json"},
	} {
		t.Run(tc.goos, func(t *testing.T) {
			home, state := t.TempDir(), t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg-config"))
			t.Setenv("APPDATA", filepath.Join(home, "appdata"))
			t.Setenv("AGENT_CLI_CREDENTIAL_STORE", "file")
			t.Setenv("CURSOR_API_KEY", "")
			t.Setenv("CURSOR_AUTH_TOKEN", "")
			prior := reviewAuthGOOS
			reviewAuthGOOS = tc.goos
			t.Cleanup(func() { reviewAuthGOOS = prior })
			source := filepath.Join(home, filepath.FromSlash(tc.source))
			if err := os.MkdirAll(filepath.Dir(source), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(source, []byte(`{"accessToken":"fixture-token"}`), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(filepath.Dir(source), "cli-config.json"), []byte(`{"permissions":"unsafe-user-settings"}`), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := PrepareReviewAuthentication(t.Context(), state, "cursor"); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(state, filepath.FromSlash(tc.target))
			got, err := os.ReadFile(target)
			if err != nil || string(got) != `{"accessToken":"fixture-token"}` {
				t.Fatalf("snapshot = %q, %v", got, err)
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(target), "cli-config.json")); !os.IsNotExist(err) {
				t.Fatalf("copied Cursor settings: %v", err)
			}
		})
	}
}

func TestCursorReviewSnapshotsKeychainLoginAndBoundsErrors(t *testing.T) {
	t.Setenv("CURSOR_API_KEY", "")
	t.Setenv("CURSOR_AUTH_TOKEN", "")
	t.Setenv("AGENT_CLI_CREDENTIAL_STORE", "")
	priorGOOS, priorRead := reviewAuthGOOS, readCursorKeychainCredential
	reviewAuthGOOS = "darwin"
	t.Cleanup(func() { reviewAuthGOOS, readCursorKeychainCredential = priorGOOS, priorRead })
	readCursorKeychainCredential = func(ctx context.Context, service string) ([]byte, error) {
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("Keychain read has no timeout")
		}
		if service == "api-key" {
			return nil, os.ErrNotExist
		}
		return []byte("fixture-" + service + "\n"), nil
	}
	state := t.TempDir()
	if err := PrepareReviewAuthentication(t.Context(), state, "cursor"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(state, "home", ".cursor", "auth.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var login map[string]string
	if err := json.Unmarshal(data, &login); err != nil {
		t.Fatal(err)
	}
	if len(login) != 2 || login["accessToken"] != "fixture-access-token" || login["refreshToken"] != "fixture-refresh-token" {
		t.Fatalf("snapshot = %v", login)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("snapshot permissions: %v, %v", info, err)
	}
	readCursorKeychainCredential = func(context.Context, string) ([]byte, error) { return nil, errors.New("keychain locked") }
	if err := PrepareReviewAuthentication(t.Context(), t.TempDir(), "cursor"); err == nil || !strings.Contains(err.Error(), "keychain locked") {
		t.Fatalf("Keychain failure = %v", err)
	}
	t.Setenv("CURSOR_API_KEY", "fixture-env-key")
	if err := PrepareReviewAuthentication(t.Context(), t.TempDir(), "cursor"); err != nil {
		t.Fatalf("environment login should not read Keychain: %v", err)
	}
	t.Setenv("CURSOR_API_KEY", "")
	readCursorKeychainCredential = func(context.Context, string) ([]byte, error) { return nil, os.ErrNotExist }
	if err := PrepareReviewAuthentication(t.Context(), t.TempDir(), "cursor"); err != nil {
		t.Fatalf("missing login: %v", err)
	}
	readCursorKeychainCredential = func(ctx context.Context, _ string) ([]byte, error) { return nil, ctx.Err() }
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := PrepareReviewAuthentication(ctx, t.TempDir(), "cursor"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled Keychain read = %v", err)
	}
}

func TestCursorKeychainReaderCancellationAndUnavailableCommand(t *testing.T) {
	// Exercise the actual command boundary without touching an OS credential.
	t.Setenv("PATH", t.TempDir())
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := readCursorKeychainCredential(ctx, "access-token"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled credential read = %v", err)
	}
	if _, err := readCursorKeychainCredential(t.Context(), "access-token"); !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("missing credential tool must remain an infrastructure error: %v", err)
	}
}

func TestCursorReviewEnvironmentUsesIsolatedFileStore(t *testing.T) {
	t.Setenv("AGENT_CLI_CREDENTIAL_STORE", "default")
	t.Setenv("APPDATA", "/operator/appdata")
	t.Setenv("CURSOR_CONFIG_DIR", "/operator/cursor")
	env := LaunchEnvironment(port.LaunchSpec{Purpose: port.LaunchPurposeReview, Workdir: "/review", RuntimeStateDir: "/state"}, "CURSOR_API_KEY", "CURSOR_AUTH_TOKEN")
	joined := strings.Join(env, "\n")
	for _, want := range []string{"AGENT_CLI_CREDENTIAL_STORE=file", "APPDATA=" + filepath.Join("/state", "home", "AppData", "Roaming"), "NODE_DISABLE_COMPILE_CACHE=1"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(joined, "/operator/") {
		t.Fatal("review inherited operator configuration paths")
	}
}

func TestPrepareReviewAuthenticationFollowsCredentialSymlink(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")
	target := filepath.Join(home, "dotfiles", "codex-auth.json")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("credential"), 0o600); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(home, ".codex", "auth.json")
	if err := os.MkdirAll(filepath.Dir(source), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, source); err != nil {
		t.Skipf("credential symlinks unavailable: %v", err)
	}
	state := t.TempDir()
	if err := PrepareReviewAuthentication(t.Context(), state, "codex"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(state, "model-auth", "codex", "auth.json"))
	if err != nil || string(got) != "credential" {
		t.Fatalf("credential snapshot = %q, %v", got, err)
	}
}

func TestPrepareReviewAuthenticationCopiesClaudeKeychainCredential(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	priorGOOS, priorRead := reviewAuthGOOS, readClaudeKeychainCredential
	reviewAuthGOOS = "darwin"
	readClaudeKeychainCredential = func() ([]byte, error) { return []byte(`{"oauth":"keychain"}`), nil }
	t.Cleanup(func() {
		reviewAuthGOOS, readClaudeKeychainCredential = priorGOOS, priorRead
	})
	state := t.TempDir()
	if err := PrepareReviewAuthentication(t.Context(), state, "claude"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(state, "model-auth", "claude", ".credentials.json"))
	if err != nil || string(got) != `{"oauth":"keychain"}` {
		t.Fatalf("keychain credential snapshot = %q, %v", got, err)
	}
}

func TestPrepareReviewAuthenticationRefusesUnsafeSnapshots(t *testing.T) {
	if err := PrepareReviewAuthentication(t.Context(), " ", "codex"); err == nil {
		t.Fatal("empty runtime state directory was accepted")
	}
	if err := PrepareReviewAuthentication(t.Context(), t.TempDir(), "unknown"); err == nil {
		t.Fatal("unsupported backend was accepted")
	}
	if got := firstNonEmpty("", " "); got != "" {
		t.Fatalf("firstNonEmpty(empty) = %q", got)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	auth := filepath.Join(home, ".codex", "auth.json")
	if err := os.MkdirAll(auth, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := PrepareReviewAuthentication(t.Context(), t.TempDir(), "codex"); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("directory credential error = %v", err)
	}
	if err := os.Remove(auth); err != nil {
		t.Fatal(err)
	}
	large, err := os.Create(auth)
	if err != nil {
		t.Fatal(err)
	}
	if err := large.Truncate(maxReviewCredentialBytes + 1); err != nil {
		t.Fatal(err)
	}
	if err := large.Close(); err != nil {
		t.Fatal(err)
	}
	if err := PrepareReviewAuthentication(t.Context(), t.TempDir(), "codex"); err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("oversized credential error = %v", err)
	}

	if err := writeReviewCredential(filepath.Join(t.TempDir(), "credential"), nil); err == nil {
		t.Fatal("empty credential snapshot was accepted")
	}
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeReviewCredential(filepath.Join(blocked, "credential"), []byte("secret")); err == nil {
		t.Fatal("blocked credential parent was accepted")
	}
	existing := filepath.Join(t.TempDir(), "credential")
	if err := os.WriteFile(existing, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeReviewCredential(existing, []byte("new")); err == nil {
		t.Fatal("existing credential was overwritten")
	}

	priorGOOS, priorRead := reviewAuthGOOS, readClaudeKeychainCredential
	reviewAuthGOOS = "darwin"
	readClaudeKeychainCredential = func() ([]byte, error) { return nil, nil }
	t.Cleanup(func() { reviewAuthGOOS, readClaudeKeychainCredential = priorGOOS, priorRead })
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(home, "missing-claude"))
	if err := PrepareReviewAuthentication(t.Context(), t.TempDir(), "claude"); err == nil {
		t.Fatal("empty keychain credential was accepted")
	}
	readClaudeKeychainCredential = func() ([]byte, error) { return nil, errors.New("keychain unavailable") }
	if err := PrepareReviewAuthentication(t.Context(), t.TempDir(), "claude"); err != nil {
		t.Fatalf("missing file remains allowed after keychain fallback: %v", err)
	}
}

func TestLaunchEnvironmentImplementationUsesTaskState(t *testing.T) {
	t.Setenv("GH_TOKEN", "host-only")
	t.Setenv("SSH_AUTH_SOCK", "/host/agent")
	env := strings.Join(LaunchEnvironment(port.LaunchSpec{Purpose: port.LaunchPurposeImplementation, Workdir: "/work", RuntimeStateDir: "/state"}), "\n")
	for _, want := range []string{"HOME=/state/home", "TMPDIR=/state/tmp", "GIT_OPTIONAL_LOCKS=0", "GIT_CONFIG_GLOBAL=/dev/null"} {
		if !strings.Contains(env, want) {
			t.Errorf("missing task control %q", want)
		}
	}
	for _, forbidden := range []string{"host-only", "/host/agent"} {
		if strings.Contains(env, forbidden) {
			t.Errorf("host authority leaked through environment")
		}
	}
}

func TestLaunchEnvironmentPreservesProxyConfiguration(t *testing.T) {
	keys := []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "all_proxy", "no_proxy"}
	for _, key := range keys {
		t.Setenv(key, "proxy-setting-"+key)
	}
	for _, purpose := range []port.LaunchPurpose{port.LaunchPurposeImplementation, port.LaunchPurposeReview} {
		env := "\n" + strings.Join(LaunchEnvironment(port.LaunchSpec{Purpose: purpose, Workdir: "/work", RuntimeStateDir: "/state"}), "\n") + "\n"
		for _, key := range keys {
			if !strings.Contains(env, "\n"+key+"=proxy-setting-"+key+"\n") {
				t.Errorf("%s dropped %s", purpose, key)
			}
		}
	}
}
