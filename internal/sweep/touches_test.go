package sweep_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/plangate/plan"
	"github.com/agenticlab-ai/pyxis/internal/sweep"
)

// The two never-shipped wiring surfaces the T4/T8 historical charter gap named,
// verbatim (brainstorm:maintenance-sweep-tech-design#F-1, fixture spec §D-5).
const (
	surfaceMergeLane = ".github/workflows/merge-lane.yml"
	surfaceWakeTick  = "scripts/wake-tick.sh"
)

// writeFile creates root/rel with its parents and the given content.
func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// writePlanFace marshals a plan/v1 machine face to
// root/docs/plans/<feature>/<version>-plan.json — the committed read contract.
func writePlanFace(t *testing.T, root, feature, version string, tasks []plan.Task) {
	t.Helper()
	doc := plan.Doc{
		Schema:  plan.SchemaVersion,
		Feature: feature,
		Version: version,
		Tasks:   tasks,
	}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal plan face: %v", err)
	}
	writeFile(t, root, "docs/plans/"+feature+"/"+version+"-plan.json", string(b))
}

// task is a terse plan.Task builder for the fixtures.
func task(id, title string, touches ...string) plan.Task {
	return plan.Task{ID: id, Title: title, Touches: touches}
}

// replayTree lays down the T4/T8 replay fixture: a frozen plan whose tasks name
// the two never-shipped wiring surfaces plus a delivered surface and a non-path
// token. delivered=true additionally ships the two wiring surfaces (the
// fully-delivered fixture). Returns the temp root.
func replayTree(t *testing.T, delivered bool) string {
	t.Helper()
	root := t.TempDir()
	writePlanFace(t, root, "argus-unattended-drive", "001", []plan.Task{
		// A delivered surface joined by the house middot to a never-shipped one.
		task("T4", "The acting lane", "internal/lane/acting · "+surfaceMergeLane),
		task("T8", "The wake tick", "internal/wake · "+surfaceWakeTick),
		// A non-path planning token — must be skipped, never interpreted.
		task("T11", "The pipeline map", "pipeline-map"),
	})
	// The delivered surfaces the tasks legitimately shipped.
	writeFile(t, root, "internal/lane/acting/acting.go", "package acting\n")
	writeFile(t, root, "internal/wake/wake.go", "package wake\n")
	if delivered {
		writeFile(t, root, surfaceMergeLane, "name: merge-lane\n")
		writeFile(t, root, surfaceWakeTick, "#!/usr/bin/env bash\n")
	}
	return root
}

// On the replay fixture the two never-shipped surfaces are reported citing plan
// and task in advisory phrasing; a fully-delivered plan yields zero
// (acceptance 1).
func TestTouchesReplayReportsNeverShippedSurfaces(t *testing.T) {
	root := replayTree(t, false)
	res := sweep.DetectTouchesUnderdelivery(root, sweep.Empty())

	if res.Fault != "" {
		t.Fatalf("unexpected fault: %s", res.Fault)
	}
	got := subjects(res)
	want := []string{surfaceMergeLane, surfaceWakeTick}
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("subjects = %v, want %v", got, want)
	}

	// Each finding cites plan and task and is framed as advisory, never a breach.
	for _, f := range res.Findings {
		if !strings.Contains(f.Evidence, "argus-unattended-drive/001") {
			t.Errorf("finding %q evidence does not cite the plan: %q", f.Subject, f.Evidence)
		}
		if f.Subject == surfaceMergeLane && !strings.Contains(f.Evidence, "T4") {
			t.Errorf("merge-lane finding does not cite task T4: %q", f.Evidence)
		}
		if f.Subject == surfaceWakeTick && !strings.Contains(f.Evidence, "T8") {
			t.Errorf("wake-tick finding does not cite task T8: %q", f.Evidence)
		}
		advisory := strings.ToLower(f.Evidence + " " + f.Remedy)
		if !strings.Contains(advisory, "advisory") || !strings.Contains(advisory, "reconcil") {
			t.Errorf("finding %q is not phrased as an advisory reconciliation signal: %q / %q", f.Subject, f.Evidence, f.Remedy)
		}
		for _, accusation := range []string{"violation", "breached", "must ship"} {
			if strings.Contains(advisory, accusation) {
				t.Errorf("finding %q reads as a breach verdict (%q): %q / %q", f.Subject, accusation, f.Evidence, f.Remedy)
			}
		}
	}
}

// A fully-delivered plan yields zero findings (acceptance 1).
func TestTouchesFullyDeliveredYieldsZero(t *testing.T) {
	root := replayTree(t, true)
	res := sweep.DetectTouchesUnderdelivery(root, sweep.Empty())
	if res.Fault != "" {
		t.Fatalf("unexpected fault: %s", res.Fault)
	}
	if len(res.Findings) != 0 {
		t.Fatalf("fully-delivered plan yielded %d findings: %v", len(res.Findings), subjects(res))
	}
}

// A non-path token appears in no finding (acceptance 2, second clause): even
// when it names nothing in the tree, "pipeline-map" is skipped, not reported.
func TestTouchesNonPathTokenNeverReported(t *testing.T) {
	root := replayTree(t, false)
	res := sweep.DetectTouchesUnderdelivery(root, sweep.Empty())
	for _, f := range res.Findings {
		if f.Subject == "pipeline-map" {
			t.Fatalf("non-path token reported as a finding: %q", f.Subject)
		}
	}
}

// A plan waiver suppresses that plan's findings, surfaces its reason, and its
// removal restores them (acceptance 2, first clause).
func TestTouchesPlanWaiverSuppressesAndSurfaces(t *testing.T) {
	root := replayTree(t, false)

	const reason = "installer plan freshly frozen and mid-execution; surfaces land as tasks close"
	pol := sweep.Empty()
	pol.PlanWaivers = []sweep.PlanWaiver{{Plan: "argus-unattended-drive/001", Reason: reason}}

	res := sweep.DetectTouchesUnderdelivery(root, pol)
	if len(res.Findings) != 0 {
		t.Fatalf("waived plan still emitted %d findings: %v", len(res.Findings), subjects(res))
	}
	if len(res.WaiversApplied) != 1 {
		t.Fatalf("applied-waiver count = %d, want 1", len(res.WaiversApplied))
	}
	w := res.WaiversApplied[0]
	if w.Kind != sweep.WaiverKindPlan || w.Subject != "argus-unattended-drive/001" || w.Reason != reason {
		t.Fatalf("applied waiver = %+v, want plan/argus-unattended-drive/001 with the recorded reason", w)
	}

	// Removing the waiver restores the two findings.
	restored := sweep.DetectTouchesUnderdelivery(root, sweep.Empty())
	if len(restored.Findings) != 2 {
		t.Fatalf("after waiver removal findings = %d, want 2", len(restored.Findings))
	}
	if len(restored.WaiversApplied) != 0 {
		t.Fatalf("no waiver defined but %d applied", len(restored.WaiversApplied))
	}
}

// A corrupted plan face yields a named skip while other plans' results are
// present (acceptance 3).
func TestTouchesCorruptPlanNamedSkip(t *testing.T) {
	root := replayTree(t, false) // the sound plan with its two findings

	// A second plan whose face is syntactically malformed JSON.
	writeFile(t, root, "docs/plans/corrupt-feature/001-plan.json", "{ this is not json ]")

	res := sweep.DetectTouchesUnderdelivery(root, sweep.Empty())

	// The sound plan's two findings survive.
	if len(res.Findings) != 2 {
		t.Fatalf("sound plan's findings = %d, want 2 (corrupt plan must not suppress them): %v", len(res.Findings), subjects(res))
	}
	// The corrupt plan is named in the in-band skip.
	if !strings.Contains(res.Fault, "corrupt-feature/001-plan.json") {
		t.Fatalf("corrupt plan not named in the skip: %q", res.Fault)
	}
}

// Every emitted finding carries all four contracted fields (acceptance 4).
func TestTouchesFindingsCarryFourFields(t *testing.T) {
	root := replayTree(t, false)
	res := sweep.DetectTouchesUnderdelivery(root, sweep.Empty())
	if len(res.Findings) == 0 {
		t.Fatal("expected findings to check the four-field contract")
	}
	for _, f := range res.Findings {
		if f.Detector != sweep.DetectorTouchesUnderdelivery {
			t.Errorf("finding %q detector = %q", f.Subject, f.Detector)
		}
		if strings.TrimSpace(f.Subject) == "" || strings.TrimSpace(f.Evidence) == "" || strings.TrimSpace(f.Remedy) == "" {
			t.Errorf("finding missing a contracted field: %+v", f)
		}
		if f.SchemaVersion != sweep.FindingSchemaVersion {
			t.Errorf("finding %q schema = %q", f.Subject, f.SchemaVersion)
		}
	}
}

// An absent plans home is the legal empty state — no plans, no findings, no
// fault (the detector is advisory).
func TestTouchesAbsentPlansHomeIsEmpty(t *testing.T) {
	root := t.TempDir()
	res := sweep.DetectTouchesUnderdelivery(root, sweep.Empty())
	if res.Fault != "" || len(res.Findings) != 0 || len(res.WaiversApplied) != 0 {
		t.Fatalf("absent plans home not empty: %+v", res)
	}
}

// The detector is deterministic: two runs on the same tree yield identical
// findings (DINV-2 — committed state only, no wall clock, no environment leak).
func TestTouchesDeterministic(t *testing.T) {
	root := replayTree(t, false)
	a := sweep.DetectTouchesUnderdelivery(root, sweep.Empty())
	b := sweep.DetectTouchesUnderdelivery(root, sweep.Empty())
	if strings.Join(subjects(a), ",") != strings.Join(subjects(b), ",") {
		t.Fatalf("nondeterministic subjects: %v vs %v", subjects(a), subjects(b))
	}
}
