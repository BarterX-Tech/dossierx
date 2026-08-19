package serve_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// claim_assets_test.go covers the one route in this server that reads a file off
// disk and sends its bytes back, and it is written from the position that such a
// route is guilty until proven innocent.
//
// The route is safe because of a specific structural fact, not because of a
// sequence of checks: the set of paths it will answer for is DERIVED FROM THE
// LOADED CLAIMS. Every legal image is an assets/ file referenced by a claim body
// the engine already rendered, so the allowlist can be computed rather than
// discovered, and nothing on disk that no claim points at is reachable at all.
// The independent path defence below that is belt and braces, and the tests
// treat it that way: they assert BOTH that a hostile path is refused and that a
// legitimate file which nothing references is refused too.

// pngBytes is a minimal valid PNG signature plus a byte, so a served file is
// recognisably itself in an assertion.
var pngBytes = []byte("\x89PNG\r\n\x1a\n-fixture")

// imageClaim is a draft card whose body references one co-located image.
func imageClaim(id, src string) string {
	return "id: " + id + "\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
		"body: |\n  ![A diagram](" + src + ")\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
}

// assetProject is the standard fixture for this file: one claim in
// claims/facet-a/ referencing assets/diagram.png beside it, plus a second file
// in the same assets directory that NO claim references, plus a file outside the
// claims tree entirely.
func assetProject(t *testing.T) (base, root string) {
	t.Helper()
	return assetProjectWatch(t, 0, 0)
}

// assetProjectWatch is assetProject with the watcher cadence exposed, for the
// staleness tests: the allowlist's freshness window is one poll interval, so a
// test that changes a claim and waits for the change to take effect runs the
// watcher fast rather than waiting out a real half-second tick.
func assetProjectWatch(t *testing.T, poll, debounce time.Duration) (base, root string) {
	t.Helper()
	_, base, root = startServerWatch(t, baseConfig, map[string]string{
		"claims/facet-a/one.yaml": imageClaim("widget.contract.one", "assets/diagram.png"),
		"claims/facet-b/two.yaml": imageClaim("widget.contract.two", "assets/other.svg"),
	}, poll, debounce)
	writeBytes(t, filepath.Join(root, "claims", "facet-a", "assets", "diagram.png"), pngBytes)
	writeBytes(t, filepath.Join(root, "claims", "facet-a", "assets", "unreferenced.png"), []byte("nobody points at me"))
	writeBytes(t, filepath.Join(root, "claims", "facet-b", "assets", "other.svg"), []byte("<svg/>"))
	writeBytes(t, filepath.Join(root, "secret.png"), []byte("outside the claims tree"))
	return base, root
}

func writeBytes(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// removeWatchedDir removes a DIRECTORY that sits inside the tree the running
// server's watcher is polling.
//
// WHY THIS IS NOT A PLAIN os.RemoveAll, and the reason is a platform difference
// rather than a flake. The watcher is a zero-dependency mtime poll: every
// defaultPollInterval it re-walks the claims tree with filepath.WalkDir, which
// opens a handle on each directory it enumerates. POSIX unlinks a directory out
// from under an open handle without comment, so on macOS and Linux the removal
// below has never once failed. Windows refuses it — "The process cannot access
// the file because it is being used by another process" — whenever the poll
// happens to be inside this directory at the moment the test asks for it, and on
// a half-second tick that is often enough to red the job. It reds the whole
// windows-latest matrix leg, which then reds CI on main, which then refuses the
// release: `make ci-evidence` fails on any failed test, by design.
//
// The race is between the test and the server the test itself started. Nothing
// about it touches the defence under test, which is that a symlinked assets
// directory is not served.
//
// IT RETRIES, AND THEN IT FAILS — it never gives up quietly and it never skips.
// The removal is a PRECONDITION of the assertion that follows it: if the fixture
// directory is still standing, the symlink is never created, and the
// assertNotFound below would pass against an ordinary missing file rather than
// against the link it is supposed to refuse. A silent return here would convert
// this test into one that cannot fail, which is the same defect #28 and #29 were
// about.
func removeWatchedDir(t *testing.T, path string) {
	t.Helper()
	const window = 10 * time.Second
	deadline := time.Now().Add(window)
	for {
		err := os.RemoveAll(path)
		if err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("remove fixture dir %s: still refused after %s: %v\n"+
				"On Windows this is the watcher's poll holding a handle on the directory. "+
				"If it never clears inside the window, the poll is not the cause and this is a real failure.",
				path, window, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// --- the happy path ---------------------------------------------------------

// TestClaimAsset_ServesAReferencedImage is the feature working end to end: the
// URL under test is the one GET / actually emits, not one the test invented.
func TestClaimAsset_ServesAReferencedImage(t *testing.T) {
	base, _ := assetProject(t)

	// The page must reference it, or the rest of this file is testing a route
	// nothing uses.
	page, body := do(t, http.MethodGet, base+"/", "")
	if page.StatusCode != http.StatusOK {
		t.Fatalf("GET /: status %d", page.StatusCode)
	}
	const wantSrc = `src="/claim-assets/widget.contract.one/assets/diagram.png"`
	if !strings.Contains(string(body), wantSrc) {
		t.Fatalf("the rendered viewer does not carry %s", wantSrc)
	}

	resp, data := do(t, http.MethodGet, base+"/claim-assets/widget.contract.one/assets/diagram.png", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET the image: status %d, body %q", resp.StatusCode, data)
	}
	if string(data) != string(pngBytes) {
		t.Errorf("served %q, want the fixture bytes", data)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", ct)
	}
	if n := resp.Header.Get("X-Content-Type-Options"); n != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", n)
	}
}

// TestClaimAsset_ContentTypeIsFromTheClosedExtensionSet pins that the type is
// chosen by a closed switch over the six legal extensions and never sniffed from
// the file's contents — an SVG is the case that matters, since it is a document
// a browser will execute script in if it is handed one under the wrong type.
func TestClaimAsset_ContentTypeIsFromTheClosedExtensionSet(t *testing.T) {
	base, _ := assetProject(t)
	resp, _ := do(t, http.MethodGet, base+"/claim-assets/widget.contract.two/assets/other.svg", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/svg+xml" {
		t.Errorf("Content-Type = %q, want image/svg+xml", ct)
	}
}

// --- the allowlist is the control ------------------------------------------

// TestClaimAsset_UnreferencedFileIsNotServed is the test that says the route is
// an allowlist rather than a file server. unreferenced.png exists, sits in a
// legal assets/ directory beside a real claim, has a legal extension, and is
// inside claims_dir — every path check passes. No claim references it, so it is
// not reachable.
func TestClaimAsset_UnreferencedFileIsNotServed(t *testing.T) {
	base, root := assetProject(t)
	if _, err := os.Stat(filepath.Join(root, "claims", "facet-a", "assets", "unreferenced.png")); err != nil {
		t.Fatalf("fixture missing: %v", err)
	}
	assertNotFound(t, base, "/claim-assets/widget.contract.one/assets/unreferenced.png")
}

// TestClaimAsset_CrossClaimReferenceIsNotServed pins co-location at the ROUTE,
// not just in the renderer: claim one may not serve claim two's image even
// though both are legal images of legal claims.
func TestClaimAsset_CrossClaimReferenceIsNotServed(t *testing.T) {
	base, _ := assetProject(t)
	assertNotFound(t, base, "/claim-assets/widget.contract.one/assets/other.svg")
}

// TestClaimAsset_UnknownClaimIs404 covers the other half of the key.
func TestClaimAsset_UnknownClaimIs404(t *testing.T) {
	base, _ := assetProject(t)
	assertNotFound(t, base, "/claim-assets/widget.contract.nosuch/assets/diagram.png")
	assertNotFound(t, base, "/claim-assets/")
	assertNotFound(t, base, "/claim-assets/widget.contract.one/")
}

// --- path defence, independent of the allowlist -----------------------------

// TestClaimAsset_TraversalIsRefused walks the traversal shapes. None of these
// can reach the allowlist (no claim emits them), so what this really asserts is
// that the refusal is a 404 with no detail in it — a 403 that distinguished
// "exists but forbidden" from "does not exist" would be an existence oracle over
// the whole filesystem.
func TestClaimAsset_TraversalIsRefused(t *testing.T) {
	base, _ := assetProject(t)
	for _, p := range []string{
		"/claim-assets/widget.contract.one/assets/../../../secret.png",
		"/claim-assets/widget.contract.one/assets/%2e%2e/%2e%2e/%2e%2e/secret.png",
		"/claim-assets/widget.contract.one/assets/..%2f..%2f..%2fsecret.png",
		"/claim-assets/../../secret.png",
		"/claim-assets/%2e%2e%2f%2e%2e%2fsecret.png",
		"/claim-assets/widget.contract.one/assets/%2fetc%2fpasswd",
		"/claim-assets/widget.contract.one//etc/passwd",
		"/claim-assets/widget.contract.one/assets/diagram.png%00.txt",
	} {
		assertNotFound(t, base, p)
	}
}

// TestClaimAsset_AbsolutePathIsRefused pins that an absolute-looking rest is not
// joined as one.
func TestClaimAsset_AbsolutePathIsRefused(t *testing.T) {
	base, root := assetProject(t)
	abs := filepath.Join(root, "secret.png")
	assertNotFound(t, base, "/claim-assets/widget.contract.one"+abs)
	assertNotFound(t, base, "/claim-assets"+abs)
}

// TestClaimAsset_SymlinkIsRefused is the check the allowlist alone cannot make.
// The claim genuinely references assets/diagram.png; the file at that path is a
// symlink pointing outside the claims tree. The allowlist entry is legitimate,
// so only canonicalising the path and comparing it against its lexical form
// catches this.
func TestClaimAsset_SymlinkIsRefused(t *testing.T) {
	base, root := assetProject(t)

	target := filepath.Join(root, "secret.png")
	link := filepath.Join(root, "claims", "facet-a", "assets", "diagram.png")
	if err := os.Remove(link); err != nil {
		t.Fatalf("remove fixture: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable on this platform: %v", err)
	}

	assertNotFound(t, base, "/claim-assets/widget.contract.one/assets/diagram.png")
}

// TestClaimAsset_SymlinkedDirectoryIsRefused is the same defence one level up: a
// legitimately named assets/ directory that is itself a link out of the tree.
func TestClaimAsset_SymlinkedDirectoryIsRefused(t *testing.T) {
	base, root := assetProject(t)

	outside := filepath.Join(root, "outside-assets")
	writeBytes(t, filepath.Join(outside, "diagram.png"), []byte("outside the tree"))

	assets := filepath.Join(root, "claims", "facet-a", "assets")
	removeWatchedDir(t, assets)
	if err := os.Symlink(outside, assets); err != nil {
		t.Skipf("symlinks unavailable on this platform: %v", err)
	}

	assertNotFound(t, base, "/claim-assets/widget.contract.one/assets/diagram.png")
}

// TestClaimAsset_SymlinkInsideTheTreeIsRefused is the case the "inside
// claims_dir" test cannot see, and therefore the one that actually exercises the
// canonicalise-and-compare clause: assets/diagram.png is a link to ANOTHER
// CLAIM'S asset. The target is a real image, in a real assets/ directory, inside
// claims_dir — every containment check passes. What refuses it is that the
// resolved path is not the path the index built, which is the same rule that
// makes ../other-facet/assets/x.png illegal in the first place. Co-location is
// not a property a symlink gets to launder.
func TestClaimAsset_SymlinkInsideTheTreeIsRefused(t *testing.T) {
	base, root := assetProject(t)

	target := filepath.Join(root, "claims", "facet-b", "assets", "other.svg")
	link := filepath.Join(root, "claims", "facet-a", "assets", "diagram.png")
	if err := os.Remove(link); err != nil {
		t.Fatalf("remove fixture: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable on this platform: %v", err)
	}

	assertNotFound(t, base, "/claim-assets/widget.contract.one/assets/diagram.png")

	// The target itself is still served through its OWN claim, so this is a
	// refusal of the route, not of the file.
	if resp, _ := do(t, http.MethodGet, base+"/claim-assets/widget.contract.two/assets/other.svg", ""); resp.StatusCode != http.StatusOK {
		t.Errorf("the link target must still be served through its own claim: status %d", resp.StatusCode)
	}
}

// TestClaimAsset_DirectoryIsNotServed pins that only a regular file is sent: no
// listing, and no attempt to read a directory as a file.
func TestClaimAsset_DirectoryIsNotServed(t *testing.T) {
	base, _ := assetProject(t)
	assertNotFound(t, base, "/claim-assets/widget.contract.one/assets")
	assertNotFound(t, base, "/claim-assets/widget.contract.one/assets/")
}

// TestClaimAsset_MissingFileIs404 pins the ordinary miss: the claim references
// it and it IS on the allowlist, but the file is gone from disk. The reviewer
// gets a broken image, not a stack trace and not a 500.
func TestClaimAsset_MissingFileIs404(t *testing.T) {
	base, root := assetProject(t)
	if err := os.Remove(filepath.Join(root, "claims", "facet-a", "assets", "diagram.png")); err != nil {
		t.Fatalf("remove fixture: %v", err)
	}
	assertNotFound(t, base, "/claim-assets/widget.contract.one/assets/diagram.png")
}

// --- method and admission ---------------------------------------------------

// TestClaimAsset_IsGetOnly pins that the route reads and nothing else.
func TestClaimAsset_IsGetOnly(t *testing.T) {
	base, _ := assetProject(t)
	const p = "/claim-assets/widget.contract.one/assets/diagram.png"
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		mods := allowedMutating(base)
		resp, _ := do(t, method, base+p, "", mods...)
		if resp.StatusCode == http.StatusOK {
			t.Errorf("%s %s returned 200; the route must be GET only", method, p)
		}
	}
}

// TestClaimAsset_GoesThroughAdmission pins that the new route did not become a
// hole in the trust boundary: it is behind the same middleware as everything
// else, so a rebound Host still fails before the handler runs.
func TestClaimAsset_GoesThroughAdmission(t *testing.T) {
	base, _ := assetProject(t)
	const p = "/claim-assets/widget.contract.one/assets/diagram.png"

	resp, _ := do(t, http.MethodGet, base+p, "", setHost("evil.example:1"))
	if resp.StatusCode != http.StatusMisdirectedRequest {
		t.Errorf("rebound Host: status %d, want 421", resp.StatusCode)
	}
	resp, _ = do(t, http.MethodGet, base+p, "", setHeader("Sec-Fetch-Site", "cross-site"))
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("cross-site fetch: status %d, want 403", resp.StatusCode)
	}
}

// --- the allowlist tracks the claims ---------------------------------------

// TestClaimAsset_AllowlistFollowsAClaimEdit pins that the allowlist is derived
// rather than captured at startup: removing the reference from the claim makes
// the file unreachable, and it does so on its own, without any request having
// touched the claim.
//
// IT IS BOUNDED, NOT INSTANTANEOUS, and that is the deliberate change. Verifying
// freshness costs a stat-walk of the whole claims tree, and a page fires one
// asset request per image, so a per-request walk made one page view cost
// images x claims. The walk is now amortised over one watcher tick, which is the
// staleness window: an entry can survive its claim by at most that long. The
// bound this test asserts is far looser than one tick on purpose — the defect it
// exists to catch is an entry that outlives its claim INDEFINITELY (see the
// dot-directory and ".tmp-" cases in claim_assets_index_test.go), and pinning a
// cache's exact timing would only buy flakes.
func TestClaimAsset_AllowlistFollowsAClaimEdit(t *testing.T) {
	base, root := assetProjectWatch(t, fastPoll, fastDebounce)
	const p = "/claim-assets/widget.contract.one/assets/diagram.png"

	if resp, _ := do(t, http.MethodGet, base+p, ""); resp.StatusCode != http.StatusOK {
		t.Fatalf("precondition: status %d", resp.StatusCode)
	}

	writeFile(t, filepath.Join(root, "claims", "facet-a", "one.yaml"),
		"id: widget.contract.one\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n"+
			"body: |\n  no image in this body any more, at all, whatsoever.\n"+
			"governed_by:\n  type: none\n  reason: fixture\n")

	assertNotFoundWithin(t, base, p, stalenessBound)
}

// stalenessBound is how long a staleness test waits for a claim change to reach
// the allowlist. The real window is one watcher tick (fastPoll in these tests);
// this is orders of magnitude larger because it is a LIVENESS bound, not a
// latency assertion — it fails only if the entry never goes away.
const stalenessBound = 3 * time.Second

func assertNotFound(t *testing.T, base, path string) {
	t.Helper()
	resp, body := do(t, http.MethodGet, base+path, "")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET %s: status %d, want 404 (body %q)", path, resp.StatusCode, truncate(body))
	}
	assertNoPathLeak(t, path, body)
}

// assertNotFoundWithin is assertNotFound for a refusal that becomes true rather
// than being true already: it polls until the route 404s, then applies the same
// no-leak assertion to the body it finally got.
func assertNotFoundWithin(t *testing.T, base, path string, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for {
		resp, body := do(t, http.MethodGet, base+path, "")
		if resp.StatusCode == http.StatusNotFound {
			assertNoPathLeak(t, path, body)
			return
		}
		if time.Now().After(deadline) {
			t.Errorf("GET %s: still %d after %s; the allowlist entry outlived the claim that produced it", path, resp.StatusCode, within)
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// assertNoPathLeak: a 404 must not confirm what it refused. Nothing that looks
// like a filesystem path may appear in the body.
func assertNoPathLeak(t *testing.T, path string, body []byte) {
	t.Helper()
	if strings.Contains(string(body), "/") && strings.Contains(strings.ToLower(string(body)), "claims") {
		t.Errorf("GET %s: the 404 body leaks a path: %q", path, truncate(body))
	}
}

func truncate(b []byte) string {
	if len(b) > 200 {
		return string(b[:200]) + "..."
	}
	return string(b)
}
