package render

import (
	"encoding/base64"
	"fmt"
)

const (
	geistSansPath = "viewer/template/fonts/geist-latin-wght.woff2"
	geistMonoPath = "viewer/template/fonts/geist-mono-latin-wght.woff2"
)

// geistFontFaceCSS inlines the engine-owned Geist faces as data: URLs so a
// viewer that names them (the claude preset's New York stacks) loads them
// from the single HTML file. A project stylesheet override replaces style.css
// wholesale and does not receive these faces.
func geistFontFaceCSS() ([]byte, error) {
	sans, err := shellFS.ReadFile(geistSansPath)
	if err != nil {
		return nil, fmt.Errorf("render: load Geist Sans: %w", err)
	}
	mono, err := shellFS.ReadFile(geistMonoPath)
	if err != nil {
		return nil, fmt.Errorf("render: load Geist Mono: %w", err)
	}
	out := []byte("@font-face{font-family:Geist;src:url(data:font/woff2;base64,")
	out = append(out, []byte(base64.StdEncoding.EncodeToString(sans))...)
	out = append(out, []byte(") format(\"woff2\");font-weight:100 900;font-style:normal;font-display:swap}")...)
	out = append(out, []byte("@font-face{font-family:\"Geist Mono\";src:url(data:font/woff2;base64,")...)
	out = append(out, []byte(base64.StdEncoding.EncodeToString(mono))...)
	out = append(out, []byte(") format(\"woff2\");font-weight:100 900;font-style:normal;font-display:swap}")...)
	return out, nil
}
