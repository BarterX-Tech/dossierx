// history.go makes "dossierx check --staged" GIT-HISTORY AWARE.
//
// THE HOLE IT CLOSES. Every other rule in this engine is evaluated against ONE
// tree, and every one of them behaves correctly. What decides which files those
// rules are evaluated over — the claim registry's scope, and whether there is a
// ledger to compare it against — is itself data inside the tree being judged.
// So the scope can be collapsed by the very commit the gate is reading, and no
// rule is in a position to notice, because from inside a single tree a
// collapsed scope and an honest small project are the same picture.
//
// The verified two-commit reproduction, on a clean, fully-locked,
// hook-installed project:
//
//	commit 1  claims_dir: claims -> archive   (a tracked directory with no
//	          claims in it) AND git rm .dossierx-lock-store.json
//	commit 2  rewrite claims/locked.yaml freely
//
// After commit 1 the registry is EMPTY, so there is no locked claim for
// lock-ledger-missing to name and no locked claim for lock-ledger-absent to
// count (its trigger is "the ledger is gone AND that cost us records", which is
// exactly right and exactly powerless here). The reverse sweep that would have
// reported the standing record as abandoned is skipped because the store file
// is gone. `check`, `check --validate`, `check --staged` and a FRESH CLONE all
// print zero findings and exit 0, and the pre-commit hook accepts both commits
// without a word, having correctly refused the identical tamper moments
// earlier. Nothing is broken; the gate is simply pointed somewhere else.
//
// WHAT THIS FILE ADDS. The commit under judgement is compared against its
// PARENT, so a scope change is visible as a CHANGE rather than as an absence.
// Two things are asked, and only two, because these are the two inputs that
// decide what every other rule can see:
//
//   - did an integrity store that the parent carried DISAPPEAR?
//     (the lock ledger, the comment digest store)
//   - did claims_dir MOVE in a way that STRANDS tracked claim files outside the
//     new scope?
//
// BOTH QUESTIONS HAVE TO BE ASKED OF CONTENT AND OF THE PARENT'S OWN TREE, and
// the first cut of this file asked neither, which left one-commit replies to
// both:
//
//   - "DID IT DISAPPEAR" was asked as "is the path still tracked?". A lock
//     ledger that is still there and has been EMPTIED of its standing records
//     is still tracked, and it costs the gate exactly what deleting the file
//     costs: `git mv` the claims out of claims_dir (which never changes, so no
//     config edit appears in the diff at all) and overwrite the ledger map with
//     {}, and the registry is empty, every forward rule has no claim to name,
//     and the reverse sweep has no record left to walk. So the comparison is
//     over the store's CONTENT — the set of standing (unreleased) approvals —
//     and a standing approval that is gone from a store that is still present,
//     for a claim that is still tracked, is the same event as the deletion and
//     is refused as the same rule.
//
//   - "WHAT WAS THE PARENT'S claims_dir" was looked up at the CURRENT config's
//     path. Moving project.config.yaml therefore made the lookup MISS, the
//     parent contribute nothing, and the scope come from the new config alone —
//     a collapse with no file deleted and no claims_dir edit visible against the
//     parent. So the parent's configuration is found where the PARENT kept it,
//     and everything resolved beside a config — claims_dir, both stores — is
//     resolved beside the parent's own copy.
//
// WHY "STRANDS" AND NOT "MOVED". A claims_dir change on its own is not a
// tamper, and refusing one would build a trap: projects reorganise, and a gate
// with no sanctioned way to reorganise is a gate that gets uninstalled. What
// makes a repoint an attack is that it leaves claims BEHIND — still tracked,
// still saying status: locked, and now judged by nothing. A genuine move takes
// the claims with it:
//
//	git mv claims docs/claims
//	edit claims_dir in project.config.yaml
//	git add -A && git commit
//
// leaves nothing outside the new scope, so this gate says nothing about it.
// That is the sanctioned way, it needs no new command and no escape hatch, and
// it is the flow a human would use anyway. Widening the scope (claims -> the
// repository root, say) strands nothing either, and is likewise silent. The
// refusal is reserved for the one shape that costs coverage.
//
// WHAT "THE PARENT" MEANS, precisely, because --staged is run from two places
// with different answers:
//
//   - FROM A PRE-COMMIT HOOK the index differs from HEAD — that difference IS
//     the commit being made — so the commit under judgement is the INDEX and
//     its parent is HEAD. HEAD's tree is present in every non-empty repository,
//     including a depth-1 shallow clone, so this comparison never goes blind
//     where it matters most.
//   - FROM CI, or by hand on a clean tree, the index is identical to HEAD.
//     Comparing them would be vacuous, and a gate that reports "no change" over
//     a checkout of an already-collapsed history would be the same silence in a
//     new place. So the commit under judgement is HEAD itself and its parents
//     are HEAD's own parents. On GitHub's pull_request event the checkout is the
//     MERGE commit, whose first parent is the base branch, so this compares the
//     WHOLE pull request against what it is merging into — which is the
//     comparison that catches a collapse spread across two commits.
//
// A ROOT COMMIT HAS NO PARENT and is therefore never refused by anything here:
// a legitimate initial commit brings its ledger and its claims_dir into
// existence, and "it was not there before" is true of every file in it.
//
// A SHALLOW CHECKOUT can leave HEAD's parent unfetched. That is the one state
// where this comparison cannot be made and the answer is not "nothing changed"
// — so it is reported as an advisory (Result.NextSteps) naming the fix, rather
// than either refusing (which would break every default actions/checkout, and a
// gate that refuses honest work gets deleted) or passing in silence. The
// shipped workflow template sets fetch-depth: 0 so it does not arise there. The
// hook is unaffected: it is always in the index-differs-from-HEAD case.
//
// EVERYTHING HERE IS READ-ONLY, on the same terms as the rest of the gate: git
// is asked questions, no object is written, no ref is moved.
package check

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/digest"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// RuleIntegrityStoreRemoved: an integrity store the PARENT commit carried — the
// lock ledger, or the comment digest store — is not in the commit under
// judgement, OR is still there having been emptied of the standing approvals the
// parent recorded in it.
//
// THE SECOND HALF IS THE SAME EVENT AS THE FIRST and is deliberately the same
// rule name rather than a new one. What the gate loses when the ledger is
// deleted is not the file, it is the set of records every other rule reads; a
// store that still exists with `"ledger": {}` in it has lost exactly the same
// thing, while satisfying every check that asks whether the path is tracked.
// Splitting it into a second rule would mean a hook, a CI job or a skill that
// branches on rule names has to learn the new one before it refuses the cheaper
// version of an attack it already refuses — so the two shapes answer to one
// name, and the message says which of them happened.
//
// It is a rule about the gate's EVIDENCE, like lock-ledger-unreadable, and it
// exists because deleting the evidence is the one edit no evidence-based rule
// can report. lock-ledger-absent comes closest and deliberately does not fire on
// its own: its trigger is "the ledger is gone AND locked claims went unrecorded
// because of it", which keeps it off every caller that audits against an
// in-memory store — and which makes it silent for free the moment the same
// commit also empties the registry. This rule needs no claim to exist. It reads
// the previous commit, sees the file that was there, and sees that it is gone.
//
// It is deliberately NOT suppressed by the pre-ledger exemption. A project that
// predates the ledger has no ledger to delete, so it cannot reach this rule; a
// project that HAS one and drops it is making a statement about the approval
// record for every locked artifact it holds, and that statement belongs in front
// of a human.
//
// It cannot fire on a project that is being removed wholesale, because the whole
// comparison is skipped when project.config.yaml is no longer tracked (see
// stagedScope).
const RuleIntegrityStoreRemoved = "integrity-store-removed"

// RuleClaimsScopeNarrowed: claims_dir moved between the parent commit and the
// commit under judgement, and tracked claim files were left OUTSIDE the new
// scope.
//
// "MOVED" INCLUDES MOVING THE CONFIG ITSELF, and includes claims_dir leaving the
// repository altogether. Both were invisible while the parent's claims_dir was
// looked up at the CURRENT config's path and while an out-of-work-tree
// claims_dir short-circuited to ErrNoIndex: relocating project.config.yaml
// repoints claims_dir without editing the line, and `claims_dir: ../../elsewhere`
// strands every claim the parent audited while reporting "there is nothing here
// to evaluate". The stranding is the trigger in all three shapes.
//
// See this file's header for why the trigger is the stranding and not the move.
// The short form: a move that takes its claims with it is how a project
// reorganises and is silent here; a move that leaves them behind is how a locked
// claim stops being audited while remaining in the repository, tracked, and
// still reading status: locked.
//
// "Claim file" means a blob that DECODES as a claim, not one that is merely
// named *.yaml — the same test indexHoldsJudgeableContent applies, and for the
// same reason: a repository full of ordinary yaml must not turn into a refusal.
const RuleClaimsScopeNarrowed = "claims-scope-narrowed"

// scopeReport is stagedScope's answer: the refusals, and the one advisory that
// is not a refusal.
type scopeReport struct {
	// Findings are gate refusals, reported ahead of every other ledger finding
	// because a scope change is the CAUSE of whatever the rules below did or did
	// not manage to see.
	Findings []lock.Finding

	// Note is set only when the comparison could not be made at all — a shallow
	// checkout whose parent commit was never fetched, or a parent whose
	// project.config.yaml this run could not identify. It rides in
	// Result.NextSteps rather than in Findings: it is not evidence of a tamper,
	// it is evidence that this run could not look, and refusing on it would
	// break every default CI checkout in existence.
	Note string

	// Parents are the parent commits this report was computed against, each
	// carrying the config that commit ACTUALLY USED (see parentProject). They
	// are handed back rather than kept private because the per-claim half of the
	// content comparison — parentLedgerContent — needs the same two things this
	// resolution already paid for: which commits to compare against, and where
	// each of them kept its stores. Recomputing that in the caller would be a
	// second answer to a question with one right answer, and the two could drift
	// apart in exactly the shape (a relocated config) that RULE B exists for.
	//
	// Empty means there is nothing to compare against and no comparison should
	// be attempted: an unborn HEAD, a root commit, or a shallow clone whose
	// parents were never fetched — the last of which sets Note instead.
	Parents []parentProject
}

// claimsScope is the scope the commit under judgement is audited over, in the
// form a comparison against the parent needs.
//
// It is a type rather than the bare pathspec it used to be because "claims_dir
// points outside the work tree" has to be COMPARABLE instead of a reason to stop
// comparing. Staged answers that state with ErrNoIndex — the exit-0 escape hatch
// — on the entirely true grounds that no commit can carry a path outside the
// repository. True of the NEW value; silent about the old one. Repointing
// claims_dir at ../../elsewhere strands every claim the parent audited, which is
// a strictly LARGER collapse than any in-repository repoint, and it was reported
// as "there is nothing here to evaluate" with exit 0.
type claimsScope struct {
	// spec is claims_dir as a repository-relative pathspec, and is meaningless
	// when outside is true.
	spec string

	// outside is true when claims_dir resolves outside the git work tree, so
	// nothing in the repository can be inside it.
	outside bool

	// dir is claims_dir as the config resolved it, kept for the message: when
	// the scope is outside the repository there is no pathspec to name.
	dir string
}

// holds reports whether the repository-relative path p is inside this scope. A
// scope outside the work tree holds nothing that git can name, which is exactly
// why every claim under the parent's claims_dir is stranded by the move.
func (s claimsScope) holds(p string) bool {
	if s.outside {
		return false
	}
	return underSpec(s.spec, p)
}

// readable renders the scope for a message a human reads.
func (s claimsScope) readable() string {
	if s.outside {
		return s.dir + " — outside the repository"
	}
	return readableSpec(s.spec)
}

// parentProject is one parent commit reduced to what a comparison needs: the
// commit, and the project configuration THAT COMMIT held.
//
// The config is the whole point. Every path this gate compares — claims_dir, the
// lock ledger, the comment digest store — is resolved beside project.config.yaml,
// so reading the parent's paths out of the CURRENT config assumes the one thing
// an attacker gets to choose: where the config is. See parentProjects.
type parentProject struct {
	// sha is the parent commit.
	sha string

	// cfg is the parent's project.config.yaml, decoded and anchored at the
	// directory the PARENT kept it in — nil when that commit carried no config
	// this run could identify or decode.
	cfg *config.Config
}

// config is the parent's own configuration, falling back to the commit's when
// the parent's could not be identified.
//
// The fallback is what the store comparison used unconditionally before, and it
// is still the right answer for the case it was written for: a parent that kept
// its config exactly where this commit keeps it. Where it is a guess — a parent
// whose config could not be found at all — it can only ever look for a store at
// a path that commit did not have, which is silence, never a false refusal.
func (p parentProject) config(fallback *config.Config) *config.Config {
	if p.cfg != nil {
		return p.cfg
	}
	return fallback
}

// stagedScope compares the commit under judgement against its parent(s).
//
// scope is the index config's claims_dir as the registry was actually assembled
// over it — passed in rather than recomputed so the scope this reports on and
// the scope that was audited cannot drift apart.
//
// configSrc is the path the CALLER's project.config.yaml was read from, not the
// index config's (which has none — config.DecodeConfig takes a display name and
// leaves Path() empty). It is threaded through so a project whose config is not
// at the default name — `dossierx check --config docs/dossierx.yaml` — compares
// against the file it actually uses instead of one it does not have.
//
// configFromIndex gates the whole comparison, and that is a deliberate
// three-way split rather than a shortcut:
//
//   - config tracked: this is a project, and a change to its scope is this
//     function's business.
//   - config not tracked, index holds claims or a store: already refused,
//     upstream, as ErrUntrackedConfig — the dangerous middle never reaches here.
//   - config not tracked, index holds nothing: either the first commit of a
//     project (no parent to compare against anyway) or a commit that REMOVES the
//     project entirely. Reporting "your ledger disappeared" at someone deleting
//     a project on purpose would be a false accusation with no recovery.
func stagedScope(g *gitRunner, cfg *config.Config, configSrc string, scope claimsScope, configFromIndex bool) (scopeReport, error) {
	if !configFromIndex {
		return scopeReport{}, nil
	}

	head, ok, err := g.revision("HEAD")
	if err != nil {
		return scopeReport{}, err
	}
	if !ok {
		// An unborn HEAD: this is the repository's first commit. There is no
		// parent, so nothing here can have been removed or moved — every file in
		// it is new, and refusing a legitimate initial commit is the one false
		// positive that would make this feature indefensible.
		return scopeReport{}, nil
	}

	shas, judged, note, err := g.parentsUnderJudgement(head)
	if err != nil {
		return scopeReport{}, err
	}
	if note != "" || len(shas) == 0 {
		return scopeReport{Note: note}, nil
	}

	// Where each parent kept its project, resolved BEFORE any comparison, so
	// every question below is asked of the parent's own paths.
	parents, note, err := g.parentProjects(shas, configSrc)
	if err != nil {
		return scopeReport{}, err
	}

	var findings []lock.Finding
	f, err := removedIntegrityStores(g, cfg, parents, judged)
	if err != nil {
		return scopeReport{}, err
	}
	findings = append(findings, f...)

	f, err = droppedApprovals(g, cfg, parents, judged)
	if err != nil {
		return scopeReport{}, err
	}
	findings = append(findings, f...)

	f, err = strandedByScopeMove(g, scope, parents, judged)
	if err != nil {
		return scopeReport{}, err
	}
	return scopeReport{Findings: append(findings, f...), Note: note, Parents: parents}, nil
}

// parentProjects locates each parent's project.config.yaml IN THAT PARENT'S OWN
// TREE and decodes it, so the paths every comparison below resolves — claims_dir,
// the lock ledger, the comment digest store — are the paths that commit actually
// used.
//
// THE LOOKUP IS TWO-STEP, and the second step is the whole fix:
//
//  1. THE SAME PATH. If the parent carried a config where this commit carries
//     one, that is it — no search, one question, and the answer for every
//     project that is not being reorganised.
//
//  2. THE ONE THAT VANISHED. Otherwise the parent's tree is scanned for
//     project.config.yaml, and a candidate is kept only if the commit under
//     judgement no longer has a config at that same path. That test is what
//     makes this safe in a repository holding SEVERAL dossierx projects: a
//     config still sitting exactly where it was belongs to somebody else and is
//     never read as this project's. A config that is no longer there is one that
//     moved, and the file that moved is the file this project is now being run
//     from.
//
// Zero candidates is silence, and correctly so: the parent had no project here
// at all (the commit that adds dossierx to a repository), so there is no earlier
// scope to have narrowed. TWO OR MORE candidates is the one genuinely ambiguous
// state — two projects reorganising in one commit — and it is reported as an
// advisory rather than guessed at: picking the wrong config would compare this
// project's claims_dir against a stranger's and refuse honest work, and a gate
// that refuses honest work gets deleted. Saying "this could not be compared" is
// the same answer a shallow checkout gets, for the same reason.
//
// COST. Step 2 costs one "ls-tree -r --name-only" of the parent plus one
// ls-files per candidate, and it is reached only when the config is NOT where
// this commit keeps it — a project being reorganised, or the single commit that
// first adds a config. Every ordinary commit stops at step 1.
func (g *gitRunner) parentProjects(shas []string, configSrc string) ([]parentProject, string, error) {
	spec, specErr := g.spec(configSrc)

	var out []parentProject
	var ambiguous []string
	for _, sha := range shas {
		p := parentProject{sha: sha}

		found := ""
		if specErr == nil {
			held, err := g.treeHolds(sha, spec)
			if err != nil {
				return nil, "", err
			}
			if held {
				found = spec
			}
		}
		if found == "" {
			candidates, err := g.vanishedConfigs(sha)
			if err != nil {
				return nil, "", err
			}
			switch {
			case len(candidates) == 1:
				found = candidates[0]
			case len(candidates) > 1:
				ambiguous = append(ambiguous, shortRev(sha))
			}
		}
		if found != "" {
			cfg, ok, err := g.configFrom(sha, found)
			if err != nil {
				return nil, "", err
			}
			if ok {
				p.cfg = cfg
			}
		}
		out = append(out, p)
	}

	var note string
	if len(ambiguous) > 0 {
		note = fmt.Sprintf(
			"check --staged could NOT identify which %s the parent commit (%s) held for THIS project — it carries more than one, and none of them is at the path this commit uses, so more than one has moved. claims_dir and the integrity stores are resolved beside the config, so a claims_dir repointed by relocating the config would NOT have been reported on this run -> move one project at a time, or re-run this check on a commit where only one %s has moved.",
			config.FileName, strings.Join(ambiguous, ", "), config.FileName)
	}
	return out, note, nil
}

// vanishedConfigs lists the project.config.yaml paths rev carried that the
// commit under judgement no longer has AT THE SAME PATH, sorted.
//
// "No longer at the same path" is the discriminator, not "no longer anywhere":
// see parentProjects for why a config that has not moved belongs to a different
// project and must never be read as this one's.
//
// It matches on the DEFAULT file name, which is what a project reached through
// `--config docs/dossierx.yaml` is not called. That project is still compared
// correctly as long as its config stays put (step 1 of parentProjects looks it
// up by its own path); what this cannot do is follow such a file across a move,
// and the honest consequence — no candidate, so no comparison against that
// parent — is silence rather than a wrong answer.
func (g *gitRunner) vanishedConfigs(rev string) ([]string, error) {
	out, err := g.run("ls-tree", "-r", "-z", "--name-only", rev)
	if err != nil {
		return nil, err
	}
	var found []string
	for _, p := range splitZ(out) {
		if path.Base(p) != config.FileName {
			continue
		}
		tracked, err := g.lsFiles(p)
		if err != nil {
			return nil, err
		}
		if len(tracked) > 0 {
			// Still exactly where it was: another project's config, or this
			// one's if it never moved (in which case step 1 already found it).
			continue
		}
		found = append(found, p)
	}
	sort.Strings(found)
	return found, nil
}

// parentsUnderJudgement resolves WHAT is being judged and WHAT it is being
// compared against. See this file's header for the two cases and why they are
// not the same question.
//
// It returns the parent commits, a human-readable name for the thing under
// judgement (used in the findings, because "the index" and "HEAD" call for
// different recovery instructions), and a note that is non-empty only when the
// parents exist but are not present in this clone.
func (g *gitRunner) parentsUnderJudgement(head string) (parents []string, judged, note string, err error) {
	differs, err := g.indexDiffersFromHead()
	if err != nil {
		return nil, "", "", err
	}

	if differs {
		// A commit is being made. The index IS the commit under judgement and
		// HEAD is its parent — always present, in every clone, however shallow.
		parents = []string{head}
		// A conflicted merge is the one shape where pre-commit fires with a
		// second parent pending. Including it means the merge commit is judged
		// against BOTH sides, which is the honest reading of what it will
		// contain.
		if mh, ok, mErr := g.revision("MERGE_HEAD"); mErr == nil && ok && mh != head {
			parents = append(parents, mh)
		}
		return parents, "the git index", "", nil
	}

	// Nothing staged: the index is HEAD, so HEAD is the commit under judgement
	// and its own parents are what it has to be compared against.
	all, err := g.commitParents(head)
	if err != nil {
		return nil, "", "", err
	}
	if len(all) == 0 {
		// NO PARENTS — and that means two completely different things, which is
		// why --is-shallow-repository is consulted here and nowhere else.
		//
		// A shallow clone GRAFTS its boundary commit: git rewrites the commit's
		// parent list to empty for every traversal, so "git rev-list --parents
		// -n 1 HEAD" in a --depth 1 checkout is byte-identical to what it prints
		// for a genuine root commit. Verified: both print the sha alone. Reading
		// that as "the repository's first commit" would make the default
		// actions/checkout — depth 1 — the one configuration in which this whole
		// comparison silently does not happen, which is precisely the shape of
		// failure this file exists to end.
		//
		// So: shallow says the parents are unknown (an advisory naming the fix),
		// and not-shallow says there really are none (silence, because a
		// legitimate initial commit must not be refused or nagged).
		shallow, sErr := g.isShallow()
		if sErr != nil {
			return nil, "", "", sErr
		}
		if shallow {
			return nil, "", scopeUnverifiedNote(head, nil), nil
		}
		// The root commit, checked out clean. Nothing to compare.
		return nil, "", "", nil
	}
	var missing []string
	for _, p := range all {
		have, hErr := g.haveCommit(p)
		if hErr != nil {
			return nil, "", "", hErr
		}
		if have {
			parents = append(parents, p)
			continue
		}
		missing = append(missing, p)
	}
	if len(parents) == 0 {
		return nil, "", scopeUnverifiedNote(head, missing), nil
	}
	return parents, "HEAD", "", nil
}

// scopeUnverifiedNote is the advisory for the only state in which this
// comparison cannot be made: HEAD has a parent and this clone does not have it.
// That is a shallow checkout, and it is worth saying out loud rather than
// passing quietly, because "no scope change was detected" and "no scope change
// could be looked for" are not the same sentence and only one of them is true.
//
// missing may be empty: at a grafted shallow boundary git does not even report
// which parent it is withholding, so the message names HEAD instead and the
// recovery is the same either way.
func scopeUnverifiedNote(head string, missing []string) string {
	subject := shortRev(head) + "'s parent"
	if len(missing) > 0 {
		subject = shortRev(missing[0])
	}
	return fmt.Sprintf(
		"check --staged could NOT compare this commit's integrity scope against its parent (%s is not in this clone — a shallow checkout): a deleted lock ledger, a deleted comment digest store or a repointed claims_dir would NOT have been reported on this run -> deepen the checkout (actions/checkout with fetch-depth: 0, or git fetch --deepen=1) and re-run. A pre-commit hook is unaffected: there the comparison is against HEAD, which every clone has.",
		subject)
}

// shortRev abbreviates a full object name for a message a human reads. It is
// deliberately a fixed-width prefix rather than "git rev-parse --short", which
// would be another subprocess to render a hint.
func shortRev(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

// removedIntegrityStores reports a store the parent carried and this commit
// does not.
//
// A store that is absent from BOTH is not reported: a project that has never had
// a comment digest store (nothing has ever been commented on) or has never
// locked anything is not deleting something, and a rule that fires on correct
// state is a rule people learn to ignore. That also covers, exactly and without
// a special case, "the previous commit legitimately had no ledger" — the
// pre-adoption project.
//
// WITH TWO PARENTS (a merge) the trigger is "present in ANY parent", not "in
// all". The alternative reading — a merge may drop whatever one side already
// dropped — is how a deletion smuggled onto a branch with --no-verify arrives on
// main unexamined, and there is no such thing as a legitimate reason to delete
// these two files, so the noise this costs is zero and the coverage it buys is
// the whole merge path.
// THE PARENT'S COPY IS LOOKED FOR WHERE THE PARENT KEPT IT. Both stores are
// resolved beside project.config.yaml, so asking "did the parent carry a store
// at the path THIS commit uses?" answers no for a store that was there all along
// the moment the config moves — `git mv project.config.yaml docs/` without the
// stores, and a deletion reads as a file that never existed. See parentProjects.
func removedIntegrityStores(g *gitRunner, cfg *config.Config, parents []parentProject, judged string) ([]lock.Finding, error) {
	stores := []struct {
		name string
		at   func(*config.Config) string
		why  string
	}{
		{
			name: lockStoreFileName,
			at:   storePath,
			why:  "the lock ledger is the ONLY record of what a human approved. Every rule that could report a tampered locked claim starts from it, so removing it does not produce findings — it removes the ability to produce them",
		},
		{
			name: digest.StoreFileName,
			at:   digest.StorePath,
			why:  "the comment digest store is the fingerprint of the review history. Without it a hand-deleted comment thread — which is how a claim gets past the lock gate with a review still open — is not compared against anything",
		},
	}

	var findings []lock.Finding
	for _, s := range stores {
		spec, err := g.spec(s.at(cfg))
		if err != nil {
			// Outside the work tree entirely: no commit on either side could
			// have carried it, so there is nothing to compare.
			continue
		}
		tracked, err := g.lsFiles(spec)
		if err != nil {
			return nil, err
		}
		if len(tracked) > 0 {
			continue
		}
		for _, p := range parents {
			parentSpec, err := g.spec(s.at(p.config(cfg)))
			if err != nil {
				continue
			}
			held, err := g.treeHolds(p.sha, parentSpec)
			if err != nil {
				return nil, err
			}
			if !held {
				continue
			}
			moved := ""
			if parentSpec != spec {
				moved = fmt.Sprintf(" The parent kept it at %q and this commit looks for it at %q, because both stores are resolved beside %s — so relocating the config is one of the ways this file goes missing without anybody deleting it.", parentSpec, spec, config.FileName)
			}
			findings = append(findings, lock.Finding{
				Rule: RuleIntegrityStoreRemoved,
				Message: fmt.Sprintf(
					"%s was in the parent commit (%s) and is NOT in %s: this commit DELETES the file the integrity gate compares everything against. %s. That is why the rules below may be silent — they are not passing, they have nothing left to read.%s Restore it with \"git checkout %s -- %s\" and commit again. There is exactly one other way out, and it is the honest one for a project that is genuinely done with dossierx: remove %s in the same commit. This comparison is skipped entirely once the config is no longer tracked, because a project being deleted is not a project whose ledger went missing.",
					s.name, shortRev(p.sha), judged, s.why, moved, shortRev(p.sha), parentSpec, config.FileName),
			})
			break
		}
	}
	return findings, nil
}

// droppedApprovals reports a lock ledger that is STILL THERE and no longer holds
// standing approvals the parent recorded in it, for claims this commit still
// carries. It is RuleIntegrityStoreRemoved's content half — see that rule for
// why emptying the file and deleting it are one event under one name.
//
// THE PREDICATE, and every clause of it is load-bearing:
//
//   - the parent held a STANDING (unreleased) CLAIM record. A released record
//     describes a claim a human deliberately unlocked, and a build-order record
//     is somebody else's rule.
//
//   - the commit's store holds NO RECORD AT ALL under that key. Not "no standing
//     record" — no record. That single word is what makes the honest path
//     silent: `dossierx claim unlock` RELEASES a record (lock.ReleaseApproval)
//     and nothing in this product ever deletes one, so "unlock -> fix -> lock"
//     leaves a released record in the middle and a fresh standing one at the
//     end, and neither step is reported here. A record that has been removed
//     outright was removed by hand.
//
//   - the claim is STILL TRACKED in the commit. This is the difference between
//     an approval that was erased and a project that was legitimately dismantled:
//     the harm is a locked artifact left in the repository with nothing auditing
//     it, and a claim that is genuinely gone leaves nothing behind to audit. A
//     locked claim deleted WITHOUT an unlock keeps its standing record and is
//     reported by lock-ledger-abandoned, which is the correct division of labour
//     — this rule is about the ledger, that one is about the claim.
//
// It deliberately does not care what the surviving claim's status says. Flipping
// `status: locked` to draft in the same commit that deletes the record is the
// cheaper laundering of the two (nothing at all is left pointing at the erased
// approval), and a predicate that asked for `locked` would have excused exactly
// that edit while catching the noisier one.
//
// THE COMMENT DIGEST STORE gets no content rule here, and does not need one:
// comment-digest-missing already reads that store's CONTENT against the ledger's
// standing approvals, once per claim, so emptying the map is reported there.
// This is the ledger's own missing half — nothing else compares the ledger
// against anything but itself.
func droppedApprovals(g *gitRunner, cfg *config.Config, parents []parentProject, judged string) ([]lock.Finding, error) {
	current, ok, err := g.indexStore(storePath(cfg))
	if err != nil {
		return nil, err
	}
	if !ok {
		// No store in the commit at all: that is removedIntegrityStores' finding
		// (or a project that has never locked anything), and reporting the same
		// deletion twice would bury the recovery.
		return nil, nil
	}

	for _, p := range parents {
		previous, ok, err := g.treeStore(p.sha, storePath(p.config(cfg)))
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}

		want := map[string]bool{}
		for _, id := range standingClaimIDs(previous) {
			if _, held := current.Record(id); held {
				continue
			}
			want[id] = true
		}
		if len(want) == 0 {
			continue
		}

		dropped, err := indexClaimsWithIDs(g, want)
		if err != nil {
			return nil, err
		}
		if len(dropped) == 0 {
			continue
		}
		return []lock.Finding{{
			Rule: RuleIntegrityStoreRemoved,
			Message: fmt.Sprintf(
				"%s is still in %s, and the approval(s) the parent commit (%s) recorded in it for %d claim(s) STILL IN THIS COMMIT are gone from it: %s. They were not released by an unlock — an unlock keeps the record and stamps it released — they were removed, which costs this gate exactly what deleting the whole file costs: every rule that could report a tampered locked claim starts from these records. Emptying the map is the cheap version of the deletion, and it survives a review diff far better. Restore the ledger with \"git checkout %s -- %s\" and commit again, or — if those claims are genuinely leaving the approval path — unlock them through it (dossierx claim unlock <id> --reason \"...\"), which records the release instead of erasing the approval.",
				lockStoreFileName, judged, shortRev(p.sha), len(dropped), strings.Join(dropped, ", "),
				shortRev(p.sha), mustSpec(g, storePath(p.config(cfg)))),
		}}, nil
	}
	return nil, nil
}

// parentLedgerContent is RULE A applied PER CLAIM, and it is the half of the
// content comparison that droppedApprovals deliberately does not cover.
//
// droppedApprovals asks one project-scoped question — "is a standing approval
// the parent held simply GONE from a store that is still there?" — and answers
// it as the same scope collapse as deleting the file. That predicate keys on
// "this commit's store holds NO RECORD AT ALL under that key", which is what
// keeps unlock -> fix -> lock silent, and it is also what makes it blind to the
// two erasures that REPLACE the evidence instead of removing it:
//
//   - the three-key erasure. Delete the ledger record, the locked_at stamp and
//     the dependency baselines together, flip status back to draft, edit the
//     body, and re-lock. The store ends the commit holding a record for that
//     claim — a fresh one, self-issued, over content nobody approved — so
//     "no record at all" is false and droppedApprovals says nothing. Every rule
//     in lock.Audit agrees with it, because from inside this one directory the
//     claim is indistinguishable from one locked honestly for the first time.
//
//   - the erased review. Delete a human's open comment thread from a DRAFT
//     claim AND its comment-digest entry in the same commit, then lock. The
//     thread is what blocks the lock and the digest entry is what proves the
//     thread existed; with both gone the claim is indistinguishable from one
//     that was never commented on, and it locks clean.
//
// Both are answered by the same evidence and it is not in this commit: the
// PARENT's copy of the two stores. lock.AuditAgainstParent owns those rules —
// see its doc comment for why it mints no new rule names and why it never
// double-reports with lock.Audit — and this function is the caller it names,
// whose entire job is the precondition that function cannot check for itself:
// EACH SIDE IS RESOLVED AGAINST ITS OWN CONFIG. The parent's stores are read
// from the paths the PARENT's config points at (RULE B, via parentProject),
// because resolving both sides against the current config would read an empty
// store for the parent the moment a project is legitimately relocated and
// report every claim in it as erased — the outage this gate exists to prevent.
//
// It is NOT called on the out-of-worktree branch of Staged. The registry there
// is empty by construction — the claims are somewhere this repository cannot
// name — so every claim the parent locked would come back as "the claim file is
// gone too", which is both wrong and a second, noisier name for the
// claims-scope-narrowed refusal that branch already returns.
func parentLedgerContent(g *gitRunner, cfg *config.Config, parents []parentProject, claims []model.Claim, in ledgerInputs) ([]lock.Finding, error) {
	for _, p := range parents {
		previous, ok, err := g.treeStore(p.sha, storePath(p.config(cfg)))
		if err != nil {
			return nil, err
		}
		if !ok {
			// The parent had no lock store this run could read. That is
			// removedIntegrityStores' finding when it is a deletion, and a
			// project that had not locked anything yet otherwise; either way
			// there is no earlier approval here to have been erased.
			continue
		}

		// A parent digest store that is absent stays nil, which is exactly the
		// signal AuditAgainstParent's comment half skips on — an absent store is
		// comment-digest-absent's business, said once, project-scoped.
		var parentDigests *digest.Store
		if d, ok, err := g.treeDigests(p.sha, digest.StorePath(p.config(cfg))); err != nil {
			return nil, err
		} else if ok {
			parentDigests = d
		}

		if f := lock.AuditAgainstParent(claims, in.store, previous, in.digests, parentDigests); len(f) > 0 {
			// One parent's evidence is enough, and reporting the same erasure
			// once per parent of a merge would say it twice for one event.
			return f, nil
		}
	}
	return nil, nil
}

// mustSpec is spec() for a message: a path that could not be expressed as a
// pathspec is rendered as itself rather than failing a finding that has already
// been decided. Nothing branches on the result.
func mustSpec(g *gitRunner, target string) string {
	if spec, err := g.spec(target); err == nil {
		return spec
	}
	return target
}

// standingClaimIDs is the set of claim ids a store holds an UNRELEASED CLAIM
// approval for, sorted. Build-order records are filtered out here, exactly as
// lock.Audit filters them, so the two agree about what a claim record is.
func standingClaimIDs(s *lock.Store) []string {
	var ids []string
	for id, r := range s.Ledger {
		if r.Subject != lock.SubjectClaim || r.Released() {
			continue
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// indexClaimsWithIDs reports which of the wanted claim ids the INDEX still holds
// a file for, sorted, each rendered as "<id> (<path>)" for the message.
//
// It scans the WHOLE index rather than claims_dir, and that is the point: a
// claim moved out of the audited scope is precisely the one that stops being
// reported, so a presence test scoped to claims_dir would answer "gone" for the
// claims most in need of an answer. It applies the same decode
// indexHoldsJudgeableContent applies, for the same reason — a repository full of
// ordinary yaml must not turn into a refusal.
//
// The cost, one ls-files plus one cat-file over the repository's yaml, is paid
// only when a parent's standing approval is missing from a store that is still
// present — a state no honest path produces.
func indexClaimsWithIDs(g *gitRunner, want map[string]bool) ([]string, error) {
	entries, err := g.indexEntries()
	if err != nil {
		return nil, err
	}
	var candidates []indexEntry
	for _, e := range entries {
		if isClaimFile(e.path) {
			candidates = append(candidates, e)
		}
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	blobs, err := g.catFile(candidates)
	if err != nil {
		return nil, err
	}

	var found []string
	for p, raw := range blobs {
		c, err := decodeClaim(p, raw)
		if err != nil || !want[strings.TrimSpace(c.ID)] {
			continue
		}
		found = append(found, fmt.Sprintf("%s (%s)", c.ID, p))
	}
	sort.Strings(found)
	return found, nil
}

// strandedByScopeMove reports a claims_dir move that left tracked claim files
// outside the new scope.
//
// It asks the question per parent and stops at the first parent that answers
// yes, because the finding is about the commit under judgement, not about which
// ancestor it disagrees with — repeating one refusal once per parent of a merge
// would bury it.
//
// A parent whose project.config.yaml cannot be read or decoded contributes
// nothing: the parent's claims_dir is unknown, so no comparison against it is
// possible. That direction is safe — an undecodable config in the PARENT is a
// commit that already could not pass this gate — and guessing would be worse
// than saying nothing.
// A PARENT WHOSE claims_dir CANNOT BE EXPRESSED as a pathspec — it was outside
// the work tree — contributes nothing either: no commit could have carried
// claims there, so moving away from it strands nothing that was ever audited.
// The reverse direction is not symmetric and must not be treated as if it were.
// A claims_dir that LEAVES the work tree strands everything, and is compared
// here rather than short-circuiting to ErrNoIndex; see claimsScope.
func strandedByScopeMove(g *gitRunner, scope claimsScope, parents []parentProject, judged string) ([]lock.Finding, error) {
	for _, p := range parents {
		if p.cfg == nil {
			continue
		}
		parentSpec, err := g.spec(p.cfg.ClaimsDir)
		if err != nil {
			continue
		}
		if !scope.outside && parentSpec == scope.spec {
			continue
		}
		stranded, err := strandedClaimPaths(g, parentSpec, scope)
		if err != nil {
			return nil, err
		}
		if len(stranded) == 0 {
			// A move that took its claims with it, or one that widened the
			// scope. Both are honest reorganisations and both are silent — see
			// this file's header for why that is the sanctioned path.
			continue
		}
		escaped := ""
		if scope.outside {
			escaped = " claims_dir now resolves OUTSIDE this repository, so nothing in the repository can be inside it and no commit can carry what it points at — which is why this run would otherwise have reported \"nothing to evaluate\" and exited 0."
		}
		return []lock.Finding{{
			Rule: RuleClaimsScopeNarrowed,
			Message: fmt.Sprintf(
				"claims_dir moved from %q to %q between the parent commit (%s) and %s, and %d claim file(s) were LEFT BEHIND outside the new scope: %s.%s Those files are still tracked and still say status: locked, and nothing audits them any more — the scope of this gate is decided entirely by data inside the commit being judged, so repointing claims_dir takes claims out of the gate without breaking a single rule, and MOVING THE CONFIG repoints it without editing the line. A move that TAKES ITS CLAIMS WITH IT is the sanctioned way to reorganise, and it is accepted here in silence: git mv the claim files into the new directory in the SAME commit that edits claims_dir, so nothing is left outside it. To fix this commit: move the listed file(s) under %q, or revert the claims_dir edit, or — if those claims are genuinely being retired — unlock them through the approval path (dossierx claim unlock <id> --reason \"...\") and delete them, so the release is on the record.",
				readableSpec(parentSpec), scope.readable(), shortRev(p.sha), judged,
				len(stranded), strings.Join(stranded, ", "), escaped, scope.readable()),
		}}, nil
	}
	return nil, nil
}

// readableSpec renders a pathspec for a message a human reads. The pathspec IS
// the repository-relative form, which is the form a reader can paste into a git
// command, so it is used as-is; the helper exists only so the "." the spec uses
// for the repository root reads as something a human recognises.
func readableSpec(spec string) string {
	if spec == "." || spec == "" {
		return "the repository root"
	}
	return spec
}

// strandedClaimPaths lists the index entries under oldSpec that are NOT under
// newSpec and that DECODE as claims, sorted.
//
// The decode is what keeps this off ordinary yaml. A project that moves
// claims_dir out of a directory that also holds a linter config, a workflow or a
// chart values file leaves those behind quite deliberately, and only a blob the
// engine would have loaded as a claim is evidence that coverage was lost.
func strandedClaimPaths(g *gitRunner, oldSpec string, scope claimsScope) ([]string, error) {
	entries, err := g.indexEntries(oldSpec)
	if err != nil {
		return nil, err
	}
	var candidates []indexEntry
	for _, e := range entries {
		if !isClaimFile(e.path) || scope.holds(e.path) {
			continue
		}
		candidates = append(candidates, e)
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	blobs, err := g.catFile(candidates)
	if err != nil {
		return nil, err
	}
	var stranded []string
	for p, raw := range blobs {
		if c, decErr := decodeClaim(p, raw); decErr == nil && strings.TrimSpace(c.ID) != "" {
			stranded = append(stranded, p)
		}
	}
	sort.Strings(stranded)
	return stranded, nil
}

// underSpec reports whether the repository-relative path p lies within the
// pathspec spec. "." is the repository root and contains everything.
func underSpec(spec, p string) bool {
	if spec == "." || spec == "" {
		return true
	}
	return p == spec || strings.HasPrefix(p, spec+"/")
}

// ---------------------------------------------------------------------
// the git binary, history half
// ---------------------------------------------------------------------

// runQuiet is run() for questions git answers with an EXIT STATUS: "does this
// revision resolve?", "do I have this object?", "is this path in that tree?".
// A non-zero exit is the answer "no", not a failure, so it comes back as
// ok=false with a nil error; anything that is not git exiting non-zero (git
// vanished, fork failed) is still a real error, because a gate must not read
// "the tool broke" as "nothing to report".
func (g *gitRunner) runQuiet(args ...string) (out []byte, ok bool, err error) {
	full := append([]string{"-c", "core.quotepath=false", "-c", "diff.relative=false"}, args...)
	cmd := exec.Command(g.bin, full...) //nolint:gosec // fixed binary, fixed argv; no shell involved
	cmd.Dir = g.dir
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err = cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// git ran and said no. That is an answer, not a failure.
			return nil, false, nil
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, false, fmt.Errorf("check --staged: git %s: %s", strings.Join(args, " "), msg)
	}
	return out, true, nil
}

// revision resolves rev to a full object name, reporting ok=false when it does
// not resolve at all (an unborn HEAD, or a MERGE_HEAD that is not there because
// no merge is in progress).
func (g *gitRunner) revision(rev string) (string, bool, error) {
	out, ok, err := g.runQuiet("rev-parse", "--verify", "--quiet", rev+"^{commit}")
	if err != nil || !ok {
		return "", false, err
	}
	sha := strings.TrimSpace(string(out))
	if sha == "" {
		return "", false, nil
	}
	return sha, true, nil
}

// indexDiffersFromHead reports whether the index holds anything HEAD's tree does
// not, or vice versa — i.e. whether a commit is actually being made.
//
// "diff-index --cached" compares the INDEX against a TREE and never looks at the
// working directory, so this answer is unaffected by the stat cache, by
// --assume-unchanged, and by whatever the author has half-edited on disk. That
// matters: the whole point of --staged is that the verdict follows the index,
// and a question about which parent to compare against must follow it too.
func (g *gitRunner) indexDiffersFromHead() (bool, error) {
	out, err := g.run("diff-index", "--cached", "-z", "--name-only", "HEAD")
	if err != nil {
		return false, err
	}
	return len(splitZ(out)) > 0, nil
}

// commitParents lists the parents of sha, in order, empty for a root commit.
func (g *gitRunner) commitParents(sha string) ([]string, error) {
	out, err := g.run("rev-list", "--parents", "-n", "1", sha)
	if err != nil {
		return nil, err
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) < 2 {
		return nil, nil
	}
	return fields[1:], nil
}

// haveCommit reports whether this clone actually holds the commit object sha.
//
// It is the shallow-clone test, asked the only way that cannot be wrong: a
// shallow clone's grafted boundary means HEAD names a parent it does not have,
// and "git rev-parse --is-shallow-repository" tells you the repository is
// shallow without telling you whether THIS parent is one of the missing ones (a
// --depth=50 clone of a 3-commit branch is shallow and complete). Asking for the
// object answers the question that is actually being asked.
func (g *gitRunner) haveCommit(sha string) (bool, error) {
	_, ok, err := g.runQuiet("cat-file", "-e", sha+"^{commit}")
	return ok, err
}

// isShallow reports whether this repository has a grafted history boundary —
// the state a "git clone --depth N" or an actions/checkout without fetch-depth
// leaves behind. See parentsUnderJudgement for why the answer cannot be inferred
// from the parent list instead.
func (g *gitRunner) isShallow() (bool, error) {
	out, err := g.run("rev-parse", "--is-shallow-repository")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) == "true", nil
}

// treeHolds reports whether the commit rev's tree contains anything matching
// spec.
func (g *gitRunner) treeHolds(rev, spec string) (bool, error) {
	out, err := g.run("ls-tree", "-r", "-z", "--name-only", rev, "--", spec)
	if err != nil {
		return false, err
	}
	return len(splitZ(out)) > 0, nil
}

// configFrom loads the project.config.yaml the commit rev held AT SPEC,
// returning ok=false when that commit did not carry it there or when what it
// carried does not decode.
//
// IT IS ANCHORED AT THE PARENT'S OWN DIRECTORY, not at the current config's.
// Everything path-shaped in a config — claims_dir, and through cfg.Dir() both
// integrity stores — is resolved relative to the file's own location, so
// decoding the parent's config against the CURRENT config's directory answers
// "where would the parent's claims_dir point if the parent had kept its config
// where this commit keeps it?", which is a question about a tree that does not
// exist. Where the config has not moved the two are the same directory and this
// is a no-op; where it has moved, this is the difference between comparing the
// parent's scope and comparing a fiction.
//
// A parent whose config does not DECODE contributes nothing, and that direction
// is safe: an undecodable config in the parent is a commit that could not have
// passed this gate itself, and guessing at its claims_dir would be worse than
// saying nothing.
func (g *gitRunner) configFrom(rev, spec string) (*config.Config, bool, error) {
	raw, ok, err := g.runQuiet("show", rev+":"+spec)
	if err != nil || !ok {
		return nil, false, err
	}
	dir, err := g.worktreeDir(path.Dir(spec))
	if err != nil {
		return nil, false, nil
	}
	parsed, err := config.DecodeConfig(raw, dir, fmt.Sprintf("%s (as of %s)", spec, shortRev(rev)))
	if err != nil {
		return nil, false, nil
	}
	return parsed, true, nil
}

// worktreeDir maps a repository-relative pathspec back to an absolute path in
// the CALLER's namespace. It is the exact inverse of spec().
//
// It cannot be built by joining onto g.dir, and the reason is the one
// gitRunner.prefix exists for: on macOS the caller reaches a temp directory
// through /var while git resolves it to /private/var, so a path assembled from
// git's top level cannot be made relative to the caller's own and every
// subsequent spec() of it would come back "outside the work tree". Both ends
// here are g.base and g.prefix, which git itself put in agreement.
func (g *gitRunner) worktreeDir(spec string) (string, error) {
	rel, err := filepath.Rel(filepath.FromSlash(g.prefix), filepath.FromSlash(spec))
	if err != nil {
		return "", err
	}
	return filepath.Join(g.base, rel), nil
}

// indexStore loads the lock store the INDEX holds at the absolute path src,
// reporting ok=false when the index does not carry it or when it does not
// decode.
//
// An undecodable store is ok=false rather than an error on purpose: it is
// already reported, with its own recovery, as lock-ledger-unreadable, and a
// second finding saying the same file is broken would only compete with it.
func (g *gitRunner) indexStore(src string) (*lock.Store, bool, error) {
	spec, err := g.spec(src)
	if err != nil {
		return nil, false, nil
	}
	tracked, err := g.lsFiles(spec)
	if err != nil {
		return nil, false, err
	}
	if len(tracked) == 0 {
		return nil, false, nil
	}
	raw, err := g.showIndexBlob(tracked[0])
	if err != nil {
		return nil, false, err
	}
	store, err := decodeLockStore(raw)
	if err != nil {
		return nil, false, nil
	}
	return store, true, nil
}

// treeStore is indexStore for a COMMIT's tree rather than the index.
func (g *gitRunner) treeStore(rev, src string) (*lock.Store, bool, error) {
	spec, err := g.spec(src)
	if err != nil {
		return nil, false, nil
	}
	raw, ok, err := g.runQuiet("show", rev+":"+spec)
	if err != nil || !ok {
		return nil, false, err
	}
	store, err := decodeLockStore(raw)
	if err != nil {
		return nil, false, nil
	}
	return store, true, nil
}

// decodeLockStore decodes a lock store from BYTES by materializing them into a
// temp file and handing that to lock.LoadStore — the same trick
// stagedLedgerInputs uses, and for the same reason: every store in this product
// is loaded by path, and adding a decode-from-bytes entry point to internal/lock
// for one caller is how two decoders start disagreeing about what a ledger
// record is.
//
// The temp directory is removed before this returns, which leaves the store
// holding a path that does not exist — a structural guarantee that nothing on
// this path can Save it, on exactly the terms stagedLedgerInputs relies on.
func decodeLockStore(raw []byte) (*lock.Store, error) {
	dir, err := os.MkdirTemp("", "dossierx-scope-")
	if err != nil {
		return nil, fmt.Errorf("check --staged: temp dir: %w", err)
	}
	// Best-effort cleanup: the run's verdict must not depend on whether a temp
	// directory could be removed.
	defer os.RemoveAll(dir) //nolint:errcheck // best-effort cleanup of our own temp dir

	out := filepath.Join(dir, lockStoreFileName)
	if err := os.WriteFile(out, raw, 0o600); err != nil {
		return nil, fmt.Errorf("check --staged: stage %s for reading: %w", lockStoreFileName, err)
	}
	return lock.LoadStore(out)
}

// treeDigests is treeStore for the COMMENT DIGEST STORE. It is a separate
// function rather than a generic one because the two stores have different
// loaders and different "absent" semantics, and collapsing them behind an
// interface would hide exactly the distinction both callers depend on: a digest
// store that is ABSENT is not a digest store that is EMPTY, and
// lock.AuditAgainstParent's comment half skips the former and reports on the
// latter.
func (g *gitRunner) treeDigests(rev, src string) (*digest.Store, bool, error) {
	spec, err := g.spec(src)
	if err != nil {
		return nil, false, nil
	}
	raw, ok, err := g.runQuiet("show", rev+":"+spec)
	if err != nil || !ok {
		return nil, false, err
	}
	store, err := decodeDigestStore(raw)
	if err != nil {
		return nil, false, nil
	}
	return store, true, nil
}

// decodeDigestStore is decodeLockStore for the comment digest store, on exactly
// the same terms and for the same reason: every store in this product is loaded
// by path, so the bytes are materialized into a temp file and handed to the one
// loader rather than growing a second decoder that can disagree with it about
// what an entry is.
func decodeDigestStore(raw []byte) (*digest.Store, error) {
	dir, err := os.MkdirTemp("", "dossierx-scope-")
	if err != nil {
		return nil, fmt.Errorf("check --staged: temp dir: %w", err)
	}
	// Best-effort cleanup: the run's verdict must not depend on whether a temp
	// directory could be removed.
	defer os.RemoveAll(dir) //nolint:errcheck // best-effort cleanup of our own temp dir

	out := filepath.Join(dir, digest.StoreFileName)
	if err := os.WriteFile(out, raw, 0o600); err != nil {
		return nil, fmt.Errorf("check --staged: stage %s for reading: %w", digest.StoreFileName, err)
	}
	return digest.LoadStore(out)
}
