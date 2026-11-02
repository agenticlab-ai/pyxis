package harvest

import (
	"context"
	"fmt"
	"strings"
)

// OperatorResolver answers the FR-16/DINV-4 question the harvest turns on: does
// this operator disposition's `attribution` resolve to the OPERATOR'S OWN
// medium-authored artifact? An operator-class disposition counts toward the
// corpus ONLY when this returns true; a forged one (a worker-planted "operator
// accepted" whose disposition-artifact author is not the operator) returns false
// and is excluded (§3.1, A-5, ABUSE-5).
//
// attribution is the opaque value the report.Disposition carries — for the
// GitHub path it references the operator's own PR review/comment (see
// [GitHubResolver]). The resolver looks up that artifact's medium AUTHOR and
// compares it to the operator identity; it NEVER trusts a payload string as the
// author (that is the exact forgery this closes — the attribution is resolved
// against the artifact's own author, not read as an author claim).
//
// It is an injected interface so the harvest is hermetic: a test injects a fake
// author-lookup and no network is touched.
type OperatorResolver interface {
	// IsOperator reports whether attribution resolves to an artifact whose medium
	// author IS the operator identity. An empty/dangling attribution is not the
	// operator (false, no error is required — a dangling reference is a legitimate
	// "not attributable" answer); a lookup transport failure is an error, which
	// the harvest treats as "not attributable" (excluded) so a lookup outage never
	// silently counts a disposition as the operator.
	IsOperator(ctx context.Context, attribution string) (bool, error)
}

// OperatorResolverFunc adapts a plain function to [OperatorResolver] — the
// ergonomic form a test uses.
type OperatorResolverFunc func(ctx context.Context, attribution string) (bool, error)

// IsOperator implements [OperatorResolver].
func (f OperatorResolverFunc) IsOperator(ctx context.Context, attribution string) (bool, error) {
	return f(ctx, attribution)
}

// AuthorResolver is the narrow, consumer-side code-host surface the GitHub
// attribution binding needs: given the reference an operator disposition's
// `attribution` carries (a PR review id or comment id), return the LOGIN of the
// artifact's medium author — the value GitHub natively attributes to the
// review/comment's `user` (§3.2's "the review/comment's `user` IS the operator,
// natively attributable"). One method, one exact code-host call, so a test
// injects a fake and the binding logic is hermetic — no network (the ghtracker /
// persist-github pattern).
//
// It returns (login, true) when the referenced artifact exists and carries an
// author, and (_, false) when the reference resolves to no artifact (a dangling
// reference). A transport failure is an error.
type AuthorResolver interface {
	// AuthorOf resolves the medium author (login) of the artifact the attribution
	// reference addresses. It is an idempotent READ; it never mutates the medium.
	AuthorOf(ctx context.Context, attribution string) (login string, found bool, err error)
}

// GitHubResolver is the v1 (GitHub-path) [OperatorResolver] — the anti-forgery
// binding the design puts on the v1 critical path (§3.2, the harder attribution
// mechanism specified here, not deferred). An operator disposition is valid iff
// the operator's OWN PR review/comment (the artifact the `attribution` references)
// has the operator's login as its `user`. The resolver looks that author up
// through the injected [AuthorResolver] and compares it to the configured
// operator identity, case-insensitively (GitHub logins are case-insensitive).
//
// A disposition whose artifact author is anyone OTHER than the operator — the
// forged "operator accepted" a worker plants (ABUSE-5) — resolves to false and
// is excluded. The mechanism (the operator's own review/comment) is what this
// implements and tests; the squash-merge survival of that reference is a Q3
// INTEGRATION detail verified against the real API later (§3.2, plan T11
// out-of-scope), not a mechanism this changes.
type GitHubResolver struct {
	// operator is the operator identity a disposition-artifact author must match
	// to attribute the disposition to the operator (a GitHub login).
	operator string
	// client resolves an attribution reference to its artifact's medium author.
	client AuthorResolver
}

// compile-time proof GitHubResolver satisfies the resolver seam.
var _ OperatorResolver = (*GitHubResolver)(nil)

// NewGitHubResolver builds the GitHub-path resolver binding an operator identity
// to the injected author-lookup client. The operator login is normalized (a
// leading "@" trimmed, surrounding space trimmed) so "@mdarabi" and "mdarabi"
// compare equal.
func NewGitHubResolver(operator string, client AuthorResolver) *GitHubResolver {
	return &GitHubResolver{operator: normalizeLogin(operator), client: client}
}

// IsOperator implements [OperatorResolver] for the GitHub path: it resolves the
// attribution reference to its artifact's author and reports whether that author
// IS the operator. A dangling reference (no such artifact) is not the operator; a
// lookup transport failure is returned so the harvest can exclude the disposition
// rather than silently count it (never a silent trust — FR-16).
func (g *GitHubResolver) IsOperator(ctx context.Context, attribution string) (bool, error) {
	if g.client == nil {
		return false, fmt.Errorf("github attribution: nil AuthorResolver")
	}
	// An empty attribution is not the operator's artifact — a worker inline
	// disposition carries no attribution, and an operator disposition MUST
	// reference a real artifact (DINV-4). Not an error: a legitimate "not
	// attributable" answer.
	if strings.TrimSpace(attribution) == "" {
		return false, nil
	}
	if strings.TrimSpace(g.operator) == "" {
		// No operator identity to compare against: fail closed — nothing can be
		// attributed to an unset operator, so the disposition is excluded, never
		// spuriously counted.
		return false, fmt.Errorf("github attribution: empty operator identity")
	}

	author, found, err := g.client.AuthorOf(ctx, attribution)
	if err != nil {
		return false, fmt.Errorf("github attribution: resolve author of %q: %w", attribution, err)
	}
	if !found {
		// A dangling reference resolves to no artifact — not the operator, and not
		// an error the harvest must abort on. It is simply not attributable
		// (excluded), wrapped so a caller CAN branch on it if it wants.
		return false, fmt.Errorf("%w: %q", ErrUnresolvedAttribution, attribution)
	}
	// The artifact's OWN author is compared to the operator identity — the payload
	// never supplies the author, so a worker cannot forge it (DINV-4). Match is
	// case-insensitive (GitHub logins are).
	return normalizeLogin(author) == g.operator, nil
}

// normalizeLogin lowercases and strips a leading "@" and surrounding space from a
// login so identity comparison is case-insensitive and "@"-tolerant (GitHub
// logins are case-insensitive; the operator identity may be written "@mdarabi").
func normalizeLogin(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "@")
	return strings.ToLower(strings.TrimSpace(s))
}
