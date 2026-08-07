package viewertests

// THE TOOLCHAIN THAT PRODUCES THE DOM, AND THE ONE THAT PUBLISHES IT.
//
// site_dom_test.go reads the site as rendered DOM and its header claims the
// dist/ it reads is built "exactly as .github/workflows/deploy-site.yml does".
// Nothing checked that sentence, and it is exactly the kind of claim this gate
// exists to distrust: true when it was written, false the moment either side
// moves, and silent in between. Add a `VITE_*` variable to the publish build and
// not to this one and the gate reads a page no visitor gets — while every
// condition in site_dom_test.go stays green, because all seven are integrity
// checks on a dump whose provenance nothing had stated.
//
// So this file states the provenance and then checks it, in three parts.
//
//   - PREREQUISITES. node and npm are resolved and version-stamped rather than
//     assumed, and a missing one is a t.Fatal for the same reason a missing
//     browser is (CLAUDE.md; harness_test.go:47). The versions go into
//     site-text.json, because "which Node produced this" is not answerable after
//     the fact from a runner image that has moved on, and a prose agent reading
//     that artifact is reading a claim about the PUBLISHED site.
//   - PARITY. The npm steps and the build environment are compared name by name
//     against deploy-site.yml, in both directions — a step or a variable on
//     either side that the other does not have is a red build. The single
//     deliberate divergence, VITE_BASE's value, is DECLARED in
//     declaredBuildEnvDivergences with its reason, in the same shape
//     surfaces.yaml uses to declare a path out of scope, so a second divergence
//     cannot arrive quietly under cover of the first.
//   - THE NODE FLOOR. deploy-site.yml must pin a Node version at all — an
//     unpinned publish build is a site built by whatever the runner image
//     shipped that week — and the Node this extraction runs under must be at
//     least that pin. The direction is not symmetry for its own sake: newer is
//     the safe side for this toolchain — vite is the binding constraint at
//     `^18.0.0 || >=20.0.0` (typescript's own floor is far lower), the range is
//     open at the top, and a newer Node that broke either one fails the build
//     loudly rather than quietly producing a different DOM. OLDER than the pin
//     is the case with the silent failure mode, and it is the one refused.
//
// WHAT THIS FILE DOES NOT DO, stated here rather than left to be discovered.
// ci.yml's `viewer` job — the only job that runs this module — has no
// actions/setup-node, no npm cache and no pinned Node, so on CI this extraction
// builds under whatever ubuntu-latest ships and re-downloads the dependency tree
// on every run, while deploy-site.yml pins Node 20 with `cache: npm`. That is a
// six-line edit to .github/workflows/ci.yml, which is outside this lane's write
// set; it is reported rather than made. checkNodeFloor below is the half of it
// that can be asserted from here, and it is what turns a runner-image Node bump
// from a cryptic vite failure into a named one.
//
// The rules are functions that RETURN their finding, and
// TestSiteToolchainRulesCatchTheirOwnDefects runs each of them over a synthetic
// workflow carrying the defect it names. That is the same arrangement
// site_source_test.go uses and for the same reason: the only honest way to show
// a rule goes red on a bad publish workflow, without editing a workflow this
// lane does not own, is to hand it one that is already bad.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// deploySiteWorkflow is the workflow that publishes the site, repo-relative.
// It is the only build of site/ a visitor ever sees.
const deploySiteWorkflow = ".github/workflows/deploy-site.yml"

// minPinnedNodeMajor is the floor the publish workflow's own pin has to clear.
// site/package-lock.json resolves vite 5.4.21, whose engines field is
// `^18.0.0 || >=20.0.0` — the binding constraint, since the typescript it
// resolves alongside asks only for >=14.17. Below 18 the publish build does not
// merely differ from the one this extraction makes, it does not run, and it says
// so somewhere inside vite rather than in terms of Node.
const minPinnedNodeMajor = 18

// ---------------------------------------------------------------------
// the prerequisites
// ---------------------------------------------------------------------

// siteToolchain is the node/npm pair a dist/ was built by. The versions are
// serialised into site-text.json; the resolved paths are not, because they are
// facts about one machine and the artifact is read on others.
type siteToolchain struct {
	NodeVersion string `json:"node"`
	NPMVersion  string `json:"npm"`
	NodeMajor   int    `json:"node_major"`

	nodePath string
	npmPath  string
}

// reNodeVersion pulls the major out of `node --version`, which prints "v24.13.0".
var reNodeVersion = regexp.MustCompile(`^v?(\d+)\.`)

// requireSiteToolchain resolves node and npm or fails. It is requireSiteBrowser's
// sibling: with no toolchain there is no build, with no build there is no
// rendered DOM, and "we did not look" must not read as "nothing is wrong".
//
// node is resolved SEPARATELY from npm even though npm is itself a node script,
// because the version is what the artifact records and what checkNodeFloor
// reads; taking it off the PATH is the same answer npm's own shim reaches for.
func requireSiteToolchain(t *testing.T) siteToolchain {
	t.Helper()
	var tc siteToolchain

	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Fatalf("node is not installed, so the site cannot be built and its rendered DOM cannot be "+
			"read. A check that cannot execute is a FAILED gate, never a skip. On CI this is what a "+
			"runner image with no Node looks like; add actions/setup-node to the job: %v", err)
	}
	npmPath, err := exec.LookPath("npm")
	if err != nil {
		t.Fatalf("npm is not installed, so the site cannot be built and its rendered DOM "+
			"cannot be read. A check that cannot execute is a FAILED gate, never a skip: %v", err)
	}
	tc.nodePath, tc.npmPath = nodePath, npmPath
	tc.NodeVersion = toolVersion(t, nodePath)
	tc.NPMVersion = toolVersion(t, npmPath)

	m := reNodeVersion.FindStringSubmatch(tc.NodeVersion)
	if m == nil {
		t.Fatalf("`node --version` printed %q, which is not vMAJOR.MINOR.PATCH. The major is what the "+
			"publish workflow's pin is compared against, so an unreadable one is a check that cannot "+
			"execute rather than one that passes", tc.NodeVersion)
	}
	major, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatalf("node major %q is not a number: %v", m[1], err)
	}
	tc.NodeMajor = major

	t.Logf("site build toolchain: node %s (%s), npm %s (%s)", tc.NodeVersion, nodePath, tc.NPMVersion, npmPath)
	return tc
}

func toolVersion(t *testing.T, path string) string {
	t.Helper()
	out, err := exec.Command(path, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("%s --version: %v\n%s", path, err, out)
	}
	return strings.TrimSpace(string(out))
}

// ---------------------------------------------------------------------
// the publish workflow, read as text
// ---------------------------------------------------------------------

// The workflow is scanned rather than parsed as YAML, following
// tests/nested_module_coverage_test.go: viewer-tests' go.mod is chromedp and
// nothing else, and a YAML dependency taken on to read three lines out of one
// file is a dependency taken for the rest of the module's life.
//
// Each of these is anchored tightly enough that a shape it cannot read is
// ABSENT rather than misread, and every absence below is asserted against — an
// empty command set, an empty variable set and a missing pin are all failures,
// because a parse that quietly returned nothing would agree with every
// declaration this file makes.
var (
	// The leading `[\s-]*` accepts the sequence-item form `- run: npm ci` as well
	// as the block form deploy-site.yml uses today. `run:` with nothing after it
	// — the `defaults:` block's key — cannot match, because a command is required.
	reWorkflowNPM     = regexp.MustCompile(`(?m)^[\s-]*run:\s*(npm[^\n]*?)\s*$`)
	reWorkflowViteEnv = regexp.MustCompile(`(?m)^\s*(VITE_[A-Z0-9_]+):\s*(\S+)\s*$`)
	reWorkflowNodePin = regexp.MustCompile(`(?m)^\s*node-version:\s*['"]?(\d+)[^'"\s]*['"]?\s*$`)
)

// deployBuild is everything about the publish build that this extraction has to
// match.
type deployBuild struct {
	npmCommands []string          // "npm ci", "npm run build", …
	viteEnv     map[string]string // VITE_BASE -> /dossierx/
	nodePins    []int             // every node-version: pin in the file
}

func parseDeployBuild(src string) deployBuild {
	db := deployBuild{viteEnv: map[string]string{}}
	for _, m := range reWorkflowNPM.FindAllStringSubmatch(src, -1) {
		db.npmCommands = append(db.npmCommands, m[1])
	}
	for _, m := range reWorkflowViteEnv.FindAllStringSubmatch(src, -1) {
		db.viteEnv[m[1]] = m[2]
	}
	for _, m := range reWorkflowNodePin.FindAllStringSubmatch(src, -1) {
		if n, err := strconv.Atoi(m[1]); err == nil {
			db.nodePins = append(db.nodePins, n)
		}
	}
	return db
}

// ---------------------------------------------------------------------
// the extraction side, read off its own declaration
// ---------------------------------------------------------------------

// extractionNPMCommands renders siteBuildSteps the way the workflow spells the
// same invocations, so the two lists compare as strings a reader can see are the
// same thing.
func extractionNPMCommands(steps []siteBuildStep) []string {
	out := make([]string, 0, len(steps))
	for _, s := range steps {
		out = append(out, "npm "+strings.Join(s.Args, " "))
	}
	return out
}

// extractionViteEnv is every VITE_* variable siteBuildSteps sets, whichever step
// sets it. A variable that reaches vite reaches it for the whole build.
func extractionViteEnv(steps []siteBuildStep) map[string]string {
	out := map[string]string{}
	for _, s := range steps {
		for _, kv := range s.Env {
			name, value, ok := strings.Cut(kv, "=")
			if ok && strings.HasPrefix(name, "VITE_") {
				out[name] = value
			}
		}
	}
	return out
}

// ---------------------------------------------------------------------
// rule 1 — the steps and the environment
// ---------------------------------------------------------------------

// buildEnvDivergence declares ONE build variable whose value the extraction
// deliberately sets differently from the publish build, and why that difference
// changes nothing this gate reads.
//
// It is an inventory rather than a rule for the same reason
// declaredHistoryLiterals is: a rule ("ignore VITE_BASE") exempts a name
// forever, including the day somebody changes what that name means, whereas a
// declaration carries the argument next to the exemption and a second divergence
// has to be argued for on its own.
type buildEnvDivergence struct {
	name       string
	extraction string // the value siteBuildSteps sets
	published  string // the value deploy-site.yml sets
	why        string
}

var declaredBuildEnvDivergences = []buildEnvDivergence{
	{
		name:       "VITE_BASE",
		extraction: "/",
		published:  "/dossierx/",
		why: "vite writes asset URLs absolutely, so a dist built for the Pages subpath cannot load " +
			"its own JavaScript from the root of the test server this extraction serves it from. " +
			"It changes the asset URL prefix and nothing in the DOM's prose",
	},
}

// checkBuildParity compares the extraction's build against the publish build in
// BOTH directions. A step or a variable on either side that the other does not
// have is a finding: an extra step in the publish build is output this gate
// never sees, and an extra one here is output no visitor gets.
func checkBuildParity(db deployBuild, steps []siteBuildStep, declared []buildEnvDivergence) error {
	if len(db.npmCommands) == 0 {
		return fmt.Errorf("%s runs no `npm …` step this scan can read, so comparing the extraction's "+
			"build against it would compare a list to an empty one. Either the publish build stopped "+
			"using npm or reWorkflowNPM stopped reading it; both are a FAILED gate", deploySiteWorkflow)
	}
	if len(db.viteEnv) == 0 {
		return fmt.Errorf("%s sets no VITE_* variable this scan can read, so the environment comparison "+
			"below would run over nothing. VITE_BASE is set there today", deploySiteWorkflow)
	}

	if got, want := sortedCopy(extractionNPMCommands(steps)), sortedCopy(db.npmCommands); !equalStrings(got, want) {
		return fmt.Errorf("the extraction runs %v but %s runs %v.\nsite_dom_test.go builds the site so "+
			"that the gate reads the page the publish build produces; a step on one side and not the "+
			"other means it is reading something else. Bring siteBuildSteps back into line, or change "+
			"both", got, deploySiteWorkflow, want)
	}

	byName := map[string]buildEnvDivergence{}
	for _, d := range declared {
		if d.extraction == d.published {
			return fmt.Errorf("declared build env divergence %q sets the same value on both sides (%q), "+
				"so it declares no divergence and hides that it declares none", d.name, d.extraction)
		}
		byName[d.name] = d
	}

	mine := extractionViteEnv(steps)
	var findings []string
	for _, name := range sortedKeys(db.viteEnv) {
		published := db.viteEnv[name]
		got, set := mine[name]
		if !set {
			findings = append(findings, fmt.Sprintf("%s=%q is set by the publish build and not by the "+
				"extraction, so the gate reads a build the visitor does not get", name, published))
			continue
		}
		d, isDeclared := byName[name]
		switch {
		case got == published:
			if isDeclared {
				findings = append(findings, fmt.Sprintf("%s is declared as a divergence but both sides "+
					"now set %q — delete the declaration rather than leaving an exemption standing",
					name, got))
			}
		case !isDeclared:
			findings = append(findings, fmt.Sprintf("%s is %q here and %q in the publish build, and "+
				"nothing declares why. Add it to declaredBuildEnvDivergences with the argument that "+
				"the difference changes no byte this gate reads, or set the same value", name, got, published))
		case d.extraction != got || d.published != published:
			findings = append(findings, fmt.Sprintf("%s is declared as %q here / %q published, but is "+
				"actually %q here / %q published. The declaration is the argument that the difference "+
				"is harmless, and it is being made about other values", name, d.extraction, d.published, got, published))
		}
	}
	for _, name := range sortedKeys(mine) {
		if _, ok := db.viteEnv[name]; !ok {
			findings = append(findings, fmt.Sprintf("%s=%q is set by the extraction and by nothing in "+
				"the publish build, so the DOM this gate reads was configured by a variable the "+
				"published site never sees", name, mine[name]))
		}
	}
	for _, d := range declared {
		if _, ok := db.viteEnv[d.name]; !ok {
			findings = append(findings, fmt.Sprintf("%s is declared as a divergence but the publish "+
				"build does not set it at all; the declaration covers nothing", d.name))
		}
	}
	if len(findings) > 0 {
		return fmt.Errorf("the extraction's build and %s have drifted apart in %d way(s):\n  %s",
			deploySiteWorkflow, len(findings), strings.Join(findings, "\n  "))
	}
	return nil
}

// ---------------------------------------------------------------------
// rule 2 — the Node floor
// ---------------------------------------------------------------------

// checkNodeFloor asserts the publish workflow pins Node at all, that the pin is
// one this site's own dependencies can be built by, and that the extraction is
// running under at least that major.
//
// "At least", not "exactly", and the asymmetry is the argument: this toolchain's
// support range is open at the top (vite 5 declares ^18 || >=20), a newer Node
// that broke tsc or vite fails the build loudly rather than quietly producing a
// different DOM, and pinning equality here would make every developer's laptop
// a red gate for a reason that has nothing to do with the site. Running OLDER
// than the pin is the case with a silent failure mode, and it is the one refused.
func checkNodeFloor(db deployBuild, runningMajor int) error {
	if len(db.nodePins) == 0 {
		return fmt.Errorf("%s pins no Node version. The site a visitor reads would then be built by "+
			"whatever the runner image shipped that week, and this extraction would have no version to "+
			"hold itself to. Add actions/setup-node with an explicit node-version", deploySiteWorkflow)
	}
	pin := db.nodePins[0]
	for _, p := range db.nodePins {
		if p != pin {
			return fmt.Errorf("%s pins more than one Node major (%v). Which one publishes the site is "+
				"then a question about job order rather than about the workflow", deploySiteWorkflow, db.nodePins)
		}
	}
	if pin < minPinnedNodeMajor {
		return fmt.Errorf("%s pins Node %d, below the %d that site/package.json's own dependencies "+
			"require (the vite it resolves declares engines ^18.0.0 || >=20.0.0). The publish build "+
			"would not run",
			deploySiteWorkflow, pin, minPinnedNodeMajor)
	}
	if runningMajor < pin {
		return fmt.Errorf("this extraction is building the site with Node %d, but %s publishes it with "+
			"Node %d. The gate would be reading a tree the publish build does not necessarily produce.\n"+
			"On CI this is what an unpinned job looks like: the `viewer` job in .github/workflows/ci.yml "+
			"has no actions/setup-node, so it builds under whatever the runner image ships. Pin it to %d "+
			"there, the way deploy-site.yml already does",
			runningMajor, deploySiteWorkflow, pin, pin)
	}
	return nil
}

// ---------------------------------------------------------------------
// the rules, against the tree under test
// ---------------------------------------------------------------------

func repoDeployBuild(t *testing.T) deployBuild {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}
	path := filepath.Join(root, filepath.FromSlash(deploySiteWorkflow))
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v\nThe extraction claims to build the site the way this workflow does; "+
			"without it there is nothing to hold that claim to", path, err)
	}
	return parseDeployBuild(string(b))
}

// TestSiteBuildMatchesTheDeploySiteWorkflow is the parity check against the real
// files. It needs no browser — the claim it checks is about how dist/ is
// produced, not about what is in it — but it does need node, because the Node
// floor is a fact about the process that would run the build.
func TestSiteBuildMatchesTheDeploySiteWorkflow(t *testing.T) {
	db := repoDeployBuild(t)
	tc := requireSiteToolchain(t)

	if err := checkBuildParity(db, siteBuildSteps, declaredBuildEnvDivergences); err != nil {
		t.Fatal(err)
	}
	if err := checkNodeFloor(db, tc.NodeMajor); err != nil {
		t.Fatal(err)
	}
	t.Logf("build parity: %v, VITE env %v (publish pins Node %v, extraction running Node %d)",
		extractionNPMCommands(siteBuildSteps), db.viteEnv, db.nodePins, tc.NodeMajor)
}

// ---------------------------------------------------------------------
// the negative controls
// ---------------------------------------------------------------------

// fixtureDeployWorkflow renders a publish workflow carrying the given npm steps,
// VITE_* variables and Node pin — enough of deploy-site.yml's shape for the two
// rules to run over, and nothing else.
func fixtureDeployWorkflow(nodePin string, viteEnv map[string]string, npmSteps ...string) string {
	var b strings.Builder
	b.WriteString("jobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n")
	if nodePin != "" {
		b.WriteString("      - name: Set up Node.js\n        uses: actions/setup-node@v7\n        with:\n")
		fmt.Fprintf(&b, "          node-version: '%s'\n          cache: npm\n", nodePin)
	}
	for _, step := range npmSteps {
		fmt.Fprintf(&b, "      - name: Step\n        run: %s\n", step)
	}
	if len(viteEnv) > 0 {
		b.WriteString("        env:\n")
		for _, k := range sortedKeys(viteEnv) {
			fmt.Fprintf(&b, "          %s: %s\n", k, viteEnv[k])
		}
	}
	return b.String()
}

// TestSiteToolchainRulesCatchTheirOwnDefects is the mutation test, kept. Both
// rules pass against the repository today, and a rule that passed against a
// drifted publish workflow too would read as coverage while asserting nothing —
// so every case is a workflow (or a siteBuildSteps) carrying exactly the defect
// the rule names, alongside the healthy twin that proves it is not simply always
// red.
//
// Fixtures rather than edits to .github/, for the reason site_source_test.go
// gives about site/: it is not this lane's tree to mutate, and a test that
// mutates the subject is a test that can lose someone else's work.
func TestSiteToolchainRulesCatchTheirOwnDefects(t *testing.T) {
	healthySteps := []siteBuildStep{
		{Args: []string{"ci"}},
		{Args: []string{"run", "build"}, Env: []string{"VITE_BASE=/"}},
	}
	declared := []buildEnvDivergence{
		{name: "VITE_BASE", extraction: "/", published: "/dossierx/", why: "asset URL prefix only"},
	}
	healthyWorkflow := fixtureDeployWorkflow("20",
		map[string]string{"VITE_BASE": "/dossierx/"}, "npm ci", "npm run build")

	t.Run("parity", func(t *testing.T) {
		cases := []struct {
			name     string
			workflow string
			steps    []siteBuildStep
			declared []buildEnvDivergence
			wantErr  bool
		}{
			{
				name:     "the tree as it stands",
				workflow: healthyWorkflow,
				steps:    healthySteps,
				declared: declared,
			},
			{
				// The finding this rule exists for: the publish build grows a
				// variable, the extraction does not, and the gate reads a page
				// configured differently from the one that ships — with every
				// condition in site_dom_test.go still green.
				name: "the publish build sets a variable the extraction does not",
				workflow: fixtureDeployWorkflow("20",
					map[string]string{"VITE_BASE": "/dossierx/", "VITE_ANALYTICS_ID": "abc123"},
					"npm ci", "npm run build"),
				steps:    healthySteps,
				declared: declared,
				wantErr:  true,
			},
			{
				name:     "the extraction sets a variable the publish build does not",
				workflow: healthyWorkflow,
				steps: []siteBuildStep{
					{Args: []string{"ci"}},
					{Args: []string{"run", "build"}, Env: []string{"VITE_BASE=/", "VITE_FAKE_FLAG=1"}},
				},
				declared: declared,
				wantErr:  true,
			},
			{
				// An UNDECLARED value divergence. VITE_BASE's is fine because
				// somebody wrote down why; a second one is not fine by
				// association.
				name:     "a value divergence nobody declared",
				workflow: fixtureDeployWorkflow("20", map[string]string{"VITE_BASE": "/dossierx/"}, "npm ci", "npm run build"),
				steps: []siteBuildStep{
					{Args: []string{"ci"}},
					{Args: []string{"run", "build"}, Env: []string{"VITE_BASE=/elsewhere/"}},
				},
				declared: nil,
				wantErr:  true,
			},
			{
				// The declaration is about specific values, so a publish build
				// that changed its side of one is a re-read rather than a
				// standing exemption.
				name:     "the declared values are no longer the values",
				workflow: fixtureDeployWorkflow("20", map[string]string{"VITE_BASE": "/dx/"}, "npm ci", "npm run build"),
				steps:    healthySteps,
				declared: declared,
				wantErr:  true,
			},
			{
				name:     "a declaration over a variable that no longer diverges",
				workflow: fixtureDeployWorkflow("20", map[string]string{"VITE_BASE": "/"}, "npm ci", "npm run build"),
				steps:    healthySteps,
				declared: declared,
				wantErr:  true,
			},
			{
				name:     "the publish build runs a step the extraction does not",
				workflow: fixtureDeployWorkflow("20", map[string]string{"VITE_BASE": "/dossierx/"}, "npm ci", "npm run lint", "npm run build"),
				steps:    healthySteps,
				declared: declared,
				wantErr:  true,
			},
			{
				name:     "the extraction installs differently from the publish build",
				workflow: healthyWorkflow,
				steps: []siteBuildStep{
					{Args: []string{"install"}},
					{Args: []string{"run", "build"}, Env: []string{"VITE_BASE=/"}},
				},
				declared: declared,
				wantErr:  true,
			},
			{
				// The vacuity guards. A workflow this scan cannot read agrees
				// with every declaration ever made.
				name:     "a workflow with no npm step at all",
				workflow: fixtureDeployWorkflow("20", map[string]string{"VITE_BASE": "/dossierx/"}),
				steps:    healthySteps,
				declared: declared,
				wantErr:  true,
			},
			{
				name:     "a workflow with no VITE_ variable at all",
				workflow: fixtureDeployWorkflow("20", nil, "npm ci", "npm run build"),
				steps:    healthySteps,
				declared: declared,
				wantErr:  true,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				err := checkBuildParity(parseDeployBuild(tc.workflow), tc.steps, tc.declared)
				if tc.wantErr && err == nil {
					t.Fatal("accepted; the extraction and the publish build would have drifted with " +
						"nothing saying so")
				}
				if !tc.wantErr && err != nil {
					t.Fatalf("rejected: %v", err)
				}
				if err != nil {
					t.Logf("rejected: %v", err)
				}
			})
		}
	})

	t.Run("the node floor", func(t *testing.T) {
		cases := []struct {
			name     string
			workflow string
			running  int
			wantErr  bool
		}{
			{name: "running the pinned major", workflow: healthyWorkflow, running: 20},
			{name: "running newer than the pin", workflow: healthyWorkflow, running: 24},
			{name: "running older than the pin", workflow: healthyWorkflow, running: 18, wantErr: true},
			{
				name:     "a publish build that pins nothing",
				workflow: fixtureDeployWorkflow("", map[string]string{"VITE_BASE": "/dossierx/"}, "npm ci", "npm run build"),
				running:  24,
				wantErr:  true,
			},
			{
				name:     "a pin this site's dependencies cannot be built by",
				workflow: fixtureDeployWorkflow("16", map[string]string{"VITE_BASE": "/dossierx/"}, "npm ci", "npm run build"),
				running:  24,
				wantErr:  true,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				err := checkNodeFloor(parseDeployBuild(tc.workflow), tc.running)
				if tc.wantErr && err == nil {
					t.Fatal("accepted; the gate would be reading a build the publish workflow does not make")
				}
				if !tc.wantErr && err != nil {
					t.Fatalf("rejected: %v", err)
				}
				if err != nil {
					t.Logf("rejected: %v", err)
				}
			})
		}
	})

	// The parse is checked last and separately, because both rules are defined
	// against what it returns: a regex that stopped reading the workflow does
	// not fail, it returns less, and "less" is what every vacuity guard above is
	// there to catch. This pins the shapes deploy-site.yml actually uses.
	t.Run("the workflow scan reads the shapes the real file uses", func(t *testing.T) {
		db := parseDeployBuild(fixtureDeployWorkflow("20",
			map[string]string{"VITE_BASE": "/dossierx/"}, "npm ci", "npm run build"))
		if got := sortedCopy(db.npmCommands); !equalStrings(got, []string{"npm ci", "npm run build"}) {
			t.Fatalf("npm steps read as %v", got)
		}
		if db.viteEnv["VITE_BASE"] != "/dossierx/" {
			t.Fatalf("VITE_BASE read as %q", db.viteEnv["VITE_BASE"])
		}
		if len(db.nodePins) != 1 || db.nodePins[0] != 20 {
			t.Fatalf("node pins read as %v", db.nodePins)
		}
		// `defaults: run:` carries no command and must not be read as one, and a
		// quoted or dotted pin is the same pin.
		db = parseDeployBuild("defaults:\n  run:\n    working-directory: site\n" +
			"      - run: npm ci\n        node-version: \"20.11.1\"\n")
		if got := sortedCopy(db.npmCommands); !equalStrings(got, []string{"npm ci"}) {
			t.Fatalf("a bare `run:` key was read as a command: %v", got)
		}
		if len(db.nodePins) != 1 || db.nodePins[0] != 20 {
			t.Fatalf("a quoted, dotted pin read as %v", db.nodePins)
		}
	})
}

// ---------------------------------------------------------------------
// small shared helpers
// ---------------------------------------------------------------------

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
