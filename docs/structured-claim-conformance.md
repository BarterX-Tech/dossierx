# Structured claim conformance v1

This document is the normative contract for the first structured claim
conformance slice. It uses only project-neutral identifiers and data. The words
MUST, MUST NOT, SHOULD, and MAY are requirements.

## Boundary

DossierX compares an authored expectation with a normalized observation. It
does not inspect source code, run an adapter, or interpret an adapter or target
identifier. A project-owned adapter produces the observation file.

Claim approval, dependency readiness, and implementation conformance are three
different facts. Computing conformance MUST NOT approve, lock, unlock, rewrite,
or mutate a claim, lock ledger, receipt, baseline, flag, edge, or observation.

## Claim schema

A claim MAY carry one `embodiment` declaration.

Comparison form:

```yaml
embodiment:
  mode: compare
  checks:
    - id: public-values
      adapter: source-symbols/v1
      target: source://widget/state
      expectation:
        shape: set
        value: [blocked, ready, waiting]
    - id: schema-version
      adapter: schema-metadata/v1
      target: schema://widget/record
      expectation:
        shape: scalar
        value: "3"
```

Deliberate no-embodiment form:

```yaml
embodiment:
  mode: none
  reason: This claim describes documentation structure only.
```

For `mode: compare`, `checks` is required and MUST contain one or more entries.
Each check requires a non-empty `id`, `adapter`, `target`, and `expectation`.
Check IDs MUST be unique within the claim and are the stable identity used by
generated status, catalog, viewer, and automation; YAML list position is not
identity. The pair `(adapter, target)` MUST also be unique within one claim.
Checks are canonicalized by ID, so reordering the list has no semantic effect.
`adapter` and `target` are non-empty opaque strings.

An expectation has exactly one of these shapes:

- `shape: set` has `value` containing one or more unique, non-empty strings.
  Order has no meaning and generated members are sorted lexicographically.
- `shape: scalar` has `value` containing exactly one non-empty string. DossierX
  compares that string exactly and does not parse versions, durations, units,
  numbers, or other domain types. Normalization belongs to the adapter.

Every check ID, adapter, target, scalar value, and set member MUST carry the
YAML string type; numeric, boolean, null, mapping, and mixed-type values are
rejected rather than coerced.

For `mode: none`, `reason` is required and `checks` MUST be absent, including a
key whose value is null or an empty list. An omitted `embodiment` means the
project has not opted that claim into
conformance; it is not equivalent to `mode: none`.

Malformed declarations and unsupported shapes are claim-schema errors. They
MUST be reported clearly before a comparison is attempted.

`embodiment` is authored claim content; declaring it does not approve the
claim. A non-empty declaration is included in the locked-claim signature when
the claim is locked. Its semantic content is also included in the
dependency content hash: mode, and for every compare check in ID order its
stable ID, shape, and canonical expected value; mode and reason for `none`.
Opaque adapter and target addresses are excluded from the dependency content
hash because moving an observation probe does not change the claim's promised
fact, but they remain covered by the exact locked-claim signature. A claim with
no declaration MUST retain its prior hashes. Reordering checks or an expected
set MUST NOT move the dependency content hash.
For compatibility, an omitted declaration returns the completed legacy digest
literally. An opted-in declaration is hashed in a separate v1 domain over that
binary legacy digest plus a closed, length-framed canonical embodiment encoding;
authored legacy strings such as `raw_html` cannot inject or collide with the new
domain.

## Observation input

A project opts into observation loading with this optional config field:

```yaml
conformance:
  observations: observations/conformance.json
  blocking: false
```

The path is resolved relative to `project.config.yaml`, must remain outside
`build_dir` lexically and after existing parent symlinks are resolved at read
time, and is treated as one literal regular file name (not a symlink or
gitlink). DossierX never guesses or scans for this file.

The file is a strict JSON envelope:

```json
{
  "format_version": 1,
  "snapshot": "7c91e2a",
  "observations": [
    {
      "adapter": "source-symbols/v1",
      "target": "source://widget/state",
      "shape": "set",
      "value": ["blocked", "ready", "waiting"]
    },
    {
      "adapter": "schema-metadata/v1",
      "target": "schema://widget/record",
      "shape": "scalar",
      "value": "3"
    }
  ]
}
```

An adapter that reached a target but could not observe it emits an error arm
instead of silently dropping the target:

```json
{
  "adapter": "source-symbols/v1",
  "target": "source://widget/state",
  "error": {"code": "input_unavailable", "message": "source snapshot is unavailable"}
}
```

`format_version` MUST be `1`. `snapshot` is optional opaque string provenance;
when present it MUST NOT be null or another JSON type.
Each observation uses non-empty opaque `adapter` and `target` strings, then
exactly one of two arms: a supported `shape` with its shape-specific `value`, or
an `error` with non-empty opaque code and human-readable message. `shape: set`
has one or more unique, non-empty string members; `shape: scalar` has one
non-empty string.
The error arm MUST NOT carry `shape` or `value`. The pair `(adapter, target)`
MUST be unique in one envelope. Unknown fields, malformed JSON,
unsupported versions or shapes, duplicate keys, invalid values, unreadable
files, and missing configured files make the input uncheckable. They MUST NOT
look matched or owed. Generated results preserve an adapter error's opaque
`observation_error` object; engine-authored prose describes the state without
copying raw filesystem errors or absolute paths.

If at least one claim uses `mode: compare` but no observation path is
configured, all of its checks are uncheckable. A valid, readable envelope that
does not contain a check's `(adapter, target)` produces owed for that check.

## Results

Comparison has exactly four outcomes in v1:

| State | Meaning |
| --- | --- |
| `matched` | The expected and observed values are equal under their declared shape. |
| `owed` | The envelope is valid but has no observation for the check's adapter and target. |
| `mismatch` | The observation exists but differs from the expected set or scalar. |
| `uncheckable` | The observation input cannot be interpreted safely, or the matching observation carries an adapter error. |

For sets, `missing` is `expected - observed` and `extra` is `observed -
expected`. Both are sorted lexicographically, as are expected and observed
members. For scalars, a mismatch reports only the exact `expected` and
`observed` strings; set-only `missing` and `extra` fields are absent. A shape
mismatch is uncheckable, never a mismatch.

`mode: none` is a declaration, not a fifth comparison outcome. It is shown as
`declared_none`, with its reason, and is implementation-ready by declaration.
Claims with no `embodiment` are absent from conformance output.

For `mode: compare`, each check carries its own state and the claim-level
`implementation_ready` is true only when every check is `matched`.
For `mode: none`, it is true. This field means ready only for this declared
embodiment. It does not imply claim, dependency, integrated, or release
readiness. This projection MUST NOT change the existing
readiness assessment or its `ready`, `local_approved`, `dependency_ready`, or
`review_pending` values.

The optional project setting `conformance.blocking` controls whether a
non-matched check is a command gate. Omitted or `false` preserves report-only
behavior. When `true`, any owed, mismatched, or uncheckable check causes
`dossierx check` to exit non-zero with stable code `conformance_failed`, after a
plain write-mode check has generated the status, catalog, and viewer so the
failure remains inspectable. This gate never changes approval, locking, or
readiness policy. `mode: none` does not block.
`check --validate` computes the same projection without writing it.
`check --staged` reads both claims and the configured observation literally
from the git index. A missing, non-regular, or oversized staged input reports
the affected checks as uncheckable; it never mixes staged claims with a
working-tree observation. Both read-only forms exit non-zero under blocking
policy without writing generated artifacts. `dossierx serve` continues serving
and displays the state; blocking policy does not terminate the server.

## Generated outputs

When at least one claim declares `embodiment`, `dossierx check` writes:

```text
build/conformance/status.json
```

The artifact has `format_version: 1`, optional copied `snapshot`, a summary,
and one entry per declared claim sorted by claim id. Compare entries group one
or more check results in stable check-ID order. It contains no timestamp or
absolute path. The catalog projects the same per-claim group, including scalar
expected/observed values and exact set missing/extra members. The generated
viewer renders the group alongside the selected claim component. Every declared
panel carries
`data-implementation-ready="true"` or `"false"`. After a completed write-mode
projection, including one that then returns `conformance_failed` under blocking
policy, the status artifact, catalog, and viewer MUST agree. All three are
preflighted before replacement; a later filesystem write
error is reported and does not claim a successfully refreshed set.

Summary fields do not blur the two levels: `declared`, `declared_none`,
`implementation_ready`, and `implementation_blocked` count claims; `checks` and
the four state fields count nested compare checks.

For a project that has never declared `embodiment`, DossierX does not read an
observation file, does not write `build/conformance/status.json`, and does not
add a conformance field, viewer markup, or conformance-only style bytes.
Existing generated output remains unchanged. Its read-only validation path also
stays unchanged: lint, theme, and ledger checks run, but no catalog or viewer is
built merely to validate a feature the project did not opt into. The in-memory
catalog/viewer agreement and capacity checks begin only when at least one claim
declares `embodiment`. On the one transition from an
opted-in project to zero declarations, `check` removes the now-stale generated
status file after regenerating a catalog and viewer with no conformance
projection. An exact private marker at
`build/conformance/.dossierx-owned` records engine ownership independently of
the project-editable `build/.gitignore`. A new lifecycle refuses an unmarked
pre-existing status, establishes the exact marker before replacing any generated
artifact, and removes the marker only after both stale status removal and the
generated ignore-rule downgrade succeed. It remains on failure so a retry can
finish. A never-opted
project with arbitrary bytes at the status path, even beside a matching ignore
rule, is untouched. DossierX never removes or rewrites the project-owned
observation input.

`dossierx serve` routes concurrent `GET` and `HEAD /api/status` requests through
one start-at-or-after single-flight pipeline: at most one status projection is
running and one is queued behind it, and all callers in a batch share the same
result. The readiness map and the capacity verdict come from that same snapshot,
responses carry `Cache-Control: no-store`, and a watcher change pre-warms a fresh
batch. Conformance-enabled viewers also reject a superseded status response at
both header and JSON completion, so transport reordering cannot repaint an older
verdict; that client guard is omitted byte-for-byte from viewers without
conformance. Projection failures remain in their actual `conformance_error`,
`catalog_error`, or `render_error` domain with `failure_phase`; capacity uses the
shared stable `conformance_capacity_exceeded` code without relabeling the cause.

## Determinism and bounds

Let `C` be claims with an embodiment declaration, `K` the total compare checks,
`O` observations, and `M` the value visits across comparisons: each scalar or
set member expected plus each scalar or set member in the matching observation,
with a shared observation counted once for every check that compares it because
its output is repeated. Let `B` be input bytes and `S` the corresponding check
ID, adapter, target, scalar, and set-member string bytes.

The loader reads at most one configured file and rejects files larger than 16
MiB. It stores at most one observation per `(adapter, target)`. Evaluation uses
one indexed lookup per check. Set construction and difference work are `O(M)`.
Sorting costs `O(M log M)`. A shared observation value is retained once;
results reference canonical observation values instead of copying them for
every check.
Before any difference slices are retained, an allocation-free pass counts the
JSON bytes that must exist: mandatory record structure plus exact encoded
strings. That lower bound may refuse only when it proves the report exceeds 64
MiB. A separate conservative upper bound can prove that a report fits, but it
is never refusal authority. Ambiguous reports are constructed and measured
exactly before any generated artifact is replaced. Capacity errors are reported
as `conformance_capacity_exceeded`; output is never partially truncated or made
order-dependent.
Claim-result record count is exactly `C`, with exactly `K` nested check records;
no result depends on claim graph paths. Serialized status and catalog growth is
`O(C + K + M + S)`. Ordering work is `O(C log C + K log K + M log M)` and does
not depend on possible graph paths. Viewer growth is the
same data rendered once per existing claim-card copy; existing overview
duplication rules remain the only multiplier. The complete serialized status,
catalog, and viewer artifacts are each capped at 64 MiB. Status uses a
mandatory-byte lower bound, a conservative fit proof, and exact measurement
when those bounds do not decide the result. Catalog likewise refuses only from
provably mandatory encoded bytes and measures ambiguous output, so padding in
an estimate cannot produce a false size claim. For the embedded viewer shell,
whose projection contract is fixed, the bounded path shares one output-derived
budget across claim fragments, graph and build-order JSON, repeated track
content, and the final shell writer. A project shell override receives those
expensive fields lazily: an omitted field or an unexecuted template branch is
not computed or charged as output. Its separate 128 MiB retained-intermediate
safety guard never claims that the final HTML is that large; only the capped
final writer decides the inclusive 64 MiB output verdict. All three capacity
checks finish before the first artifact is replaced.

No graph traversal, dependency-path enumeration, adapter execution, arbitrary
record scan, record comparison, sequence comparison, or historical drift
baseline is part of v1. `claim show` conformance, a rulings register and its
lints, defined-term ownership, records-ID and scoped-numeral lints, and
unification with file-level implementation links are explicitly deferred.
