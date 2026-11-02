package main

import (
	localcodehost "github.com/agenticlab-ai/pyxis/internal/adapters/codehost/localgit"
	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

func localCodeHostDiagnostics(root string, cfg config.Config) (codehost.Diagnostics, error) {
	options, selected, err := localCodeHostOptions(root, cfg)
	if err != nil || !selected {
		return nil, err
	}
	return localcodehost.NewDiagnostics(options), nil
}

func localCodeHostOptions(root string, cfg config.Config) (localcodehost.Options, bool, error) {
	profile, err := integrations.Load(root)
	if err != nil {
		return localcodehost.Options{}, false, err
	}
	if profile.Bindings.CodeHost == (integrations.Binding{}) {
		return localcodehost.Options{}, false, nil
	}
	binding, err := profile.CodeHost("")
	if err != nil {
		return localcodehost.Options{}, false, err
	}
	if binding.Provider != integrations.LocalGit {
		return localcodehost.Options{}, false, nil
	}
	return localcodehost.Options{Root: root, Origin: binding.Root, Remote: cfg.Remote, Mainline: cfg.Branch}, true, nil
}
