package verdict

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

func TestMostRestrictiveIsOrderIndependentAndClosed(t *testing.T) {
	for _, verdicts := range [][]string{
		{report.VerdictApprove, report.VerdictRequestChanges, report.VerdictApproveWithAdvisories},
		{report.VerdictApproveWithAdvisories, report.VerdictApprove, report.VerdictRequestChanges},
	} {
		got, err := MostRestrictive(verdicts)
		if err != nil || got != report.VerdictRequestChanges {
			t.Fatalf("MostRestrictive(%v) = %q, %v", verdicts, got, err)
		}
	}
	if _, err := MostRestrictive([]string{"favorable"}); err == nil {
		t.Fatal("unknown verdict was accepted")
	}
}

// blocking builds a blocking finding, optionally disposed.
func blocking(id string, d *report.Disposition) report.Finding {
	return report.Finding{ID: id, Message: "m", CalRow: "CAL-1", Lens: report.LensSpecFidelity, Severity: report.SeverityBlocking, Disposition: d}
}

// advisory builds an advisory finding.
func advisory(id string) report.Finding {
	return report.Finding{ID: id, Message: "m", CalRow: "CAL-2", Lens: report.LensSpecFidelity, Severity: report.SeverityAdvisory}
}

func round(findings ...report.Finding) report.Round {
	return report.Round{ReviewedRevision: "sha-latest", Findings: findings}
}

// TestComputeVerdictFromFindings exercises the FR-7 mapping: blocking-and-
// unresolved → request-changes, advisory-only → approve-with-advisories, none →
// approve — plus the operator-resolved-away cases that stop a blocker holding.
func TestComputeVerdictFromFindings(t *testing.T) {
	opAccepted := &report.Disposition{Actor: report.ActorOperator, Verdict: report.VerdictAccepted}
	opRejected := &report.Disposition{Actor: report.ActorOperator, Verdict: report.VerdictRejected}
	workerApplied := &report.Disposition{Actor: report.ActorWorker, Verdict: report.VerdictApplied}
	workerDeclined := &report.Disposition{Actor: report.ActorWorker, Verdict: report.VerdictDeclined, Reason: "restates a guard"}

	cases := []struct {
		name     string
		rounds   []report.Round
		wantV    string
		wantAuth Authority
		wantDown bool // Outcome.Downgrades()
	}{
		{
			name:     "no findings → approve, no downgrade",
			rounds:   []report.Round{round()},
			wantV:    report.VerdictApprove,
			wantAuth: AuthorityNone,
			wantDown: false,
		},
		{
			name:     "advisory only → approve-with-advisories, no downgrade",
			rounds:   []report.Round{round(advisory("FND-1"))},
			wantV:    report.VerdictApproveWithAdvisories,
			wantAuth: AuthorityNone,
			wantDown: false,
		},
		{
			name:     "unresolved blocking → request-changes, return-to-worker",
			rounds:   []report.Round{round(blocking("FND-1", nil))},
			wantV:    report.VerdictRequestChanges,
			wantAuth: AuthorityReturnToWorker,
			wantDown: true,
		},
		{
			name:     "blocking with worker-applied (unverified) still holds → request-changes",
			rounds:   []report.Round{round(blocking("FND-1", workerApplied))},
			wantV:    report.VerdictRequestChanges,
			wantAuth: AuthorityReturnToWorker,
			wantDown: true,
		},
		{
			name:     "blocking with worker-declined still holds (escalate is caller-signalled, not here) → request-changes",
			rounds:   []report.Round{round(blocking("FND-1", workerDeclined))},
			wantV:    report.VerdictRequestChanges,
			wantAuth: AuthorityReturnToWorker,
			wantDown: true,
		},
		{
			name:     "blocking accepted-away by operator → approve",
			rounds:   []report.Round{round(blocking("FND-1", opAccepted))},
			wantV:    report.VerdictApprove,
			wantAuth: AuthorityNone,
			wantDown: false,
		},
		{
			name:     "blocking rejected-as-nit by operator → approve",
			rounds:   []report.Round{round(blocking("FND-1", opRejected))},
			wantV:    report.VerdictApprove,
			wantAuth: AuthorityNone,
			wantDown: false,
		},
		{
			name:     "operator-accepted blocker + a live advisory → approve-with-advisories",
			rounds:   []report.Round{round(blocking("FND-1", opAccepted), advisory("FND-2"))},
			wantV:    report.VerdictApproveWithAdvisories,
			wantAuth: AuthorityNone,
			wantDown: false,
		},
		{
			name: "verdict reads the LATEST round: an earlier blocker, latest clean → approve",
			rounds: []report.Round{
				{ReviewedRevision: "sha-1", Findings: []report.Finding{blocking("FND-1", nil)}},
				{ReviewedRevision: "sha-2", Findings: nil},
			},
			wantV:    report.VerdictApprove,
			wantAuth: AuthorityNone,
			wantDown: false,
		},
		{
			name: "verdict reads the LATEST round: latest carries a fresh blocker → request-changes",
			rounds: []report.Round{
				{ReviewedRevision: "sha-1", Findings: nil},
				{ReviewedRevision: "sha-2", Findings: []report.Finding{blocking("FND-2", nil)}},
			},
			wantV:    report.VerdictRequestChanges,
			wantAuth: AuthorityReturnToWorker,
			wantDown: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Compute(tc.rounds, Signals{})
			if err != nil {
				t.Fatalf("Compute: unexpected error: %v", err)
			}
			if got.Verdict != tc.wantV {
				t.Errorf("verdict = %q, want %q", got.Verdict, tc.wantV)
			}
			if got.Authority != tc.wantAuth {
				t.Errorf("authority = %q, want %q", got.Authority, tc.wantAuth)
			}
			if got.Downgrades() != tc.wantDown {
				t.Errorf("Downgrades() = %v, want %v", got.Downgrades(), tc.wantDown)
			}
			// A non-escalate outcome never carries a brief.
			if got.Verdict != report.VerdictEscalate && got.Brief != nil {
				t.Errorf("non-escalate outcome carries a brief: %+v", got.Brief)
			}
			// Authority is derived from the verdict and can never disagree.
			if got.Authority != authorityOf(got.Verdict) {
				t.Errorf("authority %q disagrees with verdict %q's authority %q", got.Authority, got.Verdict, authorityOf(got.Verdict))
			}
		})
	}
}

// TestComputeNoRounds — a verdict cannot be computed for a review with no rounds.
func TestComputeNoRounds(t *testing.T) {
	_, err := Compute(nil, Signals{})
	if !errors.Is(err, ErrNoRounds) {
		t.Fatalf("Compute(nil rounds): err = %v, want ErrNoRounds", err)
	}
}

// TestForcingHumanWorksAndAutoIsUnrepresentable is the FR-7 heart: forcing HUMAN
// (and return-to-worker) is REPRESENTABLE, and granting AUTO is NOT — there is
// no verdict value, no Authority member, and no method that widens lane
// eligibility.
func TestForcingHumanWorksAndAutoIsUnrepresentable(t *testing.T) {
	// (1) Forcing HUMAN works: an escalate verdict carries force-human authority
	// and downgrades to HUMAN via the emitted signal.
	esc, err := Compute([]report.Round{round(blocking("FND-1", nil))}, Signals{
		Escalation: &Escalation{Brief: report.EscalationBrief{
			Conflict:       "the charter is ambiguous about the divergent behavior",
			Options:        []string{"ratify behavior A", "ratify behavior B"},
			DecisionNeeded: "which behavior the charter intends",
		}},
	})
	if err != nil {
		t.Fatalf("Compute escalate: %v", err)
	}
	if esc.Authority != AuthorityForceHuman {
		t.Fatalf("escalate authority = %q, want force-human", esc.Authority)
	}
	if !esc.Authority.Downgrades() {
		t.Fatalf("force-human must Downgrade()")
	}

	// Return-to-worker is representable too.
	rc, _ := Compute([]report.Round{round(blocking("FND-1", nil))}, Signals{})
	if rc.Authority != AuthorityReturnToWorker || !rc.Authority.Downgrades() {
		t.Fatalf("request-changes must carry a downgrading return-to-worker authority; got %q", rc.Authority)
	}

	// (2) AUTO-granting is unrepresentable BY DATA: no Authority value the
	// package can produce grants AUTO — every producible authority either
	// downgrades or is None (which withholds a downgrade, it does not grant).
	for _, a := range []Authority{AuthorityNone, AuthorityReturnToWorker, AuthorityForceHuman} {
		// The only three authorities that exist. None of them can be a grant:
		// Downgrades() partitions them into {downgrade} vs {none}, with no third
		// "grant" bucket. This asserts the partition is total and grant-free.
		if a == AuthorityNone && a.Downgrades() {
			t.Errorf("AuthorityNone must not downgrade")
		}
		if a != AuthorityNone && !a.Downgrades() {
			t.Errorf("a non-None authority %q must downgrade — there is no non-downgrading, non-None authority", a)
		}
	}

	// (3) AUTO-granting is unrepresentable BY CONSTRUCTION (source-level guard):
	// the package declares NO identifier granting AUTO or signing a stage-9 gate.
	// This is the "make AUTO-granting unrepresentable" DoD, enforced so a future
	// edit that adds such a member fails the test.
	assertNoAutoGrantingIdentifier(t)
}

// assertNoAutoGrantingIdentifier parses this package's own source and fails if
// any exported identifier's name suggests granting AUTO or writing a stage-9
// sign-off — a structural guard that the downgrade-only guarantee is not
// silently widened by a later edit (FR-7). It also asserts the Authority enum's
// only string constants are the three downgrade-only values.
func assertNoAutoGrantingIdentifier(t *testing.T) {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return strings.HasSuffix(fi.Name(), ".go") && !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse package source: %v", err)
	}

	// Names that would betray an AUTO grant or a stage-9 signature capability.
	banned := []string{"grantauto", "autogrant", "grantsauto", "signgate", "gatesignature", "signstage9", "stage9sign", "widenlane", "widensauto"}

	var authConstValues []string
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				switch node := n.(type) {
				case *ast.FuncDecl:
					checkBanned(t, node.Name.Name, banned)
				case *ast.TypeSpec:
					checkBanned(t, node.Name.Name, banned)
				case *ast.ValueSpec:
					for i, name := range node.Names {
						checkBanned(t, name.Name, banned)
						// Collect the string values of Authority-typed constants.
						if isAuthorityType(node.Type) {
							if i < len(node.Values) {
								if lit, ok := node.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
									authConstValues = append(authConstValues, strings.Trim(lit.Value, `"`))
								}
							}
						}
					}
				}
				return true
			})
		}
	}

	// The Authority enum's declared string values must be EXACTLY the three
	// downgrade-only ones — no AUTO-granting fourth member can exist.
	want := map[string]bool{"": true, "return-to-worker": true, "force-human": true}
	for _, v := range authConstValues {
		if !want[v] {
			t.Errorf("Authority carries an unexpected value %q — only the three downgrade-only values are allowed (no AUTO grant)", v)
		}
	}
	if len(authConstValues) != 3 {
		t.Errorf("found %d Authority constant values %v, want exactly the 3 downgrade-only ones", len(authConstValues), authConstValues)
	}
}

func checkBanned(t *testing.T, name string, banned []string) {
	t.Helper()
	low := strings.ToLower(name)
	for _, b := range banned {
		if strings.Contains(low, b) {
			t.Errorf("identifier %q suggests an AUTO grant / stage-9 sign-off — the verdict is downgrade-only and must not name one (FR-7)", name)
		}
	}
}

func isAuthorityType(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "Authority"
}

// TestSetEnvelopeStampsVerdictAndBrief — SetEnvelope stamps the computed verdict
// onto a report and satisfies report.Validate's brief-iff-escalate rule.
func TestSetEnvelopeStampsVerdictAndBrief(t *testing.T) {
	cal := report.CalLookupFunc(func(row string) (string, bool) {
		switch row {
		case "CAL-1":
			return report.SeverityBlocking, true
		case "CAL-2":
			return report.SeverityAdvisory, true
		}
		return "", false
	})

	t.Run("non-escalate: verdict set, no brief, validates", func(t *testing.T) {
		r := baseReport(round(advisory("FND-1")))
		o, err := Compute(r.Rounds, Signals{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := SetEnvelope(r, o); err != nil {
			t.Fatalf("SetEnvelope: %v", err)
		}
		if r.Verdict != report.VerdictApproveWithAdvisories {
			t.Errorf("envelope verdict = %q, want approve-with-advisories", r.Verdict)
		}
		if r.EscalationBrief != nil {
			t.Errorf("non-escalate report carries a brief: %+v", r.EscalationBrief)
		}
		if err := report.Validate(r, cal); err != nil {
			t.Errorf("report.Validate on a stamped non-escalate report: %v", err)
		}
	})

	t.Run("escalate: verdict set, brief present, validates", func(t *testing.T) {
		r := baseReport(round(blocking("FND-1", nil)))
		o, err := Compute(r.Rounds, Signals{Escalation: &Escalation{Brief: report.EscalationBrief{
			Conflict:       "spec ambiguity",
			Options:        []string{"A", "B"},
			DecisionNeeded: "pick",
		}}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := SetEnvelope(r, o); err != nil {
			t.Fatalf("SetEnvelope: %v", err)
		}
		if r.Verdict != report.VerdictEscalate {
			t.Errorf("envelope verdict = %q, want escalate", r.Verdict)
		}
		if r.EscalationBrief == nil {
			t.Fatalf("escalate report carries no brief")
		}
		if err := report.Validate(r, cal); err != nil {
			t.Errorf("report.Validate on a stamped escalate report: %v", err)
		}
	})

	t.Run("nil report is refused", func(t *testing.T) {
		if _, err := SetEnvelope(nil, Outcome{Verdict: report.VerdictApprove}); err == nil {
			t.Errorf("SetEnvelope(nil) must error")
		}
	})
}

func baseReport(rounds ...report.Round) *report.Report {
	return &report.Report{
		SchemaVersion: report.SchemaVersion,
		ProposalRef:   "github:agenticlab-ai/pyxis#412",
		ReviewID:      "rv-1",
		Band:          report.BandStandard,
		Rounds:        rounds,
	}
}
