package readiness_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/readiness"
	"github.com/BarterX-Tech/dossierx/internal/reaudit"
	"github.com/BarterX-Tech/dossierx/internal/render"
)

func TestAuditScale(t *testing.T) {
	for _, shape := range []struct {
		layers, width int
		mixed         bool
	}{
		{32, 1, false},
		{64, 1, false},
		{128, 1, false},
		{8, 5, false},
		{16, 5, false},
		{24, 5, false},
		{8, 5, true},
		{16, 5, true},
	} {
		t.Run(fmt.Sprintf("L%d-W%d-mixed%v", shape.layers, shape.width, shape.mixed), func(t *testing.T) {
			var claims []model.Claim
			for l := 0; l < shape.layers; l++ {
				for w := 0; w < shape.width; w++ {
					c := model.Claim{
						ID:     fmt.Sprintf("audit.n%03d_%02d", l, w),
						Facet:  "contract",
						Module: "audit",
						Status: model.StatusDraft,
						Body:   "audit body",
					}
					if l+1 < shape.layers {
						for k := 0; k < shape.width; k++ {
							c.RestsOn = append(c.RestsOn, fmt.Sprintf("audit.n%03d_%02d", l+1, k))
						}
					}
					if shape.mixed {
						c.Status = model.StatusLocked
					}
					claims = append(claims, c)
				}
			}
			s := &lock.Store{PolicyVersion: lock.PolicyLocalApprovalV1, Ledger: map[string]lock.LedgerRecord{}, Hashes: map[string]map[string]string{}}
			flags := &reaudit.FlagStore{Flags: map[string]reaudit.PendingFlag{}}
			for _, c := range claims {
				s.Ledger[c.ID] = lock.LedgerRecord{Subject: lock.SubjectClaim, Hash: lock.LockedClaimHash(c)}
				s.Hashes[c.ID] = map[string]string{}
				for _, d := range c.RestsOn {
					s.Hashes[c.ID][d] = "stale"
				}
				if shape.mixed {
					flags.Flags[c.ID] = reaudit.PendingFlag{Reason: "audit flag"}
				}
			}
			if !shape.mixed {
				s = nil
				flags = nil
			}
			runtime.GC()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			start := time.Now()
			a := readiness.Compute(claims, s, flags)
			elapsed := time.Since(start)
			runtime.ReadMemStats(&after)
			n := len(claims)
			e := (shape.layers - 1) * shape.width * shape.width
			count, hops := 0, 0
			for _, v := range a {
				count += len(v.Conditions) + len(v.Causes)
				for _, c := range v.Conditions {
					hops += len(c.Path)
				}
				for _, c := range v.Causes {
					hops += len(c.Path)
				}
			}
			if count > n*(3*n+2*e) {
				t.Fatalf("fact bound exceeded: %d", count)
			}
			if elapsed > 2*time.Second || after.TotalAlloc-before.TotalAlloc > 512*1024*1024 {
				t.Fatal("compute budget exceeded")
			}
			cfg := &config.Config{}
			cat, err := catalog.Build(claims, cfg)
			if err != nil {
				t.Fatal(err)
			}
			cat.SetReadiness(a)
			path := filepath.Join(t.TempDir(), "catalog.json")
			if err = catalog.WriteJSON(cat, path); err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			html, err := render.Render(cat, cfg)
			if err != nil {
				t.Fatal(err)
			}
			if len(b) > 64*1024*1024 || len(html) > 64*1024*1024 {
				t.Fatal("output byte budget exceeded")
			}
			t.Logf("V=%d E=%d facts=%d witnessIDs=%d catalogBytes=%d htmlBytes=%d compute=%s allocatedBytes=%d allocations=%d",
				n, e, count, hops, len(b), len(html), elapsed, after.TotalAlloc-before.TotalAlloc, after.Mallocs-before.Mallocs)
		})
	}
}
