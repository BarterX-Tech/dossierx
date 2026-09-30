package serve

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/briefs"
	"github.com/BarterX-Tech/dossierx/internal/render"
	"github.com/BarterX-Tech/dossierx/internal/render/markdown"
)

// briefAssetRoutePattern answers the relative src a brief page carries for
// each image, brief-assets/<folder>/<name> (render.BriefAssetDir), resolved
// against the root document. The static build puts a copy at the same path
// beside index.html (check's writeBriefAssets), so one page works both ways.
const briefAssetRoutePattern = "GET /" + render.BriefAssetDir + "/{folder}/{name}"

// handleBriefAsset serves one brief image, and only one a brief references.
//
// It answers from the allowlist the page itself implies — render.BriefAssets
// over a fresh read of the briefs tree, the same list the render emitted <img>
// tags for and a static build copies — never from the path the request spells.
// Both segments are held to the brief tree's own name rule first
// ([a-z0-9-] plus one of the six lowercase image extensions), so the lookup key
// needs no decoding and cannot traverse. The file is then refused unless it is
// a plain file whose resolved path is itself (no symlink at any step) inside
// briefs_dir. The response carries the claim-asset route's headers: nosniff,
// the sandboxing CSP (an SVG served here is an image, never a document), and
// no-store, since the file is the author's working copy.
func (s *Server) handleBriefAsset(w http.ResponseWriter, r *http.Request) {
	if r.URL.EscapedPath() != r.URL.Path {
		http.NotFound(w, r)
		return
	}
	folder, name := r.PathValue("folder"), r.PathValue("name")
	if !markdown.DocumentNameStem(folder) {
		http.NotFound(w, r)
		return
	}
	if canonical, ok := markdown.DocumentImageSrc(name); !ok || canonical != name {
		http.NotFound(w, r)
		return
	}
	ctype, ok := assetContentTypes[filepath.Ext(name)]
	if !ok {
		http.NotFound(w, r)
		return
	}

	rel := path.Join(render.BriefAssetDir, folder, name)
	var file string
	for _, a := range render.BriefAssets(s.cfg, briefs.Load(s.cfg)) {
		if a.Rel == rel {
			file = a.Src
			break
		}
	}
	if file == "" {
		http.NotFound(w, r)
		return
	}
	root, err := filepath.Abs(s.cfg.BriefsDirPath())
	if err != nil {
		http.NotFound(w, r)
		return
	}
	abs, err := filepath.Abs(file)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil || !strings.HasPrefix(resolved, resolvedRoot+string(filepath.Separator)) ||
		resolved != filepath.Join(resolvedRoot, folder, name) {
		http.NotFound(w, r)
		return
	}

	f, err := os.Open(resolved)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close() //nolint:errcheck // read-only handle
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", ctype)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", assetCSPValue)
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, "", info.ModTime(), f)
}
