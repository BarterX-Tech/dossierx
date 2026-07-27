// comment.go wires internal/comments into the CLI as the "dossierx comment"
// group: inbox, list, add, reply. Every mutating verb delegates to an
// internal/comments Deps op, which OWNS the locking: the op takes the
// project-wide claims sentinel, re-reads the claim fresh inside it, mints ids,
// writes exactly one claim back, and releases — so this file never touches
// loader.SaveClaim or the sentinel directly, and the CLI and the serve HTTP
// handlers go through one implementation and one locking discipline.
//
// FOUR VERBS ARE MISSING HERE ON PURPOSE, and they are not gone from the
// product. edit, delete, resolve and reopen are still fully implemented in
// internal/comments and still fully exposed over internal/serve's HTTP API,
// which is what the viewer drives; v0.3.0 removed them from the CLI only.
//
//   - edit/delete: a review history the agent can rewrite is not a review
//     history. They stay where the author of the message is.
//   - resolve/reopen: the advisory-rights rule (internal/comments' canAct) lets
//     an agent act only on AGENT-authored messages, and every thread opened
//     from the viewer is human-authored — so on the CLI these two could only
//     ever have acted on the agent's own threads. Vestigial. Meanwhile the
//     human's Resolve click in the viewer IS the approval signal the lock gate
//     waits for, so it belongs to the surface the rights holder is sitting at.
package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/comments"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// newCommentCmd is the "dossierx comment" command group: threaded review
// discussion attached to a claim, Google-Docs style. inbox and list read;
// add and reply mutate. Both mutating verbs take --as human|agent (the
// advisory role recorded on the message, also what the rights rule keys off)
// and echo the minted id so a caller can chain the next verb.
func newCommentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comment",
		Short: "Read the project-wide open-thread inbox, list a claim's threads, open one, and reply",
	}
	cmd.AddCommand(
		newCommentInboxCmd(),
		newCommentListCmd(),
		newCommentAddCmd(),
		newCommentReplyCmd(),
	)
	return commandGroup(cmd)
}

// parseActor validates the shared --as flag and converts it to a
// model.CommentRole. It is required on every mutating verb (an empty or
// unknown value is a clear usage error rather than a downstream rights denial).
func parseActor(as string) (model.CommentRole, error) {
	switch as {
	case string(model.CommentRoleHuman), string(model.CommentRoleAgent):
		return model.CommentRole(as), nil
	case "":
		// --as is required on every mutating comment verb and stays that way.
		// A default would let a human's own terminal action file as
		// actor=agent, which is exactly the attribution the advisory-rights
		// rule depends on being true.
		return "", cliout.Errorf(cliout.CodeMissingFlag, "comment: --as is required (human or agent)")
	default:
		return "", cliout.Errorf(cliout.CodeInvalidActor, "comment: --as must be %q or %q, got %q", model.CommentRoleHuman, model.CommentRoleAgent, as)
	}
}

// mutatingCommentDeps builds the Deps a mutating comment op runs against. The
// lock- and flag-store are supplied as PATHS, not pre-loaded snapshots, so each
// op RE-READS them fresh inside the claims sentinel before recomputing
// review_pending (see comments.Deps' LockStorePath/FlagStorePath doc): a
// snapshot loaded here, before the sentinel, could miss a `dossierx claim flag`
// that committed concurrently and orphan it with review_pending:false. Claims is left
// unset: every mutating op re-reads claims fresh inside the claims lock too.
func mutatingCommentDeps(cfg *config.Config) (*comments.Deps, error) {
	return &comments.Deps{
		Cfg:           cfg,
		LockStorePath: storePath(cfg),
		FlagStorePath: flagStorePath(cfg),
	}, nil
}

// viewHint is the trailing "how to see this" line every mutating comment verb
// prints (Blocking #5): a comment change is invisible until the viewer is
// re-rendered, so point the caller at the two commands that do so.
const viewHint = `; run "dossierx check" or "dossierx serve" to view`

// friendlyCommentBodyErr keeps the two DISTINCT store-safety failures apart and
// never lets the loader's cryptic internal round-trip text reach the user:
//
//   - comments.ErrUnsafeBody is the SUPPLIED body (add/reply) failing the
//     round-trip-accurate input pre-check; its "start with a non-whitespace
//     character / de-indent the first line" guidance is correct, so it passes
//     through unchanged.
//   - loader.ErrClaimNotRoundTrippable is the WHOLE claim's STORED bytes failing
//     to re-serialize at save time — usually a pre-existing prose or comment body
//     a user hand-edited into a store-bricking shape (a state "dossierx check"
//     passes clean), NOT the caller's supplied body. The de-indent-your-input
//     guidance is wrong for it, so it translates to a DISTINCT, claim-SCOPED
//     message that names the offending claim and points at its stored body —
//     never ErrUnsafeBody's text, never the raw internal yaml/round-trip detail.
//   - comments.ErrCommentDigestUnavailable is neither of those: it is the whole
//     op refused BEFORE anything was written, because the comment digest store
//     could not be opened. Its own message is already the right guidance, so it
//     is only reclassified, not reworded.
//
// The ErrCommentDigestUnavailable arm is the one that MUST be here rather than
// left to fall through. Without it the refusal reports as `internal`, which the
// error-code contract defines as an unclassified bug — and the documented
// response to an unclassified failure is a retry. That is exactly wrong twice
// over: this refusal is deterministic (a retry keeps failing identically until
// the store is restored from version control), and treating it as a transient
// internal fault is what the atomicity work in internal/comments exists to stop
// callers doing. The code carries the one fact a caller needs to decide —
// nothing was written — so a retry is safe but pointless.
//
// This is symmetric with serve's writeOpError, which maps ErrUnsafeBody -> 400
// unsafe_body, ErrClaimNotRoundTrippable -> 422 claim_not_serializable and
// ErrCommentDigestUnavailable -> 503 comment_digest_unavailable — and which
// still routes the CLI-retired verbs (edit/delete/resolve/reopen) through the
// same distinction. Every mutating CLI verb routes its op error through here;
// every other error (and nil) passes through unchanged.
func friendlyCommentBodyErr(claimID string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, comments.ErrCommentDigestUnavailable) {
		return cliout.Wrap(err, cliout.CodeCommentDigestUnavailable)
	}
	if errors.Is(err, loader.ErrClaimNotRoundTrippable) && !errors.Is(err, comments.ErrUnsafeBody) {
		return cliout.Errorf(cliout.CodeClaimNotSerializable, "comment: claim %q has a stored body that can't be re-serialized to YAML (likely a hand-edited leading tab or blank line in a body:); fix that claim's body and retry", claimID)
	}
	return err
}

// commentWriteData is the shared machine payload of the two surviving mutating
// comment verbs. ThreadID is the thread that was opened or replied to; ReplyID
// is set only by reply. Both are echoed so the caller can chain the next verb
// without a follow-up read.
type commentWriteData struct {
	ClaimID  string `json:"claim_id"`
	ThreadID string `json:"thread_id"`
	ReplyID  string `json:"reply_id,omitempty"`
	Actor    string `json:"actor"`
	Body     string `json:"body"`
}

// commentWriteDryRun previews a comment add/reply. Its one real precondition is
// the claim being commentable at all (banner claims cannot carry threads); the
// rest of what a reviewer needs to know is the side effects, and specifically
// the one nothing else says out loud: opening a thread on a LOCKED claim flips
// it to review_pending, which then blocks locking anything that depends on it.
func commentWriteDryRun(would string, claim model.Claim, threadID, actor, body string) *cliout.DryRun {
	dr := cliout.NewDryRun(would)

	if strings.TrimSpace(actor) == "" {
		dr.Lacking("--as")
	}
	if strings.TrimSpace(body) == "" {
		dr.Lacking("--body")
	}
	dr.Require("claim_accepts_comments", claim.Layout != model.LayoutBanner,
		fmt.Sprintf("layout is %q", claim.Layout))
	if threadID != "" {
		// Resolved here rather than through an internal/comments lookup because
		// a dry run must not go through Deps, which owns the claims sentinel and
		// exists to WRITE. A linear scan over one claim's threads is the whole
		// of the lookup anyway.
		found, open := false, false
		for _, thread := range claim.Comments {
			if thread.ID == threadID {
				found, open = true, thread.Status != model.CommentStatusResolved
				break
			}
		}
		dr.Require("thread_exists", found, "thread "+threadID)
		dr.Require("thread_is_open", open, "a resolved thread cannot take new replies")
	}

	dr.Effect("rewrites " + claim.SourcePath)
	if claim.Status == model.StatusLocked && threadID == "" {
		dr.Effect("flips this LOCKED claim to review_pending — an open thread is the third review_pending trigger, and it also blocks locking until a human resolves it")
	}
	dr.Effect("the change is invisible in the viewer until \"dossierx check\" or \"dossierx serve\" re-renders it")

	dr.Propose("actor", actor).Propose("body", body)
	return dr
}

func newCommentAddCmd() *cobra.Command {
	var as, body string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "add <claim-id>",
		Short: "Open a new comment thread on a claim",
		Args:  cobra.ExactArgs(1),
		RunE: envelopeRunE(func(cmd *cobra.Command, args []string) (cmdResult, error) {
			claimID := args[0]

			if dryRun {
				_, claims, err := loadConfigAndClaims()
				if err != nil {
					return cmdResult{}, err
				}
				claim, ok := loader.FindByID(claims, claimID)
				if !ok {
					return cmdResult{}, cliout.Errorf(cliout.CodeClaimNotFound, "comment add: claim %q not found: %w", claimID, comments.ErrClaimNotFound)
				}
				dr := commentWriteDryRun("open a comment thread on "+claimID, claim, "", as, body)
				return dryRunResult(cmd, "comment add", dr), nil
			}

			actor, err := parseActor(as)
			if err != nil {
				return cmdResult{}, err
			}
			cfg, err := loadConfig()
			if err != nil {
				return cmdResult{}, err
			}
			deps, err := mutatingCommentDeps(cfg)
			if err != nil {
				return cmdResult{}, err
			}
			_, tid, err := deps.Add(claimID, actor, body)
			if err != nil {
				return cmdResult{}, friendlyCommentBodyErr(claimID, err)
			}
			return cmdResult{
				Data: commentWriteData{ClaimID: claimID, ThreadID: tid, Actor: string(actor), Body: body},
				Text: func() {
					fmt.Fprintf(cmd.OutOrStdout(), "comment: %s added on %s%s\n", tid, claimID, viewHint)
				},
			}, nil
		}),
	}
	cmd.Flags().StringVar(&as, "as", "", "role opening the thread: human or agent (required)")
	cmd.Flags().StringVar(&body, "body", "", "the comment body (required)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what opening this thread would do, and write nothing")
	return cmd
}

func newCommentReplyCmd() *cobra.Command {
	var as, body string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "reply <claim-id> <thread-id>",
		Short: "Add a reply to an open comment thread",
		Args:  cobra.ExactArgs(2),
		RunE: envelopeRunE(func(cmd *cobra.Command, args []string) (cmdResult, error) {
			claimID, threadID := args[0], args[1]

			if dryRun {
				_, claims, err := loadConfigAndClaims()
				if err != nil {
					return cmdResult{}, err
				}
				claim, ok := loader.FindByID(claims, claimID)
				if !ok {
					return cmdResult{}, cliout.Errorf(cliout.CodeClaimNotFound, "comment reply: claim %q not found: %w", claimID, comments.ErrClaimNotFound)
				}
				dr := commentWriteDryRun("reply to thread "+threadID+" on "+claimID, claim, threadID, as, body)
				return dryRunResult(cmd, "comment reply", dr), nil
			}

			actor, err := parseActor(as)
			if err != nil {
				return cmdResult{}, err
			}
			cfg, err := loadConfig()
			if err != nil {
				return cmdResult{}, err
			}
			deps, err := mutatingCommentDeps(cfg)
			if err != nil {
				return cmdResult{}, err
			}
			_, rid, err := deps.Reply(claimID, threadID, actor, body)
			if err != nil {
				return cmdResult{}, friendlyCommentBodyErr(claimID, err)
			}
			return cmdResult{
				Data: commentWriteData{ClaimID: claimID, ThreadID: threadID, ReplyID: rid, Actor: string(actor), Body: body},
				Text: func() {
					fmt.Fprintf(cmd.OutOrStdout(), "comment: reply %s added to thread %s on %s%s\n", rid, threadID, claimID, viewHint)
				},
			}, nil
		}),
	}
	cmd.Flags().StringVar(&as, "as", "", "role replying: human or agent (required)")
	cmd.Flags().StringVar(&body, "body", "", "the reply body (required)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what replying would do, and write nothing")
	return cmd
}

// commentListData is "dossierx comment list"'s machine payload. Threads is the
// full model.Comment tree, replies included — the envelope's job is to save the
// caller a second call, not to summarize.
type commentListData struct {
	ClaimID  string          `json:"claim_id"`
	OpenOnly bool            `json:"open_only"`
	Count    int             `json:"count"`
	Threads  []model.Comment `json:"threads"`
}

func newCommentListCmd() *cobra.Command {
	var openOnly bool
	cmd := &cobra.Command{
		Use:   "list <claim-id>",
		Short: "List the comment threads on a claim (--open for unresolved only)",
		Args:  cobra.ExactArgs(1),
		RunE: envelopeRunE(func(cmd *cobra.Command, args []string) (cmdResult, error) {
			claimID := args[0]
			cfg, claims, err := loadConfigAndClaims()
			if err != nil {
				return cmdResult{}, err
			}
			deps := &comments.Deps{Cfg: cfg, Claims: claims}
			threads, err := deps.List(claimID, openOnly)
			if err != nil {
				return cmdResult{}, err
			}
			// A nil slice must encode as "[]", not "null", so machine consumers
			// always parse an array.
			if threads == nil {
				threads = []model.Comment{}
			}

			// The pre-v0.3.0 "--json" flag, which emitted a BARE ARRAY on
			// stdout, is gone: "--format json" (the default) wraps the same
			// threads in the standard envelope, and carrying a second,
			// differently-shaped JSON surface on one command was exactly the
			// inconsistency the machine contract exists to remove.
			return cmdResult{
				Data: commentListData{ClaimID: claimID, OpenOnly: openOnly, Count: len(threads), Threads: threads},
				Text: func() {
					out := cmd.OutOrStdout()
					if len(threads) == 0 {
						scope := ""
						if openOnly {
							scope = "open "
						}
						fmt.Fprintf(out, "comment list: no %sthreads on %s\n", scope, claimID)
						return
					}
					for _, cm := range threads {
						fmt.Fprintln(out, formatCommentLine(cm))
					}
				},
			}, nil
		}),
	}
	cmd.Flags().BoolVar(&openOnly, "open", false, "list only unresolved (open) threads")
	return cmd
}

// formatCommentLine is the PINNED, greppable one-line-per-thread format the
// "dossierx comment list" (non-JSON) output is a stable contract for — the
// skills bundle, the marketing site's terminal vignette, and a golden CLI test
// all reproduce it, so its shape must not drift casually:
//
//	<thread-id> <status> <author> <created> replies=<N>: <body-first-line>
//
// One line per thread root (replies are summarized by count, not expanded);
// the body is truncated to its first line so a multi-line body never breaks the
// one-line contract.
func formatCommentLine(cm model.Comment) string {
	body := cm.Body
	if i := strings.IndexByte(body, '\n'); i >= 0 {
		body = body[:i]
	}
	return fmt.Sprintf("%s %s %s %s replies=%d: %s", cm.ID, cm.Status, cm.Author, cm.Created, len(cm.Replies), body)
}

// ---------------------------------------------------------------------
// comment inbox
// ---------------------------------------------------------------------

// inboxThread is one open thread in the project-wide inbox, flattened: the
// claim it hangs off, the thread's own identity, and the two derived facts an
// agent otherwise has to work out for itself.
//
// AgentCanResolve is the important one. The advisory-rights rule
// (internal/comments' canAct) lets an agent act only on AGENT-authored
// messages, so an agent that tries to resolve a human's thread earns a
// rights_denied and has learned nothing it could not have been told up front.
// Publishing the answer here means the agent never spends that call — and,
// more to the point, never has to be TOLD not to in a skill, because the data
// already says so.
//
// AgentHasReplied is the other: the agent's actual move on a human thread is to
// fix the claim and reply, and the one thing it needs to know per thread is
// whether it has already done that.
type inboxThread struct {
	ClaimID         string `json:"claim_id"`
	ClaimTitle      string `json:"claim_title"`
	Module          string `json:"module"`
	Facet           string `json:"facet"`
	ClaimStatus     string `json:"claim_status"`
	ThreadID        string `json:"thread_id"`
	Author          string `json:"author"`
	Created         string `json:"created"`
	Body            string `json:"body"`
	Replies         int    `json:"replies"`
	LastActivity    string `json:"last_activity"`
	LastAuthor      string `json:"last_author"`
	AgentCanResolve bool   `json:"agent_can_resolve"`
	AgentHasReplied bool   `json:"agent_has_replied"`
}

// commentInboxData is "dossierx comment inbox"'s machine payload.
//
// Cursor is the highest last_activity this scan saw across EVERY open thread,
// including ones --since filtered out. Echoing it means the next poll is
// "--since <cursor>", which cannot miss a thread that arrived between two
// calls — passing the wall clock instead would silently drop anything written
// while the previous call was running.
//
// --since is INCLUSIVE of its own timestamp, deliberately. Comment timestamps
// have one-second resolution, so an exclusive cursor would silently drop any
// thread whose activity landed in the same second as the previous poll's
// newest. Re-reporting a thread costs the agent nothing (it dedupes on
// thread_id); missing the human's comment breaks the entire review loop. At
// least once, never at most once.
type commentInboxData struct {
	Since   string        `json:"since,omitempty"`
	Cursor  string        `json:"cursor"`
	Count   int           `json:"count"`
	Claims  int           `json:"claims"`
	Threads []inboxThread `json:"threads"`
}

// threadLastActivity is the timestamp a thread should be sorted and filtered
// by: the NEWEST of everything that has happened to it — its own creation, its
// newest reply, its last resolve, and its last reopen.
//
// The lifecycle stamps are in here, not just the messages, and REOPEN is the
// one that makes it load-bearing. A reopen is the human saying "this is not
// settled after all"; it puts the thread back in the inbox but adds no message,
// so a last-activity derived only from Created stamps leaves the reopened
// thread dated to its last reply. An agent polling with "--since <cursor>" —
// which the skills instruct it to do, and whose cursor has necessarily advanced
// past that older reply — then filters the reopened thread straight back out
// and never sees it. The thread the human deliberately reopened is the one
// thread the loop cannot afford to drop.
//
// ResolvedAt is folded in for symmetry and for the ordering: a thread that was
// resolved and reopened carries both, reopen is the later of the two, and
// taking the maximum means neither field's presence can drag the answer
// backwards.
//
// The values are RFC 3339 UTC strings produced by one clock, which makes them
// lexicographically comparable — so this compares raw strings rather than
// parsing to time.Time and inventing a policy for a malformed timestamp the
// engine cannot produce. Author tracks whichever event won, so last_author
// names who actually acted last.
func threadLastActivity(th model.Comment) (at string, author model.CommentRole) {
	at, author = th.Created, th.Author
	consider := func(when string, who model.CommentRole) {
		if when > at {
			at, author = when, who
		}
	}
	if n := len(th.Replies); n > 0 {
		// Replies are appended in order by internal/comments (it never reorders
		// or back-dates), so the last element is the newest.
		consider(th.Replies[n-1].Created, th.Replies[n-1].Author)
	}
	consider(th.ResolvedAt, th.ResolvedBy)
	consider(th.ReopenedAt, th.ReopenedBy)
	return at, author
}

// parseSinceCursor validates --since.
//
// An unparseable value used to be accepted and compared lexicographically
// against RFC 3339 stamps, which failed in the worst available direction:
// "yesterday" sorts ABOVE every timestamp beginning with a digit, so every open
// thread was filtered out and the command answered "0 open threads" with exit
// 0. An agent told by the skills that an empty inbox means the human has left
// nothing cannot distinguish that from the truth. Worse, the bad value was then
// echoed back as the cursor, so every subsequent poll inherited it and the
// inbox stayed empty for the rest of the session.
//
// A malformed cursor is a caller error and is reported as one. Empty stays
// legal — it means "everything", which is what the first poll wants.
//
// It also NORMALIZES to the exact UTC form the engine writes, and that half is
// not cosmetic. Every comparison in the inbox is a string comparison — the
// stored timestamps come from one clock in one format, which makes them
// lexicographically ordered — so an offset cursor like "2026-07-26T12:00:00
// +05:00" would parse happily and then compare as the characters "2026-07-26T12
// …", an hour that is really 07:00 UTC. The filter would silently answer for
// the wrong instant. Converting to UTC first makes the comparison mean what the
// caller asked.
func parseSinceCursor(since string) (string, error) {
	if since == "" {
		return "", nil
	}
	t, err := time.Parse(time.RFC3339, since)
	if err != nil {
		return "", cliout.Errorf(cliout.CodeBadRequest,
			"comment inbox: --since %q is not an RFC 3339 timestamp: %v", since, err).
			WithHint("pass the previous call's data.cursor verbatim, or omit --since to see every open thread")
	}
	return t.UTC().Format(time.RFC3339), nil
}

func newCommentInboxCmd() *cobra.Command {
	var since string
	cmd := &cobra.Command{
		Use:   "inbox",
		Short: "Every open comment thread in the project, in one call — what the human left for you",
		Long: "List every unresolved comment thread across every claim, oldest activity first.\n\n" +
			"This is the agent's half of the review loop: the human comments in the viewer and\n" +
			"says \"I left comments\"; one inbox call finds all of them, wherever they are. Use\n" +
			"--since <RFC3339> with the cursor from the previous call to see only what is new.\n\n" +
			"Note agent_can_resolve on each thread. It is almost always false, and that is by\n" +
			"design: a thread the human opened is theirs to resolve, and their Resolve click in\n" +
			"the viewer is the approval the lock gate is waiting for. Reply; do not try to close.",
		Args: cobra.NoArgs,
		RunE: envelopeRunE(func(cmd *cobra.Command, args []string) (cmdResult, error) {
			// Validated and normalized BEFORE anything is loaded: a malformed
			// cursor can only produce a wrong answer, and a wrong answer here
			// reads exactly like "the human left you nothing".
			since, err := parseSinceCursor(since)
			if err != nil {
				return cmdResult{}, err
			}
			_, claims, err := loadConfigAndClaims()
			if err != nil {
				return cmdResult{}, err
			}

			threads := make([]inboxThread, 0)
			cursor := since
			claimsWithThreads := map[string]bool{}
			for _, c := range claims {
				for _, th := range c.Comments {
					if th.Status == model.CommentStatusResolved {
						continue
					}
					at, lastAuthor := threadLastActivity(th)
					// The cursor advances over EVERY open thread, not just the
					// reported ones — see commentInboxData.Cursor.
					if at > cursor {
						cursor = at
					}
					// Inclusive: see commentInboxData.Cursor for why the
					// boundary second is re-reported rather than skipped.
					if since != "" && at < since {
						continue
					}
					agentReplied := false
					for _, rp := range th.Replies {
						if rp.Author == model.CommentRoleAgent {
							agentReplied = true
							break
						}
					}
					claimsWithThreads[c.ID] = true
					threads = append(threads, inboxThread{
						ClaimID:     c.ID,
						ClaimTitle:  claimTitle(c.ID),
						Module:      c.Module,
						Facet:       c.Facet,
						ClaimStatus: string(c.Status),
						ThreadID:    th.ID,
						Author:      string(th.Author),
						Created:     th.Created,
						Body:        th.Body,
						Replies:     len(th.Replies),

						LastActivity: at,
						LastAuthor:   string(lastAuthor),
						// The rights rule restated, not re-derived: canAct is
						// unexported in internal/comments, and duplicating its
						// ONE clause (an agent may act only on agent-authored
						// messages) is cheaper than widening that package's API
						// for a display field. TestInboxAgentCanResolveMatchesRights
						// pins the two in agreement.
						AgentCanResolve: th.Author == model.CommentRoleAgent,
						AgentHasReplied: agentReplied,
					})
				}
			}

			// Oldest activity first: an inbox is a queue to work through, and
			// the thread that has been waiting longest is the one the human is
			// most likely wondering about. Ties break on claim then thread id so
			// two runs over unchanged claims print identical bytes.
			sort.Slice(threads, func(i, j int) bool {
				if threads[i].LastActivity != threads[j].LastActivity {
					return threads[i].LastActivity < threads[j].LastActivity
				}
				if threads[i].ClaimID != threads[j].ClaimID {
					return threads[i].ClaimID < threads[j].ClaimID
				}
				return threads[i].ThreadID < threads[j].ThreadID
			})

			data := commentInboxData{
				Since:   since,
				Cursor:  cursor,
				Count:   len(threads),
				Claims:  len(claimsWithThreads),
				Threads: threads,
			}
			return cmdResult{
				Data: data,
				Text: func() {
					out := cmd.OutOrStdout()
					for _, th := range threads {
						body := th.Body
						if i := strings.IndexByte(body, '\n'); i >= 0 {
							body = body[:i]
						}
						fmt.Fprintf(out, "%s %s %s %s replies=%d agent_can_resolve=%v: %s\n",
							th.ClaimID, th.ThreadID, th.Author, th.LastActivity, th.Replies, th.AgentCanResolve, body)
					}
					fmt.Fprintf(out, "comment inbox: %d open thread(s) across %d claim(s)\n", data.Count, data.Claims)
					if data.Cursor != "" {
						fmt.Fprintf(out, "comment inbox: cursor %s\n", data.Cursor)
					}
				},
			}, nil
		}),
	}
	cmd.Flags().StringVar(&since, "since", "", "only threads whose latest activity is at or after this RFC 3339 timestamp (pass the previous call's cursor; the boundary second is re-reported rather than risk missing it)")
	return cmd
}
