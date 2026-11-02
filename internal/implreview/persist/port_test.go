package persist

import "testing"

func TestGitHubProposalRef(t *testing.T) {
	got, err := GitHubProposalRef(" agenticlab-ai/pyxis ", 1034)
	if err != nil {
		t.Fatalf("GitHubProposalRef: %v", err)
	}
	if got != "github:agenticlab-ai/pyxis#1034" {
		t.Fatalf("proposal ref = %q", got)
	}
	for _, repo := range []string{"", "owner", "owner/", "/repo", "too/many/parts"} {
		if _, err := GitHubProposalRef(repo, 1); err == nil {
			t.Fatalf("GitHubProposalRef accepted repo %q", repo)
		}
	}
	if _, err := GitHubProposalRef("owner/repo", 0); err == nil {
		t.Fatal("GitHubProposalRef accepted proposal zero")
	}
}
