package relmanifest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"path"
	"slices"
	"strings"
)

// Manifest is release-manifest/v2 — the derived trust-root object the operand
// config pins by hash (DINV-2). Field order here is the canonical encoding
// order. Parsing also supports v1 with its required dependency-bundle hash.
type Manifest struct {
	SchemaVersion    string                       `json:"schema_version"`
	SystemVersion    string                       `json:"system_version"`
	SourceCommit     string                       `json:"source_commit"`
	Binaries         map[string]map[string]string `json:"binaries"` // tool → platform → sha256
	Payload          Payload                      `json:"payload"`
	DepsBundleSHA256 string                       `json:"deps_bundle_sha256,omitempty"` // optional legacy artifact; new releases omit it
}

// Payload pins the payload archive and every file it installs.
type Payload struct {
	ArchiveSHA256 string        `json:"archive_sha256"`
	Files         []PayloadFile `json:"files"`
}

// PayloadFile is one installed file. Engine-verbatim files carry their
// sha256; repo-owned and generated files carry null — converge validates or
// regenerates those, it never byte-verifies them (installer-v1 §3.2).
type PayloadFile struct {
	Path   string  `json:"path"`
	SHA256 *string `json:"sha256"`
	Class  Class   `json:"class"`
}

// ParseManifest parses supported release-manifest versions CLOSED: an unknown key at
// any nesting level, trailing content after the object, or any field
// inconsistent with the schema's invariants is a named defect (DINV-2).
func ParseManifest(b []byte) (Manifest, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var m Manifest
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("release-manifest: %w", err)
	}
	if err := dec.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		return Manifest{}, errors.New("release-manifest: trailing content after the manifest object")
	}
	if err := m.validate(); err != nil {
		return Manifest{}, fmt.Errorf("release-manifest: %w", err)
	}
	return m, nil
}

// Encode renders the manifest's one canonical byte form — two-space-indented
// JSON, struct-ordered fields, encoding/json's sorted map keys, one trailing
// newline — so a derived manifest is byte-stable across runs and a
// parse→encode round trip reproduces its input exactly. An invalid manifest
// refuses to encode (fail closed).
func (m Manifest) Encode() ([]byte, error) {
	if err := m.validate(); err != nil {
		return nil, fmt.Errorf("release-manifest: encode: %w", err)
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("release-manifest: encode: %w", err)
	}
	return append(b, '\n'), nil
}

// validate enforces the invariants shared by parse, encode, and the walk's
// derivation; defects accumulate and each names its field or file.
func (m Manifest) validate() error {
	var errs []error
	if m.SchemaVersion != ManifestSchemaVersion && m.SchemaVersion != legacyManifestSchemaVersion {
		errs = append(errs, fmt.Errorf("schema_version %q: this parser speaks %q and %q (closed parse)", m.SchemaVersion, legacyManifestSchemaVersion, ManifestSchemaVersion))
	}
	if m.SystemVersion == "" {
		errs = append(errs, errors.New("system_version is empty"))
	}
	if m.SourceCommit == "" {
		errs = append(errs, errors.New("source_commit is empty"))
	}
	if err := checkHex("payload.archive_sha256", m.Payload.ArchiveSHA256); err != nil {
		errs = append(errs, err)
	}
	if m.SchemaVersion == legacyManifestSchemaVersion || m.DepsBundleSHA256 != "" {
		if err := checkHex("deps_bundle_sha256", m.DepsBundleSHA256); err != nil {
			errs = append(errs, err)
		}
	}
	for _, tool := range slices.Sorted(maps.Keys(m.Binaries)) {
		platforms := m.Binaries[tool]
		if len(platforms) == 0 {
			errs = append(errs, fmt.Errorf("binaries[%q]: no platform entries", tool))
		}
		for _, plat := range slices.Sorted(maps.Keys(platforms)) {
			if err := checkHex(fmt.Sprintf("binaries[%q][%q]", tool, plat), platforms[plat]); err != nil {
				errs = append(errs, err)
			}
		}
	}
	if len(m.Payload.Files) == 0 {
		errs = append(errs, errors.New("payload.files is empty: a release payload always installs files"))
	}
	seen := make(map[string]bool, len(m.Payload.Files))
	for _, f := range m.Payload.Files {
		if f.Path == "" {
			errs = append(errs, errors.New("payload file with an empty path"))
			continue
		}
		if seen[f.Path] {
			errs = append(errs, fmt.Errorf("duplicate payload path %q", f.Path))
		}
		seen[f.Path] = true
		if strings.HasPrefix(f.Path, "/") || f.Path == ".." || strings.HasPrefix(f.Path, "../") || path.Clean(f.Path) != f.Path {
			errs = append(errs, fmt.Errorf("payload path %q is not a clean, relative, slash-separated path", f.Path))
		}
		switch f.Class {
		case EngineVerbatim:
			if f.SHA256 == nil {
				errs = append(errs, fmt.Errorf("payload file %q: class %q requires a sha256, got null", f.Path, f.Class))
			} else if err := checkHex(fmt.Sprintf("payload file %q sha256", f.Path), *f.SHA256); err != nil {
				errs = append(errs, err)
			}
		case RepoOwned, Generated:
			if f.SHA256 != nil {
				errs = append(errs, fmt.Errorf("payload file %q: class %q must carry sha256 null, got %q (converge never byte-verifies it)", f.Path, f.Class, *f.SHA256))
			}
		default:
			errs = append(errs, fmt.Errorf("payload file %q: unknown ownership class %q (want %s, %s, or %s)", f.Path, f.Class, EngineVerbatim, RepoOwned, Generated))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// checkHex demands a 64-char lowercase sha256 hex string.
func checkHex(field, v string) error {
	if len(v) != 64 {
		return fmt.Errorf("%s: %q is not a 64-char lowercase sha256 hex", field, v)
	}
	for _, c := range v {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return fmt.Errorf("%s: %q is not a 64-char lowercase sha256 hex", field, v)
		}
	}
	return nil
}
