// Package codeinventory builds a deterministic inventory of first-party
// production code from a pinned Git tree. It is deliberately independent of
// Git and the working tree: callers provide the tree entries and their blobs
// from one already-resolved commit.
package codeinventory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
)

const digestDomain = "pyxis-memory-code-inventory/v1"

// ErrInvalidTree identifies an input that cannot safely describe a pinned
// code inventory. Callers must not publish a partial inventory after this
// error.
var ErrInvalidTree = errors.New("invalid code inventory tree")

// Entry is one entry from a pinned Git tree. Mode and Type use Git's values,
// for example 100644/blob, 100755/blob, 120000/blob, or 160000/commit.
type Entry struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"`
}

// UnitKind distinguishes a Go package directory from a production script.
// ExecutableScript retains its schema value for compatibility and also covers
// sourced zsh files, which do not need an executable bit.
type UnitKind string

const (
	GoPackage        UnitKind = "go-package"
	ExecutableScript UnitKind = "executable-script"
)

// Unit is one code-association target. A Go unit is a package directory and
// may contain several files. An executable-script unit is one file, including
// a sourced zsh file. Files and units are always returned in lexical order.
// SHA256 hashes the v1 canonical
// JSON tuple of kind, unit path, and each file's path, Git mode, and content
// SHA-256; exact blob bytes are significant and are never normalized.
type Unit struct {
	Kind   UnitKind `json:"kind"`
	Path   string   `json:"path"`
	Files  []string `json:"files"`
	SHA256 string   `json:"sha256"`
}

// Inventory is the complete production-code inventory for one pinned tree.
// AggregateSHA256 hashes the v1 canonical JSON tuple of every unit's kind,
// path, and SHA256 in returned order.
type Inventory struct {
	Units           []Unit `json:"units"`
	AggregateSHA256 string `json:"aggregate_sha256"`
}

type selectedFile struct {
	path string
	mode string
	body []byte
}

type unitKey struct {
	kind UnitKind
	path string
}

// Build classifies first-party production code and computes its canonical
// identities. Go source contributes one unit per package directory under
// cmd/, internal/, or pkg/. Executable .sh and .mjs files and regular sourced
// .zsh files contribute one unit each. Test-only and generated/vendor fixture
// trees are excluded.
//
// Only exact blobs supplied in blobs are hashed. A missing blob or an
// irregular in-scope entry is an error; Build never returns a partial result.
func Build(entries []Entry, blobs map[string][]byte) (Inventory, error) {
	return build(entries, blobs, false)
}

// BuildLegacyV2 reproduces the production-code inventory emitted by the v1
// and v2 reconciliation generators. Those generators predated sourced zsh
// support and therefore excluded every .zsh path, regardless of its Git mode.
// Current callers must use Build; this compatibility surface exists only to
// prove the exact inputs of an already-trusted legacy baseline.
func BuildLegacyV2(entries []Entry, blobs map[string][]byte) (Inventory, error) {
	return build(entries, blobs, true)
}

func build(entries []Entry, blobs map[string][]byte, legacyV2 bool) (Inventory, error) {
	ordered := append([]Entry(nil), entries...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Path != ordered[j].Path {
			return ordered[i].Path < ordered[j].Path
		}
		if ordered[i].Mode != ordered[j].Mode {
			return ordered[i].Mode < ordered[j].Mode
		}
		return ordered[i].Type < ordered[j].Type
	})
	seen := make(map[string]struct{}, len(entries))
	byUnit := map[unitKey][]selectedFile{}

	for _, entry := range ordered {
		if err := validatePath(entry.Path); err != nil {
			return Inventory{}, err
		}
		if _, duplicate := seen[entry.Path]; duplicate {
			return Inventory{}, fmt.Errorf("%w: duplicate entry %q", ErrInvalidTree, entry.Path)
		}
		seen[entry.Path] = struct{}{}

		kind, unitPath, candidate := classify(entry.Path)
		if !candidate {
			continue
		}
		if legacyV2 && path.Ext(path.Base(entry.Path)) == ".zsh" {
			continue
		}
		if entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") {
			return Inventory{}, fmt.Errorf("%w: in-scope path %q is %s/%s, not a regular file", ErrInvalidTree, entry.Path, entry.Mode, entry.Type)
		}
		if kind == ExecutableScript && !scriptModeSelected(entry.Path, entry.Mode) {
			continue
		}
		body, ok := blobs[entry.Path]
		if !ok {
			return Inventory{}, fmt.Errorf("%w: in-scope path %q has no blob bytes", ErrInvalidTree, entry.Path)
		}

		key := unitKey{kind: kind, path: unitPath}
		byUnit[key] = append(byUnit[key], selectedFile{path: entry.Path, mode: entry.Mode, body: body})
	}

	keys := make([]unitKey, 0, len(byUnit))
	for key := range byUnit {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].path != keys[j].path {
			return keys[i].path < keys[j].path
		}
		return keys[i].kind < keys[j].kind
	})

	inv := Inventory{Units: make([]Unit, 0, len(keys))}
	for _, key := range keys {
		files := byUnit[key]
		sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })
		unit := Unit{Kind: key.kind, Path: key.path, Files: make([]string, 0, len(files))}
		wireFiles := make([]fileDigestWire, 0, len(files))
		for _, file := range files {
			unit.Files = append(unit.Files, file.path)
			wireFiles = append(wireFiles, fileDigestWire{
				Path: file.path, Mode: file.mode, SHA256: digest(file.body),
			})
		}
		unit.SHA256 = digestJSON(unitDigestWire{
			Domain: digestDomain + "/unit", Kind: unit.Kind, Path: unit.Path, Files: wireFiles,
		})
		inv.Units = append(inv.Units, unit)
	}

	aggregate := make([]aggregateUnitWire, 0, len(inv.Units))
	for _, unit := range inv.Units {
		aggregate = append(aggregate, aggregateUnitWire{Kind: unit.Kind, Path: unit.Path, SHA256: unit.SHA256})
	}
	inv.AggregateSHA256 = digestJSON(aggregateDigestWire{Domain: digestDomain + "/aggregate", Units: aggregate})
	return inv, nil
}

var excludedComponents = map[string]struct{}{
	"fixtures":  {},
	"generated": {},
	"testdata":  {},
	"tests":     {},
	"vendor":    {},
}

func classify(name string) (UnitKind, string, bool) {
	parts := strings.Split(name, "/")
	for _, part := range parts {
		if _, excluded := excludedComponents[part]; excluded {
			return "", "", false
		}
	}
	base := path.Base(name)
	if strings.HasSuffix(base, ".go") && !strings.HasSuffix(base, "_test.go") && len(parts) > 1 {
		switch parts[0] {
		case "cmd", "internal", "pkg":
			return GoPackage, path.Dir(name), true
		}
	}
	if IsScriptPath(name) {
		return ExecutableScript, name, true
	}
	return "", "", false
}

// CandidatePath reports whether name can contribute to the production-code
// inventory. Git type and mode validation remains Build's responsibility.
func CandidatePath(name string) bool {
	_, _, candidate := classify(name)
	return candidate
}

// IsScriptPath reports whether name uses one of the production script
// extensions represented by ExecutableScript.
func IsScriptPath(name string) bool {
	switch path.Ext(path.Base(name)) {
	case ".sh", ".mjs", ".zsh":
		return true
	default:
		return false
	}
}

func scriptModeSelected(name, mode string) bool {
	return mode == "100755" || (path.Ext(path.Base(name)) == ".zsh" && mode == "100644")
}

func validatePath(name string) error {
	if name == "" || path.IsAbs(name) || path.Clean(name) != name || strings.Contains(name, "\\") || strings.ContainsRune(name, '\x00') {
		return fmt.Errorf("%w: path %q is not a canonical repository-relative path", ErrInvalidTree, name)
	}
	return nil
}

type fileDigestWire struct {
	Path   string `json:"path"`
	Mode   string `json:"mode"`
	SHA256 string `json:"sha256"`
}

type unitDigestWire struct {
	Domain string           `json:"domain"`
	Kind   UnitKind         `json:"kind"`
	Path   string           `json:"path"`
	Files  []fileDigestWire `json:"files"`
}

type aggregateUnitWire struct {
	Kind   UnitKind `json:"kind"`
	Path   string   `json:"path"`
	SHA256 string   `json:"sha256"`
}

type aggregateDigestWire struct {
	Domain string              `json:"domain"`
	Units  []aggregateUnitWire `json:"units"`
}

func digest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func digestJSON(value any) string {
	body, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("codeinventory: marshal canonical digest input: %v", err))
	}
	return digest(body)
}
