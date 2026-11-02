package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunRequiresRepositoryAndOperator(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(nil, &stdout, &stderr); code != 2 {
		t.Fatalf("run exit=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "--repo-slug") || !strings.Contains(stderr.String(), "--operator") {
		t.Fatalf("usage error omitted required identities: %q", stderr.String())
	}
}

func TestRunValidatesEveryInputBoundary(t *testing.T) {
	for _, args := range [][]string{
		{"--repo-slug", "owner/repo", "--operator", "operator", "--disposition-floor", "0"},
		{"--repo-slug", "owner/repo", "--operator", "operator", "extra"},
		{"--repo-slug", "malformed", "--operator", "operator"},
		{"--unknown"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 2 || stderr.Len() == 0 {
			t.Fatalf("run(%v) code=%d stderr=%q", args, code, stderr.String())
		}
	}
}

func TestRunEmitsEmptyUniverseAndSurfacesEncoderFailure(t *testing.T) {
	dir := t.TempDir()
	gh := filepath.Join(dir, "gh")
	if err := os.WriteFile(gh, []byte("#!/bin/sh\nprintf '[[]]'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	args := []string{"--repo-slug", "owner/repo", "--operator", "operator", "--disposition-floor", "5"}
	var stdout, stderr bytes.Buffer
	if code := run(args, &stdout, &stderr); code != 0 {
		t.Fatalf("run code=%d stderr=%q", code, stderr.String())
	}
	for _, want := range []string{`"universe": 0`, `"coverage": 0`, `"precision_ready": false`, `"misses": []`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("output %q omitted %q", stdout.String(), want)
		}
	}
	if code := run(args, failingWriter{}, &stderr); code != 1 || !strings.Contains(stderr.String(), "encode output") {
		t.Fatalf("encoder failure code=%d stderr=%q", code, stderr.String())
	}
}

func TestRunSurfacesHarvestFailure(t *testing.T) {
	dir := t.TempDir()
	gh := filepath.Join(dir, "gh")
	if err := os.WriteFile(gh, []byte("#!/bin/sh\nprintf 'not-json'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	var stdout, stderr bytes.Buffer
	code := run([]string{"--repo-slug", "owner/repo", "--operator", "operator"}, &stdout, &stderr)
	if code != 1 || stderr.Len() == 0 {
		t.Fatalf("harvest failure code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }
