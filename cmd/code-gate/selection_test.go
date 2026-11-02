package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

type unavailableOutput struct{}

func (unavailableOutput) Write([]byte) (int, error) { return 0, errors.New("closed output") }

func TestOutputFailureCannotReportGreen(t *testing.T) {
	root := writeConfig(t, "code-gate:\n  green:\n    command: true\n")
	if code := listGate(root, unavailableOutput{}, io.Discard); code != exitRed {
		t.Fatalf("list to closed output = %d", code)
	}
	if code := runSelectedGate(root, true, "green", "", "", false, 0, unavailableOutput{}); code != exitRed {
		t.Fatalf("green report to closed output = %d", code)
	}
	if code := dispatch([]string{"help"}, unavailableOutput{}, io.Discard); code != exitRed {
		t.Fatalf("help to closed output = %d", code)
	}
}

func TestSelectedCLIExecutesOnlyNamedCanonicalCheck(t *testing.T) {
	root := writeConfig(t, "code-gate:\n  red:\n    command: false\n  green:\n    command: true\n    group: tests\n")
	for _, tt := range []struct {
		args []string
		want int
		text string
	}{
		{[]string{"--no-cache", "--group", "tests"}, exitPass, "PASS"},
		{[]string{"--no-cache", "--group", "default"}, exitRed, "red"},
		{[]string{"--group", "absent"}, exitConfigErr, "unknown group"},
		{[]string{"--group", ""}, exitConfigErr, "one non-empty"},
		{[]string{"--group", "tests", "--check", "green"}, exitConfigErr, "one non-empty"},
		{[]string{"--cheap", "--check", "green"}, exitConfigErr, "one non-empty"},
		{[]string{"--cheap", "--phase", "1"}, exitConfigErr, "one non-empty"},
		{[]string{"--phase", "0"}, exitConfigErr, "phase must be >= 1"},
		{[]string{"--group", "tests", "--exclude-check", "red"}, exitConfigErr, "one non-empty"},
		{[]string{"--group", "tests;true"}, exitConfigErr, "literal group name"},
		{[]string{"--no-cache", "--check", "red"}, exitRed, "red"},
		{[]string{"--check", "green", "--no-cache"}, exitPass, "PASS"},
		{[]string{"--check", "green,red", "--no-cache"}, exitRed, "red"},
		{[]string{"--exclude-check", "red", "--no-cache"}, exitPass, "PASS"},
		{[]string{"--exclude-check", "green", "--no-cache"}, exitRed, "red"},
		{[]string{"--exclude-check", "absent"}, exitConfigErr, "unknown check"},
		{[]string{"--exclude-check", "red,green"}, exitConfigErr, "no checks"},
		{[]string{"--check", "green", "--exclude-check", "red"}, exitConfigErr, "one non-empty"},
		{[]string{"--check", ""}, exitConfigErr, "one non-empty"},
		{[]string{"--exclude-check", ""}, exitConfigErr, "one non-empty"},
		{[]string{"--check", "green,green"}, exitConfigErr, "duplicate check"},
		{[]string{"--check", "green,"}, exitConfigErr, "non-empty"},
		{[]string{"--check", "absent"}, exitConfigErr, "unknown check"},
		{[]string{"--chek", "green"}, exitConfigErr, "flag provided"},
		{[]string{"--check"}, exitConfigErr, "needs an argument"},
		{[]string{"unexpected"}, exitConfigErr, "flags only"},
		{nil, exitRed, "red"},
	} {
		var stderr bytes.Buffer
		if got := runWithFlags(root, tt.args, &stderr, &stderr); got != tt.want || !strings.Contains(stderr.String(), tt.text) {
			t.Fatalf("%v = %d: %s", tt.args, got, stderr.String())
		}
	}
}

func TestCheapSelectionSkipsLaterPhase(t *testing.T) {
	root := writeConfig(t, "code-gate:\n  green:\n    command: true\n  red:\n    command: false\n    phase: 2\n")
	var stderr bytes.Buffer
	if got := runWithFlags(root, []string{"--no-cache", "--cheap"}, &stderr, &stderr); got != exitPass || !strings.Contains(stderr.String(), "PASS") {
		t.Fatalf("cheap = %d: %s", got, stderr.String())
	}
	if strings.Contains(stderr.String(), "red") {
		t.Fatalf("cheap ran phase 2: %s", stderr.String())
	}
	stderr.Reset()
	if got := runWithFlags(root, []string{"--no-cache"}, &stderr, &stderr); got != exitRed {
		t.Fatalf("full gate = %d: %s", got, stderr.String())
	}
	stderr.Reset()
	if got := runWithFlags(root, []string{"--no-cache", "--phase", "1"}, &stderr, &stderr); got != exitPass || !strings.Contains(stderr.String(), "PASS") {
		t.Fatalf("phase 1 = %d: %s", got, stderr.String())
	}
	stderr.Reset()
	if got := runWithFlags(root, []string{"--no-cache", "--group", "default", "--phase", "2"}, &stderr, &stderr); got != exitRed || !strings.Contains(stderr.String(), "red") {
		t.Fatalf("group phase 2 = %d: %s", got, stderr.String())
	}
}
