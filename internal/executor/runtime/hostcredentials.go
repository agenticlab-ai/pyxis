package runtime

import "context"

type hostCredentialKey struct{}

// WithHostCredentialNames records secret names from the supervisor's trusted
// configuration. A child may edit its own verification configuration, so the
// deny set must be captured before launch rather than read from that tree.
// Contexts retain names only; Git and API operations keep their host credentials.
func WithHostCredentialNames(ctx context.Context, names ...string) context.Context {
	keys := map[string]bool{}
	if inherited, ok := ctx.Value(hostCredentialKey{}).(map[string]bool); ok {
		for name := range inherited {
			keys[name] = true
		}
	}
	for _, name := range names {
		if name != "" {
			keys[name] = true
		}
	}
	return context.WithValue(ctx, hostCredentialKey{}, keys)
}

func isHostCredential(ctx context.Context, name string) bool {
	keys, _ := ctx.Value(hostCredentialKey{}).(map[string]bool)
	return keys[name]
}
