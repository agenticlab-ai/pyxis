// Package forgerelease is the consumer-side Forgejo release port: types and the
// construct-a-host function used by release, resolve, and upgrade. The HTTP
// implementation lives in adapters/release/forgejo.
package forgerelease

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

type Asset struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

type Release struct {
	ID     int64   `json:"id"`
	Tag    string  `json:"tag_name"`
	Assets []Asset `json:"assets"`
}

// Host is the Forgejo release and attachment surface.
type Host interface {
	Lookup(ctx context.Context, tag string) (Release, bool, error)
	Create(ctx context.Context, tag string, requireTag bool) (Release, error)
	TagCommit(ctx context.Context, tag string) (string, error)
	CreateTag(ctx context.Context, tag, commit string) error
	Download(ctx context.Context, a Asset) (io.ReadCloser, error)
	Open(ctx context.Context, tag, name string) (io.ReadCloser, error)
	DownloadTo(ctx context.Context, a Asset, dir string) error
	Upload(ctx context.Context, releaseID int64, file string) error
	ReadInstall(ctx context.Context) ([]byte, error)
	WriteInstall(ctx context.Context, want []byte, tag string) (string, error)
}

type opener func(integrations.ResolvedRepository) (Host, error)

var (
	openMu sync.RWMutex
	open   opener
)

// Bind registers the Forgejo release adapter. Composition roots and tests import
// that adapter so this constructor is wired.
func Bind(fn opener) {
	openMu.Lock()
	open = fn
	openMu.Unlock()
}

func New(repo integrations.ResolvedRepository) (Host, error) {
	openMu.RLock()
	fn := open
	openMu.RUnlock()
	if fn == nil {
		return nil, errors.New("forgejo release adapter is not wired")
	}
	return fn(repo)
}
