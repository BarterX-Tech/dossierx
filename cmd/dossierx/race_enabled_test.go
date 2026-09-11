//go:build race

package main

// The race detector changes wall-clock cost; CLI result, allocation, and
// output-size assertions remain active while performance-only timing is omitted.
const dossierxRaceBuildEnabled = true
