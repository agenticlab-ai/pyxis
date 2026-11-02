package gitsrc

import (
	"context"
	"strings"
	"sync"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

// pinnedReads holds answers that depend on immutable objects only: the output
// of a read whose every revision operand is a full commit id, and blob bytes
// by blob id. A commit id fixes its tree, its blobs and its ancestry, so asking
// Git again within one invocation can only repeat the answer.
//
// Moving refs (HEAD, branches), the index, the working tree and repository
// probes are never kept, and neither is a failure or a missing object. A
// finished context goes straight to Git, which reports it as it always has.
type pinnedReads struct {
	mu      sync.Mutex
	outputs map[string][]byte // exact arguments → stdout
	blobs   map[string][]byte // blob id → content
}

func (p *pinnedReads) output(key string) ([]byte, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	out, ok := p.outputs[key]
	return out, ok
}

func (p *pinnedReads) keepOutput(key string, out []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.outputs[key] = out
}

func (p *pinnedReads) blob(id string) ([]byte, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	body, ok := p.blobs[id]
	return body, ok
}

func (p *pinnedReads) keepBlob(id string, body []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.blobs[id] = body
}

// KeepPinnedReads makes this handle answer each read of a pinned commit once
// for as long as the handle lives. A caller opts in per invocation. Nothing is
// written to disk or shared with another handle, so every invocation still
// reads each object it needs from the object store at least once.
func (r *Repo) KeepPinnedReads() {
	r.pinned = &pinnedReads{outputs: map[string][]byte{}, blobs: map[string][]byte{}}
}

// keeps reports whether kept answers may be used under ctx.
func (r *Repo) keeps(ctx context.Context) bool {
	return r.pinned != nil && ctx.Err() == nil
}

// pinnedRevision reports whether ref can only ever name one commit: a full
// commit id, or the first parent of one.
func pinnedRevision(ref string) bool {
	return record.ValidCommitID(strings.TrimSuffix(ref, "^1"))
}

// read runs one Git read. When pinned says every revision operand is a full
// commit id, a successful answer is kept and repeated instead of spawning Git
// again. Callers must not modify the returned bytes.
func (r *Repo) read(ctx context.Context, pinned bool, args ...string) ([]byte, error) {
	if !pinned || !r.keeps(ctx) {
		return r.run(ctx, nil, args...)
	}
	key := strings.Join(args, "\x00")
	if out, ok := r.pinned.output(key); ok {
		return out, nil
	}
	out, err := r.run(ctx, nil, args...)
	if err != nil {
		return nil, err
	}
	r.pinned.keepOutput(key, out)
	return out, nil
}

// blobIDs maps each blob path in commit's tree to its blob id. It asks for the
// same listing as EvidenceTree, so the two share one kept answer.
func (r *Repo) blobIDs(ctx context.Context, commit string) (map[string]string, error) {
	out, err := r.read(ctx, true, "ls-tree", "-rz", "--full-tree", commit)
	if err != nil {
		return nil, err
	}
	ids := map[string]string{}
	for line := range strings.SplitSeq(string(out), "\x00") {
		meta, p, ok := strings.Cut(line, "\t")
		if fields := strings.Fields(meta); ok && len(fields) == 3 && fields[1] == "blob" {
			ids[p] = fields[2]
		}
	}
	return ids, nil
}

// pinnedFiles reads paths at a pinned commit, skipping every blob this handle
// has already read for any commit. The commit's own tree names each path's
// blob. A path that tree does not list as a blob, and a tree that cannot be
// listed, take the original query unchanged, so they fail exactly as before.
func (r *Repo) pinnedFiles(ctx context.Context, commit string, paths []string) (map[string][]byte, error) {
	ids, err := r.blobIDs(ctx, commit)
	if err != nil {
		return r.readFiles(ctx, commit, paths)
	}
	out := make(map[string][]byte, len(paths))
	var unread []string
	for _, p := range paths {
		if body, ok := r.pinned.blob(ids[p]); ok {
			out[p] = body
			continue
		}
		unread = append(unread, p)
	}
	if len(unread) == 0 {
		return out, nil
	}
	read, err := r.readFiles(ctx, commit, unread)
	if err != nil {
		return nil, err
	}
	for p, body := range read {
		out[p] = body
		if id := ids[p]; id != "" {
			r.pinned.keepBlob(id, body)
		}
	}
	return out, nil
}
