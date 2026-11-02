package main

// backends.go is the AP-27 backend aggregator: the single import file that
// blank-imports every AgentRuntime adapter so each self-registers from its own
// init (runtime.Register), making the available backend set the derived
// consequence of the imported packages — never a hand-kept switch. Adding a
// backend is a new adapter package plus one line here; the engine core imports no
// adapter (AP-1, AP-25 — the surface is vendor-neutral).
//
// The resolver (resolver.go) builds each backend from this registered set via
// runtime.Resolve; the one native knob the MVP threads — claude's --max-turns —
// is applied there, since claude is the only adapter exposing a config-injectable
// turn cap (§3.7). The other three are bounded uniformly by the Dispatcher's
// wall-clock WorkerBudget.
import (
	_ "github.com/agenticlab-ai/pyxis/internal/adapters/harness/claude"
	_ "github.com/agenticlab-ai/pyxis/internal/adapters/harness/codex"
	_ "github.com/agenticlab-ai/pyxis/internal/adapters/harness/cursor"
	_ "github.com/agenticlab-ai/pyxis/internal/adapters/harness/opencode"
)
