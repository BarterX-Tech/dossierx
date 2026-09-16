package render

import (
	"encoding/base64"
	"fmt"
)

// The five engine-owned faces, inlined as data: URLs so a viewer that names
// them loads them from the single HTML file. They replace the two Geist faces
// this file used to carry: the viewer's own type is now Inter for chrome,
// Source Serif 4 for the prose a reviewer reads for meaning, and IBM Plex Mono
// for machine output (docs/design/tokens.md §7).
//
// Two of the three families are VARIABLE — one file spans the whole weight
// axis, declared `font-weight: 100 900` / `200 900` so the browser
// interpolates rather than synthesising. Source Serif 4 also carries an
// optical-size axis (opsz 8-60); a face declared normally has its opsz driven
// automatically from the used font-size, which is why nothing here spells
// font-optical-sizing: the CSS initial value (auto) is the behaviour wanted.
//
// IBM Plex Mono is the exception and it is a supply constraint, not a choice:
// no variable Plex Mono exists on the source CDN, so the three weights the
// design uses (400 regular, 500 the accented filename in a code-evidence
// header, 600 a caps authorship label) ship as three static faces under one
// family name. A weight the design does not use is deliberately absent rather
// than synthesised from 400.
const (
	interPath       = "viewer/template/fonts/inter-latin-wght.woff2"
	sourceSerifPath = "viewer/template/fonts/source-serif-4-latin-opsz-wght.woff2"
	plexMono400Path = "viewer/template/fonts/ibm-plex-mono-latin-400.woff2"
	plexMono500Path = "viewer/template/fonts/ibm-plex-mono-latin-500.woff2"
	plexMono600Path = "viewer/template/fonts/ibm-plex-mono-latin-600.woff2"
)

// engineFace is one @font-face rule's inputs. weight is the CSS font-weight
// descriptor: a range ("100 900") for a variable face, a single number for a
// static one.
type engineFace struct {
	family string
	path   string
	weight string
}

// engineFaces is the emission order, and it is the order the rules appear in
// every rendered viewer. Kept fixed so two renders of the same corpus produce
// byte-identical documents.
var engineFaces = []engineFace{
	{family: "Inter", path: interPath, weight: "100 900"},
	{family: "Source Serif 4", path: sourceSerifPath, weight: "200 900"},
	{family: "IBM Plex Mono", path: plexMono400Path, weight: "400"},
	{family: "IBM Plex Mono", path: plexMono500Path, weight: "500"},
	{family: "IBM Plex Mono", path: plexMono600Path, weight: "600"},
}

// engineFontFaceCSS inlines the engine-owned faces as data: URLs. A project
// stylesheet override replaces style.css wholesale and does not receive these
// faces — the same bound the Geist mechanism had.
func engineFontFaceCSS() ([]byte, error) {
	var out []byte
	for _, f := range engineFaces {
		b, err := shellFS.ReadFile(f.path)
		if err != nil {
			return nil, fmt.Errorf("render: load %s (%s): %w", f.family, f.path, err)
		}
		out = append(out, []byte(`@font-face{font-family:"`+f.family+`";src:url(data:font/woff2;base64,`)...)
		out = append(out, []byte(base64.StdEncoding.EncodeToString(b))...)
		out = append(out, []byte(`) format("woff2");font-weight:`+f.weight+`;font-style:normal;font-display:swap}`)...)
	}
	return out, nil
}
