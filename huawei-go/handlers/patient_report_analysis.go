package handlers

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type PatientReportAnalysis struct {
	Status       string `json:"status"`
	Content      string `json:"content"`
	Model        string `json:"model"`
	FileName     string `json:"file_name"`
	FileType     string `json:"file_type"`
	ErrorMessage string `json:"error_message,omitempty"`
	AnalyzedAt   string `json:"analyzed_at,omitempty"`
}

type reportAnalysisRow struct {
	ID        int
	Code      string
	FileURL   string
	FileName  string
	UpdatedAt time.Time
}

func patientReportFileKey(fileURL string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(fileURL)))
	return hex.EncodeToString(sum[:])
}

func patientReportFileName(fileURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(fileURL))
	if err == nil {
		if name, unescapeErr := url.PathUnescape(filepath.Base(parsed.Path)); unescapeErr == nil && name != "." && name != "/" {
			return name
		}
	}
	return filepath.Base(strings.Split(fileURL, "?")[0])
}

func validateStoredPatientReport(db *sql.DB, patientID int, fileURL string) (string, error) {
	var patientCode string
	var reportFiles sql.NullString
	if err := db.QueryRow(`SELECT COALESCE(patient_code, ''), report_files FROM detect_patient WHERE id = ? AND is_active = 1`,
		patientID).Scan(&patientCode, &reportFiles); err != nil {
		return "", err
	}
	for _, storedURL := range splitPatientReportFiles(reportFiles.String) {
		if storedURL == strings.TrimSpace(fileURL) {
			return patientCode, nil
		}
	}
	return "", sql.ErrNoRows
}

func openStoredPatientReport(patientCode, fileURL string) (io.ReadCloser, error) {
	config := loadQiniuStorageConfig()
	if objectName, ok := qiniuObjectNameFromURL(config, fileURL); ok {
		expectedPrefix := "uploads/patient_report/" + strings.ToUpper(patientCode) + "/"
		if !strings.HasPrefix(objectName, expectedPrefix) {
			return nil, fmt.Errorf("报告文件路径与患者不匹配")
		}
		signedURL, _, err := generateQiniuPrivateDownloadURL(config, fileURL, time.Now(), 10*time.Minute)
		if err != nil {
			return nil, err
		}
		request, err := http.NewRequest(http.MethodGet, signedURL, nil)
		if err != nil {
			return nil, err
		}
		response, err := (&http.Client{Timeout: 60 * time.Second}).Do(request)
		if err != nil {
			return nil, err
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			defer response.Body.Close()
			body, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
			return nil, fmt.Errorf("读取云端报告失败（HTTP %d）：%s", response.StatusCode, strings.TrimSpace(string(body)))
		}
		if response.ContentLength > 20*1024*1024 {
			response.Body.Close()
			return nil, fmt.Errorf("报告文件不能超过20MB")
		}
		return response.Body, nil
	}
	if localPath, ok := legacyPatientReportLocalPath(fileURL); ok {
		return os.Open(localPath)
	}
	return nil, fmt.Errorf("不支持分析该报告地址")
}

func getPatientReportAnalysis(db *sql.DB, patientID int, fileURL string) (PatientReportAnalysis, error) {
	var result PatientReportAnalysis
	var content, model, fileName, fileType, errorMessage sql.NullString
	var analyzedAt sql.NullTime
	err := db.QueryRow(`SELECT status, analysis_text, model, file_name, file_type, error_message, analyzed_at
		FROM patient_report_analysis WHERE patient_id = ? AND file_key = ? LIMIT 1`,
		patientID, patientReportFileKey(fileURL)).Scan(
		&result.Status, &content, &model, &fileName, &fileType, &errorMessage, &analyzedAt,
	)
	if err != nil {
		return result, err
	}
	result.Content = content.String
	result.Model = model.String
	result.FileName = fileName.String
	result.FileType = fileType.String
	result.ErrorMessage = errorMessage.String
	if analyzedAt.Valid {
		result.AnalyzedAt = analyzedAt.Time.Format("2006-01-02 15:04:05")
	}
	return result, nil
}

// AnalyzeStoredPatientReport 下载患者名下的私有报告，调用对应模型并持久化结果。
func AnalyzeStoredPatientReport(db *sql.DB, patientID int, fileURL string, force bool) (PatientReportAnalysis, error) {
	fileURL = strings.TrimSpace(fileURL)
	patientCode, err := validateStoredPatientReport(db, patientID, fileURL)
	if err != nil {
		return PatientReportAnalysis{}, fmt.Errorf("报告文件不存在")
	}
	if existing, err := getPatientReportAnalysis(db, patientID, fileURL); err == nil && existing.Status == "completed" && !force {
		return existing, nil
	}
	fileName := patientReportFileName(fileURL)
	fileType := strings.TrimPrefix(strings.ToLower(filepath.Ext(fileName)), ".")
	fileKey := patientReportFileKey(fileURL)
	_, err = db.Exec(`INSERT INTO patient_report_analysis
		(patient_id, file_key, file_url, file_name, file_type, status, error_message, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 'processing', '', NOW(), NOW())
		ON DUPLICATE KEY UPDATE file_url = VALUES(file_url), file_name = VALUES(file_name),
			file_type = VALUES(file_type), status = 'processing', error_message = '', updated_at = NOW()`,
		patientID, fileKey, fileURL, fileName, fileType)
	if err != nil {
		return PatientReportAnalysis{}, err
	}
	reader, err := openStoredPatientReport(patientCode, fileURL)
	if err != nil {
		_, _ = db.Exec(`UPDATE patient_report_analysis SET status = 'failed', error_message = ?, updated_at = NOW()
			WHERE patient_id = ? AND file_key = ?`, truncateReportAnalysisError(err), patientID, fileKey)
		return PatientReportAnalysis{}, err
	}
	defer reader.Close()
	content, model, err := AnalyzePatientReportReader(fileName, reader)
	if err != nil {
		_, _ = db.Exec(`UPDATE patient_report_analysis SET status = 'failed', model = ?, error_message = ?, updated_at = NOW()
			WHERE patient_id = ? AND file_key = ?`, model, truncateReportAnalysisError(err), patientID, fileKey)
		return PatientReportAnalysis{}, err
	}
	_, err = db.Exec(`UPDATE patient_report_analysis SET status = 'completed', analysis_text = ?, model = ?,
		error_message = '', analyzed_at = NOW(), updated_at = NOW() WHERE patient_id = ? AND file_key = ?`,
		content, model, patientID, fileKey)
	if err != nil {
		return PatientReportAnalysis{}, err
	}
	return getPatientReportAnalysis(db, patientID, fileURL)
}

func truncateReportAnalysisError(err error) string {
	value := strings.TrimSpace(err.Error())
	runes := []rune(value)
	if len(runes) > 480 {
		value = string(runes[:480])
	}
	return value
}

func reportAnalysisPatientFromAdmin(c *app.RequestContext, db *sql.DB) (int, string, bool) {
	patientID, _, err := resolvePatientID(db, strings.TrimSpace(c.Param("id")), false)
	if err != nil {
		c.JSON(consts.StatusNotFound, ApiResponse{Code: 404, Success: false, Message: "患者不存在"})
		return 0, "", false
	}
	fileURL := strings.TrimSpace(c.Query("file_url"))
	if fileURL == "" {
		var request struct {
			FileURL string `json:"file_url"`
		}
		if err := c.BindAndValidate(&request); err == nil {
			fileURL = strings.TrimSpace(request.FileURL)
		}
	}
	if fileURL == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "报告文件地址不能为空"})
		return 0, "", false
	}
	if _, err := validateStoredPatientReport(db, patientID, fileURL); err != nil {
		c.JSON(consts.StatusNotFound, ApiResponse{Code: 404, Success: false, Message: "报告文件不存在"})
		return 0, "", false
	}
	return patientID, fileURL, true
}

func HandleGetPatientReportAnalysis(c *app.RequestContext, db *sql.DB) {
	patientID, fileURL, ok := reportAnalysisPatientFromAdmin(c, db)
	if !ok {
		return
	}
	result, err := getPatientReportAnalysis(db, patientID, fileURL)
	if err == sql.ErrNoRows {
		SuccessResponse(c, "报告尚未分析", PatientReportAnalysis{Status: "not_started", FileName: patientReportFileName(fileURL)})
		return
	}
	if err != nil {
		ErrorResponse(c, consts.StatusInternalServerError, "读取AI分析失败", nil)
		return
	}
	SuccessResponse(c, "获取成功", result)
}

func HandleAnalyzePatientReport(c *app.RequestContext, db *sql.DB) {
	patientID, fileURL, ok := reportAnalysisPatientFromAdmin(c, db)
	if !ok {
		return
	}
	force := c.Query("force") == "1"
	result, err := AnalyzeStoredPatientReport(db, patientID, fileURL, force)
	if err != nil {
		log.Printf("analyze stored patient report patient=%d: %v", patientID, err)
		ErrorResponse(c, consts.StatusBadGateway, "报告分析失败："+truncateReportAnalysisError(err), nil)
		return
	}
	SuccessResponse(c, "分析成功", result)
}

func miniEmployeeReportContext(c *app.RequestContext, db *sql.DB) (int, string, string, bool) {
	employeeID, ok := requireMiniappEmployee(c, db)
	if !ok {
		return 0, "", "", false
	}
	patientID, err := strconv.Atoi(c.Param("id"))
	if err != nil || patientID <= 0 {
		ErrorResponse(c, consts.StatusBadRequest, "无效的患者ID", nil)
		return 0, "", "", false
	}
	fileURL := strings.TrimSpace(c.Query("file_url"))
	if fileURL == "" {
		var request struct {
			FileURL string `json:"file_url"`
		}
		if err := c.BindAndValidate(&request); err == nil {
			fileURL = strings.TrimSpace(request.FileURL)
		}
	}
	var patientCode string
	query := `SELECT COALESCE(patient_code, '') FROM detect_patient WHERE id = ? AND is_active = 1`
	args := []interface{}{patientID}
	if filter, filterArgs := miniappEmployeePatientAccessFilter(db, employeeID, ""); filter != "" {
		query += " AND " + filter
		args = append(args, filterArgs...)
	}
	if err := db.QueryRow(query, args...).Scan(&patientCode); err != nil {
		ErrorResponse(c, consts.StatusNotFound, "未找到患者", nil)
		return 0, "", "", false
	}
	if _, err := validateStoredPatientReport(db, patientID, fileURL); err != nil {
		ErrorResponse(c, consts.StatusNotFound, "报告文件不存在", nil)
		return 0, "", "", false
	}
	return patientID, patientCode, fileURL, true
}

func HandleUniEmployeePatientReportPreview(c *app.RequestContext, db *sql.DB) {
	_, patientCode, fileURL, ok := miniEmployeeReportContext(c, db)
	if !ok {
		return
	}
	previewURL, expiresAt, err := storedPatientReportPreviewURL(patientCode, fileURL)
	if err != nil {
		ErrorResponse(c, consts.StatusBadRequest, err.Error(), nil)
		return
	}
	SuccessResponse(c, "获取成功", utils.H{"preview_url": previewURL, "expires_at": expiresAt})
}

func HandleUniEmployeeGetPatientReportAnalysis(c *app.RequestContext, db *sql.DB) {
	patientID, _, fileURL, ok := miniEmployeeReportContext(c, db)
	if !ok {
		return
	}
	result, err := getPatientReportAnalysis(db, patientID, fileURL)
	if err == sql.ErrNoRows {
		SuccessResponse(c, "报告尚未分析", PatientReportAnalysis{Status: "not_started", FileName: patientReportFileName(fileURL)})
		return
	}
	if err != nil {
		ErrorResponse(c, consts.StatusInternalServerError, "读取AI分析失败", nil)
		return
	}
	SuccessResponse(c, "获取成功", result)
}

func HandleUniEmployeeAnalyzePatientReport(c *app.RequestContext, db *sql.DB) {
	patientID, _, fileURL, ok := miniEmployeeReportContext(c, db)
	if !ok {
		return
	}
	result, err := AnalyzeStoredPatientReport(db, patientID, fileURL, c.Query("force") == "1")
	if err != nil {
		ErrorResponse(c, consts.StatusBadGateway, "报告分析失败："+truncateReportAnalysisError(err), nil)
		return
	}
	SuccessResponse(c, "分析成功", result)
}

// BackfillPatientReportAnalyses 分析尚未成功处理的存量患者报告。
func BackfillPatientReportAnalyses(db *sql.DB, limit int, force bool) (processed, succeeded, failed int, err error) {
	query := `SELECT id, COALESCE(patient_code, ''), COALESCE(report_files, ''), updated_at
		FROM detect_patient WHERE is_active = 1 AND COALESCE(TRIM(report_files), '') <> '' ORDER BY id`
	rows, err := db.Query(query)
	if err != nil {
		return 0, 0, 0, err
	}
	defer rows.Close()
	items := make([]reportAnalysisRow, 0)
	for rows.Next() {
		var patientID int
		var code, files string
		var updatedAt time.Time
		if scanErr := rows.Scan(&patientID, &code, &files, &updatedAt); scanErr != nil {
			continue
		}
		for _, fileURL := range splitPatientReportFiles(files) {
			if limit > 0 && len(items) >= limit {
				break
			}
			items = append(items, reportAnalysisRow{ID: patientID, Code: code, FileURL: fileURL, FileName: patientReportFileName(fileURL), UpdatedAt: updatedAt})
		}
		if limit > 0 && len(items) >= limit {
			break
		}
	}
	for _, item := range items {
		if !force {
			if existing, getErr := getPatientReportAnalysis(db, item.ID, item.FileURL); getErr == nil && existing.Status == "completed" {
				continue
			}
		}
		processed++
		if _, analyzeErr := AnalyzeStoredPatientReport(db, item.ID, item.FileURL, force); analyzeErr != nil {
			failed++
			log.Printf("backfill report analysis patient=%s file=%s: %v", item.Code, item.FileName, analyzeErr)
			continue
		}
		succeeded++
	}
	return processed, succeeded, failed, nil
}
