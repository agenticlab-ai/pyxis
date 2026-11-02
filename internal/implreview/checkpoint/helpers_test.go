package checkpoint_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// fakePort is a hermetic persist.Port for the integration leg: it stores each
// persisted review as its COMPOSED artifact body (persist.ComposeArtifact — the
// identical serialization the real GitHub adapter writes) keyed by comment id,
// and reads it back through persist.ParseArtifact (the same parser the real
// adapter uses). So the harvest reads REAL payload bytes off the medium — the
// "reconstruct from payloads alone" contract (DINV-8). No network, no `gh`.
type fakePort struct {
	mu     sync.Mutex
	bodies map[string]string // comment id → artifact body
	next   int
}

func newFakePort() *fakePort { return &fakePort{bodies: map[string]string{}} }

// put stores a review payload as its composed artifact and returns the ref the
// enumerator will hand the harvest.
func (p *fakePort) put(t *testing.T, r *report.Report) persist.Ref {
	t.Helper()
	body, err := persist.ComposeArtifact(r)
	if err != nil {
		t.Fatalf("compose artifact: %v", err)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.next++
	id := fmt.Sprintf("c%d", p.next)
	p.bodies[id] = body
	return persist.Ref{Medium: persist.MediumGitHub, Locator: id}
}

func (p *fakePort) Persist(_ context.Context, _ *report.Report) (persist.Ref, error) {
	return persist.Ref{}, errors.New("fakePort.Persist not used by the harvest")
}

func (p *fakePort) Read(_ context.Context, ref persist.Ref) (*report.Report, error) {
	p.mu.Lock()
	body, ok := p.bodies[ref.Locator]
	p.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("fakePort: no artifact %s", ref.Locator)
	}
	return persist.ParseArtifact(body)
}

// fakeAuthors is a hermetic AuthorResolver: an in-memory map from an attribution
// reference to the login of the artifact's medium author — the anti-forgery
// oracle a real GitHub API would report as the PR review/comment's `user`. No
// network.
type fakeAuthors struct {
	byRef map[string]string // attribution ref → author login
}

func newFakeAuthors() *fakeAuthors { return &fakeAuthors{byRef: map[string]string{}} }

func (a *fakeAuthors) AuthorOf(_ context.Context, attribution string) (string, bool, error) {
	login, ok := a.byRef[attribution]
	return login, ok, nil
}

// refPtr is the &-of-a-copy helper the enumerator uses to carry a review ref.
func refPtr(r persist.Ref) *persist.Ref { return &r }
