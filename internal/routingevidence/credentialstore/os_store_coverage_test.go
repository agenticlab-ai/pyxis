package credentialstore

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestExecRunnerHelper(t *testing.T) {
	if os.Getenv("PYXIS_CREDENTIAL_HELPER") == "" {
		return
	}
	if os.Args[len(os.Args)-1] == "fail" {
		fmt.Fprint(os.Stderr, "native failure")
		os.Exit(4)
	}
	fmt.Fprint(os.Stdout, "native output")
	os.Exit(0)
}

func TestExecRunnerSuccessAndError(t *testing.T) {
	t.Setenv("PYXIS_CREDENTIAL_HELPER", "1")
	runner := execRunner{}
	out, err := runner.Run(context.Background(), nil, os.Args[0], "-test.run=^TestExecRunnerHelper$", "--", "success")
	if err != nil || string(out) != "native output" {
		t.Fatalf("success = %q, %v", out, err)
	}
	if _, err := runner.Run(context.Background(), nil, os.Args[0], "-test.run=^TestExecRunnerHelper$", "--", "fail"); err == nil || !strings.Contains(err.Error(), "native failure") {
		t.Fatalf("failure = %v", err)
	}
}

func TestOSStoreConstructorAndMalformedSecrets(t *testing.T) {
	if got := NewOSStore(); got == nil || got.runner == nil || got.goos == "" {
		t.Fatalf("NewOSStore = %+v", got)
	}
	for _, value := range []string{"", "a", "0g"} {
		if _, err := decodeHexSecret(value); err == nil {
			t.Fatalf("decodeHexSecret(%q) succeeded", value)
		}
	}
	if got, ok := hexNibble('9'); !ok || got != 9 {
		t.Fatalf("hexNibble digit = %d, %v", got, ok)
	}
	if got, ok := hexNibble('f'); !ok || got != 15 {
		t.Fatalf("hexNibble lower = %d, %v", got, ok)
	}
	if _, ok := hexNibble('F'); ok {
		t.Fatal("hexNibble accepted uppercase input")
	}
}
