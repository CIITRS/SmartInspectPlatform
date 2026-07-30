package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

const informedConsentText = `本人已充分了解本次肿瘤相关分子检测的目的、流程、样本要求、可能的局限性及隐私保护说明。本检测结果仅供临床参考，不能单独作为疾病诊断或治疗依据，需由医生结合病史、影像、病理及其他检查综合判断。本人同意提供样本及必要信息用于本次检测，并确认以上信息由本人或合法授权代理人自愿签署。`

func nullablePositiveInt(value int) interface{} {
	if value > 0 {
		return value
	}
	return nil
}

// ============================================================
// 小程序专用 Handler（uni.go）
// 所有接口通过 miniappAuth 中间件验证 base_miniapp_sessions
// ============================================================

func getMiniappEmployeeID(c *app.RequestContext, db *sql.DB) int {
	if userID, exists := c.Get("miniapp_user_id"); exists {
		if id, ok := userID.(int); ok && id > 0 {
			return id
		}
	}

	phone, _ := c.Get("miniapp_phone")
	phoneStr, _ := phone.(string)
	if phoneStr == "" {
		return 0
	}

	var userID int
	if err := db.QueryRow("SELECT id FROM base_manage_user WHERE phone = ? LIMIT 1", phoneStr).Scan(&userID); err != nil {
		return 0
	}
	return userID
}

func getMiniappEmployeeCode(db *sql.DB, employeeID int) string {
	if employeeID <= 0 {
		return ""
	}
	var code string
	err := db.QueryRow(`SELECT COALESCE(employee_id, '')
		FROM base_manage_user WHERE id = ? LIMIT 1`, employeeID).Scan(&code)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(code)
}

func requireMiniappEmployee(c *app.RequestContext, db *sql.DB) (int, bool) {
	employeeID := getMiniappEmployeeID(c, db)
	if employeeID <= 0 {
		c.JSON(consts.StatusUnauthorized, ApiResponse{
			Code:    401,
			Success: false,
			Message: "请使用员工身份登录",
			Data:    nil,
		})
		return 0, false
	}
	return employeeID, true
}

func miniappEmployeeCanManageAllPatients(db *sql.DB, employeeID int) bool {
	if employeeID <= 0 {
		return false
	}
	var username, employeeCode string
	_ = db.QueryRow(`SELECT COALESCE(username, ''), COALESCE(employee_id, '')
		FROM base_manage_user WHERE id = ? LIMIT 1`, employeeID).Scan(&username, &employeeCode)
	if strings.EqualFold(strings.TrimSpace(username), "admin") || strings.EqualFold(strings.TrimSpace(employeeCode), "admin") {
		return true
	}
	return hasRoleName(getUserRoleNames(db, employeeID), "管理员", "IT")
}

func miniappEmployeePatientAccessFilter(db *sql.DB, employeeID int, tableAlias string) (string, []interface{}) {
	if miniappEmployeeCanManageAllPatients(db, employeeID) {
		return "", nil
	}
	if filter, args := patientAccessFilterForUser(db, employeeID, tableAlias); filter != "" {
		return filter, args
	}

	column := "sales_person"
	if strings.TrimSpace(tableAlias) != "" {
		column = strings.TrimSpace(tableAlias) + ".sales_person"
	}
	return "(" + column + " = ? OR " + column + " IS NULL OR " + column + " = '')", []interface{}{getMiniappEmployeeCode(db, employeeID)}
}

// HandleUniEmployeeSampleOptions 获取员工小程序新增样本所需选项。
func HandleUniEmployeeSampleOptions(c *app.RequestContext, db *sql.DB) {
	employeeID, ok := requireMiniappEmployee(c, db)
	if !ok {
		return
	}

	sampleTypes, err := getSampleTypesData(db)
	if err != nil {
		log.Printf("Miniapp load sample types error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "获取样本类型失败", Data: nil})
		return
	}
	cancerTypes, err := getCancerTypesData(db)
	if err != nil {
		log.Printf("Miniapp load cancer types error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "获取检测癌种失败", Data: nil})
		return
	}
	treatmentStages, err := getTreatmentStagesData(db)
	if err != nil {
		log.Printf("Miniapp load treatment stages error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "获取患者状态失败", Data: nil})
		return
	}
	employeeNo, err := getEmployeeIDForSampleCode(db, employeeID)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: err.Error(), Data: nil})
		return
	}
	prefix := sampleCodePrefix(employeeNo)

	var historicalSample interface{}
	consentSigned := false
	consentSignedAt := ""
	patientIDs := strings.Split(strings.TrimSpace(c.Query("patient_ids")), ",")
	nextSequence := nextSampleSequence(db, prefix)
	if len(patientIDs) > 1 {
		nextSequence = nextFreshSampleSequence(db, prefix)
	}
	if len(patientIDs) == 1 {
		if patientID, parseErr := strconv.Atoi(strings.TrimSpace(patientIDs[0])); parseErr == nil && patientID > 0 {
			var signedAt sql.NullTime
			if db.QueryRow(`SELECT signed_at FROM patient_informed_consent WHERE patient_id = ? LIMIT 1`, patientID).Scan(&signedAt) == nil && signedAt.Valid {
				consentSigned = true
				consentSignedAt = signedAt.Time.Format("2006-01-02 15:04")
			}
			patientQuery := "SELECT COUNT(*) FROM detect_patient WHERE id = ? AND is_active = 1"
			patientArgs := []interface{}{patientID}
			if accessFilter, accessArgs := miniappEmployeePatientAccessFilter(db, employeeID, ""); accessFilter != "" {
				patientQuery += " AND " + accessFilter
				patientArgs = append(patientArgs, accessArgs...)
			}
			var accessible int
			if db.QueryRow(patientQuery, patientArgs...).Scan(&accessible) == nil && accessible > 0 {
				var sampleID, cancerTypeID, sampleTypeID int
				var cancerTypeName, sampleTypeName, reportType, organization string
				historyErr := db.QueryRow(`SELECT s.id, COALESCE(s.cancer_type_id, 0), COALESCE(ct.name, ''),
					COALESCE(s.sample_type_id, 0), COALESCE(st.name, ''), COALESCE(s.report_type, 'normal'), COALESCE(s.organization, '')
					FROM detect_sample s
					LEFT JOIN setting_cancer_type ct ON s.cancer_type_id = ct.id
					LEFT JOIN setting_sample_type st ON s.sample_type_id = st.id
					WHERE s.patient_id = ?
					ORDER BY COALESCE(s.collection_date, s.sample_created_at) DESC, s.id DESC LIMIT 1`, patientID).Scan(
					&sampleID, &cancerTypeID, &cancerTypeName, &sampleTypeID, &sampleTypeName, &reportType, &organization)
				if historyErr == nil {
					historicalSample = utils.H{
						"id": sampleID, "cancer_type_id": cancerTypeID, "cancer_type_name": cancerTypeName,
						"sample_type_id": sampleTypeID, "sample_type_name": sampleTypeName,
						"report_type": normalizeSampleReportType(reportType), "report_type_label": reportTypeFullLabel(reportType),
						"report_assay_label": reportTypeAssayLabel(reportType), "organization": organization,
					}
				}
			}
		}
	}
	packages := []utils.H{}
	if rows, queryErr := db.Query(`SELECT id, name, detection_count, price, COALESCE(description, '')
		FROM sale_package WHERE status = 'active' ORDER BY id`); queryErr == nil {
		defer rows.Close()
		for rows.Next() {
			var id, detectionCount int
			var name, description string
			var price float64
			if rows.Scan(&id, &name, &detectionCount, &price, &description) == nil {
				packages = append(packages, utils.H{"id": id, "name": name, "detection_count": detectionCount, "price": price, "description": description})
			}
		}
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取成功",
		Data: utils.H{
			"sample_types":      sampleTypes,
			"cancer_types":      cancerTypes,
			"treatment_stages":  treatmentStages,
			"sample_prefix":     prefix,
			"next_sequence":     nextSequence,
			"historical_sample": historicalSample,
			"packages":          packages,
			"consent_signed":    consentSigned,
			"consent_signed_at": consentSignedAt,
			"consent_text":      informedConsentText,
		},
	})
}

// HandleUniEmployeePatients 获取员工患者列表
func HandleUniEmployeePatients(c *app.RequestContext, db *sql.DB) {
	employeeID, ok := requireMiniappEmployee(c, db)
	if !ok {
		return
	}
	canManageAll := miniappEmployeeCanManageAllPatients(db, employeeID)

	keyword := strings.TrimSpace(c.Query("keyword"))
	infoStatus := strings.TrimSpace(c.Query("info_status"))
	groupID, _ := strconv.Atoi(c.Query("group_id"))
	cancerTypeID, _ := strconv.Atoi(c.Query("cancer_type_id"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	where := []string{"is_active = 1"}
	args := []interface{}{}
	if accessFilter, accessArgs := miniappEmployeePatientAccessFilter(db, employeeID, ""); accessFilter != "" {
		where = append(where, accessFilter)
		args = append(args, accessArgs...)
	}
	if keyword != "" {
		where = append(where, "(name LIKE ? OR phone LIKE ? OR id_document_no LIKE ? OR id_card LIKE ? OR patient_code LIKE ?)")
		like := "%" + keyword + "%"
		args = append(args, like, like, like, like, like)
	}
	if groupID > 0 {
		where = append(where, `EXISTS (SELECT 1 FROM sale_patient_group_member gm
			JOIN sale_patient_group g ON g.id = gm.group_id
			WHERE gm.patient_id = detect_patient.id AND g.id = ? AND g.sales_user_id = ?)`)
		args = append(args, groupID, employeeID)
	}
	if cancerTypeID > 0 {
		where = append(where, `EXISTS (SELECT 1 FROM detect_sample fs
			WHERE fs.patient_id = detect_patient.id AND fs.cancer_type_id = ?)`)
		args = append(args, cancerTypeID)
	}
	diagnosisCompletedSQL := `(patient_status = 0
		OR EXISTS (SELECT 1 FROM patient_follow_up fu WHERE fu.patient_id = detect_patient.id LIMIT 1)
		OR COALESCE(NULLIF(TRIM(cancer_pathology), ''), NULLIF(TRIM(prognosis_info), ''), NULLIF(TRIM(other_info), ''), NULLIF(TRIM(report_files), '')) IS NOT NULL)`
	switch infoStatus {
	case "pending":
		where = append(where, "NOT "+diagnosisCompletedSQL)
	case "completed":
		where = append(where, diagnosisCompletedSQL)
	}
	whereSQL := strings.Join(where, " AND ")

	var total int
	if err := db.QueryRow("SELECT COUNT(*) FROM detect_patient WHERE "+whereSQL, args...).Scan(&total); err != nil {
		log.Printf("Count miniapp employee patients error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "查询失败", Data: nil})
		return
	}

	query := `SELECT id, COALESCE(patient_code, ''), COALESCE(name, ''), COALESCE(gender, ''),
		COALESCE(id_document_type, '居民身份证'), COALESCE(NULLIF(id_document_no, ''), COALESCE(id_card, '')), COALESCE(phone, ''), birthday, COALESCE(diagnosis, ''),
		COALESCE(smoking_status, ''), completion_status, patient_status, created_at,
		CASE WHEN patient_status = 0
			OR EXISTS (SELECT 1 FROM patient_follow_up fu WHERE fu.patient_id = detect_patient.id LIMIT 1)
			OR COALESCE(NULLIF(TRIM(cancer_pathology), ''), NULLIF(TRIM(prognosis_info), ''), NULLIF(TRIM(other_info), ''), NULLIF(TRIM(report_files), '')) IS NOT NULL
			THEN 1 ELSE 0 END AS diagnosis_completed,
		COALESCE((SELECT g.id FROM sale_patient_group_member gm
			JOIN sale_patient_group g ON g.id = gm.group_id
			WHERE gm.patient_id = detect_patient.id AND g.sales_user_id = ` + strconv.Itoa(employeeID) + ` LIMIT 1), 0) AS group_id,
		COALESCE((SELECT g.name FROM sale_patient_group_member gm
			JOIN sale_patient_group g ON g.id = gm.group_id
			WHERE gm.patient_id = detect_patient.id AND g.sales_user_id = ` + strconv.Itoa(employeeID) + ` LIMIT 1), '') AS group_name,
		COALESCE((SELECT GROUP_CONCAT(DISTINCT ct.name ORDER BY ct.name SEPARATOR '、')
			FROM detect_sample cs LEFT JOIN setting_cancer_type ct ON ct.id = cs.cancer_type_id
			WHERE cs.patient_id = detect_patient.id), '') AS cancer_types
		FROM detect_patient
		WHERE ` + whereSQL + ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	queryArgs := append([]interface{}{}, args...)
	queryArgs = append(queryArgs, pageSize, (page-1)*pageSize)

	rows, err := db.Query(query, queryArgs...)
	if err != nil {
		log.Printf("Query miniapp employee patients error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "查询失败", Data: nil})
		return
	}
	defer rows.Close()

	list := []utils.H{}
	for rows.Next() {
		var id, completionStatus, patientStatus, diagnosisCompleted, patientGroupID int
		var patientCode, name, gender, idDocumentType, idDocumentNo, phone, diagnosis, smokingStatus, patientGroupName, cancerTypes string
		var birthday sql.NullTime
		var createdAt time.Time
		if err := rows.Scan(&id, &patientCode, &name, &gender, &idDocumentType, &idDocumentNo, &phone, &birthday, &diagnosis, &smokingStatus, &completionStatus, &patientStatus, &createdAt, &diagnosisCompleted, &patientGroupID, &patientGroupName, &cancerTypes); err != nil {
			log.Printf("Scan miniapp employee patient error: %v", err)
			continue
		}
		item := utils.H{
			"id":                  id,
			"patient_code":        patientCode,
			"name":                name,
			"gender":              gender,
			"id_document_type":    idDocumentType,
			"id_document_no":      idDocumentNo,
			"id_card":             idDocumentNo,
			"phone":               phone,
			"diagnosis":           diagnosis,
			"smoking_status":      smokingStatus,
			"completion_status":   completionStatus,
			"patient_status":      patientStatus,
			"diagnosis_completed": diagnosisCompleted,
			"info_status":         map[bool]string{true: "completed", false: "pending"}[diagnosisCompleted == 1],
			"created_at":          createdAt.Format("2006-01-02 15:04"),
			"group_id":            patientGroupID,
			"group_name":          patientGroupName,
			"cancer_types":        cancerTypes,
		}
		if birthday.Valid {
			item["birthday"] = birthday.Time.Format("2006-01-02")
		}
		list = append(list, item)
	}

	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "获取成功", Data: utils.H{
		"list": list, "total": total, "page": page, "page_size": pageSize, "can_manage_all": canManageAll,
	}})
}

// HandleUniEmployeePatientDetail 获取员工端患者详情和样本列表
func HandleUniEmployeePatientDetail(c *app.RequestContext, db *sql.DB) {
	employeeID, ok := requireMiniappEmployee(c, db)
	if !ok {
		return
	}
	patientID, err := strconv.Atoi(c.Param("id"))
	if err != nil || patientID <= 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "无效的患者ID", Data: nil})
		return
	}

	var patientCode, name, gender, idDocumentType, idDocumentNo, phone, address, diagnosis, smokingStatus string
	var cancerDiameter, cancerPathology, prognosisInfo, otherInfo, reportFiles string
	var birthday sql.NullTime
	var completionStatus, patientStatus int
	var createdAt time.Time
	patientQuery := `SELECT COALESCE(patient_code, ''), COALESCE(name, ''), COALESCE(gender, ''),
		COALESCE(id_document_type, '居民身份证'), COALESCE(NULLIF(id_document_no, ''), COALESCE(id_card, '')), COALESCE(phone, ''), birthday, COALESCE(address, ''),
		COALESCE(diagnosis, ''), COALESCE(smoking_status, ''),
		COALESCE(cancer_diameter, ''), COALESCE(cancer_pathology, ''), COALESCE(prognosis_info, ''),
		COALESCE(other_info, ''), COALESCE(report_files, ''),
		completion_status, patient_status, created_at
		FROM detect_patient
		WHERE id = ? AND is_active = 1`
	patientArgs := []interface{}{patientID}
	if accessFilter, accessArgs := miniappEmployeePatientAccessFilter(db, employeeID, ""); accessFilter != "" {
		patientQuery += " AND " + accessFilter
		patientArgs = append(patientArgs, accessArgs...)
	}
	patientQuery += " LIMIT 1"
	err = db.QueryRow(patientQuery, patientArgs...).Scan(
		&patientCode, &name, &gender, &idDocumentType, &idDocumentNo, &phone, &birthday, &address,
		&diagnosis, &smokingStatus, &cancerDiameter, &cancerPathology, &prognosisInfo,
		&otherInfo, &reportFiles, &completionStatus, &patientStatus, &createdAt,
	)
	if err == sql.ErrNoRows {
		c.JSON(consts.StatusNotFound, ApiResponse{Code: 404, Success: false, Message: "未找到患者", Data: nil})
		return
	}
	if err != nil {
		log.Printf("Query miniapp employee patient detail error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "查询失败", Data: nil})
		return
	}

	patient := utils.H{
		"id":                patientID,
		"patient_code":      patientCode,
		"name":              name,
		"gender":            gender,
		"id_document_type":  idDocumentType,
		"id_document_no":    idDocumentNo,
		"id_card":           idDocumentNo,
		"phone":             phone,
		"address":           address,
		"diagnosis":         diagnosis,
		"smoking_status":    smokingStatus,
		"cancer_diameter":   cancerDiameter,
		"cancer_pathology":  cancerPathology,
		"prognosis_info":    prognosisInfo,
		"other_info":        otherInfo,
		"report_files":      reportFiles,
		"completion_status": completionStatus,
		"patient_status":    patientStatus,
		"created_at":        createdAt.Format("2006-01-02 15:04"),
	}
	if birthday.Valid {
		patient["birthday"] = birthday.Time.Format("2006-01-02")
	}

	rows, err := db.Query(`SELECT s.id, s.sample_code, s.collection_date, s.sample_status,
		s.receive_date, s.notes, COALESCE(s.sample_type_id, 0), COALESCE(st.name, ''),
		COALESCE(s.cancer_type_id, 0), COALESCE(ct.name, ''), COALESCE(s.report_type, 'normal'),
		COALESCE(s.organization, ''), COALESCE(ts.name, ''),
		s.sample_created_at, s.sample_updated_at, s.test_completed_at,
		(SELECT r.id FROM detect_report r WHERE r.sample_id = s.id AND r.status IN ('reviewed', 'published') ORDER BY r.created_at DESC LIMIT 1),
		(SELECT r.generated_time FROM detect_report r WHERE r.sample_id = s.id AND r.status IN ('reviewed', 'published') ORDER BY r.created_at DESC LIMIT 1),
		(SELECT r.reviewed_time FROM detect_report r WHERE r.sample_id = s.id AND r.status IN ('reviewed', 'published') ORDER BY r.created_at DESC LIMIT 1),
		(SELECT r.status FROM detect_report r WHERE r.sample_id = s.id AND r.status IN ('reviewed', 'published') ORDER BY r.created_at DESC LIMIT 1)
		FROM detect_sample s
		LEFT JOIN setting_sample_type st ON s.sample_type_id = st.id
		LEFT JOIN setting_cancer_type ct ON s.cancer_type_id = ct.id
		LEFT JOIN setting_treatment_stage ts ON s.treatment_stage_id = ts.id
		WHERE s.patient_id = ?
		ORDER BY COALESCE(s.collection_date, s.sample_created_at) DESC`, patientID)
	if err != nil {
		log.Printf("Query miniapp employee patient samples error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "查询样本失败", Data: nil})
		return
	}
	defer rows.Close()

	samples := []utils.H{}
	for rows.Next() {
		var id, sampleTypeID, cancerTypeID int
		var sampleCode, sampleStatus, sampleTypeName, cancerTypeName, reportType, organization, treatmentStageName string
		var collectionDate, receiveDate, sampleCreatedAt, sampleUpdatedAt, testCompletedAt sql.NullTime
		var reportGeneratedTime, reportReviewedTime sql.NullTime
		var notes, publicReportStatus sql.NullString
		var reportID sql.NullInt64
		if err := rows.Scan(&id, &sampleCode, &collectionDate, &sampleStatus,
			&receiveDate, &notes, &sampleTypeID, &sampleTypeName, &cancerTypeID, &cancerTypeName, &reportType, &organization, &treatmentStageName,
			&sampleCreatedAt, &sampleUpdatedAt, &testCompletedAt,
			&reportID, &reportGeneratedTime, &reportReviewedTime, &publicReportStatus); err != nil {
			log.Printf("Scan miniapp employee patient sample error: %v", err)
			continue
		}
		sample := utils.H{
			"id":                 id,
			"sample_code":        sampleCode,
			"sample_status":      sampleStatus,
			"sample_type_id":     sampleTypeID,
			"sample_type":        sampleTypeName,
			"cancer_type_id":     cancerTypeID,
			"cancer_type":        cancerTypeName,
			"report_type":        normalizeSampleReportType(reportType),
			"report_type_label":  reportTypeFullLabel(reportType),
			"report_assay_label": reportTypeAssayLabel(reportType),
			"organization":       organization,
			"treatment_stage":    treatmentStageName,
		}
		if reportID.Valid {
			sample["report_id"] = reportID.Int64
			sample["has_report"] = true
		} else {
			sample["has_report"] = false
		}
		if collectionDate.Valid {
			sample["collection_date"] = collectionDate.Time.Format("2006-01-02")
		}
		if receiveDate.Valid {
			sample["receive_date"] = receiveDate.Time.Format("2006-01-02")
		}
		if sampleCreatedAt.Valid {
			sample["sample_created_at"] = sampleCreatedAt.Time.Format("2006-01-02 15:04")
		}
		if sampleUpdatedAt.Valid {
			sample["sample_updated_at"] = sampleUpdatedAt.Time.Format("2006-01-02 15:04")
		}
		if testCompletedAt.Valid {
			sample["test_completed_at"] = testCompletedAt.Time.Format("2006-01-02 15:04")
		}
		if reportGeneratedTime.Valid {
			sample["report_generated_time"] = reportGeneratedTime.Time.Format("2006-01-02 15:04")
		}
		if reportReviewedTime.Valid {
			sample["report_reviewed_time"] = reportReviewedTime.Time.Format("2006-01-02 15:04")
		}
		if publicReportStatus.Valid {
			sample["public_report_status"] = publicReportStatus.String
		}
		if notes.Valid {
			sample["notes"] = notes.String
		}
		samples = append(samples, sample)
	}

	followUps := employeePatientFollowUps(db, patientID)
	patient["diagnosis_completed"] = patientStatus == 0 || len(followUps) > 0 || strings.TrimSpace(cancerPathology+prognosisInfo+otherInfo+reportFiles) != ""

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取成功",
		Data:    utils.H{"patient": patient, "samples": samples, "follow_ups": followUps, "sample_total": len(samples)},
	})
}

func employeePatientFollowUps(db *sql.DB, patientID int) []utils.H {
	rows, err := db.Query(`SELECT id, diagnosis_info, report_notes, images_json, created_at, updated_at
		FROM patient_follow_up WHERE patient_id = ? ORDER BY created_at DESC`, patientID)
	if err != nil {
		log.Printf("Query employee patient follow ups error: %v", err)
		return []utils.H{}
	}
	defer rows.Close()
	list := []utils.H{}
	for rows.Next() {
		var id int
		var diagnosisInfo, reportNotes, imagesJSON sql.NullString
		var createdAt, updatedAt sql.NullTime
		if err := rows.Scan(&id, &diagnosisInfo, &reportNotes, &imagesJSON, &createdAt, &updatedAt); err != nil {
			continue
		}
		item := utils.H{"id": id}
		if diagnosisInfo.Valid {
			item["diagnosis_info"] = diagnosisInfo.String
		}
		if reportNotes.Valid {
			item["report_notes"] = reportNotes.String
		}
		images := []string{}
		if imagesJSON.Valid && strings.TrimSpace(imagesJSON.String) != "" {
			_ = json.Unmarshal([]byte(imagesJSON.String), &images)
		}
		item["images"] = images
		if createdAt.Valid {
			item["created_at"] = createdAt.Time.Format("2006-01-02 15:04")
		}
		if updatedAt.Valid {
			item["updated_at"] = updatedAt.Time.Format("2006-01-02 15:04")
		}
		list = append(list, item)
	}
	return list
}

func HandleUniEmployeeCompletePatient(c *app.RequestContext, db *sql.DB) {
	employeeID, ok := requireMiniappEmployee(c, db)
	if !ok {
		return
	}
	patientID, err := strconv.Atoi(c.Param("id"))
	if err != nil || patientID <= 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "无效的患者ID", Data: nil})
		return
	}
	var req struct {
		CancerDiameter  string   `json:"cancer_diameter"`
		CancerPathology string   `json:"cancer_pathology"`
		PrognosisInfo   string   `json:"prognosis_info"`
		OtherInfo       string   `json:"other_info"`
		DiagnosisInfo   string   `json:"diagnosis_info"`
		ReportNotes     string   `json:"report_notes"`
		ReportFiles     []string `json:"report_files"`
	}
	body, err := c.Body()
	if err != nil || json.Unmarshal(body, &req) != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "请求参数错误", Data: nil})
		return
	}
	accessQuery := "SELECT COUNT(*) FROM detect_patient WHERE id = ? AND is_active = 1"
	accessArgs := []interface{}{patientID}
	if accessFilter, scopedArgs := miniappEmployeePatientAccessFilter(db, employeeID, ""); accessFilter != "" {
		accessQuery += " AND " + accessFilter
		accessArgs = append(accessArgs, scopedArgs...)
	}
	var exists int
	if err := db.QueryRow(accessQuery, accessArgs...).Scan(&exists); err != nil || exists == 0 {
		c.JSON(consts.StatusNotFound, ApiResponse{Code: 404, Success: false, Message: "未找到患者", Data: nil})
		return
	}
	req.CancerDiameter = strings.TrimSpace(req.CancerDiameter)
	req.CancerPathology = strings.TrimSpace(req.CancerPathology)
	req.PrognosisInfo = strings.TrimSpace(req.PrognosisInfo)
	req.OtherInfo = strings.TrimSpace(req.OtherInfo)
	req.DiagnosisInfo = strings.TrimSpace(req.DiagnosisInfo)
	req.ReportNotes = strings.TrimSpace(req.ReportNotes)
	if req.CancerDiameter == "" && req.CancerPathology == "" && req.PrognosisInfo == "" &&
		req.OtherInfo == "" && req.DiagnosisInfo == "" && req.ReportNotes == "" && len(req.ReportFiles) == 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "请填写检测信息或上传报告文件", Data: nil})
		return
	}
	filesJSON, _ := json.Marshal(req.ReportFiles)
	reportFiles := strings.Join(req.ReportFiles, ",")
	if _, err := db.Exec(`UPDATE detect_patient SET
		cancer_diameter = ?, cancer_pathology = ?, prognosis_info = ?, other_info = ?,
		report_files = ?, completion_status = 1, updated_at = NOW()
		WHERE id = ?`, req.CancerDiameter, req.CancerPathology, req.PrognosisInfo, req.OtherInfo, reportFiles, patientID); err != nil {
		log.Printf("Update employee patient completion error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "保存失败", Data: nil})
		return
	}
	result, err := db.Exec(`INSERT INTO patient_follow_up
		(patient_id, phone, diagnosis_info, report_notes, images_json, created_at, updated_at)
		SELECT id, phone, ?, ?, ?, NOW(), NOW() FROM detect_patient WHERE id = ?`,
		req.DiagnosisInfo, req.ReportNotes, string(filesJSON), patientID)
	if err != nil {
		log.Printf("Insert employee patient follow up error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "保存记录失败", Data: nil})
		return
	}
	id, _ := result.LastInsertId()
	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "保存成功", Data: utils.H{"id": id}})
}

func HandleUniEmployeeReportFileUpload(c *app.RequestContext, db *sql.DB) {
	allowedExtensions := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".pdf": true,
	}
	handleFileUpload(c, db, "patient-report", allowedExtensions, int64(20*1024*1024))
}

// HandleUniEmployeeCreatePatient 新患录入
func HandleUniEmployeeCreatePatient(c *app.RequestContext, db *sql.DB) {
	employeeID, ok := requireMiniappEmployee(c, db)
	if !ok {
		return
	}
	employeeCode := getMiniappEmployeeCode(db, employeeID)

	var req struct {
		Name           string `json:"name"`
		Gender         string `json:"gender"`
		IDDocumentType string `json:"id_document_type"`
		IDDocumentNo   string `json:"id_document_no"`
		IdCard         string `json:"id_card"`
		Phone          string `json:"phone"`
		Birthday       string `json:"birthday"`
		Address        string `json:"address"`
		Diagnosis      string `json:"diagnosis"`
		CancerDiameter string `json:"cancer_diameter"`
		SmokingStatus  string `json:"smoking_status"`
		PatientStatus  *int   `json:"patient_status"`
	}
	body, err := c.Body()
	if err != nil || json.Unmarshal(body, &req) != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "请求参数错误", Data: nil})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Phone = strings.TrimSpace(req.Phone)
	req.IdCard = strings.TrimSpace(req.IdCard)
	req.IDDocumentType = normalizePatientDocumentType(req.IDDocumentType)
	req.IDDocumentNo = normalizePatientDocumentNo(req.IDDocumentNo)
	if req.IDDocumentNo == "" {
		req.IDDocumentNo = normalizePatientDocumentNo(req.IdCard)
	}
	if req.Name == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "姓名不能为空", Data: nil})
		return
	}
	if req.IDDocumentNo == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "身份证件号不能为空", Data: nil})
		return
	}
	patientStatus := 1
	if req.PatientStatus != nil {
		patientStatus = *req.PatientStatus
	}
	if patientStatus != 0 && patientStatus != 1 {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "患者状态不正确", Data: nil})
		return
	}

	var exists int
	if req.IDDocumentNo != "" {
		if err := db.QueryRow("SELECT COUNT(*) FROM detect_patient WHERE "+patientDocumentWhereClause("?")+" AND is_active = 1", req.IDDocumentNo, req.IDDocumentNo).Scan(&exists); err == nil && exists > 0 {
			c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "身份证件号已存在", Data: nil})
			return
		}
	}
	if isResidentIDCard(req.IDDocumentType) {
		if gender, parsedBirthday, ok := parseResidentIDCardInfo(req.IDDocumentNo); ok {
			req.Gender = gender
			if strings.TrimSpace(req.Birthday) == "" {
				req.Birthday = parsedBirthday
			}
		} else {
			c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "居民身份证号格式不正确", Data: nil})
			return
		}
	}
	if strings.TrimSpace(req.Gender) == "" || strings.TrimSpace(req.Birthday) == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "性别和生日不能为空", Data: nil})
		return
	}
	req.Diagnosis, req.CancerDiameter, err = normalizePatientConditionFields(patientStatus, req.Diagnosis, req.CancerDiameter)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: err.Error(), Data: nil})
		return
	}

	patientCode := generatePatientCode(db)
	var birthday interface{} = nil
	if strings.TrimSpace(req.Birthday) != "" {
		birthday = strings.TrimSpace(req.Birthday)
	}
	result, err := db.Exec(`INSERT INTO detect_patient
		(patient_code, name, gender, id_document_type, id_document_no, id_card, phone, birthday, address, diagnosis, cancer_diameter, smoking_status, sales_person,
		 is_active, completion_status, patient_status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, 1, ?, NOW(), NOW())`,
		patientCode, req.Name, req.Gender, req.IDDocumentType, req.IDDocumentNo, legacyIDCardForDocument(req.IDDocumentType, req.IDDocumentNo), req.Phone, birthday, req.Address, req.Diagnosis, req.CancerDiameter, req.SmokingStatus, employeeCode, patientStatus)
	if err != nil {
		log.Printf("Miniapp create patient error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "新患录入失败", Data: utils.H{"error": err.Error()}})
		return
	}
	id, _ := result.LastInsertId()
	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "新患录入成功", Data: utils.H{"id": id, "patient_code": patientCode}})
}

// HandleUniGetPatientInfo 获取患者个人信息（信息完善回显）
func HandleUniGetPatientInfo(c *app.RequestContext, db *sql.DB) {
	phone, _ := c.Get("miniapp_phone")
	phoneStr, _ := phone.(string)

	if phoneStr == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "未获取到手机号",
			Data:    nil,
		})
		return
	}

	query := `SELECT id, patient_code, name, gender, COALESCE(id_document_type, '居民身份证'), COALESCE(NULLIF(id_document_no, ''), COALESCE(id_card, '')), phone, birthday,
		address, diagnosis, cancer_diameter, smoking_status, COALESCE(patient_source, ''),
		is_active, completion_status, patient_status
		FROM detect_patient WHERE phone = ? AND is_active = 1 LIMIT 1`

	var id, isActive, completionStatus, patientStatus int
	var patientCode, name, gender, idDocumentType, idDocumentNo, patientPhone string
	var birthday sql.NullTime
	var address, diagnosis, cancerDiameter, smokingStatus sql.NullString
	var patientSource string

	err := db.QueryRow(query, phoneStr).Scan(
		&id, &patientCode, &name, &gender, &idDocumentType, &idDocumentNo, &patientPhone, &birthday,
		&address, &diagnosis, &cancerDiameter, &smokingStatus, &patientSource,
		&isActive, &completionStatus, &patientStatus,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(consts.StatusOK, ApiResponse{
				Code:    200,
				Success: true,
				Message: "未找到患者信息",
				Data:    nil,
			})
			return
		}
		log.Printf("Query patient info error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "查询失败",
			Data:    nil,
		})
		return
	}

	patientInfo := utils.H{
		"id":                id,
		"patient_code":      patientCode,
		"name":              name,
		"gender":            gender,
		"id_document_type":  idDocumentType,
		"id_document_no":    idDocumentNo,
		"id_card":           idDocumentNo,
		"phone":             patientPhone,
		"patient_source":    patientSource,
		"is_active":         isActive,
		"completion_status": completionStatus,
		"patient_status":    patientStatus,
	}

	if birthday.Valid {
		patientInfo["birthday"] = birthday.Time.Format("2006-01-02")
	}
	if address.Valid {
		patientInfo["address"] = address.String
	}
	if diagnosis.Valid {
		patientInfo["diagnosis"] = diagnosis.String
	}
	if cancerDiameter.Valid {
		patientInfo["cancer_diameter"] = cancerDiameter.String
	}
	if smokingStatus.Valid {
		patientInfo["smoking_status"] = smokingStatus.String
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取成功",
		Data:    patientInfo,
	})
}

// HandleUniUpdatePatientInfo 更新患者个人信息（信息完善提交）
func HandleUniUpdatePatientInfo(c *app.RequestContext, db *sql.DB) {
	phone, _ := c.Get("miniapp_phone")
	phoneStr, _ := phone.(string)

	var req struct {
		Name           string `json:"name"`
		Gender         string `json:"gender"`
		IdCard         string `json:"id_card"`
		Birthday       string `json:"birthday"`
		Address        string `json:"address"`
		Diagnosis      string `json:"diagnosis"`
		CancerDiameter string `json:"cancer_diameter"`
		SmokingStatus  string `json:"smoking_status"`
	}

	body, err := c.Body()
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "读取请求体失败",
			Data:    nil,
		})
		return
	}

	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "解析请求体失败",
			Data:    nil,
		})
		return
	}

	updateQuery := `UPDATE detect_patient SET 
		name = ?, gender = ?, address = ?, diagnosis = ?,
		cancer_diameter = ?, smoking_status = ?, completion_status = 1,
		updated_at = NOW()
		WHERE phone = ? AND is_active = 1`

	_, err = db.Exec(updateQuery,
		req.Name, req.Gender, req.Address, req.Diagnosis,
		req.CancerDiameter, req.SmokingStatus,
		phoneStr,
	)

	if err != nil {
		log.Printf("Update patient info error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "更新失败",
			Data:    nil,
		})
		return
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "更新成功",
		Data:    nil,
	})
}

// HandleUniGetDetectionPlans 获取患者检测计划/预约列表
func HandleUniGetDetectionPlans(c *app.RequestContext, db *sql.DB) {
	phone, _ := c.Get("miniapp_phone")
	phoneStr, _ := phone.(string)

	query := `SELECT dp.id, dp.sale_order_id, dp.detection_date, dp.detection_number, dp.status,
		so.sale_order_no, sp.name as package_name
		FROM sale_detection_plan dp
		JOIN sale_order so ON dp.sale_order_id = so.id
		JOIN sale_package sp ON so.sale_package_id = sp.id
		JOIN detect_patient p ON dp.detect_patient_id = p.id
		WHERE p.phone = ? AND p.is_active = 1
		ORDER BY dp.detection_date DESC`

	rows, err := db.Query(query, phoneStr)
	if err != nil {
		log.Printf("Query detection plans error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "查询失败",
			Data:    nil,
		})
		return
	}
	defer rows.Close()

	var plans []utils.H
	for rows.Next() {
		var id, orderID, detectionNumber int
		var status, orderNo, packageName string
		var detectionDate sql.NullTime

		err := rows.Scan(&id, &orderID, &detectionDate, &detectionNumber, &status, &orderNo, &packageName)
		if err != nil {
			log.Printf("Scan detection plan error: %v", err)
			continue
		}

		plan := utils.H{
			"id":               id,
			"order_id":         orderID,
			"detection_number": detectionNumber,
			"status":           status,
			"order_no":         orderNo,
			"package_name":     packageName,
		}

		if detectionDate.Valid {
			plan["detection_date"] = detectionDate.Time.Format("2006-01-02")
		}

		plans = append(plans, plan)
	}

	if plans == nil {
		plans = []utils.H{}
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取成功",
		Data:    utils.H{"list": plans, "total": len(plans)},
	})
}

// HandleUniGetMyPackages 获取患者已购买套餐和下一次预计检测时间。
func HandleUniGetMyPackages(c *app.RequestContext, db *sql.DB) {
	phone, _ := c.Get("miniapp_phone")
	phoneStr, _ := phone.(string)

	query := `SELECT so.id, so.sale_order_no, so.first_detection_date, so.payment_status, so.status,
		so.total_amount, so.created_at, sp.id, sp.name, sp.detection_count, sp.interval_days, sp.price,
		p.id, p.name, p.phone, p.address,
		dp.id, dp.detection_date, dp.detection_number, dp.status
		FROM sale_order so
		JOIN sale_package sp ON so.sale_package_id = sp.id
		JOIN detect_patient p ON so.detect_patient_id = p.id
		LEFT JOIN sale_detection_plan dp ON dp.sale_order_id = so.id
		WHERE p.phone = ? AND p.is_active = 1
		ORDER BY so.created_at DESC, dp.detection_number ASC`

	rows, err := db.Query(query, phoneStr)
	if err != nil {
		log.Printf("Query my packages error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "查询失败", Data: nil})
		return
	}
	defer rows.Close()

	orderMap := map[int]utils.H{}
	orderIDs := []int{}
	today := time.Now().Truncate(24 * time.Hour)

	for rows.Next() {
		var orderID, packageID, detectionCount, intervalDays, patientID int
		var orderNo, paymentStatus, orderStatus, packageName, patientName, patientPhone string
		var firstDetectionDate, orderCreatedAt, detectionDate sql.NullTime
		var totalAmount, price float64
		var patientAddress sql.NullString
		var planID, detectionNumber sql.NullInt64
		var planStatus sql.NullString

		if err := rows.Scan(&orderID, &orderNo, &firstDetectionDate, &paymentStatus, &orderStatus,
			&totalAmount, &orderCreatedAt, &packageID, &packageName, &detectionCount, &intervalDays, &price,
			&patientID, &patientName, &patientPhone, &patientAddress,
			&planID, &detectionDate, &detectionNumber, &planStatus); err != nil {
			log.Printf("Scan my packages error: %v", err)
			continue
		}

		item, exists := orderMap[orderID]
		if !exists {
			item = utils.H{
				"id":                  orderID,
				"order_id":            orderID,
				"order_no":            orderNo,
				"payment_status":      paymentStatus,
				"status":              orderStatus,
				"total_amount":        totalAmount,
				"package_id":          packageID,
				"package_name":        packageName,
				"detection_count":     detectionCount,
				"interval_days":       intervalDays,
				"price":               price,
				"patient_id":          patientID,
				"patient_name":        patientName,
				"patient_phone":       patientPhone,
				"plans":               []utils.H{},
				"next_detection_date": "",
			}
			if firstDetectionDate.Valid {
				item["first_detection_date"] = firstDetectionDate.Time.Format("2006-01-02")
			}
			if orderCreatedAt.Valid {
				item["created_at"] = orderCreatedAt.Time.Format("2006-01-02 15:04")
			}
			if patientAddress.Valid {
				item["patient_address"] = patientAddress.String
			}
			orderMap[orderID] = item
			orderIDs = append(orderIDs, orderID)
		}

		if planID.Valid {
			plan := utils.H{
				"id":               int(planID.Int64),
				"order_id":         orderID,
				"detection_number": int(detectionNumber.Int64),
			}
			if detectionDate.Valid {
				dateText := detectionDate.Time.Format("2006-01-02")
				plan["detection_date"] = dateText
				if planStatus.String == "scheduled" && detectionDate.Time.Truncate(24*time.Hour).After(today.AddDate(0, 0, -1)) && item["next_detection_date"] == "" {
					item["next_detection_date"] = dateText
					item["next_plan_id"] = int(planID.Int64)
				}
			}
			if planStatus.Valid {
				plan["status"] = planStatus.String
			}
			item["plans"] = append(item["plans"].([]utils.H), plan)
		}
	}

	list := []utils.H{}
	for _, id := range orderIDs {
		list = append(list, orderMap[id])
	}

	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "获取成功", Data: utils.H{"list": list, "total": len(list)}})
}

// HandleUniCreateSampleBoxRequest 预约邮寄采样盒。
func HandleUniCreateSampleBoxRequest(c *app.RequestContext, db *sql.DB) {
	patientID, _, err := getMiniappPatientSubject(c, db)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "未找到患者信息", Data: nil})
		return
	}

	var req struct {
		OrderID          int    `json:"order_id"`
		PlanID           int    `json:"plan_id"`
		ReceiverName     string `json:"receiver_name"`
		ReceiverPhone    string `json:"receiver_phone"`
		ReceiverAddress  string `json:"receiver_address"`
		Province         string `json:"province"`
		City             string `json:"city"`
		District         string `json:"district"`
		DetailAddress    string `json:"detail_address"`
		ExpectedSendDate string `json:"expected_send_date"`
		Notes            string `json:"notes"`
	}
	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "请求参数错误", Data: utils.H{"error": err.Error()}})
		return
	}

	if req.PlanID > 0 {
		_ = db.QueryRow(`SELECT sale_order_id FROM sale_detection_plan WHERE id = ? AND detect_patient_id = ? LIMIT 1`, req.PlanID, patientID).Scan(&req.OrderID)
	}
	if req.OrderID > 0 && req.PlanID == 0 {
		_ = db.QueryRow(`SELECT id FROM sale_detection_plan WHERE sale_order_id = ? AND detect_patient_id = ? ORDER BY detection_number ASC LIMIT 1`, req.OrderID, patientID).Scan(&req.PlanID)
	}

	req.Province = strings.TrimSpace(req.Province)
	req.City = strings.TrimSpace(req.City)
	req.District = strings.TrimSpace(req.District)
	req.DetailAddress = strings.TrimSpace(req.DetailAddress)
	req.ReceiverAddress = strings.TrimSpace(req.ReceiverAddress)
	if req.ReceiverAddress == "" {
		req.ReceiverAddress = strings.TrimSpace(req.Province + req.City + req.District + req.DetailAddress)
	}

	if req.ReceiverName == "" || req.ReceiverPhone == "" || req.ReceiverAddress == "" {
		var name, phone, address sql.NullString
		_ = db.QueryRow(`SELECT name, phone, address FROM detect_patient WHERE id = ? LIMIT 1`, patientID).Scan(&name, &phone, &address)
		if req.ReceiverName == "" && name.Valid {
			req.ReceiverName = name.String
		}
		if req.ReceiverPhone == "" && phone.Valid {
			req.ReceiverPhone = phone.String
		}
		if req.ReceiverAddress == "" && address.Valid {
			req.ReceiverAddress = address.String
		}
	}
	if req.ReceiverName == "" || req.ReceiverPhone == "" || req.ReceiverAddress == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "请填写收件人、手机号和邮寄地址", Data: nil})
		return
	}

	tx, err := db.Begin()
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "预约失败", Data: nil})
		return
	}
	defer tx.Rollback()

	var orderIDArg interface{}
	if req.OrderID > 0 {
		orderIDArg = req.OrderID
	}
	var planIDArg interface{}
	if req.PlanID > 0 {
		planIDArg = req.PlanID
	}

	result, err := tx.Exec(`INSERT INTO mail_address
		(detect_patient_id, sale_order_id, detection_plan_id, receiver_name, receiver_phone, province, city, district, detail_address, full_address, notes, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'requested')`,
		patientID, orderIDArg, planIDArg, req.ReceiverName, req.ReceiverPhone, req.Province, req.City, req.District, req.DetailAddress, req.ReceiverAddress, req.Notes)
	if err != nil {
		log.Printf("Create sample box request error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "预约失败", Data: utils.H{"error": err.Error()}})
		return
	}
	_, _ = tx.Exec(`INSERT INTO sale_sample_box_request
		(sale_order_id, detection_plan_id, detect_patient_id, receiver_name, receiver_phone, receiver_address, expected_send_date, notes, status)
		VALUES (?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, 'requested')`,
		orderIDArg, planIDArg, patientID, req.ReceiverName, req.ReceiverPhone, req.ReceiverAddress, req.ExpectedSendDate, req.Notes)
	_, _ = tx.Exec(`UPDATE detect_patient SET address = ?, updated_at = NOW()
		WHERE id = ? AND COALESCE(NULLIF(TRIM(address), ''), '') = ''`, req.ReceiverAddress, patientID)

	id, _ := result.LastInsertId()
	if err := tx.Commit(); err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "预约失败", Data: utils.H{"error": err.Error()}})
		return
	}
	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "预约成功", Data: utils.H{"id": id}})
}

func HandleUniGetSampleBoxRequests(c *app.RequestContext, db *sql.DB) {
	patientID, _, err := getMiniappPatientSubject(c, db)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "未找到患者信息", Data: nil})
		return
	}
	rows, err := db.Query(`SELECT ma.id, ma.detect_patient_id, ma.sale_order_id, ma.detection_plan_id,
			ma.receiver_name, ma.receiver_phone, ma.province, ma.city, ma.district, ma.detail_address, ma.full_address,
			ma.express_company, ma.tracking_number, ma.status, ma.notes, ma.shipped_at, ma.created_at, ma.updated_at,
			p.name, p.patient_code, p.phone, so.sale_order_no, sp.name, dp.detection_number, dp.detection_date
		FROM mail_address ma
		JOIN detect_patient p ON ma.detect_patient_id = p.id
		LEFT JOIN sale_order so ON ma.sale_order_id = so.id
		LEFT JOIN sale_package sp ON so.sale_package_id = sp.id
		LEFT JOIN sale_detection_plan dp ON ma.detection_plan_id = dp.id
		WHERE ma.detect_patient_id = ?
		ORDER BY ma.created_at DESC`, patientID)
	if err != nil {
		log.Printf("Query sample box requests error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "查询失败", Data: nil})
		return
	}
	defer rows.Close()
	list, err := scanMailAddressRows(rows)
	if err != nil {
		log.Printf("Scan sample box requests error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "查询失败", Data: nil})
		return
	}
	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "获取成功", Data: utils.H{"list": list, "total": len(list)}})
}

func HandleUniGetPatientManager(c *app.RequestContext, db *sql.DB) {
	patientID, _, err := getMiniappPatientSubject(c, db)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "未找到患者信息", Data: nil})
		return
	}
	var salesCode sql.NullString
	_ = db.QueryRow(`SELECT sales_person FROM detect_patient WHERE id = ? LIMIT 1`, patientID).Scan(&salesCode)

	var manager utils.H
	if salesCode.Valid && strings.TrimSpace(salesCode.String) != "" {
		var id int
		var username, realName, phone string
		err = db.QueryRow(`SELECT id, username, real_name, phone FROM base_manage_user WHERE employee_id = ? AND status = 1 LIMIT 1`, strings.TrimSpace(salesCode.String)).
			Scan(&id, &username, &realName, &phone)
		if err == nil {
			name := realName
			if name == "" {
				name = username
			}
			manager = utils.H{"id": id, "name": name, "phone": phone, "employee_id": strings.TrimSpace(salesCode.String)}
		}
	}
	if manager == nil {
		var salesID sql.NullInt64
		_ = db.QueryRow(`SELECT sales_person_id FROM sale_order WHERE detect_patient_id = ? AND sales_person_id IS NOT NULL ORDER BY created_at DESC LIMIT 1`, patientID).Scan(&salesID)
		if salesID.Valid {
			if m, err := getSalesManager(db, int(salesID.Int64)); err == nil {
				manager = m
			}
		}
	}
	if manager == nil {
		manager = utils.H{"name": "客户经理", "phone": ""}
	}
	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "获取成功", Data: manager})
}

// HandleUniGetReports 获取患者报告列表
func HandleUniGetReports(c *app.RequestContext, db *sql.DB) {
	phone, _ := c.Get("miniapp_phone")
	phoneStr, _ := phone.(string)

	query := `SELECT r.id, r.report_no, r.report_type, r.status, r.generated_time, r.file_path,
		s.sample_code, p.name as patient_name
		FROM detect_report r
		JOIN detect_sample s ON r.sample_id = s.id
		JOIN detect_patient p ON COALESCE(r.patient_id, s.patient_id) = p.id
		WHERE p.phone = ? AND p.is_active = 1 AND r.status IN ('reviewed', 'published')
			AND COALESCE(r.report_role, 'single') <> 'child'
		ORDER BY r.created_at DESC`

	rows, err := db.Query(query, phoneStr)
	if err != nil {
		log.Printf("Query reports error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "查询失败",
			Data:    nil,
		})
		return
	}
	defer rows.Close()

	var reports []utils.H
	for rows.Next() {
		var id int
		var reportNo, reportType, status string
		var generatedTime sql.NullTime
		var filePath, sampleCode, patientName sql.NullString

		err := rows.Scan(&id, &reportNo, &reportType, &status, &generatedTime, &filePath, &sampleCode, &patientName)
		if err != nil {
			log.Printf("Scan report error: %v", err)
			continue
		}

		report := utils.H{
			"id":                id,
			"report_no":         reportNo,
			"report_type":       normalizeAssignedReportType(reportType),
			"report_type_label": reportTypeDisplayLabel(reportType),
			"status":            status,
		}

		if generatedTime.Valid {
			report["generated_time"] = generatedTime.Time.Format("2006-01-02 15:04")
		}
		if filePath.Valid {
			report["file_path"] = filePath.String
		}
		if sampleCode.Valid {
			report["sample_code"] = sampleCode.String
		}
		if patientName.Valid {
			report["patient_name"] = patientName.String
		}

		reports = append(reports, report)
	}

	if reports == nil {
		reports = []utils.H{}
	}

	sampleQuery := `SELECT s.id, s.sample_code, s.sample_status, p.name
		FROM detect_sample s
		JOIN detect_patient p ON s.patient_id = p.id
		WHERE p.phone = ? AND p.is_active = 1
		  AND NOT EXISTS (
			SELECT 1 FROM detect_report r
			WHERE r.sample_id = s.id AND r.status IN ('reviewed', 'published')
		  )
		ORDER BY s.collection_date DESC, s.sample_created_at DESC`
	sampleRows, err := db.Query(sampleQuery, phoneStr)
	if err != nil {
		log.Printf("Query samples without report error: %v", err)
	} else {
		defer sampleRows.Close()
		for sampleRows.Next() {
			var id int
			var sampleCode, sampleStatus string
			var patientName sql.NullString
			if err := sampleRows.Scan(&id, &sampleCode, &sampleStatus, &patientName); err != nil {
				log.Printf("Scan sample without report error: %v", err)
				continue
			}
			item := utils.H{
				"id":            fmt.Sprintf("sample-%d", id),
				"sample_id":     id,
				"sample_code":   sampleCode,
				"sample_status": sampleStatus,
				"status":        "no_report",
				"report_type":   "检验报告",
			}
			if patientName.Valid {
				item["patient_name"] = patientName.String
			}
			reports = append(reports, item)
		}
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取成功",
		Data:    utils.H{"list": reports, "total": len(reports)},
	})
}

// HandleUniGetMailSamples 获取患者邮寄样本列表。
func HandleUniGetMailSamples(c *app.RequestContext, db *sql.DB) {
	phone, _ := c.Get("miniapp_phone")
	phoneStr, _ := phone.(string)

	query := `SELECT s.id, s.sample_code, s.sample_status, s.collection_date, s.notes,
		e.id, e.express_company, e.tracking_number, e.sender_name, e.sender_phone,
		e.sender_address, e.status, e.send_time, e.created_at
		FROM detect_sample s
		JOIN detect_patient p ON s.patient_id = p.id
		LEFT JOIN detect_sample_express e ON e.id = (
			SELECT e2.id FROM detect_sample_express e2
			WHERE e2.sample_id = s.id
			ORDER BY e2.created_at DESC LIMIT 1
		)
		WHERE p.phone = ? AND p.is_active = 1
		  AND (s.notes LIKE '%邮寄样本%' OR e.id IS NOT NULL)
		ORDER BY COALESCE(e.created_at, s.sample_created_at) DESC`

	rows, err := db.Query(query, phoneStr)
	if err != nil {
		log.Printf("Query mail samples error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "查询失败", Data: nil})
		return
	}
	defer rows.Close()

	list := []utils.H{}
	for rows.Next() {
		var sampleID int
		var sampleCode, sampleStatus string
		var collectionDate sql.NullTime
		var notes sql.NullString
		var expressID sql.NullInt64
		var company, trackingNumber, senderName, senderPhone, senderAddress, expressStatus sql.NullString
		var sendTime, createdAt sql.NullTime
		if err := rows.Scan(&sampleID, &sampleCode, &sampleStatus, &collectionDate, &notes,
			&expressID, &company, &trackingNumber, &senderName, &senderPhone,
			&senderAddress, &expressStatus, &sendTime, &createdAt); err != nil {
			log.Printf("Scan mail sample error: %v", err)
			continue
		}
		item := utils.H{
			"id":            sampleID,
			"sample_id":     sampleID,
			"sample_code":   sampleCode,
			"sample_status": sampleStatus,
		}
		if collectionDate.Valid {
			item["collection_date"] = collectionDate.Time.Format("2006-01-02")
		}
		if notes.Valid {
			item["notes"] = notes.String
		}
		if expressID.Valid {
			item["express_id"] = expressID.Int64
		}
		if company.Valid {
			item["express_company"] = company.String
		}
		if trackingNumber.Valid {
			item["tracking_number"] = trackingNumber.String
		}
		if senderName.Valid {
			item["sender_name"] = senderName.String
		}
		if senderPhone.Valid {
			item["sender_phone"] = senderPhone.String
		}
		if senderAddress.Valid {
			item["sender_address"] = senderAddress.String
		}
		if expressStatus.Valid {
			item["express_status"] = expressStatus.String
		}
		if sendTime.Valid {
			item["send_time"] = sendTime.Time.Format("2006-01-02 15:04")
		}
		if createdAt.Valid {
			item["created_at"] = createdAt.Time.Format("2006-01-02 15:04")
		}
		list = append(list, item)
	}

	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "获取成功", Data: utils.H{"list": list, "total": len(list)}})
}

// HandleUniGetHelpCenter 获取小程序帮助中心内容，后台可通过 setting_system 的 MINIAPP_HELP_CENTER_JSON 配置。
func HandleUniGetHelpCenter(c *app.RequestContext, db *sql.DB) {
	defaultCategories := []utils.H{
		{
			"name": "报告查看",
			"items": []utils.H{
				{"question": "什么时候可以查看报告？", "answer": "样本完成检测并通过审核后，可在小程序“查看结果”中查看和下载报告。"},
				{"question": "为什么看不到待审核报告？", "answer": "待审核报告属于内部处理状态，审核完成前不会展示给用户。"},
			},
		},
		{
			"name": "样本服务",
			"items": []utils.H{
				{"question": "如何邮寄样本？", "answer": "进入“样本邮寄”，填写寄件人信息和快递单号后提交。"},
				{"question": "如何查询样本进度？", "answer": "进入“进度查询”，可查看所有样本从创建到出报告的时间线。"},
			},
		},
		{
			"name": "账号与联系",
			"items": []utils.H{
				{"question": "手机号变更怎么办？", "answer": "请联系华微客服协助核验并更新账号信息。"},
				{"question": "如何联系客服？", "answer": "可在“我的-联系我们”查看电话、邮箱和公司地址。"},
			},
		},
	}

	var raw sql.NullString
	err := db.QueryRow(`SELECT key_value FROM setting_system WHERE key_name = 'MINIAPP_HELP_CENTER_JSON' LIMIT 1`).Scan(&raw)
	if err == nil && raw.Valid && strings.TrimSpace(raw.String) != "" {
		var parsed interface{}
		jsonErr := json.Unmarshal([]byte(raw.String), &parsed)
		if jsonErr == nil {
			c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "获取成功", Data: parsed})
			return
		}
		log.Printf("Parse MINIAPP_HELP_CENTER_JSON error: %v", jsonErr)
	}

	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "获取成功", Data: utils.H{"categories": defaultCategories}})
}

func getMiniappPatientSubject(c *app.RequestContext, db *sql.DB) (int, string, error) {
	if patientIDVal, exists := c.Get("miniapp_patient_id"); exists {
		if patientID, ok := patientIDVal.(int); ok && patientID > 0 {
			var phone string
			_ = db.QueryRow(`SELECT phone FROM detect_patient WHERE id = ? LIMIT 1`, patientID).Scan(&phone)
			return patientID, phone, nil
		}
	}

	phone, _ := c.Get("miniapp_phone")
	phoneStr, _ := phone.(string)
	if strings.TrimSpace(phoneStr) == "" {
		return 0, "", fmt.Errorf("未获取到手机号")
	}

	var patientID int
	err := db.QueryRow(`SELECT id FROM detect_patient WHERE phone = ? AND is_active = 1 LIMIT 1`, phoneStr).Scan(&patientID)
	if err != nil {
		return 0, phoneStr, err
	}
	return patientID, phoneStr, nil
}

// HandleUniListFollowUps 获取当前患者随访单列表。
func HandleUniListFollowUps(c *app.RequestContext, db *sql.DB) {
	patientID, _, err := getMiniappPatientSubject(c, db)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "未找到患者信息", Data: nil})
		return
	}

	rows, err := db.Query(`SELECT id, diagnosis_info, report_notes, images_json, created_at, updated_at
		FROM patient_follow_up WHERE patient_id = ? ORDER BY created_at DESC`, patientID)
	if err != nil {
		log.Printf("Query follow ups error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "查询失败", Data: nil})
		return
	}
	defer rows.Close()

	list := []utils.H{}
	for rows.Next() {
		var id int
		var diagnosisInfo, reportNotes, imagesJSON sql.NullString
		var createdAt, updatedAt sql.NullTime
		if err := rows.Scan(&id, &diagnosisInfo, &reportNotes, &imagesJSON, &createdAt, &updatedAt); err != nil {
			log.Printf("Scan follow up error: %v", err)
			continue
		}
		item := utils.H{"id": id}
		if diagnosisInfo.Valid {
			item["diagnosis_info"] = diagnosisInfo.String
		}
		if reportNotes.Valid {
			item["report_notes"] = reportNotes.String
		}
		images := []string{}
		if imagesJSON.Valid && strings.TrimSpace(imagesJSON.String) != "" {
			_ = json.Unmarshal([]byte(imagesJSON.String), &images)
		}
		item["images"] = images
		if createdAt.Valid {
			item["created_at"] = createdAt.Time.Format("2006-01-02 15:04")
		}
		if updatedAt.Valid {
			item["updated_at"] = updatedAt.Time.Format("2006-01-02 15:04")
		}
		list = append(list, item)
	}

	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "获取成功", Data: utils.H{"list": list, "total": len(list)}})
}

// HandleUniCreateFollowUp 新增当前患者随访单。
func HandleUniCreateFollowUp(c *app.RequestContext, db *sql.DB) {
	patientID, phone, err := getMiniappPatientSubject(c, db)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "未找到患者信息", Data: nil})
		return
	}

	var req struct {
		DiagnosisInfo string   `json:"diagnosis_info"`
		ReportNotes   string   `json:"report_notes"`
		Images        []string `json:"images"`
	}
	body, err := c.Body()
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "读取请求体失败", Data: nil})
		return
	}
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "解析请求体失败", Data: nil})
		return
	}

	req.DiagnosisInfo = strings.TrimSpace(req.DiagnosisInfo)
	req.ReportNotes = strings.TrimSpace(req.ReportNotes)
	if req.DiagnosisInfo == "" && req.ReportNotes == "" && len(req.Images) == 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "请填写诊断信息或上传报告图片", Data: nil})
		return
	}
	if len(req.Images) > 3 {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "最多上传3张图片", Data: nil})
		return
	}

	imagesJSON, _ := json.Marshal(req.Images)
	result, err := db.Exec(`INSERT INTO patient_follow_up
		(patient_id, phone, diagnosis_info, report_notes, images_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, NOW(), NOW())`,
		patientID, phone, req.DiagnosisInfo, req.ReportNotes, string(imagesJSON))
	if err != nil {
		log.Printf("Create follow up error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "保存失败", Data: nil})
		return
	}
	id, _ := result.LastInsertId()

	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "保存成功", Data: utils.H{"id": id}})
}

// HandleUniGetReportDetail 获取报告详情
func HandleUniGetReportDetail(c *app.RequestContext, db *sql.DB) {
	idStr := c.Param("id")
	reportID, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的报告ID",
			Data:    nil,
		})
		return
	}

	phone, _ := c.Get("miniapp_phone")
	phoneStr, _ := phone.(string)
	identityType, _ := c.Get("miniapp_identity_type")
	identityTypeStr, _ := identityType.(string)

	query := `SELECT r.id, r.report_no, r.report_type, r.report_data, r.status,
		r.generated_time, r.file_path, s.sample_code,
		p.name as patient_name, p.gender as patient_gender, p.id_card,
		s.collection_date, s.sample_type_id, st.name as sample_type,
		COALESCE(gu.real_name, gu.username), COALESCE(ru.real_name, ru.username),
		COALESCE(tu.real_name, tu.username),
		r.reviewed_time, r.rejected_reason, r.patient_viewed_at, COALESCE(r.patient_view_count, 0)
		FROM detect_report r
		JOIN detect_sample s ON r.sample_id = s.id
		LEFT JOIN detect_batch b ON s.batch_id = b.id
		JOIN detect_patient p ON COALESCE(r.patient_id, s.patient_id) = p.id
		LEFT JOIN setting_sample_type st ON s.sample_type_id = st.id
		LEFT JOIN base_manage_user gu ON r.generated_by = gu.id
		LEFT JOIN base_manage_user ru ON r.reviewed_by = ru.id
		LEFT JOIN base_manage_user tu ON COALESCE(NULLIF(s.test_operator, 0), NULLIF(b.tester_id, 0)) = tu.id
		WHERE r.id = ?`
	args := []interface{}{reportID}
	query += ` AND COALESCE(r.report_role, 'single') <> 'child'`
	if identityTypeStr != "employee" {
		query += ` AND p.phone = ? AND p.is_active = 1`
		args = append(args, phoneStr)
	}

	var id int
	var reportNo, reportType, reportData, status string
	var generatedTime sql.NullTime
	var filePath, sampleCode, patientName, patientGender, idCard, sampleType sql.NullString
	var generatedBy, reviewedBy, inspector, rejectedReason sql.NullString
	var collectionDate sql.NullTime
	var reviewedTime sql.NullTime
	var patientViewedAt sql.NullTime
	var patientViewCount int
	var sampleTypeId sql.NullInt64 // 新增这个变量

	err = db.QueryRow(query, args...).Scan(
		&id, &reportNo, &reportType, &reportData, &status,
		&generatedTime, &filePath, &sampleCode,
		&patientName, &patientGender, &idCard,
		&collectionDate, &sampleTypeId, &sampleType,
		&generatedBy, &reviewedBy, &inspector, &reviewedTime, &rejectedReason, &patientViewedAt, &patientViewCount,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(consts.StatusNotFound, ApiResponse{
				Code:    404,
				Success: false,
				Message: "报告不存在",
				Data:    nil,
			})
			return
		}
		log.Printf("Query report detail error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "查询失败",
			Data:    nil,
		})
		return
	}

	report := utils.H{
		"id":                 id,
		"report_no":          reportNo,
		"report_type":        normalizeAssignedReportType(reportType),
		"report_type_label":  reportTypeDisplayLabel(reportType),
		"status":             status,
		"patient_viewed":     patientViewedAt.Valid,
		"patient_view_count": patientViewCount,
	}
	if identityTypeStr != "employee" {
		if _, updateErr := db.Exec(`UPDATE detect_report
			SET patient_viewed_at = COALESCE(patient_viewed_at, NOW()),
				patient_view_count = COALESCE(patient_view_count, 0) + 1,
				updated_at = updated_at
			WHERE id = ?`, reportID); updateErr == nil {
			if !patientViewedAt.Valid {
				patientViewedAt = sql.NullTime{Time: time.Now(), Valid: true}
			}
			patientViewCount++
			report["patient_viewed"] = true
			report["patient_view_count"] = patientViewCount
		}
	}
	if patientViewedAt.Valid {
		report["patient_viewed_at"] = patientViewedAt.Time.Format("2006-01-02 15:04")
	}

	// 解析报告JSON数据
	var reportDataMap map[string]interface{}
	if reportData != "" {
		var parsedData interface{}
		if jsonErr := json.Unmarshal([]byte(reportData), &parsedData); jsonErr == nil {
			report["report_data"] = parsedData
			// 尝试解析为map以提取特定字段
			if dataMap, ok := parsedData.(map[string]interface{}); ok {
				reportDataMap = dataMap
			}
		}
	}

	// 从report_data中提取特定字段
	if reportDataMap != nil {
		if val, ok := reportDataMap["calculationResult"]; ok {
			if fval, ok := val.(float64); ok {
				report["calculation_result"] = fval
			}
		}
		if val, ok := reportDataMap["signalValueExplanation"]; ok {
			if sval, ok := val.(string); ok {
				report["signal_value_explanation"] = sval
			}
		}
		if val, ok := reportDataMap["resultExplanation"]; ok {
			if sval, ok := val.(string); ok {
				report["result_explanation"] = sval
			}
		}
		if val, ok := reportDataMap["trend"]; ok {
			report["trend"] = val
		}
		trendValues := []utils.H{}
		for i := 1; i <= 4; i++ {
			timeValue, _ := reportDataMap[fmt.Sprintf("time%d", i)].(string)
			typeValue, _ := reportDataMap[fmt.Sprintf("type%d", i)].(string)
			noteValue, _ := reportDataMap[fmt.Sprintf("note%d", i)].(string)
			trendValue, _ := reportDataMap[fmt.Sprintf("trend%d", i)].(string)
			signalValue, hasSignal := reportDataMap[fmt.Sprintf("signal%d", i)].(float64)
			if strings.TrimSpace(timeValue) == "" || !hasSignal {
				continue
			}
			trendValues = append(trendValues, utils.H{
				"time":   timeValue,
				"signal": signalValue,
				"trend":  trendValue,
				"type":   typeValue,
				"note":   noteValue,
			})
		}
		report["trend_values"] = trendValues
		if val, ok := reportDataMap["reporter"]; ok {
			if sval, ok := val.(string); ok {
				report["reporter"] = sval
			}
		}
		if val, ok := reportDataMap["reviewer"]; ok {
			if sval, ok := val.(string); ok {
				report["reviewer"] = sval
			}
		}
	}

	// 计算年龄
	if idCard.Valid && idCard.String != "" {
		report["patient_age"] = calculateAge(idCard.String)
	}

	if generatedTime.Valid {
		report["generated_time"] = generatedTime.Time.Format("2006-01-02 15:04")
		report["report_time"] = generatedTime.Time.Format("2006-01-02")
		report["created_at"] = generatedTime.Time.Format("2006-01-02")
	}
	if reviewedTime.Valid {
		report["reviewed_time"] = reviewedTime.Time.Format("2006-01-02 15:04")
	}
	if filePath.Valid {
		report["file_path"] = filePath.String
	}
	if sampleCode.Valid {
		report["sample_code"] = sampleCode.String
	}
	if patientName.Valid {
		report["patient_name"] = patientName.String
	}
	if patientGender.Valid {
		report["patient_gender"] = patientGender.String
	}
	if collectionDate.Valid {
		report["collection_time"] = collectionDate.Time.Format("2006-01-02")
		report["sample_collected_at"] = collectionDate.Time.Format("2006-01-02")
	}
	if sampleType.Valid {
		report["sample_type"] = sampleType.String
	}
	if generatedBy.Valid {
		report["generated_by"] = generatedBy.String
		report["reporter"] = generatedBy.String
	}
	if reviewedBy.Valid {
		report["reviewed_by"] = reviewedBy.String
		report["reviewer"] = reviewedBy.String
	}
	if inspector.Valid {
		report["inspector"] = inspector.String
	}
	if rejectedReason.Valid {
		report["rejected_reason"] = rejectedReason.String
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取成功",
		Data:    report,
	})
}

// HandleUniDownloadReportPDF 下载报告PDF
func HandleUniDownloadReportPDF(c *app.RequestContext, db *sql.DB) {
	idStr := c.Param("id")
	reportID, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的报告ID",
			Data:    nil,
		})
		return
	}
	mode := string(c.Query("mode"))
	if mode != "download" {
		mode = "view"
	}

	phone, _ := c.Get("miniapp_phone")
	phoneStr, _ := phone.(string)
	identityType, _ := c.Get("miniapp_identity_type")
	identityTypeStr, _ := identityType.(string)

	cleanupExpiredPDFs()

	query := `SELECT r.id, s.sample_code, p.name as patient_name
		FROM detect_report r
		JOIN detect_sample s ON r.sample_id = s.id
		JOIN detect_patient p ON COALESCE(r.patient_id, s.patient_id) = p.id
		WHERE r.id = ?`
	args := []interface{}{reportID}
	if identityTypeStr != "employee" {
		query += ` AND p.phone = ? AND p.is_active = 1`
		args = append(args, phoneStr)
	}

	var id int
	var sampleCode, patientName sql.NullString

	err = db.QueryRow(query, args...).Scan(&id, &sampleCode, &patientName)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(consts.StatusNotFound, ApiResponse{
				Code:    404,
				Success: false,
				Message: "报告不存在或无权访问",
				Data:    nil,
			})
			return
		}
		log.Printf("Query report PDF error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "查询失败",
			Data:    nil,
		})
		return
	}

	reportName := ""
	if patientName.Valid && sampleCode.Valid {
		reportName = fmt.Sprintf("报告_%s_%s", sampleCode.String, patientName.String)
	} else {
		reportName = fmt.Sprintf("报告_%d", reportID)
	}

	filePath, err := generateConcisePDFReport(db, reportID)
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

	fileURL, err := fileURLManager.GenerateOneTimeFileURL(filePath, 5*time.Minute)
	if err != nil {
		log.Printf("生成文件URL失败: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "生成下载链接失败",
			Data:    nil,
		})
		return
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取成功",
		Data: utils.H{
			"url":         fileURL,
			"downloadUrl": fileURL,
			"report_name": reportName,
			"mode":        mode,
		},
	})
}

// HandleUniGetReportPreviewImage 生成小程序报告预览图片。
func HandleUniGetReportPreviewImage(c *app.RequestContext, db *sql.DB) {
	idStr := c.Param("id")
	reportID, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的报告ID",
			Data:    nil,
		})
		return
	}

	phone, _ := c.Get("miniapp_phone")
	phoneStr, _ := phone.(string)
	identityType, _ := c.Get("miniapp_identity_type")
	identityTypeStr, _ := identityType.(string)

	query := `SELECT EXISTS(
		SELECT 1
		FROM detect_report r
		JOIN detect_sample s ON r.sample_id = s.id
		JOIN detect_patient p ON COALESCE(r.patient_id, s.patient_id) = p.id
		WHERE r.id = ?`
	args := []interface{}{reportID}
	if identityTypeStr != "employee" {
		query += ` AND p.phone = ? AND p.is_active = 1`
		args = append(args, phoneStr)
	}
	query += `)`

	var hasAccess bool
	if err := db.QueryRow(query, args...).Scan(&hasAccess); err != nil || !hasAccess {
		c.JSON(consts.StatusNotFound, ApiResponse{
			Code:    404,
			Success: false,
			Message: "报告不存在或无权访问",
			Data:    nil,
		})
		return
	}

	imagePath, err := generateReportPreviewImage(db, reportID)
	if err != nil {
		log.Printf("生成报告预览图片失败: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "生成预览图片失败",
			Data:    nil,
		})
		return
	}
	imageURL, err := fileURLManager.GenerateOneTimeFileURL(imagePath, 30*time.Minute)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "生成预览链接失败",
			Data:    nil,
		})
		return
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取成功",
		Data: utils.H{
			"url": imageURL,
		},
	})
}

// HandleUniGetSamples 获取患者样本列表
func HandleUniGetSamples(c *app.RequestContext, db *sql.DB) {
	phone, _ := c.Get("miniapp_phone")
	phoneStr, _ := phone.(string)
	months := 0
	if rawMonths := strings.TrimSpace(c.Query("months")); rawMonths != "" {
		if parsed, err := strconv.Atoi(rawMonths); err == nil {
			if parsed < 1 {
				parsed = 1
			}
			if parsed > 12 {
				parsed = 12
			}
			months = parsed
		}
	}

	query := `SELECT s.id, s.sample_code, s.collection_date, s.sample_status,
		s.receive_date, s.notes, st.name as sample_type_name, ts.name as treatment_stage_name,
		s.sample_created_at, s.sample_updated_at, s.test_completed_at,
		(SELECT r.generated_time FROM detect_report r WHERE r.sample_id = s.id AND r.status IN ('reviewed', 'published') ORDER BY r.created_at DESC LIMIT 1),
		(SELECT r.reviewed_time FROM detect_report r WHERE r.sample_id = s.id AND r.status IN ('reviewed', 'published') ORDER BY r.created_at DESC LIMIT 1),
		(SELECT r.status FROM detect_report r WHERE r.sample_id = s.id AND r.status IN ('reviewed', 'published') ORDER BY r.created_at DESC LIMIT 1)
		FROM detect_sample s
		JOIN detect_patient p ON s.patient_id = p.id
		LEFT JOIN setting_sample_type st ON s.sample_type_id = st.id
		LEFT JOIN setting_treatment_stage ts ON s.treatment_stage_id = ts.id
		WHERE p.phone = ? AND p.is_active = 1`
	args := []interface{}{phoneStr}
	if months > 0 {
		query += ` AND COALESCE(s.collection_date, s.sample_created_at) >= DATE_SUB(NOW(), INTERVAL ? MONTH)`
		args = append(args, months)
	}
	query += ` ORDER BY COALESCE(s.collection_date, s.sample_created_at) DESC`

	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("Query samples error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "查询失败",
			Data:    nil,
		})
		return
	}
	defer rows.Close()

	var samples []utils.H
	for rows.Next() {
		var id int
		var sampleCode, sampleStatus string
		var collectionDate sql.NullTime
		var receiveDate sql.NullTime
		var sampleCreatedAt, sampleUpdatedAt, testCompletedAt sql.NullTime
		var reportGeneratedTime, reportReviewedTime sql.NullTime
		var notes, sampleTypeName, treatmentStageName sql.NullString
		var publicReportStatus sql.NullString

		err := rows.Scan(&id, &sampleCode, &collectionDate, &sampleStatus,
			&receiveDate, &notes, &sampleTypeName, &treatmentStageName,
			&sampleCreatedAt, &sampleUpdatedAt, &testCompletedAt,
			&reportGeneratedTime, &reportReviewedTime, &publicReportStatus)
		if err != nil {
			log.Printf("Scan sample error: %v", err)
			continue
		}

		sample := utils.H{
			"id":            id,
			"sample_code":   sampleCode,
			"sample_status": sampleStatus,
		}

		if collectionDate.Valid {
			sample["collection_date"] = collectionDate.Time.Format("2006-01-02")
		}
		if receiveDate.Valid {
			sample["receive_date"] = receiveDate.Time.Format("2006-01-02")
		}
		if sampleCreatedAt.Valid {
			sample["sample_created_at"] = sampleCreatedAt.Time.Format("2006-01-02 15:04")
		}
		if sampleUpdatedAt.Valid {
			sample["sample_updated_at"] = sampleUpdatedAt.Time.Format("2006-01-02 15:04")
		}
		if testCompletedAt.Valid {
			sample["test_completed_at"] = testCompletedAt.Time.Format("2006-01-02 15:04")
		}
		if reportGeneratedTime.Valid {
			sample["report_generated_time"] = reportGeneratedTime.Time.Format("2006-01-02 15:04")
		}
		if reportReviewedTime.Valid {
			sample["report_reviewed_time"] = reportReviewedTime.Time.Format("2006-01-02 15:04")
		}
		if publicReportStatus.Valid {
			sample["public_report_status"] = publicReportStatus.String
		}
		if notes.Valid {
			sample["notes"] = notes.String
		}
		if sampleTypeName.Valid {
			sample["sample_type"] = sampleTypeName.String
		}
		if treatmentStageName.Valid {
			sample["treatment_stage"] = treatmentStageName.String
		}

		samples = append(samples, sample)
	}

	if samples == nil {
		samples = []utils.H{}
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取成功",
		Data:    utils.H{"list": samples, "total": len(samples)},
	})
}

// HandleUniCreateMailSample 提交邮寄样本申请
func HandleUniCreateMailSample(c *app.RequestContext, db *sql.DB) {
	phone, _ := c.Get("miniapp_phone")
	phoneStr, _ := phone.(string)

	var req struct {
		SenderName    string `json:"sender_name"`
		SenderPhone   string `json:"sender_phone"`
		SenderAddress string `json:"sender_address"`
		TrackingNo    string `json:"tracking_no"`
		Notes         string `json:"notes"`
	}

	body, err := c.Body()
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "读取请求体失败",
			Data:    nil,
		})
		return
	}

	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "解析请求体失败",
			Data:    nil,
		})
		return
	}

	// 查询患者ID
	var patientID int
	err = db.QueryRow("SELECT id FROM detect_patient WHERE phone = ? AND is_active = 1 LIMIT 1", phoneStr).Scan(&patientID)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "未找到患者信息",
			Data:    nil,
		})
		return
	}

	// 邮寄样本实际上是创建一个样本记录，状态为 collected
	// 使用当前日期作为采集日期
	sampleCode := "HWSM" + time.Now().Format("20060102") + generateRandomString(6)

	insertQuery := `INSERT INTO detect_sample (sample_code, patient_id, collection_date, 
		sample_status, notes, sample_created_at, sample_updated_at, sample_type_id, treatment_stage_id)
		VALUES (?, ?, NOW(), 'collected', ?, NOW(), NOW(), 1, 1)`

	mailNotes := "邮寄样本"
	if req.SenderName != "" {
		mailNotes += " | 寄件人: " + req.SenderName
	}
	if req.TrackingNo != "" {
		mailNotes += " | 快递单号: " + req.TrackingNo
	}
	if req.Notes != "" {
		mailNotes += " | 备注: " + req.Notes
	}

	result, err := db.Exec(insertQuery, sampleCode, patientID, mailNotes)
	if err != nil {
		log.Printf("Create mail sample error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "提交失败",
			Data:    nil,
		})
		return
	}
	sampleID, _ := result.LastInsertId()

	if req.TrackingNo != "" && sampleID > 0 {
		_, err = db.Exec(`INSERT INTO detect_sample_express
			(sample_id, sample_code, tracking_number, sender_name, sender_phone, sender_address, status, notes, send_time)
			VALUES (?, ?, ?, ?, ?, ?, 'in_transit', ?, NOW())`,
			sampleID, sampleCode, req.TrackingNo, req.SenderName, req.SenderPhone, req.SenderAddress, req.Notes)
		if err != nil {
			log.Printf("Create mail sample express error: %v", err)
		}
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "邮寄样本提交成功",
		Data:    utils.H{"sample_code": sampleCode},
	})
}

// ==================== 员工专用接口 ====================

// HandleUniEmployeeStats 获取员工待办统计
func HandleUniEmployeeStats(c *app.RequestContext, db *sql.DB) {
	// 获取待接收样本数量
	var pendingSamples int
	err := db.QueryRow(`SELECT COUNT(*) FROM detect_sample WHERE sample_status IN ('created', 'collected')`).Scan(&pendingSamples)
	if err != nil {
		log.Printf("Query pending samples count error: %v", err)
		pendingSamples = 0
	}

	// 获取待审核报告数量
	var pendingReports int
	err = db.QueryRow(`SELECT COUNT(*) FROM detect_report WHERE status IN ('pending', 'generated')`).Scan(&pendingReports)
	if err != nil {
		log.Printf("Query pending reports count error: %v", err)
		pendingReports = 0
	}

	// 获取新患录入数量（近7天新增患者）
	var newPatients int
	err = db.QueryRow(`SELECT COUNT(*) FROM detect_patient WHERE created_at >= DATE_SUB(NOW(), INTERVAL 7 DAY)`).Scan(&newPatients)
	if err != nil {
		log.Printf("Query new patients count error: %v", err)
		newPatients = 0
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取成功",
		Data: utils.H{
			"pending_samples": pendingSamples,
			"pending_reports": pendingReports,
			"new_patients":    newPatients,
			"pendingSamples":  pendingSamples,
			"pendingReports":  pendingReports,
			"newPatients":     newPatients,
		},
	})
}

// HandleUniEmployeeReports 获取员工报告列表（所有报告）
func HandleUniEmployeeReports(c *app.RequestContext, db *sql.DB) {
	patientName := strings.TrimSpace(c.Query("patient_name"))
	query := `SELECT r.id, r.report_no, r.report_type, r.status, r.generated_time,
		s.sample_code, p.name as patient_name, COALESCE(gu.real_name, gu.username),
		r.patient_viewed_at, COALESCE(r.patient_view_count, 0)
		FROM detect_report r
		JOIN detect_sample s ON r.sample_id = s.id
		JOIN detect_patient p ON r.patient_id = p.id
		LEFT JOIN base_manage_user gu ON r.generated_by = gu.id`
	args := []interface{}{}
	if patientName != "" {
		query += ` WHERE p.name LIKE ?`
		args = append(args, "%"+patientName+"%")
	}
	query += ` ORDER BY r.created_at DESC LIMIT 100`

	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("Query employee reports error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "查询失败",
			Data:    nil,
		})
		return
	}
	defer rows.Close()

	var reports []utils.H
	for rows.Next() {
		var id int
		var reportNo, reportType, status string
		var generatedTime sql.NullTime
		var sampleCode, patientName, generatedBy sql.NullString
		var patientViewedAt sql.NullTime
		var patientViewCount int

		err := rows.Scan(&id, &reportNo, &reportType, &status, &generatedTime, &sampleCode, &patientName, &generatedBy, &patientViewedAt, &patientViewCount)
		if err != nil {
			log.Printf("Scan employee report error: %v", err)
			continue
		}

		report := utils.H{
			"id":                 id,
			"report_no":          reportNo,
			"report_type":        normalizeAssignedReportType(reportType),
			"report_type_label":  reportTypeDisplayLabel(reportType),
			"status":             status,
			"patient_viewed":     patientViewedAt.Valid,
			"patient_view_count": patientViewCount,
		}
		if patientViewedAt.Valid {
			report["patient_viewed_at"] = patientViewedAt.Time.Format("2006-01-02 15:04")
		}

		if generatedTime.Valid {
			report["generated_time"] = generatedTime.Time.Format("2006-01-02 15:04")
		}
		if sampleCode.Valid {
			report["sample_code"] = sampleCode.String
		}
		if patientName.Valid {
			report["patient_name"] = patientName.String
		}
		if generatedBy.Valid {
			report["generated_by"] = generatedBy.String
		}

		reports = append(reports, report)
	}

	if reports == nil {
		reports = []utils.H{}
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取成功",
		Data:    utils.H{"list": reports, "total": len(reports)},
	})
}

// HandleUniEmployeePendingReports 获取待审核报告列表
func HandleUniEmployeePendingReports(c *app.RequestContext, db *sql.DB) {
	query := `SELECT r.id, r.report_no, r.report_type, r.status, r.generated_time, COALESCE(gu.real_name, gu.username),
		s.sample_code, p.name as patient_name
		FROM detect_report r
		JOIN detect_sample s ON r.sample_id = s.id
		JOIN detect_patient p ON r.patient_id = p.id
		LEFT JOIN base_manage_user gu ON r.generated_by = gu.id
		WHERE r.status IN ('pending', 'generated')
		ORDER BY r.created_at DESC`

	rows, err := db.Query(query)
	if err != nil {
		log.Printf("Query pending reports error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "查询失败",
			Data:    nil,
		})
		return
	}
	defer rows.Close()

	var reports []utils.H
	for rows.Next() {
		var id int
		var reportNo, reportType, status string
		var generatedTime sql.NullTime
		var generatedBy, sampleCode, patientName sql.NullString

		err := rows.Scan(&id, &reportNo, &reportType, &status, &generatedTime, &generatedBy, &sampleCode, &patientName)
		if err != nil {
			log.Printf("Scan pending report error: %v", err)
			continue
		}

		report := utils.H{
			"id":                id,
			"report_no":         reportNo,
			"report_type":       normalizeAssignedReportType(reportType),
			"report_type_label": reportTypeDisplayLabel(reportType),
			"status":            status,
		}

		if generatedTime.Valid {
			report["generated_time"] = generatedTime.Time.Format("2006-01-02 15:04")
		}
		if generatedBy.Valid {
			report["generated_by"] = generatedBy.String
		}
		if sampleCode.Valid {
			report["sample_code"] = sampleCode.String
		}
		if patientName.Valid {
			report["patient_name"] = patientName.String
		}

		reports = append(reports, report)
	}

	if reports == nil {
		reports = []utils.H{}
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取成功",
		Data:    utils.H{"list": reports, "total": len(reports)},
	})
}

// HandleUniEmployeeReviewReport 审核报告
func HandleUniEmployeeReviewReport(c *app.RequestContext, db *sql.DB) {
	idStr := c.Param("id")
	reportID, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的报告ID",
			Data:    nil,
		})
		return
	}

	var req struct {
		Status         string `json:"status"`
		RejectedReason string `json:"rejectedReason"`
		ReviewerID     int    `json:"reviewer_id"`
	}

	body, err := c.Body()
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "读取请求体失败",
			Data:    nil,
		})
		return
	}

	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "解析请求体失败",
			Data:    nil,
		})
		return
	}

	if req.Status != "reviewed" && req.Status != "rejected" {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的状态值",
			Data:    nil,
		})
		return
	}
	if req.Status == "rejected" && req.RejectedReason == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请填写拒绝原因",
			Data:    nil,
		})
		return
	}

	var updateQuery string
	var args []interface{}
	reviewerID := getMiniappEmployeeID(c, db)
	roleNames := getUserRoleNames(db, reviewerID)
	if hasRoleName(roleNames, "销售") && !hasRoleName(roleNames, "管理员", "IT") {
		c.JSON(consts.StatusForbidden, ApiResponse{
			Code:    403,
			Success: false,
			Message: "销售账号无审核报告权限",
			Data:    nil,
		})
		return
	}
	if req.Status == "reviewed" && req.ReviewerID > 0 {
		if !isValidReportReviewer(db, req.ReviewerID) {
			c.JSON(consts.StatusBadRequest, ApiResponse{
				Code:    400,
				Success: false,
				Message: "请选择管理角色且工号不为admin的审核人",
				Data:    nil,
			})
			return
		}
		reviewerID = req.ReviewerID
	} else if req.Status == "reviewed" && mustSelectRealReportReviewer(db, reviewerID) {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请选择真实审核人",
			Data:    nil,
		})
		return
	}

	if req.Status == "rejected" {
		updateQuery = `UPDATE detect_report SET status = ?, rejected_reason = ?, reviewed_by = ?, reviewed_time = NOW(), updated_at = NOW() WHERE id = ? AND status IN ('pending', 'generated')`
		args = []interface{}{req.Status, req.RejectedReason, reviewerID, reportID}
	} else {
		updateQuery = `UPDATE detect_report SET status = ?, rejected_reason = NULL, reviewed_by = ?, reviewed_time = NOW(), updated_at = NOW() WHERE id = ? AND status IN ('pending', 'generated')`
		args = []interface{}{req.Status, reviewerID, reportID}
	}

	result, err := db.Exec(updateQuery, args...)
	if err != nil {
		log.Printf("Update report review error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "操作失败",
			Data:    nil,
		})
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "报告不存在或当前状态不可审核",
			Data:    nil,
		})
		return
	}

	if req.Status == "reviewed" {
		go sendReportReadySMS(db, reportID)
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "审核成功",
		Data:    nil,
	})
}

func HandleUniEmployeeReportReviewers(c *app.RequestContext, db *sql.DB) {
	employeeID := getMiniappEmployeeID(c, db)
	if employeeID <= 0 {
		c.JSON(consts.StatusUnauthorized, ApiResponse{Code: 401, Success: false, Message: "未登录", Data: nil})
		return
	}
	rows, err := db.Query(`SELECT id, username, COALESCE(real_name, ''), COALESCE(employee_id, '')
		FROM base_manage_user
		WHERE status = 1
		ORDER BY real_name, username`)
	if err != nil {
		log.Printf("Query report reviewers error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "获取审核人失败", Data: nil})
		return
	}
	defer rows.Close()

	reviewers := []utils.H{}
	for rows.Next() {
		var id int
		var username, realName, employeeIDText string
		if err := rows.Scan(&id, &username, &realName, &employeeIDText); err != nil {
			continue
		}
		if !isValidReportReviewer(db, id) {
			continue
		}
		displayName := strings.TrimSpace(realName)
		if displayName == "" {
			displayName = username
		}
		reviewers = append(reviewers, utils.H{
			"id":          id,
			"name":        displayName,
			"username":    username,
			"employee_id": employeeIDText,
		})
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取成功",
		Data: utils.H{
			"requires_reviewer": mustSelectRealReportReviewer(db, employeeID),
			"list":              reviewers,
		},
	})
}

// HandleUniEmployeePendingSamples 获取待接收样本列表
func HandleUniEmployeePendingSamples(c *app.RequestContext, db *sql.DB) {
	query := `SELECT s.id, s.sample_code, s.sample_status, s.collection_date, s.sample_type_id,
		p.name as patient_name, p.gender, p.birthday, st.name as sample_type
		FROM detect_sample s
		JOIN detect_patient p ON s.patient_id = p.id
		LEFT JOIN setting_sample_type st ON s.sample_type_id = st.id
		WHERE s.sample_status IN ('created', 'collected')
		ORDER BY s.collection_date DESC`

	rows, err := db.Query(query)
	if err != nil {
		log.Printf("Query pending samples error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "查询失败",
			Data:    nil,
		})
		return
	}
	defer rows.Close()

	var samples []utils.H
	for rows.Next() {
		var id int
		var sampleTypeID sql.NullInt64
		var sampleCode, sampleStatus string
		var collectionDate sql.NullTime
		var patientName, gender, sampleType sql.NullString
		var birthday sql.NullTime

		err := rows.Scan(&id, &sampleCode, &sampleStatus, &collectionDate, &sampleTypeID,
			&patientName, &gender, &birthday, &sampleType)
		if err != nil {
			log.Printf("Scan pending sample error: %v", err)
			continue
		}

		sample := utils.H{
			"id":            id,
			"sample_code":   sampleCode,
			"sample_status": sampleStatus,
		}
		if sampleTypeID.Valid {
			sample["sample_type_id"] = sampleTypeID.Int64
		}

		if collectionDate.Valid {
			sample["collection_date"] = collectionDate.Time.Format("2006-01-02")
		}
		if patientName.Valid {
			sample["patient_name"] = patientName.String
		}
		if gender.Valid {
			sample["patient_gender"] = gender.String
		}
		if birthday.Valid {
			sample["patient_birthday"] = birthday.Time.Format("2006-01-02")
		}
		if sampleType.Valid {
			sample["sample_type"] = sampleType.String
		}

		samples = append(samples, sample)
	}

	if samples == nil {
		samples = []utils.H{}
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取成功",
		Data:    utils.H{"list": samples, "total": len(samples)},
	})
}

// HandleUniEmployeeAllocateSamples 员工小程序分配样本编号
func HandleUniEmployeeAllocateSamples(c *app.RequestContext, db *sql.DB) {
	employeeID, ok := requireMiniappEmployee(c, db)
	if !ok {
		return
	}

	var req struct {
		PatientIDs           []int  `json:"patient_ids"`
		SampleTypeID         int    `json:"sample_type_id"`
		CancerTypeID         int    `json:"cancer_type_id"`
		TreatmentStageID     int    `json:"treatment_stage_id"`
		ReportType           string `json:"report_type"`
		StartSequence        int    `json:"start_sequence"`
		ManualSuffix         string `json:"manual_suffix"`
		Organization         string `json:"organization"`
		Notes                string `json:"notes"`
		ServiceMode          string `json:"service_mode"`
		SalePackageID        int    `json:"sale_package_id"`
		ConsentSignature     string `json:"consent_signature"`
		ConsentSignedName    string `json:"consent_signed_name"`
		ReturnExpressCompany string `json:"return_express_company"`
		ReturnTrackingNumber string `json:"return_tracking_number"`
	}
	body, err := c.Body()
	if err != nil || json.Unmarshal(body, &req) != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "请求参数错误", Data: nil})
		return
	}
	if len(req.PatientIDs) == 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "请选择患者", Data: nil})
		return
	}
	if req.SampleTypeID <= 0 || req.CancerTypeID <= 0 || req.TreatmentStageID <= 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "请完善样本类型、检测癌种和治疗阶段", Data: nil})
		return
	}
	if strings.TrimSpace(req.ReportType) == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "请选择高敏或超敏", Data: nil})
		return
	}
	req.Organization = strings.TrimSpace(req.Organization)
	if req.Organization == "" {
		req.Organization = "个人送检"
	}
	req.ServiceMode = strings.ToLower(strings.TrimSpace(req.ServiceMode))
	if req.ServiceMode == "" {
		req.ServiceMode = "single"
	}
	if req.ServiceMode != "single" && req.ServiceMode != "package" {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "检测方式不正确", Data: nil})
		return
	}
	if req.ServiceMode == "package" && req.SalePackageID <= 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "请选择检测套餐", Data: nil})
		return
	}
	if len(req.PatientIDs) == 1 {
		var consentCount int
		_ = db.QueryRow(`SELECT COUNT(*) FROM patient_informed_consent WHERE patient_id = ?`, req.PatientIDs[0]).Scan(&consentCount)
		if consentCount == 0 && (strings.TrimSpace(req.ConsentSignature) == "" || strings.TrimSpace(req.ConsentSignedName) == "") {
			c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "请阅读知情同意书并完成手写签名", Data: nil})
			return
		}
	}

	employeeNo, err := getEmployeeIDForSampleCode(db, employeeID)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: err.Error(), Data: nil})
		return
	}
	prefix := sampleCodePrefix(employeeNo)
	startSequence := req.StartSequence
	req.ManualSuffix = strings.TrimSpace(req.ManualSuffix)
	if req.ManualSuffix != "" {
		if len(req.PatientIDs) != 1 {
			c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "自定义后4位只能用于单个患者分配", Data: nil})
			return
		}
		if len(req.ManualSuffix) != 4 {
			c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "后4位必须为4位数字", Data: nil})
			return
		}
		if _, err := strconv.Atoi(req.ManualSuffix); err != nil {
			c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "后4位必须为4位数字", Data: nil})
			return
		}
		startSequence, _ = strconv.Atoi(req.ManualSuffix)
	}
	if startSequence <= 0 {
		startSequence = nextSampleSequence(db, prefix)
	}
	if startSequence > 9999 || startSequence+len(req.PatientIDs)-1 > 9999 {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "后4位序号超出范围", Data: nil})
		return
	}

	codes := make([]string, 0, len(req.PatientIDs))
	for i := range req.PatientIDs {
		codes = append(codes, buildSampleCode(prefix, startSequence+i))
	}
	for _, code := range codes {
		exists, err := sampleCodeExists(db, code)
		if err != nil {
			log.Printf("Miniapp check sample code error: %v", err)
			c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "服务器内部错误", Data: nil})
			return
		}
		if exists {
			c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: fmt.Sprintf("样本编号 %s 已存在，请调整起始序号", code), Data: nil})
			return
		}
	}

	tx, err := db.Begin()
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "服务器内部错误", Data: nil})
		return
	}
	defer tx.Rollback()

	created := []utils.H{}
	reportType := normalizeSampleReportType(req.ReportType)
	patientAccessFilter, patientAccessArgs := miniappEmployeePatientAccessFilter(db, employeeID, "")
	for i, patientID := range req.PatientIDs {
		var patientName string
		var count int
		patientQuery := "SELECT COUNT(*), COALESCE(MAX(name), '') FROM detect_patient WHERE id = ? AND is_active = 1"
		queryArgs := []interface{}{patientID}
		if patientAccessFilter != "" {
			patientQuery += " AND " + patientAccessFilter
			queryArgs = append(queryArgs, patientAccessArgs...)
		}
		if err := tx.QueryRow(patientQuery, queryArgs...).Scan(&count, &patientName); err != nil || count == 0 {
			c.JSON(consts.StatusForbidden, ApiResponse{Code: 403, Success: false, Message: fmt.Sprintf("无权操作患者 %d", patientID), Data: nil})
			return
		}
		code := codes[i]
		if strings.TrimSpace(req.ConsentSignature) != "" {
			if _, err := tx.Exec(`INSERT INTO patient_informed_consent
				(patient_id, consent_version, consent_text, signature_data, signed_name, signed_by_user_id, signed_at, created_at, updated_at)
				VALUES (?, 'v1', ?, ?, ?, ?, NOW(), NOW(), NOW())
				ON DUPLICATE KEY UPDATE patient_id = patient_id`,
				patientID, informedConsentText, req.ConsentSignature, strings.TrimSpace(req.ConsentSignedName), employeeID); err != nil {
				c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "保存知情同意书失败", Data: nil})
				return
			}
		}
		result, err := tx.Exec(`INSERT INTO detect_sample
			(sample_code, patient_id, sample_type_id, cancer_type_id, treatment_stage_id, collection_date, collection_operator,
			 sample_status, report_type, notes, organization, service_mode, sale_package_id, sample_created_at, sample_updated_at)
			VALUES (?, ?, ?, ?, ?, NOW(), ?, 'created', ?, ?, ?, ?, ?, NOW(), NOW())`,
			code, patientID, req.SampleTypeID, req.CancerTypeID, req.TreatmentStageID, employeeID, reportType, req.Notes, req.Organization,
			req.ServiceMode, nullablePositiveInt(req.SalePackageID))
		if err != nil {
			log.Printf("Miniapp allocate sample %s error: %v", code, err)
			c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "新增样本失败", Data: utils.H{"error": err.Error()}})
			return
		}
		id, _ := result.LastInsertId()
		if tracking := strings.TrimSpace(req.ReturnTrackingNumber); tracking != "" {
			if _, err := tx.Exec(`INSERT INTO detect_sample_express
				(sample_id, sample_code, express_company, tracking_number, status, created_at, updated_at)
				VALUES (?, ?, ?, ?, 'in_transit', NOW(), NOW())`,
				id, code, strings.TrimSpace(req.ReturnExpressCompany), tracking); err != nil {
				c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "保存回寄快递单号失败", Data: nil})
				return
			}
		}
		markSampleCodeUsed(tx, code)
		created = append(created, utils.H{"id": id, "sample_code": code, "patient_id": patientID, "patient_name": patientName})
	}

	if err := tx.Commit(); err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "服务器内部错误", Data: nil})
		return
	}
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "新增样本成功",
		Data: utils.H{
			"prefix":          prefix,
			"start_sequence":  startSequence,
			"created_samples": created,
		},
	})
}

// HandleUniEmployeeSamples 获取当前员工亲自创建的样本。
func HandleUniEmployeeSamples(c *app.RequestContext, db *sql.DB) {
	employeeID, ok := requireMiniappEmployee(c, db)
	if !ok {
		return
	}
	rows, err := db.Query(`SELECT s.id, s.sample_code, s.sample_status, s.collection_date,
		COALESCE(p.name, ''), COALESCE(p.patient_code, ''), COALESCE(st.name, ''),
		COALESCE(ct.name, ''), COALESCE(s.report_type, 'normal'), COALESCE(s.organization, ''),
		COALESCE(s.batch_id, 0),
		EXISTS(SELECT 1 FROM detect_report r WHERE r.sample_id = s.id)
		FROM detect_sample s
		LEFT JOIN detect_patient p ON s.patient_id = p.id
		LEFT JOIN setting_sample_type st ON s.sample_type_id = st.id
		LEFT JOIN setting_cancer_type ct ON s.cancer_type_id = ct.id
		WHERE s.collection_operator = ?
		ORDER BY s.sample_created_at DESC, s.id DESC`, employeeID)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "查询样本失败", Data: nil})
		return
	}
	defer rows.Close()
	list := []utils.H{}
	for rows.Next() {
		var id, batchID int
		var sampleCode, status, patientName, patientCode, sampleType, cancerType, reportType, organization string
		var collectionDate sql.NullTime
		var hasReport bool
		if err := rows.Scan(&id, &sampleCode, &status, &collectionDate, &patientName, &patientCode,
			&sampleType, &cancerType, &reportType, &organization, &batchID, &hasReport); err != nil {
			continue
		}
		item := utils.H{
			"id": id, "sample_code": sampleCode, "sample_status": status,
			"patient_name": patientName, "patient_code": patientCode,
			"sample_type": sampleType, "cancer_type": cancerType,
			"report_type": normalizeSampleReportType(reportType), "report_type_label": reportTypeFullLabel(reportType),
			"organization": organization,
			"can_delete":   (status == "created" || status == "collected") && batchID == 0 && !hasReport,
		}
		if collectionDate.Valid {
			item["collection_date"] = collectionDate.Time.Format("2006-01-02")
		}
		list = append(list, item)
	}
	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "获取成功", Data: utils.H{"list": list, "total": len(list)}})
}

// HandleUniEmployeeSampleDetail 获取员工可访问的单个样本详情。
func HandleUniEmployeeSampleDetail(c *app.RequestContext, db *sql.DB) {
	employeeID, ok := requireMiniappEmployee(c, db)
	if !ok {
		return
	}
	sampleID, err := strconv.Atoi(c.Param("id"))
	if err != nil || sampleID <= 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "无效的样本ID", Data: nil})
		return
	}
	query := `SELECT s.id, s.sample_code, s.sample_status, COALESCE(p.name, ''), COALESCE(p.patient_code, ''),
		COALESCE(st.name, ''), COALESCE(ct.name, ''), COALESCE(s.report_type, 'normal'),
		COALESCE(s.organization, ''), COALESCE(ts.name, ''), s.collection_date, s.receive_date,
		COALESCE(s.notes, ''),
		(SELECT r.id FROM detect_report r WHERE r.sample_id = s.id AND r.status IN ('reviewed', 'published') ORDER BY r.created_at DESC LIMIT 1)
		FROM detect_sample s
		JOIN detect_patient p ON s.patient_id = p.id
		LEFT JOIN setting_sample_type st ON s.sample_type_id = st.id
		LEFT JOIN setting_cancer_type ct ON s.cancer_type_id = ct.id
		LEFT JOIN setting_treatment_stage ts ON s.treatment_stage_id = ts.id
		WHERE s.id = ?`
	args := []interface{}{sampleID}
	if accessFilter, accessArgs := miniappEmployeePatientAccessFilter(db, employeeID, "p"); accessFilter != "" {
		query += " AND " + accessFilter
		args = append(args, accessArgs...)
	}
	var id int
	var sampleCode, status, patientName, patientCode, sampleType, cancerType, reportType, organization, treatmentStage, notes string
	var collectionDate, receiveDate sql.NullTime
	var reportID sql.NullInt64
	if err := db.QueryRow(query, args...).Scan(&id, &sampleCode, &status, &patientName, &patientCode,
		&sampleType, &cancerType, &reportType, &organization, &treatmentStage, &collectionDate, &receiveDate, &notes, &reportID); err != nil {
		c.JSON(consts.StatusNotFound, ApiResponse{Code: 404, Success: false, Message: "样本不存在或无权查看", Data: nil})
		return
	}
	data := utils.H{
		"id": id, "sample_code": sampleCode, "sample_status": status,
		"patient_name": patientName, "patient_code": patientCode,
		"sample_type": sampleType, "cancer_type": cancerType,
		"report_type": normalizeSampleReportType(reportType), "report_type_label": reportTypeFullLabel(reportType),
		"report_assay_label": reportTypeAssayLabel(reportType), "organization": organization,
		"treatment_stage": treatmentStage, "notes": notes, "has_report": reportID.Valid,
	}
	if collectionDate.Valid {
		data["collection_date"] = collectionDate.Time.Format("2006-01-02")
	}
	if receiveDate.Valid {
		data["receive_date"] = receiveDate.Time.Format("2006-01-02")
	}
	if reportID.Valid {
		data["report_id"] = reportID.Int64
	}
	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "获取成功", Data: data})
}

// HandleUniEmployeeDeleteSample 删除当前员工创建且尚未进入检测流程的样本，并回收编号。
func HandleUniEmployeeDeleteSample(c *app.RequestContext, db *sql.DB) {
	employeeID, ok := requireMiniappEmployee(c, db)
	if !ok {
		return
	}
	reusable := true
	var req struct {
		Reusable *bool `json:"reusable"`
	}
	if body, bodyErr := c.Body(); bodyErr == nil && len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "管码使用选项无效", Data: nil})
			return
		}
		if req.Reusable != nil {
			reusable = *req.Reusable
		}
	}
	sampleID, err := strconv.Atoi(c.Param("id"))
	if err != nil || sampleID <= 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "无效的样本ID", Data: nil})
		return
	}
	var sampleCode, status string
	var batchID int
	var reportCount int
	err = db.QueryRow(`SELECT s.sample_code, s.sample_status, COALESCE(s.batch_id, 0),
		(SELECT COUNT(*) FROM detect_report r WHERE r.sample_id = s.id)
		FROM detect_sample s WHERE s.id = ? AND s.collection_operator = ?`, sampleID, employeeID).
		Scan(&sampleCode, &status, &batchID, &reportCount)
	if err == sql.ErrNoRows {
		c.JSON(consts.StatusNotFound, ApiResponse{Code: 404, Success: false, Message: "样本不存在或不是由当前员工创建", Data: nil})
		return
	}
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "查询样本失败", Data: nil})
		return
	}
	if (status != "created" && status != "collected") || batchID > 0 || reportCount > 0 {
		c.JSON(consts.StatusConflict, ApiResponse{Code: 409, Success: false, Message: "该样本已进入检测或报告流程，不能删除", Data: nil})
		return
	}
	if len(sampleCode) < 5 {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "样本编号格式异常，不能回收", Data: nil})
		return
	}
	prefix := sampleCode[:len(sampleCode)-4]
	sequence, parseErr := strconv.Atoi(sampleCode[len(sampleCode)-4:])
	if parseErr != nil || sequence <= 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "样本编号格式异常，不能回收", Data: nil})
		return
	}
	tx, err := db.Begin()
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "删除失败", Data: nil})
		return
	}
	defer tx.Rollback()
	if _, err = tx.Exec("DELETE FROM detect_sample WHERE id = ? AND collection_operator = ?", sampleID, employeeID); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "删除样本失败", Data: nil})
		return
	}
	if err = recycleSampleCode(tx, sampleCode, prefix, sequence, employeeID, reusable); err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "编号回收失败", Data: nil})
		return
	}
	if err = tx.Commit(); err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "删除失败", Data: nil})
		return
	}
	message := "删除成功，管码已停用"
	if reusable {
		message = "删除成功，管码已进入号池"
	}
	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: message, Data: utils.H{"sample_code": sampleCode, "reusable": reusable}})
}

// HandleUniEmployeeReceiveSample 接收单个样本
func HandleUniEmployeeReceiveSample(c *app.RequestContext, db *sql.DB) {
	var req struct {
		SampleCode string `json:"sample_code"`
	}

	body, err := c.Body()
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "读取请求体失败",
			Data:    nil,
		})
		return
	}

	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "解析请求体失败",
			Data:    nil,
		})
		return
	}

	if req.SampleCode == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "样本编号不能为空",
			Data:    nil,
		})
		return
	}

	var currentStatus string
	err = db.QueryRow("SELECT sample_status FROM detect_sample WHERE sample_code = ?", req.SampleCode).Scan(&currentStatus)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(consts.StatusNotFound, ApiResponse{
				Code:    404,
				Success: false,
				Message: "样本不存在",
				Data:    nil,
			})
			return
		}
		log.Printf("Query sample status error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "查询样本失败",
			Data:    nil,
		})
		return
	}

	if currentStatus != "created" && currentStatus != "collected" {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "样本当前状态不可接收",
			Data:    utils.H{"sample_status": currentStatus},
		})
		return
	}

	receiveOperator := getMiniappEmployeeID(c, db)
	updateQuery := `UPDATE detect_sample SET sample_status = 'received', receive_date = NOW(), receive_operator = ?, sample_updated_at = NOW() WHERE sample_code = ? AND sample_status IN ('created', 'collected')`
	result, err := db.Exec(updateQuery, receiveOperator, req.SampleCode)
	if err != nil {
		log.Printf("Receive sample error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "接收失败",
			Data:    nil,
		})
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "样本接收失败，请刷新后重试",
			Data:    nil,
		})
		return
	}

	data := getSampleReceiveDetail(db, req.SampleCode)

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "接收成功",
		Data:    data,
	})
}

// HandleUniEmployeeBatchReceiveSamples 批量接收样本
func HandleUniEmployeeBatchReceiveSamples(c *app.RequestContext, db *sql.DB) {
	var req struct {
		SampleCodes []string `json:"sample_codes"`
		SampleIDs   []int    `json:"sample_ids"`
	}

	body, err := c.Body()
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "读取请求体失败",
			Data:    nil,
		})
		return
	}

	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "解析请求体失败",
			Data:    nil,
		})
		return
	}

	if len(req.SampleCodes) == 0 && len(req.SampleIDs) == 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请选择要接收的样本编号",
			Data:    nil,
		})
		return
	}

	receiveOperator := getMiniappEmployeeID(c, db)
	updateQuery := `UPDATE detect_sample SET sample_status = 'received', receive_date = NOW(), receive_operator = ?, sample_updated_at = NOW() WHERE sample_status IN ('created', 'collected') AND `
	args := []interface{}{receiveOperator}
	if len(req.SampleCodes) > 0 {
		updateQuery += "sample_code IN ("
		for i, sampleCode := range req.SampleCodes {
			if i > 0 {
				updateQuery += ","
			}
			updateQuery += "?"
			args = append(args, sampleCode)
		}
	} else {
		updateQuery += "id IN ("
		for i, id := range req.SampleIDs {
			if i > 0 {
				updateQuery += ","
			}
			updateQuery += "?"
			args = append(args, id)
		}
	}
	updateQuery += ")"

	result, err := db.Exec(updateQuery, args...)
	if err != nil {
		log.Printf("Batch receive samples error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "批量接收失败",
			Data:    nil,
		})
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "没有可接收的样本",
			Data:    nil,
		})
		return
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "批量接收成功",
		Data: utils.H{
			"count":        affected,
			"samples":      receivedSamplesForCodes(db, req.SampleCodes),
			"panel_groups": groupSamplesByPanel(receivedSamplesForCodes(db, req.SampleCodes)),
		},
	})
}
