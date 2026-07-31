package handlers

import (
	"strings"
	"testing"
)

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

func TestParsePatientReportAnalysisExtractsStructuredFields(t *testing.T) {
	content := `**报告类型**：检查/检验报告

**内容摘要**：
- **医院**：哈尔滨医科大学附属第二医院
- **日期**：2026年7月26日
- **检查项目**：肺结节三维成像（三维CT）
- 检查所见：右肺见结节影。

**温馨提示**：本总结仅用于帮助阅读原报告，不能替代医生诊断。`
	got := parsePatientReportAnalysis(content)
	if got.ReportType != "检查/检验报告" || got.Hospital != "哈尔滨医科大学附属第二医院" {
		t.Fatalf("unexpected structured fields: %#v", got)
	}
	if got.ExaminationTime != "2026年7月26日" || got.ExaminationItem != "肺结节三维成像（三维CT）" {
		t.Fatalf("unexpected examination fields: %#v", got)
	}
	if strings.Contains(got.Content, "温馨提示") || strings.Contains(got.Content, "不能替代医生诊断") {
		t.Fatalf("disclaimer should be removed: %s", got.Content)
	}
}
