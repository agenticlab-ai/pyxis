package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"testing"
)

func TestRootCommandRegistryDiscoversDispatchAndSortedHelp(t *testing.T) {
	registry := newRootCommandRegistry()
	called := false
	fixture := func(context.Context, []string, io.Writer, io.Writer) int { called = true; return 23 }
	if err := registry.register(rootCommand{key: "z-fixture", usage: "z usage", handler: fixture}); err != nil {
		t.Fatal(err)
	}
	if err := registry.register(rootCommand{key: "a-fixture", usage: "a usage", handler: fixture}); err != nil {
		t.Fatal(err)
	}
	if code, ok := registry.dispatch(t.Context(), "z-fixture", nil, io.Discard, io.Discard); !ok || code != 23 || !called {
		t.Fatalf("dispatch = %d, %t, called=%t", code, ok, called)
	}
	if got, want := registry.usages(), []string{"a usage", "z usage"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("usages = %#v, want %#v", got, want)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"maintenance", "status"}, &stdout, &stderr); code != 2 || !bytes.Contains(stderr.Bytes(), []byte("--repo is required")) {
		t.Fatalf("registered production dispatch = %d, stderr=%q", code, stderr.String())
	}
}

func TestRootCommandRegistryRejectsInvalidDuplicateAndLegacy(t *testing.T) {
	registry := newRootCommandRegistry()
	handler := func(context.Context, []string, io.Writer, io.Writer) int { return 0 }
	if err := registry.register(rootCommand{}); !errors.Is(err, errInvalidRootCommand) {
		t.Fatalf("empty error = %v", err)
	}
	if err := registry.register(rootCommand{key: "bad key", usage: "usage", handler: handler}); !errors.Is(err, errInvalidRootCommand) {
		t.Fatalf("invalid key error = %v", err)
	}
	if err := registry.register(rootCommand{key: "doctor", usage: "usage", handler: handler}); !errors.Is(err, errLegacyRootCommand) {
		t.Fatalf("legacy error = %v", err)
	}
	command := rootCommand{key: "fixture", usage: "usage", handler: handler}
	if err := registry.register(command); err != nil {
		t.Fatal(err)
	}
	if err := registry.register(command); !errors.Is(err, errDuplicateRootCommand) {
		t.Fatalf("duplicate error = %v", err)
	}
}

func TestUsageIncludesRegisteredCommandOnce(t *testing.T) {
	var output bytes.Buffer
	if code := usage(&output); code != 2 {
		t.Fatalf("code = %d", code)
	}
	if got := bytes.Count(output.Bytes(), []byte("maintenance install|run|status|uninstall")); got != 1 {
		t.Fatalf("maintenance help count = %d\n%s", got, output.String())
	}
}
