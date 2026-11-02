package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/adapters/codehost/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// All fields are comparable, so the caller can prove a repeated sweep did
// not silently duplicate progress/review comments or exact-head statuses.
type forgejoPublicationState struct {
	ActorID               int64
	ActorLogin            string
	HeadSHA               string
	PRComments            int
	TaskComments          int
	LocalGateStatuses     int
	PRCommentDigest       string
	TaskCommentDigest     string
	LocalGateStatusDigest string
}

type forgejoPublicationComment struct {
	ID               int64        `json:"id"`
	Body             string       `json:"body"`
	User             forgejo.User `json:"user"`
	OriginalAuthor   string       `json:"original_author"`
	OriginalAuthorID int64        `json:"original_author_id"`
	UpdatedAt        string       `json:"updated_at"`
}

type forgejoPublicationStatus struct {
	ID          int64        `json:"id"`
	Context     string       `json:"context"`
	Status      string       `json:"status"`
	SHA         string       `json:"sha"`
	URL         string       `json:"url"`
	Creator     forgejo.User `json:"creator"`
	Description string       `json:"description"`
	TargetURL   string       `json:"target_url"`
}

// forgejoPublicationSnapshot inspects only the fixture's existing objects.
// It never authenticates as a second actor, adds feedback, or changes a status.
func forgejoPublicationSnapshot(t *testing.T, remote *forgejoRemote, pull forgejocodehost.Pull, task port.Task) forgejoPublicationState {
	t.Helper()
	actor, err := remote.Client.CurrentUser(t.Context())
	if err != nil || actor.ID <= 0 || actor.Login == "" {
		t.Fatalf("resolve real publication actor: %+v %v", actor, err)
	}
	var nativePull struct {
		Number int          `json:"number"`
		User   forgejo.User `json:"user"`
		Head   struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	if err := remote.Client.Request(t.Context(), http.MethodGet, remote.path(fmt.Sprintf("pulls/%d", pull.Number)), nil, &nativePull); err != nil {
		t.Fatal(err)
	}
	if nativePull.Number != pull.Number || nativePull.Head.SHA != pull.Head.SHA || !forgejoPublicationActor(actor, nativePull.User) {
		t.Fatalf("native proposal identity/head/author differs from authenticated actor: %+v", nativePull)
	}
	comments := func(number int) []forgejoPublicationComment {
		t.Helper()
		var rows []forgejoPublicationComment
		if err := remote.Client.List(t.Context(), remote.path(fmt.Sprintf("issues/%d/comments", number)), &rows); err != nil {
			t.Fatal(err)
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
		for i, row := range rows {
			if row.ID <= 0 || (i > 0 && row.ID == rows[i-1].ID) || !forgejoPublicationActor(actor, row.User) || row.OriginalAuthor != "" || row.OriginalAuthorID != 0 {
				t.Fatalf("owned item #%d comment %d lacks native authenticated-actor provenance", number, row.ID)
			}
		}
		return rows
	}
	prComments, taskComments := comments(pull.Number), comments(task.Number)
	var statuses []forgejoPublicationStatus
	if err := remote.Client.List(t.Context(), remote.path("commits/"+pull.Head.SHA+"/statuses"), &statuses); err != nil {
		t.Fatal(err)
	}
	var gates []forgejoPublicationStatus
	for _, status := range statuses {
		if status.Context != "pyxis/local-gate" {
			continue
		}
		// Forgejo 16 omits sha from CommitStatus. The exact-SHA request above
		// binds the response, and its native status URL repeats that revision.
		// Reject a conflicting SHA if a server supplies the optional field.
		statusURL, parseErr := url.Parse(status.URL)
		if parseErr != nil || statusURL == nil || statusURL.RawQuery != "" || statusURL.Fragment != "" ||
			!strings.HasSuffix(statusURL.Path, "/statuses/"+pull.Head.SHA) ||
			status.ID <= 0 || (status.SHA != "" && status.SHA != pull.Head.SHA) || !forgejoPublicationActor(actor, status.Creator) {
			t.Fatalf("local-gate status %d lacks exact-head authenticated-actor provenance", status.ID)
		}
		gates = append(gates, status)
	}
	sort.Slice(gates, func(i, j int) bool { return gates[i].ID < gates[j].ID })
	if len(gates) == 0 || gates[len(gates)-1].Status != "success" {
		t.Fatal("latest exact-head native local-gate status is not successful")
	}
	for i := 1; i < len(gates); i++ {
		if gates[i].ID == gates[i-1].ID {
			t.Fatal("duplicate status identity in native response")
		}
	}
	return forgejoPublicationState{
		ActorID: actor.ID, ActorLogin: actor.Login, HeadSHA: pull.Head.SHA,
		PRComments: len(prComments), TaskComments: len(taskComments), LocalGateStatuses: len(gates),
		PRCommentDigest: forgejoPublicationDigest(t, prComments), TaskCommentDigest: forgejoPublicationDigest(t, taskComments),
		LocalGateStatusDigest: forgejoPublicationDigest(t, gates),
	}
}

func forgejoPublicationActor(expected, actual forgejo.User) bool {
	return expected.ID > 0 && actual.ID == expected.ID && expected.Login != "" && strings.EqualFold(actual.Login, expected.Login)
}

func forgejoPublicationDigest(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return forgejoBriefSHA(string(data))
}

func TestForgejoPublicationActorRequiresNativeIdentity(t *testing.T) {
	actor := forgejo.User{ID: 7, Login: "fixture-actor"}
	for _, tc := range []struct {
		name string
		user forgejo.User
		want bool
	}{
		{"same", actor, true},
		{"case", forgejo.User{ID: 7, Login: "FIXTURE-ACTOR"}, true},
		{"missing ID", forgejo.User{Login: actor.Login}, false},
		{"different ID", forgejo.User{ID: 8, Login: actor.Login}, false},
		{"different login", forgejo.User{ID: 7, Login: "other"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := forgejoPublicationActor(actor, tc.user); got != tc.want {
				t.Fatalf("native actor match = %t, want %t", got, tc.want)
			}
		})
	}
}
