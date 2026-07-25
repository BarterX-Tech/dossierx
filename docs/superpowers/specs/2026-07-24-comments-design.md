# DossierX Comments — Design (v2, post-audit)

Date: 2026-07-24
Status: draft, pending review
Revision: v2 — hardened against the multi-agent audit (33 confirmed findings). See
`audit-findings-raw.md` for the full finding text; the changes below are the applied
synthesis.

## Problem

Reviewers (human and agent) need Google-Docs/Notion-style comments on claims: remarks
attach to a claim, get discussed, and must all be resolved before the claim can lock.
The human reviews only the generated HTML viewer, which today is a static file with no
write path back to the claim YAML. Comments must live in the claim files themselves and
be visible in the viewer at runtime.

## Decisions (settled during brainstorming)

| Question | Decision |
|---|---|
| Anchoring | Whole claim card (no text-range anchors) |
| Threading | Threaded: top-level comment + replies; resolve acts on the thread |
| Comment on locked claim | New open thread sets `review_pending: true` — **a legal, long-lived steady state** (see Invariants) |
| Locked claim with an open thread | `dossierx lint` / `dossierx check` still exit 0 and still render; only that claim's own re-lock/reaudit is gated |
| Resolve rights | Human-opened threads resolved only by human; agent-opened threads by either. Advisory — asserted via the `--as` actor flag, not authenticated (see Threat model) |
| Human write-back path | New `dossierx serve` local HTTP server + JSON API |
| Content hash | Comments excluded from `ContentHash` — bookkeeping like `audit_notes` |
| Reply addressing | By engine-generated reply **id**, never by ordinal index |
| Comment write locking | One project-wide claims sentinel; every claim-file writer takes it and re-reads inside the critical section (see Concurrency) |

## Invariants (normative — every section below obeys these)

1. **No comment state may ever stop `render` from running.** `serve` is the human's
   only UI; a claim you cannot render is a claim you cannot un-block. Therefore open
   comment threads are never an error-severity lint finding, and `GET /` never gates
   the page on lint.
2. **`review_pending` is derived, not authoritative.** The source of truth for the
   comment trigger is `comments[].status == open` in the claim YAML. `review_pending`
   is a mirror maintained by a single reconciler and a single shared predicate.
3. **Claim files are a shared resource.** Once the engine writes per-claim comment
   content, every writer of a claim file must serialize and re-read inside a lock.
4. **The engine is config-driven and CLI-first.** No project-specific behavior enters
   engine source; `serve` is an optional interactive surface, not a new required mode.

## Data model

New `comments:` field on the claim YAML, engine-managed (precedent: `audit_notes`,
`review_pending`). Added to `model.Claim` (strict decoder `KnownFields(true)` requires
the struct field + a FORMAT.md update in the same change, or every existing claim fails
to load).

```yaml
comments:
  - id: c-8f3a2b            # engine-generated, unique within the claim file
    status: open            # open | resolved
    author: human           # human | agent — role, not identity
    created: 2026-07-24T10:12:00Z   # RFC 3339 UTC
    body: |
      This row contradicts the API facet — which is right?
    edited: false           # set true after an Edit
    replies:
      - id: r-4c9e11        # engine-generated, unique within the claim file
        author: agent
        created: 2026-07-24T10:40:00Z
        body: Fixed the rows; API facet was stale.
        edited: false
    resolved_by: human
    resolved_at: 2026-07-24T11:02:00Z
    reopened_by: ""         # last reopener role, for rights on the next cycle
    reopened_at: ""
```

Go types in `internal/model`:
- `Comment` (`id, status, author, created, body, edited, replies, resolved_by,
  resolved_at, reopened_by, reopened_at`)
- `Reply` (`id, author, created, body, edited`)
- `CommentRole` string enum (`human` | `agent`) — one type, used by every op and flag.

Tagged `yaml:"comments,omitempty"` so comment-free claims stay byte-identical on disk.
Excluded from `lock.ContentHash` so commenting never flips dependents to
`review_pending`.

**Id generation.** The repo has no RNG today (no `crypto/rand`/`math/rand` import).
Phase 1 introduces one shared generator: prefix (`c-` threads, `r-` replies) + 6
lowercase hex. Ids are minted **after the re-read and inside the claims-lock critical
section**, regenerated on collision within the claim file.

**Hand-authored / legacy comments with no `id`.** Decision: backfill missing ids on the
next `SaveClaim` (a load that finds an id-less comment/reply assigns one). This keeps
`KnownFields(true)` happy without forcing humans to hand-write ids.

**YAML fidelity risk (must be verified in Phase 1).** `loader.SaveClaim` does
`yaml.Marshal(whole struct)` + atomic replace — it does **not** preserve `#` comments,
key order, or block-scalar style of the original file. This is pre-existing behavior but
comments make bodies user-authored free text. Phase 1 must prove (property/fuzz test)
that hostile bodies (quotes, colons, leading `-`, `---`, tabs, unicode, emoji, CRLF,
long text) round-trip byte-exact through Marshal→decode.

## Concurrency & claim-file write discipline (Phase 0 — lands before Phase 1)

**The premise "the existing file lock serializes CLI vs serve" is false and must be
built.** `lock.AcquireFileLock(p)` is an advisory O_EXCL sentinel at `p + ".lock"`,
per-store-path; today three distinct sentinels exist (`.dossierx-lock-store.json`,
`.dossierx-flag-store.json`, per-module buildorder). **None covers a claim file.**
`loader.SaveClaim` marshals the whole struct, so any writer holding a pre-comment
snapshot silently erases comments.

Phase 0 establishes:

1. **One project-wide claims sentinel:** `lock.AcquireFileLock(filepath.Join(cfg.Dir(),
   ".dossierx-claims"))` (note `AcquireFileLock` appends `.lock`; pass the base path).
   It lives under `cfg.Dir()`, **outside `claims_dir`**, so acquiring/releasing it never
   trips the fsnotify watcher.
2. **Every claim-file writer takes it and re-reads inside the critical section**
   (acquire → load → mutate → SaveClaim → release; never load-before-acquire). This is a
   retrofit of existing commands, not additive-only:
   - `lock` (`main.go:690` loads before `:705` acquire — reorder; the `comments_unresolved`
     gate is only sound if `lock.Lock` lints claims read inside the lock)
   - `unlock` (`main.go:748` — no lock today)
   - `check`'s `review_pending` persistence (`main.go:317-329` — no lock today)
   - `flag` (`flag.go` — holds only the flag-store lock)
   - `reaudit --confirm` (`main.go:852`)
   - all new `dossierx comment` verbs and every serve handler.
3. **Acquisition order (global, to avoid AB-BA deadlock):** claims → lock-store →
   flag-store, matching reaudit's existing store-then-flag order. Simplification: comment
   ops read the two JSON stores **without** taking their sentinels (a stale read only
   risks leaving `review_pending` set, which the next reconcile corrects) and take
   exactly one sentinel — the claims lock — eliminating the ordering hazard by
   construction.
4. **Bounded critical section.** Hold the claims lock across load→mutate→save only.
   Render/catalog/implink scan run **outside** it. `serve` must release before running
   the check/render pipeline, or every page load blocks agent CLI writes for a full
   render.
5. **serve lock lifecycle.** serve is the first long-lived holder. It installs a
   SIGINT/SIGTERM handler that releases held locks before exit (`defer` does not run on
   signal death). Optionally write PID+timestamp into the sentinel so a lock older than N
   seconds whose PID is gone can be broken rather than wedging every CLI invocation for
   the 10 s `AcquireFileLock` timeout.
6. **Optimistic-concurrency backstop (recommended).** Add `loader.SaveClaimIfUnchanged`
   that records mtime+size (or content hash) at load and refuses the write if the file
   changed underneath, returning a structured conflict error the API surfaces as a
   "reload — this claim changed" toast (HTTP 409). This is the only defense against
   out-of-band edits (an editor, or a future writer that forgets the lock).

Phase 0 tests (`-race`, but note the dangerous races are cross-process TOCTOU that
`-race` cannot see — assert on file content, not the detector):
- `comment add` raced against `dossierx lock` on the same claim → outcome is always
  "lock refused" or "lock succeeded and no comment lost", never "locked with an open
  thread".
- `comment add` raced against `unlock`, `flag`, and a `check` run that flips
  `review_pending` → the `comments:` block survives.
- `GET /` raced against `POST .../comments` inside one serve process → comment survives,
  response body is a complete document.
- assertion that a claim's comment count never decreases across a concurrency run.

## Business logic — `internal/comments`

Shared package used by both CLI and serve (single code path). Because ops must consult
the lock store and flag store to compute pending triggers, they take a dependency bundle,
not bare ids:

```go
type Deps struct {
    Cfg       *config.Config
    Claims    []model.Claim        // loaded via loader.LoadClaims — SaveClaim needs SourcePath
    LockStore *lock.Store
    FlagStore *reaudit.FlagStore
}
func (d *Deps) Add(claimID string, actor model.CommentRole, body string) (model.Claim, string, error) // returns new thread id
func (d *Deps) Reply(claimID, threadID string, actor model.CommentRole, body string) (model.Claim, string, error) // returns new reply id
func (d *Deps) Resolve(claimID, threadID string, actor model.CommentRole) (model.Claim, error)
func (d *Deps) Reopen(claimID, threadID string, actor model.CommentRole) (model.Claim, error)
func (d *Deps) Edit(claimID, threadID, replyID string, actor model.CommentRole, body string) (model.Claim, error) // replyID=="" ⇒ thread root
func (d *Deps) Delete(claimID, threadID, replyID string, actor model.CommentRole) (model.Claim, error)
func (d *Deps) List(claimID string, openOnly bool) ([]model.Comment, error)
```

Rules:
- **Rights (advisory).** Human-opened threads resolvable/reopenable/editable/deletable
  only by `human`; agent-opened by either. Rights checks run against the target resolved
  **by id first** — an unknown thread/reply id is a structured not-found error, never a
  positional fallback onto a neighbouring message. `reopen` carries an actor and the same
  rule as resolve (an agent may not reopen a human-resolved human thread).
- **`review_pending` is derived** via one shared predicate (below); comment ops never
  call `lock.ClearReviewPending`.
- Each op: acquire claims lock → `loader.LoadClaims` → mutate the target → mint ids →
  `loader.SaveClaim` → release. No in-memory state carried across requests.

### The single pending-trigger predicate

`review_pending` has **three independent triggers** and **three clearers**. To stop them
diverging, one predicate is the sole authority, with four callers (`comments.Resolve/
Delete`, `reaudit`, `check`/`serve` reconcile, `buildorder` — see below):

```go
// lives where it can be imported by comments, reaudit, and cmd; the raw
// "has open threads" test lives on model.Claim so internal/lint can use it
// without an import cycle (lock imports lint).
func PendingTriggers(c model.Claim, claims []model.Claim, ls *lock.Store, fs *reaudit.FlagStore) (drift, flag bool, openThreads int)
func Recompute(...) bool // = drift || flag || openThreads > 0
```
- `drift` = any dep in `c.Mirrors ∪ c.RestsOn` with `ls.Hashes[dep] != lock.ContentHash(depClaim)` (exactly `DetectStale`'s test).
- `flag` = `fs.Flags[c.ID]` present.
- `openThreads` = count of `c.Comments` with `status: open`.

`Resolve`/`Delete` of the last open thread clear `review_pending` **iff** `!drift &&
!flag`; otherwise they leave it set and print why ("review_pending retained: pending
flag" / "dependency <id> drifted").

### review_pending contract change (touches four docs — do not let them drift)

`comments` introduces a second/third clearing path, contradicting a "only a confirmed
reaudit --confirm clears review_pending" statement asserted verbatim in four places.
Update all, in the code phases (not phase 6) where the behavior lands:
- `internal/lock/lock.go:4-10` package-doc state machine → three triggers, three clearers.
- `internal/lock/lock.go` `ClearReviewPending` doc + a "do not call from internal/comments;
  it re-baselines hashes" warning. Split it: `RefreshBaseline` (rebaseline hashes + stamp
  `LockedAt`, correct after a confirmed reaudit) vs setting `ReviewPending` (a whole-claim
  verdict the caller makes from `Recompute`).
- `internal/reaudit/flagstore.go:13-16` ("both paths converge on the same reaudit --confirm
  gate — there is deliberately no second lifecycle state") → note comments are a third
  trigger that deliberately does NOT route through reaudit (a comment carries no proposed
  content edit, so there is nothing to diff-and-confirm).
- `FORMAT.md:130-137`, `README.md:97`, `skills/dossierx-claims/SKILL.md:115-116,143-145`,
  and `site/src/content.ts:184,261,271` — rewrite (do not append) the now-false sentences.

## CLI

New `dossierx comment` command group. One actor flag `--as` on every mutating verb:

```
dossierx comment add     <claim-id> --as human|agent --body "..."
dossierx comment reply   <claim-id> <thread-id> --as human|agent --body "..."
dossierx comment resolve <claim-id> <thread-id> --as human|agent
dossierx comment reopen  <claim-id> <thread-id> --as human|agent
dossierx comment edit    <claim-id> <thread-id> [--reply <reply-id>] --as ... --body "..."
dossierx comment delete  <claim-id> <thread-id> [--reply <reply-id>] --as ...
dossierx comment list    [<claim-id>] [--open] [--json]
```

(`author`/`resolved_by` remain the stored YAML field names — `--as` sets them; the flag
name and the schema field are deliberately distinct.)

**Output contract:**
- Mutating verbs echo the id they minted: `comment add: <claim-id> thread <tid> opened`,
  `comment reply: <claim-id> thread <tid> reply <rid> added` — so an agent needn't
  re-`list` to learn the id (matches implink/flag/build_order echo style).
- `comment list` prints one thread per line, greppable, ids included:
  `<claim-id>  <tid>  <status>  <author>  <created>  <reply-count>  <first-line-of-body>`;
  replies indented with their `<rid>`. `--open` filters to `status: open`. This exact
  format is a contract consumed by the skill, the site terminal vignette, and a golden
  test — pin it once.
- `--json` on `list` (reusing the `lint --json` encoder settings) emits a top-level
  array; struct carries claim id, thread id, status, author, created, body, edited,
  resolved_by/at, reopened_by/at, and per-reply id/author/created/body/edited.
- **No `viewer:` trailer on comment verbs** (they do not render). Instead:
  `comment: <tid> added on <claim-id>; run "dossierx check" or "dossierx serve" to view`.
  Under `--json`, stdout is the JSON document only (any hint goes to stderr).
- Drop the `viewer:` trailer for `render`/`check` too: `runRender` already prints
  `render: wrote <abs path>` and `cfg.Dir()` is already absolute — the trailer would
  double-print. Never print a path for a file that does not exist (`renderOutPath` is a
  pure join; stat first).

Agents read thread/reply ids from `comment list` (or the claim YAML) but **mutate only
through the CLI** (the claims lock is the reason).

## Lock gate

**The gate is NOT an error-severity lint.** (Routing it through the lint registry is
unsound: `Lint.Check(claims, cfg)` has no lock-time context, so the only proxy is
`Status==locked`, which makes every already-locked commented claim emit an error;
`lock.Lock` counts errors project-wide, so one open thread anywhere would freeze locking
for every claim and make `dossierx check` exit non-zero — killing the render the reviewer
needs. `rest_on_locked.go` is also mis-cited by v1: it is silent on drafts and always
error, never a draft-warning.)

Two pieces:

1. **`internal/lint/comments_unresolved.go` — WARNING severity, always**, for draft and
   locked claims alike, no `Status` branching:
   `Finding{LintName:"comments-unresolved", ClaimID:c.ID, Message:"N unresolved comment thread(s)", Severity: lint.SeverityWarning}`.
   Warnings print but never fail, so `lint`/`check`/`serve` stay exit-0 and catalog+render
   still run.
2. **The actual gate is a candidate-scoped refusal inside `lock.Lock`,** beside the
   existing non-lint `checkHubGating`:
   ```go
   if open := openThreadIDs(claim); len(open) > 0 {
       return claim, fmt.Errorf("lock: refused, claim %q has %d unresolved comment thread(s): %v — resolve with \"dossierx comment resolve %s <thread-id>\"", claim.ID, len(open), open, claim.ID)
   }
   ```
   `openThreadIDs` is a pure predicate over `claim.Comments` (`internal/model` only).
   Update `Lock`'s doc comment to name this third refusal path. **Delete** v1's "no
   lock.go change for the gate itself" and "scoped like rest_on_locked.go" sentences.

Truth table:

| state | `dossierx lint` / `check` | `dossierx lock <this claim>` |
|---|---|---|
| draft + open thread | warning, exit 0 | refused (gate) |
| locked + open thread (`review_pending` set) | warning, exit 0, catalog+render still run | n/a (already locked) |
| any + all resolved | silent | allowed |

`Unlock` stays deliberately ungated — the escape hatch from a locked+commented claim is
`unlock → fix → lock`.

**`dossierx check` additions:** a non-blocking `open comments: module "x": N` summary
line, and next-steps entries **partitioned by trigger** (using `PendingTriggers`):
comment-only pending → `resolve N open comment thread(s) on <id> → dossierx comment
resolve <id> <tid>`; drift/flag pending → the existing `reaudit` hint; both → comment
hint first. (The v1 wording "before locking" is wrong for an already-locked claim.)

**reaudit interaction:** `reaudit` on a comment-only `review_pending` claim exits 2 with a
comment-specific message and does **not** propose or write. On a drift/flag+comment claim
it applies the drift half (`RefreshBaseline` + delete flag entry) but leaves
`review_pending` true. A reconciler in `check`/`serve` sets `review_pending` for any
locked claim with an open thread (self-heals from disk after external edits / re-lock).

**buildorder completeness gate (code, not just skill prose):** extend
`buildorder.Propose`'s completeness gate (`internal/buildorder/buildorder.go:170-183`)
with a second accumulator that rejects any module claim with an open comment thread,
gating on `c.Comments` directly (not `review_pending`), listing the offending ids. Update
the `Propose` doc comment and reconcile `check`'s `fullyLocked` hint to the same
predicate. This lands in Phase 2 (with the gate), not Phase 6.

## `dossierx serve`

New subcommand `dossierx serve [--port]`. Default to a random high port (a well-known
port makes drive-by DNS-rebinding cheap); print the chosen URL
(`serving: http://127.0.0.1:<port>/`).

### Request-admission middleware (applied to EVERY route, before any handler)

An Origin check alone does not secure a localhost write API. Normative rules:

1. **Host allowlist (anti-rebinding; every request incl. `GET /` and `/api/events`):**
   reject `421` unless the host part of `r.Host` is exactly `127.0.0.1` or `localhost`
   with the listening port. **Never compare Origin against `r.Host`** (rebinding makes
   attacker.com match itself). This is the only DNS-rebinding defense.
2. **Origin allowlist on mutating methods (POST/PATCH/DELETE):** reject `403` unless
   `Origin` string-matches `http://127.0.0.1:<port>` or `http://localhost:<port>`.
   **`Origin: null` and absent Origin are rejected** — this is the decision implementers
   get wrong. The CI walkthrough's HTTP client sends an explicit allowed `Origin`.
3. **Content-Type:** POST/PATCH require parsed media type exactly `application/json`
   (kills the `enctype="text/plain"` simple-request CSRF path).
4. **Sec-Fetch-Site:** when present, reject anything other than `same-origin`/`none`.
5. **No CORS headers, ever.** The server emits no `Access-Control-Allow-Origin`.
6. Request body size limit (`http.MaxBytesReader`).

### `GET /` renders; it never gates the page on lint, and it never writes disk

`catalog.Build` cannot fail (returns `nil` error unconditionally; correctness is lint's
job), so duplicate ids / dangling refs / cycles / unlocked `rests_on` all still render.
Therefore:
- `GET /` calls `catalog.Build` + `render.Render` on current claims and returns 200 with
  the viewer, **always**. Do **not** reuse `newCheckCmd`'s RunE (it returns before
  catalog/render on the first error finding and hard-fails on impl-link scan errors) —
  this is a deliberate divergence from check's fail-fast contract.
- Lint + `implink.Scan` results become **data on the page** (status strip) and terminal
  output, never a gate. An error-severity finding makes the strip red; it never suppresses
  the viewer.
- **serve renders to memory** (holds the rendered `[]byte`, serves from RAM); it does
  **not** write `viewer/index.html` or `.catalog.json` per request (those are truncating
  `os.WriteFile`, readable half-written, and would race the watcher). `dossierx render`/
  `check` remain the only disk writers.
- `DetectStale`→`SaveClaim` (`review_pending` persistence) runs **report-only** on a bare
  page load; it persists only at startup and inside the (lock-held) mutation path.
- `implink.Scan` (a full `source_dirs` walk + per-tag artifact mutation) does **not** run
  per page load — startup-only or behind `serve --scan`.
- The only genuine `GET /` failure is a `render.Render`/template-override parse error →
  500 with a minimal self-contained HTML error page (not a bare string dump) + stderr.
  `/api/*` internal failures return JSON `{"error":...}`.

### In-process serialization

All pipeline runs go through one serialized owner (single worker goroutine / mutex):
at most one run in flight plus one queued; concurrent requests share the result
(single-flight). Debounce fsnotify + mutation events into one run.

### Endpoints

- `GET /` — render from current claims (see above).
- `GET /api/ping` — `application/json {"dossierx":"serve","version":"<v>"}` (probe target).
- `POST /api/claims/{id}/comments` — add thread.
- `POST /api/claims/{id}/comments/{tid}/replies` — reply.
- `POST /api/claims/{id}/comments/{tid}/resolve`, `.../reopen`.
- `PATCH /api/claims/{id}/comments/{tid}[?reply=<reply-id>]` — edit body.
- `DELETE /api/claims/{id}/comments/{tid}[?reply=<reply-id>]` — delete thread or one reply.
- `GET /api/comments?open=1` — list.
- `GET /api/status` — structured check result for the strip (see Status strip).
- `GET /api/events` — SSE live-reload stream.

Mutating handlers call `internal/comments`. Responses carry both `body` (raw, for
edit-textarea prefill) and `body_html` (server-rendered — see escaping contract). Errors
return JSON; the UI shows a toast and rolls back the optimistic update. Unknown thread/
reply id → `404 {"error":"thread_not_found"|"reply_not_found"}`.

### Client-side escaping contract (XSS)

The safe markdown renderer (`internal/render/markdown`) is Go-only; there is no JS
markdown implementation and none will be written. Therefore:
- The server is the **only** markdown renderer. Every JSON comment payload carries
  `body_html` produced by `markdown.Render`.
- **`innerHTML` may be assigned exactly two things:** a `body_html` field from the API, or
  the server-rendered fragment the SSE handler pulls. Everything else — the optimistic
  placeholder from the textarea, author roles, ids, timestamps, toast/error text — uses
  `textContent`. The optimistic placeholder is deliberately plain text and is replaced by
  the server's `body_html` when the POST resolves (markdown "snaps in" a moment later).
- **CSP** on `GET /`: `default-src 'none'; style-src 'unsafe-inline'; script-src
  'unsafe-inline'; connect-src 'self'` — blocks the exfiltration half of any injection
  even under the inline-script requirement of the existing IIFE.
- Threat model: the interesting attacker is not the human composer (self-XSS on
  localhost) but an **agent-authored or repo-inherited** comment body (`comment add` is
  agent-facing; a cloned `claims/` tree can arrive with bodies in it). Because the serve
  API is unauthenticated and same-origin, injected script could call `.../resolve` on
  every open thread and silently defeat the lock gate this feature exists to enforce.

### SSE hub contract

- **Lifecycle:** `hub.Subscribe() (<-chan struct{}, func())`; handler does `defer unsub()`
  and selects on `r.Context().Done()`. The hub never holds a channel whose handler
  returned.
- **Coalescing:** each subscriber gets a **capacity-1** channel carrying a bare signal
  (not an event queue); the broadcaster does a non-blocking send and drops on full
  (`changed` is idempotent). This is what makes "doesn't block mutations" a mechanism.
- **Timeouts:** `http.Server{WriteTimeout: 0}` (or per-response `SetWriteDeadline`) so
  server hardening does not silently kill the stream; `ReadHeaderTimeout` still set;
  keep-alive comment frames for idle streams.
- **Client:** `EventSource` with reconnect; close on `pagehide`/hidden, reopen on visible
  (bounds the HTTP/1.1 6-connections-per-origin limit with several tabs open).

### fsnotify watcher contract

- **Recursive:** fsnotify's `Add` is not recursive but `LoadClaims` walks recursively and
  FORMAT.md recommends `claims_dir/<module>/<facet>/<slug>.yaml`. Walk the tree at startup
  adding every dir; on a `Create` of a dir, `Add` + re-scan it; on `Remove`/`Rename` of a
  dir, drop the watch. Skip dot-dirs.
- **Atomic-save aware:** `atomicWriteFile` writes `<name>.yaml.tmp-*` then renames, so the
  target never gets a `Write`. Treat `Create|Write|Rename|Remove` uniformly as "changed";
  never filter on `Write` alone. Match `.yaml`/`.yml` and **exclude `*.tmp-*`**.
- **Debounce + single-flight:** ~200 ms trailing debounce → at most one `changed`; a
  comment mutation emits `changed` once (its own write also raises a watcher event —
  the debounce collapses the two).
- **Guardrail:** assert at startup that `renderOutPath`/`catalogPath`/`storePath` are
  outside the watched tree (only reachable when `claims_dir: "."`).
- **Dependency:** `fsnotify` is a **new** third-party dep — `go.mod` today is cobra +
  yaml.v3 only. Add it (Phase 4) and update the site's `badges: ["cobra + yaml.v3 only"]`
  string. If the two-dep posture is a selling point, the alternative (a ~500 ms mtime poll
  over the recursive walk `LoadClaims` already does, zero new deps) must be weighed and
  the rejection recorded.

### Status strip plumbing

`render.Render(cat, cfg)` has no parameter for check results and `shell.html` is parsed
with **no** `.Funcs`. Extract the pipeline into a value-returning API
`internal/check.Run(claims, cfg) (check.Result, error)` carrying `LintErrors`,
`LintWarnings`, `OpenComments map[string]int`, `NextSteps []string` — the `check` command
formats it for the terminal, `serve` reuses it. Feed the strip **client-side** via
`GET /api/status` on probe + each SSE `changed` (keeps `Render` untouched; consistent with
"controls only when API reachable"). **Static `dossierx render` output has no status
strip** (it would be permanently stale).

### file:// and the reachability probe

Every render/check/comment command prints an absolute HTML path to copy into a browser,
so `file://` is a first-class read-only path. Contract:
- Client probe: relative `fetch('/api/ping')`, `AbortController` ~1 s timeout, `try/catch`
  around the whole thing; success requires `res.ok` **and** JSON content-type **and**
  `body.dossierx === "serve"`. On `file://` the fetch rejects; the rejection is caught, no
  error surfaces, and the composer/resolve controls simply never mount. The probe is
  self-contained (its rejection must not abort the rest of the IIFE init).
- The static `file://` viewer is read-only **by design**; its probe failing is the
  correct, intended outcome. For interactive review the human opens the serve URL. Do not
  "fix" the failing probe with `Access-Control-Allow-Origin: *` — that undoes the
  admission middleware.

## Viewer UI

Viewer theme tokens only (`--accent, --card-bg, --border, --muted, --warn, --radius,
--font-sans`), dark + light via the existing `color-scheme` setup. Overridable via
`viewer.template_overrides`.

- Claim card footer: 💬 chip + open-thread count (accent when open, muted when resolved).
- Click → comments panel. Desktop: non-modal right rail `role="complementary"`
  `min(380px, 90vw)`. Mobile: modal bottom sheet, `role="dialog" aria-modal="true"`,
  max-height 70vh internal scroll.
- Thread: author-role pill, relative timestamp (absolute on hover), markdown body,
  replies indented one level, reply composer.
- Composer: textarea (auto-grows) + button; actor fixed to `human` in browser.
- Thread icon actions top-right: ✓ resolve, ✎ edit, ✕ delete (`--warn` hover, confirm for
  whole-thread delete). Edit/delete/reopen only on own-role messages; edited messages
  show "(edited)".
- Cards with open threads: 3px accent left border, `claim-card--commented` (distinct from
  the `--warn` boundary family). For `tree` layout (no `.card` class) use a dedicated
  `.claim-tree.claim-card--commented` rule.
- ≥44px touch targets on coarse pointers; no horizontal page scroll at any width.

### DOM-duplication constraint (overview claims render N times)

`buildGroups` injects a module's overview-facet HTML into **every** facet group, so an
overview claim's `id="..."` node appears N times, all but one inside a `hidden`
`.claim-group`. Therefore `comments.html` **must emit zero `id` attributes** — identify
via `data-claim-id` / `data-thread-id`; use `<details>` (needs no id) for the "N resolved"
collapse. Comment JS must **never** `getElementById(claimID)` (returns the first, usually
hidden, copy); use `querySelectorAll('[data-claim-id="…"]')` (attribute selectors tolerate
the dots in claim ids) and act on `e.target.closest('.claim')`. All state fan-out
(chip-count, `claim-card--commented`, `review_pending` pill) iterates every matching node.

### Footer injection points (per layout — v1's "card/table/banner" list is wrong)

`edges` is called by card, table, list, steps, tree, mockup — **not banner** (banner has
no footer). Decision: **exclude banner claims from commenting** — `comments_unresolved`
skips `LayoutBanner` and the CLI/API reject `comment add` on a banner claim (otherwise a
module could be locked out by a thread the viewer can't render). `build_order.html` is not
claim-scoped → no chip. The chip reaches the footer by **extending the one existing
`attachEdgesOverride` closure** (`depended_by_view.go`); **no new `tmpl.Funcs` may bind
`edges`** (a second binding silently discards the first override). Widen
`attachEdgesOverride`'s empty-lookup early-return to include the comments lookup. A render
test must assert the chip appears for every `model.Layout` value so the banner decision
can't silently regress.

### SSE in-place re-render (budgeted shell.html refactor — Phase 5)

Every `.module-section`/`.claim-group` ships `hidden`; only `showFromHash()` at load
unhides them, and the IIFE caches static NodeLists + lookup maps and binds per-element
listeners, exporting nothing. A naive markup swap yields a blank page + dead tabs.
Required:
- Add `GET /api/fragment` (or `fetch('/') → DOMParser`) returning **both** the
  `<main class="content-area">` and `<nav id="nav">` subtrees (the sidebar 🔒 lives
  outside `.content-area`).
- Extract an **idempotent `initViewer()`** that re-queries sections/tabs and re-derives
  `facetToModule`/`moduleDefaultFacet`/`claimToFacet`/`firstModuleID` on every call.
- **Delegated** listeners on a surviving node (`document`/`.layout`), not per-element.
- Separate "restore view" from "deep-link jump": capture `content-area.scrollTop` + open
  thread/claim id before the swap; re-apply module/facet from the hash **without** the
  `scrollIntoView`+highlight block; restore `scrollTop` and re-open the panel by thread
  id. (Re-running the deep-link path on every SSE tick would yank the viewport — the
  opposite of "preserving scroll".)
- Override degradation: if a project ships its own `shell.html` with no SSE handler,
  `serve` degrades to read-only + a terminal warning, not silent breakage.

### Accessibility & overlay ownership

The repo already uses `aria-controls`/`aria-expanded`/`aria-label`, so bare glyph buttons
are a regression, not a greenfield gap.
- Icon buttons get text `aria-label` + `aria-hidden="true"` glyph; the 💬 chip gets
  `aria-expanded`/`aria-controls` and a count-bearing label.
- One Escape handler: **modify the existing** unconditional `window` keydown listener in
  place (`if commentPanelOpen() { closeCommentPanel(); return } setDrawer(false)`) — a
  second listener can't `stopPropagation` it.
- Own overlay: a new `#commentsOverlay` + `body.comments-open` class with its own
  `overflow:hidden` lock; do **not** reuse `body.nav-open` (it drives the sidebar
  transform and `#navOverlay`'s hardcoded `setDrawer(false)`). Opening one overlay closes
  the other.
- Render test asserts every comment control has a non-empty accessible name and
  decorative glyphs are `aria-hidden`.

## Error handling

- Claim/thread/reply not found, rights violation, wrong actor, lock-timeout, 409 conflict
  → structured JSON error → UI toast, optimistic mutation rolled back.
- Concurrency: the claims sentinel serializes all claim-file writers, each re-reading
  inside the lock (see Concurrency). `-race` is necessary but not sufficient — the
  dangerous races are cross-process TOCTOU; assert on file content.

## Testing — end-to-end coverage including edge cases

TDD throughout. The risky semantics (lock gate, pending-trigger arithmetic, reply-id
addressing, claim-file locking) get their tests written **first**.

**0. Concurrency / write-discipline (Phase 0)** — see the Phase 0 test list above.

**1. Unit — `internal/comments` (table-driven)**
- Happy paths for all seven ops on draft + locked claims.
- Rights matrix: every (operation × actor × target-author) incl. **reopen**; expect
  denial for agent-resolve/reopen/edit/delete of a human message.
- Pending-trigger arithmetic: open-on-locked sets `review_pending`; resolve/delete-last
  clears it iff no drift and no flag; reopen re-sets; the `flag → unlock → lock → comment
  → resolve-last` stale-flag case.
- Edge cases: unknown claim/thread/**reply** id → structured not-found, no neighbour
  mutated; reply to a resolved thread (rejected); double-resolve; reopen an open thread;
  empty body rejected; huge body; **YAML-hostile bodies round-trip byte-exact** (property/
  fuzz); id collision → regenerate (threads and replies); id-less legacy comment →
  backfilled; claim with no `comments:` field; read-only claim file → clean error, no
  partial write; two edits to the same reply body → last-write-wins (stated as accepted).

**2. Model / persistence**
- YAML round-trip identical (threads + replies + ids); `omitempty` keeps comment-free
  claims byte-identical.
- `ContentHash` unchanged by any comment op; a dependent locked claim does not flip.
- Strict-decode rejects a misspelled comment field.

**3. Lint / lock / reaudit / buildorder integration**
- `comments_unresolved` is **warning**, exit 0, on both draft and locked claims;
  `dossierx check` on a locked claim with an open thread **exits 0, still writes
  `.catalog.json` + `index.html`, and prints the "resolve N open threads" next-step**.
- `lock.Lock` refuses when the **candidate** has an open thread and names the ids;
  `lock B` **succeeds** while unrelated locked claim A has an open thread.
- Full loop: add → `lock` refused → reply → resolve → lock succeeds; locked → comment →
  `review_pending` → `stale` lists it → resolve → cleared.
- `reaudit --confirm` on a comment-only pending claim → exit 2, claim file byte-identical
  (no `audit_notes` growth, no hash/`LockedAt` mutation); on a drift+comment claim →
  drift cleared, `review_pending` stays true, still in `stale`.
- `check` next-steps emits the comment hint (not the reaudit hint) for comment-only.
- `buildorder.Propose` on a module with one commented claim → refused, names the id →
  resolve → succeeds.
- Lint coverage fixture: add `testdata/fixture-coverage/lint/comments-unresolved/`
  (dir name == rule `Name()` == `"comments-unresolved"`, a single **draft** claim with one
  open thread, lint-clean otherwise); bump `lint_fixtures_test.go` count 21→22, add
  `"comments-unresolved": 0` to the expected-exit map (warning → exit 0), bump the
  `internal/lint/lint.go` package-doc count.

**4. Serve (`httptest`)**
- Admission matrix, each asserting the claim YAML is byte-unchanged after a rejected
  request: rebinding `Host: evil.com` rejected on GET /, /api/comments, /api/events, and
  every mutating endpoint; `Origin: null` rejected; absent Origin rejected; cross-origin
  rejected on POST, PATCH **and** DELETE; `Content-Type: text/plain` rejected;
  `Sec-Fetch-Site: cross-site` rejected; no `Access-Control-Allow-Origin` on any response;
  happy path (allowed Host + Origin + `application/json`) accepted; `GET /api/ping` shape.
- `GET /` returns 200 **with composer markup and a red status strip** when the project has
  a duplicate id / dangling ref / an unlocked claim tagged in source (the exact
  comment-on-locked-claim loop) — never a blank page.
- XSS: POST a `<img src=x onerror=…>` body → the response `body_html` and
  `GET /api/comments` return `&lt;img`, never live markup.
- SSE: one mutation → exactly one delivered `changed` after debounce; external nested-dir
  write via the rename path → `changed`; a `.tmp-*` file appearing/vanishing → no event;
  subscriber count returns to zero after client cancel (assert hub size — `-race` won't
  catch the leak); a never-reading subscriber does not delay a POST.
- Concurrency: N concurrent `GET /` + a concurrent `POST` under `-race` → every body a
  complete document, the comment survives, the pipeline ran fewer times than the request
  count (coalescing).

**5. Render / viewer**
- Static render: threads baked in; counts correct in **both** facet groups for a commented
  overview claim; resolved collapsed; `claim-card--commented` only on open-thread cards;
  chip present for **every** `model.Layout` value (banner explicitly excluded); comment
  markup contributes **zero `id=` attributes**; body markdown rendered AND HTML-escaped
  (script-tag fixture inert); no composer markup in static output; every control has a
  non-empty accessible name, decorative glyphs `aria-hidden`.
- Viewer JS interactivity (needs a headless-browser / JS harness the repo lacks today —
  **budget it in Phase 5**): after a `changed` event the active section is not `hidden`,
  the active facet is visible, a subtab click still switches modules, `scrollTop` is
  unchanged, a newly added claim id resolves via `claimToFacet`; a chip click on facet 2
  opens the visible card's thread.

Definition of done: all layers green, `-race` clean, plus one scripted CI end-to-end
walkthrough (serve started, comment added via HTTP with an explicit allowed Origin,
resolved via CLI, lock succeeds) runnable headlessly.

## Out of scope (v1)

Text-range anchors; auth / multi-user identity; comment mentions/notifications;
preserving `#` comments / key order / block-scalar style through `SaveClaim` (pre-existing
whole-struct rewrite behavior — documented, not changed).

## Agent skills

New embedded skill `skills/dossierx-comments/SKILL.md` (fourth bundle, added to
`skills/embed.go` go:embed list, exported by `dossierx skills export`):
- Comment vs flag rule: `flag` only when a locked claim's stated meaning drifted from
  reality (feeds reaudit, carries a proposed content edit); a comment for any discussion/
  remark needing human dialogue (no content edit). State the boundary crisply — both
  signal "needs human attention", so the discriminator is "is there a specific proposed
  wording change?".
- Rights discipline (advisory): agent never resolves/reopens human-opened threads —
  replies "addressed, please confirm" and waits; resolve/edit/delete/reopen only own
  messages.
- Pending-trigger + lock-gate semantics for comments; reaudit refuses on comment-only
  pending.
- Loop: `dossierx check` next-steps drive which threads to address before locking; always
  mutate via CLI (claims lock) since a human may be live in serve.

Targeted updates to existing skills (same change): `dossierx-claims` (three triggers/
clearers, lock-gate rule, cross-link), `dossierx-build-order` (completeness gate now
enforced in `Propose` on open threads), `dossierx-code-links` (flag vs comment pointer).
Note `dossierx skills export` consumers who exported the old three bundles need to
re-export — call this out in the changelog.

## Phase-wise implementation outline

Dependency-ordered; each phase lands with tests green and leaves `main` shippable.

0. **Claim-file write discipline** — one project-wide claims sentinel; retrofit every
   claim-file writer (lock/unlock/check/flag/reaudit) to acquire-then-reload;
   `SaveClaimIfUnchanged`; concurrency tests. (No user-visible feature yet — pure
   correctness foundation the rest depends on.)
1. **Data model & core ops** — `Comment`/`Reply` structs (with ids + `edited`), shared
   id generator, round-trip + ContentHash-exclusion + hostile-body fuzz tests,
   `internal/comments` ops + rights matrix + pending-trigger predicate, FORMAT.md schema
   block, `review_pending` contract-doc updates.
2. **CLI + lock gate + buildorder gate** — `dossierx comment` group (with `--as`, id
   echo, `list` format, `--json`), `comments_unresolved` **warning** lint + coverage
   fixture, `lock.Lock` candidate-scoped refusal, reaudit refusal for comment-only,
   `buildorder.Propose` open-thread gate, `check` summary + trigger-partitioned
   next-steps, reconciler.
3. **Static rendering** — `components/comments.html` (zero ids, `data-*`), chip/panel CSS,
   `claim-card--commented`, per-layout footer wiring via `attachEdgesOverride` (banner
   excluded), render tests incl. overview-duplication + accessibility.
4. **Serve** — server + admission middleware + random port, `internal/check.Run` value
   API, memory render, JSON API (id-addressed, `body_html`), CSP, SSE hub, fsnotify
   contract (+ `go.mod` fsnotify), single-flight, httptest coverage.
5. **Viewer interactivity** — API probe, thread panel, composer, ✓ ✎ ✕ actions,
   optimistic updates/toasts (client-side escaping contract), idempotent `initViewer()` +
   delegated listeners + restore-view SSE reload, accessibility, headless-JS test harness.
6. **Skills & docs** — `dossierx-comments` skill + embed, updates to the three skills,
   FORMAT.md / README / lock package-doc / flagstore-doc lifecycle prose, changelog
   (incl. re-export note).
7. **Website** — see Website updates.

## Website updates

The marketing site (`site/`, Vite + React + TS) is composed from `contentSpec` in
`content.ts` (typed nav + section entries), one component per section under
`site/src/sections/`, assembled in `App.tsx`. Exact changes:

- `site/src/sections/Comments.tsx` — new section (id `comments`, "Review with comments")
  from `SectionContainer`/`SectionHeader`/`AnimatedReveal` + `motion-tokens`: (a) animated
  six-step workflow with human/agent/engine role pills; (b) UI vignette — React recreation
  of card + thread panel (no screenshots); (c) terminal vignette — `dossierx serve` /
  `comment list --open` in `CodeBlock`.
- `site/src/content.ts` — new section kind `"comments-workflow"` in the kind union
  (**check every exhaustive switch/map over the union renders it** — not just App.tsx);
  new `sections` entry between `lifecycle` and `build-order`; nav item + scroll-spy;
  hero body + summary extended; **"Three embedded skills" → "Four"**; the three false
  `review_pending` strings at `:184,:261,:271`; the `badges: ["cobra + yaml.v3 only"]`
  string (fsnotify is a new dep).
- `site/src/App.tsx` — mount `<Comments />` between `<Lifecycle />` and `<BuildOrder />`.
- `site/src/sections/Lifecycle.tsx` (+ content) — diagram gains the third `review_pending`
  trigger (open thread) and a review_pending→locked "comment resolve" outbound edge;
  verify `LifecycleDiagram` renders a second outbound edge (today only one exists).
- `site/src/sections/Cli.tsx` (+ `cli-explorer` content) — `comment` verbs + `serve`.
- `site/src/sections/Versions.tsx` + `pages/ReleasesPage.tsx` / `releases.html` — release
  entry + changelog cross-link, version bumps.
- `site/src/sections/Compare.tsx` — comments-vs-wiki/ADR row.
- `site/index.html` — meta description + version.

## FORMAT.md / docs impact

- Document `comments:` (incl. ids and `edited`) in the claim schema block.
- Rewrite (not append) the lock-lifecycle section: three `review_pending` triggers
  (dependency drift; `dossierx flag`; open comment thread on a locked claim) and three
  clearers (`reaudit --confirm`; `unlock`; resolving/deleting the last open thread when no
  other trigger stands); "a claim cannot lock while it has an unresolved comment thread".
- Changelog entry (incl. the skills re-export note and the new `fsnotify` dependency).

## Residual risk (after all fixes)

- **Six audit dimensions were raised but their verification died on a session limit**
  (data-model, website, skills-docs, tests, phasing, architecture-devil) — their findings
  are NOT in the confirmed 33 and were dropped unverified. Re-run that half of the audit
  after the limit resets before treating the spec as fully vetted.
- **Browser JS has no test harness in the repo today.** Phase 5 (viewer interactivity) is
  the least-testable, highest-risk area; the headless-JS harness is new scope.
- **Whole-struct `SaveClaim` + YAML round-trip fidelity** of arbitrary user bodies is a
  standing risk mitigated but not eliminated by the Phase 1 fuzz test.
- **Advisory rights** (no auth) mean a determined agent can assert `--as human`; the model
  is a coordination convention, not a security boundary. Acceptable for a local
  single-user tool; stated so no one mistakes it for enforcement.
