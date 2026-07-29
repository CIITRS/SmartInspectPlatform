package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

func TestGenerateQiniuPrivateDownloadURL(t *testing.T) {
	config := QiniuStorageConfig{
		Enabled: true, AccessKey: "test-access", SecretKey: "test-secret",
		Bucket: "test-bucket", Domain: "https://files.example.com",
	}
	now := time.Unix(1_700_000_000, 0)
	got, deadline, err := generateQiniuPrivateDownloadURL(
		config,
		"https://files.example.com/uploads/patient_report/HWP1/report01.pdf",
		now,
		10*time.Minute,
	)
	if err != nil {
		t.Fatalf("generateQiniuPrivateDownloadURL() error = %v", err)
	}
	unsignedURL := fmt.Sprintf(
		"https://files.example.com/uploads/patient_report/HWP1/report01.pdf?e=%d",
		now.Add(10*time.Minute).Unix(),
	)
	mac := hmac.New(sha1.New, []byte(config.SecretKey))
	_, _ = mac.Write([]byte(unsignedURL))
	want := unsignedURL + "&token=" + config.AccessKey + ":" + base64.URLEncoding.EncodeToString(mac.Sum(nil))
	if got != want {
		t.Fatalf("download URL = %q, want %q", got, want)
	}
	if deadline != now.Add(10*time.Minute).Unix() {
		t.Fatalf("deadline = %d", deadline)
	}
	if _, _, err := generateQiniuPrivateDownloadURL(
		config, "https://attacker.example/report.pdf", now, time.Minute,
	); err == nil {
		t.Fatal("expected mismatched domain to be rejected")
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

func TestQiniuRegionUploadURL(t *testing.T) {
	got, ok := qiniuRegionUploadURL([]byte(
		`{"error":"incorrect region, please use up-z1.qiniup.com, bucket is: example"}`,
	))
	if !ok || got != "https://up-z1.qiniup.com" {
		t.Fatalf("qiniuRegionUploadURL() = %q, %v", got, ok)
	}
	for _, body := range []string{
		`{"error":"incorrect region"}`,
		`{"error":"please use attacker.example.com"}`,
		`{"error":"please use up-z1.qiniup.com.attacker.example"}`,
	} {
		if endpoint, ok := qiniuRegionUploadURL([]byte(body)); ok {
			t.Fatalf("unexpected endpoint %q from %q", endpoint, body)
		}
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
	if os.Getenv("QINIU_FORCE_REGION_DISCOVERY") == "1" {
		config.UploadURL = "https://upload.qiniup.com"
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

func TestQiniuUploadIntegration(t *testing.T) {
	if os.Getenv("QINIU_UPLOAD_INTEGRATION") != "1" {
		t.Skip("set QINIU_UPLOAD_INTEGRATION=1 to verify upload and cleanup")
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Fatalf("db.Ping() error = %v", err)
	}
	previousDB := DB
	SetDB(db)
	defer SetDB(previousDB)

	config := loadQiniuStorageConfig()
	if !config.configured() {
		t.Fatal("Qiniu integration configuration is incomplete")
	}
	objectName := "uploads/diagnostics/upload-smoke-" + time.Now().UTC().Format("20060102T150405.000000000Z") + ".txt"
	defer func() {
		if err := deleteFileFromQiniu(objectName, config); err != nil {
			t.Errorf("delete diagnostic object: %v", err)
		}
	}()
	fileURL, err := uploadFileToQiniu(
		bytes.NewReader([]byte("SmartInspectPlatform Qiniu upload smoke test\n")),
		objectName,
		"upload-smoke.txt",
		"text/plain",
		config,
	)
	if err != nil {
		t.Fatalf("upload diagnostic object: %v", err)
	}
	if fileURL != qiniuObjectURL(config.Domain, objectName) {
		t.Fatalf("file URL = %q", fileURL)
	}
}

func TestQiniuPrivateDownloadIntegration(t *testing.T) {
	if os.Getenv("QINIU_PRIVATE_DOWNLOAD_INTEGRATION") != "1" {
		t.Skip("set QINIU_PRIVATE_DOWNLOAD_INTEGRATION=1 to verify private object access")
	}
	rawURL := strings.TrimSpace(os.Getenv("QINIU_PRIVATE_DOWNLOAD_URL"))
	if rawURL == "" {
		t.Fatal("QINIU_PRIVATE_DOWNLOAD_URL is required")
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Fatalf("db.Ping() error = %v", err)
	}
	previousDB := DB
	SetDB(db)
	defer SetDB(previousDB)

	config := loadQiniuStorageConfig()
	signedURL, _, err := generateQiniuPrivateDownloadURL(config, rawURL, time.Now(), time.Minute)
	if err != nil {
		t.Fatalf("sign private download URL: %v", err)
	}
	response, err := (&http.Client{Timeout: 30 * time.Second}).Get(signedURL)
	if err != nil {
		t.Fatalf("download private object: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		t.Fatalf("download private object status = %d, body = %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
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
