package release_test

import (
	"context"
	"fmt"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/release"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// Assert the configured repository on every operation before replaying the
// existing upload-or-verify transcript.
type bindingGH struct {
	inner      *fakeGH
	repository string
}

func (b *bindingGH) RunGH(ctx context.Context, args ...string) (string, string, int, error) {
	if len(args) < 2 || args[len(args)-2] != "--repo" || args[len(args)-1] != b.repository {
		return "", "", 0, fmt.Errorf("release call lost bound repository: %v", args)
	}
	return b.inner.RunGH(ctx, args[:len(args)-2]...)
}

func TestBoundPublishCreatesAbsentRelease(t *testing.T) {
	out, res := packFixture(t, fixtureRepo(t))
	gh := &fakeGH{t: t, steps: []ghStep{
		viewStep("", "release not found", 1),
		{check: func(t *testing.T, args []string) {
			t.Helper()
			if args[0] != "release" || args[1] != "create" || args[2] != fixtureVersion {
				t.Fatalf("gh args = %v, want a release create", args)
			}
			if !slices.Contains(args, "--verify-tag") {
				t.Fatalf("create without --verify-tag: %v", args)
			}
			for _, name := range res.Artifacts {
				if !slices.Contains(args, filepath.Join(out, name)) {
					t.Fatalf("create is missing artifact %s: %v", name, args)
				}
			}
		}},
	}}
	pres, err := release.Publish(context.Background(), release.PublishOptions{Tag: fixtureVersion, OutDir: out, Runner: &bindingGH{inner: gh, repository: "owner/bound"}, Target: &integrations.ResolvedRepository{Provider: integrations.GitHub, Repository: "owner/bound"}})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	gh.done()
	if !pres.Created || len(pres.Actions) != len(res.Artifacts) {
		t.Fatalf("result = %+v, want created with every artifact uploaded", pres)
	}
}

func TestBoundPublishUploadOrVerifyConverges(t *testing.T) {
	out, res := packFixture(t, fixtureRepo(t))
	existing := "payload-" + fixtureVersion + ".tar.gz"
	existingBytes, err := os.ReadFile(filepath.Join(out, existing))
	if err != nil {
		t.Fatal(err)
	}
	steps := []ghStep{viewStep(assetsJSON(existing), "", 0)}
	for _, name := range res.Artifacts { // sorted walk order
		if name == existing {
			steps = append(steps, downloadStep(name, existingBytes))
			continue
		}
		steps = append(steps, uploadStep(fixtureVersion, out, name))
	}
	gh := &fakeGH{t: t, steps: steps}
	pres, err := release.Publish(context.Background(), release.PublishOptions{Tag: fixtureVersion, OutDir: out, Runner: &bindingGH{inner: gh, repository: "owner/bound"}, Target: &integrations.ResolvedRepository{Provider: integrations.GitHub, Repository: "owner/bound"}})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	gh.done()
	if pres.Created || pres.Actions[existing] != "verified" {
		t.Fatalf("result = %+v, want the existing asset verified, not re-created", pres)
	}
	uploaded := 0
	for _, action := range pres.Actions {
		if action == "uploaded" {
			uploaded++
		}
	}
	if uploaded != len(res.Artifacts)-1 {
		t.Fatalf("uploaded %d, want %d", uploaded, len(res.Artifacts)-1)
	}
}
