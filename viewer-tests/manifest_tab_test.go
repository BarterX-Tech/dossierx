package viewertests

// THE MODULE MANIFEST TAB (NIT-19), AS A READER SEES IT.
//
// The Manifest peer tab renders claims_dir/<module>/manifest.yaml read-only:
//
//   - healthy: the summary as plain escaped text, every provides id linked to
//     its Contract claim, every depends_on id linked to its claim and to its
//     provider module, and a toggle that reveals the file's raw YAML;
//   - refused (missing, oversize, malformed, invalid deps): the module-manifest
//     finding(s) `dossierx check` reports for that module, word for word, and
//     the copyable `dossierx manifest show <module> --isolation` command — and
//     nothing of the broken file.
//
// WHAT THIS COVERS, stated because a rendered-state claim is a coverage claim:
// a two-module project rendered by the binary under test, in the static
// file:// viewer (check) and the served viewer (serve), in the default light
// scheme at the harness's default viewport. The healthy case asserts the two
// viewers render the identical .manifest-view. The refused case is exercised
// through serve only, because a failing check writes no static viewer at all;
// it covers a malformed file and a missing file. The oversize and invalid-deps
// shapes render through the same code path and are covered by the Go unit
// tests in internal/manifest and internal/render, not here.

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/chromedp/chromedp"
)

// widgetManifestYAML carries the characters an HTML or markdown renderer
// would consume; the tab must show them literally.
const widgetManifestYAML = "summary: Widget keeps <b>one</b> boundary & *never* leaks.\n" +
	"provides:\n" +
	"  - widget.contract.overview\n" +
	"depends_on:\n" +
	"  - gadget.contract.overview\n"

const gadgetManifestYAML = "summary: Gadget store.\n" +
	"provides:\n" +
	"  - gadget.contract.overview\n" +
	"depends_on: []\n"

func manifestTabProject(t *testing.T) *project {
	t.Helper()
	p := newProjectRaw(t, twoModuleConfig)
	p.writeClaim("widget.yaml", twoModuleClaim("widget.contract.overview", "widget"))
	p.writeClaim("gadget.yaml", twoModuleClaim("gadget.contract.overview", "gadget"))
	p.writeManifest("widget", widgetManifestYAML)
	p.writeManifest("gadget", gadgetManifestYAML)
	return p
}

func (p *project) writeManifest(module, body string) {
	p.t.Helper()
	if err := os.WriteFile(filepath.Join(p.claimsDir, module, "manifest.yaml"), []byte(body), 0o644); err != nil {
		p.t.Fatalf("write %s manifest: %v", module, err)
	}
}

// openManifestTab loads url, checks the module still opens on Contract with
// exactly Manifest | Contract | Internals, then clicks its Manifest tab.
func openManifestTab(t *testing.T, ctx context.Context, url, module string) {
	t.Helper()
	runCDP(t, ctx, chromedp.Navigate(url+"#"+module))
	// A served page mounts its live controls asynchronously; clicking before
	// they settle races the runtime's own view restore.
	pollTrue(t, ctx, `document.readyState === 'complete' && (location.protocol === 'file:' || (document.body.classList.contains('comments-live') && document.body.classList.contains('comments-sse-open')))`)
	pollTrue(t, ctx, `(function(){var s=document.getElementById('`+module+`');return !!s && !s.hidden && !!s.querySelector('.subtab.on');})()`)
	requireAll(t, ctx, "the "+module+" module's peer tabs", `var sec = document.getElementById('`+module+`');
		var tabs = Array.prototype.map.call(sec.querySelectorAll(':scope > .sub-nav .subtab .sec-tab__label'), function (e) { return e.textContent.trim(); });
		var open = sec.querySelector(':scope > .claim-group:not([hidden])');`,
		[][2]string{
			{"tabs are exactly Manifest|Contract|Internals", `tabs.join('|') === 'Manifest|Contract|Internals'`},
			{"the module opens on Contract", `open && open.id === '` + module + `-contract'`},
			{"the manifest is a peer tab, not a banner above the tabs", `!sec.querySelector(':scope > .manifest-view') && sec.querySelectorAll('.manifest-view').length === 1 && sec.querySelector('.manifest-view').closest('.claim-group').id === '` + module + `-manifest'`},
		})
	runCDP(t, ctx, chromedp.Click(`#`+module+` .subtab[data-target="#`+module+`-manifest"]`, chromedp.ByQuery))
	pollTrue(t, ctx, `(function(){var g=document.getElementById('`+module+`-manifest');return !!g && !g.hidden && !!g.querySelector('.manifest-view');})()`)
}

func TestManifestTabRendersTheHealthyManifest(t *testing.T) {
	p := manifestTabProject(t)
	staticURL := p.renderStatic()
	base, _ := p.serve()

	var viewHTML [2]string
	for i, url := range []string{staticURL, base + "/"} {
		ctx := browserContext(t)
		openManifestTab(t, ctx, url, "widget")

		requireAll(t, ctx, "the healthy widget manifest ("+url+")", `var v = document.querySelector('#widget-manifest .manifest-view');
			var summary = v.querySelector('.manifest-summary');
			var provides = v.querySelectorAll('.manifest-provides .manifest-id');
			var deps = v.querySelectorAll('.manifest-depends-on .manifest-id');
			var raw = v.querySelector('details.manifest-raw');`,
			[][2]string{
				{"state is ok", `v.getAttribute('data-manifest-state') === 'ok'`},
				{"no refusal chrome", `!v.querySelector('.manifest-findings') && !v.querySelector('.manifest-copy')`},
				{"summary is the literal, escaped text", `summary.textContent === 'Widget keeps <b>one</b> boundary & *never* leaks.' && summary.children.length === 0`},
				{"one provides row linked to its Contract claim", `provides.length === 1 && provides[0].querySelector('a.claim-ref').getAttribute('href') === '#widget.contract.overview'`},
				{"one depends_on row linked to its claim", `deps.length === 1 && deps[0].querySelector('a.claim-ref').getAttribute('href') === '#gadget.contract.overview'`},
				{"the depends_on row links its provider module", `deps[0].querySelector('a.manifest-provider-link').getAttribute('href') === '#gadget' && deps[0].querySelector('a.manifest-provider-link').textContent === 'Gadget'`},
				{"raw YAML starts collapsed", `raw && !raw.open && !raw.querySelector('pre').checkVisibility()`},
				{"nothing is editable", `!v.querySelector('input, textarea, select, [contenteditable]')`},
			})

		// The raw toggle reveals the file byte for byte.
		runCDP(t, ctx, chromedp.Click(`#widget-manifest details.manifest-raw > summary`, chromedp.ByQuery))
		pollTrue(t, ctx, `document.querySelector('#widget-manifest details.manifest-raw').open`)
		var raw string
		runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('#widget-manifest .manifest-raw-text').textContent`, &raw))
		if raw != widgetManifestYAML {
			t.Fatalf("%s: raw YAML toggle shows %q, want the file %q", url, raw, widgetManifestYAML)
		}
		if !evalBool(t, ctx, `document.querySelector('#widget-manifest .manifest-raw-text').checkVisibility() && document.querySelector('#widget-manifest .manifest-raw-text').getBoundingClientRect().height > 0`) {
			t.Fatalf("%s: raw YAML is open but not painted", url)
		}
		runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('#widget-manifest .manifest-view').outerHTML`, &viewHTML[i]))

		// provides -> the claim on this module's Contract tab.
		runCDP(t, ctx, chromedp.Click(`#widget-manifest .manifest-provides a.claim-ref`, chromedp.ByQuery))
		pollTrue(t, ctx, `!document.getElementById('widget').hidden && !document.getElementById('widget-contract').hidden && !!document.getElementById('widget.contract.overview')`)

		// depends_on claim -> the provider's Contract claim.
		openManifestTab(t, ctx, url, "widget")
		runCDP(t, ctx, chromedp.Click(`#widget-manifest .manifest-depends-on a.claim-ref`, chromedp.ByQuery))
		pollTrue(t, ctx, `!document.getElementById('gadget').hidden && document.getElementById('widget').hidden && !document.getElementById('gadget-contract').hidden`)

		// depends_on provider -> the provider module, opened on Contract.
		openManifestTab(t, ctx, url, "widget")
		runCDP(t, ctx, chromedp.Click(`#widget-manifest a.manifest-provider-link`, chromedp.ByQuery))
		pollTrue(t, ctx, `!document.getElementById('gadget').hidden && document.getElementById('widget').hidden && !document.getElementById('gadget-contract').hidden`)
	}
	if viewHTML[0] == "" || viewHTML[0] != viewHTML[1] {
		t.Fatalf("static and served viewers render different Manifest tabs:\n static: %s\n served: %s", viewHTML[0], viewHTML[1])
	}
}

// checkManifestFindings runs `dossierx check --format json` and returns the
// module-manifest messages it reports for module: the reference text.
func checkManifestFindings(t *testing.T, p *project, module string) []string {
	t.Helper()
	out, err := exec.Command(p.bin, "--config", p.config, "--format", "json", "check").Output()
	if err == nil {
		t.Fatalf("check passed on a broken manifest:\n%s", out)
	}
	var env struct {
		Data struct {
			Findings []struct {
				Lint    string `json:"lint"`
				ClaimID string `json:"claim_id"`
				Message string `json:"message"`
			} `json:"lint_findings"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &env); err != nil {
		t.Fatalf("check --format json: %v\n%s", err, out)
	}
	var msgs []string
	for _, f := range env.Data.Findings {
		if f.Lint == "module-manifest" && f.ClaimID == module {
			msgs = append(msgs, f.Message)
		}
	}
	if len(msgs) == 0 {
		t.Fatalf("check reported no module-manifest finding for %s:\n%s", module, out)
	}
	return msgs
}

func TestManifestTabRefusesABrokenManifest(t *testing.T) {
	cases := []struct {
		name    string
		breakIt func(p *project)
	}{
		{"malformed", func(p *project) { p.writeManifest("widget", "summary: [<b>half</b>\n") }},
		{"missing", func(p *project) {
			if err := os.Remove(filepath.Join(p.claimsDir, "widget", "manifest.yaml")); err != nil {
				p.t.Fatalf("remove widget manifest: %v", err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := manifestTabProject(t)
			tc.breakIt(p)
			want := checkManifestFindings(t, p, "widget")
			wantJSON, err := json.Marshal(want)
			if err != nil {
				t.Fatal(err)
			}

			base, _ := p.serve()
			ctx := browserContext(t)
			openManifestTab(t, ctx, base+"/", "widget")

			const cmd = "dossierx manifest show widget --isolation"
			requireAll(t, ctx, "the refused widget manifest", `var v = document.querySelector('#widget-manifest .manifest-view');
				var msgs = Array.prototype.map.call(v.querySelectorAll('.manifest-finding'), function (li) {
					return li.querySelector('.manifest-finding-message').textContent;
				});
				var lints = Array.prototype.map.call(v.querySelectorAll('.manifest-finding-lint'), function (e) { return e.textContent; });`,
				[][2]string{
					{"state is refused", `v.getAttribute('data-manifest-state') === 'refused'`},
					{"messages are check's, word for word", `JSON.stringify(msgs) === ` + strconv.Quote(string(wantJSON))},
					{"each message names the lint", `lints.length === msgs.length && lints.every(function (l) { return l === 'module-manifest:'; })`},
					{"the command is shown", `v.querySelector('.manifest-command-text').textContent === ` + strconv.Quote(cmd)},
					{"the copy control carries the command", `v.querySelector('button.manifest-copy').getAttribute('data-copy-text') === ` + strconv.Quote(cmd)},
					{"nothing of the broken file is soft-rendered", `!v.querySelector('.manifest-summary, .manifest-raw, .manifest-provides, .manifest-depends-on') && v.textContent.indexOf('half') < 0`},
					{"the gadget manifest is still healthy", `document.querySelector('#gadget-manifest .manifest-view').getAttribute('data-manifest-state') === 'ok'`},
				})

			// Copy selects the command, so it is one keystroke from copied
			// even where the clipboard API is unavailable.
			runCDP(t, ctx, chromedp.Click(`#widget-manifest button.manifest-copy`, chromedp.ByQuery))
			pollTrue(t, ctx, `window.getSelection().toString() === `+strconv.Quote(cmd))
		})
	}
}
