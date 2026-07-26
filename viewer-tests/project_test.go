package viewertests

import (
	"bufio"
	"bytes"
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
		return "", fmt.Errorf("go build ./cmd/dossierx: %v\n%s", err, out)
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

// run executes `dossierx --config <cfg> <args...>` and returns its combined
// output, failing the test on a non-zero exit.
func (p *project) run(args ...string) string {
	p.t.Helper()
	full := append([]string{"--config", p.config}, args...)
	cmd := exec.Command(p.bin, full...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		p.t.Fatalf("dossierx %v failed: %v\n%s", args, err, out)
	}
	return string(out)
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

// render writes viewer/index.html and returns a file:// URL pointing at it.
func (p *project) renderStatic() string {
	p.t.Helper()
	p.run("render")
	out := filepath.Join(p.dir, "viewer", "index.html")
	if _, err := os.Stat(out); err != nil {
		p.t.Fatalf("render did not write %s: %v", out, err)
	}
	return "file://" + out
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
func (p *project) serve() (string, func()) {
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
	stop := func() {
		if stopped {
			return
		}
		stopped = true
		_ = cmd.Process.Signal(os.Interrupt)
		done := make(chan struct{})
		go func() { _ = cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
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

	var base string
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
