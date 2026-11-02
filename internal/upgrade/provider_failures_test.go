package upgrade

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

func TestForgejoUpgradeRefusesInvalidOrUnavailableSource(t *testing.T) {
	bad := integrations.ResolvedRepository{Provider: integrations.Forgejo, BaseURL: "invalid", Repository: "o/r"}
	if _, err := discover(t.Context(), Options{Release: &bad}); err == nil {
		t.Fatal("invalid discovery source accepted")
	}
	if _, err := fetchArtifact(t.Context(), Options{Release: &bad}, "v1.2.3", "asset"); err == nil {
		t.Fatal("invalid artifact source accepted")
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) }))
	defer s.Close()
	bad.BaseURL = s.URL
	if _, err := discover(t.Context(), Options{Release: &bad}); err == nil {
		t.Fatal("denied discovery accepted")
	}
	if _, err := fetchArtifact(t.Context(), Options{Release: &bad}, "v1.2.3", "asset"); err == nil {
		t.Fatal("denied artifact accepted")
	}
	if _, err := Run(t.Context(), Options{Release: &bad, Channel: "https://mirror.example"}); err == nil {
		t.Fatal("ambiguous repository and channel accepted")
	}
}
