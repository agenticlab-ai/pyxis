package main

import (
	"bytes"
	"crypto/ed25519"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/routingevidence"
	"github.com/agenticlab-ai/pyxis/internal/routingevidence/credentialstore"
)

func TestCoverage90KeyAndSigningFailures(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := keygen(t.Context(), []string{"--repository", "owner#/repo", "--purpose", "operator"}, &stdout, &stderr); code != 1 {
		t.Fatalf("keygen = %d, want 1; stderr=%q", code, stderr.String())
	}
	if _, err := loadOperatorKey(t.Context(), &commandCredentialStore{}, "bad#repo", "key", make(ed25519.PublicKey, ed25519.PublicKeySize)); err == nil {
		t.Fatal("loadOperatorKey accepted an invalid repository")
	}
	if _, err := loadOperatorKey(t.Context(), &commandCredentialStore{}, "owner/repo", "", nil); err == nil {
		t.Fatal("loadOperatorKey accepted a missing authorization")
	}

	policy, private := releasePolicyFixture(t)
	policyWire, err := routingevidence.EncodeProducerPolicy(policy)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	policyPath := writeReleaseFixture(t, dir, "policy.json", policyWire)
	store := &commandCredentialStore{}
	account, err := credentialstore.Account(policy.Repository, "operator")
	if err != nil {
		t.Fatal(err)
	}
	store.values = map[string][]byte{account: append([]byte(nil), private.Seed()...)}
	prior := newCredentialStore
	newCredentialStore = func() credentialstore.Store { return store }
	t.Cleanup(func() { newCredentialStore = prior })

	manifest := releaseManifestFixture()
	manifest.EvidenceHead = ""
	manifestPath := writeReleaseJSON(t, dir, "invalid-manifest.json", manifest)
	stdout.Reset()
	stderr.Reset()
	if code := signManifest(t.Context(), []string{"--repository", policy.Repository, "--policy", policyPath, "--input", manifestPath}, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "sign manifest") {
		t.Fatalf("signManifest = %d, stderr=%q", code, stderr.String())
	}

	mainline := routingevidence.MainlineProof{
		Schema:                  routingevidence.MainlineProofSchema,
		Repository:              policy.Repository,
		OperatorAuthorizationID: "auth-activation",
	}
	mainlinePath := writeReleaseJSON(t, dir, "invalid-mainline.json", mainline)
	stdout.Reset()
	stderr.Reset()
	if code := signMainline(t.Context(), []string{"--repository", policy.Repository, "--policy", policyPath, "--input", mainlinePath}, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "sign mainline proof") {
		t.Fatalf("signMainline = %d, stderr=%q", code, stderr.String())
	}
}

func TestCoverage90CheckpointDefaultsRemoteAndRejectsCrossRepository(t *testing.T) {
	t.Run("default remote", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		repo := t.TempDir()
		code := checkpoint(t.Context(), []string{
			"--repository", "owner/repo", "--repo-dir", repo,
			"--policy", "missing.json", "--manifest", "manifest.json", "--mainline", "mainline.json",
		}, &stdout, &stderr)
		if code != 1 || !strings.Contains(stderr.String(), "read producer policy") {
			t.Fatalf("checkpoint = %d, stderr=%q", code, stderr.String())
		}
	})

	policy, private := releasePolicyFixture(t)
	policyWire, err := routingevidence.EncodeProducerPolicy(policy)
	if err != nil {
		t.Fatal(err)
	}
	repo := t.TempDir()
	writeReleaseFixture(t, repo, "policy.json", policyWire)

	manifest := releaseManifestFixture()
	manifest.Repository = "other/repo"
	manifest, err = routingevidence.SignActivationManifest(manifest, "operator-key-1", private)
	if err != nil {
		t.Fatal(err)
	}
	manifestWire, err := routingevidence.EncodeActivationManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := writeReleaseFixture(t, repo, "manifest.json", manifestWire)

	mainline := routingevidence.MainlineProof{
		Schema:                  routingevidence.MainlineProofSchema,
		Repository:              policy.Repository,
		MainlineRef:             manifest.MainlineRef,
		ExactMainlineObject:     manifest.ExactMainlineObject,
		CurrentObject:           manifest.ExactMainlineObject,
		HumanMerged:             true,
		HumanMergeActor:         "human:operator",
		OperatorAuthorizationID: manifest.OperatorAuthorizationID,
		Proofs:                  append([]routingevidence.ActivationProof(nil), manifest.Proofs...),
	}
	digest, err := routingevidence.DigestActivationManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	mainline.ManifestDigest = digest
	mainline, err = routingevidence.SignMainlineProof(mainline, "operator-key-1", private)
	if err != nil {
		t.Fatal(err)
	}
	mainlinePath := writeReleaseJSON(t, repo, "mainline.json", mainline)

	var stdout, stderr bytes.Buffer
	code := checkpoint(t.Context(), []string{
		"--repository", policy.Repository, "--repo-dir", repo, "--remote", "origin",
		"--policy", filepath.Base("policy.json"), "--manifest", manifestPath, "--mainline", mainlinePath,
	}, &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "cross repository") {
		t.Fatalf("checkpoint = %d, stderr=%q", code, stderr.String())
	}
}
