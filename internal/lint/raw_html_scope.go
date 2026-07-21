// raw_html_scope.go implements the "raw-html-scope" lint. Claim content is
// portable markdown, not HTML — nothing in internal/render treats Body,
// Steps entries, or Row values as trusted markup (they flow through
// html/template as plain strings and get escaped on output), so raw HTML
// tags appearing in those fields are never actually "live" anywhere. The
// first half of this lint keeps that assumption honest: it flags a fixed
// denylist of structural/scripting tags (script, style, iframe, object,
// embed, form, link, meta) and inline HTML event-handler attributes
// (onclick=, onerror=, ...) wherever they appear in a claim's Body, Steps,
// or Row values — content that, if it were ever rendered unescaped by a
// future/overridden template, would be a real injection risk, so it is out
// of scope everywhere, independent of layout.
//
// The second half governs the one field that IS meant to be rendered
// unescaped: model.Claim.RawHTML, for layout: mockup claims (see
// model.LayoutMockup's doc comment and render/components' mockup
// component). Because that content is genuinely live markup, this lint
// enforces the round-3 "hardening issue 9" gate in full, in this order:
//
//  1. Layout gate — RawHTML may only be non-empty on a layout: mockup
//     claim; it must never be smuggled into a card/table/list/steps/banner
//     body (those bodies go through Body/Steps/Rows instead, and stay
//     covered by the denylist above regardless of layout).
//  2. Module allowlist — only a module present in cfg.MockupModules (a
//     checked-in field of project.config.yaml — see internal/config) may
//     author a layout: mockup claim at all. An unset/empty allowlist means
//     no module may.
//  3. Tag/attribute allowlist — RawHTML's markup may use only div, span,
//     b, and br elements (br carrying no attributes at all, same as the
//     others' "class only" ceiling), carrying at most a class attribute;
//     every other tag or attribute (including style, id, href, src,
//     data-*, any inline event handler) is a hard failure.
//  4. CSS-class allowlist — every class token used must be prefixed
//     gcp- or mockup-, matching the classes ported into the engine's
//     shared stylesheet; anything else is a hard failure.
//  5. Lock-lifecycle gate — a claim carrying RawHTML must have
//     RawHTMLReviewed == true. This is enforced here, as a lint finding,
//     rather than as a separate check inside internal/lock: internal/lock.
//     Lock already refuses to lock a claim against which the full lint
//     suite (lint.RunAll) reports any error-severity finding (see that
//     function's doc comment), so a RawHTML claim with
//     raw_html_reviewed: false simply can never pass lint, and therefore
//     can never lock, without internal/lock needing any RawHTML-specific
//     code of its own. A human flips RawHTMLReviewed to true during slice
//     review (same discipline as every other reaudit-driven field), which
//     is exactly when this finding stops firing.
//
// Every finding below defaults to SeverityError (the zero value — see
// Finding's doc comment), matching the "hard-fail" language in the round-3
// hardening spec: none of these are meant to be warn-and-proceed.
package lint

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, RawHTMLScope{})
}

// RawHTMLScope is the "raw-html-scope" lint.
type RawHTMLScope struct{}

// Name returns this lint's rule name.
func (RawHTMLScope) Name() string { return "raw-html-scope" }

// rawHTMLPattern matches an opening or closing tag for one of the denied
// structural/scripting elements, or any "on<word>=" inline event-handler
// attribute, case-insensitively. It is the denylist applied to Body, Steps,
// and Row values — every field except RawHTML, which is checked by the
// allowlist-based checkRawHTML below instead.
var rawHTMLPattern = regexp.MustCompile(`(?i)</?(script|style|iframe|object|embed|form|link|meta)\b[^>]*>|\bon[a-z]+\s*=`)

// mockupTagPattern matches one HTML start or end tag in a RawHTML blob,
// capturing whether it is a closing tag, the (lowercased-by-regexp-flag)
// tag name, and its raw attribute text (empty for closing tags).
var mockupTagPattern = regexp.MustCompile(`(?i)<(/?)([a-zA-Z][a-zA-Z0-9]*)((?:\s+[^<>]*)?)>`)

// mockupAttrPattern pulls out double-quoted name="value" attribute pairs
// from a start tag's captured attribute text. RawHTML is hand-authored
// project content (never templated from external input), so this
// intentionally does not need to handle every HTML attribute-quoting
// edge case general-purpose HTML does.
var mockupAttrPattern = regexp.MustCompile(`([a-zA-Z_:][-a-zA-Z0-9_:.]*)\s*=\s*"([^"]*)"`)

// mockupOnAttrPattern matches an inline event-handler attribute name
// (onclick, onerror, ...), case-insensitively.
var mockupOnAttrPattern = regexp.MustCompile(`(?i)^on[a-zA-Z]+$`)

// mockupClassTokenPattern is the CSS-class allowlist: a class value may
// only be made up of gcp-*/mockup-*-prefixed tokens.
var mockupClassTokenPattern = regexp.MustCompile(`^(gcp|mockup)-[a-zA-Z0-9_-]+$`)

// mockupAbsoluteURLPattern matches a URL-shaped attribute value that is
// not relative: an explicit scheme (http:, https:, javascript:, data:,
// ...) or a protocol-relative "//" prefix. A same-origin root-relative
// path ("/foo") or a plain relative path ("foo.png", "./foo") is not
// matched, and is allowed.
var mockupAbsoluteURLPattern = regexp.MustCompile(`(?i)^\s*([a-zA-Z][a-zA-Z0-9+.-]*:|//)`)

// mockupAllowedTags is the fixed tag allowlist for RawHTML content. br is
// included alongside div/span/b for the Google Cloud Console mockups'
// multi-line message cells (the source docs/ markup this content was
// migrated from uses a bare <br> as a line break inside a .gcp-msg span,
// never carrying any attribute) — it goes through the same
// attribute-allowlist enforcement below as every other allowed tag, so a
// <br class="..."> or <br style="..."> is still caught if ever authored.
//
// img is included for static diagram/flowchart claims (docs/'s own
// precedent: an <img src="../diagrams/*.svg"> with an alt-text fallback
// description — see e.g. docs/tabs/llm-internals.html's health-state-
// machine diagram). Unlike div/span/b/br, img may additionally carry src
// and alt (see checkMockupMarkup) — src still goes through
// mockupAbsoluteURLPattern so only a same-repo-relative path is legal,
// never an absolute/external URL.
var mockupAllowedTags = map[string]bool{
	"div":  true,
	"span": true,
	"b":    true,
	"br":   true,
	"img":  true,
}

// Check flags:
//   - any claim whose Body, Steps entries, or Row string values contain
//     out-of-scope raw HTML per rawHTMLPattern (independent of layout);
//   - any claim whose RawHTML content, module, or layout violates the
//     five-part mockup gate described in this file's package-level doc
//     comment.
func (RawHTMLScope) Check(claims []model.Claim, cfg *config.Config) []Finding {
	var findings []Finding
	for _, c := range claims {
		if rawHTMLPattern.MatchString(c.Body) {
			findings = append(findings, Finding{
				LintName: "raw-html-scope",
				ClaimID:  c.ID,
				Message:  "body contains out-of-scope raw HTML",
			})
		}
		for i, step := range c.Steps {
			if rawHTMLPattern.MatchString(step) {
				findings = append(findings, Finding{
					LintName: "raw-html-scope",
					ClaimID:  c.ID,
					Message:  fmt.Sprintf("steps[%d] contains out-of-scope raw HTML", i),
				})
			}
		}
		for i, row := range c.Rows {
			for k, v := range row {
				s, ok := v.(string)
				if !ok {
					continue
				}
				if rawHTMLPattern.MatchString(s) {
					findings = append(findings, Finding{
						LintName: "raw-html-scope",
						ClaimID:  c.ID,
						Message:  fmt.Sprintf("rows[%d].%s contains out-of-scope raw HTML", i, k),
					})
				}
			}
		}

		findings = append(findings, checkMockupGate(c, cfg)...)
	}
	return findings
}

// checkMockupGate applies the five-part round-3 "hardening issue 9" gate
// to one claim's RawHTML field. A claim with an empty RawHTML has nothing
// to check here and is skipped entirely (mockup claims with genuinely
// empty content are layout-shape-mismatch's concern, not this lint's).
func checkMockupGate(c model.Claim, cfg *config.Config) []Finding {
	if c.RawHTML == "" {
		return nil
	}

	var findings []Finding
	mockupFinding := func(msg string) {
		findings = append(findings, Finding{
			LintName: "raw-html-scope",
			ClaimID:  c.ID,
			Message:  msg,
		})
	}

	// 1. Layout gate.
	if c.Layout != model.LayoutMockup {
		mockupFinding(fmt.Sprintf("raw_html is only legal on layout: mockup, got layout: %q", c.Layout))
	}

	// 2. Module allowlist.
	allowlisted := cfg != nil && contains(cfg.MockupModules, c.Module)
	if !allowlisted {
		mockupFinding(fmt.Sprintf("module %q is not in the project's mockup_modules allowlist and may not author layout: mockup / raw_html claims", c.Module))
	}

	// 3 & 4. Tag/attribute allowlist and CSS-class allowlist.
	findings = append(findings, checkMockupMarkup(c.ID, c.RawHTML)...)

	// 5. Lock-lifecycle gate. Enforced here, not in internal/lock: Lock
	// refuses to lock against any error-severity lint finding, so this
	// finding alone is sufficient to block locking (see this file's
	// package doc comment, point 5).
	if !c.RawHTMLReviewed {
		mockupFinding("raw_html is set but raw_html_reviewed is not true; a human must review this markup and set raw_html_reviewed: true before this claim can lock")
	}

	return findings
}

// checkMockupMarkup scans raw for tags outside mockupAllowedTags, attributes
// other than a single, allowlisted-class-valued "class", inline event
// handlers, style attributes, and non-relative URLs — the tag/attribute and
// CSS-class allowlist legs of the mockup gate.
func checkMockupMarkup(claimID, raw string) []Finding {
	var findings []Finding
	add := func(msg string) {
		findings = append(findings, Finding{
			LintName: "raw-html-scope",
			ClaimID:  claimID,
			Message:  msg,
		})
	}

	for _, tag := range mockupTagPattern.FindAllStringSubmatch(raw, -1) {
		closing := tag[1] == "/"
		name := strings.ToLower(tag[2])
		attrText := tag[3]

		if !mockupAllowedTags[name] {
			add(fmt.Sprintf("raw_html contains disallowed tag <%s%s> (only div, span, b, br, img are allowed)", tag[1], name))
			continue
		}
		if closing {
			continue
		}

		for _, attr := range mockupAttrPattern.FindAllStringSubmatch(attrText, -1) {
			attrName := strings.ToLower(attr[1])
			attrValue := attr[2]

			switch {
			case mockupOnAttrPattern.MatchString(attrName):
				add(fmt.Sprintf("raw_html <%s> has disallowed inline event-handler attribute %q", name, attr[1]))
			case attrName == "style":
				add(fmt.Sprintf("raw_html <%s> has a disallowed style attribute (RawHTML markup must be style-free)", name))
			case attrName == "class":
				for _, token := range strings.Fields(attrValue) {
					if !mockupClassTokenPattern.MatchString(token) {
						add(fmt.Sprintf("raw_html <%s> class %q is not in the .gcp-*/.mockup-* CSS-class allowlist", name, token))
					}
				}
			case name == "img" && attrName == "alt":
				// Free-text alt description; no allowlist beyond the
				// shared event-handler/style checks above.
			case attrName == "href" || attrName == "src":
				if name != "img" || attrName != "src" {
					add(fmt.Sprintf("raw_html <%s> has disallowed attribute %q (only class is permitted on div/span/b/br)", name, attr[1]))
					continue
				}
				if mockupAbsoluteURLPattern.MatchString(attrValue) {
					add(fmt.Sprintf("raw_html <%s> %s=%q is a non-relative URL, which is disallowed", name, attrName, attrValue))
				}
			default:
				add(fmt.Sprintf("raw_html <%s> has disallowed attribute %q (only class is permitted on div/span/b/br)", name, attr[1]))
			}
		}
	}

	return findings
}
