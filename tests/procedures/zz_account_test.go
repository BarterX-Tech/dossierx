// zz_account_test.go is the suite's cross-check over itself: after every
// scenario has run, re-verify that each one executed the complete plan it
// declared. The zz_ prefix is load-bearing — Go runs a package's tests in
// file-name order, so this file runs after every scenario file.
//
// Each scenario already verifies its own account in a t.Cleanup, which runs on
// every exit path including t.Fatal. This test exists for the failure mode
// that per-scenario cleanup cannot see: a future edit that removes or bypasses
// the cleanup itself, or a helper change that stops recording. Re-reading the
// shared ledger from a different test means a scenario cannot quietly become
// one that "passes" over fewer steps than it claims — the exact shape of skip
// this suite exists to refuse (a pass over a prefix of a procedure asserts
// nothing about the rest of it).
//
// This is a check over whatever DID register, not a roll call of scenario
// names: hardcoding the expected scenario list here would rot the day a
// scenario is added, and a scenario deleted outright is visible in review in a
// way a scenario silently truncated is not. When this package runs with a
// -run filter that selects no scenario, the registry is legitimately empty
// and there is nothing to cross-check — that is the one case the emptiness
// guard permits, detected by the filtered scenarios' absence, not assumed.
package procedures

import (
	"testing"
)

func TestZZEveryScenarioExecutedEverythingItDeclared(t *testing.T) {
	records := snapshotRegistry()
	if len(records) == 0 {
		// Under `go test ./tests/procedures` (no filter) at least one scenario
		// always registers, so an empty registry on a full run means recording
		// itself broke — fail, never shrug. A -run filter that excludes every
		// scenario also excludes this test in practice only if the caller asked
		// for it by name; in that case the emptiness is the caller's selection,
		// but there is still nothing true to assert, so say so and fail rather
		// than pass over zero scenarios.
		t.Fatal("no scenario registered a plan; either recording broke or this test was run in isolation — a cross-check over zero scenarios is a pass over zero assertions, which this suite refuses")
	}
	for i := range records {
		rec := records[i]
		t.Run(rec.Name, func(t *testing.T) {
			verifyAccount(t, &rec)
		})
	}
}
