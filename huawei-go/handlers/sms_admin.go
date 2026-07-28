package handlers

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/baidubce/bce-sdk-go/services/sms"
	"github.com/baidubce/bce-sdk-go/services/sms/api"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/common/utils"
)

func newBaiduSMSClient() (*sms.Client, error) {
	cfg := getBaiduSMSConfig()
	if cfg.Endpoint == "" || cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("百度短信 Access Key、Secret Key 或服务域名未配置")
	}
	return sms.NewClient(cfg.AccessKey, cfg.SecretKey, cfg.Endpoint)
}

func smsAdminError(c *app.RequestContext, status int, message string, err error) {
	data := utils.H{}
	if err != nil {
		data["error"] = err.Error()
	}
	c.JSON(status, ApiResponse{Code: status, Success: false, Message: message, Data: data})
}

func HandleGetSMSPackages(c *app.RequestContext, db *sql.DB) {
	ensureSystemSettingDefaults(db)
	userID := strings.TrimSpace(getRuntimeSetting("SMS_BAIDU_USER_ID", "SMS_BAIDU_USER_ID", ""))
	if userID == "" {
		smsAdminError(c, consts.StatusBadRequest, "请先配置百度智能云用户 ID", nil)
		return
	}
	client, err := newBaiduSMSClient()
	if err != nil {
		smsAdminError(c, consts.StatusBadRequest, "百度短信配置不完整", err)
		return
	}
	page := c.DefaultQuery("page", "1")
	pageSize := c.DefaultQuery("page_size", "10")
	result, err := client.GetPrepaidPackages(&api.GetPrepaidPackageArgs{
		UserID: userID, CountryType: strings.TrimSpace(c.Query("country_type")),
		PackageStatus: strings.TrimSpace(c.Query("status")), PageNo: page, PageSize: pageSize,
	})
	if err != nil {
		smsAdminError(c, consts.StatusBadGateway, "查询百度短信量包失败", err)
		return
	}
	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "查询短信量包成功", Data: utils.H{
		"list": result.PrepaidPackages, "total": result.TotalCount, "page": page, "page_size": pageSize,
	}})
}

func candidateSMSTemplateIDs() []string {
	cfg := getBaiduSMSConfig()
	raw := getRuntimeSetting("SMS_BAIDU_TEMPLATE_IDS", "SMS_BAIDU_TEMPLATE_IDS", "")
	raw += "," + cfg.LoginTemplateID + "," + cfg.ReportTemplateID
	seen := map[string]bool{}
	ids := make([]string, 0)
	for _, value := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == '\n' || r == ';' }) {
		id := strings.TrimSpace(value)
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

func HandleGetSMSTemplates(c *app.RequestContext, db *sql.DB) {
	ensureSystemSettingDefaults(db)
	client, err := newBaiduSMSClient()
	if err != nil {
		smsAdminError(c, consts.StatusBadRequest, "百度短信配置不完整", err)
		return
	}
	items := make([]utils.H, 0)
	for _, id := range candidateSMSTemplateIDs() {
		result, queryErr := client.GetTemplate(&api.GetTemplateArgs{TemplateId: id})
		if queryErr != nil {
			items = append(items, utils.H{"templateId": id, "status": "QUERY_FAILED", "error": queryErr.Error()})
			continue
		}
		items = append(items, utils.H{
			"templateId": result.TemplateId, "name": result.Name, "content": result.Content,
			"countryType": result.CountryType, "smsType": result.SmsType, "status": result.Status,
			"description": result.Description, "review": result.Review,
		})
	}
	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "查询短信模板成功", Data: utils.H{"list": items}})
}

func maskSMSMobile(mobile string) string {
	chars := []rune(strings.TrimSpace(mobile))
	if len(chars) < 7 {
		return mobile
	}
	return string(chars[:3]) + "****" + string(chars[len(chars)-4:])
}

func HandleGetSMSLogs(c *app.RequestContext, db *sql.DB) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if page < 1 { page = 1 }
	if pageSize < 1 || pageSize > 100 { pageSize = 10 }
	where := []string{"1 = 1"}
	args := make([]interface{}, 0)
	if purpose := strings.TrimSpace(c.Query("purpose")); purpose != "" {
		where = append(where, "purpose = ?"); args = append(args, purpose)
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		where = append(where, "status = ?"); args = append(args, status)
	}
	if mobile := strings.TrimSpace(c.Query("mobile")); mobile != "" {
		where = append(where, "mobile LIKE ?"); args = append(args, "%"+mobile+"%")
	}
	if start := strings.TrimSpace(c.Query("start_date")); start != "" {
		where = append(where, "created_at >= ?"); args = append(args, start+" 00:00:00")
	}
	if end := strings.TrimSpace(c.Query("end_date")); end != "" {
		where = append(where, "created_at < DATE_ADD(?, INTERVAL 1 DAY)"); args = append(args, end)
	}
	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := db.QueryRow("SELECT COUNT(*) FROM base_sms_send_log WHERE "+whereSQL, args...).Scan(&total); err != nil {
		smsAdminError(c, consts.StatusInternalServerError, "查询短信日志失败", err); return
	}
	queryArgs := append(append([]interface{}{}, args...), pageSize, (page-1)*pageSize)
	rows, err := db.Query(`SELECT id, purpose, mobile, template_id, status, provider_code, provider_message,
		DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:%s') FROM base_sms_send_log WHERE `+whereSQL+` ORDER BY id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil { smsAdminError(c, consts.StatusInternalServerError, "查询短信日志失败", err); return }
	defer rows.Close()
	items := make([]utils.H, 0)
	for rows.Next() {
		var id int64
		var purpose, mobile, templateID, status, code, message, createdAt string
		if err := rows.Scan(&id, &purpose, &mobile, &templateID, &status, &code, &message, &createdAt); err != nil { continue }
		items = append(items, utils.H{"id": id, "purpose": purpose, "purpose_name": smsPurposeDisplayName(purpose),
			"mobile": maskSMSMobile(mobile), "template_id": templateID, "status": status,
			"provider_code": code, "provider_message": message, "created_at": createdAt})
	}
	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "查询短信日志成功", Data: utils.H{
		"list": items, "total": total, "page": page, "page_size": pageSize,
	}})
}
