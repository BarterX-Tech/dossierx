// staged.go implements "dossierx check --staged": the whole check gate
// evaluated against the GIT INDEX instead of the working tree. It is the first
// and only git integration in this engine, and it is deliberately nothing more
// than os/exec around the git binary — no library, no dependency, no assumption
// about where .git lives or whether the caller is in a worktree, a submodule,
// or a repo with core.hooksPath pointed somewhere unusual. Everything we need,
// git will answer if asked.
//
// WHY THE INDEX. This is what a pre-commit hook has to validate. The working
// tree is whatever the author is midway through typing; the index is what the
// commit will contain. A hook that validated the worktree would pass a commit
// that stages a compliant blob and leaves the tampered version unstaged (or the
// reverse: refuse a commit that is perfectly fine because an unrelated file is
// dirty). Both are worse than no hook, because a gate that answers a question
// nobody asked teaches people to pass --no-verify.
//
// WHY IT WRITES NOTHING. The hook runs as part of the commit the user is
// making. If validating dirtied the tree — reconciled review_pending into claim
// files, saved a migrated lock store, rewrote .catalog.json and the viewer, all
// of which plain "dossierx check" does on every run — the hook would change the
// thing it was asked to judge, mid-commit, behind the user's back. So --staged
// drives check.StatusStaged, the non-writing sibling that internal/serve
// already relies on for exactly the same reason (a bare GET must not truncate
// the viewer). The guarantee is testable and is tested: "git status --porcelain"
// is byte-identical before and after.
//
// WHY THE WHOLE REGISTRY, NOT JUST THE STAGED FILES. Most of the lint suite is
// whole-corpus by construction — dangling references, cycles, mirror
// reciprocity, hub gating — so a claim can only be judged against every other
// claim. Staging one file and linting one file would report a dangling
// reference for every edge that points outside the commit. So the registry is
// assembled complete, with the index's content substituted in.
//
// ONE THING HERE IS DELIBERATELY NOT FROM THE INDEX. Result.NextSteps —
// check.Status's non-blocking "what to run next" advisory — reads the flag
// store and the build-order artifacts off disk, because those are advice about
// what the author should do next, not a verdict on the commit. They cannot
// change the pass/fail answer (nothing in runCheckStaged consults them), and
// re-plumbing two more stores through the index to improve the wording of a
// hint would be cost without a corresponding property.
//
// WHY THE STORES COME FROM THE INDEX TOO. The lock ledger is a tracked,
// committed file, and reading it from the worktree while reading claims from
// the index would break the one property that makes the ledger worth having in
// a hook: a commit that carries a newly-locked claim WITHOUT its approval record
// must be refused. Read from the worktree, that commit passes — the worktree
// ledger has the record, it just is not being committed — and the record lands
// in a later commit, or never. Read from the index, the claim and its approval
// have to travel together, which is exactly the invariant.
//
// ---------------------------------------------------------------------
// REMOVED, DELIBERATELY: THE COMPARISON AGAINST THE PARENT COMMIT
// ---------------------------------------------------------------------
//
// This file used to have a sibling, history.go, and it is gone. A future reader
// who finds the hole it filled and reaches for the same fix should read this
// first, because the hole is real, the diagnosis was right, and the fix was
// still in the wrong layer.
//
// WHAT WAS BUILT. `check --staged` compared the commit under judgement against
// its PARENT commit and refused two changes no single tree can see: an integrity
// store the parent carried that this commit deletes or empties
// (`integrity-store-removed`), and a claims_dir that moved and left tracked claim
// files outside the new scope (`claims-scope-narrowed`). A per-claim half read
// the parent's two stores through lock.AuditAgainstParent for approvals this
// commit had REPLACED rather than removed. It existed because the gate's SCOPE —
// which files the rules run over, and whether there is a ledger to compare them
// against — is itself data inside the tree being judged, so one commit could
// repoint claims_dir AND delete the ledger and leave every individual rule
// behaving perfectly over an empty registry.
//
// WHY IT WAS REMOVED. The parent commit is outside the COMMIT but it is not
// outside the COMMITTER. Git history is written by the same person the gate is
// judging, so `git checkout --orphan`, a second config file, an interactive
// rebase or a squash all move the comparison's other side — the control is
// evaded by the party it constrains, which is the definition of the wrong layer.
// And because it reasons about two trees, it has to infer INTENT that neither
// tree records, which it cannot do. That produced two refusals of ordinary git
// work, both measured:
//
//   - `git revert` OF A COMMIT THAT LOCKED A CLAIM WAS REFUSED. A legitimate
//     revert removes that lock's ledger record — byte-identical to erasing it —
//     so the comparison reported `integrity-store-removed` (whole-store revert)
//     or `lock-ledger-deleted` (single-record revert). Worse, git does not run
//     pre-commit for revert, so the refusal landed locally at rc=0 and only CI
//     objected, after the fact, with a message telling the author to restore the
//     thing they had just deliberately removed.
//   - A PROJECT THAT IS NEW IN A COMMIT WAS AUDITED AGAINST AN UNRELATED
//     PROJECT'S LEDGER. In a monorepo, "retire projB and add projC in one
//     commit" made projC's config absent from the parent, so the parent-config
//     search fell through to "the one project.config.yaml that vanished" — projB's
//     — and projC was refused with findings naming projB's ledger and projB's
//     claims.
//
// WHAT THIS COSTS, exactly, measured against the binary that had it: THREE
// detections — not one, and not two. Both earlier statements of this cost were
// under-counts, and they are recorded here because the same mistake is easy to
// make twice. The FIRST said ONE, measured against the scope comparison
// (stagedScope) alone. The SECOND said TWO, adding the erased review but still
// crediting the per-claim half — lock.AuditAgainstParent, which read the
// PARENT's two stores — with only one shape; it independently covered the
// disowned claim as well. Understating the boundary is worse than the boundary:
// a gap nobody has measured is folklore, and folklore is what gets a comparison
// against rewritable history re-added.
//
// All three are ONE MOVE AT THREE TARGETS — erase a claim's EVIDENCE together
// with whatever was left to judge it against, in a single coordinated change, so
// that no surviving file in the tree can name the disagreement. All three are
// CONJUNCTIONS: either sabotage alone is still refused from this one tree, which
// is what makes the surviving rules worth having. And all three are loud in a
// diff, which is where the forge catches them.
//
//  1. SCOPE COLLAPSE. claims_dir repointed AND the lock ledger removed IN THE
//     SAME CHANGE now passes: nothing is left in scope to judge, so every rule
//     runs perfectly over an empty registry. Repoint only and the standing
//     records have no claims left to cover, which is lock-ledger-abandoned;
//     delete the ledger only and the locked claims have no records, which is
//     lock-ledger-absent.
//  2. DISOWNED CLAIM. For ONE claim, in the same change: delete ledger[id] and
//     locked_at[id] (and hashes[id] when non-empty), flip `status: locked` to
//     draft, and rewrite the body. It is CHEAPER than 1 — no claims_dir edit, no
//     store deleted, one claim's worth of diff — and it is invisible because the
//     evidence that says "this claim was locked by the engine" is exactly what
//     was deleted alongside the record. A variant moves the claim file out of
//     scope instead of flipping its status. Either half alone is still refused,
//     and the three near-misses are worth naming separately, because they are
//     three DIFFERENT rules and an earlier condensed spelling of this bullet
//     ("drop the record only ... which is lock-ledger-missing") was read two
//     ways by two readers. Measured against a binary built from this tree:
//     dropping ledger[id] ALONE, with locked_at and the baselines surviving, is
//     lock-ledger-deleted, because the survivors prove this engine locked it;
//     dropping ALL THREE keys but leaving `status: locked` is
//     lock-ledger-missing, a locked claim with no approval at all; flipping the
//     status ALONE, with the record surviving, is lock-ledger-orphan, a standing
//     record covering a draft. Only the FULL conjunction — all three keys gone
//     AND the status flipped AND the body rewritten — is silent. See
//     lock.RuleLockLedgerDeleted.
//  3. ERASED REVIEW. A DRAFT claim's `comments:` block deleted AND that claim's
//     key dropped from the digest store IN THE SAME CHANGE now passes, after
//     which the claim locks over a review nobody had. Erase the block only and
//     the recorded digest still describes threads the claim no longer has, which
//     is comment-ledger-drift; drop the key only and the threads have no entry
//     beside them, which is comment-digest-unrecorded (whose predicate is the
//     threads themselves — erasing them is exactly what takes the claim out of
//     that rule's evidence set; see lock.RuleCommentDigestUnrecorded). This one
//     is sharper than its size suggests: an OPEN thread on a draft is what BLOCKS
//     `claim lock`, so the erasure buys the lock. It is confined to DRAFT claims
//     because check.RuleCommentDigestMissing keys on a STANDING lock-ledger
//     record with no digest entry: a locked claim has one, so the same edit is
//     caught there as comment-digest-missing; a draft claim has none, so the
//     rule is never asked. NOT lock-content-drift — `comments` is excluded from
//     the locked-claim hash by lock.lockedClaimHashExcluded (dossierx serve
//     writes comments and has no write authority over the lock store), so no
//     comments edit on any claim can produce that rule.
//
// All three are pinned as PASSING tests in staged_no_parent_test.go, beside the
// "either half alone is still refused" assertions that keep them honest, and
// internal/lock/audit_boundary_test.go pins 2 and 3 again from the rules' side.
// internal/lock/audit.go states the same boundary in the same three terms; the
// two passages must be corrected together or the next reader will meet only one
// of them and under-count again.
//
// AND THERE IS NO CHEAP SINGLE-TREE REPLACEMENT for any of them, which is worth
// saying plainly so nobody spends a day looking for one: "zero claims in scope
// and no ledger" is also exactly what a brand-new project looks like, "a draft
// with no record and no baselines" is exactly what every honest draft looks
// like, and "a draft with no threads and no digest entry" is exactly what most
// drafts look like — so a rule that refused any of the three would refuse every
// project's first commit, every draft, or every uncommented draft. The honest
// closure for all three is evidence that is outside the committer as well as
// outside the commit — a signature, or a server-side record — not another read
// of the same person's git history.
//
// WHAT STAYED, and it is most of it: --staged still evaluates the GIT INDEX
// rather than the worktree, still writes nothing, and still runs every
// single-tree ledger rule. Only the second tree is gone.
package check

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/BarterX-Tech/dossierx/internal/buildorder"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/digest"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// ErrNoIndex means there is no index CONTENT to evaluate: git is not installed,
// the project is not inside a work tree, or claims_dir sits outside the WORK
// TREE entirely — not merely outside the config file's own directory — AND the
// index holds no store the ledger gate could judge on its own either.
//
// That last conjunct is not decoration. An out-of-work-tree claims_dir with a
// standing lock ledger beside it used to come here on the strength of the
// pathspec alone, which reported "nothing to evaluate", exit 0, over a tree
// `check --validate` refused. See stagedWithUnreachableClaims.
//
// That distinction is the whole of it. This used to fire for
// `claims_dir: ../claims` — an ordinary monorepo layout, with the config in
// docs/ and the claims beside it — because every pathspec was computed relative
// to the config's directory and "../claims" looked, to that arithmetic, like a
// path git could not resolve. Git resolves it perfectly well; only the
// arithmetic could not. The consequence was the worst one available: the gate
// the pre-commit hook and CI both run reported "nothing to evaluate" and exited
// 0 over a tampered locked claim, while `check --validate` on the identical
// tree reported lock-content-drift. Specs are now anchored at
// `git rev-parse --show-toplevel`, so this is reached only when a path really
// is outside the repository, where no commit could carry it anyway.
//
// It is a distinct sentinel because it is NOT a failure. "dossierx check
// --staged" answers it with a warning and exit 0, on purpose: --staged is what
// a pre-commit hook runs, and a hook is only ever reached FROM git, so this
// condition means someone ran the command by hand somewhere it cannot apply.
// Failing there would break "run check --staged in CI" for every project that
// checks out a tarball, and would teach hook authors to swallow exit codes —
// which is the one habit that makes every other gate in this release worthless.
var ErrNoIndex = errors.New("no git index to evaluate")

// ErrUntrackedConfig means project.config.yaml is not in the index while the
// index DOES hold dossierx content — claims, or a lock ledger — that the gate
// would have to judge against some configuration.
//
// It is emphatically NOT ErrNoIndex. ErrNoIndex is the exit-0 escape hatch for
// "there is nothing here to evaluate"; this is the opposite condition, "there
// is something here to evaluate and the only configuration available cannot be
// trusted to say what it is". See stagedConfig for why an untracked config is
// attacker-writable in a way a staged one is not.
var ErrUntrackedConfig = errors.New("project.config.yaml is not tracked, but the index holds claims to judge")

// StagedProject is the project exactly as the git index holds it: the config,
// the full claim registry, the lock ledger, the comment digest store and every
// locked build order, all read from that one index.
type StagedProject struct {
	// Config is project.config.yaml AS THE INDEX HOLDS IT, and it is what the
	// rest of this value was assembled against.
	//
	// It is read from the index for exactly the reason the claims are. The
	// config names claims_dir, the module list, the doctrine facet and the hub
	// gating switch — every input that decides WHICH files the gate looks at
	// and what it demands of them. Read from the worktree, one unstaged line is
	// a complete bypass: stage a tampered locked claim, edit `claims_dir:
	// claims` to `claims_dir: decoy` in the working tree only, and the gate
	// dutifully audits an empty directory, reports nothing, and lets the commit
	// through. The commit itself still carries the real claims_dir, because that
	// edit was never staged. Reading the config from the index closes it: the
	// gate is evaluated against the configuration the commit will actually have.
	Config *config.Config

	// ConfigFromIndex reports whether Config came out of the index (true) or is
	// the caller's worktree config reused because project.config.yaml is not
	// tracked at all.
	//
	// False is now a much narrower state than it reads. The fallback survives
	// only for the case it was written for — an index that holds no claim, no
	// lock ledger and no comment digest store, i.e. a project whose first commit
	// has not been staged yet — because an untracked config is a WORKTREE file,
	// and a worktree file is editable without staging anything. Every other
	// untracked-config run is refused with ErrUntrackedConfig; see stagedConfig.
	ConfigFromIndex bool

	// Claims is the complete registry, sorted by SourcePath exactly as
	// loader.LoadClaims sorts it, with SourcePath pointing at the WORKING-TREE
	// location of each claim even for content that came out of the index. That
	// is deliberate: a finding a human has to act on must name a path they can
	// open, and "the index's copy of claims/foo.yaml" is not a path.
	Claims []model.Claim

	// FromIndex lists, sorted, the paths whose INDEX content differs from the
	// worktree file — the claims where "what you are committing" and "what you
	// are looking at" are not the same bytes. It is reported so a hook's output
	// can say what it actually judged; an empty list on a dirty-looking tree is
	// a real answer (it means every tracked claim's worktree copy already
	// matches the index).
	//
	// The paths are REPOSITORY-relative, slash form — git's own form, the one
	// `git status` prints and the one a reader can paste back into a git
	// command. They used to be relative to the config file's directory, which
	// was indistinguishable in the layout everything else assumes (config at the
	// repository root) and became "../claims/foo.yaml" in the layout that
	// anchoring at the top level now supports.
	//
	// It is REPORTING ONLY. Every claim's content is read from the index
	// regardless, and this list is computed by comparing the bytes afterwards —
	// see Staged for why asking git which files differ cannot be allowed to
	// decide where content is read from.
	FromIndex []string

	// ledger is the gate's input state, built from the index. Unexported: a
	// caller's only legitimate use for it is handing this whole value back to
	// StatusStaged, and exporting the stores would invite someone to Save() one
	// whose path points into a deleted temp directory.
	ledger ledgerInputs
}

// Staged assembles the project from the git index. It returns ErrNoIndex
// (wrapped, with a human-readable reason) when there is nothing to evaluate;
// every other error is a genuine failure the caller should surface.
//
// It writes nothing anywhere under cfg.Dir(). The only files it creates at all
// are the two store blobs, materialized into an os.MkdirTemp directory that is
// removed before this function returns — see stagedLedgerInputs.
func Staged(cfg *config.Config) (StagedProject, error) {
	g, err := newGitRunner(cfg.Dir())
	if err != nil {
		return StagedProject{}, err
	}

	// The CONFIG comes out of the index first, and everything below is resolved
	// against it — see StagedProject.Config for why a worktree config makes the
	// whole gate bypassable with one unstaged line.
	var sp StagedProject
	sp.Config, sp.ConfigFromIndex, err = stagedConfig(g, cfg)
	if err != nil {
		return StagedProject{}, err
	}
	cfg = sp.Config

	// claims_dir as a git pathspec, anchored at the REPOSITORY TOP LEVEL rather
	// than at the config file's own directory — see gitRunner.spec. It fails
	// only when the claims are outside the work tree altogether, which no commit
	// could carry.
	//
	// AND THAT FAILURE IS NOT, BY ITSELF, THE ESCAPE HATCH. It used to be: this
	// returned ErrNoIndex here, ahead of stagedLedgerInputs, so a claims_dir
	// repointed OUT of the work tree while the lock ledger stayed put never
	// reached the ledger gate at all — `check --staged` reported skipped:true,
	// ok:true, exit 0, over a tree that `check --validate` refused as
	// lock-ledger-abandoned. Two modes, one tree, opposite answers, with the
	// laxer one being the mode the pre-commit hook runs; whichever of the two is
	// laxer is the one an edit travels through.
	//
	// The single-tree ledger rules are exactly the ones that do not need the
	// claims: lock-ledger-abandoned walks the LEDGER's own records and asks
	// whether the claims they approve are still in scope, and an unreachable
	// claims_dir is the most complete way there is of putting them out of scope.
	// The evidence is right there in the index — the ledger names claims no
	// commit can carry — so answering "there is nothing here to evaluate" is
	// false on its own terms. ErrNoIndex means what its doc comment says: no
	// index CONTENT to judge. That is now decided below, after the stores have
	// been read, rather than assumed from the claims pathspec alone.
	claimsSpec, err := g.spec(cfg.ClaimsDir)
	if err != nil {
		return stagedWithUnreachableClaims(g, cfg, sp)
	}

	// EVERY claim's content comes from the index. Unconditionally, with no
	// worktree shortcut for the ones git says are clean.
	//
	// There used to be one: "git diff" was asked which paths differed, those
	// were fetched from the index, and the rest were read off disk as a cheaper
	// equivalent. It is not an equivalent. "git diff" consults git's stat cache
	// and honours the per-path skip bits, so a single
	//
	//	git update-index --assume-unchanged claims/whatever.yaml
	//
	// makes git report a modified file as clean — and the gate then read the
	// clean WORKTREE copy while the tampered blob sat in the index waiting to be
	// committed. The refusal disappeared and the commit landed. The same is true
	// of --skip-worktree, of a racily-clean stat entry, and of anything else
	// that ever teaches git's cache a lie. A gate whose evidence source is
	// chosen by a mutable, attacker-writable bit is not a gate.
	//
	// The cost is one "git cat-file --batch" for the whole registry, which is a
	// single subprocess either way.
	//
	// indexBlobs is ALSO the authority on WHICH claims exist — the file list and
	// the content come from one query rather than two that could disagree. That
	// is what makes three otherwise-invisible cases come out right: a claim
	// staged for DELETION is gone from the index and so must not be linted; a
	// claim that is merely UNTRACKED is not part of the commit and must not be
	// linted either; and a claim staged for ADDITION is in the index before it is
	// in any commit and must be.
	blobs, err := g.indexBlobs(claimsSpec)
	if err != nil {
		return StagedProject{}, err
	}

	rels := make([]string, 0, len(blobs))
	for rel := range blobs {
		rels = append(rels, rel)
	}
	sort.Strings(rels)

	for _, rel := range rels {
		if !isClaimFile(rel) {
			continue
		}
		abs := worktreePath(cfg.ClaimsDir, claimsSpec, rel)
		raw := blobs[rel]

		// FromIndex is derived by comparing bytes we already hold, NOT by asking
		// git what differs. That keeps the report honest under exactly the
		// conditions that broke the old shortcut: an assume-unchanged file whose
		// worktree copy differs is still listed here, because this comparison
		// consults no cache. A worktree file that cannot be read (staged
		// deletion, permissions) counts as differing — it certainly is not
		// identical.
		//
		// LINE ENDINGS ARE NORMALISED ON BOTH SIDES FIRST, and that is what makes
		// the field mean the same thing on all three CI platforms. Under
		// core.autocrlf=true — the Windows default, and the configuration the
		// windows-latest leg runs in — git stores LF in the index and checks out
		// CRLF, so a byte comparison reported EVERY claim as differing on a
		// perfectly clean tree: "2 claim(s) from the git index (2 differ from the
		// working tree)" with `git status --porcelain` empty. The field is
		// documented as "the files where index and worktree disagree", and a field
		// that is unconditionally saturated on one platform carries no signal at
		// all. Normalising here preserves the anti-stat-cache property the
		// paragraph above defends — the comparison still consults no git cache,
		// only bytes — while dropping exactly the difference git itself introduced
		// on the way to disk. The verdict was never affected (YAML parsing
		// normalises line breaks, so hashes and lint agree either way); the report
		// was.
		if onDisk, readErr := os.ReadFile(abs); readErr != nil || !bytes.Equal(normalizeLineEndings(onDisk), normalizeLineEndings(raw)) {
			sp.FromIndex = append(sp.FromIndex, rel)
		}

		c, err := decodeClaim(abs, raw)
		if err != nil {
			return StagedProject{}, err
		}
		sp.Claims = append(sp.Claims, c)
	}

	// loader.LoadClaims sorts by SourcePath, and every downstream consumer —
	// lint's finding order, the catalog, the reporting — inherits that order.
	// git ls-files already sorts, but it sorts BYTES of slash-separated paths
	// while loader sorts the platform-separated absolute path, so sort here
	// rather than assume the two agree.
	sort.Slice(sp.Claims, func(i, j int) bool { return sp.Claims[i].SourcePath < sp.Claims[j].SourcePath })
	sort.Strings(sp.FromIndex)

	// The two stores and every build-order artifact, from the same index. That
	// is the LAST thing this function does: everything the gate is evaluated
	// against now comes from one tree, and nothing here looks at a second one.
	// See this file's REMOVED section for the comparison that used to sit at
	// exactly this point and why re-adding it here would be a mistake.
	sp.ledger, err = stagedLedgerInputs(g, cfg)
	if err != nil {
		return StagedProject{}, err
	}
	return sp, nil
}

// stagedWithUnreachableClaims finishes a run whose claims_dir is outside the
// work tree: the registry is empty, because no commit can carry a file git
// cannot name, and the verdict is whatever the ledger gate makes of the INDEX's
// stores standing alone.
//
// It is the difference between "this commit contains no claims" and "there is
// nothing here to evaluate", and only the second is ErrNoIndex. A standing lock
// ledger whose records name claims that are now unreachable is a refusal
// (lock-ledger-abandoned) that needs one tree, no history and no claims at all —
// see the pathspec comment in Staged for the false clean that came of assuming
// otherwise.
//
// The escape hatch survives for the case it was written for and nothing else: a
// checkout whose claims genuinely live outside the repository AND that carries
// no lock ledger, no comment digest store and no build-order artifact either. On
// that tree the gate really is being asked about content the commit does not
// have, `check --validate` says nothing either, and refusing would break
// "run check --staged in CI" for a layout the rest of the product supports.
//
// WHAT IT COSTS, stated plainly because it is a real cost: a project whose
// claims live outside the repository and whose lock ledger lives inside it is
// now refused by --staged, per unreachable approval. That project's approvals
// really do cover files no commit carries — its ledger is a record about
// something this repository does not contain — and the same tree is refused by
// `check --validate` the moment those claims are not where claims_dir points.
// The alternative is the false clean above, in the mode that runs in the hook.
func stagedWithUnreachableClaims(g *gitRunner, cfg *config.Config, sp StagedProject) (StagedProject, error) {
	in, err := stagedLedgerInputs(g, cfg)
	if err != nil {
		return StagedProject{}, err
	}
	if !holdsGateEvidence(in) {
		return StagedProject{}, fmt.Errorf(
			"%w: claims_dir %s is outside the git work tree at %s, so no commit can carry it — and the index holds no lock ledger, no comment digest store and no build-order artifact either, so there is nothing in it to judge",
			ErrNoIndex, cfg.ClaimsDir, g.dir)
	}
	sp.ledger = in
	return sp, nil
}

// holdsGateEvidence reports whether the index carries anything the ledger gate
// can reach a verdict from WITHOUT any claims — a lock ledger, a comment digest
// store, or a build-order artifact, present or merely unreadable.
//
// Unreadable counts, and deliberately: a store that is there and will not decode
// is reported (lock-ledger-unreadable, build-order-unreadable) precisely so that
// corrupting the gate's evidence cannot be quieter than deleting it. Treating it
// as "no evidence" here would restore that inversion through the one door left.
func holdsGateEvidence(in ledgerInputs) bool {
	if in.storeErr != nil || in.digestErr != nil {
		return true
	}
	if in.store != nil && in.store.FileExists() {
		return true
	}
	if in.digests != nil && in.digests.FileExists() {
		return true
	}
	for _, o := range in.buildOrders {
		if o.Present || o.Unreadable {
			return true
		}
	}
	return false
}

// configSource is the config file's OWN path — not Dir()+FileName, because
// --config takes an arbitrary path and a project that named its config something
// else must be read from the file it actually uses. Looking up a name the
// project does not use would find nothing in the index and fall back to the
// worktree, which is the bypass stagedConfig exists to close.
func configSource(cfg *config.Config) string {
	if p := cfg.Path(); p != "" {
		return p
	}
	return filepath.Join(cfg.Dir(), config.FileName)
}

// stagedConfig loads project.config.yaml as the INDEX holds it.
//
// It returns the caller's own config unchanged, with fromIndex=false, in exactly
// one case: the config file is not tracked AND the index holds nothing this gate
// would have to judge — no claim file, no lock ledger, no comment digest store.
// That is the first commit of a project, where there is nothing in the index to
// read and the worktree copy is the only configuration that exists, and where
// nothing is being bypassed because there is nothing there to bypass.
//
// ANY OTHER UNTRACKED CONFIG IS A REFUSAL (ErrUntrackedConfig), and the reason
// is that "not tracked" was doing the work "not staged" was assumed to do. An
// untracked project.config.yaml is a WORKING-TREE file: it can be rewritten
// without staging anything, which is precisely the property reading the config
// from the index exists to deny. Point claims_dir at an empty decoy directory in
// an untracked config, stage a tampered locked claim, and the gate audited the
// decoy, found nothing, and passed the commit — the same bypass
// TestStaged_ConfigComesFromTheIndex closed, reached through the fallback
// instead of through the file. The refusal is deliberately NOT ErrNoIndex,
// because ErrNoIndex exits 0.
//
// The index's CONTENT is decoded but the WORKTREE directory stays the anchor:
// claims_dir, the stores and the build-order artifacts are still resolved
// against cfg.Dir(), because that is where the files actually live. What comes
// from the index is the part that decides what the gate looks at — claims_dir,
// the module list, the doctrine facet, hub gating.
//
// A config that is in the index but does not LOAD is a hard error, not a
// fallback to the worktree. Falling back would restore the bypass in a slightly
// noisier form: stage a config that fails validation and the gate would go
// right on reading the worktree's.
func stagedConfig(g *gitRunner, cfg *config.Config) (*config.Config, bool, error) {
	// The config's OWN path, not Dir()+FileName: --config takes an arbitrary
	// path, and looking up a name the project does not use would find nothing
	// in the index and fall back to the worktree — reopening the bypass for
	// exactly the projects that named their config something else.
	src := configSource(cfg)

	// A config OUTSIDE the work tree cannot be in the index either, so it takes
	// the same path as an untracked one rather than a silent worktree fallback
	// of its own.
	var tracked []string
	spec, err := g.spec(src)
	if err == nil {
		if tracked, err = g.lsFiles(spec); err != nil {
			return nil, false, err
		}
	}
	if len(tracked) == 0 {
		held, err := indexHoldsJudgeableContent(g)
		if err != nil {
			return nil, false, err
		}
		if held != "" {
			return nil, false, fmt.Errorf(
				"%w: the commit carries dossierx content (%s) and no %s, so there is no configuration in it "+
					"to say which files are claims — and the working-tree copy can be edited without staging "+
					"anything, which is exactly the bypass reading the config from the index prevents. "+
					"Stage the config (git add %s) and commit it alongside them",
				ErrUntrackedConfig, held, config.FileName, src)
		}
		return cfg, false, nil
	}

	raw, err := g.showIndexBlob(tracked[0])
	if err != nil {
		return nil, false, err
	}
	staged, err := config.DecodeConfig(raw, cfg.Dir(), src+" (as staged)")
	if err != nil {
		return nil, false, fmt.Errorf("check --staged: the staged %s does not load: %w", config.FileName, err)
	}
	return staged, true, nil
}

// indexHoldsJudgeableContent reports the first thing in the index that this
// gate would have had to judge — a claim file, the lock ledger, or the comment
// digest store — as a repository-relative path, or "" when the index holds none
// of them.
//
// It is the test that separates stagedConfig's one legitimate fallback (a
// project whose first commit has not been staged yet) from the bypass that
// fallback had become. It asks the WHOLE index rather than the worktree
// config's claims_dir, and that is the point: the question is being asked
// precisely because the configuration naming claims_dir cannot be trusted, so
// consulting it to answer would be circular — an untracked config pointing at
// an empty decoy directory would report "nothing here" and wave the commit
// through, which is the defect.
//
// "A claim file" means a .yaml/.yml blob that DECODES as a claim, not merely
// one that is named like one. A repository's ordinary yaml — workflows, linter
// configs, chart values — fails the engine's strict decode, so the common case
// of a repository full of unrelated yaml does not turn into a refusal. The
// direction of any error here is safe by construction: a false positive refuses
// a commit and says exactly which file and exactly what to stage; a false
// negative would be a silent pass, and nothing in this function can produce
// one, because "is this a claim?" is answered by the same decoder the registry
// is built with.
//
// The cost — one ls-files and one cat-file over the repository's yaml — is paid
// ONLY on the untracked-config path, which the hook reaches only when no
// project.config.yaml is tracked anywhere in the repository.
func indexHoldsJudgeableContent(g *gitRunner) (string, error) {
	entries, err := g.indexEntries()
	if err != nil {
		return "", err
	}

	var candidates []indexEntry
	for _, e := range entries {
		switch path.Base(e.path) {
		case lockStoreFileName, digest.StoreFileName:
			return e.path, nil
		}
		if isClaimFile(e.path) {
			candidates = append(candidates, e)
		}
	}
	if len(candidates) == 0 {
		return "", nil
	}

	blobs, err := g.catFile(candidates)
	if err != nil {
		return "", err
	}
	// Sorted, so the path named in the refusal is the same one on every run and
	// on every platform.
	paths := make([]string, 0, len(blobs))
	for p := range blobs {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		if c, err := decodeClaim(p, blobs[p]); err == nil && strings.TrimSpace(c.ID) != "" {
			return p, nil
		}
	}
	return "", nil
}

// worktreePath maps a repository-relative path git reported back to the
// WORKING-TREE path a human can open, given the pathspec the listing was scoped
// to and the absolute directory that pathspec names.
//
// It exists because the two namespaces stopped being the same the moment specs
// were anchored at the repository top level: git answers "claims/foo.yaml" for a
// project whose claims_dir is "../claims" from a config in docs/, and joining
// that onto the config's directory would name docs/claims/foo.yaml — a file that
// does not exist. A finding a human has to act on must name a path they can
// open.
func worktreePath(base, spec, rel string) string {
	if spec == "." || spec == "" {
		return filepath.Join(base, filepath.FromSlash(rel))
	}
	return filepath.Join(base, filepath.FromSlash(strings.TrimPrefix(rel, spec+"/")))
}

// stagedLedgerInputs loads the lock ledger and the comment digest store as the
// INDEX holds them.
//
// Both stores are read by path (lock.LoadStore / digest.LoadStore), and both
// distinguish "the file was there" from "the file was not" — a distinction the
// gate depends on, because lock.RuleLockLedgerAbsent fires on exactly that.
// Rather than widen either package with a decode-from-bytes entry point purely
// for this caller, the index blobs are materialized into a temp directory and
// loaded from there, so the "was the file present?" answer is the honest one
// for the index and both stores keep a single load path.
//
// The temp directory is removed before returning. That leaves each store
// holding a path that no longer exists, which is a FEATURE here and not an
// oversight: a --staged run must never write, and a store whose Save() could
// only fail is a structural guarantee rather than a promise. Nothing on this
// path calls Save.
func stagedLedgerInputs(g *gitRunner, cfg *config.Config) (ledgerInputs, error) {
	dir, err := os.MkdirTemp("", "dossierx-staged-")
	if err != nil {
		return ledgerInputs{}, fmt.Errorf("check --staged: temp dir: %w", err)
	}
	// Best-effort cleanup: the run's verdict must not depend on whether a temp
	// directory could be removed.
	defer os.RemoveAll(dir) //nolint:errcheck // best-effort cleanup of our own temp dir

	var in ledgerInputs

	lockPath, err := materializeIndexFile(g, dir, storePath(cfg))
	if err != nil {
		return ledgerInputs{}, err
	}
	if store, err := lock.LoadStore(lockPath); err != nil {
		in.storeErr = err
	} else {
		in.store = store
	}

	digestPath, err := materializeIndexFile(g, dir, digest.StorePath(cfg))
	if err != nil {
		return ledgerInputs{}, err
	}
	if digests, err := digest.LoadStore(digestPath); err != nil {
		in.digestErr = err
	} else {
		in.digests = digests
	}

	// The build-order artifacts come from the index for the same reason the
	// ledger does. A locked build order read from the WORKTREE and compared
	// against an INDEX ledger record would refuse commits over edits that are
	// not being committed, and — the direction that matters — would pass a
	// commit that stages a tampered artifact while the worktree copy still
	// matches its record.
	in.buildOrders = collectBuildOrderStates(cfg, func(module string) (*buildorder.Artifact, error) {
		path, err := materializeIndexFile(g, dir, buildorder.ArtifactPath(cfg, module))
		if err != nil {
			return nil, err
		}
		return buildorder.LoadArtifact(path)
	})

	return in, nil
}

// materializeIndexFile writes the index's copy of src (an absolute path) into
// dir and returns the written path. When src is not tracked in the index it
// writes nothing and returns the path anyway — the store loaders read a missing
// file as an absent store, which is precisely what "not in the index" means and
// is the state lock.RuleLockLedgerAbsent exists to catch.
func materializeIndexFile(g *gitRunner, dir, src string) (string, error) {
	out := filepath.Join(dir, filepath.Base(src))

	spec, err := g.spec(src)
	if err != nil {
		// Outside the work tree entirely: treat as untracked, which is the
		// conservative reading (an absent ledger is a finding, never a pass).
		return out, nil
	}
	tracked, err := g.lsFiles(spec)
	if err != nil {
		return "", err
	}
	if len(tracked) == 0 {
		return out, nil
	}

	raw, err := g.showIndexBlob(tracked[0])
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(out, raw, 0o600); err != nil {
		return "", fmt.Errorf("check --staged: stage %s for reading: %w", src, err)
	}
	return out, nil
}

// decodeClaim parses one claim out of raw, attributing it to sourcePath.
//
// It mirrors loader.LoadClaims's per-file decode exactly — strict unknown-field
// rejection, and the one-claim-per-file rule enforced by requiring the decoder
// to be at EOF afterwards. It is duplicated rather than shared because
// internal/loader reads BY PATH and index content has no path to read: the blob
// exists only in git's object store. TestStagedDecodeMatchesLoader pins the two
// against each other on the inputs that distinguish them, so a rule added to
// one and not the other fails the build rather than quietly letting --staged
// accept a file plain check rejects.
func decodeClaim(sourcePath string, raw []byte) (model.Claim, error) {
	var c model.Claim
	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return model.Claim{}, fmt.Errorf("loader: parse %s: %w", sourcePath, err)
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return model.Claim{}, fmt.Errorf("loader: %s contains more than one YAML document; exactly one claim per file is required", sourcePath)
	}
	c.SourcePath = sourcePath
	return c, nil
}

// isClaimFile applies loader.LoadClaims's file filter: *.yaml and *.yml, case
// insensitive, everything else ignored.
func isClaimFile(rel string) bool {
	ext := strings.ToLower(path.Ext(rel))
	return ext == ".yaml" || ext == ".yml"
}

// normalizeLineEndings collapses CRLF to LF so the index copy of a claim and the
// worktree copy of the same claim can be compared for CONTENT rather than for
// the line-ending convention git applied on checkout. See the FromIndex
// comparison in Staged for why the difference is git's own doing and why
// reporting it as a disagreement made the field useless on Windows.
//
// It is deliberately a plain byte substitution rather than anything smarter: a
// lone CR (a classic-Mac line ending, or a CR inside a quoted YAML scalar) is
// left exactly as it is, so this can only ever erase the one transformation
// core.autocrlf performs, never a real difference between the two copies.
func normalizeLineEndings(b []byte) []byte {
	if !bytes.Contains(b, []byte("\r\n")) {
		return b
	}
	return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
}

// ---------------------------------------------------------------------
// the git binary
// ---------------------------------------------------------------------

// errOutsideWorkTree is gitRunner.spec's "this path is not in the repository"
// answer. Callers decide what that means for them: for claims_dir it is
// ErrNoIndex (nothing a commit could carry), for a store it is "absent from the
// index", which is a finding rather than a pass.
var errOutsideWorkTree = errors.New("path is outside the git work tree")

// gitRunner runs git with a fixed working directory. Every command it issues is
// read-only; nothing here can modify the index, the worktree, or the object
// store.
type gitRunner struct {
	bin string

	// dir is git's working directory, and it is the repository's TOP LEVEL —
	// not the project's own directory. Everything git reports is therefore
	// repository-relative, which is the same namespace the index itself uses,
	// so a pathspec can name any tracked file in the repository regardless of
	// where the config happens to sit.
	dir string

	// base is the directory the callers' absolute paths are expressed relative
	// to (the config's own directory), and prefix is that directory as a
	// slash-separated path relative to dir — "" when the project sits at the top
	// level. Together they are all spec() needs to translate a project path into
	// a repository path.
	//
	// prefix comes from git ("rev-parse --show-prefix") rather than from
	// comparing strings, and that is deliberate: on macOS a temp directory is
	// reached through /var while git resolves it to /private/var, so any
	// arithmetic over the two absolute paths would disagree with git about a
	// path both of them can open.
	base   string
	prefix string
}

// newGitRunner locates git, confirms dir is inside a work tree, and re-anchors
// itself at that work tree's TOP LEVEL. A missing git and a directory outside
// any work tree both come back as ErrNoIndex — the caller's job is to warn and
// exit 0, not to explain git to somebody who does not have it.
//
// THE RE-ANCHORING IS THE FIX FOR A HOLE, not tidiness. Pathspecs used to be
// computed relative to the config's own directory, so an ordinary monorepo
// layout — docs/project.config.yaml with `claims_dir: ../claims` — produced the
// spec "../claims", which this package read as "outside the repository" and
// answered with ErrNoIndex: the exit-0 escape hatch. The claims were not outside
// anything; git resolves them from the top level without complaint. The gate the
// pre-commit hook and CI both run therefore evaluated NOTHING, silently, on a
// layout that every other command in the product handles, and an out-of-band
// edit to a locked claim committed clean while `check --validate` on the same
// tree reported lock-content-drift.
func newGitRunner(dir string) (*gitRunner, error) {
	bin, err := exec.LookPath("git")
	if err != nil {
		return nil, fmt.Errorf("%w: git is not installed or not on PATH", ErrNoIndex)
	}
	g := &gitRunner{bin: bin, dir: dir, base: dir}
	out, err := g.run("rev-parse", "--is-inside-work-tree")
	if err != nil || strings.TrimSpace(string(out)) != "true" {
		return nil, fmt.Errorf("%w: %s is not inside a git work tree", ErrNoIndex, dir)
	}

	// One invocation, two answers, in the order they are asked for: the top
	// level, then this directory's path within it. Asking git for both keeps
	// the two consistent with each other and with the index, which is what
	// makes them safe to do path arithmetic with.
	out, err = g.run("rev-parse", "--show-toplevel", "--show-prefix")
	if err != nil {
		return nil, fmt.Errorf("%w: git would not name the work tree containing %s: %w", ErrNoIndex, dir, err)
	}
	lines := strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n")
	if len(lines) < 2 || strings.TrimSpace(lines[0]) == "" {
		return nil, fmt.Errorf("%w: git would not name the top level of the work tree containing %s", ErrNoIndex, dir)
	}
	g.dir = lines[0]
	// --show-prefix is slash-terminated and empty at the top level.
	g.prefix = strings.Trim(lines[1], "/")
	return g, nil
}

// spec expresses target — an absolute path, or one relative to the project
// directory — as a git pathspec relative to the REPOSITORY TOP LEVEL, which is
// both what the runner's commands are issued from and the namespace git reports
// paths in.
//
// It fails with errOutsideWorkTree only when the result climbs above the top
// level, i.e. when the path really is outside the repository. "Outside the
// config file's directory" is not that, and treating it as if it were is the
// defect newGitRunner's comment describes.
func (g *gitRunner) spec(target string) (string, error) {
	rel, err := filepath.Rel(g.base, target)
	if err != nil {
		return "", err
	}
	// path.Join cleans, so a "../" in rel is consumed by prefix when there is
	// prefix left to consume and survives when there is not.
	p := path.Join(g.prefix, filepath.ToSlash(rel))
	if p == ".." || strings.HasPrefix(p, "../") {
		return "", fmt.Errorf("%w: %s is not under %s", errOutsideWorkTree, target, g.dir)
	}
	if p == "" {
		p = "."
	}
	return p, nil
}

// run executes git with the runner's directory as cwd and returns stdout.
//
// Two -c overrides are not optional. core.quotepath=false and the -z flags the
// callers pass keep non-ASCII paths raw instead of C-quoted, and
// diff.relative=false pins whether "git diff" reports paths relative to cwd or
// to the repository root — a user config setting that would otherwise silently
// change which paths this code matches against ls-files' output. A gate whose
// answer depends on the auditee's git config is not a gate.
func (g *gitRunner) run(args ...string) ([]byte, error) {
	full := append([]string{"-c", "core.quotepath=false", "-c", "diff.relative=false"}, args...)
	cmd := exec.Command(g.bin, full...) //nolint:gosec // fixed binary, fixed argv; no shell involved
	cmd.Dir = g.dir
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("check --staged: git %s: %s", strings.Join(args, " "), msg)
	}
	return out, nil
}

// runWithStdin is run() with input fed to git's stdin. Only cat-file --batch
// needs it, and it needs it because the alternative — one "git show" per object
// — is what made "always read from the index" look expensive enough to shortcut
// in the first place.
func (g *gitRunner) runWithStdin(stdin string, args ...string) ([]byte, error) {
	full := append([]string{"-c", "core.quotepath=false", "-c", "diff.relative=false"}, args...)
	cmd := exec.Command(g.bin, full...) //nolint:gosec // fixed binary, fixed argv; no shell involved
	cmd.Dir = g.dir
	cmd.Stdin = strings.NewReader(stdin)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("check --staged: git %s: %s", strings.Join(args, " "), msg)
	}
	return out, nil
}

// lsFiles lists the index entries matching spec, as paths relative to the
// runner's directory (git's default output form when not asked for
// --full-name), slash-separated.
func (g *gitRunner) lsFiles(spec string) ([]string, error) {
	out, err := g.run("ls-files", "-z", "--", spec)
	if err != nil {
		return nil, err
	}
	return splitZ(out), nil
}

// indexEntry is one stage-0 index entry: a repository-relative path and the OID
// of the blob the commit will carry for it.
type indexEntry struct {
	oid  string
	path string
}

// indexEntries lists the index's stage-0 entries under specs — or the whole
// index when no spec is given, which is what indexHoldsJudgeableContent needs.
func (g *gitRunner) indexEntries(specs ...string) ([]indexEntry, error) {
	args := []string{"ls-files", "-s", "-z"}
	if len(specs) > 0 {
		args = append(args, "--")
		args = append(args, specs...)
	}
	out, err := g.run(args...)
	if err != nil {
		return nil, err
	}

	// Each -s entry is "<mode> SP <oid> SP <stage>\t<path>".
	var entries []indexEntry
	for _, entry := range splitZ(out) {
		tab := strings.IndexByte(entry, '\t')
		if tab < 0 {
			continue
		}
		fields := strings.Fields(entry[:tab])
		if len(fields) < 3 {
			continue
		}
		// Stage != 0 is an unmerged path: the index holds several conflicting
		// versions and there is no single "what will be committed". Skipping it
		// here means the claim is absent from the registry, which the lint suite
		// reports as a dangling reference rather than silently auditing one side
		// of a conflict.
		if fields[2] != "0" {
			continue
		}
		entries = append(entries, indexEntry{oid: fields[1], path: entry[tab+1:]})
	}
	return entries, nil
}

// indexBlobs returns the index's content for every path under spec, keyed by
// repository-relative path.
//
// It reads the OIDs from "git ls-files -s" and streams them through a single
// "git cat-file --batch", which is what makes "always read from the index"
// affordable: one subprocess for the whole registry rather than one per claim,
// and — the part that matters — no consultation of git's stat cache or its
// per-path skip bits anywhere in the path. An assume-unchanged claim's index
// content comes back exactly like any other's.
func (g *gitRunner) indexBlobs(spec string) (map[string][]byte, error) {
	entries, err := g.indexEntries(spec)
	if err != nil {
		return nil, err
	}
	return g.catFile(entries)
}

// catFile fetches the content of every entry in one "git cat-file --batch".
//
// The response per request is "<oid> <type> <size>\n", then <size> raw bytes,
// then a newline. Sizes are read from that header rather than by scanning for a
// delimiter, because claim bodies contain newlines and NULs are legal in a blob.
func (g *gitRunner) catFile(entries []indexEntry) (map[string][]byte, error) {
	if len(entries) == 0 {
		return map[string][]byte{}, nil
	}
	oids := make([]string, 0, len(entries))
	for _, e := range entries {
		oids = append(oids, e.oid)
	}

	raw, err := g.runWithStdin(strings.Join(oids, "\n")+"\n", "cat-file", "--batch")
	if err != nil {
		return nil, err
	}

	blobs := make(map[string][]byte, len(entries))
	rd := bufio.NewReader(bytes.NewReader(raw))
	for _, e := range entries {
		p := e.path
		header, err := rd.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("check --staged: git cat-file --batch: short read for %s: %w", p, err)
		}
		fields := strings.Fields(strings.TrimSpace(header))
		if len(fields) != 3 {
			return nil, fmt.Errorf("check --staged: git cat-file --batch: unexpected header %q for %s", strings.TrimSpace(header), p)
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, fmt.Errorf("check --staged: git cat-file --batch: unreadable size in %q for %s", strings.TrimSpace(header), p)
		}
		content := make([]byte, size)
		if _, err := io.ReadFull(rd, content); err != nil {
			return nil, fmt.Errorf("check --staged: git cat-file --batch: short body for %s: %w", p, err)
		}
		// The trailing newline cat-file writes after each object.
		if _, err := rd.Discard(1); err != nil {
			return nil, fmt.Errorf("check --staged: git cat-file --batch: malformed record for %s: %w", p, err)
		}
		blobs[p] = content
	}
	return blobs, nil
}

// showIndexBlob returns the index's content for rel (relative to the runner's
// directory). The "./" prefix is what makes ":<path>" resolve relative to the
// current directory instead of the repository root, which is how this stays
// correct for a project whose project.config.yaml lives in a subdirectory of
// the repo.
func (g *gitRunner) showIndexBlob(rel string) ([]byte, error) {
	return g.run("show", ":./"+rel)
}

// splitZ splits git's NUL-delimited output, dropping the empty tail every -z
// listing ends with (and tolerating empty output, which is how git says "no
// matches").
func splitZ(out []byte) []string {
	var paths []string
	for _, p := range strings.Split(string(out), "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}
