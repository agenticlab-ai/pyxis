package githost

import (
	"context"
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/lane/verdict"
)

// An unreadable GitHub object must remain an error after the host-selector
// lift; it cannot become a vacuous green or an empty proposal/history set.
func TestGitHubCompatibilityUnreadableObjectsFailClosed(t *testing.T) {
	for _, tc := range []struct {
		name string
		read func(*Host) error
	}{
		{"required checks", func(h *Host) error { _, err := h.AllRequiredGreen(t.Context(), "7", "sha"); return err }},
		{"review verdict", func(h *Host) error { _, err := h.ReviewVerdict(t.Context(), "7", "sha"); return err }},
		{"merge proof", func(h *Host) error { _, err := h.MergedIntoMainline(t.Context(), "7", "sha"); return err }},
		{"proposals", func(h *Host) error { _, err := h.OpenProposals(t.Context()); return err }},
		{"history", func(h *Host) error { _, err := h.LastRecordedHead(t.Context(), "7"); return err }},
		{"record", func(h *Host) error { return h.RecordVerdict(t.Context(), "7", verdict.Record{}) }},
		{"act", func(h *Host) error { return (&Machine{Host: h, Name: "machine"}).Act(t.Context(), "7", "sha") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, fail := range []bool{false, true} {
				h := New("o/r", t.TempDir(), "origin", "main")
				h.Run = func(context.Context, ...string) ([]byte, error) {
					if fail {
						return nil, errors.New("offline")
					}
					return []byte("{"), nil
				}
				if err := tc.read(h); err == nil {
					t.Fatal("unreadable host object was accepted")
				}
			}
		})
	}
}
