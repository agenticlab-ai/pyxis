package github

import (
	"context"
	"errors"

	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	"github.com/agenticlab-ai/pyxis/internal/implreview/publication"
)

// NativeClient supplies the host-specific operations to the shared, durable
// comment publication protocol. Implementations use their provider's native API.
type NativeClient interface {
	PostComment(context.Context, int, string) (string, error)
	ReadComment(context.Context, string) (string, error)
	CurrentLogin(context.Context) (string, error)
	RepositoryIdentity(context.Context) (string, error)
	CanWriteComments(context.Context) (bool, error)
}

type nativeBridge struct{ NativeClient }

func (b nativeBridge) postComment(c context.Context, n int, s string) (string, error) {
	return b.PostComment(c, n, s)
}
func (b nativeBridge) readComment(c context.Context, s string) (string, error) {
	return b.ReadComment(c, s)
}
func (b nativeBridge) currentLogin(c context.Context) (string, error) { return b.CurrentLogin(c) }
func (b nativeBridge) repositoryNodeID(c context.Context) (string, error) {
	return b.RepositoryIdentity(c)
}
func (b nativeBridge) canWriteProposalComments(c context.Context) (bool, error) {
	return b.CanWriteComments(c)
}

// NewNativeHostAdapter reuses the existing publication fence and exact-read
// verification without invoking GitHub or changing either provider's protocol.
func NewNativeHostAdapter(repository string, medium persist.Medium, client NativeClient, state publication.Store) (*HostAdapter, error) {
	if medium != persist.MediumForgejo || client == nil {
		return nil, errors.New("host persistence: unsupported native medium or missing client")
	}
	a, err := newHostAdapter(repository, nativeBridge{client}, state)
	if err != nil {
		return nil, err
	}
	a.medium = medium
	return a, nil
}
