package registry

import (
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

func TestRegisterAndRetrieve(t *testing.T) {
	check := &schema.Check{
		ID:                "test-check-1",
		Rung:              "0",
		VerbDeps:          []string{},
		AuthorIndependent: true,
		InputSet:          []string{"tree"},
	}

	Register(check)

	retrieved := GetCheck(check.ID)
	if retrieved == nil {
		t.Fatal("check not found after registration")
	}
	if retrieved.ID != check.ID {
		t.Errorf("ID mismatch: got %s, want %s", retrieved.ID, check.ID)
	}
}

func TestDuplicateRegistrationPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate registration, got none")
		}
	}()

	check := &schema.Check{
		ID:   "duplicate-check",
		Rung: "0",
	}

	Register(check)
	Register(check)
}

func TestUnknownRungRejected(t *testing.T) {
	// #402 review: a check with a rung outside the canonical set must be rejected
	// at registration, not silently sorted into the instant (rung-0) band.
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic registering an unknown rung, got none")
		}
	}()
	Register(&schema.Check{ID: "bad-rung-check", Rung: "9"})
}

func TestLadderDerivation(t *testing.T) {
	checks := []*schema.Check{
		{ID: "check-0-1", Rung: "0"},
		{ID: "check-0-2", Rung: "0"},
		{ID: "check-1-1", Rung: "1"},
		{ID: "check-4-1", Rung: "4"},
	}

	for _, check := range checks {
		Register(check)
	}

	ladder := GetLadder()
	if len(ladder) == 0 {
		t.Fatal("ladder is empty")
	}

	if len(ladder) < 3 {
		t.Errorf("expected at least 3 rungs, got %d", len(ladder))
	}

	for i := 0; i < len(ladder)-1; i++ {
		if ladder[i].Order > ladder[i+1].Order {
			t.Errorf("rungs not ordered correctly: %d > %d", ladder[i].Order, ladder[i+1].Order)
		}
	}
}

func TestAllChecks(t *testing.T) {
	all := AllChecks()
	if len(all) == 0 {
		t.Fatalf("expected at least some registered checks")
	}

	for _, c := range all {
		if c.ID == "" {
			t.Error("check has empty ID")
		}
		if c.Rung == "" {
			t.Error("check has empty rung")
		}
	}
}
