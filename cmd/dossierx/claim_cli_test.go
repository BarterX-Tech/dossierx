// claim_cli_test.go is the fixture suite for the three leaves v0.3.0 ADDED —
// claim show, claim list, claim new — plus check --validate and comment inbox.
//
// The five are tested together because they exist for one reason: the
// restructure removed six verbs and could only do that if what replaced them
// answered strictly more. So the assertions here are mostly of the form "the
// thing the deleted verb reported is still reported, AND here is the thing it
// never could".
//
// It follows envelope_cli_test.go's shape (execCLIJSON for the contract,
// execCLI for the prose) rather than re-inventing a harness.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/comments"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// firstFieldWithPrefix pulls the minted "c-"/"r-" id out of a comment verb's
// prose echo line.
func firstFieldWithPrefix(t *testing.T, out, prefix string) string {
	t.Helper()
	for _, f := range strings.Fields(out) {
		if strings.HasPrefix(f, prefix) {
			return f
		}
	}
	t.Fatalf("no %q token in output: %s", prefix, out)
	return ""
}

// resolveThreadInProcess performs the human's Resolve.
//
// There is no CLI verb for it — v0.3.0 moved resolve to the viewer, where the
// rights holder is — but resolving is a PRECONDITION for several things this
// file tests (a claim cannot lock while a thread is open), so it goes through
// the same internal/comments call serve's handler makes.
func resolveThreadInProcess(t *testing.T, cfgPath, claimID, threadID string) {
	t.Helper()
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	deps := &comments.Deps{
		Cfg:           cfg,
		LockStorePath: storePath(cfg),
		FlagStorePath: flagStorePath(cfg),
	}
	if _, err := deps.Resolve(claimID, threadID, model.CommentRoleHuman); err != nil {
		t.Fatalf("resolve %s on %s: %v", threadID, claimID, err)
	}
}

// claimWriteFixture writes a project with two claims wired by a rests_on edge,
// so both edge DIRECTIONS have something to report — the half "deps" reported
// and the half an agent could not otherwise see.
func claimWriteFixture(t *testing.T, root string) string {
	t.Helper()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims: %v", err)
	}
	cfgPath := filepath.Join(root, "project.config.yaml")
	cfg := "schema_version: 1\nfacets:\n  - contract\n  - doctrine\nmodules:\n  - widget\nclaims_dir: claims\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	base := "id: widget.contract.retry-policy\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
		"body: |\n  requests retry three times with backoff.\n" +
		"migrated_from: docs/tabs/widget.html\n" +
		"governed_by:\n  type: none\n  reason: fixture claim\n"
	if err := os.WriteFile(filepath.Join(claimsDir, "retry.yaml"), []byte(base), 0o644); err != nil {
		t.Fatalf("write base claim: %v", err)
	}

	dependent := "id: widget.contract.timeout-budget\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
		"body: |\n  the total time budget across retries.\n" +
		"rests_on:\n  - widget.contract.retry-policy\n" +
		"governed_by:\n  type: none\n  reason: fixture claim\n"
	if err := os.WriteFile(filepath.Join(claimsDir, "timeout.yaml"), []byte(dependent), 0o644); err != nil {
		t.Fatalf("write dependent claim: %v", err)
	}
	return cfgPath
}

// ---------------------------------------------------------------------
// claim show
// ---------------------------------------------------------------------

func TestEnvelope_ClaimShowAnswersEverythingDepsAndImplinkStatusDid(t *testing.T) {
	root := t.TempDir()
	cfgPath := claimWriteFixture(t, root)

	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "show", "widget.contract.retry-policy")
	if err != nil {
		t.Fatalf("claim show: %v", err)
	}
	if !env.OK || env.Command != "claim show" {
		t.Fatalf("envelope drift: %+v", env)
	}
	var data claimShowData
	envData(t, env, &data)

	if data.ClaimID != "widget.contract.retry-policy" || data.Title != "Retry Policy" {
		t.Fatalf("identity drift: %+v", data)
	}
	if data.Status != "draft" || data.Locked || data.ReviewPending {
		t.Fatalf("lifecycle drift: %+v", data)
	}
	if data.MigratedFrom != "docs/tabs/widget.html" {
		t.Fatalf("expected the migrated_from note carried, got %+v", data)
	}
	// BOTH edge directions in one call. The incoming half is the whole reason
	// this replaced "deps" rather than being a rename of it.
	if len(data.Edges.DependedOnBy) != 1 || data.Edges.DependedOnBy[0] != "widget.contract.timeout-budget" {
		t.Fatalf("expected the incoming rests_on edge, got %+v", data.Edges)
	}
	if len(data.Edges.RestsOn) != 0 || len(data.Edges.Mirrors) != 0 {
		t.Fatalf("this claim has no outgoing edges, got %+v", data.Edges)
	}
	if data.Edges.GovernedBy != "none" || data.Edges.GovernedReason == "" {
		t.Fatalf("expected governed_by reported in both parts, got %+v", data.Edges)
	}
	// Empty lists must be arrays, never null: a caller ranges over them.
	if data.ImplementedIn == nil {
		t.Fatalf("implemented_in must be an array even when empty")
	}
	if data.Comments.OpenThreadIDs == nil {
		t.Fatalf("open_thread_ids must be an array even when empty")
	}
	// next_actions: nothing the retired verbs had.
	if len(data.NextActions) == 0 {
		t.Fatalf("claim show must report the legal next actions, got %+v", data)
	}
	if !strings.Contains(strings.Join(data.NextActions, " "), "dossierx claim lock") {
		t.Fatalf("a lint-clean draft's next action is to lock it, got %v", data.NextActions)
	}
}

// TestClaimShowNextActionsFollowTheLifecycle walks one claim through the states
// the skills teach and pins that the advice changes with them — which is the
// only thing that makes next_actions worth carrying at all.
func TestClaimShowNextActionsFollowTheLifecycle(t *testing.T) {
	root := t.TempDir()
	cfgPath := claimWriteFixture(t, root)
	const id = "widget.contract.retry-policy"

	nextActions := func() string {
		t.Helper()
		env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "show", id)
		if err != nil {
			t.Fatalf("claim show: %v", err)
		}
		var data claimShowData
		envData(t, env, &data)
		return strings.Join(data.NextActions, "\n")
	}

	// 1. Draft and clean -> lock it (with the human's words).
	if got := nextActions(); !strings.Contains(got, "ready to lock") {
		t.Fatalf("draft: expected a lock suggestion, got:\n%s", got)
	}

	// 2. An open thread on the draft -> the lock gate now names the blocker,
	// and points at the human, since only they can resolve it.
	addOut, _, err := execReviewedCLI(t, "--config", cfgPath, "comment", "add", id, "--as", "human", "--body", "is three the right number?")
	if err != nil {
		t.Fatalf("comment add: %v (out: %s)", err, addOut)
	}
	if got := nextActions(); !strings.Contains(got, "open comment thread") || !strings.Contains(got, "human") {
		t.Fatalf("open thread: expected the comment gate named, got:\n%s", got)
	}

	// 3. Locked and settled -> the ONLY sanctioned way to change it.
	tid := strings.TrimRight(firstFieldWithPrefix(t, addOut, "c-"), ";,")
	resolveThreadInProcess(t, cfgPath, id, tid)
	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", id, "--reason", "approved"); err != nil {
		t.Fatalf("lock: %v", err)
	}
	got := nextActions()
	if !strings.Contains(got, "unlock") || !strings.Contains(got, "relock") {
		t.Fatalf("locked: expected the unlock -> fix -> lock path, got:\n%s", got)
	}
	if !strings.Contains(got, "no implementation link yet") {
		t.Fatalf("locked: expected the code-link gap reported, got:\n%s", got)
	}
}

func TestClaimShowUnknownIDIsTheNotFoundFamily(t *testing.T) {
	root := t.TempDir()
	cfgPath := claimWriteFixture(t, root)

	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "show", "widget.contract.ghost")
	if err == nil {
		t.Fatal("expected claim show on an unknown id to fail")
	}
	if env.OK || env.Error == nil || env.Error.Code != cliout.CodeClaimNotFound {
		t.Fatalf("expected a claim_not_found envelope, got %+v", env)
	}
	if exitStatusFor(err) != 2 {
		t.Fatalf("an id that does not exist is exit 2, got %d", exitStatusFor(err))
	}
}

// ---------------------------------------------------------------------
// claim list
// ---------------------------------------------------------------------

func TestEnvelope_ClaimListFilters(t *testing.T) {
	root := t.TempDir()
	cfgPath := claimWriteFixture(t, root)

	listData := func(args ...string) claimListData {
		t.Helper()
		env, _, err := execReviewedCLIJSON(t, append([]string{"--config", cfgPath, "claim", "list"}, args...)...)
		if err != nil {
			t.Fatalf("claim list %v: %v", args, err)
		}
		var data claimListData
		envData(t, env, &data)
		return data
	}

	all := listData()
	if all.Count != 2 || all.Total != 2 || all.PercentOfTotal != 100 {
		t.Fatalf("unfiltered list drift: %+v", all)
	}

	// --migrated: the retired "coverage" verb's ratio, plus the names.
	migrated := listData("--migrated")
	if migrated.Count != 1 || migrated.Total != 2 || migrated.PercentOfTotal != 50 {
		t.Fatalf("expected 1 of 2 (50%%) migrated, got %+v", migrated)
	}
	if migrated.Claims[0].ClaimID != "widget.contract.retry-policy" {
		t.Fatalf("expected the migrated claim named, got %+v", migrated.Claims)
	}

	// --review-pending: the retired "stale" verb.
	if pending := listData("--review-pending"); pending.Count != 0 {
		t.Fatalf("nothing is locked, so nothing is review_pending; got %+v", pending)
	}

	// --facet / --module.
	if byFacet := listData("--facet", "doctrine"); byFacet.Count != 0 {
		t.Fatalf("no claim is in the doctrine facet, got %+v", byFacet)
	}
	if byModule := listData("--module", "widget"); byModule.Count != 2 {
		t.Fatalf("both claims are in widget, got %+v", byModule)
	}

	// The echoed filters: an agent showing a human "here are the 3" has to be
	// able to say WHICH 3 without re-deriving it from its own call site.
	echoed := listData("--migrated", "--facet", "contract")
	if !echoed.Filters.Migrated || echoed.Filters.Facet != "contract" || echoed.Filters.ReviewPending {
		t.Fatalf("filters must be echoed exactly as applied, got %+v", echoed.Filters)
	}
}

func TestClaimListMatchResolvesWhatAHumanWouldSay(t *testing.T) {
	root := t.TempDir()
	cfgPath := claimWriteFixture(t, root)

	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "list", "--match", "the retry card")
	if err != nil {
		t.Fatalf("claim list --match: %v", err)
	}
	var data claimListData
	envData(t, env, &data)
	if data.Count == 0 {
		t.Fatalf("expected --match to resolve a human's phrasing, got %+v", data)
	}
	if data.Claims[0].ClaimID != "widget.contract.retry-policy" {
		t.Fatalf("expected the retry claim ranked first, got %+v", data.Claims)
	}
	// The score is exposed so an agent can tell a confident hit from a tie it
	// should hand back to the human.
	if data.Claims[0].Score <= 0 {
		t.Fatalf("a matched claim must carry its score, got %+v", data.Claims[0])
	}
	if len(data.Claims) > 1 && data.Claims[0].Score < data.Claims[1].Score {
		t.Fatalf("results must be ranked by score, got %+v", data.Claims)
	}

	// A query that matches nothing is an EMPTY list, not an error: "no card by
	// that name" is an answer.
	env, _, err = execReviewedCLIJSON(t, "--config", cfgPath, "claim", "list", "--match", "zzzzzz")
	if err != nil {
		t.Fatalf("a no-match query must still succeed: %v", err)
	}
	envData(t, env, &data)
	if data.Count != 0 {
		t.Fatalf("expected no matches, got %+v", data)
	}
}

// ---------------------------------------------------------------------
// claim new
// ---------------------------------------------------------------------

// TestClaimNewProducesALintCleanClaim is the command's whole promise: an agent
// that may not hand-write claim YAML must be able to author one that passes the
// suite on the very next validate.
func TestClaimNewProducesALintCleanClaim(t *testing.T) {
	root := t.TempDir()
	cfgPath := claimWriteFixture(t, root)

	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "new", "widget.contract.circuit-breaker",
		"--body", "opens the circuit after five consecutive failures.",
		"--governed-reason", "a fresh fact, not yet backed by doctrine",
		"--rests-on", "widget.contract.retry-policy")
	if err != nil {
		t.Fatalf("claim new: %v", err)
	}
	var data claimNewData
	envData(t, env, &data)
	if data.LintErrorCount != 0 {
		t.Fatalf("claim new must produce a claim that passes lint, got %+v", data)
	}
	if data.Path != filepath.Join(root, "claims", "widget.contract.circuit-breaker.yaml") {
		t.Fatalf("unexpected path: %+v", data)
	}
	if data.Facet != "contract" || data.Module != "widget" || data.Status != "draft" {
		t.Fatalf("facet/module must be derived from the id, and the claim is a DRAFT: %+v", data)
	}

	// Independently confirmed through the gate the agent would actually run.
	validate, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "check", "--validate")
	if err != nil {
		t.Fatalf("check --validate after claim new: %v", err)
	}
	var checked checkData
	envData(t, validate, &checked)
	if checked.LintErrorCount != 0 {
		t.Fatalf("the authored claim must validate clean, got %+v", checked.LintFindings)
	}

	// And it is a real claim: show finds it, with the edge it was given.
	show, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "show", "widget.contract.circuit-breaker")
	if err != nil {
		t.Fatalf("claim show on the new claim: %v", err)
	}
	var shown claimShowData
	envData(t, show, &shown)
	if len(shown.Edges.RestsOn) != 1 || shown.Edges.RestsOn[0] != "widget.contract.retry-policy" {
		t.Fatalf("expected the authored rests_on edge, got %+v", shown.Edges)
	}
}

func TestClaimNewRefusals(t *testing.T) {
	root := t.TempDir()
	cfgPath := claimWriteFixture(t, root)

	cases := []struct {
		name string
		args []string
		code cliout.Code
	}{
		{
			// The id grammar is enforced at the DOOR, not after the write: a
			// claim whose id the project cannot accept, once on disk, breaks
			// every other command until someone hand-deletes it.
			name: "id is not three segments",
			args: []string{"claim", "new", "nope", "--body", "x", "--governed-reason", "y"},
			code: cliout.CodeBadRequest,
		},
		{
			name: "slug is not kebab-case",
			args: []string{"claim", "new", "widget.contract.NotKebab", "--body", "x", "--governed-reason", "y"},
			code: cliout.CodeBadRequest,
		},
		{
			name: "module is not one the project declares",
			args: []string{"claim", "new", "gadget.contract.thing", "--body", "x", "--governed-reason", "y"},
			code: cliout.CodeUnknownModule,
		},
		{
			name: "facet is not one the project declares",
			args: []string{"claim", "new", "widget.nosuch.thing", "--body", "x", "--governed-reason", "y"},
			code: cliout.CodeBadRequest,
		},
		{
			name: "no body: a claim with no content states nothing",
			args: []string{"claim", "new", "widget.contract.empty", "--governed-reason", "y"},
			code: cliout.CodeMissingFlag,
		},
		{
			// The governed-required lint would reject this claim, so the
			// command refuses to author it rather than writing something it
			// knows will fail.
			name: "governed_by none with no reason",
			args: []string{"claim", "new", "widget.contract.ungoverned", "--body", "x"},
			code: cliout.CodeMissingFlag,
		},
		{
			name: "id already exists",
			args: []string{"claim", "new", "widget.contract.retry-policy", "--body", "x", "--governed-reason", "y"},
			code: cliout.CodeBadRequest,
		},
		{
			// A claim outside claims_dir is never loaded, so writing one there
			// would report a cheerful success for a file the project can never
			// see — worse than a refusal.
			name: "--file escapes claims_dir",
			args: []string{"claim", "new", "widget.contract.escapee", "--body", "x", "--governed-reason", "y", "--file", "../escapee.yaml"},
			code: cliout.CodeBadRequest,
		},
		{
			name: "--file is absolute",
			args: []string{"claim", "new", "widget.contract.absolute", "--body", "x", "--governed-reason", "y", "--file", filepath.Join(root, "elsewhere.yaml")},
			code: cliout.CodeBadRequest,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env, _, err := execReviewedCLIJSON(t, append([]string{"--config", cfgPath}, tc.args...)...)
			if err == nil {
				t.Fatalf("expected a refusal, got success: %+v", env)
			}
			if env.Error == nil || env.Error.Code != tc.code {
				t.Fatalf("code drift: got %+v, want %q", env.Error, tc.code)
			}
		})
	}

	// Nothing was written by any of them.
	entries, err := os.ReadDir(filepath.Join(root, "claims"))
	if err != nil {
		t.Fatalf("read claims dir: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("a refused claim new must write nothing; claims dir now holds %d files", len(entries))
	}
}

func TestClaimNewDryRunWritesNothing(t *testing.T) {
	root := t.TempDir()
	cfgPath := claimWriteFixture(t, root)

	// An incomplete invocation is REPORTED, not refused: the preview still
	// succeeds (exit 0, ok:true) and names what is missing, because previewing
	// before you have every field is the point of previewing.
	dr := dryRunOf(t, "--config", cfgPath, "claim", "new", "widget.contract.new-thing", "--body", "a fact")
	if !containsStr(dr.Missing, "--governed-reason") {
		t.Fatalf("expected --governed-reason in missing[], got %+v", dr.Missing)
	}
	if !dr.Blocked {
		t.Fatalf("a missing required input blocks the real run, and the preview must say so: %+v", dr)
	}

	// Complete, and now unblocked: the two preconditions that matter (the id
	// and the file are both free) hold.
	dr = dryRunOf(t, "--config", cfgPath, "claim", "new", "widget.contract.new-thing",
		"--body", "a fact", "--governed-reason", "fixture")
	if dr.Blocked {
		t.Fatalf("a complete invocation for a fresh id must not be blocked, got %+v", dr)
	}
	if dr.To != "draft" {
		t.Fatalf("claim new creates a DRAFT, got %+v", dr)
	}

	// Neither preview touched the disk.
	if _, err := os.Stat(filepath.Join(root, "claims", "widget.contract.new-thing.yaml")); err == nil {
		t.Fatal("a dry run must not create the claim file")
	}
}

// ---------------------------------------------------------------------
// comment inbox
// ---------------------------------------------------------------------

func TestEnvelope_CommentInboxFindsEveryOpenThreadInOneCall(t *testing.T) {
	root := t.TempDir()
	cfgPath := claimWriteFixture(t, root)

	// Two threads on two different claims: the case the inbox exists for,
	// where a per-claim "comment list" would cost one call per claim.
	humanOut, _, err := execReviewedCLI(t, "--config", cfgPath, "comment", "add", "widget.contract.retry-policy", "--as", "human", "--body", "is three the right number?")
	if err != nil {
		t.Fatalf("comment add (human): %v", err)
	}
	if _, _, err := execReviewedCLI(t, "--config", cfgPath, "comment", "add", "widget.contract.timeout-budget", "--as", "agent", "--body", "flagging my own uncertainty here"); err != nil {
		t.Fatalf("comment add (agent): %v", err)
	}

	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "comment", "inbox")
	if err != nil {
		t.Fatalf("comment inbox: %v", err)
	}
	if !env.OK || env.Command != "comment inbox" {
		t.Fatalf("envelope drift: %+v", env)
	}
	var data commentInboxData
	envData(t, env, &data)
	if data.Count != 2 || data.Claims != 2 {
		t.Fatalf("expected 2 open threads across 2 claims, got %+v", data)
	}
	if data.Cursor == "" {
		t.Fatalf("the inbox must echo a cursor to poll with, got %+v", data)
	}

	// agent_can_resolve is the field the whole design rests on: advisory rights
	// let an agent act only on agent-authored messages, so the human's thread
	// must report false and the agent's own must report true. An agent that
	// reads this never spends a call earning rights_denied.
	byClaim := map[string]inboxThread{}
	for _, th := range data.Threads {
		byClaim[th.ClaimID] = th
	}
	if th := byClaim["widget.contract.retry-policy"]; th.Author != "human" || th.AgentCanResolve {
		t.Fatalf("an agent may NOT resolve a human's thread: %+v", th)
	}
	if th := byClaim["widget.contract.timeout-budget"]; th.Author != "agent" || !th.AgentCanResolve {
		t.Fatalf("an agent may resolve its own thread: %+v", th)
	}
	// The claim's human label rides along so the agent can say "the Retry
	// Policy card" back to the human rather than reciting an id.
	if byClaim["widget.contract.retry-policy"].ClaimTitle != "Retry Policy" {
		t.Fatalf("expected the derived claim title, got %+v", byClaim["widget.contract.retry-policy"])
	}

	// A resolved thread leaves the inbox — it is an inbox, not an archive.
	tid := strings.TrimRight(firstFieldWithPrefix(t, humanOut, "c-"), ";,")
	resolveThreadInProcess(t, cfgPath, "widget.contract.retry-policy", tid)
	env, _, err = execReviewedCLIJSON(t, "--config", cfgPath, "comment", "inbox")
	if err != nil {
		t.Fatalf("comment inbox after resolve: %v", err)
	}
	envData(t, env, &data)
	if data.Count != 1 {
		t.Fatalf("a resolved thread must leave the inbox, got %+v", data)
	}
}

// TestCommentInboxSinceIsInclusiveAndNeverMissesAThread pins --since's
// boundary rule, which is the one place this command could silently lose the
// human's words.
//
// The timestamps are hand-authored in the fixture rather than minted by the
// engine, so the boundary case is deterministic: comment timestamps have
// one-second resolution, and a test that wrote two threads back to back could
// not tell "same second" from "different second" at all.
func TestCommentInboxSinceIsInclusiveAndNeverMissesAThread(t *testing.T) {
	root := t.TempDir()
	cfgPath := claimWriteFixture(t, root)

	const (
		oldAt   = "2026-01-01T00:00:00Z"
		newAt   = "2026-06-01T00:00:00Z"
		between = "2026-03-01T00:00:00Z"
	)
	threaded := "id: widget.contract.threaded\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
		"body: |\n  a claim under discussion.\n" +
		"governed_by:\n  type: none\n  reason: fixture claim\n" +
		"comments:\n" +
		"  - id: c-old001\n    status: open\n    author: human\n    created: \"" + oldAt + "\"\n    body: the older question\n    edited: false\n" +
		"  - id: c-new001\n    status: open\n    author: human\n    created: \"" + oldAt + "\"\n    body: the newer question\n    edited: false\n" +
		"    replies:\n      - id: r-000001\n        author: agent\n        created: \"" + newAt + "\"\n        body: looking into it\n        edited: false\n"
	if err := os.WriteFile(filepath.Join(root, "claims", "threaded.yaml"), []byte(threaded), 0o644); err != nil {
		t.Fatalf("write threaded claim: %v", err)
	}

	inbox := func(args ...string) commentInboxData {
		t.Helper()
		env, _, err := execReviewedCLIJSON(t, append([]string{"--config", cfgPath, "comment", "inbox"}, args...)...)
		if err != nil {
			t.Fatalf("comment inbox %v: %v", args, err)
		}
		var data commentInboxData
		envData(t, env, &data)
		return data
	}

	all := inbox()
	if all.Count != 2 {
		t.Fatalf("expected both open threads, got %+v", all)
	}
	// The cursor is the newest activity anywhere — a REPLY's timestamp here,
	// not the thread's own, which is the point of keying on latest activity.
	if all.Cursor != newAt {
		t.Fatalf("cursor must be the newest activity (%s), got %q", newAt, all.Cursor)
	}
	// Oldest first: an inbox is a queue to work through.
	if all.Threads[0].ThreadID != "c-old001" {
		t.Fatalf("expected oldest-activity-first ordering, got %+v", all.Threads)
	}
	if !all.Threads[1].AgentHasReplied || all.Threads[0].AgentHasReplied {
		t.Fatalf("agent_has_replied must reflect who actually replied, got %+v", all.Threads)
	}

	// A cursor between the two drops only the older one.
	mid := inbox("--since", between)
	if mid.Count != 1 || mid.Threads[0].ThreadID != "c-new001" {
		t.Fatalf("expected only the newer thread after %s, got %+v", between, mid)
	}
	if mid.Since != between {
		t.Fatalf("the inbox must echo the --since it was given, got %+v", mid)
	}

	// The boundary: polling with the previous cursor RE-REPORTS the thread at
	// that exact timestamp rather than dropping it. Missing the human's comment
	// would break the loop; seeing it twice costs the agent nothing.
	boundary := inbox("--since", all.Cursor)
	if boundary.Count != 1 || boundary.Threads[0].ThreadID != "c-new001" {
		t.Fatalf("--since must be INCLUSIVE of its own second, got %+v", boundary)
	}
	if boundary.Cursor != all.Cursor {
		t.Fatalf("the cursor must not regress across polls: %q -> %q", all.Cursor, boundary.Cursor)
	}

	// A cursor past everything is empty — and still echoes a usable cursor.
	future := inbox("--since", "2027-01-01T00:00:00Z")
	if future.Count != 0 {
		t.Fatalf("expected nothing after a future cursor, got %+v", future)
	}
	if future.Cursor != "2027-01-01T00:00:00Z" {
		t.Fatalf("with nothing newer, the cursor holds at the caller's own value, got %+v", future)
	}
}

// ---------------------------------------------------------------------
// check --validate
// ---------------------------------------------------------------------

// TestCheckValidateIsTrulyReadOnly is the assertion the flag exists for. Plain
// "check" writes on every run — claim files, the lock store, build/catalog/catalog.json and
// the viewer — so "read-only" has to be proved against the FILESYSTEM, not
// against the absence of a "wrote" line.
func TestCheckValidateIsTrulyReadOnly(t *testing.T) {
	root := t.TempDir()
	cfgPath := claimWriteFixture(t, root)

	// Set up a state a WRITING check would visibly change: two locked claims
	// with a drifted dependency, which plain check reconciles to
	// review_pending and persists back to the claim file.
	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.retry-policy", "--reason", "approved"); err != nil {
		t.Fatalf("lock base: %v", err)
	}
	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.timeout-budget", "--reason", "approved"); err != nil {
		t.Fatalf("lock dependent: %v", err)
	}
	basePath := filepath.Join(root, "claims", "retry.yaml")
	base, err := os.ReadFile(basePath)
	if err != nil {
		t.Fatalf("read base claim: %v", err)
	}
	drifted := strings.Replace(string(base), "three times", "five times", 1)
	if err := os.WriteFile(basePath, []byte(drifted), 0o644); err != nil {
		t.Fatalf("rewrite base claim: %v", err)
	}
	// The rewrite above stands in for the real path (unlock -> edit -> lock),
	// which records the new content as approved. Re-record it, so what is under
	// test stays "does --validate write anything" rather than "does the ledger
	// gate notice a hand-edited locked claim" — which it does, elsewhere.
	armLedgerFixture(t, cfgPath)

	before := snapshotTree(t, root)

	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "check", "--validate"); err != nil {
		t.Fatalf("check --validate: %v", err)
	}

	after := snapshotTree(t, root)
	for path, want := range before {
		got, ok := after[path]
		if !ok {
			t.Fatalf("check --validate deleted %s", path)
		}
		if got != want {
			t.Fatalf("check --validate modified %s; it must write nothing", path)
		}
	}
	for path := range after {
		if _, ok := before[path]; !ok {
			t.Fatalf("check --validate created %s; it must write nothing", path)
		}
	}

	// The writing form, on the same tree, DOES change it — which is what makes
	// the assertion above meaningful rather than vacuous.
	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "check"); err != nil {
		t.Fatalf("check: %v", err)
	}
	if snapshotTree(t, root)[filepath.Join(root, "claims", "timeout.yaml")] == before[filepath.Join(root, "claims", "timeout.yaml")] {
		t.Fatal("precondition broken: a writing check should have flipped the dependent to review_pending")
	}
}

// snapshotTree maps every regular file under root to its contents, so a whole
// project directory can be compared before and after a command.
func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		out[path] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	return out
}

// TestCheckValidateAndCheckAgreeOnTheLintVerdict pins that --validate is the
// SAME gate as check's lint step, not a laxer one. If the two could disagree,
// an agent could get a clean validate and then a failing check, and the
// authoring loop the flag exists to serve would be worthless.
func TestCheckValidateAndCheckAgreeOnTheLintVerdict(t *testing.T) {
	root := t.TempDir()
	cfgPath := claimWriteFixture(t, root)

	broken := "id: widget.contract.broken\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
		"body: |\n  broken.\n" +
		"rests_on:\n  - widget.contract.does-not-exist\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
	if err := os.WriteFile(filepath.Join(root, "claims", "broken.yaml"), []byte(broken), 0o644); err != nil {
		t.Fatalf("write broken claim: %v", err)
	}

	validateEnv, validateErr := envelopeOf(t, "--config", cfgPath, "check", "--validate")
	checkEnv, checkErr := envelopeOf(t, "--config", cfgPath, "check")

	if validateErr == nil || checkErr == nil {
		t.Fatal("both forms must fail on a dangling rests_on")
	}
	if validateEnv.Error.Code != checkEnv.Error.Code {
		t.Fatalf("the two forms must report the same code, got %q vs %q", validateEnv.Error.Code, checkEnv.Error.Code)
	}
	if validateEnv.StoppedAt != "lint" || checkEnv.StoppedAt != "lint" {
		t.Fatalf("both must stop at lint, got %q and %q", validateEnv.StoppedAt, checkEnv.StoppedAt)
	}
	if exitStatusFor(validateErr) != exitStatusFor(checkErr) {
		t.Fatalf("the two forms must exit alike, got %d vs %d", exitStatusFor(validateErr), exitStatusFor(checkErr))
	}

	var validated, checked checkData
	envData(t, validateEnv, &validated)
	envData(t, checkEnv, &checked)
	if validated.LintErrorCount != checked.LintErrorCount {
		t.Fatalf("lint verdicts diverged: %d vs %d", validated.LintErrorCount, checked.LintErrorCount)
	}
	if !validated.ReadOnly {
		t.Fatal("--validate must mark its payload read_only")
	}
	if checked.ReadOnly {
		t.Fatal("a writing check must NOT claim to be read-only")
	}
}

// envelopeOf runs a command and returns its envelope plus the run error,
// without failing the test on a non-nil error the way execCLIJSON's callers
// usually want. It exists so a test can compare two FAILING runs.
func envelopeOf(t *testing.T, args ...string) (cliout.Envelope, error) {
	t.Helper()
	root := newRootCmd()
	var outBuf, errBuf strings.Builder
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs(append([]string{"--format", formatJSON}, args...))
	err := runCLI(root)
	var env cliout.Envelope
	if decodeErr := json.Unmarshal([]byte(outBuf.String()), &env); decodeErr != nil {
		t.Fatalf("stdout is not a single envelope (%v): %s", decodeErr, outBuf.String())
	}
	return env, err
}

// ---------------------------------------------------------------------
// a lock refusal has to name the rule, not count it
// ---------------------------------------------------------------------

// buildRoleAdoptedFixture is a module that has ADOPTED build_role — one locked
// claim carries it — plus a draft claim that does not.
//
// That combination is the whole point. build-role-required-for-locked fires
// only against a LOCKED claim in an adopted module, so against the project as it
// stands (b still draft) it reports nothing at all: `check --validate` is
// ok:true with lint_error_count 0. The rule nonetheless refuses `claim lock b`,
// because the lock gate lints the about-to-be-locked form. Every claim-status
// lint has this shape — rest-on-locked and roll-up too — which is why "go run
// check --validate to see the finding" was never an answer.
func buildRoleAdoptedFixture(t *testing.T) string {
	t.Helper()
	return writeCheckFixture(t, t.TempDir(), parityConfig, map[string]string{
		"claims/a.yaml": "id: widget.contract.a\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\n" +
			"build_role: schema\n" +
			"body: |\n  a locked claim that carries a build role.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
		"claims/b.yaml": "id: widget.contract.b\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"body: |\n  a draft claim with no build role.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
	})
}

// The refusal used to carry `details: {"lint_errors": 1, ...}` — a count, and no
// finding. The router's documented recovery for lint_failed is "read
// data.lint_findings", which this envelope did not have; `check --validate`
// reported zero findings; `claim show`'s next_action pointed at that same
// command; and `check`'s next_steps offered three candidate causes, none of them
// the real one. The word the agent needed — build_role — was reachable from no
// command in the surface, and adding `build_role: behavior` made the identical
// lock succeed. That is an unbreakable loop, and the findings were computed and
// discarded one line above the refusal.
func TestLockRefusalNamesTheLintRuleThatBlockedIt(t *testing.T) {
	cfgPath := buildRoleAdoptedFixture(t)

	// The premise, stated as an assertion: the read-only pass sees nothing.
	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "check", "--validate"); err != nil {
		t.Fatalf("fixture precondition: check --validate must be clean, got %v", err)
	}

	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.b", "--reason", "go")
	if err == nil || env.OK {
		t.Fatalf("locking a claim with no build_role in an adopted module must be refused, got %+v", env)
	}
	if env.Error == nil || env.Error.Code != cliout.CodeLintFailed {
		t.Fatalf("expected %q, got %+v", cliout.CodeLintFailed, env.Error)
	}
	details, ok := env.Error.Details.(map[string]any)
	if !ok {
		t.Fatalf("the refusal must carry structured details, got %#v", env.Error.Details)
	}
	findings, ok := details["lint_findings"].([]any)
	if !ok || len(findings) == 0 {
		t.Fatalf("the refusal must carry error.details.lint_findings — the key the router tells agents to read: %+v", details)
	}
	named := false
	for _, raw := range findings {
		f, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("a lint finding must be an object: %#v", raw)
		}
		if f["lint"] == "build-role-required-for-locked" && f["claim_id"] == "widget.contract.b" {
			named = true
		}
		// The same snake_case shape check publishes as data.lint_findings, so an
		// agent that learned one shape can read the other.
		for _, key := range []string{"lint", "claim_id", "severity", "message"} {
			if _, present := f[key]; !present {
				t.Fatalf("lint finding is missing %q: %#v", key, f)
			}
		}
	}
	if !named {
		t.Fatalf("the refusal must name the rule and the claim, got %#v", findings)
	}

	// And the command claim show points at answers the same question. A
	// next_action naming a command that reports nothing is worse than none.
	dr := dryRunOf(t, "--config", cfgPath, "claim", "lock", "widget.contract.b")
	lintDetail := ""
	for _, p := range dr.Preconditions {
		if p.Name == "lint_clean" {
			if p.OK {
				t.Fatalf("the preview must agree with the write path that the lint gate blocks: %+v", p)
			}
			lintDetail = p.Detail
		}
	}
	if !strings.Contains(lintDetail, "build-role-required-for-locked") {
		t.Fatalf("the dry run's lint_clean detail must name the rule, got %q", lintDetail)
	}

	// Adding the field makes the identical command succeed — which is what makes
	// the missing rule name the ONLY thing that was ever in the way.
	claimFile := filepath.Join(filepath.Dir(cfgPath), "claims", "b.yaml")
	tamper(t, claimFile, "status: draft\n", "status: draft\nbuild_role: behavior\n")
	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.b", "--reason", "go"); err != nil {
		t.Fatalf("with build_role set, the identical lock must succeed: %v", err)
	}
}

// `claim show`'s next_action for a lint-blocked draft said "-> dossierx check
// --validate". For the whole family of lints that decide a LOCK that is a dead
// end: the claim is still draft, so the rule does not fire, and the command
// reports ok:true with zero findings. The agent is told a finding blocks its
// lock and sent to a command that reports none.
func TestClaimShowPointsAtACommandThatNamesTheBlockingLint(t *testing.T) {
	cfgPath := buildRoleAdoptedFixture(t)

	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "show", "widget.contract.b")
	if err != nil {
		t.Fatalf("claim show: %v", err)
	}
	var data struct {
		NextActions []string `json:"next_actions"`
	}
	envData(t, env, &data)

	action := ""
	for _, a := range data.NextActions {
		if strings.Contains(a, "lint finding") {
			action = a
		}
	}
	if action == "" {
		t.Fatalf("claim show must report the lint gate as what blocks locking: %v", data.NextActions)
	}
	if strings.Contains(action, "check --validate") {
		t.Fatalf("check --validate reports zero of these findings; sending the agent there is the loop: %q", action)
	}
	if !strings.Contains(action, "dossierx claim lock widget.contract.b --dry-run") {
		t.Fatalf("the next_action must name a command that can answer it: %q", action)
	}
	// The rule is named here too, so the cheapest read already carries it.
	if !strings.Contains(action, "build-role-required-for-locked") {
		t.Fatalf("the next_action must name the rule: %q", action)
	}
}
