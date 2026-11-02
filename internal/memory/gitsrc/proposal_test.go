package gitsrc

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestProposalIgnoreChecksEveryExactDestination(t *testing.T) {
	t.Parallel()
	paths := []string{".pyxis/logs/archive/a\nquoted", ".pyxis/logs/archive/b"}
	for _, tc := range []struct {
		name string
		out  string
		err  error
		bad  bool
	}{
		{"all ignored", strings.Join(paths, "\x00") + "\x00", nil, false},
		{"partial match", paths[0] + "\x00", nil, true},
		{"failed read", "", context.Canceled, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			r := &Repo{run: func(_ context.Context, in []byte, args ...string) ([]byte, error) {
				calls++
				if string(in) != strings.Join(paths, "\x00")+"\x00" || !reflect.DeepEqual(args, []string{"--no-literal-pathspecs", "check-ignore", "--stdin", "-z"}) {
					t.Fatal("ignore check lost literal filename boundaries", args, string(in))
				}
				return []byte(tc.out), tc.err
			}}
			if err := r.RequireIgnored(t.Context(), nil); err != nil || calls != 0 {
				t.Fatal("empty selection invoked Git", err)
			}
			if err := r.RequireIgnored(t.Context(), paths); (err != nil) != tc.bad || calls != 1 {
				t.Fatal("ignore verdict", err, calls)
			}
		})
	}
}

func TestProposalArchiveMustBeIgnoredAndUntrackedInRealGit(t *testing.T) {
	t.Parallel()
	dir := newRepo(t)
	write(t, dir, ".gitignore", ".pyxis/logs/\n")
	git(t, dir, "add", ".gitignore")
	git(t, dir, "commit", "-qm", "ignore diagnostics")
	r, err := Open(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	ignored := ".pyxis/logs/archive/a\nquoted"
	if err := r.RequireIgnored(t.Context(), []string{ignored}); err != nil {
		t.Fatal("ignored archive refused", err)
	}
	if err := r.RequireIgnored(t.Context(), []string{ignored, "published.json"}); err == nil {
		t.Fatal("partial ignored batch admitted")
	}
	write(t, dir, ignored, "previously published")
	git(t, dir, "add", "-f", "--", ignored)
	if err := r.RequireIgnored(t.Context(), []string{ignored}); err == nil {
		t.Fatal("tracked archive admitted")
	}
}

func TestProposalIndexReadDoesNotRefreshOrFilterWorkingBytes(t *testing.T) {
	t.Parallel()
	dir := newRepo(t)
	r, err := Open(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	write(t, dir, "memory/policy.json", "changed index bytes")
	git(t, dir, "add", "memory/policy.json")
	write(t, dir, "memory/policy.json", `{"schema":1}`)
	paths, err := r.IndexChangedPaths(t.Context(), "HEAD", "memory")
	if err != nil || !reflect.DeepEqual(paths, []string{"memory/policy.json"}) {
		t.Fatal("staged change hidden by clean working bytes", paths, err)
	}
	r.run = func(context.Context, []byte, ...string) ([]byte, error) { return nil, context.Canceled }
	if _, err := r.IndexChangedPaths(t.Context(), "HEAD", "memory"); !errors.Is(err, context.Canceled) {
		t.Fatal("index read failed open", err)
	}
}
