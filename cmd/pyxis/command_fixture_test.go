package main

import (
	"bytes"
	"context"
	"io"
	"testing"
)

// A second file unit registers itself exactly as a production command does.
func init() {
	mustRegisterRootCommand(rootCommand{
		key: "conformance-fixture", usage: "  conformance-fixture test-only additive command",
		handler: func(ctx context.Context, _ []string, stdout, _ io.Writer) int {
			terminalPrintf(stdout, "fixture context: %v\n", ctx.Err())
			return 23
		},
	})
}

func TestRootCommandAdditiveFileDispatchHelpAndContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var stdout, stderr bytes.Buffer
	if code := runContext(ctx, []string{"conformance-fixture"}, &stdout, &stderr); code != 23 || stdout.String() != "fixture context: context canceled\n" {
		t.Fatalf("additive dispatch = %d, stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if code := run([]string{"help"}, &stdout, &stderr); code != 2 || bytes.Count(stderr.Bytes(), []byte("conformance-fixture test-only additive command")) != 1 {
		t.Fatalf("additive help = %d, stderr=%q", code, stderr.String())
	}
}
