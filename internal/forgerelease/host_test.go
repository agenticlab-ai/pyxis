package forgerelease

import (
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

type stubHost struct{ Host }

func TestNewRequiresBoundAdapter(t *testing.T) {
	t.Cleanup(func() { Bind(nil) })
	Bind(nil)
	if _, err := New(integrations.ResolvedRepository{}); err == nil || err.Error() != "forgejo release adapter is not wired" {
		t.Fatalf("unwired New: %v", err)
	}
}

func TestBindWiresNew(t *testing.T) {
	t.Cleanup(func() { Bind(nil) })
	want := &stubHost{}
	Bind(func(integrations.ResolvedRepository) (Host, error) { return want, nil })
	got, err := New(integrations.ResolvedRepository{Repository: "o/r"})
	if err != nil || got != want {
		t.Fatalf("wired New: got=%T err=%v", got, err)
	}
	Bind(func(integrations.ResolvedRepository) (Host, error) { return nil, errors.New("refused") })
	if _, err := New(integrations.ResolvedRepository{}); err == nil || err.Error() != "refused" {
		t.Fatalf("bound error: %v", err)
	}
}
