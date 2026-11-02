package release

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Deterministic tar.gz writing (the T11 charter; NFR-2's re-runnable
// publish depends on it): entries sorted by name, timestamps zeroed to the
// epoch, uid/gid zeroed, owner names empty, modes normalized to 0644/0755
// by the source's owner-exec bit. Two packs of the same tree are
// byte-identical on the same toolchain, so upload-or-verify converges.

// tarEntry is one archive member: exactly one of src (a file to stream) or
// data (literal bytes) supplies the content.
type tarEntry struct {
	name string // archive path, slash-separated, relative
	src  string // source file path ("" when data is set)
	data []byte
	exec bool // normalize mode to 0755 (else 0644)
}

// entryEpoch is the zeroed timestamp every member carries.
var entryEpoch = time.Unix(0, 0).UTC()

// writeDeterministicTarGz writes entries (sorted by name, duplicates
// refused) as a gzip'd tar at outPath.
func writeDeterministicTarGz(outPath string, entries []tarEntry) (err error) {
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
	for i := 1; i < len(entries); i++ {
		if entries[i].name == entries[i-1].name {
			return fmt.Errorf("release: archive %s: duplicate entry %q", filepath.Base(outPath), entries[i].name)
		}
	}
	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("release: create %s: %w", outPath, err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("release: close %s: %w", outPath, cerr)
		}
	}()
	gz := gzip.NewWriter(f)
	gz.ModTime = entryEpoch // zero the gzip header's mtime too
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		if werr := writeMember(tw, e); werr != nil {
			return fmt.Errorf("release: archive %s: %w", filepath.Base(outPath), werr)
		}
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("release: finish tar %s: %w", outPath, err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("release: finish gzip %s: %w", outPath, err)
	}
	return nil
}

func writeMember(tw *tar.Writer, e tarEntry) error {
	mode := int64(0o644)
	if e.exec {
		mode = 0o755
	}
	hdr := &tar.Header{
		Name:    e.name,
		Mode:    mode,
		ModTime: entryEpoch,
		Uid:     0,
		Gid:     0,
	}
	if e.src == "" {
		hdr.Size = int64(len(e.data))
		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("header %s: %w", e.name, err)
		}
		if _, err := tw.Write(e.data); err != nil {
			return fmt.Errorf("write %s: %w", e.name, err)
		}
		return nil
	}
	info, err := os.Stat(e.src)
	if err != nil {
		return fmt.Errorf("stat %s: %w", e.name, err)
	}
	hdr.Size = info.Size()
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("header %s: %w", e.name, err)
	}
	f, err := os.Open(e.src)
	if err != nil {
		return fmt.Errorf("open %s: %w", e.name, err)
	}
	defer func() { _ = f.Close() }() // Read-only handle; the copy result governs.
	if _, err := io.Copy(tw, f); err != nil {
		return fmt.Errorf("stream %s: %w", e.name, err)
	}
	return nil
}

// treeEntries collects every regular file under root as tar entries named
// by the root-relative slash path.
func treeEntries(root string) ([]tarEntry, error) {
	var entries []tarEntry
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("entry %s is not a regular file", rel)
		}
		info, ierr := d.Info()
		if ierr != nil {
			return ierr
		}
		entries = append(entries, tarEntry{
			name: rel,
			src:  p,
			exec: info.Mode()&0o100 != 0,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}
