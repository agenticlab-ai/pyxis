package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestDiscardProposalFilesystemRefusesLinksAndMissingParents(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	putFile(t, dir, "present/regular", "retained bytes")
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer closePolicyRoot(root)
	for _, name := range []string{"absent/file", "present/absent/file"} {
		if _, err := proposalFilePresent(t.Context(), root, proposalDiscardFile{Path: name}); err == nil {
			t.Fatal("missing parent admitted", name)
		}
		if err := removeProposalFile(t.Context(), root, proposalDiscardFile{Path: name}, ".pyxis/logs/recovery", nil); err == nil {
			t.Fatal("missing parent removal admitted", name)
		}
	}
	if err := os.Symlink("regular", filepath.Join(dir, "present/link")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"present/link", "present"} {
		if _, err := proposalFilePresent(t.Context(), root, proposalDiscardFile{Path: name}); err == nil {
			t.Fatal("nonregular proposal admitted", name)
		}
		if err := saveProposalFile(t.Context(), root, name, []byte("replacement"), 0600); err == nil {
			t.Fatal("nonregular recovery destination overwritten", name)
		}
	}
	if string(mustRead(t, filepath.Join(dir, "present/regular"))) != "retained bytes" {
		t.Fatal("symlink target changed")
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	if err := saveProposalFile(t.Context(), root, "closed/file", []byte("never written"), 0600); err == nil {
		t.Fatal("closed root accepted a recovery write")
	}
}

func TestDiscardProposalAtomicArchiveSkipsInterruptedTemps(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	putFile(t, dir, "copies/.intent.json.pending-0", "interrupted partial bytes")
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer closePolicyRoot(root)
	if err := saveProposalFile(t.Context(), root, "copies/intent.json", []byte("complete exact bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if string(mustRead(t, filepath.Join(dir, "copies/intent.json"))) != "complete exact bytes" || string(mustRead(t, filepath.Join(dir, "copies/.intent.json.pending-0"))) != "interrupted partial bytes" {
		t.Fatal("interrupted temp was published or overwritten")
	}
	for i := range 100 {
		putFile(t, dir, fmt.Sprintf("copies/.full.pending-%d", i), "retained failed attempt")
	}
	if err := saveProposalFile(t.Context(), root, "copies/full", []byte("unpublished"), 0600); err == nil {
		t.Fatal("exhausted temporary namespace accepted a write")
	}
	if _, err := os.Stat(filepath.Join(dir, "copies/full")); !os.IsNotExist(err) {
		t.Fatal("failed atomic copy published partial bytes", err)
	}
}

func TestDiscardProposalExecutableRestorationPreservesMode(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer closePolicyRoot(root)
	file := proposalDiscardFile{Path: "memory/problem/demo/capture.json", Mode: "100755", Bytes: []byte("exact proposal")}
	if err := restoreProposalFile(t.Context(), root, file, ".pyxis/logs/recovery"); err != nil {
		t.Fatal(err)
	}
	if present, err := proposalFilePresent(t.Context(), root, file); err != nil || !present {
		t.Fatal("restored executable mode differs", err)
	}
	if err := restoreProposalFile(t.Context(), root, file, ".pyxis/logs/recovery"); err != nil {
		t.Fatal("identical restore did not converge", err)
	}
	if !bytes.Equal(file.Bytes, mustRead(t, filepath.Join(dir, file.Path))) {
		t.Fatal("restored bytes differ")
	}
}

// A deterministic interruption/mutation at an IO cancellation boundary uses
// real filesystem operations without a scheduler race or a timing sleep.
type proposalIOContext struct {
	context.Context
	beforeCheck func()
}

func (c proposalIOContext) Err() error {
	c.beforeCheck()
	return c.Context.Err()
}

func TestDiscardProposalFilesystemCancellationAndPublicationRace(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	putFile(t, dir, "copies/retained", "original")
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer closePolicyRoot(root)
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, _, err := newProposalTemp(canceled, root, "never", 0600); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled temp creation continued", err)
	}
	if err := matchingProposalDestination(canceled, root, "never", nil, 0600); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled destination read continued", err)
	}
	ctx, stop := context.WithCancel(t.Context())
	defer stop()
	checks := 0
	duringWalk := proposalIOContext{Context: ctx, beforeCheck: func() {
		checks++
		if checks == 2 {
			stop()
		}
	}}
	if _, err := proposalParent(duringWalk, root, "copies/next/file", true); !errors.Is(err, context.Canceled) {
		t.Fatal("directory walk continued after cancellation", err)
	}
	for _, conflicting := range []bool{false, true} {
		name := fmt.Sprintf("race-%t", conflicting)
		checks = 0
		duringCopy := proposalIOContext{Context: t.Context(), beforeCheck: func() {
			checks++
			if checks == 2 {
				body := "requested bytes"
				if conflicting {
					body = "concurrent bytes"
				}
				putFile(t, dir, "copies/"+name, body)
			}
		}}
		err := saveProposalFile(duringCopy, root, "copies/"+name, []byte("requested bytes"), 0600)
		if (err != nil) != conflicting {
			t.Fatal("concurrent publication verdict", err)
		}
		if conflicting && string(mustRead(t, filepath.Join(dir, "copies", name))) != "concurrent bytes" {
			t.Fatal("atomic publication overwrote concurrent bytes")
		}
	}
}
