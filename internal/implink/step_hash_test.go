package implink

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// Curtainly mac adapters hash `" ".join(text.split())` UTF-8. These vectors
// were produced with that Python one-liner so the engine stays on the same
// digest the app already tags.
func TestStepContentHash_MatchesMacWhitespaceNormalisation(t *testing.T) {
	const wantHelloWorld = "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	const wantEmpty = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	if got := StepContentHash("hello world"); got != wantHelloWorld {
		t.Fatalf("plain text: got %s want %s", got, wantHelloWorld)
	}
	if got := StepContentHash("hello\n\tworld  "); got != wantHelloWorld {
		t.Fatalf("wrapped text must not change the digest: got %s want %s", got, wantHelloWorld)
	}
	if got := StepContentHash("  hello   world"); got != wantHelloWorld {
		t.Fatalf("extra spaces must not change the digest: got %s want %s", got, wantHelloWorld)
	}
	if got := StepContentHash(""); got != wantEmpty {
		t.Fatalf("empty: got %s want %s", got, wantEmpty)
	}
	if got := StepContentHash(" \n\t"); got != wantEmpty {
		t.Fatalf("whitespace-only must hash as empty: got %s want %s", got, wantEmpty)
	}
}

func TestStepContentHash_RawWrappingIsNotTheDigest(t *testing.T) {
	raw := "hello\nworld"
	rawSum := sha256.Sum256([]byte(raw))
	rawHex := hex.EncodeToString(rawSum[:])
	got := StepContentHash(raw)
	if got == rawHex {
		t.Fatal("normalised digest must differ from a raw hash of text that still has a newline")
	}
	if got != StepContentHash("hello world") {
		t.Fatalf("newline wrap must match the one-line form, got %s", got)
	}
}

func TestStepHashMatches_PrefixContract(t *testing.T) {
	want := StepContentHash("hello world")
	if !StepHashMatches(want, want) {
		t.Fatal("full 64-hex must match")
	}
	if !StepHashMatches(want[:StepHashPreferredPrefixLen], want) {
		t.Fatal("12-hex preferred prefix must match")
	}
	if !StepHashMatches(want[:StepHashMinPrefixLen], want) {
		t.Fatal("8-hex minimum prefix must match")
	}
	if !StepHashMatches("B94D27B9934D", want) {
		t.Fatal("uppercase hex must match")
	}
	if StepHashMatches(want[:StepHashMinPrefixLen-1], want) {
		t.Fatal("7-hex must not match")
	}
	if StepHashMatches(want[:12]+"0", want) {
		t.Fatal("a longer string that is not a prefix must not match")
	}
	if StepHashMatches("00000000", want) {
		t.Fatal("wrong prefix must not match")
	}
}
