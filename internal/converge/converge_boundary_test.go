package converge

import (
	"context"
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
)

func TestConvergeEntryPointsNameUnavailableReleaseInputs(t *testing.T) {
	cfg := resolve.Config{Root: t.TempDir()}
	if _, err := Run(t.Context(), cfg, Options{}); err == nil {
		t.Fatal("Run accepted a release operand without pins")
	}
	if _, err := Drift(t.Context(), cfg); err == nil {
		t.Fatal("Drift accepted a release operand without pins")
	}
	if _, _, err := ClassifyWithArchive(context.Background(), resolve.Config{}); err == nil {
		t.Fatal("ClassifyWithArchive accepted an empty root")
	}
	if _, _, err := ClassifyWithArchive(t.Context(), cfg); err == nil {
		t.Fatal("ClassifyWithArchive accepted a release operand without pins")
	}

	// A nil logger deliberately falls back to slog.Default; emitting an event is
	// observational and must remain safe for library callers with a zero logger.
	emit(t.Context(), cfg, Report{RunID: "boundary", Mode: ModeNoop})
}

func TestConvergeGuardAndModeBoundaries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := guard(ctx, resolve.Config{Root: t.TempDir()}); !errors.Is(err, context.Canceled) {
		t.Fatalf("guard canceled error = %v", err)
	}
	if err := guard(t.Context(), resolve.Config{Root: t.TempDir(), Repo: config.Config{System: config.System{Mode: "source"}}}); !errors.Is(err, ErrSourceMode) {
		t.Fatalf("guard source-mode error = %v", err)
	}
	if got := mode(Options{}, 1); got != ModeInstall {
		t.Fatalf("mode install = %q", got)
	}
	if got := mode(Options{Repair: true}, 1); got != ModeRepair {
		t.Fatalf("mode repair = %q", got)
	}
}
