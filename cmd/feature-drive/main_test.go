package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
)

func TestDogfoodCommandRunsLocalAndRemoteGitJourneys(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "dogfood")
	var stdout, stderr bytes.Buffer
	if err := run(context.Background(), []string{"dogfood", "--workspace", workspace}, &stdout, &stderr); err != nil {
		t.Fatalf("run: %v\nstderr: %s", err, stderr.String())
	}
	var report featuredrive.ExecutableDogfoodReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if err := report.Validate(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestDogfoodCommandRequiresWorkspace(t *testing.T) {
	if err := run(context.Background(), []string{"dogfood"}, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Fatal("expected missing workspace to fail")
	}
}
