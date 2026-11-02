package conformancetest

import (
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
)

func TestMemoryStoreConsumerContract(t *testing.T) {
	RunStore(t, func(*testing.T) StoreFixture {
		store := featuredrive.NewMemoryStore()
		created := featuredrive.Transition{Kind: "create", NewVersion: 1}
		return StoreFixture{Store: store,
			Initial: featuredrive.FeatureDrive{Schema: featuredrive.Schema, Repository: "repository", FeatureID: "feature", PlanFingerprint: "plan", TargetMainRef: "refs/heads/main", IntegrationRef: "refs/heads/feature/feature", Version: 1, Created: created, Last: created, Ownership: featuredrive.Ownership{State: featuredrive.Unowned}, Phase: featuredrive.Integrating},
			Owner:   featuredrive.Owner{Domain: "local", ID: "machine-1"},
			Reopen:  func(*testing.T) featuredrive.FeatureDriveStore { return store },
		}
	})
}
