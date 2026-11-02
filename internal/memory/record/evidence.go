package record

import (
	"fmt"
	"path"
	"strings"
)

const (
	EvidenceSchema       = "memory-evidence/v1"
	EvidenceIntroduction = "record-introduction"
	EvidenceMainline     = "mainline-commit"
	EvidenceInventory    = "pyxis-memory-code-inventory/v3"
	EvidenceUnitDomain   = "pyxis-memory-code-inventory/v1/unit"
)

// Evidence locates exact repository bytes in retained mainline history. The
// introduction locator deliberately contains no self-referential commit ID.
type Evidence struct {
	Schema  string         `json:"schema_version"`
	Locator string         `json:"locator"`
	Files   []EvidenceFile `json:"files,omitempty"`
	Units   []EvidenceUnit `json:"units,omitempty"`
}

// EvidenceFile identifies the raw Git blob, including its regular file mode.
type EvidenceFile struct {
	Path   string `json:"path"`
	Mode   string `json:"mode"`
	SHA256 string `json:"sha256"`
}

// EvidenceUnit retains the exact membership and digest of a production unit.
type EvidenceUnit struct {
	Inventory string   `json:"inventory"`
	Domain    string   `json:"digest_domain"`
	Kind      string   `json:"kind"`
	Path      string   `json:"path"`
	SHA256    string   `json:"sha256"`
	Files     []string `json:"files"`
}

// ValidEvidencePath excludes Git revision/pathspec syntax and control bytes.
// Evidence refers to regular files; no path is evaluated as a shell expression.
func ValidEvidencePath(p string) bool {
	for _, c := range p {
		if c < 32 || c == 127 {
			return false
		}
	}
	return p != "" && p != "." && p != ".." && path.Clean(p) == p && !path.IsAbs(p) &&
		!strings.HasPrefix(p, "../") && !strings.HasPrefix(p, "-") &&
		!strings.ContainsAny(p, ":\\\x00\r\n\t")
}

// ValidCommitID accepts full Git SHA-1 or SHA-256 commit identities, not refs.
func ValidCommitID(s string) bool {
	return (len(s) == 40 || len(s) == 64) && strings.Trim(s, "0123456789abcdef") == "" && strings.Trim(s, "0") != ""
}

// ValidateEvidence is schema validation only. The shared verifier additionally
// proves source selection, content, membership, and retained-history reachability.
func (r *Record) ValidateEvidence() error {
	e := r.Provenance.Evidence
	if e == nil {
		return nil
	} // compatibility: the transition gate classifies legacy records
	bad := func(reason string) error { return fmt.Errorf("%w %s: evidence %s", ErrInvalid, r.ID, reason) }
	if e.Schema != EvidenceSchema {
		return bad("has an unsupported schema version")
	}
	switch e.Locator {
	case EvidenceIntroduction:
		if r.Provenance.Commit != "" || len(e.Files) == 0 {
			return bad("record-introduction needs files and no provenance.commit")
		}
	case EvidenceMainline:
		if !ValidCommitID(r.Provenance.Commit) {
			return bad("mainline-commit needs a full provenance.commit")
		}
	default:
		return bad("has an unknown locator")
	}
	previous := ""
	files := make(map[string]bool, len(e.Files))
	for _, f := range e.Files {
		if !ValidEvidencePath(f.Path) || f.Path <= previous || f.Path == "memory/"+r.Path() {
			return bad("file paths must be canonical, unique, sorted, and not the record itself")
		}
		if f.Mode != "100644" && f.Mode != "100755" {
			return bad("file mode must be regular Git mode 100644 or 100755")
		}
		if !sha256RE.MatchString(f.SHA256) {
			return bad("file SHA-256 must be 64 lowercase hex digits")
		}
		previous = f.Path
		files[f.Path] = true
	}
	previous = ""
	for _, u := range e.Units {
		key := u.Path + "\x00" + u.Kind
		if !ValidEvidencePath(u.Path) || key <= previous {
			return bad("units must have canonical unique sorted addresses")
		}
		if u.Inventory != EvidenceInventory || u.Domain != EvidenceUnitDomain {
			return bad("unit uses an unsupported inventory or digest domain")
		}
		if u.Kind != "go-package" && u.Kind != "executable-script" {
			return bad("unit kind is unsupported")
		}
		if !sha256RE.MatchString(u.SHA256) || len(u.Files) == 0 {
			return bad("unit requires a digest and nonempty membership")
		}
		last := ""
		for _, p := range u.Files {
			if p <= last || !files[p] {
				return bad("unit members must be sorted, unique, and present in the file manifest")
			}
			last = p
		}
		previous = key
	}
	return nil
}
