package contractgate

import (
	"strings"
	"testing"
)

// TestParseSeverityMapClosed table-drives the closed-schema parse (FR-13, UB-6):
// a well-formed policy parses to the intended split; every malformed shape —
// unknown top-level key, unknown trigger, unknown check class, unknown severity,
// missing schema_version, wrong schema_version, missing triggers section, and a
// structurally broken line — is a named defect (a configuration-error edge). On
// any defect the returned map is the built-in default (never trusted alongside a
// defect).
func TestParseSeverityMapClosed(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantErr   string // substring the first defect must contain ("" = must parse clean)
		checkFlip func(t *testing.T, m SeverityMap)
	}{
		{
			name: "a well-formed override parses",
			yaml: "schema_version: \"1\"\ntriggers:\n  contract-doc-pr:\n    coverage: blocking\n  planning-entry:\n    coverage: blocking\n",
			checkFlip: func(t *testing.T, m SeverityMap) {
				if m.Severity(TriggerContractDocPR, SeverityClassCoverage) != SeverityBlocking {
					t.Errorf("coverage on contract-doc-pr not promoted to blocking")
				}
				// An unstated key keeps its default (currency stays blocking).
				if m.Severity(TriggerContractDocPR, SeverityClassCurrency) != SeverityBlocking {
					t.Errorf("unstated currency did not inherit its default (blocking)")
				}
			},
		},
		{
			name: "a policy demoting a check to advisory parses",
			yaml: "schema_version: \"1\"\ntriggers:\n  planning-entry:\n    currency: advisory\n",
			checkFlip: func(t *testing.T, m SeverityMap) {
				if m.Severity(TriggerPlanningEntry, SeverityClassCurrency) != SeverityAdvisory {
					t.Errorf("currency was not demoted to advisory at planning-entry")
				}
			},
		},
		{
			name:    "an unknown top-level key fails closed",
			yaml:    "schema_version: \"1\"\ntriggers:\n  planning-entry:\n    coverage: advisory\nbogus:\n  x: y\n",
			wantErr: "unknown top-level key",
		},
		{
			name:    "an unknown trigger fails closed",
			yaml:    "schema_version: \"1\"\ntriggers:\n  ci-nightly:\n    coverage: advisory\n",
			wantErr: "unknown trigger",
		},
		{
			name:    "an unknown check class fails closed",
			yaml:    "schema_version: \"1\"\ntriggers:\n  contract-doc-pr:\n    coveragex: advisory\n",
			wantErr: "unknown check class",
		},
		{
			name:    "an unknown severity fails closed",
			yaml:    "schema_version: \"1\"\ntriggers:\n  contract-doc-pr:\n    coverage: warn\n",
			wantErr: "is not blocking or advisory",
		},
		{
			name:    "a missing schema_version fails closed",
			yaml:    "triggers:\n  contract-doc-pr:\n    coverage: advisory\n",
			wantErr: "no schema_version",
		},
		{
			name:    "a wrong schema_version fails closed",
			yaml:    "schema_version: \"2\"\ntriggers:\n  contract-doc-pr:\n    coverage: advisory\n",
			wantErr: "is not the pinned",
		},
		{
			name:    "a missing triggers section fails closed",
			yaml:    "schema_version: \"1\"\n",
			wantErr: "no triggers section",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m, defects := ParseSeverityMap([]byte(tc.yaml))
			if tc.wantErr == "" {
				if len(defects) != 0 {
					t.Fatalf("expected a clean parse, got defects: %v", defects)
				}
				if tc.checkFlip != nil {
					tc.checkFlip(t, m)
				}
				return
			}
			if len(defects) == 0 {
				t.Fatalf("expected a defect containing %q, got none", tc.wantErr)
			}
			found := false
			for _, d := range defects {
				if strings.Contains(d.Reason, tc.wantErr) {
					found = true
				}
				// Every defect names the severity-map file with a helpful remedy.
				if d.File != SeverityMapRelPath {
					t.Errorf("defect file = %q, want %q", d.File, SeverityMapRelPath)
				}
				if d.Remedy == "" {
					t.Errorf("defect carries no remedy: %+v", d)
				}
			}
			if !found {
				t.Errorf("no defect contained %q; defects: %v", tc.wantErr, defects)
			}
			// On any defect the returned map is the safe built-in default.
			assertDefaultSplit(t, m)
		})
	}
}

// TestDefaultSeverityMapIsTheD4Split pins the built-in default (the seed FR-13
// names): planning-entry every class blocking; contract-doc-pr all blocking
// except inverse-coverage advisory.
func TestDefaultSeverityMapIsTheD4Split(t *testing.T) {
	assertDefaultSplit(t, DefaultSeverityMap())
	// DefaultSeverityMap returns a FRESH map each call — mutating one must not
	// affect another (no shared-state leak).
	a := DefaultSeverityMap()
	a.byTrigger[TriggerContractDocPR][SeverityClassCoverage] = SeverityBlocking
	b := DefaultSeverityMap()
	if b.Severity(TriggerContractDocPR, SeverityClassCoverage) != SeverityAdvisory {
		t.Errorf("DefaultSeverityMap shares mutable state across calls")
	}
}

// TestSeverityMapCoversEveryRegisteredCheck proves the policy space and the
// registered check set agree (the closed-schema completeness guard): every
// contract-scope check declares exactly one SeverityClass, that class is in the
// closed severity-class set, and every closed severity-class is claimed by some
// check — so no check is un-dispositionable and no policy key is dead.
func TestSeverityMapCoversEveryRegisteredCheck(t *testing.T) {
	claimed := map[SeverityClass]string{}
	for _, chk := range contractScopeChecks() {
		if chk.SeverityClass == "" {
			t.Errorf("check %q declares no SeverityClass — it cannot be dispositioned by the severity map", chk.Name)
			continue
		}
		if !severityClasses[chk.SeverityClass] {
			t.Errorf("check %q declares severity class %q, not in the closed set", chk.Name, chk.SeverityClass)
		}
		if other, dup := claimed[chk.SeverityClass]; dup {
			t.Errorf("checks %q and %q share severity class %q — each check maps to its own class (EVT-2 taxonomy)", chk.Name, other, chk.SeverityClass)
		}
		claimed[chk.SeverityClass] = chk.Name
	}
	// Every closed severity class is claimed by a check (no dead policy key).
	for _, class := range severityMapClasses() {
		if _, ok := claimed[class]; !ok {
			t.Errorf("severity class %q is claimed by no registered check — a dead policy key", class)
		}
	}
	// And the default map covers every registered check at both triggers (a total
	// lookup — no check falls through to the implicit-blocking fallback).
	def := DefaultSeverityMap()
	for _, chk := range contractScopeChecks() {
		for trig := range triggers {
			if _, ok := def.byTrigger[trig][chk.SeverityClass]; !ok {
				t.Errorf("default map has no entry for check %q (class %q) at trigger %q", chk.Name, chk.SeverityClass, trig)
			}
		}
	}
}

// TestParseTrigger proves the CLI trigger flag parse: the two closed values, the
// empty default, and an unknown value rejected.
func TestParseTrigger(t *testing.T) {
	if trig, err := ParseTrigger(""); err != "" || trig != TriggerPlanningEntry {
		t.Errorf("empty trigger = (%q,%q), want (planning-entry,\"\")", trig, err)
	}
	if trig, err := ParseTrigger("planning-entry"); err != "" || trig != TriggerPlanningEntry {
		t.Errorf("planning-entry = (%q,%q)", trig, err)
	}
	if trig, err := ParseTrigger("contract-doc-pr"); err != "" || trig != TriggerContractDocPR {
		t.Errorf("contract-doc-pr = (%q,%q)", trig, err)
	}
	if _, err := ParseTrigger("nightly"); err == "" {
		t.Errorf("an unknown trigger was accepted")
	}
}
