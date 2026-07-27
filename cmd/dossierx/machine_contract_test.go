// machine_contract_test.go pins the three promises the v0.3.0 envelope makes to
// a machine reader, each of which had a hole in it that only an actual consumer
// would ever have noticed:
//
//  1. every key in every envelope is snake_case, on EVERY command;
//  2. every way of asking a question answers in an envelope — including
//     "--version", which is a flag rather than a verb;
//  3. every --dry-run agrees with the write path it previews. A preview is the
//     one output an agent is told it may show a human before asking for a yes,
//     so a preview that says "blocked: false" for an invocation the real run
//     refuses is worse than having no preview at all.
//
// The dry-run tests here are all shaped the same way on purpose: run the
// preview, then run the REAL command against the same fixture, and assert the
// two agreed about whether it would be refused. Asserting the preview alone
// would only pin whatever the preview currently says.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
	dxskills "github.com/BarterX-Tech/dossierx/skills"
)

// ---------------------------------------------------------------------
// 1. snake_case keys, everywhere
// ---------------------------------------------------------------------

// hasUpperASCII is the leak detector. It looks for an uppercase letter in a JSON
// key rather than for a full snake_case match, because some payload maps are
// keyed by DATA — check's open_comments is keyed by module name, a dry run's
// "proposed" block by whatever the verb proposes — and those keys are the
// project's vocabulary, not this contract's. An uppercase letter, on the other
// hand, can only come from encoding/json falling back to a Go FIELD NAME, which
// is exactly the defect: model.Comment carries yaml tags and no json tags, so
// "dossierx comment list" emitted "ID"/"Status"/"ResolvedBy" while every other
// command emitted snake_case.
func hasUpperASCII(s string) bool {
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			return true
		}
	}
	return false
}

// walkJSONKeys calls visit for every object key anywhere in v, with the path
// that reached it, so a failure names the field rather than just the key.
func walkJSONKeys(v any, path string, visit func(path, key string)) {
	switch t := v.(type) {
	case map[string]any:
		for k, sub := range t {
			visit(path, k)
			walkJSONKeys(sub, path+"."+k, visit)
		}
	case []any:
		for _, sub := range t {
			walkJSONKeys(sub, path+"[]", visit)
		}
	}
}

// TestEveryEnvelopeKeyIsSnakeCase drives one command per payload shape against a
// live fixture and inspects the RAW bytes, which is the only vantage point from
// which the defect was visible: decoding into the payload struct reads the Go
// field names back just as happily as it wrote them out, so a typed test cannot
// see the leak at all.
func TestEveryEnvelopeKeyIsSnakeCase(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")

	// A thread with a reply, so comment list's whole tree — thread fields AND
	// reply fields — is exercised rather than just the root of it.
	env, _, err := execCLIJSON(t, "--config", cfgPath, "comment", "add", "widget.contract.overview", "--as", "human", "--body", "does this still hold?")
	if err != nil {
		t.Fatalf("comment add: %v", err)
	}
	var added commentWriteData
	envData(t, env, &added)
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "comment", "reply", "widget.contract.overview", added.ThreadID, "--as", "agent", "--body", "checked, it holds"); err != nil {
		t.Fatalf("comment reply: %v", err)
	}

	commands := [][]string{
		{"version"},
		{"check", "--validate"},
		{"claim", "show", "widget.contract.overview"},
		{"claim", "list"},
		{"claim", "new", "widget.contract.second", "--body", "another claim", "--dry-run"},
		{"claim", "lock", "widget.contract.overview", "--reason", "r", "--dry-run"},
		{"claim", "unlock", "widget.contract.overview", "--reason", "r", "--dry-run"},
		{"claim", "flag", "widget.contract.overview", "--claim-says", "a", "--now-does", "b", "--reason", "c", "--dry-run"},
		{"claim", "link", "--module", "widget", "--claim", "widget.contract.overview", "--file", "claims/overview.yaml", "--dry-run"},
		{"comment", "list", "widget.contract.overview"},
		{"comment", "inbox"},
		{"comment", "add", "widget.contract.overview", "--as", "agent", "--body", "b", "--dry-run"},
		{"comment", "reply", "widget.contract.overview", added.ThreadID, "--as", "agent", "--body", "b", "--dry-run"},
		{"build-order", "status", "--module", "widget"},
		{"build-order", "propose", "--module", "widget", "--dry-run"},
		{"build-order", "lock", "--module", "widget", "--reason", "r", "--dry-run"},
	}

	for _, args := range commands {
		name := strings.Join(args, " ")
		t.Run(name, func(t *testing.T) {
			out := captureJSONBytes(t, append([]string{"--config", cfgPath}, args...)...)
			var doc any
			if err := json.Unmarshal(out, &doc); err != nil {
				t.Fatalf("%s: stdout is not one JSON envelope (%v):\n%s", name, err, out)
			}
			walkJSONKeys(doc, "", func(path, key string) {
				if hasUpperASCII(key) {
					t.Errorf("%s: envelope key %q at %s is not snake_case — an untagged Go struct is leaking its field names into the machine contract", name, key, path)
				}
			})
		})
	}
}

// captureJSONBytes runs the CLI in-process under the default (JSON) format and
// returns stdout verbatim. It exists because execCLIJSON decodes into an
// Envelope and this test has to look at the bytes.
func captureJSONBytes(t *testing.T, args ...string) []byte {
	t.Helper()
	env, _, _ := execCLIJSON(t, args...)
	// Re-marshalling the decoded envelope would re-tag everything through
	// cliout's own tags and hide the very thing under test, so run again and
	// keep the raw stdout instead.
	raw := rawStdout(t, args...)
	if len(raw) == 0 {
		t.Fatalf("no stdout for %v (decoded envelope: %+v)", args, env)
	}
	return raw
}

func rawStdout(t *testing.T, args ...string) []byte {
	t.Helper()
	root := newRootCmd()
	var outBuf, errBuf strings.Builder
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs(append([]string{"--format", formatJSON}, args...))
	_ = runCLI(root) //nolint:errcheck // a refused command still emits an envelope, which is what is inspected
	return []byte(outBuf.String())
}

// jsonTagName is the snake_case grammar every field in the contract uses.
var jsonTagName = regexp.MustCompile(`^[a-z0-9]+(_[a-z0-9]+)*$`)

// TestEnvelopePayloadTypesDeclareSnakeCaseJSONTags is the declaration-time twin
// of the test above, and the one that generalizes: it walks every payload type
// this package publishes — through the nested structs, slices and maps they
// reach — and requires an explicit, snake_case json tag on every exported field.
//
// It catches the class rather than the instance. A future payload that embeds a
// type from internal/model or internal/lock, both of which are persistence
// shapes carrying yaml/no tags, fails here the moment it is declared instead of
// the first time an agent reads the output and finds nothing under the key the
// contract promised.
func TestEnvelopePayloadTypesDeclareSnakeCaseJSONTags(t *testing.T) {
	payloads := []any{
		versionData{},
		checkData{},
		claimShowData{},
		claimListData{},
		claimNewData{},
		claimLinkData{},
		lockData{},
		lockRefusedData{},
		unlockData{},
		reauditData{},
		flagData{},
		commentWriteData{},
		commentListData{},
		commentInboxData{},
		buildOrderProposeData{},
		buildOrderStatusData{},
		buildOrderLockData{},
		skillsExportData{},
		cliout.DryRun{},
		cliout.Envelope{},
	}
	for _, p := range payloads {
		rt := reflect.TypeOf(p)
		t.Run(rt.Name(), func(t *testing.T) {
			requireJSONTags(t, rt, rt.Name(), map[reflect.Type]bool{})
		})
	}
}

func requireJSONTags(t *testing.T, rt reflect.Type, path string, seen map[reflect.Type]bool) {
	t.Helper()
	for rt.Kind() == reflect.Pointer || rt.Kind() == reflect.Slice || rt.Kind() == reflect.Array || rt.Kind() == reflect.Map {
		rt = rt.Elem()
	}
	if rt.Kind() != reflect.Struct || seen[rt] {
		return
	}
	seen[rt] = true

	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if f.PkgPath != "" {
			continue // unexported: encoding/json never emits it
		}
		tag, ok := f.Tag.Lookup("json")
		if !ok {
			t.Errorf("%s.%s has no json tag, so it marshals under its Go field name", path, f.Name)
			continue
		}
		name := strings.Split(tag, ",")[0]
		if name == "-" {
			continue
		}
		if !jsonTagName.MatchString(name) {
			t.Errorf("%s.%s has json tag %q, which is not snake_case", path, f.Name, name)
		}
		requireJSONTags(t, f.Type, path+"."+f.Name, seen)
	}
}

// ---------------------------------------------------------------------
// 2. --version answers in an envelope
// ---------------------------------------------------------------------

// TestVersionFlagAnswersInAnEnvelope pins the fix for the last hole of the
// bare-noun shape: cobra's built-in --version printed a prose line on stdout and
// exited 0 without ever reaching a RunE, so the one binary that emits JSON by
// default answered its own version question in a sentence.
func TestVersionFlagAnswersInAnEnvelope(t *testing.T) {
	raw := rawStdout(t, "--version")
	var env cliout.Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("--version did not emit one JSON envelope (%v); got:\n%s", err, raw)
	}
	if !env.OK {
		t.Fatalf("--version must succeed, got: %s", raw)
	}
	// The name of the thing that was ASKED FOR, not the empty string
	// commandPath(root) would produce: a caller correlates a response with its
	// call on this field.
	if env.Command != "version" {
		t.Fatalf(`expected command "version", got %q (%s)`, env.Command, raw)
	}
	var data versionData
	envData(t, env, &data)
	if data.Name != "dossierx" || data.Version == "" || data.Commit == "" || data.Date == "" {
		t.Fatalf("--version payload drift: %+v", data)
	}
}

// TestVersionFlagAndVerbAgree: two spellings of one question must not be able to
// answer differently, which is exactly what happened while one of them was
// cobra's built-in and the other was a converted command.
func TestVersionFlagAndVerbAgree(t *testing.T) {
	flagEnv, _, err := execCLIJSON(t, "--version")
	if err != nil {
		t.Fatalf("--version: %v", err)
	}
	verbEnv, _, err := execCLIJSON(t, "version")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	var viaFlag, viaVerb versionData
	envData(t, flagEnv, &viaFlag)
	envData(t, verbEnv, &viaVerb)
	if viaFlag != viaVerb {
		t.Fatalf("--version and version disagree: %+v vs %+v", viaFlag, viaVerb)
	}
}

// ---------------------------------------------------------------------
// 3. every --dry-run agrees with its own write path
// ---------------------------------------------------------------------

// dryRunBlocked runs a --dry-run and returns its verdict plus the names of the
// preconditions that failed.
func dryRunBlocked(t *testing.T, args ...string) (blocked bool, missing []string) {
	t.Helper()
	env, _, err := execCLIJSON(t, append(args, "--dry-run")...)
	if err != nil {
		t.Fatalf("%v --dry-run failed outright (a dry run answers, it does not refuse): %v", args, err)
	}
	var dr cliout.DryRun
	envData(t, env, &dr)
	return dr.Blocked, dr.Missing
}

// assertDryRunAgrees is the whole shape of these tests: preview, then really
// run, then compare the two verdicts.
func assertDryRunAgrees(t *testing.T, wantBlocked bool, args ...string) {
	t.Helper()
	blocked, missing := dryRunBlocked(t, args...)
	_, _, realErr := execCLIJSON(t, args...)
	realRefused := realErr != nil
	if blocked != realRefused {
		t.Fatalf("%v: dry run says blocked=%v (missing %v) but the real run %s",
			args, blocked, missing,
			map[bool]string{true: "REFUSED: " + errText(realErr), false: "SUCCEEDED"}[realRefused])
	}
	if wantBlocked != blocked {
		t.Fatalf("%v: expected blocked=%v, got %v (missing %v)", args, wantBlocked, blocked, missing)
	}
}

func errText(err error) string {
	if err == nil {
		return "<nil>"
	}
	return err.Error()
}

// TestUnlockDryRunAgreesOnADraftClaim.
//
// "claim unlock" has no gates by design — it is the recovery escape hatch, and
// a project may need it precisely to fix what something else is complaining
// about — so unlocking an already-draft claim succeeds and exits 0. The preview
// declared "claim_is_locked" a precondition and reported blocked:true for it,
// which is the disagreement in its most damaging direction: an agent reading the
// preview would not reach for the one command that gets a wedged project moving.
func TestUnlockDryRunAgreesOnADraftClaim(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")

	assertDryRunAgrees(t, false,
		"--config", cfgPath, "claim", "unlock", "widget.contract.overview", "--reason", "already draft, but allowed")
}

// TestClaimLinkDryRunAgreesWithTheWritePath covers the three refusals implink.Set
// performs that the preview did not evaluate at all — a --file that does not
// exist, a --file that is absolute or escapes the project, and a --claim that
// belongs to a different module than --module — plus the happy path, so the
// added preconditions cannot be satisfied by simply always blocking.
func TestClaimLinkDryRunAgreesWithTheWritePath(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")
	// Two modules, so "the claim is in a different module" is expressible.
	cfg := "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\n  - gadget\nclaims_dir: claims\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("rewrite config: %v", err)
	}
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.overview", "--reason", "fixture"); err != nil {
		t.Fatalf("lock the claim so it is linkable: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "impl.go"), []byte("package impl\n"), 0o644); err != nil {
		t.Fatalf("write the implementing file: %v", err)
	}

	base := []string{"--config", cfgPath, "claim", "link", "--claim", "widget.contract.overview"}
	cases := []struct {
		name        string
		args        []string
		wantBlocked bool
	}{
		{"missing file", []string{"--module", "widget", "--file", "no/such/file.go"}, true},
		{"absolute file", []string{"--module", "widget", "--file", filepath.Join(root, "impl.go")}, true},
		{"escaping file", []string{"--module", "widget", "--file", filepath.Join("..", "impl.go")}, true},
		{"wrong module", []string{"--module", "gadget", "--file", "impl.go"}, true},
		{"linkable", []string{"--module", "widget", "--file", "impl.go"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertDryRunAgrees(t, tc.wantBlocked, append(append([]string{}, base...), tc.args...)...)
		})
	}
}

// TestClaimLinkDryRunOnADraftClaimStaysBlocked guards the precondition that was
// already there while the three above were added around it.
func TestClaimLinkDryRunOnADraftClaimStaysBlocked(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")
	if err := os.WriteFile(filepath.Join(root, "impl.go"), []byte("package impl\n"), 0o644); err != nil {
		t.Fatalf("write the implementing file: %v", err)
	}
	assertDryRunAgrees(t, true,
		"--config", cfgPath, "claim", "link", "--module", "widget", "--claim", "widget.contract.overview", "--file", "impl.go")
}

// TestCommentWriteDryRunAgreesWithTheWritePath covers the two input refusals the
// write path performs immediately and the preview ignored: an --as that is
// neither role, and a body that cannot survive a YAML round trip.
func TestCommentWriteDryRunAgreesWithTheWritePath(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")

	// A first content line indented by a tab: yaml.v3 re-parses it lossily, so
	// internal/comments refuses it outright (ErrUnsafeBody).
	const unsafeBody = "\tcode line\nsecond line"

	cases := []struct {
		name        string
		args        []string
		wantBlocked bool
	}{
		{"unknown actor", []string{"--as", "robot", "--body", "hello"}, true},
		{"unstorable body", []string{"--as", "agent", "--body", unsafeBody}, true},
		{"ordinary thread", []string{"--as", "agent", "--body", "hello"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"--config", cfgPath, "comment", "add", "widget.contract.overview"}, tc.args...)
			assertDryRunAgrees(t, tc.wantBlocked, args...)
		})
	}
}

// TestBuildOrderLockDryRunAgreesOnAStaleOrder.
//
// buildorder.Lock refuses a STALE order before it looks at anything else — a
// bare relock would freeze an order whose claims have moved — and the preview
// not only failed to check it, its "not_already_current" gate actively PASSED a
// stale order (it reads !locked || stale). So the one artifact state that always
// refuses previewed as go-ahead.
func TestBuildOrderLockDryRunAgreesOnAStaleOrder(t *testing.T) {
	root := t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims: %v", err)
	}
	cfgPath := filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte("schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	claimPath := filepath.Join(claimsDir, "a.yaml")
	claim := "id: widget.contract.a\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
		"build_role: schema\nbody: |\n  claim a.\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
	if err := os.WriteFile(claimPath, []byte(claim), 0o644); err != nil {
		t.Fatalf("write claim: %v", err)
	}

	mustSucceed := func(args ...string) {
		t.Helper()
		if _, _, err := execCLIJSON(t, append([]string{"--config", cfgPath}, args...)...); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	mustSucceed("claim", "lock", "widget.contract.a", "--reason", "fixture")
	mustSucceed("build-order", "propose", "--module", "widget")
	mustSucceed("build-order", "lock", "--module", "widget", "--reason", "fixture")

	// Make the order stale the sanctioned way: unlock, edit, relock. The
	// artifact's recorded claim hash no longer matches.
	mustSucceed("claim", "unlock", "widget.contract.a", "--reason", "fixing it")
	onDisk, err := os.ReadFile(claimPath)
	if err != nil {
		t.Fatalf("read claim: %v", err)
	}
	if err := os.WriteFile(claimPath, []byte(strings.Replace(string(onDisk), "claim a.", "claim a, revised.", 1)), 0o644); err != nil {
		t.Fatalf("rewrite claim: %v", err)
	}
	mustSucceed("claim", "lock", "widget.contract.a", "--reason", "fixture")

	assertDryRunAgrees(t, true,
		"--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "relock it")
}

// TestBuildOrderLockDryRunAgreesOnAHandEditedOrder.
//
// The hand-edit gate (buildorder.ErrHandEdited) is the LAST refusal Lock makes
// and it was the only one the preview could not see. It is also the one the
// preview most needed: an artifact whose phase blocks were reversed by hand
// between propose and lock is UNLOCKED, so it is never stale (staleness is a
// locked-artifact concept and recomputeStale early-returns on an unlocked one)
// and it is not already current — meaning every precondition the preview did
// evaluate passed. The run previewed blocked:false and then exited 1.
//
// Reversing the phase blocks is the specific edit chosen here because it is the
// one that survives a per-claim comparison: a claim's signature is
// phase/position-within-phase/File, so moving whole blocks leaves every
// signature byte-identical. Only the explicit phase-sequence comparison catches
// it, which is exactly the check that lived behind an unexported function.
func TestBuildOrderLockDryRunAgreesOnAHandEditedOrder(t *testing.T) {
	root := t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims: %v", err)
	}
	cfgPath := filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte("schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	// Two claims in DIFFERENT phases, so the artifact has two phase blocks to
	// reverse. One claim could not express this edit at all.
	for _, c := range []struct{ name, id, role string }{
		{"a", "widget.contract.a", "schema"},
		{"b", "widget.contract.b", "behavior"},
	} {
		claim := "id: " + c.id + "\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"build_role: " + c.role + "\nbody: |\n  claim " + c.name + ".\n" +
			"governed_by:\n  type: none\n  reason: fixture\n"
		if err := os.WriteFile(filepath.Join(claimsDir, c.name+".yaml"), []byte(claim), 0o644); err != nil {
			t.Fatalf("write claim %s: %v", c.id, err)
		}
	}

	mustSucceed := func(args ...string) {
		t.Helper()
		if _, _, err := execCLIJSON(t, append([]string{"--config", cfgPath}, args...)...); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	mustSucceed("claim", "lock", "widget.contract.a", "--reason", "fixture")
	mustSucceed("claim", "lock", "widget.contract.b", "--reason", "fixture")
	mustSucceed("build-order", "propose", "--module", "widget")

	// A freshly proposed order previews AND locks cleanly. Asserting this first
	// is what keeps the test honest: without it, a preview that blocked
	// unconditionally would pass the assertion below.
	artifactPath := filepath.Join(root, ".build-order.widget.json")
	pristine, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	blocked, missing := dryRunBlocked(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "approve")
	if blocked {
		t.Fatalf("a freshly proposed order must preview as go-ahead, got blocked (missing %v)", missing)
	}

	// Now the edit: reverse the phase BLOCKS and change nothing else.
	var doc map[string]any
	if err := json.Unmarshal(pristine, &doc); err != nil {
		t.Fatalf("parse artifact: %v", err)
	}
	phases, ok := doc["phases"].([]any)
	if !ok || len(phases) < 2 {
		t.Fatalf("fixture must produce at least two phase blocks, got %v", doc["phases"])
	}
	for i, j := 0, len(phases)-1; i < j; i, j = i+1, j-1 {
		phases[i], phases[j] = phases[j], phases[i]
	}
	doc["phases"] = phases
	edited, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal artifact: %v", err)
	}
	if err := os.WriteFile(artifactPath, edited, 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}

	assertDryRunAgrees(t, true,
		"--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "approve")
}

// ---------------------------------------------------------------------
// 4. one condition, one error code
// ---------------------------------------------------------------------

// TestClaimsSentinelContentionIsAWriteConflictOnEveryVerb.
//
// The project-wide claims sentinel is taken by two different kinds of writer:
// cmd/dossierx takes it directly (lock/unlock/flag/reaudit/new) and wraps the
// failure as write_conflict, while internal/comments takes it INSIDE its own ops
// and returned the raw error, which errorForCLI could not classify and reported
// as `internal`. The contract defines `internal` as an unclassified bug whose
// documented response is to retry and, failing that, file one — so the identical
// condition on the identical file told an agent two different stories, and the
// comment one was actively wrong.
//
// The contention is produced by making the project directory unwritable, which
// fails the sentinel's create immediately rather than after its ten-second
// acquire timeout.
func TestClaimsSentinelContentionIsAWriteConflictOnEveryVerb(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permission bits do not gate file creation on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permission bits")
	}
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")

	if err := os.Chmod(root, 0o555); err != nil {
		t.Fatalf("make the project dir read-only: %v", err)
	}
	t.Cleanup(func() { os.Chmod(root, 0o755) }) //nolint:errcheck // best-effort restore so TempDir cleanup works

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"comment add", []string{"comment", "add", "widget.contract.overview", "--as", "agent", "--body", "hello"}},
		{"claim unlock", []string{"claim", "unlock", "widget.contract.overview", "--reason", "r"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env, _, err := execCLIJSON(t, append([]string{"--config", cfgPath}, tc.args...)...)
			if err == nil {
				t.Fatalf("expected %v to be refused on a read-only project dir", tc.args)
			}
			if env.Error == nil || env.Error.Code != cliout.CodeWriteConflict {
				t.Fatalf("expected %s for %v, got %+v", cliout.CodeWriteConflict, tc.args, env.Error)
			}
		})
	}
}

// ---------------------------------------------------------------------
// 5. the skill enumerates every stopped_at the CLI can emit
// ---------------------------------------------------------------------

// TestRouterSkillEnumeratesEveryStoppedAtValue pins the router skill's published
// stopped_at value set against the values this package actually assigns.
//
// stopped_at is a closed vocabulary an agent branches on, and the skill is the
// only place it is written down. A value the CLI emits but the skill omits is
// read by an agent as an unrecognized state — and "ledger" is the one that
// matters most, because reaching it means the catalog and viewer WERE
// regenerated and only the commit is refused: a gate, not an outage. The
// emitted set is read out of the source rather than restated here, so adding a
// step without documenting it fails this test.
func TestRouterSkillEnumeratesEveryStoppedAtValue(t *testing.T) {
	emitted := stoppedAtValuesInSource(t)
	if len(emitted) < 5 {
		t.Fatalf("only found %v stopped_at values in the source; the scan is broken, not the skill", emitted)
	}

	skill, err := dxskills.FS.ReadFile("dossierx/SKILL.md")
	if err != nil {
		t.Fatalf("read the router skill: %v", err)
	}
	// The one paragraph that enumerates the set.
	text := string(skill)
	idx := strings.Index(text, "`stopped_at` names the pipeline step")
	if idx < 0 {
		t.Fatal("the router skill no longer documents stopped_at at all")
	}
	paragraph := text[idx:min(idx+400, len(text))]

	for _, v := range emitted {
		if !strings.Contains(paragraph, "`"+v+"`") {
			t.Errorf("the CLI emits stopped_at %q but the router skill's value set does not list it:\n%s", v, paragraph)
		}
	}
}

// stoppedAtValuesInSource collects every literal this package assigns to a
// cmdResult's StoppedAt, plus the ones checkStoppedAt returns, by reading the
// source. Deriving the set beats restating it: a restated list is one more thing
// that can silently fall behind the code.
func stoppedAtValuesInSource(t *testing.T) []string {
	t.Helper()
	assign := regexp.MustCompile(`StoppedAt(?::|\s*=)\s*"([a-z]+)"`)
	ret := regexp.MustCompile(`return\s+"([a-z]+)"`)

	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	text := string(src)

	seen := map[string]bool{}
	for _, m := range assign.FindAllStringSubmatch(text, -1) {
		seen[m[1]] = true
	}
	// checkStoppedAt's own returns, which never pass through a StoppedAt field
	// literal.
	if start := strings.Index(text, "func checkStoppedAt("); start >= 0 {
		body := text[start:]
		if end := strings.Index(body, "\n}\n"); end > 0 {
			body = body[:end]
		}
		for _, m := range ret.FindAllStringSubmatch(body, -1) {
			seen[m[1]] = true
		}
	}
	delete(seen, "")

	out := make([]string, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	return out
}
