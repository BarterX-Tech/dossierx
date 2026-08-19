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
# resolve_baseline_inventory — the baseline's BYTES, derived from its identity.
#
# This is the harness's copy of the ONE rule for "which document is this
# release's baseline", and the rule is gateBaselineFor's in
# cmd/dossierx/gate_baseline_test.go: the frozen bootstrap is chosen because the
# baseline commit IS v0.5.0's commit — chosen by identity, never because some
# other way of getting a baseline returned an error — and every other commit's
# inventory is read out of that commit's own tree. TestGateStage2DeltaProducer-
# ResolvesOrFails pins the two constants below to the Go side's, so the copies
# cannot drift apart in silence.
#
# WHY `delta` NO LONGER TAKES A FILE. It used to take --baseline-file, and the
# flag was the defect: docs/RELEASING.md's staging block hard-coded it to the
# committed v0.5.0 bootstrap, which was the right file for exactly one release
# and then silently became two releases old — the v0.6.0 gate run computed its
# delta against v0.5.0's frozen inventory while recording baseline ref v0.5.1,
# and thirteen reading agents were handed a comparison spanning two releases as
# the truth about what that release changed. A file argument is a second answer
# to a question git already holds the answer to, and a second answer can drift;
# deriving the bytes from the commit makes the mismatch impossible rather than
# merely discouraged. The retired flag is refused by name in `delta` below, so
# an invocation copied from an older revision of docs/RELEASING.md is a loud
# refusal, never a silent two-release comparison.
#
# THE FAILURE ARM SUBSTITUTES NOTHING. In a shallow clone `git show
# <commit>:surface.json` fails character for character the way it fails for a
# release that predates the emitter, so an arm that fell back to the bootstrap
# on that failure would reproduce the exact defect this function removes —
# full, plausible, and wrong about which past it describes. An unreadable
# baseline is exit 3, the same "never an empty delta" refusal the identity
# checks use.
# ---------------------------------------------------------------------------
# The frozen release's identity and its only inventory. v0.5.0 shipped before
# the surface emitter existed, so its tree carries no surface.json and the
# committed bootstrap is the ONLY record of what its surface was. The commit
# sha, not the tag name, is the trigger: a tag is a pointer `git tag -f`
# re-points, and gateBaselineCommit in cmd/dossierx/gate_baseline_test.go pins
# the same forty digits for the same reason.
BOOTSTRAP_COMMIT="3217a48b4a123ea4b8b02f93fac6337b985eb7ce"
BOOTSTRAP_FILE="surface.baseline.json"

resolve_baseline_inventory() {
  # $1 = the baseline's resolved commit (already a full object name),
  # $2 = the path to write the inventory's bytes to
  if [ "$1" = "$BOOTSTRAP_COMMIT" ]; then
    [ -f "$ROOT/$BOOTSTRAP_FILE" ] || die "the baseline is the frozen release $BOOTSTRAP_COMMIT, whose only inventory is the committed $BOOTSTRAP_FILE — and it is not under $ROOT. That release carries no surface.json of its own, so there is no second way to get its inventory; a run in this state has no past to diff against, which is a FAILED run and never an empty delta." 3
    cp "$ROOT/$BOOTSTRAP_FILE" "$2"
    return 0
  fi
  command -v git >/dev/null 2>&1 || die "no git on PATH, so the baseline inventory cannot be read out of commit $1. A check that cannot run is a failure, not a pass." 2
  if ! git -C "$ROOT" show "$1:surface.json" > "$2"; then
    rm -f "$2"
    die "the baseline inventory could not be read out of commit $1 (git show $1:surface.json under $ROOT; git's own reason is above). The committed $BOOTSTRAP_FILE is NOT a fallback for this: in a shallow or unfetched clone this failure reads character for character like a release that predates the inventory, and substituting the bootstrap would hand thirteen agents a delta spanning two releases as the truth about the past. Fetch the history — git fetch --tags --force --unshallow — and run this again." 3
  fi
}

# require_recorded_baseline_is_resolved is `record`'s use of the same rule:
# derive the named commit's inventory AGAIN and refuse gate/baseline.json when
# its bytes are anything else. Both guarded branches call it — gate/baseline.json
# because those bytes are hashed into all thirteen keys, and gate/delta.json
# because the recomputation there runs OVER gate/baseline.json, so a stale
# baseline with a delta honestly computed over it agrees with itself and the
# recomputation alone waves the pair through.
require_recorded_baseline_is_resolved() {
  [ -f "$ROOT/gate/baseline.json" ] \
    || die "record: gate/baseline.json is not there, so nothing can say whether the baseline this run compared against is the named release's inventory. A check that cannot run is a failure, not a pass." 4
  _rr="$(mktemp)"
  resolve_baseline_inventory "$BASELINE_COMMIT" "$_rr"
  if ! cmp -s "$_rr" "$ROOT/gate/baseline.json"; then
    rm -f "$_rr"
    die "record: gate/baseline.json is not the inventory that baseline $BASELINE_REF ($BASELINE_COMMIT) carries. Those bytes are hashed into every surface's key and handed to every agent as the past this release is being compared with — recorded as they are, a delta over them describes some other release's distance from this one (a two-release-old bootstrap under the previous release's ref is exactly how this guard came to exist). Re-run \`delta\` for this checkout and \`record\` again." 3
  fi
  rm -f "$_rr"
}

# ---------------------------------------------------------------------------
# provenance_bearing — which of the artifacts `record` names it can hold to
# account at the moment they are recorded, rather than merely digest.
#
# Everything else this mode is handed is opaque bytes. These four are documents
# THIS repository produces with a declared shape, so recording one that
# disagrees with the run is a disagreement that can be caught at the moment it
# is created rather than several minutes of agent time later. They are held to
# account in three different ways, and the guard loop in `record` says which is
# which:
#
#   gate/baseline.json    is RE-RESOLVED. It is the named baseline commit's own
#                         inventory (resolve_baseline_inventory), so the guard
#                         derives the same bytes again from the commit and
#                         byte-compares. This is the refusal for the state the
#                         delta recomputation alone cannot see: a stale baseline
#                         and a delta computed honestly OVER it agree with each
#                         other perfectly — that pair recorded cleanly once,
#                         under a ref two releases newer than its bytes.
#   gate/delta.json       is RECOMPUTED. It is a pure function of surface.json
#                         and gate/baseline.json, so the guard re-derives the
#                         whole document and byte-compares — after gate/
#                         baseline.json itself has been re-resolved, since a
#                         recomputation over stale bytes agrees with the stale
#                         answer. It deliberately carries no tree stamp — see
#                         emit_delta_document for why a stamp there re-keys
#                         every surface on every commit.
#   gate/render-diff.json states the tree and the baseline commit it compared,
#                         and cannot be recomputed here (it needs the rendered
#                         output of two releases), so its stamps are read and
#                         checked against the run's own.
#   gate/site-text.json   states the tree whose build it was extracted from,
#                         and cannot be recomputed here either (it needs a real
#                         build and a real browser). It compares this tree's
#                         site against nothing, so it carries a tree stamp and
#                         no baseline commit.
#
# It is a function rather than a literal comparison inside the loop because the
# guard used to be `[ "$a" = "gate/delta.json" ] || continue`, and the second
# artifact that needed it — gate/render-diff.json, the cross-release render diff
# the CHANGELOG agent writes its silent-change entries from — was walked straight
# past. The third, gate/site-text.json, was walked past for longer still: it
# carried a node/npm toolchain stamp and no tree at all, so an extraction left
# over from the previous release recorded cleanly and was hashed into the site
# surface's key as this release's evidence. The fourth, gate/baseline.json, was
# walked past longest of all — `record` digested whatever bytes sat there, so a
# baseline two releases old, with a delta honestly computed over it, recorded
# under the previous release's ref with every digest true. A fifth one added
# later is covered by adding a line here and a branch in `record`'s guard loop.
# ---------------------------------------------------------------------------
provenance_bearing() {
  case "$1" in
    gate/baseline.json | gate/delta.json | gate/render-diff.json | gate/site-text.json) return 0 ;;
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

# ---------------------------------------------------------------------------
# emit_delta_document — THE ONE SPELLING of gate/delta.json's bytes.
#
# The delta is a PURE FUNCTION of four values: this tree's inventory, the
# resolved baseline inventory, and the ref/commit pair the baseline was resolved
# from. `delta` redirects this function into gate/delta.json; `record` runs the
# SAME function over what is on disk at record time and refuses on any byte of
# disagreement. One implementation is what makes that comparison a freshness
# proof rather than a formatting accident: two printf sequences would drift
# apart in whitespace, and every disagreement would then read as a stale delta.
#
# NOTE WHAT IS DELIBERATELY NOT IN IT: THE TREE. This file's bytes are hashed
# into all thirteen surface keys and assembled verbatim into all thirteen
# bundles, so any per-commit value written here re-keys every surface on every
# commit. The landed version stamped $TREE into this document, and the gate's
# carry-forward machinery never once fired because of it: a one-character README
# fix moved the tree, which moved these bytes, which moved all thirteen keys and
# re-ran all thirteen reading agents. The freshness that stamp used to buy —
# refusing a delta computed over a different tree — is bought by recomputation
# instead, which is strictly stronger: a stamp says who claims to have written
# the file, a recomputation checks what it says. And when recomputation agrees,
# carrying the file is CORRECT even after the tree moves, because a byte of it
# is not a claim about the tree's name — a delta over two unchanged inventories
# is exactly the delta the new tree would produce.
#
# The baseline ref, commit and sha256 stay in the file because they move only
# when the PREVIOUS RELEASE moves — which is precisely when every surface should
# re-run. What this function alone cannot promise: it ties the delta to whatever
# bytes sit at gate/baseline.json; whether THOSE bytes are really the named
# commit's inventory is resolve_baseline_inventory's answer, which `delta` uses
# to produce the file and `record` re-performs against the file before naming
# it — so a baseline that is not the named commit's own inventory is refused at
# both points it could enter a run.
# ---------------------------------------------------------------------------
emit_delta_document() {
  # $1 = this tree's inventory, $2 = the resolved baseline inventory,
  # $3 = the baseline's human-readable ref, $4 = the baseline's resolved commit
  _changed="$(changed_keys "$1" "$2" | json_string_array)"
  # THE DELTA RECORDS THE DIGEST OF THE BYTES IT READ. gate/baseline.json is
  # what every key hashes; this file is a summary of a comparison against it.
  # Without the digest the two are only assumed to be about each other, and a
  # re-resolved baseline with an un-recomputed delta leaves thirteen keys
  # carrying an inventory the comparison never saw.
  _baseline_sha="$(sha256_of "$2")"
  printf '{\n'
  printf '  "baseline": {"ref": "%s", "commit": "%s", "sha256": "%s"},\n' "$3" "$4" "$_baseline_sha"
  printf '  "changed": %s\n' "$_changed"
  printf '}\n'
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
  delta   --baseline-ref R --baseline-commit C
                                 resolve the baseline out of commit C itself,
                                 write gate/baseline.json and gate/delta.json
                                 (no --tree: the delta is a pure function of the
                                 two inventories, and its freshness is proven by
                                 recomputation in `record` rather than by a
                                 stamp; no --baseline-file either: the baseline
                                 is derived, never handed in — a file argument
                                 is refused by name)
  record  --tree T --baseline-ref R --baseline-commit C <artifact>...
                                 write gate/run.json over exactly these artifacts

  --root DIR  operate on another checkout (default: this script's repository)
USAGE
  exit 1
}

[ $# -ge 1 ] || usage
MODE="$1"; shift

TREE=""; BASELINE_REF=""; BASELINE_COMMIT=""; BASELINE_FILE=""; SURFACE=""; BUNDLE=""
ARTIFACTS=""
while [ $# -gt 0 ]; do
  case "$1" in
    --root)             ROOT="$(cd "$2" && pwd)"; shift 2 ;;
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

    # The producer's own output — the refusal, or the run it minted — goes to
    # stderr, so stdout carries the invocations and nothing else.
    ( cd "$ROOT" && go test ./cmd/dossierx -run "$PRODUCER_TEST" -count=1 -v -fanout-out -fanout-tree="$TREE" ) >&2 \
      || die "fanout: the producer refused this run (its reason is above). No run was minted and gate/fanout.json was not written, so nothing downstream can attribute an answer to this release; fix what it named and run \`fanout\` again." 5

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
    # NO --tree HERE, AND THAT IS A DECISION RATHER THAN AN OMISSION. The delta
    # used to be stamped with the tree so that `record` could refuse one
    # computed over a different tree, and the stamp defeated the gate's whole
    # cache: gate/delta.json is hashed into all thirteen surface keys and
    # assembled into all thirteen bundles, so the per-commit stamp re-keyed
    # every surface on every commit and a carried-forward verdict never once
    # fired. The freshness guard survives — `record` now RE-DERIVES the delta
    # from surface.json and gate/baseline.json and refuses on any byte of
    # disagreement (see emit_delta_document). A --tree passed by a driver
    # following the older procedure is accepted by the option parser and unused.
    #
    # THE BASELINE IS RESOLVED OR THE RUN FAILS. There is no branch here that
    # turns "I could not find the previous release" into an empty delta. An
    # empty delta is a legitimate and expected answer — this project's first
    # gated release changes no shipped code — and it is a completely different
    # statement from "there was nothing to compare against".
    resolved_object_name "$BASELINE_COMMIT" \
      || die "delta: the baseline could not be resolved — --baseline-commit is ${BASELINE_COMMIT:-empty} and must be a full 40-digit object name. A tag is a mutable pointer, an abbreviation is a prefix, and forty characters of something else is neither. An unresolvable baseline is a FAILED run; it is never an empty delta." 3
    [ -n "$BASELINE_REF" ] || die "delta: --baseline-ref is required (the human-readable name of what --baseline-commit resolved from)" 1
    # THE RETIRED FLAG IS A REFUSAL WITH ITS HISTORY, not an unknown option. An
    # invocation carrying it was copied from a revision of docs/RELEASING.md
    # that hard-coded the v0.5.0 bootstrap — the exact invocation that computed
    # the v0.6.0 gate's delta against a two-release-old inventory while
    # recording it under ref v0.5.1 — so the operator typing it believes they
    # are choosing the baseline's bytes, and silently ignoring the flag would
    # leave that belief standing over different bytes.
    [ -z "$BASELINE_FILE" ] || die "delta: --baseline-file is retired and refused. The baseline inventory is derived from --baseline-commit itself — the committed $BOOTSTRAP_FILE when that commit IS the frozen $BOOTSTRAP_COMMIT, and \`git show <commit>:surface.json\` for every later release — because a file argument is a second answer that can drift: hard-coded to the bootstrap, it computed one release's delta against an inventory two releases old. Drop the flag and run \`delta\` again." 1
    [ -f "$ROOT/surface.json" ] || die "delta: no surface.json under $ROOT to diff against the baseline" 2

    # Resolved into a private file first, so a refusal partway through `git
    # show` cannot leave a truncated gate/baseline.json behind looking like a
    # short inventory.
    _resolved="$(mktemp)"
    resolve_baseline_inventory "$BASELINE_COMMIT" "$_resolved"
    mkdir -p "$ROOT/gate"
    cp "$_resolved" "$ROOT/gate/baseline.json"
    rm -f "$_resolved"
    emit_delta_document "$ROOT/surface.json" "$ROOT/gate/baseline.json" "$BASELINE_REF" "$BASELINE_COMMIT" > "$ROOT/gate/delta.json"
    printf 'gate-stage2: wrote gate/baseline.json and gate/delta.json against %s (%s)\n' "$BASELINE_REF" "$BASELINE_COMMIT" >&2
    ;;

  record)
    [ -n "$TREE" ] || die "record: --tree is required" 1
    resolved_object_name "$BASELINE_COMMIT" \
      || die "record: the baseline could not be resolved — --baseline-commit is ${BASELINE_COMMIT:-empty} and must be a full 40-digit object name. A run manifest that cannot name the release it was compared against covers nothing." 3
    [ -n "$BASELINE_REF" ] || die "record: --baseline-ref is required" 1
    [ -n "$ARTIFACTS" ] || die "record: name the artifacts this run produced; a manifest over zero artifacts asserts nothing" 1

    # RECORDING A GUARDED ARTIFACT MEANS CLAIMING IT. Every other artifact this
    # mode names is opaque bytes it can only digest, but these it can hold to
    # account — so recording one that disagrees with this run is refused HERE,
    # at the point the disagreement is created.
    #
    # The sequence is ordinary and it is why this exists: a gate FAILS, a fix
    # lands, the tree moves, and the driver re-runs the captures and `record`
    # but not `delta` — or re-runs `delta` and not the captures. Re-digesting
    # whatever is on disk would launder the stale one into a manifest that is
    # honest about every byte it names.
    #
    # THREE KINDS OF ACCOUNT, per provenance_bearing above. The baseline is
    # re-resolved outright — it IS the named commit's inventory, so the guard
    # derives the same bytes again from the commit. The delta is recomputed
    # outright, because it is a pure function of two files this checkout holds;
    # a recomputation that agrees makes the file fresh BY CONSTRUCTION, however
    # long it has sat on disk. The two captures cannot be recomputed here — one
    # needs the rendered output of two releases, the other a real build in a
    # real browser — so their own stamps are read and
    # checked against the run's. Those stamped documents are read with
    # json_scalar, which takes the FIRST match on any line and exits; that is
    # correct only because both put "tree" (and, for the render diff,
    # "baseline"."commit") before everything else, so no later key and no diff
    # hunk can be read in their place. The ordering is a promise the producers
    # make to this reader, and it is pinned on their side —
    # tests/render_diff_capture_test.go, TestRenderDiffCaptureProvenanceComesFirst,
    # and viewer-tests/site_dom_test.go, TestSiteTextProvenanceComesFirst.
    for a in $ARTIFACTS; do
      provenance_bearing "$a" || continue
      # NOT `|| continue`. This loop IS the guard, and stepping over a guarded
      # artifact because it is absent would be the guard declining to run on a
      # state it exists for. Same refusal, same exit code as the digest loop
      # below.
      [ -f "$ROOT/$a" ] || die "record: $a was named as produced by this run and is not there" 4
      case "$a" in
        gate/baseline.json)
          # FRESHNESS BY RE-RESOLUTION. The baseline is not a comparison and
          # not a capture; it IS the named commit's inventory, so the guard is
          # to derive that inventory again and require byte equality. This is
          # the branch that refuses the two-release-old baseline recorded
          # under the previous release's ref — a state in which every digest
          # below is honest and the delta recomputation agrees with itself.
          require_recorded_baseline_is_resolved
          ;;
        gate/delta.json)
          # FRESHNESS BY RECOMPUTATION. The delta is emit_delta_document over
          # surface.json, gate/baseline.json and this run's baseline flags;
          # re-run the same function and require byte equality. This catches
          # every stale shape the old tree stamp caught — a delta left from
          # before a fix that moved the inventory now disagrees with the
          # recomputation — and it refuses hand-written or truncated deltas the
          # stamp waved through when the stamp happened to be right. The two
          # inputs are required, not defaulted: a recomputation that cannot run
          # is a failure, never a pass.
          [ -f "$ROOT/surface.json" ] \
            || die "record: gate/delta.json is named as produced and there is no surface.json under $ROOT, so the delta cannot be recomputed and its freshness cannot be checked. A check that cannot run is a failure, not a pass." 2
          [ -f "$ROOT/gate/baseline.json" ] \
            || die "record: gate/delta.json is named as produced and gate/baseline.json is not there. The delta is a comparison against those bytes; without them nothing can say whether the comparison on disk is the one this tree would make." 2
          # The baseline is re-resolved FIRST, because the recomputation below
          # runs over gate/baseline.json's bytes: recomputed over a stale
          # baseline, a delta honestly computed over that same stale baseline
          # byte-agrees, and the pair records cleanly — which is exactly the
          # laundering that shipped a two-release comparison once.
          require_recorded_baseline_is_resolved
          _expected="$(mktemp)"
          emit_delta_document "$ROOT/surface.json" "$ROOT/gate/baseline.json" "$BASELINE_REF" "$BASELINE_COMMIT" > "$_expected"
          if ! cmp -s "$_expected" "$ROOT/$a"; then
            rm -f "$_expected"
            die "record: gate/delta.json is not the delta this tree would produce over gate/baseline.json and baseline $BASELINE_REF ($BASELINE_COMMIT). It is stale — computed before a fix moved the inventory, against another baseline, or written by hand — and recording it would hand every surface agent a comparison this release never made. Re-run \`delta\` for this checkout and \`record\` again." 3
          fi
          rm -f "$_expected"
          ;;
        *)
          _dtree="$(json_scalar "$ROOT/$a" tree)"
          # THE IDENTITY RULE APPLIED TO WHAT THE FILE SAYS, not just to what
          # the caller said. An artifact that names no tree — `printf '{}' >
          # gate/render-diff.json`, the one-line workaround for a gate that has
          # been refusing for ten minutes, or a site extraction run without
          # DOSSIERX_SITE_TEXT_TREE — is refused here rather than stepped over:
          # downstream it is indistinguishable from a capture of this release,
          # because the manifest is honest about its bytes and its digest
          # matches.
          resolved_object_name "$_dtree" \
            || die "record: $a records tree ${_dtree:-nothing}, which is not a full 40-digit object name. An artifact that cannot say which tree it covers cannot be checked against this run at all, and a file that says nothing hashes into every key exactly as cleanly as one that says the truth." 3
          [ "$_dtree" = "$TREE" ] \
            || die "record: $a was computed over tree $_dtree and this run covers $TREE. Re-produce it for this tree — the -render-diff-out capture entry point for gate/render-diff.json, the DOSSIERX_SITE_TEXT_OUT extraction for gate/site-text.json; recording it as produced here would hand a surface agent another release's evidence as this one's." 3
          case "$a" in
            gate/render-diff.json)
              # The render diff is a comparison AGAINST the baseline, so it
              # also states which baseline — the site extraction compares this
              # tree's build against nothing and carries no commit to check.
              _dcommit="$(json_scalar "$ROOT/$a" commit)"
              resolved_object_name "$_dcommit" \
                || die "record: $a records baseline commit ${_dcommit:-nothing}, which is not a full 40-digit object name. A tag is a mutable pointer and an abbreviation is a prefix; either can mean a different release tomorrow than it meant when this run recorded it." 3
              [ "$_dcommit" = "$BASELINE_COMMIT" ] \
                || die "record: $a compared against baseline $_dcommit and this run resolved $BASELINE_COMMIT" 3
              ;;
          esac
          ;;
      esac
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
