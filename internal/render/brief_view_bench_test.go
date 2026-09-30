package render

import (
	"fmt"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/briefs"
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
				out, err := briefsPayloadJSONWithBudget(set, &renderByteBudget{remaining: 1 << 30})
				if err != nil {
					b.Fatal(err)
				}
				size = len(out)
			}
			b.ReportMetric(float64(size), "payload-bytes")
		})
	}
}
