package wire

// The read-only repository listing the init interview offers as its first
// question (#1043): every EMPTY repository the operator's own credential can
// write to. It is a read, never a mutation — the declared operation table
// (Table/Execute) remains the only thing that changes anything on GitHub.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// RepoCandidate is one repository the interview may offer.
type RepoCandidate struct {
	Slug       string // owner/name
	Permission string // ADMIN | MAINTAIN | WRITE
}

// RepoListing is what one listing pass learned. Truncated matters as much as
// the candidates: the query reads a WINDOW (the 100 most recently updated),
// and empty repositories rank lowest on that axis, so "no candidates" and
// "no candidates in the window" are different answers and only one of them
// may be told to an operator as fact.
type RepoListing struct {
	Candidates []RepoCandidate
	Truncated  bool // more repositories exist beyond the window read
}

// repoListQuery lists the viewer's repositories with the three facts the
// filter needs.
//
// ownerAffiliations is NOT optional decoration: it defaults to
// [OWNER, COLLABORATOR], which silently drops every ORGANIZATION-owned
// repository — the common case for a team's repos. Without it a verified
// fixture (an org repo the viewer administers) is simply absent from the
// list, and the operator is told to create a repository that already exists.
const repoListQuery = `query {
  viewer {
    repositories(first: 100,
      affiliations: [OWNER, ORGANIZATION_MEMBER, COLLABORATOR],
      ownerAffiliations: [OWNER, ORGANIZATION_MEMBER, COLLABORATOR],
      orderBy: {field: UPDATED_AT, direction: DESC}) {
      pageInfo { hasNextPage }
      nodes { nameWithOwner isEmpty isArchived viewerPermission }
    }
  }
}`

// writablePermissions is the closed set that admits a candidate: wiring
// creates labels, sets protection, and files issues, so read access is not
// enough to offer a repository as a target.
var writablePermissions = map[string]bool{"ADMIN": true, "MAINTAIN": true, "WRITE": true}

// ListEmptyRepositories returns the empty, unarchived, writable
// repositories visible to the ambient credential in the window the query
// reads, most recently updated first, and whether that window was truncated.
//
// It fails SOFT by contract: any failure — gh absent, offline, a non-zero
// exit, output that does not parse — returns an error the caller is expected
// to treat as "no list available" and keep asking, never as a reason to
// block an install. A listing is a convenience over a question that still
// works without it.
func ListEmptyRepositories(ctx context.Context, runner Runner) (RepoListing, error) {
	if runner == nil {
		return RepoListing{}, fmt.Errorf("wire: list repositories: no gh runner")
	}
	stdout, stderr, exit, err := runner.RunGH(ctx, "api", "graphql", "-f", "query="+repoListQuery)
	if err != nil {
		return RepoListing{}, fmt.Errorf("wire: list repositories: %w", err)
	}
	if exit != 0 {
		return RepoListing{}, fmt.Errorf("wire: list repositories: gh exited %d: %s", exit, firstLine(stderr))
	}

	var payload struct {
		Data struct {
			Viewer struct {
				Repositories struct {
					PageInfo struct {
						HasNextPage bool `json:"hasNextPage"`
					} `json:"pageInfo"`
					Nodes []struct {
						NameWithOwner    string `json:"nameWithOwner"`
						IsEmpty          bool   `json:"isEmpty"`
						IsArchived       bool   `json:"isArchived"`
						ViewerPermission string `json:"viewerPermission"`
					} `json:"nodes"`
				} `json:"repositories"`
			} `json:"viewer"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		return RepoListing{}, fmt.Errorf("wire: list repositories: decode gh output: %w", err)
	}

	repos := payload.Data.Viewer.Repositories
	out := RepoListing{Truncated: repos.PageInfo.HasNextPage}
	for _, n := range repos.Nodes {
		if !n.IsEmpty || n.IsArchived || !writablePermissions[strings.ToUpper(n.ViewerPermission)] {
			continue
		}
		if !ValidCoordinate(n.NameWithOwner) {
			continue // a slug this package could not act on later
		}
		out.Candidates = append(out.Candidates, RepoCandidate{Slug: n.NameWithOwner, Permission: strings.ToUpper(n.ViewerPermission)})
	}
	return out, nil
}

// ActiveAccount reports the login the ambient gh credential authenticates
// as, or "" when that cannot be learned. It exists for one purpose: a
// repository miss reads identically whether the repository is absent or
// merely invisible to THIS credential, and on a machine with more than one
// account logged in the account's name is the whole answer.
func ActiveAccount(ctx context.Context, runner Runner) string {
	if runner == nil {
		return ""
	}
	stdout, _, exit, err := runner.RunGH(ctx, "api", "user", "--jq", ".login")
	if err != nil || exit != 0 {
		return ""
	}
	return strings.TrimSpace(stdout)
}

// RepoState is what a probe learned about one named repository.
type RepoState int

const (
	// RepoUnknown — the probe could not answer. GitHub answers 404 both for
	// a repository that does not exist and for a private one this credential
	// cannot see, so "unknown" is the honest reading of a miss.
	RepoUnknown RepoState = iota
	// RepoEmpty — it exists and carries no commits: usable as an init target.
	RepoEmpty
	// RepoPopulated — it exists and already has content.
	RepoPopulated
)

const repoStateQuery = `query($owner:String!, $name:String!) {
  repository(owner:$owner, name:$name) { isEmpty }
}`

// ProbeRepository reports whether slug exists and is empty. Like the listing
// it never blocks: an unreachable gh, a non-zero exit, or an unparseable
// answer all read as RepoUnknown, which the caller treats as "proceed as if
// it must be created" — the same posture the create-repo op already takes,
// where an already-existing repository is a benign outcome.
func ProbeRepository(ctx context.Context, runner Runner, slug string) RepoState {
	owner, name, ok := strings.Cut(slug, "/")
	if !ok || runner == nil {
		return RepoUnknown
	}
	stdout, _, exit, err := runner.RunGH(ctx, "api", "graphql",
		"-f", "query="+repoStateQuery, "-F", "owner="+owner, "-F", "name="+name)
	if err != nil || exit != 0 {
		return RepoUnknown
	}
	var payload struct {
		Data struct {
			Repository *struct {
				IsEmpty bool `json:"isEmpty"`
			} `json:"repository"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil || payload.Data.Repository == nil {
		return RepoUnknown
	}
	if payload.Data.Repository.IsEmpty {
		return RepoEmpty
	}
	return RepoPopulated
}
