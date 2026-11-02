package release

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func legacyVersion(tool, tag string) (string, error) {
	if tool != "docgate" && tool != "codegate" {
		return "", fmt.Errorf("release: legacy tool must be docgate or codegate")
	}
	version, ok := strings.CutPrefix(tag, tool+"/")
	if !ok {
		return "", fmt.Errorf("release: legacy tag must begin %s/", tool)
	}
	if err := checkVersion(version); err != nil {
		return "", err
	}
	return version, nil
}

// BuildLegacy retains the two pre-system tag channels using the same static
// compiler flags and checksum writer as a unified system release.
func BuildLegacy(ctx context.Context, tool, tag, root, out string) error {
	version, err := legacyVersion(tool, tag)
	if err != nil {
		return err
	}
	if out == "" {
		return fmt.Errorf("release: legacy build requires --out")
	}
	out, err = filepath.Abs(out)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(out, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(out)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("release: legacy build requires an empty output directory")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		return err
	}
	for _, platform := range Platforms {
		if err := buildOne(ctx, goBin, root, filepath.Join(out, tool+"-"+version+"-"+platform), tool, platform, version); err != nil {
			return err
		}
	}
	if _, err := writeSums(out); err != nil {
		return err
	}
	return verifyLegacy(out, tool, tag)
}

func verifyPublish(opts PublishOptions) error {
	if opts.LegacyTool != "" {
		return verifyLegacy(opts.OutDir, opts.LegacyTool, opts.Tag)
	}
	return Verify(opts.OutDir, opts.Tag)
}

func verifyLegacy(out, tool, tag string) error {
	version, err := legacyVersion(tool, tag)
	if err != nil {
		return err
	}
	sums, err := readSums(filepath.Join(out, SumsName))
	if err != nil {
		return err
	}
	names, err := listArtifacts(out)
	if err != nil {
		return err
	}
	if len(names) != len(Platforms)+1 || len(sums) != len(Platforms) {
		return fmt.Errorf("release: legacy artifact set must contain exactly four platforms and SHA256SUMS")
	}
	for _, platform := range Platforms {
		name := tool + "-" + version + "-" + platform
		actual, err := hashFile(filepath.Join(out, name))
		if err != nil {
			return err
		}
		if sums[name] != actual {
			return fmt.Errorf("release: legacy artifact %s failed checksum verification", name)
		}
	}
	return nil
}
