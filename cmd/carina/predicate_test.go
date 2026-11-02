package main

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// buildCorpus creates the dispatch fixture through the CLI and returns the ids
// of the authorized-ready item and the planned (ready-not-triage) item. It is
// the same shape the port conformance kit builds, exercised here through the
// binary's own verbs.
func buildCorpus(t *testing.T, root string) (authorized, planned string) {
	t.Helper()
	authorized = createID(t, root, "task", "authorized leaf")
	sig(t, root, authorized)

	createID(t, root, "task", "unauthorized leaf") // #2 — triage-only

	blocked := createID(t, root, "task", "blocked leaf")
	sig(t, root, blocked)
	if _, errOut, code := exec("dep", "add", blocked, authorized, "--root", root); code != 0 {
		t.Fatalf("block: %s", errOut)
	}

	epic := createID(t, root, "epic", "the epic")
	sig(t, root, epic)

	rejected := createID(t, root, "task", "rejected leaf")
	sig(t, root, rejected)
	record(t, root, rejected, "reject")

	planned = createID(t, root, "task", "planned leaf")
	sig(t, root, planned)
	record(t, root, planned, "plan")
	return authorized, planned
}

func sig(t *testing.T, root, id string) {
	t.Helper()
	if _, errOut, code := exec("signal", "add", id, "agent", "--root", root); code != 0 {
		t.Fatalf("signal add agent #%s: %s", id, errOut)
	}
}

func record(t *testing.T, root, id, payload string) {
	t.Helper()
	if _, errOut, code := exec("record", id, "triage-disposition", "--body", payload, "--root", root); code != 0 {
		t.Fatalf("record %s #%s: %s", payload, id, errOut)
	}
}

// TestReadyVerbListsExpectedSet: the ready verb prints exactly the authorized,
// unblocked, non-rollup leaves with no closing disposition, in creation order
// (FR-12, CLI half of acceptance criterion 1).
func TestReadyVerbListsExpectedSet(t *testing.T) {
	root := t.TempDir()
	authorized, planned := buildCorpus(t, root)

	out, _, code := exec("ready", "--root", root, "--json")
	if code != 0 {
		t.Fatalf("ready exit = %d", code)
	}
	if got, want := setIDsJSON(t, out), []string{authorized, planned}; !equalStrs(got, want) {
		t.Fatalf("ready set = %v, want %v", got, want)
	}
}

// TestTriageVerbListsExpectedSet: the triage verb prints exactly the open,
// undecided, unblocked, unassigned non-rollups (FR-13, CLI half of acceptance
// criterion 2). The unauthorized leaf (#2) is in triage but not ready.
func TestTriageVerbListsExpectedSet(t *testing.T) {
	root := t.TempDir()
	authorized, _ := buildCorpus(t, root)
	if authorized != "1" {
		t.Fatalf("fixture id drift: authorized=%s", authorized)
	}
	const unauth = "2"

	out, _, code := exec("triage", "--root", root, "--json")
	if code != 0 {
		t.Fatalf("triage exit = %d", code)
	}
	if got, want := setIDsJSON(t, out), []string{authorized, unauth}; !equalStrs(got, want) {
		t.Fatalf("triage set = %v, want %v", got, want)
	}
}

// TestCLIAndPortByteIdentical: the CLI and the port return byte-identical set
// output for both predicates (FR-16/DINV-6, acceptance criterion 3). The CLI
// emits model.Set(...) marshaled; building the same value directly from the port
// over the same store must produce the identical bytes.
func TestCLIAndPortByteIdentical(t *testing.T) {
	root := t.TempDir()
	buildCorpus(t, root)

	store, defects := openStore(root)
	if len(defects) > 0 {
		t.Fatalf("open store: %v", defects)
	}
	ctx := context.Background()

	for _, which := range []string{"ready", "triage"} {
		items, ds := store.ReadySet(ctx)
		if which == "triage" {
			items, ds = store.TriageSet(ctx)
		}
		if len(ds) > 0 {
			t.Fatalf("port %s: %v", which, ds)
		}
		want, err := json.Marshal(model.Set(which, items))
		if err != nil {
			t.Fatal(err)
		}
		out, _, code := exec(which, "--root", root, "--json")
		if code != 0 {
			t.Fatalf("%s exit = %d", which, code)
		}
		if strings.TrimSpace(out) != string(want) {
			t.Fatalf("%s: CLI and port bytes differ:\nCLI:  %s\nport: %s", which, strings.TrimSpace(out), want)
		}
	}
}

// TestPredicateHumanOutput: the human form names the set size and each member;
// an empty set prints just the header.
func TestPredicateHumanOutput(t *testing.T) {
	root := t.TempDir()
	buildCorpus(t, root)
	out, _, code := exec("ready", "--root", root)
	if code != 0 || !strings.Contains(out, "ready set: 2") || !strings.Contains(out, "authorized leaf") {
		t.Fatalf("human ready output unexpected: %q", out)
	}

	empty := t.TempDir()
	createID(t, empty, "task", "not authorized") // no agent signal ⇒ empty ready set
	out, _, code = exec("ready", "--root", empty)
	if code != 0 || !strings.Contains(out, "ready set: 0") {
		t.Fatalf("empty ready output unexpected: %q", out)
	}
}

// setIDsJSON extracts the member ids (as strings) from a --json predicate set,
// in the order emitted.
func setIDsJSON(t *testing.T, out string) []string {
	t.Helper()
	var set struct {
		Items []struct {
			ID int `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &set); err != nil {
		t.Fatalf("set output not JSON: %v (%q)", err, out)
	}
	ids := make([]string, len(set.Items))
	for i, it := range set.Items {
		ids[i] = strconv.Itoa(it.ID)
	}
	return ids
}

func equalStrs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
