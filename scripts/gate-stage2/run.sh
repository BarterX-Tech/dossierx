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

set -eu

die() { printf 'gate-stage2: %s\n' "$1" >&2; exit "${2:-1}"; }

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
MANIFEST_FILE="surfaces.yaml"
METHOD_FILE="gate/method.yaml"

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
  delta   --tree T --baseline-ref R --baseline-commit C --baseline-file F
                                 resolve the baseline, write gate/baseline.json
                                 and gate/delta.json
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
