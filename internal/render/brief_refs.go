package render

import (
	"html"
	"path"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/render/components"
)

// linkCodeRefs turns an inline code span that names another brief or a claim
// into a link to it (NIT-248, Paper boards H1 and H1b of Release 0.7.23).
// Briefs cite each other and claims as code: `features/onboarding`,
// `voice/talking-about-money`,
// `app-shell.contract.removing-or-hiding-the-icon-quits-nothing`. A span
// resolves when its text, trimmed, is
//
//   - a claim id the catalog holds (statuses), linking to "#<id>" the way a
//     relationship row's claim-ref does;
//   - a brief's <folder>/<slug>, with or without ".md", or any path
//     briefLinkAnchor accepts for a markdown link, linking to the page.
//
// Anything else (an API name, a host, a folder that is not a brief) stays a
// plain code span, so nothing reads as a link that goes nowhere.
//
// The span keeps its <code> element and its text; only an <a> is wrapped
// around it. Word counts are unaffected: briefs count
// markdown.DocumentText(body), which never sees this HTML.
//
// The markdown renderer writes an inline span as a bare "<code>" with its text
// HTML-escaped, and a block as "<pre><code>", so a "<code>" not preceded by
// "<pre>" is always an inline span it wrote. A span already inside a link
// ("[`x`](url)") is left alone. O(body bytes).
func linkCodeRefs(body, briefPath string, targets map[string]string, statuses map[string]components.TargetStatus) string {
	const open, closeTag = "<code>", "</code>"
	if !strings.Contains(body, open) {
		return body
	}
	dir := path.Dir(briefPath)
	root := path.Dir(dir)
	var b strings.Builder
	b.Grow(len(body) + 64)
	rest := body
	inLink := false
	for {
		i := strings.Index(rest, open)
		if i < 0 {
			b.WriteString(rest)
			return b.String()
		}
		inLink = linkStateAfter(rest[:i], inLink)
		end := strings.Index(rest[i+len(open):], closeTag)
		if end < 0 {
			b.WriteString(rest)
			return b.String()
		}
		spanEnd := i + len(open) + end + len(closeTag)
		span := rest[i:spanEnd]
		text := strings.TrimSpace(html.UnescapeString(rest[i+len(open) : i+len(open)+end]))
		b.WriteString(rest[:i])
		switch {
		case inLink || strings.HasSuffix(rest[:i], "<pre>"):
			b.WriteString(span)
		case statuses != nil && isClaimRef(text, statuses):
			esc := html.EscapeString(text)
			b.WriteString(`<a class="code-ref code-ref--claim" href="#` + esc + `" data-claim-id="` + esc + `">` + span + `</a>`)
		default:
			if anchor, ok := briefCodeAnchor(text, dir, root, targets); ok {
				b.WriteString(`<a class="code-ref code-ref--brief" href="#` + anchor + `">` + span + `</a>`)
			} else {
				b.WriteString(span)
			}
		}
		rest = rest[spanEnd:]
	}
}

// linkStateAfter reports whether an <a> element is still open at the end of
// s, given whether one was open at its start.
func linkStateAfter(s string, open bool) bool {
	o := strings.LastIndex(s, "<a ")
	c := strings.LastIndex(s, "</a>")
	switch {
	case o < 0 && c < 0:
		return open
	default:
		return o > c
	}
}

func isClaimRef(text string, statuses map[string]components.TargetStatus) bool {
	if text == "" || strings.ContainsAny(text, " /") {
		return false
	}
	_, ok := statuses[text]
	return ok
}

// briefCodeAnchor resolves a code span's text to a brief page: a markdown
// link path (briefLinkAnchor), or <folder>/<slug> under the briefs root.
func briefCodeAnchor(text, dir, root string, targets map[string]string) (string, bool) {
	if text == "" || strings.ContainsAny(text, " :#?") {
		return "", false
	}
	if strings.HasSuffix(text, ".md") {
		if anchor, ok := briefLinkAnchor(text, dir, targets); ok {
			return anchor, true
		}
		text = strings.TrimSuffix(text, ".md")
	}
	if strings.Count(text, "/") != 1 {
		return "", false
	}
	anchor, ok := targets[path.Join(root, text+".md")]
	return anchor, ok
}
