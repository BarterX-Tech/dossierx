package check

import "github.com/BarterX-Tech/dossierx/internal/model"

// DecodeClaimForTest exposes decodeClaim to the external check_test package.
//
// It is test-only on purpose. decodeClaim is a duplicate of internal/loader's
// per-file decode — necessary because index content has no path to read from —
// and the ONE thing that must be true of it is that it agrees with the original
// (see TestStagedDecodeMatchesLoader). Exporting it for real would invite a
// second production caller, and a second caller is how a duplicate stops being
// a duplicate and starts being a divergence.
func DecodeClaimForTest(sourcePath string, raw []byte) (model.Claim, error) {
	return decodeClaim(sourcePath, raw)
}
