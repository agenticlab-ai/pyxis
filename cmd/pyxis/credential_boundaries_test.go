package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCredentialRejectsInvalidGrammarBeforeServing(t *testing.T) {
	old := serveCredential
	t.Cleanup(func() { serveCredential = old })
	serveCredential = func(context.Context, string, string, io.Reader, io.Writer) error {
		t.Fatal("invalid grammar reached credential serving")
		return nil
	}
	for _, args := range [][]string{
		{"--unknown"}, {"--repo"}, {"get"}, {"--repo", "repo"}, {"--repo", "repo", "get", "extra"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var out, diagnostics bytes.Buffer
			if code := credentialVerb(args, strings.NewReader(""), &out, &diagnostics); code != 2 || out.Len() != 0 || diagnostics.Len() == 0 {
				t.Fatalf("invalid invocation: code=%d stdout=%q stderr=%q", code, &out, &diagnostics)
			}
		})
	}
}

func TestCredentialConfigureNoopAndFailureBoundaries(t *testing.T) {
	for _, scenario := range []string{"legacy no-op", "invalid profile", "output failure"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			if scenario == "invalid profile" {
				if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte("integrations:\n  schema_version: unknown\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, diagnostics bytes.Buffer
			var sink io.Writer = &out
			if scenario == "output failure" {
				sink = credentialFailedWriter{}
			}
			code := credentialVerb([]string{"--repo", root, "configure"}, strings.NewReader(""), sink, &diagnostics)
			switch scenario {
			case "legacy no-op":
				if code != 0 || out.String() != "configured 0 repository-scoped Git credential settings\n" || diagnostics.Len() != 0 {
					t.Fatalf("no-op: code=%d stdout=%q stderr=%q", code, &out, &diagnostics)
				}
				if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
					t.Fatalf("legacy configure mutated repository: %v %v", entries, err)
				}
			case "invalid profile":
				if code != 1 || out.Len() != 0 || !strings.Contains(diagnostics.String(), "schema_version") {
					t.Fatalf("invalid profile: code=%d stdout=%q stderr=%q", code, &out, &diagnostics)
				}
			case "output failure":
				if code != 1 {
					t.Fatalf("output failure reported success: %d", code)
				}
			}
			if _, err := os.Stat(filepath.Join(root, ".git")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("credential configure initialized Git: %v", err)
			}
		})
	}
}

func TestCredentialServingFailureAndBrokenProtocolOutput(t *testing.T) {
	old := serveCredential
	t.Cleanup(func() { serveCredential = old })
	for _, brokenOutput := range []bool{false, true} {
		root := t.TempDir()
		serveCredential = func(ctx context.Context, gotRoot, action string, input io.Reader, output io.Writer) error {
			deadline, bounded := ctx.Deadline()
			if !bounded || time.Until(deadline) <= 0 || time.Until(deadline) > 30*time.Second || gotRoot != root || action != "get" {
				t.Fatal("credential serving lost its root, action, or bounded context")
			}
			if data, err := io.ReadAll(input); err != nil || string(data) != "protocol=https\n" {
				t.Fatalf("protocol input: %q %v", data, err)
			}
			if brokenOutput {
				_, err := io.WriteString(output, "password=synthetic-canary\n\n")
				return err
			}
			return errors.New("selected credential is unavailable")
		}
		var out, diagnostics bytes.Buffer
		var sink io.Writer = &out
		if brokenOutput {
			sink = credentialFailedWriter{}
		}
		if code := credentialVerb([]string{"--repo", root, "get"}, strings.NewReader("protocol=https\n"), sink, &diagnostics); code != 1 {
			t.Fatalf("credential failure exit=%d", code)
		}
		if out.Len() != 0 || diagnostics.Len() == 0 || strings.Contains(diagnostics.String(), "synthetic-canary") {
			t.Fatalf("failure leaked protocol data: stdout=%q stderr=%q", &out, &diagnostics)
		}
	}
}

type credentialFailedWriter struct{}

func (credentialFailedWriter) Write([]byte) (int, error) {
	return 0, errors.New("protocol output unavailable")
}
