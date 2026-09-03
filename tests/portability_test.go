// This file covers the "Portability & build" edge-case category:
//
//  1. facet/module iteration order is always explicitly sorted before
//     output (never leaks Go's randomized map iteration order) — covered
//     at the package level by internal/catalog/catalog_test.go's
//     TestBuild_LargeListDeterminism, which asserts doc.Claims is *strictly*
//     sorted by id (not merely reproducible), so it fails if that sort is
//     ever removed. No additional test needed here; see that file for the
//     row-1 proof.
//  2. a second toy project, with entirely different facet/module names and
//     count than testdata/fixture-basic, passes "dossierx check" with zero
//     engine source changes -> TestSecondToyProjectDifferentFacetsChecksClean
//  3. the binary cross-compiles via GOOS/GOARCH for linux and windows, and
//     the engine is free of OS-specific syscalls/hardcoded path separators
//     -> TestCrossCompilesForLinuxAndWindows,
//     TestNoHardcodedPathSeparatorsOrOSSpecificSyscalls
//  4. build-machine absolute paths are not embedded in the binary
//     -> TestTrimpathBuildDoesNotEmbedBuildMachinePaths
//  5. the engine works fully offline: no CDN fonts, no remote schema
//     fetch, no telemetry call -> TestNoNetworkReferencesAnywhereInEngine,
//     TestCheckSucceedsWithNetworkDisabled. Commentary in .js is exempt from
//     that scan and executable code is not, both directions pinned by
//     TestStripJSComments.
//  6. the engine directory, copied into a repo where a colliding path
//     segment is already in use, makes zero assumptions about its parent
//     directory's name -> TestEngineCopiedIntoCollidingParentDirNameWorks
package tests

import (
	"bytes"
	"crypto/sha256"
	"debug/elf"
	"debug/pe"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// moduleRoot is this module's absolute root directory (the parent of
// tests/), computed once for tests that need to copy or rebuild the engine
// from source.
func moduleRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}
	return root
}

// copyEngineTree copies every file of the engine module (go.mod, go.sum,
// cmd/, internal/, testdata/) into dst, preserving relative paths. It skips
// this tests/ directory itself (irrelevant to a downstream consumer of the
// engine), any already-built artifacts (.git, bin/), and .claude/ (this
// repo's own Claude Code skill wiring, e.g. .claude/skills/dossierx-*
// symlinks into skills/ -- local editor tooling, not part of the engine a
// downstream consumer builds, and filepath.Walk's Lstat-based traversal
// below does not follow symlinks the way a plain file copy would need to).
func copyEngineTree(t *testing.T, dst string) {
	t.Helper()
	src := moduleRoot(t)

	skipTop := map[string]bool{
		".git":    true,
		"bin":     true,
		".claude": true,
	}

	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		top := strings.SplitN(rel, string(filepath.Separator), 2)[0]
		if skipTop[top] {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
	if err != nil {
		t.Fatalf("copy engine tree into %s: %v", dst, err)
	}
}

// ---------------------------------------------------------------------
// Row 2: a second toy project, with facet/module names and a facet COUNT
// entirely different from testdata/fixture-basic, must pass "dossierx check"
// against the already-built binary — no engine source touched to get
// there.
// ---------------------------------------------------------------------

func TestSecondToyProjectDifferentFacetsChecksClean(t *testing.T) {
	root := t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Five facets (fixture-basic has two), two modules (fixture-basic has
	// one), names sharing nothing with fixture-basic's "contract"/
	// "internals"/"widget" vocabulary.
	cfg := "schema_version: 1\n" +
		"facets:\n  - blueprint\n  - lineage\n  - risk\n  - onboarding\n  - retirement\n" +
		"modules:\n  - sprocket\n  - gizmo\n" +
		"claims_dir: claims\n"
	if err := os.WriteFile(filepath.Join(root, "project.config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	claims := map[string]string{
		"sprocket-blueprint.yaml": "id: sprocket.blueprint.overview\n" +
			"facet: blueprint\nmodule: sprocket\nstatus: draft\nlayout: card\n" +
			"body: sprocket blueprint overview.\n" +
			"governed_by:\n  type: none\n  reason: toy project fixture\n" +
			"rests_on:\n  - gizmo.lineage.overview\n",
		"gizmo-lineage.yaml": "id: gizmo.lineage.overview\n" +
			"facet: lineage\nmodule: gizmo\nstatus: draft\nlayout: card\n" +
			"body: gizmo lineage overview.\n" +
			"governed_by:\n  type: none\n  reason: toy project fixture\n",
		"gizmo-risk.yaml": "id: gizmo.risk.overview\n" +
			"facet: risk\nmodule: gizmo\nstatus: draft\nlayout: card\n" +
			"body: gizmo risk overview.\n" +
			"governed_by:\n  type: none\n  reason: toy project fixture\n" +
			"rests_on:\n  - gizmo.lineage.overview\n",
		"sprocket-onboarding.yaml": "id: sprocket.onboarding.steps\n" +
			"facet: onboarding\nmodule: sprocket\nstatus: draft\n" +
			"steps:\n  - unbox the sprocket\n  - attach to the gizmo\n" +
			"governed_by:\n  type: none\n  reason: toy project fixture\n" +
			"rests_on:\n  - sprocket.blueprint.overview\n",
		"sprocket-retirement.yaml": "id: sprocket.retirement.overview\n" +
			"facet: retirement\nmodule: sprocket\nstatus: draft\nlayout: card\n" +
			"body: sprocket retirement overview.\n" +
			"governed_by:\n  type: none\n  reason: toy project fixture\n" +
			"rests_on:\n  - sprocket.onboarding.steps\n",
	}
	for name, body := range claims {
		if err := os.WriteFile(filepath.Join(claimsDir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	stdout, stderr, code := run(t, root, "check")
	if code != 0 {
		t.Fatalf("check on second toy project (different facets/modules): expected exit 0, got %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "check: OK") {
		t.Fatalf("expected check to report OK, got: %s", stdout)
	}

	if _, err := os.Stat(filepath.Join(root, "build", "catalog", "catalog.json")); err != nil {
		t.Fatalf("expected build/catalog/catalog.json to be written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "build", "viewer", "index.html")); err != nil {
		t.Fatalf("expected build/viewer/index.html to be written: %v", err)
	}
}

// ---------------------------------------------------------------------
// Row 3a: the binary cross-compiles for linux and windows via GOOS/GOARCH
// and produces a real, correctly-formatted executable for each target (no
// build errors that would indicate a darwin-only dependency).
// ---------------------------------------------------------------------

func TestCrossCompilesForLinuxAndWindows(t *testing.T) {
	root := moduleRoot(t)

	targets := []struct {
		goos, goarch, ext string
	}{
		{"linux", "amd64", ""},
		{"linux", "arm64", ""},
		{"windows", "amd64", ".exe"},
	}

	for _, tgt := range targets {
		tgt := tgt
		t.Run(tgt.goos+"_"+tgt.goarch, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "dossierx"+tgt.ext)
			cmd := exec.Command("go", "build", "-o", out, "./cmd/dossierx")
			cmd.Dir = root
			cmd.Env = append(os.Environ(),
				"GOOS="+tgt.goos,
				"GOARCH="+tgt.goarch,
				"CGO_ENABLED=0",
			)
			if outBytes, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("cross-compile for GOOS=%s GOARCH=%s failed: %v\n%s", tgt.goos, tgt.goarch, err, outBytes)
			}

			data, err := os.ReadFile(out)
			if err != nil {
				t.Fatalf("read cross-compiled binary: %v", err)
			}
			if len(data) == 0 {
				t.Fatalf("cross-compiled binary for %s/%s is empty", tgt.goos, tgt.goarch)
			}

			// Confirm the produced file is actually a valid executable in
			// the target platform's own format, not just "some bytes" —
			// this is what would break if the cross-compile silently
			// produced a broken/host-format binary.
			switch tgt.goos {
			case "linux":
				f, err := elf.Open(out)
				if err != nil {
					t.Fatalf("cross-compiled linux binary is not valid ELF: %v", err)
				}
				defer f.Close()
			case "windows":
				f, err := pe.Open(out)
				if err != nil {
					t.Fatalf("cross-compiled windows binary is not valid PE: %v", err)
				}
				defer f.Close()
			}
		})
	}
}

// ---------------------------------------------------------------------
// Row 3b: static guard against the classes of code that would make a
// cross-compiled binary behave differently per OS: raw syscall use and
// hand-rolled path-separator string concatenation instead of path/filepath.
// This test walks actual engine source (excluding tests) so it fails the
// moment either pattern is (re)introduced.
// ---------------------------------------------------------------------

func TestNoHardcodedPathSeparatorsOrOSSpecificSyscalls(t *testing.T) {
	root := moduleRoot(t)

	// Matches string concatenation of a variable/expression with a
	// literal "/" or "\\" path separator, e.g. `dir + "/" + file`, which
	// bypasses filepath.Join and can behave inconsistently across OSes.
	hardcodedSep := regexp.MustCompile(`\+\s*"[/\\]"\s*\+|\+\s*"[/\\]"$`)
	syscallUse := regexp.MustCompile(`\bsyscall\.`)

	var offenders []string
	walkErr := filepath.Walk(filepath.Join(root, "cmd"), scanForPatterns(t, hardcodedSep, syscallUse, &offenders))
	if walkErr != nil {
		t.Fatalf("walk cmd/: %v", walkErr)
	}
	walkErr = filepath.Walk(filepath.Join(root, "internal"), scanForPatterns(t, hardcodedSep, syscallUse, &offenders))
	if walkErr != nil {
		t.Fatalf("walk internal/: %v", walkErr)
	}

	if len(offenders) > 0 {
		t.Fatalf("found OS-specific path/syscall patterns that risk breaking cross-platform builds:\n%s", strings.Join(offenders, "\n"))
	}
}

// portableSignalConst matches the termination-signal constants a graceful
// server legitimately catches. syscall.SIGTERM/SIGINT are defined on every Go
// target (including Windows), so — unlike a genuinely platform-specific
// syscall — they do NOT make a cross-compiled binary diverge per OS; they are
// the idiomatic, portable way to handle Ctrl-C / `kill`, and "dossierx serve"
// needs them for its SIGINT/SIGTERM shutdown handler. Only these two constants
// are exempted below; any OTHER syscall.* reference still trips the guard.
var portableSignalConst = regexp.MustCompile(`syscall\.SIG(TERM|INT)\b`)

func scanForPatterns(t *testing.T, hardcodedSep, syscallUse *regexp.Regexp, offenders *[]string) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(data), "\n") {
			if hardcodedSep.MatchString(line) {
				*offenders = append(*offenders, path+":"+itoa(i+1)+": hardcoded path separator: "+strings.TrimSpace(line))
			}
			if syscallUse.MatchString(line) {
				// Exempt only the portable termination-signal constants: strip
				// them and re-test, so any other syscall use on the same line
				// still fails.
				if residual := portableSignalConst.ReplaceAllString(line, ""); syscallUse.MatchString(residual) {
					*offenders = append(*offenders, path+":"+itoa(i+1)+": raw syscall use: "+strings.TrimSpace(line))
				}
			}
		}
		return nil
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

// ---------------------------------------------------------------------
// Row 4: build-machine absolute paths must not be embedded in the binary.
// The canonical build (documented in the Makefile) passes -trimpath for
// exactly this reason; this test proves that build actually achieves it,
// using a build directory whose absolute path is unique enough that it
// could not coincidentally match anything else in the binary.
// ---------------------------------------------------------------------

func TestTrimpathBuildDoesNotEmbedBuildMachinePaths(t *testing.T) {
	// Copy the engine into a temp directory with a deliberately distinctive
	// name, so any embedded build path would be unambiguous in the binary.
	uniqueRoot := filepath.Join(t.TempDir(), "unlikely-marker-4f8c2a91-portability-check")
	if err := os.MkdirAll(uniqueRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	copyEngineTree(t, uniqueRoot)

	out := filepath.Join(t.TempDir(), "dossierx-trimpath")
	cmd := exec.Command("go", "build", "-trimpath", "-o", out, "./cmd/dossierx")
	cmd.Dir = uniqueRoot
	if outBytes, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("trimpath build failed: %v\n%s", err, outBytes)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read trimpath binary: %v", err)
	}

	if bytes.Contains(data, []byte(uniqueRoot)) {
		t.Fatalf("trimpath binary embeds the build machine's absolute path %q", uniqueRoot)
	}
	// Deliberately NOT also asserting non-containment of the bare
	// os.TempDir() value (e.g. "/tmp" on Linux): that's a short, common
	// string the Go runtime/stdlib can legitimately embed on its own
	// (unrelated to this build's path), so checking for it produces
	// false failures independent of whether -trimpath actually worked.
	// uniqueRoot — a path built from t.TempDir() plus a distinctive
	// marker — is the meaningful, non-coincidental assertion; it already
	// covers "does this binary leak the build machine's temp directory."

	// Negative control: prove this detection method actually works by
	// showing a build WITHOUT -trimpath from the same unique tree does
	// leak the path — if this stopped being true (e.g. a future Go
	// toolchain change), the positive assertions above would no longer be
	// meaningfully tested. Skipped on Windows: GitHub's windows-latest
	// runner image builds are trimpath-equivalent even without the flag
	// (almost certainly a GOFLAGS default baked into the image), which
	// defeats only this self-check, not the actual guarantee — the
	// positive assertions above (which are what DossierX ships) already
	// ran and passed on this platform.
	if runtime.GOOS == "windows" {
		return
	}
	outNoTrim := filepath.Join(t.TempDir(), "dossierx-no-trimpath")
	cmdNoTrim := exec.Command("go", "build", "-o", outNoTrim, "./cmd/dossierx")
	cmdNoTrim.Dir = uniqueRoot
	if outBytes, err := cmdNoTrim.CombinedOutput(); err != nil {
		t.Fatalf("non-trimpath build failed: %v\n%s", err, outBytes)
	}
	dataNoTrim, err := os.ReadFile(outNoTrim)
	if err != nil {
		t.Fatalf("read non-trimpath binary: %v", err)
	}
	if !bytes.Contains(dataNoTrim, []byte(uniqueRoot)) {
		t.Fatalf("negative control failed: expected a non-trimpath build to embed the build path %q, but it did not — detection method may be broken", uniqueRoot)
	}
}

// ---------------------------------------------------------------------
// Row 5: the engine must work fully offline. Static scan for any
// CDN/remote-fetch/telemetry-shaped reference anywhere in engine source,
// templates, or CSS, plus a live check run with network access removed via
// an unroutable proxy environment (so any accidental network call would
// hang/fail loudly rather than silently succeeding through a real
// connection).
// ---------------------------------------------------------------------

// networkRefPattern matches any CDN/remote-fetch/telemetry-shaped reference
// that would violate the fully-offline requirement. It is shared by the
// engine-source scan below and its positive-control self-test so the two can
// never drift apart.
var networkRefPattern = regexp.MustCompile(`(?i)https?://|cdn\.|fonts\.googleapis|fonts\.gstatic|analytics|telemetry|sentry|segment\.io`)

// loopbackURL matches an http(s) URL whose host is the loopback interface
// (127.0.0.1 or localhost). Such a reference is NOT external egress — it is the
// local "dossierx serve" address (the server binds loopback only) and is fully
// offline-compatible. These are stripped before the offline scan so the guard
// still catches every real remote CDN/telemetry/external URL while permitting
// the local server's own address in serve source and its admission checks.
var loopbackURL = regexp.MustCompile(`(?i)https?://(127\.0\.0\.1|localhost)\b`)

// stripJSComments blanks every "//" line comment and "/* … */" block comment
// in JavaScript source, replacing each commented byte with a space and leaving
// every newline in place. Its output therefore has the same length, the same
// line count and the same line numbering as its input, which is what lets the
// scan below report an offender's real line number after stripping.
//
// Why this exists: the property TestNoNetworkReferencesAnywhereInEngine
// protects is that the shipped engine makes no network request, and that must
// not weaken. But a citation in a comment makes no request. Failing the build
// over a doc comment naming the paper an algorithm came from pushes an author
// toward writing worse comments — precisely the wrong incentive for the
// viewer's client files, whose non-obvious algorithms deserve citation.
//
// It is STRING-LITERAL AWARE for ', " and `, honouring backslash escapes, so
// a "//" inside a string does not start a comment and
// `const u = "https://evil.example/x";` still fails. Executable code is not
// exempt from anything; only commentary is.
//
// Two limits, stated rather than hidden:
//
//   - A regular-expression literal is treated as division, so an adjacent
//     "//" inside one would read as a comment start and blank the rest of that
//     line. JS regex literals escape their interior slashes (`/\/\//`), so an
//     adjacent pair does not occur in practice; distinguishing a regex literal
//     from division needs a real tokenizer, which is not worth carrying here.
//   - A backtick template literal containing a nested backtick inside a
//     ${…} interpolation would end the literal early.
//
// Both failure modes blank MORE than they should, never less on executable
// code that reads as code — they can hide an offender on one line, they cannot
// invent an exemption for a URL sitting in a plain string or a call.
func stripJSComments(content string) string {
	const (
		inCode = iota
		inLineComment
		inBlockComment
		inString
	)

	out := []byte(content)
	state := inCode
	var quote byte

	for i := 0; i < len(out); i++ {
		c := out[i]
		switch state {
		case inCode:
			switch {
			case c == '/' && i+1 < len(out) && out[i+1] == '/':
				out[i], out[i+1] = ' ', ' '
				i++
				state = inLineComment
			case c == '/' && i+1 < len(out) && out[i+1] == '*':
				out[i], out[i+1] = ' ', ' '
				i++
				state = inBlockComment
			case c == '"' || c == '\'' || c == '`':
				quote = c
				state = inString
			}
		case inString:
			switch {
			case c == '\\':
				i++ // the escaped byte is never a delimiter
			case c == quote:
				state = inCode
			case c == '\n' && quote != '`':
				// An unterminated ' or " literal ends at the newline in real
				// JS. Recovering here stops one stray quote from exempting
				// the whole rest of the file.
				state = inCode
			}
		case inLineComment:
			if c == '\n' {
				state = inCode
				continue // the newline itself is kept: line numbers must not move
			}
			out[i] = ' '
		case inBlockComment:
			if c == '\n' {
				continue // kept, same reason
			}
			if c == '*' && i+1 < len(out) && out[i+1] == '/' {
				out[i], out[i+1] = ' ', ' '
				i++
				state = inCode
				continue
			}
			out[i] = ' '
		}
	}
	return string(out)
}

// scanForNetworkRefs returns one "label:line: text" entry for every line in
// content that matches networkRefPattern after loopback URLs are removed.
// label is a human-readable source identifier (a file path in the real scan; a
// synthetic name in the test).
//
// When label names a .js file, comments are blanked first (see
// stripJSComments). The exemption is scoped to .js deliberately: .go, .html
// and .css are matched exactly as before. The reported text is the ORIGINAL
// line, not the stripped one, so a human reading a failure sees the source as
// written.
func scanForNetworkRefs(label, content string) []string {
	probeSource := content
	if strings.EqualFold(filepath.Ext(label), ".js") {
		probeSource = stripJSComments(content)
	}

	source := strings.Split(content, "\n")
	probed := strings.Split(probeSource, "\n")

	var offenders []string
	for i, line := range probed {
		probe := loopbackURL.ReplaceAllString(line, "")
		if !networkRefPattern.MatchString(probe) {
			continue
		}
		raw := line
		if i < len(source) {
			raw = source[i]
		}
		offenders = append(offenders, label+":"+itoa(i+1)+": "+strings.TrimSpace(raw))
	}
	return offenders
}

// vendoredMermaidPath is the ONLY file under internal/render that is not
// engine source: the mermaid build the Build order tab renders with. Its
// version, licence, hash record and re-vendoring notes live in
// third_party/mermaid/ (not beside the file, because everything tracked
// under internal/render must be embedded and fingerprinted, and metadata is
// neither).
const vendoredMermaidPath = "internal/render/viewer/template/vendor/mermaid.min.js"

func vendoredMermaidSHA256(t *testing.T, data []byte) string {
	t.Helper()
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func recordedMermaidSHA256(t *testing.T, root string) string {
	t.Helper()
	rec, err := os.ReadFile(filepath.Join(root, "third_party", "mermaid", "mermaid.SHA256"))
	if err != nil {
		t.Fatalf("read the recorded mermaid hash: %v", err)
	}
	fields := strings.Fields(string(rec))
	if len(fields) == 0 || len(fields[0]) != 64 {
		t.Fatalf("third_party/mermaid/mermaid.SHA256 does not hold a sha256 hex digest: %q", rec)
	}
	return fields[0]
}

// TestVendoredMermaidHashMatchesRecord is its own named test so a re-vendor
// without a hash update is a named failure: the offline allowlist in
// TestNoNetworkReferencesAnywhereInEngine was measured against the recorded
// build and is only as true as that record.
func TestVendoredMermaidHashMatchesRecord(t *testing.T) {
	root := moduleRoot(t)
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(vendoredMermaidPath)))
	if err != nil {
		t.Fatalf("the vendored mermaid build must exist at %s: %v", vendoredMermaidPath, err)
	}
	if len(data) < 1<<20 {
		t.Fatalf("vendored mermaid.min.js is %d bytes; a real mermaid build is several MB", len(data))
	}
	got, want := vendoredMermaidSHA256(t, data), recordedMermaidSHA256(t, root)
	if got != want {
		t.Fatalf("vendored mermaid.min.js sha256 = %s, third_party/mermaid/mermaid.SHA256 = %s: re-vendored without updating the record (and re-measuring the offline allowlist)", got, want)
	}
	version, err := os.ReadFile(filepath.Join(root, "third_party", "mermaid", "mermaid.VERSION"))
	if err != nil || strings.TrimSpace(string(version)) == "" {
		t.Fatalf("third_party/mermaid/mermaid.VERSION must name the vendored version: %v %q", err, version)
	}
	if _, err := os.Stat(filepath.Join(root, "third_party", "mermaid", "mermaid.LICENSE")); err != nil {
		t.Fatalf("third_party/mermaid/mermaid.LICENSE must be present: %v", err)
	}
}

// TestVendoredMermaidIsAClassicScript is the cheap, non-browser half of the
// vendoring check. The bundle is injected inline as template.JS into a plain
// <script> (no type="module"), so two shapes of a re-vendored file would ship a
// viewer whose diagrams never render with nothing but the browser suite to
// notice: an ESM-only dist file (a top-level `export`/`import` statement is a
// syntax error in a classic script), and a bundle carrying a literal `<script`
// or `</script` inside a string, which the HTML tokenizer reads as the end of
// the script element and swallows the rest of the document as script text — a
// blank viewer over file:// and under serve alike. `<!--` is tolerated in
// script data and only counted here.
func TestVendoredMermaidIsAClassicScript(t *testing.T) {
	root := moduleRoot(t)
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(vendoredMermaidPath)))
	if err != nil {
		t.Fatalf("the vendored mermaid build must exist at %s: %v", vendoredMermaidPath, err)
	}
	src := string(data)
	lower := strings.ToLower(src)
	for _, tag := range []string{"<script", "</script"} {
		if n := strings.Count(lower, tag); n != 0 {
			t.Errorf("%s contains %q %d time(s); injected inline into a <script> element it would end the element early and the browser would read the rest of the viewer as script text", vendoredMermaidPath, tag, n)
		}
	}
	// A classic script cannot carry a top-level import/export. The bundle is
	// minified, so "top level" is approximated by a statement boundary: the
	// start of the file, or a `;`, `}` or newline immediately before the
	// keyword, followed by a space, `{` or `*`, which is how an ESM statement
	// begins and how a property named "export"/"import" (obj.export, "import":)
	// does not.
	esm := regexp.MustCompile(`(?m)(^|[;}
])\s*(export|import)\s*(\{|\*|\s+[A-Za-z_$"'])`)
	if m := esm.FindAllString(src, 5); len(m) > 0 {
		t.Errorf("%s carries what looks like a top-level ESM statement (%q); a plain <script> cannot load it and every diagram would fail to render", vendoredMermaidPath, m)
	}
	t.Logf("%s: %d `<!--` sequence(s), tolerated in script data", vendoredMermaidPath, strings.Count(src, "<!--"))
}

func TestNoNetworkReferencesAnywhereInEngine(t *testing.T) {
	root := moduleRoot(t)

	// Scope the scan to the engine's own source trees ONLY (cmd/ and
	// internal/), skipping *_test.go — matching scanForPatterns above. This
	// permanently excludes site/ (the marketing website, including its
	// gitignored dist/ build bundles, which legitimately reference external
	// URLs/CDNs), node_modules, and testdata/ (fixtures that legitimately
	// carry example URLs). Walking the whole module root instead made this
	// test red locally after a `site` build yet falsely green on a clean CI
	// checkout where dist/ is absent.
	exts := map[string]bool{".go": true, ".html": true, ".css": true, ".js": true}
	var offenders []string
	scan := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !exts[filepath.Ext(path)] {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		offenders = append(offenders, scanForNetworkRefs(path, string(data))...)
		return nil
	}
	vendored := filepath.Join(root, filepath.FromSlash(vendoredMermaidPath))
	for _, dir := range []string{"cmd", "internal"} {
		if err := filepath.Walk(filepath.Join(root, dir), func(path string, info os.FileInfo, err error) error {
			if err == nil && path == vendored {
				// The ONE exempted path. It is not skipped: the block after
				// this walk scans it with the full pattern against an
				// allowlist measured from the file actually vendored.
				return nil
			}
			return scan(path, info, err)
		}); err != nil {
			t.Fatalf("walk %s/: %v", dir, err)
		}
	}
	if len(offenders) > 0 {
		t.Fatalf("found network-shaped references, violating the offline requirement:\n%s", strings.Join(offenders, "\n"))
	}

	// The vendored mermaid build (internal/render/viewer/template/vendor/
	// mermaid.min.js, mermaid 11.17.2 — third_party/mermaid/ records it) is a
	// 3.5 MB third-party bundle whose minified source carries http:// strings
	// that make no request: SVG/XML namespaces, licence pointers, and
	// runtime error-message strings a comment stripper cannot remove. It is
	// the one file the walk above hands to this block instead of the scan,
	// and the exemption is an ALLOWLIST over the same pattern, not a weaker
	// pattern and not "skip this file": the file must exist, its SHA-256 must
	// equal third_party/mermaid/mermaid.SHA256 (TestVendoredMermaidHashMatchesRecord
	// is the same pin as its own named test), and every match of the FULL
	// networkRefPattern, widened to the token around it, must be on the list
	// below. The list was produced by running
	//   grep -oE 'https?://[A-Za-z0-9.-]+' vendor/mermaid.min.js | sort | uniq -c
	// and each of the six other arms individually over 11.17.2, and it is
	// regenerated and re-justified whenever mermaid.SHA256 changes — which the
	// hash test forces by failing on a re-vendor without a hash update.
	t.Run("vendored_mermaid_is_scanned_against_a_measured_allowlist", func(t *testing.T) {
		data, err := os.ReadFile(vendored)
		if err != nil {
			t.Fatalf("the vendored mermaid build must exist: %v", err)
		}
		if got, want := vendoredMermaidSHA256(t, data), recordedMermaidSHA256(t, root); got != want {
			t.Fatalf("vendored mermaid.min.js sha256 %s != third_party/mermaid/mermaid.SHA256 %s; the allowlist below was measured against the recorded build", got, want)
		}
		allowed := map[string]string{
			// https?:// arm — hosts, with the count measured on 11.17.2.
			"http://www.w3.org":        "34: SVG/XML/XHTML namespace URIs on created elements",
			"https://chevrotain.io":    "20: parser-library error-message strings (\"For further details see: ...\")",
			"https://github.com":       "10: licence pointers for bundled jquery/d3/dagre code",
			"https://lodash.com":       "4: lodash licence header",
			"https://openjsf.org":      "2: OpenJS Foundation licence text",
			"http://underscorejs.org":  "2: underscore licence header",
			"http://en.wikipedia.org":  "2: algorithm citations in comments the minifier kept as strings",
			"https://en.wikipedia.org": "1: same",
			"https://tldrlegal.com":    "1: licence pointer",
			"https://langium.org":      "1: langium licence pointer",
			"https://jquery.org":       "1: jquery licence pointer",
			"http://opensource.org":    "1: MIT licence pointer",
			"http://engelschall.com":   "1: sprintf.js licence header",
			"http://":                  "1: marked's autolink prefix, prepended to a bare www. link in label markdown; a string, never a fetch",
			// telemetry arm — langium's LSP protocol constant, not a call.
			"e.TelemetryEventNotification": "2: LSP message-type identifier",
			"telemetry/event":              "1: LSP method name string",
			// cdn., fonts.googleapis, fonts.gstatic, analytics, sentry and
			// segment.io: zero matches in 11.17.2.
		}
		tokenRe := regexp.MustCompile(`[A-Za-z0-9_./:-]+`)
		seen := map[string]int{}
		var unexplained []string
		text := string(data)
		for _, loc := range networkRefPattern.FindAllStringIndex(text, -1) {
			// Widen the match to the token around it so "telemetry" is judged
			// as the identifier it sits in and a URL as its scheme+host.
			start, end := loc[0], loc[1]
			for start > 0 && tokenRe.MatchString(text[start-1:start]) {
				start--
			}
			for end < len(text) && tokenRe.MatchString(text[end:end+1]) {
				end++
			}
			token := text[start:end]
			if hostMatch := regexp.MustCompile(`^https?://[A-Za-z0-9.-]+`).FindString(token); hostMatch != "" {
				token = hostMatch
			}
			if _, ok := allowed[token]; !ok {
				unexplained = append(unexplained, token)
			}
			seen[token]++
		}
		if len(unexplained) > 0 {
			t.Fatalf("vendored mermaid.min.js carries network-shaped tokens not on the measured allowlist (re-measure after a re-vendor and justify each): %v", unexplained)
		}
		for token := range allowed {
			if seen[token] == 0 {
				t.Errorf("allowlist entry %q matched nothing in the vendored file; the list is stale against the vendored build", token)
			}
		}
		if len(seen) == 0 {
			t.Fatal("the full pattern matched nothing in a 3.5 MB bundle known to carry namespace URIs; the scan is not looking at the file")
		}
	})

	// Positive control: the scoped walk above must not have weakened
	// detection. Run the very same scan predicate against synthetic
	// in-memory content carrying a genuine external URL and assert it IS
	// flagged — proving the scanner still catches a real offline-requirement
	// leak (a scoped walk that scanned nothing would silently pass forever).
	t.Run("positive_control_detects_real_leak", func(t *testing.T) {
		content := "const ok = 1;\nfetch(\"https://evil.example/x\");\n"
		hits := scanForNetworkRefs("synthetic.js", content)
		if len(hits) == 0 {
			t.Fatalf("positive control: scanner failed to detect a real external URL in %q", content)
		}
	})
}

// TestStripJSComments pins both directions of the comment exemption at once:
// commentary in .js is exempt, and executable code is not — including a URL in
// a plain string, and including a string whose CONTENTS look like a comment.
//
// It asserts through scanForNetworkRefs rather than on stripJSComments' output
// text, because the scan is the thing with a contract; the stripping is an
// implementation detail of it. The one structural property asserted directly
// is line-count preservation, which is what makes reported line numbers true.
func TestStripJSComments(t *testing.T) {
	cases := []struct {
		name         string
		label        string
		content      string
		wantCount    int
		wantAtLine   int    // 1-based; 0 means "not asserted"
		wantInReport string // substring the offender text must carry
	}{
		{
			name:      "exempt: a URL in a line comment",
			label:     "synthetic.js",
			content:   "// Tarjan 1972, see https://doi.example/10.1137\nconst a = 1;\n",
			wantCount: 0,
		},
		{
			name:      "exempt: a URL in a block comment",
			label:     "synthetic.js",
			content:   "/*\n * Algorithm from https://doi.example/10.1137\n */\nconst a = 1;\n",
			wantCount: 0,
		},
		{
			name:      "exempt: a block comment closed and reopened on one line",
			label:     "synthetic.js",
			content:   "const a = 1; /* https://a.example */ const b = 2; /* https://b.example */\n",
			wantCount: 0,
		},
		{
			// The existing positive control, byte for byte. If the stripping
			// ever swallowed this, the offline guarantee would be gone and
			// every other test here would still be green.
			name:         "offender: fetch in executable code",
			label:        "synthetic.js",
			content:      "const ok = 1;\nfetch(\"https://evil.example/x\");\n",
			wantCount:    1,
			wantAtLine:   2,
			wantInReport: "fetch(",
		},
		{
			name:         "offender: a URL in a plain string literal",
			label:        "synthetic.js",
			content:      "const u = \"https://evil.example/x\";\n",
			wantCount:    1,
			wantAtLine:   1,
			wantInReport: "const u =",
		},
		{
			// The string-literal-awareness case: the "//" here is DATA. A
			// naive stripper would treat it as a comment start and exempt the
			// URL that follows it.
			name:         "offender: a string whose contents look like a comment",
			label:        "synthetic.js",
			content:      "var s = \"// not a comment https://evil.example/x\";\n",
			wantCount:    1,
			wantAtLine:   1,
			wantInReport: "not a comment",
		},
		{
			name:         "offender: a URL in a template literal",
			label:        "synthetic.js",
			content:      "const u = `https://evil.example/x`;\n",
			wantCount:    1,
			wantAtLine:   1,
			wantInReport: "const u =",
		},
		{
			name:         "offender: a URL in a single-quoted string with an escaped quote",
			label:        "synthetic.js",
			content:      "const u = 'it\\'s https://evil.example/x';\n",
			wantCount:    1,
			wantAtLine:   1,
			wantInReport: "const u =",
		},
		{
			name:      "exempt: a trailing comment after clean code",
			label:     "synthetic.js",
			content:   "foo(); // https://doc.example/x\n",
			wantCount: 0,
		},
		{
			// Half a line exempt, half a line still scanned.
			name:         "offender: the code half of a line carrying a trailing comment",
			label:        "synthetic.js",
			content:      "fetch(\"https://evil.example/a\"); // and see https://doc.example/b\n",
			wantCount:    1,
			wantAtLine:   1,
			wantInReport: "evil.example/a",
		},
		{
			// Line numbering is the property the space-preserving strip buys.
			// The URL is on line 5 and must be reported as line 5, after four
			// lines of comment above it.
			name:       "offender: line number survives the stripping",
			label:      "synthetic.js",
			content:    "/* a\n b\n c */\n// d\nfetch(\"https://evil.example/x\");\n",
			wantCount:  1,
			wantAtLine: 5,
		},
		{
			// Scope: the exemption is .js only. A Go comment carrying a URL
			// still fails, exactly as before this change.
			name:       "offender: a comment in .go is NOT exempt",
			label:      "synthetic.go",
			content:    "// see https://evil.example/x\nvar a = 1\n",
			wantCount:  1,
			wantAtLine: 1,
		},
		{
			name:       "offender: a comment in .css is NOT exempt",
			label:      "synthetic.css",
			content:    "/* https://fonts.googleapis.com/x */\nbody { color: red }\n",
			wantCount:  1,
			wantAtLine: 1,
		},
		{
			name:       "offender: a comment in .html is NOT exempt",
			label:      "synthetic.html",
			content:    "<!-- https://cdn.example/x -->\n<p>hi</p>\n",
			wantCount:  1,
			wantAtLine: 1,
		},
		{
			// The loopback exemption must survive the new pre-pass.
			name:      "exempt: the local serve address is still not egress",
			label:     "synthetic.js",
			content:   "const base = \"http://127.0.0.1:7777/api/graph\";\n",
			wantCount: 0,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			offenders := scanForNetworkRefs(tc.label, tc.content)
			if len(offenders) != tc.wantCount {
				t.Fatalf("got %d offenders, want %d: %v", len(offenders), tc.wantCount, offenders)
			}
			if tc.wantCount == 0 {
				return
			}
			if tc.wantAtLine > 0 {
				want := tc.label + ":" + itoa(tc.wantAtLine) + ":"
				if !strings.HasPrefix(offenders[0], want) {
					t.Errorf("offender %q does not start with %q; a stripped line must keep its number", offenders[0], want)
				}
			}
			if tc.wantInReport != "" && !strings.Contains(offenders[0], tc.wantInReport) {
				t.Errorf("offender %q does not carry %q; the report must show the line as written", offenders[0], tc.wantInReport)
			}
		})
	}

	// Structural: stripping must never change the number of lines, which is
	// the invariant every line number above rests on.
	t.Run("line_count_is_preserved", func(t *testing.T) {
		samples := []string{
			"",
			"\n",
			"const a = 1;\n// x\n/* y\n z */\nconst b = 2;\n",
			"var s = \"unterminated;\nfetch(\"https://evil.example/x\");\n",
			"/* never closed\nconst a = 1;\n",
			"const t = `multi\nline`;\n",
		}
		for _, s := range samples {
			got := stripJSComments(s)
			if a, b := strings.Count(s, "\n"), strings.Count(got, "\n"); a != b {
				t.Errorf("stripJSComments(%q) changed the newline count: %d -> %d", s, a, b)
			}
			if len(got) != len(s) {
				t.Errorf("stripJSComments(%q) changed the byte length: %d -> %d", s, len(s), len(got))
			}
		}
	})
}

func TestCheckSucceedsWithNetworkDisabled(t *testing.T) {
	root := t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - offlinemod\nclaims_dir: claims\n"
	if err := os.WriteFile(filepath.Join(root, "project.config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	claim := "id: offlinemod.contract.overview\n" +
		"facet: contract\nmodule: offlinemod\nstatus: draft\nlayout: card\n" +
		"body: offline fixture claim.\n" +
		"governed_by:\n  type: none\n  reason: offline fixture\n"
	if err := os.WriteFile(filepath.Join(claimsDir, "overview.yaml"), []byte(claim), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binPath, "--format", "text", "check")
	cmd.Dir = root
	// Route any accidental outbound HTTP(S) through an unroutable address
	// (TEST-NET-1, RFC 5737) so a real network call would time out/fail
	// loudly rather than quietly succeeding because this machine happens
	// to have internet access.
	cmd.Env = append(os.Environ(),
		"HTTP_PROXY=http://192.0.2.1:9",
		"HTTPS_PROXY=http://192.0.2.1:9",
		"NO_PROXY=",
	)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		t.Fatalf("check failed with network egress blackholed (proves a hidden network dependency): %v\nstdout: %s\nstderr: %s", err, outBuf.String(), errBuf.String())
	}
	if !strings.Contains(outBuf.String(), "check: OK") {
		t.Fatalf("expected check: OK, got: %s", outBuf.String())
	}
}

// ---------------------------------------------------------------------
// Row 6: the engine directory, copied into a repo where a colliding
// path segment is already in use higher up the tree, must build and run
// correctly — it must make zero assumptions about its parent directory's
// name (not "engine", not "nested", not anything else).
// ---------------------------------------------------------------------

func TestEngineCopiedIntoCollidingParentDirNameWorks(t *testing.T) {
	// Deliberately hostile nesting: a "nested" segment already exists
	// twice above an unrelated project, and the engine itself lands under
	// a directory that is NOT named "engine".
	nestedParent := filepath.Join(t.TempDir(), "nested", "nested", "some-other-project", "totally-different-name")
	if err := os.MkdirAll(nestedParent, 0o755); err != nil {
		t.Fatal(err)
	}
	copyEngineTree(t, nestedParent)

	// Confirm it builds from its new, colliding location.
	buildCmd := exec.Command("go", "build", "./...")
	buildCmd.Dir = nestedParent
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("go build ./... from colliding parent path failed: %v\n%s", err, out)
	}

	// And confirm the resulting binary actually works end to end against a
	// fresh project fixture placed elsewhere (not under the engine tree at
	// all), proving no assumption leaked about paths relative to the
	// engine's own location either.
	binOut := filepath.Join(t.TempDir(), "dossierx"+exeSuffix())
	runBuildCmd := exec.Command("go", "build", "-o", binOut, "./cmd/dossierx")
	runBuildCmd.Dir = nestedParent
	if out, err := runBuildCmd.CombinedOutput(); err != nil {
		t.Fatalf("build binary from colliding parent path failed: %v\n%s", err, out)
	}

	projectRoot := t.TempDir()
	claimsDir := filepath.Join(projectRoot, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - collidemod\nclaims_dir: claims\n"
	if err := os.WriteFile(filepath.Join(projectRoot, "project.config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	claim := "id: collidemod.contract.overview\n" +
		"facet: contract\nmodule: collidemod\nstatus: draft\nlayout: card\n" +
		"body: fixture claim exercised from a colliding-parent-path build.\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
	if err := os.WriteFile(filepath.Join(claimsDir, "overview.yaml"), []byte(claim), 0o644); err != nil {
		t.Fatal(err)
	}

	runCmd := exec.Command(binOut, "--format", "text", "check")
	runCmd.Dir = projectRoot
	if out, err := runCmd.CombinedOutput(); err != nil {
		t.Fatalf("check via binary built from colliding parent path failed: %v\n%s", err, out)
	}
}
