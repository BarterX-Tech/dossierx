//go:build !race

package markdown

// raceEnabled reports whether this binary was built with -race. See the
// //go:build race twin of this file for why the cost guards care.
const raceEnabled = false

// costTimeScale is 1 in an ordinary build: the measured numbers are the real
// ones and the ceilings apply as written.
const costTimeScale = 1
