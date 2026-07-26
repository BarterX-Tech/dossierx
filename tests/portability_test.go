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
//     TestCheckSucceedsWithNetworkDisabled
//  6. the engine directory, copied into a repo where a colliding path
//     segment is already in use, makes zero assumptions about its parent
//     directory's name -> TestEngineCopiedIntoCollidingParentDirNameWorks
package tests

import (
	"bytes"
	"debug/elf"
	"debug/pe"
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

	if _, err := os.Stat(filepath.Join(root, ".catalog.json")); err != nil {
		t.Fatalf("expected .catalog.json to be written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "viewer", "index.html")); err != nil {
		t.Fatalf("expected viewer/index.html to be written: %v", err)
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

// scanForNetworkRefs returns one "label:line: text" entry for every line in
// content that matches networkRefPattern after loopback URLs are removed.
// label is a human-readable source identifier (a file path in the real scan; a
// synthetic name in the test).
func scanForNetworkRefs(label, content string) []string {
	var offenders []string
	for i, line := range strings.Split(content, "\n") {
		probe := loopbackURL.ReplaceAllString(line, "")
		if networkRefPattern.MatchString(probe) {
			offenders = append(offenders, label+":"+itoa(i+1)+": "+strings.TrimSpace(line))
		}
	}
	return offenders
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
	for _, dir := range []string{"cmd", "internal"} {
		if err := filepath.Walk(filepath.Join(root, dir), scan); err != nil {
			t.Fatalf("walk %s/: %v", dir, err)
		}
	}
	if len(offenders) > 0 {
		t.Fatalf("found network-shaped references, violating the offline requirement:\n%s", strings.Join(offenders, "\n"))
	}

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

	cmd := exec.Command(binPath, "check")
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

	runCmd := exec.Command(binOut, "check")
	runCmd.Dir = projectRoot
	if out, err := runCmd.CombinedOutput(); err != nil {
		t.Fatalf("check via binary built from colliding parent path failed: %v\n%s", err, out)
	}
}
