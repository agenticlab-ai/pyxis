package converge

// Payload-archive extraction and the atomic tree writer — the mechanics
// behind byte-exact engine-verbatim repair (installer-v1 §3.2; UB-5's
// kill-and-re-run posture rides the per-file atomicity here).

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// payloadEntry is one file extracted from the verified payload archive.
type payloadEntry struct {
	b    []byte
	exec bool // the archive's owner-exec bit (the packer normalizes 0755/0644)
}

func (e payloadEntry) mode() fs.FileMode {
	if e.exec {
		return 0o755
	}
	return 0o644
}

// extractFiles reads the verified payload archive once and returns exactly
// the wanted entries (manifest-validated clean relative paths; nothing else
// is materialized — a stray archive member is ignored, never written). A
// wanted path the archive does not carry is the CALLER's named refusal: the
// map simply lacks it.
func extractFiles(archivePath string, want map[string]bool) (map[string]payloadEntry, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, fmt.Errorf("open payload archive: %w", err)
	}
	defer func() { _ = f.Close() }() // read-only handle
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("payload archive %s: gzip: %w", filepath.Base(archivePath), err)
	}
	defer func() { _ = gz.Close() }() // read side; the copy error below governs
	tr := tar.NewReader(gz)
	got := make(map[string]payloadEntry, len(want))
	for len(got) < len(want) {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("payload archive %s: read: %w", filepath.Base(archivePath), err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue // the packed payload carries regular files only
		}
		name := path.Clean(strings.TrimPrefix(hdr.Name, "./"))
		if !want[name] {
			continue
		}
		b, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("payload archive %s: extract %s: %w", filepath.Base(archivePath), name, err)
		}
		got[name] = payloadEntry{b: b, exec: hdr.FileInfo().Mode()&0o100 != 0}
	}
	return got, nil
}

// writeTreeAtomic writes one repo-tree file via tmp + rename in the target's
// own directory (same filesystem by construction), creating parents. Readers
// — and killed runs — only ever see the old bytes, the new bytes, or an
// abandoned dot-tmp; never a torn file (UB-5, AP-22).
func writeTreeAtomic(abs string, b []byte, mode fs.FileMode) error {
	dir := filepath.Dir(abs)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".pyxis-converge-*")
	if err != nil {
		return fmt.Errorf("create tmp in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer func() {
		if tmpName != "" {
			_ = os.Remove(tmpName) // best-effort: an abandoned tmp is scratch, never content
		}
	}()
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close() // the write error governs; the tmp is removed by the deferred cleanup
		return fmt.Errorf("write %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write %s: %w", tmpName, err)
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return fmt.Errorf("chmod %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, abs); err != nil {
		return fmt.Errorf("install %s: %w", abs, err)
	}
	tmpName = "" // renamed into place; nothing to clean
	return nil
}
