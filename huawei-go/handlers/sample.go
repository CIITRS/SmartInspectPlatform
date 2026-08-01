package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"image/png"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// 样本管理相关处理函数
func parseSampleDateTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z07:00", "2006-01-02 15:04:05", "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported datetime format: %s", value)
}

func HandleGetSamples(c *app.RequestContext, db *sql.DB) {
	// 获取样本数据
	sampleData, err := getSampleData(c, db)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}
	// 返回样本列表
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取样本列表成功",
		Data:    sampleData,
	})
}

func HandleSampleBarcode(c *app.RequestContext, db *sql.DB) {
	sampleCode := strings.TrimSpace(c.Query("sample_code"))
	if sampleCode == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "样本编号不能为空", Data: nil})
		return
	}

	var exists bool
	if err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM detect_sample WHERE sample_code = ?)", sampleCode).Scan(&exists); err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "服务器内部错误", Data: nil})
		return
	}
	if !exists {
		c.JSON(consts.StatusNotFound, ApiResponse{Code: 404, Success: false, Message: "样本不存在", Data: nil})
		return
	}

	code, err := code128.Encode(sampleCode)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "样本编号无法生成Code128条形码", Data: nil})
		return
	}

	scaledCode, err := barcode.Scale(code, 520, 160)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "生成条形码失败", Data: nil})
		return
	}

	var buffer bytes.Buffer
	if err := png.Encode(&buffer, scaledCode); err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "生成条形码失败", Data: nil})
		return
	}

	c.Header("Cache-Control", "public, max-age=259200")
	c.Data(consts.StatusOK, "image/png", buffer.Bytes())
}

func currentUserID(c *app.RequestContext) (int, bool) {
	userID, exists := c.Get(UserIDKey)
	if !exists {
		return 0, false
	}
	switch v := userID.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

func getEmployeeIDForSampleCode(db *sql.DB, userID int) (string, error) {
	var employeeID string
	if err := db.QueryRow("SELECT employee_id FROM base_manage_user WHERE id = ?", userID).Scan(&employeeID); err != nil {
		return "", err
	}
	employeeID = strings.TrimSpace(employeeID)
	if employeeID == "" {
		return "", fmt.Errorf("当前用户未配置工号，无法自动生成样本编号")
	}
	return employeeID, nil
}

func sampleCodePrefix(employeeID string) string {
	return fmt.Sprintf("%s%s", time.Now().Format("06"), employeeID)
}

func nextSampleSequence(db *sql.DB, prefix string) int {
	var recycledSequence int
	if err := db.QueryRow(`SELECT sequence_no FROM detect_sample_code_pool
		WHERE prefix = ? AND status = 'available'
			AND NOT EXISTS (SELECT 1 FROM detect_sample s WHERE s.sample_code = detect_sample_code_pool.sample_code)
		ORDER BY sequence_no ASC LIMIT 1`, prefix).Scan(&recycledSequence); err == nil && recycledSequence > 0 {
		return recycledSequence
	}
	return nextFreshSampleSequence(db, prefix)
}

func nextFreshSampleSequence(db *sql.DB, prefix string) int {
	var maxSeq int
	if err := db.QueryRow(`SELECT GREATEST(
		COALESCE((SELECT MAX(CAST(SUBSTRING(sample_code, ?, 4) AS UNSIGNED)) FROM detect_sample WHERE sample_code LIKE ?), 0),
		COALESCE((SELECT MAX(sequence_no) FROM detect_sample_code_pool WHERE prefix = ?), 0))`,
		len(prefix)+1, prefix+"____", prefix).Scan(&maxSeq); err != nil {
		return 1
	}
	return maxSeq + 1
}

type sampleCodeExecer interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}

func markSampleCodeUsed(execer sampleCodeExecer, sampleCode string) {
	_, _ = execer.Exec(`UPDATE detect_sample_code_pool
		SET status = 'used', used_at = NOW() WHERE sample_code = ?`, sampleCode)
}

func recycleSampleCode(execer sampleCodeExecer, sampleCode, prefix string, sequence, employeeID int, reusable bool) error {
	status := "retired"
	if reusable {
		status = "available"
	}
	_, err := execer.Exec(`INSERT INTO detect_sample_code_pool
		(sample_code, prefix, sequence_no, recycled_by, status, created_at, used_at, updated_at)
		VALUES (?, ?, ?, ?, ?, NOW(), NULL, NOW())
		ON DUPLICATE KEY UPDATE prefix = VALUES(prefix), sequence_no = VALUES(sequence_no),
			recycled_by = VALUES(recycled_by), status = VALUES(status), used_at = NULL, updated_at = NOW()`,
		sampleCode, prefix, sequence, employeeID, status)
	return err
}

func buildSampleCode(prefix string, sequence int) string {
	return fmt.Sprintf("%s%04d", prefix, sequence)
}

func sampleCodeExists(db *sql.DB, sampleCode string) (bool, error) {
	var exists bool
	err := db.QueryRow(`SELECT EXISTS(
		SELECT 1 FROM detect_sample WHERE sample_code = ?
		UNION ALL
		SELECT 1 FROM detect_sample_code_pool WHERE sample_code = ? AND status = 'retired'
	)`, sampleCode, sampleCode).Scan(&exists)
	return exists, err
}

func normalizeSampleReportType(reportType string) string {
	return normalizeAssignedReportType(reportType)
}

func getSamplePanels(db *sql.DB, panelIDs string) []utils.H {
	panelIDs = strings.TrimSpace(panelIDs)
	if panelIDs == "" {
		return []utils.H{}
	}
	rows, err := db.Query(`SELECT id, panel_name, panel_code FROM setting_panel WHERE FIND_IN_SET(id, ?) AND is_active = 1 ORDER BY id`, panelIDs)
	if err != nil {
		log.Printf("Failed to query sample panels: %v", err)
		return []utils.H{}
	}
	defer rows.Close()

	panels := []utils.H{}
	for rows.Next() {
		var id int
		var name, code string
		if err := rows.Scan(&id, &name, &code); err == nil {
			panels = append(panels, utils.H{
				"id":         id,
				"panel_name": name,
				"panel_code": code,
			})
		}
	}
	return panels
}

func panelDisplayList(panels []utils.H) string {
	values := make([]string, 0, len(panels))
	for _, panel := range panels {
		code, _ := panel["panel_code"].(string)
		name, _ := panel["panel_name"].(string)
		if code != "" {
			values = append(values, code)
		} else if name != "" {
			values = append(values, name)
		}
	}
	return strings.Join(values, "，")
}

func getSampleReceiveDetail(db *sql.DB, sampleCode string) utils.H {
	var id int
	var patientName, cancerTypeName, sampleTypeName, reportType, panelIDs, sampleStatus sql.NullString
	var receiveDate sql.NullTime
	err := db.QueryRow(`SELECT s.id, COALESCE(p.name, ''), COALESCE(ct.name, ''),
		COALESCE(st.name, ''), COALESCE(s.report_type, ''), COALESCE(ct.panel_ids, ''),
		COALESCE(s.sample_status, ''), s.receive_date
		FROM detect_sample s
		LEFT JOIN detect_patient p ON s.patient_id = p.id
		LEFT JOIN setting_cancer_type ct ON s.cancer_type_id = ct.id
		LEFT JOIN setting_sample_type st ON s.sample_type_id = st.id
		WHERE s.sample_code = ?`, sampleCode).Scan(&id, &patientName, &cancerTypeName, &sampleTypeName, &reportType, &panelIDs, &sampleStatus, &receiveDate)
	if err != nil {
		log.Printf("Failed to query sample receive detail for %s: %v", sampleCode, err)
		return utils.H{"sample_code": sampleCode, "sample_status": "received"}
	}

	panels := getSamplePanels(db, panelIDs.String)
	data := utils.H{
		"id":               id,
		"sample_code":      sampleCode,
		"sample_status":    sampleStatus.String,
		"patient_name":     patientName.String,
		"cancer_type_name": cancerTypeName.String,
		"sample_type_name": sampleTypeName.String,
		"report_type":      reportType.String,
		"panels":           panels,
		"panel_summary":    panelDisplayList(panels),
	}
	if receiveDate.Valid {
		data["receive_date"] = receiveDate.Time.Format("2006-01-02 15:04")
	}
	return data
}

func HandleSampleReceivePreview(c *app.RequestContext, db *sql.DB) {
	sampleCode := strings.TrimSpace(c.Query("sample_code"))
	if sampleCode == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "样本编号不能为空", Data: nil})
		return
	}
	var exists bool
	if err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM detect_sample WHERE sample_code = ?)", sampleCode).Scan(&exists); err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "服务器内部错误", Data: nil})
		return
	}
	if !exists {
		c.JSON(consts.StatusNotFound, ApiResponse{Code: 404, Success: false, Message: "样本不存在", Data: nil})
		return
	}
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取样本接收信息成功",
		Data:    getSampleReceiveDetail(db, sampleCode),
	})
}

func groupSamplesByPanel(samples []utils.H) []utils.H {
	grouped := map[string][]string{}
	order := []string{}
	for _, sample := range samples {
		sampleCode, _ := sample["sample_code"].(string)
		panels, _ := sample["panels"].([]utils.H)
		for _, panel := range panels {
			key, _ := panel["panel_code"].(string)
			if key == "" {
				key, _ = panel["panel_name"].(string)
			}
			if key == "" {
				continue
			}
			if _, exists := grouped[key]; !exists {
				order = append(order, key)
			}
			grouped[key] = append(grouped[key], sampleCode)
		}
	}

	result := make([]utils.H, 0, len(order))
	for _, key := range order {
		result = append(result, utils.H{
			"panel":        key,
			"sample_codes": grouped[key],
		})
	}
	return result
}

func receivedSamplesForCodes(db *sql.DB, sampleCodes []string) []utils.H {
	samples := make([]utils.H, 0, len(sampleCodes))
	seen := map[string]bool{}
	for _, sampleCode := range sampleCodes {
		sampleCode = strings.TrimSpace(sampleCode)
		if sampleCode == "" || seen[sampleCode] {
			continue
		}
		seen[sampleCode] = true
		samples = append(samples, getSampleReceiveDetail(db, sampleCode))
	}
	return samples
}

func HandleCreateSample(c *app.RequestContext, db *sql.DB) {
	// 解析请求体
	var req struct {
		PatientID            int       `json:"patient_id"`
		PatientCode          string    `json:"patient_code"`
		PatientIDCard        string    `json:"patient_id_card"`
		SampleTypeID         int       `json:"sample_type_id" binding:"required"`
		CancerTypeID         int       `json:"cancer_type_id" binding:"required"`
		TreatmentStageID     int       `json:"treatment_stage_id" binding:"required"`
		CollectionDate       time.Time `json:"collection_date" binding:"required"`
		Notes                string    `json:"notes"`
		SampleCode           string    `json:"sample_code"`
		ReportType           string    `json:"report_type"`
		Status               string    `json:"status"`
		Organization         string    `json:"organization"`
		ServiceMode          string    `json:"service_mode"`
		SalePackageID        int       `json:"sale_package_id"`
		ReturnExpressCompany string    `json:"return_express_company"`
		ReturnTrackingNumber string    `json:"return_tracking_number"`
	}
	if err := c.Bind(&req); err != nil {
		log.Printf("Failed to bind detect_sample creation request: %v", err)
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请求参数错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}
	if req.PatientID == 0 && req.PatientCode != "" {
		if err := db.QueryRow("SELECT id FROM detect_patient WHERE patient_code = ? AND is_active = 1", req.PatientCode).Scan(&req.PatientID); err != nil {
			c.JSON(consts.StatusBadRequest, ApiResponse{
				Code:    400,
				Success: false,
				Message: "患者不存在",
				Data:    nil,
			})
			return
		}
	}
	if req.PatientID == 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "患者编号不能为空",
			Data:    nil,
		})
		return
	}

	// 从上下文获取当前用户ID
	userID, exists := currentUserID(c)
	if !exists {
		c.JSON(consts.StatusUnauthorized, ApiResponse{
			Code:    401,
			Success: false,
			Message: "未认证",
			Data:    nil,
		})
		return
	}
	patientQuery := "SELECT COUNT(*) FROM detect_patient WHERE id = ? AND is_active = 1"
	patientArgs := []interface{}{req.PatientID}
	if accessFilter, accessArgs := patientAccessFilterForUser(db, userID, ""); accessFilter != "" {
		patientQuery += " AND " + accessFilter
		patientArgs = append(patientArgs, accessArgs...)
	}
	var patientCount int
	if err := db.QueryRow(patientQuery, patientArgs...).Scan(&patientCount); err != nil || patientCount == 0 {
		c.JSON(consts.StatusForbidden, ApiResponse{Code: 403, Success: false, Message: "无权操作该患者", Data: nil})
		return
	}

	// 生成样本编码
	var detect_sampleCode string
	if req.SampleCode != "" {
		detect_sampleCode = strings.TrimSpace(req.SampleCode)
		exists, err := sampleCodeExists(db, detect_sampleCode)
		if err != nil {
			log.Printf("Failed to check sample code: %v", err)
			c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "服务器内部错误", Data: nil})
			return
		}
		if exists {
			c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "样本编号已存在，请更换后4位序号", Data: nil})
			return
		}
	} else {
		employeeID, err := getEmployeeIDForSampleCode(db, userID)
		if err != nil {
			log.Printf("Failed to query employee_id for sample code: %v", err)
			c.JSON(consts.StatusBadRequest, ApiResponse{
				Code:    400,
				Success: false,
				Message: err.Error(),
				Data:    nil,
			})
			return
		}
		prefix := sampleCodePrefix(employeeID)
		detect_sampleCode = buildSampleCode(prefix, nextSampleSequence(db, prefix))
	}
	reportType := normalizeSampleReportType(req.ReportType)
	req.Organization = strings.TrimSpace(req.Organization)
	if req.Organization == "" {
		req.Organization = "个人送检"
	}
	req.ServiceMode = strings.ToLower(strings.TrimSpace(req.ServiceMode))
	if req.ServiceMode == "" {
		req.ServiceMode = "single"
	}
	if req.ServiceMode == "package" && req.SalePackageID <= 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "请选择检测套餐", Data: nil})
		return
	}

	// 插入样本到数据库
	result, err := db.Exec(`INSERT INTO detect_sample (sample_code, patient_id, sample_type_id, cancer_type_id, treatment_stage_id, collection_date, collection_operator, sample_status, report_type, notes, organization, service_mode, sale_package_id, sample_created_at, sample_updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, 'created', ?, ?, ?, ?, ?, NOW(), NOW())`, detect_sampleCode, req.PatientID, req.SampleTypeID, req.CancerTypeID, req.TreatmentStageID, req.CollectionDate, userID, reportType, req.Notes, req.Organization, req.ServiceMode, nullablePositiveInt(req.SalePackageID))
	if err != nil {
		log.Printf("Failed to create detect_sample: %v", err)
		message := "服务器内部错误"
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			message = "样本编号已存在，请更换后4位序号"
		}
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: message,
			Data:    utils.H{"error": err.Error()},
		})
		return
	}
	markSampleCodeUsed(db, detect_sampleCode)

	// 前端新增样本后的回写逻辑：查询 detect_batch_sample 是否存在该样本
	// 查询患者姓名
	var patientName string
	err = db.QueryRow("SELECT name FROM detect_patient WHERE id = ?", req.PatientID).Scan(&patientName)
	if err == nil {
		// 更新 detect_batch_sample 表中匹配的记录
		_, err = db.Exec("UPDATE detect_batch_sample SET patient_id = ?, patient_name = ?, match_status = 1 WHERE sample_code = ?",
			req.PatientID, patientName, detect_sampleCode)
		if err != nil {
			log.Printf("Failed to update detect_batch_sample: %v", err)
		}
	}

	// 获取插入的样本ID
	id, err := result.LastInsertId()
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
	if tracking := strings.TrimSpace(req.ReturnTrackingNumber); tracking != "" {
		if _, expressErr := db.Exec(`INSERT INTO detect_sample_express
			(sample_id, sample_code, direction, express_type, express_company, tracking_number, status, created_at, updated_at)
			VALUES (?, ?, 'inbound', 'auto', ?, ?, 'in_transit', NOW(), NOW())`, id, detect_sampleCode, strings.TrimSpace(req.ReturnExpressCompany), tracking); expressErr != nil {
			log.Printf("Failed to save return express for sample %s: %v", detect_sampleCode, expressErr)
		}
	}

	// 查询刚创建的样本详细信息
	var detect_sampleCodeStr, status, notes, organization, detect_patientIDCard, detect_patientPhone string
	var detect_patientID, detect_sampleTypeID, cancerTypeID, treatmentStageID, collectionOperator, receiveOperator int
	var collectionDate, createdAt, updatedAt time.Time
	var receiveDate sql.NullTime
	var detect_sampleTypeName, cancerTypeName, treatmentStageName, detect_patientName, detect_patientCode string

	err = db.QueryRow(`SELECT s.sample_code, s.patient_id, s.collection_date, s.collection_operator, s.receive_date, s.receive_operator, s.sample_status, s.notes, s.organization, s.sample_created_at, s.sample_updated_at, s.sample_type_id, st.name as setting_sample_type_name, s.cancer_type_id, ct.name as setting_cancer_type_name, s.treatment_stage_id, ts.name as setting_treatment_stage_name, COALESCE(p.name, '') as patient_name, COALESCE(p.patient_code, ''), COALESCE(p.id_card, ''), COALESCE(p.phone, '') FROM detect_sample s LEFT JOIN setting_sample_type st ON s.sample_type_id = st.id LEFT JOIN setting_cancer_type ct ON s.cancer_type_id = ct.id LEFT JOIN setting_treatment_stage ts ON s.treatment_stage_id = ts.id LEFT JOIN detect_patient p ON s.patient_id = p.id WHERE s.id = ?`, id).Scan(&detect_sampleCodeStr, &detect_patientID, &collectionDate, &collectionOperator, &receiveDate, &receiveOperator, &status, &notes, &organization, &createdAt, &updatedAt, &detect_sampleTypeID, &detect_sampleTypeName, &cancerTypeID, &cancerTypeName, &treatmentStageID, &treatmentStageName, &detect_patientName, &detect_patientCode, &detect_patientIDCard, &detect_patientPhone)

	if err != nil {
		log.Printf("Failed to query created detect_sample: %v", err)
		// 即使查询失败，也返回创建成功，因为样本已经插入数据库
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "创建样本成功",
			Data: utils.H{
				"id":                 id,
				"detect_sample_code": detect_sampleCode,
			},
		})
		return
	}

	// 构建返回的样本信息
	detect_sample := utils.H{
		"id":                   id,
		"sample_code":          detect_sampleCodeStr,
		"patient_id":           detect_patientID,
		"patient_name":         detect_patientName,
		"patient_code":         detect_patientCode,
		"id_card":              detect_patientIDCard,
		"phone":                detect_patientPhone,
		"collection_date":      collectionDate.Format("2006-01-02T15:04:05+08:00"),
		"collection_operator":  collectionOperator,
		"status":               status,
		"sample_type_id":       detect_sampleTypeID,
		"sample_type_name":     detect_sampleTypeName,
		"cancer_type_id":       cancerTypeID,
		"cancer_type_name":     cancerTypeName,
		"treatment_stage_id":   treatmentStageID,
		"treatment_stage_name": treatmentStageName,
		"created_at":           createdAt.Format("2006-01-02T15:04:05+08:00"),
		"updated_at":           updatedAt.Format("2006-01-02T15:04:05+08:00"),
		"notes":                notes,
		"organization":         organization,
	}

	// 处理可选字段
	if receiveDate.Valid {
		detect_sample["receive_date"] = receiveDate.Time.Format("2006-01-02T15:04:05+08:00")
	}
	if receiveOperator > 0 {
		detect_sample["receive_operator"] = receiveOperator
	}

	// 返回创建的样本信息
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "创建样本成功",
		Data:    detect_sample,
	})
}

func HandleAllocateSamples(c *app.RequestContext, db *sql.DB) {
	var req struct {
		PatientIDs           []int     `json:"patient_ids"`
		SampleTypeID         int       `json:"sample_type_id" binding:"required"`
		CancerTypeID         int       `json:"cancer_type_id" binding:"required"`
		TreatmentStageID     int       `json:"treatment_stage_id" binding:"required"`
		ReportType           string    `json:"report_type"`
		StartSequence        int       `json:"start_sequence"`
		ManualSuffix         string    `json:"manual_suffix"`
		CollectionDate       time.Time `json:"collection_date"`
		Notes                string    `json:"notes"`
		Organization         string    `json:"organization"`
		UseCurrentPatients   bool      `json:"use_current_patients"`
		ServiceMode          string    `json:"service_mode"`
		SalePackageID        int       `json:"sale_package_id"`
		ReturnExpressCompany string    `json:"return_express_company"`
		ReturnTrackingNumber string    `json:"return_tracking_number"`
	}
	if err := c.Bind(&req); err != nil {
		log.Printf("Failed to bind sample allocation request: %v", err)
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "请求参数错误", Data: utils.H{"error": err.Error()}})
		return
	}
	if len(req.PatientIDs) == 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "请选择患者", Data: nil})
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
	if req.ServiceMode == "package" && req.SalePackageID <= 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "请选择检测套餐", Data: nil})
		return
	}

	userID, exists := currentUserID(c)
	if !exists {
		c.JSON(consts.StatusUnauthorized, ApiResponse{Code: 401, Success: false, Message: "未认证", Data: nil})
		return
	}
	employeeID, err := getEmployeeIDForSampleCode(db, userID)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: err.Error(), Data: nil})
		return
	}

	prefix := sampleCodePrefix(employeeID)
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
	if req.CollectionDate.IsZero() {
		req.CollectionDate = time.Now()
	}
	reportType := normalizeSampleReportType(req.ReportType)

	codes := make([]string, 0, len(req.PatientIDs))
	for i := range req.PatientIDs {
		codes = append(codes, buildSampleCode(prefix, startSequence+i))
	}
	for _, code := range codes {
		exists, err := sampleCodeExists(db, code)
		if err != nil {
			log.Printf("Failed to check allocated sample code %s: %v", code, err)
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
	patientAccessFilter, patientAccessArgs := patientAccessFilterForUser(db, userID, "")
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
		result, err := tx.Exec(`INSERT INTO detect_sample
			(sample_code, patient_id, sample_type_id, cancer_type_id, treatment_stage_id, collection_date, collection_operator, sample_status, report_type, notes, organization, service_mode, sale_package_id, sample_created_at, sample_updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, 'created', ?, ?, ?, ?, ?, NOW(), NOW())`,
			code, patientID, req.SampleTypeID, req.CancerTypeID, req.TreatmentStageID, req.CollectionDate, userID, reportType, req.Notes, req.Organization,
			req.ServiceMode, nullablePositiveInt(req.SalePackageID))
		if err != nil {
			log.Printf("Failed to allocate sample %s: %v", code, err)
			message := "新增样本失败"
			if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
				message = fmt.Sprintf("样本编号 %s 已存在，请调整起始序号", code)
			}
			c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: message, Data: nil})
			return
		}
		id, _ := result.LastInsertId()
		if len(req.PatientIDs) == 1 && strings.TrimSpace(req.ReturnTrackingNumber) != "" {
			if _, err := tx.Exec(`INSERT INTO detect_sample_express
				(sample_id, sample_code, direction, express_type, express_company, tracking_number, status, created_at, updated_at)
				VALUES (?, ?, 'inbound', 'auto', ?, ?, 'in_transit', NOW(), NOW())`,
				id, code, strings.TrimSpace(req.ReturnExpressCompany), strings.TrimSpace(req.ReturnTrackingNumber)); err != nil {
				c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "保存回寄快递单号失败", Data: nil})
				return
			}
		}
		markSampleCodeUsed(tx, code)
		created = append(created, utils.H{
			"id":           id,
			"sample_code":  code,
			"patient_id":   patientID,
			"patient_name": patientName,
		})
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

func HandleSampleReceived(c *app.RequestContext, db *sql.DB) {
	// 解析请求体
	var req struct {
		SampleCode string `json:"sample_code" binding:"required"`
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

	// 从上下文获取当前用户ID作为接收人员
	var receiveOperator int
	if userID, exists := c.Get(UserIDKey); exists {
		receiveOperator = userID.(int)
	}

	// 更新样本状态为已接收，并记录接收人员和接收时间
	result, err := db.Exec(`UPDATE detect_sample SET sample_status = 'received', receive_date = NOW(), receive_operator = ?, sample_updated_at = NOW() WHERE sample_code = ? AND sample_status != 'received'`, receiveOperator, req.SampleCode)
	if err != nil {
		log.Printf("Failed to update detect_sample status: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 检查是否有行被更新
	affected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Failed to get rows affected: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	if affected == 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "样本状态已经是已接收或者样本不存在",
			Data:    nil,
		})
		return
	}

	// 返回成功响应
	data := getSampleReceiveDetail(db, req.SampleCode)
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "样本接收成功",
		Data:    data,
	})
}

func HandleUpdateSample(c *app.RequestContext, db *sql.DB) {
	// 获取样本ID
	id := c.Param("id")
	if id == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请求参数错误，缺少样本ID",
			Data:    nil,
		})
		return
	}

	// 解析请求体
	var req struct {
		SampleCode       string `json:"sample_code"`
		Status           string `json:"status"`
		SampleTypeID     int    `json:"sample_type_id"`
		CancerTypeID     int    `json:"cancer_type_id"`
		TreatmentStageID int    `json:"treatment_stage_id"`
		Notes            string `json:"notes"`
		Organization     string `json:"organization"`
		ReceiveDate      string `json:"receive_date"`
	}
	if err := c.Bind(&req); err != nil {
		log.Printf("Failed to bind detect_sample update request: %v", err)
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请求参数错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 构建动态更新查询，只更新非空字段
	var setClauses []string
	var args []interface{}

	// 只更新非空的字段
	if req.SampleCode != "" {
		setClauses = append(setClauses, "sample_code = ?")
		args = append(args, req.SampleCode)
	}
	if req.Status != "" {
		setClauses = append(setClauses, "sample_status = ?")
		args = append(args, req.Status)
	}
	if req.SampleTypeID > 0 {
		setClauses = append(setClauses, "sample_type_id = ?")
		args = append(args, req.SampleTypeID)
	}
	if req.CancerTypeID > 0 {
		setClauses = append(setClauses, "cancer_type_id = ?")
		args = append(args, req.CancerTypeID)
	}
	if req.TreatmentStageID > 0 {
		setClauses = append(setClauses, "treatment_stage_id = ?")
		args = append(args, req.TreatmentStageID)
	}
	if req.Notes != "" {
		setClauses = append(setClauses, "notes = ?")
		args = append(args, req.Notes)
	}
	if req.Organization != "" {
		setClauses = append(setClauses, "organization = ?")
		args = append(args, req.Organization)
	}
	if strings.TrimSpace(req.ReceiveDate) != "" {
		receiveDate, err := parseSampleDateTime(req.ReceiveDate)
		if err != nil {
			c.JSON(consts.StatusBadRequest, ApiResponse{
				Code:    400,
				Success: false,
				Message: "接收时间格式错误",
				Data:    utils.H{"error": err.Error()},
			})
			return
		}
		setClauses = append(setClauses, "receive_date = ?")
		args = append(args, receiveDate)
	}

	// 添加更新时间
	setClauses = append(setClauses, "sample_updated_at = NOW()")

	// 构建完整的查询
	query := `UPDATE detect_sample SET ` +
		" " +
		strings.Join(setClauses, ", ") +
		" WHERE id = ?"

	// 添加样本ID到参数
	args = append(args, id)

	// 执行更新
	_, err := db.Exec(query, args...)
	if err != nil {
		log.Printf("Failed to update detect_sample: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	if req.CancerTypeID > 0 {
		if _, err := db.Exec(`UPDATE detect_batch_sample bs
			JOIN detect_sample s ON bs.sample_code = s.sample_code
			SET bs.cancer_type_id = ?
			WHERE s.id = ?`, req.CancerTypeID, id); err != nil {
			log.Printf("Failed to sync detect_batch_sample cancer_type_id: %v", err)
		}
		if _, err := db.Exec(`DELETE FROM detect_sample_panel_match
			WHERE sample_code IN (SELECT sample_code FROM detect_sample WHERE id = ?)`, id); err != nil {
			log.Printf("Failed to invalidate sample panel match cache: %v", err)
		}
	}

	// 查询更新后的样本信息
	var detect_sampleCode, status, notes, organization, detect_patientIDCard, detect_patientPhone string
	var detect_patientID, detect_sampleTypeID, cancerTypeID, treatmentStageID, collectionOperator, receiveOperator int
	var collectionDate, createdAt, updatedAt time.Time
	var receiveDate sql.NullTime
	var detect_sampleTypeName, cancerTypeName, treatmentStageName, detect_patientName, detect_patientCode string

	err = db.QueryRow(`SELECT s.sample_code, s.patient_id, s.collection_date, s.collection_operator, 
		s.receive_date, s.receive_operator, s.sample_status, s.notes, s.organization, s.sample_created_at, s.sample_updated_at, 
		s.sample_type_id as sample_type_id, st.name as sample_type_name, 
		s.cancer_type_id as cancer_type_id, ct.name as cancer_type_name, 
		s.treatment_stage_id as treatment_stage_id, ts.name as treatment_stage_name, 
		COALESCE(p.name, '') as patient_name, COALESCE(p.patient_code, ''), COALESCE(p.id_card, ''), COALESCE(p.phone, '')
		FROM detect_sample s 
		LEFT JOIN setting_sample_type st ON s.sample_type_id = st.id 
		LEFT JOIN setting_cancer_type ct ON s.cancer_type_id = ct.id 
		LEFT JOIN setting_treatment_stage ts ON s.treatment_stage_id = ts.id 
		LEFT JOIN detect_patient p ON s.patient_id = p.id 
		WHERE s.id = ?`, id).Scan(
		&detect_sampleCode, &detect_patientID, &collectionDate, &collectionOperator,
		&receiveDate, &receiveOperator, &status, &notes, &organization, &createdAt, &updatedAt,
		&detect_sampleTypeID, &detect_sampleTypeName, &cancerTypeID, &cancerTypeName,
		&treatmentStageID, &treatmentStageName, &detect_patientName, &detect_patientCode, &detect_patientIDCard, &detect_patientPhone)

	if err != nil {
		log.Printf("Failed to query updated detect_sample: %v", err)
		// 即使查询失败，也返回更新成功，因为样本已经更新到数据库
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "样本编辑成功",
			Data:    nil,
		})
		return
	}

	// 构建返回的样本信息
	detect_sample := utils.H{
		"id":                   id,
		"sample_code":          detect_sampleCode,
		"patient_id":           detect_patientID,
		"patient_name":         detect_patientName,
		"patient_code":         detect_patientCode,
		"id_card":              detect_patientIDCard,
		"phone":                detect_patientPhone,
		"collection_date":      collectionDate.Format("2006-01-02T15:04:05+08:00"),
		"collection_operator":  collectionOperator,
		"status":               status,
		"sample_type_id":       detect_sampleTypeID,
		"sample_type_name":     detect_sampleTypeName,
		"cancer_type_id":       cancerTypeID,
		"cancer_type_name":     cancerTypeName,
		"treatment_stage_id":   treatmentStageID,
		"treatment_stage_name": treatmentStageName,
		"created_at":           createdAt.Format("2006-01-02T15:04:05+08:00"),
		"updated_at":           updatedAt.Format("2006-01-02T15:04:05+08:00"),
		"notes":                notes,
		"organization":         organization,
	}

	// 处理可选字段
	if receiveDate.Valid {
		detect_sample["receive_date"] = receiveDate.Time.Format("2006-01-02T15:04:05+08:00")
	}
	if receiveOperator > 0 {
		detect_sample["receive_operator"] = receiveOperator
	}

	// 返回更新后的样本信息
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "样本编辑成功",
		Data:    detect_sample,
	})
}

func HandleBatchReceiveSamples(c *app.RequestContext, db *sql.DB) {
	// 解析请求体
	var req struct {
		SampleCodes []string `json:"sample_codes" binding:"required"`
	}
	if err := c.Bind(&req); err != nil {
		log.Printf("Failed to bind batch receive request: %v", err)
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请求参数错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 从上下文获取当前用户ID
	userID, exists := c.Get(UserIDKey)
	if !exists {
		c.JSON(consts.StatusUnauthorized, ApiResponse{
			Code:    401,
			Success: false,
			Message: "未认证",
			Data:    nil,
		})
		return
	}

	// 批量更新样本状态
	updatedCount := 0
	receivedSamples := []utils.H{}
	for _, detect_sampleCode := range req.SampleCodes {
		// 更新样本状态为已接收
		result, err := db.Exec(`UPDATE detect_sample SET sample_status = 'received', receive_date = NOW(), receive_operator = ?, sample_updated_at = NOW() WHERE sample_code = ? AND sample_status != 'received'`,
			userID, detect_sampleCode)
		if err != nil {
			log.Printf("Failed to update detect_sample %s: %v", detect_sampleCode, err)
			continue
		}
		affected, _ := result.RowsAffected()
		if affected > 0 {
			updatedCount++
			receivedSamples = append(receivedSamples, getSampleReceiveDetail(db, detect_sampleCode))
		}
	}

	// 返回批量更新结果
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "批量接收成功",
		Data: utils.H{
			"total":        len(req.SampleCodes),
			"updated":      updatedCount,
			"samples":      receivedSamples,
			"panel_groups": groupSamplesByPanel(receivedSamples),
		},
	})
}

func mysqlTableExists(tx *sql.Tx, tableName string) bool {
	tableName = strings.TrimSpace(tableName)
	if tableName == "" {
		return false
	}
	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?`, tableName).Scan(&count); err != nil {
		log.Printf("Failed to inspect table %s before sample delete cleanup: %v", tableName, err)
		return false
	}
	return count > 0
}

func HandleDeleteSample(c *app.RequestContext, db *sql.DB) {
	// 获取样本ID
	id := c.Param("id")
	if id == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请求参数错误，缺少样本ID",
			Data:    nil,
		})
		return
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
	defer tx.Rollback()

	var sampleCode string
	var batchID sql.NullInt64
	if err := tx.QueryRow(`SELECT sample_code, batch_id FROM detect_sample WHERE id = ?`, id).Scan(&sampleCode, &batchID); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(consts.StatusNotFound, ApiResponse{
				Code:    404,
				Success: false,
				Message: "样本不存在或已删除",
				Data:    nil,
			})
			return
		}
		log.Printf("Failed to query detect_sample before delete: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "查询样本失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	reportIDs := []int{}
	reportRows, err := tx.Query(`SELECT id FROM detect_report WHERE sample_id = ? OR parent_report_id IN (SELECT id FROM detect_report WHERE sample_id = ?)`, id, id)
	if err != nil {
		log.Printf("Failed to query reports before sample delete: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "查询关联报告失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}
	for reportRows.Next() {
		var reportID int
		if err := reportRows.Scan(&reportID); err == nil {
			reportIDs = append(reportIDs, reportID)
		}
	}
	reportRows.Close()
	if err := reportRows.Err(); err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "读取关联报告失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	if len(reportIDs) > 0 {
		placeholders := make([]string, 0, len(reportIDs))
		args := make([]interface{}, 0, len(reportIDs))
		for _, reportID := range reportIDs {
			placeholders = append(placeholders, "?")
			args = append(args, reportID)
		}
		if mysqlTableExists(tx, "detect_report_change_log") {
			if _, err := tx.Exec(`DELETE FROM detect_report_change_log WHERE report_id IN (`+strings.Join(placeholders, ",")+`)`, args...); err != nil {
				log.Printf("Failed to delete report change logs before sample delete: %v", err)
				c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "删除报告变更记录失败", Data: utils.H{"error": err.Error()}})
				return
			}
		}
		if _, err := tx.Exec(`DELETE FROM detect_report WHERE parent_report_id IN (`+strings.Join(placeholders, ",")+`)`, args...); err != nil {
			log.Printf("Failed to delete child reports before sample delete: %v", err)
			c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "删除子报告失败", Data: utils.H{"error": err.Error()}})
			return
		}
		if _, err := tx.Exec(`DELETE FROM detect_report WHERE id IN (`+strings.Join(placeholders, ",")+`)`, args...); err != nil {
			log.Printf("Failed to delete reports before sample delete: %v", err)
			c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "删除关联报告失败", Data: utils.H{"error": err.Error()}})
			return
		}
	}

	cleanupStatements := []struct {
		query string
		args  []interface{}
		name  string
		table string
	}{
		{`DELETE FROM result WHERE detect_sample_id = ?`, []interface{}{id}, "结果记录", "result"},
		{`DELETE FROM detect_sample_express WHERE sample_id = ? OR sample_code = ?`, []interface{}{id, sampleCode}, "样本快递记录", "detect_sample_express"},
		{`DELETE FROM detect_sample_panel_match WHERE sample_code = ?`, []interface{}{sampleCode}, "样本Panel匹配缓存", "detect_sample_panel_match"},
		{`DELETE FROM detect_batch_sample WHERE sample_code = ?`, []interface{}{sampleCode}, "批次样本关联", "detect_batch_sample"},
		{`DELETE FROM detect_batch_platform_data WHERE sample_code = ?`, []interface{}{sampleCode}, "批次平台数据", "detect_batch_platform_data"},
	}
	for _, statement := range cleanupStatements {
		if !mysqlTableExists(tx, statement.table) {
			continue
		}
		if _, err := tx.Exec(statement.query, statement.args...); err != nil {
			log.Printf("Failed to delete %s before sample delete: %v", statement.name, err)
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code:    500,
				Success: false,
				Message: "删除" + statement.name + "失败",
				Data:    utils.H{"error": err.Error()},
			})
			return
		}
	}

	// 删除样本
	_, err = tx.Exec(`DELETE FROM detect_sample WHERE id = ?`, id)
	if err != nil {
		log.Printf("Failed to delete detect_sample: %v", err)
		tx.Rollback()
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	if batchID.Valid {
		if _, err := tx.Exec(`UPDATE detect_batch SET sample_count = GREATEST(sample_count - 1, 0), updated_at = NOW() WHERE id = ?`, batchID.Int64); err != nil {
			log.Printf("Failed to update batch sample count after sample delete: %v", err)
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code:    500,
				Success: false,
				Message: "更新批次数量失败",
				Data:    utils.H{"error": err.Error()},
			})
			return
		}
	}

	// 提交事务
	if err = tx.Commit(); err != nil {
		log.Printf("Failed to commit transaction: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 返回删除成功
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "样本删除成功",
		Data:    nil,
	})
}

func HandleUpdateGeneData(c *app.RequestContext, db *sql.DB) {
	// 获取样本ID
	id := c.Param("id")
	if id == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请求参数错误，缺少样本ID",
			Data:    nil,
		})
		return
	}

	// 解析请求体
	var req struct {
		GeneData map[string]float64 `json:"gene_data" binding:"required"`
	}
	if err := c.Bind(&req); err != nil {
		log.Printf("Failed to bind gene data update request: %v", err)
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请求参数错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 查询现有的结果记录
	var resultData string
	err := db.QueryRow(`SELECT result_data FROM detect_sample WHERE id = ?`, id).Scan(&resultData)
	if err != nil {
		log.Printf("Failed to find result for detect_sample: %v", err)
		c.JSON(consts.StatusNotFound, ApiResponse{
			Code:    404,
			Success: false,
			Message: "未找到该样本的检测结果",
			Data:    nil,
		})
		return
	}

	// 解析现有的resultData
	var existingData map[string]interface{}
	if err := json.Unmarshal([]byte(resultData), &existingData); err != nil {
		log.Printf("Failed to parse existing result data: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 更新gene_data字段
	existingData["gene_data"] = req.GeneData

	// 重新序列化resultData
	updatedResultData, err := json.Marshal(existingData)
	if err != nil {
		log.Printf("Failed to marshal updated result data: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 更新数据库
	_, err = db.Exec(`UPDATE detect_sample SET result_data = ?, result_updated_at = NOW() WHERE id = ?`, string(updatedResultData), id)
	if err != nil {
		log.Printf("Failed to update gene data: %v", err)
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
		Message: "基因表达值更新成功",
		Data:    nil,
	})
}

// 获取样本数据
func getSampleData(c *app.RequestContext, db *sql.DB) (interface{}, error) {
	// 获取查询参数
	idParam := c.Query("id")
	patientIdParam := c.Query("patient_id")
	patientCodeParam := c.Query("patient_code")
	sampleCodeParam := c.Query("sample_code")
	if idParam == "" && patientIdParam == "" && patientCodeParam == "" {
		return getSampleListData(c, db)
	}

	// 构建SQL查询
	var query string
	var args []interface{}

	if idParam != "" {
		// 如果有id参数，只查询对应ID的样本
		// 尝试将idParam转换为整数
		id, err := strconv.Atoi(idParam)
		if err == nil {
			query = `SELECT s.id, s.sample_code as sample_code, s.patient_id as patient_id, s.collection_date, s.collection_operator, 
			s.receive_date, s.receive_operator, s.test_operator, s.test_completed_at, s.sample_status as status, s.notes, s.organization, s.sample_created_at as created_at, s.sample_updated_at as updated_at, 
			s.sample_type_id as sample_type_id, st.name as sample_type_name, 
			s.treatment_stage_id as treatment_stage_id, ts.name as treatment_stage_name, 
			s.cancer_type_id as cancer_type_id, ct.name as cancer_type_name, 
			COALESCE(p.name, '') as patient_name, COALESCE(p.patient_code, '') as patient_code, COALESCE(p.id_card, ''), COALESCE(p.phone, ''), COALESCE(p.gender, ''),
			cu.real_name as collection_user_name, ru.real_name as receive_user_name, tu.real_name as test_user_name,
			s.result_data, s.result_status, s.signalvalue, s.batch_id, COALESCE(s.model_id, 0) as model_id,
			ms.model_name as model_name, ms.version as model_version
			FROM detect_sample s 
			LEFT JOIN setting_sample_type st ON s.sample_type_id = st.id 
			LEFT JOIN setting_treatment_stage ts ON s.treatment_stage_id = ts.id 
			LEFT JOIN setting_cancer_type ct ON s.cancer_type_id = ct.id 
			LEFT JOIN detect_patient p ON s.patient_id = p.id 
			LEFT JOIN base_manage_user cu ON s.collection_operator = cu.id 
			LEFT JOIN base_manage_user ru ON s.receive_operator = ru.id 
			LEFT JOIN base_manage_user tu ON s.test_operator = tu.id 
			LEFT JOIN setting_model ms ON s.model_id = ms.id 
				WHERE s.id = ? 
				ORDER BY s.sample_created_at DESC`
			args = append(args, id)
		} else {
			// 如果转换失败，尝试作为sample_code查询
			sampleCode := idParam
			query = `SELECT s.id, s.sample_code as sample_code, s.patient_id as patient_id, s.collection_date, s.collection_operator, 
				s.receive_date, s.receive_operator, s.test_operator, s.test_completed_at, s.sample_status as status, s.notes, s.organization, s.sample_created_at as created_at, s.sample_updated_at as updated_at, 
				s.sample_type_id as sample_type_id, st.name as sample_type_name, 
				s.treatment_stage_id as treatment_stage_id, ts.name as treatment_stage_name, 
				s.cancer_type_id as cancer_type_id, ct.name as cancer_type_name, 
				COALESCE(p.name, '') as patient_name, COALESCE(p.patient_code, '') as patient_code, COALESCE(p.id_card, ''), COALESCE(p.phone, ''), COALESCE(p.gender, ''),
				cu.real_name as collection_user_name, ru.real_name as receive_user_name, tu.real_name as test_user_name,
				s.result_data, s.result_status, s.signalvalue, s.batch_id, COALESCE(s.model_id, 0) as model_id,
				ms.model_name as model_name, ms.version as model_version
				FROM detect_sample s 
				LEFT JOIN setting_sample_type st ON s.sample_type_id = st.id 
				LEFT JOIN setting_treatment_stage ts ON s.treatment_stage_id = ts.id 
				LEFT JOIN setting_cancer_type ct ON s.cancer_type_id = ct.id 
				LEFT JOIN detect_patient p ON s.patient_id = p.id 
				LEFT JOIN base_manage_user cu ON s.collection_operator = cu.id 
				LEFT JOIN base_manage_user ru ON s.receive_operator = ru.id 
				LEFT JOIN base_manage_user tu ON s.test_operator = tu.id 
				LEFT JOIN setting_model ms ON s.model_id = ms.id 
					WHERE s.sample_code = ? 
					ORDER BY s.sample_created_at DESC`
			args = append(args, sampleCode)
		}
	} else if patientIdParam != "" {
		// 如果有patient_id参数，只查询对应患者的样本
		// 尝试将patientIdParam转换为整数
		patientId, err := strconv.Atoi(patientIdParam)
		if err == nil {
			query = `SELECT s.id, s.sample_code as sample_code, s.patient_id as patient_id, s.collection_date, s.collection_operator, 
			s.receive_date, s.receive_operator, s.test_operator, s.test_completed_at, s.sample_status as status, s.notes, s.organization, s.sample_created_at as created_at, s.sample_updated_at as updated_at, 
			s.sample_type_id as sample_type_id, st.name as sample_type_name, 
			s.treatment_stage_id as treatment_stage_id, ts.name as treatment_stage_name, 
			s.cancer_type_id as cancer_type_id, ct.name as cancer_type_name, 
			COALESCE(p.name, '') as patient_name, COALESCE(p.patient_code, '') as patient_code, COALESCE(p.id_card, ''), COALESCE(p.phone, ''), COALESCE(p.gender, ''),
			cu.real_name as collection_user_name, ru.real_name as receive_user_name, tu.real_name as test_user_name,
			s.result_data, s.result_status, s.signalvalue, s.batch_id, COALESCE(s.model_id, 0) as model_id,
			ms.model_name as model_name, ms.version as model_version
			FROM detect_sample s 
			LEFT JOIN setting_sample_type st ON s.sample_type_id = st.id 
			LEFT JOIN setting_treatment_stage ts ON s.treatment_stage_id = ts.id 
			LEFT JOIN setting_cancer_type ct ON s.cancer_type_id = ct.id 
			LEFT JOIN detect_patient p ON s.patient_id = p.id 
			LEFT JOIN base_manage_user cu ON s.collection_operator = cu.id 
			LEFT JOIN base_manage_user ru ON s.receive_operator = ru.id 
			LEFT JOIN base_manage_user tu ON s.test_operator = tu.id 
			LEFT JOIN setting_model ms ON s.model_id = ms.id 
				WHERE s.patient_id = ? 
				ORDER BY s.sample_created_at DESC`
			args = append(args, patientId)
		} else {
			// 如果转换失败，返回空列表
			return utils.H{
				"list":  []utils.H{},
				"total": 0,
			}, nil
		}
	} else if patientCodeParam != "" {
		query = `SELECT s.id, s.sample_code as sample_code, s.patient_id as patient_id, s.collection_date, s.collection_operator, 
			s.receive_date, s.receive_operator, s.test_operator, s.test_completed_at, s.sample_status as status, s.notes, s.organization, s.sample_created_at as created_at, s.sample_updated_at as updated_at, 
			s.sample_type_id as sample_type_id, st.name as sample_type_name, 
			s.treatment_stage_id as treatment_stage_id, ts.name as treatment_stage_name, 
			s.cancer_type_id as cancer_type_id, ct.name as cancer_type_name, 
			COALESCE(p.name, '') as patient_name, COALESCE(p.patient_code, '') as patient_code, COALESCE(p.id_card, ''), COALESCE(p.phone, ''), COALESCE(p.gender, ''),
			cu.real_name as collection_user_name, ru.real_name as receive_user_name, tu.real_name as test_user_name,
			s.result_data, s.result_status, s.signalvalue, s.batch_id, COALESCE(s.model_id, 0) as model_id,
			ms.model_name as model_name, ms.version as model_version
			FROM detect_sample s 
			LEFT JOIN setting_sample_type st ON s.sample_type_id = st.id 
			LEFT JOIN setting_treatment_stage ts ON s.treatment_stage_id = ts.id 
			LEFT JOIN setting_cancer_type ct ON s.cancer_type_id = ct.id 
			LEFT JOIN detect_patient p ON s.patient_id = p.id 
			LEFT JOIN base_manage_user cu ON s.collection_operator = cu.id 
			LEFT JOIN base_manage_user ru ON s.receive_operator = ru.id 
			LEFT JOIN base_manage_user tu ON s.test_operator = tu.id 
			LEFT JOIN setting_model ms ON s.model_id = ms.id 
				WHERE p.patient_code = ? 
				ORDER BY s.sample_created_at DESC`
		args = append(args, patientCodeParam)
	} else if sampleCodeParam != "" {
		// 如果有sample_code参数，只查询对应样本
		sampleCode := sampleCodeParam
		query = `SELECT s.id, s.sample_code as sample_code, s.patient_id as patient_id, s.collection_date, s.collection_operator, 
			s.receive_date, s.receive_operator, s.test_operator, s.test_completed_at, s.sample_status as status, s.notes, s.organization, s.sample_created_at as created_at, s.sample_updated_at as updated_at, 
			s.sample_type_id as sample_type_id, st.name as sample_type_name, 
			s.treatment_stage_id as treatment_stage_id, ts.name as treatment_stage_name, 
			s.cancer_type_id as cancer_type_id, ct.name as cancer_type_name, 
			COALESCE(p.name, '') as patient_name, COALESCE(p.patient_code, '') as patient_code, COALESCE(p.id_card, ''), COALESCE(p.phone, ''), COALESCE(p.gender, ''),
			cu.real_name as collection_user_name, ru.real_name as receive_user_name, tu.real_name as test_user_name,
			s.result_data, s.result_status, s.signalvalue, s.batch_id, COALESCE(s.model_id, 0) as model_id,
			ms.model_name as model_name, ms.version as model_version
			FROM detect_sample s 
			LEFT JOIN setting_sample_type st ON s.sample_type_id = st.id 
			LEFT JOIN setting_treatment_stage ts ON s.treatment_stage_id = ts.id 
			LEFT JOIN setting_cancer_type ct ON s.cancer_type_id = ct.id 
			LEFT JOIN detect_patient p ON s.patient_id = p.id 
			LEFT JOIN base_manage_user cu ON s.collection_operator = cu.id 
			LEFT JOIN base_manage_user ru ON s.receive_operator = ru.id 
			LEFT JOIN base_manage_user tu ON s.test_operator = tu.id 
			LEFT JOIN setting_model ms ON s.model_id = ms.id 
				WHERE s.sample_code = ? 
				ORDER BY s.sample_created_at DESC`
		args = append(args, sampleCode)
	} else {
		// 没有id参数，返回所有样本
		query = `SELECT s.id, s.sample_code as sample_code, s.patient_id as patient_id, s.collection_date, s.collection_operator, 
			s.receive_date, s.receive_operator, s.test_operator, s.test_completed_at, s.sample_status as status, s.notes, s.organization, s.sample_created_at as created_at, s.sample_updated_at as updated_at, 
			s.sample_type_id as sample_type_id, st.name as sample_type_name, 
			s.treatment_stage_id as treatment_stage_id, ts.name as treatment_stage_name, 
			s.cancer_type_id as cancer_type_id, ct.name as cancer_type_name, 
			COALESCE(p.name, '') as patient_name, COALESCE(p.patient_code, '') as patient_code, COALESCE(p.id_card, ''), COALESCE(p.phone, ''), COALESCE(p.gender, ''),
			cu.real_name as collection_user_name, ru.real_name as receive_user_name, tu.real_name as test_user_name,
			s.result_data, s.result_status, s.signalvalue, s.batch_id,
			ms.model_name as model_name, ms.version as model_version
			FROM detect_sample s 
			LEFT JOIN setting_sample_type st ON s.sample_type_id = st.id 
			LEFT JOIN setting_treatment_stage ts ON s.treatment_stage_id = ts.id 
			LEFT JOIN setting_cancer_type ct ON s.cancer_type_id = ct.id 
			LEFT JOIN detect_patient p ON s.patient_id = p.id 
			LEFT JOIN base_manage_user cu ON s.collection_operator = cu.id 
			LEFT JOIN base_manage_user ru ON s.receive_operator = ru.id 
			LEFT JOIN base_manage_user tu ON s.test_operator = tu.id 
			LEFT JOIN setting_model ms ON s.model_id = ms.id 
			ORDER BY s.sample_created_at DESC`
	}

	// 执行查询
	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("Failed to query detect_samples: %v", err)
		return nil, err
	}
	defer rows.Close()

	// 遍历查询结果
	var detect_samples []utils.H
	var detect_sampleIDs []int

	// 先收集所有样本信息
	for rows.Next() {
		// 为每个样本初始化变量
		var id, detect_patientID, modelID int
		var collectionOperator, receiveOperator, testOperator, batchID sql.NullInt64
		var detect_sampleTypeID, cancerTypeID, treatmentStageID sql.NullInt64
		var detect_sampleCode, detect_sampleStatus, detect_patientName, detect_patientCode, detect_patientIDCard, detect_patientPhone, organization, gender string
		var detect_sampleTypeName, treatmentStageName, cancerTypeName sql.NullString
		var collectionUserName, receiveUserName, testUserName, modelName, modelVersion, resultStatus sql.NullString
		var collectionDate, detect_sampleCreatedAt, detect_sampleUpdatedAt time.Time
		var receiveDate, testCompletedAt sql.NullTime
		var notes, resultData sql.NullString
		var signalvalue sql.NullFloat64

		// 扫描数据
		err := rows.Scan(&id, &detect_sampleCode, &detect_patientID, &collectionDate, &collectionOperator,
			&receiveDate, &receiveOperator, &testOperator, &testCompletedAt, &detect_sampleStatus, &notes, &organization, &detect_sampleCreatedAt, &detect_sampleUpdatedAt,
			&detect_sampleTypeID, &detect_sampleTypeName, &treatmentStageID, &treatmentStageName, &cancerTypeID, &cancerTypeName,
			&detect_patientName, &detect_patientCode, &detect_patientIDCard, &detect_patientPhone, &gender, &collectionUserName, &receiveUserName, &testUserName,
			&resultData, &resultStatus, &signalvalue, &batchID, &modelID, &modelName, &modelVersion)
		if err != nil {
			log.Printf("Failed to scan detect_sample: %v", err)
			// 跳过有错误的样本，继续处理下一个
			continue
		}

		// 构建样本信息
		detect_sample := utils.H{
			"id":                   id,
			"sample_code":          detect_sampleCode,
			"sampleCode":           detect_sampleCode,
			"patient_id":           detect_patientID,
			"patientId":            detect_patientID,
			"patient_name":         detect_patientName,
			"patientName":          detect_patientName,
			"patient_code":         detect_patientCode,
			"patientCode":          detect_patientCode,
			"id_card":              detect_patientIDCard,
			"phone":                detect_patientPhone,
			"gender":               gender,
			"collection_date":      collectionDate.Format("2006-01-02T15:04:05+08:00"),
			"collectionDate":       collectionDate.Format("2006-01-02T15:04:05+08:00"),
			"status":               detect_sampleStatus,
			"sample_type_id":       0,
			"sampleTypeId":         0,
			"sample_type_name":     "",
			"sampleTypeName":       "",
			"sampleType":           "",
			"cancer_type_id":       0,
			"cancerTypeId":         0,
			"cancer_type_name":     "",
			"cancerTypeName":       "",
			"treatment_stage_id":   0,
			"treatmentStageId":     0,
			"treatment_stage_name": "",
			"treatmentStageName":   "",
			"created_at":           detect_sampleCreatedAt.Format("2006-01-02T15:04:05+08:00"),
			"createdAt":            detect_sampleCreatedAt.Format("2006-01-02T15:04:05+08:00"),
			"updated_at":           detect_sampleUpdatedAt.Format("2006-01-02T15:04:05+08:00"),
			"updatedAt":            detect_sampleUpdatedAt.Format("2006-01-02T15:04:05+08:00"),
			"organization":         organization,
			"gene_data":            nil,       // 初始化基因数据为nil
			"geneData":             nil,       // 初始化基因数据为nil
			"result":               utils.H{}, // 初始化结果信息为空
			"model_id":             modelID,
			"modelId":              modelID,
		}
		if detect_sampleTypeID.Valid {
			detect_sample["sample_type_id"] = int(detect_sampleTypeID.Int64)
			detect_sample["sampleTypeId"] = int(detect_sampleTypeID.Int64)
		}
		if detect_sampleTypeName.Valid {
			detect_sample["sample_type_name"] = detect_sampleTypeName.String
			detect_sample["sampleTypeName"] = detect_sampleTypeName.String
			detect_sample["sampleType"] = detect_sampleTypeName.String
		}
		if cancerTypeID.Valid {
			detect_sample["cancer_type_id"] = int(cancerTypeID.Int64)
			detect_sample["cancerTypeId"] = int(cancerTypeID.Int64)
		}
		if cancerTypeName.Valid {
			detect_sample["cancer_type_name"] = cancerTypeName.String
			detect_sample["cancerTypeName"] = cancerTypeName.String
		}
		if treatmentStageID.Valid {
			detect_sample["treatment_stage_id"] = int(treatmentStageID.Int64)
			detect_sample["treatmentStageId"] = int(treatmentStageID.Int64)
		}
		if treatmentStageName.Valid {
			detect_sample["treatment_stage_name"] = treatmentStageName.String
			detect_sample["treatmentStageName"] = treatmentStageName.String
		}

		// 处理可选的用户名字段
		if collectionUserName.Valid {
			detect_sample["collection_user_name"] = collectionUserName.String
		} else {
			detect_sample["collection_user_name"] = nil
		}

		if receiveUserName.Valid {
			detect_sample["receive_user_name"] = receiveUserName.String
		} else {
			detect_sample["receive_user_name"] = nil
		}

		// 处理检测人员信息
		if testOperator.Valid {
			detect_sample["test_operator"] = int(testOperator.Int64)
			if testUserName.Valid {
				detect_sample["test_user_name"] = testUserName.String
			}
		} else {
			detect_sample["test_operator"] = nil
			detect_sample["test_user_name"] = nil
		}

		// 处理检测完成时间
		if testCompletedAt.Valid {
			detect_sample["test_completed_at"] = testCompletedAt.Time.Format("2006-01-02T15:04:05+08:00")
		} else {
			detect_sample["test_completed_at"] = nil
		}

		// 构建带版本的模型名称
		modelNameWithVersion := "-"
		if modelName.Valid {
			modelNameWithVersion = modelName.String
			if modelVersion.Valid && modelVersion.String != "" {
				modelNameWithVersion = fmt.Sprintf("%s [V%s]", modelNameWithVersion, modelVersion.String)
			}
		}
		detect_sample["model_name"] = modelNameWithVersion

		// 处理可选字段
		if collectionOperator.Valid {
			detect_sample["collection_operator"] = int(collectionOperator.Int64)
		} else {
			detect_sample["collection_operator"] = nil
		}

		if receiveDate.Valid {
			detect_sample["receive_date"] = receiveDate.Time.Format("2006-01-02T15:04:05+08:00")
		}

		if receiveOperator.Valid {
			detect_sample["receive_operator"] = int(receiveOperator.Int64)
		} else {
			detect_sample["receive_operator"] = nil
		}

		if notes.Valid {
			detect_sample["notes"] = notes.String
		}

		// 处理结果信息
		if resultData.Valid || resultStatus.Valid || signalvalue.Valid || batchID.Valid {
			result := utils.H{
				"status":      resultStatus.String,
				"signalvalue": nil,
				"batch_id":    nil,
				"gene_data":   nil,
			}

			if signalvalue.Valid {
				result["signalvalue"] = signalvalue.Float64
			}

			if batchID.Valid {
				result["batch_id"] = int(batchID.Int64)
				detect_sample["batch_id"] = int(batchID.Int64)
				var batchCode, batchStatus string
				if err := db.QueryRow("SELECT COALESCE(batch_code, ''), COALESCE(status, '') FROM detect_batch WHERE id = ?", int(batchID.Int64)).Scan(&batchCode, &batchStatus); err == nil {
					result["batch_code"] = batchCode
					result["batch_status"] = batchStatus
					detect_sample["batch_code"] = batchCode
					detect_sample["batch_status"] = batchStatus
					detect_sample["batchStatus"] = batchStatus
				}
				samePatientSamples := getSamePatientBatchSamples(db, id)
				detect_sample["samePatientSamples"] = samePatientSamples
				detect_sample["same_patient_batch_samples"] = samePatientSamples
			}

			// 解析result_data中的基因数据
			if resultData.Valid {
				var resultDataMap map[string]interface{}
				if err := json.Unmarshal([]byte(resultData.String), &resultDataMap); err == nil {
					if geneData, ok := resultDataMap["gene_data"].(map[string]interface{}); ok {
						result["gene_data"] = geneData
						detect_sample["gene_data"] = geneData
						result["geneData"] = geneData
						detect_sample["geneData"] = geneData
					}
				}
			}

			detect_sample["result"] = result
		}

		detect_samples = append(detect_samples, detect_sample)
		detect_sampleIDs = append(detect_sampleIDs, id)
	}

	// 检查遍历过程中是否有错误
	if err = rows.Err(); err != nil {
		log.Printf("Error iterating detect_samples: %v", err)
		return nil, err
	}

	// 使用实际返回的样本列表长度作为总数，确保总数与返回的列表一致
	enrichSamplesWithReportStatus(db, detect_samples)
	enrichSamplesWithPanelMatches(db, detect_samples)
	total := len(detect_samples)

	// 返回样本列表
	return utils.H{
		"list":  detect_samples,
		"total": total,
	}, nil
}

// enrichSamplesWithPanelMatches exposes the batch-level matching cache on an
// individual sample response, so the result detail page does not need to know
// about the batch implementation.
func enrichSamplesWithPanelMatches(db *sql.DB, samples []utils.H) {
	for _, sample := range samples {
		batchID, batchOK := sample["batch_id"].(int)
		sampleCode, codeOK := sample["sample_code"].(string)
		if !batchOK || batchID <= 0 || !codeOK || strings.TrimSpace(sampleCode) == "" {
			continue
		}

		var idsJSON, matchesJSON, genesJSON sql.NullString
		err := db.QueryRow(`SELECT matched_panel_ids_json, panel_matches_json, COALESCE(sample_genes_json, '')
			FROM detect_sample_panel_match WHERE batch_id = ? AND sample_code = ?`, batchID, sampleCode).
			Scan(&idsJSON, &matchesJSON, &genesJSON)
		if err != nil {
			if err != sql.ErrNoRows {
				log.Printf("Load panel matches for sample %s: %v", sampleCode, err)
			}
			continue
		}

		var matchedIDs []int
		var panelMatches []utils.H
		var sampleGenes []string
		if idsJSON.Valid && strings.TrimSpace(idsJSON.String) != "" {
			_ = json.Unmarshal([]byte(idsJSON.String), &matchedIDs)
		}
		if matchesJSON.Valid && strings.TrimSpace(matchesJSON.String) != "" {
			_ = json.Unmarshal([]byte(matchesJSON.String), &panelMatches)
		}
		if genesJSON.Valid && strings.TrimSpace(genesJSON.String) != "" {
			_ = json.Unmarshal([]byte(genesJSON.String), &sampleGenes)
		}
		sample["matched_panel_ids"] = matchedIDs
		sample["panel_matches"] = panelMatches
		sample["sample_genes"] = sampleGenes
	}
}

func enrichSamplesWithReportStatus(db *sql.DB, samples []utils.H) {
	for _, sample := range samples {
		sampleID, _ := sample["id"].(int)
		if sampleID <= 0 {
			continue
		}
		var reportID int
		var status string
		var generatedAt, reviewedAt, patientViewedAt sql.NullTime
		var generatedBy, reviewedBy sql.NullString
		err := db.QueryRow(`SELECT r.id, COALESCE(r.status, ''), r.generated_time, r.reviewed_time,
			COALESCE(gu.real_name, gu.username), COALESCE(ru.real_name, ru.username), r.patient_viewed_at
			FROM detect_report r
			LEFT JOIN base_manage_user gu ON gu.id = r.generated_by
			LEFT JOIN base_manage_user ru ON ru.id = r.reviewed_by
			WHERE r.sample_id = ? ORDER BY r.created_at DESC, r.id DESC LIMIT 1`, sampleID).
			Scan(&reportID, &status, &generatedAt, &reviewedAt, &generatedBy, &reviewedBy, &patientViewedAt)
		if err == sql.ErrNoRows {
			sample["has_report"] = false
			sample["report_status"] = "none"
			continue
		}
		if err != nil {
			log.Printf("Load report status for sample %d: %v", sampleID, err)
			continue
		}
		sample["has_report"] = true
		sample["report_id"] = reportID
		sample["report_status"] = status
		sample["patient_viewed"] = patientViewedAt.Valid
		if generatedAt.Valid {
			sample["report_generated_time"] = generatedAt.Time.Format("2006-01-02T15:04:05+08:00")
		}
		if reviewedAt.Valid {
			sample["report_reviewed_time"] = reviewedAt.Time.Format("2006-01-02T15:04:05+08:00")
		}
		if patientViewedAt.Valid {
			sample["patient_viewed_at"] = patientViewedAt.Time.Format("2006-01-02T15:04:05+08:00")
		}
		if generatedBy.Valid {
			sample["report_generated_by_name"] = generatedBy.String
		}
		if reviewedBy.Valid {
			sample["report_reviewed_by_name"] = reviewedBy.String
		}
	}
}

func getSampleListData(c *app.RequestContext, db *sql.DB) (interface{}, error) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", c.DefaultQuery("page_size", "10")))
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

	sampleCode := strings.TrimSpace(c.Query("sample_code"))
	if sampleCode == "" {
		sampleCode = strings.TrimSpace(c.Query("sampleCode"))
	}
	patientName := strings.TrimSpace(c.Query("patient_name"))
	if patientName == "" {
		patientName = strings.TrimSpace(c.Query("patientName"))
	}
	sampleType := strings.TrimSpace(c.Query("sample_type"))
	if sampleType == "" {
		sampleType = strings.TrimSpace(c.Query("sampleType"))
	}
	status := strings.TrimSpace(c.Query("status"))

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
	if sampleType != "" {
		where = append(where, "(st.name LIKE ? OR CAST(s.sample_type_id AS CHAR) = ?)")
		args = append(args, "%"+sampleType+"%", sampleType)
	}
	if status != "" {
		where = append(where, "s.sample_status = ?")
		args = append(args, status)
	}
	whereSQL := strings.Join(where, " AND ")

	var total int
	countQuery := `SELECT COUNT(*)
		FROM detect_sample s
		LEFT JOIN detect_patient p ON s.patient_id = p.id
		LEFT JOIN setting_sample_type st ON s.sample_type_id = st.id
		WHERE ` + whereSQL
	if err := db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, err
	}

	queryArgs := append([]interface{}{}, args...)
	queryArgs = append(queryArgs, pageSize, offset)
	rows, err := db.Query(`SELECT s.id, s.sample_code, s.patient_id, COALESCE(p.name, ''), COALESCE(p.patient_code, ''),
			COALESCE(p.id_card, ''), COALESCE(p.phone, ''), COALESCE(p.gender, ''),
			s.collection_date, s.collection_operator, s.receive_date, s.receive_operator, s.test_operator, s.test_completed_at,
			COALESCE(s.sample_status, ''), COALESCE(s.organization, ''), s.sample_created_at, s.sample_updated_at,
			COALESCE(s.sample_type_id, 0), COALESCE(st.name, ''), COALESCE(s.treatment_stage_id, 0), COALESCE(ts.name, ''),
			COALESCE(s.cancer_type_id, 0), COALESCE(ct.name, ''), COALESCE(cu.real_name, ''), COALESCE(ru.real_name, ''),
			COALESCE(tu.real_name, ''), s.batch_id, COALESCE(b.batch_code, ''), COALESCE(b.status, '')
		FROM detect_sample s
		LEFT JOIN setting_sample_type st ON s.sample_type_id = st.id
		LEFT JOIN setting_treatment_stage ts ON s.treatment_stage_id = ts.id
		LEFT JOIN setting_cancer_type ct ON s.cancer_type_id = ct.id
		LEFT JOIN detect_patient p ON s.patient_id = p.id
		LEFT JOIN base_manage_user cu ON s.collection_operator = cu.id
		LEFT JOIN base_manage_user ru ON s.receive_operator = ru.id
		LEFT JOIN base_manage_user tu ON s.test_operator = tu.id
		LEFT JOIN detect_batch b ON s.batch_id = b.id
		WHERE `+whereSQL+`
		ORDER BY s.sample_created_at DESC
		LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []utils.H{}
	for rows.Next() {
		var id, patientID, sampleTypeID, treatmentStageID, cancerTypeID int
		var sampleCodeVal, patientNameVal, patientCode, idCard, phone, gender, statusVal, organization string
		var sampleTypeName, treatmentStageName, cancerTypeName, collectionUserName, receiveUserName, testUserName, batchCode, batchStatus string
		var collectionOperator, receiveOperator, testOperator, batchID sql.NullInt64
		var collectionDate, createdAt, updatedAt time.Time
		var receiveDate, testCompletedAt sql.NullTime
		if err := rows.Scan(&id, &sampleCodeVal, &patientID, &patientNameVal, &patientCode, &idCard, &phone, &gender,
			&collectionDate, &collectionOperator, &receiveDate, &receiveOperator, &testOperator, &testCompletedAt,
			&statusVal, &organization, &createdAt, &updatedAt, &sampleTypeID, &sampleTypeName, &treatmentStageID, &treatmentStageName,
			&cancerTypeID, &cancerTypeName, &collectionUserName, &receiveUserName, &testUserName, &batchID, &batchCode, &batchStatus); err != nil {
			log.Printf("Failed to scan sample list row: %v", err)
			continue
		}
		item := utils.H{
			"id":                   id,
			"sample_code":          sampleCodeVal,
			"sampleCode":           sampleCodeVal,
			"patient_id":           patientID,
			"patientId":            patientID,
			"patient_name":         patientNameVal,
			"patientName":          patientNameVal,
			"patient_code":         patientCode,
			"patientCode":          patientCode,
			"id_card":              idCard,
			"phone":                phone,
			"gender":               gender,
			"collection_date":      collectionDate.Format("2006-01-02T15:04:05+08:00"),
			"collectionDate":       collectionDate.Format("2006-01-02T15:04:05+08:00"),
			"status":               statusVal,
			"organization":         organization,
			"created_at":           createdAt.Format("2006-01-02T15:04:05+08:00"),
			"createdAt":            createdAt.Format("2006-01-02T15:04:05+08:00"),
			"updated_at":           updatedAt.Format("2006-01-02T15:04:05+08:00"),
			"updatedAt":            updatedAt.Format("2006-01-02T15:04:05+08:00"),
			"sample_type_id":       sampleTypeID,
			"sampleTypeId":         sampleTypeID,
			"sample_type_name":     sampleTypeName,
			"sampleTypeName":       sampleTypeName,
			"sampleType":           sampleTypeName,
			"treatment_stage_id":   treatmentStageID,
			"treatmentStageId":     treatmentStageID,
			"treatment_stage_name": treatmentStageName,
			"treatmentStageName":   treatmentStageName,
			"cancer_type_id":       cancerTypeID,
			"cancerTypeId":         cancerTypeID,
			"cancer_type_name":     cancerTypeName,
			"cancerTypeName":       cancerTypeName,
			"collection_user_name": collectionUserName,
			"receive_user_name":    receiveUserName,
			"test_user_name":       testUserName,
			"gene_data":            nil,
			"geneData":             nil,
			"result":               utils.H{},
		}
		if collectionOperator.Valid {
			item["collection_operator"] = int(collectionOperator.Int64)
		}
		if receiveDate.Valid {
			item["receive_date"] = receiveDate.Time.Format("2006-01-02T15:04:05+08:00")
		}
		if receiveOperator.Valid {
			item["receive_operator"] = int(receiveOperator.Int64)
		}
		if testOperator.Valid {
			item["test_operator"] = int(testOperator.Int64)
		}
		if testCompletedAt.Valid {
			item["test_completed_at"] = testCompletedAt.Time.Format("2006-01-02T15:04:05+08:00")
		}
		if batchID.Valid {
			item["batch_id"] = int(batchID.Int64)
			item["batch_code"] = batchCode
			item["batch_status"] = batchStatus
			item["batchStatus"] = batchStatus
		}
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return utils.H{"list": list, "total": total}, nil
}

// 处理获取样本列表及相关系统数据的请求
func HandleGetSamplesWithSystemData(c *app.RequestContext, db *sql.DB) {
	// 获取样本数据
	sampleData, err := getSampleData(c, db)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "获取样本数据失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 获取癌症类型数据
	cancerTypesData, err := getCancerTypesData(db)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "获取癌症类型数据失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 获取用户数据
	usersData, err := getUsersData(db)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "获取用户数据失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 获取治疗阶段数据
	treatmentStagesData, err := getTreatmentStagesData(db)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "获取治疗阶段数据失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 获取样本类型数据
	sampleTypesData, err := getSampleTypesData(db)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "获取样本类型数据失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 构建组合响应
	combinedData := utils.H{
		"samples":         sampleData,
		"cancerTypes":     cancerTypesData,
		"users":           usersData,
		"treatmentStages": treatmentStagesData,
		"sampleTypes":     sampleTypesData,
	}

	// 返回组合响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取样本及系统数据成功",
		Data:    combinedData,
	})
}
