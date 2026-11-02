package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"

	carinatracker "github.com/agenticlab-ai/pyxis/internal/adapters/tracker/carina"
	"github.com/agenticlab-ai/pyxis/internal/executor/taskspec"
	"github.com/agenticlab-ai/pyxis/internal/plangate/plan"
	"github.com/agenticlab-ai/pyxis/internal/plangate/render"
	"github.com/agenticlab-ai/pyxis/internal/taskrequirements"
	"github.com/agenticlab-ai/pyxis/internal/taskrequirements/requirementstest"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

const nativeE2ERoutes = requirementstest.Input
const nativeE2EGoal = "The fixture output is committed."
const nativeE2EAcceptance = "reading the committed output returns the requested implementation text"
const nativeE2EPlanDir = "docs/plans/native-routing"

// Planning and task retrieval use real binaries, Carina and Git. PATH contains
// only those tools, proving authoring does not require an agent installation.
// This boundary deliberately stops before execution-time route selection.
func TestPlanningRequirementsEndToEnd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX Git fixture")
	}
	expected, err := taskrequirements.Parse([]byte(requirementstest.Input))
	if err != nil {
		t.Fatal(err)
	}
	expected, err = expected.Compile(taskrequirements.Sources(nil))
	if err != nil {
		t.Fatal(err)
	}
	source, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	bins := installedCLITools(t, "plangate", "carina")
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(git, filepath.Join(bins, "git")); err != nil {
		t.Fatal(err)
	}
	for _, producer := range []string{"plan", "direct"} {
		t.Run(producer, func(t *testing.T) {
			f := newPlanningE2EFixture(t, bins)
			store := filepath.Join(f.root, "carina")
			var item model.ItemView
			if producer == "plan" {
				seedNativeE2EPlan(t, f, source)
				var choices taskrequirements.DimensionValues
				decodeNativeE2E(t, nativeCLI(t, bins, f.repo, "plangate", "dimensions"), &choices)
				if !reflect.DeepEqual(choices, taskrequirements.Values()) {
					t.Fatal("lost enum values or policy")
				}
				nativeCLI(t, bins, f.repo, "plangate", "fold", nativeE2EPlanDir)
				nativeCLI(t, bins, f.repo, "plangate", "verify", nativeE2EPlanDir)
				nativeCLI(t, bins, f.repo, "plangate", "render", nativeE2EPlanDir)
				preview := string(readNativeE2EFile(t, filepath.Join(f.repo, render.ScratchDir, "plan-native-routing-001.html")))
				for _, want := range []string{"reasoning=complex", "coupling=composed", "novelty=adapt", "verification=composed", "impact_scope=multi_surface", "architectural_role=shared_substrate", "capability=strong", "deliberation=deep", "context=large", "assurance=elevated", "&lt;review&gt;"} {
					if !strings.Contains(preview, want) {
						t.Fatalf("preview missing %s", want)
					}
				}
				freezeNativeE2EPlan(f)
				projectNativeE2EPlan(t, bins, f, store, "--no-routing")
				if len(nativeE2EItems(t, bins, f, store, "ready")) != 0 {
					t.Fatal("authorized pickup before signal")
				}
				projectNativeE2EPlan(t, bins, f, store)
				before := nativeE2EItems(t, bins, f, store, "list")
				projectNativeE2EPlan(t, bins, f, store)
				assertNativeProjectionReplay(t, before, nativeE2EItems(t, bins, f, store, "list"))
				ready := nativeE2EItems(t, bins, f, store, "ready")
				if len(ready) != 1 {
					t.Fatalf("ready: %+v", ready)
				}
				item = ready[0]
			} else {
				input := writeNativeE2EInput(t, f, nativeE2ERoutes)
				decodeNativeE2E(t, nativeCLI(t, bins, f.repo, "carina", "create", "--root", store, "--native-task", input, "--json"), &item)
				if len(nativeE2EItems(t, bins, f, store, "ready")) != 0 {
					t.Fatal("create authorized pickup")
				}
				nativeCLI(t, bins, f.repo, "carina", "signal", "--root", store, "add", strconv.Itoa(item.ID), "agent")
			}
			var stored model.ItemView
			decodeNativeE2E(t, nativeCLI(t, bins, f.repo, "carina", "show", "--root", store, "--json", strconv.Itoa(item.ID)), &stored)
			var comments []string
			for _, entry := range stored.Log {
				if entry.Kind == "comment" {
					comments = append(comments, entry.Payload)
				}
			}
			spec, err := taskspec.ParseTask(stored.Body, comments)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(spec.Requirements, &expected) || spec.Goal != nativeE2EGoal {
				t.Fatalf("stored contract differs: %+v", spec)
			}
			if producer == "plan" {
				var folded plan.Doc
				decodeNativeE2E(t, readNativeE2EFile(t, filepath.Join(f.repo, nativeE2EPlanDir, "001-plan.json")), &folded)
				if !reflect.DeepEqual(folded.Tasks[0].Requirements, spec.Requirements) || spec.PlanFingerprint != folded.Fingerprint || spec.PlanTaskID != "native-routing/001/T1" {
					t.Fatal("projection lost frozen source")
				}
			}
			tracker := carinatracker.New(store, carinatracker.WithBinary(filepath.Join(bins, "carina")), carinatracker.WithRepository(e2eRepository))
			frontier, err := tracker.Frontier(t.Context())
			if err != nil || len(frontier) != 1 {
				t.Fatalf("frontier: %+v %v", frontier, err)
			}
			if !reflect.DeepEqual(frontier[0].Requirements, spec.Requirements) || frontier[0].NativeRouting != nil || frontier[0].DemandID == "" {
				t.Fatalf("readback lost requirements or selected a route: %+v", frontier[0])
			}
			assertNoPlanningE2EAgent(t, f)
		})
	}
	t.Run("invalid-producers", func(t *testing.T) {
		f := newPlanningE2EFixture(t, bins)
		store := filepath.Join(f.root, "carina")
		seedNativeE2EPlan(t, f, source)
		path := filepath.Join(f.repo, nativeE2EPlanDir, "001-plan.md")
		valid := string(readNativeE2EFile(t, path))
		for _, bad := range []string{strings.Replace(nativeE2ERoutes, "complex", "unknown", 1), strings.Replace(nativeE2ERoutes, `"coupling":"composed",`, "", 1), strings.Replace(nativeE2ERoutes, `"assessment":`, `"harness":"codex","assessment":`, 1), strings.Replace(nativeE2ERoutes, `"assessment":`, `"rung":2,"assessment":`, 1)} {
			writeNativeE2EFile(t, path, []byte(strings.Replace(valid, nativeE2ERoutes, bad, 1)))
			nativeCLIRefused(t, bins, f.repo, "plangate", 1, "fold", nativeE2EPlanDir)
			if _, err := os.Stat(filepath.Join(f.repo, nativeE2EPlanDir, "001-plan.json")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("invalid fold emitted output")
			}
			nativeCLIRefused(t, bins, f.repo, "carina", 1, "create", "--root", store, "--native-task", writeNativeE2EInput(t, f, bad), "--json")
			if len(nativeE2EItems(t, bins, f, store, "list")) != 0 {
				t.Fatal("invalid create wrote an item")
			}
		}
		assertNoPlanningE2EAgent(t, f)
	})
	t.Run("conflicting-readback", func(t *testing.T) {
		f := newPlanningE2EFixture(t, bins)
		store := filepath.Join(f.root, "carina")
		seedNativeE2EPlan(t, f, source)
		nativeCLI(t, bins, f.repo, "plangate", "fold", nativeE2EPlanDir)
		freezeNativeE2EPlan(f)
		projectNativeE2EPlan(t, bins, f, store, "--no-routing")
		var task model.ItemView
		for _, i := range nativeE2EItems(t, bins, f, store, "list") {
			if i.Type == "task" {
				task = i
			}
		}
		var payload string
		for _, e := range task.Log {
			if strings.Contains(e.Payload, taskspec.NativeMarkerPrefix) {
				payload = strings.Replace(e.Payload, "Behavior across components", "Changed judgment", 1)
			}
		}
		if task.ID == 0 || payload == "" {
			t.Fatal("missing task capsule")
		}
		nativeCLI(t, bins, f.repo, "carina", "comment", "--root", store, strconv.Itoa(task.ID), "--body", payload)
		nativeCLIRefused(t, bins, f.repo, "plangate", 1, "project", "--tracker", "carina", "--carina-root", store, nativeE2EPlanDir)
		if len(nativeE2EItems(t, bins, f, store, "ready")) != 0 {
			t.Fatal("conflicting task authorized")
		}
		nativeCLI(t, bins, f.repo, "carina", "signal", "--root", store, "add", strconv.Itoa(task.ID), "agent")
		tracker := carinatracker.New(store, carinatracker.WithBinary(filepath.Join(bins, "carina")), carinatracker.WithRepository(e2eRepository))
		if _, err := tracker.Frontier(t.Context()); err == nil || !strings.Contains(err.Error(), "conflicting") {
			t.Fatalf("conflicting carrier accepted: %v", err)
		}
		assertNoPlanningE2EAgent(t, f)
	})
}

func newPlanningE2EFixture(t *testing.T, bins string) *implementNowE2EFixture {
	t.Helper()
	f := newImplementNowE2EFixture(t, e2eFixtureOptions{})
	writeNativeE2EFile(t, filepath.Join(f.stateDir, "calls.jsonl"), nil)
	// Git installations may locate dashed subprocesses relative to argv[0].
	// Our isolated git symlink therefore needs the real Git helper directory,
	// including receive-pack/upload-pack for the local bare origin.
	gitHelpers := f.gitAt(f.repo, "--exec-path")
	t.Setenv("PATH", bins+string(os.PathListSeparator)+gitHelpers)
	writeNativeE2EFile(t, filepath.Join(f.repo, ".gitignore"), []byte("/.pyxis/logs/\n/.pyxis/scratch/\n/.pyxis/vendor/\n"))
	f.gitAt(f.repo, "add", ".gitignore")
	f.gitAt(f.repo, "commit", "-m", "test: isolate planning scratch")
	f.gitAt(f.repo, "push", "origin", "main")
	return f
}

func assertNoPlanningE2EAgent(t *testing.T, f *implementNowE2EFixture) {
	t.Helper()
	if calls := readE2ECalls(t, f.stateDir); len(calls) != 0 {
		t.Fatalf("planning invoked external agents/host: %+v", calls)
	}
}

func nativeCLI(t *testing.T, bins, dir, tool string, args ...string) []byte {
	t.Helper()
	out, stderr, err := runNativeE2ECommand(t, bins, dir, tool, args...)
	if err != nil {
		t.Fatalf("%s %v: %v\n%s\n%s", tool, args, err, out, stderr)
	}
	return out
}
func nativeCLIRefused(t *testing.T, bins, dir, tool string, wantExit int, args ...string) {
	t.Helper()
	out, stderr, err := runNativeE2ECommand(t, bins, dir, tool, args...)
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != wantExit {
		t.Fatalf("%s: want exit %d, got %v\n%s\n%s", tool, wantExit, err, out, stderr)
	}
}
func runNativeE2ECommand(t *testing.T, bins, dir, tool string, args ...string) ([]byte, []byte, error) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), filepath.Join(bins, tool), args...)
	cmd.Dir = dir
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	err := cmd.Run()
	return out.Bytes(), stderr.Bytes(), err
}
func nativeE2EItems(t *testing.T, bins string, f *implementNowE2EFixture, store, verb string) []model.ItemView {
	t.Helper()
	var view struct {
		Items []model.ItemView `json:"items"`
	}
	decodeNativeE2E(t, nativeCLI(t, bins, f.repo, "carina", verb, "--root", store, "--json"), &view)
	return view.Items
}
func projectNativeE2EPlan(t *testing.T, bins string, f *implementNowE2EFixture, store string, flags ...string) {
	t.Helper()
	args := append([]string{"project", "--tracker", "carina", "--carina-root", store}, flags...)
	nativeCLI(t, bins, f.repo, "plangate", append(args, nativeE2EPlanDir)...)
}
func assertNativeProjectionReplay(t *testing.T, before, after []model.ItemView) {
	t.Helper()
	if len(before) != len(after) {
		t.Fatal("projection duplicated items")
	}
	for i := range before {
		if before[i].Type == "feature" && after[i].Type == "feature" {
			if len(after[i].Log) != len(before[i].Log)+1 {
				t.Fatal("missing epic projection report")
			}
			before[i].Log, after[i].Log = nil, nil
		}
		before[i].Updated, after[i].Updated = "", ""
		if !reflect.DeepEqual(before[i], after[i]) {
			t.Fatalf("projection changed item %d", before[i].ID)
		}
	}
}
func freezeNativeE2EPlan(f *implementNowE2EFixture) {
	f.gitAt(f.repo, "add", "docs", "go.mod")
	f.gitAt(f.repo, "commit", "-m", "test: freeze dimensioned plan")
	f.gitAt(f.repo, "push", "origin", "main")
}
func writeNativeE2EInput(t *testing.T, f *implementNowE2EFixture, requirements string) string {
	t.Helper()
	input := fmt.Sprintf(`{"schema_version":"native-task-input/v1","title":"Add fixture","goal":%q,"scope":"output behavior","acceptance":[%q],"requirements":%s}`, nativeE2EGoal, nativeE2EAcceptance, requirements)
	path := filepath.Join(f.root, "task.json")
	writeNativeE2EFile(t, path, []byte(input))
	return path
}
func decodeNativeE2E(t *testing.T, data []byte, into any) {
	t.Helper()
	if err := json.Unmarshal(data, into); err != nil {
		t.Fatalf("decode %s: %v", data, err)
	}
}
func readNativeE2EFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func writeNativeE2EFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}
