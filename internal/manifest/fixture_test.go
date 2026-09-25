package manifest

// minimalYAML and writeMinimal seed a valid empty-surface manifest for this
// package's own tests. Tests elsewhere use internal/manifest/manifesttest,
// which imports this package and so cannot be imported back from here.
func minimalYAML(module string) []byte {
	return []byte("summary: module " + module + " — fixture module context.\nprovides: []\ndepends_on: []\n")
}

func writeMinimal(claimsDir, module string) error {
	return writeFile(claimsDir, module, minimalYAML(module))
}
