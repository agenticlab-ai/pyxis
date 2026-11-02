package issueentry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestRemoteRepairAdmissionRejectsInvalidAndLegacyAuthority(t *testing.T) {
	store, _ := stores(t)
	input, snapshot := intake(t)
	c, _, err := Prepare(input, "github", "github:https://github.com:o/r", 17, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	invalid := c
	invalid.ID = "invalid"
	if err := store.AdmitWithRemoteRepairs(t.Context(), invalid, 1); err == nil {
		t.Fatal("invalid contract admitted")
	}
	if _, err := store.ReserveRemoteRepair(t.Context(), invalid); err == nil {
		t.Fatal("invalid contract reserved authority")
	}
	if err := store.AdmitWithRemoteRepairs(t.Context(), c, -1); err == nil {
		t.Fatal("negative allowance admitted")
	}
	if err := store.Admit(t.Context(), c); err != nil {
		t.Fatal(err)
	}
	if err := store.AdmitWithRemoteRepairs(t.Context(), c, 1); err == nil || !strings.Contains(err.Error(), "no remote CI repair authority") {
		t.Fatalf("legacy admission acquired authority: %v", err)
	}
	if _, err := store.ReserveRemoteRepair(t.Context(), c); err == nil {
		t.Fatal("legacy contract received repair authority")
	}
}

func TestRemoteRepairAdmissionReplayPreservesSpentAuthority(t *testing.T) {
	store, fresh := stores(t)
	input, snapshot := intake(t)
	c, _, err := Prepare(input, "github", "github:https://github.com:o/r", 17, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AdmitWithRemoteRepairs(t.Context(), c, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReserveRemoteRepair(t.Context(), c); err != nil {
		t.Fatal(err)
	}
	if err := fresh.AdmitWithRemoteRepairs(t.Context(), c, 2); err != nil {
		t.Fatal(err)
	}
	if err := fresh.AdmitWithRemoteRepairs(t.Context(), c, 3); err == nil {
		t.Fatal("admission replay raised the frozen allowance")
	}
	changed := input
	changed.Goal = "A different recovery target"
	other, _, err := Prepare(changed, "github", "github:https://github.com:o/r", 17, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := fresh.AdmitWithRemoteRepairs(t.Context(), other, 2); err == nil {
		t.Fatal("another contract replaced repair authority")
	}
	reserved, err := fresh.ReserveRemoteRepair(t.Context(), c)
	if err != nil || reserved.Attempt != 2 || reserved.Cap != 2 {
		t.Fatalf("replay reset or changed allowance: %+v %v", reserved, err)
	}
	if _, err := store.ReserveRemoteRepair(t.Context(), c); !errors.Is(err, ErrRemoteRepairExhausted) {
		t.Fatalf("replay restored spent authority: %v", err)
	}
}

func TestRemoteRepairAuthorityRejectsMalformedStateWithoutRewritingIt(t *testing.T) {
	store, fresh := stores(t)
	input, snapshot := intake(t)
	c, _, err := Prepare(input, "github", "github:https://github.com:o/r", 17, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AdmitWithRemoteRepairs(t.Context(), c, 2); err != nil {
		t.Fatal(err)
	}
	path, err := repairPath(c)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"invalid JSON", "unknown field", "future schema", "foreign contract", "negative cap", "negative reserved", "over-reserved"} {
		t.Run(name, func(t *testing.T) {
			authority := RemoteRepairAuthority{Schema: remoteRepairSchema, ContractID: c.ID, Cap: 2}
			switch name {
			case "future schema":
				authority.Schema = "future/v9"
			case "foreign contract":
				authority.ContractID = "sha256:" + strings.Repeat("f", 64)
			case "negative cap":
				authority.Cap = -1
			case "negative reserved":
				authority.Reserved = -1
			case "over-reserved":
				authority.Reserved = 3
			}
			wire, err := json.Marshal(authority)
			if err != nil {
				t.Fatal(err)
			}
			switch name {
			case "invalid JSON":
				wire = []byte("{")
			case "unknown field":
				wire = append(bytes.TrimSuffix(wire, []byte("}")), []byte(`,"extra":true}`)...)
			}
			// Corrupt only this local Git fixture to exercise closed evidence parsing.
			if _, err := store.git.Update(t.Context(), path, "test: malformed repair evidence", func([]byte) ([]byte, bool, error) { return wire, true, nil }); err != nil {
				t.Fatal(err)
			}
			if _, err := fresh.ReserveRemoteRepair(t.Context(), c); err == nil || errors.Is(err, ErrRemoteRepairExhausted) {
				t.Fatalf("malformed evidence granted authority or became legitimate exhaustion: %v", err)
			}
			if err := fresh.AdmitWithRemoteRepairs(t.Context(), c, 2); err == nil {
				t.Fatal("admission replay repaired ambiguous authority")
			}
			after, err := store.git.Read(t.Context(), path)
			if err != nil || !bytes.Equal(after, wire) {
				t.Fatalf("refusal rewrote evidence: %s %v", after, err)
			}
		})
	}
}

func TestRemoteRepairAuthorityCancellationConsumesNoAttempt(t *testing.T) {
	store, fresh := stores(t)
	input, snapshot := intake(t)
	c, _, err := Prepare(input, "github", "github:https://github.com:o/r", 17, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := store.AdmitWithRemoteRepairs(ctx, c, 2); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled admission: %v", err)
	}
	if err := store.AdmitWithRemoteRepairs(t.Context(), c, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReserveRemoteRepair(ctx, c); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled reservation: %v", err)
	}
	reserved, err := fresh.ReserveRemoteRepair(t.Context(), c)
	if err != nil || reserved.Attempt != 1 || reserved.Cap != 2 {
		t.Fatalf("cancellation consumed authority: %+v %v", reserved, err)
	}
}
