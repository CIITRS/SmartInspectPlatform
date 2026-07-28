package handlers

import "testing"

func TestBloodNormalTableCalibrationUsesFullCenteredColumns(t *testing.T) {
	positions := map[string]ReportPositionValue{}
	applyBloodNormalTableCalibration(positions)

	expectations := map[string]ReportPositionValue{
		"Time1":   {X: 61.06, Y: 90.04, Width: 25.86},
		"Signal1": {X: 86.92, Y: 90.04, Width: 19.40},
		"Trend1":  {X: 106.32, Y: 90.04, Width: 19.72},
		"Type1":   {X: 126.04, Y: 90.04, Width: 30.67},
		"Note1":   {X: 156.71, Y: 90.04, Width: 40.79},
		"Note4":   {X: 156.71, Y: 112.19, Width: 40.79},
	}
	for key, expected := range expectations {
		actual, ok := positions[key]
		if !ok {
			t.Fatalf("missing calibrated position %s", key)
		}
		if actual.X != expected.X || actual.Y != expected.Y || actual.Width != expected.Width {
			t.Errorf("%s = %#v, want x=%v y=%v width=%v", key, actual, expected.X, expected.Y, expected.Width)
		}
		if actual.Align != "center" {
			t.Errorf("%s align = %q, want center", key, actual.Align)
		}
	}
}

func TestNeedsBloodNormalTableCalibrationOnlyMigratesKnownLayouts(t *testing.T) {
	legacy := map[string]ReportPositionValue{
		"Time1": {X: 61.5},
		"Note1": {X: 126.6},
	}
	if !needsBloodNormalTableCalibration(legacy) {
		t.Fatal("legacy layout should be migrated")
	}

	rough := map[string]ReportPositionValue{
		"Time1": {X: 62.0},
		"Note1": {X: 157.5},
	}
	if !needsBloodNormalTableCalibration(rough) {
		t.Fatal("roughly aligned production layout should be migrated")
	}

	custom := map[string]ReportPositionValue{
		"Time1": {X: 60.4},
		"Note1": {X: 158.3},
	}
	if needsBloodNormalTableCalibration(custom) {
		t.Fatal("manually tuned layout must not be overwritten")
	}
}
