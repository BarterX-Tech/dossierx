package main

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/BarterX-Tech/dossierx/internal/briefs"
	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/comments"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/digest"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// comment_brief.go routes `comment add`, `comment reply` and `comment list` to
// a brief (NIT-205) when their first argument is a brief's PATH — the form
// `brief list` prints, briefs/<folder>/<slug>.md. A path holds a slash and a
// claim id never does, so the two cannot be confused; a brief's
// <folder>.<slug> id could be, which is why it is not accepted here.

// isBriefRef reports whether a comment verb's subject is a brief path.
func isBriefRef(arg string) bool {
	return strings.Contains(arg, "/")
}

// briefIDHint turns a claim_not_found on a comment verb into one that says
// what went wrong when the argument is a brief's <folder>.<slug> id: the verbs
// take a brief by its path (an id can collide with a claim id), so the hint
// names the path. Any other error, or an argument that is no brief's id, is
// returned unchanged.
func briefIDHint(cfg *config.Config, verb, arg string, err error) error {
	if err == nil || cfg == nil {
		return err
	}
	if e := errorForCLI(err); e == nil || e.Code != cliout.CodeClaimNotFound {
		return err
	}
	b, ok := briefs.Load(cfg).Lookup(arg)
	if !ok || b.ID != arg {
		return err
	}
	return cliout.Errorf(cliout.CodeClaimNotFound, "%s: no claim %q — that is a brief's id, and comment verbs take a brief by its path: %w", verb, arg, err).
		WithHint(fmt.Sprintf("run: dossierx %s %s … (the brief's path, as brief list prints it)", verb, b.Path))
}

// briefCommentError maps a brief comment op's error onto the envelope codes the
// claim ops use.
func briefCommentError(cfg *config.Config, ref string, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, comments.ErrBriefNotFound):
		return cliout.Errorf(cliout.CodeBriefNotFound, "comment: no brief at %q: %w", ref, err).
			WithHint("run: dossierx brief list — and pass the path it prints")
	case errors.Is(err, comments.ErrBriefFileChanged):
		return cliout.Wrap(err, cliout.CodeClaimFileChanged)
	}
	return commentOpError(cfg, ref, err)
}

// briefCommentWrite is `comment add` (threadID "") and `comment reply` on a
// brief, with their --dry-run.
func briefCommentWrite(cmd *cobra.Command, verb, ref, threadID, as, body string, dryRun bool) (cmdResult, error) {
	cfg, err := loadConfig()
	if err != nil {
		return cmdResult{}, err
	}
	if dryRun {
		b, ok := briefs.Load(cfg).Lookup(ref)
		if !ok {
			return cmdResult{}, briefCommentError(cfg, ref, fmt.Errorf("comments: brief %q: %w", ref, comments.ErrBriefNotFound))
		}
		would := "open a comment thread on " + b.Path
		if threadID != "" {
			would = "reply to thread " + threadID + " on " + b.Path
		}
		return dryRunResult(cmd, verb, briefCommentWriteDryRun(would, cfg, b, threadID, as, body)), nil
	}
	actor, err := parseActor(as)
	if err != nil {
		return cmdResult{}, err
	}
	deps, err := mutatingCommentDeps(cfg)
	if err != nil {
		return cmdResult{}, err
	}
	var b briefs.Brief
	var tid, rid string
	if threadID == "" {
		b, tid, err = deps.BriefAdd(ref, actor, body)
	} else {
		tid = threadID
		b, rid, err = deps.BriefReply(ref, threadID, actor, body)
	}
	if err != nil {
		return cmdResult{}, briefCommentError(cfg, ref, err)
	}
	return cmdResult{
		Data: commentWriteData{ClaimID: b.Path, ThreadID: tid, ReplyID: rid, Actor: string(actor), Body: body},
		Text: func() {
			if rid != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "comment: reply %s added to thread %s on %s%s\n", rid, tid, b.Path, viewHint)
				return
			}
			fmt.Fprintf(cmd.OutOrStdout(), "comment: %s added on %s%s\n", tid, b.Path, viewHint)
		},
	}, nil
}

// briefCommentWriteDryRun is commentWriteDryRun for a brief: the same input
// checks, the brief's own digest gate, and the thread state.
func briefCommentWriteDryRun(would string, cfg *config.Config, b briefs.Brief, threadID, actor, body string) *cliout.DryRun {
	dr := cliout.NewDryRun(would)
	if strings.TrimSpace(actor) == "" {
		dr.Lacking("--as")
	} else {
		dr.Require("actor_is_human_or_agent",
			actor == string(model.CommentRoleHuman) || actor == string(model.CommentRoleAgent),
			fmt.Sprintf("--as is %q", actor))
	}
	if strings.TrimSpace(body) == "" {
		dr.Lacking("--body")
	} else {
		ok := loader.CommentBodyRoundTrips(body)
		dr.Require("body_is_storable", ok, boolDetail(ok,
			"this body survives a YAML round trip byte-exact",
			"a body that cannot be stored and read back byte-exact through YAML is refused: start it with a non-whitespace character"))
	}
	if store, err := digest.LoadStore(digest.StorePath(cfg)); err != nil {
		dr.Require("comment_digest_store_readable", false, err.Error())
	} else {
		recorded, known := store.BriefDigest(b.ID)
		match := !known || recorded == digest.BriefCommentsDigest(b.ID, b.Comments)
		dr.Require("comment_block_matches_digest", match,
			fmt.Sprintf("a digest for this brief is recorded=%v; the stored comments block still matches it=%v", known, match))
	}
	if threadID != "" {
		found, open := false, false
		for _, th := range b.Comments {
			if th.ID == threadID {
				found, open = true, th.Status != model.CommentStatusResolved
				break
			}
		}
		dr.Require("thread_exists", found, boolDetail(found, "thread "+threadID+" is on this brief", "this brief has no thread "+threadID))
		dr.Require("thread_is_open", open, boolDetail(open, "thread "+threadID+" is open and can take a reply", "a resolved thread cannot take new replies"))
	}
	dr.Effect("rewrites the comments block of " + b.Path + "; no other byte of the brief changes, and its lock hash does not move")
	if threadID == "" {
		dr.Effect("an open thread refuses brief lock and brief reaudit (comment_open) until the human resolves it")
	}
	dr.Effect("the change is invisible in the viewer until \"dossierx check\" or \"dossierx serve\" re-renders it")
	dr.Propose("actor", actor).Propose("body", body)
	return dr
}

// writeOpenBriefComments prints one line per brief with an open thread, by
// path, after the modules' lines.
func writeOpenBriefComments(out io.Writer, counts map[string]int) {
	paths := make([]string, 0, len(counts))
	for p := range counts {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		fmt.Fprintf(out, "open comments: brief %q: %d\n", p, counts[p])
	}
}
