package preflight_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/preflight"
)

// stubTools pins PATH to a directory holding exactly the named stubs, each
// printing the given `--version` line (the doctor-fixture pattern, with
// output control added so the floor leg is exercisable).
func stubTools(t *testing.T, tools map[string]string) {
	t.Helper()
	dir := t.TempDir()
	for name, version := range tools {
		body := "#!/bin/sh\necho \"" + version + "\"\n"
		if version == "" {
			body = "#!/bin/sh\nexit 0\n"
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
			t.Fatalf("write stub %s: %v", name, err)
		}
	}
	t.Setenv("PATH", dir)
}

func rowByName(t *testing.T, rows []preflight.PrereqTool, name string) preflight.PrereqTool {
	t.Helper()
	for _, r := range rows {
		if r.Name == name {
			return r
		}
	}
	t.Fatalf("no %q row in %+v", name, rows)
	return preflight.PrereqTool{}
}

// ghKnobs resolves to at least one gh-transport operation (tracker=github
// declares ensure-labels + the welcome item), carinaKnobs to none — the
// applicability pair the gh row keys off.
func ghKnobs() config.Knobs {
	return config.Knobs{Tracker: "github", Wiring: config.Wiring{Repo: "use", Protection: "none"}}
}

func carinaKnobs() config.Knobs {
	return config.Knobs{Tracker: "carina", Wiring: config.Wiring{Repo: "use", Protection: "none"}}
}

func TestPrerequisitesAllPresentAtFloor(t *testing.T) {
	stubTools(t, map[string]string{
		"git": "git version 99.0.0", "gh": "gh version 99.0.0",
	})
	rows := preflight.Prerequisites(context.Background(), ghKnobs(), nil)
	if len(rows) != 2 {
		t.Fatalf("rows = %+v, want git+gh (GitHub-mode knobs)", rows)
	}
	for _, name := range []string{"git", "gh"} {
		r := rowByName(t, rows, name)
		if !r.Present || r.BelowFloor {
			t.Errorf("%s row = %+v, want present and at floor", name, r)
		}
	}
}

func TestPrerequisitesAbsentToolRowsNotPresent(t *testing.T) {
	stubTools(t, nil) // git absent
	rows := preflight.Prerequisites(context.Background(), carinaKnobs(), nil)
	if len(rows) != 1 {
		t.Fatalf("rows = %+v, want Git only (carina knobs: no gh row)", rows)
	}
	if r := rowByName(t, rows, "git"); r.Present {
		t.Errorf("git row = %+v, want not present", r)
	}
}

func TestPrerequisitesBelowFloorIsReported(t *testing.T) {
	stubTools(t, map[string]string{
		"git": "git version 0.0.1",
	})
	rows := preflight.Prerequisites(context.Background(), carinaKnobs(), nil)
	r := rowByName(t, rows, "git")
	if !r.Present || !r.BelowFloor || r.Version != "0.0.1" || r.Floor == "" {
		t.Errorf("git row = %+v, want present + below-floor with version 0.0.1 and the recorded floor", r)
	}
}

// TestPrerequisitesUnparseableVersionStaysPresentOnly is the tolerant
// post-install posture: a stub printing nothing (the doctor-fixture shape)
// must yield a present row with the floor question unanswered — never a
// failure, never a below-floor claim.
func TestPrerequisitesUnparseableVersionStaysPresentOnly(t *testing.T) {
	stubTools(t, map[string]string{"git": ""})
	rows := preflight.Prerequisites(context.Background(), carinaKnobs(), nil)
	r := rowByName(t, rows, "git")
	if !r.Present || r.BelowFloor || r.Version != "" {
		t.Errorf("git row = %+v, want present-only with no version claim", r)
	}
}

func TestPrerequisitesGHRowOnlyWhenOpsImplied(t *testing.T) {
	stubTools(t, map[string]string{"git": "git version 99.0.0"})
	rows := preflight.Prerequisites(context.Background(), ghKnobs(), nil)
	if r := rowByName(t, rows, "gh"); r.Present {
		t.Errorf("gh row = %+v, want a not-present row (gh absent from the stub PATH)", r)
	}
	for _, r := range preflight.Prerequisites(context.Background(), carinaKnobs(), nil) {
		if r.Name == "gh" {
			t.Errorf("carina knobs must yield no gh row, got %+v", r)
		}
	}
}
