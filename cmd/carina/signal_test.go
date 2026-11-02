package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func mustExec(t *testing.T, args ...string) string {
	t.Helper()
	out, errOut, code := exec(args...)
	if code != 0 {
		t.Fatalf("%v failed: code=%d stderr=%s", args, code, errOut)
	}
	return out
}

func decodeInto(t *testing.T, s string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(strings.TrimSpace(s)), v); err != nil {
		t.Fatalf("not JSON: %v (%q)", err, s)
	}
}

func countAny(xs []any, v string) int {
	n := 0
	for _, x := range xs {
		if s, _ := x.(string); s == v {
			n++
		}
	}
	return n
}

// The seed vocabulary round-trips through the CLI and every signal reports its
// role (acceptance criterion 1).
func TestSignalLsSeedVocabulary(t *testing.T) {
	root := t.TempDir()
	out, errOut, code := exec("signal", "ls", "--root", root, "--json")
	if code != 0 {
		t.Fatalf("signal ls failed: code=%d stderr=%s", code, errOut)
	}
	var v struct {
		Schema  string `json:"schema"`
		Signals []struct {
			Name string `json:"name"`
			Role string `json:"role"`
		} `json:"signals"`
	}
	decodeInto(t, out, &v)
	if v.Schema != "worktracker.signal-vocabulary/v1" {
		t.Fatalf("vocabulary schema tag = %q", v.Schema)
	}
	roles := map[string]bool{"routing": true, "authorization": true, "status": true, "classification": true}
	seen := map[string]string{}
	for _, s := range v.Signals {
		if !roles[s.Role] {
			t.Fatalf("signal %q reports out-of-set role %q", s.Name, s.Role)
		}
		seen[s.Name] = s.Role
	}
	if seen["agent"] != "authorization" {
		t.Fatalf("seed signal agent role = %q, want authorization", seen["agent"])
	}
	if seen["needs-plan"] != "routing" {
		t.Fatalf("seed signal needs-plan role = %q, want routing", seen["needs-plan"])
	}
}

// Adding a present signal and removing an absent one both succeed as recorded
// no-ops (acceptance criterion 2).
func TestSignalAddRemoveIdempotent(t *testing.T) {
	root := t.TempDir()
	mustExec(t, "create", "--root", root, "--title", "x", "--json")

	// Real add.
	if _, _, code := exec("signal", "add", "--root", root, "1", "agent"); code != 0 {
		t.Fatalf("add exit = %d", code)
	}
	// Adding the present signal again succeeds (recorded no-op).
	if _, errOut, code := exec("signal", "add", "--root", root, "1", "agent"); code != 0 {
		t.Fatalf("re-add exit = %d stderr=%s", code, errOut)
	}
	// The item carries the signal exactly once.
	show, _, _ := exec("show", "--root", root, "1", "--json")
	m := decode(t, show)
	sigs, _ := m["signals"].([]any)
	if n := countAny(sigs, "agent"); n != 1 {
		t.Fatalf("signal count = %d, want 1: %v", n, sigs)
	}
	// Removing an absent signal succeeds (recorded no-op).
	if _, errOut, code := exec("signal", "rm", "--root", root, "1", "hold"); code != 0 {
		t.Fatalf("rm-absent exit = %d stderr=%s", code, errOut)
	}
	// Real remove.
	if _, _, code := exec("signal", "rm", "--root", root, "1", "agent"); code != 0 {
		t.Fatalf("rm exit = %d", code)
	}
	if _, _, code := exec("signal", "rm", "--root", root, "1", "agent"); code != 0 {
		t.Fatalf("rm-again exit = %d", code)
	}
}

// An undefined signal and an engine-recognized redefinition are each refused
// with a defect naming the offense (acceptance criterion 3).
func TestSignalRefusals(t *testing.T) {
	root := t.TempDir()
	mustExec(t, "create", "--root", root, "--title", "x", "--json")

	// Undefined signal add.
	_, errOut, code := exec("signal", "add", "--root", root, "--json", "1", "no-such-signal")
	if code != 1 {
		t.Fatalf("undefined-signal add exit = %d, want 1", code)
	}
	d := decode(t, errOut)
	if !strings.Contains(d["reason"].(string), "no-such-signal") {
		t.Fatalf("defect does not name the offense: %s", errOut)
	}
	for _, k := range []string{"file", "op", "reason", "remedy"} {
		if s, _ := d[k].(string); s == "" {
			t.Errorf("undefined-signal defect missing %q: %s", k, errOut)
		}
	}

	// Engine-recognized redefinition.
	_, errOut, code = exec("signal", "def", "--root", root, "--json", "agent", "classification")
	if code != 1 {
		t.Fatalf("redefinition exit = %d, want 1", code)
	}
	d = decode(t, errOut)
	if !strings.Contains(d["reason"].(string), "agent") || !strings.Contains(d["reason"].(string), "redefin") {
		t.Fatalf("redefinition defect does not name the offense: %s", errOut)
	}
}

// An operand may define a new classification signal and then use it.
func TestSignalDefineClassification(t *testing.T) {
	root := t.TempDir()
	mustExec(t, "create", "--root", root, "--title", "x", "--json")
	if _, errOut, code := exec("signal", "def", "--root", root, "kind:security", "classification", "--description", "a security change"); code != 0 {
		t.Fatalf("def exit = %d stderr=%s", code, errOut)
	}
	if _, errOut, code := exec("signal", "add", "--root", root, "1", "kind:security"); code != 0 {
		t.Fatalf("add of defined signal exit = %d stderr=%s", code, errOut)
	}
}
