package invoke

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

func TestInvocationAndWorkspaceBoundaryRefusals(t *testing.T) {
	if got := (&ConfigError{Code: ExitRungErr}).ExitCode(); got != ExitRungErr {
		t.Fatalf("explicit exit code = %d", got)
	}
	for _, args := range [][]string{
		{"--workspace", "fixture", "--diff-base", "main", "--", "late"},
		{"--workspace", "fixture", "--diff-base", "main", "--"},
	} {
		inv, cerr := ParseInvocation(args)
		if len(args) == 6 {
			if inv != nil || cerr == nil {
				t.Fatalf("ParseInvocation(%v) = %+v, %v; want refusal", args, inv, cerr)
			}
		} else if inv == nil || cerr != nil {
			t.Fatalf("ParseInvocation terminal -- = %+v, %v", inv, cerr)
		}
	}
	if cerr := ValidateWorkspacePath(t.TempDir()); cerr == nil || !cerr.StdoutOnly {
		t.Fatalf("non-repository validation = %+v", cerr)
	}
}

func TestManifestAndBaselineReadBoundaries(t *testing.T) {
	workspace := t.TempDir()
	manifestPath := filepath.Join(workspace, ManifestPath)
	if err := os.MkdirAll(manifestPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, cerr := LoadManifest(workspace); cerr == nil || cerr.Code != ExitRungErr {
		t.Fatalf("directory manifest error = %+v", cerr)
	}

	if _, cerr := resolveBaselineHash(workspace, &schema.Manifest{BaselinePath: "missing.json"}); cerr == nil {
		t.Fatal("resolveBaselineHash accepted a missing declared baseline")
	}
	bad := filepath.Join(workspace, "bad.json")
	if err := os.WriteFile(bad, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, cerr := resolveBaselineHash(workspace, &schema.Manifest{BaselinePath: "bad.json"}); cerr == nil {
		t.Fatal("resolveBaselineHash accepted malformed JSON")
	}
}

func TestWorkspaceLockFilesystemBoundaries(t *testing.T) {
	fileWorkspace := filepath.Join(t.TempDir(), "workspace-file")
	if err := os.WriteFile(fileWorkspace, []byte("occupied"), 0o600); err != nil {
		t.Fatal(err)
	}
	if lock, cerr := AcquireWorkspaceLock(fileWorkspace); lock != nil || cerr == nil {
		t.Fatalf("file workspace lock = %+v, %+v", lock, cerr)
	}

	workspace := t.TempDir()
	lockPath := filepath.Join(workspace, ScratchDirRelPath, "workspace.lock")
	if err := os.MkdirAll(lockPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if lock, cerr := AcquireWorkspaceLock(workspace); lock != nil || cerr == nil {
		t.Fatalf("directory lock path = %+v, %+v", lock, cerr)
	}
	if err := (&WorkspaceLock{}).Release(); err != nil {
		t.Fatalf("empty lock release: %v", err)
	}
}

func TestPruneRunScratchMissingUnreadableAndRetainedCases(t *testing.T) {
	if err := PruneRunScratch(t.TempDir()); err != nil {
		t.Fatalf("missing runs root: %v", err)
	}

	unreadable := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(unreadable, RunsRelPath)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(unreadable, RunsRelPath), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := PruneRunScratch(unreadable); err == nil {
		t.Fatal("PruneRunScratch accepted a non-directory runs root")
	}

	workspace := t.TempDir()
	runs := filepath.Join(workspace, RunsRelPath)
	for _, name := range []string{"keep", "stale"} {
		if err := os.MkdirAll(filepath.Join(runs, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(runs, "not-a-run"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	verdictPath := filepath.Join(workspace, VerdictRelPath)
	if err := os.MkdirAll(filepath.Dir(verdictPath), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(schema.Verdict{RunID: "keep"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(verdictPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := PruneRunScratch(workspace); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(runs, "keep")); err != nil {
		t.Fatalf("retained run missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(runs, "stale")); !os.IsNotExist(err) {
		t.Fatalf("stale run still present: %v", err)
	}

	if err := os.WriteFile(verdictPath, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := currentVerdictRunID(workspace); got != "" {
		t.Fatalf("malformed verdict retained run %q", got)
	}
}

func TestEmissionAndVersionParseBoundaries(t *testing.T) {
	if err := (&ConfigError{Message: "missing workspace"}).Emit(); err == nil {
		t.Fatal("Emit accepted an empty artifact workspace")
	}
	if got := parseVersion("x.2.3"); got != [3]int{0, 2, 3} {
		t.Fatalf("parseVersion nonnumeric component = %v", got)
	}
}
