// main_test.go owns the package's TestMain for exactly one job: removing the
// once-per-package binary build directory after the run. The build itself is
// lazy (see harness.go's binaryPath) so that `go build ./...` compiles the
// harness without a test context; TestMain only cleans up whatever that
// laziness created.
package procedures

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	code := m.Run()
	if builtBinDir != "" {
		os.RemoveAll(builtBinDir)
	}
	os.Exit(code)
}
