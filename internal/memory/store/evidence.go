package store

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

// ReplaceDraftEvidence is the narrowly scoped exception for an unlanded
// envelope. The caller holds Lock, proves absence at its pinned trusted base,
// and preflights the complete batch. Only commit/evidence may differ.
func ReplaceDraftEvidence(root string, r *record.Record, expected []byte) error {
	return replaceDraftEvidence(root, r, expected, func(dir string) (evidenceTemp, error) {
		return os.CreateTemp(dir, ".evidence-*")
	}, os.Rename)
}

// The private IO boundary lets interruption tests fail each publication step
// while retaining real files and proving the original envelope stays intact.
type evidenceTemp interface {
	Name() string
	Chmod(os.FileMode) error
	Write([]byte) (int, error)
	Sync() error
	Close() error
}

func replaceDraftEvidence(root string, r *record.Record, expected []byte, create func(string) (evidenceTemp, error), rename func(string, string) error) error {
	old, err := record.Unmarshal(expected)
	if err != nil {
		return err
	}
	before, after := *old, *r
	before.Provenance.Commit = ""
	before.Provenance.Evidence = nil
	after.Provenance.Commit = ""
	after.Provenance.Evidence = nil
	if !reflect.DeepEqual(before, after) {
		return fmt.Errorf("draft replacement changes non-evidence fields")
	}
	if err := r.Validate(); err != nil {
		return err
	}
	body, err := record.Marshal(r)
	if err != nil {
		return err
	}
	full, err := confined(root, r.Path())
	if err != nil {
		return err
	}
	info, err := os.Lstat(full)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: draft is not a regular file", ErrEscapes)
	}
	check := func() error {
		current, err := os.ReadFile(full)
		if err != nil {
			return err
		}
		if !bytes.Equal(current, expected) {
			return fmt.Errorf("draft %s changed since preflight", r.ID)
		}
		return nil
	}
	if err := check(); err != nil {
		return err
	}
	if bytes.Equal(body, expected) {
		return nil
	}
	tmp, err := create(filepath.Dir(full))
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // Best-effort removal of unpublished scratch.
	defer func() { _ = tmp.Close() }()           // Close after errors; explicit close below handles success.
	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		return err
	}
	if _, err := tmp.Write(body); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := check(); err != nil {
		return err
	}
	if err := rename(tmp.Name(), full); err != nil {
		return fmt.Errorf("replace draft %s: %w", r.ID, err)
	}
	return nil
}
