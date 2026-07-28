package handlers

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joho/godotenv"
)

func TestGenerateQiniuUploadToken(t *testing.T) {
	config := QiniuStorageConfig{
		Enabled:   true,
		AccessKey: "test-access",
		SecretKey: "test-secret",
		Bucket:    "test-bucket",
		Domain:    "https://files.example.com",
		UploadURL: "https://upload.qiniup.com",
		TokenTTL:  time.Hour,
	}
	now := time.Unix(1_700_000_000, 0)
	token, deadline, err := generateQiniuUploadToken(config, "uploads/report.pdf", now)
	if err != nil {
		t.Fatalf("generateQiniuUploadToken() error = %v", err)
	}
	parts := strings.Split(token, ":")
	if len(parts) != 3 || parts[0] != config.AccessKey {
		t.Fatalf("unexpected token format: %q", token)
	}
	policyJSON, err := base64.URLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("decode policy: %v", err)
	}
	var policy map[string]interface{}
	if err := json.Unmarshal(policyJSON, &policy); err != nil {
		t.Fatalf("unmarshal policy: %v", err)
	}
	if policy["scope"] != "test-bucket:uploads/report.pdf" {
		t.Fatalf("scope = %v", policy["scope"])
	}
	if deadline != now.Add(time.Hour).Unix() || int64(policy["deadline"].(float64)) != deadline {
		t.Fatalf("deadline = %d, policy deadline = %v", deadline, policy["deadline"])
	}
	mac := hmac.New(sha1.New, []byte(config.SecretKey))
	_, _ = mac.Write([]byte(parts[2]))
	if parts[1] != base64.URLEncoding.EncodeToString(mac.Sum(nil)) {
		t.Fatal("signature does not match HMAC-SHA1 policy signature")
	}
}

func TestGenerateQiniuManagementToken(t *testing.T) {
	config := QiniuStorageConfig{AccessKey: "test-access", SecretKey: "test-secret"}
	got := generateQiniuManagementToken(
		config,
		"GET",
		"rsf.qiniuapi.com",
		"/list",
		"bucket=test-bucket&limit=1",
		"application/x-www-form-urlencoded",
		"20260729T010203Z",
	)
	signingText := "GET /list?bucket=test-bucket&limit=1\n" +
		"Host: rsf.qiniuapi.com\n" +
		"Content-Type: application/x-www-form-urlencoded\n" +
		"X-Qiniu-Date: 20260729T010203Z\n\n"
	mac := hmac.New(sha1.New, []byte(config.SecretKey))
	_, _ = mac.Write([]byte(signingText))
	want := config.AccessKey + ":" + base64.URLEncoding.EncodeToString(mac.Sum(nil))
	if got != want {
		t.Fatalf("management token = %q, want %q", got, want)
	}
}

func TestQiniuListIntegration(t *testing.T) {
	if os.Getenv("QINIU_INTEGRATION") != "1" {
		t.Skip("set QINIU_INTEGRATION=1 to verify configured bucket access")
	}
	_ = godotenv.Load("../.env", ".env")
	config := loadQiniuStorageConfig()
	if !config.configured() {
		t.Fatal("Qiniu integration configuration is incomplete")
	}
	if _, err := listQiniuObjectsPage(config, "", "/", "", 1); err != nil {
		t.Fatalf("list configured Qiniu bucket: %v", err)
	}
	totalFiles, totalBytes, truncated, err := getQiniuStorageUsage(config)
	if err != nil {
		t.Fatalf("read configured Qiniu bucket usage: %v", err)
	}
	t.Logf("Qiniu usage verified: files=%d bytes=%d truncated=%v", totalFiles, totalBytes, truncated)
}

func TestCalculateReportTrendUsesOnePointTolerance(t *testing.T) {
	tests := []struct {
		name              string
		current, previous float64
		want              string
	}{
		{name: "equal", current: 10, previous: 10, want: "-"},
		{name: "difference below one", current: 10.9, previous: 10, want: "-"},
		{name: "negative difference below one", current: 9.1, previous: 10, want: "-"},
		{name: "increase at boundary", current: 11, previous: 10, want: "↑"},
		{name: "decrease at boundary", current: 9, previous: 10, want: "↓"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := calculateReportTrend(test.current, test.previous); got != test.want {
				t.Fatalf("calculateReportTrend(%v, %v) = %q, want %q", test.current, test.previous, got, test.want)
			}
		})
	}
}

func TestBuildPatientReportObjectName(t *testing.T) {
	uploadedAt := time.Date(2026, time.July, 28, 16, 20, 18, 0, time.Local)
	got := buildPatientReportObjectName("hw2500001", "old.JPG", uploadedAt, 1)
	want := "uploads/patient_report/HW2500001/HW2500001_20260728162018_report01.jpg"
	if got != want {
		t.Fatalf("object name = %q, want %q", got, want)
	}
}

func TestPatientReportUploadTimeFromLegacyFilename(t *testing.T) {
	got := patientReportUploadTime(
		"https://bgpt.huaweibio.com.cn/uploads/20662/95293ecc0e2152953196d3dd4d4b5ae1_1785222018.jpg",
		time.Time{},
	)
	if got.Unix() != 1785222018 {
		t.Fatalf("upload unix time = %d, want 1785222018", got.Unix())
	}
}

func TestNextPatientReportSequenceUsesLargestExistingNumber(t *testing.T) {
	files := strings.Join([]string{
		"https://files.example.com/uploads/patient_report/HWP1/HWP1_20260728120000_report01.pdf",
		"https://files.example.com/uploads/patient_report/HWP1/HWP1_20260728130000_report03.jpg",
	}, ",")
	if got := nextPatientReportSequenceFromFiles(files); got != 4 {
		t.Fatalf("next sequence = %d, want 4", got)
	}
}

func TestQiniuObjectNameFromURL(t *testing.T) {
	config := QiniuStorageConfig{Domain: "https://bucket01.huaweibio.com.cn"}
	got, ok := qiniuObjectNameFromURL(config,
		"https://bucket01.huaweibio.com.cn/uploads/patient_report/HWP1/HWP1_20260728130000_report02.pdf")
	if !ok || got != "uploads/patient_report/HWP1/HWP1_20260728130000_report02.pdf" {
		t.Fatalf("object name = %q, ok = %v", got, ok)
	}
}

func TestLegacyPatientReportLocalPathRejectsTraversal(t *testing.T) {
	if _, ok := legacyPatientReportLocalPath("https://bgpt.huaweibio.com.cn/uploads/../private.pem"); ok {
		t.Fatal("expected traversal path to be rejected")
	}
	path, ok := legacyPatientReportLocalPath("https://bgpt.huaweibio.com.cn/uploads/20662/report.jpg")
	if !ok || !strings.HasSuffix(filepath.ToSlash(path), "/uploads/20662/report.jpg") {
		t.Fatalf("path = %q, ok = %v", path, ok)
	}
}
