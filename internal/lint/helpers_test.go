package lint

import "testing"

// findingClaimIDs and assertStringSlicesEqual are small shared test
// helpers used across the lint package's table-driven tests.
func findingClaimIDs(findings []Finding) []string {
	if len(findings) == 0 {
		return nil
	}
	ids := make([]string, len(findings))
	for i, f := range findings {
		ids[i] = f.ClaimID
	}
	return ids
}

func assertStringSlicesEqual(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
