package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/baidubce/bce-sdk-go/bce"
	bcehttp "github.com/baidubce/bce-sdk-go/http"
	"github.com/baidubce/bce-sdk-go/services/sms"
	"github.com/baidubce/bce-sdk-go/services/sms/api"
)

type baiduSMSConfig struct {
	Endpoint         string
	AccessKey        string
	SecretKey        string
	SignatureID      string
	LoginTemplateID  string
	ReportTemplateID string
	CertificateID    string
}

var smsPurposeSettings = map[string]string{
	"admin_login":        "SMS_ADMIN_LOGIN_ENABLED",
	"miniapp_login":      "SMS_MINIAPP_LOGIN_ENABLED",
	"admin_bind_phone":   "SMS_ADMIN_BIND_PHONE_ENABLED",
	"miniapp_bind_phone": "SMS_MINIAPP_BIND_PHONE_ENABLED",
	"invite_register":    "SMS_INVITE_REGISTER_ENABLED",
	"report_ready":       "SMS_REPORT_READY_ENABLED",
}

func smsPurposeDisplayName(purpose string) string {
	names := map[string]string{
		"admin_login": "管理后台登录验证码短信", "miniapp_login": "小程序登录验证码短信",
		"admin_bind_phone": "管理后台绑定手机短信", "miniapp_bind_phone": "小程序绑定手机短信",
		"invite_register": "邀请注册验证码短信", "report_ready": "报告出具通知短信",
	}
	if name := names[purpose]; name != "" {
		return name
	}
	return "短信发送功能"
}

func isSMSPurposeEnabled(purpose string) bool {
	key := smsPurposeSettings[purpose]
	if key == "" {
		return true
	}
	value := strings.ToLower(strings.TrimSpace(getRuntimeSetting(key, key, "1")))
	switch value {
	case "0", "false", "off", "no", "disabled":
		return false
	default:
		return true
	}
}

func writeSMSSendLog(db *sql.DB, purpose, mobile, templateID, status, providerCode, providerMessage string) int64 {
	if db == nil {
		return 0
	}
	result, err := db.Exec(`INSERT INTO base_sms_send_log
		(purpose, mobile, template_id, status, provider_code, provider_message, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, NOW(), NOW())`, purpose, mobile, templateID, status, providerCode, providerMessage)
	if err != nil {
		log.Printf("Write SMS send log failed: %v", err)
		return 0
	}
	id, _ := result.LastInsertId()
	return id
}

func finishSMSSendLog(db *sql.DB, id int64, status, providerCode, providerMessage string) {
	if db == nil || id <= 0 {
		return
	}
	if _, err := db.Exec(`UPDATE base_sms_send_log SET status = ?, provider_code = ?, provider_message = ?, updated_at = NOW() WHERE id = ?`,
		status, providerCode, providerMessage, id); err != nil {
		log.Printf("Update SMS send log %d failed: %v", id, err)
	}
}

func getBaiduSMSConfig() baiduSMSConfig {
	if db := GetDB(); db != nil {
		ensureSystemSettingDefaults(db)
	}

	endpoint := getRuntimeSetting("SMS_BAIDU_ENDPOINT", "SMS_BAIDU_ENDPOINT", "http://sms.bj.baidubce.com")
	if strings.Contains(endpoint, "smsv3.bj.baidubce.com") {
		endpoint = "http://sms.bj.baidubce.com"
	}
	if endpoint != "" && !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "https://" + endpoint
	}

	return baiduSMSConfig{
		Endpoint:         strings.TrimRight(endpoint, "/"),
		AccessKey:        getRuntimeSetting("SMS_BAIDU_ACCESS_KEY", "SMS_BAIDU_ACCESS_KEY", ""),
		SecretKey:        strings.TrimSpace(getRuntimeSetting("SMS_BAIDU_SECRET_KEY", "SMS_BAIDU_SECRET_KEY", "")),
		SignatureID:      getRuntimeSetting("SMS_BAIDU_SIGNATURE_ID", "SMS_BAIDU_SIGNATURE_ID", "sms-sign-ShdJQl83240"),
		LoginTemplateID:  getRuntimeSetting("SMS_BAIDU_LOGIN_TEMPLATE_ID", "SMS_BAIDU_LOGIN_TEMPLATE_ID", "sms-tmpl-BhdTpq84685"),
		ReportTemplateID: getRuntimeSetting("SMS_BAIDU_REPORT_TEMPLATE_ID", "SMS_BAIDU_REPORT_TEMPLATE_ID", "sms-tmpl-iVHgkc58950"),
		CertificateID:    getRuntimeSetting("SMS_BAIDU_CERTIFICATE_ID", "SMS_BAIDU_CERTIFICATE_ID", "sms-cert-RtzbqZ63190"),
	}
}

func sendBaiduSMS(db *sql.DB, mobile, templateID, purpose string, contentVar map[string]interface{}) error {
	cfg := getBaiduSMSConfig()
	if !isSMSPurposeEnabled(purpose) {
		writeSMSSendLog(db, purpose, mobile, templateID, "skipped", "DISABLED", "该类短信已关闭")
		return fmt.Errorf("%s已关闭", smsPurposeDisplayName(purpose))
	}
	logID := writeSMSSendLog(db, purpose, mobile, templateID, "sending", "", "")
	fail := func(code string, err error) error {
		finishSMSSendLog(db, logID, "failed", code, err.Error())
		return err
	}
	if cfg.Endpoint == "" || cfg.AccessKey == "" || cfg.SecretKey == "" || cfg.SignatureID == "" || templateID == "" {
		return fail("CONFIG_INCOMPLETE", fmt.Errorf("百度短信配置未完成"))
	}

	client, err := sms.NewClient(cfg.AccessKey, cfg.SecretKey, cfg.Endpoint)
	if err != nil {
		return fail("CLIENT_ERROR", err)
	}

	args := &api.SendSmsArgs{
		Mobile:      mobile,
		Template:    templateID,
		SignatureId: cfg.SignatureID,
		ContentVar:  contentVar,
	}
	bodyBytes, err := json.Marshal(args)
	if err != nil {
		return fail("ENCODE_ERROR", err)
	}
	body, err := bce.NewBodyFromBytes(bodyBytes)
	if err != nil {
		return fail("BODY_ERROR", err)
	}

	req := &bce.BceRequest{}
	req.SetUri("/api/v3/sendSms")
	req.SetMethod(bcehttp.POST)
	req.SetHeader(bcehttp.CONTENT_TYPE, bce.DEFAULT_CONTENT_TYPE)
	req.SetBody(body)

	resp := &bce.BceResponse{}
	if err := client.SendRequest(req, resp); err != nil {
		return fail("REQUEST_ERROR", err)
	}
	if resp.IsFail() {
		return fail("SERVICE_ERROR", resp.ServiceError())
	}

	result := &api.SendSmsResult{}
	if err := resp.ParseJsonBody(result); err != nil {
		return fail("PARSE_ERROR", err)
	}
	if result.Code != "" && result.Code != "1000" {
		return fail(result.Code, fmt.Errorf("百度短信发送失败: %s %s", result.Code, result.Message))
	}
	providerCode, providerMessage := result.Code, result.Message
	if len(result.Data) > 0 {
		item := result.Data[0]
		if item.Code != "" {
			providerCode = item.Code
		}
		if item.Message != "" {
			providerMessage = item.Message
		}
		if item.Code != "" && item.Code != "1000" {
			return fail(item.Code, fmt.Errorf("百度短信发送失败: %s %s", item.Code, item.Message))
		}
	}

	finishSMSSendLog(db, logID, "success", providerCode, providerMessage)
	log.Printf("Baidu SMS sent to %s with template %s", mobile, templateID)
	return nil
}

func sendReportReadySMS(db *sql.DB, reportID int) {
	if db == nil || reportID <= 0 {
		return
	}

	var name string
	var phone string
	err := db.QueryRow(`SELECT COALESCE(p.name, ''), COALESCE(p.phone, '')
		FROM detect_report r
		LEFT JOIN detect_sample s ON r.sample_id = s.id
		LEFT JOIN detect_patient p ON COALESCE(s.patient_id, r.patient_id) = p.id
		WHERE r.id = ?`, reportID).Scan(&name, &phone)
	if err != nil {
		log.Printf("Query report SMS recipient failed for report %d: %v", reportID, err)
		return
	}
	phone = strings.TrimSpace(phone)
	if phone == "" {
		log.Printf("Skip report SMS for report %d: patient phone is empty", reportID)
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "用户"
	}

	if err := sendBaiduSMS(db, phone, getBaiduSMSConfig().ReportTemplateID, "report_ready", map[string]interface{}{"Name": name}); err != nil {
		log.Printf("Failed to send report ready SMS for report %d to %s: %v", reportID, phone, err)
	}
}
