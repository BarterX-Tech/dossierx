package viewertests

// THE SITE, READ AS RENDERED DOM.
//
// site/ is a client-facing surface (surfaces.yaml: "the surface is the RENDERED
// DOM of a real build (plus its head metadata), not the component source"), and
// this file is what turns that sentence into an artifact: it builds the Vite app
// exactly as .github/workflows/deploy-site.yml does, drives BOTH entry points
// with the same headless Chrome the rest of this module uses, and writes every
// string a visitor can reach to site-text.json for the release gate's prose
// agents to read against surface.json.
//
// BOTH entry points, and by their REAL names. vite.config.ts declares two static
// inputs — index.html and releases.html — and there is no SPA rewrite, so
// "/releases/" is a 404 rather than the release history. Fetching it and finding
// no version prose reads as "the site says nothing false about releases", which
// is a false negative this project has already been bitten by; the entry list
// below is therefore the built FILE names, and assertEntryPointsReachable pins
// that /releases/ really is a 404 so nobody quietly "fixes" the URL back.
//
// RENDERED DOM IS NOT RENDERED TEXT. Three kinds of falsifiable engine prose sit
// outside anything innerText would return, and each was found the hard way:
//
//   - ATTRIBUTE PROSE. AgentCompat.tsx states engine claims ("One embedded
//     markdown bundle, exported into three places") as aria-label on path-only
//     SVGs, where there is no text node at all. So the dump collects aria-label,
//     title and alt alongside the text.
//   - CLOSED DISCLOSURES. SurfacePair.tsx renders a <details> whose children ARE
//     in the DOM and are absent from innerText. So the dump reads textContent.
//   - CONDITIONAL RENDER. Cli.tsx mounts a command's body only while it is the
//     open one, TabbedCode.tsx mounts only the active tab's panel, and Hero.tsx
//     mounts each card's claim snippet only while that card is active. None of
//     that is in the DOM on a plain load, so the driver CLICKS: every <summary>,
//     every button.cmd__head, every button[role="tab"], everything matching
//     [role="button"] or [aria-expanded] — and, last, every remaining <button>.
//
// THAT LAST SELECTOR IS NOT A ROUNDING ERROR. The five before it enumerate the
// shapes that hide prose on this site TODAY, and a plain `<button>` gated on
// useState — no aria-expanded, no role, no <details> — is the same defect in a
// form none of them match: it takes its prose out of this artifact while every
// numbered condition below stays green. So the list ends in a bare `button`, and
// it ends there rather than beginning there because Cli's filter chips are plain
// buttons that REMOVE command rows from the DOM. Clicking them last means every
// target inside the list they filter has already been visited, and the paths of
// what remains to click — the chips themselves, in a sibling container — do not
// move when the list shrinks.
//
// AND THE DUMP IS MEASURED AGAINST THE SOURCE, not against itself. Every
// condition below that COUNTS something can be satisfied by an extraction that
// stopped reaching the prose it was built to reach. "No aria-label at all"
// leaves sixteen unrelated labels standing on index.html when the two that carry
// AgentCompat's engine claims are blanked; "bodies >= summaries" is 0 >= 0 on a
// page whose <details> became a button. So condition 6 reads a FLOOR out of
// site/src: every sentence a component hard-codes — as an aria-label / title /
// alt / label attribute, or as JSX text — must be findable in the dump. A floor
// derived from the source cannot be zeroed by the change that hides the prose,
// which is the whole difference between a count and a check.
// declaredCodeExampleGroups does the same job for condition 2, and the note
// there says why that one needed it.
//
// AND IT MUST FAIL RATHER THAN SKIP. Everything below is a t.Fatal, including
// "npm is not installed" and "DOSSIERX_TEST_BROWSER is unset": a check that
// cannot execute is a FAILED gate and never a quiet pass over zero assertions
// (CLAUDE.md; harness_test.go:47). That is why this file does NOT reuse
// resolveBrowser, whose skip is the right answer for the viewer suite on a
// laptop and the wrong one here.
//
// AND THE TOOLCHAIN THAT PRODUCED IT IS PART OF THE ARTIFACT. A dump is only
// evidence about the published site if it was built the way the published site
// is built, and this file used to assert that in a comment — "exactly as
// .github/workflows/deploy-site.yml does" — with nothing comparing the two.
// siteBuildSteps below is therefore a DECLARATION that site_toolchain_test.go
// reads and checks against that workflow, and the node/npm versions the build
// ran under are stamped into site-text.json rather than left to whichever ones
// the machine happened to have.
//
// AND A REPEAT IS EVIDENCE, NOT NOISE. The artifact records each pass by what it
// was the first to reveal, so ~30 clicks do not store ~30 copies of one page.
// That saving was once taken per STRING, which also deleted every repetition
// WITHIN a page: the hero's "v0.5.1" arrives first, and the release-history
// intro's and the timeline's copies of it were dropped as already-seen. The
// gate's site agent read the survivor, found the version in one place, and
// reported that the landing page names no release version — a false finding
// produced by the gate's own eyes, not by the site. The dedupe is now counted
// per OCCURRENCE (readSitePage's counters, and sitePass), and condition 9 pins
// that the intro and the timeline still reach the artifact through the hero's
// shadow.
//
// NOTHING IS WRITTEN INTO site/. The tree under review is the tree being read,
// so the build happens in a copy under t.TempDir(); site/node_modules and
// site/dist never appear.
//
// ONE CONDITION IS NOT IMPLEMENTED AS SPECIFIED, and this is the disclosure of
// it rather than a footnote to it. The gate's condition 7 was written as "fail
// if ANY element in the final DOM still carries aria-expanded=false after the
// traversal", on the premise that no offender finishes the traversal closed.
// That premise is false on this site: Cli's accordion, Hero's card stack and
// Nav's menu are SINGLE-OPEN, so opening one closes its siblings and 22 of the
// 25 aria-expanded elements on index.html necessarily end at "false". Taken
// literally the condition is unsatisfiable and the gate would be permanently
// red, which is a gate nobody can read. Subtest 7 below therefore asserts the
// achievable form of the same guarantee — no element that can expand finished
// the traversal never having been expanded — which fires on exactly the same
// defects (a conditional block the driver never opens) and on nothing else. It
// is an OVERRIDE of a numbered condition, it is recorded here and in this lane's
// report, and a maintainer who restores the literal wording will get a red build
// that no change to the site can turn green.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/chromedp/chromedp"
)

// siteTextFileName is the extraction's output, written beside this suite (and
// git-ignored — it is derived from the tree, like the dist/ it is read from).
// DOSSIERX_SITE_TEXT_OUT overrides it so a gate driver can collect the artifact
// wherever it keeps the rest of the run's evidence.
const siteTextFileName = "site-text.json"

// siteEntry is one of the site's two REAL entry points: the built file name as
// it is actually fetched, plus the JS that is true once that page's React tree
// has mounted. The readiness expression is per-page on purpose — a generic
// "#root has children" would go true while the tree was still empty of the
// things the assertions below count.
type siteEntry struct {
	name  string
	ready string
}

var siteEntries = []siteEntry{
	{name: "index.html", ready: `document.querySelectorAll('#cli .cmd__head').length > 0`},
	{name: "releases.html", ready: `document.querySelectorAll('.timeline .release__version').length > 0`},
}

// ---------------------------------------------------------------------
// the dump
// ---------------------------------------------------------------------

type siteAttr struct {
	Element string `json:"element"`
	Attr    string `json:"attr"`
	Value   string `json:"value"`
}

type siteDetails struct {
	Summary string `json:"summary"`
	Open    bool   `json:"open"`
	Body    string `json:"body"`
}

type siteExpandable struct {
	Path     string `json:"path"`
	Label    string `json:"label"`
	Expanded bool   `json:"expanded"`
}

type siteCodeTabs struct {
	Index  int      `json:"index"`
	Tabs   []string `json:"tabs"`
	Panels []string `json:"panels"`
}

// siteTranscriptLine is one line of a depicted terminal session as the page
// actually renders it. Prompt comes from the line element's own class, not from
// the text: Cli.tsx strips the "$ " before rendering and CSS draws it back with
// ::before, so the character a reader sees is in no text node.
type siteTranscriptLine struct {
	Prompt bool   `json:"prompt"`
	Text   string `json:"text"`
}

// siteTranscript is one command card's session, keyed by the command name shown
// on the same card.
type siteTranscript struct {
	Command string               `json:"command"`
	Lines   []siteTranscriptLine `json:"lines"`
}

// siteSnapshot is one read of the whole document, as __dxSnapshot returns it.
type siteSnapshot struct {
	Text        []string         `json:"text"`
	Attrs       []siteAttr       `json:"attrs"`
	Details     []siteDetails    `json:"details"`
	Summaries   int              `json:"summaries"`
	Expandable  []siteExpandable `json:"expandable"`
	CodeTabs    []siteCodeTabs   `json:"codetabs"`
	CLI         []string         `json:"cli"`
	Head        []string         `json:"head"`
	Transcripts []siteTranscript `json:"transcripts"`
}

// sitePass is one snapshot as it is RECORDED: the text and attributes are the
// ones this interaction revealed for the first time, because storing every
// string again for each of ~30 clicks would make the artifact thirty times the
// size and say nothing extra. The union lives on sitePage.
//
// "FOR THE FIRST TIME" IS COUNTED PER OCCURRENCE, NOT PER STRING, and that
// distinction is the whole difference between an artifact that is smaller than
// the traversal and one that is smaller than the PAGE. A page may render the
// same string in several places on purpose — the current release is spelled
// v0.5.1 in the hero kicker, in the hero badge list, inside the release-history
// intro's <strong>, and again as the timeline entry's version label — and every
// one of those is a separate thing a visitor reads. Flagging a string as "seen"
// the first time any element emitted it deleted the other three from this
// artifact, the gate's site agent read what was left, and it reported that the
// landing page names no version at all. The page was right; the evidence was
// lossy. So the counter below keeps, for every distinct string, as many copies
// as the snapshot that showed the most of them, in the tree-walker's order —
// which is what makes a repeat readable as a repeat, and its neighbours the
// only locality the artifact needs to state. The ~30x saving that motivated
// this dedupe is untouched: it comes from passes that reveal nothing new, and a
// pass that reveals nothing new still records nothing.
type sitePass struct {
	Label      string           `json:"label"`
	Selector   string           `json:"selector,omitempty"`
	NewText    []string         `json:"new_text"`
	NewAttrs   []siteAttr       `json:"new_attributes"`
	Details    []siteDetails    `json:"details"`
	Summaries  int              `json:"summaries"`
	Expandable []siteExpandable `json:"expandable"`
	CodeTabs   []siteCodeTabs   `json:"codetabs"`
}

type sitePage struct {
	Entry       string        `json:"entry"`
	URL         string        `json:"url"`
	Head        []string      `json:"head"`
	CLICommands []string      `json:"cli_commands"`
	Text        []string      `json:"text"`
	Attributes  []siteAttr    `json:"attributes"`
	Details     []siteDetails `json:"details"`
	Passes      []sitePass    `json:"passes"`
	// Transcripts is the union across every pass, keyed by command. A command
	// card renders its session only while it is expanded, so no single pass holds
	// them all; the union is what the assertions read.
	Transcripts []siteTranscript `json:"transcripts"`
}

// transcript returns the session depicted for one command, and whether the dump
// carries one at all. Absence is a distinct answer from an empty session and the
// caller says so differently: a command whose card was never opened is a hole in
// the traversal, where a card with no lines is a page that shows a reader a
// heading and lets them assume the rest.
func (p sitePage) transcript(command string) (siteTranscript, bool) {
	for _, tr := range p.Transcripts {
		if tr.Command == command {
			return tr, true
		}
	}
	return siteTranscript{}, false
}

type siteDump struct {
	// Tree is the full 40-digit tree object name this extraction's build was
	// made from, SUPPLIED by the driver via DOSSIERX_SITE_TEXT_TREE and never
	// probed from git here — for the reason every other capture producer gives
	// (tests/render_diff_capture_test.go, cmd/dossierx/gate_fanout_test.go):
	// each piece of a run's evidence is computed from an identity the driver
	// hands down, and a producer that resolved its own would be a second answer
	// to "which tree is this run about" inside one run.
	//
	// IT IS THE FIRST FIELD ON PURPOSE, and encoding/json preserves struct
	// order. scripts/gate-stage2/run.sh `record` cross-checks this document
	// with json_scalar, which takes the FIRST `"tree"` match it sees on any
	// line and exits — so the stamp must precede every page's text, or a
	// sentence on the site that happened to look like provenance would be read
	// in its place. That is the same positional promise gate/render-diff.json
	// makes, pinned there by TestRenderDiffCaptureProvenanceComesFirst and here
	// by TestSiteTextProvenanceComesFirst.
	//
	// WHY IT EXISTS AT ALL: before this stamp the artifact recorded which node
	// and npm built the site and nothing that named a RELEASE, so an extraction
	// left on disk from the previous gate run recorded cleanly and was hashed
	// into the site surface's key as this release's rendered DOM. The delta
	// gets no such stamp — record RECOMPUTES it — but an extraction cannot be
	// recomputed at record time (it needs this build, this browser), so the
	// stamp is the account it can give. What the stamp cannot promise, exactly
	// as gate/render-diff.json's cannot: that the working tree the build read
	// was clean at that identity. The driver resolves the tree; this file
	// records it verbatim.
	Tree        string `json:"tree"`
	GeneratedBy string `json:"generated_by"`
	// Toolchain is the node/npm pair this dist/ was built by. It is in the
	// artifact because a prose agent reading site-text.json is reading a claim
	// about the PUBLISHED site, and that claim is only as good as the build
	// behind it: "which Node produced this" is not answerable after the fact
	// from a runner whose image moved on.
	Toolchain siteToolchain `json:"toolchain"`
	Pages     []sitePage    `json:"pages"`
}

func (d siteDump) page(t *testing.T, entry string) sitePage {
	t.Helper()
	for _, p := range d.Pages {
		if p.Entry == entry {
			return p
		}
	}
	t.Fatalf("no page %q in the dump; pages read: %v", entry, d.entries())
	return sitePage{}
}

func (d siteDump) entries() []string {
	out := make([]string, 0, len(d.Pages))
	for _, p := range d.Pages {
		out = append(out, p.Entry)
	}
	return out
}

// siteTarget is one thing the driver clicks.
type siteTarget struct {
	Path     string `json:"path"`
	Selector string `json:"selector"`
	Label    string `json:"label"`
}

// ---------------------------------------------------------------------
// the page-side extractor
// ---------------------------------------------------------------------

// siteExtractorJS installs the extraction into the page. Elements are addressed
// by a CHILD-INDEX PATH from documentElement rather than by a selector or by
// their text: .cmd__head is nineteen identically-classed buttons, and the Hero
// cards' text CHANGES when they open (the claim snippet mounts inside the same
// element that carries aria-expanded), so any label-derived key would rename
// itself mid-traversal and lose track of what had been opened. A path is stable
// because every conditional block in this site mounts as a LAST child or as a
// later sibling — it never shifts an ancestor's index.
//
// Clicks go through HTMLElement.click(), not a synthesised mouse at coordinates.
// That is deliberate: Nav.tsx's toggle is display:none above its breakpoint and
// a coordinate click could not reach it at all, so the prose behind it would be
// silently uncollected — the exact class of hole this file exists to close. The
// event is still a real DOM event dispatched by the browser, and React's
// delegated listener runs on it.
const siteExtractorJS = `(function () {
  window.__dxPath = function (el) {
    var parts = [];
    for (var n = el; n && n.parentElement; n = n.parentElement) {
      parts.unshift(Array.prototype.indexOf.call(n.parentElement.children, n));
    }
    return parts.join('/');
  };
  window.__dxAt = function (path) {
    var n = document.documentElement;
    var segs = path === '' ? [] : path.split('/');
    for (var i = 0; i < segs.length && n; i++) { n = n.children[Number(segs[i])]; }
    return n || null;
  };
  window.__dxTidy = function (s) { return (s || '').replace(/\s+/g, ' ').trim(); };
  window.__dxLabel = function (el) {
    return window.__dxTidy(el.getAttribute('aria-label') || el.textContent).slice(0, 90);
  };

  // The click list, in the order the surfaces were found to hide prose, ending
  // in the general case: every remaining button, whatever it is made of. The
  // order is load-bearing — see the header — because the plain buttons on this
  // site today are Cli's filter chips, which delete command rows from the DOM,
  // and a path recorded before they run must still resolve when it is clicked.
  window.__dxTargets = function () {
    var sels = ['summary', 'button.cmd__head', 'button[role="tab"]', '[role="button"]', '[aria-expanded]', 'button'];
    var seen = {}, out = [];
    for (var i = 0; i < sels.length; i++) {
      var els = document.querySelectorAll(sels[i]);
      for (var j = 0; j < els.length; j++) {
        var p = window.__dxPath(els[j]);
        if (seen[p]) { continue; }
        seen[p] = true;
        out.push({ path: p, selector: sels[i], label: window.__dxLabel(els[j]) });
      }
    }
    return out;
  };

  window.__dxClick = function (path) {
    var el = window.__dxAt(path);
    if (!el) { return false; }
    el.click();
    return true;
  };

  window.__dxSnapshot = function () {
    var skip = { SCRIPT: 1, STYLE: 1, NOSCRIPT: 1 };
    var text = [];
    var walker = document.createTreeWalker(document.documentElement, NodeFilter.SHOW_TEXT, {
      acceptNode: function (n) {
        var parent = n.parentElement;
        return parent && skip[parent.tagName] ? NodeFilter.FILTER_REJECT : NodeFilter.FILTER_ACCEPT;
      }
    });
    for (var n = walker.nextNode(); n; n = walker.nextNode()) {
      var t = window.__dxTidy(n.textContent);
      if (t) { text.push(t); }
    }

    var attrs = [];
    var named = document.querySelectorAll('[aria-label],[title],[alt]');
    var keys = ['aria-label', 'title', 'alt'];
    for (var i = 0; i < named.length; i++) {
      for (var k = 0; k < keys.length; k++) {
        var v = window.__dxTidy(named[i].getAttribute(keys[k]));
        if (v) { attrs.push({ element: named[i].tagName.toLowerCase(), attr: keys[k], value: v }); }
      }
    }

    var details = [];
    var ds = document.querySelectorAll('details');
    for (var d = 0; d < ds.length; d++) {
      var sum = ds[d].querySelector('summary');
      var body = '';
      for (var c = 0; c < ds[d].childNodes.length; c++) {
        if (ds[d].childNodes[c] !== sum) { body += ds[d].childNodes[c].textContent || ''; }
      }
      details.push({ summary: window.__dxTidy(sum ? sum.textContent : ''), open: !!ds[d].open, body: window.__dxTidy(body) });
    }

    var expandable = [];
    var xs = document.querySelectorAll('[aria-expanded]');
    for (var x = 0; x < xs.length; x++) {
      expandable.push({
        path: window.__dxPath(xs[x]),
        label: window.__dxLabel(xs[x]),
        expanded: xs[x].getAttribute('aria-expanded') === 'true'
      });
    }

    var codetabs = [];
    var cts = document.querySelectorAll('.codetabs');
    for (var ct = 0; ct < cts.length; ct++) {
      var tabs = [], panels = [];
      var tabEls = cts[ct].querySelectorAll('[role="tab"]');
      for (var a = 0; a < tabEls.length; a++) { tabs.push(window.__dxTidy(tabEls[a].textContent)); }
      var panelEls = cts[ct].querySelectorAll('[role="tabpanel"]');
      for (var b = 0; b < panelEls.length; b++) {
        panels.push(panelEls[b].id || window.__dxTidy(panelEls[b].textContent).slice(0, 60));
      }
      codetabs.push({ index: ct, tabs: tabs, panels: panels });
    }

    var cli = [];
    var names = document.querySelectorAll('#cli .cmd__name');
    for (var m = 0; m < names.length; m++) { cli.push(window.__dxTidy(names[m].textContent)); }

    // The depicted terminal sessions, read as STRUCTURE rather than as text.
    // Cli.tsx's Transcript renders one <div> per line inside a pre.term-out and
    // marks the shell prompt with a class, and the leading "$ " a reader sees is
    // drawn by CSS ::before — it is not in the DOM at all. So a line's own
    // element says whether it is a command or output, which is what lets a Go
    // assertion run the one and compare against the other. Flattening this to
    // text would lose both the prompt/output split and the line boundaries, and
    // a transcript is only true or false as an ordered whole.
    //
    // Keyed by the command name out of the same card, because at most one
    // command is expanded at a time and the assertion has to know which session
    // it is looking at.
    var transcripts = [];
    var cards = document.querySelectorAll('#cli .cmd');
    for (var cd = 0; cd < cards.length; cd++) {
      var nameEl = cards[cd].querySelector('.cmd__name');
      var pre = cards[cd].querySelector('pre.term-out');
      if (!nameEl || !pre) { continue; }
      var tlines = [];
      for (var tl = 0; tl < pre.children.length; tl++) {
        var lineEl = pre.children[tl];
        tlines.push({
          prompt: String(lineEl.className || '').indexOf('term-out__line--prompt') >= 0,
          text: window.__dxTidy(lineEl.textContent)
        });
      }
      transcripts.push({ command: window.__dxTidy(nameEl.textContent), lines: tlines });
    }

    var head = [];
    var title = window.__dxTidy(document.title);
    if (title) { head.push('title: ' + title); }
    var metas = document.head.querySelectorAll('meta[name]');
    for (var q = 0; q < metas.length; q++) {
      var content = window.__dxTidy(metas[q].getAttribute('content'));
      if (content) { head.push('meta[' + metas[q].getAttribute('name') + ']: ' + content); }
    }

    return {
      text: text, attrs: attrs, details: details, summaries: document.querySelectorAll('summary').length,
      expandable: expandable, codetabs: codetabs, cli: cli, head: head, transcripts: transcripts
    };
  };
  return true;
})()`

// siteActivatedJS answers "did that click actually open the thing", asked of the
// element itself in whatever vocabulary it speaks: aria-expanded for the
// accordions and the card stack, details.open for a disclosure, aria-selected
// PLUS a mounted panel for a tab. The tab case needs both halves — TabbedCode
// uses AnimatePresence mode="wait", so aria-selected flips one animation before
// the panel exists, and a dump taken on the attribute alone would record an
// empty panel and call the tab covered.
//
// It is a SELF-CONTAINED function expression that touches nothing on window, and
// that is not a style choice: chromedp.Poll evaluates its predicate in an
// execution context that cannot see the page's own globals. The first cut of
// this file polled window.__dxActivated and got "is not a function" forever —
// i.e. every click that had in fact worked was reported as one that never did.
// Whatever the poll needs, the poll has to carry.
const siteActivatedJS = `function (path) {
  var n = document.documentElement;
  var segs = path === '' ? [] : path.split('/');
  for (var i = 0; i < segs.length && n; i++) { n = n.children[Number(segs[i])]; }
  var el = n || null;
  if (!el) { return false; }
  if (el.hasAttribute('aria-expanded')) { return el.getAttribute('aria-expanded') === 'true'; }
  if (el.tagName === 'SUMMARY') { return !!(el.parentElement && el.parentElement.open); }
  if (el.getAttribute('role') === 'tab') {
    var id = el.getAttribute('aria-controls');
    return el.getAttribute('aria-selected') === 'true' && !!(id && document.getElementById(id));
  }
  return true;
}`

// ---------------------------------------------------------------------
// building and serving the site
// ---------------------------------------------------------------------

// requireSiteBrowser is resolveBrowser's strict twin: with no browser there is
// no rendered DOM, and "we did not look" must not read as "nothing is wrong".
func requireSiteBrowser(t *testing.T) string {
	t.Helper()
	p := os.Getenv("DOSSIERX_TEST_BROWSER")
	if p == "" {
		t.Fatal("DOSSIERX_TEST_BROWSER is unset, so the site's rendered DOM cannot be read. " +
			"This extraction fails rather than skips: a skipped check is indistinguishable " +
			"from a pass over zero assertions. Point it at a Chrome/Chromium binary.")
	}
	if !fileExists(p) {
		t.Fatalf("DOSSIERX_TEST_BROWSER=%q does not exist", p)
	}
	t.Logf("site extraction driving: %s", p)
	return p
}

// siteBuildStep is one npm invocation this extraction makes, with whatever it
// adds to the environment for that step.
type siteBuildStep struct {
	Args []string
	Env  []string // "NAME=value", as exec.Cmd takes them
}

// siteBuildSteps is how the site under test is built: npm ci, then the same
// `npm run build` (tsc -b && vite build) the publish workflow runs.
//
// It is a DECLARATION rather than two inline exec calls because
// site_toolchain_test.go reads it and compares it, step by step and variable by
// variable, against .github/workflows/deploy-site.yml. The header's claim that
// this builds the site "exactly as deploy-site.yml does" was a sentence for a
// long time and a sentence is not a check: a `VITE_*` variable added to the
// publish build and not to this one makes the gate read a page no visitor gets,
// with every condition in this file still green, because they are all integrity
// checks on a dump whose provenance nothing had stated.
//
// VITE_BASE is the one deliberate divergence, and it is declared as such over
// there rather than only explained here: the built assets are referenced
// absolutely, so a dist built for the Pages subpath "/dossierx/" cannot load its
// own JavaScript from the root of a test server. Only the asset URL prefix
// differs; every byte this file reads is produced by the same tsc + vite build.
var siteBuildSteps = []siteBuildStep{
	{Args: []string{"ci"}},
	{Args: []string{"run", "build"}, Env: []string{"VITE_BASE=/"}},
}

// buildSite runs siteBuildSteps in a COPY of site/ and returns the dist
// directory.
func buildSite(t *testing.T, tc siteToolchain) string {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}

	work := filepath.Join(t.TempDir(), "site")
	copySiteTree(t, filepath.Join(root, "site"), work)

	for _, step := range siteBuildSteps {
		runNPM(t, tc.npmPath, work, step.Env, step.Args...)
	}

	dist := filepath.Join(work, "dist")
	for _, e := range siteEntries {
		if _, err := os.Stat(filepath.Join(dist, e.name)); err != nil {
			t.Fatalf("the build produced no %s: %v", e.name, err)
		}
	}
	return dist
}

func runNPM(t *testing.T, npm, dir string, extraEnv []string, args ...string) {
	t.Helper()
	cmd := exec.Command(npm, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), extraEnv...)
	if out, err := cmd.CombinedOutput(); err != nil {
		// `npm ci` reaches the registry, so this is also how a runner with no
		// network egress reports itself. Naming both prerequisites keeps that
		// from being read as a defect in the site.
		t.Fatalf("npm %s failed. This step needs a working npm and reachable registry — both are "+
			"prerequisites of reading the site's rendered DOM, and a check that cannot execute is a "+
			"FAILED gate, never a skip.\n%v\n%s", strings.Join(args, " "), err, out)
	}
}

// copySiteTree copies site/ into a scratch directory, skipping the two
// generated trees. It exists so a gate run never writes into the tree it is
// judging — and so a concurrent editor of site/src is never racing an npm
// install.
func copySiteTree(t *testing.T, src, dst string) {
	t.Helper()
	skip := map[string]bool{"node_modules": true, "dist": true}
	err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(src, path)
		if relErr != nil {
			return relErr
		}
		if d.IsDir() {
			if skip[d.Name()] {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		b, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		return os.WriteFile(filepath.Join(dst, rel), b, 0o644)
	})
	if err != nil {
		t.Fatalf("copy site tree: %v", err)
	}
}

// assertEntryPointsReachable pins the URLs, before a browser is involved: each
// declared entry answers 200, and "/releases/" — the directory form somebody
// reaches for every time — does not. A reader who fetches that and finds no
// release prose concludes the site makes no release claim, which is the false
// negative this whole file is built to prevent.
func assertEntryPointsReachable(t *testing.T, base string) {
	t.Helper()
	for _, e := range siteEntries {
		if code := httpStatus(t, base+"/"+e.name); code != http.StatusOK {
			t.Fatalf("entry point %s answered %d, want 200 — the extraction would read an error page", e.name, code)
		}
	}
	if code := httpStatus(t, base+"/releases/"); code == http.StatusOK {
		t.Fatalf("/releases/ answered 200, but vite.config.ts declares releases.html as a static entry " +
			"and the site deploys with no SPA rewrite. If that changed, this suite's entry list must " +
			"change with it — reading /releases/ and finding nothing is not evidence of anything.")
	}
}

func httpStatus(t *testing.T, url string) int {
	t.Helper()
	resp, err := http.Get(url) // loopback, and the test's own timeout bounds it
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("closing %s: %v", url, err)
		}
	}()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatalf("draining %s: %v", url, err)
	}
	return resp.StatusCode
}

// ---------------------------------------------------------------------
// the traversal
// ---------------------------------------------------------------------

// readSitePage loads one entry point, dumps it, then clicks every disclosure on
// it and dumps again after each — recording, for every target, whether the click
// actually opened anything.
func readSitePage(t *testing.T, browser, base string, e siteEntry) sitePage {
	t.Helper()
	ctx := browserContextFor(t, browser)
	url := base + "/" + e.name

	runCDP(t, ctx, chromedp.Navigate(url))
	desktopViewport(t, ctx)
	pollTrue(t, ctx, e.ready)
	if !evalBool(t, ctx, siteExtractorJS) {
		t.Fatalf("%s: the page-side extractor did not install", e.name)
	}

	page := sitePage{Entry: e.name, URL: url}
	// HOW MANY of each string, attribute and disclosure the richest snapshot so
	// far has held — not WHETHER one has been seen. The dedupe these counters
	// implement is across PASSES, where the same page re-reads the same DOM ~30
	// times; a page that renders one string in four places is not a repetition to
	// collapse but four things a visitor reads, and the collapse is what took the
	// release version out of this artifact and put a false finding in the gate's
	// report. See sitePass. A counter reaches the same volume as the old flag on
	// everything the flag was written for, because a pass that reveals no new
	// OCCURRENCE increments nothing.
	textSeen := map[string]int{}
	attrSeen := map[siteAttr]int{}
	detailsSeen := map[siteDetails]int{}
	seenTranscript := map[string]bool{}

	record := func(label, selector string, snap siteSnapshot) {
		pass := sitePass{
			Label:      label,
			Selector:   selector,
			Details:    snap.Details,
			Summaries:  snap.Summaries,
			Expandable: snap.Expandable,
			CodeTabs:   snap.CodeTabs,
		}
		// nth is this snapshot's running occurrence index for the value in hand.
		// The nth copy is new to the artifact exactly when no earlier snapshot
		// carried n of them, and it is appended where the tree walker found it,
		// so the neighbours a reader needs to tell one copy from another travel
		// with it.
		textNth := map[string]int{}
		for _, s := range snap.Text {
			textNth[s]++
			if textNth[s] <= textSeen[s] {
				continue
			}
			textSeen[s] = textNth[s]
			pass.NewText = append(pass.NewText, s)
			page.Text = append(page.Text, s)
		}
		attrNth := map[siteAttr]int{}
		for _, a := range snap.Attrs {
			attrNth[a]++
			if attrNth[a] <= attrSeen[a] {
				continue
			}
			attrSeen[a] = attrNth[a]
			pass.NewAttrs = append(pass.NewAttrs, a)
			page.Attributes = append(page.Attributes, a)
		}
		detailsNth := map[siteDetails]int{}
		for _, d := range snap.Details {
			detailsNth[d]++
			if detailsNth[d] <= detailsSeen[d] {
				continue
			}
			detailsSeen[d] = detailsNth[d]
			page.Details = append(page.Details, d)
		}
		// FIRST reading of a command's session wins, and a session with no lines
		// is not recorded as a reading at all. A closed card contributes no
		// pre.term-out, so the entry simply is not there until the card opens;
		// recording an empty one would let "the traversal never opened it" arrive
		// downstream as "the page depicts nothing", which sends a reader to the
		// wrong file.
		for _, tr := range snap.Transcripts {
			if len(tr.Lines) == 0 || seenTranscript[tr.Command] {
				continue
			}
			seenTranscript[tr.Command] = true
			page.Transcripts = append(page.Transcripts, tr)
		}
		page.Passes = append(page.Passes, pass)
	}

	load := siteSnapshotOf(t, ctx)
	page.Head = load.Head
	page.CLICommands = load.CLI
	record("load", "", load)

	var targets []siteTarget
	evalInto(t, ctx, `window.__dxTargets()`, &targets)
	t.Logf("%s: %d disclosure targets to traverse", e.name, len(targets))

	for _, tgt := range targets {
		quoted := strconv.Quote(tgt.Path)
		if !evalBool(t, ctx, `window.__dxClick(`+quoted+`)`) {
			t.Fatalf("%s: the element at path %s (%s, %q) vanished before it could be clicked — "+
				"its prose would be collected by nothing", e.name, tgt.Path, tgt.Selector, tgt.Label)
		}
		// Deterministic, and the failure names the element rather than an
		// expression: a target that never activates is a surface the dump does
		// not contain, which is the one outcome this traversal may not have.
		if !pollActivated(t, ctx, quoted) {
			t.Fatalf("%s: clicking %s (%s, %q) never opened it, so whatever it hides is missing "+
				"from the dump", e.name, tgt.Path, tgt.Selector, tgt.Label)
		}
		record(tgt.Selector+" "+tgt.Label, tgt.Selector, siteSnapshotOf(t, ctx))
	}
	return page
}

// pollActivated waits for siteActivatedJS to go true instead of reading it once:
// framer-motion mounts a tab's panel one animation after aria-selected flips.
// It returns the verdict rather than failing, so the caller can name the element
// that would not open — "condition never became true: (function (path) {…})"
// names a path and nothing a reader could act on.
func pollActivated(t *testing.T, ctx context.Context, quotedPath string) bool {
	t.Helper()
	var ok bool
	err := chromedp.Run(ctx, chromedp.Poll(`(`+siteActivatedJS+`)(`+quotedPath+`)`, &ok,
		chromedp.WithPollingInterval(40*time.Millisecond),
		chromedp.WithPollingTimeout(20*time.Second),
	))
	return err == nil && ok
}

func siteSnapshotOf(t *testing.T, ctx context.Context) siteSnapshot {
	t.Helper()
	var snap siteSnapshot
	evalInto(t, ctx, `window.__dxSnapshot()`, &snap)
	return snap
}

// ---------------------------------------------------------------------
// the artifact
// ---------------------------------------------------------------------

func siteTextPath(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("DOSSIERX_SITE_TEXT_OUT"); p != "" {
		return p
	}
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}
	return filepath.Join(root, "viewer-tests", siteTextFileName)
}

// siteTextTreeEnv names the tree the extraction is being driven for. See
// siteDump.Tree for why it is supplied rather than probed.
const siteTextTreeEnv = "DOSSIERX_SITE_TEXT_TREE"

// siteTextObjectName is the identity rule, applied on this producer's side the
// way every other gate producer applies it: forty hexadecimal digits and
// nothing else. A tag name is a mutable pointer, an abbreviation is a prefix,
// and either can mean a different release tomorrow than it means today.
var siteTextObjectName = regexp.MustCompile(`^[0-9a-f]{40}$`)

// siteTextTree resolves the tree stamp for this extraction, or fails.
//
// THE TWO VARIABLES IMPLY EACH OTHER, the same contract the render diff's
// flags keep (-render-diff-out and -render-diff-tree): a driver that sets the
// output path is collecting gate evidence and MUST say which release it is
// evidence of, and a driver that names a tree while writing to the default
// local path is confused about which of the two runs it is performing. Both
// halves fail rather than guess, because the guessable answers are exactly the
// wrong ones — an unstamped collected artifact is the pre-stamp defect back
// again, and a locally stamped one looks collectable when it was built from
// whatever this laptop had checked out.
//
// With NEITHER set this is a local run: the artifact lands beside the suite,
// git-ignored, and its stamp is the empty string — which run.sh `record`
// refuses ("not a full 40-digit object name"), so a local artifact that drifts
// into gate/ cannot be recorded as a release's evidence.
func siteTextTree(t *testing.T) string {
	t.Helper()
	tree := os.Getenv(siteTextTreeEnv)
	out := os.Getenv("DOSSIERX_SITE_TEXT_OUT")
	switch {
	case out != "" && tree == "":
		t.Fatalf("DOSSIERX_SITE_TEXT_OUT is set and %s is not. A collected extraction that cannot say "+
			"which tree its build came from records into the gate exactly as cleanly as one left over from "+
			"the previous release — that unaccountability is the defect the stamp exists to close. Pass the "+
			"same tree the rest of the run uses: %s=$(git rev-parse \"HEAD^{tree}\")", siteTextTreeEnv, siteTextTreeEnv)
	case out == "" && tree != "":
		t.Fatalf("%s is set and DOSSIERX_SITE_TEXT_OUT is not. A tree was named but the artifact is about "+
			"to be written to the suite's local default path, which no gate run reads; one of the two "+
			"intentions is a mistake, and this producer will not pick which", siteTextTreeEnv)
	case tree != "" && !siteTextObjectName.MatchString(tree):
		t.Fatalf("%s is %q, which is not a full 40-digit object name. A tag is a mutable pointer and an "+
			"abbreviation is a prefix; a stamp made of either cannot be checked against the run that records it",
			siteTextTreeEnv, tree)
	}
	return tree
}

func writeSiteText(t *testing.T, dump siteDump) string {
	t.Helper()
	b, err := json.MarshalIndent(dump, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s: %v", siteTextFileName, err)
	}
	out := siteTextPath(t)
	if err := os.WriteFile(out, append(b, '\n'), 0o644); err != nil {
		t.Fatalf("write %s: %v", out, err)
	}
	t.Logf("wrote %s (%d bytes)", out, len(b)+1)
	return out
}

// TestSiteTextProvenanceComesFirst is the direct check on the thing the
// record-time cross-check silently depends on, and it needs no browser: field
// ORDER in the marshalled artifact.
//
// scripts/gate-stage2/run.sh reads this document with json_scalar, a
// deliberately tiny awk reader that is not a JSON parser: it takes the FIRST
// `"tree"` string on any line and exits. Its answer is therefore POSITIONAL —
// the stamp must be the first key of the document, ahead of every page's
// captured text, or a sentence the site happens to render that looks like a
// provenance line would be read as the artifact's identity. The pages are
// exactly the untrusted half here: they are whatever the DOM said, and this
// fixture poisons one with a stamp-shaped string to prove the reader cannot
// reach it. This is gate/render-diff.json's promise
// (TestRenderDiffCaptureProvenanceComesFirst), made by this producer for its
// own document.
func TestSiteTextProvenanceComesFirst(t *testing.T) {
	tree := strings.Repeat("a", 40)
	poison := "the release delta records \"tree\": \"" + strings.Repeat("d", 40) + "\" for the previous build"
	dump := siteDump{
		Tree:        tree,
		GeneratedBy: "viewer-tests/site_dom_test.go",
		Toolchain:   siteToolchain{NodeVersion: "v24.0.0", NPMVersion: "11.0.0"},
		Pages: []sitePage{{
			Entry: "index.html",
			URL:   "http://127.0.0.1/index.html",
			Head:  []string{"title: DossierX"},
			Text:  []string{poison},
		}},
	}
	data, err := json.MarshalIndent(dump, "", "  ")
	if err != nil {
		t.Fatalf("marshal the dump: %v", err)
	}
	doc := string(data) + "\n"

	// The top-level key order, read back with a real decoder. encoding/json
	// preserves struct field order, so this pins the struct's shape: a field
	// added above Tree, or Tree moved for tidiness, reds this before it reds a
	// release.
	dec := json.NewDecoder(strings.NewReader(doc))
	if _, err := dec.Token(); err != nil {
		t.Fatalf("read the opening brace: %v", err)
	}
	var keys []string
	for dec.More() {
		tok, tokErr := dec.Token()
		if tokErr != nil {
			t.Fatalf("read a key: %v", tokErr)
		}
		keys = append(keys, tok.(string))
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			t.Fatalf("skip a value: %v", err)
		}
	}
	if len(keys) == 0 || keys[0] != "tree" {
		t.Fatalf("the document's keys are %v; run.sh reads it with an awk one-liner that takes the FIRST "+
			"\"tree\" it sees on any line, so the stamp must be the first key. Anywhere else, the record-time "+
			"cross-check compares this run against whatever a page's own text happens to say, and an extraction "+
			"from another release records cleanly.", keys)
	}

	// And the first-match reader itself, mimicked the way json_scalar reads:
	// line by line, first `"tree": "<value>"` wins. The poisoned page text must
	// lose to the stamp — note that JSON escaping alone does not defend it,
	// so the position is the whole defence.
	re := regexp.MustCompile(`"tree"[ \t]*:[ \t]*"([^"]*)"`)
	first := ""
	for _, line := range strings.Split(doc, "\n") {
		if m := re.FindStringSubmatch(line); m != nil {
			first = m[1]
			break
		}
	}
	if first != tree {
		t.Fatalf("the first \"tree\" scalar in the document is %q, want the driver's tree %q; whatever "+
			"produced that value — a page's text, a reordered struct — is what run.sh would check the run against", first, tree)
	}
}

// surfaceCommandCount reads counts.commands out of the committed surface.json —
// the mechanically extracted truth the site's command list is measured against.
func surfaceCommandCount(t *testing.T) int {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(root, "surface.json"))
	if err != nil {
		t.Fatalf("read surface.json: %v", err)
	}
	var doc struct {
		Counts struct {
			Commands int `json:"commands"`
		} `json:"counts"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("parse surface.json: %v", err)
	}
	if doc.Counts.Commands == 0 {
		t.Fatal("surface.json reports counts.commands == 0, so comparing the site against it would assert nothing")
	}
	return doc.Counts.Commands
}

// reCodeExamples anchors on the field TabbedCode is fed from. It is matched
// against code bytes only (tsScan's inert mask), so the same characters sitting
// inside one of content.ts's embedded code samples are not mistaken for a
// declaration.
var reCodeExamples = regexp.MustCompile(`codeExamples: \[`)

// declaredCodeExampleGroups is the mechanical truth condition 2 is measured
// against: the size of every `codeExamples` array in content.ts, in declaration
// order. TabbedCode renders exactly one `.codetabs` per array, and a tablist
// only when the array holds more than one example — so these numbers say both
// how many tab groups the rendered DOM must contain and how many tabs each of
// them must expose.
//
// Counting them is what stops condition 2 from being satisfied by an empty
// traversal. Its per-group comparison is "panels seen >= tabs seen", which is
// trivially true of a page where the extraction saw no groups and no tabs at
// all: a class rename or a move off ARIA roles deletes every code sample from
// the artifact while the subtest stays green. A count read from the source
// cannot be zeroed by a change to the markup.
func declaredCodeExampleGroups(t *testing.T) []int {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}
	path := siteContentPath(root)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	src := string(b)
	_, inert := tsScan(src)

	var sizes []int
	for _, loc := range reCodeExamples.FindAllStringIndex(src, -1) {
		if inert[loc[0]] {
			continue
		}
		open := loc[1] - 1 // the `[` the match ends on
		end := matchBracket(src, inert, open)
		if end < 0 {
			t.Fatalf("%s:%d: a codeExamples array is never closed", path, 1+strings.Count(src[:loc[0]], "\n"))
		}
		sizes = append(sizes, topLevelObjects(src, inert, open+1, end))
	}
	if len(sizes) == 0 {
		t.Fatalf("%s declares no codeExamples arrays, so comparing the site's tab groups against it "+
			"would assert nothing. Two sections carry them today (claims, code-links).", path)
	}
	return sizes
}

// ---------------------------------------------------------------------
// the source-derived prose floor
// ---------------------------------------------------------------------

// siteProse is one sentence a component hard-codes, with the place it was
// declared — so a failure names a file and a line rather than a loose string,
// and the reader can go and see whether the site stopped rendering it or this
// extraction stopped reaching it.
type siteProse struct {
	Where string // site/src/…:LINE, relative to site/src
	Kind  string // "attribute" or "text"
	Text  string
}

// reProseAttr matches a JSX attribute that carries prose to a reader. The three
// ARIA/HTML ones are what __dxSnapshot collects; `label` is in the list because
// that is how a component FORWARDS one, and it is the form the two claims this
// whole file was built around actually take — AgentCompat writes
// `label="One embedded markdown bundle, exported into three places"` on
// ConnectorFan, which spends it as an aria-label two frames down. A scan that
// read only `aria-label=` would find neither of them.
//
// The leading class rejects `data-label=` and any other hyphenated or prefixed
// attribute that merely ends in one of these names; `aria-labelledby={…}` cannot
// match at all, since the name must be followed by `="`.
var reProseAttr = regexp.MustCompile(`(?:^|[^-\w])(?:aria-label|title|alt|label)="([^"]*)"`)

var reWhitespaceRun = regexp.MustCompile(`\s+`)

// tidySpace collapses runs of whitespace the way __dxTidy does in the page, so
// a sentence Prettier broke across three source lines compares equal to the one
// text node the browser built out of it.
func tidySpace(s string) string { return strings.TrimSpace(reWhitespaceRun.ReplaceAllString(s, " ")) }

// The thresholds a candidate must clear. They are deliberately blunt: this scan
// is a FLOOR, and a sentence it declines to claim is coverage not gained rather
// than a false alarm raised. "▶", "1", "Reply" and `dossierx check` are all
// prose in some sense and none of them distinguishes a working extraction from a
// broken one.
const (
	proseAttrMinLen  = 12
	proseTextMinLen  = 24
	proseTextMinWord = 4
)

// scanSiteProse reads the .tsx components under srcDir and returns every
// hard-coded sentence in them.
//
// It reads COMPONENTS only, not content.ts: content.ts is the copy file, its
// strings are markdown, transcripts and embedded YAML that render through
// Markdown/CodeBlock in forms this scan could not predict — and it is the file
// another lane of a gate run is most likely to be editing. The components are
// where prose gets hard-coded next to the markup that hides it, which is the
// class this floor exists for.
//
// TWO KINDS, because the two failures are different. An ATTRIBUTE literal is
// prose with no text node behind it at all; a JSX TEXT run is prose that has a
// text node, which some interaction may or may not have mounted. The dump has to
// contain both.
func scanSiteProse(srcDir string) ([]siteProse, error) {
	var out []siteProse
	err := filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".tsx" {
			return nil
		}
		b, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		src := string(b)
		_, inert := tsScan(src)

		rel := path
		if r, relErr := filepath.Rel(srcDir, path); relErr == nil {
			rel = filepath.ToSlash(r)
		}
		at := func(i int) string { return fmt.Sprintf("%s:%d", rel, 1+strings.Count(src[:i], "\n")) }

		for _, loc := range reProseAttr.FindAllStringSubmatchIndex(src, -1) {
			if inert[loc[0]] {
				continue
			}
			v := tidySpace(src[loc[2]:loc[3]])
			if len(v) < proseAttrMinLen || !strings.Contains(v, " ") {
				continue
			}
			out = append(out, siteProse{Where: at(loc[0]), Kind: "attribute", Text: v})
		}
		out = append(out, jsxTextProse(src, inert, at)...)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan %s for prose: %w", srcDir, err)
	}
	return out, nil
}

// jsxTextProse pulls the sentences out of a .tsx file's JSX text — the runs
// between a `>` and the next `<` that a lexer did not mark inert.
//
// It is not a parser and does not need to be, because the direction it errs in
// is the safe one. A run holding any of `{}=<>` is thrown away, which discards
// every interpolated sentence (`no command matches “{query}”`) and every stretch
// of ordinary code an arrow function's `=>` opened — at the cost of also
// discarding a sentence containing an apostrophe, since tsScan reads that as a
// string opening and marks the rest of the line inert, so the run runs on past
// the closing tag and is rejected for containing one.
//
// Everything that survives is a whole sentence a component states in its own
// bytes. Missing one is coverage this floor does not claim; claiming one that
// the site cannot render would be a red build nobody could fix, which is the
// failure a gate may not have.
func jsxTextProse(src string, inert []bool, at func(int) string) []siteProse {
	var out []siteProse
	for i := 0; i < len(src); i++ {
		if src[i] != '>' || inert[i] {
			continue
		}
		start := i + 1
		j := start
		for j < len(src) && !(src[j] == '<' && !inert[j]) {
			j++
		}
		i = j - 1 // the loop's i++ lands on the `<` that closed this run

		t := tidySpace(src[start:j])
		if len(t) < proseTextMinLen || strings.ContainsAny(t, "{}=<>") {
			continue
		}
		if len(strings.Fields(t)) < proseTextMinWord {
			continue
		}
		if r := []rune(t)[0]; !unicode.IsLetter(r) {
			continue
		}
		out = append(out, siteProse{Where: at(start), Kind: "text", Text: t})
	}
	return out
}

// declaredSiteProse is scanSiteProse against the tree under test, with the
// anti-vacuity guard that matters most: a scan that returns nothing agrees with
// every dump ever produced. Both kinds must be present, because they are the
// two halves of condition 6 and either one going quiet is this floor breaking
// rather than the site changing.
func declaredSiteProse(t *testing.T) []siteProse {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}
	srcDir := filepath.Join(root, "site", "src")
	prose, err := scanSiteProse(srcDir)
	if err != nil {
		t.Fatal(err)
	}
	attrs, texts := 0, 0
	for _, p := range prose {
		if p.Kind == "attribute" {
			attrs++
		} else {
			texts++
		}
	}
	if attrs == 0 || texts == 0 {
		t.Fatalf("the prose scan of %s found %d hard-coded attribute literal(s) and %d JSX sentence(s); "+
			"both must be non-zero or condition 6 is measured against an empty floor and passes over "+
			"nothing. The components carry both today — Nav, Cli, TabbedCode, SurfacePair, TwoSurfaces "+
			"and AgentCompat all write attribute prose, and Hero, Claims, FileTree and SurfacePair all "+
			"write sentences into JSX. If they still do, this scan has stopped reading them.",
			srcDir, attrs, texts)
	}
	return prose
}

// proseMissingFrom returns the declared sentences that no string in pool
// contains. Containment rather than equality, and pool rather than one field:
// the browser splits a sentence across text nodes wherever an inline element
// sits inside it, and a component's `label` may reach the reader as an attribute
// on one page and as visible text on another. What is being asserted is that the
// artifact CARRIES the sentence, not where in it the sentence landed.
func proseMissingFrom(declared []siteProse, pool []string) []siteProse {
	var missing []siteProse
	for _, p := range declared {
		found := false
		for _, s := range pool {
			if strings.Contains(s, p.Text) {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, p)
		}
	}
	return missing
}

func summariseProse(missing []siteProse, n int) string {
	var b strings.Builder
	for i, p := range missing {
		if i == n {
			fmt.Fprintf(&b, "\n  … (%d more)", len(missing)-n)
			break
		}
		fmt.Fprintf(&b, "\n  %s [%s] %q", p.Where, p.Kind, p.Text)
	}
	return b.String()
}

func summarise(vals []string, n int) string {
	if len(vals) <= n {
		return fmt.Sprint(vals)
	}
	return fmt.Sprintf("%v … (%d more)", vals[:n], len(vals)-n)
}

// ---------------------------------------------------------------------
// the extraction, and the seven ways it is allowed to fail
// ---------------------------------------------------------------------

// TestSiteRenderedDOMExtraction builds the site, reads both entry points as
// rendered DOM, writes site-text.json, and then asserts that the extraction
// really covered what it claims to have covered.
//
// Every subtest below is an INTEGRITY check on the dump, not a judgement of the
// site's copy — the gate's agents make that judgement, and they can only make it
// against an artifact that is known to be complete. A dump that quietly omitted
// the CLI table, a tab panel, the head metadata, the aria-label prose or a
// closed disclosure would let a false statement through while reporting green,
// which is the failure this file exists to make impossible.
func TestSiteRenderedDOMExtraction(t *testing.T) {
	// The provenance first, before anything is paid for: a gate driver that
	// forgot the tree should hear about it now, not after a build and a
	// browser traversal whose artifact would be refused at record time anyway.
	tree := siteTextTree(t)
	// Condition 5, next: with no browser there is no rendered DOM, and every
	// other condition here would be a pass over zero assertions.
	browser := requireSiteBrowser(t)
	// And the same rule applied to the other two prerequisites: no node, no
	// npm, no build, no DOM. Resolved before anything else so the failure names
	// the missing tool rather than surfacing three steps later as a vite error.
	tc := requireSiteToolchain(t)

	dist := buildSite(t, tc)
	srv := httptest.NewServer(http.FileServer(http.Dir(dist)))
	t.Cleanup(srv.Close)
	assertEntryPointsReachable(t, srv.URL)

	dump := siteDump{Tree: tree, GeneratedBy: "viewer-tests/site_dom_test.go", Toolchain: tc}
	for _, e := range siteEntries {
		dump.Pages = append(dump.Pages, readSitePage(t, browser, srv.URL, e))
	}
	out := writeSiteText(t, dump)

	t.Run("1 the cli section carries every command surface.json counts", func(t *testing.T) {
		want := surfaceCommandCount(t)
		got := dump.page(t, "index.html").CLICommands
		if len(got) < want {
			t.Fatalf("the #cli section yielded %d command names but surface.json counts %d commands: %s\n"+
				"Either the site is no longer showing the whole CLI, or the extraction stopped seeing it.",
				len(got), want, summarise(got, 25))
		}
	})

	t.Run("2 every code tab group was found, and every panel mounted and read", func(t *testing.T) {
		// What the source says must be there. Both halves below are compared
		// against it rather than against the traversal's own findings, because a
		// traversal that found nothing agrees with itself perfectly.
		declared := declaredCodeExampleGroups(t)
		want := make([]int, 0, len(declared))
		for _, n := range declared {
			if n > 1 {
				want = append(want, n)
			} else {
				want = append(want, 0) // TabbedCode omits the tablist for a lone example
			}
		}

		var got []int
		for _, page := range dump.Pages {
			// seen is kept apart from tabs so that a group with NO tabs still
			// counts as a group. Reading the group count off tabs would report a
			// site whose tab vocabulary changed as one with no tab groups, which
			// is a true failure told as the wrong story.
			seen := map[int]bool{}
			tabs := map[int]int{}
			panels := map[int]map[string]bool{}
			for _, pass := range page.Passes {
				for _, ct := range pass.CodeTabs {
					seen[ct.Index] = true
					if len(ct.Tabs) > tabs[ct.Index] {
						tabs[ct.Index] = len(ct.Tabs)
					}
					if panels[ct.Index] == nil {
						panels[ct.Index] = map[string]bool{}
					}
					for _, p := range ct.Panels {
						panels[ct.Index][p] = true
					}
				}
			}
			for idx, wantPanels := range tabs {
				if gotPanels := len(panels[idx]); gotPanels < wantPanels {
					t.Fatalf("%s: .codetabs #%d has %d tabs but the traversal only ever saw %d panel(s). "+
						"TabbedCode mounts one panel at a time, so an unclicked tab is code prose that "+
						"reaches the reader and not the gate.", page.Entry, idx, wantPanels, gotPanels)
				}
			}
			for idx := 0; idx < len(seen); idx++ {
				got = append(got, tabs[idx])
			}
		}

		// Sorted, not positional: content.ts's declaration order and App.tsx's
		// section order are two independent decisions, and this condition is
		// about coverage rather than about which group is which.
		sort.Ints(got)
		sort.Ints(want)
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatalf("the traversal saw %d code tab group(s) with tab counts %v, but content.ts declares "+
				"%d codeExamples array(s), which must render as %v.\nThe comparison above is "+
				"\"panels seen >= tabs seen\", which a traversal that saw NO groups satisfies without "+
				"asserting anything — renaming .codetabs, or moving the tabs off role=\"tab\" and "+
				"role=\"tabpanel\", drops every code sample out of the artifact and past every other "+
				"condition here. Either the site stopped rendering them, or this extraction stopped "+
				"seeing them; both are a FAILED gate.", len(got), got, len(declared), want)
		}
	})

	t.Run("3 both entry points yielded head metadata", func(t *testing.T) {
		for _, page := range dump.Pages {
			if len(page.Head) == 0 {
				t.Fatalf("%s: the head block is empty. The <title> and meta description are client-facing "+
					"prose (surfaces.yaml calls them part of this surface), and an empty head means they "+
					"were never read.", page.Entry)
			}
			t.Logf("%s head: %v", page.Entry, page.Head)
		}
	})

	t.Run("4 site-text.json exists and is not empty", func(t *testing.T) {
		info, err := os.Stat(out)
		if err != nil {
			t.Fatalf("stat %s: %v", out, err)
		}
		if info.Size() == 0 {
			t.Fatalf("%s is zero bytes", out)
		}
		b, err := os.ReadFile(out)
		if err != nil {
			t.Fatalf("read back %s: %v", out, err)
		}
		var round siteDump
		if err := json.Unmarshal(b, &round); err != nil {
			t.Fatalf("%s is not readable JSON: %v", out, err)
		}
		if len(round.Pages) != len(siteEntries) {
			t.Fatalf("%s holds %d pages, want %d", out, len(round.Pages), len(siteEntries))
		}
		// The provenance has to survive the round trip too. A dump that does not
		// say which toolchain built it is a claim about the published site with
		// the one fact that would let a reader check it left out.
		if round.Toolchain.NodeVersion == "" || round.Toolchain.NPMVersion == "" {
			t.Fatalf("%s carries no toolchain stamp (node=%q npm=%q). The artifact has to say what "+
				"produced the DOM it records; site_toolchain_test.go checks that toolchain against the "+
				"publish workflow, and neither is worth anything if the dump does not carry it",
				out, round.Toolchain.NodeVersion, round.Toolchain.NPMVersion)
		}
		// The tree stamp has to survive the round trip too — it is the one fact
		// that lets run.sh `record` tell this release's extraction from one
		// left over by the last, and a stamp that was set going in and blank
		// coming back is a marshalling defect the gate would read as a stale
		// artifact.
		if round.Tree != tree {
			t.Fatalf("%s records tree %q and this extraction was driven for %q; the stamp did not survive "+
				"the round trip, and record-time freshness would judge the wrong value", out, round.Tree, tree)
		}
		for _, page := range round.Pages {
			if len(page.Text) == 0 {
				t.Fatalf("%s: %s carries no text at all", out, page.Entry)
			}
		}
	})

	t.Run("5 the browser was resolved strictly", func(t *testing.T) {
		// requireSiteBrowser already fatalled if it was not; restating it here
		// keeps the condition visible in the run's output rather than implied
		// by the absence of a failure.
		if browser == "" {
			t.Fatal("no browser was resolved")
		}
	})

	t.Run("6 attribute prose and closed disclosures were read", func(t *testing.T) {
		labels := 0
		for _, page := range dump.Pages {
			for _, a := range page.Attributes {
				if a.Attr == "aria-label" {
					labels++
				}
			}
		}
		if labels == 0 {
			t.Fatalf("the dump contains no aria-label entries. AgentCompat.tsx states engine claims on "+
				"path-only SVGs where no text node exists, so a dump with none of them is reading "+
				"visible text and calling it the DOM. Artifact: %s", out)
		}

		// The floor, read out of site/src. The counter above is a global one:
		// index.html carries eighteen aria-labels and only two of them are
		// AgentCompat's engine claims, so blanking BOTH of those leaves it at
		// sixteen and the dump silently stops carrying the two sentences this
		// half of the condition is named after. A count cannot notice that. A
		// list of the sentences the source declares can, and it stays honest
		// when the site grows: the check is not "how many" but "which".
		var pool []string
		for _, page := range dump.Pages {
			pool = append(pool, page.Text...)
			for _, a := range page.Attributes {
				pool = append(pool, a.Value)
			}
		}
		declared := declaredSiteProse(t)
		if missing := proseMissingFrom(declared, pool); len(missing) > 0 {
			t.Fatalf("%d of the %d sentence(s) hard-coded in site/src's components are absent from the "+
				"dump:%s\nEach one reaches a visitor and none of them reaches the gate. An attribute "+
				"sentence means the aria-label/title/alt collection stopped seeing it; a text sentence "+
				"means whatever mounts it was never opened — add its control to __dxTargets' selector "+
				"list. Artifact: %s",
				len(missing), len(declared), summariseProse(missing, 12), out)
		}
		t.Logf("all %d hard-coded sentence(s) declared in site/src's components are present in the dump",
			len(declared))

		// EVERY pass, not the union of them. The union is satisfied the moment
		// the traversal clicks a <summary> open, which proves only that an OPEN
		// disclosure has readable contents — a dump built on innerText passes
		// that and still loses the closed state, which is the state the page
		// ships in. Asserting it pass by pass includes the load pass, where
		// every disclosure on this site is shut.
		for _, page := range dump.Pages {
			for _, pass := range page.Passes {
				bodies := 0
				for _, d := range pass.Details {
					if d.Body != "" {
						bodies++
					}
				}
				if bodies < pass.Summaries {
					t.Fatalf("%s, after %q: %d <summary> element(s) but only %d <details> body/bodies. "+
						"A closed <details> keeps its children in the DOM and out of innerText — "+
						"reading textContent is the whole point.",
						page.Entry, pass.Label, pass.Summaries, bodies)
				}
			}
		}
	})

	// Condition 7, the general form of the conditional-render class — and the
	// one condition this file does not implement as written. See the DISCLOSURE
	// in the header: "nothing carries aria-expanded=false at the end" is not a
	// state this site can be driven into, because Cli's accordion, Hero's card
	// stack and Nav's menu are all SINGLE-OPEN, so asserting it would be
	// asserting a bug that no fix could clear. What IS both achievable and
	// equivalent in force is: no element that can expand finished the traversal
	// never having been expanded. Every current offender satisfies it only
	// because the driver clicks it, and the next component of that shape fails on
	// the day it is written rather than on the day someone audits for it —
	// including one that appears only after another disclosure opens, since the
	// check reads every pass and not just the first.
	t.Run("7 every expandable element was opened at least once (substituted — see header)", func(t *testing.T) {
		total := 0
		for _, page := range dump.Pages {
			ever := map[string]bool{}
			label := map[string]string{}
			for _, pass := range page.Passes {
				for _, x := range pass.Expandable {
					label[x.Path] = x.Label
					ever[x.Path] = ever[x.Path] || x.Expanded
				}
			}
			total += len(ever)
			var never []string
			for path, opened := range ever {
				if !opened {
					never = append(never, fmt.Sprintf("%s (%q)", path, label[path]))
				}
			}
			if len(never) > 0 {
				t.Fatalf("%s: %d element(s) carrying aria-expanded were never seen expanded, so whatever "+
					"they mount is absent from the dump: %s\nAdd them to __dxTargets' selector list, or "+
					"explain why their contents are not prose.", page.Entry, len(never), summarise(never, 10))
			}
		}
		if total == 0 {
			t.Fatal("no element on either page carries aria-expanded, so the assertion above ran over " +
				"nothing. Cli, Hero and Nav all do today — the extraction has stopped seeing them.")
		}
	})

	// Condition 8 is the first assertion in this file that JUDGES THE SITE'S COPY
	// rather than the dump's integrity, and it is here rather than beside the
	// engine's tests for the reason CLAUDE.md gives: verify the thing the user
	// sees, not the thing you edited.
	//
	// WHAT IT REPLACES. This claim used to be made in the root module against
	// site/src/content.ts — a regexp pulled the `example` template literal out of
	// the source, another substituted the interpolations it recognised, and the
	// result was compared to a binary's output. It was defeated four separate
	// times without a single assertion going red, and every defeat was the same
	// shape: the file's TEXT is not the page. A five-line prose comment satisfied
	// a substring search. A commented-out declaration satisfied it again. A second
	// declaration after the good one satisfied it a third time, while the bundler
	// evaluated the second. None of those survives a read of the rendered DOM: a
	// comment renders nothing, a commented-out entry renders nothing, and a
	// contradictory later declaration renders — and is what is compared.
	t.Run("8 the `version` transcript depicts what the released binary prints", func(t *testing.T) {
		assertVersionTranscriptIsRealOutput(t, dump)
	})

	// Condition 9 is not one of the gate's numbered conditions. It is here
	// because the dump lost something none of the eight above could notice, and
	// the gate then published the loss as a fact about the site: with the
	// cross-pass dedupe taken per string, the hero's copy of the release version
	// arrived first and the release-history intro's and the timeline's copies
	// were dropped as already-seen. Conditions 1-7 are integrity checks that all
	// stayed green — nothing they count changed — and condition 8 reads the
	// `version` CARD, a different string in a different section. The finding that
	// came back said the landing page names no version anywhere. It was wrong,
	// and it was wrong because of this file.
	//
	// So this condition asserts the one property the artifact silently did not
	// have: a string a page renders more than once reaches the artifact more than
	// once, positioned so a reader can tell the copies apart.
	t.Run("9 the release version survives in every place the page renders it", func(t *testing.T) {
		assertRepeatedVersionSurvivesTheDump(t, dump)
	})
}

// ---------------------------------------------------------------------
// condition 9 — the repetition the dump used to eat
// ---------------------------------------------------------------------

// releaseIntroVersionExpr is how content.ts states the current version inside
// the release-history intro: an interpolation wrapped in markdown bold, which
// the renderer turns into a <strong> and therefore into a text node of its own,
// separate from the sentence that introduces it. Both halves of that split are
// what condition 9 anchors on.
const releaseIntroVersionExpr = "**${latestRelease.version}**"

// reReleaseDate reads an entry's `date` the way reReleaseEntry reads its
// version, and for the same reason: the timeline renders the two as adjacent
// siblings inside .release__head, so the date is the anchor that says WHICH of a
// page's several copies of the version string is the timeline's.
var reReleaseDate = regexp.MustCompile(`(?m)^[ \t]*date: "(\d{4}-\d{2}-\d{2})",(?:[ \t]*//.*)?$`)

// currentReleaseDate is the date of the entry ReleaseTimeline badges "latest".
//
// It cross-checks the count of date lines against the count of version lines
// loadReleasesArray already parsed, for the reason that file gives about
// versions: a date line this regex cannot read does not fail, it silently makes
// the PREVIOUS release's date the anchor, and the condition below would then
// look for a neighbour pair the page never renders and report the dump as lossy
// when it is whole. A red build nobody can fix is the failure a gate may not
// have.
func currentReleaseDate(t *testing.T, ra releasesArray) string {
	t.Helper()
	var dates []string
	for _, m := range reReleaseDate.FindAllStringSubmatch(ra.src[ra.start:ra.end], -1) {
		dates = append(dates, m[1])
	}
	if len(dates) != len(ra.versions) {
		t.Fatalf("%s: the releases array holds %d readable `version:` line(s) but %d readable `date:` "+
			"line(s) (%v).\nCondition 9 identifies the timeline's copy of the version string by the date "+
			"rendered beside it, and a date line this suite cannot parse would move that anchor to the "+
			"wrong release — which reads as a lossy dump rather than as an unreadable source line.\n"+
			"Restore the field to `date: \"YYYY-MM-DD\",` or teach reReleaseDate the new shape",
			ra.path, len(ra.versions), len(dates), dates)
	}
	return dates[len(dates)-1]
}

// releaseIntroLeadIn is the sentence content.ts writes immediately before the
// bolded version in the release-history intro, as the page will render it.
//
// It is DERIVED from content.ts rather than typed here, which is what keeps
// condition 9 from going red on a copy edit: reword the intro and both sides of
// the comparison move together. Only removing the bolded interpolation — the
// thing that makes the version its own text node — breaks it, and that is a
// change to the shape this condition is about, so it is a red build with a real
// answer rather than a formatting tax.
//
// The cut is at the last markdown, interpolation or newline boundary before the
// version, so what remains is a run of plain prose that survives rendering
// unchanged. Anything shorter than a clause is refused: a two-word anchor would
// match half the page and the condition would pass on a dump that had kept
// nothing.
func releaseIntroLeadIn(t *testing.T, ra releasesArray) string {
	t.Helper()
	at := strings.Index(ra.src, releaseIntroVersionExpr)
	if at < 0 {
		t.Fatalf("%s no longer writes %s. The release-history intro states the current version there, and "+
			"the markdown bold is what gives it a text node of its own — which is the copy this condition "+
			"checks survived the dump. If the intro states the version some other way, this anchor has to "+
			"be re-derived from the new shape; if it no longer states it at all, that is a site change to "+
			"make deliberately, since both pages carry that sentence today",
			ra.path, releaseIntroVersionExpr)
	}
	before := ra.src[:at]
	lead := tidySpace(before[strings.LastIndexAny(before, "`*}$\n")+1:])
	if len(lead) < proseTextMinLen || len(strings.Fields(lead)) < proseTextMinWord {
		t.Fatalf("%s: the prose immediately before %s reads %q, which is too short to identify one "+
			"paragraph on a page. Condition 9 would then match anywhere and assert nothing",
			ra.path, releaseIntroVersionExpr, lead)
	}
	return lead
}

// occurrencesOf is every index at which pool holds want, where indexOfString
// gives only the first. The plural is the point of this condition: "is it in
// there" is exactly the question the collapsed dump answered YES to while three
// of the four copies were gone.
func occurrencesOf(pool []string, want string) []int {
	var at []int
	for i, got := range pool {
		if got == want {
			at = append(at, i)
		}
	}
	return at
}

// neighbourhood is the dump around one position, for a failure a human can read
// without opening the artifact.
func neighbourhood(pool []string, at, span int) string {
	lo, hi := at-span, at+span+1
	if lo < 0 {
		lo = 0
	}
	if hi > len(pool) {
		hi = len(pool)
	}
	var b strings.Builder
	for i := lo; i < hi; i++ {
		marker := "   "
		if i == at {
			marker = ">> "
		}
		fmt.Fprintf(&b, "\n  %s[%d] %q", marker, i, pool[i])
	}
	return b.String()
}

// assertRepeatedVersionSurvivesTheDump is condition 9.
//
// THE SITE RENDERS THE CURRENT VERSION IN SEVERAL PLACES ON ONE PAGE, all of
// them derived from one array so they cannot disagree: index.html spells it in
// the hero kicker, in the hero badge list, in the release-history intro's bold
// span, and as the timeline entry's version label. That is a design decision of
// the site's, and it is exactly the input the artifact used to be unable to
// carry.
//
// TWO OF THOSE PLACES ARE IDENTIFIED HERE, and each by a neighbour derived from
// content.ts rather than by its ordinal in the dump: the INTRO copy is the one
// whose preceding text node is the sentence that introduces it, and the TIMELINE
// copy is the one whose following text node is the release's date, because
// ReleaseTimeline renders .release__version and .release__date as adjacent
// siblings. An ordinal would be satisfied by any three copies in any order,
// which is a count again.
//
// AND THE SHADOW IS ASSERTED TOO, on index.html. This check only proves the
// G4FIX loss cannot recur while some element still emits the version BEFORE the
// intro and the timeline do — that earlier copy is what the collapse kept and
// the later two are what it deleted. If the hero stops rendering the version,
// the two positions below can be found by an extraction that is lossy again, so
// the absence of the shadow fails here rather than quietly weakening the
// condition to nothing.
func assertRepeatedVersionSurvivesTheDump(t *testing.T, dump siteDump) {
	t.Helper()

	ra := repoReleases(t)
	tag := ra.current()
	date := currentReleaseDate(t, ra)
	lead := releaseIntroLeadIn(t, ra)

	// Both pages: they share the section, so they share the intro sentence and
	// the timeline, and a dump that lost the repetition lost it on both.
	for _, entry := range []string{"index.html", "releases.html"} {
		page := dump.page(t, entry)
		at := occurrencesOf(page.Text, tag)
		if len(at) == 0 {
			t.Fatalf("%s: the dump does not carry %q as a text node at all, though content.ts declares it "+
				"the current release and this page renders it in the release-history intro and in the "+
				"timeline. Either the site stopped saying which release it is, or the extraction stopped "+
				"carrying it — and the second is what the gate reads as the first.\n%s",
				entry, tag, summarise(page.Text, 40))
		}

		intro, timeline := -1, -1
		for _, i := range at {
			if intro < 0 && i > 0 && strings.HasSuffix(page.Text[i-1], lead) {
				intro = i
			}
			if timeline < 0 && i+1 < len(page.Text) && page.Text[i+1] == date {
				timeline = i
			}
		}

		if intro < 0 {
			t.Fatalf("%s: %q appears %d time(s) in the dump, and none of them follows the release-history "+
				"intro's sentence %q.\nThat sentence and the bolded version are one paragraph the reader "+
				"sees; in the artifact the version is a text node of its own, so losing it leaves the "+
				"sentence ending in mid-air and a prose agent reading this file cannot tell which release "+
				"the page names. If the copies collapsed, the cross-pass dedupe in readSitePage is counting "+
				"strings again instead of occurrences. Copies found:%s",
				entry, tag, len(at), lead, neighbourhood(page.Text, at[0], 2))
		}
		if timeline < 0 {
			t.Fatalf("%s: %q appears %d time(s) in the dump, and none of them is followed by %q, the "+
				"current release's date.\nReleaseTimeline renders .release__version and .release__date as "+
				"adjacent siblings, so the timeline's copy of the version has that date immediately after "+
				"it. Without it the artifact carries a release history whose newest entry has no version "+
				"label — which is a site defect if the page really renders none, and a lossy extraction if "+
				"it does. Copies found:%s",
				entry, tag, len(at), date, neighbourhood(page.Text, at[len(at)-1], 3))
		}
		if intro == timeline {
			t.Fatalf("%s: one copy of %q at index %d is answering for both the intro and the timeline. "+
				"They are different paragraphs of the page and cannot be the same text node; the anchors "+
				"this condition uses have stopped identifying what they name.%s",
				entry, tag, intro, neighbourhood(page.Text, intro, 3))
		}
		t.Logf("%s: %q reaches the dump %d time(s), including the intro (index %d) and the timeline "+
			"(index %d)", entry, tag, len(at), intro, timeline)

		if entry != "index.html" {
			continue
		}
		// The shadow. On the landing page the hero states the version before
		// either of the two positions above, and that is the copy a per-string
		// dedupe keeps while dropping these.
		if first := at[0]; first >= intro || first >= timeline {
			t.Fatalf("index.html renders %q first at index %d, which is the intro's copy (%d) or the "+
				"timeline's (%d) — nothing on the page emits it earlier.\nThis condition is about a "+
				"repetition SURVIVING: the copy that arrives first is the one a per-string dedupe keeps, "+
				"and the later ones are what it deletes. With no earlier copy the two checks above pass "+
				"over an extraction that never had to keep a duplicate, so they would go on passing after "+
				"the defect came back. The hero's kicker and badge list carry that earlier copy today; if "+
				"they no longer do, this condition needs a page that still repeats the version.%s",
				tag, first, intro, timeline, neighbourhood(page.Text, first, 3))
		}
		t.Logf("index.html: the first copy of %q is at index %d, ahead of both the intro and the "+
			"timeline — the repetition this condition exists for is present", tag, at[0])
	}
}

// versionCommandCard is the CLI card whose depicted session condition 8 judges.
const versionCommandCard = "version"

// assertVersionTranscriptIsRealOutput compares the RENDERED `dossierx version`
// session against a binary linked the way a release links one.
//
// THE TWO OPERANDS COME FROM DIFFERENT PLACES, which is the whole design and the
// thing three previous versions of this check got wrong in three different ways.
// The depicted side is read out of the browser's DOM. The real side is produced by
// running a binary whose version was derived from the site's RELEASE TAG — not
// from the transcript — so a page that stopped stripping the leading `v`, or that
// started depicting a different release, makes the two disagree. A check whose two
// operands come from one substitution is not comparing anything.
//
// WHAT IT CANNOT SEE, stated rather than implied. A hand-typed literal renders
// identically to a derived one on the day it is written; this passes over it and
// so would any read of output. That invariant is about how the source produces the
// page, and it is held where such things can be held — viewer-tests/site_source_test.go's
// rule 3, which forbids a renderable version literal outside the releases array in
// BOTH spellings, the tag's and the binary's.
func assertVersionTranscriptIsRealOutput(t *testing.T, dump siteDump) {
	t.Helper()

	page := dump.page(t, "index.html")
	depicted, ok := page.transcript(versionCommandCard)
	if !ok {
		var cards []string
		for _, tr := range page.Transcripts {
			cards = append(cards, tr.Command)
		}
		t.Fatalf("the dump carries no rendered session for the %q command card. The traversal opens every `.cmd__head`, so a card whose transcript never reached the dump means the card is gone, the markup changed, or it was never opened — "+
			"and this assertion would otherwise pass over nothing. Cards whose sessions WERE read: %v", versionCommandCard, summarise(cards, 25))
	}
	if len(depicted.Lines) < 2 {
		t.Fatalf("the rendered %q session is %d line(s): %+v\nA session that abridges to nothing shows a reader a command and lets them assume what it prints", versionCommandCard, len(depicted.Lines), depicted.Lines)
	}

	// THE COMMAND, taken from the page's own prompt line. Running what the page
	// tells a reader to run is what keeps this from being a true recording of one
	// command labelled as another — a transcript true line by line and false as a
	// whole, which is the worst shape a depicted session can have.
	//
	// The prompt is identified by the line ELEMENT's class rather than by a
	// leading "$": Cli.tsx strips that character before rendering and CSS draws it
	// back with ::before, so it is in no text node and a text-based split would
	// find no prompt at all.
	if !depicted.Lines[0].Prompt {
		t.Fatalf("the rendered %q session does not open with a prompt line: %+v\nWithout one there is no command to run, and every line below would be judged against output this test chose for itself", versionCommandCard, depicted.Lines)
	}
	for _, line := range depicted.Lines[1:] {
		if line.Prompt {
			t.Fatalf("the rendered %q session depicts more than one command (%q). This assertion runs the first and compares everything beneath it; a second command means some depicted output belongs to a run this test never made, and it would be judged against the wrong process",
				versionCommandCard, line.Text)
		}
	}

	argv := strings.Fields(depicted.Lines[0].Text)
	if len(argv) < 2 || argv[0] != "dossierx" {
		t.Fatalf("the rendered %q session echoes %q, which is not a `dossierx <args>` invocation this test can run. The output beneath it would then be compared against nothing", versionCommandCard, depicted.Lines[0].Text)
	}

	// THE VERSION THE RELEASE WOULD STAMP, derived from the tag the site itself
	// calls current. `.goreleaser.yaml` stamps `-X main.version={{.Version}}`, and
	// GoReleaser's `{{.Version}}` is the tag with its leading `v` stripped —
	// `{{.Tag}}` is the spelling that keeps it. That MODEL is not held here; it is
	// held in the root module by gateRequireReleaseTransform, which parses
	// .goreleaser.yaml and fails if the template ever changes. Splitting it that
	// way is deliberate: this module cannot parse YAML (its go.mod is chromedp and
	// nothing else), and a second hand-rolled reader of that file would be a second
	// model to keep in step.
	ra := repoReleases(t)
	tag := ra.current()
	stamped := strings.TrimPrefix(tag, "v")
	if stamped == tag {
		t.Fatalf("the current release entry is %q, which carries no leading \"v\" to strip. The release build's version and the release's name would then be one string, and this comparison could no longer tell the page depicting the right one from the page depicting the wrong one",
			tag)
	}

	// The tag has to be a string the SITE renders, not only one its source
	// declares. Without this the comparison rests on a source array the page might
	// no longer be built from.
	if releases := dump.page(t, "releases.html"); !containsString(releases.Text, tag) {
		t.Fatalf("the releases page does not render %q anywhere, though content.ts declares it as the current release. The version this assertion judges the transcript against is then a fact about the source and not about the page:\n%s",
			tag, summarise(releases.Text, 30))
	}

	binary := buildReleaseLinkedDossierx(t, ra.root, stamped)
	out := strings.Split(strings.TrimRight(runTool(t, binary, argv[1:]...), "\n"), "\n")

	// Tidied the way the browser's text nodes were tidied. The binary indents its
	// commit and date lines; the DOM read collapses whitespace runs, so comparing
	// raw output against rendered text would fail on indentation and say nothing
	// about the claim.
	printed := make([]string, 0, len(out))
	for _, line := range out {
		printed = append(printed, tidySpace(line))
	}
	if len(printed) == 0 || printed[0] == "" {
		t.Fatalf("`dossierx %s` printed nothing; there is no real output for the page to be an abridgement of", strings.Join(argv[1:], " "))
	}

	// EVERY DEPICTED LINE, IN ORDER. Dropping lines is what makes the session an
	// abridgement and is allowed — the page has no truthful source for the commit
	// or the build timestamp. Inventing one, or reordering two, is not.
	remaining := printed
	for _, line := range depicted.Lines[1:] {
		at := indexOfString(remaining, line.Text)
		if at >= 0 {
			remaining = remaining[at+1:]
			continue
		}
		if indexOfString(printed, line.Text) >= 0 {
			t.Errorf("the rendered %q session depicts %q out of order: the binary prints it, but before the line the page puts above it.\nreal output:\n  %s\nrendered:\n  %s",
				versionCommandCard, line.Text, strings.Join(printed, "\n  "), renderedTranscript(depicted))
			return
		}
		t.Errorf("the rendered %q session depicts %q, which `dossierx %s` does not print.\nreal output (from a binary linked the way the release links one, `-X main.version=%s`):\n  %s\nrendered:\n  %s\n"+
			"The page may abridge that output and may not invent a line of it. If the difference is the version's leading `v`: the release build strips it, so the tag is %s and the binary prints %s.",
			versionCommandCard, line.Text, strings.Join(argv[1:], " "), stamped, strings.Join(printed, "\n  "), renderedTranscript(depicted), tag, stamped)
		return
	}

	// And it must depict the version line specifically. The walk above admits any
	// subset, including the empty one after the prompt.
	if !containsString(linesOf(depicted), printed[0]) {
		t.Errorf("the rendered %q session never shows the binary's version line. The binary prints %q; the page renders:\n  %s\n"+
			"The version is the ONE value on this page with a truthful source, and it is spelled %s — %s is the release's NAME, which is not what the command prints",
			versionCommandCard, printed[0], renderedTranscript(depicted), stamped, tag)
	}

	// The two lines it may not depict whatever the walk above allows, because it
	// has no truthful source for either. They would be admitted as a legitimate
	// abridgement only if they happened to match the values THIS test linked in,
	// which is exactly the coincidence a denylist is for.
	for _, line := range depicted.Lines[1:] {
		if strings.HasPrefix(line.Text, "commit:") {
			t.Errorf("the rendered %q session depicts a `commit:` line. The site has no truthful source for it: the release entry field that fed it was deleted for naming the wrong sha two releases running, and the binary spells it as forty characters", versionCommandCard)
		}
		if strings.HasPrefix(line.Text, "date:") {
			t.Errorf("the rendered %q session depicts a `date:` line. The binary prints an RFC 3339 timestamp from GoReleaser's `main.date` where the release entry carries a calendar day, so depicting it asserts output no build produces", versionCommandCard)
		}
	}

	t.Logf("rendered %q session matches a release-linked binary (tag %s -> stamped %s):\n  %s",
		versionCommandCard, tag, stamped, renderedTranscript(depicted))
}

// buildReleaseLinkedDossierx builds the engine from the ROOT module with the
// three ldflags a release build passes, and returns the binary.
//
// The commit and date are fixed values rather than real ones on purpose: they
// make the two lines the site may not depict impossible to match by accident, so
// a depicted `commit:` fails the comparison instead of coinciding with whatever
// this machine's git happens to say.
func buildReleaseLinkedDossierx(t *testing.T, root, version string) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "dossierx")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	ldflags := "-s -w -X main.version=" + version +
		" -X main.commit=0000000000000000000000000000000000000000" +
		" -X main.date=2000-01-01T00:00:00Z"
	build := exec.Command("go", "build", "-o", binary, "-ldflags", ldflags, "./cmd/dossierx")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build the engine with the release's ldflags: %v\n%s\nWithout a binary there is no real output for the page's transcript to be compared against, which is a check that did not run", err, out)
	}
	return binary
}

// linesOf is a rendered session's text, prompt line included.
func linesOf(tr siteTranscript) []string {
	out := make([]string, 0, len(tr.Lines))
	for _, line := range tr.Lines {
		out = append(out, line.Text)
	}
	return out
}

// renderedTranscript is a session formatted for a failure message, with the "$"
// the CSS draws put back so the reader sees what the page shows.
func renderedTranscript(tr siteTranscript) string {
	out := make([]string, 0, len(tr.Lines))
	for _, line := range tr.Lines {
		if line.Prompt {
			out = append(out, "$ "+line.Text)
			continue
		}
		out = append(out, line.Text)
	}
	return strings.Join(out, "\n  ")
}

func containsString(pool []string, want string) bool { return indexOfString(pool, want) >= 0 }

// indexOfString is the position of want in pool, or -1. The subsequence walk
// above needs the INDEX and not merely whether it is there.
func indexOfString(pool []string, want string) int {
	for i, got := range pool {
		if got == want {
			return i
		}
	}
	return -1
}

// TestSiteProseFloorCatchesADumpThatLostProse is condition 6's floor tested the
// way site_source_test.go tests its three rules: over a synthetic component
// carrying exactly the shapes that hid prose from earlier versions of this file,
// and over a pool that has lost them.
//
// It runs without a browser on purpose. The floor's whole job is to be the thing
// that goes red when the traversal quietly stops reaching something, and a check
// whose own correctness could only be demonstrated by a full build-and-drive
// would be demonstrated approximately never.
func TestSiteProseFloorCatchesADumpThatLostProse(t *testing.T) {
	const component = `import { Thing } from "./Thing";

// A line comment about the site that nobody renders at all.
export function Panel() {
  return (
    <figure aria-label="One claim, seen from both surfaces">
      <ConnectorFan label="Every tool's commit arrives at the same git gate" />
      <p className="lede">
        Hover or focus a tab to inspect the claim as trust changes.
      </p>
      <details className="resolved">
        <summary>1 resolved</summary>
        <p>Confirmed the envelope field names against the contract facet.</p>
      </details>
      <button type="button" onClick={() => setOpen(!open)}>
        show
      </button>
      <p data-label="not an attribute this collects">{interpolated} prose here in the middle</p>
    </figure>
  );
}
`
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "Panel.tsx"), []byte(component), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	// A .ts sibling, to pin that the scan reads components and not the copy file.
	if err := os.WriteFile(filepath.Join(src, "content.ts"),
		[]byte("export const lede = \"A sentence that lives in the copy file, not a component.\";\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	declared, err := scanSiteProse(src)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{} // text -> kind
	for _, p := range declared {
		got[p.Text] = p.Kind
	}

	const (
		ariaOnly    = "One claim, seen from both surfaces"
		forwarded   = "Every tool's commit arrives at the same git gate"
		visible     = "Hover or focus a tab to inspect the claim as trust changes."
		disclosed   = "Confirmed the envelope field names against the contract facet."
		interpolate = "prose here in the middle"
		copyFile    = "A sentence that lives in the copy file, not a component."
	)
	for _, want := range []struct{ text, kind string }{
		// The attribute with no text node behind it, and the one a component
		// FORWARDS as a prop — the form AgentCompat's two engine claims take,
		// and the one a scan for `aria-label=` alone would miss.
		{ariaOnly, "attribute"},
		{forwarded, "attribute"},
		// Plain visible copy, and the sentence a closed <details> keeps in the
		// DOM and out of innerText.
		{visible, "text"},
		{disclosed, "text"},
	} {
		if kind, ok := got[want.text]; !ok {
			t.Fatalf("the prose scan did not find %q, so nothing would notice the dump losing it", want.text)
		} else if kind != want.kind {
			t.Fatalf("%q was scanned as %q, want %q", want.text, kind, want.kind)
		}
	}
	for _, unwanted := range []string{interpolate, copyFile} {
		if _, ok := got[unwanted]; ok {
			t.Fatalf("the prose scan claimed %q. Interpolated JSX and the copy file are out of scope — "+
				"claiming a sentence the extraction cannot be held to is a red build with no fix",
				unwanted)
		}
	}

	// A healthy dump carries all four; the mutation each of them was written for
	// is one sentence dropping out of it, and the floor has to say which.
	full := []string{
		"noise before", ariaOnly, "noise between", forwarded, visible,
		"a wrapper around " + disclosed + " and more", "noise after",
	}
	if missing := proseMissingFrom(declared, full); len(missing) > 0 {
		t.Fatalf("a complete dump was reported as missing %d sentence(s): %s",
			len(missing), summariseProse(missing, 10))
	}
	for _, lost := range []string{ariaOnly, forwarded, visible, disclosed} {
		t.Run("a dump without "+lost[:20], func(t *testing.T) {
			var pool []string
			for _, s := range full {
				if !strings.Contains(s, lost) {
					pool = append(pool, s)
				}
			}
			missing := proseMissingFrom(declared, pool)
			if len(missing) != 1 || missing[0].Text != lost {
				t.Fatalf("dropping %q from the dump produced %d finding(s) %s, want exactly that one. "+
					"A floor that does not fire on a lost sentence is a count wearing a check's name",
					lost, len(missing), summariseProse(missing, 10))
			}
		})
	}
}
