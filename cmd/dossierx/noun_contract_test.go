// noun_contract_test.go pins the three CLI-surface repairs that a caller can
// only discover by making the call: what a bare NOUN answers, what an
// unparseable --since answers, and which timestamp an inbox thread is dated by.
//
// All three share a failure shape. Each one used to return a SUCCESSFUL,
// well-formed-looking answer that was wrong, which is the worst thing a machine
// contract can do — an agent reading only ok/exit status has no way to notice.
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/check"
	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// TestBareNounIsOneUsageEnvelope: a command group does no work, so invoking one
// is an incomplete invocation, not a successful operation.
//
// Cobra's default for a parent with no Run/RunE is to print its help text and
// return nil, which broke the contract in both halves at once — the bytes on
// stdout were help prose rather than the one envelope --format json promises,
// and the process exited 0. An agent that checked the status concluded its call
// had succeeded and that the empty result was the answer.
func TestBareNounIsOneUsageEnvelope(t *testing.T) {
	for _, noun := range []string{"claim", "comment", "build-order", "track", "theme", "skills"} {
		t.Run(noun, func(t *testing.T) {
			env, _, err := execReviewedCLIJSON(t, noun)
			if err == nil {
				t.Fatalf("a bare %q must fail; it did not", noun)
			}
			if env.OK {
				t.Fatalf("a bare %q must emit ok:false, got %+v", noun, env)
			}
			if env.Error == nil || env.Error.Code != cliout.CodeUsage {
				t.Fatalf("a bare %q must carry error.code %q, got %+v", noun, cliout.CodeUsage, env.Error)
			}
			if exitStatusFor(err) != 1 {
				t.Fatalf("a bare %q must exit 1, got %d", noun, exitStatusFor(err))
			}
			// The recovery has to name the leaves, or the agent's only move is
			// to re-read documentation it was told to stop re-deriving.
			if env.Error.Hint == "" {
				t.Fatalf("a bare %q must hint at its subcommands, got %+v", noun, env.Error)
			}
		})
	}
}

// TestBareRootIsOneUsageEnvelope: the ROOT had the same hole the four nouns
// were fixed for, and it was the last one left.
//
// `dossierx` alone printed the help banner on STDOUT and exited 0 — help prose
// where --format json promises exactly one envelope, and a success status for an
// invocation that did nothing. An agent building its argv programmatically
// reaches this whenever the verb comes from an empty variable ("dossierx", or
// "dossierx $NOUN" with NOUN unset); the wrapper reports success, and json.loads
// on stdout either throws on the banner or — in a pipeline that tolerates parse
// failures — records a successful no-op. Both the nouns and the leaves fail
// loudly here; only the root silently succeeded.
//
// "dossierx --help" is unaffected: cobra handles it before RunE, and a caller
// that ASKED for prose gets prose. "--version" no longer belongs in that
// sentence — it was cobra's built-in, which had the identical hole (prose on
// stdout, exit 0, no envelope) and is now answered by the root's own RunE. See
// TestVersionFlagAnswersInAnEnvelope in machine_contract_test.go.
func TestBareRootIsOneUsageEnvelope(t *testing.T) {
	env, _, err := execCLIJSON(t)
	if err == nil {
		t.Fatal("a bare \"dossierx\" must fail; it did not")
	}
	if env.OK {
		t.Fatalf("a bare \"dossierx\" must emit ok:false, got %+v", env)
	}
	if env.Error == nil || env.Error.Code != cliout.CodeUsage {
		t.Fatalf("a bare \"dossierx\" must carry error.code %q, got %+v", cliout.CodeUsage, env.Error)
	}
	if exitStatusFor(err) != 1 {
		t.Fatalf("a bare \"dossierx\" must exit 1, got %d", exitStatusFor(err))
	}
	// The hint has to name the nouns: requireSubcommand's own label is
	// commandPath, which is EMPTY for the root, so a reused message would have
	// read "': a subcommand is required; ' is a command group".
	if !strings.Contains(env.Error.Hint, "claim") || !strings.Contains(env.Error.Hint, "check") {
		t.Fatalf("the root's hint must name the nouns, got %q", env.Error.Hint)
	}
	if strings.Contains(env.Error.Message, "\"\"") {
		t.Fatalf("the root's message must not be labelled with an empty command path: %q", env.Error.Message)
	}
}

// An unknown leaf reaches the same place (cobra's legacyArgs lets a non-root
// parent take arbitrary positionals), and naming what was actually typed is
// worth more than a generic complaint.
func TestUnknownLeafUnderANounIsUsage(t *testing.T) {
	env, _, err := execReviewedCLIJSON(t, "claim", "definitely-not-a-verb")
	if err == nil || env.OK {
		t.Fatalf("an unknown leaf must fail, got %+v", env)
	}
	if env.Error == nil || env.Error.Code != cliout.CodeUsage {
		t.Fatalf("expected %q, got %+v", cliout.CodeUsage, env.Error)
	}
}

// TestMalformedSinceIsRefused: an unparseable --since used to be compared
// lexicographically against RFC 3339 stamps, and it failed in the worst
// available direction. "yesterday" sorts ABOVE every timestamp beginning with a
// digit, so every open thread was filtered out and the command answered "0 open
// threads" at exit 0 — indistinguishable, to an agent, from the human having
// left nothing. The bad value was then echoed back as data.cursor, so every
// subsequent poll in the session inherited it.
func TestMalformedSinceIsRefused(t *testing.T) {
	for _, bad := range []string{"yesterday", "2026-07-26", "now", "1753488000"} {
		t.Run(bad, func(t *testing.T) {
			if _, err := parseSinceCursor(bad); err == nil {
				t.Fatalf("--since %q must be refused", bad)
			} else if e := cliout.As(err); e == nil || e.Code != cliout.CodeBadRequest {
				t.Fatalf("--since %q must be %q, got %+v", bad, cliout.CodeBadRequest, e)
			}
		})
	}
	// Empty stays legal: it means "everything", which is what a first poll wants.
	if got, err := parseSinceCursor(""); err != nil || got != "" {
		t.Fatalf("an empty --since must remain legal and stay empty, got %q / %v", got, err)
	}
	// A real cursor — the exact shape data.cursor emits — passes through
	// unchanged, so the documented "pass the cursor verbatim" loop is stable.
	if got, err := parseSinceCursor("2026-07-26T12:00:00Z"); err != nil || got != "2026-07-26T12:00:00Z" {
		t.Fatalf("a well-formed UTC cursor must round-trip unchanged, got %q / %v", got, err)
	}

	// An OFFSET cursor is normalized to UTC. Every comparison in the inbox is a
	// STRING comparison — the stored stamps come from one clock in one format,
	// which is what makes them lexicographically ordered — so "+05:00" would
	// otherwise compare as the characters "T12:…" while meaning 07:00 UTC, and
	// the filter would silently answer for the wrong instant.
	got, err := parseSinceCursor("2026-07-26T12:00:00+05:00")
	if err != nil {
		t.Fatalf("a valid offset cursor must be accepted, got %v", err)
	}
	if got != "2026-07-26T07:00:00Z" {
		t.Fatalf("an offset cursor must normalize to UTC, got %q", got)
	}
}

// TestThreadLastActivityTracksTheReopen is the inbox's half of the review loop.
//
// A reopen is the human saying "this is not settled after all". It puts the
// thread back in the inbox but adds no message, so a last-activity derived only
// from Created stamps left the reopened thread dated to its last reply — and an
// agent polling with "--since <cursor>", exactly as the skills instruct, has a
// cursor that has necessarily advanced past that older reply. The thread the
// human deliberately reopened was the one thread the loop dropped.
func TestThreadLastActivityTracksTheReopen(t *testing.T) {
	const (
		created  = "2026-07-26T10:00:00Z"
		replied  = "2026-07-26T11:00:00Z"
		resolved = "2026-07-26T12:00:00Z"
		reopened = "2026-07-26T13:00:00Z"
	)

	thread := model.Comment{
		ID: "c-abc123", Status: model.CommentStatusOpen,
		Author: model.CommentRoleHuman, Created: created,
		Replies:    []model.Reply{{ID: "r-1", Author: model.CommentRoleAgent, Created: replied}},
		ResolvedBy: model.CommentRoleHuman, ResolvedAt: resolved,
		ReopenedBy: model.CommentRoleHuman, ReopenedAt: reopened,
	}

	at, author := threadLastActivity(thread)
	if at != reopened {
		t.Fatalf("last_activity must be the reopen (%s), got %s", reopened, at)
	}
	if author != model.CommentRoleHuman {
		t.Fatalf("last_author must be whoever acted last (human), got %q", author)
	}

	// A thread that was reopened is dated AFTER a poll taken at its newest
	// reply, which is the filter comparison the inbox actually makes.
	if !(at >= replied) {
		t.Fatalf("a reopened thread must not sort before its last reply")
	}

	// The ordinary cases must not regress: a plain thread is dated by its own
	// Created, and a replied-to thread by its newest reply.
	plain := model.Comment{ID: "c-1", Author: model.CommentRoleHuman, Created: created}
	if at, who := threadLastActivity(plain); at != created || who != model.CommentRoleHuman {
		t.Fatalf("a plain thread must be dated by its own Created, got %s/%s", at, who)
	}
	withReply := model.Comment{
		ID: "c-2", Author: model.CommentRoleHuman, Created: created,
		Replies: []model.Reply{{ID: "r-1", Author: model.CommentRoleAgent, Created: replied}},
	}
	if at, who := threadLastActivity(withReply); at != replied || who != model.CommentRoleAgent {
		t.Fatalf("a replied-to thread must be dated by its newest reply, got %s/%s", at, who)
	}
}

// review_pending has THREE documented triggers — dependency drift, a pending
// "dossierx claim flag", and an open comment thread — and check's reconciler
// only ever knew about two of them. It ran lock.DetectStale (drift) and its own
// open-thread test, and never opened the flag store.
//
// That made check disagree with every other reader of the same state.
// comments.Recompute (the comment write path) and comments.PendingTriggers
// (check's own next_steps) both consult the flag store, so a claim whose
// review_pending had been cleared — by a hand edit, or by any path that
// recomputed it without the flag in view — kept a live flag entry that check
// could see in its hints and would not restore to the claim file. The flag
// stayed pending, the claim read as settled, and "claim list --review-pending"
// did not list it.
func TestCheckReconcilesReviewPendingFromTheFlagStore(t *testing.T) {
	root := t.TempDir()
	cfgPath := writeCheckFixture(t, root, parityConfig, map[string]string{
		"claims/a.yaml": "id: widget.contract.a\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\n" +
			"body: |\n  the original approved body.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
	})
	claimFile := filepath.Join(root, "claims", "a.yaml")

	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "flag", "widget.contract.a",
		"--claim-says", "the original approved body.", "--now-does", "something else entirely",
		"--reason", "the code does not match the claim"); err != nil {
		t.Fatalf("claim flag: %v", err)
	}
	if raw, _ := os.ReadFile(claimFile); !strings.Contains(string(raw), "review_pending: true") { //nolint:errcheck // a missing file fails the assertion below, which is the point
		t.Fatalf("fixture precondition: flagging must set review_pending")
	}

	// The hand edit: clear review_pending in the YAML. The flag entry in
	// .dossierx-flag-store.json is untouched and still pending.
	raw, err := os.ReadFile(claimFile)
	if err != nil {
		t.Fatalf("read claim: %v", err)
	}
	var kept []string
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.Contains(line, "review_pending") {
			kept = append(kept, line)
		}
	}
	if err := os.WriteFile(claimFile, []byte(strings.Join(kept, "\n")), 0o644); err != nil {
		t.Fatalf("write claim: %v", err)
	}

	// check must put it back.
	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "check"); err != nil {
		t.Logf("check returned %v (the gate's verdict is not what this test is about)", err)
	}
	after, err := os.ReadFile(claimFile)
	if err != nil {
		t.Fatalf("re-read claim: %v", err)
	}
	if !strings.Contains(string(after), "review_pending: true") {
		t.Fatalf("check left a claim with a live flag entry reading as settled:\n%s", after)
	}
}

// TestBuildOrderSignatureMatchesTheGate is the parity pin between the two sides
// of the build-order ledger record: cmd/dossierx WRITES the signature and
// internal/check's gate RE-COMPUTES it to compare. They are separate functions
// (sharing one would mean cmd/ and check/ importing each other), so nothing but
// this test stops them drifting — and a one-byte disagreement would report
// build-order-content-drift on every honestly locked build order in every
// project, which is a gate firing on correct state.
func TestBuildOrderSignatureMatchesTheGate(t *testing.T) {
	cfgPath := buildOrderFixture(t)
	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "build-order", "propose", "--module", "widget"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "approved"); err != nil {
		t.Fatalf("lock: %v", err)
	}

	// The gate's own verdict, through the public seam: silence means the two
	// signatures agreed on a freshly locked, untouched artifact.
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	claims, err := loader.LoadClaims(cfg.ClaimsDir)
	if err != nil {
		t.Fatalf("load claims: %v", err)
	}
	for _, f := range check.Status(claims, cfg).LedgerFindings {
		if strings.HasPrefix(f.Rule, "build-order-") {
			t.Fatalf("the writer's signature and the gate's disagree on an untouched artifact: %s: %s", f.Rule, f.Message)
		}
	}
}

// TestAPreLedgerProjectCrossesByEmptyingItself is v0.4.0's whole answer to
// "how does a project that predates the lock ledger ever get onto it", end to
// end, through the CLI.
//
// It replaces a test that ran the removed migration and asserted a GRANDFATHERED
// record at the end. That is the assertion this release exists to delete:
// grandfathering records content nobody approved as the baseline every later
// change is judged against, and no amount of ceremony around the command makes
// the record it writes evidence of anything.
//
// What replaces it costs more and claims less. The project is emptied of
// everything that predates the ledger, in the order the refusal gives (propose
// FIRST — propose requires the module still fully locked, so unlocking a claim
// first would leave the order stuck), and the first re-lock crosses the store.
// Nothing is grandfathered, because by then there is nothing to grandfather.
//
// The last two assertions are the ones that catch a crossing point that stamps
// only the schema version: the comment digest store must be created by the same
// act, and the finding set is asserted EMPTY rather than merely free of two
// named rules.
func TestAPreLedgerProjectCrossesByEmptyingItself(t *testing.T) {
	cfgPath := buildOrderFixture(t) // one claim, status: locked, module widget
	root := filepath.Dir(cfgPath)
	const id = "widget.contract.a"

	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "build-order", "propose", "--module", "widget"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "approved"); err != nil {
		t.Fatalf("lock: %v", err)
	}

	// Rewind to what a pre-ledger build would have left behind: an existing
	// store file, at the old schema version, with no ledger at all, and no
	// comment digest store beside it. The shared helper is what knows the full
	// list — rewinding only the store reproduces the downgrade attack instead
	// of an upgrade, and lock.Store.LedgerDowngraded refuses that on purpose.
	storeFile := filepath.Join(root, ".dossierx-lock-store.json")
	rewindStoreToPreLedger(t, storeFile)

	// 1. The refusal, on both write paths, and that it IS the refusal.
	lockEnv, _, lockErr := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", id, "--reason", "approved")
	if lockErr == nil || lockEnv.Error == nil || lockEnv.Error.Code != cliout.CodePreLedgerUnadopted {
		t.Fatalf("claim lock must refuse with %q, got %+v", cliout.CodePreLedgerUnadopted, lockEnv.Error)
	}
	boEnv, _, boErr := execReviewedCLIJSON(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "approved")
	if boErr == nil || boEnv.Error == nil || boEnv.Error.Code != cliout.CodePreLedgerUnadopted {
		t.Fatalf("build-order lock must refuse with %q, got %+v", cliout.CodePreLedgerUnadopted, boEnv.Error)
	}

	// 2. The recovery, in the order the refusal text gives.
	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "build-order", "propose", "--module", "widget"); err != nil {
		t.Fatalf("re-propose (FIRST: propose needs the module still fully locked): %v", err)
	}
	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "unlock", id, "--reason", "crossing onto the ledger"); err != nil {
		t.Fatalf("unlock is gateless and always has been: %v", err)
	}
	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", id, "--reason", "re-approved after the crossing"); err != nil {
		t.Fatalf("the crossing lock: %v", err)
	}
	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "build-order", "propose", "--module", "widget"); err != nil {
		t.Fatalf("re-propose after the crossing: %v", err)
	}
	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "re-approved after the crossing"); err != nil {
		t.Fatalf("build-order lock after the crossing: %v", err)
	}

	// 3. The assertions.
	after, err := os.ReadFile(storeFile)
	if err != nil {
		t.Fatalf("re-read store: %v", err)
	}
	if !strings.Contains(string(after), `"version": 2`) {
		t.Fatalf("the crossing must stamp the ledger schema:\n%s", after)
	}
	for _, key := range []string{id, lock.BuildOrderLedgerKey("widget")} {
		rec, ok := readLedger(t, storeFile)[key]
		if !ok {
			t.Fatalf("expected a record for %q:\n%s", key, after)
		}
		if rec.Grandfathered {
			t.Fatalf("%q must hold a real APPROVAL, never a grandfathered adoption: %+v", key, rec)
		}
	}
	if _, statErr := os.Stat(filepath.Join(root, ".dossierx-comment-digest.json")); statErr != nil {
		t.Fatalf("the crossing must create the comment digest store in the same act: %v", statErr)
	}

	// The finding set is asserted EMPTY, not merely free of two named rules: a
	// crossing point that stamped only the schema version would leave
	// comment-digest-absent behind, pointing at a file that never existed.
	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "check", "--validate")
	if err != nil || !env.OK {
		t.Fatalf("check --validate after the crossing: %v (%+v)", err, env)
	}
	var data checkData
	envData(t, env, &data)
	if len(data.LedgerFindings) != 0 {
		t.Fatalf("the crossed project must be clean, got %+v", data.LedgerFindings)
	}
}

// ---------------------------------------------------------------------
// a removed verb must fail on the REMOVAL, not on the flags it used to take
// ---------------------------------------------------------------------

// TestRetiredInvocationsNameTheirReplacement runs each removed verb the way an
// agent carrying pre-v0.3.0 memory actually types it — WITH the flags that verb
// used to take — and requires a hint naming where the capability went.
//
// The flags are the whole test. `dossierx comment resolve <claim> <thread>`
// already reached requireSubcommand's good message ("comment is a command group
// and does nothing on its own", hint listing add/inbox/list/reply). Add `--as
// agent` and cobra failed at flag-parse time with `unknown flag: --as` and no
// hint at all — which reads as though `comment resolve` exists and merely takes
// different flags now, and which is unreachable for the good message because
// parsing runs before the unknown-subcommand handler. SKILL.md anticipates that
// exact recall; the binary answered the bare form and not the remembered one.
//
// The retired TOP-LEVEL names had a second, separate version of the same hole:
// cobra rejects an unknown root command during Execute, so `dossierx lint`
// reported `unknown command "lint" for "dossierx"` with an empty command field
// and no hint whatsoever.
func TestRetiredInvocationsNameTheirReplacement(t *testing.T) {
	// wantRemovedIn is per-case rather than a shared literal because the stubs no
	// longer all come from one release: v0.4.0 removed the migration verb, and a
	// stub that claimed the wrong release would send a reader to the wrong
	// CHANGELOG entry for the reasoning.
	cases := []struct {
		name          string
		argv          []string
		wantHint      string
		wantRemovedIn string
	}{
		// The four comment verbs, each with the flags it used to take.
		{"comment resolve --as", []string{"comment", "resolve", "widget.contract.a", "c-abc123", "--as", "agent"}, "dossierx comment reply", "removed in v0.3.0"},
		{"comment reopen --as", []string{"comment", "reopen", "widget.contract.a", "c-abc123", "--as", "human"}, "dossierx comment reply", "removed in v0.3.0"},
		{"comment edit --body", []string{"comment", "edit", "widget.contract.a", "c-abc123", "--as", "agent", "--body", "revised"}, "dossierx comment reply", "removed in v0.3.0"},
		{"comment delete --as", []string{"comment", "delete", "widget.contract.a", "c-abc123", "--as", "agent"}, "dossierx comment reply", "removed in v0.3.0"},

		// The retired top-level verbs, with the flags they used to take.
		{"lint --json", []string{"lint", "--json"}, "dossierx check", "removed in v0.3.0"},
		{"catalog --out", []string{"catalog", "--out", ".catalog.json"}, "dossierx check", "removed in v0.3.0"},
		{"render --out", []string{"render", "--out", "viewer/index.html"}, "dossierx check", "removed in v0.3.0"},
		{"deps <id>", []string{"deps", "widget.contract.a"}, "dossierx claim show", "removed in v0.3.0"},
		{"stale --json", []string{"stale", "--json"}, "dossierx claim list --review-pending", "removed in v0.3.0"},
		{"coverage --module", []string{"coverage", "--module", "widget"}, "dossierx claim list --migrated", "removed in v0.3.0"},
		{"implink set", []string{"implink", "set", "widget.contract.a", "--file", "main.go"}, "dossierx claim link", "removed in v0.3.0"},
		{"implink status", []string{"implink", "status", "--module", "widget"}, "dossierx claim show", "removed in v0.3.0"},

		// v0.4.0's removal. It is the invocation README, SKILL.md, the CI
		// template and CHANGELOG all spent a release telling agents to type, and
		// flag parsing runs first — so without the stub, `--adopt` surfaces as
		// `unknown flag` and the removal is never named at all.
		{"migrate --adopt", []string{"migrate", "--adopt"}, "dossierx claim unlock", "removed in v0.4.0"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env, _, err := execReviewedCLIJSON(t, tc.argv...)
			if err == nil || env.OK {
				t.Fatalf("a removed verb must fail, got %+v", env)
			}
			if env.Error == nil || env.Error.Code != cliout.CodeUsage {
				t.Fatalf("a removed verb is the CALLER's invocation being wrong, so %q; got %+v", cliout.CodeUsage, env.Error)
			}
			// The failure must be about the removal, never about a flag the
			// removed verb legitimately used to take.
			if strings.Contains(env.Error.Message, "unknown flag") {
				t.Fatalf("the failure must be about the removed verb, not its flags: %+v", env.Error)
			}
			if !strings.Contains(env.Error.Message, tc.wantRemovedIn) {
				t.Fatalf("the message must say which release removed the verb (%q): %+v", tc.wantRemovedIn, env.Error)
			}
			if !strings.Contains(env.Error.Hint, tc.wantHint) {
				t.Fatalf("the hint must name the replacement %q, got %+v", tc.wantHint, env.Error)
			}
			// The envelope has to say WHICH call this answers. The root-level
			// cobra rejection reported command:"" — nothing to correlate with.
			if env.Command == "" {
				t.Fatalf("the envelope must name the command it answers: %+v", env)
			}
		})
	}
}

// TestRetiredCommentVerbsSayResolvingIsTheHumans is the rights half, stated
// separately because it is the part an agent must not work around.
//
// "run this other command instead" is not sufficient guidance for resolve: an
// agent told only that will go looking for another door. The hint has to say
// that resolving is the human's approval and lives in the viewer, which is the
// same rule internal/comments' canAct enforces in code.
func TestRetiredCommentVerbsSayResolvingIsTheHumans(t *testing.T) {
	env, _, err := execReviewedCLIJSON(t, "comment", "resolve", "widget.contract.a", "c-abc123", "--as", "agent")
	if err == nil {
		t.Fatal("comment resolve must fail")
	}
	if env.Error == nil {
		t.Fatalf("expected an error envelope, got %+v", env)
	}
	for _, want := range []string{"the human", "viewer"} {
		if !strings.Contains(env.Error.Hint, want) {
			t.Fatalf("the hint must state the rights rule (%q missing): %+v", want, env.Error)
		}
	}
	if !strings.Contains(env.Error.Message, "the approval the lock gate waits for") {
		t.Fatalf("the message must say what a Resolve click IS: %+v", env.Error)
	}
}

// TestRetiredVerbsAreNotSurface: the stubs answer, and they stay invisible.
//
// They must not appear in --help, in requireSubcommand's "run one of:" list, or
// in the leaf count TestSurfaceIsTwentyFiveLeavesUnderNineNouns pins.
// A removal explanation that advertises itself is a re-addition.
func TestRetiredVerbsAreNotSurface(t *testing.T) {
	env, _, err := execReviewedCLIJSON(t, "comment")
	if err == nil {
		t.Fatal("a bare noun must fail")
	}
	if env.Error == nil {
		t.Fatalf("expected an error envelope, got %+v", env)
	}
	for _, gone := range []string{"resolve", "reopen", "edit", "delete"} {
		if strings.Contains(env.Error.Hint, gone) {
			t.Fatalf("the comment group's leaf list must not advertise the removed verb %q: %q", gone, env.Error.Hint)
		}
	}
	for _, cmd := range newRootCmd().Commands() {
		if retired(cmd) && !cmd.Hidden {
			t.Fatalf("a removal stub must be hidden: %q", cmd.Name())
		}
	}
}
