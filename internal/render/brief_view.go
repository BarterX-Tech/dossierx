package render

import (
	"encoding/json"
	"fmt"
	"html/template"

	"github.com/BarterX-Tech/dossierx/internal/briefs"
	"github.com/BarterX-Tech/dossierx/internal/config"
)

// briefsPayload is the viewer's data for briefs (NIT-204): the
// <script type="application/json" id="dossierx-briefs"> block. It is data
// only: the brief pages (NIT-197) are rendered into the shell as sections, not
// built from this block, so a live reload's fragment swap carries them. Its
// shape is the contract a later pane (the index, NIT-203) reads, and every number a brief page
// or a folder index needs to show a count against its cap is here beside the
// cap itself.
//
// It carries nothing a claim contributes and nothing a claim reads. A brief's
// rests_on is the brief's own list of claim ids, as authored; whether each
// names a claim is brief-rests-on-unknown's question, answered in check.
type briefsPayload struct {
	Caps    config.BriefCaps `json:"caps"`
	Total   int              `json:"total"`
	Folders []briefs.Folder  `json:"folders"`
	Briefs  []briefPayload   `json:"briefs"`
}

// briefPayload is one brief. BodyHTML is the body after the frontmatter,
// rendered in document mode (markdown.RenderDocument), so "# Title" is an h1
// here while the same line in a claim body stays literal text. Its images
// point at brief-assets/<folder>/<name> (BriefAssetDir), the path the static
// build copies them to and serve answers (NIT-197). The list of images, with
// their sizes, is carried so the UI can show them against the cap.
//
// Anchor is the brief's page id in this viewer (NIT-197): the section a
// "#<anchor>" link opens. It is carried rather than left for a reader to
// spell, because two briefs, or a brief and a module, can spell the same
// brief-<folder>-<slug> and the later one is given a suffix (briefAnchors).
type briefPayload struct {
	ID         string         `json:"id"`
	Anchor     string         `json:"anchor"`
	Path       string         `json:"path"`
	Folder     string         `json:"folder"`
	Title      string         `json:"title"`
	Summary    string         `json:"summary"`
	Status     string         `json:"status"`
	RestsOn    []string       `json:"rests_on"`
	BodyHTML   string         `json:"body_html"`
	Words      int            `json:"words"`
	ImageCount int            `json:"image_count"`
	Images     []briefs.Image `json:"images"`
}

// briefsPayloadJSONWithBudget encodes set for the shell and charges every byte
// to budget — the same shared budget the claim fragments and the graph payload
// draw on, so a project cannot put an unbounded amount of brief prose into a
// viewer whose claims are held to a bound. An empty set returns an empty
// payload and charges nothing, which is what keeps a project with no briefs
// byte-identical.
//
// encoding/json's default HTML escaping is the guard here exactly as it is for
// the graph payload: body_html is markup, and it reaches the page as a JSON
// string whose "<" is written <, never as a tag.
//
// rendered is renderBriefs' output for set, shared with the brief pages so
// each body is rendered once; nil renders it here, with no catalog or config
// to reserve page ids against.
func briefsPayloadJSONWithBudget(set *briefs.Set, rendered map[string]renderedBrief, budget *renderByteBudget) (template.JS, error) {
	if set.Empty() {
		return "", nil
	}
	if rendered == nil {
		rendered = renderBriefs(set, nil, nil)
	}
	p := briefsPayload{
		Caps:    set.Caps,
		Total:   len(set.Briefs),
		Folders: set.Folders,
		Briefs:  make([]briefPayload, 0, len(set.Briefs)),
	}
	for _, b := range set.Briefs {
		restsOn := b.RestsOn
		if restsOn == nil {
			restsOn = []string{}
		}
		images := b.Images
		if images == nil {
			images = []briefs.Image{}
		}
		p.Briefs = append(p.Briefs, briefPayload{
			ID:         b.ID,
			Anchor:     rendered[b.ID].anchor,
			Path:       b.Path,
			Folder:     b.Folder,
			Title:      b.Title,
			Summary:    b.Summary,
			Status:     string(b.Status),
			RestsOn:    restsOn,
			BodyHTML:   rendered[b.ID].body,
			Words:      b.Words,
			ImageCount: len(b.Images),
			Images:     images,
		})
	}
	out, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("render: encode briefs payload: %w", err)
	}
	if err := budget.consume(len(out)); err != nil {
		return "", fmt.Errorf("render: briefs payload: %w", err)
	}
	return template.JS(out), nil
}
