package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func getSalesManager(db *sql.DB, salesID int) (utils.H, error) {
	var id int
	var username, realName, phone, employeeID string
	err := db.QueryRow(`SELECT id, username, real_name, phone, COALESCE(employee_id, '') FROM base_manage_user WHERE id = ? AND status = 1`, salesID).
		Scan(&id, &username, &realName, &phone, &employeeID)
	if err != nil {
		return nil, err
	}
	displayName := realName
	if displayName == "" {
		displayName = username
	}
	return utils.H{
		"id":          id,
		"username":    username,
		"real_name":   realName,
		"name":        displayName,
		"phone":       phone,
		"employee_id": employeeID,
	}, nil
}

func generateInviteQRCode(salesID int) (string, error) {
	accessToken, err := getWechatAccessToken()
	if err != nil {
		return "", err
	}

	reqBody := utils.H{
		"scene":       fmt.Sprintf("sales_id=%d", salesID),
		"page":        "pages/invite/index",
		"check_path":  false,
		"env_version": getRuntimeSetting("WECHAT_QRCODE_ENV_VERSION", "WECHAT_QRCODE_ENV_VERSION", "release"),
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	apiURL := fmt.Sprintf("https://api.weixin.qq.com/wxa/getwxacodeunlimit?access_token=%s", accessToken)
	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") || bytes.HasPrefix(respBytes, []byte("{")) {
		var result struct {
			Errcode int    `json:"errcode"`
			Errmsg  string `json:"errmsg"`
		}
		if err := json.Unmarshal(respBytes, &result); err == nil && result.Errcode != 0 {
			return "", fmt.Errorf("生成小程序码失败: %d - %s", result.Errcode, result.Errmsg)
		}
		return "", fmt.Errorf("生成小程序码失败")
	}

	dir := filepath.Join("static", "invite-qrcodes")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	fileName := fmt.Sprintf("sales_%d.png", salesID)
	filePath := filepath.Join(dir, fileName)
	if err := os.WriteFile(filePath, respBytes, 0644); err != nil {
		return "", err
	}

	return "/invite-qrcodes/" + fileName, nil
}

// HandleUniEmployeeInviteCode 返回当前员工的邀请页路径和小程序码。
func HandleUniEmployeeInviteCode(c *app.RequestContext, db *sql.DB) {
	salesID := getMiniappEmployeeID(c, db)
	if salesID <= 0 {
		c.JSON(consts.StatusUnauthorized, ApiResponse{Code: 401, Success: false, Message: "未识别到员工身份", Data: nil})
		return
	}

	manager, err := getSalesManager(db, salesID)
	if err != nil {
		c.JSON(consts.StatusNotFound, ApiResponse{Code: 404, Success: false, Message: "未找到客户经理信息", Data: nil})
		return
	}

	invitePath := fmt.Sprintf("pages/invite/index?sales_id=%d", salesID)
	qrcodeURL := ""
	if isWechatConfigured() {
		if url, err := generateInviteQRCode(salesID); err != nil {
			log.Printf("Generate invite qrcode error: %v", err)
		} else {
			qrcodeURL = url
		}
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取邀请码成功",
		Data: utils.H{
			"sales_id":     salesID,
			"manager":      manager,
			"invite_path":  invitePath,
			"invite_query": fmt.Sprintf("sales_id=%d", salesID),
			"qrcode_url":   qrcodeURL,
		},
	})
}

// HandleGetInviteManager 获取邀请页底部展示的专属客户经理。
func HandleGetInviteManager(c *app.RequestContext, db *sql.DB) {
	idStr := string(c.Query("sales_id"))
	salesID, _ := strconv.Atoi(idStr)
	if salesID <= 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "邀请参数无效", Data: nil})
		return
	}
	manager, err := getSalesManager(db, salesID)
	if err != nil {
		c.JSON(consts.StatusNotFound, ApiResponse{Code: 404, Success: false, Message: "客户经理不存在或已停用", Data: nil})
		return
	}
	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "获取成功", Data: manager})
}

type inviteRegisterPayload struct {
	SalesID                 int    `json:"sales_id"`
	Name                    string `json:"name"`
	IdCard                  string `json:"id_card"`
	IDDocumentType          string `json:"id_document_type"`
	IDDocumentNo            string `json:"id_document_no"`
	Gender                  string `json:"gender"`
	Birthday                string `json:"birthday"`
	Address                 string `json:"address"`
	DetectionMode           string `json:"detection_mode"`
	Diagnosis               string `json:"diagnosis"`
	CancerDiameter          string `json:"cancer_diameter"`
	SmokingStatus           string `json:"smoking_status"`
	PatientStatus           *int   `json:"patient_status"`
	SmsCode                 string `json:"sms_code"`
	Code                    string `json:"code"`
	Phone                   string `json:"phone"`
	JsCode                  string `json:"jsCode"`
	ReportSubscribeAccepted bool   `json:"report_subscribe_accepted"`
}

func createOrUpdateInvitedPatient(db *sql.DB, phone string, req inviteRegisterPayload, salesCode string) (int, string, error) {
	documentType := normalizePatientDocumentType(req.IDDocumentType)
	documentNo := normalizePatientDocumentNo(req.IDDocumentNo)
	if documentNo == "" {
		documentNo = normalizePatientDocumentNo(req.IdCard)
	}
	if documentNo == "" {
		return 0, "", fmt.Errorf("身份证件号不能为空")
	}
	gender := ""
	birthday := ""
	if isResidentIDCard(documentType) {
		var ok bool
		gender, birthday, ok = parseResidentIDCardInfo(documentNo)
		if !ok {
			return 0, "", fmt.Errorf("居民身份证号格式不正确")
		}
	}
	if strings.TrimSpace(req.Gender) != "" {
		gender = strings.TrimSpace(req.Gender)
	}
	if strings.TrimSpace(req.Birthday) != "" {
		birthday = strings.TrimSpace(req.Birthday)
	}
	req.Address = strings.TrimSpace(req.Address)
	req.DetectionMode = strings.TrimSpace(req.DetectionMode)
	req.Diagnosis = strings.TrimSpace(req.Diagnosis)
	req.CancerDiameter = strings.TrimSpace(req.CancerDiameter)
	req.SmokingStatus = strings.TrimSpace(req.SmokingStatus)
	patientStatus := 1
	if req.PatientStatus != nil {
		patientStatus = *req.PatientStatus
	}
	diagnosis, cancerDiameter, conditionErr := normalizePatientConditionFields(patientStatus, req.Diagnosis, req.CancerDiameter)
	if conditionErr != nil {
		return 0, "", conditionErr
	}
	req.Diagnosis = diagnosis
	req.CancerDiameter = cancerDiameter

	var existingID int
	var patientCode, existingPhone string
	err := db.QueryRow("SELECT id, patient_code, COALESCE(phone, '') FROM detect_patient WHERE "+patientDocumentWhereClause("?")+" AND is_active = 1 LIMIT 1", documentNo, documentNo).
		Scan(&existingID, &patientCode, &existingPhone)
	if err == nil {
		if strings.TrimSpace(existingPhone) != "" {
			return 0, "", fmt.Errorf("该身份证件号已存在患者信息")
		}
		openID, openIDErr := getWechatOpenID(req.JsCode)
		if openIDErr != nil {
			log.Printf("Get invited patient openid error: %v", openIDErr)
		}
		subscribeTemplateID := ""
		if req.ReportSubscribeAccepted {
			subscribeTemplateID = reportSubscribeTemplateID
		}
		_, err = db.Exec(`UPDATE detect_patient SET phone = ?, name = COALESCE(NULLIF(name, ''), ?),
			id_document_type = ?, id_document_no = ?, id_card = ?,
			gender = ?, birthday = ?, address = COALESCE(NULLIF(address, ''), ?),
			diagnosis = ?, cancer_diameter = ?, smoking_status = ?, detection_mode = ?,
			sales_person = ?, wechat_openid = COALESCE(NULLIF(wechat_openid, ''), ?),
			report_subscribe_enabled = ?, report_subscribe_template_id = ?, patient_status = ?, updated_at = NOW()
			WHERE id = ?`,
			phone, req.Name, documentType, documentNo, legacyIDCardForDocument(documentType, documentNo), gender, birthday, req.Address,
			req.Diagnosis, req.CancerDiameter, req.SmokingStatus, req.DetectionMode, salesCode,
			openID, boolToInt(req.ReportSubscribeAccepted), subscribeTemplateID, patientStatus, existingID)
		if err != nil {
			return 0, "", err
		}
		return existingID, patientCode, nil
	}
	if err != sql.ErrNoRows {
		return 0, "", err
	}

	patientCode = generatePatientCode(db)
	openID, openIDErr := getWechatOpenID(req.JsCode)
	if openIDErr != nil {
		log.Printf("Get invited patient openid error: %v", openIDErr)
	}
	subscribeTemplateID := ""
	if req.ReportSubscribeAccepted {
		subscribeTemplateID = reportSubscribeTemplateID
	}
	result, err := db.Exec(`INSERT INTO detect_patient
		(patient_code, name, gender, id_document_type, id_document_no, id_card, phone, birthday, address, diagnosis, cancer_diameter, smoking_status, detection_mode,
		 sales_person, patient_source, wechat_openid, report_subscribe_enabled, report_subscribe_template_id, is_active, completion_status, patient_status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'sales_invite', ?, ?, ?, 1, 1, ?, NOW(), NOW())`,
		patientCode, req.Name, gender, documentType, documentNo, legacyIDCardForDocument(documentType, documentNo), phone, birthday, req.Address, req.Diagnosis, req.CancerDiameter, req.SmokingStatus, req.DetectionMode, salesCode,
		openID, boolToInt(req.ReportSubscribeAccepted), subscribeTemplateID, patientStatus)
	if err != nil {
		return 0, "", err
	}
	id, err := result.LastInsertId()
	return int(id), patientCode, err
}

func verifyInviteSMSCode(db *sql.DB, phone string, code string) error {
	return verifyAndUseSMSCode(db, phone, "invite_register", code)
}

// HandleInviteRegister 通过销售邀请页免登录提交患者信息并校验短信验证码。
func HandleInviteRegister(c *app.RequestContext, db *sql.DB) {
	var req inviteRegisterPayload
	body, err := c.Body()
	if err != nil || json.Unmarshal(body, &req) != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "请求参数错误", Data: nil})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.IdCard = strings.TrimSpace(req.IdCard)
	req.IDDocumentType = normalizePatientDocumentType(req.IDDocumentType)
	req.IDDocumentNo = normalizePatientDocumentNo(req.IDDocumentNo)
	if req.IDDocumentNo == "" {
		req.IDDocumentNo = normalizePatientDocumentNo(req.IdCard)
	}
	req.Phone = strings.TrimSpace(req.Phone)
	req.SmsCode = strings.TrimSpace(req.SmsCode)
	req.DetectionMode = strings.TrimSpace(req.DetectionMode)
	if req.SalesID <= 0 || req.Name == "" || req.IDDocumentNo == "" || req.Phone == "" || req.SmsCode == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "姓名、身份证件号、手机号、验证码和邀请参数不能为空", Data: nil})
		return
	}
	if isResidentIDCard(req.IDDocumentType) && len(req.IDDocumentNo) != 18 {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "居民身份证号格式不正确", Data: nil})
		return
	}
	if strings.TrimSpace(req.Gender) == "" || strings.TrimSpace(req.Birthday) == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "性别和生日不能为空", Data: nil})
		return
	}
	patientStatus := 1
	if req.PatientStatus != nil {
		patientStatus = *req.PatientStatus
	}
	req.Diagnosis, req.CancerDiameter, err = normalizePatientConditionFields(patientStatus, req.Diagnosis, req.CancerDiameter)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: err.Error(), Data: nil})
		return
	}
	manager, err := getSalesManager(db, req.SalesID)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "客户经理不存在或已停用", Data: nil})
		return
	}
	salesCode, _ := manager["employee_id"].(string)
	salesCode = strings.TrimSpace(salesCode)
	if salesCode == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "客户经理未配置工号，无法绑定", Data: nil})
		return
	}

	if err := verifyInviteSMSCode(db, req.Phone, req.SmsCode); err != nil {
		c.JSON(consts.StatusUnauthorized, ApiResponse{Code: 401, Success: false, Message: err.Error(), Data: nil})
		return
	}

	patientID, patientCode, err := createOrUpdateInvitedPatient(db, req.Phone, req, salesCode)
	if err != nil {
		log.Printf("Invite register patient error: %v", err)
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: err.Error(), Data: nil})
		return
	}

	createMiniappPatientSession(c, db, req.Phone, patientID, req.Name, patientCode)
}

// HandleUniGetSampleExpress 使用小程序患者登录态读取自己的样本运单。
func HandleUniGetSampleExpress(c *app.RequestContext, db *sql.DB) {
	phone, _ := c.Get("miniapp_phone")
	phoneStr, _ := phone.(string)
	sampleID, _ := strconv.Atoi(c.Param("id"))
	if sampleID <= 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "样本ID无效", Data: nil})
		return
	}

	var ownerCount int
	err := db.QueryRow(`SELECT COUNT(*) FROM detect_sample s JOIN detect_patient p ON s.patient_id = p.id
		WHERE s.id = ? AND p.phone = ? AND p.is_active = 1`, sampleID, phoneStr).Scan(&ownerCount)
	if err != nil || ownerCount == 0 {
		c.JSON(consts.StatusNotFound, ApiResponse{Code: 404, Success: false, Message: "样本不存在或无权访问", Data: nil})
		return
	}

	query := `SELECT id, sample_id, sample_code, express_company, tracking_number,
		sender_name, sender_phone, sender_address, receiver_name, receiver_phone,
		receiver_address, send_time, receive_time, status, notes, created_at, updated_at
		FROM detect_sample_express WHERE sample_id = ? ORDER BY created_at DESC LIMIT 1`
	var express Express
	err = db.QueryRow(query, sampleID).Scan(
		&express.ID, &express.SampleID, &express.SampleCode, &express.ExpressCompany,
		&express.TrackingNumber, &express.SenderName, &express.SenderPhone, &express.SenderAddress,
		&express.ReceiverName, &express.ReceiverPhone, &express.ReceiverAddress,
		&express.SendTime, &express.ReceiveTime, &express.Status, &express.Notes,
		&express.CreatedAt, &express.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "暂无快递运单", Data: nil})
		return
	}
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "查询失败", Data: nil})
		return
	}
	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "获取成功", Data: scanExpress(&express)})
}
