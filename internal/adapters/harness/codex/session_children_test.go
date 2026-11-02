package codex

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// sessionMeta is the first line Codex writes to a thread's session record.
func sessionMeta(id, parent, source, agentPath string) string {
	return fmt.Sprintf(`{"type":"session_meta","payload":{"id":%q,"parent_thread_id":%q,"thread_source":%q,"agent_path":%q,"source":{"subagent":{"thread_spawn":{"parent_thread_id":%q}}}}}`, id, parent, source, agentPath, parent)
}

const (
	sessionTaskStarted = `{"type":"event_msg","payload":{"type":"task_started"}}`
	sessionOtherItem   = `{"type":"response_item","payload":{"type":"message","role":"assistant"}}`
)

func sessionTaskComplete(message string) string {
	return fmt.Sprintf(`{"type":"event_msg","payload":{"type":"task_complete","last_agent_message":%q}}`, message)
}

func writeSessionRecord(t *testing.T, sessions, name string, lines ...string) {
	t.Helper()
	path := filepath.Join(sessions, "2026", "10", "11", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestSessionsDirFollowsTheLaunchCodexHome(t *testing.T) {
	env := []string{"PATH=/bin", "CODEX_HOME=/state/model-auth/codex", "HOME=/state/home"}
	if got, want := sessionsDir(env), filepath.Join("/state/model-auth/codex", "sessions"); got != want {
		t.Fatalf("sessionsDir = %q, want %q", got, want)
	}
	for _, env := range [][]string{nil, {"PATH=/bin"}, {"CODEX_HOME="}} {
		if got := sessionsDir(env); got != "" {
			t.Fatalf("sessionsDir(%v) = %q, want none", env, got)
		}
	}
}

// Current Codex names a spawned child only by task name and reports no child
// identity in its event stream. The children of one reviewer are read from the
// session records instead: its own, in spawn order, and nobody else's.
func TestSessionReviewChildrenAreThisReviewersOwn(t *testing.T) {
	sessions := t.TempDir()
	writeSessionRecord(t, sessions, "rollout-2026-10-11T08-00-00-parent.jsonl",
		`{"type":"session_meta","payload":{"id":"parent","thread_source":"user","source":"exec"}}`, sessionTaskStarted, sessionTaskComplete("aggregate"))
	writeSessionRecord(t, sessions, "rollout-2026-10-11T08-00-01-child-a.jsonl",
		sessionMeta("child-a", "parent", "subagent", "/root/spec_fidelity"), sessionTaskStarted, sessionOtherItem, "not json", sessionTaskComplete("lens a"))
	writeSessionRecord(t, sessions, "rollout-2026-10-11T08-00-02-child-b.jsonl",
		sessionMeta("child-b", "parent", "subagent", "/root/design_conformance"), sessionTaskStarted)
	writeSessionRecord(t, sessions, "rollout-2026-10-11T08-00-03-child-c.jsonl",
		sessionMeta("child-c", "parent", "subagent", "/root/security_abuse"), sessionTaskStarted, sessionTaskComplete("first answer"), sessionTaskStarted)
	// Everything below must be ignored.
	writeSessionRecord(t, sessions, "rollout-2026-10-11T08-00-04-foreign.jsonl",
		sessionMeta("foreign", "another-parent", "subagent", "/root/spec_fidelity"), sessionTaskComplete("foreign"))
	writeSessionRecord(t, sessions, "rollout-2026-10-11T08-00-05-not-a-child.jsonl",
		sessionMeta("sibling", "parent", "user", ""), sessionTaskComplete("sibling"))
	writeSessionRecord(t, sessions, "rollout-2026-10-11T08-00-06-self.jsonl",
		sessionMeta("parent", "parent", "subagent", "/root/self"), sessionTaskComplete("self"))
	writeSessionRecord(t, sessions, "rollout-2026-10-11T08-00-07-blank.jsonl",
		sessionMeta("", "parent", "subagent", "/root/blank"), sessionTaskComplete("blank"))
	writeSessionRecord(t, sessions, "rollout-2026-10-11T08-00-08-garbled.jsonl", "not json", sessionTaskComplete("garbled"))
	writeSessionRecord(t, sessions, "rollout-2026-10-11T08-00-09-child-a-again.jsonl",
		sessionMeta("child-a", "parent", "subagent", "/root/spec_fidelity"), sessionTaskComplete("a second record"))
	writeSessionRecord(t, sessions, "notes.jsonl", sessionMeta("stray", "parent", "subagent", "/root/stray"), sessionTaskComplete("stray"))

	got := sessionReviewChildren(t.Context(), sessions, "parent")
	want := &port.NativeReviewChildren{Children: []port.NativeReviewChild{
		{ContextID: "child-a", InvocationID: "/root/spec_fidelity", Completed: true, Output: "lens a"},
		{ContextID: "child-b", InvocationID: "/root/design_conformance"},
		{ContextID: "child-c", InvocationID: "/root/security_abuse"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("children = %+v\nwant     %+v", got, want)
	}
}

func TestSessionReviewChildrenFailClosed(t *testing.T) {
	sessions := t.TempDir()
	writeSessionRecord(t, sessions, "rollout-child.jsonl",
		sessionMeta("child", "parent", "subagent", "/root/spec_fidelity"), sessionTaskStarted, sessionTaskComplete("lens"))

	if got := sessionReviewChildren(t.Context(), sessions, ""); got != nil {
		t.Fatalf("children without a reviewer identity: %+v", got)
	}
	if got := sessionReviewChildren(t.Context(), filepath.Join(sessions, "missing"), "parent"); got != nil {
		t.Fatalf("children from a missing directory: %+v", got)
	}
	if got := sessionReviewChildren(t.Context(), sessions, "someone-else"); got != nil {
		t.Fatalf("children of another reviewer: %+v", got)
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if got := sessionReviewChildren(cancelled, sessions, "parent"); got != nil {
		t.Fatalf("children read after cancellation: %+v", got)
	}

	// A read cut short cannot vouch for the child's last turn.
	root, err := os.OpenRoot(sessions)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }() // read-only handle; no state to flush
	child, ok := readSessionChild(cancelled, root, "2026/10/11/rollout-child.jsonl", "parent")
	if !ok || child.ContextID != "child" || child.Completed || child.Output != "" {
		t.Fatalf("interrupted read = (%+v, %t), want the child without a completion", child, ok)
	}
	if _, ok := readSessionChild(t.Context(), root, "2026/10/11/absent.jsonl", "parent"); ok {
		t.Fatal("an absent record yielded a child")
	}
}

func TestSessionReviewChildrenAreBounded(t *testing.T) {
	sessions := t.TempDir()
	for i := range maxSessionRecords + 3 {
		id := fmt.Sprintf("child-%03d", i)
		writeSessionRecord(t, sessions, "rollout-"+id+".jsonl", sessionMeta(id, "parent", "subagent", "/root/"+id), sessionTaskComplete(id))
	}
	got := sessionReviewChildren(t.Context(), sessions, "parent")
	if got == nil || len(got.Children) != maxSessionRecords || got.Children[0].ContextID != "child-000" {
		t.Fatalf("bounded read = %+v", got)
	}
}

// A review launch keeps its sessions inside its own isolated home and Await
// reads the children from them once the process tree is gone. An
// implementation launch never consults them, and children the event stream
// itself proves take precedence.
func TestReviewAwaitReadsChildrenFromItsOwnSessionRecords(t *testing.T) {
	const stream = `{"type":"thread.started","thread_id":"parent"}
{"type":"item.completed","item":{"type":"collab_tool_call","tool":"wait","status":"completed","sender_thread_id":"parent","receiver_thread_ids":[],"agents_states":{}}}
{"type":"item.completed","item":{"type":"agent_message","text":"aggregate"}}
{"type":"turn.completed","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`
	state := t.TempDir()
	sessions := filepath.Join(state, "model-auth", "codex", "sessions")
	writeSessionRecord(t, sessions, "rollout-child.jsonl",
		sessionMeta("child", "parent", "subagent", "/root/spec_fidelity"), sessionTaskStarted, sessionTaskComplete("lens"))
	want := []port.NativeReviewChild{{ContextID: "child", InvocationID: "/root/spec_fidelity", Completed: true, Output: "lens"}}

	review := port.LaunchSpec{Purpose: port.LaunchPurposeReview, Model: "gpt-5.6-sol", Effort: "high", Brief: "review", Workdir: t.TempDir(), RuntimeStateDir: state}
	fc := &fakeCommander{procStdout: []byte(stream)}
	a := mustAdapter(t, withCommander(fc))
	h, err := a.Launch(t.Context(), review)
	if err != nil {
		t.Fatal(err)
	}
	if got := h.(*handle).sessions; got != sessions {
		t.Fatalf("review handle reads sessions from %q, want %q", got, sessions)
	}
	result, err := a.Await(t.Context(), h)
	if err != nil {
		t.Fatal(err)
	}
	if result.ReviewChildren == nil || !reflect.DeepEqual(result.ReviewChildren.Children, want) {
		t.Fatalf("review children = %+v, want %+v", result.ReviewChildren, want)
	}

	implementation := review
	implementation.Purpose, implementation.HeadBranch = port.LaunchPurposeImplementation, "feat/x"
	h, err = a.Launch(t.Context(), implementation)
	if err != nil {
		t.Fatal(err)
	}
	if got := h.(*handle).sessions; got != "" {
		t.Fatalf("implementation handle reads sessions from %q", got)
	}
	if result, err = a.Await(t.Context(), h); err != nil || result.ReviewChildren != nil {
		t.Fatalf("implementation launch reported review children: %+v (%v)", result.ReviewChildren, err)
	}

	proven := `{"type":"thread.started","thread_id":"parent"}
{"type":"item.completed","item":{"type":"collab_tool_call","tool":"spawn_agent","status":"completed","sender_thread_id":"parent","receiver_thread_ids":["event-child"]}}
{"type":"item.completed","item":{"type":"collab_tool_call","tool":"wait","status":"completed","sender_thread_id":"parent","receiver_thread_ids":["event-child"],"agents_states":{"event-child":{"status":"completed","message":"from the stream"}}}}
{"type":"turn.completed","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`
	fc.procStdout = []byte(proven)
	if h, err = a.Launch(t.Context(), review); err != nil {
		t.Fatal(err)
	}
	result, err = a.Await(t.Context(), h)
	if err != nil {
		t.Fatal(err)
	}
	if result.ReviewChildren == nil || len(result.ReviewChildren.Children) != 1 || result.ReviewChildren.Children[0].ContextID != "event-child" {
		t.Fatalf("stream-proven children were replaced: %+v", result.ReviewChildren)
	}
}
