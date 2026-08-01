package handlers

import (
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestManagedTemporaryReportLifecycle(t *testing.T) {
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
	reportPath := filepath.Join(tempReportDir, "download.zip")
	if err := os.WriteFile(reportPath, []byte("report"), 0o600); err != nil {
		t.Fatal(err)
	}
	outsidePath := filepath.Join(root, "keep.zip")
	if err := os.WriteFile(outsidePath, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	scheduleManagedTemporaryFileRemovalAfter(reportPath, 10*time.Millisecond)
	scheduleManagedTemporaryFileRemovalAfter(outsidePath, 10*time.Millisecond)
	deadline := time.Now().Add(time.Second)
	for {
		_, statErr := os.Stat(reportPath)
		if os.IsNotExist(statErr) || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if _, err := os.Stat(reportPath); !os.IsNotExist(err) {
		t.Fatalf("managed temporary report still exists: %v", err)
	}
	if _, err := os.Stat(outsidePath); err != nil {
		t.Fatalf("file outside managed temporary directories was removed: %v", err)
	}
}

func TestCleanupManagedTemporaryFilesRemovesOnlyExpiredFiles(t *testing.T) {
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalDir) })

	directory := filepath.Join("file", "temp", "detect_report_preview")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(directory, "old.png")
	newPath := filepath.Join(directory, "new.png")
	for _, path := range []string{oldPath, newPath} {
		if err := os.WriteFile(path, []byte("preview"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	oldTime := time.Now().Add(-time.Hour)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	cleanupManagedTemporaryFiles(30 * time.Minute)
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("expired preview still exists: %v", err)
	}
	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("fresh preview was removed: %v", err)
	}
}
