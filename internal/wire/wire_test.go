package wire

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/payload"
	"github.com/agenticlab-ai/pyxis/internal/plangate/dialect"
)

const testSlug = "octo/newrepo"

// knobsConfig builds a wire.Config carrying exactly the given knob values —
// the table-derivation operand for every combination test.
func knobsConfig(tracker, ciMode, repo, protection string) Config {
	c := config.Defaults()
	c.Knobs = config.Knobs{
		Tracker: tracker,
		CIMode:  ciMode,
		Wiring:  config.Wiring{Repo: repo, Protection: protection},
	}
	return Config{
		Repo:         c,
		Slug:         testSlug,
		WelcomeTitle: "Welcome to your pyxis factory",
		WelcomeBody:  "First steps live in AGENTS.md.\nAuthor your vision, then run /feature-brainstorm.",
	}
}

// knobSpace enumerates the full closed knob space the table derives from.
func knobSpace() (trackers, ciModes, repos, protections []string) {
	return []string{"github", "carina"},
		[]string{"local", "actions", "both"},
		[]string{"use", "create"},
		[]string{"none", "required-checks", "checks+review"}
}

// wantOps is the test's INDEPENDENT oracle for the shape's conditions —
// deliberately re-stated rather than shared with Table, in the table's
// dependency order.
func wantOps(tracker, ciMode, repo, protection string) []string {
	var want []string
	if repo == "create" {
		want = append(want, OpCreateRepo)
	}
	if tracker == "github" {
		want = append(want, OpEnsureLabels)
	}
	if ciMode == "actions" || ciMode == "both" {
		want = append(want, OpPushWorkflow)
	}
	if protection == "required-checks" || protection == "checks+review" {
		want = append(want, OpSetProtection)
	}
	if tracker == "github" {
		want = append(want, OpWelcomeItem)
	}
	return want
}

func opNames(ops []Op) []string {
	names := make([]string, 0, len(ops))
	for _, o := range ops {
		names = append(names, o.Name)
	}
	return names
}

func TestTableKnobCombinations(t *testing.T) {
	trackers, ciModes, repos, protections := knobSpace()
	combos := 0
	for _, tr := range trackers {
		for _, ci := range ciModes {
			for _, rp := range repos {
				for _, pr := range protections {
					combos++
					ops := Table(knobsConfig(tr, ci, rp, pr))
					got := opNames(ops)
					want := wantOps(tr, ci, rp, pr)
					if !slices.Equal(got, want) {
						t.Errorf("tracker=%s ci_mode=%s repo=%s protection=%s: table %v, want %v", tr, ci, rp, pr, got, want)
					}
				}
			}
		}
	}
	if combos != 36 {
		t.Fatalf("combination sweep covered %d combos, want 36", combos)
	}
}

func TestTableZeroValueKnobsDeclareNothing(t *testing.T) {
	// Zero values mean "never captured" (T2): the conditions match
	// positively over the closed enums, so protection "" is NOT "≠ none".
	cfg := Config{Repo: config.Defaults(), Slug: testSlug}
	if ops := Table(cfg); len(ops) != 0 {
		t.Fatalf("zero-value knobs declared %v, want an empty table", opNames(ops))
	}
}

func TestTableConditionsAndTransports(t *testing.T) {
	ops := Table(knobsConfig("github", "both", "create", "checks+review"))
	if len(ops) != 5 {
		t.Fatalf("full knob set declared %d ops, want 5", len(ops))
	}
	wantCond := map[string]string{
		OpCreateRepo:    "wiring.repo=create",
		OpEnsureLabels:  "tracker=github",
		OpPushWorkflow:  "ci_mode in {actions, both}",
		OpSetProtection: "wiring.protection != none",
		OpWelcomeItem:   "tracker=github",
	}
	for _, op := range ops {
		if op.Condition != wantCond[op.Name] {
			t.Errorf("%s: condition %q, want %q", op.Name, op.Condition, wantCond[op.Name])
		}
		wantTransport := TransportGH
		if op.Name == OpPushWorkflow {
			wantTransport = TransportConverge
		}
		if op.Transport != wantTransport {
			t.Errorf("%s: transport %q, want %q", op.Name, op.Transport, wantTransport)
		}
		if op.Transport == TransportGH && op.Slug != testSlug {
			t.Errorf("%s: slug %q, want %q", op.Name, op.Slug, testSlug)
		}
	}
}

func TestTableWelcomeBodyCarriesMarker(t *testing.T) {
	find := func(ops []Op) Op {
		for _, o := range ops {
			if o.Name == OpWelcomeItem {
				return o
			}
		}
		t.Fatal("welcome-item not declared")
		return Op{}
	}

	cfg := knobsConfig("github", "local", "use", "none")
	w := find(Table(cfg))
	if !strings.HasSuffix(w.Body, "\n\n"+WelcomeMarker) {
		t.Errorf("marker not appended to a marker-less body:\n%s", w.Body)
	}
	if !strings.Contains(w.Body, cfg.WelcomeBody) {
		t.Errorf("original body content lost:\n%s", w.Body)
	}

	cfg.WelcomeBody = "already keyed\n" + WelcomeMarker + "\ntail"
	if w := find(Table(cfg)); w.Body != cfg.WelcomeBody {
		t.Errorf("body with marker was modified:\n%s", w.Body)
	}

	cfg.WelcomeBody = ""
	if w := find(Table(cfg)); w.Body != WelcomeMarker {
		t.Errorf("empty body: got %q, want the bare marker", w.Body)
	}
}

func TestTableBranchDefault(t *testing.T) {
	cfg := knobsConfig("carina", "local", "use", "required-checks")
	cfg.Repo.Branch = ""
	ops := Table(cfg)
	if len(ops) != 1 || ops[0].Name != OpSetProtection {
		t.Fatalf("table %v, want exactly set-protection", opNames(ops))
	}
	// The blank-branch fallback resolves the config schema's single source, not a
	// baked "main" (config-driven-portability lift, DINV-1) — byte-identical to
	// the old literal on a default config.
	if want := config.Defaults().Branch; ops[0].Branch != want {
		t.Errorf("empty branch: got %q, want the config default %q", ops[0].Branch, want)
	}
	if config.Defaults().Branch != "main" {
		t.Fatalf("default branch is no longer byte-identical to the old literal (%q)", config.Defaults().Branch)
	}

	cfg.Repo.Branch = "trunk"
	op := Table(cfg)[0]
	if op.Branch != "trunk" {
		t.Errorf("captured branch: got %q, want trunk", op.Branch)
	}
	if want := "repos/" + testSlug + "/branches/trunk/protection"; !slices.Contains(op.protectionArgv(), want) {
		t.Errorf("protection argv %v does not target %q", op.protectionArgv(), want)
	}
}

func TestProtectionArgvLevels(t *testing.T) {
	base := []string{
		"api", "--method", "PUT",
		"-H", "Accept: application/vnd.github+json",
		"repos/octo/newrepo/branches/main/protection",
		"-F", "required_status_checks[strict]=false",
		"-f", "required_status_checks[contexts][]=docs-gate",
		"-F", "enforce_admins=false",
	}
	tests := []struct {
		protection string
		tail       []string
	}{
		{"required-checks", []string{"-F", "required_pull_request_reviews=null", "-F", "restrictions=null"}},
		{"checks+review", []string{"-F", "required_pull_request_reviews[required_approving_review_count]=1", "-F", "restrictions=null"}},
	}
	for _, tc := range tests {
		ops := Table(knobsConfig("carina", "local", "use", tc.protection))
		got := ops[0].protectionArgv()
		want := append(append([]string{}, base...), tc.tail...)
		if !slices.Equal(got, want) {
			t.Errorf("%s:\n got %q\nwant %q", tc.protection, got, want)
		}
	}
}

func TestRoutingLabelsAreTheDialectSet(t *testing.T) {
	// FR-10's "routing labels" are the plan dialect's canonical vocabulary
	// — reused, never re-declared (AP-10).
	if !reflect.DeepEqual(RoutingLabels(), dialect.Harbormaster().Labels()) {
		t.Fatalf("wire's label set diverged from dialect.Harbormaster().Labels()")
	}
	if n := len(RoutingLabels()); n == 0 {
		t.Fatalf("empty routing-label set (%d labels)", n)
	}
}

func TestPreviewRendersEveryOpFromTheTable(t *testing.T) {
	ops := Table(knobsConfig("github", "both", "create", "checks+review"))
	entries := Preview(ops)
	if len(entries) != len(ops) {
		t.Fatalf("preview rendered %d entries for %d ops", len(entries), len(ops))
	}
	for i, op := range ops {
		e := entries[i]
		if !strings.Contains(e, op.Name) || !strings.Contains(e, op.Condition) {
			t.Errorf("entry %d misses name/condition:\n%s", i, e)
		}
		switch op.Transport {
		case TransportConverge:
			if strings.Contains(e, "gh ") {
				t.Errorf("converge-transport entry renders a gh command:\n%s", e)
			}
			if !strings.Contains(e, "converge write set") {
				t.Errorf("converge-transport entry misses the carried-by note:\n%s", e)
			}
		case TransportGH:
			if p := op.probeArgv(); p != nil && !strings.Contains(e, "probe: "+shellJoin(p)) {
				t.Errorf("entry %d misses its probe line:\n%s", i, e)
			}
			for _, c := range op.ensureCommands() {
				if !strings.Contains(e, shellJoin(c)) {
					t.Errorf("entry %d misses command %q:\n%s", i, shellJoin(c), e)
				}
			}
		}
	}
	// The label entry renders the blind-run-safe ensure form.
	for _, e := range entries {
		if strings.Contains(e, "label create") && !strings.Contains(e, "--force") {
			t.Errorf("label ensure command is not paste-safe (--force missing):\n%s", e)
		}
	}
}

func TestShellQuote(t *testing.T) {
	tests := []struct{ in, want string }{
		{"abc", "abc"},
		{"a/b-c_d.e:f,g=h@i%j+k", "a/b-c_d.e:f,g=h@i%j+k"},
		{"", "''"},
		{"a b", "'a b'"},
		{"don't", `'don'\''t'`},
		{"line1\nline2", "'line1\nline2'"},
		{"required_status_checks[strict]=false", "'required_status_checks[strict]=false'"},
		{"$HOME", "'$HOME'"},
		{"a;rm -rf", "'a;rm -rf'"},
	}
	for _, tc := range tests {
		if got := shellQuote(tc.in); got != tc.want {
			t.Errorf("shellQuote(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	if got := shellJoin([]string{"label", "create", "kind:feat", "--description", "Factory: feature work"}); got != "gh label create kind:feat --description 'Factory: feature work'" {
		t.Errorf("shellJoin rendered %q", got)
	}
}

// TestRequiredCheckContextMatchesInstalledWorkflow pins RequiredCheckContext
// to the payload's installed-repo workflow variant: the required-status
// context GitHub reports for an Actions job is the job's id (no name:
// override is used there), so the constant and the shipped job id must
// never drift.
func TestRequiredCheckContextMatchesInstalledWorkflow(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(payload.InstalledWorkflow)))
	if err != nil {
		t.Fatalf("read the installed workflow variant: %v", err)
	}
	m := regexp.MustCompile(`(?m)^jobs:\n  ([A-Za-z0-9_-]+):`).FindSubmatch(b)
	if m == nil {
		t.Fatalf("no jobs: block found in %s", payload.InstalledWorkflow)
	}
	if got := string(m[1]); got != RequiredCheckContext {
		t.Fatalf("installed workflow job id %q != RequiredCheckContext %q", got, RequiredCheckContext)
	}
	if regexp.MustCompile(`(?m)^    name:`).Match(b) {
		t.Fatalf("the installed workflow job overrides name: — the required context would no longer be the job id")
	}
}
