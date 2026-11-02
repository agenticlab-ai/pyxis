package config_test

import (
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/config"
)

func TestSessionContinuityKeyAndRemediationDefault(t *testing.T) {
	cfg, err := config.Parse([]byte("executor:\n  review:\n    backend: claude\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.SessionContinuity() || cfg.ReviewRemediationRounds() != 3 {
		t.Fatalf("defaults: continuity=%v rounds=%d", cfg.SessionContinuity(), cfg.ReviewRemediationRounds())
	}
	cfg, err = config.Parse([]byte("executor:\n  session:\n    continuity: false\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SessionContinuity() {
		t.Fatal("continuity: false must disable")
	}
	if _, err := config.Parse([]byte("executor:\n  session:\n    continuity: maybe\n")); !errors.Is(err, config.ErrMalformed) {
		t.Errorf("bad bool: %v", err)
	}
	if _, err := config.Parse([]byte("executor:\n  session:\n    ttl: 1\n")); !errors.Is(err, config.ErrUnknownKey) {
		t.Errorf("unknown session key: %v", err)
	}
}
