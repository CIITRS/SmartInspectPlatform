package handlers

import "testing"

func TestPatientReportFileKeyIsStableAndURLSpecific(t *testing.T) {
	first := patientReportFileKey("https://example.com/report/a.png")
	second := patientReportFileKey("https://example.com/report/a.png")
	other := patientReportFileKey("https://example.com/report/b.png")
	if len(first) != 64 {
		t.Fatalf("expected SHA-256 hex key, got length %d", len(first))
	}
	if first != second {
		t.Fatal("same report URL should produce the same key")
	}
	if first == other {
		t.Fatal("different report URLs should produce different keys")
	}
}

func TestPatientReportFileNameDecodesURLPath(t *testing.T) {
	got := patientReportFileName("https://example.com/uploads/%E6%A3%80%E6%9F%A5%E6%8A%A5%E5%91%8A.png?e=1")
	if got != "检查报告.png" {
		t.Fatalf("unexpected report filename: %s", got)
	}
}
