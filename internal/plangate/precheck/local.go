package precheck

import (
	"context"
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/draft/lifecycle"
	"github.com/agenticlab-ai/pyxis/internal/draft/lock"
	"github.com/agenticlab-ai/pyxis/internal/draft/readiness"
)

// RunWithLocalLock keeps the selected origin and its complete lock verifier
// explicit. A local record is never decoded as an authenticated hosted lock.
func RunWithLocalLock(ctx context.Context, root, feature, origin string, run lifecycle.RunCommand, reader codehost.DraftLockReader) (string, []schema.Defect) {
	if origin == "" || reader == nil {
		return "", []schema.Defect{def(feature, "precheck", "local planning requires a verified origin and lock reader", "select the local code host through the CLI")}
	}
	judge := func(ctx context.Context, root string, cfg config.Config, feature string, version int, mainline string, run lifecycle.RunCommand) (readiness.Verdict, error) {
		verified, err := reader.VerifyDraftLock(ctx, feature, version)
		if err != nil {
			return readiness.Verdict{}, err
		}
		seed, err := lock.LoadLocalContract(ctx, root, feature, version, reader)
		if err != nil {
			return readiness.Verdict{}, err
		}
		if seed.Digest != verified.Review.Receipt.Binding.Digest {
			return readiness.Verdict{}, fmt.Errorf("local binding lock changed during verification")
		}
		verdict, err := judgeContractAt(ctx, root, cfg, feature, version, mainline, run, seed, verified.MergeCommit, verified.MergeCommit)
		if err != nil || !verdict.Ready {
			return verdict, err
		}
		// The plan fold resolves Basis through the checkout's tag. Import that
		// exact tag without forcing a conflicting cache, then reverify its
		// origin record and peeled landing before reporting a usable Basis.
		ref := fmt.Sprintf("refs/tags/lock/%s/v%d", feature, version)
		if _, err := run(ctx, root, nil, "git", "fetch", "--no-tags", "--no-write-fetch-head", origin, ref+":"+ref); err != nil {
			return readiness.Verdict{}, fmt.Errorf("fetch verified planning Basis %s without replacing a conflicting local tag: %w", ref, err)
		}
		commit, err := run(ctx, root, nil, "git", "rev-parse", "--verify", ref+"^{commit}")
		if err != nil {
			return readiness.Verdict{}, err
		}
		current, err := reader.VerifyDraftLock(ctx, feature, version)
		if err != nil {
			return readiness.Verdict{}, err
		}
		if current != verified || strings.TrimSpace(string(commit)) != verified.MergeCommit {
			return readiness.Verdict{}, fmt.Errorf("local planning Basis changed during lock verification")
		}
		return verdict, nil
	}
	return entry(ctx, root, feature, run, origin, judge)
}
