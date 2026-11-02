package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/taskspec"
	"github.com/agenticlab-ai/pyxis/internal/taskrequirements/requirementstest"
)

const nativeInput = `{"schema_version":"native-task-input/v1","title":"Implement native widget","goal":"Widget responds","scope":"Widget response behavior","acceptance":["A missing widget yields a named miss"],"requirements":` + requirementstest.Input + `}`

func TestNativeCreateShowReady(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "native.json")
	if err := os.WriteFile(input, []byte(nativeInput), 0600); err != nil {
		t.Fatal(err)
	}
	out, errOut, code := exec("create", "--root", root, "--native-task", input, "--json")
	if code != 0 {
		t.Fatalf("create: %s", errOut)
	}
	created := decode(t, out)
	id := strconv.Itoa(int(created["id"].(float64)))
	body := created["body"].(string)
	s, err := taskspec.ParseTask(body, nil)
	if err != nil {
		t.Fatal(err)
	}
	if s.Requirements == nil || !strings.HasPrefix(s.PlanTaskID, "adhoc/") {
		t.Fatalf("not a native task: %+v", s)
	}
	if s.Requirements.Implementation.Assessment.Reasoning != "complex" || s.Requirements.Review.Profile.Deliberation != "maximum" {
		t.Fatal("task dimensions changed")
	}
	shown, errOut, code := exec("show", "--root", root, "--json", id)
	if code != 0 || decode(t, shown)["body"] != body {
		t.Fatalf("stored task differs: %s", errOut)
	}
	ready, _, code := exec("ready", "--root", root, "--json")
	if code != 0 || len(setIDsJSON(t, ready)) != 0 {
		t.Fatal("creation authorized execution without an agent signal")
	}
	sig(t, root, id)
	ready, errOut, code = exec("ready", "--root", root, "--json")
	if code != 0 || len(setIDsJSON(t, ready)) != 1 {
		t.Fatalf("complete task not ready: %s", errOut)
	}
	if _, err := s.RoutingData(taskspec.DemandIdentity{Repository: "o/r", Task: s.TaskIdentity("")}); err != nil {
		t.Fatal(err)
	}
}

func TestNativeCreateRejectsInvalidInputBeforeWriting(t *testing.T) {
	for _, input := range []string{
		strings.Replace(nativeInput, "complex", "invented-reasoning", 1),
		strings.Replace(nativeInput, `"coupling":"composed"`, `"coupling":"invalid"`, 1),
		strings.Replace(nativeInput, `"title":`, `"rung":2,"title":`, 1),
		strings.Replace(nativeInput, "Widget responds", "", 1),
		strings.Replace(nativeInput, "A missing widget yields a named miss", "", 1),
		`{`,
		strings.Replace(nativeInput, "Widget responds", "<!-- fs-spec:v3 {} -->", 1),
	} {
		root := t.TempDir()
		path := filepath.Join(root, "input.json")
		if err := os.WriteFile(path, []byte(input), 0600); err != nil {
			t.Fatal(err)
		}
		if _, _, code := exec("create", "--root", root, "--native-task", path); code == 0 {
			t.Fatal("accepted invalid task")
		}
		items, _, code := exec("list", "--root", root, "--json")
		if code != 0 || len(setIDsJSON(t, items)) != 0 {
			t.Fatal("invalid native create wrote an item")
		}
	}
}

func TestNativeCreateFlagsAreUnambiguous(t *testing.T) {
	if _, err := nativeCreateSpec(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("accepted missing input file")
	}
	for _, args := range [][]string{
		{"create", "--native-task", "", "--title", "override"},
		{"create", "--native-task", "missing.json", "--title", "override"},
		{"edit", "1", "--native-task", "missing.json"},
		{"create", "--native-task", "missing.json", "--type", "epic"},
	} {
		if _, _, code := exec(append(args, "--root", t.TempDir())...); code != 2 {
			t.Fatalf("accepted ambiguous flags %v", args)
		}
	}
}
