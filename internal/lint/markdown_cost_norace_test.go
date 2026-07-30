//go:build !race

package lint

// lintRaceEnabled reports whether this binary was built with -race. See the
// //go:build race twin of this file for why the cost guard cares.
const lintRaceEnabled = false

// lintCostTimeScale is 1 in an ordinary build: the measured numbers are the
// real ones and the ceilings apply as written.
const lintCostTimeScale = 1
