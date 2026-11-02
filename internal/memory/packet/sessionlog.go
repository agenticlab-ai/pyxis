package packet

import (
	"path"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/memory/sessionlog"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

// retiredSessionLog admits only the exact empty imports approved for removal.
// It cannot retire a referenced record or hide an edit behind a deletion.
func retiredSessionLog(name string, ancestor, base, head, union *store.Set) bool {
	id := strings.TrimSuffix(path.Base(name), path.Ext(name))
	r := ancestor.Get(id)
	if !sessionlog.Retired(r, ancestor.Bytes[ancestor.Paths[id]], ancestor.Bodies[id]) {
		return false
	}
	if name != r.Path() && name != r.BodyPath() {
		return false
	}
	if _, present := head.Bytes[r.Path()]; present {
		return false
	}
	if _, present := head.Bytes[r.BodyPath()]; present {
		return false
	}
	if !sameRecordBytes(id, ancestor, base) {
		return false
	}
	return len(ancestor.To[id]) == 0 && len(base.To[id]) == 0 && len(head.To[id]) == 0 && len(union.To[id]) == 0
}
