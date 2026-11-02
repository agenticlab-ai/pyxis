package config_test

// SetEpochCommit tests (existing-repo-onboarding/001 T8): the targeted
// epoch.commit writer adoption's apply uses to record the install epoch into an
// existing config surface without disturbing the rest of it. The three
// structural branches — replace an existing commit line, insert one into a
// commit-less epoch block, append a fresh epoch block — plus the closed-parse
// guard and the byte-preservation of every other line.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

const epochCommitSHA = "0123456789abcdef0123456789abcdef01234567"

func writeConfigFixture(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func readConfigFixture(t *testing.T, root string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, ".pyxis", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// assertClosedParseWithCommit reports that the config re-parses under the closed
// schema with no defect and records commit as its epoch.commit.
func assertClosedParseWithCommit(t *testing.T, body, commit string) {
	t.Helper()
	c, defects := config.Parse([]byte(body))
	if len(defects) > 0 {
		t.Fatalf("result does not closed-parse: %v\n%s", defects, body)
	}
	if c.Epoch.Commit != commit {
		t.Errorf("epoch.commit = %q, want %q\n%s", c.Epoch.Commit, commit, body)
	}
}

func TestSetEpochCommitReplacesExistingCommit(t *testing.T) {
	root := writeConfigFixture(t, "schema_version: pyxis-config/v2\nsystem:\n  version: \"v1.0.0\"\n  mode: release\nepoch:\n  system_version: \"v1.0.0\"\n  commit: \"deadbeef\"\n  manifest_sha256: \"aa\"\n")
	if err := config.SetEpochCommit(root, epochCommitSHA); err != nil {
		t.Fatalf("SetEpochCommit: %v", err)
	}
	body := readConfigFixture(t, root)
	assertClosedParseWithCommit(t, body, epochCommitSHA)
	// The other epoch fields and the system block are preserved.
	for _, keep := range []string{"system_version: \"v1.0.0\"", "manifest_sha256: \"aa\"", "mode: release"} {
		if !strings.Contains(body, keep) {
			t.Errorf("SetEpochCommit clobbered %q:\n%s", keep, body)
		}
	}
	if strings.Contains(body, "deadbeef") {
		t.Errorf("old commit still present:\n%s", body)
	}
}

func TestSetEpochCommitInsertsIntoCommitlessEpochBlock(t *testing.T) {
	root := writeConfigFixture(t, "schema_version: pyxis-config/v2\nepoch:\n  system_version: \"v1.0.0\"\n")
	if err := config.SetEpochCommit(root, epochCommitSHA); err != nil {
		t.Fatalf("SetEpochCommit: %v", err)
	}
	body := readConfigFixture(t, root)
	assertClosedParseWithCommit(t, body, epochCommitSHA)
	if !strings.Contains(body, "system_version: \"v1.0.0\"") {
		t.Errorf("existing epoch field clobbered:\n%s", body)
	}
}

func TestSetEpochCommitAppendsEpochBlockWhenAbsent(t *testing.T) {
	root := writeConfigFixture(t, "schema_version: pyxis-config/v2\nsystem:\n  version: \"v1.0.0\"\n  mode: release\n")
	if err := config.SetEpochCommit(root, epochCommitSHA); err != nil {
		t.Fatalf("SetEpochCommit: %v", err)
	}
	body := readConfigFixture(t, root)
	assertClosedParseWithCommit(t, body, epochCommitSHA)
	if !strings.Contains(body, "mode: release") {
		t.Errorf("system block clobbered by the appended epoch:\n%s", body)
	}
}

func TestSetEpochCommitIsIdempotent(t *testing.T) {
	root := writeConfigFixture(t, "schema_version: pyxis-config/v2\n")
	if err := config.SetEpochCommit(root, epochCommitSHA); err != nil {
		t.Fatalf("first SetEpochCommit: %v", err)
	}
	once := readConfigFixture(t, root)
	if err := config.SetEpochCommit(root, epochCommitSHA); err != nil {
		t.Fatalf("second SetEpochCommit: %v", err)
	}
	if twice := readConfigFixture(t, root); twice != once {
		t.Errorf("SetEpochCommit not idempotent:\nonce  %q\ntwice %q", once, twice)
	}
}

func TestSetEpochCommitRefusesAbsentConfig(t *testing.T) {
	root := t.TempDir() // no .pyxis/config.yaml
	if err := config.SetEpochCommit(root, epochCommitSHA); err == nil {
		t.Fatal("SetEpochCommit on an absent config: want an error, got nil")
	}
}

// ClearEpochCommit is undo's surgical reversal of the epoch apply recorded
// (existing-repo-onboarding/001 T10): the commit line goes, the rest stays.

func TestClearEpochCommitRemovesTheCommitPreservingTheRest(t *testing.T) {
	root := writeConfigFixture(t, "schema_version: pyxis-config/v2\nsystem:\n  version: \"v1.0.0\"\n  mode: release\nepoch:\n  system_version: \"v1.0.0\"\n  commit: \"deadbeef\"\n  manifest_sha256: \"aa\"\n")
	if err := config.ClearEpochCommit(root); err != nil {
		t.Fatalf("ClearEpochCommit: %v", err)
	}
	body := readConfigFixture(t, root)
	c, defects := config.Parse([]byte(body))
	if len(defects) > 0 {
		t.Fatalf("result does not closed-parse: %v\n%s", defects, body)
	}
	if c.Epoch.Commit != "" {
		t.Errorf("epoch.commit = %q, want cleared\n%s", c.Epoch.Commit, body)
	}
	// The other epoch fields and the system block are preserved.
	for _, keep := range []string{"system_version: \"v1.0.0\"", "manifest_sha256: \"aa\"", "mode: release"} {
		if !strings.Contains(body, keep) {
			t.Errorf("ClearEpochCommit clobbered %q:\n%s", keep, body)
		}
	}
	if strings.Contains(body, "deadbeef") {
		t.Errorf("the commit value survived the clear:\n%s", body)
	}
}

func TestClearEpochCommitIsANoOpWithoutAnEpoch(t *testing.T) {
	const base = "schema_version: pyxis-config/v2\nsystem:\n  version: \"v1.0.0\"\n  mode: release\n"
	root := writeConfigFixture(t, base)
	if err := config.ClearEpochCommit(root); err != nil {
		t.Fatalf("ClearEpochCommit: %v", err)
	}
	if body := readConfigFixture(t, root); body != base {
		t.Errorf("ClearEpochCommit changed an epoch-less config:\n got %q\nwant %q", body, base)
	}
}

func TestClearEpochCommitToleratesAbsentConfig(t *testing.T) {
	if err := config.ClearEpochCommit(t.TempDir()); err != nil {
		t.Fatalf("ClearEpochCommit on an absent config: want nil, got %v", err)
	}
}

func TestSetThenClearEpochCommitRoundTrips(t *testing.T) {
	root := writeConfigFixture(t, "schema_version: pyxis-config/v2\n")
	if err := config.SetEpochCommit(root, epochCommitSHA); err != nil {
		t.Fatalf("SetEpochCommit: %v", err)
	}
	if err := config.ClearEpochCommit(root); err != nil {
		t.Fatalf("ClearEpochCommit: %v", err)
	}
	c, defects := config.Parse([]byte(readConfigFixture(t, root)))
	if len(defects) > 0 {
		t.Fatalf("result does not closed-parse: %v", defects)
	}
	if c.Epoch.Commit != "" {
		t.Errorf("epoch.commit = %q after set-then-clear, want empty", c.Epoch.Commit)
	}
}
