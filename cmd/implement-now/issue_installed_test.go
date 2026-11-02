package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/issueentry"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewobligation"
	"github.com/agenticlab-ai/pyxis/internal/executor/taskspec"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/taskrequirements"
	"github.com/agenticlab-ai/pyxis/internal/taskrequirements/requirementstest"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

func directIssueFixture(t *testing.T) *implementNowE2EFixture {
	t.Helper()
	f := newImplementNowE2EFixture(t, e2eFixtureOptions{Review: true})
	f.cfg.Capsule = ""
	f.cfg.TaskTitle = "Produce the fixture output"
	writeJSONFile(t, f.config, f.cfg)
	writeJSONFile(t, filepath.Join(f.repo, ".pyxis", "e2e-stub-config.json"), f.cfg)
	// The installed command resolves its catalog from the operated repository;
	// it cannot borrow the source checkout's configured model profiles.
	const catalog = `harness-config:
  codex:
    implementation:
      fixture-implementation:
        model: gpt-5.6-sol
        effort: xhigh
        capability: frontier
        deliberation: maximum
        context: extended
        assurance: critical
        cost_rank: 1
        strongest: true
    review:
      fixture-review:
        model: gpt-5.6-sol
        effort: xhigh
        capability: frontier
        deliberation: maximum
        context: extended
        assurance: critical
        cost_rank: 1
        strongest: true
`
	appendDirectIssueConfig(t, f, catalog+"code-gate:\n  fixture-output:\n    command: git show HEAD:e2e-output.txt\n")
	return f
}

func TestDirectIssueInstalledRecoversInterruptedPublication(t *testing.T) {
	bins := installedCLITools(t, "implement-now")
	for _, failure := range []string{"proposal-receipt-unavailable", "checks-unavailable"} {
		t.Run(failure, func(t *testing.T) {
			f := directIssueFixture(t)
			number := strconv.Itoa(f.cfg.IssueNumber)
			inspection := nativeCLI(t, bins, f.repo, "implement-now", append(f.args(), "--inspect-issue", number)...)
			input := issueInputFile(t, f, inspection)
			marker := filepath.Join(f.stateDir, failure)
			if err := os.WriteFile(marker, nil, 0600); err != nil {
				t.Fatal(err)
			}
			args := append(f.args(), "--issue", number, "--issue-input", input)
			_, diagnostic, err := runNativeE2ECommand(t, bins, f.repo, "implement-now", args...)
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 1 {
				t.Fatalf("publication interruption: %v %s", err, diagnostic)
			}
			pr, exists, err := readE2EPRState(f.cfg)
			if err != nil || !exists {
				t.Fatalf("interruption did not retain the remote PR: %+v %v", pr, err)
			}
			store, err := issueentry.NewStore(f.repo, "origin")
			if err != nil {
				t.Fatal(err)
			}
			binding := integrations.ResolvedRepository{Provider: integrations.GitHub, BaseURL: "https://github.com", Repository: f.cfg.Repository}
			contract, admitted, err := store.Load(t.Context(), "github", binding.Key(), f.cfg.IssueNumber)
			if err != nil || !admitted {
				t.Fatalf("admitted authority lost: %+v %v", contract, err)
			}
			_, associated, err := store.Proposal(t.Context(), contract)
			if err != nil || associated != (failure == "checks-unavailable") {
				t.Fatalf("wrong interruption boundary: association=%v error=%v", associated, err)
			}
			reviews, err := reviewobligation.NewGitStore(f.repo, "origin", reviewobligation.DefaultEvidenceRef)
			if err != nil {
				t.Fatal(err)
			}
			rows, err := reviews.Open(t.Context(), "issue/"+strings.TrimPrefix(contract.ID, "sha256:"))
			if err != nil || len(rows) != 0 {
				t.Fatalf("fixture already captured a review obligation: %+v %v", rows, err)
			}
			before := readE2ECalls(t, f.stateDir)
			implementation := codexLaunchesWithArg(before, "--dangerously-bypass-approvals-and-sandbox")
			if len(implementation) != 1 || len(codexLaunchesWithArg(before, "danger-full-access")) != 0 {
				t.Fatal("fixture did not interrupt between implementation and review")
			}
			// Recovery must preserve the original worktree's WIP and use the
			// admitted contract even if the live tracker has since changed.
			wip := filepath.Join(implementation[0].Dir, "README.md")
			if err := os.WriteFile(wip, []byte("unrelated uncommitted work\n"), 0600); err != nil {
				t.Fatal(err)
			}
			f.cfg.TaskTitle, f.cfg.TaskBody = "Changed after publication", "A different scope"
			writeJSONFile(t, f.config, f.cfg)
			fresh := filepath.Join(f.root, "recovery-clone")
			f.gitAt(f.root, "clone", f.origin, fresh)
			args = append(args, "--repo", fresh, "--journal", filepath.Join(fresh, defaultJournalRel))
			if err := os.Remove(marker); err != nil {
				t.Fatal(err)
			}
			output := nativeCLI(t, bins, fresh, "implement-now", args...)
			if !bytes.Contains(output, []byte("verified_pr #77")) {
				t.Fatalf("no recovered verified terminal: %s", output)
			}
			after := readE2ECalls(t, f.stateDir)
			if len(codexLaunchesWithArg(after, "--dangerously-bypass-approvals-and-sandbox")) != 1 || len(codexLaunchesWithArg(after, "danger-full-access")) != 1 {
				t.Fatal("recovery must launch only the missing independent review")
			}
			if got := githubCallsContaining(after, "POST", "repos/"+f.cfg.Repository+"/pulls"); len(got) != 1 {
				t.Fatalf("recovery created another PR: %+v", got)
			}
			if got := githubCallsContaining(after[len(before):], "GET", "repos/"+f.cfg.Repository+"/issues/"+number); len(got) != 0 {
				t.Fatalf("recovery consulted changed live source: %+v", got)
			}
			if contents, err := os.ReadFile(wip); err != nil || string(contents) != "unrelated uncommitted work\n" {
				t.Fatalf("recovery altered existing work: %q %v", contents, err)
			}
			nativeCLI(t, bins, fresh, "implement-now", args...)
			if len(codexLaunchesWithArg(readE2ECalls(t, f.stateDir), "exec")) != 2 {
				t.Fatal("recovered terminal was not replayable")
			}
		})
	}
}

func appendDirectIssueConfig(t *testing.T, f *implementNowE2EFixture, addition string) {
	t.Helper()
	path := filepath.Join(f.repo, ".pyxis", "config.yaml")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(b, []byte(addition)...), 0600); err != nil {
		t.Fatal(err)
	}
	f.gitAt(f.repo, "add", ".pyxis/config.yaml", ".pyxis/e2e-stub-config.json")
	f.gitAt(f.repo, "commit", "-m", "test: configure direct issue fixture")
	f.gitAt(f.repo, "push", "origin", "main")
}

func issueInputFile(t *testing.T, f *implementNowE2EFixture, inspection []byte) string {
	t.Helper()
	var inspected issueentry.Inspection
	if err := json.Unmarshal(inspection, &inspected); err != nil {
		t.Fatalf("inspection: %v %s", err, inspection)
	}
	requirements, err := taskrequirements.Parse([]byte(requirementstest.Input))
	if err != nil {
		t.Fatal(err)
	}
	input := issueentry.Input{Schema: issueentry.InputSchema, Source: inspected.Source, NativeInput: taskspec.NativeInput{Title: f.cfg.TaskTitle, Goal: "Produce e2e-output.txt", Scope: "Add the fixture output file", Acceptance: []string{"e2e-output.txt contains implemented by the stub harness"}, Requirements: requirements}}
	path := filepath.Join(f.root, "issue-input.json")
	writeJSONFile(t, path, input)
	return path
}

// This invokes the built binary, the real Carina CLI, native HTTP tracker,
// registered harnesses and authenticated review broker against a local Git
// remote. Only the external model and GitHub service are controlled doubles.
func TestDirectIssueInstalledCLIWithCarinaAndForgejoSources(t *testing.T) {
	bins := installedCLITools(t, "implement-now", "carina")
	for _, provider := range []string{"carina", "forgejo"} {
		t.Run(provider, func(t *testing.T) {
			f := directIssueFixture(t)
			args := f.args()
			if provider == "carina" {
				root := filepath.Join(f.root, "carina-store")
				var selected, blocker model.ItemView
				decodeNativeE2E(t, nativeCLI(t, bins, f.repo, "carina", "create", "--root", root, "--title", f.cfg.TaskTitle, "--body", f.cfg.TaskBody, "--json"), &selected)
				decodeNativeE2E(t, nativeCLI(t, bins, f.repo, "carina", "create", "--root", root, "--title", "Outside blocker", "--json"), &blocker)
				nativeCLI(t, bins, f.repo, "carina", "dep", "--root", root, "add", strconv.Itoa(selected.ID), strconv.Itoa(blocker.ID))
				if ready := nativeE2EItems(t, bins, f, root, "ready"); len(ready) != 0 {
					t.Fatalf("unlabelled issue entered ordinary ready set: %+v", ready)
				}
				f.cfg.IssueNumber = selected.ID
				writeJSONFile(t, f.config, f.cfg)
				args = append(args, "--tracker", "carina", "--carina-root", root, "--carina-bin", filepath.Join(bins, "carina"))
				relativeArgs := append(append([]string{}, args...), "--carina-root", "../carina-store", "--inspect-issue", strconv.Itoa(selected.ID))
				relative := nativeCLI(t, bins, f.root, "implement-now", relativeArgs...)
				var inspected issueentry.Inspection
				decodeNativeE2E(t, relative, &inspected)
				if inspected.Source.BindingDigest != issueentry.BindingDigest("carina:"+root) {
					t.Fatal("relative Carina root depended on invocation directory")
				}
				inspection := nativeCLI(t, bins, f.repo, "implement-now", append(args, "--inspect-issue", strconv.Itoa(selected.ID))...)
				input := issueInputFile(t, f, inspection)
				blockedArgs := append(append([]string{}, args...), "--issue", strconv.Itoa(selected.ID), "--issue-input", input)
				_, diagnostic, err := runNativeE2ECommand(t, bins, f.repo, "implement-now", blockedArgs...)
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != 2 || !strings.Contains(string(diagnostic), "non-executable") {
					t.Fatalf("open blocker admitted: %v %s", err, diagnostic)
				}
				if launches := codexLaunchesWithArg(readE2ECalls(t, f.stateDir), "exec"); len(launches) != 0 {
					t.Fatal("blocked issue spent a model call")
				}
				nativeCLI(t, bins, f.repo, "carina", "close", "--root", root, strconv.Itoa(blocker.ID), "--reason", "completed")
			} else {
				t.Setenv("PYXIS_DIRECT_ISSUE_TASK_TOKEN", "fixture-task-token")
				selectedPath := "/api/v1/repos/owner/tasks/issues/" + strconv.Itoa(f.cfg.IssueNumber)
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Header.Get("Authorization") != "token fixture-task-token" {
						t.Error("wrong task credential")
						w.WriteHeader(http.StatusForbidden)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					var value any
					switch r.Method + " " + r.URL.Path {
					case "GET " + selectedPath:
						value = map[string]any{"number": f.cfg.IssueNumber, "state": "open", "title": f.cfg.TaskTitle, "body": f.cfg.TaskBody}
					case "GET " + selectedPath + "/comments":
						value = []any{}
					case "GET " + selectedPath + "/dependencies":
						value = []any{map[string]int{"number": 9}}
						if page := r.URL.Query().Get("page"); page != "" && page != "1" {
							value = []any{}
						}
					case "GET /api/v1/repos/owner/tasks/issues/9":
						value = map[string]any{"number": 9, "state": "closed"}
					case "POST " + selectedPath + "/comments":
						value = map[string]int{"id": 3}
					default:
						t.Errorf("unselected task API: %s %s", r.Method, r.URL)
						w.WriteHeader(http.StatusNotFound)
						return
					}
					if err := json.NewEncoder(w).Encode(value); err != nil {
						t.Error(err)
					}
				}))
				defer server.Close()
				appendDirectIssueConfig(t, f, fmt.Sprintf("integrations:\n  schema_version: pyxis-integrations/v1\n  connections:\n    tasks:\n      provider: forgejo\n      base_url: %s\n      token_env: PYXIS_DIRECT_ISSUE_TASK_TOKEN\n  bindings:\n    tasks:\n      connection: tasks\n      repository: owner/tasks\n", server.URL))
			}
			number := strconv.Itoa(f.cfg.IssueNumber)
			inspection := nativeCLI(t, bins, f.repo, "implement-now", append(append([]string{}, args...), "--inspect-issue", number)...)
			input := issueInputFile(t, f, inspection)
			args = append(args, "--issue", number, "--issue-input", input)
			output := nativeCLI(t, bins, f.repo, "implement-now", args...)
			if !bytes.Contains(output, []byte("verified_pr #77")) {
				t.Fatalf("no verified installed terminal: %s", output)
			}
			pr, ok, err := readE2EPRState(f.cfg)
			if err != nil || !ok {
				t.Fatalf("proposal: %+v %v", pr, err)
			}
			if strings.Contains(pr.Body, f.root) || strings.Contains(pr.Body, "Closes #") {
				t.Fatalf("public body leaked a local path or closed an issue on the wrong host: %s", pr.Body)
			}
			assertIssueNativeUsage(t, f, pr.Body)
			nativeCLI(t, bins, f.repo, "implement-now", args...)
			if len(codexLaunchesWithArg(readE2ECalls(t, f.stateDir), "exec")) != 2 {
				t.Fatal("installed retry redispatched")
			}
		})
	}
}

func assertIssueNativeUsage(t *testing.T, f *implementNowE2EFixture, body string) {
	t.Helper()
	_, section, ok := strings.Cut(body, "<!-- pyxis:issue-execution-summary/v1 -->")
	if !ok {
		t.Fatal("missing execution section")
	}
	_, wire, ok := strings.Cut(section, "```json\n")
	if !ok {
		t.Fatal("missing execution JSON")
	}
	wire, _, ok = strings.Cut(wire, "\n```")
	if !ok {
		t.Fatal("unterminated execution JSON")
	}
	summary, err := issueentry.DecodeSummary([]byte(wire))
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.Attempts) != 2 || summary.TotalTokens.Value == nil || *summary.TotalTokens.Value != 48 || summary.TotalTokens.State != "observed" {
		t.Fatalf("native usage not counted once per launch: %+v", summary)
	}
	calls := codexLaunchesWithArg(readE2ECalls(t, f.stateDir), "exec")
	for i, call := range calls {
		attempt := summary.Attempts[i]
		if attempt.Harness != "codex" || !containsArg(call.Args, attempt.Model) || !containsArg(call.Args, "model_reasoning_effort="+strconv.Quote(attempt.Effort)) {
			t.Fatalf("attribution differs from launched argv: %+v %v", attempt, call.Args)
		}
	}
}
