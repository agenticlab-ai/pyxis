package checks

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/verify/diffset"
	"github.com/agenticlab-ai/pyxis/internal/verify/ecosystem"
	"github.com/agenticlab-ai/pyxis/internal/verify/registry"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

func TestForbiddenArtifacts_RegisteredOnRung0(t *testing.T) {
	check := registry.GetCheck(ForbiddenArtifactsCheckID)
	if check == nil {
		t.Fatal("forbidden-artifacts is not registered")
	}
	if check.Rung != "0" {
		t.Errorf("rung = %q, want 0", check.Rung)
	}
	if !check.AuthorIndependent {
		t.Error("forbidden-artifacts must be author-independent: it runs no repo-supplied verb")
	}
	if len(check.VerbDeps) != 0 {
		t.Errorf("VerbDeps = %v, want none (verbless check)", check.VerbDeps)
	}
}

// TestForbiddenArtifacts_CommittedBinary is T4's acceptance criterion 2's
// binary leg, end to end through real git binary detection.
func TestForbiddenArtifacts_CommittedBinary(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "README.md", "text\n")
	base := commit(t, dir, "base")

	binPath := filepath.Join(dir, "blob.bin")
	if err := os.WriteFile(binPath, []byte{0x00, 0x01, 0x02, 'P', 'N', 'G', 0x00, 0xFF}, 0o644); err != nil {
		t.Fatal(err)
	}
	head := commit(t, dir, "add binary")

	ds, err := diffset.Extract(context.Background(), dir, base, head)
	if err != nil {
		t.Fatalf("diffset.Extract: %v", err)
	}

	env := &Env{Manifest: &schema.Manifest{Version: "1", Stack: "go"}, Ecosystem: ecosystem.New(), Diff: ds}
	result, findings, err := runForbiddenArtifacts(context.Background(), env)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.State != "fail" {
		t.Fatalf("state = %q, want fail", result.State)
	}
	found := false
	for _, f := range findings {
		if f.Class == "forbidden-binary" && f.Path == "blob.bin" {
			found = true
		}
	}
	if !found {
		t.Errorf("findings = %+v, want a forbidden-binary finding on blob.bin", findings)
	}
}

// TestForbiddenArtifacts_BakedAbsolutePath is acceptance criterion 2's
// absolute-path leg.
func TestForbiddenArtifacts_BakedAbsolutePath(t *testing.T) {
	ds := &diffset.DiffSet{Files: []diffset.FileChange{
		{Status: diffset.StatusAdded, Path: "setup.sh", AddedLines: []diffset.AddedLine{
			{Line: 1, Content: "#!/bin/bash"},
			{Line: 2, Content: "cp config.yaml /Users/agent/.secrets/config.yaml"},
		}},
	}}
	env := &Env{Manifest: &schema.Manifest{Version: "1", Stack: "go"}, Ecosystem: ecosystem.New(), Diff: ds}
	result, findings, err := runForbiddenArtifacts(context.Background(), env)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.State != "fail" {
		t.Fatalf("state = %q, want fail", result.State)
	}
	if len(findings) != 1 || findings[0].Class != "forbidden-path" || findings[0].Line != 2 {
		t.Errorf("findings = %+v, want one forbidden-path finding at line 2", findings)
	}
}

// TestForbiddenArtifacts_SizeThresholdManifestOverride proves the manifest
// override (a lowered ceiling) is honored, and exercises the size-threshold
// class without needing a genuinely oversized fixture file.
func TestForbiddenArtifacts_AbsolutePathInStringLiteral(t *testing.T) {
	// Baked machine-specific absolute paths as they actually appear in source —
	// inside a quoted string or after '=' — which the whitespace-anchored,
	// 5-prefix floor previously missed.
	for _, line := range []string{
		`f, _ := os.Open("/Users/agent/.ssh/id_rsa")`,
		`dir = "/tmp/stash"`,
		`data, _ := os.ReadFile("/etc/shadow")`,
	} {
		ds := &diffset.DiffSet{Files: []diffset.FileChange{
			{Status: diffset.StatusAdded, Path: "main.go", AddedLines: []diffset.AddedLine{{Line: 1, Content: line}}},
		}}
		env := &Env{Manifest: &schema.Manifest{Version: "1", Stack: "go"}, Ecosystem: ecosystem.New(), Diff: ds}
		result, _, err := runForbiddenArtifacts(context.Background(), env)
		if err != nil {
			t.Fatalf("Run(%q): %v", line, err)
		}
		if result.State != "fail" {
			t.Errorf("line %q: state = %q, want fail (baked absolute path must be caught)", line, result.State)
		}
	}
}

func TestForbiddenArtifacts_SizeFloorOnlyOnAddedFiles(t *testing.T) {
	manifest := &schema.Manifest{Version: "1", Stack: "go", ForbiddenArtifacts: &schema.ForbiddenArtifactsConfig{MaxFileBytes: 1000}}

	// A one-line edit to a pre-existing large file must NOT red — the bulk the
	// diff never introduced is not a forbidden artifact (R1 red-blindness).
	mod := &diffset.DiffSet{Files: []diffset.FileChange{
		{Status: diffset.StatusModified, Path: "go.sum", SizeBytes: 5000, AddedLines: []diffset.AddedLine{{Line: 1, Content: "one new dependency line"}}},
	}}
	env := &Env{Manifest: manifest, Ecosystem: ecosystem.New(), Diff: mod}
	result, _, err := runForbiddenArtifacts(context.Background(), env)
	if err != nil {
		t.Fatalf("Run (modify): %v", err)
	}
	if result.State != "pass" {
		t.Errorf("a one-line edit to a pre-existing large file must not red forbidden-size, got %q", result.State)
	}

	// A newly-ADDED oversized file still reds.
	add := &diffset.DiffSet{Files: []diffset.FileChange{
		{Status: diffset.StatusAdded, Path: "data/dump.json", SizeBytes: 5000},
	}}
	env = &Env{Manifest: manifest, Ecosystem: ecosystem.New(), Diff: add}
	result, _, err = runForbiddenArtifacts(context.Background(), env)
	if err != nil {
		t.Fatalf("Run (add): %v", err)
	}
	if result.State != "fail" {
		t.Errorf("a newly-added oversized file must red, got %q", result.State)
	}
}

func TestForbiddenArtifacts_SizeThresholdManifestOverride(t *testing.T) {
	ds := &diffset.DiffSet{Files: []diffset.FileChange{
		{Status: diffset.StatusAdded, Path: "data/dump.json", SizeBytes: 5000},
	}}
	manifest := &schema.Manifest{
		Version: "1", Stack: "go",
		ForbiddenArtifacts: &schema.ForbiddenArtifactsConfig{MaxFileBytes: 1000},
	}
	env := &Env{Manifest: manifest, Ecosystem: ecosystem.New(), Diff: ds}
	result, findings, err := runForbiddenArtifacts(context.Background(), env)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.State != "fail" {
		t.Fatalf("state = %q, want fail", result.State)
	}
	if len(findings) != 1 || findings[0].Class != "forbidden-size" {
		t.Errorf("findings = %+v, want one forbidden-size finding", findings)
	}

	// The ecosystem default ceiling (1 MiB) would NOT have flagged 5000 bytes —
	// confirms the override, not the default, drove the finding.
	defaultEnv := &Env{Manifest: &schema.Manifest{Version: "1", Stack: "go"}, Ecosystem: ecosystem.New(), Diff: ds}
	defaultResult, _, err := runForbiddenArtifacts(context.Background(), defaultEnv)
	if err != nil {
		t.Fatalf("Run (default ceiling): %v", err)
	}
	if defaultResult.State != "pass" {
		t.Errorf("state with the default ceiling = %q, want pass", defaultResult.State)
	}
}

// TestForbiddenArtifacts_PureRenameNotFlagged proves the ContentIntroduced
// gate suppresses ALL three floors for a pure rename — even one moving an
// oversized file — since diff-scoped hygiene has nothing new to say about
// content that already existed before the diff.
func TestForbiddenArtifacts_PureRenameNotFlagged(t *testing.T) {
	ds := &diffset.DiffSet{Files: []diffset.FileChange{
		{Status: diffset.StatusRenamed, OldPath: "old/huge.bin", Path: "new/huge.bin", SizeBytes: 5_000_000},
	}}
	env := &Env{Manifest: &schema.Manifest{Version: "1", Stack: "go"}, Ecosystem: ecosystem.New(), Diff: ds}
	result, findings, err := runForbiddenArtifacts(context.Background(), env)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.State != "pass" || len(findings) != 0 {
		t.Errorf("pure rename produced findings: state=%q findings=%+v", result.State, findings)
	}
}

func TestForbiddenArtifacts_DeletionNotFlagged(t *testing.T) {
	ds := &diffset.DiffSet{Files: []diffset.FileChange{
		{Status: diffset.StatusDeleted, Path: "old/secret.bin"},
	}}
	env := &Env{Manifest: &schema.Manifest{Version: "1", Stack: "go"}, Ecosystem: ecosystem.New(), Diff: ds}
	result, findings, err := runForbiddenArtifacts(context.Background(), env)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.State != "pass" || len(findings) != 0 {
		t.Errorf("deletion produced findings: state=%q findings=%+v", result.State, findings)
	}
}

func TestForbiddenArtifacts_CleanDiffPasses(t *testing.T) {
	ds := &diffset.DiffSet{Files: []diffset.FileChange{
		{Status: diffset.StatusAdded, Path: "main.go", SizeBytes: 200, AddedLines: []diffset.AddedLine{
			{Line: 1, Content: "package main"},
		}},
	}}
	env := &Env{Manifest: &schema.Manifest{Version: "1", Stack: "go"}, Ecosystem: ecosystem.New(), Diff: ds}
	result, findings, err := runForbiddenArtifacts(context.Background(), env)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.State != "pass" || len(findings) != 0 {
		t.Errorf("clean diff produced findings: state=%q findings=%+v", result.State, findings)
	}
}

func TestForbiddenArtifacts_NilDiffErrors(t *testing.T) {
	env := &Env{Manifest: &schema.Manifest{Version: "1", Stack: "go"}, Ecosystem: ecosystem.New()}
	if _, _, err := runForbiddenArtifacts(context.Background(), env); err == nil {
		t.Error("expected an error when no diff set is provided")
	}
}
