package prdescription

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/changesize"
)

// Event is the part of a pull request event the check reads.
type Event struct {
	Number  int
	Body    string
	BaseRef string
	BaseSHA string
	HeadRef string
	HeadSHA string
}

// ReadEvent reads a GitHub pull_request event payload, the file
// GITHUB_EVENT_PATH names. The payload is the code host's, so fields the
// check does not read are ignored.
func ReadEvent(path string) (Event, error) {
	if path == "" {
		return Event{}, errors.New("prdescription: no event payload path (GITHUB_EVENT_PATH is unset)")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return Event{}, fmt.Errorf("prdescription: read event payload: %w", err)
	}
	type ref struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	}
	var payload struct {
		PullRequest *struct {
			Number int     `json:"number"`
			Body   *string `json:"body"`
			Base   ref     `json:"base"`
			Head   ref     `json:"head"`
		} `json:"pull_request"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return Event{}, fmt.Errorf("prdescription: decode event payload: %w", err)
	}
	pr := payload.PullRequest
	if pr == nil {
		return Event{}, errors.New("prdescription: the event is not a pull request event")
	}
	if pr.Base.Ref == "" || pr.Base.SHA == "" || pr.Head.Ref == "" || pr.Head.SHA == "" {
		return Event{}, errors.New("prdescription: the pull request event lacks its base or head")
	}
	e := Event{Number: pr.Number, BaseRef: pr.Base.Ref, BaseSHA: pr.Base.SHA, HeadRef: pr.Head.Ref, HeadSHA: pr.Head.SHA}
	if pr.Body != nil {
		e.Body = *pr.Body
	}
	return e, nil
}

// GateResult is the check's verdict on one pull request.
type GateResult struct {
	// Exempt is set for a pull request from a feature-draft branch, whose
	// description is the approval view the feature-draft contract defines.
	Exempt   bool
	Size     changesize.Size
	Problems []Problem
}

// Gate checks the description of the pull request e describes, in the
// checkout at root. The change is measured against the base branch as the
// checkout last fetched it (remote/BaseRef), which is how the code host shows
// the diff; the event's base commit is the fallback when that ref is absent.
func Gate(ctx context.Context, root string, e Event, remote, draftPrefix string) (GateResult, error) {
	if draftPrefix != "" && strings.HasPrefix(e.HeadRef, draftPrefix) {
		return GateResult{Exempt: true}, nil
	}
	base := e.BaseSHA
	tracking := remote + "/" + e.BaseRef
	if err := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "--verify", "--quiet", "--end-of-options", tracking+"^{commit}").Run(); err == nil {
		base = tracking
	}
	size, err := changesize.Measure(ctx, root, base, e.HeadSHA)
	if err != nil {
		return GateResult{}, fmt.Errorf("prdescription: measure pull request #%d: %w", e.Number, err)
	}
	return GateResult{Size: size, Problems: Check(e.Body, size)}, nil
}
