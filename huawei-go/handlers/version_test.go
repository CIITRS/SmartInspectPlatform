package handlers

import "testing"

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name        string
		left, right string
		want        int
	}{
		{name: "equal short and patch", left: "v0.1", right: "v0.1.0", want: 0},
		{name: "patch update", left: "v0.1.0", right: "v0.1.1", want: -1},
		{name: "minor update", left: "v0.1.9", right: "v0.2.0", want: -1},
		{name: "major newer", left: "v2.0.0", right: "v1.9.9", want: 1},
		{name: "prerelease numeric core", left: "v1.2.0-beta.1", right: "v1.2.0", want: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := compareVersions(test.left, test.right); got != test.want {
				t.Fatalf("compareVersions(%q, %q) = %d; want %d", test.left, test.right, got, test.want)
			}
		})
	}
}

func TestReleaseTagPattern(t *testing.T) {
	valid := []string{"v0.1", "v0.1.0", "v12.34.56", "v1.2.3-rc.1"}
	for _, tag := range valid {
		if !releaseTagPattern.MatchString(tag) {
			t.Errorf("expected %q to be valid", tag)
		}
	}
	invalid := []string{"0.1.0", "v1", "main", "v1.2.3; rm -rf /"}
	for _, tag := range invalid {
		if releaseTagPattern.MatchString(tag) {
			t.Errorf("expected %q to be invalid", tag)
		}
	}
}
