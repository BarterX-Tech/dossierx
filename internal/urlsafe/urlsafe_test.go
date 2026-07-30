package urlsafe

import "testing"

// TestIsOffOrigin is the whole gate, stated as a table. Every shape the four
// pre-existing copies of this rule between them knew about appears here, plus
// the shapes only one of them knew — which is the point of the package.
func TestIsOffOrigin(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		// --- the hole this package was created to close ---------------------
		// A browser normalises "\" to "/" in the authority position of an
		// http/https URL, so every one of these loads off-origin. The regexp in
		// internal/lint/raw_html_scope.go blocked only the first of the four.
		{"protocol-relative slash-slash", "//evil.example/p.png", true},
		{"authority backslash-backslash", `\\evil.example/p.png`, true},
		{"authority slash-backslash", `/\evil.example/p.png`, true},
		{"authority backslash-slash", `\/evil.example/p.png`, true},
		{"bare double slash", "//", true},
		{"bare double backslash", `\\`, true},
		{"three slashes", "///evil.example", true},

		// --- root-relative ---------------------------------------------------
		{"root-relative path", "/assets/x.png", true},
		{"bare slash", "/", true},
		{"bare backslash", `\`, true},

		// --- any explicit scheme, not a denylist -----------------------------
		{"http", "http://evil.example/p.png", true},
		{"https", "https://evil.example/p.png", true},
		{"uppercase scheme", "HTTP://evil.example/p.png", true},
		{"mixed-case scheme", "HtTpS://evil.example/p.png", true},
		{"javascript", "javascript:alert(1)", true},
		{"data", "data:image/png;base64,AAAA", true},
		{"vbscript", "vbscript:msgbox", true},
		{"mailto", "mailto:a@b.example", true},
		{"unheard-of scheme", "wibble:whatever", true},
		{"scheme with plus dot dash", "a+b-c.d:x", true},
		{"single-letter scheme", "a:b", true},
		{"scheme-only, no path", "http:", true},

		// --- control-character and whitespace evasion inside a scheme --------
		{"literal tab inside scheme", "ht\ttp://evil.example/p.png", true},
		{"literal newline inside scheme", "ht\ntp://evil.example/p.png", true},
		{"literal CR inside scheme", "ht\rtp://evil.example/p.png", true},
		{"NUL inside scheme", "ht\x00tp://evil.example/p.png", true},
		{"SOH before authority", "\x01//evil.example/p.png", true},
		{"DEL before authority", "\x7f//evil.example/p.png", true},
		{"leading spaces before scheme", "   javascript:alert(1)", true},
		{"leading spaces before authority", "  //evil.example/p.png", true},
		{"space inside scheme", "java script:alert(1)", true},
		{"trailing space after scheme", "javascript :alert(1)", true},

		// --- HTML-entity-encoded evasion -------------------------------------
		{"decimal-entity slashes", "&#47;&#47;evil.example/p.png", true},
		{"hex-entity slashes", "&#x2f;&#x2F;evil.example/p.png", true},
		{"entity backslashes", `&#92;&#92;evil.example/p.png`, true},
		{"mixed entity and literal slash", "&#47;/evil.example/p.png", true},
		{"entity colon in scheme", "http&#x3a;//evil.example/p.png", true},
		{"entity tab inside scheme", "ht&#9;tp://evil.example/p.png", true},
		{"entity newline inside scheme", "http&#10;://evil.example/p.png", true},
		{"hex entity tab inside scheme", "ht&#x9;tp://evil.example/p.png", true},
		{"named entity Tab inside scheme", "ht&Tab;tp://evil.example/p.png", true},
		{"entity-encoded javascript scheme letter", "&#106;avascript:alert(1)", true},

		// --- not a path at all ------------------------------------------------
		{"empty", "", true},
		{"only whitespace", "   ", true},
		{"only control bytes", "\x00\x01\x7f", true},
		{"fragment only", "#frag", true},
		{"query only", "?q=1", true},
		{"relative path with query", "assets/x.png?q=1", true},
		{"relative path with fragment", "assets/x.png#f", true},

		// --- the legitimate relative forms, which MUST pass -------------------
		{"bare filename", "x.png", false},
		{"dot-slash", "./x.png", false},
		{"nested relative", "assets/diagram.png", false},
		{"deep nested relative", "assets/sub/dir/diagram.png", false},
		{"dot-slash nested", "./assets/diagram.png", false},
		{"interior dot segment", "assets/./diagram.png", false},
		{"parent traversal is relative, not off-origin", "../diagrams/x.svg", false},
		{"interior parent traversal", "assets/../x.png", false},
		{"backslash traversal is relative, not off-origin", `assets\..\x.png`, false},
		{"filename containing a colon-free plus", "a+b.png", false},
		{"filename with an entity-encoded space stays relative", "assets/a&#32;b.png", false},
		{"percent-encoded slashes are not slashes", "%2f%2fevil.example", false},
		{"dot-dot alone", "..", false},
		{"filename that starts with a digit", "1abc.png", false},
		// "1abc:x" has no scheme (RFC 3986 requires ALPHA first) and does not
		// start with a slash byte, so it is a relative reference. Whether it is
		// a legal ASSET is ImageSrc's question, not this gate's.
		{"digit-led pseudo-scheme is not a scheme", "1abc:x", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsOffOrigin(tc.raw); got != tc.want {
				t.Errorf("IsOffOrigin(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

// TestIsRelativePath pins the ".." clause that separates this from IsOffOrigin,
// including the backslash-separated spelling of a traversal.
func TestIsRelativePath(t *testing.T) {
	cases := []struct {
		raw  string
		want bool
	}{
		{"x.png", true},
		{"./x.png", true},
		{"assets/x.png", true},
		{"assets/sub/x.png", true},
		{"assets/./x.png", true},
		{"..", false},
		{"../x.png", false},
		{"assets/../x.png", false},
		{`assets\..\x.png`, false},
		{`..\x.png`, false},
		{"assets/..%2fx.png", true}, // percent-encoding is not decoded here
		{"//evil.example/x.png", false},
		{`\\evil.example/x.png`, false},
		{`/\evil.example/x.png`, false},
		{`\/evil.example/x.png`, false},
		{"/x.png", false},
		{"http://evil.example/x.png", false},
		{"", false},
		{"#f", false},
		{"?q", false},
		// A ".." spelled with entities is still a traversal, because the decode
		// happens before the split.
		{"assets/&#46;&#46;/x.png", false},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			if got := IsRelativePath(tc.raw); got != tc.want {
				t.Errorf("IsRelativePath(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

// TestSchemeOf pins the RFC 3986 grammar and the strip-then-read evasion
// resistance every other gate here is built on.
func TestSchemeOf(t *testing.T) {
	cases := []struct {
		url        string
		wantScheme string
		wantOK     bool
	}{
		{"http://h", "http", true},
		{"HTTP://h", "http", true},
		{"  JavaScript:x", "javascript", true},
		{"java\tscript:x", "javascript", true},
		{"java\nscript:x", "javascript", true},
		{"a+b-c.d:x", "a+b-c.d", true},
		{"mailto:a@b", "mailto", true},
		{"h1:x", "h1", true},
		{":x", "", false},     // empty scheme is no scheme
		{"1abc:x", "", false}, // must start with ALPHA
		{"/a:b", "", false},   // "/" before the colon
		{"#a:b", "", false},   // "#" before the colon
		{"?a:b", "", false},   // "?" before the colon
		{"a_b:x", "", false},  // "_" is not in the scheme grammar
		{"relative/x", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.url, func(t *testing.T) {
			gotScheme, gotOK := SchemeOf(tc.url)
			if gotScheme != tc.wantScheme || gotOK != tc.wantOK {
				t.Errorf("SchemeOf(%q) = %q, %v; want %q, %v", tc.url, gotScheme, gotOK, tc.wantScheme, tc.wantOK)
			}
		})
	}
}

// TestIsNetworkPath pins all four authority spellings plus the strip.
func TestIsNetworkPath(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		{"//h/x", true},
		{`\\h/x`, true},
		{`/\h/x`, true},
		{`\/h/x`, true},
		{"  //h/x", true},
		{"/\t/h/x", true},
		{"\x01//h/x", true},
		{"/x", false},
		{`\x`, false},
		{"x/y", false},
		{"http://h", false}, // schemed, so not a NETWORK-path reference
		{"", false},
		{"/", false},
	}
	for _, tc := range cases {
		t.Run(tc.url, func(t *testing.T) {
			if got := IsNetworkPath(tc.url); got != tc.want {
				t.Errorf("IsNetworkPath(%q) = %v, want %v", tc.url, got, tc.want)
			}
		})
	}
}

// TestIsAllowedHref pins the anchor allowlist EXACTLY as internal/render/markdown
// enforced it before this package existed — including the two ways it is
// deliberately different from IsOffOrigin (a fragment and a query are fine in an
// href) and the one way it is deliberately weaker (it does not entity-decode,
// because the href it guards is emitted html-escaped).
func TestIsAllowedHref(t *testing.T) {
	allowed := []string{
		"http://h/x", "https://h/x", "HTTPS://h/x", "mailto:a@b.example",
		"relative/x.md", "./x.md", "../x.md", "x.md", "#frag", "?q=1",
		"/root-relative", `\x`, "",
		// Not decoded: this reaches the browser as literal text, never a scheme.
		"&#106;avascript:alert(1)",
	}
	refused := []string{
		"javascript:alert(1)", "  JavaScript:alert(1)", "java\tscript:alert(1)",
		"data:text/html,<script>", "vbscript:msgbox", "file:///etc/passwd",
		"ftp://h/x", "//h/x", `\\h/x`, `/\h/x`, `\/h/x`, "  //h/x",
	}
	for _, u := range allowed {
		if !IsAllowedHref(u) {
			t.Errorf("IsAllowedHref(%q) = false, want true", u)
		}
	}
	for _, u := range refused {
		if IsAllowedHref(u) {
			t.Errorf("IsAllowedHref(%q) = true, want false", u)
		}
	}
}

// TestStripCtrlAndSpace pins the exact byte set, since two gates disagreeing
// about it is the failure this package exists to prevent.
func TestStripCtrlAndSpace(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"abc", "abc"},
		{" a b ", "ab"},
		{"a\tb\nc\rd", "abcd"},
		{"a\x00b\x01c\x1fd", "abcd"},
		{"a\x7fb", "ab"},
		{"a\x21b", "a!b"},       // 0x21 is the first kept byte
		{"a\x20b", "ab"},        // 0x20 (space) is dropped
		{"caf\xc3\xa9", "café"}, // multi-byte UTF-8 survives byte-wise
	}
	for _, tc := range cases {
		if got := StripCtrlAndSpace(tc.in); got != tc.want {
			t.Errorf("StripCtrlAndSpace(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
