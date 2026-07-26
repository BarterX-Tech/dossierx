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
package check

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/digest"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// ErrNoIndex means there is no git index to evaluate: git is not installed, the
// project is not inside a work tree, or claims_dir sits outside the repository
// that contains it.
//
// It is a distinct sentinel because it is NOT a failure. "dossierx check
// --staged" answers it with a warning and exit 0, on purpose: --staged is what
// a pre-commit hook runs, and a hook is only ever reached FROM git, so this
// condition means someone ran the command by hand somewhere it cannot apply.
// Failing there would break "run check --staged in CI" for every project that
// checks out a tarball, and would teach hook authors to swallow exit codes —
// which is the one habit that makes every other gate in this release worthless.
var ErrNoIndex = errors.New("no git index to evaluate")

// StagedProject is the project exactly as the git index holds it: the full
// claim registry with staged blobs substituted in, plus the lock ledger and
// comment digest store read from the same index.
type StagedProject struct {
	// Claims is the complete registry, sorted by SourcePath exactly as
	// loader.LoadClaims sorts it, with SourcePath pointing at the WORKING-TREE
	// location of each claim even for content that came out of the index. That
	// is deliberate: a finding a human has to act on must name a path they can
	// open, and "the index's copy of claims/foo.yaml" is not a path.
	Claims []model.Claim

	// FromIndex lists, sorted, the paths (relative to cfg.Dir(), slash form)
	// whose content was read out of the index rather than off disk. It is
	// reported so a hook's output can say what it actually judged; an empty
	// list on a dirty-looking tree is a real answer, not a bug (it means every
	// tracked claim's worktree copy already matches the index).
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

	// claims_dir as a git pathspec relative to cfg.Dir(). A ".." prefix means
	// the claims live outside the directory git was asked about, which git
	// would reject as "outside repository" — a condition a hook can do nothing
	// about, so it is reported as "no index to evaluate" rather than as a
	// failure.
	claimsSpec, err := relativeSpec(cfg.Dir(), cfg.ClaimsDir)
	if err != nil {
		return StagedProject{}, fmt.Errorf("%w: claims_dir %s is outside %s, so git cannot resolve it", ErrNoIndex, cfg.ClaimsDir, cfg.Dir())
	}

	// The index's own file list is the authority on WHICH claims exist, and
	// using it (rather than the changed-files list alone) is what makes three
	// otherwise-invisible cases come out right: a claim staged for DELETION is
	// gone from the index and so must not be linted; a claim that is merely
	// UNTRACKED is not part of the commit and must not be linted either; and a
	// claim staged for ADDITION is in the index before it is in any commit and
	// must be.
	indexed, err := g.lsFiles(claimsSpec)
	if err != nil {
		return StagedProject{}, err
	}

	// Which of those files' worktree copies differ from the index. This is the
	// precise question — "git diff" with no revision compares worktree against
	// index — and it is asked because it is also the cheap answer: for every
	// path it does NOT list, the worktree file IS the index content, so it can
	// be read straight off disk instead of costing a git subprocess.
	dirty, err := g.dirtyAgainstIndex(claimsSpec)
	if err != nil {
		return StagedProject{}, err
	}

	var sp StagedProject
	for _, rel := range indexed {
		if !isClaimFile(rel) {
			continue
		}
		abs := filepath.Join(cfg.Dir(), filepath.FromSlash(rel))

		var raw []byte
		if dirty[rel] {
			raw, err = g.showIndexBlob(rel)
			if err != nil {
				return StagedProject{}, err
			}
			sp.FromIndex = append(sp.FromIndex, rel)
		} else {
			raw, err = os.ReadFile(abs)
			if err != nil {
				return StagedProject{}, fmt.Errorf("check --staged: read %s: %w", abs, err)
			}
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

	sp.ledger, err = stagedLedgerInputs(g, cfg)
	if err != nil {
		return StagedProject{}, err
	}
	return sp, nil
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

	lockPath, err := materializeIndexFile(g, cfg.Dir(), dir, storePath(cfg))
	if err != nil {
		return ledgerInputs{}, err
	}
	if store, err := lock.LoadStore(lockPath); err != nil {
		in.storeErr = err
	} else {
		in.store = store
	}

	digestPath, err := materializeIndexFile(g, cfg.Dir(), dir, digest.StorePath(cfg))
	if err != nil {
		return ledgerInputs{}, err
	}
	if digests, err := digest.LoadStore(digestPath); err != nil {
		in.digestErr = err
	} else {
		in.digests = digests
	}

	return in, nil
}

// materializeIndexFile writes the index's copy of src (an absolute path under
// base) into dir and returns the written path. When src is not tracked in the
// index it writes nothing and returns the path anyway — the store loaders read
// a missing file as an absent store, which is precisely what "not in the index"
// means and is the state lock.RuleLockLedgerAbsent exists to catch.
func materializeIndexFile(g *gitRunner, base, dir, src string) (string, error) {
	out := filepath.Join(dir, filepath.Base(src))

	spec, err := relativeSpec(base, src)
	if err != nil {
		// Outside the directory git was asked about: treat as untracked, which
		// is the conservative reading (an absent ledger is a finding, never a
		// pass).
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

// relativeSpec expresses target as a slash-separated path relative to base,
// suitable both as a git pathspec (with base as git's working directory) and as
// the tail of a "./"-prefixed index object name. It fails when target is not
// under base.
func relativeSpec(base, target string) (string, error) {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return "", err
	}
	rel = filepath.ToSlash(rel)
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return "", fmt.Errorf("%s is not under %s", target, base)
	}
	return rel, nil
}

// ---------------------------------------------------------------------
// the git binary
// ---------------------------------------------------------------------

// gitRunner runs git with a fixed working directory. Every command it issues is
// read-only; nothing here can modify the index, the worktree, or the object
// store.
type gitRunner struct {
	bin string
	dir string
}

// newGitRunner locates git and confirms dir is inside a work tree. Both
// failures come back as ErrNoIndex — the caller's job is to warn and exit 0,
// not to explain git to somebody who does not have it.
func newGitRunner(dir string) (*gitRunner, error) {
	bin, err := exec.LookPath("git")
	if err != nil {
		return nil, fmt.Errorf("%w: git is not installed or not on PATH", ErrNoIndex)
	}
	g := &gitRunner{bin: bin, dir: dir}
	out, err := g.run("rev-parse", "--is-inside-work-tree")
	if err != nil || strings.TrimSpace(string(out)) != "true" {
		return nil, fmt.Errorf("%w: %s is not inside a git work tree", ErrNoIndex, dir)
	}
	return g, nil
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

// dirtyAgainstIndex returns the set of paths under spec whose WORKTREE content
// differs from the index — "git diff" with no revision. --relative makes the
// output relative to the runner's directory, matching lsFiles.
//
// No --diff-filter is applied on purpose. Any path git lists here differs from
// the index for some reason (content, mode, type, deletion), and "differs" is
// the entire question being asked: every such path must be read from the index
// rather than from disk. Filtering would be a way to accidentally read a
// tampered worktree file.
func (g *gitRunner) dirtyAgainstIndex(spec string) (map[string]bool, error) {
	out, err := g.run("diff", "--name-only", "-z", "--relative", "--", spec)
	if err != nil {
		return nil, err
	}
	dirty := map[string]bool{}
	for _, p := range splitZ(out) {
		dirty[p] = true
	}
	return dirty, nil
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
