package codegatemirror

import (
	"context"
	"slices"
	"sync"
	"testing"
)

func TestProgressPairsStartedAndTerminalChecksIncludingSkippedPhases(t *testing.T) {
	var mu sync.Mutex
	var events []Progress
	ctx := WithProgressObserver(t.Context(), func(event Progress) { mu.Lock(); defer mu.Unlock(); events = append(events, event) })
	cfg := Config{Checks: []Check{{Name: "first", Command: "false", Class: ClassRequired}, {Name: "later", Command: "true", Class: ClassRequired, Phase: 2}}}
	rep := cfg.run(ctx, t.TempDir(), func(context.Context, string, Check) Outcome { return Outcome{Name: "first", Class: ClassRequired} })
	if rep.Passed {
		t.Fatal("failed required gate passed")
	}
	var got []string
	for _, event := range events {
		got = append(got, event.Name+":"+event.Status)
	}
	if !slices.Equal(got, []string{"first:started", "first:failed", "later:skipped"}) {
		t.Fatalf("progress: %v", got)
	}
}
