package conformance

import "testing"

func TestPredicateSetHelpers(t *testing.T) {
	if !equalInts([]int{1, 2}, []int{1, 2}) || equalInts([]int{1}, []int{2}) || equalInts([]int{1}, []int{1, 2}) {
		t.Fatal("equalInts comparison failed")
	}
	if !contains([]int{1, 2}, 2) || contains([]int{1, 2}, 3) {
		t.Fatal("contains comparison failed")
	}
}

func TestDependencyPredicateHelpers(t *testing.T) {
	if !hasRef([]string{"7"}, 7) || hasRef([]string{"owner/repo#7", "bad"}, 7) {
		t.Fatal("hasRef classification failed")
	}
	if !namesAll("cycle #1 -> #2", 1, 2) || namesAll("cycle #1", 1, 2) {
		t.Fatal("namesAll classification failed")
	}
}
