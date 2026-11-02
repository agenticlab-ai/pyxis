package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("closed output") }

func TestRun(t *testing.T) {
	root := filepath.Join("..", "..")
	for _, tt := range []struct {
		root string
		args []string
		want int
	}{
		{root, nil, 0},
		{t.TempDir(), nil, 1},
		{root, []string{"--help"}, 0},
		{root, []string{"-h"}, 0},
		{root, []string{"unexpected"}, 1},
	} {
		var output bytes.Buffer
		if got := run(t.Context(), tt.root, tt.args, &output); got != tt.want || output.Len() == 0 {
			t.Fatalf("run(%s, %v) = %d: %s", tt.root, tt.args, got, output.String())
		}
	}
	if run(t.Context(), root, nil, brokenWriter{}) != 1 {
		t.Fatal("output failure passed")
	}
	if run(t.Context(), root, []string{"--help"}, brokenWriter{}) != 1 {
		t.Fatal("help output failure passed")
	}
}

func TestMainHelp(t *testing.T) {
	old := os.Args
	os.Args = []string{"ci-parity", "--help"}
	t.Cleanup(func() { os.Args = old })
	main()
}
