package converge

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractFilesRefusesUnreadableAndMalformedArchives(t *testing.T) {
	if _, err := extractFiles(filepath.Join(t.TempDir(), "missing.tgz"), map[string]bool{"wanted": true}); err == nil {
		t.Fatal("missing archive: want error")
	}

	badGzip := filepath.Join(t.TempDir(), "bad.tgz")
	if err := os.WriteFile(badGzip, []byte("not gzip"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := extractFiles(badGzip, map[string]bool{"wanted": true}); err == nil {
		t.Fatal("invalid gzip: want error")
	}

	badTar := gzipBytes(t, []byte("short tar header"))
	if _, err := extractFiles(writeArchiveBytes(t, badTar), map[string]bool{"wanted": true}); err == nil {
		t.Fatal("invalid tar: want error")
	}

	var raw strings.Builder
	tw := tar.NewWriter(&raw)
	if err := tw.WriteHeader(&tar.Header{Name: "wanted", Mode: 0o644, Size: 10, Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte("abc")); err != nil {
		t.Fatal(err)
	}
	_ = tw.Close() // deliberately short relative to the declared entry size
	if _, err := extractFiles(writeArchiveBytes(t, gzipBytes(t, []byte(raw.String()))), map[string]bool{"wanted": true}); err == nil {
		t.Fatal("truncated member: want error")
	}
}

func TestExtractFilesSkipsDirectoriesAndUnwantedMembers(t *testing.T) {
	var raw strings.Builder
	tw := tar.NewWriter(&raw)
	for _, header := range []tar.Header{
		{Name: "dir", Mode: 0o755, Typeflag: tar.TypeDir},
		{Name: "other", Mode: 0o644, Size: 5, Typeflag: tar.TypeReg},
		{Name: "./wanted", Mode: 0o755, Size: 4, Typeflag: tar.TypeReg},
	} {
		if err := tw.WriteHeader(&header); err != nil {
			t.Fatal(err)
		}
		switch header.Name {
		case "other":
			_, _ = tw.Write([]byte("other"))
		case "./wanted":
			_, _ = tw.Write([]byte("data"))
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := extractFiles(writeArchiveBytes(t, gzipBytes(t, []byte(raw.String()))), map[string]bool{"wanted": true})
	if err != nil {
		t.Fatal(err)
	}
	if string(got["wanted"].b) != "data" || !got["wanted"].exec {
		t.Fatalf("wanted entry = %+v", got["wanted"])
	}
}

func TestWriteTreeAtomicRefusesBlockedParent(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "parent")
	if err := os.WriteFile(parent, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeTreeAtomic(filepath.Join(parent, "child"), []byte("data"), 0o644); err == nil {
		t.Fatal("blocked parent: want error")
	}
}

func TestWriteTreeAtomicCleansTemporaryFileAfterRenameFailure(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "occupied"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeTreeAtomic(target, []byte("replacement"), 0o644); err == nil {
		t.Fatal("install over non-empty directory: want error")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".pyxis-converge-") {
			t.Fatalf("temporary file was not cleaned: %s", entry.Name())
		}
	}
}

func gzipBytes(t *testing.T, raw []byte) []byte {
	t.Helper()
	var compressed strings.Builder
	zw := gzip.NewWriter(&compressed)
	if _, err := zw.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return []byte(compressed.String())
}

func writeArchiveBytes(t *testing.T, data []byte) string {
	t.Helper()
	archive := filepath.Join(t.TempDir(), "payload.tgz")
	if err := os.WriteFile(archive, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return archive
}
