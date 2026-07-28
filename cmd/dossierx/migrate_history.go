// migrate_history.go asks GIT what this project looked like the last time
// anybody committed it, and hands migrate.go the answer.
//
// WHY MIGRATE, OF ALL COMMANDS, NEEDS EVIDENCE FROM OUTSIDE THE DIRECTORY.
// lock.Store.LedgerDowngraded states the limit of the in-directory evidence in
// its own words: an attacker who deletes the digest store as well as the ledger,
// in the same commit, "produces a project that is byte-for-byte the shape of a
// legitimate pre-ledger one, and nothing in these three files can tell them
// apart — the evidence is gone", and closing it "needs evidence that lives
// outside the project directory entirely (a signature, or the commit history),
// which is a different release". This is that release, and migrate is where it
// matters most: migrate is the ONE command whose job is to bless on-disk content
// as the approved baseline, so it is the one command an attacker wants to reach.
//
// The two verified reproductions this file exists to refuse, both in ONE commit,
// both previously exiting 0 through `migrate --adopt` and then through
// `check --staged` and `check --validate`:
//
//	A. a fully ledger-covered project: delete the "ledger" key, set "version"
//	   back to 1, delete .dossierx-comment-digest.json, rewrite a locked claim's
//	   body, run `dossierx migrate --adopt`. The directory now has the exact
//	   shape of an honest v0.2.x project, so every in-directory predicate agrees
//	   it is one, and the rewritten body is adopted as the approved baseline.
//	B. an honest pre-ledger project: rewrite a locked claim's body, run
//	   `dossierx migrate --adopt`. There is no pre-ledger record of the approved
//	   content to compare against — inside the directory. In git there is: the
//	   claim's last committed blob.
//
// Reproduction A's directory is a forgery of reproduction B's directory. What
// separates them is not in either directory. It is that in A the last commit
// carried a lock ledger and a comment digest store, and in B it did not.
//
// THE TWO QUESTIONS, and they are deliberately only two:
//
//   - WAS THIS PROJECT ALREADY LEDGER-COVERED AT HEAD? Asked with exactly the
//     evidence lock.Store.LedgerDowngraded asks the working directory for — a
//     comment digest store, or a "ledger" key in the lock store — read out of
//     HEAD's tree instead. Deliberately NOT the store's "version" field: that
//     field is the one an attacker edits, and reading their number back from a
//     tree they also wrote adds nothing the ledger key does not already say.
//   - DOES A LOCKED CLAIM DIFFER FROM THE CONTENT HEAD HOLDS FOR IT? Asked as
//     lock.LockedClaimHash of HEAD's decoded blob against lock.LockedClaimHash
//     of the claim on disk, never as a byte comparison of the two files. That is
//     what makes the answer survive honest noise: LockedClaimHash's deny-list
//     excludes status, review_pending and comments, so an upgrade run that
//     reconciles review_pending or a `dossierx check` that rewrites a comment
//     block is invisible here, and so is any yaml reformatting, because both
//     sides are compared after decoding.
//
// WHY "HEAD" AND NOT THE WHOLE HISTORY. migrate runs at a keyboard, on a working
// tree, and what it is being asked to adopt is the working tree. HEAD is the
// last state a reviewer could have seen, so it is the state the adoption is
// being smuggled past. Reading further back would not make the answer better: an
// adoption is a statement about what is on disk NOW against what was last agreed,
// and "last agreed" is one commit.
//
// This paragraph used to hand a case off to check.RuleIntegrityStoreRemoved —
// the gate rule that compared a commit against its PARENT and refused one that
// dropped an integrity store. That rule and that comparison were REMOVED (the
// parent commit is outside the commit but not outside the committer, and the
// rule refused ordinary reverts), so nothing downstream catches a tamper spread
// across two commits on this route any more, and saying otherwise here would be
// borrowing a guarantee that no longer exists. What is left is this file's own
// two questions, asked of HEAD, which are unaffected: they compare the working
// tree against the last commit, and both of the reproductions above put the
// forgery in the working tree.
//
// WHY NOTHING HERE EVER REFUSES ON ITS OWN, and why this file has no error
// return. Git can be absent, the project can live outside a work tree, HEAD can
// be unborn on the repository's first commit, and a claim file can be untracked.
// Every one of those is an HONEST state, and a migration that refused them would
// be the outage the implicit grandfathering it replaced existed to prevent. So
// the answer is a three-way one — corroborated, refuted, or NOT LOOKED AT — and
// the third is reported (a warning on the run, a named side effect on the
// preview) rather than either refusing or passing in silence, because "nothing
// was found" and "nothing could be looked for" are different sentences and only
// one of them is true.
//
// A SHALLOW CLONE IS NOT ONE OF THOSE STATES. Everything here reads HEAD's own
// tree, which every clone has complete however shallow, so a depth-1 CI checkout
// is fully corroborated and gets no degradation notice at all. That is why this
// file survived the removal of `check --staged`'s parent-commit comparison: that
// comparison needed a tree a shallow clone does not have, and could be switched
// off by rewriting the history it read. Nothing here reads history — it reads
// the one commit the working tree was made from.
//
// WHY THE GIT PLUMBING IS HERE RATHER THAN SHARED WITH internal/check. That
// package's gitRunner is unexported and is built around the INDEX — pathspecs
// anchored at the repository top level, cat-file --batch over staged entries —
// because `check --staged` judges the commit being made. This asks a different
// tree a much smaller set of questions, and asks them by handing git a path
// relative to the project directory ("HEAD:./claims/x.yaml", run with the
// project directory as cwd) so git does the path resolution itself. That is not
// a shortcut: computing repository-relative specs by hand is what made
// internal/check anchor at `rev-parse --show-toplevel`, because on macOS a temp
// directory reached through /var resolves to /private/var and any arithmetic
// over the two absolute paths disagrees with git about a path both can open.
// Letting git resolve "./" cannot have that bug. Consolidating the two runners
// is worth doing the day a third caller needs one; today it would mean exporting
// index machinery this file does not use.
//
// EVERYTHING HERE IS READ-ONLY. git is asked questions; no object is written, no
// ref is moved, no file in the project is touched.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/digest"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// migrateHistory is git's answer about this project, in the shape the migration
// plan consumes it.
//
// Looked is the field every consumer has to read first: false means git was
// never in a position to answer, and the other fields are all zero because
// nothing was asked, NOT because nothing was found.
type migrateHistory struct {
	// Looked reports whether the comparison against HEAD could be made at all.
	Looked bool

	// Unlooked is why it could not, in a sentence a human can act on. Set only
	// when Looked is false.
	Unlooked string

	// Covered is true when HEAD's tree proves this project had already been
	// through a ledger-aware build, whatever the lock store on disk now says.
	Covered bool

	// CoveredBy names the evidence, because the two pieces have different
	// restore instructions and a reader who cannot tell which one fired cannot
	// act on it.
	CoveredBy string

	// Modified is the ids of claims that are locked BOTH at HEAD and on disk and
	// whose signed content differs between the two. This is the refusal: a
	// locked claim's approved content may only ever change through
	// unlock -> fix -> lock, so a difference here is an edit no approval covers.
	Modified []string

	// NewlyLocked is the ids of claims that read status: locked on disk and did
	// NOT at HEAD.
	//
	// It is a WARNING and not a refusal, and the line between them is drawn on
	// which honest path each would break. A content change to a claim that was
	// already locked has no honest cause on an un-migrated project — locking is
	// refused there until the migration runs — so refusing it costs nothing. A
	// status flip does have one, narrow but real: a `dossierx claim lock` run
	// under the OLD v0.2.x binary and not yet committed when the upgrade
	// happened. Refusing that would wedge the upgrader completely, because the
	// commit that would clear the refusal is itself refused by the pre-commit
	// hook until the migration runs. So it is put in front of the human instead,
	// in the same list they are being asked to approve.
	NewlyLocked []string

	// Uncorroborated is the ids of locked claims HEAD holds no readable copy of
	// — untracked, newly added, or moved in this working tree. Reported, never
	// refused: a claim git has never seen has no committed state to differ from.
	Uncorroborated []string
}

// notes renders everything git could NOT corroborate, plus everything it
// corroborated that a human approving the adoption should still see. One list,
// rendered identically into the preview's side effects and the run's warnings,
// so the two cannot say different things about the same run.
//
// It is empty on the ordinary case — a git project whose locked claims all match
// version control — because a notice that fires on correct state is a notice
// people learn to scroll past.
func (h migrateHistory) notes() []string {
	var out []string
	if !h.Looked {
		return append(out, "NOT CHECKED AGAINST VERSION CONTROL: "+h.Unlooked+
			". This run cannot tell an honest pre-ledger project from one whose lock ledger was deleted, and cannot tell a locked claim's approved content from an edit made to it just now — everything below is adopted on the working tree's word alone")
	}
	if len(h.NewlyLocked) > 0 {
		out = append(out, fmt.Sprintf(
			"%d claim(s) read status: locked here and did NOT in the last commit, so what is being adopted for them is a lock version control has never seen: %s. If that was not a dossierx claim lock you ran with an older build a moment ago, restore them (git checkout HEAD -- <path>) before adopting",
			len(h.NewlyLocked), strings.Join(h.NewlyLocked, ", ")))
	}
	if len(h.Uncorroborated) > 0 {
		out = append(out, fmt.Sprintf(
			"%d locked claim(s) have no readable copy in the last commit (untracked, newly added, or moved), so their content could not be compared against version control and is adopted as-found: %s",
			len(h.Uncorroborated), strings.Join(h.Uncorroborated, ", ")))
	}
	return out
}

// inspectMigrateHistory answers both questions for cfg and claims, and NEVER
// fails: every way of not getting an answer comes back as Looked false with the
// reason attached. See this file's header for why that is the contract rather
// than an error return.
func inspectMigrateHistory(cfg *config.Config, claims []model.Claim) migrateHistory {
	g, why := newMigrateGit(cfg)
	if g == nil {
		return migrateHistory{Unlooked: why}
	}
	if ok, err := g.hasHead(); err != nil {
		return migrateHistory{Unlooked: "git could not resolve HEAD in " + g.dir + ": " + err.Error()}
	} else if !ok {
		return migrateHistory{Unlooked: "this repository has no commits yet, so there is no previously committed state to compare against"}
	}

	h := migrateHistory{Looked: true}
	if err := h.readCoverage(g, cfg); err != nil {
		return migrateHistory{Unlooked: err.Error()}
	}
	if err := h.readClaims(g, cfg, claims); err != nil {
		return migrateHistory{Unlooked: err.Error()}
	}
	sort.Strings(h.Modified)
	sort.Strings(h.NewlyLocked)
	sort.Strings(h.Uncorroborated)
	return h
}

// readCoverage answers "was this project already ledger-covered at HEAD?" from
// the two pieces of evidence lock.Store.LedgerDowngraded reads in the working
// directory, read out of HEAD's tree instead.
//
// The digest store is asked about first because its evidence is the stronger of
// the two and needs no parsing: the file did not exist before v0.3.0, so a
// commit that carries one is a commit made by a ledger-aware build, full stop.
func (h *migrateHistory) readCoverage(g *migrateGit, cfg *config.Config) error {
	held, err := g.headHolds(digest.StorePath(cfg))
	if err != nil {
		return err
	}
	if held {
		h.Covered = true
		h.CoveredBy = digest.StoreFileName + " was in the last commit, and that file did not exist before the lock ledger did"
		return nil
	}

	raw, ok, err := g.headBlob(storePath(cfg))
	if err != nil {
		return err
	}
	if !ok {
		// HEAD carried no lock store. That is the ordinary shape of a project
		// whose store is untracked or newly added, and it is not evidence of
		// coverage — the absence of a ledger is exactly what a pre-ledger
		// project looks like.
		return nil
	}
	// Only the KEY's presence is read, never the records inside it and never
	// the version field beside it. A store that predates the ledger cannot
	// carry the key at all, so presence alone is conclusive and an emptied map
	// ("ledger": {}, the smaller diff) is caught by the same test — which is the
	// half lock.Store.LedgerDowngraded had to grow for the same reason.
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		// A store that does not decode says nothing either way, and guessing
		// from a file this command is not going to read anyway would be worse
		// than saying nothing.
		return nil
	}
	if _, has := doc["ledger"]; has {
		h.Covered = true
		h.CoveredBy = lock.StoreFileName + " carried a \"ledger\" key in the last commit, and a store that predates the lock ledger cannot have one"
	}
	return nil
}

// readClaims compares every LOCKED claim on disk against the copy HEAD holds.
//
// Draft claims are not asked about at all, and that is the point of the whole
// design rather than an optimisation: draft claims are free to edit ON PURPOSE,
// and a migration that reported one as changed would be reporting correct work.
func (h *migrateHistory) readClaims(g *migrateGit, cfg *config.Config, claims []model.Claim) error {
	for _, c := range claims {
		if c.Status != model.StatusLocked {
			continue
		}
		raw, ok, err := g.headBlob(c.SourcePath)
		if err != nil {
			return err
		}
		if !ok {
			h.Uncorroborated = append(h.Uncorroborated, c.ID)
			continue
		}
		var was model.Claim
		if err := decodeClaimBytes(c.SourcePath, raw, &was); err != nil {
			// HEAD's copy does not decode as a claim, so there is nothing to
			// compare. Naming it as uncorroborated is the honest answer: this
			// run did not establish that the content is unchanged.
			h.Uncorroborated = append(h.Uncorroborated, c.ID)
			continue
		}
		if was.Status != model.StatusLocked {
			h.NewlyLocked = append(h.NewlyLocked, c.ID)
			continue
		}
		if lock.LockedClaimHash(was) != lock.LockedClaimHash(c) {
			h.Modified = append(h.Modified, c.ID)
		}
	}
	return nil
}

// decodeClaimBytes decodes one claim from raw under loader.LoadClaims' rules:
// strict fields, exactly one YAML document per file. The strictness matters —
// a lenient decode would silently drop a field LockedClaimHash signs, and two
// claims that differ only in a dropped field would hash the same, which is the
// one failure mode this comparison must not have.
func decodeClaimBytes(sourcePath string, raw []byte, out *model.Claim) error {
	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("parse %s as of HEAD: %w", sourcePath, err)
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%s as of HEAD holds more than one YAML document", sourcePath)
	}
	// SourcePath is yaml:"-" and therefore unsigned (see lock.PersistedYAMLName),
	// so it is set for message-rendering only and cannot affect the comparison.
	out.SourcePath = sourcePath
	return nil
}

// ---------------------------------------------------------------------
// the git binary, HEAD half
// ---------------------------------------------------------------------

// migrateGit runs git read-only with the PROJECT DIRECTORY as its working
// directory, which is what lets every path be handed to git as "./<relative>"
// and resolved by git rather than by arithmetic here. See this file's header.
type migrateGit struct {
	bin string
	dir string
}

// newMigrateGit locates git and confirms the project directory is inside a work
// tree. Both failures come back as a nil runner and a sentence, never an error:
// this whole file degrades, it does not refuse.
func newMigrateGit(cfg *config.Config) (git *migrateGit, unavailable string) {
	bin, err := exec.LookPath("git")
	if err != nil {
		return nil, "git is not installed or not on PATH, so this project's previously committed state could not be read"
	}
	g := &migrateGit{bin: bin, dir: cfg.Dir()}
	out, ok, err := g.probe("rev-parse", "--is-inside-work-tree")
	if err != nil {
		return nil, "git would not answer for " + g.dir + ": " + err.Error()
	}
	if !ok || strings.TrimSpace(string(out)) != "true" {
		return nil, g.dir + " is not inside a git work tree, so there is no committed history to compare this project against"
	}
	return g, ""
}

// probe runs git and returns ok=false when git itself exits non-zero — which is
// how git answers "no such path in that tree", "not a repository" and "that is
// outside the repository", all of which are answers rather than failures here. A
// real failure (git vanished mid-run, fork failed) is still an error, because a
// corroboration must never read "the tool broke" as "nothing to report".
//
// core.quotepath=false is set for the same reason internal/check sets it: so a
// non-ASCII path comes back raw instead of C-quoted. Nothing here depends on the
// auditee's git configuration.
func (g *migrateGit) probe(args ...string) (out []byte, ok bool, err error) {
	full := append([]string{"-c", "core.quotepath=false"}, args...)
	cmd := exec.Command(g.bin, full...) //nolint:gosec // fixed binary, fixed argv; no shell involved
	cmd.Dir = g.dir
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err = cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, false, nil
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, false, fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return out, true, nil
}

// hasHead reports whether HEAD resolves to a commit. It does not on the
// repository's very first commit, which is an honest state and not a finding.
func (g *migrateGit) hasHead() (bool, error) {
	out, ok, err := g.probe("rev-parse", "--verify", "--quiet", "HEAD^{commit}")
	if err != nil || !ok {
		return false, err
	}
	return strings.TrimSpace(string(out)) != "", nil
}

// rev names target — an absolute path, or one relative to the project directory
// — as the revision syntax "HEAD:./<path>". The "./" prefix is what tells git to
// resolve the rest against ITS working directory, which is this runner's; git
// then reports "outside repository" itself for anything that climbs out, so this
// function has no failure of its own.
func (g *migrateGit) rev(target string) string {
	rel, err := filepath.Rel(g.dir, target)
	if err != nil {
		// Rel only fails when one path is absolute and the other is not, which
		// cannot happen for the two callers (both pass paths built from
		// cfg.Dir()). Handing git the original is still safe: it will resolve it
		// or say no, and "no" is a valid answer everywhere this is used.
		rel = target
	}
	return "HEAD:./" + filepath.ToSlash(rel)
}

// headHolds reports whether HEAD's tree contains target.
func (g *migrateGit) headHolds(target string) (bool, error) {
	_, ok, err := g.probe("cat-file", "-e", g.rev(target))
	return ok, err
}

// headBlob returns the bytes HEAD holds for target, with ok=false when that
// commit did not carry it.
func (g *migrateGit) headBlob(target string) (blob []byte, found bool, err error) {
	out, ok, err := g.probe("show", g.rev(target))
	if err != nil || !ok {
		return nil, false, err
	}
	return out, true, nil
}
