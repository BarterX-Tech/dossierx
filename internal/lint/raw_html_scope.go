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
	"html"
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

// MockupGateFindings runs ONLY the RawHTML mockup gate (checkMockupGate) over
// claims and returns its error-severity findings, with Severity normalized the
// same way RunAll does. It is the subset a renderer enforces as a security gate
// before emitting .RawHTML into the client-shared viewer (DX-AUD-08) —
// deliberately NOT the full RunAll suite, whose warning-severity relationship
// lints (orphan, body-edge-hint) must not block a render during draft authoring.
// A clean project (no mockup claims, or only gate-passing ones) yields a nil
// slice.
//
// Its caller is internal/serve's renderViewer (see disarmUngatedMockups there),
// which has no lint step of its own: "dossierx check" fails at lint before it
// ever renders, but serve loads-builds-renders, so without this the payload a
// human ran serve to go LOOK at was the payload serve executed. The standalone
// "render"/"catalog" verbs that used to be the callers were retired in v0.3.0,
// which left this function with none at all for a release — the shape of dead
// safety code, so if a future change removes serve's call, delete this too
// rather than leaving the comment asserting a gate nothing runs.
func MockupGateFindings(claims []model.Claim, cfg *config.Config) []Finding {
	var findings []Finding
	for _, c := range claims {
		findings = append(findings, checkMockupGate(c, cfg)...)
	}
	var errs []Finding
	for _, f := range findings {
		if f.Severity == "" {
			f.Severity = SeverityError
		}
		if f.Severity != SeverityWarning {
			errs = append(errs, f)
		}
	}
	return errs
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

// mockupAttr is one parsed HTML attribute: its raw (case-preserved) name,
// its value, and whether a value was present at all. A valueless attribute
// (hasValue == false, e.g. a bare "hidden" or "onerror") is a distinct case
// from an empty-valued one (class="") — the DX-AUD-07 fix must reject the
// former by default rather than ignore it the way the old double-quote-only
// regex silently did.
type mockupAttr struct {
	name     string
	value    string
	hasValue bool
}

// mockupTag is one parsed HTML tag: whether it is a closing tag, its
// (case-preserved) name, its attributes, and — for a tag whose text could
// not be reduced to clean name/value pairs (an unterminated tag or quoted
// value) — a malformed flag and human-readable reason.
type mockupTag struct {
	closing   bool
	name      string
	attrs     []mockupAttr
	malformed bool
	malfMsg   string
}

// scanMockupTags is a hand-rolled (no golang.org/x/net/html) HTML tag
// scanner for RawHTML content. Unlike the old double-quoted-only regex it
// replaces (DX-AUD-07), it recognizes attribute values in every quote form —
// double-quoted, single-quoted, unquoted, and valueless — and it tracks the
// active quote character while looking for a tag's terminating ">", so a ">"
// embedded inside a quoted value never truncates the scan and hides a
// trailing attribute (e.g. an onerror after alt="a > b"). The caller
// (checkMockupMarkup) then DEFAULT-DENIES: any parsed attribute that isn't an
// explicitly-permitted (name,value) pair is a finding. A bare "<" not
// followed by a tag-name letter is treated as literal text (matching how the
// old regex simply didn't match it), not a tag.
func scanMockupTags(raw string) []mockupTag {
	var tags []mockupTag
	n := len(raw)
	i := 0
	for i < n {
		lt := strings.IndexByte(raw[i:], '<')
		if lt < 0 {
			break
		}
		start := i + lt
		j := start + 1
		closing := false
		if j < n && raw[j] == '/' {
			closing = true
			j++
		}
		// A tag name must start with an ASCII letter; anything else means
		// this "<" is literal text, so skip past it and keep scanning.
		if j >= n || !isMockupLetter(raw[j]) {
			i = start + 1
			continue
		}
		nameStart := j
		for j < n && isMockupNameChar(raw[j]) {
			j++
		}
		name := raw[nameStart:j]

		attrs, end, malformed, malfMsg := scanMockupAttrs(raw, j)
		tags = append(tags, mockupTag{
			closing:   closing,
			name:      name,
			attrs:     attrs,
			malformed: malformed,
			malfMsg:   malfMsg,
		})
		if malformed {
			// The tag never reached a clean terminating ">"; there is no
			// reliable resume point after it, so stop scanning entirely
			// (the malformed finding already condemns the whole blob).
			break
		}
		i = end + 1 // resume just past the terminating ">"
	}
	return tags
}

// scanMockupAttrs parses the attribute region of a tag starting at j (just
// past the tag name) up to the terminating unquoted ">", returning the parsed
// attributes, the index of that ">", and — if the region can't be reduced to
// clean pairs before end-of-input — a malformed flag and reason. Every path
// advances j, so this cannot loop forever.
func scanMockupAttrs(raw string, j int) (attrs []mockupAttr, end int, malformed bool, malfMsg string) {
	n := len(raw)
	for {
		for j < n && isMockupSpace(raw[j]) {
			j++
		}
		if j >= n {
			return attrs, n, true, "tag is not terminated with '>'"
		}
		switch raw[j] {
		case '>':
			return attrs, j, false, ""
		case '/':
			// Self-closing slash (e.g. <br/> or <img .../>): tolerate it and
			// keep scanning for the terminating ">".
			j++
			continue
		}
		nameStart := j
		for j < n && !isMockupSpace(raw[j]) && raw[j] != '=' && raw[j] != '>' && raw[j] != '/' {
			j++
		}
		attrName := raw[nameStart:j]
		for j < n && isMockupSpace(raw[j]) {
			j++
		}
		if j < n && raw[j] == '=' {
			j++ // consume '='
			for j < n && isMockupSpace(raw[j]) {
				j++
			}
			if j >= n {
				return append(attrs, mockupAttr{name: attrName, hasValue: true}), n, true, "attribute value is not terminated before '>'"
			}
			q := raw[j]
			if q == '"' || q == '\'' {
				j++ // consume opening quote
				valStart := j
				for j < n && raw[j] != q {
					j++
				}
				if j >= n {
					return append(attrs, mockupAttr{name: attrName, value: raw[valStart:], hasValue: true}), n, true, "quoted attribute value is not terminated before '>'"
				}
				attrs = append(attrs, mockupAttr{name: attrName, value: raw[valStart:j], hasValue: true})
				j++ // consume closing quote
			} else {
				valStart := j
				for j < n && !isMockupSpace(raw[j]) && raw[j] != '>' {
					j++
				}
				attrs = append(attrs, mockupAttr{name: attrName, value: raw[valStart:j], hasValue: true})
			}
		} else {
			attrs = append(attrs, mockupAttr{name: attrName, hasValue: false})
		}
	}
}

// stripCtrlAndSpace removes every ASCII control byte and whitespace/space
// (code point <= 0x20, plus DEL 0x7f) from s. It mirrors the markdown
// renderer's schemeOf normalization (internal/render/markdown) so the img-src
// relative-only gate and the markdown anchor scheme gate defend the same class
// of control-char scheme evasion (e.g. "ht\ttp://host", "\x01//host") the same
// way. A browser strips these bytes before resolving a URL, so the lint must
// too before deciding whether a value is relative.
func stripCtrlAndSpace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if c := s[i]; c > 0x20 && c != 0x7f {
			b.WriteByte(c)
		}
	}
	return b.String()
}

func isMockupLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func isMockupNameChar(b byte) bool {
	return isMockupLetter(b) || (b >= '0' && b <= '9')
}

func isMockupSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f'
}

// checkMockupMarkup scans raw for tags outside mockupAllowedTags, attributes
// other than a single, allowlisted-class-valued "class" (plus relative src /
// free-text alt on img), inline event handlers, style attributes, and
// non-relative URLs — the tag/attribute and CSS-class allowlist legs of the
// mockup gate. It DEFAULT-DENIES (DX-AUD-07): every parsed attribute, in any
// quote form, must reduce to an explicitly-permitted pair or it is a finding.
func checkMockupMarkup(claimID, raw string) []Finding {
	var findings []Finding
	add := func(msg string) {
		findings = append(findings, Finding{
			LintName: "raw-html-scope",
			ClaimID:  claimID,
			Message:  msg,
		})
	}

	for _, tag := range scanMockupTags(raw) {
		if tag.malformed {
			add(fmt.Sprintf("raw_html contains malformed markup at tag <%s>: %s", tag.name, tag.malfMsg))
			continue
		}
		name := strings.ToLower(tag.name)
		if !mockupAllowedTags[name] {
			slash := ""
			if tag.closing {
				slash = "/"
			}
			add(fmt.Sprintf("raw_html contains disallowed tag <%s%s> (only div, span, b, br, img are allowed)", slash, name))
			continue
		}
		if tag.closing {
			if len(tag.attrs) > 0 {
				add(fmt.Sprintf("raw_html closing tag </%s> must not carry attributes", name))
			}
			continue
		}
		for _, attr := range tag.attrs {
			checkMockupAttr(name, attr, add)
		}
	}

	return findings
}

// checkMockupAttr applies the per-attribute allowlist for a start tag. Only
// class (allowlisted-token-valued) on any allowed tag, and src (relative-only)
// / alt (free text) on img, are permitted; every other attribute name — and
// any of these carried without a value — is a hard finding, so an event
// handler, style, external src, or unknown attribute cannot slip through in
// any quote form.
func checkMockupAttr(tagName string, attr mockupAttr, add func(string)) {
	name := strings.ToLower(attr.name)
	switch {
	case name == "":
		add(fmt.Sprintf("raw_html <%s> has a malformed, nameless attribute", tagName))
	case mockupOnAttrPattern.MatchString(name):
		add(fmt.Sprintf("raw_html <%s> has disallowed inline event-handler attribute %q", tagName, attr.name))
	case name == "style":
		add(fmt.Sprintf("raw_html <%s> has a disallowed style attribute (RawHTML markup must be style-free)", tagName))
	case name == "class":
		if !attr.hasValue {
			add(fmt.Sprintf("raw_html <%s> has a valueless class attribute", tagName))
			return
		}
		for _, token := range strings.Fields(attr.value) {
			if !mockupClassTokenPattern.MatchString(token) {
				add(fmt.Sprintf("raw_html <%s> class %q is not in the .gcp-*/.mockup-* CSS-class allowlist", tagName, token))
			}
		}
	case tagName == "img" && name == "alt":
		if !attr.hasValue {
			add("raw_html <img> has a valueless alt attribute")
		}
	case tagName == "img" && name == "src":
		if !attr.hasValue {
			add("raw_html <img> has a valueless src attribute")
			return
		}
		// Entity-decode AND strip ASCII control/whitespace bytes before the
		// relative-only check. mockupAbsoluteURLPattern matches raw bytes and its
		// scheme class excludes control chars while it only tolerates *leading*
		// whitespace, so without this an encoded absolute URL (src="&#47;&#47;host")
		// OR a control byte smuggled inside/ahead of the scheme
		// (src="ht&#9;tp://host", a literal embedded tab, a leading NUL/SOH) would
		// slip past it as a "relative" path — yet a browser strips those bytes and
		// loads the external URL. stripCtrlAndSpace mirrors the markdown renderer's
		// schemeOf normalization so both boundaries default-deny the same class.
		if mockupAbsoluteURLPattern.MatchString(stripCtrlAndSpace(html.UnescapeString(attr.value))) {
			add(fmt.Sprintf("raw_html <img> src=%q is a non-relative URL, which is disallowed", attr.value))
		}
	default:
		add(fmt.Sprintf("raw_html <%s> has disallowed attribute %q (only class is permitted on div/span/b; img also allows relative src and alt)", tagName, attr.name))
	}
}
