package viewertests

// THE RELEASE BUILD, DRY-RUN, AND WHY IT LIVES IN THIS MODULE.
//
// This file was the second half of site_toolchain_test.go. The first half
// compared the npm steps and build environment that produced the site against
// the ones .github/workflows/deploy-site.yml publishes it with, step by step and
// in both directions — and that half is gone, because site/ is no longer built.
// It is two static HTML pages and a stylesheet, deploy-site.yml uploads the
// directory as it stands, and tests/ci_workflow_test.go's
// TestThePublishWorkflowUploadsTheTreeWithoutBuildingIt refuses the
// reintroduction of any build step. The invariant that the gate reads the same
// bytes a visitor is served is now true by construction rather than by
// comparison.
//
// What follows never had anything to do with the site. It was in that file
// because it shares this module's one property — a check with an external
// prerequisite can be strict here without holding the engine's build hostage —
// and the header below is the original explanation, unchanged.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

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
	{"main.version", "the tag as tagged — `{{.Tag}}` keeps the leading `v`, so it is the same string the site's `latestVersion` renders and the rendered `dossierx version` transcript is judged against"},
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
