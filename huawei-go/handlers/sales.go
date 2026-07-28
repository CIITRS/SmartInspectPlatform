package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func isAdminRoleName(roleName string) bool {
	return strings.Contains(roleName, "管理员") || strings.Contains(roleName, "IT")
}

func getCurrentRoleName(db *sql.DB, c *app.RequestContext) (int, string, error) {
	userIDValue, exists := c.Get(UserIDKey)
	if !exists {
		return 0, "", fmt.Errorf("未授权")
	}
	userID, err := strconv.Atoi(fmt.Sprint(userIDValue))
	if err != nil || userID <= 0 {
		return 0, "", fmt.Errorf("未授权")
	}
	var roleName string
	if err := db.QueryRow(`SELECT COALESCE(r.name, '') FROM base_manage_user u
		LEFT JOIN setting_role r ON u.role_id = r.id WHERE u.id = ?`, userID).Scan(&roleName); err != nil {
		return 0, "", err
	}
	return userID, roleName, nil
}

func requireSalesAssignmentAdmin(c *app.RequestContext, db *sql.DB) bool {
	_, roleName, err := getCurrentRoleName(db, c)
	if err != nil {
		c.JSON(consts.StatusUnauthorized, ApiResponse{Code: 401, Success: false, Message: "未授权", Data: nil})
		return false
	}
	if !isAdminRoleName(roleName) {
		c.JSON(consts.StatusForbidden, ApiResponse{Code: 403, Success: false, Message: "仅管理员可分配销售", Data: nil})
		return false
	}
	return true
}

// HandleListSalesAssignmentPatients 查询自主注册且尚未分配销售的患者。
func HandleListSalesAssignmentPatients(c *app.RequestContext, db *sql.DB) {
	if !requireSalesAssignmentAdmin(c, db) {
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize
	keyword := strings.TrimSpace(c.Query("keyword"))

	query := `SELECT id, patient_code, COALESCE(name, ''), COALESCE(gender, ''), COALESCE(id_card, ''), COALESCE(phone, ''), created_at
		FROM detect_patient
		WHERE is_active = 1
			AND COALESCE(NULLIF(TRIM(sales_person), ''), '') = ''
			AND (COALESCE(patient_source, '') = 'miniapp_self' OR COALESCE(NULLIF(TRIM(sales_person), ''), '') = '')`
	args := []interface{}{}
	if keyword != "" {
		query += ` AND (name LIKE ? OR phone LIKE ? OR id_card LIKE ? OR patient_code LIKE ?)`
		like := "%" + keyword + "%"
		args = append(args, like, like, like, like)
	}
	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, pageSize, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("Failed to query sales assignment patients: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "查询待分配患者失败", Data: nil})
		return
	}
	defer rows.Close()

	list := []utils.H{}
	for rows.Next() {
		var id int
		var patientCode, name, gender, idCard, phone string
		var createdAt time.Time
		if err := rows.Scan(&id, &patientCode, &name, &gender, &idCard, &phone, &createdAt); err != nil {
			log.Printf("Failed to scan sales assignment patient: %v", err)
			continue
		}
		list = append(list, utils.H{
			"id":             id,
			"patientCode":    patientCode,
			"patient_code":   patientCode,
			"name":           name,
			"gender":         gender,
			"idCard":         idCard,
			"id_card":        idCard,
			"phone":          phone,
			"patientSource":  "miniapp_self",
			"patient_source": "miniapp_self",
			"createdAt":      createdAt,
			"created_at":     createdAt,
		})
	}

	countQuery := `SELECT COUNT(*)
		FROM detect_patient
		WHERE is_active = 1
			AND COALESCE(NULLIF(TRIM(sales_person), ''), '') = ''
			AND (COALESCE(patient_source, '') = 'miniapp_self' OR COALESCE(NULLIF(TRIM(sales_person), ''), '') = '')`
	countArgs := []interface{}{}
	if keyword != "" {
		countQuery += ` AND (name LIKE ? OR phone LIKE ? OR id_card LIKE ? OR patient_code LIKE ?)`
		like := "%" + keyword + "%"
		countArgs = append(countArgs, like, like, like, like)
	}
	total := len(list)
	if err := db.QueryRow(countQuery, countArgs...).Scan(&total); err != nil {
		log.Printf("Failed to count sales assignment patients: %v", err)
	}

	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "获取待分配患者成功", Data: utils.H{"list": list, "total": total}})
}

// HandleAssignSalesToSelfRegisteredPatient 为自主注册患者分配销售，但保留来源字段。
func HandleAssignSalesToSelfRegisteredPatient(c *app.RequestContext, db *sql.DB) {
	if !requireSalesAssignmentAdmin(c, db) {
		return
	}

	var req struct {
		PatientID        int    `json:"patient_id"`
		PatientIDAlias   int    `json:"patientId"`
		PatientIDs       []int  `json:"patient_ids"`
		PatientIDsAlias  []int  `json:"patientIds"`
		SalesPersonCode  string `json:"sales_person"`
		SalesPersonAlias string `json:"salesPerson"`
	}
	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "请求参数错误", Data: nil})
		return
	}
	if req.PatientID == 0 {
		req.PatientID = req.PatientIDAlias
	}
	if len(req.PatientIDs) == 0 {
		req.PatientIDs = req.PatientIDsAlias
	}
	if len(req.PatientIDs) == 0 && req.PatientID > 0 {
		req.PatientIDs = []int{req.PatientID}
	}
	if req.SalesPersonCode == "" {
		req.SalesPersonCode = req.SalesPersonAlias
	}
	req.SalesPersonCode = strings.TrimSpace(req.SalesPersonCode)
	if len(req.PatientIDs) == 0 || req.SalesPersonCode == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "请选择患者和销售", Data: nil})
		return
	}

	var salesName, roleName string
	err := db.QueryRow(`SELECT COALESCE(NULLIF(u.real_name, ''), u.username), COALESCE(r.name, '')
		FROM base_manage_user u
		LEFT JOIN setting_role r ON u.role_id = r.id
		WHERE u.status = 1 AND u.employee_id = ? LIMIT 1`, req.SalesPersonCode).Scan(&salesName, &roleName)
	if err == sql.ErrNoRows || !strings.Contains(roleName, "销售") {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "请选择有效销售人员", Data: nil})
		return
	}
	if err != nil {
		log.Printf("Failed to validate sales person: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "校验销售失败", Data: nil})
		return
	}

	tx, err := db.Begin()
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "分配销售失败", Data: nil})
		return
	}
	defer tx.Rollback()
	var affected int64
	for _, patientID := range req.PatientIDs {
		if patientID <= 0 {
			continue
		}
		result, err := tx.Exec(`UPDATE detect_patient
			SET sales_person = ?, updated_at = NOW()
			WHERE id = ?
				AND is_active = 1
				AND COALESCE(NULLIF(TRIM(sales_person), ''), '') = ''`,
			req.SalesPersonCode, patientID)
		if err != nil {
			log.Printf("Failed to assign sales person: %v", err)
			c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "分配销售失败", Data: nil})
			return
		}
		rowAffected, _ := result.RowsAffected()
		affected += rowAffected
	}
	if affected == 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "患者不存在或已分配销售", Data: nil})
		return
	}
	if err := tx.Commit(); err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "分配销售失败", Data: nil})
		return
	}

	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "分配销售成功", Data: utils.H{
		"patient_ids":       req.PatientIDs,
		"affected":          affected,
		"sales_person":      req.SalesPersonCode,
		"sales_person_name": salesName,
	}})
}

// 套餐管理相关API

// 处理获取套餐列表请求
func HandleListPackages(c *app.RequestContext, db *sql.DB) {
	// 从数据库查询套餐列表
	rows, err := db.Query(`SELECT id, name, detection_count, interval_days, price, description, status, created_at, updated_at FROM sale_package ORDER BY created_at DESC`)
	if err != nil {
		log.Printf("Failed to query sale_packages: %v", err)
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "获取套餐列表成功",
			Data:    utils.H{"list": []utils.H{}, "total": 0},
		})
		return
	}
	defer rows.Close()

	// 遍历查询结果
	var sale_packages []utils.H
	for rows.Next() {
		var id int
		var name, description, status string
		var detectionCount, intervalDays int
		var price float64
		var createdAt, updatedAt time.Time

		err := rows.Scan(&id, &name, &detectionCount, &intervalDays, &price, &description, &status, &createdAt, &updatedAt)
		if err != nil {
			log.Printf("Failed to scan sale_package: %v", err)
			continue
		}

		sale_packages = append(sale_packages, utils.H{
			"id":             id,
			"name":           name,
			"detectionCount": detectionCount,
			"intervalDays":   intervalDays,
			"price":          price,
			"description":    description,
			"status":         status,
			"createdAt":      createdAt,
			"updatedAt":      updatedAt,
		})
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取套餐列表成功",
		Data:    utils.H{"list": sale_packages, "total": len(sale_packages)},
	})
}

// 处理创建套餐请求
func HandleCreatePackage(c *app.RequestContext, db *sql.DB) {
	// 解析请求体
	var req struct {
		Name           string  `json:"name" binding:"required"`
		DetectionCount int     `json:"detectionCount" binding:"required"`
		IntervalDays   int     `json:"intervalDays" binding:"required"`
		Price          float64 `json:"price" binding:"required"`
		Description    string  `json:"description"`
		Status         string  `json:"status"`
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

	// 设置默认状态
	status := req.Status
	if status == "" {
		status = "active"
	}

	// 创建套餐
	result, err := db.Exec(`INSERT INTO sale_package (name, detection_count, interval_days, price, description, status) VALUES (?, ?, ?, ?, ?, ?)`,
		req.Name, req.DetectionCount, req.IntervalDays, req.Price, req.Description, status)
	if err != nil {
		log.Printf("Failed to create sale_package: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "创建套餐失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 获取新创建的套餐ID
	id, err := result.LastInsertId()
	if err != nil {
		log.Printf("Failed to get last insert id: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "创建套餐失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "创建套餐成功",
		Data:    utils.H{"id": id},
	})
}

// 处理更新套餐请求
func HandleUpdatePackage(c *app.RequestContext, db *sql.DB) {
	// 解析请求体
	var req struct {
		ID             int     `json:"id" binding:"required"`
		Name           string  `json:"name" binding:"required"`
		DetectionCount int     `json:"detectionCount" binding:"required"`
		IntervalDays   int     `json:"intervalDays" binding:"required"`
		Price          float64 `json:"price" binding:"required"`
		Description    string  `json:"description"`
		Status         string  `json:"status" binding:"required"`
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

	// 更新套餐
	_, err := db.Exec(`UPDATE sale_package SET name = ?, detection_count = ?, interval_days = ?, price = ?, description = ?, status = ?, updated_at = NOW() WHERE id = ?`,
		req.Name, req.DetectionCount, req.IntervalDays, req.Price, req.Description, req.Status, req.ID)
	if err != nil {
		log.Printf("Failed to update sale_package: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "更新套餐失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "更新套餐成功",
		Data:    nil,
	})
}

// HandleListPatientPackages 查询当前销售绑定给患者的套餐。
func HandleListPatientPackages(c *app.RequestContext, db *sql.DB) {
	userID, exists := c.Get(UserIDKey)
	if !exists {
		c.JSON(consts.StatusUnauthorized, ApiResponse{Code: 401, Success: false, Message: "未授权", Data: nil})
		return
	}
	var roleName string
	_ = db.QueryRow(`SELECT COALESCE(r.name, '') FROM base_manage_user u
		LEFT JOIN setting_role r ON u.role_id = r.id WHERE u.id = ?`, userID).Scan(&roleName)

	query := `SELECT o.id, o.sale_order_no, o.detect_patient_id, COALESCE(p.name, ''), COALESCE(p.phone, ''),
		o.sale_package_id, COALESCE(pk.name, ''), o.setting_cancer_type_id, COALESCE(ct.name, ''),
		o.first_detection_date, o.payment_status, o.status, o.sales_person_id, COALESCE(u.real_name, u.username), o.created_at
		FROM sale_order o
		LEFT JOIN detect_patient p ON o.detect_patient_id = p.id
		LEFT JOIN sale_package pk ON o.sale_package_id = pk.id
		LEFT JOIN setting_cancer_type ct ON o.setting_cancer_type_id = ct.id
		LEFT JOIN base_manage_user u ON o.sales_person_id = u.id`
	args := []interface{}{}
	if !isAdminRoleName(roleName) {
		query += ` WHERE o.sales_person_id = ?`
		args = append(args, userID)
	}
	query += ` ORDER BY o.created_at DESC`

	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("Failed to query patient packages: %v", err)
		c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "获取绑定套餐成功", Data: utils.H{"list": []utils.H{}, "total": 0}})
		return
	}
	defer rows.Close()

	list := []utils.H{}
	for rows.Next() {
		var id, patientID, packageID, cancerTypeID, salesPersonID int
		var orderNo, patientName, patientPhone, packageName, cancerTypeName, paymentStatus, status, salesPersonName string
		var firstDate sql.NullString
		var createdAt time.Time
		if err := rows.Scan(&id, &orderNo, &patientID, &patientName, &patientPhone, &packageID, &packageName, &cancerTypeID, &cancerTypeName, &firstDate, &paymentStatus, &status, &salesPersonID, &salesPersonName, &createdAt); err != nil {
			log.Printf("Failed to scan patient package: %v", err)
			continue
		}
		list = append(list, utils.H{
			"id":                 id,
			"sale_orderNo":       orderNo,
			"patientId":          patientID,
			"patientName":        patientName,
			"patientPhone":       patientPhone,
			"packageId":          packageID,
			"packageName":        packageName,
			"cancerTypeId":       cancerTypeID,
			"cancerTypeName":     cancerTypeName,
			"firstDetectionDate": firstDate.String,
			"paymentStatus":      paymentStatus,
			"status":             status,
			"salesPersonId":      salesPersonID,
			"salesPersonName":    salesPersonName,
			"createdAt":          createdAt,
		})
	}
	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "获取绑定套餐成功", Data: utils.H{"list": list, "total": len(list)}})
}

// HandleBindPatientPackage 由销售为患者绑定套餐，并自动生成检测计划。
func HandleBindPatientPackage(c *app.RequestContext, db *sql.DB) {
	var req struct {
		PatientID          int    `json:"patient_id"`
		PatientIdCard      string `json:"patientIdCard"`
		PackageID          int    `json:"package_id"`
		PackageIdAlias     int    `json:"packageId"`
		CancerTypeID       int    `json:"cancer_type_id"`
		CancerTypeIdAlias  int    `json:"cancerTypeId"`
		FirstDetectionDate string `json:"first_detection_date"`
		FirstDateAlias     string `json:"firstDetectionDate"`
		PaymentMethod      string `json:"payment_method"`
		PaymentMethodAlias string `json:"paymentMethod"`
		PaymentStatus      string `json:"payment_status"`
	}
	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "请求参数错误", Data: utils.H{"error": err.Error()}})
		return
	}
	if req.PackageID == 0 {
		req.PackageID = req.PackageIdAlias
	}
	if req.CancerTypeID == 0 {
		req.CancerTypeID = req.CancerTypeIdAlias
	}
	if req.FirstDetectionDate == "" {
		req.FirstDetectionDate = req.FirstDateAlias
	}
	if req.PaymentMethod == "" {
		req.PaymentMethod = req.PaymentMethodAlias
	}
	if req.PaymentStatus == "" {
		req.PaymentStatus = "paid"
	}
	if req.PatientID == 0 && strings.TrimSpace(req.PatientIdCard) != "" {
		_ = db.QueryRow("SELECT id FROM detect_patient WHERE id_card = ? AND is_active = 1 LIMIT 1", strings.TrimSpace(req.PatientIdCard)).Scan(&req.PatientID)
	}
	if req.PatientID == 0 || req.PackageID == 0 || req.CancerTypeID == 0 || req.FirstDetectionDate == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "患者、套餐、检测类型和首次检测日期不能为空", Data: nil})
		return
	}

	userID, exists := c.Get(UserIDKey)
	if !exists {
		c.JSON(consts.StatusUnauthorized, ApiResponse{Code: 401, Success: false, Message: "未授权", Data: nil})
		return
	}
	salesPersonID := userID.(int)

	var patientIdCard string
	if err := db.QueryRow("SELECT id_card FROM detect_patient WHERE id = ? AND is_active = 1", req.PatientID).Scan(&patientIdCard); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "患者不存在", Data: nil})
		return
	}
	var detectionCount, intervalDays int
	var price float64
	if err := db.QueryRow(`SELECT detection_count, interval_days, price FROM sale_package WHERE id = ? AND status = 'active'`, req.PackageID).Scan(&detectionCount, &intervalDays, &price); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "套餐不存在或已停用", Data: nil})
		return
	}
	firstDate, err := time.Parse("2006-01-02", req.FirstDetectionDate)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "首次检测日期格式应为 YYYY-MM-DD", Data: nil})
		return
	}

	tx, err := db.Begin()
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "绑定套餐失败", Data: nil})
		return
	}
	defer tx.Rollback()

	orderNo := generateOrderNo()
	result, err := tx.Exec(`INSERT INTO sale_order
		(sale_order_no, detect_patient_id, detect_patient_id_card, sale_package_id, setting_cancer_type_id,
		first_detection_date, payment_method, payment_status, sales_person_id, total_amount, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'active')`,
		orderNo, req.PatientID, patientIdCard, req.PackageID, req.CancerTypeID, req.FirstDetectionDate,
		req.PaymentMethod, req.PaymentStatus, salesPersonID, price)
	if err != nil {
		log.Printf("Failed to bind patient package: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "绑定套餐失败", Data: nil})
		return
	}
	orderID, _ := result.LastInsertId()
	for i := 0; i < detectionCount; i++ {
		date := firstDate.AddDate(0, 0, i*intervalDays).Format("2006-01-02")
		if _, err := tx.Exec(`INSERT INTO sale_detection_plan
			(sale_order_id, detect_patient_id, detection_date, detection_number, status)
			VALUES (?, ?, ?, ?, 'scheduled')`, orderID, req.PatientID, date, i+1); err != nil {
			c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "生成检测计划失败", Data: nil})
			return
		}
	}
	if err := tx.Commit(); err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "绑定套餐失败", Data: nil})
		return
	}
	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "套餐绑定成功", Data: utils.H{"sale_orderId": orderID, "sale_orderNo": orderNo}})
}

// 订单管理相关API

// 生成订单编号
func generateOrderNo() string {
	timeStr := time.Now().Format("20060102150405")
	randomStr := fmt.Sprintf("%06d", rand.Intn(1000000))
	return "ORD" + timeStr + randomStr
}

// 处理创建订单请求
func HandleCreateOrder(c *app.RequestContext, db *sql.DB) {
	// 解析请求体
	var req struct {
		PatientIdCard      string `json:"detect_patientIdCard"`
		PatientIdCardAlias string `json:"patientIdCard"`
		PackageId          int    `json:"sale_packageId"`
		PackageIdAlias     int    `json:"packageId"`
		CancerTypeId       int    `json:"cancerTypeId" binding:"required"`
		FirstDetectionDate string `json:"firstDetectionDate" binding:"required"`
		PaymentMethod      string `json:"paymentMethod" binding:"required"`
		SurgeryDate        string `json:"surgeryDate"`
		ChemoStartDate     string `json:"chemoStartDate"`
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

	if req.PatientIdCard == "" {
		req.PatientIdCard = req.PatientIdCardAlias
	}
	if req.PackageId == 0 {
		req.PackageId = req.PackageIdAlias
	}
	if req.PatientIdCard == "" || req.PackageId == 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "患者身份证号和套餐不能为空",
			Data:    nil,
		})
		return
	}

	// 从上下文中获取用户ID（销售人员）
	userID, exists := c.Get(UserIDKey)
	if !exists {
		c.JSON(consts.StatusUnauthorized, ApiResponse{
			Code:    401,
			Success: false,
			Message: "未授权",
			Data:    nil,
		})
		return
	}
	salesPersonId := userID.(int)

	// 查询患者信息
	var detect_patientId int
	err := db.QueryRow(`SELECT id FROM detect_patient WHERE id_card = ?`, req.PatientIdCard).Scan(&detect_patientId)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "患者不存在",
			Data:    nil,
		})
		return
	}

	// 查询套餐信息
	var sale_packageName string
	var detectionCount, intervalDays int
	var price float64
	err = db.QueryRow(`SELECT name, detection_count, interval_days, price FROM sale_package WHERE id = ? AND status = 'active'`, req.PackageId).Scan(&sale_packageName, &detectionCount, &intervalDays, &price)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "套餐不存在或已停用",
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
			Message: "创建订单失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 生成订单编号
	sale_orderNo := generateOrderNo()

	// 创建订单
	var sale_orderId int64
	result, err := tx.Exec("INSERT INTO `sale_order` (sale_order_no, detect_patient_id, detect_patient_id_card, sale_package_id, setting_cancer_type_id, first_detection_date, payment_method, payment_status, sales_person_id, total_amount, status) VALUES (?, ?, ?, ?, ?, ?, ?, 'pending', ?, ?, 'pending')",
		sale_orderNo, detect_patientId, req.PatientIdCard, req.PackageId, req.CancerTypeId, req.FirstDetectionDate, req.PaymentMethod, salesPersonId, price)
	if err != nil {
		tx.Rollback()
		log.Printf("Failed to create sale_order: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "创建订单失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	if req.SurgeryDate != "" || req.ChemoStartDate != "" {
		_, err := tx.Exec(`UPDATE detect_patient SET
			surgery_date = COALESCE(NULLIF(?, ''), surgery_date),
			chemo_start_date = COALESCE(NULLIF(?, ''), chemo_start_date),
			updated_at = NOW()
			WHERE id = ?`, req.SurgeryDate, req.ChemoStartDate, detect_patientId)
		if err != nil {
			tx.Rollback()
			log.Printf("Failed to update patient treatment dates: %v", err)
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code:    500,
				Success: false,
				Message: "保存患者治疗日期失败",
				Data:    utils.H{"error": err.Error()},
			})
			return
		}
	}

	sale_orderId, err = result.LastInsertId()
	if err != nil {
		tx.Rollback()
		log.Printf("Failed to get last insert id: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "创建订单失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 解析第一次检测日期
	firstDate, err := time.Parse("2006-01-02", req.FirstDetectionDate)
	if err != nil {
		tx.Rollback()
		log.Printf("Failed to parse first detection date: %v", err)
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "日期格式错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 创建检测计划
	for i := 0; i < detectionCount; i++ {
		detectionDate := firstDate.AddDate(0, 0, i*intervalDays)
		_, err := tx.Exec(`INSERT INTO sale_detection_plan (sale_order_id, detect_patient_id, detection_date, detection_number, status) VALUES (?, ?, ?, ?, 'scheduled')`,
			sale_orderId, detect_patientId, detectionDate.Format("2006-01-02"), i+1)
		if err != nil {
			tx.Rollback()
			log.Printf("Failed to create detection plan: %v", err)
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code:    500,
				Success: false,
				Message: "创建检测计划失败",
				Data:    utils.H{"error": err.Error()},
			})
			return
		}
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		log.Printf("Failed to commit transaction: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "创建订单失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "创建订单成功",
		Data:    utils.H{"sale_orderId": sale_orderId, "sale_orderNo": sale_orderNo},
	})
}

// 处理获取订单列表请求
func HandleListOrders(c *app.RequestContext, db *sql.DB) {
	// 从上下文中获取用户ID和角色
	userID, exists := c.Get(UserIDKey)
	if !exists {
		c.JSON(consts.StatusUnauthorized, ApiResponse{
			Code:    401,
			Success: false,
			Message: "未授权",
			Data:    nil,
		})
		return
	}

	// 根据用户ID查询用户角色信息
	var roleName string
	err := db.QueryRow(`SELECT COALESCE(r.name, '') FROM base_manage_user u
		LEFT JOIN setting_role r ON u.role_id = r.id WHERE u.id = ?`, userID).Scan(&roleName)
	if err != nil {
		log.Printf("Failed to get user role: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "获取用户角色失败",
			Data:    nil,
		})
		return
	}

	// 构建查询语句
	var query string
	var args []interface{}

	if isAdminRoleName(roleName) {
		// 管理员可以查看所有订单
		query = "SELECT o.id, o.sale_order_no, o.detect_patient_id, o.detect_patient_id_card, o.sale_package_id, o.setting_cancer_type_id, o.first_detection_date, o.payment_method, o.payment_status, o.sales_person_id, o.total_amount, o.status, o.created_at, o.updated_at, p.name as detect_patientName, pk.name as sale_packageName, ct.name as cancerTypeName, su.username as salesPersonName FROM `sale_order` o LEFT JOIN detect_patient p ON o.detect_patient_id = p.id LEFT JOIN sale_package pk ON o.sale_package_id = pk.id LEFT JOIN setting_cancer_type ct ON o.setting_cancer_type_id = ct.id LEFT JOIN base_manage_user su ON o.sales_person_id = su.id ORDER BY o.created_at DESC"
	} else {
		// 销售人员只能查看自己的订单
		query = "SELECT o.id, o.sale_order_no, o.detect_patient_id, o.detect_patient_id_card, o.sale_package_id, o.setting_cancer_type_id, o.first_detection_date, o.payment_method, o.payment_status, o.sales_person_id, o.total_amount, o.status, o.created_at, o.updated_at, p.name as detect_patientName, pk.name as sale_packageName, ct.name as cancerTypeName, su.username as salesPersonName FROM `sale_order` o LEFT JOIN detect_patient p ON o.detect_patient_id = p.id LEFT JOIN sale_package pk ON o.sale_package_id = pk.id LEFT JOIN setting_cancer_type ct ON o.setting_cancer_type_id = ct.id LEFT JOIN base_manage_user su ON o.sales_person_id = su.id WHERE o.sales_person_id = ? ORDER BY o.created_at DESC"
		args = append(args, userID)
	}

	// 执行查询
	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("Failed to query sale_orders: %v", err)
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "获取订单列表成功",
			Data:    utils.H{"list": []utils.H{}, "total": 0},
		})
		return
	}
	defer rows.Close()

	// 遍历查询结果
	var sale_orders []utils.H
	for rows.Next() {
		var id, sale_packageId, cancerTypeId, salesPersonId int
		var sale_orderNo, detect_patientIdCard, paymentMethod, paymentStatus, status, detect_patientName, sale_packageName, cancerTypeName, salesPersonName string
		var detect_patientId sql.NullInt64
		var totalAmount float64
		var firstDetectionDate sql.NullString
		var createdAt, updatedAt time.Time

		err := rows.Scan(&id, &sale_orderNo, &detect_patientId, &detect_patientIdCard, &sale_packageId, &cancerTypeId, &firstDetectionDate, &paymentMethod, &paymentStatus, &salesPersonId, &totalAmount, &status, &createdAt, &updatedAt, &detect_patientName, &sale_packageName, &cancerTypeName, &salesPersonName)
		if err != nil {
			log.Printf("Failed to scan sale_order: %v", err)
			continue
		}

		sale_orders = append(sale_orders, utils.H{
			"id":                   id,
			"sale_orderNo":         sale_orderNo,
			"detect_patientId":     detect_patientId.Int64,
			"detect_patientIdCard": detect_patientIdCard,
			"detect_patientName":   detect_patientName,
			"sale_packageId":       sale_packageId,
			"sale_packageName":     sale_packageName,
			"cancerTypeId":         cancerTypeId,
			"cancerTypeName":       cancerTypeName,
			"firstDetectionDate":   firstDetectionDate.String,
			"paymentMethod":        paymentMethod,
			"paymentStatus":        paymentStatus,
			"salesPersonId":        salesPersonId,
			"salesPersonName":      salesPersonName,
			"totalAmount":          totalAmount,
			"status":               status,
			"createdAt":            createdAt,
			"updatedAt":            updatedAt,
		})
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取订单列表成功",
		Data:    utils.H{"list": sale_orders, "total": len(sale_orders)},
	})
}

// 检测计划管理相关API

// 处理获取检测计划列表请求
func HandleListDetectionPlans(c *app.RequestContext, db *sql.DB) {
	// 解析订单ID参数
	sale_orderIdStr := c.Query("sale_orderId")
	if sale_orderIdStr == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "订单ID不能为空",
			Data:    nil,
		})
		return
	}

	sale_orderId, err := strconv.Atoi(sale_orderIdStr)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的订单ID",
			Data:    nil,
		})
		return
	}

	// 查询检测计划
	rows, err := db.Query(`SELECT id, sale_order_id, detect_patient_id, detection_date, detection_number, status, created_at, updated_at FROM sale_detection_plan WHERE sale_order_id = ? ORDER BY detection_number`, sale_orderId)
	if err != nil {
		log.Printf("Failed to query detection plans: %v", err)
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "获取检测计划成功",
			Data:    utils.H{"list": []utils.H{}, "total": 0},
		})
		return
	}
	defer rows.Close()

	// 遍历查询结果
	var plans []utils.H
	for rows.Next() {
		var id, sale_orderId, detect_patientId, detectionNumber int
		var detectionDate, status string
		var createdAt, updatedAt time.Time

		err := rows.Scan(&id, &sale_orderId, &detect_patientId, &detectionDate, &detectionNumber, &status, &createdAt, &updatedAt)
		if err != nil {
			log.Printf("Failed to scan detection plan: %v", err)
			continue
		}

		plans = append(plans, utils.H{
			"id":               id,
			"sale_orderId":     sale_orderId,
			"detect_patientId": detect_patientId,
			"detectionDate":    detectionDate,
			"detectionNumber":  detectionNumber,
			"status":           status,
			"createdAt":        createdAt,
			"updatedAt":        updatedAt,
		})
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取检测计划成功",
		Data:    utils.H{"list": plans, "total": len(plans)},
	})
}

// 处理更新检测计划请求
func HandleUpdateDetectionPlan(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的检测计划ID",
			Data:    nil,
		})
		return
	}

	// 解析请求体
	var req struct {
		DetectionDate string `json:"detectionDate" binding:"required"`
		Status        string `json:"status"`
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

	// 更新检测计划
	query := `UPDATE sale_detection_plan SET detection_date = ?`
	var args []interface{}
	args = append(args, req.DetectionDate)

	if req.Status != "" {
		query += `, status = ?`
		args = append(args, req.Status)
	}

	query += ` WHERE id = ?`
	args = append(args, id)

	_, err = db.Exec(query, args...)
	if err != nil {
		log.Printf("Failed to update detection plan: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "更新检测计划失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "更新检测计划成功",
		Data:    nil,
	})
}

// 销售统计相关API

// 处理获取销售统计请求
func HandleGetSalesStatistics(c *app.RequestContext, db *sql.DB) {
	// 从上下文中获取用户ID和角色
	userID, exists := c.Get(UserIDKey)
	if !exists {
		c.JSON(consts.StatusUnauthorized, ApiResponse{
			Code:    401,
			Success: false,
			Message: "未授权",
			Data:    nil,
		})
		return
	}

	// 根据用户ID查询用户角色信息
	var roleName string
	err := db.QueryRow(`SELECT COALESCE(r.name, '') FROM base_manage_user u
		LEFT JOIN setting_role r ON u.role_id = r.id WHERE u.id = ?`, userID).Scan(&roleName)
	if err != nil {
		log.Printf("Failed to get user role: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "获取用户角色失败",
			Data:    nil,
		})
		return
	}

	// 构建查询语句
	var query string
	var args []interface{}

	if isAdminRoleName(roleName) {
		// 管理员可以查看所有销售统计
		query = "SELECT su.id as salesPersonId, su.username as salesPersonName, COUNT(o.id) as sale_orderCount, SUM(o.total_amount) as totalAmount FROM `sale_order` o JOIN base_manage_user su ON o.sales_person_id = su.id WHERE o.status != 'cancelled' GROUP BY su.id, su.username ORDER BY totalAmount DESC"
	} else {
		// 销售人员只能查看自己的销售统计
		query = "SELECT su.id as salesPersonId, su.username as salesPersonName, COUNT(o.id) as sale_orderCount, SUM(o.total_amount) as totalAmount FROM `sale_order` o JOIN base_manage_user su ON o.sales_person_id = su.id WHERE o.sales_person_id = ? AND o.status != 'cancelled' GROUP BY su.id, su.username"
		args = append(args, userID)
	}

	// 执行查询
	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("Failed to query sales statistics: %v", err)
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "获取销售统计成功",
			Data:    utils.H{"list": []utils.H{}, "total": 0},
		})
		return
	}
	defer rows.Close()

	// 遍历查询结果
	var statistics []utils.H
	var totalOrderCount int
	var totalSalesAmount float64

	for rows.Next() {
		var salesPersonId int
		var salesPersonName string
		var sale_orderCount int
		var totalAmount float64

		err := rows.Scan(&salesPersonId, &salesPersonName, &sale_orderCount, &totalAmount)
		if err != nil {
			log.Printf("Failed to scan sales statistics: %v", err)
			continue
		}

		statistics = append(statistics, utils.H{
			"salesPersonId":   salesPersonId,
			"salesPersonName": salesPersonName,
			"sale_orderCount": sale_orderCount,
			"totalAmount":     totalAmount,
		})

		totalOrderCount += sale_orderCount
		totalSalesAmount += totalAmount
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取销售统计成功",
		Data: utils.H{
			"list":             statistics,
			"total":            len(statistics),
			"totalOrderCount":  totalOrderCount,
			"totalSalesAmount": totalSalesAmount,
		},
	})
}
