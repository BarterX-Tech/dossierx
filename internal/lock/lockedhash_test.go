package lock

import (
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// claimFieldDecision is the WRITTEN decision for one persisted field of
// model.Claim: is it signed by LockedClaimHash, and how does a test change it?
type claimFieldDecision struct {
	// hashed is the decision itself: true = LockedClaimHash signs this field,
	// false = it is on the deny-list.
	hashed bool
	// mutate makes a minimal, realistic change to the field, so the test can
	// prove the decision rather than assert it.
	mutate func(*model.Claim)
}

// claimFieldDecisions records an explicit include/exclude decision for EVERY
// yaml-persisted field of model.Claim. It is the human half of the deny-list:
// LockedClaimHash covers new fields automatically (that is the point of a
// deny-list), but "automatically covered" must never mean "nobody looked". A
// field added to the schema without an entry here fails
// TestEveryPersistedClaimFieldHasALedgerDecision, and the fix is to write down
// which side of the line it falls on and why.
//
// Whoever adds one: the question to answer is "if someone changed this field by
// hand on a LOCKED claim, should the ledger gate refuse?" Almost always yes.
// The three exclusions exist only because the ENGINE itself rewrites those
// fields as routine bookkeeping — see lockedClaimHashExcluded's doc comment.
var claimFieldDecisions = map[string]claimFieldDecision{
	"id":                {hashed: true, mutate: func(c *model.Claim) { c.ID = "widget.contract.other" }},
	"facet":             {hashed: true, mutate: func(c *model.Claim) { c.Facet = "internals" }},
	"module":            {hashed: true, mutate: func(c *model.Claim) { c.Module = "gadget" }},
	"layout":            {hashed: true, mutate: func(c *model.Claim) { c.Layout = model.LayoutTable }},
	"kind":              {hashed: true, mutate: func(c *model.Claim) { c.Kind = model.KindOrientationNote }},
	"build_role":        {hashed: true, mutate: func(c *model.Claim) { c.BuildRole = model.BuildRoleAPI }},
	"body":              {hashed: true, mutate: func(c *model.Claim) { c.Body = "a different body" }},
	"rows":              {hashed: true, mutate: func(c *model.Claim) { c.Rows = []model.Row{{"col": "changed"}} }},
	"section":           {hashed: true, mutate: func(c *model.Claim) { c.Section = "9 - elsewhere" }},
	"raw_html":          {hashed: true, mutate: func(c *model.Claim) { c.RawHTML = `<script>alert(1)</script>` }},
	"raw_html_reviewed": {hashed: true, mutate: func(c *model.Claim) { c.RawHTMLReviewed = false }},
	"steps":             {hashed: true, mutate: func(c *model.Claim) { c.Steps = []string{"step one", "step two"} }},
	"mirrors":           {hashed: true, mutate: func(c *model.Claim) { c.Mirrors = []string{"widget.contract.elsewhere"} }},
	"rests_on":          {hashed: true, mutate: func(c *model.Claim) { c.RestsOn = nil }},
	"governed_by":       {hashed: true, mutate: func(c *model.Claim) { c.Governed = model.Governed{Type: "none", Reason: "different reason"} }},
	"migrated_from":     {hashed: true, mutate: func(c *model.Claim) { c.MigratedFrom = "docs/other.html" }},
	"order":             {hashed: true, mutate: func(c *model.Claim) { c.Order = 99 }},
	"emphasis":          {hashed: true, mutate: func(c *model.Claim) { c.Emphasis = false }},
	"audit_notes":       {hashed: true, mutate: func(c *model.Claim) { c.AuditNotes = []string{"a fabricated audit note"} }},

	"status":         {hashed: false, mutate: func(c *model.Claim) { c.Status = model.StatusDraft }},
	"review_pending": {hashed: false, mutate: func(c *model.Claim) { c.ReviewPending = false }},
	"comments": {hashed: false, mutate: func(c *model.Claim) {
		c.Comments = append(c.Comments, model.Comment{ID: "c-999999", Status: model.CommentStatusOpen, Author: model.CommentRoleHuman, Created: "2026-01-01T00:00:00Z", Body: "another thread"})
	}},
}

// fullyPopulatedClaim is a claim with EVERY persisted field set to a non-zero
// value, so each decision's mutate changes a real value to a different real
// value rather than filling in a blank. A test that mutated zero values could
// pass while the hasher silently skipped empty fields.
func fullyPopulatedClaim() model.Claim {
	return model.Claim{
		ID:              "widget.contract.overview",
		Facet:           "contract",
		Module:          "widget",
		Status:          model.StatusLocked,
		Layout:          model.LayoutMockup,
		Kind:            model.KindFact,
		BuildRole:       model.BuildRoleSchema,
		Body:            "the approved body",
		Rows:            []model.Row{{"col": "value", "other": 2}},
		Section:         "1 - orientation",
		RawHTML:         `<div class="mockup">approved markup</div>`,
		RawHTMLReviewed: true,
		Steps:           []string{"step one"},
		Mirrors:         []string{"widget.internals.mirror"},
		RestsOn:         []string{"widget.contract.dep"},
		Governed:        model.Governed{Type: "none", Reason: "a governed reason"},
		MigratedFrom:    "docs/legacy.html",
		Order:           3,
		Emphasis:        true,
		ReviewPending:   true,
		AuditNotes:      []string{"reaudit 2026-01-01: confirmed"},
		Comments: []model.Comment{{
			ID: "c-abc123", Status: model.CommentStatusOpen, Author: model.CommentRoleHuman,
			Created: "2026-01-01T00:00:00Z", Body: "is this still true?",
			Replies: []model.Reply{{ID: "r-abc123", Author: model.CommentRoleAgent, Created: "2026-01-02T00:00:00Z", Body: "checking"}},
		}},
		SourcePath: "/somewhere/claims/overview.yaml",
	}
}

// TestEveryPersistedClaimFieldHasALedgerDecision is the schema tripwire the
// deny-list design depends on. LockedClaimHash signs new model.Claim fields
// automatically, which is the safe default — but a field can be added for which
// signing is WRONG (another engine-managed bookkeeping field, say), and that
// mistake would show up as every locked claim in every project reporting
// content drift the moment the engine wrote it. This test refuses the schema
// change until someone records a decision in claimFieldDecisions.
//
// It walks model.Claim's yaml tags through the same YAMLTagName the hasher
// uses, rather than a hand-rolled tag parser, so the test cannot disagree with
// the code about which fields are persisted.
func TestEveryPersistedClaimFieldHasALedgerDecision(t *testing.T) {
	persisted := persistedClaimTags()

	for _, tag := range persisted {
		if _, ok := claimFieldDecisions[tag]; !ok {
			t.Errorf("model.Claim has a new persisted field %q with no lock-ledger decision.\n"+
				"LockedClaimHash already signs it (it is a deny-list). Confirm that is right, then add an\n"+
				"entry to claimFieldDecisions in this file — and if it must NOT be signed, add it to\n"+
				"lockedClaimHashExcluded in lockedhash.go with a comment saying why.", tag)
		}
	}

	known := make(map[string]bool, len(persisted))
	for _, tag := range persisted {
		known[tag] = true
	}
	for tag := range claimFieldDecisions {
		if !known[tag] {
			t.Errorf("claimFieldDecisions records a decision for %q, which model.Claim no longer persists; remove the stale entry", tag)
		}
	}
}

// TestDenyListMatchesTheRecordedDecisions pins lockedClaimHashExcluded and
// claimFieldDecisions to each other, so the deny-list cannot be widened (or
// quietly emptied) without the written decisions moving with it.
func TestDenyListMatchesTheRecordedDecisions(t *testing.T) {
	for tag, d := range claimFieldDecisions {
		if lockedClaimHashExcluded[tag] == d.hashed {
			t.Errorf("field %q: claimFieldDecisions says hashed=%v but lockedClaimHashExcluded says excluded=%v", tag, d.hashed, lockedClaimHashExcluded[tag])
		}
	}
	if len(lockedClaimHashExcluded) != 3 {
		t.Errorf("the deny-list has %d entries; it is meant to have exactly three (status, review_pending, comments). "+
			"Adding a fourth means the ledger stops signing a field a human approved — say why in lockedhash.go", len(lockedClaimHashExcluded))
	}
}

// TestLockedClaimHashSignsEveryFieldItSaysItSigns proves each recorded decision
// rather than trusting it: mutate one field at a time and check the hash moves
// exactly when the decision says it should.
//
// The ones that matter most, because nothing in the engine signed them before
// this hash existed: raw_html (swapping the payload on a locked, reviewed,
// allowlisted mockup — the only unescaped render path in the engine) and
// build_role/section/order/emphasis. ContentHash covers none of those except
// raw_html, which it took on in v0.4.1 as a STALENESS baseline once raw_html
// became legal on any layout — a different job from certifying that a locked
// claim still holds the bytes a human approved, which is why every one of them
// is still this hash's to sign. See TestLockedClaimHashSeesWhatContentHashCannot
// for the split stated as assertions.
func TestLockedClaimHashSignsEveryFieldItSaysItSigns(t *testing.T) {
	base := fullyPopulatedClaim()
	baseHash := LockedClaimHash(base)

	for tag, decision := range claimFieldDecisions {
		mutated := fullyPopulatedClaim()
		decision.mutate(&mutated)

		if got := LockedClaimHash(mutated); decision.hashed && got == baseHash {
			t.Errorf("changing %q left LockedClaimHash unchanged: an edit to a locked claim's %s would be blessed as unchanged", tag, tag)
		} else if !decision.hashed && got != baseHash {
			t.Errorf("changing %q changed LockedClaimHash: the engine rewrites this field itself, so every routine write would be reported as tampering", tag)
		}
	}
}

// TestLockedClaimHashIgnoresSourcePath proves an unpersisted field is not
// signed. SourcePath is filled in by the loader from wherever the file happens
// to live, so signing it would make moving a claim file — or checking the same
// project out at a different path — read as content drift.
func TestLockedClaimHashIgnoresSourcePath(t *testing.T) {
	a := fullyPopulatedClaim()
	b := fullyPopulatedClaim()
	b.SourcePath = "/a/completely/different/path.yaml"

	if LockedClaimHash(a) != LockedClaimHash(b) {
		t.Fatalf("LockedClaimHash must ignore SourcePath (yaml:\"-\"): it is not persisted, so it is not part of what was approved")
	}
}

// TestLockedClaimHashIsIndependentOfStatus is what lets every write hook hash
// the claim before or after flipping its status and still agree. Lock records
// the hash of the claim it has just flipped to locked; the gate re-computes it
// from the file. If Status were signed, those two would differ by construction
// and every lock would immediately report drift against its own record.
func TestLockedClaimHashIsIndependentOfStatus(t *testing.T) {
	draft := fullyPopulatedClaim()
	draft.Status = model.StatusDraft
	locked := fullyPopulatedClaim()
	locked.Status = model.StatusLocked

	if LockedClaimHash(draft) != LockedClaimHash(locked) {
		t.Fatalf("LockedClaimHash must not depend on Status")
	}
}

// TestLockedClaimHashSeesWhatContentHashCannot is the audit's headline finding,
// as a test. model.Claim persists twenty-two fields; ContentHash covers a small
// allowlist. A ledger built on ContentHash would certify each of the edits below
// as unchanged.
//
// raw_html USED to head this list — it is the one field here that renders
// unescaped, so it was the most dangerous blind spot of the lot. As of v0.4.1 it
// is no longer a blind spot: raw_html became an attachment legal on any layout,
// including a rule-bearing claim other claims rest_on, so ContentHash had to
// start covering it or an edited attachment would leave every dependent
// unflagged. It is asserted separately below, because the assertion it needs is
// now the opposite one. The two hashes still answer different questions and
// LockedClaimHash must still sign it — that half is unchanged.
func TestLockedClaimHashSeesWhatContentHashCannot(t *testing.T) {
	blindSpots := map[string]func(*model.Claim){
		"raw_html_reviewed": func(c *model.Claim) { c.RawHTMLReviewed = false },
		"build_role":        func(c *model.Claim) { c.BuildRole = model.BuildRoleOutOfScope },
		"kind":              func(c *model.Claim) { c.Kind = model.KindOrientationNote },
		"section":           func(c *model.Claim) { c.Section = "somewhere else entirely" },
		"order":             func(c *model.Claim) { c.Order = 1000 },
		"emphasis":          func(c *model.Claim) { c.Emphasis = false },
		"migrated_from":     func(c *model.Claim) { c.MigratedFrom = "somewhere/else.html" },
		"audit_notes":       func(c *model.Claim) { c.AuditNotes = []string{"a note nobody wrote"} },
	}

	base := fullyPopulatedClaim()
	for field, mutate := range blindSpots {
		mutated := fullyPopulatedClaim()
		mutate(&mutated)

		if ContentHash(mutated) != ContentHash(base) {
			t.Errorf("precondition changed: ContentHash now covers %q. That is not automatically wrong, but it flips every "+
				"locked claim in every existing project to review_pending on upgrade — see ContentHash's doc comment", field)
		}
		if LockedClaimHash(mutated) == LockedClaimHash(base) {
			t.Errorf("LockedClaimHash does not cover %q — the ledger would sign off on that edit as unchanged", field)
		}
	}

	// raw_html, the former blind spot: BOTH hashes must move now. The split
	// between them is not that one is coarse and one is fine — it is that they
	// answer different questions, and since v0.4.1 a raw_html swap is a "yes"
	// to both. "Would a dependent need to re-review?" — yes, the attachment can
	// sit on a claim others rest_on. "Are these the bytes a human approved?" —
	// no, and this is still the only path that renders author bytes unescaped.
	// The conditional that keeps raw_html-free claims hashing byte-identically
	// is pinned by TestContentHash_RawHTMLIsHashedOnlyWhenPresent in lock_test.go
	// (and, for a claim carrying none, by TestContentHashIsUnchangedByTheLedger
	// below, whose frozen constant did not move when raw_html joined the list).
	swapped := fullyPopulatedClaim()
	swapped.RawHTML = `<img src=x onerror=alert(1)>`
	if ContentHash(swapped) == ContentHash(base) {
		t.Errorf("ContentHash does not cover \"raw_html\" — since v0.4.1 raw_html is legal on any layout, " +
			"so editing the attachment on a claim others rest_on would leave every dependent unflagged")
	}
	if LockedClaimHash(swapped) == LockedClaimHash(base) {
		t.Errorf("LockedClaimHash does not cover \"raw_html\" — the ledger would sign off on a swapped " +
			"unescaped-HTML payload as unchanged, which is the one edit that most needs a signature")
	}
}

// TestContentHashIsUnchangedByTheLedger guards the other half of the split. The
// dependency-drift baseline in Store.Hashes is ContentHash, recorded per locked
// claim in every existing project; widening it would make every recorded
// baseline mismatch and flip every locked claim to review_pending on upgrade
// day. This pins its exact byte output for a known claim, so a well-meant
// "let's just hash everything" edit to ContentHash fails here with the reason
// attached rather than in a user's project.
func TestContentHashIsUnchangedByTheLedger(t *testing.T) {
	c := model.Claim{
		ID: "widget.contract.overview", Facet: "contract", Module: "widget",
		Status: model.StatusLocked, Layout: model.LayoutCard, Body: "the approved body",
		Steps: []string{"step one"}, Mirrors: []string{"widget.internals.mirror"},
		RestsOn:  []string{"widget.contract.dep"},
		Governed: model.Governed{Type: "none", Reason: "a governed reason"},
	}
	const want = "5c6be2d6bfa41d3bdfb78c7b17dd6c70598999853d110393ccc8e5343b86c502"

	if got := ContentHash(c); got != want {
		t.Fatalf("ContentHash changed.\n got: %s\nwant: %s\n\n"+
			"ContentHash is the dependency-drift baseline recorded in every existing project's lock store.\n"+
			"Changing it flips every locked claim to review_pending on the day they upgrade. If the ledger\n"+
			"needs to cover more fields, that is what LockedClaimHash is for — it is a separate hash for\n"+
			"exactly this reason. If this change really is intended, update `want` deliberately.", got, want)
	}
}

// TestLockedClaimHashIsStableAcrossRuns catches the classic reflection-hashing
// bug: Go randomizes map iteration order, so a hasher that walked model.Row's
// keys in native order would produce a different hash for the same claim on
// (roughly) every other run — and the gate would report drift at random, which
// is worse than no gate at all, because people learn to ignore it.
func TestLockedClaimHashIsStableAcrossRuns(t *testing.T) {
	c := fullyPopulatedClaim()
	c.Rows = []model.Row{{"alpha": 1, "beta": "two", "gamma": true, "delta": nil, "epsilon": 5.5}}

	first := LockedClaimHash(c)
	for i := 0; i < 200; i++ {
		if got := LockedClaimHash(c); got != first {
			t.Fatalf("LockedClaimHash is not deterministic across runs (iteration %d): %s != %s", i, got, first)
		}
	}
}

// TestLockedClaimHashResistsFieldSmuggling proves the length-prefixed encoding
// does its job: without it, a body could be crafted to reproduce the byte
// stream of a different claim's fields, letting an edit hash to what was
// approved. Here the two claims differ only in where the boundary between two
// values falls.
func TestLockedClaimHashResistsFieldSmuggling(t *testing.T) {
	a := model.Claim{ID: "widget.contract.a", Body: "one", Section: "two"}
	b := model.Claim{ID: "widget.contract.a", Body: "onetwo", Section: ""}

	if LockedClaimHash(a) == LockedClaimHash(b) {
		t.Fatalf("two claims with different field boundaries hashed identically: the encoding is ambiguous")
	}
}

// TestLockedClaimHashSeesRowColumnOrder pins that a table claim's authored
// column order is signed. It IS persisted (model.Row.MarshalYAML writes it back
// in the authored order), so it is part of what a human approved — and
// reordering a locked table's columns silently would be exactly the kind of
// "the file changed but nothing noticed" edit the ledger exists to end.
func TestLockedClaimHashSeesRowColumnOrder(t *testing.T) {
	// model.Row's UnmarshalYAML stashes the authored order inside the row
	// itself; a Row built in Go carries none, so this test constructs the two
	// orders the way a YAML decode would.
	const orderKey = "\x00order"
	ab := model.Row{"a": 1, "b": 2, orderKey: []string{"a", "b"}}
	ba := model.Row{"a": 1, "b": 2, orderKey: []string{"b", "a"}}

	first := model.Claim{ID: "widget.contract.t", Rows: []model.Row{ab}}
	second := model.Claim{ID: "widget.contract.t", Rows: []model.Row{ba}}

	if LockedClaimHash(first) == LockedClaimHash(second) {
		t.Fatalf("a locked table's authored column order is persisted but not signed")
	}
}

// persistedClaimTags returns the on-disk name of every persisted field of
// model.Claim, sorted — the same set hashStructFields walks.
func persistedClaimTags() []string {
	return persistedNames(reflect.TypeOf(model.Claim{}))
}

// persistedNames is persistedClaimTags over an arbitrary struct type, so the
// schema tripwire above can itself be tested against shapes model.Claim does not
// currently have (see TestTheSchemaTripwireSeesAnUntaggedField). It asks
// PersistedYAMLName rather than re-reading the tags, so the test and the hasher
// cannot disagree about what is on disk.
func persistedNames(t reflect.Type) []string {
	var names []string
	for i := 0; i < t.NumField(); i++ {
		name, persisted := PersistedYAMLName(t.Field(i))
		if !persisted {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// taglessSchema is the shape the deny-list used to fail OPEN on: an exported
// field with no yaml tag at all. yaml.v3 persists it under its lower-cased Go
// name — it is written to the file and read back from it like any other field —
// while the old hasher, which skipped anything without a tag, left it out of the
// signature entirely. A locked claim carrying such a field could be edited with
// the gate reporting nothing at all, and no ledger edit was needed to do it.
type taglessSchema struct {
	Tagged   string `yaml:"tagged"`
	Untagged string
	Omitted  string `yaml:",omitempty"`
	Excluded string `yaml:"-"`
	//nolint:unused // present precisely to prove unexported fields stay unsigned
	unexported string
}

// TestPersistedYAMLNameAgreesWithYAMLv3 pins the fail-closed rule to the only
// authority that settles it: what yaml.v3 actually writes. A hand-reasoned
// answer here is how the original bug happened — "no tag means not persisted"
// sounds right and is false — so the test marshals the struct and compares the
// keys, rather than restating the rule in a second place.
func TestPersistedYAMLNameAgreesWithYAMLv3(t *testing.T) {
	raw, err := yaml.Marshal(taglessSchema{Tagged: "a", Untagged: "b", Omitted: "c", Excluded: "d"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var onDisk map[string]any
	if err := yaml.Unmarshal(raw, &onDisk); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	written := make([]string, 0, len(onDisk))
	for k := range onDisk {
		written = append(written, k)
	}
	sort.Strings(written)

	got := persistedNames(reflect.TypeOf(taglessSchema{}))
	if strings.Join(got, ",") != strings.Join(written, ",") {
		t.Fatalf("PersistedYAMLName disagrees with yaml.v3 about what is on disk.\n  hasher: %v\n  yaml.v3: %v\n\n"+
			"Anything yaml.v3 writes is part of what a human approved and must be signed; anything it does not\n"+
			"write must not be, or moving a file would read as content drift.", got, written)
	}
}

// TestLockedClaimHashSignsAnUntaggedExportedField is the defect itself, as a
// test: the hash must move when a persisted-but-untagged field changes.
func TestLockedClaimHashSignsAnUntaggedExportedField(t *testing.T) {
	hashOf := func(v taglessSchema) string {
		h := sha256.New()
		hashStructFields(h, reflect.ValueOf(v), lockedClaimHashExcluded)
		return hex.EncodeToString(h.Sum(nil))
	}

	base := taglessSchema{Tagged: "a", Untagged: "b", Omitted: "c", Excluded: "d", unexported: "e"}

	edited := base
	edited.Untagged = "quietly rewritten"
	if hashOf(edited) == hashOf(base) {
		t.Errorf("an exported field with no yaml tag is written to the claim file by yaml.v3 and read back from it, " +
			"but the hash did not move: the deny-list is failing OPEN on the one shape nobody remembered to tag")
	}

	omitted := base
	omitted.Omitted = "also rewritten"
	if hashOf(omitted) == hashOf(base) {
		t.Errorf("`yaml:\",omitempty\"` names no field but still persists it under the lower-cased Go name; it must be signed")
	}

	// The other direction: what yaml.v3 does NOT write must not be signed, or
	// every claim would drift on a source refactor.
	notOnDisk := base
	notOnDisk.Excluded = "not on disk"
	notOnDisk.unexported = "also not on disk"
	if hashOf(notOnDisk) != hashOf(base) {
		t.Errorf("`yaml:\"-\"` and unexported fields are not persisted, so they are not part of what was approved")
	}
}

// TestTheSchemaTripwireSeesAnUntaggedField closes the loop on the fix. The
// tripwire (TestEveryPersistedClaimFieldHasALedgerDecision) is what forces a
// written include/exclude decision for every new field of model.Claim — and it
// walked the yaml TAGS, so the very shape that slipped past the hasher would
// have slipped past the test that exists to catch it too. Both now ask
// PersistedYAMLName, so an untagged field added to the schema fails the build
// with a decision to write down rather than becoming a silent hole.
func TestTheSchemaTripwireSeesAnUntaggedField(t *testing.T) {
	got := persistedNames(reflect.TypeOf(taglessSchema{}))
	for _, want := range []string{"untagged", "omitted"} {
		found := false
		for _, name := range got {
			if name == want {
				found = true
			}
		}
		if !found {
			t.Errorf("the schema tripwire does not see %q as persisted, so an untagged field could be added to model.Claim with no recorded decision; saw %v", want, got)
		}
	}
	for _, never := range []string{"excluded", "unexported"} {
		for _, name := range got {
			if name == never {
				t.Errorf("%q is not persisted by yaml.v3 and must not demand a decision", never)
			}
		}
	}
}
