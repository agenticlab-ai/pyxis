package runtime

import "context"

type verificationBaseKey struct{}

// WithVerificationBase pins a fetched and integrated mainline revision for one
// gate evaluation. It is scoped to the context, never a process-global env
// mutation shared by concurrent tasks.
func WithVerificationBase(ctx context.Context, revision string) context.Context {
	return context.WithValue(ctx, verificationBaseKey{}, revision)
}

func VerificationBase(ctx context.Context) string {
	base, _ := ctx.Value(verificationBaseKey{}).(string)
	return base
}
