//go:build race

package readiness_test

// The race detector changes wall-clock cost; correctness and allocation/output
// bounds remain active while performance-only timing is omitted.
const readinessRaceBuildEnabled = true
