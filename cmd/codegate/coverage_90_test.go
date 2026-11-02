package main

import (
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/verify/invoke"
)

func TestCoverage90MainDispatch(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want int
	}{
		{name: "usage", want: invoke.ExitConfigErr},
		{name: "help", args: []string{"help"}, want: invoke.ExitPass},
		{name: "version", args: []string{"version"}, want: invoke.ExitPass},
		{name: "unknown", args: []string{"unknown"}, want: invoke.ExitConfigErr},
		{name: "capture dispatch", args: []string{"capture-baseline"}, want: invoke.ExitConfigErr},
		{name: "seed dispatch", args: []string{"seed", "--unknown"}, want: invoke.ExitConfigErr},
		{name: "run dispatch", args: []string{"run"}, want: invoke.ExitConfigErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := runMain(tc.args); got != tc.want {
				t.Fatalf("runMain(%v) = %d, want %d", tc.args, got, tc.want)
			}
		})
	}
}
