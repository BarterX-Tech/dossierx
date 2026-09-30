package briefs

import (
	"fmt"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// BenchmarkBriefsAtScale is the scale evidence docs/graph-safety/nit-204-briefs.md
// records: discovery (FromFiles, which parses every brief and checks the caps)
// and the claim-aware rules (Findings) over n briefs at the default word cap,
// each referencing three present images, twelve to a folder, and every brief
// resting on the SAME two claims — the worst case for brief-rests-on-duplicate,
// one group of n. n = 60 is the default total cap; 600 and 2,000 are what a
// raised max_briefs allows. It reports the findings' message bytes, which the
// REG-4 bound keeps linear in n.
func BenchmarkBriefsAtScale(b *testing.B) {
	cfg := testConfig(b, b.TempDir(), "")
	claims := []model.Claim{{ID: "widget.contract.a"}, {ID: "widget.contract.b"}}
	body := strings.Repeat("word ", 2000) + "\n\n![one](one.svg) ![two](two.svg) ![three](three.svg)\n"
	for _, n := range []int{60, 600, 2000} {
		files := make([]File, 0, n*4)
		for i := 0; i < n; i++ {
			folder := fmt.Sprintf("f%04d", i/12)
			brief := md("---\nsummary: s\nrests_on: [widget.contract.a, widget.contract.b]\n---\n" + body)
			brief.Rel = fmt.Sprintf("%s/b%04d.md", folder, i)
			files = append(files, brief)
			if i%12 == 0 {
				for _, img := range []string{"one.svg", "two.svg", "three.svg"} {
					files = append(files, File{Rel: folder + "/" + img, Regular: true, Size: 100})
				}
			}
		}
		b.Run(fmt.Sprintf("briefs=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			var msgBytes int
			for i := 0; i < b.N; i++ {
				in := append([]File(nil), files...)
				s := FromFiles(cfg, in)
				msgBytes = 0
				for _, f := range s.Findings(claims) {
					msgBytes += len(f.Message)
				}
			}
			b.ReportMetric(float64(msgBytes), "finding-bytes")
		})
	}
}
