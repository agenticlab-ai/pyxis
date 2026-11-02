package release

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/wire"
)

// Publish is the upload-or-verify publisher (installer-v1 §3.1; AP-22): a
// re-run of a partially-published release COMPLETES it or VERIFIES it
// byte-for-byte — never a second, different artifact set under the same
// tag. Concretely, against GitHub Releases via the gh CLI:
//
//   - no release for the tag → `gh release create --verify-tag` with the
//     whole artifact set (--verify-tag refuses a tag the remote does not
//     carry, keeping tag == commit == artifacts);
//   - release exists → per local artifact: absent remotely → `gh release
//     upload` (no --clobber, ever); present remotely → DOWNLOAD and
//     hash-compare — a byte-diverging asset is a named refusal (an
//     operator decision, not a clobber), a matching one is verified;
//   - a remote asset with no local counterpart is a named refusal too: the
//     set under a tag is closed.
//
// The transport is wire.Runner — the gh seam T10 established (AP-11: one gh
// boundary, reused, not re-grown); production passes wire.GHRunner{},
// tests a transcript fake. Publish runs Verify first (the local half), so
// nothing incoherent is ever uploaded. gh ≥ 2.x with `release download
// --output` is assumed — the same minimum-gh contract the design's open
// Q-2 pins for the wiring leg.
type PublishOptions struct {
	LegacyTool string // docgate or codegate; retains the pre-system tag channel
	Target     *integrations.ResolvedRepository
	Mirror     bool // a distribution target may create a tag at its default branch
	Tag        string
	OutDir     string
	Runner     wire.Runner
	// Repo routes all GitHub operations to "owner/name". A configured Target
	// supplies this coordinate. Canonical publication requires its real tag;
	// only an explicitly bound Mirror may create a tag at the default branch,
	// with provenance retained in the verified artifact manifest.
	Repo string
}

// PublishResult reports the per-artifact outcome.
type PublishResult struct {
	Created bool              // true when the release itself was created
	Actions map[string]string // artifact name → "uploaded" | "verified"
}

// Publish uploads-or-verifies the packed set in OutDir under Tag.
func Publish(ctx context.Context, opts PublishOptions) (PublishResult, error) {
	if opts.Target != nil && opts.Target.Provider == integrations.Forgejo {
		return publishForgejo(ctx, opts)
	}
	if opts.Target != nil {
		opts.Repo = opts.Target.Repository
		opts.Runner = githubRunner(opts.Runner, opts.Target)
	}
	if opts.Runner == nil {
		return PublishResult{}, fmt.Errorf("release: publish: no gh runner wired (programmer error)")
	}
	if err := verifyPublish(opts); err != nil {
		return PublishResult{}, fmt.Errorf("release: publish refused — the local set failed verification first: %w", err)
	}
	names, err := listArtifacts(opts.OutDir)
	if err != nil {
		return PublishResult{}, err
	}

	remote, exists, err := remoteAssets(ctx, opts)
	if err != nil {
		return PublishResult{}, err
	}
	res := PublishResult{Actions: map[string]string{}}
	if !exists {
		notes := "pyxis system release " + opts.Tag + " — the complete artifact set (binaries, payload, release manifest, SHA256SUMS); pin system.version + system.manifest_sha256 in .pyxis/config.yaml (installer-v1 §3.1, DINV-2)."
		args := []string{"release", "create", opts.Tag}
		if !opts.Mirror || opts.Target == nil || opts.LegacyTool != "" {
			args = append(args, "--verify-tag")
		} else {
			notes += " Mirrored from the canonical release on the source repo; this tag anchors at the mirror's default branch — the source commit lives in the release manifest inside the set."
		}
		args = append(args, "--title", opts.Tag, "--notes", notes)
		for _, name := range names {
			args = append(args, filepath.Join(opts.OutDir, name))
		}
		if err := ghRun(ctx, opts.Runner, releaseRepoArgs(opts, args...)); err != nil {
			return PublishResult{}, err
		}
		res.Created = true
		for _, name := range names {
			res.Actions[name] = "uploaded"
		}
		return res, nil
	}

	local := map[string]bool{}
	for _, name := range names {
		local[name] = true
	}
	var foreign []string
	for name := range remote {
		if !local[name] {
			foreign = append(foreign, name)
		}
	}
	if len(foreign) > 0 {
		sort.Strings(foreign)
		return PublishResult{}, fmt.Errorf("release: publish refused — tag %s already carries asset(s) outside this packed set: %s; the artifact set under a tag is closed (AP-22), resolve by hand", opts.Tag, strings.Join(foreign, ", "))
	}
	for _, name := range names {
		if !remote[name] {
			if err := ghRun(ctx, opts.Runner, releaseRepoArgs(opts, "release", "upload", opts.Tag, filepath.Join(opts.OutDir, name))); err != nil {
				return PublishResult{}, err
			}
			res.Actions[name] = "uploaded"
			continue
		}
		if err := verifyRemoteAsset(ctx, opts, name); err != nil {
			return PublishResult{}, err
		}
		res.Actions[name] = "verified"
	}
	return res, nil
}

// remoteAssets asks gh for the tag's asset names; exists=false when the
// release is absent (the create leg).
func remoteAssets(ctx context.Context, opts PublishOptions) (map[string]bool, bool, error) {
	stdout, stderr, code, err := opts.Runner.RunGH(ctx, releaseRepoArgs(opts, "release", "view", opts.Tag, "--json", "assets")...)
	if err != nil {
		return nil, false, fmt.Errorf("release: publish: %w", err)
	}
	if code != 0 {
		if strings.Contains(strings.ToLower(stderr), "not found") {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("release: publish: gh release view %s exited %d: %s", opts.Tag, code, strings.TrimSpace(stderr))
	}
	var view struct {
		Assets []struct {
			Name string `json:"name"`
		} `json:"assets"`
	}
	if err := json.Unmarshal([]byte(stdout), &view); err != nil {
		return nil, false, fmt.Errorf("release: publish: parse gh release view output: %w", err)
	}
	assets := map[string]bool{}
	for _, a := range view.Assets {
		assets[a.Name] = true
	}
	return assets, true, nil
}

// verifyRemoteAsset downloads one existing asset and hash-compares it
// against the local artifact — the verify leg that replaces clobbering.
func verifyRemoteAsset(ctx context.Context, opts PublishOptions, name string) error {
	tmpDir, err := os.MkdirTemp("", "pyxis-publish-*")
	if err != nil {
		return fmt.Errorf("release: publish: %w", err)
	}
	defer func() {
		// Best-effort scratch cleanup; preserve the release operation result.
		_ = os.RemoveAll(tmpDir)
	}()
	tmp := filepath.Join(tmpDir, name)
	if err := ghRun(ctx, opts.Runner, releaseRepoArgs(opts, "release", "download", opts.Tag, "--pattern", name, "--output", tmp)); err != nil {
		return err
	}
	localSum, err := hashFile(filepath.Join(opts.OutDir, name))
	if err != nil {
		return fmt.Errorf("release: publish: %w", err)
	}
	remoteSum, err := hashFile(tmp)
	if err != nil {
		return fmt.Errorf("release: publish: read downloaded %s: %w", name, err)
	}
	if localSum != remoteSum {
		return fmt.Errorf("release: publish refused — asset %s already published under %s with sha256 %s, local pack has %s; a published artifact never changes under its tag (AP-22, fail closed) — never clobbered", name, opts.Tag, remoteSum, localSum)
	}
	return nil
}

// releaseRepoArgs binds every GitHub operation to the selected repository.
// Unconfigured canonical publication retains gh's current-checkout default.
func releaseRepoArgs(opts PublishOptions, args ...string) []string {
	if opts.Repo != "" {
		return append(args, "--repo", opts.Repo)
	}
	return args
}

// ghRun executes one gh command through the seam, folding a non-zero exit
// into a named error (publish has no partial-success legs: any gh failure
// stops the run; a re-run resumes upload-or-verify from the remote state).
func ghRun(ctx context.Context, r wire.Runner, args []string) error {
	return ghRunOp(ctx, r, "publish", args)
}

// ghRunOp is ghRun with the calling verb named in the error, so a failure
// reported by pins or cut does not claim to come from publish.
func ghRunOp(ctx context.Context, r wire.Runner, op string, args []string) error {
	_, stderr, code, err := r.RunGH(ctx, args...)
	if err != nil {
		return fmt.Errorf("release: %s: %w", op, err)
	}
	if code != 0 {
		return fmt.Errorf("release: %s: gh %s exited %d: %s", op, strings.Join(args, " "), code, strings.TrimSpace(stderr))
	}
	return nil
}
