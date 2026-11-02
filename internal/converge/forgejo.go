package converge

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/payload"
	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
)

func workflowMode(root string, repo config.Config) config.Config {
	if c, err := integrations.Load(root); err == nil && c.CI.Mode != "" {
		repo.Knobs.CIMode = c.CI.Mode
	}
	return repo
}

// ProjectWorkflowManifest locates the one generated workflow on the selected
// code host. The signed manifest is never rewritten; this is the same
// consumption-time projection for converge, doctor, and adoption.
func ProjectWorkflowManifest(root string, m relmanifest.Manifest, profiles ...*integrations.Config) (relmanifest.Manifest, error) {
	_, target, err := payload.WorkflowForRoot(root, profiles...)
	if err != nil {
		return m, err
	}
	if target == payload.InstalledWorkflowTarget {
		return m, nil
	}
	m.Payload.Files = slices.Clone(m.Payload.Files)
	for i := range m.Payload.Files {
		if m.Payload.Files[i].Path == payload.InstalledWorkflowTarget {
			m.Payload.Files[i].Path = target
		}
	}
	return m, nil
}

func desiredForgejoWorkflow(root string, repo config.Config, m relmanifest.Manifest, archive string, profiles ...*integrations.Config) ([]byte, string) {
	var tmpl []byte
	if archive != "" {
		files, err := extractFiles(archive, map[string]bool{payload.ForgejoWorkflow: true})
		if err != nil {
			return nil, err.Error()
		}
		entry, ok := files[payload.ForgejoWorkflow]
		if !ok {
			return nil, "verified release does not contain the Forgejo workflow template"
		}
		tmpl = entry.b
	} else {
		b, err := os.ReadFile(treePath(root, payload.ForgejoWorkflow))
		if err != nil {
			return nil, err.Error()
		}
		tmpl = b
	}
	var ci integrations.Config
	var err error
	var override *integrations.Config
	if len(profiles) > 0 {
		override = profiles[0]
	}
	if override != nil {
		ci = *override
	} else {
		ci, err = integrations.Load(root)
	}
	if err != nil {
		return nil, err.Error()
	}
	label := ci.CI.RunnerLabel
	if label == "" {
		label = "pyxis-linux"
	}
	checksums := m.Binaries["pyxis"]
	if checksums["linux-arm64"] == "" || checksums["linux-amd64"] == "" {
		return nil, "verified release manifest must pin both Linux pyxis binaries for the Forgejo workflow"
	}
	source, _ := resolve.EffectiveSource(resolve.Config{Root: root, Repo: repo, Integrations: override}, m.SystemVersion)
	tokenEnv := "PYXIS_FORGEJO_TOKEN"
	if ar, found, err := ci.Artifacts(); err != nil {
		return nil, err.Error()
	} else if found && ar.TokenEnv != "" {
		tokenEnv = ar.TokenEnv
	}
	if tokenEnv == "PYXIS_ARTIFACT_TOKEN" {
		tmpl = []byte(strings.ReplaceAll(string(tmpl), "      {{artifact_token_env}}: ${{ secrets.PYXIS_ARTIFACT_TOKEN }}\n", ""))
	} else if tokenEnv == "PYXIS_VERSION" || tokenEnv == "PYXIS_ARTIFACT_SOURCE" || strings.HasPrefix(tokenEnv, "PYXIS_SHA256_") {
		return nil, "artifact token_env conflicts with a reserved workflow setting"
	}
	// shell operands are intentionally restricted; configuration cannot append a
	// command to the job's git fetch or merge-base invocation.
	for _, v := range []string{repo.Branch, repo.Remote, label} {
		if strings.ContainsAny(v, " \t\r\n;$`|&<>\\'\"") {
			return nil, "workflow branch, remote, and runner label must be plain identifiers"
		}
	}
	out, err := payload.RenderForgejoWorkflow(tmpl, map[string]string{"branch": repo.Branch, "remote": repo.Remote, "runner_label": label, "version": m.SystemVersion, "linux_arm_checksum": checksums["linux-arm64"], "linux_amd_checksum": checksums["linux-amd64"], "artifact_source": source, "artifact_token_env": tokenEnv})
	if err != nil {
		return nil, fmt.Sprintf("render Forgejo workflow: %v", err)
	}
	return out, ""
}

func configuredFindings(findings []Finding, cfg resolve.Config) []Finding {
	if cfg.Integrations == nil {
		return desiredFindings(findings, cfg.Repo, cfg.Root)
	}
	repo := cfg.Repo
	if cfg.Integrations.CI.Mode != "" {
		repo.Knobs.CIMode = cfg.Integrations.CI.Mode
	}
	return desiredFindings(findings, repo)
}
