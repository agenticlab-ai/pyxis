package implreview

import (
	"context"
	"testing"
)

func TestNativeModelBinding(t *testing.T) {
	if model, bound := NativeModel(context.Background()); bound || model != "" {
		t.Fatalf("unbound model = %q, %t", model, bound)
	}
	for _, model := range []string{"gpt-5.6-sol", ""} {
		ctx := WithNativeModel(context.Background(), model)
		ctx, cancel := context.WithCancel(ctx)
		cancel()
		if got, bound := NativeModel(context.WithoutCancel(ctx)); !bound || got != model {
			t.Fatalf("native model binding = %q, %t, want %q", got, bound, model)
		}
	}
}
