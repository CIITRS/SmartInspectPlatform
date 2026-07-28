package handlers

import "testing"

func TestReportTypeFullLabel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "normal", want: "高敏（MePlex高敏98CpG）"},
		{input: "高敏", want: "高敏（MePlex高敏98CpG）"},
		{input: "high", want: "超敏（MePlex超敏180CpG）"},
		{input: "超敏", want: "超敏（MePlex超敏180CpG）"},
	}
	for _, test := range tests {
		if got := reportTypeFullLabel(test.input); got != test.want {
			t.Fatalf("reportTypeFullLabel(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

func TestBuildSampleCode(t *testing.T) {
	if got := buildSampleCode("25A01", 7); got != "25A010007" {
		t.Fatalf("buildSampleCode returned %q", got)
	}
}
