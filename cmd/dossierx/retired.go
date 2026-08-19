// retired.go holds the surface's memory of its own past: the top-level verbs
// and four comment verbs v0.3.0 removed, plus `migrate`, removed by v0.4.0 —
// registered as HIDDEN stubs that exist for exactly one purpose, to fail with
// the sentence that names the replacement.
//
// The reason they have to exist as commands, rather than as a paragraph in
// SKILL.md, is that cobra decides two things BEFORE any unknown-command handler
// of ours can run, and both decisions were wrong for precisely the invocation an
// agent carrying pre-v0.3.0 memory actually types.
//
//   - Flag parsing runs first. `dossierx comment resolve <claim> <thread>` did
//     reach requireSubcommand's good message ("comment is a command group and
//     does nothing on its own", hint naming add/inbox/list/reply). Add the flag
//     the verb used to take — `--as agent` — and the run failed at parse time
//     with `unknown flag: --as` and no hint at all, which reads as though
//     `comment resolve` exists and merely takes different flags now. SKILL.md
//     anticipates that exact recall ("if you remember one of those, you are
//     remembering a version that no longer exists"); the binary answered the
//     bare form and not the remembered one.
//   - An unknown command at the ROOT never reaches our RunE either: cobra's
//     legacyArgs rejects it during Execute, so `dossierx lint` produced
//     `{"command":"","error":{"code":"usage","message":"unknown command
//     \"lint\" for \"dossierx\""}}` — no hint, no replacement named, and an
//     empty command field.
//
// Each stub therefore whitelists unknown flags and accepts arbitrary positional
// arguments, so that no invocation of it can fail for any reason other than the
// one this file is written to report. The message names the removal, the hint
// names the replacement, and the code is `usage` — the caller's invocation is
// wrong, not the project's state.
//
// They are HIDDEN (absent from --help, from the completion script, and from
// requireSubcommand's "run one of:" list) because they are not surface: nothing
// should discover them, and the seven-noun/nineteen-leaf contract is a design
// constraint the release argues for. annotationRetired is what keeps
// TestSurfaceIsNineteenLeavesUnderSevenNouns honest about that — it excludes these
// by MARK, not by hidden-ness, so a real leaf can never be smuggled past the
// count by hiding it.
package main

import (
	"github.com/spf13/cobra"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
)

// annotationRetired marks a command that exists only to explain its own
// removal. See this file's doc comment.
const annotationRetired = "dossierx/retired"

// retired reports whether cmd is one of the removal stubs.
func retired(cmd *cobra.Command) bool {
	return cmd != nil && cmd.Annotations[annotationRetired] == "true"
}

// retiredCmd builds one removal stub.
//
// The two cobra settings are not stylistic. FParseErrWhitelist lets `--as`,
// `--body`, `--json` and every other flag a caller remembers parse without
// error, so the failure is always this function's message rather than cobra's
// flag complaint; Args accepts whatever positional arguments came with them.
// Together they mean the stub answers the REMEMBERED invocation, in full, which
// is the only invocation it will ever see.
func retiredCmd(use, message, hint string) *cobra.Command {
	cmd := &cobra.Command{
		Use:                use,
		Hidden:             true,
		Args:               cobra.ArbitraryArgs,
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
		Annotations:        map[string]string{annotationRetired: "true"},
		SilenceUsage:       true,
		RunE: envelopeRunE(func(cmd *cobra.Command, args []string) (cmdResult, error) {
			return cmdResult{}, cliout.Errorf(cliout.CodeUsage, "%s", message).WithHint(hint)
		}),
	}
	return cmd
}

// viewerOnlyHint is the recovery shared by the four comment verbs the CLI
// dropped. It states the rights rule rather than only the mechanics, because an
// agent that learns "run this other command instead" and not "resolving is the
// human's approval" will go looking for another way to resolve.
const viewerOnlyHint = `resolving or reopening a thread is the human's approval and lives only in the viewer (dossierx serve); an agent may reply: dossierx comment reply <claim-id> <thread-id> --as agent --body "…"`

// retiredCommentVerbs are the four verbs v0.3.0 removed from the comment group.
// They are still fully implemented in internal/comments and still served over
// internal/serve's HTTP API — see comment.go's package doc for why the CLI is
// not where they belong.
func retiredCommentVerbs() []*cobra.Command {
	return []*cobra.Command{
		retiredCmd("resolve",
			`comment resolve: removed in v0.3.0; a thread is resolved in the viewer, by the human, and that click is the approval the lock gate waits for`,
			viewerOnlyHint),
		retiredCmd("reopen",
			`comment reopen: removed in v0.3.0; a thread is reopened in the viewer, by the human who is reading the claim`,
			viewerOnlyHint),
		retiredCmd("edit",
			`comment edit: removed in v0.3.0; a review history the agent can rewrite is not a review history, so editing a message stays where its author is — the viewer`,
			viewerOnlyHint),
		retiredCmd("delete",
			`comment delete: removed in v0.3.0; a review history the agent can rewrite is not a review history, so deleting a message stays where its author is — the viewer`,
			viewerOnlyHint),
	}
}

// retiredTopLevelCmds are the verbs v0.3.0 folded into the seven nouns, plus the
// one v0.4.0 removed outright. Each hint is the corresponding row of SKILL.md's
// "if you remember an older command" table, in the same words, so the binary and
// the skill cannot drift into disagreeing about where a caller should go next.
//
// lock/unlock/flag/reaudit ARE here now, and the reason they were not for so
// long is worth keeping because it was wrong in an instructive way. It said they
// moved under a noun of the same name, so the root "already" answers
// `dossierx lock <id>` with a hint listing the seven nouns. It does not. This
// file's own package doc states why: cobra's legacyArgs rejects an unknown
// command during Execute, so the root's RunE — the only hint-bearing branch —
// is never reached. Measured before the fix:
//
//	dossierx lock some.claim.id  -> {"ok":false,"command":"","error":{
//	                                 "code":"usage","message":"unknown command
//	                                 \"lock\" for \"dossierx\""}}
//
// No hint, no replacement named, empty command field: exactly the answer this
// file exists to remove, on the four verbs most likely to be typed from
// pre-v0.3.0 memory, since each still exists at `dossierx claim <verb>`.
//
// A second false reason nearly kept them out — that four new commands would move
// the noun and leaf counts every skill and document states. A retired stub does
// not enter `commands` in surface.json; it enters `retired`, and `counts` reads
// 19 commands under 7 nouns either way. The twelve stubs already here prove it.

func retiredTopLevelCmds() []*cobra.Command {
	checkHint := `run: dossierx check (add --validate for a read-only pass that writes nothing)`
	return []*cobra.Command{
		retiredCmd("lock",
			`lock: moved in v0.3.0 — the verb is now a leaf of the claim noun`,
			`run: dossierx claim lock <id> --reason "..."`),
		retiredCmd("unlock",
			`unlock: moved in v0.3.0 — the verb is now a leaf of the claim noun`,
			`run: dossierx claim unlock <id> --reason "..."`),
		retiredCmd("flag",
			`flag: moved in v0.3.0 — the verb is now a leaf of the claim noun`,
			`run: dossierx claim flag <id> --claim-says "<what the claim asserts>" --now-does "<what the code does>" --reason "<their words>"`),
		retiredCmd("reaudit",
			`reaudit: moved in v0.3.0 — the verb is now a leaf of the claim noun`,
			// --confirm alone is not enough to apply: newReauditCmd's own doc
			// comment states the rule (--reason is required on the writing
			// path, never on the preview), and main.go's requireReason enforces
			// it. A hint that named --confirm without --reason would exit 1 /
			// missing_flag on the very command it recommended — the same
			// defect this file's other three hints were written to avoid.
			`run: dossierx claim reaudit <id> (add --confirm and --reason "..." to accept the proposal)`),
		retiredCmd("lint",
			`lint: removed in v0.3.0; linting is a stage of check, not a verb — findings are data.lint_findings on check's envelope`,
			checkHint),
		retiredCmd("catalog",
			`catalog: removed in v0.3.0; building the catalog is a stage of check, not a verb`,
			checkHint),
		retiredCmd("render",
			`render: removed in v0.3.0; rendering the viewer is a stage of check, not a verb`,
			checkHint),
		retiredCmd("deps",
			`deps: removed in v0.3.0; a claim's dependencies, dependents and links are part of what claim show reports`,
			`run: dossierx claim show <id>`),
		retiredCmd("stale",
			`stale: removed in v0.3.0; "stale" was a filter wearing a verb's clothes, and it is now a flag on claim list`,
			`run: dossierx claim list --review-pending`),
		retiredCmd("coverage",
			`coverage: removed in v0.3.0; "coverage" was a filter wearing a verb's clothes, and it is now a flag on claim list`,
			`run: dossierx claim list --migrated`),
		retiredCmd("implink",
			`implink: removed in v0.3.0; recording a code link is dossierx claim link, and reading one back is part of what claim show reports`,
			// `claim link` is declared cobra.NoArgs and requires --module, --claim
			// and --file, so the old `claim link <id> --file <path>` shape this hint
			// printed exits non-zero twice over: the positional id is rejected before
			// the body runs, and two required flags are absent. The spelling below is
			// claim.go's own next_actions spelling, which is the one that runs.
			`run: dossierx claim link --module <module> --claim <id> --file <path> (to record one), or dossierx claim show <id> (to read them back)`),
		// migrate is the one stub NOT from v0.3.0, and it is the one that most
		// needs to exist: README, SKILL.md, the CI template and CHANGELOG all
		// spent a release telling every agent to type exactly `dossierx migrate
		// --adopt`, and flag parsing runs before any unknown-command handler, so
		// without the stub that invocation fails as `unknown flag: --adopt`.
		retiredCmd("migrate",
			`migrate: removed in v0.4.0; there is no automatic adoption and no migration command — nothing can attest to content no ledger ever recorded`,
			preLedgerCrossingHint),
	}
}
