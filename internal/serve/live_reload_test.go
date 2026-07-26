package serve_test

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/serve"
)

// startServerFast is startServer's live-reload sibling: it drives the watcher on
// a short, deterministic cadence (instead of the ~500ms/200ms production cadence)
// so the SSE integration tests observe a "changed" within a tight window rather
// than a real half-second poll.
func startServerFast(t *testing.T, files map[string]string) (srv *serve.Server, base, root string) {
	t.Helper()
	root = t.TempDir()
	writeFile(t, filepath.Join(root, "project.config.yaml"), baseConfig)
	for rel, content := range files {
		writeFile(t, filepath.Join(root, rel), content)
	}
	cfg, err := config.LoadConfig(filepath.Join(root, "project.config.yaml"))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	srv = serve.New(cfg, testVersion)
	srv.SetWatchIntervals(15*time.Millisecond, 25*time.Millisecond)
	if err := srv.Listen(0); err != nil {
		t.Fatalf("listen: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		srv.Serve(ctx) //nolint:errcheck // test server; Serve returns ErrServerClosed on cancel
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return srv, fmt.Sprintf("http://127.0.0.1:%d", srv.Port()), root
}

// sseClient opens a live /api/events stream and returns a channel that receives
// the name of every SSE event ("changed", ...) plus a cancel that closes the
// stream. It reads in a background goroutine so a test can assert on delivery
// and timing; comment frames (keep-alives, the ": connected" opener) are not
// events and are skipped.
func sseClient(t *testing.T, base string) (<-chan string, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/events", http.NoBody)
	if err != nil {
		cancel()
		t.Fatalf("new events request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		cancel()
		t.Fatalf("open events stream: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		cancel()
		t.Fatalf("GET /api/events: got %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		resp.Body.Close()
		cancel()
		t.Fatalf("GET /api/events content-type = %q, want text/event-stream", ct)
	}
	events := make(chan string, 16)
	go func() {
		defer resp.Body.Close()
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			line := sc.Text()
			if name, ok := strings.CutPrefix(line, "event: "); ok {
				select {
				case events <- name:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return events, cancel
}

func waitChanged(t *testing.T, events <-chan string, timeout time.Duration) {
	t.Helper()
	select {
	case ev := <-events:
		if ev != "changed" {
			t.Fatalf("SSE event = %q, want \"changed\"", ev)
		}
	case <-time.After(timeout):
		t.Fatalf("timed out after %s waiting for a changed event", timeout)
	}
}

func waitHubSize(t *testing.T, srv *serve.Server, want int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if srv.HubSize() == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("hub size = %d after %s, want %d", srv.HubSize(), timeout, want)
}

// One mutation delivers exactly ONE changed after the poll+debounce — no more.
func TestSSE_OneMutationDeliversExactlyOneChanged(t *testing.T) {
	_, base, _ := startServerFast(t, standardFiles())
	events, cancel := sseClient(t, base)
	defer cancel()

	resp, data := do(t, http.MethodPost, base+"/api/claims/widget.contract.one/comments",
		`{"body":"hello"}`, allowedMutating(base)...)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST add: got %d, want 200 (%s)", resp.StatusCode, data)
	}

	waitChanged(t, events, 3*time.Second)

	// A single mutation must not produce a second changed.
	select {
	case ev := <-events:
		t.Fatalf("unexpected second event %q for a single mutation", ev)
	case <-time.After(700 * time.Millisecond):
	}
}

// An external process writing a brand-new claim in a nested subdir is delivered
// as a changed to a connected client.
func TestSSE_ExternalNestedWriteDeliversChanged(t *testing.T) {
	_, base, root := startServerFast(t, standardFiles())
	events, cancel := sseClient(t, base)
	defer cancel()

	writeFile(t, filepath.Join(root, "claims", "nested", "extra.yaml"),
		draftClaim("widget.contract.extra"))

	waitChanged(t, events, 3*time.Second)
}

// A *.tmp-* file appearing then vanishing (the atomic-writer scratch pattern)
// delivers no event at all.
func TestSSE_TmpFileDeliversNoChanged(t *testing.T) {
	_, base, root := startServerFast(t, standardFiles())
	events, cancel := sseClient(t, base)
	defer cancel()

	tmp := filepath.Join(root, "claims", "one.yaml.tmp-777777")
	writeFile(t, tmp, "scratch: true\n")
	time.Sleep(200 * time.Millisecond)
	if err := os.Remove(tmp); err != nil {
		t.Fatalf("remove tmp: %v", err)
	}

	select {
	case ev := <-events:
		t.Fatalf("a *.tmp-* file produced an SSE event %q, want none", ev)
	case <-time.After(700 * time.Millisecond):
	}
}

// The hub size returns to zero after a client disconnects (the handler's
// deferred unsub runs). This leak is invisible to -race, so the count is
// asserted directly.
func TestSSE_SubscriberCountReturnsToZeroOnDisconnect(t *testing.T) {
	srv, base, _ := startServerFast(t, standardFiles())

	_, cancel := sseClient(t, base)
	waitHubSize(t, srv, 1, 2*time.Second)

	cancel() // client disconnects
	waitHubSize(t, srv, 0, 2*time.Second)
}

// A connected SSE subscriber never delays a POST: the write path does not block
// on event fan-out.
func TestSSE_ConnectedClientDoesNotDelayPost(t *testing.T) {
	srv, base, _ := startServerFast(t, standardFiles())
	_, cancel := sseClient(t, base)
	defer cancel()
	waitHubSize(t, srv, 1, 2*time.Second)

	start := time.Now()
	resp, data := do(t, http.MethodPost, base+"/api/claims/widget.contract.one/comments",
		`{"body":"x"}`, allowedMutating(base)...)
	elapsed := time.Since(start)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST: got %d, want 200 (%s)", resp.StatusCode, data)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("POST took %s with a live SSE subscriber; the write path must not block on fan-out", elapsed)
	}
}

// The startup guardrail refuses to serve when the render/catalog/store outputs
// would sit inside the watched claims tree (a claims_dir of "." makes the tree
// the whole project dir), which would otherwise drive the watcher in a loop.
func TestServe_RefusesWhenOutputsInsideClaimsTree(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "project.config.yaml"),
		"schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: .\n")
	cfg, err := config.LoadConfig(filepath.Join(root, "project.config.yaml"))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	srv := serve.New(cfg, testVersion)
	if err := srv.Listen(0); err != nil {
		t.Fatalf("listen: %v", err)
	}
	// A finite context so a guardrail regression (Serve running instead of
	// refusing) surfaces as a nil error at timeout rather than hanging the test.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err = srv.Serve(ctx)
	if err == nil {
		t.Fatal("Serve accepted a claims_dir that contains the render/catalog/store outputs")
	}
	if !strings.Contains(err.Error(), "claims_dir") {
		t.Fatalf("guardrail error = %v, want it to mention claims_dir", err)
	}
}
