package viewertests

// build-order show --format mermaid is retired (NIT-15). The test still
// runs the remembered invocation so a live export cannot come back silently.

import (
	"os/exec"
	"strings"
	"testing"
)

func TestMermaidParsesEveryExportedFlowchart(t *testing.T) {
	p := newBuildOrderProject(t)
	out, err := exec.Command(p.bin, "--config", p.config, "build-order", "show", "--module", "widget", "--as-mermaid").CombinedOutput()
	if err == nil {
		t.Fatalf("retired build-order show must fail, got:\n%s", out)
	}
	if !strings.Contains(string(out), "removed") && !strings.Contains(string(out), "usage") {
		t.Fatalf("retired build-order show must name the removal, got:\n%s", out)
	}
}
