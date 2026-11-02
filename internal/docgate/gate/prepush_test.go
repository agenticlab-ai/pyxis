package gate_test

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
)

func TestDeletionOnlyPush(t *testing.T) {
	const zero = "0000000000000000000000000000000000000000"
	const sha = "ec487ae49c1358bff81212437bf495013c94a8f2"
	cases := []struct {
		name  string
		stdin string
		want  bool
	}{
		{"single deletion", "(delete) " + zero + " refs/heads/x " + sha + "\n", true},
		{"two deletions", "(delete) " + zero + " refs/heads/x " + sha + "\n(delete) " + zero + " refs/heads/y " + sha + "\n", true},
		{"content push", "refs/heads/x " + sha + " refs/heads/x " + zero + "\n", false},
		{"mixed push gates", "(delete) " + zero + " refs/heads/x " + sha + "\nrefs/heads/y " + sha + " refs/heads/y " + zero + "\n", false},
		{"empty stdin gates", "", false},
		{"blank lines only gate", "\n\n", false},
		{"malformed line gates", "just-two fields\n", false},
		{"short zero sha gates", "(delete) 000000 refs/heads/x " + sha + "\n", false},
	}
	for _, c := range cases {
		if got := gate.DeletionOnlyPush(strings.NewReader(c.stdin)); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
