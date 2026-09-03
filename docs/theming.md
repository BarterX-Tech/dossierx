# Theming the viewer

This is a worked guide to `viewer.theme`. For the full token table, the
validation rules, and how `check`/`serve` apply them, read
[FORMAT.md](../FORMAT.md#viewertheme). For the twenty-minute version aimed at
an agent, read [`skills/dossierx-theme/SKILL.md`](../skills/dossierx-theme/SKILL.md).

Everything below assumes a project with a `project.config.yaml` already in
place. Every fenced example is a complete `viewer.theme` block you can paste
in as-is.

## Change one colour

The smallest possible theme sets a single token. This changes the accent
colour — locked-state highlights and active tabs — in both light and dark
mode, because a flat key applies to both:

```yaml
viewer:
  theme:
    accent: '#C6613F'
```

Run `dossierx check` and open `build/viewer/index.html`. The accent colour is the
only thing that changed; every other token still renders the engine's own
default.

**Quote every colour (a bare #hex is a YAML comment).** A bare
`accent: #C6613F` is, to YAML, the key `accent` with a trailing comment, and
therefore a *null* value. The decoder catches this and says so, but quoting
from the start costs nothing.

Every value is also checked for stray whitespace: any Unicode space other
than a plain U+0020 (a non-breaking space, for instance) is refused, and so
is a leading or trailing space — `accent: ' #C6613F'` is refused with
`value " #C6613F" has leading or trailing whitespace; write it as
"#C6613F"`, naming the exact fix.

## Light and dark, and the trap: a flat colour key pins both modes

Fourteen of the twenty-eight tokens have a different default depending on
the reader's OS colour scheme — `paper`, `card-bg`, `ink`, `muted`, `faint`,
`border`, `link`, `accent`, `accent-bg`, `warn`, `warn-bg`, `shadow`,
`shadow-strong`, and `scrim`. Setting one of these as a flat key pins it to
that value in **both** schemes. That is sometimes exactly what you want, and
nothing warns you when it is not:

```yaml
viewer:
  theme:
    paper: '#FAF9F5'          # legal, and it gives a dark-mode reader a
                               # near-white page — probably not the goal
```

To set different values per scheme, use `light:` and `dark:` instead:

```yaml
viewer:
  theme:
    light:
      paper: '#FAF9F5'
    dark:
      paper: '#151515'
```

Setting only `dark:` for a token is legal; the light default survives
unchanged. The reverse is also true. **Printing always uses the light
values**, regardless of the reader's OS setting — see "What a theme cannot
do" below.

Two more tokens vary between schemes without being re-declared in the dark
block: `code-inline-bg` and `code-bg` default to
`color-mix(in srgb, var(--paper) 72%|82%, var(--card-bg))`, an expression
over `paper` and `card-bg` rather than a fixed colour, so their *computed*
value is darker in dark mode even though the engine declares them once.
Setting either flat freezes that expression's current result — a light
tint baked permanently onto a dark-mode reader's page — which is exactly
the trap above. Treat them as mode-varying: set them under `light:`/`dark:`
if you override them at all, or leave them alone and let them keep tracking
`paper`/`card-bg`.

The other twelve tokens — `font-sans`, `font-mono`, `radius`, and nine more
added alongside the per-mode work (`table-head-bg`, `image-bg`, `hover-bg`,
`border-strong`, `shadow-cast`, `selection-bg`, `status-draft`,
`status-draft-bg`, `mockup-bg`) render the same value in both schemes by
design. Setting these flat is exactly right; there is no light/dark variant
to accidentally collapse.

## Start from the `claude` preset

`preset: claude` sets every colour token plus `font-sans`/`font-mono`
(system-font stacks — a preset ships no font files) and `radius`, with no
file to maintain:

```yaml
viewer:
  theme:
    preset: claude
```

`dossierx theme list` reports every built-in preset and the tokens it sets.
Preset values may change between minor releases — they track a palette
DossierX does not own — and every change gets a CHANGELOG "Changed" entry.
A project that needs a value frozen writes it inline, where it always wins
over the preset:

```yaml
viewer:
  theme:
    preset: claude
    accent: '#2F6FCB'          # frozen, regardless of what the preset does next
```

The claims graph's facet colour ramp is generated to stay distinguishable
and does not follow a preset or any theme token.

## Export, edit, extends

When a preset is the right *starting point* but you want to diverge from
more than one or two tokens, export it to a file, edit the file, and point
`extends` at it:

```
dossierx theme export claude themes/mine.yaml
```

This writes the whole `claude` palette as ordinary, editable YAML. Open
`themes/mine.yaml` and change whatever you like — it is a plain
`viewer.theme`-shaped file, minus `preset` and `extends` (a theme file
cannot chain to another one). Then:

```yaml
viewer:
  theme:
    extends: themes/mine.yaml
    accent: '#2F6FCB'          # inline keys still beat the extended file
```

`extends` is resolved relative to `project.config.yaml` and must stay under
the project directory — `../shared/theme.yaml` outside the project is
refused. `theme export` refuses to overwrite an existing file
(`write_conflict`) unless you pass `--force`, because the exported file is
the one a human is expected to edit by hand. With no path argument, the
YAML comes back in the envelope's `data.yaml` instead of being written.

## Add fonts, and their cost

A theme can inline the project's own local font files as base64 `data:`
URLs. There is no network fetch, ever — the viewer stays one self-contained
file:

```yaml
viewer:
  theme:
    font-sans: '"Inter Variable", -apple-system, sans-serif'
    fonts:
      - family: Inter Variable
        src: fonts/InterVariable.woff2
        weight: "100 900"
        style: normal
```

Limits, all enforced at load time rather than as a silent fallback:

- `src` must end in `.woff2`, `.woff`, `.ttf`, or `.otf`, and its bytes must
  match that extension's file signature. A renamed or truncated file is
  refused outright — a browser handed one drops the face silently and
  renders a fallback nobody chose.
- `src` is resolved relative to the file that declares it: relative to
  `project.config.yaml` for a font declared inline, but relative to the
  theme file itself for a font declared inside one reached through
  `extends` — a `themes/` directory that carries its own `fonts/` stays
  movable as a unit.
- Every declared `family` must appear in the merged `font-sans` or
  `font-mono` value, or the theme is refused: a face nothing names is
  downloaded by every reader and rendered by none.
- Total raw font bytes across every face are capped at 2 MiB — roughly four
  generous variable faces. Base64 adds about a third on top of that in the
  emitted HTML. Going over the cap is a load-time error, never a silent
  drop of the excess. The error names only the files read up to and
  including the one that tipped the total over the cap, not every font the
  theme declares — it does not read the rest just to list them.

Read `data.theme_font_count` and `data.theme_font_bytes` from
`dossierx check`'s envelope to see exactly what a reader downloads before
the page renders anything. Both fields are `omitempty`: they are **absent
from `data`**, not present as `0`, when the theme declares no fonts. A
theme that fails to resolve at the RENDER phase (see the `stopped_at` table
below) carries `data.theme_error` naming what went wrong, under `check`,
`--validate`, and `--staged` alike. A theme that fails at the CONFIG phase
carries no `data` at all — the config loader refuses before any check
result exists — so there is no `theme_error` to read; `error.message` is
the only place that failure is named.

## Verify it applied, in both OS modes

A theme that resolves is not the same as a theme that looks right. Work
through this in order:

1. Run `dossierx check`. A theme that does not resolve fails here, always as
   `invalid_config`, but `stopped_at` names where the failure was caught:

   | what failed | `stopped_at` | `data.theme_error` |
   |---|---|---|
   | a grammar/allowlist/shape problem in the inline `viewer.theme` block itself — an unknown token, a malformed colour, length or font-family value, a control character, stray whitespace, or an `extends` path that resolves outside the project | `config` | absent — no `data` exists yet; only `error.message` names it |
   | an unknown `preset` name, a missing or unstaged `extends` file (including a grammar problem inside that file's own content), a font that does not exist, fails its signature check, or blows the 2 MiB cap, or a font family nothing names | `render` | present, naming the failure |

   `check --validate` and `check --staged` apply the identical rule set, so
   a hook or CI run is not a way around either failure.
2. Open the rendered `build/viewer/index.html` and confirm the colour you set is
   the colour you see. Every one of the twenty-eight tokens has an engine
   default, so a typo'd value is a load error, but a *correct value in the
   wrong layer* (set under `light:` when you meant flat, say) renders as
   the untouched default with no complaint.
3. Switch your OS to the other colour scheme and reload. This is the only
   way to catch the flat-key trap described above.
4. Read `data.theme_font_count` and `data.theme_font_bytes` from `check`'s
   envelope if the theme declares fonts — they are absent from `data`
   entirely when there are none. If the theme fails at the render phase,
   look at `data.theme_error` instead (present under `check`, `--validate`,
   and `--staged` alike); a config-phase failure carries no `data` at all,
   so `error.message` is where that one is named.

## `check`, `--validate`, `--staged`

`dossierx check`, `dossierx check --validate`, and `dossierx check --staged`
run the identical theme rule set through one validation function with a
different byte source. `--staged` reads the theme file named by `extends`,
and every font it names, from the **git index** rather than the working
tree — so an edited-but-unstaged theme file or font is judged by what the
commit will actually carry, not by what happens to be on disk. A theme
file or font that is not staged at all names itself in the error and tells
you to `git add` it.

`family-consistency` — the "every font family must be named by font-sans or
font-mono" rule above — is **skipped when `viewer.template_overrides` is
set**, because an override stylesheet may reference the family itself and
this check has no way to read that sheet.

## Restart `serve` after a theme or font change

`dossierx serve` resolves the theme once, at startup. It does not watch
`project.config.yaml`, a theme file, or a font file for changes — after
editing any of them, stop and restart `dossierx serve` to see the result.

## What a theme cannot do

- **No raw CSS.** The twenty-eight tokens in FORMAT.md's table are the
  whole vocabulary. There is no CSS injection point, and asking for one
  every so often does not create one.
- **No in-page toggle.** Light or dark follows the reader's OS setting.
  There is no in-viewer switch to add, and no token controls which mode is
  active.
- **No graph ramp.** The claims graph's facet colours are generated to stay
  distinguishable from each other; no preset or token repaints them.
- **The colour grammar is a shape check, not a CSS parser.** A value like
  `rgb(1)` passes validation — it has the right shape — and is then
  dropped by the browser as an invalid declaration, silently leaving the
  reader on the engine default. Passing `dossierx check` proves the value
  is *well-formed*, not that it is a colour a browser will actually paint.
  Always look at the rendered page.
- **`viewer.template_overrides` is a replacement, not a layer.** An
  override `style.css` replaces the engine's stylesheet wholesale; it must
  declare or consume the theme tokens itself, or theming has no effect on
  it. An override `shell.html` that omits `{{.ThemeCSS}}` gets no theme
  and no fonts at all — this is a deliberate bound on what an override can
  drop, not a bug.
- **Print always uses the light palette.** A project's `dark:` values never
  reach a printed page, even for a token declared only under `dark:`. This
  is deliberate: `light:`/flat values apply to print, `dark:` values are
  scoped to screen only.
