// gate_archives_test.go is D7: the archives a release ACTUALLY published, read
// the way the person who downloads them reads them.
//
// WHERE IT SITS, and why that is the whole design constraint. D7 runs between
// the two irreversible acts — after the tag is pushed (D6) and before main is
// (D8). Everything before it is about a tree on a maintainer's disk; this is the
// first step whose subject is a file on a public forge that strangers are
// already fetching. If it says PASS wrongly, D8 announces a release whose
// artifacts nobody checked. If it cannot run at all, the release stops with the
// tag published and main not, which is a state a human has to finish or unwind
// by hand — and stopping there is CORRECT. `gateDriverUnwired.Archives` is that
// refusal today; this file is the check that replaces it.
//
// THE FOUR QUESTIONS IT ASKS, and none of them is implied by another:
//
//  1. IS THERE A CHECKSUM FILE AT ALL. A release with no `checksums.txt` is
//     UNCHECKABLE, never "no mismatches found". Every comparison below is
//     against a line in that file, so a missing one turns this whole check into
//     a loop over zero assertions that exits clean — the exact shape CLAUDE.md
//     forbids ("indistinguishable from a pass over zero assertions").
//
//  2. IS EVERY ARCHIVE THE RELEASE IS SUPPOSED TO CONTAIN PRESENT. The expected
//     set is DERIVED from `.goreleaser.yaml` as of the released commit — the
//     build matrix, the archive block's `name_template`, and the format
//     overrides — and never hard-coded. A hard-coded list of six names describes
//     a release shape the configuration no longer has the day a seventh target
//     is added: the seventh archive would be published, downloaded and verified
//     by nobody, while this check counted six and passed. Deriving it means the
//     day the matrix grows, this check grows with it or fails saying it cannot
//     read the matrix.
//
//  3. DO THE BYTES MATCH THE CLAIM, IN BOTH DIRECTIONS. Each archive's sha256 is
//     compared against its line, AND every line is compared against an archive
//     that was checked. The second direction is the one that is easy to leave
//     out and is not redundant: a line naming a file this check never looked at
//     is a published artifact nobody verified, and a one-directional check
//     reports "all six matched" beside it.
//
//  4. IS THE BINARY INSIDE THE RELEASE'S BINARY. The host platform's archive is
//     extracted, the binary is RUN, and what it reports about itself is compared
//     with the release being published. This is the only question here that
//     cannot be answered from metadata: an archive can carry a correct name, a
//     correct sha256 and a stale binary, and every other check in this file
//     passes over it. The stamp is read exactly the way
//     viewer-tests/site_toolchain_test.go's snapshot dry run reads it —
//     `<binary> version --format json`, decoded out of the `data` envelope —
//     rather than by inventing a second way to ask one question.
//
// THE SEAM, AND WHY THERE IS EXACTLY ONE. gateArchivesSource is the only place
// this file touches the network or the repository. Everything else is
// arithmetic over a directory of files, which is what makes the refusals below
// constructible offline: a test hands the seam a fixture directory of real
// archives in GoReleaser's layout and the same code path runs over it. The
// directory's LIFETIME belongs to the seam and is never cleaned up here — a
// production run leaves the downloaded artifacts for the operator to look at,
// and deleting a directory a test handed in would delete the test's own
// fixture.
//
// WHY IT WAITS, AND WHY IT MUST NEVER ASK TO BE INVOKED AGAIN. D7's subject is
// a file the Release workflow is still uploading: the tag push at D6 is what
// STARTS that workflow, so the ordinary state of the forge one second after D6
// is "the tag is there and the assets are not". A version of this check that
// refused on the first empty answer and told the operator to wait and try again
// would strand EVERY release, because there is no second try: once the tag is
// public, refuseIfAlreadyPublished (gate_driver_test.go) refuses every later
// invocation rather than resuming a half-done release, and rightly so. So the
// waiting has to happen INSIDE this step, and it does — gateArchivesForge.Download
// polls until the assets are there or gateArchivesPollBudget is spent. The
// budget is bounded, which means the refusal on timeout is real: it stops the
// release with the tag published and main not, and it says so in those words
// rather than proposing a recovery the driver cannot perform.
//
// THE TWO SENTINELS, kept apart for the reason errGateTreeMismatch and
// errGateUncheckable are (gate_receipt_test.go:445). errGateUncheckable accuses
// the READING — `gh` is not installed, the forge is unreachable, the tag is not
// there, the configuration cannot be parsed — and the recovery is to supply what
// was missing and ask again. errGateArchivesWrong accuses the RELEASE — the
// bytes were fetched, they were read, and they are not what this tree says the
// release publishes — and the recovery is a human's, because the tag is already
// out. Collapsing them would send an operator to install a tool when what
// actually happened is that the published binary reports the wrong commit.
//
// WHAT THIS FILE DOES NOT ESTABLISH, stated here rather than discovered later:
//
//   - IT AUTHENTICATES NOTHING. `checksums.txt` is fetched from the same release
//     as the archives, so an attacker who can replace one can replace both. What
//     it closes is the failure that actually happens — a build that published a
//     truncated, stale or mis-stamped artifact — not a signed-supply-chain
//     claim. Signing is a different mechanism and is not pretended at here.
//   - IT RUNS ONE BINARY, NOT SIX. Only the host platform's archive can be
//     executed on this machine. The other five are checked for presence and for
//     bytes; that their binaries are stamped correctly rests on their having
//     been produced by the same build, which is what
//     TestGoreleaserSnapshotBuildsSixArchivesAndStampsTheBinary exercises before
//     the tag rather than after it.
//   - THE EXPECTED NAMES ARE A MODEL OF ANOTHER PROGRAM. GoReleaser decides the
//     archive names; this file renders the same templates with the same
//     variables. Where the model is wrong the failure is LOUD — the expected
//     name is not among the assets and the check refuses naming it — rather than
//     quiet, which is the direction a model is allowed to be wrong in.
package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
	"text/template"
	"time"

	"gopkg.in/yaml.v3"
)

// errGateArchivesWrong is the ANSWER "no": the release was fetched, it was read,
// and what it publishes is not what the tree it was built from says it
// publishes.
//
// It is a second sentinel rather than a second spelling of errGateUncheckable
// because the two are different accusations with different recoveries, which is
// the same argument gate_receipt_test.go makes for errGateTreeMismatch and
// gate_driver_test.go makes for errGateVersionMismatch. UNCHECKABLE says the
// question could not be asked and points at a tool, a network or a document.
// This one says the question WAS asked and the answer is no — and by the time it
// can be raised the tag is already published, so there is no "fix the input and
// re-run": what it demands is a human deciding what to do about a release that
// is already out.
var errGateArchivesWrong = errors.New("the published archives are not the release this tree declares")

// ---------------------------------------------------------------------
// the seam: the only place the network and the repository appear
// ---------------------------------------------------------------------

// gateArchivesEvidence is everything this check must be TOLD rather than
// compute. It is the same shape, and exists for the same reason, as
// gateDriverEvidence one level up: the boundary is one interface so that a
// reader can see the whole of it in one place, and so that the offline half —
// which is all the arithmetic and every refusal — can be exercised without a
// forge.
type gateArchivesEvidence interface {
	// ReleaseConfig is `.goreleaser.yaml` AS OF the released commit, not as of
	// the working copy. The archives were built by the configuration the tag
	// carries; reading the working copy would derive an expected set from a file
	// that may have been edited after the build, and the mismatch would be
	// reported against the release rather than against the edit.
	ReleaseConfig(commit string) ([]byte, error)

	// Download places every asset published under version into a directory and
	// returns it. The directory belongs to the implementation — nothing here
	// deletes it — because in production it is the operator's copy of the
	// artifacts and in a test it is the fixture the test owns.
	//
	// An implementation that reaches a forge WAITS: at the moment D7 runs, the
	// workflow the tag push started is still uploading, so "not there yet" is
	// the ordinary first answer and not a verdict. How long it waits is that
	// implementation's business — the fixture below has its directory already
	// and returns at once — but no implementation returns an error meaning
	// "ask again later", because nothing will ask again (see the file header).
	Download(repo, version string) (dir string, err error)
}

// gateArchivesSource is the evidence the real run uses. It is a package-level
// var so that a test can substitute a fixture without a forge; production never
// assigns it.
var gateArchivesSource gateArchivesEvidence = gateArchivesForge{}

// gateArchivesForgeCLI is the tool this check drives, and it is `gh` because
// that is the tool docs/RELEASING.md already tells a maintainer to verify a
// release with (`gh release download vX.Y.Z --repo …`). A second downloader here
// would be a second procedure, and the release procedure is a file of which
// there is exactly one.
const gateArchivesForgeCLI = "gh"

// gateArchivesPollBudget is how long Download waits for the Release workflow to
// finish publishing the assets before it gives up.
//
// It is 20 minutes because of the headroom gateDriverTimeoutFloor documents: the
// release-time invocation must carry at least a 30-minute `-timeout`, and D7 has
// to WAIT for the workflow and then verify what it produced inside that one
// budget. Spending the whole floor on the wait would leave the download, the six
// checksums and the extract-and-run with nothing, and the way a `go test` budget
// runs out is a panic — a goroutine dump instead of the per-step statement of
// what is already published. So the wait takes two thirds and leaves the third
// that does the actual checking. The number is a bound, not an estimate: a
// workflow that finishes in four minutes costs four minutes here.
const gateArchivesPollBudget = 20 * time.Minute

// gateArchivesPollInterval is how often it asks inside that budget. Fifteen
// seconds is short enough that the wait costs little beyond the workflow's own
// run time and long enough that a twenty-minute wait is 81 API calls rather than
// a rate-limit incident during a release.
const gateArchivesPollInterval = 15 * time.Second

// gateArchivesForge is the production evidence source.
//
// Every field is a SEAM WITH A PRODUCTION DEFAULT, and they exist so that the
// timeout refusal below is constructible: it is a refusal that in production
// takes twenty minutes of a real forge saying no, and a refusal no test can
// reach is the sentence CLAUDE.md refuses to count as a check. A zero value —
// which is what production assigns to gateArchivesSource — means `gh`, the real
// clock and the real budget, so nothing about the released path is decided by
// these.
type gateArchivesForge struct {
	// fetch performs ONE download attempt into dir. Nil means drive `gh`.
	fetch func(dir string) error

	pollBudget   time.Duration // zero means gateArchivesPollBudget
	pollInterval time.Duration // zero means gateArchivesPollInterval
	now          func() time.Time
	sleep        func(time.Duration)
}

// gateArchivesRepoRoot is the repository this check reads the configuration out
// of, resolved from this package's directory the same way surfaceRepoRoot does
// — this is test code in cmd/dossierx (see gate_driver_test.go's "WHY IT IS TEST
// CODE"), so the working directory under `go test` is the package directory.
//
// It is checked rather than assumed: a root with no `.git` would send every
// `git show` below into whatever repository happens to contain the working
// directory, and the configuration it returned would describe some other
// project's release.
func gateArchivesRepoRoot() (string, error) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		return "", fmt.Errorf("the repository root could not be resolved from this package's directory: %w", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return "", fmt.Errorf("%s does not look like a git repository (%w), so `git show <commit>:%s` there would read some other tree's release configuration or nothing at all",
			root, err, gateGoreleaserFile)
	}
	return root, nil
}

func (gateArchivesForge) ReleaseConfig(commit string) ([]byte, error) {
	root, err := gateArchivesRepoRoot()
	if err != nil {
		return nil, err
	}
	// `git show <commit>:<path>` and never the working copy, for the reason
	// gateDriverTreeFile gives about the changelog: an uncommitted edit to the
	// release configuration is exactly the shape of a maintainer who changed the
	// build matrix and has not committed it, and it would clear this check while
	// being absent from the tag the archives were built from.
	body, err := gateGit(root, "show", commit+":"+gateGoreleaserFile)
	if err != nil {
		return nil, fmt.Errorf("`git show %s:%s` failed in %s: %w. That object has to be present in this clone — fetch it (`git fetch --all`) and re-run",
			commit, gateGoreleaserFile, root, err)
	}
	return []byte(body), nil
}

// Download fetches every asset the release published, WAITING for them to
// appear.
//
// The waiting is the whole shape of this function and it is forced by where D7
// sits. D6 pushed the tag, which is what starts the Release workflow; the
// workflow then builds six GOOS/GOARCH archives and uploads them. So the first
// answer this function gets is normally "no such release" or "no assets", and
// that answer means the build is in progress rather than that the release is
// wrong. The reason it cannot simply report that and let somebody try later is
// that nobody can: refuseIfAlreadyPublished refuses every invocation made after
// the tag is public, deliberately, because resuming would mean trusting steps
// this process never performed. A single-attempt Download therefore does not
// fail occasionally — it fails on every release there has ever been, and leaves
// each one with the tag out, main behind it, and a human finishing it by hand.
//
// The budget is bounded, so the refusal at the end of it is a real refusal and
// not a promise: it states how long was waited and how often it asked, quotes
// what the last attempt actually said, and does not propose an action this
// driver is capable of taking again.
func (f gateArchivesForge) Download(repo, version string) (string, error) {
	fetch := f.fetch
	if fetch == nil {
		tool, err := exec.LookPath(gateArchivesForgeCLI)
		if err != nil {
			// Not polled: `gh` will not install itself while this waits. Only
			// the forge's answer changes with time, so only it is worth asking
			// twice.
			return "", fmt.Errorf("`%s` is not on PATH (%v), so the published assets cannot be fetched and this check cannot run. "+
				"It FAILS rather than skips: a release whose archives nobody downloaded is exactly the unchecked publish D7 exists to stop. "+
				"Install the GitHub CLI (https://cli.github.com) and authenticate it with `%s auth login` before authorizing a release",
				gateArchivesForgeCLI, err, gateArchivesForgeCLI)
		}
		fetch = func(dir string) error {
			// No `--pattern`: every asset the release carries is fetched,
			// because the second direction of the checksum comparison below is
			// about lines naming files this check never looked at, and a
			// pattern would decide in advance which files those are.
			//
			// `--clobber`: an attempt that ran while the workflow was still
			// uploading can leave a partial file behind, and without this the
			// next attempt would fail on "already exists" — a poll that
			// poisons its own directory reports a timeout whose cause is
			// itself.
			cmd := exec.Command(tool, "release", "download", version, "--repo", repo, "--dir", dir, "--clobber")
			out, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("`%s release download %s --repo %s` failed: %w\n%s",
					gateArchivesForgeCLI, version, repo, err, strings.TrimSpace(string(out)))
			}
			return nil
		}
	}

	budget, interval := f.pollBudget, f.pollInterval
	if budget <= 0 {
		budget = gateArchivesPollBudget
	}
	if interval <= 0 {
		interval = gateArchivesPollInterval
	}
	now, sleep := f.now, f.sleep
	if now == nil {
		now = time.Now
	}
	if sleep == nil {
		sleep = time.Sleep
	}

	dir, err := os.MkdirTemp("", "dossierx-release-assets-")
	if err != nil {
		return "", fmt.Errorf("a directory to download %s's assets into could not be created: %w", version, err)
	}

	deadline := now().Add(budget)
	for attempts := 1; ; attempts++ {
		err := fetch(dir)
		if err == nil {
			return dir, nil
		}
		if !now().Before(deadline) {
			// Every failure is waited through rather than only the ones that
			// look like "not published yet": telling a workflow that is still
			// running apart from one that failed means reading `gh`'s prose,
			// and a misread there is a release refused for being slow. What the
			// operator gets instead is the last attempt's own words, which say
			// which of the two it was.
			return "", fmt.Errorf("the assets for %s on %s did not appear within %s of asking every %s (%d attempts). The last attempt said:\n%s\n"+
				"This is where the release stops, with the tag published and the commit it names not yet on %s: either the Release workflow for %s is still building — in which case the archives will arrive after this driver has gone, and nobody will have checked them — or it failed and they never will. "+
				"Read that workflow run and decide; this driver cannot pick the release up again, because once the tag is public it refuses every later invocation rather than resuming a release whose earlier steps it did not perform",
				version, repo, budget, interval, attempts, err, gateBaseRef, version)
		}
		sleep(interval)
	}
}

// ---------------------------------------------------------------------
// what the release is supposed to contain, derived from the configuration
// ---------------------------------------------------------------------

// gateArchivesWanted is one archive a release publishes.
type gateArchivesWanted struct {
	Name   string
	GOOS   string
	GOARCH string
	Format string
}

// gateArchivesRelease is the whole expected shape of a published release: which
// repository carries it, what the checksum file is called, what the binary
// inside an archive is called, and every archive there should be.
type gateArchivesRelease struct {
	Repo      string
	Checksums string
	Binary    string
	Wanted    []gateArchivesWanted
}

// gateArchivesConfig is the part of `.goreleaser.yaml` that decides what a
// release contains.
//
// It is a SEPARATE model from gateGoreleaserConfig in gate_release_stamp_test.go
// on purpose, and the difference is what each is for. That one is a strict
// contract — it refuses anything but one build and one archive, because it is
// asserting that this repository's configuration still says what the procedure
// says it says. This one has to DERIVE a file list from whatever the
// configuration declares, so it reads the fields that change the answer and that
// the other model has no reason to carry: `project_name`, a build's `ignore`
// list and `binary` name, `format`/`formats` in both their spellings, and the
// `release.github` block that names the repository to download from.
type gateArchivesConfig struct {
	ProjectName string `yaml:"project_name"`

	Builds []struct {
		ID     string   `yaml:"id"`
		Binary string   `yaml:"binary"`
		GOOS   []string `yaml:"goos"`
		GOARCH []string `yaml:"goarch"`
		Ignore []struct {
			GOOS   string `yaml:"goos"`
			GOARCH string `yaml:"goarch"`
		} `yaml:"ignore"`
	} `yaml:"builds"`

	Archives []struct {
		ID              string   `yaml:"id"`
		IDs             []string `yaml:"ids"`
		NameTemplate    string   `yaml:"name_template"`
		Format          string   `yaml:"format"`
		Formats         []string `yaml:"formats"`
		FormatOverrides []struct {
			GOOS    string   `yaml:"goos"`
			Format  string   `yaml:"format"`
			Formats []string `yaml:"formats"`
		} `yaml:"format_overrides"`
	} `yaml:"archives"`

	Checksum struct {
		NameTemplate string `yaml:"name_template"`
	} `yaml:"checksum"`

	Release struct {
		GitHub struct {
			Owner string `yaml:"owner"`
			Name  string `yaml:"name"`
		} `yaml:"github"`
	} `yaml:"release"`
}

// gateArchivesDefaultFormat is GoReleaser's own default when an archive block
// declares no format, and it is spelled out here because a missing key is the
// ordinary case (this repository's configuration declares none) and treating
// "absent" as "unreadable" would refuse every correct release.
const gateArchivesDefaultFormat = "tar.gz"

// gateArchivesExtension maps an archive format to the suffix the published file
// carries, and it is deliberately SHORT.
//
// Only formats this check can both NAME and OPEN are listed. A release that
// starts publishing `tar.xz` is then uncheckable rather than mis-named: the
// alternative — guessing the extension for a format whose bytes this file cannot
// read — would find the file, verify its sha256, and then be unable to answer
// question 4 about it, which is a partial check reported as a whole one.
var gateArchivesExtension = map[string]string{
	"tar.gz": ".tar.gz",
	"tgz":    ".tgz",
	"tar":    ".tar",
	"zip":    ".zip",
}

// gateArchivesReadableFormats is that map's key set, sorted, for the refusals
// that have to say what this check can read.
func gateArchivesReadableFormats() []string {
	out := make([]string, 0, len(gateArchivesExtension))
	for format := range gateArchivesExtension {
		out = append(out, format)
	}
	sort.Strings(out)
	return out
}

// gateArchivesRender renders one GoReleaser name template.
//
// `missingkey=error` is the whole reason this is a function rather than a
// fmt.Sprintf. text/template's default over a map is to render a missing field
// as `<no value>`, so a template naming a variable this file does not model
// would produce a plausible-looking archive name that no release has ever
// published — and the refusal would be "that archive is missing" rather than
// "this check cannot read that template". The first sends an operator to look at
// the forge; only the second is true.
func gateArchivesRender(what, tmpl string, data map[string]string) (string, error) {
	if strings.TrimSpace(tmpl) == "" {
		return "", fmt.Errorf("%w: %s declares no %s, so there is no name to look for among the published assets",
			errGateUncheckable, gateGoreleaserFile, what)
	}
	parsed, err := template.New(what).Option("missingkey=error").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("%w: %s's %s (%q) is not a template this check can parse: %w",
			errGateUncheckable, gateGoreleaserFile, what, tmpl, err)
	}
	var rendered strings.Builder
	if err := parsed.Execute(&rendered, data); err != nil {
		return "", fmt.Errorf("%w: %s's %s (%q) names a value this check does not model (%v). "+
			"It refuses rather than rendering the field empty, because an empty field produces a well-formed name that no release publishes and the failure would read as a missing archive",
			errGateUncheckable, gateGoreleaserFile, what, tmpl, err)
	}
	name := strings.TrimSpace(rendered.String())
	if name == "" || strings.ContainsAny(name, " \t\n") {
		return "", fmt.Errorf("%w: %s's %s renders to %q, which is not a single file name. Every comparison below matches an asset by name, and a name carrying whitespace cannot be matched against `checksums.txt`'s `<sha256>  <file>` lines",
			errGateUncheckable, gateGoreleaserFile, what, name)
	}
	return name, nil
}

// gateArchivesOneFormat resolves the singular `format:` and the plural
// `formats:` spellings — GoReleaser v2 accepts both — down to the ONE format a
// target is packaged in.
//
// More than one is refused rather than modelled. A `formats: [tar.gz, zip]`
// publishes TWO files per target, so the expected set this file builds would
// name half of what the release contains and the "every line was checked"
// direction would report the other half as extra. Refusing says which edit
// happened; guessing produces a report about the wrong release.
func gateArchivesOneFormat(where, single string, plural []string, fallback string) (string, error) {
	switch {
	case single != "" && len(plural) > 0:
		return "", fmt.Errorf("%w: %s declares both `format: %s` and `formats: %v` for %s. Which one packages the release is then a question about GoReleaser's precedence rather than about this file, and the expected asset names differ between the two answers",
			errGateUncheckable, gateGoreleaserFile, single, plural, where)
	case len(plural) > 1:
		return "", fmt.Errorf("%w: %s declares %d formats (%v) for %s, so each target publishes %d files. This check derives ONE expected name per target, so it would name a fraction of what the release contains and report the rest as lines nobody checked",
			errGateUncheckable, gateGoreleaserFile, len(plural), plural, where, len(plural))
	case len(plural) == 1:
		return plural[0], nil
	case single != "":
		return single, nil
	case fallback != "":
		return fallback, nil
	}
	return "", fmt.Errorf("%w: %s declares no archive format for %s, so this check cannot say what the published file is called",
		errGateUncheckable, gateGoreleaserFile, where)
}

// gateArchivesDerive turns the release configuration into the set of files the
// release is supposed to carry.
//
// Every refusal here is errGateUncheckable, and that is the classification the
// whole function is about: a configuration this file cannot read is a check that
// did not happen. The tempting alternative — fall back to the six names this
// repository publishes today — is precisely the hard-coded list the derivation
// exists to remove, and it would be at its most confident on the day the matrix
// changed.
func gateArchivesDerive(raw []byte, version string) (gateArchivesRelease, error) {
	var release gateArchivesRelease

	var config gateArchivesConfig
	if err := yaml.Unmarshal(raw, &config); err != nil {
		return release, fmt.Errorf("%w: %s does not parse as YAML, so what the release is supposed to contain cannot be read from it: %w",
			errGateUncheckable, gateGoreleaserFile, err)
	}
	if len(config.Builds) == 0 {
		return release, fmt.Errorf("%w: %s declares no builds, so it names no platform the release publishes an archive for. An empty expected set makes every comparison below true over nothing",
			errGateUncheckable, gateGoreleaserFile)
	}
	if len(config.Archives) != 1 {
		return release, fmt.Errorf("%w: %s declares %d archive blocks, not 1. Each block publishes its own set of files under its own `name_template`, and this check derives one set — so with a second block it would verify one set and report the other as lines nobody checked",
			errGateUncheckable, gateGoreleaserFile, len(config.Archives))
	}
	archive := config.Archives[0]

	// The repository to download from, which is the one fact here that is about
	// WHERE the release lives rather than what is in it.
	owner, repoName := strings.TrimSpace(config.Release.GitHub.Owner), strings.TrimSpace(config.Release.GitHub.Name)
	if owner == "" || repoName == "" {
		return release, fmt.Errorf("%w: %s's `release.github` block does not name both an owner and a repository (owner=%q, name=%q), so there is no forge to ask for the published assets",
			errGateUncheckable, gateGoreleaserFile, owner, repoName)
	}
	release.Repo = owner + "/" + repoName

	// GoReleaser defaults `project_name` from the git remote's repository name.
	// This file takes the declared `release.github.name` instead of modelling
	// that lookup, because it is the same string in every configuration that has
	// a `release` block and it is one this file has already read. Where the two
	// ever differ, the expected names differ from the published ones and the
	// refusal below names the archive it could not find — which is the direction
	// a model is allowed to be wrong in. Declaring `project_name:` explicitly
	// removes the question.
	project := strings.TrimSpace(config.ProjectName)
	if project == "" {
		project = repoName
	}

	checksums, err := gateArchivesRender("checksum name_template", config.Checksum.NameTemplate, map[string]string{
		"ProjectName": project,
		"Version":     gateDriverNormalizeVersion(version),
		"Tag":         version,
	})
	if err != nil {
		return release, err
	}
	release.Checksums = checksums

	format, err := gateArchivesOneFormat("the archives block", archive.Format, archive.Formats, gateArchivesDefaultFormat)
	if err != nil {
		return release, err
	}
	overrides := map[string]string{}
	for _, override := range archive.FormatOverrides {
		// No fallback for an override: an override that declares no format
		// overrides nothing, and silently treating it as the default would make
		// a deleted `format: zip` invisible — the Windows archive would then be
		// looked for as a `.tar.gz`, found missing, and reported as an absent
		// archive rather than as an unreadable configuration.
		value, err := gateArchivesOneFormat("the format override for goos "+override.GOOS, override.Format, override.Formats, "")
		if err != nil {
			return release, err
		}
		overrides[override.GOOS] = value
	}

	// Which builds this archive block packages. An empty `ids` means all of them,
	// which is GoReleaser's own rule.
	selected := config.Builds
	if len(archive.IDs) > 0 {
		selected = selected[:0:0]
		for _, want := range archive.IDs {
			for _, build := range config.Builds {
				if build.ID == want {
					selected = append(selected, build)
				}
			}
		}
		if len(selected) != len(archive.IDs) {
			return release, fmt.Errorf("%w: %s's archive block packages build ids %v, and the builds it declares are %v. An `ids` entry naming no declared build packages NOTHING — the release page still lists files, so no count of assets can tell — and this check would derive its expected set from the builds that do exist",
				errGateUncheckable, gateGoreleaserFile, archive.IDs, gateArchivesBuildIDs(config))
		}
	}

	binary := strings.TrimSpace(selected[0].Binary)
	if binary == "" {
		binary = project
	}
	for _, build := range selected {
		if name := strings.TrimSpace(build.Binary); name != "" && name != binary {
			return release, fmt.Errorf("%w: the builds this archive packages disagree about the binary's name (%q and %q). This check extracts ONE name out of the host platform's archive, so it would look for one of them and report the other's archive as holding no binary",
				errGateUncheckable, binary, name)
		}
	}
	release.Binary = binary

	seen := map[string]string{}
	for _, build := range selected {
		if len(build.GOOS) == 0 {
			return release, fmt.Errorf("%w: build %q in %s declares no `goos`, so it names no operating system the release publishes for and every archive derived from it would be missing from the expected set",
				errGateUncheckable, build.ID, gateGoreleaserFile)
		}
		if len(build.GOARCH) == 0 {
			return release, fmt.Errorf("%w: build %q in %s declares no `goarch`; the same argument as the `goos` list above",
				errGateUncheckable, build.ID, gateGoreleaserFile)
		}
		for _, goos := range build.GOOS {
			for _, goarch := range build.GOARCH {
				if gateArchivesIgnored(build.Ignore, goos, goarch) {
					continue
				}
				targetFormat := format
				if override, ok := overrides[goos]; ok {
					targetFormat = override
				}
				extension, ok := gateArchivesExtension[targetFormat]
				if !ok {
					return release, fmt.Errorf("%w: %s packages %s/%s as %q, which this check can neither name nor open. It reads %v. "+
						"Verifying the archive's sha256 while being unable to look inside it would answer three of D7's four questions and report all four",
						errGateUncheckable, gateGoreleaserFile, goos, goarch, targetFormat, gateArchivesReadableFormats())
				}
				stem, err := gateArchivesRender("archive name_template", archive.NameTemplate, map[string]string{
					"ProjectName": project,
					"Binary":      binary,
					"Os":          goos,
					"Arch":        goarch,
					"Arm":         "",
					"Amd64":       "",
					"Version":     gateDriverNormalizeVersion(version),
					"Tag":         version,
				})
				if err != nil {
					return release, err
				}
				name := stem + extension
				if previous, clash := seen[name]; clash {
					return release, fmt.Errorf("%w: %s names the archive for %s/%s and the one for %s the same file (%s). The release then publishes fewer files than it builds, and this check cannot say which target the surviving one came from",
						errGateUncheckable, gateGoreleaserFile, goos, goarch, previous, name)
				}
				seen[name] = goos + "/" + goarch
				release.Wanted = append(release.Wanted, gateArchivesWanted{Name: name, GOOS: goos, GOARCH: goarch, Format: targetFormat})
			}
		}
	}
	if len(release.Wanted) == 0 {
		return release, fmt.Errorf("%w: %s declares builds but every goos/goarch pair in them is ignored, so the expected set is empty. A check over an empty set finds no mismatches and says so, which is the pass over zero assertions this gate refuses",
			errGateUncheckable, gateGoreleaserFile)
	}
	sort.Slice(release.Wanted, func(i, j int) bool { return release.Wanted[i].Name < release.Wanted[j].Name })
	return release, nil
}

// gateArchivesIgnored reports whether a build excludes this goos/goarch pair.
func gateArchivesIgnored(ignore []struct {
	GOOS   string `yaml:"goos"`
	GOARCH string `yaml:"goarch"`
}, goos, goarch string) bool {
	for _, skip := range ignore {
		if (skip.GOOS == "" || skip.GOOS == goos) && (skip.GOARCH == "" || skip.GOARCH == goarch) {
			return true
		}
	}
	return false
}

// gateArchivesBuildIDs lists the build ids a configuration declares, so the
// refusal about an `ids` entry naming nothing can print both sides.
func gateArchivesBuildIDs(config gateArchivesConfig) []string {
	out := make([]string, 0, len(config.Builds))
	for _, build := range config.Builds {
		out = append(out, build.ID)
	}
	return out
}

// ---------------------------------------------------------------------
// the checksum file
// ---------------------------------------------------------------------

// gateArchivesSumRE is one sha256 as `sha256sum` and GoReleaser both write it.
var gateArchivesSumRE = regexp.MustCompile(`^[0-9a-f]{64}$`)

// gateArchivesReadChecksums reads the release's checksum file into name -> sum.
//
// Every failure here is uncheckable, and the empty-file case is the one worth
// naming: an empty `checksums.txt` is present, downloadable, and verifies
// nothing. A reader that returned an empty map for it would make the loop below
// pass over zero comparisons and report that every archive matched.
func gateArchivesReadChecksums(dir, name string) (map[string]string, error) {
	blob, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return nil, fmt.Errorf("%w: the release publishes no readable %s (%v). Every sha256 below is compared against a line in that file, so without it this check would examine zero archives and report no mismatches — which is a skip wearing a pass's clothes. "+
			"docs/RELEASING.md's artifact step downloads this file beside the archive for the same reason",
			errGateUncheckable, name, err)
	}
	sums := map[string]string{}
	for index, line := range strings.Split(string(blob), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 || !gateArchivesSumRE.MatchString(fields[0]) {
			return nil, fmt.Errorf("%w: %s line %d reads %q, which is not `<sha256>  <file>`. A line this check cannot read is an archive whose claim nobody compared, and skipping it would leave that archive verified by nothing while the file as a whole reported clean",
				errGateUncheckable, name, index+1, line)
		}
		if previous, duplicate := sums[fields[1]]; duplicate {
			return nil, fmt.Errorf("%w: %s carries two lines for %s, claiming %s and %s. Which one the archive is supposed to match is then a choice this check would be making on its own",
				errGateUncheckable, name, fields[1], previous, fields[0])
		}
		sums[fields[1]] = fields[0]
	}
	if len(sums) == 0 {
		return nil, fmt.Errorf("%w: %s carries no checksum lines at all. An empty checksum file is what a build that died mid-write leaves behind: it is present, it downloads, and it says nothing about any archive",
			errGateUncheckable, name)
	}
	return sums, nil
}

// gateArchivesSHA256 is one file's digest.
func gateArchivesSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

// ---------------------------------------------------------------------
// reading inside an archive
// ---------------------------------------------------------------------

// gateArchivesWalk visits every regular file in an archive, in one place for
// both container formats so that "what is in this archive" and "extract this one
// entry" cannot disagree about what the archive holds.
func gateArchivesWalk(archivePath, format string, visit func(name string, body io.Reader) (stop bool, err error)) error {
	switch format {
	case "zip":
		reader, err := zip.OpenReader(archivePath)
		if err != nil {
			return err
		}
		defer reader.Close()
		for _, entry := range reader.File {
			if entry.FileInfo().IsDir() {
				continue
			}
			body, err := entry.Open()
			if err != nil {
				return err
			}
			stop, err := visit(entry.Name, body)
			body.Close()
			if err != nil {
				return err
			}
			if stop {
				return nil
			}
		}
		return nil

	case "tar.gz", "tgz", "tar":
		file, err := os.Open(archivePath)
		if err != nil {
			return err
		}
		defer file.Close()
		var source io.Reader = file
		if format != "tar" {
			zipped, err := gzip.NewReader(file)
			if err != nil {
				return err
			}
			defer zipped.Close()
			source = zipped
		}
		entries := tar.NewReader(source)
		for {
			header, err := entries.Next()
			if errors.Is(err, io.EOF) {
				return nil
			}
			if err != nil {
				return err
			}
			if header.Typeflag != tar.TypeReg {
				continue
			}
			stop, err := visit(header.Name, entries)
			if err != nil {
				return err
			}
			if stop {
				return nil
			}
		}
	}
	// Unreachable through gateArchivesDerive, which refuses a format that is not
	// in gateArchivesExtension. It is here because this function is the only
	// place that knows how to open bytes, and a future caller that skipped the
	// derivation must fail rather than report an empty archive.
	return fmt.Errorf("this check cannot open a %q archive; it reads %v", format, gateArchivesReadableFormats())
}

// gateArchivesEntries is every file name an archive holds, sorted. It exists as
// its own pass so that "the binary is not in here" can be reported as what the
// archive DOES hold — which tells a reader whether the binary was renamed or
// never packaged, two different edits — rather than as a bare absence.
func gateArchivesEntries(archivePath, format string) ([]string, error) {
	var names []string
	err := gateArchivesWalk(archivePath, format, func(name string, _ io.Reader) (bool, error) {
		names = append(names, name)
		return false, nil
	})
	sort.Strings(names)
	return names, err
}

// gateArchivesHolds reports whether one of those entries IS the named file.
// Entries are matched on their base name because GoReleaser packages the binary
// at the archive root while some layouts nest it one directory down, and the
// question being asked is "is the release's binary in here", not "where".
func gateArchivesHolds(entries []string, name string) bool {
	for _, entry := range entries {
		if path.Base(entry) == name {
			return true
		}
	}
	return false
}

// gateArchivesExtract writes one entry out of an archive into dest and returns
// its path. Only the named entry is written — nothing else in the archive is
// unpacked — so a published artifact carrying a `../` entry cannot write outside
// dest through this function.
func gateArchivesExtract(archivePath, format, name, dest string) (string, error) {
	out := filepath.Join(dest, name)
	written := false
	err := gateArchivesWalk(archivePath, format, func(entry string, body io.Reader) (bool, error) {
		if path.Base(entry) != name {
			return false, nil
		}
		// 0o755: the archive's own mode bits are not trusted to be executable —
		// a zip written on a filesystem with no execute bit carries none — and
		// the next thing this file does is run it.
		file, err := os.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return true, err
		}
		defer file.Close()
		if _, err := io.Copy(file, body); err != nil {
			return true, err
		}
		written = true
		return true, nil
	})
	if err != nil {
		return "", err
	}
	if !written {
		return "", fmt.Errorf("%s holds no entry named %s", archivePath, name)
	}
	return out, nil
}

// gateArchivesStamp is what a built binary says about itself.
type gateArchivesStamp struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

// gateArchivesStampOf runs the binary and reads its version envelope.
//
// It asks the question the way viewer-tests/site_toolchain_test.go's snapshot
// dry run asks it — `version --format json`, decoded out of the `data` envelope
// — rather than inventing a second reading. Two readers of one envelope drift,
// and the one that drifts is always the one nobody runs before a tag.
func gateArchivesStampOf(binary string) (gateArchivesStamp, error) {
	var envelope struct {
		Data gateArchivesStamp `json:"data"`
	}
	out, err := exec.Command(binary, "version", "--format", "json").Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return envelope.Data, fmt.Errorf("`%s version --format json` exited non-zero: %w\n%s", filepath.Base(binary), err, strings.TrimSpace(string(exit.Stderr)))
		}
		return envelope.Data, fmt.Errorf("`%s version --format json` could not be run: %w", filepath.Base(binary), err)
	}
	if err := json.Unmarshal(out, &envelope); err != nil {
		return envelope.Data, fmt.Errorf("the binary's version envelope does not decode as JSON (%w); it printed:\n%s", err, strings.TrimSpace(string(out)))
	}
	return envelope.Data, nil
}

// ---------------------------------------------------------------------
// D7 itself
// ---------------------------------------------------------------------

// gateArchivesVerify is the entry point the release driver's D7 calls: for the
// release named version, built at commit, confirm that what the forge is serving
// is what this tree says it should be serving.
//
// It returns nil ONLY when all four questions in the file header were asked and
// answered. Every other return is an error wrapping one of the two sentinels,
// and there is no third outcome — in particular nothing here returns nil because
// something could not be examined.
func gateArchivesVerify(version, commit string) error {
	if strings.TrimSpace(version) == "" {
		return fmt.Errorf("%w: D7 was asked to verify a release's published archives without being told which release. "+
			"There is no default and there must not be one: the version is what names the tag on the forge, and guessing it would verify some other release and report on this one", errGateUncheckable)
	}
	if strings.TrimSpace(commit) == "" {
		return fmt.Errorf("%w: D7 was asked to verify the archives for %s without being told which commit it was built at. "+
			"The commit is both where the release configuration is read from and what the published binary's stamp is compared against, so without it this check could confirm the archives are internally consistent and say nothing about whether they are THIS release",
			errGateUncheckable, version)
	}

	raw, err := gateArchivesSource.ReleaseConfig(commit)
	if err != nil {
		return fmt.Errorf("%w: %s could not be read at %s, so what %s is supposed to publish is unknown and there is nothing to compare the forge against: %w",
			errGateUncheckable, gateGoreleaserFile, commit, version, err)
	}
	release, err := gateArchivesDerive(raw, version)
	if err != nil {
		return err
	}

	dir, err := gateArchivesSource.Download(release.Repo, version)
	if err != nil {
		return fmt.Errorf("%w: the assets published for %s on %s could not be fetched, so the release this driver has already tagged is unexamined. "+
			"The tag is out; this is a failed check and not an absent one: %w",
			errGateUncheckable, version, release.Repo, err)
	}

	sums, err := gateArchivesReadChecksums(dir, release.Checksums)
	if err != nil {
		return err
	}

	// Both directions, gathered rather than returned one at a time: a release
	// that dropped a platform usually drops it from the archive list AND from
	// the checksum file, and a report naming one of those sends the reader back
	// for the other.
	var findings []string
	checked := map[string]bool{}
	for _, want := range release.Wanted {
		asset := filepath.Join(dir, want.Name)
		info, err := os.Stat(asset)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			findings = append(findings, fmt.Sprintf("%s (%s/%s) is not among the assets %s publishes. %s declares it, so either the build did not produce it or the release did not upload it — and a platform's users have no download either way",
				want.Name, want.GOOS, want.GOARCH, version, gateGoreleaserFile))
			continue
		case err != nil:
			return fmt.Errorf("%w: %s was downloaded and cannot be read (%w), so its sha256 cannot be compared with the claim %s makes about it",
				errGateUncheckable, want.Name, err, release.Checksums)
		case info.IsDir():
			return fmt.Errorf("%w: %s is a directory in the downloaded assets, not an archive", errGateUncheckable, want.Name)
		}
		checked[want.Name] = true

		sum, err := gateArchivesSHA256(asset)
		if err != nil {
			return fmt.Errorf("%w: %s could not be digested (%w), so nothing can be said about whether it is the file %s claims",
				errGateUncheckable, want.Name, err, release.Checksums)
		}
		declared, listed := sums[want.Name]
		if !listed {
			findings = append(findings, fmt.Sprintf("%s is published and %s carries no line for it, so the one artifact a downloader is told to verify against says nothing about it",
				want.Name, release.Checksums))
			continue
		}
		if declared != sum {
			findings = append(findings, fmt.Sprintf("%s hashes to %s and %s claims %s. The bytes on the forge are not the bytes the release says it published",
				want.Name, sum, release.Checksums, declared))
		}
	}
	// The second direction. A line naming a file this check never looked at is a
	// published artifact nobody verified, and without this loop the report would
	// say "every declared archive matched" beside it.
	for _, name := range gateArchivesSortedNames(sums) {
		if !checked[name] {
			findings = append(findings, fmt.Sprintf("%s carries a line for %s, which is not one of the archives %s declares. It was published, it is being downloaded, and nothing here or in this tree has looked at it",
				release.Checksums, name, gateGoreleaserFile))
		}
	}
	if len(findings) > 0 {
		return fmt.Errorf("%w: the release published for %s does not match what %s at %s declares, in %d way(s):\n  %s",
			errGateArchivesWrong, version, gateGoreleaserFile, commit, len(findings), strings.Join(findings, "\n  "))
	}

	return gateArchivesVerifyHostBinary(dir, release, version, commit)
}

// gateArchivesSortedNames is a checksum map's file names, sorted, so that a
// report over an unchanged release is the identical document twice.
func gateArchivesSortedNames(sums map[string]string) []string {
	out := make([]string, 0, len(sums))
	for name := range sums {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// gateArchivesVerifyHostBinary is question 4: the archive for THIS platform is
// opened, the binary is run, and what it says about itself is compared with the
// release being published.
//
// It is a separate pass from the checksums above, and deliberately runs after
// them: asking a binary what version it is, out of an archive whose bytes are
// not the bytes the release claims, would produce a finding about a file that is
// already known to be the wrong file.
func gateArchivesVerifyHostBinary(dir string, release gateArchivesRelease, version, commit string) error {
	var host *gateArchivesWanted
	for index := range release.Wanted {
		if release.Wanted[index].GOOS == runtime.GOOS && release.Wanted[index].GOARCH == runtime.GOARCH {
			host = &release.Wanted[index]
			break
		}
	}
	if host == nil {
		return fmt.Errorf("%w: %s publishes no archive for %s/%s, which is the platform this check is running on, so there is no artifact here that can be executed. "+
			"It refuses rather than declaring the archive checks sufficient: a correct name and a correct sha256 are satisfied by an archive holding a stale binary. "+
			"The release publishes %v — run this driver on one of them, or add this platform to %s",
			errGateUncheckable, version, runtime.GOOS, runtime.GOARCH, gateArchivesTargets(release), gateGoreleaserFile)
	}

	binary := path.Base(release.Binary)
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	archivePath := filepath.Join(dir, host.Name)

	entries, err := gateArchivesEntries(archivePath, host.Format)
	if err != nil {
		return fmt.Errorf("%w: %s could not be opened as a %s archive (%w), so the binary a downloader on this platform gets cannot be looked at",
			errGateUncheckable, host.Name, host.Format, err)
	}
	if !gateArchivesHolds(entries, binary) {
		return fmt.Errorf("%w: %s carries no %s. It holds %v. A downloader on %s/%s unpacks this archive and has nothing to run",
			errGateArchivesWrong, host.Name, binary, entries, host.GOOS, host.GOARCH)
	}

	work, err := os.MkdirTemp("", "dossierx-d7-extract-")
	if err != nil {
		return fmt.Errorf("%w: a directory to unpack %s into could not be created: %w", errGateUncheckable, host.Name, err)
	}
	// This directory is this function's own, unlike the downloaded assets, so it
	// is cleaned up here.
	defer os.RemoveAll(work)

	extracted, err := gateArchivesExtract(archivePath, host.Format, binary, work)
	if err != nil {
		return fmt.Errorf("%w: %s could not be unpacked out of %s: %w", errGateUncheckable, binary, host.Name, err)
	}

	stamp, err := gateArchivesStampOf(extracted)
	if err != nil {
		return fmt.Errorf("%w: the binary published in %s could not be asked what it is (%v). "+
			"Either the artifact does not run or this machine cannot run it, and this check cannot tell those apart — both stop the release, and neither is a pass",
			errGateUncheckable, host.Name, err)
	}

	var findings []string
	// Normalized on both sides so this check is about which RELEASE the binary
	// is, never about the spelling. `.goreleaser.yaml` stamps `main.version`
	// from `{{.Tag}}` now, so the two sides agree byte for byte on a healthy
	// release — but the spelling has its own pin (gate_release_stamp_test.go),
	// and this check must not double as it: a stamp that moved back to the
	// stripped form is that pin's finding, while a binary carrying a DIFFERENT
	// release's version is this one's, and normalizing keeps the two apart.
	if gateDriverNormalizeVersion(stamp.Version) != gateDriverNormalizeVersion(version) {
		findings = append(findings, fmt.Sprintf("it reports version %q, and the release being published is %s. The archive is named for one release and carries the binary of another — every name, every checksum and every download link is correct, and `dossierx version` tells its user something else",
			stamp.Version, version))
	}
	if stamp.Commit != commit {
		findings = append(findings, fmt.Sprintf("it reports commit %q, and this release was tagged on %s. The published binary was built from other content than the tag names",
			stamp.Commit, commit))
	}
	if strings.TrimSpace(stamp.Date) == "" {
		// Non-empty rather than RFC 3339: that the stamp is a well-formed
		// timestamp is asserted before the tag, by
		// TestGoreleaserSnapshotBuildsSixArchivesAndStampsTheBinary over the
		// same build. What is checked HERE is the failure that reaches the forge
		// — the flag never applied, and `debug.ReadBuildInfo` had nothing to
		// fall back to either — and duplicating the shape check would be a
		// second reader of one envelope.
		findings = append(findings, "it reports no build date at all. `main.date` is stamped from `{{.Date}}`, so an empty one means the flag reached nothing and the build carried no VCS metadata to fall back on")
	}
	if len(findings) > 0 {
		return fmt.Errorf("%w: the binary published in %s is not the binary %s should carry, in %d way(s):\n  %s",
			errGateArchivesWrong, host.Name, version, len(findings), strings.Join(findings, "\n  "))
	}
	return nil
}

// gateArchivesTargets lists the platforms a release publishes for, for the
// refusal that has to say which ones they are.
func gateArchivesTargets(release gateArchivesRelease) []string {
	out := make([]string, 0, len(release.Wanted))
	for _, want := range release.Wanted {
		out = append(out, want.GOOS+"/"+want.GOARCH)
	}
	return out
}

// gateArchivesDriverShape pins gateArchivesVerify to the signature D7 calls it
// through.
//
// The driver's evidence interface lives in gate_driver_test.go, which this lane
// does not own, and the lane that wires the two together is a different one
// again. This declaration is the contract between them stated as a compile
// error: change gateArchivesVerify's parameters or its return and this file
// stops building, rather than the mismatch surfacing in somebody else's lane.
// The embedded interface is nil and nothing ever calls the other three methods —
// this exists to be type-checked, never to be run.
type gateArchivesDriverShape struct{ gateDriverEvidence }

func (gateArchivesDriverShape) Archives(version, commit string) error {
	return gateArchivesVerify(version, commit)
}

var _ gateDriverEvidence = gateArchivesDriverShape{}

// ---------------------------------------------------------------------
// the fixtures
// ---------------------------------------------------------------------

// gateArchivesStub is the offline evidence source. It is what makes every
// refusal above constructible without a forge: a directory of real archives, and
// the configuration bytes the derivation reads.
type gateArchivesStub struct {
	config      []byte
	configErr   error
	dir         string
	downloadErr error

	askedRepo string // what the check asked the forge for, recorded for assertion
}

func (s *gateArchivesStub) ReleaseConfig(string) ([]byte, error) {
	if s.configErr != nil {
		return nil, s.configErr
	}
	return s.config, nil
}

func (s *gateArchivesStub) Download(repo, _ string) (string, error) {
	s.askedRepo = repo
	if s.downloadErr != nil {
		return "", s.downloadErr
	}
	return s.dir, nil
}

// gateArchivesUse installs an evidence source for one test and puts the previous
// one back afterwards, so a test that fails part way cannot leave the production
// source replaced for every test after it.
func gateArchivesUse(t *testing.T, source gateArchivesEvidence) {
	t.Helper()
	previous := gateArchivesSource
	gateArchivesSource = source
	t.Cleanup(func() { gateArchivesSource = previous })
}

// gateArchivesRealConfig is this repository's own release configuration, read
// off the working copy. The tests below build their fixtures from what it
// DERIVES rather than from a list written here, so a seventh target added to the
// configuration produces a seventh archive in the fixture and this file needs no
// edit — which is the property the derivation exists to give the production
// path.
func gateArchivesRealConfig(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(surfaceRepoRoot(t), gateGoreleaserFile))
	if err != nil {
		t.Fatalf("read %s: %v", gateGoreleaserFile, err)
	}
	return raw
}

// gateArchivesFile is one member of a fixture archive.
type gateArchivesFile struct {
	Name string
	Body []byte
	Mode int64
}

// gateArchivesPack writes a real archive — a real gzip stream over a real tar,
// or a real zip — because the code under test opens these with archive/tar and
// archive/zip and a hand-rolled stand-in would prove nothing about either.
func gateArchivesPack(t *testing.T, dest, format string, files ...gateArchivesFile) {
	t.Helper()
	out, err := os.Create(dest)
	if err != nil {
		t.Fatalf("create %s: %v", dest, err)
	}
	defer out.Close()

	switch format {
	case "zip":
		writer := zip.NewWriter(out)
		for _, file := range files {
			header := &zip.FileHeader{Name: file.Name, Method: zip.Deflate}
			header.SetMode(fs.FileMode(file.Mode))
			entry, err := writer.CreateHeader(header)
			if err != nil {
				t.Fatalf("write %s into %s: %v", file.Name, dest, err)
			}
			if _, err := entry.Write(file.Body); err != nil {
				t.Fatalf("write %s into %s: %v", file.Name, dest, err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatalf("close %s: %v", dest, err)
		}
	default:
		gz := gzip.NewWriter(out)
		writer := tar.NewWriter(gz)
		for _, file := range files {
			if err := writer.WriteHeader(&tar.Header{
				Name: file.Name, Mode: file.Mode, Size: int64(len(file.Body)), Typeflag: tar.TypeReg,
			}); err != nil {
				t.Fatalf("write %s into %s: %v", file.Name, dest, err)
			}
			if _, err := writer.Write(file.Body); err != nil {
				t.Fatalf("write %s into %s: %v", file.Name, dest, err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatalf("close tar in %s: %v", dest, err)
		}
		if err := gz.Close(); err != nil {
			t.Fatalf("close gzip in %s: %v", dest, err)
		}
	}
}

// gateArchivesFixtureBinary builds a real, runnable, LINK-STAMPED binary that
// answers `version --format json` the way the engine does.
//
// It is built with the same `-X main.<symbol>=` mechanism `.goreleaser.yaml`
// uses rather than having the values written into its source, so the fixture
// exercises the same path a published artifact does: the binary reports what it
// was linked with. Building it is what lets the three stamp refusals below be
// constructed at all — a mis-stamped binary cannot be faked with a text file,
// because the check RUNS it.
func gateArchivesFixtureBinary(t *testing.T, version, commit, date string) []byte {
	t.Helper()
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "go.mod"), []byte("module dossierxarchivefixture\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatalf("write the fixture module: %v", err)
	}
	program := `package main

import (
	"encoding/json"
	"fmt"
	"os"
)

var version, commit, date string

func main() {
	if len(os.Args) < 2 || os.Args[1] != "version" {
		fmt.Fprintln(os.Stderr, "usage: version --format json")
		os.Exit(2)
	}
	out, err := json.Marshal(map[string]any{"data": map[string]string{
		"version": version, "commit": commit, "date": date,
	}})
	if err != nil {
		os.Exit(1)
	}
	fmt.Println(string(out))
}
`
	if err := os.WriteFile(filepath.Join(source, "main.go"), []byte(program), 0o644); err != nil {
		t.Fatalf("write the fixture program: %v", err)
	}

	name := "dossierx"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	built := filepath.Join(source, name)
	build := exec.Command("go", "build",
		"-ldflags", fmt.Sprintf("-X main.version=%s -X main.commit=%s -X main.date=%s", version, commit, date),
		"-o", built, ".")
	build.Dir = source
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build the fixture binary: %v\n%s", err, out)
	}
	body, err := os.ReadFile(built)
	if err != nil {
		t.Fatalf("read the fixture binary: %v", err)
	}
	return body
}

// gateArchivesWriteChecksums writes a checksum file over whatever archives are
// in dir — computed from the real bytes, so the healthy fixture is healthy by
// construction and a mutation that rewrites an archive makes the file disagree
// for the same reason a bad build would.
func gateArchivesWriteChecksums(t *testing.T, dir string, release gateArchivesRelease) {
	t.Helper()
	var lines []string
	for _, want := range release.Wanted {
		sum, err := gateArchivesSHA256(filepath.Join(dir, want.Name))
		if err != nil {
			t.Fatalf("digest %s: %v", want.Name, err)
		}
		lines = append(lines, sum+"  "+want.Name)
	}
	if err := os.WriteFile(filepath.Join(dir, release.Checksums), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", release.Checksums, err)
	}
}

// gateArchivesFixtureRelease lays out a whole published release on disk: every
// archive the plan declares, the host platform's carrying the stamped binary,
// and a checksum file over all of them.
func gateArchivesFixtureRelease(t *testing.T, release gateArchivesRelease, binary []byte) string {
	t.Helper()
	dir := t.TempDir()
	for _, want := range release.Wanted {
		files := []gateArchivesFile{
			{Name: "LICENSE", Body: []byte("MIT\n"), Mode: 0o644},
			{Name: "README.md", Body: []byte("# dossierx\n"), Mode: 0o644},
		}
		if want.GOOS == runtime.GOOS && want.GOARCH == runtime.GOARCH && binary != nil {
			name := path.Base(release.Binary)
			if want.GOOS == "windows" {
				name += ".exe"
			}
			files = append(files, gateArchivesFile{Name: name, Body: binary, Mode: 0o755})
		}
		gateArchivesPack(t, filepath.Join(dir, want.Name), want.Format, files...)
	}
	gateArchivesWriteChecksums(t, dir, release)
	return dir
}

// gateArchivesRepack rewrites one archive in a laid-out release WITHOUT
// rewriting the checksum file, which is what a mutation that changes an
// archive's contents has to do — the whole point of most of them is that the
// bytes and the claim about them come apart.
func gateArchivesRepack(t *testing.T, dir string, want gateArchivesWanted, files ...gateArchivesFile) {
	t.Helper()
	gateArchivesPack(t, filepath.Join(dir, want.Name), want.Format, files...)
}

// gateArchivesHostOf is the plan's entry for the platform the tests are running
// on, which is the only one whose binary can be executed here.
func gateArchivesHostOf(t *testing.T, release gateArchivesRelease) gateArchivesWanted {
	t.Helper()
	for _, want := range release.Wanted {
		if want.GOOS == runtime.GOOS && want.GOARCH == runtime.GOARCH {
			return want
		}
	}
	t.Fatalf("this repository's release configuration publishes nothing for %s/%s (%v), so the host-binary refusals below cannot be constructed here",
		runtime.GOOS, runtime.GOARCH, gateArchivesTargets(release))
	return gateArchivesWanted{}
}

// gateArchivesOtherOf is any archive that is NOT the host's, for the mutations
// that must break an archive without disturbing the one that gets executed.
func gateArchivesOtherOf(t *testing.T, release gateArchivesRelease) gateArchivesWanted {
	t.Helper()
	for _, want := range release.Wanted {
		if want.GOOS != runtime.GOOS || want.GOARCH != runtime.GOARCH {
			return want
		}
	}
	t.Fatalf("the release publishes only one archive (%v), so there is none to break without breaking the host's", gateArchivesTargets(release))
	return gateArchivesWanted{}
}

// gateArchivesRewriteChecksumLine replaces the line for one file, or removes it
// when replacement is empty.
func gateArchivesRewriteChecksumLine(t *testing.T, dir, checksums, file, replacement string) {
	t.Helper()
	path := filepath.Join(dir, checksums)
	blob, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", checksums, err)
	}
	var kept []string
	found := false
	for _, line := range strings.Split(strings.TrimRight(string(blob), "\n"), "\n") {
		if fields := strings.Fields(line); len(fields) == 2 && fields[1] == file {
			found = true
			if replacement != "" {
				kept = append(kept, replacement)
			}
			continue
		}
		kept = append(kept, line)
	}
	if !found {
		t.Fatalf("%s carries no line for %s, so the mutation would not be the one it claims to be", checksums, file)
	}
	if err := os.WriteFile(path, []byte(strings.Join(kept, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", checksums, err)
	}
}

// ---------------------------------------------------------------------
// the tests
// ---------------------------------------------------------------------

// gateArchivesFixtureCommit is a well-formed forty-character object name. It is
// not any commit in this repository, deliberately: every comparison the fixtures
// make is between two strings this test controls, so a fixture that happened to
// name a real object could pass for a reason nothing here intended.
const gateArchivesFixtureCommit = "1111111111111111111111111111111111111111"

// TestArchiveVerificationDerivesTheDownloadsThisTreePublishes holds the
// derivation against the repository's own configuration.
//
// It is the test that makes "do not hard-code six names" checkable: the expected
// set is computed by gateArchivesDerive out of `.goreleaser.yaml`, and compared
// against the six names written out INDEPENDENTLY — the same goos/goarch lists
// gate_release_stamp_test.go pins and the same spelling
// viewer-tests/site_toolchain_test.go stats. Two derivations of one fact that
// have to agree; if the configuration's matrix moves, the derived side moves
// with it and this test says the written side did not.
func TestArchiveVerificationDerivesTheDownloadsThisTreePublishes(t *testing.T) {
	release, err := gateArchivesDerive(gateArchivesRealConfig(t), "v9.9.9")
	if err != nil {
		t.Fatalf("this repository's own %s could not be read as a release: %v", gateGoreleaserFile, err)
	}

	if got, want := release.Repo, "BarterX-Tech/dossierx"; got != want {
		t.Errorf("the derivation would download the release from %q, not %q; every asset below would be fetched from the wrong repository", got, want)
	}
	if got := release.Checksums; got != gateChecksumFile {
		t.Errorf("the derivation expects the checksum file to be called %q, where the rest of this tree spells it %q", got, gateChecksumFile)
	}
	if got := release.Binary; got != "dossierx" {
		t.Errorf("the derivation expects the archive to hold a binary called %q, not %q; the host archive's binary is looked up by that name", got, "dossierx")
	}

	var want []string
	for _, goos := range gateGoreleaserGOOS {
		for _, goarch := range gateGoreleaserGOARCH {
			suffix := ".tar.gz"
			if goos == gateWindowsGOOS {
				suffix = "." + gateWindowsFormat
			}
			want = append(want, "dossierx_"+goos+"_"+goarch+suffix)
		}
	}
	var got []string
	for _, entry := range release.Wanted {
		got = append(got, entry.Name)
	}
	sort.Strings(want)
	sort.Strings(got)
	if !gateSameSet(got, want) {
		t.Fatalf("the derivation expects %v and this tree's own model of the release is %v.\n"+
			"One of the two moved: either %s's matrix, name template or format override changed and the written spelling here (and in docs/RELEASING.md's `gh release download --pattern`) has not, or the derivation stopped reading a field it needs",
			got, want, gateGoreleaserFile)
	}
	t.Logf("derived from %s: %v plus %s, from %v", gateGoreleaserFile, got, release.Checksums, gateArchivesTargets(release))
}

// TestArchiveVerificationAcceptsTheReleaseTheTreeDescribes is the healthy twin
// every refusal below is measured against. Without it a check that refused
// everything would look like thorough coverage.
func TestArchiveVerificationAcceptsTheReleaseTheTreeDescribes(t *testing.T) {
	config := gateArchivesRealConfig(t)
	release, err := gateArchivesDerive(config, "v9.9.9")
	if err != nil {
		t.Fatalf("derive the release: %v", err)
	}
	binary := gateArchivesFixtureBinary(t, "9.9.9", gateArchivesFixtureCommit, "2026-08-10T00:00:00Z")
	stub := &gateArchivesStub{config: config, dir: gateArchivesFixtureRelease(t, release, binary)}
	gateArchivesUse(t, stub)

	if err := gateArchivesVerify("v9.9.9", gateArchivesFixtureCommit); err != nil {
		t.Fatalf("a release that is exactly what %s describes was refused: %v", gateGoreleaserFile, err)
	}
	if stub.askedRepo != release.Repo {
		t.Errorf("the assets were fetched from %q, where the configuration names %q", stub.askedRepo, release.Repo)
	}
}

// TestArchiveVerificationRefusesEveryWayAPublishedReleaseCanBeWrong constructs
// each refusal in gateArchivesVerify.
//
// Every case is a real published release on disk carrying exactly one defect,
// because that is the only way to show that a refusal is reachable rather than
// written. The `want` column is the SENTINEL, not merely "an error": the two
// send an operator to different places — one to install a tool or fetch an
// object, the other to a release that is already public — and a refusal that
// fired with the wrong accusation would be as misleading as no refusal at all.
func TestArchiveVerificationRefusesEveryWayAPublishedReleaseCanBeWrong(t *testing.T) {
	config := gateArchivesRealConfig(t)
	release, err := gateArchivesDerive(config, "v9.9.9")
	if err != nil {
		t.Fatalf("derive the release: %v", err)
	}
	host := gateArchivesHostOf(t, release)
	other := gateArchivesOtherOf(t, release)
	healthy := gateArchivesFixtureBinary(t, "9.9.9", gateArchivesFixtureCommit, "2026-08-10T00:00:00Z")

	// A configuration whose only platform is not this one, for the refusal that
	// fires when nothing published can be executed here.
	elsewhere := "linux"
	if runtime.GOOS == "linux" {
		elsewhere = "darwin"
	}
	foreignConfig := gateArchivesConfigYAML(t, []string{elsewhere}, []string{runtime.GOARCH})

	cases := []struct {
		name    string
		version string
		commit  string
		config  []byte
		stub    func(s *gateArchivesStub)
		mutate  func(t *testing.T, dir string)
		want    error
		says    string
	}{
		{
			name: "the release as this tree describes it",
		},
		{
			name:    "no release was named",
			version: "  ",
			want:    errGateUncheckable,
			says:    "without being told which release",
		},
		{
			// Whitespace rather than "": an empty cell in this table means "use
			// the healthy value", and a blank string is what the argument
			// actually looks like when a caller passes an unset environment
			// variable through — which is the shape gateDriverPlanFromEnv trims.
			name:   "no commit was named",
			commit: "  ",
			want:   errGateUncheckable,
			says:   "without being told which commit",
		},
		{
			name: "the release configuration cannot be read at that commit",
			stub: func(s *gateArchivesStub) {
				s.configErr = errors.New("`git show`: fatal: invalid object name")
			},
			want: errGateUncheckable,
			says: "could not be read at",
		},
		{
			name: "the assets cannot be fetched",
			stub: func(s *gateArchivesStub) {
				s.downloadErr = errors.New("`gh` is not on PATH")
			},
			want: errGateUncheckable,
			says: "could not be fetched",
		},
		{
			name: "the release publishes no checksum file",
			mutate: func(t *testing.T, dir string) {
				if err := os.Remove(filepath.Join(dir, release.Checksums)); err != nil {
					t.Fatal(err)
				}
			},
			want: errGateUncheckable,
			says: "no readable " + gateChecksumFile,
		},
		{
			name: "the checksum file is empty",
			mutate: func(t *testing.T, dir string) {
				if err := os.WriteFile(filepath.Join(dir, release.Checksums), []byte("\n\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			want: errGateUncheckable,
			says: "carries no checksum lines at all",
		},
		{
			name: "a checksum line is not a checksum line",
			mutate: func(t *testing.T, dir string) {
				gateArchivesRewriteChecksumLine(t, dir, release.Checksums, other.Name, "not-a-sha  "+other.Name)
			},
			want: errGateUncheckable,
			says: "is not `<sha256>  <file>`",
		},
		{
			name: "the checksum file claims one archive twice",
			mutate: func(t *testing.T, dir string) {
				path := filepath.Join(dir, release.Checksums)
				blob, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				extra := strings.Repeat("a", 64) + "  " + other.Name + "\n"
				if err := os.WriteFile(path, append(blob, []byte(extra)...), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			want: errGateUncheckable,
			says: "carries two lines for",
		},
		{
			name: "an archive the release declares was never published",
			mutate: func(t *testing.T, dir string) {
				if err := os.Remove(filepath.Join(dir, other.Name)); err != nil {
					t.Fatal(err)
				}
			},
			want: errGateArchivesWrong,
			says: "is not among the assets",
		},
		{
			name: "an archive's bytes are not the bytes it claims",
			mutate: func(t *testing.T, dir string) {
				gateArchivesRepack(t, dir, other, gateArchivesFile{Name: "LICENSE", Body: []byte("tampered\n"), Mode: 0o644})
			},
			want: errGateArchivesWrong,
			says: "The bytes on the forge are not the bytes the release says it published",
		},
		{
			name: "a published archive has no line in the checksum file",
			mutate: func(t *testing.T, dir string) {
				gateArchivesRewriteChecksumLine(t, dir, release.Checksums, other.Name, "")
			},
			want: errGateArchivesWrong,
			says: "carries no line for it",
		},
		{
			name: "the checksum file names a file nobody checked",
			mutate: func(t *testing.T, dir string) {
				path := filepath.Join(dir, release.Checksums)
				blob, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				extra := strings.Repeat("b", 64) + "  dossierx_sourcecode.tar.gz\n"
				if err := os.WriteFile(path, append(blob, []byte(extra)...), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			want: errGateArchivesWrong,
			says: "nothing here or in this tree has looked at it",
		},
		{
			name:   "the release publishes nothing this machine can run",
			config: foreignConfig,
			want:   errGateUncheckable,
			says:   "publishes no archive for " + runtime.GOOS + "/" + runtime.GOARCH,
		},
		{
			name: "the host's archive carries no binary",
			mutate: func(t *testing.T, dir string) {
				gateArchivesRepack(t, dir, host, gateArchivesFile{Name: "LICENSE", Body: []byte("MIT\n"), Mode: 0o644})
				gateArchivesWriteChecksums(t, dir, release)
			},
			want: errGateArchivesWrong,
			says: "carries no dossierx",
		},
		{
			name: "the published binary cannot be run at all",
			mutate: func(t *testing.T, dir string) {
				gateArchivesRepack(t, dir, host,
					gateArchivesFile{Name: path.Base(release.Binary) + gateArchivesHostSuffix(), Body: []byte("not a binary\n"), Mode: 0o755})
				gateArchivesWriteChecksums(t, dir, release)
			},
			want: errGateUncheckable,
			says: "could not be asked what it is",
		},
		{
			name: "the published binary reports another release",
			mutate: func(t *testing.T, dir string) {
				stale := gateArchivesFixtureBinary(t, "9.9.8", gateArchivesFixtureCommit, "2026-08-10T00:00:00Z")
				gateArchivesRepack(t, dir, host,
					gateArchivesFile{Name: path.Base(release.Binary) + gateArchivesHostSuffix(), Body: stale, Mode: 0o755})
				gateArchivesWriteChecksums(t, dir, release)
			},
			want: errGateArchivesWrong,
			says: "it reports version",
		},
		{
			name: "the published binary was built from other content",
			mutate: func(t *testing.T, dir string) {
				stale := gateArchivesFixtureBinary(t, "9.9.9", strings.Repeat("c", 40), "2026-08-10T00:00:00Z")
				gateArchivesRepack(t, dir, host,
					gateArchivesFile{Name: path.Base(release.Binary) + gateArchivesHostSuffix(), Body: stale, Mode: 0o755})
				gateArchivesWriteChecksums(t, dir, release)
			},
			want: errGateArchivesWrong,
			says: "it reports commit",
		},
		{
			name: "the published binary carries no build date",
			mutate: func(t *testing.T, dir string) {
				undated := gateArchivesFixtureBinary(t, "9.9.9", gateArchivesFixtureCommit, "")
				gateArchivesRepack(t, dir, host,
					gateArchivesFile{Name: path.Base(release.Binary) + gateArchivesHostSuffix(), Body: undated, Mode: 0o755})
				gateArchivesWriteChecksums(t, dir, release)
			},
			want: errGateArchivesWrong,
			says: "reports no build date at all",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			caseConfig := config
			caseRelease := release
			if testCase.config != nil {
				caseConfig = testCase.config
				var err error
				if caseRelease, err = gateArchivesDerive(caseConfig, "v9.9.9"); err != nil {
					t.Fatalf("derive the case's release: %v", err)
				}
			}
			dir := gateArchivesFixtureRelease(t, caseRelease, healthy)
			if testCase.mutate != nil {
				testCase.mutate(t, dir)
			}
			stub := &gateArchivesStub{config: caseConfig, dir: dir}
			if testCase.stub != nil {
				testCase.stub(stub)
			}
			gateArchivesUse(t, stub)

			version, commit := "v9.9.9", gateArchivesFixtureCommit
			if testCase.version != "" {
				version = testCase.version
			}
			if testCase.commit != "" {
				commit = testCase.commit
			}

			err := gateArchivesVerify(version, commit)
			if testCase.want == nil {
				if err != nil {
					t.Fatalf("a healthy release was refused: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("accepted. D7 would have reported the archives verified, and the driver would have gone on to push main")
			}
			if !errors.Is(err, testCase.want) {
				t.Fatalf("refused with the wrong accusation.\n  want: %v\n  got:  %v\n"+
					"The two sentinels send an operator to different places — one to a missing tool or object, the other to a release that is already public", testCase.want, err)
			}
			if testCase.says != "" && !strings.Contains(err.Error(), testCase.says) {
				t.Fatalf("the refusal does not say %q, so it may be firing for some other reason:\n%v", testCase.says, err)
			}
			t.Logf("refused: %v", err)
		})
	}
}

// gateArchivesClock is a clock a test owns: `sleep` moves it rather than
// stopping the test for twenty minutes, so the poll below is exercised over its
// whole budget in microseconds. Sleeping for real would make this test either
// slow or a test of a different, shorter budget than the one that ships.
type gateArchivesClock struct {
	at     time.Time
	slept  []time.Duration
	waited time.Duration
}

func (c *gateArchivesClock) Now() time.Time { return c.at }

func (c *gateArchivesClock) Sleep(d time.Duration) {
	c.slept = append(c.slept, d)
	c.waited += d
	c.at = c.at.Add(d)
}

// TestArchiveDownloadWaitsForTheReleaseWorkflowInsteadOfAskingToBeRunAgain is
// the test for the failure this check was BUILT with and that would have
// stranded every release.
//
// The draft's Download asked the forge once and, on the ordinary first answer —
// the workflow the tag push started has not uploaded anything yet — refused with
// "wait and re-run". There is no re-run: refuseIfAlreadyPublished refuses every
// invocation made after the tag is public, and D6 pushes the tag before D7 runs.
// So that refusal did not fire on unlucky releases, it fired on all of them, and
// every one ended with the tag out, main behind it and a human finishing the
// release by hand — the outcome the whole driver exists to remove.
//
// The three things asserted here are the three the fix consists of: it asks
// again, it stops asking, and what it says when it stops does not ask for
// something nobody can do.
func TestArchiveDownloadWaitsForTheReleaseWorkflowInsteadOfAskingToBeRunAgain(t *testing.T) {
	const (
		budget   = 2 * time.Minute
		interval = 10 * time.Second
	)

	// The wait has to fit INSIDE the timeout the release-time invocation
	// carries, with room left for the checking that follows it. A budget at or
	// above the floor does not produce this refusal at all — it produces `go
	// test`'s panic, mid-release, with the tag already published.
	t.Run("the wait fits inside the timeout floor with room to verify", func(t *testing.T) {
		if gateArchivesPollBudget >= gateDriverTimeoutFloor {
			t.Fatalf("D7 waits up to %s inside an invocation guaranteed only %s. The wait would consume the whole budget and the run would panic between the tag push and the main push, printing a goroutine dump instead of what is already published",
				gateArchivesPollBudget, gateDriverTimeoutFloor)
		}
		if left := gateDriverTimeoutFloor - gateArchivesPollBudget; left < 5*time.Minute {
			t.Fatalf("the wait leaves %s of the %s floor to download six archives, digest them and run one binary. That is not enough to finish the check the wait exists to make possible",
				left, gateDriverTimeoutFloor)
		}
	})

	t.Run("it asks again while the workflow is still uploading", func(t *testing.T) {
		clock := &gateArchivesClock{at: time.Unix(0, 0).UTC()}
		attempts := 0
		forge := gateArchivesForge{
			pollBudget:   budget,
			pollInterval: interval,
			now:          clock.Now,
			sleep:        clock.Sleep,
			fetch: func(dir string) error {
				attempts++
				if attempts < 3 {
					return errors.New("release not found")
				}
				return os.WriteFile(filepath.Join(dir, gateChecksumFile), []byte("ok\n"), 0o644)
			},
		}

		dir, err := forge.Download("BarterX-Tech/dossierx", "v9.9.9")
		if err != nil {
			t.Fatalf("the assets appeared on the third attempt and Download refused anyway: %v", err)
		}
		if attempts != 3 {
			t.Fatalf("the forge was asked %d time(s). One is the defect this test exists for: at D7 the workflow has just been started by the tag push, so the first answer is normally 'not yet'", attempts)
		}
		if len(clock.slept) != 2 || clock.slept[0] != interval {
			t.Fatalf("it waited %v between attempts, where the interval it was given is %s", clock.slept, interval)
		}
		if _, err := os.Stat(filepath.Join(dir, gateChecksumFile)); err != nil {
			t.Fatalf("Download returned %s and the assets the last attempt wrote are not in it: %v", dir, err)
		}
	})

	t.Run("it stops, and says so without asking to be run again", func(t *testing.T) {
		clock := &gateArchivesClock{at: time.Unix(0, 0).UTC()}
		attempts := 0
		last := "HTTP 404: Not Found (release v9.9.9 has no assets)"
		forge := gateArchivesForge{
			pollBudget:   budget,
			pollInterval: interval,
			now:          clock.Now,
			sleep:        clock.Sleep,
			fetch: func(string) error {
				attempts++
				return errors.New(last)
			},
		}

		dir, err := forge.Download("BarterX-Tech/dossierx", "v9.9.9")
		if err == nil {
			t.Fatalf("the assets never appeared and Download returned %s with no error. D7 would have gone on to read a checksum file that is not there — or worse, an empty directory — and the driver would have pushed main behind a release nobody looked at", dir)
		}
		if attempts < 2 {
			t.Fatalf("it gave up after %d attempt(s) rather than polling out the %s budget", attempts, budget)
		}
		if clock.waited < budget {
			t.Fatalf("it waited %s of the %s budget before refusing, so the release is refused while the workflow it is waiting for may still be uploading", clock.waited, budget)
		}

		// The deadline and the interval, because a wait an operator cannot see
		// the size of is indistinguishable from a hang, and the number they
		// need is the one this run actually used rather than the constant.
		for _, want := range []string{budget.String(), interval.String()} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("the timeout refusal does not say %q, so it does not tell the operator how long D7 waited or how often it asked:\n%v", want, err)
			}
		}
		// The last attempt's own words: "did not appear" alone cannot tell a
		// workflow that is still building from one that failed, and those are
		// different decisions for the human who now owns this release.
		if !strings.Contains(err.Error(), last) {
			t.Fatalf("the timeout refusal does not quote what the forge last said (%q), so the reader cannot tell a slow build from a failed one:\n%v", last, err)
		}
		// The defect itself. Anything that reads as "do this again" is a
		// promise the driver cannot keep, because the tag is public and every
		// later invocation is refused.
		for _, forbidden := range []string{"re-run", "rerun", "try again", "run it again"} {
			if strings.Contains(strings.ToLower(err.Error()), forbidden) {
				t.Fatalf("the timeout refusal says %q. There is no such invocation — refuseIfAlreadyPublished refuses every run made after the tag is pushed — so this sends the operator to a command that will refuse them:\n%v", forbidden, err)
			}
		}
		t.Logf("refused: %v", err)
	})
}

// gateArchivesHostSuffix is the executable suffix the host platform's archive
// carries, so the mutations above name the same file the check looks for.
func gateArchivesHostSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// gateArchivesConfigYAML renders a minimal release configuration for one
// goos/goarch matrix. The refusal cases below hand it deliberately broken
// documents; this is the shape they are broken FROM.
func gateArchivesConfigYAML(t *testing.T, goos, goarch []string) []byte {
	t.Helper()
	return []byte(fmt.Sprintf(`project_name: dossierx
builds:
  - id: dossierx
    binary: dossierx
    goos: [%s]
    goarch: [%s]
archives:
  - id: dossierx
    ids: [dossierx]
    name_template: "{{ .ProjectName }}_{{ .Os }}_{{ .Arch }}"
    format_overrides:
      - goos: windows
        format: zip
checksum:
  name_template: "checksums.txt"
release:
  github:
    owner: BarterX-Tech
    name: dossierx
`, strings.Join(goos, ", "), strings.Join(goarch, ", ")))
}

// TestArchiveDerivationRefusesAConfigurationItCannotReadAReleaseFrom constructs
// every refusal in gateArchivesDerive.
//
// These are all errGateUncheckable and all for one reason: a release
// configuration this file cannot read leaves it with no expected set, and an
// empty expected set makes every comparison in gateArchivesVerify true over
// nothing. The alternative it refuses to take — falling back to the six names
// this repository publishes today — is the hard-coded list the whole derivation
// exists to remove, and it would be wrong at exactly the moment it mattered.
func TestArchiveDerivationRefusesAConfigurationItCannotReadAReleaseFrom(t *testing.T) {
	healthy := string(gateArchivesConfigYAML(t, []string{"linux", "windows"}, []string{"amd64"}))

	cases := []struct {
		name   string
		config string
		says   string
	}{
		{
			name:   "not YAML at all",
			config: "builds: [\n",
			says:   "does not parse as YAML",
		},
		{
			name: "no builds",
			config: `archives:
  - name_template: "x_{{ .Os }}"
checksum:
  name_template: "checksums.txt"
release:
  github: {owner: a, name: b}
`,
			says: "declares no builds",
		},
		{
			name:   "no archives",
			config: strings.Replace(healthy, "archives:", "unarchives:", 1),
			says:   "declares 0 archive blocks",
		},
		{
			name: "two archive blocks",
			config: strings.Replace(healthy, "checksum:\n",
				"  - id: second\n    ids: [dossierx]\n    name_template: \"other_{{ .Os }}_{{ .Arch }}\"\nchecksum:\n", 1),
			says: "declares 2 archive blocks",
		},
		{
			name:   "no forge to download from",
			config: strings.Replace(healthy, "    owner: BarterX-Tech\n", "", 1),
			says:   "does not name both an owner and a repository",
		},
		{
			name:   "no checksum file name",
			config: strings.Replace(healthy, "  name_template: \"checksums.txt\"\n", "", 1),
			says:   "declares no checksum name_template",
		},
		{
			name:   "a build with no goos",
			config: strings.Replace(healthy, "    goos: [linux, windows]\n", "", 1),
			says:   "declares no `goos`",
		},
		{
			name:   "a build with no goarch",
			config: strings.Replace(healthy, "    goarch: [amd64]\n", "", 1),
			says:   "declares no `goarch`",
		},
		{
			name:   "an archive packaging a build that does not exist",
			config: strings.Replace(healthy, "    ids: [dossierx]\n", "    ids: [dossierx, ghost]\n", 1),
			says:   "packages build ids",
		},
		{
			name:   "a name template naming something nothing models",
			config: strings.Replace(healthy, `name_template: "{{ .ProjectName }}_{{ .Os }}_{{ .Arch }}"`, `name_template: "{{ .ProjectName }}_{{ .Runtime }}"`, 1),
			says:   "names a value this check does not model",
		},
		{
			name:   "a name template that does not parse",
			config: strings.Replace(healthy, `name_template: "{{ .ProjectName }}_{{ .Os }}_{{ .Arch }}"`, `name_template: "{{ .ProjectName"`, 1),
			says:   "is not a template this check can parse",
		},
		{
			name:   "a name template that renders to a name with a space in it",
			config: strings.Replace(healthy, `name_template: "{{ .ProjectName }}_{{ .Os }}_{{ .Arch }}"`, `name_template: "{{ .ProjectName }} {{ .Os }}"`, 1),
			says:   "is not a single file name",
		},
		{
			name:   "an archive format this check cannot open",
			config: strings.Replace(healthy, "        format: zip\n", "        format: tar.xz\n", 1),
			says:   "which this check can neither name nor open",
		},
		{
			name:   "a format override that overrides nothing",
			config: strings.Replace(healthy, "        format: zip\n", "", 1),
			says:   "declares no archive format for the format override for goos windows",
		},
		{
			name:   "both spellings of the format at once",
			config: strings.Replace(healthy, "    ids: [dossierx]\n", "    ids: [dossierx]\n    format: tar.gz\n    formats: [zip]\n", 1),
			says:   "declares both `format:",
		},
		{
			name:   "more than one format per target",
			config: strings.Replace(healthy, "    ids: [dossierx]\n", "    ids: [dossierx]\n    formats: [tar.gz, zip]\n", 1),
			says:   "so each target publishes 2 files",
		},
		{
			// Two platforms packaged the same way — the windows override would
			// otherwise separate them by extension — under a template that has
			// stopped naming the operating system.
			name: "two targets that would publish the same file",
			config: strings.Replace(
				string(gateArchivesConfigYAML(t, []string{"linux", "darwin"}, []string{"amd64"})),
				`name_template: "{{ .ProjectName }}_{{ .Os }}_{{ .Arch }}"`, `name_template: "{{ .ProjectName }}_{{ .Arch }}"`, 1),
			says: "the same file",
		},
		{
			name:   "every target ignored",
			config: strings.Replace(healthy, "    goarch: [amd64]\n", "    goarch: [amd64]\n    ignore:\n      - goarch: amd64\n", 1),
			says:   "every goos/goarch pair in them is ignored",
		},
		{
			name: "builds that disagree about the binary's name",
			config: strings.Replace(
				strings.Replace(healthy, "    ids: [dossierx]\n", "    ids: [dossierx, second]\n", 1),
				"archives:\n",
				"  - id: second\n    binary: dossierx2\n    goos: [darwin]\n    goarch: [arm64]\narchives:\n", 1),
			says: "disagree about the binary's name",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// The healthy document derives, so every refusal below is the one
			// edit and not the shape it was edited from.
			if _, err := gateArchivesDerive([]byte(healthy), "v1.2.3"); err != nil {
				t.Fatalf("the healthy fixture configuration does not derive: %v", err)
			}
			_, err := gateArchivesDerive([]byte(testCase.config), "v1.2.3")
			if err == nil {
				t.Fatal("accepted. The expected set would have been derived from a configuration this check cannot read, and every comparison against the forge would have been made about the wrong files")
			}
			if !errors.Is(err, errGateUncheckable) {
				t.Fatalf("refused with the wrong accusation — a configuration that cannot be read is uncheckable, not a wrong release: %v", err)
			}
			if !strings.Contains(err.Error(), testCase.says) {
				t.Fatalf("the refusal does not say %q, so it may be firing for some other reason:\n%v", testCase.says, err)
			}
			t.Logf("refused: %v", err)
		})
	}
}

// TestArchiveVerificationReadsBothContainerFormats holds the two readers against
// archives this test wrote, because everything above rests on them: a reader
// that silently returned no entries would make "the archive holds no binary"
// fire on every correct release, and one that silently returned the wrong bytes
// would run the wrong binary.
func TestArchiveVerificationReadsBothContainerFormats(t *testing.T) {
	for _, format := range []string{"tar.gz", "zip"} {
		t.Run(format, func(t *testing.T) {
			dir := t.TempDir()
			archive := filepath.Join(dir, "sample."+format)
			gateArchivesPack(t, archive, format,
				gateArchivesFile{Name: "LICENSE", Body: []byte("MIT\n"), Mode: 0o644},
				gateArchivesFile{Name: "dossierx", Body: []byte("#!/bin/sh\n"), Mode: 0o755},
			)
			entries, err := gateArchivesEntries(archive, format)
			if err != nil {
				t.Fatalf("read %s: %v", archive, err)
			}
			if !gateSameSet(entries, []string{"LICENSE", "dossierx"}) {
				t.Fatalf("a %s archive holding LICENSE and dossierx read as %v", format, entries)
			}
			if !gateArchivesHolds(entries, "dossierx") {
				t.Fatalf("the binary is in the archive and gateArchivesHolds says it is not")
			}
			out, err := gateArchivesExtract(archive, format, "dossierx", t.TempDir())
			if err != nil {
				t.Fatalf("extract from %s: %v", format, err)
			}
			body, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			if string(body) != "#!/bin/sh\n" {
				t.Fatalf("the extracted entry holds %q, not the bytes that were packed", body)
			}
			info, err := os.Stat(out)
			if err != nil {
				t.Fatal(err)
			}
			if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
				t.Fatalf("the extracted binary is not executable (%v), so the stamp check could never run it", info.Mode())
			}
		})
	}
}
