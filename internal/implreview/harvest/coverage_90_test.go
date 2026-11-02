package harvest

import "testing"

func TestCoverage90ExactFenceMatch(t *testing.T) {
	if !hasExactFence("before\n```impl-review\nafter", "impl-review") {
		t.Fatal("exact review fence was not found")
	}
}
