package audit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

type inspectionReadFailure struct{ err error }

func (r inspectionReadFailure) Read([]byte) (int, error) { return 0, r.err }

func TestInspectReaderReturnsEvidenceBeforeReadFailure(t *testing.T) {
	root := t.TempDir()
	event := completeEvent()
	event.RepositoryID = fmt.Sprintf("%x", sha256.Sum256([]byte(root)))
	event.Event, event.Phase, event.JobID, event.Outcome = EventRun, PhaseTerminal, "daily", OutcomeIncomplete
	event.References = []Reference{{Kind: "retained_workspace", Location: "maintenance-attempt:kept"}}
	data, err := encode(event)
	if err != nil {
		t.Fatal(err)
	}
	for _, failure := range []error{errors.New("injected reader failure"), context.Canceled} {
		reader := io.MultiReader(bytes.NewReader(append(data, []byte("{malformed}\n")...)), inspectionReadFailure{failure})
		observed, err := inspectReader(t.Context(), root, reader)
		if !errors.Is(err, failure) || len(observed.Events) != 1 || observed.Malformed != 1 || observed.Events[0].References[0].Location != "maintenance-attempt:kept" {
			t.Fatalf("partial reader evidence = %+v, %v", observed, err)
		}
	}
}

func TestInspectNonregularJournalProcessHelper(t *testing.T) {
	root := os.Getenv("PYXIS_AUDIT_INSPECT_ROOT")
	if root == "" {
		return
	}
	if _, err := Inspect(t.Context(), root); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("nonregular journal was not refused as such: %v", err)
	}
}

func TestInspectRejectsNonregularJournalWithoutBlocking(t *testing.T) {
	for _, kind := range []string{"directory", "fifo without writer"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, filepath.FromSlash(journalPath))
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			var err error
			if kind == "directory" {
				err = os.Mkdir(path, 0o700)
			} else {
				err = syscall.Mkfifo(path, 0o600)
			}
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestInspectNonregularJournalProcessHelper$")
			command.Env = append(os.Environ(), "PYXIS_AUDIT_INSPECT_ROOT="+root)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("inspection blocked or failed: %v: %s", err, output)
			}
		})
	}
}
