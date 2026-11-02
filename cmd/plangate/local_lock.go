package main

import (
	"context"
	"fmt"

	localcodehost "github.com/agenticlab-ai/pyxis/internal/adapters/codehost/localgit"
	draftshell "github.com/agenticlab-ai/pyxis/internal/adapters/draft/shell"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/draft/lifecycle"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/plangate/precheck"
)

func configuredPlanningPrecheck(ctx context.Context, root, feature string, run lifecycle.RunCommand) (string, []schema.Defect) {
	if handled, defects := precheck.RunDocument(root, feature); handled {
		return "", defects
	}
	refuse := func(err error) (string, []schema.Defect) {
		return "", []schema.Defect{{Verb: "plangate", File: feature, Op: "precheck/local-code-host", Reason: err.Error(), Remedy: "repair the configured local origin or retained lock and retry"}}
	}
	profile, err := integrations.Load(root)
	if err != nil {
		return refuse(err)
	}
	if profile.Bindings.CodeHost == (integrations.Binding{}) {
		return precheck.Run(ctx, root, feature, run)
	}
	binding, err := profile.CodeHost("")
	if err != nil {
		return refuse(err)
	}
	if binding.Provider != integrations.LocalGit {
		return precheck.Run(ctx, root, feature, run)
	}
	cfg, defects := config.Load(root)
	if len(defects) != 0 {
		return "", defects
	}
	options := localcodehost.Options{Root: root, Origin: binding.Root, Remote: cfg.Remote, Mainline: cfg.Branch}
	origin, err := localcodehost.NewDiagnostics(options).Inspect(ctx)
	if err != nil {
		return refuse(err)
	}
	if origin.MainlineOID == "" {
		return refuse(fmt.Errorf("%s is absent; %s", origin.MainlineRef, origin.MainlineRemedy))
	}
	options.Origin = origin.Origin
	return precheck.RunWithLocalLock(ctx, root, feature, origin.Origin, draftshell.RunLocalGit, localcodehost.NewDraftLandings(options))
}
