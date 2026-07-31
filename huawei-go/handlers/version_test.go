package handlers

import (
	"path/filepath"
	"testing"
)

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

func TestSystemUpgradeStatusPersistsAcrossProcessRestart(t *testing.T) {
	statusPath := filepath.Join(t.TempDir(), "upgrade-status.json")
	t.Setenv("SYSTEM_UPGRADE_STATUS_FILE", statusPath)
	want := systemUpgradeStatus{
		Version: "v0.2.4", State: "running", CurrentStep: 4, Progress: 57,
		Message: "正在替换系统文件", StartedAt: "2026-08-01T00:00:00Z",
	}
	if err := writeSystemUpgradeStatus(want); err != nil {
		t.Fatalf("write upgrade status: %v", err)
	}
	got := readSystemUpgradeStatus()
	if got.Version != want.Version || got.State != want.State || got.CurrentStep != want.CurrentStep || got.Progress != want.Progress {
		t.Fatalf("unexpected persisted status: %#v", got)
	}
	if got.TotalSteps != systemUpgradeTotalSteps || got.UpdatedAt == "" {
		t.Fatalf("missing normalized status metadata: %#v", got)
	}
}
