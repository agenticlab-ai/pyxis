package plan

import "fmt"

// charter — FR-12 charter completeness + FR-7 mechanical quarantine.
func init() { RegisterTask(TaskCheck{Name: "charter", Run: checkCharter}) }

func checkCharter(_ *Env, t *Task, bad Bad) {
	if t.Mechanical {
		if len(t.Provenance) == 0 {
			bad(t.ID, "mechanical task without provenance", "a mechanical charter states the provenance to verify — upstream tag + checksum, or move evidence (PRD FR-7)")
		}
		if t.Band != (SizeBand{}) {
			bad(t.ID, "mechanical task with a non-zero size band", "mechanical work carries no logic to read: Size band is 0 (PRD FR-7)")
		}
		if t.Goal == "" {
			bad(t.ID, "empty goal", "state the one provenance-only change this task lands")
		}
		if len(t.Acceptance) == 0 {
			bad(t.ID, "no acceptance criteria", "every dispatched task carries a decidable done signal — for mechanical work, the provenance check itself (PRD FR-12)")
		}
		return
	}
	req := func(name, v string) {
		if v == "" {
			bad(t.ID, "empty "+name, "the charter field set is §4.5 layer 2 — every behavior task carries it (PRD FR-12)")
		}
	}
	req("Goal", t.Goal)
	req("Current behavior", t.Current)
	req("Desired behavior", t.Desired)
	if len(t.Scope) == 0 {
		bad(t.ID, "empty scope", "state the included work behaviorally (PRD FR-12)")
	}
	if len(t.OutOfScope) == 0 {
		bad(t.ID, "empty out-of-scope", "explicit boundaries stop gold-plating — `none — <reason>` is a legal item (PRD §4.5 row 10)")
	}
	if len(t.Acceptance) == 0 {
		bad(t.ID, "no acceptance criteria", "independently verifiable checks are the worker's done signal (PRD FR-12)")
	}
	if len(t.KeyInterfaces) == 0 {
		bad(t.ID, "no key interfaces", "name the durable types/contracts the worker looks for (PRD §4.5 row 12)")
	}
	if len(t.Provenance) > 0 {
		bad(t.ID, "behavior task carrying provenance", "provenance belongs to mechanical tasks; mixing is the quarantine breach (PRD FR-7)")
	}
	if t.Band.Lo < 1 || t.Band.Hi < t.Band.Lo {
		bad(t.ID, fmt.Sprintf("size band %d–%d invalid", t.Band.Lo, t.Band.Hi), "a behavior task estimates a positive lo–hi band (PRD FR-22)")
	}
}
