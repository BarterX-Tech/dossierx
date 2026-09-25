package viewertests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
)

// The roof, as rendered (NIT-27). Every other file in this suite names
// .constitution-section only to EXCLUDE it from a module count; this one
// asserts the surface itself, in a real browser, on a project with a locked
// constitution and one project claim:
//
//   - the Constitution pin sits above the Modules group in the sidebar, and
//     its kicker "PROJECT — NOT A MODULE" is painted;
//   - the section carries the same kicker, two tabs "The file" and "Project
//     claims", with "The file" open by default;
//   - the word meter reads "N of 800 words" with N the engine's own count;
//   - "Project claims" lists the fixture's project claim by id;
//   - the constitution's body is plain escaped text: a `*` and a `<b>` in the
//     file appear literally, never as markup.

// roofWithMarkupYAML is the fixture roof: 17 words by the engine's meter
// (unicode letter/number runs over every entry's title and body; slugs are
// not counted), carrying the two characters a markdown or HTML renderer
// would consume. "<b>one</b>" is three runs: b, one, b.
const roofWithMarkupYAML = "status: draft\n" +
	"invariants:\n" +
	"  - slug: one-roof\n" +
	"    title: One roof\n" +
	"    body: This fixture keeps *every* module under <b>one</b> roof, and no module redefines it.\n"

const roofWithMarkupWords = 17

const projectClaimScopeYAML = `id: project.scope
scope: project
status: draft
layout: card
body: |
  Every widget is kept under <b>one</b> roof.
rests_on:
  none: true
  reason: the roof above it is the constitution, which is not a claim
`

const overviewRestingOnScopeYAML = `id: widget.contract.overview
facet: contract
module: widget
status: draft
body: |
  a claim under review.
rests_on:
  - project.scope
`

// roofProject is the one-module project with a locked constitution that
// carries markup characters, and one project claim the module claim rests
// on. The roof is re-locked through the real command after the fixture's
// default roof is replaced, so the lock store's record matches the file the
// viewer renders.
func roofProject(t *testing.T) *project {
	t.Helper()
	p := newProjectRaw(t, defaultConfigYAML)
	if err := os.WriteFile(filepath.Join(p.dir, "constitution.yaml"), []byte(roofWithMarkupYAML), 0o644); err != nil {
		t.Fatalf("write the roof: %v", err)
	}
	p.run("constitution", "lock", "--reason", "roof with markup characters")
	p.writeClaim("overview.yaml", overviewRestingOnScopeYAML)
	store := filepath.Join(p.dir, "project-claims")
	if err := os.MkdirAll(store, 0o755); err != nil {
		t.Fatalf("mkdir project-claims: %v", err)
	}
	if err := os.WriteFile(filepath.Join(store, "scope.yaml"), []byte(projectClaimScopeYAML), 0o644); err != nil {
		t.Fatalf("write the project claim: %v", err)
	}
	return p
}

// engineWordCount asks the binary for the roof's word count, so the meter is
// compared against the real meter and not only against this file's arithmetic.
func engineWordCount(t *testing.T, p *project) int {
	t.Helper()
	out := p.run("constitution", "show", "--format", "json")
	var env struct {
		Data struct {
			Digest struct {
				Words   int    `json:"words"`
				WordCap int    `json:"word_cap"`
				Status  string `json:"status"`
			} `json:"digest"`
			Lock struct {
				State string `json:"state"`
			} `json:"lock"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("constitution show --format json: %v\n%s", err, out)
	}
	if env.Data.Lock.State != "locked" || env.Data.Digest.Status != "locked" {
		t.Fatalf("the fixture roof is not locked: %s", out)
	}
	if env.Data.Digest.WordCap != 800 {
		t.Fatalf("word cap = %d, want 800", env.Data.Digest.WordCap)
	}
	return env.Data.Digest.Words
}

func TestConstitutionSectionRendersTheRoof(t *testing.T) {
	p := roofProject(t)
	words := engineWordCount(t, p)
	if words != roofWithMarkupWords {
		t.Fatalf("the engine counts %d words in the fixture roof; this file expects %d — one of them is wrong about the meter", words, roofWithMarkupWords)
	}
	url := p.renderStatic()

	ctx := browserContext(t)
	runCDP(t, ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible(".constitution-tab", chromedp.ByQuery),
	)
	// The runtime resolves the initial hash asynchronously; wait for the
	// default reading view (the first module) before reading the sidebar.
	pollTrue(t, ctx, `document.querySelectorAll('.module-section:not(.constitution-section):not(.track-section)').length === 1 && !document.querySelectorAll('.module-section:not(.constitution-section):not(.track-section)')[0].hidden`)

	// The pin, before anything is clicked: above the Modules group, first
	// among the sidebar tabs, its kicker painted.
	requireAll(t, ctx, "the Constitution pin in the sidebar",
		`var pin = document.querySelector('#nav .constitution-pin');
		 var groups = document.querySelector('#nav .system-nav-groups');
		 var tabs = document.querySelectorAll('#nav .sec-tab');
		 var meta = document.querySelector('#nav .constitution-tab .sec-tab__meta');
		 var modules = document.querySelectorAll('.module-section:not(.constitution-section):not(.track-section)');`,
		[][2]string{
			{"pin exists", `!!pin`},
			{"Modules group exists", `!!groups`},
			{"pin precedes the Modules group", `!!(pin.compareDocumentPosition(groups) & Node.DOCUMENT_POSITION_FOLLOWING)`},
			{"pin is the first sidebar tab", `tabs.length >= 2 && tabs[0].classList.contains('constitution-tab')`},
			{"kicker text", `meta && meta.textContent.trim() === 'PROJECT — NOT A MODULE'`},
			{"kicker painted", `meta && meta.offsetParent !== null && getComputedStyle(meta).visibility !== 'hidden'`},
			{"one module section beside it", `modules.length === 1 && !modules[0].hidden`},
			{"section starts hidden", `document.getElementById('constitution').hidden === true`},
		})

	// Open it the way a reader does, through the delegated sidebar handler.
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('#nav .constitution-tab').click();`, nil))
	pollTrue(t, ctx, `!document.getElementById('constitution').hidden`)

	requireAll(t, ctx, "the Constitution section with 'The file' open by default",
		`var sec = document.getElementById('constitution');
		 var kicker = sec.querySelector('.constitution-kicker');
		 var subtabs = sec.querySelectorAll('.sub-nav .subtab');
		 var meter = sec.querySelector('.constitution-meter');
		 var file = document.getElementById('constitution-file');
		 var claims = document.getElementById('constitution-project-claims');
		 var modules = document.querySelectorAll('.module-section:not(.constitution-section):not(.track-section)');`,
		[][2]string{
			{"section kicker text", `kicker && kicker.textContent.trim() === 'PROJECT — NOT A MODULE'`},
			{"section kicker painted", `kicker && kicker.offsetParent !== null`},
			{"two tabs", `subtabs.length === 2`},
			{"first tab is The file", `subtabs[0].textContent.trim() === 'The file' && subtabs[0].dataset.target === '#constitution-file'`},
			{"second tab is Project claims", `subtabs[1].textContent.trim() === 'Project claims' && subtabs[1].dataset.target === '#constitution-project-claims'`},
			{"The file is on", `subtabs[0].classList.contains('on') && !subtabs[1].classList.contains('on')`},
			{"The file is shown", `file && !file.hidden && file.offsetParent !== null`},
			{"Project claims is hidden", `claims && claims.hidden === true`},
			{"the module section gave way", `modules.length === 1 && modules[0].hidden === true`},
			{"word meter", `meter && meter.textContent.trim().indexOf('` + strconv.Itoa(words) + ` of 800 words') === 0`},
			{"meter names the lock state", `meter && meter.textContent.indexOf('locked') >= 0`},
			{"the invariant is rendered", `!!file.querySelector('#constitution-invariants-one-roof')`},
			{"no <b> element was created from the body", `file.querySelector('b') === null`},
			{"the literal <b> is visible text", `file.textContent.indexOf('<b>one</b>') >= 0`},
			{"the literal asterisks are visible text", `file.textContent.indexOf('*every*') >= 0`},
			{"no <em> element was created from the asterisks", `file.querySelector('em') === null`},
		})

	// The second tab, through the same delegated handler.
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelectorAll('#constitution .sub-nav .subtab')[1].click();`, nil))
	pollTrue(t, ctx, `!document.getElementById('constitution-project-claims').hidden`)

	requireAll(t, ctx, "the Project claims tab",
		`var sec = document.getElementById('constitution');
		 var subtabs = sec.querySelectorAll('.sub-nav .subtab');
		 var file = document.getElementById('constitution-file');
		 var claims = document.getElementById('constitution-project-claims');
		 var cards = claims.querySelectorAll('article.project-claim');`,
		[][2]string{
			{"Project claims is on", `subtabs[1].classList.contains('on') && !subtabs[0].classList.contains('on')`},
			{"The file is hidden", `file.hidden === true`},
			{"section still shown", `!sec.hidden`},
			{"exactly the fixture's project claim", `cards.length === 1`},
			{"listed by id", `cards[0].querySelector('h4').textContent.trim() === 'project.scope'`},
			{"its body is escaped text too", `cards[0].querySelector('b') === null && cards[0].textContent.indexOf('<b>one</b>') >= 0`},
			{"no placeholder", `claims.textContent.indexOf('No project claims') < 0`},
		})

	// The rendered file carries the escaped bytes, so a reader of the static
	// HTML sees the same thing the browser painted.
	raw, err := os.ReadFile(strings.TrimPrefix(url, "file://"))
	if err != nil {
		t.Fatalf("read the rendered viewer: %v", err)
	}
	for _, want := range []string{
		"&lt;b&gt;one&lt;/b&gt; roof, and no module redefines it.",
		`<article class="project-claim"><h4>project.scope</h4>`,
	} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("rendered viewer lacks %q", want)
		}
	}
}
