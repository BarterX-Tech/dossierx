// release_notes_predict_lib_test.go is the `release-notes` surface's
// predictor: the reusable machinery that answers "what will the published
// GitHub release body say", BEFORE the tag that generates it exists. That is
// surfaces.yaml's entry whose only path is .goreleaser.yaml — "the config file
// IS the reviewable surface", because the notes themselves do not exist until
// the tag.
//
// The surface is cited by NAME. This header used to say "Surface 13's
// predictor"; surfaces.yaml defines no numbering, and by position
// `release-notes` is entry 12 while entry 13 is `binary-and-viewer` — the
// label pointed at the code surface, i.e. at the wrong thing entirely. See
// skills_export_capture_helpers_test.go's header for the same correction.
//
// This is a _test.go file rather than a plain package file so that adding it
// does not turn `tests` into a real, importable Go package: cmd/dossierx's
// TestSurfaceBehaviourFingerprintCoversEveryPackage independently derives its
// package list from `git ls-files` and requires every NON-test Go source to
// be either fingerprinted (behaviourRoots: cmd, internal, skills) or
// explicitly named in behaviourExclusions — neither of which `tests` is or
// should become, since nothing here ships in the binary. A _test.go file is
// invisible to that scan, exactly like every other helper in this package
// already is.
//
// WHY THIS HAS TO BE A PREDICTOR AND NOT A READER. GoReleaser builds the
// release body from Conventional Commit SUBJECTS at tag time
// (internal/pipe/changelog/changelog.go in the goreleaser module). The
// pre-merge gate (G1) runs before any tag exists, so it cannot read the
// notes; it has to run the same pipeline GoReleaser will run and record what
// comes out. The pre-tag gate (G2) re-runs it over the range that WILL
// exist once the release PR's merge commit lands, and diffs its output
// against G1's recorded prediction — so the two runs must use the exact
// same algorithm, which is the whole reason this lives in one reusable
// place rather than being copied into each gate stage.
//
// GROUND TRUTH IS GORELEASER v2, NOT v1. .goreleaser.yaml:1 pins the config
// SCHEMA to "version: 2", and release.yml's goreleaser-action pins the tool
// to the same v2 version ci.yml tests under (the two pins are held equal by
// tests/ci_workflow_test.go); any v2 release refuses a version:2 config
// entirely under v1. The only GoReleaser module cached
// locally ($GOPATH/pkg/mod/github.com/goreleaser/goreleaser@v1.26.2) is v1
// and is NOT what runs at release time — a predictor mirrored against it
// silently drifts from what actually publishes (see the abbrev-commit note
// below, which is exactly that drift). The algorithm below was checked
// against goreleaser/goreleaser/v2@v2.17.1's
// internal/pipe/changelog/changelog.go (fetched from
// github.com/goreleaser/goreleaser, since no v2 module is vendored here)
// field for field for filterEntries/remove, formatChangelog's group-matching
// and group-sorting, and the "asc"/"desc" sortEntries comparator — all four
// operate on entry.Message (the SUBJECT, %s) exactly as modeled below.
// Re-verify this citation if this predictor is ever revised — the source,
// not GoReleaser's own docs, is what settles a disagreement, because the
// docs do not spell out match-order or sort-key details, and this package
// doc has previously stated a claim (item 4 below, see its own note) that
// read correctly against the wrong cached version and was wrong in general.
//
// gitChangeloger.Log itself is NOT field-for-field, and an earlier version of
// this comment claimed it was — that claim was wrong, caught by re-fetching
// the real v2.17.1 source rather than trusting a citation already in the
// file. Real GoReleaser does not run `--pretty=format:%H %s` and split on
// newline the way gitLogOneline below does; it wraps FIVE fields (SHA,
// Message, MessageBody, AuthorName, AuthorEmail) in fixed
// "<goreleaser_x>...</goreleaser_x>" delimiter tags and splits records on an
// explicit "<goreleaser_commit_divider>\n" marker, then extracts each field
// by locating its own tags with strings.Index — a scheme that is immune to
// ANY extra text `git log` happens to emit into stdout around or inside a
// record (a multi-line commit body via %b, git notes, a signature-verification
// banner, ...), because none of that text can contain goreleaser's own
// tag strings and decode() never looks outside them.
//
// gitLogOneline's plain "%H %s" + newline-per-line parsing has no such
// immunity: ANY ambient git config that makes `git log` print an extra stdout
// line ahead of a commit's own formatted line — signature verification banners
// under log.showSignature=true being the one case verified against this
// project's real history and use of signed commits — is read by
// splitCommitLine as its own fabricated, malformed entry. GoReleaser's own
// invocation is immune to that ambient case specifically because it ALSO
// forces "-c log.showSignature=false" ahead of every git call
// (internal/git.RunWithEnv, git.go:21-24) — a defense gitLogOneline below
// reproduces — but that guard is the only line of defense left once the
// tag+divider robustness isn't also reproduced. This predictor deliberately
// does NOT reproduce goreleaser's full tag+divider scheme (a materially
// larger rewrite of a well-tested core path, for defense against noise
// sources — git notes, mailmap warnings — this project has no evidence of
// ever enabling); the log.showSignature guard closes the one such gap that IS
// realistic here, and TestPredictReleaseNotesForRange_ImmuneToAmbientGitConfig_SignedCommits
// is what pins it. Flagged in this lane's own report as a known, narrower-
// than-upstream limitation rather than silently left for the next reader to
// rediscover.
//
// THE ALGORITHM, mirrored field for field from changelog.go:
//
//  1. gitChangeloger.Log runs `git log --no-decorate --no-color
//     --pretty=format:<sha><message>...` over <revRange>, and — this is
//     easy to read past — EVERY git invocation goreleaser makes, this one
//     included, is run through internal/git.RunWithEnv, which prepends "-c
//     log.showSignature=false" ahead of the subcommand (git.go:21-24) before
//     any caller-supplied args. gitLogOneline below reproduces both the "-c"
//     guard and the %H/%s format verbs, rather than relying on
//     `--pretty=oneline`, which this predictor used to call instead: that
//     was wrong on two independent axes, not one. `--pretty=oneline`'s sha
//     field is `%h`-shaped in the general case — its length tracks
//     log.abbrevCommit / core.abbrev the same way `%h` does, and only
//     happens to print the full 40 characters at git's own untouched
//     defaults, whereas `%H` is unconditionally full-length regardless of
//     any abbreviation config (verified locally: `git -c
//     log.abbrevCommit=true log --pretty=oneline -1` prints a 7-character
//     sha, `git -c log.abbrevCommit=true log --pretty=format:%H -1` still
//     prints the full 40). And with no `-c log.showSignature=false` guard, a
//     signed commit under an ambient `log.showSignature=true` (repo-local,
//     global, or system config) prints a "Good \"git\" signature ..." (or
//     "No signature", or an "error: ..." — verified against a real ssh-signed
//     commit locally, all three land on STDOUT, so exec.Command's Output()
//     does capture them) block ahead of the commit line that a per-line parse
//     reads as its own malformed entry, landing in the catch-all "Other
//     changes" group of a PUBLISHED prediction. GoReleaser is immune to this
//     for two independent reasons, not one: it always forces the guard, AND
//     its own parser extracts fields from fixed delimiter tags rather than
//     splitting on newline (see the package doc's "gitChangeloger.Log itself
//     is NOT field-for-field" paragraph above), so a stray stdout line simply
//     falls outside every tag pair and is discarded. gitLogOneline below only
//     has the first defense, which is exactly why the guard is load-bearing
//     here in a way it is not for GoReleaser itself; a predictor that leaves
//     it out can produce a plausible, wrong prediction that G1 and G2 both
//     compute identically (same broken invocation, same ambient config) so
//     PublishedEqual passes over a shared error rather than catching it. See
//     gitLogOneline's own doc comment for the fix's exact shape, and
//     TestPredictReleaseNotesForRange_ImmuneToAmbientGitConfig (log.abbrevCommit)
//     and TestPredictReleaseNotesForRange_ImmuneToAmbientGitConfig_SignedCommits
//     (log.showSignature, over real SSH-signed commits) for the regression
//     tests.
//  2. filterEntries: for each pattern in changelog.filters.exclude, in
//     order, drop every remaining entry whose SUBJECT (entry.Message; the
//     sha plays no part in this match — changelog.go's `remove` calls
//     `filter.MatchString(entry.Message)`) matches it.
//  3. sortEntries: changelog.sort == "asc" sorts the SURVIVING entries by
//     plain lexicographic comparison of the subject text (entry.Message
//     again) — NOT commit date. ("asc"/"desc" is the only distinction
//     GoReleaser draws; this project's config uses "asc". v2's own sort,
//     slices.SortFunc, is not guaranteed stable; this predictor uses
//     sort.SliceStable, which only matters for two entries with
//     byte-identical subjects, an edge case not otherwise modeled.)
//  4. formatChangelog: walk changelog.groups IN THE ORDER THEY APPEAR IN THE
//     CONFIG. A group with a non-empty regexp claims every remaining entry
//     whose SUBJECT ALONE matches it — changelog.go:185 is
//     `match := re.MatchString(entry.Message)`, and entry.Message is never
//     the sha-prefixed line, unlike a plain `git log --pretty=oneline`
//     reading might suggest. (An EARLIER version of this predictor, and of
//     this doc comment, matched the FULL "<sha> <subject>" line instead —
//     masked in this project's own committed regexes only because they all
//     open with `^.*?`, and a hex sha cannot spell "feat" or "fix", so the
//     wrong target happened to accept the same entries the right one does.
//     TestPredictReleaseNotes_GroupRegexMatchesSubjectOnly pins the
//     corrected, subject-only behavior against a regexp that is NOT
//     `^.*?`-prefixed, where the two targets provably disagree.) A group
//     with NO regexp (this project's "Other changes") is a catch-all: it
//     takes everything still unclaimed, regardless of its own position in
//     the order list. Once every group has run, the GROUPS THEMSELVES (not
//     their entries) are re-sorted by their `order` field, ascending, and
//     rendered as "## Changelog", "### <title>", "* <sha> <subject>" — a
//     group with zero entries is omitted entirely.
//
// filters.include, changelog.abbrev, changelog.format, changelog.disable and
// changelog.use are deliberately NOT modeled: this project's .goreleaser.yaml
// sets none of them, and each changes the published body in a way this
// predictor's algorithm does not implement (filters.include, if present,
// REPLACES exclude filtering rather than combining with it — changelog.go
// returns before Exclude ever runs; abbrev shortens every sha; disable skips
// the changelog step entirely; format replaces "* <sha> <subject>"; use
// switches the log source away from git). Rather than silently dropping
// these on the floor if a future edit to .goreleaser.yaml sets one — the
// failure mode a bare, non-strict yaml.Unmarshal has no way to catch —
// LoadReleaseNotesConfig decodes the full goreleaser v2 Changelog schema with
// KnownFields(true) and rejects any of these five if the committed config
// ever sets them to a non-default value. See goreleaserChangelogFull's doc
// comment and TestLoadReleaseNotesConfig_RejectsUnmodeledChangelogKeys.
package tests

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ReleaseNotesGroup is one entry of .goreleaser.yaml's changelog.groups.
type ReleaseNotesGroup struct {
	Title  string `yaml:"title"`
	Regexp string `yaml:"regexp"` // "" means catch-all, see the package doc.
	Order  int    `yaml:"order"`
}

// ReleaseNotesConfig is the slice of .goreleaser.yaml that decides what the
// published release body says: changelog.sort, changelog.groups and
// changelog.filters.exclude. Every other changelog.* key (use, abbrev,
// format, disable, filters.include) is irrelevant to this project's
// committed config, and LoadReleaseNotesConfig actively REJECTS the file if
// any of them is ever set to a non-default value — see
// goreleaserChangelogFull's doc comment for why silently ignoring them, the
// previous behavior, was the bug.
type ReleaseNotesConfig struct {
	Sort    string              `yaml:"sort"`
	Groups  []ReleaseNotesGroup `yaml:"groups"`
	Filters ReleaseNotesFilters `yaml:"filters"`
	// Footer is release.footer, NOT a changelog: key — it is carried here
	// because it changes the published BODY, which is what this predictor
	// answers about. LoadReleaseNotesConfig admits it only when it contains no
	// template, so PredictReleaseNotes can append it byte for byte; see that
	// function's refusal for what a templated one would cost.
	Header string `yaml:"-"`

	// Footer is release.footer, NOT a changelog: key.
	Footer string `yaml:"-"`
}

// ReleaseNotesFilters is changelog.filters — only its Exclude half, see
// ReleaseNotesConfig's doc comment.
type ReleaseNotesFilters struct {
	Exclude []string `yaml:"exclude"`
}

// goreleaserChangelogFull is the FULL schema of .goreleaser.yaml's
// `changelog:` stanza, named field for field after
// goreleaser/goreleaser/v2@v2.17.1's own pkg/config.Changelog struct
// (Filters, Sort, Disable, Use, Format, Groups, Abbrev — no more, no fewer;
// fetched from github.com/goreleaser/goreleaser at that tag, the same
// ground-truth source release_notes_predict_lib_test.go's package doc cites
// throughout). LoadReleaseNotesConfig decodes the committed file's
// "changelog:" node into THIS struct with yaml.Decoder.KnownFields(true), so
// a key goreleaser v2 does not even recognize fails to decode at all, and
// then separately rejects any of the five FIELDS this predictor's algorithm
// does not implement (abbrev, filters.include, disable, format, use) if the
// file ever sets one away from its default — a key that decodes fine but is
// silently dropped by the narrowing to ReleaseNotesConfig below is exactly
// the finding this closes: TestLoadReleaseNotesConfig_RejectsUnmodeledChangelogKeys
// pins all five.
//
// Disable is modeled as `interface{}`, not goreleaser's own `string`: v2's
// jsonschema tag on that field is `oneof_type=string;boolean` (a raw YAML
// `true`/`false` is common in practice, alongside a templated string like
// `"{{ .Env.SKIP_CHANGELOG }}"`), and yaml.v3 refuses to decode a `!!bool`
// scalar into a Go `string` field under KnownFields — this predictor does
// not need to interpret Disable's value, only detect that it is set away
// from goreleaser's own default (empty string / false), so `interface{}`
// accepts either shape without that decode failure being a false negative
// for "the file is fine".
type goreleaserChangelogFull struct {
	Sort    string              `yaml:"sort"`
	Disable interface{}         `yaml:"disable"`
	Use     string              `yaml:"use"`
	Format  string              `yaml:"format"`
	Groups  []ReleaseNotesGroup `yaml:"groups"`
	Abbrev  int                 `yaml:"abbrev"`
	Filters struct {
		Include []string `yaml:"include"`
		Exclude []string `yaml:"exclude"`
	} `yaml:"filters"`
}

// goreleaserReleaseFull is the slice of .goreleaser.yaml's `release:` stanza
// this predictor cares about, decoded with the same KnownFields(true) rigor
// goreleaserChangelogFull applies to `changelog:` — a rigor that, until this
// finding, stopped at the changelog stanza and left `release:` completely
// unread. It is not goreleaser v2.17.1's full pkg/config.Release struct
// (fetched at that tag from github.com/goreleaser/goreleaser/pkg/config;
// that struct has ~17 fields, most of them cosmetic to the RELEASE — a
// title template, which artifacts attach, a discussion category — and never
// touch the published BODY this predictor exists to predict). Two groups of
// fields are modeled here instead:
//
//   - GitHub, Draft, Prerelease: the fields this project's committed file
//     actually sets today. KnownFields(true) needs them named or the base
//     fixture itself would fail to decode.
//   - Header, Footer: fields the committed file sets, as literals, which this
//     predictor reproduces verbatim — Header beside the body and Footer inside
//     it. Disable, Mode: fields the committed file does NOT set,
//     but which change what gets PUBLISHED in a way this predictor's
//     algorithm does not implement, so they must be recognized (not folded
//     into "unknown key" rejection) and then explicitly checked against
//     goreleaser's own default:
//   - internal/pipe/release/body.go's describeBody wraps this
//     predictor's own Body — ctx.ReleaseNotes — in a template,
//     `{{ with .Header }}{{ . }}\n{{ end }}{{ .ReleaseNotes }}{{ with .Footer }}\n{{ . }}{{ end }}`,
//     reading release.header/release.footer verbatim (both empty by
//     default, so the template contributes nothing beyond
//     ReleaseNotes — the whitespace-only difference this adds even at
//     the default is already covered by PublishedBodyMatches's
//     trailing-whitespace trim). A non-empty header lands BEFORE
//     "## Changelog" and would be silently accepted by
//     PublishedBodyMatches as an "expected hand-written prefix" (see
//     that function's own doc comment) — exactly the failure this
//     predictor exists to catch, not produce. NOTE: goreleaser's
//     --release-header / --release-header-tmpl CLI flags (consumed
//     inside internal/pipe/changelog/changelog.go, a SEPARATE
//     header/footer pair from release.header/release.footer, sourced
//     from ctx.ReleaseHeaderFile/Tmpl rather than this YAML stanza) are
//     a different mechanism this predictor does not need to model at
//     all: .github/workflows/release.yml invokes goreleaser as exactly
//     `release --clean`, with no such flag ever passed, so those
//     context fields are always empty strings for every release this
//     project has ever cut or ever will while that workflow file is
//     unchanged.
//   - release.disable skips the entire release pipe (no GitHub release is
//     created at all), the same shape changelog.disable already guards —
//     predicting a body for a release that GoReleaser was told not to
//     publish is a wrong answer dressed as a right one, same as an
//     unmodeled changelog key.
//   - release.mode (goreleaser's ReleaseNotesMode: keep-existing / append /
//     prepend / replace, default "keep-existing" per goreleaser's own
//     jsonschema default tag) governs how a PRE-EXISTING GitHub release at
//     the same tag is merged with the newly generated notes. This
//     predictor's single "here is what publishes" answer only holds when
//     there is nothing pre-existing to merge with — the ordinary case for
//     every tag this project has ever pushed — so a non-default mode must
//     be rejected rather than silently mispredicted.
type goreleaserReleaseFull struct {
	GitHub struct {
		Owner string `yaml:"owner"`
		Name  string `yaml:"name"`
	} `yaml:"github"`
	Draft      bool        `yaml:"draft"`
	Prerelease string      `yaml:"prerelease"`
	Header     string      `yaml:"header"`
	Footer     string      `yaml:"footer"`
	Disable    interface{} `yaml:"disable"`
	Mode       string      `yaml:"mode"`
}

// disableIsDefault reports whether v — goreleaser v2's own `disable` value,
// decoded loosely as `interface{}` — is at goreleaser's default (the
// changelog step runs): unset, an empty string, the string "false", or the
// boolean false. Anything else (bool true, a non-empty templated string, a
// number, a list) means the file is opting into behavior this predictor does
// not implement.
func disableIsDefault(v interface{}) bool {
	switch t := v.(type) {
	case nil:
		return true
	case bool:
		return !t
	case string:
		return t == "" || t == "false"
	default:
		return false
	}
}

// topLevelNode returns the value node of root's top-level key named name.
// root must be the *yaml.Node a plain yaml.Unmarshal into a fresh Node
// produces (a DocumentNode whose Content[0] is the top-level mapping) — this
// exists solely so LoadReleaseNotesConfig can re-marshal just one sub-tree at
// a time (first "changelog:", now also "release:" — see
// goreleaserReleaseFull's doc comment) and decode EACH strictly, rather than
// requiring one struct to enumerate every unrelated top-level .goreleaser.yaml
// key (builds, archives, checksum) that KnownFields(true) on the WHOLE
// document would otherwise demand.
func topLevelNode(root *yaml.Node, name string) (*yaml.Node, bool) {
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return nil, false
	}
	doc := root.Content[0]
	if doc.Kind != yaml.MappingNode {
		return nil, false
	}
	for i := 0; i+1 < len(doc.Content); i += 2 {
		if doc.Content[i].Value == name {
			return doc.Content[i+1], true
		}
	}
	return nil, false
}

// LoadReleaseNotesConfig reads goreleaserPath's changelog: AND release:
// stanzas. It fails loudly on a missing or unparsable file rather than
// falling back to a zero value — a predictor silently running with NO groups
// and NO excludes would dump every commit into one ungrouped list and call
// that "the prediction", which is a wrong answer dressed as a right one — and
// it fails loudly, in exactly the same way, when either stanza sets a key
// this predictor's algorithm does not implement (see goreleaserChangelogFull
// and goreleaserReleaseFull's doc comments): a silent narrowing to
// ReleaseNotesConfig's three modeled fields would produce a plausible,
// confidently wrong prediction instead.
//
// The release: stanza check exists because, until this finding, the rigor
// above applied ONLY to changelog: — release.header, release.footer and
// release.mode each change the published body in a way this predictor does
// not implement, and nothing read that stanza at all to notice.
func LoadReleaseNotesConfig(goreleaserPath string) (ReleaseNotesConfig, error) {
	raw, err := os.ReadFile(goreleaserPath)
	if err != nil {
		return ReleaseNotesConfig{}, fmt.Errorf("read %s: %w", goreleaserPath, err)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return ReleaseNotesConfig{}, fmt.Errorf("parse %s: %w", goreleaserPath, err)
	}
	node, ok := topLevelNode(&root, "changelog")
	if !ok {
		return ReleaseNotesConfig{}, fmt.Errorf("%s: no top-level \"changelog:\" key found; either the file changed shape or the path is wrong", goreleaserPath)
	}

	// Re-marshal just the changelog sub-tree and decode IT strictly, so an
	// unrecognized key inside "changelog:" fails to decode at all — a plain,
	// non-strict yaml.Unmarshal (the previous implementation) drops any key
	// goreleaserChangelogFull doesn't name on the floor without a trace.
	changelogYAML, err := yaml.Marshal(node)
	if err != nil {
		return ReleaseNotesConfig{}, fmt.Errorf("%s: re-marshal changelog node: %w", goreleaserPath, err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(changelogYAML))
	dec.KnownFields(true)
	var full goreleaserChangelogFull
	if err := dec.Decode(&full); err != nil {
		return ReleaseNotesConfig{}, fmt.Errorf("%s: changelog: has a key this predictor does not recognize (%w) — teach LoadReleaseNotesConfig and PredictReleaseNotes about it before trusting a prediction against this config", goreleaserPath, err)
	}

	// Every key this predictor recognizes but does NOT implement must still
	// be at goreleaser's own default, or the prediction below would be
	// silently wrong: abbrev shortens every sha, filters.include REPLACES
	// exclude filtering outright (changelog.go returns before Exclude ever
	// runs), disable skips the changelog step entirely, format replaces "*
	// <sha> <subject>", and use switches the log source away from git.
	switch {
	case full.Abbrev != 0:
		return ReleaseNotesConfig{}, fmt.Errorf("%s: changelog.abbrev is set to %d; this predictor always emits the full 40-character sha and does not model abbreviation", goreleaserPath, full.Abbrev)
	case len(full.Filters.Include) > 0:
		return ReleaseNotesConfig{}, fmt.Errorf("%s: changelog.filters.include is set (%v); goreleaser applies ONLY Include when it is non-empty (Exclude never runs), which this predictor does not model", goreleaserPath, full.Filters.Include)
	case !disableIsDefault(full.Disable):
		return ReleaseNotesConfig{}, fmt.Errorf("%s: changelog.disable is set to %v; this predictor always assumes the changelog step runs", goreleaserPath, full.Disable)
	case full.Format != "":
		return ReleaseNotesConfig{}, fmt.Errorf("%s: changelog.format is set to %q; this predictor always renders \"* <sha> <subject>\" and does not model a custom format", goreleaserPath, full.Format)
	case full.Use != "" && full.Use != "git":
		return ReleaseNotesConfig{}, fmt.Errorf("%s: changelog.use is set to %q; this predictor only models \"git\"", goreleaserPath, full.Use)
	}

	if len(full.Groups) == 0 {
		return ReleaseNotesConfig{}, fmt.Errorf("%s: changelog.groups is empty; either the file changed shape or the path is wrong", goreleaserPath)
	}

	// --- release: stanza — see goreleaserReleaseFull's doc comment for what each
	// of header/footer/disable/mode has to be. Header and footer are SET in the
	// committed file and required to be LITERAL rather than templated; disable and
	// mode are required to be at their DEFAULTS, which is not the same as absent —
	// disableIsDefault accepts nil, false and "false", and an explicit
	// `mode: keep-existing` is accepted too. An earlier version of this comment
	// said all four had to be at their defaults, which stopped being true when
	// v0.5.2 set the first two; the version after that said disable and mode had
	// to be absent, which was never true. ---
	relNode, ok := topLevelNode(&root, "release")
	if !ok {
		return ReleaseNotesConfig{}, fmt.Errorf("%s: no top-level \"release:\" key found; either the file changed shape or the path is wrong", goreleaserPath)
	}
	releaseYAML, err := yaml.Marshal(relNode)
	if err != nil {
		return ReleaseNotesConfig{}, fmt.Errorf("%s: re-marshal release node: %w", goreleaserPath, err)
	}
	relDec := yaml.NewDecoder(bytes.NewReader(releaseYAML))
	relDec.KnownFields(true)
	var rel goreleaserReleaseFull
	if err := relDec.Decode(&rel); err != nil {
		return ReleaseNotesConfig{}, fmt.Errorf("%s: release: has a key this predictor does not recognize (%w) — teach LoadReleaseNotesConfig about it before trusting a prediction against this config", goreleaserPath, err)
	}
	switch {
	// A HEADER IS MODELLED ON EXACTLY THE FOOTER'S TERMS, and for the same
	// reason: goreleaser composes header and footer into the release BODY at
	// publish time, so a template in either first renders on the publish path,
	// where this project has no undo. Literal only. This used to be a flat
	// refusal of any header at all, which was right while the config set none —
	// v0.5.2 sets one, so the refusal narrows to the part that is actually
	// unsafe rather than blocking the field.
	case strings.Contains(rel.Header, "{{"):
		return ReleaseNotesConfig{}, fmt.Errorf("%s: release.header contains a Go template (%q).\n"+
			"Same contract as the footer: this predictor reproduces it verbatim and has no template engine, and nothing catches a broken template before publish — `goreleaser check` validates one naming a field that does not exist, `goreleaser release --skip=publish` exits 0 over it, and it never reaches dist/CHANGELOG.md because header and footer are composed into the release BODY. Use a literal",
			goreleaserPath, rel.Header)
	// A FOOTER IS MODELLED, BUT ONLY A LITERAL ONE, and the distinction is the
	// whole reason this stays a refusal rather than becoming a passthrough.
	//
	// goreleaser applies its template engine to the body it composes, so a
	// footer containing `{{ ... }}` publishes something other than the bytes in
	// this file — and PredictReleaseNotes has no template engine, so its
	// prediction would be the unrendered source. The two would differ on every
	// release, and the difference would be reported as a release-notes mismatch
	// against a config that is perfectly fine. Worse in the other direction: a
	// footer template naming a field that does not exist is caught by NOTHING
	// before publish. Measured with the pinned v2.17.1 — `goreleaser check`
	// validates it, `goreleaser release --skip=publish` exits 0 over it, and it
	// never reaches dist/CHANGELOG.md, because header and footer are composed
	// into the RELEASE BODY and that file is the changelog. The first render of
	// a footer template is the published page, after the tag is on the forge.
	//
	// So the contract is: a footer with nothing to resolve, which this predictor
	// can then reproduce byte for byte.
	case strings.Contains(rel.Footer, "{{"):
		return ReleaseNotesConfig{}, fmt.Errorf("%s: release.footer contains a Go template (%q).\n"+
			"This predictor reproduces the footer verbatim and has no template engine, so it would predict the unrendered source and report a mismatch on every release.\n"+
			"And nothing catches a BROKEN template before publish: `goreleaser check` validates one naming a field that does not exist, `goreleaser release --skip=publish` exits 0 over it, and it never appears in dist/CHANGELOG.md — header and footer are composed into the release BODY at publish time, and that file is the changelog. The first render would be the published page, after the tag is public.\n"+
			"Use a literal. The CHANGELOG is cumulative, so a link to it on main carries this release's entry and every later one, which is what a reader deciding whether to upgrade needs — a tag-pinned URL buys nothing and costs the only unverifiable moving part",
			goreleaserPath, rel.Footer)
	case !disableIsDefault(rel.Disable):
		return ReleaseNotesConfig{}, fmt.Errorf("%s: release.disable is set to %v; this predictor always assumes GoReleaser actually publishes a release", goreleaserPath, rel.Disable)
	case rel.Mode != "" && rel.Mode != "keep-existing":
		return ReleaseNotesConfig{}, fmt.Errorf("%s: release.mode is set to %q; this predictor assumes the default \"keep-existing\" (no pre-existing GitHub release at the tag being predicted) and does not model append/prepend/replace", goreleaserPath, rel.Mode)
	}

	return ReleaseNotesConfig{
		Sort:    full.Sort,
		Groups:  full.Groups,
		Filters: ReleaseNotesFilters{Exclude: full.Filters.Exclude},
		Header:  rel.Header,
		Footer:  rel.Footer,
	}, nil
}

// ReleaseNotesCommit is one surviving or dropped commit, in the raw
// "<sha> <subject>" shape git log --pretty=oneline produces — sha in FULL
// (40 hex characters), matching GoReleaser v2's git changeloger, which does
// not abbreviate.
type ReleaseNotesCommit struct {
	SHA     string `json:"sha"`
	Subject string `json:"subject"`
}

// DroppedCommit is a commit filters.exclude removed before grouping — the
// half of the prediction that a name-only "what got published" view cannot
// see, and that surfaces.yaml's release-notes entry calls out by name: a
// user-visible change landing under a `docs:` or `chore:` subject (or now, a
// merge commit's own subject) is dropped here and invisible on the release
// page.
type DroppedCommit struct {
	ReleaseNotesCommit
	ExcludedBy string `json:"excluded_by"` // the filters.exclude pattern that matched
}

// PredictedGroup is one rendered "### <title>" section, in publish order
// (i.e. already sorted by the group's `order` field, and omitted from the
// prediction entirely when it claimed no commits).
type PredictedGroup struct {
	Title   string               `json:"title"`
	Order   int                  `json:"order"`
	Commits []ReleaseNotesCommit `json:"commits"`
}

// ReleaseNotesPrediction is what PredictReleaseNotes returns: Body is the
// exact markdown GoReleaser's "git" changeloger generates (see
// internal/pipe/release/body.go in the goreleaser module), plus the
// Groups/Dropped breakdown a gate agent diffs and reasons over without
// re-parsing the markdown, and the header carried beside it.
//
// That parenthesis used to end "— this project's .goreleaser.yaml sets neither
// release.header nor release.footer, so GoReleaser itself hands ctx.ReleaseNotes
// to the release pipe unmodified". v0.5.2 set both, and the sentence stayed. It
// is corrected here rather than deleted because it names the mechanism the
// header and footer are modelled ON: goreleaser wraps the generated notes as
// `{{ with .Header }}{{ . }}\n{{ end }}{{ .ReleaseNotes }}{{ with .Footer }}\n{{ . }}{{ end }}`,
// which is why the footer lands inside Body and the header cannot.
//
// Body is NOT, in general, the entire published GitHub release body byte for
// byte, and G3 must not compare it that way. docs/RELEASING.md's own
// checklist requires "breaking changes and silent-behaviour changes are
// called out first" in the CHANGELOG.md entry, and this project's release
// process carries that same hand-written prose onto the published GitHub
// release page ahead of the generated section for a breaking release — this
// repository's own v0.5.0 is exactly that case: its published body opens
// with roughly 3,190 characters of hand-written breaking-change prose before
// "## Changelog" begins, prose no predictor can generate because nothing
// about it is a Conventional Commit subject. v0.4.1, an ordinary release,
// has "## Changelog" at byte 0 and matches Body exactly — so a contract of
// "prediction.Body == published body" happens to hold on an unbroken sample
// and silently breaks on the next breaking release, which is precisely a
// false negative CLAUDE.md's "every finding reaches the human" rule exists to
// prevent, not produce. G3 must compare with PublishedBodyMatches below, not
// with ==, reflect.DeepEqual, or PublishedEqual against a second prediction
// (PublishedEqual compares two PREDICTIONS to each other, G1's against G2's —
// it is not, and was never meant to be, a check against the real published
// body, which is Body-shaped only from its own "## Changelog" onward).
type ReleaseNotesPrediction struct {
	Body    string           `json:"body"`
	Groups  []PredictedGroup `json:"groups"`
	Dropped []DroppedCommit  `json:"dropped"`

	// Header is release.header verbatim, carried BESIDE Body rather than inside
	// it. Inside it would fail G3 on every release — PublishedBodyMatches
	// compares from the "## Changelog" anchor onward, and a header sits ahead of
	// that anchor.
	//
	// It is here because of what the v0.5.2 gate reported about v0.5.2 itself:
	// the header is the only client-facing line this release adds that reached NO
	// artifact any reading agent is handed. The footer is client-facing too and
	// was already covered — it lands inside Body, which the release-notes surface
	// reads — and so are the four retired stubs, which reach surface.json. The
	// prediction is what the release-notes surface reads, so a header absent
	// from it is a line that ships to every consumer with nobody having read it.
	// Carrying it does not verify it against the published page — nothing here
	// can, for the anchor reason above — but it is the difference between a
	// reviewed line and an unreviewed one.
	Header string `json:"header"`
}

// mergeExcludePattern is exactly the "^Merge " changelog.filters.exclude
// entry .goreleaser.yaml carries. It is named here, once, so PublishedEqual's
// one deliberate exemption (below) and any test that needs to construct or
// recognize a merge-commit drop can't drift out of sync with each other or
// with the committed config — TestLoadReleaseNotesConfig_MatchesCommittedGoreleaserYAML
// is what pins THAT side of the agreement.
const mergeExcludePattern = "^Merge "

// withoutMergeDrops returns dropped with every entry excluded by
// mergeExcludePattern removed, in place order. It exists so PublishedEqual
// can compare what remains of Dropped exactly, rather than ignoring Dropped
// altogether — see PublishedEqual's doc comment for why the merge commit
// specifically has to be set aside rather than compared.
func withoutMergeDrops(dropped []DroppedCommit) []DroppedCommit {
	out := make([]DroppedCommit, 0, len(dropped))
	for _, d := range dropped {
		if d.ExcludedBy == mergeExcludePattern {
			continue
		}
		out = append(out, d)
	}
	return out
}

// PublishedEqual reports whether p and other would produce the SAME
// published GitHub release — i.e. whether G2's re-run over the merge-commit
// range confirms G1's branch-range prediction. It compares Body and Groups
// in full, and Dropped MODULO the one entry that is structurally allowed to
// differ: the "^Merge "-excluded merge commit itself.
//
// That one entry is range-dependent in a way nothing else in Dropped is: a
// --no-ff merge commit exists ONLY in the merge-commit range (G1's branch
// range runs before the release PR merges, so the merge commit does not
// exist yet to be logged at all), so its mergeExcludePattern-tagged entry can
// appear in one prediction's Dropped and structurally can never appear in
// the other's — even though the two describe the exact same published body,
// because the filter's whole job is to keep that commit out of what
// publishes. A caller that reflect.DeepEqual's two ReleaseNotesPredictions
// across that boundary gets a spurious mismatch on every single release, not
// just ones where something is actually wrong.
//
// EVERY OTHER Dropped entry IS compared. A "^chore:" or "^docs:" commit that
// lands between G1 and G2 — the exact case surfaces.yaml's release-notes
// entry warns about, a user-visible change going missing from the published
// page while its Body-level footprint is zero, because a dropped commit by
// definition never appears in Body — must still fail this check, or G2's
// entire reason to inspect Dropped in the first place is defeated. See
// TestPublishedEqual_CatchesNewlyDroppedCommit for the regression this
// guards: an earlier version of this method excluded Dropped ENTIRELY
// (rather than modulo the merge commit alone) and let exactly that scenario
// through undetected.
func (p ReleaseNotesPrediction) PublishedEqual(other ReleaseNotesPrediction) bool {
	if p.Body != other.Body {
		return false
	}
	// The header publishes with the body, so two predictions that disagree about
	// it describe two different pages — even though nothing downstream compares
	// it against what GitHub actually served.
	if p.Header != other.Header {
		return false
	}
	if len(p.Groups) != len(other.Groups) {
		return false
	}
	for i := range p.Groups {
		a, b := p.Groups[i], other.Groups[i]
		if a.Title != b.Title || a.Order != b.Order {
			return false
		}
		if len(a.Commits) != len(b.Commits) {
			return false
		}
		for j := range a.Commits {
			if a.Commits[j] != b.Commits[j] {
				return false
			}
		}
	}
	pd, od := withoutMergeDrops(p.Dropped), withoutMergeDrops(other.Dropped)
	if len(pd) != len(od) {
		return false
	}
	for i := range pd {
		if pd[i] != od[i] {
			return false
		}
	}
	return true
}

// changelogAnchorPattern finds the generated changelog section's own opening
// line inside a larger string — anchored to the START of a line ("(?m)^")
// so a hand-written prefix that happens to quote the words "## Changelog" in
// running prose (inside a sentence, not as its own line) cannot be mistaken
// for the generated section's actual boundary.
var changelogAnchorPattern = regexp.MustCompile(`(?m)^## Changelog$`)

// PublishedBodyCheck is PublishedBodyMatches's result. It is a struct, not a
// bare bool, because of a finding against an earlier version of this
// function: a single bool forced two very different situations — real drift,
// and a published body that never had a generated section to compare in the
// first place — into the same "false", and a human confirming a BLOCKING G3
// finding cannot tell those apart from the verdict alone. Matched is the one
// bit G3 gates on (true only when the anchor was found AND everything from it
// matched exactly); AnchorFound tells the human WHICH kind of "not matched"
// they are looking at, so the finding they read says the right thing. See
// PublishedBodyMatches's doc comment for the three real causes AnchorFound:
// false can mean, only one of which is this project's own doing.
type PublishedBodyCheck struct {
	Matched     bool
	AnchorFound bool
}

// PublishedBodyMatches is G3's check: it reports whether publishedBody — the
// real body read back from the published GitHub release — is consistent with
// p's prediction. It is deliberately NOT "publishedBody == p.Body": see
// ReleaseNotesPrediction's doc comment for why full equality is the wrong
// check on a breaking release, where this project's own process hand-writes
// prose ahead of the generated section.
//
// The check: publishedBody must contain a line that is exactly "##
// Changelog", and everything from that line onward — trailing whitespace
// trimmed on both sides, since a human editing the release page in GitHub's
// own UI can introduce or remove trailing blank lines that carry no meaning
// — must match p.Body exactly, trailing whitespace trimmed the same way.
// Nothing before that anchor is inspected: a hand-written prefix is
// EXPECTED, not evidence of drift, and auditing its prose content is a
// prose-review job for the human-in-the-loop step CLAUDE.md describes, not
// this predictor's.
//
// A publishedBody with no such anchor at all makes Matched unconditionally
// false, never a vacuous true — CLAUDE.md's "a skip is a failure, not a
// pass" applies here exactly as it does to every other gate check in this
// project. But false is NOT the same claim as "there is drift", and an
// earlier version of this function collapsed the two into one bool. There are
// at least THREE real causes for a missing anchor, not the two this
// function's contract used to enumerate:
//  1. the changelog step never ran (release.disable — now rejected at
//     LoadReleaseNotesConfig, before a wrong prediction is ever produced);
//  2. it ran with a changelog: or release: config this predictor doesn't
//     model (also now rejected at LoadReleaseNotesConfig, for the same
//     reason);
//  3. a human wholesale-replaced the generated section as a deliberate part
//     of publishing the release. RELEASING.md documents no step that does
//     this — it is not a sanctioned procedure today — but it is not
//     hypothetical either: this project's OWN published v0.2.0 and v0.3.0
//     release bodies (confirmed via `gh api
//     repos/BarterX-Tech/dossierx/releases/tags/<tag>`) are entirely
//     hand-written prose with no "## Changelog" line anywhere, even though
//     .goreleaser.yaml carried the same changelog: stanza shape at both tags
//     that generates one. Treating that the same way as cause 1 — the
//     phrasing this function used to imply, "the published release doesn't
//     have what was predicted" — tells the human confirming a BLOCKING
//     finding that the release process is broken, for a release that
//     published exactly as its author intended.
//
// AnchorFound is what lets a caller tell cause 3 apart from a FOURTH, and
// genuinely alarming, shape: the anchor line IS present, but the generated
// section's own content disagrees with the prediction (a commit's subject
// hand-edited after tagging, a filter that behaved differently than
// predicted, ...). That is real drift, and AnchorFound is true for it even
// though Matched is false — the two fields are independent, not one deriving
// from the other. A gate stage reads AnchorFound before deciding what to tell
// the human: false means "confirm this replacement was deliberate", true
// (with Matched still false) means "the release doesn't say what was
// predicted."
func (p ReleaseNotesPrediction) PublishedBodyMatches(publishedBody string) PublishedBodyCheck {
	loc := changelogAnchorPattern.FindStringIndex(publishedBody)
	if loc == nil {
		return PublishedBodyCheck{Matched: false, AnchorFound: false}
	}
	generated := strings.TrimRight(publishedBody[loc[0]:], " \t\n")
	wantBody := strings.TrimRight(p.Body, " \t\n")
	return PublishedBodyCheck{Matched: generated == wantBody, AnchorFound: true}
}

// CompareReleaseNotesPrediction loads a previously recorded
// ReleaseNotesPrediction from recordedPath (written by an earlier
// -release-notes-predict-out capture, i.e. G1's) and reports an error unless
// it is PublishedEqual to fresh (a new prediction over G2's merge-commit
// range). This is the actual, gate-callable G2 check: it is invoked from
// TestPredictReleaseNotesForRange_G1Capture via -release-notes-predict-compare
// so that the equality check runs from a plain `go test -args ...`
// invocation, the same way every other gate check in this package does —
// PublishedEqual on its own was reachable only from within this package's
// own tests, which is not a gate stage any release workflow can call.
//
// A missing or unparsable recordedPath is reported as an error, never as a
// silent pass — an absent G1 recording is not evidence the release notes are
// fine, it is evidence G1 never ran, and CLAUDE.md's rule that a check which
// cannot execute is a FAILED gate applies here exactly as it does anywhere
// else in this project.
func CompareReleaseNotesPrediction(fresh ReleaseNotesPrediction, recordedPath string) error {
	data, err := os.ReadFile(recordedPath)
	if err != nil {
		return fmt.Errorf("read recorded prediction %s: %w", recordedPath, err)
	}
	var recorded ReleaseNotesPrediction
	if err := json.Unmarshal(data, &recorded); err != nil {
		return fmt.Errorf("parse recorded prediction %s: %w", recordedPath, err)
	}
	if !recorded.PublishedEqual(fresh) {
		return fmt.Errorf(
			"recorded prediction at %s does not match the fresh prediction — the published release notes would differ from what was approved.\nrecorded body:\n%s\nfresh body:\n%s\nrecorded dropped (excl. the merge commit): %+v\nfresh dropped (excl. the merge commit): %+v",
			recordedPath, recorded.Body, fresh.Body, withoutMergeDrops(recorded.Dropped), withoutMergeDrops(fresh.Dropped),
		)
	}
	return nil
}

// commitLinePattern splits a "git log --pretty=oneline" line into its sha
// and subject. GoReleaser's own extractCommitInfo does the
// equivalent by splitting on the first space and re-joining the rest; this
// keeps the same contract (a subject may itself contain spaces, a sha never
// does) as a regexp for symmetry with the group/exclude matching below.
var commitLinePattern = regexp.MustCompile(`^(\S+) (.*)$`)

// splitCommitLine parses one "<sha> <subject>" line. A line with no space at
// all (possible only for a commit with a completely empty subject, which git
// refuses to create) is treated as an all-subject, empty-sha line rather than
// panicking — there is no wrong answer to be silently correct about here, and
// erroring would make one malformed line fail an entire range prediction.
func splitCommitLine(line string) ReleaseNotesCommit {
	if m := commitLinePattern.FindStringSubmatch(line); m != nil {
		return ReleaseNotesCommit{SHA: m[1], Subject: m[2]}
	}
	return ReleaseNotesCommit{Subject: line}
}

// PredictReleaseNotes runs the algorithm the package doc describes over
// rawLines — "<sha> <subject>" lines in `git log` order (newest first, the
// order git itself produces) — and cfg, with no git invocation of its own.
// It is separated from PredictReleaseNotesForRange so a caller that already
// has the log lines (a fixed test fixture, or a range fetched once and
// reused) can drive the pure function directly.
func PredictReleaseNotes(rawLines []string, cfg ReleaseNotesConfig) (ReleaseNotesPrediction, error) {
	entries := make([]string, 0, len(rawLines))
	for _, l := range rawLines {
		if strings.TrimSpace(l) != "" {
			entries = append(entries, l)
		}
	}

	// --- filter (changelog.filters.exclude), sequentially, one pattern at a time ---
	var dropped []DroppedCommit
	for _, pattern := range cfg.Filters.Exclude {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return ReleaseNotesPrediction{}, fmt.Errorf("changelog.filters.exclude %q: %w", pattern, err)
		}
		var kept []string
		for _, e := range entries {
			c := splitCommitLine(e)
			if re.MatchString(c.Subject) {
				dropped = append(dropped, DroppedCommit{ReleaseNotesCommit: c, ExcludedBy: pattern})
				continue
			}
			kept = append(kept, e)
		}
		entries = kept
	}

	// --- sort (changelog.sort) ---
	if cfg.Sort == "asc" || cfg.Sort == "desc" {
		sort.SliceStable(entries, func(i, j int) bool {
			si, sj := splitCommitLine(entries[i]).Subject, splitCommitLine(entries[j]).Subject
			if cfg.Sort == "asc" {
				return si < sj
			}
			return si > sj
		})
	}

	// --- group (changelog.groups), config order, catch-all last regardless of position ---
	pool := entries
	groups := make([]PredictedGroup, 0, len(cfg.Groups))
	for _, g := range cfg.Groups {
		pg := PredictedGroup{Title: g.Title, Order: g.Order}
		if g.Regexp == "" {
			for _, e := range pool {
				pg.Commits = append(pg.Commits, splitCommitLine(e))
			}
			pool = nil
		} else {
			re, err := regexp.Compile(g.Regexp)
			if err != nil {
				return ReleaseNotesPrediction{}, fmt.Errorf("changelog.groups %q regexp: %w", g.Title, err)
			}
			var remaining []string
			for _, e := range pool {
				c := splitCommitLine(e)
				if re.MatchString(c.Subject) { // subject only — entry.Message in changelog.go, never the sha-prefixed line; see package doc item 4
					pg.Commits = append(pg.Commits, c)
				} else {
					remaining = append(remaining, e)
				}
			}
			pool = remaining
		}
		groups = append(groups, pg)
		if len(pool) == 0 {
			break
		}
	}
	sort.SliceStable(groups, func(i, j int) bool { return groups[i].Order < groups[j].Order })

	// --- render, exactly as formatChangelog does: "## Changelog", then
	// "### <title>" + "* <sha> <subject>" per non-empty group ---
	lines := []string{"## Changelog"}
	rendered := make([]PredictedGroup, 0, len(groups))
	for _, g := range groups {
		if len(g.Commits) == 0 {
			continue
		}
		rendered = append(rendered, g)
		lines = append(lines, "### "+g.Title)
		for _, c := range g.Commits {
			lines = append(lines, "* "+c.SHA+" "+c.Subject)
		}
	}
	body := strings.Join(lines, "\n") + "\n"

	// --- then the release-level footer, exactly as describeBody does ---
	//
	// goreleaser's internal/pipe/release/body.go wraps the generated notes in
	//   {{ with .Header }}{{ . }}\n{{ end }}{{ .ReleaseNotes }}{{ with .Footer }}\n{{ . }}{{ end }}
	// so a non-empty footer is appended after the changelog with exactly one
	// newline in front of it. That is what is reproduced here, and it is the
	// whole of the footer's effect on the published body: it does not change
	// grouping, ordering or which commits survive, so Groups and Dropped are
	// untouched.
	//
	// This lands in Body rather than beside it because PublishedBodyMatches
	// compares from the "## Changelog" anchor TO THE END of the published body.
	// A footer modelled anywhere else would leave that comparison reporting a
	// mismatch on every release, for a difference the predictor knew about —
	// which is a false finding, and this file exists to prevent those rather
	// than manufacture them.
	// THE HEADER IS DELIBERATELY NOT PREPENDED HERE, and the asymmetry with the
	// footer is a property of what G3 can check rather than an oversight.
	//
	// goreleaser renders release.header BEFORE the "## Changelog" anchor and the
	// footer after it. PublishedBodyMatches finds that anchor in the published
	// body and compares from there onward against Body, ignoring everything
	// ahead of it as an expected hand-written prefix. So a footer inside Body IS
	// verified against the real published page; a header inside Body would make
	// that comparison fail on every release, because Body would no longer start
	// at the anchor.
	//
	// The residual, stated rather than implied: the header's bytes are never
	// compared against the published page. What stands behind them is that a
	// templated header is refused at load (the case above), so what publishes is
	// the literal in .goreleaser.yaml, which is tracked and reviewed like any
	// other line in it. That is weaker than the footer's guarantee and it is the
	// most this check can honestly offer, since the anchor tolerance ahead of it
	// exists for a real reason — this project hand-writes prose above the
	// generated section on a breaking release.
	if cfg.Footer != "" {
		body += "\n" + cfg.Footer
	}

	return ReleaseNotesPrediction{Body: body, Groups: rendered, Dropped: dropped, Header: cfg.Header}, nil
}

// gitLogOneline runs a NARROWED form of the invocation GoReleaser's
// gitChangeloger.Log runs (internal/pipe/changelog/changelog.go and
// internal/git/git.go) — the two fields (sha, subject) this predictor needs,
// against revRange inside root, as plain "%H %s" lines rather than
// GoReleaser's own delimiter-tagged, multi-field format. See the package
// doc's "gitChangeloger.Log itself is NOT field-for-field" paragraph for
// exactly what that narrowing gives up (immunity to ambient git noise other
// than log.showSignature) and why it was judged an acceptable, documented
// limitation rather than reproduced in full.
//
// It does not reproduce the "tags/<prev>..tags/<current>" ref form
// GoReleaser builds when both ends are tags: at prediction time the SECOND
// end is not a tag yet (there is no tag until the release this predicts),
// so the caller passes a plain range — "v0.5.0..HEAD" for G1's branch-tip
// prediction, "v0.5.0..<merge-sha>" for G2's merge-commit-range re-check —
// and git resolves "v0.5.0" against refs/tags identically either way.
//
// Two things this used to get wrong, both load-bearing for why G1 and G2 must
// actually match what publishes rather than agreeing with each other over a
// shared error — see the package doc's item 1 for the full account and
// TestPredictReleaseNotesForRange_ImmuneToAmbientGitConfig (log.abbrevCommit)
// and TestPredictReleaseNotesForRange_ImmuneToAmbientGitConfig_SignedCommits
// (log.showSignature, over real SSH-signed commits — the only ambient-noise
// shape verified to reach exec.Command's stdout, and the one gap this
// predictor's narrower-than-upstream parsing actually has) for the
// regression tests:
//
//   - "-c log.showSignature=false" is prepended here exactly as
//     internal/git.RunWithEnv prepends it to EVERY invocation goreleaser
//     makes (git.go:21-24) — not an incidental flag on this one call.
//     Without it, a signed commit under an ambient `log.showSignature=true`
//     prints a "Good \"git\" signature ..." (or "No signature", or an
//     "error: ...") block ahead of the commit line, ALL THREE confirmed
//     locally to land on stdout — not stderr, where they would be invisible
//     to exec.Command's Output() — which a per-line parse reads as its own
//     malformed entry.
//   - "--pretty=format:%H %s" replaces the previous "--pretty=oneline":
//     %H is unconditionally the full 40-character sha regardless of any
//     abbreviation config (log.abbrevCommit, core.abbrev, an "alias.log"),
//     where --pretty=oneline's sha field tracks that config and only happens
//     to print 40 characters at git's own untouched defaults. GoReleaser's
//     own gitLogFormat names %H explicitly for the same reason. The space
//     between %H and %s reproduces "<sha> <subject>" per line exactly as
//     --pretty=oneline did — git's subject field (%s) is, by git's own
//     definition, always the commit message's first line with no embedded
//     newline, so splitCommitLine's existing first-space split below still
//     parses it correctly; only the two format-string bugs above needed
//     fixing, not the line shape callers already depend on.
func gitLogOneline(root, revRange string) ([]string, error) {
	cmd := exec.Command("git", "-c", "log.showSignature=false", "log", "--no-decorate", "--no-color", "--pretty=format:%H %s", revRange)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		// cmd.Output populates *exec.ExitError.Stderr since Stderr was left
		// nil above; surface it. Without this, a bad revRange (a shallow
		// clone missing the tag it names, a typo) reports only "exit status
		// 128" with no hint which ref git could not resolve.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return nil, fmt.Errorf("git log %s: %w: %s", revRange, err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("git log %s: %w", revRange, err)
	}
	lines := strings.Split(string(out), "\n")
	if n := len(lines); n > 0 && strings.TrimSpace(lines[n-1]) == "" {
		lines = lines[:n-1] // trailing blank line from the final trailing newline, same trim buildChangelog applies
	}
	return lines, nil
}

// PredictReleaseNotesForRange is the entry point a gate stage calls: run
// GoReleaser's real git invocation over revRange inside root, then apply cfg
// exactly as GoReleaser's changelog pipe would. This is the "reusable, not
// one-shot" surface — G1 calls it with the branch range and records the
// result, G2 calls it again with the merge-commit range and feeds the result
// to CompareReleaseNotesPrediction against G1's recording (not
// reflect.DeepEqual — see PublishedEqual's doc comment for why plain
// equality is the wrong check here), and G3 checks the G2 prediction's
// PublishedBodyMatches against the real published release body (not Body ==
// publishedBody — see ReleaseNotesPrediction's doc comment for why plain
// equality is the wrong check there too, on a breaking release).
func PredictReleaseNotesForRange(root, revRange string, cfg ReleaseNotesConfig) (ReleaseNotesPrediction, error) {
	lines, err := gitLogOneline(root, revRange)
	if err != nil {
		return ReleaseNotesPrediction{}, err
	}
	return PredictReleaseNotes(lines, cfg)
}
