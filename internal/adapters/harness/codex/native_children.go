package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// Native collab events bind a wait result to a child actually spawned by this
// thread. Parent agent-message text cannot supply that proof.
type codexReviewChildren struct {
	order []string
	byID  map[string]port.NativeReviewChild
}

func (c *codexReviewChildren) observe(ev codexEvent, parent string) {
	item := ev.Item
	if ev.Type != "item.completed" || item == nil || item.Type != "collab_tool_call" || item.Status != "completed" || parent == "" || item.Sender != parent {
		return
	}
	if c.byID == nil {
		c.byID = make(map[string]port.NativeReviewChild)
	}
	for _, id := range item.Receivers {
		if id == "" || id == parent {
			continue
		}
		switch item.Tool {
		case "spawn_agent":
			if _, exists := c.byID[id]; !exists {
				c.order = append(c.order, id)
				c.byID[id] = port.NativeReviewChild{ContextID: id}
			}
		case "wait":
			if _, spawned := c.byID[id]; !spawned {
				continue
			}
			state, exists := item.States[id]
			if !exists || state.Status == "running" || state.Status == "pending_init" {
				continue
			}
			c.byID[id] = port.NativeReviewChild{ContextID: id, Completed: state.Status == "completed", Output: state.Message}
		}
	}
}

func (c *codexReviewChildren) result() *port.NativeReviewChildren {
	if len(c.order) == 0 {
		return nil
	}
	result := &port.NativeReviewChildren{Children: make([]port.NativeReviewChild, 0, len(c.order))}
	for _, id := range c.order {
		result.Children = append(result.Children, c.byID[id])
	}
	return result
}

const (
	// A review spawns four lens children; the bound leaves room for a reviewer
	// that respawned some without letting a runaway run be read forever.
	maxSessionRecords     = 64
	maxSessionRecordBytes = 32 << 20
)

// sessionsDir is where the CLI records each thread of a launch whose
// environment redirects its home: the sessions directory under CODEX_HOME.
func sessionsDir(env []string) string {
	for _, item := range env {
		if home, ok := strings.CutPrefix(item, "CODEX_HOME="); ok && home != "" {
			return filepath.Join(home, "sessions")
		}
	}
	return ""
}

// sessionReviewChildren reads the children of one reviewer thread from the
// session records Codex wrote under dir. Current Codex returns only a task
// name from spawn_agent and reports no child identity in the exec event
// stream, so these records are the one place a child's thread, its parent, and
// its final response appear. A record whose parent is not this reviewer is
// ignored, and an unreadable directory yields no children.
func sessionReviewChildren(ctx context.Context, dir, parent string) *port.NativeReviewChildren {
	if parent == "" {
		return nil
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil
	}
	defer func() { _ = root.Close() }() // read-only handle; no state to flush
	var names []string
	// The callback only ever stops the walk; it returns no error to report.
	_ = fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil || ctx.Err() != nil {
			return fs.SkipAll
		}
		base := path.Base(name)
		if entry.Type().IsRegular() && strings.HasPrefix(base, "rollout-") && strings.HasSuffix(base, ".jsonl") {
			if len(names) == maxSessionRecords {
				return fs.SkipAll
			}
			names = append(names, name)
		}
		return nil
	})
	// A record's path carries its date and start time, so name order is spawn order.
	sort.Strings(names)
	result := &port.NativeReviewChildren{}
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		child, ok := readSessionChild(ctx, root, name, parent)
		if !ok || seen[child.ContextID] {
			continue
		}
		seen[child.ContextID] = true
		result.Children = append(result.Children, child)
	}
	if len(result.Children) == 0 {
		return nil
	}
	return result
}

// sessionRecordLine is the subset of one session-record line this adapter
// reads, parsed open like codexEvent: third-party output that grows fields.
type sessionRecordLine struct {
	Type    string `json:"type"`
	Payload struct {
		Type      string `json:"type"`
		ID        string `json:"id"`
		Parent    string `json:"parent_thread_id"`
		Source    string `json:"thread_source"`
		AgentPath string `json:"agent_path"`
		Last      string `json:"last_agent_message"`
	} `json:"payload"`
}

// readSessionChild reads one record. Its first line must name a subagent
// thread spawned by parent. The child counts as completed only when its last
// turn ended with task_complete; that event's message is its final response.
func readSessionChild(ctx context.Context, root *os.Root, name, parent string) (port.NativeReviewChild, bool) {
	file, err := root.Open(name)
	if err != nil {
		return port.NativeReviewChild{}, false
	}
	defer func() { _ = file.Close() }() // read-only handle; no state to flush
	limited := &io.LimitedReader{R: file, N: maxSessionRecordBytes}
	reader := bufio.NewReader(limited)
	var child port.NativeReviewChild
	for first := true; ; first = false {
		line, readErr := reader.ReadBytes('\n')
		var record sessionRecordLine
		// A line that does not decode is skipped, as in the event stream.
		decoded := len(line) > 0 && json.Unmarshal(line, &record) == nil
		if first {
			meta := record.Payload
			if !decoded || record.Type != "session_meta" || meta.Source != "subagent" || meta.Parent != parent || meta.ID == "" || meta.ID == parent {
				return port.NativeReviewChild{}, false
			}
			child = port.NativeReviewChild{ContextID: meta.ID, InvocationID: meta.AgentPath}
		} else if decoded && record.Type == "event_msg" {
			switch record.Payload.Type {
			case "task_started":
				child.Completed, child.Output = false, ""
			case "task_complete":
				child.Completed, child.Output = true, record.Payload.Last
			}
		}
		if readErr != nil || ctx.Err() != nil {
			// A read that stopped before the record's end cannot vouch for its
			// last turn.
			if !errors.Is(readErr, io.EOF) || limited.N == 0 {
				child.Completed, child.Output = false, ""
			}
			return child, true
		}
	}
}
