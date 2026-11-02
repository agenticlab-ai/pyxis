package taskspectest

import (
	"fmt"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/taskspec"
	"github.com/agenticlab-ai/pyxis/internal/taskrequirements/requirementstest"
)

func TestFixtureCarriesIndependentValidatedRequirements(t *testing.T) {
	s, err := Spec("T1")
	if err != nil {
		t.Fatal(err)
	}
	if s.Requirements.Implementation.Profile.Capability == s.Requirements.Review.Profile.Capability {
		t.Fatal("fixture lost independent role profiles")
	}
	got, err := taskspec.Parse([]string{Marker(t, "T1")})
	if err != nil || got.PlanTaskID != s.PlanTaskID {
		t.Fatalf("fixture round trip: %+v, %v", got, err)
	}
}

func TestMalformedFixtureCannotBecomeTaskCarrier(t *testing.T) {
	for _, tc := range []struct{ name, input string }{
		{"malformed JSON", "{"},
		{"unresolved citation", strings.ReplaceAll(requirementstest.Input, "task:acceptance", "prd:missing/FR-1")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := spec("T1", tc.input); err == nil {
				t.Fatal("accepted malformed or unsupported authored requirements")
			}
			defer func() {
				if _, ok := recover().(fixtureFailure); !ok {
					t.Fatal("marker did not report the invalid fixture through Fatal")
				}
			}()
			marker(failingFixture{TB: t}, "T1", tc.input)
		})
	}
}

type fixtureFailure string
type failingFixture struct{ testing.TB }

func (f failingFixture) Fatal(args ...any) { panic(fixtureFailure(fmt.Sprint(args...))) }
