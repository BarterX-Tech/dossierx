package loader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestParseClaim_LinksNoneAndStepsOwnedBy(t *testing.T) {
	dir := t.TempDir()
	none := "" +
		"id: widget.contract.boundary\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
		"body: a refusal\ngoverned_by:\n  type: none\n  reason: fixture\n" +
		"links:\n  mode: none\n  reason: no implementing declaration\n"
	if err := os.WriteFile(filepath.Join(dir, "none.yaml"), []byte(none), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := ParseClaim([]byte(none), filepath.Join(dir, "none.yaml"))
	if err != nil {
		t.Fatalf("parse none: %v", err)
	}
	if !c.LinksNone() || c.Links.Reason != "no implementing declaration" {
		t.Fatalf("links: %+v", c.Links)
	}

	owned := "" +
		"id: widget.contract.walk\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: steps\n" +
		"body: walkthrough\nsteps:\n  - freeze the subject\n  - run the suite\n" +
		"governed_by:\n  type: none\n  reason: fixture\n" +
		"steps_owned_by:\n  1: process\n"
	c, err = ParseClaim([]byte(owned), filepath.Join(dir, "owned.yaml"))
	if err != nil {
		t.Fatalf("parse owned: %v", err)
	}
	if got := c.ProcessOwnedSteps(); len(got) != 1 || got[0] != 1 {
		t.Fatalf("owned steps = %v", got)
	}
}

func TestParseClaim_LinksUnknownField(t *testing.T) {
	raw := "" +
		"id: widget.contract.boundary\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
		"body: a refusal\ngoverned_by:\n  type: none\n  reason: fixture\n" +
		"links:\n  mode: none\n  reason: none\n  invented: true\n"
	_, err := ParseClaim([]byte(raw), "claim.yaml")
	if err == nil || !strings.Contains(err.Error(), "invented") {
		t.Fatalf("expected unknown field, got %v", err)
	}
}

func TestParseClaim_LinksNoneWithoutReason(t *testing.T) {
	raw := "" +
		"id: widget.contract.boundary\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
		"body: a refusal\ngoverned_by:\n  type: none\n  reason: fixture\n" +
		"links:\n  mode: none\n"
	_, err := ParseClaim([]byte(raw), "claim.yaml")
	if err == nil || !strings.Contains(err.Error(), "reason") {
		t.Fatalf("expected reason, got %v", err)
	}
	_ = model.LinksModeNone
}
