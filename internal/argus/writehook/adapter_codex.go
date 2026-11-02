package writehook

// adapter_codex is the backstop-documented adapter for the Codex CLI (the
// .codex vendor surface). Codex exposes no honoring, fail-closable pre-write
// hook against which a write-time deny can be wired (its write mediation is a
// sandbox/approval posture, not a scriptable per-write deny the repo controls).
// Per UB-2 / DINV-4 a CLI with no fail-closable hook is *listed* with the
// merge-boundary layers as its stated guarantee — never a silent fail-open.
//
// Honoring=false leaves the I/O funcs nil, so Adapter.Evaluate denies any
// evaluation attempt fail-closed rather than pretend to enforce, and the
// guarantee listing (`argus-writehook guarantees`) names the backstop. Adding
// a fail-closable Codex hook later is editing this one file (Honoring=true + the
// I/O funcs), never touching the shared decision — the extension-by-addition
// shape (AP-27).
func init() {
	register(&Adapter{
		Name:     "codex",
		Hook:     "Codex CLI — no honoring, fail-closable pre-write hook surface",
		Honoring: false,
		Backstop: "the merge-boundary layers — the authority-contract structure arm (docgate gate) and the attribution gate (required CI check)",
	})
}
