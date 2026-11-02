package implreview

import "context"

type nativeModelKey struct{}

// WithNativeModel binds the host-resolved launch identity for the coordinator's
// current-round provenance check. The value never comes from reviewer output
// and does not attest to execution of individual lens children.
func WithNativeModel(ctx context.Context, model string) context.Context {
	return context.WithValue(ctx, nativeModelKey{}, model)
}

// NativeModel returns the host's launch expectation when one was bound.
func NativeModel(ctx context.Context) (string, bool) {
	model, bound := ctx.Value(nativeModelKey{}).(string)
	return model, bound
}
