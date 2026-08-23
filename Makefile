# Canonical build entrypoint for the docs engine.
#
# -trimpath is required, not cosmetic: without it, "go build" embeds the
# full absolute path of the build machine's module directory into the
# resulting binary (visible via `strings` on the binary, and in panic/stack
# traces). That leaks local filesystem layout and defeats the portability
# bar this engine is held to (see tests/portability_test.go, row 4).
BINARY := dossierx
PKG := ./cmd/dossierx

.PHONY: build test viewer-test viewer-lint hook-test

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
# It is not folded into "test" because it needs a real Chrome/Chromium, and what
# happens without one is no longer uniform across the module. The chromedp tests
# SKIP when DOSSIERX_TEST_BROWSER is unset (viewer-tests/harness_test.go:85), the
# right answer on a laptop that has no browser to drive. The release dry run
# (release_build_test.go) does NOT: it FAILS when the `goreleaser` binary is
# unnamed, because "we did not look" must not read as "nothing is wrong".
# A rendered-DOM extraction of site/ used to live here too and is gone with the
# site's build step; the site is static HTML now.
# So this target's exit status is meaningful only with both tools supplied, and
# folding it into "test" would put a laptop's legitimate skips and the release
# dry run's refusals behind one green line.
# CI's "viewer" job runs this against the runner image's Chrome.
# Set DOSSIERX_TEST_BROWSER to point at a specific browser binary, and
# DOSSIERX_TEST_GORELEASER at a `goreleaser` binary for the release dry run.
viewer-test:
	cd viewer-tests && go test -count=1 ./...

# The pre-commit gate is shell driving a real binary against a real git
# repository; scripts/hook-smoke-test.sh is its behavioural suite, and CI runs
# it on all three platforms. It is no longer the ONLY coverage: an earlier
# version of this comment said no Go test could cover the gate, and that
# claim aged into a blind spot — tests/hook_hostile_paths_test.go now drives
# the installer over a hostile-path corpus from inside the root Go suite,
# which `make test` runs and counts, where a shell script's green is not. The PowerShell wrapper has its own
# suite too, scripts/install-git-hook.Tests.ps1 (Pester; CI runs it under
# pwsh on windows-latest — the wrapper used to be executed by no CI job at
# all).
# viewer-tests/ is a separate module, so `golangci-lint run ./...` at the root
# does not read a line of it — the same blind spot `go test ./...` has, and the
# reason viewer-test exists above. CI runs this as a second step of the lint
# job; tests/nested_module_coverage_test.go fails the build if neither exists.
viewer-lint:
	cd viewer-tests && golangci-lint run ./...

hook-test:
	bash scripts/hook-smoke-test.sh
