package render

import (
	"fmt"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/briefs"
	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// BenchmarkBriefsPayload is the payload half of the scale evidence in
// docs/graph-safety/nit-204-briefs.md: the viewer's dossierx-briefs block for n
// briefs at the default word cap (every body rendered in document mode, the
// whole set JSON-encoded once, and its bytes charged to the budget). n = 60 is
// the default total cap; 2,000 is a raised max_briefs. It reports the
// payload's bytes, which grow with the briefs' own bytes and nothing else.
func BenchmarkBriefsPayload(b *testing.B) {
	_, cfg := briefViewFixture()
	body := "# Title\n\n## Why\n\n" + strings.Repeat("word ", 2000) + "\n"
	for _, n := range []int{60, 2000} {
		files := make([]briefs.File, 0, n)
		for i := 0; i < n; i++ {
			files = append(files, briefFile(fmt.Sprintf("f%04d/b%04d.md", i/12, i), "---\nsummary: s\n---\n"+body))
		}
		set := briefs.FromFiles(cfg, files)
		b.Run(fmt.Sprintf("briefs=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			var size int
			for i := 0; i < b.N; i++ {
				out, err := briefsPayloadJSONWithBudget(set, nil, &renderByteBudget{remaining: 1 << 30})
				if err != nil {
					b.Fatal(err)
				}
				size = len(out)
			}
			b.ReportMetric(float64(size), "payload-bytes")
		})
	}
}

// BenchmarkBriefsView is the page half (NIT-197): every body rendered once
// (renderBriefs) and the tree and pages assembled against a catalog in which
// every claim carries an internal source naming one brief, the worst case for
// the cited-by index. It reports the bytes of the rendered brief sections, the
// part of the page that grows with the briefs.
func BenchmarkBriefsView(b *testing.B) {
	_, cfg := briefViewFixture()
	body := "# Title\n\n## Why\n\n" + strings.Repeat("word ", 2000) + "\n"
	for _, tc := range []struct{ briefs, claims int }{{60, 1000}, {2000, 10000}} {
		files := make([]briefs.File, 0, tc.briefs)
		for i := 0; i < tc.briefs; i++ {
			files = append(files, briefFile(fmt.Sprintf("f%04d/b%04d.md", i/12, i), "---\nsummary: s\n---\n"+body))
		}
		set := briefs.FromFiles(cfg, files)
		cat := &catalog.Catalog{}
		for i := 0; i < tc.claims; i++ {
			cat.Claims = append(cat.Claims, model.Claim{
				ID: fmt.Sprintf("widget.contract.c%05d", i), Module: "widget", Facet: "contract", Status: model.StatusDraft,
				Sources: []model.Source{{Ref: 1, Kind: model.SourceKindInternal, Path: set.Briefs[i%len(set.Briefs)].Path}},
			})
		}
		b.Run(fmt.Sprintf("briefs=%d/claims=%d", tc.briefs, tc.claims), func(b *testing.B) {
			b.ReportAllocs()
			var size int
			for i := 0; i < b.N; i++ {
				view := buildBriefsView(set, renderBriefs(set, cat, cfg), cat)
				size = 0
				for _, f := range view.Folders {
					for _, p := range f.Pages {
						size += len(p.Body) + len(p.RestsOn) + len(p.CitedBy)
					}
				}
			}
			b.ReportMetric(float64(size), "page-bytes")
		})
	}
}

// BenchmarkBriefsPayloadWithReview is the NIT-205 half of the payload's scale
// evidence (docs/graph-safety/nit-192-briefs.md): n locked briefs at the
// default word cap, each with a standing record whose approved markdown is the
// brief's own body, resting on r = 10 claims of about 1,800 bytes that have ALL
// moved — every changed claim carries its wording then and now, and every
// brief its approved text rendered as HTML. It reports the payload's bytes,
// which grow with the briefs' bytes plus n·r claim wordings and nothing else.
func BenchmarkBriefsPayloadWithReview(b *testing.B) {
	_, cfg := briefViewFixture()
	body := "# Title\n\n## Why\n\n" + strings.Repeat("word ", 2000) + "\n"
	const r = 10
	claims := make([]model.Claim, 0, r)
	var rests strings.Builder
	for j := 0; j < r; j++ {
		id := fmt.Sprintf("widget.contract.c%02d", j)
		claims = append(claims, model.Claim{ID: id, Summary: "s", Body: strings.Repeat("claim words ", 150)})
		fmt.Fprintf(&rests, "  - %s\n", id)
	}
	moved := make([]model.Claim, len(claims))
	for i, c := range claims {
		c.Body += " moved"
		moved[i] = c
	}
	for _, n := range []int{60, 2000} {
		files := make([]briefs.File, 0, n)
		for i := 0; i < n; i++ {
			files = append(files, briefFile(fmt.Sprintf("f%04d/b%04d.md", i/12, i), "---\nsummary: s\nstatus: locked\nrests_on:\n"+rests.String()+"---\n"+body))
		}
		set := briefs.FromFiles(cfg, files)
		store, err := lock.LoadStore(b.TempDir() + "/lock-store.json")
		if err != nil {
			b.Fatal(err)
		}
		for _, br := range set.Briefs {
			hashes, receipts, _ := briefs.Baselines(br, claims)
			lock.RecordBriefApproval(store, br.ID, lock.BriefRecord{Path: br.Path, Hash: br.LockHash, Approved: lock.BriefApproved{Markdown: br.Body}, Baselines: hashes, Receipts: receipts})
		}
		review := briefs.Evaluate(set, moved, store)
		b.Run(fmt.Sprintf("briefs=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			var size int
			for i := 0; i < b.N; i++ {
				out, err := briefsPayloadJSON(set, nil, review, &renderByteBudget{remaining: 1 << 30})
				if err != nil {
					b.Fatal(err)
				}
				size = len(out)
			}
			b.ReportMetric(float64(size), "payload-bytes")
		})
	}
}
