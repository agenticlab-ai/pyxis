package store

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

type interruptedEvidenceTemp struct {
	*os.File
	step       string
	failure    error
	beforeSync func()
}

func (f interruptedEvidenceTemp) Chmod(mode os.FileMode) error {
	if f.step == "chmod" {
		return f.failure
	}
	return f.File.Chmod(mode)
}

func (f interruptedEvidenceTemp) Write(b []byte) (int, error) {
	if f.step == "write" {
		return 0, f.failure
	}
	return f.File.Write(b)
}

func (f interruptedEvidenceTemp) Sync() error {
	if f.step == "sync" {
		return f.failure
	}
	if f.beforeSync != nil {
		f.beforeSync()
	}
	return f.File.Sync()
}

func (f interruptedEvidenceTemp) Close() error {
	err := f.File.Close()
	if f.step == "close" {
		return f.failure
	}
	return err
}

func TestDraftEvidencePublicationFailuresLeaveWholeOriginal(t *testing.T) {
	for _, step := range []string{"create", "chmod", "write", "sync", "close", "rename", "concurrent edit", "unreadable"} {
		t.Run(step, func(t *testing.T) {
			r := rec(record.Learning, "learning/fault", "", "Original lesson.", 1)
			r.Provenance.Source = "README.md"
			root := t.TempDir()
			if _, err := Write(root, r); err != nil {
				t.Fatal(err)
			}
			full := filepath.Join(root, r.Path())
			original := bytesOf(t, r)
			bound := *r
			bound.Provenance.Evidence = &record.Evidence{Schema: record.EvidenceSchema, Locator: record.EvidenceIntroduction, Files: []record.EvidenceFile{{Path: "README.md", Mode: "100644", SHA256: strings.Repeat("a", 64)}}}
			failure := errors.New("publication interrupted at " + step)
			want := original
			create := func(dir string) (evidenceTemp, error) {
				if step == "create" {
					return nil, failure
				}
				f, err := os.CreateTemp(dir, ".evidence-*")
				if err != nil {
					return nil, err
				}
				wrapped := interruptedEvidenceTemp{File: f, step: step, failure: failure}
				if step == "concurrent edit" {
					want = append(append([]byte(nil), original...), '\n')
					wrapped.beforeSync = func() {
						if err := os.WriteFile(full, want, 0o600); err != nil {
							t.Fatal(err)
						}
					}
				}
				return wrapped, nil
			}
			rename := func(from, to string) error {
				if step == "rename" {
					return failure
				}
				return os.Rename(from, to)
			}
			if step == "unreadable" {
				if err := os.Chmod(full, 0); err != nil {
					t.Fatal(err)
				}
			}
			err := replaceDraftEvidence(root, &bound, original, create, rename)
			if err == nil {
				t.Fatal("interrupted replacement succeeded")
			}
			if step != "unreadable" && step != "concurrent edit" && !errors.Is(err, failure) {
				t.Fatalf("lost IO cause: %v", err)
			}
			if err := os.Chmod(full, 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(full)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatal("publication damaged original or overwrote concurrent edit")
			}
			files, err := filepath.Glob(filepath.Join(filepath.Dir(full), ".evidence-*"))
			if err != nil || len(files) > 0 {
				t.Fatalf("unpublished scratch remains: %v %v", files, err)
			}
		})
	}
}

func TestDraftEvidenceReplacementPreservesAndGuardsBytes(t *testing.T) {
	r := rec(record.Learning, "learning/example", "", "Keep the exact lesson.", 1)
	r.Provenance.Source = "README.md"
	original := bytesOf(t, r)
	bound := *r
	bound.Provenance.Evidence = &record.Evidence{Schema: record.EvidenceSchema, Locator: record.EvidenceIntroduction, Files: []record.EvidenceFile{{Path: "README.md", Mode: "100644", SHA256: strings.Repeat("a", 64)}}}
	root := t.TempDir()
	if _, err := Write(root, r); err != nil {
		t.Fatal(err)
	}
	full := filepath.Join(root, r.Path())
	if err := os.Chmod(full, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ReplaceDraftEvidence(root, &bound, original); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(full)
	if err != nil {
		t.Fatal(err)
	}
	if err := ReplaceDraftEvidence(root, &bound, after); err != nil {
		t.Fatal("idempotent replacement", err)
	}
	if err := ReplaceDraftEvidence(root, &bound, original); err == nil {
		t.Fatal("stale original overwritten")
	}
	changed := bound
	changed.Statement = "rewrite prose"
	if err := ReplaceDraftEvidence(root, &changed, after); err == nil {
		t.Fatal("prose changed")
	}
	if err := ReplaceDraftEvidence(root, &bound, []byte("bad json")); err == nil {
		t.Fatal("malformed original")
	}
	invalid := bound
	invalid.Provenance.Commit = "not a commit"
	if err := ReplaceDraftEvidence(root, &invalid, after); err == nil {
		t.Fatal("invalid binding")
	}
	if err := ReplaceDraftEvidence(t.TempDir(), &bound, original); err == nil {
		t.Fatal("missing draft")
	}
	if err := os.Remove(full); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("README.md", full); err != nil {
		t.Fatal(err)
	}
	if err := ReplaceDraftEvidence(root, &bound, original); err == nil {
		t.Fatal("symlink draft")
	}
	if !bytes.Equal(original, bytesOf(t, r)) {
		t.Fatal("input record mutated")
	}
	linked := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(root, linked); err != nil {
		t.Fatal(err)
	}
	if err := ReplaceDraftEvidence(linked, &bound, original); err == nil {
		t.Fatal("symlink root")
	}
}
