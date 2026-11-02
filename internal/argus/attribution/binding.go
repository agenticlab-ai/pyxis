package attribution

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strings"
)

// BindTrailerKey is the stable, documented commit-trailer key that carries the
// produced Argus trailer's in-tree content binding — a `Argus-Bind: <hex>`
// trailer the producer stamps alongside `Argus-Drive: <slug>` and the gate
// re-derives from the commit itself (§3.2, DINV-4, §7 Q2). It is a contractual,
// in-tree signal (AP-6): a well-formed Argus commit carries BOTH trailers, and
// the gate treats a `Argus-Drive` without a matching `Argus-Bind` as
// underivable, exactly as it treats a replayed one.
const BindTrailerKey = "Argus-Bind"

// bindingVersion pins the binding's derivation so the producer and the gate
// compute the same digest and a future change to the composite is a visible,
// version-gated break rather than a silent drift (AP-6).
const bindingVersion = "argus-bind/v1"

// ComputeBinding derives the in-tree content binding a produced Argus trailer
// carries: a credential-free SHA-256 over the commit's own tree and parent
// object identities (§3.2, DINV-4). It is the single-sourced contract both
// halves consume — the producer (internal/argus/stamp, at commit-authoring time
// from the tree it is about to commit and HEAD) and the gate (this package, at
// CI from the finished commit's `%T`/`%P`) — so the two can never drift.
//
// The binding closes replay: a trailer lifted verbatim onto a DIFFERENT commit
// binds that original commit's tree+parents, which no longer match the new
// commit's own, so the gate's re-derivation mismatches and the diff derives
// underivable — never Argus (no frame) and, on a protected-path diff, fail
// closed (no silent pass). It reads no key and no credential. In the
// shared-account workflow this detects stale or replayed trailers, but a drive
// with raw Git access can mint a new binding (§3.4 of argus-draft-attribution).
// Binding to both tree AND parents is deliberate: a tree bind
// alone would admit a content-identical replay grafted onto a different parent,
// and a parent bind alone would admit a content-substituted commit under the
// same parent — the pair rejects both, and a replay that preserved both
// produces a byte-identical diff (nothing to launder).
//
// parents are the commit's parent SHAs in order (empty for a root commit);
// blank entries are ignored so the producer's `git rev-parse HEAD` (single
// parent, or a failure on an unborn branch → none) and the gate's `%P` (space-
// separated, or empty) agree.
func ComputeBinding(tree string, parents []string) string {
	h := sha256.New()
	_, _ = io.WriteString(h, bindingVersion+"\n")
	_, _ = io.WriteString(h, strings.TrimSpace(tree)+"\n")
	for _, p := range parents {
		if p = strings.TrimSpace(p); p != "" {
			_, _ = io.WriteString(h, p+"\n")
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}
