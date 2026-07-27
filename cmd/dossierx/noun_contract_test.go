// noun_contract_test.go pins the three CLI-surface repairs that a caller can
// only discover by making the call: what a bare NOUN answers, what an
// unparseable --since answers, and which timestamp an inbox thread is dated by.
//
// All three share a failure shape. Each one used to return a SUCCESSFUL,
// well-formed-looking answer that was wrong, which is the worst thing a machine
// contract can do — an agent reading only ok/exit status has no way to notice.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/check"
	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/loader"
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
	for _, noun := range []string{"claim", "comment", "build-order", "skills"} {
		t.Run(noun, func(t *testing.T) {
			env, _, err := execCLIJSON(t, noun)
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
// "dossierx --help" and "dossierx --version" are unaffected: cobra handles both
// before RunE, and a caller that ASKED for prose gets prose.
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
	env, _, err := execCLIJSON(t, "claim", "definitely-not-a-verb")
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

// TestCheckReconcilesReviewPendingFromTheFlagStore.
//
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

	if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "flag", "widget.contract.a",
		"--claim-says", "the original approved body.", "--now-does", "something else entirely",
		"--reason", "the code does not match the claim"); err != nil {
		t.Fatalf("claim flag: %v", err)
	}
	if raw, _ := os.ReadFile(claimFile); !strings.Contains(string(raw), "review_pending: true") {
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
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "check"); err != nil {
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
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "propose", "--module", "widget"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "approved"); err != nil {
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

// TestPreLedgerProjectGrandfathersItsLockedBuildOrder.
//
// Build orders could be locked before this release gave them a ledger record.
// Without adoption, every such project would fail `check` on upgrade with a
// build-order-ledger-missing it had no way to have avoided — a gate firing on
// correct state, which is how gates get switched off. Adoption is scoped to a
// store file that EXISTS and predates the ledger, the same predicate the
// per-claim grandfathering uses, so deleting the ledger still cannot re-bless
// anything.
func TestPreLedgerProjectGrandfathersItsLockedBuildOrder(t *testing.T) {
	cfgPath := buildOrderFixture(t)
	root := filepath.Dir(cfgPath)

	if _, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "propose", "--module", "widget"); err != nil {
		t.Fatalf("propose: %v", err)
	}
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "approved"); err != nil {
		t.Fatalf("lock: %v", err)
	}

	// Rewind the store to what a pre-ledger build would have left behind: an
	// existing file, at the old schema version, with no ledger at all.
	storeFile := filepath.Join(root, ".dossierx-lock-store.json")
	raw, err := os.ReadFile(storeFile)
	if err != nil {
		t.Fatalf("read store: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse store: %v", err)
	}
	delete(doc, "ledger")
	doc["version"] = 1
	rewound, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal store: %v", err)
	}
	if err := os.WriteFile(storeFile, rewound, 0o644); err != nil {
		t.Fatalf("write store: %v", err)
	}

	// The upgrade run. It must not report the build order as unapproved.
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "check"); err != nil {
		t.Fatalf("the first check after an upgrade must not fail: %v", err)
	}

	// And the adoption is on the record, marked as an adoption rather than an
	// approval, so nobody mistakes it for one.
	after, err := os.ReadFile(storeFile)
	if err != nil {
		t.Fatalf("re-read store: %v", err)
	}
	if !strings.Contains(string(after), `"build-order:widget"`) {
		t.Fatalf("the locked build order was not grandfathered:\n%s", after)
	}
	if !strings.Contains(string(after), `"grandfathered": true`) {
		t.Fatalf("an adopted build-order record must be marked grandfathered:\n%s", after)
	}
}
