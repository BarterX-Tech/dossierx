package approvalrecovery

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// repo is a disposable git work tree with a claims directory in it. Every
// mutation case in this file runs on its own, because recovery's whole
// argument is about what it does and does not write.
type repo struct {
	t    *testing.T
	root string
	dir  string // the project directory (root/project)
}

func newRepo(t *testing.T) *repo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		// A missing tool is a blocker to report, not permission to reduce
		// coverage: these cases cannot run without git and must not be
		// mistaken for having passed.
		t.Fatal("git is required for approved-content recovery tests; install git")
	}
	root := t.TempDir()
	r := &repo{t: t, root: root, dir: filepath.Join(root, "project")}
	if err := os.MkdirAll(filepath.Join(r.dir, "claims"), 0o755); err != nil {
		t.Fatal(err)
	}
	r.git("init")
	r.git("config", "user.email", "fixture@example.com")
	r.git("config", "user.name", "Fixture")
	return r
}

func (r *repo) git(args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.root
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// write puts a claim on disk at rel (relative to the project directory) and
// returns the parsed claim, so a test can hash exactly what it wrote.
func (r *repo) write(rel, body string, extra ...string) model.Claim {
	r.t.Helper()
	return r.writeID(rel, "widget.contract."+strings.TrimSuffix(filepath.Base(rel), ".yaml"), body, extra...)
}

// writeID is write with the claim id given rather than derived from the file
// name. The rename case needs it: a claim's id is part of what the approval
// hash signs, so moving a claim's FILE while its id stays put is the rename
// that actually happens, and deriving the id from the path would make the
// fixture change two things at once and prove neither.
func (r *repo) writeID(rel, id, body string, extra ...string) model.Claim {
	r.t.Helper()
	raw := "id: " + id + "\nfacet: contract\nstatus: draft\nsummary: Fixture claim used by the engine test corpus.\nbody: |\n"
	for _, line := range strings.Split(body, "\n") {
		raw += "  " + line + "\n"
	}
	raw += strings.Join(extra, "\n")
	path := filepath.Join(r.dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		r.t.Fatal(err)
	}
	claim, err := loader.ParseClaim([]byte(raw), path)
	if err != nil {
		r.t.Fatalf("fixture claim does not parse: %v", err)
	}
	return claim
}

func (r *repo) commit(message string) {
	r.t.Helper()
	r.git("add", "-A")
	r.git("commit", "-m", message)
}

func (r *repo) claims() []model.Claim {
	r.t.Helper()
	claims, err := loader.LoadClaims(filepath.Join(r.dir, "claims"))
	if err != nil {
		r.t.Fatal(err)
	}
	return claims
}

// releasedStore is a ledger holding one released approval per entry, written
// in the legacy shape: a hash with no wording behind it.
func releasedStore(t *testing.T, approvals map[string]string) *lock.Store {
	t.Helper()
	store := &lock.Store{Ledger: map[string]lock.LedgerRecord{}}
	for id, hash := range approvals {
		store.Ledger[id] = lock.LedgerRecord{
			Subject: lock.SubjectClaim, Hash: hash,
			At: "2026-01-01T00:00:00Z", Actor: "approver", Reason: "reviewed",
			ReleasedAt: "2026-02-01T00:00:00Z", ReleasedBy: "maintainer", ReleasedReason: "rewording",
		}
	}
	return store
}

// The base case, and the one the whole feature turns on: the approved revision
// is in history, it is identified by HASH rather than by date or message, and
// the content that comes back is the approved content.
func TestRecoverFindsTheRevisionWhoseHashTheApprovalSigned(t *testing.T) {
	r := newRepo(t)
	approved := r.write("claims/alpha.yaml", "The approved wording.")
	r.commit("approve alpha")
	// Two later edits, so the answer is genuinely not "the newest revision"
	// and not "the oldest" either.
	r.write("claims/alpha.yaml", "An intermediate rewrite.")
	r.commit("rewrite once")
	r.write("claims/alpha.yaml", "The current wording.")
	r.commit("rewrite again")

	store := releasedStore(t, map[string]string{approved.ID: lock.LockedClaimHash(approved)})
	result, err := Recover(r.claims(), store, r.dir)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if result.Eligible != 1 || len(result.Recovered) != 1 || len(result.Unrecovered) != 0 {
		t.Fatalf("want 1 eligible, 1 recovered, 0 unrecovered; got %d/%d/%d",
			result.Eligible, len(result.Recovered), len(result.Unrecovered))
	}
	got, ok := result.Recovered[0].Found()
	if !ok {
		t.Fatal("a recovered outcome must carry its content")
	}
	if got.Body != approved.Body {
		t.Fatalf("recovered body %q, want %q", got.Body, approved.Body)
	}
	if lock.LockedClaimHash(got) != lock.LockedClaimHash(approved) {
		t.Fatal("recovered content must hash to the approval")
	}
	// The walk is newest-first, so reaching a revision two commits back costs
	// three reads and not the file's whole history.
	if result.Recovered[0].Revisions != 3 {
		t.Fatalf("searched %d revisions, want the 3 between HEAD and the approval", result.Recovered[0].Revisions)
	}
}

// A history with no matching revision must come back NAMED and empty, never
// with the nearest thing to a match. This is the case a corpus imported
// without history lands in, and answering it with an approximate revision
// would put words on the record that nobody approved.
func TestRecoverReportsAClaimWhoseApprovedRevisionIsNotInHistory(t *testing.T) {
	r := newRepo(t)
	current := r.write("claims/alpha.yaml", "The current wording.")
	r.commit("only ever this")

	// An approval for wording that was never committed.
	neverCommitted := current
	neverCommitted.Body = "Wording that only ever existed in a working tree.\n"
	store := releasedStore(t, map[string]string{current.ID: lock.LockedClaimHash(neverCommitted)})

	result, err := Recover(r.claims(), store, r.dir)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if len(result.Recovered) != 0 || len(result.Unrecovered) != 1 {
		t.Fatalf("want 0 recovered and 1 unrecovered, got %d/%d", len(result.Recovered), len(result.Unrecovered))
	}
	out := result.Unrecovered[0]
	if out.ClaimID != current.ID {
		t.Fatalf("unrecovered outcome names %q", out.ClaimID)
	}
	if _, ok := out.Found(); ok {
		t.Fatal("an unrecovered outcome must carry no content")
	}
	if !strings.Contains(out.Reason, "no revision") {
		t.Fatalf("the reason must say no revision matched, got %q", out.Reason)
	}
}

// A renamed claim must still be recoverable. Reusing today's path would make
// `git show <old commit>:<current path>` name a file that did not exist yet,
// and every revision before the move would be skipped in silence — which for a
// long-lived claim is exactly where its approval is.
func TestRecoverFollowsARenamedClaimFile(t *testing.T) {
	r := newRepo(t)
	const id = "widget.contract.stable"
	approved := r.writeID("claims/old-name.yaml", id, "The approved wording.")
	r.commit("approve under the old name")
	r.git("mv", "project/claims/old-name.yaml", "project/claims/new-name.yaml")
	r.commit("rename the claim file")
	// Rewrite under the new name so the approved revision is only reachable
	// through the rename.
	r.writeID("claims/new-name.yaml", id, "The current wording.")
	r.commit("rewrite after the rename")

	store := releasedStore(t, map[string]string{id: lock.LockedClaimHash(approved)})

	result, err := Recover(r.claims(), store, r.dir)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if len(result.Recovered) != 1 {
		t.Fatalf("a renamed claim must still be recoverable; unrecovered: %+v", result.Unrecovered)
	}
	got, _ := result.Recovered[0].Found()
	if got.Body != approved.Body {
		t.Fatalf("recovered body %q, want the pre-rename approved body %q", got.Body, approved.Body)
	}
}

// A monorepo layout — the git root above the project directory — must be
// searched, not read as "outside the repository". This is the shape the
// Curtainly corpus has, and the hole gitrepo.NewRunner's re-anchoring closes.
func TestRecoverWorksWhenTheProjectIsBelowTheRepositoryRoot(t *testing.T) {
	r := newRepo(t)
	approved := r.write("claims/alpha.yaml", "The approved wording.")
	r.commit("approve")
	r.write("claims/alpha.yaml", "The current wording.")
	r.commit("rewrite")

	if r.dir == r.root {
		t.Fatal("this fixture must put the project below the repository root")
	}
	store := releasedStore(t, map[string]string{approved.ID: lock.LockedClaimHash(approved)})
	result, err := Recover(r.claims(), store, r.dir)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if len(result.Recovered) != 1 {
		t.Fatalf("a project below the repository root must be searched; unrecovered: %+v", result.Unrecovered)
	}
}

// Recovery must never search — or overwrite — a record that already carries
// its wording, and must say so rather than counting it as nothing to do.
func TestRecoverSkipsRecordsThatAlreadyCarryTheirWording(t *testing.T) {
	r := newRepo(t)
	approved := r.write("claims/alpha.yaml", "The approved wording.")
	r.commit("approve")
	r.write("claims/alpha.yaml", "The current wording.")
	r.commit("rewrite")

	store := releasedStore(t, map[string]string{approved.ID: lock.LockedClaimHash(approved)})
	if _, err := store.RetainApprovedContent(approved.ID, approved); err != nil {
		t.Fatalf("seed retained content: %v", err)
	}
	result, err := Recover(r.claims(), store, r.dir)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if result.Eligible != 0 || result.AlreadyRetained != 1 {
		t.Fatalf("want 0 eligible and 1 already retained, got %d/%d", result.Eligible, result.AlreadyRetained)
	}
	if len(result.Recovered)+len(result.Unrecovered) != 0 {
		t.Fatal("a record that already carries its wording must not be searched")
	}
}

// A claim that is not edited-since-approval is not recovery's business, and
// the predicate that decides it is the shared one.
func TestRecoverIgnoresClaimsThatAreNotEditedSinceApproval(t *testing.T) {
	r := newRepo(t)
	claim := r.write("claims/alpha.yaml", "Unchanged since approval.")
	r.commit("approve")

	store := releasedStore(t, map[string]string{claim.ID: lock.LockedClaimHash(claim)})
	result, err := Recover(r.claims(), store, r.dir)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if result.Eligible != 0 {
		t.Fatalf("a claim matching its approval is not eligible; got %d", result.Eligible)
	}
}

// Recovery reads. It must leave the store, the claims and the repository
// exactly as it found them — the command's own write is the only mutation in
// this feature, and it happens somewhere else.
func TestRecoverMutatesNothing(t *testing.T) {
	r := newRepo(t)
	approved := r.write("claims/alpha.yaml", "The approved wording.")
	r.commit("approve")
	r.write("claims/alpha.yaml", "The current wording.")
	r.commit("rewrite")

	store := releasedStore(t, map[string]string{approved.ID: lock.LockedClaimHash(approved)})
	beforeRecord := store.Ledger[approved.ID]
	claims := r.claims()
	beforeClaims := fmt.Sprintf("%+v", claims)
	beforeStatus := r.git("status", "--porcelain")

	if _, err := Recover(claims, store, r.dir); err != nil {
		t.Fatalf("recover: %v", err)
	}

	if store.Ledger[approved.ID] != beforeRecord {
		t.Fatal("recovery must not write the ledger; the command owns that")
	}
	if got := fmt.Sprintf("%+v", claims); got != beforeClaims {
		t.Fatal("recovery must not mutate the claims it was given")
	}
	if got := r.git("status", "--porcelain"); got != beforeStatus {
		t.Fatalf("recovery must not touch the work tree:\n%s", got)
	}
}

// Outside a work tree, recovery must REFUSE. An empty result would say no
// approval could be recovered when in fact none was looked for, and a caller
// cannot tell those apart after the fact.
func TestRecoverRefusesOutsideAWorkTree(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "claims"), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := "id: widget.contract.alpha\nfacet: contract\nstatus: draft\nsummary: Fixture claim used by the engine test corpus.\nbody: |\n  Current.\n"
	if err := os.WriteFile(filepath.Join(dir, "claims", "alpha.yaml"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	claims, err := loader.LoadClaims(filepath.Join(dir, "claims"))
	if err != nil {
		t.Fatal(err)
	}
	approved := claims[0]
	approved.Body = "Approved.\n"
	store := releasedStore(t, map[string]string{claims[0].ID: lock.LockedClaimHash(approved)})

	if _, err := Recover(claims, store, dir); !errors.Is(err, ErrGitUnavailable) {
		t.Fatalf("outside a work tree recovery must refuse with ErrGitUnavailable, got %v", err)
	}
}

// Apply is the write, and it re-checks every hash through the store's own
// guard. A result carrying content for a record whose hash has since moved
// must stop the run rather than write part of it.
func TestApplyWritesOnlyWhatStillHashesToItsApproval(t *testing.T) {
	r := newRepo(t)
	approved := r.write("claims/alpha.yaml", "The approved wording.")
	r.commit("approve")
	r.write("claims/alpha.yaml", "The current wording.")
	r.commit("rewrite")

	store := releasedStore(t, map[string]string{approved.ID: lock.LockedClaimHash(approved)})
	result, err := Recover(r.claims(), store, r.dir)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}

	// Move the record's hash under the result, as a concurrent lock would.
	moved := store.Ledger[approved.ID]
	moved.Hash = strings.Repeat("0", 64)
	store.Ledger[approved.ID] = moved

	written, err := Apply(store, result)
	if err == nil {
		t.Fatal("applying content that no longer hashes to its record must refuse")
	}
	if len(written) != 0 {
		t.Fatalf("a refused apply must write nothing, wrote %v", written)
	}
	if store.Ledger[approved.ID].Content != nil {
		t.Fatal("a refused apply must leave the record without content")
	}

	// Restore, and the same result applies cleanly — so the refusal above
	// discriminates rather than failing always.
	moved.Hash = lock.LockedClaimHash(approved)
	store.Ledger[approved.ID] = moved
	written, err = Apply(store, result)
	if err != nil || len(written) != 1 || written[0] != approved.ID {
		t.Fatalf("apply: written=%v err=%v", written, err)
	}
	got, ok := store.ApprovedContent(approved.ID)
	if !ok || got.Body != approved.Body {
		t.Fatalf("applied body %q ok=%v, want %q", got.Body, ok, approved.Body)
	}
}

// Several claims at once must each get their own answer, and the report must
// be ordered so two runs over one corpus read the same.
func TestRecoverHandlesManyClaimsDeterministically(t *testing.T) {
	r := newRepo(t)
	approvals := map[string]string{}
	var approved []model.Claim
	for _, name := range []string{"charlie", "alpha", "bravo"} {
		c := r.write("claims/"+name+".yaml", "Approved "+name+".")
		approved = append(approved, c)
	}
	r.commit("approve all three")
	for _, name := range []string{"charlie", "alpha", "bravo"} {
		r.write("claims/"+name+".yaml", "Current "+name+".")
	}
	r.commit("rewrite all three")

	for _, c := range approved {
		approvals[c.ID] = lock.LockedClaimHash(c)
	}
	// One claim whose approval names wording never committed, so the run has
	// both outcomes in it.
	unfindable := approved[0]
	unfindable.Body = "Never on disk.\n"
	approvals[approved[0].ID] = lock.LockedClaimHash(unfindable)

	store := releasedStore(t, approvals)
	result, err := Recover(r.claims(), store, r.dir)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if result.Eligible != 3 || len(result.Recovered) != 2 || len(result.Unrecovered) != 1 {
		t.Fatalf("want 3 eligible, 2 recovered, 1 unrecovered; got %d/%d/%d",
			result.Eligible, len(result.Recovered), len(result.Unrecovered))
	}
	for _, list := range [][]Outcome{result.Recovered, result.Unrecovered} {
		for i := 1; i < len(list); i++ {
			if list[i-1].ClaimID > list[i].ClaimID {
				t.Fatalf("outcomes must be sorted by claim id, got %s before %s", list[i-1].ClaimID, list[i].ClaimID)
			}
		}
	}
}

// Stopping early must be DISCLOSED and must not look like an answer. A claim
// whose walk hit the cap is reported unrecovered with Capped set and a reason
// that says history was left unread — never as "no revision matched", which
// would tell a reader the approved wording is gone when it may be two commits
// further back.
func TestRecoverDisclosesWhenItStopsAtTheRevisionCap(t *testing.T) {
	r := newRepo(t)
	approved := r.write("claims/alpha.yaml", "The approved wording.")
	r.commit("approve")
	for i := 0; i < 4; i++ {
		r.write("claims/alpha.yaml", fmt.Sprintf("Rewrite %d.", i))
		r.commit(fmt.Sprintf("rewrite %d", i))
	}

	restore := MaxRevisionsPerClaim
	MaxRevisionsPerClaim = 2
	defer func() { MaxRevisionsPerClaim = restore }()

	store := releasedStore(t, map[string]string{approved.ID: lock.LockedClaimHash(approved)})
	result, err := Recover(r.claims(), store, r.dir)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if len(result.Recovered) != 0 || len(result.Unrecovered) != 1 {
		t.Fatalf("a capped walk must not report a recovery; got %d/%d", len(result.Recovered), len(result.Unrecovered))
	}
	out := result.Unrecovered[0]
	if !out.Capped {
		t.Fatal("a walk that stopped at the cap must set Capped")
	}
	if out.Revisions != 2 {
		t.Fatalf("a capped walk must stop at the cap, searched %d", out.Revisions)
	}
	if !strings.Contains(out.Reason, "still unread") {
		t.Fatalf("the reason must say history was left unread, got %q", out.Reason)
	}
	if strings.Contains(out.Reason, "no revision of this file hashes") {
		t.Fatal("a capped walk must never claim no revision matched")
	}

	// Raise the cap past the history and the same corpus recovers, so the
	// cap is what stopped it and not an unrelated failure.
	MaxRevisionsPerClaim = 50
	result, err = Recover(r.claims(), store, r.dir)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if len(result.Recovered) != 1 {
		t.Fatalf("with the cap above the history the claim must recover; got %+v", result.Unrecovered)
	}
}

// The complexity contract, executed. Recovery's cost must depend on a claim's
// OWN history and on nothing about the corpus around it: no dependency
// traversal, no path enumeration, no term that grows with edges. This asserts
// the structural work directly — revisions read — across a corpus that varies
// both the number of claims and the depth of history.
//
// Bound: for A eligible claims each with R revisions before its approval, the
// walk reads at most sum(min(R_i + 1, cap)) revisions in total, and reads
// NOTHING for a claim that is not eligible. Budget below is that exact sum.
func TestRecoverWorkIsBoundedByEachClaimsOwnHistory(t *testing.T) {
	r := newRepo(t)
	const claims = 6
	// Every claim gains an edge to every other, so the corpus is dense. If
	// any cost here followed edges rather than history, this is where it
	// would show.
	var ids []string
	for i := 0; i < claims; i++ {
		ids = append(ids, fmt.Sprintf("widget.contract.c%02d", i))
	}
	// Every claim rests on every other one, so the corpus is dense.
	edges := func(self string) string {
		out := "rests_on:\n"
		for _, id := range ids {
			if id != self {
				out += "  - " + id + "\n"
			}
		}
		return out
	}

	approved := map[string]model.Claim{}
	for i, id := range ids {
		approved[id] = r.writeID(fmt.Sprintf("claims/c%02d.yaml", i), id, "Approved wording.", edges(id))
	}
	r.commit("approve every claim")

	// Claim i gets i extra rewrites, so depth varies across the corpus and a
	// single per-claim number cannot accidentally satisfy the assertion.
	for round := 0; round < claims; round++ {
		for i := round; i < claims; i++ {
			r.writeID(fmt.Sprintf("claims/c%02d.yaml", i), ids[i], fmt.Sprintf("Rewrite %d.", round), edges(ids[i]))
		}
		r.commit(fmt.Sprintf("rewrite round %d", round))
	}

	// Only the first three are eligible; the rest match their approval, so
	// they must cost zero revisions.
	store := &lock.Store{Ledger: map[string]lock.LedgerRecord{}}
	loaded := r.claims()
	byID := map[string]model.Claim{}
	for _, c := range loaded {
		byID[c.ID] = c
	}
	eligibleIDs := map[string]bool{}
	for i, id := range ids {
		hash := lock.LockedClaimHash(approved[id])
		if i >= 3 {
			// Signed as the claim is NOW, so it is not edited-since-approval.
			hash = lock.LockedClaimHash(byID[id])
		} else {
			eligibleIDs[id] = true
		}
		store.Ledger[id] = lock.LedgerRecord{
			Subject: lock.SubjectClaim, Hash: hash,
			At: "2026-01-01T00:00:00Z", Actor: "approver", Reason: "reviewed",
			ReleasedAt: "2026-02-01T00:00:00Z", ReleasedBy: "maintainer", ReleasedReason: "rewording",
		}
	}

	result, err := Recover(loaded, store, r.dir)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if result.Eligible != len(eligibleIDs) {
		t.Fatalf("eligible %d, want %d", result.Eligible, len(eligibleIDs))
	}
	if len(result.Recovered) != len(eligibleIDs) {
		t.Fatalf("recovered %d of %d eligible; unrecovered %+v",
			len(result.Recovered), len(eligibleIDs), result.Unrecovered)
	}

	total := 0
	for _, out := range append(append([]Outcome{}, result.Recovered...), result.Unrecovered...) {
		if !eligibleIDs[out.ClaimID] {
			t.Fatalf("%s is not eligible and must not have been searched", out.ClaimID)
		}
		total += out.Revisions
		// Claim i was rewritten in rounds 0..i, so its approval is i+1
		// revisions back — that is the ONLY thing its cost may depend on.
		var index int
		if _, err := fmt.Sscanf(out.ClaimID, "widget.contract.c%02d", &index); err != nil {
			t.Fatal(err)
		}
		if want := index + 2; out.Revisions != want {
			t.Fatalf("%s read %d revisions, want %d — its own history depth and nothing else",
				out.ClaimID, out.Revisions, want)
		}
	}
	// 2 + 3 + 4 for claims 0, 1 and 2; the dense edges between six claims
	// contribute nothing.
	if want := 9; total != want {
		t.Fatalf("total revisions read %d, want the %d each claim's own history costs", total, want)
	}
}
