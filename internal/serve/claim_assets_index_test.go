package serve_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// claim_assets_index_test.go is about ONE sentence, which is the whole safety
// argument for the /claim-assets/ route:
//
//	A path the route answers for is a path the renderer would have emitted,
//	and no other.
//
// claim_assets_test.go tests the route's behaviour given an index. This file
// tests the INDEX ITSELF, from the two directions it can be wrong:
//
//   - TOO WIDE. The index is built from the model, the permission is per-partial,
//     and the two are not the same thing. layout:tree renders Body raw inside a
//     <pre> with no markdown at all; every layout except steps ignores Steps
//     entirely. A claim whose id is not unique is a third way: two claims
//     collapse onto one URL, and whoever indexes first decides which file the
//     other one's page is served.
//   - STALE. The index is invalidated by comparing a fingerprint of the claims
//     tree, and a fingerprint that enumerates FEWER files than loader.LoadClaims
//     does cannot notice a change to one of the files it does not look at. Such a
//     claim is loaded, rendered and indexed, and then its entry outlives it.
//
// Every test below asserts BOTH halves where it can: what the page emits, and
// what the route answers. They are supposed to be the same set.

// claimFile builds a claim with an explicit layout and optional steps, so a
// fixture can put an image on a surface the layout does not render.
func claimFile(id, layout, body string, steps ...string) string {
	s := "id: " + id + "\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: " + layout + "\n"
	if body != "" {
		s += "body: |\n  " + body + "\n"
	}
	if len(steps) > 0 {
		s += "steps:\n"
		for _, st := range steps {
			s += "  - \"" + st + "\"\n"
		}
	}
	return s + "governed_by:\n  type: none\n  reason: fixture\n"
}

// --- the index must not be wider than the render ----------------------------

// TestClaimAsset_TreeLayoutBodyIsNotIndexed is the plainest "too wide" case.
// tree.html emits {{.Body}} inside a <pre> with no markdown call in it, so a
// layout:tree claim's body NEVER produces an <img> — and a route that answered
// for that file would be answering for a path the renderer does not emit, which
// is the one thing the allowlist is supposed to make impossible.
func TestClaimAsset_TreeLayoutBodyIsNotIndexed(t *testing.T) {
	_, base, root := startServer(t, baseConfig, map[string]string{
		"claims/facet-a/tree.yaml": claimFile("widget.contract.treeclaim", "tree", "![never rendered](assets/tree-only.png)"),
	})
	writeBytes(t, filepath.Join(root, "claims", "facet-a", "assets", "tree-only.png"), pngBytes)

	assertPageHasNoImage(t, base)
	assertNotFound(t, base, "/claim-assets/widget.contract.treeclaim/assets/tree-only.png")
}

// TestClaimAsset_StepsOnANonStepsLayoutAreNotIndexed is the same defect on the
// other field. Only steps.html ranges over .Steps; a layout:card claim carrying
// steps drops them silently (dossierx check says so in as many words), so the
// image in one is never emitted and must never be reachable.
func TestClaimAsset_StepsOnANonStepsLayoutAreNotIndexed(t *testing.T) {
	_, base, root := startServer(t, baseConfig, map[string]string{
		"claims/facet-a/card.yaml": claimFile("widget.contract.cardsteps", "card", "",
			"![step image on a card layout](assets/card-steps.png)"),
	})
	writeBytes(t, filepath.Join(root, "claims", "facet-a", "assets", "card-steps.png"), pngBytes)

	assertPageHasNoImage(t, base)
	assertNotFound(t, base, "/claim-assets/widget.contract.cardsteps/assets/card-steps.png")
}

// TestClaimAsset_StepsOnAStepsLayoutAreIndexed is the control for the two above:
// the same Steps entry on the layout that DOES render it is emitted and IS
// served. Without this, "index nothing" would pass the pair.
func TestClaimAsset_StepsOnAStepsLayoutAreIndexed(t *testing.T) {
	_, base, root := startServer(t, baseConfig, map[string]string{
		"claims/facet-a/steps.yaml": claimFile("widget.contract.realsteps", "steps",
			"![body](assets/body.png)", "![step](assets/step.png)"),
	})
	writeBytes(t, filepath.Join(root, "claims", "facet-a", "assets", "body.png"), pngBytes)
	writeBytes(t, filepath.Join(root, "claims", "facet-a", "assets", "step.png"), pngBytes)

	for _, p := range []string{
		"/claim-assets/widget.contract.realsteps/assets/body.png",
		"/claim-assets/widget.contract.realsteps/assets/step.png",
	} {
		page, body := do(t, http.MethodGet, base+"/", "")
		if page.StatusCode != http.StatusOK {
			t.Fatalf("GET /: status %d", page.StatusCode)
		}
		if !strings.Contains(string(body), `src="`+p+`"`) {
			t.Fatalf("the rendered viewer does not emit %s", p)
		}
		if resp, _ := do(t, http.MethodGet, base+p, ""); resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s: status %d, want 200", p, resp.StatusCode)
		}
	}
}

// TestClaimAsset_DuplicateClaimIDServesNeitherClaimsFile is co-location defeated
// without a symlink. Two claims in different directories share an id, so both
// pages emit the IDENTICAL URL — and one directory has to lose. First-wins does
// not merely pick a winner, it points the LOSER'S OWN CARD at the winner's file:
// dup-b's diagram would be dup-a's bytes. There is no answer to that which
// serves anything, so an ambiguous id loses the image capability outright, which
// is the same degradation an unroutable id already gets.
func TestClaimAsset_DuplicateClaimIDServesNeitherClaimsFile(t *testing.T) {
	_, base, root := startServer(t, baseConfig, map[string]string{
		"claims/dup-a/c.yaml": claimFile("widget.contract.dup", "card", "![diagram](assets/d.png)"),
		"claims/dup-b/c.yaml": claimFile("widget.contract.dup", "card", "![diagram](assets/d.png)"),
	})
	writeBytes(t, filepath.Join(root, "claims", "dup-a", "assets", "d.png"), []byte("IMAGE-FROM-DUP-a"))
	writeBytes(t, filepath.Join(root, "claims", "dup-b", "assets", "d.png"), []byte("IMAGE-FROM-DUP-b"))

	resp, data := do(t, http.MethodGet, base+"/claim-assets/widget.contract.dup/assets/d.png", "")
	if resp.StatusCode == http.StatusOK {
		t.Fatalf("an ambiguous id served %q; one of the two claims is being shown another claim's file", data)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status %d, want 404", resp.StatusCode)
	}
}

// TestClaimAsset_DuplicateIDDoesNotPoisonAnUnrelatedClaim pins the blast radius:
// only the duplicated id loses images.
func TestClaimAsset_DuplicateIDDoesNotPoisonAnUnrelatedClaim(t *testing.T) {
	_, base, root := startServer(t, baseConfig, map[string]string{
		"claims/dup-a/c.yaml": claimFile("widget.contract.dup", "card", "![diagram](assets/d.png)"),
		"claims/dup-b/c.yaml": claimFile("widget.contract.dup", "card", "![diagram](assets/d.png)"),
		"claims/ok/one.yaml":  claimFile("widget.contract.one", "card", "![diagram](assets/diagram.png)"),
	})
	writeBytes(t, filepath.Join(root, "claims", "dup-a", "assets", "d.png"), []byte("IMAGE-FROM-DUP-a"))
	writeBytes(t, filepath.Join(root, "claims", "dup-b", "assets", "d.png"), []byte("IMAGE-FROM-DUP-b"))
	writeBytes(t, filepath.Join(root, "claims", "ok", "assets", "diagram.png"), pngBytes)

	resp, data := do(t, http.MethodGet, base+"/claim-assets/widget.contract.one/assets/diagram.png", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the unambiguous claim's image: status %d", resp.StatusCode)
	}
	if string(data) != string(pngBytes) {
		t.Errorf("served %q, want the fixture bytes", data)
	}
}

// --- the index must not outlive the claim -----------------------------------

// TestClaimAsset_AllowlistFollowsADotDirectoryClaim and its .tmp- sibling below
// are the same defect twice, and the defect is a DISAGREEMENT rather than a bug
// in either party: loader.LoadClaims takes any *.yaml/*.yml anywhere under
// claims_dir, while the watcher's fingerprint deliberately skips dot-directories
// and the atomic writer's "*.tmp-*" scratch files (it must, or every SaveClaim
// would flap live-reload). Building the ALLOWLIST'S freshness check on the
// narrower scan means such a claim is loaded, rendered, indexed — and then
// editing or deleting it changes nothing the index looks at, so the entry
// outlives the claim indefinitely.
// These three are the tests that keep the ALLOWLIST'S freshness signal bound to
// scanLoadedClaimFingerprint. The freshness CHECK is amortised over one watcher
// tick, but it is still that scan — wiring invalidation to the watcher's own
// onChange instead would make these three fail permanently, because the watcher
// cannot see either claim shape below. They wait for a bounded window (see
// stalenessBound) and fail only if the entry never clears.
func TestClaimAsset_AllowlistFollowsADotDirectoryClaim(t *testing.T) {
	_, base, root := startServerWatch(t, baseConfig, map[string]string{
		"claims/.archive/hidden.yaml": claimFile("widget.contract.hidden", "card", "![hidden](assets/hidden.png)"),
	}, fastPoll, fastDebounce)
	writeBytes(t, filepath.Join(root, "claims", ".archive", "assets", "hidden.png"), pngBytes)

	const p = "/claim-assets/widget.contract.hidden/assets/hidden.png"
	if resp, _ := do(t, http.MethodGet, base+p, ""); resp.StatusCode != http.StatusOK {
		t.Fatalf("precondition: the claim IS loaded and rendered, so its image must be served: status %d", resp.StatusCode)
	}

	if err := os.Remove(filepath.Join(root, "claims", ".archive", "hidden.yaml")); err != nil {
		t.Fatalf("remove the claim: %v", err)
	}
	assertNotFoundWithin(t, base, p, stalenessBound)
}

// TestClaimAsset_AllowlistFollowsATmpNamedClaim is the second half: an ordinary
// claim whose FILENAME happens to contain ".tmp-". It is a real claim to the
// loader — the scratch files the watcher means to skip are "<name>.yaml.tmp-
// <rand>", which do not end in .yaml at all — so nothing about it is transient
// except the fingerprint's opinion of it.
func TestClaimAsset_AllowlistFollowsATmpNamedClaim(t *testing.T) {
	_, base, root := startServerWatch(t, baseConfig, map[string]string{
		"claims/facet-g/retry.tmp-policy.yaml": claimFile("widget.contract.tmpname", "card", "![t](assets/tmpname.png)"),
	}, fastPoll, fastDebounce)
	writeBytes(t, filepath.Join(root, "claims", "facet-g", "assets", "tmpname.png"), pngBytes)

	const p = "/claim-assets/widget.contract.tmpname/assets/tmpname.png"
	if resp, _ := do(t, http.MethodGet, base+p, ""); resp.StatusCode != http.StatusOK {
		t.Fatalf("precondition: status %d", resp.StatusCode)
	}

	// An EDIT that drops the reference must take effect within the same window
	// as it does for a claim in an ordinary directory — this claim is invisible
	// to the watcher, so nothing but the allowlist's own scan can clear it.
	writeFile(t, filepath.Join(root, "claims", "facet-g", "retry.tmp-policy.yaml"),
		claimFile("widget.contract.tmpname", "card", "no image in this body any more."))
	assertNotFoundWithin(t, base, p, stalenessBound)
}

// TestClaimAsset_AllowlistFollowsADotDirectoryClaimEdit covers the edit (rather
// than delete) path for the dot-directory case, since an edit is the change a
// reviewer actually makes.
func TestClaimAsset_AllowlistFollowsADotDirectoryClaimEdit(t *testing.T) {
	_, base, root := startServerWatch(t, baseConfig, map[string]string{
		"claims/.archive/hidden.yaml": claimFile("widget.contract.hidden", "card", "![hidden](assets/hidden.png)"),
	}, fastPoll, fastDebounce)
	writeBytes(t, filepath.Join(root, "claims", ".archive", "assets", "hidden.png"), pngBytes)

	const p = "/claim-assets/widget.contract.hidden/assets/hidden.png"
	if resp, _ := do(t, http.MethodGet, base+p, ""); resp.StatusCode != http.StatusOK {
		t.Fatalf("precondition: status %d", resp.StatusCode)
	}
	writeFile(t, filepath.Join(root, "claims", ".archive", "hidden.yaml"),
		claimFile("widget.contract.hidden", "card", "the diagram was removed from this claim."))
	assertNotFoundWithin(t, base, p, stalenessBound)
}

// --- the substitution defect, end to end ------------------------------------

// TestClaimAsset_SrcWithASpaceServesNothing is markdown's refuse-never-rewrite
// property observed from the outside. The claim names "assets/team photo.png";
// beside it sit BOTH that file and a differently-named one. A gate that
// normalised the src instead of refusing it would emit, index and serve the
// second — a file the author never wrote down.
func TestClaimAsset_SrcWithASpaceServesNothing(t *testing.T) {
	_, base, root := startServer(t, baseConfig, map[string]string{
		"claims/facet-h/space.yaml": claimFile("widget.contract.spacename", "card", "![the file I meant](assets/team photo.png)"),
	})
	writeBytes(t, filepath.Join(root, "claims", "facet-h", "assets", "team photo.png"), []byte("THE-FILE-THE-AUTHOR-NAMED"))
	writeBytes(t, filepath.Join(root, "claims", "facet-h", "assets", "teamphoto.png"), []byte("A-DIFFERENT-FILE-NOBODY-NAMED"))

	assertPageHasNoImage(t, base)
	assertNotFound(t, base, "/claim-assets/widget.contract.spacename/assets/teamphoto.png")
	assertNotFound(t, base, "/claim-assets/widget.contract.spacename/assets/team%20photo.png")
}

// assertPageHasNoImage fails when GET / carries a claim-body image tag at all.
// It is the "what the page emits" half every too-wide test above pairs with a
// 404 assertion.
func assertPageHasNoImage(t *testing.T, base string) {
	t.Helper()
	resp, body := do(t, http.MethodGet, base+"/", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /: status %d", resp.StatusCode)
	}
	if strings.Contains(string(body), `<img class="md-img"`) {
		t.Fatalf("the rendered viewer emitted a claim-body image it was not supposed to")
	}
}
