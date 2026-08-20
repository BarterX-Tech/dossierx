// track.go is the "dossierx track" noun: the read-only half of the second axis
// the corpus is organized on.
//
// The first axis is the module, and it answers "who guarantees this?" — one
// module per claim, which is the right shape for writing and reviewing
// contracts and the shape every other verb in this CLI is built around. It
// cannot answer "what does the user get, and is it finished?", because a
// user-facing feature is assembled from claims spread across many modules and a
// module serves many features. See internal/model/track.go for why that made a
// second axis necessary rather than merely convenient, and why membership is a
// set rather than an edge.
//
// EVERY LEAF HERE IS A QUERY. Nothing under this noun writes a file, takes a
// write sentinel, or changes what the project treats as approved, and that is a
// design constraint rather than an accident of what has been implemented so far:
//
//   - "track status" must never gate "claim lock". A track is a cross-cutting
//     concern and a module review is not; making a claim's promotion depend on
//     the completeness of a feature it merely participates in would invert the
//     dependency the whole model rests on — the module owns the claim, the track
//     only references it — and would hand every reviewer of one module a veto
//     sourced from a document they were not reading.
//   - A half-finished track is a NORMAL state, not a defect, so no leaf here
//     participates in "check" and none of them can fail a gate. A feature under
//     construction is what a feature under construction looks like; reporting it
//     as a finding would train readers to wave findings through, which is the one
//     thing a gate cannot afford.
//
// What that buys is that "is this feature done?" is a question an agent can ask
// at any moment, about a tree in any state, without the asking changing
// anything.
package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// newTrackCmd is the "dossierx track" command group: list, show, and status.
func newTrackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "track",
		Short: "Read the cross-cutting feature axis: what each track covers, the document it assembles, and whether it is finished",
	}
	cmd.AddCommand(
		newTrackListCmd(),
		newTrackShowCmd(),
		newTrackStatusCmd(),
	)
	return commandGroup(cmd)
}

// ---------------------------------------------------------------------
// shared: membership, and what "complete" means
// ---------------------------------------------------------------------

// trackMembers partitions the claim set into the claims that OWN track id and
// the claims that merely CITE it, each sorted by claim id.
//
// Ownership is read through model.Claim.OwnedTrackID rather than by inspecting
// the membership's own role, and the difference only shows up on a malformed
// claim — one declaring `owns` on two tracks, which the track-multi-owner lint
// refuses. Reading the accessor means the CLI upholds "at most one track may
// call a claim its own" even while that defect stands: the claim appears owned
// in the first track's document and cited in the second, rather than owned in
// both, which is a state the model says cannot exist. The lint reports the
// defect; this renders deterministically in the meantime, which is exactly the
// division OwnedTrackID's own doc comment describes.
//
// Sorting is by claim id and nothing else. A track has no authored ordering
// field — see config.Track, which carries an id, a title and a summary — so
// there is no author's sequence to honour, and the property that actually
// matters is that the same tree prints the same bytes on every run: a document
// a human is asked to read, or an agent to diff, must not shuffle.
func trackMembers(claims []model.Claim, id string) (owned, cited []model.Claim) {
	for _, c := range claims {
		if !c.InTrack(id) {
			continue
		}
		if c.OwnedTrackID() == id {
			owned = append(owned, c)
			continue
		}
		cited = append(cited, c)
	}
	sort.Slice(owned, func(i, j int) bool { return owned[i].ID < owned[j].ID })
	sort.Slice(cited, func(i, j int) bool { return cited[i].ID < cited[j].ID })
	return owned, cited
}

// trackComplete answers "is this feature done?" from the owned and cited sets,
// and returns the locked count alongside it so no caller has to re-walk them.
//
// The rule is that every claim the track owns AND every claim it cites is
// locked. Citing counts because it is what makes a track a document rather than
// a label: the track's claim is that these sentences TOGETHER describe a
// finished feature, and one of them still being editable makes that claim false
// no matter which module happens to hold it.
//
// An EMPTY track is deliberately NOT complete, and this is the one place the
// answer is not a plain reading of the rule — vacuously, a track with no claims
// has every claim locked. It is reported incomplete because of what the question
// is: a reader asking "is checkout done?" about a track declared this morning
// and joined by nothing would be told yes, and the direction that error runs in
// is shipping. The counts beside the verdict say which case it is.
//
// review_pending is reported everywhere here and blocks nothing. A locked claim
// with a question open on it is still locked, so it does not change the verdict;
// carrying the flag into the payload is what lets a reader see the difference
// without a second call per claim.
func trackComplete(owned, cited []model.Claim) (complete bool, locked int) {
	for _, c := range append(append([]model.Claim(nil), owned...), cited...) {
		if c.Status == model.StatusLocked {
			locked++
		}
	}
	total := len(owned) + len(cited)
	return total > 0 && locked == total, locked
}

// trackClaimView is one claim's appearance inside a track.
//
// Body is populated for OWNED claims only, and its presence is the whole
// difference between the two halves of a track's document. An owned claim's
// body is the track's own prose — the feature-level sentence that belongs to no
// single module — so a "document" that omitted it would be a table of contents,
// and reading the actual feature would cost one "claim show" per row. A cited
// claim's body is deliberately absent for the opposite reason: citing is a
// reference and never a copy (see model.TrackRoleCites), and reproducing the
// text here would put a second copy of a sentence its own module guarantees into
// a document nothing hashes.
type trackClaimView struct {
	ClaimID       string `json:"claim_id"`
	Title         string `json:"title"`
	Facet         string `json:"facet"`
	Module        string `json:"module"`
	Status        string `json:"status"`
	Locked        bool   `json:"locked"`
	ReviewPending bool   `json:"review_pending"`
	Body          string `json:"body,omitempty"`
}

// trackClaimViews projects claims into the payload shape. withBody is false for
// the cited half — see trackClaimView.Body.
func trackClaimViews(claims []model.Claim, withBody bool) []trackClaimView {
	out := make([]trackClaimView, 0, len(claims))
	for _, c := range claims {
		v := trackClaimView{
			ClaimID:       c.ID,
			Title:         claimTitle(c.ID),
			Facet:         c.Facet,
			Module:        c.Module,
			Status:        string(c.Status),
			Locked:        c.Status == model.StatusLocked,
			ReviewPending: c.ReviewPending,
		}
		if withBody {
			v.Body = c.Body
		}
		out = append(out, v)
	}
	return out
}

// ---------------------------------------------------------------------
// track list
// ---------------------------------------------------------------------

// trackListEntry is one declared track with its roll-up. The counts are carried
// rather than the claim ids: the ids are what "track show" is for, and a caller
// asking "which features are there, and how far along" wants the shape of the
// whole registry, not every claim in it.
type trackListEntry struct {
	TrackID      string `json:"track_id"`
	Title        string `json:"title"`
	Summary      string `json:"summary,omitempty"`
	OwnedClaims  int    `json:"owned_claims"`
	CitedClaims  int    `json:"cited_claims"`
	TotalClaims  int    `json:"total_claims"`
	LockedClaims int    `json:"locked_claims"`
	Complete     bool   `json:"complete"`
}

// trackListData is "dossierx track list"'s machine payload.
//
// A project that declares no tracks answers count:0 with an empty list at exit
// 0, and that is a success rather than a refusal: the second axis is additive,
// and a corpus that never adopted it must behave exactly as it did before the
// axis existed. Nothing here is reachable by a project that does not opt in.
type trackListData struct {
	Count  int              `json:"count"`
	Tracks []trackListEntry `json:"tracks"`
}

func newTrackListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List every declared track with its claim counts and whether it is complete",
		Args:  cobra.NoArgs,
		RunE: envelopeRunE(func(cmd *cobra.Command, args []string) (cmdResult, error) {
			cfg, claims, err := loadConfigAndClaims()
			if err != nil {
				return cmdResult{}, err
			}

			// DECLARATION ORDER, not alphabetical. The registry is a hand-written
			// list in project.config.yaml and its order is the author's own
			// grouping of the roadmap; re-sorting it would discard information
			// the config carries and replace it with none. It is equally
			// deterministic either way — config.TrackIDs reads the slice — so the
			// tie-break the other list commands need (see claim list) does not
			// arise: two tracks cannot share an id.
			entries := make([]trackListEntry, 0, len(cfg.Tracks))
			for _, t := range cfg.Tracks {
				owned, cited := trackMembers(claims, t.ID)
				complete, locked := trackComplete(owned, cited)
				entries = append(entries, trackListEntry{
					TrackID:      t.ID,
					Title:        t.Title,
					Summary:      t.Summary,
					OwnedClaims:  len(owned),
					CitedClaims:  len(cited),
					TotalClaims:  len(owned) + len(cited),
					LockedClaims: locked,
					Complete:     complete,
				})
			}

			data := trackListData{Count: len(entries), Tracks: entries}
			return cmdResult{
				Data: data,
				Text: func() { writeTrackListText(cmd, data) },
			}, nil
		}),
	}
}

// writeTrackListText renders the registry as one greppable line per track. The
// id leads, because it is what a caller copies into "track show".
func writeTrackListText(cmd *cobra.Command, d trackListData) {
	out := cmd.OutOrStdout()
	for _, e := range d.Tracks {
		state := "in progress"
		if e.Complete {
			state = "complete"
		}
		fmt.Fprintf(out, "%s %s (%d/%d locked, %d owned, %d cited) — %s\n",
			e.TrackID, state, e.LockedClaims, e.TotalClaims, e.OwnedClaims, e.CitedClaims, e.Title)
	}
	// The empty case says so in words. "track list: 0 track(s)" alone reads as
	// though the command failed to find something it was looking for, where the
	// truth is that this project has not adopted the axis at all.
	if d.Count == 0 {
		fmt.Fprintln(out, "track list: this project declares no tracks")
		return
	}
	fmt.Fprintf(out, "track list: %d track(s)\n", d.Count)
}

// ---------------------------------------------------------------------
// track show <id>
// ---------------------------------------------------------------------

// trackShowData is "dossierx track show"'s machine payload: the assembled
// document.
//
// It is the one place in the CLI where a feature reads as a whole. Owned claims
// arrive with their bodies, in id order, and are the document's prose; cited
// claims arrive as references carrying the two things a reader needs to go and
// find them — the module and facet that guarantee them — plus their lock state,
// because a cited claim that is still draft is a sentence this document leans on
// that anybody may still change.
type trackShowData struct {
	TrackID      string           `json:"track_id"`
	Title        string           `json:"title"`
	Summary      string           `json:"summary,omitempty"`
	Complete     bool             `json:"complete"`
	TotalClaims  int              `json:"total_claims"`
	LockedClaims int              `json:"locked_claims"`
	OwnedClaims  []trackClaimView `json:"owned_claims"`
	CitedClaims  []trackClaimView `json:"cited_claims"`
}

func newTrackShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Assemble one track's document: its owned claims in full, and the claims it cites with their owning module, facet, and lock state",
		Args:  cobra.ExactArgs(1),
		RunE: envelopeRunE(func(cmd *cobra.Command, args []string) (cmdResult, error) {
			id := args[0]
			cfg, claims, err := loadConfigAndClaims()
			if err != nil {
				return cmdResult{}, err
			}
			track, err := resolveTrack(cfg, id, "track show")
			if err != nil {
				return cmdResult{}, err
			}

			owned, cited := trackMembers(claims, track.ID)
			complete, locked := trackComplete(owned, cited)
			data := trackShowData{
				TrackID:      track.ID,
				Title:        track.Title,
				Summary:      track.Summary,
				Complete:     complete,
				TotalClaims:  len(owned) + len(cited),
				LockedClaims: locked,
				OwnedClaims:  trackClaimViews(owned, true),
				CitedClaims:  trackClaimViews(cited, false),
			}

			return cmdResult{
				Data: data,
				Text: func() { writeTrackShowText(cmd, data) },
			}, nil
		}),
	}
}

// writeTrackShowText renders the assembled document for a human reading the
// terminal — the courtesy surface. The JSON envelope is the contract.
func writeTrackShowText(cmd *cobra.Command, d trackShowData) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "track show: %s (%s)\n", d.TrackID, d.Title)
	if d.Summary != "" {
		fmt.Fprintf(out, "  %s\n", d.Summary)
	}
	state := "in progress"
	if d.Complete {
		state = "complete"
	}
	fmt.Fprintf(out, "  state:              %s (%d of %d claim(s) locked)\n", state, d.LockedClaims, d.TotalClaims)

	fmt.Fprintln(out, "  owns:")
	if len(d.OwnedClaims) == 0 {
		fmt.Fprintln(out, "    (no claim owns this track; its feature-level sentences have no home in the corpus yet)")
	}
	for _, c := range d.OwnedClaims {
		fmt.Fprintf(out, "    %s %s (%s)%s\n", c.Status, c.ClaimID, c.Title, reviewPendingSuffix(c.ReviewPending))
		for _, line := range strings.Split(strings.TrimRight(c.Body, "\n"), "\n") {
			fmt.Fprintf(out, "      %s\n", line)
		}
	}

	fmt.Fprintln(out, "  cites:")
	if len(d.CitedClaims) == 0 {
		fmt.Fprintln(out, "    (nothing)")
	}
	for _, c := range d.CitedClaims {
		fmt.Fprintf(out, "    %s %s [%s / %s] (%s)%s\n", c.Status, c.ClaimID, c.Module, c.Facet, c.Title, reviewPendingSuffix(c.ReviewPending))
	}
}

// reviewPendingSuffix annotates a claim line with its review state. It is a
// suffix rather than a column because it is the exception: most claims are not
// review_pending, and a blank column on every other line is noise a reader
// learns to skim.
func reviewPendingSuffix(pending bool) string {
	if pending {
		return " review_pending"
	}
	return ""
}

// ---------------------------------------------------------------------
// track status <id>
// ---------------------------------------------------------------------

// trackCountsView is one role's tally within a track.
type trackCountsView struct {
	Total  int `json:"total"`
	Locked int `json:"locked"`
}

// trackBlockerView is one claim standing between a track and completion: it is
// in the track, in either role, and not locked.
//
// The owning module and facet are carried because they are the recovery. A
// track's document cannot lock anything — locking is a module review, and it is
// the module's reviewer who has to say yes — so "what do I do about this?" is
// answered by naming where the claim actually lives, not by naming the track
// again.
type trackBlockerView struct {
	ClaimID       string `json:"claim_id"`
	Role          string `json:"role"`
	Module        string `json:"module"`
	Facet         string `json:"facet"`
	Status        string `json:"status"`
	ReviewPending bool   `json:"review_pending"`
}

// trackStatusData is "dossierx track status"'s machine payload: the answer to
// "is this feature done?", and the list of what is in the way.
//
// Complete false with an EMPTY blocking list is a real and meaningful pair, and
// it means exactly one thing: no claim has joined this track yet. See
// trackComplete for why an empty track is not reported as finished. Every other
// incomplete track names its blockers.
//
// The whole payload is derived from the tree as it stands and nothing is
// written, consulted by "check", or fed to any gate — see this file's package
// comment for why that is a constraint rather than a current limitation.
type trackStatusData struct {
	TrackID     string             `json:"track_id"`
	Title       string             `json:"title"`
	Complete    bool               `json:"complete"`
	TotalClaims int                `json:"total_claims"`
	Owned       trackCountsView    `json:"owned"`
	Cited       trackCountsView    `json:"cited"`
	Blocking    []trackBlockerView `json:"blocking"`
}

func newTrackStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <id>",
		Short: "Report whether a track is complete — every claim it owns and cites locked — and name every claim that is not",
		Args:  cobra.ExactArgs(1),
		RunE: envelopeRunE(func(cmd *cobra.Command, args []string) (cmdResult, error) {
			id := args[0]
			cfg, claims, err := loadConfigAndClaims()
			if err != nil {
				return cmdResult{}, err
			}
			track, err := resolveTrack(cfg, id, "track status")
			if err != nil {
				return cmdResult{}, err
			}

			owned, cited := trackMembers(claims, track.ID)
			complete, _ := trackComplete(owned, cited)

			// Owned blockers first, then cited, each already in id order. The
			// order is the order the reader can act in: an unlocked OWNED claim is
			// the track's own sentence and is usually the caller's to finish,
			// where an unlocked cited claim belongs to somebody else's module
			// review.
			blocking := make([]trackBlockerView, 0)
			ownedCounts := trackCountsView{Total: len(owned)}
			citedCounts := trackCountsView{Total: len(cited)}
			for _, group := range []struct {
				claims []model.Claim
				role   model.TrackRole
				counts *trackCountsView
			}{
				{owned, model.TrackRoleOwns, &ownedCounts},
				{cited, model.TrackRoleCites, &citedCounts},
			} {
				for _, c := range group.claims {
					if c.Status == model.StatusLocked {
						group.counts.Locked++
						continue
					}
					blocking = append(blocking, trackBlockerView{
						ClaimID:       c.ID,
						Role:          string(group.role),
						Module:        c.Module,
						Facet:         c.Facet,
						Status:        string(c.Status),
						ReviewPending: c.ReviewPending,
					})
				}
			}

			data := trackStatusData{
				TrackID:     track.ID,
				Title:       track.Title,
				Complete:    complete,
				TotalClaims: len(owned) + len(cited),
				Owned:       ownedCounts,
				Cited:       citedCounts,
				Blocking:    blocking,
			}

			return cmdResult{
				Data: data,
				Text: func() { writeTrackStatusText(cmd, data) },
			}, nil
		}),
	}
}

func writeTrackStatusText(cmd *cobra.Command, d trackStatusData) {
	out := cmd.OutOrStdout()
	verdict := "in progress"
	if d.Complete {
		verdict = "complete"
	}
	fmt.Fprintf(out, "track status: %s (%s) — %s\n", d.TrackID, d.Title, verdict)
	fmt.Fprintf(out, "  owns:  %d of %d locked\n", d.Owned.Locked, d.Owned.Total)
	fmt.Fprintf(out, "  cites: %d of %d locked\n", d.Cited.Locked, d.Cited.Total)
	if d.TotalClaims == 0 {
		fmt.Fprintln(out, "  no claim has joined this track yet, so there is nothing to be finished")
		return
	}
	if len(d.Blocking) == 0 {
		return
	}
	fmt.Fprintln(out, "  blocking completion:")
	for _, b := range d.Blocking {
		fmt.Fprintf(out, "    %s %s (%s) [%s / %s]%s\n", b.Status, b.ClaimID, b.Role, b.Module, b.Facet, reviewPendingSuffix(b.ReviewPending))
	}
}

// ---------------------------------------------------------------------
// shared: resolving the positional id
// ---------------------------------------------------------------------

// resolveTrack turns the positional argument into a declared track, or into the
// refusal an unknown id has to be. verb prefixes the message the way every other
// command in this package prefixes its own.
func resolveTrack(cfg *config.Config, id, verb string) (config.Track, error) {
	if err := requireKnownTrack(cfg, id); err != nil {
		return config.Track{}, cliout.Errorf(cliout.CodeUnknownTrack, "%s: %w", verb, err)
	}
	track, _ := cfg.TrackByID(id)
	return track, nil
}
