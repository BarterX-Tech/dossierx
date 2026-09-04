# Vendored mermaid

The viewer's Build order tab renders its per-phase flowcharts with
[mermaid](https://github.com/mermaid-js/mermaid), embedded into the `dossierx`
binary and injected inline into a rendered `build/viewer/index.html` — only for
a project with at least one locked build order — so a viewer opened over
`file://` draws its diagrams with no network request. The engine references no
CDN anywhere (`tests/portability_test.go`, `TestNoNetworkReferencesAnywhereInEngine`).

| What | Where |
|---|---|
| the bundle | `internal/render/viewer/template/vendor/mermaid.min.js` — the npm package's `dist/mermaid.min.js`, unmodified, the ONLY file under that directory |
| version | `mermaid.VERSION` (one line) |
| licence | `mermaid.LICENSE` (the package's `LICENSE`, verbatim; MIT) |
| hash | `mermaid.SHA256` (`shasum -a 256` of the bundle) |

## Why the metadata lives here and not beside the file

`cmd/dossierx/surface_meta_test.go` (`TestSurfaceRenderFingerprintHashesEveryRenderSource`)
requires every tracked non-Go file under `internal/render/` to be embedded by a
`//go:embed` directive and hashed into the surface's render fingerprint. The
bundle is embedded, so it is hashed. A version file, a licence and a hash record
are not embedded by anything and would each be a "tracked render template and
NOT in the render fingerprint" error, so they live outside the render tree.

## Re-vendoring

1. `npm pack mermaid@<version>` and unpack the tarball.
2. Copy `package/dist/mermaid.min.js` over
   `internal/render/viewer/template/vendor/mermaid.min.js`. Nothing else from
   the package is vendored.
3. `shasum -a 256 internal/render/viewer/template/vendor/mermaid.min.js | awk '{print $1}' > third_party/mermaid/mermaid.SHA256`
   and write the version into `mermaid.VERSION`; replace `mermaid.LICENSE` if
   the package's licence text changed.
4. Re-measure the offline allowlist in `tests/portability_test.go`
   (`TestNoNetworkReferencesAnywhereInEngine`'s vendored-mermaid subtest):
   `grep -oE 'https?://[A-Za-z0-9.-]+' internal/render/viewer/template/vendor/mermaid.min.js | sort | uniq -c`
   plus each other arm of `networkRefPattern` individually. Every entry
   carries a count and a one-line reason; a token the new build no longer
   carries fails the test as stale, and one it newly carries fails it as
   unexplained. `TestVendoredMermaidHashMatchesRecord` fails on a re-vendor
   whose hash record was not updated, which is what forces this step.
5. Regenerate the rendered fixture viewers (`docs/RELEASING.md`) and run the
   browser suite: `viewer-tests/build_order_tab_test.go` measures the rendered
   SVG's element shapes and colours against the page's tokens, which is what
   catches a mermaid release that renames a class or changes a shape's element.
