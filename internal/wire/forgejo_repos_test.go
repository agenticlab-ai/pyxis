package wire

import (
	"context"
	"net/http"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
)

func TestForgejoRepositoryDiscoveryAndProbe(t *testing.T) {
	client, err := forgejo.NewWithHTTP("https://forge.example/base", "test", &http.Client{Transport: forgeRoundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet {
			t.Fatalf("discovery mutated %s", r.URL)
		}
		switch r.URL.Path {
		case "/base/api/v1/user/repos":
			return forgeResponse(r, 200, `[{"id":1,"full_name":"a/empty","empty":true,"permissions":{"push":true}},{"id":2,"full_name":"a/full","empty":false,"permissions":{"admin":true}},{"id":3,"full_name":"a/read-only","empty":true,"permissions":{"push":false}},{"id":4,"full_name":"a/archived","empty":true,"archived":true,"permissions":{"admin":true}}]`), nil
		case "/base/api/v1/repos/a/empty":
			return forgeResponse(r, 200, `{"id":1,"empty":true}`), nil
		case "/base/api/v1/repos/a/full":
			return forgeResponse(r, 200, `{"id":2,"empty":false}`), nil
		default:
			return forgeResponse(r, 404, `{}`), nil
		}
	})})
	if err != nil {
		t.Fatal(err)
	}
	listing, err := ListEmptyForgejoRepositories(context.Background(), client)
	if err != nil || len(listing.Candidates) != 1 || listing.Candidates[0].Slug != "a/empty" || listing.Candidates[0].Permission != "WRITE" {
		t.Fatalf("listing %#v err %v", listing, err)
	}
	for _, tc := range []struct {
		repo  string
		state RepoState
		fail  bool
	}{{"a/empty", RepoEmpty, false}, {"a/full", RepoPopulated, false}, {"a/missing", RepoUnknown, true}, {"invalid", RepoUnknown, true}} {
		got, err := ProbeForgejoRepository(context.Background(), client, tc.repo)
		if got != tc.state || (err != nil) != tc.fail {
			t.Fatalf("probe %s: got %v err %v", tc.repo, got, err)
		}
	}
	if _, err := ListEmptyForgejoRepositories(context.Background(), nil); err == nil {
		t.Fatal("nil listing client admitted")
	}
}

func TestForgejoRepositoryDiscoveryRejectsIncompleteAndUnavailableEvidence(t *testing.T) {
	for _, status := range []int{200, 503} {
		client, err := forgejo.NewWithHTTP("https://forge.example", "test", &http.Client{Transport: forgeRoundTrip(func(r *http.Request) (*http.Response, error) {
			body := `{}`
			if r.URL.Path == "/api/v1/user/repos" {
				body = `[{"id":1,"full_name":"a/admin","empty":true,"permissions":{"admin":true}}]`
			}
			return forgeResponse(r, status, body), nil
		})})
		if err != nil {
			t.Fatal(err)
		}
		listing, err := ListEmptyForgejoRepositories(t.Context(), client)
		if status == 503 && err == nil {
			t.Fatal("unavailable repository list silently became empty")
		}
		if status == 200 && (err != nil || len(listing.Candidates) != 1 || listing.Candidates[0].Permission != "ADMIN") {
			t.Fatalf("administrator candidate lost: %#v %v", listing, err)
		}
		if state, err := ProbeForgejoRepository(t.Context(), client, "a/unknown"); err == nil || state != RepoUnknown {
			t.Fatalf("incomplete or unavailable repository guessed: %v %v", state, err)
		}
	}
}
