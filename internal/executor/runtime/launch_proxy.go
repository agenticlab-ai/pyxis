package runtime

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"sync"
)

const launchProxyArg = "__pyxis_confined_launch__"
const maxLaunchFrame = 1 << 20

// The host executable already imports runtime. Re-exec it as a stdio proxy,
// before its normal entry point, rather than shipping a second helper binary.
// This mode cannot launch anything without a live, scoped host capability.
func init() {
	if code, handled := launchProxyMain(os.Args, os.Stdin, os.Stdout, os.Stderr); handled {
		os.Exit(code)
	}
}

func launchProxyMain(args []string, stdin io.ReadCloser, stdout, stderr io.Writer) (int, bool) {
	if len(args) != 3 || args[1] != launchProxyArg {
		return 0, false
	}
	data, err := readLaunchRequest(args[2])
	code := 125
	if err == nil {
		code, err = runLaunchProxy(context.Background(), data, stdin, stdout, stderr)
	}
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "runtime: confined launch:", err) // a broken stderr cannot turn failure into success
	}
	return code, true
}

func readLaunchRequest(path string) ([]byte, error) {
	if _, present, err := currentLaunchCapability(); err != nil {
		return nil, err
	} else if !present {
		return nil, errors.New("missing launch capability")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }() // the request is read-only disposable data
	data, err := io.ReadAll(io.LimitReader(file, maxLaunchFrame+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxLaunchFrame {
		return nil, errors.New("launch request exceeds size bound")
	}
	return data, nil
}

func confineProcess(cmd *exec.Cmd, paths accessPaths) error {
	ref, present, err := currentLaunchCapability()
	if err != nil {
		return err
	}
	if !present {
		return confineNativeProcess(cmd, paths)
	}
	if len(cmd.ExtraFiles) != 0 {
		return errors.New("runtime: delegated launches do not accept inherited file descriptors")
	}
	env := cleanLaunchEnv(cmd.Environ())
	request := launchRequest{Schema: launchSchema, Path: cmd.Path, Args: cmd.Args, Env: env, Paths: wireLaunchPaths(paths)}
	data, err := json.Marshal(request)
	if err != nil {
		return err
	}
	if len(data) > maxLaunchFrame {
		return errors.New("runtime: launch request exceeds size bound")
	}
	// Keep environment values and potentially large argv out of the proxy's
	// process arguments. The private request stays in disposable runtime state
	// and is removed by its owner, never by an unauthenticated proxy argument.
	file, err := os.CreateTemp(paths.state, "pyxis-launch-*.json")
	if err != nil {
		return fmt.Errorf("runtime: launch request: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()           // preserve the write error
		_ = os.Remove(file.Name()) // failed request has not been exposed
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	cmd.Path, cmd.Args = ref.Proxy, []string{ref.Proxy, launchProxyArg, file.Name()}
	cmd.Env = withLaunchCapability(env, ref)
	return nil
}

func runLaunchProxy(ctx context.Context, data []byte, stdin io.ReadCloser, stdout, stderr io.Writer) (int, error) {
	ref, present, err := currentLaunchCapability()
	if err != nil {
		return 125, err
	}
	if !present {
		return 125, errors.New("missing launch capability")
	}
	if len(data) > maxLaunchFrame {
		return 125, errors.New("launch request exceeds size bound")
	}
	var request launchRequest
	if err := decodeLaunchJSON(data, &request); err != nil {
		return 125, err
	}
	request.Token = ref.Token
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp4", ref.Address)
	if err != nil {
		return 125, err
	}
	defer func() { _ = conn.Close() }()                         // this proxy owns the connection
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() }) // unblock owned I/O on cancellation
	defer stop()
	if err := writeLaunchJSON(conn, request); err != nil {
		return 125, err
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 16<<10)
		for {
			n, err := stdin.Read(buf)
			if n > 0 {
				if err := writeLaunchJSON(conn, launchFrame{Schema: launchSchema, Kind: "stdin", Data: buf[:n]}); err != nil {
					return
				}
			}
			if err != nil {
				if errors.Is(err, io.EOF) {
					_ = writeLaunchJSON(conn, launchFrame{Schema: launchSchema, Kind: "stdin-eof"}) // peer failure is reported by the reply reader
				} else {
					_ = conn.Close() // an input failure cancels the delegated command
				}
				return
			}
		}
	}()
	defer func() {
		_ = conn.Close()  // release a blocked input write
		_ = stdin.Close() // release a blocked input read
		<-done
	}()
	for {
		var frame launchFrame
		if err := readLaunchJSON(conn, &frame); err != nil {
			return 125, err
		}
		if frame.Schema != launchSchema {
			return 125, errors.New("invalid launch reply schema")
		}
		switch frame.Kind {
		case "stdout", "stderr":
			out := stdout
			if frame.Kind == "stderr" {
				out = stderr
			}
			if _, err := out.Write(frame.Data); err != nil {
				return 125, err
			}
		case "exit":
			if frame.Error != "" {
				return 125, errors.New(frame.Error)
			}
			if frame.ExitCode < 0 || frame.ExitCode > 255 {
				return 125, errors.New("invalid launch exit status")
			}
			return frame.ExitCode, nil
		default:
			return 125, errors.New("invalid launch reply kind")
		}
	}
}

type launchFrame struct {
	Schema   string `json:"schema_version"`
	Kind     string `json:"kind"`
	Data     []byte `json:"data,omitempty"`
	ExitCode int    `json:"exit_code,omitempty"`
	Error    string `json:"error,omitempty"`
}

func readLaunchJSON(reader io.Reader, target any) error {
	var prefix [4]byte
	if _, err := io.ReadFull(reader, prefix[:]); err != nil {
		return err
	}
	size := binary.BigEndian.Uint32(prefix[:])
	if size == 0 || size > maxLaunchFrame {
		return errors.New("invalid launch frame size")
	}
	data := make([]byte, size)
	if _, err := io.ReadFull(reader, data); err != nil {
		return err
	}
	return decodeLaunchJSON(data, target)
}

func writeLaunchJSON(writer io.Writer, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(data) > maxLaunchFrame {
		return errors.New("launch frame exceeds size bound")
	}
	var prefix [4]byte
	binary.BigEndian.PutUint32(prefix[:], uint32(len(data)))
	if _, err := writer.Write(prefix[:]); err != nil {
		return err
	}
	_, err = writer.Write(data)
	return err
}

type launchWriter struct {
	conn io.Writer
	mu   sync.Mutex
}

type launchStream struct {
	writer *launchWriter
	kind   string
}

func (w *launchWriter) stream(kind string) io.Writer { return launchStream{writer: w, kind: kind} }

func (s launchStream) Write(data []byte) (int, error) {
	s.writer.mu.Lock()
	defer s.writer.mu.Unlock()
	written := 0
	for len(data) > 0 {
		n := min(len(data), 16<<10)
		if err := writeLaunchJSON(s.writer.conn, launchFrame{Schema: launchSchema, Kind: s.kind, Data: data[:n]}); err != nil {
			return written, err
		}
		written += n
		data = data[n:]
	}
	return written, nil
}

func (w *launchWriter) finish(code int, err error) {
	frame := launchFrame{Schema: launchSchema, Kind: "exit", ExitCode: code}
	if err != nil {
		frame.Error = err.Error()
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	_ = writeLaunchJSON(w.conn, frame) // a disconnected caller cannot receive a verdict; never retry unconfined
}
