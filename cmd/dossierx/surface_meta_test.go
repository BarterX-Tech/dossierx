// surface_meta_test.go is the half of the surface inventory that watches the
// other half. surface_test.go extracts; these tests assert that each extraction
// saw EVERYTHING it claims to cover.
//
// The distinction matters because of how this document fails. A stale
// surface.json is loud — the generator regenerates it and the bytes differ. An
// extraction that silently sees less than it should is quiet: the document is
// internally consistent, the staleness test is green, and the gate goes green
// over a surface nobody inventoried. Every test in this file exists to make one
// of those quiet failures loud, by re-deriving the same set through a different
// route and refusing to agree with a narrower answer.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/lint"
	surfaceskills "github.com/BarterX-Tech/dossierx/skills"
)

// ---------------------------------------------------------------------
// the independent authority
// ---------------------------------------------------------------------

// surfaceTrackedFiles is `git ls-files`: the set of files this repository
// actually carries, as repo-relative slash paths.
//
// It is what the coverage cross-checks below re-derive from, and the reason they
// re-derive from IT is that it is a different authority than the emitter's. The
// emitter reaches the tree through its own scope constants and its own directory
// walks; if a test re-derived the same way it would inherit the same blind
// spots, which is precisely how a render package once dropped out of the
// fingerprint with every test green. git's answer does not move when a slice in
// surface_test.go does.
//
// A failure to run git fails the test rather than emptying the set: an empty
// file list would make every assertion here pass over zero comparisons, which is
// the shape of green this whole file exists to prevent.
func surfaceTrackedFiles(t *testing.T, root string) []string {
	t.Helper()
	cmd := exec.Command("git", "ls-files", "-z")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	var files []string
	for _, name := range strings.Split(string(out), "\x00") {
		if name != "" {
			files = append(files, filepath.ToSlash(name))
		}
	}
	if len(files) == 0 {
		t.Fatal("git ls-files reported no tracked files; every cross-check below would pass over nothing")
	}
	sort.Strings(files)
	return files
}

// surfaceToolchainEmbeds is `go list -json ./...`: the GO TOOLCHAIN's own answer
// to "which files does each package embed", keyed by the package's repo-relative
// directory and valued with repo-relative file paths.
//
// It is the second authority the embed coverage needs, and it is the right one
// for the same reason `git ls-files` is right for the render fingerprint: it
// shares no code, no scope constant and no reading strategy with the emitter.
// The emitter finds directives by walking a syntax tree and reading doc
// comments; go list resolves them the way the compiler does. When the emitter's
// reading narrowed — a //go:embed inside a grouped `var (...)` block attaches to
// the ValueSpec rather than the GenDecl, and the walk read only the latter — the
// cross-check that re-derived the set by CALLING the emitter agreed with it
// perfectly, and five shipped SKILL.md bundles left behaviour_fingerprint with
// the whole suite green. This one disagrees.
//
// A failure to run go list fails the test rather than emptying the map, for the
// same reason git failing does.
func surfaceToolchainEmbeds(t *testing.T, root string) map[string][]string {
	t.Helper()
	cmd := exec.Command("go", "list", "-json", "./...")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list -json ./...: %v", err)
	}

	// `go list -json` writes a CONCATENATION of objects, not an array.
	decoder := json.NewDecoder(bytes.NewReader(out))
	embeds := map[string][]string{}
	packages := 0
	for {
		var pkg struct {
			ImportPath string
			Dir        string
			EmbedFiles []string
		}
		if decodeErr := decoder.Decode(&pkg); decodeErr != nil {
			if errors.Is(decodeErr, io.EOF) {
				break
			}
			t.Fatalf("decode go list output: %v", decodeErr)
		}
		packages++
		if len(pkg.EmbedFiles) == 0 {
			continue
		}
		dir, relErr := filepath.Rel(root, pkg.Dir)
		if relErr != nil {
			t.Fatalf("locate package %s: %v", pkg.ImportPath, relErr)
		}
		for _, f := range pkg.EmbedFiles {
			embeds[filepath.ToSlash(dir)] = append(embeds[filepath.ToSlash(dir)], path.Join(filepath.ToSlash(dir), f))
		}
	}
	if packages == 0 {
		t.Fatal("go list reported no packages; every embed cross-check below would pass over nothing")
	}
	if len(embeds) == 0 {
		t.Fatal("go list reports no package embedding anything, but this binary ships templates and skill bundles; the cross-check has lost its authority")
	}
	for dir := range embeds {
		sort.Strings(embeds[dir])
	}
	return embeds
}

// ---------------------------------------------------------------------
// embedded files
// ---------------------------------------------------------------------

// TestSurfaceEmbeddedFilesMatchTheToolchain requires the emitter's own reading
// of //go:embed to agree, package for package and file for file, with what the
// Go toolchain says is embedded.
//
// Both fingerprints depend on that reading: render_fingerprint covers the viewer
// shell and the claim partials through it, and behaviour_fingerprint covers the
// five SKILL.md bundles `dossierx skills export` writes into other people's
// repositories. When the reading goes narrow, both fields keep hashing the .go
// files and quietly stop hashing the bytes — the document stays plausible, the
// staleness test stays green after one regeneration, and an edited skill bundle
// becomes invisible. This is the test that makes that narrowing loud.
func TestSurfaceEmbeddedFilesMatchTheToolchain(t *testing.T) {
	root := surfaceRepoRoot(t)
	want := surfaceToolchainEmbeds(t, root)

	got := map[string][]string{}
	for _, file := range surfaceTrackedFiles(t, root) {
		if !strings.HasSuffix(file, ".go") || strings.HasSuffix(file, "_test.go") {
			continue
		}
		embedded, err := embeddedFiles(root, file)
		if err != nil {
			t.Fatalf("embeddedFiles(%s): %v", file, err)
		}
		if len(embedded) == 0 {
			continue
		}
		got[path.Dir(file)] = append(got[path.Dir(file)], embedded...)
	}

	for dir, files := range want {
		for _, f := range files {
			if !containsStr(got[dir], f) {
				t.Errorf("the Go toolchain embeds %s into package %s and surface_test.go's own reading does not see it — every fingerprint over that package is blind to those bytes", f, dir)
			}
		}
	}
	for dir, files := range got {
		for _, f := range files {
			if !containsStr(want[dir], f) {
				t.Errorf("surface_test.go hashes %s as embedded by package %s; the Go toolchain does not embed it", f, dir)
			}
		}
	}
}

// ---------------------------------------------------------------------
// counts
// ---------------------------------------------------------------------

// TestSurfaceCountsAreTheEnforcedNumbers pins the five counts the release
// argues for. They are stated in README, in the skills, on the site and in
// internal/lint's own package doc, and each of those is prose that has already
// gone stale at least once: the site's meta description said "20-command" for
// two minor releases after the surface became nineteen.
//
// A disagreement here is a decision, never an adjustment. Either the extraction
// is wrong — in which case fixing this expectation would hide a broken gate —
// or the surface really moved, in which case the number is a thing somebody
// changes on purpose and writes down, exactly the way
// TestSurfaceIsTwentyFourLeavesUnderNineNouns treats the leaf count.
func TestSurfaceCountsAreTheEnforcedNumbers(t *testing.T) {
	root := surfaceRepoRoot(t)
	doc := buildSurfaceDoc(t, root)

	want := map[string]int{
		"nouns":       9,
		"commands":    24,
		"lint_rules":  38,
		"error_codes": 46,
		"http_routes": 14,
	}
	for name, expected := range want {
		if got := doc.Counts[name]; got != expected {
			t.Errorf("counts.%s = %d, want %d — either the extraction is wrong or the surface moved; both are findings, and neither is fixed by editing this number", name, got, expected)
		}
	}
	for name := range doc.Counts {
		if _, ok := want[name]; !ok {
			t.Errorf("counts.%s is not pinned; a derived count nobody checks is a count that goes stale", name)
		}
	}
}

// ---------------------------------------------------------------------
// error codes
// ---------------------------------------------------------------------

// codeConstREs are a second, independent reading of package cliout: raw text
// rather than a syntax tree. The two patterns are the two ways a Code
// declaration can be spotted without a type checker — an explicit `Code` type
// under any name, and a Code-prefixed name whose type is inferred or written as
// a conversion — and they are disjoint, so the counts add.
//
// They scan the WHOLE PACKAGE and not codes.go alone. Pinning both the
// extraction and its cross-check to one file is what let a Code constant
// declared in a sibling file sit in the binary, reachable by every client, with
// counts.error_codes reading 44 and every guard green.
var codeConstREs = []*regexp.Regexp{
	regexp.MustCompile(`(?m)^\s*[A-Za-z0-9_]+\s+Code\s*=\s*"[a-z_]+"`),
	regexp.MustCompile(`(?m)^\s*Code[A-Za-z0-9_]*\s*=\s*(?:Code\()?"[a-z_]+"`),
}

// TestSurfaceErrorCodesFindEveryConstantInThePackage is the meta-test the error
// code extraction needs: the parse must find EVERY Code constant package cliout
// declares, wherever it declares it — not every constant it recognised, and not
// every constant in one file.
func TestSurfaceErrorCodesFindEveryConstantInThePackage(t *testing.T) {
	root := surfaceRepoRoot(t)
	dir := filepath.Join(root, filepath.FromSlash(errorCodePackageDir))

	sources, err := goSourceFiles(dir)
	if err != nil {
		t.Fatalf("list %s: %v", dir, err)
	}
	if len(sources) == 0 {
		t.Fatalf("%s holds no non-test Go sources; this cross-check reads nothing", errorCodePackageDir)
	}

	declared := 0
	perFile := map[string]int{}
	for _, src := range sources {
		raw, readErr := os.ReadFile(src)
		if readErr != nil {
			t.Fatalf("read %s: %v", src, readErr)
		}
		for _, re := range codeConstREs {
			perFile[filepath.Base(src)] += len(re.FindAll(raw, -1))
		}
		declared += perFile[filepath.Base(src)]
	}
	if declared == 0 {
		t.Fatalf("%s: the text scan found no Code constants, so it cannot cross-check the parse", errorCodePackageDir)
	}

	codes, err := surfaceErrorCodes(root)
	if err != nil {
		t.Fatalf("surfaceErrorCodes: %v", err)
	}
	if len(codes) != declared {
		t.Errorf("the AST parse found %d error codes; a raw text scan of %s finds %d declarations (%v). One of the two readings is narrower than the package.",
			len(codes), errorCodePackageDir, declared, perFile)
	}
}

// ---------------------------------------------------------------------
// http routes
// ---------------------------------------------------------------------

// TestSurfaceRoutesCoverEveryRegistrationInServe counts mux registration calls
// across the whole of package serve and requires the extraction to have produced
// exactly that many patterns.
//
// It is the guard against the extraction's one plausible silent failure: a
// registration whose pattern argument it cannot fold being dropped instead of
// reported. Today there is exactly one such argument — assetRoutePattern,
// composed from components.AssetRoutePrefix across a package boundary — so the
// case is live, not hypothetical. It also catches a registration moving out of
// routes() into another file in the package, which the parse (which reads
// routes() alone) would otherwise miss entirely.
//
// BOTH ServeMux registration methods are counted, and the two names are written
// out here rather than shared with the extraction. They used to be one name in
// each file, which made mux.Handle a blind spot the two halves AGREED on: a
// Handle route was served by the binary, absent from http_routes, and this test
// counted 14 against a parse of 14 and passed. Two independent readings that
// share the definition of what they are reading are one reading.
func TestSurfaceRoutesCoverEveryRegistrationInServe(t *testing.T) {
	root := surfaceRepoRoot(t)
	dir := filepath.Join(root, "internal", "serve")

	files, err := goSourceFiles(dir)
	if err != nil {
		t.Fatalf("list %s: %v", dir, err)
	}
	calls := 0
	for _, src := range files {
		file, parseErr := parser.ParseFile(token.NewFileSet(), src, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", src, parseErr)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && (sel.Sel.Name == "HandleFunc" || sel.Sel.Name == "Handle") {
				calls++
			}
			return true
		})
	}
	if calls == 0 {
		t.Fatalf("no route registrations found under %s", dir)
	}

	routes, err := surfaceHTTPRoutes(root)
	if err != nil {
		t.Fatalf("surfaceHTTPRoutes: %v", err)
	}
	if len(routes) != calls {
		t.Errorf("the route parse produced %d patterns; package serve makes %d Handle/HandleFunc calls.\n"+
			"A pattern the parse cannot resolve must fail loudly, never be skipped — and a registration outside routes() is invisible to it.\nparsed: %v",
			len(routes), calls, routes)
	}
}

// ---------------------------------------------------------------------
// render fingerprint
// ---------------------------------------------------------------------

// renderTreePrefix is the render surface, spelled out HERE and deliberately not
// read from the emitter's renderRootDir.
//
// That sharing is what broke the previous version of this test. It read the
// emitter's own list of render packages and the emitter's own list of embed
// sources, so deleting "internal/render/markdown" from the first one narrowed
// the fingerprint AND narrowed this test's idea of what to check, in one edit,
// and the whole markdown renderer left the fingerprint with the build green. A
// cross-check that sources its scope from the thing it is checking is not a
// cross-check.
const renderTreePrefix = "internal/render/"

// TestSurfaceRenderFingerprintHashesEveryRenderSource re-derives the
// fingerprint's whole input set from `git ls-files` — a different mechanism,
// reading a different authority, sharing no scope variable with the emitter —
// and requires the two to agree exactly in both directions.
//
// This is the guard the field was written for. Naming markdown.go alone would
// miss markdown_tables.go — which carries the whole GFM table grammar — and a
// past release's silent change WAS table layout. The template half is checked
// the same way: every tracked NON-Go file under internal/render is a go:embed-ed
// template today, so all of them must be hashed. If a non-template file is ever
// added there this test fails and somebody decides what to do about it, which is
// the correct direction for a coverage assertion to err in.
func TestSurfaceRenderFingerprintHashesEveryRenderSource(t *testing.T) {
	root := surfaceRepoRoot(t)

	hashed, err := renderFingerprintFiles(root)
	if err != nil {
		t.Fatalf("renderFingerprintFiles: %v", err)
	}
	inSet := map[string]bool{}
	for _, f := range hashed {
		inSet[f] = true
	}

	var sources, templates []string
	for _, file := range surfaceTrackedFiles(t, root) {
		if !strings.HasPrefix(file, renderTreePrefix) {
			continue
		}
		switch {
		case strings.HasSuffix(file, "_test.go"):
			// A test is not part of what a claim renders as.
		case strings.HasSuffix(file, ".go"):
			sources = append(sources, file)
		default:
			templates = append(templates, file)
		}
	}
	if len(sources) == 0 || len(templates) == 0 {
		t.Fatalf("git shows %d render sources and %d templates under %s; with either at zero this test cross-checks nothing",
			len(sources), len(templates), renderTreePrefix)
	}

	for _, want := range sources {
		if !inSet[want] {
			t.Errorf("%s is a tracked render source and is NOT in the render fingerprint — the walk has narrowed into a file list", want)
		}
	}
	for _, want := range templates {
		if !inSet[want] {
			t.Errorf("%s is a tracked render template and is NOT in the render fingerprint — an edit to it would move nothing in surface.json", want)
		}
	}

	// And nothing crept in that git does not carry under internal/render: a
	// fingerprint over files nobody ships is noise the gate would have to diff.
	tracked := append(append([]string(nil), sources...), templates...)
	sort.Strings(tracked)
	for _, got := range hashed {
		if !containsStr(tracked, got) {
			t.Errorf("%s is hashed into the render fingerprint but is not a tracked non-test file under %s", got, renderTreePrefix)
		}
	}
}

// ---------------------------------------------------------------------
// behaviour fingerprint
// ---------------------------------------------------------------------

// behaviourExclusions are the Go packages this repository carries that are NOT
// compiled into the shipped binary, each with the reason it is absent from
// behaviour_fingerprint.
//
// It is a NAMED list rather than an absence, and it lives in the test rather
// than in the emitter, because those are the two properties that make the check
// below mean something. Every Go package git carries must be either
// fingerprinted or named here: dropping a tree from the emitter's own
// behaviourRoots — which is how skills/ came to be missing, one line — leaves
// its packages in neither column and this test red.
var behaviourExclusions = map[string]string{
	"scripts/normalize-claims": "a one-off repository maintenance tool. It is in nobody's binary and is not embedded, so nothing it does can reach a client.",
	"tests/procedures":         "the procedure replay suite: it EXECUTES the documented procedures against a fixture project and asserts the outcome each one promises. Nothing here is compiled into the shipped binary. It is excluded for the same reason the rest of tests/ is, and named here rather than left absent so that deleting the suite is a deliberate edit a human reviews rather than a package quietly leaving both columns.",
}

// TestSurfaceBehaviourFingerprintCoversEveryPackage requires an entry for every
// Go package this repository carries that is not explicitly excluded above, and
// requires each entry's INPUTS to include every source of that package plus
// everything those sources embed. It re-derives all of that from `git ls-files`,
// sharing no scope variable with the emitter.
//
// A package missing here is a package whose behaviour can change without moving
// one byte of surface.json. So is a package present but hashed over less than it
// ships — which is what skills/ would have been had it been added with its
// embedded SKILL.md bundles left out of the hash: present, plausible, and still
// blind to the five files `dossierx skills export` writes into somebody else's
// repository.
func TestSurfaceBehaviourFingerprintCoversEveryPackage(t *testing.T) {
	root := surfaceRepoRoot(t)

	got, err := surfaceBehaviourFingerprint(root)
	if err != nil {
		t.Fatalf("surfaceBehaviourFingerprint: %v", err)
	}
	inputs, err := behaviourPackageFiles(root)
	if err != nil {
		t.Fatalf("behaviourPackageFiles: %v", err)
	}
	// The embedded half is taken from the TOOLCHAIN and not from the emitter's
	// embeddedFiles. Calling the emitter here made this half of the check a
	// restatement of the thing it was checking: narrow the emitter's reading of
	// //go:embed and this test narrowed with it, in one edit, and agreed.
	embeds := surfaceToolchainEmbeds(t, root)

	// The independent derivation: git's own file list, grouped into packages by
	// directory. No walk of the emitter's roots, no reading of its lists.
	packages := map[string][]string{}
	for _, file := range surfaceTrackedFiles(t, root) {
		if !strings.HasSuffix(file, ".go") || strings.HasSuffix(file, "_test.go") {
			continue
		}
		dir := path.Dir(file)
		packages[dir] = append(packages[dir], file)
	}
	if len(packages) == 0 {
		t.Fatalf("git shows no Go packages; this test cross-checks nothing")
	}

	for pkg, sources := range packages {
		if reason, excluded := behaviourExclusions[pkg]; excluded {
			if _, present := got[pkg]; present {
				t.Errorf("package %s is fingerprinted but is also declared excluded (%s); it cannot be both", pkg, reason)
			}
			continue
		}
		if _, ok := got[pkg]; !ok {
			t.Errorf("package %s has no behaviour fingerprint and is not in behaviourExclusions — its code can change without moving surface.json", pkg)
			continue
		}
		for _, source := range sources {
			if !containsStr(inputs[pkg], source) {
				t.Errorf("%s is a source of package %s and is not among the files its fingerprint is taken over", source, pkg)
			}
		}
		// Everything the package embeds ships with it, so it belongs in the same
		// digest. skills/ is the live case: five SKILL.md bundles, installed
		// verbatim into client repositories by `dossierx skills export`.
		for _, f := range embeds[pkg] {
			if !containsStr(inputs[pkg], f) {
				t.Errorf("%s is go:embed-ed into package %s and is NOT hashed into its fingerprint — editing it would leave surface.json byte-identical", f, pkg)
			}
		}
	}
	for pkg := range got {
		if _, ok := packages[pkg]; !ok {
			t.Errorf("behaviour_fingerprint carries %q, which git does not show as a Go package", pkg)
		}
	}
}

// ---------------------------------------------------------------------
// envelope payloads
// ---------------------------------------------------------------------

// TestSurfacePayloadTableCoversEveryDataType keeps surfacePayloadTypes honest.
// Reflection cannot enumerate a package's types, so the table is written by
// hand; this walks cmd/dossierx's own sources for every declared struct named
// *Data and fails if the two disagree in either direction.
//
// Without it, a new payload type would simply be absent from the inventory: no
// staleness failure, no count change, and the one field of surface.json whose
// whole job is to pin the envelope's key SET would be quietly short by one
// shape.
func TestSurfacePayloadTableCoversEveryDataType(t *testing.T) {
	root := surfaceRepoRoot(t)
	dir := filepath.Join(root, "cmd", "dossierx")

	files, err := goSourceFiles(dir)
	if err != nil {
		t.Fatalf("list %s: %v", dir, err)
	}
	declared := map[string]bool{}
	for _, src := range files {
		file, parseErr := parser.ParseFile(token.NewFileSet(), src, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", src, parseErr)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || !strings.HasSuffix(ts.Name.Name, "Data") {
					continue
				}
				if _, isStruct := ts.Type.(*ast.StructType); !isStruct {
					continue
				}
				declared[ts.Name.Name] = true
			}
		}
	}
	if len(declared) == 0 {
		t.Fatalf("no *Data struct types found under %s; the cross-check has nothing to compare", dir)
	}

	table := surfacePayloadTypes()
	for name := range declared {
		if _, ok := table[name]; !ok {
			t.Errorf("%s is an envelope payload type and is missing from surfacePayloadTypes; add it, or surface.json under-reports the machine contract", name)
		}
	}
	for name := range table {
		if strings.Contains(name, ".") {
			continue // a payload from another package, named explicitly
		}
		if !declared[name] {
			t.Errorf("surfacePayloadTypes names %q, which cmd/dossierx no longer declares", name)
		}
	}
}

// ---------------------------------------------------------------------
// skills
// ---------------------------------------------------------------------

// TestSurfaceSkillsMatchTheEmbeddedBundles asserts the emitted skill list is the
// set actually compiled into the binary, not just whatever Order happens to say.
func TestSurfaceSkillsMatchTheEmbeddedBundles(t *testing.T) {
	entries, err := surfaceskills.FS.ReadDir(".")
	if err != nil {
		t.Fatalf("read the embedded skills FS: %v", err)
	}
	var embedded []string
	for _, entry := range entries {
		if entry.IsDir() {
			embedded = append(embedded, entry.Name())
		}
	}
	sort.Strings(embedded)

	got := append([]string(nil), surfaceSkills()...)
	sort.Strings(got)

	if fmt.Sprint(embedded) != fmt.Sprint(got) {
		t.Errorf("surface.json lists skills %v; the binary embeds %v", got, embedded)
	}
}

// ---------------------------------------------------------------------
// lint rules
// ---------------------------------------------------------------------

// TestSurfaceLintRulesAreTheWholeRegistry asserts the emitted rule list has one
// entry per registered lint and no duplicates — the registry is a slice, so two
// files registering the same name would otherwise collapse into one line of the
// inventory and read as a rule having been removed.
func TestSurfaceLintRulesAreTheWholeRegistry(t *testing.T) {
	names := surfaceLintRules()
	if len(names) != len(lint.Registry) {
		t.Errorf("surface.json lists %d lint rules; internal/lint.Registry holds %d", len(names), len(lint.Registry))
	}
	seen := map[string]bool{}
	for _, name := range names {
		if seen[name] {
			t.Errorf("lint rule %q is registered twice; the inventory cannot represent that", name)
		}
		seen[name] = true
	}
}
