//go:build race

package readiness

// The race detector changes wall-clock cost, so performance-only bounds are
// omitted while graph correctness and allocation/output bounds remain tested.
const readinessInternalRaceBuildEnabled = true
