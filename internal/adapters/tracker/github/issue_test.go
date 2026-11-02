package ghtracker

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestExactIssueReadIncludesOutsideUnlabelledBlocker(t *testing.T) {
	for _, state := range []string{"open", "closed"} {
		t.Run(state, func(t *testing.T) {
			calls := []string{}
			c := &ghClient{repo: "o/r", run: func(_ context.Context, args ...string) ([]byte, error) {
				assertFreshRead(t, args)
				path := args[len(args)-1]
				calls = append(calls, path)
				switch path {
				case "repos/o/r/issues/17":
					return []byte(`{"number":17,"title":"ordinary issue","state":"open"}`), nil
				case "repos/o/r/issues/17/comments?per_page=100":
					return []byte(`[{"id":3,"body":"criterion"}]` + "\n" + `[{"id":4,"body":"later comment"}]`), nil
				case "repos/o/r/issues/17/dependencies/blocked_by?per_page=100":
					return []byte(`[{"number":9,"repository_url":"https://api.github.com/repos/o/r"}]`), nil
				case "repos/o/r/issues/9":
					return []byte(`{"number":9,"state":"` + state + `"}`), nil
				default:
					t.Fatalf("unselected read %s", path)
					return nil, errors.New("unexpected")
				}
			}}
			tr := newTracker(c, "irrelevant-label")
			s, err := tr.ReadIssue(t.Context(), 17)
			if err != nil || !s.Open || !s.Executable || len(s.Comments) != 2 || len(s.Dependencies) != 1 || s.Dependencies[0].Closed != (state == "closed") || len(calls) != 4 {
				t.Fatalf("snapshot %+v calls %v error %v", s, calls, err)
			}
		})
	}
}
func TestExactIssueRefusesUnknownAndForeignFacts(t *testing.T) {
	for _, tc := range []struct {
		name, path, body string
		failure          bool
	}{
		{"wrong number", "repos/o/r/issues/17", `{"number":18,"state":"open"}`, false},
		{"unknown state", "repos/o/r/issues/17", `{"number":17,"state":"unknown"}`, false},
		{"broken JSON", "repos/o/r/issues/17", `broken`, false},
		{"missing issue", "repos/o/r/issues/17", "", true},
		{"comments denied", "repos/o/r/issues/17/comments?per_page=100", "", true},
		{"comments invalid page", "repos/o/r/issues/17/comments?per_page=100", "broken", false},
		{"missing blocker", "repos/o/r/issues/9", "", true},
		{"unreadable blocker API", "repos/o/r/issues/17/dependencies/blocked_by?per_page=100", "", true},
		{"empty blocker response", "repos/o/r/issues/17/dependencies/blocked_by?per_page=100", "", false},
		{"null blocker response", "repos/o/r/issues/17/dependencies/blocked_by?per_page=100", "null", false},
		{"foreign repository", "repos/o/r/issues/17/dependencies/blocked_by?per_page=100", `[{"number":9,"repository_url":"https://api.github.com/repos/other/r"}]`, false},
		{"foreign server", "repos/o/r/issues/17/dependencies/blocked_by?per_page=100", `[{"number":9,"repository_url":"https://foreign.test/repos/o/r"}]`, false},
		{"bad comment identity", "repos/o/r/issues/17/comments?per_page=100", `[{"body":"unidentified"}]`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &ghClient{repo: "o/r", run: func(_ context.Context, args ...string) ([]byte, error) {
				path := args[len(args)-1]
				if path == tc.path {
					if tc.failure {
						return nil, errors.New("HTTP 404")
					}
					return []byte(tc.body), nil
				}
				if strings.HasSuffix(path, "issues/17") {
					return []byte(`{"number":17,"state":"open"}`), nil
				}
				if strings.Contains(path, "/dependencies/") && tc.path == "repos/o/r/issues/9" {
					return []byte("[{\"number\":9,\"repository_url\":\"https://api.github.com/repos/o/r\"}]"), nil
				}
				return []byte(`[]`), nil
			}}
			if _, err := newTracker(c, "").ReadIssue(t.Context(), 17); err == nil {
				t.Fatal("unproved readiness accepted")
			}
		})
	}
}

func TestExactIssueRefusesNoReaderAndInvalidNumberAndSkipsNonExecutable(t *testing.T) {
	if _, err := newTracker(&fakeClient{}, "").ReadIssue(t.Context(), 17); err == nil {
		t.Fatal("ordinary frontier masqueraded as exact reader")
	}
	c := &ghClient{repo: "o/r", run: func(_ context.Context, args ...string) ([]byte, error) {
		return []byte("{\"number\":17,\"state\":\"closed\"}"), nil
	}}
	if _, err := newTracker(c, "").ReadIssue(t.Context(), 0); err == nil {
		t.Fatal("nonpositive issue read")
	}
	snapshot, err := newTracker(c, "").ReadIssue(t.Context(), 17)
	if err != nil || snapshot.Open {
		t.Fatalf("closed snapshot: %+v %v", snapshot, err)
	}
}
