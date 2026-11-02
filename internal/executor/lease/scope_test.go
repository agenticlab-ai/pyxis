package lease

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func TestClaimsSeparateIdenticalNumbersAcrossTaskHosts(t *testing.T) {
	remote := newRemote(t)
	l := newLease(t, remote, "holder")
	one := port.Task{ID: "T1", Number: 1, SourceKey: "forgejo:http://localhost:3000:o/r"}
	two := one
	two.SourceKey = "forgejo:http://localhost:3001:o/r"
	first, err := l.Acquire(context.Background(), one)
	if err != nil {
		t.Fatal(err)
	}
	second, err := l.Acquire(context.Background(), two)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.Acquire(context.Background(), one); !errors.Is(err, port.ErrClaimLost) {
		t.Fatalf("same-source duplicate = %v", err)
	}
	refs := claimRefs(t, remote)
	if len(refs) != 2 {
		t.Fatalf("claims collided: %v", refs)
	}
	if err := first.Release(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := second.Release(context.Background()); err != nil {
		t.Fatal(err)
	}
	if refs := claimRefs(t, remote); len(refs) != 0 {
		t.Fatalf("claims leaked: %v", refs)
	}
}

func TestScopedClaimFailureReleasesReservation(t *testing.T) {
	source := "forgejo:http://localhost:3000:o/r"
	l, err := New(Config{WorkDir: t.TempDir(), Git: "/bin/false", LegacySourceKey: source})
	if err != nil || !strings.HasPrefix(l.Holder(), "pyxis-local/") {
		t.Fatalf("default holder=%v %v", l, err)
	}
	task := port.Task{ID: "T1", Number: 1, SourceKey: source}
	for range 2 {
		if _, err := l.Acquire(t.Context(), task); err == nil || errors.Is(err, port.ErrClaimLost) {
			t.Fatalf("failed mint wedged claim reservation: %v", err)
		}
	}
	if err := l.releaseRemote(t.Context(), "refs/claim/"+task.ScopeToken()+"/1", "sha"); err == nil {
		t.Fatal("failed release reported success")
	}
}
