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
  and their 2 MiB budget, how to verify a theme actually applied, and the four
  things a theme deliberately cannot do. Load the DossierX router skill first.
---

# DossierX theming — the viewer's colours and fonts

Read **[`dossierx`](../dossierx/SKILL.md)** for the envelope and error codes first.

A theme is a set of CSS custom-property values the engine injects into the viewer. It changes
**how the document looks and nothing about what it says** — no claim, no lock, no gate, no
approval is touched, and no lint rule reads it. That is why restyling is fully yours: it is a
presentation change, and the human sees the result the moment they reload.

**Reach for this when** the human asks for the viewer to match their product, says it "looks
generic", asks about dark mode, or wants their own typeface. **Do not** reach for it to fix a
layout complaint, to highlight one claim, or to make something stand out — there is no per-claim
styling, and the answer to "this claim is hard to find" is the corpus, not the palette.

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

Five keys are structure — `preset`, `extends`, `light`, `dark`, `fonts` — and **every other key is
a token name**, drawn from the twenty-eight below. A typo is a load-time error naming the whole
vocabulary, never a silently ignored key.

**Quote every colour.** A bare `accent: #C6613F` is, to YAML, the key `accent` with a comment and
therefore a *null* value; the decoder catches it and says so, but it is the single most common
mistake here. Lengths and font stacks do not need it (`radius: 10px` above is fine unquoted) —
quoting them anyway is harmless, and `dossierx theme export` quotes everything for that reason.

Layers, lowest first: **preset → `extends` file → inline keys**. Within each layer, flat keys apply
to both colour schemes and `light:`/`dark:` apply to one. Nothing merges across projects and there
is no chaining: a theme file may not itself carry `extends`.

## The tokens

Twenty-eight, and there are no others. Defaults are the engine's own; **"light / dark" columns that
differ are the ones that matter** — see the trap below.

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

**`*(derived)*` is not `*(same)*`.** Two defaults are `color-mix()` **expressions over other
tokens** rather than fixed colours: `code-inline-bg` and `code-bg` are mixes of `paper` and
`card-bg`, so they follow whatever those two are — in either scheme, and in a theme that only ever
sets `paper`. They are declared once, so the engine's dark block does not re-point them, but their
*computed* value is darker in dark mode all the same. Overriding either with a flat colour opts out
of that: it will then be the same colour on a dark page as on a light one, which is exactly the
trap below. Set them under `light:`/`dark:`, or leave them alone and set `paper`.

Values are validated as hostile input, not trusted as CSS. Colours must be `#hex`, an
`rgb(`/`hsl(`/`oklch(`/`color-mix(`-family function, or a CSS named colour; `radius` must carry a
unit (`10px`, `0.5rem`, or bare `0`); font families are comma-separated items, quoted or plain.
Anything carrying `;`, `{}`, `<>`, a comment marker or a control character is refused outright.

### The trap: a flat colour key pins both modes

**Fourteen of these tokens are re-declared in dark mode** — every row with a value in the dark
column — and two more, the `*(derived)*` pair, vary without being re-declared. Writing one of them flat sets it for **both** schemes:

```yaml
viewer:
  theme:
    paper: '#FAF9F5'
```

That is a legal, silent way to give every dark-mode reader a near-white page. It is legal because
it is sometimes exactly right (`mockup-bg` is flat on purpose), so nothing warns about it — **you**
have to know which column you are in. The rule:

- a token whose two defaults differ → set it under `light:` **and** `dark:`, or accept that you have
  pinned both;
- a `*(derived)*` token → treat it as differing: flat freezes an expression that was tracking the
  mode;
- a token whose dark column says *(same)*, and `font-sans`, `font-mono`, `radius` → flat is fine and
  is what you want;
- setting only `dark:` is legal, and the light default survives. **Printing always uses the light
  values**, whatever the reader's OS is set to.

## Presets, and the export → edit → extends workflow

`dossierx theme list` reports every built-in preset and the tokens it sets. `preset: claude` is the
whole adoption step — no file, nothing to maintain. Two things to tell the human:

- **preset values may change between minor releases.** They track a palette this project does not
  own. Every change gets a CHANGELOG line, but a project that needs a value frozen writes it
  inline (or exports), where it wins.
- **the claims graph's facet colour ramp does not follow a preset.** It is generated to stay
  distinguishable, and a preset cannot repoint it.

When the human wants the preset as a *starting point* and then diverges, export it, edit the file,
and extend it:

```
dossierx theme export claude themes/mine.yaml     # writes the whole palette as editable YAML
```

```yaml
viewer:
  theme:
    extends: themes/mine.yaml
    accent: '#2F6FCB'
```

The exported file carries no `extends` of its own and no version stamp, so re-exporting is
byte-identical and a diff means the preset moved. `extends` is resolved relative to
`project.config.yaml` and **must stay under the project directory**. `theme export` refuses to
overwrite an existing file (`write_conflict`); pass `--force` only when the human says to, because
that file is the one they edit. With no path, the YAML comes back in `data.yaml` instead.

## Fonts

A theme may inline the project's **own local font files**. There is no network fetch, ever: the
viewer is one self-contained file and the faces are base64 `data:` URLs inside it.

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

- `src` is relative to the file that declares it (the config, or the theme file), extension one of
  `.woff2 .woff .ttf .otf`, and the **bytes must match the extension** — a renamed file is refused,
  because a browser handed one drops the face silently and renders a fallback nobody chose.
- `weight` is `"400"` or a variable range `"100 900"`; `style` is `normal` or `italic`. Both default.
- **Every declared family must appear in `font-sans` or `font-mono`**, or the theme is refused: a
  face nothing names is bytes every reader downloads and no reader sees. (Skipped when
  `viewer.template_overrides` is set — an override sheet may name it itself.)
- **Total raw font bytes are capped at 2 MiB**, roughly four generous variable faces. Over it is an
  error, never a silent drop. Base64 adds a third on top in the emitted file.
- Under `dossierx serve` the CSP allows `font-src data:` and nothing else.

## Verifying a theme actually applied

Work through this in order and report step 4's numbers back to the human.

1. Run `dossierx check`. A theme that does not resolve fails here, always as `invalid_config`, and
   `stopped_at` says which half of the rules caught it: **`stopped_at: config`** for anything
   readable from the config text alone (an unknown token, a value the grammar refuses, a duplicate
   key, an `extends` pointing outside the project), **`stopped_at: render`** for anything that
   needed a file read (an unknown `preset`, a missing or unstaged `extends`, a font whose bytes are
   not its extension, a family nothing names, the 2 MiB cap). `check --validate` and `check
   --staged` report the identical code and step, so a hook or CI run is not a way to skip the theme
   rules.
2. Open the rendered `viewer/index.html` and confirm the colour you set is the colour you see. The
   engine's sheet declares defaults for all twenty-eight tokens, so a *typo'd value* is a load
   error but a *right value in the wrong layer* renders as the untouched default.
3. Check **both** OS colour schemes, not just yours. This is where a flat colour key shows itself.
4. Read `data.theme_font_count` and `data.theme_font_bytes` from `dossierx check`'s envelope. Both
   keys are **omitted entirely** when the project inlines no fonts, so treat a missing key as zero —
   and if the human declared a face and the keys are still absent, no face was accepted. The byte
   count is what a reader downloads before seeing anything: say it out loud.

## What a theme deliberately cannot do

Do not promise any of these, and do not go looking for a flag:

- **No raw CSS.** The twenty-eight tokens are the whole vocabulary. There is no injection point.
- **No in-page toggle.** Light or dark follows the reader's OS; there is no switch to add.
- **No graph ramp.** The claims graph's facet colours are generated, not themed.
- **No per-claim, per-facet or per-module styling.** A theme is document-wide.

`viewer.template_overrides` is the escape hatch, and it is a *replacement*, not a layer: an override
`style.css` replaces the engine's sheet wholesale and must then declare or consume the tokens
itself, and an override `shell.html` that drops `{{.ThemeCSS}}` gets no theme and no fonts at all.

## error.code → what you actually do about it

| code | exit | recovery |
|---|---|---|
| `invalid_config` | 1 | the theme did not resolve. Every way it can fail arrives here: an unknown token, a value the grammar refuses, **an unknown preset in `viewer.theme.preset`** (the message lists the known names), an unreadable or unstaged `extends` file, a font whose bytes are not its extension, a family nothing names, or the 2 MiB cap. `message` names the offending key. Fix `project.config.yaml` or the theme file, then re-run the **same** command that refused you. |
| `unknown_preset` | 1 | **`dossierx theme export` only** — the positional preset argument names something this binary does not carry (a typo, or a binary older than the preset). It is not what a bad `viewer.theme.preset` reports: that command loads no config, so there is nothing to edit. Run `dossierx theme list` and pass one of those names; the hint already names them. |
| `write_conflict` | 1 | `theme export` found a file at that path and did not overwrite it. Read it first: if the human has edited it, export somewhere else. `--force` replaces it, and only on their say-so. |
| `write_failed` | 1 | the export could not be written (a directory that is not writable). Ordinary filesystem problem, ordinary fix. |

## Adding a theme to a project on an older binary

`viewer.theme` grew per-mode keys, presets, `extends` and `fonts` in this release. A binary that
predates it **fails the whole config load** rather than ignoring the new keys, at
`stopped_at: config`, `invalid_config`. Two shapes. Both are fragments the `message` **contains**,
never the whole of it: each is prefixed `load config: config: `, and the second also carries
`parse <path>: yaml: unmarshal errors:` and a `line N:` preamble. Match on the fragment.

```
viewer.theme: unknown theme token "preset" (must be one of accent, accent-bg, ink, muted, faint,
paper, card-bg, border, link, warn, warn-bg, font-sans, font-mono, radius)
```

for a scalar key (`preset`, `extends`), and a YAML-level `cannot unmarshal !!map into string` (or
`!!seq` for `fonts:`) for `light:`, `dark:` and `fonts:`. If a human reports either, the answer is
that their binary is older than their config, not that their config is wrong. Failing closed is
deliberate: a viewer rendered with half a theme applied is worse than one that refuses to build.

## Portability

`viewer.theme` is entirely opt-in; a project that never writes it renders exactly as it always did,
byte for byte. The engine ships no font it was not handed, fetches nothing, and hardcodes no
project's palette — presets carry font *stacks* that fall through to system faces, never files.
