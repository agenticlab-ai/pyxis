package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	implreport "github.com/agenticlab-ai/pyxis/internal/implreview/report"
	implruntime "github.com/agenticlab-ai/pyxis/internal/implreview/runtime"
)

const fixtureReviewParent = "e2e-native-reviewer"

type fixtureLensResult struct {
	Schema     string               `json:"schema_version"`
	Lens       string               `json:"lens"`
	PackDigest string               `json:"pack_digest"`
	Status     string               `json:"status"`
	Candidates []implreport.Finding `json:"candidates"`
}

type fixtureRetainedLens struct {
	Lens       string               `json:"lens"`
	ContextID  string               `json:"native_context_id"`
	PackDigest string               `json:"shared_pack_digest"`
	Status     string               `json:"status"`
	Candidates []implreport.Finding `json:"candidates"`
}

// Exercise the real native-event adapters and host verifier in installed-flow
// tests. The fixture supplies four native children, not just a parent verdict.
func emitFixtureNativeLenses(cursor bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(filepath.Dir(home), "review-evidence")
	pack, err := os.ReadFile(filepath.Join(dir, "pack.txt"))
	if err != nil {
		return err
	}
	_, body, ok := bytes.Cut(pack, []byte("\n\n"))
	if !ok {
		return fmt.Errorf("fixture pack lacks a manifest")
	}
	digest := fmt.Sprintf("sha256:%x", sha256.Sum256(body))
	contextID, err := implruntime.ContextID(fixtureReviewParent)
	if err != nil {
		return err
	}
	evidence := struct {
		Schema       string                `json:"schema_version"`
		PackDigest   string                `json:"shared_pack_digest"`
		ReviewerRoot string                `json:"reviewer_root"`
		ContextID    string                `json:"context_id"`
		Lenses       []fixtureRetainedLens `json:"lenses"`
	}{Schema: "impl-review-lens-evidence/v1", PackDigest: digest, ReviewerRoot: fixtureReviewParent, ContextID: contextID}
	if !cursor {
		if err := json.NewEncoder(os.Stdout).Encode(struct {
			Type     string `json:"type"`
			ThreadID string `json:"thread_id"`
		}{"thread.started", fixtureReviewParent}); err != nil {
			return err
		}
	}
	for _, lens := range []string{implreport.LensSpecFidelity, implreport.LensDesignConformance, implreport.LensSecurityAbuse, implreport.LensCorrectnessCritic} {
		id := "e2e-native-" + lens
		candidates := []implreport.Finding{}
		wire, err := json.Marshal(fixtureLensResult{"impl-review-lens-result/v1", lens, digest, "completed", candidates})
		if err != nil {
			return err
		}
		output := "```impl-review-lens\n" + string(wire) + "\n```\n"
		evidence.Lenses = append(evidence.Lenses, fixtureRetainedLens{lens, id, digest, "completed", candidates})
		if cursor {
			if err := emitFixtureCursorChild(id, output); err != nil {
				return err
			}
		} else if err := emitFixtureCodexChild(id, output); err != nil {
			return err
		}
	}
	wire, err := json.Marshal(evidence)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "lenses.json"), wire, 0o600)
}

func emitFixtureCodexChild(id, output string) error {
	type state struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	event := struct {
		Type string `json:"type"`
		Item struct {
			Type      string           `json:"type"`
			Tool      string           `json:"tool"`
			Status    string           `json:"status"`
			Sender    string           `json:"sender_thread_id"`
			Receivers []string         `json:"receiver_thread_ids"`
			States    map[string]state `json:"agents_states,omitempty"`
		} `json:"item"`
	}{Type: "item.completed"}
	event.Item.Type, event.Item.Tool, event.Item.Status = "collab_tool_call", "spawn_agent", "completed"
	event.Item.Sender, event.Item.Receivers = fixtureReviewParent, []string{id}
	if err := json.NewEncoder(os.Stdout).Encode(event); err != nil {
		return err
	}
	event.Item.Tool, event.Item.States = "wait", map[string]state{id: {"completed", output}}
	return json.NewEncoder(os.Stdout).Encode(event)
}

func emitFixtureCursorChild(id, output string) error {
	event := struct {
		Type      string `json:"type"`
		Subtype   string `json:"subtype"`
		SessionID string `json:"session_id"`
		CallID    string `json:"call_id"`
		ToolCall  struct {
			Task struct {
				Result struct {
					Success struct {
						AgentID string `json:"agentId"`
						Steps   []struct {
							Assistant struct {
								Text string `json:"text"`
							} `json:"assistantMessage"`
						} `json:"conversationSteps"`
					} `json:"success"`
				} `json:"result"`
			} `json:"taskToolCall"`
		} `json:"tool_call"`
	}{Type: "tool_call", Subtype: "completed", SessionID: fixtureReviewParent, CallID: id}
	success := &event.ToolCall.Task.Result.Success
	success.AgentID = id
	success.Steps = append(success.Steps, struct {
		Assistant struct {
			Text string `json:"text"`
		} `json:"assistantMessage"`
	}{})
	success.Steps[0].Assistant.Text = output
	return json.NewEncoder(os.Stdout).Encode(event)
}
