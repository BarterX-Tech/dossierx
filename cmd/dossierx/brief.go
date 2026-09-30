// brief.go is the "dossierx brief" noun (NIT-204): the read side of briefs.
//
// A brief is a markdown document beside the claims — briefs/<folder>/<slug>.md
// — for the prose a project needs that is not a claim. internal/briefs owns the
// shape, the frontmatter and the caps; this file only answers two questions
// about the tree as it stands: which briefs are there (list), and what does
// one say (show).
//
// BOTH LEAVES ARE QUERIES. Nothing here writes a file, takes a sentinel or
// records anything, and neither leaf refuses a brief for breaking a rule — a
// brief with a frontmatter finding is still listed and still shown, because
// the author needs to read the file they wrote beside the rule it broke.
// `check` is where a brief's findings fail a run.
//
// REVIEW STATE IS IN THE ENVELOPE AND EMPTY. review_pending and
// review_pending_trigger are on every entry so the shape does not move when
// NIT-205 gives briefs a lock store, content drift and dependency drift; until
// then nothing can be pending, review_pending is false everywhere and `brief
// list --review-pending` returns no brief. A consumer can branch on the field
// today and be right on the day it fills.
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
)

// newBriefCmd is the "dossierx brief" command group: list and show.
func newBriefCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "brief",
		Short: "Read the project's briefs — markdown documents beside the claims: list them, or show one",
	}
	cmd.AddCommand(
		newBriefListCmd(),
		newBriefShowCmd(),
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
	ReviewPending        bool   `json:"review_pending"`
	ReviewPendingTrigger string `json:"review_pending_trigger"`
}

// briefListData is "dossierx brief list"'s machine payload. total is every
// brief in the project and count is how many this call listed, so a filtered
// call that listed none is told apart from a project with none.
type briefListData struct {
	Count             int              `json:"count"`
	Total             int              `json:"total"`
	ReviewPendingOnly bool             `json:"review_pending_only"`
	Briefs            []briefListEntry `json:"briefs"`
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
}

// loadBriefs is the shared setup: the config (which refuses a legacy layout
// like every verb) and the briefs tree. The claims are not loaded: neither leaf
// reports a finding, and a brief must stay readable while a claim file is
// broken.
func loadBriefs() (*briefs.Set, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}
	return briefs.Load(cfg), nil
}

func newBriefListCmd() *cobra.Command {
	var reviewPending bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List every brief with its path, summary, status and review state; --review-pending lists only the briefs awaiting review",
		Args:  cobra.NoArgs,
		RunE: envelopeRunE(func(cmd *cobra.Command, args []string) (cmdResult, error) {
			set, err := loadBriefs()
			if err != nil {
				return cmdResult{}, err
			}
			entries := make([]briefListEntry, 0, len(set.Briefs))
			for _, b := range set.Briefs {
				e := briefListEntry{
					ID:      b.ID,
					Path:    b.Path,
					Folder:  b.Folder,
					Title:   b.Title,
					Summary: b.Summary,
					Status:  string(b.Status),
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
			}
			return cmdResult{
				Data: data,
				Text: func() { writeBriefListText(cmd, data) },
			}, nil
		}),
	}
	cmd.Flags().BoolVar(&reviewPending, "review-pending", false, "list only briefs awaiting review (none can be until briefs have a lock store)")
	return cmd
}

// writeBriefListText prints one greppable line per brief, path first, because
// the path is what a caller copies into "brief show".
func writeBriefListText(cmd *cobra.Command, d briefListData) {
	out := cmd.OutOrStdout()
	for _, e := range d.Briefs {
		fmt.Fprintf(out, "%s %s%s — %s\n", e.Path, e.Status, reviewPendingSuffix(e.ReviewPending), e.Summary)
	}
	switch {
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
			set, err := loadBriefs()
			if err != nil {
				return cmdResult{}, err
			}
			b, ok := set.Lookup(args[0])
			if !ok {
				return cmdResult{}, cliout.Errorf(cliout.CodeBriefNotFound, "brief show: no brief at %q", args[0]).
					WithHint("run: dossierx brief list — and pass the path it prints (briefs/<folder>/<slug>.md) or the <folder>.<slug> id")
			}
			restsOn := b.RestsOn
			if restsOn == nil {
				restsOn = []string{}
			}
			images := b.Images
			if images == nil {
				images = []briefs.Image{}
			}
			data := briefShowData{
				ID:      b.ID,
				Path:    b.Path,
				Folder:  b.Folder,
				Title:   b.Title,
				Summary: b.Summary,
				Status:  string(b.Status),
				RestsOn: restsOn,
				Digest:  b.Digest,
				Content: b.Content,
				Words:   b.Words,
				Images:  images,
			}
			return cmdResult{
				Data: data,
				Text: func() { writeBriefShowText(cmd, data) },
			}, nil
		}),
	}
}

// writeBriefShowText prints the brief for a human reading the terminal: a short
// header, then the file as written. The JSON envelope is the contract.
func writeBriefShowText(cmd *cobra.Command, d briefShowData) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "brief show: %s (%s)\n", d.Path, d.Title)
	fmt.Fprintf(out, "  status:   %s%s\n", d.Status, reviewPendingSuffix(d.ReviewPending))
	fmt.Fprintf(out, "  digest:   %s\n", d.Digest)
	fmt.Fprintf(out, "  rests_on: %s\n", joinOrNone(d.RestsOn))
	fmt.Fprintf(out, "  words:    %d\n", d.Words)
	fmt.Fprintln(out)
	fmt.Fprint(out, d.Content)
	if !strings.HasSuffix(d.Content, "\n") {
		fmt.Fprintln(out)
	}
}
