package handlers

import "testing"

func TestValidPatientSampleCode(t *testing.T) {
	tests := map[string]bool{
		"BOX-2026-0001": true,
		" 管码001 ":       true,
		"":              false,
		"BOX 0001":      false,
		"BOX\n0001":     false,
	}
	for value, want := range tests {
		if got := validPatientSampleCode(value); got != want {
			t.Errorf("validPatientSampleCode(%q) = %v, want %v", value, got, want)
		}
	}
}
