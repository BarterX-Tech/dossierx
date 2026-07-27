# Canonical build entrypoint for the docs engine.
#
# -trimpath is required, not cosmetic: without it, "go build" embeds the
# full absolute path of the build machine's module directory into the
# resulting binary (visible via `strings` on the binary, and in panic/stack
# traces). That leaks local filesystem layout and defeats the portability
# bar this engine is held to (see tests/portability_test.go, row 4).
BINARY := dossierx
PKG := ./cmd/dossierx

.PHONY: build test viewer-test hook-test

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
hook-test:
	bash scripts/hook-smoke-test.sh
