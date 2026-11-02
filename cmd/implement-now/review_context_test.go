package main

import (
	"bytes"
	"testing"

	reviewruntime "github.com/agenticlab-ai/pyxis/internal/implreview/runtime"
)

func TestInternalReviewContextDoesNotInitializeExecutor(t *testing.T) {
	var out, diagnostic bytes.Buffer
	code := run([]string{"--internal-review-context", "--reviewer-root", "native-root"}, &out, &diagnostic)
	want, err := reviewruntime.ContextID("native-root")
	if code != 0 || err != nil || out.String() != want+"\n" {
		t.Fatalf("code=%d output=%q diagnostic=%s err=%v", code, &out, &diagnostic, err)
	}
}
