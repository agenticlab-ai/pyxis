package persist

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

func TestRunConformanceFailureDiagnostics(t *testing.T) {
	switch os.Getenv("PYXIS_CONFORMANCE_FAILURE_CHILD") {
	case "missing-factory":
		RunConformance(t, Conformance{})
		return
	case "adapter-failure":
		wantErr := errors.New("adapter unavailable")
		RunConformance(t, Conformance{
			NewPort: func(*testing.T) LegacyPort {
				return scriptedLegacyPort{
					persist: func(context.Context, *report.Report) (Ref, error) { return Ref{}, wantErr },
					read:    func(context.Context, Ref) (*report.Report, error) { return nil, wantErr },
				}
			},
			ArtifactCount: func(LegacyPort) int { return 0 },
		})
		return
	}

	for _, mode := range []string{"missing-factory", "adapter-failure"} {
		args := []string{"-test.run=^TestRunConformanceFailureDiagnostics$"}
		for _, arg := range os.Args[1:] {
			if strings.HasPrefix(arg, "-test.gocoverdir=") {
				args = append(args, arg)
			}
		}
		cmd := exec.Command(os.Args[0], args...)
		cmd.Env = append(os.Environ(), "PYXIS_CONFORMANCE_FAILURE_CHILD="+mode)
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("conformance failure mode %q unexpectedly passed", mode)
		}
		if !strings.Contains(string(out), "conformance") && !strings.Contains(string(out), "adapter unavailable") {
			t.Fatalf("conformance failure mode %q omitted its diagnostic:\n%s", mode, out)
		}
	}
}

type scriptedLegacyPort struct {
	persist func(context.Context, *report.Report) (Ref, error)
	read    func(context.Context, Ref) (*report.Report, error)
}

func (p scriptedLegacyPort) Persist(ctx context.Context, r *report.Report) (Ref, error) {
	return p.persist(ctx, r)
}

func (p scriptedLegacyPort) Read(ctx context.Context, ref Ref) (*report.Report, error) {
	return p.read(ctx, ref)
}

type memoryPort struct {
	artifacts map[string][]byte
}

func (p *memoryPort) Persist(_ context.Context, r *report.Report) (Ref, error) {
	artifact, err := ComposeArtifact(r)
	if err != nil {
		return Ref{}, err
	}
	if p.artifacts == nil {
		p.artifacts = make(map[string][]byte)
	}
	locator := fmt.Sprintf("artifact-%d", len(p.artifacts)+1)
	p.artifacts[locator] = append([]byte(nil), artifact...)
	return Ref{Medium: "memory", Locator: locator}, nil
}

func (p *memoryPort) Read(_ context.Context, ref Ref) (*report.Report, error) {
	artifact, ok := p.artifacts[ref.Locator]
	if !ok {
		return nil, fmt.Errorf("artifact %q is absent", ref.Locator)
	}
	return ParseArtifact(string(artifact))
}

func TestRunConformanceExercisesTheSharedPersistenceContract(t *testing.T) {
	RunConformance(t, Conformance{
		NewPort: func(*testing.T) LegacyPort { return &memoryPort{} },
		ArtifactCount: func(p LegacyPort) int {
			return len(p.(*memoryPort).artifacts)
		},
	})
}

func TestConformanceValidationNamesAdapterFailures(t *testing.T) {
	wantErr := errors.New("adapter failed")
	persistFailure := scriptedLegacyPort{
		persist: func(context.Context, *report.Report) (Ref, error) { return Ref{}, wantErr },
		read:    func(context.Context, Ref) (*report.Report, error) { return nil, nil },
	}
	if err := validateRoundTrip(persistFailure, goldenReport()); !errors.Is(err, wantErr) {
		t.Fatalf("round-trip persist error = %v", err)
	}
	if err := validateRenderRoundTrip(persistFailure, goldenReport()); !errors.Is(err, wantErr) {
		t.Fatalf("render persist error = %v", err)
	}

	readFailure := scriptedLegacyPort{
		persist: func(context.Context, *report.Report) (Ref, error) { return Ref{Locator: "one"}, nil },
		read:    func(context.Context, Ref) (*report.Report, error) { return nil, wantErr },
	}
	if err := validateRoundTrip(readFailure, goldenReport()); !errors.Is(err, wantErr) {
		t.Fatalf("round-trip read error = %v", err)
	}
	if err := validateRenderRoundTrip(readFailure, goldenReport()); !errors.Is(err, wantErr) {
		t.Fatalf("render read error = %v", err)
	}

	mismatch := goldenReport()
	mismatch.ReviewID = "different-review"
	corrupt := scriptedLegacyPort{
		persist: func(context.Context, *report.Report) (Ref, error) { return Ref{Locator: "one"}, nil },
		read:    func(context.Context, Ref) (*report.Report, error) { return mismatch, nil },
	}
	if err := validateRoundTrip(corrupt, goldenReport()); err == nil {
		t.Fatal("validateRoundTrip accepted a changed payload")
	}
	valid := scriptedLegacyPort{
		persist: func(context.Context, *report.Report) (Ref, error) { return Ref{Locator: "one"}, nil },
		read:    func(context.Context, Ref) (*report.Report, error) { return goldenReport(), nil },
	}
	if err := validateRenderRoundTripWithRenderer(valid, goldenReport(), func(*report.Report) string { return "stale human face" }); err == nil {
		t.Fatal("validateRenderRoundTrip accepted a stale rendered face")
	}
	if err := validateRoundTrip(valid, &report.Report{}); err == nil {
		t.Fatal("validateRoundTrip accepted an invalid input report")
	}
	invalidRecovered := scriptedLegacyPort{
		persist: func(context.Context, *report.Report) (Ref, error) { return Ref{Locator: "one"}, nil },
		read:    func(context.Context, Ref) (*report.Report, error) { return &report.Report{}, nil },
	}
	if err := validateRoundTrip(invalidRecovered, goldenReport()); err == nil {
		t.Fatal("validateRoundTrip accepted an invalid recovered report")
	}
}

func TestConformanceOptionalCountAndZeroFormatter(t *testing.T) {
	RunConformance(t, Conformance{NewPort: func(*testing.T) LegacyPort { return &memoryPort{} }})
	if got := itoa(0); got != "0" {
		t.Fatalf("itoa(0) = %q", got)
	}
}

func TestPersistenceDiagnosticsExposeTypedCauses(t *testing.T) {
	cause := fmt.Errorf("host unavailable")
	err := &AbsenceRecordedError{
		Proposal:        "github:owner/repo#7",
		Attempts:        3,
		AlternateSignal: Ref{Medium: "github-check", Locator: "check-9"},
		Recorded:        true,
		Cause:           cause,
	}
	if got := err.Error(); got == "" {
		t.Fatal("recorded absence has an empty diagnostic")
	}
	if !err.Is(ErrRecordedAbsence) || err.Is(cause) || err.Unwrap() != cause {
		t.Fatalf("typed error contract was lost: %v", err)
	}
	if got := (Ref{Medium: "github", Locator: "42"}).String(); got != "github:42" {
		t.Fatalf("Ref.String() = %q", got)
	}

	err.Recorded = false
	if got := err.Error(); got == "" {
		t.Fatal("unrecorded absence has an empty diagnostic")
	}
}
