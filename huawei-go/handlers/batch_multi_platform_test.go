package handlers

import "testing"

func TestChooseCancerTypeForSamplePrefersRegisteredCancerType(t *testing.T) {
	cancerTypes := []cancerTypePanelInfo{
		{ID: 10, Name: "早筛检查", PanelIDs: []int{3, 5, 6}},
		{ID: 12, Name: "智柯-肠癌", PanelIDs: []int{4}},
	}

	got, ok := chooseCancerTypeForSample(cancerTypes, []int{3, 5, 6}, 12)
	if !ok {
		t.Fatal("expected a cancer type match")
	}
	if got.ID != 12 {
		t.Fatalf("matched cancer type ID = %d, want registered sample cancer type 12", got.ID)
	}
}

func TestChooseCancerTypeForSampleFallsBackToPanels(t *testing.T) {
	cancerTypes := []cancerTypePanelInfo{
		{ID: 10, Name: "早筛检查", PanelIDs: []int{3, 5, 6}},
		{ID: 12, Name: "智柯-肠癌", PanelIDs: []int{4}},
	}

	got, ok := chooseCancerTypeForSample(cancerTypes, []int{3, 5, 6}, 0)
	if !ok || got.ID != 10 {
		t.Fatalf("panel fallback = (%d, %v), want early screening ID 10", got.ID, ok)
	}
}
