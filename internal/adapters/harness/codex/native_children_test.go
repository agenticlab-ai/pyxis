package codex

import (
	"strings"
	"testing"
)

func TestNativeReviewChildrenRequireSpawnAndTerminalWait(t *testing.T) {
	spawn := `{"type":"item.completed","item":{"type":"collab_tool_call","tool":"spawn_agent","status":"completed","sender_thread_id":"parent","receiver_thread_ids":["child"]}}`
	wait := `{"type":"item.completed","item":{"type":"collab_tool_call","tool":"wait","status":"completed","sender_thread_id":"parent","receiver_thread_ids":["child"],"agents_states":{"child":{"status":"completed","message":"native child withheld judgment"}}}}`
	for _, kind := range []string{"complete", "duplicate", "failed", "running", "pending", "missing-state", "missing-wait", "missing-spawn", "foreign-parent", "missing-parent", "parent-as-child", "blank-child", "wrong-tool", "nonterminal-tool"} {
		t.Run(kind, func(t *testing.T) {
			body := spawn + "\n" + wait
			prefix := `{"type":"thread.started","thread_id":"parent"}` + "\n"
			switch kind {
			case "duplicate":
				body += "\n" + spawn + "\n" + wait
			case "failed":
				body = spawn + "\n" + strings.Replace(wait, `"child":{"status":"completed"`, `"child":{"status":"errored"`, 1)
			case "running", "pending":
				status := map[string]string{"running": "running", "pending": "pending_init"}[kind]
				body = spawn + "\n" + strings.Replace(wait, `"child":{"status":"completed"`, `"child":{"status":"`+status+`"`, 1)
			case "missing-state":
				body = spawn + "\n" + strings.Replace(wait, `"agents_states":{"child":`, `"agents_states":{"other":`, 1)
			case "missing-wait":
				body = spawn
			case "missing-spawn":
				body = wait
			case "foreign-parent":
				body = strings.ReplaceAll(body, `"sender_thread_id":"parent"`, `"sender_thread_id":"other"`)
			case "missing-parent":
				prefix = ""
			case "parent-as-child":
				body = strings.ReplaceAll(body, `"receiver_thread_ids":["child"]`, `"receiver_thread_ids":["parent"]`)
			case "blank-child":
				body = strings.ReplaceAll(body, `"receiver_thread_ids":["child"]`, `"receiver_thread_ids":[""]`)
			case "wrong-tool":
				body = strings.ReplaceAll(body, `"tool":"spawn_agent"`, `"tool":"send_input"`)
			case "nonterminal-tool":
				body = strings.ReplaceAll(body, `"status":"completed"`, `"status":"in_progress"`)
			}
			result, err := parseResult([]byte(prefix + body + "\n" + `{"type":"item.completed","item":{"type":"agent_message","text":"parent approves"}}` + "\n" + `{"type":"turn.completed","usage":{"input_tokens":1,"output_tokens":1}}`))
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "complete", "duplicate", "failed", "running", "pending", "missing-state", "missing-wait":
				if result.ReviewChildren == nil || len(result.ReviewChildren.Children) != 1 {
					t.Fatalf("native child lost: %+v", result)
				}
				child := result.ReviewChildren.Children[0]
				if child.Completed != (kind == "complete" || kind == "duplicate") || (child.Completed && child.Output != "native child withheld judgment") {
					t.Fatalf("native completion became parent approval: %+v", child)
				}
			default:
				if result.ReviewChildren != nil {
					t.Fatalf("unproven child accepted: %+v", result.ReviewChildren)
				}
			}
		})
	}
}
