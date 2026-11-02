// Command probe is fixture-go's exec-probe target: a trivial "is the built
// artifact alive" smoke check (FR-13's execution-reality floor). It exits
// nonzero when FIXTURE_PROBE_FAIL is set, so tests can drive both the pass
// and fail paths through the same binary via a pinned env entry.
package main

import (
	"fmt"
	"os"
)

func main() {
	if os.Getenv("FIXTURE_PROBE_FAIL") != "" {
		fmt.Fprintln(os.Stderr, "probe: simulated failure")
		os.Exit(1)
	}
	fmt.Println("probe: ok")
}
