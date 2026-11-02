package upgrade

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

type coverageRunner struct{ err error }

func (r coverageRunner) Run(context.Context, string, ...string) (string, error) {
	return "", r.err
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

type errorReadCloser struct{}

func (errorReadCloser) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func (errorReadCloser) Close() error             { return nil }

func TestUpgradeHelperFailureBoundaries(t *testing.T) {
	digest := strings.Repeat("a", 64)
	if err := checkExplicitFlags(Options{SHA256: digest}); err == nil {
		t.Fatal("digest without target was accepted")
	}
	if err := checkExplicitFlags(Options{ManifestSHA256: digest}); err == nil {
		t.Fatal("manifest digest without target was accepted")
	}

	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, []byte("blocked"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := reconcileReleaseEnv(blocked, "v1.2.3", digest, ""); err == nil {
		t.Fatal("blocked release-env directory was accepted")
	}
	if _, err := writeReleaseEnv(blocked, "v1.2.3", digest, ""); err == nil {
		t.Fatal("release pin was written through a regular file")
	}

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "release.env"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := writeReleaseEnv(dir, "v1.2.3", digest, ""); err == nil {
		t.Fatal("release pin replaced a directory")
	}

	targetDir := filepath.Join(t.TempDir(), "target-dir")
	if err := os.Mkdir(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := replaceExecutable(targetDir, []byte("binary")); err == nil {
		t.Fatal("executable replacement accepted a directory target")
	}
}

func TestUpgradeDiscoveryAndHTTPFailureBoundaries(t *testing.T) {
	wantErr := errors.New("runner failed")
	private := integrations.ResolvedRepository{Provider: integrations.GitHub, BaseURL: "https://github.com", Repository: "owner/repo", TokenEnv: "PRIVATE_TOKEN"}
	if _, err := discover(t.Context(), Options{Repo: "owner/repo", Release: &private, Runner: coverageRunner{err: wantErr}}); !errors.Is(err, wantErr) {
		t.Fatalf("discover error = %v", err)
	}
	if _, err := channelGet(t.Context(), Options{}, "http://[::1"); err == nil {
		t.Fatal("malformed channel URL was accepted")
	}

	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("transport failed")
	})}
	if _, err := channelGet(t.Context(), Options{HTTP: client}, "https://example.invalid/value"); err == nil {
		t.Fatal("transport failure was hidden")
	}
	client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: errorReadCloser{}}, nil
	})}
	if _, err := channelGet(t.Context(), Options{HTTP: client}, "https://example.invalid/value"); err == nil {
		t.Fatal("response read failure was hidden")
	}
	if err := assertTLSChannel("https://example.invalid/%zz"); err == nil {
		t.Fatal("malformed TLS channel was accepted")
	}
}

func TestUpgradeProductionAndPureHelpers(t *testing.T) {
	if got, err := resolveTargetPath(""); err != nil || got == "" {
		t.Fatalf("resolve running executable = %q, %v", got, err)
	}
	if got := Platform(); !strings.Contains(got, "-") {
		t.Fatalf("Platform() = %q", got)
	}

	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	if got, err := releaseEnvDir(""); err != nil || got != filepath.Join(configHome, "pyxis") {
		t.Fatalf("releaseEnvDir = %q, %v", got, err)
	}

	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".pyxis", "config.yaml"), []byte("unknown: value\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := RepoPin(repo); got != "" {
		t.Fatalf("RepoPin malformed config = %q", got)
	}

	runner := execRunner{}
	if out, err := runner.Run(t.Context(), "/bin/echo", "hello"); err != nil || strings.TrimSpace(out) != "hello" {
		t.Fatalf("execRunner success = %q, %v", out, err)
	}
	if _, err := runner.Run(t.Context(), "/bin/sh", "-c", "printf 'first\\nsecond\\n' >&2; exit 7"); err == nil || !strings.Contains(err.Error(), "first") || strings.Contains(err.Error(), "second") {
		t.Fatalf("execRunner stderr failure = %v", err)
	}
	if _, err := runner.Run(t.Context(), filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("execRunner missing command succeeded")
	}
	if got := firstLine("one line"); got != "one line" {
		t.Fatalf("firstLine = %q", got)
	}
	if got := sortedKeys(map[string]string{"z": "", "a": "", "m": ""}); !reflect.DeepEqual(got, []string{"a", "m", "z"}) {
		t.Fatalf("sortedKeys = %v", got)
	}

	// Keep io imported through the exact interface channelGet consumes.
	var _ io.ReadCloser = errorReadCloser{}
}

func TestCoverage90RemainingPureBranches(t *testing.T) {
	digest := strings.Repeat("a", 64)
	install := "PINNED_VERSION=\"v1.2.3\"\nMANIFEST_SHA256=\"" + digest + "\"\nsha_linux_amd64=\"" + digest + "\"\nsha_broken=\"" + digest + "\"\n"
	if _, err := parsePins(install); err != nil {
		t.Fatal(err)
	}
	if err := checkExplicitFlags(Options{To: "v1.2.3", SHA256: digest, ManifestSHA256: "bad"}); err == nil {
		t.Fatal("malformed manifest digest was accepted")
	}
	keys := []string{"z", "a"}
	sortStrings(keys)
	if !reflect.DeepEqual(keys, []string{"a", "z"}) {
		t.Fatalf("sortStrings = %v", keys)
	}
	if _, ok := parseRelease("1.2.3.4"); ok {
		t.Fatal("release with too many numeric components was accepted")
	}
	prior, hadPrior := os.LookupEnv("XDG_CONFIG_HOME")
	if err := os.Unsetenv("XDG_CONFIG_HOME"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if hadPrior {
			_ = os.Setenv("XDG_CONFIG_HOME", prior)
		}
	})
	if dir, err := releaseEnvDir(""); err != nil || !strings.HasSuffix(dir, filepath.Join(".config", "pyxis")) {
		t.Fatalf("default releaseEnvDir = %q, %v", dir, err)
	}
}
