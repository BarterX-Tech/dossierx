package briefs

import (
	"fmt"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/lock"
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

// BenchmarkEvaluateAtScale is the scale evidence docs/graph-safety/
// nit-192-briefs.md records for the lock lifecycle (NIT-205): n locked briefs,
// each with a standing record and r rests_on baselines, EVERY baseline drifted
// (the worst case: every changed claim carries its wording then and now), over
// c claims of ordinary length. The brief's own receipts carry the baseline
// wording, which is the path every record `brief lock` writes takes. It reports
// the changed-claim count, which must be n·r, never more.
func BenchmarkEvaluateAtScale(b *testing.B) {
	cfg := testConfig(b, b.TempDir(), "")
	for _, tc := range []struct{ n, r int }{{60, 10}, {600, 10}, {2000, 10}} {
		claims := make([]model.Claim, 0, tc.r*4)
		for i := 0; i < tc.r*4; i++ {
			claims = append(claims, model.Claim{ID: fmt.Sprintf("widget.contract.c%03d", i), Summary: "s", Body: strings.Repeat("claim words ", 150)})
		}
		files := make([]File, 0, tc.n)
		for i := 0; i < tc.n; i++ {
			var rests strings.Builder
			for j := 0; j < tc.r; j++ {
				fmt.Fprintf(&rests, "  - %s\n", claims[(i+j)%len(claims)].ID)
			}
			f := md("---\nsummary: s\nstatus: locked\nrests_on:\n" + rests.String() + "---\n" + strings.Repeat("word ", 500) + "\n")
			f.Rel = fmt.Sprintf("f%04d/b%04d.md", i/12, i)
			files = append(files, f)
		}
		set := FromFiles(cfg, files)
		store, err := lock.LoadStore(b.TempDir() + "/lock-store.json")
		if err != nil {
			b.Fatal(err)
		}
		for _, br := range set.Briefs {
			hashes, receipts, _ := Baselines(br, claims)
			lock.RecordBriefApproval(store, br.ID, lock.BriefRecord{Path: br.Path, Hash: br.LockHash, Baselines: hashes, Receipts: receipts})
		}
		moved := make([]model.Claim, len(claims))
		for i, c := range claims {
			c.Body += " moved"
			moved[i] = c
		}
		b.Run(fmt.Sprintf("briefs=%d/rests_on=%d", tc.n, tc.r), func(b *testing.B) {
			b.ReportAllocs()
			var changed int
			for i := 0; i < b.N; i++ {
				e := Evaluate(set, moved, store)
				changed = 0
				for _, r := range e.Reviews {
					changed += len(r.ChangedClaims)
				}
			}
			if changed != tc.n*tc.r {
				b.Fatalf("changed claims = %d, want %d", changed, tc.n*tc.r)
			}
			b.ReportMetric(float64(changed), "changed-claims")
		})
	}
}
