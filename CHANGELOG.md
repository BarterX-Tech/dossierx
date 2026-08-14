# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.5.2] - 2026-08-14

**SILENT: your viewer's bytes change on the next `dossierx check`, with no claim edit.** Three
maintainer comments inside the viewer's inlined stylesheet were factually wrong — `graph.css`
described a backdrop dim and a drop shadow the pane has never had, and both z-index ledgers called
the opaque pane root a "backdrop". Correcting them changes the generated `index.html` for every
project, because that CSS is inlined and its comments ship with it. Nothing renders differently to a
reader: no rule, no colour and no layout moved, and the cross-release report classifies all three as
silent precisely because the inputs are byte-identical. If you diff your committed viewer after
upgrading, this is what you are looking at.

**SILENT: the embedded agent skills changed, and nothing on your side reports it.
Re-run `dossierx skills export` after upgrading.** Those bundles are written into a project as
committed artifacts, and nothing in `dossierx check` compares an exported copy against the
binary's. A project that skips the re-export keeps v0.5.1's guidance: an install line fetching
`scripts/install-git-hook.sh` from the v0.5.1 raw path, and a build-order rule with no answer for
a declared contract.

**VISIBLE, and the one thing to check before upgrading: `dossierx version` prints a different
string from the published archive.** v0.5.1's archive printed `0.5.1`; v0.5.2's prints `v0.5.2`.

The same release used to answer that question two ways depending on how it was installed.
`.goreleaser.yaml` stamped `-X main.version={{.Version}}`, which is the tag with its leading `v`
stripped, so the archive printed the bare form. `go install github.com/BarterX-Tech/dossierx/cmd/dossierx@v0.5.1`
applies no ldflags at all, falls back to `debug.ReadBuildInfo`, and gets the tag verbatim from the
module proxy — so that binary printed `v0.5.1`. The practical hazard was never cosmetic: a scripted
`dossierx version --format json | jq -r .data.version` compared against a `vX.Y.Z` tag succeeded via
one install path and failed via the other. The stamp is now `{{.Tag}}`, both paths print the tag
exactly as tagged, and that is the form the git tag and this file's own headings already used. The
site's `dossierx version` transcript DID move with it: v0.5.1 deliberately rendered the v-stripped
form, and it now renders the tag verbatim like everything else. **If your tooling compares that field against a bare `X.Y.Z`, this is the
release where it changes.** Nothing in `cmd/dossierx` moved to achieve it — `resolveVersionInfo`
already produced the tag verbatim, so the two paths converge rather than being reconciled.

**No new or changed command, flag, `error.code`, lint rule or schema field**, and the noun and leaf
counts are where they were — nineteen under seven. What did move is the RETIRED spelling set, from
twelve to sixteen, so four invocations the binary used to reject blankly now answer properly; see
below. Two engine behaviours also move: the version string above, and the Windows claims file lock described under **Fixed** below, which stops
refusing the contended case it exists for. If you are on Windows and two `dossierx` invocations can
touch one project at once, read that entry — it is a fix, not a nicety.

Beyond those two, nothing a consumer runs behaves differently. The remaining engine changes are
comment-only: five doc comments naming a verb this CLI has not had since the noun surface landed —
`dossierx flag`, where the verb is `dossierx claim flag` — one that described the `mockup_modules`
allowlist as gating `layout: mockup` when v0.4.1 widened it to gate any claim carrying `raw_html` on
any layout, and and one in `retired.go` that gave a false reason for four verbs having no stub — see below, where
the stubs are.

### The release pipeline can complete unattended

This is what the release is for. The driver pushes the tag first, verifies the six archives, and
pushes main last — deliberately, so the site never announces a release whose archives do not exist
yet. The forge's gate then required the tagged commit to be reachable from `origin/main`. Each
guard is right on its own and proven by its own mutation test; together they deadlock. The gate job
fires on the **tag** push and asks there for a branch the driver pushes two steps later, so it
refuses, so no archives are built, so the driver waits for archives that can never exist, so main
never moves. Nothing in that ring can move first and no timeout resolves it. Measured in v0.5.1:
the driver polled for twenty minutes and stopped with the tag public and nothing else done, and a
human finished that release by hand following the driver's printed recovery.

Reachability is replaced by a fact the gate can establish at tag-push time holding nothing but the
tag: **the tagged commit must be a merge.** A real release always passes, because the driver merges
with `--no-ff` and tags that merge by value.

The cost is stated rather than described as fixed, in `.github/workflows/release.yml`'s header and
in `docs/RELEASING.md`. Reachability refused a tag on any commit not on main; merge-ness refuses a
tag on a single-parent commit. So this still closes the failure the old check was written for —
tagging the release branch instead of the merge — and no longer closes a merge commit created
locally and never pushed. The release stamp does not cover that either: a branch ready to merge
already carries this release's stamp, which is exactly when it would be mistagged. And the gate
receipt cannot close it from the forge, because the receipt is never committed — `gate/.gitignore`
ignores every run-produced *evidence* artifact on purpose, so that a copy left on disk cannot look
authoritative. (The subject freeze added later in this release is the one run-written file that is
tracked, and it is evidence about the QUESTION rather than about the answer.) A forge-side tag protection rule on `v*` is what would close it, which is the same
accepted residual that file already recorded.

Nothing pinned the forge's guard list, so deleting a guard was invisible to `go test ./...` and so
is restoring one. `TestTheReleaseGateDoesNotAskTheForgeForOriginMain` now refuses both spellings of
the restoration and requires the replacement to still be present.

### The gate stopped re-reporting a class of finding about itself

`surfaces.yaml` claims every tracked file for exactly one surface, and each reviewing agent is
handed that one surface's documents split into handed and withheld. So a sentence in
`CONTRIBUTING.md` whose truth turns on `docs/RELEASING.md` was unanswerable **by construction**:
that file belongs to another surface, so it was not handed, not withheld, and not present at all.
The frame tells an agent that a byte it does not hold is never a reason to guess and never a reason
to pass, so every gate round produced the same findings — correct each time, and about the gate's
own material rather than about the release.

A surface entry may now carry a `reads:` list of exact repository-relative paths it does not own
but needs. Those bytes are handed over as context and marked as belonging to another surface.
Ownership is untouched: `reads:` takes no part in the manifest's exactly-one rule, and the
assembler refuses any overlap with the surface's own documents. An unresolvable entry refuses the
whole fan-out rather than producing a shorter bundle, because a bundle assembled over less than it
should be still hashes, still looks like a match, and still carries a verdict forward.

The frame now distinguishes three states rather than two — yours and handed, yours and withheld,
handed as context and not yours. Without that an agent reviews another surface's file under its own
name, and since nothing is filtered on the way to the human that finding arrives twice, attributed
to the wrong surface.

### The gate stopped needing four rounds to say the same thing

This release's own gate ran four reading rounds and returned 39, 31, 24 and 18 findings — decaying
and not converging. Three mechanisms address the three reasons, and none of them relaxes a refusal:
coverage is not reduced, a check that cannot run still fails, every finding still reaches the human
unfiltered, and the receipt still evaluates FAILED on any finding at all.

**Prose that restates a number the tree already derives is now checked by a test, not by an agent.**
Two such sentences had already gone wrong inside this one release. The site's "26 → 19: what was
cut" table enumerated twelve retired commands while the binary retires sixteen — `lock`, `unlock`,
`reaudit` and `flag` were missing, and every one of them was a real top-level command until v0.3.0
folded them under the claim noun. The table gains a row naming all four. `docs/RELEASING.md`'s
"FIVE pins across FOUR files" is the same shape, and its own paragraph admits the list "has now gone
stale twice". Both are now compared against `surface.json`, which is regenerated by a test and
byte-compared in CI, so those two sentences cannot go stale again. The wider class — prose restating
any number the tree derives — is not closed by two checks, and the site entry says so in the same
words. A consumer sees
one change from this: the migration table on the site is complete.

**A release now freezes the question its rounds are counted over.** `surfaces.yaml` grew during
rounds 1, 2 and 4 of this release — real coverage, arriving mid-count, which makes the finding curve
unreadable: a smaller round might mean a better tree or a narrower question, and nothing on the
record said which. The first fan-out of a release records the manifest's digest in a tracked
`gate/subject.json`; later rounds refuse if it moved. This is not narrowing. Coverage stays where
round one set it, a gap found later is written down as a finding against the next release rather
than dropped, and a maintainer who rules one blocking thaws deliberately — new digest, stated
reason. It does not re-read all thirteen surfaces — no stage-2 key hashes the manifest — but it
makes the widening visible, dated and reasoned instead of absorbed into the next round's count.

**A fix wave is read before a full round pays to discover what it broke.** Every round of this
release after the first opened by repairing the round before it; round four's three highest-severity
findings were all introduced by round three's fixes. `run.sh wave --range A..B` hands two agents the
diff plus the full text of every file the wave touched — a sentence is false or true against the
paragraph around it, and a hunk hides that paragraph. Its answer is advice to the agent writing the
wave: keyed to a range rather than to a tree, filed nowhere, reaching no receipt. A clean wave read
means "no regression found in this diff" and never "this surface passes".

None of this is in the shipped binary. It changes how this project is released, not what you run.

### Fixed

- **The release page never showed the BREAKING notice.** A published release page carried grouped
  commit subjects and nothing else, so v0.5.0's breaking change appeared as an ordinary Features
  bullet. Every page now carries a footer pointing at this file. The footer resolves no template,
  and that is deliberate: goreleaser composes header and footer into the release **body** at publish
  time, so a broken template is caught by nothing — `goreleaser check` validates one naming a field
  that does not exist, and `--skip=publish` never composes a body at all. The release-notes
  predictor models the literal footer and refuses a templated one. (#33)
- **A release branch named `vX.Y.Z` collides with the tag it becomes.** `git push origin v0.5.0`
  failed outright with `src refspec … matches more than one`, and `git rev-parse v0.5.0^{commit}`
  succeeded by git's search order while the branch pointed elsewhere. The procedure's commands are
  fully qualified, and it now refuses the colliding branch name up front. (#40)
- **`build_role` had no phase for a declared contract.** `schema` is documented as covering anything
  that must exist before the things below it can conform to it, which includes a signature an
  implementation is written against. No code changes and no corpus revalidates. (#31)
- **The site under-described what shipped.** The `serve` card omitted `GET /api/graph`, and the
  claims graph appeared nowhere outside its own release entry. (#34)
- **A local green said less than it looked like.** `CONTRIBUTING.md` now states that the CI hook
  matrix outranks a local `make hook-test`, and that a green against a Chromium fork is not a green
  against Chrome. (#36)
- **Two test guards advertised more than they checked.** The summary dash guard promised to refuse
  both the en dash and the em dash and checked one — and could not check the other against rendered
  output, since the em dash is live in the same emitter. The `</details>` ordering probe found the
  first closer in the output, which is the footer's only because that fixture claim carries an edge.
  (#29, #28)
- **A gate test reported findings about a file it never touched.** In a checkout where
  `make ci-evidence` had already written a fan-out record, all five refusal rows of the fanout
  flag-contract test failed, each claiming a run had written `gate/fanout.json` when it had written
  nothing.
- **`dossierx lock`, `unlock`, `flag` and `reaudit` answered with nothing useful.** Each moved under
  the `claim` noun in v0.3.0, and `retired.go` — the file whose whole job is to answer a remembered
  invocation with its replacement — left all four out, on the recorded grounds that the root already
  answered them with a hint. It does not: cobra rejects an unknown command before the hint-bearing
  branch runs, so what a caller got was `{"ok":false,"command":"","error":{"code":"usage","message":
  "unknown command \"lock\" for \"dossierx\""}}` — no hint, no replacement named, empty command
  field. All four are now stubs naming `dossierx claim <verb>`. The retired set goes from twelve
  spellings to sixteen; `counts` is untouched, because a retired stub enters `retired` and never
  `commands`, which is also why this is a patch and not a minor.
- **The `cli-operator` subject vocabulary had no value for a CI check**, forcing a hand override at
  every release. It gains `ci`.
- **A serve test raced the server it started, and only Windows noticed.** The watcher re-walks the
  claims tree with `filepath.WalkDir` twice a second, which holds a directory handle. POSIX unlinks
  a directory out from under one; Windows refuses with "The process cannot access the file because
  it is being used by another process", so `TestClaimAsset_SymlinkedDirectoryIsRefused` reddened the
  `windows-latest` leg and, through it, CI on `main` — which is a refused release, since
  `make ci-evidence` fails on any failed test. The removal now retries inside a bounded window and
  then **fails**: giving up quietly would leave the fixture directory standing, the symlink never
  created, and the assertion passing against an ordinary missing file. Mutation-checked — with the
  removal silently skipped, the test reports SKIP and the package prints `ok`.

### Fixed — the claims file lock could refuse the one case it exists for, on Windows

**If you run DossierX on Windows and two `dossierx` invocations can touch one project at the same
time — a parallel script, a CI matrix, an agent and a human — this is the fix to have.**
`AcquireFileLock` serialises concurrent CLI runs against a project's lock ledger with an `O_EXCL`
sentinel file, retrying until the holder releases. It retried only when the open failed with "already
exists".

Windows does not remove a deleted file's directory entry when the unlink returns. The entry survives
in a **delete-pending** state until the last handle closes, and while it is there every open of that
path fails with `ERROR_ACCESS_DENIED` rather than `ERROR_FILE_EXISTS`. That state begins at the exact
instant the holder releases the lock — which is the instant a waiter is polling. `os.IsExist` is false
for it, so the waiter stopped retrying and returned `acquire file lock …: Access is denied.` at the
one moment it should have waited one more poll.

The failure is intermittent by construction, which is how it survived: it needs the waiter's poll to
land inside the holder's release. It reddened a `windows-latest` leg of this release's own CI, in
`TestConcurrentClaimWritersNeverCorruptClaimFiles`, having passed the two runs before it.

POSIX never reaches this — `unlink` is atomic, so `EACCES` on a lock path is a real permission problem
that should fail fast rather than spin. The classification is therefore per-platform and is pinned as
such, and the two contended timeouts now report apart: a holder that never let go names the holder and
the manual recovery, while a path that stayed unopenable for the whole window says so and points at
permissions instead of at a process that was never there.

### Fixed — found by this release's own reading gate

Thirteen agents read the thirteen declared surfaces against this tree and returned 39 findings. The
ones a consumer can observe:

- **The hook installer told you to run a file you do not have.** Every recovery it printed named
  `scripts/install-git-hook.sh`, which is where that file lives in *this* repository. The ordinary
  reader curls one file into their own project, so the single instruction offered to somebody whose
  hook had just been refused was a file-not-found. It now names the invocation you actually used, or
  the pinned raw URL when it was piped from stdin and there is no `$0` to name. The same fix reaches
  the "add the CI workflow" message, which named a path in this repository and gave no way to obtain
  it. This is the defect v0.5.1 fixed inside the hook body and left standing in the installer around
  it.
- **A global `core.hooksPath` install is now stated to be machine-wide.** The installer reads that
  setting rather than assuming `.git/hooks`, which is right — but when the value comes from your
  global or system config, the hook it writes runs for every repository on the machine, and the note
  only named the setting and the command to inspect it. It now says so outright, and names the
  uninstall command that removes the same path. The behaviour is unchanged and deliberate: repointing
  `core.hooksPath` would silently disable every other hook you run.
- **Two shipped skill bundles cited a file that never ships with them.** `dossierx-claims` and
  `dossierx-comments` both defer to `FORMAT.md` for the markdown ceiling and for the in-repo-ledger
  principle. `dossierx skills export` writes the SKILL.md tree, the AGENTS.md section and the agent
  guide — never `FORMAT.md`. Both now link it at this release's tag, which is the convention the
  router bundle already used for the two other files it knows are absent.
- **`dossierx claim new --help` promised a card layout for a facet that gets a banner.** A claim in
  the reserved `overview` facet is written as `layout: banner` when `--layout` is not given, because a
  card there fails `orientation-note-shape`. The behaviour was correct and the help text described the
  behaviour it replaced.
- **`mailto:` was the one allowlisted link scheme nothing pinned.** `FORMAT.md` promised it twice and
  the construct corpus had no case for it — every other allowed scheme and every rejected one did. It
  renders correctly; it is now pinned as `link-mailto`, so the promise cannot quietly stop being true.
- **`FORMAT.md` stated an absolute about tables that its own corpus contradicts.** "A well-formed
  table is always rendered as a table" is true of size and shape but not of position: a table indented
  at the top level is prose, which is a separate rule from the list-item one and was undescribed. The
  blockquote section had the matching gap — it described only the permissive half and never said there
  is no lazy continuation, which CommonMark trains an author to expect.

And the ones only a maintainer sees, each of which is this release being wrong about itself:

- `docs/RELEASING.md` opened by saying the forge checks that the tagged commit is reachable from
  `origin/main` — the check this very release removed, contradicted by the same file further down.
- The gate's own baseline command still passed `surface.baseline.json`, the v0.5.0 bootstrap, with the
  correction sitting *below* the block rather than in it. Somebody running this release's gate copied
  it and caught the mismatch only by reading on. The command now does the right thing and the note
  says when it does not apply.
- Step 1 said a bundle is assembled from four things. Since this release it is five — `reads:`
  documents are the fifth.
- `CHANGELOG.md` had no `[0.5.2]` link definition, so the shipped release was the only version in the
  file whose heading was not a link, and `[Unreleased]` still compared from `v0.5.1`, which would have
  presented this entire release as unreleased the moment it was tagged.
- `CONTRIBUTING.md` never mentioned `make viewer-lint`, so a contributor following it end to end
  linted strictly less than CI does; and its account of `surfaces.yaml` described two kinds of entry
  when this release added a third.
- The `Makefile` comment said the browser tests skip "when `DOSSIERX_TEST_BROWSER` is unset".
  `resolveBrowser` skips only when no browser is found anywhere, and *fails* when the variable is set
  to a path that does not exist. `CONTRIBUTING.md` was right and the Makefile was wrong.
- A second fan-out test carried the same defect this release fixed in its sibling — asserting no
  record was written while a real one sat on disk — and reported a false finding during this release's
  own gate run. The stash is now one shared helper rather than two copies of which one was fixed.
- Twelve surfaces now declare, in `surfaces.yaml`, the documents they read but do not own. Twenty of
  the 39 findings were an agent naming a byte it needed and was not handed; `reads:` is the mechanism
  this release shipped to close exactly that, and it was under-applied at the moment it landed.

The last four the rounds recorded and did not close — and one more this release's own work created:

- **The hook installer's `--help` stopped mid-sentence.** A `sed` range ends at its closing match,
  so the two-line exit-status paragraph printed its first line and dropped the second: the last thing
  a reader saw was "1 declined, refused,". For somebody who curled this file into their own project,
  `--help` is the whole of the documentation.
- **And it mangled a Windows path on the way out.** The invocation was substituted into that text
  with `sed`, whose replacement treats a backslash as an escape — so on the one platform where the
  path always has backslashes, the line whose job is to name a command you can type lost characters.
  Both are now done with literal text handling, and both are pinned by tests.
- **The installer's machine-wide warning was wrong in both directions.** It matched the shape of the
  config path, so a setting scoped to a single *submodule* — whose config lives under the
  superproject's `.git/modules/` — was announced as running for every repository on the machine, and
  `--separate-git-dir` did the same. In the other direction, a git without `--show-origin` returns
  nothing, and nothing was read as "this repository's own", so the warning was skipped exactly when
  the question could not be answered. It now asks git where its own config is, and an origin that
  cannot be read says so instead of passing.
- **`FORMAT.md` credited slug uniqueness to `id-shape`,** which reads one claim at a time and cannot
  compare two. The rule that establishes uniqueness is `ambiguous`. The guarantee was never missing;
  the attribution was, and an implementer would have gone looking in the wrong rule.
- **The one client-facing line this release adds was in no artifact anybody reviews.**
  `release.header` publishes ahead of the `## Changelog` anchor the published-body check compares
  from, so it cannot sit inside the predicted body without failing that check on every release. It is
  now carried beside it, so the reading gate sees the text before it ships.

### Closed without a change

- **#39** asked that the ldflags assertion move off the `go install` path onto the published
  archive. `docs/RELEASING.md` already does exactly that, and the file the issue names was deleted
  with the old checklist.
- **#35** is a stale example version inside that same deleted file.

## [0.5.1] - 2026-08-10

**SILENT: the embedded agent skills changed, and nothing on your side reports it.
Re-run `dossierx skills export` after upgrading.** Those bundles are written into a project as
committed artifacts, and nothing in `dossierx check` compares an exported copy against the binary's
— v0.5.0's entry below says so in as many words. A project that skips the re-export keeps v0.5.0's
guidance, including an install line that fetches `scripts/install-git-hook.sh` from the v0.5.0 raw
path and a stale account of when `dossierx claim flag` is refused.

**Nothing a consumer runs behaves differently.** There is no new or changed command, flag,
`error.code`, lint rule, schema field or rendered-viewer byte, and no engine behaviour moves:
`internal/` is untouched end to end and there is nothing to re-render. What moved outside the
release machinery is the four install pins, now `v0.5.1` — one of them inside
`skills/dossierx/SKILL.md`, which is the byte that makes the exported bundle a different bundle —
and seven wrong strings in what a consumer's own tooling prints or ships: three from the binary,
two carried by the exported skill bundles, and two from the pre-commit hook installer and the hook
it writes.

The three the binary printed each named an invocation it rejects. The retired `implink` stub's
replacement command omitted `--module` and `--claim` and passed the id positionally at a command
declared `cobra.NoArgs`. The missing-`--reason` refusal printed the verb without the id or
`--module` the verb also requires. And `dossierx claim show`'s next action for a drifted
implementation link offered a bare `dossierx claim flag <id>` at a verb that requires
`--claim-says`, `--now-does` and `--reason`, all three, before it does anything at all. All three
now print the whole invocation; `claim show`'s is the one whose own doc comment already promised
"the advice can never disagree with what the command would do", which the other two are only held
to by `internal/cliout`'s definition of a hint.

The two in the bundles: the cross-references between bundles were written as `[[wikilink]]`, which
the two derived export forms rewrote into an anchor and the `SKILL.md` tree — the form Claude Code
actually loads — shipped to a client's agent as the literal characters `[[` and `]]`. Each is now
an ordinary relative link to the sibling bundle, which resolves as written in the exported tree and
is still retargeted to an anchor in the guide and in the `AGENTS.md` section. And the code-links
bundle's account of `dossierx claim flag` still stated v0.4.0's layout rule after v0.4.1 made the
refusal key on content.

The two in the hook: the installer's note used to open "this repository sets
core.hooksPath", when the value is read with a plain `git config --get`, which resolves across every
scope: a `git config --global core.hooksPath ~/.githooks` is an ordinary setup, and that reader was
being sent to look for the setting in a `.git/config` that never mentions it. The note now states
the value, says the setting may be the repository's or the global one, and hands over
`git config --show-origin --get core.hooksPath`, which answers it. Separately, the hook body's
"remove the hook" recovery — printed on two different refusal paths — named
`scripts/install-git-hook.sh --uninstall`, a path that exists in this repository and in no
consumer's: the installer is deliberately one file with the hook embedded so it can be fetched into
a project that has the binary and not this repository, which is the ordinary case and precisely the
reader being refused. It now names the hook where git will actually look for it, resolved by git at
the moment the line is run, so it is right under `core.hooksPath` and in a linked worktree too. The
hook body's version marker moves to v7 with it: re-running the installer replaces an installed v6
rather than reporting it current.

Four non-test Go files move for the binary's three and the bundles' two —
`cmd/dossierx/retired.go`, `cmd/dossierx/output.go`, `cmd/dossierx/claim.go` and
`cmd/dossierx/skills_embed.go` — and none of them changes anything but the text a reader is handed.
The hook's two are shell, in `scripts/install-git-hook.sh`.

What this release is otherwise is the machinery that publishes the next one. Everything that has ever
gone wrong with a DossierX release has been in the half a maintainer performs by hand — a version
string copied into prose and left behind, a verification step that read the source instead of the
artifact, a `commit` field that named the wrong sha for two releases running — and every one of
those was a promise to look rather than something that fails when it is not true. Each item in
`docs/RELEASING.md` now has a check behind it, the release itself is performed by a program rather
than by a person following a list, and the parts that genuinely cannot be checked from inside this
repository are named as such instead of being quietly assumed.

### Added — `surfaces.yaml`, and one reading agent per surface

`surfaces.yaml` declares every client-facing surface this project has — thirteen of them, from
`README.md` to the compiled binary — and, beside them, seven declarations of what is deliberately
out of scope and why. `tests/surfaces_manifest_test.go` requires every tracked file to be claimed by
**exactly one** entry: a file matching nothing fails the build, and an out-of-scope entry cannot
quietly swallow a path a surface also claims, because the test names both and fails. Before this,
the list of things to review lived in a scope document, so a new client-facing file could appear
with nothing to notice it and the only way to find the gap was an audit.

At release time each surface is read by its own agent against a bundle assembled for it — the
prompt (`gate/prompts/<surface>.md`), the surface's own files, and the extracted evidence its
questions need. The bundle is fingerprinted, and the cache key is the digest of what the agent was
**actually handed**, not the surface's name: change a byte in the evidence and that surface is
re-read rather than carried forward. `gate/method.yaml` grants the agent exactly two tools, both
report-only (`SurfaceFinding`, `SurfaceVerdict`), as an exclusive allow-list rather than a deny
list — there is no file, shell, search, network or subagent tool, because "the bundle is the whole
evidence set" is the property every key in the system rests on. What that file cannot promise, and
says so, is that the harness outside this repository honoured the request.

Findings are never filtered, deduplicated or ranked away on their route to the report, and a receipt
carrying any finding at all evaluates to FAILED. A finding's `severity` is free text the reporting
agent wrote about its own work: one sort comparator reads it so that a re-run over an unchanged tree
produces an identical document, and no verdict, filter or threshold consults it anywhere.

### Added — `make release-publish`, the only thing in this repository that tags

The irreversible half of a release is now a nine-step driver rather than a sequence of commands a
person types. It is authorized by the version typed twice —
`make release-publish DOSSIERX_RELEASE_VERSION=vX.Y.Z DOSSIERX_RELEASE_AUTHORIZE=vX.Y.Z` — and
deliberately not by a boolean, because a `=1` left in a shell profile or a CI secret authorizes
every release forever, including the next one somebody triggers by accident.

Before it touches git it establishes, in this order: that no part of this release is already
published; that **the tree declares the release being tagged** — `CHANGELOG.md`'s newest heading and
`site/src/content.ts`'s last `releases[]` entry must agree with each other and with what the human
typed, which is the one question content-matching cannot answer, since a self-consistent tree tagged
as some other release passes every other check; that the gate is green, **recomputed in this
process** rather than read out of a record, because "no findings" cannot stand in for six separate
refusals about coverage; and that the CI-run evidence for this exact commit exists. Then it merges
`--no-ff`, reads the merge commit once and uses that value everywhere after, tags the named object,
reads the tag back through its ref and re-checks the tree it points at, pushes the tag **by value**,
verifies the published archives, and only then pushes `main`.

A run that stops leaves a state a human can read: the step it stopped at, what had already been
published, and what had not. It never proposes a retry it cannot perform.

### Added — the published archives are verified before `main` moves

Between the two irreversible acts, the driver reads the artifacts the way somebody downloading them
does. It polls the forge until the Release workflow's assets exist — the ordinary state one second
after a tag push is "the tag is there and the assets are not", so waiting happens inside the step
rather than as advice to run the command again, which there is no way to do once the tag is public.
Then: a missing `checksums.txt` is UNCHECKABLE and never "no mismatches found"; the expected archive
names are derived from `.goreleaser.yaml` at the released commit — the build matrix, the
`name_template` and the format overrides — so the day a seventh target is added this check grows
with it instead of counting six and passing; every archive's sha256 is compared against its line
**and** every line against an archive that was actually read; and the host platform's archive is
extracted and its binary **run**, because an archive can carry a correct name, a correct checksum
and a stale binary while every metadata check passes over it.

### Added — the forge refuses an ungated tag

`.github/workflows/release.yml` used to be `on: push: tags: ['v*']` and, one job later, six archives
and a GitHub release, with no condition of any kind between the two. A new `gate` job now runs first
and the publishing job `needs:` it, so a tag that does not get past it produces no archives at all.
Two facts have to hold, both about the tagged tree rather than about whoever pushed: the tagged
commit is reachable from `origin/main`, and the tree at that commit carries the release stamp for
exactly this version. Every exit path that is not a pass is a refusal — there is deliberately no
branch that reports "could not check" and exits 0.

> **SUPERSEDED BY 0.5.2.** The first of those two facts no longer holds. Reachability from
> `origin/main` was removed in 0.5.2 because it deadlocked the release driver, and the gate now asks
> instead that the tagged commit be a **merge**. The stamp check is unchanged. This paragraph is left
> as written because it is the record of what 0.5.1 shipped; read the 0.5.2 entry for what the gate
> does today.

One residual is recorded in that file rather than described as fixed: the workflow GitHub runs for a
tag is the one in the tagged tree, so anyone with push rights can weaken this job and tag that
commit. Nothing inside this repository closes that — a check cannot be its own enforcement — and
only a forge-side tag protection rule can.

### Added — `make ci-evidence`: the run's own account, not the run's conclusion

A green badge is not the check, and neither is a green check run: a conclusion is `success` over
zero tests, so a suite emptied by a `-run` selector prints `ok [no tests to run]` for every package
and the step, the job and the check run all conclude success over it. `make ci-evidence` fetches the
CI run for a named merge commit and adjudicates the `go test -json` account the test binary itself
emitted — per package, per test, per matrix cell — against the job set derived from `ci.yml`. No
conclusion is read as evidence anywhere; conclusions are recorded in the verdict record for a human
to look at and are adjudicated by nothing. The record is required to exist and to name the commit
being released: a release nobody ran this for is refused, not assumed.

### Added — `surface.json`, and v0.5.0's inventory frozen as the baseline

`surface.json` is the machine-readable inventory of what this tree exposes — 19 commands under 7
nouns, 3 root flags, 12 retired spellings, 28 lint rules, 44 error codes, 5 skills, 14 HTTP routes,
129 markdown constructs, a render fingerprint, a per-package behaviour fingerprint, the JSON
envelope's keys and exit codes, and the version pins — extracted from the tree by a test and
regenerated, never written by hand. It is what prose gets judged against, which is what turns "the
README says twenty commands" from a reviewer's memory into a comparison.

`surface.baseline.json` freezes v0.5.0's inventory, because v0.5.0 shipped before the emitter existed
and carries no `surface.json` of its own. It is the only record of what that release's surface was,
so the first gated release has a real predecessor to be diffed against.
`testdata/render-across-releases.golden.txt` does the same job for rendered output: the class of
change a consumer's own gate cannot detect for them is exactly the class this project has shipped
three times, and it is now compared release to release rather than noticed.

### Changed — the published release notes no longer carry the merge commit's subject

`.goreleaser.yaml`'s `changelog.filters.exclude` gains `^Merge `. The GitHub release body is
generated from Conventional Commit subjects at tag time, and a `--no-ff` merge's own subject matches
neither `^chore:` nor `^docs:`, so the catch-all "Other changes" group swallowed it — v0.5.0's range
carries exactly one such subject, `eab3a63`, "Merge pull request #32 — v0.5.0, a claims graph in the
viewer". It also made the notes unpredictable by construction: the pre-merge prediction runs before
the merge commit exists, so it could never have seen the line the published page would carry. Both
are closed by the one exclude, proved against a real `git merge --no-ff` in a from-scratch
repository rather than against this project's own history, with the pre-fix config kept as the
negative control so the scenario is shown to be real and not hypothetical.

The release notes are a declared surface in their own right, and `.goreleaser.yaml` is its only
path: the notes themselves do not exist until the tag, so the rules that decide what they say are
what gets reviewed. The reason that surface exists is a shape nothing audited before — a
user-visible change landing under a `docs:` or `chore:` subject is dropped by the filters and is
invisible on the release page while being fully described in this file.

### Changed — one release procedure, and the encoded second one is retired

`.claude/workflows/release-checklist.js` — 447 lines that offered themselves to every agent in this
repository as a runnable release procedure under their own name — is deleted, and the deletion is
pinned. That distinction is the whole of it: restoring the file verbatim used to leave every test in
this repository green, because nothing had ever read that directory, so the deletion was a fact
about one commit rather than an invariant about the tree. `tests/ci_run_evidence_test.go` now parses
every workflow declaration under `.claude/workflows/` and refuses any that declares itself a release
procedure; a file it cannot parse is a failure and not a pass, because an unexamined corner of the
directory the harness loads is exactly where a second procedure would sit unnoticed.

`docs/RELEASING.md` is the single description of how this project releases, and it is now read by
tests as well as by people — its pin sweep, the ordering of its tagging steps, and the three checks
it keeps a person's are all held against the driver that performs them.

### Changed — CI reports what it ran

The test job checks out at `fetch-depth: 0`, because a checkout with no tags cannot resolve a
release baseline and would otherwise take the "no tag yet" branch of every date and cross-release
comparison and pass. `go test -race ./...` becomes `go test -race -json ./...`, which is what makes
the run's per-test account exist at all — and it stays ONE command, spelled entirely out of the
closed vocabulary that keeps `|| true`, `set +e` and `| tee` out of a suite step. The viewer job
installs a pinned GoReleaser (v2.17.1) and fails rather than skips when the binary is not there, so
the release build is watched doing its job instead of being read for what it was told to do; the
site is built with a real toolchain so the browser suite reads rendered DOM rather than source.

### Fixed — the site advertised a twenty-command CLI

`site/index.html`'s `<meta name="description">` had claimed "a 20-command JSON CLI" since v0.3.0.
Twenty was the v0.3.0 surface; v0.4.0 cut it to seven nouns and nineteen leaves, and the tag stayed
wrong through two minor releases. It survived because it is the one count on the site that nothing
derives — every other version and count comes from `latestRelease` / `latestVersion` /
`commandCount`, deliberately, after three of them went stale once before — and `index.html` is
static HTML that cannot interpolate. It is also the string search engines and link previews quote,
which is the least likely place for anyone editing the site to look. The guard walks the real
command tree rather than pinning a second literal, so changing the surface fails the build until the
site follows; a phrasing that carries no count at all is not held to one, since a sentence without a
number cannot go stale.

### Fixed — the `dossierx version` transcript, and the hand-stamped release sha

The site depicted `dossierx v0.5.0` where the published binary prints `dossierx version 0.5.0` —
two errors in one short line, and nothing compared it to real output. GoReleaser's `{{.Version}}` is
the tag with its leading `v` stripped, so the release spelling and the transcript spelling are
different strings for good reasons; the transcript now derives from the release entry with the `v`
removed, and it is checked against a binary linked the way a release links one, read out of the
**rendered page** rather than out of the source.

> **SUPERSEDED BY 0.5.2.** Both halves of that first sentence stopped being true. The stamp is now
> `{{.Tag}}`, so the release spelling and the transcript spelling are the SAME string, and the
> transcript no longer strips anything — it renders the tag verbatim. The two-spellings-for-good-
> reasons framing is what 0.5.2 removed; see its entry. Left as written because it is the record of
> what 0.5.1 shipped.

The `commit` field is deleted from every release entry, along with the step that wrote it, the
fallback that rendered it and its type declaration. It could not converge — writing the sha is
itself a commit, so the value was stale the moment it landed — and it named the wrong sha twice
running: v0.4.1 shipped naming `5327923` while `refs/tags/v0.4.1` points at `206b4a4`. It also
disagreed with the binary by construction, seven characters against the forty GoReleaser stamps into
`main.commit`. The optional type declaration outlived the data, the reader and the release step,
which is what would have let the field come back silently: `commit: "abc1234"` on a new entry would
have type-checked, and the compiler was the only thing that would have objected.

**Not in this release, and stated so deliberately.** Nothing derives a finding's classification from
the evidence behind it, and there is no override field on a receipt — a finding a human has judged
non-blocking can be cleared only by fixing the tree or by deleting the finding by hand, and deleting
it leaves an adjudicated finding indistinguishable from one nobody raised. Why neither was built, and
what each would need first, is recorded in the tests rather than left to be rediscovered. Nor does
anything here verify the deployed site, the workflow run or the CDN: those are the three checks the
driver hands to a person at the end, and it says in those words that it examined none of them.

## [0.5.0] - 2026-08-07

**BREAKING: `dossierx check` now fails on a dependency loop that alternates `rests_on` and
`governed_by`, and no migration path accompanies it.** The new `mixed-cycle` lint runs at ERROR
severity, taking the registered rule count from 27 to 28. It walks the union of the `rests_on`
and `governed_by` graphs with the edge kind carried on every hop, and reports a cycle whose hops
include at least one of each — "A rests_on B, B governed_by A". Neither existing cycle rule can
see that shape: `cycle` walks `rests_on` alone and `governed-cycle` walks `governed_by` alone, so
a mixed loop presents no back edge to either walk and passed the entire registry. A project
carrying one passed `dossierx check` before this release and exits 1 after it, with no edit on
its side, no content-hash move and nothing in `.dossierx-lock-store.json` to explain the change.

**Re-run `dossierx skills export` after upgrading.** Upgrading the binary does not touch skills
already exported into a project — they are plain files in your repo, and nothing in `dossierx check`
reports that they are a version behind. A project that upgrades without re-exporting keeps the
v0.4.1 router, which has no `mixed-cycle` section, so its agent meets this refusal on a corpus it did
not touch, hunts for what it broke, finds nothing, and loops. That is the exact failure the section
below exists to prevent.

There is deliberately no migration command and no migration document: a corpus containing this
shape was always malformed, the engine simply could not see it. The recovery is to break the loop
— the finding names every claim on it — and re-run `dossierx check`. Where those claims are
locked, that is `dossierx claim unlock`, the edit, then `dossierx claim lock`, the same as any
other correction to a locked claim. `mirrors` is not part of the union graph and never trips this
rule; a pure `rests_on` or pure `governed_by` loop still reports as `cycle` or `governed-cycle`
and this rule stays silent on it.

**Every project's rendered viewer changes on its next render, and roughly triples in size.** The
basic fixture goes from 108,577 to 348,496 bytes for the same `dossierx check`. The pane is rendered
unconditionally and there is no config opt-out. Your own gate cannot tell you this: no claim's
content hash moves, no `review_pending` flips, and `dossierx check` reports exactly what it reported
before — the artifact you ship is simply a different, larger file. No claim's own markup changed, so
this is additive chrome rather than v0.4.1's shape where locked bodies re-rendered. Re-run the render
to pick it up, and see the graph-pane section below.

### Added — a claims graph pane in the viewer

The rendered viewer now carries a "Claims graph" pane: a canvas view of the corpus's `rests_on`,
`governed_by` and `mirrors` edges, with selectable overlays (isolated & weakly linked, dependency
cycles, governance, review pending, open comment threads, draft vs locked), a per-claim detail
panel that includes an `in a cycle` row, granularity collapse to modules or facets, zoom, pan and
drag. Above 300 claims the pane opens at module granularity rather than drawing every claim.

It is built by a new `internal/graph` package as a JSON payload inlined into the single
self-contained `index.html`, alongside three new embedded client files (`graph-core.js`,
`graph-ui.js`, `graph.css`). No external assets, so it works over `file://`. There is no new CLI
noun and no new schema field.

The viewer's size and re-render consequence are stated in the preamble above, since a consumer's own
gate cannot report either. The pane is a third full-viewport overlay with its own body scroll
lock (`body.dxg-open`) that is additive with the sidebar drawer's and the comment panel's rather
than mutually exclusive with them.

### Added — `GET /api/graph` under `dossierx serve`

Backs a refresh button in the pane, rebuilding and re-stamping the payload from the current
catalog at request time rather than reusing the render's. The button is absent over `file://`,
where there is nothing to refresh from.

### Added — `testdata/fixture-graph-demo`, a third committed sample viewer

A 58-claim fixture project with a tracked `viewer/index.html`. It is the first fixture in this
repo that is itself a ledger-covered dossierx project, so its `.dossierx-comment-digest.json` is
a tracked input (`.gitignore` gained one negation line for exactly that path) alongside its
`.dossierx-lock-store.json`. Without the digest, a fresh clone or CI checkout would fail the
fixture on `comment-digest-absent`. `docs/RELEASING.md` now names three sample viewers to
regenerate, not two.

### Added — `tests/fixture_staleness_test.go`

Fails the build when a committed sample viewer no longer matches what the current renderer
produces, instead of leaving a stale generated artifact to be noticed at release time.

### Changed — the offline scan strips comments before matching

`tests/portability_test.go`'s check that no shipped `.js` reaches the network now strips `//` and
`/* */` comments from the file before matching, so a comment that names an endpoint — `graph-ui.js`
documents its single `/api/graph` call — no longer reads as a network call.

### Changed — the embedded skills teach `mixed-cycle`, and the release gate checks them

The router skill (`skills/dossierx/SKILL.md`) gains a `mixed-cycle` section, because the router is
the one file an agent is guaranteed to have read before it meets the finding, and this is a refusal
that fires on a corpus the agent did not touch. An agent meeting one otherwise hunts for what it
broke, finds nothing, and loops. The section says three things: you did not cause it, there is no
migration, and the claims on the loop are usually locked so the recovery is `unlock → fix → lock`
with the human's approval. `dossierx-claims` gains one bullet listing all three loop shapes beside
the edge schema, and the router's surface table now names the claims graph as part of what the human
reads.

The skill line budget moves 230 → 255 to fit it. That is a deliberate resize on the same reasoning
that moved it 200 → 230 for v0.3.0's adoption section, not a per-release ratchet: the router was
already at 229 of 230, so the choice was to cover the breaking change or to cut something else
load-bearing. The four companion skills are unaffected and still sit under 235.

`docs/RELEASING.md` gains a matching pre-merge item. These skills are `go:embed`-ed into the binary
and installed into *other people's* repositories by `dossierx skills export`, where a stale rule does
not render a wrong page — it teaches an agent the wrong recovery on somebody else's locked claims,
and it ships inside the binary, so a fix after the tag never reaches anyone who already installed.
The gate asks the falsification question ("did this release make that assertion FALSE?") rather than
the mention question, and singles out new refusals that can fire on an unchanged corpus.

### Fixed — the browser suite is linted, and says which browser it drove

`viewer-tests/` is a separate module, so `golangci-lint run ./...` at the repository root never read
a line of it — the same blind spot `go test ./...` has, and the reason a `viewer-test` target already
existed. CI's lint job gains a second step with `working-directory: viewer-tests`, and the Makefile
gains `viewer-lint`. The first run found ten findings in code no linter had ever read: five
unchecked errors around the `serve` subprocess teardown, a non-wrapping `%v` that should be `%w`,
and four gocritic style findings. All ten are fixed rather than silenced.

`tests/nested_module_coverage_test.go` — which already refused to let a nested module exist without
a CI test job and a Makefile target — now refuses to let one exist without a lint job either, so the
next nested module cannot repeat this. It checks per STEP rather than per file: the first version
asked whether `ci.yml` contained `working-directory: viewer-tests` and `golangci-lint` anywhere,
and passed immediately against a workflow that linted nothing but the root, because those two true
facts belonged to different jobs.

The suite also now logs which browser it resolved. A green run against the Comet fallback is not the
same evidence as a green run against Chrome — Comet is a Chromium fork that serves its own
`chrome://` UI, and its traffic is what the offline test was misreading as the page's own.

### Fixed — the offline viewer stops probing for a comment backend

Opened over `file://`, the viewer no longer issues the relative fetch that backs its comment
probe, so it no longer logs a `net::ERR_FILE_NOT_FOUND` to the console on load. What the reader
sees is unchanged — the panel was, and remains, read-only offline — and the probe still runs
normally over `http://` and `https://`.

**Not in this release, and stated so deliberately:** any code-grounding signal. The graph audits
claims, not code. There is no `has_code_link` field, no "locked, ungrounded" rule and no
`implink` argument.

## [0.4.1] - 2026-08-04

**Two things change for already-locked claims that `dossierx check` cannot tell you about.**
Neither is an edit a user made, and neither produces a ledger event, so the gate reports exactly
what it reported before the upgrade. They are listed here together because that is the only place
a consumer can see both; each has its own section below.

**1. An already-locked, byte-identical claim renders differently in the viewer.** The shared edges
footer collapses into a `<details>` disclosure and the comment chip moves out of that footer into
the claim's head, so every layout that carries a chip or a footer emits different HTML from the
same locked bytes — no edit, no content-hash move, no `review_pending` flip, nothing in
`.dossierx-lock-store.json`, and nothing for `dossierx check` to report. This is the same shape as
v0.4.0's table fix and v0.3.1's renderer expansion, and the same tool applies: re-run the render
to pick it up, and use `dossierx claim unlock` when you want to revisit the claim's rendered
output on a human's own review. See the edges-footer section below.

**2. A locked claim that already carries `raw_html` re-hashes once against its dependents on the
first check after this upgrade, with no edit and no ledger event.** `ContentHash` — the
`rests_on`/`mirrors`/`governed_by` drift baseline — now folds in `raw_html` when it is non-empty,
so that editing the attachment this release newly allows on a rule-bearing claim is no longer
invisible to that claim's dependents. The change is gated, not additive: a claim with no
`raw_html` — every claim before this release, and most after it — feeds `ContentHash`
byte-identical input to before and re-hashes nothing. The one case that does move is a claim that
already had `raw_html` set (only possible pre-upgrade on an allowlisted, reviewed `layout: mockup`
claim) and is named by another locked claim's `rests_on`, `mirrors`, or `governed_by`: that
dependent's recorded baseline no longer matches the recomputed hash, and it flips to
`review_pending` once, the same no-edit, no-ledger-event shape as this file's last two releases.
See the `raw_html` section below.

**No migration is required** for either — there is no schema change, no new store field, and no
command to run; a flip caused only by the `ContentHash` widening clears the ordinary way,
`dossierx claim reaudit --confirm` or `unlock` then `lock`.

### Changed — the edges footer collapses into a native `<details>`, and the comment chip moves into the claim head

The shared edges footer (`rests_on`, `mirrors`, `governed_by`, `depended on by`, `migrated_from`,
`implemented in`, `review_pending`) now sits inside `<details class="claim-links">` with a
`<summary>` giving a pluralized digest — `"1 link - 2 files"`, `"4 links - 2 files - 1 drifted"` —
where the drifted segment appears only when it is non-zero. `governed_by: none` no longer counts
toward the link total, since it states an absence rather than a followable edge; a claim with no
links and no linked files now emits no `<details>` at all rather than an empty disclosure. The
footer opens automatically, server-side, when a linked file has drifted or the claim is locked and
`review_pending`; a deep link to the claim (`:target`) and print output also force it open, both
via CSS only, since neither signal is knowable at render time.

The comment chip is no longer an `<li>` inside that footer — it moved into each claim's head, as
the head's trailing child, beside the new `<span class="label">` that holds the id/title and
status pill together. A claim with no comment threads reveals its chip on card hover or keyboard
focus (never `display:none`, so it stays in the tab order), rather than being hidden until the
footer was opened. A toolbar toggle expands or collapses every footer on the page at once, as a
client-side DOM change only, with no persistence across reloads. `layout: banner` is the one
partial that renders neither a chip nor an edges footer, and neither half of this touches it.

This changes rendered viewer output on every layout that carries a chip or a footer, so re-run the
render to pick it up.

### Fixed — `raw_html` is an attachment legal on any layout, not a layout a claim must adopt (closes issue #25)

`raw_html` used to be legal only on `layout: mockup`; a claim that was genuinely a table or a list
of steps could not also carry a diagram or a small rendered mockup alongside its own content.
`checkMockupGate`'s layout leg (`internal/lint/raw_html_scope.go`) is removed: `raw_html` may now
sit alongside `body`/`rows`/`steps` on any of the seven layouts. `layout: mockup` stays a valid
layout in its own right — it is the one that also swaps in a "No mockup content." empty state.

This changes **where** `raw_html` may sit, never **who** may author it or **what** reaches the
viewer unescaped — every other leg of the gate is untouched and still fires on every
`raw_html`-bearing claim regardless of layout: the `mockup_modules` allowlist, the
tag/attribute/class markup allowlist, the `raw_html_reviewed` human review flag, and the
lock-lifecycle check. `components.MockupHTML`'s render-time escaping gate is byte-identical. All
seven layout partials (`card`, `table`, `list`, `steps`, `tree`, `banner`, `mockup`) now render
the attachment — reusing the existing `claim-mockup-body` class — after the claim's own
body/rows/steps content and before the edges footer.

A related gap this same widening opened is closed alongside it: `dossierx claim flag`'s body-only
classifier used to key "safe to flag" purely on layout (card/banner/list/tree), which was only
sound while those layouts could not carry `raw_html`. It now keys on whether the claim actually
carries `raw_html`, regardless of layout.

## [0.4.0] - 2026-08-03

**Three things change for already-locked claims that `dossierx check` cannot tell you about.**
None involves an edit, a content-hash change or a ledger event, so the gate reports exactly what
it reported before the upgrade. They are listed here together because that is the only place a
consumer can see the whole set; each has its own section below.

**1. A locked `layout: table` claim renders differently, with no edit and no ledger event.** The
lock ledger signs a claim's `rows` bytes, not the HTML those bytes produce — so the table fix in
this release changes what an already-locked, byte-identical claim looks like in the viewer.
Ordinary table content redistributes by roughly 12% on a two-column 520px table, and a long
identifier that used to force its column wide now wraps. Nothing flips `review_pending`, and
nothing appears in `.dossierx-lock-store.json`. This is the same shape as v0.3.1's renderer
expansion, and the same tool applies: re-run the render to pick it up, and use `dossierx claim
unlock` when you want to revisit the claim's rendered output on a human's own review.

**2. The first write to a locked claim after upgrading may reindent its block scalars.** Claim
writes now merge only the changed top-level keys onto the existing document, but the merged tree
is re-emitted at two-space indent — which is what makes the bytes safe to hand to the round-trip
guard. A `body: |` authored at four spaces comes back at two. This changes bytes, not values: no
content hash moves, no ledger record is affected, and no locked claim is flagged by it. The
place a reviewer will see it is the `git diff`, once.

**3. A claim locked before this upgrade has no governance baseline, and the first edit to its
governor after upgrading does not flag it.** `governed_by` becomes a drift edge in this release,
and a baseline is the governor's content hash recorded at lock time — so a claim locked by v0.3.x
simply has no such entry in `.dossierx-lock-store.json`. There is deliberately **no backfill**, no
adoption event and no announcement: with the migration path removed in this same release there is
no adoption vocabulary left to reuse, and manufacturing a baseline out of content nobody approved
is precisely what v0.4.0 removes. A locked claim gains its baseline the next time it is locked or
re-audited, and only from then on does a governor edit flag it `review_pending`. The deliberate
tools are the ordinary ones — `dossierx claim reaudit --confirm` refreshes the baseline, and
`dossierx claim unlock` followed by `dossierx claim lock` mints one.

### BREAKING — migration is removed; a pre-ledger project crosses by holding nothing locked (closes issue #18)

**`dossierx migrate` — and `dossierx migrate --adopt` — is gone.** Adoption is removed rather
than carried forward: nothing can attest to content no ledger ever recorded, and a command that
manufactures approval out of observed bytes is the one operation v0.3.0 already called unsound.

The replacement is not a command but an **ordered sequence**, quoted here in the exact words the
binary emits so the CHANGELOG, the skills' retired-command row and the retired stub's own hint
cannot drift apart:

> re-propose any locked build order (`dossierx build-order propose --module <m>`), unlock every
> locked claim (`dossierx claim unlock <id> --reason "..."`), then lock only what you still stand
> behind — the first lock in a project with nothing locked crosses the store onto the ledger

The order matters. `build-order propose` requires the module still fully locked, so re-proposing
has to happen *before* any claim is unlocked; the other order strands the locked order with no
way to release it. And what the crossing records matters just as much: the first lock in a
project holding nothing locked stamps the store onto the ledger schema and records a **real
approval**, with nothing grandfathered. That is the whole difference from the removed adoption
path. Commit the updated `.dossierx-lock-store.json` and `.dossierx-comment-digest.json`
alongside the re-locks.

`dossierx migrate` survives as a **hidden retired stub** that fails naming the new path. It has
to exist as a command at all because flag parsing runs before any unknown-command handler — so
without it, the invocation a whole release told agents to type would fail as `unknown flag:
--adopt` rather than as an explanation.

The wire changes, each stated as the rename or removal a consumer's parser will see:

- `error.code` **`adoption_required` → `pre_ledger_unadopted`**. Same three emitters — `claim
  lock`, `claim reaudit --confirm`, `build-order lock` — but the condition is narrower and now
  literal: the project's lock store predates the ledger **and** it still holds locked artifacts.
  On a project holding nothing locked those commands no longer refuse at all; they perform the
  crossing. The recovery is no longer one command but the ordered sequence above, which is why
  the code was renamed rather than left in place.
- `error.code` **`already_migrated` is REMOVED**, together with its `data.mode` discriminator
  (`already_covered` / `nothing_to_adopt`). There is no command left that can be run twice.
- Ledger finding rule **`lock-ledger-adoption-required` → `lock-ledger-pre-ledger`**, and it is
  now **conditional**: silent unless the project holds at least one locked claim or at least one
  locked build order. A pre-ledger project with nothing locked is not in a bad state — it is one
  lock away from being on the ledger — and reporting a finding there told a clean project to run
  a recovery it did not need. The message now names the on-disk schema version, the count of
  locked claims and the count of locked build orders, and carries the ordered crossing steps.
- **`dossierx check`'s `data.ledger_adopted` field is REMOVED.** With no adoption path it could
  only ever be empty, and it was dropped rather than left as a permanently-absent key a consumer
  might wait for. `data.comment_digests_adopted` is unaffected and still reported.
- `dossierx claim show`'s `data.ledger.grandfathered` key **stays**, without `omitempty`, and is
  **always `false`** for any record this build mints. It is true only for records surviving from
  a project that ran the removed adoption path, and the key is kept precisely so a consumer can
  still tell the two apart.

The surface shrinks with it: **eight nouns and twenty leaves become seven nouns and nineteen
leaves** — `check`, `claim`, `comment`, `build-order`, `serve`, `skills`, `version`. The removed
noun is `migrate`, the same one that took the surface from nineteen-under-six to twenty-under-
seven when v0.3.0 added it, so this returns the leaf count to nineteen. The hidden retired stub
is deliberately not counted as surface: it is excluded by mark rather than by hidden-ness, so a
real leaf can never be smuggled past the count by hiding it.

**Two further machine-contract changes fall out of the same removal.** Neither is in the wire
list above, and both would be found the hard way by an agent that branches on them:

- **The dry-run precondition `project_migrated` is renamed `pre_ledger`.** It surfaces in every
  `--dry-run` envelope as `data.preconditions[].name` and, when it blocks, in `data.blocked[]`.
  The eight migrate-only preconditions (`adopt_flag_given`, `history_confirms_pre_ledger`,
  `lock_store_exists`, `locked_claims_match_version_control`, `not_already_migrated`,
  `pre_ledger_claim_not_contradicted`, `something_to_adopt`) go with the command; `pre_ledger` is
  the one replacement, and it is the only one of the set that ever applied to a surviving command.
- **An approval-recording command that REFUSES can still cross the store.** The crossing runs
  before the operation's own preconditions, so a `build-order lock` that then fails — on a
  hand-edited order, say — leaves the store stamped onto the ledger schema even though it
  reported `ok: false`. This is safe rather than merely tolerated: the crossing only runs at all
  when the project holds nothing locked, so it discards no approval, and the post-state passes
  `dossierx check` cleanly. It is recorded here because a durable write by a command that
  reported failure is genuinely surprising, and no other bullet would tell you. `--dry-run`
  writes nothing, and no read-only command crosses.

### Changed — `governed_by` is a drift dependency (closes issue #21)

A claim-valued `governed_by.type` now joins `mirrors` and `rests_on` as a **drift edge**. When
the governing claim's comparable content changes underneath a locked claim, that locked claim is
flagged `review_pending`, `claim show` reports `review_trigger: drift`, and `check`'s
`next_steps` lists it under the drift/reaudit step.

Four boundaries, each of them a decision rather than an accident:

- It is **not a gating edge.** Hub gating is byte-for-byte unchanged — a claim naming an
  unlocked doctrine-facet claim only through `governed_by.type` still locks.
- `governed_by.type: none` creates no baseline and never triggers drift.
- Propagation is **staged**, matching the existing drift edges: flagging a direct dependent does
  not itself flag claims downstream of it.
- A claim reaching the same target through two edge types (`rests_on: X` and
  `governed_by.type: X`) produces exactly **one** baseline entry, deterministically.

For what this means on claims locked before the upgrade, see the note at the top of this entry.
The internal correction that made it possible is worth naming: the drift set had three
independent implementations — `internal/lock`, the pending walk in `internal/comments`, and an
inline copy in `main.go` — which is why a reader who tested drift on v0.3.x may have seen
different answers depending on which command they asked. All three now call one function.

### Changed — comment bodies with a space-indented first content line are now storable (closes issue #24)

A comment or reply body whose first content line begins with **space** indentation used to be
refused as unstorable and now stores fine. The two shapes, as the tests spell them:
`"    func main(){}\n    return"` (space-indented first content line) and `"  a\n  b"` (a
two-space-indented multiline body).

The cause of the loosening is mechanical rather than a relaxed rule: the loader now emits claim
YAML at a two-space indent through one shared encoder, and at that indent a block scalar carries
those bodies back byte-exact — so the round-trip guard, which is the actual gate, stops refusing
them. This is a **widening**, and it lands at every layer that pinned the old behaviour
together, because they are matched by construction: `comments.validateBody`, the `comment add
--dry-run` `body_is_storable` precondition, the CLI's `unsafe_body` refusal, and `dossierx
serve`'s 400 on `POST /api/comments`.

The limit, stated as sharply as the widening: a **tab-led** first content line is still refused,
at any indent width. The store-bricking class survives; it is now tab-led only. (`ErrUnsafeBody`'s
own message still advises "de-indent the first line", which is now broader than the surviving
rule.)

### Fixed — a claim write touches only the keys that changed (closes issue #24)

Adding one comment used to land as a whole-file rewrite — a 117-line diff in which exactly one
key was new — because `SaveClaim` re-serialised the claim from the struct. It now merges the
changed top-level keys onto the existing document's node tree, so an unchanged key keeps its
authored quoting, block-scalar form, key order and YAML comments.

Two modes, because `SaveClaim` is also the file-**create** path: create emits the fresh document
wholesale, mutate merges. Both still end at `verifyRoundTrip` and an atomic write — the
round-trip guard is a gate, not a nicety — and the merge falls back to the fresh whole-document
bytes whenever it cannot be done faithfully, so no write is ever less safe than before.

One deliberate exception, stated as such: **block-scalar indent width is not preserved.** The
merged tree is re-emitted at the loader's two-space indent, which is exactly what makes the
merged bytes safe to hand to the round-trip guard. This changes bytes, not values — a claim's
content hash is computed over decoded field values, which a block scalar's indent width does not
change — so no ledger record is affected and no locked claim is flagged by it.

### Fixed — wide `layout: table` claims scroll instead of running under the viewer chrome (closes issues #22, #23)

A `layout: table` claim wider than its column used to overflow with no way to reach the content.
The table now sits in a `.claim-table-scroll` container that scrolls on its own axis, so the page
body never scrolls sideways, and cells wrap rather than forcing the track wider, with a 5rem
per-cell floor so a column of short values does not collapse to unreadable slivers.

Two consequences a reader will notice:

- **Ordinary table content redistributes slightly — about 12% on a two-column 520px table.**
  This is the knowing price of `overflow-wrap: anywhere` rather than `break-word`: only
  `anywhere` contributes its soft-wrap opportunities to min-content, which is what stops a single
  65-character identifier taking ~80% of the table under auto table layout. `break-word` leaves
  proportions byte-identical and does not fix the bug at all.
- **Markdown tables inside a claim `body` are deliberately not changed.** Both new rules are
  scoped to `.claim-table-scroll`, which a `.md-table` is never inside; a body pipe table is
  already contained by `width: max-content` plus `overflow-x: auto`, so a long identifier makes a
  body table *scroll* rather than wrap. Body tables therefore keep scrolling rather than wrapping
  — a deliberate deferral, not an oversight.

This changes rendered viewer output, so re-run the render to pick it up.

### Fixed — `markdown-sanity` no longer fires on punctuation after a closing delimiter run (closes issue #20)

The `markdown-sanity` lint rule reported an unmatched delimiter run whenever a correctly-closed
emphasis or strikethrough run was followed by punctuation. The measured cases are the clearest
statement of the bug: `"Only ~~strike~~, comma after."` produced one finding while `"Only
~~strike~~ no comma."` produced none, and `"Has **bold**, and more."` produced one with no `~`
anywhere in the string.

The issue report named the wrong cause, and the record is worth correcting: it was not a shared
cursor requiring `**`, `*` and `~~` together — it was flanking being decided on whitespace
alone, so any closing run followed by punctuation looked like an unclosed opener. The scanner now
applies CommonMark's punctuation clause **per rune** rather than per byte, which is what the spec
specifies and what multi-byte punctuation requires.

Widening right-flanking admits every run with a word character on one side and punctuation on the
other, so a carve-out holds the noise budget. It covers the bracket family — `(*Store)`, `a[*p]`,
`Topic_(disambiguation)` — and the **path separator**, without which every underscore-prefixed
path segment (`internal/_generated`, `docs/_partials`, `testdata/_fixtures`) becomes an unmatched
`_` warning. Those were silent before this release; admitting them would have traded the false
positive #20 removed for one this project's own claim bodies hit more often.

The finding's message was also wrong about the cause and no longer claims emphasis is outside the
renderer's subset — it has not been since v0.3.1 — and now names the delimiter it found and where
it opened. These are warning-severity craft findings, so the false positive was noise rather than
a blocked `claim lock`.

## [0.3.1] - 2026-07-30

**Locked claim bodies may render differently after this upgrade, with no edit and no ledger
event.** The lock ledger signs a claim's `body` bytes, not the HTML those bytes produce — so
widening the renderer changes what an already-locked, byte-identical body looks like in the
viewer without touching the field the ledger hashes, without flipping `review_pending`, and
without leaving any entry in `.dossierx-lock-store.json` naming the change. `dossierx check`
will report exactly what it reported before the upgrade: nothing. If a body relied on a
construct that used to fall through to literal text — a line that happened to start with a
dash, a stray backslash, an indented block that used to escape its enclosing list — it may now
render as a live construct instead. There is no automatic re-review step for this; the tool to
revisit a claim's rendered output deliberately, on the human's own review, is `dossierx claim
unlock` (or, for genuine drift, `dossierx claim reaudit`) — see FORMAT.md's markdown-ceiling
section for what changed.

### Changed — renderer expansion

The claim-body markdown renderer grew substantially beyond the previous release's ceiling —
this is the largest single change in v0.3.1, and it lands on every layout that renders `body`
or a `steps` entry (card, banner, list, steps, table, mockup), on comment bodies, and on `rows`
table cells, in each case exactly as far as that surface's ceiling reaches. See FORMAT.md's
markdown-ceiling section for the authoritative construct-by-surface table; summarized here:

- **Block structure.** Fenced code blocks are now recognized by the line scanner itself rather
  than a whole-body pre-pass, so a fence nested inside an open list item or an open blockquote
  no longer splits the container around it; a fence's info string now contributes
  `class="language-x"` to the rendered `<code>` element. One-level blockquotes recurse into the
  same block scanner, so paragraphs, lists, task items, headings, thematic breaks and fenced
  code all render inside a quote. ATX headings at levels 3–6 (`#`/`##` stay reserved for the
  viewer's own chrome and render as literal text). Thematic breaks. Unordered/ordered lists
  nest to unbounded depth via an indent-keyed stack, with GFM task-item checkboxes, CommonMark
  list looseness (a property of the list, not the item — a tight nested list stays tight inside
  a loose parent), and `<ol start="n">` for a list that does not begin at 1. Hard line breaks (a
  trailing backslash or two trailing spaces) become `<br>`.
- **Inline constructs.** Backslash escapes over a closed 15-character set (`<` and `&` are
  deliberately outside it, so `\<` still shows its backslash); double-backtick code spans;
  `**bold**`, `*italic*`/`_italic_` under strict CommonMark flanking (an intraword underscore
  can never open or close, so `governed_by`-shaped tokens never italicize), and `~~strike~~`;
  angle-bracket and bare-URL autolinks. The inline-only ceiling used by `rows` table cells and
  by a GFM pipe-table cell (`markdown.RenderInline`) gained the same emphasis, strikethrough and
  autolink constructs as the block ceiling, on top of the escapes/code-spans/links it already
  had — **every `rows` table cell renders differently after this upgrade** if its text happens
  to contain `*`, `_`, `~~`, or a bare URL. A third field joins that ceiling: a `governed_by:
  none` claim's `reason`, which the edges footer used to write through `html.EscapeString`, now
  renders through `markdown.RenderInline` — so a reason that names a claim id in backticks or
  carries a link, an asterisk or an underscore-flanked token renders differently after this
  upgrade too, on the same no-edit, no-ledger-event terms as a body.
- **GFM pipe tables**, new in this release: a header row, a required delimiter row (which must
  itself carry a pipe — a bare `---` stays a thematic break), and zero or more body rows. A
  well-formed table is always rendered as a table, at any size or shape: a body row with fewer
  cells than the header renders **short** rather than padded, and a longer row has its extra
  cells dropped. Row splitting happens before inline parsing, so a pipe inside a cell's code
  span still splits the cell unless escaped (`` `a\|b` `` is the working spelling for a literal
  pipe inside inline code).
- **Images**, new in this release, and the one construct that is not available everywhere the
  rest of this list is: `![alt](src)` renders as a real `<img>` in claim-authored `body` and
  `steps` text and in a table cell embedded in that text, and **never** in a comment body. `src`
  must resolve under the claim's own `assets/` directory (a fixed, non-configurable name) and
  end in one of six extensions (`.png .jpg .jpeg .gif .webp .svg`); anything else renders the
  whole `![alt](src)` as escaped literal text rather than as a broken image. Two new lint rules
  ship with it: `markdown-sanity` (mostly warning-severity craft findings — a malformed table, an
  unclosed fence — but error-severity on the security-relevant ones, such as an off-origin image
  or link `src`) and `asset-scope` (error-severity throughout — it can refuse `dossierx claim
  lock` on an image `src` that resolves outside `assets/` or carries an unlisted extension, on a
  corpus that had no such check before this upgrade).

### Changed — actionable status pills on claim-edge references (closes issue #11)

v0.3.0 shipped readable edge labels but left the last piece of issue #11 unbuilt: a claim-edge
reference said nothing about the state of the claim it pointed at. A `governed_by`, `mirrors`,
`rests_on` or "depended on by" target now carries a small status pill — `draft`, or
`review_pending` for a locked target flagged for re-review — reusing the same three-way
status-to-class mapping every claim-head pill already uses. The pill is **actionable-only**: a
healthy locked target gets nothing, so the footer stays quiet and lights up exactly on the case
it exists to explain, a claim gated on an edge that is not ready yet. It requires the whole
catalog to resolve a target's status, so it rides the catalog-aware edges binding
(`internal/render`'s `attachEdgesOverride`); the parse-time `edges`/`claimEdgeList` funcMap
bindings have no catalog and render every target with no pill, exactly as before — which is why
`build_order.html`'s `rests_on` list, which lists only locked claims anyway, is unchanged.

### Changed — new HTTP surface in `dossierx serve`

`dossierx serve` mounts a second, non-API route: `GET /claim-assets/<claim-id>/<path>`, which
serves exactly the images the loaded claims reference, answered from an allowlist computed from
those claims rather than by walking the filesystem — a percent-encoded path, a path outside the
extension allowlist, a symlink that resolves outside the claims directory, or anything that is
not a regular file is a bare 404, with no distinction from "does not exist". The viewer's
`Content-Security-Policy` on `GET /` widens accordingly, from `default-src 'none'; style-src ...`
to `default-src 'none'; img-src 'self'; style-src ...` — the one relaxation this release makes to
the CSP, scoped to same-origin images only.

### Fixed — a denial-of-service path in `parseLink`, live in the currently-shipped binary

This bug **predates** v0.3.1 and is live in the v0.3.0 binary you are upgrading from; the fix
ships in this release. `parseLink`'s bracket-matching had several quadratic-ish rescans —
repeatedly walking forward to a `]` or `)` from each `[` in the remaining text instead of
indexing them once — that produced no wrong byte of output but cost seconds to minutes of CPU on
an adversarial input. This is the smallest of **four** quadratic paths this release bounds,
each measured against a 1 MiB reviewer-authored body (the practical ceiling on a comment or
claim `body`) at the same commit:

| path | before | after |
|---|---|---|
| list continuation accumulator | 10.6s CPU, a 65 GB allocation | 23ms |
| fences indented under a list item | 8.55s CPU | 21ms |
| `parseLink` bracket rescan | 5.8s CPU | 12ms |
| fence rescan | 299ms @ 8 KiB | 5ms @ 16 KiB |

The `parseLink` case matters more on the comment surface than the CPU number alone suggests,
because `handleListComments` (`internal/serve/handlers.go`) re-renders **every** stored comment
on **every** `GET /api/comments` call — so one stored hostile body is not a one-time cost, it is
amplified across every later read of the comment panel for as long as that comment exists. The
65 GB allocation in the list-continuation path is reachable from a 1 MiB comment body with no
special privilege — any reviewer who can leave a comment could trigger it before this fix. All
four paths are bounded with an index built once per scan and guarded by a 16-shape growth sweep
plus a 1 MiB absolute-budget test in `internal/render/markdown/markdown_cost_test.go`.

### Security — an off-origin `<img>` could pass the mockup review gate, live in the currently-shipped binary

This hole **predates** v0.3.1 and is live in the v0.3.0 binary you are upgrading from. A
`layout: mockup` claim's `raw_html` is the one field DossierX renders unescaped, and it is
allowed only for an allowlisted module, only with `raw_html_reviewed: true`, and only through a
tag/attribute allowlist in which `<img>` may carry a **relative** `src`. That relative-only test
was a regular expression that treated the literal bytes `//` as the only authority prefix.
Browsers normalise `\` to `/` in the authority position of an `http`/`https` URL, so
`src="\\evil.example/p.png"`, `src="/\evil.example/p.png"` and `src="\/evil.example/p.png"` all
resolve off-origin — and all three passed the gate, meaning a **reviewed, locked** mockup claim
could load a third-party image (and leak the viewer's IP, User-Agent and referrer) from a page a
human had signed off on. `src="//evil.example/p.png"`, the one spelling the regex knew, was
correctly refused, which is why the gap was invisible in review.

The fix is not a stronger regex. The same rule was written **four** independent times in this
repository — the mockup gate, `internal/lint`'s markdown scanner, and two places in
`internal/render/markdown` — and the other three already knew that a backslash counts as a
slash; the weakest copy had simply never been brought up to the others. All four now call one
leaf package, **`internal/urlsafe`**, which imports nothing else in the module (so both the
renderer and the linter can depend on it) and exports a single by-construction gate:
`IsOffOrigin` refuses any explicit scheme, all four authority spellings, a root-relative path, a
query or a fragment, after HTML-entity-decoding and after stripping every ASCII control byte and
space — so `&#47;&#47;host`, `ht&#9;tp://host` and `\x01//host` are refused on the same terms as
the bytes they decode to. The four local copies are deleted rather than left delegating.

Four behaviour changes follow, all narrowing what is accepted, all confined to the mockup
`<img src>` gate. A **root-relative** `src` (`/foo.png`), an **empty** `src`, a `src` carrying a
**query** (`x.png?v=2`) and one carrying a **fragment** (`x.png#frag`) are now refused where they
previously passed. The last two are the ones most likely to surprise: a cache-busting query on an
otherwise relative path is same-origin, and the finding still describes it as a non-relative URL.
The gate implements the rule as written — a relative path with no scheme, no authority prefix, no
leading `/`, no `..`, no `#` and no `?` — and is deliberately stricter than the security argument
alone requires. No fixture or shipped mockup uses either form. Relative forms are unaffected:
`../diagrams/x.svg`, `./x.png` and `x.png` still pass, including the `../` form the shipped Google
Cloud Console mockups use.
Nothing on the claim-body image path or the markdown link-scheme allowlist changed: those were
already on the strong rule, and their accept/reject decisions are byte-identical across this
change.

## [0.3.0] - 2026-07-28

The agent-first restructure. DossierX has two users with opposite needs: an **agent** that
operates it, and a **human** who reviews what the agent did. Until now both were half-served by
one command line. v0.3.0 gives each its own surface and takes the other away — the agent gets a
20-command machine-readable CLI, the human gets the viewer and one command (`dossierx serve`).

Alongside the split, this release closes the gap that made the split worth making: until now a
locked claim could be hand-edited and **nothing would notice**. The new lock ledger records what
was approved, when, by whom, and on whose words, and a gate compares the claims against it in
`dossierx check`, in a pre-commit hook, and in CI.

**This release is not backward compatible at the CLI.** Twelve commands were removed and four were
moved. The migration table below maps every one of them.

### BREAKING — every existing project must run `dossierx migrate --adopt` once

**This is the one change that breaks a project rather than a script, and it breaks on the first
`dossierx check` after the upgrade.** If your project has ever locked a claim, run this once,
before you lean on the hook or CI:

```sh
dossierx migrate --adopt --dry-run   # look first: it names every artifact it would adopt
dossierx migrate --adopt
```

then commit the rewritten `.dossierx-lock-store.json` — and the `.dossierx-comment-digest.json` the
same run creates — in the same commit as the claims they now cover. `--adopt` is required, so a bare
`dossierx migrate` refuses with `missing_flag` rather than guessing at a migration you did not name.
`--dry-run` lists every claim and build order it would adopt and writes nothing.

**There is deliberately no `--reason`,** and that is the one place this command breaks the pattern
every other record-writing verb follows. They take the human's words because a human approved
something. Nobody approved this. Every record the migration writes carries a fixed reason saying so
— *"grandfathered by `dossierx migrate --adopt`: locked before this project had a lock ledger;
content adopted as-found on migration day, never approved by anyone"* — and `grandfathered: true`,
permanently. A human-supplied reason would make an adoption read like an approval in a ledger diff,
which is exactly the confusion the fail-closed decision exists to remove.

**Why this is not automatic any more.** Earlier in this release cycle it was: a pre-ledger project
was grandfathered in on its first plain `check`, which observed whatever the claims said at that
moment and recorded them as approved. It was convenient and it was unsound, because *adoption is
the one operation in the design that manufactures approval out of nothing*. A gate that performs it
on sight rewards deleting the ledger with a clean bill of health over content nobody reviewed, and
turns "arrive with no ledger" into a universal bypass. Review rounds then tried to distinguish an
honest v0.2.x store from a deliberately downgraded one using evidence inside the project, and could
not: `locked_at` shipped in v0.2.0 (verifiable with `git show v0.2.0:internal/lock/lock.go`), so no
field, no timestamp and no sibling file tells the two apart. When no predicate can be trusted the
answer is not a cleverer predicate — it is to stop guessing and make a human decide, once.

So **adoption now fails closed, in every run**: a missing or unreadable ledger never grandfathers
anything on plain `check`, on `--validate`, or on `--staged`. The only code path that writes an
adopted record is the one a human invokes deliberately.

What the migration does and does not do: it hashes each currently-locked claim and each locked
build order **exactly as they sit on disk** and records them as the baseline, marked as adopted
rather than approved, permanently — an adopted hash is content that was *observed*, not reviewed.
It changes no claim's `status`, resolves no thread, and clears no `review_pending`. Read the claims
before you run it; nothing in the command can check them for you. It is an upgrade step and never a
recovery tool: on a project that already has ledger coverage it refuses, and reaching for it to
silence a gate on a project that *has* a ledger would record tampered bytes as approved, which is
precisely what the fail-closed rule exists to prevent.

Skipping it is loud rather than silent. `dossierx check` fails on the lock-ledger gate with the new
project-scoped finding **`lock-ledger-adoption-required`**, under the `integrity_failed` code the
gate already uses — **no new `error.code` was added for it**. It is one finding naming the
migration, deliberately in place of one `lock-ledger-missing` per claim: repeating "this claim is
locked with no record" N times would attach a recovery (set it back to draft and re-lock) that is
actively destructive advice at a project which has done nothing wrong. It is also genuinely
distinct from its neighbour, and the **lock store itself** is what tells the two apart, with no git
history required: `lock-ledger-adoption-required` fires when the store is **present** and still on
the pre-ledger schema (benign; recovery is the migration), `lock-ledger-absent` when the store
**file is gone** while locked claims remain (tampering; recovery is version control).

Running it twice is refused, with the new `error.code` **`already_migrated`** and a `data.mode` of
`already_covered` or `nothing_to_adopt` so a caller can tell the two apart — a migration that can be
re-run is a laundering command, since deleting one record and re-migrating would re-sign the edit it
covered as approved. A pre-commit hook and a CI run fail the same way as `check` — the run that
would previously have blessed a project quietly now refuses it.

### `check --staged` judges the git index, and only the git index

`--staged` reads the **git index** — what the commit will actually contain — with `git show`
instead of the worktree, and writes nothing. That is what makes it meaningful in a pre-commit hook,
and it is unchanged. What it does **not** do is read git history: it judges one tree, and its
verdict is identical in every clone, at every depth, on every branch.

**Removed before release: the parent-commit comparison.** An earlier build on this branch had
`--staged` resolve the parent of the commit under judgement and compare the two, reporting a removed
lock ledger or a repointed `claims_dir` as **`integrity-store-removed`** and
**`claims-scope-narrowed`**. Both rules are gone, along with the shallow-clone advisory in
`data.next_steps` that told you to set `fetch-depth: 0`. **The shipped CI template changed with
them**: it is now a single `dossierx check` step and pins no `fetch-depth`, because a shallow
checkout is a complete tree and one tree is the whole evidence base. The second `check --staged`
step is gone from CI too — on a fresh checkout the index, the worktree and `HEAD` are three names
for one tree, so it re-ran the same rules over the same bytes. `--staged` itself is **not**
deprecated; its home is the pre-commit hook, where the index and the worktree genuinely differ.
The diagnosis behind the removed rules was right — a change that takes a claim's
evidence together with whatever was left to judge it against leaves the gate nothing to disagree
with, so it reports `ok: true` — but the fix was in the wrong layer. The parent commit is outside the
*commit* and not outside the *committer*: git history is written by exactly the party the gate
constrains, so `--orphan`, a rebase or a second config file switched the comparison off without
looking unusual in a log, and every other in-repo source of that evidence has the same property.
It also could not see intent the tree does not record, and charged two ordinary git operations for
that: a legitimate **`git revert`** of a commit containing a claim lock was refused (the revert
removes that lock's records, byte-identical to erasing them) and, because git does not run
`pre-commit` for `revert`, it landed locally at exit 0 and only CI objected; and a project that was
**new in a commit** was audited against an unrelated retired project's ledger in a monorepo and
refused with findings naming another project's claim ids. A control that a rebase disables and a
revert trips is not a control, so it was removed rather than patched.

**What this costs, and the boundary it leaves.** Removing the comparison gives up the detections
that needed a second tree: those where one change writes a claim's evidence **and** whatever was
left to judge it against, so that the single-tree gate has nothing surviving to disagree with.
Earlier revisions of this entry tried to count those cases and publish the list. The count rose
every time someone looked harder, and two of the published statements turned out to be false, so
the list is gone and the boundary is stated as what it always was:

> **An in-repo ledger cannot attest anything against the person who can write it.**

The gate catches every change that leaves a surviving file **disagreeing** with the one that moved,
which is what drift and tampering usually look like: an agent editing a locked claim, a careless
hand-edit, a bad merge, a status flipped by hand, a deleted approval record, an erased comment
thread. Each leaves the files nobody touched disagreeing with the one that was, and that
disagreement is a named finding. What has nothing to disagree with it is not caught — a record's
`reason`, `at` and `actor` are testimony, not signature, and no rule compares them to anything. Nor
is **coordinated** change — a claim and its ledger record rewritten together, in one commit, by
someone entitled to write both. The sharpest form is not a deletion at all: unlock a locked claim, rewrite its body,
re-lock it (minting a fresh record whose hash correctly covers the new content), then hand-edit
that record's `reason`, `at` and `actor` back to the original approval's values. `check` and
`check --staged` both report `ok: true`, over a ledger crediting a human who approved nothing. That
is an illustration, not an inventory.

No in-repo mechanism closes it, which is the general form of why the parent comparison failed: any
evidence the tool consults lives in the repository, and the repository is writable by the person
being gated. It is caught where a control the committer cannot rewrite actually lives — **branch
protection with a required CI check, plus review of a diff in which such a change is loud**, since
it is a hand edit of a tracked JSON store whose whole purpose is to be read in a diff, sitting
beside the claim change it was made to permit. **DossierX detects; the forge enforces.** The
boundary is pinned rather than asserted, in `internal/check/staged_no_parent_test.go` end to end
and `internal/lock/audit_boundary_test.go` at the audit layer, each beside its "the uncoordinated
half is still refused" assertions.
[FORMAT.md](FORMAT.md#what-the-gate-detects-what-it-does-not-and-where-the-rest-is-caught) states
the principle in full, including the one direction that would move it: signing the ledger with a key
held outside the repository.

Everything else in the gate is untouched and still single-tree: `lock-content-drift`,
`lock-ledger-missing`, `lock-ledger-deleted`, `lock-ledger-released`, `lock-ledger-orphan`,
`lock-ledger-abandoned`, `lock-ledger-absent`, `lock-ledger-unreadable`, `lock-ledger-downgraded`,
`lock-ledger-adoption-required`, `comment-ledger-drift`, the `comment-digest-*` family and the
`build-order-*` family, plus fail-closed adoption and `dossierx migrate --adopt`.

### Migration — every retired command and its replacement

| Retired | Replacement | Notes |
| --- | --- | --- |
| `dossierx lint` | `dossierx check --validate` | `--validate` is a **read-only** run — no claim files, no lock store, no `.catalog.json`, no viewer. Plain `check` writes all four. |
| `dossierx lint --json` | `dossierx check --validate` (JSON is the default format) | Findings are `data.lint_findings[]`, in snake_case: `lint`, `claim_id`, `severity`, `message` (the old bare array used Go field names). |
| `dossierx catalog` | `dossierx check` | It was a stage of `check`, exposed as a verb only because the extraction had no Go API. |
| `dossierx render` | `dossierx check` | Same. |
| `dossierx deps <id>` | `dossierx claim show <id>` | Reports both edge directions as before, **plus** lock state, `review_pending` and its trigger, code links with drift, comment counts, and `next_actions`. |
| `dossierx stale` | `dossierx claim list --review-pending` | Names the claims and reports the count, as before. The bespoke "nothing locked" message is gone; an empty project is an empty result. |
| `dossierx coverage` | `dossierx claim list --migrated` | Reports the same ratio (`count`, `total`, `percent_of_total`) **and** names the claims. |
| `dossierx implink set` | `dossierx claim link` | Identical flags (`--module --claim --file --symbol`) and identical behavior. |
| `dossierx implink status` | `dossierx claim show <id>` | Per-claim `implemented_in[]` with a `drifted` verdict on each file. `dossierx check` still reports module-wide impl-link status. |
| `dossierx lock <id>` | `dossierx claim lock <id>` | `--reason` is required (see below). |
| `dossierx unlock <id>` | `dossierx claim unlock <id>` | `--reason` is required. |
| `dossierx flag <id>` | `dossierx claim flag <id>` | Unchanged otherwise. |
| `dossierx reaudit <id>` | `dossierx claim reaudit <id>` | `--reason` is required with `--confirm`. |
| `dossierx comment edit` | the viewer | A review history the agent can rewrite is not a review history. Still fully available over `dossierx serve`'s HTTP API. |
| `dossierx comment delete` | the viewer | Same. |
| `dossierx comment resolve` | the viewer | Advisory rights already forbade an agent acting on a human-authored thread, and every viewer thread is human-authored — so on the CLI this could only ever act on the agent's own threads. The human's **Resolve click is the approval** the lock gate waits for. |
| `dossierx comment reopen` | the viewer | Same. |
| `dossierx comment list --json` | `dossierx comment list` (JSON is the default format) | Threads are `data.threads[]` inside the standard envelope, rather than a bare array. |

Nothing was removed from the **product**: `internal/comments` still implements all six comment
operations, and `dossierx serve`'s HTTP API — which is what the viewer drives — still exposes
every one of them. Only the CLI surface shrank.

### Added — two integrity holes closed at the command that used to launder them

Both were found by reproducing them against the shipped binary, and in both the *gate* was already
correct: `check` named the tampered state at every step. What was missing is that naming it is not
refusing it, and in each case the next ordinary command wrote the evidence whose absence was the
finding — so the sequence ended green, permanently.

- **`lock-ledger-deleted` is now a refusal on `claim lock`, not only a finding.** Delete one
  claim's entry from the `ledger` map, flip `status: locked` to `draft`, rewrite the body, and
  `dossierx claim lock` used to succeed — `RecordApproval` wrote a *fresh* record over the
  rewritten content, and every check from then on exited 0 with zero findings. The claim's own
  `locked_at` stamp and dependency baselines survive the deletion (nothing removes them; `unlock`
  *releases* a record rather than deleting it), so "this engine locked it and the record is gone"
  is answerable, and locking is now refused with `integrity_failed`. The recovery is restoring the
  lock store — **not** `unlock → fix → lock`, which signs the attacker's edit. A claim this engine
  never locked is untouched by the gate, so a first lock still works normally.
- **`comment-digest-unrecorded` is now a refusal on `claim lock` and on every comment op.** An
  approval records the claim's comment digest in the same act, unconditionally — so on a claim
  whose digest key had been dropped, locking *manufactured* an entry from whatever the comments
  block said at that moment. Measured on a covered project: a human's open thread blocks the lock;
  forge `status: resolved` and drop that one key; `check` correctly reports
  `comment-digest-unrecorded`; `claim lock` then exits 0 and certifies the forged block; every
  later check exits 0. The human's objection is gone and the record says the review was clean.
  `dossierx comment add`/`reply` closed the same door from the other side — an *unknown* digest on
  a covered claim that carries threads is no longer treated as "cannot have drifted". Silent, in
  both, where evidence is honestly absent: an uncovered project, an absent digest store
  (`comment-digest-absent` is that cause, said once), and a claim with no threads at all.

### Added
- **`dossierx claim` — one noun for everything you do to a claim**: `show`, `list`, `new`,
  `lock`, `unlock`, `flag`, `reaudit`, `link`.
- **`dossierx claim show <id>`** — one call returning a claim's whole state: lifecycle status,
  lock state and timestamp, `review_pending` **and which of the three triggers caused it**, both
  edge directions (outgoing `mirrors`/`rests_on`/`governed_by`, and the derived incoming
  `mirrored_by`/`depended_on_by`), `implemented_in` with a per-file drift verdict, comment
  counts with the open thread ids, and `next_actions` — the legal next steps, computed from the
  same gates the write paths enforce, so the advice can never disagree with what the command
  would actually do.
- **`dossierx claim list`** with `--review-pending`, `--migrated`, `--drifted`, `--facet`,
  `--module`, and `--match`. `--match` is a fuzzy, ranked search over each claim's id and its
  derived title, so a human's "the retry-policy card in the contract facet" resolves to an id in
  one call; each result carries its `score` so an agent can tell a confident hit from a tie it
  should hand back.
- **`dossierx claim new <id>`** — the sanctioned way for an agent to author a claim. Since the
  release gates hand-editing claim YAML, an agent needs a way to write one at all; this writes
  `<claims_dir>/<id>.yaml` shaped to pass the lint suite immediately, validates the project with
  the new claim in it, and reports the verdict. The id grammar (`module.facet.slug`, kebab-case
  slug, module and facet the project actually declares) is enforced at the door rather than
  after the write. Draft authoring is deliberately unfrictioned: no `--reason`, no confirmation.
- **`dossierx migrate --adopt`** — the seventh noun, and the only command in the surface a
  *human* is expected to run other than `serve`. It exists because adoption stopped being
  something a `check` does on its own; see the BREAKING section above. `--adopt` is required and
  `--dry-run` previews; there is deliberately no `--reason` (see the BREAKING section).
- **`dossierx check --validate`** — a read-only run over `internal/check`'s existing non-writing
  seam (the same one `serve`'s status endpoint uses). It exists because cutting `lint` would
  otherwise have turned the per-claim authoring loop into a writer.
- **`dossierx comment inbox`** — every open thread in the project in one call, oldest activity
  first, with `--since <RFC3339>` and an echoed `cursor` to poll with. Each thread carries
  **`agent_can_resolve`**, so an agent never spends a call earning `rights_denied` on a thread
  it was never allowed to close. `--since` is inclusive of its own second: comment timestamps
  have one-second resolution, and re-reporting a thread costs nothing while missing the human's
  comment breaks the entire loop.
- **A machine contract on every command.** `--format json|text` is global and **JSON is the
  default**; every run emits exactly one envelope — `{ok, command, data, warnings, error,
  stopped_at}` — and every failure carries a stable snake_case `error.code` (`lint_failed`,
  `claim_not_found`, `rights_denied`, `integrity_failed`, `unresolved_comments`, …) so a skill
  branches on a token instead of regexing an English sentence. `message` and `hint` are prose
  and will be reworded; `code` is the promise.
- **`--dry-run` on every mutating command**, reporting what would change, what is missing, and
  what else it affects. A dry run fails *only* when it cannot compute the preview: a refusal —
  including a missing required flag — is a **successful** blocked report (exit 0, `ok: true`,
  `data.blocked: true`), because "would this be allowed?" is a question, and answering "no" is
  not an error. `claim reaudit` keeps `--confirm` as its apply gate; `--dry-run` always wins
  over it.

### Added — integrity: the lock ledger

Claims are YAML in git, so nothing can *prevent* an edit. The goal is that no out-of-band edit
of a **locked** claim is *silent*. Before this release, every one of these was invisible: a
`status: draft` flipped to `locked` by hand (walking past the lint gate, hub-gating and the
unresolved-comment gate as though all three had passed); an edited locked body with no locked
dependents; a swapped `raw_html` on a locked, reviewed, allowlisted mockup — which the viewer
renders **unescaped**; a flipped `build_role`/`section`/`order`/`emphasis`; a comment thread
deleted straight out of the YAML; a `locked` flipped back to `draft` to dodge review.

- **The lock ledger.** Every legitimate approval — `claim lock`, a confirmed `claim reaudit`,
  `build-order lock` — now records `{hash, at, actor, reason}` for the artifact it approved, in
  `.dossierx-lock-store.json`. `reason` is the human's own approving words, carried in from
  `--reason`: the one part of the record a machine cannot generate for itself. `claim unlock`
  **releases** a record rather than deleting it, so the evidence that a claim was ever locked
  survives the window in which it matters.
- **`.dossierx-lock-store.json`, `.dossierx-comment-digest.json` and `.dossierx-flag-store.json`
  are TRACKED ARTIFACTS.** Commit them; never `.gitignore` them. CI compares the claims against
  the ledger, so a project that does not track it has no gate; and a `review_pending` claim whose
  flag-store entry did not travel with it reaudits to an *empty* proposal, whose `--confirm`
  clears the human's flag having applied nothing. Documented in README, FORMAT.md, the skills,
  the hook installer's own output, and the CI workflow template.
- **A new hash, `LockedClaimHash`, separate from `ContentHash`.** It is a **deny-list** over
  every persisted claim field except `status`, `review_pending` and `comments` (each excluded
  because the engine rewrites it as ordinary bookkeeping), so a field added to the schema
  tomorrow is signed by default. `ContentHash` — the dependency-drift baseline — is
  **byte-identical to v0.2.0**: widening it would have flipped every locked claim in every
  existing project to `review_pending` on upgrade day. It covers ten of the schema's fields;
  the nine it cannot see include `raw_html`, the only path in the engine that renders author
  bytes unescaped, which is why the ledger could not reuse it.
- **The comment digest lives in its own store** (`internal/digest`, `.dossierx-comment-digest.json`),
  refreshed on every legitimate comment write. Putting it in the lock store would have made
  `dossierx serve` a lock-store writer and falsified this release's own headline guarantee.
- **Ten named findings**, stable strings the hook, CI and the skills branch on:
  `lock-ledger-absent` (locked claims but no ledger file — a hard error, never a silent pass,
  because "no ledger means bless everything" would make `rm` the universal bypass),
  `lock-ledger-missing`, `lock-ledger-released` (a `locked` claim whose only record was released
  by an unlock — a released record is not a standing approval, and the hash still matches because
  the hash excludes `status`, so `lock` → `unlock` → hand-edit `status:` back was otherwise a
  complete bypass that fired no rule at all), `lock-content-drift`, `lock-ledger-orphan`,
  `lock-ledger-abandoned` (an unreleased record whose claim FILE is gone — every other per-claim
  rule is driven by the claims that exist, so deleting one walked past all of them at once and
  there is no `claim delete` verb to have made it deliberate), `comment-ledger-drift`,
  `build-order-content-drift` and `build-order-ledger-missing` (a locked
  `.build-order.<module>.json` is what an implementing agent actually builds from; its approval
  record was being written and never read, so reordering the phases or splicing the frozen
  `hashes` baseline changed the plan with no finding anywhere), and
  `lock-ledger-unreadable` (a ledger that exists but will not parse fails closed and loudly).
  The gate is deliberately **not** a lint: registering these in the lint registry would let one
  tampered file freeze locking project-wide and stop the viewer regenerating. It runs as
  `check`'s last step, after the catalog and viewer are written — a disputed project still
  regenerates its documentation, it just does not exit 0.
- **`dossierx check --staged`** judges the **git index** — what the commit will actually
  contain — and writes nothing at all. `project.config.yaml`, every claim, the lock ledger, the
  comment digest and every locked build order all come from that one snapshot, so no unstaged
  edit can change the verdict on a commit that does not carry it. Claim content is read through
  a single `git cat-file --batch` over the index's own object ids, never conditionally off disk:
  git's stat cache and its `assume-unchanged` / `skip-worktree` bits are attacker-writable, and
  a gate whose evidence source they can choose is not a gate. Outside a work tree it warns and
  exits 0.
- **A pre-commit hook installer**, `scripts/install-git-hook.sh` (plus `install-git-hook.ps1`
  for PowerShell). It **asks before writing anything**, resolves `core.hooksPath` instead of
  assuming `.git/hooks` (so a repo using husky/lefthook is installed into *its* hook directory,
  never hijacked), handles linked worktrees, refuses to replace a foreign hook without
  `--force`, and is a no-op when re-run. The hook body is embedded in the one file so an agent
  can drop it into a project that has the binary but not this repository. In a repository holding
  **more than one** DossierX project the hook checks **every** one of them, in index order;
  `DOSSIERX_CONFIG` narrows it to a single project but is never required. It also refuses the
  commit — rather than reporting "no project here, skipping" — when it located a config that
  `dossierx` then returned `config_not_found` for, since a gate that cannot run must not pass.
- **A CI workflow template**, `scripts/ci/dossierx-check.yml`, to copy into a consuming
  project. **CI is the authority, not the hook**: git does not run `pre-commit` for a clean
  merge, a rebase, a cherry-pick or a revert, `--no-verify` is one keystroke away, and most
  contributors never installed the hook. If you adopt one of the two, adopt CI.
- **Adoption**, covered in full by the BREAKING section at the top of this release. In brief: a
  pre-ledger project is no longer grandfathered by any `check` and is adopted only by
  `dossierx migrate --adopt`, with claims and already-locked build orders adopted in the same act
  (splitting the two halves across the ledger line would leave a project half-covered).
- **`check --staged` judges the git index and nothing else** — no parent-commit comparison, no git
  history, the same verdict in every clone. See the `--staged` section above for what that does and
  does not detect, and where the remainder is caught.
- **Moving `claims_dir` needs no flag, and there is deliberately no escape hatch.** An exemption
  switch on an integrity gate is the attack, since the party who reliably remembers to set it is
  the one who read the source looking for it. None is needed either: move the claims and the
  stores in the **same commit**, keeping the claim files byte-identical, and it passes because
  every locked claim is still reachable and still hashes to its existing record. A repoint that
  **strands** locked claims fails from state alone, as `lock-ledger-abandoned` once per claim —
  their records are left naming claims the project can no longer see.

### Added — graph integrity and readability

- **New `self-edge` lint** (error): a claim may not name its own id in `rests_on`, `mirrors`, or
  `governed_by`. A self-edge is trivially satisfied by every content rule — a claim always
  equals itself, always mirrors itself back, always resolves — so it asserted nothing while
  looking like a well-formed edge.
- **New `governed-cycle` lint** (error): `governed_by` is now cycle-checked, with its own
  message distinct from `cycle`'s. Following `governed_by` must reach `type: none` in finitely
  many steps; a cycle means a set of claims whose authority rests only on each other, which is
  to say on nothing.
- The `cycle` lint's depth-first search is now an **explicit stack** rather than recursion. Its
  depth was the longest authored edge chain in the project, with no engine-imposed bound.
- **Readable edge labels in the viewer** (issue #11). A claim-to-claim edge used to render as
  its raw id. It now renders as a derived label with prefix elision keyed on how far the target
  is from the claim doing the pointing — bare within the same facet, `facet › Label` across
  facets, `module · facet › Label` across modules. The machine id stays reachable via
  `data-claim-id` and a tooltip, and an id that is not exactly three non-empty segments renders
  as the **raw id, verbatim** — never a partial label — because rendering does not run the lint
  suite.

### Fixed

- **`dossierx build-order propose` now releases the approval it discards, and a hand-cleared
  `"locked": false` is a finding no matter what else was edited beside it.** The build-order
  orphan rule could only identify a *lone* flag flip: it re-signed the artifact as if the flag
  were still true and required that hash to match the record, because without that test it could
  not tell a hand edit from the honest window between a re-propose and the lock that follows.
  A content edit re-signs to something else, so flipping the flag **and** gutting the phases in
  the same write was strictly quieter than flipping the flag alone — `check --validate` reported
  `OK`, exit 0, and offered `dossierx build-order lock` as a next step over a sequence nobody
  approved. `propose` now writes the truth instead of leaving the gate to guess it: it releases
  the module's ledger record as it overwrites the artifact (under the lock-store sentinel, held
  across both writes, so contention refuses with nothing written). The honest window is therefore
  the only unlocked artifact whose record is *released*, and the rule needs no exception —
  any unlocked artifact under a **standing** record is `build-order-ledger-orphan`. The
  `--dry-run` discloses the release as a side effect.
- **A run that adopts a comment digest says so.** `lock.PrepareStore` left the adopted ids on the
  store and `cmd/dossierx` dropped them, so `dossierx check` printed `ok:true`, zero findings,
  exit 0 on the very run that recorded a hand-added comment block as truth. The ids now ride the
  same channel as the adopted claim records, reaching `data.comment_digests_adopted` and the
  envelope warnings — with the recovery that actually applies, which is **not** a re-lock: no verb
  in this binary clears a recorded comment digest, so the only way back is version control.
  Deliberately silent when the digest store is being *created* (a new project, or the one-time
  `migrate --adopt` crossing), where every block is adopted by definition and nothing has been
  laundered.
- **`claim lock --dry-run` no longer previews a lock the real run refuses.** Its
  `claim_is_draft` precondition read the claim file's own `status:` line — the exact line a hand
  edit rewrites — so a claim flipped out of locked without an unlock, still carrying a standing
  approval, previewed as lockable and was then refused `already_locked`. The preview now asks the
  question the real run asks, as a `no_standing_ledger_record` precondition. The refusal's
  `error.hint` also splits by state: a claim whose file says `locked` is pointed at `unlock`,
  while one that says `draft` under a standing approval is told to **restore from version control
  first**, since unlocking there accepts the edit that caused it.
- **A comment op refused because the digest store is unreadable now reports
  `comment_digest_unavailable`, not `internal`.** `internal` is defined as an unclassified
  failure — "a bug report, not a branch target" — and the reflex it invites is a retry. This
  refusal is deterministic and keeps failing identically until `.dossierx-comment-digest.json`
  is restored, so classifying it as internal sent a caller into a retry loop over a write. The
  code carries the fact that makes it actionable: **nothing was written**.
- **`build-order lock` refusing a hand-edited artifact now reports `build_order_hand_edited`,
  not `build_order_refused`.** Every recovery documented for `build_order_refused` is a repair
  to the *claims* (lock the remaining ones, resolve a thread, set a missing `build_role`, break
  a `rests_on` cycle). In this refusal the claims are correct and the *artifact* is not, so an
  agent following any of them inspected correct claims, found nothing to fix, and looped. The
  recovery for the new code is the one that works: re-`propose`, then `lock`.
- **A fresh project now acquires its comment digest store at the moment its lock store is
  created**, not only when an older project migrates across. A project that reached
  ledger-covered through its first `claim lock` never ran a migration, so it ended up
  ledger-covered with no digest store — on disk, indistinguishable from a project whose digest
  store had been **deleted**. Deleting the store from an already-covered project is still never
  silently re-created, so the deletion stays visible to the gate.
- **`FORMAT.md` no longer states that `governed_by` is hub-gated.** Hub gating walks `mirrors`
  and `rests_on` only, so a doctrine claim named *only* by `governed_by` is not gated — a reader
  who believed otherwise would drop the redundant `rests_on` edge and lock against an unapproved
  doctrine claim. `FORMAT.md` also no longer claims there is deliberately no comment-digest
  absence rule; `comment-digest-absent` ships, and its real boundary is now documented.
- **The viewer's 💬 chip now appears on every card, not only on cards that already have a
  thread** — so the first comment on a claim can actually be opened, which was the whole
  premise of the human review loop. Two gates were involved and both had to move together: the
  server emitted no chip for a zero-thread claim, and the client hid any chip reading `0`, so
  an empty chip would have vanished the moment it was clicked. Empty chips are now hidden only
  when no live comment API answered — the static `file://` export, where there would be nothing
  to open — and the three chip states (`--open`, `--resolved`, `--empty`) are mutually
  exclusive, so "no comments" no longer reads as "everything raised was settled".
- **Every command the engine advises you to run is a command that exists** — and, where the
  verb requires it, one that would succeed as printed. `check`'s next steps named five retired
  invocations (`dossierx lock`, `dossierx reaudit`, `dossierx implink set`, and a
  `dossierx comment resolve` that this release deliberately removed from the CLI); they now
  name `claim lock … --reason`, `claim reaudit`, `claim link`, and — for an open thread — the
  human's viewer rather than any agent-runnable command, because resolving a thread is the
  approval itself. `lock`'s and `build-order propose`'s comment refusals and `build-order`'s
  cycle diagnostic were stale in the same way and were corrected alongside. This matters more
  than the wording suggests: the v0.3.0 skills tell an agent to read `next_actions` and
  `error.hint` instead of re-deriving the lifecycle, so a stale hint is advice an agent acts on.
- **Generated viewers no longer advertise a deleted command.** Every `viewer/index.html`
  carried `generated by dossierx render … re-run "dossierx render"`; the banner now names
  `dossierx check`, which is the command that actually regenerates it.
- **`claim lock` refuses a claim that is already `locked`** (`already_locked`), instead of
  re-signing the ledger over whatever the file currently says. Re-locking was the single command
  that laundered every gate this release adds: it re-stamped the approval over drifted content,
  re-snapshotted the dependency baselines, cleared `review_pending` with no diff shown, and left
  the human's `claim flag` entry stranded where `reaudit` could no longer reach it — all at
  exit 0, on the verb the drift finding itself names. `lock --dry-run` had reported
  `blocked: true` for this case all along; the write path now agrees with its own preview.
- **A comment write no longer re-blesses a tampered `comments:` block.** Every comment operation
  recorded the digest unconditionally, so a single ordinary `comment reply` — on an unrelated
  thread, on the same claim — erased a standing `comment-ledger-drift` finding and made a
  forged `resolved` the recorded truth. The digest is now compared against the claim as it was
  read, and a disagreement refuses the write (`comment_digest_drift`) rather than overwriting the
  record. Adoption on a never-seen store is unchanged.
- **The pre-commit hook no longer refuses every commit in a repository whose project lives
  under a non-ASCII path.** git's `core.quotepath` defaults to *true*, so the hook's
  `git ls-files` discovery query got a project at `café/project.config.yaml` back as the
  C-quoted string `"caf\303\251/project.config.yaml"` — surrounding double quotes and all.
  Handed to `--config`, that names no file, `dossierx` answers `config_not_found`, and the
  hook's (correct) rule that a config it discovered but cannot open is a refusal rather than a
  skip did the rest: **every** commit refused, on every branch, for every developer, including
  commits touching no claim at all, until somebody uninstalled the gate. Discovery now passes
  `-c core.quotepath=false`, the same override `check --staged`'s git runner has always used.
  Pinned by a `scripts/hook-smoke-test.sh` case that asserts both halves — an honest commit
  still passes, and a tampered locked claim is still refused — because "still refuses" alone is
  satisfied by a hook that refuses unconditionally, which was the bug. The hook body's marker is
  now `pre-commit v4` (see the two entries below for what v4 added); re-run the installer to
  pick it up. A v3 install is classified `outdated` and replaced cleanly by re-running the
  installer — no `--force` needed.
- **`check --staged` no longer disarms itself when `claims_dir` points outside the config's own
  directory.** Every git pathspec was built relative to the *config file's* directory, so the
  ordinary monorepo layout — `docs/project.config.yaml` with `claims_dir: ../claims` — produced
  the spec `../claims`, which mapped to the deliberate "no index here" escape hatch and exited 0
  having evaluated nothing. A tampered locked claim committed with **no hook output at all**.
  The git runner now anchors itself at `git rev-parse --show-toplevel` and takes the project's
  position from `--show-prefix` (asked of git rather than derived by string arithmetic, so
  macOS's `/var` vs `/private/var` symlink cannot desync it); the escape hatch is now reached
  only when a path is genuinely outside the work tree. `data.staged_files` consequently carries
  repository-relative paths — identical in the layout where the config sits at the repo root,
  and only different for the layout that used to be broken.
- **A skipped `check --staged` is no longer indistinguishable from a pass.** `--format json`
  never printed the skip warning and the hook branched on the exit code alone, so a gate that
  evaluated nothing looked byte-identical to a clean run. The hook (`pre-commit v4`) now matches
  `data.skipped`, re-runs in text mode so the reason reaches the screen, and **refuses** the
  commit, naming the likely cause and the `--no-verify` hatch.
- **An untracked `project.config.yaml` no longer judges tracked content.** `check --staged` fell
  back to the *worktree* config whenever the config was merely untracked — and an untracked
  config can be edited without staging anything, so pointing it at a pristine decoy directory
  made the gate report `OK`, exit 0, over an index carrying a tampered locked claim. It now
  refuses with a distinct, non-skippable error unless the index genuinely holds nothing to judge
  (no claim blob, no lock ledger, no comment digest store), which keeps the first-commit case the
  fallback exists for working. Ordinary repository YAML — workflows, chart values — does not
  decode as a claim, so a repo full of unrelated YAML is not turned into a refusal.
- **The viewer's browser suite is actually run.** `viewer-tests/` is a separate Go module (it
  needs `chromedp`; the engine's `go.mod` stays cobra + yaml.v3), which means the root
  `go test ./...` cannot descend into it — and until now nothing else did either: no CI job, no
  Makefile target. Its assertions against the viewer's inline JavaScript, including this
  release's comment-chip suite, had only ever executed on a maintainer's laptop while CI was
  green on three platforms. There is now a `viewer` CI job running it against the runner's
  headless Chrome, and a `make viewer-test` target (plus `make hook-test`) so both
  outside-the-root-module suites are reachable locally. The job sets `DOSSIERX_TEST_BROWSER`
  explicitly, because the suite *skips* when it cannot find a browser and a skip in CI is
  indistinguishable from a pass. `tests/nested_module_coverage_test.go` fails the build if a
  nested module is ever added without both.
- **`check --staged` reads `project.config.yaml` from the index as well.** It read the claims,
  the lock ledger and the comment digest from the git index and the config from the working tree,
  so an UNSTAGED one-line `claims_dir:` edit pointing at an empty directory enumerated zero
  claims, audited zero claims and passed every commit that followed. The gate now judges one
  consistent snapshot.
- **`check --staged` no longer trusts `git diff` to decide which files it may read from the
  worktree.** git deliberately omits paths carrying the assume-unchanged or skip-worktree bit, so
  those were precisely the paths whose worktree copy was read in place of the index blob —
  the substitution `--staged` exists to prevent. Both cases are pinned end to end in
  `scripts/hook-smoke-test.sh`.
- **`review_pending` reconciliation consults the flag store.** `check` re-derived only two of the
  three triggers, so a `review_pending: true` line deleted by hand (or by a bad merge) on a
  *flagged* claim was never restored and never reported: the claim vanished from
  `claim list --review-pending`, `reaudit` refused it, and the recorded doc/code mismatch became
  unreachable. It now uses the same shared predicate the comment ops and `reaudit` use.
- **`comment inbox` no longer drops a thread the human REOPENED.** `last_activity` was the newest
  reply's timestamp, or the thread's own creation time — never the resolve or reopen — so a
  reopened thread's activity sorted *before* any cursor the agent already held and disappeared
  from every incremental `--since` poll, which is exactly the message the inbox exists to deliver.
- **`comment inbox --since` validates its argument.** A malformed value answered `ok: true` with
  an empty inbox and echoed the bad value back as the next cursor, so the failure was
  self-perpetuating and byte-indistinguishable from "the human left you nothing new". It is now
  refused (`bad_request`) and normalized to UTC before comparison.
- **`build-order lock` on a stale order returns `build_order_stale`**, the code the skill's
  refusal table has always documented, instead of the generic `build_order_refused` whose three
  documented recoveries do not apply — leaving an agent that branches on `error.code` (as it is
  told to) with no reachable path to "re-propose, then re-lock".
- **`check`'s next steps only name a claim that would actually lock.** It named the first draft
  claim in load order without evaluating the gates, so on a module drafted alongside its own
  dependencies — including this repository's own shipped fixture — the one command an agent is
  told to trust exited 1 with `lint_failed`. The example is now chosen through the same gate
  evaluation `claim show` reports, and when nothing is lockable yet it says so.
- **A noun with no subcommand emits an envelope.** `dossierx claim`, `comment`, `build-order` and
  `skills` printed help prose on stdout and exited 0, so an agent that dropped a subcommand got
  the success signal plus output its JSON parse throws on — and no way to tell that from a
  command that genuinely had nothing to report. They now behave like any other bad invocation:
  one envelope, `usage`, exit 1, with `error.hint` naming the available leaves. An unknown leaf
  (`dossierx claim nonsense`) lands in the same place and names what was typed.
  `dossierx version` is the verb that reports the version; `--version` is unchanged.

### Changed
- The CLI is **20 leaf commands under 7 nouns**, down from 26. A test pins the exact set, so
  adding to the surface is a decision someone makes on purpose.
- `--reason` is **required** on `claim lock`, `claim unlock`, `claim reaudit --confirm`, and
  `build-order lock`. Under the new split the human never types these — they say "good, lock it"
  in chat and the agent executes — so `--reason` is where the human's own approving words enter
  the record.
- Exit codes are **unchanged**: still 0 / 1 / 2 with the meanings the README documents. The
  fine-grained signal is `error.code` in the envelope, not a new status.
- `dossierx check` now runs **four** stages, not three: lint, catalog, render, and the
  lock-ledger gate. `--validate` is the read-only form — it runs the lint gate and the ledger
  gate in memory and writes nothing, and is honest about what it therefore does not do (no
  `review_pending` reconcile, no catalog, no viewer, no source scan for code links).
- `dossierx skills export <dir>` now writes **five** skill bundles. A project that exported the
  skills before must **re-export** to pick up the new router and the rewritten companions; the
  export overwrites in place, so re-running the same command is all that is needed.

### Docs
- **README rewritten around the two roles.** It now opens on who does what, carries a
  copy-paste bootstrap block a human hands to their agent (install, export the skills, propose
  the config, *ask* before installing the git hook, prove itself with `check`, commit the
  ledger, then hand `dossierx serve` back to the human), and documents `dossierx serve` as the
  human's one command. The lint → catalog → render walkthrough and the per-verb command table
  are gone: a human is not expected to run any of it, and the CLI is now documented as a
  machine contract — the envelope, `error.code`, `--dry-run`, and the unchanged exit codes.
- **FORMAT.md gained an "Integrity invariants" section**: the two tracked ledger files, what
  `LockedClaimHash` signs and the three fields it deliberately does not, all six findings and
  the invariant each one enforces, and how one-time adoption works. It also gained the three
  **graph invariants** (`rests_on` acyclic, `mirrors` a reciprocal 2-cycle, `governed_by`
  terminating, and no self-edges in any of the three), and a short, quotable statement of the
  **markdown ceiling** — the subset `body`, `rows` cells, `steps` and comment bodies all render
  through, with everything outside it staying literal text.

## [0.2.0] - 2026-07-26

### Added
- **Comments on claims** — threaded, Google-Docs-style review discussion attached to any claim,
  so a human and an agent can talk *about* a claim without editing it.
  - New `dossierx comment` command group: `add`, `reply`, `resolve`, `reopen`, `edit`, `delete`,
    and `list` (with `--open` and `--json`). Every mutating verb takes `--as human|agent`
    (recording the actor's role, which the advisory-rights rule keys off) and takes the
    project-wide claims lock, so concurrent CLI and browser edits can't clobber each other.
    Threads and replies carry engine-minted ids; the new `comments:` claim field is `omitempty`
    and **excluded from a claim's content hash**, so commenting on a claim never rewrites an
    uncommented claim's bytes and never flips its dependents to `review_pending`.
  - New `dossierx serve`: a localhost-only HTTP server that renders the claims viewer from
    memory and exposes the same comment operations to the browser, with an interactive thread
    panel and composer, a same-origin admission layer (Host/Origin checks, no CORS), and live
    reload that pushes changes over server-sent events as claim files change on disk. Binds a
    random high port by default (`--port` to override) and never writes `viewer/index.html` or
    `.catalog.json` on a page load. **Adds no new runtime dependency** — the file watcher is a
    standard-library modification-time poll, so the runtime stays cobra + yaml.v3 only.
  - An open comment thread is now a third `review_pending` trigger on a locked claim, alongside
    dependency drift and `dossierx flag`. A claim **cannot be locked while it has an unresolved
    comment thread** (and `dossierx build-order propose` refuses a module with any open thread);
    `review_pending` clears when the last open thread is resolved, unless drift or a flag also
    stands. `dossierx reaudit` refuses a claim that is `review_pending` only because of an open
    thread — there is no content diff to confirm, so it directs you to resolve the thread
    instead. `dossierx check` reports open-comment counts per module and points its next-steps
    at the exact `dossierx comment resolve` command.
  - New `comments-unresolved` lint (warning severity): surfaces claims that still carry an open
    comment thread.
- A fourth embedded Claude Code skill, **`dossierx-comments`**, teaching an agent when to
  comment versus `flag` (the discriminator is "is there a specific proposed wording change?"),
  the advisory-rights rule (an agent never resolves a human-opened thread), and how an open
  thread gates locking. The three existing skills were updated for the new three-trigger
  lifecycle and cross-linked to it.

### Changed
- `dossierx skills export` now writes **four** skill bundles instead of three. Projects that
  previously exported the skills (e.g. into `.claude/skills/`) with the old three bundles must
  **re-export** to pick up `dossierx-comments`; the export overwrites in place, so re-running
  the same `dossierx skills export <dir>` is all that's needed.

### Docs
- Documented the `comments:` claim field and rewrote the lock lifecycle in `FORMAT.md`,
  `README.md`, and the skills to the three-trigger (dependency drift / `dossierx flag` / open
  comment thread) model with its three matching clearers.

## [0.1.2] - 2026-07-25

Consolidated audit-fix release: a deep audit against a real 202-claim consumer project
surfaced 25 confirmed defects, fixed together here rather than as a stream of point
releases. Despite adding user-facing capabilities this is a patch bump — `internal/` is not
importable, there are no breaking CLI changes, and the lock-store migrates automatically.

### Added
- `dossierx version` subcommand and a `--version` flag (previously the binary could not
  report its own version, and the release-time `-X` ldflags targeted variables that did not
  exist).
- Markdown `[text](url)` links now render as anchors in claim bodies **and** in `table`
  cells; backtick code spans now render inside table cells too. Link URL schemes are
  allowlisted (`http`, `https`, `mailto`, relative, `#`-fragment); `javascript:`, `data:`,
  and `vbscript:` are neutralized to inert text. Bare URLs are not autolinked.
- New `status-shape` lint: `status` must be exactly `draft` or `locked`.
- `rows-shape` now flags any non-string table cell (number/bool/list/map) instead of letting
  it render as Go-native text (e.g. an unquoted `1.0` silently becoming `1`).

### Fixed
- A YAML file containing a second `---`-separated document silently dropped all but the first
  claim; it is now a hard load error (one claim per file is enforced).
- `lint --json` printed `null` instead of `[]` when there were no findings, and
  error-severity findings serialized with an empty severity; both now emit correct JSON.
- `lock` / `unlock` / `flag` returned exit code 1 for an unknown claim id; they now return
  exit 2, matching the documented exit-code contract (as `deps` / `reaudit` already did).
- `build-order status` and `implink status` accepted an unknown `--module` and exited 0; they
  now reject it.
- The invalid-`layout` lint message omitted `mockup`; it now lists all seven layouts.
- Dependency-hash baselines were keyed by dependency id alone and shared across dependents,
  so locking or reauditing one claim erased another's drift baseline and that claim would
  never flip to `review_pending` when the shared dependency changed. Baselines are now keyed
  per-dependent; the on-disk lock-store is versioned and migrates automatically, re-arming
  baselines for every currently-locked claim from current content on first run so drift
  detection is live immediately after upgrade without a manual re-lock.
- `unlock` left a claim's pending flag in the flag-store, so a later dependency-drift reaudit
  could silently re-apply stale pre-unlock content; `unlock` now clears the flag entry.
- `unlock` hard-failed when the flag-store file was missing or corrupt; flag-clearing is now
  best-effort — a missing store is silent, an unreadable one warns and still returns the claim
  to draft — so the recovery escape hatch stays reliable.
- `flag` on a `table` / `steps` / `mockup` claim rewrote only the body, leaving the rendered
  rows/steps/raw HTML stale while clearing `review_pending`; `flag` is now refused on those
  structured layouts (use unlock → edit → relock).
- Build-order staleness is now computed structurally: `status` re-derives the order a fresh
  `propose` would produce from the current claims and flags the locked artifact stale whenever
  they differ. This covers every order-affecting change in one check — a covered claim's
  `build_role` or `order:` edit, a source-file rename, `rests_on` reordering, additions,
  deletions, and an excluded claim promoted into a phase (or edited to an empty/invalid role) —
  plus content edits via the existing per-claim hash. It also runs for a locked module that
  covers only out-of-scope claims, which previously escaped every check and could not be relocked.
- Build-order staleness ignored newly-added claims (an artifact could silently omit a claim);
  additions now flag the artifact stale, symmetric with deletions.
- `build-order lock` re-blessed a stale artifact without recomputing its order; it now refuses
  a stale artifact and directs you to re-propose first.
- The Build Order section was emitted without an id and hidden by the facet-tab logic on every
  view, making the feature unreachable; it now renders visibly and its cards are deep-linkable.
- A module overview/router claim was injected into every facet with the same id, producing
  duplicate ids (invalid HTML) and broken deep-links; the canonical id is now stamped on a
  single copy while the overview stays visible in every facet.
- The offline-guarantee test walked the whole repo including built site bundles, so it went
  red locally after a site build while passing on a clean CI checkout; it is now scoped to the
  engine directories with a positive control.

### Security
- The `raw_html` mockup allowlist only inspected double-quoted attributes, so single-quoted,
  unquoted, and valueless event handlers, styles, and external `src` bypassed it. It is
  replaced with a default-deny parser covering every quote form, and an `img` `src` is now
  HTML-entity-decoded and stripped of ASCII control bytes before the relative-only check, so
  neither an entity-encoded (`&#47;&#47;host`) nor a control-char-obfuscated (`ht&#9;tp://host`)
  absolute/external URL can slip past it.
- `render` and `catalog` never ran the `raw_html` gate (only `lock` did), so they could
  publish unreviewed or non-allowlisted mockup HTML into the viewer; both now enforce the gate
  and fail on a violation.

### Docs
- Corrected the build-order skill (orientation-note/overview claims do carry a `build_role`
  and render in the orientation phase). Updated the claims and code-links skills, `FORMAT.md`,
  and the marketing site to reflect the behavior above.

## [0.1.1] - 2026-07-24

### Fixed
- `layout: steps` claims rendered a numbered circle (`.snum`) that sat visibly higher than the
  first line of step text. Step text is routed through the shared markdown renderer, which wraps
  it in a `<p>`; the `<p>`'s default browser top margin pushed the text down inside the
  `display:flex` step row while the fixed-height number circle stayed flush at the top. The
  default viewer stylesheet now resets step-body block margins (first-child top / last-child
  bottom) so the number and first line align. Affects every `layout: steps` claim in any project
  using the default viewer theme; a project overriding `style.css` is unaffected.

## [0.1.0] - 2026-07-23

### Changed
- Renamed every generic "docs" placeholder to the tool's actual name, `dossierx`: CLI-invocation
  examples across comments, tests, README/ROADMAP/FORMAT, and the website; the `docs-claim:`
  source tag (including the real Go regex in `internal/implink/scan.go`); `docs-v1` naming in
  the skill docs; and the default viewer title (`"docs viewer"` → `"dossierx viewer"`).

### Breaking
- `.docs-lock-store.json` and `.docs-flag-store.json` are renamed to `.dossierx-lock-store.json`
  and `.dossierx-flag-store.json`, with no migration. An existing project's lock/flag store will
  not be found after upgrading past this release — hence the minor version bump rather than a
  patch, under pre-1.0 semver.

## [0.0.3] - 2026-07-22

### Added
- The rendered viewer's sidebar now shows a "Generated <timestamp>" footer,
  the same render-time timestamp already stamped into the leading
  generated-by HTML comment, so a reviewer can tell how fresh the page is
  without needing to view source.

## [0.0.2] - 2026-07-22

First real CI run (Linux/Windows/macOS matrix, `-race`, gofmt, golangci-lint) surfaced gaps
that only local macOS testing had missed:

### Fixed
- Two files had minor gofmt drift.
- The CLI-integration test harness built the `dossierx` test binary without a `.exe` suffix on
  Windows, so `os/exec` couldn't launch it.
- Two POSIX-permission-based tests (unreadable file, read-only directory) don't apply under
  Windows's ACL model; skipped there.
- A concurrency test's non-trimpath "negative control" assertion is inconclusive on GitHub's
  windows-latest image (trimpath-equivalent by default); skipped there, the actual positive
  guarantee (a `-trimpath` build doesn't leak paths) is unaffected and still runs everywhere.
- **Real bug:** running many `dossierx lock` invocations concurrently against the same
  `claims_dir` could fail on Windows with a transient "being used by another process" error —
  Windows's mandatory file locking can collide a concurrent atomic rename with a concurrent
  read of the same claim file, unlike POSIX's atomic rename semantics. Both the read and write
  paths in `internal/loader` now retry a few times with a short backoff, Windows-only.
- `golangci-lint` config/version pinning tightened so CI's linter binary matches this module's
  actual `go 1.26` floor.

## [0.0.1] - 2026-07-21

DossierX is a config-driven CLI that turns YAML "claims" — atomic, reviewable facts about a
system — into a linted, validated, human-reviewable HTML documentation site, with a built-in
audit trail via a lock/lint/reaudit lifecycle: a claim is freely editable while in `draft`,
gets promoted to `locked` only once it passes lint, and any subsequent drift (a changed
dependency, code that no longer matches) is surfaced as `review_pending` and resolved through
an explicit, confirm-before-write reaudit rather than a silent auto-update.

The engine originated as an internal documentation tool built and hardened against real,
multi-module projects — proving out the claim schema, the lint → catalog → render → check
pipeline, the lock lifecycle, per-module build ordering, and claim-to-code linking against
production use before anything here was written with an external audience in mind. This
release extracts that engine into its own repository and genericizes it: every project-specific
name, facet, and module that had leaked into the original code has been removed, so the only
project-specific input the CLI now reads is a project's own `project.config.yaml`.

This is DossierX's first public release. It ships the `dossierx` CLI (`lint`, `catalog`,
`render`, `check`, `deps`, `coverage`, `stale`, `lock`/`unlock`, `reaudit`, `build-order`,
`flag`, `implink`), documented in [README.md](README.md), along with three Claude Code skills
in `skills/` for projects that consume DossierX to author claims, derive build order, and link
code back to claims from within an agentic workflow.

[Unreleased]: https://github.com/BarterX-Tech/dossierx/compare/v0.5.2...HEAD
[0.5.2]: https://github.com/BarterX-Tech/dossierx/compare/v0.5.1...v0.5.2
[0.5.1]: https://github.com/BarterX-Tech/dossierx/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/BarterX-Tech/dossierx/compare/v0.4.1...v0.5.0
[0.4.1]: https://github.com/BarterX-Tech/dossierx/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/BarterX-Tech/dossierx/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/BarterX-Tech/dossierx/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/BarterX-Tech/dossierx/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.2.0
[0.1.2]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.1.2
[0.1.1]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.1.1
[0.1.0]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.1.0
[0.0.3]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.0.3
[0.0.2]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.0.2
[0.0.1]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.0.1
