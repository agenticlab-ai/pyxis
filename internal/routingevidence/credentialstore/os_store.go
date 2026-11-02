package credentialstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

type commandRunner interface {
	Run(context.Context, []byte, string, ...string) ([]byte, error)
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, stdin []byte, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = bytes.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// OSStore uses the host's native credential service. Secrets are passed only
// on the child's stdin; they never appear in argv or the inherited environment.
type OSStore struct {
	goos   string
	runner commandRunner
}

func NewOSStore() *OSStore { return &OSStore{goos: runtime.GOOS, runner: execRunner{}} }

func newOSStore(goos string, runner commandRunner) *OSStore {
	return &OSStore{goos: goos, runner: runner}
}

func (s *OSStore) Put(ctx context.Context, account string, secret []byte) error {
	if s == nil || s.runner == nil || strings.TrimSpace(account) == "" || len(secret) == 0 {
		return errors.New("routing credential store: account and secret are required")
	}
	encoded := []byte(hexSecret(secret) + "\n")
	defer clear(encoded)
	var err error
	switch s.goos {
	case "darwin":
		// `-w` as the final option makes security read the password from its
		// prompt channel. With a non-terminal stdin it consumes the supplied
		// line, avoiding the documented-insecure `-w password` argv form.
		_, err = s.runner.Run(ctx, encoded, "security", "add-generic-password", "-U", "-s", Service, "-a", account, "-w")
	case "linux":
		_, err = s.runner.Run(ctx, encoded, "secret-tool", "store", "--label=Pyxis routing evidence", "service", Service, "account", account)
	default:
		return fmt.Errorf("%w: OS credential store is unsupported on %s", ErrCredentialUnavailable, s.goos)
	}
	if err != nil {
		return fmt.Errorf("%w: store %s credential: %v", ErrCredentialUnavailable, s.goos, err)
	}
	return nil
}

func (s *OSStore) Get(ctx context.Context, account string) ([]byte, error) {
	if s == nil || s.runner == nil || strings.TrimSpace(account) == "" {
		return nil, errors.New("routing credential store: account is required")
	}
	var out []byte
	var err error
	switch s.goos {
	case "darwin":
		out, err = s.runner.Run(ctx, nil, "security", "find-generic-password", "-s", Service, "-a", account, "-w")
	case "linux":
		out, err = s.runner.Run(ctx, nil, "secret-tool", "lookup", "service", Service, "account", account)
	default:
		return nil, fmt.Errorf("%w: OS credential store is unsupported on %s", ErrCredentialUnavailable, s.goos)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: read %s credential: %v", ErrCredentialUnavailable, s.goos, err)
	}
	secret, decodeErr := decodeHexSecret(strings.TrimSpace(string(out)))
	clear(out)
	if decodeErr != nil {
		return nil, fmt.Errorf("%w: stored value is malformed", ErrCredentialUnavailable)
	}
	return secret, nil
}

func hexSecret(secret []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, len(secret)*2)
	for i, b := range secret {
		out[2*i], out[2*i+1] = digits[b>>4], digits[b&15]
	}
	return string(out)
}

func decodeHexSecret(value string) ([]byte, error) {
	if len(value)%2 != 0 || value == "" {
		return nil, errors.New("invalid hex length")
	}
	out := make([]byte, len(value)/2)
	for i := range out {
		hi, ok1 := hexNibble(value[2*i])
		lo, ok2 := hexNibble(value[2*i+1])
		if !ok1 || !ok2 {
			clear(out)
			return nil, errors.New("invalid hex byte")
		}
		out[i] = hi<<4 | lo
	}
	return out, nil
}

func hexNibble(b byte) (byte, bool) {
	switch {
	case b >= '0' && b <= '9':
		return b - '0', true
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10, true
	default:
		return 0, false
	}
}
