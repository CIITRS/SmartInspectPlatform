package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type PatientReportMigrationOptions struct {
	Apply       bool
	PatientCode string
	SourceBase  string
	LocalRoot   string
	Limit       int
}

type PatientReportMigrationItem struct {
	PatientID   int
	PatientCode string
	Source      string
	ObjectName  string
	TargetURL   string
	Status      string
	Error       string
}

type PatientReportMigrationResult struct {
	Patients int
	Planned  int
	Uploaded int
	Skipped  int
	Failed   int
	Items    []PatientReportMigrationItem
}

type patientReportMigrationRow struct {
	ID          int
	PatientCode string
	ReportFiles string
	UpdatedAt   time.Time
}

func splitPatientReportFiles(value string) []string {
	files := make([]string, 0)
	for _, item := range strings.Split(value, ",") {
		if cleaned := strings.TrimSpace(item); cleaned != "" {
			files = append(files, cleaned)
		}
	}
	return files
}

func patientReportSourceExtension(source string) string {
	if parsed, err := url.Parse(source); err == nil && parsed.Path != "" {
		if ext := strings.ToLower(filepath.Ext(parsed.Path)); ext != "" {
			return ext
		}
	}
	return strings.ToLower(filepath.Ext(strings.Split(source, "?")[0]))
}

func patientReportUploadTime(source string, fallback time.Time) time.Time {
	base := filepath.Base(strings.Split(source, "?")[0])
	name := strings.TrimSuffix(base, filepath.Ext(base))
	parts := strings.Split(name, "_")
	for index := len(parts) - 1; index >= 0; index-- {
		value := strings.TrimSpace(parts[index])
		if len(value) != 10 && len(value) != 13 {
			continue
		}
		unixValue, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			continue
		}
		if len(value) == 13 {
			unixValue /= 1000
		}
		parsed := time.Unix(unixValue, 0)
		if parsed.Year() >= 2000 && parsed.Year() <= 2100 {
			return parsed
		}
	}
	if fallback.IsZero() {
		return time.Now()
	}
	return fallback
}

func resolvePatientReportSource(source, sourceBase, localRoot string) (string, bool) {
	source = strings.TrimSpace(source)
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return source, true
	}
	normalized := strings.ReplaceAll(source, "\\", "/")
	if strings.HasPrefix(normalized, "/uploads/") || strings.HasPrefix(normalized, "uploads/") {
		return strings.TrimRight(sourceBase, "/") + "/" + strings.TrimLeft(normalized, "/"), true
	}
	if filepath.IsAbs(source) {
		return filepath.Clean(source), false
	}
	return filepath.Join(localRoot, filepath.FromSlash(normalized)), false
}

func openPatientReportSource(ctx context.Context, source, sourceBase, localRoot string) (io.ReadCloser, string, error) {
	resolved, remote := resolvePatientReportSource(source, sourceBase, localRoot)
	if !remote {
		file, err := os.Open(resolved)
		if err != nil {
			return nil, "", err
		}
		return file, mime.TypeByExtension(strings.ToLower(filepath.Ext(resolved))), nil
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, resolved, nil)
	if err != nil {
		return nil, "", err
	}
	response, err := (&http.Client{Timeout: 5 * time.Minute}).Do(request)
	if err != nil {
		return nil, "", err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_ = response.Body.Close()
		return nil, "", fmt.Errorf("下载源文件失败（HTTP %d）", response.StatusCode)
	}
	contentType := response.Header.Get("Content-Type")
	if contentType == "" {
		contentType = mime.TypeByExtension(patientReportSourceExtension(source))
	}
	return response.Body, contentType, nil
}

func replaceMigratedPatientReportReferences(tx *sql.Tx, patientID int, patientCode string, replacements map[string]string, objectNames map[string]string) error {
	for source, target := range replacements {
		objectName := objectNames[source]
		fileName := filepath.Base(objectName)
		if _, err := tx.Exec(`UPDATE base_files_patient
			SET patient_id = ?, patient_code = ?, storage_path = ?, file_name = ?, file_path = ?, file_url = ?, updated_at = NOW()
			WHERE (patient_id = ? OR patient_id IS NULL OR patient_id = 0)
			  AND (file_url = ? OR file_path = ? OR storage_path = ?)`,
			patientID, patientCode, objectName, fileName, objectName, target,
			patientID, source, source, source); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE patient_follow_up
			SET images_json = REPLACE(images_json, ?, ?), updated_at = NOW()
			WHERE patient_id = ? AND images_json LIKE ?`,
			source, target, patientID, "%"+source+"%"); err != nil {
			return err
		}
	}
	return nil
}

func MigrateExistingPatientReportFiles(ctx context.Context, db *sql.DB, options PatientReportMigrationOptions) (PatientReportMigrationResult, error) {
	result := PatientReportMigrationResult{Items: make([]PatientReportMigrationItem, 0)}
	config := loadQiniuStorageConfig()
	if !config.configured() {
		return result, fmt.Errorf("七牛云存储尚未完整配置")
	}
	if strings.TrimSpace(options.SourceBase) == "" {
		options.SourceBase = "https://bgpt.huaweibio.com.cn"
	}
	if strings.TrimSpace(options.LocalRoot) == "" {
		options.LocalRoot = "."
	}

	query := `SELECT id, patient_code, COALESCE(report_files, ''), updated_at
		FROM detect_patient
		WHERE COALESCE(NULLIF(TRIM(report_files), ''), '') <> ''`
	args := make([]interface{}, 0, 2)
	if strings.TrimSpace(options.PatientCode) != "" {
		query += " AND patient_code = ?"
		args = append(args, strings.TrimSpace(options.PatientCode))
	}
	query += " ORDER BY id"
	if options.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, options.Limit)
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return result, err
	}
	defer rows.Close()

	patients := make([]patientReportMigrationRow, 0)
	for rows.Next() {
		var row patientReportMigrationRow
		if err := rows.Scan(&row.ID, &row.PatientCode, &row.ReportFiles, &row.UpdatedAt); err != nil {
			return result, err
		}
		patients = append(patients, row)
	}
	if err := rows.Err(); err != nil {
		return result, err
	}

	for _, patient := range patients {
		result.Patients++
		if !isValidPatientCode(patient.PatientCode) {
			result.Failed++
			result.Items = append(result.Items, PatientReportMigrationItem{
				PatientID: patient.ID, PatientCode: patient.PatientCode, Status: "failed", Error: "患者编号不是 HW 开头的有效编号",
			})
			continue
		}

		files := splitPatientReportFiles(patient.ReportFiles)
		replacements := make(map[string]string, len(files))
		objectNames := make(map[string]string, len(files))
		updatedFiles := make([]string, 0, len(files))
		patientFailed := false

		for index, source := range files {
			expectedPrefix := strings.TrimRight(config.Domain, "/") + "/uploads/patient_report/" +
				strings.ToUpper(patient.PatientCode) + "/" + strings.ToUpper(patient.PatientCode) + "_"
			if strings.HasPrefix(source, expectedPrefix) && strings.Contains(filepath.Base(strings.Split(source, "?")[0]), "_report") {
				result.Planned++
				result.Skipped++
				updatedFiles = append(updatedFiles, source)
				result.Items = append(result.Items, PatientReportMigrationItem{
					PatientID: patient.ID, PatientCode: patient.PatientCode, Source: source,
					ObjectName: strings.TrimPrefix(source, strings.TrimRight(config.Domain, "/")+"/"),
					TargetURL:  source, Status: "skipped",
				})
				continue
			}
			ext := patientReportSourceExtension(source)
			if ext == "" {
				ext = ".bin"
			}
			uploadedAt := patientReportUploadTime(source, patient.UpdatedAt)
			objectName := buildPatientReportObjectName(patient.PatientCode, "report"+ext, uploadedAt, index+1)
			targetURL := qiniuObjectURL(config.Domain, objectName)
			item := PatientReportMigrationItem{
				PatientID: patient.ID, PatientCode: patient.PatientCode, Source: source,
				ObjectName: objectName, TargetURL: targetURL, Status: "planned",
			}
			result.Planned++

			if !options.Apply {
				updatedFiles = append(updatedFiles, targetURL)
				result.Items = append(result.Items, item)
				continue
			}

			reader, contentType, openErr := openPatientReportSource(ctx, source, options.SourceBase, options.LocalRoot)
			if openErr != nil {
				item.Status = "failed"
				item.Error = openErr.Error()
				result.Failed++
				patientFailed = true
				result.Items = append(result.Items, item)
				continue
			}
			_, uploadErr := uploadFileToQiniu(reader, objectName, filepath.Base(objectName), contentType, config)
			_ = reader.Close()
			if uploadErr != nil {
				item.Status = "failed"
				item.Error = uploadErr.Error()
				result.Failed++
				patientFailed = true
				result.Items = append(result.Items, item)
				continue
			}

			item.Status = "uploaded"
			result.Uploaded++
			replacements[source] = targetURL
			objectNames[source] = objectName
			updatedFiles = append(updatedFiles, targetURL)
			result.Items = append(result.Items, item)
		}

		if !options.Apply || patientFailed {
			continue
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return result, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE detect_patient SET report_files = ?, updated_at = NOW() WHERE id = ?`,
			strings.Join(updatedFiles, ","), patient.ID); err != nil {
			_ = tx.Rollback()
			return result, err
		}
		if err := replaceMigratedPatientReportReferences(tx, patient.ID, strings.ToUpper(patient.PatientCode), replacements, objectNames); err != nil {
			_ = tx.Rollback()
			return result, err
		}
		if err := tx.Commit(); err != nil {
			return result, err
		}
	}
	return result, nil
}
