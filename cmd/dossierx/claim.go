// claim.go is the "dossierx claim" noun — the eight-leaf group v0.3.0's
// restructure built by pulling four existing top-level verbs under one parent
// (lock, unlock, flag, reaudit), renaming a fifth (implink set -> claim link),
// and adding three new ones (show, list, new).
//
// The three new leaves are not conveniences. They exist because of what the
// release TOOK AWAY:
//
//   - "claim show" absorbs "deps" and "implink status", and answers in ONE call
//     what previously took three or four: state, both edge directions, code
//     links and their drift, comment counts, and — the part no old verb had at
//     all — the legal next actions.
//   - "claim list" absorbs "stale" and "coverage", which were never verbs; they
//     were filters over the claim set wearing a verb's clothes. --match is here
//     because a human says "the retry-policy card in the contract facet", and
//     resolving that to an id is the agent's first move in almost every loop.
//   - "claim new" exists because the release gates hand-editing claim YAML. An
//     agent that may not hand-write a claim file and has no sanctioned way to
//     author one cannot do the work at all, so this is the sanctioned way; it
//     writes a claim shaped to pass the lint suite on the very next validate.
//
// The lifecycle leaves keep their own files (main.go's lock/unlock/reaudit,
// flag.go, claim_link.go) and their own "<verb>:" error prefixes. Only the
// invocation path moved; the envelope's "command" field picks the new path up
// automatically from cobra (see commandPath).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/BarterX-Tech/dossierx/internal/check"
	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/comments"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/implink"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/reaudit"
)

// newClaimCmd is the "dossierx claim" command group: everything an agent does
// to one claim, from authoring it to locking it to grounding it in code.
func newClaimCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "claim",
		Short: "Author, inspect, filter, and move claims through their lifecycle",
	}
	cmd.AddCommand(
		newClaimShowCmd(),
		newClaimListCmd(),
		newClaimNewCmd(),
		newLockCmd(),
		newUnlockCmd(),
		newFlagCmd(),
		newReauditCmd(),
		newClaimLinkCmd(),
	)
	return commandGroup(cmd)
}

// ---------------------------------------------------------------------
// shared: a claim's human label, and the incoming-edge scan
// ---------------------------------------------------------------------

// claimTitle derives a claim's human-readable label from its id.
//
// There is deliberately no "title" field in the claim schema: an id is
// module.facet.slug and the slug IS the label, which is why FORMAT.md requires
// it to be kebab-case. Deriving instead of storing means the two can never
// disagree, and it is what lets --match search "what a human would call this
// card" without any project having to author a second name for everything.
//
// Anything that is not exactly three non-empty segments falls back to the raw
// id verbatim, for the same reason Phase 4's edge labels will: this runs
// outside the lint suite, so it must never assume a well-formed id.
func claimTitle(id string) string {
	segs := strings.Split(id, ".")
	if len(segs) != 3 || segs[2] == "" {
		return id
	}
	words := strings.Split(segs[2], "-")
	for i, w := range words {
		if w == "" {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

// incomingEdges returns the ids of every OTHER claim that points at id through
// mirrors and through rests_on, each sorted.
//
// internal/render has a rests_on-only reverse index (buildDependedByLookup)
// built for the viewer's "depended on by" footer. It is deliberately not reused
// here: it is unexported, in a package whose job is HTML, and covers only one of
// the two edge kinds "claim show" has to report. A linear scan over the claim
// set — the same scan the retired "deps" verb did — is cheaper than the coupling.
func incomingEdges(claims []model.Claim, id string) (mirroredBy, dependedOnBy []string) {
	for _, c := range claims {
		if c.ID == id {
			continue
		}
		if containsStr(c.Mirrors, id) {
			mirroredBy = append(mirroredBy, c.ID)
		}
		if containsStr(c.RestsOn, id) {
			dependedOnBy = append(dependedOnBy, c.ID)
		}
	}
	sort.Strings(mirroredBy)
	sort.Strings(dependedOnBy)
	return mirroredBy, dependedOnBy
}

// emptyIfNil coerces a nil string slice to an empty one so every list in a
// payload encodes as "[]" rather than "null" — a consumer must be able to range
// over edges.mirrors without first testing it for null.
func emptyIfNil(ss []string) []string {
	if ss == nil {
		return []string{}
	}
	return ss
}

// linkViewsFor returns claim's implementation links annotated with drift, or
// nil when the claim's module has never linked anything.
//
// Every failure mode here degrades to "no links": a module with no artifact is
// the ordinary case (implink.ErrNoArtifact), and a corrupt or unreadable one
// must not stop "claim show" from reporting the state it CAN see. The
// authoritative drift report is "dossierx check".
func linkViewsFor(cfg *config.Config, claim model.Claim) []claimLinkView {
	if claim.Module == "" {
		return nil
	}
	byClaim, err := implink.ViewsByClaim(cfg, claim.Module)
	if err != nil {
		return nil
	}
	views := byClaim[claim.ID]
	if len(views) == 0 {
		return nil
	}
	out := make([]claimLinkView, 0, len(views))
	for _, v := range views {
		out = append(out, claimLinkView{File: v.File, Symbol: v.Symbol, Drifted: v.Drifted})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].File < out[j].File })
	return out
}

// ---------------------------------------------------------------------
// claim show <id>
// ---------------------------------------------------------------------

// claimLinkView is one implementing file with its current drift verdict —
// implink.ViewFile in snake_case (that type carries no JSON tags).
type claimLinkView struct {
	File    string `json:"file"`
	Symbol  string `json:"symbol,omitempty"`
	Drifted bool   `json:"drifted"`
}

// claimSourceView is one cited source in snake_case — model.Source projected
// into the machine contract.
//
// It is a projection rather than the model type embedded directly, and that is
// forced rather than stylistic: model.Source is a PERSISTENCE shape carrying
// yaml tags only, so embedding it would marshal every field under its Go name
// (AccessedOn, DoesNotSupport) into an envelope whose whole grammar is
// snake_case. TestEnvelopePayloadTypesDeclareSnakeCaseJSONTags refuses that at
// declaration time for exactly this class of leak, and claimLinkView is the
// existing precedent for the same fix.
//
// Every authored field is carried, including the anchors. A caller looking at a
// citation is usually deciding whether to trust it, and the two things that
// answer that — an external source's AccessedOn, an internal source's SHA256 —
// are precisely the fields a summary would drop. Supports and DoesNotSupport
// come with them because an overread citation is the common defect and
// DoesNotSupport is the author's own statement of the limit.
type claimSourceView struct {
	Ref            int    `json:"ref"`
	Kind           string `json:"kind"`
	Title          string `json:"title"`
	URL            string `json:"url,omitempty"`
	AccessedOn     string `json:"accessed_on,omitempty"`
	Path           string `json:"path,omitempty"`
	RecordID       string `json:"record_id,omitempty"`
	SHA256         string `json:"sha256,omitempty"`
	Supports       string `json:"supports,omitempty"`
	DoesNotSupport string `json:"does_not_support,omitempty"`
}

// claimSourceViews projects a claim's sources into the payload shape, in the
// order they were authored.
//
// Authored order, not sorted by Ref: Ref is author-assigned rather than derived
// from position (see model.Source.Ref), so the two can legitimately disagree,
// and the file's own order is the one a reader comparing the envelope against
// the YAML will be looking at. It is equally deterministic — a claim's Sources
// slice is loaded once from one file — and the duplicate-Ref case that would
// make a Ref sort ambiguous is a lint finding (source-shape), not something to
// resolve silently here.
func claimSourceViews(sources []model.Source) []claimSourceView {
	out := make([]claimSourceView, 0, len(sources))
	for _, s := range sources {
		out = append(out, claimSourceView{
			Ref:            s.Ref,
			Kind:           string(s.Kind),
			Title:          s.Title,
			URL:            s.URL,
			AccessedOn:     s.AccessedOn,
			Path:           s.Path,
			RecordID:       s.RecordID,
			SHA256:         s.SHA256,
			Supports:       s.Supports,
			DoesNotSupport: s.DoesNotSupport,
		})
	}
	return out
}

// claimTrackView is one of a claim's track memberships, with the role resolved.
//
// Role is the EFFECTIVE role (model.TrackRef.EffectiveRole), never the raw
// field. An omitted role means cites, and a payload that echoed the empty
// string would make a consumer re-implement that default — which is the one
// place it could be re-implemented differently from the engine.
type claimTrackView struct {
	TrackID string `json:"track_id"`
	Role    string `json:"role"`
}

// claimTrackViews projects a claim's memberships into the payload shape, in
// authored order — the same reasoning as claimSourceViews: the claim file's own
// order is what a reader comparing the two will see, and it is total.
func claimTrackViews(refs []model.TrackRef) []claimTrackView {
	out := make([]claimTrackView, 0, len(refs))
	for _, r := range refs {
		out = append(out, claimTrackView{TrackID: r.ID, Role: string(r.EffectiveRole())})
	}
	return out
}

// claimEdgesData is a claim's graph position in BOTH directions. Outgoing edges
// are authored on the claim; incoming ones are derived by scanning every other
// claim, and are the half an agent could never see without a second call.
type claimEdgesData struct {
	Mirrors        []string `json:"mirrors"`
	RestsOn        []string `json:"rests_on"`
	GovernedBy     string   `json:"governed_by"`
	GovernedReason string   `json:"governed_reason,omitempty"`
	MirroredBy     []string `json:"mirrored_by"`
	DependedOnBy   []string `json:"depended_on_by"`
}

// claimCommentCounts is the discussion roll-up. OpenThreadIDs is carried in
// full, not just counted, because an open thread is a LOCK GATE: an agent that
// knows only "there are two" still has to make another call to say anything
// useful to its human about them.
type claimCommentCounts struct {
	Threads       int      `json:"threads"`
	Open          int      `json:"open"`
	Resolved      int      `json:"resolved"`
	Replies       int      `json:"replies"`
	OpenThreadIDs []string `json:"open_thread_ids"`
}

// claimLedgerView is a claim's LOCK-LEDGER state — the half of "what is the
// state of this claim?" that claim show could not see.
//
// Without it, show and "check --validate" returned opposite verdicts about the
// same tree: show reported a tampered locked claim as locked, not
// review_pending, settled, while the gate reported lock-content-drift. An agent
// asked to orient itself on a claim reads show, and show is where next_actions
// come from — so the missing field was not cosmetic, it made show recommend
// unlock -> relock on exactly the claim for which the skills forbid it.
//
// It is omitted entirely (omitempty on a pointer) for a claim with no record at
// all: a draft claim in a project that has never locked anything has no ledger
// state to report, and emitting a block of zero values would invite a consumer
// to read "recorded: false" as a finding when it is the ordinary case. The gate
// owns the reporting of a MISSING record for a locked claim (lock-ledger-missing).
type claimLedgerView struct {
	// Recorded: a ledger record for this claim exists.
	Recorded bool `json:"recorded"`
	// Grandfathered: a LEGACY record, adopted on upgrade rather than approved.
	// Its hash is what was on disk on adoption day. Nothing in this build writes
	// one, so it is false for every record minted here and true only for records
	// surviving from a project that ran the removed adoption path; the key stays
	// (no omitempty) because a consumer must be able to tell the two apart.
	Grandfathered bool `json:"grandfathered"`
	// Released: an unlock released the record. A released record describes a
	// claim that is allowed to be draft and allowed to change.
	Released bool `json:"released"`
	// ContentMatches: the claim's current content still hashes to what the
	// STANDING record approved. It is true whenever nothing standing
	// contradicts the file (including a released record), and false only for a
	// real disagreement — see standingLedgerRecord.
	ContentMatches bool   `json:"content_matches"`
	ApprovedAt     string `json:"approved_at,omitempty"`
	ApprovedReason string `json:"approved_reason,omitempty"`
}

// claimShowData is "dossierx claim show"'s machine payload: everything the two
// verbs it replaced (deps, implink status) reported, plus lock state, review
// state, discussion, and next_actions — in one call, because the loop this
// release is built around starts with an agent orienting itself on one card
// and it should not cost four round trips.
type claimShowData struct {
	ClaimID      string `json:"claim_id"`
	Title        string `json:"title"`
	Facet        string `json:"facet"`
	Module       string `json:"module"`
	Status       string `json:"status"`
	Layout       string `json:"layout"`
	Kind         string `json:"kind"`
	BuildRole    string `json:"build_role,omitempty"`
	Section      string `json:"section,omitempty"`
	MigratedFrom string `json:"migrated_from,omitempty"`
	SourcePath   string `json:"source_path"`
	// Sources is the claim's own evidence — what it rests on, not where its
	// text came from. It sits beside MigratedFrom deliberately, because
	// MigratedFrom is the impoverished thing this replaces: a single free-text
	// note saying WHICH registry a claim came out of, which a reader had to know
	// how to open before they could verify one sentence. See model.Source.
	//
	// It is always present, as [] for a claim with none, rather than omitted.
	// The absent case and the empty case mean the same thing here — this claim
	// cites nothing — and a consumer that has to test for null before ranging is
	// being asked to encode a distinction the data does not make.
	Sources []claimSourceView `json:"sources"`
	// Tracks is this claim's membership in the cross-cutting axis, in either
	// role. It is the inverse of "dossierx track show", and it is here because
	// show is where an agent orients itself on one card: without it, a claim
	// that OWNS a track — whose body is therefore a feature's own prose rather
	// than one module's contract — reads here exactly like any other claim, and
	// the agent about to unlock it cannot see that a track's completeness turns
	// on it. Always present, as [] for a claim in no track.
	Tracks        []claimTrackView   `json:"tracks"`
	Locked        bool               `json:"locked"`
	LockedAt      string             `json:"locked_at,omitempty"`
	ReviewPending bool               `json:"review_pending"`
	Trigger       string             `json:"review_pending_trigger"`
	Edges         claimEdgesData     `json:"edges"`
	ImplementedIn []claimLinkView    `json:"implemented_in"`
	Comments      claimCommentCounts `json:"comments"`
	Ledger        *claimLedgerView   `json:"ledger,omitempty"`
	NextActions   []string           `json:"next_actions"`
}

// claimLedgerViewFor projects the lock store's record for claim into the
// payload, or nil when there is no record to report.
func claimLedgerViewFor(store *lock.Store, claim model.Claim) *claimLedgerView {
	if store == nil {
		return nil
	}
	rec, ok := store.Record(claim.ID)
	if !ok || rec.Subject != lock.SubjectClaim {
		return nil
	}
	_, _, matches := standingLedgerRecord(store, claim)
	return &claimLedgerView{
		Recorded:       true,
		Grandfathered:  rec.Grandfathered,
		Released:       rec.Released(),
		ContentMatches: matches,
		ApprovedAt:     rec.At,
		ApprovedReason: rec.Reason,
	}
}

// claimReviewTrigger names why a claim is review_pending, from the three
// triggers the engine actually has, in the order that decides what to DO about
// it: a flag or a drift is a content change reaudit can propose; an open thread
// is discussion only a human can close. "none" is the real fourth state (a
// drift that was reverted, or a thread hand-resolved in YAML) and matters
// because it is the one case where reaudit has nothing to offer.
func claimReviewTrigger(claim model.Claim, claims []model.Claim, store *lock.Store, flagStore *reaudit.FlagStore) string {
	if !claim.ReviewPending {
		return ""
	}
	drift, flagged, open := comments.PendingTriggers(claim, claims, store, flagStore)
	switch {
	case flagged:
		return "flag"
	case drift:
		return "drift"
	case open > 0:
		return "comments"
	default:
		return "none"
	}
}

// claimNextActions is the whole reason "claim show" exists rather than a
// prettier "deps".
//
// The skills teach a loop — draft freely, lock only with the human's yes,
// unlock->fix->lock to change anything locked, reaudit only for drift — and an
// agent that has to re-derive which step it is on from status + review_pending
// + open threads + lint state will get it wrong sooner or later. This computes
// it once, in the binary, from the same gates the write paths enforce
// (evaluateLockGates is literally lock.Lock's refusal order), so the advice can
// never disagree with what the command would actually do.
func claimNextActions(claim model.Claim, claims []model.Claim, cfg *config.Config, trigger string, links []claimLinkView, ledger *claimLedgerView) []string {
	var actions []string
	id := claim.ID

	if claim.Status != model.StatusLocked {
		gate := evaluateLockGates(claim, claims, cfg)
		switch {
		// The rules NAMED, and the next command pointed at the one that can
		// name them again.
		//
		// This used to read "-> dossierx check --validate", and for the whole
		// family of lints that decide a LOCK that was a dead end: rest-on-locked,
		// roll-up and build-role-required-for-locked all key off a claim's own
		// status, so against the project as it stands — with this claim still
		// draft — `check --validate` reports ok:true and zero findings. The
		// agent was told a finding blocks the lock, sent to a command that
		// reports none, and left with no CLI path to the rule's name.
		// evaluateLockGates lints the ABOUT-TO-BE-LOCKED form, so the answer is
		// right here; --dry-run is where the same answer lives in full.
		case gate.LintErrors > 0:
			actions = append(actions, fmt.Sprintf("%s block locking -> dossierx claim lock %s --dry-run", gate.lintBlockerDetail(), id))
		case gate.UnlockedDoctrineDep != "":
			actions = append(actions, fmt.Sprintf("dependency %s is doctrine and still draft -> lock it first", gate.UnlockedDoctrineDep))
		case len(gate.OpenThreads) > 0:
			actions = append(actions, fmt.Sprintf("%d open comment thread(s) block locking -> the human resolves them in the viewer; that click is the approval", len(gate.OpenThreads)))
		default:
			actions = append(actions, fmt.Sprintf("ready to lock -> ask the human, then dossierx claim lock %s --reason \"<their words>\"", id))
		}
		return actions
	}

	// THE INTEGRITY BRANCH, ahead of every other locked-claim action.
	//
	// A locked claim whose content no longer matches the standing approval on
	// its lock-ledger record is not in any of the four states below. It is the
	// one case where the advice this function otherwise gives is actively
	// harmful: "unlock, edit, relock" RELEASES the record and then re-signs the
	// tampered bytes under a fresh approval, the standing lock-content-drift
	// finding disappears, and no human ever sees the diff. The router skill says
	// so in as many words on the integrity_failed row — "Do not re-lock to make
	// it go away" — and show was the one surface telling an agent to do exactly
	// that, with no way to know better, because it carried no ledger state at
	// all.
	//
	// reaudit --confirm is not offered either: it now refuses this claim for the
	// same reason (see the pre-reaudit integrity gate in main.go), so offering it
	// would violate this function's own contract that every command it prints
	// would actually succeed.
	if ledger != nil && ledger.Recorded && !ledger.Released && !ledger.ContentMatches {
		actions = append(actions, fmt.Sprintf("this claim's content no longer matches the approval on its lock-ledger record -> restore %s from version control; do NOT unlock and relock, that would re-sign content nobody approved (dossierx check --validate names the finding)", claim.SourcePath))
	} else {
		switch trigger {
		case "comments":
			actions = append(actions, "review_pending because of open discussion -> reply on the thread; only the human can resolve it")
		case "drift", "flag":
			actions = append(actions, fmt.Sprintf("review_pending from %s -> dossierx claim reaudit %s (preview), then --confirm --reason \"<their words>\"", trigger, id))
		case "none":
			actions = append(actions, fmt.Sprintf("review_pending with no active trigger -> dossierx claim reaudit %s, or unlock -> fix -> lock", id))
		default:
			actions = append(actions, fmt.Sprintf("locked and settled -> to change it: dossierx claim unlock %s --reason \"<their words>\", edit, relock", id))
		}
	}

	drifted := 0
	for _, l := range links {
		if l.Drifted {
			drifted++
		}
	}
	switch {
	case drifted > 0:
		// "or claim flag it" is only offered when claim flag would actually
		// RUN. flag.go hard-refuses any claim whose rendered content lives
		// outside Body — table rows, steps, raw HTML — because a flag-sourced
		// reaudit rewrites Body only and would clear review_pending while
		// leaving the rendered content stale (DX-AUD-11). Suggesting it anyway
		// cost the agent the real work of composing --claim-says/--now-does
		// /--reason, usually after asking its human for the wording, to be told
		// structured_layout at exit 1 — and the route that does work was never
		// named. This function's contract is that its advice can never disagree
		// with what the command would do, so the layout gate is consulted here,
		// through the very function flag.go refuses with.
		if lay := flagStructuredLayout(claim); lay != "" {
			actions = append(actions, fmt.Sprintf("%d implementation link(s) drifted -> re-run dossierx claim link, or if the code is right and the claim is wrong: dossierx claim unlock %s --reason \"<their words>\", edit, relock (a %s layout cannot be flagged)", drifted, id, lay))
			break
		}
		// The flag invocation is printed WHOLE, with its three required flags,
		// for the same reason the branch above consults the layout gate: this
		// function's contract is that its advice can never disagree with what
		// the command would do, and flag.go refuses a bare `claim flag <id>`
		// with missing_flag before it reads anything else — --claim-says,
		// --now-does and --reason are all required and all non-empty. Naming
		// the verb alone was advice, and a hint that has to be repaired before
		// it runs is the defect cmd/dossierx/output.go's reasonInvocations
		// exists to have stopped printing.
		actions = append(actions, fmt.Sprintf("%d implementation link(s) drifted -> re-run dossierx claim link, or if the code is right and the claim is wrong: dossierx claim flag %s --claim-says \"<what the claim asserts>\" --now-does \"<what the code does>\" --reason \"<their words>\"", drifted, id))
	case len(links) == 0 && claim.Module != "":
		actions = append(actions, fmt.Sprintf("no implementation link yet -> dossierx claim link --module %s --claim %s --file <path>", claim.Module, id))
	}
	return actions
}

func newClaimShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Report one claim's full state in a single call: lifecycle, both edge directions, code links, discussion, and the legal next actions",
		Args:  cobra.ExactArgs(1),
		RunE: envelopeRunE(func(cmd *cobra.Command, args []string) (cmdResult, error) {
			id := args[0]
			cfg, claims, err := loadConfigAndClaims()
			if err != nil {
				return cmdResult{}, err
			}
			claim, ok := loader.FindByID(claims, id)
			if !ok {
				return cmdResult{}, cliout.Errorf(cliout.CodeClaimNotFound, "claim show: claim %q not found: %w", id, errClaimNotFound)
			}

			// Both stores are read best-effort and WITHOUT their sentinels:
			// show is a pure read, and a project whose lock store has not been
			// created yet (nothing locked) must still be inspectable. A load
			// failure degrades the trigger to what the claim itself says, never
			// fails the command — comments.PendingTriggers nil-checks both.
			store, storeErr := lock.LoadStore(storePath(cfg))
			if storeErr != nil {
				store = nil
			}
			flagStore, flagErr := reaudit.LoadFlagStore(flagStorePath(cfg))
			if flagErr != nil {
				flagStore = nil
			}

			mirroredBy, dependedOnBy := incomingEdges(claims, id)
			links := linkViewsFor(cfg, claim)
			trigger := claimReviewTrigger(claim, claims, store, flagStore)

			counts := claimCommentCounts{Threads: len(claim.Comments)}
			for _, th := range claim.Comments {
				counts.Replies += len(th.Replies)
				if th.Status == model.CommentStatusResolved {
					counts.Resolved++
				} else {
					counts.Open++
				}
			}
			counts.OpenThreadIDs = emptyIfNil(claim.OpenThreadIDs())

			lockedAt := ""
			if store != nil {
				lockedAt = store.LockedAt[id]
			}
			ledger := claimLedgerViewFor(store, claim)

			data := claimShowData{
				ClaimID:       claim.ID,
				Title:         claimTitle(claim.ID),
				Facet:         claim.Facet,
				Module:        claim.Module,
				Status:        string(claim.Status),
				Layout:        string(claim.Layout),
				Kind:          string(claim.EffectiveKind()),
				BuildRole:     string(claim.BuildRole),
				Section:       claim.Section,
				MigratedFrom:  claim.MigratedFrom,
				SourcePath:    claim.SourcePath,
				Sources:       claimSourceViews(claim.Sources),
				Tracks:        claimTrackViews(claim.Tracks),
				Locked:        claim.Status == model.StatusLocked,
				LockedAt:      lockedAt,
				ReviewPending: claim.ReviewPending,
				Trigger:       trigger,
				Edges: claimEdgesData{
					Mirrors:        emptyIfNil(claim.Mirrors),
					RestsOn:        emptyIfNil(claim.RestsOn),
					GovernedBy:     claim.Governed.Type,
					GovernedReason: claim.Governed.Reason,
					MirroredBy:     emptyIfNil(mirroredBy),
					DependedOnBy:   emptyIfNil(dependedOnBy),
				},
				ImplementedIn: links,
				Comments:      counts,
				Ledger:        ledger,
				NextActions:   claimNextActions(claim, claims, cfg, trigger, links, ledger),
			}
			if data.ImplementedIn == nil {
				data.ImplementedIn = []claimLinkView{}
			}

			return cmdResult{
				Data: data,
				Text: func() { writeClaimShowText(cmd, data) },
			}, nil
		}),
	}
}

// writeClaimShowText renders a claim show for a human reading the terminal —
// the courtesy surface. The JSON envelope is the contract.
func writeClaimShowText(cmd *cobra.Command, d claimShowData) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "claim show: %s (%s)\n", d.ClaimID, d.Title)
	fmt.Fprintf(out, "  status:             %s", d.Status)
	if d.ReviewPending {
		fmt.Fprintf(out, " review_pending (%s)", d.Trigger)
	}
	fmt.Fprintln(out)
	fmt.Fprintf(out, "  facet/module:       %s / %s\n", d.Facet, d.Module)
	if d.LockedAt != "" {
		fmt.Fprintf(out, "  locked at:          %s\n", d.LockedAt)
	}
	// Printed ONLY in the disagreement case. A line on every clean claim would
	// train the reader to skim past the one that matters, which is the same
	// reasoning reportLedgerFindings prints nothing on a clean run.
	if d.Ledger != nil && d.Ledger.Recorded && !d.Ledger.Released && !d.Ledger.ContentMatches {
		fmt.Fprintln(out, "  lock ledger:        CONTENT DOES NOT MATCH THE APPROVAL ON RECORD (see dossierx check --validate)")
	}
	fmt.Fprintf(out, "  outgoing mirrors:   %v\n", d.Edges.Mirrors)
	fmt.Fprintf(out, "  outgoing rests_on:  %v\n", d.Edges.RestsOn)
	if d.Edges.GovernedBy != "" {
		fmt.Fprintf(out, "  governed_by:        %s", d.Edges.GovernedBy)
		if d.Edges.GovernedReason != "" {
			fmt.Fprintf(out, " (%s)", d.Edges.GovernedReason)
		}
		fmt.Fprintln(out)
	} else {
		fmt.Fprintln(out, "  governed_by:        (unset)")
	}
	fmt.Fprintf(out, "  incoming mirrors:   %v\n", d.Edges.MirroredBy)
	fmt.Fprintf(out, "  incoming rests_on:  %v\n", d.Edges.DependedOnBy)
	if len(d.ImplementedIn) == 0 {
		fmt.Fprintln(out, "  implemented in:     (nothing linked)")
	} else {
		for _, l := range d.ImplementedIn {
			target := l.File
			if l.Symbol != "" {
				target += "#" + l.Symbol
			}
			drift := ""
			if l.Drifted {
				drift = " (DRIFTED)"
			}
			fmt.Fprintf(out, "  implemented in:     %s%s\n", target, drift)
		}
	}
	// Sources print one line per citation, led by the "[n]" marker the body
	// cites it as, so a reader scanning the prose and a reader scanning this
	// block are looking at the same handle. The ANCHOR is on the line — the date
	// for an external source, the hash for an internal one — because the anchor
	// is what makes the citation checkable rather than merely locatable, and a
	// list that showed only titles would read as complete while omitting the one
	// field a sceptic needs. Nothing prints for a claim that cites nothing: a
	// "sources: (none)" line on every card in a corpus that has not adopted
	// citations is noise on every card.
	for _, s := range d.Sources {
		anchor := s.AccessedOn
		if anchor == "" {
			anchor = s.SHA256
		}
		target := s.URL
		if target == "" {
			target = s.Path
			if s.RecordID != "" {
				target += "#" + s.RecordID
			}
		}
		fmt.Fprintf(out, "  source [%d]:         %s — %s", s.Ref, s.Title, s.Kind)
		if target != "" {
			fmt.Fprintf(out, " %s", target)
		}
		if anchor != "" {
			fmt.Fprintf(out, " (%s)", anchor)
		}
		fmt.Fprintln(out)
		if s.Supports != "" {
			fmt.Fprintf(out, "    supports:         %s\n", s.Supports)
		}
		// The limit is printed whenever the author stated one, and it is the half
		// of a citation a hurried reader most needs put in front of them: the
		// common defect is not a fabricated source but an overread one.
		if s.DoesNotSupport != "" {
			fmt.Fprintf(out, "    does not support: %s\n", s.DoesNotSupport)
		}
	}
	for _, tr := range d.Tracks {
		fmt.Fprintf(out, "  track:              %s (%s)\n", tr.TrackID, tr.Role)
	}
	fmt.Fprintf(out, "  comments:           %d thread(s), %d open, %d reply(ies)\n",
		d.Comments.Threads, d.Comments.Open, d.Comments.Replies)
	for _, tid := range d.Comments.OpenThreadIDs {
		fmt.Fprintf(out, "    open thread: %s\n", tid)
	}
	if len(d.NextActions) > 0 {
		fmt.Fprintln(out, "  next actions:")
		for _, a := range d.NextActions {
			fmt.Fprintf(out, "    %s\n", a)
		}
	}
}

// ---------------------------------------------------------------------
// claim list [filters]
// ---------------------------------------------------------------------

// claimListEntry is one row of "claim list". It carries the answer to every
// filter, not just the ones that were asked for, so a caller that filtered on
// --review-pending can still see which of the results are also drifted without
// a second pass.
type claimListEntry struct {
	ClaimID       string `json:"claim_id"`
	Title         string `json:"title"`
	Facet         string `json:"facet"`
	Module        string `json:"module"`
	Status        string `json:"status"`
	ReviewPending bool   `json:"review_pending"`
	MigratedFrom  string `json:"migrated_from,omitempty"`
	Drifted       bool   `json:"drifted"`
	OpenThreads   int    `json:"open_threads"`
	// Sources is the COUNT of this claim's citations, not the citations.
	//
	// A count is what a list can honestly carry: the citations themselves are
	// ten fields each and would make a list of two hundred claims unreadable in
	// both formats, and "claim show" already answers them in full. What the
	// count buys is the one question a list is asked about evidence — which
	// claims have none — which is otherwise unanswerable without one call per
	// claim.
	//
	// No matching --sources FILTER is added. Every existing filter here replaces
	// a verb v0.3.0 retired (stale, coverage) or resolves a card for a human;
	// "claims with no citations" is not a lifecycle state, nothing gates on it,
	// and a filter nothing acts on is surface that has to be maintained forever.
	// The count is on every row, so a caller that wants that set has it already.
	Sources int `json:"sources"`
	// Score is populated only under --match: the fuzzy relevance the row was
	// ranked by. It is exposed rather than hidden so an agent resolving "the
	// retry card" can tell a confident single hit from a three-way tie it
	// should hand back to the human to disambiguate.
	Score int `json:"score,omitempty"`
}

// claimListFilters echoes the filters that produced a result set. An agent
// showing its human "here are the 3 claims" needs to be able to say WHICH 3,
// and re-deriving that from its own call site is exactly the kind of
// bookkeeping the envelope should carry.
type claimListFilters struct {
	ReviewPending bool   `json:"review_pending"`
	Migrated      bool   `json:"migrated"`
	Drifted       bool   `json:"drifted"`
	Facet         string `json:"facet,omitempty"`
	Module        string `json:"module,omitempty"`
	Match         string `json:"match,omitempty"`
}

// claimListData is "dossierx claim list"'s machine payload. Total is the
// unfiltered claim count and PercentOfTotal the share that survived, which is
// what makes this a strict superset of the retired "coverage" verb:
// "claim list --migrated" answers "what fraction of claims carry
// migrated_from" AND names them, where coverage only ever printed the ratio.
type claimListData struct {
	Count          int              `json:"count"`
	Total          int              `json:"total"`
	PercentOfTotal float64          `json:"percent_of_total"`
	Filters        claimListFilters `json:"filters"`
	Claims         []claimListEntry `json:"claims"`
}

func newClaimListCmd() *cobra.Command {
	var reviewPending, migrated, drifted bool
	var facet, module, match string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List claims, optionally filtered by review state, migration note, code drift, facet, module, or a fuzzy --match",
		Args:  cobra.NoArgs,
		RunE: envelopeRunE(func(cmd *cobra.Command, args []string) (cmdResult, error) {
			cfg, claims, err := loadConfigAndClaims()
			if err != nil {
				return cmdResult{}, err
			}
			if module != "" {
				if err := requireKnownModule(cfg, module); err != nil {
					return cmdResult{}, cliout.Errorf(cliout.CodeUnknownModule, "claim list: %w", err)
				}
			}
			// --facet gets the same membership test --module has, and for the
			// reason cliout.CodeUnknownModule already states about modules: "an
			// empty report for a typo'd module looks exactly like success". A
			// human says "show me the contracts facet", the project declares
			// `contract`, and an unchecked filter answers ok:true / count 0 /
			// exit 0 — indistinguishable from the truth, and every decision after
			// it is made against an empty set. The config declares facets: the
			// same way it declares modules:, and "claim new" already refuses an
			// undeclared facet with this exact shape (see parseClaimID).
			if facet != "" && !containsStr(cfg.Facets, facet) && facet != config.ReservedOverviewFacet {
				return cmdResult{}, cliout.Errorf(cliout.CodeBadRequest,
					"claim list: unknown facet %q; this project declares: %s", facet, strings.Join(cfg.Facets, ", ")).
					WithHint("run: dossierx claim list (unfiltered) to see what is there")
			}

			// The drift lookup is built once per module rather than per claim:
			// implink.ViewsByClaim re-hashes every linked file on disk, and
			// doing that once per claim would re-read the same files N times.
			// It is computed only when --drifted is asked for or the module
			// actually has an artifact, and a module with none contributes
			// nothing (the ordinary case).
			driftedIDs := map[string]bool{}
			for _, m := range cfg.Modules {
				byClaim, viewErr := implink.ViewsByClaim(cfg, m)
				if viewErr != nil {
					continue
				}
				for cid, views := range byClaim {
					for _, v := range views {
						if v.Drifted {
							driftedIDs[cid] = true
							break
						}
					}
				}
			}

			entries := make([]claimListEntry, 0, len(claims))
			for _, c := range claims {
				if reviewPending && !c.ReviewPending {
					continue
				}
				if migrated && c.MigratedFrom == "" {
					continue
				}
				if drifted && !driftedIDs[c.ID] {
					continue
				}
				if facet != "" && c.Facet != facet {
					continue
				}
				if module != "" && c.Module != module {
					continue
				}
				score := 0
				if match != "" {
					score = claimMatchScore(match, c)
					if score == 0 {
						continue
					}
				}
				entries = append(entries, claimListEntry{
					ClaimID:       c.ID,
					Title:         claimTitle(c.ID),
					Facet:         c.Facet,
					Module:        c.Module,
					Status:        string(c.Status),
					ReviewPending: c.ReviewPending,
					MigratedFrom:  c.MigratedFrom,
					Drifted:       driftedIDs[c.ID],
					OpenThreads:   len(c.OpenThreadIDs()),
					Sources:       len(c.Sources),
					Score:         score,
				})
			}

			// Ranked by score under --match (the whole point of the flag),
			// alphabetical otherwise. Both orders are total: two claims can
			// never share an id, so the same inputs always print the same
			// bytes — a list a human is asked to pick from must not shuffle.
			sort.Slice(entries, func(i, j int) bool {
				if match != "" && entries[i].Score != entries[j].Score {
					return entries[i].Score > entries[j].Score
				}
				return entries[i].ClaimID < entries[j].ClaimID
			})

			pct := 0.0
			if len(claims) > 0 {
				pct = 100 * float64(len(entries)) / float64(len(claims))
			}
			data := claimListData{
				Count:          len(entries),
				Total:          len(claims),
				PercentOfTotal: pct,
				Filters: claimListFilters{
					ReviewPending: reviewPending,
					Migrated:      migrated,
					Drifted:       drifted,
					Facet:         facet,
					Module:        module,
					Match:         match,
				},
				Claims: entries,
			}

			return cmdResult{
				Data: data,
				Text: func() { writeClaimListText(cmd, data) },
			}, nil
		}),
	}
	cmd.Flags().BoolVar(&reviewPending, "review-pending", false, "only claims currently flagged review_pending (replaces the retired \"stale\" verb)")
	cmd.Flags().BoolVar(&migrated, "migrated", false, "only claims carrying a migrated_from note (replaces the retired \"coverage\" verb)")
	cmd.Flags().BoolVar(&drifted, "drifted", false, "only claims with at least one implementation link whose file changed since it was linked")
	cmd.Flags().StringVar(&facet, "facet", "", "only claims in this facet")
	cmd.Flags().StringVar(&module, "module", "", "only claims in this module")
	cmd.Flags().StringVar(&match, "match", "", "fuzzy-match against each claim's id and derived title, ranked by relevance")
	return cmd
}

// writeClaimListText renders the list as one greppable line per claim plus a
// summary. The per-claim line leads with the id because that is what a caller
// copies into the next command.
func writeClaimListText(cmd *cobra.Command, d claimListData) {
	out := cmd.OutOrStdout()
	for _, e := range d.Claims {
		var flags []string
		if e.ReviewPending {
			flags = append(flags, "review_pending")
		}
		if e.Drifted {
			flags = append(flags, "drifted")
		}
		if e.OpenThreads > 0 {
			flags = append(flags, fmt.Sprintf("open_threads=%d", e.OpenThreads))
		}
		// Printed only when there are some, exactly like open_threads: a
		// "sources=0" token on every line of a corpus that cites nothing is a
		// column of zeroes a reader learns to skip past.
		if e.Sources > 0 {
			flags = append(flags, fmt.Sprintf("sources=%d", e.Sources))
		}
		if e.MigratedFrom != "" {
			flags = append(flags, "migrated_from="+e.MigratedFrom)
		}
		suffix := ""
		if len(flags) > 0 {
			suffix = " " + strings.Join(flags, " ")
		}
		fmt.Fprintf(out, "%s %s%s\n", e.Status, e.ClaimID, suffix)
	}
	fmt.Fprintf(out, "claim list: %d of %d claim(s) (%.1f%%)\n", d.Count, d.Total, d.PercentOfTotal)
}

// ---------------------------------------------------------------------
// --match: fuzzy card resolution
// ---------------------------------------------------------------------

// claimMatchScore scores how well query names claim, 0 meaning "not a match".
//
// The case it exists for is a human saying "the retry-policy card in the
// contract facet" and the agent having to turn that into an id before it can do
// anything at all. So the query is scored against three haystacks and the best
// wins: the id (what a machine would say), the derived title (what a human
// would say), and the two joined with facet and module (what a human ACTUALLY
// says, which mixes both). Scoring the joined form is what lets "retry contract"
// find widget.contract.retry-policy even though neither word alone is the slug.
func claimMatchScore(query string, c model.Claim) int {
	title := claimTitle(c.ID)
	haystack := strings.Join([]string{c.ID, title, c.Facet, c.Module, c.Section}, " ")
	best := fuzzyScore(query, c.ID)
	if s := fuzzyScore(query, title); s > best {
		best = s
	}
	// The joined haystack is scored one tier lower (it can match by accident
	// across field boundaries), so a real id or title hit always outranks it.
	if s := fuzzyScore(query, haystack) - 50; s > best {
		best = s
	}
	if best < 0 {
		return 0
	}
	return best
}

// fuzzyScore is the scoring ladder claimMatchScore ranks with. Higher is a
// better match; 0 means no match at all.
//
// The tiers are ordered by how much the caller had to guess:
//
//	1000       the query IS the target
//	 800       the target starts with the query ("widget.contract" -> that module)
//	 700       the target contains the query verbatim
//	 500       every whitespace/punctuation-separated token appears somewhere
//	 150-300   some, but not all, tokens appear (scaled by how many)
//	 100       the query's characters appear in order but not contiguously
//
// The bands do not overlap, and they must not: TestFuzzyScoreRanksByHowMuchTheCallerGuessed
// pins that a caller who typed more of the real thing always outranks one who
// typed less, which is the only property that makes --match worth trusting.
//
// It is deliberately a small hand-written ladder rather than an edit-distance
// implementation: this ranks a few hundred claim ids for a human to confirm,
// not a search index, and the module's dependency budget is cobra + yaml.
func fuzzyScore(query, target string) int {
	q := strings.ToLower(strings.TrimSpace(query))
	t := strings.ToLower(target)
	if q == "" || t == "" {
		return 0
	}
	if q == t {
		return 1000
	}
	if strings.HasPrefix(t, q) {
		return 800
	}
	if strings.Contains(t, q) {
		return 700
	}

	tokens := matchTokens(q)
	if len(tokens) > 0 {
		matched := 0
		for _, tok := range tokens {
			if strings.Contains(t, tok) {
				matched++
			}
		}
		if matched == len(tokens) {
			return 500
		}
		if matched > 0 {
			// Kept strictly inside (100, 500) so a partial token match always
			// beats a bare subsequence and always loses to a full one.
			return 150 + 150*matched/len(tokens)
		}
	}

	if isSubsequence(q, t) {
		return 100
	}
	return 0
}

// matchTokens splits a query into the alphanumeric runs a human separated with
// spaces, dots, hyphens, or slashes — so "widget/contract retry-policy" and
// "widget contract retry policy" score identically, which is the point.
func matchTokens(q string) []string {
	fields := strings.FieldsFunc(q, func(r rune) bool {
		switch {
		case r >= 'a' && r <= 'z':
			return false
		case r >= '0' && r <= '9':
			return false
		default:
			return true
		}
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

// isSubsequence reports whether every character of q appears in t in order.
// This is the last-resort tier: it is what makes "wcrp" find
// "widget.contract.retry-policy", and also what makes it match a great many
// other things, which is why it scores lowest.
func isSubsequence(q, t string) bool {
	i := 0
	for _, r := range t {
		if i < len(q) && rune(q[i]) == r {
			i++
		}
	}
	return i == len(q)
}

// ---------------------------------------------------------------------
// claim new <id>
// ---------------------------------------------------------------------

// claimNewData is "dossierx claim new"'s machine payload. Path is echoed so the
// caller knows where the claim landed without guessing at the naming rule, and
// Lint reports whether the freshly-written claim actually passes — the whole
// promise of this command is "authoring through here produces a claim that
// validates", and reporting the answer is cheaper than making the agent ask.
type claimNewData struct {
	ClaimID          string `json:"claim_id"`
	Path             string `json:"path"`
	Facet            string `json:"facet"`
	Module           string `json:"module"`
	Status           string `json:"status"`
	Layout           string `json:"layout"`
	LintErrorCount   int    `json:"lint_error_count"`
	LintWarningCount int    `json:"lint_warning_count"`
}

// parseClaimID splits a proposed id into module.facet.slug and validates it
// against the project's own configured modules and facets.
//
// This is the id-shape lint's grammar, enforced at the door instead of after
// the write. Writing a claim whose id the project cannot accept and then
// reporting a lint error about it would be strictly worse: the file is on disk
// either way, and now every other command in the project fails until someone
// deletes it by hand — which is precisely the hand-editing this release gates.
func parseClaimID(cfg *config.Config, id string) (module, facet, slug string, err error) {
	segs := strings.Split(id, ".")
	if len(segs) != 3 || segs[0] == "" || segs[1] == "" || segs[2] == "" {
		return "", "", "", cliout.Errorf(cliout.CodeBadRequest,
			"claim new: id %q must be exactly three non-empty dot-separated segments (module.facet.slug)", id)
	}
	module, facet, slug = segs[0], segs[1], segs[2]
	if !slugPatternMatches(slug) {
		return "", "", "", cliout.Errorf(cliout.CodeBadRequest,
			"claim new: id slug %q must be kebab-case (lowercase alphanumerics separated by single hyphens)", slug)
	}
	if !containsStr(cfg.Modules, module) {
		return "", "", "", cliout.Errorf(cliout.CodeUnknownModule,
			"claim new: id module segment %q is not one of this project's modules: %s", module, strings.Join(cfg.Modules, ", "))
	}
	if !containsStr(cfg.Facets, facet) && facet != config.ReservedOverviewFacet {
		return "", "", "", cliout.Errorf(cliout.CodeBadRequest,
			"claim new: id facet segment %q is not one of this project's facets: %s", facet, strings.Join(cfg.Facets, ", "))
	}
	return module, facet, slug, nil
}

// slugPatternMatches is the kebab-case rule from FORMAT.md, hand-checked rather
// than shared with internal/lint's compiled regexp because that one is
// unexported and this is four lines. The two are pinned equal by
// TestClaimNewProducesALintCleanClaim, which writes a claim and validates it.
func slugPatternMatches(slug string) bool {
	if slug == "" || strings.HasPrefix(slug, "-") || strings.HasSuffix(slug, "-") || strings.Contains(slug, "--") {
		return false
	}
	for _, r := range slug {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
		default:
			return false
		}
	}
	return true
}

// claimNewPath is where a new claim's file goes: <claims_dir>/<id>.yaml.
//
// Flat and derived from the id on purpose. Directory layout is deliberately NOT
// part of the claim schema (see model.Claim.Section's doc comment for the same
// decision made about section headings), so there is no correct subdirectory to
// infer; and because ids are unique project-wide, the flat name can never
// collide. --file overrides the NAME for projects that organize their claims_dir
// their own way.
//
// The override is required to stay INSIDE claims_dir — absolute paths and
// "../" escapes are refused rather than honored. This is not defensive
// paranoia: loader.LoadClaims only ever walks claims_dir, so a claim written
// anywhere else would report a cheerful success for a file the project can
// never see, which is a worse outcome than a clear refusal.
func claimNewPath(cfg *config.Config, id, override string) (string, error) {
	if override == "" {
		return filepath.Join(cfg.ClaimsDir, id+".yaml"), nil
	}
	if filepath.IsAbs(override) {
		return "", cliout.Errorf(cliout.CodeBadRequest,
			"claim new: --file %q must be relative to claims_dir, not absolute", override)
	}
	path := filepath.Join(cfg.ClaimsDir, override)
	rel, err := filepath.Rel(cfg.ClaimsDir, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", cliout.Errorf(cliout.CodeBadRequest,
			"claim new: --file %q escapes claims_dir; a claim outside claims_dir is never loaded", override)
	}
	return path, nil
}

func newClaimNewCmd() *cobra.Command {
	var body, layout, section, buildRole, governedBy, governedReason, file string
	var restsOn, mirrors []string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "new <id>",
		Short: "Author a new DRAFT claim (the sanctioned alternative to hand-writing claim YAML)",
		Long: "Author a new draft claim at <claims_dir>/<id>.yaml.\n\n" +
			"The claim it writes is shaped to pass the lint suite immediately: a body, a\n" +
			"governed_by that satisfies the governed-required rule, and a layout chosen to\n" +
			"suit the facet — card everywhere, except a claim in the reserved overview facet,\n" +
			"which gets a banner because a card there fails orientation-note-shape. Passing\n" +
			"--layout overrides that choice. Draft\n" +
			"authoring is deliberately unfrictioned — no --reason, no confirmation — because\n" +
			"drafts are the agent's workshop. The gate in this release is on LOCKED claims.",
		Args: cobra.ExactArgs(1),
		RunE: envelopeRunE(func(cmd *cobra.Command, args []string) (cmdResult, error) {
			id := args[0]
			cfg, claims, err := loadConfigAndClaims()
			if err != nil {
				return cmdResult{}, err
			}
			module, facet, _, err := parseClaimID(cfg, id)
			if err != nil {
				return cmdResult{}, err
			}
			layout = defaultLayoutForFacet(facet, layout, cmd.Flags().Changed("layout"))
			path, err := claimNewPath(cfg, id, file)
			if err != nil {
				return cmdResult{}, err
			}

			_, exists := loader.FindByID(claims, id)

			if dryRun {
				dr := cliout.NewDryRun("create draft claim "+id).
					Transition("(does not exist)", string(model.StatusDraft))
				if strings.TrimSpace(body) == "" {
					dr.Lacking("--body")
				}
				if governedBy == string(model.GovernedNone) && strings.TrimSpace(governedReason) == "" {
					dr.Lacking("--governed-reason")
				}
				// Both details are written for the verdict they are attached to,
				// not for the failure. A Detail is emitted verbatim whether OK is
				// true or false, so "a claim with this id already exists" printed
				// under [ok] told a human reading the preview the opposite of what
				// the preview had just decided.
				dr.Require("id_is_unused", !exists, boolDetail(exists,
					"a claim with this id already exists",
					"no claim currently has this id"))
				dr.Require("file_is_unused", !fileExists(path), boolDetail(fileExists(path),
					path+" already exists",
					path+" does not exist yet"))
				dr.Effect("creates " + path).
					Effect("the claim is created as a DRAFT: it is yours to edit freely until someone locks it")
				dr.Propose("path", path).
					Propose("facet", facet).
					Propose("module", module).
					Propose("layout", layout).
					Propose("body", body)
				return dryRunResult(cmd, "claim new", dr), nil
			}

			if strings.TrimSpace(body) == "" {
				return cmdResult{}, cliout.Errorf(cliout.CodeMissingFlag,
					"claim new: --body is required and must be non-empty; a claim with no content states nothing")
			}
			if governedBy == string(model.GovernedNone) && strings.TrimSpace(governedReason) == "" {
				return cmdResult{}, cliout.Errorf(cliout.CodeMissingFlag,
					"claim new: --governed-reason is required when --governed-by is %q; the governed-required lint refuses an unexplained ungoverned claim", model.GovernedNone)
			}

			// Claim-file write discipline (Phase 0): take the project-wide
			// claims sentinel FIRST and re-check uniqueness INSIDE it, so two
			// concurrent "claim new" calls for the same id cannot both pass the
			// check above and have the second silently overwrite the first.
			releaseClaims, err := lock.AcquireFileLock(claimsSentinelPath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteConflict, "claim new: %w", err)
			}
			defer releaseClaims()

			claims, err = loadClaims(cfg)
			if err != nil {
				return cmdResult{}, err
			}
			if _, taken := loader.FindByID(claims, id); taken {
				return cmdResult{}, cliout.Errorf(cliout.CodeBadRequest,
					"claim new: claim %q already exists; edit it, or pick another id", id).
					WithHint("run: dossierx claim show " + id)
			}
			if fileExists(path) {
				return cmdResult{}, cliout.Errorf(cliout.CodeBadRequest,
					"claim new: %s already exists and does not hold claim %q; refusing to overwrite it", path, id)
			}

			claim := model.Claim{
				ID:         id,
				Facet:      facet,
				Module:     module,
				Status:     model.StatusDraft,
				Layout:     model.Layout(layout),
				Body:       normalizeClaimBody(body),
				Section:    section,
				BuildRole:  model.BuildRole(buildRole),
				Mirrors:    mirrors,
				RestsOn:    restsOn,
				Governed:   model.Governed{Type: governedBy, Reason: governedReason},
				SourcePath: path,
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "claim new: create claim dir: %w", err)
			}
			if err := loader.SaveClaim(claim); err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "claim new: %w", err)
			}

			// Lint the project WITH the new claim in it and report the verdict.
			// This is the command's promise being kept out loud: the numbers
			// here are what "check --validate" would say a moment later, so an
			// agent that authored something the suite rejects finds out now,
			// from the call that wrote it, rather than three commands later.
			res := check.Status(append(claims, claim), cfg)
			data := claimNewData{
				ClaimID:          id,
				Path:             path,
				Facet:            facet,
				Module:           module,
				Status:           string(model.StatusDraft),
				Layout:           layout,
				LintErrorCount:   len(res.LintErrors),
				LintWarningCount: len(res.LintWarnings),
			}
			return cmdResult{
				Data:     data,
				Warnings: lintWarningLines(res.LintWarnings),
				Text: func() {
					out := cmd.OutOrStdout()
					fmt.Fprintf(out, "claim new: wrote %s (%s, draft)\n", path, id)
					fmt.Fprintf(out, "claim new: lint reports %d error(s), %d warning(s)\n", len(res.LintErrors), len(res.LintWarnings))
				},
			}, nil
		}),
	}
	cmd.Flags().StringVar(&body, "body", "", "the claim's markdown body — what it asserts (required)")
	cmd.Flags().StringVar(&layout, "layout", string(model.LayoutCard), "render layout: card, list, tree, banner (table/steps/mockup need rows/steps/raw_html, which this command does not author)")
	cmd.Flags().StringVar(&section, "section", "", "optional in-content section heading this claim sits under")
	cmd.Flags().StringVar(&buildRole, "build-role", "", "optional build phase: orientation, schema, behavior, api, verification, out-of-scope (required only once the claim locks)")
	cmd.Flags().StringVar(&governedBy, "governed-by", string(model.GovernedNone), "the doctrine claim id backing this claim, or \"none\"")
	cmd.Flags().StringVar(&governedReason, "governed-reason", "", "why this claim is deliberately ungoverned (required when --governed-by is \"none\")")
	cmd.Flags().StringSliceVar(&mirrors, "mirrors", nil, "claim ids this claim mirrors")
	cmd.Flags().StringSliceVar(&restsOn, "rests-on", nil, "claim ids this claim rests on")
	cmd.Flags().StringVar(&file, "file", "", "write to this path instead of <claims_dir>/<id>.yaml (relative to claims_dir)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what creating this claim would do, and write nothing")
	return cmd
}

// defaultLayoutForFacet resolves --layout's default against the facet the id
// lands in.
//
// This command's help text promises "the claim it writes is shaped to pass the
// lint suite immediately", and for the one reserved facet it did the opposite,
// every single time. A claim under `overview` IS an orientation note — the facet
// name is what makes it one (model.Claim.EffectiveKind) — and
// orientation-note-shape requires every orientation note to render as layout:
// banner. The flag's default is "card", so `claim new widget.overview.router`
// wrote a file the very next lint call rejected, and the command's own
// lint_error_count reported the failure it had just created.
//
// The fix is a default, not a refusal: an explicit --layout still wins, because
// a caller who names a layout is making a choice and this command's job is to
// carry it out (the lint suite is where a wrong choice gets answered, and it
// will say so in the same call's lint_error_count). Only the UNSET case moves,
// which is precisely the case where the command is the one picking.
func defaultLayoutForFacet(facet, layout string, layoutWasSet bool) string {
	if layoutWasSet || facet != config.ReservedOverviewFacet {
		return layout
	}
	return string(model.LayoutBanner)
}

// fileExists is a plain "is something already there" probe. Any stat error
// other than a clean success is treated as "not there" on purpose: the write
// that follows will fail loudly with the real reason, and guessing here would
// only replace a good error message with a worse one.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// normalizeClaimBody makes a --body value safe to persist.
//
// loader.SaveClaim refuses (ErrClaimNotRoundTrippable) any claim whose body
// would not survive a marshal/strict-decode round trip byte-exact, and yaml.v3
// mishandles exactly two shapes: a leading blank/whitespace-only line, and a
// body with no trailing newline emitted as a block scalar. Trimming the leading
// whitespace and guaranteeing the trailing newline is what stops a perfectly
// reasonable shell heredoc from being rejected at the door — and matches how
// every hand-authored claim in the corpus is written anyway.
func normalizeClaimBody(body string) string {
	b := strings.TrimLeft(body, " \t\r\n")
	if b == "" {
		return b
	}
	if !strings.HasSuffix(b, "\n") {
		b += "\n"
	}
	return b
}
