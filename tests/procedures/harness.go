// Package procedures EXECUTES the step-by-step procedures DossierX ships for
// other teams to follow — the skill guides under skills/*/SKILL.md, README's
// paste-block and pre-ledger crossing fence, and the commented recipe inside
// scripts/ci/dossierx-check.yml — against a real built binary, in the exact
// order each document gives them, and asserts each step's DOCUMENTED outcome.
//
// WHY EXECUTION AND NOT READING. The last release attempt found 11 serious
// defects by having an LLM read these documents; 10 of them were procedures
// that simply fail when run. Reading finds a different handful each pass and
// costs a gate round every time; executing finds all of them, on every push,
// for free, and cannot forget. This is derived_facts_test.go's move — check
// prose against a fact, not against another piece of prose — applied to the
// prose that is hardest to check by reading: imperative sequences whose truth
// is only visible in the state they leave behind.
//
// THE ONE RULE EVERY SCENARIO FOLLOWS: assert the DOCUMENTED outcome, never
// the observed one. A scenario that asserts what the binary does today is a
// change detector; a scenario that asserts what the document PROMISES is a
// defect detector, and stays red until either the engine or the document is
// fixed. Consequently a scenario being red on main is not a broken test — it
// is the finding, stated mechanically.
//
// WHAT THE HARNESS CANNOT PROMISE, said here so nobody reads more into a green
// run than it proves:
//
//   - It cannot prove a procedure's PROSE matches the steps enacted here. Each
//     scenario pins the load-bearing phrases of the document it replays with
//     requireDocAnchor, so a rewritten procedure fails loudly ("the document
//     moved; re-read it and update the enactment") instead of silently testing
//     a sequence nobody documents any more — but an anchor is a tripwire, not
//     a parser. A document edit that keeps the phrase while changing its
//     meaning gets past it.
//   - It cannot enact the human. Steps the documents assign to the human (the
//     Resolve click, the "yes" before a lock) are simulated on the same
//     surfaces the human uses — the viewer's HTTP API for Resolve, --reason
//     carrying stand-in words — which exercises the engine's gates but not the
//     human's judgement.
//   - It is hermetic by construction, not by firewall: HOME is redirected,
//     GIT_CONFIG_NOSYSTEM is set, and no step here dials out — the one
//     documented network fetch (the bootstrap's install-hook/CI-template
//     download) is substituted with a copy from this checkout, and the
//     substitution is recorded in the scenario's plan where a reader will see
//     it. Nothing STOPS a future step from reaching the network; review does.
//   - Expected error codes and exit statuses are read from surface.json (the
//     mechanically regenerated inventory) rather than typed in, so this suite
//     rots with the inventory, not ahead of it — but surface.json carries no
//     ledger RULE names, so the few fixture-sanity checks that need one (e.g.
//     "the rewind really produced lock-ledger-pre-ledger") type it and say so.
//
// THE ACCOUNT. Every scenario declares, up front, the full ordered list of
// steps it is about to execute (fixture.Plan), and every execution — CLI run,
// HTTP call, or hand edit enacted through Enact — is recorded against that
// plan. A t.Cleanup comparison fails the scenario if it executed fewer steps,
// different steps, or steps in a different order than it claimed: a scenario
// that early-returns half way through must FAIL, because a pass over a prefix
// of the procedure is a pass over zero assertions about the rest, which is the
// shape CLAUDE.md refuses by name. zz_account_test.go re-checks the same
// ledger after every scenario has run.
//
// A CHECK THAT CANNOT RUN IS A FAILURE. Nothing in this package calls t.Skip,
// for any reason. A missing git, an unbuildable binary, a serve that never
// answers, an unreadable surface.json — each fails the test naming the thing
// that was missing, because "we did not check" must never read as "it is
// fine" (viewer-tests/harness_test.go:47 states the principle this package
// inherits).
package procedures

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// the built binary — once per package
// ---------------------------------------------------------------------------

// exeSuffix mirrors tests/cli_test.go's helper: on Windows os/exec's
// CreateProcess-backed lookup requires the .exe suffix to launch a file even
// though the suffix-less file exists and is readable.
func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

var (
	buildOnce sync.Once
	builtBin  string
	// builtBinDir is exported to main_test.go's TestMain so the one build
	// artifact per `go test` run is removed when the package finishes.
	builtBinDir string
	buildFail   error
)

// binaryPath builds cmd/dossierx exactly once per package run and returns the
// binary's path. The build happening lazily (first fixture) rather than in
// TestMain keeps `go build ./...` compiling this file without a test context,
// while still paying the build once, not per scenario.
func binaryPath(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "dossierx-procedures-bin-")
		if err != nil {
			buildFail = fmt.Errorf("mkdir temp bin dir: %w", err)
			return
		}
		builtBinDir = dir
		bin := filepath.Join(dir, "dossierx"+exeSuffix())
		build := exec.Command("go", "build", "-o", bin, "./cmd/dossierx")
		build.Dir = repoRootUnchecked()
		if out, err := build.CombinedOutput(); err != nil {
			buildFail = fmt.Errorf("go build ./cmd/dossierx: %w\n%s", err, out)
			return
		}
		builtBin = bin
	})
	if buildFail != nil {
		// A binary that cannot build fails every scenario, loudly, rather than
		// skipping any: there is nothing this suite can honestly report about a
		// tree whose CLI does not compile.
		t.Fatalf("cannot build the dossierx binary this suite executes: %v", buildFail)
	}
	return builtBin
}

// repoRootUnchecked is the module root relative to this package's directory
// (tests/procedures -> ../..). Kept separate from repoRoot so binaryPath can
// use it before a *testing.T exists.
func repoRootUnchecked() string {
	abs, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		return filepath.Join("..", "..")
	}
	return abs
}

// repoRoot returns the root of the tree under test and FAILS if it does not
// look like a dossierx checkout. The suite is designed to be copied verbatim
// into a worktree of another branch (that is how the cross-release comparison
// runs), so the root is located relatively, and misplacement fails with the
// missing marker's name instead of a cascade of confusing open() errors.
func repoRoot(t *testing.T) string {
	t.Helper()
	root := repoRootUnchecked()
	for _, marker := range []string{"go.mod", "surface.json", filepath.Join("skills", "dossierx", "SKILL.md")} {
		if _, err := os.Stat(filepath.Join(root, marker)); err != nil {
			t.Fatalf("tests/procedures must sit two levels under a dossierx checkout; %s has no %s: %v", root, marker, err)
		}
	}
	return root
}

// requireDocAnchor pins the load-bearing phrase of the document a scenario
// replays. If the phrase is gone the PROCEDURE has moved, and executing this
// enactment would test a sequence nobody documents any more — so the scenario
// fails asking for a re-read, never silently narrowing to whatever still runs.
func requireDocAnchor(t *testing.T, relPath, phrase string) {
	t.Helper()
	path := filepath.Join(repoRoot(t), filepath.FromSlash(relPath))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read %s, the document this scenario enacts: %v", relPath, err)
	}
	if !strings.Contains(string(raw), phrase) {
		t.Fatalf("%s no longer contains the phrase this scenario enacts:\n  %q\nThe documented procedure moved. Re-read the document and update this enactment to match it — do not delete the scenario, and do not keep executing steps the document no longer gives.", relPath, phrase)
	}
}

// ---------------------------------------------------------------------------
// surface.json — the derived side of every expected-failure assertion
// ---------------------------------------------------------------------------

// inventory is the slice of surface.json this suite branches on. surface.json
// is regenerated by cmd/dossierx/surface_test.go and compared byte for byte
// against the committed copy, so a code or exit status read from it is a fact
// about the binary, not a copy of one — the reason this suite reads it instead
// of typing "exit 1" next to every expected refusal.
type inventory struct {
	ErrorCodes map[string]bool
	ExitCodes  map[string]int
}

var (
	invOnce sync.Once
	inv     *inventory
	invErr  error
)

func surfaceInventory(t *testing.T) *inventory {
	t.Helper()
	invOnce.Do(func() {
		raw, err := os.ReadFile(filepath.Join(repoRootUnchecked(), "surface.json"))
		if err != nil {
			invErr = fmt.Errorf("read surface.json: %w", err)
			return
		}
		var doc struct {
			ErrorCodes []string `json:"error_codes"`
			Envelope   struct {
				ExitCodes map[string]int `json:"exit_codes"`
			} `json:"envelope"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			invErr = fmt.Errorf("decode surface.json: %w", err)
			return
		}
		if len(doc.ErrorCodes) == 0 || len(doc.Envelope.ExitCodes) == 0 {
			// Empty is fatal, not "no expectations": a suite that quietly ran
			// with an empty vocabulary would accept any code as unexpected-but-
			// unchecked, which is a pass over zero assertions.
			invErr = errors.New("surface.json carries no error_codes or no envelope.exit_codes; the derived side of this suite's assertions is gone")
			return
		}
		codes := make(map[string]bool, len(doc.ErrorCodes))
		for _, c := range doc.ErrorCodes {
			codes[c] = true
		}
		inv = &inventory{ErrorCodes: codes, ExitCodes: doc.Envelope.ExitCodes}
	})
	if invErr != nil {
		t.Fatalf("cannot load the error-code inventory this suite asserts against: %v", invErr)
	}
	return inv
}

// ---------------------------------------------------------------------------
// the envelope, as this suite reads it
// ---------------------------------------------------------------------------

// envelope mirrors internal/cliout's wire shape. Redeclared rather than
// imported on purpose: this suite must keep working when copied onto another
// branch's tree whose internal packages may differ, and the JSON keys — not
// the Go types — are the published contract the skills in the field read.
type envelope struct {
	OK        bool           `json:"ok"`
	Command   string         `json:"command"`
	Data      map[string]any `json:"data"`
	Warnings  []string       `json:"warnings"`
	StoppedAt string         `json:"stopped_at"`
	Error     *envError      `json:"error"`
}

type envError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Hint    string         `json:"hint"`
	Details map[string]any `json:"details"`
}

// invocation is one executed step: what was run, what came back, and the
// decoded envelope when the output was one. Env is nil for --format text runs
// and for non-CLI steps; every assertion helper handles that rather than
// assuming JSON.
type invocation struct {
	Step   string // the plan entry this execution was recorded under
	Argv   []string
	Exit   int
	Stdout string
	Stderr string
	Env    *envelope
}

// errorCode returns the envelope's error.code, or "" when there is none —
// the ONLY error field this suite ever branches on. Message and hint are
// prose; they appear in failure output for the human reading the report and
// nowhere in an assertion.
func (i *invocation) errorCode() string {
	if i.Env == nil || i.Env.Error == nil {
		return ""
	}
	return i.Env.Error.Code
}

// ledgerRules extracts data.ledger_findings[].rule so a red scenario's report
// can NAME what the gate found, without any assertion keying on it.
func (i *invocation) ledgerRules() []string {
	if i.Env == nil || i.Env.Data == nil {
		return nil
	}
	raw, ok := i.Env.Data["ledger_findings"].([]any)
	if !ok {
		return nil
	}
	var rules []string
	for _, f := range raw {
		if m, ok := f.(map[string]any); ok {
			if r, ok := m["rule"].(string); ok {
				rules = append(rules, r)
			}
		}
	}
	return rules
}

// ---------------------------------------------------------------------------
// the scenario account
// ---------------------------------------------------------------------------

// scenarioRecord is one scenario's ledger entry: the steps it claimed it would
// execute and the steps it actually did. zz_account_test.go reads the registry
// after every scenario has run and re-verifies each entry, so a scenario that
// slipped past its own cleanup (or whose cleanup was edited away) still fails
// the package.
type scenarioRecord struct {
	Name     string
	Plan     []string
	Executed []string
}

var (
	registryMu sync.Mutex
	registry   []*scenarioRecord
)

func snapshotRegistry() []scenarioRecord {
	registryMu.Lock()
	defer registryMu.Unlock()
	out := make([]scenarioRecord, 0, len(registry))
	for _, r := range registry {
		out = append(out, scenarioRecord{
			Name:     r.Name,
			Plan:     append([]string(nil), r.Plan...),
			Executed: append([]string(nil), r.Executed...),
		})
	}
	return out
}

// verifyAccount compares one scenario's executed steps against its declared
// plan, in order, and reports every divergence. It is the check that makes an
// early return a FAILURE: a scenario that stopped after step 3 of 7 has not
// "passed a shorter test", it has skipped four steps of the procedure it
// claimed to replay.
func verifyAccount(t *testing.T, rec *scenarioRecord) {
	t.Helper()
	registryMu.Lock()
	plan := append([]string(nil), rec.Plan...)
	executed := append([]string(nil), rec.Executed...)
	registryMu.Unlock()

	n := len(plan)
	if len(executed) < n {
		n = len(executed)
	}
	for i := 0; i < n; i++ {
		if plan[i] != executed[i] {
			t.Errorf("scenario %q diverged from its declared plan at step %d:\n  declared: %s\n  executed: %s", rec.Name, i+1, plan[i], executed[i])
		}
	}
	for i := n; i < len(plan); i++ {
		t.Errorf("scenario %q declared step %d but never executed it (an early return is a skipped check, and a skipped check is a failure):\n  %s", rec.Name, i+1, plan[i])
	}
	for i := n; i < len(executed); i++ {
		t.Errorf("scenario %q executed a step it never declared (update the plan so the account stays honest):\n  %s", rec.Name, executed[i])
	}
}

// ---------------------------------------------------------------------------
// the fixture
// ---------------------------------------------------------------------------

// fixture is one throwaway project plus everything a scenario needs to operate
// it: the built binary, a redirected HOME, an optional serve process, and the
// account its executions are recorded into.
type fixture struct {
	t    *testing.T
	dir  string // the fixture's own scratch space (serve.out, home, project)
	root string // the project directory commands run in
	home string
	bin  string

	rec     *scenarioRecord // nil until Plan is called; only then are steps recorded
	planned bool

	serveBase string
	serveCmd  *exec.Cmd
	serveOut  string
}

// defaultClaimID is the one claim every full fixture starts with — the same
// id and body shape scripts/hook-smoke-test.sh's new_project uses, so anyone
// who has read that script recognizes this project on sight.
const defaultClaimID = "widget.contract.overview"

// newBareFixture is a git repository with a user configured, commit signing
// neutralized, and NO dossierx project — the state the bootstrap procedure
// starts from. git init is not optional and not skippable: a machine without
// git cannot execute the procedures this suite exists to execute, so its
// absence is a failure that names git, never a silent pass.
func newBareFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{t: t, bin: binaryPath(t)}
	f.dir = t.TempDir()
	f.home = filepath.Join(f.dir, "home")
	f.root = filepath.Join(f.dir, "project")
	for _, d := range []string{f.home, f.root} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatalf("git is required to execute these procedures and was not found on PATH: %v", err)
	}
	f.git("init", "-q", ".")
	f.git("config", "user.email", "procedures@example.invalid")
	f.git("config", "user.name", "procedure suite")
	// gpgsign from a developer's global config would make every fixture commit
	// prompt for a key; neutralized exactly as hook-smoke-test.sh does.
	f.git("config", "commit.gpgsign", "false")
	return f
}

// newFixture is newBareFixture plus a minimal one-claim project: the config
// shape from hook-smoke-test.sh's new_project and one DRAFT claim created
// through the binary (never hand-written YAML — a hand-written claim would
// bypass the same write path the procedures under test rely on).
func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := newBareFixture(t)
	f.WriteProjectConfig()
	f.NewClaim(defaultClaimID, "the widget answers within 200ms.", "")
	return f
}

// WriteProjectConfig writes the minimal single-facet single-module config plus
// an empty claims/ dir. Exported as a builder (not folded into newFixture)
// because the bootstrap scenario must perform this exact write as one of its
// DOCUMENTED steps, in the documented position, not as setup.
func (f *fixture) WriteProjectConfig() {
	f.t.Helper()
	if err := os.MkdirAll(filepath.Join(f.root, "claims"), 0o755); err != nil {
		f.t.Fatalf("mkdir claims: %v", err)
	}
	cfg := "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"
	if err := os.WriteFile(filepath.Join(f.root, "project.config.yaml"), []byte(cfg), 0o644); err != nil {
		f.t.Fatalf("write project.config.yaml: %v", err)
	}
}

func (f *fixture) git(args ...string) {
	f.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = f.root
	cmd.Env = f.env()
	if out, err := cmd.CombinedOutput(); err != nil {
		f.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// env is the hermetic environment every child process gets: the caller's
// PATH and platform plumbing survive, but HOME points into the fixture and
// GIT_CONFIG_NOSYSTEM cuts the system config off, so no developer dotfile can
// change what a procedure does here. Variables that would smuggle outside
// state in are dropped by name.
func (f *fixture) env() []string {
	drop := map[string]bool{
		"HOME": true, "XDG_CONFIG_HOME": true,
		"GIT_CONFIG_GLOBAL": true, "GIT_CONFIG_SYSTEM": true,
		"GIT_AUTHOR_NAME": true, "GIT_AUTHOR_EMAIL": true,
		"GIT_COMMITTER_NAME": true, "GIT_COMMITTER_EMAIL": true,
	}
	var out []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if drop[name] {
			continue
		}
		out = append(out, kv)
	}
	return append(out,
		"HOME="+f.home,
		"XDG_CONFIG_HOME="+filepath.Join(f.home, ".config"),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_TERMINAL_PROMPT=0",
	)
}

// ---------------------------------------------------------------------------
// executing steps
// ---------------------------------------------------------------------------

// Plan declares the scenario's full ordered step list BEFORE any of it runs,
// registers the scenario in the package account, and arranges the executed-vs-
// declared comparison to run at cleanup — which testing runs even after a
// t.Fatal, so no exit path dodges it.
func (f *fixture) Plan(name string, steps ...string) {
	f.t.Helper()
	if f.planned {
		f.t.Fatalf("scenario %q called Plan twice; one scenario, one plan", name)
	}
	if len(steps) == 0 {
		f.t.Fatalf("scenario %q declared an empty plan; a scenario with no steps asserts nothing", name)
	}
	f.rec = &scenarioRecord{Name: name, Plan: steps}
	f.planned = true
	registryMu.Lock()
	registry = append(registry, f.rec)
	registryMu.Unlock()
	f.t.Cleanup(func() { verifyAccount(f.t, f.rec) })
}

func (f *fixture) record(step string) {
	if !f.planned {
		return // setup runs before Plan; only the declared procedure is accounted
	}
	registryMu.Lock()
	f.rec.Executed = append(f.rec.Executed, step)
	registryMu.Unlock()
}

// placeholderRe matches a whole <name> token. Substitution is whole-token so a
// bound value containing spaces stays a single argv element, exactly as a
// shell user quoting the flag value would get.
var placeholderRe = regexp.MustCompile(`^<([a-zA-Z][a-zA-Z0-9_-]*)>$`)

// substitute splits a documented command line into argv and binds every
// <placeholder> token. An unbound placeholder is fatal: it means the scenario
// quotes a step it did not fill in, and running it would exec a literal "<id>".
func (f *fixture) substitute(template string, bind map[string]string) []string {
	f.t.Helper()
	fields := strings.Fields(template)
	argv := make([]string, 0, len(fields))
	for _, tok := range fields {
		m := placeholderRe.FindStringSubmatch(tok)
		if m == nil {
			argv = append(argv, tok)
			continue
		}
		val, ok := bind[m[1]]
		if !ok {
			f.t.Fatalf("step %q names placeholder <%s> and the scenario bound no value for it", template, m[1])
		}
		argv = append(argv, val)
	}
	return argv
}

// Run executes one DOCUMENTED step: substitutes the placeholders, execs the
// binary from the project directory, decodes the JSON envelope when there is
// one, and records the step — template form, so the account reads like the
// document — into the scenario's ledger.
func (f *fixture) Run(template string, bind map[string]string) *invocation {
	f.t.Helper()
	inv := f.exec(template, bind)
	f.record(template)
	return inv
}

// Setup executes a state-building command that is NOT part of the documented
// procedure (locking the fixture claim, opening the human's thread, ...) and
// therefore is not recorded against the plan. It is fatal on any failure:
// setup that did not reach its state invalidates every assertion after it, and
// limping on would report defects in a state no document describes.
func (f *fixture) Setup(template string, bind map[string]string) *invocation {
	f.t.Helper()
	inv := f.exec(template, bind)
	if inv.Exit != 0 || (inv.Env != nil && !inv.Env.OK) {
		f.t.Fatalf("fixture setup failed: %s\nexit %d, error.code=%q\nstdout: %s\nstderr: %s",
			strings.Join(inv.Argv, " "), inv.Exit, inv.errorCode(), inv.Stdout, inv.Stderr)
	}
	return inv
}

func (f *fixture) exec(template string, bind map[string]string) *invocation {
	f.t.Helper()
	argv := f.substitute(template, bind)
	if len(argv) == 0 || argv[0] != "dossierx" {
		f.t.Fatalf("step %q does not start with \"dossierx\"; this harness executes the documented command lines verbatim", template)
	}
	cmd := exec.Command(f.bin, argv[1:]...)
	cmd.Dir = f.root
	cmd.Env = f.env()
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	exit := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exit = exitErr.ExitCode()
		} else {
			f.t.Fatalf("exec %s: %v", strings.Join(argv, " "), err)
		}
	}
	inv := &invocation{Step: template, Argv: argv, Exit: exit, Stdout: outBuf.String(), Stderr: errBuf.String()}
	// The envelope is the default output; --format text runs produce prose. A
	// run is only treated as enveloped when its stdout decodes as one, so text
	// steps assert on exit status alone rather than failing to parse.
	var env envelope
	if jsonErr := json.Unmarshal([]byte(inv.Stdout), &env); jsonErr == nil && env.Command != "" {
		inv.Env = &env
	}
	return inv
}

// Enact records a documented step that is not a dossierx invocation — a hand
// edit the document instructs, a file the document says to put in place, or an
// explicitly documented skip. The step string goes into the account verbatim,
// so the plan stays a complete transcript of the procedure, including the
// parts no binary executes. do may be nil for a pure "this step is a
// documented no-op here" entry.
func (f *fixture) Enact(step string, do func()) {
	f.t.Helper()
	if do != nil {
		do()
	}
	f.record(step)
}

// ---------------------------------------------------------------------------
// state builders
// ---------------------------------------------------------------------------

// NewClaim creates a claim through the binary. buildRole may be "" — several
// scenarios specifically need claims that never adopted build_role.
func (f *fixture) NewClaim(id, body, buildRole string) {
	f.t.Helper()
	tmpl := "dossierx claim new <id> --body <body> --governed-reason <why>"
	bind := map[string]string{"id": id, "body": body, "why": "procedure-suite fixture, not backed by any doctrine claim"}
	if buildRole != "" {
		tmpl += " --build-role <role>"
		bind["role"] = buildRole
	}
	f.Setup(tmpl, bind)
}

// LockClaim locks a claim with a stand-in for the human's approving words —
// the fixture equivalent of the "preview, ask, act" ceremony whose judgement
// half this harness cannot enact (see the package comment).
func (f *fixture) LockClaim(id, reason string) {
	f.t.Helper()
	f.Setup("dossierx claim lock <id> --reason <reason>", map[string]string{"id": id, "reason": reason})
}

// OpenHumanThread opens a comment thread as the human and returns the minted
// thread id from the envelope (data.thread_id — a field, never a regex over
// prose). The CLI's --as human is the same code path the viewer's composer
// reaches, per the comments skill, so this is the honest simulation of loop
// step 1 available without a browser.
func (f *fixture) OpenHumanThread(claimID, body string) string {
	f.t.Helper()
	inv := f.Setup("dossierx comment add <id> --as human --body <body>", map[string]string{"id": claimID, "body": body})
	if inv.Env == nil {
		f.t.Fatalf("comment add printed no envelope; cannot learn the minted thread id\nstdout: %s", inv.Stdout)
	}
	tid, _ := inv.Env.Data["thread_id"].(string)
	if tid == "" {
		f.t.Fatalf("comment add's envelope carries no data.thread_id: %s", inv.Stdout)
	}
	return tid
}

// claimFile is the path claim new writes a claim to: claims/<full-id>.yaml
// (the same shape hook-smoke-test.sh's tamper() relies on).
func (f *fixture) claimFile(id string) string {
	return filepath.Join(f.root, "claims", id+".yaml")
}

// RewriteClaimBody performs the draft-claim hand edit the documents license
// ("draft is your workshop"). Two refusals keep the enactment honest:
//
//   - It refuses to be a silent no-op — an edit that matched nothing means the
//     fixture is not in the state the scenario thinks.
//   - It refuses to touch the engine-managed comments: block. The comments
//     skill forbids hand-editing that block in exactly those words ("Never
//     hand-edit the `comments:` block in a claim file"), and an enactment that
//     did so anyway would manufacture a comment_digest_drift the DOCUMENT
//     never caused — a fixture artifact indistinguishable from a finding,
//     which is worse than no finding. The first run of this suite produced
//     precisely that artifact (the thread's text shared a token with the
//     body), which is why this split exists.
func (f *fixture) RewriteClaimBody(id, old, new string) {
	f.t.Helper()
	path := f.claimFile(id)
	raw, err := os.ReadFile(path)
	if err != nil {
		f.t.Fatalf("read %s: %v", path, err)
	}
	head := string(raw)
	tail := ""
	// The engine writes comments as a top-level "comments:" key; splitting at
	// its line boundary fences the edit off from everything under it.
	if idx := strings.Index(head, "\ncomments:"); idx >= 0 {
		head, tail = head[:idx], head[idx:]
	}
	if !strings.Contains(head, old) {
		f.t.Fatalf("claim %s's own content does not contain %q; the fixture is not in the state this edit assumes", id, old)
	}
	if strings.Contains(tail, old) {
		f.t.Fatalf("the edit %q -> %q would also match inside claim %s's engine-managed comments: block; pick fixture text that does not collide, or this enactment would hand-edit review history and manufacture a drift the document never caused", old, new, id)
	}
	if err := os.WriteFile(path, []byte(strings.ReplaceAll(head, old, new)+tail), 0o644); err != nil {
		f.t.Fatalf("write %s: %v", path, err)
	}
}

// SetBuildRoleByHand appends build_role to a claim FILE — there is no command
// that sets it after creation, which is why the build-order skill's recovery
// row routes through unlock first. On a DRAFT claim this is the ordinary
// workshop edit, and that is where the recovery scenario now calls it; on a
// LOCKED one it would be the edit the ledger exists to catch, which the old
// "set it, then re-propose" row instructed verbatim until it was fixed.
func (f *fixture) SetBuildRoleByHand(id, role string) {
	f.t.Helper()
	path := f.claimFile(id)
	fh, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		f.t.Fatalf("open %s: %v", path, err)
	}
	defer fh.Close()
	if _, err := fmt.Fprintf(fh, "build_role: %s\n", role); err != nil {
		f.t.Fatalf("append build_role to %s: %v", path, err)
	}
}

// RewindStoreToPreLedger rewrites the lock store as a pre-ledger (v0.2.x)
// build would have left it — file present, schema version 1, no ledger key —
// and removes the comment digest store, WHICH IS NOT TIDINESS: the downgrade
// detector treats a digest store beside a "version: 1" lock store as proof the
// project has been through a ledger-aware build (the digest file did not exist
// before v0.3.0), so rewinding only the store would reproduce the downgrade
// ATTACK, not an honest old project, and the gate would be right to say so.
// cmd/dossierx/integrity_gates_test.go's rewindStoreToPreLedger documents the
// same two-file requirement; this is its exec-boundary twin.
func (f *fixture) RewindStoreToPreLedger() {
	f.t.Helper()
	storePath := filepath.Join(f.root, ".dossierx-lock-store.json")
	raw, err := os.ReadFile(storePath)
	if err != nil {
		f.t.Fatalf("read %s (the fixture must lock something before rewinding): %v", storePath, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		f.t.Fatalf("decode %s: %v", storePath, err)
	}
	delete(doc, "ledger")
	doc["version"] = 1
	rewound, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		f.t.Fatalf("re-encode rewound store: %v", err)
	}
	if err := os.WriteFile(storePath, rewound, 0o644); err != nil {
		f.t.Fatalf("write rewound store: %v", err)
	}
	digest := filepath.Join(f.root, ".dossierx-comment-digest.json")
	if err := os.Remove(digest); err != nil && !os.IsNotExist(err) {
		f.t.Fatalf("remove %s: %v", digest, err)
	}
}

// ---------------------------------------------------------------------------
// the HTTP surface — the human's half of the loop
// ---------------------------------------------------------------------------

var serveURLRe = regexp.MustCompile(`http://127\.0\.0\.1:[0-9]+`)

// StartServe runs "dossierx serve" against the fixture project on an
// ephemeral port, waits until /api/ping answers 200, and registers an orderly
// stop. The wait is not optional courtesy: a resolve POST racing serve's
// startup would fail with a connection error that reads like a defect in the
// procedure under test. SIGINT first, kill as fallback — serve drains
// in-flight comment writes on SIGINT so no sentinel lock file is left behind
// for the scenario's NEXT CLI step to trip over (write_conflict), which a bare
// kill can cause.
func (f *fixture) StartServe() string {
	f.t.Helper()
	if f.serveBase != "" {
		return f.serveBase
	}
	f.serveOut = filepath.Join(f.dir, "serve.out")
	out, err := os.Create(f.serveOut)
	if err != nil {
		f.t.Fatalf("create serve.out: %v", err)
	}
	cmd := exec.Command(f.bin, "serve")
	cmd.Dir = f.root
	cmd.Env = f.env()
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Start(); err != nil {
		out.Close()
		f.t.Fatalf("start dossierx serve: %v", err)
	}
	f.serveCmd = cmd
	f.t.Cleanup(func() {
		f.stopServe()
		out.Close()
	})

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		raw, _ := os.ReadFile(f.serveOut)
		if base := serveURLRe.FindString(string(raw)); base != "" {
			f.serveBase = base
			break
		}
		if cmd.ProcessState != nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if f.serveBase == "" {
		raw, _ := os.ReadFile(f.serveOut)
		f.t.Fatalf("dossierx serve never printed its URL; a scenario that needs the viewer surface cannot run without it, and cannot pass without running\nserve output: %s", raw)
	}
	for time.Now().Before(deadline) {
		resp, err := http.Get(f.serveBase + "/api/ping")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return f.serveBase
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	f.t.Fatalf("dossierx serve printed %s but /api/ping never answered 200 within the deadline", f.serveBase)
	return ""
}

func (f *fixture) stopServe() {
	if f.serveCmd == nil || f.serveCmd.Process == nil {
		return
	}
	_ = f.serveCmd.Process.Signal(os.Interrupt)
	done := make(chan struct{})
	go func() {
		_, _ = f.serveCmd.Process.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = f.serveCmd.Process.Kill()
		<-done
	}
	f.serveCmd = nil
}

// ResolveThreadAsHuman performs loop step 5 — the human's Resolve click — on
// the surface the human actually uses: the viewer's HTTP API, with a
// same-origin Origin header and an empty body whose absent "as" field the
// server deliberately reads as human. This is the ONE place this suite may
// touch that endpoint: on any other path, curling resolve is forging the
// human's approval, which the comments skill forbids in exactly those words.
// The call is recorded into the account like any other documented step.
func (f *fixture) ResolveThreadAsHuman(step, claimID, threadID string) {
	f.t.Helper()
	base := f.StartServe()
	url := base + "/api/claims/" + claimID + "/comments/" + threadID + "/resolve"
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader("{}"))
	if err != nil {
		f.t.Fatalf("build resolve request: %v", err)
	}
	req.Header.Set("Origin", base)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		f.t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		buf := make([]byte, 4096)
		n, _ := resp.Body.Read(buf)
		f.t.Fatalf("the human's Resolve click failed: POST %s -> HTTP %d\n%s", url, resp.StatusCode, string(buf[:n]))
	}
	f.record(step)
}

// ---------------------------------------------------------------------------
// assertions
// ---------------------------------------------------------------------------

// DocumentedSuccess asserts the outcome the document promises for this step:
// the command succeeds. It is non-fatal (t.Errorf) on purpose — the scenario
// keeps executing the REST of the documented procedure so the account stays
// complete and the report shows every divergence in one run, not one per run.
// The failure text carries error.code, exit, stopped_at and any ledger rule
// names as OBSERVATIONS for the human reading the report; no assertion here
// or anywhere else keys on message prose.
func (f *fixture) DocumentedSuccess(inv *invocation, doc string) {
	f.t.Helper()
	ok := inv.Exit == 0 && (inv.Env == nil || inv.Env.OK)
	if ok {
		return
	}
	stopped := ""
	if inv.Env != nil {
		stopped = inv.Env.StoppedAt
	}
	f.t.Errorf("FINDING — the documented step failed when executed as documented.\n  step:        %s\n  document:    %s\n  observed:    exit %d, error.code=%q, stopped_at=%q, ledger rules=%v\n  invocation:  %s\n  stdout:      %s",
		inv.Step, doc, inv.Exit, inv.errorCode(), stopped, inv.ledgerRules(), strings.Join(inv.Argv, " "), strings.TrimSpace(inv.Stdout))
}

// RequireFailure asserts a step fails with a specific error.code — used only
// where the DOCUMENT itself promises the refusal (a premise, like "propose
// refuses a claim with no build_role"). The code named by the test is first
// checked against surface.json's vocabulary and the expected exit status is
// READ from surface.json, never typed: if the vocabulary drops or renames the
// code, this fails naming the inventory, not with a mysterious mismatch.
func (f *fixture) RequireFailure(inv *invocation, code, why string) {
	f.t.Helper()
	i := surfaceInventory(f.t)
	if !i.ErrorCodes[code] {
		f.t.Fatalf("this scenario expects error code %q, which surface.json's error_codes inventory does not contain — either the scenario typo'd it or the vocabulary moved; reconcile with the inventory before trusting any result here", code)
	}
	wantExit, okExit := i.ExitCodes[code]
	if !okExit {
		f.t.Fatalf("surface.json's envelope.exit_codes carries no exit status for %q; the derived expectation this assertion needs is gone", code)
	}
	if inv.errorCode() != code || inv.Exit != wantExit {
		f.t.Fatalf("premise failed — %s\n  expected: error.code=%q with exit %d (exit read from surface.json)\n  observed: error.code=%q with exit %d\n  invocation: %s\n  stdout: %s",
			why, code, wantExit, inv.errorCode(), inv.Exit, strings.Join(inv.Argv, " "), strings.TrimSpace(inv.Stdout))
	}
}
