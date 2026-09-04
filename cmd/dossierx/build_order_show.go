// build_order_show.go is "dossierx build-order show": the leaf that HANDS OVER
// a module's locked build order, as against the three that compute, inspect and
// approve one.
//
// It is split from build_order.go for the same reason build_order.go was split
// from main.go — file size — and because it is the one leaf on the noun whose
// product is a rendering rather than a state change: it writes nothing, takes
// no sentinel, and its whole surface is which of three renderings the caller
// wants.
package main

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/BarterX-Tech/dossierx/internal/buildorder"
	"github.com/BarterX-Tech/dossierx/internal/cliout"
)

// formatMermaid is the third --format value, and it exists on exactly one leaf.
//
// It is NOT added to the root's persistent --format, and that is the decision
// rather than an implementation detail: "mermaid" is a rendering of one
// specific payload, and a root flag advertising it would promise every leaf a
// diagram none of the other twenty-four has anything to draw. The root's
// validator (main.go) therefore still accepts json and text only; this leaf
// registers a LOCAL --format that shadows it, so the extra value is available
// exactly where it means something.
const formatMermaid = "mermaid"

// buildOrderShowPhaseData is one of the six blocks "build-order show" hands
// over: the five phases in buildorder.Phases numbered 1..5, then the excluded
// block numbered 0.
//
// It is a struct in THIS package rather than buildorder.PhaseView marshalled
// directly, and the verb copies field by field, because PhaseView carries no
// json tags: the machine contract's key names are declared where the contract
// lives, so a rename inside internal/buildorder cannot silently rename a key an
// agent branches on.
//
// Mermaid is "" for a phase with no claims and for the excluded block — see
// buildorder.Mermaid for why emitting a header-only chunk there would let a
// diagram that failed to generate pass for a phase that legitimately has
// nothing in it.
type buildOrderShowPhaseData struct {
	Phase        string              `json:"phase"`
	Number       int                 `json:"number"`
	Definition   string              `json:"definition"`
	Claims       []string            `json:"claims"`
	Levels       [][]string          `json:"levels"`
	Ghosts       []buildorder.Ghost  `json:"ghosts"`
	CrossModule  map[string][]string `json:"cross_module"`
	ExcludedDeps []string            `json:"excluded_deps"`
	Locked       int                 `json:"locked"`
	Mermaid      string              `json:"mermaid"`
}

// buildOrderShowData is "build-order show"'s machine payload.
//
// Phases always has SIX entries in the fixed order, whatever the artifact
// stores, so a consumer can index by position; the artifact itself omits an
// empty phase, which is why Views fills the six by name.
//
// Path is the RESOLVED artifact path — the same absolute value propose and lock
// already report — never a project-relative string. An agent that reads
// data.path and opens it resolves a relative path against its own working
// directory, which --config makes routinely different from the project's.
type buildOrderShowData struct {
	Module   string                    `json:"module"`
	Path     string                    `json:"path"`
	Locked   bool                      `json:"locked"`
	LockedAt string                    `json:"locked_at,omitempty"`
	Stale    bool                      `json:"stale"`
	Phases   []buildOrderShowPhaseData `json:"phases"`
}

// buildOrderStaleWarning is the one warning this leaf can emit. It is a
// warning and not a refusal because a stale order is still the order that was
// approved, and a reader asking to see it is entitled to see it — with the fact
// that its claims have moved stated, not implied.
const buildOrderStaleWarning = "the locked order is stale: re-propose and re-lock before following it"

func newBuildOrderShowCmd() *cobra.Command {
	var module string
	var format string
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Print --module's build order: the machine envelope, a phase table, or one mermaid flowchart per phase",
		// Selection is --module only; see newBuildOrderProposeCmd's Args in
		// build_order.go for what a discarded positional costs on this noun.
		Args: cobra.NoArgs,
		RunE: envelopeRunE(func(cmd *cobra.Command, args []string) (cmdResult, error) {
			// The format is validated FIRST, before --module and before the
			// project is loaded, for the reason main.go's PersistentPreRunE
			// gives for validating the root flag at the front door: a value
			// nothing renders must fail as itself, not fall through to a
			// default the caller did not ask for and not be masked by a second
			// complaint about a flag the caller may well have supplied.
			switch format {
			case formatJSON, formatText, formatMermaid:
			default:
				return cmdResult{}, cliout.Errorf(cliout.CodeUnsupportedFormat,
					"build-order show: --format must be %q, %q or %q, got %q", formatJSON, formatText, formatMermaid, format)
			}
			// text and mermaid are both PROSE as far as the envelope machinery
			// is concerned: emit() switches on formatFlag alone, and both of
			// them want the Text closure printed rather than an envelope
			// wrapped around a string. Which of the two renderings that closure
			// produces is decided below, from `format` itself.
			if format != formatJSON {
				formatFlag = formatText
			}

			if err := requireModuleFlag(module); err != nil {
				return cmdResult{}, err
			}
			cfg, claims, err := loadConfigAndClaims()
			if err != nil {
				return cmdResult{}, err
			}
			if err := requireKnownModule(cfg, module); err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeUnknownModule, "build-order show: %w", err)
			}

			// Status, not LoadArtifact: `stale` has to be recomputed against
			// the claims as they are now, or this command would hand over an
			// order it describes as current while the claims under it have
			// moved.
			path := buildorder.ArtifactPath(cfg, module)
			artifact, err := buildorder.Status(path, claims, cfg)
			if err != nil {
				if errors.Is(err, buildorder.ErrNotProposed) {
					// DELIBERATELY UNLIKE "build-order status", which answers
					// this same state with ok:true, proposed:false and exit 0.
					// The two verbs are asked different questions: status asks
					// "is there one?", for which no is a complete and
					// successful answer, while show asks "give it to me", and
					// under --format text or --format mermaid an empty stdout
					// at exit 0 is indistinguishable from a diagram of a module
					// that genuinely has nothing in it. A caller piping this
					// into a file would write an empty .mmd and never learn
					// why. TestCLI_BuildOrderShow_NotProposed pins the
					// divergence so it reads as the decision it is.
					return cmdResult{}, cliout.Errorf(cliout.CodeNotProposed,
						"build-order show: module %q has no build order to show", module).
						WithHint(fmt.Sprintf("run \"dossierx build-order propose --module %s\"", module))
				}
				return cmdResult{}, cliout.Errorf(cliout.CodeInternal, "build-order show: %w", err)
			}

			views, _, err := buildorder.Views(artifact, claims)
			if err != nil {
				// Views fails only on the artifact's OWN stored bytes — a
				// rests_on cycle among a phase's entries, or a claim in a block
				// whose phase name is not one of the five and which therefore
				// appears in no view at all. Propose writes neither. The claims
				// are not the problem in either case, and telling the caller to
				// go fix them would send them looking at content that is
				// already correct; the recovery is the one that discards the
				// artifact.
				return cmdResult{}, cliout.Errorf(cliout.CodeBuildOrderHandEdited, "build-order show: %w", err).
					WithHint(fmt.Sprintf("re-run \"dossierx build-order propose --module %s\" and lock the order the engine derives", module))
			}

			// The CLI's palette is the fixed literal one: a terminal, a .mmd
			// file and a pasted PR comment have no colour scheme to follow. The
			// viewer's <pre> is the same generator under PaletteCSS.
			opts := buildorder.MermaidOptions{Palette: buildorder.PaletteLiteral}
			phases := make([]buildOrderShowPhaseData, 0, len(views))
			for _, v := range views {
				phases = append(phases, showPhaseData(v, opts))
			}

			var warnings []string
			if artifact.Stale {
				warnings = append(warnings, buildOrderStaleWarning)
			}

			return cmdResult{
				Warnings: warnings,
				Data: buildOrderShowData{
					Module:   module,
					Path:     path,
					Locked:   artifact.Locked,
					LockedAt: artifact.LockedAt,
					Stale:    artifact.Stale,
					Phases:   phases,
				},
				Text: func() {
					out := cmd.OutOrStdout()
					if format == formatMermaid {
						writeBuildOrderMermaid(out, cmd.ErrOrStderr(), module, artifact.Stale, phases)
						return
					}
					writeBuildOrderTable(out, module, artifact.Locked, artifact.LockedAt, artifact.Stale, views)
				},
			}, nil
		}),
	}
	cmd.Flags().StringVar(&module, "module", "", "module to show the build order for (required)")
	// A LOCAL --format, shadowing the root's persistent one for this leaf only.
	// cobra's Flags() merges a parent's persistent flags into a command's own
	// set and never overwrites a name already there, so this definition is the
	// one that binds wherever --format appears on this command line, and the
	// root's validator sees its untouched default.
	cmd.Flags().StringVar(&format, "format", formatJSON, "output format: json (the machine contract — one envelope per run), text (a phase table) or mermaid (one flowchart per phase)")
	return cmd
}

// showPhaseData copies one buildorder.PhaseView into the tagged payload shape,
// field by field, and renders its diagram.
//
// Every slice and map is materialised as empty rather than left nil: a machine
// contract that answers `null` where it promised a list makes every consumer
// write the same three-line guard, and (f)'s envelope says arrays are never
// null.
func showPhaseData(v buildorder.PhaseView, opts buildorder.MermaidOptions) buildOrderShowPhaseData {
	ids := make([]string, 0, len(v.Claims))
	for _, entry := range v.Claims {
		ids = append(ids, entry.ID)
	}
	levels := make([][]string, 0, len(v.Levels))
	for _, layer := range v.Levels {
		levels = append(levels, append([]string(nil), layer...))
	}
	ghosts := v.Ghosts
	if ghosts == nil {
		ghosts = []buildorder.Ghost{}
	}
	cross := v.CrossModule
	if cross == nil {
		cross = map[string][]string{}
	}
	deps := v.ExcludedDeps
	if deps == nil {
		deps = []string{}
	}
	return buildOrderShowPhaseData{
		Phase:        v.Name,
		Number:       v.Number,
		Definition:   v.Definition,
		Claims:       ids,
		Levels:       levels,
		Ghosts:       ghosts,
		CrossModule:  cross,
		ExcludedDeps: deps,
		Locked:       v.Locked,
		Mermaid:      buildorder.Mermaid(v, opts),
	}
}

// writeBuildOrderMermaid prints the non-empty per-phase flowcharts, separated
// by one blank line and nothing else.
//
// The STDOUT bytes are exactly the non-empty data.phases[].mermaid strings the
// same run reports under --format json — no header, no banner, no stale note.
// That is what makes `... --format mermaid > out/<m>.mmd` produce a file a
// mermaid parser accepts chunk by chunk: a consumer splits on blank lines and
// every chunk must contain `flowchart TD`, so a chunk that does not is a FAILED
// parse rather than a comment block that quietly passes for an empty phase.
//
// Two facts about the run cannot go on stdout without breaking that contract,
// and both are stated on STDERR instead, where a redirect of stdout still
// captures exactly the diagrams:
//
//   - An order with NO phase carrying claims — every claim in the module marked
//     out-of-scope, which propose and lock both allow — prints zero bytes at
//     exit 0, which is the one shape this leaf otherwise refuses to produce
//     (see the not_proposed branch). The person watching the terminal is told
//     why the file they just wrote is empty instead of being left to guess.
//   - A STALE order. Under --format json the staleness rides in warnings[] and
//     data.stale; under --format text it is in the header's parenthetical. This
//     format had NEITHER, on any stream, so `--format mermaid > out/<m>.mmd`
//     over a module list — the exact loop the acceptance procedure runs — wrote
//     a .mmd file byte-indistinguishable from a current one for an order nobody
//     should be following, and said nothing anywhere.
func writeBuildOrderMermaid(out, errOut io.Writer, module string, stale bool, phases []buildOrderShowPhaseData) {
	first := true
	for _, p := range phases {
		if p.Mermaid == "" {
			continue
		}
		if !first {
			fmt.Fprintln(out)
		}
		first = false
		fmt.Fprint(out, p.Mermaid)
	}
	if first {
		fmt.Fprintf(errOut, "build-order show: module %q has a build order in which no phase carries a claim, so there is no diagram to draw; run --format text to see what it does contain\n", module)
	}
	if stale {
		fmt.Fprintf(errOut, "build-order show: module %q: %s\n", module, buildOrderStaleWarning)
	}
}

// writeBuildOrderTable prints the human rendering: one header line, then one
// row per phase with its counts, then that phase's claims by level with the
// file each was loaded from and what it rests on.
func writeBuildOrderTable(out io.Writer, module string, locked bool, lockedAt string, stale bool, views []buildorder.PhaseView) {
	fmt.Fprintf(out, "build-order show: %s (%s)\n", module, buildOrderStateWords(locked, lockedAt, stale))

	for _, v := range views {
		if v.Number == 0 {
			writeExcludedRow(out, v)
			continue
		}
		fmt.Fprintf(out, "  phase %d of %d  %-15s%s\n", v.Number, len(buildorder.Phases), v.Name, v.Counts())
		writeLevelRows(out, v)
	}
}

// buildOrderStateWords is the parenthetical on the header line. A proposed but
// unlocked order says so rather than borrowing the word "locked" with a "false"
// beside it: this leaf shows an unlocked order deliberately (there is nothing
// to hide about a proposal), and the reader's first question is whether what
// they are looking at is the approved sequence.
func buildOrderStateWords(locked bool, lockedAt string, stale bool) string {
	if !locked {
		return "proposed, not locked"
	}
	if stale {
		return "locked " + lockedAt + ", STALE"
	}
	return "locked " + lockedAt + ", not stale"
}

// writeLevelRows prints one phase's claims grouped by Kahn level, then the
// edges that leave the phase.
//
// The levels are the artifact's own, re-derived by buildorder.Views from the
// locked bytes; L0 is "nothing in this phase blocks it".
func writeLevelRows(out io.Writer, v buildorder.PhaseView) {
	file := map[string]string{}
	restsOn := map[string][]string{}
	for _, entry := range v.Claims {
		file[entry.ID] = entry.File
		restsOn[entry.ID] = entry.RestsOn
	}
	inPhase := map[string]bool{}
	for _, entry := range v.Claims {
		inPhase[entry.ID] = true
	}
	ghostPhase := map[string]string{}
	for _, g := range v.Ghosts {
		ghostPhase[g.ID] = g.Phase
	}

	for level, ids := range v.Levels {
		for _, id := range ids {
			// The file column is padded to 29 and then followed by an EXPLICIT
			// space, not padded to 30 and butted against the tail, because a
			// pad is not a separator: a source path of 29 characters or more
			// consumes the whole field and the next column starts in the very
			// next byte, so "…/behavior.yaml" and "rests on:" arrive as one
			// unbroken token and the reader cannot see where the path ends. It
			// is 29-plus-one rather than 30-plus-one so a path shorter than
			// that lands "rests on:" in exactly the column the id column's own
			// "%-44s " already puts it in.
			//
			// Trimmed on the right rather than printed straight: the column
			// padding exists to line the rows up, and a claim with no drawn
			// edge would otherwise end in thirty spaces no reader can see and
			// every diff, editor and golden comparison can.
			row := fmt.Sprintf("    L%d  %-44s %-29s %s", level, id, file[id], drawnEdges(restsOn[id], inPhase, ghostPhase))
			fmt.Fprintln(out, strings.TrimRight(row, " "))
		}
	}

	// The edges that do NOT appear in this phase's diagram, listed once per
	// phase because that is the granularity buildorder.PhaseView carries them
	// at — and listed at all because an edge silently dropped from a build
	// order is a dependency an implementer does not know about.
	modules := make([]string, 0, len(v.CrossModule))
	for m := range v.CrossModule {
		modules = append(modules, m)
	}
	sort.Strings(modules)
	for _, m := range modules {
		ids := v.CrossModule[m]
		fmt.Fprintf(out, "        cross-module: %s (%d): %s\n", m, len(ids), strings.Join(ids, ", "))
	}
	if len(v.ExcludedDeps) > 0 {
		fmt.Fprintf(out, "        rests on out-of-scope (%d): %s\n", len(v.ExcludedDeps), strings.Join(v.ExcludedDeps, ", "))
	}
}

// drawnEdges renders the "rests on:" tail of a claim row: the same-phase and
// earlier-phase targets, which are the two kinds the diagram draws. An
// earlier-phase target carries its phase in parentheses, because the reader's
// next question about one is where to look for it.
func drawnEdges(deps []string, inPhase map[string]bool, ghostPhase map[string]string) string {
	var parts []string
	for _, dep := range deps {
		switch {
		case inPhase[dep]:
			parts = append(parts, dep)
		case ghostPhase[dep] != "":
			parts = append(parts, fmt.Sprintf("%s (%s)", dep, ghostPhase[dep]))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "rests on: " + strings.Join(parts, ", ")
}

// writeExcludedRow prints the sixth block: the claims this module marked
// out-of-scope. They are named, never counted only — a claim dropped from a
// build order with nothing but a number to show for it is the silent
// disappearance internal/buildorder's Excluded field exists to prevent.
func writeExcludedRow(out io.Writer, v buildorder.PhaseView) {
	if len(v.Claims) == 0 {
		fmt.Fprintf(out, "  %-14s0 claims\n", "excluded")
		return
	}
	ids := make([]string, 0, len(v.Claims))
	for _, entry := range v.Claims {
		ids = append(ids, entry.ID)
	}
	word := "claims"
	if len(ids) == 1 {
		word = "claim"
	}
	fmt.Fprintf(out, "  %-14s%d %s: %s\n", "excluded", len(ids), word, strings.Join(ids, ", "))
}
