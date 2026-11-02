//go:build unix

package main

import (
	"strings"
	"testing"
)

func TestExecToolReturnsNamedExecveFailure(t *testing.T) {
	code, err := execTool("/definitely/not/a/pyxis/tool", []string{"missing"}, []string{})
	if code != 0 || err == nil || !strings.Contains(err.Error(), "execve /definitely/not/a/pyxis/tool") {
		t.Fatalf("execTool = (%d, %v)", code, err)
	}
}
