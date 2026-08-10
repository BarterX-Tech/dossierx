// output.go is the CLI half of the v0.3.0 machine contract: the global
// --format flag, the wrapper that turns a command body into either today's
// terminal prose or one internal/cliout.Envelope, and the single place that
// decides an error's machine code and the process's exit status.
//
// The shape it enforces on a converted command is: the RunE body does the work
// and returns a cmdResult plus an error; it does NOT print. emit() then renders
// that ONE result under whichever format was asked for. Keeping the two apart
// is what makes "JSON by default, byte-identical prose on demand" possible
// without writing every command twice — the Text closure a body hands back is
// literally the printing code the command had before, moved down a level.
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
	"github.com/BarterX-Tech/dossierx/internal/comments"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/implink"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/lock"
)

// The two legal --format values.
//
// JSON is the DEFAULT, and that is the deliberate half of the decision: the
// agent is the operator of this CLI (the human's one command is "dossierx
// serve"), and a surface whose primary consumer has to opt in to the machine
// format has its priorities backwards. Text remains complete and supported —
// every converted command renders the exact prose it always did under
// --format text, which is how the golden fixtures in check_parity_test.go stay
// byte-for-byte green.
const (
	formatJSON = "json"
	formatText = "text"
)

// formatFlag is the raw value of the global --format flag. It is a package
// global for the same reason configPath is: cobra binds persistent flags to
// addresses at command-construction time, and every RunE in this package needs
// to see the result. Re-registering the flag in each newRootCmd() call resets
// it to formatJSON, so an in-process test that builds a fresh root never
// inherits the previous test's format.
var formatFlag string

// jsonOutput reports whether this invocation should emit envelopes. Anything
// other than an explicit "text" is JSON, so the default and any value that
// somehow escaped validation both land on the machine format rather than
// silently producing prose an agent cannot parse.
func jsonOutput() bool { return formatFlag != formatText }

// annotationTextOnly marks a command that has NOT been converted to the
// envelope and prints prose regardless of --format. The mark, not a comment, is
// what stops emit() and runCLI from mixing a JSON error document into such a
// command's text output.
//
// Exactly one command carries it, and permanently: "serve". It is a
// long-running process, not a request/response call — its useful output (the
// URL to open) has to appear BEFORE it blocks, which the
// one-envelope-per-invocation contract cannot express, and its consumer is the
// human anyway. During Phase 1 the annotation also carried the ten commands
// Phase 2 then deleted; those are gone, and nothing should be added here
// without the same "structurally cannot be one envelope" justification.
const annotationTextOnly = "dossierx/text-only"

// textOnly reports whether cmd (or any ancestor, so marking a group marks its
// leaves) opted out of the envelope.
func textOnly(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Annotations[annotationTextOnly] == "true" {
			return true
		}
	}
	return false
}

// markTextOnly stamps the opt-out annotation and returns cmd, so it composes
// inline in newRootCmd's AddCommand list.
func markTextOnly(cmd *cobra.Command) *cobra.Command {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[annotationTextOnly] = "true"
	return cmd
}

// commandPath is the envelope's "command" field: the command path with the
// binary name stripped, e.g. "build-order lock". A skill correlates a response
// with the call it made by this string, so it must name the SUBcommand, not the
// binary every response would share.
func commandPath(cmd *cobra.Command) string {
	return strings.TrimSpace(strings.TrimPrefix(cmd.CommandPath(), cmd.Root().Name()))
}

// cmdResult is what a converted command body hands back instead of printing.
//
// Text is the body's own terminal renderer, called only under --format text.
// It takes no arguments and closes over the *cobra.Command it was built in, so
// it writes to exactly the streams that command was given — which is how the
// in-process golden fixtures capture output, and how check keeps its
// stdout/stderr split (scan errors to stderr, everything else to stdout) pinned
// byte-for-byte.
type cmdResult struct {
	Data      any
	Warnings  []string
	StoppedAt string
	Text      func()

	// Command overrides the envelope's "command" field, which emit() otherwise
	// derives from the *cobra.Command it is rendering. It exists for the one
	// case where the two disagree: a FLAG on the root that answers as a verb
	// (--version), where commandPath(root) is the empty string but the caller
	// asked for "version" and has to be able to correlate the response with the
	// call. Every ordinary command leaves it zero and gets commandPath.
	Command string
}

// emittedErr marks an error whose rendering emit() has ALREADY done, so runCLI
// does not report it a second time. It keeps the cause reachable, because the
// exit-status decision at the top of main() still has to inspect it.
type emittedErr struct{ err error }

func (e *emittedErr) Error() string { return e.err.Error() }
func (e *emittedErr) Unwrap() error { return e.err }

// emit renders one command's outcome and is the ONLY place a converted command
// writes to stdout.
//
// Under --format text it replays the body's own printing and hands the error
// back untouched, so runCLI prints the same "Error: <message>" line cobra used
// to and the observable bytes are unchanged. Under --format json it writes
// exactly one envelope — to STDOUT, on failure as well as success, because a
// consumer that has to check two streams to find out what happened does not
// have a machine contract — and returns the error wrapped so nothing prints
// twice.
//
// A failing run still carries its partial Data and StoppedAt into the envelope.
// That is the entire point of stopped_at: a check that got as far as writing
// the viewer and then failed the impl-link scan has produced something real,
// and throwing it away would force a second full run to discover that.
func emit(cmd *cobra.Command, res cmdResult, runErr error) error {
	if !jsonOutput() {
		if res.Text != nil {
			res.Text()
		}
		return runErr
	}

	name := commandPath(cmd)
	if res.Command != "" {
		name = res.Command
	}
	env := cliout.Success(name, res.Data, res.Warnings)
	if runErr != nil {
		env = cliout.Failure(name, errorForCLI(runErr))
		// A failed run keeps whatever it managed to produce — see this
		// function's doc comment, and stopped_at's.
		env.Data = res.Data
		env.Warnings = res.Warnings
	}
	env.StoppedAt = res.StoppedAt
	if writeErr := cliout.Write(cmd.OutOrStdout(), env); writeErr != nil {
		// The envelope itself could not be written. There is nowhere left to
		// report that in the contract, so fall back to the stderr channel and
		// let the original outcome stand.
		fmt.Fprintf(cmd.ErrOrStderr(), "dossierx: could not write the output envelope: %v\n", writeErr)
	}
	if runErr == nil {
		return nil
	}
	return &emittedErr{err: runErr}
}

// envelopeRunE adapts a print-free command body to cobra's RunE signature. A
// converted command's wiring is therefore always the same one line, which is
// what keeps the contract uniform across nineteen leaves.
func envelopeRunE(body func(cmd *cobra.Command, args []string) (cmdResult, error)) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		res, err := body(cmd, args)
		return emit(cmd, res, err)
	}
}

// requireSubcommand is the RunE every NOUN carries — the command groups (claim,
// comment, build-order, skills) that exist only to hold leaves and do no work of
// their own.
//
// Without it, cobra's default for a parent with no Run/RunE is to print its help
// text and return nil. That breaks the machine contract in both halves at once:
// the bytes on stdout are help prose rather than the one envelope --format json
// promises, and the process exits 0, so an agent that checked only the status
// concludes its call succeeded. `dossierx claim` is not a successful claim
// operation; it is an incomplete invocation, and the contract has a code for
// exactly that.
//
// An unknown leaf reaches here too. Cobra's legacyArgs lets a non-root parent
// take arbitrary positional arguments, so "dossierx claim bogus" is not rejected
// as an unknown command — it arrives here with args, and naming what was
// actually typed is more useful than a generic complaint.
func requireSubcommand(cmd *cobra.Command, args []string) error {
	noun := commandPath(cmd)

	leaves := make([]string, 0, len(cmd.Commands()))
	for _, sub := range cmd.Commands() {
		if !sub.Hidden && sub.Name() != "help" {
			leaves = append(leaves, sub.Name())
		}
	}
	sort.Strings(leaves)
	available := strings.Join(leaves, ", ")

	if len(args) > 0 {
		return cliout.Errorf(cliout.CodeUsage,
			"%s: unknown subcommand %q; %s is a command group and does nothing on its own", noun, args[0], noun).
			WithHint(fmt.Sprintf("run one of: dossierx %s <%s>", noun, available))
	}
	return cliout.Errorf(cliout.CodeUsage,
		"%s: a subcommand is required; %s is a command group and does nothing on its own", noun, noun).
		WithHint(fmt.Sprintf("run one of: dossierx %s <%s>", noun, available))
}

// commandGroup stamps requireSubcommand onto a noun and returns it, so the
// wiring in each group's constructor is one line and no group can be added
// without it.
func commandGroup(cmd *cobra.Command) *cobra.Command {
	cmd.RunE = envelopeRunE(func(c *cobra.Command, args []string) (cmdResult, error) {
		return cmdResult{}, requireSubcommand(c, args)
	})
	// Cobra prints usage on a RunE error unless silenced, which would put help
	// prose back on the very stream the envelope owns.
	cmd.SilenceUsage = true
	return cmd
}

// dryRunResult wraps a completed dry-run report as the command's result.
//
// A dry run ALWAYS succeeds: the command was asked a question and answered it.
// "Blocked: true" is the answer, not a failure — it rides in data.blocked and
// leaves the exit status 0, so a caller can tell "I asked and the answer is no"
// apart from "the preview itself broke", which a non-zero exit would conflate.
//
// verb is the command's own name and is used only for the text rendering's
// prefix, keeping it consistent with each command's normal "<verb>: ..." lines.
func dryRunResult(cmd *cobra.Command, verb string, dr *cliout.DryRun) cmdResult {
	return cmdResult{
		Data: dr,
		Text: func() { writeDryRunText(cmd.OutOrStdout(), verb, dr) },
	}
}

// boolDetail picks a dry-run precondition's Detail from the condition it
// describes, so the sentence a reader sees always matches the verdict beside it.
//
// cliout.Precondition.Detail is emitted verbatim on BOTH branches — that is the
// whole point of reporting passing gates, since the passes are the evidence —
// and several call sites had written theirs for the failing branch only. The
// result was a preview that printed "[ok] thread_is_open: a resolved thread
// cannot take new replies", which reads as a contradiction of the verdict
// standing next to it, in exactly the document an agent shows a human to get a
// yes. whenTrue is the sentence for the condition holding, whenFalse for it not.
func boolDetail(cond bool, whenTrue, whenFalse string) string {
	if cond {
		return whenTrue
	}
	return whenFalse
}

// writeDryRunText renders a dry run for a human reading the terminal. It exists
// so the preview an agent shows its human is legible when pasted into chat;
// the JSON form is the contract, this is the courtesy.
func writeDryRunText(out io.Writer, verb string, dr *cliout.DryRun) {
	fmt.Fprintf(out, "%s --dry-run: %s\n", verb, dr.Would)
	if dr.From != "" || dr.To != "" {
		fmt.Fprintf(out, "  from: %s\n", dr.From)
		fmt.Fprintf(out, "  to:   %s\n", dr.To)
	}
	if len(dr.Preconditions) > 0 {
		fmt.Fprintln(out, "  preconditions:")
		for _, p := range dr.Preconditions {
			mark := "ok"
			if !p.OK {
				mark = "BLOCKED"
			}
			fmt.Fprintf(out, "    [%s] %s", mark, p.Name)
			if p.Detail != "" {
				fmt.Fprintf(out, ": %s", p.Detail)
			}
			fmt.Fprintln(out)
		}
	}
	if len(dr.Missing) > 0 {
		fmt.Fprintln(out, "  missing:")
		for _, m := range dr.Missing {
			fmt.Fprintf(out, "    %s\n", m)
		}
	}
	if len(dr.SideEffects) > 0 {
		fmt.Fprintln(out, "  side effects:")
		for _, e := range dr.SideEffects {
			fmt.Fprintf(out, "    %s\n", e)
		}
	}
	if len(dr.Proposed) > 0 {
		// Sorted so the same dry run prints the same bytes every time: a
		// preview a human is asked to approve must not shuffle between runs.
		keys := make([]string, 0, len(dr.Proposed))
		for k := range dr.Proposed {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fmt.Fprintln(out, "  proposed:")
		for _, k := range keys {
			fmt.Fprintf(out, "    %s = %v\n", k, dr.Proposed[k])
		}
	}
	fmt.Fprintf(out, "  blocked: %v\n", dr.Blocked)
}

// runCLI executes root and renders anything that escaped a converted command's
// emit(): a bad flag, the wrong argument count, an unknown subcommand, or an
// error from "serve" (the one text-only command).
//
// It exists because cobra's own error printing cannot serve both formats.
// Cobra prints "Error: <msg>" to stderr before Execute returns, which is
// correct for text and wrong for JSON — and the errors that arise BEFORE a RunE
// runs (flag parsing, argument validation) are precisely the ones no RunE
// wrapper can catch. So root.SilenceErrors is set unconditionally and this
// function reproduces cobra's exact line for text mode, or emits a proper
// failure envelope for JSON mode. Both main() and the in-process test helper
// go through here, so the two see identical output.
func runCLI(root *cobra.Command) error {
	cmd, err := root.ExecuteC()
	if err == nil {
		return nil
	}

	// Already rendered by emit(); reporting it again would duplicate the
	// envelope or the error line.
	var already *emittedErr
	if errors.As(err, &already) {
		return err
	}

	// cobra returns the command it resolved; on an unknown command there is
	// none and root is the right context to report against.
	target := cmd
	if target == nil {
		target = root
	}

	if jsonOutput() && !textOnly(target) {
		env := cliout.Failure(commandPath(target), usageErrorForCLI(err))
		if writeErr := cliout.Write(target.OutOrStdout(), env); writeErr != nil {
			fmt.Fprintf(root.ErrOrStderr(), "dossierx: could not write the output envelope: %v\n", writeErr)
		}
		return err
	}

	// Byte-identical to cobra's own SilenceErrors=false behavior: "Error:",
	// then the message, on the ROOT's stderr (which is what the golden
	// fixtures capture).
	fmt.Fprintln(root.ErrOrStderr(), "Error:", err.Error())
	return err
}

// usageErrorForCLI classifies an error that escaped WITHOUT passing through a
// converted command's emit().
//
// By construction there is only one family of those. Every converted command
// either attaches a code or wraps a sentinel, and every one of them returns
// through emit(), which marks the error as already-rendered. So an error
// arriving here that errorForCLI cannot classify — one that falls through to
// CodeInternal — is a cobra invocation error: an unknown command, an unknown
// flag, the wrong number of positional arguments. Reporting those as "internal"
// would tell an agent to file a bug when the correct response is to fix its own
// call.
func usageErrorForCLI(err error) *cliout.Error {
	e := errorForCLI(err)
	if e.Code == cliout.CodeInternal {
		e.Code = cliout.CodeUsage
	}
	return e
}

// errorForCLI decides the machine code, message, and exit status for an error
// on its way into an envelope.
//
// An explicitly attached code always wins: the call site that raised the
// failure knows more than this function ever can. Everything else is
// classified from the sentinel it wraps, which is the SAME set of sentinels
// internal/serve's writeOpError switches on — that is what makes "one language"
// literal rather than aspirational. Two surfaces, one switch shape, one
// vocabulary.
//
// The exit statuses this assigns follow README's documented families, not each
// call site's history: an id that does not exist is family 2 whether the caller
// said "lock" or "comment add". No new status is introduced — see
// cliout.ExitCode for why the three we have are enough now that error.code
// carries the detail.
func errorForCLI(err error) *cliout.Error {
	if e := cliout.As(err); e != nil {
		return e
	}

	code := cliout.CodeInternal
	switch {
	case errors.Is(err, config.ErrNotFound):
		code = cliout.CodeConfigNotFound
	case errors.Is(err, errClaimNotFound), errors.Is(err, comments.ErrClaimNotFound):
		code = cliout.CodeClaimNotFound
	case errors.Is(err, comments.ErrThreadNotFound):
		code = cliout.CodeThreadNotFound
	case errors.Is(err, comments.ErrReplyNotFound):
		code = cliout.CodeReplyNotFound
	case errors.Is(err, comments.ErrBannerClaim):
		code = cliout.CodeBannerClaim
	case errors.Is(err, comments.ErrEmptyBody):
		code = cliout.CodeEmptyBody
	case errors.Is(err, comments.ErrUnsafeBody):
		// Checked before ErrClaimNotRoundTrippable: the input pre-check wraps
		// the loader's sentinel in some paths, and "your body is unstorable" is
		// the more actionable of the two when both match.
		code = cliout.CodeUnsafeBody
	case errors.Is(err, loader.ErrClaimNotRoundTrippable):
		code = cliout.CodeClaimNotSerializable
	case errors.Is(err, comments.ErrInvalidActor):
		code = cliout.CodeInvalidActor
	case errors.Is(err, comments.ErrRightsDenied):
		code = cliout.CodeRightsDenied
	case errors.Is(err, comments.ErrThreadResolved):
		code = cliout.CodeThreadResolved
	case errors.Is(err, comments.ErrThreadOpen):
		code = cliout.CodeThreadOpen
	case errors.Is(err, comments.ErrCommentDigestDrift):
		code = cliout.CodeCommentDigestDrift
	case errors.Is(err, loader.ErrClaimFileChanged):
		code = cliout.CodeClaimFileChanged
	case errors.Is(err, buildorder.ErrNotProposed):
		code = cliout.CodeNotProposed
	case errors.Is(err, implink.ErrNoArtifact):
		code = cliout.CodeNoArtifact
	case errors.Is(err, errWrongState):
		code = cliout.CodeWrongState
	case errors.Is(err, lock.ErrPreLedgerUnadopted):
		// The fail-closed pre-ledger refusal. It is classified here as well as at
		// its call site so that any path which surfaces the sentinel without
		// attaching a code still reports something an agent can branch on:
		// `internal` for this state would say "file a bug" about a project that
		// needs one documented sequence run once. See
		// cliout.CodePreLedgerUnadopted.
		code = cliout.CodePreLedgerUnadopted
	case errors.Is(err, lock.ErrLedgerRecordDeleted):
		// A lock refused because the claim's ledger record was DELETED. It is
		// integrity_failed rather than a code of its own because it is the same
		// condition `check` reports as lock-ledger-deleted and it has that
		// family's recovery exactly: restore from version control, never
		// re-lock. Giving it a bespoke code would invite an agent to look for a
		// bespoke fix, and the whole point of this refusal is that there is no
		// command that clears it.
		code = cliout.CodeIntegrityFailed
	case errors.Is(err, lock.ErrCommentDigestUnrecorded):
		// Same family, same reason as ErrLedgerRecordDeleted above: `check`
		// reports this state as comment-digest-unrecorded, and its recovery is
		// version control rather than any command.
		code = cliout.CodeIntegrityFailed
	}
	return &cliout.Error{Code: code, Message: err.Error()}
}

// exitStatusFor maps a failed run to a process exit status, preserving every
// mapping DossierX has ever documented or pinned.
//
// The coded path is consulted first (a call site may pin a status), then the
// code's default family, and the errors.Is checks at the bottom are the
// belt-and-braces reproduction of the original main(): they are redundant with
// errorForCLI today and stay because they are cheap and they are what the
// pinned tests actually assert.
func exitStatusFor(err error) int {
	if err == nil {
		return 0
	}
	if e := errorForCLI(err); e != nil {
		if status := e.ExitStatus(); status != 0 {
			return status
		}
	}
	if errors.Is(err, config.ErrNotFound) ||
		errors.Is(err, errClaimNotFound) ||
		errors.Is(err, errWrongState) {
		return 2
	}
	return 1
}

// requireReason enforces --reason on the four verbs that change what the
// project treats as approved: claim lock, claim unlock, claim reaudit
// --confirm, and build-order lock.
//
// verb is the full command path, and it names the failure. The HINT is looked up
// in reasonInvocations rather than composed from verb, because composing it was
// wrong for all four callers: this used to print `run: dossierx <verb> --reason
// "…"`, and `claim lock`, `claim unlock` and `claim reaudit` are each declared
// cobra.ExactArgs(1) while `build-order lock` refuses without --module, so every
// one of those four lines named an invocation that exits non-zero before it could
// reach the missing --reason. internal/cliout's WithHint doc says a hint is "a
// literal next command to run"; a shape the reader has to repair first is not one.
//
// The reason is not paperwork. Under the v0.3.0 split the human never types
// these commands — they say "good, lock it" in chat and the agent executes —
// so --reason is where the human's own approving words are carried into the
// record. Nothing here can PREVENT an agent inventing a reason; what a required
// free-text field buys is that an unprompted lifecycle action has to fabricate
// an approval in writing rather than simply happen quietly, which is the
// difference between a convention that can be audited and one that cannot.
// Phase 3 persists this string into the lock ledger; Phase 1 requires it and
// echoes it back so the record it will be written into already exists.
// reasonInvocations is, for each verb that calls requireReason, the WHOLE
// invocation that verb accepts, minus --reason. Every required argument the verb
// declares is present, because that is the difference between a hint and a shape:
// `dossierx claim lock --reason "…"` exits 1 on the missing positional id long
// before --reason is looked at.
//
// The four keys are the four call sites, and there are no others:
// cmd/dossierx/main.go's claim lock, claim unlock and claim reaudit, and
// cmd/dossierx/build_order.go's build-order lock. Adding a fifth caller without
// adding its entry here is handled below rather than left to print the old wrong
// shape.
var reasonInvocations = map[string]string{
	"claim lock":       "dossierx claim lock <id>",
	"claim unlock":     "dossierx claim unlock <id>",
	"claim reaudit":    "dossierx claim reaudit <id> --confirm",
	"build-order lock": "dossierx build-order lock --module <module>",
}

func requireReason(verb, reason string) error {
	if strings.TrimSpace(reason) != "" {
		return nil
	}
	err := cliout.Errorf(cliout.CodeMissingFlag,
		"%s: --reason is required and must be non-empty; it records the human approval this action is executing", verb)
	invocation, known := reasonInvocations[verb]
	if !known {
		// A verb with no entry has no hint. That is deliberate: guessing
		// `dossierx <verb> --reason "…"` is how this defect existed in the first
		// place, and an absent hint sends the reader to --help, which is right,
		// where a wrong one sends them to a command that fails.
		return err
	}
	return err.WithHint(fmt.Sprintf(`run: %s --reason "<the approving words>"`, invocation))
}
