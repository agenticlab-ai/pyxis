package budget

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/statedir"
)

// Path keys, per design:state-reader-v4 §3.5 and §3.8.
//
// Every path component derived from medium-supplied text — the host as much
// as the identity — is encoded as "<readable>-<digest>". Sanitize-and-
// truncate alone is NOT injective: "a/b" and "a?b" both reduce to "a_b", two
// identities sharing a long prefix collide once truncated, and "." and ".."
// survive an allowed-character filter untouched, so a "sanitized" component
// can still name its own parent. The digest carries the uniqueness; the
// readable half is legibility and nothing depends on it.
//
// The guarantee is probabilistic and stated as such: a truncated digest can
// collide by pigeonhole. What is bound is that a collision requires a HASH
// collision rather than a sanitizing one, and 64 bits puts that beyond reach
// for the population involved — the handful of identities and hosts one
// machine writes as.
const (
	// digestHex is the hex length of the digest suffix (64 bits).
	digestHex = 16
	// readableMax bounds the legible half, so a component stays short.
	readableMax = 32
)

// atom encodes one raw key component into a path-safe, collision-resistant
// segment. Because the result always ends in "-<digest>", it can never equal
// "." or "..", can never be empty, and can never traverse.
func atom(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	digest := hex.EncodeToString(sum[:])[:digestHex]

	var b strings.Builder
	b.Grow(readableMax)
	for _, r := range raw {
		if b.Len() >= readableMax {
			break
		}
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '.', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String() + "-" + digest
}

// hostDir is the directory holding one medium host's records: its umbrella,
// its cutover marker, and every identity ledger beneath it.
func hostDir(host string) (string, error) {
	return statedir.Sub("content-budget", atom(host))
}

// Path is the ledger for one (medium host, authenticated identity).
//
// The key is never partitioned finer than the platform documents its limit:
// GitHub publishes the content-write caps but not the entity they are scoped
// to, so a per-repository ledger could let N repositories each spend a full
// window against what may be one identity's quota. There is no repository
// input here at all, so that partition cannot be reintroduced by mistake.
func Path(host, identity string) (string, error) {
	dir, err := hostDir(host)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, atom(identity)+".json"), nil
}

// HostPath is the host-wide umbrella ledger: the window covering every
// identity writing to that host from this machine. Every identity ledger
// charges it too (see NewForIdentity), which is what makes it a genuine
// upper bound rather than one more sibling — and what makes the
// unresolved-identity fallback able only to over-throttle (DINV-9).
func HostPath(host string) (string, error) {
	dir, err := hostDir(host)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "_host.json"), nil
}

// MarkerPath is the durable cutover marker for one (machine, medium host):
// the record that this host's ledgers have paid their first-initialization
// barrier and that nothing failing to charge the umbrella has written since
// (DINV-13).
func MarkerPath(host string) (string, error) {
	dir, err := hostDir(host)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "_cutover.json"), nil
}

// LegacyPaths names ledger locations earlier builds used. They are never
// read, migrated, or counted: state written under a different key against a
// different window is not evidence about this one. They are reported only so
// an operator can delete them once their window has passed.
func LegacyPaths(repoPath string) []string {
	out := []string{filepath.Join(repoPath, ".pyxis", "state", "github-content-budget.json")}
	if dir, err := os.UserCacheDir(); err == nil {
		out = append(out, filepath.Join(dir, "pyxis", "github-budget"))
	}
	return out
}

// StaleLegacyPaths is LegacyPaths filtered to what exists, plus the debris a
// ledger leaves beside it — every record has a sibling lock, and an
// interrupted write can strand a temp. Reporting only the JSON would tell an
// operator a location is clean while two files remain.
func StaleLegacyPaths(repoPath string) []string {
	var out []string
	for _, p := range LegacyPaths(repoPath) {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		out = append(out, p)
		if info.IsDir() {
			continue
		}
		out = append(out, debris(p)...)
	}
	return out
}

func debris(path string) []string {
	var out []string
	if _, err := os.Stat(path + lockSuffix); err == nil {
		out = append(out, path+lockSuffix)
	}
	if temps, err := filepath.Glob(filepath.Join(filepath.Dir(path), tempPrefix+"*")); err == nil {
		out = append(out, temps...)
	}
	return out
}
