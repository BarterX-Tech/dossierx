//go:build race

package lint

// lintRaceEnabled reports whether this binary was built with -race. The race
// detector instruments every memory access, which makes an ABSOLUTE wall-clock
// budget say nothing about the lint — while leaving a GROWTH RATIO fully
// meaningful, because a roughly uniform slowdown cancels in a ratio of two
// measurements.
//
// So the sweep runs under -race and the absolute budget does not. This mirrors
// internal/render/markdown's own cost guard, which bounds the same shapes on
// the render side.
const lintRaceEnabled = true

// lintCostTimeScale widens the sweep's per-measurement ceiling under
// instrumentation so a legitimately linear shape does not trip it.
const lintCostTimeScale = 8
