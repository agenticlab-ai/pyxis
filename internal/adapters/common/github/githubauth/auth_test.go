package githubauth

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEnvironmentRequiresSelectedCredentialAndPreservesParent(t *testing.T) {
	t.Setenv("TEST_GITHUB_ACCOUNT", "selected")
	for _, key := range []string{"GH_TOKEN", "GITHUB_TOKEN", "GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN", "GH_HOST"} {
		t.Setenv(key, "ambient")
	}
	env, err := Environment("TEST_GITHUB_ACCOUNT")
	if err != nil {
		t.Fatal(err)
	}
	values := make(map[string]string)
	for _, entry := range env {
		k, v, _ := strings.Cut(entry, "=")
		values[k] = v
	}
	if values["GH_TOKEN"] != "selected" || values["GH_HOST"] != "github.com" {
		t.Fatalf("selected environment was not routed")
	}
	for _, key := range []string{"GITHUB_TOKEN", "GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN"} {
		if _, ok := values[key]; ok {
			t.Fatalf("inherited %s", key)
		}
	}
	if os.Getenv("GH_TOKEN") != "ambient" {
		t.Fatal("parent credential changed")
	}
	t.Setenv("TEST_GITHUB_ACCOUNT", "")
	if _, err := Environment("TEST_GITHUB_ACCOUNT"); err == nil {
		t.Fatal("used ambient auth when selected token absent")
	}
	if env, err := Environment(""); err != nil || env != nil {
		t.Fatal("legacy environment changed")
	}
}

func TestRunUsesStdinRedactsCredentialAndHonorsCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX gh executable fixture")
	}
	dir := t.TempDir()
	stub := "#!/bin/sh\nif [ \"$1\" = fail ]; then printf 'denied %s (HTTP 403)' \"$GH_TOKEN\" >&2; exit 1; fi\nIFS= read -r body\nprintf '%s|%s|%s' \"$GH_HOST\" \"$GH_TOKEN\" \"$body\"\n"
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("TEST_GITHUB_ACCOUNT", "selected-secret")
	out, err := Run(context.Background(), "TEST_GITHUB_ACCOUNT", []byte("body\n"), "api")
	if err != nil || string(out) != "github.com|selected-secret|body" {
		t.Fatalf("transport result %q %v", out, err)
	}
	_, err = Run(context.Background(), "TEST_GITHUB_ACCOUNT", nil, "fail")
	if err == nil || strings.Contains(err.Error(), "selected-secret") || !strings.Contains(err.Error(), "403") {
		t.Fatalf("error redaction/classification %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Run(ctx, "TEST_GITHUB_ACCOUNT", nil, "api"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation %v", err)
	}
	t.Setenv("TEST_GITHUB_ACCOUNT", "")
	if _, err := Run(context.Background(), "TEST_GITHUB_ACCOUNT", nil, "api"); err == nil {
		t.Fatal("absent credential accepted")
	}
}
