package handlers

import (
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/common/utils"
)

func TestReportPageDefaultsTimeAndTrend(t *testing.T) {
	createdAt := time.Date(2026, 6, 4, 12, 0, 0, 0, time.Local)
	pages := generatePageRenderData(ReportData{
		CreatedAt: createdAt,
		ReportDataMap: map[string]interface{}{
			"time1":  "",
			"trend1": "",
		},
	})
	values := map[string]string{}
	for _, page := range pages {
		elements, _ := page["elements"].([]utils.H)
		for _, element := range elements {
			values[element["key"].(string)] = element["content"].(string)
		}
	}
	if values["Time1"] != "2026-06-04" {
		t.Fatalf("Time1 = %q", values["Time1"])
	}
	if values["Trend1"] != "-" {
		t.Fatalf("Trend1 = %q", values["Trend1"])
	}
}

func TestFormatReportProjectName(t *testing.T) {
	cases := []struct {
		name       string
		project    string
		reportType string
		want       string
	}{
		{name: "high sensitivity", project: "智柯-肠癌", reportType: "normal", want: "智柯-肠癌(MePlex高敏98CpG)"},
		{name: "ultra sensitivity", project: "智朗-肺癌", reportType: "high", want: "智朗-肺癌(MePlex超敏180CpG)"},
		{name: "already formatted", project: "智朗-肺癌(MePlex超敏180CpG)", reportType: "high", want: "智朗-肺癌(MePlex超敏180CpG)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatReportProjectName(tc.project, tc.reportType); got != tc.want {
				t.Fatalf("formatReportProjectName() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestApplyAbsoluteReportPositionDoesNotAddLegacyYOffset(t *testing.T) {
	element := utils.H{
		"key": "Time1",
		"x":   61.5,
		"y":   110.9,
	}
	position := ReportPositionValue{
		X: 61.06, Y: 90.04, Width: 25.86, Height: 6, FontSize: 10,
	}

	applyAbsoluteReportPosition(element, position)

	if got := element["y"]; got != 90.04 {
		t.Fatalf("Time1 y = %v, want absolute configured y 90.04", got)
	}
	if got := element["x"]; got != 61.06 {
		t.Fatalf("Time1 x = %v, want absolute configured x 61.06", got)
	}
}

func TestRemovePreviewElementByKey(t *testing.T) {
	pages := generatePageRenderData(ReportData{Organization: ""})

	removePreviewElementByKey(pages, "Organization")

	for _, page := range pages {
		elements, _ := page["elements"].([]utils.H)
		for _, element := range elements {
			if element["key"] == "Organization" {
				t.Fatal("Organization should be absent from the ultra-sensitive preview")
			}
		}
	}
}
