# testdata/gate-stage3 — answers the join must refuse

One file per way a surface agent's answer can be wrong while still being a
JSON document that parses. `cmd/dossierx/gate_stage3_test.go` reads each of
them, substitutes `<<RUN>>` and `<<FINGERPRINT>>`, and requires the refusal.

**These are the cases that PARSE.** An answer that is not JSON at all is the easy
case and proves the least: it fails at `json.Unmarshal` and no design decision is
involved. What has to be refused deliberately is the answer that decodes cleanly
and is still not something an agent produced — a `FAILED` whose findings never
arrived, a `PASS` that lists what it found, a fingerprint from the tree before
the fix landed, a run identifier belonging to the previous fan-out.

Every fixture is `well-formed.json` with exactly ONE thing changed, so a test
that means to exercise one refusal cannot quietly be starting from a fixture that
broke two. `well-formed.json` itself is asserted to be ACCEPTED, which is what
keeps the other eleven honest.

The two placeholders are substituted by the test rather than baked in, because
the run identifier is minted per run and the fingerprint is stage 2's key for the
surface over the tree under test; a fixture that hard-coded either would be
refused for the wrong reason.
