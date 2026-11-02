package release

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/payload"
)

// MaterializeSourceWorkflows is an explicit source-repository operation. The
// installed-repository gate is rendered separately by converge. Existing
// workflow content is preserved; changed templates need a reviewed replacement.
func MaterializeSourceWorkflows(root string) ([]string, error) {
	profile, err := integrations.Load(root)
	if err != nil {
		return nil, err
	}
	if profile.Bindings.CodeHost == (integrations.Binding{}) || profile.Bindings.Releases.Source == (integrations.Binding{}) {
		return nil, fmt.Errorf("release: source workflows require explicit code_host and releases.source bindings")
	}
	host, err := profile.CodeHost("")
	if err != nil {
		return nil, err
	}
	if host.Provider != integrations.Forgejo {
		return nil, fmt.Errorf("release: Forgejo workflows require a Forgejo code_host binding")
	}
	source, err := profile.Release("source", "")
	if err != nil {
		return nil, err
	}
	repo, defects := config.Load(root)
	if len(defects) > 0 {
		return nil, fmt.Errorf("release: source configuration has %d defects", len(defects))
	}
	mode := profile.CI.Mode
	if mode == "" {
		mode = repo.Knobs.CIMode
	}
	if mode != "actions" && mode != "both" {
		return nil, fmt.Errorf("release: source workflows require CI mode actions or both")
	}
	label := profile.CI.RunnerLabel
	if label == "" {
		label = "pyxis-linux"
	}
	for _, v := range []string{repo.Branch, repo.Remote, label} {
		if v == "" || strings.ContainsAny(v, " \t\r\n;$`|&<>\\'\"[]{}") {
			return nil, fmt.Errorf("release: unsafe workflow branch, remote, or runner label")
		}
	}
	tokenEnv := source.TokenEnv
	if tokenEnv == "" {
		tokenEnv = "GH_TOKEN"
	}
	if !WorkflowCredentialAllowed(tokenEnv) {
		return nil, fmt.Errorf("release: token_env conflicts with a reserved workflow setting")
	}
	// Independent targets may share one credential reference intentionally.
	// Avoid duplicate YAML keys; in that case the source secret supplies both.
	distributionEnv := "PYXIS_UNUSED_DISTRIBUTION_TOKEN"
	distributionEnabled := "false"
	if profile.Bindings.Releases.Distribution != (integrations.Binding{}) {
		dist, err := profile.Release("distribution", "")
		if err != nil {
			return nil, err
		}
		distributionEnabled = "true"
		distEnv := dist.TokenEnv
		if distEnv == "" {
			distEnv = "GH_TOKEN"
		}
		if !WorkflowCredentialAllowed(distEnv) {
			return nil, fmt.Errorf("release: distribution token_env conflicts with a workflow setting")
		}
		if distEnv != tokenEnv {
			distributionEnv = distEnv
		}
	}
	for _, dir := range []string{".forgejo", ".forgejo/workflows"} {
		st, err := os.Lstat(filepath.Join(root, dir))
		if err == nil && (!st.IsDir() || st.Mode()&os.ModeSymlink != 0) {
			return nil, fmt.Errorf("release: workflow directory %s is not an ordinary directory", dir)
		}
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	templates := filepath.Join(root, ".pyxis/dist/forgejo-source-workflows")
	entries, err := os.ReadDir(templates)
	if err != nil {
		return nil, err
	}
	pending := map[string][]byte{}
	var paths []string
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".yml") {
			continue
		}
		// This captured source stays in Git until memory retirement can seal it,
		// but the duplicate workflow must not be installed into source CI.
		if entry.Name() == "memory-reconciliation.yml" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(templates, entry.Name()))
		if err != nil {
			return nil, err
		}
		if distributionEnv == "PYXIS_UNUSED_DISTRIBUTION_TOKEN" {
			// No binding, or a shared credential, needs no second secret.
			data = bytes.ReplaceAll(data, []byte("      {{distribution_token_env}}: ${{ secrets.PYXIS_DISTRIBUTION_RELEASE_TOKEN }}\n"), nil)
		}
		data, err = payload.RenderForgejoWorkflow(data, map[string]string{"branch": repo.Branch, "remote": repo.Remote, "runner_label": label, "release_token_env": tokenEnv, "distribution_token_env": distributionEnv, "distribution_enabled": distributionEnabled})
		if err != nil {
			return nil, err
		}
		path := filepath.Join(root, ".forgejo/workflows", entry.Name())
		old, err := os.ReadFile(path)
		switch {
		case err == nil:
			if !bytes.Equal(old, data) {
				return nil, fmt.Errorf("release: existing workflow %s differs; preserve it and review the template before replacing", path)
			}
		case os.IsNotExist(err):
			pending[path] = data
		default:
			return nil, err
		}
		paths = append(paths, path)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("release: no Forgejo source workflow templates in %s", templates)
	}
	for _, path := range paths {
		if data, ok := pending[path]; ok {
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return nil, err
			}
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
			if err != nil {
				return nil, err
			}
			_, writeErr := f.Write(data)
			closeErr := f.Close()
			if writeErr != nil {
				return nil, writeErr
			}
			if closeErr != nil {
				return nil, closeErr
			}
		}
	}
	return paths, nil
}

// WorkflowCredentialAllowed excludes environment keys the generated workflow
// controls itself. The CLI rechecks this at execution because configuration
// can change after a workflow was materialized.
func WorkflowCredentialAllowed(name string) bool {
	switch name {
	case "PYXIS_RELEASE_TAG", "PYXIS_UNUSED_DISTRIBUTION_TOKEN":
		return false
	default:
		return true
	}
}
