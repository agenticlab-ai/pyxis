package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/maintenance/installation"
)

func TestMaintenanceInstallInterleavedRefusalCountsEachInvocationOnce(t *testing.T) {
	root, cron := maintenanceAdmissionFixture(t)
	renamed := filepath.Join(filepath.Dir(root), "repository\nname")
	if err := os.Rename(root, renamed); err != nil {
		t.Fatal(err)
	}
	root = renamed
	store, err := installation.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 28, 2, 0, 0, 0, time.UTC)
	other := strings.Repeat("b", 32)
	inserted, reads := false, 0
	prior := maintenanceNow
	t.Cleanup(func() { maintenanceNow = prior })
	maintenanceNow = func() time.Time {
		reads++
		// Put invocation B between A's writes if A attempts to count the
		// same refusal twice. This reproduces the process interleaving using
		// the existing clock seam and the real counter store, without races.
		if reads == 2 {
			if err := store.RecordRefusal(t.Context(), other, "installation", installation.CauseInstallationFailed, at); err != nil {
				t.Fatal(err)
			}
			inserted = true
		}
		return at
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"maintenance", "install", "--repo", root}, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "canonical repository root must be clean and absolute") || cron.installs != 0 {
		t.Fatalf("fixture did not reach the unowned Render refusal: code=%d installs=%d stderr=%q", code, cron.installs, stderr.String())
	}
	if !inserted {
		if err := store.RecordRefusal(t.Context(), other, "installation", installation.CauseInstallationFailed, at); err != nil {
			t.Fatal(err)
		}
	}
	state, err := store.LoadRefusals(t.Context())
	if err != nil || len(state.Counters) != 1 || state.Counters[0].Lifetime != 2 || state.Counters[0].Consecutive != 2 {
		t.Fatalf("two installation invocations were not counted exactly once: %+v, %v", state, err)
	}
}
