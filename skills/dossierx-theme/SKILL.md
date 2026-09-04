---
name: dossierx-theme
description: >-
  Restyling the DossierX viewer with a project's own colours and fonts through
  viewer.theme. Use this WHENEVER a human asks for the viewer to match their
  product or brand, mentions colours, dark mode, fonts or "it looks generic",
  or when dossierx check refuses with a viewer.theme message. Covers the
  twenty-eight-token vocabulary and its light/dark defaults, flat vs. per-mode
  keys and the dark-mode trap in setting a colour flat, the built-in presets,
  theme files and extends, dossierx theme list/export, project-supplied fonts
  and their 2 MiB budget, how to verify a theme actually applied, and the five
  things a theme deliberately cannot do. Load the DossierX router skill first.
---

# DossierX theming — the viewer's colours and fonts

Read **[`dossierx`](../dossierx/SKILL.md)** for the envelope and error codes first.

A theme is CSS custom-property values the engine injects into the viewer. It changes **how the
document looks and nothing about what it says** — no claim, lock, gate or approval, and no lint
rule reads it — so restyling is fully yours. The human does **not** see it on a reload, though:
`serve` resolves the theme once at startup, so a change lands only after a re-render *and* a restart.

**Reach for this** when the human wants the viewer to match their product, asks about dark mode, or
wants their own typeface. **Not** for a layout complaint or to make one claim stand out: there is
no per-claim styling, and "this claim is hard to find" is answered by the corpus.

## The shape

Everything lives under `viewer.theme` in `project.config.yaml`:

```yaml
viewer:
  theme:
    preset: claude
    accent: '#C6613F'
    radius: 10px
    light:
      paper: '#FAF9F5'
    dark:
      paper: '#151515'
```

Five keys are structure — `preset`, `extends`, `light`, `dark`, `fonts`; **every other key is a
token name** from the twenty-eight below, and a typo is a load-time error naming the whole
vocabulary rather than a silently ignored key.

**Quote every colour.** A bare `accent: #C6613F` is, to YAML, the key `accent` with a comment and
therefore a *null* value — the commonest mistake here, and the decoder says so. Lengths and font
stacks need no quotes (`radius: 10px` above is fine); `dossierx theme export` quotes everything
anyway.

Layers, lowest first: **preset → `extends` file → inline keys**. Within one layer, flat keys apply
to both schemes and `light:`/`dark:` to one — and **`light:`/`dark:` beats the same layer's flat
value** for that token, so flat `accent` plus `light: {accent: …}` leaves the flat one dark-only.

A **theme file** is that block's contents unwrapped: token keys at the top level plus `light:`,
`dark:`, `fonts:`. No `viewer:`/`theme:` nesting; neither `preset:` nor `extends:` is allowed
inside one — themes do not chain, and the preset is named in `project.config.yaml`.

## The tokens

Twenty-eight, and there are no others. Defaults are the engine's own; the rows whose two columns
**differ** are the ones that matter — see the trap below.

| token | light default | dark default | what it paints |
|---|---|---|---|
| `paper` | `#f6f8fc` | `#0a1220` | the page behind everything |
| `card-bg` | `#ffffff` | `#0f1b2e` | claim cards, panels, the sidebar |
| `ink` | `#091426` | `#e8eef8` | body text |
| `muted` | `#536179` | `#a9b5c8` | secondary text, metadata |
| `faint` | `#7d899a` | `#75839a` | the quietest labels |
| `border` | `#d8deea` | `#263754` | ordinary rules and card edges |
| `border-strong` | `#aab5c7` | *(same)* | emphasised edges |
| `accent` | `#287052` | `#70c99c` | the brand colour: locked state, active tabs |
| `accent-bg` | `rgba(40, 112, 82, .12)` | `rgba(112, 201, 156, .12)` | the accent as a fill |
| `link` | `#205b78` | `#8ab7ff` | hyperlinks |
| `warn` | `#a2433d` | `#ff8b94` | warnings and refusals |
| `warn-bg` | `rgba(162, 67, 61, .10)` | `rgba(255, 139, 148, .10)` | the warning fill |
| `status-draft` | `#976600` | *(same)* | the draft pill's text |
| `status-draft-bg` | `rgba(151, 102, 0, .12)` | *(same)* | the draft pill's fill |
| `code-inline-bg` | `color-mix(in srgb, var(--paper) 72%, var(--card-bg))` | *(derived)* | `` `inline code` `` |
| `code-bg` | `color-mix(in srgb, var(--paper) 82%, var(--card-bg))` | *(derived)* | fenced blocks, claim trees |
| `table-head-bg` | `rgba(127, 127, 127, .10)` | *(same)* | table header rows |
| `image-bg` | `rgba(127, 127, 127, .06)` | *(same)* | the mat behind images |
| `mockup-bg` | `#fff` | *(same)* | mockup diagrams (light artwork in both modes, on purpose) |
| `hover-bg` | `rgba(125, 137, 154, .08)` | *(same)* | hover highlight on rows and tabs |
| `selection-bg` | `rgba(40, 112, 82, .20)` | *(same)* | selected text |
| `shadow` | `rgba(0, 0, 0, .08)` | `rgba(0, 0, 0, .28)` | the comments panel |
| `shadow-strong` | `rgba(0, 0, 0, .14)` | `rgba(0, 0, 0, .34)` | the toast |
| `shadow-cast` | `rgba(9, 20, 38, .12)` | *(same)* | the rail, the nav toggle, the facet ToC |
| `scrim` | `rgba(0, 0, 0, .22)` | `rgba(0, 0, 0, .42)` | the dim behind a modal |
| `font-sans` | `"Avenir Next", -apple-system, BlinkMacSystemFont, "Inter", "Segoe UI", sans-serif` | *(same)* | all body text |
| `font-mono` | `ui-monospace, "SFMono-Regular", "IBM Plex Mono", Menlo, monospace` | *(same)* | code and ids |
| `radius` | `6px` | *(same)* | every rounded corner |

**`*(derived)*` is not `*(same)*`.** `code-inline-bg` and `code-bg` are `color-mix()` over `paper`
and `card-bg`, so they follow those two in either scheme — including in a theme that only sets
`paper`. The dark block never re-points them, yet their *computed* value is darker there. A flat
override freezes that, which is the trap below: set them per-mode, or leave them and set `paper`.

Values are validated as hostile input, not trusted as CSS. Colours must be `#hex`, an
`rgb(`/`hsl(`/`oklch(`/`color-mix(`-family function, or a CSS named colour; `radius` must carry a
unit (`10px`, `0.5rem`, bare `0`); font families are comma-separated items, quoted or plain.
Refused outright: `;`, `{}`, `<>`, comment markers, control characters, leading/trailing whitespace,
and any Unicode whitespace that is not a plain space — usually a non-breaking space pasted from a
design tool.

But it is a **shape check, not a CSS parser**: `accent: 'rgb(1)'` is a well-formed call and passes,
then the browser discards the declaration and the token falls back to the engine default. Nothing
in `check` catches that, which is why step 2 below is the real evidence.

### The trap: a flat colour key pins both modes

**Fourteen of these tokens are re-declared in dark mode** — every row with a value in the dark
column — and two more, the `*(derived)*` pair, vary without being re-declared. Writing one of them flat sets it for **both** schemes:

```yaml
viewer:
  theme:
    paper: '#FAF9F5'
```

That is a legal, silent way to give every dark-mode reader a near-white page — legal because it is
sometimes right (`mockup-bg` is flat on purpose), so nothing warns. The rule:

- a token whose two defaults differ → set it under `light:` **and** `dark:`, or accept that you have
  pinned both;
- a `*(derived)*` token → treat it as differing: flat freezes an expression that was tracking the
  mode;
- a token whose dark column says *(same)*, and `font-sans`, `font-mono`, `radius` → flat is fine and
  is what you want;
- setting only `dark:` is legal and the light default survives. **Printing always uses the light
  values**, whatever the reader's OS says.

## Presets, and the export → edit → extends workflow

`dossierx theme list` reports every preset and the tokens it sets; `preset: claude` is the whole
adoption step. Two things to tell the human:

- **preset values may change between minor releases** (they track a palette this project does not
  own). Each change gets a CHANGELOG line; a project needing a value frozen writes it inline, or
  exports, where it wins.
- **the claims graph's facet ramp does not follow a preset.** It is generated to stay
  distinguishable and a preset cannot repoint it.

When the human wants the preset as a *starting point* and then diverges, export it, edit the file,
and extend it:

```
dossierx theme export claude themes/mine.yaml     # writes the whole palette as editable YAML
```

```yaml
viewer:
  theme:
    extends: themes/mine.yaml
    light:
      accent: '#2F6FCB'
    dark:
      accent: '#6DA7EC'
```

The export carries no version stamp, so re-exporting is byte-identical and a diff means the preset
moved. `extends` resolves against `project.config.yaml` and **must stay under the project
directory**; a font `src` in a theme file resolves against *that* file. `theme export` refuses to
overwrite (`write_conflict`); `--force` only on the human's say-so. With no path, YAML in `data.yaml`.

## Fonts

A theme may inline the project's **own local font files** — no network fetch, ever: the viewer is
one file and the faces are base64 `data:` URLs inside it.

```yaml
viewer:
  theme:
    font-sans: '"Inter Variable", -apple-system, sans-serif'
    fonts:
      - family: Inter Variable
        src: fonts/InterVariable.woff2
        weight: "100 900"
```

- `src` is relative to the file declaring it, extension one of `.woff2 .woff .ttf .otf`, and the
  **bytes must match the extension** — a renamed file is refused, because a browser handed one
  drops the face silently and renders a fallback nobody chose.
- `weight` is `"400"` or a range `"100 900"`; `style` is `normal` or `italic`. Both default.
- **Every declared family must appear in `font-sans` or `font-mono`**, or the theme is refused: a
  face nothing names is bytes every reader downloads and none sees. (Skipped under
  `viewer.template_overrides`.)
- **Total raw font bytes are capped at 2 MiB** (~four generous variable faces) — an error, never a
  silent drop; base64 adds a third. Under `serve` the CSP allows `font-src data:` and nothing else.

## Verifying a theme actually applied

Work through this in order and report step 4's numbers back to the human.

1. Run `dossierx check`. A theme that does not resolve fails here, always as `invalid_config`, and
   `stopped_at` says which half of the rules caught it: **`stopped_at: config`** for anything
   readable from the config text alone (an unknown token, a value the grammar refuses, a duplicate
   key, an `extends` pointing outside the project), **`stopped_at: render`** for anything that
   needed a file read (an unknown `preset`, a missing or unstaged `extends`, a font whose bytes are
   not its extension, a family nothing names, the 2 MiB cap). `check --validate` and `check
   --staged` report the identical code and step, so a hook or CI run is not a way to skip the theme
   rules. **`data.theme_error` carries the detail on a `render` failure and is absent on a `config`
   one** (the config never loaded, so there is no result to carry it) — read it there rather than
   regexing `message`, which the router forbids.
2. Open the rendered `build/viewer/index.html` and confirm the colour you set is the colour you **see**.
   Only this catches a shape-valid non-colour or a value in the wrong layer — both pass `check` and
   render as the untouched engine default.
3. Check **both** OS colour schemes, not just yours. This is where a flat colour key shows itself.
   If you are serving rather than opening the file, **restart `dossierx serve`** first: it reads
   the config, the theme file and the fonts once at startup and watches none of them, so an
   un-restarted server shows the old theme however many times you reload.
4. Read `data.theme_font_count` and `data.theme_font_bytes` from the envelope. Both are **omitted**
   when no fonts are inlined, so a missing key is zero — and if the human declared a face and they
   are still absent, none was accepted. The byte count is what a reader downloads: say it out loud.

## What a theme deliberately cannot do

Do not promise any of these, and do not go looking for a flag:

- **No raw CSS.** The twenty-eight tokens are the whole vocabulary. There is no injection point.
- **No in-page toggle.** Light or dark follows the reader's OS; there is no switch to add.
- **No graph ramp.** The claims graph's facet colours are generated, not themed.
- **No per-claim, per-facet or per-module styling.** A theme is document-wide.
- **No proof your colour is a colour.** The grammar checks shape, not meaning: a well-formed value
  the browser rejects passes `check` and renders as the default. Only looking catches it.

`viewer.template_overrides` is the escape hatch, and it *replaces* rather than layers: an override
`style.css` supplants the engine's sheet wholesale and must consume the tokens itself, and an
override `shell.html` dropping `{{.ThemeCSS}}` gets no theme and no fonts at all.

## error.code → what you actually do about it

| code | exit | recovery |
|---|---|---|
| `invalid_config` | 1 | the theme did not resolve. Every way it can fail arrives here: an unknown token, a value the grammar refuses, **an unknown preset in `viewer.theme.preset`** (the message lists the known names), an unreadable or unstaged `extends` file, a font whose bytes are not its extension, a family nothing names, or the 2 MiB cap. `message` names the offending key. Fix `project.config.yaml` or the theme file, then re-run the **same** command that refused you. |
| `unknown_preset` | 1 | **`dossierx theme export` only** — the positional preset argument names something this binary does not carry (a typo, or a binary older than the preset). It is not what a bad `viewer.theme.preset` reports: that command loads no config, so there is nothing to edit. Run `dossierx theme list` and pass one of those names; the hint already names them. |
| `write_conflict` | 1 | `theme export` found a file at that path and did not overwrite it. Read it first: if the human has edited it, export somewhere else. `--force` replaces it, and only on their say-so. |
| `write_failed` | 1 | the export could not be written — an unwritable directory. Ordinary filesystem problem, ordinary fix. |

## Adding a theme to a project on an older binary

`viewer.theme` grew per-mode keys, presets, `extends` and `fonts` in this release. A binary that
predates it **fails the whole config load** rather than ignoring the new keys — `invalid_config`,
`stopped_at: config`. Two shapes, both *fragments* the `message` contains: each is prefixed
`load config: config: `, and the second also carries `parse <path>: yaml: unmarshal errors:` and a
`line N:` preamble. Match on the fragment.

```
viewer.theme: unknown theme token "preset" (must be one of accent, accent-bg, ink, muted, faint,
paper, card-bg, border, link, warn, warn-bg, font-sans, font-mono, radius)
```

for a scalar key (`preset`, `extends`), and `cannot unmarshal !!map into string` (or `!!seq` for
`fonts:`) for `light:`, `dark:` and `fonts:`. Either means their binary is older than their config,
not that their config is wrong. Failing closed is deliberate: half a theme applied is worse.

## Portability

`viewer.theme` is opt-in: a project that never writes it renders byte for byte as it always did.
The engine ships no font it was not handed, fetches nothing, and hardcodes no project's palette —
presets carry font *stacks* that fall through to system faces, never files.
