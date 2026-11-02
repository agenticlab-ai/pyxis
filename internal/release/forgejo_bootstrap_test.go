package release_test

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestForgejoBootstrapAndInstallerAuthenticateAndVerify(t *testing.T) {
	blob, err := os.ReadFile("/usr/bin/false")
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(blob)
	want := hex.EncodeToString(digest[:])
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "token fetch-secret" {
			w.WriteHeader(401)
			return
		}
		_, _ = w.Write(blob)
	}))
	defer s.Close()
	for _, installer := range []bool{false, true} {
		for _, tamper := range []bool{false, true} {
			t.Run(strings.Join([]string{map[bool]string{false: "bootstrap", true: "installer"}[installer], map[bool]string{false: "valid", true: "tampered"}[tamper]}, "/"), func(t *testing.T) {
				dest := t.TempDir()
				script := filepath.Join(root, ".pyxis/scripts/pyxis-bootstrap.sh")
				if installer {
					script = filepath.Join(root, "install.sh")
				}
				pin := want
				if tamper {
					pin = strings.Repeat("0", 64)
				}
				cmd := exec.CommandContext(t.Context(), "bash", script)
				cmd.Dir = dest
				// Do not inherit any operator credential, channel, or install location.
				for _, e := range os.Environ() {
					key, _, _ := strings.Cut(e, "=")
					if strings.HasPrefix(key, "PYXIS_") || strings.HasPrefix(key, "GH_") || strings.HasPrefix(key, "GITHUB_") || key == "XDG_CONFIG_HOME" {
						continue
					}
					cmd.Env = append(cmd.Env, e)
				}
				cmd.Env = append(cmd.Env, "PYXIS_VERSION=v1.2.3", "PYXIS_SHA256="+pin, "PYXIS_SHA256_"+strings.ToUpper(runtime.GOOS+"_"+runtime.GOARCH)+"="+pin, "PYXIS_ARTIFACT_TOKEN=fetch-secret", "PYXIS_RELEASE_PROVIDER=forgejo", "PYXIS_RELEASE_BASE_URL="+s.URL, "PYXIS_RELEASE_REPOSITORY=o/r", "PYXIS_RELEASE_TOKEN_ENV=FETCH_TEST_TOKEN", "FETCH_TEST_TOKEN=fetch-secret", "PYXIS_INSTALL_DIR="+filepath.Join(dest, "installed"), "XDG_CONFIG_HOME="+filepath.Join(dest, "config"))
				if !installer {
					cmd.Env = append(cmd.Env, "PYXIS_ARTIFACT_SOURCE="+s.URL+"/o/r/releases/download/v1.2.3")
				}
				out, err := cmd.CombinedOutput()
				if strings.Contains(string(out), "fetch-secret") {
					t.Fatal("credential leaked to output")
				}
				path := filepath.Join(dest, ".pyxis/bin/pyxis")
				if installer {
					path = filepath.Join(dest, "installed/pyxis")
				}
				if tamper {
					if err == nil {
						t.Fatal("accepted wrong checksum")
					}
					if _, err := os.Stat(path); !os.IsNotExist(err) {
						t.Fatal("installed tampered artifact")
					}
					return
				}
				if err != nil {
					t.Fatalf("fetch: %v %s", err, out)
				}
				installed, err := os.ReadFile(path)
				if err != nil || string(installed) != string(blob) {
					t.Fatalf("installed bytes mismatch: %v", err)
				}
			})
		}
	}
}
