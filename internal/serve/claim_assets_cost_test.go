package serve_test

import (
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// claim_assets_cost_test.go pins the COST SHAPE of the /claim-assets/ route,
// which is a correctness property here rather than a performance nicety.
//
// Answering an image request means deciding whether the allowlist still
// describes the claims on disk, and deciding that means stat-walking every claim
// file — O(claims), paid identically by a hit and by a miss. A rendered page
// fires one asset GET per image. So a freshness walk PER REQUEST makes one page
// view cost images x claims, and that is not a constant factor: measured on a
// synthetic corpus, a hit and a miss each cost about what a bare walk cost
// (~20ms at 4800 claims), one twenty-image page view cost 412ms, and 400
// concurrent misses took 2.5s because each request walked the tree outside the
// index lock. Every 404 probe paid the same walk, so misses alone could pin the
// server.
//
// The fix is amortisation: the freshness check is memoised for one watcher tick,
// so a page view pays at most one walk regardless of how many images it has.
// These tests assert that shape directly — by counting walks, not by timing
// anything, because a timing assertion on a shared CI box is a flake generator
// and a walk count is exactly the thing that regressed.
//
// The freshness the amortisation trades away is asserted next door: the four
// "allowlist follows the claim" tests in claim_assets_test.go and
// claim_assets_index_test.go prove the entry still goes away on its own.

// longWatch is a poll interval that will not elapse during a test, so the
// allowlist's freshness window (defined as one watcher tick) covers the whole
// test body and every scan the counter sees is one a request asked for.
const longWatch = 30 * time.Second

// TestClaimAsset_ManyRequestsShareOneTreeScan is the regression test for the
// defect. N requests against an unchanged tree must perform a bounded number of
// tree scans, not N.
func TestClaimAsset_ManyRequestsShareOneTreeScan(t *testing.T) {
	srv, base, root := startServerWatch(t, baseConfig, map[string]string{
		"claims/facet-a/one.yaml": imageClaim("widget.contract.one", "assets/diagram.png"),
	}, longWatch, longWatch)
	writeBytes(t, filepath.Join(root, "claims", "facet-a", "assets", "diagram.png"), pngBytes)

	const (
		hit  = "/claim-assets/widget.contract.one/assets/diagram.png"
		miss = "/claim-assets/widget.contract.nosuch/assets/diagram.png"
	)

	// One request to build the index, so what follows is the steady state a
	// reviewer's browser actually produces rather than cold-start cost.
	if resp, _ := do(t, http.MethodGet, base+hit, ""); resp.StatusCode != http.StatusOK {
		t.Fatalf("precondition: status %d", resp.StatusCode)
	}
	before := srv.AssetTreeScans()

	// A page of twenty images, plus twenty misses. A miss must be as cheap as a
	// hit: if only hits were amortised, a 404 probe would still walk the tree
	// and a local caller could pin the server with misses alone.
	const n = 40
	for i := 0; i < n/2; i++ {
		if resp, _ := do(t, http.MethodGet, base+hit, ""); resp.StatusCode != http.StatusOK {
			t.Fatalf("hit %d: status %d", i, resp.StatusCode)
		}
		if resp, _ := do(t, http.MethodGet, base+miss, ""); resp.StatusCode != http.StatusNotFound {
			t.Fatalf("miss %d: status %d", i, resp.StatusCode)
		}
	}

	scans := srv.AssetTreeScans() - before
	if scans > 1 {
		t.Errorf("%d asset requests against an unchanged tree performed %d tree scans; the freshness walk is per-request again", n, scans)
	}
}

// TestClaimAsset_ConcurrentFirstRequestsShareOneTreeScan is the other half, and
// the one the concurrency numbers came from: the walk must be single-flighted,
// not merely cached. Before the fix every concurrent request walked the tree on
// its own, outside the index lock, and the walks contended super-additively —
// throughput fell from 2081 to 160 req/s as the corpus grew from 300 to 4800
// claims. These requests all arrive before any index exists, which is exactly
// the burst a browser produces on the first page load.
func TestClaimAsset_ConcurrentFirstRequestsShareOneTreeScan(t *testing.T) {
	srv, base, root := startServerWatch(t, baseConfig, map[string]string{
		"claims/facet-a/one.yaml": imageClaim("widget.contract.one", "assets/diagram.png"),
	}, longWatch, longWatch)
	writeBytes(t, filepath.Join(root, "claims", "facet-a", "assets", "diagram.png"), pngBytes)

	const n = 60
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			do(t, http.MethodGet, base+"/claim-assets/widget.contract.one/assets/diagram.png", "")
		}()
	}
	wg.Wait()

	if scans := srv.AssetTreeScans(); scans > 1 {
		t.Errorf("%d concurrent asset requests performed %d tree scans; the walk is not single-flighted", n, scans)
	}
}

// TestClaimAsset_TheScanIsAMemoNotAPermanentCache is the guard on the other
// side of the trade. The cheapest way to make the two tests above pass is to
// scan once and never again, which would make an allowlist entry immortal — the
// exact defect the "allowlist follows the claim" tests exist for. This asserts
// the counter keeps moving as ticks elapse, so the amortisation cannot be
// "fixed" into a permanent cache without something failing here first.
func TestClaimAsset_TheScanIsAMemoNotAPermanentCache(t *testing.T) {
	srv, base, root := startServerWatch(t, baseConfig, map[string]string{
		"claims/facet-a/one.yaml": imageClaim("widget.contract.one", "assets/diagram.png"),
	}, fastPoll, fastDebounce)
	writeBytes(t, filepath.Join(root, "claims", "facet-a", "assets", "diagram.png"), pngBytes)

	const p = "/claim-assets/widget.contract.one/assets/diagram.png"
	do(t, http.MethodGet, base+p, "")
	before := srv.AssetTreeScans()

	deadline := time.Now().Add(stalenessBound)
	for srv.AssetTreeScans() <= before {
		if time.Now().After(deadline) {
			t.Fatalf("the freshness scan never ran again in %s; the index would outlive its claims", stalenessBound)
		}
		time.Sleep(fastPoll)
		do(t, http.MethodGet, base+p, "")
	}
}

// TestClaimAsset_APartialWriteKeepsThePreviousIndex covers the failure mode a
// per-request rebuild made routine. An agent rewriting a claim publishes a
// partial YAML file for a few milliseconds; a rebuild that catches it gets a
// LoadClaims error, and installing an EMPTY index on that error 404s every image
// on the page until the next fingerprint delta — which, under sustained writes,
// is most of the time. A 404 is not retried by a browser, so the reviewer keeps
// looking at broken images for a state that no longer exists.
//
// A failed re-verification must leave the allowlist exactly as it was: it cannot
// widen it (the previous index was computed from claims that really were on
// disk) and it must not narrow it to nothing.
func TestClaimAsset_APartialWriteKeepsThePreviousIndex(t *testing.T) {
	srv, base, root := startServerWatch(t, baseConfig, map[string]string{
		"claims/facet-a/one.yaml": imageClaim("widget.contract.one", "assets/diagram.png"),
	}, fastPoll, fastDebounce)
	writeBytes(t, filepath.Join(root, "claims", "facet-a", "assets", "diagram.png"), pngBytes)

	const p = "/claim-assets/widget.contract.one/assets/diagram.png"
	if resp, _ := do(t, http.MethodGet, base+p, ""); resp.StatusCode != http.StatusOK {
		t.Fatalf("precondition: status %d", resp.StatusCode)
	}

	// A claim file caught mid-rewrite: still a *.yaml the walk stamps, no longer
	// something the loader can parse. The truncation lands inside a flow
	// sequence, which is a genuine parse error — a truncated block scalar is
	// not, since YAML closes it at EOF.
	claim := filepath.Join(root, "claims", "facet-a", "one.yaml")
	writeFile(t, claim, "id: widget.contract.one\nfacet: contract\nmodule: widget\nsteps: [\"a\", \"b\"")

	// Wait until a rebuild has actually been attempted against the broken file,
	// so this is not merely asserting that the memo had not expired yet.
	scansBefore := srv.AssetTreeScans()
	deadline := time.Now().Add(stalenessBound)
	for srv.AssetTreeScans() <= scansBefore+1 {
		if time.Now().After(deadline) {
			t.Fatalf("no rebuild was attempted while the claim was unparseable")
		}
		time.Sleep(fastPoll)
		do(t, http.MethodGet, base+p, "")
	}

	if resp, _ := do(t, http.MethodGet, base+p, ""); resp.StatusCode != http.StatusOK {
		t.Errorf("a claim caught mid-rewrite made the image 404 (status %d); a failed re-verification must keep the previous allowlist, not install an empty one", resp.StatusCode)
	}

	// And it must still SELF-HEAL: once the write completes with the reference
	// gone, the entry goes away like any other edit.
	writeFile(t, claim,
		"id: widget.contract.one\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n"+
			"body: |\n  the diagram was removed from this claim.\n"+
			"governed_by:\n  type: none\n  reason: fixture\n")
	assertNotFoundWithin(t, base, p, stalenessBound)
}

// TestClaimAsset_AKeptIndexStopsAuthorisingAfterTheGrace is the other side of
// that keep, and the regression test for what keeping without a bound cost.
//
// THE MEMO WINDOW BOUNDS HOW OFTEN THE CHECK IS ATTEMPTED, NOT HOW LONG A STALE
// ENTRY SURVIVES. The keep above exists for a transient — a claim file caught
// mid-rewrite, gone within a tick. The failure it actually generalises to is not
// transient at all: buildAssetIndex fails on ANY loader.LoadClaims error, the
// loader decodes with KnownFields(true), so ONE unparseable *.yaml anywhere
// under claims_dir makes every check from then on fail. Keeping the previous
// allowlist unconditionally across that meant the check ran every window, failed
// every window, and the allowlist never changed: measured on a real server at
// production cadence, an image no claim referenced any more kept answering 200
// with its bytes for 386 seconds and counting, and 404'd 0.5s after the
// unparseable sibling was removed.
//
// So the index carries a second clock — when it was last CONFIRMED, not when a
// check was last attempted — and past assetKeepGrace a failed check installs an
// empty index instead of the previous one. This test asserts both halves: the
// keep still covers the transient (the image is still served while a rebuild has
// demonstrably been tried and failed), and it expires (the image goes away while
// the tree still does not parse).
func TestClaimAsset_AKeptIndexStopsAuthorisingAfterTheGrace(t *testing.T) {
	srv, base, root := startServerWatch(t, baseConfig, map[string]string{
		"claims/facet-a/one.yaml": twoImageClaim("widget.contract.one", "assets/keep.png", "assets/drop.png"),
	}, fastPoll, fastDebounce)
	writeBytes(t, filepath.Join(root, "claims", "facet-a", "assets", "keep.png"), pngBytes)
	writeBytes(t, filepath.Join(root, "claims", "facet-a", "assets", "drop.png"), pngBytes)

	const (
		keep = "/claim-assets/widget.contract.one/assets/keep.png"
		drop = "/claim-assets/widget.contract.one/assets/drop.png"
	)
	if resp, _ := do(t, http.MethodGet, base+drop, ""); resp.StatusCode != http.StatusOK {
		t.Fatalf("precondition: status %d", resp.StatusCode)
	}

	// A stray unparseable YAML somewhere in the tree — NOT the claim under test,
	// because the point is that any file the loader reads can do this and that
	// nothing about it is transient. An unknown field is a hard KnownFields
	// error, which is what a non-claim YAML dropped into claims/ looks like.
	stray := filepath.Join(root, "claims", "facet-a", "stray.yaml")
	writeFile(t, stray, "id: widget.contract.stray\nfacet: contract\nmodule: widget\nstatus: draft\n"+
		"body: x\nnot_a_real_field: true\ngoverned_by:\n  type: none\n  reason: fixture\n")
	// And the claim stops referencing drop.png, so the allowlist is now wrong.
	writeFile(t, filepath.Join(root, "claims", "facet-a", "one.yaml"),
		twoImageClaim("widget.contract.one", "assets/keep.png"))

	// First: the keep still works. Wait until a rebuild has actually been
	// attempted and failed, so this is not merely the memo not having expired.
	scansBefore := srv.AssetTreeScans()
	deadline := time.Now().Add(stalenessBound)
	for srv.AssetTreeScans() <= scansBefore+1 {
		if time.Now().After(deadline) {
			t.Fatalf("no rebuild was attempted while the tree was unparseable")
		}
		time.Sleep(fastPoll)
		do(t, http.MethodGet, base+drop, "")
	}
	if resp, _ := do(t, http.MethodGet, base+drop, ""); resp.StatusCode != http.StatusOK {
		t.Errorf("the previous allowlist was dropped immediately (status %d); the keep must still cover a claim caught mid-rewrite", resp.StatusCode)
	}

	// Then: it expires. The tree still does not parse, nothing has confirmed the
	// index in longer than the grace, so the route must stop answering — for the
	// de-referenced file AND for the one still referenced, because an index
	// nothing has vouched for authorises nothing.
	assertNotFoundWithin(t, base, drop, keepGraceBound)
	assertNotFoundWithin(t, base, keep, keepGraceBound)

	// And failing closed is not permanent: remove the unparseable file and the
	// next successful build re-authorises exactly what the claims now say.
	if err := os.Remove(stray); err != nil {
		t.Fatalf("remove the stray file: %v", err)
	}
	assertOKWithin(t, base, keep, stalenessBound)
	assertNotFound(t, base, drop)
}

// keepGraceBound is how long these tests wait for an UNCONFIRMED allowlist to
// stop authorising. The bound in the code is assetKeepGrace measured from the
// last confirmation plus the poll interval it takes to notice; this is looser
// on purpose, for the same reason stalenessBound is — it fails only if the
// entry never goes away, which is the defect, not if the timing drifts.
const keepGraceBound = 8 * time.Second

// twoImageClaim is a card whose body references each src on its own line.
func twoImageClaim(id string, srcs ...string) string {
	s := "id: " + id + "\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\nbody: |\n"
	for _, src := range srcs {
		s += "  ![d](" + src + ")\n\n"
	}
	return s + "governed_by:\n  type: none\n  reason: fixture\n"
}

// assertOKWithin polls until path is served, for assertions about a route that
// becomes reachable again rather than one that becomes unreachable.
func assertOKWithin(t *testing.T, base, path string, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for {
		resp, _ := do(t, http.MethodGet, base+path, "")
		if resp.StatusCode == http.StatusOK {
			return
		}
		if time.Now().After(deadline) {
			t.Errorf("GET %s: still %d after %s; the allowlist never recovered once the tree parsed again", path, resp.StatusCode, within)
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// TestClaimAsset_AnUnreadableTreeKeepsThePreviousIndex is the same rule for the
// other re-verification failure: the walk itself erroring (a directory
// mid-rename, a permission change) is not evidence that the claims changed.
//
// The unreadable directory is a DIFFERENT one from the claim under test, so what
// this measures is the freshness walk failing rather than the image file
// becoming unopenable — the latter is an honest 404 and is covered by
// TestClaimAsset_MissingFileIs404.
func TestClaimAsset_AnUnreadableTreeKeepsThePreviousIndex(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: an unreadable directory is still readable")
	}
	srv, base, root := startServerWatch(t, baseConfig, map[string]string{
		"claims/facet-a/one.yaml": imageClaim("widget.contract.one", "assets/diagram.png"),
		"claims/facet-z/two.yaml": imageClaim("widget.contract.two", "assets/other.png"),
	}, fastPoll, fastDebounce)
	writeBytes(t, filepath.Join(root, "claims", "facet-a", "assets", "diagram.png"), pngBytes)
	writeBytes(t, filepath.Join(root, "claims", "facet-z", "assets", "other.png"), pngBytes)

	const p = "/claim-assets/widget.contract.one/assets/diagram.png"
	if resp, _ := do(t, http.MethodGet, base+p, ""); resp.StatusCode != http.StatusOK {
		t.Fatalf("precondition: status %d", resp.StatusCode)
	}

	dir := filepath.Join(root, "claims", "facet-z")
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Skipf("cannot make the directory unreadable: %v", err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) }) //nolint:errcheck // best-effort restore for TempDir cleanup

	scansBefore := srv.AssetTreeScans()
	deadline := time.Now().Add(stalenessBound)
	for srv.AssetTreeScans() <= scansBefore+1 {
		if time.Now().After(deadline) {
			t.Fatalf("no freshness scan was attempted while the tree was unreadable")
		}
		time.Sleep(fastPoll)
		do(t, http.MethodGet, base+p, "")
	}

	if resp, _ := do(t, http.MethodGet, base+p, ""); resp.StatusCode != http.StatusOK {
		t.Errorf("an unreadable claims tree made the image 404 (status %d); a failed freshness check must keep the previous allowlist", resp.StatusCode)
	}
}
