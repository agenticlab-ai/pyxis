package seed

import (
	"errors"
	"os"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/verify/ecosystem"
	"github.com/agenticlab-ai/pyxis/internal/verify/invoke"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

func TestCoverage90SeedSerializationAndExclusiveCreateBoundaries(t *testing.T) {
	t.Run("template refusal", func(t *testing.T) {
		prior := seedMarshalTemplate
		t.Cleanup(func() { seedMarshalTemplate = prior })
		seedMarshalTemplate = func(*schema.Manifest) ([]byte, *Refusal) {
			return nil, &Refusal{Message: "injected template failure", Code: invoke.ExitRungErr}
		}
		if result, refusal := Seed(workspaceWith(t, "go.mod"), ecosystem.New()); result != nil || refusal == nil {
			t.Fatalf("Seed template failure = (%+v, %+v)", result, refusal)
		}
	})

	for _, tc := range []struct {
		name string
		err  error
		code int
	}{
		{name: "manifest appeared", err: os.ErrExist, code: invoke.ExitConfigErr},
		{name: "write failed", err: errors.New("disk unavailable"), code: invoke.ExitRungErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prior := seedWriteExclusive
			t.Cleanup(func() { seedWriteExclusive = prior })
			seedWriteExclusive = func(string, []byte) error { return tc.err }
			result, refusal := Seed(workspaceWith(t, "go.mod"), ecosystem.New())
			if result != nil || refusal == nil || refusal.Code != tc.code {
				t.Fatalf("Seed exclusive-create failure = (%+v, %+v), want code %d", result, refusal, tc.code)
			}
		})
	}
}
