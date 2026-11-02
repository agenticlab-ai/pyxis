package stamp

// adapter_claude wires Claude Code's commit-authoring path to the canonical
// stamp. Claude Code authors commits by running `git commit` (through its Bash
// tool), so its commit path is standard git: the repo's own
// `core.hooksPath` prepare-commit-msg hook fires on every commit it authors,
// and the shim there delegates to the canonical stamp (cmd/argus-stamp). The
// wiring is therefore a pointer — activate the repo hook surface and set the
// active drive's slug in the drive context — never a content copy of the
// trailer logic (DINV-3, AP-25).
//
// The producer sits on Git's commit-message surface. In the current shared
// account the drive can invoke Git, so the hook supplies audit evidence but
// cannot establish an inaccessible supervisor boundary. A CLI that authored
// commits through its own non-Git path would add its own adapter file.
func init() {
	register(&Adapter{
		Name:       "claude",
		CommitPath: "Claude Code runs `git commit` via its Bash tool — standard git, so the repo's core.hooksPath prepare-commit-msg hook fires on every commit it authors",
		Wiring:     ".pyxis/stamp/claude.md — a pointer: activate the source repo's shared hooks with `go run ./cmd/hooks-setup` and supply $ARGUS_DRIVE_SLUG on each drive commit; the canonical stamp (cmd/argus-stamp) holds the trailer logic once (AP-25)",
	})
}
