// Package approvalrecovery recovers the WORDING a released approval signed,
// for ledger records written before the ledger kept it.
//
// The gap it closes is a real one and it is not small. Until LedgerRecord.Content
// existed, approving a claim recorded a hash and not the text. That is enough
// to PROVE the text later moved — which is what readiness.CauseUnapprovedEdit
// reports — and not enough to SHOW what it moved from, so every claim edited
// after such an approval reaches the viewer with a panel that can only say the
// approved wording was not kept. For a corpus that has been locking claims for
// months, that is every claim in the state the panel exists for.
//
// The approved bytes are not gone, though. A claim is a file in the project's
// own repository, and the revision that was approved is still in its history.
// What history cannot say is WHICH revision — commit messages are prose and
// the approval timestamp is not a commit id. The ledger answers that exactly:
// it signed LockedClaimHash of the claim as approved, so a revision whose
// LockedClaimHash equals the signed hash IS the approved claim, byte for byte
// over every persisted field. Recovery is therefore a search with a proof at
// the end of it, not an inference from dates or messages, and a revision that
// does not hash equal is never offered.
//
// Two properties follow from that, and both are load-bearing:
//
//   - Recovery cannot forge an approval. It writes only content whose hash the
//     existing record already certifies; it changes no hash, no approval, no
//     timestamp, and no claim status. A tampered or reconstructed revision
//     cannot hash equal without breaking SHA-256.
//   - Recovery cannot quietly fail upward. A claim whose history holds no
//     matching revision — a corpus imported without history, a claim approved
//     from an uncommitted working tree, a rewritten history — comes back
//     unrecovered and NAMED, and the viewer keeps saying the wording was not
//     retained. There is no "closest" match.
//
// This package reads git and the lock store. It never writes either; the
// caller (cmd/dossierx claim recover-approved-content) owns the write, behind
// the dry-run and reason discipline every other approval-touching verb uses.
package approvalrecovery

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/gitrepo"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// ErrGitUnavailable means recovery cannot run at all: git is not installed, or
// the project is not inside a work tree. It is a refusal and not a silent
// empty result, because "no approvals could be recovered" and "nothing was
// looked at" are different answers and a caller that cannot tell them apart
// will report the second as the first.
var ErrGitUnavailable = errors.New("approved-content recovery needs git and a work tree")

// MaxRevisionsPerClaim bounds the history walked for ONE claim.
//
// The cost of recovery is (eligible claims x revisions examined), and the
// second factor is the project's history, which this package does not control.
// A cap keeps one pathological file — a claim edited thousands of times, or a
// --follow that walks into a large unrelated file after a rename — from
// turning a recovery run into an unbounded one.
//
// Hitting it is DISCLOSED, never swallowed: the outcome carries Capped, the
// command prints it, and the claim is reported unrecovered rather than
// reported as having no approved revision. A cap that could be mistaken for an
// answer would be worse than no cap at all.
//
// It is a var rather than a const so the cap ITSELF is testable. A cap that
// can only be exercised by building a fixture with more commits than it
// allows is a cap nothing checks, and "we disclose when we stop early" is a
// promise that has to be executed to be worth anything.
var MaxRevisionsPerClaim = 500

// Outcome is what recovery found, or did not find, for one claim.
type Outcome struct {
	// ClaimID is the claim, and Path is its file as the repository names it.
	ClaimID string `json:"claim_id"`
	Path    string `json:"path"`

	// Hash is the LockedClaimHash the ledger signed — the thing being
	// searched for, quoted so a reader can check the match by hand.
	Hash string `json:"hash"`

	// ApprovedAt/By are the approval this recovers the wording for.
	ApprovedAt string `json:"approved_at,omitempty"`
	ApprovedBy string `json:"approved_by,omitempty"`

	// Commit and CommitDate identify the revision whose content hashed equal.
	// Empty when nothing matched.
	Commit     string `json:"commit,omitempty"`
	CommitDate string `json:"commit_date,omitempty"`

	// Revisions is how many revisions were examined before the answer was
	// reached, and Capped says the walk stopped at MaxRevisionsPerClaim with
	// history still unread. Capped is never true on a found outcome.
	Revisions int  `json:"revisions"`
	Capped    bool `json:"capped,omitempty"`

	// Reason explains an unrecovered outcome in the words the command prints.
	Reason string `json:"reason,omitempty"`

	// claim is the recovered content. It is unexported so no consumer can
	// take it without going through Found, which is the only thing that says
	// it hashed equal.
	claim model.Claim
}

// Found reports whether this outcome carries an approved revision, and returns
// it. The content is reachable only this way, so a caller cannot read a
// candidate that failed the hash check.
func (o Outcome) Found() (model.Claim, bool) {
	if o.Commit == "" {
		return model.Claim{}, false
	}
	return o.claim, true
}

// Result is one recovery run.
type Result struct {
	// Recovered and Unrecovered partition the eligible claims. Both are
	// sorted by claim id so two runs over the same corpus print the same
	// report.
	Recovered   []Outcome `json:"recovered"`
	Unrecovered []Outcome `json:"unrecovered"`

	// Eligible is how many claims were in the state recovery applies to:
	// edited since a released approval, with no content on the record yet.
	// It is stated separately so "0 recovered" out of 0 eligible cannot read
	// like "0 recovered" out of 14.
	Eligible int `json:"eligible"`

	// AlreadyRetained is how many edited claims already carry their approved
	// wording and were therefore not searched for. Recovery never overwrites
	// content that is already on the record.
	AlreadyRetained int `json:"already_retained"`
}

// Eligible returns the claims recovery applies to: those lock.EditedSinceApproval
// reports, minus the ones whose record already carries its content.
//
// It shares the edited-since-approval predicate with readiness and
// approvaledit rather than restating it — see lock.EditedSinceApproval for why
// that question has one implementation. The extra condition here is this
// package's alone: a record that already has its wording needs no search, and
// re-deriving one would risk replacing a retained approval with a historical
// guess.
func Eligible(claims []model.Claim, store *lock.Store) (eligible []model.Claim, alreadyRetained int) {
	for _, c := range claims {
		if _, ok := lock.EditedSinceApproval(c, store); !ok {
			continue
		}
		if _, retained := store.ApprovedContent(c.ID); retained {
			alreadyRetained++
			continue
		}
		eligible = append(eligible, c)
	}
	sort.Slice(eligible, func(i, j int) bool { return eligible[i].ID < eligible[j].ID })
	return eligible, alreadyRetained
}

// Recover searches each eligible claim's history for the revision whose
// LockedClaimHash equals the hash its released approval signed.
//
// dir is the project directory; the runner re-anchors itself at the
// repository top level, so a claims directory outside the config's own folder
// (an ordinary monorepo layout) is searched correctly rather than read as
// being outside the repository. See gitrepo.NewRunner.
//
// COST. Let A be the eligible claims and R the revisions of one claim's file,
// bounded by MaxRevisionsPerClaim. The walk is O(A) invocations of `git log`
// plus at most O(A x R) invocations of `git show`, each followed by one YAML
// parse and one LockedClaimHash of a single claim. There is no dependency
// traversal, no path enumeration, and no term that grows with the corpus's
// edges: a claim's cost depends on ITS OWN history and nothing else. Memory is
// one revision's bytes at a time plus one recovered claim per eligible claim;
// nothing accumulates across revisions. The search stops at the first hash
// match, so the common case — a claim approved recently — reads a handful of
// revisions, not R.
//
// It mutates nothing. Every git command it issues is a read.
func Recover(claims []model.Claim, store *lock.Store, dir string) (Result, error) {
	eligible, alreadyRetained := Eligible(claims, store)
	// Both lists start empty rather than nil so the envelope carries `[]` and
	// never `null`. An agent writing `for _, o := range data.recovered` over a
	// run that recovered nothing should get an empty list, which is what
	// happened, not a null it has to guard.
	result := Result{
		Recovered:       []Outcome{},
		Unrecovered:     []Outcome{},
		Eligible:        len(eligible),
		AlreadyRetained: alreadyRetained,
	}
	if len(eligible) == 0 {
		return result, nil
	}
	runner, err := gitrepo.NewRunner(dir, ErrGitUnavailable)
	if err != nil {
		return Result{}, err
	}
	runner.SetVerb("claim recover-approved-content")

	for _, c := range eligible {
		record, _ := store.Record(c.ID)
		out := Outcome{
			ClaimID:    c.ID,
			Hash:       record.Hash,
			ApprovedAt: record.At,
			ApprovedBy: record.Actor,
		}
		spec, specErr := runner.Spec(c.SourcePath)
		if specErr != nil {
			out.Path = c.SourcePath
			out.Reason = "the claim file is outside the git work tree, so no commit can carry it"
			result.Unrecovered = append(result.Unrecovered, out)
			continue
		}
		out.Path = spec
		search(runner, spec, record.Hash, &out)
		if out.Commit != "" {
			result.Recovered = append(result.Recovered, out)
		} else {
			result.Unrecovered = append(result.Unrecovered, out)
		}
	}
	sort.Slice(result.Recovered, func(i, j int) bool { return result.Recovered[i].ClaimID < result.Recovered[j].ClaimID })
	sort.Slice(result.Unrecovered, func(i, j int) bool { return result.Unrecovered[i].ClaimID < result.Unrecovered[j].ClaimID })
	return result, nil
}

// search walks one file's history newest-first and fills out with the first
// revision whose content hashes equal to want.
//
// Newest-first is not arbitrary. The revision being looked for is the one that
// was approved, and an approval is followed by the edits that released it —
// usually few. Walking from the other end would read the claim's whole life
// before reaching the answer on every single claim.
func search(runner *gitrepo.Runner, spec, want string, out *Outcome) {
	revisions, err := history(runner, spec)
	if err != nil {
		out.Reason = "git could not read this file's history: " + err.Error()
		return
	}
	if len(revisions) == 0 {
		out.Reason = "this file has no committed history to search"
		return
	}
	for _, rev := range revisions {
		if out.Revisions >= MaxRevisionsPerClaim {
			out.Capped = true
			out.Reason = "stopped after " + strconv.Itoa(MaxRevisionsPerClaim) +
				" revisions with history still unread; the approved revision was not among them"
			return
		}
		out.Revisions++
		blob, err := runner.Run("show", rev.commit+":"+rev.path)
		if err != nil {
			// A revision git will not hand over is one revision, not the
			// answer. Keep walking; the reason for the whole claim is set
			// only if nothing matches.
			continue
		}
		candidate, err := loader.ParseClaim(blob, rev.path)
		if err != nil {
			// A revision that does not parse under today's strict loader
			// cannot be the approved claim either: the approval was taken
			// over a claim this engine had loaded.
			continue
		}
		if lock.LockedClaimHash(candidate) != want {
			continue
		}
		// The proof. Everything after this point is reporting.
		out.Commit = rev.commit
		out.CommitDate = rev.date
		candidate.SourcePath = ""
		out.claim = candidate
		return
	}
	out.Reason = "no revision of this file hashes to the approved content (" +
		strconv.Itoa(out.Revisions) + " searched)"
}

// revision is one commit that touched the file, with the path the file had AT
// that commit — which is not always the path it has now.
type revision struct {
	commit string
	date   string
	path   string
}

// history lists the commits touching spec, newest first, carrying each
// commit's own path for the file.
//
// --follow is what makes a renamed claim recoverable, and carrying the
// per-commit path is what makes --follow usable: after a rename, `git show
// <old commit>:<current path>` names a file that did not exist yet, so a
// walker that reused today's path would silently skip every revision before
// the move — exactly the revisions a long-lived claim's approval is likely to
// be in.
func history(runner *gitrepo.Runner, spec string) ([]revision, error) {
	// The NUL prefix on the format line separates commit records without
	// relying on blank lines, which --name-only also emits.
	out, err := runner.Run("log", "--follow", "--name-only",
		"--format=%x00%H %ad", "--date=short", "--", spec)
	if err != nil {
		return nil, err
	}
	var revisions []revision
	for _, chunk := range strings.Split(string(out), "\x00") {
		lines := strings.Split(strings.ReplaceAll(chunk, "\r\n", "\n"), "\n")
		head := strings.Fields(lines[0])
		if len(head) < 2 {
			continue
		}
		rev := revision{commit: head[0], date: head[1]}
		for _, line := range lines[1:] {
			name := strings.TrimSpace(line)
			if name == "" {
				continue
			}
			// git C-quotes a path containing a quote, a backslash or a
			// control character even with core.quotepath=false. Unquoting is
			// cheap insurance; a path this fails on is skipped rather than
			// searched under a wrong name.
			if strings.HasPrefix(name, `"`) {
				unquoted, uerr := strconv.Unquote(name)
				if uerr != nil {
					continue
				}
				name = unquoted
			}
			rev.path = name
			break
		}
		if rev.path == "" {
			continue
		}
		revisions = append(revisions, rev)
	}
	return revisions, nil
}

// Apply writes each recovered claim onto its ledger record's Content, and
// returns the claim ids it wrote.
//
// Every write goes through lock.Store.RetainApprovedContent, which re-checks
// the hash itself. That check is redundant against search's own, and it stays
// there rather than here: the guarantee that a record's content is only ever
// the bytes its own hash certifies belongs to the package that owns the
// record, not to whichever caller happens to be writing today.
//
// A record that gained content between the search and the write is left
// alone, so a recovered historical revision can never replace retained
// wording. Any mismatch stops the run rather than writing part of it.
func Apply(store *lock.Store, result Result) ([]string, error) {
	var written []string
	for _, out := range result.Recovered {
		claim, ok := out.Found()
		if !ok {
			continue
		}
		stored, err := store.RetainApprovedContent(out.ClaimID, claim)
		if err != nil {
			return nil, fmt.Errorf("approved-content recovery: %w; nothing further was written", err)
		}
		if stored {
			written = append(written, out.ClaimID)
		}
	}
	sort.Strings(written)
	return written, nil
}
