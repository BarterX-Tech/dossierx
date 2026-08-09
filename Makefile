# Canonical build entrypoint for the docs engine.
#
# -trimpath is required, not cosmetic: without it, "go build" embeds the
# full absolute path of the build machine's module directory into the
# resulting binary (visible via `strings` on the binary, and in panic/stack
# traces). That leaks local filesystem layout and defeats the portability
# bar this engine is held to (see tests/portability_test.go, row 4).
BINARY := dossierx
PKG := ./cmd/dossierx

.PHONY: build test viewer-test viewer-lint hook-test ci-evidence

build:
	go build -trimpath -o bin/$(BINARY) $(PKG)

test:
	go build ./...
	go vet ./...
	go test ./...

# The browser suite lives in its own module (viewer-tests/go.mod requires
# chromedp) precisely so chromedp cannot leak into the engine's go.mod, which
# stays cobra + yaml.v3. That isolation means the "test" target above CANNOT
# reach it — "go test ./..." does not descend into a nested module — so it needs
# a target of its own or it is code nobody can run without knowing it is there.
# It is not folded into "test" because it needs a real Chrome/Chromium: without
# one the suite t.Skip()s, and a skip inside "make test" would read as a pass.
# CI's "viewer" job runs this against the runner image's Chrome.
# Set DOSSIERX_TEST_BROWSER to point at a specific browser binary.
viewer-test:
	cd viewer-tests && go test -count=1 ./...

# The pre-commit gate is shell driving a real binary against a real git
# repository, so no Go test can cover it; scripts/hook-smoke-test.sh is its
# suite, and CI runs it on all three platforms.
# viewer-tests/ is a separate module, so `golangci-lint run ./...` at the root
# does not read a line of it — the same blind spot `go test ./...` has, and the
# reason viewer-test exists above. CI runs this as a second step of the lint
# job; tests/nested_module_coverage_test.go fails the build if neither exists.
viewer-lint:
	cd viewer-tests && golangci-lint run ./...

hook-test:
	bash scripts/hook-smoke-test.sh

# THE RELEASE GATE'S CI-RUN EVIDENCE, and the reason it is a make target rather
# than a `go test -run` line somebody types from memory.
#
# The stage it invokes fetches the CI run for one commit, parses the `go test
# -json` account every declared suite emitted, and refuses a release whose
# packages passed having executed nothing. It lives in a _test.go, so `go test`
# is what runs it — and `go test` EXITS 0 for a skipped test and EXITS 0 for a
# `-run` selector that matches nothing, printing `ok … [no tests to run]` for
# the second. So an exit status alone cannot tell "adjudicated and cleared" from
# "adjudicated nothing", and an invocation whose selector drifted by one
# character would report a clean gate for every release after, forever.
#
# What closes that is the RECORD, not the exit status. The stage writes a
# verdict record naming the commit it examined, the suites it derived, every
# matrix instantiation and what each one accounted for; the two `test` lines
# below refuse to succeed unless that record exists and names the sha this run
# was asked about. An invocation that examined nothing is then as loud as a
# suite that ran nothing, which is the whole subject of the check applied to
# itself. tests/ci_run_evidence_test.go's TestTheReleaseTimeInvocationNamesThisStage
# holds this recipe, docs/RELEASING.md and .claude/workflows/release-checklist.js
# to the same identifiers, so no one side of it can move alone.
#
#     make ci-evidence DOSSIERX_GATE_CI_SHA=<the merge commit's full sha>
#
# DOSSIERX_GATE_CI_EVIDENCE_OUT may be overridden to put the record elsewhere.
# It defaults OUTSIDE the repository on purpose: the pre-tag phase also requires
# `git status --porcelain` to be empty, and a gate that leaves an untracked file
# at the repository root every time it runs would either fail the next check or
# get committed by somebody clearing it.
DOSSIERX_GATE_CI_EVIDENCE_OUT ?= /tmp/dossierx-ci-run-evidence.json

ci-evidence:
	@test -n "$(DOSSIERX_GATE_CI_SHA)" || { \
	  echo "DOSSIERX_GATE_CI_SHA is unset. This gate is keyed to the MERGE COMMIT, never to"     >&2; \
	  echo "HEAD or to the newest run on main: the content.ts sha stamp lands on main after the" >&2; \
	  echo "merge as a matter of routine, so a run fetched any other way is evidence about a"    >&2; \
	  echo "tree that is not the one being tagged. Run:"                                          >&2; \
	  echo "    make ci-evidence DOSSIERX_GATE_CI_SHA=\$$(git log --merges -1 --format=%H main)" >&2; \
	  exit 1; }
	@rm -f "$(DOSSIERX_GATE_CI_EVIDENCE_OUT)"
	DOSSIERX_GATE_CI_SHA="$(DOSSIERX_GATE_CI_SHA)" \
	DOSSIERX_GATE_CI_EVIDENCE_OUT="$(DOSSIERX_GATE_CI_EVIDENCE_OUT)" \
	go test -count=1 -run '^TestReleaseGateCIRunEvidence$$' ./tests/
	@test -s "$(DOSSIERX_GATE_CI_EVIDENCE_OUT)" || { \
	  echo "the stage exited 0 without writing a verdict record to"                              >&2; \
	  echo "$(DOSSIERX_GATE_CI_EVIDENCE_OUT), so it adjudicated NOTHING and this is a FAILED"    >&2; \
	  echo "gate, not a clean one. go test exits 0 for a skip and for a selector that matches"   >&2; \
	  echo "nothing, so that is what an invocation drifted away from its stage looks like."      >&2; \
	  exit 1; }
	@grep -q "$(DOSSIERX_GATE_CI_SHA)" "$(DOSSIERX_GATE_CI_EVIDENCE_OUT)" || { \
	  echo "the verdict record does not name $(DOSSIERX_GATE_CI_SHA), so it is a record of"      >&2; \
	  echo "some other adjudication and says nothing about the commit about to be tagged."       >&2; \
	  exit 1; }
	@cat "$(DOSSIERX_GATE_CI_EVIDENCE_OUT)"
