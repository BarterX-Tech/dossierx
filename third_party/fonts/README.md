# Vendored viewer faces

The viewer's own type, engine-owned and inlined as `data:` URLs by
`internal/render/engine_fonts.go`: Inter for chrome, Source Serif 4 for the
prose a reviewer reads for meaning, IBM Plex Mono for machine output
(`docs/design/tokens.md` §7). The woff2 files live next to the shell so
`go:embed` can reach them; metadata stays here so it is not hashed as a render
template.

Every file is the latin subset only. Inter and Source Serif 4 are variable —
one file spans the weight axis (Source Serif 4 also carries optical size, opsz
8–60, which the browser drives from the used font-size). No variable IBM Plex
Mono is published, so its three weights ship as three static faces under one
family name.

| Family | Axes / weight | Where |
|---|---|---|
| Inter | variable, wght 100–900 | `internal/render/viewer/template/fonts/inter-latin-wght.woff2` |
| Source Serif 4 | variable, opsz 8–60 · wght 200–900 | `internal/render/viewer/template/fonts/source-serif-4-latin-opsz-wght.woff2` |
| IBM Plex Mono | static 400 | `internal/render/viewer/template/fonts/ibm-plex-mono-latin-400.woff2` |
| IBM Plex Mono | static 500 | `internal/render/viewer/template/fonts/ibm-plex-mono-latin-500.woff2` |
| IBM Plex Mono | static 600 | `internal/render/viewer/template/fonts/ibm-plex-mono-latin-600.woff2` |
| licence | `LICENSE` (SIL Open Font License 1.1, all three families) | |

These replaced the Geist / Geist Mono pair the `claude` preset used to name.
