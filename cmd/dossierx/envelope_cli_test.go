// envelope_cli_test.go is the golden-fixture suite for the machine contract: for
// every one of the nineteen leaves, it runs the real command tree with the
// default (JSON) format and pins the envelope the agent actually receives.
//
// TestEveryLeafButServeEmitsAnEnvelope below is the coverage floor — every leaf
// answers --format json with exactly one envelope — and the per-command tests
// after it pin what is IN those envelopes for the cases a skill branches on.
//
// The exact byte-for-byte prose fixtures live next door in
// check_parity_test.go, and every test in this package that asserts prose goes
// through execCLI (which pins --format text). This file's twin helper,
// execCLIJSON, pins --format json. The two together are the whole "JSON by
// default, byte-identical prose on demand" claim, tested from both sides.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
)

// envData re-decodes an envelope's Data (an any, so it arrives as a
// map[string]any) into a typed shape the test can assert on precisely.
func envData(t *testing.T, env cliout.Envelope, into any) {
	t.Helper()
	raw, err := json.Marshal(env.Data)
	if err != nil {
		t.Fatalf("re-marshal envelope data: %v", err)
	}
	if err := json.Unmarshal(raw, into); err != nil {
		t.Fatalf("decode envelope data into %T: %v (raw: %s)", into, err, raw)
	}
}

// ---------------------------------------------------------------------
// The default format, and the flag that opts out of it
// ---------------------------------------------------------------------

// TestFormatDefaultsToJSON is the headline decision, asserted at the surface:
// with no --format at all, stdout is one envelope, not prose. The agent is the
// operator of this CLI, so it does not have to opt in to the machine format.
func TestFormatDefaultsToJSON(t *testing.T) {
	root := newRootCmd()
	var out, errBuf strings.Builder
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs([]string{"version"})
	if err := runCLI(root); err != nil {
		t.Fatalf("version: %v (stderr: %s)", err, errBuf.String())
	}

	var env cliout.Envelope
	if err := json.Unmarshal([]byte(out.String()), &env); err != nil {
		t.Fatalf("with no --format, stdout must be one JSON envelope; got %q", out.String())
	}
	if !env.OK || env.Command != "version" {
		t.Fatalf("envelope drift: %+v", env)
	}
}

func TestFormatTextReproducesTheProse(t *testing.T) {
	out, _, err := execCLI(t, "version")
	if err != nil {
		t.Fatalf("version --format text: %v", err)
	}
	if !strings.HasPrefix(out, "dossierx ") || !strings.Contains(out, "commit:") {
		t.Fatalf("--format text must still print the human version block, got %q", out)
	}
	if strings.Contains(out, `"ok"`) {
		t.Fatalf("--format text must not emit an envelope, got %q", out)
	}
}

func TestUnknownFormatIsRefusedWithACode(t *testing.T) {
	root := newRootCmd()
	var out, errBuf strings.Builder
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs([]string{"--format", "yaml", "version"})
	err := runCLI(root)
	if err == nil {
		t.Fatal("an unrecognized --format must fail at the front door, not fall through to a default")
	}
	var env cliout.Envelope
	if decodeErr := json.Unmarshal([]byte(out.String()), &env); decodeErr != nil {
		t.Fatalf("the refusal must itself be an envelope; got %q", out.String())
	}
	if env.OK || env.Error == nil || env.Error.Code != cliout.CodeUnsupportedFormat {
		t.Fatalf("expected an unsupported_format failure envelope, got %+v", env)
	}
	if exitStatusFor(err) != 1 {
		t.Fatalf("a usage-class failure is exit 1, got %d", exitStatusFor(err))
	}
}

// TestEveryLeafButServeEmitsAnEnvelope is the inverse of Phase 1's
// text-only-opt-out fixture, and it is the stronger statement now that the ten
// commands that carried the opt-out are gone: with the surface at nineteen
// leaves, EVERY leaf except "serve" must answer --format json with exactly one
// envelope. A leaf that quietly printed prose instead would be a hole in the
// contract that no per-command test would notice.
//
// "serve" is exempt by construction (it blocks forever) and is asserted by
// TestServeIsTheOnlyTextOnlyCommand instead.
func TestEveryLeafButServeEmitsAnEnvelope(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")

	// One invocation per leaf, chosen to SUCCEED against the fixture project so
	// what is being asserted is the envelope, not an error envelope.
	for _, args := range [][]string{
		{"--config", cfgPath, "check", "--validate"},
		{"--config", cfgPath, "claim", "show", "widget.contract.overview"},
		{"--config", cfgPath, "claim", "list"},
		{"--config", cfgPath, "claim", "list", "--review-pending"},
		{"--config", cfgPath, "claim", "list", "--migrated"},
		{"--config", cfgPath, "claim", "new", "widget.contract.fresh", "--body", "a new fact", "--governed-reason", "fixture"},
		{"--config", cfgPath, "claim", "lock", "widget.contract.overview", "--reason", "approved", "--dry-run"},
		{"--config", cfgPath, "claim", "unlock", "widget.contract.overview", "--reason", "approved", "--dry-run"},
		{"--config", cfgPath, "claim", "flag", "widget.contract.overview", "--dry-run"},
		{"--config", cfgPath, "claim", "reaudit", "widget.contract.overview", "--dry-run"},
		{"--config", cfgPath, "claim", "link", "--dry-run"},
		{"--config", cfgPath, "comment", "inbox"},
		{"--config", cfgPath, "comment", "list", "widget.contract.overview"},
		{"--config", cfgPath, "comment", "add", "widget.contract.overview", "--as", "agent", "--body", "a note"},
		{"--config", cfgPath, "comment", "reply", "widget.contract.overview", "c-000000", "--dry-run"},
		{"--config", cfgPath, "build-order", "propose", "--module", "widget", "--dry-run"},
		{"--config", cfgPath, "build-order", "status", "--module", "widget"},
		{"--config", cfgPath, "build-order", "lock", "--module", "widget", "--dry-run"},
		{"--config", cfgPath, "skills", "export", filepath.Join(root, "skills-out")},
		{"version"},
	} {
		cmd := newRootCmd()
		var out, errBuf strings.Builder
		cmd.SetOut(&out)
		cmd.SetErr(&errBuf)
		cmd.SetArgs(append([]string{"--format", formatJSON}, args...))
		if err := runCLI(cmd); err != nil {
			t.Fatalf("%v: %v (stderr %s)", args, err, errBuf.String())
		}
		var env cliout.Envelope
		if decodeErr := json.Unmarshal([]byte(out.String()), &env); decodeErr != nil {
			t.Fatalf("%v: stdout is not a single envelope (%v): %q", args, decodeErr, out.String())
		}
		if !env.OK {
			t.Fatalf("%v: expected a success envelope, got %+v", args, env)
		}
		if env.Command == "" {
			t.Fatalf("%v: the envelope must name the subcommand it answers", args)
		}
	}
}

// TestServeIsTheOnlyTextOnlyCommand pins the ONE permanent exemption from the
// envelope, so that "text-only" can never quietly become a place to park a
// command someone did not want to convert.
func TestServeIsTheOnlyTextOnlyCommand(t *testing.T) {
	var marked []string
	var walk func(cmd *cobra.Command)
	walk = func(cmd *cobra.Command) {
		if cmd.Annotations[annotationTextOnly] == "true" {
			marked = append(marked, cmd.Name())
		}
		for _, child := range cmd.Commands() {
			walk(child)
		}
	}
	walk(newRootCmd())

	if len(marked) != 1 || marked[0] != "serve" {
		t.Fatalf("serve must be the only text-only command, got %v", marked)
	}
}

// ---------------------------------------------------------------------
// Golden envelopes: the surviving commands
// ---------------------------------------------------------------------

func TestEnvelope_Version(t *testing.T) {
	env, _, err := execCLIJSON(t, "version")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if !env.OK || env.Command != "version" {
		t.Fatalf("envelope drift: %+v", env)
	}
	var data versionData
	envData(t, env, &data)
	// The binary NAMES itself: a version envelope that does not say what is at
	// that version is useless to an agent inspecting a toolchain it did not
	// install.
	if data.Name != "dossierx" || data.Version == "" || data.Commit == "" || data.Date == "" {
		t.Fatalf("version payload must be fully populated, got %+v", data)
	}
}

func TestEnvelope_CheckSuccess(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")

	env, _, err := execCLIJSON(t, "--config", cfgPath, "check")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !env.OK || env.Command != "check" {
		t.Fatalf("envelope drift: %+v", env)
	}
	if env.StoppedAt != "" {
		t.Fatalf("a successful pipeline must not report stopped_at, got %q", env.StoppedAt)
	}
	// The single fixture claim is an orphan: a warning, not a failure. It has to
	// reach warnings[], because a successful run's warnings are exactly the
	// ones a caller reading only "ok: true" would otherwise miss.
	if len(env.Warnings) != 1 || !strings.Contains(env.Warnings[0], "orphan") {
		t.Fatalf("expected the orphan warning in warnings[], got %v", env.Warnings)
	}

	var data checkData
	envData(t, env, &data)
	if data.LintErrorCount != 0 || data.LintWarningCount != 1 {
		t.Fatalf("lint partition drift: %+v", data)
	}
	if data.CatalogPath == "" || data.ViewerPath == "" {
		t.Fatalf("a completed run must report both write paths, got %+v", data)
	}
	if len(data.NextSteps) == 0 {
		t.Fatalf("a project with a draft claim must carry a next step, got %+v", data)
	}
}

// TestEnvelope_CheckFailsAtLint is the stopped_at fixture, and the reason the
// field exists: "ok: false" alone cannot tell an agent whether the viewer on
// disk was rewritten or never touched.
func TestEnvelope_CheckFailsAtLint(t *testing.T) {
	root := t.TempDir()
	cfgPath := writeCheckFixture(t, root, parityConfig, map[string]string{
		"claims/broken.yaml": "id: widget.contract.broken\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"body: |\n  broken fixture.\n" +
			"rests_on:\n  - widget.contract.does-not-exist\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
	})

	env, _, err := execCLIJSON(t, "--config", cfgPath, "check")
	if err == nil {
		t.Fatal("a dangling rests_on must fail check")
	}
	if env.OK {
		t.Fatalf("envelope must report failure: %+v", env)
	}
	if env.StoppedAt != "lint" {
		t.Fatalf("stopped_at must name the failing step, got %q", env.StoppedAt)
	}
	if env.Error == nil || env.Error.Code != cliout.CodeLintFailed {
		t.Fatalf("expected a lint_failed error, got %+v", env.Error)
	}
	if env.Error.Message != "check: lint: 1 error-level finding(s)" {
		t.Fatalf("the envelope message must be the same string the terminal prints, got %q", env.Error.Message)
	}
	// tests/check_exit_test.go pins this at the process level; assert it here
	// too so a code-table change is caught in the fast suite.
	if exitStatusFor(err) != 1 {
		t.Fatalf("a check failure is exit 1, never 2, got %d", exitStatusFor(err))
	}

	// The partial result survives onto the failed envelope: this is what saves
	// the agent a second full run.
	var data checkData
	envData(t, env, &data)
	if data.LintErrorCount != 1 || len(data.LintFindings) != 1 {
		t.Fatalf("the lint findings must ride along on the failure, got %+v", data)
	}
	if data.CatalogPath != "" || data.ViewerPath != "" {
		t.Fatalf("a run that stopped at lint wrote nothing; paths must be empty, got %+v", data)
	}
}

func TestEnvelope_LockUnlockRoundTrip(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")
	id := "widget.contract.overview"

	env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", id, "--reason", "reviewed on the call")
	if err != nil {
		t.Fatalf("lock: %v", err)
	}
	if !env.OK || env.Command != "claim lock" {
		t.Fatalf("envelope drift: %+v", env)
	}
	var locked lockData
	envData(t, env, &locked)
	if locked.ClaimID != id || locked.From != "draft" || locked.To != "locked" {
		t.Fatalf("transition drift: %+v", locked)
	}
	// The human's words come back out. Phase 3 writes them into the ledger; the
	// contract that carries them exists now.
	if locked.Reason != "reviewed on the call" {
		t.Fatalf("the approving words must be echoed, got %+v", locked)
	}
	if locked.LockedAt == "" {
		t.Fatalf("lock must report when it happened, got %+v", locked)
	}

	env, _, err = execCLIJSON(t, "--config", cfgPath, "claim", "unlock", id, "--reason", "needs a fix")
	if err != nil {
		t.Fatalf("unlock: %v", err)
	}
	var unlocked unlockData
	envData(t, env, &unlocked)
	if unlocked.From != "locked" || unlocked.To != "draft" || unlocked.Reason != "needs a fix" {
		t.Fatalf("unlock payload drift: %+v", unlocked)
	}
	if unlocked.FlagCleared {
		t.Fatalf("no flag was pending; flag_cleared must be false, got %+v", unlocked)
	}
}

func TestEnvelope_FlagThenReaudit(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")
	id := "widget.contract.overview"

	if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", id, "--reason", "approved"); err != nil {
		t.Fatalf("lock: %v", err)
	}

	env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "flag", id,
		"--claim-says", "it retries twice", "--now-does", "it retries five times", "--reason", "code changed")
	if err != nil {
		t.Fatalf("flag: %v", err)
	}
	var flagged flagData
	envData(t, env, &flagged)
	if flagged.ClaimSays != "it retries twice" || flagged.NowDoes != "it retries five times" || !flagged.ReviewPending {
		t.Fatalf("flag payload drift: %+v", flagged)
	}

	// Bare reaudit is a preview and stays one: applied must be false and the
	// claim must still be review_pending.
	env, _, err = execCLIJSON(t, "--config", cfgPath, "claim", "reaudit", id)
	if err != nil {
		t.Fatalf("reaudit preview: %v", err)
	}
	var preview reauditData
	envData(t, env, &preview)
	if preview.Applied {
		t.Fatal("a bare reaudit must never apply")
	}
	if preview.Trigger != "flag" {
		t.Fatalf("trigger must name why the claim is pending, got %q", preview.Trigger)
	}
	if preview.BodyDiffHTML == "" {
		t.Fatalf("the preview must carry the proposed body, got %+v", preview)
	}

	env, _, err = execCLIJSON(t, "--config", cfgPath, "claim", "reaudit", id, "--confirm", "--reason", "checked the diff")
	if err != nil {
		t.Fatalf("reaudit --confirm: %v", err)
	}
	var applied reauditData
	envData(t, env, &applied)
	if !applied.Applied || applied.ReviewPending {
		t.Fatalf("a confirmed reaudit must apply and clear review_pending, got %+v", applied)
	}
	if applied.Reason != "checked the diff" {
		t.Fatalf("the approving words must be echoed, got %+v", applied)
	}
}

func TestEnvelope_CommentAddReplyList(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")
	id := "widget.contract.overview"

	env, _, err := execCLIJSON(t, "--config", cfgPath, "comment", "add", id, "--as", "human", "--body", "is this still true?")
	if err != nil {
		t.Fatalf("comment add: %v", err)
	}
	if env.Command != "comment add" {
		t.Fatalf("command path must name the leaf, got %q", env.Command)
	}
	var added commentWriteData
	envData(t, env, &added)
	if added.ThreadID == "" || added.Actor != "human" {
		t.Fatalf("add payload drift: %+v", added)
	}

	env, _, err = execCLIJSON(t, "--config", cfgPath, "comment", "reply", id, added.ThreadID, "--as", "agent", "--body", "fixed, please confirm")
	if err != nil {
		t.Fatalf("comment reply: %v", err)
	}
	var replied commentWriteData
	envData(t, env, &replied)
	if replied.ReplyID == "" || replied.ThreadID != added.ThreadID {
		t.Fatalf("reply payload drift: %+v", replied)
	}

	env, _, err = execCLIJSON(t, "--config", cfgPath, "comment", "list", id)
	if err != nil {
		t.Fatalf("comment list: %v", err)
	}
	var listed commentListData
	envData(t, env, &listed)
	if listed.Count != 1 || len(listed.Threads) != 1 {
		t.Fatalf("list payload drift: %+v", listed)
	}
	if len(listed.Threads[0].Replies) != 1 {
		t.Fatalf("list must carry the whole tree, replies included, got %+v", listed.Threads[0])
	}
}

func TestEnvelope_BuildOrderStatusNotProposedIsASuccess(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")

	env, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "status", "--module", "widget")
	if err != nil {
		t.Fatalf("build-order status: %v", err)
	}
	// "Nothing here yet" is an answer, not a failure — a status command that
	// errored for want of a thing to report on would be unusable in exactly the
	// case it is most needed.
	if !env.OK {
		t.Fatalf("an unproposed module must be a successful status, got %+v", env)
	}
	var data buildOrderStatusData
	envData(t, env, &data)
	if data.Proposed || data.Module != "widget" {
		t.Fatalf("status payload drift: %+v", data)
	}
}

func TestEnvelope_SkillsExport(t *testing.T) {
	target := filepath.Join(t.TempDir(), "skills")
	env, _, err := execCLIJSON(t, "skills", "export", target)
	if err != nil {
		t.Fatalf("skills export: %v", err)
	}
	var data skillsExportData
	envData(t, env, &data)
	if data.Count == 0 || len(data.Written) != data.Count {
		t.Fatalf("export payload drift: %+v", data)
	}
	// The agent that just ran this is about to LOAD these files; the paths save
	// it a directory walk.
	for _, p := range data.Written {
		if _, statErr := os.Stat(p); statErr != nil {
			t.Fatalf("reported path %s does not exist: %v", p, statErr)
		}
	}
}

// ---------------------------------------------------------------------
// Error codes and their exit statuses
// ---------------------------------------------------------------------

// TestErrorCodes walks the failures a skill is most likely to branch on and
// pins code + exit status together, because the pair is the contract: the code
// says what happened, the status says which of the two documented families it
// belongs to.
func TestErrorCodes(t *testing.T) {
	project := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, project, "widget")

	// A locked claim with an open thread, for the lock gate.
	gated := t.TempDir()
	gatedCfg, _ := icWriteFixtureProject(t, gated, "widget")
	if _, _, err := execCLIJSON(t, "--config", gatedCfg, "comment", "add", "widget.contract.overview", "--as", "human", "--body", "hold on"); err != nil {
		t.Fatalf("seed comment: %v", err)
	}

	cases := []struct {
		name     string
		args     []string
		wantCode cliout.Code
		wantExit int
	}{
		{
			name:     "no config anywhere",
			args:     []string{"--config", filepath.Join(project, "nope.yaml"), "check"},
			wantCode: cliout.CodeConfigNotFound,
			wantExit: 2,
		},
		{
			name:     "unknown claim id",
			args:     []string{"--config", cfgPath, "claim", "lock", "widget.contract.ghost", "--reason", "x"},
			wantCode: cliout.CodeClaimNotFound,
			wantExit: 2,
		},
		{
			name:     "flag on a draft claim",
			args:     []string{"--config", cfgPath, "claim", "flag", "widget.contract.overview", "--claim-says", "a", "--now-does", "b", "--reason", "c"},
			wantCode: cliout.CodeNotLocked,
			wantExit: 2,
		},
		{
			name:     "reaudit on a claim that is not review_pending",
			args:     []string{"--config", cfgPath, "claim", "reaudit", "widget.contract.overview"},
			wantCode: cliout.CodeNotReviewPending,
			wantExit: 2,
		},
		{
			name:     "lock without the human's words",
			args:     []string{"--config", cfgPath, "claim", "lock", "widget.contract.overview"},
			wantCode: cliout.CodeMissingFlag,
			wantExit: 1,
		},
		{
			name:     "comment without an actor",
			args:     []string{"--config", cfgPath, "comment", "add", "widget.contract.overview", "--body", "hi"},
			wantCode: cliout.CodeMissingFlag,
			wantExit: 1,
		},
		{
			name:     "comment with a nonsense actor",
			args:     []string{"--config", cfgPath, "comment", "add", "widget.contract.overview", "--as", "robot", "--body", "hi"},
			wantCode: cliout.CodeInvalidActor,
			wantExit: 1,
		},
		{
			name:     "unknown module",
			args:     []string{"--config", cfgPath, "build-order", "status", "--module", "nope"},
			wantCode: cliout.CodeUnknownModule,
			wantExit: 1,
		},
		{
			name:     "lock refused by an open comment thread",
			args:     []string{"--config", gatedCfg, "claim", "lock", "widget.contract.overview", "--reason", "approved"},
			wantCode: cliout.CodeUnresolvedComments,
			wantExit: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env, _, err := execCLIJSON(t, tc.args...)
			if err == nil {
				t.Fatalf("expected a failure, got envelope %+v", env)
			}
			if env.OK || env.Error == nil {
				t.Fatalf("expected a failure envelope, got %+v", env)
			}
			if env.Error.Code != tc.wantCode {
				t.Fatalf("code drift: got %q, want %q (message: %s)", env.Error.Code, tc.wantCode, env.Error.Message)
			}
			if got := exitStatusFor(err); got != tc.wantExit {
				t.Fatalf("exit status drift: got %d, want %d", got, tc.wantExit)
			}
		})
	}
}

// TestLockGateCodes pins the agreement between lock.Lock's refusals and the
// read-only reimplementation --dry-run uses to classify them. The duplication
// is deliberate (see lockGate's doc comment); this is what keeps it honest
// until Phase 3 promotes the gates to sentinels.
func TestLockGateCodes(t *testing.T) {
	// Open comment thread => unresolved_comments.
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "comment", "add", "widget.contract.overview", "--as", "human", "--body", "wait"); err != nil {
		t.Fatalf("seed comment: %v", err)
	}
	env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.overview", "--reason", "approved")
	if err == nil {
		t.Fatal("an open thread must refuse the lock")
	}
	if env.Error.Code != cliout.CodeUnresolvedComments {
		t.Fatalf("open-thread refusal must be unresolved_comments, got %q", env.Error.Code)
	}
	// The blocker's specifics ride in details so a skill does not parse prose.
	details, ok := env.Error.Details.(map[string]any)
	if !ok {
		t.Fatalf("expected structured error.details, got %T", env.Error.Details)
	}
	threads, ok := details["open_threads"].([]any)
	if !ok || len(threads) != 1 {
		t.Fatalf("expected the open thread id in error.details, got %v", env.Error.Details)
	}

	// A dangling rests_on => lint_failed.
	broken := t.TempDir()
	brokenCfg := writeCheckFixture(t, broken, parityConfig, map[string]string{
		"claims/a.yaml": "id: widget.contract.a\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"body: |\n  a claim.\nrests_on:\n  - widget.contract.nope\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
	})
	env, _, err = execCLIJSON(t, "--config", brokenCfg, "claim", "lock", "widget.contract.a", "--reason", "approved")
	if err == nil {
		t.Fatal("a dangling dependency must refuse the lock")
	}
	if env.Error.Code != cliout.CodeLintFailed {
		t.Fatalf("lint refusal must be lint_failed, got %q", env.Error.Code)
	}
}

// ---------------------------------------------------------------------
// --dry-run
// ---------------------------------------------------------------------

// dryRunOf runs a command with --dry-run and returns the decoded report,
// asserting the two invariants every dry run shares: it succeeds, and it wrote
// nothing.
func dryRunOf(t *testing.T, args ...string) cliout.DryRun {
	t.Helper()
	env, _, err := execCLIJSON(t, append(args, "--dry-run")...)
	if err != nil {
		t.Fatalf("%v --dry-run: %v", args, err)
	}
	if !env.OK {
		t.Fatalf("a dry run answers a question and always succeeds; got %+v", env)
	}
	var dr cliout.DryRun
	envData(t, env, &dr)
	return dr
}

func TestDryRun_LockReportsGatesAndLeavesDiskAlone(t *testing.T) {
	root := t.TempDir()
	cfgPath, claimPath := icWriteFixtureProject(t, root, "widget")
	id := "widget.contract.overview"
	before, err := os.ReadFile(claimPath)
	if err != nil {
		t.Fatalf("read claim: %v", err)
	}

	dr := dryRunOf(t, "--config", cfgPath, "claim", "lock", id, "--reason", "approved")
	if dr.Blocked {
		t.Fatalf("a clean draft claim must not be blocked, got %+v", dr)
	}
	if dr.From != "draft" || dr.To != "locked" {
		t.Fatalf("transition drift: %+v", dr)
	}
	// Passing preconditions are the evidence the human is being asked to
	// approve on, so they must be present, not merely "no failures".
	names := map[string]bool{}
	for _, p := range dr.Preconditions {
		names[p.Name] = p.OK
	}
	for _, want := range []string{"claim_is_draft", "lint_clean", "no_open_comment_threads"} {
		if ok, present := names[want]; !present || !ok {
			t.Fatalf("expected a passing %q precondition, got %+v", want, dr.Preconditions)
		}
	}
	if len(dr.SideEffects) == 0 {
		t.Fatalf("the blast radius is the part a reviewer cannot infer; it must be listed: %+v", dr)
	}

	after, err := os.ReadFile(claimPath)
	if err != nil {
		t.Fatalf("re-read claim: %v", err)
	}
	if string(after) != string(before) {
		t.Fatal("--dry-run wrote to the claim file")
	}
	if _, statErr := os.Stat(storePathForTest(t, cfgPath)); !os.IsNotExist(statErr) {
		t.Fatalf("--dry-run created a lock store: %v", statErr)
	}
}

// storePathForTest resolves the lock store path a project WOULD use, so a dry
// run can be asserted not to have created it.
func storePathForTest(t *testing.T, cfgPath string) string {
	t.Helper()
	return filepath.Join(filepath.Dir(cfgPath), ".dossierx-lock-store.json")
}

// TestDryRun_MissingReasonIsReportedNotRefused pins the ordering the approval
// loop needs: the agent previews FIRST, shows the human, and only then has
// words to put in --reason. A preview that failed for want of the approval it
// exists to solicit would be backwards.
func TestDryRun_MissingReasonIsReportedNotRefused(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")

	dr := dryRunOf(t, "--config", cfgPath, "claim", "lock", "widget.contract.overview")
	if !dr.Blocked {
		t.Fatalf("a lock with no --reason would be refused, so the dry run must say blocked: %+v", dr)
	}
	found := false
	for _, m := range dr.Missing {
		if m == "--reason" {
			found = true
		}
	}
	if !found {
		t.Fatalf("--reason must appear in missing[], got %v", dr.Missing)
	}
	// And the real run does refuse it — the preview is not lying.
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.overview"); err == nil {
		t.Fatal("a lock with no --reason must be refused for real")
	}
}

func TestDryRun_LockBlockedByAnOpenThread(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "comment", "add", "widget.contract.overview", "--as", "human", "--body", "hold on"); err != nil {
		t.Fatalf("seed comment: %v", err)
	}

	dr := dryRunOf(t, "--config", cfgPath, "claim", "lock", "widget.contract.overview", "--reason", "approved")
	if !dr.Blocked {
		t.Fatalf("an open thread blocks the lock; the preview must say so: %+v", dr)
	}
	for _, p := range dr.Preconditions {
		if p.Name == "no_open_comment_threads" && p.OK {
			t.Fatalf("the open-thread gate must be reported as failing: %+v", dr.Preconditions)
		}
	}
}

// TestDryRun_WinsOverConfirm is the collision rule, asserted rather than merely
// documented: --dry-run never writes, even when --confirm asks it to.
func TestDryRun_WinsOverConfirm(t *testing.T) {
	root := t.TempDir()
	cfgPath, claimPath := icWriteFixtureProject(t, root, "widget")
	id := "widget.contract.overview"

	if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", id, "--reason", "approved"); err != nil {
		t.Fatalf("lock: %v", err)
	}
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "flag", id,
		"--claim-says", "old", "--now-does", "new", "--reason", "changed"); err != nil {
		t.Fatalf("flag: %v", err)
	}
	before, err := os.ReadFile(claimPath)
	if err != nil {
		t.Fatalf("read claim: %v", err)
	}

	env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "reaudit", id, "--confirm", "--reason", "approved", "--dry-run")
	if err != nil {
		t.Fatalf("reaudit --dry-run --confirm: %v", err)
	}
	var dr cliout.DryRun
	envData(t, env, &dr)
	if dr.Would == "" || len(dr.SideEffects) == 0 {
		t.Fatalf("the preview must still describe the apply, got %+v", dr)
	}

	after, err := os.ReadFile(claimPath)
	if err != nil {
		t.Fatalf("re-read claim: %v", err)
	}
	if string(after) != string(before) {
		t.Fatal("--dry-run --confirm APPLIED the reaudit; --dry-run must always win")
	}
}

func TestDryRun_CommentAddWarnsThatItFlipsALockedClaim(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")
	id := "widget.contract.overview"
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", id, "--reason", "approved"); err != nil {
		t.Fatalf("lock: %v", err)
	}

	dr := dryRunOf(t, "--config", cfgPath, "comment", "add", id, "--as", "human", "--body", "is this true?")
	joined := strings.Join(dr.SideEffects, "\n")
	if !strings.Contains(joined, "review_pending") {
		t.Fatalf("opening a thread on a LOCKED claim flips it to review_pending; the preview must say so: %v", dr.SideEffects)
	}
}

// buildOrderFixture writes a project whose single claim is locked and carries a
// build_role, which is the minimum a build order can be proposed from.
func buildOrderFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	cfgPath := writeCheckFixture(t, root, parityConfig, map[string]string{
		"claims/a.yaml": "id: widget.contract.a\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\n" +
			"build_role: schema\n" +
			"body: |\n  a locked claim with a build role.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
	})
	return cfgPath
}

func TestEnvelope_BuildOrderProposeThenLock(t *testing.T) {
	cfgPath := buildOrderFixture(t)

	env, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "propose", "--module", "widget")
	if err != nil {
		t.Fatalf("build-order propose: %v", err)
	}
	var proposed buildOrderProposeData
	envData(t, env, &proposed)
	if proposed.Locked || proposed.Path == "" {
		t.Fatalf("propose payload drift: %+v", proposed)
	}
	// The per-phase claim IDS, not just counts: the whole reason to ask for a
	// build order is to know what to implement next, and a count cannot say.
	found := false
	for _, p := range proposed.Phases {
		for _, id := range p.Claims {
			if id == "widget.contract.a" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("the ordered claim ids must be in the envelope, got %+v", proposed.Phases)
	}

	env, _, err = execCLIJSON(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "reviewed the order")
	if err != nil {
		t.Fatalf("build-order lock: %v", err)
	}
	var locked buildOrderLockData
	envData(t, env, &locked)
	if locked.LockedAt == "" || locked.Reason != "reviewed the order" {
		t.Fatalf("build-order lock payload drift: %+v", locked)
	}
}

// TestBuildOrderLockCodes pins buildOrderLockCode's classification of the THREE
// refusals buildorder.Lock can produce. ErrNotProposed and ErrStale are
// sentinels; the already-locked refusal is still matched on a fragment of its
// own prose, so this is the guard that catches that message being reworded.
//
// The stale case is the one that shipped wrong. cliout.CodeBuildOrderStale was
// declared, documented in the build-order skill as the ONE route to
// "re-propose, then re-lock", and never emitted by anything: the stale refusal
// fell through to build_order_refused, whose three documented recoveries ("lock
// every claim in the module", "give each claim a build_role", "resolve the open
// threads") the stale artifact has already satisfied. An agent branching on
// code, exactly as the router skill instructs, got a dead branch and a set of
// recoveries that could not apply.
func TestBuildOrderLockCodes(t *testing.T) {
	cfgPath := buildOrderFixture(t)

	env, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "approved")
	if err == nil {
		t.Fatal("locking an unproposed build order must fail")
	}
	if env.Error.Code != cliout.CodeNotProposed {
		t.Fatalf("expected not_proposed, got %q (%s)", env.Error.Code, env.Error.Message)
	}

	if _, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "propose", "--module", "widget"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "approved"); err != nil {
		t.Fatalf("first lock: %v", err)
	}
	env, _, err = execCLIJSON(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "approved again")
	if err == nil {
		t.Fatal("re-locking a current build order must fail rather than silently succeed")
	}
	if env.Error.Code != cliout.CodeAlreadyLocked {
		t.Fatalf("expected already_locked, got %q (%s)", env.Error.Code, env.Error.Message)
	}

	// Move a covered claim underneath the frozen order, which is what makes it
	// stale: unlock, edit the body (changing its content hash), lock again.
	for _, args := range [][]string{
		{"claim", "unlock", "widget.contract.a", "--reason", "reopening to fix the wording"},
	} {
		if _, _, err := execCLIJSON(t, append([]string{"--config", cfgPath}, args...)...); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	claimFile := filepath.Join(filepath.Dir(cfgPath), "claims", "a.yaml")
	raw, err := os.ReadFile(claimFile)
	if err != nil {
		t.Fatalf("read claim: %v", err)
	}
	edited := strings.Replace(string(raw), "a locked claim with a build role.", "a rewritten claim with a build role.", 1)
	if edited == string(raw) {
		t.Fatalf("fixture precondition: the body substitution did not apply")
	}
	if err := os.WriteFile(claimFile, []byte(edited), 0o644); err != nil {
		t.Fatalf("edit claim: %v", err)
	}
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.a", "--reason", "re-approved"); err != nil {
		t.Fatalf("re-lock the edited claim: %v", err)
	}

	env, _, err = execCLIJSON(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "relock the stale order")
	if err == nil {
		t.Fatal("locking a stale build order must fail rather than freezing an outdated order")
	}
	if env.Error.Code != cliout.CodeBuildOrderStale {
		t.Fatalf("expected build_order_stale, got %q (%s)", env.Error.Code, env.Error.Message)
	}
}

func TestDryRun_BuildOrderLockNeedsAProposalAndAReason(t *testing.T) {
	cfgPath := buildOrderFixture(t)

	dr := dryRunOf(t, "--config", cfgPath, "build-order", "lock", "--module", "widget")
	if !dr.Blocked {
		t.Fatalf("no proposal and no --reason: the preview must say blocked, got %+v", dr)
	}
	missing := strings.Join(dr.Missing, ",")
	if !strings.Contains(missing, "--reason") || !strings.Contains(missing, "build_order_proposed") {
		t.Fatalf("expected both the absent input and the failed gate in missing[], got %v", dr.Missing)
	}

	if _, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "propose", "--module", "widget"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	dr = dryRunOf(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "approved")
	if dr.Blocked {
		t.Fatalf("a fresh proposal with a reason must not be blocked, got %+v", dr)
	}
}

func TestDryRun_BuildOrderProposeWritesNothing(t *testing.T) {
	cfgPath := buildOrderFixture(t)
	artifactDir := filepath.Dir(cfgPath)

	dr := dryRunOf(t, "--config", cfgPath, "build-order", "propose", "--module", "widget")
	if dr.Blocked {
		t.Fatalf("a fully locked module is orderable; got %+v", dr)
	}
	entries, err := os.ReadDir(artifactDir)
	if err != nil {
		t.Fatalf("read project dir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), "build-order") {
			t.Fatalf("--dry-run wrote a build-order artifact: %s", e.Name())
		}
	}
}

func TestDryRun_ImplinkSetChecksTheClaimIsLocked(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")

	// The fixture claim is DRAFT: an implementation link asserts "this code
	// implements a reviewed fact", and a draft claim is not yet a fact.
	dr := dryRunOf(t, "--config", cfgPath, "claim", "link",
		"--module", "widget", "--claim", "widget.contract.overview", "--file", "src/impl.go")
	if !dr.Blocked {
		t.Fatalf("linking to a draft claim must be reported as blocked, got %+v", dr)
	}
	for _, p := range dr.Preconditions {
		if p.Name == "claim_is_locked" && p.OK {
			t.Fatalf("claim_is_locked must fail for a draft claim: %+v", dr.Preconditions)
		}
	}
}

// TestUsageErrorsAreEnvelopesToo covers the failures that happen BEFORE any
// RunE runs — an unknown command, a missing positional argument. Cobra reports
// those itself, which is why runCLI takes error printing over entirely; without
// that, an agent's mis-invocation would come back as prose in the middle of a
// JSON conversation.
func TestUsageErrorsAreEnvelopesToo(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"unknown command", []string{"bogus"}},
		{"missing positional argument", []string{"--config", cfgPath, "claim", "lock"}},
		{"unknown flag", []string{"--config", cfgPath, "check", "--nope"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newRootCmd()
			var out, errBuf strings.Builder
			cmd.SetOut(&out)
			cmd.SetErr(&errBuf)
			cmd.SetArgs(append([]string{"--format", formatJSON}, tc.args...))
			err := runCLI(cmd)
			if err == nil {
				t.Fatalf("%v must fail", tc.args)
			}
			var env cliout.Envelope
			if decodeErr := json.Unmarshal([]byte(out.String()), &env); decodeErr != nil {
				t.Fatalf("a usage failure must still be one envelope on stdout; got %q", out.String())
			}
			if env.OK || env.Error == nil || env.Error.Code != cliout.CodeUsage {
				t.Fatalf("expected a usage failure envelope, got %+v", env)
			}
			if exitStatusFor(err) != 1 {
				t.Fatalf("a usage failure is exit 1, got %d", exitStatusFor(err))
			}
		})
	}
}

// TestTextModeErrorLineMatchesCobra pins the byte shape runCLI now owns. The
// golden fixtures in check_parity_test.go assert this line as part of check's
// stderr, so it is not decorative: "Error: " + the message, and nothing else.
func TestTextModeErrorLineMatchesCobra(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")

	_, stderr, err := execCLI(t, "--config", cfgPath, "claim", "lock", "widget.contract.ghost", "--reason", "x")
	if err == nil {
		t.Fatal("locking a nonexistent claim must fail")
	}
	want := "Error: " + err.Error() + "\n"
	if stderr != want {
		t.Fatalf("text-mode error line drift:\n got: %q\nwant: %q", stderr, want)
	}
}

// TestEnvelopeKeysAreSnakeCase walks the payloads a skill reads most and
// asserts every key is snake_case. Two engine types (lint.Finding,
// implink.ScanError) carry no JSON tags at all and would otherwise leak Go
// field names into the contract; cmd/dossierx projects them, and this is the
// guard that catches a new field added straight through.
func TestEnvelopeKeysAreSnakeCase(t *testing.T) {
	root := t.TempDir()
	cfgPath := writeCheckFixture(t, root, parityConfig, map[string]string{
		"claims/broken.yaml": "id: widget.contract.broken\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"body: |\n  broken fixture.\n" +
			"rests_on:\n  - widget.contract.does-not-exist\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
	})

	env, _, err := execCLIJSON(t, "--config", cfgPath, "check")
	if err == nil {
		t.Fatal("expected the fixture to fail lint")
	}

	var walk func(prefix string, v any)
	walk = func(prefix string, v any) {
		switch node := v.(type) {
		case map[string]any:
			for k, child := range node {
				if k != strings.ToLower(k) {
					t.Fatalf("envelope key %s%s is not snake_case", prefix, k)
				}
				walk(prefix+k+".", child)
			}
		case []any:
			for _, child := range node {
				walk(prefix, child)
			}
		}
	}
	walk("data.", env.Data)
}

// TestDryRun_ReauditOnAWrongStateClaimIsBlockedNotBroken pins the rule that
// separates the two ways a dry run can end. internal/reaudit refuses to propose
// for a claim that is not locked+review_pending; surfacing that as a command
// error would make "the answer is no" indistinguishable from "the preview
// itself broke", which is exactly the ambiguity --dry-run exists to remove.
func TestDryRun_ReauditOnAWrongStateClaimIsBlockedNotBroken(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")

	dr := dryRunOf(t, "--config", cfgPath, "claim", "reaudit", "widget.contract.overview", "--reason", "approved")
	if !dr.Blocked {
		t.Fatalf("reaudit on a draft claim must report blocked, got %+v", dr)
	}
	for _, want := range []string{"claim_is_locked", "claim_is_review_pending", "has_a_content_trigger"} {
		found := false
		for _, p := range dr.Preconditions {
			if p.Name == want && !p.OK {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected a failing %q precondition, got %+v", want, dr.Preconditions)
		}
	}
	// No proposal is computed for a claim that cannot be reaudited, so the
	// preview must not pretend to have a body to write.
	if _, present := dr.Proposed["body"]; present {
		t.Fatalf("a blocked reaudit preview must not propose a body, got %+v", dr.Proposed)
	}

	// A claim whose ONLY pending trigger is discussion is the same shape: the
	// remedy is a human clicking Resolve, not a diff to confirm.
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.overview", "--reason", "approved"); err != nil {
		t.Fatalf("lock: %v", err)
	}
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "comment", "add", "widget.contract.overview", "--as", "human", "--body", "wait"); err != nil {
		t.Fatalf("comment add: %v", err)
	}
	dr = dryRunOf(t, "--config", cfgPath, "claim", "reaudit", "widget.contract.overview", "--reason", "approved")
	if !dr.Blocked {
		t.Fatalf("a comment-only trigger leaves reaudit nothing to do; got %+v", dr)
	}
}
