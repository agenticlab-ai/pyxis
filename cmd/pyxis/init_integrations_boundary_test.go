package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/installer"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

func TestInitIntegrationProfileRefusesBeforeInstaller(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{name: "missing file", want: "no such file"},
		{name: "unknown schema", body: "integrations:\n  schema_version: unknown\n", want: "schema_version"},
		{name: "missing integrations block", body: "tracker: carina\n", want: "must contain an integrations block"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			t.Chdir(root)
			denySeams(t)
			profile := filepath.Join(t.TempDir(), "integrations.yaml")
			if tc.body != "" {
				writeFile(t, profile, []byte(tc.body))
			}
			var out, diagnostic bytes.Buffer
			code := run([]string{"init", "--preview", "--integrations", profile, "--system-version", "v1.2.3", "--manifest-sha256", strings.Repeat("ab", 32)}, &out, &diagnostic)
			if code == 0 || out.Len() != 0 || !strings.Contains(diagnostic.String(), tc.want) {
				t.Fatalf("code=%d out=%q diagnostic=%q", code, &out, &diagnostic)
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 0 {
				t.Fatalf("refused preview changed target: entries=%v err=%v", entries, err)
			}
		})
	}
}

func TestInitIntegrationProfileCarriesIndependentBindingsWithoutPrompts(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	profile := filepath.Join(t.TempDir(), "integrations.yaml")
	writeFile(t, profile, []byte(`integrations:
  schema_version: pyxis-integrations/v1
  connections:
    forge:
      provider: forgejo
      base_url: https://forgejo.test/base
      token_env: INIT_TEST_FORGEJO_TOKEN
    github:
      provider: github
      token_env: INIT_TEST_GITHUB_TOKEN
  bindings:
    tasks:
      connection: forge
      repository: owner/tasks
    code_host:
      connection: github
      repository: owner/code
  ci:
    mode: actions
`))
	rec := &initRec{rep: installer.Report{Preview: true, Branch: "main"}}
	swap(t, &installerInit, rec.call)
	var out, diagnostic bytes.Buffer
	code := run([]string{"init", "--preview", "--integrations", profile, "--system-version", "v1.2.3", "--manifest-sha256", strings.Repeat("ab", 32)}, &out, &diagnostic)
	if code != 0 || rec.calls != 1 || !rec.opts.AnswersProvided || rec.opts.Stdin != nil || rec.opts.AcceptDefaults {
		t.Fatalf("profile not passed promptlessly: code=%d calls=%d opts=%+v diagnostic=%s", code, rec.calls, rec.opts, &diagnostic)
	}
	tasks, err := rec.opts.Integrations.Tasks("legacy/repo", "github", root)
	if err != nil || tasks.Provider != integrations.Forgejo || tasks.Repository != "owner/tasks" || tasks.BaseURL != "https://forgejo.test/base" || tasks.TokenEnv != "INIT_TEST_FORGEJO_TOKEN" {
		t.Fatalf("task binding=%+v err=%v", tasks, err)
	}
	host, err := rec.opts.Integrations.CodeHost("legacy/repo")
	if err != nil || host.Provider != integrations.GitHub || host.Repository != "owner/code" || host.TokenEnv != "INIT_TEST_GITHUB_TOKEN" || rec.opts.Integrations.CI.Mode != "actions" {
		t.Fatalf("code binding=%+v ci=%+v err=%v", host, rec.opts.Integrations.CI, err)
	}
}
