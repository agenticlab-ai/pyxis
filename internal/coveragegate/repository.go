package coveragegate

// repositoryPlan splits this repository's slowest test packages, slowest
// first, when one package's sequential tests would otherwise bound the suite's
// wall time. Parts only schedule work, never the verdict, so a stale entry
// costs time rather than correctness.
//
// It is empty since the tests of local code-host mode were disabled. Measured
// on 2026-10-17 on an idle Mac, one invocation took 250 s and 1,442 CPU-s, the
// previous 15-package split 251 s and 1,669 CPU-s, and a 4-package split 263 s
// and 1,614 CPU-s: each part repeats its package's setup, and on the shared
// runner the extra CPU costs more than the parallelism saves.
var repositoryPlan = Plan{}

// repositoryExcluded leaves code out of the coverage verdict, by package or by
// file. The operator disabled the tests of the localgit code host, both its
// own and those elsewhere that drive local code-host mode (see localgittest).
// The other entries are code that only those tests exercised; other tests
// still run some of it.
//
// round.go and prepare.go also hold hosted code: their local branches sit
// inside functions both modes share, so the whole file is left out.
//
// confine_linux.go and landlock_linux.go bind Landlock to the kernel. They run
// only in the re-executed confinement child, where coverage is not collected;
// the runner package's Landlock protocol tests and process-boundary cover them.
var repositoryExcluded = []string{
	"github.com/agenticlab-ai/pyxis/internal/verify/runner/confine_linux.go",
	"github.com/agenticlab-ai/pyxis/internal/verify/runner/landlock_linux.go",
	"github.com/agenticlab-ai/pyxis/internal/adapters/codehost/localgit",
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewgate/conformancetest",
	"github.com/agenticlab-ai/pyxis/internal/featuredrive/candidatereview/conformancetest",
	"github.com/agenticlab-ai/pyxis/internal/featuredrive/conformancetest/proposals.go",
	"github.com/agenticlab-ai/pyxis/internal/adapters/featuredrive/localgit/proposals.go",
	"github.com/agenticlab-ai/pyxis/internal/adapters/featuredrive/localgit/delivery.go",
	"github.com/agenticlab-ai/pyxis/internal/adapters/draft/shell/local.go",
	"github.com/agenticlab-ai/pyxis/internal/draft/lifecycle/proposals.go",
	"github.com/agenticlab-ai/pyxis/internal/draft/lifecycle/proposal_tasks.go",
	"github.com/agenticlab-ai/pyxis/internal/draft/lock/local.go",
	"github.com/agenticlab-ai/pyxis/internal/draft/lock/local_prepared.go",
	"github.com/agenticlab-ai/pyxis/internal/draft/lock/prepare.go",
	"github.com/agenticlab-ai/pyxis/internal/draft/lock/round.go",
	"github.com/agenticlab-ai/pyxis/internal/plangate/precheck/local.go",
	"github.com/agenticlab-ai/pyxis/cmd/plangate/local_lock.go",
	"github.com/agenticlab-ai/pyxis/cmd/pyxis/draft_local.go",
	"github.com/agenticlab-ai/pyxis/cmd/pyxis/draft_local_lock.go",
	"github.com/agenticlab-ai/pyxis/cmd/pyxis/draft_review.go",
}
