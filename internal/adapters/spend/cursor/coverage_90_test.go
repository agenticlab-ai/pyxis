package cursor

import "testing"

func TestCoverage90CommandWithoutArguments(t *testing.T) {
	msg := rawMessage{Content: []rawContent{{Type: "text", Text: "/prd-new"}}}
	if got := commandName(msg); got != "prd-new" {
		t.Fatalf("commandName = %q", got)
	}
}
