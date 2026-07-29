//go:build race

package markdown

// raceEnabled reports whether this binary was built with -race. The race
// detector instruments every memory access, which measured ~41x slower on the
// heaviest shape in the cost sweep (19.7ms became 812ms). That multiplier makes
// an ABSOLUTE wall-clock budget meaningless — it says nothing about the
// renderer — while leaving a GROWTH RATIO fully meaningful, because a roughly
// uniform slowdown cancels in a ratio of two measurements.
//
// So the sweep runs under -race and the absolute budget does not. See
// TestRender_CostAtOneMiBIsBounded and costMeasurementCeiling.
const raceEnabled = true

// costTimeScale widens the sweep's per-measurement ceiling under
// instrumentation so a legitimately linear shape does not trip it.
const costTimeScale = 8
