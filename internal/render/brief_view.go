package render

import (
	"encoding/json"
	"fmt"
	"html/template"

	"github.com/BarterX-Tech/dossierx/internal/briefs"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/render/markdown"
)

// briefsPayload is the viewer's data for briefs (NIT-204): the
// <script type="application/json" id="dossierx-briefs"> block. It is DATA
// ONLY — no pane reads it yet; NIT-197 builds the UI on it — so its shape is
// the contract that UI will be written against, and every number a brief page
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
// here while the same line in a claim body stays literal text. Images render as
// literal text for now: no route serves a brief's image yet, and an <img>
// pointing at nothing is worse than the reference the author wrote. The list
// of images, with their sizes, is carried so the UI can show them against the
// cap either way.
type briefPayload struct {
	ID         string         `json:"id"`
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
func briefsPayloadJSONWithBudget(set *briefs.Set, budget *renderByteBudget) (template.JS, error) {
	if set.Empty() {
		return "", nil
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
			Path:       b.Path,
			Folder:     b.Folder,
			Title:      b.Title,
			Summary:    b.Summary,
			Status:     string(b.Status),
			RestsOn:    restsOn,
			BodyHTML:   string(markdown.RenderDocument(b.Body, "")),
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
