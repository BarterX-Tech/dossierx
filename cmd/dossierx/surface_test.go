// surface_test.go emits surface.json: the mechanically derived inventory of
// everything a client of this project can observe. The release gate diffs it
// against the previous release to answer "did this release change something a
// client must know about", and hands it to the agents that read this project's
// prose against its code.
//
// WHY IT IS A TEST AND NOT A COMMAND. The cobra tree is built by unexported
// newRootCmd() in main.go and Go forbids importing package main, so the walk has
// to happen inside this package. The other door — a hidden `dossierx __surface`
// verb — is worse than inconvenient: it would itself be a twentieth leaf, and
// TestSurfaceIsNineteenLeavesUnderSevenNouns excludes commands by the
// annotationRetired MARK and deliberately NOT by hidden-ness (see retired.go),
// so the emitter would break the very count it exists to protect. A generator
// test has neither problem, and it comes with the staleness check for free:
// TestGenerateSurfaceJSON regenerates the document and compares it byte for byte
// against the committed copy, so a change to the surface that is not
// regenerated is a red build rather than a gate that goes green over a change
// nobody documented.
//
// It follows the repo's existing golden convention rather than inventing one:
// the -regenerate-goldens flag is the same flag
// internal/render/markdown/markdown_golden_test.go registers, so
//
//	go test ./cmd/dossierx -run TestGenerateSurfaceJSON -regenerate-goldens
//
// rewrites the committed file the same way regenerating a markdown golden does.
//
// NOTHING IN HERE MAY BE VERSION- OR TIME-DEPENDENT. The document is compared
// byte for byte on three operating systems and two Go versions, so every field
// is derived from tracked content alone — no build info, no timestamps, no
// absolute paths — which is also why it needs no normalization step of the kind
// tests/fixture_staleness_test.go has to apply to the sample viewers.
//
// The meta-tests that keep each extraction honest live next door in
// surface_meta_test.go.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/lint"
	surfaceskills "github.com/BarterX-Tech/dossierx/skills"
)

// regenerateGoldens rewrites the committed surface.json instead of asserting
// against it. The name is deliberately the one internal/render/markdown already
// uses for its own goldens: one convention, two artifacts.
var regenerateGoldens = flag.Bool("regenerate-goldens", false, "rewrite the committed surface.json instead of asserting the committed copy matches")

// surfaceFileName is the committed document's path, relative to the repository
// root. The release gate reads the previous release's copy as
// `git show <prev-tag>:surface.json`, so it lives at the top level.
const surfaceFileName = "surface.json"

// surfaceModulePath is this module's import path. The AST extractions below use
// it to turn an import path back into a directory on disk, and refuse to resolve
// anything outside it.
const surfaceModulePath = "github.com/BarterX-Tech/dossierx"

// ---------------------------------------------------------------------
// the document
// ---------------------------------------------------------------------

// surfaceDoc is surface.json. Field order here is the document's key order;
// every map inside is emitted with sorted keys by encoding/json, and every slice
// is sorted before it is stored, so the bytes are a function of the tree alone.
type surfaceDoc struct {
	Commands []surfaceCommand `json:"commands"`

	// RootFlags is the flags registered on the ROOT itself. It is a field of
	// its own because the commands walk only reports LEAVES, and a root flag is
	// reachable from none of them: --version is a LOCAL root flag (persistent
	// would make "dossierx claim --version" parse and be silently ignored, which
	// is the hole it was added to close), so it is inherited by nothing and
	// would be a client-observable door this inventory never mentions.
	RootFlags []string `json:"root_flags"`

	Retired    []string `json:"retired"`
	LintRules  []string `json:"lint_rules"`
	ErrorCodes []string `json:"error_codes"`
	Skills     []string `json:"skills"`
	HTTPRoutes []string `json:"http_routes"`

	// MarkdownConstructs is the case-name list of testdata/markdown-cases,
	// which is FORMAT.md's real truth-source for what the renderer accepts.
	MarkdownConstructs []string `json:"markdown_constructs"`

	// RenderFingerprint moves whenever anything that decides what a claim
	// RENDERS AS moves: every non-test source under internal/render, at any
	// depth, plus every template those sources embed. A past release changed
	// table layout silently; a name-level inventory cannot see that, and this
	// can.
	RenderFingerprint string `json:"render_fingerprint"`

	// BehaviourFingerprint is what makes "any code change moves surface.json"
	// true. Broadening a lint rule so a corpus that passed now fails moves no
	// name, no count and nothing else in this document — it moves exactly one
	// entry here. It covers the shipped packages' embedded bytes as well as
	// their statements, which is what makes a rewritten SKILL.md a change.
	BehaviourFingerprint map[string]string `json:"behaviour_fingerprint"`

	Envelope    surfaceEnvelope `json:"envelope"`
	VersionPins []surfacePin    `json:"version_pins"`
	Counts      map[string]int  `json:"counts"`
}

// surfaceCommand is one leaf of the CLI surface.
type surfaceCommand struct {
	Path  string   `json:"path"`
	Short string   `json:"short"`
	Flags []string `json:"flags"`
}

// surfaceEnvelope is what this inventory can say about the JSON envelope with
// no guessing. SKILL.md documents the envelope as a machine contract and
// nothing pinned the key SET before this.
//
// It is deliberately NOT keyed by command path. Associating a command with the
// payload it returns cannot be derived here: half the commands hand emit() a
// local variable or a helper's return value, several return two different
// shapes depending on which branch they take, and the ONE way to observe the
// real association — executing every leaf against a fixture — reports the keys
// of whichever branch the fixture happened to take, not the contract, because
// most payload fields are `omitempty`. A per-command map built that way would
// look populated and be wrong, and the gate would diff it and report key
// churn that no code change caused. So the three things that ARE fully
// derivable are emitted, and the association is left out rather than faked.
type surfaceEnvelope struct {
	// Keys is the outer envelope's own key set (internal/cliout.Envelope).
	Keys []string `json:"keys"`
	// ExitCodes is every error code mapped to the process exit status it
	// defaults to. The mapping is per CODE, not per command — cliout.ExitCode
	// is what decides it and it takes nothing else.
	ExitCodes map[string]int `json:"exit_codes"`
	// Payloads is every envelope payload type this package publishes, mapped
	// to the data keys it marshals to.
	Payloads map[string][]string `json:"payloads"`
}

// surfacePin is one release-version pin found by the sweep. The LOCATION is
// data, never a list in this file: the old checklist's hard-coded list of pin
// sites went stale twice, so a pin appearing in a fourth place has to show up
// here on its own.
type surfacePin struct {
	File    string `json:"file"`
	Pin     string `json:"pin"`
	Version string `json:"version"`
}

// ---------------------------------------------------------------------
// the generator + the staleness assertion
// ---------------------------------------------------------------------

// TestGenerateSurfaceJSON is both halves of the contract: with
// -regenerate-goldens it writes surface.json, and without it asserts the
// committed copy is byte-identical to what it would write. That assertion IS
// the staleness test — change the surface without regenerating and CI is red.
func TestGenerateSurfaceJSON(t *testing.T) {
	root := surfaceRepoRoot(t)
	got := surfaceBytes(t, root)
	committed := filepath.Join(root, surfaceFileName)

	if *regenerateGoldens {
		if err := os.WriteFile(committed, got, 0o644); err != nil {
			t.Fatalf("write %s: %v", surfaceFileName, err)
		}
		t.Logf("regenerated %s (%d bytes)", surfaceFileName, len(got))
		return
	}

	want, err := os.ReadFile(committed)
	if err != nil {
		t.Fatalf("read %s: %v\n%s", surfaceFileName, err, surfaceRegenHint)
	}
	if bytes.Equal(want, got) {
		return
	}
	t.Errorf("%s is stale: the committed copy is not what this tree generates.\n%s\n%s",
		surfaceFileName, surfaceLineDiff(string(want), string(got)), surfaceRegenHint)
}

// surfaceRegenHint is the one recovery, spelled as a command rather than as
// advice, because the failure above is the first thing a contributor who has
// never heard of this file will see.
const surfaceRegenHint = "regenerate it with:\n\tgo test ./cmd/dossierx -run TestGenerateSurfaceJSON -regenerate-goldens"

// surfaceBytes builds the document and marshals it exactly the way the
// committed file is written: two-space indent, trailing newline.
func surfaceBytes(t *testing.T, root string) []byte {
	t.Helper()
	doc := buildSurfaceDoc(t, root)
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal surface document: %v", err)
	}
	return append(raw, '\n')
}

// buildSurfaceDoc runs every extraction. Each one FAILS the test rather than
// returning a partial answer: an inventory that quietly drops a surface it
// could not read is the shape of gate this project exists to not have.
func buildSurfaceDoc(t *testing.T, root string) surfaceDoc {
	t.Helper()

	rootCmd := newRootCmd()
	commands, retiredPaths, nouns := surfaceCommandTree(rootCmd)

	codes, err := surfaceErrorCodes(root)
	if err != nil {
		t.Fatalf("extract error codes: %v", err)
	}
	routes, err := surfaceHTTPRoutes(root)
	if err != nil {
		t.Fatalf("extract http routes: %v", err)
	}
	constructs, err := surfaceMarkdownConstructs(root)
	if err != nil {
		t.Fatalf("extract markdown constructs: %v", err)
	}
	renderFiles, err := renderFingerprintFiles(root)
	if err != nil {
		t.Fatalf("resolve render fingerprint inputs: %v", err)
	}
	renderPrint, err := hashRepoFiles(root, renderFiles)
	if err != nil {
		t.Fatalf("hash render fingerprint inputs: %v", err)
	}
	behaviour, err := surfaceBehaviourFingerprint(root)
	if err != nil {
		t.Fatalf("compute behaviour fingerprint: %v", err)
	}
	pins, err := surfaceVersionPins(root)
	if err != nil {
		t.Fatalf("sweep version pins: %v", err)
	}

	// Every count is taken over the FIELD it counts, never over the source that
	// field was extracted from. len(lint.Registry) was the exception and it was
	// a hole: dropping a rule on the way into lint_rules left counts.lint_rules
	// reading 28 beside a list of 27, so the one number the site and README
	// quote agreed with the registry while disagreeing with the document it
	// appears in.
	rules := surfaceLintRules()

	return surfaceDoc{
		Commands:             commands,
		RootFlags:            surfaceCommandFlags(rootCmd),
		Retired:              retiredPaths,
		LintRules:            rules,
		ErrorCodes:           codes,
		Skills:               surfaceSkills(),
		HTTPRoutes:           routes,
		MarkdownConstructs:   constructs,
		RenderFingerprint:    renderPrint,
		BehaviourFingerprint: behaviour,
		Envelope:             surfaceEnvelopeContract(codes),
		VersionPins:          pins,
		Counts: map[string]int{
			"nouns":       nouns,
			"commands":    len(commands),
			"lint_rules":  len(rules),
			"error_codes": len(codes),
			"http_routes": len(routes),
		},
	}
}

// ---------------------------------------------------------------------
// 1 + 2. commands, flags, retired
// ---------------------------------------------------------------------

// surfaceCommandTree walks newRootCmd() the same way
// TestSurfaceIsNineteenLeavesUnderSevenNouns does, and for the same reason: a
// naive walk yields ~31 leaves against an enforced 19, because cobra's own
// help/completion furniture and the twelve removal stubs are all in the tree.
//
// The stubs are excluded by the annotationRetired MARK and never by
// Hidden — retired.go documents that distinction as load-bearing — and they are
// INVENTORIED rather than dropped, because a retired noun reappearing in prose
// is a finding the gate has to be able to make.
func surfaceCommandTree(root *cobra.Command) (leaves []surfaceCommand, retiredPaths []string, nouns int) {
	var walk func(cmd *cobra.Command, prefix string)
	walk = func(cmd *cobra.Command, prefix string) {
		leaf := true
		for _, child := range cmd.Commands() {
			// cobra injects these into every tree; they are framework
			// furniture, not part of the product's surface.
			if child.Name() == "help" || child.Name() == "completion" {
				continue
			}
			name := child.Name()
			if prefix != "" {
				name = prefix + " " + name
			}
			if retired(child) {
				retiredPaths = append(retiredPaths, name)
				continue
			}
			leaf = false
			walk(child, name)
		}
		if leaf && prefix != "" {
			leaves = append(leaves, surfaceCommand{
				Path:  prefix,
				Short: cmd.Short,
				Flags: surfaceCommandFlags(cmd),
			})
		}
	}
	walk(root, "")

	for _, child := range root.Commands() {
		if child.Name() == "help" || child.Name() == "completion" || retired(child) {
			continue
		}
		nouns++
	}

	sort.Slice(leaves, func(i, j int) bool { return leaves[i].Path < leaves[j].Path })
	sort.Strings(retiredPaths)
	return leaves, retiredPaths, nouns
}

// surfaceCommandFlags is every flag a caller may legally pass to cmd: the
// command's own flags plus the persistent ones it inherits (--config and
// --format apply to every leaf and are part of what a client can observe).
//
// It reads the names through reflection instead of importing
// github.com/spf13/pflag, which is deliberate and not clever-for-its-own-sake:
// pflag is an INDIRECT requirement of this module, CI runs `go mod tidy` and
// fails on any diff, and importing it here would promote it to a direct one —
// so a test-only convenience would change go.mod. Reading pflag.Flag.Name off
// the value pflag itself hands the visitor costs one small function and touches
// nothing outside this file. Parsing FlagUsages() was the other option and is
// strictly worse: it re-derives from formatted prose what the struct already
// states.
func surfaceCommandFlags(cmd *cobra.Command) []string {
	seen := map[string]bool{}
	for _, set := range []any{cmd.LocalFlags(), cmd.InheritedFlags()} {
		visitFlagNames(set, func(name string) {
			// cobra adds --help during Execute, not construction, so it is
			// absent here; filtering it anyway keeps the document identical
			// whether or not something in this binary has executed a tree.
			if name != "help" {
				seen[name] = true
			}
		})
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, "--"+name)
	}
	sort.Strings(names)
	return names
}

// visitFlagNames calls visit with the Name of every flag in a *pflag.FlagSet,
// reached through reflection. See surfaceCommandFlags for why it is not a plain
// VisitAll call.
func visitFlagNames(set any, visit func(name string)) {
	method := reflect.ValueOf(set).MethodByName("VisitAll")
	if !method.IsValid() || method.Type().NumIn() != 1 {
		panic("pflag.FlagSet no longer has a one-argument VisitAll; surface_test.go must be updated")
	}
	callback := reflect.MakeFunc(method.Type().In(0), func(args []reflect.Value) []reflect.Value {
		visit(args[0].Elem().FieldByName("Name").String())
		return nil
	})
	method.Call([]reflect.Value{callback})
}

// ---------------------------------------------------------------------
// 3. lint rules
// ---------------------------------------------------------------------

// surfaceLintRules reads internal/lint.Registry, which is exported and
// self-registering via each rule's init().
func surfaceLintRules() []string {
	names := make([]string, 0, len(lint.Registry))
	for _, l := range lint.Registry {
		names = append(names, l.Name())
	}
	sort.Strings(names)
	return names
}

// ---------------------------------------------------------------------
// 4. error codes
// ---------------------------------------------------------------------

// errorCodePackageDir is the package the error-code vocabulary is extracted
// from, relative to the repository root. It is the PACKAGE and deliberately not
// one file: a Code constant is client-observable wherever in package cliout it
// is declared, and a reading pinned to codes.go reports 44 while the binary
// serves 45. That is the same drift the route extraction is written against —
// see TestSurfaceRoutesCoverEveryRegistrationInServe, which counts across the
// whole of package serve for exactly this reason.
const errorCodePackageDir = "internal/cliout"

// errorCodeHomeFile is the file the vocabulary lives in today. Its only
// privilege is STRICTNESS: everything declared in it must be a Code, so a
// constant added there in a shape the parse does not recognise is an error
// rather than a silent omission. Codes declared in any other file of the
// package are collected just the same; they are simply not the only thing
// those files may contain.
const errorCodeHomeFile = "codes.go"

// surfaceErrorCodes parses package cliout and returns every Code constant's
// VALUE — the snake_case token a skill branches on.
//
// It recognises the three shapes a Code can be written in — an explicit `Code`
// type, a `Code("...")` conversion, and a Code-prefixed name whose type is
// inferred — and it ERRORS on a constant matching any of those that it cannot
// reduce to a string, rather than filtering for the shape it expects. Inside
// errorCodeHomeFile it errors on anything it cannot classify at all. A filter
// would silently skip a constant declared in an unexpected form, which is
// precisely the drift that leaves a live code out of the inventory.
func surfaceErrorCodes(root string) ([]string, error) {
	dir := filepath.Join(root, filepath.FromSlash(errorCodePackageDir))
	sources, err := goSourceFiles(dir)
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, fmt.Errorf("%s holds no non-test Go sources", errorCodePackageDir)
	}

	var codes []string
	seen := map[string]string{}
	for _, src := range sources {
		home := filepath.Base(src) == errorCodeHomeFile
		file, parseErr := parser.ParseFile(token.NewFileSet(), src, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return nil, fmt.Errorf("parse %s: %w", src, parseErr)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					return nil, fmt.Errorf("%s: unexpected const spec %T", src, spec)
				}
				if !declaresErrorCode(value) {
					if home {
						return nil, fmt.Errorf("%s: const %v is not declared as a Code; every constant in this file must be one or the inventory is incomplete", src, value.Names)
					}
					continue // an ordinary constant of package cliout
				}
				text, codeErr := errorCodeValue(value)
				if codeErr != nil {
					return nil, fmt.Errorf("%s: %w", src, codeErr)
				}
				if prev, dup := seen[text]; dup {
					return nil, fmt.Errorf("%s: %s and %s both declare the code %q", src, prev, value.Names[0].Name, text)
				}
				seen[text] = value.Names[0].Name
				codes = append(codes, text)
			}
		}
	}
	if len(codes) == 0 {
		return nil, fmt.Errorf("%s: no Code constants found", errorCodePackageDir)
	}
	sort.Strings(codes)
	return codes, nil
}

// declaresErrorCode reports whether a const spec declares a cliout.Code, in any
// of the three forms one can be written in. It errs towards YES: a constant it
// wrongly claims is a Code fails loudly in errorCodeValue, whereas one it
// wrongly passes over disappears from the inventory in silence.
func declaresErrorCode(value *ast.ValueSpec) bool {
	if ident, ok := value.Type.(*ast.Ident); ok && ident.Name == "Code" {
		return true
	}
	for _, name := range value.Names {
		if strings.HasPrefix(name.Name, "Code") {
			return true
		}
	}
	if len(value.Values) == 1 {
		if call, ok := value.Values[0].(*ast.CallExpr); ok {
			if ident, isIdent := call.Fun.(*ast.Ident); isIdent && ident.Name == "Code" {
				return true
			}
		}
	}
	return false
}

// errorCodeValue reduces a Code const spec to the token it declares, unwrapping
// a `Code("...")` conversion on the way.
func errorCodeValue(value *ast.ValueSpec) (string, error) {
	if len(value.Names) != 1 || len(value.Values) != 1 {
		return "", fmt.Errorf("const %v does not declare exactly one name and one value", value.Names)
	}
	name := value.Names[0].Name
	expr := value.Values[0]
	if call, ok := expr.(*ast.CallExpr); ok {
		ident, isIdent := call.Fun.(*ast.Ident)
		if !isIdent || ident.Name != "Code" || len(call.Args) != 1 {
			return "", fmt.Errorf("const %s is a call this parse cannot reduce to a code", name)
		}
		expr = call.Args[0]
	}
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", fmt.Errorf("const %s is not a string literal", name)
	}
	text, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", fmt.Errorf("const %s: %w", name, err)
	}
	return text, nil
}

// ---------------------------------------------------------------------
// 5. skills
// ---------------------------------------------------------------------

// surfaceSkills reads skills/embed.go, which exports both the embedded FS and
// the reading Order. Order is what is emitted: it is the sequence the derived
// forms present the bundles in, and skills_embed.go already fails loudly when it
// disagrees with the FS.
func surfaceSkills() []string {
	out := make([]string, len(surfaceskills.Order))
	copy(out, surfaceskills.Order)
	return out
}

// ---------------------------------------------------------------------
// 6. http routes
// ---------------------------------------------------------------------

// surfaceHTTPRoutes parses internal/serve/server.go's routes() body and returns
// every registered pattern.
//
// "Walking the mux" is not implementable: http.ServeMux exposes no enumeration
// API and routes() is unexported, so a static read of the registrations is the
// only door. The parse FOLDS constants across packages — one pattern is
// assetRoutePattern, composed in claim_assets.go from
// components.AssetRoutePrefix over in images.go — and it FAILS on an argument it
// cannot resolve rather than skipping it. Skipping would mean a route silently
// missing from the inventory, which is the exact failure this field exists to
// prevent; the meta-test next door cross-checks the count against the number of
// registration calls in the package.
func surfaceHTTPRoutes(root string) ([]string, error) {
	dir := filepath.Join(root, "internal", "serve")
	src := filepath.Join(dir, "server.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, src, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", src, err)
	}

	var body *ast.BlockStmt
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == "routes" && fn.Body != nil {
			body = fn.Body
			break
		}
	}
	if body == nil {
		return nil, fmt.Errorf("%s: no routes() function; the route inventory has no source", src)
	}

	resolver := &surfaceConsts{root: root, cache: map[string]map[string]surfaceConst{}}
	var routes []string
	var failure error
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		// http.ServeMux registers through EXACTLY TWO methods and both take
		// the pattern as their first argument. Keying on HandleFunc alone —
		// which this did — let a mux.Handle route be served by the binary while
		// staying invisible to the inventory, with the meta-test next door
		// agreeing because it keyed on the same one name. The two names are
		// spelled out in both places on purpose: a shared predicate would be
		// one edit away from re-opening the same blind spot in both halves.
		//
		// A call this matches that is NOT a mux registration (some other
		// receiver with a Handle method) is not a false pass: its first
		// argument will not fold to a string constant and the resolve below
		// fails loudly, which is the direction this file errs in everywhere.
		if !ok || (sel.Sel.Name != "HandleFunc" && sel.Sel.Name != "Handle") {
			return true
		}
		if len(call.Args) == 0 {
			failure = fmt.Errorf("%s: %s call with no pattern argument", fset.Position(call.Pos()), sel.Sel.Name)
			return false
		}
		pattern, err := resolver.resolve(dir, file, call.Args[0])
		if err != nil {
			failure = fmt.Errorf("%s: cannot resolve the route pattern: %w", fset.Position(call.Args[0].Pos()), err)
			return false
		}
		routes = append(routes, pattern)
		return true
	})
	if failure != nil {
		return nil, failure
	}
	if len(routes) == 0 {
		return nil, fmt.Errorf("%s: routes() registered nothing", src)
	}
	sort.Strings(routes)
	return routes, nil
}

// surfaceConst is one file-level constant and the file it was declared in (which
// is what carries the imports needed to resolve a qualified reference in its
// own value).
type surfaceConst struct {
	value ast.Expr
	file  *ast.File
	dir   string
}

// surfaceConsts resolves compile-time string constants across this module's
// packages. It is the smallest thing that can fold
// "GET " + components.AssetRoutePrefix + "{rest...}" into one pattern without
// pulling a type checker into the module's dependency graph.
type surfaceConsts struct {
	root  string
	cache map[string]map[string]surfaceConst
}

// resolve evaluates expr to a string, or returns an error naming what it could
// not fold. There is no "give up quietly" branch on purpose.
func (c *surfaceConsts) resolve(dir string, file *ast.File, expr ast.Expr) (string, error) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind != token.STRING {
			return "", fmt.Errorf("literal %s is not a string", e.Value)
		}
		return strconv.Unquote(e.Value)

	case *ast.ParenExpr:
		return c.resolve(dir, file, e.X)

	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			return "", fmt.Errorf("unsupported operator %s in a string constant", e.Op)
		}
		left, err := c.resolve(dir, file, e.X)
		if err != nil {
			return "", err
		}
		right, err := c.resolve(dir, file, e.Y)
		if err != nil {
			return "", err
		}
		return left + right, nil

	case *ast.Ident:
		def, err := c.lookup(dir, e.Name)
		if err != nil {
			return "", err
		}
		return c.resolve(def.dir, def.file, def.value)

	case *ast.SelectorExpr:
		pkg, ok := e.X.(*ast.Ident)
		if !ok {
			return "", fmt.Errorf("qualified reference %s is not a package selector", e.Sel.Name)
		}
		importDir, err := c.importDir(file, pkg.Name)
		if err != nil {
			return "", err
		}
		def, err := c.lookup(importDir, e.Sel.Name)
		if err != nil {
			return "", err
		}
		return c.resolve(def.dir, def.file, def.value)

	default:
		return "", fmt.Errorf("expression of type %T is not a foldable string constant", expr)
	}
}

// importDir maps a package name as used in file back to the directory that
// declares it. Only imports inside this module can be resolved; anything else is
// an error rather than a skip.
func (c *surfaceConsts) importDir(file *ast.File, name string) (string, error) {
	for _, imp := range file.Imports {
		imported, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return "", err
		}
		local := imported[strings.LastIndex(imported, "/")+1:]
		if imp.Name != nil {
			local = imp.Name.Name
		}
		if local != name {
			continue
		}
		if !strings.HasPrefix(imported, surfaceModulePath+"/") {
			return "", fmt.Errorf("import %q is outside this module, so its constants cannot be folded", imported)
		}
		return filepath.Join(c.root, filepath.FromSlash(strings.TrimPrefix(imported, surfaceModulePath+"/"))), nil
	}
	return "", fmt.Errorf("no import in %s provides package %q", file.Name.Name, name)
}

// lookup finds a file-level constant by name in the package rooted at dir.
func (c *surfaceConsts) lookup(dir, name string) (surfaceConst, error) {
	consts, ok := c.cache[dir]
	if !ok {
		var err error
		consts, err = c.parsePackageConsts(dir)
		if err != nil {
			return surfaceConst{}, err
		}
		c.cache[dir] = consts
	}
	def, ok := consts[name]
	if !ok {
		return surfaceConst{}, fmt.Errorf("no constant named %q in %s", name, dir)
	}
	return def, nil
}

// parsePackageConsts indexes every single-name, single-value const in a
// package's non-test sources.
func (c *surfaceConsts) parsePackageConsts(dir string) (map[string]surfaceConst, error) {
	files, err := goSourceFiles(dir)
	if err != nil {
		return nil, err
	}
	out := map[string]surfaceConst{}
	for _, src := range files {
		file, err := parser.ParseFile(token.NewFileSet(), src, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", src, err)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok || len(value.Names) != 1 || len(value.Values) != 1 {
					continue
				}
				out[value.Names[0].Name] = surfaceConst{value: value.Values[0], file: file, dir: dir}
			}
		}
	}
	return out, nil
}

// ---------------------------------------------------------------------
// 7. markdown constructs
// ---------------------------------------------------------------------

// surfaceMarkdownConstructs is the case-name list of testdata/markdown-cases:
// the corpus that actually decides what the claim-body renderer accepts, which
// is what FORMAT.md is a claim about.
func surfaceMarkdownConstructs(root string) ([]string, error) {
	dir := filepath.Join(root, "testdata", "markdown-cases")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		names = append(names, strings.TrimSuffix(entry.Name(), ".yaml"))
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("%s holds no cases", dir)
	}
	sort.Strings(names)
	return names, nil
}

// ---------------------------------------------------------------------
// 8. render fingerprint
// ---------------------------------------------------------------------

// renderRootDir is the one directory render_fingerprint is derived from,
// relative to the repository root.
//
// It is a ROOT, and deliberately not a list. The previous shape of this field
// named the three render packages in one slice and, in another, the two files
// carrying embed directives; deleting a single line from either one silently
// dropped a whole renderer — the markdown package, or every claim partial — out
// of the fingerprint. There is no line here to delete: everything under this
// directory is in, the embed directives are found by reading the sources rather
// than by listing the files that carry them, and surface_meta_test.go re-derives
// the same set from `git ls-files` to say so.
const renderRootDir = "internal/render"

// renderFingerprintFiles is the input set of render_fingerprint: every non-test
// Go source under internal/render at ANY DEPTH, plus every file those sources
// embed — the viewer shell, CSS and graph client, and the per-layout claim
// partials.
//
// The Go half is a WALK, never a file list. Naming markdown.go alone would miss
// markdown_tables.go, which carries the whole GFM table grammar — and a past
// release's silent change WAS table layout.
func renderFingerprintFiles(root string) ([]string, error) {
	dir := filepath.Join(root, filepath.FromSlash(renderRootDir))
	sources, err := goSourceFilesUnder(root, dir)
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, fmt.Errorf("%s holds no non-test Go sources", renderRootDir)
	}

	seen := map[string]bool{}
	embedded := 0
	for _, rel := range sources {
		seen[rel] = true
		templates, err := embeddedFiles(root, rel)
		if err != nil {
			return nil, err
		}
		for _, f := range templates {
			seen[f] = true
			embedded++
		}
	}
	// A fingerprint over the sources alone would miss a shell.html or a
	// card.html edit entirely, which is half of what this field is for. Zero
	// embedded files means the directives stopped being found, not that the
	// viewer stopped having templates.
	if embedded == 0 {
		return nil, fmt.Errorf("no //go:embed directive was found under %s; the template half of the render fingerprint is empty", renderRootDir)
	}

	out := make([]string, 0, len(seen))
	for f := range seen {
		out = append(out, f)
	}
	sort.Strings(out)
	return out, nil
}

// embeddedFiles expands every //go:embed pattern in one source file into the
// repo-relative paths it actually embeds. A source carrying no directive embeds
// nothing and yields an empty list — the callers scan every file of a package
// and most of them embed nothing — but a pattern that matches NOTHING is an
// error: go:embed itself fails the build in that case, and an extractor that
// shrugged would report a smaller template set than the binary ships.
//
// BOTH doc positions are read, and that is not defensiveness. A go:embed
// directive attaches to whatever declaration node it immediately precedes, and
// which node that is depends on how the var was written:
//
//	//go:embed dossierx          →  the GenDecl's Doc
//	var FS embed.FS
//
//	var (
//		//go:embed dossierx      →  the ValueSpec's Doc; the GenDecl has none
//		FS embed.FS
//	)
//
// The two forms embed identical bytes, so gofmt, `go build` and `go vet` all
// stay silent across the rewrite. Reading only the GenDecl — which this did —
// meant that one legal refactor of skills/embed.go dropped all five shipped
// SKILL.md bundles out of behaviour_fingerprint with nothing to say so.
func embeddedFiles(root, relSource string) ([]string, error) {
	src := filepath.Join(root, filepath.FromSlash(relSource))
	file, err := parser.ParseFile(token.NewFileSet(), src, nil, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", src, err)
	}
	dir := filepath.Dir(src)

	var patterns []string
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		patterns = append(patterns, embedPatternsIn(gen.Doc)...)
		for _, spec := range gen.Specs {
			if value, isValue := spec.(*ast.ValueSpec); isValue {
				patterns = append(patterns, embedPatternsIn(value.Doc)...)
			}
		}
	}
	if len(patterns) == 0 {
		return nil, nil
	}

	var out []string
	for _, pattern := range patterns {
		matched, err := expandEmbedPattern(root, dir, pattern)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", relSource, err)
		}
		out = append(out, matched...)
	}
	return out, nil
}

// embedPatternsIn is the go:embed patterns carried by one doc comment group,
// or nothing when the group is absent or says nothing about embedding.
func embedPatternsIn(doc *ast.CommentGroup) []string {
	if doc == nil {
		return nil
	}
	var out []string
	for _, comment := range doc.List {
		text, found := strings.CutPrefix(comment.Text, "//go:embed ")
		if !found {
			continue
		}
		out = append(out, strings.Fields(text)...)
	}
	return out
}

// expandEmbedPattern resolves one go:embed pattern to repo-relative file paths.
// A directory embeds its whole tree (minus the dot- and underscore-prefixed
// names go:embed itself skips); anything else goes through filepath.Glob, which
// covers both a plain path and a wildcard.
func expandEmbedPattern(root, dir, pattern string) ([]string, error) {
	target := filepath.Join(dir, filepath.FromSlash(pattern))
	if info, err := os.Stat(target); err == nil && info.IsDir() {
		var out []string
		walkErr := filepath.WalkDir(target, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			name := filepath.Base(path)
			if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if entry.IsDir() {
				return nil
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			out = append(out, filepath.ToSlash(rel))
			return nil
		})
		if walkErr != nil {
			return nil, walkErr
		}
		return out, nil
	}

	matches, err := filepath.Glob(target)
	if err != nil {
		return nil, fmt.Errorf("pattern %q: %w", pattern, err)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("pattern %q matches nothing", pattern)
	}
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		rel, relErr := filepath.Rel(root, match)
		if relErr != nil {
			return nil, relErr
		}
		out = append(out, filepath.ToSlash(rel))
	}
	return out, nil
}

// ---------------------------------------------------------------------
// 9. behaviour fingerprint
// ---------------------------------------------------------------------

// behaviourRoots are the trees holding the packages that are COMPILED INTO the
// shipped binary, relative to the repository root.
//
// skills/ is one of them and was missing. It is a real Go package — one source
// and a go:embed of the five SKILL.md bundles — and `dossierx skills export`
// installs those bundles into OTHER people's repositories, which surfaces.yaml
// calls unfixable after the tag. While it was outside this walk, a bundle could
// be rewritten, or the reading Order re-sequenced, with surface.json coming out
// byte-identical: the field whose whole job is to make every code change move
// this document had a shipped package outside it.
//
// scripts/normalize-claims is the one Go package deliberately NOT here — a
// repository maintenance tool, in nobody's binary — and the meta-test next door
// names it as an exclusion rather than letting it be absent quietly.
var behaviourRoots = []string{"cmd", "internal", "skills"}

// surfaceBehaviourFingerprint hashes every shipped package's own inputs, keyed
// by package path.
//
// Without it the inventory is name-level only. Broadening a lint rule so a
// corpus that passed check now fails moves no name, no count and nothing else in
// this document — this is the field that makes "any code change moves
// surface.json" true, and therefore the field that turns a name-only delta into
// a SILENT-BEHAVIOUR classification the gate can act on.
func surfaceBehaviourFingerprint(root string) (map[string]string, error) {
	packages, err := behaviourPackageFiles(root)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(packages))
	for pkg, files := range packages {
		sum, hashErr := hashRepoFiles(root, files)
		if hashErr != nil {
			return nil, hashErr
		}
		out[pkg] = sum
	}
	return out, nil
}

// behaviourPackageFiles maps each shipped package to the repo-relative files its
// fingerprint is taken over: the package's own non-test Go sources PLUS
// everything those sources go:embed.
//
// The embedded half is not a flourish. A package's embedded bytes are shipped
// as surely as its statements are — skills/embed.go's SKILL.md bundles are
// installed into a client's repository verbatim, and internal/render's
// templates are what every viewer is built from — so a fingerprint over the .go
// files alone would call a rewritten skill bundle "no change".
//
// It is exported to the meta-test as a map rather than folded into the digest
// so the cross-check can assert over the INPUTS. A meta-test that can only see
// the digest can say the two disagree but never which file went missing.
func behaviourPackageFiles(root string) (map[string][]string, error) {
	out := map[string][]string{}
	for _, top := range behaviourRoots {
		walkErr := filepath.WalkDir(filepath.Join(root, top), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() {
				return nil
			}
			sources, listErr := goSourceFiles(path)
			if listErr != nil {
				return listErr
			}
			if len(sources) == 0 {
				return nil // a directory that holds no Go source is not a package
			}
			pkg, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}

			seen := map[string]bool{}
			for _, source := range sources {
				rel, err := filepath.Rel(root, source)
				if err != nil {
					return err
				}
				slashed := filepath.ToSlash(rel)
				seen[slashed] = true
				embedded, embedErr := embeddedFiles(root, slashed)
				if embedErr != nil {
					return embedErr
				}
				for _, f := range embedded {
					seen[f] = true
				}
			}
			files := make([]string, 0, len(seen))
			for f := range seen {
				files = append(files, f)
			}
			sort.Strings(files)
			out[filepath.ToSlash(pkg)] = files
			return nil
		})
		if walkErr != nil {
			return nil, walkErr
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no packages found under %v", behaviourRoots)
	}
	return out, nil
}

// ---------------------------------------------------------------------
// 10. envelope
// ---------------------------------------------------------------------

// surfaceEnvelopeContract is everything the envelope's machine contract can be
// stated as without guessing: the outer key set, the code-to-exit-status table,
// and the data keys of every payload type this package publishes. See
// surfaceEnvelope for why it is not keyed by command path.
func surfaceEnvelopeContract(codes []string) surfaceEnvelope {
	exits := make(map[string]int, len(codes))
	for _, code := range codes {
		exits[code] = cliout.ExitCode(cliout.Code(code))
	}
	payloads := map[string][]string{}
	for name, sample := range surfacePayloadTypes() {
		payloads[name] = jsonFieldNames(reflect.TypeOf(sample))
	}
	return surfaceEnvelope{
		Keys:      jsonFieldNames(reflect.TypeOf(cliout.Envelope{})),
		ExitCodes: exits,
		Payloads:  payloads,
	}
}

// surfacePayloadTypes is every envelope payload this package publishes. The list
// is explicit — reflection cannot enumerate a package's types — and
// TestSurfacePayloadTableCoversEveryDataType fails the moment a *Data type is
// declared and not added here, so it cannot go stale in silence.
func surfacePayloadTypes() map[string]any {
	return map[string]any{
		"buildOrderLockData":    buildOrderLockData{},
		"buildOrderPhaseData":   buildOrderPhaseData{},
		"buildOrderProposeData": buildOrderProposeData{},
		"buildOrderStatusData":  buildOrderStatusData{},
		"checkData":             checkData{},
		"claimEdgesData":        claimEdgesData{},
		"claimLinkData":         claimLinkData{},
		"claimListData":         claimListData{},
		"claimNewData":          claimNewData{},
		"claimShowData":         claimShowData{},
		"cliout.DryRun":         cliout.DryRun{},
		"commentInboxData":      commentInboxData{},
		"commentListData":       commentListData{},
		"commentWriteData":      commentWriteData{},
		"flagData":              flagData{},
		"lintFindingData":       lintFindingData{},
		"lockData":              lockData{},
		"lockRefusedData":       lockRefusedData{},
		"reauditData":           reauditData{},
		"scanErrorData":         scanErrorData{},
		"skillsExportData":      skillsExportData{},
		"unlockData":            unlockData{},
		"versionData":           versionData{},
	}
}

// jsonFieldNames is the marshalled key set of a struct type: the json tag name
// of every exported field, or the Go field name where a tag is absent (which
// TestEnvelopePayloadTypesDeclareSnakeCaseJSONTags separately forbids, so it
// shows up here as the eyesore it is rather than being hidden).
func jsonFieldNames(rt reflect.Type) []string {
	var names []string
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name := field.Name
		if tag, ok := field.Tag.Lookup("json"); ok {
			tagName := strings.Split(tag, ",")[0]
			if tagName == "-" {
				continue
			}
			if tagName != "" {
				name = tagName
			}
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ---------------------------------------------------------------------
// 11. version pins
// ---------------------------------------------------------------------

// surfacePinSweep is docs/RELEASING.md's own sweep expression, verbatim. It is
// run through `git grep` and NEVER a plain `grep -r`: grep resolves to ugrep on
// some machines, whose -r skips dot-directories, so .github/ is never searched
// and a pin in a workflow file is invisible.
const surfacePinSweep = `dossierx(/cmd/dossierx)?@v|githubusercontent\.com/[^ ]*dossierx/v`

// surfacePinRE extracts the pin token out of a swept line. Every hit MUST yield
// one: a line the sweep found and this cannot parse is an error, not a line to
// drop, for the same reason an unresolvable route pattern is.
var surfacePinRE = regexp.MustCompile(`(?:dossierx(?:/cmd/dossierx)?@|githubusercontent\.com/[^ ]*?dossierx/)v\d+\.\d+\.\d+`)

// surfacePinVersionRE pulls the version out of a pin token.
var surfacePinVersionRE = regexp.MustCompile(`v\d+\.\d+\.\d+$`)

// surfaceVersionPins sweeps the tree for release-version pins and reports where
// they are. The LOCATIONS are data, derived from the search — the old
// checklist's hard-coded list of pin sites went stale twice, so a pin appearing
// in a fourth place has to show up here on its own.
//
// Four paths are excluded and each for its own reason. CHANGELOG.md and
// docs/RELEASING.md are RELEASING.md's own exclusions (both are full of
// historical version strings that are correct precisely because they are old).
// surface.json is excluded because this field WRITES pin tokens into it: without
// the exclusion the next sweep would find its own output and the document would
// never converge. surface.baseline.json is excluded for CHANGELOG.md's reason
// exactly: it is a frozen copy of THIS document as of v0.5.0, so it carries that
// release's four pin tokens forever and they are correct precisely because they
// are old. The surface.json exclusion does not reach it — that one keeps the
// field from finding its own output, and this is a copy of that output under
// another name — so without this line the live pin inventory grows entries
// pointing at a historical record, and the release checklist then tells a
// maintainer to move them. See gate_baseline_test.go for what moving them costs.
//
// That path is spelled out here rather than taken from gate_baseline_test.go's
// gateBaselineBootstrapFile, and the reason is a property of THIS FILE that is
// easy to break by accident: surface_test.go has to compile on its own inside an
// OLD tree, because manufacturing a baseline for a release that predates the
// emitter means copying this file (and surface_meta_test.go) into a detached
// worktree of that release and running it there. A reference to an identifier
// declared in any other file of this package makes that copy fail to build, and
// the failure appears years later in the hands of whoever is trying to
// reconstruct a baseline. The two spellings cannot drift in silence:
// TestGateBaselineIsExcludedFromTheVersionPinSweep writes a file named by the
// constant into a fixture repository and requires this sweep not to report it.
//
// The exclusions are spelled out here rather than read from surfaces.yaml, and
// that is deliberate: "not reviewed as prose" and "carries no release pin" are
// different questions with different answers. .github/ is out of scope for the
// gate's prose agents and is exactly where the old checklist's ugrep-based sweep
// went blind, so it must stay IN this search.
func surfaceVersionPins(root string) ([]surfacePin, error) {
	cmd := exec.Command("git", "grep", "-nE", surfacePinSweep, "--",
		".", ":!"+surfaceFileName, ":!CHANGELOG.md", ":!docs/RELEASING.md", ":!surface.baseline.json")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		// git grep exits 1 for "no matches". A release that pins nothing
		// anywhere is not a state this project can be in — README and the CI
		// template both carry one — so it is reported rather than accepted as
		// an empty sweep.
		return nil, fmt.Errorf("git grep found no version pins (or could not run): %w", err)
	}

	var pins []surfacePin
	seen := map[surfacePin]bool{}
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		// "<file>:<line>:<text>". The line NUMBER is dropped on purpose: it
		// churns whenever an unrelated paragraph is added above the pin, and a
		// document diffed release-to-release should move only when the pin does.
		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("unparseable git grep line %q", line)
		}
		matches := surfacePinRE.FindAllString(parts[2], -1)
		if len(matches) == 0 {
			return nil, fmt.Errorf("%s: the sweep matched %q but no pin could be extracted from it; the extractor and the sweep disagree", parts[0], parts[2])
		}
		for _, match := range matches {
			version := surfacePinVersionRE.FindString(match)
			if version == "" {
				return nil, fmt.Errorf("%s: pin %q carries no version", parts[0], match)
			}
			pin := surfacePin{File: filepath.ToSlash(parts[0]), Pin: match, Version: version}
			if seen[pin] {
				continue
			}
			seen[pin] = true
			pins = append(pins, pin)
		}
	}
	if len(pins) == 0 {
		return nil, fmt.Errorf("the version-pin sweep produced no pins")
	}
	sort.Slice(pins, func(i, j int) bool {
		if pins[i].File != pins[j].File {
			return pins[i].File < pins[j].File
		}
		return pins[i].Pin < pins[j].Pin
	})
	return pins, nil
}

// ---------------------------------------------------------------------
// shared helpers
// ---------------------------------------------------------------------

// surfaceRepoRoot is the repository root, resolved from this package's own
// directory the same way main_test.go reaches site/index.html.
func surfaceRepoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	return root
}

// goSourceFiles lists a directory's non-test Go sources, absolute and sorted.
// It does NOT descend: a subdirectory is a different package.
func goSourceFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	var out []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		out = append(out, filepath.Join(dir, name))
	}
	sort.Strings(out)
	return out, nil
}

// goSourceFilesUnder lists the non-test Go sources of a whole SUBTREE, as
// repo-relative slash paths, sorted. It is what the render fingerprint walks
// with: goSourceFiles stops at one directory because a subdirectory is a
// different package, and the render surface is a tree of packages rather than
// one of them.
func goSourceFilesUnder(root, dir string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() {
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", dir, err)
	}
	sort.Strings(out)
	return out, nil
}

// hashRepoFiles is the one hash function this document uses: sha256 over each
// file's repo-relative path, byte length and contents, in sorted path order. The
// path and the length are in the stream so that renaming a file, or moving
// bytes between two files, changes the digest.
func hashRepoFiles(root string, rels []string) (string, error) {
	sorted := append([]string(nil), rels...)
	sort.Strings(sorted)

	h := sha256.New()
	for _, rel := range sorted {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return "", fmt.Errorf("hash %s: %w", rel, err)
		}
		fmt.Fprintf(h, "%s\x00%d\x00%s", rel, len(data), data)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// surfaceLineDiff renders the first handful of differing lines, so a failure
// says WHAT moved instead of dumping two copies of a large document.
func surfaceLineDiff(want, got string) string {
	wantLines := strings.Split(want, "\n")
	gotLines := strings.Split(got, "\n")
	var b strings.Builder
	shown := 0
	for i := 0; i < len(wantLines) || i < len(gotLines); i++ {
		var w, g string
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if w == g {
			continue
		}
		if shown == 10 {
			fmt.Fprintf(&b, "  ... and more\n")
			break
		}
		fmt.Fprintf(&b, "  line %d:\n    committed: %s\n    generated: %s\n", i+1, w, g)
		shown++
	}
	return b.String()
}
