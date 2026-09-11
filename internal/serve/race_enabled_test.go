//go:build race

package serve_test

// The race detector changes wall-clock cost; response, graph, size, and
// allocation assertions remain active while performance-only timing is omitted.
const serveRaceBuildEnabled = true
