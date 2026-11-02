// Package forgejopersist implements authenticated review publication through
// Forgejo's native comment API and the shared durable publication protocol.
package forgejopersist

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	api "github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	"github.com/agenticlab-ai/pyxis/internal/implreview/persist/github"
	"github.com/agenticlab-ai/pyxis/internal/implreview/publication"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

type Client struct {
	API        *api.Client
	Repository integrations.ResolvedRepository
}
type Comment struct {
	ID             int64    `json:"id"`
	Body           string   `json:"body"`
	User           api.User `json:"user"`
	OriginalAuthor string   `json:"original_author"`
}

func NewHostAdapter(repository integrations.ResolvedRepository, state publication.Store) (*github.HostAdapter, error) {
	if repository.Provider != integrations.Forgejo {
		return nil, errors.New("forgejo persistence: wrong provider")
	}
	client, err := api.New(repository.BaseURL, repository.Token())
	if err != nil {
		return nil, err
	}
	return github.NewNativeHostAdapter(repository.Key(), persist.MediumForgejo, &Client{API: client, Repository: repository}, state)
}

func (c *Client) PostComment(ctx context.Context, proposal int, body string) (string, error) {
	var comment Comment
	err := c.API.Request(ctx, http.MethodPost, api.RepoPath(c.Repository.Repository, fmt.Sprintf("issues/%d/comments", proposal)), struct {
		Body string `json:"body"`
	}{body}, &comment)
	if err != nil {
		return "", err
	}
	if comment.ID <= 0 {
		return "", errors.New("forgejo persistence: comment acknowledgement has no identity")
	}
	return strconv.FormatInt(comment.ID, 10), nil
}

func (c *Client) Read(ctx context.Context, id string) (Comment, error) {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil || n <= 0 {
		return Comment{}, errors.New("forgejo persistence: invalid comment identity")
	}
	var comment Comment
	if err := c.API.Request(ctx, http.MethodGet, api.RepoPath(c.Repository.Repository, "issues/comments/"+id), nil, &comment); err != nil {
		return Comment{}, err
	}
	if comment.ID != n {
		return Comment{}, errors.New("forgejo persistence: exact comment identity mismatch")
	}
	return comment, nil
}

func (c *Client) ReadComment(ctx context.Context, id string) (string, error) {
	comment, err := c.Read(ctx, id)
	return comment.Body, err
}
func (c *Client) CurrentLogin(ctx context.Context) (string, error) {
	u, err := c.API.CurrentUser(ctx)
	return u.Login, err
}
func (c *Client) RepositoryIdentity(ctx context.Context) (string, error) {
	var repo struct {
		ID int64 `json:"id"`
	}
	if err := c.API.Request(ctx, http.MethodGet, api.RepoPath(c.Repository.Repository, ""), nil, &repo); err != nil {
		return "", err
	}
	if repo.ID <= 0 {
		return "", errors.New("forgejo persistence: missing repository identity")
	}
	return base64.RawURLEncoding.EncodeToString([]byte(c.Repository.BaseURL + "/repositories/" + strconv.FormatInt(repo.ID, 10))), nil
}
func (c *Client) CanWriteComments(ctx context.Context) (bool, error) {
	var repo struct {
		Permissions struct {
			Admin bool `json:"admin"`
			Push  bool `json:"push"`
			Pull  bool `json:"pull"`
		} `json:"permissions"`
	}
	if err := c.API.Request(ctx, http.MethodGet, api.RepoPath(c.Repository.Repository, ""), nil, &repo); err != nil {
		return false, err
	}
	return repo.Permissions.Admin || repo.Permissions.Push, nil
}
