package conformancetest

import (
	"context"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
)

// RunOwnerIdentity exercises the consumer's registered-caller and cancellation
// contract. Credential format and medium-specific refusals belong to adapters.
func RunOwnerIdentity(t *testing.T, identity featuredrive.OwnerIdentity, assignment featuredrive.ExecutionAssignment, expected featuredrive.Owner) {
	t.Helper()
	expect(t, expected.ID != "" && (expected.Domain == "local" || expected.Domain == "remote"), "conformance requires a registered owner")
	got := identity.Authenticate(t.Context(), assignment)
	expect(t, got == expected, "registered caller: got %+v, want %+v", got, expected)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	expect(t, identity.Authenticate(ctx, assignment) == (featuredrive.Owner{}), "cancelled caller retained execution identity")
}
