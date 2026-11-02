package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/entity"
)

const maxReviewCredentialBytes = 4 << 20

var (
	reviewAuthGOOS               = goruntime.GOOS
	reviewUserHomeDir            = os.UserHomeDir
	openReviewCredential         = os.Open
	readClaudeKeychainCredential = func() ([]byte, error) {
		return exec.Command("security", "find-generic-password", "-s", "Claude Code-credentials", "-w").Output()
	}
	readCursorKeychainCredential = func(ctx context.Context, service string) ([]byte, error) {
		secret, err := exec.CommandContext(ctx, "security", "find-generic-password", "-a", "cursor-user", "-s", "cursor-"+service, "-w").Output()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 44 {
			return nil, os.ErrNotExist
		}
		return secret, err
	}
)

// PrepareReviewAuthentication snapshots only the selected model CLI's login
// token file into disposable reviewer state. It deliberately excludes the
// user's settings, hooks, transcripts, caches, and code-host credentials.
// Missing files are allowed because a backend may authenticate through an
// explicitly allowlisted environment token or an OS credential provider.
func PrepareReviewAuthentication(ctx context.Context, runtimeStateDir string, harness entity.Harness) error {
	runtimeStateDir = strings.TrimSpace(runtimeStateDir)
	if runtimeStateDir == "" {
		return errors.New("review runtime authentication: empty state directory")
	}
	home, err := reviewUserHomeDir()
	if err != nil {
		return fmt.Errorf("review runtime authentication: resolve user home: %w", err)
	}

	type credential struct{ source, target string }
	var files []credential
	switch harness {
	case entity.Codex:
		sourceRoot := firstNonEmpty(os.Getenv("CODEX_HOME"), filepath.Join(home, ".codex"))
		files = []credential{{filepath.Join(sourceRoot, "auth.json"), filepath.Join(runtimeStateDir, "model-auth", "codex", "auth.json")}}
	case entity.Claude:
		sourceRoot := firstNonEmpty(os.Getenv("CLAUDE_CONFIG_DIR"), filepath.Join(home, ".claude"))
		target := filepath.Join(runtimeStateDir, "model-auth", "claude", ".credentials.json")
		source := filepath.Join(sourceRoot, ".credentials.json")
		if _, err := os.Stat(source); errors.Is(err, os.ErrNotExist) && reviewAuthGOOS == "darwin" {
			secret, keychainErr := readClaudeKeychainCredential()
			if keychainErr == nil {
				if err := writeReviewCredential(target, secret); err != nil {
					return err
				}
				return nil
			}
		}
		files = []credential{{source, target}}
	case entity.OpenCode:
		sourceRoot := firstNonEmpty(os.Getenv("XDG_DATA_HOME"), filepath.Join(home, ".local", "share"))
		files = []credential{{filepath.Join(sourceRoot, "opencode", "auth.json"), filepath.Join(runtimeStateDir, "model-auth", "xdg-data", "opencode", "auth.json")}}
	case entity.Cursor:
		// cli-config.json holds settings, not the login. Cursor uses Keychain on
		// macOS and auth.json in file-store mode. Reviews always use an isolated
		// file store so they can authenticate without inheriting user settings.
		if firstNonEmpty(os.Getenv("CURSOR_API_KEY"), os.Getenv("CURSOR_AUTH_TOKEN")) != "" {
			return nil
		}
		sourceRoot := filepath.Join(home, ".cursor")
		targetRoot := filepath.Join(runtimeStateDir, "home", ".cursor")
		switch reviewAuthGOOS {
		case "darwin":
			if os.Getenv("AGENT_CLI_CREDENTIAL_STORE") != "file" {
				ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
				defer cancel()
				return snapshotCursorKeychain(ctx, filepath.Join(targetRoot, "auth.json"))
			}
		case "windows":
			sourceRoot = filepath.Join(firstNonEmpty(os.Getenv("APPDATA"), filepath.Join(home, "AppData", "Roaming")), "Cursor")
			targetRoot = filepath.Join(runtimeStateDir, "home", "AppData", "Roaming", "Cursor")
		default:
			sourceRoot = filepath.Join(firstNonEmpty(os.Getenv("XDG_CONFIG_HOME"), filepath.Join(home, ".config")), "cursor")
			targetRoot = filepath.Join(runtimeStateDir, "xdg", "config", "cursor")
		}
		files = []credential{{filepath.Join(sourceRoot, "auth.json"), filepath.Join(targetRoot, "auth.json")}}
	default:
		return fmt.Errorf("review runtime authentication: unsupported harness %q", harness)
	}
	for _, file := range files {
		if err := copyReviewCredential(file.source, file.target); err != nil {
			return err
		}
	}
	return nil
}

func snapshotCursorKeychain(ctx context.Context, target string) error {
	var login struct {
		AccessToken  string `json:"accessToken,omitempty"`
		RefreshToken string `json:"refreshToken,omitempty"`
		APIKey       string `json:"apiKey,omitempty"`
	}
	for _, field := range []struct {
		service string
		target  *string
	}{{"access-token", &login.AccessToken}, {"refresh-token", &login.RefreshToken}, {"api-key", &login.APIKey}} {
		secret, err := readCursorKeychainCredential(ctx, field.service)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("review runtime authentication: read Cursor %s from Keychain: %w", field.service, err)
		}
		*field.target = strings.TrimSpace(string(secret))
	}
	if login.AccessToken == "" && login.RefreshToken == "" && login.APIKey == "" {
		return nil // An absent login is reported by the adapter's auth preflight.
	}
	data, err := json.Marshal(login)
	if err != nil {
		return fmt.Errorf("review runtime authentication: encode Cursor login: %w", err)
	}
	return writeReviewCredential(target, data)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func copyReviewCredential(source, target string) error {
	info, err := os.Stat(source)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("review runtime authentication: inspect credential: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("review runtime authentication: credential %s is not a regular file", source)
	}
	if info.Size() > maxReviewCredentialBytes {
		return fmt.Errorf("review runtime authentication: credential %s exceeds size limit", source)
	}
	in, err := openReviewCredential(source)
	if err != nil {
		return fmt.Errorf("review runtime authentication: open credential: %w", err)
	}
	defer func() { _ = in.Close() }() // release is best-effort; the operation result is already decided
	opened, err := in.Stat()
	if err != nil {
		return fmt.Errorf("review runtime authentication: inspect opened credential: %w", err)
	}
	if !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return fmt.Errorf("review runtime authentication: credential %s changed during validation", source)
	}
	var snapshot bytes.Buffer
	copied, copyErr := io.Copy(&snapshot, io.LimitReader(in, maxReviewCredentialBytes+1))
	if copyErr == nil && copied > maxReviewCredentialBytes {
		copyErr = errors.New("credential changed beyond size limit while copying")
	}
	if copyErr != nil {
		return fmt.Errorf("review runtime authentication: copy snapshot: %w", copyErr)
	}
	return writeReviewCredential(target, snapshot.Bytes())
}

func writeReviewCredential(target string, credential []byte) error {
	if len(credential) == 0 || len(credential) > maxReviewCredentialBytes {
		return errors.New("review runtime authentication: credential snapshot is empty or exceeds size limit")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return fmt.Errorf("review runtime authentication: create snapshot directory: %w", err)
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("review runtime authentication: create snapshot: %w", err)
	}
	_, writeErr := out.Write(credential)
	if syncErr := out.Sync(); writeErr == nil {
		writeErr = syncErr
	}
	if closeErr := out.Close(); writeErr == nil {
		writeErr = closeErr
	}
	if writeErr != nil {
		_ = os.Remove(target)
		return fmt.Errorf("review runtime authentication: write snapshot: %w", writeErr)
	}
	return nil
}
