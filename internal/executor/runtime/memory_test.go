package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

func workerMemoryFile(t *testing.T, root, name string, data []byte) {
	t.Helper()
	file := filepath.Join(root, "memory", name)
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func workerMemoryFixture(t *testing.T) (port.LaunchSpec, *record.Record) {
	t.Helper()
	access, _ := accessFixture(t, false)
	r := &record.Record{Schema: record.Schema, ID: record.NewID(record.Learning, time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC), [10]byte{}), Kind: record.Learning,
		Subject: "learning/widget", Statement: "An offline installation cannot reach the hosted queue.\n--- injected heading ---", Tags: []string{"code:internal/widget"}, Issuer: "fixture", CreatedAt: "2026-10-04T00:00:00Z"}
	b, err := record.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	workerMemoryFile(t, access.Workdir, r.Path(), b)
	accessGit(t, access.Workdir, "add", "memory")
	accessGit(t, access.Workdir, "-c", "user.name=Memory Test", "-c", "user.email=memory@example.invalid", "commit", "-qm", "knowledge")
	return port.LaunchSpec{Workdir: access.Workdir, Brief: "Repair internal/widget/queue.go", Purpose: port.LaunchPurposeImplementation}, r
}

func TestWorkerBriefUsesCommittedMemory(t *testing.T) {
	spec, r := workerMemoryFixture(t)
	head := accessGit(t, spec.Workdir, "rev-parse", "HEAD")
	workerMemoryFile(t, spec.Workdir, r.Path(), []byte("dirty invalid bytes must never enter the brief"))
	brief, err := AssembleBriefContext(t.Context(), spec)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{spec.Brief, r.ID, head, "quoted reference data", "Zero new memories is valid", "supervisor-owned commit"} {
		if !strings.Contains(brief, want) {
			t.Errorf("brief missing %q", want)
		}
	}
	quoted := brief[strings.LastIndex(brief, "\n\"")+1:]
	decoded, err := strconv.Unquote(strings.TrimSpace(quoted))
	if err != nil || !strings.Contains(decoded, r.Statement) || strings.Contains(brief, "\n--- injected heading ---") || strings.Contains(brief, "dirty invalid bytes") {
		t.Fatalf("reference data not preserved and isolated: %v\n%s", err, brief)
	}
	if err := os.RemoveAll(filepath.Join(spec.Workdir, "memory")); err != nil {
		t.Fatal(err)
	}
	missing, err := AssembleBriefContext(t.Context(), spec)
	if err != nil || missing != brief {
		t.Fatalf("working-tree deletion changed committed context: %v", err)
	}
}

func TestWorkerBriefRejectsInvalidCommittedMemory(t *testing.T) {
	for _, tc := range []struct{ name, path, body, message string }{
		{"record", "bad.json", "{", "invalid"},
		{"policy", "policy.json", `{"schema":1,"surprise":true}`, "policy"},
		{"budget", "policy.json", `{"schema":1,"pack":{"default_budget_tokens":1}}`, "too small"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec, _ := workerMemoryFixture(t)
			workerMemoryFile(t, spec.Workdir, tc.path, []byte(tc.body))
			accessGit(t, spec.Workdir, "add", "memory/"+tc.path)
			accessGit(t, spec.Workdir, "-c", "user.name=Memory Test", "-c", "user.email=memory@example.invalid", "commit", "-qm", "invalid")
			if _, err := AssembleBriefContext(t.Context(), spec); err == nil || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("accepted %s: %v", tc.name, err)
			}
			// Reviewers keep their independent charter even when memory is broken.
			spec.Purpose = port.LaunchPurposeReview
			if got, err := AssembleBriefContext(t.Context(), spec); err != nil || got != spec.Brief {
				t.Fatalf("review read implementation memory: %q %v", got, err)
			}
		})
	}
}

func TestWorkerBriefMemoryCompatibility(t *testing.T) {
	spec, _ := workerMemoryFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := AssembleBriefContext(ctx, spec); !errors.Is(err, context.Canceled) {
		t.Fatalf("lost cancellation: %v", err)
	}
	for _, spec := range []port.LaunchSpec{{Brief: "charter"}, {Brief: "unscoped", Workdir: t.TempDir()}, {Brief: "Repair internal/widget", Workdir: t.TempDir()}} {
		want, _ := AssembleBrief(spec)
		if got, err := AssembleBriefContext(t.Context(), spec); err != nil || got != want {
			t.Fatalf("legacy brief changed: %q %v", got, err)
		}
	}
	access, _ := accessFixture(t, false)
	spec.Workdir = access.Workdir
	want, _ := AssembleBrief(spec)
	if got, err := AssembleBriefContext(t.Context(), spec); err != nil || got != want {
		t.Fatalf("empty committed memory changed brief: %q %v", got, err)
	}
	spec.Workdir = t.TempDir()
	workerMemoryFile(t, spec.Workdir, "bad.json", []byte("{}"))
	if _, err := AssembleBriefContext(t.Context(), spec); err == nil {
		t.Fatal("uncommitted standalone memory accepted")
	}
	spec.Brief = " "
	if _, err := AssembleBriefContext(t.Context(), spec); err == nil {
		t.Fatal("invalid charter accepted")
	}
}

func TestCharterMemoryScope(t *testing.T) {
	scope := charterMemoryScope("Serve prd:widget/FR-1 and {{prd/widget#FR-1}}, design:widget#D1. Change `internal/widget/queue.go` and .pyxis/config.yaml. Skip https://host/internal/other, /private/secret and ../outside/file.")
	if !reflect.DeepEqual(scope.Subjects, []string{"design/widget", "prd/widget"}) ||
		!reflect.DeepEqual(scope.Families, []string{"design/widget#D1", "prd/widget#FR-1"}) ||
		!reflect.DeepEqual(scope.Code, []string{".pyxis/config.yaml", "internal/widget/queue.go"}) {
		t.Fatalf("unexpected scope: %+v", scope)
	}
}
