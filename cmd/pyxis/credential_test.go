package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCredentialProtocolBypassesLogging(t *testing.T) {
	root := t.TempDir()
	old := serveCredential
	t.Cleanup(func() { serveCredential = old })
	serveCredential = func(_ context.Context, gotRoot, action string, _ io.Reader, w io.Writer) error {
		if gotRoot != root || action != "get" {
			t.Fatalf("wrong dispatch %s %s", gotRoot, action)
		}
		_, err := io.WriteString(w, "username=assistant\npassword=credential-canary\n\n")
		return err
	}
	var out, diagnostics bytes.Buffer
	if code := credentialVerb([]string{"--repo", root, "get"}, strings.NewReader(""), &out, &diagnostics); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out.String(), "password=credential-canary") || diagnostics.Len() != 0 {
		t.Fatal("credential did not remain on protocol stdout")
	}
	if _, err := os.Stat(filepath.Join(root, ".pyxis", "logs")); !os.IsNotExist(err) {
		t.Fatal("credential invocation created a log")
	}
	if code := credentialVerb([]string{"get"}, strings.NewReader(""), &bytes.Buffer{}, &diagnostics); code != 2 {
		t.Fatalf("invalid grammar exit %d", code)
	}
}
