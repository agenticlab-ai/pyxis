// Package prdescriptiontest supplies the final message of a compliant
// implementing agent for executor tests: a complete pull request description
// between the markers (internal/prdescription).
package prdescriptiontest

import "github.com/agenticlab-ai/pyxis/internal/prdescription"

// Message is a complete description as an implementing agent ends its final
// message with it. Pyxis computes Lines of code, so the agent leaves it.
const Message = prdescription.BeginMarker + `
## Human Reviewer data
- Lines of code: Pyxis computes this
- Complexity: low
- Blast radius: low
- Decision type: two-way
- Review notes: Nothing unusual.

## What
- Implement the task.

## Why
The task asks for it.

## How
- Directly.

## Testing
- [x] go test ./...: passed.
` + prdescription.EndMarker
