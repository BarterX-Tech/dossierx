package lint

import (
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestRawHTMLScope(t *testing.T) {
	cases := []struct {
		name       string
		claims     []model.Claim
		wantClaims []string
	}{
		{
			name: "passing: plain markdown body, no raw HTML",
			claims: []model.Claim{
				{ID: "a.contract.one", Body: "A *widget* is the smallest unit, see `code`."},
			},
			wantClaims: nil,
		},
		{
			name: "passing: benign inline tags like <code> or <b> are not denylisted",
			claims: []model.Claim{
				{ID: "a.contract.one", Body: "Use <code>foo</code> and <b>bar</b>."},
			},
			wantClaims: nil,
		},
		{
			name: "failing: script tag in body",
			claims: []model.Claim{
				{ID: "a.contract.one", Body: "before <script>alert(1)</script> after"},
			},
			wantClaims: []string{"a.contract.one"},
		},
		{
			name: "failing: inline event handler attribute in a step",
			claims: []model.Claim{
				{ID: "a.contract.one", Steps: []string{"click <a onclick=\"doThing()\">here</a>"}},
			},
			wantClaims: []string{"a.contract.one"},
		},
		{
			name: "failing: iframe tag in a row value",
			claims: []model.Claim{
				{
					ID: "a.internals.fields",
					Rows: []model.Row{
						{"field": "id", "notes": "<iframe src=\"evil\"></iframe>"},
					},
				},
			},
			wantClaims: []string{"a.internals.fields"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RawHTMLScope{}.Check(tc.claims, nil)
			gotIDs := findingClaimIDs(got)
			assertStringSlicesEqual(t, gotIDs, tc.wantClaims)
		})
	}
}

// mockupTestConfig is a project config allowlisting only "widget" to
// author layout: mockup claims, per the round-3 hardening spec's "start
// with just one module" instruction.
func mockupTestConfig() *config.Config {
	return &config.Config{
		SchemaVersion: 1,
		Facets:        []string{"contract", "internals"},
		Modules:       []string{"widget", "other"},
		ClaimsDir:     "claims",
		MockupModules: []string{"widget"},
	}
}

// validMockupClaim is a fully valid layout: mockup claim: allowlisted
// module, layout: mockup, markup using only allowed tags/attributes/
// classes, and raw_html_reviewed: true. Individual test cases mutate a
// copy of this to break exactly one requirement at a time.
func validMockupClaim() model.Claim {
	return model.Claim{
		ID:              "widget.internals.console-mockup",
		Facet:           "internals",
		Module:          "widget",
		Layout:          model.LayoutMockup,
		RawHTML:         `<div class="gcp-row"><span class="mockup-badge">ERROR</span><b>boom</b></div>`,
		RawHTMLReviewed: true,
	}
}

func TestRawHTMLScope_Mockup(t *testing.T) {
	cases := []struct {
		name     string
		claim    model.Claim
		cfg      *config.Config
		wantFind bool
	}{
		{
			name:     "passing: fully valid mockup claim",
			claim:    validMockupClaim(),
			cfg:      mockupTestConfig(),
			wantFind: false,
		},
		{
			name: "failing: raw_html on a non-mockup layout",
			claim: func() model.Claim {
				c := validMockupClaim()
				c.Layout = model.LayoutCard
				return c
			}(),
			cfg:      mockupTestConfig(),
			wantFind: true,
		},
		{
			name: "failing: unlisted module authoring layout: mockup",
			claim: func() model.Claim {
				c := validMockupClaim()
				c.Module = "other"
				c.ID = "other.internals.console-mockup"
				return c
			}(),
			cfg:      mockupTestConfig(),
			wantFind: true,
		},
		{
			name: "failing: disallowed tag inside raw_html",
			claim: func() model.Claim {
				c := validMockupClaim()
				c.RawHTML = `<div class="gcp-row"><script>alert(1)</script></div>`
				return c
			}(),
			cfg:      mockupTestConfig(),
			wantFind: true,
		},
		{
			name: "failing: disallowed attribute inside raw_html",
			claim: func() model.Claim {
				c := validMockupClaim()
				c.RawHTML = `<div class="gcp-row" onclick="doThing()">boom</div>`
				return c
			}(),
			cfg:      mockupTestConfig(),
			wantFind: true,
		},
		{
			name: "failing: disallowed CSS class inside raw_html",
			claim: func() model.Claim {
				c := validMockupClaim()
				c.RawHTML = `<div class="not-allowlisted">boom</div>`
				return c
			}(),
			cfg:      mockupTestConfig(),
			wantFind: true,
		},
		{
			name: "failing: clean mockup claim with raw_html_reviewed false",
			claim: func() model.Claim {
				c := validMockupClaim()
				c.RawHTMLReviewed = false
				return c
			}(),
			cfg:      mockupTestConfig(),
			wantFind: true,
		},
		{
			name: "passing: img with relative src and alt for a diagram claim",
			claim: func() model.Claim {
				c := validMockupClaim()
				c.RawHTML = `<img class="mockup-diagram" src="../diagrams/health-state-machine.svg" alt="State diagram">`
				return c
			}(),
			cfg:      mockupTestConfig(),
			wantFind: false,
		},
		{
			name: "failing: img with an absolute/external src",
			claim: func() model.Claim {
				c := validMockupClaim()
				c.RawHTML = `<img class="mockup-diagram" src="//not-relative/x.svg" alt="State diagram">`
				return c
			}(),
			cfg:      mockupTestConfig(),
			wantFind: true,
		},
		{
			name: "failing: img with a disallowed attribute other than class/src/alt",
			claim: func() model.Claim {
				c := validMockupClaim()
				c.RawHTML = `<img class="mockup-diagram" src="../diagrams/x.svg" onerror="doThing()">`
				return c
			}(),
			cfg:      mockupTestConfig(),
			wantFind: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RawHTMLScope{}.Check([]model.Claim{tc.claim}, tc.cfg)
			if tc.wantFind && len(got) == 0 {
				t.Fatalf("expected at least one finding, got none")
			}
			if !tc.wantFind && len(got) != 0 {
				t.Fatalf("expected no findings, got: %+v", got)
			}
		})
	}
}

// TestRawHTMLScope_Mockup_DefaultDeny is the DX-AUD-07 regression: the
// mockup attribute gate must DEFAULT-DENY across every HTML quote form, not
// just double-quoted name="value" pairs. Before the fix, single-quoted,
// unquoted, and valueless attributes — and any attribute hidden behind a ">"
// embedded inside a quoted value — bypassed the allowlist entirely, so an
// onerror/style/external-src smuggled in any of those forms linted clean,
// locked, and rendered live (stored XSS on the blessed path). Each wantFind
// case below is exactly one such bypass; the wantFind==false cases prove the
// hand-rolled scanner does not false-positive on legitimate single-quoted
// markup or a ">" that is genuinely part of an attribute value.
func TestRawHTMLScope_Mockup_DefaultDeny(t *testing.T) {
	cases := []struct {
		name     string
		rawHTML  string
		wantFind bool
	}{
		{
			name:     "single-quoted event handler bypasses double-quote-only gate",
			rawHTML:  `<img class="mockup-diagram" src='../diagrams/x.svg' onerror='alert(1)'>`,
			wantFind: true,
		},
		{
			name:     "unquoted attributes bypass the gate",
			rawHTML:  `<img class="mockup-diagram" src=x onerror=alert(1)>`,
			wantFind: true,
		},
		{
			name:     "single-quoted external/absolute img src bypasses the gate",
			rawHTML:  `<img class="mockup-diagram" src='//evil.example/x.svg'>`,
			wantFind: true,
		},
		{
			name:     "decimal-entity-encoded protocol-relative img src is caught",
			rawHTML:  `<img class="mockup-diagram" src="&#47;&#47;evil.example/x.svg">`,
			wantFind: true,
		},
		{
			name:     "hex-entity-encoded absolute img src is caught",
			rawHTML:  `<img class="mockup-diagram" src="http&#x3a;//evil.example/x.svg">`,
			wantFind: true,
		},
		{
			name:     "a > embedded in a quoted value must not truncate the tag scan",
			rawHTML:  `<img class="mockup-diagram" src="../diagrams/x.svg" alt="a > b" onerror="alert(1)">`,
			wantFind: true,
		},
		{
			name:     "valueless event-handler attribute bypasses the gate",
			rawHTML:  `<img class="mockup-diagram" src="../diagrams/x.svg" onerror>`,
			wantFind: true,
		},
		{
			name:     "valueless non-allowlisted attribute is denied by default",
			rawHTML:  `<div class="gcp-row" hidden>boom</div>`,
			wantFind: true,
		},
		{
			name:     "unquoted disallowed class token is caught",
			rawHTML:  `<div class=not-allowlisted>boom</div>`,
			wantFind: true,
		},
		{
			name:     "legitimate > inside a quoted alt value with no smuggled attr is allowed",
			rawHTML:  `<img class="mockup-diagram" src="../diagrams/x.svg" alt="a > b">`,
			wantFind: false,
		},
		{
			name:     "single-quoted allowed class/src/alt on img stays valid",
			rawHTML:  `<img class='mockup-diagram' src='../diagrams/x.svg' alt='ok'>`,
			wantFind: false,
		},
		{
			name:     "single-quoted allowed classes on div/span/b stay valid",
			rawHTML:  `<div class='gcp-row'><span class='mockup-badge'>ERROR</span><b>boom</b></div>`,
			wantFind: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validMockupClaim()
			c.RawHTML = tc.rawHTML
			got := RawHTMLScope{}.Check([]model.Claim{c}, mockupTestConfig())
			if tc.wantFind && len(got) == 0 {
				t.Fatalf("expected at least one finding for %q, got none", tc.rawHTML)
			}
			if !tc.wantFind && len(got) != 0 {
				t.Fatalf("expected no findings for %q, got: %+v", tc.rawHTML, got)
			}
		})
	}
}

// TestRawHTMLScope_Mockup_BackslashAuthorityBypass is the regression for the
// hole this gate carried until internal/urlsafe existed.
//
// The check used to be a regexp, `(?i)^\s*([a-zA-Z][a-zA-Z0-9+.-]*:|//)`, which
// recognised the literal bytes "//" as an authority prefix and nothing else. A
// browser normalises "\" to "/" in the authority position of a URL under a
// special (http/https) scheme, so "\\host", "/\host" and "\/host" resolve
// off-origin exactly as "//host" does — and all three linted clean, which means
// they could be locked and then rendered live into the client-shared viewer,
// because this gate reviews the one field DossierX emits unescaped.
//
// Measured against the old regexp: "//evil.example/p.png" was blocked and the
// other three were NOT. Every row below must now be blocked, and the legitimate
// relative forms must still pass.
func TestRawHTMLScope_Mockup_BackslashAuthorityBypass(t *testing.T) {
	blocked := []string{
		"//evil.example/p.png",
		`\\evil.example/p.png`,
		`/\evil.example/p.png`,
		`\/evil.example/p.png`,
		`\\evil.example\p.png`,
		"///evil.example/p.png",
		// The entity spellings of the same three authorities.
		"&#47;&#47;evil.example/p.png",
		"&#92;&#92;evil.example/p.png",
		"&#47;&#92;evil.example/p.png",
		// A root-relative src is not same-repo either: amendment A4's rule is
		// "no leading /", and the first byte of every authority spelling above
		// is one of the two slash bytes, so one test covers both.
		"/evil.example/p.png",
	}
	allowed := []string{
		"../diagrams/health-state-machine.svg",
		"./x.png",
		"x.svg",
		"assets/sub/x.png",
	}

	run := func(src string) []Finding {
		c := validMockupClaim()
		c.RawHTML = `<img class="mockup-diagram" src="` + src + `" alt="ok">`
		return RawHTMLScope{}.Check([]model.Claim{c}, mockupTestConfig())
	}
	for _, src := range blocked {
		t.Run("blocked:"+src, func(t *testing.T) {
			got := run(src)
			if len(got) == 0 {
				t.Fatalf("src %q must be refused as non-relative, got no findings", src)
			}
		})
	}
	for _, src := range allowed {
		t.Run("allowed:"+src, func(t *testing.T) {
			if got := run(src); len(got) != 0 {
				t.Fatalf("src %q must still pass, got: %+v", src, got)
			}
		})
	}
}

// TestRawHTMLScope_Mockup_ControlCharSchemeEvasion is the DX-AUD-07/08
// regression for control-char scheme smuggling in an <img src>. A control char
// (tab, newline, or a non-\s control like NUL/SOH) embedded inside or ahead of
// the scheme breaks the scheme run, so a check reading the raw bytes treats the
// value as a relative path that lints clean, locks, and renders live — yet a
// browser strips the control byte and loads the external URL. The gate must
// entity-decode and strip every ASCII control byte and whitespace before
// testing, which is what urlsafe.IsOffOrigin does, so each wantFind case below
// is FLAGGED while a legitimate relative src still passes.
func TestRawHTMLScope_Mockup_ControlCharSchemeEvasion(t *testing.T) {
	cases := []struct {
		name     string
		rawHTML  string
		wantFind bool
	}{
		{
			name:     "decimal tab entity inside the scheme is caught",
			rawHTML:  `<img class="mockup-diagram" src="ht&#9;tp://evil.example/x.svg">`,
			wantFind: true,
		},
		{
			name:     "decimal newline entity inside the scheme is caught",
			rawHTML:  `<img class="mockup-diagram" src="http&#10;://evil.example/x.svg">`,
			wantFind: true,
		},
		{
			name:     "hex tab entity inside the scheme is caught",
			rawHTML:  `<img class="mockup-diagram" src="ht&#x9;tp://evil.example/x.svg">`,
			wantFind: true,
		},
		{
			name:     "literal embedded tab inside the scheme is caught",
			rawHTML:  "<img class=\"mockup-diagram\" src=\"ht\ttp://evil.example/x.svg\">",
			wantFind: true,
		},
		{
			name:     "leading non-whitespace control byte before a protocol-relative URL is caught",
			rawHTML:  "<img class=\"mockup-diagram\" src=\"\x01//evil.example/x.svg\">",
			wantFind: true,
		},
		{
			name:     "legitimate relative diagram src still passes",
			rawHTML:  `<img class="mockup-diagram" src="../diagrams/x.svg" alt="ok">`,
			wantFind: false,
		},
		{
			name:     "dot-slash relative src still passes",
			rawHTML:  `<img class="mockup-diagram" src="./x.png" alt="ok">`,
			wantFind: false,
		},
		{
			name:     "bare filename relative src still passes",
			rawHTML:  `<img class="mockup-diagram" src="x.svg" alt="ok">`,
			wantFind: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validMockupClaim()
			c.RawHTML = tc.rawHTML
			got := RawHTMLScope{}.Check([]model.Claim{c}, mockupTestConfig())
			if tc.wantFind && len(got) == 0 {
				t.Fatalf("expected at least one finding for %q, got none", tc.rawHTML)
			}
			if !tc.wantFind && len(got) != 0 {
				t.Fatalf("expected no findings for %q, got: %+v", tc.rawHTML, got)
			}
		})
	}
}

// TestRawHTMLScope_Mockup_LockGate confirms which layer enforces the
// raw_html_reviewed gate: it is this lint (raw-html-scope), not
// internal/lock itself — internal/lock.Lock refuses to lock any claim
// against which lint.RunAll (which includes this lint) reports an
// error-severity finding, so a RawHTML claim with raw_html_reviewed: false
// is blocked at lock time purely as a consequence of failing lint first.
// This test only asserts the lint-time half of that chain (this package
// cannot import internal/lock without a cycle); internal/lock's own tests
// are the source of truth for the lock-time refusal itself.
func TestRawHTMLScope_Mockup_LockGate(t *testing.T) {
	c := validMockupClaim()
	c.RawHTMLReviewed = false

	got := RawHTMLScope{}.Check([]model.Claim{c}, mockupTestConfig())
	found := false
	for _, f := range got {
		if f.Severity != SeverityWarning {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected an error-severity finding for raw_html_reviewed: false, got: %+v", got)
	}
}
