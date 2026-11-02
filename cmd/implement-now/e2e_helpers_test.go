package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/implreview/pack"
	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	implreport "github.com/agenticlab-ai/pyxis/internal/implreview/report"
	implruntime "github.com/agenticlab-ai/pyxis/internal/implreview/runtime"
	"github.com/agenticlab-ai/pyxis/internal/prdescription"
	"github.com/agenticlab-ai/pyxis/internal/prdescription/prdescriptiontest"
)

const (
	e2eHelperModeEnv = "PYXIS_IMPLEMENT_NOW_E2E_HELPER"
	e2eStubConfigEnv = "PYXIS_IMPLEMENT_NOW_E2E_STUB_CONFIG"
	e2eMaxJSONLLine  = 4 << 20
	e2eReviewComment = 9100
)

type e2eHarnessMode string

const (
	e2eHarnessImplement e2eHarnessMode = "implement"
	e2eHarnessHollow    e2eHarnessMode = "hollow"
)

type e2eStubConfig struct {
	CallEndpoint              string            `json:"call_endpoint,omitempty"`
	StateDir                  string            `json:"state_dir"`
	Origin                    string            `json:"origin"`
	Repository                string            `json:"repository"`
	Label                     string            `json:"label"`
	IssueNumber               int               `json:"issue_number"`
	PRNumber                  int               `json:"pr_number"`
	TaskTitle                 string            `json:"task_title"`
	TaskBody                  string            `json:"task_body"`
	Capsule                   string            `json:"capsule"`
	HarnessMode               e2eHarnessMode    `json:"harness_mode"`
	ReviewAttemptsSourceWrite bool              `json:"review_attempts_source_write,omitempty"`
	GHTokenRoles              map[string]string `json:"gh_token_roles,omitempty"`
}

type e2eStubCall struct {
	Program        string   `json:"program"`
	Dir            string   `json:"dir"`
	Args           []string `json:"args"`
	Stdin          string   `json:"stdin,omitempty"`
	CredentialRole string   `json:"credential_role,omitempty"`
}

type e2ePRState struct {
	Number int    `json:"number"`
	Head   string `json:"head"`
	Base   string `json:"base"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

type e2eCommentState struct {
	ID   int    `json:"id"`
	Body string `json:"body"`
}

var e2eHelperBinDir string

// TestMain doubles the compiled Go test binary as the external processes used
// by the hermetic command-level test. The parent test installs one shared
// binary under the CLI names and prepends its directory to PATH.
// Production subprocess assembly is therefore exercised without a network, a
// real agent, shell fixtures, or test-only injection into the composition root.
func TestMain(m *testing.M) {
	program := filepath.Base(os.Args[0])
	if os.Getenv(e2eHelperModeEnv) != "" || program == "gh" || program == "codex" || program == "opencode" || program == "cursor-agent" {
		os.Exit(runE2EHelper())
	}
	if runtime.GOOS == "windows" {
		os.Exit(m.Run())
	}
	dir, err := prepareE2EHelperBinaries()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	e2eHelperBinDir = dir
	code := m.Run()
	if err := os.RemoveAll(dir); err != nil && code == 0 {
		fmt.Fprintf(os.Stderr, "remove shared e2e helper directory: %v\n", err)
		code = 1
	}
	os.Exit(code)
}

// prepareE2EHelperBinaries copies the race-instrumented package test binary
// once per package run, then hard-links the external-program names to that
// inode. Fixtures share these immutable helpers; their behavior remains
// isolated by a per-fixture config path in the subprocess environment.
func prepareE2EHelperBinaries() (string, error) {
	srcPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate e2e test executable: %w", err)
	}
	dir, err := os.MkdirTemp("", "pyxis-implement-now-e2e-bin-*")
	if err != nil {
		return "", fmt.Errorf("create shared e2e helper directory: %w", err)
	}
	cleanup := func(err error) (string, error) {
		if removeErr := os.RemoveAll(dir); removeErr != nil {
			return "", fmt.Errorf("%w (cleanup: %v)", err, removeErr)
		}
		return "", err
	}
	ghPath := filepath.Join(dir, "gh")
	if err := copyExecutable(srcPath, ghPath); err != nil {
		return cleanup(err)
	}
	for _, program := range []string{"codex", "opencode", "cursor-agent"} {
		helperPath := filepath.Join(dir, program)
		if err := os.Link(ghPath, helperPath); err != nil {
			// A filesystem may forbid hard links. Fall back to a copy.
			if copyErr := copyExecutable(srcPath, helperPath); copyErr != nil {
				return cleanup(fmt.Errorf("link shared %s e2e helper: %v; fallback copy: %w", program, err, copyErr))
			}
		}
	}
	return dir, nil
}

func copyExecutable(srcPath, dstPath string) (result error) {
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open e2e test executable: %w", err)
	}
	defer func() { result = errors.Join(result, src.Close()) }()
	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o755)
	if err != nil {
		return fmt.Errorf("create e2e helper executable: %w", err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return fmt.Errorf("copy e2e helper executable: %w", err)
	}
	if err := dst.Close(); err != nil {
		return fmt.Errorf("close e2e helper executable: %w", err)
	}
	return nil
}

func runE2EHelper() int {
	cfg, err := readE2EStubConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 64
	}
	switch filepath.Base(os.Args[0]) {
	case "gh":
		return runFakeGH(cfg, os.Args[1:])
	case "codex":
		return runFakeCodex(cfg, os.Args[1:])
	case "opencode":
		return runFakeOpenCode(cfg, os.Args[1:])
	case "cursor-agent":
		return runFakeCursor(cfg, os.Args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown e2e helper executable %q\n", os.Args[0])
		return 64
	}
}

func readE2EStubConfig() (e2eStubConfig, error) {
	path := os.Getenv(e2eStubConfigEnv)
	if strings.TrimSpace(path) == "" {
		path = filepath.Join(".pyxis", "e2e-stub-config.json")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return e2eStubConfig{}, fmt.Errorf("read e2e stub config %q: %w", path, err)
	}
	var cfg e2eStubConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return e2eStubConfig{}, fmt.Errorf("decode e2e stub config: %w", err)
	}
	return cfg, nil
}

func runFakeCodex(cfg e2eStubConfig, args []string) int {
	call := e2eStubCall{Program: "codex", Dir: mustGetwd(), Args: append([]string(nil), args...)}
	if len(args) == 1 && args[0] == "--version" {
		if err := appendE2ECall(cfg, call); err != nil {
			return helperError(err)
		}
		fmt.Println("codex-cli e2e-stub")
		return 0
	}
	if !containsArg(args, "exec") || len(args) == 0 || args[len(args)-1] != "-" {
		return unexpectedHelperCall(cfg, call)
	}
	brief, err := io.ReadAll(os.Stdin)
	if err != nil {
		return helperError(fmt.Errorf("read codex brief: %w", err))
	}
	call.Stdin = string(brief)
	if err := appendE2ECall(cfg, call); err != nil {
		return helperError(err)
	}
	if containsArg(args, "--sandbox") && containsArg(args, "danger-full-access") {
		if _, err := os.Stat(filepath.Join(cfg.StateDir, "review-unavailable")); err == nil {
			return helperError(errors.New("fixture reviewer unavailable"))
		} else if !errors.Is(err, os.ErrNotExist) {
			return helperError(err)
		}
		if cfg.ReviewAttemptsSourceWrite {
			if _, err := os.ReadFile("e2e-output.txt"); err != nil {
				return helperError(fmt.Errorf("review cannot read exact implementation source: %w", err))
			}
			if err := os.WriteFile("e2e-output.txt", []byte("reviewer overwrite"), 0o600); err == nil {
				return helperError(errors.New("shared guard allowed a reviewer source overwrite"))
			}
			if err := os.WriteFile(filepath.Join(os.Getenv("TMPDIR"), "review-scratch"), []byte("allowed"), 0o600); err != nil {
				return helperError(fmt.Errorf("review scratch write failed: %w", err))
			}
		}
		model := ""
		for index := 0; index+1 < len(args); index++ {
			if args[index] == "-m" || args[index] == "--model" {
				model = args[index+1]
			}
		}
		if model == "" {
			return helperError(errors.New("review launch lacks its native model"))
		}
		artifact, err := reviewArtifactFromBrief(call.Stdin, model)
		if err != nil {
			return helperError(err)
		}
		if err := emitFixtureNativeLenses(false); err != nil {
			return helperError(err)
		}
		if code := writeHelperJSON(map[string]any{"type": "item.completed", "item": map[string]string{"id": "review", "type": "agent_message", "text": artifact}}); code != 0 {
			return code
		}
		fmt.Println(`{"type":"turn.completed","usage":{"input_tokens":13,"output_tokens":17,"total_tokens":30}}`)
		return 0
	}
	if strings.Contains(call.Stdin, "fixture remote CI failure") {
		if err := os.WriteFile("ci-repaired.txt", []byte("repaired by the CI-only fixer\n"), 0o644); err != nil {
			return helperError(err)
		}
	} else if err := writeE2EImplementation(cfg); err != nil {
		return helperError(err)
	}
	if strings.Contains(call.Stdin, prdescription.BeginMarker) {
		if code := writeHelperJSON(map[string]any{"type": "item.completed", "item": map[string]string{"id": "final", "type": "agent_message", "text": prdescriptiontest.Message}}); code != 0 {
			return code
		}
	}
	fmt.Println(`{"type":"turn.completed","usage":{"input_tokens":11,"output_tokens":7,"total_tokens":18}}`)
	return 0
}

func writeE2EImplementation(cfg e2eStubConfig) error {
	if cfg.HarnessMode != e2eHarnessHollow {
		if err := os.WriteFile("e2e-output.txt", []byte("implemented by the stub harness\n"), 0o644); err != nil {
			return fmt.Errorf("write implementation artifact: %w", err)
		}
		// The helper models file edits only. Real Go workspace code must stage,
		// commit and publish them after this confined process has exited.
	}
	return nil
}

func runFakeOpenCode(cfg e2eStubConfig, args []string) int {
	call := e2eStubCall{Program: "opencode", Dir: mustGetwd(), Args: append([]string(nil), args...)}
	if equalArgs(args, "models") {
		if err := appendE2ECall(cfg, call); err != nil {
			return helperError(err)
		}
		fmt.Println("opencode-go/kimi-k3")
		return 0
	}
	if equalArgs(args, "--version") {
		if err := appendE2ECall(cfg, call); err != nil {
			return helperError(err)
		}
		fmt.Println("opencode e2e-stub")
		return 0
	}
	if len(args) < 6 || args[0] != "run" || !containsArg(args, "--model") || containsArg(args, "--agent") {
		return unexpectedHelperCall(cfg, call)
	}
	call.Stdin = args[len(args)-1]
	if err := appendE2ECall(cfg, call); err != nil {
		return helperError(err)
	}
	if err := writeE2EImplementation(cfg); err != nil {
		return helperError(err)
	}
	if strings.Contains(call.Stdin, prdescription.BeginMarker) {
		if code := writeHelperJSON(map[string]any{"type": "text", "sessionID": "ses_e2e_opencode", "part": map[string]string{"type": "text", "text": prdescriptiontest.Message}}); code != 0 {
			return code
		}
	}
	fmt.Println(`{"type":"step_finish","sessionID":"ses_e2e_opencode","part":{"type":"step-finish","tokens":{"input":11,"output":7}}}`)
	return 0
}

func runFakeCursor(cfg e2eStubConfig, args []string) int {
	call := e2eStubCall{Program: "cursor-agent", Dir: mustGetwd(), Args: append([]string(nil), args...)}
	if equalArgs(args, "--version") {
		if err := appendE2ECall(cfg, call); err != nil {
			return helperError(err)
		}
		fmt.Println("cursor-agent e2e-stub")
		return 0
	}
	if len(args) < 10 || !containsArg(args, "-p") || !containsArg(args, "--force") || !containsArg(args, "--sandbox") || !containsArg(args, "disabled") {
		return unexpectedHelperCall(cfg, call)
	}
	call.Stdin = args[len(args)-1]
	if err := appendE2ECall(cfg, call); err != nil {
		return helperError(err)
	}
	model := ""
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "--model" {
			model = args[i+1]
		}
	}
	artifact, err := reviewArtifactFromBrief(call.Stdin, model)
	if err != nil {
		return helperError(err)
	}
	if err := emitFixtureNativeLenses(true); err != nil {
		return helperError(err)
	}
	return writeHelperJSON(struct {
		Type      string `json:"type"`
		SessionID string `json:"session_id"`
		Result    string `json:"result"`
	}{Type: "result", SessionID: fixtureReviewParent, Result: artifact})
}

func reviewArtifactFromBrief(brief, model string) (string, error) {
	lines := strings.Split(brief, "\n")
	if len(lines) == 0 {
		return "", fmt.Errorf("review brief is empty")
	}
	fields := strings.Fields(lines[0])
	if len(fields) != 3 || fields[0] != "/impl-review" {
		return "", fmt.Errorf("review brief has invalid invocation line %q", lines[0])
	}
	const processMarker = "Review process: "
	_, process, found := strings.Cut(brief, processMarker)
	if !found {
		return "", fmt.Errorf("review brief lacks process identity")
	}
	processFields := strings.Fields(process)
	if len(processFields) == 0 {
		return "", fmt.Errorf("review brief has empty process identity")
	}
	// Model a completed review's inputs, not only an approval message. The
	// production host independently checks this pack before accepting it.
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	evidence := filepath.Join(filepath.Dir(home), "review-evidence")
	root := filepath.Join(evidence, "input-root")
	if err := os.WriteFile(filepath.Join(root, "review", "design-bindings.md"), []byte("Governing design bindings for the hermetic review fixture.\n"), 0o600); err != nil {
		return "", err
	}
	body, err := pack.BuildFromRoot(context.Background(), root, fields[1])
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(evidence, "pack.txt"), body, 0o600); err != nil {
		return "", err
	}
	contextID, err := implruntime.ContextID(fixtureReviewParent)
	if err != nil {
		return "", err
	}
	return persist.ComposeArtifact(&implreport.Report{
		SchemaVersion: implreport.SchemaVersion,
		ProposalRef:   fields[1],
		ReviewID:      processFields[0],
		Band:          implreport.BandStandard,
		Verdict:       implreport.VerdictApprove,
		Rounds: []implreport.Round{{
			ReviewedRevision: fields[2],
			ContextID:        contextID,
			Provenance: implreport.Provenance{
				Model:          model,
				PromptsVersion: "e2e-stub/v1",
				CalVersion:     "impl-review-calibration/v1",
			},
			Findings: []implreport.Finding{},
		}},
	})
}

func runFakeGH(cfg e2eStubConfig, args []string) int {
	call := e2eStubCall{Program: "gh", Dir: mustGetwd(), Args: append([]string(nil), args...)}
	if len(cfg.GHTokenRoles) != 0 {
		call.CredentialRole = cfg.GHTokenRoles[os.Getenv("GH_TOKEN")]
		if call.CredentialRole == "" || os.Getenv("GH_HOST") != "github.com" {
			return helperError(errors.New("GitHub invocation escaped its selected credentials or host"))
		}
	}
	if containsArg(args, "--input") && containsArg(args, "-") {
		stdin, err := io.ReadAll(os.Stdin)
		if err != nil {
			return helperError(fmt.Errorf("read gh stdin: %w", err))
		}
		call.Stdin = string(stdin)
	}
	if err := appendE2ECall(cfg, call); err != nil {
		return helperError(err)
	}
	if equalArgs(args, "api", "--hostname", "github.com", "user", "-q", ".login") {
		login := "pyxis-e2e"
		if call.CredentialRole != "" {
			login += "-" + call.CredentialRole
		}
		fmt.Println(login)
		return 0
	}
	if len(args) >= 3 && args[0] == "run" && args[1] == "view" && containsArg(args, "--log-failed") {
		fmt.Println("fixture remote CI failure")
		return 0
	}
	if len(args) == 0 || args[0] != "api" {
		return unexpectedRecordedCall(call)
	}
	if equalArgs(args, "api", "user", "--jq", ".login", "--header", "Cache-Control: no-cache") {
		fmt.Println("pyxis-e2e")
		return 0
	}
	repositoryPath := "repos/" + cfg.Repository
	if equalArgs(args, "api", repositoryPath, "--jq", ".node_id") {
		fmt.Println("R_e2e_repository")
		return 0
	}
	if equalArgs(args, "api", repositoryPath, "--jq", ".permissions | (.admin or .maintain or .push or .triage)") {
		fmt.Println("true")
		return 0
	}
	method := methodOf(args)
	path := args[len(args)-1]
	issuesPath := fmt.Sprintf("repos/%s/issues?state=open&labels=%s&per_page=100", cfg.Repository, cfg.Label)
	commentsPath := fmt.Sprintf("repos/%s/issues/%d/comments?per_page=100", cfg.Repository, cfg.IssueNumber)
	blockedPath := fmt.Sprintf("repos/%s/issues/%d/dependencies/blocked_by?per_page=100", cfg.Repository, cfg.IssueNumber)
	commentWritePath := fmt.Sprintf("repos/%s/issues/%d/comments", cfg.Repository, cfg.IssueNumber)
	reviewCommentWritePath := fmt.Sprintf("repos/%s/issues/%d/comments", cfg.Repository, cfg.PRNumber)
	reviewCommentReadPath := fmt.Sprintf("repos/%s/issues/comments/%d", cfg.Repository, e2eReviewComment)
	pullsPath := fmt.Sprintf("repos/%s/pulls", cfg.Repository)

	switch {
	case method == "GET" && path == fmt.Sprintf("repos/%s/issues/%d", cfg.Repository, cfg.IssueNumber):
		return writeHelperJSON(map[string]any{"number": cfg.IssueNumber, "title": cfg.TaskTitle, "body": cfg.TaskBody, "state": "open"})
	case method == "GET" && path == issuesPath:
		return writeHelperJSON([]map[string]any{{
			"number": cfg.IssueNumber,
			"title":  cfg.TaskTitle,
			"body":   cfg.TaskBody,
			"state":  "open",
			"labels": []map[string]string{{"name": cfg.Label}},
		}})
	case method == "GET" && path == commentsPath:
		if cfg.Capsule == "" {
			return writeHelperJSON([]any{})
		}
		return writeHelperJSON([]map[string]any{{"id": 1, "body": cfg.Capsule}})
	case method == "GET" && path == blockedPath:
		return writeHelperJSON([]any{})
	case method == "POST" && path == commentWritePath:
		return writeHelperJSON(map[string]any{"id": 9001})
	case method == "POST" && containsArg(args, reviewCommentWritePath):
		var payload struct {
			Body string `json:"body"`
		}
		if err := json.Unmarshal([]byte(call.Stdin), &payload); err != nil {
			return helperError(fmt.Errorf("decode review comment input: %w", err))
		}
		if strings.TrimSpace(payload.Body) == "" {
			return helperError(fmt.Errorf("review comment body is empty"))
		}
		if err := writeE2ECommentState(cfg, e2eCommentState{ID: e2eReviewComment, Body: payload.Body}); err != nil {
			return helperError(err)
		}
		return writeHelperJSON(map[string]int{"id": e2eReviewComment})
	case method == "GET" && containsArg(args, reviewCommentReadPath):
		comment, ok, err := readE2ECommentState(cfg)
		if err != nil {
			return helperError(err)
		}
		if !ok {
			return helperError(fmt.Errorf("review comment %d read before publication", e2eReviewComment))
		}
		return writeHelperJSON(comment)
	case method == "GET" && strings.HasPrefix(path, pullsPath+"?head="):
		state, ok, err := readE2EPRState(cfg)
		if err != nil {
			return helperError(err)
		}
		requestedHead, err := pullHeadQuery(path)
		if err != nil {
			return helperError(err)
		}
		if ok && state.Head == requestedHead {
			return writeHelperJSON([]map[string]int{{"number": state.Number}})
		}
		return writeHelperJSON([]any{})
	case method == "GET" && path == pullsPath+"?state=open&per_page=100":
		state, ok, err := readE2EPRState(cfg)
		if err != nil {
			return helperError(err)
		}
		if !ok {
			return writeHelperJSON([]any{})
		}
		return writeHelperJSON([]map[string]any{{"number": state.Number, "head": map[string]string{"ref": state.Head}}})
	case method == "GET" && strings.HasPrefix(path, fmt.Sprintf("repos/%s/compare/", cfg.Repository)):
		base, head, err := compareRefs(path)
		if err != nil {
			return helperError(err)
		}
		ahead, err := gitOutput("--git-dir", cfg.Origin, "rev-list", "--count", base+".."+head)
		if err != nil {
			return helperError(fmt.Errorf("compare local origin %s...%s: %w", base, head, err))
		}
		n, err := strconv.Atoi(strings.TrimSpace(ahead))
		if err != nil {
			return helperError(fmt.Errorf("parse ahead count %q: %w", ahead, err))
		}
		return writeHelperJSON(map[string]int{"ahead_by": n})
	case method == "POST" && path == pullsPath:
		state := e2ePRState{
			Number: cfg.PRNumber,
			Head:   formValue(args, "head"),
			Base:   formValue(args, "base"),
			Title:  formValue(args, "title"),
			Body:   formValue(args, "body"),
		}
		if state.Head == "" || state.Base == "" {
			return helperError(fmt.Errorf("create PR omitted head or base: %v", args))
		}
		if err := writeE2EPRState(cfg, state); err != nil {
			return helperError(err)
		}
		if _, err := os.Stat(filepath.Join(cfg.StateDir, "proposal-receipt-unavailable")); err == nil {
			return helperError(errors.New("fixture interrupted after remote PR creation"))
		} else if !errors.Is(err, os.ErrNotExist) {
			return helperError(err)
		}
		return writeHelperJSON(map[string]int{"number": state.Number})
	case method == "PATCH" && path == fmt.Sprintf("repos/%s/pulls/%d", cfg.Repository, cfg.PRNumber):
		if _, err := os.Stat(filepath.Join(cfg.StateDir, "summary-unavailable")); err == nil {
			return helperError(errors.New("fixture metadata publisher unavailable"))
		} else if !errors.Is(err, os.ErrNotExist) {
			return helperError(err)
		}
		state, ok, err := readE2EPRState(cfg)
		if err != nil || !ok {
			return helperError(fmt.Errorf("patch missing PR: %w", err))
		}
		var input struct {
			Body string `json:"body"`
		}
		if err := json.Unmarshal([]byte(call.Stdin), &input); err != nil {
			return helperError(err)
		}
		state.Body = input.Body
		if err := writeE2EPRState(cfg, state); err != nil {
			return helperError(err)
		}
		return writeHelperJSON(map[string]int{"number": state.Number})
	case method == "GET" && path == fmt.Sprintf("repos/%s/pulls/%d", cfg.Repository, cfg.PRNumber):
		state, ok, err := readE2EPRState(cfg)
		if err != nil {
			return helperError(err)
		}
		if !ok {
			return helperError(fmt.Errorf("pull #%d read before creation", cfg.PRNumber))
		}
		sha, err := gitOutput("--git-dir", cfg.Origin, "rev-parse", state.Head)
		if err != nil {
			return helperError(fmt.Errorf("read pushed head %s: %w", state.Head, err))
		}
		return writeHelperJSON(map[string]any{
			"number": state.Number, "body": state.Body,
			"state": "open", "merged": false,
			"head": map[string]string{"sha": strings.TrimSpace(sha), "ref": state.Head},
		})
	case method == "GET" && strings.HasPrefix(path, fmt.Sprintf("repos/%s/commits/", cfg.Repository)) && strings.HasSuffix(path, "/statuses?per_page=100"):
		return writeHelperJSON([]any{})
	case method == "GET" && strings.HasPrefix(path, fmt.Sprintf("repos/%s/commits/", cfg.Repository)) && strings.HasSuffix(path, "/check-runs?filter=latest&per_page=100"):
		if _, err := os.Stat(filepath.Join(cfg.StateDir, "checks-unavailable")); err == nil {
			return helperError(errors.New("fixture interrupted before review capture"))
		} else if !errors.Is(err, os.ErrNotExist) {
			return helperError(err)
		}
		if marker := filepath.Join(cfg.StateDir, "checks-pending-once"); func() bool {
			if _, err := os.Stat(marker); err == nil {
				_ = os.Remove(marker)
				return true
			}
			return false
		}() {
			return writeHelperJSON(map[string]any{"check_runs": []map[string]string{{"name": "e2e", "status": "in_progress"}}})
		}
		conclusion := "success"
		if _, err := os.Stat(filepath.Join(cfg.StateDir, "checks-red")); err == nil {
			commit := strings.TrimSuffix(strings.TrimPrefix(path, fmt.Sprintf("repos/%s/commits/", cfg.Repository)), "/check-runs?filter=latest&per_page=100")
			if _, err := gitOutput("--git-dir", cfg.Origin, "cat-file", "-e", commit+":ci-repaired.txt"); err != nil {
				conclusion = "failure"
			}
		}
		return writeHelperJSON(map[string]any{"check_runs": []map[string]string{{"name": "e2e", "status": "completed", "conclusion": conclusion}}})
	case method == "GET" && strings.HasPrefix(path, fmt.Sprintf("repos/%s/actions/runs?head_sha=", cfg.Repository)):
		head := strings.TrimSuffix(strings.TrimPrefix(path, fmt.Sprintf("repos/%s/actions/runs?head_sha=", cfg.Repository)), "&per_page=100")
		return writeHelperJSON(map[string]any{"workflow_runs": []map[string]any{{
			"id": 91, "workflow_id": 7, "event": "pull_request", "head_sha": head, "status": "completed", "conclusion": "failure",
		}}})
	default:
		return unexpectedRecordedCall(call)
	}
}

func appendE2ECall(cfg e2eStubConfig, call e2eStubCall) error {
	if cfg.CallEndpoint != "" {
		conn, err := net.DialTimeout("tcp", cfg.CallEndpoint, 5*time.Second)
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }() // request/ack errors are returned below
		if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
			return err
		}
		if err := json.NewEncoder(conn).Encode(call); err != nil {
			return err
		}
		var ack string
		if err := json.NewDecoder(conn).Decode(&ack); err != nil {
			return err
		}
		if ack != "ok" {
			return fmt.Errorf("record helper call: %s", ack)
		}
		return nil
	}
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		return fmt.Errorf("create e2e state dir: %w", err)
	}
	b, err := json.Marshal(call)
	if err != nil {
		return fmt.Errorf("encode e2e call: %w", err)
	}
	b = append(b, '\n')
	f, err := os.OpenFile(filepath.Join(cfg.StateDir, "calls.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open e2e call log: %w", err)
	}
	if _, err := f.Write(b); err != nil {
		return errors.Join(fmt.Errorf("append e2e call: %w", err), f.Close())
	}
	return f.Close()
}

// The test parent owns the trace file. A confined helper can report one typed
// call but receives no filesystem exemption for the fixture's host state.
func startE2ECallRecorder(t *testing.T, cfg e2eStubConfig, reject ...func(e2eStubCall) error) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			func() {
				defer func() { _ = conn.Close() }() // bounded trace connection is complete
				if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
					return
				}
				var call e2eStubCall
				err := json.NewDecoder(io.LimitReader(conn, e2eMaxJSONLLine)).Decode(&call)
				if err == nil {
					err = appendE2ECall(cfg, call)
				}
				for _, rejectCall := range reject {
					if err == nil {
						err = rejectCall(call)
					}
				}
				ack := "ok"
				if err != nil {
					ack = err.Error()
				}
				// A disconnected helper already fails its own launch; the trace
				// server has no recovery action for an unread acknowledgement.
				_ = json.NewEncoder(conn).Encode(ack)
			}()
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close() // stops the owned Accept loop
		<-done
	})
	return listener.Addr().String()
}

func readE2ECalls(t *testing.T, stateDir string) []e2eStubCall {
	t.Helper()
	f, err := os.Open(filepath.Join(stateDir, "calls.jsonl"))
	if err != nil {
		t.Fatalf("open e2e call log: %v", err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Errorf("close e2e call log: %v", err)
		}
	}()
	var calls []e2eStubCall
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 64*1024), e2eMaxJSONLLine)
	for s.Scan() {
		var call e2eStubCall
		if err := json.Unmarshal(s.Bytes(), &call); err != nil {
			t.Fatalf("decode e2e call %q: %v", s.Text(), err)
		}
		calls = append(calls, call)
	}
	if err := s.Err(); err != nil {
		t.Fatalf("scan e2e calls: %v", err)
	}
	return calls
}

func writeE2ECommentState(cfg e2eStubConfig, state e2eCommentState) error {
	b, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode e2e review comment: %w", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.StateDir, "review-comment.json"), b, 0o600); err != nil {
		return fmt.Errorf("write e2e review comment: %w", err)
	}
	return nil
}

func readE2ECommentState(cfg e2eStubConfig) (e2eCommentState, bool, error) {
	b, err := os.ReadFile(filepath.Join(cfg.StateDir, "review-comment.json"))
	if os.IsNotExist(err) {
		return e2eCommentState{}, false, nil
	}
	if err != nil {
		return e2eCommentState{}, false, fmt.Errorf("read e2e review comment: %w", err)
	}
	var state e2eCommentState
	if err := json.Unmarshal(b, &state); err != nil {
		return e2eCommentState{}, false, fmt.Errorf("decode e2e review comment: %w", err)
	}
	return state, true, nil
}

func writeE2EPRState(cfg e2eStubConfig, state e2ePRState) error {
	b, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cfg.StateDir, "pr.json"), b, 0o600)
}

func readE2EPRState(cfg e2eStubConfig) (e2ePRState, bool, error) {
	b, err := os.ReadFile(filepath.Join(cfg.StateDir, "pr.json"))
	if os.IsNotExist(err) {
		return e2ePRState{}, false, nil
	}
	if err != nil {
		return e2ePRState{}, false, err
	}
	var state e2ePRState
	if err := json.Unmarshal(b, &state); err != nil {
		return e2ePRState{}, false, err
	}
	return state, true, nil
}

func writeHelperJSON(v any) int {
	if err := json.NewEncoder(os.Stdout).Encode(v); err != nil {
		return helperError(err)
	}
	return 0
}

func methodOf(args []string) string {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "--method" {
			return args[i+1]
		}
	}
	return "GET"
}

func pullHeadQuery(path string) (string, error) {
	_, rawQuery, found := strings.Cut(path, "?")
	if !found {
		return "", fmt.Errorf("pull lookup lacks query: %q", path)
	}
	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		return "", err
	}
	head := query.Get("head")
	if j := strings.IndexByte(head, ':'); j >= 0 {
		head = head[j+1:]
	}
	return head, nil
}

func compareRefs(path string) (string, string, error) {
	_, pair, found := strings.Cut(path, "/compare/")
	if !found {
		return "", "", fmt.Errorf("invalid compare path %q", path)
	}
	base, head, ok := strings.Cut(pair, "...")
	if !ok || base == "" || head == "" {
		return "", "", fmt.Errorf("invalid compare refs %q", pair)
	}
	return base, head, nil
}

func formValue(args []string, key string) string {
	prefix := key + "="
	for _, arg := range args {
		if value, found := strings.CutPrefix(arg, prefix); found {
			return value
		}
	}
	return ""
}

func gitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(stderr.String()), err)
	}
	return string(out), nil
}

func helperError(err error) int {
	fmt.Fprintln(os.Stderr, err)
	return 70
}

func unexpectedHelperCall(cfg e2eStubConfig, call e2eStubCall) int {
	if err := appendE2ECall(cfg, call); err != nil {
		return helperError(fmt.Errorf("record unexpected helper call: %w", err))
	}
	return unexpectedRecordedCall(call)
}

func unexpectedRecordedCall(call e2eStubCall) int {
	fmt.Fprintf(os.Stderr, "unexpected %s invocation: %q\n", call.Program, call.Args)
	return 64
}

func mustGetwd() string {
	dir, err := os.Getwd()
	if err != nil {
		return "<getwd failed: " + err.Error() + ">"
	}
	return dir
}

func containsArg(args []string, want string) bool {
	return slices.Contains(args, want)
}

func equalArgs(got []string, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
