package handlers

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/tealeg/xlsx"
)

func normalizeMailExpressCompany(value string) string {
	value = strings.TrimSpace(value)
	switch value {
	case "", "顺丰", "SF", "sf":
		return "顺丰速运"
	case "京东", "京东物流":
		return "京东快递"
	default:
		return value
	}
}

func mailStatusText(status string) string {
	switch status {
	case "shipped":
		return "已邮寄"
	default:
		return "待邮寄"
	}
}

func scanMailAddressRows(rows *sql.Rows) ([]utils.H, error) {
	list := []utils.H{}
	for rows.Next() {
		var id, patientID int
		var orderID, planID sql.NullInt64
		var receiverName, receiverPhone, province, city, district, detailAddress, fullAddress sql.NullString
		var expressCompany, trackingNumber, status, notes sql.NullString
		var patientName, patientCode, patientPhone, orderNo, packageName, cancerType, detectionMode sql.NullString
		var detectionNumber, detectionCount sql.NullInt64
		var detectionDate, shippedAt, createdAt, updatedAt sql.NullTime
		if err := rows.Scan(
			&id, &patientID, &orderID, &planID, &receiverName, &receiverPhone, &province, &city, &district,
			&detailAddress, &fullAddress, &expressCompany, &trackingNumber, &status, &notes, &shippedAt, &createdAt, &updatedAt,
			&patientName, &patientCode, &patientPhone, &orderNo, &packageName, &detectionNumber, &detectionDate,
			&detectionCount, &cancerType, &detectionMode,
		); err != nil {
			return nil, err
		}
		item := utils.H{
			"id":              id,
			"patient_id":      patientID,
			"receiver_name":   receiverName.String,
			"receiver_phone":  receiverPhone.String,
			"province":        province.String,
			"city":            city.String,
			"district":        district.String,
			"detail_address":  detailAddress.String,
			"full_address":    fullAddress.String,
			"express_company": expressCompany.String,
			"tracking_number": trackingNumber.String,
			"status":          status.String,
			"status_text":     mailStatusText(status.String),
			"notes":           notes.String,
			"patient_name":    patientName.String,
			"patient_code":    patientCode.String,
			"patient_phone":   patientPhone.String,
			"order_no":        orderNo.String,
			"package_name":    packageName.String,
			"cancer_type":     cancerType.String,
			"detection_mode":  detectionMode.String,
		}
		if orderID.Valid {
			item["order_id"] = int(orderID.Int64)
		}
		if planID.Valid {
			item["plan_id"] = int(planID.Int64)
		}
		if detectionNumber.Valid {
			item["detection_number"] = int(detectionNumber.Int64)
		}
		if detectionCount.Valid {
			item["detection_count"] = int(detectionCount.Int64)
		}
		if detectionNumber.Valid && detectionCount.Valid {
			item["package_progress"] = fmt.Sprintf("%d次联检 · 第%d次", detectionCount.Int64, detectionNumber.Int64)
		}
		if detectionDate.Valid {
			item["detection_date"] = detectionDate.Time.Format("2006-01-02")
		}
		if shippedAt.Valid {
			item["shipped_at"] = shippedAt.Time.Format("2006-01-02 15:04:05")
		}
		if createdAt.Valid {
			item["created_at"] = createdAt.Time.Format("2006-01-02 15:04:05")
		}
		if updatedAt.Valid {
			item["updated_at"] = updatedAt.Time.Format("2006-01-02 15:04:05")
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

func refreshMailAppointmentTracking(db *sql.DB, appointmentID int) {
	var company, trackingNumber, receiverPhone, status sql.NullString
	var lastQueryAt sql.NullTime
	if err := db.QueryRow(`SELECT express_company, tracking_number, receiver_phone, express_status, express_last_query_at
		FROM mail_address WHERE id = ?`, appointmentID).Scan(&company, &trackingNumber, &receiverPhone, &status, &lastQueryAt); err != nil {
		return
	}
	if strings.TrimSpace(trackingNumber.String) == "" || status.String == "delivered" ||
		(lastQueryAt.Valid && time.Since(lastQueryAt.Time) < time.Hour) {
		return
	}
	result, err := queryExpressProvider(nil, expressConfig(), "auto", trackingNumber.String, receiverPhone.String)
	if err != nil {
		_, _ = db.Exec(`UPDATE mail_address SET express_last_query_at = NOW(), express_last_query_error = ?, updated_at = NOW() WHERE id = ?`,
			truncateExpressText(err.Error(), 500), appointmentID)
		return
	}
	var routeJSON interface{}
	if result.Status != "delivered" && len(result.Route) > 0 {
		encoded, _ := json.Marshal(result.Route)
		routeJSON = string(encoded)
	}
	var deliveredAt interface{}
	if result.DeliveredAt != nil {
		deliveredAt = *result.DeliveredAt
	}
	_, _ = db.Exec(`UPDATE mail_address SET express_company = COALESCE(NULLIF(?, ''), express_company),
		express_status = ?, express_route_json = ?, express_delivered_at = ?, express_last_query_at = NOW(),
		express_last_query_error = '', updated_at = NOW() WHERE id = ?`,
		result.CompanyName, result.Status, routeJSON, deliveredAt, appointmentID)
}

func enrichMailAppointmentTracking(db *sql.DB, list []utils.H) {
	for _, item := range list {
		id, _ := item["id"].(int)
		if id <= 0 || strings.TrimSpace(fmt.Sprint(item["tracking_number"])) == "" {
			continue
		}
		refreshMailAppointmentTracking(db, id)
		var status, routeJSON, lastError sql.NullString
		var deliveredAt sql.NullTime
		if err := db.QueryRow(`SELECT express_status, express_route_json, express_delivered_at, express_last_query_error
			FROM mail_address WHERE id = ?`, id).Scan(&status, &routeJSON, &deliveredAt, &lastError); err != nil {
			continue
		}
		item["express_status"] = firstNonEmptyString(status.String, "pending")
		item["express_last_query_error"] = lastError.String
		item["express_route"] = []expressProviderEvent{}
		if status.String != "delivered" && routeJSON.Valid {
			var route []expressProviderEvent
			if json.Unmarshal([]byte(routeJSON.String), &route) == nil {
				item["express_route"] = route
			}
		}
		if deliveredAt.Valid {
			item["express_delivered_at"] = deliveredAt.Time.Format("2006-01-02 15:04:05")
		}
	}
}

func HandleAdminListMailAppointments(c *app.RequestContext, db *sql.DB) {
	current, _ := strconv.Atoi(c.Query("current"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize"))
	if current <= 0 {
		current = 1
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 20
	}
	offset := (current - 1) * pageSize

	status := strings.TrimSpace(c.Query("status"))
	keyword := strings.TrimSpace(c.Query("keyword"))
	conditions := []string{"1=1"}
	args := []interface{}{}
	if userID, ok := GetUserID(c); ok {
		roleNames := getUserRoleNames(db, userID)
		if hasRoleName(roleNames, "销售") && !hasRoleName(roleNames, "管理员", "IT") {
			salesCode := salesPersonCodeForUser(db, userID)
			conditions = append(conditions, "p.sales_person = ?")
			args = append(args, salesCode)
		}
	}
	if status != "" {
		conditions = append(conditions, "ma.status = ?")
		args = append(args, status)
	}
	if keyword != "" {
		conditions = append(conditions, "(p.name LIKE ? OR p.patient_code LIKE ? OR p.phone LIKE ? OR ma.receiver_name LIKE ? OR ma.receiver_phone LIKE ? OR ma.tracking_number LIKE ?)")
		like := "%" + keyword + "%"
		args = append(args, like, like, like, like, like, like)
	}
	whereSQL := strings.Join(conditions, " AND ")

	var total int
	countArgs := append([]interface{}{}, args...)
	if err := db.QueryRow(`SELECT COUNT(*)
		FROM mail_address ma
		JOIN detect_patient p ON ma.detect_patient_id = p.id
		LEFT JOIN sale_order so ON ma.sale_order_id = so.id
		LEFT JOIN sale_package sp ON so.sale_package_id = sp.id
		WHERE `+whereSQL, countArgs...).Scan(&total); err != nil {
		log.Printf("Count mail appointments error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "查询失败", Data: nil})
		return
	}

	queryArgs := append([]interface{}{}, args...)
	queryArgs = append(queryArgs, pageSize, offset)
	rows, err := db.Query(`SELECT ma.id, ma.detect_patient_id, ma.sale_order_id, ma.detection_plan_id,
			ma.receiver_name, ma.receiver_phone, ma.province, ma.city, ma.district, ma.detail_address, ma.full_address,
			ma.express_company, ma.tracking_number, ma.status, ma.notes, ma.shipped_at, ma.created_at, ma.updated_at,
			p.name, p.patient_code, p.phone, so.sale_order_no, sp.name, dp.detection_number, dp.detection_date,
			sp.detection_count, ct.name, p.detection_mode
		FROM mail_address ma
		JOIN detect_patient p ON ma.detect_patient_id = p.id
		LEFT JOIN sale_order so ON ma.sale_order_id = so.id
		LEFT JOIN sale_package sp ON so.sale_package_id = sp.id
		LEFT JOIN sale_detection_plan dp ON ma.detection_plan_id = dp.id
		LEFT JOIN setting_cancer_type ct ON so.setting_cancer_type_id = ct.id
		WHERE `+whereSQL+`
		ORDER BY ma.created_at DESC
		LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		log.Printf("List mail appointments error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "查询失败", Data: nil})
		return
	}
	defer rows.Close()

	list, err := scanMailAddressRows(rows)
	if err != nil {
		log.Printf("Scan mail appointments error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "查询失败", Data: nil})
		return
	}
	enrichMailAppointmentTracking(db, list)

	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "获取成功", Data: utils.H{"list": list, "total": total}})
}

func sampleLocationText(sampleStatus, expressStatus, direction string) string {
	if expressStatus != "" && expressStatus != "delivered" {
		if direction == expressDirectionOutbound {
			return "报告/物料寄往患者途中"
		}
		return "患者样本寄往实验室途中"
	}
	switch sampleStatus {
	case "created", "collected":
		return "患者处，待寄回"
	case "received":
		return "实验室已签收"
	case "testing":
		return "实验室检测中"
	case "tested", "completed":
		return "检测已完成"
	default:
		return "待确认"
	}
}

// HandleAdminListSampleLogistics 聚合展示患者样本当前所在环节。
func HandleAdminListSampleLogistics(c *app.RequestContext, db *sql.DB) {
	current, _ := strconv.Atoi(c.DefaultQuery("current", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if current <= 0 {
		current = 1
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 20
	}
	conditions := []string{
		"COALESCE(s.sample_status, '') NOT IN ('received', 'processing', 'tested', 'completed')",
		"COALESCE(e.direction, 'inbound') = 'inbound'",
		"COALESCE(TRIM(e.tracking_number), '') <> ''",
	}
	args := []interface{}{}
	if userID, ok := GetUserID(c); ok {
		roleNames := getUserRoleNames(db, userID)
		if hasRoleName(roleNames, "销售") && !hasRoleName(roleNames, "管理员", "IT") {
			conditions = append(conditions, "p.sales_person = ?")
			args = append(args, salesPersonCodeForUser(db, userID))
		}
	}
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		conditions = append(conditions, "(p.name LIKE ? OR p.patient_code LIKE ? OR p.phone LIKE ? OR s.sample_code LIKE ? OR e.tracking_number LIKE ?)")
		like := "%" + keyword + "%"
		args = append(args, like, like, like, like, like)
	}
	whereSQL := strings.Join(conditions, " AND ")
	joinSQL := ` FROM detect_sample s
		JOIN detect_patient p ON p.id = s.patient_id
		LEFT JOIN detect_sample_express e ON e.id = (
			SELECT e2.id FROM detect_sample_express e2
			WHERE e2.sample_id = s.id ORDER BY e2.updated_at DESC, e2.id DESC LIMIT 1
		)`
	var total int
	if err := db.QueryRow(`SELECT COUNT(*)`+joinSQL+` WHERE `+whereSQL, args...).Scan(&total); err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "查询样本物流失败", Data: nil})
		return
	}
	queryArgs := append([]interface{}{}, args...)
	queryArgs = append(queryArgs, pageSize, (current-1)*pageSize)
	rows, err := db.Query(`SELECT s.id, s.sample_code, COALESCE(s.sample_status, ''), s.sample_created_at,
			p.name, p.patient_code, p.phone,
			COALESCE(e.id, 0), COALESCE(e.direction, ''), COALESCE(e.express_company, ''),
			COALESCE(e.tracking_number, ''), COALESCE(e.status, ''), COALESCE(e.latest_event_status, ''),
			e.delivered_at, e.updated_at
		`+joinSQL+` WHERE `+whereSQL+`
		ORDER BY s.sample_created_at DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		log.Printf("List sample logistics error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "查询样本物流失败", Data: nil})
		return
	}
	defer rows.Close()
	list := []utils.H{}
	for rows.Next() {
		var sampleID, expressID int
		var sampleCode, sampleStatus, patientName, patientCode, patientPhone string
		var direction, company, trackingNumber, expressStatus, latestEvent string
		var createdAt time.Time
		var deliveredAt, expressUpdatedAt sql.NullTime
		if err := rows.Scan(&sampleID, &sampleCode, &sampleStatus, &createdAt,
			&patientName, &patientCode, &patientPhone, &expressID, &direction, &company,
			&trackingNumber, &expressStatus, &latestEvent, &deliveredAt, &expressUpdatedAt); err != nil {
			continue
		}
		item := utils.H{
			"id": sampleID, "sample_code": sampleCode, "sample_status": sampleStatus,
			"patient_name": patientName, "patient_code": patientCode, "patient_phone": patientPhone,
			"express_id": expressID, "direction": direction, "express_company": company,
			"tracking_number": trackingNumber, "express_status": expressStatus,
			"latest_event_status": latestEvent,
			"current_location":    sampleLocationText(sampleStatus, expressStatus, direction),
			"created_at":          createdAt.Format("2006-01-02 15:04:05"),
		}
		if deliveredAt.Valid {
			item["delivered_at"] = deliveredAt.Time.Format("2006-01-02 15:04:05")
		}
		if expressUpdatedAt.Valid {
			item["express_updated_at"] = expressUpdatedAt.Time.Format("2006-01-02 15:04:05")
		}
		list = append(list, item)
	}
	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "获取成功", Data: utils.H{"list": list, "total": total}})
}

func HandleAdminUpdateMailAppointment(c *app.RequestContext, db *sql.DB) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "预约ID无效", Data: nil})
		return
	}
	var req struct {
		ExpressCompany string `json:"express_company"`
		TrackingNumber string `json:"tracking_number"`
		Status         string `json:"status"`
		Notes          string `json:"notes"`
	}
	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "请求参数错误", Data: utils.H{"error": err.Error()}})
		return
	}
	status := strings.TrimSpace(req.Status)
	if strings.TrimSpace(req.TrackingNumber) != "" {
		status = "shipped"
	}
	if status != "shipped" {
		status = "requested"
	}
	_, err = db.Exec(`UPDATE mail_address
		SET express_company = ?, tracking_number = ?, status = ?, notes = ?,
			express_status = CASE WHEN ? <> '' THEN 'pending' ELSE express_status END,
			express_route_json = CASE WHEN ? <> '' THEN NULL ELSE express_route_json END,
			express_delivered_at = CASE WHEN ? <> '' THEN NULL ELSE express_delivered_at END,
			express_last_query_at = CASE WHEN ? <> '' THEN NULL ELSE express_last_query_at END,
			express_last_query_error = CASE WHEN ? <> '' THEN '' ELSE express_last_query_error END,
			shipped_at = CASE WHEN ? = 'shipped' AND shipped_at IS NULL THEN NOW() ELSE shipped_at END,
			updated_at = NOW()
		WHERE id = ?`,
		normalizeMailExpressCompany(req.ExpressCompany), strings.TrimSpace(req.TrackingNumber), status, req.Notes,
		strings.TrimSpace(req.TrackingNumber), strings.TrimSpace(req.TrackingNumber), strings.TrimSpace(req.TrackingNumber),
		strings.TrimSpace(req.TrackingNumber), strings.TrimSpace(req.TrackingNumber), status, id)
	if err != nil {
		log.Printf("Update mail appointment error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "保存失败", Data: utils.H{"error": err.Error()}})
		return
	}
	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "保存成功", Data: nil})
}

type appointmentTrackingImportRow struct {
	AppointmentID  string
	PatientCode    string
	PatientPhone   string
	ReceiverPhone  string
	ExpressCompany string
	TrackingNumber string
}

func normalizeAppointmentHeader(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.TrimPrefix(value, "\ufeff")
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "_", "")
	value = strings.ReplaceAll(value, "-", "")
	switch value {
	case "预约id", "预约编号", "id", "appointmentid", "mailaddressid":
		return "appointment_id"
	case "患者编号", "患者id", "patientcode":
		return "patient_code"
	case "患者电话", "患者手机号", "手机号", "联系电话", "patientphone", "phone":
		return "patient_phone"
	case "收件人电话", "收件电话", "receiverphone":
		return "receiver_phone"
	case "快递公司", "物流公司", "expresscompany":
		return "express_company"
	case "运单号", "快递单号", "物流单号", "trackingnumber", "trackingno":
		return "tracking_number"
	default:
		return value
	}
}

func appointmentTrackingRowsFromTable(records [][]string) []appointmentTrackingImportRow {
	rows := []appointmentTrackingImportRow{}
	if len(records) == 0 {
		return rows
	}
	headerIndexes := map[string]int{}
	for idx, header := range records[0] {
		headerIndexes[normalizeAppointmentHeader(header)] = idx
	}
	get := func(record []string, key string) string {
		idx, ok := headerIndexes[key]
		if !ok || idx < 0 || idx >= len(record) {
			return ""
		}
		return strings.TrimSpace(record[idx])
	}
	for _, record := range records[1:] {
		row := appointmentTrackingImportRow{
			AppointmentID:  get(record, "appointment_id"),
			PatientCode:    get(record, "patient_code"),
			PatientPhone:   get(record, "patient_phone"),
			ReceiverPhone:  get(record, "receiver_phone"),
			ExpressCompany: get(record, "express_company"),
			TrackingNumber: get(record, "tracking_number"),
		}
		if row.AppointmentID == "" && row.PatientCode == "" && row.PatientPhone == "" && row.ReceiverPhone == "" && row.TrackingNumber == "" {
			continue
		}
		rows = append(rows, row)
	}
	return rows
}

func parseAppointmentTrackingImport(file io.Reader, fileSize int64, filename string) ([]appointmentTrackingImportRow, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == ".csv" {
		reader := csv.NewReader(file)
		reader.FieldsPerRecord = -1
		reader.TrimLeadingSpace = true
		records, err := reader.ReadAll()
		if err != nil {
			return nil, err
		}
		return appointmentTrackingRowsFromTable(records), nil
	}

	readerAt, ok := file.(io.ReaderAt)
	if !ok {
		return nil, fmt.Errorf("文件读取器不支持Excel解析")
	}
	xlFile, err := xlsx.OpenReaderAt(readerAt, fileSize)
	if err != nil {
		return nil, err
	}
	if len(xlFile.Sheets) == 0 {
		return nil, fmt.Errorf("Excel中没有工作表")
	}
	records := [][]string{}
	for _, row := range xlFile.Sheets[0].Rows {
		values := []string{}
		for _, cell := range row.Cells {
			values = append(values, strings.TrimSpace(cell.String()))
		}
		records = append(records, values)
	}
	return appointmentTrackingRowsFromTable(records), nil
}

func findAppointmentIDForTracking(db *sql.DB, row appointmentTrackingImportRow) (int, error) {
	if row.AppointmentID != "" {
		id, err := strconv.Atoi(row.AppointmentID)
		if err != nil || id <= 0 {
			return 0, fmt.Errorf("预约ID无效")
		}
		var exists int
		if err := db.QueryRow(`SELECT COUNT(*) FROM mail_address WHERE id = ?`, id).Scan(&exists); err != nil {
			return 0, err
		}
		if exists == 0 {
			return 0, fmt.Errorf("预约ID不存在")
		}
		return id, nil
	}

	conditions := []string{}
	args := []interface{}{}
	if row.PatientCode != "" {
		conditions = append(conditions, "p.patient_code = ?")
		args = append(args, row.PatientCode)
	}
	if row.PatientPhone != "" {
		conditions = append(conditions, "p.phone = ?")
		args = append(args, row.PatientPhone)
	}
	if row.ReceiverPhone != "" {
		conditions = append(conditions, "ma.receiver_phone = ?")
		args = append(args, row.ReceiverPhone)
	}
	if len(conditions) == 0 {
		return 0, fmt.Errorf("缺少预约ID或患者识别信息")
	}
	query := `SELECT ma.id
		FROM mail_address ma
		JOIN detect_patient p ON ma.detect_patient_id = p.id
		WHERE ` + strings.Join(conditions, " AND ") + `
		ORDER BY CASE WHEN ma.status = 'requested' THEN 0 ELSE 1 END, ma.created_at DESC
		LIMIT 1`
	var id int
	if err := db.QueryRow(query, args...).Scan(&id); err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("未找到匹配预约")
		}
		return 0, err
	}
	return id, nil
}

func HandleAdminUploadAppointmentTracking(c *app.RequestContext, db *sql.DB) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "请选择要上传的文件", Data: nil})
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "文件打开失败", Data: nil})
		return
	}
	defer file.Close()

	rows, err := parseAppointmentTrackingImport(file, fileHeader.Size, fileHeader.Filename)
	if err != nil {
		log.Printf("Parse appointment tracking import error: %v", err)
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "文件格式错误，请上传CSV或Excel文件", Data: utils.H{"error": err.Error()}})
		return
	}
	if len(rows) == 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "文件中没有可导入的数据", Data: nil})
		return
	}

	successCount := 0
	failed := []utils.H{}
	for index, row := range rows {
		rowNo := index + 2
		if strings.TrimSpace(row.TrackingNumber) == "" {
			failed = append(failed, utils.H{"row": rowNo, "reason": "运单号为空"})
			continue
		}
		id, err := findAppointmentIDForTracking(db, row)
		if err != nil {
			failed = append(failed, utils.H{"row": rowNo, "reason": err.Error()})
			continue
		}
		_, err = db.Exec(`UPDATE mail_address
			SET express_company = ?, tracking_number = ?, status = 'shipped',
				shipped_at = CASE WHEN shipped_at IS NULL THEN NOW() ELSE shipped_at END,
				updated_at = NOW()
			WHERE id = ?`,
			normalizeMailExpressCompany(row.ExpressCompany), strings.TrimSpace(row.TrackingNumber), id)
		if err != nil {
			failed = append(failed, utils.H{"row": rowNo, "reason": "保存失败: " + err.Error()})
			continue
		}
		successCount++
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: fmt.Sprintf("导入完成，成功%d条，失败%d条", successCount, len(failed)),
		Data: utils.H{
			"success_count": successCount,
			"failed_count":  len(failed),
			"failed":        failed,
		},
	})
}
