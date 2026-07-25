// comment.go wires internal/comments into the CLI as the "dossierx comment"
// group (add/reply/resolve/reopen/edit/delete/list), mirroring the
// newBuildOrderCmd/newImplinkCmd split (a newCommentCmd() constructor in its
// own file, added to newRootCmd()'s AddCommand list). Every mutating verb
// delegates to an internal/comments Deps op, which OWNS the locking: the op
// takes the project-wide claims sentinel, re-reads the claim fresh inside it,
// mints ids, writes exactly one claim back, and releases — so this file never
// touches loader.SaveClaim or the sentinel directly, and the CLI and (in a
// later phase) the serve HTTP handlers go through one implementation and one
// locking discipline.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/BarterX-Tech/dossierx/internal/comments"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// newCommentCmd is the "dossierx comment" command group: threaded review
// discussion attached to a claim, Google-Docs style. add/reply/resolve/reopen/
// edit/delete mutate; list reads. Every mutating verb takes --as human|agent
// (the advisory role recorded on the message, also what the rights rule keys
// off) and echoes the minted/affected id so a caller can chain the next verb.
func newCommentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comment",
		Short: "Add, reply to, resolve, reopen, edit, delete, and list comment threads on a claim",
	}
	cmd.AddCommand(
		newCommentAddCmd(),
		newCommentReplyCmd(),
		newCommentResolveCmd(),
		newCommentReopenCmd(),
		newCommentEditCmd(),
		newCommentDeleteCmd(),
		newCommentListCmd(),
	)
	return cmd
}

// parseActor validates the shared --as flag and converts it to a
// model.CommentRole. It is required on every mutating verb (an empty or
// unknown value is a clear usage error rather than a downstream rights denial).
func parseActor(as string) (model.CommentRole, error) {
	switch as {
	case string(model.CommentRoleHuman), string(model.CommentRoleAgent):
		return model.CommentRole(as), nil
	case "":
		return "", fmt.Errorf("comment: --as is required (human or agent)")
	default:
		return "", fmt.Errorf("comment: --as must be %q or %q, got %q", model.CommentRoleHuman, model.CommentRoleAgent, as)
	}
}

// mutatingCommentDeps builds the Deps a mutating comment op runs against. The
// lock- and flag-store are supplied as PATHS, not pre-loaded snapshots, so each
// op RE-READS them fresh inside the claims sentinel before recomputing
// review_pending (see comments.Deps' LockStorePath/FlagStorePath doc): a
// snapshot loaded here, before the sentinel, could miss a `dossierx flag` that
// committed concurrently and orphan it with review_pending:false. Claims is left
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

func newCommentAddCmd() *cobra.Command {
	var as, body string
	cmd := &cobra.Command{
		Use:   "add <claim-id>",
		Short: "Open a new comment thread on a claim",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			actor, err := parseActor(as)
			if err != nil {
				return err
			}
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			deps, err := mutatingCommentDeps(cfg)
			if err != nil {
				return err
			}
			_, tid, err := deps.Add(args[0], actor, body)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "comment: %s added on %s%s\n", tid, args[0], viewHint)
			return nil
		},
	}
	cmd.Flags().StringVar(&as, "as", "", "role opening the thread: human or agent (required)")
	cmd.Flags().StringVar(&body, "body", "", "the comment body (required)")
	return cmd
}

func newCommentReplyCmd() *cobra.Command {
	var as, body string
	cmd := &cobra.Command{
		Use:   "reply <claim-id> <thread-id>",
		Short: "Add a reply to an open comment thread",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			actor, err := parseActor(as)
			if err != nil {
				return err
			}
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			deps, err := mutatingCommentDeps(cfg)
			if err != nil {
				return err
			}
			_, rid, err := deps.Reply(args[0], args[1], actor, body)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "comment: reply %s added to thread %s on %s%s\n", rid, args[1], args[0], viewHint)
			return nil
		},
	}
	cmd.Flags().StringVar(&as, "as", "", "role replying: human or agent (required)")
	cmd.Flags().StringVar(&body, "body", "", "the reply body (required)")
	return cmd
}

func newCommentResolveCmd() *cobra.Command {
	var as string
	cmd := &cobra.Command{
		Use:   "resolve <claim-id> <thread-id>",
		Short: "Mark a comment thread resolved (advisory rights: a human-opened thread only by a human)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			actor, err := parseActor(as)
			if err != nil {
				return err
			}
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			deps, err := mutatingCommentDeps(cfg)
			if err != nil {
				return err
			}
			if _, err := deps.Resolve(args[0], args[1], actor); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "comment: thread %s resolved on %s%s\n", args[1], args[0], viewHint)
			return nil
		},
	}
	cmd.Flags().StringVar(&as, "as", "", "role resolving: human or agent (required)")
	return cmd
}

func newCommentReopenCmd() *cobra.Command {
	var as string
	cmd := &cobra.Command{
		Use:   "reopen <claim-id> <thread-id>",
		Short: "Reopen a resolved comment thread (same advisory rights as resolve)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			actor, err := parseActor(as)
			if err != nil {
				return err
			}
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			deps, err := mutatingCommentDeps(cfg)
			if err != nil {
				return err
			}
			if _, err := deps.Reopen(args[0], args[1], actor); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "comment: thread %s reopened on %s%s\n", args[1], args[0], viewHint)
			return nil
		},
	}
	cmd.Flags().StringVar(&as, "as", "", "role reopening: human or agent (required)")
	return cmd
}

func newCommentEditCmd() *cobra.Command {
	var as, body, replyID string
	cmd := &cobra.Command{
		Use:   "edit <claim-id> <thread-id>",
		Short: "Edit a thread root's body, or a reply's body with --reply (rights key off the edited message's own author)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			actor, err := parseActor(as)
			if err != nil {
				return err
			}
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			deps, err := mutatingCommentDeps(cfg)
			if err != nil {
				return err
			}
			if _, err := deps.Edit(args[0], args[1], replyID, actor, body); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "comment: %s edited on %s%s\n", editTarget(args[1], replyID), args[0], viewHint)
			return nil
		},
	}
	cmd.Flags().StringVar(&as, "as", "", "role editing: human or agent (required)")
	cmd.Flags().StringVar(&body, "body", "", "the new body (required)")
	cmd.Flags().StringVar(&replyID, "reply", "", "edit this reply id instead of the thread root")
	return cmd
}

func newCommentDeleteCmd() *cobra.Command {
	var as, replyID string
	cmd := &cobra.Command{
		Use:   "delete <claim-id> <thread-id>",
		Short: "Delete a whole thread, or a single reply with --reply (rights key off the removed message's own author)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			actor, err := parseActor(as)
			if err != nil {
				return err
			}
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			deps, err := mutatingCommentDeps(cfg)
			if err != nil {
				return err
			}
			if _, err := deps.Delete(args[0], args[1], replyID, actor); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "comment: %s deleted on %s%s\n", editTarget(args[1], replyID), args[0], viewHint)
			return nil
		},
	}
	cmd.Flags().StringVar(&as, "as", "", "role deleting: human or agent (required)")
	cmd.Flags().StringVar(&replyID, "reply", "", "delete this reply id instead of the whole thread")
	return cmd
}

// editTarget renders the affected-message phrase for edit/delete echoes:
// "thread <tid>" when no reply is targeted, "reply <rid> in thread <tid>" when
// --reply is set.
func editTarget(threadID, replyID string) string {
	if replyID == "" {
		return "thread " + threadID
	}
	return fmt.Sprintf("reply %s in thread %s", replyID, threadID)
}

func newCommentListCmd() *cobra.Command {
	var openOnly, asJSON bool
	cmd := &cobra.Command{
		Use:   "list <claim-id>",
		Short: "List the comment threads on a claim (--open for unresolved only, --json for machine output)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, claims, err := loadConfigAndClaims()
			if err != nil {
				return err
			}
			deps := &comments.Deps{Cfg: cfg, Claims: claims}
			threads, err := deps.List(args[0], openOnly)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if asJSON {
				// Mirror reportLintFindings: a nil slice must encode as "[]",
				// not "null", so machine consumers always parse an array.
				if threads == nil {
					threads = []model.Comment{}
				}
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				if err := enc.Encode(threads); err != nil {
					return fmt.Errorf("comment list: encode: %w", err)
				}
				// Keep stdout pure JSON: the human hint goes to stderr.
				fmt.Fprintf(cmd.ErrOrStderr(), "comment list: %d thread(s) on %s\n", len(threads), args[0])
				return nil
			}
			if len(threads) == 0 {
				scope := ""
				if openOnly {
					scope = "open "
				}
				fmt.Fprintf(out, "comment list: no %sthreads on %s\n", scope, args[0])
				return nil
			}
			for _, cm := range threads {
				fmt.Fprintln(out, formatCommentLine(cm))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&openOnly, "open", false, "list only unresolved (open) threads")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print threads as JSON")
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
