// Package gitcredential supplies repository-scoped Git credentials without
// persisting token values or exposing them to unrelated remotes.
package gitcredential

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/adapters/common/github/githubauth"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

type request struct{ protocol, host, path, username string }
type actorResolver func(context.Context, integrations.ResolvedRepository) (string, error)

// Serve implements Git's credential protocol. Only get can emit credentials;
// store and erase are intentional no-ops because no secret is persisted.
// Callers must send its output straight to Git, never to a logging tee.
func Serve(ctx context.Context, root, action string, input io.Reader, output io.Writer) error {
	return serve(ctx, root, action, input, output, configuredActor)
}

func serve(ctx context.Context, root, action string, input io.Reader, output io.Writer, actor actorResolver) error {
	switch action {
	case "store", "erase":
		return nil
	case "get":
	default:
		return fmt.Errorf("credential: unsupported Git credential operation")
	}
	req, err := readRequest(input)
	if err != nil {
		return err
	}
	profile, err := integrations.Load(root)
	if err != nil {
		return fmt.Errorf("credential: read integration configuration: %w", err)
	}
	if profile.Bindings.CodeHost == (integrations.Binding{}) {
		return nil
	}
	bound, err := profile.CodeHost("")
	if err != nil {
		return fmt.Errorf("credential: code-host binding: %w", err)
	}
	if !usesHelper(bound) || !matches(req, bound) {
		return nil
	}
	token := bound.Token()
	if token == "" {
		return fmt.Errorf("credential: selected code-host credential is missing: set %s", bound.TokenEnv)
	}
	if strings.ContainsAny(token, "\x00\r\n") {
		return fmt.Errorf("credential: selected token is not a valid Git protocol value")
	}
	login, err := actor(ctx, bound)
	if err != nil {
		return fmt.Errorf("credential: resolve selected code-host actor: %w", err)
	}
	if login == "" || strings.ContainsAny(login, "\x00\r\n=") {
		return fmt.Errorf("credential: selected actor is malformed")
	}
	if req.username != "" && req.username != login {
		return nil
	}
	_, err = fmt.Fprintf(output, "username=%s\npassword=%s\n\n", login, token)
	if err != nil {
		return fmt.Errorf("credential: write Git credential response: %w", err)
	}
	return nil
}

func configuredActor(ctx context.Context, bound integrations.ResolvedRepository) (string, error) {
	if bound.Provider == integrations.GitHub {
		out, err := githubauth.RunForHost(ctx, bound.TokenEnv, "github.com", nil, "api", "user", "--jq", ".login")
		return strings.TrimSpace(string(out)), err
	}
	client, err := forgejo.New(bound.BaseURL, bound.Token())
	if err != nil {
		return "", err
	}
	user, err := client.CurrentUser(ctx)
	return user.Login, err
}

func usesHelper(bound integrations.ResolvedRepository) bool {
	return bound.Provider == integrations.Forgejo || bound.Provider == integrations.GitHub && bound.TokenEnv != ""
}

func readRequest(input io.Reader) (request, error) {
	var req request
	scanner := bufio.NewScanner(io.LimitReader(input, 65537))
	scanner.Buffer(make([]byte, 4096), 65537)
	seen := make(map[string]bool)
	total := 0
	for scanner.Scan() {
		line := scanner.Text()
		total += len(line) + 1
		if total > 65536 {
			return req, fmt.Errorf("credential: Git credential request exceeds the size limit")
		}
		if line == "" {
			return req, nil
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.ContainsAny(line, "\x00\r") {
			return req, fmt.Errorf("credential: malformed Git credential request")
		}
		var target *string
		switch key {
		case "protocol":
			target = &req.protocol
		case "host":
			target = &req.host
		case "path":
			target = &req.path
		case "username":
			target = &req.username
		default:
			continue // Git extensions and supplied secrets are not consumed.
		}
		if seen[key] {
			return req, fmt.Errorf("credential: duplicate Git credential field")
		}
		seen[key] = true
		*target = value
	}
	if err := scanner.Err(); err != nil {
		return req, fmt.Errorf("credential: read Git credential request: %w", err)
	}
	return req, nil
}

func matches(req request, bound integrations.ResolvedRepository) bool {
	u, err := url.Parse(bound.BaseURL)
	if err != nil || req.protocol != u.Scheme || !strings.EqualFold(req.host, u.Host) {
		return false
	}
	requested, err := url.PathUnescape(req.path)
	if err != nil {
		return false
	}
	requested = strings.TrimPrefix(requested, "/")
	expected := strings.TrimPrefix(u.Path, "/")
	if expected != "" {
		expected += "/"
	}
	expected += bound.Repository
	return requested == expected || requested == expected+".git"
}
