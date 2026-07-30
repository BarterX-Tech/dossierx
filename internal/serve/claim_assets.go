// claim_assets.go is the ONE route in this server that reads a file off disk
// and sends its bytes back, and it is built so that the reason it is safe is
// structural rather than procedural.
//
// WHY A ROUTE EXISTS AT ALL. "dossierx serve" is the only human surface — the
// standalone render verb was retired in v0.3.0 and nothing writes static HTML —
// so a claim-body image that cannot load under serve is the feature not working.
// The two fast ways to make one load are "img-src *" and an http.FileServer
// rooted at the project, and each demolishes one of the two controls that make
// this server safe today. Neither is here.
//
// WHAT MAKES IT SAFE: THE SET OF LEGAL PATHS IS COMPUTED, NOT DISCOVERED. Every
// legal image is an assets/ file referenced by a claim body the engine has
// already loaded, so the allowlist is derivable — run the SAME renderer over the
// same claims and collect what it accepted (markdown.ClaimBodyImages), key each
// one by the SAME URL the page emits (components.ClaimAssetURLPrefix). A file
// nothing references is not reachable, even when it sits in a legal assets/
// directory beside a real claim with a legal extension. That is the whole
// control, and it is why the co-location rule earns its keep: without it the
// legal set would be "some subtree", which is a filesystem question, and answers
// to filesystem questions are how directory traversal happens.
//
// "WHAT THE RENDERER EMITS" IS A NARROWER SET THAN "WHAT THE CLAIM CONTAINS",
// and the index has to be built from the narrow one or the sentence above is
// false. Three things make them differ, and all three are handled in
// buildAssetIndex rather than left to the request path:
//
//   - THE PERMISSION IS PER-PARTIAL. layout:tree renders Body raw inside a <pre>
//     with no markdown at all; every layout but steps ignores Steps entirely. So
//     the index asks components.ClaimImageSurfaces which of a claim's fields its
//     partial actually renders, instead of assuming Body and Steps always count.
//   - THE LAYOUT MAY BE INFERRED. loader.LoadClaims does not fill it in;
//     internal/catalog does, and the page is rendered from the catalog. The index
//     is built from the catalog for the same reason.
//   - AN ID MAY NOT BE UNIQUE. The URL is keyed by claim id, so two claims that
//     share one collapse onto a single key — and "first writer wins" does not
//     merely pick a winner, it points the LOSER'S page at the winner's file. An
//     ambiguous id therefore loses the image capability outright, which is the
//     same degradation an unroutable id already gets.
//
// AND THE INDEX MUST NOT OUTLIVE THE CLAIM. Freshness is a fingerprint
// comparison, so the fingerprint has to enumerate exactly the files the loader
// reads — see scanLoadedClaimFingerprint, and see scanFingerprint for why the
// watcher's narrower scan is not usable here. That comparison is AMORTISED
// rather than made per request; claimAssets owns that argument and states the
// staleness window it buys.
//
// THE PATH IS DEFENDED ANYWAY, independently of the allowlist, because an
// allowlist entry is only as good as the claim that produced it and a file on
// disk can change after the entry was made:
//
//   - Every path in the index is built under the CANONICALISED claims_dir, so
//     "inside claims_dir" is a property of how it was constructed.
//   - At request time the file is canonicalised again and must still equal its
//     own lexical path. That is what catches a symlink — a legitimately
//     referenced assets/diagram.png that is a link out of the tree passes every
//     other check there is.
//   - Only a regular file is opened, so a directory is never listed and never
//     read.
//   - The Content-Type comes from a closed switch over the six legal
//     extensions, never from sniffing, and the response carries nosniff plus its
//     own restrictive CSP — an SVG is a document a browser will run script in if
//     it is ever navigated to directly.
//
// EVERY REFUSAL IS 404. Not 403, and never 403-with-a-reason: a status that
// distinguished "exists but forbidden" from "does not exist" would be an
// existence oracle over the filesystem, answerable from any page the admission
// middleware lets through.
package serve

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/render/components"
	"github.com/BarterX-Tech/dossierx/internal/render/markdown"
)

// assetRoutePattern is the route's ServeMux pattern. The prefix is
// components.AssetRoutePrefix — the same constant the renderer emits — so the
// server and the page cannot disagree about where images live. "{rest...}" is
// only there to make the pattern match a subtree; the handler keys off the
// whole request path rather than off the wildcard, because the whole path is
// what the index is keyed by.
const assetRoutePattern = "GET " + components.AssetRoutePrefix + "{rest...}"

// assetCSPValue is the policy sent with an image response. It is not the page's
// policy: an image is not a document, and the one case that matters is a
// reviewer (or a script) navigating DIRECTLY to an .svg URL, where the browser
// treats the response as a document and will run script in it. "default-src
// 'none'; sandbox" makes that document inert. It has no effect on the same file
// loaded as an <img> subresource, which is the ordinary path.
const assetCSPValue = "default-src 'none'; sandbox"

// assetContentTypes is the closed extension -> media type switch. It is the same
// six extensions markdown.ImageSrc accepts and internal/lint's asset-scope rule
// enforces; a file whose extension is not here is not served at all, so this map
// is a second, independent statement of the allowlist rather than a convenience.
var assetContentTypes = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".svg":  "image/svg+xml",
}

// assetIndex is the computed allowlist: the exact set of request paths this
// server will answer, each mapped to the exact file it answers with.
//
// fp is the claims-tree fingerprint the index was computed from — a
// path+mtime+size stamp over EXACTLY the files loader.LoadClaims reads
// (scanLoadedClaimFingerprint), which is not the same set the watcher's
// scanFingerprint stamps. Holding it is what lets a later check decide whether
// this index still describes the claims on disk: re-stamp the tree, compare,
// rebuild on a difference. HOW OFTEN that check runs is claimAssets' business,
// not this struct's.
//
// THE TWO SCANS MUST NOT BE SWAPPED. The watcher's excludes dot-directories and
// any name containing ".tmp-", because live reload must not flap on the atomic
// writer's scratch files; the loader excludes neither, so a claim in
// claims/.archive/ or a claim named "retry.tmp-policy.yaml" is loaded, rendered
// and indexed while being invisible to that scan. Deriving this index's
// freshness from it meant such a claim's entry survived the claim's own deletion
// indefinitely — the entry could only be cleared by an unrelated, non-excluded
// claim happening to change. That is also why the watcher's onChange callback is
// NOT wired to invalidate this index: it fires on a signal that cannot see two
// of the claim shapes the index carries, so it would be correct for most claims
// and silently wrong for those, which is the worst of the available answers.
type assetIndex struct {
	// root is the canonicalised, absolute claims_dir every path below sits
	// under. Empty when it could not be resolved, in which case paths is empty
	// too and every request 404s.
	root string
	// paths maps a full request path to an absolute file path.
	paths map[string]string
	fp    map[string]fileStamp
}

// claimAssets returns the current index, re-verifying it against the claims on
// disk at most once per watcher tick and rebuilding it when the tree has changed.
//
// THE FRESHNESS CHECK IS A WALK, SO IT IS AMORTISED, NOT PER REQUEST. Verifying
// freshness means stat-ing every claim file (scanLoadedClaimFingerprint), and
// that walk costs O(claims) whether the answer is yes or no. A rendered page
// fires one asset GET per image, so a per-request walk makes ONE PAGE VIEW cost
// images x claims — quadratic in the corpus, on the path a browser hits twenty
// times in a burst. Measured on a synthetic corpus, the walk WAS the whole
// request: at 4800 claims a hit and a miss both cost ~20ms against a ~20ms bare
// walk, one page view of twenty images cost 412ms, and 400 concurrent misses
// took 2.5s because every one of them walked the tree outside this lock and they
// contended super-additively. A 404 probe paid the same walk as a hit, so a
// local caller could pin the server with misses alone.
//
// SO THE ANSWER IS MEMOISED FOR ONE WATCHER TICK (assetFreshnessWindow). Inside
// that window the index is returned without touching the disk at all, which
// makes the per-view aggregate O(claims) — one walk for the page, not one per
// image — and makes a miss as cheap as a map lookup.
//
// THE WINDOW IS THE STALENESS WINDOW ONLY WHILE EVERY CHECK SUCCEEDS. On that
// path it is bounded by the interval and nothing else: an edit that lands just
// after a check is invisible until that check expires, so a file the claim no
// longer references stays reachable for at most one poll interval (500ms in
// production), and a newly referenced one is unreachable for at most the same.
// That is acceptable because it is strictly tighter than the staleness the
// reviewer is already looking at: the page itself only re-renders after the
// watcher's poll PLUS its debounce (500ms + 200ms), so an asset index at most
// one poll old can never be the reason a human sees a stale image — the markup
// naming that image is older. It is also far below the human loop this serves
// (save the file, switch to the browser, reload), and the claim of "synchronous"
// freshness this route used to make was never worth paying a full tree walk per
// image for.
//
// WHEN A CHECK CANNOT COMPLETE, THE WINDOW BOUNDS NOTHING, and that is why
// keepAssets carries a second, absolute bound instead of merely returning the
// previous index. The window says how often the check is ATTEMPTED; on its own
// it says nothing about how long an UNCONFIRMED entry survives, and conflating
// the two was a real defect: buildAssetIndex fails on ANY loader.LoadClaims
// error, one unparseable *.yaml anywhere under claims_dir is enough (the loader
// decodes with KnownFields(true), so a stray non-claim YAML file does it), and
// while that file sits there every check fails and the allowlist never changes.
// Measured on a real server at production cadence: a de-referenced image kept
// answering 200 with its bytes for 386 seconds and counting, and 404'd 0.5s
// after the unparseable sibling was deleted. Re-measured the same way while
// fixing it: 45 seconds of continuous 200 before the bound, 2.0s to 404 after,
// with the sibling present throughout in both runs.
//
// SO THE HONEST WORST CASE IS TWO NUMBERS, not one. While the tree parses: one
// poll interval (500ms in production). While it does not: assetKeepGrace since
// the index was last CONFIRMED, plus the poll interval it takes to notice —
// 2.5s in production — after which an empty index is installed and every image
// 404s until the tree parses again.
//
// THE WALK RUNS UNDER THE LOCK, deliberately, as does the rebuild behind it. N
// concurrent image requests that arrive with an expired memo must produce ONE
// walk and ONE load, not N of each — that is the same argument the render
// pipeline's single-flight makes, and it is what turns the super-additive
// contention above into a queue behind a single scan.
func (s *Server) claimAssets() *assetIndex {
	s.assetsMu.Lock()
	defer s.assetsMu.Unlock()

	if s.assets != nil && time.Since(s.assetsCheckedAt) < s.assetFreshnessWindow() {
		return s.assets
	}
	now := time.Now()
	// THE ATTEMPT CLOCK. Stamped BEFORE the work, and stamped even when the work
	// fails, so a tree that is unreadable or mid-rewrite is retried once per
	// window rather than once per request. It is evidence that a check ran, and
	// of nothing else — the moment it is read as evidence that the index is
	// still true, an index that can never be re-verified becomes immortal. The
	// clock that carries that meaning is assetsConfirmedAt, stamped only on the
	// two branches below that actually establish the index describes disk.
	s.assetsCheckedAt = now

	s.assetScans.Add(1)
	fp, err := scanLoadedClaimFingerprint(s.cfg.ClaimsDir)
	if err != nil {
		// The tree is unreadable (mid-rename, permissions). Nothing was
		// confirmed, so the confirmation clock keeps running.
		return s.keepAssets(now)
	}
	if s.assets != nil && fingerprintsEqual(s.assets.fp, fp) {
		// A CONFIRMATION. The stamps over exactly the files the loader reads are
		// unchanged, so the installed index still describes the claims on disk —
		// as strong a statement as a rebuild makes, without the rebuild.
		s.assetsConfirmedAt = now
		return s.assets
	}
	idx, ok := s.buildAssetIndex(fp)
	if !ok {
		return s.keepAssets(now)
	}
	s.assets = idx
	s.assetsConfirmedAt = now
	return s.assets
}

// assetKeepGrace is how long an index may go UNCONFIRMED and still authorise
// requests. Past it, a failed re-verification installs an EMPTY index instead of
// the previous one, and the route 404s until the tree can be loaded again.
//
// TWO SECONDS, TAKEN FROM THE TRANSIENT THE KEEP EXISTS FOR — a claim file being
// rewritten. Every writer in this codebase renames a temp file over its target
// (lock, digest and buildorder each spell out atomicWriteFile), so the
// unparseable state is at worst the few milliseconds an in-place editor leaves a
// half-written file visible, and it is gone by the following freshness check.
// Two seconds is FOUR production poll intervals: a legitimate rewrite has to
// lose four consecutive re-verifications before the allowlist narrows, which is
// margin of a different order than the event needs, while a tree that simply
// does not parse stops authorising anything after seconds instead of never.
//
// IT IS A REAL-TIME CONSTANT, NOT A MULTIPLE OF pollInterval, unlike
// assetFreshnessWindow — the one place in this file where following the
// watcher's cadence would be wrong. That window is a cadence (how often to ask)
// and belongs on the watcher's clock. This is a CAP ON STALE AUTHORISATION, and
// the transient it tolerates takes the same milliseconds no matter how often
// this server polls; deriving it from pollInterval would let a test that drives
// the watcher fast silently shrink a bound that exists for safety. The coupling
// that does matter runs the other way: a poll interval at or above this grace
// degenerates the keep into "fail closed on the first failed check", which is
// the safe direction to degenerate in.
//
// FAILING CLOSED HERE MEANS BROKEN IMAGES, WHICH IS THE RIGHT NOISE. The same
// LoadClaims failure already makes GET / render a 500 error page, so a reviewer
// whose tree does not parse is looking at a broken page, not at a page that
// quietly shows them files their claims no longer reference.
const assetKeepGrace = 2 * time.Second

// keepAssets is the answer to a freshness check that could not be completed —
// an unreadable tree, or a load that caught a claim file mid-rewrite.
//
// A FAILED RE-VERIFICATION LEAVES THE ALLOWLIST EXACTLY AS IT WAS — for
// assetKeepGrace, and then no longer (the paragraph after this one is half the
// rule, not a footnote to it). It must not
// widen it, and this cannot: the previous index was computed from claims that
// really were on disk, so every path in it was a path the renderer really did
// emit. It must not NARROW it either, which is what installing an empty index
// did. An agent rewriting a claim publishes a partial YAML file for a few
// milliseconds; a request landing in that window got a LoadClaims error and
// installed an empty index — so every image on the page 404'd, and because the
// empty index was installed carrying the CURRENT fingerprint, it stayed until
// the next fingerprint delta. Under sustained writes that was most of the time,
// and a 404 in a browser is not retried: the reviewer sees broken images for a
// state that no longer exists. Keeping the last good index costs nothing in
// safety and removes that whole failure mode.
//
// BUT IT IS KEPT FOR A BOUNDED TIME, NOT INDEFINITELY, and the bound is measured
// from the last CONFIRMATION (assetsConfirmedAt) rather than from the last
// attempt. Those are different clocks and treating them as one is what made this
// keep unsafe: the failure above is not always transient. buildAssetIndex fails
// on any loader.LoadClaims error, and a single unparseable *.yaml anywhere under
// claims_dir is one — so with a stray non-claim YAML file sitting in the tree,
// every check from then on fails, every check keeps the same index, and a file
// the claims stopped referencing stays reachable for as long as that file
// exists. Measured: 386 seconds and counting, cleared 0.5s after the sibling was
// deleted. Past assetKeepGrace this installs an EMPTY index, so the route stops
// answering rather than answering from an allowlist nothing has vouched for in
// seconds.
//
// The failed fingerprint is deliberately NOT recorded, so the next check after
// the window elapses re-does the work rather than treating the failure as the
// new truth — and neither is assetsConfirmedAt advanced, so a run of failures
// expires the keep on schedule instead of resetting it. now is the caller's
// single clock read for this check. Caller holds assetsMu.
func (s *Server) keepAssets(now time.Time) *assetIndex {
	if s.assets != nil && now.Sub(s.assetsConfirmedAt) <= assetKeepGrace {
		return s.assets
	}
	// Either nothing has ever been built — there is no previous answer to keep —
	// or what was built has not been confirmed against disk for longer than the
	// grace. The failure direction for this structure is "serve no images".
	s.assets = &assetIndex{}
	return s.assets
}

// assetFreshnessWindow is how long one freshness check is trusted for: exactly
// one watcher tick. Tying it to the poll interval rather than to a constant of
// its own means there is ONE cadence in this server to reason about — the index
// cannot lag the live-reload the reviewer is watching, and a test that drives
// the watcher fast (SetWatchIntervals) drives this at the same speed for free.
func (s *Server) assetFreshnessWindow() time.Duration { return s.pollInterval }

// buildAssetIndex computes the allowlist from the claims on disk. It returns
// false when it could not compute one at all — an unresolvable claims_dir, or a
// load that caught the tree mid-write — and its caller then keeps whatever index
// was already installed (see keepAssets). It never returns a PARTIAL index: the
// only two outcomes are a complete allowlist and "I could not tell", because a
// half-built one is indistinguishable from a corpus that really did shrink.
func (s *Server) buildAssetIndex(fp map[string]fileStamp) (*assetIndex, bool) {
	idx := &assetIndex{paths: map[string]string{}, fp: fp}

	// The index is built under the CANONICAL claims_dir so that a later
	// EvalSymlinks of any path in it is a no-op unless a symlink sits INSIDE
	// the tree. Resolving the root here rather than comparing against the raw
	// path is what lets the request-time check be an exact equality — on macOS
	// a temp-dir claims_dir is itself reached through /var -> /private/var, and
	// a check that did not account for that would either refuse everything or
	// have to be loosened into something weaker.
	rawRoot, err := filepath.Abs(s.cfg.ClaimsDir)
	if err != nil {
		return nil, false
	}
	root, err := filepath.EvalSymlinks(rawRoot)
	if err != nil {
		return nil, false
	}
	idx.root = root

	claims, err := loader.LoadClaims(s.cfg.ClaimsDir)
	if err != nil {
		return nil, false
	}
	// THROUGH THE CATALOG, because that is what the page is rendered from. Its
	// one contribution here is layout inference — loader.LoadClaims leaves
	// Layout empty when the claim omits it, and layout is what decides which of
	// a claim's fields can produce an image at all (see indexClaimAssets).
	// Building the index off the raw loader output would ask
	// components.ClaimImageSurfaces about a layout no claim is ever rendered
	// with. serve's other normalisation step, disarmUngatedMockups, is not
	// applied: it only ever clears RawHTMLReviewed, and RawHTML is not an
	// image-permitting surface on any layout.
	cat, err := catalog.Build(claims, s.cfg)
	if err != nil {
		return nil, false
	}

	dupIDs := duplicateClaimIDs(cat.Claims)
	for _, c := range cat.Claims {
		if dupIDs[c.ID] {
			// AN AMBIGUOUS ID HAS NO IMAGE CAPABILITY. Two claims sharing an id
			// emit the identical <img src>, so any single answer shows at least
			// one of them a file from the other's directory — co-location
			// defeated without needing a symlink, and silently, since serve does
			// not lint. Refusing the id entirely is the only answer that never
			// hands one claim another claim's bytes; both cards get the broken
			// image every other refusal in this feature produces.
			continue
		}
		s.indexClaimAssets(idx, rawRoot, c)
	}
	return idx, true
}

// duplicateClaimIDs returns the ids carried by more than one loaded claim.
// Duplicate ids are a lint error, and serve deliberately does not lint (see
// disarmUngatedMockups for the same argument applied to raw_html), so this is
// checked here rather than assumed away.
func duplicateClaimIDs(claims []model.Claim) map[string]bool {
	seen := make(map[string]int, len(claims))
	for _, c := range claims {
		seen[c.ID]++
	}
	dup := map[string]bool{}
	for id, n := range seen {
		if n > 1 {
			dup[id] = true
		}
	}
	return dup
}

// indexClaimAssets adds one claim's accepted images to the index.
//
// A claim contributes nothing when it has no usable route key, no SourcePath
// (a claim built in memory has no directory to resolve against), or a directory
// that does not sit inside claims_dir. Its caller has already excluded claims
// whose id is not unique.
func (s *Server) indexClaimAssets(idx *assetIndex, rawRoot string, c model.Claim) {
	prefix, ok := components.ClaimAssetURLPrefix(c)
	if !ok || c.SourcePath == "" {
		return
	}
	dir, err := filepath.Abs(filepath.Dir(c.SourcePath))
	if err != nil {
		return
	}
	if !isInsideDir(rawRoot, dir) {
		return
	}
	relDir, err := filepath.Rel(rawRoot, dir)
	if err != nil {
		return
	}
	// The claim's own directory, re-expressed under the CANONICAL root. Every
	// path in the index descends from here, which is what makes "inside
	// claims_dir" a property of construction rather than of a later check.
	base := filepath.Join(idx.root, relDir)

	add := func(rel string) {
		urlPath := string(prefix) + rel
		if _, seen := idx.paths[urlPath]; seen {
			// The same claim referencing the same file twice. Ids are unique by
			// the time this runs, so a repeat can only resolve to the file
			// already recorded.
			return
		}
		file := filepath.Join(base, filepath.FromSlash(rel))
		if !isInsideDir(idx.root, file) {
			return
		}
		idx.paths[urlPath] = file
	}

	// TWO FUNCTIONS, ONE STATEMENT EACH, AND BETWEEN THEM THE WHOLE EQUIVALENCE
	// the route's safety rests on. components.ClaimImageSurfaces says WHICH of
	// this claim's fields its layout's partial renders through the
	// image-permitting entry point — Body alone for most layouts, Body plus
	// every step for layout:steps, and NOTHING for layout:tree, whose partial
	// emits Body raw inside a <pre>. markdown.ClaimBodyImages then says which
	// srcs in that text the renderer accepts, by running the same block and
	// inline passes it renders with. Neither is a second, simpler scanner that
	// could disagree with the emitter.
	for _, text := range components.ClaimImageSurfaces(c) {
		for _, rel := range markdown.ClaimBodyImages(text) {
			add(rel)
		}
	}
}

// handleClaimAsset answers one image request. Every branch that is not the
// success branch is a bare 404 with no detail.
func (s *Server) handleClaimAsset(w http.ResponseWriter, r *http.Request) {
	// (1) NO PERCENT-ENCODING. Every path the renderer emits is drawn from
	// [A-Za-z0-9._/-] (see markdown.ImageSrc and components.ClaimAssetURLPrefix),
	// so an escaped request path is by construction not one of ours. Refusing
	// it outright means nothing downstream ever has to reason about what
	// "%2e%2e%2f" decodes to, or about whether the mux decoded it before or
	// after it cleaned the path.
	if r.URL.EscapedPath() != r.URL.Path {
		http.NotFound(w, r)
		return
	}

	// (2) THE ALLOWLIST. Exact match on the whole request path, which is the
	// exact string the page emitted. There is no path arithmetic here at all.
	// The index is taken ONCE and every later check reads that same snapshot,
	// so a rebuild racing this request cannot have the lookup answered from one
	// index and the root check from another.
	idx := s.claimAssets()
	file, ok := idx.paths[r.URL.Path]
	if !ok {
		http.NotFound(w, r)
		return
	}

	// (3) THE EXTENSION, again, at the point of service. The index cannot hold
	// anything else, and this is what makes that true of the response too.
	ctype, ok := assetContentTypes[strings.ToLower(filepath.Ext(file))]
	if !ok {
		http.NotFound(w, r)
		return
	}

	// (4) CANONICALISE AND COMPARE. The index built this path under the
	// canonical root, so a resolved path that differs from its lexical form
	// means a symlink lies on it — the one attack an allowlist derived from
	// claim text cannot see, because the claim's own reference is legitimate.
	// The isInsideDir check after it is redundant by construction and kept
	// because "redundant by construction" is a property of today's builder.
	resolved, err := filepath.EvalSymlinks(file)
	if err != nil || resolved != file || !isInsideDir(idx.root, resolved) {
		http.NotFound(w, r)
		return
	}

	// (5) A REGULAR FILE, opened as one. No directory listing, no device, no
	// fifo, and no http.FileServer anywhere in the path.
	f, err := os.Open(resolved)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close() //nolint:errcheck // read-only handle
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", ctype)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", assetCSPValue)
	// The viewer live-reloads and an author replaces a diagram in place; a
	// cached image would show the reviewer the previous one with no way to
	// know. This server is local, so there is nothing to save by caching.
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, "", info.ModTime(), f)
}
