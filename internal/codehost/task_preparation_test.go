package codehost

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestTaskPreparationUsesItsOwnOperationAndExactNonce(t *testing.T) {
	p, err := newTaskPreparation(bytes.NewReader(bytes.Repeat([]byte{0xab}, 16)))
	if err != nil || p.Nonce != strings.Repeat("ab", 16) || p.Validate() != nil {
		t.Fatalf("task preparation: %+v %v", p, err)
	}
	if fresh, err := NewTaskPreparation(); err != nil || fresh.Validate() != nil {
		t.Fatalf("fresh task preparation: %+v %v", fresh, err)
	}
	if _, err := newTaskPreparation(bytes.NewReader(nil)); err != io.EOF {
		t.Fatalf("entropy failure lost: %v", err)
	}
	for _, mutate := range []func(*TaskPreparation){
		func(p *TaskPreparation) { p.SchemaVersion = "unknown" },
		func(p *TaskPreparation) { p.Operation = DraftLock },
		func(p *TaskPreparation) { p.Nonce = "" },
	} {
		bad := p
		mutate(&bad)
		if bad.Validate() == nil {
			t.Fatal("invalid task preparation accepted")
		}
	}
}
