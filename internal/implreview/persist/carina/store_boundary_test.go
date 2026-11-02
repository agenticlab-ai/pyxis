package carina

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRecordStoreRejectsConfigurationAndPreflightBoundaries(t *testing.T) {
	if invalidRecord(nil) != nil {
		t.Fatal("invalidRecord(nil) must remain nil")
	}
	wrapped := invalidRecord(errors.New("corrupt"))
	if invalidRecord(wrapped) != wrapped {
		t.Fatal("invalidRecord wrapped an already typed error twice")
	}
	if _, err := NewFileRecordStore(" "); err == nil {
		t.Fatal("NewFileRecordStore accepted an empty root")
	}

	missing, err := NewFileRecordStore(filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatal(err)
	}
	if err := missing.Preflight(t.Context(), "o/r"); err == nil {
		t.Fatal("Preflight accepted a missing root")
	}

	fileRoot := filepath.Join(t.TempDir(), "root-file")
	if err := os.WriteFile(fileRoot, []byte("occupied"), 0o644); err != nil {
		t.Fatal(err)
	}
	notDirectory, err := NewFileRecordStore(fileRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := notDirectory.Preflight(t.Context(), "o/r"); err == nil {
		t.Fatal("Preflight accepted a file root")
	}

	valid, err := NewFileRecordStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := valid.Preflight(t.Context(), " "); err == nil {
		t.Fatal("Preflight accepted an empty repository")
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := valid.Preflight(canceled, "o/r"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Preflight canceled error = %v", err)
	}
}

func TestRecordStoreRejectsCreateAndReadBoundaries(t *testing.T) {
	store, err := NewFileRecordStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.CreateIfAbsent(canceled, carinaRecord()); !errors.Is(err, context.Canceled) {
		t.Fatalf("CreateIfAbsent canceled error = %v", err)
	}
	if _, err := store.CreateIfAbsent(t.Context(), ReviewRecord{}); err == nil {
		t.Fatal("CreateIfAbsent accepted an incomplete record")
	}
	escaping := carinaRecord()
	escaping.IdempotencyID = "../../../../../../escape"
	if _, err := store.CreateIfAbsent(t.Context(), escaping); err == nil {
		t.Fatal("CreateIfAbsent accepted an escaping identity")
	}
	if _, err := store.ExactRead(canceled, "anything"); !errors.Is(err, context.Canceled) {
		t.Fatalf("ExactRead canceled error = %v", err)
	}
	for _, receipt := range []string{
		"invalid",
		"carina-review-record:not-base64!:7:id",
		"carina-review-record:by9y:not-a-number:id",
	} {
		if _, err := store.ExactRead(t.Context(), receipt); err == nil {
			t.Fatalf("ExactRead accepted malformed receipt %q", receipt)
		}
	}
}

func TestRecordStoreRejectsMalformedAndTrailingRecordData(t *testing.T) {
	store, err := NewFileRecordStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	receipt := receiptFor("o/r", 7, "malformed")
	path, err := store.pathForReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ExactRead(t.Context(), receipt); err == nil {
		t.Fatal("ExactRead accepted malformed JSON")
	}

	valid := carinaRecord()
	valid.SchemaVersion = recordSchema
	valid.CreatedAt = store.now().UTC()
	valid.RecordDigest = recordDigest(valid)
	encoded, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(encoded, []byte("\n{}")...), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ExactRead(t.Context(), receipt); err == nil {
		t.Fatal("ExactRead accepted trailing JSON")
	}
}
