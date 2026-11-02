package issueentry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/prdescription"
	"github.com/agenticlab-ai/pyxis/internal/prdescription/prdescriptiontest"
)

func retainedCheckpoint(t *testing.T) Checkpoint {
	t.Helper()
	description, problems := prdescription.FromAgent(prdescriptiontest.Message)
	if len(problems) != 0 {
		t.Fatal(problems)
	}
	return Checkpoint{Schema: CheckpointSchema, ContractID: "sha256:" + strings.Repeat("c", 64), TaskID: "issue-task", Branch: "feat/task-17", Revision: strings.Repeat("a", 40), Description: description.Render()}
}

func TestCheckpointRetainsTheImplementerDescriptionForItsRevision(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "task-runtime")
	if _, ok, err := LoadCheckpoint(dir); ok || err != nil {
		t.Fatalf("absent record: %v %v", ok, err)
	}
	want := retainedCheckpoint(t)
	if err := RetainCheckpoint(dir, want); err != nil {
		t.Fatal(err)
	}
	got, ok, err := LoadCheckpoint(dir)
	if err != nil || !ok || got != want {
		t.Fatalf("round trip: %+v %v %v", got, ok, err)
	}
	description, problems := prdescription.FromAgent(got.Description)
	original, _ := prdescription.FromAgent(prdescriptiontest.Message)
	if len(problems) != 0 || description != original {
		t.Fatalf("description changed in retention: %+v %v", description, problems)
	}
	// A later described attempt replaces the record whole.
	want.Revision = strings.Repeat("b", 64)
	if err := RetainCheckpoint(dir, want); err != nil {
		t.Fatal(err)
	}
	if got, _, err := LoadCheckpoint(dir); err != nil || got.Revision != want.Revision {
		t.Fatalf("replacement: %+v %v", got, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary files left behind: %v %v", entries, err)
	}
	if info, err := os.Stat(filepath.Join(dir, checkpointFile)); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("record permissions: %v %v", info, err)
	}
}

func TestCheckpointRefusesRecordsRecoveryCannotPublish(t *testing.T) {
	for name, mutate := range map[string]func(*Checkpoint){
		"schema":           func(c *Checkpoint) { c.Schema = "issue-checkpoint/v2" },
		"contract":         func(c *Checkpoint) { c.ContractID = "contract" },
		"task":             func(c *Checkpoint) { c.TaskID = "" },
		"branch":           func(c *Checkpoint) { c.Branch = "" },
		"short revision":   func(c *Checkpoint) { c.Revision = "abc123" },
		"uppercase":        func(c *Checkpoint) { c.Revision = strings.Repeat("A", 40) },
		"no description":   func(c *Checkpoint) { c.Description = "" },
		"missing sections": func(c *Checkpoint) { c.Description = prdescription.Description{What: "- x"}.Render() },
	} {
		t.Run(name, func(t *testing.T) {
			c := retainedCheckpoint(t)
			mutate(&c)
			dir := t.TempDir()
			if err := RetainCheckpoint(dir, c); err == nil {
				t.Fatal("invalid record retained")
			}
			if _, err := os.Stat(filepath.Join(dir, checkpointFile)); !os.IsNotExist(err) {
				t.Fatalf("refused record written: %v", err)
			}
		})
	}
	for name, wire := range map[string]string{
		"unknown field": `{"schema_version":"issue-checkpoint/v1","extra":true}`,
		"two objects":   `{} {}`,
		"not json":      `{`,
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, checkpointFile), []byte(wire), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, ok, err := LoadCheckpoint(dir); err == nil || ok {
				t.Fatal("malformed record loaded")
			}
		})
	}
	invalid := retainedCheckpoint(t)
	invalid.Revision = "not-a-revision"
	wire, err := json.Marshal(invalid)
	if err != nil {
		t.Fatal(err)
	}
	stale := t.TempDir()
	if err := os.WriteFile(filepath.Join(stale, checkpointFile), wire, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := LoadCheckpoint(stale); err == nil || ok {
		t.Fatal("well-formed invalid record loaded")
	}
	if os.Getuid() != 0 {
		readOnly := t.TempDir()
		if err := os.Chmod(readOnly, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(readOnly, 0o700) }) // let TempDir cleanup remove it
		if err := RetainCheckpoint(readOnly, retainedCheckpoint(t)); err == nil {
			t.Fatal("record written into a read-only directory")
		}
	}
	unreadable := t.TempDir()
	if err := os.Mkdir(filepath.Join(unreadable, checkpointFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := LoadCheckpoint(unreadable); err == nil || ok {
		t.Fatal("unreadable record loaded")
	}
	if err := RetainCheckpoint(unreadable, retainedCheckpoint(t)); err == nil {
		t.Fatal("record replaced a directory")
	}
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RetainCheckpoint(filepath.Join(blocked, "runtime"), retainedCheckpoint(t)); err == nil {
		t.Fatal("record written under a file")
	}
}
