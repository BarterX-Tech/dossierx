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
	"os/exec"
	"path"
	"sort"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/digest"
	"github.com/BarterX-Tech/dossierx/internal/lock"
)

// RuleIntegrityStoreRemoved: a tracked integrity store the PARENT commit
// carried — the lock ledger, or the comment digest store — is not in the commit
// under judgement.
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
	// checkout whose parent commit was never fetched. It rides in
	// Result.NextSteps rather than in Findings: it is not evidence of a tamper,
	// it is evidence that this run could not look, and refusing on it would
	// break every default CI checkout in existence.
	Note string
}

// stagedScope compares the commit under judgement against its parent(s).
//
// claimsSpec is the index config's claims_dir as a repository-relative pathspec
// — the same value Staged assembled the registry from, passed in rather than
// recomputed so the scope this reports on and the scope that was actually
// audited cannot drift apart.
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
func stagedScope(g *gitRunner, cfg *config.Config, claimsSpec string, configFromIndex bool) (scopeReport, error) {
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

	parents, judged, note, err := g.parentsUnderJudgement(head)
	if err != nil {
		return scopeReport{}, err
	}
	if note != "" || len(parents) == 0 {
		return scopeReport{Note: note}, nil
	}

	var findings []lock.Finding
	f, err := removedIntegrityStores(g, cfg, parents, judged)
	if err != nil {
		return scopeReport{}, err
	}
	findings = append(findings, f...)

	f, err = strandedByScopeMove(g, cfg, claimsSpec, parents, judged)
	if err != nil {
		return scopeReport{}, err
	}
	return scopeReport{Findings: append(findings, f...)}, nil
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
func removedIntegrityStores(g *gitRunner, cfg *config.Config, parents []string, judged string) ([]lock.Finding, error) {
	stores := []struct {
		name string
		path string
		why  string
	}{
		{
			name: lockStoreFileName,
			path: storePath(cfg),
			why:  "the lock ledger is the ONLY record of what a human approved. Every rule that could report a tampered locked claim starts from it, so removing it does not produce findings — it removes the ability to produce them",
		},
		{
			name: digest.StoreFileName,
			path: digest.StorePath(cfg),
			why:  "the comment digest store is the fingerprint of the review history. Without it a hand-deleted comment thread — which is how a claim gets past the lock gate with a review still open — is not compared against anything",
		},
	}

	var findings []lock.Finding
	for _, s := range stores {
		spec, err := g.spec(s.path)
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
			held, err := g.treeHolds(p, spec)
			if err != nil {
				return nil, err
			}
			if !held {
				continue
			}
			findings = append(findings, lock.Finding{
				Rule: RuleIntegrityStoreRemoved,
				Message: fmt.Sprintf(
					"%s was in the parent commit (%s) and is NOT in %s: this commit DELETES the file the integrity gate compares everything against. %s. That is why the rules below may be silent — they are not passing, they have nothing left to read. Restore it with \"git checkout %s -- %s\" and commit again. There is exactly one other way out, and it is the honest one for a project that is genuinely done with dossierx: remove %s in the same commit. This comparison is skipped entirely once the config is no longer tracked, because a project being deleted is not a project whose ledger went missing.",
					s.name, shortRev(p), judged, s.why, shortRev(p), spec, config.FileName),
			})
			break
		}
	}
	return findings, nil
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
func strandedByScopeMove(g *gitRunner, cfg *config.Config, claimsSpec string, parents []string, judged string) ([]lock.Finding, error) {
	for _, p := range parents {
		parentCfg, ok, err := g.configAt(p, cfg)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		parentSpec, err := g.spec(parentCfg.ClaimsDir)
		if err != nil {
			// The parent's claims_dir was outside the work tree, so no commit
			// could have carried claims there and nothing can have been
			// stranded by moving away from it.
			continue
		}
		if parentSpec == claimsSpec {
			continue
		}
		stranded, err := strandedClaimPaths(g, parentSpec, claimsSpec)
		if err != nil {
			return nil, err
		}
		if len(stranded) == 0 {
			// A move that took its claims with it, or one that widened the
			// scope. Both are honest reorganisations and both are silent — see
			// this file's header for why that is the sanctioned path.
			continue
		}
		return []lock.Finding{{
			Rule: RuleClaimsScopeNarrowed,
			Message: fmt.Sprintf(
				"claims_dir moved from %q to %q between the parent commit (%s) and %s, and %d claim file(s) were LEFT BEHIND outside the new scope: %s. Those files are still tracked and still say status: locked, and nothing audits them any more — the scope of this gate is decided entirely by data inside the commit being judged, so repointing claims_dir takes claims out of the gate without breaking a single rule. A move that TAKES ITS CLAIMS WITH IT is the sanctioned way to reorganise, and it is accepted here in silence: git mv the claim files into the new directory in the SAME commit that edits claims_dir, so nothing is left outside it. To fix this commit: move the listed file(s) under %q, or revert the claims_dir edit, or — if those claims are genuinely being retired — unlock them through the approval path (dossierx claim unlock <id> --reason \"...\") and delete them, so the release is on the record.",
				readableSpec(parentSpec), readableSpec(claimsSpec), shortRev(p), judged,
				len(stranded), strings.Join(stranded, ", "), readableSpec(claimsSpec)),
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
func strandedClaimPaths(g *gitRunner, oldSpec, newSpec string) ([]string, error) {
	entries, err := g.indexEntries(oldSpec)
	if err != nil {
		return nil, err
	}
	var candidates []indexEntry
	for _, e := range entries {
		if !isClaimFile(e.path) || underSpec(newSpec, e.path) {
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

// configAt loads project.config.yaml as the commit rev held it, returning
// ok=false when that commit did not carry it or when what it carried does not
// decode.
//
// The WORKTREE directory stays the anchor, exactly as stagedConfig anchors the
// index's copy: claims_dir is resolved against cfg.Dir() so the parent's scope
// and this commit's scope are expressed in the same namespace and can be
// compared at all.
func (g *gitRunner) configAt(rev string, cfg *config.Config) (*config.Config, bool, error) {
	src := cfg.Path()
	if src == "" {
		src = path.Join(cfg.Dir(), config.FileName)
	}
	spec, err := g.spec(src)
	if err != nil {
		return nil, false, nil
	}
	raw, ok, err := g.runQuiet("show", rev+":"+spec)
	if err != nil || !ok {
		return nil, false, err
	}
	parsed, err := config.DecodeConfig(raw, cfg.Dir(), fmt.Sprintf("%s (as of %s)", src, shortRev(rev)))
	if err != nil {
		return nil, false, nil
	}
	return parsed, true, nil
}
