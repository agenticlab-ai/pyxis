package runtime

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// LaunchEnvironment builds a closed environment. A harness receives ordinary
// process plumbing plus the selected adapter's explicitly allowlisted model
// authentication keys; it never inherits HOME, code-host/SSH credentials, git
// credential helpers, or canonical-store locations by default.
func LaunchEnvironment(spec port.LaunchSpec, modelAuthKeys ...string) []string {
	allowed := map[string]bool{
		"PATH": true, "TMPDIR": true, "TMP": true, "TEMP": true,
		"LANG": true, "TERM": true, "COLORTERM": true, "NO_COLOR": true,
		"SSL_CERT_FILE": true, "SSL_CERT_DIR": true,
		"HTTP_PROXY": true, "HTTPS_PROXY": true, "ALL_PROXY": true, "NO_PROXY": true,
		"http_proxy": true, "https_proxy": true, "all_proxy": true, "no_proxy": true,
	}
	for _, key := range modelAuthKeys {
		if key = strings.TrimSpace(key); key != "" {
			allowed[key] = true
		}
	}
	// Directory-valued login overrides are never inherited. The invoker copies
	// only the backend's credential file into disposable runtime state and these
	// variables point there. Inheriting the ambient directory would expose
	// writable settings, hooks, transcripts, and unrelated credentials.
	redirected := map[string]bool{
		"CODEX_HOME": true, "CLAUDE_CONFIG_DIR": true, "XDG_DATA_HOME": true,
	}
	env := make([]string, 0, len(allowed)+12)
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if key == "TMPDIR" || key == "TMP" || key == "TEMP" {
			continue
		}
		if allowed[key] && !redirected[key] {
			env = append(env, item)
		}
	}
	stateDir := strings.TrimSpace(spec.RuntimeStateDir)
	if stateDir == "" {
		// Direct adapter conformance fixtures predate RuntimeStateDir. Keep their
		// review environment writable without placing state in the read-only tree;
		// the production invoker always supplies and cleans a unique directory.
		stateDir = spec.Workdir + "-runtime"
	}
	if allowed["CODEX_HOME"] {
		env = append(env, "CODEX_HOME="+filepath.Join(stateDir, "model-auth", "codex"))
	}
	if allowed["CLAUDE_CONFIG_DIR"] {
		env = append(env, "CLAUDE_CONFIG_DIR="+filepath.Join(stateDir, "model-auth", "claude"))
	}
	if allowed["XDG_DATA_HOME"] {
		env = append(env, "XDG_DATA_HOME="+filepath.Join(stateDir, "model-auth", "xdg-data"))
	}
	if allowed["CURSOR_API_KEY"] {
		dataDir := filepath.Join(stateDir, "cursor-data")
		// Cursor falls back to /tmp/.cursor when its socket path is too long.
		// A relative path keeps that state inside the supervisor-owned directory.
		if relative, err := filepath.Rel(spec.Workdir, dataDir); err == nil {
			dataDir = relative
		}
		env = append(env, "AGENT_CLI_CREDENTIAL_STORE=file", "NODE_DISABLE_COMPILE_CACHE=1",
			"CURSOR_DATA_DIR="+dataDir,
			"APPDATA="+filepath.Join(stateDir, "home", "AppData", "Roaming"))
	}
	return append(env,
		"TMPDIR="+filepath.Join(stateDir, "tmp"),
		"TMP="+filepath.Join(stateDir, "tmp"),
		"TEMP="+filepath.Join(stateDir, "tmp"),
		"GIT_OPTIONAL_LOCKS=0",
		"PWD="+spec.Workdir,
		"HOME="+filepath.Join(stateDir, "home"),
		"GH_CONFIG_DIR="+filepath.Join(stateDir, "no-codehost-credentials"),
		"XDG_CONFIG_HOME="+filepath.Join(stateDir, "xdg", "config"),
		"XDG_CACHE_HOME="+filepath.Join(stateDir, "xdg", "cache"),
		"XDG_STATE_HOME="+filepath.Join(stateDir, "xdg", "state"),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_TERMINAL_PROMPT=0",
	)
}
