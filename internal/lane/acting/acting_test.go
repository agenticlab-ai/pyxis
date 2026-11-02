package acting

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/lane/classifier"
	"github.com/agenticlab-ai/pyxis/internal/lane/policy"
	"github.com/agenticlab-ai/pyxis/internal/lane/verdict"
)

// --- fakes: the local-substitutable seams (merge-lane-v1 §3.8, no injected mocks) ---

// fakeMedium is the read-only medium seam. Every read is data-configured; an
// injected error exercises the infrastructure-error path.
type fakeMedium struct {
	head      string
	headErr   error
	green     bool
	greenErr  error
	review    verdict.CheckState
	reviewErr error
	merged    bool
	mergedErr error
}

func (m *fakeMedium) Head(_ context.Context, _ string) (string, error) { return m.head, m.headErr }
func (m *fakeMedium) AllRequiredGreen(_ context.Context, _, _ string) (bool, error) {
	return m.green, m.greenErr
}
func (m *fakeMedium) ReviewVerdict(_ context.Context, _, _ string) (verdict.CheckState, error) {
	return m.review, m.reviewErr
}
func (m *fakeMedium) MergedIntoMainline(_ context.Context, _, _ string) (bool, error) {
	return m.merged, m.mergedErr
}

// fakeRecorder captures every recorded verdict per proposal.
type fakeRecorder struct{ recs map[string][]verdict.Record }

func newRecorder() *fakeRecorder { return &fakeRecorder{recs: map[string][]verdict.Record{}} }
func (r *fakeRecorder) RecordVerdict(_ context.Context, proposal string, rec verdict.Record) error {
	r.recs[proposal] = append(r.recs[proposal], rec)
	return nil
}

// fakeMachine is the credentialed acting seam. `credential` is PRIVATE — a
// drive-side probe must never reach it (DINV-5, A-8); the tests assert it never
// escapes into a Result.
type fakeMachine struct {
	id         Identity
	credential string
	acts       int
	gotHead    string
	actErr     error
}

func (m *fakeMachine) Identity() Identity { return m.id }
func (m *fakeMachine) Act(_ context.Context, _, head string) error {
	m.acts++
	m.gotHead = head
	return m.actErr
}

// noActMachine fails the test the instant Act is called — the shadow
// zero-side-effect sentinel (A-5).
type noActMachine struct{ t *testing.T }

func (m noActMachine) Identity() Identity { return machineID }
func (m noActMachine) Act(_ context.Context, proposal, _ string) error {
	m.t.Fatalf("Act called for %q in a non-acting state — shadow must have zero acting side effects (A-5)", proposal)
	return nil
}

// --- shared fixtures ---

var (
	machineID = Identity{Name: "pyxis-lane-bot", Machine: true}
	humanID   = Identity{Name: "mdarabi", Machine: false}

	fleetTask = policy.AutoClass{Name: "fleet-task", Paths: []string{"internal/carina/**", "cmd/carina/**"}}
	driveDoc  = policy.AutoClass{Name: "drive-doc", Paths: []string{"docs/wiki/**"}, RequiresReviewerVerdict: true}
)

// policyFor builds a well-formed policy with the self-unlock minimum floor and
// the given activation + AUTO classes. Floor is always the design minimum so
// the classifier's floor arm behaves exactly as in production.
func policyFor(enforce bool, backstop string, kill bool, classes ...policy.AutoClass) *policy.Policy {
	return &policy.Policy{
		SchemaVersion: policy.SchemaVersion,
		RatifiedBy:    "@mdarabi",
		Floor: policy.Floor{
			HardGatedPaths:   policy.SelfUnlockMinPaths,
			HardGatedClasses: policy.SelfUnlockMinClasses,
		},
		AutoClasses: classes,
		Activation:  policy.Activation{Enforce: enforce, BackstopChangeRef: backstop},
		KillSwitch:  kill,
	}
}

// autoEvent is a floor-clear proposal whose whole diff falls inside fleetTask.
func autoEvent(proposal string) Event {
	return Event{
		Proposal:      proposal,
		Diff:          classifier.Diff{Paths: []string{"internal/carina/foo.go"}},
		Base:          "base-sha",
		Head:          "head-sha",
		CheckSnapshot: "snap-1",
	}
}

// docEvent is a floor-clear proposal inside driveDoc (reviewer-required).
func docEvent(proposal string) Event {
	e := autoEvent(proposal)
	e.Diff = classifier.Diff{Paths: []string{"docs/wiki/topic.md"}}
	return e
}

// humanEvent floor-hits (a control-plane path) → HUMAN.
func humanEvent(proposal string) Event {
	e := autoEvent(proposal)
	e.Diff = classifier.Diff{Paths: []string{".github/workflows/ci.yml"}}
	return e
}

// fullActivation is the medium state under which an AUTO fleet-task act
// completes: head unmoved, checks green, merged into mainline.
func fullMedium() *fakeMedium {
	return &fakeMedium{head: "head-sha", green: true, merged: true}
}

// refusalMentions reports whether any refusal's reason contains sub.
func refusalMentions(res Result, sub string) bool {
	for _, d := range res.Refusals {
		if strings.Contains(d.Reason, sub) {
			return true
		}
	}
	return false
}

// --- A-3: the fail-closed activation gate ---

// With activation absent (enforce:false) no AUTO fixture produces an act, and
// the refusal names the missing act.
func TestActivationAbsent_NoActRefusalNamesMissingAct(t *testing.T) {
	rec := newRecorder()
	mach := &fakeMachine{id: machineID, credential: "s3cr3t"}
	a := &Adapter{Medium: fullMedium(), Recorder: rec, Machine: mach}

	res, err := a.Handle(context.Background(), autoEvent("p1"), policyFor(false, "", false, fleetTask), Preconditions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.State != Shadow {
		t.Fatalf("state = %q, want shadow", res.State)
	}
	if res.Acted || mach.acts != 0 {
		t.Fatalf("an act occurred with activation absent (acted=%v, machine acts=%d)", res.Acted, mach.acts)
	}
	if res.Verdict.Lane != classifier.Auto {
		t.Fatalf("fixture should classify AUTO, got %q", res.Verdict.Lane)
	}
	if !refusalMentions(res, "not activated") {
		t.Fatalf("refusal does not name the missing activation act: %+v", res.Refusals)
	}
}

// A half-present activation resolves shadow PLUS a named activation-incomplete
// defect — and it surfaces at EVERY state read (A-3), so Resolve is called
// repeatedly and must name it each time.
func TestActivationHalfPresent_NamedDefectAtEveryRead(t *testing.T) {
	cases := []struct {
		name      string
		backstop  string
		refProt   bool
		wantNames []string
	}{
		{"backstop absent, ref-protection present", "", true, []string{"org-backstop"}},
		{"backstop present, ref-protection absent", "refs/backstop@1", false, []string{"ref-protection"}},
		{"both absent", "", false, []string{"org-backstop", "ref-protection"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := policyFor(true, tc.backstop, false, fleetTask)
			pc := Preconditions{RefProtectionPresent: tc.refProt}
			// Every state read names the defect(s) — three reads, identical result.
			for read := 0; read < 3; read++ {
				state, defects := Resolve(p, pc)
				if state != Shadow {
					t.Fatalf("read %d: state = %q, want shadow (half-present activation never enforces)", read, state)
				}
				if len(defects) != len(tc.wantNames) {
					t.Fatalf("read %d: got %d defects, want %d: %+v", read, len(defects), len(tc.wantNames), defects)
				}
				joined := ""
				for _, d := range defects {
					joined += d.Reason + "\n"
				}
				for _, want := range tc.wantNames {
					if !strings.Contains(joined, want) {
						t.Fatalf("read %d: defects do not name %q: %s", read, want, joined)
					}
				}
			}
			// And through Handle: an AUTO fixture still does not act, and the
			// named defect rides the Result.
			mach := &fakeMachine{id: machineID}
			a := &Adapter{Medium: fullMedium(), Recorder: newRecorder(), Machine: mach}
			res, err := a.Handle(context.Background(), autoEvent("p"), p, pc)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.Acted || mach.acts != 0 {
				t.Fatal("acted under a half-present activation")
			}
			if len(res.Refusals) == 0 {
				t.Fatal("half-present activation produced no named refusal on the Result")
			}
		})
	}
}

// --- A-3 / DINV-3: revision binding ---

// With activation present an act binds to the evaluated head; a post-evaluation
// push (the medium's current head no longer equals the evaluated head) voids the
// verdict before any act.
func TestEnforce_ActBindsToHead_PushVoids(t *testing.T) {
	p := policyFor(true, "refs/backstop@1", false, fleetTask)
	pc := Preconditions{RefProtectionPresent: true}

	t.Run("head unmoved: acts bound to the evaluated head", func(t *testing.T) {
		mach := &fakeMachine{id: machineID, credential: "s3cr3t"}
		a := &Adapter{Medium: fullMedium(), Recorder: newRecorder(), Machine: mach}
		res, err := a.Handle(context.Background(), autoEvent("p1"), p, pc)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.State != Enforcing {
			t.Fatalf("state = %q, want enforcing", res.State)
		}
		if !res.Acted || mach.acts != 1 {
			t.Fatalf("expected exactly one act, got acted=%v acts=%d refusals=%+v", res.Acted, mach.acts, res.Refusals)
		}
		if mach.gotHead != "head-sha" {
			t.Fatalf("act bound to head %q, want the evaluated head 'head-sha'", mach.gotHead)
		}
		if res.Act == nil || res.Act.HeadRevision != "head-sha" || res.Act.GoalVerify != "pass" {
			t.Fatalf("act record wrong: %+v", res.Act)
		}
	})

	t.Run("post-evaluation push: voided before any act", func(t *testing.T) {
		med := fullMedium()
		med.head = "head-sha-2" // the head moved after evaluation
		mach := &fakeMachine{id: machineID}
		a := &Adapter{Medium: med, Recorder: newRecorder(), Machine: mach}
		res, err := a.Handle(context.Background(), autoEvent("p1"), p, pc)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Acted || mach.acts != 0 {
			t.Fatal("acted on a moved head — the verdict must be void before any act (DINV-3)")
		}
		if !refusalMentions(res, "post-evaluation push") {
			t.Fatalf("refusal does not name the voided revision: %+v", res.Refusals)
		}
	})
}

// --- A-5: shadow corpus, every proposal recorded, zero acting side effects ---

func TestShadowCorpus_EveryProposalRecorded_ZeroActs(t *testing.T) {
	p := policyFor(false, "", false, fleetTask, driveDoc) // shadow: enforce unset
	a := &Adapter{
		Medium:   fullMedium(),
		Recorder: newRecorder(),
		Machine:  noActMachine{t: t}, // fails the test if ever asked to act
	}
	corpus := []Event{
		autoEvent("auto-1"),
		autoEvent("auto-2"),
		docEvent("doc-1"),
		humanEvent("human-1"),
		humanEvent("human-2"),
	}
	rec := a.Recorder.(*fakeRecorder)
	for _, ev := range corpus {
		res, err := a.Handle(context.Background(), ev, p, Preconditions{})
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", ev.Proposal, err)
		}
		if res.Acted {
			t.Fatalf("%s: an act occurred in shadow", ev.Proposal)
		}
		if got := rec.recs[ev.Proposal]; len(got) != 1 {
			t.Fatalf("%s: recorded %d verdicts, want exactly 1", ev.Proposal, len(got))
		}
	}
	if len(rec.recs) != len(corpus) {
		t.Fatalf("recorded %d proposals, want %d", len(rec.recs), len(corpus))
	}
}

// --- A-8: the credential is unreachable, the identity distinguishable ---

// A drive-side probe holds only a read Medium and a Recorder — never a Machine —
// so it cannot act and there is no path to a credential. And even a credentialed
// Adapter never leaks the secret into a Result.
func TestDriveProbe_NoCredentialReach_IdentityDistinguishable(t *testing.T) {
	// (1) A drive probe: nil Machine. It records verdicts but can never act.
	probe := &Adapter{Medium: fullMedium(), Recorder: newRecorder(), Machine: nil}
	// Even handed an enforcing policy, a probe with no Machine fails closed.
	res, err := probe.Handle(context.Background(), autoEvent("p1"),
		policyFor(true, "refs/backstop@1", false, fleetTask), Preconditions{RefProtectionPresent: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Acted {
		t.Fatal("a drive probe with no machine identity acted")
	}
	if !refusalMentions(res, "no machine identity") {
		t.Fatalf("probe refusal does not name the missing machine identity: %+v", res.Refusals)
	}

	// (2) A credentialed Adapter acts, but the secret never escapes into the
	// Result: marshalling the whole Result must not contain the credential.
	const secret = "TOP-SECRET-CREDENTIAL"
	mach := &fakeMachine{id: machineID, credential: secret}
	a := &Adapter{Medium: fullMedium(), Recorder: newRecorder(), Machine: mach}
	res2, err := a.Handle(context.Background(), autoEvent("p2"),
		policyFor(true, "refs/backstop@1", false, fleetTask), Preconditions{RefProtectionPresent: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res2.Acted {
		t.Fatal("expected the credentialed adapter to act")
	}
	blob, _ := json.Marshal(res2)
	if strings.Contains(string(blob), secret) {
		t.Fatalf("the machine-identity credential leaked into the Result: %s", blob)
	}
	if strings.Contains(string(res2.Act.Marshal()), secret) {
		t.Fatal("the credential leaked into the act self-description")
	}

	// (3) The identity is distinguishable from any human in history.
	if machineID.IsHuman() {
		t.Fatal("the machine identity reports as human")
	}
	if !humanID.IsHuman() {
		t.Fatal("a human identity reports as machine")
	}
	if machineID.Name == humanID.Name {
		t.Fatal("the machine identity's name is not distinguishable from a human's")
	}
	if !res2.Act.MachineActor {
		t.Fatal("the act self-description does not mark the actor as the machine identity")
	}
}

// --- the rest of the act gate (DINV-3): checks, reviewer precondition ---

func TestActGate_FailClosedStops(t *testing.T) {
	enforceP := func(classes ...policy.AutoClass) *policy.Policy {
		return policyFor(true, "refs/backstop@1", false, classes...)
	}
	pc := Preconditions{RefProtectionPresent: true}

	t.Run("a not-green required check stops the act", func(t *testing.T) {
		med := fullMedium()
		med.green = false
		mach := &fakeMachine{id: machineID}
		a := &Adapter{Medium: med, Recorder: newRecorder(), Machine: mach}
		res, _ := a.Handle(context.Background(), autoEvent("p"), enforceP(fleetTask), pc)
		if res.Acted || mach.acts != 0 {
			t.Fatal("acted with a required check not green")
		}
		if !refusalMentions(res, "not green") {
			t.Fatalf("refusal does not name the not-green check: %+v", res.Refusals)
		}
	})

	t.Run("reviewer precondition met: acts", func(t *testing.T) {
		med := fullMedium()
		med.review = verdict.CheckState{Context: verdict.CheckContext, Status: verdict.Approving}
		mach := &fakeMachine{id: machineID}
		a := &Adapter{Medium: med, Recorder: newRecorder(), Machine: mach}
		res, _ := a.Handle(context.Background(), docEvent("p"), enforceP(driveDoc), pc)
		if !res.Acted || mach.acts != 1 {
			t.Fatalf("expected an act with an approving reviewer verdict: %+v", res.Refusals)
		}
		if !strings.Contains(res.Act.ReviewerVerdict, "approving") {
			t.Fatalf("act does not pin the consumed reviewer verdict: %+v", res.Act)
		}
	})

	t.Run("reviewer precondition unmet: stops the act", func(t *testing.T) {
		med := fullMedium()
		med.review = verdict.CheckState{Context: verdict.CheckContext, Status: verdict.NonApproving, Reason: "no published state"}
		mach := &fakeMachine{id: machineID}
		a := &Adapter{Medium: med, Recorder: newRecorder(), Machine: mach}
		res, _ := a.Handle(context.Background(), docEvent("p"), enforceP(driveDoc), pc)
		if res.Acted || mach.acts != 0 {
			t.Fatal("acted with the reviewer-verdict precondition unmet")
		}
		if !refusalMentions(res, "precondition is unmet") {
			t.Fatalf("refusal does not name the unmet reviewer precondition: %+v", res.Refusals)
		}
	})
}

// --- kill switch and halt stop acts ---

func TestKillAndHaltStopActs(t *testing.T) {
	pc := Preconditions{RefProtectionPresent: true}

	t.Run("kill switch: killed, HUMAN-only, classifier routes HUMAN", func(t *testing.T) {
		mach := &fakeMachine{id: machineID}
		a := &Adapter{Medium: fullMedium(), Recorder: newRecorder(), Machine: mach}
		res, _ := a.Handle(context.Background(), autoEvent("p"),
			policyFor(true, "refs/backstop@1", true, fleetTask), pc)
		// The kill switch flips the whole lane HUMAN-only in one data change: the
		// classifier routes every proposal HUMAN and the state resolves Killed,
		// so no act occurs.
		if res.State != Killed || res.Acted || mach.acts != 0 {
			t.Fatalf("kill switch did not stop the act: state=%q acted=%v", res.State, res.Acted)
		}
		if res.Verdict.Lane != classifier.Human {
			t.Fatalf("kill switch did not route the verdict HUMAN: %q", res.Verdict.Lane)
		}
	})

	t.Run("halt: no further act pending a human disposition", func(t *testing.T) {
		mach := &fakeMachine{id: machineID}
		a := &Adapter{Medium: fullMedium(), Recorder: newRecorder(), Machine: mach}
		res, _ := a.Handle(context.Background(), autoEvent("p"),
			policyFor(true, "refs/backstop@1", false, fleetTask),
			Preconditions{RefProtectionPresent: true, Halted: true})
		if res.State != Halted || res.Acted || mach.acts != 0 {
			t.Fatalf("halt did not stop the act: state=%q acted=%v", res.State, res.Acted)
		}
		if !refusalMentions(res, "halted") {
			t.Fatalf("refusal does not name the halt: %+v", res.Refusals)
		}
	})
}

// --- goal verification failure halts (DINV-7) ---

func TestGoalVerifyFailure_HaltsLane(t *testing.T) {
	med := fullMedium()
	med.merged = false // the badge says merged; mainline ancestry says otherwise
	mach := &fakeMachine{id: machineID}
	a := &Adapter{Medium: med, Recorder: newRecorder(), Machine: mach}
	res, err := a.Handle(context.Background(), autoEvent("p"),
		policyFor(true, "refs/backstop@1", false, fleetTask), Preconditions{RefProtectionPresent: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Acted {
		t.Fatal("expected the act to occur before goal verification")
	}
	if !res.Halt {
		t.Fatal("a failed goal-object verification did not signal a halt (DINV-7)")
	}
	if res.Act.GoalVerify != "fail" {
		t.Fatalf("act record goal-verify = %q, want fail", res.Act.GoalVerify)
	}
}

// --- infrastructure errors surface, not silent acts ---

func TestInfraErrorsSurface(t *testing.T) {
	p := policyFor(true, "refs/backstop@1", false, fleetTask)
	pc := Preconditions{RefProtectionPresent: true}
	med := fullMedium()
	med.headErr = errors.New("medium unreachable")
	a := &Adapter{Medium: med, Recorder: newRecorder(), Machine: &fakeMachine{id: machineID}}
	if _, err := a.Handle(context.Background(), autoEvent("p"), p, pc); err == nil {
		t.Fatal("a medium read error must surface, never a silent act")
	}
}

// --- the act record marshals deterministically ---

func TestActRecordDeterministic(t *testing.T) {
	a := ActRecord{
		RecordVersion: ActRecordVersion, ApprovedBy: "pyxis-lane-bot", MachineActor: true,
		HeadRevision: "h", PolicyVersion: "risk-policy/v1", VerdictRecord: "p@h", GoalVerify: "pass",
	}
	if string(a.Marshal()) != string(a.Marshal()) {
		t.Fatal("act record marshal is not byte-stable")
	}
	var round ActRecord
	if err := json.Unmarshal(a.Marshal(), &round); err != nil || round != a {
		t.Fatalf("act record does not round-trip: %v", err)
	}
}

func TestActRecordMarshalPanicsOnSerializationFailure(t *testing.T) {
	old := marshalActRecord
	marshalActRecord = func(any) ([]byte, error) { return nil, errors.New("encode failed") }
	t.Cleanup(func() { marshalActRecord = old })
	defer func() {
		if recover() == nil {
			t.Fatal("Marshal did not panic on an impossible serialization failure")
		}
	}()
	_ = (ActRecord{}).Marshal()
}
