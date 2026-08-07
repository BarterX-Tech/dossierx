// Unit tests for cmd/dossierx's own wiring — as opposed to tests/cli_test.go,
// which execs the built binary as a subprocess (and so never counts toward
// this package's own "go test ./... -cover" number), these tests call the
// package's unexported helpers directly: config discovery (resolveConfigPath),
// the small pure helpers (containsStr, pickChangedDependency, claimTitle, the
// --match scorer), lint-finding reporting (reportLintFindings), the
// store/catalog/render path helpers, and the shape of the command tree itself.
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/lint"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// ---------------------------------------------------------------------
// resolveConfigPath
// ---------------------------------------------------------------------

func TestResolveConfigPathExplicitFlagWins(t *testing.T) {
	old := configPath
	defer func() { configPath = old }()

	configPath = "/some/explicit/path.yaml"
	got, err := resolveConfigPath()
	if err != nil {
		t.Fatalf("resolveConfigPath: %v", err)
	}
	if got != configPath {
		t.Fatalf("expected explicit --config value %q, got %q", configPath, got)
	}
}

func TestResolveConfigPathWalksUpward(t *testing.T) {
	old := configPath
	configPath = ""
	defer func() { configPath = old }()

	root := t.TempDir()
	cfgFile := filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgFile, []byte("schema_version: 1\n"), 0o644); err != nil {
		t.Fatalf("write fixture config: %v", err)
	}

	deep := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("mkdir deep: %v", err)
	}

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		if err := os.Chdir(origWD); err != nil {
			t.Logf("restore cwd: %v", err)
		}
	}()
	if err := os.Chdir(deep); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	got, err := resolveConfigPath()
	if err != nil {
		t.Fatalf("resolveConfigPath: %v", err)
	}
	// Resolve symlinks on both sides: on macOS, t.TempDir() lives under
	// /var, which is itself a symlink to /private/var, so a naive
	// filepath.Abs comparison spuriously fails even when resolveConfigPath
	// found the right file.
	gotAbs, err := filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatalf("EvalSymlinks(got): %v", err)
	}
	wantAbs, err := filepath.EvalSymlinks(cfgFile)
	if err != nil {
		t.Fatalf("EvalSymlinks(want): %v", err)
	}
	if gotAbs != wantAbs {
		t.Fatalf("expected walk-upward to find %q, got %q", wantAbs, gotAbs)
	}
}

func TestResolveConfigPathNotFound(t *testing.T) {
	old := configPath
	configPath = ""
	defer func() { configPath = old }()

	isolated := t.TempDir()
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		if err := os.Chdir(origWD); err != nil {
			t.Logf("restore cwd: %v", err)
		}
	}()
	if err := os.Chdir(isolated); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	_, err = resolveConfigPath()
	if err == nil {
		t.Fatalf("expected an error when no project.config.yaml exists anywhere upward")
	}
	if !strings.Contains(err.Error(), "--config") {
		t.Fatalf("expected error to suggest --config, got: %v", err)
	}
}

// ---------------------------------------------------------------------
// containsStr
// ---------------------------------------------------------------------

func TestContainsStr(t *testing.T) {
	ss := []string{"a", "b", "c"}
	if !containsStr(ss, "b") {
		t.Fatalf("expected containsStr to find %q in %v", "b", ss)
	}
	if containsStr(ss, "z") {
		t.Fatalf("expected containsStr to not find %q in %v", "z", ss)
	}
	if containsStr(nil, "a") {
		t.Fatalf("expected containsStr(nil, ...) to be false")
	}
}

// ---------------------------------------------------------------------
// pickChangedDependency
// ---------------------------------------------------------------------

func TestPickChangedDependencyPrefersStaleHash(t *testing.T) {
	fresh := model.Claim{ID: "m.contract.fresh", Body: "fresh"}
	stale := model.Claim{ID: "m.contract.stale", Body: "stale, changed since lock"}
	claim := model.Claim{ID: "m.contract.main", RestsOn: []string{"m.contract.fresh", "m.contract.stale"}}
	claims := []model.Claim{claim, fresh, stale}

	store := &lock.Store{Hashes: map[string]map[string]string{
		claim.ID: {
			"m.contract.fresh": lock.ContentHash(fresh),
			"m.contract.stale": "stale-hash-that-no-longer-matches",
		},
	}}

	got := pickChangedDependency(claim, claims, store)
	if got.ID != stale.ID {
		t.Fatalf("expected the claim with the mismatched stored hash (%q), got %q", stale.ID, got.ID)
	}
}

func TestPickChangedDependencyFallsBackToFirstDep(t *testing.T) {
	dep := model.Claim{ID: "m.contract.dep"}
	claim := model.Claim{ID: "m.contract.main", Mirrors: []string{"m.contract.dep"}}
	claims := []model.Claim{claim, dep}

	store := &lock.Store{Hashes: map[string]map[string]string{}}

	got := pickChangedDependency(claim, claims, store)
	if got.ID != dep.ID {
		t.Fatalf("expected fallback to first declared dependency %q, got %q", dep.ID, got.ID)
	}
}

func TestPickChangedDependencyNoDepsReturnsZeroValue(t *testing.T) {
	claim := model.Claim{ID: "m.contract.lonely"}
	store := &lock.Store{Hashes: map[string]map[string]string{}}

	got := pickChangedDependency(claim, []model.Claim{claim}, store)
	if got.ID != "" {
		t.Fatalf("expected zero-value Claim for a claim with no dependencies, got %+v", got)
	}
}

// ---------------------------------------------------------------------
// reportLintFindings
// ---------------------------------------------------------------------

func TestReportLintFindingsEmptyIsClean(t *testing.T) {
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := reportLintFindings(cmd, nil); err != nil {
		t.Fatalf("expected no error for zero findings, got: %v", err)
	}
	if !strings.Contains(buf.String(), "0 findings") {
		t.Fatalf("expected 0-findings message, got: %q", buf.String())
	}
}

func TestReportLintFindingsWarningOnlyDoesNotFail(t *testing.T) {
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	findings := []lint.Finding{
		{LintName: "orphan", ClaimID: "m.contract.x", Message: "no edges", Severity: lint.SeverityWarning},
	}
	if err := reportLintFindings(cmd, findings); err != nil {
		t.Fatalf("expected warning-only findings to not fail the command, got: %v", err)
	}
	if !strings.Contains(buf.String(), "orphan") {
		t.Fatalf("expected finding text in output, got: %q", buf.String())
	}
}

func TestReportLintFindingsErrorSeverityFails(t *testing.T) {
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	findings := []lint.Finding{
		{LintName: "dangling", ClaimID: "m.contract.x", Message: "missing target", Severity: lint.SeverityError},
	}
	err := reportLintFindings(cmd, findings)
	if err == nil {
		t.Fatalf("expected an error-severity finding to fail the command")
	}
	if !strings.Contains(buf.String(), "1 error(s)") {
		t.Fatalf("expected error count in output, got: %q", buf.String())
	}
}

// ---------------------------------------------------------------------
// storePath / catalogPath / renderOutPath: resolved against cfg.Dir(),
// never the process cwd (same convention as claims_dir).
// ---------------------------------------------------------------------

func TestPathHelpersResolveAgainstConfigDir(t *testing.T) {
	root := t.TempDir()
	cfgFile := filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgFile, []byte("schema_version: 1\nfacets:\n  - contract\nmodules:\n  - m\nclaims_dir: claims\n"), 0o644); err != nil {
		t.Fatalf("write fixture config: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "claims"), 0o755); err != nil {
		t.Fatalf("mkdir claims: %v", err)
	}

	cfg, err := config.LoadConfig(cfgFile)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if got, want := storePath(cfg), filepath.Join(root, ".dossierx-lock-store.json"); got != want {
		t.Fatalf("storePath: got %q, want %q", got, want)
	}
	if got, want := catalogPath(cfg), filepath.Join(root, ".catalog.json"); got != want {
		t.Fatalf("catalogPath: got %q, want %q", got, want)
	}
	if got, want := renderOutPath(cfg), filepath.Join(root, "viewer", "index.html"); got != want {
		t.Fatalf("renderOutPath: got %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------
// The shape of the surface itself
// ---------------------------------------------------------------------

// TestSurfaceIsNineteenLeavesUnderSevenNouns pins the headline of the v0.3.0
// restructure as a test rather than a promise in a changelog.
//
// The number is a design constraint: every verb here is something an AGENT
// does, and the argument for the release is that the surface got SMALLER while
// getting more capable. Adding a leaf should be a decision someone makes on
// purpose and writes down here, not something that accretes.
//
// It moved from nineteen-under-six to twenty-under-seven exactly once, for the
// migration verb v0.3.0 added when ledger adoption became fail-closed — and
// v0.4.0 takes it straight back out, which is why this reads nineteen again.
// Removal is the same kind of decision as addition and is recorded the same way:
// v0.4.0 removes adoption itself, so a project that predates the lock ledger
// crosses onto it by holding nothing that predates it (lock.CrossPreLedger)
// rather than by running a command that records content nobody approved. With
// nothing left to adopt there is nothing for the verb to do. It survives as a
// hidden retired stub, counted by TestRetiredInvocationsNameTheirReplacement
// rather than here.
func TestSurfaceIsNineteenLeavesUnderSevenNouns(t *testing.T) {
	want := map[string]bool{
		"check": true,

		"claim show":    true,
		"claim list":    true,
		"claim new":     true,
		"claim lock":    true,
		"claim unlock":  true,
		"claim flag":    true,
		"claim reaudit": true,
		"claim link":    true,

		"comment inbox": true,
		"comment list":  true,
		"comment add":   true,
		"comment reply": true,

		"build-order propose": true,
		"build-order status":  true,
		"build-order lock":    true,

		"serve":         true,
		"skills export": true,
		"version":       true,
	}

	got := map[string]bool{}
	var walk func(cmd *cobra.Command, prefix string)
	walk = func(cmd *cobra.Command, prefix string) {
		children := cmd.Commands()
		leaf := true
		for _, child := range children {
			// cobra injects "help" and "completion" into every tree; they are
			// framework furniture, not part of the product's surface.
			if child.Name() == "help" || child.Name() == "completion" {
				continue
			}
			// The removal stubs (see retired.go) are not surface: every one of
			// them does exactly one thing, which is to fail with the sentence
			// naming its replacement. They are excluded by their MARK rather
			// than by Hidden, so a real leaf can never be smuggled past this
			// count by hiding it — and TestRetiredInvocationsNameTheirReplacement
			// pins the set itself, so they are still counted, just elsewhere.
			if retired(child) {
				continue
			}
			leaf = false
			name := child.Name()
			if prefix != "" {
				name = prefix + " " + name
			}
			walk(child, name)
		}
		if leaf && prefix != "" {
			got[prefix] = true
		}
	}
	walk(newRootCmd(), "")

	for name := range want {
		if !got[name] {
			t.Errorf("missing leaf command: %q", name)
		}
	}
	for name := range got {
		if !want[name] {
			t.Errorf("unexpected leaf command %q — adding to the surface is a decision, not an accident; if it is intended, add it to this test's table and to the CHANGELOG", name)
		}
	}
	if len(got) != 19 {
		t.Errorf("the surface is 19 leaves; got %d: %v", len(got), sortedCommandNames(got))
	}
}

// sortedCommandNames renders a command-name set deterministically for a
// failure message.
func sortedCommandNames(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// ---------------------------------------------------------------------
// claimTitle and the --match scorer
// ---------------------------------------------------------------------

func TestClaimTitleDerivesTheLabelFromTheSlug(t *testing.T) {
	cases := map[string]string{
		"widget.contract.retry-policy": "Retry Policy",
		"widget.contract.overview":     "Overview",
		"widget.contract.a-b-c":        "A B C",
		// Not three segments: the raw id, verbatim. This runs outside the lint
		// suite, so it must never assume a well-formed id.
		"not-an-id":     "not-an-id",
		"two.segments":  "two.segments",
		"a.b.c.d":       "a.b.c.d",
		"trailing.dot.": "trailing.dot.",
	}
	for id, want := range cases {
		if got := claimTitle(id); got != want {
			t.Errorf("claimTitle(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestFuzzyScoreRanksByHowMuchTheCallerGuessed(t *testing.T) {
	const target = "widget.contract.retry-policy"

	exact := fuzzyScore(target, target)
	prefix := fuzzyScore("widget.contract", target)
	contains := fuzzyScore("retry-policy", target)
	tokens := fuzzyScore("retry contract", target)
	partial := fuzzyScore("retry nonsense", target)
	subseq := fuzzyScore("wcrp", target)
	none := fuzzyScore("zzzz", target)

	// The ladder must be strictly descending: a caller that typed more of the
	// real thing has to outrank one that typed less, or --match cannot be
	// trusted to put the right card first.
	ordered := []int{exact, prefix, contains, tokens, partial, subseq}
	for i := 1; i < len(ordered); i++ {
		if ordered[i] >= ordered[i-1] {
			t.Fatalf("fuzzy tiers must strictly descend, got %v", ordered)
		}
	}
	if none != 0 {
		t.Fatalf("a non-match must score 0, got %d", none)
	}
	if subseq <= 0 {
		t.Fatalf("an in-order subsequence must still match, got %d", subseq)
	}
}

func TestClaimMatchScorePrefersAnIDOrTitleHitOverTheJoinedHaystack(t *testing.T) {
	claim := model.Claim{ID: "widget.contract.retry-policy", Facet: "contract", Module: "widget"}

	// "retry-policy" is contained in the id itself.
	direct := claimMatchScore("retry-policy", claim)
	// "widget contract" appears only across the joined id/title/facet/module
	// haystack, never as one substring of the id or the title.
	joined := claimMatchScore("policy widget contract", claim)
	if direct <= joined {
		t.Fatalf("a direct id hit (%d) must outrank a joined-haystack hit (%d)", direct, joined)
	}
	if joined <= 0 {
		t.Fatalf("the joined haystack must still match how a human actually speaks, got %d", joined)
	}
	if claimMatchScore("completely unrelated words", claim) != 0 {
		t.Fatalf("an unrelated query must score 0")
	}
}

// TestSiteMetaDescriptionNamesTheRealLeafCount: the site's <meta description>
// is static HTML and cannot interpolate, so its command count is the one number
// on the whole site that nothing derives and nothing checked.
//
// It said "a 20-command JSON CLI" from v0.3.0 until v0.5.0's post-release gate
// read the DEPLOYED page and found it. Twenty was the v0.3.0 surface; v0.4.0 cut
// it to nineteen, and the meta tag stayed wrong through two minor releases —
// live, in the description search engines and link previews quote, which is
// exactly where a stale claim is least likely to be noticed by anyone editing
// the page.
//
// The count is derived here rather than pinned to a literal because this file
// is where the leaf set is authoritative: TestSurfaceIsNineteenLeavesUnderSevenNouns
// walks the same tree. Change the surface and this fails until the site follows.
func TestSiteMetaDescriptionNamesTheRealLeafCount(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "site", "index.html"))
	if err != nil {
		t.Fatalf("read site/index.html: %v", err)
	}

	m := regexp.MustCompile(`(\d+)-command`).FindSubmatch(raw)
	if m == nil {
		// Phrasing that carries no count cannot go stale, so this is a pass.
		t.Skip("site/index.html's description states no command count")
	}
	claimed, err := strconv.Atoi(string(m[1]))
	if err != nil {
		t.Fatalf("unparseable command count %q: %v", m[1], err)
	}

	leaves := 0
	var walk func(cmd *cobra.Command, prefix string)
	walk = func(cmd *cobra.Command, prefix string) {
		leaf := true
		for _, child := range cmd.Commands() {
			if child.Name() == "help" || child.Name() == "completion" || retired(child) {
				continue
			}
			leaf = false
			name := child.Name()
			if prefix != "" {
				name = prefix + " " + name
			}
			walk(child, name)
		}
		if leaf && prefix != "" {
			leaves++
		}
	}
	walk(newRootCmd(), "")

	if claimed != leaves {
		t.Errorf("site/index.html's meta description claims a %d-command CLI; the surface has %d leaves.\n"+
			"This string is served to search engines and link previews, where nobody editing the site will see it.\n"+
			"Fix the number, or drop the count from the sentence so it cannot go stale again.", claimed, leaves)
	}
}
