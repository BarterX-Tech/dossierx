package catalog

import (
	"fmt"
	"testing"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// buildClaims generates n well-formed, distinct claims spread evenly
// across two facets and one module, so Build's ByFacet/ByModule grouping
// does real work rather than degenerating to a single bucket.
func buildClaims(n int) []model.Claim {
	facets := []string{"contract", "internals"}
	claims := make([]model.Claim, n)
	for i := 0; i < n; i++ {
		claims[i] = model.Claim{
			ID:     fmt.Sprintf("widget.%s.claim-%06d", facets[i%2], i),
			Facet:  facets[i%2],
			Module: "widget",
			Status: model.StatusDraft,
			Body:   "generated fixture claim for scale testing",
		}
	}
	return claims
}

// TestBuild_LinearScaling is a coarse regression guard against Build
// degrading to O(n^2) in the claim count (e.g. via an accidental
// claim-against-claim cross-check creeping into the per-claim loop). It
// times Build at two sizes, one 8x the other, and asserts the larger run
// isn't drastically more than 8x slower — true linear (or n log n, which
// the per-facet/module sort contributes) scaling stays well under that
// bound, while an O(n^2) implementation would be roughly 8x slower again
// (i.e. ~64x), which this comfortably catches without being flaky.
func TestBuild_LinearScaling(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scale timing test in -short mode")
	}

	small := buildClaims(2000)
	large := buildClaims(16000)
	cfg := &config.Config{SchemaVersion: 1, Facets: []string{"contract", "internals"}, Modules: []string{"widget"}}

	// Warm up (first call pays for allocator/OS caching effects) before
	// timing either size.
	if _, err := Build(small, cfg); err != nil {
		t.Fatalf("warmup Build: %v", err)
	}

	// Both sizes are measured per-operation, repeating until the total clears
	// a floor. A single timed call is not safe on Windows, whose clock
	// granularity rounds a fast Build to exactly zero — and the previous guard
	// for that ("if smallElapsed <= 0 { smallElapsed = time.Nanosecond }")
	// turned an unmeasurable baseline into a ratio in the millions, failing the
	// test for being too FAST rather than saving it.
	smallElapsed := perBuildTime(t, small, cfg)
	largeElapsed := perBuildTime(t, large, cfg)

	ratio := float64(largeElapsed) / float64(smallElapsed)
	const sizeFactor = 8.0       // 16000 / 2000
	const maxAllowedRatio = 32.0 // generous slack above linear (8x), well under O(n^2)'s ~64x

	t.Logf("Build(2000)=%v Build(16000)=%v ratio=%.2f (size factor %.0fx)", smallElapsed, largeElapsed, ratio, sizeFactor)
	if ratio > maxAllowedRatio {
		t.Errorf("Build appears superlinear: 8x claims took %.2fx as long (want <= %.0fx); smallElapsed=%v largeElapsed=%v",
			ratio, maxAllowedRatio, smallElapsed, largeElapsed)
	}
}

// TestBuild_LargeClaimsDirCorrectness pins down that Build stays correct
// (not just fast) at a claim count too large to eyeball: every claim
// survives Build, and the by-facet/by-module indexes both contain every
// id exactly once.
func TestBuild_LargeClaimsDirCorrectness(t *testing.T) {
	const n = 5000
	claims := buildClaims(n)
	cfg := &config.Config{SchemaVersion: 1, Facets: []string{"contract", "internals"}, Modules: []string{"widget"}}

	cat, err := Build(claims, cfg)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(cat.Claims) != n {
		t.Fatalf("got %d claims, want %d", len(cat.Claims), n)
	}

	total := 0
	for _, ids := range cat.ByFacet {
		total += len(ids)
	}
	if total != n {
		t.Errorf("ByFacet holds %d ids total, want %d", total, n)
	}

	total = 0
	for _, ids := range cat.ByModule {
		total += len(ids)
	}
	if total != n {
		t.Errorf("ByModule holds %d ids total, want %d", total, n)
	}
}

// perBuildTime returns the cost of one Build(claims, cfg), repeating the build
// until the measured total clears buildTimerFloor so the result is a real
// per-operation duration on every platform.
func perBuildTime(t *testing.T, claims []model.Claim, cfg *config.Config) time.Duration {
	t.Helper()
	const buildTimerFloor = 2 * time.Millisecond
	for n := 1; ; n *= 2 {
		start := time.Now()
		for i := 0; i < n; i++ {
			if _, err := Build(claims, cfg); err != nil {
				t.Fatalf("Build: %v", err)
			}
		}
		if total := time.Since(start); total >= buildTimerFloor {
			return total / time.Duration(n)
		}
	}
}
