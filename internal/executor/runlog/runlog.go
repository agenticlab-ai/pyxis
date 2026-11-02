// Package runlog owns the simple, human-readable implement-now execution log.
package runlog

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Open opens the one log reused by every run of feature. The first run puts
// its creation date in the filename; later runs find and append to that file.
func Open(repo, directory, feature string, now time.Time) (*os.File, string, error) {
	feature = filenameFeature(feature)
	if !filepath.IsAbs(directory) {
		directory = filepath.Join(repo, directory)
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return nil, "", fmt.Errorf("create implement-now log directory: %w", err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, "", fmt.Errorf("find implement-now log: %w", err)
	}
	path := filepath.Join(directory, now.Format("01-02-06")+"-"+feature+".log")
	// ReadDir returns names in sorted order. Match the entire date-and-feature
	// structure, without interpreting the directory or feature as a glob.
	for _, entry := range entries {
		date, ok := strings.CutSuffix(entry.Name(), "-"+feature+".log")
		if entry.IsDir() || !ok || len(date) != len("01-02-06") {
			continue
		}
		if _, err := time.Parse("01-02-06", date); err != nil {
			continue
		}
		path = filepath.Join(directory, entry.Name())
		break
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, "", fmt.Errorf("open implement-now log: %w", err)
	}
	return f, path, nil
}

// WriteLine appends one timestamped, plain-English line.
func WriteLine(w io.Writer, now time.Time, format string, args ...any) error {
	if w == nil {
		return nil
	}
	_, err := fmt.Fprintf(w, "%s | %s\n", now.UTC().Format(time.RFC3339), fmt.Sprintf(format, args...))
	return err
}

// filenameFeature preserves feature identity, including on case-insensitive
// filesystems. Lowercase slug bytes stay readable; every other byte (including
// uppercase letters and the escape marker itself) gets a fixed percent escape.
func filenameFeature(value string) string {
	if value == "" {
		// This cannot be the encoding of a named feature: an encoded percent is
		// always followed by two hex digits.
		return "%unscoped"
	}
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		c := value[i]
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-' {
			b.WriteByte(c)
		} else {
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&15])
		}
	}
	return b.String()
}
