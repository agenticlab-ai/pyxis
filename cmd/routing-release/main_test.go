package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/routingevidence"
	"github.com/agenticlab-ai/pyxis/internal/routingevidence/credentialstore"
)

type commandCredentialStore struct {
	values map[string][]byte
	putErr error
}

func (s *commandCredentialStore) Put(_ context.Context, account string, secret []byte) error {
	if s.putErr != nil {
		return s.putErr
	}
	if s.values == nil {
		s.values = map[string][]byte{}
	}
	s.values[account] = append([]byte(nil), secret...)
	return nil
}

func (s *commandCredentialStore) Get(_ context.Context, account string) ([]byte, error) {
	value, ok := s.values[account]
	if !ok {
		return nil, credentialstore.ErrCredentialUnavailable
	}
	return append([]byte(nil), value...), nil
}

func TestKeygenKeepsPrivateMaterialInCredentialStore(t *testing.T) {
	store := &commandCredentialStore{}
	prior := newCredentialStore
	newCredentialStore = func() credentialstore.Store { return store }
	t.Cleanup(func() { newCredentialStore = prior })
	var stdout, stderr bytes.Buffer
	code := run(t.Context(), []string{"keygen", "--repository", "owner/repo", "--purpose", "producer:resolver-orchestrator"}, &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("run = %d stderr=%q", code, stderr.String())
	}
	var key credentialstore.Key
	if err := json.Unmarshal(stdout.Bytes(), &key); err != nil {
		t.Fatal(err)
	}
	private, err := credentialstore.Load(t.Context(), store, key)
	if err != nil || len(private) == 0 || bytes.Contains(stdout.Bytes(), store.values[key.Account]) {
		t.Fatalf("key descriptor/private separation failed: key=%+v private=%d err=%v", key, len(private), err)
	}
}

func TestKeygenRejectsOpenPurposeWithoutWriting(t *testing.T) {
	store := &commandCredentialStore{}
	prior := newCredentialStore
	newCredentialStore = func() credentialstore.Store { return store }
	t.Cleanup(func() { newCredentialStore = prior })
	var stdout, stderr bytes.Buffer
	code := run(t.Context(), []string{"keygen", "--repository", "owner/repo", "--purpose", "producer:root"}, &stdout, &stderr)
	if code != 2 || len(store.values) != 0 {
		t.Fatalf("run = %d stores=%d stderr=%q", code, len(store.values), stderr.String())
	}
}

func TestKeygenSurfacesCredentialAndOutputFailures(t *testing.T) {
	prior := newCredentialStore
	t.Cleanup(func() { newCredentialStore = prior })

	newCredentialStore = func() credentialstore.Store {
		return &commandCredentialStore{putErr: errors.New("credential store unavailable")}
	}
	var stdout, stderr bytes.Buffer
	if code := keygen(t.Context(), []string{"--repository", "owner/repo", "--purpose", "operator"}, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "keygen") {
		t.Fatalf("credential failure = %d stderr=%q", code, stderr.String())
	}

	newCredentialStore = func() credentialstore.Store { return &commandCredentialStore{} }
	stderr.Reset()
	if code := keygen(t.Context(), []string{"--repository", "owner/repo", "--purpose", "operator"}, failingWriter{}, &stderr); code != 1 || !strings.Contains(stderr.String(), "encode key descriptor") {
		t.Fatalf("output failure = %d stderr=%q", code, stderr.String())
	}
}

type policyArtifactReader struct {
	wire []byte
	err  error
}

func (r policyArtifactReader) ReadActivationArtifact(context.Context, string, string) ([]byte, error) {
	return append([]byte(nil), r.wire...), r.err
}

func TestBindProducerPolicyArtifactRequiresExactPinnedMainlineBytes(t *testing.T) {
	wire := []byte("canonical-policy\n")
	digest := sha256.Sum256(wire)
	path := ".pyxis/routing/producer-policy.json"
	manifest := routingevidence.ActivationManifest{ExactMainlineObject: strings.Repeat("a", 40), Proofs: []routingevidence.ActivationProof{
		{Kind: routingevidence.ProofRuntimeGeneration, ID: "repo:" + path, Digest: fmt.Sprintf("%x", digest[:])},
	}}
	if err := bindProducerPolicyArtifact(t.Context(), policyArtifactReader{wire: wire}, manifest, path, wire); err != nil {
		t.Fatal(err)
	}
	if err := bindProducerPolicyArtifact(t.Context(), policyArtifactReader{err: errors.New("read failed")}, manifest, path, wire); err == nil {
		t.Fatal("artifact reader failure was hidden")
	}
	if err := bindProducerPolicyArtifact(t.Context(), policyArtifactReader{wire: wire}, manifest, path, []byte("dirty")); err == nil {
		t.Fatal("dirty producer policy was accepted")
	}
	manifest.Proofs[0].ID = "repo:other.json"
	if err := bindProducerPolicyArtifact(t.Context(), policyArtifactReader{wire: wire}, manifest, path, wire); err == nil {
		t.Fatal("unbound producer policy was accepted")
	}
}

func TestRunUsageAndValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want int
		out  string
	}{
		{name: "empty", want: 2, out: "usage:"},
		{name: "help", args: []string{"help"}, want: 2, out: "usage:"},
		{name: "unknown", args: []string{"nope"}, want: 2, out: "unknown verb"},
		{name: "keygen bad flags", args: []string{"keygen", "--bad"}, want: 2},
		{name: "manifest missing flags", args: []string{"sign-manifest"}, want: 2},
		{name: "mainline missing flags", args: []string{"sign-mainline"}, want: 2},
		{name: "checkpoint missing flags", args: []string{"checkpoint"}, want: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			got := run(t.Context(), tc.args, &stdout, &stderr)
			if got != tc.want || (tc.out != "" && !strings.Contains(stdout.String()+stderr.String(), tc.out)) {
				t.Fatalf("run = %d stdout=%q stderr=%q", got, stdout.String(), stderr.String())
			}
		})
	}
}

func TestSignManifestAndMainline(t *testing.T) {
	policy, operatorPrivate := releasePolicyFixture(t)
	policyWire, err := routingevidence.EncodeProducerPolicy(policy)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	policyPath := writeReleaseFixture(t, dir, "policy.json", append(policyWire, '\n'))
	manifest := releaseManifestFixture()
	manifestPath := writeReleaseJSON(t, dir, "manifest.json", manifest)

	store := &commandCredentialStore{}
	account, err := credentialstore.Account(policy.Repository, "operator")
	if err != nil {
		t.Fatal(err)
	}
	store.values = map[string][]byte{account: append([]byte(nil), operatorPrivate.Seed()...)}
	prior := newCredentialStore
	newCredentialStore = func() credentialstore.Store { return store }
	t.Cleanup(func() { newCredentialStore = prior })

	var manifestOut, stderr bytes.Buffer
	code := run(t.Context(), []string{"sign-manifest", "--repository", policy.Repository, "--policy", policyPath, "--input", manifestPath}, &manifestOut, &stderr)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("sign-manifest = %d stderr=%q", code, stderr.String())
	}
	signedManifest, err := routingevidence.DecodeActivationManifest(trimOneNewline(manifestOut.Bytes()))
	if err != nil || signedManifest.OperatorSignature.KeyID != "operator-key-1" {
		t.Fatalf("signed manifest = %+v, %v", signedManifest, err)
	}
	digest, err := routingevidence.DigestActivationManifest(signedManifest)
	if err != nil {
		t.Fatal(err)
	}
	mainline := routingevidence.MainlineProof{Schema: routingevidence.MainlineProofSchema, Repository: policy.Repository,
		MainlineRef: manifest.MainlineRef, ExactMainlineObject: manifest.ExactMainlineObject, CurrentObject: manifest.ExactMainlineObject,
		HumanMerged: true, HumanMergeActor: "human:operator", ManifestDigest: digest,
		OperatorAuthorizationID: manifest.OperatorAuthorizationID, Proofs: append([]routingevidence.ActivationProof(nil), manifest.Proofs...)}
	mainlinePath := writeReleaseJSON(t, dir, "mainline.json", mainline)
	var mainlineOut bytes.Buffer
	stderr.Reset()
	code = run(t.Context(), []string{"sign-mainline", "--repository", policy.Repository, "--policy", policyPath, "--input", mainlinePath}, &mainlineOut, &stderr)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("sign-mainline = %d stderr=%q", code, stderr.String())
	}
	if _, err := routingevidence.DecodeMainlineProof(trimOneNewline(mainlineOut.Bytes())); err != nil {
		t.Fatalf("decode signed mainline: %v", err)
	}

	stderr.Reset()
	code = run(t.Context(), []string{"sign-manifest", "--repository", policy.Repository, "--policy", policyPath, "--input", manifestPath}, failingWriter{}, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "write signed manifest") {
		t.Fatalf("failed output = %d stderr=%q", code, stderr.String())
	}
	stderr.Reset()
	code = run(t.Context(), []string{"sign-mainline", "--repository", policy.Repository, "--policy", policyPath, "--input", mainlinePath}, failingWriter{}, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "write signed mainline") {
		t.Fatalf("failed output = %d stderr=%q", code, stderr.String())
	}
}

func TestSigningFailureClasses(t *testing.T) {
	policy, _ := releasePolicyFixture(t)
	policyWire, err := routingevidence.EncodeProducerPolicy(policy)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	policyPath := writeReleaseFixture(t, dir, "policy.json", policyWire)
	manifest := releaseManifestFixture()
	missingAuth := manifest
	missingAuth.OperatorAuthorizationID = "unknown"
	missingAuthPath := writeReleaseJSON(t, dir, "missing-auth.json", missingAuth)
	unknownPath := writeReleaseFixture(t, dir, "unknown.json", []byte(`{"schema":"routing-activation-proof/v2","unknown":true}`))
	trailingPath := writeReleaseFixture(t, dir, "trailing.json", []byte(`{} {}`))

	prior := newCredentialStore
	newCredentialStore = func() credentialstore.Store { return &commandCredentialStore{} }
	t.Cleanup(func() { newCredentialStore = prior })

	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{name: "missing policy", args: []string{"sign-manifest", "--repository", policy.Repository, "--policy", filepath.Join(dir, "absent"), "--input", missingAuthPath}, want: "read producer policy"},
		{name: "different repository", args: []string{"sign-manifest", "--repository", "other/repo", "--policy", policyPath, "--input", missingAuthPath}, want: "different repository"},
		{name: "closed input", args: []string{"sign-manifest", "--repository", policy.Repository, "--policy", policyPath, "--input", unknownPath}, want: "read unsigned manifest"},
		{name: "trailing input", args: []string{"sign-manifest", "--repository", policy.Repository, "--policy", policyPath, "--input", trailingPath}, want: "trailing JSON"},
		{name: "missing authorization", args: []string{"sign-manifest", "--repository", policy.Repository, "--policy", policyPath, "--input", missingAuthPath}, want: "activation authorization"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(t.Context(), tc.args, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("run = %d stderr=%q", code, stderr.String())
			}
		})
	}

	validManifestPath := writeReleaseJSON(t, dir, "valid-manifest.json", manifest)
	var stdout, stderr bytes.Buffer
	if code := run(t.Context(), []string{"sign-manifest", "--repository", policy.Repository, "--policy", policyPath, "--input", validManifestPath}, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "operator key") {
		t.Fatalf("missing key run = %d stderr=%q", code, stderr.String())
	}

	mainline := routingevidence.MainlineProof{
		Schema:                  routingevidence.MainlineProofSchema,
		Repository:              policy.Repository,
		OperatorAuthorizationID: "unknown",
	}
	mainlinePath := writeReleaseJSON(t, dir, "mainline-missing-auth.json", mainline)
	unknownMainlinePath := writeReleaseFixture(t, dir, "mainline-unknown.json", []byte(`{"unknown":true}`))
	validMainline := mainline
	validMainline.OperatorAuthorizationID = "auth-activation"
	validMainlinePath := writeReleaseJSON(t, dir, "mainline-valid-auth.json", validMainline)
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{name: "mainline missing policy", args: []string{"sign-mainline", "--repository", policy.Repository, "--policy", filepath.Join(dir, "absent"), "--input", mainlinePath}, want: "read producer policy"},
		{name: "mainline different repository", args: []string{"sign-mainline", "--repository", "other/repo", "--policy", policyPath, "--input", mainlinePath}, want: "different repository"},
		{name: "mainline closed input", args: []string{"sign-mainline", "--repository", policy.Repository, "--policy", policyPath, "--input", unknownMainlinePath}, want: "read unsigned mainline"},
		{name: "mainline missing authorization", args: []string{"sign-mainline", "--repository", policy.Repository, "--policy", policyPath, "--input", mainlinePath}, want: "activation authorization"},
		{name: "mainline missing operator key", args: []string{"sign-mainline", "--repository", policy.Repository, "--policy", policyPath, "--input", validMainlinePath}, want: "operator key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stdout.Reset()
			stderr.Reset()
			if code := run(t.Context(), tc.args, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("run = %d stderr=%q", code, stderr.String())
			}
		})
	}
}

func TestCheckpointFailsClosedAcrossEveryInputBoundary(t *testing.T) {
	policy, operatorPrivate := releasePolicyFixture(t)
	repo := t.TempDir()
	policyWire, err := routingevidence.EncodeProducerPolicy(policy)
	if err != nil {
		t.Fatal(err)
	}
	policyPath := writeReleaseFixture(t, repo, "policy.json", append(policyWire, '\n'))
	manifest, err := routingevidence.SignActivationManifest(releaseManifestFixture(), "operator-key-1", operatorPrivate)
	if err != nil {
		t.Fatal(err)
	}
	manifestWire, err := routingevidence.EncodeActivationManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := writeReleaseFixture(t, repo, "manifest.json", append(manifestWire, '\n'))
	badManifestPath := writeReleaseFixture(t, repo, "bad-manifest.json", []byte("{}"))
	digest, err := routingevidence.DigestActivationManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	mainline := routingevidence.MainlineProof{Schema: routingevidence.MainlineProofSchema, Repository: policy.Repository,
		MainlineRef: manifest.MainlineRef, ExactMainlineObject: manifest.ExactMainlineObject, CurrentObject: manifest.ExactMainlineObject,
		HumanMerged: true, HumanMergeActor: "human:operator", ManifestDigest: digest,
		OperatorAuthorizationID: manifest.OperatorAuthorizationID, Proofs: append([]routingevidence.ActivationProof(nil), manifest.Proofs...)}
	mainline, err = routingevidence.SignMainlineProof(mainline, "operator-key-1", operatorPrivate)
	if err != nil {
		t.Fatal(err)
	}
	mainlineWire, err := routingevidence.EncodeMainlineProof(mainline)
	if err != nil {
		t.Fatal(err)
	}
	mainlinePath := writeReleaseFixture(t, repo, "mainline.json", append(mainlineWire, '\n'))
	badMainlinePath := writeReleaseFixture(t, repo, "bad-mainline.json", []byte("{}"))
	writeReleaseFixture(t, repo, "bad-policy.json", []byte("{}"))

	base := []string{"checkpoint", "--repository", policy.Repository, "--repo-dir", repo, "--remote", "origin"}
	for _, tc := range []struct {
		name string
		args []string
		want int
		text string
	}{
		{name: "non repository policy path", args: append(append([]string(nil), base...), "--policy", filepath.Join(repo, "policy.json"), "--manifest", manifestPath, "--mainline", mainlinePath), want: 2, text: "producer policy path"},
		{name: "missing policy", args: append(append([]string(nil), base...), "--policy", "missing.json", "--manifest", manifestPath, "--mainline", mainlinePath), want: 1, text: "read producer policy"},
		{name: "malformed policy", args: append(append([]string(nil), base...), "--policy", "bad-policy.json", "--manifest", manifestPath, "--mainline", mainlinePath), want: 1, text: "producer policy"},
		{name: "wrong policy repository", args: append(append([]string(nil), base...), "--repository", "other/repo", "--policy", filepath.Base(policyPath), "--manifest", manifestPath, "--mainline", mainlinePath), want: 1, text: "different repository"},
		{name: "missing manifest", args: append(append([]string(nil), base...), "--policy", filepath.Base(policyPath), "--manifest", filepath.Join(repo, "missing-manifest"), "--mainline", mainlinePath), want: 1, text: "read manifest"},
		{name: "malformed manifest", args: append(append([]string(nil), base...), "--policy", filepath.Base(policyPath), "--manifest", badManifestPath, "--mainline", mainlinePath), want: 1, text: "decode manifest"},
		{name: "missing mainline", args: append(append([]string(nil), base...), "--policy", filepath.Base(policyPath), "--manifest", manifestPath, "--mainline", filepath.Join(repo, "missing-mainline")), want: 1, text: "read mainline proof"},
		{name: "malformed mainline", args: append(append([]string(nil), base...), "--policy", filepath.Base(policyPath), "--manifest", manifestPath, "--mainline", badMainlinePath), want: 1, text: "decode mainline proof"},
		{name: "remote exact read", args: append(append([]string(nil), base...), "--policy", filepath.Base(policyPath), "--manifest", manifestPath, "--mainline", mainlinePath), want: 1, text: "exact remote mainline differs"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(t.Context(), tc.args, &stdout, &stderr)
			if code != tc.want || !strings.Contains(stderr.String(), tc.text) {
				t.Fatalf("checkpoint = %d stderr=%q; want %d containing %q", code, stderr.String(), tc.want, tc.text)
			}
		})
	}
}

func TestCheckpointRejectsInvalidRepositoryConfig(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".pyxis", "config.yaml"), []byte("remote: \"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := checkpoint(t.Context(), []string{"--repository", "owner/repo", "--repo-dir", repo, "--policy", "policy.json", "--manifest", "manifest.json", "--mainline", "mainline.json"}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "repository config has 1 defect") {
		t.Fatalf("checkpoint = %d stderr=%q", code, stderr.String())
	}
}

func TestCheckpointReachesReleaseGateCredentialBoundary(t *testing.T) {
	policy, operatorPrivate := releasePolicyFixture(t)
	policyWire, err := routingevidence.EncodeProducerPolicy(policy)
	if err != nil {
		t.Fatal(err)
	}
	policyFile := append(append([]byte(nil), policyWire...), '\n')
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	repo := filepath.Join(root, "repo")
	runReleaseGit(t, root, "init", "--bare", "-q", remote)
	runReleaseGit(t, root, "init", "-q", repo)
	runReleaseGit(t, repo, "config", "user.name", "Pyxis Test")
	runReleaseGit(t, repo, "config", "user.email", "pyxis@example.invalid")
	writeReleaseFixture(t, repo, "policy.json", policyFile)
	runReleaseGit(t, repo, "add", "policy.json")
	runReleaseGit(t, repo, "commit", "-qm", "policy")
	runReleaseGit(t, repo, "remote", "add", "origin", remote)
	runReleaseGit(t, repo, "push", "-q", "origin", "HEAD:refs/heads/main")
	mainObject := strings.TrimSpace(runReleaseGit(t, repo, "rev-parse", "HEAD"))

	manifest := releaseManifestFixture()
	manifest.ExactMainlineObject = mainObject
	manifest.Proofs[0].ID = "repo:policy.json"
	policyDigest := sha256.Sum256(policyFile)
	manifest.Proofs[0].Digest = hex.EncodeToString(policyDigest[:])
	manifest, err = routingevidence.SignActivationManifest(manifest, "operator-key-1", operatorPrivate)
	if err != nil {
		t.Fatal(err)
	}
	manifestWire, err := routingevidence.EncodeActivationManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := writeReleaseFixture(t, root, "manifest.json", manifestWire)
	digest, err := routingevidence.DigestActivationManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	mainline := routingevidence.MainlineProof{Schema: routingevidence.MainlineProofSchema, Repository: policy.Repository,
		MainlineRef: "refs/heads/main", ExactMainlineObject: mainObject, CurrentObject: mainObject, HumanMerged: true,
		HumanMergeActor: "human:operator", ManifestDigest: digest, OperatorAuthorizationID: manifest.OperatorAuthorizationID,
		Proofs: append([]routingevidence.ActivationProof(nil), manifest.Proofs...)}
	mainline, err = routingevidence.SignMainlineProof(mainline, "operator-key-1", operatorPrivate)
	if err != nil {
		t.Fatal(err)
	}
	mainlineWire, err := routingevidence.EncodeMainlineProof(mainline)
	if err != nil {
		t.Fatal(err)
	}
	mainlinePath := writeReleaseFixture(t, root, "mainline.json", mainlineWire)

	prior := newCredentialStore
	newCredentialStore = func() credentialstore.Store { return &commandCredentialStore{} }
	t.Cleanup(func() { newCredentialStore = prior })
	var stdout, stderr bytes.Buffer
	code := checkpoint(t.Context(), []string{"--repository", policy.Repository, "--repo-dir", repo, "--remote", "origin",
		"--policy", "policy.json", "--manifest", manifestPath, "--mainline", mainlinePath}, &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "release-gate key") {
		t.Fatalf("checkpoint = %d stderr=%q", code, stderr.String())
	}
}

func runReleaseGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}

func TestReleaseInputHelpers(t *testing.T) {
	for value, want := range map[string]bool{"owner/repo": true, "": false, "owner": false, "/repo": false, "owner/": false, "a/b/c": false, "owner /repo": false} {
		if got := validRepository(value); got != want {
			t.Errorf("validRepository(%q) = %v, want %v", value, got, want)
		}
	}
	for _, value := range []string{"operator", "producer:plan-gate", "producer:resolver-orchestrator", "producer:attempt-owner", "producer:attempt-reaper", "producer:recovery-coordinator", "producer:review-orchestrator", "producer:proposal-medium-adapter", "producer:evidence-owner", "producer:release-gate"} {
		if !validPurpose(value) {
			t.Errorf("validPurpose(%q) = false", value)
		}
	}
	for _, value := range []string{"", "producer:", "producer:root", "producer:legacy-translator", "operator:root"} {
		if validPurpose(value) {
			t.Errorf("validPurpose(%q) = true", value)
		}
	}

	for _, tc := range []struct {
		in   string
		want string
		ok   bool
	}{
		{in: ".pyxis/policy.json", want: ".pyxis/policy.json", ok: true},
		{in: " policy.json ", want: "policy.json", ok: true},
		{in: "", ok: false}, {in: "/absolute", ok: false}, {in: "a/../b", ok: false}, {in: "a\\b", ok: false}, {in: "a//b", ok: false},
	} {
		got, err := repositoryArtifactPath(tc.in)
		if (err == nil) != tc.ok || got != tc.want {
			t.Errorf("repositoryArtifactPath(%q) = %q, %v", tc.in, got, err)
		}
	}

	seed := sha256.Sum256([]byte("operator"))
	private := ed25519.NewKeyFromSeed(seed[:])
	publicHex := hex.EncodeToString(private.Public().(ed25519.PublicKey))
	if got, err := decodePublicKey(publicHex); err != nil || !bytes.Equal(got, private.Public().(ed25519.PublicKey)) {
		t.Fatalf("decodePublicKey = %x, %v", got, err)
	}
	for _, bad := range []string{"", strings.ToUpper(publicHex), "zz", publicHex[:len(publicHex)-2]} {
		if _, err := decodePublicKey(bad); err == nil {
			t.Errorf("decodePublicKey(%q) succeeded", bad)
		}
	}

	var decoded map[string]string
	if err := readClosedJSON(nil, &decoded); err == nil {
		t.Fatal("nil input path succeeded")
	}
	missing := filepath.Join(t.TempDir(), "missing.json")
	if err := readClosedJSON(&missing, &decoded); err == nil {
		t.Fatal("missing input succeeded")
	}
	path := writeReleaseFixture(t, t.TempDir(), "value.json", []byte(`{"value":"ok"}`))
	if err := readClosedJSON(&path, &decoded); err != nil || decoded["value"] != "ok" {
		t.Fatalf("readClosedJSON = %v, %v", decoded, err)
	}
	if got := string(trimOneNewline([]byte("x\n\n"))); got != "x\n" {
		t.Fatalf("trimOneNewline = %q", got)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func releasePolicyFixture(t *testing.T) (routingevidence.ProducerPolicy, ed25519.PrivateKey) {
	t.Helper()
	operatorSeed := sha256.Sum256([]byte("release-test-operator"))
	operatorPrivate := ed25519.NewKeyFromSeed(operatorSeed[:])
	policy := routingevidence.ProducerPolicy{Schema: routingevidence.ProducerPolicySchema, Version: "policy/v1", Repository: "owner/repo",
		EvidenceRef: "refs/pyxis/routing-evidence", ActivationPoint: "policy-point/1"}
	policy.Genesis = routingevidence.GenesisCheckpoint{Repository: policy.Repository, EvidenceRef: policy.EvidenceRef, Commit: "genesis-commit", Tree: "genesis-tree", AuthorizationReference: "operator:init"}
	policy.Authorizations = []routingevidence.AuthorizationReference{
		{ID: "auth-policy", Kind: routingevidence.AuthorizationPolicy},
		{ID: "auth-fence", Kind: routingevidence.AuthorizationRuntimeFence, Reference: "fence-proof"},
		{ID: "auth-native", Kind: routingevidence.AuthorizationNativeAuthor, Reference: "author-proof", ActorClass: "worker", ActorIdentity: "worker:test"},
		{ID: "auth-invalidation", Kind: routingevidence.AuthorizationInvalidation, Reference: "operator-decision"},
		{ID: "auth-activation", Kind: routingevidence.AuthorizationActivationManifest, ActorClass: "operator", ActorIdentity: "operator-key-1", PublicKey: hex.EncodeToString(operatorPrivate.Public().(ed25519.PublicKey))},
	}
	type eventRule struct {
		kind  routingevidence.EventKind
		class routingevidence.ProducerClass
		role  string
		auth  string
	}
	rules := []eventRule{
		{routingevidence.ExecutionDemandFrozen, routingevidence.ProducerPlanGate, "", "auth-policy"},
		{routingevidence.RoutingResolved, routingevidence.ProducerResolver, "implementation", "auth-policy"},
		{routingevidence.RoutingResolutionFailed, routingevidence.ProducerResolver, "implementation", "auth-policy"},
		{routingevidence.RoutedAttemptCompleted, routingevidence.ProducerAttemptOwner, "", "auth-policy"},
		{routingevidence.RoutedTaskOutcome, routingevidence.ProducerAttemptOwner, "", "auth-policy"},
		{routingevidence.AttemptOwnerTransferred, routingevidence.ProducerRecoveryCoordinator, "", "auth-fence"},
		{routingevidence.AssurancePassCompleted, routingevidence.ProducerReviewOrchestrator, "", "auth-policy"},
		{routingevidence.FindingPersisted, routingevidence.ProducerReviewOrchestrator, "", "auth-policy"},
		{routingevidence.DispositionRecorded, routingevidence.ProducerMediumAdapter, "worker", "auth-native"},
		{routingevidence.ReviewProcessCompleted, routingevidence.ProducerReviewOrchestrator, "", "auth-policy"},
		{routingevidence.RoutingEvidenceInvalidated, routingevidence.ProducerEvidenceOwner, "rerun_required", "auth-invalidation"},
		{routingevidence.ActivationCheckpointEvent, routingevidence.ProducerReleaseGate, "", "auth-activation"},
	}
	seen := map[routingevidence.ProducerClass]bool{}
	for _, rule := range rules {
		generation, keyID := string(rule.class)+"/v1", "key-"+string(rule.class)
		if !seen[rule.class] {
			seed := sha256.Sum256([]byte("release-test:" + rule.class))
			private := ed25519.NewKeyFromSeed(seed[:])
			policy.Keys = append(policy.Keys, routingevidence.KeyGeneration{Algorithm: "ed25519", ImplementationGeneration: generation, KeyID: keyID,
				PolicyActivationPoints: []string{policy.ActivationPoint}, ProducerClass: rule.class, PublicKey: hex.EncodeToString(private.Public().(ed25519.PublicKey))})
			seen[rule.class] = true
		}
		policy.Matrix = append(policy.Matrix, routingevidence.ProducerAuthorization{AuthorizationID: rule.auth, EventKind: rule.kind,
			ImplementationGeneration: generation, KeyID: keyID, ProducerClass: rule.class, Role: rule.role})
	}
	digest, err := routingevidence.DigestProducerPolicy(policy)
	if err != nil {
		t.Fatal(err)
	}
	policy.Digest = digest
	if err := routingevidence.ValidateProducerPolicy(policy); err != nil {
		t.Fatal(err)
	}
	return policy, operatorPrivate
}

func releaseManifestFixture() routingevidence.ActivationManifest {
	proofs := make([]routingevidence.ActivationProof, 0, 17)
	for i := range 6 {
		id := fmt.Sprintf("repo:proofs/runtime-%d.json", i)
		proofs = append(proofs, routingevidence.ActivationProof{Kind: routingevidence.ProofRuntimeGeneration, ID: id, Digest: releaseDigest(id)})
	}
	for _, kind := range []routingevidence.ActivationProofKind{routingevidence.ProofCatalog, routingevidence.ProofQualification, routingevidence.ProofEvidenceEvent,
		routingevidence.ProofConformance, routingevidence.ProofEvaluation, routingevidence.ProofDogfood, routingevidence.ProofDefinitionOfDone,
		routingevidence.ProofSeam, routingevidence.ProofFallback, routingevidence.ProofInvalidation, routingevidence.ProofReachability} {
		id := "repo:proofs/" + string(kind) + ".json"
		proofs = append(proofs, routingevidence.ActivationProof{Kind: kind, ID: id, Digest: releaseDigest(id)})
	}
	return routingevidence.ActivationManifest{Schema: routingevidence.ActivationManifestSchema, Repository: "owner/repo", EvidenceRef: "refs/pyxis/routing-evidence",
		EvidenceHead: "evidence-head", MainlineRef: "refs/heads/main", ExactMainlineObject: strings.Repeat("a", 40), ActivationPolicyVersion: "activation/v2",
		OperatorAuthorizationID: "auth-activation", Proofs: proofs, Predicate: routingevidence.ActivationPredicate{ActivationReady: true}}
}

func releaseDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func writeReleaseFixture(t *testing.T, dir, name string, wire []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, wire, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeReleaseJSON(t *testing.T, dir, name string, value any) string {
	t.Helper()
	wire, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return writeReleaseFixture(t, dir, name, wire)
}

var _ io.Writer = failingWriter{}
