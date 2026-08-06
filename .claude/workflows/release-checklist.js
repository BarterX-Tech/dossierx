export const meta = {
  name: 'release-checklist',
  description: 'Runs docs/RELEASING.md as three verification gates around a release. Agents verify; the human/orchestrator performs every irreversible action.',
  whenToUse: 'Invoke three times during a release: {phase:"pre-merge"} before merging the release PR, {phase:"pre-tag"} after merging but before tagging, {phase:"post-release"} after the Release workflow finishes. Pass {version:"v0.4.0", pr:26}.',
  phases: [
    { title: 'Pre-merge', detail: 'pin sweep, CHANGELOG shape, site invariants, the two suites go test misses, the embedded skills' },
    { title: 'Pre-tag', detail: 'CI green on the MERGE COMMIT, commit sha set in content.ts' },
    { title: 'Post-release', detail: 'verify the artifact the user sees, not the source you edited' },
  ],
}

// ---------------------------------------------------------------------------
// docs/RELEASING.md, encoded. Its central rule, quoted, because every phase-3
// agent is judged against it:
//
//   "Verify the thing the user sees, not the thing you edited. Confirming a
//    string is present in a source file proves you made an edit. It does not
//    prove the edit reached the built output, that the built output deployed,
//    or that the deployed page renders it. Those are four different claims and
//    only the last one matters."
//
// Every irreversible action — merge, tag, push, publish — is performed by the
// orchestrator between invocations, never by an agent. Agents are read-only.
// ---------------------------------------------------------------------------

const REPO = '/Users/nitinkhanna/Documents/Services/dossierx'
const GH = 'BarterX-Tech/dossierx'

// args can arrive as a JSON-encoded STRING rather than an object. When that
// happened, `args.phase` was undefined and an earlier version of this script
// fell through to a 'pre-merge' default — so a pre-tag run silently re-ran the
// pre-merge gates and reported clear. A release tool must never quietly run a
// phase other than the one it was asked for, so parse defensively and then
// REQUIRE the phase rather than defaulting it.
const a = typeof args === 'string' ? JSON.parse(args) : (args || {})

const phase_ = a.phase
const VERSION = a.version
const PR = a.pr || null
const PREV = a.previous || null

if (!phase_) {
  throw new Error('phase is required — pass {phase:"pre-merge"|"pre-tag"|"post-release"}. ' +
    'It is deliberately not defaulted: a release gate that guesses which phase it is in ' +
    'can report a clean run for checks it never made.')
}

// Same reasoning as phase_, applied to the other two identities. Both used to carry hard-coded
// fallbacks ('v0.4.0' and 'v0.3.1'), which is the phase bug in a quieter form: the run does not
// fail, it verifies the wrong release against the wrong baseline and reports clear. The v0.4.1
// post-release gate caught the previous-version fallback firing — every gate that release ran
// believing the predecessor was v0.3.1 when it was v0.4.0. Nothing was missed, because the
// agents derived the real predecessor themselves, but a gate must not depend on that.
if (!VERSION) {
  throw new Error('version is required — pass {version:"vX.Y.Z"}. It is deliberately not ' +
    'defaulted: a release gate that guesses which release it is verifying can report a clean ' +
    'run for a release it never looked at.')
}
if (!PREV) {
  throw new Error('previous is required — pass {previous:"vX.Y.Z"}, the release this one ' +
    'follows. The pin sweep and the stale-mention checks both compare against it, so a wrong ' +
    'baseline silently narrows what they look for.')
}

const COMMON = `
Repo: ${REPO}. Release under verification: ${VERSION} (previous release ${PREV}).
The checklist you are executing is ${REPO}/docs/RELEASING.md — read it before you start.

YOU ARE READ-ONLY. Do not run any git command that writes (no add/commit/tag/push/merge/
checkout/reset), do not edit any file, and do not trigger any workflow. If something is wrong,
REPORT it; the orchestrator fixes it. Reporting a problem is the job; fixing it is not.

Report PASS only for what you actually verified with a command whose output you read. If you
could not run a check, say COULD_NOT_RUN and why — never infer a PASS from a related check.
`

const RESULT = {
  type: 'object',
  additionalProperties: false,
  required: ['check', 'result', 'evidence'],
  properties: {
    check: { type: 'string' },
    result: { type: 'string', enum: ['PASS', 'FAIL', 'COULD_NOT_RUN'] },
    evidence: { type: 'string', description: 'the exact command run and the output you read' },
    problems: {
      type: 'array',
      items: {
        type: 'object', additionalProperties: false,
        required: ['severity', 'what', 'fix'],
        properties: {
          severity: { type: 'string', enum: ['blocking', 'important', 'minor'] },
          what: { type: 'string' },
          fix: { type: 'string' },
          where: { type: 'string' },
        },
      },
    },
  },
}

function run(list, phaseTitle) {
  return parallel(list.map(c => () =>
    agent(`${COMMON}\n${c.prompt}`,
      { label: c.key, phase: phaseTitle, schema: RESULT, effort: c.effort || 'medium' })))
    .then(r => r.filter(Boolean))
}

// ===========================================================================
// PHASE 1 — before the release PR is merged
// ===========================================================================
const PRE_MERGE = [
  {
    key: 'pin-sweep', effort: 'high',
    prompt: `CHECK: the version pins are moved.

The checklist is explicit that this is done by SWEEP, not by memory — it went stale through
v0.3.0 and v0.3.1 and was caught by a sweep both times. Run the command the checklist CURRENTLY specifies — read it out of docs/RELEASING.md rather
than trusting the one quoted here, because the checklist has been revised mid-release before
and a hard-coded copy goes stale. As of this writing it is:

    git grep -nE "dossierx(/cmd/dossierx)?@v|githubusercontent\\.com/[^ ]*dossierx/v" \\
      -- . ':!CHANGELOG.md' ':!docs/RELEASING.md'

Do NOT substitute a plain "grep -rn --include=...". On some machines "grep" resolves to ugrep,
whose -r skips dot-directories, so .github/ is never searched — a pin in a workflow file would
be invisible there. "git grep" has no such blind spot.

Every hit must name ${VERSION}. A hit naming ${PREV} or older is a FAIL unless it is
unambiguously historical prose (see the "remaining mentions" rule in the checklist).

Then do the part the checklist cannot: look for a pin location that is NOT in its list. The
list names README.md, skills/dossierx/SKILL.md and scripts/ci/dossierx-check.yml. If the sweep
surfaces a fourth, that is the finding — the checklist's own list has gone stale and should be
updated in the same release.

scripts/ci/dossierx-check.yml matters most: users copy that template into their own repository,
so a stale pin there ships a stale binary into someone else's merge gate.`,
  },
  {
    key: 'changelog-shape', effort: 'high',
    prompt: `CHECK: CHANGELOG.md's ${VERSION} entry exists, is dated, and is ordered correctly.

Requirements, from the checklist:
  - an entry for ${VERSION}, dated, following Keep a Changelog
  - BREAKING and SILENT-BEHAVIOUR changes called out FIRST, before ordinary bullets

The reasoning behind that ordering is stated in the checklist and is the thing to judge
against: ${PREV}'s renderer expansion changed what already-locked claim bodies rendered as,
with no edit, no content-hash change and no ledger event — 'dossierx check' reported exactly
what it reported before. A change a consumer's own gate CANNOT DETECT FOR THEM belongs at the
top of the entry, not halfway down.

So: read the ${VERSION} entry and ask, for each change in it, "could a consumer's gate catch
this on its own?" Anything that answers no must be near the top. Compare the entry's shape
against ${PREV}'s entry in the same file, which the checklist treats as the reference.

Also confirm the date is today's date, and that GoReleaser's generated notes are not being
relied on as a substitute — they are commit subjects only.`,
  },
  {
    key: 'site-invariants', effort: 'high',
    prompt: `CHECK: site/src/content.ts's release entry obeys every invariant the checklist names.

  - the 'releases' array is OLDEST-FIRST, and ReleaseTimeline treats
    releases[releases.length - 1] as current. The ${VERSION} entry must be APPENDED, not
    prepended. Verify by reading the array order, not by assuming.
  - 'tag: "Latest release"' has MOVED off ${PREV} and onto ${VERSION}. Both carrying it, or
    neither, is a FAIL.
  - 'commit' on the ${VERSION} entry is EXPECTED TO BE UNSET at this phase — the release commit
    does not exist until the tag. Confirm it is either absent or marked with a comment saying
    to set it at tag time. If it is set to a sha that already exists, that is a FAIL: it would
    be the previous release's sha under a ${VERSION} heading.
  - NO hand-typed version string was reintroduced. Every other version on the site — the hero
    kicker, the hero badge, the release-history intro, and the 'dossierx version' example —
    must derive from latestRelease / latestVersion. The checklist notes each of those four
    once had a hand-typed copy and three went stale. Grep site/src for literal "v0.3" and
    "v0.4" outside content.ts's releases array and judge each hit.

Also check site/src/sections/*.tsx and site/src/index.css for counts that went stale with the
surface change (seven nouns, nineteen leaves).`,
  },
  {
    key: 'the-two-suites', effort: 'medium',
    prompt: `CHECK: the suites 'go test ./...' does not reach.

CONTRIBUTING.md ("The two suites go test ./... does not reach") names them, and
tests/nested_module_coverage_test.go fails the build if a nested module is ever added without
both a CI job and a Makefile target — so the list cannot quietly go stale, but RUN them anyway:

    go test ./...
    make viewer-test     # viewer-tests/, a separate module (chromedp)
    make hook-test       # scripts/hook-smoke-test.sh
    golangci-lint run ./...   # try ~/go/bin/golangci-lint if not on PATH

Note on viewer-test: it SKIPS when it finds no Chrome/Chromium. A skip is not a pass. Check the
output for whether it actually ran; if it skipped, say so and report COULD_NOT_RUN for that
half rather than PASS.

golangci-lint matters specifically because 'go vet' does not catch what it catches — errcheck
with check-blank already failed CI once in this release after a clean local vet.`,
  },
  {
    key: 'skills-correctness', effort: 'high',
    prompt: `CHECK: the embedded agent skills still describe THIS release's engine.

Why this gate exists, and why it is not just another docs check: 'dossierx skills export' installs
skills/*/SKILL.md into OTHER PEOPLE'S repositories, where they become the operating instructions an
agent follows against a corpus you will never see. A stale rule here does not render a wrong page —
it teaches an agent the wrong recovery on somebody else's locked claims. Treat a wrong instruction
as BLOCKING, not minor.

The skills are go:embed-ed (skills/embed.go) and asserted on by cmd/dossierx/skills_embed_test.go,
so they ship inside the binary: a fix after the tag does not reach anyone who already installed.

Read every skills/*/SKILL.md. For each, run these four sweeps:

1. FALSIFICATION — the untrue sweep, not a mention sweep. Do not ask "does it mention the new
   feature". Ask, for each factual assertion: "did anything in ${VERSION} make this FALSE?" Check
   in particular:
     - every command, flag and noun named — against 'dossierx <noun> --help', not memory
     - every error.code and lint rule name — against internal/lint and the error-code constants
     - every count ("nineteen leaves", "seven nouns", rule counts) — against the code that pins it
     - every "as of vX" / "in vX" claim — is the version still the right one?

2. NEW BREAKING BEHAVIOUR — did ${VERSION} add a rule, refusal or error.code that can fire on a
   corpus the agent did NOT change? That is the case an agent handles worst, because its instinct
   is to hunt for what it broke. If such a change exists and no skill names it, that is BLOCKING.
   Compare 'git diff ${PREV}..HEAD -- internal/lint internal/check' against what the skills say.

3. RECOVERY REACHABILITY — for each new or changed refusal, trace the path an agent actually walks:
   error.code -> the router's recovery table -> the companion skill it routes to. If the recovery
   for a new refusal is only derivable by reading Go source or the CHANGELOG, the chain is broken.
   Say WHERE it breaks.

4. INSTALL INTEGRITY — the version pins inside the skills (there is a raw.githubusercontent pin in
   skills/dossierx/SKILL.md) must name ${VERSION}; and 'dossierx skills export' must still write
   what the repo holds. Run it into a temp dir and diff against skills/:
       cd $(mktemp -d) && go run <repo>/cmd/dossierx skills export . ; diff -r . <repo>/skills

Report a finding per skill file, naming the exact line. An empty finding list is a valid and
expected result for a release that changed no engine behaviour — say so plainly rather than
manufacturing a nit. ${VERSION} is not such a release if it added a lint rule.`,
  },
]

// ===========================================================================
// PHASE 2 — merged, not yet tagged
// ===========================================================================
const PRE_TAG = [
  {
    key: 'ci-on-merge-commit', effort: 'high',
    prompt: `CHECK: CI is green on MAIN — on the merge commit, not on the branch.

The checklist is emphatic about this distinction and it is the whole check: a green branch and
a green merge commit are different claims. Verify against the merge commit itself.

    git checkout is FORBIDDEN — read state with: git log --oneline -3 main
    gh run list --branch main --limit 10
    gh api repos/${GH}/commits/<merge-sha>/check-runs

Confirm the merge used --no-ff and produced a real merge commit (git log --merges -1 main), as
the checklist requires so the release has a merge commit to name. A squashed or rebased merge
is a FAIL — report it; do not try to repair history.

Every check on that commit must be a pass. A pending check is COULD_NOT_RUN, not a PASS.`,
  },
  {
    key: 'commit-sha-set', effort: 'medium',
    prompt: `CHECK: site/src/content.ts's ${VERSION} entry now has 'commit' set to the MERGE COMMIT's
short sha.

This is the one checklist item that deliberately cannot be done before the merge, so it is the
one most likely to be forgotten. Verify:
  - 'commit' is present on the ${VERSION} entry
  - its value equals the short sha of main's merge commit (git log --merges -1 --format=%h main)
  - it is NOT the previous release's sha
  - the 'dossierx version' example no longer falls back to "(devel)" for the current release

If it is still unset, that is BLOCKING: the site will render the release with no commit, or
with a fallback, and the checklist's own verification step will then pass against the wrong
thing.`,
  },
  {
    key: 'tree-ready', effort: 'medium',
    prompt: `CHECK: the tree is genuinely ready to tag.

  - working tree clean (git status --porcelain is empty)
  - local main is in sync with origin/main
  - the CHANGELOG's ${VERSION} date matches the date the tag will carry
  - go test ./... is green on main as it now stands
  - no file in the tree still describes ${VERSION} as unreleased or upcoming

WHICH COMMIT THE TAG GOES ON — verify this, do not assume HEAD. The checklist's command is
'git tag -a ${VERSION}' with no ref, which tags HEAD, and that is only correct when nothing has
landed since the merge. Establish the answer from the previous release rather than from the
command:

    git log -1 --format="%h %p %s" ${PREV}     # does the tag sit ON a merge commit?
    git log --oneline -1 -S'commit: "<prev-sha>"' -- site/src/content.ts

For ${PREV} the tag sits on the MERGE COMMIT, and content.ts's 'commit' stamp was added in a
LATER commit on main — it cannot be inside the tagged tree, because the sha does not exist until
that commit does. So the shape is: stamp names commit X, tag sits on commit X, and the stamp
itself lands on main after the tag.

Therefore, if anything has landed on main since the merge — the sha stamp itself will have —
tagging HEAD is WRONG. GoReleaser would stamp main.commit with a sha content.ts does not name,
so the site's 'dossierx version' example would disagree with what the binary prints. That
example is exactly what the post-release phase verifies, so the mismatch would surface late.

Report the ref the tag must go on, and the command filled in for this release:

    git tag -a ${VERSION} <the-merge-commit-sha> -m "${VERSION} — <title>"
    git push origin main
    git push origin ${VERSION}`,
  },
]

// ===========================================================================
// PHASE 3 — tagged, Release workflow finished.
// The checklist's rule governs every agent here: verify the ARTIFACT, not the source.
// ===========================================================================
const POST_RELEASE = [
  {
    key: 'release-page', effort: 'medium',
    prompt: `CHECK: the ${VERSION} release page carries its artifacts.

    gh release view ${VERSION} --repo ${GH}
    gh release view ${VERSION} --repo ${GH} --json assets --jq '.assets[].name'

The checklist requires all SIX platform archives plus checksums.txt. Count them and name any
that is missing. Also confirm the Release workflow itself passed — a failure there leaves a tag
with no artifacts behind it, which is the specific failure mode the checklist calls out.`,
  },
  {
    key: 'clean-install', effort: 'high',
    prompt: `CHECK: a CLEAN INSTALL reports the new version. This is an artifact check, not a source
check — it is verifying the ldflags reached the published binary.

    cd $(mktemp -d)
    GOBIN=$(mktemp -d) go install github.com/BarterX-Tech/dossierx/cmd/dossierx@${VERSION}
    $GOBIN/dossierx version --format text

It must print ${VERSION}. If it prints a "(devel)" fallback, the ldflags did not apply — the
checklist points at the comment in .goreleaser.yaml about '-X main.version' needing the 'main.'
prefix rather than the full import path. Report that as BLOCKING with the actual output.

Also run 'dossierx version --format json' and confirm the commit and date fields are stamped
rather than empty.`,
  },
  {
    key: 'site-deployed', effort: 'high',
    prompt: `CHECK: the site actually redeployed, and the deployed bundle is the one that was built.

Two separate claims, and the checklist warns the first silently fails:

1. deploy-site.yml triggers ONLY on changes under site/**. A release touching no site file
   publishes nothing and the site keeps serving the previous version. Check whether it ran for
   this release:
       gh run list --workflow deploy-site.yml --limit 5 --repo ${GH}
   If it did not run, that is the finding, and the fix is a workflow_dispatch.

2. The deployed bundle is the one that was built. Vite content-hashes its assets, so fetch the
   live index.html, extract the asset hashes from it, and compare against a local build's
   dist/. A match rules out a stale cache or a failed deploy serving an older build. If you
   cannot build locally, at minimum confirm the live asset hashes CHANGED from what the
   previous deploy served.

Use WebFetch for anything live.`,
  },
  {
    key: 'rendered-pages', effort: 'high',
    prompt: `CHECK: the rendered pages READ correctly. This is the check the checklist says has
historically failed, and it is explicit that grep cannot do it:

  "since the version strings became derived, they are minified into variables and a grep for
   the release tag in the bundle returns nothing whether the page is correct or broken. A zero
   from a 404 or a bad selector looks identical to a zero from a clean fix."

So: FETCH BOTH PAGES AND READ THE TEXT.
  - the site root /
  - /releases.html — note this is a SECOND ROLLUP ENTRY POINT, not a route. '/releases/' is a
    404. Fetching the wrong URL and finding nothing is exactly the false negative above.

Read what a visitor sees and confirm: ${VERSION} is presented as the current release, the hero
kicker and badge name it, the release history lists it, and the 'dossierx version' example
shows ${VERSION} rather than a "(devel)" fallback or ${PREV}.

Quote the actual rendered text you read for each claim. "The page contains the string" is not
what is being asked.`,
  },
  {
    key: 'historical-mentions', effort: 'high',
    prompt: `CHECK: every remaining mention of ${PREV} or older is HISTORICAL, and none is a stale
claim about what is current.

The checklist is explicit that this requires reading in context, and that over-correcting is
the failure mode: "Most prose about a past release is correct and must not be bumped —
'v0.3.0 made the machine contract the product's spine' describes history; rewriting it would
make the page lie. Only the claims about what is CURRENT move."

Sweep the repo and the live site for ${PREV} and earlier, then classify EACH hit:
  - historical (describes a past release) -> correct, leave alone, say so
  - current-claim (says or implies this is what you get today) -> FAIL, report it

Do not propose bumping anything you classified as historical. Report the count of each class so
the ratio is visible; a sweep that finds everything stale is usually a misclassification.`,
  },
]

// ===========================================================================
const PHASES = {
  'pre-merge': { title: 'Pre-merge', checks: PRE_MERGE,
    gate: `Merge PR${PR ? ' #' + PR : ''} to main with --no-ff (NOT squash, NOT rebase) — the checklist requires a real merge commit for the release to name.` },
  'pre-tag': { title: 'Pre-tag', checks: PRE_TAG,
    gate: `Tag and push: git tag -a ${VERSION} -m "${VERSION} — <title>" && git push origin main && git push origin ${VERSION}. Then watch the Release workflow.` },
  'post-release': { title: 'Post-release', checks: POST_RELEASE,
    gate: `Close the issues this release resolves, naming the tag. If rendered output changed for existing consumers, say so where they will see it — locked claims do not re-review themselves.` },
}

const sel = PHASES[phase_]
if (!sel) {
  throw new Error(`unknown phase "${phase_}" — expected pre-merge, pre-tag or post-release`)
}

phase(sel.title)
log(`${sel.title}: ${sel.checks.length} checklist gates, read-only`)

const results = await run(sel.checks, sel.title)

const failures = results.filter(r => r.result === 'FAIL')
const couldNot = results.filter(r => r.result === 'COULD_NOT_RUN')
const blocking = results.flatMap(r => (r.problems || []).filter(p => p.severity === 'blocking'))

log(`${sel.title}: ${results.filter(r => r.result === 'PASS').length}/${results.length} PASS · ${failures.length} FAIL · ${couldNot.length} could not run · ${blocking.length} blocking`)

return {
  phase: phase_,
  version: VERSION,
  clear: failures.length === 0 && couldNot.length === 0 && blocking.length === 0,
  results,
  failures,
  couldNotRun: couldNot,
  blocking,
  // What the orchestrator does next, BY HAND, only if clear is true.
  nextIrreversibleStep: sel.gate,
}
