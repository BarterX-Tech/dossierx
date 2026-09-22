package model

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestValidateCodeLinkDeclarations_NoneNeedsReason(t *testing.T) {
	err := ValidateCodeLinkDeclarations(&Claim{Links: &ClaimLinks{Mode: LinksModeNone}})
	if err == nil || !strings.Contains(err.Error(), "reason") {
		t.Fatalf("expected a reason refusal, got %v", err)
	}
}

func TestValidateCodeLinkDeclarations_UnknownMode(t *testing.T) {
	err := ValidateCodeLinkDeclarations(&Claim{Links: &ClaimLinks{Mode: "files", Reason: "x"}})
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected unsupported mode, got %v", err)
	}
}

func TestValidateCodeLinkDeclarations_StepsOwnedByNeedsSteps(t *testing.T) {
	err := ValidateCodeLinkDeclarations(&Claim{StepsOwnedBy: StepsOwnedBy{1: StepOwnerProcess}})
	if err == nil || !strings.Contains(err.Error(), "requires steps") {
		t.Fatalf("expected requires steps, got %v", err)
	}
}

func TestValidateCodeLinkDeclarations_OutOfRangeAndUnknownOwner(t *testing.T) {
	c := Claim{Steps: []string{"one"}, StepsOwnedBy: StepsOwnedBy{2: StepOwnerProcess}}
	if err := ValidateCodeLinkDeclarations(&c); err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("expected out of range, got %v", err)
	}
	c = Claim{Steps: []string{"one"}, StepsOwnedBy: StepsOwnedBy{1: "machine"}}
	if err := ValidateCodeLinkDeclarations(&c); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected unsupported owner, got %v", err)
	}
}

func TestValidateCodeLinkDeclarations_CannotCombineNoneAndOwned(t *testing.T) {
	c := Claim{
		Steps:        []string{"one"},
		Links:        &ClaimLinks{Mode: LinksModeNone, Reason: "doctrine"},
		StepsOwnedBy: StepsOwnedBy{1: StepOwnerProcess},
	}
	if err := ValidateCodeLinkDeclarations(&c); err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("expected combination refusal, got %v", err)
	}
}

func TestValidateCodeLinkDeclarations_ValidNoneAndProcess(t *testing.T) {
	none := Claim{Links: &ClaimLinks{Mode: LinksModeNone, Reason: "boundary"}}
	if err := ValidateCodeLinkDeclarations(&none); err != nil {
		t.Fatalf("valid none: %v", err)
	}
	if !none.LinksNone() {
		t.Fatal("LinksNone")
	}
	owned := Claim{Steps: []string{"a", "b"}, StepsOwnedBy: StepsOwnedBy{2: StepOwnerProcess}}
	if err := ValidateCodeLinkDeclarations(&owned); err != nil {
		t.Fatalf("valid owned: %v", err)
	}
	got := owned.ProcessOwnedSteps()
	if len(got) != 1 || got[0] != 2 {
		t.Fatalf("ProcessOwnedSteps = %v", got)
	}
}

func TestStepsOwnedByUnmarshal_IntegerKeys(t *testing.T) {
	var owned StepsOwnedBy
	if err := yaml.Unmarshal([]byte("1: process\n3: process\n"), &owned); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if owned[1] != StepOwnerProcess || owned[3] != StepOwnerProcess {
		t.Fatalf("got %#v", owned)
	}
}
