package main

import (
	"os"
	"testing"
)

func TestMainDelegatesExitCodeToRun(t *testing.T) {
	oldArgs, oldExit := os.Args, mainExit
	defer func() {
		os.Args = oldArgs
		mainExit = oldExit
	}()
	os.Args = []string{"implement-now", "--definitely-unknown"}
	got := -1
	mainExit = func(code int) { got = code }
	main()
	if got != 2 {
		t.Fatalf("main exit = %d, want usage exit 2", got)
	}
}
