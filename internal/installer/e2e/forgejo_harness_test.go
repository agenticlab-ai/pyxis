package e2e

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	implreport "github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

const (
	forgejoHarnessTaskMarker = "forgejo-live-feature-e2e"
	forgejoHarnessConfigPath = ".pyxis/forgejo-e2e-harness.json"
	forgejoHarnessSchema     = "forgejo-model-harness/v1"
	forgejoHarnessMaxBrief   = 2 << 20
	forgejoHarnessSource     = "package feature\n\n// Add returns the sum of its operands.\nfunc Add(a, b int) int { return a + b }\n"
)

type forgejoHarnessConfig struct {
	SchemaVersion string `json:"schema_version"`
	CallEndpoint  string `json:"call_endpoint"`
}

// The parent records the model boundary only. It never answers Git, Forgejo,
// verification, or release requests on behalf of the installed executable.
type forgejoHarnessCall struct {
	SchemaVersion string   `json:"schema_version"`
	Mode          string   `json:"mode"`
	Dir           string   `json:"dir"`
	Args          []string `json:"args"`
	PromptSHA     string   `json:"prompt_sha256,omitempty"`
	Prompt        string   `json:"prompt,omitempty"`
	Accepted      bool     `json:"accepted"`
	// Version probes are host-side readiness checks. Worker/reviewer launches
	// must attest this after the production runtime strips host credentials.
	ForgejoTokenAbsent bool `json:"forgejo_token_absent"`
}

type forgejoHarnessReply struct {
	Accepted bool   `json:"accepted"`
	Error    string `json:"error,omitempty"`
}

// newForgejoHarness returns an environment with only codex replaced, a safe
// snapshot of its call trace, and configuration the fixture must track at
// forgejoHarnessConfigPath. The config survives the genuine worker/reviewer
// checkouts; custom test environment variables intentionally do not survive
// the production runtime's closed launch environment.
func newForgejoHarness(t *testing.T, env []string, repo string, reviewCalls int) ([]string, func() []forgejoHarnessCall, []byte) {
	t.Helper()
	if reviewCalls < 0 {
		t.Fatal("negative review script length")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	copyFile(t, executable, filepath.Join(binDir, "codex"), 0o755)
	var mu sync.Mutex
	var calls []forgejoHarnessCall
	probes, executions := 0, 0
	logPath := filepath.Join(repo, ".pyxis/logs/tests/forgejo-model-calls.jsonl")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/calls" {
			http.Error(w, "unknown model recorder operation", http.StatusNotFound)
			return
		}
		var call forgejoHarnessCall
		if err := decodeForgejoHarness(io.LimitReader(r.Body, forgejoHarnessMaxBrief*2), &call); err != nil {
			http.Error(w, "invalid model recorder request", http.StatusBadRequest)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		mode, _, err := forgejoHarnessArguments(call.Args)
		reply := forgejoHarnessReply{}
		switch {
		case err != nil || mode != call.Mode || call.SchemaVersion != forgejoHarnessSchema || !filepath.IsAbs(call.Dir):
			reply.Error = "unexpected model invocation grammar"
		case mode == "version":
			probes++
			reply.Accepted = probes <= 32 && call.Prompt == "" && call.PromptSHA == ""
			if !reply.Accepted {
				reply.Error = "version probe exceeded its bounded script"
			}
		case !call.ForgejoTokenAbsent:
			reply.Error = "Forgejo credential crossed the model process boundary"
		case call.PromptSHA != forgejoBriefSHA(call.Prompt):
			reply.Error = "model brief checksum mismatch"
		case mode == "implement" && executions == 0 && strings.Contains(call.Prompt, forgejoHarnessTaskMarker):
			reply.Accepted = true
			executions++
		case mode == "review" && executions > 0 && executions <= reviewCalls:
			if _, err := forgejoReviewArtifact(call.Prompt, "recording-only"); err != nil {
				reply.Error = err.Error()
			} else {
				reply.Accepted = true
				executions++
			}
		default:
			reply.Error = "model invocation is outside the implementation/review script"
		}
		call.Accepted = reply.Accepted
		calls = append(calls, call)
		if err := appendForgejoHarnessCall(logPath, call); err != nil {
			reply = forgejoHarnessReply{Error: "cannot persist model invocation trace"}
		}
		if !reply.Accepted {
			w.WriteHeader(http.StatusConflict)
		}
		if err := json.NewEncoder(w).Encode(reply); err != nil {
			t.Errorf("write model invocation acknowledgement: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	nextEnv := make([]string, 0, len(env)+2)
	path := ""
	for _, entry := range env {
		key, value, _ := strings.Cut(entry, "=")
		switch key {
		case "PATH":
			path = value
		case "OPENAI_API_KEY", "CODEX_API_KEY":
			// No real model credential is needed or inherited by this helper.
		default:
			nextEnv = append(nextEnv, entry)
		}
	}
	nextEnv = append(nextEnv, "PATH="+binDir+string(os.PathListSeparator)+path, "CODEX_API_KEY=fixture-only-no-model-service")
	config, err := json.Marshal(forgejoHarnessConfig{SchemaVersion: forgejoHarnessSchema, CallEndpoint: server.URL + "/calls"})
	if err != nil {
		t.Fatal(err)
	}
	snapshot := func() []forgejoHarnessCall {
		mu.Lock()
		defer mu.Unlock()
		out := append([]forgejoHarnessCall(nil), calls...)
		for i := range out {
			out[i].Args = append([]string(nil), out[i].Args...)
		}
		return out
	}
	return nextEnv, snapshot, config
}

func appendForgejoHarnessCall(path string, call forgejoHarnessCall) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	err = json.NewEncoder(f).Encode(call)
	return errors.Join(err, f.Close())
}

// TestMain calls this before flag.Parse. A copied Go test binary under codex
// executes the deterministic model protocol instead of recursively running
// tests or reaching a real model. Every other program remains genuine.
func runForgejoModelHelper() (int, bool) {
	if filepath.Base(os.Args[0]) != "codex" {
		return 0, false
	}
	if err := executeForgejoModelHelper(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "forgejo model helper:", err) // Best-effort diagnostic; exit code carries the failure.
		return 64, true
	}
	return 0, true
}

func executeForgejoModelHelper(args []string, stdin io.Reader, stdout io.Writer) error {
	mode, model, err := forgejoHarnessArguments(args)
	if err != nil {
		return err
	}
	_, forgejoTokenPresent := os.LookupEnv("PYXIS_FORGEJO_TOKEN")
	if mode != "version" && forgejoTokenPresent {
		return errors.New("PYXIS_FORGEJO_TOKEN must be absent from implementation and review environments")
	}
	b, err := os.ReadFile(forgejoHarnessConfigPath)
	if err != nil {
		return fmt.Errorf("read tracked model fixture: %w", err)
	}
	var config forgejoHarnessConfig
	if err := decodeForgejoHarness(bytes.NewReader(b), &config); err != nil {
		return err
	}
	u, err := url.Parse(config.CallEndpoint)
	if err != nil || config.SchemaVersion != forgejoHarnessSchema || u == nil || u.Scheme != "http" || u.User != nil || u.Path != "/calls" || u.RawQuery != "" || u.Fragment != "" || !net.ParseIP(u.Hostname()).IsLoopback() {
		return errors.New("model recorder must be the configured loopback HTTP endpoint")
	}
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	call := forgejoHarnessCall{SchemaVersion: forgejoHarnessSchema, Mode: mode, Dir: dir, Args: append([]string(nil), args...), ForgejoTokenAbsent: !forgejoTokenPresent}
	if mode != "version" {
		brief, err := io.ReadAll(io.LimitReader(stdin, forgejoHarnessMaxBrief+1))
		if err != nil || len(brief) > forgejoHarnessMaxBrief {
			return errors.New("model brief unreadable or too large")
		}
		call.Prompt, call.PromptSHA = string(brief), forgejoBriefSHA(string(brief))
	}
	b, err = json.Marshal(call)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, config.CallEndpoint, bytes.NewReader(b))
	if err != nil {
		return err
	}
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	response, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("record model invocation: %w", err)
	}
	var reply forgejoHarnessReply
	err = decodeForgejoHarness(io.LimitReader(response.Body, 8192), &reply)
	closeErr := response.Body.Close()
	if err != nil || closeErr != nil {
		return errors.Join(err, closeErr)
	}
	if response.StatusCode != http.StatusOK || !reply.Accepted {
		return fmt.Errorf("model script refused invocation: %s", reply.Error)
	}
	if mode == "version" {
		_, err := fmt.Fprintln(stdout, "codex-cli forgejo-e2e-stub/v1")
		return err
	}
	if mode == "implement" {
		if info, err := os.Lstat("sum.go"); err == nil && !info.Mode().IsRegular() {
			return errors.New("feature output must be an ordinary file")
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := os.WriteFile("sum.go", []byte(forgejoHarnessSource), 0o644); err != nil {
			return fmt.Errorf("write genuine Go feature: %w", err)
		}
	} else {
		artifact, err := forgejoReviewArtifact(call.Prompt, model)
		if err != nil {
			return err
		}
		event := struct {
			Type string `json:"type"`
			Item struct {
				ID   string `json:"id"`
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"item"`
		}{Type: "item.completed"}
		event.Item.ID, event.Item.Type, event.Item.Text = "review", "agent_message", artifact
		if err := json.NewEncoder(stdout).Encode(event); err != nil {
			return err
		}
	}
	_, err = io.WriteString(stdout, "{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":11,\"output_tokens\":7,\"total_tokens\":18}}\n")
	return err
}

var forgejoHarnessModelName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:/-]*$`)

func forgejoHarnessArguments(args []string) (mode, model string, err error) {
	if len(args) == 1 && args[0] == "--version" {
		return "version", "", nil
	}
	var expected []string
	var effort string
	switch {
	case len(args) == 9:
		mode, model, effort = "implement", args[3], args[5]
		expected = []string{"exec", "--json", "-m", model, "-c", effort, "--dangerously-bypass-approvals-and-sandbox", "--skip-git-repo-check", "-"}
	case len(args) == 14:
		mode, model, effort = "review", args[5], args[7]
		expected = []string{"-a", "never", "exec", "--json", "-m", model, "-c", effort, "--sandbox", "read-only", "--ephemeral", "--ignore-user-config", "--skip-git-repo-check", "-"}
	default:
		return "", "", errors.New("unsupported Codex invocation; only version, fresh implementation, or fresh read-only review is scripted")
	}
	if !forgejoHarnessModelName.MatchString(model) || strings.Join(args, "\x00") != strings.Join(expected, "\x00") {
		return "", "", errors.New("unexpected Codex arguments")
	}
	raw, ok := strings.CutPrefix(effort, "model_reasoning_effort=")
	value, unquoteErr := strconv.Unquote(raw)
	if !ok || unquoteErr != nil || !strings.Contains("|minimal|low|medium|high|xhigh|max|ultra|", "|"+value+"|") {
		return "", "", errors.New("unexpected Codex reasoning effort")
	}
	return mode, model, nil
}

func forgejoBriefSHA(brief string) string {
	sum := sha256.Sum256([]byte(brief))
	return hex.EncodeToString(sum[:])
}

func decodeForgejoHarness(r io.Reader, out any) error {
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return errors.New("model fixture contains trailing JSON")
	}
	return nil
}

func forgejoReviewArtifact(brief, model string) (string, error) {
	first, _, _ := strings.Cut(brief, "\n")
	fields := strings.Fields(first)
	if len(fields) != 3 || fields[0] != "/impl-review" || !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(fields[2]) {
		return "", errors.New("review brief lacks exact proposal/revision identity")
	}
	const marker = "Review process: "
	_, process, ok := strings.Cut(brief, marker)
	identity := strings.Fields(process)
	if !ok || len(identity) == 0 {
		return "", errors.New("review brief lacks process identity")
	}
	return persist.ComposeArtifact(&implreport.Report{
		SchemaVersion: implreport.SchemaVersion, ProposalRef: fields[1], ReviewID: identity[0],
		Band: implreport.BandStandard, Verdict: implreport.VerdictApprove,
		Rounds: []implreport.Round{{ReviewedRevision: fields[2], ContextID: "forgejo-e2e-" + identity[0], Provenance: implreport.Provenance{
			Model: model, PromptsVersion: "forgejo-e2e-stub/v1", CalVersion: "impl-review-calibration/v1",
		}, Findings: []implreport.Finding{}}},
	})
}

func TestForgejoHarnessRejectsUnscriptedArguments(t *testing.T) {
	valid := []string{"exec", "--json", "-m", "fixture-model", "-c", `model_reasoning_effort="high"`, "--dangerously-bypass-approvals-and-sandbox", "--skip-git-repo-check", "-"}
	for _, args := range [][]string{
		{"login"}, {"exec", "resume", "old-thread", "-"},
		append(append([]string(nil), valid...), "--unknown"),
		{"exec", "--json", "-m", "fixture-model", "-c", `untrusted_setting="high"`, "--dangerously-bypass-approvals-and-sandbox", "--skip-git-repo-check", "-"},
	} {
		if _, _, err := forgejoHarnessArguments(args); err == nil {
			t.Errorf("accepted unscripted arguments %q", args)
		}
	}
	if mode, model, err := forgejoHarnessArguments(valid); err != nil || mode != "implement" || model != "fixture-model" {
		t.Fatalf("production implementation grammar: %s/%s %v", mode, model, err)
	}
}

func TestForgejoHarnessProcessScript(t *testing.T) {
	repo := t.TempDir()
	env, calls, config := newForgejoHarness(t, os.Environ(), repo, 1)
	// This direct helper protocol test does not launch through the production
	// runtime; model that runtime's environment stripping locally. The live
	// installed scenario keeps the host token and must prove real stripping.
	var modelEnv []string
	for _, entry := range env {
		if !strings.HasPrefix(entry, "PYXIS_FORGEJO_TOKEN=") {
			modelEnv = append(modelEnv, entry)
		}
	}
	env = modelEnv
	writeFile(t, filepath.Join(repo, forgejoHarnessConfigPath), config, 0o600)
	var binary string
	for _, entry := range env {
		if path, ok := strings.CutPrefix(entry, "PATH="); ok {
			binary = filepath.Join(filepath.SplitList(path)[0], "codex")
		}
	}
	run := func(brief string, args ...string) ([]byte, error) {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Dir, cmd.Env, cmd.Stdin = repo, env, strings.NewReader(brief)
		return cmd.CombinedOutput()
	}
	if output, err := run("", "--version"); err != nil || !bytes.Contains(output, []byte("forgejo-e2e-stub")) {
		t.Fatalf("version probe: %s %v", output, err)
	}
	implement := []string{"exec", "--json", "-m", "fixture-model", "-c", `model_reasoning_effort="high"`, "--dangerously-bypass-approvals-and-sandbox", "--skip-git-repo-check", "-"}
	if output, err := run(forgejoHarnessTaskMarker, append(append([]string(nil), implement...), "--unexpected")...); err == nil || !bytes.Contains(output, []byte("unsupported Codex invocation")) {
		t.Fatalf("unknown arguments were not refused by the process: %s %v", output, err)
	}
	if _, err := os.Stat(filepath.Join(repo, "sum.go")); !os.IsNotExist(err) || len(calls()) != 1 {
		t.Fatal("unknown arguments reached the recorder or wrote the feature")
	}
	if output, err := run(forgejoHarnessTaskMarker, implement...); err != nil {
		t.Fatalf("implementation: %s %v", output, err)
	}
	if got, err := os.ReadFile(filepath.Join(repo, "sum.go")); err != nil || string(got) != forgejoHarnessSource {
		t.Fatalf("actual feature write: %q %v", got, err)
	}
	brief := "/impl-review forgejo:http://127.0.0.1:3000/owner/repo#1 " + strings.Repeat("a", 40) + "\n\nReview process: test-review-1\n"
	review := []string{"-a", "never", "exec", "--json", "-m", "fixture-model", "-c", `model_reasoning_effort="high"`, "--sandbox", "read-only", "--ephemeral", "--ignore-user-config", "--skip-git-repo-check", "-"}
	if output, err := run(brief, review...); err != nil || !bytes.Contains(output, []byte("impl-review-report/v1")) {
		t.Fatalf("review: %s %v", output, err)
	}
	if output, err := run(forgejoHarnessTaskMarker, implement...); err == nil || !bytes.Contains(output, []byte("outside the implementation/review script")) {
		t.Fatalf("unexpected repeated implementation was accepted: %s %v", output, err)
	}
	trace := calls()
	if len(trace) != 4 || !trace[0].Accepted || !trace[1].Accepted || !trace[2].Accepted || trace[3].Accepted {
		t.Fatalf("unexpected call trace: %+v", trace)
	}
	if trace[1].PromptSHA != forgejoBriefSHA(forgejoHarnessTaskMarker) {
		t.Fatal("call trace does not attest the actual brief")
	}
	if !trace[1].ForgejoTokenAbsent || !trace[2].ForgejoTokenAbsent {
		t.Fatal("worker or reviewer failed to attest the missing Forgejo credential")
	}
}

func TestForgejoHarnessRefusesLeakedHostToken(t *testing.T) {
	repo := t.TempDir()
	env, calls, config := newForgejoHarness(t, os.Environ(), repo, 1)
	writeFile(t, filepath.Join(repo, forgejoHarnessConfigPath), config, 0o600)
	const sentinel = "fixture-token-must-never-be-printed"
	var binary string
	for _, entry := range env {
		if path, ok := strings.CutPrefix(entry, "PATH="); ok {
			binary = filepath.Join(filepath.SplitList(path)[0], "codex")
		}
	}
	env = append(env, "PYXIS_FORGEJO_TOKEN="+sentinel)
	for _, args := range [][]string{
		{"exec", "--json", "-m", "fixture-model", "-c", `model_reasoning_effort="high"`, "--dangerously-bypass-approvals-and-sandbox", "--skip-git-repo-check", "-"},
		{"-a", "never", "exec", "--json", "-m", "fixture-model", "-c", `model_reasoning_effort="high"`, "--sandbox", "read-only", "--ephemeral", "--ignore-user-config", "--skip-git-repo-check", "-"},
	} {
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Dir, cmd.Env, cmd.Stdin = repo, env, strings.NewReader(forgejoHarnessTaskMarker)
		output, err := cmd.CombinedOutput()
		cancel()
		if err == nil || !bytes.Contains(output, []byte("must be absent")) || bytes.Contains(output, []byte(sentinel)) {
			t.Fatalf("model process did not reject host credential without disclosing it: %s %v", output, err)
		}
	}
	if len(calls()) != 0 {
		t.Fatal("credential-bearing model process reached the recorder")
	}
	if _, err := os.Stat(filepath.Join(repo, "sum.go")); !os.IsNotExist(err) {
		t.Fatal("credential-bearing model process wrote feature code")
	}
}
