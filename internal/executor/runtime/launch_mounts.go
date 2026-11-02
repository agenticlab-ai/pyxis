package runtime

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type launchMounts struct {
	workFD, stateFD     int
	workRoot, stateRoot *os.Root
}

// Retain each capability's directory roots. Later generations resolve beneath
// these handles, so swapping a narrower ancestor's name cannot regain a wider
// ancestor's permissions. Each scope owns and closes its handles at revocation.
func openLaunchMounts(paths accessPaths) (accessPaths, func(), error) {
	var roots []*os.Root
	cleanup := func() {
		for _, root := range roots {
			_ = root.Close() // release the capability's owned directory handle
		}
	}
	for _, path := range []string{paths.state, paths.work} {
		if path == paths.work && paths.readOnly {
			continue
		}
		var root *os.Root
		var err error
		if len(paths.ancestors) == 0 {
			root, err = os.OpenRoot(path) // supervisor-owned top-level roots
		} else {
			parent := paths.ancestors[len(paths.ancestors)-1]
			rootPath, parentRoot := parent.state, parent.stateRoot
			if !pathContains(rootPath, path) {
				if parent.readOnly || !pathContains(parent.work, path) {
					cleanup()
					return paths, nil, errors.New("mount escapes parent write roots")
				}
				rootPath, parentRoot = parent.work, parent.workRoot
			}
			if parentRoot == nil {
				cleanup()
				return paths, nil, errors.New("missing parent launch root")
			}
			// pathContains above already proved Rel valid and beneath rootPath.
			rel, _ := filepath.Rel(rootPath, path)
			root, err = parentRoot.OpenRoot(rel)
		}
		if err != nil {
			cleanup()
			return paths, nil, fmt.Errorf("pin launch directory: %w", err)
		}
		roots = append(roots, root)
		if path == paths.state {
			paths.stateRoot = root
		} else {
			paths.workRoot = root
		}
	}
	return paths, cleanup, nil
}

// Bubblewrap consumes descriptors from the already pinned capability roots,
// rather than reopening mutable source names before applying its mount policy.
func pinLaunchMounts(cmd *exec.Cmd, paths accessPaths) (accessPaths, func(), error) {
	var files []*os.File
	cleanup := func() {
		for _, file := range files {
			_ = file.Close() // release the owned mount descriptor
		}
	}
	if len(paths.ancestors) == 0 {
		return paths, cleanup, nil
	}
	for _, path := range []string{paths.state, paths.work} {
		if path == paths.work && paths.readOnly {
			continue
		}
		root := paths.stateRoot
		if path == paths.work {
			root = paths.workRoot
		}
		if root == nil {
			cleanup()
			return paths, nil, errors.New("missing pinned launch directory")
		}
		file, err := root.Open(".")
		if err != nil {
			cleanup()
			return paths, nil, fmt.Errorf("open pinned launch directory: %w", err)
		}
		files = append(files, file)
		fd := 3 + len(cmd.ExtraFiles)
		cmd.ExtraFiles = append(cmd.ExtraFiles, file)
		if path == paths.state {
			paths.stateFD = fd
		} else {
			paths.workFD = fd
		}
	}
	return paths, cleanup, nil
}
