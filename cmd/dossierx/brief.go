// brief.go is the "dossierx brief" noun's read side (NIT-204, NIT-205):
// brief list and brief show. The write side — lock, unlock, reaudit — is
// brief_lock.go.
//
// A brief is a markdown document beside the claims — briefs/<folder>/<slug>.md
// — for the prose a project needs that is not a claim. internal/briefs owns the
// shape, the frontmatter, the caps and the lock lifecycle; this file answers two
// questions about the tree as it stands: which briefs are there, in which
// lock and review state (list), and what does one say (show).
//
// BOTH LEAVES ARE QUERIES. Nothing here writes a file, takes a sentinel or
// records anything, and neither leaf refuses a brief for breaking a rule — a
// brief with a frontmatter finding is still listed and still shown, because
// the author needs to read the file they wrote beside the rule it broke.
// `check` is where a brief's findings fail a run.
//
// BUT NEITHER LEAF HIDES A FINDING. Both carry `findings` — brief findings
// (shape, frontmatter, caps; the two rests_on rules need the claims, which
// these leaves do not load) in check's lint_findings shape, with the path as
// claim_id: list the tree's, show those on its own path or a folder above it —
// beside what they read. It is how manifest show
// reports a manifest's defects: the answer is still given, ok, and the defect
// is in it. What matters most is the case where the answer is incomplete: an
// unreadable folder is a finding and the other folders are still listed, and a
// tree that could not be read at all is a finding, never "this project holds no
// briefs". A consumer that sees findings non-empty has not seen every brief,
// and the text form says so.
//
// REVIEW STATE (NIT-205) is read against the lock store and the claims:
// lock_state (draft / locked / edited / unrecorded), review_pending and its
// trigger, and — on show — the changed rests_on claims and one line per
// rests_on claim with its status. The claims are needed for review state
// alone: when they do not load, list and show still answer, with the review
// state they could not compute left false and an envelope warning saying so,
// because a brief must stay readable while a claim file is broken. `brief list
// --review-pending` is the one call that cannot answer without them, and
// refuses with the claims' own load error.
//
// WHAT show DOES NOT DERIVE. No specified/built status, no conformance, no
// readiness: a brief rests on claims, and whether those claims are implemented
// is the claims' question, answered by `claim show`. Deriving a verdict for the
// brief from them would let brief state stand in for claim state, which is the
// coupling the whole design refuses.
package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/BarterX-Tech/dossierx/internal/briefs"
	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/lint"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// newBriefCmd is the "dossierx brief" command group: list and show.
func newBriefCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "brief",
		Short: "The project's briefs — markdown documents beside the claims: list, show, lock, unlock, reaudit",
	}
	cmd.AddCommand(
		newBriefListCmd(),
		newBriefShowCmd(),
		newBriefLockCmd(),
		newBriefUnlockCmd(),
		newBriefReauditCmd(),
	)
	return commandGroup(cmd)
}

// briefListEntry is one brief as `brief list` reports it: enough to choose one
// and to see its state, never its content.
type briefListEntry struct {
	ID                   string `json:"id"`
	Path                 string `json:"path"`
	Folder               string `json:"folder"`
	Title                string `json:"title"`
	Summary              string `json:"summary"`
	Status               string `json:"status"`
	LockState            string `json:"lock_state"`
	ReviewPending        bool   `json:"review_pending"`
	ReviewPendingTrigger string `json:"review_pending_trigger"`
	OpenThreads          int    `json:"open_threads"`
}

// briefListData is "dossierx brief list"'s machine payload. total is every
// brief in the project and count is how many this call listed, so a filtered
// call that listed none is told apart from a project with none.
type briefListData struct {
	Count             int               `json:"count"`
	Total             int               `json:"total"`
	ReviewPendingOnly bool              `json:"review_pending_only"`
	Briefs            []briefListEntry  `json:"briefs"`
	Findings          []lintFindingData `json:"findings"`
}

// briefShowData is "dossierx brief show"'s machine payload: the brief's whole
// content with its digest and status, and the counts its caps are measured in.
type briefShowData struct {
	ID                   string         `json:"id"`
	Path                 string         `json:"path"`
	Folder               string         `json:"folder"`
	Title                string         `json:"title"`
	Summary              string         `json:"summary"`
	Status               string         `json:"status"`
	RestsOn              []string       `json:"rests_on"`
	Digest               string         `json:"digest"`
	Content              string         `json:"content"`
	Words                int            `json:"words"`
	Images               []briefs.Image `json:"images"`
	ReviewPending        bool           `json:"review_pending"`
	ReviewPendingTrigger string         `json:"review_pending_trigger"`
	// ContentHash is the brief's lock hash (summary, rests_on, body) — what
	// `brief lock` signs; Digest is the whole file's. Review is the lock and
	// review state (NIT-205), flattened; RestsOnClaims is one entry per
	// rests_on claim with its status.
	ContentHash   string                `json:"content_hash"`
	LockState     string                `json:"lock_state"`
	LockedAt      string                `json:"locked_at"`
	LockReason    string                `json:"lock_reason"`
	OpenThreads   int                   `json:"open_threads"`
	ChangedClaims []briefs.ChangedClaim `json:"changed_claims"`
	RestsOnClaims []briefRestsOnClaim   `json:"rests_on_claims"`
	// Findings is the tree findings about this brief: on its path, or on its
	// folder or the tree (a cap, an unreadable entry), whose claim_id is a
	// directory path ending in "/".
	Findings []lintFindingData `json:"findings"`
}

// briefRestsOnClaim is one rests_on claim as `brief show` reports it: whether
// it exists, and its lock state. Nothing is derived from it for the brief.
type briefRestsOnClaim struct {
	ID            string `json:"id"`
	Exists        bool   `json:"exists"`
	Status        string `json:"status"`
	ReviewPending bool   `json:"review_pending"`
}

// briefReviewInputs loads what review state needs — the claims and the lock
// store — without refusing when either cannot be read: the brief is still
// listed and shown, and the returned warning says what was not computed.
func briefReviewInputs(cfg *config.Config, set *briefs.Set) (*briefs.Evaluation, []model.Claim, []string, error) {
	var warnings []string
	claims, claimsErr := loadClaims(cfg)
	store, storeErr := lock.LoadStore(storePath(cfg))
	if storeErr != nil {
		warnings = append(warnings, fmt.Sprintf("the lock store could not be read (%v); every locked brief reads as unrecorded until it is restored", storeErr))
		store = nil
	}
	if claimsErr != nil {
		warnings = append(warnings, fmt.Sprintf("review state was not computed: the claims did not load (%v)", claimsErr))
		return briefs.EvaluateLocks(set, store), nil, warnings, claimsErr
	}
	return briefs.Evaluate(set, claims, store), claims, warnings, nil
}

// briefFindingsData projects brief findings into check's lint_findings shape,
// never null.
func briefFindingsData(in []lint.Finding) []lintFindingData {
	out := make([]lintFindingData, 0, len(in))
	for _, f := range in {
		out = append(out, lintFindingData{Lint: f.LintName, ClaimID: f.ClaimID, Severity: string(f.Severity), Message: f.Message})
	}
	return out
}

// findingsAbout keeps the findings on path itself or on a directory holding it.
func findingsAbout(in []lint.Finding, path string) []lint.Finding {
	var out []lint.Finding
	for _, f := range in {
		if f.ClaimID == path || (strings.HasSuffix(f.ClaimID, "/") && strings.HasPrefix(path, f.ClaimID)) {
			out = append(out, f)
		}
	}
	return out
}

// writeBriefFindingsText prints one line per finding, rule and path first.
func writeBriefFindingsText(cmd *cobra.Command, fs []lintFindingData) {
	out := cmd.OutOrStdout()
	for _, f := range fs {
		fmt.Fprintf(out, "  %s %s %s: %s\n", f.Severity, f.Lint, f.ClaimID, f.Message)
	}
}

func newBriefListCmd() *cobra.Command {
	var reviewPending bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List every brief with its path, summary, status and review state; --review-pending lists only the briefs awaiting review",
		Args:  cobra.NoArgs,
		RunE: envelopeRunE(func(cmd *cobra.Command, args []string) (cmdResult, error) {
			cfg, err := loadConfig()
			if err != nil {
				return cmdResult{}, err
			}
			set := briefs.Load(cfg)
			eval, _, warnings, claimsErr := briefReviewInputs(cfg, set)
			if reviewPending && claimsErr != nil {
				// "none is review_pending" is not an answer this call can give
				// without the claims the baselines are compared against.
				return cmdResult{}, claimsErr
			}
			entries := make([]briefListEntry, 0, len(set.Briefs))
			for _, b := range set.Briefs {
				r := eval.Review(b)
				e := briefListEntry{
					ID:                   b.ID,
					Path:                 b.Path,
					Folder:               b.Folder,
					Title:                b.Title,
					Summary:              b.Summary,
					Status:               string(b.Status),
					LockState:            string(r.LockState),
					ReviewPending:        r.ReviewPending,
					ReviewPendingTrigger: r.ReviewPendingTrigger,
					OpenThreads:          r.OpenThreads,
				}
				if reviewPending && !e.ReviewPending {
					continue
				}
				entries = append(entries, e)
			}
			data := briefListData{
				Count:             len(entries),
				Total:             len(set.Briefs),
				ReviewPendingOnly: reviewPending,
				Briefs:            entries,
				Findings:          briefFindingsData(set.TreeFindings()),
			}
			return cmdResult{
				Warnings: warnings,
				Data:     data,
				Text:     func() { writeBriefListText(cmd, data) },
			}, nil
		}),
	}
	cmd.Flags().BoolVar(&reviewPending, "review-pending", false, "list only locked briefs whose rests_on claims moved since their baseline")
	return cmd
}

// writeBriefListText prints one greppable line per brief, path first, because
// the path is what a caller copies into "brief show".
func writeBriefListText(cmd *cobra.Command, d briefListData) {
	out := cmd.OutOrStdout()
	for _, e := range d.Briefs {
		fmt.Fprintf(out, "%s %s%s — %s\n", e.Path, e.LockState, reviewPendingSuffix(e.ReviewPending), e.Summary)
	}
	if len(d.Findings) > 0 {
		fmt.Fprintf(out, "brief list: %d finding(s) in the briefs tree; an entry that could not be read lists no brief:\n", len(d.Findings))
		writeBriefFindingsText(cmd, d.Findings)
	}
	switch {
	case d.Total == 0 && len(d.Findings) > 0:
		fmt.Fprintln(out, "brief list: no brief listed, and the tree has findings; that is not a project with no briefs")
	case d.Total == 0:
		fmt.Fprintln(out, "brief list: this project holds no briefs")
	case d.ReviewPendingOnly:
		fmt.Fprintf(out, "brief list: %d of %d brief(s) awaiting review\n", d.Count, d.Total)
	default:
		fmt.Fprintf(out, "brief list: %d brief(s)\n", d.Count)
	}
}

// reviewPendingSuffix annotates a line with its review state. A suffix rather
// than a column, because it is the exception.
func reviewPendingSuffix(pending bool) string {
	if pending {
		return " review_pending"
	}
	return ""
}

func newBriefShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <path>",
		Short: "Show one brief — its content, digest and status — by the path brief list prints or by its <folder>.<slug> id",
		Args:  cobra.ExactArgs(1),
		RunE: envelopeRunE(func(cmd *cobra.Command, args []string) (cmdResult, error) {
			cfg, err := loadConfig()
			if err != nil {
				return cmdResult{}, err
			}
			set := briefs.Load(cfg)
			treeFindings := set.TreeFindings()
			b, ok := set.Lookup(args[0])
			if !ok {
				// The path shape is briefs_dir as the config names it (the
				// Set's DisplayDir), the same spelling brief list prints.
				listHint := fmt.Sprintf("run: dossierx brief list — and pass the path it prints (%s/<folder>/<slug>.md) or the <folder>.<slug> id", set.DisplayDir)
				err := cliout.Errorf(cliout.CodeBriefNotFound, "brief show: no brief at %q", args[0]).
					WithHint(listHint)
				if len(treeFindings) > 0 {
					// Not found in a tree that has findings is not "absent": the
					// brief may sit in an entry that could not be read.
					err = err.WithHint(fmt.Sprintf("the briefs tree has %d finding(s), in details.findings; a brief in an entry that could not be read is not found. %s", len(treeFindings), listHint)).
						WithDetails(map[string]any{"findings": briefFindingsData(treeFindings)})
				}
				return cmdResult{}, err
			}
			restsOn := b.RestsOn
			if restsOn == nil {
				restsOn = []string{}
			}
			images := b.Images
			if images == nil {
				images = []briefs.Image{}
			}
			eval, claims, warnings, claimsErr := briefReviewInputs(cfg, set)
			r := eval.Review(b)
			restsOnClaims := make([]briefRestsOnClaim, 0, len(b.RestsOn))
			for _, id := range b.RestsOn {
				entry := briefRestsOnClaim{ID: id}
				switch c, ok := loader.FindByID(claims, id); {
				case claimsErr != nil:
					// The claims did not load: whether this one exists is
					// unknown, not false (the warning says why).
					entry.Status = "unknown"
				case ok:
					entry.Exists, entry.Status, entry.ReviewPending = true, string(c.Status), c.ReviewPending
				}
				restsOnClaims = append(restsOnClaims, entry)
			}
			data := briefShowData{
				ID:       b.ID,
				Path:     b.Path,
				Folder:   b.Folder,
				Title:    b.Title,
				Summary:  b.Summary,
				Status:   string(b.Status),
				RestsOn:  restsOn,
				Digest:   b.Digest,
				Content:  b.Content,
				Words:    b.Words,
				Images:   images,
				Findings: briefFindingsData(findingsAbout(treeFindings, b.Path)),

				ReviewPending:        r.ReviewPending,
				ReviewPendingTrigger: r.ReviewPendingTrigger,
				ContentHash:          b.LockHash,
				LockState:            string(r.LockState),
				LockedAt:             r.LockedAt,
				LockReason:           r.LockReason,
				OpenThreads:          r.OpenThreads,
				ChangedClaims:        r.ChangedClaims,
				RestsOnClaims:        restsOnClaims,
			}
			return cmdResult{
				Warnings: warnings,
				Data:     data,
				Text:     func() { writeBriefShowText(cmd, data) },
			}, nil
		}),
	}
}

// writeBriefShowText prints the brief for a human reading the terminal: a short
// header, then the file as written. The JSON envelope is the contract.
func writeBriefShowText(cmd *cobra.Command, d briefShowData) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "brief show: %s (%s)\n", d.Path, d.Title)
	fmt.Fprintf(out, "  status:   %s (%s)%s\n", d.Status, d.LockState, reviewPendingSuffix(d.ReviewPending))
	if d.LockedAt != "" {
		fmt.Fprintf(out, "  locked:   %s (%q)\n", d.LockedAt, d.LockReason)
	}
	fmt.Fprintf(out, "  digest:   %s\n", d.Digest)
	fmt.Fprintf(out, "  rests_on: %s\n", joinOrNone(d.RestsOn))
	for _, c := range d.RestsOnClaims {
		state := "not a claim"
		if c.Exists {
			state = c.Status + reviewPendingSuffix(c.ReviewPending)
		}
		fmt.Fprintf(out, "    %s: %s\n", c.ID, state)
	}
	for _, c := range d.ChangedClaims {
		fmt.Fprintf(out, "  changed:  %s\n", c.ID)
	}
	if d.OpenThreads > 0 {
		fmt.Fprintf(out, "  threads:  %d open\n", d.OpenThreads)
	}
	fmt.Fprintf(out, "  words:    %d\n", d.Words)
	if len(d.Findings) > 0 {
		fmt.Fprintf(out, "  findings: %d\n", len(d.Findings))
		writeBriefFindingsText(cmd, d.Findings)
	}
	fmt.Fprintln(out)
	fmt.Fprint(out, d.Content)
	if !strings.HasSuffix(d.Content, "\n") {
		fmt.Fprintln(out)
	}
}
