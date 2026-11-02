// Package port declares the five consumer-side interfaces the pyxis executor
// depends on to turn one planned task into a verified proposal: WorkTracker,
// Lease, Workspace, AgentRuntime and CodeHost (docs/tech-design/implement-now.md
// §3.1).
//
// These are ports in the hexagonal sense (AP-1): the engine core declares what
// it needs and adapters implement them in their own packages. This package
// imports no adapter and no runtime-specific (local vs remote) code, so the same
// Worker core drives both the local in-process runtime and a future
// remote/Docker fleet with no change (DINV-10).
//
// The value types crossing these ports — Task, LaunchSpec, RunResult, Proposal,
// CheckState — are plain structs and enums with explicit fields; they evolve
// additively (AP-6). Sentinel errors (ErrClaimLost, ErrNoProposal) are part of
// the contract and are matched with errors.Is.
//
// ProposalStateReader is a later, additive port (docs/tech-design/
// state-reader-v2.md §3.1): one batched read of a proposal's landing,
// per-context check, and comment state, that a wake/watch consumer polls on a
// cadence instead of inventing its own medium client. It lives in its own
// file (statereader.go), and every implementation — present or future — is
// bound by this package's conformance kit (statereaderkit.go,
// CheckReaderConformance): a new medium adapter passes the kit or it does not
// exist (AP-3).
package port
