package persist

import "testing"

func TestProposalRefPreservesProviderServerPortAndBasePath(t *testing.T) {
	for _, repository := range []string{"forgejo:http://127.0.0.1:3000:owner/repo", "forgejo:https://forgejo.test/base:owner/repo", "forgejo:https://other.test/base:owner/repo", "forgejo:http://[::1]:3000/base:owner/repo"} {
		got, err := ProposalRef(repository, 7)
		if err != nil || got != repository+"#7" {
			t.Fatalf("%s -> %s, %v", repository, got, err)
		}
	}
	if got, err := ProposalRef("owner/repo", 7); err != nil || got != "github:owner/repo#7" {
		t.Fatalf("legacy=%s %v", got, err)
	}
	for _, repository := range []string{"forgejo:missing", "forgejo::owner/repo", "forgejo:file:///tmp:owner/repo", "forgejo:https://user:password@forgejo.test:owner/repo", "forgejo:https://forgejo.test?query:owner/repo", "forgejo:https://forgejo.test#fragment:owner/repo", "forgejo:https://%zz:owner/repo", "forgejo:https://forgejo.test:owner"} {
		if _, err := ProposalRef(repository, 7); err == nil {
			t.Fatalf("invalid repository accepted: %s", repository)
		}
	}
	if _, err := ProposalRef("forgejo:https://forgejo.test:owner/repo", 0); err == nil {
		t.Fatal("invalid proposal accepted")
	}
}
