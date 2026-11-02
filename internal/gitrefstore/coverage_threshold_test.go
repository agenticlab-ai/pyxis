package gitrefstore

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestStorePublicBoundaryRefusals(t *testing.T) {
	_, repo := cloneFixture(t)
	store, err := New(repo, "origin", "refs/pyxis/coverage-boundaries")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.Read(t.Context(), "../outside"); err == nil {
		t.Fatal("Read accepted an unsafe path")
	}
	if got, err := store.Read(t.Context(), "events/missing"); err != nil || got != nil {
		t.Fatalf("Read on an absent ref = %q, %v", got, err)
	}
	if _, err := store.ReadMany(t.Context(), "ok", "../outside"); err == nil {
		t.Fatal("ReadMany accepted an unsafe path")
	}
	if _, err := store.Fetch(t.Context(), "../outside/"); err == nil {
		t.Fatal("Fetch accepted an unsafe prefix")
	}
	if _, err := store.ReadExact(t.Context(), "not-an-object", "events/a"); err == nil {
		t.Fatal("ReadExact accepted an invalid object")
	}
	if _, err := store.SnapshotAt(t.Context(), "not-an-object", "events"); err == nil {
		t.Fatal("SnapshotAt accepted an invalid object")
	}
	if _, err := store.SnapshotAt(t.Context(), strings.Repeat("a", 40), "../outside/"); err == nil {
		t.Fatal("SnapshotAt accepted an unsafe prefix")
	}
	if _, err := store.EstablishGenesis(t.Context(), " \n "); err == nil {
		t.Fatal("EstablishGenesis accepted empty authorization")
	}
}

func TestStoreMutationBoundaryRefusals(t *testing.T) {
	_, repo := cloneFixture(t)
	store, err := New(repo, "origin", "refs/pyxis/coverage-mutations")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.Apply(t.Context(), "", "nil", nil); err == nil {
		t.Fatal("Apply accepted a nil mutation")
	}
	if _, err := store.Apply(t.Context(), "../outside/", "bad prefix", func(Snapshot) (Mutation, error) {
		return Mutation{}, nil
	}); err == nil {
		t.Fatal("Apply accepted an unsafe prefix")
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.Apply(canceled, "events", "canceled", func(Snapshot) (Mutation, error) {
		return Mutation{}, nil
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Apply canceled error = %v", err)
	}

	wantErr := errors.New("mutation failed")
	if _, err := store.Apply(t.Context(), "events", "callback error", func(Snapshot) (Mutation, error) {
		return Mutation{}, wantErr
	}); !errors.Is(err, wantErr) {
		t.Fatalf("Apply callback error = %v", err)
	}
	if _, err := store.Apply(t.Context(), "events", "empty change", func(Snapshot) (Mutation, error) {
		return Mutation{Changed: true}, nil
	}); err == nil {
		t.Fatal("Apply accepted a changed mutation without files")
	}
	if _, err := store.Apply(t.Context(), "events", "unsafe path", func(Snapshot) (Mutation, error) {
		return Mutation{Changed: true, Files: map[string][]byte{"../outside": []byte("x")}}, nil
	}); err == nil {
		t.Fatal("Apply accepted an unsafe mutation path")
	}

	if _, err := store.Update(t.Context(), "../outside", "unsafe", func([]byte) ([]byte, bool, error) {
		return nil, false, nil
	}); err == nil {
		t.Fatal("Update accepted an unsafe path")
	}
	if _, err := store.Update(t.Context(), "events/a", "nil", nil); err == nil {
		t.Fatal("Update accepted a nil mutation")
	}
	if _, err := store.Update(t.Context(), "events/a", "callback error", func([]byte) ([]byte, bool, error) {
		return nil, false, wantErr
	}); !errors.Is(err, wantErr) {
		t.Fatalf("Update callback error = %v", err)
	}
}

func TestStoreValidationHelpersRejectMalformedValues(t *testing.T) {
	if validObjectID("abc") {
		t.Fatal("validObjectID accepted a short object")
	}
	if err := validPath(""); err == nil {
		t.Fatal("validPath accepted an empty path")
	}
}
