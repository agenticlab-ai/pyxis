package startup_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The real Claude process resumes a saved conversation without submitting a
// prompt. Its SessionStart hook reports the engine's cwd after resume, so the
// assertion catches a restored directory even if the shell entered a worktree.
func TestRealClaudeResumeUsesWorktree(t *testing.T) {
	if os.Getenv("PYXIS_TEST_REAL_HARNESSES") != "1" {
		t.Skip("set PYXIS_TEST_REAL_HARNESSES=1 to check the installed Claude CLI")
	}
	f := setup(t, "claude")
	if err := os.Remove(filepath.Join(f.base, "bin", "claude")); err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(f.base, "operator-home")
	config := filepath.Join(home, ".claude")
	project := regexp.MustCompile(`[^a-zA-Z0-9]`).ReplaceAllString(f.repo, "-")
	sessions := filepath.Join(config, "projects", project)
	if err := os.MkdirAll(sessions, 0700); err != nil {
		t.Fatal(err)
	}
	id := resumeArgs("claude")[1]
	// Fixture text contains no task to execute and no pending tool calls.
	cwd, err := json.Marshal(f.repo)
	if err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(sessions, id+".jsonl"), fmt.Sprintf(`{"type":"user","parentUuid":null,"isSidechain":false,"userType":"external","cwd":%s,"sessionId":%q,"version":"2.1.268","uuid":"01990eab-2222-7222-8222-222222222222","timestamp":"2026-09-01T00:00:00Z","message":{"role":"user","content":"Offline fixture"}}
{"type":"assistant","parentUuid":"01990eab-2222-7222-8222-222222222222","isSidechain":false,"userType":"external","cwd":%s,"sessionId":%q,"version":"2.1.268","uuid":"01990eab-3333-7333-8333-333333333333","timestamp":"2026-09-01T00:00:01Z","message":{"id":"msg_fixture","type":"message","role":"assistant","model":"claude-sonnet-4-6","content":[{"type":"text","text":"Offline fixture complete."}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":1,"output_tokens":1}}}
`, cwd, id, cwd, id))
	preferences := fmt.Sprintf(`{"hasCompletedOnboarding":true,"theme":"dark","numStartups":1,"customApiKeyResponses":{"approved":["offline-fixture-key"],"rejected":[]},"projects":{%s:{"hasTrustDialogAccepted":true,"hasCompletedProjectOnboarding":true}}}`, cwd)
	put(t, filepath.Join(home, ".claude.json"), preferences)
	put(t, filepath.Join(config, ".claude.json"), preferences)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	hook, err := json.Marshal("'" + strings.ReplaceAll(exe, "'", "'\"'\"'") + "' pyxis-session-hook")
	if err != nil {
		t.Fatal(err)
	}
	settings := fmt.Sprintf(`{"hooks":{"SessionStart":[{"matcher":"resume","hooks":[{"type":"command","command":%s}]}]},"autoUpdatesChannel":"stable"}`, hook)
	f.env = append(f.env, "HOME="+home, "CLAUDE_CONFIG_DIR="+config, "ANTHROPIC_API_KEY=offline-fixture-key",
		"ANTHROPIC_BASE_URL=http://127.0.0.1:1", "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1", "DISABLE_AUTOUPDATER=1", "TERM=xterm-256color")
	before := run(t, f.repo, f.env, "git", "status", "--porcelain=v1")
	attachTerminal(t, f, []terminalStep{{"PYXIS_RESUME_READY", "/exit"}, {"/exit", "\r"}})
	args := append(resumeArgs("claude"), "--ax-screen-reader", "--settings", settings)
	inv, out, err := f.launch(f.repo, true, true, args...)
	if err != nil || !strings.Contains(inv.Cwd, ".pyxis-worktrees") {
		t.Fatalf("real Claude resume: %+v %v\n%s", inv, err, terminalANSI.ReplaceAllString(out, ""))
	}
	data, err := os.ReadFile(f.record)
	if err != nil {
		t.Fatal(err)
	}
	var event struct {
		Session string `json:"session_id"`
		Source  string `json:"source"`
	}
	if err := json.Unmarshal(data, &event); err != nil || event.Session != id || event.Source != "resume" {
		t.Fatalf("resume event: %s %v", data, err)
	}
	assertOriginal(t, f, before)
}

type terminalStep struct{ marker, input string }

type scriptedTerminal struct {
	input    *os.File
	data     bytes.Buffer
	stepData bytes.Buffer
	queries  int
	steps    []terminalStep
}

func attachTerminal(t *testing.T, f *fixture, steps []terminalStep) {
	t.Helper()
	input, replies, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := input.Close(); err != nil {
			t.Error(err)
		}
		if err := replies.Close(); err != nil {
			t.Error(err)
		}
	})
	f.terminalInput = input
	f.terminalOutput = &scriptedTerminal{input: replies, steps: steps}
}

func (s *scriptedTerminal) Write(data []byte) (int, error) {
	if _, err := s.data.Write(data); err != nil {
		return 0, err
	}
	if _, err := s.stepData.Write(data); err != nil {
		return 0, err
	}
	queries := bytes.Count(s.data.Bytes(), []byte("\x1b[6n"))
	for s.queries < queries {
		if _, err := fmt.Fprint(s.input, "\x1b[1;1R"); err != nil {
			return 0, err
		}
		s.queries++
	}
	plain := terminalANSI.ReplaceAllString(s.stepData.String(), "")
	if len(s.steps) > 0 && strings.Contains(plain, s.steps[0].marker) {
		if _, err := fmt.Fprint(s.input, s.steps[0].input); err != nil {
			return 0, err
		}
		s.steps = s.steps[1:]
		s.stepData.Reset()
	}
	return len(data), nil
}
