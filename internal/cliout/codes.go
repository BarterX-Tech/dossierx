package cliout

// Code is a stable, snake_case machine token naming WHY something failed. It is
// the only part of an error a skill may branch on: Message is prose that will be
// reworded, Hint is advice that will improve, Code is a promise.
//
// The vocabulary below is one table shared by two surfaces. Everything under
// "shared with the serve HTTP API" is emitted BOTH as internal/serve's
// {"error":"<code>"} body and as a CLI envelope's error.code for the same
// condition, so a skill that learned "rights_denied means the human owns that
// thread" from the viewer's API already knows what the terminal is telling it.
// The CLI-only half names conditions the HTTP API structurally cannot reach
// (there is no locking, linting, or build-order endpoint).
//
// Adding a code is cheap and safe. Repurposing or deleting one is a breaking
// change to every skill in the field, so don't.
type Code string

const (
	// ---------------------------------------------------------------
	// Shared with the serve HTTP API (internal/serve/handlers.go).
	// These strings are the wire format of the viewer's comment API and
	// predate the CLI envelope; they are reproduced here verbatim, and
	// internal/serve now writes THESE constants rather than its own literals.
	// ---------------------------------------------------------------

	// CodeBadRequest is a malformed request the surface could not even parse
	// (an undecodable JSON body over HTTP; an unusable argument on the CLI).
	CodeBadRequest Code = "bad_request"
	// CodeInvalidActor is an --as / actor value that is neither "human" nor
	// "agent". Distinct from CodeMissingFlag: the caller said something, and
	// what they said is not a role.
	CodeInvalidActor Code = "invalid_actor"
	// CodeReadOnly is a write attempted against a read-only surface — a
	// file:// viewer export, or serve running with writes disabled.
	CodeReadOnly Code = "read_only"
	// CodeClaimNotFound is a claim id that does not exist in claims_dir.
	CodeClaimNotFound Code = "claim_not_found"
	// CodeThreadNotFound is a comment thread id that does not exist on the
	// named claim.
	CodeThreadNotFound Code = "thread_not_found"
	// CodeReplyNotFound is a reply id that does not exist in the named thread.
	CodeReplyNotFound Code = "reply_not_found"
	// CodeBannerClaim is a comment attempted on a banner claim, which cannot
	// carry threads.
	CodeBannerClaim Code = "banner_claim"
	// CodeEmptyBody is a comment body that is empty or whitespace-only.
	CodeEmptyBody Code = "empty_body"
	// CodeUnsafeBody is a SUPPLIED comment body that cannot be stored as YAML
	// without corrupting the claim file. The caller's input is at fault, and
	// the message says how to fix it.
	CodeUnsafeBody Code = "unsafe_body"
	// CodeClaimNotSerializable is the WHOLE claim's STORED bytes failing to
	// re-serialize — usually a pre-existing hand-edited body, NOT the caller's
	// input. Kept distinct from CodeUnsafeBody precisely so the two are not
	// confused: telling a caller to de-indent their input is wrong advice when
	// their input was fine.
	CodeClaimNotSerializable Code = "claim_not_serializable"
	// CodeRightsDenied is the advisory-rights rule refusing an actor an action
	// on someone else's message — an agent resolving a human's thread, above
	// all. This is a design invariant, not a transient failure: retrying, or
	// retrying "as" another role, is the wrong response.
	CodeRightsDenied Code = "rights_denied"
	// CodeThreadResolved is a write against an already-resolved thread.
	CodeThreadResolved Code = "thread_resolved"
	// CodeThreadOpen is a reopen of a thread that is already open.
	CodeThreadOpen Code = "thread_open"
	// CodeClaimFileChanged is the loader's optimistic-concurrency sentinel: the
	// claim file changed on disk between load and save, so the write was
	// refused rather than silently clobbering the other writer.
	CodeClaimFileChanged Code = "claim_file_changed"
	// CodeCommentDigestDrift is a comment write refused because the claim's
	// STORED comments block no longer matches the digest recorded at the
	// engine's last comment write to it — the block was edited out of band.
	//
	// The refusal is the whole point. Every comment op ends by re-recording the
	// digest from whatever the file now says, so without this gate the first
	// comment written to a hand-edited claim would silently adopt the tampered
	// block as the new truth and clear the `comment-ledger-drift` finding that
	// named it. An integrity record that any ordinary write launders is not a
	// record. Restoring the claim file from version control is the recovery;
	// re-running the op is not.
	CodeCommentDigestDrift Code = "comment_digest_drift"
	// CodeCommentDigestUnavailable is a comment write refused BEFORE anything
	// was changed because the comment digest store could not be opened for
	// writing: it does not decode, its sentinel is held, or its directory is not
	// writable.
	//
	// It is distinct from CodeInternal for the reason that made it necessary.
	// The digest refresh used to happen after the claim was already saved and
	// its failures were returned as unclassified errors, so the caller was told
	// "internal" for an op that HAD written — and the natural response to an
	// unclassified failure is a retry, which appended the same thread again on
	// every attempt. Nothing is written when this code is returned, so a retry
	// is safe and will keep failing identically until the store is restored;
	// that is what the code is for.
	CodeCommentDigestUnavailable Code = "comment_digest_unavailable"
	// CodeInternal is an unclassified failure. Its presence in a transcript is
	// a bug report, not a branch target.
	CodeInternal Code = "internal"

	// ---------------------------------------------------------------
	// CLI-only. No HTTP endpoint can reach these conditions.
	// ---------------------------------------------------------------

	// CodeConfigNotFound is "no project.config.yaml found" — the upward search
	// from the working directory came up empty and no --config was given.
	CodeConfigNotFound Code = "config_not_found"
	// CodeInvalidConfig is a project.config.yaml that exists but does not load
	// or validate.
	CodeInvalidConfig Code = "invalid_config"
	// CodeUntrackedConfig is "check --staged was asked to judge an index whose
	// project.config.yaml is not tracked". The gate reads claims, the lock
	// ledger and the digest store from the git INDEX; an untracked config is
	// editable without staging anything, so honouring the worktree copy let a
	// one-line claims_dir edit point the whole gate at a pristine decoy while
	// the index carried a tampered locked claim.
	//
	// It is its own code rather than `internal` because internal is defined as
	// an unclassified failure — "a bug report, not a branch target" — and the
	// reflex it invites is a retry. This refusal is deterministic, it is nobody's
	// bug, and it keeps failing identically until the one action that fixes it
	// is taken: git add project.config.yaml. A code an agent can branch on is
	// what turns that from a wedged commit into a one-command recovery.
	CodeUntrackedConfig Code = "untracked_config"
	// CodeInvalidClaim is a claim file under claims_dir that does not parse.
	CodeInvalidClaim Code = "invalid_claim"
	// CodeLintFailed is one or more error-severity lint findings. Emitted by
	// "check" when it stops at the lint step, and by "lock" when the lint gate
	// refuses the promotion. The recovery is the same either way: fix the
	// findings, which are in the envelope's data.
	CodeLintFailed Code = "lint_failed"
	// CodeIntegrityFailed is one or more LOCK-LEDGER findings: a locked claim
	// with no approval record, a locked claim whose content no longer matches
	// the record, a draft claim still holding an unreleased record, a comment
	// block changed outside the engine, or a ledger that could not be read.
	// Emitted by "check", "check --validate" and "check --staged"; the findings
	// themselves are in data.ledger_findings, each with its own stable rule name.
	//
	// It is deliberately NOT CodeLintFailed. The recovery is entirely different
	// — a lint finding is fixed by editing the claim, a ledger finding is fixed
	// by unlock -> fix -> lock (or by restoring the ledger from version control)
	// and must never be "fixed" by re-locking whatever the files say now — and a
	// skill that could not tell the two apart would recommend the wrong one.
	CodeIntegrityFailed Code = "integrity_failed"
	// CodeNotLocked is an operation that requires a locked claim, applied to
	// one that is still draft (flagging, linking code to it).
	CodeNotLocked Code = "not_locked"
	// CodeAlreadyLocked covers TWO states, and they do not share a recovery, so
	// read the hint rather than the code.
	//
	//  1. A lock applied to something already locked and not stale — there is
	//     nothing to do, which is a refusal rather than a silent success so the
	//     caller learns its model of the world was wrong.
	//  2. A claim whose FILE says `status: draft` while an unreleased approval
	//     still stands in the lock ledger. Here "there is nothing to do" is not
	//     merely incomplete, it is wrong in the damaging direction: content
	//     changed outside the approval path, and an agent that walks away leaves
	//     a tampered locked claim standing. The hint carries the real recovery —
	//     restore the file from version control FIRST, because unlocking now
	//     accepts the edit that caused it, then unlock, edit and lock again.
	//
	// Do not reuse this code for a third state without checking that sense (1)
	// is not what a reader will apply. internal/buildorder's unbacked-flag case
	// deliberately did not reuse it for exactly that reason.
	CodeAlreadyLocked Code = "already_locked"
	// CodePreLedgerUnadopted is an approval-recording command — claim lock, claim
	// reaudit --confirm, build-order lock — refused because this project's lock
	// store predates the lock ledger and the project still holds locked artifacts
	// that predate it. It is the write-path twin of the gate's
	// lock-ledger-pre-ledger finding.
	//
	// It is its own code for CodeUntrackedConfig's reason: a deterministic
	// refusal with exactly ONE recovery, that recovery is a fixed sequence of
	// ordinary commands, and the reflex an unclassified code invites (retry)
	// loops forever. It is also the one integrity-family condition an agent can
	// clear by itself once the human has said yes, which is precisely what a code
	// is for: the skills' recovery table can carry "pre_ledger_unadopted -> show
	// the human the ordered crossing (re-propose locked build orders, unlock
	// every locked claim, then re-lock only what they still stand behind)".
	//
	// Nothing is grandfathered on the way through. The first lock in a project
	// holding nothing locked is what stamps the store onto the ledger schema, and
	// it records a real approval, which is the whole difference from the adoption
	// path v0.4.0 removed.
	CodePreLedgerUnadopted Code = "pre_ledger_unadopted"
	// CodeReviewPending is "this claim IS review_pending, and that is what
	// blocks you" — reaudit refusing a claim whose only pending trigger is an
	// open comment thread, for instance. Contrast CodeNotReviewPending.
	CodeReviewPending Code = "review_pending"
	// CodeNotReviewPending is reaudit refusing a claim that is not
	// locked+review_pending. reaudit is the DRIFT tool, not the general edit
	// tool; the general path is unlock -> fix -> lock.
	CodeNotReviewPending Code = "not_review_pending"
	// CodeWrongState is the residual "not in the state this command requires"
	// case for which no sharper code exists yet.
	CodeWrongState Code = "wrong_state"
	// CodeUnresolvedComments is the lock gate refusing a claim that still
	// carries an open comment thread. The resolution is a human clicking
	// Resolve in the viewer — that click IS the approval this gate is waiting
	// for — so an agent must not treat this as something to work around.
	CodeUnresolvedComments Code = "unresolved_comments"
	// CodeDependencyNotLocked is doctrine hub gating: a claim cannot lock while
	// a dependency in the doctrine facet is still draft.
	CodeDependencyNotLocked Code = "dependency_not_locked"
	// CodeStructuredLayout is a body-only operation refused on a claim whose
	// rendered content lives outside body. The test is on CONTENT, not on the
	// layout name, and that is v0.4.1's widening rather than a detail: any claim
	// carrying `raw_html` is refused whatever its layout, so `layout: card`,
	// `banner`, `list` and `tree` all reach this code once one is attached — as
	// do a table's `rows` and a steps list. An agent branching on the layout name
	// alone will not expect it from a card. "dossierx claim flag" raises it because a flag-sourced reaudit
	// rewrites body and nothing else, so accepting the flag would clear
	// review_pending while leaving the actually-rendered content stale.
	CodeStructuredLayout Code = "structured_layout"
	// CodeNotProposed is a build-order operation on a module with no artifact
	// proposed yet.
	CodeNotProposed Code = "not_proposed"
	// CodeBuildOrderStale is a locked build-order artifact whose claims have
	// moved underneath it.
	CodeBuildOrderStale Code = "build_order_stale"
	// CodeBuildOrderRefused is buildorder's own refusal of a propose/lock (a
	// module with unlocked claims, a missing build_role, a dependency cycle).
	CodeBuildOrderRefused Code = "build_order_refused"
	// CodeBuildOrderHandEdited is "dossierx build-order lock" refusing to freeze
	// an artifact that is not what a fresh propose computes — the phase sequence,
	// a claim's placement, its File, or the excluded set was changed by hand.
	//
	// It is deliberately NOT CodeBuildOrderRefused. Every documented recovery for
	// that code is a repair to the CLAIMS (lock the remaining ones, resolve a
	// thread, set a missing build_role, break a rests_on cycle), and an agent
	// that reads this refusal as one of those goes and inspects claims that are
	// already correct, finds nothing to fix, and loops. Here the claims are fine
	// and the ARTIFACT is wrong, so the recovery is the one move that discards
	// it: re-run "build-order propose --module <m>" and lock what the engine
	// derives. Splitting the code is what makes those two situations
	// distinguishable without parsing the message.
	CodeBuildOrderHandEdited Code = "build_order_hand_edited"
	// CodeNoArtifact is an implementation-link operation on a module with no
	// link artifact yet.
	CodeNoArtifact Code = "no_artifact"
	// CodeImplinkRefused is implink's own refusal of a set (an unknown claim,
	// a claim that may not be linked).
	CodeImplinkRefused Code = "implink_refused"
	// CodeUnknownModule is a --module that the project's config does not
	// declare. Reported rather than answered with an empty report, because an
	// empty report for a typo'd module looks exactly like success.
	CodeUnknownModule Code = "unknown_module"
	// CodeMissingFlag is a required flag that was not supplied — --reason,
	// --as, --module. Distinct from CodeInvalidActor / CodeUnsupportedFormat,
	// which mean "supplied, but not a legal value".
	CodeMissingFlag Code = "missing_flag"
	// CodeUnsupportedFormat is a --format value other than json or text.
	CodeUnsupportedFormat Code = "unsupported_format"
	// CodeUsage is an invocation cobra itself rejected: an unknown command, an
	// unknown flag, the wrong number of positional arguments.
	CodeUsage Code = "usage"
	// CodeWriteConflict is contention on one of the project's write sentinels —
	// another dossierx process (or a serve request) holds the claims lock.
	// Retrying is the correct response, unlike CodeClaimFileChanged, which
	// means someone already wrote and the caller's snapshot is stale.
	CodeWriteConflict Code = "write_conflict"
	// CodeWriteFailed is a filesystem write that failed outright (permissions,
	// a missing directory, a full disk).
	CodeWriteFailed Code = "write_failed"
)

// ExitCode is the DEFAULT process exit status for a code.
//
// This table is additive by construction. DossierX has shipped exactly three
// statuses since v0.1.x and both tests/check_exit_test.go and
// tests/cli_uxaudit_test.go pin their meanings, with README's exit-code table
// as the public promise:
//
//	0  success
//	1  failure — a lint error, a validation failure, a write error
//	2  not found / not in the right state — no project.config.yaml, an id that
//	   doesn't exist, a claim that isn't in the state the command requires
//
// The v0.3.0 machine contract does NOT renumber them and adds no fourth
// status. It does not need one: the error CODE is now the fine-grained signal a
// skill branches on, and a fourth exit status would buy nothing while breaking
// every consumer that learned the three. So every code below resolves to 1 or 2,
// and the only question this function answers is which of the two documented
// families it belongs to.
//
// Membership follows README's own wording rather than each call site's historical
// accident: an id that does not exist, or a claim that is not in the required
// state, is family 2 wherever it is reported. Where a specific historical call
// site must keep a status this table would change, that call site sets
// Error.Exit explicitly instead of relying on this default.
func ExitCode(c Code) int {
	switch c {
	case CodeConfigNotFound,
		CodeClaimNotFound,
		CodeThreadNotFound,
		CodeReplyNotFound,
		CodeNotLocked,
		CodeNotReviewPending,
		CodeReviewPending,
		CodeWrongState:
		return 2
	default:
		return 1
	}
}
