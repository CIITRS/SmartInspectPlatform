package handlers

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func getReportPDFMode(c *app.RequestContext) reportPDFMode {
	version := strings.ToLower(strings.TrimSpace(c.Query("version")))
	if version == "" {
		version = strings.ToLower(strings.TrimSpace(c.Query("type")))
	}
	if version == string(reportPDFModeConcise) || version == "simple" || version == "brief" {
		return reportPDFModeConcise
	}
	return reportPDFModeFull
}

func truncateWechatThing(value string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	if maxRunes <= 1 {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-1]) + "…"
}

func sendWechatReportSubscribeMessage(db *sql.DB, reportID int) {
	if db == nil || reportID <= 0 {
		return
	}
	var openID, patientName, cancerTypeName string
	var reviewedTime sql.NullTime
	var subscribeEnabled int
	err := db.QueryRow(`SELECT COALESCE(p.wechat_openid, ''), COALESCE(p.report_subscribe_enabled, 0),
			COALESCE(p.name, ''), COALESCE(ct.name, ''), r.reviewed_time
		FROM detect_report r
		LEFT JOIN detect_patient p ON r.patient_id = p.id
		LEFT JOIN detect_sample s ON r.sample_id = s.id
		LEFT JOIN setting_cancer_type ct ON s.cancer_type_id = ct.id
		WHERE r.id = ?`, reportID).Scan(&openID, &subscribeEnabled, &patientName, &cancerTypeName, &reviewedTime)
	if err != nil {
		log.Printf("Query report subscribe target error report=%d: %v", reportID, err)
		return
	}
	if strings.TrimSpace(openID) == "" || subscribeEnabled != 1 {
		return
	}
	accessToken, err := getWechatAccessToken()
	if err != nil {
		log.Printf("Get wechat access token for report subscribe error: %v", err)
		return
	}
	finishedAt := time.Now()
	if reviewedTime.Valid {
		finishedAt = reviewedTime.Time
	}
	payload := utils.H{
		"touser":      openID,
		"template_id": reportSubscribeTemplateID,
		"page":        "pages/patient/reports/index",
		"data": utils.H{
			"thing11": utils.H{"value": truncateWechatThing(patientName, 20)},
			"thing3":  utils.H{"value": truncateWechatThing(firstNonEmptyString(cancerTypeName, "检查报告"), 20)},
			"phrase4": utils.H{"value": "已完成"},
			"date2":   utils.H{"value": finishedAt.Format("2006-01-02 15:04:05")},
		},
	}
	body, _ := json.Marshal(payload)
	apiURL := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/message/subscribe/send?access_token=%s", accessToken)
	resp, err := http.Post(apiURL, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("Send report subscribe message error report=%d: %v", reportID, err)
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Errcode int    `json:"errcode"`
		Errmsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		log.Printf("Parse report subscribe response error report=%d: %v body=%s", reportID, err, string(respBody))
		return
	}
	if result.Errcode != 0 {
		log.Printf("Wechat report subscribe message failed report=%d code=%d msg=%s", reportID, result.Errcode, result.Errmsg)
	}
}

func generateReportPDFByMode(db *sql.DB, reportID int, mode reportPDFMode) (string, error) {
	if mode == reportPDFModeConcise {
		return generateConcisePDFReport(db, reportID)
	}
	return generatePDFReport(db, reportID)
}

func sanitizeReportFileNamePart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	replacer := strings.NewReplacer("\\", "_", "/", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return replacer.Replace(value)
}

func getSampleBatchStatus(db *sql.DB, sampleID int) (int, string, error) {
	var batchID sql.NullInt64
	var batchStatus sql.NullString
	err := db.QueryRow(`SELECT s.batch_id, COALESCE(b.status, '')
		FROM detect_sample s
		LEFT JOIN detect_batch b ON s.batch_id = b.id
		WHERE s.id = ?`, sampleID).Scan(&batchID, &batchStatus)
	if err != nil {
		return 0, "", err
	}
	if !batchID.Valid {
		return 0, "", nil
	}
	return int(batchID.Int64), strings.TrimSpace(strings.ToLower(batchStatus.String)), nil
}

func getSamePatientBatchSamples(db *sql.DB, sampleID int) []utils.H {
	rows, err := db.Query(`SELECT other.id, other.sample_code, COALESCE(other.sample_status, ''),
			COALESCE(st.name, ''), COALESCE(ts.name, ''), COALESCE(p.name, ''),
			COALESCE(other.receive_date, other.collection_date, other.sample_created_at), COALESCE(other.result_data, ''), COALESCE(other.signalvalue, 0)
		FROM detect_sample current
		JOIN detect_sample other ON other.batch_id = current.batch_id AND other.patient_id = current.patient_id
		LEFT JOIN setting_sample_type st ON other.sample_type_id = st.id
		LEFT JOIN setting_treatment_stage ts ON other.treatment_stage_id = ts.id
		LEFT JOIN detect_patient p ON other.patient_id = p.id
		WHERE current.id = ? AND current.batch_id IS NOT NULL AND current.patient_id IS NOT NULL
		ORDER BY CASE WHEN other.id = current.id THEN 0 ELSE 1 END, other.sample_created_at ASC, other.id ASC`, sampleID)
	if err != nil {
		log.Printf("Failed to query same patient batch samples: %v", err)
		return []utils.H{}
	}
	defer rows.Close()

	samples := []utils.H{}
	for rows.Next() {
		var id int
		var sampleCode, sampleStatus, sampleType, treatmentStageName, patientName, resultData string
		var sampleTime time.Time
		var signalValue float64
		if err := rows.Scan(&id, &sampleCode, &sampleStatus, &sampleType, &treatmentStageName, &patientName, &sampleTime, &resultData, &signalValue); err != nil {
			log.Printf("Failed to scan same patient batch sample: %v", err)
			continue
		}
		if strings.TrimSpace(resultData) != "" {
			resultDataMap := map[string]interface{}{}
			if err := json.Unmarshal([]byte(resultData), &resultDataMap); err == nil {
				if value := reportFloatValue(resultDataMap["signalValue"]); value != 0 {
					signalValue = value
				} else if value := reportFloatValue(resultDataMap["calculationResult"]); value != 0 {
					signalValue = value
				}
			}
		}
		samples = append(samples, utils.H{
			"id":                   id,
			"sampleCode":           sampleCode,
			"sample_code":          sampleCode,
			"status":               sampleStatus,
			"sampleType":           sampleType,
			"sample_type_name":     sampleType,
			"treatmentStageName":   treatmentStageName,
			"treatment_stage_name": treatmentStageName,
			"patientName":          patientName,
			"createdAt":            sampleTime.Format("2006-01-02T15:04:05+08:00"),
			"receive_date":         sampleTime.Format("2006-01-02T15:04:05+08:00"),
			"signalValue":          signalValue,
		})
	}
	return samples
}

func int64SliceToInterfaceSlice(values []int64) []interface{} {
	args := make([]interface{}, 0, len(values))
	for _, value := range values {
		args = append(args, value)
	}
	return args
}

func sampleHasResultData(resultData sql.NullString) bool {
	if !resultData.Valid {
		return false
	}
	value := strings.TrimSpace(resultData.String)
	if value == "" || value == "{}" || strings.EqualFold(value, "null") {
		return false
	}
	return true
}

func syncSampleStatusesAfterReportChangeTx(tx *sql.Tx, sampleIDs ...int) error {
	seen := map[int]bool{}
	for _, sampleID := range sampleIDs {
		if sampleID <= 0 || seen[sampleID] {
			continue
		}
		seen[sampleID] = true

		var activeReportCount int
		if err := tx.QueryRow(`SELECT COUNT(*)
			FROM detect_report
			WHERE sample_id = ? AND status IN ('pending', 'generated', 'reviewed', 'published')`, sampleID).Scan(&activeReportCount); err != nil {
			return err
		}

		nextStatus := "completed"
		if activeReportCount == 0 {
			var resultData sql.NullString
			if err := tx.QueryRow(`SELECT result_data FROM detect_sample WHERE id = ?`, sampleID).Scan(&resultData); err != nil {
				if err == sql.ErrNoRows {
					continue
				}
				return err
			}
			if sampleHasResultData(resultData) {
				nextStatus = "tested"
			} else {
				nextStatus = "received"
			}
		}

		if _, err := tx.Exec(`UPDATE detect_sample
			SET sample_status = ?, sample_updated_at = NOW()
			WHERE id = ? AND COALESCE(sample_status, '') <> ?`, nextStatus, sampleID, nextStatus); err != nil {
			return err
		}
	}
	return nil
}

func syncSampleStatusesAfterReportChange(db *sql.DB, sampleIDs ...int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if err := syncSampleStatusesAfterReportChangeTx(tx, sampleIDs...); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func getParentReportID(db *sql.DB, reportID int) int {
	var parentReportID sql.NullInt64
	var reportRole string
	if err := db.QueryRow(`SELECT COALESCE(report_role, 'single'), parent_report_id FROM detect_report WHERE id = ?`, reportID).Scan(&reportRole, &parentReportID); err != nil {
		return 0
	}
	if strings.EqualFold(reportRole, "child") && parentReportID.Valid && parentReportID.Int64 > 0 {
		return int(parentReportID.Int64)
	}
	return 0
}

func createChildReportsForSelectedHistories(db *sql.DB, parentReportID int64, parentReportNo string, patientID int, currentSampleID int, reportType string, reportData []byte, status string, generatedBy int, selectedHistoricalReports []map[string]interface{}) {
	if parentReportID <= 0 || currentSampleID <= 0 || len(selectedHistoricalReports) == 0 {
		return
	}
	seen := map[int]bool{}
	for _, history := range selectedHistoricalReports {
		sampleID := int(reportFloatValue(history["sampleId"]))
		if sampleID == 0 {
			sampleID = int(reportFloatValue(history["sample_id"]))
		}
		if sampleID == 0 {
			sampleCode := strings.TrimSpace(fmt.Sprint(history["sampleCode"]))
			if sampleCode == "" {
				sampleCode = strings.TrimSpace(fmt.Sprint(history["sample_code"]))
			}
			if sampleCode != "" {
				_ = db.QueryRow(`SELECT id FROM detect_sample WHERE sample_code = ? AND patient_id = ? LIMIT 1`, sampleCode, patientID).Scan(&sampleID)
			}
		}
		if sampleID <= 0 || sampleID == currentSampleID || seen[sampleID] {
			continue
		}
		seen[sampleID] = true

		var samplePatientID int
		if err := db.QueryRow(`SELECT COALESCE(patient_id, 0) FROM detect_sample WHERE id = ?`, sampleID).Scan(&samplePatientID); err != nil || samplePatientID != patientID {
			continue
		}

		var existingReportID int
		err := db.QueryRow(`SELECT id FROM detect_report WHERE sample_id = ? AND status NOT IN ('draft', 'rejected') LIMIT 1`, sampleID).Scan(&existingReportID)
		if err == nil && existingReportID > 0 {
			continue
		}

		_, _ = db.Exec("DELETE FROM detect_report WHERE sample_id = ? AND status IN ('draft', 'rejected', 'pending', 'generated')", sampleID)
		childReportNo := fmt.Sprintf("%s-S%d", parentReportNo, sampleID%1000)
		if len(childReportNo) > 30 {
			childReportNo = fmt.Sprintf("SUB_%d_%d", parentReportID, sampleID)
		}
		_, err = db.Exec(`INSERT INTO detect_report
			(sample_id, report_no, patient_id, report_type, report_data, status, generated_by, generated_time, parent_report_id, report_role, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, NOW(), ?, 'child', NOW(), NOW())`,
			sampleID, childReportNo, patientID, reportType, reportData, status, generatedBy, parentReportID)
		if err != nil {
			log.Printf("Failed to create child report for sample %d parent %d: %v", sampleID, parentReportID, err)
			if syncErr := syncSampleStatusesAfterReportChange(db, sampleID); syncErr != nil {
				log.Printf("Failed to sync sample status after child report create failure sample %d: %v", sampleID, syncErr)
			}
			continue
		}
		if syncErr := syncSampleStatusesAfterReportChange(db, sampleID); syncErr != nil {
			log.Printf("Failed to sync sample status after child report create sample %d: %v", sampleID, syncErr)
		}
	}
}

// 处理获取报告列表请求
func HandleListReports(c *app.RequestContext, db *sql.DB) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", c.DefaultQuery("pageSize", "10")))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize
	sampleCode := strings.TrimSpace(c.Query("sampleCode"))
	if sampleCode == "" {
		sampleCode = strings.TrimSpace(c.Query("sample_code"))
	}
	patientName := strings.TrimSpace(c.Query("patientName"))
	if patientName == "" {
		patientName = strings.TrimSpace(c.Query("patient_name"))
	}
	statusFilter := strings.TrimSpace(c.Query("status"))
	reportType := strings.TrimSpace(c.Query("reportType"))
	if reportType == "" {
		reportType = strings.TrimSpace(c.Query("report_type"))
	}
	startDate := strings.TrimSpace(c.Query("startDate"))
	if startDate == "" {
		startDate = strings.TrimSpace(c.Query("start_date"))
	}
	endDate := strings.TrimSpace(c.Query("endDate"))
	if endDate == "" {
		endDate = strings.TrimSpace(c.Query("end_date"))
	}
	salesPerson := strings.TrimSpace(c.Query("salesPerson"))
	if salesPerson == "" {
		salesPerson = strings.TrimSpace(c.Query("sales_person"))
	}
	cancerTypeID, _ := strconv.Atoi(c.DefaultQuery("cancerTypeId", c.DefaultQuery("cancer_type_id", "0")))
	where := []string{"1=1"}
	args := []interface{}{}
	if sampleCode != "" {
		where = append(where, "s.sample_code LIKE ?")
		args = append(args, "%"+sampleCode+"%")
	}
	if patientName != "" {
		where = append(where, "p.name LIKE ?")
		args = append(args, "%"+patientName+"%")
	}
	if statusFilter != "" {
		where = append(where, "r.status = ?")
		args = append(args, statusFilter)
	}
	if reportType != "" {
		where = append(where, "r.report_type = ?")
		args = append(args, reportType)
	}
	if startDate != "" {
		where = append(where, "COALESCE(r.generated_time, r.created_at) >= ?")
		args = append(args, startDate+" 00:00:00")
	}
	if endDate != "" {
		where = append(where, "COALESCE(r.generated_time, r.created_at) <= ?")
		args = append(args, endDate+" 23:59:59")
	}
	if salesPerson != "" {
		where = append(where, "p.sales_person = ?")
		args = append(args, salesPerson)
	}
	if cancerTypeID > 0 {
		where = append(where, "s.cancer_type_id = ?")
		args = append(args, cancerTypeID)
	}
	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := db.QueryRow(`SELECT COUNT(*)
		FROM detect_report r
		LEFT JOIN detect_sample s ON r.sample_id = s.id
		LEFT JOIN detect_patient p ON s.patient_id = p.id
		WHERE `+whereSQL, args...).Scan(&total); err != nil {
		log.Printf("Failed to count detect_reports: %v", err)
		total = 0
	}
	queryArgs := append([]interface{}{}, args...)
	queryArgs = append(queryArgs, pageSize, offset)
	// 从数据库查询报告列表
	rows, err := db.Query(`SELECT r.id, r.status, COALESCE(r.report_role, 'single'), COALESCE(r.parent_report_id, 0), r.created_at, r.updated_at, r.generated_time as generatedTime, r.reviewed_time as reviewedTime,
			COALESCE(r.generated_time, r.created_at) as reportDate, COALESCE(r.report_type, '') as reportType,
			p.name as detect_patientName, p.id_card as detect_patientIdCard, p.gender,
			s.sample_code as sampleCode, COALESCE(st.name, '') as sampleType, COALESCE(s.receive_date, s.collection_date) as sampleCollectedAt,
			COALESCE(s.cancer_type_id, 0) as cancerTypeId, COALESCE(ct.name, '') as cancerTypeName,
			COALESCE(p.sales_person, '') as salesPerson,
			COALESCE((SELECT COALESCE(mu.real_name, mu.username) FROM base_manage_user mu
				WHERE mu.employee_id = p.sales_person OR mu.username = p.sales_person
				ORDER BY (mu.employee_id = p.sales_person) DESC LIMIT 1), p.sales_person, '') as salesName,
			COALESCE(gu.real_name, gu.username) as generatedBy,
			COALESCE(ru.real_name, ru.username) as reviewedBy
			, r.patient_viewed_at, COALESCE(r.patient_view_count, 0)
		FROM detect_report r
		LEFT JOIN detect_sample s ON r.sample_id = s.id
		LEFT JOIN detect_patient p ON s.patient_id = p.id
		LEFT JOIN setting_sample_type st ON s.sample_type_id = st.id
		LEFT JOIN setting_cancer_type ct ON s.cancer_type_id = ct.id
		LEFT JOIN base_manage_user gu ON r.generated_by = gu.id
		LEFT JOIN base_manage_user ru ON r.reviewed_by = ru.id
		WHERE `+whereSQL+`
		ORDER BY r.created_at DESC
		LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		log.Printf("Failed to query detect_reports: %v", err)
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "获取报告列表成功",
			Data:    utils.H{"list": []utils.H{}, "total": 0},
		})
		return
	}
	defer rows.Close()

	// 遍历查询结果
	var detect_reports []utils.H
	for rows.Next() {
		var id, parentReportID, cancerTypeID int
		var status, reportRole, reportType, detect_patientName, detect_patientIdCard, sampleCode, sampleType, cancerTypeName, salesPerson, salesName, generatedBy, reviewedBy sql.NullString
		var gender sql.NullString
		var createdAt, updatedAt, generatedTime, reviewedTime, reportDate, sampleCollectedAt sql.NullTime
		var patientViewedAt sql.NullTime
		var patientViewCount int

		err := rows.Scan(&id, &status, &reportRole, &parentReportID, &createdAt, &updatedAt, &generatedTime, &reviewedTime, &reportDate, &reportType,
			&detect_patientName, &detect_patientIdCard, &gender, &sampleCode, &sampleType, &sampleCollectedAt,
			&cancerTypeID, &cancerTypeName, &salesPerson, &salesName, &generatedBy, &reviewedBy, &patientViewedAt, &patientViewCount)
		if err != nil {
			log.Printf("Failed to scan detect_report: %v", err)
			continue
		}

		// 计算年龄
		detect_patientAge := calculateAge(detect_patientIdCard.String)

		// 构建报告信息
		detect_report := utils.H{
			"id":               id,
			"sampleCode":       sampleCode.String,
			"patientName":      detect_patientName.String,
			"status":           status.String,
			"reportType":       reportType.String,
			"reportDate":       reportDate.Time.Format("2006-01-02T15:04:05+08:00"),
			"cancerTypeId":     cancerTypeID,
			"cancerTypeName":   cancerTypeName.String,
			"salesPerson":      salesPerson.String,
			"salesName":        salesName.String,
			"generatedBy":      generatedBy.String,
			"reviewedBy":       reviewedBy.String,
			"createdAt":        createdAt.Time.Format("2006-01-02T15:04:05+08:00"),
			"updatedAt":        updatedAt.Time.Format("2006-01-02T15:04:05+08:00"),
			"patientAge":       detect_patientAge,
			"gender":           gender.String,
			"sampleType":       sampleType.String,
			"patientViewed":    patientViewedAt.Valid,
			"patientViewCount": patientViewCount,
		}
		if patientViewedAt.Valid {
			detect_report["patientViewedAt"] = patientViewedAt.Time.Format("2006-01-02T15:04:05+08:00")
		}
		detect_report["reportRole"] = reportRole.String
		detect_report["report_role"] = reportRole.String
		detect_report["parentReportId"] = parentReportID
		detect_report["isChildReport"] = strings.EqualFold(reportRole.String, "child")

		// 添加生成时间和审核时间（如果存在）
		if generatedTime.Valid {
			detect_report["generatedTime"] = generatedTime.Time.Format("2006-01-02T15:04:05+08:00")
		}
		if reviewedTime.Valid {
			detect_report["reviewedTime"] = reviewedTime.Time.Format("2006-01-02T15:04:05+08:00")
		}
		if sampleCollectedAt.Valid {
			detect_report["sampleCollectedAt"] = sampleCollectedAt.Time.Format("2006-01-02")
		}

		detect_reports = append(detect_reports, detect_report)
	}

	// 检查遍历过程中是否有错误
	if err = rows.Err(); err != nil {
		log.Printf("Error iterating detect_reports: %v", err)
	}

	// 返回报告列表
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取报告列表成功",
		Data:    utils.H{"list": detect_reports, "total": total},
	})
}

// 处理获取未生成报告的样本列表请求
func HandleGetSamplesWithoutReports(c *app.RequestContext, db *sql.DB) {
	// 从数据库查询未生成报告的样本列表，包括报告类型为草稿的报告
	rows, err := db.Query(`SELECT s.id, s.sample_code as sampleCode, s.sample_status as detectionStatus,
				p.id as detect_patientId, p.name as detect_patientName, p.gender, p.id_card,
				s.sample_created_at as testDate,
				COALESCE(rep.id, 0) as detect_reportId, COALESCE(rep.status, '') as detect_reportStatus
			FROM detect_sample s
		LEFT JOIN detect_patient p ON s.patient_id = p.id
		LEFT JOIN detect_batch b ON s.batch_id = b.id
		LEFT JOIN detect_report rep ON s.id = rep.sample_id
		WHERE s.result_data IS NOT NULL
		AND b.status = 'submitted'
		AND (rep.id IS NULL OR rep.status IN ('draft', 'rejected'))
			ORDER BY s.sample_created_at DESC`)
	if err != nil {
		log.Printf("Failed to query samples without detect_reports: %v", err)
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "获取未生成报告的样本列表成功",
			Data:    utils.H{"list": []utils.H{}, "total": 0},
		})
		return
	}
	defer rows.Close()

	// 遍历查询结果
	var samples []utils.H
	for rows.Next() {
		var id, detect_patientId, detect_reportId int
		var sampleCode, detectionStatus, detect_patientName, gender, idCard, detect_reportStatus string
		var testDate time.Time

		err := rows.Scan(&id, &sampleCode, &detectionStatus, &detect_patientId, &detect_patientName, &gender, &idCard, &testDate, &detect_reportId, &detect_reportStatus)
		if err != nil {
			log.Printf("Failed to scan sample: %v", err)
			continue
		}

		// 计算年龄
		age := calculateAge(idCard)

		// 查询样本类型和治疗阶段
		var sampleType string
		var treatmentStageName string
		err = db.QueryRow("SELECT st.name, COALESCE(ts.name, '') FROM detect_sample s JOIN setting_sample_type st ON s.sample_type_id = st.id LEFT JOIN setting_treatment_stage ts ON s.treatment_stage_id = ts.id WHERE s.id = ?", id).Scan(&sampleType, &treatmentStageName)
		if err != nil {
			sampleType = ""
			treatmentStageName = ""
		}

		// 构建样本信息
		sample := utils.H{
			"id":                   id,
			"patientId":            detect_patientId,
			"patient_id":           detect_patientId,
			"sampleCode":           sampleCode,
			"sample_code":          sampleCode,
			"sampleType":           sampleType,
			"sampleTypeName":       sampleType,
			"sample_type_name":     sampleType,
			"treatmentStageName":   treatmentStageName,
			"treatment_stage_name": treatmentStageName,
			"detectionStatus":      detectionStatus,
			"patientName":          detect_patientName,
			"patient_name":         detect_patientName,
			"gender":               gender,
			"age":                  age,
			"testDate":             testDate.Format("2006-01-02"),
			"detect_reportId":      detect_reportId,
			"detect_reportStatus":  detect_reportStatus,
			"samePatientSamples":   getSamePatientBatchSamples(db, id),
		}

		samples = append(samples, sample)
	}

	// 检查遍历过程中是否有错误
	if err = rows.Err(); err != nil {
		log.Printf("Error iterating samples: %v", err)
	}

	// 返回未生成报告的样本列表
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取未生成报告的样本列表成功",
		Data:    utils.H{"list": samples, "total": len(samples)},
	})
}

// 处理获取待审核报告列表请求
func HandleGetPendingReviewReports(c *app.RequestContext, db *sql.DB) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", c.DefaultQuery("pageSize", "10")))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize
	sampleCode := strings.TrimSpace(c.Query("sampleCode"))
	if sampleCode == "" {
		sampleCode = strings.TrimSpace(c.Query("sample_code"))
	}
	patientName := strings.TrimSpace(c.Query("patientName"))
	if patientName == "" {
		patientName = strings.TrimSpace(c.Query("patient_name"))
	}
	reportType := strings.TrimSpace(c.Query("reportType"))
	if reportType == "" {
		reportType = strings.TrimSpace(c.Query("report_type"))
	}
	startDate := strings.TrimSpace(c.Query("startDate"))
	if startDate == "" {
		startDate = strings.TrimSpace(c.Query("start_date"))
	}
	endDate := strings.TrimSpace(c.Query("endDate"))
	if endDate == "" {
		endDate = strings.TrimSpace(c.Query("end_date"))
	}
	salesPerson := strings.TrimSpace(c.Query("salesPerson"))
	if salesPerson == "" {
		salesPerson = strings.TrimSpace(c.Query("sales_person"))
	}
	cancerTypeID, _ := strconv.Atoi(c.DefaultQuery("cancerTypeId", c.DefaultQuery("cancer_type_id", "0")))
	where := []string{"r.status IN ('pending', 'generated')"}
	args := []interface{}{}
	if sampleCode != "" {
		where = append(where, "s.sample_code LIKE ?")
		args = append(args, "%"+sampleCode+"%")
	}
	if patientName != "" {
		where = append(where, "p.name LIKE ?")
		args = append(args, "%"+patientName+"%")
	}
	if reportType != "" {
		where = append(where, "r.report_type = ?")
		args = append(args, reportType)
	}
	if startDate != "" {
		where = append(where, "COALESCE(r.generated_time, r.created_at) >= ?")
		args = append(args, startDate+" 00:00:00")
	}
	if endDate != "" {
		where = append(where, "COALESCE(r.generated_time, r.created_at) <= ?")
		args = append(args, endDate+" 23:59:59")
	}
	if salesPerson != "" {
		where = append(where, "p.sales_person = ?")
		args = append(args, salesPerson)
	}
	if cancerTypeID > 0 {
		where = append(where, "s.cancer_type_id = ?")
		args = append(args, cancerTypeID)
	}
	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := db.QueryRow(`SELECT COUNT(*)
		FROM detect_report r
		LEFT JOIN detect_sample s ON r.sample_id = s.id
		LEFT JOIN detect_patient p ON s.patient_id = p.id
		WHERE `+whereSQL, args...).Scan(&total); err != nil {
		log.Printf("Failed to count pending review detect_reports: %v", err)
		total = 0
	}
	queryArgs := append([]interface{}{}, args...)
	queryArgs = append(queryArgs, pageSize, offset)
	rows, err := db.Query(`SELECT r.id, COALESCE(s.id, 0) as sample_id, r.status, COALESCE(r.report_role, 'single'), COALESCE(r.parent_report_id, 0), r.created_at, r.updated_at, r.generated_time as generatedTime,
			COALESCE(r.generated_time, r.created_at) as reportDate,
			COALESCE(r.report_type, '') as reportType, COALESCE(r.report_data, '{}') as reportData,
			COALESCE(s.sample_code, '') as sampleCode,
			COALESCE(p.name, '') as detect_patientName,
			COALESCE(s.cancer_type_id, 0) as cancerTypeId, COALESCE(ct.name, '') as cancerTypeName,
			COALESCE(p.sales_person, '') as salesPerson,
			COALESCE((SELECT COALESCE(mu.real_name, mu.username) FROM base_manage_user mu
				WHERE mu.employee_id = p.sales_person OR mu.username = p.sales_person
				ORDER BY (mu.employee_id = p.sales_person) DESC LIMIT 1), p.sales_person, '') as salesName,
			COALESCE(gu.real_name, gu.username) as generatedBy
		FROM detect_report r
		LEFT JOIN detect_sample s ON r.sample_id = s.id
		LEFT JOIN detect_patient p ON s.patient_id = p.id
		LEFT JOIN setting_cancer_type ct ON s.cancer_type_id = ct.id
		LEFT JOIN base_manage_user gu ON r.generated_by = gu.id
		WHERE `+whereSQL+`
		ORDER BY r.created_at DESC
		LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		log.Printf("Failed to query pending review detect_reports: %v", err)
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "获取待审核报告列表成功",
			Data:    utils.H{"list": []utils.H{}, "total": 0},
		})
		return
	}
	defer rows.Close()

	// 遍历查询结果
	var detect_reports []utils.H
	for rows.Next() {
		var id, sampleId, parentReportID, cancerTypeID int
		var status, reportRole, reportType, reportData, sampleCode, detect_patientName, cancerTypeName, salesPerson, salesName, generatedBy sql.NullString
		var createdAt, updatedAt, generatedTime, reportDate sql.NullTime

		err := rows.Scan(&id, &sampleId, &status, &reportRole, &parentReportID, &createdAt, &updatedAt, &generatedTime, &reportDate,
			&reportType, &reportData, &sampleCode, &detect_patientName, &cancerTypeID, &cancerTypeName, &salesPerson, &salesName, &generatedBy)
		if err != nil {
			log.Printf("Failed to scan detect_report: %v", err)
			continue
		}

		// 构建报告信息
		detect_report := utils.H{
			"id":             id,
			"sampleId":       sampleId,
			"sampleCode":     sampleCode.String,
			"patientName":    detect_patientName.String,
			"reportType":     reportType.String,
			"reportData":     reportData.String,
			"status":         status.String,
			"reportDate":     reportDate.Time.Format("2006-01-02T15:04:05+08:00"),
			"cancerTypeId":   cancerTypeID,
			"cancerTypeName": cancerTypeName.String,
			"salesPerson":    salesPerson.String,
			"salesName":      salesName.String,
			"generatedBy":    generatedBy.String,
			"createdAt":      createdAt.Time.Format("2006-01-02T15:04:05+08:00"),
			"updatedAt":      updatedAt.Time.Format("2006-01-02T15:04:05+08:00"),
		}
		detect_report["reportRole"] = reportRole.String
		detect_report["report_role"] = reportRole.String
		detect_report["parentReportId"] = parentReportID
		detect_report["isChildReport"] = strings.EqualFold(reportRole.String, "child")

		// 添加生成时间（如果存在）
		if generatedTime.Valid {
			detect_report["generatedTime"] = generatedTime.Time.Format("2006-01-02T15:04:05+08:00")
		}

		detect_reports = append(detect_reports, detect_report)
	}

	// 检查遍历过程中是否有错误
	if err = rows.Err(); err != nil {
		log.Printf("Error iterating detect_reports: %v", err)
	}

	// 返回待审核报告列表
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取待审核报告列表成功",
		Data:    utils.H{"list": detect_reports, "total": total},
	})
}

func findPostoperativePairReportBySampleCode(db *sql.DB, sampleCode string) int {
	sampleCode = strings.TrimSpace(sampleCode)
	if sampleCode == "" {
		return 0
	}

	var sampleID, patientID, batchID int
	var stageName string
	if err := db.QueryRow(`
		SELECT s.id, COALESCE(s.patient_id, 0), COALESCE(s.batch_id, 0), COALESCE(ts.name, '')
		FROM detect_sample s
		LEFT JOIN setting_treatment_stage ts ON s.treatment_stage_id = ts.id
		WHERE s.sample_code = ?
		LIMIT 1`, sampleCode).Scan(&sampleID, &patientID, &batchID, &stageName); err != nil {
		return 0
	}
	if sampleID == 0 || patientID == 0 || batchID == 0 || (!isPreoperativeStage(stageName) && !isPostoperativeStage(stageName)) {
		return 0
	}

	var pairedStageCount int
	if err := db.QueryRow(`
		SELECT COUNT(DISTINCT CASE
			WHEN ts.name LIKE '%术前%' THEN 'pre'
			WHEN ts.name IN ('术后检测', '术后', '手术后') THEN 'post'
		END)
		FROM detect_sample s
		LEFT JOIN setting_treatment_stage ts ON s.treatment_stage_id = ts.id
		WHERE s.patient_id = ? AND s.batch_id = ?
			AND (ts.name LIKE '%术前%' OR ts.name IN ('术后检测', '术后', '手术后'))`, patientID, batchID).Scan(&pairedStageCount); err != nil || pairedStageCount < 2 {
		return 0
	}

	var reportID int
	if err := db.QueryRow(`
		SELECT r.id
		FROM detect_report r
		JOIN detect_sample s ON r.sample_id = s.id
		LEFT JOIN setting_treatment_stage ts ON s.treatment_stage_id = ts.id
		WHERE s.patient_id = ? AND s.batch_id = ?
			AND ts.name IN ('术后检测', '术后', '手术后')
			AND r.status NOT IN ('rejected')
		ORDER BY r.created_at DESC, r.id DESC
		LIMIT 1`, patientID, batchID).Scan(&reportID); err != nil {
		return 0
	}
	return reportID
}

func resolveReportIDParam(db *sql.DB, param string) (int, error) {
	param = strings.TrimSpace(param)
	if param == "" {
		return 0, sql.ErrNoRows
	}
	if id, err := strconv.Atoi(param); err == nil {
		var exists int
		if err := db.QueryRow("SELECT COUNT(*) FROM detect_report WHERE id = ?", id).Scan(&exists); err == nil && exists > 0 {
			return id, nil
		}
	}
	if pairedReportID := findPostoperativePairReportBySampleCode(db, param); pairedReportID > 0 {
		return pairedReportID, nil
	}
	var id int
	err := db.QueryRow(`
		SELECT r.id
		FROM detect_report r
		JOIN detect_sample s ON r.sample_id = s.id
		WHERE s.sample_code = ? AND r.status <> 'rejected'
		ORDER BY CASE WHEN COALESCE(r.parent_report_id, 0) = 0 THEN 0 ELSE 1 END,
			r.created_at DESC, r.id DESC
		LIMIT 1`, param).Scan(&id)
	return id, err
}

// 处理获取报告详情请求
func HandleGetReportById(c *app.RequestContext, db *sql.DB) {
	// 报告详情的公开入口统一使用样本编号。即使样本编号完全由数字组成，
	// 也不能再回退为 detect_report.id，否则会把真实样本编号误判为报告 ID。
	sampleCodeParam := strings.TrimSpace(c.Param("id"))
	if sampleCodeParam == "" {
		c.JSON(consts.StatusNotFound, ApiResponse{Code: 404, Success: false, Message: "报告不存在", Data: nil})
		return
	}

	// 从数据库查询报告详情，包括模型和癌种信息
	var detect_reportId, sampleId, generatedBy, reviewedBy, detect_patientId, testOperator, cancerTypeId, modelId, modelDeprecated int
	var status, reportType, sampleCode, detect_patientName, gender, generatedByName, reviewedByName, testOperatorName, detect_patientBirthdate, cancerTypeName, modelName, modelVersion string
	var createdAt, updatedAt, generatedTime, reviewedTime sql.NullTime

	query := `SELECT r.id, COALESCE(s.id, 0) as sample_id, r.status, COALESCE(r.report_type, ''), r.created_at, r.updated_at, r.generated_time, r.reviewed_time, COALESCE(r.generated_by, 0), COALESCE(r.reviewed_by, 0), COALESCE(NULLIF(s.test_operator, 0), NULLIF(b.tester_id, 0), 0),
			COALESCE(s.sample_code, ''),
			COALESCE(p.name, ''),
			COALESCE(p.gender, ''),
			COALESCE(p.id_card, ''),
			COALESCE(p.id, 0),
		COALESCE(s.result_cancer_type_id, 0),
		COALESCE(ct.name, ''),
		COALESCE(s.model_id, 0),
		COALESCE(ms.model_name, ''),
			COALESCE(ms.version, ''),
			COALESCE(ms.is_deprecated, 0),
			COALESCE(gu.real_name, COALESCE(gu.username, '')),
		COALESCE(ru.real_name, COALESCE(ru.username, '')),
		COALESCE(uu.real_name, COALESCE(uu.username, ''))
		FROM detect_report r
		LEFT JOIN detect_sample s ON r.sample_id = s.id
			LEFT JOIN detect_batch b ON s.batch_id = b.id
			LEFT JOIN setting_cancer_type ct ON s.cancer_type_id = ct.id
		LEFT JOIN setting_model ms ON s.model_id = ms.id
			LEFT JOIN detect_patient p ON COALESCE(s.patient_id, r.patient_id) = p.id
			LEFT JOIN base_manage_user gu ON r.generated_by = gu.id
			LEFT JOIN base_manage_user ru ON r.reviewed_by = ru.id
			LEFT JOIN base_manage_user uu ON COALESCE(NULLIF(s.test_operator, 0), NULLIF(b.tester_id, 0)) = uu.id
		WHERE %s AND r.status <> 'rejected'
		ORDER BY CASE WHEN COALESCE(r.parent_report_id, 0) = 0 THEN 0 ELSE 1 END,
			r.created_at DESC, r.id DESC
		LIMIT 1`
	args := []interface{}{sampleCodeParam}
	where := "s.sample_code = ?"
	if pairedReportID := findPostoperativePairReportBySampleCode(db, sampleCodeParam); pairedReportID > 0 {
		where = "r.id = ?"
		args = []interface{}{pairedReportID}
	}
	query = fmt.Sprintf(query, where)

	err := db.QueryRow(query, args...).Scan(
		&detect_reportId, &sampleId, &status, &reportType, &createdAt, &updatedAt, &generatedTime, &reviewedTime, &generatedBy, &reviewedBy, &testOperator, &sampleCode, &detect_patientName, &gender, &detect_patientBirthdate, &detect_patientId, &cancerTypeId, &cancerTypeName, &modelId, &modelName, &modelVersion, &modelDeprecated, &generatedByName, &reviewedByName, &testOperatorName)
	if err != nil {
		log.Printf("Failed to query detect_report: %v", err)
		c.JSON(consts.StatusNotFound, ApiResponse{
			Code:    404,
			Success: false,
			Message: "报告不存在",
			Data:    nil,
		})
		return
	}

	// 先确认报告存在，再做患者权限判断，以便准确区分“不存在”和“无权限”。
	if userID, ok := GetUserID(c); ok {
		if accessFilter, accessArgs := patientAccessFilterForUser(db, userID, "p"); accessFilter != "" {
			permissionQuery := "SELECT COUNT(*) FROM detect_patient p WHERE p.id = ? AND " + accessFilter
			permissionArgs := append([]interface{}{detect_patientId}, accessArgs...)
			var accessible int
			if err := db.QueryRow(permissionQuery, permissionArgs...).Scan(&accessible); err != nil || accessible == 0 {
				c.JSON(consts.StatusForbidden, ApiResponse{Code: 403, Success: false, Message: "无权限查看此报告", Data: nil})
				return
			}
		}
	}

	// 计算患者年龄
	detect_patientAge := calculateAge(detect_patientBirthdate)

	// 从数据库查询报告数据JSON
	var detect_reportDataJSON []byte
	err = db.QueryRow("SELECT report_data FROM detect_report WHERE id = ?", detect_reportId).Scan(&detect_reportDataJSON)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Failed to query report_data: %v", err)
	}

	// 初始化报告数据映射
	var detect_reportDataMap map[string]interface{}
	detect_reportDataMap = make(map[string]interface{})

	// 解析报告数据JSON
	if detect_reportDataJSON != nil {
		err = json.Unmarshal(detect_reportDataJSON, &detect_reportDataMap)
		if err != nil {
			log.Printf("Failed to unmarshal report_data: %v", err)
		}
	}

	// 从报告数据JSON中获取样本类型、组织信息和治疗阶段
	// 不再查询样本库，以报告生成时存储的数据为准
	var sampleType string
	var organization string
	var treatmentStageName string

	// 从detect_report_data中获取值
	if val, ok := detect_reportDataMap["sampleType"].(string); ok {
		sampleType = val
	}
	if val, ok := detect_reportDataMap["organization"].(string); ok {
		organization = val
	}
	if val, ok := detect_reportDataMap["treatmentStageName"].(string); ok {
		treatmentStageName = val
	}

	// 查询样本采样时间（如果需要）
	var sampleCollectedAt sql.NullTime
	if sampleId > 0 {
		var sampleTypeFromDB, organizationFromDB, treatmentStageFromDB string
		err = db.QueryRow(`
			SELECT COALESCE(st.name, ''), COALESCE(s.organization, ''), COALESCE(ts.name, ''), COALESCE(s.receive_date, s.collection_date)
			FROM detect_sample s
			LEFT JOIN setting_sample_type st ON s.sample_type_id = st.id
			LEFT JOIN setting_treatment_stage ts ON s.treatment_stage_id = ts.id
			WHERE s.id = ?`, sampleId).Scan(&sampleTypeFromDB, &organizationFromDB, &treatmentStageFromDB, &sampleCollectedAt)
		if err != nil {
			log.Printf("Failed to query sample collected time: %v", err)
			sampleCollectedAt = sql.NullTime{}
		} else {
			if strings.TrimSpace(sampleType) == "" {
				sampleType = sampleTypeFromDB
			}
			if strings.TrimSpace(organization) == "" {
				organization = organizationFromDB
			}
			if strings.TrimSpace(treatmentStageName) == "" {
				treatmentStageName = treatmentStageFromDB
			}
		}
	}

	currentReportRow := ReportHistoryRow{
		Time:   reportDateString(detect_reportDataMap["time1"]),
		Signal: reportFloatValue(detect_reportDataMap["signal1"]),
		Trend:  normalizeReportTrend(reportStringValue(detect_reportDataMap["trend1"])),
		Type:   reportStringValue(detect_reportDataMap["type1"]),
		Note:   reportStringValue(detect_reportDataMap["note1"]),
	}
	if currentReportRow.Time == "" && sampleCollectedAt.Valid {
		currentReportRow.Time = sampleCollectedAt.Time.Format("2006-01-02")
	}
	if currentReportRow.Signal == 0 {
		currentReportRow.Signal = reportFloatValue(detect_reportDataMap["calculationResult"])
	}
	if currentReportRow.Type == "" {
		currentReportRow.Type = treatmentStageName
	}
	if currentReportRow.Note == "" {
		currentReportRow.Note = reportStringValue(detect_reportDataMap["remarks"])
	}
	syncReportHistoryFields(detect_reportDataMap, currentReportRow, nil)

	// 查询文件路径
	var filePath, pdfGenerationStatus string
	err = db.QueryRow("SELECT COALESCE(file_path, ''), COALESCE(pdf_generation_status, '') FROM detect_report WHERE id = ?", detect_reportId).Scan(&filePath, &pdfGenerationStatus)
	if err != nil {
		filePath = ""
	}

	// 构建带版本的模型名称
	modelNameWithVersion := "-"
	if modelName != "" {
		modelNameWithVersion = modelName
		if modelVersion != "" {
			modelNameWithVersion = fmt.Sprintf("%s [V%s]", modelNameWithVersion, modelVersion)
		}
		if modelDeprecated == 1 {
			modelNameWithVersion = fmt.Sprintf("%s【弃用模型】", modelNameWithVersion)
		}
	}

	// 从报告数据中获取核心字段
	var calculationResult interface{} = nil
	var originalCalculationResult interface{} = nil
	var calculationModified interface{} = false
	var selectedModelId interface{} = nil
	var geneData interface{} = nil
	var resultExplanation interface{} = nil
	var signalValueExplanation interface{} = nil
	var selectedHistoricalReports interface{} = nil
	var remarks interface{} = nil
	var trend interface{} = nil
	var time1 interface{} = nil
	var signal1 interface{} = nil
	var trend1 interface{} = nil
	var type1 interface{} = nil
	var note1 interface{} = nil
	var time2 interface{} = nil
	var signal2 interface{} = nil
	var trend2 interface{} = nil
	var type2 interface{} = nil
	var note2 interface{} = nil
	var time3 interface{} = nil
	var signal3 interface{} = nil
	var trend3 interface{} = nil
	var type3 interface{} = nil
	var note3 interface{} = nil
	var time4 interface{} = nil
	var signal4 interface{} = nil
	var trend4 interface{} = nil
	var type4 interface{} = nil
	var note4 interface{} = nil

	// 从detect_report_data中获取值
	if val, ok := detect_reportDataMap["calculationResult"]; ok {
		calculationResult = val
	}
	if val, ok := detect_reportDataMap["originalCalculationResult"]; ok {
		originalCalculationResult = val
	}
	if val, ok := detect_reportDataMap["calculationModified"]; ok {
		calculationModified = val
	}
	if val, ok := detect_reportDataMap["selectedModelId"]; ok {
		selectedModelId = val
	}
	if val, ok := detect_reportDataMap["geneData"]; ok {
		geneData = val
	}
	if val, ok := detect_reportDataMap["resultExplanation"]; ok {
		resultExplanation = val
	}
	if val, ok := detect_reportDataMap["signalValueExplanation"]; ok {
		signalValueExplanation = val
	}
	if val, ok := detect_reportDataMap["selectedHistoricalReports"]; ok {
		selectedHistoricalReports = val
	}
	if val, ok := detect_reportDataMap["remarks"]; ok {
		remarks = val
	}
	if val, ok := detect_reportDataMap["trend"]; ok {
		trend = val
	}
	if val, ok := detect_reportDataMap["time1"]; ok {
		time1 = val
	}
	if val, ok := detect_reportDataMap["signal1"]; ok {
		signal1 = val
	}
	if val, ok := detect_reportDataMap["trend1"]; ok {
		trend1 = val
	}
	if val, ok := detect_reportDataMap["type1"]; ok {
		type1 = val
	}
	if val, ok := detect_reportDataMap["note1"]; ok {
		note1 = val
	}
	if val, ok := detect_reportDataMap["time2"]; ok {
		time2 = val
	}
	if val, ok := detect_reportDataMap["signal2"]; ok {
		signal2 = val
	}
	if val, ok := detect_reportDataMap["trend2"]; ok {
		trend2 = val
	}
	if val, ok := detect_reportDataMap["type2"]; ok {
		type2 = val
	}
	if val, ok := detect_reportDataMap["note2"]; ok {
		note2 = val
	}
	if val, ok := detect_reportDataMap["time3"]; ok {
		time3 = val
	}
	if val, ok := detect_reportDataMap["signal3"]; ok {
		signal3 = val
	}
	if val, ok := detect_reportDataMap["trend3"]; ok {
		trend3 = val
	}
	if val, ok := detect_reportDataMap["type3"]; ok {
		type3 = val
	}
	if val, ok := detect_reportDataMap["note3"]; ok {
		note3 = val
	}
	if val, ok := detect_reportDataMap["time4"]; ok {
		time4 = val
	}
	if val, ok := detect_reportDataMap["signal4"]; ok {
		signal4 = val
	}
	if val, ok := detect_reportDataMap["trend4"]; ok {
		trend4 = val
	}
	if val, ok := detect_reportDataMap["type4"]; ok {
		type4 = val
	}
	if val, ok := detect_reportDataMap["note4"]; ok {
		note4 = val
	}

	// 构建报告信息
	detect_report := utils.H{
		"id":                        detect_reportId,
		"sampleId":                  sampleId,
		"patientId":                 detect_patientId,
		"status":                    status,
		"reportType":                normalizeAssignedReportType(reportType),
		"reportTypeLabel":           reportTypeDisplayLabel(reportType),
		"createdAt":                 createdAt.Time.Format("2006-01-02T15:04:05+08:00"),
		"updatedAt":                 updatedAt.Time.Format("2006-01-02T15:04:05+08:00"),
		"sampleCode":                sampleCode,
		"patientName":               detect_patientName,
		"gender":                    gender,
		"patientAge":                detect_patientAge,
		"sampleType":                sampleType,
		"cancerTypeId":              cancerTypeId,
		"cancerTypeName":            cancerTypeName,
		"modelId":                   modelId,
		"modelName":                 modelNameWithVersion,
		"modelDeprecated":           modelDeprecated == 1,
		"generatedBy":               generatedByName,
		"reviewedBy":                reviewedByName,
		"inspector":                 testOperatorName,
		"reporter":                  generatedByName,
		"detect_reporter":           generatedByName,
		"reviewer":                  reviewedByName,
		"calculationResult":         calculationResult,
		"originalCalculationResult": originalCalculationResult,
		"calculationModified":       calculationModified,
		"selectedModelId":           selectedModelId,
		"geneData":                  geneData,
		"resultExplanation":         resultExplanation,
		"signalValueExplanation":    signalValueExplanation,
		"organization":              organization,
		"treatmentStageName":        treatmentStageName,
		"selectedHistoricalReports": selectedHistoricalReports,
		"remarks":                   remarks,
		"trend":                     trend,
		"time1":                     time1,
		"signal1":                   signal1,
		"trend1":                    trend1,
		"type1":                     type1,
		"note1":                     note1,
		"time2":                     time2,
		"signal2":                   signal2,
		"trend2":                    trend2,
		"type2":                     type2,
		"note2":                     note2,
		"time3":                     time3,
		"signal3":                   signal3,
		"trend3":                    trend3,
		"type3":                     type3,
		"note3":                     note3,
		"time4":                     time4,
		"signal4":                   signal4,
		"trend4":                    trend4,
		"type4":                     type4,
		"note4":                     note4,
	}

	// 如果有文件路径，生成安全的临时URL
	if filePath != "" && strings.EqualFold(pdfGenerationStatus, "completed") {
		// 生成3分钟过期的安全URL
		tempURL, err := GenerateSecureFileURL(filePath, filepath.Base(filePath), 3*time.Minute)
		if err != nil {
			log.Printf("生成临时文件URL失败: %v", err)
			// 不再暴露文件路径，只记录错误
			detect_report["previewUrl"] = ""
			detect_report["fileError"] = "文件预览链接生成失败"
		} else {
			detect_report["previewUrl"] = tempURL
		}
	}
	detect_report["pdfGenerationStatus"] = pdfGenerationStatus

	// 添加生成时间和审核时间（如果存在）
	if generatedTime.Valid {
		detect_report["generatedTime"] = generatedTime.Time.Format("2006-01-02T15:04:05+08:00")
	}
	if reviewedTime.Valid {
		detect_report["reviewedTime"] = reviewedTime.Time.Format("2006-01-02T15:04:05+08:00")
	}
	// 添加采样日期（如果存在）
	if sampleCollectedAt.Valid {
		detect_report["sampleCollectedAt"] = sampleCollectedAt.Time.Format("2006-01-02")
	}

	// 查询患者历史报告（已审核或已发布的）
	var historicalReports []utils.H
	if detect_patientId > 0 {
		rows, err := db.Query(`SELECT r.id, r.created_at, r.generated_time
			FROM detect_report r
			WHERE r.patient_id = ? AND r.id != ? AND r.status IN ('reviewed', 'published')
			ORDER BY r.created_at DESC
			LIMIT 3`, detect_patientId, detect_reportId)
		if err != nil {
			log.Printf("Failed to query historical detect_reports: %v", err)
		} else {
			defer rows.Close()
			for rows.Next() {
				var histReportId int
				var histCreatedAt, histGeneratedTime sql.NullTime
				err := rows.Scan(&histReportId, &histCreatedAt, &histGeneratedTime)
				if err != nil {
					log.Printf("Failed to scan historical detect_report: %v", err)
					continue
				}

				// 构建历史报告信息
				histReport := utils.H{
					"id":                 histReportId,
					"signalValue":        nil,
					"createdAt":          histCreatedAt.Time.Format("2006-01-02"),
					"treatmentStageName": nil,
					"remarks":            nil,
				}
				if histGeneratedTime.Valid {
					histReport["generatedTime"] = histGeneratedTime.Time.Format("2006-01-02")
				}

				historicalReports = append(historicalReports, histReport)
			}
		}
	}

	var mergeReportCandidates []utils.H
	if detect_patientId > 0 && sampleId > 0 {
		rows, err := db.Query(`SELECT other_r.id, other_s.sample_code, other_r.created_at, other_r.generated_time, other_r.report_data, COALESCE(other_ts.name, '')
			FROM detect_report current_r
			JOIN detect_sample current_s ON current_s.id = current_r.sample_id
			JOIN detect_sample other_s ON other_s.batch_id = current_s.batch_id
			JOIN detect_report other_r ON other_r.sample_id = other_s.id
			LEFT JOIN setting_treatment_stage other_ts ON other_s.treatment_stage_id = other_ts.id
			WHERE current_r.id = ? AND current_s.batch_id IS NOT NULL
				AND other_r.id != current_r.id AND other_r.patient_id = current_r.patient_id
			ORDER BY other_r.created_at DESC`, detect_reportId)
		if err != nil {
			log.Printf("Failed to query merge report candidates: %v", err)
		} else {
			defer rows.Close()
			for rows.Next() {
				var candidateID int
				var candidateSampleCode, candidateReportData, candidateStageName string
				var candidateCreatedAt, candidateGeneratedTime sql.NullTime
				if err := rows.Scan(&candidateID, &candidateSampleCode, &candidateCreatedAt, &candidateGeneratedTime, &candidateReportData, &candidateStageName); err != nil {
					log.Printf("Failed to scan merge report candidate: %v", err)
					continue
				}
				candidateData := make(map[string]interface{})
				if strings.TrimSpace(candidateReportData) != "" {
					_ = json.Unmarshal([]byte(candidateReportData), &candidateData)
				}
				if value := reportStringValue(candidateData["treatmentStageName"]); value != "" {
					candidateStageName = value
				}
				if treatmentStageRank(candidateStageName) >= treatmentStageRank(treatmentStageName) {
					continue
				}
				candidateTime := ""
				if value, ok := candidateData["time1"].(string); ok && strings.TrimSpace(value) != "" {
					candidateTime = value
				} else if candidateGeneratedTime.Valid {
					candidateTime = candidateGeneratedTime.Time.Format("2006-01-02")
				} else if candidateCreatedAt.Valid {
					candidateTime = candidateCreatedAt.Time.Format("2006-01-02")
				}
				candidateSignal := reportFloatValue(candidateData["signal1"])
				if candidateSignal == 0 {
					candidateSignal = reportFloatValue(candidateData["calculationResult"])
				}
				mergeReportCandidates = append(mergeReportCandidates, utils.H{
					"id":         candidateID,
					"sampleCode": candidateSampleCode,
					"time":       candidateTime,
					"signal":     candidateSignal,
					"trend":      candidateData["trend"],
					"type":       candidateStageName,
					"note":       candidateData["remarks"],
				})
			}
		}
	}

	// 计算趋势
	// 趋势已经从报告数据中获取，不需要在这里重新计算
	// var trend string
	// trend = "-"

	// 添加历史报告和趋势到报告信息
	detect_report["historicalReports"] = historicalReports
	detect_report["mergeReportCandidates"] = mergeReportCandidates
	detect_report["trend"] = trend

	// 设置NumberID为样本编号(sample_code)
	detect_report["NumberID"] = sampleCode

	// 返回报告详情
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取报告详情成功",
		Data:    detect_report,
	})
}

// 处理生成报告请求
func HandleGenerateReport(c *app.RequestContext, db *sql.DB) {
	// 历史报告数据结构
	type HistoricalReport struct {
		Time   string  `json:"time"`
		Signal float64 `json:"signal"`
		Type   string  `json:"type"`
		Trend  string  `json:"trend"`
		Note   string  `json:"note"`
	}

	// 解析请求体
	var req struct {
		SampleId                  int                    `json:"SampleId" binding:"required"`
		ReportType                string                 `json:"ReportType" binding:"required"`
		CalculationResult         float64                `json:"CalculationResult"`
		SelectedModelId           int                    `json:"SelectedModelId"`
		GeneData                  map[string]interface{} `json:"geneData"`
		ResultExplanation         string                 `json:"ResultExplanation" binding:"required"`
		SignalValueExplanation    string                 `json:"SignalValueExplanation" binding:"required"`
		Organization              string                 `json:"Organization"`
		SelectedHistoricalReports []HistoricalReport     `json:"SelectedHistoricalReports"`
		TreatmentStageName        string                 `json:"treatmentStageName"`
		SampleType                string                 `json:"sampleType"`
		Remarks                   string                 `json:"remarks"`
		Trend                     string                 `json:"trend"`
		Time1                     string                 `json:"time1"`
		Signal1                   float64                `json:"signal1"`
		Trend1                    string                 `json:"trend1"`
		Type1                     string                 `json:"type1"`
		Note1                     string                 `json:"note1"`
		Time2                     string                 `json:"time2"`
		Signal2                   float64                `json:"signal2"`
		Trend2                    string                 `json:"trend2"`
		Type2                     string                 `json:"type2"`
		Note2                     string                 `json:"note2"`
		Time3                     string                 `json:"time3"`
		Signal3                   float64                `json:"signal3"`
		Trend3                    string                 `json:"trend3"`
		Type3                     string                 `json:"type3"`
		Note3                     string                 `json:"note3"`
		Time4                     string                 `json:"time4"`
		Signal4                   float64                `json:"signal4"`
		Trend4                    string                 `json:"trend4"`
		Type4                     string                 `json:"type4"`
		Note4                     string                 `json:"note4"`
		PrimarySampleId           int                    `json:"primarySampleId"`
		PrimarySampleID           int                    `json:"primarySampleID"`
		SecondarySampleIds        []int                  `json:"secondarySampleIds"`
		SecondarySampleIDs        []int                  `json:"secondarySampleIDs"`
	}

	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请求参数错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}
	// 转换GeneData为map[string]float64，处理字符串类型的值并过滤掉非基因字段
	geneDataFloat := make(map[string]float64)
	for gene, value := range req.GeneData {
		// 跳过非基因字段，如sqrt
		if gene == "sqrt" {
			continue
		}

		// 处理不同类型的值
		switch v := value.(type) {
		case float64:
			geneDataFloat[gene] = v
		case int:
			geneDataFloat[gene] = float64(v)
		case string:
			if floatVal, err := strconv.ParseFloat(v, 64); err == nil {
				geneDataFloat[gene] = floatVal
			}
		}
	}

	// 检查样本是否存在
	var sampleExists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM detect_sample WHERE id = ?)", req.SampleId).Scan(&sampleExists)
	if err != nil || !sampleExists {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "样本不存在",
			Data:    nil,
		})
		return
	}

	primarySampleID := req.PrimarySampleId
	if primarySampleID == 0 {
		primarySampleID = req.PrimarySampleID
	}
	if primarySampleID == 0 {
		primarySampleID = req.SampleId
	}
	secondarySampleIDs := append([]int{}, req.SecondarySampleIds...)
	secondarySampleIDs = append(secondarySampleIDs, req.SecondarySampleIDs...)
	reportSampleIDs := []int{primarySampleID}
	seenReportSampleIDs := map[int]bool{primarySampleID: true}
	for _, sampleID := range secondarySampleIDs {
		if sampleID <= 0 || seenReportSampleIDs[sampleID] {
			continue
		}
		reportSampleIDs = append(reportSampleIDs, sampleID)
		seenReportSampleIDs[sampleID] = true
	}
	if !seenReportSampleIDs[req.SampleId] {
		reportSampleIDs = append(reportSampleIDs, req.SampleId)
		seenReportSampleIDs[req.SampleId] = true
	}

	var baseBatchID, basePatientID int
	for index, sampleID := range reportSampleIDs {
		batchID, batchStatus, err := getSampleBatchStatus(db, sampleID)
		if err != nil || batchID == 0 {
			c.JSON(consts.StatusBadRequest, ApiResponse{
				Code:    400,
				Success: false,
				Message: "样本未关联已提交批次，不能生成报告",
				Data:    nil,
			})
			return
		}
		if batchStatus != "submitted" {
			c.JSON(consts.StatusBadRequest, ApiResponse{
				Code:    400,
				Success: false,
				Message: "只有已提交批次才能生成报告",
				Data:    nil,
			})
			return
		}
		var samplePatientID int
		if err := db.QueryRow("SELECT COALESCE(patient_id, 0) FROM detect_sample WHERE id = ?", sampleID).Scan(&samplePatientID); err != nil || samplePatientID == 0 {
			c.JSON(consts.StatusBadRequest, ApiResponse{
				Code:    400,
				Success: false,
				Message: "该样本没有患者信息",
				Data:    nil,
			})
			return
		}
		if index == 0 {
			baseBatchID = batchID
			basePatientID = samplePatientID
		} else if batchID != baseBatchID || samplePatientID != basePatientID {
			c.JSON(consts.StatusBadRequest, ApiResponse{
				Code:    400,
				Success: false,
				Message: "主报告和子报告必须属于同一批次、同一患者",
				Data:    nil,
			})
			return
		}
	}
	req.SampleId = primarySampleID

	// 获取患者ID
	var detect_patientId int
	var sampleTreatmentStageName string
	var sampleReportTime sql.NullTime
	log.Printf("Querying detect_patient_id for sample_id: %d", req.SampleId)
	// 直接查询sample表获取患者ID
	err = db.QueryRow(`
		SELECT s.patient_id, COALESCE(ts.name, ''), COALESCE(s.receive_date, s.collection_date, s.sample_created_at)
		FROM detect_sample s
		LEFT JOIN setting_treatment_stage ts ON s.treatment_stage_id = ts.id
		WHERE s.id = ?`, req.SampleId).Scan(&detect_patientId, &sampleTreatmentStageName, &sampleReportTime)
	if err != nil {
		log.Printf("Error querying sample: %v", err)
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "该样本没有患者信息",
			Data:    nil,
		})
		return
	}
	log.Printf("Found sample: sample_id=%d, detect_patient_id=%d", req.SampleId, detect_patientId)

	for _, sampleID := range reportSampleIDs {
		var detect_reportExists bool
		var detect_reportStatus string
		err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM detect_report WHERE sample_id = ?), COALESCE((SELECT status FROM detect_report WHERE sample_id = ? ORDER BY created_at DESC LIMIT 1), '')", sampleID, sampleID).Scan(&detect_reportExists, &detect_reportStatus)
		if err != nil || (detect_reportExists && detect_reportStatus != "draft" && detect_reportStatus != "rejected") {
			c.JSON(consts.StatusBadRequest, ApiResponse{
				Code:    400,
				Success: false,
				Message: "样本已经生成过报告",
				Data:    nil,
			})
			return
		}
		if detect_reportExists && (detect_reportStatus == "draft" || detect_reportStatus == "rejected") {
			_, err = db.Exec("DELETE FROM detect_report WHERE sample_id = ?", sampleID)
			if err != nil {
				log.Printf("删除报告失败: %v", err)
				c.JSON(consts.StatusInternalServerError, ApiResponse{
					Code:    500,
					Success: false,
					Message: "服务器内部错误",
					Data:    utils.H{"error": err.Error()},
				})
				return
			}
		}
	}

	// 生成报告编号
	detect_reportNo := fmt.Sprintf("REPORT_%s_%d", time.Now().Format("20060102"), time.Now().UnixNano()%10000)

	// 计算趋势
	trend := req.Trend
	if trend == "" {
		trend = "-"
	}
	if len(req.SelectedHistoricalReports) > 0 {
		// 获取本次检测的信号值
		currentSignal := req.CalculationResult

		// 找到最近的历史检测
		var latestHistory *HistoricalReport
		var latestTime time.Time

		for _, history := range req.SelectedHistoricalReports {
			// 解析检测时间
			historyTime, err := time.Parse("2006-01-02", history.Time)
			if err != nil {
				continue
			}

			// 找到最新的历史记录
			if latestHistory == nil || historyTime.After(latestTime) {
				latestHistory = &history
				latestTime = historyTime
			}
		}

		// 计算趋势
		if latestHistory != nil {
			trend = calculateReportTrend(currentSignal, latestHistory.Signal)
		}
	}

	// 确保selectedHistoricalReports不为空
	selectedHistoricalReports := req.SelectedHistoricalReports
	if selectedHistoricalReports == nil {
		selectedHistoricalReports = []HistoricalReport{}
	}
	if req.TreatmentStageName == "" {
		req.TreatmentStageName = sampleTreatmentStageName
	}
	currentTime := req.Time1
	if strings.TrimSpace(currentTime) == "" {
		if sampleReportTime.Valid {
			currentTime = sampleReportTime.Time.Format("2006-01-02")
		} else {
			currentTime = time.Now().Format("2006-01-02")
		}
	}
	if len(selectedHistoricalReports) == 0 {
		for _, history := range loadSameBatchPriorStageHistories(db, req.SampleId, detect_patientId, req.TreatmentStageName, 0) {
			selectedHistoricalReports = append(selectedHistoricalReports, HistoricalReport{
				Time:   history.Time,
				Signal: history.Signal,
				Type:   history.Type,
				Trend:  history.Trend,
				Note:   history.Note,
			})
		}
	}
	historyRows := make([]ReportHistoryRow, 0, len(selectedHistoricalReports))
	for _, history := range selectedHistoricalReports {
		historyRows = append(historyRows, ReportHistoryRow{
			Time:   history.Time,
			Signal: history.Signal,
			Type:   history.Type,
			Trend:  history.Trend,
			Note:   history.Note,
		})
	}
	currentReportRow := ReportHistoryRow{
		Time:   currentTime,
		Signal: req.CalculationResult,
		Type:   req.Type1,
		Note:   req.Note1,
	}
	if currentReportRow.Type == "" {
		currentReportRow.Type = req.TreatmentStageName
	}

	// 构建报告数据JSON
	detect_reportData := map[string]interface{}{
		"calculationResult":         req.CalculationResult,
		"selectedModelId":           req.SelectedModelId,
		"geneData":                  geneDataFloat,
		"resultExplanation":         req.ResultExplanation,
		"signalValueExplanation":    req.SignalValueExplanation,
		"organization":              req.Organization,
		"selectedHistoricalReports": selectedHistoricalReports,
		"treatmentStageName":        req.TreatmentStageName,
		"sampleType":                req.SampleType,
		"remarks":                   req.Remarks,
		"trend":                     trend,
		"time1":                     currentReportRow.Time,
		"signal1":                   currentReportRow.Signal,
		"trend1":                    currentReportRow.Trend,
		"type1":                     currentReportRow.Type,
		"note1":                     currentReportRow.Note,
		"time2":                     "",
		"signal2":                   0,
		"trend2":                    "",
		"type2":                     "",
		"note2":                     "",
		"time3":                     "",
		"signal3":                   0,
		"trend3":                    "",
		"type3":                     "",
		"note3":                     "",
		"time4":                     "",
		"signal4":                   0,
		"trend4":                    "",
		"type4":                     "",
		"note4":                     "",
	}
	syncReportHistoryFields(detect_reportData, currentReportRow, historyRows)

	// 处理历史报告趋势
	for i, history := range selectedHistoricalReports {
		// 跳过本次检测（第一个）
		if i == 0 {
			continue
		}

		// 为历史报告计算趋势
		if history.Trend == "" {
			// 解析当前历史报告的时间
			historyTime, err := time.Parse("2006-01-02", history.Time)
			if err != nil {
				continue
			}

			// 找到该历史报告的前一次检测（时间上更早的最近一次）
			var prevHistory *HistoricalReport
			var prevTime time.Time

			for j := i - 1; j > 0; j-- {
				prevHistoryTime, err := time.Parse("2006-01-02", selectedHistoricalReports[j].Time)
				if err != nil {
					continue
				}

				// 找到时间上更早但最接近的历史记录
				if prevHistoryTime.Before(historyTime) {
					if prevHistory == nil || prevHistoryTime.After(prevTime) {
						prevHistory = &selectedHistoricalReports[j]
						prevTime = prevHistoryTime
					}
				}
			}

			// 计算趋势
			if prevHistory != nil {
				history.Trend = calculateReportTrend(history.Signal, prevHistory.Signal)
			}
		}

		// 更新报告数据中的历史报告
		selectedHistoricalReports[i] = history
	}

	// 更新selectedHistoricalReports
	detect_reportData["selectedHistoricalReports"] = selectedHistoricalReports

	detect_reportDataJSON, err := json.Marshal(detect_reportData)
	if err != nil {
		log.Printf("Failed to marshal detect_report data: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 从上下文中获取用户ID（生成者ID）
	userID, exists := c.Get(UserIDKey)
	var generatedBy int
	if exists {
		generatedBy = userID.(int)
	} else {
		generatedBy = 0 // 默认为0，表示系统生成
	}

	assignedReportType := normalizeAssignedReportType(req.ReportType)

	reportInsertSampleIDs := append([]int{}, reportSampleIDs...)
	reportIDs := make([]int64, 0, len(reportInsertSampleIDs))
	var parentReportID interface{} = nil
	for index, sampleID := range reportInsertSampleIDs {
		reportRole := "single"
		parentValue := interface{}(nil)
		if len(reportSampleIDs) > 1 {
			if index == 0 {
				reportRole = "primary"
			} else {
				reportRole = "child"
				parentValue = parentReportID
			}
		}
		reportNo := detect_reportNo
		if index > 0 {
			reportNo = fmt.Sprintf("%s_%d", detect_reportNo, index+1)
		}
		result, err := db.Exec(`INSERT INTO detect_report (sample_id, report_no, patient_id, report_type, report_data, status, generated_by, generated_time, parent_report_id, report_role, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, 'draft', ?, NOW(), ?, ?, NOW(), NOW())`,
			sampleID, reportNo, detect_patientId, assignedReportType, detect_reportDataJSON, generatedBy, parentValue, reportRole)
		if err != nil {
			log.Printf("Failed to generate detect_report: %v", err)
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code:    500,
				Success: false,
				Message: "服务器内部错误",
				Data:    utils.H{"error": err.Error()},
			})
			return
		}
		insertedID, err := result.LastInsertId()
		if err != nil {
			log.Printf("Failed to get last insert ID: %v", err)
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code:    500,
				Success: false,
				Message: "服务器内部错误",
				Data:    utils.H{"error": err.Error()},
			})
			return
		}
		if index == 0 {
			parentReportID = insertedID
		}
		reportIDs = append(reportIDs, insertedID)
	}
	detect_reportId := reportIDs[0]

	generatedSampleIDs := append([]int{}, reportInsertSampleIDs...)

	// 异步处理报告生成流程
	go func() {
		// 更新状态为generating
		_, err := db.Exec("UPDATE detect_report SET status = 'generating', updated_at = NOW() WHERE id IN ("+strings.TrimRight(strings.Repeat("?,", len(reportIDs)), ",")+")", int64SliceToInterfaceSlice(reportIDs)...)
		if err != nil {
			log.Printf("Failed to update status to generating: %v", err)
			return
		}

		// 模拟生成过程
		time.Sleep(2 * time.Second)

		// 更新状态为pending
		_, err = db.Exec("UPDATE detect_report SET status = 'pending', updated_at = NOW() WHERE id IN ("+strings.TrimRight(strings.Repeat("?,", len(reportIDs)), ",")+")", int64SliceToInterfaceSlice(reportIDs)...)
		if err != nil {
			log.Printf("Failed to update status to pending: %v", err)
			return
		}
		if err := syncSampleStatusesAfterReportChange(db, generatedSampleIDs...); err != nil {
			log.Printf("Failed to sync sample statuses after report generation: %v", err)
		}
	}()

	// 返回生成的报告ID
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "报告生成成功",
		Data:    utils.H{"id": detect_reportId, "ids": reportIDs},
	})
}

// 处理审核报告请求
func HandleReviewReport(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	param := c.Param("id")

	// 尝试将参数转换为整数（旧格式：数据库ID）
	id, err := strconv.Atoi(param)
	if err != nil {
		// 参数不是数字，尝试作为样本编号查询报告ID
		err = db.QueryRow(`SELECT r.id FROM detect_report r LEFT JOIN detect_sample s ON r.sample_id = s.id WHERE s.sample_code = ?`, param).Scan(&id)
		if err != nil {
			c.JSON(consts.StatusBadRequest, ApiResponse{
				Code:    400,
				Success: false,
				Message: "无效的报告ID或样本编号",
				Data:    nil,
			})
			return
		}
	}

	// 解析请求体
	var req struct {
		Status         string `json:"status" binding:"required"`
		RejectedReason string `json:"rejectedReason"`
		Remarks        string `json:"remarks"`
		ReviewerID     int    `json:"reviewer_id"`
	}

	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请求参数错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}
	if req.Status != "reviewed" && req.Status != "rejected" && req.Status != "pending" {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的状态值",
			Data:    nil,
		})
		return
	}

	// 从上下文中获取用户ID和用户名（审核者）
	userID, exists := c.Get(UserIDKey)
	var reviewedBy int
	if exists {
		reviewedBy = userID.(int)
	} else {
		reviewedBy = 0 // 默认为0，表示系统审核
	}
	if reviewedBy > 0 {
		roleNames := getUserRoleNames(db, reviewedBy)
		if hasRoleName(roleNames, "销售") && !hasRoleName(roleNames, "管理员", "IT") {
			c.JSON(consts.StatusForbidden, ApiResponse{
				Code:    403,
				Success: false,
				Message: "销售账号无审核报告权限",
				Data:    nil,
			})
			return
		}
	}
	if req.ReviewerID > 0 {
		if !isValidReportReviewer(db, req.ReviewerID) {
			c.JSON(consts.StatusBadRequest, ApiResponse{
				Code:    400,
				Success: false,
				Message: "请选择管理角色且工号不为admin的审核人",
				Data:    nil,
			})
			return
		}
		reviewedBy = req.ReviewerID
	} else if req.Status == "reviewed" && (reviewedBy == 0 || mustSelectRealReportReviewer(db, reviewedBy)) {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请选择真实审核人",
			Data:    nil,
		})
		return
	}

	// 获取审核者用户名
	var reviewerName string
	err = db.QueryRow("SELECT username FROM base_manage_user WHERE id = ?", reviewedBy).Scan(&reviewerName)
	if err != nil {
		reviewerName = "系统"
	}

	// 开始事务
	tx, err := db.Begin()
	if err != nil {
		log.Printf("Failed to begin transaction: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 获取当前报告状态和同批次同患者信息
	var currentStatus string
	var currentPatientID, currentBatchID, currentSampleID int
	err = tx.QueryRow(`SELECT r.status, COALESCE(r.patient_id, 0), COALESCE(s.batch_id, 0), COALESCE(r.sample_id, 0)
		FROM detect_report r
		LEFT JOIN detect_sample s ON r.sample_id = s.id
		WHERE r.id = ?`, id).Scan(&currentStatus, &currentPatientID, &currentBatchID, &currentSampleID)
	if err != nil {
		tx.Rollback()
		log.Printf("Failed to get current detect_report status: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	reviewedReportIDs := []int{id}
	affectedSampleIDs := []int{currentSampleID}

	// 更新报告状态
	var updateErr error
	if req.Status == "rejected" {
		// 审核拒绝
		_, updateErr = tx.Exec("UPDATE detect_report SET status = ?, reviewed_by = ?, reviewed_time = NOW(), updated_at = NOW() WHERE id = ?",
			req.Status, reviewedBy, id)
	} else if req.Status == "reviewed" {
		// 审核通过，重置PDF生成状态为pending
		_, updateErr = tx.Exec("UPDATE detect_report SET status = ?, reviewed_by = ?, reviewed_time = NOW(), file_path = '', pdf_generation_status = 'pending', updated_at = NOW() WHERE id = ?",
			req.Status, reviewedBy, id)
		if updateErr == nil && currentPatientID > 0 && currentBatchID > 0 {
			linkedRows, linkedErr := tx.Query(`SELECT r.id, COALESCE(r.sample_id, 0)
				FROM detect_report r
				LEFT JOIN detect_sample s ON r.sample_id = s.id
				WHERE r.patient_id = ? AND s.batch_id = ? AND r.id <> ? AND r.status IN ('pending', 'generated', 'draft')`,
				currentPatientID, currentBatchID, id)
			if linkedErr != nil {
				updateErr = linkedErr
			} else {
				for linkedRows.Next() {
					var linkedID int
					var linkedSampleID int
					if scanErr := linkedRows.Scan(&linkedID, &linkedSampleID); scanErr == nil {
						reviewedReportIDs = append(reviewedReportIDs, linkedID)
						affectedSampleIDs = append(affectedSampleIDs, linkedSampleID)
					}
				}
				if rowsErr := linkedRows.Err(); rowsErr != nil {
					updateErr = rowsErr
				}
				linkedRows.Close()
			}
			if updateErr == nil {
				for _, linkedID := range reviewedReportIDs {
					if linkedID == id {
						continue
					}
					if _, err := tx.Exec("UPDATE detect_report SET status = ?, reviewed_by = ?, reviewed_time = NOW(), file_path = '', pdf_generation_status = 'pending', updated_at = NOW() WHERE id = ?",
						req.Status, reviewedBy, linkedID); err != nil {
						updateErr = err
						break
					}
				}
			}
		}
	} else if req.Status == "pending" {
		// 反审核，回退到待审核状态
		_, updateErr = tx.Exec("UPDATE detect_report SET status = ?, reviewed_by = ?, reviewed_time = NOW(), updated_at = NOW() WHERE id = ?",
			req.Status, reviewedBy, id)
	}

	if updateErr != nil {
		tx.Rollback()
		log.Printf("Failed to update detect_report status: %v", updateErr)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": updateErr.Error()},
		})
		return
	}
	if err := syncSampleStatusesAfterReportChangeTx(tx, affectedSampleIDs...); err != nil {
		tx.Rollback()
		log.Printf("Failed to sync sample status after report review: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "同步样本状态失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		log.Printf("Failed to commit transaction: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	if req.Status == "reviewed" && currentStatus != "reviewed" {
		for _, reportID := range reviewedReportIDs {
			go sendReportReadySMS(db, reportID)
			go sendWechatReportSubscribeMessage(db, reportID)
		}
	}

	// 返回成功响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "报告审核成功",
		Data:    utils.H{"reviewedReportIds": reviewedReportIDs},
	})
}

// 处理下载患者报告请求
func HandleDownloadPatientReport(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	param := c.Param("id")

	// 尝试将参数转换为整数（旧格式：数据库ID）
	detect_reportId, err := strconv.Atoi(param)
	if err != nil {
		if pairedReportID := findPostoperativePairReportBySampleCode(db, param); pairedReportID > 0 {
			detect_reportId = pairedReportID
			err = nil
		}
	}
	if err != nil {
		// 参数不是数字，尝试作为样本编号查询报告ID
		err = db.QueryRow(`SELECT r.id FROM detect_report r LEFT JOIN detect_sample s ON r.sample_id = s.id WHERE s.sample_code = ?`, param).Scan(&detect_reportId)
		if err != nil {
			c.JSON(consts.StatusBadRequest, ApiResponse{
				Code:    400,
				Success: false,
				Message: "无效的报告ID或样本编号",
				Data:    nil,
			})
			return
		}
	}

	// 清理过期的PDF文件（超过1分钟的）
	cleanupExpiredPDFs()

	// 从数据库查询报告信息
	var sampleCode, detect_patientName string
	err = db.QueryRow(`SELECT s.sample_code, p.name 
		FROM detect_report r 
		LEFT JOIN detect_sample s ON r.sample_id = s.id 
		LEFT JOIN detect_patient p ON s.patient_id = p.id 
		WHERE r.id = ?`, detect_reportId).Scan(&sampleCode, &detect_patientName)
	if err != nil {
		log.Printf("Failed to query detect_report for download: %v", err)
		c.JSON(consts.StatusNotFound, ApiResponse{
			Code:    404,
			Success: false,
			Message: "报告不存在或文件未生成",
			Data:    nil,
		})
		return
	}

	mode := getReportPDFMode(c)
	effectiveReportID := detect_reportId
	if parentReportID := getParentReportID(db, detect_reportId); parentReportID > 0 {
		effectiveReportID = parentReportID
	}
	filePath, err := generateReportPDFByMode(db, effectiveReportID, mode)
	if err != nil {
		log.Printf("Failed to generate PDF report: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "生成PDF失败",
			Data:    nil,
		})
		return
	}

	// 生成安全的临时URL（1分钟有效期，可多次下载）
	reportName := fmt.Sprintf("报告_%s_%s_%s.pdf", sampleCode, detect_patientName, mode)
	tempURL, err := fileURLManager.GenerateOneTimeFileURL(filePath, 5*time.Minute)
	if err != nil {
		log.Printf("生成临时文件URL失败: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "生成下载链接失败",
			Data:    nil,
		})
		return
	}

	// 返回临时URL
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取下载链接成功",
		Data: utils.H{
			"downloadUrl": tempURL,
			"fileName":    reportName,
		},
	})
}

// cleanupExpiredPDFs 清理未被下载流程正常回收的临时报告文件。
func cleanupExpiredPDFs() {
	cleanupManagedTemporaryFiles(generatedReportOrphanMaxAge)
}

// HandleDownloadReportPdf 处理报告PDF下载请求（现场生成）
func HandleDownloadReportPdf(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	param := c.Param("id")

	// 尝试将参数转换为整数
	detect_reportId, err := strconv.Atoi(param)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的报告ID",
			Data:    nil,
		})
		return
	}

	// 清理过期的PDF文件
	cleanupExpiredPDFs()

	// 从数据库查询报告信息
	var sampleCode, detect_patientName string
	err = db.QueryRow(`SELECT s.sample_code, p.name 
		FROM detect_report r 
		LEFT JOIN detect_sample s ON r.sample_id = s.id 
		LEFT JOIN detect_patient p ON s.patient_id = p.id 
		WHERE r.id = ?`, detect_reportId).Scan(&sampleCode, &detect_patientName)
	if err != nil {
		log.Printf("Failed to query detect_report for download: %v", err)
		c.JSON(consts.StatusNotFound, ApiResponse{
			Code:    404,
			Success: false,
			Message: "报告不存在",
			Data:    nil,
		})
		return
	}

	mode := getReportPDFMode(c)
	filePath, err := generateReportPDFByMode(db, detect_reportId, mode)
	if err != nil {
		log.Printf("Failed to generate PDF report: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "生成PDF失败",
			Data:    nil,
		})
		return
	}
	// 设置响应头，触发下载
	fileName := fmt.Sprintf("报告_%s_%s_%s.pdf", sampleCode, detect_patientName, mode)
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", "attachment; filename="+fileName)
	// The same URL is reused after report edits, so public/browser caching can
	// return an obsolete full PDF even though report_data has been updated.
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	// 读取文件并返回
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("Failed to read PDF file: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "读取PDF文件失败",
			Data:    nil,
		})
		return
	}

	c.Data(consts.StatusOK, "application/pdf", fileData)
	scheduleManagedTemporaryFileRemoval(filePath)
}

// HandleDownloadConciseTestPdf generates a concise report PDF with fixed test data.
// It is used from the report generation page to visually verify template alignment
// before a real report record exists.
func HandleDownloadConciseTestPdf(c *app.RequestContext, db *sql.DB) {
	cleanupExpiredPDFs()

	reportType := strings.TrimSpace(c.Query("reportType"))
	if reportType == "" {
		reportType = "normal"
	}
	reportType = normalizeAssignedReportType(reportType)

	sampleTypeID, _ := strconv.Atoi(strings.TrimSpace(c.Query("sampleTypeId")))
	if sampleTypeID <= 0 {
		sampleTypeID = 1
	}

	now := time.Now()
	reportData := map[string]interface{}{
		"calculationResult": 36.8,
		"time1":             "2026-07-01",
		"signal1":           36.8,
		"trend1":            "-",
		"type1":             "辅助诊断",
		"note1":             "本次检测",
		"time2":             "2026-04-08",
		"signal2":           28.4,
		"trend2":            "↑",
		"type2":             "术前评估",
		"note2":             "历史一",
		"time3":             "2026-01-15",
		"signal3":           18.6,
		"trend3":            "↓",
		"type3":             "残留检测",
		"note3":             "历史二",
		"time4":             "2025-10-20",
		"signal4":           52.1,
		"trend4":            "↑",
		"type4":             "复发监测",
		"note4":             "历史三",
		"selectedHistoricalReports": []interface{}{
			map[string]interface{}{"time": "2026-04-08", "signal": 28.4, "trend": "↑", "type": "术前评估", "note": "历史一"},
			map[string]interface{}{"time": "2026-01-15", "signal": 18.6, "trend": "↓", "type": "残留检测", "note": "历史二"},
			map[string]interface{}{"time": "2025-10-20", "signal": 52.1, "trend": "↑", "type": "复发监测", "note": "历史三"},
		},
	}

	outputDir := filepath.Join("file", "temp", "detect_report")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Printf("Failed to create concise test PDF dir: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "创建临时文件失败"})
		return
	}
	filePath := filepath.Join(outputDir, fmt.Sprintf("concise_test_%d.pdf", now.UnixNano()))
	err := FillPDFFormFixed(
		db,
		sampleTypeID,
		reportType,
		filePath,
		reportPDFModeConcise,
		"测试患者",
		"女",
		48,
		"华微智检测试送检单位",
		now,
		[]utils.H{},
		"外周血",
		"2026-07-01",
		"检验员A",
		"审核员B",
		36.8,
		"MePlex检出阳性标记信号，检测信号值36.8为【中风险】结果，这可能暗示存在肿瘤发生的风险。建议除临床常规监测外，定期复检ctDNA甲基化观察信号进行性变化情况。",
		"基于多中心队列研究，本检测通过检测ctDNA甲基化异常模式，结合机器学习模型生成信号值风险评分。此处为测试数据，用于检查简洁版PDF所有文本框是否与模板坐标对齐。",
		0,
		"HWT20260701001",
		"智朗-肺",
		"辅助诊断",
		reportData,
	)
	if err != nil {
		log.Printf("Failed to generate concise test PDF: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "生成测试PDF失败"})
		return
	}
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("Failed to read concise test PDF: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "读取测试PDF失败"})
		return
	}

	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", "attachment; filename=concise-test-report.pdf")
	c.Header("Cache-Control", "no-store")
	c.Data(consts.StatusOK, "application/pdf", fileData)
	scheduleManagedTemporaryFileRemoval(filePath)
}

// HandleBatchDownloadReports 批量生成报告PDF并打包为ZIP下载。
func HandleBatchDownloadReports(c *app.RequestContext, db *sql.DB) {
	var req struct {
		IDs     []int  `json:"ids"`
		Version string `json:"version"`
	}
	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请求参数错误",
			Data:    nil,
		})
		return
	}
	if len(req.IDs) == 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请选择要下载的报告",
			Data:    nil,
		})
		return
	}

	mode := reportPDFModeFull
	if strings.EqualFold(strings.TrimSpace(req.Version), string(reportPDFModeConcise)) {
		mode = reportPDFModeConcise
	}

	cleanupExpiredPDFs()
	zipDir := filepath.Join("file", "temp", "detect_report")
	if err := os.MkdirAll(zipDir, 0755); err != nil {
		log.Printf("Failed to create report temp directory: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "创建临时目录失败", Data: nil})
		return
	}

	zipName := fmt.Sprintf("reports_%s_%d.zip", mode, time.Now().UnixNano())
	zipPath := filepath.Join(zipDir, zipName)
	zipFile, err := os.Create(zipPath)
	if err != nil {
		log.Printf("Failed to create reports zip: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "创建压缩包失败", Data: nil})
		return
	}

	zipWriter := zip.NewWriter(zipFile)
	added := 0
	for _, reportID := range req.IDs {
		var sampleCode, detect_patientName, status string
		err := db.QueryRow(`SELECT COALESCE(s.sample_code, ''), COALESCE(p.name, ''), COALESCE(r.status, '')
			FROM detect_report r
			LEFT JOIN detect_sample s ON r.sample_id = s.id
			LEFT JOIN detect_patient p ON s.patient_id = p.id
			WHERE r.id = ?`, reportID).Scan(&sampleCode, &detect_patientName, &status)
		if err != nil {
			log.Printf("Skip report %d while batching: %v", reportID, err)
			continue
		}
		if status != "reviewed" && status != "published" {
			log.Printf("Skip report %d with status %s while batching", reportID, status)
			continue
		}

		effectiveReportID := reportID
		if parentReportID := getParentReportID(db, reportID); parentReportID > 0 {
			effectiveReportID = parentReportID
		}
		filePath, err := generateReportPDFByMode(db, effectiveReportID, mode)
		if err != nil {
			log.Printf("Skip report %d due to PDF generation error: %v", reportID, err)
			continue
		}

		file, err := os.Open(filePath)
		if err != nil {
			log.Printf("Skip report %d due to PDF open error: %v", reportID, err)
			_ = os.Remove(filePath)
			continue
		}

		entryName := fmt.Sprintf("报告_%s_%s_%s.pdf",
			sanitizeReportFileNamePart(sampleCode),
			sanitizeReportFileNamePart(detect_patientName),
			mode,
		)
		entry, err := zipWriter.Create(entryName)
		if err != nil {
			log.Printf("Skip report %d due to ZIP entry error: %v", reportID, err)
			_ = file.Close()
			_ = os.Remove(filePath)
			continue
		}
		if _, err := io.Copy(entry, file); err != nil {
			log.Printf("Skip report %d due to ZIP copy error: %v", reportID, err)
			_ = file.Close()
			_ = os.Remove(filePath)
			continue
		}
		_ = file.Close()
		_ = os.Remove(filePath)
		added++
	}

	if err := zipWriter.Close(); err != nil {
		_ = zipFile.Close()
		_ = os.Remove(zipPath)
		log.Printf("Failed to close reports zip: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "生成压缩包失败", Data: nil})
		return
	}
	if err := zipFile.Close(); err != nil {
		_ = os.Remove(zipPath)
		log.Printf("Failed to close reports zip file: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "生成压缩包失败", Data: nil})
		return
	}

	if added == 0 {
		_ = os.Remove(zipPath)
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "没有可下载的已审核报告",
			Data:    nil,
		})
		return
	}

	downloadURL, err := fileURLManager.GenerateOneTimeFileURL(zipPath, 10*time.Minute)
	if err != nil {
		_ = os.Remove(zipPath)
		log.Printf("生成批量下载链接失败: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "生成下载链接失败", Data: nil})
		return
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "生成批量下载链接成功",
		Data: utils.H{
			"downloadUrl": downloadURL,
			"fileName":    fmt.Sprintf("报告批量下载_%s.zip", mode),
			"count":       added,
		},
	})
}

// 处理获取报告PDF状态请求
func HandleGetReportPdfStatus(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	param := c.Param("id")

	// 尝试将参数转换为整数（旧格式：数据库ID）
	detect_reportId, err := strconv.Atoi(param)
	if err != nil {
		// 参数不是数字，尝试作为样本编号查询报告ID
		err = db.QueryRow(`SELECT r.id FROM detect_report r LEFT JOIN detect_sample s ON r.sample_id = s.id WHERE s.sample_code = ?`, param).Scan(&detect_reportId)
		if err != nil {
			c.JSON(consts.StatusBadRequest, ApiResponse{
				Code:    400,
				Success: false,
				Message: "无效的报告ID或样本编号",
				Data:    nil,
			})
			return
		}
	}

	// 从数据库查询报告状态
	var status, filePath, pdfGenerationStatus string
	err = db.QueryRow("SELECT status, COALESCE(file_path, ''), COALESCE(pdf_generation_status, '') FROM detect_report WHERE id = ?", detect_reportId).Scan(&status, &filePath, &pdfGenerationStatus)
	if err != nil {
		log.Printf("Failed to query detect_report status: %v", err)
		c.JSON(consts.StatusNotFound, ApiResponse{
			Code:    404,
			Success: false,
			Message: "报告不存在",
			Data:    nil,
		})
		return
	}

	// 检查PDF文件是否存在
	pdfExists := false
	if strings.EqualFold(pdfGenerationStatus, "completed") && filePath != "" && fileExists(filePath) {
		pdfExists = true
	}

	// 返回PDF状态
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取PDF状态成功",
		Data: utils.H{
			"status":              status,
			"pdfExists":           pdfExists,
			"pdfGenerationStatus": pdfGenerationStatus,
		},
	})
}

// 处理更新报告状态请求
func HandleUpdateReportStatus(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	param := c.Param("id")

	// 尝试将参数转换为整数（旧格式：数据库ID）
	detect_reportId, err := strconv.Atoi(param)
	if err != nil {
		// 参数不是数字，尝试作为样本编号查询报告ID
		err = db.QueryRow(`SELECT r.id FROM detect_report r LEFT JOIN detect_sample s ON r.sample_id = s.id WHERE s.sample_code = ?`, param).Scan(&detect_reportId)
		if err != nil {
			c.JSON(consts.StatusBadRequest, ApiResponse{
				Code:    400,
				Success: false,
				Message: "无效的报告ID或样本编号",
				Data:    nil,
			})
			return
		}
	}

	// 解析请求体
	var req struct {
		Status string `json:"status" binding:"required"`
	}

	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请求参数错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 从上下文中获取用户ID（操作人ID）
	// 暂时未使用operatorId，保留获取逻辑以备后续使用
	_, _ = c.Get(UserIDKey)

	tx, err := db.Begin()
	if err != nil {
		log.Printf("Failed to begin transaction: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}
	var sampleID int
	if err := tx.QueryRow("SELECT COALESCE(sample_id, 0) FROM detect_report WHERE id = ?", detect_reportId).Scan(&sampleID); err != nil {
		tx.Rollback()
		log.Printf("Failed to query detect_report sample for status update: %v", err)
		c.JSON(consts.StatusNotFound, ApiResponse{
			Code:    404,
			Success: false,
			Message: "报告不存在",
			Data:    nil,
		})
		return
	}

	// 更新报告状态
	_, err = tx.Exec("UPDATE detect_report SET status = ?, updated_at = NOW() WHERE id = ?", req.Status, detect_reportId)
	if err != nil {
		tx.Rollback()
		log.Printf("Failed to update detect_report status: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}
	if err := syncSampleStatusesAfterReportChangeTx(tx, sampleID); err != nil {
		tx.Rollback()
		log.Printf("Failed to sync sample status after report status update: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "同步样本状态失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}
	if err := tx.Commit(); err != nil {
		log.Printf("Failed to commit report status update: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 返回成功响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "更新报告状态成功",
		Data:    nil,
	})
}

// 处理删除报告请求
func HandleDeleteReport(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	param := c.Param("id")

	// 尝试将参数转换为整数（旧格式：数据库ID）
	detect_reportId, err := strconv.Atoi(param)
	if err != nil {
		// 参数不是数字，尝试作为样本编号查询报告ID
		err = db.QueryRow(`SELECT r.id FROM detect_report r LEFT JOIN detect_sample s ON r.sample_id = s.id WHERE s.sample_code = ?`, param).Scan(&detect_reportId)
		if err != nil {
			c.JSON(consts.StatusBadRequest, ApiResponse{
				Code:    400,
				Success: false,
				Message: "无效的报告ID或样本编号",
				Data:    nil,
			})
			return
		}
	}

	// 开始事务
	tx, err := db.Begin()
	if err != nil {
		log.Printf("Failed to begin transaction: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 查询报告信息（用于删除文件）
	var filePath string
	var sampleID int
	err = tx.QueryRow("SELECT COALESCE(file_path, ''), COALESCE(sample_id, 0) FROM detect_report WHERE id = ?", detect_reportId).Scan(&filePath, &sampleID)
	if err != nil && err != sql.ErrNoRows {
		tx.Rollback()
		log.Printf("Failed to query detect_report for deletion: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}
	if err == sql.ErrNoRows {
		tx.Rollback()
		c.JSON(consts.StatusNotFound, ApiResponse{
			Code:    404,
			Success: false,
			Message: "报告不存在",
			Data:    nil,
		})
		return
	}

	// 删除报告
	_, err = tx.Exec("DELETE FROM detect_report WHERE id = ?", detect_reportId)
	if err != nil {
		tx.Rollback()
		log.Printf("Failed to delete detect_report: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}
	if err := syncSampleStatusesAfterReportChangeTx(tx, sampleID); err != nil {
		tx.Rollback()
		log.Printf("Failed to sync sample status after report deletion: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "同步样本状态失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		log.Printf("Failed to commit transaction: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 删除报告文件（如果存在）
	if filePath != "" {
		if err := os.Remove(filePath); err != nil {
			log.Printf("Failed to delete detect_report file: %v", err)
			// 继续执行，不中断删除操作
		}
	}

	// 返回成功响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "删除报告成功",
		Data:    nil,
	})
}

// 处理更新报告请求
func HandleUpdateReport(c *app.RequestContext, db *sql.DB) {
	// 历史报告数据结构
	type HistoricalReport struct {
		Time   string  `json:"time"`
		Signal float64 `json:"signal"`
		Type   string  `json:"type"`
		Trend  string  `json:"trend"`
		Note   string  `json:"note"`
	}

	// 解析路径参数
	param := c.Param("id")

	// 尝试将参数转换为整数（旧格式：数据库ID）
	detect_reportId, err := strconv.Atoi(param)
	if err != nil {
		// 参数不是数字，尝试作为样本编号查询报告ID
		err = db.QueryRow(`SELECT r.id FROM detect_report r LEFT JOIN detect_sample s ON r.sample_id = s.id WHERE s.sample_code = ?`, param).Scan(&detect_reportId)
		if err != nil {
			c.JSON(consts.StatusBadRequest, ApiResponse{
				Code:    400,
				Success: false,
				Message: "无效的报告ID或样本编号",
				Data:    nil,
			})
			return
		}
	}

	// 解析请求体
	var req struct {
		CalculationResult         *float64               `json:"CalculationResult"`
		CalculationResultLower    *float64               `json:"calculationResult"`
		SelectedModelId           int                    `json:"SelectedModelId"`
		SelectedModelIdLower      int                    `json:"selectedModelId"`
		GeneData                  map[string]interface{} `json:"geneData"`
		ResultExplanation         string                 `json:"ResultExplanation"`
		ResultExplanationLower    string                 `json:"resultExplanation"`
		SignalValueExplanation    string                 `json:"SignalValueExplanation"`
		SignalValueExplanationLow string                 `json:"signalValueExplanation"`
		Organization              string                 `json:"Organization"`
		OrganizationLower         string                 `json:"organization"`
		SelectedHistoricalReports []HistoricalReport     `json:"SelectedHistoricalReports"`
		SelectedHistoricalLow     []HistoricalReport     `json:"selectedHistoricalReports"`
		TreatmentStageName        string                 `json:"treatmentStageName"`
		SampleType                string                 `json:"sampleType"`
		ReportType                string                 `json:"reportType"`
		Remarks                   string                 `json:"remarks"`
		Trend                     string                 `json:"trend"`
		Time1                     string                 `json:"time1"`
		Signal1                   float64                `json:"signal1"`
		Trend1                    string                 `json:"trend1"`
		Type1                     string                 `json:"type1"`
		Note1                     string                 `json:"note1"`
		Time2                     string                 `json:"time2"`
		Signal2                   float64                `json:"signal2"`
		Trend2                    string                 `json:"trend2"`
		Type2                     string                 `json:"type2"`
		Note2                     string                 `json:"note2"`
		Time3                     string                 `json:"time3"`
		Signal3                   float64                `json:"signal3"`
		Trend3                    string                 `json:"trend3"`
		Type3                     string                 `json:"type3"`
		Note3                     string                 `json:"note3"`
		Time4                     string                 `json:"time4"`
		Signal4                   float64                `json:"signal4"`
		Trend4                    string                 `json:"trend4"`
		Type4                     string                 `json:"type4"`
		Note4                     string                 `json:"note4"`
	}

	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请求参数错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	calculationResult := 0.0
	if req.CalculationResult != nil {
		calculationResult = *req.CalculationResult
	} else if req.CalculationResultLower != nil {
		calculationResult = *req.CalculationResultLower
	}
	if req.SelectedModelId == 0 && req.SelectedModelIdLower != 0 {
		req.SelectedModelId = req.SelectedModelIdLower
	}
	if req.ResultExplanation == "" {
		req.ResultExplanation = req.ResultExplanationLower
	}
	if req.SignalValueExplanation == "" {
		req.SignalValueExplanation = req.SignalValueExplanationLow
	}
	if req.Organization == "" {
		req.Organization = req.OrganizationLower
	}
	if req.SelectedHistoricalReports == nil {
		req.SelectedHistoricalReports = req.SelectedHistoricalLow
	}
	if req.Signal1 == 0 && calculationResult != 0 {
		req.Signal1 = calculationResult
	}

	var oldReportDataJSON, oldPDFFilePath string
	if err := db.QueryRow("SELECT COALESCE(report_data, ''), COALESCE(file_path, '') FROM detect_report WHERE id = ?", detect_reportId).Scan(&oldReportDataJSON, &oldPDFFilePath); err != nil {
		log.Printf("Failed to query old detect_report data: %v", err)
		c.JSON(consts.StatusNotFound, ApiResponse{
			Code:    404,
			Success: false,
			Message: "报告不存在",
			Data:    nil,
		})
		return
	}
	oldReportData := make(map[string]interface{})
	if strings.TrimSpace(oldReportDataJSON) != "" {
		if err := json.Unmarshal([]byte(oldReportDataJSON), &oldReportData); err != nil {
			log.Printf("Failed to parse old detect_report data: %v", err)
		}
	}
	originalCalculationResult := reportFloatValue(oldReportData["originalCalculationResult"])
	if originalCalculationResult == 0 {
		originalCalculationResult = reportFloatValue(oldReportData["calculationResult"])
	}
	calculationModified := math.Abs(calculationResult-originalCalculationResult) >= 0.05

	// 转换GeneData为map[string]float64，处理字符串类型的值并过滤掉非基因字段
	geneDataFloat := make(map[string]float64)
	for gene, value := range req.GeneData {
		// 跳过非基因字段，如sqrt
		if gene == "sqrt" {
			continue
		}

		// 处理不同类型的值
		switch v := value.(type) {
		case float64:
			geneDataFloat[gene] = v
		case int:
			geneDataFloat[gene] = float64(v)
		case string:
			if floatVal, err := strconv.ParseFloat(v, 64); err == nil {
				geneDataFloat[gene] = floatVal
			}
		}
	}

	// 确保SelectedHistoricalReports不为空
	selectedHistoricalReports := req.SelectedHistoricalReports
	if selectedHistoricalReports == nil {
		selectedHistoricalReports = []HistoricalReport{}
	}

	// 构建报告数据JSON
	detect_reportData := map[string]interface{}{
		"calculationResult":         calculationResult,
		"originalCalculationResult": originalCalculationResult,
		"calculationModified":       calculationModified,
		"selectedModelId":           req.SelectedModelId,
		"geneData":                  geneDataFloat,
		"resultExplanation":         req.ResultExplanation,
		"signalValueExplanation":    req.SignalValueExplanation,
		"organization":              req.Organization,
		"selectedHistoricalReports": selectedHistoricalReports,
		"treatmentStageName":        req.TreatmentStageName,
		"sampleType":                req.SampleType,
		"remarks":                   req.Remarks,
		"trend":                     req.Trend,
		"time1":                     req.Time1,
		"signal1":                   req.Signal1,
		"trend1":                    req.Trend1,
		"type1":                     req.Type1,
		"note1":                     req.Note1,
		"time2":                     req.Time2,
		"signal2":                   req.Signal2,
		"trend2":                    req.Trend2,
		"type2":                     req.Type2,
		"note2":                     req.Note2,
		"time3":                     req.Time3,
		"signal3":                   req.Signal3,
		"trend3":                    req.Trend3,
		"type3":                     req.Type3,
		"note3":                     req.Note3,
		"time4":                     req.Time4,
		"signal4":                   req.Signal4,
		"trend4":                    req.Trend4,
		"type4":                     req.Type4,
		"note4":                     req.Note4,
	}
	historyRows := make([]ReportHistoryRow, 0, len(selectedHistoricalReports))
	for _, history := range selectedHistoricalReports {
		historyRows = append(historyRows, ReportHistoryRow{
			Time:   history.Time,
			Signal: history.Signal,
			Type:   history.Type,
			Trend:  history.Trend,
			Note:   history.Note,
		})
	}
	syncReportHistoryFields(detect_reportData, ReportHistoryRow{
		Time:   req.Time1,
		Signal: req.Signal1,
		Trend:  req.Trend1,
		Type:   req.Type1,
		Note:   req.Note1,
	}, historyRows)

	detect_reportDataJSON, err := json.Marshal(detect_reportData)
	if err != nil {
		log.Printf("Failed to marshal detect_report data: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	assignedReportType := ""
	if req.ReportType != "" {
		assignedReportType = normalizeAssignedReportType(req.ReportType)
	}

	tx, err := db.Begin()
	if err != nil {
		log.Printf("Failed to begin report update transaction: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}
	defer tx.Rollback()

	// 更新报告
	_, err = tx.Exec(`UPDATE detect_report SET report_data = ?,
		report_type = CASE WHEN ? = '' THEN report_type ELSE ? END,
		file_path = '', pdf_generation_status = 'pending', updated_at = NOW() WHERE id = ?`,
		detect_reportDataJSON, assignedReportType, assignedReportType, detect_reportId)
	if err != nil {
		log.Printf("Failed to update detect_report: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	changedBy := 0
	if userID, exists := c.Get("userID"); exists {
		if id, ok := userID.(int); ok {
			changedBy = id
		}
	}
	for _, fieldName := range []string{
		"calculationResult", "selectedModelId", "resultExplanation", "signalValueExplanation", "organization",
		"treatmentStageName", "sampleType", "remarks", "trend",
		"time1", "signal1", "trend1", "type1", "note1",
		"time2", "signal2", "trend2", "type2", "note2",
		"time3", "signal3", "trend3", "type3", "note3",
		"time4", "signal4", "trend4", "type4", "note4",
	} {
		oldValue := reportJSONValueToString(oldReportData[fieldName])
		newValue := reportJSONValueToString(detect_reportData[fieldName])
		if oldValue == newValue {
			continue
		}
		var changedByValue interface{}
		if changedBy > 0 {
			changedByValue = changedBy
		}
		_, err = tx.Exec(`INSERT INTO detect_report_change_log (report_id, field_name, old_value, new_value, changed_by, changed_at)
			VALUES (?, ?, ?, ?, ?, NOW())`, detect_reportId, fieldName, oldValue, newValue, changedByValue)
		if err != nil {
			log.Printf("Failed to insert detect_report_change_log: %v", err)
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code:    500,
				Success: false,
				Message: "保存报告修改记录失败",
				Data:    utils.H{"error": err.Error()},
			})
			return
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Failed to commit report update transaction: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}
	removeStaleGeneratedReportPDF(oldPDFFilePath)

	// 返回成功响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "更新报告成功",
		Data:    nil,
	})
}

// Reviewed reports used to keep a generated full PDF in file_path. Once any
// report field changes that file is stale and must no longer be served.
func removeStaleGeneratedReportPDF(filePath string) {
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return
	}
	cleanPath, err := filepath.Abs(filepath.Clean(filePath))
	if err != nil {
		return
	}
	tempReportDir, err := filepath.Abs(filepath.Join("file", "temp", "detect_report"))
	if err != nil {
		return
	}
	prefix := tempReportDir + string(os.PathSeparator)
	if cleanPath != tempReportDir && !strings.HasPrefix(cleanPath, prefix) {
		log.Printf("Skip deleting stale report PDF outside temporary directory: %s", cleanPath)
		return
	}
	if err := os.Remove(cleanPath); err != nil && !os.IsNotExist(err) {
		log.Printf("Failed to delete stale report PDF %s: %v", cleanPath, err)
	}
}

func reportJSONValueToString(value interface{}) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprint(v)
		}
		return string(data)
	}
}

func resolveTemplateContent(db *sql.DB, templateType string, modelID int, signal float64) string {
	return resolveTemplateContentWithContext(db, templateType, modelID, signal, "", "", "")
}

func reportListValueMatches(actual string, expected string) bool {
	actual = strings.TrimSpace(actual)
	expected = normalizeReportTreatmentStageName(expected)
	if actual == "" || expected == "" {
		return true
	}
	for _, item := range strings.Split(actual, ",") {
		if strings.EqualFold(normalizeReportTreatmentStageName(item), expected) {
			return true
		}
	}
	return false
}

func normalizeReportTreatmentStageName(stageName string) string {
	stageName = strings.TrimSpace(stageName)
	switch stageName {
	case "健康筛查":
		return "健康体检"
	case "术后", "手术后":
		return "术后检测"
	case "残留检测（术前中后）":
		return "残留检测"
	default:
		return stageName
	}
}

func resolveTemplateContentWithContext(db *sql.DB, templateType string, modelID int, signal float64, project string, detectionType string, valueType string) string {
	rows, err := db.Query(`
		SELECT content, model_id, min_signal_value, max_signal_value,
			COALESCE(project, ''), COALESCE(detection_type, ''), COALESCE(value_type, '')
		FROM setting_report_template
		WHERE type = ?
		ORDER BY
			CASE WHEN model_id IS NOT NULL THEN 0 ELSE 1 END,
			CASE WHEN project IS NOT NULL AND project != '' THEN 0 ELSE 1 END,
			CASE WHEN detection_type IS NOT NULL AND detection_type != '' THEN 0 ELSE 1 END,
			CASE WHEN value_type IS NOT NULL AND value_type != '' THEN 0 ELSE 1 END,
			CASE WHEN min_signal_value IS NOT NULL OR max_signal_value IS NOT NULL THEN 0 ELSE 1 END,
			created_at DESC
	`, templateType)
	if err != nil {
		return ""
	}
	defer rows.Close()

	fallbackContent := ""
	thresholdContent := ""

	for rows.Next() {
		var content string
		var rowModelID sql.NullInt64
		var minSignalValue, maxSignalValue sql.NullFloat64
		var rowProject, rowDetectionType, rowValueType string

		if err := rows.Scan(&content, &rowModelID, &minSignalValue, &maxSignalValue, &rowProject, &rowDetectionType, &rowValueType); err != nil {
			continue
		}

		if !reportListValueMatches(rowProject, project) || !reportListValueMatches(rowDetectionType, detectionType) || !reportListValueMatches(rowValueType, valueType) {
			continue
		}

		if rowModelID.Valid && modelID > 0 && int(rowModelID.Int64) == modelID {
			return content
		}

		if (minSignalValue.Valid || maxSignalValue.Valid) && thresholdContent == "" {
			if (!minSignalValue.Valid || signal >= minSignalValue.Float64) &&
				(!maxSignalValue.Valid || signal < maxSignalValue.Float64) {
				thresholdContent = content
			}
		}

		if !rowModelID.Valid && !minSignalValue.Valid && !maxSignalValue.Valid && fallbackContent == "" {
			fallbackContent = content
		}
	}

	if thresholdContent != "" {
		return thresholdContent
	}

	return fallbackContent
}

func reportStageProject(stageName string) string {
	stageName = normalizeReportTreatmentStageName(stageName)
	switch {
	case strings.Contains(stageName, "术后") || strings.Contains(stageName, "手术后"):
		return "postoperative"
	case strings.Contains(stageName, "残留"):
		return "residual"
	case strings.Contains(stageName, "复发"):
		return "recurrence"
	case strings.Contains(stageName, "化疗"):
		return "chemo"
	case strings.Contains(stageName, "术前") || strings.Contains(stageName, "健康") || strings.Contains(stageName, "辅助"):
		return "auxiliary"
	default:
		return "auxiliary"
	}
}

func reportSignalText(score float64) string {
	return fmt.Sprintf("%.1f", math.Round(score*10)/10)
}

func reportHistoryStringValue(item map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			return strings.TrimSpace(fmt.Sprint(value))
		}
	}
	return ""
}

func reportHistoryFloatValue(item map[string]interface{}, keys ...string) (float64, bool) {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			switch v := value.(type) {
			case float64:
				return v, true
			case float32:
				return float64(v), true
			case int:
				return float64(v), true
			case int64:
				return float64(v), true
			case json.Number:
				parsed, err := v.Float64()
				return parsed, err == nil
			case string:
				parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
				return parsed, err == nil
			}
		}
	}
	return 0, false
}

func referenceHistorySignalForStage(stageName string, histories []map[string]interface{}) (float64, bool) {
	stageName = normalizeReportTreatmentStageName(stageName)
	for _, history := range histories {
		historyStage := normalizeReportTreatmentStageName(reportHistoryStringValue(history, "type", "treatmentStageName", "treatment_stage_name"))
		if ((strings.Contains(stageName, "术后") || strings.Contains(stageName, "手术后")) && (strings.Contains(historyStage, "术前") || strings.Contains(historyStage, "辅助"))) ||
			(strings.Contains(stageName, "化疗后") && strings.Contains(historyStage, "化疗前")) {
			return reportHistoryFloatValue(history, "signal", "signalValue", "signal_value")
		}
	}
	return 0, false
}

func postoperativeResultExplanation(diff float64) string {
	switch {
	case diff >= 10:
		return "术后ctDNA甲基化检测信号值明显下降，表明患者体内肿瘤负荷降低，进而反映出治疗效果显著。为有效管理患者病情，建议除临床常规监测外，定期复检ctDNA甲基化。"
	case diff >= 5:
		return "术后ctDNA甲基化检测信号值有所下降，表明患者体内肿瘤负荷降低，进而反映出有一定治疗效果。为有效管理患者病情，建议除临床常规监测外，定期复检ctDNA甲基化。"
	case diff >= 1:
		return "术后ctDNA甲基化检测信号值略有下降，表明患者体内肿瘤负荷降低，进而反映出有一定治疗效果。为有效管理患者病情，建议除临床常规监测外，定期复检ctDNA甲基化。"
	default:
		return ""
	}
}

func defaultResultExplanationByStage(stageName string, score float64, histories []map[string]interface{}) string {
	project := reportStageProject(stageName)
	if project == "postoperative" {
		if referenceSignal, ok := referenceHistorySignalForStage(stageName, histories); ok {
			if explanation := postoperativeResultExplanation(referenceSignal - score); explanation != "" {
				return explanation
			}
		}
		project = "residual"
	}

	scoreText := reportSignalText(score)
	target := "肿瘤发生"
	switch project {
	case "recurrence":
		target = "肿瘤复发"
	case "residual", "chemo":
		target = "肿瘤残留"
	}

	if score < 25 {
		return fmt.Sprintf("血液游离肿瘤DNA的分析信号值低于检测下限，检测信号值%s为【低风险】结果，这表明当前受检者%s的风险较低。但应注意，此结果不排除所有风险，建议除临床常规监测外，定期复检ctDNA甲基化观察信号进行性变化情况。", scoreText, target)
	}
	if score <= 30 {
		if project == "chemo" {
			return "MePlex检出阳性标记信号，肿瘤负荷略高。建议除临床常规监测外，定期复检ctDNA甲基化，观察信号进行性变化情况，密切监控病情变化，以便实施早期干预。"
		}
		if project == "residual" || project == "recurrence" {
			return "MePlex检出少量阳性标记信号，检测信号值略高于正常水平。建议除临床常规监测外，定期复检ctDNA甲基化观察信号进行性变化情况。"
		}
		return fmt.Sprintf("MePlex检出少量阳性标记信号，检测信号值%s为【中风险】结果，这可能暗示存在%s的风险。建议除临床常规监测外，定期复检ctDNA甲基化观察信号进行性变化情况。", scoreText, target)
	}
	if score <= 45 {
		if project == "chemo" {
			return "MePlex检出阳性标记信号，肿瘤负荷较高，建议进一步进行相关治疗。除临床常规监测外，定期复检ctDNA甲基化，观察信号进行性变化情况，密切监控病情变化，以便实施早期干预。"
		}
		return fmt.Sprintf("MePlex检出阳性标记信号，检测信号值%s为【中风险】结果，这可能暗示存在%s的风险。建议除临床常规监测外，定期复检ctDNA甲基化观察信号进行性变化情况。", scoreText, target)
	}
	if score <= 50 {
		if project == "chemo" {
			return "MePlex检出多个阳性标记信号，肿瘤负荷较高，建议进一步进行相关治疗。除临床常规监测外，定期复检ctDNA甲基化，观察信号进行性变化情况，密切监控病情变化，以便实施早期干预。"
		}
		return fmt.Sprintf("MePlex检出多个阳性标记信号，检测信号值%s为【中风险】结果，这可能暗示存在%s的风险。建议除临床常规监测外，定期复检ctDNA甲基化观察信号进行性变化情况。", scoreText, target)
	}
	if project == "chemo" {
		return "MePlex检出多个阳性标记信号，肿瘤负荷较高，建议进一步进行相关治疗。除临床常规监测外，定期复检ctDNA甲基化，观察信号进行性变化情况，密切监控病情变化，以便实施早期干预。"
	}
	return fmt.Sprintf("MePlex检出多个阳性标记信号，检测信号值%s为【高风险】结果，这可能暗示%s的风险较高。建议除临床常规监测外，定期复检ctDNA甲基化观察信号进行性变化情况。", scoreText, target)
}

func nullableString(value string) interface{} {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func getReportTemplatesData(db *sql.DB, filters map[string]string) (utils.H, error) {
	// 构建查询语句
	query := `SELECT id, title, content, type, model_id, min_signal_value, max_signal_value,
		COALESCE(detection_type, ''), COALESCE(value_type, ''), COALESCE(report_category, ''),
		COALESCE(report_version, ''), COALESCE(project, ''), created_at
		FROM setting_report_template`
	var args []interface{}
	var conditions []string

	templateType := strings.TrimSpace(filters["type"])
	detectionType := strings.TrimSpace(filters["detectionType"])
	valueType := strings.TrimSpace(filters["valueType"])
	reportCategory := strings.TrimSpace(filters["reportCategory"])
	reportVersion := strings.TrimSpace(filters["reportVersion"])
	project := strings.TrimSpace(filters["project"])

	if templateType != "" {
		conditions = append(conditions, "type = ?")
		args = append(args, templateType)
	}
	if detectionType != "" {
		conditions = append(conditions, "detection_type = ?")
		args = append(args, detectionType)
	}
	if valueType != "" {
		conditions = append(conditions, "value_type = ?")
		args = append(args, valueType)
	}
	if reportCategory != "" {
		conditions = append(conditions, "report_category = ?")
		args = append(args, reportCategory)
	}
	if reportVersion != "" {
		conditions = append(conditions, "report_version = ?")
		args = append(args, reportVersion)
	}
	if project != "" {
		conditions = append(conditions, "project = ?")
		args = append(args, project)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY created_at DESC"

	// 执行查询
	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("Failed to query templates: %v", err)
		return utils.H{"list": []utils.H{}, "total": 0}, err
	}
	defer rows.Close()

	// 遍历查询结果
	var templates []utils.H
	for rows.Next() {
		var id int
		var title, content, templateTypeStr string
		var detectionType, valueType, reportCategory, reportVersion, project string
		var modelID sql.NullInt64
		var minSignalValue, maxSignalValue sql.NullFloat64
		var createdAt time.Time

		err := rows.Scan(&id, &title, &content, &templateTypeStr, &modelID, &minSignalValue, &maxSignalValue, &detectionType, &valueType, &reportCategory, &reportVersion, &project, &createdAt)
		if err != nil {
			log.Printf("Failed to scan template: %v", err)
			continue
		}

		// 构建模板信息
		template := utils.H{
			"id":        id,
			"title":     title,
			"content":   content,
			"type":      templateTypeStr,
			"createdAt": createdAt.Format("2006-01-02T15:04:05+08:00"),
		}
		template["detectionType"] = detectionType
		template["valueType"] = valueType
		template["reportCategory"] = reportCategory
		template["reportVersion"] = reportVersion
		template["project"] = project
		if modelID.Valid {
			template["modelId"] = modelID.Int64
		}
		if minSignalValue.Valid {
			template["minSignalValue"] = minSignalValue.Float64
		}
		if maxSignalValue.Valid {
			template["maxSignalValue"] = maxSignalValue.Float64
		}

		templates = append(templates, template)
	}

	return utils.H{"list": templates, "total": len(templates)}, rows.Err()
}

// 处理获取模板列表请求
func HandleGetTemplates(c *app.RequestContext, db *sql.DB) {
	data, err := getReportTemplatesData(db, map[string]string{
		"type":           c.Query("type"),
		"detectionType":  c.Query("detectionType"),
		"valueType":      c.Query("valueType"),
		"reportCategory": c.Query("reportCategory"),
		"reportVersion":  c.Query("reportVersion"),
		"project":        c.Query("project"),
	})
	if err != nil {
		data = utils.H{"list": []utils.H{}, "total": 0}
	}
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取模板列表成功",
		Data:    data,
	})
}

// 处理创建模板请求
func HandleCreateTemplate(c *app.RequestContext, db *sql.DB) {
	// 解析请求体
	var req struct {
		Title          string   `json:"title" binding:"required"`
		Content        string   `json:"content" binding:"required"`
		Type           string   `json:"type" binding:"required"`
		ModelId        *int     `json:"modelId"`
		MinSignalValue *float64 `json:"minSignalValue"`
		MaxSignalValue *float64 `json:"maxSignalValue"`
		DetectionType  string   `json:"detectionType"`
		ValueType      string   `json:"valueType"`
		ReportCategory string   `json:"reportCategory"`
		ReportVersion  string   `json:"reportVersion"`
		Project        string   `json:"project"`
	}

	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的请求参数",
			Data:    nil,
		})
		return
	}

	// 验证模板类型
	if req.Type != "result_explanation" && req.Type != "signal_explanation" {
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的模板类型",
			Data:    nil,
		})
		return
	}

	// 创建模板
	result, err := db.Exec(
		`INSERT INTO setting_report_template
			(title, content, type, model_id, min_signal_value, max_signal_value, detection_type, value_type, report_category, report_version, project)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.Title, req.Content, req.Type, req.ModelId, req.MinSignalValue, req.MaxSignalValue,
		nullableString(req.DetectionType), nullableString(req.ValueType), nullableString(req.ReportCategory), nullableString(req.ReportVersion), nullableString(req.Project),
	)
	if err != nil {
		log.Printf("Failed to create template: %v", err)
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    500,
			Success: false,
			Message: "创建模板失败",
			Data:    nil,
		})
		return
	}

	// 获取新创建的模板ID
	id, err := result.LastInsertId()
	if err != nil {
		log.Printf("Failed to get last insert id: %v", err)
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    500,
			Success: false,
			Message: "创建模板失败",
			Data:    nil,
		})
		return
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "创建模板成功",
		Data:    utils.H{"id": id},
	})
}

// 处理更新模板请求
func HandleUpdateTemplate(c *app.RequestContext, db *sql.DB) {
	// 获取模板ID
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的模板ID",
			Data:    nil,
		})
		return
	}

	// 解析请求体
	var req struct {
		Title          string   `json:"title" binding:"required"`
		Content        string   `json:"content" binding:"required"`
		Type           string   `json:"type" binding:"required"`
		ModelId        *int     `json:"modelId"`
		MinSignalValue *float64 `json:"minSignalValue"`
		MaxSignalValue *float64 `json:"maxSignalValue"`
		DetectionType  string   `json:"detectionType"`
		ValueType      string   `json:"valueType"`
		ReportCategory string   `json:"reportCategory"`
		ReportVersion  string   `json:"reportVersion"`
		Project        string   `json:"project"`
	}

	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的请求参数",
			Data:    nil,
		})
		return
	}

	// 验证模板类型
	if req.Type != "result_explanation" && req.Type != "signal_explanation" {
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的模板类型",
			Data:    nil,
		})
		return
	}

	// 更新模板
	_, err = db.Exec(
		`UPDATE setting_report_template SET title = ?, content = ?, type = ?, model_id = ?, min_signal_value = ?, max_signal_value = ?,
			detection_type = ?, value_type = ?, report_category = ?, report_version = ?, project = ?
			WHERE id = ?`,
		req.Title, req.Content, req.Type, req.ModelId, req.MinSignalValue, req.MaxSignalValue,
		nullableString(req.DetectionType), nullableString(req.ValueType), nullableString(req.ReportCategory), nullableString(req.ReportVersion), nullableString(req.Project), id,
	)
	if err != nil {
		log.Printf("Failed to update template: %v", err)
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    500,
			Success: false,
			Message: "更新模板失败",
			Data:    nil,
		})
		return
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "更新模板成功",
		Data:    nil,
	})
}

// 处理删除模板请求
func HandleDeleteTemplate(c *app.RequestContext, db *sql.DB) {
	// 获取模板ID
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的模板ID",
			Data:    nil,
		})
		return
	}

	// 删除模板
	_, err = db.Exec("DELETE FROM setting_report_template WHERE id = ?", id)
	if err != nil {
		log.Printf("Failed to delete template: %v", err)
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    500,
			Success: false,
			Message: "删除模板失败",
			Data:    nil,
		})
		return
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "删除模板成功",
		Data:    nil,
	})
}

// 处理批量生成报告请求
func HandleBatchGenerateReports(c *app.RequestContext, db *sql.DB) {
	type BatchReportRow struct {
		SampleId                  int                      `json:"sampleId"`
		SampleCode                string                   `json:"sampleCode"`
		PatientId                 int                      `json:"patientId"`
		SelectedModelId           int                      `json:"selectedModelId"`
		CalculationResult         float64                  `json:"calculationResult"`
		OriginalCalculationResult *float64                 `json:"originalCalculationResult"`
		GeneData                  map[string]interface{}   `json:"geneData"`
		ResultExplanation         string                   `json:"resultExplanation"`
		SignalValueExplanation    string                   `json:"signalValueExplanation"`
		Organization              string                   `json:"organization"`
		TreatmentStageName        string                   `json:"treatmentStageName"`
		SampleType                string                   `json:"sampleType"`
		ReportType                string                   `json:"reportType"`
		Remarks                   string                   `json:"remarks"`
		Trend                     string                   `json:"trend"`
		MergeHistorical           *bool                    `json:"mergeHistorical"`
		SelectedHistoricalReports []map[string]interface{} `json:"selectedHistoricalReports"`
		Time1                     string                   `json:"time1"`
		Signal1                   float64                  `json:"signal1"`
		Trend1                    string                   `json:"trend1"`
		Type1                     string                   `json:"type1"`
		Note1                     string                   `json:"note1"`
		Time2                     string                   `json:"time2"`
		Signal2                   float64                  `json:"signal2"`
		Trend2                    string                   `json:"trend2"`
		Type2                     string                   `json:"type2"`
		Note2                     string                   `json:"note2"`
		Time3                     string                   `json:"time3"`
		Signal3                   float64                  `json:"signal3"`
		Trend3                    string                   `json:"trend3"`
		Type3                     string                   `json:"type3"`
		Note3                     string                   `json:"note3"`
		Time4                     string                   `json:"time4"`
		Signal4                   float64                  `json:"signal4"`
		Trend4                    string                   `json:"trend4"`
		Type4                     string                   `json:"type4"`
		Note4                     string                   `json:"note4"`
	}

	// 解析请求体
	var req struct {
		BatchId    int              `json:"batchId" binding:"required"`
		ModelId    int              `json:"modelId"`
		ReportType string           `json:"reportType" binding:"required"`
		Rows       []BatchReportRow `json:"rows"`
	}

	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请求参数错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}
	if req.ModelId == 0 && len(req.Rows) == 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "缺少模型信息",
			Data:    nil,
		})
		return
	}
	var batchStatus string
	if err := db.QueryRow("SELECT COALESCE(status, '') FROM detect_batch WHERE id = ?", req.BatchId).Scan(&batchStatus); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "批次不存在",
			Data:    nil,
		})
		return
	}
	if strings.TrimSpace(strings.ToLower(batchStatus)) != "submitted" {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "只有已提交批次才能生成报告",
			Data:    nil,
		})
		return
	}

	// 从上下文中获取用户ID（生成者ID）
	userID, exists := c.Get(UserIDKey)
	var generatedBy int
	if exists {
		generatedBy = userID.(int)
	} else {
		generatedBy = 0 // 默认为0，表示系统生成
	}

	rowOverrides := make(map[int]BatchReportRow)
	rowOverridesBySampleCode := make(map[string]BatchReportRow)
	requestedSampleIDs := make(map[int]bool)
	requestedSampleCodes := make(map[string]bool)
	for _, row := range req.Rows {
		if row.SampleId > 0 {
			rowOverrides[row.SampleId] = row
			requestedSampleIDs[row.SampleId] = true
		}
		if strings.TrimSpace(row.SampleCode) != "" {
			sampleCode := strings.TrimSpace(row.SampleCode)
			rowOverridesBySampleCode[sampleCode] = row
			requestedSampleCodes[sampleCode] = true
		}
	}

	// 获取批次中的样本列表。批次提交后 detect_batch_sample 会被清空，结果以 detect_sample.batch_id 为准。
	rows, err := db.Query(`SELECT s.id, s.sample_code, s.patient_id, s.result_data, s.sample_type_id, s.treatment_stage_id, COALESCE(s.organization, ''),
			COALESCE(s.receive_date, s.collection_date, s.sample_created_at)
		FROM detect_sample s
		WHERE s.batch_id = ?
		AND s.result_data IS NOT NULL`, req.BatchId)
	if err != nil {
		log.Printf("Failed to query batch samples: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}
	defer rows.Close()

	// 初始化样本ID列表
	var sampleIds []int
	var sampleCodes []string
	var patientIds []int
	var resultDataList []string
	var sampleTypeIds []int
	var treatmentStageIds []int
	var organizations []string
	var sampleTimes []time.Time

	// 遍历样本
	for rows.Next() {
		var sampleId, patientId, sampleTypeId, treatmentStageId int
		var sampleCode, resultData, organization string
		var sampleTime time.Time
		if err := rows.Scan(&sampleId, &sampleCode, &patientId, &resultData, &sampleTypeId, &treatmentStageId, &organization, &sampleTime); err != nil {
			log.Printf("Failed to scan sample: %v", err)
			continue
		}
		if len(req.Rows) > 0 && !requestedSampleIDs[sampleId] && !requestedSampleCodes[sampleCode] {
			continue
		}
		sampleIds = append(sampleIds, sampleId)
		sampleCodes = append(sampleCodes, sampleCode)
		patientIds = append(patientIds, patientId)
		resultDataList = append(resultDataList, resultData)
		sampleTypeIds = append(sampleTypeIds, sampleTypeId)
		treatmentStageIds = append(treatmentStageIds, treatmentStageId)
		organizations = append(organizations, organization)
		sampleTimes = append(sampleTimes, sampleTime)
	}

	// 检查遍历过程中是否有错误
	if err = rows.Err(); err != nil {
		log.Printf("Error iterating samples: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 检查是否有样本
	if len(sampleIds) == 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "批次中没有可生成报告的样本",
			Data:    nil,
		})
		return
	}

	// 获取样本类型和治疗阶段名称
	sampleTypeMap := make(map[int]string)
	treatmentStageMap := make(map[int]string)

	// 获取样本类型
	sampleTypeRows, err := db.Query("SELECT id, name FROM setting_sample_type")
	if err == nil {
		defer sampleTypeRows.Close()
		for sampleTypeRows.Next() {
			var id int
			var name string
			if err := sampleTypeRows.Scan(&id, &name); err == nil {
				sampleTypeMap[id] = name
			}
		}
	}

	// 获取治疗阶段
	treatmentStageRows, err := db.Query("SELECT id, name FROM setting_treatment_stage")
	if err == nil {
		defer treatmentStageRows.Close()
		for treatmentStageRows.Next() {
			var id int
			var name string
			if err := treatmentStageRows.Scan(&id, &name); err == nil {
				treatmentStageMap[id] = name
			}
		}
	}

	// 异步处理批量报告生成
	go func() {
		order := make([]int, len(sampleIds))
		for i := range order {
			order[i] = i
		}
		stageNameForIndex := func(i int) string {
			rowOverride, hasOverride := rowOverrides[sampleIds[i]]
			if !hasOverride {
				rowOverride, hasOverride = rowOverridesBySampleCode[sampleCodes[i]]
			}
			if hasOverride && strings.TrimSpace(rowOverride.TreatmentStageName) != "" {
				return strings.TrimSpace(rowOverride.TreatmentStageName)
			}
			return strings.TrimSpace(treatmentStageMap[treatmentStageIds[i]])
		}
		sort.SliceStable(order, func(left, right int) bool {
			i := order[left]
			j := order[right]
			if patientIds[i] != patientIds[j] {
				return patientIds[i] < patientIds[j]
			}
			leftRank := treatmentStageRank(stageNameForIndex(i))
			rightRank := treatmentStageRank(stageNameForIndex(j))
			if leftRank != rightRank {
				return leftRank < rightRank
			}
			return sampleIds[i] < sampleIds[j]
		})
		previousRowsByPatient := make(map[int][]ReportHistoryRow)

		for _, i := range order {
			sampleId := sampleIds[i]
			// 解析结果数据
			var resultDataMap map[string]interface{}
			if err := json.Unmarshal([]byte(resultDataList[i]), &resultDataMap); err != nil {
				log.Printf("Failed to unmarshal result data for sample %d: %v", sampleId, err)
				continue
			}

			rowOverride, hasOverride := rowOverrides[sampleId]
			if !hasOverride {
				rowOverride, hasOverride = rowOverridesBySampleCode[sampleCodes[i]]
			}

			// 提取基因数据和计算结果
			var geneData map[string]interface{}
			if hasOverride && rowOverride.GeneData != nil {
				geneData = rowOverride.GeneData
			} else if geneDataVal, ok := resultDataMap["gene_data"].(map[string]interface{}); ok {
				geneData = geneDataVal
			}
			if geneData != nil {
				geneData = normalizeGeneDataKeysToSymbols(db, geneData)
			}

			var calculationResult float64
			if hasOverride {
				calculationResult = rowOverride.CalculationResult
			} else if scoreVal, ok := resultDataMap["score"].(float64); ok {
				calculationResult = scoreVal
			}

			selectedModelID := req.ModelId
			if hasOverride && rowOverride.SelectedModelId > 0 {
				selectedModelID = rowOverride.SelectedModelId
			}
			if calculationResult == 0 && selectedModelID > 0 && geneData != nil {
				if _, enrichErr := enrichExcelDuplicateGeneVariablesForSample(db, sampleId, selectedModelID, geneData); enrichErr != nil {
					log.Printf("Failed to load Excel platform variables for sample %d: %v", sampleId, enrichErr)
				}
				if score, ok, err := calculateModelFormulaScore(db, selectedModelID, geneData); err != nil {
					logModelScoreError(sampleId, selectedModelID, err)
				} else if ok {
					calculationResult = score
				}
			}
			calculationResult = math.Round(calculationResult*10) / 10
			originalCalculationResult := calculationResult
			if hasOverride && rowOverride.OriginalCalculationResult != nil {
				originalCalculationResult = math.Round((*rowOverride.OriginalCalculationResult)*10) / 10
			}
			calculationModified := math.Abs(calculationResult-originalCalculationResult) >= 0.05

			treatmentStageName := treatmentStageMap[treatmentStageIds[i]]
			if hasOverride && rowOverride.TreatmentStageName != "" {
				treatmentStageName = rowOverride.TreatmentStageName
			}

			resultExplanation := ""
			if hasOverride {
				resultExplanation = rowOverride.ResultExplanation
			}
			if resultExplanation == "" {
				resultExplanation = resolveTemplateContentWithContext(db, "result_explanation", selectedModelID, calculationResult, treatmentStageName, sampleTypeMap[sampleTypeIds[i]], "signal")
				resultExplanation = strings.ReplaceAll(resultExplanation, "***", reportSignalText(calculationResult))
			}
			if resultExplanation == "" {
				resultExplanation = defaultResultExplanationByStage(treatmentStageName, calculationResult, rowOverride.SelectedHistoricalReports)
			}

			signalValueExplanation := ""
			if hasOverride {
				signalValueExplanation = rowOverride.SignalValueExplanation
			}
			if signalValueExplanation == "" {
				signalValueExplanation = resolveTemplateContentWithContext(db, "signal_explanation", selectedModelID, calculationResult, treatmentStageName, sampleTypeMap[sampleTypeIds[i]], "signal")
				signalValueExplanation = strings.ReplaceAll(signalValueExplanation, "***", reportSignalText(calculationResult))
			}
			if signalValueExplanation == "" {
				signalValueExplanation = "(a) 信号值人群正常参考值范围为-25\n(b) 信号值越低反应身体状态越好\n(c) 连续检测信号值进行性增高，说明复发风险增高\n(d) 连续检测信号值进行性降低，说明治疗效果显著"
			}

			organization := organizations[i]
			if hasOverride && rowOverride.Organization != "" {
				organization = rowOverride.Organization
			}
			if organization == "" {
				organization = "哈尔滨医科大学附属第二医院"
			}

			sampleTypeName := sampleTypeMap[sampleTypeIds[i]]
			if hasOverride && rowOverride.SampleType != "" {
				sampleTypeName = rowOverride.SampleType
			}

			// 准备报告数据
			detect_reportData := map[string]interface{}{
				"calculationResult":         calculationResult,
				"originalCalculationResult": originalCalculationResult,
				"calculationModified":       calculationModified,
				"selectedModelId":           selectedModelID,
				"geneData":                  geneData,
				"resultExplanation":         resultExplanation,
				"signalValueExplanation":    signalValueExplanation,
				"organization":              organization,
				"treatmentStageName":        treatmentStageName,
				"sampleType":                sampleTypeName,
				"remarks":                   "",
				"trend":                     "-",
				"selectedHistoricalReports": []map[string]interface{}{},
				"time1":                     sampleTimes[i].Format("2006-01-02"),
				"signal1":                   calculationResult,
				"trend1":                    "-",
				"type1":                     treatmentStageName,
				"note1":                     "",
			}
			if hasOverride {
				detect_reportData["remarks"] = rowOverride.Remarks
				if rowOverride.Trend != "" {
					detect_reportData["trend"] = rowOverride.Trend
				}
				detect_reportData["selectedHistoricalReports"] = rowOverride.SelectedHistoricalReports
				if rowOverride.Time1 != "" {
					detect_reportData["time1"] = rowOverride.Time1
				}
				detect_reportData["signal1"] = calculationResult
				if rowOverride.Trend1 != "" {
					detect_reportData["trend1"] = rowOverride.Trend1
				}
				if rowOverride.Type1 != "" {
					detect_reportData["type1"] = rowOverride.Type1
				}
				detect_reportData["note1"] = rowOverride.Note1
				detect_reportData["time2"] = rowOverride.Time2
				detect_reportData["signal2"] = rowOverride.Signal2
				detect_reportData["trend2"] = rowOverride.Trend2
				detect_reportData["type2"] = rowOverride.Type2
				detect_reportData["note2"] = rowOverride.Note2
				detect_reportData["time3"] = rowOverride.Time3
				detect_reportData["signal3"] = rowOverride.Signal3
				detect_reportData["trend3"] = rowOverride.Trend3
				detect_reportData["type3"] = rowOverride.Type3
				detect_reportData["note3"] = rowOverride.Note3
				detect_reportData["time4"] = rowOverride.Time4
				detect_reportData["signal4"] = rowOverride.Signal4
				detect_reportData["trend4"] = rowOverride.Trend4
				detect_reportData["type4"] = rowOverride.Type4
				detect_reportData["note4"] = rowOverride.Note4
			}

			historyRows := reportHistoryRowsFromValue(detect_reportData["selectedHistoricalReports"])
			if len(historyRows) == 0 && (rowOverride.MergeHistorical == nil || *rowOverride.MergeHistorical) {
				historyRows = append(historyRows, previousRowsByPatient[patientIds[i]]...)
			}
			currentReportRow := ReportHistoryRow{
				Time:   reportDateString(detect_reportData["time1"]),
				Signal: calculationResult,
				Trend:  normalizeReportTrend(reportStringValue(detect_reportData["trend"])),
				Type:   treatmentStageName,
				Note:   reportStringValue(detect_reportData["note1"]),
			}
			if currentReportRow.Type == "" {
				currentReportRow.Type = reportStringValue(detect_reportData["type1"])
			}
			syncReportHistoryFields(detect_reportData, currentReportRow, historyRows)

			detect_reportDataJSON, err := json.Marshal(detect_reportData)
			if err != nil {
				log.Printf("Failed to marshal detect_report data for sample %d: %v", sampleId, err)
				continue
			}

			// 生成报告编号
			detect_reportNo := fmt.Sprintf("REPORT_%s_%d", time.Now().Format("20060102"), time.Now().UnixNano()%10000)

			_, _ = db.Exec("DELETE FROM detect_report WHERE sample_id = ? AND status IN ('draft', 'rejected', 'pending', 'generated')", sampleId)

			assignedReportType := req.ReportType
			if hasOverride && rowOverride.ReportType != "" {
				assignedReportType = rowOverride.ReportType
			}
			assignedReportType = normalizeAssignedReportType(assignedReportType)

			// 插入报告到数据库（批量生成后进入待审核）
			result, err := db.Exec(`INSERT INTO detect_report (sample_id, report_no, patient_id, report_type, report_data, status, generated_by, generated_time, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, 'pending', ?, NOW(), NOW(), NOW())`,
				sampleId, detect_reportNo, patientIds[i], assignedReportType, detect_reportDataJSON, generatedBy)
			if err != nil {
				log.Printf("Failed to generate detect_report for sample %d: %v", sampleId, err)
				continue
			}
			if parentReportID, err := result.LastInsertId(); err == nil {
				createChildReportsForSelectedHistories(db, parentReportID, detect_reportNo, patientIds[i], sampleId, assignedReportType, detect_reportDataJSON, "pending", generatedBy, rowOverride.SelectedHistoricalReports)
			}
			if err := syncSampleStatusesAfterReportChange(db, sampleId); err != nil {
				log.Printf("Failed to sync sample status after batch report generation sample %d: %v", sampleId, err)
			}

			previousRowsByPatient[patientIds[i]] = append(previousRowsByPatient[patientIds[i]], ReportHistoryRow{
				Time:   reportDateString(detect_reportData["time1"]),
				Signal: reportFloatValue(detect_reportData["signal1"]),
				Trend:  normalizeReportTrend(reportStringValue(detect_reportData["trend1"])),
				Type:   reportStringValue(detect_reportData["type1"]),
				Note:   reportStringValue(detect_reportData["note1"]),
			})

			// 模拟生成过程
			time.Sleep(500 * time.Millisecond)
		}

		if _, err := db.Exec("UPDATE detect_batch SET status = 'completed', updated_at = NOW() WHERE id = ?", req.BatchId); err != nil {
			log.Printf("Failed to update batch %d status after report generation: %v", req.BatchId, err)
		}
	}()

	// 返回成功响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "批量报告生成任务已启动",
		Data:    utils.H{"sampleCount": len(sampleIds)},
	})
}

// 处理获取患者历史报告请求
func HandleGetPatientHistoricalReports(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	detect_patientIdStr := c.Param("patientId")
	if strings.TrimSpace(detect_patientIdStr) == "" {
		detect_patientIdStr = c.Param("detect_patientId")
	}
	detect_patientId, err := strconv.Atoi(detect_patientIdStr)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的患者ID",
			Data:    nil,
		})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	excludeSampleID, _ := strconv.Atoi(c.DefaultQuery("exclude_sample_id", "0"))

	// 查询患者历史报告。批量生成时，前两份报告通常还处于 pending，也需要作为第三份报告的趋势参考。
	// 子报告只作为电脑端入口使用，历史趋势要使用样本自身结果，避免显示成父报告信号值。
	query := `SELECT r.id, r.report_type, r.report_data, r.status, r.created_at, r.generated_time,
			COALESCE(s.id, 0) as sampleId,
			COALESCE(s.sample_code, '') as sampleCode,
			COALESCE(s.receive_date, s.collection_date, s.sample_created_at, r.generated_time, r.created_at) as sampleTime
		FROM detect_report r
		LEFT JOIN detect_sample s ON r.sample_id = s.id
		WHERE r.patient_id = ? AND r.status IN ('pending', 'generated', 'reviewed', 'published')
			AND COALESCE(r.report_role, 'single') <> 'child'
			AND (? = 0 OR COALESCE(r.sample_id, 0) <> ?)
		ORDER BY COALESCE(s.receive_date, s.collection_date, s.sample_created_at, r.generated_time, r.created_at) DESC, r.id DESC
		LIMIT ?`
	rows, err := db.Query(query, detect_patientId, excludeSampleID, excludeSampleID, limit)
	if err != nil {
		log.Printf("Failed to query detect_patient historical detect_reports: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "获取历史报告失败",
			Data:    nil,
		})
		return
	}
	defer rows.Close()

	var historicalReports []utils.H
	for rows.Next() {
		var detect_reportId, sampleID int
		var detect_reportType, detect_reportData, status, sampleCode string
		var createdAt, generatedTime, sampleTime sql.NullTime

		err := rows.Scan(&detect_reportId, &detect_reportType, &detect_reportData, &status, &createdAt, &generatedTime, &sampleID, &sampleCode, &sampleTime)
		if err != nil {
			log.Printf("Failed to scan historical detect_report: %v", err)
			continue
		}

		// 解析报告数据
		var detect_reportDataMap map[string]interface{}
		if detect_reportData != "" {
			if err := json.Unmarshal([]byte(detect_reportData), &detect_reportDataMap); err != nil {
				log.Printf("Failed to unmarshal detect_report data: %v", err)
				detect_reportDataMap = make(map[string]interface{})
			}
		} else {
			detect_reportDataMap = make(map[string]interface{})
		}

		// 构建历史报告信息
		histReport := utils.H{
			"id":                 detect_reportId,
			"sampleId":           sampleID,
			"sample_id":          sampleID,
			"status":             status,
			"signalValue":        detect_reportDataMap["calculationResult"],
			"treatmentStageName": detect_reportDataMap["treatmentStageName"],
			"remarks":            detect_reportDataMap["remarks"],
			"sampleCode":         sampleCode,
		}
		if createdAt.Valid {
			histReport["createdAt"] = createdAt.Time.Format("2006-01-02")
		}
		if generatedTime.Valid {
			histReport["generatedTime"] = generatedTime.Time.Format("2006-01-02")
		}
		if sampleTime.Valid {
			formattedSampleTime := sampleTime.Time.Format("2006-01-02T15:04:05+08:00")
			histReport["receiveDate"] = formattedSampleTime
			histReport["receive_date"] = formattedSampleTime
			histReport["sampleReceivedAt"] = formattedSampleTime
		}

		historicalReports = append(historicalReports, histReport)
	}

	sampleRows, err := db.Query(`SELECT s.id, s.sample_code, COALESCE(ts.name, ''), COALESCE(s.notes, ''),
			COALESCE(s.receive_date, s.collection_date, s.sample_created_at), COALESCE(s.signalvalue, 0), COALESCE(s.result_data, ''), COALESCE(s.model_id, 0)
		FROM detect_sample s
		LEFT JOIN setting_treatment_stage ts ON s.treatment_stage_id = ts.id
		WHERE s.patient_id = ?
			AND (? = 0 OR s.id <> ?)
			AND s.result_data IS NOT NULL
			AND TRIM(s.result_data) NOT IN ('', '{}', 'null')
			AND NOT EXISTS (
				SELECT 1 FROM detect_report r
				WHERE r.sample_id = s.id
					AND r.status IN ('pending', 'generated', 'reviewed', 'published')
					AND COALESCE(r.report_role, 'single') <> 'child'
			)`, detect_patientId, excludeSampleID, excludeSampleID)
	if err != nil {
		log.Printf("Failed to query historical samples: %v", err)
	} else {
		defer sampleRows.Close()
		for sampleRows.Next() {
			var sampleID int
			var sampleCode, treatmentStageName, remarks, resultData string
			var sampleTime time.Time
			var signalValue float64
			var modelID int
			if err := sampleRows.Scan(&sampleID, &sampleCode, &treatmentStageName, &remarks, &sampleTime, &signalValue, &resultData, &modelID); err != nil {
				log.Printf("Failed to scan historical sample: %v", err)
				continue
			}
			hasSignal := signalValue > 0
			if strings.TrimSpace(resultData) != "" {
				resultDataMap := map[string]interface{}{}
				if err := json.Unmarshal([]byte(resultData), &resultDataMap); err == nil {
					if value := reportFloatValue(resultDataMap["signalValue"]); value != 0 {
						signalValue = value
						hasSignal = true
					} else if value := reportFloatValue(resultDataMap["calculationResult"]); value != 0 {
						signalValue = value
						hasSignal = true
					} else if value := reportFloatValue(resultDataMap["score"]); value != 0 {
						signalValue = value
						hasSignal = true
					} else if modelID > 0 {
						if geneData, ok := resultDataMap["gene_data"].(map[string]interface{}); ok && len(geneData) > 0 {
							geneData = normalizeGeneDataKeysToSymbols(db, geneData)
							if _, enrichErr := enrichExcelDuplicateGeneVariablesForSample(db, sampleID, modelID, geneData); enrichErr != nil {
								log.Printf("Failed to load historical Excel platform variables for sample %d: %v", sampleID, enrichErr)
							}
							if score, calculated, err := calculateModelFormulaScore(db, modelID, geneData); err != nil {
								logModelScoreError(sampleID, modelID, err)
							} else if calculated {
								signalValue = math.Round(score*10) / 10
								hasSignal = true
							}
						}
					}
				}
			}
			if !hasSignal {
				continue
			}
			formattedSampleTime := sampleTime.Format("2006-01-02T15:04:05+08:00")
			historicalReports = append(historicalReports, utils.H{
				"id":                 -sampleID,
				"sampleId":           sampleID,
				"sample_id":          sampleID,
				"status":             "sample_result",
				"signalValue":        signalValue,
				"createdAt":          sampleTime.Format("2006-01-02"),
				"generatedTime":      sampleTime.Format("2006-01-02"),
				"receiveDate":        formattedSampleTime,
				"receive_date":       formattedSampleTime,
				"sampleReceivedAt":   formattedSampleTime,
				"treatmentStageName": treatmentStageName,
				"remarks":            remarks,
				"sampleCode":         sampleCode,
			})
		}
	}

	sort.SliceStable(historicalReports, func(i, j int) bool {
		timeValue := func(item utils.H) time.Time {
			for _, key := range []string{"receiveDate", "receive_date", "sampleReceivedAt", "generatedTime", "createdAt"} {
				if raw, ok := item[key]; ok {
					text := strings.TrimSpace(fmt.Sprint(raw))
					if text == "" {
						continue
					}
					if parsed, err := time.Parse(time.RFC3339, text); err == nil {
						return parsed
					}
					if parsed, err := time.Parse("2006-01-02T15:04:05-07:00", text); err == nil {
						return parsed
					}
					if parsed, err := time.Parse("2006-01-02", text); err == nil {
						return parsed
					}
				}
			}
			return time.Time{}
		}
		return timeValue(historicalReports[i]).After(timeValue(historicalReports[j]))
	})
	if len(historicalReports) > limit {
		historicalReports = historicalReports[:limit]
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取历史报告成功",
		Data:    historicalReports,
	})
}

// 处理报告PDF预览请求
func HandlePreviewReportPdf(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	idStr := c.Param("id")
	id, err := resolveReportIDParam(db, idStr)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的报告ID或样本编号",
			Data:    nil,
		})
		return
	}

	// 检查报告是否存在
	var reportExists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM detect_report WHERE id = ?)", id).Scan(&reportExists)
	if err != nil || !reportExists {
		log.Printf("Failed to query detect_report: %v", err)
		c.JSON(consts.StatusNotFound, ApiResponse{
			Code:    404,
			Success: false,
			Message: "报告不存在",
			Data:    nil,
		})
		return
	}

	mode := getReportPDFMode(c)
	effectiveReportID := id
	if parentReportID := getParentReportID(db, id); parentReportID > 0 {
		effectiveReportID = parentReportID
	}
	filePath, err := generateReportPDFByMode(db, effectiveReportID, mode)
	if err != nil {
		log.Printf("生成PDF报告失败: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "生成PDF报告失败",
			Data:    nil,
		})
		return
	}

	// 生成30分钟过期的一次性预览URL
	tempURL, err := fileURLManager.GenerateOneTimeFileURL(filePath, 30*time.Minute)
	if err != nil {
		log.Printf("生成临时文件URL失败: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "文件预览链接生成失败",
			Data:    nil,
		})
		return
	}

	// 返回临时URL
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取PDF预览链接成功",
		Data:    utils.H{"previewUrl": tempURL},
	})
}

// 处理重新生成报告PDF请求
func HandleRegenerateReportPdf(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	idStr := c.Param("id")
	id, err := resolveReportIDParam(db, idStr)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的报告ID或样本编号",
			Data:    nil,
		})
		return
	}

	var oldPDFFilePath string
	_ = db.QueryRow("SELECT COALESCE(file_path, '') FROM detect_report WHERE id = ?", id).Scan(&oldPDFFilePath)
	_, err = db.Exec("UPDATE detect_report SET file_path = '', pdf_generation_status = 'pending', updated_at = NOW() WHERE id = ?", id)
	if err != nil {
		log.Printf("Failed to reset PDF generation status: %v", err)
	} else {
		removeStaleGeneratedReportPDF(oldPDFFilePath)
	}

	// 返回成功响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "PDF重新生成请求已提交",
		Data:    nil,
	})
}

// 处理获取报告预览数据请求
func HandleGetReportPreviewData(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	idStr := c.Param("id")
	id, err := resolveReportIDParam(db, idStr)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的报告ID或样本编号",
			Data:    nil,
		})
		return
	}

	// 提取报告预览数据
	effectiveReportID := id
	if parentReportID := getParentReportID(db, id); parentReportID > 0 {
		effectiveReportID = parentReportID
	}
	previewData, err := ExtractReportPreviewData(db, effectiveReportID)
	if err != nil {
		log.Printf("Failed to extract report preview data: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "获取报告预览数据失败",
			Data:    nil,
		})
		return
	}

	// 返回预览数据
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取报告预览数据成功",
		Data:    previewData,
	})
}

// 处理生成报告一次性下载链接请求
func HandleGenerateReportDownloadURL(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	idStr := c.Param("id")
	id, err := resolveReportIDParam(db, idStr)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的报告ID或样本编号",
			Data:    nil,
		})
		return
	}

	// 首先检查报告是否存在
	var reportExists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM detect_report WHERE id = ?)", id).Scan(&reportExists)
	if err != nil || !reportExists {
		c.JSON(consts.StatusNotFound, ApiResponse{
			Code:    404,
			Success: false,
			Message: "报告不存在",
			Data:    nil,
		})
		return
	}

	filePath, err := generatePDFReport(db, id)
	if err != nil {
		log.Printf("生成PDF报告失败: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "生成PDF报告失败",
			Data:    nil,
		})
		return
	}

	// 生成一次性使用的下载链接（5分钟过期）
	downloadURL, err := fileURLManager.GenerateOneTimeFileURL(filePath, 5*time.Minute)
	if err != nil {
		log.Printf("生成下载链接失败: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "生成下载链接失败",
			Data:    nil,
		})
		return
	}

	// 返回下载链接
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "生成下载链接成功",
		Data:    utils.H{"downloadUrl": downloadURL},
	})
}
