package viewertests

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"testing"
	"time"
)

// This file builds the dossierx binary ONCE per test run and drives it
// black-box, exactly like scripts/e2e-comments.sh: a throwaway project on disk,
// `dossierx render` for the static file:// case, and `dossierx serve` for the
// live case. Nothing here imports the engine's Go packages — the harness talks
// to the built CLI + the HTTP API only, which is what keeps chromedp out of the
// engine's module graph.

var (
	testBin    string // absolute path to the built dossierx binary
	testBinDir string // temp dir holding it (removed in TestMain)
	buildErr   error
)

func TestMain(m *testing.M) {
	if dir, err := os.MkdirTemp("", "dossierx-viewer-tests-bin"); err == nil {
		testBinDir = dir
		testBin, buildErr = buildDossierx(dir)
	} else {
		buildErr = err
	}

	code := m.Run()

	allocMu.Lock()
	if allocCancel != nil {
		allocCancel()
	}
	allocMu.Unlock()
	if testBinDir != "" {
		_ = os.RemoveAll(testBinDir)
	}
	os.Exit(code)
}

// repoRoot is the module root two levels up from this test source file
// (…/dossierx/viewer-tests/project_test.go → …/dossierx).
func repoRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("cannot locate test source via runtime.Caller")
	}
	return filepath.Dir(filepath.Dir(file)), nil
}

func buildDossierx(dir string) (string, error) {
	root, err := repoRoot()
	if err != nil {
		return "", err
	}
	bin := filepath.Join(dir, "dossierx")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/dossierx")
	cmd.Dir = root
	cmd.Env = os.Environ()
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go build ./cmd/dossierx: %w\n%s", err, out)
	}
	return bin, nil
}

func requireBin(t *testing.T) string {
	t.Helper()
	if buildErr != nil {
		t.Fatalf("dossierx binary was not built: %v", buildErr)
	}
	return testBin
}

// defaultConfigYAML is a one-module, one-facet project — the smallest shape a
// lockable claim needs.
const defaultConfigYAML = `schema_version: 1
facets:
  - contract
modules:
  - widget
claims_dir: claims
`

// draftClaimYAML is a single lockable draft claim with a governed_by: none
// escape hatch (so it needs no doctrine claim to be lint-clean).
const draftClaimYAML = `id: widget.contract.overview
facet: contract
module: widget
status: draft
body: |
  a claim under review.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`

const testClaimID = "widget.contract.overview"

type project struct {
	t         *testing.T
	bin       string
	dir       string
	config    string
	claimsDir string

	// base is the URL of the serve process started by ensureServe, cached so a
	// test that needs the HTTP API before it opens a tab (and newLiveTab, which
	// opens one) share ONE server rather than racing two on the same project
	// directory. Empty until ensureServe runs.
	base string
}

// newProjectRaw creates a project dir + config with an empty claims/ dir; the
// caller writes the claim files it needs via writeClaim.
func newProjectRaw(t *testing.T, configYAML string) *project {
	t.Helper()
	bin := requireBin(t)
	dir := t.TempDir()
	claimsDir := filepath.Join(dir, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims: %v", err)
	}
	cfg := filepath.Join(dir, "project.config.yaml")
	if err := os.WriteFile(cfg, []byte(configYAML), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return &project{t: t, bin: bin, dir: dir, config: cfg, claimsDir: claimsDir}
}

func (p *project) writeClaim(name, content string) {
	p.t.Helper()
	if err := os.WriteFile(filepath.Join(p.claimsDir, name), []byte(content), 0o644); err != nil {
		p.t.Fatalf("write claim %s: %v", name, err)
	}
}

// newProject is the common single-module, single-claim fixture the comment
// tests use.
func newProject(t *testing.T) *project {
	t.Helper()
	p := newProjectRaw(t, defaultConfigYAML)
	p.writeClaim("overview.yaml", draftClaimYAML)
	return p
}

// run executes `dossierx --config <cfg> --format text <args...>` and returns
// its combined output, failing the test on a non-zero exit.
//
// --format text is PINNED here rather than passed at each of the call sites.
// v0.3.0 made JSON the default output format, so without this every helper
// below that parses prose (seedComment's thread-id regex, the assertions on
// human-readable output) would be reading an envelope instead. Pinning it in
// the one helper is what kept this suite's call sites unchanged across the
// restructure. pflag takes the LAST occurrence of a repeated flag, so a test
// that wants the machine surface simply passes "--format", "json" in its own
// args and wins — the pin is a default, not a lock.
func (p *project) run(args ...string) string {
	p.t.Helper()
	if isClaimLock(args) {
		previewArgs := append([]string{}, args...)
		previewArgs = append(previewArgs, "--format", "json", "--dry-run")
		previewCmd := exec.Command(p.bin, append([]string{"--config", p.config}, previewArgs...)...)
		preview, err := previewCmd.CombinedOutput()
		if err != nil {
			p.t.Fatalf("dossierx lock preview %v failed: %v\n%s", args, err, preview)
		}
		var envelope struct {
			Data struct {
				Snapshot string `json:"snapshot"`
			} `json:"data"`
		}
		if err := json.Unmarshal(preview, &envelope); err != nil || envelope.Data.Snapshot == "" {
			p.t.Fatalf("dossierx lock preview %v returned no proposal token: %v\n%s", args, err, preview)
		}
		args = append(args, "--proposal", envelope.Data.Snapshot)
	}
	full := append([]string{"--config", p.config, "--format", "text"}, args...)
	cmd := exec.Command(p.bin, full...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		p.t.Fatalf("dossierx %v failed: %v\n%s", args, err, out)
	}
	return string(out)
}

func isClaimLock(args []string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "claim" && args[i+1] == "lock" {
			for _, arg := range args {
				if arg == "--dry-run" || arg == "--proposal" {
					return false
				}
			}
			return true
		}
	}
	return false
}

// seedComment adds one open thread on the test claim as the given role and
// returns the minted thread id (parsed from the id-echo output).
func (p *project) seedComment(role, body string) string {
	p.t.Helper()
	out := p.run("comment", "add", testClaimID, "--as", role, "--body", body)
	m := regexp.MustCompile(`c-[0-9a-z]+`).FindString(out)
	if m == "" {
		p.t.Fatalf("comment add did not echo a thread id: %s", out)
	}
	return m
}

// renderStatic writes build/viewer/index.html and returns a file:// URL pointing at
// it.
//
// It drives "check", not the "render" verb this called before v0.3.0: rendering
// stopped being a verb when the surface went 26 -> 19 and became a stage of
// check. check does strictly more (lint, catalog, the ledger gate) but it
// writes the same build/viewer/index.html, which is the only thing this helper is
// after.
func (p *project) renderStatic() string {
	p.t.Helper()
	p.run("check")
	out := filepath.Join(p.dir, "build", "viewer", "index.html")
	if _, err := os.Stat(out); err != nil {
		p.t.Fatalf("render did not write %s: %v", out, err)
	}
	return "file://" + out
}

// ensureServe returns a running server's base URL, starting one on first call
// and reusing it after. Two serve processes on one project directory would both
// watch and write the same claim files, so anything needing both the HTTP API
// and a browser tab must share a single server.
func (p *project) ensureServe() string {
	p.t.Helper()
	if p.base == "" {
		p.base, _ = p.serve()
	}
	return p.base
}

// resolveViaAPI resolves thread tid on the test claim through the serve HTTP
// API, as the given role, and fails the test on any non-200.
//
// This exists because there is no CLI path to it any more. "comment resolve"
// was removed from the CLI in v0.3.0 on purpose: resolving a thread IS the
// human's approval, so it lives only where the rights holder is — the viewer.
// The viewer reaches it over this endpoint, so a browser test that needs a
// pre-resolved thread must go the same way the browser does. This harness also
// CANNOT reach internal/comments directly: viewer-tests is a separate Go module
// and internal/ is unreachable from outside the engine's module.
//
// The three admission headers are not optional decoration — serve's middleware
// requires Origin on any mutating request (absent and "null" are both refused),
// Content-Type: application/json on POST, and a Sec-Fetch-Site that is absent
// or same-origin. Sending base as the Origin is exactly what the viewer does.
func (p *project) resolveViaAPI(tid, role string) {
	p.t.Helper()
	base := p.ensureServe()
	url := fmt.Sprintf("%s/api/claims/%s/comments/%s/resolve", base, testClaimID, tid)
	body := fmt.Sprintf(`{"as":%q}`, role)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader([]byte(body)))
	if err != nil {
		p.t.Fatalf("resolve request: %v", err)
	}
	req.Header.Set("Origin", base)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Sec-Fetch-Site", "same-origin")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		p.t.Fatalf("resolve %s: %v", tid, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		var buf bytes.Buffer
		if _, err := buf.ReadFrom(resp.Body); err != nil {
			buf.WriteString("<body unreadable: " + err.Error() + ">")
		}
		p.t.Fatalf("resolve %s: got status %d, want 200\nbody: %s", tid, resp.StatusCode, buf.String())
	}
}

// claimBytes returns the on-disk claim YAML (for byte-unchanged assertions).
func (p *project) claimBytes() []byte {
	p.t.Helper()
	b, err := os.ReadFile(filepath.Join(p.claimsDir, "overview.yaml"))
	if err != nil {
		p.t.Fatalf("read claim: %v", err)
	}
	return b
}

// serve starts `dossierx serve`, waits for it to print its URL and answer
// /api/ping, and returns the base URL plus a stop func. The stop func is also
// registered with t.Cleanup so a failing test never leaks the process.
func (p *project) serve() (base string, stop func()) {
	p.t.Helper()
	cmd := exec.Command(p.bin, "--config", p.config, "serve")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		p.t.Fatalf("serve stdout pipe: %v", err)
	}
	var stderr syncBuffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		p.t.Fatalf("serve start: %v", err)
	}

	stopped := false
	// Every error below is expected rather than exceptional: by the time stop
	// runs, the server has often already exited on its own, and signalling or
	// killing a finished process reports os.ErrProcessDone. Anything ELSE is
	// worth seeing, so it is logged rather than discarded — this used to be
	// three blank assignments, which no linter had ever read.
	stop = func() {
		if stopped {
			return
		}
		stopped = true
		if err := cmd.Process.Signal(os.Interrupt); err != nil && !errors.Is(err, os.ErrProcessDone) {
			p.t.Logf("interrupting serve: %v", err)
		}
		done := make(chan struct{})
		go func() {
			defer close(done)
			// A non-nil error here is the interrupt or kill above landing.
			if err := cmd.Wait(); err != nil {
				return
			}
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
				p.t.Logf("killing serve: %v", err)
			}
			<-done
		}
	}
	p.t.Cleanup(stop)

	// Read stdout until the URL line appears (deterministic: Scan blocks on IO,
	// no sleep). A background drain keeps the pipe from ever blocking serve.
	urlCh := make(chan string, 1)
	go func() {
		sc := bufio.NewScanner(stdout)
		re := regexp.MustCompile(`http://127\.0\.0\.1:\d+`)
		for sc.Scan() {
			if u := re.FindString(sc.Text()); u != "" {
				urlCh <- u
				for sc.Scan() { // drain remaining output
				}
				return
			}
		}
		close(urlCh) // EOF without a URL
	}()

	select {
	case base = <-urlCh:
		if base == "" {
			stop()
			p.t.Fatalf("serve exited before printing its URL\nstderr:\n%s", stderr.String())
		}
	case <-time.After(15 * time.Second):
		stop()
		p.t.Fatalf("timed out waiting for serve URL\nstderr:\n%s", stderr.String())
	}

	// Confirm the HTTP handler actually answers. The socket is bound by the time
	// the URL prints (Listen precedes the print), so this normally passes on the
	// first try; the short retry only covers Serve's Accept loop starting.
	if !waitHTTPReady(base+"/api/ping", 10*time.Second) {
		stop()
		p.t.Fatalf("serve /api/ping never returned 200\nstderr:\n%s", stderr.String())
	}
	return base, stop
}

// waitHTTPReady polls url until it returns HTTP 200 or the deadline passes. The
// 20ms gap between attempts is a subprocess-readiness poll (mirrors
// scripts/e2e-comments.sh) — every BROWSER wait in this suite uses chromedp's
// deterministic Poll/WaitVisible instead.
func waitHTTPReady(url string, within time.Duration) bool {
	deadline := time.Now().Add(within)
	client := &http.Client{Timeout: 2 * time.Second}
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return true
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// syncBuffer is a tiny concurrency-safe bytes.Buffer for capturing serve's
// stderr from the wait goroutine while the test reads it on failure.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
