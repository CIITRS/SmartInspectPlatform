package handlers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveStaleGeneratedReportPDFOnlyDeletesTemporaryReport(t *testing.T) {
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalDir) })

	tempReportDir := filepath.Join("file", "temp", "detect_report")
	if err := os.MkdirAll(tempReportDir, 0o755); err != nil {
		t.Fatal(err)
	}
	stalePath := filepath.Join(tempReportDir, "old-full.pdf")
	if err := os.WriteFile(stalePath, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	outsidePath := filepath.Join(root, "keep.pdf")
	if err := os.WriteFile(outsidePath, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	removeStaleGeneratedReportPDF(stalePath)
	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Fatalf("temporary stale PDF still exists: %v", err)
	}

	removeStaleGeneratedReportPDF(outsidePath)
	if _, err := os.Stat(outsidePath); err != nil {
		t.Fatalf("file outside temporary report directory was removed: %v", err)
	}
}
