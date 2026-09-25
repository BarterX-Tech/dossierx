package main

import (
	"os"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"path/filepath"
	"testing"
)

// fixtureConstitutionYAML is the smallest roof a fixture can carry: one
// invariant, well under the cap. Fixtures that test the roof itself write
// their own.
const fixtureConstitutionYAML = "status: draft\n" +
	"invariants:\n" +
	"  - slug: one-roof\n" +
	"    title: One roof\n" +
	"    body: This fixture has one lockable constitution above every module.\n"

// lockFixtureConstitution gives a fixture project the locked roof the gate
// demands (NIT-26): it writes constitution.yaml beside cfgPath when there is
// none and locks it through the REAL command, so the lock store carries the
// same record a human's `constitution lock` would leave. Without it no claim
// in the fixture can lock and no plain check passes — which is the point of
// the gate, and the fixture cost the ticket accepts.
func lockFixtureConstitution(t *testing.T, cfgPath string) {
	t.Helper()
	path := filepath.Join(filepath.Dir(cfgPath), "constitution.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.WriteFile(path, []byte(fixtureConstitutionYAML), 0o644); err != nil {
			t.Fatalf("write fixture constitution: %v", err)
		}
	}
	env, stderr, err := execCLIJSON(t, "--config", cfgPath, "constitution", "lock", "--reason", "fixture roof")
	if err == nil && env.OK {
		return
	}
	if env.Error != nil {
		switch env.Error.Code {
		case cliout.CodeAlreadyLocked:
			// Idempotent: a builder that composes another builder arms the
			// roof twice, and a roof that is locked and unchanged is done.
			return
		case cliout.CodeInvalidConfig, cliout.CodeConfigNotFound:
			// A fixture whose config cannot load yet (a source_dirs entry
			// written after the config, a deliberately broken config) has
			// no roof to lock; the verb under test refuses at config before
			// the roof matters, and a fixture that needs both arms the roof
			// again once its layout is complete.
			t.Logf("lock fixture constitution: skipped, %s", env.Error.Message)
			return
		}
	}
	t.Fatalf("lock fixture constitution: %v\nenvelope: %+v\nstderr: %s", err, env, stderr)
}
