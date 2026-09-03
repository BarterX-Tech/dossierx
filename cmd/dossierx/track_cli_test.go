// track_cli_test.go covers the "dossierx track" noun end to end through the
// real command tree.
//
// Three of the tests here are not about output at all. TestTrackLeavesWriteNothing
// pins the constraint the whole noun rests on (see track.go's package comment),
// TestUnknownTrackIsItsOwnRefusal pins the failure that would otherwise be
// indistinguishable from success, and TestAnEmptyTrackIsNotComplete pins the one
// place the completeness rule is deliberately not a plain reading of itself.
package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
)

// The track payloads are registered here rather than written into
// surface_test.go's literal, because that file is copied into OLDER trees and
// compiled there to re-manufacture the frozen release baseline — a tree that has
// no track noun and therefore no trackListData to name. See
// surfacePayloadTypes, which states the rule; this is the half of it that has to
// live next to the noun.
func init() {
	registerSurfacePayloadType("trackListData", trackListData{})
	registerSurfacePayloadType("trackShowData", trackShowData{})
	registerSurfacePayloadType("trackStatusData", trackStatusData{})
}

// writeTrackFixture builds a project that exercises every shape the noun has to
// report: a track whose owned claim is locked but which cites a claim in ANOTHER
// module that is not, and a second track nothing has joined.
//
// The cross-module citation is the point of the fixture. A track that lived
// inside one module would be a facet, and the completeness question would be a
// module question; what makes the axis worth having is precisely that
// "checkout" is finished only when a payments claim it never owned is locked.
func writeTrackFixture(t *testing.T) (cfgPath, root string) {
	t.Helper()
	root = t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims dir: %v", err)
	}

	cfg := "schema_version: 1\n" +
		"facets:\n  - contract\n" +
		"modules:\n  - checkout\n  - payments\n" +
		"claims_dir: claims\n" +
		"tracks:\n" +
		"  - id: guest-checkout\n    title: Guest Checkout\n    summary: buying without an account\n" +
		"  - id: refunds\n    title: Refunds\n"
	cfgPath = filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write project.config.yaml: %v", err)
	}

	claims := map[string]string{
		// The track's OWN sentence: feature-level, locked, and the only claim
		// whose body belongs in the assembled document.
		"owned.yaml": "id: checkout.contract.guest-flow\n" +
			"facet: contract\nmodule: checkout\nstatus: locked\nlayout: card\n" +
			"body: |\n  a guest completes a purchase without creating an account.\n" +
			"tracks:\n  - id: guest-checkout\n    role: owns\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
		// A citation with an EXPLICIT cites role, already locked.
		"cited-locked.yaml": "id: checkout.contract.session-ttl\n" +
			"facet: contract\nmodule: checkout\nstatus: locked\nlayout: card\n" +
			"body: |\n  a guest session expires after thirty minutes.\n" +
			"tracks:\n  - id: guest-checkout\n    role: cites\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
		// A citation with NO role — the default, which must read as cites — in a
		// different module, and still draft. This is the blocker.
		"cited-draft.yaml": "id: payments.contract.card-capture\n" +
			"facet: contract\nmodule: payments\nstatus: draft\nlayout: card\n" +
			"body: |\n  a card is captured at authorization time.\n" +
			"tracks:\n  - id: guest-checkout\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
		// In no track at all: it must not appear in any track's report.
		"unrelated.yaml": "id: payments.contract.settlement\n" +
			"facet: contract\nmodule: payments\nstatus: draft\nlayout: card\n" +
			"body: |\n  settlement runs nightly.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
	}
	for name, body := range claims {
		if err := os.WriteFile(filepath.Join(claimsDir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return cfgPath, root
}

func TestTrackListReportsEveryDeclaredTrackInDeclarationOrder(t *testing.T) {
	cfgPath, _ := writeTrackFixture(t)

	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "track", "list")
	if err != nil {
		t.Fatalf("track list: %v (%+v)", err, env)
	}
	var data trackListData
	envData(t, env, &data)

	if data.Count != 2 || len(data.Tracks) != 2 {
		t.Fatalf("expected the two declared tracks, got %+v", data)
	}
	// DECLARATION order, not alphabetical: "refunds" sorts before
	// "guest-checkout", so an alphabetical listing would pass a weaker
	// assertion and lose the config's own grouping.
	if data.Tracks[0].TrackID != "guest-checkout" || data.Tracks[1].TrackID != "refunds" {
		t.Fatalf("track list must follow the config's declaration order, got %s then %s",
			data.Tracks[0].TrackID, data.Tracks[1].TrackID)
	}

	first := data.Tracks[0]
	if first.Title != "Guest Checkout" || first.Summary != "buying without an account" {
		t.Fatalf("the declared title and summary must ride the entry, got %+v", first)
	}
	if first.OwnedClaims != 1 || first.CitedClaims != 2 || first.TotalClaims != 3 {
		t.Fatalf("expected 1 owned + 2 cited = 3, got %+v", first)
	}
	if first.LockedClaims != 2 {
		t.Fatalf("expected 2 of the 3 locked, got %+v", first)
	}
	if first.Complete {
		t.Fatalf("a track citing a draft claim is not complete: %+v", first)
	}
}

// TestTrackListOnAProjectWithNoTracksSucceeds is the zero-cost-when-unused
// half of the contract. The second axis is additive, so a corpus that never
// adopted it must behave exactly as it did before the axis existed — an empty
// list at exit 0, never a refusal that a project has to opt out of.
func TestTrackListOnAProjectWithNoTracksSucceeds(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")

	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "track", "list")
	if err != nil {
		t.Fatalf("track list on a track-less project must succeed: %v (%+v)", err, env)
	}
	var data trackListData
	envData(t, env, &data)
	if data.Count != 0 || len(data.Tracks) != 0 {
		t.Fatalf("expected an empty registry, got %+v", data)
	}

	// And the text rendering says WHY it is empty rather than printing a bare
	// count, which reads as a failed search.
	out, _, err := execReviewedCLI(t, "--config", cfgPath, "track", "list")
	if err != nil {
		t.Fatalf("track list --format text: %v", err)
	}
	if !strings.Contains(out, "declares no tracks") {
		t.Fatalf("the empty case must say the project declares no tracks, got %q", out)
	}
}

func TestTrackShowSeparatesOwnedProseFromCitedReferences(t *testing.T) {
	cfgPath, _ := writeTrackFixture(t)

	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "track", "show", "guest-checkout")
	if err != nil {
		t.Fatalf("track show: %v (%+v)", err, env)
	}
	var data trackShowData
	envData(t, env, &data)

	if len(data.OwnedClaims) != 1 || data.OwnedClaims[0].ClaimID != "checkout.contract.guest-flow" {
		t.Fatalf("expected the one owning claim, got %+v", data.OwnedClaims)
	}
	// The owned claim arrives WITH its body: that body is the track's own prose,
	// and a document that omitted it would cost one "claim show" per row to read.
	if !strings.Contains(data.OwnedClaims[0].Body, "without creating an account") {
		t.Fatalf("an owned claim must carry its body — it IS the document: %+v", data.OwnedClaims[0])
	}

	if len(data.CitedClaims) != 2 {
		t.Fatalf("expected two cited claims, got %+v", data.CitedClaims)
	}
	byID := map[string]trackClaimView{}
	for _, c := range data.CitedClaims {
		byID[c.ClaimID] = c
		// A citation is a REFERENCE, never a copy: the claim's own module keeps
		// guaranteeing it, and reproducing its text here would put a second copy
		// of that sentence in a document nothing hashes.
		if c.Body != "" {
			t.Fatalf("a cited claim must not carry a copy of its body: %+v", c)
		}
	}
	// The claim whose membership OMITS a role must read as cites — the default
	// resolved by the engine, not left for a consumer to re-derive.
	draft, ok := byID["payments.contract.card-capture"]
	if !ok {
		t.Fatalf("a membership with no role must default to cites; got %+v", data.CitedClaims)
	}
	// The owning module and facet ride along, because they are how a reader gets
	// from the track's document to the review that can lock the claim.
	if draft.Module != "payments" || draft.Facet != "contract" {
		t.Fatalf("a cited claim must name the module and facet that guarantee it: %+v", draft)
	}
	if draft.Locked || draft.Status != "draft" {
		t.Fatalf("a cited draft claim must report as unlocked: %+v", draft)
	}

	// A claim in no track must not appear anywhere in the document.
	for _, c := range append(append([]trackClaimView(nil), data.OwnedClaims...), data.CitedClaims...) {
		if c.ClaimID == "payments.contract.settlement" {
			t.Fatalf("a claim that joined no track leaked into one: %+v", data)
		}
	}

	if data.Complete || data.TotalClaims != 3 || data.LockedClaims != 2 {
		t.Fatalf("expected an incomplete 3-claim track with 2 locked, got %+v", data)
	}
}

func TestTrackStatusNamesEveryClaimBlockingCompletion(t *testing.T) {
	cfgPath, _ := writeTrackFixture(t)

	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "track", "status", "guest-checkout")
	if err != nil {
		t.Fatalf("track status: %v (%+v)", err, env)
	}
	var data trackStatusData
	envData(t, env, &data)

	if data.Complete {
		t.Fatalf("a track citing a draft claim is not complete: %+v", data)
	}
	if data.Owned != (trackCountsView{Total: 1, Locked: 1}) {
		t.Fatalf("owned counts: %+v", data.Owned)
	}
	if data.Cited != (trackCountsView{Total: 2, Locked: 1}) {
		t.Fatalf("cited counts: %+v", data.Cited)
	}
	if len(data.Blocking) != 1 {
		t.Fatalf("expected exactly the one unlocked claim, got %+v", data.Blocking)
	}
	blocker := data.Blocking[0]
	if blocker.ClaimID != "payments.contract.card-capture" || blocker.Role != "cites" {
		t.Fatalf("the blocker must be named with its role: %+v", blocker)
	}
	// The module is the RECOVERY: a track cannot lock anything, so the only
	// useful answer to "what do I do about this?" names where the claim lives.
	if blocker.Module != "payments" {
		t.Fatalf("a blocker must name its owning module: %+v", blocker)
	}
}

// TestTrackBecomesCompleteWhenTheLastCitedClaimLocks walks the state change the
// whole noun exists to report, through the real lock path rather than by editing
// YAML — so what is asserted is that "complete" tracks the same lock the rest of
// the engine recognizes.
func TestTrackBecomesCompleteWhenTheLastCitedClaimLocks(t *testing.T) {
	cfgPath, _ := writeTrackFixture(t)

	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock",
		"payments.contract.card-capture", "--reason", "the human approved it"); err != nil {
		t.Fatalf("claim lock: %v", err)
	}

	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "track", "status", "guest-checkout")
	if err != nil {
		t.Fatalf("track status: %v (%+v)", err, env)
	}
	var data trackStatusData
	envData(t, env, &data)
	if !data.Complete {
		t.Fatalf("with every owned and cited claim locked the track is complete, got %+v", data)
	}
	if len(data.Blocking) != 0 {
		t.Fatalf("a complete track blocks on nothing, got %+v", data.Blocking)
	}
}

// TestAnEmptyTrackIsNotComplete pins the one place the completeness rule is
// deliberately not a plain reading of itself: vacuously, a track with no claims
// has every claim locked. Answering "yes, done" for a track declared this
// morning and joined by nothing errs in the direction of shipping, so it is
// reported incomplete with the counts beside it saying which case it is.
func TestAnEmptyTrackIsNotComplete(t *testing.T) {
	cfgPath, _ := writeTrackFixture(t)

	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "track", "status", "refunds")
	if err != nil {
		t.Fatalf("track status refunds: %v (%+v)", err, env)
	}
	var data trackStatusData
	envData(t, env, &data)
	if data.Complete {
		t.Fatalf("a track nothing has joined must not read as finished: %+v", data)
	}
	if data.TotalClaims != 0 || len(data.Blocking) != 0 {
		t.Fatalf("the empty case is complete:false with nothing blocking, got %+v", data)
	}

	// The text form has to say it in words, because complete:false with an empty
	// blocking list is the one pair a reader cannot interpret from its shape.
	out, _, err := execReviewedCLI(t, "--config", cfgPath, "track", "status", "refunds")
	if err != nil {
		t.Fatalf("track status --format text: %v", err)
	}
	if !strings.Contains(out, "no claim has joined this track yet") {
		t.Fatalf("the empty case must explain itself, got %q", out)
	}
}

// TestUnknownTrackIsItsOwnRefusal is the failure that would otherwise be
// invisible. A mistyped track id answered with an empty document is
// byte-identical to the answer a real, declared, not-yet-joined track gives —
// and that state is ordinary at the start of a feature, so there is nothing in
// the response for a reader to notice.
func TestUnknownTrackIsItsOwnRefusal(t *testing.T) {
	cfgPath, _ := writeTrackFixture(t)

	for _, leaf := range []string{"show", "status"} {
		t.Run(leaf, func(t *testing.T) {
			env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "track", leaf, "guest-chekout")
			if err == nil || env.OK {
				t.Fatalf("an unknown track id must fail, got %+v", env)
			}
			if env.Error == nil || env.Error.Code != cliout.CodeUnknownTrack {
				t.Fatalf("expected %q, got %+v", cliout.CodeUnknownTrack, env.Error)
			}
			// unknown_module would send an agent to read `modules:`, where a
			// track has never been and never will be. The message names the
			// registry that actually holds the answer.
			if !strings.Contains(env.Error.Message, "guest-checkout") {
				t.Fatalf("the refusal must name the tracks this project declares: %+v", env.Error)
			}
		})
	}

	// A project with NO tracks says so, rather than printing "known: " with
	// nothing after it: naming a track where none are declared is a different
	// mistake from misspelling one of three.
	root := t.TempDir()
	bare, _ := icWriteFixtureProject(t, root, "widget")
	env, _, err := execReviewedCLIJSON(t, "--config", bare, "track", "show", "anything")
	if err == nil || env.Error == nil {
		t.Fatalf("expected a refusal, got %+v", env)
	}
	if !strings.Contains(env.Error.Message, "declares no tracks") {
		t.Fatalf("expected the no-registry wording, got %+v", env.Error)
	}
}

// TestTrackLeavesWriteNothing pins the constraint the entire noun rests on: a
// track is a QUERY over the corpus, and asking whether a feature is finished
// must never change the tree it is asking about. See track.go's package comment
// for why — a track that could write is a track that could gate a lock, which
// inverts the dependency the model rests on.
func TestTrackLeavesWriteNothing(t *testing.T) {
	cfgPath, root := writeTrackFixture(t)

	before := treeSnapshot(t, root)
	for _, args := range [][]string{
		{"track", "list"},
		{"track", "show", "guest-checkout"},
		{"track", "status", "guest-checkout"},
		{"track", "status", "refunds"},
	} {
		if _, _, err := execReviewedCLIJSON(t, append([]string{"--config", cfgPath}, args...)...); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	after := treeSnapshot(t, root)

	if len(before) != len(after) {
		t.Fatalf("a track query changed the file set:\nbefore %v\nafter  %v", before, after)
	}
	for i := range before {
		if before[i] != after[i] {
			t.Fatalf("a track query changed the tree: %q became %q", before[i], after[i])
		}
	}
}

// treeSnapshot is every file under dir as "path\x00content", sorted. It compares
// CONTENT rather than mtimes: a rewrite that happened to reproduce the same
// bytes is not a write anyone can observe, and an mtime comparison would be
// flaky on filesystems with coarse timestamps.
func treeSnapshot(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		out = append(out, rel+"\x00"+string(raw))
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", dir, err)
	}
	sort.Strings(out)
	return out
}
