package reconcile

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"

	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

type memoryClosureEntry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// MemoryClosureSHA256 returns the canonical identity of every regular file
// the memory loader observed. Invalid envelopes and orphaned bodies remain in
// Set.Bytes, so fixing, adding, deleting, renaming, or changing any memory file
// changes this identity even when the resulting finding addresses do not.
func MemoryClosureSHA256(set *store.Set) string {
	if set == nil {
		return ""
	}
	paths := make([]string, 0, len(set.Bytes))
	for name := range set.Bytes {
		paths = append(paths, name)
	}
	sort.Strings(paths)
	entries := make([]memoryClosureEntry, 0, len(paths))
	for _, name := range paths {
		digest := sha256.Sum256(set.Bytes[name])
		entries = append(entries, memoryClosureEntry{Path: name, SHA256: hex.EncodeToString(digest[:])})
	}
	return digestJSON(entries)
}
