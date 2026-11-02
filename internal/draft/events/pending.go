package events

import (
	"context"
	"fmt"
	"regexp"

	"github.com/agenticlab-ai/pyxis/internal/gitrefstore"
)

var featureSlug = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

const pendingFile = "events.md"

// Pending is an advisory ref beside the draft branch. It holds events from
// failed steps without changing any working file. Every clone fetches it
// before appending to the next successful step's working log.
type Pending struct {
	store *gitrefstore.Store
}

func OpenPending(repo, remote, feature string, version int) (*Pending, error) {
	if !featureSlug.MatchString(feature) || version < 1 {
		return nil, fmt.Errorf("pending draft events need a feature and positive version")
	}
	ref := fmt.Sprintf("refs/pyxis/draft-pending/%s/v%d", feature, version)
	store, err := gitrefstore.New(repo, remote, ref)
	if err != nil {
		return nil, fmt.Errorf("open draft pending ref: %w", err)
	}
	return &Pending{store: store}, nil
}

func (p *Pending) Add(ctx context.Context, event Event) error {
	if p == nil || p.store == nil {
		return fmt.Errorf("draft pending ref is unavailable")
	}
	_, err := p.store.Update(ctx, pendingFile, "append pending draft event", func(body []byte) ([]byte, bool, error) {
		return Append(body, event)
	})
	if err != nil {
		return fmt.Errorf("append pending draft event: %w", err)
	}
	return nil
}

func (p *Pending) Read(ctx context.Context) ([]Event, error) {
	if p == nil || p.store == nil {
		return nil, fmt.Errorf("draft pending ref is unavailable")
	}
	body, err := p.store.Read(ctx, pendingFile)
	if err != nil {
		return nil, fmt.Errorf("read pending draft events: %w", err)
	}
	return Parse(body)
}
