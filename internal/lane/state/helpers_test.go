package state_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/lane/state"
)

// --- the local-substitutable seams (merge-lane-v1 §3.8, no injected mocks) ---

// memStore is an in-memory lane-state ref: a block and the platform-attributed
// writer of its tip. AppendState is synchronous — it returns only after the
// block grows, exactly the durability WriteHalt relies on. An injected err
// exercises the fail-closed write path.
type memStore struct {
	block     []byte
	writer    state.Writer
	appendErr error
	appends   int
}

func (s *memStore) ReadState(_ context.Context) ([]byte, state.Writer, error) {
	return s.block, s.writer, nil
}

func (s *memStore) AppendState(_ context.Context, e state.Entry) error {
	if s.appendErr != nil {
		return s.appendErr
	}
	s.appends++
	s.block = state.AppendBlock(s.block, e)
	return nil
}

// memAlarm captures every alarm raised on the delivery ledger.
type memAlarm struct {
	events   []state.AlarmEvent
	raiseErr error
}

func (a *memAlarm) Raise(_ context.Context, ev state.AlarmEvent) error {
	if a.raiseErr != nil {
		return a.raiseErr
	}
	a.events = append(a.events, ev)
	return nil
}

// memDisp is an in-memory delivery-ledger disposition set: a citation is a
// recorded human disposition iff it is in the set. An injected err exercises the
// unverifiable-clear fail-closed path.
type memDisp struct {
	has map[string]bool
	err error
}

func newDisp(refs ...state.DispositionRef) *memDisp {
	m := &memDisp{has: map[string]bool{}}
	for _, r := range refs {
		m.has[r.String()] = true
	}
	return m
}

func (d *memDisp) HasDisposition(_ context.Context, ref state.DispositionRef) (bool, error) {
	if d.err != nil {
		return false, d.err
	}
	return d.has[ref.String()], nil
}

var errDisp = errors.New("delivery ledger unreadable")

// --- shared identities ---

var (
	machineWriter = state.Writer{Name: "pyxis-lane-bot", Machine: true}
	humanWriter   = state.Writer{Name: "mallory", Machine: false}
)

// incidentFixture stands up a temp incident registry the coded writer appends
// to, returning the root and its config (the incidents package's own test
// pattern).
const baseRegistry = `# Incidents registry

## Entries

**I1 (2026-07-07) [docgate, tech-design]**

- **Decision:** d
- **Broke:** b
- **Lesson:** l
- **Links:** PR #38.
`

func incidentFixture(t *testing.T) (string, config.Config) {
	t.Helper()
	root := t.TempDir()
	cfg := config.Defaults()
	p := filepath.Join(root, filepath.FromSlash(cfg.Incidents))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(baseRegistry), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, cfg
}

func readRegistry(t *testing.T, root string, cfg config.Config) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(cfg.Incidents)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
