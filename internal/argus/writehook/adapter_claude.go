package writehook

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// adapter_claude wires Claude Code's own PreToolUse hook to the canonical
// Decide. It is a pointer to the shared decision — I/O translation only, no
// protected-path knowledge (DINV-5).
//
// Claude Code's real PreToolUse contract (docs: code.claude.com/docs/en/hooks):
//   - INPUT (stdin JSON): { "tool_name": "Write"|"Edit"|"MultiEdit",
//     "cwd": "<project dir>", "tool_input": { "file_path": "<abs path>", … } }.
//   - DENY: exit 0 with stdout JSON { "hookSpecificOutput": { "hookEventName":
//     "PreToolUse", "permissionDecision": "deny", "permissionDecisionReason":
//     "<reason>" } } blocks the tool and shows the reason to the agent.
//   - ALLOW: exit 0 with permissionDecision "allow" (or empty stdout).
//   - FAIL-OPEN DEFAULT: a hook that crashes, times out, or exits non-2-nonzero
//     lets the write PROCEED. The hook has no fail-closed config knob, so this
//     adapter *is* the explicit fail-closed setting: every internal error
//     renders a deny (never a bare nonzero exit), and the shim's top-level
//     recover maps a panic to the hard-deny exit code (failClosedExit = 2).
func init() {
	register(&Adapter{
		Name:           "claude",
		Hook:           "Claude Code PreToolUse hook (matcher Write|Edit|MultiEdit)",
		Honoring:       true,
		parse:          claudeParse,
		renderDeny:     claudeRenderDeny,
		renderAllow:    claudeRenderAllow,
		failClosedExit: 2, // Claude's hard-deny exit — blocks the tool when JSON can't be trusted
	})
}

// claudeRequest is the subset of Claude Code's PreToolUse stdin JSON this
// adapter reads. Unread fields are ignored; the target is read structurally
// (a planted directive elsewhere in the payload is never consulted — the
// verdict is target-path classification, DINV-1 / A-5).
type claudeRequest struct {
	ToolName  string `json:"tool_name"`
	CWD       string `json:"cwd"`
	ToolInput struct {
		FilePath string `json:"file_path"`
	} `json:"tool_input"`
}

// claudeParse extracts the pending write's repo-relative target from a
// PreToolUse request. Claude passes an absolute file_path and the project cwd;
// the target is relativized to cwd so it classifies against the repo-relative
// protected-path set. A request with no readable file_path is a hook-runtime
// error (fail closed), not a silent allow.
func claudeParse(request []byte) (string, error) {
	var r claudeRequest
	if err := json.Unmarshal(request, &r); err != nil {
		return "", fmt.Errorf("unparseable PreToolUse request: %w", err)
	}
	fp := strings.TrimSpace(r.ToolInput.FilePath)
	if fp == "" {
		return "", fmt.Errorf("PreToolUse request for tool %q carries no tool_input.file_path", r.ToolName)
	}
	// Relativize an absolute target against the project cwd; a target outside
	// the repo stays as-is (it owns no in-scope class, so it denies).
	if cwd := strings.TrimSpace(r.CWD); cwd != "" && filepath.IsAbs(fp) {
		if rel, err := filepath.Rel(cwd, fp); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel), nil
		}
	}
	return filepath.ToSlash(fp), nil
}

// claudeHookOutput is Claude Code's PreToolUse decision JSON.
type claudeHookOutput struct {
	HookSpecificOutput struct {
		HookEventName            string `json:"hookEventName"`
		PermissionDecision       string `json:"permissionDecision"`
		PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
	} `json:"hookSpecificOutput"`
}

func claudeOutput(decision, reason string) claudeHookOutput {
	var o claudeHookOutput
	o.HookSpecificOutput.HookEventName = "PreToolUse"
	o.HookSpecificOutput.PermissionDecision = decision
	o.HookSpecificOutput.PermissionDecisionReason = reason
	return o
}

// claudeRenderDeny emits Claude's structured deny (exit 0 + deny JSON), which
// blocks the tool and surfaces the three-part reason to the agent. If the JSON
// cannot be written (a broken/closed stdout), it falls back to the hard-deny
// exit code (2): an exit-0 with empty or partial stdout would be read as an
// allow (Claude's fail-open default), so a write failure must still block
// (DINV-3, AP-17) — block over explain.
func claudeRenderDeny(w io.Writer, d Decision) int {
	if err := json.NewEncoder(w).Encode(claudeOutput("deny", d.Reason())); err != nil {
		return 2 // Claude's hard-deny exit — blocks even when stdout can't be trusted
	}
	return 0
}

// claudeRenderAllow emits Claude's allow decision for an in-scope target.
func claudeRenderAllow(w io.Writer, d Decision) int {
	_ = json.NewEncoder(w).Encode(claudeOutput("allow", ""))
	return 0
}
