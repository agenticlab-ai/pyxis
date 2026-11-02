package gitcredential

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

func credentialRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0755); err != nil {
		t.Fatal(err)
	}
	config := `integrations:
  schema_version: pyxis-integrations/v1
  connections:
    private:
      provider: forgejo
      base_url: https://forge.example:8443/base
      token_env: TEST_GIT_CREDENTIAL
  bindings:
    code_host:
      connection: private
      repository: owner/code
    tasks:
      connection: private
      repository: owner/tasks
`
	if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestCredentialsMatchOnlySelectedCodeRepository(t *testing.T) {
	root := credentialRoot(t)
	t.Setenv("TEST_GIT_CREDENTIAL", "canary-private-token")
	for _, tc := range []struct {
		name, protocol, host, path, user string
		want                             bool
	}{
		{"selected", "https", "forge.example:8443", "base/owner/code.git", "", true},
		{"extensionless", "https", "forge.example:8443", "base/owner/code", "assistant", true},
		{"other origin", "https", "other.example:8443", "base/owner/code.git", "", false},
		{"other port", "https", "forge.example:8444", "base/owner/code.git", "", false},
		{"downgrade", "http", "forge.example:8443", "base/owner/code.git", "", false},
		{"task binding", "https", "forge.example:8443", "base/owner/tasks.git", "", false},
		{"other base", "https", "forge.example:8443", "other/owner/code.git", "", false},
		{"traversal", "https", "forge.example:8443", "base/owner/../owner/code.git", "", false},
		{"suffix", "https", "forge.example:8443", "base/owner/code.git/extra", "", false},
		{"no path", "https", "forge.example:8443", "", "", false},
		{"different actor", "https", "forge.example:8443", "base/owner/code.git", "other", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := "protocol=" + tc.protocol + "\nhost=" + tc.host + "\npath=" + tc.path + "\nusername=" + tc.user + "\n\n"
			var out bytes.Buffer
			err := serve(context.Background(), root, "get", strings.NewReader(input), &out, func(context.Context, integrations.ResolvedRepository) (string, error) { return "assistant", nil })
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Contains(out.String(), "password=canary-private-token"); got != tc.want {
				t.Fatalf("credential emission %v, want %v", got, tc.want)
			}
			if !tc.want && out.Len() != 0 {
				t.Fatal("unselected request emitted protocol output")
			}
		})
	}
	for _, action := range []string{"store", "erase"} {
		var out bytes.Buffer
		if err := Serve(context.Background(), root, action, strings.NewReader("password=canary-private-token\n\n"), &out); err != nil || out.Len() != 0 {
			t.Fatalf("%s retained a credential: %v", action, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".pyxis", "logs")); !os.IsNotExist(err) {
		t.Fatal("credential helper created logs")
	}
}

func TestCredentialFailuresNeverEmitPartialSecret(t *testing.T) {
	root := credentialRoot(t)
	input := "protocol=https\nhost=forge.example:8443\npath=base/owner/code.git\n\n"
	for _, tc := range []struct {
		token, login string
		actorErr     error
	}{
		{"", "assistant", nil}, {"token\npassword=injected", "assistant", nil}, {"token", "bad\nactor", nil}, {"token", "", errors.New("offline")},
	} {
		t.Setenv("TEST_GIT_CREDENTIAL", tc.token)
		var out bytes.Buffer
		err := serve(context.Background(), root, "get", strings.NewReader(input), &out, func(context.Context, integrations.ResolvedRepository) (string, error) { return tc.login, tc.actorErr })
		if err == nil || out.Len() != 0 {
			t.Fatalf("unsafe output %d bytes, err %v", out.Len(), err)
		}
	}
	for _, input := range []string{"protocol=https\nprotocol=http\n\n", "host\n", strings.Repeat("a", 65537), "host=evil\x00value\n"} {
		if _, err := readRequest(strings.NewReader(input)); err == nil {
			t.Fatal("malformed credential request accepted")
		}
	}
	if err := Serve(context.Background(), root, "unsupported", strings.NewReader(""), io.Discard); err == nil {
		t.Fatal("unsupported operation accepted")
	}
}
