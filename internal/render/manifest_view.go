// manifest_view.go renders a module's Manifest peer tab (NIT-19): the
// module's claims_dir/<module>/manifest.yaml, read-only.
//
// ONE SOURCE OF TRUTH. The data is manifest.Viewer, which runs the same
// rules the module-manifest lint runs, and the static viewer ("check") and
// the served viewer ("serve") both reach this file through Render, so the
// two cannot disagree about a module's manifest.
//
// A BROKEN FILE IS NEVER SOFT-RENDERED. A missing, oversize, malformed or
// invalid manifest shows check's own module-manifest message(s) verbatim and
// the command that drafts it, and nothing of the file's content: no summary,
// no id lists, no raw text. A healthy one shows the summary, provides (each
// linked to its Contract claim), depends_on (each linked to its claim and
// its provider module) and a disclosure holding the raw YAML.
//
// Every value is author input and is escaped here: this returns
// template.HTML, which html/template does not escape again.
package render

import (
	"html"
	"html/template"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/lint"
	"github.com/BarterX-Tech/dossierx/internal/manifest"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/render/components"
)

// manifestTabHTML renders one module's Manifest tab body. byID resolves a
// depends_on id to its provider module.
func manifestTabHTML(v manifest.ViewerModule, byID map[string]model.Claim) template.HTML {
	var b strings.Builder
	if !v.Healthy() {
		writeManifestRefusal(&b, v)
		return template.HTML(b.String())
	}

	b.WriteString(`<div class="manifest-view" data-manifest-state="ok" data-module="`)
	b.WriteString(html.EscapeString(v.Module))
	b.WriteString(`">`)
	writeManifestPath(&b, v.Path)

	b.WriteString(`<h3 class="manifest-heading">Summary</h3><p class="manifest-summary">`)
	b.WriteString(html.EscapeString(strings.TrimSpace(v.Manifest.Summary)))
	b.WriteString(`</p>`)

	b.WriteString(`<h3 class="manifest-heading">Provides</h3>`)
	if len(v.Manifest.Provides) == 0 {
		b.WriteString(`<p class="manifest-none">provides: []</p>`)
	} else {
		b.WriteString(`<ul class="manifest-ids manifest-provides">`)
		for _, id := range v.Manifest.Provides {
			b.WriteString(`<li class="manifest-id">`)
			b.WriteString(string(components.ClaimRefHTML(id, v.Module, config.FacetContract)))
			b.WriteString(`</li>`)
		}
		b.WriteString(`</ul>`)
	}

	b.WriteString(`<h3 class="manifest-heading">Depends on</h3>`)
	if len(v.Manifest.DependsOn) == 0 {
		b.WriteString(`<p class="manifest-none">depends_on: []</p>`)
	} else {
		b.WriteString(`<ul class="manifest-ids manifest-depends-on">`)
		for _, id := range v.Manifest.DependsOn {
			b.WriteString(`<li class="manifest-id">`)
			b.WriteString(string(components.ClaimRefHTML(id, v.Module, config.FacetContract)))
			// A healthy manifest's depends_on id is always another module's
			// contract claim (module-manifest refuses anything else), so the
			// provider is known; the guard only keeps an impossible miss from
			// printing an empty link.
			if c, ok := byID[id]; ok && c.Module != "" {
				b.WriteString(` <span class="manifest-provider">provided by <a class="manifest-provider-link" href="#`)
				b.WriteString(html.EscapeString(slugify(c.Module)))
				b.WriteString(`" data-module="`)
				b.WriteString(html.EscapeString(c.Module))
				b.WriteString(`">`)
				b.WriteString(html.EscapeString(displayCase(c.Module)))
				b.WriteString(`</a></span>`)
			}
			b.WriteString(`</li>`)
		}
		b.WriteString(`</ul>`)
	}

	b.WriteString(`<details class="manifest-raw"><summary>Raw YAML</summary><pre class="manifest-raw-text"><code>`)
	b.WriteString(html.EscapeString(v.Raw))
	b.WriteString(`</code></pre></details></div>`)
	return template.HTML(b.String())
}

// writeManifestRefusal is the refused state: check's findings verbatim, under
// the lint's own name, and the copyable command that drafts the file.
func writeManifestRefusal(b *strings.Builder, v manifest.ViewerModule) {
	b.WriteString(`<div class="manifest-view manifest-view--refused" data-manifest-state="refused" data-module="`)
	b.WriteString(html.EscapeString(v.Module))
	b.WriteString(`">`)
	writeManifestPath(b, v.Path)
	b.WriteString(`<ul class="manifest-findings">`)
	for _, f := range v.Findings {
		b.WriteString(`<li class="manifest-finding"><span class="manifest-finding-lint">`)
		b.WriteString(lint.ModuleManifestLintName)
		b.WriteString(`:</span> <span class="manifest-finding-message">`)
		b.WriteString(html.EscapeString(f.Message))
		b.WriteString(`</span></li>`)
	}
	b.WriteString(`</ul><div class="manifest-command"><code class="manifest-command-text">`)
	b.WriteString(html.EscapeString(v.Command))
	b.WriteString(`</code><button type="button" class="manifest-copy" data-copy-text="`)
	b.WriteString(html.EscapeString(v.Command))
	b.WriteString(`">Copy</button></div></div>`)
}

func writeManifestPath(b *strings.Builder, rel string) {
	b.WriteString(`<p class="manifest-path"><code>`)
	b.WriteString(html.EscapeString(rel))
	b.WriteString(`</code></p>`)
}
