package converge

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/authoritycontract"
	"github.com/agenticlab-ai/pyxis/internal/payload"
	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
)

// This is the live Source policy accidentally shipped in the verified
// v0.1.5–v0.1.8 manifests, not a desired policy or the current Source grant.
// Keep its fingerprint after those manifests leave the cache or the operand
// pins a corrected release. Historical fixture: testdata/legacy-authority.md.
const leakedSourceAuthoritySHA256 = "2a656133db3a8f91bba81d912b479c332fb98c536f45849f1806b6acf55e0fab"

// authorityHealth checks repository-owned policy even when the release does
// not list it, including retained policy after disabling Argus. It reports
// only: no missing, malformed or possibly inherited grant authorizes an
// installer/converger to change policy.
func authorityHealth(ctx context.Context, cfg resolve.Config, m relmanifest.Manifest) (Finding, bool) {
	f := Finding{Path: payload.AuthorityContractTarget, Class: relmanifest.RepoOwned, State: StateInvalid}
	if !cfg.Repo.Knobs.Argus {
		// FR-16 governs installed repositories. Source owns its policy without
		// opting into an installer knob (installer-v1 DINV-8).
		if cfg.Repo.System.Mode == "source" {
			return Finding{}, false
		}
		_, err := os.Lstat(treePath(cfg.Root, f.Path))
		if errors.Is(err, fs.ErrNotExist) {
			return Finding{}, false
		}
		f.Detail = "knobs.argus=false but an authority object remains (installer FR-16); the operator must remove retained policy or re-enable Argus intentionally — converge never deletes repository-owned authority"
		if err != nil {
			f.Detail = fmt.Sprintf("cannot inspect disabled Argus authority: %v", err)
		}
		return f, true
	}
	if err := ctx.Err(); err != nil {
		f.Detail = err.Error()
		return f, true
	}
	b, err := readAuthority(ctx, cfg.Root)
	if errors.Is(err, fs.ErrNotExist) {
		f.State = StateMissing
		f.Detail = "knobs.argus=true requires an authority contract; restore repository-owned policy from your history or have the operator instantiate it from the authority template — converge never creates policy"
		return f, true
	}
	if err != nil {
		f.Detail = fmt.Sprintf("enabled Argus authority cannot be read: %v — restore readable repository-owned policy; converge never replaces it", err)
		return f, true
	}
	contract, defects := authoritycontract.Parse("authority-health", f.Path, b)
	if len(defects) == 0 {
		defects = authoritycontract.Evaluate("authority-health", f.Path, contract)
	}
	if len(defects) != 0 {
		f.Detail = "enabled Argus authority fails closed-schema validation: " + summarizeDefects(defects) + " — the operator must correct policy; converge never rewrites it"
		return f, true
	}
	if cfg.Repo.System.Mode != "source" && sha256Hex(b) == leakedSourceAuthoritySHA256 {
		f.Detail = "authority is byte-identical to the Source policy leaked by Pyxis v0.1.5–v0.1.8, including after upgrading to a release that omits it; the operator must review and record repository-specific authority — converge preserves policy and never migrates or revokes grants"
		return f, true
	}
	for _, pf := range m.Payload.Files {
		if pf.Path == f.Path && pf.Class == relmanifest.EngineVerbatim && pf.SHA256 != nil && sha256Hex(b) == *pf.SHA256 {
			f.Detail = "authority is byte-identical to the live policy shipped by this legacy release; repository-specific grant provenance cannot be established from a payload match — the operator must review and record the intended policy in this repository; converge preserves the file and never migrates or revokes grants"
			return f, true
		}
	}
	if err := exec.CommandContext(ctx, "git", "-C", cfg.Root, "ls-files", "--error-unmatch", "--", payload.AuthorityContractTarget).Run(); err != nil {
		f.Detail = fmt.Sprintf("enabled Argus authority is not verifiably Git-tracked: %v — the operator must restore policy to repository history; converge never stages or commits delegated policy", err)
		return f, true
	}
	return Finding{}, false
}

// The authority surface must be contained, regular repository content. A
// valid external target does not make a policy link healthy or reviewable.
func readAuthority(ctx context.Context, target string) ([]byte, error) {
	root, err := os.OpenRoot(target)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }() // read-only directory handle
	parts := strings.Split(payload.AuthorityContractTarget, "/")
	for i := range parts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		rel := filepath.FromSlash(strings.Join(parts[:i+1], "/"))
		info, err := root.Lstat(rel)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 || (i < len(parts)-1 && !info.IsDir()) || (i == len(parts)-1 && !info.Mode().IsRegular()) {
			return nil, fmt.Errorf("authority component %s must be regular repository content, never a symlink", rel)
		}
	}
	return root.ReadFile(filepath.FromSlash(payload.AuthorityContractTarget))
}

// Doctor still checks presence and structure with a cold cache or in Source
// mode. Legacy-byte comparison additionally requires the verified manifest.
func (c *checks) authority(ctx context.Context, cfg resolve.Config) {
	if f, found := authorityHealth(ctx, cfg, relmanifest.Manifest{}); found {
		c.finding(CheckDrift, f.Path, string(f.State)+" — "+f.Detail)
	} else if cfg.Repo.Knobs.Argus && cfg.Repo.System.Mode != "source" {
		// A valid-looking contract is not evidence that it was not copied
		// from a legacy release. Cold/malformed-cache diagnostics must not
		// certify health when that comparison could not be performed.
		c.finding(CheckDrift, payload.AuthorityContractTarget,
			"authority provenance is unverified: the legacy release-policy comparison requires a verified manifest; doctor never fetches — run `pyxis converge` to verify the pinned release and classify policy without rewriting it")
	}
}
