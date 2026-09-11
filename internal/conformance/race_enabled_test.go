//go:build race

package conformance

// raceBuildEnabled marks absolute performance assertions that are meaningful
// only in an ordinary production build. Correctness, output-size, and
// allocation bounds remain asserted under -race; the race-only allocation
// ceiling is still checked where instrumentation changes allocation cost.
const raceBuildEnabled = true
