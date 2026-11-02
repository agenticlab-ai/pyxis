package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestDiscardProposalTakenObjectFailuresPreserveSource(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"ignored-destination", "archive-parent", "retained-attempts", "missing-leaf", "cancel-before-take", "cancel-after-take"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			file := proposalDiscardFile{Path: "memory/x.json", Mode: "100644", Bytes: []byte("committed bytes")}
			putFile(t, dir, file.Path, string(file.Bytes))
			root, err := os.OpenRoot(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer closePolicyRoot(root)
			archive := ".pyxis/logs/recovery"
			ignored := func(context.Context, []string) error { return nil }
			ctx := t.Context()
			switch scenario {
			case "ignored-destination":
				ignored = func(context.Context, []string) error { return errors.New("destination is not ignored") }
			case "archive-parent":
				putFile(t, dir, ".pyxis/logs", "not a directory")
			case "retained-attempts":
				for i := range 100 {
					if err := os.MkdirAll(filepath.Join(dir, archive, fmt.Sprintf("taken-x.json-%d", i)), 0700); err != nil {
						t.Fatal(err)
					}
				}
			case "missing-leaf":
				if err := os.Remove(filepath.Join(dir, file.Path)); err != nil {
					t.Fatal(err)
				}
			case "cancel-before-take", "cancel-after-take":
				cancelable, cancel := context.WithCancel(ctx)
				defer cancel()
				ctx = proposalIOContext{Context: cancelable, beforeCheck: func() {
					if scenario == "cancel-before-take" {
						if _, err := os.Stat(filepath.Join(dir, archive, "taken-x.json-0")); err == nil {
							cancel()
						}
					} else if _, err := os.Stat(filepath.Join(dir, file.Path)); os.IsNotExist(err) {
						cancel()
					}
				}}
			}
			if err := removeProposalFile(ctx, root, file, archive, ignored); err == nil {
				t.Fatal("failed withdrawal reported success")
			}
			if scenario != "missing-leaf" && string(mustRead(t, filepath.Join(dir, file.Path))) != string(file.Bytes) {
				t.Fatal("failed withdrawal lost source bytes")
			}
		})
	}
}
