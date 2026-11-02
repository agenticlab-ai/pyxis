package runtime

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
)

const launchCapabilityEnv = "PYXIS_CONFINED_LAUNCH"
const launchSchema = "confined-launch/v1"

type launchCapability struct {
	Schema  string `json:"schema_version"`
	Address string `json:"address"`
	Token   string `json:"token"`
	Proxy   string `json:"proxy"`
	State   string `json:"state"`
}

type launchPaths struct {
	Work      string   `json:"work"`
	State     string   `json:"state"`
	Protected []string `json:"protected"`
	ReadOnly  bool     `json:"read_only"`
}

func currentLaunchCapability() (launchCapability, bool, error) {
	value, present := os.LookupEnv(launchCapabilityEnv)
	var ref launchCapability
	if !present {
		return ref, false, nil
	}
	if err := decodeLaunchJSON([]byte(value), &ref); err != nil {
		return ref, true, fmt.Errorf("runtime: invalid launch capability: %w", err)
	}
	host, _, err := net.SplitHostPort(ref.Address)
	if err != nil || host != "127.0.0.1" || ref.Schema != launchSchema || len(ref.Token) != 64 || !filepath.IsAbs(ref.Proxy) || !filepath.IsAbs(ref.State) || ref.State == string(filepath.Separator) || strings.ContainsAny(ref.State, "\x00\r\n") {
		return ref, true, errors.New("runtime: invalid launch capability")
	}
	return ref, true, nil
}

func withLaunchCapability(env []string, ref launchCapability) []string {
	data, err := json.Marshal(ref)
	if err != nil {
		panic(err) // this struct contains only strings; marshaling cannot fail
	}
	result := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if !strings.HasPrefix(entry, launchCapabilityEnv+"=") {
			result = append(result, entry)
		}
	}
	return append(result, launchCapabilityEnv+"="+string(data))
}

func decodeLaunchJSON(data []byte, target any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return err
	}
	if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
		return errors.New("trailing launch data")
	}
	return nil
}

// A child may narrow its parent's write roots but cannot regain an ancestor's
// Git permissions. Resolve symlinks on the supervisor side, not from claims
// supplied by the sandboxed caller.
func narrowLaunchPaths(parent accessPaths, requested launchPaths) (accessPaths, error) {
	paths := accessPaths{readOnly: requested.ReadOnly}
	var err error
	if requested.Work == "/" && requested.ReadOnly {
		paths.work = "/" // the read-only prerequisite probe uses the filesystem root
	} else if paths.work, err = accessDirectory(requested.Work); err != nil {
		return paths, fmt.Errorf("nested worktree: %w", err)
	}
	if paths.state, err = accessDirectory(requested.State); err != nil {
		return paths, fmt.Errorf("nested runtime state: %w", err)
	}
	for _, root := range []string{paths.state, paths.work} {
		if root == paths.work && paths.readOnly {
			continue
		}
		if !pathContains(parent.state, root) && (parent.readOnly || !pathContains(parent.work, root)) {
			return paths, errors.New("nested launch expands writable scope")
		}
		for _, protected := range parent.protected {
			if pathContains(protected, root) {
				return paths, errors.New("nested launch makes Git administration writable")
			}
		}
	}
	paths.protected = append(paths.protected, parent.protected...)
	paths.ancestors = append(append([]accessPaths{}, parent.ancestors...), parent)
	for _, path := range requested.Protected {
		if !filepath.IsAbs(path) || strings.ContainsAny(path, "\x00\r\n") {
			return paths, errors.New("invalid nested protected path")
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return paths, fmt.Errorf("nested protected path: %w", err)
		}
		paths.protected = append(paths.protected, resolved)
	}
	return paths, nil
}

func wireLaunchPaths(paths accessPaths) launchPaths {
	return launchPaths{Work: paths.work, State: paths.state, Protected: paths.protected, ReadOnly: paths.readOnly}
}
