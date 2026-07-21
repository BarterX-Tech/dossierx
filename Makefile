# Canonical build entrypoint for the docs engine.
#
# -trimpath is required, not cosmetic: without it, "go build" embeds the
# full absolute path of the build machine's module directory into the
# resulting binary (visible via `strings` on the binary, and in panic/stack
# traces). That leaks local filesystem layout and defeats the portability
# bar this engine is held to (see tests/portability_test.go, row 4).
BINARY := dossierx
PKG := ./cmd/dossierx

.PHONY: build test

build:
	go build -trimpath -o bin/$(BINARY) $(PKG)

test:
	go build ./...
	go vet ./...
	go test ./...
