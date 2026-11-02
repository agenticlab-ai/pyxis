package issueentry

import (
	"errors"
	"strings"
	"sync"
	"testing"
)

func TestRemoteRepairAuthorityReservesOnceAcrossConcurrentStoresAndRestart(t *testing.T) {
	left, right := stores(t)
	input, snapshot := intake(t)
	contract, _, err := Prepare(input, "github", "github:https://github.com:o/r", 17, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := left.AdmitWithRemoteRepairs(t.Context(), contract, 1); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, store := range []*Store{left, right} {
		wg.Go(func() {
			reservation, err := store.ReserveRemoteRepair(t.Context(), contract)
			if err == nil && (reservation.Attempt != 1 || reservation.Cap != 1) {
				err = errors.New("reservation lost its durable ordinal or cap")
			}
			results <- err
		})
	}
	wg.Wait()
	close(results)
	succeeded, exhausted := 0, 0
	for err := range results {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, ErrRemoteRepairExhausted):
			exhausted++
		default:
			t.Fatal(err)
		}
	}
	if succeeded != 1 || exhausted != 1 {
		t.Fatalf("concurrent reservations: succeeded=%d exhausted=%d", succeeded, exhausted)
	}
	if _, err := right.ReserveRemoteRepair(t.Context(), contract); !errors.Is(err, ErrRemoteRepairExhausted) {
		t.Fatalf("restart regained reserved repair: %v", err)
	}
}

func TestRemoteRepairAuthorityFailsClosedWhenLegacyEvidenceIsMissing(t *testing.T) {
	store, _ := stores(t)
	input, snapshot := intake(t)
	contract, _, err := Prepare(input, "github", "github:https://github.com:o/r", 17, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Admit(t.Context(), contract); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReserveRemoteRepair(t.Context(), contract); err == nil || !strings.Contains(err.Error(), "no retained") {
		t.Fatalf("missing legacy authority was repaired or inferred: %v", err)
	}
}

func TestZeroRemoteRepairAllowanceStaysExhausted(t *testing.T) {
	store, _ := stores(t)
	input, snapshot := intake(t)
	contract, _, err := Prepare(input, "github", "github:https://github.com:o/r", 17, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AdmitWithRemoteRepairs(t.Context(), contract, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReserveRemoteRepair(t.Context(), contract); !errors.Is(err, ErrRemoteRepairExhausted) {
		t.Fatalf("zero allowance acquired a default repair budget: %v", err)
	}
}
