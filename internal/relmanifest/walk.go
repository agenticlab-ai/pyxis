package relmanifest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Meta carries the release-level fields the payload walk cannot derive from
// the tree itself: the version pair, provenance, per-binary per-platform
// hashes, and the two aggregate artifact hashes. The release build computes
// these and hands them in; Walk folds them into the derived manifest
// verbatim.
type Meta struct {
	SystemVersion    string
	SourceCommit     string
	Binaries         map[string]map[string]string // tool → platform → sha256
	ArchiveSHA256    string                       // the payload archive's own hash
	DepsBundleSHA256 string
}

// UnclassedError reports the payload files no class-policy glob matches —
// the DINV-6 refusal that fails the release build. Paths are
// payload-relative, slash-separated, sorted.
type UnclassedError struct {
	Paths []string
}

func (e *UnclassedError) Error() string {
	return fmt.Sprintf("%d payload file(s) match no class-policy glob (DINV-6): %s",
		len(e.Paths), strings.Join(e.Paths, ", "))
}

// Walk derives a release manifest from the packed payload tree rooted at
// payloadRoot: every regular file is classed by the policy's first matching
// glob; engine-verbatim files are sha256-hashed, repo-owned and generated
// files carry null. A file no glob matches fails the walk with an
// *UnclassedError naming exactly the unmatched files (DINV-6, A-11), and a
// non-regular entry (symlink, device) is refused by name — the packed
// payload carries regular files only. Output is deterministic: files sort by
// path, and the derived manifest validates before it is returned (fail
// closed).
func Walk(payloadRoot string, policy ClassPolicy, meta Meta) (Manifest, error) {
	files := []PayloadFile{}
	var unclassed []string
	walkErr := filepath.WalkDir(payloadRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(payloadRoot, p)
		if err != nil {
			return fmt.Errorf("relativize %q: %w", p, err)
		}
		relPath := filepath.ToSlash(rel)
		if !d.Type().IsRegular() {
			return fmt.Errorf("payload entry %q is not a regular file: the packed payload carries regular files only", relPath)
		}
		class, ok, err := policy.ClassOf(relPath)
		if err != nil {
			return err
		}
		if !ok {
			unclassed = append(unclassed, relPath)
			return nil
		}
		file := PayloadFile{Path: relPath, Class: class}
		if class == EngineVerbatim {
			sum, err := hashFile(p)
			if err != nil {
				return fmt.Errorf("hash %q: %w", relPath, err)
			}
			file.SHA256 = &sum
		}
		files = append(files, file)
		return nil
	})
	if walkErr != nil {
		return Manifest{}, fmt.Errorf("walk payload %s: %w", payloadRoot, walkErr)
	}
	if len(unclassed) > 0 {
		slices.Sort(unclassed)
		return Manifest{}, fmt.Errorf("walk payload %s: %w", payloadRoot, &UnclassedError{Paths: unclassed})
	}
	slices.SortFunc(files, func(a, b PayloadFile) int { return strings.Compare(a.Path, b.Path) })
	binaries := meta.Binaries
	if binaries == nil {
		binaries = map[string]map[string]string{}
	}
	m := Manifest{
		SchemaVersion:    ManifestSchemaVersion,
		SystemVersion:    meta.SystemVersion,
		SourceCommit:     meta.SourceCommit,
		Binaries:         binaries,
		Payload:          Payload{ArchiveSHA256: meta.ArchiveSHA256, Files: files},
		DepsBundleSHA256: meta.DepsBundleSHA256,
	}
	if err := m.validate(); err != nil {
		return Manifest{}, fmt.Errorf("derived manifest: %w", err)
	}
	return m, nil
}

// hashFile streams one file through sha256, returning lowercase hex.
func hashFile(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
