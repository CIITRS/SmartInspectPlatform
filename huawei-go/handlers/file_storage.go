package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// 文件存储配置结构
type FileStorageConfig struct {
	Endpoint        string `json:"endpoint"`
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	BucketName      string `json:"bucket_name"`
	UseSSL          bool   `json:"use_ssl"`
}

type QiniuStorageConfig struct {
	Enabled   bool
	AccessKey string
	SecretKey string
	Bucket    string
	Domain    string
	UploadURL string
	TokenTTL  time.Duration
}

type QiniuObjectItem struct {
	Key        string `json:"key"`
	Hash       string `json:"hash"`
	Size       int64  `json:"fsize"`
	MimeType   string `json:"mimeType"`
	PutTime    int64  `json:"putTime"`
	Type       int    `json:"type"`
	Status     int    `json:"status"`
	UploadedAt string `json:"uploaded_at,omitempty"`
	URL        string `json:"url,omitempty"`
}

type qiniuListResponse struct {
	Marker         string            `json:"marker"`
	CommonPrefixes []string          `json:"commonPrefixes"`
	Items          []QiniuObjectItem `json:"items"`
}

// 全局文件存储配置
var fileStorageConfig FileStorageConfig

// 全局MinIO客户端（用于Cloudflare R2）
var minioClient *minio.Client

var patientReportSequencePattern = regexp.MustCompile(`(?i)_report(\d+)\.[^.]+$`)
var qiniuRegionHostPattern = regexp.MustCompile(`(?i)\bplease\s+use\s+([a-z0-9.-]+\.qiniup\.com)([,\s"]|$)`)

func storageSettingValue(db *sql.DB, key string) string {
	if db == nil {
		return ""
	}
	var value string
	var encrypted int
	if err := db.QueryRow("SELECT key_value, is_encrypted FROM setting_system WHERE key_name = ? LIMIT 1", key).Scan(&value, &encrypted); err != nil {
		return ""
	}
	if encrypted == 1 {
		value = decryptConfigValue(value)
	}
	return strings.TrimSpace(value)
}

func loadQiniuStorageConfig() QiniuStorageConfig {
	value := func(key, fallback string) string {
		if setting := storageSettingValue(DB, key); setting != "" {
			return setting
		}
		if envValue := strings.TrimSpace(os.Getenv(key)); envValue != "" {
			return envValue
		}
		return fallback
	}
	enabledText := strings.ToLower(value("QINIU_ENABLED", "1"))
	ttlSeconds, _ := strconv.Atoi(value("QINIU_TOKEN_TTL_SECONDS", "3600"))
	if ttlSeconds < 60 {
		ttlSeconds = 3600
	}
	return QiniuStorageConfig{
		Enabled:   enabledText != "0" && enabledText != "false" && enabledText != "off" && enabledText != "disabled",
		AccessKey: value("QINIU_ACCESS_KEY", ""),
		SecretKey: value("QINIU_SECRET_KEY", ""),
		Bucket:    value("QINIU_BUCKET", "bucket01-bgpt-huaweibio-com-cn"),
		Domain:    strings.TrimRight(value("QINIU_DOMAIN", "https://bucket01.huaweibio.com.cn"), "/"),
		UploadURL: strings.TrimRight(value("QINIU_UPLOAD_URL", "https://upload.qiniup.com"), "/"),
		TokenTTL:  time.Duration(ttlSeconds) * time.Second,
	}
}

func (config QiniuStorageConfig) configured() bool {
	return config.Enabled && config.AccessKey != "" && config.SecretKey != "" && config.Bucket != "" && config.Domain != ""
}

func qiniuURLSafeBase64(data []byte) string {
	return base64.URLEncoding.EncodeToString(data)
}

func generateQiniuManagementToken(config QiniuStorageConfig, method, host, requestPath, rawQuery, contentType, qiniuDate string) string {
	signingText := method + " " + requestPath
	if rawQuery != "" {
		signingText += "?" + rawQuery
	}
	signingText += "\nHost: " + host
	if contentType != "" {
		signingText += "\nContent-Type: " + contentType
	}
	if qiniuDate != "" {
		signingText += "\nX-Qiniu-Date: " + qiniuDate
	}
	signingText += "\n\n"
	mac := hmac.New(sha1.New, []byte(config.SecretKey))
	_, _ = mac.Write([]byte(signingText))
	return config.AccessKey + ":" + qiniuURLSafeBase64(mac.Sum(nil))
}

func generateQiniuUploadToken(config QiniuStorageConfig, objectName string, now time.Time) (string, int64, error) {
	if !config.configured() {
		return "", 0, fmt.Errorf("七牛云存储尚未完整配置")
	}
	deadline := now.Add(config.TokenTTL).Unix()
	policy := map[string]interface{}{
		"scope":      config.Bucket + ":" + objectName,
		"deadline":   deadline,
		"returnBody": `{"key":"$(key)","hash":"$(etag)","size":$(fsize),"mimeType":"$(mimeType)"}`,
	}
	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return "", 0, err
	}
	encodedPolicy := qiniuURLSafeBase64(policyJSON)
	mac := hmac.New(sha1.New, []byte(config.SecretKey))
	_, _ = mac.Write([]byte(encodedPolicy))
	encodedSign := qiniuURLSafeBase64(mac.Sum(nil))
	return config.AccessKey + ":" + encodedSign + ":" + encodedPolicy, deadline, nil
}

func qiniuObjectURL(domain, objectName string) string {
	parts := strings.Split(strings.ReplaceAll(objectName, "\\", "/"), "/")
	for index, part := range parts {
		parts[index] = url.PathEscape(part)
	}
	return strings.TrimRight(domain, "/") + "/" + strings.Join(parts, "/")
}

func qiniuRegionUploadURL(responseBody []byte) (string, bool) {
	matches := qiniuRegionHostPattern.FindSubmatch(responseBody)
	if len(matches) < 2 {
		return "", false
	}
	host := strings.ToLower(strings.TrimSpace(string(matches[1])))
	if host == "" || strings.Contains(host, "..") || !strings.HasSuffix(host, ".qiniup.com") {
		return "", false
	}
	return "https://" + host, true
}

func rememberQiniuUploadURL(uploadURL string) {
	if DB == nil || strings.TrimSpace(uploadURL) == "" {
		return
	}
	if _, err := DB.Exec(`INSERT INTO setting_system
		(key_name, key_value, key_type, is_encrypted, description, created_at, updated_at)
		VALUES ('QINIU_UPLOAD_URL', ?, 'text', 0, '七牛云表单上传地址', NOW(), NOW())
		ON DUPLICATE KEY UPDATE key_value = VALUES(key_value), is_encrypted = 0, updated_at = NOW()`,
		uploadURL); err != nil {
		log.Printf("保存七牛云区域上传地址失败: %v", err)
	}
}

func uploadFileToQiniuOnce(file io.Reader, objectName, fileName, contentType string, config QiniuStorageConfig) ([]byte, int, error) {
	token, _, err := generateQiniuUploadToken(config, objectName, time.Now())
	if err != nil {
		return nil, 0, err
	}
	reader, writer := io.Pipe()
	form := multipart.NewWriter(writer)
	writeErr := make(chan error, 1)
	go func() {
		defer close(writeErr)
		if err := form.WriteField("token", token); err != nil {
			_ = writer.CloseWithError(err)
			writeErr <- err
			return
		}
		if err := form.WriteField("key", objectName); err != nil {
			_ = writer.CloseWithError(err)
			writeErr <- err
			return
		}
		part, err := form.CreateFormFile("file", fileName)
		if err == nil {
			_, err = io.Copy(part, file)
		}
		if closeErr := form.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			_ = writer.CloseWithError(err)
			writeErr <- err
			return
		}
		_ = writer.Close()
		writeErr <- nil
	}()

	request, err := http.NewRequest(http.MethodPost, config.UploadURL, reader)
	if err != nil {
		return nil, 0, err
	}
	request.Header.Set("Content-Type", form.FormDataContentType())
	if contentType != "" {
		request.Header.Set("X-Upload-Content-Type", contentType)
	}
	response, err := (&http.Client{Timeout: 5 * time.Minute}).Do(request)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err := <-writeErr; err != nil {
		return body, response.StatusCode, err
	}
	return body, response.StatusCode, nil
}

func uploadFileToQiniu(file io.Reader, objectName, fileName, contentType string, config QiniuStorageConfig) (string, error) {
	var replayable io.Seeker
	var initialOffset int64
	if seeker, ok := file.(io.Seeker); ok {
		if offset, err := seeker.Seek(0, io.SeekCurrent); err == nil {
			replayable = seeker
			initialOffset = offset
		}
	}

	body, statusCode, err := uploadFileToQiniuOnce(file, objectName, fileName, contentType, config)
	if err != nil {
		return "", err
	}
	if statusCode >= 200 && statusCode < 300 {
		return qiniuObjectURL(config.Domain, objectName), nil
	}

	regionURL, regionMismatch := qiniuRegionUploadURL(body)
	if regionMismatch && regionURL != strings.TrimRight(config.UploadURL, "/") {
		if replayable == nil {
			rememberQiniuUploadURL(regionURL)
			return "", fmt.Errorf("七牛云上传区域不匹配，已自动修正上传地址为 %s，请重试", regionURL)
		}
		if _, err := replayable.Seek(initialOffset, io.SeekStart); err != nil {
			return "", fmt.Errorf("七牛云上传区域不匹配且文件无法重试: %w", err)
		}
		retryConfig := config
		retryConfig.UploadURL = regionURL
		retryBody, retryStatus, retryErr := uploadFileToQiniuOnce(file, objectName, fileName, contentType, retryConfig)
		if retryErr != nil {
			return "", retryErr
		}
		if retryStatus >= 200 && retryStatus < 300 {
			rememberQiniuUploadURL(regionURL)
			return qiniuObjectURL(config.Domain, objectName), nil
		}
		return "", fmt.Errorf("七牛云上传失败（HTTP %d）：%s", retryStatus, strings.TrimSpace(string(retryBody)))
	}
	return "", fmt.Errorf("七牛云上传失败（HTTP %d）：%s", statusCode, strings.TrimSpace(string(body)))
}

func qiniuObjectNameFromURL(config QiniuStorageConfig, fileURL string) (string, bool) {
	fileURL = strings.TrimSpace(fileURL)
	domainPrefix := strings.TrimRight(config.Domain, "/") + "/"
	if !strings.HasPrefix(fileURL, domainPrefix) {
		return "", false
	}
	objectName, err := url.PathUnescape(strings.TrimPrefix(strings.Split(fileURL, "?")[0], domainPrefix))
	if err != nil {
		return "", false
	}
	objectName = strings.TrimLeft(strings.ReplaceAll(objectName, "\\", "/"), "/")
	if objectName == "" || strings.Contains(objectName, "../") || strings.HasPrefix(objectName, "..") {
		return "", false
	}
	return objectName, true
}

func legacyPatientReportLocalPath(fileURL string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(fileURL))
	if err != nil {
		return "", false
	}
	candidate := strings.TrimLeft(strings.ReplaceAll(parsed.Path, "\\", "/"), "/")
	if !strings.HasPrefix(candidate, "uploads/") {
		return "", false
	}
	rootPath, err := filepath.Abs("uploads")
	if err != nil {
		return "", false
	}
	targetPath, err := filepath.Abs(filepath.FromSlash(candidate))
	if err != nil || !strings.HasPrefix(targetPath, rootPath+string(os.PathSeparator)) {
		return "", false
	}
	return targetPath, true
}

func deleteFileFromQiniu(objectName string, config QiniuStorageConfig) error {
	if !config.configured() {
		return fmt.Errorf("七牛云存储尚未完整配置")
	}
	entryURI := qiniuURLSafeBase64([]byte(config.Bucket + ":" + objectName))
	requestPath := "/delete/" + entryURI
	host := "rs.qiniuapi.com"
	qiniuDate := time.Now().UTC().Format("20060102T150405Z")
	contentType := "application/x-www-form-urlencoded"
	accessToken := generateQiniuManagementToken(config, http.MethodPost, host, requestPath, "", contentType, qiniuDate)

	request, err := http.NewRequest(http.MethodPost, "https://"+host+requestPath, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Qiniu "+accessToken)
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("X-Qiniu-Date", qiniuDate)
	response, err := (&http.Client{Timeout: 30 * time.Second}).Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == 200 || response.StatusCode == 612 {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	return fmt.Errorf("七牛云删除失败（HTTP %d）：%s", response.StatusCode, strings.TrimSpace(string(body)))
}

func listQiniuObjectsPage(config QiniuStorageConfig, prefix, delimiter, marker string, limit int) (qiniuListResponse, error) {
	result := qiniuListResponse{Items: make([]QiniuObjectItem, 0), CommonPrefixes: make([]string, 0)}
	if !config.configured() {
		return result, fmt.Errorf("七牛云存储尚未完整配置")
	}
	if limit < 1 || limit > 1000 {
		limit = 1000
	}
	query := url.Values{}
	query.Set("bucket", config.Bucket)
	query.Set("limit", strconv.Itoa(limit))
	if prefix != "" {
		query.Set("prefix", prefix)
	}
	if delimiter != "" {
		query.Set("delimiter", delimiter)
	}
	if marker != "" {
		query.Set("marker", marker)
	}
	rawQuery := query.Encode()
	host := "rsf.qiniuapi.com"
	requestPath := "/list"
	qiniuDate := time.Now().UTC().Format("20060102T150405Z")
	contentType := "application/x-www-form-urlencoded"
	accessToken := generateQiniuManagementToken(config, http.MethodGet, host, requestPath, rawQuery, contentType, qiniuDate)

	request, err := http.NewRequest(http.MethodGet, "https://"+host+requestPath+"?"+rawQuery, nil)
	if err != nil {
		return result, err
	}
	request.Header.Set("Authorization", "Qiniu "+accessToken)
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("X-Qiniu-Date", qiniuDate)
	response, err := (&http.Client{Timeout: 30 * time.Second}).Do(request)
	if err != nil {
		return result, err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return result, fmt.Errorf("七牛云文件列表读取失败（HTTP %d）：%s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return result, err
	}
	for index := range result.Items {
		result.Items[index].URL = qiniuObjectURL(config.Domain, result.Items[index].Key)
		if result.Items[index].PutTime > 0 {
			result.Items[index].UploadedAt = time.Unix(result.Items[index].PutTime/1e7, 0).Format("2006-01-02 15:04:05")
		}
	}
	return result, nil
}

func getQiniuStorageUsage(config QiniuStorageConfig) (int64, int64, bool, error) {
	var totalFiles int64
	var totalBytes int64
	marker := ""
	for page := 0; page < 1000; page++ {
		result, err := listQiniuObjectsPage(config, "", "", marker, 1000)
		if err != nil {
			return totalFiles, totalBytes, false, err
		}
		for _, item := range result.Items {
			totalFiles++
			totalBytes += item.Size
		}
		if result.Marker == "" {
			return totalFiles, totalBytes, false, nil
		}
		marker = result.Marker
	}
	return totalFiles, totalBytes, true, nil
}

func HandleGetQiniuStorageOverview(c *app.RequestContext) {
	config := loadQiniuStorageConfig()
	if !config.configured() {
		c.JSON(consts.StatusServiceUnavailable, ApiResponse{Code: 503, Success: false, Message: "请先完整保存七牛云配置", Data: nil})
		return
	}
	prefix := strings.TrimLeft(strings.ReplaceAll(strings.TrimSpace(c.Query("prefix")), "\\", "/"), "/")
	marker := strings.TrimSpace(c.Query("marker"))
	limit, _ := strconv.Atoi(strings.TrimSpace(c.Query("limit")))
	if limit < 1 || limit > 500 {
		limit = 200
	}
	list, err := listQiniuObjectsPage(config, prefix, "/", marker, limit)
	if err != nil {
		c.JSON(consts.StatusBadGateway, ApiResponse{Code: 502, Success: false, Message: err.Error(), Data: nil})
		return
	}
	totalFiles, totalBytes, truncated, err := getQiniuStorageUsage(config)
	if err != nil {
		c.JSON(consts.StatusBadGateway, ApiResponse{Code: 502, Success: false, Message: err.Error(), Data: nil})
		return
	}
	c.JSON(consts.StatusOK, ApiResponse{
		Code: 200, Success: true, Message: "七牛云连接正常",
		Data: utils.H{
			"bucket": config.Bucket, "domain": config.Domain, "prefix": prefix,
			"total_files": totalFiles, "total_bytes": totalBytes, "usage_truncated": truncated,
			"common_prefixes": list.CommonPrefixes, "items": list.Items, "marker": list.Marker,
		},
	})
}

func buildCloudObjectName(prefix, originalName string) string {
	ext := strings.ToLower(filepath.Ext(originalName))
	base := strings.TrimSuffix(filepath.Base(originalName), filepath.Ext(originalName))
	base = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == 0 {
			return '-'
		}
		return r
	}, base)
	if strings.TrimSpace(base) == "" {
		base = "file"
	}
	return fmt.Sprintf("%s/%s/%s_%d%s", strings.Trim(prefix, "/"), time.Now().Format("2006/01/02"), base, time.Now().UnixNano(), ext)
}

func isValidPatientCode(patientCode string) bool {
	patientCode = strings.TrimSpace(patientCode)
	if !strings.HasPrefix(strings.ToUpper(patientCode), "HW") || len(patientCode) <= 2 {
		return false
	}
	for _, char := range patientCode {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '-' || char == '_' {
			continue
		}
		return false
	}
	return true
}

func nextPatientReportSequence(db *sql.DB, patientID int) int {
	if db == nil || patientID <= 0 {
		return 1
	}
	var reportFiles sql.NullString
	if err := db.QueryRow(`SELECT report_files FROM detect_patient WHERE id = ?`, patientID).Scan(&reportFiles); err != nil {
		return 1
	}
	return nextPatientReportSequenceFromFiles(reportFiles.String)
}

func nextPatientReportSequenceFromFiles(reportFiles string) int {
	count := 0
	maxSequence := 0
	for _, file := range strings.Split(reportFiles, ",") {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}
		count++
		matches := patientReportSequencePattern.FindStringSubmatch(strings.Split(file, "?")[0])
		if len(matches) == 2 {
			if sequence, err := strconv.Atoi(matches[1]); err == nil && sequence > maxSequence {
				maxSequence = sequence
			}
		}
	}
	if maxSequence > 0 {
		return maxSequence + 1
	}
	return count + 1
}

func buildPatientReportObjectName(patientCode, originalName string, uploadedAt time.Time, sequence int) string {
	patientCode = strings.ToUpper(strings.TrimSpace(patientCode))
	ext := strings.ToLower(filepath.Ext(originalName))
	if ext == "" {
		ext = ".bin"
	}
	if sequence < 1 {
		sequence = 1
	}
	fileName := fmt.Sprintf("%s_%s_report%02d%s", patientCode, uploadedAt.Format("20060102150405"), sequence, ext)
	return fmt.Sprintf("uploads/patient_report/%s/%s", patientCode, fileName)
}

func HandleGetQiniuUploadToken(c *app.RequestContext, db *sql.DB) {
	config := loadQiniuStorageConfig()
	if !config.configured() {
		c.JSON(consts.StatusServiceUnavailable, ApiResponse{Code: 503, Success: false, Message: "七牛云存储尚未完整配置", Data: nil})
		return
	}
	fileName := strings.TrimSpace(c.PostForm("file_name"))
	if fileName == "" {
		fileName = strings.TrimSpace(c.Query("file_name"))
	}
	if fileName == "" {
		fileName = "upload.bin"
	}
	fileType := strings.TrimSpace(c.PostForm("type"))
	if fileType == "" {
		fileType = strings.TrimSpace(c.Query("type"))
	}
	switch fileType {
	case "image", "video", "attachment", "report", "patient-report":
	default:
		fileType = "attachment"
	}
	objectName := buildCloudObjectName("uploads/"+fileType, fileName)
	if fileType == "patient-report" {
		patientIdentifier := strings.TrimSpace(c.PostForm("patient_code"))
		if patientIdentifier == "" {
			patientIdentifier = strings.TrimSpace(c.Query("patient_code"))
		}
		patientID, patientCode, resolveErr := resolvePatientID(db, patientIdentifier, false)
		if resolveErr != nil || patientID <= 0 || !isValidPatientCode(patientCode) {
			c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "患者编号无效", Data: nil})
			return
		}
		objectName = buildPatientReportObjectName(patientCode, fileName, time.Now(), nextPatientReportSequence(db, patientID))
	}
	token, deadline, err := generateQiniuUploadToken(config, objectName, time.Now())
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "生成上传凭证失败", Data: utils.H{"error": err.Error()}})
		return
	}
	c.JSON(consts.StatusOK, ApiResponse{
		Code: 200, Success: true, Message: "获取上传凭证成功",
		Data: utils.H{
			"token": token, "key": objectName, "upload_url": config.UploadURL,
			"file_url": qiniuObjectURL(config.Domain, objectName), "expires_at": deadline,
		},
	})
}

// 初始化文件存储配置
func InitFileStorage() error {
	// 从配置文件加载存储配置
	configData, err := os.ReadFile("file_storage_config.json")
	if err == nil {
		err = json.Unmarshal(configData, &fileStorageConfig)
		if err == nil {
			// 初始化MinIO客户端
			return initMinioClient()
		}
	}
	return nil
}

// 初始化MinIO客户端
func initMinioClient() error {
	var err error
	minioClient, err = minio.New(fileStorageConfig.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(fileStorageConfig.AccessKeyID, fileStorageConfig.SecretAccessKey, ""),
		Secure: fileStorageConfig.UseSSL,
	})
	if err != nil {
		log.Printf("初始化MinIO客户端失败: %v", err)
		return err
	}

	// 检查存储桶是否存在
	exists, err := minioClient.BucketExists(context.Background(), fileStorageConfig.BucketName)
	if err != nil {
		log.Printf("检查存储桶失败: %v", err)
		return err
	}

	if !exists {
		// 创建存储桶
		err = minioClient.MakeBucket(context.Background(), fileStorageConfig.BucketName, minio.MakeBucketOptions{})
		if err != nil {
			log.Printf("创建存储桶失败: %v", err)
			return err
		}
		log.Printf("存储桶 %s 创建成功", fileStorageConfig.BucketName)
	}

	log.Println("文件存储初始化成功")
	return nil
}

// 处理获取文件存储配置
func HandleGetFileStorageConfig(c *app.RequestContext) {
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取文件存储配置成功",
		Data:    fileStorageConfig,
	})
}

// 处理更新文件存储配置
func HandleUpdateFileStorageConfig(c *app.RequestContext) {
	var req FileStorageConfig
	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请求参数错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 保存配置到文件
	configData, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "保存配置失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	err = os.WriteFile("file_storage_config.json", configData, 0644)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "保存配置失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 更新全局配置
	fileStorageConfig = req

	// 重新初始化MinIO客户端
	err = initMinioClient()
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "初始化存储客户端失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "更新文件存储配置成功",
		Data:    nil,
	})
}

// 上传文件到Cloudflare R2
func uploadFileToR2(file multipart.File, objectName string, contentType string) (string, error) {
	qiniuConfig := loadQiniuStorageConfig()
	if qiniuConfig.configured() {
		return uploadFileToQiniu(file, objectName, filepath.Base(objectName), contentType, qiniuConfig)
	}
	if minioClient == nil {
		return saveFileToLocalUploads(file, objectName)
	}

	_, err := minioClient.PutObject(context.Background(),
		fileStorageConfig.BucketName,
		objectName,
		file,
		-1, // 自动检测文件大小
		minio.PutObjectOptions{ContentType: contentType},
	)
	if err != nil {
		return "", err
	}

	// 生成访问URL
	presignedURL, err := minioClient.PresignedGetObject(context.Background(),
		fileStorageConfig.BucketName,
		objectName,
		7*24*time.Hour, // 7天有效期
		nil,
	)
	if err != nil {
		return "", err
	}

	return presignedURL.String(), nil
}

func saveFileToLocalUploads(file multipart.File, objectName string) (string, error) {
	cleanObjectName := filepath.Clean(strings.ReplaceAll(objectName, "\\", "/"))
	cleanObjectName = strings.TrimPrefix(cleanObjectName, string(filepath.Separator))
	if strings.HasPrefix(cleanObjectName, "..") || !strings.HasPrefix(strings.ReplaceAll(cleanObjectName, "\\", "/"), "uploads/") {
		return "", fmt.Errorf("非法文件路径")
	}

	localPath := filepath.Join(".", cleanObjectName)
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return "", err
	}

	dst, err := os.Create(localPath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		return "", err
	}

	return "/" + strings.ReplaceAll(cleanObjectName, "\\", "/"), nil
}

// 处理文件上传到Cloudflare R2
func HandleUploadFileToR2(c *app.RequestContext, db *sql.DB) {
	// 获取上传的文件
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请选择要上传的文件",
			Data:    nil,
		})
		return
	}

	// 打开文件
	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "文件打开失败",
			Data:    nil,
		})
		return
	}
	defer file.Close()

	// 确定文件类型并存储到相应的表
	fileType := c.Query("type")
	if fileType == "" {
		fileType = c.PostForm("type")
	}
	patientIdentifier := c.PostForm("patient_code")
	if patientIdentifier == "" {
		patientIdentifier = c.Query("patient_code")
	}
	if patientIdentifier == "" {
		patientIdentifier = c.PostForm("patient_id")
	}
	if patientIdentifier == "" {
		patientIdentifier = c.Query("patient_id")
	}

	patientID := 0
	patientCode := ""
	if patientIdentifier != "" {
		resolvedID, resolvedCode, resolveErr := resolvePatientID(db, patientIdentifier, false)
		if resolveErr != nil {
			c.JSON(consts.StatusBadRequest, ApiResponse{
				Code:    400,
				Success: false,
				Message: "患者不存在",
				Data:    nil,
			})
			return
		}
		patientID = resolvedID
		patientCode = resolvedCode
	}

	objectName := buildCloudObjectName("uploads", fileHeader.Filename)
	if patientID > 0 && isValidPatientCode(patientCode) {
		objectName = buildPatientReportObjectName(patientCode, fileHeader.Filename, time.Now(), nextPatientReportSequence(db, patientID))
	}
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = mime.TypeByExtension(strings.ToLower(filepath.Ext(fileHeader.Filename)))
	}
	fileURL, err := uploadFileToR2(file, objectName, contentType)
	if err != nil {
		log.Printf("患者报告上传失败 patient=%s object=%s: %v", patientCode, objectName, err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "文件上传失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	fileID, err := generateUUID()
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "生成文件ID失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}
	accessToken, err := generateAccessToken()
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "生成文件访问令牌失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}
	urlPath, err := generateRandomPath()
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "生成文件访问路径失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	if fileType == "report" {
		// 存储到base_files_report表
		_, err := db.Exec(`INSERT INTO base_files_report 
			(id, original_name, storage_path, file_name, file_path, file_url, access_token, url_path, expires_at, is_public, created_at, updated_at) 
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, DATE_ADD(NOW(), INTERVAL 7 DAY), 0, NOW(), NOW())`,
			fileID, fileHeader.Filename, objectName, fileHeader.Filename, objectName, fileURL, accessToken, urlPath)
		if err != nil {
			log.Printf("存储报告文件信息失败: %v", err)
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code:    500,
				Success: false,
				Message: "存储文件信息失败",
				Data:    utils.H{"error": err.Error()},
			})
			return
		}
	} else {
		// 存储到base_files_patient表
		_, err := db.Exec(`INSERT INTO base_files_patient
			(id, patient_id, patient_code, original_name, storage_path, file_name, file_path, file_url, access_token, url_path, expires_at, is_public, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, DATE_ADD(NOW(), INTERVAL 7 DAY), 0, NOW(), NOW())`,
			fileID, patientID, patientCode, fileHeader.Filename, objectName, fileHeader.Filename, objectName, fileURL, accessToken, urlPath)
		if err != nil {
			log.Printf("存储患者文件信息失败: %v", err)
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code:    500,
				Success: false,
				Message: "存储文件信息失败",
				Data:    utils.H{"error": err.Error()},
			})
			return
		}

		if patientID > 0 {
			var existingFiles sql.NullString
			_ = db.QueryRow(`SELECT report_files FROM detect_patient WHERE id = ?`, patientID).Scan(&existingFiles)
			updatedFiles := fileURL
			if existingFiles.Valid && strings.TrimSpace(existingFiles.String) != "" {
				updatedFiles = existingFiles.String + "," + fileURL
			}
			if _, err := db.Exec(`UPDATE detect_patient SET report_files = ?, updated_at = NOW() WHERE id = ?`, updatedFiles, patientID); err != nil {
				log.Printf("更新患者报告文件失败: %v", err)
			}
		}
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "文件上传成功",
		Data: utils.H{
			"id":   fileID,
			"name": fileHeader.Filename,
			"url":  fileURL,
			"path": objectName,
		},
	})
}

// 处理生成文件后上传到Cloudflare R2
func UploadGeneratedFileToR2(filePath string, fileName string, fileType string, db *sql.DB) (string, error) {
	// 打开文件
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// 生成唯一对象名
	objectName := buildCloudObjectName("generated", fileName)
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(fileName)))

	// 上传到Cloudflare R2
	fileURL, err := uploadFileToR2(file, objectName, contentType)
	if err != nil {
		return "", err
	}

	// 存储到数据库
	fileID, err := generateUUID()
	if err != nil {
		return fileURL, err
	}
	accessToken, err := generateAccessToken()
	if err != nil {
		return fileURL, err
	}
	urlPath, err := generateRandomPath()
	if err != nil {
		return fileURL, err
	}
	if fileType == "report" {
		_, err = db.Exec(`INSERT INTO base_files_report
			(id, original_name, storage_path, file_name, file_path, file_url, access_token, url_path, expires_at, is_public, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, DATE_ADD(NOW(), INTERVAL 7 DAY), 0, NOW(), NOW())`,
			fileID, fileName, objectName, fileName, objectName, fileURL, accessToken, urlPath)
	} else {
		_, err = db.Exec(`INSERT INTO base_files_patient
			(id, original_name, storage_path, file_name, file_path, file_url, access_token, url_path, expires_at, is_public, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, DATE_ADD(NOW(), INTERVAL 7 DAY), 0, NOW(), NOW())`,
			fileID, fileName, objectName, fileName, objectName, fileURL, accessToken, urlPath)
	}

	if err != nil {
		log.Printf("存储文件信息失败: %v", err)
		return fileURL, err
	}

	// 删除本地文件
	os.Remove(filePath)

	return fileURL, nil
}
