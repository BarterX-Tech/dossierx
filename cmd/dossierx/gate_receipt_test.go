// gate_receipt_test.go is the G1->G2 handshake: the RECEIPT a gate run records
// for the tree it verified, and the checks that make that receipt mean
// something at merge time.
//
// WHY A RECEIPT AT ALL. A verdict is worthless if the thing tagged is not the
// thing verified. G1 reads one tree and produces findings; the merge happens
// later, by a human, and nothing in between obliges the content to be the same.
// So G1 records `git rev-parse HEAD^{tree}` for the tree it read, alongside its
// findings, and G2 asserts the merge commit's tree IS that tree. A mismatch
// means the merge carries content G1 never read, and the verdict does not cover
// it.
//
// THE PRECONDITION THAT MAKES IT CONVERGE, and it is the load-bearing part. A
// `--no-ff` merge's tree equals the branch's tree ONLY when origin/main is
// already an ancestor of the branch head. Otherwise git performs a real
// three-way merge and the resulting tree carries content from main that G1 never
// saw. The handshake then fails, correctly — but re-running G1 against the same
// branch reproduces the IDENTICAL receipt, so the pipeline wedges with no way
// out and the only remaining moves are to switch the gate off or to tag
// something nobody read. gateRequireAncestor's failure therefore names the one
// move that clears it: merge origin/main into the release branch and re-trigger.
//
// WHERE IT IS ASSERTED, named precisely, because "twice" is easy to write and
// hard to check. There are three call sites and they are three different
// questions asked of three different moments:
//
//	gateG1Precondition        before G1's fan-out — the cheap refusal. A
//	                          diverged branch cannot produce a usable receipt
//	                          however good its findings are, so it is refused
//	                          before a read-only agent is spent per surface.
//	gateRecordReceipt         at record time — G1's last act. The fan-out takes
//	                          real time and origin/main can advance during it, so
//	                          the pre-flight answer is stale by the time there is
//	                          something to record. A receipt for a branch that
//	                          cannot converge is worse than no receipt.
//	gatePreMergePrecondition  immediately before the merge, by the driver. More
//	                          time passes between the receipt and the human's
//	                          authorization than passes inside a gate run, and
//	                          this is the last moment anything can refuse.
//
// The middle one is not the first one. Only gateG1Precondition runs before the
// fan-out is paid for, and only gatePreMergePrecondition is asked after the
// receipt exists — which is the only one that can catch a base ref that advanced
// while the release waited for a human.
//
// AND ALL THREE ARE WORTHLESS WITHOUT A FETCH, which is the correction that made
// the three of them mean anything at all. `origin/main` is a local
// remote-tracking ref: it is a file in this clone and it does not move because
// somebody pushed to the forge. Asked without refreshing it, `git merge-base
// --is-ancestor origin/main <branch>` reports on the state of main as of the last
// time anyone happened to fetch — so the two re-assertions above, whose entire
// justification is that main advances while the release waits, could not observe
// the thing they exist to observe. Worse than useless: with a stale local main
// the `--no-ff` merge really does produce the branch's tree, the handshake below
// really does pass, and the push is then rejected as non-fast-forward — after
// which the operator pulls and the three-way merge tree that results is the one
// that gets tagged, behind a receipt that already said PASS. That is precisely
// the failure the receipt exists to prevent, reached through the check meant to
// prevent it. gateRefreshBase therefore runs before every ancestry question, and
// a fetch that cannot run is errGateUncheckable rather than a satisfied
// precondition.
//
// The fetch is the one thing in this file that writes, and it writes exactly one
// thing: the remote-tracking ref for the base branch. No local branch moves, the
// work tree is untouched, and nothing the operator owns is rewritten — it is the
// minimum required to make the question be about the remote rather than about
// this clone's memory of it. docs/RELEASING.md's manual checklist item spells the
// same pair (`git fetch origin && git merge-base --is-ancestor origin/main
// <branch>`), and the two must not drift apart —
// TestReleaseProcedurePinsTheAncestryPrecondition below is what holds them
// together. That sentence stood here unasserted through a whole cycle, and a
// review deleted the checklist item outright with the package still green; since
// none of this is wired to a command yet, what it deleted was not a description
// of the precondition but the only performance of it.
//
// A CHECK THAT CANNOT RUN IS A FAILURE. `git merge-base --is-ancestor` answers
// "no" with exit 1 and "I could not tell" with anything else (an unknown ref
// exits 128, a missing git binary never starts). Those two are kept apart on
// purpose: collapsing them would let a typo'd base ref read as a clean negative
// or, worse, a swallowed error read as a pass over zero assertions.
//
// WHY IT IS TEST CODE. The same reason surface_test.go is: the gate is not a CLI
// feature. Nothing here is wired into a cobra command, nothing here is compiled
// into the shipped binary, and none of it moves surface.json's
// behaviour_fingerprint — a _test.go file is the shape that guarantees all
// three, and it is the emitter's own precedent.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------
// the receipt
// ---------------------------------------------------------------------

// gateBaseRef is the ref a release branch must already contain. It is
// origin/main and deliberately not "main": the merge that publishes lands on the
// REMOTE's main, and a local main that is behind it would make the precondition
// pass over a question nobody asked.
const gateBaseRef = "origin/main"

// The two verdicts. There is no third, and in particular there is nothing that
// means "we did not check" — a run that could not examine something reports
// FAILED, which is CLAUDE.md's rule stated as a constant set.
const (
	gateVerdictPass   = "PASS"
	gateVerdictFailed = "FAILED"
)

// gateReceipt is what a G1 run records about the tree it verified.
//
// It deliberately carries NO verdict field. A stored verdict is a second copy of
// something the findings and the surface verdicts already determine, and the two
// can disagree — which is precisely the failure mode this gate exists to remove
// one level up (a tag that disagrees with the tree that was approved). G2
// recomputes the verdict from the receipt's own contents through evaluate(), so
// there is nothing to forge and nothing to go stale.
type gateReceipt struct {
	// Gate names the run that produced this, so a receipt found on disk says
	// which half of the handshake wrote it.
	Gate string `json:"gate"`
	// Version is the release under verification.
	Version string `json:"version"`
	// Branch is the ref G1 was pointed at, and BaseRef the ref it had to
	// already contain. Both are recorded rather than assumed, because the
	// pre-merge re-assertion has to ask the identical question G1 asked.
	Branch  string `json:"branch"`
	BaseRef string `json:"base_ref"`
	// HeadCommit is context for a human reading the receipt; Tree is the thing
	// the handshake actually compares. They are separate because they answer
	// different questions: two different commits can carry the same tree (a
	// rebase, an amended message, the `--no-ff` merge itself), and it is the
	// CONTENT that was verified, not the commit that happened to hold it.
	HeadCommit string `json:"head_commit"`
	Tree       string `json:"tree"`

	Surfaces []gateSurfaceVerdict `json:"surfaces"`
	Findings []gateFinding        `json:"findings"`
}

// gateFinding is one thing the gate found.
//
// Severity is carried but is NOT what decides the verdict. CLAUDE.md's rule is
// that every finding reaches the human and the HUMAN confirms what blocks a
// release, so a receipt carrying any finding at all evaluates to FAILED and the
// override is the human's to record. Filtering by severity here would be an
// agent making that call alone.
type gateFinding struct {
	Surface  string `json:"surface"`
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	Detail   string `json:"detail"`
}

// ---------------------------------------------------------------------
// git, read-only
// ---------------------------------------------------------------------

// gateSHARE is what a resolved object name must look like. `git rev-parse`
// already exits non-zero on an unknown revision, so this is not the primary
// guard; it is here because an abbreviated or decorated answer (core.abbrev, a
// ref name echoed back) would compare unequal against a full sha and the failure
// would read as tree drift rather than as a bad read.
var gateSHARE = regexp.MustCompile(`^[0-9a-f]{40}$`)

// gateGit runs one READ-ONLY git command in dir and returns its trimmed stdout.
//
// It never falls back to a default. Every caller here treats "git could not
// answer" as a failed check, so an empty string that reads like an answer is the
// one return value this must not have.
func gateGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// gateResolve resolves rev to a full object name, refusing anything that is not
// one. See gateSHARE.
func gateResolve(dir, rev string) (string, error) {
	sha, err := gateGit(dir, "rev-parse", rev)
	if err != nil {
		return "", err
	}
	if !gateSHARE.MatchString(sha) {
		return "", fmt.Errorf("git rev-parse %s answered %q, which is not a full object name", rev, sha)
	}
	return sha, nil
}

// gateTreeSHA is the tree of rev — `git rev-parse <rev>^{tree}` — which is the
// one value the handshake compares.
func gateTreeSHA(dir, rev string) (string, error) {
	return gateResolve(dir, rev+"^{tree}")
}

// ---------------------------------------------------------------------
// the ancestry precondition
// ---------------------------------------------------------------------

// errGateNotAncestor is the ANSWER "no": the base ref is not contained in the
// branch. It is a sentinel because the caller has to be able to tell it apart
// from "the question could not be asked", which takes a different recovery — see
// the package comment.
var errGateNotAncestor = errors.New("the base ref is not an ancestor of the branch head")

// gateAncestryRecovery is the one move that clears a not-an-ancestor refusal,
// spelled as commands. It says explicitly that re-running the gate cannot help,
// because that is the move an operator reaches for first and it is the move that
// wedges the pipeline: the receipt is a function of the branch's tree, so a
// re-run against an unchanged branch reproduces it byte for byte.
const gateAncestryRecovery = "merge the base ref into the release branch and re-trigger the gate:\n" +
	"\tgit fetch origin && git merge origin/main\n" +
	"Re-running the gate against the branch as it stands cannot clear this — it reads the same tree and " +
	"reproduces the identical receipt — so the merge is the only way forward."

// gateRefreshBase brings the base ref up to date with the remote before anyone
// asks a question about it. See the package comment: without this, every
// ancestry answer below is about a ref that last moved whenever someone happened
// to fetch, and the two re-assertions that exist to catch "main advanced while
// the release waited" are no-ops that cannot fail.
//
// The refspec is written out in full rather than left to `git fetch origin main`
// and git's opportunistic tracking-ref update, because the whole point is that
// refs/remotes/<remote>/<branch> ends up current: a fetch that landed only in
// FETCH_HEAD would leave the ancestry question reading the same stale ref it
// reads today, and the failure would be invisible.
//
// Every failure here is errGateUncheckable, never nil. A base ref that could not
// be refreshed — no network, no such remote, a base that is not a
// remote-tracking ref at all — leaves the local ref saying whatever it said
// before, and answering from it would be CLAUDE.md's forbidden reading exactly:
// a definite "yes" derived from a ref nobody refreshed, presented as a check
// that ran.
func gateRefreshBase(dir, base string) error {
	remote, branch, ok := strings.Cut(base, "/")
	if !ok || remote == "" || branch == "" {
		return fmt.Errorf("%w: base ref %q is not a <remote>/<branch> remote-tracking ref, so it cannot be refreshed before the ancestry question is asked; "+
			"answering from an unrefreshed ref would report on this clone's memory of the base rather than on the base", errGateUncheckable, base)
	}
	refspec := fmt.Sprintf("+refs/heads/%s:refs/remotes/%s/%s", branch, remote, branch)
	if _, err := gateGit(dir, "fetch", "--quiet", remote, refspec); err != nil {
		return fmt.Errorf("%w: could not refresh %s from %s, so the ancestry question below would be answered from a remote-tracking ref that may be behind the remote; "+
			"treat it as a failed gate rather than as a satisfied precondition: %w", errGateUncheckable, base, remote, err)
	}
	return nil
}

// gateRequireAncestor asserts that base is already contained in branch, which is
// what makes a `--no-ff` merge a fast-forward-shaped merge: one parent's tree,
// unchanged, under a new commit. See the package comment for why the whole
// handshake rests on it.
//
// It REFRESHES base first. That is not an optimisation and not politeness to the
// operator's clone: an unrefreshed origin/main makes this function answer a
// question about local state that reads as a question about the remote, and the
// answer it gives is "yes" exactly when the release is about to go wrong.
//
// The four outcomes are kept distinct on purpose:
//
//	fetch fails -> the base could not be refreshed; errGateUncheckable
//	exit 0      -> contained; nil
//	exit 1      -> NOT contained; errGateNotAncestor, with the recovery
//	anything    -> the question could not be asked; a plain error, never nil
//
// The last two cases are the ones worth guarding. An unknown ref exits 128 and a
// missing git binary never starts, and either of those folded into the exit-1
// branch would report a definite "no" the operator would then try to fix by
// merging — or, folded into the nil branch, would wave the precondition through
// over a check that never ran.
func gateRequireAncestor(dir, base, branch string) error {
	if err := gateRefreshBase(dir, base); err != nil {
		return err
	}
	cmd := exec.Command("git", "merge-base", "--is-ancestor", base, branch)
	cmd.Dir = dir
	err := cmd.Run()
	if err == nil {
		return nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return fmt.Errorf("%w: %s is not contained in %s, so a --no-ff merge would be a real three-way merge "+
			"carrying content from %s that the gate never read.\n%s",
			errGateNotAncestor, base, branch, base, gateAncestryRecovery)
	}
	return fmt.Errorf("could not determine whether %s is an ancestor of %s: git merge-base --is-ancestor did not answer: %w",
		base, branch, err)
}

// ---------------------------------------------------------------------
// recording, and the assertions around it
// ---------------------------------------------------------------------

// gateG1Precondition is the FIRST assertion, made before G1 fans a read-only
// agent out per declared surface.
//
// It buys nothing the record-time check does not also catch, and it is here
// anyway, for cost: a diverged branch cannot produce a receipt the merge can
// honour, so every surface agent the fan-out would spend on it is spent on a
// verdict that is going to be thrown away. Refusing at the door turns a
// thirteen-way fan-out into one `git merge-base` call and one message naming
// the merge that clears it.
func gateG1Precondition(dir, branch string) error {
	if err := gateRequireAncestor(dir, gateBaseRef, branch); err != nil {
		return fmt.Errorf("gate precondition: %w", err)
	}
	return nil
}

// gateRecordReceipt is G1's last act: it asserts the ancestry precondition, then
// records the branch head and the tree it verified alongside the run's surface
// verdicts and findings.
//
// The precondition is checked FIRST and the receipt is not written when it
// fails. A receipt for a branch that cannot converge is worse than no receipt:
// it is a document that will be compared, will disagree, and will be blamed on
// the merge rather than on the branch.
//
// This re-asks the question gateG1Precondition already asked at the door rather
// than trusting its answer, because the fan-out sits between them and origin/main
// can advance while it runs. The pre-flight check is the cheap refusal; this one
// is the correct one.
//
// The verdict list is then checked for a surface reported twice, which is the
// earliest point that defect can be caught — a duplicate that gets as far as
// disk is a duplicate every later reader has to re-detect. See
// errGateDuplicateVerdict for why it is refused rather than resolved.
func gateRecordReceipt(dir, version, branch string, surfaces []gateSurfaceVerdict, findings []gateFinding) (gateReceipt, error) {
	if err := gateRequireAncestor(dir, gateBaseRef, branch); err != nil {
		return gateReceipt{}, fmt.Errorf("gate precondition: %w", err)
	}
	if _, err := gateIndexVerdicts(surfaces); err != nil {
		return gateReceipt{}, fmt.Errorf("gate receipt: %w", err)
	}
	head, err := gateResolve(dir, branch)
	if err != nil {
		return gateReceipt{}, err
	}
	tree, err := gateTreeSHA(dir, branch)
	if err != nil {
		return gateReceipt{}, err
	}

	// Both comparators are TOTAL orders over the fields their records carry, so
	// the receipt is a function of the SET it was handed and not of the order the
	// fan-out happened to return in. That matters because the receipt's whole
	// value is being reproducible: a re-run over an unchanged tree has to produce
	// the identical document, and a comparator that leaves ties — two findings
	// against the same surface and rule, which is an ordinary thing for one agent
	// to report — leaves their order to the sort algorithm. sort.SliceStable then
	// pins even the ties this cannot see, such as two findings equal in every
	// recorded field.
	//
	// Surfaces need no tiebreaker beyond the name: the check above has already
	// established the names are unique.
	sorted := append([]gateSurfaceVerdict(nil), surfaces...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Surface < sorted[j].Surface })
	found := append([]gateFinding(nil), findings...)
	sort.SliceStable(found, func(i, j int) bool {
		if found[i].Surface != found[j].Surface {
			return found[i].Surface < found[j].Surface
		}
		if found[i].Rule != found[j].Rule {
			return found[i].Rule < found[j].Rule
		}
		if found[i].Severity != found[j].Severity {
			return found[i].Severity < found[j].Severity
		}
		return found[i].Detail < found[j].Detail
	})

	return gateReceipt{
		Gate:       "G1",
		Version:    version,
		Branch:     branch,
		BaseRef:    gateBaseRef,
		HeadCommit: head,
		Tree:       tree,
		Surfaces:   sorted,
		Findings:   found,
	}, nil
}

// gatePreMergePrecondition is the second assertion, made immediately before the
// merge by the deterministic driver.
//
// It re-asks BOTH questions rather than trusting the receipt. Time passes
// between G1 and the merge: origin/main advances, and the release branch can
// pick up one more commit. Either one invalidates the receipt, and each is
// reported as itself — an advanced base is an ancestry failure with a merge as
// its recovery, a moved branch is a stale receipt with a re-run as its recovery,
// and telling an operator the wrong one costs a wasted cycle at the worst
// moment.
func gatePreMergePrecondition(dir string, r gateReceipt) error {
	if r.Tree == "" || r.Branch == "" || r.BaseRef == "" {
		return fmt.Errorf("%w: the receipt records no %s, so there is nothing to re-assert before the merge; treat it as a failed gate rather than as a pass", errGateUncheckable, gateReceiptMissingField(r))
	}
	if err := gateRequireAncestor(dir, r.BaseRef, r.Branch); err != nil {
		return fmt.Errorf("pre-merge precondition: %w", err)
	}
	tree, err := gateTreeSHA(dir, r.Branch)
	if err != nil {
		return fmt.Errorf("pre-merge precondition: %w", err)
	}
	if tree != r.Tree {
		return fmt.Errorf("pre-merge precondition: %s now holds tree %s, but the gate verified tree %s — "+
			"something landed on the branch after the gate read it, so the receipt covers content that is no longer what would be merged; re-run the gate",
			r.Branch, tree, r.Tree)
	}
	return nil
}

// gateReceiptMissingField names the first field a receipt is missing, so the
// refusal above says which one rather than "something".
func gateReceiptMissingField(r gateReceipt) string {
	switch {
	case r.Tree == "":
		return "tree"
	case r.Branch == "":
		return "branch"
	default:
		return "base ref"
	}
}

// The two ways the handshake fails, kept apart as sentinels because they are
// different accusations with different recoveries.
//
// A MISMATCH accuses the MERGE: the gate read one tree, the merge produced
// another, and the recovery is the ancestry one — merge the base in and re-run.
// UNCHECKABLE accuses the RECEIPT: there was never a tree to compare, so the
// merge is not implicated at all and the recovery is to run the gate.
//
// Collapsing them is tempting and wrong, because the arithmetic happens to work:
// an empty receipt tree compares unequal to any real sha, so a single comparison
// would return "mismatch" and look correct while sending an operator to
// investigate a merge that did nothing wrong.
var (
	errGateTreeMismatch = errors.New("the merge commit's tree is not the tree the gate verified")
	errGateUncheckable  = errors.New("the handshake could not be made")
)

// gateAssertMergeMatchesReceipt is the handshake itself: the merge commit's tree
// must be the tree the gate recorded. An input it cannot read is a failure and
// never a pass — "we did not check" must not read as "it is fine".
func gateAssertMergeMatchesReceipt(r gateReceipt, mergeTree string) error {
	if r.Tree == "" {
		return fmt.Errorf("%w: the receipt records no tree, so there is nothing for the merge to be checked against; a gate whose receipt is empty is a failed gate, not an unchecked pass", errGateUncheckable)
	}
	if mergeTree == "" {
		return fmt.Errorf("%w: the merge commit's tree could not be read", errGateUncheckable)
	}
	if mergeTree != r.Tree {
		return fmt.Errorf("%w: the merge holds %s where the gate verified %s, so it carries content the gate never read.\n%s",
			errGateTreeMismatch, mergeTree, r.Tree, gateAncestryRecovery)
	}
	return nil
}

// evaluate is the receipt's verdict, recomputed from its own contents plus the
// surfaces the manifest declares and the fingerprints this tree produces.
//
// PASS requires all three: no findings, and every declared surface holding a
// PASS, and every one of those PASSes fingerprinted against THIS tree. Anything
// else is FAILED with an error naming every reason, because a verdict that says
// only "FAILED" sends the reader back to re-derive what the gate already knew.
func (r gateReceipt) evaluate(declared []string, current map[string]string) (string, error) {
	var reasons []string
	if len(r.Findings) > 0 {
		reasons = append(reasons, fmt.Sprintf("%d finding(s) reached the report; a human confirms what blocks the release, so the gate does not clear itself", len(r.Findings)))
	}
	if err := gateIsGreen(declared, r.Surfaces, current); err != nil {
		reasons = append(reasons, err.Error())
	}
	if len(reasons) == 0 {
		return gateVerdictPass, nil
	}
	return gateVerdictFailed, errors.New(strings.Join(reasons, "\n"))
}

// ---------------------------------------------------------------------
// the tests
// ---------------------------------------------------------------------

// gateTestGit runs one git command in a FIXTURE repository, with the
// contributor's global and system configuration neutralized — the same
// isolation check_staged_cli_test.go's stagedGit applies, and for the same
// reason: a developer's commit.gpgsign or merge.ff setting must not change what
// these tests measure.
//
// The gate helpers above deliberately do NOT do this. They run against the real
// repository, where overriding a maintainer's git configuration would be a
// read-only tool quietly changing the environment it was asked to observe.
func gateTestGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_DATE=2026-08-07T00:00:00Z",
		"GIT_COMMITTER_DATE=2026-08-07T00:00:00Z",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// gateFixtureRepo builds a repository with a real `origin` remote, so
// `origin/main` is a genuine remote-tracking ref rather than a local branch
// wearing the name. The precondition under test is stated in terms of
// origin/main, and a fixture that substituted a local ref would be testing a
// different question than the one the gate asks.
//
// It returns the work tree. On return: main holds one commit, origin/main points
// at it, and a `release` branch is checked out one commit ahead — the converging
// shape.
func gateFixtureRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatalf("git is not on PATH; this gate cannot be exercised, which is a failure and not a skip: %v", err)
	}

	origin := t.TempDir()
	gateTestGit(t, origin, "init", "-q", "--bare", "-b", "main")

	work := t.TempDir()
	gateTestGit(t, work, "init", "-q", "-b", "main")
	gateTestGit(t, work, "config", "user.email", "fixture@example.invalid")
	gateTestGit(t, work, "config", "user.name", "fixture")
	gateWrite(t, work, "README.md", "the base tree\n")
	gateTestGit(t, work, "add", "-A")
	gateTestGit(t, work, "commit", "-qm", "base")
	gateTestGit(t, work, "remote", "add", "origin", origin)
	gateTestGit(t, work, "push", "-q", "-u", "origin", "main")

	gateTestGit(t, work, "checkout", "-q", "-b", "release")
	gateWrite(t, work, "feature.md", "the branch's own content\n")
	gateTestGit(t, work, "add", "-A")
	gateTestGit(t, work, "commit", "-qm", "feat: the change under release")
	return work
}

// gateAdvanceOrigin lands a commit on the REMOTE's main that the release branch
// does not have, which is the diverged shape the precondition exists to refuse.
//
// It pushes from a SECOND CLONE, and that is the whole design of this helper
// rather than an incidental detail. A push performed from `work` updates
// `work`'s own refs/remotes/origin/main as a side effect, so a fixture that
// advanced main from the release clone would hand the gate a tracking ref that
// is already current — and every assertion built on it would pass without the
// gate ever fetching anything. That is not the production topology: main advances
// because somebody else merged their PR, in their clone or on the forge, and this
// clone learns about it only by fetching. Measuring the fetch requires a fixture
// in which not fetching is observably wrong.
//
// So on return: the remote's main carries a commit, and `work`'s origin/main
// still points where it did — deliberately stale, exactly as a real clone's would
// be.
func gateAdvanceOrigin(t *testing.T, work string) {
	t.Helper()
	origin := gateTestGit(t, work, "remote", "get-url", "origin")

	other := t.TempDir()
	gateTestGit(t, other, "clone", "-q", origin, other)
	gateTestGit(t, other, "config", "user.email", "someone-else@example.invalid")
	gateTestGit(t, other, "config", "user.name", "someone else")
	gateWrite(t, other, "hotfix.md", "landed on main after the branch was cut\n")
	gateTestGit(t, other, "add", "-A")
	gateTestGit(t, other, "commit", "-qm", "fix: something urgent")
	gateTestGit(t, other, "push", "-q", "origin", "main")
}

// gateLocalBaseRef is `work`'s own remote-tracking ref for the base, read
// WITHOUT refreshing it. The tests below use it to establish that a fixture is
// stale before asking the gate anything, so that a refusal is the fetch and not
// an accident of the fixture.
func gateLocalBaseRef(t *testing.T, work string) string {
	t.Helper()
	return gateTestGit(t, work, "rev-parse", gateBaseRef)
}

// TestGateReceiptTreeSurvivesANoFFMerge is the premise the whole handshake rests
// on, asserted against real git rather than quoted from the manual: when
// origin/main is already an ancestor of the branch, a --no-ff merge produces a
// REAL merge commit whose tree is the branch's tree unchanged.
func TestGateReceiptTreeSurvivesANoFFMerge(t *testing.T) {
	work := gateFixtureRepo(t)

	receipt, err := gateRecordReceipt(work, "v9.9.9", "release", gatePassingSurfaces("readme"), nil)
	if err != nil {
		t.Fatalf("record receipt on a converging branch: %v", err)
	}
	if err := gatePreMergePrecondition(work, receipt); err != nil {
		t.Fatalf("pre-merge precondition on an unmoved branch: %v", err)
	}

	gateTestGit(t, work, "checkout", "-q", "main")
	gateTestGit(t, work, "merge", "-q", "--no-ff", "-m", "merge release", "release")

	// A real merge commit, not a fast-forward: HEAD^2 must resolve.
	if _, err := gateResolve(work, "HEAD^2"); err != nil {
		t.Fatalf("--no-ff did not produce a merge commit: %v", err)
	}
	mergeTree, err := gateTreeSHA(work, "HEAD")
	if err != nil {
		t.Fatalf("read the merge commit's tree: %v", err)
	}
	if err := gateAssertMergeMatchesReceipt(receipt, mergeTree); err != nil {
		t.Fatalf("the handshake refused a merge whose tree IS the verified tree: %v", err)
	}
}

// TestGateAncestryPreconditionRefusesADivergedBranch covers the refusal and,
// beside it, the reason the refusal has to happen BEFORE the receipt is written:
// the merge of a diverged branch produces a different tree, and re-running the
// gate reproduces the identical receipt, so without the precondition the
// pipeline has no move that clears it.
func TestGateAncestryPreconditionRefusesADivergedBranch(t *testing.T) {
	work := gateFixtureRepo(t)

	// Before divergence the branch converges, and the receipt is reproducible.
	first, err := gateRecordReceipt(work, "v9.9.9", "release", gatePassingSurfaces("readme"), nil)
	if err != nil {
		t.Fatalf("record receipt: %v", err)
	}
	again, err := gateRecordReceipt(work, "v9.9.9", "release", gatePassingSurfaces("readme"), nil)
	if err != nil {
		t.Fatalf("re-record receipt: %v", err)
	}
	if first.Tree != again.Tree {
		t.Fatalf("re-running the gate against an unchanged branch must reproduce the same tree; got %s then %s", first.Tree, again.Tree)
	}

	gateAdvanceOrigin(t, work)

	err = gateRequireAncestor(work, gateBaseRef, "release")
	if !errors.Is(err, errGateNotAncestor) {
		t.Fatalf("a diverged branch must be refused as not-an-ancestor; got %v", err)
	}
	if !strings.Contains(err.Error(), "git merge origin/main") {
		t.Errorf("the refusal must name the move that clears it; got:\n%s", err)
	}
	if _, err := gateRecordReceipt(work, "v9.9.9", "release", gatePassingSurfaces("readme"), nil); err == nil {
		t.Fatal("a receipt was recorded for a branch whose merge cannot reproduce its tree")
	}

	// And the reason: merging now yields a tree the receipt does not name, while
	// the receipt itself has not moved. The local main has to be brought up to
	// the remote's first — it is the REMOTE's main that advanced, in a clone this
	// one has never heard from — which is the same pull an operator performs
	// after the forge rejects a non-fast-forward push.
	gateTestGit(t, work, "fetch", "-q", "origin")
	gateTestGit(t, work, "checkout", "-q", "main")
	gateTestGit(t, work, "merge", "-q", "--ff-only", gateBaseRef)
	gateTestGit(t, work, "merge", "-q", "--no-ff", "-m", "merge release", "release")
	mergeTree, err := gateTreeSHA(work, "HEAD")
	if err != nil {
		t.Fatalf("read the merge commit's tree: %v", err)
	}
	if mergeTree == first.Tree {
		t.Fatal("a three-way merge of a diverged branch produced the branch's own tree; the precondition would be unnecessary")
	}
	if err := gateAssertMergeMatchesReceipt(first, mergeTree); !errors.Is(err, errGateTreeMismatch) {
		t.Fatalf("the handshake must report the three-way merge tree as a mismatch; got %v", err)
	}
}

// TestGateAncestryRefreshesTheBaseRefBeforeAskingAboutIt is the assertion the
// fetch exists for, and the only one in this file that can see it.
//
// The shape is production's, not the fixture's convenience: somebody else's clone
// pushes to main, and THIS clone is never told. Its refs/remotes/origin/main is a
// file that has not moved, so the raw question — the one gateRequireAncestor used
// to ask — is answered from a ref describing the past. The test establishes that
// staleness first, by asking git directly and requiring exit 0, and only then
// asks the gate. Two answers to one question, and the gate has to give the one
// about the remote.
//
// Delete the gateRefreshBase call from gateRequireAncestor and this is the
// assertion that goes red; nothing else in this file notices, because every other
// fixture is already current. What the deletion would buy in production is the
// full failure: the precondition passes, the `--no-ff` merge of a
// locally-converged branch produces the branch's own tree, the handshake below
// agrees to the byte, and the push is rejected as non-fast-forward — after which
// the operator pulls and tags the three-way merge nobody read.
func TestGateAncestryRefreshesTheBaseRefBeforeAskingAboutIt(t *testing.T) {
	work := gateFixtureRepo(t)
	before := gateLocalBaseRef(t, work)

	// Somebody else merges to main. This clone is not told.
	gateAdvanceOrigin(t, work)

	// The fixture is the real topology, asserted rather than assumed: the local
	// tracking ref has not moved, so the unrefreshed question is answerable and
	// answers "yes". If this ever fails, the fixture has started fetching behind
	// the test's back and the rows below would prove nothing.
	if after := gateLocalBaseRef(t, work); after != before {
		t.Fatalf("the fixture advanced this clone's %s (%s -> %s) as well as the remote's main, so not fetching would not be observable and this test could not measure the fetch", gateBaseRef, before, after)
	}
	stale := exec.Command("git", "merge-base", "--is-ancestor", gateBaseRef, "release")
	stale.Dir = work
	if err := stale.Run(); err != nil {
		t.Fatalf("the unrefreshed question did not answer \"contained\", so this test is not measuring the fetch: %v", err)
	}

	// The gate, asked the same question, must answer about the remote.
	err := gateRequireAncestor(work, gateBaseRef, "release")
	if !errors.Is(err, errGateNotAncestor) {
		t.Fatalf("the ancestry check answered from an unrefreshed %s: the remote's main has advanced and the branch does not contain it, but the precondition was satisfied; got %v", gateBaseRef, err)
	}
	if !strings.Contains(err.Error(), "git merge origin/main") {
		t.Errorf("the refusal must name the move that clears it; got:\n%s", err)
	}

	// And the refresh is what did it: the tracking ref is now current, so the
	// refusal is a real answer about the remote rather than a fetch that failed
	// and was reported as a negative.
	if after := gateLocalBaseRef(t, work); after == before {
		t.Errorf("%s still points at %s after the ancestry check; the refusal cannot have come from reading the remote", gateBaseRef, before)
	}

	// Both re-assertions ask through the same helper, so both inherit the
	// refresh — which is the point of the two of them existing. Asserted rather
	// than assumed, because a future caller could reach past gateRequireAncestor
	// to the raw git command and silently lose it.
	if err := gateG1Precondition(work, "release"); !errors.Is(err, errGateNotAncestor) {
		t.Errorf("the pre-fan-out precondition admitted a branch the remote's main has moved past; got %v", err)
	}
	if _, err := gateRecordReceipt(work, "v9.9.9", "release", gatePassingSurfaces("readme"), nil); !errors.Is(err, errGateNotAncestor) {
		t.Errorf("a receipt was recorded for a branch the remote's main has moved past; got %v", err)
	}
}

// TestGateAncestryNeverReadsAFailureToRunAsAnAnswer is the CLAUDE.md rule in one
// assertion: a question that could not be asked must be neither a pass nor a
// definite "no". A definite "no" would send the operator off to merge a ref that
// does not exist.
//
// There are now two places the question can fail to be asked, and they are
// covered separately because they fail in different commands. A base ref the
// remote does not carry fails in the FETCH, and is uncheckable — the local ref
// may well still exist, which is exactly the answer that must not be used. A base
// that refreshes fine against a branch git cannot resolve fails in `merge-base`
// itself, which is the case the exit-code demultiplexing exists for.
func TestGateAncestryNeverReadsAFailureToRunAsAnAnswer(t *testing.T) {
	work := gateFixtureRepo(t)

	t.Run("a base ref the remote does not carry", func(t *testing.T) {
		err := gateRequireAncestor(work, "origin/no-such-ref", "release")
		if err == nil {
			t.Fatal("an unrefreshable base ref was waved through as a satisfied precondition")
		}
		if !errors.Is(err, errGateUncheckable) {
			t.Fatalf("a base ref that could not be refreshed must be reported as uncheckable; got %v", err)
		}
		if errors.Is(err, errGateNotAncestor) {
			t.Fatalf("an unrefreshable base ref was reported as a definite not-an-ancestor, whose recovery is a merge that cannot help; got:\n%s", err)
		}
	})

	t.Run("a base ref that is not a remote-tracking ref", func(t *testing.T) {
		// A bare "main" is the plausible mistake, and it is the one gateBaseRef's
		// own comment warns about: it names a LOCAL branch, which can sit behind
		// the remote indefinitely and would make the precondition pass over a
		// question nobody asked. It cannot be refreshed, so it is refused here
		// rather than answered from.
		err := gateRequireAncestor(work, "main", "release")
		if !errors.Is(err, errGateUncheckable) {
			t.Fatalf("a base that is not a <remote>/<branch> ref must be refused as uncheckable rather than answered from the local ref; got %v", err)
		}
		// The wanted phrase belongs to the SHAPE guard and to nothing else. An
		// earlier version of this row looked for "remote-tracking", which the
		// fetch-failed message also carries — so deleting the shape guard left
		// the row green: `git fetch main +refs/heads/:refs/remotes/main/` fails
		// on its own, the fallthrough error says "remote-tracking", and the
		// assertion could not tell a named refusal from an accident.
		if !strings.Contains(err.Error(), `is not a <remote>/<branch>`) {
			t.Errorf("the refusal must name the SHAPE as the problem — a bare branch name cannot be refreshed and must not be answered from — rather than reporting whatever a nonsense refspec did; got:\n%s", err)
		}
	})

	t.Run("a branch git cannot resolve", func(t *testing.T) {
		err := gateRequireAncestor(work, gateBaseRef, "no-such-branch")
		if err == nil {
			t.Fatal("an unresolvable branch was waved through as a satisfied precondition")
		}
		if errors.Is(err, errGateNotAncestor) {
			t.Fatalf("an unresolvable branch was reported as a definite not-an-ancestor; got:\n%s", err)
		}
		if !strings.Contains(err.Error(), "could not determine") {
			t.Errorf("the failure must say the question could not be answered; got:\n%s", err)
		}
	})

	if _, err := gateRecordReceipt(t.TempDir(), "v9.9.9", "release", gatePassingSurfaces("readme"), nil); err == nil {
		t.Fatal("a receipt was recorded outside a git work tree, where no precondition could be evaluated")
	}
}

// TestGateResolveRefusesAnAnswerThatIsNotAnObjectName pins gateSHARE, which is
// otherwise a guard no test notices the loss of.
//
// `git rev-parse` is git's general-purpose argument parser, not a revision
// resolver, and a good half of its modes exit ZERO while answering a question
// about the repository rather than about a revision. So "git said yes" is not
// enough to treat what it printed as a sha, and the shape has to be checked.
//
// The direction matters more than the case. Every value gateResolve produces is
// ultimately COMPARED against another sha, so an answer of the wrong shape that
// is passed through does not fail as a bad read — it fails as an INEQUALITY, and
// the handshake reports it as the merge carrying content the gate never saw.
// That sends an operator to investigate a merge that did nothing wrong, over
// what is really a malformed git invocation.
//
// `--is-inside-work-tree` is the cheapest reproduction of that shape: exit 0,
// and "true" on stdout.
func TestGateResolveRefusesAnAnswerThatIsNotAnObjectName(t *testing.T) {
	work := gateFixtureRepo(t)

	// The fixture is honest: a real revision resolves, so the refusal below is
	// the guard rather than a broken repository.
	head, err := gateResolve(work, "release")
	if err != nil {
		t.Fatalf("a real revision was refused: %v", err)
	}
	if !gateSHARE.MatchString(head) {
		t.Fatalf("gateResolve returned %q, which is not a full object name", head)
	}

	got, err := gateResolve(work, "--is-inside-work-tree")
	if err == nil {
		t.Fatalf("gateResolve passed %q through as an object name; it would then be compared against a sha and the inequality reported as tree drift", got)
	}
	if !strings.Contains(err.Error(), "not a full object name") {
		t.Errorf("the refusal must say the answer was the wrong shape rather than that a revision was missing; got:\n%s", err)
	}
}

// TestGatePreMergePreconditionRefusesAMovedBranch: one commit landing on the
// branch between the gate run and the merge invalidates the receipt, and it is
// reported as a stale receipt rather than as an ancestry problem.
func TestGatePreMergePreconditionRefusesAMovedBranch(t *testing.T) {
	work := gateFixtureRepo(t)

	receipt, err := gateRecordReceipt(work, "v9.9.9", "release", gatePassingSurfaces("readme"), nil)
	if err != nil {
		t.Fatalf("record receipt: %v", err)
	}
	gateWrite(t, work, "afterthought.md", "landed after the gate read the tree\n")
	gateTestGit(t, work, "add", "-A")
	gateTestGit(t, work, "commit", "-qm", "docs: one more thing")

	err = gatePreMergePrecondition(work, receipt)
	if err == nil {
		t.Fatal("the pre-merge precondition passed over a branch that moved after the gate read it")
	}
	if errors.Is(err, errGateNotAncestor) {
		t.Fatalf("a moved branch was reported as an ancestry failure, whose recovery is a merge rather than a re-run; got:\n%s", err)
	}
	if !strings.Contains(err.Error(), "re-run the gate") {
		t.Errorf("the refusal must name the re-run as its recovery; got:\n%s", err)
	}
}

// TestGatePreMergePreconditionRefusesABaseThatAdvancedAfterTheReceipt is the
// shape only the SECOND assertion can catch, and it is the reason the second
// assertion exists at all.
//
// A release waits for a human between the gate run and the merge, and main does
// not wait with it. Here origin/main takes a commit while the release branch
// takes none: the receipt's tree is still exactly the branch's tree, so every
// other check in gatePreMergePrecondition passes — the tree comparison agrees to
// the byte, and it agrees CORRECTLY, because nothing about the branch moved. The
// only thing that has changed is that the merge is no longer a fast-forward, so
// the tree that would land is one nobody read.
//
// The test asserts the tree still matches before asking, so a failure here is
// the ancestry check and cannot be the tree comparison arriving at the right
// answer for the wrong reason. Delete the gateRequireAncestor call from
// gatePreMergePrecondition and this is the assertion that goes red; without it
// the function returns nil and clears the driver to perform exactly the
// three-way merge the receipt exists to prevent.
//
// It measures the ancestry check ONLY because gateAdvanceOrigin pushes from a
// second clone. An earlier version of this fixture pushed from `work` itself,
// which updates `work`'s own tracking ref as a side effect of the push — so the
// refusal came from a ref that had been handed the new value rather than from
// anything the gate did, and substituting the real topology turned it green over
// a diverged branch. The staleness is therefore asserted below, not assumed.
func TestGatePreMergePreconditionRefusesABaseThatAdvancedAfterTheReceipt(t *testing.T) {
	work := gateFixtureRepo(t)

	if err := gateG1Precondition(work, "release"); err != nil {
		t.Fatalf("the pre-fan-out precondition refused a converging branch: %v", err)
	}
	receipt, err := gateRecordReceipt(work, "v9.9.9", "release", gatePassingSurfaces("readme"), nil)
	if err != nil {
		t.Fatalf("record receipt: %v", err)
	}
	base := gateLocalBaseRef(t, work)

	// main moves, in somebody else's clone; the branch does not, and this clone
	// is not told about either.
	gateAdvanceOrigin(t, work)
	if after := gateLocalBaseRef(t, work); after != base {
		t.Fatalf("the fixture advanced this clone's %s as well as the remote's main, so the refusal below could come from the fixture rather than from the check", gateBaseRef)
	}

	tree, err := gateTreeSHA(work, "release")
	if err != nil {
		t.Fatalf("read the branch's tree: %v", err)
	}
	if tree != receipt.Tree {
		t.Fatalf("the fixture moved the branch as well as the base ref, so this row would be caught by the tree comparison and would prove nothing about the ancestry check; branch holds %s, receipt names %s", tree, receipt.Tree)
	}

	err = gatePreMergePrecondition(work, receipt)
	if err == nil {
		t.Fatal("the pre-merge precondition passed over a branch that no longer contains origin/main; the driver would merge a tree carrying content the gate never read")
	}
	if !errors.Is(err, errGateNotAncestor) {
		t.Fatalf("a base ref that advanced after the receipt must be reported as an ancestry failure; got %v", err)
	}
	if !strings.Contains(err.Error(), "git merge origin/main") {
		t.Errorf("the refusal must name the move that clears it; got:\n%s", err)
	}
	// The misdirection to avoid: blaming the branch sends the operator to re-run
	// the gate, which reads the same unmoved tree and reproduces this receipt.
	if strings.Contains(err.Error(), "something landed on the branch") {
		t.Errorf("an advanced base ref was reported as a branch that moved, whose recovery is a re-run that cannot help; got:\n%s", err)
	}

	// And the cheap door agrees, so the re-run this refusal invites does not pay
	// for a fan-out before reaching the same answer.
	if err := gateG1Precondition(work, "release"); !errors.Is(err, errGateNotAncestor) {
		t.Fatalf("the pre-fan-out precondition admitted a diverged branch, so a re-run would spend one agent per surface on a receipt the merge cannot honour; got %v", err)
	}
}

// TestGatePreMergePreconditionRefusesAnUncheckableReceipt is the sibling of
// TestGateHandshakeRefusesAnUncheckableReceipt, for the SECOND assertion — and
// it exists because the two guards are identical in intent while only one of
// them was pinned.
//
// A receipt missing the field the check is about must be reported as
// uncheckable, never as whatever the comparison happens to produce. Remove the
// guard and an empty Tree falls straight through to the mismatch branch, where
// it compares unequal to whatever the branch holds — so the operator is told
// something landed on the branch after the gate read it, and is sent to re-run
// the gate against a branch that has not moved. That is "we did not check"
// wearing the costume of a definite answer, and it costs a cycle at the worst
// possible moment.
//
// Each row blanks exactly one field of a receipt that is otherwise real and
// otherwise passes, so a row that goes red is the guard and not the fixture.
func TestGatePreMergePreconditionRefusesAnUncheckableReceipt(t *testing.T) {
	work := gateFixtureRepo(t)
	full, err := gateRecordReceipt(work, "v9.9.9", "release", gatePassingSurfaces("readme"), nil)
	if err != nil {
		t.Fatalf("record receipt: %v", err)
	}
	if err := gatePreMergePrecondition(work, full); err != nil {
		t.Fatalf("the intact receipt was refused, so the rows below would prove nothing: %v", err)
	}

	for _, tc := range []struct {
		name  string
		blank func(r *gateReceipt)
		field string
	}{
		{"no tree", func(r *gateReceipt) { r.Tree = "" }, "tree"},
		{"no branch", func(r *gateReceipt) { r.Branch = "" }, "branch"},
		{"no base ref", func(r *gateReceipt) { r.BaseRef = "" }, "base ref"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			receipt := full
			tc.blank(&receipt)

			err := gatePreMergePrecondition(work, receipt)
			if err == nil {
				t.Fatalf("a receipt recording no %s was accepted as a satisfied pre-merge precondition", tc.field)
			}
			if !errors.Is(err, errGateUncheckable) {
				t.Fatalf("a receipt recording no %s must be reported as uncheckable; got %v", tc.field, err)
			}
			if errors.Is(err, errGateNotAncestor) {
				t.Errorf("a receipt recording no %s was reported as an ancestry failure, whose recovery is a merge that cannot help; got:\n%s", tc.field, err)
			}
			if !strings.Contains(err.Error(), tc.field) {
				t.Errorf("the refusal must name the field the receipt is missing; want a mention of %q, got:\n%s", tc.field, err)
			}
			// The specific misdirection the guard prevents: blaming the branch.
			if strings.Contains(err.Error(), "something landed on the branch") {
				t.Errorf("a receipt recording no %s was reported as a branch that moved, which sends the operator to re-run the gate against an unchanged branch; got:\n%s", tc.field, err)
			}
		})
	}
}

// TestGateReceiptRefusesToRecordASurfaceTwice is errGateDuplicateVerdict at the
// earliest point it can be caught, plus the property that catching it there
// buys: a receipt that is a function of the SET of results, not of the order the
// fan-out returned them in.
//
// Reproducibility is not a nicety here. The ancestry precondition's whole
// argument (see the package comment) is that re-running G1 against an unchanged
// branch reproduces the identical receipt — that is why a diverged branch wedges
// and why the merge is the only way out. A receipt whose bytes depended on
// goroutine scheduling would make that argument false.
func TestGateReceiptRefusesToRecordASurfaceTwice(t *testing.T) {
	work := gateFixtureRepo(t)

	twice := []gateSurfaceVerdict{
		{Surface: "readme", Verdict: gateVerdictPass, Fingerprint: "sha256:aa"},
		{Surface: "site", Verdict: gateVerdictFailed, Fingerprint: "sha256:bb"},
		{Surface: "site", Verdict: gateVerdictPass, Fingerprint: "sha256:bb"},
	}
	if _, err := gateRecordReceipt(work, "v9.9.9", "release", twice, nil); !errors.Is(err, errGateDuplicateVerdict) {
		t.Fatalf("a receipt was recorded carrying two verdicts for one surface; got %v", err)
	}

	// The same results handed over in a different order must produce the same
	// receipt, ties in the findings included: both entries below share a surface
	// AND a rule, which is exactly what a comparator over (surface, rule) alone
	// leaves to the sort.
	surfaces := gatePassingSurfaces("changelog", "readme", "site")
	findings := []gateFinding{
		{Surface: "site", Rule: "stale-count", Severity: "minor", Detail: "says 27, the registry holds 28"},
		{Surface: "site", Rule: "stale-count", Severity: "minor", Detail: "says 6 nouns, the contract lists 7"},
		{Surface: "readme", Rule: "undocumented-flag", Severity: "major", Detail: "--strict is not described"},
	}
	first, err := gateRecordReceipt(work, "v9.9.9", "release", surfaces, findings)
	if err != nil {
		t.Fatalf("record receipt: %v", err)
	}
	shuffled, err := gateRecordReceipt(work, "v9.9.9", "release",
		[]gateSurfaceVerdict{surfaces[2], surfaces[0], surfaces[1]},
		[]gateFinding{findings[1], findings[2], findings[0]})
	if err != nil {
		t.Fatalf("record receipt over the same results in another order: %v", err)
	}

	want, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal receipt: %v", err)
	}
	got, err := json.Marshal(shuffled)
	if err != nil {
		t.Fatalf("marshal receipt: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("the receipt depends on the order its results arrived in, so a re-run over an unchanged tree need not reproduce it:\n first: %s\nsecond: %s", want, got)
	}
}

// TestGateHandshakeRefusesAnUncheckableReceipt pins the direction the handshake
// errs in: a receipt or a merge tree it cannot read is a failure, never a pass
// that happens to compare unequal.
func TestGateHandshakeRefusesAnUncheckableReceipt(t *testing.T) {
	full := gateReceipt{Tree: strings.Repeat("a", 40)}
	other := strings.Repeat("b", 40)

	// An unreadable input is UNCHECKABLE, never a mismatch. The distinction is
	// the assertion: an empty receipt tree compares unequal to any real sha, so
	// a single comparison would report "mismatch" and read as correct while
	// sending an operator to investigate a merge that did nothing wrong.
	if err := gateAssertMergeMatchesReceipt(gateReceipt{}, other); !errors.Is(err, errGateUncheckable) {
		t.Errorf("an empty receipt tree must be reported as uncheckable, not as a merge that disagrees; got %v", err)
	}
	if err := gateAssertMergeMatchesReceipt(full, ""); !errors.Is(err, errGateUncheckable) {
		t.Errorf("an unreadable merge tree must be reported as uncheckable; got %v", err)
	}
	if err := gateAssertMergeMatchesReceipt(full, other); !errors.Is(err, errGateTreeMismatch) {
		t.Errorf("a merge tree that disagrees with the receipt must be reported as a mismatch; got %v", err)
	}
	if err := gateAssertMergeMatchesReceipt(full, full.Tree); err != nil {
		t.Errorf("a matching merge tree was refused: %v", err)
	}
}

// gateAncestryItemRE isolates docs/RELEASING.md's ancestry checklist item, from
// its bolded title to the next item at the same level. The assertions below are
// made against THAT SLICE and not against the whole document, for the reason
// gate_release_stamp_test.go's gateLdflagsItemRE gives: the file discusses
// merging and fetching in several places, so a whole-file search would report
// "the procedure mentions this" where the question is "the item a maintainer
// reads tells them to do it".
var gateAncestryItemRE = regexp.MustCompile("(?s)- \\[ \\] \\*\\*`origin/main` is already an ancestor of the release branch\\.\\*\\*.*?\\n- \\[ \\] ")

// TestReleaseProcedurePinsTheAncestryPrecondition holds the manual half of the
// is-ancestor precondition, which is the only half anything executes today.
//
// This file's package comment states as an invariant that "docs/RELEASING.md's
// manual checklist item spells the same pair (`git fetch origin && git merge-base
// --is-ancestor origin/main <branch>`), and the two must not drift apart", and
// until now nothing asserted it. A review deleted the entire item — every line of
// it — and `go test ./...` stayed green.
//
// It matters more than an ordinary doc-pin gap. Everything above is test-only
// code wired to no command and no driver (see WHY IT IS TEST CODE), so until the
// driver exists, THIS CHECKLIST ITEM IS THE PRECONDITION as actually performed.
// Deleting it does not weaken a redundant copy; it removes the only copy anyone
// runs.
//
// And the half most likely to go is the `git fetch`. It is the half the package
// comment spends fifteen lines arguing is load-bearing — `origin/main` is a file
// in the operator's clone, so the unrefreshed question is answered about whenever
// they last happened to fetch, and it answers "yes" exactly when the release is
// about to go wrong — and it is also the half that reads as ceremony to a later
// editor trimming a long document. So it is asserted by name and separately from
// the ancestry command, rather than as one string that a reflow could break.
func TestReleaseProcedurePinsTheAncestryPrecondition(t *testing.T) {
	root := surfaceRepoRoot(t)
	procedure := gateReadRepoFile(t, root, filepath.Join("docs", "RELEASING.md"))

	item := gateAncestryItemRE.FindString(procedure)
	if item == "" {
		t.Fatal("docs/RELEASING.md no longer carries an item titled **`origin/main` is already an ancestor of the release branch.**. " +
			"That item is the is-ancestor precondition as this repository actually performs it — gateRequireAncestor is test-only code wired to no command — so removing it removes the check, not a description of one. " +
			"Without it a `--no-ff` merge can produce a three-way tree carrying content the gate never read, behind a receipt that already said PASS")
	}

	for _, want := range []struct{ fragment, why string }{
		{"git fetch origin",
			"`origin/main` is a remote-tracking ref: a file in the operator's clone that does not move because somebody pushed. Asked without refreshing it, the ancestry question is answered about the last time anyone happened to fetch — and it answers \"yes\" precisely when main has advanced and the release is about to go wrong. " +
				"The fetch is the reason the answer is about the remote at all"},
		{"git merge-base --is-ancestor origin/main",
			"This is the ancestry question itself, and it is the pair gate_receipt_test.go's package comment says must not drift from this item"},
	} {
		if !strings.Contains(item, want.fragment) {
			t.Errorf("the ancestry item no longer names %q. %s.\nThe item reads:\n%s", want.fragment, want.why, item)
		}
	}

	// The item has to say what a "no" MEANS and how to clear it. An operator who
	// meets a bare non-zero exit here re-runs the checks against the same branch,
	// gets the identical answer, and the only moves left are to switch the gate
	// off or to tag something nobody read — which is the wedge gateRequireAncestor's
	// own failure message exists to prevent.
	if !strings.Contains(item, "git merge origin/main") {
		t.Errorf("the ancestry item no longer names `git merge origin/main` as the recovery. A precondition that refuses without naming the one move that clears it wedges the release: re-running the checks against the same branch reads the same tree and reaches the same answer.\nThe item reads:\n%s", item)
	}
}

// TestGateReceiptVerdictIsDerivedNotStored covers the receipt's own arithmetic:
// PASS needs no findings AND full, current, passing surface coverage, and every
// way of losing one of those reports FAILED.
func TestGateReceiptVerdictIsDerivedNotStored(t *testing.T) {
	declared := []string{"readme", "site"}
	current := map[string]string{"readme": "sha256:aa", "site": "sha256:bb"}
	green := []gateSurfaceVerdict{
		{Surface: "readme", Verdict: gateVerdictPass, Fingerprint: "sha256:aa"},
		{Surface: "site", Verdict: gateVerdictPass, Fingerprint: "sha256:bb"},
	}

	clean := gateReceipt{Surfaces: green}
	verdict, err := clean.evaluate(declared, current)
	if err != nil || verdict != gateVerdictPass {
		t.Fatalf("a clean, fully covered receipt must evaluate PASS; got %s (%v)", verdict, err)
	}

	// A receipt cannot store its way to a pass: there is no verdict field to
	// store, and the JSON shape says so.
	raw, err := json.Marshal(clean)
	if err != nil {
		t.Fatalf("marshal receipt: %v", err)
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		t.Fatalf("unmarshal receipt: %v", err)
	}
	if _, present := keys["verdict"]; present {
		t.Error("the receipt carries a stored verdict; it is derived so that a forged or stale one cannot exist")
	}
	for key := range keys {
		if key != strings.ToLower(key) {
			t.Errorf("receipt key %q is not snake_case", key)
		}
	}

	for _, tc := range []struct {
		name    string
		receipt gateReceipt
	}{
		{"a finding reached the report", gateReceipt{Surfaces: green, Findings: []gateFinding{{Surface: "site", Rule: "stale-count", Severity: "minor", Detail: "says 27, the registry holds 28"}}}},
		{"a declared surface holds no verdict", gateReceipt{Surfaces: green[:1]}},
		{"a surface holds a FAIL", gateReceipt{Surfaces: []gateSurfaceVerdict{green[0], {Surface: "site", Verdict: gateVerdictFailed, Fingerprint: "sha256:bb"}}}},
		{"a PASS was fingerprinted against another tree", gateReceipt{Surfaces: []gateSurfaceVerdict{green[0], {Surface: "site", Verdict: gateVerdictPass, Fingerprint: "sha256:stale"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			verdict, err := tc.receipt.evaluate(declared, current)
			if err == nil || verdict != gateVerdictFailed {
				t.Fatalf("expected FAILED with a reason; got %s (%v)", verdict, err)
			}
		})
	}
}
