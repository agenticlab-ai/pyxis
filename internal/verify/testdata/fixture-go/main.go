// Command fixture-go is codegate's Go-stack fixture repo (T3, extended by
// later ladder tasks): a minimal module that is gofmt-clean, vet-clean, and
// builds cleanly, so tests can copy it into a workspace and mutate a copy to
// construct a red or error scenario without disturbing this checked-in tree.
package main

import "fmt"

func main() {
	fmt.Println(greeting())
}

// greeting returns fixture-go's fixed greeting.
func greeting() string {
	return "fixture-go: hello"
}
