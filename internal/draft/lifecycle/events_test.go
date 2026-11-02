package lifecycle

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/events"
)

type rejectingPending struct {
	readErr error
	addErr  error
}

func (p rejectingPending) Read(context.Context) ([]events.Event, error) { return nil, p.readErr }
func (p rejectingPending) Add(context.Context, events.Event) error      { return p.addErr }

func TestScaffoldEventsRejectsUnreadableOrMalformedLog(t *testing.T) {
	opened, err := events.New(events.DraftOpened, "widget", "sha256:fixture", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	opened.Origin = "new"
	if _, err := scaffoldEvents(t.Context(), []scaffoldFile{{Path: "events.md", Body: []byte("# Draft events\n")}}, rejectingPending{readErr: errors.New("offline")}, opened); err == nil || !strings.Contains(err.Error(), "read pending") {
		t.Fatalf("pending read error = %v", err)
	}
	if _, err := scaffoldEvents(t.Context(), []scaffoldFile{{Path: "events.md", Body: []byte("malformed")}}, rejectingPending{}, opened); err == nil || !strings.Contains(err.Error(), "append draft opening") {
		t.Fatalf("malformed log error = %v", err)
	}
	if _, err := scaffoldEvents(t.Context(), nil, rejectingPending{}, opened); err == nil || !strings.Contains(err.Error(), "no events.md") {
		t.Fatalf("missing log error = %v", err)
	}
}

func TestStepFailureReportsPendingRefFailure(t *testing.T) {
	prior := newPendingEvents
	t.Cleanup(func() { newPendingEvents = prior })
	cause := errors.New("draft step broke")
	newPendingEvents = func(string, string, string, int) (pendingEvents, error) {
		return nil, errors.New("open denied")
	}
	if err := recordStepFailure(t.Context(), t.TempDir(), config.Defaults(), "widget", 1, "review", cause); !errors.Is(err, cause) || !strings.Contains(err.Error(), "open denied") {
		t.Fatalf("pending open error = %v", err)
	}
	newPendingEvents = func(string, string, string, int) (pendingEvents, error) {
		return rejectingPending{addErr: errors.New("push denied")}, nil
	}
	if err := recordStepFailure(t.Context(), t.TempDir(), config.Defaults(), "widget", 1, "review", cause); !errors.Is(err, cause) || !strings.Contains(err.Error(), "push denied") {
		t.Fatalf("pending write error = %v", err)
	}
}

func TestAbandonmentEventsRefusesInvalidEvidence(t *testing.T) {
	root, _, cfg := abandonmentFixture(t)
	prior := newPendingEvents
	newPendingEvents = func(string, string, string, int) (pendingEvents, error) { return rejectingPending{}, nil }
	t.Cleanup(func() { newPendingEvents = prior })
	if _, err := abandonmentEvents(t.Context(), t.TempDir(), cfg, "widget", 1, "HEAD"); err == nil || !strings.Contains(err.Error(), "open abandoned draft") {
		t.Fatalf("missing Git repository error = %v", err)
	}
	if _, err := abandonmentEvents(t.Context(), root, cfg, "widget", 1, "missing-head"); err == nil || !strings.Contains(err.Error(), "find draft history base") {
		t.Fatalf("missing draft head error = %v", err)
	}
	newPendingEvents = func(string, string, string, int) (pendingEvents, error) {
		return rejectingPending{readErr: errors.New("remote unavailable")}, nil
	}
	if _, err := abandonmentEvents(t.Context(), root, cfg, "widget", 1, gitTest(t, root, "rev-parse", "HEAD")); err == nil || !strings.Contains(err.Error(), "read pending abandonment events") {
		t.Fatalf("pending read error = %v", err)
	}
	newPendingEvents = func(string, string, string, int) (pendingEvents, error) { return rejectingPending{}, nil }
	logPath := filepath.Join(root, "drafts/widget/events.md")
	if err := os.WriteFile(logPath, []byte("malformed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitTest(t, root, "add", "--", "drafts/widget/events.md")
	gitTest(t, root, "commit", "-m", "invalid event log")
	if _, err := abandonmentEvents(t.Context(), root, cfg, "widget", 1, gitTest(t, root, "rev-parse", "HEAD")); err == nil || !strings.Contains(err.Error(), "retain abandoned draft events") {
		t.Fatalf("invalid event error = %v", err)
	}
	foreign, err := events.New(events.DraftOpened, "other", "sha256:fixture", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	foreign.Origin = "new"
	body, _, err := events.Append(nil, foreign)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, body, 0o644); err != nil {
		t.Fatal(err)
	}
	gitTest(t, root, "add", "--", "drafts/widget/events.md")
	gitTest(t, root, "commit", "-m", "foreign event log")
	if _, err := abandonmentEvents(t.Context(), root, cfg, "widget", 1, gitTest(t, root, "rev-parse", "HEAD")); err == nil || !strings.Contains(err.Error(), "belongs to other") {
		t.Fatalf("foreign event error = %v", err)
	}
}
