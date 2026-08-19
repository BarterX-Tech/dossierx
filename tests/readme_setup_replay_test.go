// readme_setup_replay_test.go replays README's "Start here" paste block — the
// eight numbered steps a user hands to their agent to set DossierX up in a
// consuming repository — against a real fixture repo, once for each answer to
// the block's one decision point: "ASK ME before installing the git pre-commit
// hook."
//
// WHY A REPLAY AND NOT A READING. The paste block is the single most-followed
// document in the repository: it is executed, verbatim, by an agent inside a
// project this repository will never see, and every sentence in it is an
// instruction rather than a description. A reading agent judging it can decide
// the words are plausible; only executing them decides whether the state they
// leave behind is the state the rest of README assumes. The specific gap this
// suite exists to catch was found by reading and is pinned by running: the
// block's step 4 fetches the CI workflow ONLY on the branch where the human
// DECLINES the hook, so the nudged answer — yes to the hook — ends the
// transcript with the local, skippable gate installed and the authority never
// set up, while the sentence "CI is the authority either way" sits inside the
// branch that was not taken. README's own "Where the gate runs" section says of
// CI: "If you adopt only one of the two, adopt this one."
//
// WHAT IS REPLAYED AND WHAT IS SUBSTITUTED. The steps run in order, against a
// git-initialized temp repo, with two substitutions both forced by "no network
// in tests":
//
//   - `go install github.com/BarterX-Tech/dossierx/cmd/dossierx@vX.Y.Z` is
//     replaced by the binary cli_test.go's TestMain already built from THIS
//     tree. NOT VERIFIED, therefore: that the published module path resolves,
//     that the pinned tag exists, or that what `go install` would fetch matches
//     this tree. The pin's coherence with the release the tree declares is
//     derived_facts_test.go's C4; the published artifact is the release gate's
//     subject, not a test's.
//   - every https://raw.githubusercontent.com/... fetch is replaced by the
//     file at the same repo-relative path in THIS tree (the URL's path names
//     it: scripts/install-git-hook.sh), and "add the CI workflow" is realized
//     from the local scripts/ci/dossierx-check.yml template. NOT VERIFIED,
//     therefore: that the raw URLs resolve at the pinned tag, or that the
//     bytes published at that tag match this tree's copy — a consumer follows
//     the URL, this test follows the tree, and only the release process can
//     make those the same bytes.
//
// The block is located by its ANCHOR HEADING, never as "the first fenced block":
// README carries several fenced blocks (`dossierx serve`, the CLI surface, the
// envelope example, the crossing), and a positional pick would silently start
// replaying one of those the day a fence is added above the paste block.
package tests

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// locating the paste block
// ---------------------------------------------------------------------------

// readmePasteBlock returns the text inside the paste block under README's
// "Start here" heading. Everything about the lookup fails loudly: a missing
// heading, a heading with no fenced block, or a section carrying more than one
// fence each mean the replay has lost its subject, and a replay over the wrong
// block would be worse than no replay — it would green-light a document it
// never read.
func readmePasteBlock(t *testing.T) string {
	t.Helper()
	readme := readRepoFile(t, "README.md")

	// Matched on the stable prefix rather than the full heading so a reworded
	// tail ("— paste this to your agent") does not orphan the replay.
	const anchor = "\n## Start here"
	i := strings.Index(readme, anchor)
	if i < 0 {
		t.Fatal(`README.md no longer carries a "## Start here" heading; the paste block this suite replays has moved or gone, and the replay must be repointed at whatever replaced it`)
	}
	section := readme[i+1:]
	if j := strings.Index(section, "\n## "); j >= 0 {
		section = section[:j]
	}

	const fence = "```"
	first := strings.Index(section, fence)
	if first < 0 {
		t.Fatal(`README.md's "Start here" section carries no fenced block; there is nothing to replay`)
	}
	open := section[first:]
	// Skip past the info string ("```text") to the block body.
	nl := strings.Index(open, "\n")
	if nl < 0 {
		t.Fatal(`README.md's "Start here" fence has no body`)
	}
	body := open[nl+1:]
	closing := strings.Index(body, fence)
	if closing < 0 {
		t.Fatal(`README.md's "Start here" fenced block is unterminated`)
	}
	// One fence pair and no more: a second fenced block in the section would
	// make "the block under the heading" ambiguous, which is the positional
	// drift this lookup exists to refuse.
	if strings.Contains(body[closing+len(fence):], fence) {
		t.Fatal(`README.md's "Start here" section now carries more than one fenced block, so "the paste block under the heading" is ambiguous; disambiguate the lookup deliberately rather than letting it pick one`)
	}
	return body[:closing]
}

// pasteBlockStep returns step n of the block — the text between "n. " and the
// next numbered step (or the end of the numbered list for the final step).
func pasteBlockStep(t *testing.T, block string, n int) string {
	t.Helper()
	marker := "\n" + strconv.Itoa(n) + ". "
	i := strings.Index(block, marker)
	if i < 0 {
		t.Fatalf("the paste block no longer carries a step %d; the numbered procedure this replay follows has been restructured, and the replay must be restructured with it", n)
	}
	rest := block[i+len(marker):]
	next := "\n" + strconv.Itoa(n+1) + ". "
	if j := strings.Index(rest, next); j >= 0 {
		return rest[:j]
	}
	// The final numbered step is followed by the free-prose coda, which starts
	// at the first blank line.
	if j := strings.Index(rest, "\n\n"); j >= 0 {
		return rest[:j]
	}
	return rest
}

// ---------------------------------------------------------------------------
// the replay
// ---------------------------------------------------------------------------

// ciWorkflowDestination is where the CI template tells a consumer to put
// itself, read from the template's own header ("COPY THIS FILE INTO YOUR
// REPOSITORY at .github/workflows/dossierx-check.yml") rather than restated
// here — the template names its destination once, and a replay that hardcoded a
// second copy of it would drift exactly the way this suite exists to catch.
func ciWorkflowDestination(t *testing.T) string {
	t.Helper()
	template := readRepoFile(t, filepath.Join("scripts", "ci", "dossierx-check.yml"))
	re := regexp.MustCompile(`at (\.github/workflows/[\w.-]+\.ya?ml)`)
	m := re.FindStringSubmatch(template)
	if m == nil {
		t.Fatal("scripts/ci/dossierx-check.yml's header no longer names its own destination path; the replay cannot know where a consumer is told to install it")
	}
	return m[1]
}

// gitInConsumer runs one git command inside the fixture repo, fatally: every
// git step here is scaffolding the replay depends on, never the thing under
// test.
func gitInConsumer(t *testing.T, consumer string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", consumer}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s in the fixture repo: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// rawFetchRE matches the raw.githubusercontent.com URLs the paste block tells
// the agent to fetch, capturing the repo-relative path after the version
// segment — which is what lets the network fetch be substituted with this
// tree's own copy of the same file.
var rawFetchRE = regexp.MustCompile(`https://raw\.githubusercontent\.com/[^/\s]+/[^/\s]+/v\d+\.\d+\.\d+/(\S+)`)

// shCommandRE matches an inline `sh <script> <flags>` command the block tells
// the agent to run after a fetch.
var shCommandRE = regexp.MustCompile("`sh ([^`]+)`")

// ciWorkflowMentionRE matches an instruction to add the CI workflow. Matched as
// a phrase rather than a filename because the block speaks to an agent in
// prose; README's "Where the gate runs" section and the template's own header
// define what "add the CI workflow" means (copy scripts/ci/dossierx-check.yml
// to the destination the template names).
var ciWorkflowMentionRE = regexp.MustCompile(`(?i)add the CI workflow`)

// replayDecisionBranch executes, in the order the text gives them, every
// action one branch of step 4 instructs: raw-URL fetches (substituted, see the
// package comment), `sh ...` invocations, and CI-workflow installation. The
// interpreter is driven by the TEXT, not by which branch the caller believes it
// is running: if a future README fixes the yes branch by adding "then add the
// CI workflow as well", this replay starts installing it with no test change —
// and until it does, the yes branch executes exactly what it says, which is the
// point. A branch whose text yields no executable action at all is fatal: the
// decision point still exists but this replay can no longer read it, and a
// silent empty replay would pass over zero actions.
func replayDecisionBranch(t *testing.T, consumer, branchText string) {
	t.Helper()

	type action struct {
		pos int
		run func()
	}
	var actions []action

	for _, m := range rawFetchRE.FindAllStringSubmatchIndex(branchText, -1) {
		rel := branchText[m[2]:m[3]]
		pos := m[0]
		actions = append(actions, action{pos, func() {
			// SUBSTITUTION: the fetch is served from this tree, not the
			// network. The URL's path names the repo-relative file, so this
			// verifies the block points at a file that exists here — and does
			// NOT verify that the URL resolves at its pinned tag or that the
			// published bytes match these.
			body := readRepoFile(t, filepath.FromSlash(rel))
			dst := filepath.Join(consumer, filepath.Base(rel))
			if err := os.WriteFile(dst, []byte(body), 0o644); err != nil {
				t.Fatalf("substituted fetch of %s into the fixture repo: %v", rel, err)
			}
		}})
	}

	for _, m := range shCommandRE.FindAllStringSubmatchIndex(branchText, -1) {
		args := strings.Fields(branchText[m[2]:m[3]])
		pos := m[0]
		actions = append(actions, action{pos, func() {
			cmd := exec.Command("sh", args...)
			cmd.Dir = consumer
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("the paste block's `sh %s` failed in the fixture repo: %v\n%s", strings.Join(args, " "), err, out)
			}
		}})
	}

	if m := ciWorkflowMentionRE.FindStringIndex(branchText); m != nil {
		actions = append(actions, action{m[0], func() {
			dst := filepath.Join(consumer, filepath.FromSlash(ciWorkflowDestination(t)))
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				t.Fatalf("mkdir for the CI workflow in the fixture repo: %v", err)
			}
			body := readRepoFile(t, filepath.Join("scripts", "ci", "dossierx-check.yml"))
			if err := os.WriteFile(dst, []byte(body), 0o644); err != nil {
				t.Fatalf("installing the CI template into the fixture repo: %v", err)
			}
		}})
	}

	if len(actions) == 0 {
		t.Fatalf("no executable action could be read out of this branch of the paste block's step 4; the replay has lost the ability to follow the text and must be widened deliberately rather than passing empty:\n%s", branchText)
	}
	sort.Slice(actions, func(i, j int) bool { return actions[i].pos < actions[j].pos })
	for _, a := range actions {
		a.run()
	}
}

// replayPasteBlock runs the block's numbered steps, in order, in a fresh
// git-initialized repo, answering the step-4 question with sayYes. It returns
// the consumer repo's root with the transcript's terminal state on disk, for
// the caller to assert postconditions against.
func replayPasteBlock(t *testing.T, module string, sayYes bool) string {
	t.Helper()
	block := readmePasteBlock(t)

	consumer := t.TempDir()
	gitInConsumer(t, consumer, "init")
	gitInConsumer(t, consumer, "config", "user.email", "replay@example.invalid")
	gitInConsumer(t, consumer, "config", "user.name", "Paste Block Replay")

	// STEP 1 — install the binary, then run `dossierx version`. The install is
	// the substituted half (see the package comment: binPath is built from this
	// tree by TestMain); the version invocation the step also demands is real.
	if _, stderr, code := run(t, consumer, "version"); code != 0 {
		t.Fatalf("step 1's `dossierx version` exited %d: %s", code, stderr)
	}

	// STEP 2 — the skills export, with the directory read from the block rather
	// than assumed, because the block's whole argument for naming one ("it runs
	// before the config exists") is a behavior worth replaying as written.
	exportRE := regexp.MustCompile("`dossierx skills export ([^`\\s]+)`")
	m := exportRE.FindStringSubmatch(pasteBlockStep(t, block, 2))
	if m == nil {
		t.Fatal("step 2 of the paste block no longer names a `dossierx skills export <dir>` command; the replay cannot follow it")
	}
	if _, stderr, code := run(t, consumer, "skills", "export", m[1]); code != 0 {
		t.Fatalf("step 2's `dossierx skills export %s` exited %d in a repo with no project yet: %s", m[1], code, stderr)
	}

	// STEP 3 — the block has the agent PROPOSE a config and WAIT for the human.
	// The test plays both parties: the proposal is a minimal valid project, and
	// the human said yes.
	writeFixtureProject(t, consumer, module)

	// STEP 4 — the decision point. The two branches are located by their own
	// phrasing; losing either one means the decision point this suite exists to
	// replay is gone, which is fatal, not a smaller test.
	step4 := pasteBlockStep(t, block, 4)
	yesIdx := strings.Index(step4, "If I say yes")
	noIdx := strings.Index(step4, "If I say no")
	if yesIdx < 0 || noIdx < 0 || noIdx < yesIdx {
		t.Fatalf("step 4 of the paste block no longer reads as an \"If I say yes … If I say no …\" decision; the replay cannot tell the branches apart:\n%s", step4)
	}
	if sayYes {
		replayDecisionBranch(t, consumer, step4[yesIdx:noIdx])
	} else {
		replayDecisionBranch(t, consumer, step4[noIdx:])
	}

	// STEP 5 — conditional on `lock-ledger-pre-ledger`, and the block itself
	// says of a project created at step 3: "this never fires — say you skipped
	// it". Nothing is executed, and the claim that it never fires is not taken
	// on faith: step 6 requires plain `check` to exit 0, which it cannot do
	// over a pre-ledger finding.

	// STEP 6 — `dossierx check --format text`, exiting 0, shown rather than
	// asserted by the agent. (run() itself passes --format text.)
	if stdout, stderr, code := run(t, consumer, "check"); code != 0 {
		t.Fatalf("step 6's `dossierx check --format text` exited %d over the project the block's own steps produced:\n%s%s", code, stdout, stderr)
	}

	// STEPS 7 and 8 are instructions to the human (what to commit, what to
	// run); the commit-timing half of step 7's subject matter is pinned by
	// TestREADME_DigestStoreCommitTimingMatchesTheEngine below.
	return consumer
}

// assertHookInstalled resolves the fixture repo's hook directory the same way
// the installer does (git rev-parse --git-path hooks, which is also correct in
// linked worktrees) and reports whether a pre-commit hook exists there.
func hookInstalled(t *testing.T, consumer string) bool {
	t.Helper()
	cmd := exec.Command("git", "-C", consumer, "rev-parse", "--git-path", "hooks")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("resolving the fixture repo's hooks path: %v", err)
	}
	hooks := strings.TrimSpace(string(out))
	if !filepath.IsAbs(hooks) {
		hooks = filepath.Join(consumer, hooks)
	}
	_, statErr := os.Stat(filepath.Join(hooks, "pre-commit"))
	return statErr == nil
}

// requireCIPostconditionStated fails the suite fatally if README no longer
// states the requirement these tests enforce. The postcondition is not this
// test's invention: it is README's, twice over, and if both statements vanish
// the tests below would be enforcing a sentence nobody wrote — the right
// response to that is a loud failure demanding the tests be re-derived, never
// a quiet pass.
func requireCIPostconditionStated(t *testing.T) {
	t.Helper()
	readme := readRepoFile(t, "README.md")
	const inBlock = "CI is the authority either way"
	const inGateSection = "adopt only one of the two, adopt this one"
	if !strings.Contains(readme, inBlock) && !strings.Contains(readme, inGateSection) {
		t.Fatalf("README no longer states that CI is the authority (%q / %q are both gone), so the postcondition this replay asserts has lost its written source; re-derive the requirement before trusting either branch's result", inBlock, inGateSection)
	}
}

// assertCIWorkflowArrived is the terminal postcondition for BOTH answers to
// step 4, and the sentence that makes it the requirement is the paste block's
// own: "CI is the authority either way." README's "Where the gate runs" section
// says the same thing at length — git never runs pre-commit for a clean merge,
// a rebase, a cherry-pick or a revert, `--no-verify` is one keystroke away —
// and concludes: "If you adopt only one of the two, adopt this one." A
// transcript that ends without the CI workflow has therefore ended without the
// one gate README says a consumer must not be without, whatever they answered
// about the hook.
func assertCIWorkflowArrived(t *testing.T, consumer, answered string) {
	t.Helper()
	dst := ciWorkflowDestination(t)
	if _, err := os.Stat(filepath.Join(consumer, filepath.FromSlash(dst))); err != nil {
		t.Errorf("the paste block's transcript ended with no CI workflow at %s after the human answered %q to the hook question.\n\n"+
			"The block says \"CI is the authority either way\", and README's \"Where the gate runs\" section says of the CI "+
			"workflow: \"If you adopt only one of the two, adopt this one.\" A consumer who takes this branch of step 4 ends "+
			"setup with only the local, skippable gate — the exact configuration those sentences exist to warn against — and "+
			"the sentence promising otherwise sits inside the branch they did not take. Fix the block so BOTH answers leave "+
			"the workflow installed; the hook question should decide whether the hook is added, never whether CI is.",
			dst, answered)
	}
}

// TestPasteBlockYesToHookStillEndsWithCI replays the transcript where the human
// accepts the pre-commit hook — the answer the block's own framing nudges
// toward, since the hook is what step 4 is ABOUT. This is the branch with the
// known defect: the CI template is fetched only inside "If I say no", so saying
// yes leaves the consumer with the gate git skips on every merge, rebase,
// cherry-pick, revert and --no-verify, and no authority behind it.
func TestPasteBlockYesToHookStillEndsWithCI(t *testing.T) {
	requireCIPostconditionStated(t)
	consumer := replayPasteBlock(t, "pasteyes", true)

	// The branch's own promise must hold before the shared postcondition is
	// judged, or a broken installer would masquerade as the CI defect.
	if !hookInstalled(t, consumer) {
		t.Fatal("the yes branch ran to completion and the pre-commit hook is not installed; the replay of `sh install-git-hook.sh --yes` did not do what the block says it does, so nothing below this can be trusted")
	}
	assertCIWorkflowArrived(t, consumer, "yes")
}

// TestPasteBlockNoToHookEndsWithCI replays the declining transcript, which is
// the branch the block already handles: "If I say no, add the CI workflow
// instead". It exists both as the control for the yes branch and as its own
// regression pin — a future edit that moves the CI instruction OUT of step 4
// entirely breaks this branch first.
func TestPasteBlockNoToHookEndsWithCI(t *testing.T) {
	requireCIPostconditionStated(t)
	consumer := replayPasteBlock(t, "pasteno", false)

	// The human said no; a hook installed anyway would mean the replay (or the
	// block) stopped honoring the answer, which is a different defect and worth
	// its own line.
	if hookInstalled(t, consumer) {
		t.Error("the human answered no to the hook question and the transcript installed the pre-commit hook anyway")
	}
	assertCIWorkflowArrived(t, consumer, "no")
}

// ---------------------------------------------------------------------------
// the digest store's commit timing, pinned against the engine
// ---------------------------------------------------------------------------

// TestREADME_DigestStoreCommitTimingMatchesTheEngine pins README's instruction
// for WHEN to commit `.dossierx-comment-digest.json` against when the engine
// actually creates it.
//
// README's tracked-artifacts paragraph says: commit "the lock store the moment
// anything is locked, the digest once anyone comments". The second clause is
// false about the engine in a way that costs the reader a red gate: the digest
// store is created EMPTY by the FIRST LOCK (internal/lock — Store.Save creates
// it in the same act that creates a lock store for a fresh project, and
// CrossPreLedger does the same at the crossing; README's own crossing section
// says so: the first lock "creates `.dossierx-comment-digest.json` in the same
// act"). A reader who does what the paragraph says — waits for a comment —
// commits the lock store without the digest store beside it, and the very next
// `check --staged` (the pre-commit hook's own invocation) reports
// `comment-digest-absent` against a project where nobody has commented and
// nothing is wrong.
//
// Both halves are established here mechanically, in the docs_site_audit style
// of asserting file existence and rule strings rather than message prose:
// first that the lock creates the store with zero comments in the project,
// then that the waiting reader's index draws the finding. Only then is the
// sentence judged. The prose assertion is one-directional on purpose: it
// forbids the comment-time claim and does not dictate the replacement wording,
// because "commit it with the lock store" and simply deleting the wrong timing
// clause are both truthful fixes and this test has no business choosing.
func TestREADME_DigestStoreCommitTimingMatchesTheEngine(t *testing.T) {
	root := t.TempDir()
	writeFixtureProject(t, root, "digesttm")
	gitInConsumer(t, root, "init")
	gitInConsumer(t, root, "config", "user.email", "replay@example.invalid")
	gitInConsumer(t, root, "config", "user.name", "Digest Timing Replay")
	gitInConsumer(t, root, "add", "-A")
	gitInConsumer(t, root, "commit", "-m", "project setup, nothing locked")

	// The first lock in the project. Nobody has commented, and nobody will.
	if _, stderr, code := run(t, root, "claim", "lock", "digesttm.contract.overview", "--reason", "digest-timing fixture"); code != 0 {
		t.Fatalf("claim lock exited %d: %s", code, stderr)
	}

	// Half one — the engine's side: the digest store exists NOW, created by the
	// lock, with zero comments anywhere. If this ever fails, the engine moved
	// and it is README's "created in the same act" crossing paragraph that
	// needs re-verifying, not this assertion loosening.
	digestPath := filepath.Join(root, ".dossierx-comment-digest.json")
	if _, err := os.Stat(digestPath); err != nil {
		t.Fatalf(".dossierx-comment-digest.json does not exist after the project's first lock (%v); the engine no longer creates it at lock time, and README's crossing section (\"creates `.dossierx-comment-digest.json` in the same act\") now needs re-verifying alongside the timing sentence this test pins", err)
	}

	// Half two — the reader's side. Follow the tracked-artifacts sentence
	// literally: the lock store goes in "the moment anything is locked"; the
	// digest waits for a comment that has not happened. Stage exactly that.
	gitInConsumer(t, root, "add", "claims", ".dossierx-lock-store.json")

	stdout, _, code := run(t, root, "check", "--staged", "--format", "json")
	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			LedgerFindings []struct {
				Rule string `json:"rule"`
			} `json:"ledger_findings"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("check --staged --format json produced an undecodable envelope: %v\n%s", err, stdout)
	}
	absent := false
	for _, f := range envelope.Data.LedgerFindings {
		if f.Rule == "comment-digest-absent" {
			absent = true
		}
	}
	if code == 0 || envelope.OK || !absent {
		t.Fatalf("staging the lock store without the digest store did NOT draw comment-digest-absent (exit %d, ok=%v, findings=%+v); the engine's behavior has moved, and the README sentence this test pins must be re-judged against whatever it does now rather than against this reproduction", code, envelope.OK, envelope.Data.LedgerFindings)
	}

	// The behavior is established: created by the first lock, and a red gate
	// for whoever waits. Now the sentence.
	readme := readRepoFile(t, "README.md")
	const paragraphAnchor = "**All three are tracked artifacts."
	if !strings.Contains(readme, paragraphAnchor) {
		t.Fatalf("README.md no longer carries the %q paragraph; the commit-timing instruction this test pins has moved, and the pin must move with it", paragraphAnchor)
	}
	const commentTimeClaim = "the digest once anyone comments"
	if strings.Contains(readme, commentTimeClaim) {
		t.Errorf("README tells the reader to commit the digest store %q, and the engine creates it at the FIRST LOCK — reproduced above: in a project where nobody has ever commented, locking one claim created .dossierx-comment-digest.json, and an index holding the lock store without it fails `check --staged` with `comment-digest-absent`. The pre-commit hook runs exactly that command, so the reader who waits as instructed is stopped by the gate on their next commit with a finding about a store the instruction told them not to commit yet. Tie the digest store's commit timing to the lock that creates it (its own crossing section already says \"in the same act\"), or drop the timing clause", commentTimeClaim)
	}
}
