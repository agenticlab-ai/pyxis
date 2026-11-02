// routing-release is the mechanical release-gate producer for semantic model
// routing. It provisions repository-scoped signing keys in the host credential
// store, signs closed activation inputs, and appends a checkpoint only after
// exact mainline and canonical evidence verification. It performs no inference.
package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	repoconfig "github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/routingevidence"
	"github.com/agenticlab-ai/pyxis/internal/routingevidence/credentialstore"
	"github.com/agenticlab-ai/pyxis/internal/routingevidence/gitstore"
)

var newCredentialStore = func() credentialstore.Store { return credentialstore.NewOSStore() }

func main() { os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr)) }

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return usage(stderr)
	}
	switch args[0] {
	case "keygen":
		return keygen(ctx, args[1:], stdout, stderr)
	case "sign-manifest":
		return signManifest(ctx, args[1:], stdout, stderr)
	case "sign-mainline":
		return signMainline(ctx, args[1:], stdout, stderr)
	case "checkpoint":
		return checkpoint(ctx, args[1:], stdout, stderr)
	case "help", "-h", "--help":
		return usage(stdout)
	default:
		diagnostic(stderr, "routing-release: unknown verb %q\n", args[0])
		return usage(stderr)
	}
}

func usage(w io.Writer) int {
	// Usage output is best effort; the command exit code remains authoritative.
	_, _ = fmt.Fprintln(w, `usage: routing-release <verb> ...
verbs:
  keygen --repository owner/name --purpose operator|producer:<class>
  sign-manifest --repository owner/name --policy policy.json --input unsigned.json
  sign-mainline --repository owner/name --policy policy.json --input unsigned.json
  checkpoint --repository owner/name --repo-dir path [--remote configured-name]
             --policy policy.json --manifest signed.json --mainline signed.json

Private keys remain in the operating-system credential store. Signed canonical
JSON and checkpoint proofs are written to stdout; diagnostics go to stderr.`)
	return 2
}

func keygen(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("keygen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	repository := fs.String("repository", "", "repository identity (owner/name)")
	purpose := fs.String("purpose", "", "operator or producer:<class>")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 {
		return 2
	}
	if !validRepository(*repository) || !validPurpose(*purpose) {
		diagnostic(stderr, "routing-release: keygen requires a valid repository and closed purpose\n")
		return 2
	}
	account, err := credentialstore.Account(*repository, *purpose)
	if err != nil {
		diagnostic(stderr, "routing-release: %v\n", err)
		return 1
	}
	key, err := credentialstore.Generate(ctx, newCredentialStore(), account, *purpose)
	if err != nil {
		diagnostic(stderr, "routing-release: keygen: %v\n", err)
		return 1
	}
	if err := json.NewEncoder(stdout).Encode(key); err != nil {
		diagnostic(stderr, "routing-release: encode key descriptor: %v\n", err)
		return 1
	}
	return 0
}

func signManifest(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("sign-manifest", flag.ContinueOnError)
	fs.SetOutput(stderr)
	repository := fs.String("repository", "", "repository identity")
	policyPath := fs.String("policy", "", "canonical producer policy")
	input := fs.String("input", "", "unsigned activation manifest")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 || !validRepository(*repository) || *policyPath == "" || *input == "" {
		return 2
	}
	policy, err := loadPolicy(*policyPath)
	if err != nil {
		diagnostic(stderr, "routing-release: read producer policy: %v\n", err)
		return 1
	}
	if policy.Repository != *repository {
		diagnostic(stderr, "routing-release: producer policy belongs to a different repository\n")
		return 1
	}
	var manifest routingevidence.ActivationManifest
	if err := readClosedJSON(input, &manifest); err != nil {
		diagnostic(stderr, "routing-release: read unsigned manifest: %v\n", err)
		return 1
	}
	auth, public, err := activationAuthorization(policy, manifest.OperatorAuthorizationID)
	if err != nil {
		diagnostic(stderr, "routing-release: activation authorization: %v\n", err)
		return 1
	}
	authKey, err := loadOperatorKey(ctx, newCredentialStore(), *repository, auth.ActorIdentity, public)
	if err != nil {
		diagnostic(stderr, "routing-release: operator key: %v\n", err)
		return 1
	}
	manifest, err = routingevidence.SignActivationManifest(manifest, authKey.KeyID, authKey.Private)
	clear(authKey.Private)
	if err != nil {
		diagnostic(stderr, "routing-release: sign manifest: %v\n", err)
		return 1
	}
	wire, err := routingevidence.EncodeActivationManifest(manifest)
	if err != nil {
		diagnostic(stderr, "routing-release: encode manifest: %v\n", err)
		return 1
	}
	if _, err := stdout.Write(append(wire, '\n')); err != nil {
		diagnostic(stderr, "routing-release: write signed manifest: %v\n", err)
		return 1
	}
	return 0
}

func signMainline(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("sign-mainline", flag.ContinueOnError)
	fs.SetOutput(stderr)
	repository := fs.String("repository", "", "repository identity")
	policyPath := fs.String("policy", "", "canonical producer policy")
	input := fs.String("input", "", "unsigned mainline proof")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 || !validRepository(*repository) || *policyPath == "" || *input == "" {
		return 2
	}
	policy, err := loadPolicy(*policyPath)
	if err != nil {
		diagnostic(stderr, "routing-release: read producer policy: %v\n", err)
		return 1
	}
	if policy.Repository != *repository {
		diagnostic(stderr, "routing-release: producer policy belongs to a different repository\n")
		return 1
	}
	var proof routingevidence.MainlineProof
	if err := readClosedJSON(input, &proof); err != nil {
		diagnostic(stderr, "routing-release: read unsigned mainline proof: %v\n", err)
		return 1
	}
	auth, public, err := activationAuthorization(policy, proof.OperatorAuthorizationID)
	if err != nil {
		diagnostic(stderr, "routing-release: activation authorization: %v\n", err)
		return 1
	}
	key, err := loadOperatorKey(ctx, newCredentialStore(), *repository, auth.ActorIdentity, public)
	if err != nil {
		diagnostic(stderr, "routing-release: operator key: %v\n", err)
		return 1
	}
	proof, err = routingevidence.SignMainlineProof(proof, key.KeyID, key.Private)
	clear(key.Private)
	if err != nil {
		diagnostic(stderr, "routing-release: sign mainline proof: %v\n", err)
		return 1
	}
	wire, err := routingevidence.EncodeMainlineProof(proof)
	if err != nil {
		diagnostic(stderr, "routing-release: encode mainline proof: %v\n", err)
		return 1
	}
	if _, err := stdout.Write(append(wire, '\n')); err != nil {
		diagnostic(stderr, "routing-release: write signed mainline proof: %v\n", err)
		return 1
	}
	return 0
}

func checkpoint(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("checkpoint", flag.ContinueOnError)
	fs.SetOutput(stderr)
	repository := fs.String("repository", "", "repository identity")
	repoDir := fs.String("repo-dir", "", "repository checkout")
	remote := fs.String("remote", "", "git remote (defaults to repository config)")
	policyPath := fs.String("policy", "", "canonical producer policy")
	manifestPath := fs.String("manifest", "", "signed activation manifest")
	mainlinePath := fs.String("mainline", "", "signed mainline proof")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 || !validRepository(*repository) || *repoDir == "" || *policyPath == "" || *manifestPath == "" || *mainlinePath == "" {
		return 2
	}
	if strings.TrimSpace(*remote) == "" {
		repoCfg, defects := repoconfig.Load(*repoDir)
		if len(defects) != 0 {
			diagnostic(stderr, "routing-release: repository config has %d defect(s)\n", len(defects))
			return 2
		}
		*remote = repoCfg.Remote
	}
	if strings.TrimSpace(*remote) == "" {
		diagnostic(stderr, "routing-release: repository config has no git remote\n")
		return 2
	}
	policyArtifact, err := repositoryArtifactPath(*policyPath)
	if err != nil {
		diagnostic(stderr, "routing-release: producer policy path: %v\n", err)
		return 2
	}
	policyWire, err := os.ReadFile(filepath.Join(*repoDir, filepath.FromSlash(policyArtifact)))
	if err != nil {
		diagnostic(stderr, "routing-release: read producer policy: %v\n", err)
		return 1
	}
	policy, err := routingevidence.DecodeProducerPolicy(trimOneNewline(policyWire))
	if err != nil {
		diagnostic(stderr, "routing-release: producer policy: %v\n", err)
		return 1
	}
	if policy.Repository != *repository {
		diagnostic(stderr, "routing-release: producer policy belongs to a different repository\n")
		return 1
	}
	manifestWire, err := os.ReadFile(*manifestPath)
	if err != nil {
		diagnostic(stderr, "routing-release: read manifest: %v\n", err)
		return 1
	}
	manifest, err := routingevidence.DecodeActivationManifest(trimOneNewline(manifestWire))
	if err != nil {
		diagnostic(stderr, "routing-release: decode manifest: %v\n", err)
		return 1
	}
	mainlineWire, err := os.ReadFile(*mainlinePath)
	if err != nil {
		diagnostic(stderr, "routing-release: read mainline proof: %v\n", err)
		return 1
	}
	mainline, err := routingevidence.DecodeMainlineProof(trimOneNewline(mainlineWire))
	if err != nil {
		diagnostic(stderr, "routing-release: decode mainline proof: %v\n", err)
		return 1
	}
	if manifest.Repository != *repository || mainline.Repository != *repository || manifest.EvidenceRef != policy.EvidenceRef {
		diagnostic(stderr, "routing-release: manifest, mainline proof, and producer policy cross repository or evidence ref\n")
		return 1
	}
	remoteObject, err := gitstore.ReadRemoteRef(ctx, *repoDir, *remote, mainline.MainlineRef)
	if err != nil || remoteObject != mainline.CurrentObject {
		diagnostic(stderr, "routing-release: exact remote mainline differs from signed proof: object=%q err=%v\n", remoteObject, err)
		return 1
	}
	store, err := gitstore.New(*repoDir, *remote, policy)
	if err != nil {
		diagnostic(stderr, "routing-release: evidence store: %v\n", err)
		return 1
	}
	artifacts, err := gitstore.NewArtifactReader(*repoDir)
	if err != nil {
		diagnostic(stderr, "routing-release: artifact reader: %v\n", err)
		return 1
	}
	if err := bindProducerPolicyArtifact(ctx, artifacts, manifest, policyArtifact, policyWire); err != nil {
		diagnostic(stderr, "routing-release: producer policy proof: %v\n", err)
		return 1
	}
	releaseKey, err := credentialstore.LoadProducerCapability(ctx, newCredentialStore(), policy, routingevidence.ProducerReleaseGate,
		[]routingevidence.EventKind{routingevidence.ActivationCheckpointEvent}, []string{""})
	if err != nil {
		diagnostic(stderr, "routing-release: release-gate key: %v\n", err)
		return 1
	}
	defer clear(releaseKey.PrivateKey)
	service := routingevidence.ActivationService{Reader: store, Reducer: routingevidence.NewReducer(policy), Store: store, Artifacts: artifacts,
		Appender: routingevidence.SignedCheckpointAppender{Store: store, Capability: releaseKey}}
	proof, err := service.Activate(ctx, manifest, mainline)
	if err != nil {
		diagnostic(stderr, "routing-release: checkpoint refused: %v\n", err)
		return 1
	}
	if err := json.NewEncoder(stdout).Encode(proof); err != nil {
		diagnostic(stderr, "routing-release: encode checkpoint proof: %v\n", err)
		return 1
	}
	return 0
}

type loadedKey struct {
	KeyID   string
	Private ed25519.PrivateKey
}

func loadOperatorKey(ctx context.Context, store credentialstore.Store, repository, keyID string, public ed25519.PublicKey) (loadedKey, error) {
	account, err := credentialstore.Account(repository, "operator")
	if err != nil {
		return loadedKey{}, err
	}
	if keyID == "" || len(public) == 0 {
		return loadedKey{}, errors.New("operator key requires a producer-policy activation authorization")
	}
	private, err := credentialstore.Load(ctx, store, credentialstore.Key{Account: account, KeyID: keyID, PublicKey: fmt.Sprintf("%x", public)})
	return loadedKey{KeyID: keyID, Private: private}, err
}

func bindProducerPolicyArtifact(ctx context.Context, reader routingevidence.ActivationArtifactReader, manifest routingevidence.ActivationManifest, path string, worktree []byte) error {
	exact, err := reader.ReadActivationArtifact(ctx, manifest.ExactMainlineObject, path)
	if err != nil {
		return err
	}
	if !bytes.Equal(exact, worktree) {
		return errors.New("worktree policy differs from the exact candidate-mainline policy")
	}
	digest := sha256.Sum256(exact)
	wantID, wantDigest := "repo:"+path, hex.EncodeToString(digest[:])
	for _, proof := range manifest.Proofs {
		if proof.ID == wantID && proof.Digest == wantDigest && proof.Kind != routingevidence.ProofEvidenceEvent {
			return nil
		}
	}
	return errors.New("manifest does not pin the exact producer policy artifact")
}

func repositoryArtifactPath(path string) (string, error) {
	path = filepath.ToSlash(strings.TrimSpace(path))
	if path == "" || filepath.IsAbs(path) || filepath.Clean(filepath.FromSlash(path)) != filepath.FromSlash(path) || strings.Contains(path, "\\") {
		return "", errors.New("path must be clean and repository-relative")
	}
	for component := range strings.SplitSeq(path, "/") {
		if component == "" || component == "." || component == ".." {
			return "", errors.New("path must be clean and repository-relative")
		}
	}
	return path, nil
}

func activationAuthorization(policy routingevidence.ProducerPolicy, id string) (routingevidence.AuthorizationReference, ed25519.PublicKey, error) {
	for _, auth := range policy.Authorizations {
		if auth.ID == id && auth.Kind == routingevidence.AuthorizationActivationManifest {
			public, err := decodePublicKey(auth.PublicKey)
			return auth, public, err
		}
	}
	return routingevidence.AuthorizationReference{}, nil, errors.New("operator activation authorization is absent")
}

func decodePublicKey(value string) (ed25519.PublicKey, error) {
	b, err := hex.DecodeString(value)
	if err != nil || len(b) != ed25519.PublicKeySize || hex.EncodeToString(b) != value {
		return nil, errors.New("operator public key is not lowercase hex")
	}
	return ed25519.PublicKey(b), nil
}

func loadPolicy(path string) (routingevidence.ProducerPolicy, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return routingevidence.ProducerPolicy{}, err
	}
	return routingevidence.DecodeProducerPolicy(trimOneNewline(b))
}

func readClosedJSON(path *string, out any) error {
	if path == nil || *path == "" {
		return errors.New("input path is required")
	}
	b, err := os.ReadFile(*path)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return errors.New("input contains trailing JSON")
	}
	return nil
}

func validRepository(value string) bool {
	parts := strings.Split(value, "/")
	return len(parts) == 2 && parts[0] != "" && parts[1] != "" && !strings.ContainsAny(value, "\x00\r\n \t")
}

func validPurpose(value string) bool {
	if value == "operator" {
		return true
	}
	if !strings.HasPrefix(value, "producer:") {
		return false
	}
	class := routingevidence.ProducerClass(strings.TrimPrefix(value, "producer:"))
	return slices.Contains([]routingevidence.ProducerClass{
		routingevidence.ProducerPlanGate, routingevidence.ProducerResolver,
		routingevidence.ProducerAttemptOwner, routingevidence.ProducerAttemptReaper, routingevidence.ProducerRecoveryCoordinator,
		routingevidence.ProducerReviewOrchestrator, routingevidence.ProducerMediumAdapter, routingevidence.ProducerEvidenceOwner,
		routingevidence.ProducerReleaseGate,
	}, class)
}

func trimOneNewline(b []byte) []byte {
	return []byte(strings.TrimSuffix(string(b), "\n"))
}

func diagnostic(w io.Writer, format string, args ...any) {
	// Diagnostics are best effort; callers return the authoritative error status.
	_, _ = fmt.Fprintf(w, format, args...)
}
