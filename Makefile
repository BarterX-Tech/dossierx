# Canonical build entrypoint for the docs engine.
#
# -trimpath is required, not cosmetic: without it, "go build" embeds the
# full absolute path of the build machine's module directory into the
# resulting binary (visible via `strings` on the binary, and in panic/stack
# traces). That leaks local filesystem layout and defeats the portability
# bar this engine is held to (see tests/portability_test.go, row 4).
BINARY := dossierx
PKG := ./cmd/dossierx

.PHONY: build test viewer-test viewer-lint hook-test ci-evidence release-publish

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

# THE RELEASE DRIVER — the one command that performs the irreversible half of a
# release, and the only thing that authorizes it.
#
# WHY IT IS A TARGET AND NOT A `go test -run` LINE. The driver lives in a
# _test.go file, because every gate symbol it calls does (nothing in the gate is
# compiled into the shipped binary, and a Go package anywhere else reds the
# behaviour-fingerprint meta-test for the whole tree). That means `go test ./...`
# — `make test`, and every lane landing — also runs it, on a maintainer's machine
# with a maintainer's push credentials. So the driver acts only when a human
# NAMED a release, never when its preconditions merely happen to be satisfiable:
# at a lane landing the gate can be green, the ancestry can hold and the trees can
# match, which is exactly why every guard inside the driver would pass.
#
# The authorization is the version typed TWICE. A boolean (`=1`, `=yes`) left in
# a shell profile, a history or a CI secret authorizes every release forever,
# including the next one somebody triggers by accident; a second spelling of the
# version authorizes one release and nothing else.
#
#     make release-publish DOSSIERX_RELEASE_VERSION=vX.Y.Z \
#                          DOSSIERX_RELEASE_AUTHORIZE=vX.Y.Z
#
# WHAT THIS RECIPE ADDS THAT THE DRIVER CANNOT ADD FOR ITSELF, and each one is a
# release it saves:
#
#   -count=1  `go test` caches a successful package result and REPLAYS it, and a
#             subprocess's effects are not tracked inputs. Without this the
#             second invocation for the same version prints `ok (cached)` in
#             under a second and exits 0 having merged nothing, tagged nothing
#             and pushed nothing — and that exit 0 is then read as "the release
#             happened".
#   -timeout  `go test`'s default is ten minutes and the answer to exceeding it
#             is a PANIC. The driver's step between pushing the tag and pushing
#             main waits for the Release workflow to build six GOOS/GOARCH
#             archives and then verifies them, which routinely exceeds ten. On
#             the default the binary panics THERE: tag on the forge, main behind
#             it, and the per-step report naming what is already published never
#             printed.
#   the record refuses to exit 0 over a run that did nothing, the same way
#             ci-evidence does above, and additionally requires the record to
#             name the version this invocation was asked about.
#
# The recipe passes the three variables explicitly because make does not export
# its variables to a recipe's environment. cmd/dossierx/gate_driver_test.go's
# TestTheReleaseInvocationCannotSucceedHavingDoneNothing parses this recipe and
# holds every one of those mechanisms, so no side of it can move alone.
#
# DOSSIERX_RELEASE_RECORD_OUT defaults OUTSIDE the repository, for ci-evidence's
# reason — the release requires `git status --porcelain` to be empty — and for a
# second one: gate/.gitignore ignores everything it does not name, so a record
# written there would be invisible to git, which is the wrong property for
# something a human is meant to find.
DOSSIERX_RELEASE_RECORD_OUT ?= /tmp/dossierx-release-record.json

release-publish:
	@test -n "$(DOSSIERX_RELEASE_VERSION)" || { \
	  echo "DOSSIERX_RELEASE_VERSION is unset, so no release was named and this driver does"     >&2; \
	  echo "nothing. Publishing is authorized by a human running this target for one specific"   >&2; \
	  echo "release, never by the gate's preconditions being satisfiable. Run:"                  >&2; \
	  echo "    make release-publish DOSSIERX_RELEASE_VERSION=vX.Y.Z \\"                         >&2; \
	  echo "                         DOSSIERX_RELEASE_AUTHORIZE=vX.Y.Z"                          >&2; \
	  exit 1; }
	@test -n "$(DOSSIERX_RELEASE_AUTHORIZE)" || { \
	  echo "DOSSIERX_RELEASE_AUTHORIZE is unset. The authorization is the version typed a"       >&2; \
	  echo "second time, and it is deliberately not a boolean: a =1 left in a profile or a"      >&2; \
	  echo "secret authorizes every release forever, including the next one triggered by"        >&2; \
	  echo "accident. Re-run with DOSSIERX_RELEASE_AUTHORIZE=$(DOSSIERX_RELEASE_VERSION)"        >&2; \
	  exit 1; }
	@rm -f "$(DOSSIERX_RELEASE_RECORD_OUT)"
	DOSSIERX_RELEASE_VERSION="$(DOSSIERX_RELEASE_VERSION)" \
	DOSSIERX_RELEASE_AUTHORIZE="$(DOSSIERX_RELEASE_AUTHORIZE)" \
	DOSSIERX_RELEASE_RECORD_OUT="$(DOSSIERX_RELEASE_RECORD_OUT)" \
	go test -count=1 -timeout 90m -run '^TestReleaseDriverPublishes$$' ./cmd/dossierx/
	@test -s "$(DOSSIERX_RELEASE_RECORD_OUT)" || { \
	  echo "the driver exited 0 without writing a run record to"                                 >&2; \
	  echo "$(DOSSIERX_RELEASE_RECORD_OUT), so it published NOTHING and this is a FAILED"        >&2; \
	  echo "release, not a completed one. go test exits 0 for a cached result, for a skip and"   >&2; \
	  echo "for a selector that matches nothing, and all three look exactly like success."       >&2; \
	  exit 1; }
	@grep -q "$(DOSSIERX_RELEASE_VERSION)" "$(DOSSIERX_RELEASE_RECORD_OUT)" || { \
	  echo "the run record does not name $(DOSSIERX_RELEASE_VERSION), so it is the record of"    >&2; \
	  echo "some other release and says nothing about the one just asked for."                   >&2; \
	  exit 1; }
	@cat "$(DOSSIERX_RELEASE_RECORD_OUT)"
