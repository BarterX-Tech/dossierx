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
// So this file states the provenance and then checks it, in three parts — plus
// one prerequisite of a different kind, the release build, which lives here for
// the reason given above that section rather than because it is about the site.
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
// WHERE THE FOURTH RULE WENT, because it used to be here and this file's claim
// about its own coverage has to move with it. Rule 3 was
// TestCIToolchainGuardSpeaksUnderTheRealCIShell: it read a `run: |` guard step
// out of ci.yml, executed it under `bash --noprofile --norc -eo pipefail`, and
// mutation-tested its diagnostics. That step is gone and so is the test, and the
// invariant they shared — ci.yml's Node pin equals deploy-site.yml's — is now
// tests/ci_workflow_test.go in the ROOT module, which parses both workflows as
// YAML and identifies the job by the step that enters this module.
//
// It moved because the pair could be defeated together. The guard lived inside
// the job it guarded and located that job by its own step name, so moving the
// step AND the guard verbatim into another job left the guard slicing out that
// job, finding the pin it had travelled with, printing "site toolchain pinned"
// and exiting 0 — with this file's mutation test green, because the mutation it
// ran was against a workflow the guard was still inside. A check cannot be a
// reliable witness to a file it is part of.
//
// WHAT THIS FILE DOES NOT DO, stated here rather than left to be discovered: the
// DIRECTION OF THE BLINDNESS IN RULES 1 AND 2. Neither of them opens ci.yml at
// all, which is why the invariant above needs a reader outside this module and
// cannot be a third comparison in Go here.
//
//   - checkNodeFloor compares deploy-site.yml's pin against the RUNNING process's
//     Node. It never reads ci.yml. So it catches the publish workflow pinning
//     ABOVE what this extraction is running under, and it is blind to ci.yml's own
//     pin being deleted — a maintainer running Node 22 on a laptop sees nothing,
//     and neither does a runner image that ships a new-enough Node.
//   - checkBuildParity reads deploy-site.yml and siteBuildSteps, and likewise
//     never opens ci.yml.
//
// Nor does either reach the surface fingerprint: surfaces.yaml puts .github/ out
// of scope, so no fingerprint entry exists for either workflow.
//
// The rules are functions that RETURN their finding, and
// TestSiteToolchainRulesCatchTheirOwnDefects runs each of them over a synthetic
// workflow carrying the defect it names. That is the same arrangement
// site_source_test.go uses and for the same reason: the only honest way to show
// a rule goes red on a bad publish workflow, without editing a workflow this
// lane does not own, is to hand it one that is already bad.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
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
			"WHAT MOVED IS THE PIN, not the job: %s's node-version is the only number this rule reads, "+
			"and it is now higher than the Node this process is running. Two places have to follow it.\n"+
			"  - On a developer machine, that is the machine: install Node %d or newer and re-run. "+
			"Nothing in the repository is wrong.\n"+
			"  - In CI, the job in .github/workflows/ci.yml that runs this module pins its own Node with "+
			"actions/setup-node and that pin has to be moved to %d as well. Reaching this branch there "+
			"means tests/ci_workflow_test.go — which parses both workflows and compares the two pins "+
			"directly, and fails first — did not run, so check that too rather than only bumping the "+
			"number.",
			runningMajor, deploySiteWorkflow, pin, deploySiteWorkflow, pin, pin)
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

// ---------------------------------------------------------------------
// the third tool this module refuses to skip on: the release build
// ---------------------------------------------------------------------

// THE GORELEASER DRY RUN, and why it is in THIS module.
//
// It used to live in cmd/dossierx, in the root module, and it was right about
// everything except where it stood. It requires a tool `go test` cannot produce,
// it FAILS rather than skips when that tool is not named — both correct, both
// kept below — and the consequence was that plain `go test ./...` was RED for
// every developer who had not installed GoReleaser. That is not a strict gate; it
// is a gate that makes the ordinary build unusable, and the pressure it creates is
// pressure to narrow a package selector, which is the one repair this repository
// does not accept.
//
// The browser suite had already solved this and the copy took half the solution:
// viewer-tests/ is a separate module precisely so that a check with an external
// prerequisite can be strict without holding the engine's build hostage. The root
// `go test ./...` does not descend here, CI runs this module as its own job, and
// the job supplies the tool exactly as it supplies a browser. So the check moves
// beside the suite that already works this way, and nothing about its strictness
// changes: with DOSSIERX_TEST_GORELEASER unset this file FAILS.
//
// WHAT IT IS FOR. Everything in cmd/dossierx/gate_release_stamp_test.go reads the
// release CONFIGURATION, which is an input. The failure that matters is not an
// input failure: `-X` aimed at a main package's full import path is a perfectly
// well-formed ldflags line that the linker accepts, records in the build settings,
// and applies to nothing. Parsing the file catches that spelling by name — it
// does — but only the spellings someone thought to name. Building the binary and
// asking it its version catches the class.

// goreleaserEnv names the GoReleaser binary this suite drives.
//
// It is the same contract requireSiteBrowser has with DOSSIERX_TEST_BROWSER, and
// for the same reason: the check needs a tool `go test` cannot produce, the tool
// must be supplied by whoever runs the suite rather than fetched at run time, and
// the case where it is missing is a FAILURE. There is no value of this variable
// that means "we did not build the release" and reads as "the release builds."
const goreleaserEnv = "DOSSIERX_TEST_GORELEASER"

// goreleaserConfigFile is the release build's configuration, repo-relative.
//
// It is what this repository INTENDS, and this file no longer takes it on trust.
// The dry run below builds from a copy of that file, so if GoReleaser would load
// a different one the dry run exercises a configuration the release does not use
// — a green build of a document nobody publishes. See
// requireGoreleaserLoadsThisConfig.
const goreleaserConfigFile = ".goreleaser.yaml"

// goreleaserCandidates is GoReleaser's configuration search order, copied from
// the pinned tool: goreleaser/v2@v2.17.1, cmd/config.go, loadConfigCheck.
//
// THE ORDER IS NOT THE OBVIOUS ONE and that is the whole reason this list is
// here. `.goreleaser.yml` is tried BEFORE `.goreleaser.yaml`, and two `.config/`
// paths precede both — so four filenames shadow the one this tree keeps, and
// `.github/workflows/release.yml` runs `goreleaser release --clean` with no
// `--config` at all. A copy of the real configuration with one template changed,
// dropped in at any of those four paths, publishes the release while every check
// that reads `.goreleaser.yaml` stays green, including the one below: it would
// dry-run the shadowed file, watch it produce six correct archives, and certify
// a build that never happens.
var goreleaserCandidates = []string{
	".config/goreleaser.yml",
	".config/goreleaser.yaml",
	".goreleaser.yml",
	".goreleaser.yaml",
	"goreleaser.yml",
	"goreleaser.yaml",
}

// reGoreleaserConfigPath reads the path GoReleaser reports it is using out of
// `goreleaser check`'s own output — `• checking   path=.goreleaser.yaml`.
//
// THIS IS THE TOOL ANSWERING, not a file being searched. The list above is a
// model of another program's behaviour, and this repository's whole history with
// this gate is a history of unread models: it is checked against the program
// itself wherever the program is available, which is exactly where this module's
// contract says it must be.
var reGoreleaserConfigPath = regexp.MustCompile(`(?m)^.*\bchecking\b.*\bpath=(\S+)\s*$`)

// reANSIEscape matches the CSI escape sequences GoReleaser's logger emits when
// it decides its output is being rendered — which on a CI runner it is.
var reANSIEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// requireGoreleaserLoadsThisConfig proves that goreleaserConfigFile is the file a
// release would open, and fails saying so when it is not.
//
// Two independent readings, because either alone has a hole:
//
//   - THE FILESYSTEM. Every one of GoReleaser's six candidate paths is stat'ed,
//     and a second one existing is a failure even when the first is the right
//     one. Which file governs is then a fact about this repository rather than a
//     question about a search order.
//   - THE TOOL. `goreleaser check` is run in the repository root with no
//     `--config`, exactly as release.yml runs `goreleaser release --clean`, and
//     it reports the path it resolved. Its exit status is deliberately ignored —
//     a configuration that is invalid for some other reason is a different
//     finding, made elsewhere — but the path it names is read as the answer it
//     is.
func requireGoreleaserLoadsThisConfig(t *testing.T, tool, root string) {
	t.Helper()

	var present []string
	for _, candidate := range goreleaserCandidates {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(candidate))); err == nil {
			present = append(present, candidate)
		}
	}
	switch {
	case len(present) == 0:
		t.Fatalf("none of GoReleaser's six candidate configuration paths exists in this repository: %v. A release would be built from the tool's built-in defaults — no ldflags, no archive names, no checksum file — and the dry run below would be exercising nothing this tree describes",
			goreleaserCandidates)
	case len(present) > 1:
		t.Fatalf("%d of GoReleaser's candidate configuration paths exist: %v.\n"+
			"It tries them in the order %v and loads the FIRST, so %q is what would publish and the rest are read by nobody. This test builds its dry run from %q, so in this state it would run the release build against a file the release does not open and report six correct archives about it.\n"+
			"There is exactly one release configuration in this repository: delete the others",
			len(present), present, goreleaserCandidates, present[0], goreleaserConfigFile)
	case present[0] != goreleaserConfigFile:
		t.Fatalf("GoReleaser would load %q — the first of its candidate paths that exists — and this test dry-runs %q. Every assertion below would describe a build that does not happen",
			present[0], goreleaserConfigFile)
	}

	// And the tool's own answer, which is the only reading here that is not a
	// model of the tool.
	check := exec.Command(tool, "check")
	check.Dir = root
	// The exit status is captured and reported rather than asserted on: a
	// configuration that is invalid for some other reason is a different finding,
	// made by the dry run below, and failing here would report it twice under the
	// wrong name. What is read is the path the tool says it resolved.
	out, checkErr := check.CombinedOutput()
	// The answer is parsed with the colours stripped, not asked for uncoloured.
	// GoReleaser decorates this line on CI runners and leaves it plain under a
	// local `go test`, and it does NOT honour NO_COLOR when it has decided the
	// environment renders ANSI (a CI run is exactly that environment) — the
	// escape codes land between `path` and `=`, so the regex found the line on
	// one machine and missed the identical line on the other. Asking the tool
	// nicely is a model of its TTY heuristics; deleting the codes is not.
	match := reGoreleaserConfigPath.FindStringSubmatch(reANSIEscape.ReplaceAllString(string(out), ""))
	if match == nil {
		t.Fatalf("`goreleaser check`, run from the repository root with no `--config`, did not report which configuration path it resolved (exit: %v), so the search order above is a model with nothing holding it. Its output was:\n%s", checkErr, out)
	}
	if got := filepath.ToSlash(filepath.Clean(match[1])); got != goreleaserConfigFile {
		t.Fatalf("`goreleaser check`, run the way release.yml runs the release — from the repository root, with no `--config` — reports it is using %q, where this test dry-runs %q.\n"+
			"The release would be built from a file nothing in this tree has read. Its output was:\n%s", got, goreleaserConfigFile, out)
	}
	t.Logf("goreleaser resolves its configuration to %s, which is the file this dry run builds from", goreleaserConfigFile)
}

// TestGoreleaserResolvesTheConfigurationThisGateReads is that proof on its own,
// so the failure arrives as a sentence about which file publishes rather than
// buried in a message about archives.
func TestGoreleaserResolvesTheConfigurationThisGateReads(t *testing.T) {
	tool := requireGoreleaser(t)
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}
	requireGoreleaserLoadsThisConfig(t, tool, root)
}

// releaseChecksumFile is the checksum artifact docs/RELEASING.md's artifact step
// downloads alongside the archive.
const releaseChecksumFile = "checksums.txt"

// releaseGOOS and releaseGOARCH are the platforms a release publishes an archive
// for, PINNED here rather than read back out of the configuration.
//
// Reading the matrix out of the config would make this test agree with a config
// that had dropped `windows`: it would look for four archives, find four, and
// pass. What it must say is that the release still publishes the six downloads
// docs/RELEASING.md tells a maintainer to expect. The configuration's own copy of
// this list is checked separately, against these same six, by
// cmd/dossierx/gate_release_stamp_test.go.
var (
	releaseGOOS   = []string{"linux", "darwin", "windows"}
	releaseGOARCH = []string{"amd64", "arm64"}
)

// releaseStamps is EVERY symbol the release build stamps, with what each stands
// for. All three, because the import-path no-op is PER SYMBOL: an `-X` aimed at
// `github.com/BarterX-Tech/dossierx/cmd/dossierx.commit` is accepted, recorded and
// applied to nothing, and the binary then falls back to `debug.ReadBuildInfo`,
// whose `vcs.revision` IS a forty-character sha and whose `vcs.time` IS an RFC
// 3339 timestamp. Every SHAPE check below is satisfied by that fallback, which is
// why each field is also compared against the flag that names it.
var releaseStamps = []struct{ symbol, stands string }{
	{"main.version", "the tag with its leading `v` stripped, which is what the site's `latestBinaryVersion` derives and what the rendered `dossierx version` transcript is judged against"},
	{"main.commit", "the full forty-character sha — the width docs/RELEASING.md contrasts with the seven the deleted site field carried"},
	{"main.date", "the BUILD's RFC 3339 timestamp, which is why the site's transcript may not depict a `date:` line beside a calendar day"},
}

// releaseArchiveName is the archive a release publishes for one goos/goarch pair,
// spelled the way `.goreleaser.yaml`'s `name_template` and docs/RELEASING.md's
// `gh release download --pattern 'dossierx_<os>_<arch>*'` both spell it. Written
// out rather than derived from the template, because the procedure tells a
// maintainer to type this name and the day the template stops producing it the
// procedure is wrong. That template is held against this spelling by
// gateRequireArchiveNaming in the root module.
func releaseArchiveName(goos, goarch string) string {
	if goos == "windows" {
		return "dossierx_" + goos + "_" + goarch + ".zip"
	}
	return "dossierx_" + goos + "_" + goarch + ".tar.gz"
}

// reLinkedSymbol reads one `-X <symbol>=` assignment back out of a built binary's
// recorded link flags — the line docs/RELEASING.md's artifact item tells a
// maintainer to read.
func linkedSymbol(settings, symbol string) (string, bool) {
	match := regexp.MustCompile(`-X\s+` + regexp.QuoteMeta(symbol) + `=(\S+?)["\s]`).FindStringSubmatch(settings)
	if match == nil {
		return "", false
	}
	return match[1], true
}

// reFullSHA is a forty-character lowercase hex sha, which is what GoReleaser's
// `{{.Commit}}` stamps.
var reFullSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)

// repoFile reads a repo-relative file, or fails: every caller reads the result as
// evidence, and an empty string would be read as evidence of absence.
func repoFile(t *testing.T, rel string) string {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	if len(raw) == 0 {
		t.Fatalf("%s is empty", rel)
	}
	return string(raw)
}

// runTool executes a command and returns its stdout, failing on any error: every
// caller reads the output as evidence.
func runTool(t *testing.T, name string, args ...string) string {
	t.Helper()
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		t.Fatalf("%s %s: %v", name, strings.Join(args, " "), err)
	}
	return string(out)
}

// requireGoreleaser resolves the tool, or fails.
//
// It never skips. A previous version of this check recorded an override saying
// the dry run was not implemented because installing GoReleaser inside `go test`
// would make the suite's green depend on a network fetch. The premise is right —
// a check whose prerequisite is fetched at run time reports "we could not check"
// as a pass the day the fetch fails — and the conclusion does not follow from it.
// This module had already solved exactly this: it does not fetch a browser, it
// requires one, fails when it is not named, and CI supplies it as a pinned job
// dependency. Nothing is fetched here either.
func requireGoreleaser(t *testing.T) string {
	t.Helper()
	path := os.Getenv(goreleaserEnv)
	if path == "" {
		t.Fatalf("%s is unset, so the release build has not been run and this gate cannot say whether it produces the six archives, the checksum file, or a stamped binary. "+
			"It FAILS rather than skips: a skipped check is indistinguishable from a pass over zero assertions (harness_test.go:47). Point it at a `goreleaser` binary — "+
			"`go install github.com/goreleaser/goreleaser/v2@latest` puts one in $(go env GOPATH)/bin, which is where it already is on a machine that has ever released. Nothing here fetches it, on purpose", goreleaserEnv)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("%s=%q cannot be used: %v", goreleaserEnv, path, err)
	}
	t.Logf("release dry run driving: %s", path)
	return path
}

// TestGoreleaserSnapshotBuildsSixArchivesAndStampsTheBinary RUNS the release
// build, which is the one thing nothing else in this tree does.
//
// WHAT IT ASSERTS, and each is a sentence in docs/RELEASING.md that had no
// executable form until it was written:
//
//	six archives, one per declared goos/goarch pair, named the way the
//	  procedure's `gh release download` pattern spells them
//	checksums.txt beside them, listing all six
//	the host platform's snapshot binary reporting a stamped version, a
//	  forty-character commit and an RFC 3339 date — and reporting the SAME
//	  version its own recorded `-ldflags` line names, which is what tells a
//	  stamped build from one whose flags were accepted and discarded
//
// NOTHING IS WRITTEN INTO THE TREE. `dist` is redirected to a temp directory by
// appending one key to a copy of the real configuration; the copy is otherwise
// byte-for-byte the tree's, so the build under test is the release's build.
// GoReleaser's `before` hook still runs `go mod tidy` in the repository root,
// which is deliberate — it is part of the release build — and is a no-op here
// because the `tidy` job in CI fails on any diff it would produce.
func TestGoreleaserSnapshotBuildsSixArchivesAndStampsTheBinary(t *testing.T) {
	tool := requireGoreleaser(t)
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}

	// WHICH FILE, before what is in it. This test passes `--config` — it has to,
	// because redirecting `dist` into a temp directory is the only way to run the
	// release build without writing into the tree — and a `--config` pointed at a
	// file the release would not load turns the strongest check in this repository
	// into a green build of a document nobody publishes.
	requireGoreleaserLoadsThisConfig(t, tool, root)

	// The real configuration plus one key. Single-quoted and slash-separated so a
	// Windows temp path is a valid YAML scalar rather than a string full of
	// escapes.
	dist := filepath.Join(t.TempDir(), "dist")
	config := filepath.Join(t.TempDir(), "goreleaser-snapshot.yaml")
	redirected := repoFile(t, goreleaserConfigFile) + "\n\n# Appended by " + t.Name() + ": build into a temp directory, touch nothing else.\ndist: '" + filepath.ToSlash(dist) + "'\n"
	if err := os.WriteFile(config, []byte(redirected), 0o644); err != nil {
		t.Fatalf("write the snapshot configuration: %v", err)
	}

	run := exec.Command(tool, "release", "--snapshot", "--clean", "--config", config)
	run.Dir = root
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("`goreleaser release --snapshot --clean` failed: %v\n%s\n"+
			"This is the release build. A failure here is a release that would not have built, found before the tag instead of after it", err, out)
	}

	// The archives, counted rather than sampled.
	var missing []string
	for _, goos := range releaseGOOS {
		for _, goarch := range releaseGOARCH {
			name := releaseArchiveName(goos, goarch)
			if _, err := os.Stat(filepath.Join(dist, name)); err != nil {
				missing = append(missing, name)
			}
		}
	}
	if len(missing) > 0 {
		// What the build DID produce, in the message. A list of what is absent
		// sends a reader to look; a list of what is present tells them whether the
		// name changed or the platform went away, which are different edits.
		listed, globErr := filepath.Glob(filepath.Join(dist, "*"))
		if globErr != nil {
			listed = []string{"(could not list dist: " + globErr.Error() + ")"}
		}
		t.Errorf("the release build produced no %v. docs/RELEASING.md's verification step tells a maintainer the release page lists all six archives and to download one by that exact name; a name the build does not produce is a download nobody gets. dist holds:\n%s",
			missing, strings.Join(listed, "\n"))
	}

	// The checksum file, and its contents — an empty or partial checksums.txt is
	// present, downloadable, and verifies nothing.
	sumBytes, err := os.ReadFile(filepath.Join(dist, releaseChecksumFile))
	if err != nil {
		t.Fatalf("read %s out of the release build: %v", releaseChecksumFile, err)
	}
	sums := string(sumBytes)
	for _, goos := range releaseGOOS {
		for _, goarch := range releaseGOARCH {
			if name := releaseArchiveName(goos, goarch); !strings.Contains(sums, name) {
				t.Errorf("%s does not list %s. The procedure's artifact step verifies the download against this file, so an archive missing from it is an archive nobody can check:\n%s", releaseChecksumFile, name, sums)
			}
		}
	}

	// The host platform's binary, which is the only one this machine can run.
	// GoReleaser puts each build in its own directory whose suffix carries the
	// microarchitecture level (`_v1`, `_v8.0`), so the directory is matched rather
	// than spelled.
	name := "dossierx"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	matches, err := filepath.Glob(filepath.Join(dist, "dossierx_"+runtime.GOOS+"_"+runtime.GOARCH+"*", name))
	if err != nil || len(matches) != 1 {
		t.Fatalf("the release build produced %d binaries for this host (%s/%s), not 1 (%v). Without exactly one there is nothing to ask for its version, and a gate that cannot run the artifact certifies the configuration instead of the release",
			len(matches), runtime.GOOS, runtime.GOARCH, err)
	}
	binary := matches[0]

	var envelope struct {
		Data struct {
			Version string `json:"version"`
			Commit  string `json:"commit"`
			Date    string `json:"date"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(runTool(t, binary, "version", "--format", "json")), &envelope); err != nil {
		t.Fatalf("decode the snapshot binary's version envelope: %v", err)
	}

	// The fallbacks, by name. Each is what this binary prints when the stamping
	// did not reach it, and each is a plausible-looking string.
	switch envelope.Data.Version {
	case "", "dev", "(devel)":
		t.Errorf("the snapshot binary reports version %q — resolveVersionInfo's no-ldflags fallback. The build ran, the archives exist, and nothing was stamped into them: this is the `-X` that the linker accepted and applied to nothing", envelope.Data.Version)
	}
	if !reFullSHA.MatchString(envelope.Data.Commit) {
		t.Errorf("the snapshot binary reports commit %q, which is not the forty-character sha GoReleaser's `{{.Commit}}` stamps. docs/RELEASING.md rests half its argument for deleting the site's `commit` field on that width", envelope.Data.Commit)
	}
	if _, err := time.Parse(time.RFC3339, envelope.Data.Date); err != nil {
		t.Errorf("the snapshot binary reports date %q, which is not the RFC 3339 timestamp GoReleaser's `{{.Date}}` stamps: %v. The site depicts a calendar day and the binary does not, which is why the transcript may not carry a `date:` line", envelope.Data.Date, err)
	}

	// And the assertion the others cannot make: what the binary PRINTS is what its
	// own link flags NAME. A build whose `-X` was aimed at the import path records
	// the flag and reports something else — the two readings agree only when the
	// stamping actually landed.
	settings := runTool(t, "go", "version", "-m", binary)
	reported := map[string]string{
		"main.version": envelope.Data.Version,
		"main.commit":  envelope.Data.Commit,
		"main.date":    envelope.Data.Date,
	}
	for _, stamp := range releaseStamps {
		linked, ok := linkedSymbol(settings, stamp.symbol)
		if !ok {
			t.Errorf("`go version -m` on the snapshot binary records no `-X %s=` in its link flags, while the binary reports %q for it. The flag was aimed somewhere else — the import-path spelling is the way that happens — so the value it reports came from `debug.ReadBuildInfo`, not from the release build. "+
				"docs/RELEASING.md's artifact item reads exactly this line:\n%s", stamp.symbol, reported[stamp.symbol], settings)
			continue
		}
		if linked != reported[stamp.symbol] {
			t.Errorf("the snapshot binary was linked with `-X %s=%s` and reports %q. The flag was recorded and did not reach the variable it names — the `main.` prefix failure `.goreleaser.yaml` warns about, and the one failure no reading of the version envelope alone can distinguish from success. "+
				"That symbol must carry %s", stamp.symbol, linked, reported[stamp.symbol], stamp.stands)
		}
	}
}
