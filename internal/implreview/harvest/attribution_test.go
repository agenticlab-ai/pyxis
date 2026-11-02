package harvest_test

import (
	"context"
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/harvest"
)

// TestGitHubResolverMatchesOperatorArtifact — the happy path of the anti-forgery
// binding: an attribution whose referenced artifact's author IS the operator
// resolves true. The author comes from the injected code-host lookup, not from any
// payload field.
func TestGitHubResolverMatchesOperatorArtifact(t *testing.T) {
	t.Parallel()
	authors := newFakeAuthors()
	authors.byRef["github:pr-comment/500"] = operatorLogin

	r := harvest.NewGitHubResolver(operatorLogin, authors)
	ok, err := r.IsOperator(context.Background(), "github:pr-comment/500")
	if err != nil {
		t.Fatalf("IsOperator: %v", err)
	}
	if !ok {
		t.Fatal("an operator-authored artifact must resolve to the operator")
	}
}

// TestGitHubResolverRejectsForeignAuthor — an attribution whose artifact author is
// someone OTHER than the operator resolves false. This is the ABUSE-5 forgery leg
// at the resolver level.
func TestGitHubResolverRejectsForeignAuthor(t *testing.T) {
	t.Parallel()
	authors := newFakeAuthors()
	authors.byRef["github:pr-comment/501"] = "agenticlab-factory-bot"

	r := harvest.NewGitHubResolver(operatorLogin, authors)
	ok, err := r.IsOperator(context.Background(), "github:pr-comment/501")
	if err != nil {
		t.Fatalf("IsOperator: %v", err)
	}
	if ok {
		t.Fatal("a non-operator-authored artifact must NOT resolve to the operator (ABUSE-5)")
	}
}

// TestGitHubResolverCaseInsensitiveAndAtTolerant — GitHub logins are
// case-insensitive and the operator identity may be written "@mdarabi"; the
// resolver normalizes both sides so "@MDarabi" configured against a host-reported
// "mdarabi" still matches.
func TestGitHubResolverCaseInsensitiveAndAtTolerant(t *testing.T) {
	t.Parallel()
	authors := newFakeAuthors()
	authors.byRef["ref"] = "mdarabi"

	r := harvest.NewGitHubResolver("@MDarabi", authors)
	ok, err := r.IsOperator(context.Background(), "ref")
	if err != nil {
		t.Fatalf("IsOperator: %v", err)
	}
	if !ok {
		t.Fatal("login comparison must be case-insensitive and @-tolerant")
	}
}

// TestGitHubResolverEmptyAttributionIsNotOperator — an empty attribution (a worker
// inline disposition carries none) is not the operator's artifact, and is not an
// error — a legitimate "not attributable" answer.
func TestGitHubResolverEmptyAttributionIsNotOperator(t *testing.T) {
	t.Parallel()
	r := harvest.NewGitHubResolver(operatorLogin, newFakeAuthors())
	ok, err := r.IsOperator(context.Background(), "")
	if err != nil {
		t.Fatalf("empty attribution should not be an error: %v", err)
	}
	if ok {
		t.Fatal("an empty attribution must not resolve to the operator")
	}
}

// TestGitHubResolverDanglingReferenceIsUnresolved — an attribution referencing an
// artifact the code host does not have (a dangling reference) resolves false with
// ErrUnresolvedAttribution — never silently the operator.
func TestGitHubResolverDanglingReferenceIsUnresolved(t *testing.T) {
	t.Parallel()
	r := harvest.NewGitHubResolver(operatorLogin, newFakeAuthors()) // empty lookup

	ok, err := r.IsOperator(context.Background(), "github:pr-review/does-not-exist")
	if ok {
		t.Fatal("a dangling attribution must not resolve to the operator")
	}
	if !errors.Is(err, harvest.ErrUnresolvedAttribution) {
		t.Fatalf("dangling attribution error=%v, want ErrUnresolvedAttribution", err)
	}
}

// TestGitHubResolverTransportFailureIsError — a lookup transport failure surfaces
// as an error (the harvest then EXCLUDES the disposition), so a lookup outage never
// silently counts a disposition as the operator.
func TestGitHubResolverTransportFailureIsError(t *testing.T) {
	t.Parallel()
	authors := newFakeAuthors()
	boom := errors.New("github api 503")
	authors.failRef["ref"] = boom

	r := harvest.NewGitHubResolver(operatorLogin, authors)
	ok, err := r.IsOperator(context.Background(), "ref")
	if ok {
		t.Fatal("a transport failure must not resolve to the operator")
	}
	if !errors.Is(err, boom) {
		t.Fatalf("transport error=%v, want it to wrap the lookup failure", err)
	}
}

// TestGitHubResolverEmptyOperatorFailsClosed — an unset operator identity cannot
// attribute anything to "the operator": the resolver fails closed (error, false)
// rather than spuriously match.
func TestGitHubResolverEmptyOperatorFailsClosed(t *testing.T) {
	t.Parallel()
	authors := newFakeAuthors()
	authors.byRef["ref"] = "anyone"

	r := harvest.NewGitHubResolver("", authors)
	ok, err := r.IsOperator(context.Background(), "ref")
	if ok {
		t.Fatal("an unset operator identity must not attribute a disposition to the operator")
	}
	if err == nil {
		t.Fatal("an unset operator identity must fail closed with an error")
	}
}

// TestGitHubResolverNilClientFailsClosed — a resolver with no author-lookup client
// fails closed rather than panic or silently match.
func TestGitHubResolverNilClientFailsClosed(t *testing.T) {
	t.Parallel()
	r := harvest.NewGitHubResolver(operatorLogin, nil)
	ok, err := r.IsOperator(context.Background(), "ref")
	if ok || err == nil {
		t.Fatalf("nil client must fail closed: ok=%v err=%v", ok, err)
	}
}

// TestForgedForcesExclusionThroughOperatorResolverFunc — the harvest works against
// ANY OperatorResolver, not just the GitHub one: a table-driven OperatorResolverFunc
// exercising accepted/excluded shows the exclusion is a property of the resolver's
// answer, not of the GitHub adapter specifically (the seam is honestly injectable).
func TestForgedForcesExclusionThroughOperatorResolverFunc(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		resolveOperator bool
		resolveErr      error
		wantDisposed    int
		wantAccepted    int
		wantExcluded    int
	}{
		{name: "attributable operator accepts", resolveOperator: true, wantDisposed: 1, wantAccepted: 1},
		{name: "forged is excluded", resolveOperator: false, wantExcluded: 1},
		{name: "lookup failure excludes", resolveErr: errors.New("boom"), wantExcluded: 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			port := newFakePort()
			r := reviewWith("github:owner/repo#1", "rv-1",
				blockingFinding("FND-1", "accepted", "some/ref"))
			universe := []harvest.Proposal{{Ref: "github:owner/repo#1", Review: refPtr(port.put(t, r))}}

			resolver := harvest.OperatorResolverFunc(func(context.Context, string) (bool, error) {
				return tc.resolveOperator, tc.resolveErr
			})
			m, err := harvest.Harvest(context.Background(),
				harvest.EnumeratorFunc(func(context.Context) ([]harvest.Proposal, error) { return universe, nil }),
				port, resolver)
			if err != nil {
				t.Fatalf("harvest: %v", err)
			}
			if m.BlockingDisposed != tc.wantDisposed || m.BlockingAccepted != tc.wantAccepted || m.ExcludedDispositions != tc.wantExcluded {
				t.Fatalf("disposed=%d accepted=%d excluded=%d, want %d/%d/%d",
					m.BlockingDisposed, m.BlockingAccepted, m.ExcludedDispositions,
					tc.wantDisposed, tc.wantAccepted, tc.wantExcluded)
			}
		})
	}
}
