package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func salesPersonCodeForUser(db *sql.DB, userID int) string {
	if userID <= 0 {
		return ""
	}
	var code string
	err := db.QueryRow(`SELECT COALESCE(employee_id, '')
		FROM base_manage_user WHERE id = ? LIMIT 1`, userID).Scan(&code)
	if err != nil {
		log.Printf("Failed to get sales person code for user %d: %v", userID, err)
		return ""
	}
	return strings.TrimSpace(code)
}

func salesPatientAccessFilter(roleNames []string, salesPersonCode, tableAlias string) (string, []interface{}) {
	if !hasRoleName(roleNames, "销售") || hasRoleName(roleNames, "管理员", "IT") {
		return "", nil
	}

	column := "sales_person"
	if strings.TrimSpace(tableAlias) != "" {
		column = strings.TrimSpace(tableAlias) + ".sales_person"
	}
	salesPersonCode = strings.TrimSpace(salesPersonCode)
	if salesPersonCode == "" {
		return "1 = 0", nil
	}
	return column + " = ?", []interface{}{salesPersonCode}
}

func patientAccessFilterForUser(db *sql.DB, userID int, tableAlias string) (string, []interface{}) {
	return salesPatientAccessFilter(getUserRoleNames(db, userID), salesPersonCodeForUser(db, userID), tableAlias)
}

func getSalesPersonNames(db *sql.DB, salesPersonCodes []string) map[string]string {
	salesPersonNames := make(map[string]string)
	if len(salesPersonCodes) == 0 {
		return salesPersonNames
	}

	seen := make(map[string]bool)
	var codes []string
	for _, code := range salesPersonCodes {
		code = strings.TrimSpace(code)
		if code != "" && !seen[code] {
			seen[code] = true
			codes = append(codes, code)
		}
	}
	if len(codes) == 0 {
		return salesPersonNames
	}

	cacheKey := "sales_person_names_" + strings.Join(codes, "_")
	if err := GetCache(cacheKey, &salesPersonNames); err == nil {
		return salesPersonNames
	}

	placeholders := make([]string, len(codes))
	args := make([]interface{}, len(codes))
	for i, code := range codes {
		placeholders[i] = "?"
		args[i] = code
	}

	query := `SELECT COALESCE(employee_id, ''),
			COALESCE(NULLIF(real_name, ''), username)
		FROM base_manage_user
		WHERE status = 1 AND employee_id IN (` + strings.Join(placeholders, ", ") + `)`
	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("Failed to get sales person names: %v", err)
		return salesPersonNames
	}
	defer rows.Close()

	for rows.Next() {
		var employeeID, name string
		if err := rows.Scan(&employeeID, &name); err != nil {
			log.Printf("Failed to scan sales person: %v", err)
			continue
		}
		employeeID = strings.TrimSpace(employeeID)
		if employeeID != "" {
			salesPersonNames[employeeID] = name
		}
	}

	SetCache(cacheKey, salesPersonNames, time.Hour)
	return salesPersonNames
}

func getSalesPersonName(db *sql.DB, salesPersonCode string) string {
	salesPersonCode = strings.TrimSpace(salesPersonCode)
	if salesPersonCode == "" {
		return ""
	}
	names := getSalesPersonNames(db, []string{salesPersonCode})
	return names[salesPersonCode]
}

func buildSalesPersonInfo(db *sql.DB, salesPersonCode string) utils.H {
	salesPersonCode = strings.TrimSpace(salesPersonCode)
	return utils.H{
		"id":   salesPersonCode,
		"code": salesPersonCode,
		"name": getSalesPersonName(db, salesPersonCode),
	}
}

func resolvePatientID(db *sql.DB, patientIdentifier string, includeInactive bool) (int, string, error) {
	patientIdentifier = strings.TrimSpace(patientIdentifier)
	if patientIdentifier == "" {
		return 0, "", fmt.Errorf("患者编号不能为空")
	}

	whereActive := " AND is_active = 1"
	if includeInactive {
		whereActive = ""
	}

	var id int
	var patientCode string
	queryByCode := "SELECT id, patient_code FROM detect_patient WHERE patient_code = ?" + whereActive + " LIMIT 1"
	if err := db.QueryRow(queryByCode, patientIdentifier).Scan(&id, &patientCode); err == nil {
		return id, patientCode, nil
	} else if err != sql.ErrNoRows {
		return 0, "", err
	}

	if numericID, err := strconv.Atoi(patientIdentifier); err == nil {
		queryByID := "SELECT id, patient_code FROM detect_patient WHERE id = ?" + whereActive + " LIMIT 1"
		if err := db.QueryRow(queryByID, numericID).Scan(&id, &patientCode); err != nil {
			return 0, "", err
		}
		return id, patientCode, nil
	}

	return 0, "", sql.ErrNoRows
}

func normalizePatientDate(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}

	if t, err := time.Parse("2006-01-02", value); err == nil {
		return t.Format("2006-01-02"), nil
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t.Format("2006-01-02"), nil
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.Local); err == nil {
		return t.Format("2006-01-02"), nil
	}

	return "", fmt.Errorf("日期格式错误，请使用 YYYY-MM-DD")
}

func generatePatientCodeForTime(db *sql.DB, t time.Time) string {
	if t.IsZero() {
		t = time.Now()
	}
	prefix := "HWP" + t.Format("200601")
	start := len(prefix) + 1
	var maxSeq int
	err := db.QueryRow(`SELECT COALESCE(MAX(CAST(SUBSTRING(patient_code, ?) AS UNSIGNED)), 0)
		FROM detect_patient
		WHERE patient_code LIKE ? AND SUBSTRING(patient_code, ?) REGEXP '^[0-9]+$'`,
		start, prefix+"%", start).Scan(&maxSeq)
	if err != nil {
		log.Printf("Failed to calculate next patient code for %s: %v", prefix, err)
		maxSeq = int(time.Now().UnixNano() % 10000)
	}
	return fmt.Sprintf("%s%04d", prefix, maxSeq+1)
}

func generatePatientCode(db *sql.DB) string {
	return generatePatientCodeForTime(db, time.Now())
}

var allowedPatientDocumentTypes = map[string]bool{
	"居民身份证":       true,
	"护照":          true,
	"港澳居民来往内地通行证": true,
	"台湾居民来往大陆通行证": true,
	"自编号":         true,
}

func normalizePatientDocumentType(value string) string {
	value = strings.TrimSpace(value)
	if allowedPatientDocumentTypes[value] {
		return value
	}
	return "居民身份证"
}

func normalizePatientDocumentNo(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func isResidentIDCard(docType string) bool {
	return normalizePatientDocumentType(docType) == "居民身份证"
}

func legacyIDCardForDocument(docType, docNo string) string {
	docNo = normalizePatientDocumentNo(docNo)
	if isResidentIDCard(docType) {
		return docNo
	}
	return ""
}

func patientDocumentWhereClause(column string) string {
	return fmt.Sprintf("(COALESCE(id_document_no, '') = %s OR (COALESCE(id_document_no, '') = '' AND COALESCE(id_card, '') = %s))", column, column)
}

func ensurePatientDocumentNoAvailable(db *sql.DB, docNo string, excludePatientID int) error {
	docNo = normalizePatientDocumentNo(docNo)
	if docNo == "" {
		return nil
	}
	query := "SELECT COUNT(*) FROM detect_patient WHERE is_active = 1 AND " + patientDocumentWhereClause("?")
	args := []interface{}{docNo, docNo}
	if excludePatientID > 0 {
		query += " AND id <> ?"
		args = append(args, excludePatientID)
	}
	var count int
	if err := db.QueryRow(query, args...).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("该患者已存在")
	}
	return nil
}

func parseResidentIDCardInfo(idCard string) (string, string, bool) {
	idCard = normalizePatientDocumentNo(idCard)
	if len(idCard) != 18 {
		return "", "", false
	}
	birthdayText := idCard[6:14]
	birthday, err := time.Parse("20060102", birthdayText)
	if err != nil {
		return "", "", false
	}
	genderCode, err := strconv.Atoi(idCard[16:17])
	if err != nil {
		return "", "", false
	}
	gender := "女"
	if genderCode%2 == 1 {
		gender = "男"
	}
	return gender, birthday.Format("2006-01-02"), true
}

func appendPatientKeywordFilter(query string, args []interface{}, keyword string) (string, []interface{}) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return query, args
	}

	like := "%" + keyword + "%"
	query += ` AND (name LIKE ? OR phone LIKE ? OR patient_code LIKE ?
		OR COALESCE(id_document_no, '') LIKE ? OR COALESCE(id_card, '') LIKE ?)`
	for i := 0; i < 5; i++ {
		args = append(args, like)
	}
	return query, args
}

// 患者管理相关处理函数
func HandleListPatients(c *app.RequestContext, db *sql.DB) {
	// 从查询参数获取is_active值，默认是1（活跃患者）
	isActive := c.DefaultQuery("is_active", "1")

	// 从查询参数获取分页参数，默认第1页，每页10条
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("pageSize", "10")

	// 从查询参数获取搜索参数
	name := c.Query("name")
	idCard := c.Query("idCard")
	if idCard == "" {
		idCard = c.Query("id_card")
	}
	idDocumentNo := strings.TrimSpace(c.Query("idDocumentNo"))
	if idDocumentNo == "" {
		idDocumentNo = strings.TrimSpace(c.Query("id_document_no"))
	}
	if idDocumentNo == "" {
		idDocumentNo = strings.TrimSpace(idCard)
	}
	phone := c.Query("phone")
	patientCode := c.Query("patient_code")
	keyword := c.Query("keyword")
	completionStatus := c.Query("completion_status")
	salesPerson := c.Query("sales_person")
	patientAccessFilter := ""
	patientAccessArgs := []interface{}{}
	if userID, ok := GetUserID(c); ok {
		patientAccessFilter, patientAccessArgs = patientAccessFilterForUser(db, userID, "")
	}

	// 转换为整数
	var page, pageSize int
	_, err := fmt.Sscanf(pageStr, "%d", &page)
	if err != nil {
		page = 1
	}
	_, err = fmt.Sscanf(pageSizeStr, "%d", &pageSize)
	if err != nil {
		pageSize = 10
	}

	// 计算偏移量
	offset := (page - 1) * pageSize

	// 构建SQL查询
	query := `SELECT id, patient_code, name, gender, COALESCE(id_document_type, '居民身份证'), COALESCE(NULLIF(id_document_no, ''), COALESCE(id_card, '')), COALESCE(id_card, ''), phone, birthday, address, 
					diagnosis, cancer_diameter, 
					smoking_status, detection_mode, sales_person, COALESCE(patient_source, ''), is_active, COALESCE(created_by, 0), created_at, updated_at, 
					completion_status
					FROM detect_patient WHERE is_active = ?`
	args := []interface{}{isActive}
	query, args = appendPatientKeywordFilter(query, args, keyword)

	// 添加搜索条件
	if name != "" {
		query += " AND name LIKE ?"
		args = append(args, "%"+name+"%")
	}
	if idDocumentNo != "" {
		query += " AND (id_document_no LIKE ? OR id_card LIKE ?)"
		like := "%" + idDocumentNo + "%"
		args = append(args, like, like)
	}
	if phone != "" {
		query += " AND phone LIKE ?"
		args = append(args, "%"+phone+"%")
	}
	if patientCode != "" {
		query += " AND patient_code LIKE ?"
		args = append(args, "%"+patientCode+"%")
	}
	if completionStatus != "" {
		if completionStatus == "pending" || completionStatus == "0" {
			query += " AND completion_status = 0"
		} else if completionStatus == "completed" || completionStatus == "1" {
			query += " AND completion_status = 1"
		}
		// 其他值忽略该条件
	}
	if patientAccessFilter != "" {
		query += " AND " + patientAccessFilter
		args = append(args, patientAccessArgs...)
	} else if salesPerson != "" {
		query += " AND sales_person = ?"
		args = append(args, salesPerson)
	}

	// 添加排序和分页
	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, pageSize, offset)

	// 从数据库查询患者列表，支持分页和搜索
	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("Failed to query detect_patients: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}
	defer rows.Close()

	// 遍历查询结果，收集患者数据和销售代表工号
	var detect_patients []utils.H
	var salesPersonCodes []string
	var detect_patientSalesMap []string

	for rows.Next() {
		var id, createdBy, isActive, completionStatus int
		var detect_patientCode, name, gender, idDocumentType, idDocumentNo, idCard, phone string
		var birthday, createdAt, updatedAt time.Time
		var address, diagnosis, cancerDiameter, smokingStatus, detectionMode sql.NullString
		var salesPerson, patientSource sql.NullString

		err := rows.Scan(&id, &detect_patientCode, &name, &gender, &idDocumentType, &idDocumentNo, &idCard, &phone, &birthday, &address,
			&diagnosis, &cancerDiameter,
			&smokingStatus, &detectionMode, &salesPerson, &patientSource, &isActive, &createdBy, &createdAt, &updatedAt,
			&completionStatus)
		if err != nil {
			log.Printf("Failed to scan detect_patient: %v", err)
			detect_patientSalesMap = append(detect_patientSalesMap, "")
			continue
		}

		// 计算年龄
		age := 0
		if !birthday.IsZero() {
			age = int(time.Since(birthday).Hours() / 24 / 365.25)
		}

		// 构建患者信息，使用驼峰命名，符合前端期望
		detect_patient := utils.H{
			"id":               id,
			"patientCode":      detect_patientCode,
			"name":             name,
			"gender":           gender,
			"idDocumentType":   idDocumentType,
			"idDocumentNo":     idDocumentNo,
			"id_card_type":     idDocumentType,
			"id_card":          idDocumentNo,
			"idCard":           idDocumentNo,
			"phone":            phone,
			"birthday":         birthday.Format("2006-01-02T15:04:05+08:00"),
			"age":              age,
			"isActive":         isActive,
			"completionStatus": completionStatus,
			"createdAt":        createdAt.Format("2006-01-02T15:04:05+08:00"),
			"updatedAt":        updatedAt.Format("2006-01-02T15:04:05+08:00"),
		}
		if patientSource.Valid {
			detect_patient["patientSource"] = patientSource.String
			detect_patient["patient_source"] = patientSource.String
		}

		// 处理可选字段，使用驼峰命名
		if address.Valid {
			detect_patient["address"] = address.String
		}
		if diagnosis.Valid {
			detect_patient["diagnosis"] = diagnosis.String
		}
		if cancerDiameter.Valid {
			detect_patient["cancerDiameter"] = cancerDiameter.String
		}
		if smokingStatus.Valid {
			detect_patient["smokingStatus"] = smokingStatus.String
		}
		if detectionMode.Valid {
			detect_patient["detectionMode"] = detectionMode.String
		}

		// 记录销售代表工号，兼容历史存储的用户ID。
		salesCode := ""
		if salesPerson.Valid {
			salesCode = strings.TrimSpace(salesPerson.String)
			if salesCode != "" {
				salesPersonCodes = append(salesPersonCodes, salesCode)
			}
		}
		detect_patientSalesMap = append(detect_patientSalesMap, salesCode)
		detect_patients = append(detect_patients, detect_patient)
	}

	// 批量获取销售代表信息
	salesPersonNames := getSalesPersonNames(db, salesPersonCodes)

	// 填充销售代表信息
	for i, detect_patient := range detect_patients {
		salesCode := detect_patientSalesMap[i]
		if salesCode != "" {
			name := salesPersonNames[salesCode]
			detect_patient["salesPerson"] = utils.H{
				"id":   salesCode,
				"code": salesCode,
				"name": name,
			}
		}
	}

	// 检查遍历过程中是否有错误
	if err = rows.Err(); err != nil {
		log.Printf("Error iterating detect_patients: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 查询总数
	var total int
	countQuery := "SELECT COUNT(*) FROM detect_patient WHERE is_active = ?"
	countArgs := []interface{}{isActive}
	countQuery, countArgs = appendPatientKeywordFilter(countQuery, countArgs, keyword)

	// 添加与查询相同的筛选条件
	if name != "" {
		countQuery += " AND name LIKE ?"
		countArgs = append(countArgs, "%"+name+"%")
	}
	if idDocumentNo != "" {
		countQuery += " AND (id_document_no LIKE ? OR id_card LIKE ?)"
		like := "%" + idDocumentNo + "%"
		countArgs = append(countArgs, like, like)
	}
	if phone != "" {
		countQuery += " AND phone LIKE ?"
		countArgs = append(countArgs, "%"+phone+"%")
	}
	if patientCode != "" {
		countQuery += " AND patient_code LIKE ?"
		countArgs = append(countArgs, "%"+patientCode+"%")
	}
	if completionStatus != "" {
		if completionStatus == "pending" || completionStatus == "0" {
			countQuery += " AND completion_status = 0"
		} else if completionStatus == "completed" || completionStatus == "1" {
			countQuery += " AND completion_status = 1"
		}
	}
	if patientAccessFilter != "" {
		countQuery += " AND " + patientAccessFilter
		countArgs = append(countArgs, patientAccessArgs...)
	} else if salesPerson != "" {
		countQuery += " AND sales_person = ?"
		countArgs = append(countArgs, salesPerson)
	}

	if err := db.QueryRow(countQuery, countArgs...).Scan(&total); err != nil {
		log.Printf("Failed to count detect_patients: %v", err)
		total = len(detect_patients)
	}

	// 返回患者列表
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取患者列表成功",
		Data: utils.H{
			"list":  detect_patients,
			"total": total,
		},
	})
}

// 处理获取回收站患者列表请求
func HandleListPatientsTrash(c *app.RequestContext, db *sql.DB) {
	// 从查询参数获取分页参数，默认第1页，每页10条
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("pageSize", "10")

	// 转换为整数
	var page, pageSize int
	_, err := fmt.Sscanf(pageStr, "%d", &page)
	if err != nil {
		page = 1
	}
	_, err = fmt.Sscanf(pageSizeStr, "%d", &pageSize)
	if err != nil {
		pageSize = 10
	}

	// 计算偏移量
	offset := (page - 1) * pageSize

	// 从数据库查询回收站患者列表，is_active = 0表示已删除
	rows, err := db.Query(`SELECT id, patient_code, name, gender, COALESCE(id_document_type, '居民身份证'), COALESCE(NULLIF(id_document_no, ''), COALESCE(id_card, '')), COALESCE(id_card, ''), phone, birthday, address, 
					diagnosis, cancer_diameter, 
					smoking_status, sales_person, other_info, is_active, COALESCE(created_by, 0), created_at, updated_at 
					FROM detect_patient WHERE is_active = 0 ORDER BY updated_at DESC LIMIT ? OFFSET ?`, pageSize, offset)
	if err != nil {
		log.Printf("Failed to query trash detect_patients: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}
	defer rows.Close()

	// 遍历查询结果，收集患者数据和销售代表工号
	var detect_patients []utils.H
	var salesPersonCodes []string
	var detect_patientSalesMap []string

	for rows.Next() {
		var id, createdBy, isActive int
		var detect_patientCode, name, gender, idDocumentType, idDocumentNo, idCard, phone string
		var birthday, createdAt, updatedAt time.Time
		var address, diagnosis, cancerDiameter, smokingStatus, otherInfo sql.NullString
		var salesPerson sql.NullString

		err := rows.Scan(&id, &detect_patientCode, &name, &gender, &idDocumentType, &idDocumentNo, &idCard, &phone, &birthday, &address,
			&diagnosis, &cancerDiameter,
			&smokingStatus, &salesPerson, &otherInfo, &isActive, &createdBy, &createdAt, &updatedAt)
		if err != nil {
			log.Printf("Failed to scan trash detect_patient: %v", err)
			detect_patientSalesMap = append(detect_patientSalesMap, "")
			continue
		}

		// 计算年龄
		age := 0
		if !birthday.IsZero() {
			age = int(time.Since(birthday).Hours() / 24 / 365.25)
		}

		// 构建患者信息，使用驼峰命名，符合前端期望
		detect_patient := utils.H{
			"id":             id,
			"patientCode":    detect_patientCode,
			"name":           name,
			"gender":         gender,
			"idDocumentType": idDocumentType,
			"idDocumentNo":   idDocumentNo,
			"id_card_type":   idDocumentType,
			"id_card":        idDocumentNo,
			"idCard":         idDocumentNo,
			"phone":          phone,
			"birthday":       birthday.Format("2006-01-02T15:04:05+08:00"),
			"age":            age,
			"isActive":       isActive,
			"createdAt":      createdAt.Format("2006-01-02T15:04:05+08:00"),
			"updatedAt":      updatedAt.Format("2006-01-02T15:04:05+08:00"),
		}

		// 处理可选字段，使用驼峰命名
		if address.Valid {
			detect_patient["address"] = address.String
		}

		if cancerDiameter.Valid {
			detect_patient["cancerDiameter"] = cancerDiameter.String
		}
		if smokingStatus.Valid {
			detect_patient["smokingStatus"] = smokingStatus.String
		}
		if otherInfo.Valid {
			detect_patient["otherInfo"] = otherInfo.String
		}

		// 记录销售代表工号，兼容历史存储的用户ID。
		salesCode := ""
		if salesPerson.Valid {
			salesCode = strings.TrimSpace(salesPerson.String)
			if salesCode != "" {
				salesPersonCodes = append(salesPersonCodes, salesCode)
			}
		}
		detect_patientSalesMap = append(detect_patientSalesMap, salesCode)
		detect_patients = append(detect_patients, detect_patient)
	}

	// 批量获取销售代表信息
	salesPersonNames := getSalesPersonNames(db, salesPersonCodes)

	// 填充销售代表信息
	for i, detect_patient := range detect_patients {
		salesCode := detect_patientSalesMap[i]
		if salesCode != "" {
			name := salesPersonNames[salesCode]
			detect_patient["salesPerson"] = utils.H{
				"id":   salesCode,
				"code": salesCode,
				"name": name,
			}
		}
	}

	// 检查遍历过程中是否有错误
	if err = rows.Err(); err != nil {
		log.Printf("Error iterating trash detect_patients: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 查询总数
	var total int
	if err := db.QueryRow("SELECT COUNT(*) FROM detect_patient WHERE is_active = 0").Scan(&total); err != nil {
		log.Printf("Failed to count trash detect_patients: %v", err)
		total = len(detect_patients)
	}

	// 返回回收站患者列表
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取回收站患者列表成功",
		Data: utils.H{
			"list":  detect_patients,
			"total": total,
		},
	})
}

func HandleGetPatientById(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	idParam := c.Param("id")
	var patientCode string
	var query string
	var args []interface{}

	// 外部优先使用患者编号；数字ID仅做历史兼容。
	if _, err := strconv.Atoi(idParam); err != nil {
		patientCode = idParam
		query = `SELECT id, patient_code, name, gender, COALESCE(id_document_type, '居民身份证'), COALESCE(NULLIF(id_document_no, ''), COALESCE(id_card, '')), COALESCE(id_card, ''), phone, birthday, address, 
				diagnosis, cancer_diameter, 
				smoking_status, sales_person, COALESCE(patient_source, ''), is_active, COALESCE(created_by, 0), created_at, updated_at, 
				cancer_pathology, prognosis_info, report_files, completion_status, patient_status, 
				other_info
				FROM detect_patient WHERE patient_code = ? AND is_active = 1`
		args = []interface{}{patientCode}
	} else {
		query = `SELECT id, patient_code, name, gender, COALESCE(id_document_type, '居民身份证'), COALESCE(NULLIF(id_document_no, ''), COALESCE(id_card, '')), COALESCE(id_card, ''), phone, birthday, address, 
				diagnosis, cancer_diameter, 
				smoking_status, sales_person, COALESCE(patient_source, ''), is_active, COALESCE(created_by, 0), created_at, updated_at, 
				cancer_pathology, prognosis_info, report_files, completion_status, patient_status, 
				other_info
				FROM detect_patient WHERE id = ? AND is_active = 1`
		args = []interface{}{idParam}
	}

	// 从数据库查询患者详情
	var detect_patientCode, name, gender, idDocumentType, idDocumentNo, idCard, phone string
	var birthday, createdAt, updatedAt sql.NullTime
	var address, diagnosis, cancerDiameter, smokingStatus, patientSource, cancerPathology, prognosisInfo, reportFiles, otherInfo sql.NullString
	var salesPerson sql.NullString
	var idInt, createdBy, isActive, completionStatus, detect_patientStatus int

	err := db.QueryRow(query, args...).Scan(
		&idInt, &detect_patientCode, &name, &gender, &idDocumentType, &idDocumentNo, &idCard, &phone, &birthday, &address,
		&diagnosis, &cancerDiameter,
		&smokingStatus, &salesPerson, &patientSource, &isActive, &createdBy, &createdAt, &updatedAt,
		&cancerPathology, &prognosisInfo, &reportFiles, &completionStatus, &detect_patientStatus,
		&otherInfo)

	if err != nil {
		log.Printf("Failed to query detect_patient: %v", err)
		if err == sql.ErrNoRows {
			c.JSON(consts.StatusNotFound, ApiResponse{
				Code:    404,
				Success: false,
				Message: "患者不存在",
				Data:    nil,
			})
			return
		}
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 计算年龄
	age := 0
	if birthday.Valid {
		age = int(time.Since(birthday.Time).Hours() / 24 / 365.25)
	}

	// 构建患者信息，使用驼峰命名，符合前端期望
	detect_patient := utils.H{
		"id":               detect_patientCode,
		"patientCode":      detect_patientCode,
		"name":             name,
		"gender":           gender,
		"idDocumentType":   idDocumentType,
		"idDocumentNo":     idDocumentNo,
		"id_card_type":     idDocumentType,
		"id_card":          idDocumentNo,
		"idCard":           idDocumentNo,
		"phone":            phone,
		"age":              age,
		"isActive":         isActive,
		"completionStatus": completionStatus,
		"patientStatus":    detect_patientStatus,
		"createdAt":        createdAt.Time.Format("2006-01-02T15:04:05+08:00"),
		"updatedAt":        updatedAt.Time.Format("2006-01-02T15:04:05+08:00"),
	}
	if patientSource.Valid {
		detect_patient["patientSource"] = patientSource.String
		detect_patient["patient_source"] = patientSource.String
	}

	// 处理可选字段，使用驼峰命名
	if birthday.Valid {
		detect_patient["birthday"] = birthday.Time.Format("2006-01-02T15:04:05+08:00")
	}
	if address.Valid {
		detect_patient["address"] = address.String
	}
	if diagnosis.Valid {
		detect_patient["diagnosis"] = diagnosis.String
	}
	if cancerDiameter.Valid {
		detect_patient["cancerDiameter"] = cancerDiameter.String
	}
	if smokingStatus.Valid {
		detect_patient["smokingStatus"] = smokingStatus.String
	}
	if salesPerson.Valid {
		detect_patient["salesPerson"] = buildSalesPersonInfo(db, salesPerson.String)
	}
	if cancerPathology.Valid {
		detect_patient["cancerPathology"] = cancerPathology.String
	}
	if prognosisInfo.Valid {
		detect_patient["prognosisInfo"] = prognosisInfo.String
	}
	if reportFiles.Valid {
		detect_patient["reportFiles"] = reportFiles.String
	}

	if otherInfo.Valid {
		detect_patient["otherInfo"] = otherInfo.String
	}
	detect_patient["followUps"] = employeePatientFollowUps(db, idInt)

	// 返回患者详情
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取患者详情成功",
		Data:    detect_patient,
	})
}

func HandleCheckIdCard(c *app.RequestContext, db *sql.DB) {
	// 解析查询参数
	idCard := c.Query("id_card")
	documentNo := strings.TrimSpace(c.Query("id_document_no"))
	if documentNo == "" {
		documentNo = strings.TrimSpace(idCard)
	}
	documentType := normalizePatientDocumentType(c.Query("id_document_type"))
	if documentNo == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "身份证件号不能为空",
			Data:    nil,
		})
		return
	}
	idCard = documentNo

	// 从数据库查询身份证号是否存在
	var id int
	var detect_patientCode, name, gender, phone, idDocumentType, idDocumentNo string
	var birthday, createdAt, updatedAt time.Time
	var surgeryDate, chemoStartDate sql.NullTime
	var isActive int

	err := db.QueryRow(`SELECT id, patient_code, name, gender, phone, birthday, is_active, created_at, updated_at,
				COALESCE(id_document_type, '居民身份证'), COALESCE(NULLIF(id_document_no, ''), COALESCE(id_card, '')),
				surgery_date, chemo_start_date
				FROM detect_patient WHERE `+patientDocumentWhereClause("?")+` AND is_active = 1`, documentNo, documentNo).Scan(
		&id, &detect_patientCode, &name, &gender, &phone, &birthday, &isActive, &createdAt, &updatedAt,
		&idDocumentType, &idDocumentNo,
		&surgeryDate, &chemoStartDate)

	if err == sql.ErrNoRows {
		// 身份证号不存在
		gender := ""
		birthday := ""
		if isResidentIDCard(documentType) {
			gender, birthday, _ = parseResidentIDCardInfo(documentNo)
		}
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "验证身份证件号成功",
			Data: utils.H{
				"exists":           false,
				"gender":           gender,
				"birthday":         birthday,
				"idDocumentType":   documentType,
				"idDocumentNo":     documentNo,
				"id_document_type": documentType,
				"id_document_no":   documentNo,
				"patient":          nil,
			},
		})
		return
	} else if err != nil {
		// 查询出错
		log.Printf("Failed to check id card: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 构建患者信息
	patientInfo := utils.H{
		"id":             id,
		"patientCode":    detect_patientCode,
		"name":           name,
		"gender":         gender,
		"idDocumentType": idDocumentType,
		"idDocumentNo":   idDocumentNo,
		"idCard":         idDocumentNo,
		"phone":          phone,
		"birthday":       birthday.Format("2006-01-02T15:04:05+08:00"),
		"isActive":       isActive,
		"createdAt":      createdAt.Format("2006-01-02T15:04:05+08:00"),
		"updatedAt":      updatedAt.Format("2006-01-02T15:04:05+08:00"),
	}
	if surgeryDate.Valid {
		patientInfo["surgeryDate"] = surgeryDate.Time.Format("2006-01-02")
	}
	if chemoStartDate.Valid {
		patientInfo["chemoStartDate"] = chemoStartDate.Time.Format("2006-01-02")
	}

	// 处理可选的collectionDate字段

	// 身份证号存在，返回患者信息
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "验证身份证件号成功",
		Data: utils.H{
			"exists":  true,
			"patient": patientInfo,
		},
	})
}

func normalizePatientConditionFields(patientStatus int, diagnosis, cancerDiameter string) (string, string, error) {
	diagnosis = strings.TrimSpace(diagnosis)
	cancerDiameter = strings.TrimSpace(cancerDiameter)
	if patientStatus != 0 && patientStatus != 1 {
		return "", "", fmt.Errorf("患者状态不正确")
	}
	if patientStatus == 0 {
		return "", "", nil
	}
	if diagnosis == "" {
		return "", "", fmt.Errorf("患病患者必须填写诊断")
	}
	if cancerDiameter == "" {
		return "", "", fmt.Errorf("患病患者必须填写肿瘤直径")
	}
	return diagnosis, cancerDiameter, nil
}

func HandleCreatePatient(c *app.RequestContext, db *sql.DB) {
	// 解析请求体
	var req struct {
		Name                string `json:"name" binding:"required"`
		Gender              string `json:"gender" binding:"required"`
		IdDocumentType      string `json:"idDocumentType"`
		IdDocumentTypeSnake string `json:"id_document_type"`
		IdDocumentNo        string `json:"idDocumentNo"`
		IdDocumentNoSnake   string `json:"id_document_no"`
		IdCard              string `json:"idCard"`
		IdCardSnake         string `json:"id_card"`
		Phone               string `json:"phone"`
		Birthday            string `json:"birthday" binding:"required"`
		Address             string `json:"address"`
		Diagnosis           string `json:"diagnosis"`
		CancerDiameter      string `json:"cancerDiameter"`
		CancerDiameterSnake string `json:"cancer_diameter"`
		SmokingStatus       string `json:"smokingStatus"`
		SmokingStatusSnake  string `json:"smoking_status"`
		SalesPerson         string `json:"salesPerson"`
		SalesPersonSnake    string `json:"sales_person"`
		PatientStatus       *int   `json:"patientStatus"`
		PatientStatusSnake  *int   `json:"patient_status"`
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
	documentType := normalizePatientDocumentType(req.IdDocumentType)
	if strings.TrimSpace(req.IdDocumentTypeSnake) != "" {
		documentType = normalizePatientDocumentType(req.IdDocumentTypeSnake)
	}
	documentNo := normalizePatientDocumentNo(req.IdDocumentNo)
	if documentNo == "" {
		documentNo = normalizePatientDocumentNo(req.IdDocumentNoSnake)
	}
	if documentNo == "" {
		documentNo = normalizePatientDocumentNo(req.IdCard)
	}
	if documentNo == "" {
		documentNo = normalizePatientDocumentNo(req.IdCardSnake)
	}
	if documentNo == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "身份证件号不能为空",
			Data:    nil,
		})
		return
	}
	if err := ensurePatientDocumentNoAvailable(db, documentNo, 0); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: err.Error(), Data: nil})
		return
	}
	if isResidentIDCard(documentType) {
		if gender, parsedBirthday, ok := parseResidentIDCardInfo(documentNo); ok {
			if strings.TrimSpace(req.Gender) == "" {
				req.Gender = gender
			}
			if strings.TrimSpace(req.Birthday) == "" {
				req.Birthday = parsedBirthday
			}
		} else {
			c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "居民身份证号格式不正确", Data: nil})
			return
		}
	}
	birthday, err := normalizePatientDate(req.Birthday)
	if err != nil || birthday == "" {
		if err == nil {
			err = fmt.Errorf("出生日期不能为空")
		}
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: err.Error(),
			Data:    nil,
		})
		return
	}

	// 插入患者到数据库，确保is_active字段设置为1，completion_status默认为0
	// 使用前端发送的detect_patientStatus值，前端已经处理了默认值的设置
	// 注意：前端在创建患者时会默认设置detect_patientStatus为1（患病）
	detect_patientStatus := 1
	if req.PatientStatus != nil {
		detect_patientStatus = *req.PatientStatus
	} else if req.PatientStatusSnake != nil {
		detect_patientStatus = *req.PatientStatusSnake
	}
	cancerDiameter := strings.TrimSpace(req.CancerDiameter)
	if cancerDiameter == "" {
		cancerDiameter = strings.TrimSpace(req.CancerDiameterSnake)
	}
	diagnosis, cancerDiameter, err := normalizePatientConditionFields(detect_patientStatus, req.Diagnosis, cancerDiameter)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: err.Error(), Data: nil})
		return
	}
	smokingStatus := strings.TrimSpace(req.SmokingStatus)
	if smokingStatus == "" {
		smokingStatus = strings.TrimSpace(req.SmokingStatusSnake)
	}
	salesPerson := strings.TrimSpace(req.SalesPerson)
	if salesPerson == "" {
		salesPerson = strings.TrimSpace(req.SalesPersonSnake)
	}

	// 生成患者编号
	detect_patientCode := generatePatientCode(db)

	legacyIDCard := legacyIDCardForDocument(documentType, documentNo)
	result, err := db.Exec(`INSERT INTO detect_patient (patient_code, name, gender, id_document_type, id_document_no, id_card, phone, birthday, address, 
				diagnosis, cancer_diameter, 
				smoking_status, sales_person, is_active, completion_status, patient_status, created_at, updated_at) 
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, 0, ?, NOW(), NOW())`,
		detect_patientCode, req.Name, req.Gender, documentType, documentNo, legacyIDCard, req.Phone, birthday, req.Address,
		diagnosis, cancerDiameter,
		smokingStatus, salesPerson, detect_patientStatus)
	if err != nil {
		log.Printf("Failed to create detect_patient: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 获取插入的患者ID
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

	// 返回创建的患者ID和患者编号
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "创建患者成功",
		Data: utils.H{
			"id":          id,
			"patientCode": detect_patientCode,
		},
	})
}

func HandleUpdatePatient(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数和请求体
	idParam := c.Param("id")
	id, _, err := resolvePatientID(db, idParam, false)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "患者不存在",
			Data:    nil,
		})
		return
	}

	var req struct {
		Name                string `json:"name"`
		Gender              string `json:"gender"`
		IdDocumentType      string `json:"idDocumentType"`
		IdDocumentTypeSnake string `json:"id_document_type"`
		IdDocumentNo        string `json:"idDocumentNo"`
		IdDocumentNoSnake   string `json:"id_document_no"`
		IdCard              string `json:"idCard"`
		IdCardSnake         string `json:"id_card"`
		Phone               string `json:"phone"`
		Birthday            string `json:"birthday"`
		Address             string `json:"address"`
		Diagnosis           string `json:"diagnosis"`
		CancerDiameter      string `json:"cancerDiameter"`
		CancerDiameterSnake string `json:"cancer_diameter"`
		SmokingStatus       string `json:"smokingStatus"`
		SmokingStatusSnake  string `json:"smoking_status"`
		SalesPerson         string `json:"salesPerson"`
		CompletionStatus    int    `json:"completionStatus"`
		PatientStatus       int    `json:"patientStatus"`
		// 兼容前端可能发送的蛇形命名字段
		SalesPersonSnake      string `json:"sales_person"`
		CompletionStatusSnake int    `json:"completion_status"`
		PatientStatusSnake    int    `json:"patient_status"`
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

	// 构建动态更新查询，只更新非空字段
	var setClauses []string
	var args []interface{}

	// 确定使用哪个销售代表字段值
	salesPersonValue := strings.TrimSpace(req.SalesPerson)
	if salesPersonValue == "" {
		salesPersonValue = strings.TrimSpace(req.SalesPersonSnake)
	}

	// 确定使用哪个完成状态字段值
	completionStatusValue := req.CompletionStatus
	if completionStatusValue == 0 {
		completionStatusValue = req.CompletionStatusSnake
	}

	// 确定使用哪个患者状态字段值
	detect_patientStatusValue := req.PatientStatus
	if detect_patientStatusValue == 0 {
		detect_patientStatusValue = req.PatientStatusSnake
	}

	// 只更新非空的字段，避免覆盖基本信息
	if req.Name != "" {
		setClauses = append(setClauses, "name = ?")
		args = append(args, req.Name)
	}
	if req.Gender != "" {
		setClauses = append(setClauses, "gender = ?")
		args = append(args, req.Gender)
	}
	documentTypeInput := strings.TrimSpace(req.IdDocumentType)
	if documentTypeInput == "" {
		documentTypeInput = strings.TrimSpace(req.IdDocumentTypeSnake)
	}
	documentNoInput := normalizePatientDocumentNo(req.IdDocumentNo)
	if documentNoInput == "" {
		documentNoInput = normalizePatientDocumentNo(req.IdDocumentNoSnake)
	}
	if documentNoInput == "" {
		documentNoInput = normalizePatientDocumentNo(req.IdCard)
	}
	if documentNoInput == "" {
		documentNoInput = normalizePatientDocumentNo(req.IdCardSnake)
	}
	if documentTypeInput != "" || documentNoInput != "" {
		if documentTypeInput == "" {
			var existingType string
			_ = db.QueryRow("SELECT COALESCE(id_document_type, '居民身份证') FROM detect_patient WHERE id = ?", id).Scan(&existingType)
			documentTypeInput = existingType
		}
		documentType := normalizePatientDocumentType(documentTypeInput)
		if documentNoInput == "" {
			c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "身份证件号不能为空", Data: nil})
			return
		}
		if err := ensurePatientDocumentNoAvailable(db, documentNoInput, id); err != nil {
			c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: err.Error(), Data: nil})
			return
		}
		if isResidentIDCard(documentType) {
			if gender, parsedBirthday, ok := parseResidentIDCardInfo(documentNoInput); ok {
				if req.Gender == "" {
					req.Gender = gender
					setClauses = append(setClauses, "gender = ?")
					args = append(args, gender)
				}
				if strings.TrimSpace(req.Birthday) == "" {
					req.Birthday = parsedBirthday
				}
			} else {
				c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "居民身份证号格式不正确", Data: nil})
				return
			}
		}
		setClauses = append(setClauses, "id_document_type = ?", "id_document_no = ?", "id_card = ?")
		args = append(args, documentType, documentNoInput, legacyIDCardForDocument(documentType, documentNoInput))
	}
	setClauses = append(setClauses, "phone = ?")
	args = append(args, strings.TrimSpace(req.Phone))
	if strings.TrimSpace(req.Birthday) != "" {
		birthday, err := normalizePatientDate(req.Birthday)
		if err != nil {
			c.JSON(consts.StatusBadRequest, ApiResponse{
				Code:    400,
				Success: false,
				Message: err.Error(),
				Data:    nil,
			})
			return
		}
		setClauses = append(setClauses, "birthday = ?")
		args = append(args, birthday)
	}
	if req.Address != "" {
		setClauses = append(setClauses, "address = ?")
		args = append(args, req.Address)
	}
	if req.Diagnosis != "" {
		setClauses = append(setClauses, "diagnosis = ?")
		args = append(args, req.Diagnosis)
	}
	cancerDiameterInput := strings.TrimSpace(req.CancerDiameter)
	if cancerDiameterInput == "" {
		cancerDiameterInput = strings.TrimSpace(req.CancerDiameterSnake)
	}
	if cancerDiameterInput != "" {
		setClauses = append(setClauses, "cancer_diameter = ?")
		args = append(args, cancerDiameterInput)
	}
	smokingStatusInput := strings.TrimSpace(req.SmokingStatus)
	if smokingStatusInput == "" {
		smokingStatusInput = strings.TrimSpace(req.SmokingStatusSnake)
	}
	if smokingStatusInput != "" {
		setClauses = append(setClauses, "smoking_status = ?")
		args = append(args, smokingStatusInput)
	}
	if salesPersonValue != "" {
		setClauses = append(setClauses, "sales_person = ?")
		args = append(args, salesPersonValue)
	}
	// 始终更新完成状态
	setClauses = append(setClauses, "completion_status = ?")
	args = append(args, completionStatusValue)

	// 更新患者状态
	setClauses = append(setClauses, "patient_status = ?")
	args = append(args, detect_patientStatusValue)

	// 添加更新时间
	setClauses = append(setClauses, "updated_at = NOW()")

	// 构建完整的查询
	query := `UPDATE detect_patient SET ` +
		" " +
		strings.Join(setClauses, ", ") +
		" WHERE id = ?"

	// 添加患者ID到参数
	args = append(args, id)

	// 执行更新
	_, err = db.Exec(query, args...)

	if err != nil {
		log.Printf("Failed to update detect_patient: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 更新成功后，获取更新后的患者信息
	var detect_patientCode, name, gender, idDocumentType, idDocumentNo, idCard, phone string
	var birthday, createdAt, updatedAt sql.NullTime
	var address, diagnosis, cancerDiameter, smokingStatus, patientSource, cancerPathology, prognosisInfo, reportFiles, otherInfo sql.NullString
	var salesPerson sql.NullString
	var idInt, createdBy, isActive, completionStatus, detect_patientStatus int

	err = db.QueryRow(`SELECT id, patient_code, name, gender, COALESCE(id_document_type, '居民身份证'), COALESCE(NULLIF(id_document_no, ''), COALESCE(id_card, '')), COALESCE(id_card, ''), phone, birthday, address, 
					diagnosis, cancer_diameter, 
					smoking_status, sales_person, COALESCE(patient_source, ''), is_active, COALESCE(created_by, 0), created_at, updated_at, 
					cancer_pathology, prognosis_info, report_files, completion_status, patient_status, 
					other_info
					FROM detect_patient WHERE id = ? AND is_active = 1`, id).Scan(
		&idInt, &detect_patientCode, &name, &gender, &idDocumentType, &idDocumentNo, &idCard, &phone, &birthday, &address,
		&diagnosis, &cancerDiameter,
		&smokingStatus, &salesPerson, &patientSource, &isActive, &createdBy, &createdAt, &updatedAt,
		&cancerPathology, &prognosisInfo, &reportFiles, &completionStatus, &detect_patientStatus,
		&otherInfo)

	if err != nil {
		log.Printf("Failed to query updated detect_patient: %v", err)
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "更新患者成功",
			Data: utils.H{
				"id": id,
			},
		})
		return
	}

	// 计算年龄
	age := 0
	if birthday.Valid {
		age = int(time.Since(birthday.Time).Hours() / 24 / 365.25)
	}

	// 构建患者信息，使用驼峰命名，符合前端期望
	detect_patient := utils.H{
		"id":               detect_patientCode,
		"patientCode":      detect_patientCode,
		"name":             name,
		"gender":           gender,
		"idDocumentType":   idDocumentType,
		"idDocumentNo":     idDocumentNo,
		"id_card_type":     idDocumentType,
		"id_card":          idDocumentNo,
		"idCard":           idDocumentNo,
		"phone":            phone,
		"age":              age,
		"isActive":         isActive,
		"completionStatus": completionStatus,
		"patientStatus":    detect_patientStatus,
		"createdAt":        createdAt.Time.Format("2006-01-02T15:04:05+08:00"),
		"updatedAt":        updatedAt.Time.Format("2006-01-02T15:04:05+08:00"),
	}
	if patientSource.Valid {
		detect_patient["patientSource"] = patientSource.String
		detect_patient["patient_source"] = patientSource.String
	}

	// 处理可选字段，使用驼峰命名
	if birthday.Valid {
		detect_patient["birthday"] = birthday.Time.Format("2006-01-02T15:04:05+08:00")
	}
	if address.Valid {
		detect_patient["address"] = address.String
	}
	if diagnosis.Valid {
		detect_patient["diagnosis"] = diagnosis.String
	}
	if cancerDiameter.Valid {
		detect_patient["cancerDiameter"] = cancerDiameter.String
	}
	if smokingStatus.Valid {
		detect_patient["smokingStatus"] = smokingStatus.String
	}
	if salesPerson.Valid {
		detect_patient["salesPerson"] = buildSalesPersonInfo(db, salesPerson.String)
	}
	if cancerPathology.Valid {
		detect_patient["cancerPathology"] = cancerPathology.String
	}
	if prognosisInfo.Valid {
		detect_patient["prognosisInfo"] = prognosisInfo.String
	}
	if reportFiles.Valid {
		detect_patient["reportFiles"] = reportFiles.String
	}
	if otherInfo.Valid {
		detect_patient["otherInfo"] = otherInfo.String
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "更新患者成功",
		Data:    detect_patient,
	})
}

func HandleDeletePatient(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	idParam := c.Param("id")
	id, _, err := resolvePatientID(db, idParam, false)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "患者不存在",
			Data:    nil,
		})
		return
	}

	// 执行软删除：将is_active字段设置为0
	_, err = db.Exec("UPDATE detect_patient SET is_active = 0, updated_at = NOW() WHERE id = ?", id)
	if err != nil {
		log.Printf("Failed to delete detect_patient: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 返回删除成功响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "删除患者成功",
		Data:    nil,
	})
}

// 处理恢复患者请求
func HandleRestorePatient(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	idParam := c.Param("id")
	id, _, err := resolvePatientID(db, idParam, true)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "患者不存在",
			Data:    nil,
		})
		return
	}

	// 恢复患者：将is_active字段设置为1
	_, err = db.Exec("UPDATE detect_patient SET is_active = 1, updated_at = NOW() WHERE id = ?", id)
	if err != nil {
		log.Printf("Failed to restore detect_patient: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 返回恢复成功响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "恢复患者成功",
		Data:    nil,
	})
}

// 处理彻底删除患者请求
func HandleForceDeletePatient(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	idParam := c.Param("id")
	id, _, err := resolvePatientID(db, idParam, true)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "患者不存在",
			Data:    nil,
		})
		return
	}

	// 彻底删除患者：物理删除记录
	_, err = db.Exec("DELETE FROM detect_patient WHERE id = ?", id)
	if err != nil {
		log.Printf("Failed to force delete detect_patient: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 返回彻底删除成功响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "彻底删除患者成功",
		Data:    nil,
	})
}

// 处理文件上传请求
func HandleUploadFile(c *app.RequestContext, db *sql.DB) {
	detect_patientIdentifier := c.PostForm("patient_code")
	if detect_patientIdentifier == "" {
		detect_patientIdentifier = c.PostForm("patient_id")
	}
	if detect_patientIdentifier == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "患者编号不能为空",
			Data:    nil,
		})
		return
	}

	detect_patientId, detect_patientCode, err := resolvePatientID(db, detect_patientIdentifier, false)
	if err != nil {
		log.Printf("Failed to get detect_patient code: %v", err)
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "患者不存在",
			Data:    nil,
		})
		return
	}

	// 获取上传的文件
	fileHeader, err := c.FormFile("file")
	if err != nil {
		log.Printf("Failed to get file: %v", err)
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请选择要上传的文件",
			Data:    nil,
		})
		return
	}

	// 检查文件大小（限制为10MB）
	const maxFileSize = 10 * 1024 * 1024 // 10MB
	if fileHeader.Size > maxFileSize {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "文件大小不能超过10MB",
			Data:    nil,
		})
		return
	}

	// 打开文件
	file, err := fileHeader.Open()
	if err != nil {
		log.Printf("Failed to open file: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "文件打开失败",
			Data:    nil,
		})
		return
	}
	defer file.Close()

	// 检查文件类型
	allowedExtensions := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".pdf":  true,
		".doc":  true,
		".docx": true,
	}

	ext := filepath.Ext(fileHeader.Filename)
	if !allowedExtensions[ext] {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "只允许上传JPG、PNG、PDF、DOC、DOCX格式的文件",
			Data:    nil,
		})
		return
	}

	// 创建患者文件目录
	baseDir := "file"
	detect_patientDir := filepath.Join(baseDir, "patient", detect_patientCode)
	if err := os.MkdirAll(detect_patientDir, 0755); err != nil {
		log.Printf("Failed to create patient directory: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 生成唯一文件名
	fileName := fmt.Sprintf("%s_%d%s", detect_patientCode, time.Now().Unix(), ext)
	filePath := filepath.Join(detect_patientDir, fileName)

	// 创建目标文件
	dst, err := os.Create(filePath)
	if err != nil {
		log.Printf("Failed to create file: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}
	defer dst.Close()

	// 复制文件内容
	if _, err := io.Copy(dst, file); err != nil {
		log.Printf("Failed to copy file content: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 获取当前报告文件列表
	var reportFiles string
	err = db.QueryRow("SELECT report_files FROM detect_patient WHERE id = ?", detect_patientId).Scan(&reportFiles)
	if err != nil {
		reportFiles = ""
	}

	// 更新报告文件路径
	var updatedReportFiles string
	if reportFiles == "" {
		updatedReportFiles = filePath
	} else {
		updatedReportFiles = reportFiles + "," + filePath
	}

	_, err = db.Exec("UPDATE detect_patient SET report_files = ?, updated_at = NOW() WHERE id = ?", updatedReportFiles, detect_patientId)
	if err != nil {
		log.Printf("Failed to update report files: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 返回上传成功响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "文件上传成功",
		Data: utils.H{
			"file_path":    filePath,
			"file_name":    fileName,
			"patient_code": detect_patientCode,
		},
	})
}

// HandleDeletePatientReportFile 从患者资料中删除一个独立报告文件。
func HandleDeletePatientReportFile(c *app.RequestContext, db *sql.DB) {
	patientIdentifier := strings.TrimSpace(c.Param("id"))
	patientID, patientCode, err := resolvePatientID(db, patientIdentifier, false)
	if err != nil {
		c.JSON(consts.StatusNotFound, ApiResponse{Code: 404, Success: false, Message: "患者不存在", Data: nil})
		return
	}
	var request struct {
		FileURL string `json:"file_url"`
	}
	body, err := c.Body()
	if err != nil || json.Unmarshal(body, &request) != nil || strings.TrimSpace(request.FileURL) == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "报告文件地址不能为空", Data: nil})
		return
	}
	request.FileURL = strings.TrimSpace(request.FileURL)

	tx, err := db.Begin()
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "删除报告失败", Data: nil})
		return
	}
	defer tx.Rollback()

	var reportFiles sql.NullString
	if err := tx.QueryRow(`SELECT report_files FROM detect_patient WHERE id = ? FOR UPDATE`, patientID).Scan(&reportFiles); err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "读取报告文件失败", Data: nil})
		return
	}
	files := splitPatientReportFiles(reportFiles.String)
	remaining := make([]string, 0, len(files))
	found := false
	for _, file := range files {
		if file == request.FileURL {
			found = true
			continue
		}
		remaining = append(remaining, file)
	}
	if !found {
		c.JSON(consts.StatusNotFound, ApiResponse{Code: 404, Success: false, Message: "报告文件不存在", Data: nil})
		return
	}

	if _, err := tx.Exec(`UPDATE detect_patient SET report_files = ?, updated_at = NOW() WHERE id = ?`,
		strings.Join(remaining, ","), patientID); err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "更新患者报告失败", Data: nil})
		return
	}
	if _, err := tx.Exec(`DELETE FROM base_files_patient
		WHERE patient_id = ? AND (file_url = ? OR file_path = ? OR storage_path = ?)`,
		patientID, request.FileURL, request.FileURL, request.FileURL); err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "删除文件索引失败", Data: nil})
		return
	}

	type followUpFiles struct {
		ID     int
		Images string
	}
	followUps := make([]followUpFiles, 0)
	rows, queryErr := tx.Query(`SELECT id, COALESCE(images_json, '') FROM patient_follow_up
		WHERE patient_id = ? AND images_json LIKE ?`, patientID, "%"+request.FileURL+"%")
	if queryErr != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "读取随访报告失败", Data: nil})
		return
	}
	for rows.Next() {
		var item followUpFiles
		if err := rows.Scan(&item.ID, &item.Images); err != nil {
			_ = rows.Close()
			c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "读取随访报告失败", Data: nil})
			return
		}
		followUps = append(followUps, item)
	}
	if err := rows.Close(); err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "读取随访报告失败", Data: nil})
		return
	}
	for _, followUp := range followUps {
		var images []string
		if json.Unmarshal([]byte(followUp.Images), &images) != nil {
			continue
		}
		filtered := make([]string, 0, len(images))
		for _, image := range images {
			if strings.TrimSpace(image) != request.FileURL {
				filtered = append(filtered, image)
			}
		}
		encoded, _ := json.Marshal(filtered)
		if _, err := tx.Exec(`UPDATE patient_follow_up SET images_json = ?, updated_at = NOW() WHERE id = ?`, string(encoded), followUp.ID); err != nil {
			c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "更新随访报告失败", Data: nil})
			return
		}
	}
	if err := tx.Commit(); err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "删除报告失败", Data: nil})
		return
	}

	cleanupWarning := ""
	qiniuConfig := loadQiniuStorageConfig()
	if objectName, ok := qiniuObjectNameFromURL(qiniuConfig, request.FileURL); ok {
		expectedPrefix := "uploads/patient_report/" + strings.ToUpper(patientCode) + "/"
		if strings.HasPrefix(objectName, expectedPrefix) {
			if err := deleteFileFromQiniu(objectName, qiniuConfig); err != nil {
				cleanupWarning = "报告记录已删除，云端文件清理失败"
				log.Printf("Failed to delete Qiniu patient report %s: %v", objectName, err)
			}
		}
	} else if localPath, ok := legacyPatientReportLocalPath(request.FileURL); ok {
		if err := os.Remove(localPath); err != nil && !os.IsNotExist(err) {
			cleanupWarning = "报告记录已删除，旧文件清理失败"
			log.Printf("Failed to delete legacy patient report %s: %v", localPath, err)
		}
	}
	c.JSON(consts.StatusOK, ApiResponse{
		Code: 200, Success: true, Message: "报告删除成功",
		Data: utils.H{"report_files": remaining, "cleanup_warning": cleanupWarning},
	})
}
