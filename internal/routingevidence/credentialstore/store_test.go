package credentialstore

import (
	"context"
	"crypto/ed25519"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/routingevidence"
)

type memoryStore struct{ values map[string][]byte }

func (s *memoryStore) Put(_ context.Context, account string, secret []byte) error {
	if s.values == nil {
		s.values = map[string][]byte{}
	}
	s.values[account] = append([]byte(nil), secret...)
	return nil
}
func (s *memoryStore) Get(_ context.Context, account string) ([]byte, error) {
	secret, ok := s.values[account]
	if !ok {
		return nil, ErrCredentialUnavailable
	}
	return append([]byte(nil), secret...), nil
}

type failingStore struct {
	putErr error
	getErr error
	secret []byte
}

func (s failingStore) Put(context.Context, string, []byte) error { return s.putErr }
func (s failingStore) Get(context.Context, string) ([]byte, error) {
	return append([]byte(nil), s.secret...), s.getErr
}

func TestCredentialCoreRejectsInvalidInputsAndStoreFailures(t *testing.T) {
	if _, err := Generate(t.Context(), nil, "account", "purpose"); err == nil {
		t.Fatal("Generate accepted a nil store")
	}
	if _, err := Generate(t.Context(), failingStore{putErr: errors.New("put failed")}, "account", "purpose"); err == nil {
		t.Fatal("Generate ignored Put failure")
	}
	if _, err := Load(t.Context(), nil, Key{}); err == nil {
		t.Fatal("Load accepted incomplete identity")
	}
	key := Key{Account: "account", KeyID: "purpose:key", PublicKey: strings.Repeat("0", ed25519.PublicKeySize*2)}
	if _, err := Load(t.Context(), failingStore{getErr: errors.New("get failed")}, key); err == nil {
		t.Fatal("Load ignored Get failure")
	}
	if _, err := Load(t.Context(), failingStore{secret: []byte("short")}, key); !errors.Is(err, ErrCredentialUnavailable) {
		t.Fatalf("short seed error = %v", err)
	}
}

func TestGenerateAndLoadBindsRepositoryPurposeAndPublicKey(t *testing.T) {
	store := &memoryStore{}
	account, err := Account("agenticlab-ai/pyxis", "operator")
	if err != nil {
		t.Fatal(err)
	}
	key, err := Generate(t.Context(), store, account, "operator")
	if err != nil {
		t.Fatal(err)
	}
	private, err := Load(t.Context(), store, key)
	if err != nil {
		t.Fatal(err)
	}
	if len(private) != ed25519.PrivateKeySize || !strings.HasPrefix(key.KeyID, "operator:") || key.Account != account {
		t.Fatalf("key = %+v private=%d", key, len(private))
	}
	forged := key
	forged.PublicKey = strings.Repeat("0", ed25519.PublicKeySize*2)
	if _, err := Load(t.Context(), store, forged); !errors.Is(err, ErrCredentialUnavailable) {
		t.Fatalf("forged public key = %v", err)
	}
}

func TestLoadProducerCapabilitySelectsOnlyCurrentActivationPoint(t *testing.T) {
	store := &memoryStore{}
	account, err := Account("owner/repo", "producer:resolver-orchestrator")
	if err != nil {
		t.Fatal(err)
	}
	active, err := Generate(t.Context(), store, account, "producer:resolver-orchestrator")
	if err != nil {
		t.Fatal(err)
	}
	policy := routingevidence.ProducerPolicy{Repository: "owner/repo", EvidenceRef: "refs/pyxis/routing-evidence", ActivationPoint: "policy/2",
		Keys: []routingevidence.KeyGeneration{
			{KeyID: "retired-generation", PublicKey: strings.Repeat("0", ed25519.PublicKeySize*2), ProducerClass: routingevidence.ProducerResolver, PolicyActivationPoints: []string{"policy/1"}},
			{KeyID: active.KeyID, PublicKey: active.PublicKey, ProducerClass: routingevidence.ProducerResolver, ImplementationGeneration: "resolver/v2", PolicyActivationPoints: []string{"policy/2"}},
		}}
	capability, err := LoadProducerCapability(t.Context(), store, policy, routingevidence.ProducerResolver,
		[]routingevidence.EventKind{routingevidence.RoutingResolved}, []string{"implementation"})
	if err != nil || capability.KeyID != active.KeyID || capability.ImplementationGeneration != "resolver/v2" {
		t.Fatalf("LoadProducerCapability = %+v, %v", capability, err)
	}
}

type call struct {
	stdin []byte
	name  string
	args  []string
}
type fakeRunner struct {
	calls []call
	out   []byte
	err   error
}

func (r *fakeRunner) Run(_ context.Context, stdin []byte, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, call{append([]byte(nil), stdin...), name, append([]string(nil), args...)})
	return append([]byte(nil), r.out...), r.err
}

func TestOSStoreNeverPlacesSecretInArguments(t *testing.T) {
	for _, goos := range []string{"darwin", "linux"} {
		t.Run(goos, func(t *testing.T) {
			runner := &fakeRunner{}
			store := newOSStore(goos, runner)
			secret := []byte{0xde, 0xad, 0xbe, 0xef}
			if err := store.Put(t.Context(), "owner/repo#operator", secret); err != nil {
				t.Fatal(err)
			}
			if len(runner.calls) != 1 || !strings.Contains(string(runner.calls[0].stdin), "deadbeef") {
				t.Fatalf("call = %+v", runner.calls)
			}
			for _, arg := range runner.calls[0].args {
				if strings.Contains(arg, "deadbeef") {
					t.Fatalf("secret leaked into argv: %v", runner.calls[0].args)
				}
			}
		})
	}
}

func TestOSStoreReadsAndDecodesNativeCredential(t *testing.T) {
	runner := &fakeRunner{out: []byte("00010203\n")}
	got, err := newOSStore("darwin", runner).Get(t.Context(), "owner/repo#operator")
	if err != nil || !reflect.DeepEqual(got, []byte{0, 1, 2, 3}) {
		t.Fatalf("Get = %x, %v", got, err)
	}
	if len(runner.calls) != 1 || runner.calls[0].name != "security" || reflect.ValueOf(runner.calls[0].args).String() == "00010203" {
		t.Fatalf("call = %+v", runner.calls)
	}
}

func TestOSStoreFailsClosedWhenUnsupportedOrMissing(t *testing.T) {
	if err := newOSStore("windows", &fakeRunner{}).Put(t.Context(), "a", []byte("x")); !errors.Is(err, ErrCredentialUnavailable) {
		t.Fatalf("unsupported Put = %v", err)
	}
	if _, err := newOSStore("darwin", &fakeRunner{err: errors.New("not found")}).Get(t.Context(), "a"); !errors.Is(err, ErrCredentialUnavailable) {
		t.Fatalf("missing Get = %v", err)
	}
}
