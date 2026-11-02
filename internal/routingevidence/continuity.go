package routingevidence

// RefCommit is the bounded, transport-neutral commit view required for
// continuity verification. Object and tree are exact git object identities.
type RefCommit struct {
	Object  string   `json:"object"`
	Parents []string `json:"parents"`
	Tree    string   `json:"tree"`
}

// RefView is produced by an exact fetch of the configured evidence ref. The
// adapter records destructive remote observations instead of hiding them
// behind the ref's current value.
type RefView struct {
	Commits                     []RefCommit `json:"commits"`
	DeletionObserved            bool        `json:"deletion_observed"`
	EvidenceRef                 string      `json:"evidence_ref"`
	Exists                      bool        `json:"exists"`
	Head                        string      `json:"head"`
	InitializationAuthorization string      `json:"initialization_authorization"`
	NonFastForwardObserved      bool        `json:"non_fast_forward_observed"`
	Repository                  string      `json:"repository"`
}

// VerifyContinuity proves an unbroken, linear history from the exact
// operator-authorized genesis object to the fetched head. Any destructive
// observation poisons the view even if the ref was later recreated.
func VerifyContinuity(view RefView, checkpoint GenesisCheckpoint) error {
	if checkpoint.Repository == "" || checkpoint.EvidenceRef == "" || checkpoint.Commit == "" || checkpoint.Tree == "" || checkpoint.AuthorizationReference == "" {
		return integrity(IntegrityContinuity, "checkpoint is incomplete")
	}
	if view.Repository != checkpoint.Repository || view.EvidenceRef != checkpoint.EvidenceRef {
		return integrity(IntegrityContinuity, "fetched repository or ref differs from checkpoint")
	}
	if !view.Exists || view.Head == "" || view.DeletionObserved {
		return integrity(IntegrityContinuity, "evidence ref is missing or was deleted")
	}
	if view.NonFastForwardObserved {
		return integrity(IntegrityContinuity, "non-fast-forward update was observed")
	}
	if view.InitializationAuthorization != checkpoint.AuthorizationReference {
		return integrity(IntegrityContinuity, "genesis initialization is unauthorized")
	}
	commits := make(map[string]RefCommit, len(view.Commits))
	for _, commit := range view.Commits {
		if commit.Object == "" || commit.Tree == "" {
			return integrity(IntegrityContinuity, "commit view is incomplete")
		}
		if _, duplicate := commits[commit.Object]; duplicate {
			return integrity(IntegrityContinuity, "duplicate commit object %q", commit.Object)
		}
		commits[commit.Object] = commit
	}
	genesis, ok := commits[checkpoint.Commit]
	if !ok || genesis.Tree != checkpoint.Tree || len(genesis.Parents) != 0 {
		return integrity(IntegrityContinuity, "genesis object is missing, changed, or not a root")
	}
	seen := map[string]bool{}
	for object := view.Head; ; {
		if seen[object] {
			return integrity(IntegrityContinuity, "commit ancestry contains a cycle at %q", object)
		}
		seen[object] = true
		commit, exists := commits[object]
		if !exists {
			return integrity(IntegrityContinuity, "commit ancestry is missing %q", object)
		}
		if object == checkpoint.Commit {
			return nil
		}
		if len(commit.Parents) != 1 || commit.Parents[0] == "" {
			return integrity(IntegrityContinuity, "commit %q does not have one continuous parent", object)
		}
		object = commit.Parents[0]
	}
}
