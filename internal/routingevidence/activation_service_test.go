package routingevidence

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestActivationServiceAppendsAndExactReadsCheckpoint(t *testing.T) {
	reducer, _ := reducerFixture(t)
	state, manifest, mainline, _ := activationFixture(t, false)
	reader := activationReaderStub{read: ExactRead{
		ObservedHead: ObservedHead{EvidenceRef: state.EvidenceRef, Commit: state.EvidenceHead},
		Events:       append([]Event(nil), state.events...),
	}}
	appender := &activationAppenderStub{proof: EvidenceProof{EvidenceRef: state.EvidenceRef, Commit: "checkpoint-head"}}
	store := &activationStoreStub{}
	store.readProof = func(_ EvidenceProof) (ExactRead, error) {
		return ExactRead{Events: []Event{{Kind: ActivationCheckpointEvent, Payload: Payload{ActivationCheckpoint: &appender.checkpoint.Payload}}}}, nil
	}

	got, err := (ActivationService{
		Reader: reader, Reducer: reducer, Appender: appender, Store: store, Artifacts: activationArtifactFixture{},
	}).Activate(t.Context(), manifest, mainline)
	if err != nil {
		t.Fatal(err)
	}
	if got.Commit != "checkpoint-head" || appender.calls != 1 || store.readProofCalls != 1 {
		t.Fatalf("activation = %+v, append calls=%d read calls=%d", got, appender.calls, store.readProofCalls)
	}
	if !appender.checkpoint.Payload.ActivationReady || appender.checkpoint.Payload.ProofManifestDigest == "" {
		t.Fatalf("checkpoint = %+v", appender.checkpoint.Payload)
	}
}

func TestActivationServiceFailsClosedAtEveryBoundary(t *testing.T) {
	reducer, _ := reducerFixture(t)
	state, manifest, mainline, _ := activationFixture(t, false)
	validRead := ExactRead{
		ObservedHead: ObservedHead{EvidenceRef: state.EvidenceRef, Commit: state.EvidenceHead},
		Events:       append([]Event(nil), state.events...),
	}
	base := func() (ActivationService, *activationAppenderStub, *activationStoreStub) {
		appender := &activationAppenderStub{proof: EvidenceProof{EvidenceRef: state.EvidenceRef, Commit: "checkpoint-head"}}
		store := &activationStoreStub{}
		store.readProof = func(_ EvidenceProof) (ExactRead, error) {
			return ExactRead{Events: []Event{{Kind: ActivationCheckpointEvent, Payload: Payload{ActivationCheckpoint: &appender.checkpoint.Payload}}}}, nil
		}
		return ActivationService{Reader: activationReaderStub{read: validRead}, Reducer: reducer, Appender: appender, Store: store, Artifacts: activationArtifactFixture{}}, appender, store
	}

	t.Run("missing dependency", func(t *testing.T) {
		_, err := (ActivationService{}).Activate(t.Context(), manifest, mainline)
		assertActivationCode(t, err, ActivationEvidenceStale)
	})
	t.Run("read head", func(t *testing.T) {
		svc, _, _ := base()
		svc.Reader = activationReaderStub{err: errors.New("unavailable")}
		_, err := svc.Activate(t.Context(), manifest, mainline)
		assertActivationCode(t, err, ActivationEvidenceStale)
	})
	t.Run("reduce", func(t *testing.T) {
		svc, _, _ := base()
		bad := validRead
		bad.Events = append([]Event(nil), validRead.Events...)
		bad.Events[0].Schema = "invalid"
		svc.Reader = activationReaderStub{read: bad}
		_, err := svc.Activate(t.Context(), manifest, mainline)
		assertActivationCode(t, err, ActivationEvidenceStale)
	})
	t.Run("bind evidence head", func(t *testing.T) {
		svc, _, _ := base()
		bad := validRead
		bad.ObservedHead.EvidenceRef = "refs/heads/other"
		svc.Reader = activationReaderStub{read: bad}
		if _, err := svc.Activate(t.Context(), manifest, mainline); !errors.Is(err, ErrReducerIntegrity) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("artifact", func(t *testing.T) {
		svc, _, _ := base()
		svc.Artifacts = activationArtifactFixture{tamper: true}
		_, err := svc.Activate(t.Context(), manifest, mainline)
		assertActivationCode(t, err, ActivationProofSetInvalid)
	})
	t.Run("verification", func(t *testing.T) {
		svc, _, _ := base()
		bad := mainline
		bad.HumanMerged = false
		_, err := svc.Activate(t.Context(), manifest, bad)
		assertActivationCode(t, err, ActivationMainlineInvalid)
	})
	t.Run("append", func(t *testing.T) {
		svc, appender, _ := base()
		appender.err = errors.New("lease lost")
		_, err := svc.Activate(t.Context(), manifest, mainline)
		if err == nil || !strings.Contains(err.Error(), "activation checkpoint: append: lease lost") {
			t.Fatalf("error = %v", err)
		}
	})
	for _, tc := range []struct {
		name string
		read func(EvidenceProof) (ExactRead, error)
	}{
		{name: "read after append error", read: func(EvidenceProof) (ExactRead, error) { return ExactRead{}, errors.New("gone") }},
		{name: "missing event", read: func(EvidenceProof) (ExactRead, error) { return ExactRead{}, nil }},
		{name: "wrong event kind", read: func(EvidenceProof) (ExactRead, error) {
			return ExactRead{Events: []Event{{Kind: RoutingResolved}}}, nil
		}},
		{name: "wrong manifest digest", read: func(EvidenceProof) (ExactRead, error) {
			return ExactRead{Events: []Event{{Kind: ActivationCheckpointEvent, Payload: Payload{ActivationCheckpoint: &ActivationCheckpointPayload{ProofManifestDigest: "other"}}}}}, nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, store := base()
			store.readProof = tc.read
			_, err := svc.Activate(t.Context(), manifest, mainline)
			assertActivationCode(t, err, ActivationEvidenceStale)
		})
	}
}

type activationReaderStub struct {
	read ExactRead
	err  error
}

func (r activationReaderStub) ReadHead(context.Context) (ExactRead, error) { return r.read, r.err }

type activationAppenderStub struct {
	checkpoint ActivationCheckpoint
	proof      EvidenceProof
	err        error
	calls      int
}

func (a *activationAppenderStub) AppendCheckpoint(_ context.Context, checkpoint ActivationCheckpoint) (EvidenceProof, error) {
	a.calls++
	a.checkpoint = checkpoint
	return a.proof, a.err
}

type activationStoreStub struct {
	readProof      func(EvidenceProof) (ExactRead, error)
	readProofCalls int
}

func (*activationStoreStub) Append(context.Context, AppendRequest) (EvidenceProof, error) {
	return EvidenceProof{}, errors.New("unused")
}

func (*activationStoreStub) ReadSubject(context.Context, SubjectID) (ExactRead, error) {
	return ExactRead{}, errors.New("unused")
}

func (s *activationStoreStub) ReadProof(_ context.Context, proof EvidenceProof) (ExactRead, error) {
	s.readProofCalls++
	return s.readProof(proof)
}
