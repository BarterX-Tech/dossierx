#!/usr/bin/env bash
# run.sh — stage 2 of the release gate: the thirteen reading agents.
#
# WHAT THIS IS. Two things that have to live in the same place because they are
# the same promise:
#
#   1. THE HARNESS. It derives the fan-out from surfaces.yaml, reads the tool
#      grant from gate/method.yaml, and invokes the agent with EXACTLY that
#      grant, as an exclusive allow-list. That is what makes "an input outside
#      the assembled bundle cannot exist" a fact rather than a hope about what
#      the model chose to read — and every cache key in this gate rests on it.
#
#   2. THE PRODUCER of the run's release evidence: the resolved baseline
#      inventory, the delta over it, and the run manifest that says which tree
#      and which baseline these were produced from. gate/delta.json is a named
#      component of all thirteen keys, and until this script existed nothing in
#      the repository wrote it — so whatever happened to be at that path on the
#      day of the run, hand-written or left from a previous run, hashed cleanly
#      into every key.
#
# WHY IT IS NOT A GO PACKAGE. cmd/dossierx/surface_meta_test.go requires every Go
# package git carries to be either fingerprinted into surface.json or named in
# behaviourExclusions, whose only member is scripts/normalize-claims. A Go
# harness here would red that meta-test for the whole tree. Nothing in this
# repository lints shell, so a shell harness trips no other suite.
#
# WHAT IT CANNOT PROMISE. It can prove the runner REQUESTS the declared grant. It
# cannot prove the runtime honoured it: the entity that actually grants or
# withholds tools is the agent harness, which is outside this repository and
# outside every test here. That boundary is stated rather than papered over. See
# gate/method.yaml.
#
# WRITTEN FOR bash 3.2, which is what macOS ships. No mapfile, no associative
# arrays, no ${x^^}.
#
# EXIT CODES
#   0  did what was asked
#   1  usage error
#   2  an input could not be read
#   3  the baseline could not be RESOLVED — never reported as an empty delta
#   4  an artifact the run is supposed to have produced is missing
#   5  the fan-out could not be produced — no run was minted, so there is no
#      identifier any answer on disk can be attributed to
#   6  the SUBJECT MOVED mid-release — surfaces.yaml no longer matches the digest
#      this release froze, or it was re-opened with nothing on record saying why

set -eu

die() { printf 'gate-stage2: %s\n' "$1" >&2; exit "${2:-1}"; }

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
MANIFEST_FILE="surfaces.yaml"
METHOD_FILE="gate/method.yaml"
# The fan-out's ONE implementation, which `fanout` below invokes and never
# re-implements. It is named here so that pointing this script at a checkout that
# does not carry it is a refusal with a reason rather than a `go test` error
# about an unknown package.
PRODUCER_FILE="cmd/dossierx/gate_fanout_test.go"
PRODUCER_TEST="^TestGateFanoutProduce$"
# THE SUBJECT FREEZE. Tracked, unlike everything else this script writes under
# gate/ — a freeze that a run could regenerate is not a freeze, and a human has
# to be able to read what their release committed to. gate/.gitignore names it.
SUBJECT_FILE="gate/subject.json"
# The document the release version is derived from. It is one of the two sources
# the driver's D1 derives from (the other is the site's newest releases[] entry);
# a disagreement between them is D1's refusal to make, not this script's, and
# reading one here is enough to tell one RELEASE from the next.
VERSION_FILE="CHANGELOG.md"
# The snag check's question. It is a prompt file rather than a string in this
# script for the reason every other prompt is: what an agent is asked is reviewed
# material, and material nobody can diff is material nobody reviews.
WAVE_PROMPT_FILE="gate/prompts/_wave.md"

# ---------------------------------------------------------------------------
# sha256, from whichever of the two spellings this machine has. A missing hasher
# is a refusal, not a fallback: an artifact recorded without a digest is an
# artifact whose freshness nothing can check, and that is the "we did not check"
# reading as "it is fine" that this whole gate is written against.
# ---------------------------------------------------------------------------
sha256_of() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  elif command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    die "no shasum and no sha256sum on PATH; this run cannot record what it produced" 2
  fi
}

# ---------------------------------------------------------------------------
# resolved_object_name — the ONE spelling of "this baseline is an identity".
#
# Both modes below need it and they used to test it separately, which meant one
# of the two was never exercised: `record` accepted anything the tests happened
# not to pass it. It is a single function so that a row proving it for one mode
# is proving the same code the other mode runs.
#
# Forty hexadecimal digits, nothing else. A TAG NAME is a mutable pointer —
# `git tag -f` re-points an annotated tag under anything that names only the tag
# — and an ABBREVIATION is a prefix that can mean a different object in a
# different clone. Both are answers that stop being true later, and every
# carry-forward decision in the gate rests on which release the comparison was
# against.
# ---------------------------------------------------------------------------
# THE CHARACTER CLASS IS SPELLED OUT RATHER THAN WRITTEN AS A RANGE. `[0-9a-f]`
# is a COLLATION range, and outside the C locale most collations interleave the
# cases — so `[!0-9a-f]` does not match "B", and forty upper-case B's were
# accepted as a resolved object name. Git spells object names in lower case, so
# that value names no object and resolves to nothing. Enumerating the sixteen
# digits removes the locale from the answer entirely.
resolved_object_name() {
  case "$1" in
    *[!0123456789abcdef]* | "") return 1 ;;
  esac
  [ ${#1} -eq 40 ]
}

# ---------------------------------------------------------------------------
# provenance_bearing — which of the artifacts `record` names state, on their own
# face, which tree and which baseline they were produced from.
#
# Everything else this mode is handed is opaque bytes it can only digest. These
# two are documents THIS repository writes with a declared shape, so recording
# one that disagrees with the run is a disagreement that can be caught at the
# moment it is created rather than several minutes of agent time later.
#
# It is a function rather than a literal comparison inside the loop because the
# guard used to be `[ "$a" = "gate/delta.json" ] || continue`, and the second
# artifact that needed it — gate/render-diff.json, the cross-release render diff
# the CHANGELOG agent writes its silent-change entries from — was walked straight
# past. A third one added later is covered by adding a line here.
# ---------------------------------------------------------------------------
provenance_bearing() {
  case "$1" in
    gate/delta.json | gate/render-diff.json) return 0 ;;
  esac
  return 1
}

# ---------------------------------------------------------------------------
# THE FAN-OUT, read from surfaces.yaml and never written down here.
#
# A count in this file is a count that goes stale on the day a fourteenth surface
# is declared, and the run would then report green over thirteen while the
# manifest declares fourteen. Everything downstream — gateIsGreen, the receipt —
# already refuses a declared surface holding no verdict, PROVIDED the fan-out
# comes from the manifest. This is where that proviso is kept.
# ---------------------------------------------------------------------------
declared_surfaces() {
  awk '
    /^surfaces:[[:space:]]*$/   { in_surfaces = 1; next }
    /^[a-zA-Z_]+:[[:space:]]*$/ { in_surfaces = 0 }
    in_surfaces && /^  - name:/ {
      name = $0
      sub(/^  - name:[[:space:]]*/, "", name)
      print name
    }
  ' "$ROOT/$MANIFEST_FILE" | LC_ALL=C sort
}

# ---------------------------------------------------------------------------
# THE METHOD, read from gate/method.yaml and never written down here either.
# ---------------------------------------------------------------------------
method_model() {
  awk '/^model:[[:space:]]*/ { sub(/^model:[[:space:]]*/, ""); print; exit }' "$ROOT/$METHOD_FILE"
}

method_tools() {
  awk '
    /^tools:[[:space:]]*$/      { in_tools = 1; next }
    /^[a-zA-Z_]+:/              { in_tools = 0 }
    in_tools && /^  - /         { name = $0; sub(/^  - [[:space:]]*/, "", name); print name }
  ' "$ROOT/$METHOD_FILE"
}

# ---------------------------------------------------------------------------
# top_level_blocks emits "<key> <sha256-of-that-key's-block>" for a document
# generated by encoding/json's MarshalIndent with a two-space indent — which is
# what surface.json is. A top-level key is a line beginning with exactly two
# spaces and a quote; its block runs to the line before the next one.
#
# It is a comparison of BLOCKS rather than of whole files because the delta has
# to say WHICH parts of the inventory moved. A delta that says only "the
# inventory changed" tells thirteen agents nothing they could act on.
# ---------------------------------------------------------------------------
top_level_blocks() {
  awk '
    /^  "[^"]+":/ {
      if (key != "") print key "\t" body
      key = $0
      sub(/^  "/, "", key)
      sub(/":.*$/, "", key)
      body = $0
      next
    }
    # \001 rather than a newline: the caller reads these back as ONE
    # tab-separated record per key, and a body carrying real newlines would be
    # read as several records whose first field is a line of JSON. That is not a
    # cosmetic bug — it reported the two differing INNER lines as the names of
    # the top-level keys that moved.
    key != "" { body = body "\001" $0 }
    END { if (key != "") print key "\t" body }
  ' "$1"
}

changed_keys() {
  # $1 = this release's inventory, $2 = the baseline's
  _mine="$(mktemp)"; _theirs="$(mktemp)"
  top_level_blocks "$1" > "$_mine"
  top_level_blocks "$2" > "$_theirs"
  awk -F'\t' '
    NR == FNR { theirs[$1] = $2; seen[$1] = 1; next }
    { mine[$1] = $2; keys[$1] = 1 }
    END {
      for (k in keys)  if (!(k in theirs) || theirs[k] != mine[k]) print k
      for (k in seen)  if (!(k in mine)) print k
    }
  ' "$_theirs" "$_mine" | LC_ALL=C sort
  rm -f "$_mine" "$_theirs"
}

# json_scalar reads the first `"<key>": "<value>"` string out of a JSON document.
#
# Deliberately small: the only documents it reads are the ones this script wrote,
# and the only fields it reads are flat string scalars. It is not a JSON parser
# and must not become one — a value it cannot find comes back empty, and every
# caller treats empty as a refusal rather than as a match.
json_scalar() {
  awk -v key="$2" '
    {
      pattern = "\"" key "\"[[:space:]]*:[[:space:]]*\""
      if ($0 ~ pattern) {
        line = $0
        sub(".*" pattern, "", line)
        sub(/".*/, "", line)
        print line
        exit
      }
    }
  ' "$1"
}

# ---------------------------------------------------------------------------
# THE SUBJECT FREEZE — why a release may not grow its own question list.
#
# The v0.5.2 gate ran four reading rounds returning 39, 31, 24 and 18 findings,
# and surfaces.yaml gained paths during rounds 1, 2 AND 4. Round four's own fix
# wave records that the widening "is what let round four adjudicate the
# retired-set question at all" — real coverage, arriving mid-count. A curve
# measured over a subject that grows underneath it cannot converge, and worse, it
# cannot be READ: a round returning fewer findings might mean a better tree or a
# narrower question, and nothing on the record says which.
#
# So the manifest is frozen at the first fan-out of a release, and every later
# fan-out for that same release refuses if it moved.
#
# THIS IS NOT NARROWING, and the distinction is load-bearing against CLAUDE.md's
# rule that the gate never narrows coverage silently. Coverage stays exactly
# where round one set it — nothing is sampled, truncated or dropped. What is
# refused is GROWING it mid-release, and the refusal names the deferral rather
# than hiding it: a gap found in round three is recorded as a finding against the
# next release, where the next release's round one will read it.
#
# THE THAW. A maintainer who rules a gap blocking edits gate/subject.json: the
# new digest into surfaces_sha256, and their reason into thaw_reason. frozen_sha256
# is never edited, so the two fields disagreeing is the machine-readable fact that
# this release re-opened its subject — and a re-opening with an empty reason is
# refused.
#
# WHAT A THAW ACTUALLY COSTS. This comment has been wrong twice, in opposite
# directions, and the second time was itself a correction — which is why the
# components are enumerated here rather than summarised. A stage-2 key hashes
# FIVE things (gateSurfaceFingerprint): the surface's name, its own documents,
# the four shared evidence files, THE ASSEMBLED BUNDLE, and method_version.
#
# surfaces.yaml is owned by no surface — release-gate-artifacts claims it
# out_of_scope — and it is not one of the four shared files. The first version of
# this comment concluded from that that a thaw re-reads all thirteen; the second
# concluded that no key hashes the manifest at all. Both skipped the bundle.
# `contributing` declares surfaces.yaml in its `reads:`, so the manifest's bytes
# are assembled into that surface's bundle and its key moves whenever the
# manifest does.
#
# So a thaw moves `contributing`'s key always, plus the key of every surface
# whose resolved documents or whose own `reads:` list the edit changed. Every
# other key is byte-identical and carries forward. The maintainer thawing decides
# what else needs re-reading; what the freeze guarantees is that the widening is
# visible, dated and reasoned.
# ---------------------------------------------------------------------------

# release_version reads the newest version heading out of the CHANGELOG, which is
# what tells one release's freeze from the next one's. Keep-a-Changelog's own
# shape: `## [X.Y.Z] - YYYY-MM-DD`, newest first.
release_version() {
  # A missing CHANGELOG is a refusal with a reason rather than awk's own error on
  # stderr. The difference matters where this is called from: `fanout`'s other
  # refusals name exactly what the checkout is missing, and awk noise arriving
  # first buries them.
  [ -f "$ROOT/$VERSION_FILE" ] || die "subject: $ROOT carries no $VERSION_FILE, so no release version can be derived and this run cannot tell its own freeze from another release's" 2
  awk '
    /^## \[[0-9]+\.[0-9]+\.[0-9]+\]/ {
      line = $0
      sub(/^## \[/, "", line)
      sub(/\].*$/, "", line)
      print "v" line
      exit
    }
  ' "$ROOT/$VERSION_FILE"
}

# subject_verify refuses a run whose manifest has moved since this release froze
# it. Silence is the pass. It reads and never writes: a mode that repaired the
# freeze on its way past would be a freeze that a run can rewrite, which is no
# freeze at all.
subject_verify() {
  _version="$(release_version)"
  [ -n "$_version" ] || die "subject: $VERSION_FILE carries no \`## [X.Y.Z]\` heading, so this run cannot say WHICH release it belongs to and cannot tell its own freeze from another release's. A gate that cannot name its release is not a smaller gate." 2

  # No freeze yet — the first fan-out of this release is what mints it, and that
  # happens after the producer has minted a run to name.
  [ -f "$ROOT/$SUBJECT_FILE" ] || return 0

  _recorded_version="$(json_scalar "$ROOT/$SUBJECT_FILE" version)"
  [ -n "$_recorded_version" ] || die "subject: $SUBJECT_FILE names no version, so nothing can say which release froze it. Delete it and let the next fan-out mint a fresh freeze, or repair the field by hand." 2

  # A different release: this file belongs to the previous one and the freeze is
  # re-minted below, not compared against. A release inheriting its predecessor's
  # subject would be exactly the stale-evidence failure the rest of this gate
  # refuses everywhere else.
  [ "$_recorded_version" = "$_version" ] || return 0

  _frozen="$(json_scalar "$ROOT/$SUBJECT_FILE" frozen_sha256)"
  _current="$(json_scalar "$ROOT/$SUBJECT_FILE" surfaces_sha256)"
  _reason="$(json_scalar "$ROOT/$SUBJECT_FILE" thaw_reason)"
  [ -n "$_frozen" ] && [ -n "$_current" ] || die "subject: $SUBJECT_FILE is missing frozen_sha256 or surfaces_sha256, so it records no subject and cannot be compared against one" 2

  if [ "$_current" != "$_frozen" ] && [ -z "$_reason" ]; then
    die "subject: $SUBJECT_FILE records a THAW with no reason — surfaces_sha256 ($_current) differs from frozen_sha256 ($_frozen) and thaw_reason is empty.
  A release that re-opened its subject and did not say why leaves the next reader unable to tell a deliberate widening from a hand-edit. Fill thaw_reason, or restore surfaces_sha256 to the frozen digest and revert $MANIFEST_FILE." 6
  fi

  _actual="$(sha256_of "$ROOT/$MANIFEST_FILE")"
  if [ "$_actual" != "$_current" ]; then
    die "subject: $MANIFEST_FILE moved during release $_version.
  frozen at:  $_frozen
  accepted:   $_current
  on disk:    $_actual
  The manifest is the QUESTION this release's rounds are counted over, and a question that grows between rounds makes the finding curve unreadable — a smaller round may mean a better tree or a narrower ask, and nothing on the record says which.
  Two ways forward, both deliberate:
    revert $MANIFEST_FILE and record the coverage gap as a finding against the NEXT release, which is where its round one will read it; or
    rule it blocking, set surfaces_sha256 to $_actual and write thaw_reason in $SUBJECT_FILE. That re-reads contributing, which declares this manifest in its reads: and so carries it inside its bundle, plus every surface whose own documents or reads: list your edit changed. The rest carry forward." 6
  fi
}

# subject_freeze mints the freeze for this release, or leaves an existing one
# alone. It is called AFTER the producer has minted a run, so the record can name
# the run that first asked this release's question.
subject_freeze() {
  _run="$1"
  [ -n "$_run" ] || die "subject: --run is required to freeze; a freeze that names no run cannot say which fan-out first asked this release's question" 1

  _version="$(release_version)"
  [ -n "$_version" ] || die "subject: $VERSION_FILE carries no \`## [X.Y.Z]\` heading, so a freeze would name no release" 2

  if [ -f "$ROOT/$SUBJECT_FILE" ]; then
    _recorded_version="$(json_scalar "$ROOT/$SUBJECT_FILE" version)"
    # Same release: subject_verify has already agreed the manifest has not moved,
    # so there is nothing to write. Rewriting here would reset a thaw's reason.
    [ "$_recorded_version" = "$_version" ] && return 0
  fi

  _actual="$(sha256_of "$ROOT/$MANIFEST_FILE")"
  mkdir -p "$ROOT/gate"
  {
    printf '{\n'
    printf '  "version": "%s",\n' "$_version"
    printf '  "frozen_sha256": "%s",\n' "$_actual"
    printf '  "surfaces_sha256": "%s",\n' "$_actual"
    printf '  "frozen_at_run": "%s",\n' "$_run"
    printf '  "thaw_reason": ""\n'
    printf '}\n'
  } > "$ROOT/$SUBJECT_FILE"
  printf 'gate-stage2: froze the subject for %s at %s (run %s)\n' "$_version" "$_actual" "$_run" >&2
}

json_string_array() {
  # reads lines on stdin, writes a JSON array of strings
  awk '
    BEGIN { printf "[" ; first = 1 }
    {
      gsub(/\\/, "\\\\"); gsub(/"/, "\\\"")
      if (!first) printf ", "
      printf "\"%s\"", $0
      first = 0
    }
    END { printf "]" }
  '
}

# ---------------------------------------------------------------------------
# the modes
# ---------------------------------------------------------------------------

usage() {
  cat >&2 <<'USAGE'
usage: run.sh <mode> [options]

  surfaces                       the fan-out, read from surfaces.yaml
  grant                          the exact tool grant, read from gate/method.yaml
  model                          the model id, read from gate/method.yaml
  command --surface S --bundle P the invocation the harness would exec
  fanout  --tree T               mint this run, write gate/bundles/<surface>.md
                                 for every declared surface and gate/fanout.json,
                                 then print one invocation per surface
  delta   --tree T --baseline-ref R --baseline-commit C --baseline-file F
                                 resolve the baseline, write gate/baseline.json
                                 and gate/delta.json
  record  --tree T --baseline-ref R --baseline-commit C <artifact>...
                                 write gate/run.json over exactly these artifacts
  subject [--freeze --run ID]    verify that surfaces.yaml still matches the
                                 digest this release froze; --freeze mints that
                                 record. `fanout` runs both and never skips them
  wave    --range A..B           assemble gate/wave/bundle.md over a fix wave's
                                 own diff and print two invocations. ADVISORY:
                                 it answers about a RANGE, never about a surface

  --root DIR  operate on another checkout (default: this script's repository)
USAGE
  exit 1
}

[ $# -ge 1 ] || usage
MODE="$1"; shift

TREE=""; BASELINE_REF=""; BASELINE_COMMIT=""; BASELINE_FILE=""; SURFACE=""; BUNDLE=""
RUN_ID=""; FREEZE=""; RANGE=""
ARTIFACTS=""
while [ $# -gt 0 ]; do
  case "$1" in
    --root)             ROOT="$(cd "$2" && pwd)"; shift 2 ;;
    --run)              RUN_ID="$2"; shift 2 ;;
    --freeze)           FREEZE="yes"; shift ;;
    --range)            RANGE="$2"; shift 2 ;;
    --tree)             TREE="$2"; shift 2 ;;
    --baseline-ref)     BASELINE_REF="$2"; shift 2 ;;
    --baseline-commit)  BASELINE_COMMIT="$2"; shift 2 ;;
    --baseline-file)    BASELINE_FILE="$2"; shift 2 ;;
    --surface)          SURFACE="$2"; shift 2 ;;
    --bundle)           BUNDLE="$2"; shift 2 ;;
    -*)                 die "unknown option: $1" 1 ;;
    *)                  ARTIFACTS="$ARTIFACTS $1"; shift ;;
  esac
done

[ -f "$ROOT/$MANIFEST_FILE" ] || die "no $MANIFEST_FILE under $ROOT" 2

case "$MODE" in

  surfaces)
    out="$(declared_surfaces)"
    [ -n "$out" ] || die "$MANIFEST_FILE declares no surfaces; the fan-out would be empty" 2
    printf '%s\n' "$out"
    ;;

  grant)
    [ -f "$ROOT/$METHOD_FILE" ] || die "no $METHOD_FILE under $ROOT; the tool grant is undeclared" 2
    out="$(method_tools)"
    # An empty grant is refused rather than passed on. An agent that can call
    # nothing cannot report a verdict, and a run that asks thirteen agents a
    # question none of them can answer produces no findings — which reads
    # exactly like thirteen clean passes.
    [ -n "$out" ] || die "$METHOD_FILE declares no tools; an agent that can call nothing cannot even report FAILED" 2
    printf '%s\n' "$out"
    ;;

  model)
    [ -f "$ROOT/$METHOD_FILE" ] || die "no $METHOD_FILE under $ROOT" 2
    out="$(method_model)"
    [ -n "$out" ] || die "$METHOD_FILE declares no model" 2
    printf '%s\n' "$out"
    ;;

  command)
    [ -n "$SURFACE" ] || die "command: --surface is required" 1
    [ -n "$BUNDLE" ]  || die "command: --bundle is required" 1
    declared_surfaces | grep -qx "$SURFACE" \
      || die "command: $MANIFEST_FILE declares no surface named $SURFACE" 1
    model="$("$0" model --root "$ROOT")"
    tools="$("$0" grant --root "$ROOT" | paste -sd, -)"
    # THE GRANT IS PASSED AS AN EXCLUSIVE ALLOW-LIST. There is deliberately no
    # --disallowed-tools here: a deny list of named bad tools is walked past by
    # the next name anybody invents, and the screen stays green while doing it.
    printf '%s --model %s --allowed-tools %s --bundle-file %s\n' \
      "${DOSSIERX_GATE_AGENT:-<DOSSIERX_GATE_AGENT is unset>}" "$model" "$tools" "$BUNDLE"
    ;;

  # -------------------------------------------------------------------------
  # -------------------------------------------------------------------------
  # wave — THE SNAG CHECK. Two agents read a fix wave's own diff before a full
  # round pays thirteen agents to discover what that wave broke.
  #
  # WHY. Every round of the v0.5.2 gate since the second opened by repairing the
  # round before it. Round two: "Four were regressions introduced by round one's
  # fixes". Round three carries a section titled "MINE, FROM ROUND TWO". Round
  # four: "three of these are high severity and all three are mine". The fix wave
  # is written by an agent and nothing reads it until the next full round does.
  #
  # THE PROPERTY THE WHOLE THING RESTS ON: A WAVE ANSWER IS NEVER A SURFACE
  # ANSWER. This mode writes its bundle under gate/wave/, never under
  # gate/answers/, and mints nothing. Its answer is keyed to a RANGE; a surface
  # answer is keyed to a tree, a run identifier and a surface fingerprint, none of
  # which exist here. So a wave answer copied into gate/answers/ is refused by
  # stage 3 on its face rather than by anyone remembering the difference. A clean
  # wave read means "no regression found in this diff" and NEVER "this surface
  # passes" — anything looser would let a narrow read stand where a full bundle
  # read is required, which is the skipped check that reads as a pass.
  #
  # WHAT IT DELIBERATELY DOES NOT DO: map changed paths onto declared surfaces.
  # surfaces.yaml's path matching has exactly one implementation, in the Go
  # producer, and a second one written in awk here would be free to disagree with
  # it — a bundle assembled by the wrong matcher is a question nobody asked. The
  # prompt says plainly that the reader is judging a change, not a surface.
  # -------------------------------------------------------------------------
  wave)
    [ -n "$RANGE" ] || die "wave: --range is required, as A..B. This mode reads a fix wave's own diff, and a wave with no range is not a smaller reading — there is nothing to read." 1
    [ -f "$ROOT/$WAVE_PROMPT_FILE" ] || die "wave: $ROOT carries no $WAVE_PROMPT_FILE, so there is no question to ask. A bundle assembled with no prompt is material nobody was asked about." 2
    command -v git >/dev/null 2>&1 || die "wave: no git on PATH, so the wave's diff cannot be read. A check that cannot run is a failure, not a pass." 2

    _changed="$(git -C "$ROOT" diff --name-only "$RANGE" --)" \
      || die "wave: git could not read the range $RANGE (its reason is above). An unreadable range is a failed read, never an empty one." 2
    [ -n "$_changed" ] || die "wave: $RANGE names no changed files. A wave that changed nothing needs no reading, and a range that resolves to nothing because it was mistyped must not read as one." 1

    mkdir -p "$ROOT/gate/wave"
    _bundle="gate/wave/bundle.md"
    {
      sed "s|<<RANGE>>|$RANGE|g" "$ROOT/$WAVE_PROMPT_FILE"
      printf '\n## The files this wave changed\n\n'
      printf '%s\n' "$_changed" | sed 's/^/- /'
      # SEVEN TILDES, not three backticks. A fence is closed only by a fence of
      # the SAME character, so a ~~~~~~~ wrapper survives every ``` block inside
      # the files it wraps — and every markdown file in this repository has them.
      # The protection is the character, not the length. Nothing goes missing
      # either way: this is about the frame the reader is handed, not the bytes.
      printf '\n## The diff\n\n~~~~~~~diff\n'
      git -C "$ROOT" diff "$RANGE" --
      printf '~~~~~~~\n'
      printf '\n## Each changed file, in full, as it stands in this checkout\n\n'
      printf '%s\n' "$_changed" | while IFS= read -r _file; do
        [ -n "$_file" ] || continue
        printf '### %s\n\n' "$_file"
        if [ -f "$ROOT/$_file" ]; then
          printf '~~~~~~~\n'
          cat "$ROOT/$_file"
          printf '~~~~~~~\n\n'
        else
          # A deleted file has no "after", and saying so is not the same as
          # handing over an empty block — one is a fact about the wave, the other
          # reads as a file that is now empty.
          printf '_Deleted by this wave; there is no text after it._\n\n'
        fi
      done
    } > "$ROOT/$_bundle"

    # TWO READERS, NOT ONE. A single reader's silence is indistinguishable from a
    # reader that lost the thread; two independent ones make a shared silence
    # mean something. They are not thirteen, because this is a diff and not a
    # release.
    model="$("$0" model --root "$ROOT")"
    tools="$("$0" grant --root "$ROOT" | paste -sd, -)"
    _n=1
    while [ "$_n" -le 2 ]; do
      printf '%s --model %s --allowed-tools %s --bundle-file %s\n' \
        "${DOSSIERX_GATE_AGENT:-<DOSSIERX_GATE_AGENT is unset>}" "$model" "$tools" "$_bundle"
      _n=$((_n + 1))
    done
    printf 'gate-stage2: wrote %s over %s (%s changed file(s)); this reading is ADVISORY and files no answer\n' \
      "$_bundle" "$RANGE" "$(printf '%s\n' "$_changed" | wc -l | tr -d ' ')" >&2
    ;;

  subject)
    if [ -n "$FREEZE" ]; then
      subject_freeze "$RUN_ID"
    else
      subject_verify
    fi
    ;;

  # fanout — mint this run, write every surface's bundle, then say what to exec.
  #
  # A THIN WRAPPER OVER THE GO PRODUCER, and thin is the whole design. Every rule
  # a fan-out obeys already lives in cmd/dossierx: the fan-out is surfaces.yaml's
  # (gateDeclaredSurfaces), a surface's documents are `git ls-files` resolved
  # against the manifest's patterns, what is handed over and what is only named is
  # gateStage2BundleSpec's answer, and the exact bytes are gateBundleAssemble's —
  # the same bytes every surface key is a digest of. An awk re-implementation of
  # any of that here would be a second answer to "what is this agent being asked",
  # and the two would diverge in silence, because only the Go one is under test.
  #
  # SO THIS MODE DOES EXACTLY THREE THINGS: it refuses what it can refuse before
  # spending a toolchain invocation, it runs the producer and dies with the
  # producer's own message, and it prints one line per DECLARED surface by calling
  # `command` — never by re-deriving the model, the grant or the fan-out.
  # -------------------------------------------------------------------------
  fanout)
    [ -n "$TREE" ] || die "fanout: --tree is required. A fan-out mints an identifier that every one of its answers must name, and an identifier minted over no tree attaches those answers to no release at all." 1


    # THE PRODUCER IS THE CHECKOUT'S OWN. It assembles the bundles from the files
    # under --root and its assembler must be that tree's assembler, because the
    # bundle digest IS the surface key: bytes assembled by another checkout's code
    # are a question no key in the tree being released was ever taken over.
    [ -f "$ROOT/$PRODUCER_FILE" ] || die "fanout: $ROOT carries no $PRODUCER_FILE, so the checkout being fanned out holds no producer. There is exactly one implementation of a fan-out and it is that file; this mode will not stand in for it." 2

    # A missing toolchain is a refusal, not a fallback, for sha256_of's reason one
    # screen up: there is no smaller fan-out to fall back to, and a mode that
    # printed thirteen invocations without minting a run would hand the harness
    # thirteen paths holding nothing and an answer set nobody can attribute.
    command -v go >/dev/null 2>&1 || die "fanout: no go toolchain on PATH, so the fan-out cannot be produced. A check that cannot run is a failure, not a pass." 2

    # THE SUBJECT IS VERIFIED BEFORE ANYTHING IS MINTED — after the refusals that
    # name a checkout this mode cannot run in at all, and before the producer that
    # mints the run. So a manifest that moved mid-release costs one digest rather
    # than thirteen agents, and no run identifier can exist beside the refusal,
    # which on disk is indistinguishable from a fan-out that half happened.
    # subject_verify writes nothing and exits 6, which propagates through `set -e`.
    subject_verify

    # The producer's own output — the refusal, or the run it minted — goes to
    # stderr, so stdout carries the invocations and nothing else.
    ( cd "$ROOT" && go test ./cmd/dossierx -run "$PRODUCER_TEST" -count=1 -v -fanout-out -fanout-tree="$TREE" ) >&2 \
      || die "fanout: the producer refused this run (its reason is above). No run was minted and gate/fanout.json was not written, so nothing downstream can attribute an answer to this release; fix what it named and run \`fanout\` again." 5

    # The freeze is minted AFTER the run exists, so the record can name the
    # fan-out that first asked this release's question. On every later round of
    # the same release this is a no-op — subject_verify above has already agreed
    # the manifest has not moved, and rewriting here would erase a thaw's reason.
    subject_freeze "$(json_scalar "$ROOT/gate/fanout.json" run)"

    # ONE LINE PER SURFACE THE MANIFEST DECLARES, and the fan-out is read from the
    # manifest here for the same reason it is read from the manifest inside the
    # producer: a count written down in this file is a count that goes stale on the
    # day a fourteenth surface is declared, and the run would then exec thirteen
    # agents and report on fourteen surfaces' worth of nothing.
    for _surface in $(declared_surfaces); do
      "$0" command --root "$ROOT" --surface "$_surface" --bundle "gate/bundles/$_surface.md"
    done
    ;;

  delta)
    [ -n "$TREE" ] || die "delta: --tree is required; a delta that does not say which tree it covers cannot be checked for freshness" 1
    # THE BASELINE IS RESOLVED OR THE RUN FAILS. There is no branch here that
    # turns "I could not find the previous release" into an empty delta. An
    # empty delta is a legitimate and expected answer — this project's first
    # gated release changes no shipped code — and it is a completely different
    # statement from "there was nothing to compare against".
    resolved_object_name "$BASELINE_COMMIT" \
      || die "delta: the baseline could not be resolved — --baseline-commit is ${BASELINE_COMMIT:-empty} and must be a full 40-digit object name. A tag is a mutable pointer, an abbreviation is a prefix, and forty characters of something else is neither. An unresolvable baseline is a FAILED run; it is never an empty delta." 3
    [ -n "$BASELINE_REF" ] || die "delta: --baseline-ref is required (the human-readable name of what --baseline-commit resolved from)" 1
    [ -n "$BASELINE_FILE" ] || die "delta: --baseline-file is required" 1
    [ -f "$BASELINE_FILE" ] || die "delta: the baseline inventory $BASELINE_FILE cannot be read; the baseline could not be resolved" 3
    [ -f "$ROOT/surface.json" ] || die "delta: no surface.json under $ROOT to diff against the baseline" 2

    mkdir -p "$ROOT/gate"
    cp "$BASELINE_FILE" "$ROOT/gate/baseline.json"
    changed="$(changed_keys "$ROOT/surface.json" "$ROOT/gate/baseline.json" | json_string_array)"
    # THE DELTA RECORDS THE DIGEST OF THE BYTES IT READ. gate/baseline.json is
    # what every key hashes; this file is a summary of a comparison against it.
    # Without the digest the two are only assumed to be about each other, and a
    # re-resolved baseline with an un-recomputed delta leaves thirteen keys
    # carrying an inventory the comparison never saw.
    baseline_sha="$(sha256_of "$ROOT/gate/baseline.json")"
    {
      printf '{\n'
      printf '  "tree": "%s",\n' "$TREE"
      printf '  "baseline": {"ref": "%s", "commit": "%s", "sha256": "%s"},\n' "$BASELINE_REF" "$BASELINE_COMMIT" "$baseline_sha"
      printf '  "changed": %s\n' "$changed"
      printf '}\n'
    } > "$ROOT/gate/delta.json"
    printf 'gate-stage2: wrote gate/baseline.json and gate/delta.json against %s (%s)\n' "$BASELINE_REF" "$BASELINE_COMMIT" >&2
    ;;

  record)
    [ -n "$TREE" ] || die "record: --tree is required" 1
    resolved_object_name "$BASELINE_COMMIT" \
      || die "record: the baseline could not be resolved — --baseline-commit is ${BASELINE_COMMIT:-empty} and must be a full 40-digit object name. A run manifest that cannot name the release it was compared against covers nothing." 3
    [ -n "$BASELINE_REF" ] || die "record: --baseline-ref is required" 1
    [ -n "$ARTIFACTS" ] || die "record: name the artifacts this run produced; a manifest over zero artifacts asserts nothing" 1

    # RECORDING A PROVENANCED ARTIFACT MEANS CLAIMING IT. Every other artifact
    # this mode names is opaque bytes it can only digest, but these state which
    # tree and which baseline they were computed from — so recording one that
    # disagrees with this run is refused HERE, at the point the disagreement is
    # created.
    #
    # The sequence is ordinary and it is why this exists: a gate FAILS, a fix
    # lands, the tree moves, and the driver re-runs the captures and `record`
    # but not `delta` — or re-runs `delta` and not the captures. Re-digesting
    # whatever is on disk would launder the stale one into a manifest that is
    # honest about every byte it names.
    #
    # THESE DOCUMENTS ARE READ WITH json_scalar, which takes the FIRST match on
    # any line and exits. That is correct only because both of them put "tree"
    # and "baseline"."commit" before everything else, so no later key and no
    # diff hunk can be read in their place. The ordering is a promise the
    # producers make to this reader, and it is pinned on their side —
    # tests/render_diff_capture_test.go, TestRenderDiffCaptureProvenanceComesFirst.
    for a in $ARTIFACTS; do
      provenance_bearing "$a" || continue
      # NOT `|| continue`. This loop IS the guard, and stepping over a
      # provenance-bearing artifact because it is absent would be the guard
      # declining to run on a state it exists for. Same refusal, same exit code
      # as the digest loop below.
      [ -f "$ROOT/$a" ] || die "record: $a was named as produced by this run and is not there" 4
      _dtree="$(json_scalar "$ROOT/$a" tree)"
      _dcommit="$(json_scalar "$ROOT/$a" commit)"
      # THE IDENTITY RULE APPLIED TO WHAT THE FILE SAYS, not just to what the
      # caller said. An artifact that names NEITHER a tree nor a baseline —
      # `printf '{}' > gate/render-diff.json`, the one-line workaround for a
      # gate that has been refusing for ten minutes — is refused here rather
      # than stepped over: downstream it is indistinguishable from a comparison
      # that ran and found nothing, because the manifest is honest about its
      # bytes and its digest matches.
      resolved_object_name "$_dtree" \
        || die "record: $a records tree ${_dtree:-nothing}, which is not a full 40-digit object name. An artifact that cannot say which tree it covers cannot be checked against this run at all, and a file that says nothing hashes into every key exactly as cleanly as one that says the truth." 3
      resolved_object_name "$_dcommit" \
        || die "record: $a records baseline commit ${_dcommit:-nothing}, which is not a full 40-digit object name. A tag is a mutable pointer and an abbreviation is a prefix; either can mean a different release tomorrow than it meant when this run recorded it." 3
      [ "$_dtree" = "$TREE" ] \
        || die "record: $a was computed over tree $_dtree and this run covers $TREE. Re-produce it for this tree — \`delta\` for gate/delta.json, the -render-diff-out capture entry point for gate/render-diff.json; recording it as produced here would hand every surface agent a comparison against a different release." 3
      [ "$_dcommit" = "$BASELINE_COMMIT" ] \
        || die "record: $a compared against baseline $_dcommit and this run resolved $BASELINE_COMMIT" 3
    done

    # Every artifact is checked and digested BEFORE anything is written. A
    # refusal partway through a redirect leaves a truncated run.json behind, and
    # a truncated manifest is worse than none: it parses far enough to look like
    # a run that recorded less than it produced.
    ENTRIES=""
    for a in $ARTIFACTS; do
      # A named artifact that is not there is a refusal, never an omission from
      # the list. Omitting it would produce a manifest that is internally
      # consistent and covers less than the run was asked to cover.
      [ -f "$ROOT/$a" ] || die "record: $a was named as produced by this run and is not there" 4
      _sep=""
      [ -z "$ENTRIES" ] || _sep=",
"
      ENTRIES="$ENTRIES$_sep    {\"path\": \"$a\", \"sha256\": \"$(sha256_of "$ROOT/$a")\"}"
    done

    mkdir -p "$ROOT/gate"
    {
      printf '{\n'
      printf '  "tree": "%s",\n' "$TREE"
      printf '  "baseline": {"ref": "%s", "commit": "%s"},\n' "$BASELINE_REF" "$BASELINE_COMMIT"
      printf '  "artifacts": [\n'
      printf '%s\n' "$ENTRIES"
      printf '  ]\n'
      printf '}\n'
    } > "$ROOT/gate/run.json"
    printf 'gate-stage2: wrote gate/run.json over%s\n' "$ARTIFACTS" >&2
    ;;

  *)
    usage
    ;;
esac
