package resolve_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/resolve"
)

func TestHTTPSourceEndToEnd(t *testing.T) {
	plat := resolve.Platform()
	content := []byte("http-served binary")
	f := newFixture(t, "v0.4.0", map[string]map[string][]byte{"docgate": {plat: content}})
	srv := httptest.NewServer(http.FileServer(http.Dir(f.dir)))
	defer srv.Close()

	t.Run("resolves over http with the same verification", func(t *testing.T) {
		cfg := f.cfg(t.TempDir())
		cfg.Repo.System.ArtifactSource = srv.URL // scheme dispatch picks the http fetcher
		path, err := resolve.Tool(context.Background(), cfg, "docgate", plat)
		if err != nil {
			t.Fatalf("Tool over http: %v", err)
		}
		if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, content) {
			t.Fatalf("wrong bytes over http (err %v)", err)
		}
	})

	t.Run("a 404 source is an offline miss", func(t *testing.T) {
		cfg := f.cfg(t.TempDir())
		cfg.Repo.System.ArtifactSource = srv.URL + "/no-such-prefix"
		_, err := resolve.Tool(context.Background(), cfg, "docgate", plat)
		var off *resolve.OfflineError
		if !errors.As(err, &off) {
			t.Fatalf("err = %v, want *OfflineError", err)
		}
		if !strings.Contains(off.Err.Error(), "404") {
			t.Fatalf("underlying cause does not carry the status: %v", off.Err)
		}
	})
}

func TestHTTPFetchCancelsInFlight(t *testing.T) {
	plat := resolve.Platform()
	f := newFixture(t, "v0.4.0", map[string]map[string][]byte{"docgate": {plat: []byte("x")}})
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute) // hang watchdog
	defer cancel()
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		cancel() // the request has reached the real server
		<-r.Context().Done()
	}))
	defer srv.Close()

	cfg := f.cfg(t.TempDir())
	cfg.Repo.System.ArtifactSource = srv.URL
	_, err := resolve.Tool(ctx, cfg, "docgate", plat)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want in-flight cancellation surfaced, not an offline miss", err)
	}
}

type deadlineTransport struct{ entered chan struct{} }

func (tr deadlineTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	close(tr.entered)
	<-r.Context().Done()
	return nil, r.Context().Err()
}

func TestHTTPFetchHonorsContextDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		plat := resolve.Platform()
		f := newFixture(t, "v0.4.0", map[string]map[string][]byte{"docgate": {plat: []byte("x")}})
		cfg := f.cfg(t.TempDir())
		cfg.Repo.System.ArtifactSource = "https://artifacts.test"
		entered := make(chan struct{})
		cfg.Fetcher = resolve.HTTPFetcher{Client: &http.Client{Transport: deadlineTransport{entered: entered}}}
		const timeout = time.Minute
		ctx, cancel := context.WithTimeout(t.Context(), timeout)
		defer cancel()
		done := make(chan error, 1)
		go func() { _, err := resolve.Tool(ctx, cfg, "docgate", plat); done <- err }()
		<-entered
		time.Sleep(timeout - time.Nanosecond)
		synctest.Wait()
		select {
		case err := <-done:
			t.Fatalf("request returned before deadline: %v", err)
		default:
		}
		time.Sleep(time.Nanosecond)
		if err := <-done; !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("err = %v, want context.DeadlineExceeded surfaced, not an offline miss", err)
		}
	})
}

func TestFileFetcher(t *testing.T) {
	t.Run("reads a file URL", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "artifact")
		if err := os.WriteFile(p, []byte("payload"), 0o644); err != nil {
			t.Fatal(err)
		}
		rc, err := resolve.FileFetcher{}.Fetch(context.Background(), "file://"+p)
		if err != nil {
			t.Fatalf("Fetch: %v", err)
		}
		defer func() {
			if err := rc.Close(); err != nil {
				t.Error(err)
			}
		}()
		b, err := io.ReadAll(rc)
		if err != nil || string(b) != "payload" {
			t.Fatalf("read = (%q, %v)", b, err)
		}
	})
	t.Run("refuses remote hosts", func(t *testing.T) {
		_, err := resolve.FileFetcher{}.Fetch(context.Background(), "file://example.com/x")
		if err == nil || !strings.Contains(err.Error(), "remote hosts") {
			t.Fatalf("err = %v, want remote-host refusal", err)
		}
	})
	t.Run("honors a dead context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := (resolve.FileFetcher{}).Fetch(ctx, "file:///tmp/x"); !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	})
}

func TestBySchemeDispatch(t *testing.T) {
	t.Run("unsupported scheme refuses by name", func(t *testing.T) {
		_, err := resolve.ByScheme{}.Fetch(context.Background(), "ftp://host/artifact")
		if err == nil || !strings.Contains(err.Error(), `unsupported scheme "ftp"`) {
			t.Fatalf("err = %v, want the unsupported-scheme refusal", err)
		}
	})
	t.Run("file scheme dispatches to the file fetcher", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "artifact")
		if err := os.WriteFile(p, []byte("via dispatch"), 0o644); err != nil {
			t.Fatal(err)
		}
		rc, err := resolve.ByScheme{}.Fetch(context.Background(), "file://"+p)
		if err != nil {
			t.Fatalf("Fetch: %v", err)
		}
		defer func() {
			if err := rc.Close(); err != nil {
				t.Error(err)
			}
		}()
		if b, _ := io.ReadAll(rc); string(b) != "via dispatch" {
			t.Fatalf("read %q", b)
		}
	})
}

func TestHTTPFetcherNon200(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	_, err := resolve.HTTPFetcher{}.Fetch(context.Background(), srv.URL+"/missing")
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("err = %v, want the status named", err)
	}
}

// A public release download answers with a redirect to GitHub's object host.
// The resolver's default client follows it, and only the bytes' hash decides
// whether they are accepted.
func TestHTTPFetcherFollowsARedirectToAnotherHost(t *testing.T) {
	object := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte("object bytes")); err != nil {
			t.Error(err)
		}
	}))
	defer object.Close()
	download := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, object.URL+"/signed", http.StatusFound)
	}))
	defer download.Close()

	rc, err := resolve.HTTPFetcher{}.Fetch(context.Background(), download.URL+"/o/r/releases/download/v1/asset")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	defer func() { _ = rc.Close() }()
	got, err := io.ReadAll(rc)
	if err != nil || string(got) != "object bytes" {
		t.Fatalf("body = %q, %v", got, err)
	}
}
