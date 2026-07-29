// asset_scope.go implements the "asset-scope" lint: a claim's images must live
// in an assets/ directory beside the claim file itself.
//
// THIS IS THE FIRST RULE IN THE ENGINE THAT TIES A CLAIM TO WHERE IT LIVES,
// and that is the whole reason its message is as long as it is.
//
// Every other rule in this package reads a claim as a bag of fields. The
// loader works the same way on purpose: it walks the claims directory, reads
// every file it finds, and takes module and facet from fields INSIDE each
// claim — directory structure is deliberately not part of the claim schema, so
// a project may keep every claim in one flat claims/ directory, or one
// directory per module, or one per facet, or any mixture, and every command in
// the engine behaves identically. That freedom is a documented property, not an
// accident.
//
// Co-location breaks it, for one construct only. Gate 0's asset-scope decision
// is that an image src resolves against filepath.Dir(claim.SourcePath) and the
// resolved path must sit under <that directory>/assets/. So a project with a
// flat claims/ layout — the layout the rest of the engine encourages — has
// exactly one place every image in the project may live (claims/assets/), and
// an author who moves a diagram next to the module it documents gets a lint
// failure with no obvious cause, because nothing else they have ever done with
// this tool cared which file a claim was in. The message below therefore says
// that in as many words. An author who reads it should not have to guess.
//
// The rule is fixed, not configurable: "assets" is a literal, and the
// extension allowlist is .png .jpg .jpeg .gif .webp .svg. There is no project
// setting for either, because a per-project asset root would be a second way
// to answer a question the co-location rule exists to have exactly one answer
// to, and the reviewing human would then have to check the config to know
// whether a diagram is in scope.
//
// SEVERITY IS ERROR (gate 0, decision A5: an asset-scope violation is
// security-relevant, not craft). An out-of-scope src is a reference the viewer
// will be asked to serve from outside the claim's own directory, and serve's
// image route answers from an allowlist computed from loaded claims; a src the
// lint refuses is a src that route must never learn about. Unlike
// markdown-sanity's craft findings this one is meant to block a lock, and it
// does, through internal/lock.Lock's existing error-severity gate.
//
// WHAT THIS LINT DOES NOT CHECK, and why:
//
//   - It does not stat the filesystem. A missing file is a different failure
//     with a different fix, it is not knowable from a claim corpus loaded from
//     anywhere but disk, and a lint that touched the filesystem could not be
//     run over an in-memory candidate the way internal/lock runs the suite.
//
//   - It does not report an OFF-ORIGIN src. An src with a scheme, an authority
//     prefix, a root-relative leading slash, a query or a fragment is not a
//     path at all: it has no directory to be in or out of, so it is entirely
//     markdown-sanity's "rejected image src" and reporting it twice under two
//     rule names would send an author looking for two problems.
//
//     A ".." traversal is the deliberate exception. "../other-facet/assets/
//     x.png" is refused by the renderer AND is the single most likely
//     co-location mistake, and the message below is the only one in the engine
//     that explains where images must live — so both rules fire on it, on the
//     package's existing co-firing precedent (cycle/self-edge,
//     dangling/mirror-unanchored). See mdImageSrcOffOrigin.
//
//   - It skips a claim whose SourcePath is empty. SourcePath is populated by
//     the loader and is not part of the YAML schema, so a claim built in
//     memory (a test fixture, a proposed edit) has no directory to resolve
//     against. There is nothing to check rather than something to permit.
package lint

import (
	"fmt"
	"html"
	"path"
	"path/filepath"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, AssetScope{})
}

// AssetScope is the "asset-scope" lint.
type AssetScope struct{}

// Name returns this lint's rule name.
func (AssetScope) Name() string { return "asset-scope" }

// assetDirName is the fixed directory an image must resolve into, relative to
// the directory holding the claim file. It is deliberately not configurable —
// see this file's doc comment.
const assetDirName = "assets"

// assetExtensions is the fixed extension allowlist from gate 0.
var assetExtensions = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".gif":  true,
	".webp": true,
	".svg":  true,
}

// Check reports every image whose src, resolved against the directory holding
// the claim file, does not land on a file directly or indirectly under that
// directory's assets/ subdirectory, or whose extension is outside the
// allowlist.
//
// Only Body and Steps are scanned: they are the two image-permitting surfaces
// (amendment A3's per-surface permission table). A table cell, a comment body
// and Governed.Reason render no image at all, so an image there is
// markdown-sanity's business and never becomes an asset reference.
func (AssetScope) Check(claims []model.Claim, _ *config.Config) []Finding {
	var findings []Finding
	for _, c := range claims {
		if c.SourcePath == "" {
			continue
		}
		dir := filepath.Dir(c.SourcePath)
		if c.Body != "" {
			findings = append(findings, assetScopeSurface(c, dir, "body", c.Body)...)
		}
		for i, step := range c.Steps {
			if step == "" {
				continue
			}
			findings = append(findings, assetScopeSurface(c, dir, fmt.Sprintf("steps[%d]", i), step)...)
		}
	}
	return findings
}

// assetScopeSurface checks every image on one surface of one claim.
func assetScopeSurface(c model.Claim, dir, surface, source string) []Finding {
	var findings []Finding
	for _, img := range mdImagesIn(source) {
		if mdImageSrcOffOrigin(img.src) {
			// Not a path at all — a scheme, an authority prefix, a
			// root-relative slash, a query or a fragment. It has no directory
			// to be in or out of, so markdown-sanity's "rejected image src"
			// is the whole story. See mdImageSrcOffOrigin for why a ".."
			// traversal is NOT in this bucket and is reported here as well.
			continue
		}
		src := assetCleanSrc(img.src)
		if src == "" {
			continue
		}
		if !assetExtensions[strings.ToLower(path.Ext(assetRelSrc(src)))] {
			findings = append(findings, Finding{
				LintName: "asset-scope",
				ClaimID:  c.ID,
				Message: fmt.Sprintf(
					"%s: image src %q has an unsupported extension — a claim image must be one of .png .jpg .jpeg .gif .webp .svg. The list is fixed, not a project setting",
					surface, img.src),
				Severity: SeverityError,
			})
			continue
		}
		if assetUnderAssetsDir(dir, src) {
			continue
		}
		// PATHS IN THE MESSAGE ARE RELATIVE TO THE CLAIM FILE, never absolute.
		// SourcePath is an absolute filesystem path, and interpolating it would
		// put the author's home directory into a finding that is printed to the
		// terminal, serialized into the JSON envelope an agent parses, and
		// pasted into review discussion — and would make the same corpus
		// produce different lint bytes on two machines.
		findings = append(findings, Finding{
			LintName: "asset-scope",
			ClaimID:  c.ID,
			Message: fmt.Sprintf(
				"%s: image src %q resolves to %q relative to the directory holding %s, which is not inside that directory's %q subdirectory. "+
					"A claim's images must sit in an \"assets\" directory beside the claim file itself — no ../other-facet/assets/, and no shared top-level pool. "+
					"This is the ONLY rule in the engine that cares which directory a claim file is in: the loader ignores directory structure entirely and takes module and facet from fields inside the claim, so a flat claims/ layout works everywhere else and still fails here. "+
					"Either move the image to %q beside %s, or give this claim its own directory with an assets/ folder next to it",
				surface,
				img.src,
				assetRelSrc(src),
				filepath.Base(c.SourcePath),
				assetDirName,
				path.Join(assetDirName, path.Base(assetRelSrc(src))),
				filepath.Base(c.SourcePath)),
			Severity: SeverityError,
		})
	}
	return findings
}

// assetCleanSrc normalizes an src for path resolution: entity-decoded (the
// same decode the legality gate applies, so "assets&#47;x.png" resolves the
// way a browser would) and trimmed. Control-byte stripping is deliberately NOT
// applied here — mdImageSrcLegal has already refused anything a control byte
// could have disguised, and stripping bytes at this point would silently
// rewrite a filename that legitimately contains a space.
func assetCleanSrc(raw string) string {
	return strings.TrimSpace(html.UnescapeString(raw))
}

// assetRelSrc is the src as a normalized path relative to the claim's own
// directory ("./assets/x.png" becomes "assets/x.png", "../a/b.png" stays
// "../a/b.png"), always with forward slashes. It is what the message shows
// instead of an absolute resolved path.
//
// An image src is a URL path, not an OS path, so everything DISPLAYED here
// goes through package path (slash-only) while everything RESOLVED against
// SourcePath goes through path/filepath. Keeping the two apart is also what
// keeps this file free of hand-built "x + \"/\" + y" separators, which the
// portability guard in tests/ rejects outright.
func assetRelSrc(src string) string {
	return filepath.ToSlash(filepath.Clean(filepath.FromSlash(src)))
}

// assetUnderAssetsDir reports whether src, resolved against dir, lands
// strictly inside dir/assets/. Nested subdirectories under assets/ are in
// scope; assets/ itself is not a file, so an src that resolves to exactly the
// directory is out.
func assetUnderAssetsDir(dir, src string) bool {
	base := filepath.Clean(filepath.Join(dir, assetDirName))
	resolved := filepath.Clean(filepath.Join(dir, filepath.FromSlash(src)))
	rel, err := filepath.Rel(base, resolved)
	if err != nil {
		return false
	}
	if rel == "." || rel == ".." {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
