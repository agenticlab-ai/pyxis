package checks

import (
	"context"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/verify/diffset"
	"github.com/agenticlab-ai/pyxis/internal/verify/ecosystem"
	"github.com/agenticlab-ai/pyxis/internal/verify/registry"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

func TestSecretScan_RegisteredOnRung0(t *testing.T) {
	check := registry.GetCheck(SecretScanCheckID)
	if check == nil {
		t.Fatal("secret-scan is not registered")
	}
	if check.Rung != "0" {
		t.Errorf("rung = %q, want 0", check.Rung)
	}
	if !check.AuthorIndependent {
		t.Error("secret-scan must be author-independent: it runs no repo-supplied verb")
	}
	if len(check.VerbDeps) != 0 {
		t.Errorf("VerbDeps = %v, want none (verbless check)", check.VerbDeps)
	}
}

// TestSecretScan_SeededSecretInDiff is T4's acceptance criterion 1's positive
// leg: a seeded secret riding the proposal diff reds rung 0 with a
// class-named finding, end to end through real git extraction.
func TestSecretScan_SeededSecretInDiff(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "README.md", "hello\n")
	base := commit(t, dir, "base")

	writeFile(t, dir, "config.py", "DB_PASSWORD = 'irrelevant'\napi_key: sk-live-abcdef0123456789\n")
	head := commit(t, dir, "add config with a seeded secret")

	ds, err := diffset.Extract(context.Background(), dir, base, head)
	if err != nil {
		t.Fatalf("diffset.Extract: %v", err)
	}

	env := &Env{Manifest: &schema.Manifest{Version: "1", Stack: "go"}, Ecosystem: ecosystem.New(), Diff: ds}
	result, findings, err := runSecretScan(context.Background(), env)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.State != "fail" {
		t.Fatalf("state = %q, want fail", result.State)
	}
	if len(findings) == 0 {
		t.Fatal("expected at least one finding for the seeded secret")
	}
	for _, f := range findings {
		if f.Class != "secret" {
			t.Errorf("finding class = %q, want secret", f.Class)
		}
		if f.Path != "config.py" {
			t.Errorf("finding path = %q, want config.py", f.Path)
		}
		if f.Line == 0 {
			t.Error("finding line must be set (the class has a line)")
		}
	}
}

// TestSecretScan_PreExistingSecretOutsideDiffNotFlagged is acceptance
// criterion 1's negative leg: the same secret, already committed in the tree
// BEFORE the diff base, must never be flagged when the proposal diff never
// touches its file — diff-scoped, not tree-scoped.
func TestSecretScan_PreExistingSecretOutsideDiffNotFlagged(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "config.py", "DB_PASSWORD = 'irrelevant'\napi_key: sk-live-abcdef0123456789\n")
	writeFile(t, dir, "other.txt", "irrelevant\n")
	base := commit(t, dir, "base with a pre-existing secret")

	// The proposal diff touches an unrelated file only.
	writeFile(t, dir, "other.txt", "irrelevant\nmore\n")
	head := commit(t, dir, "unrelated change")

	ds, err := diffset.Extract(context.Background(), dir, base, head)
	if err != nil {
		t.Fatalf("diffset.Extract: %v", err)
	}

	env := &Env{Manifest: &schema.Manifest{Version: "1", Stack: "go"}, Ecosystem: ecosystem.New(), Diff: ds}
	result, findings, err := runSecretScan(context.Background(), env)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.State != "pass" {
		t.Errorf("state = %q, want pass (the secret sits outside the diff)", result.State)
	}
	if len(findings) != 0 {
		t.Errorf("findings = %+v, want none", findings)
	}
}

// TestSecretScan_GitattributesCannotHideSecret extends acceptance criterion
// 1: a proposal diff cannot pair a `path -diff` .gitattributes entry with a
// live secret in that same file to blind the scan (diffset.Extract forces
// git's textual diff algorithm precisely so this stays caught).
func TestSecretScan_GitattributesCannotHideSecret(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "README.md", "hello\n")
	base := commit(t, dir, "base")

	writeFile(t, dir, ".gitattributes", "config.py -diff\n")
	writeFile(t, dir, "config.py", "api_key: sk-live-abcdef0123456789\n")
	head := commit(t, dir, "add secret config marked -diff")

	ds, err := diffset.Extract(context.Background(), dir, base, head)
	if err != nil {
		t.Fatalf("diffset.Extract: %v", err)
	}

	env := &Env{Manifest: &schema.Manifest{Version: "1", Stack: "go"}, Ecosystem: ecosystem.New(), Diff: ds}
	result, findings, err := runSecretScan(context.Background(), env)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.State != "fail" {
		t.Fatalf("state = %q, want fail (the -diff attribute must not hide the secret)", result.State)
	}
	found := false
	for _, f := range findings {
		if f.Class == "secret" && f.Path == "config.py" {
			found = true
		}
	}
	if !found {
		t.Errorf("findings = %+v, want a secret finding on config.py", findings)
	}
}

func TestSecretScan_CaseInsensitiveAndEmbeddedKeywords(t *testing.T) {
	// The real secret forms the pre-fix case-sensitive, delimiter-adjacent
	// patterns missed entirely.
	for _, line := range []string{
		"API_KEY=sk-live-abcdef0123456789",
		"Password = hunter2hunter2",
		"TOKEN=ghp_deadbeefdeadbeefdead",
		"aws_secret_access_key = AKIAIOSFODNN7EXAMPLE",
		"Authorization: Bearer sk-live-abcdef0123456789",
	} {
		ds := &diffset.DiffSet{Files: []diffset.FileChange{
			{Status: diffset.StatusAdded, Path: "config.txt", AddedLines: []diffset.AddedLine{{Line: 1, Content: line}}},
		}}
		env := &Env{Manifest: &schema.Manifest{Version: "1", Stack: "go"}, Ecosystem: ecosystem.New(), Diff: ds}
		result, _, err := runSecretScan(context.Background(), env)
		if err != nil {
			t.Fatalf("Run(%q): %v", line, err)
		}
		if result.State != "fail" {
			t.Errorf("line %q: state = %q, want fail (secret must be caught)", line, result.State)
		}
	}
}

func TestSecretScan_CleanDiffPasses(t *testing.T) {
	ds := &diffset.DiffSet{Files: []diffset.FileChange{
		{Status: diffset.StatusAdded, Path: "main.go", AddedLines: []diffset.AddedLine{
			{Line: 1, Content: "package main"},
			{Line: 2, Content: "func main() {}"},
		}},
	}}
	env := &Env{Manifest: &schema.Manifest{Version: "1", Stack: "go"}, Ecosystem: ecosystem.New(), Diff: ds}
	result, findings, err := runSecretScan(context.Background(), env)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.State != "pass" {
		t.Errorf("state = %q, want pass", result.State)
	}
	if len(findings) != 0 {
		t.Errorf("findings = %+v, want none", findings)
	}
}

// TestSecretScan_ManifestExtraPattern proves the manifest extension point
// (schema.SecretScanConfig) is honored: a repo-specific pattern the
// ecosystem table knows nothing about still catches its own fixture.
func TestSecretScan_ManifestExtraPattern(t *testing.T) {
	ds := &diffset.DiffSet{Files: []diffset.FileChange{
		{Status: diffset.StatusAdded, Path: "creds.txt", AddedLines: []diffset.AddedLine{
			{Line: 1, Content: "internal-token-42"},
		}},
	}}
	manifest := &schema.Manifest{
		Version: "1", Stack: "go",
		SecretScan: &schema.SecretScanConfig{ExtraPatterns: []string{`internal-token-[0-9]+`}},
	}
	env := &Env{Manifest: manifest, Ecosystem: ecosystem.New(), Diff: ds}
	result, findings, err := runSecretScan(context.Background(), env)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.State != "fail" {
		t.Fatalf("state = %q, want fail", result.State)
	}
	if len(findings) != 1 || findings[0].Path != "creds.txt" {
		t.Errorf("findings = %+v, want one finding on creds.txt", findings)
	}
}

func TestSecretScan_NilDiffErrors(t *testing.T) {
	env := &Env{Manifest: &schema.Manifest{Version: "1", Stack: "go"}, Ecosystem: ecosystem.New()}
	if _, _, err := runSecretScan(context.Background(), env); err == nil {
		t.Error("expected an error when no diff set is provided")
	}
}
