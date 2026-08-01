package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

const (
	expressDirectionInbound  = "inbound"
	expressDirectionOutbound = "outbound"
)

type expressProviderConfig struct {
	Enabled   bool
	URL       string
	AuthMode  string
	AppKey    string
	AppSecret string
}

type expressProviderEvent struct {
	Time   string `json:"time"`
	Status string `json:"status"`
}

type expressProviderResponse struct {
	Status int    `json:"status"`
	Msg    string `json:"msg"`
	Result struct {
		Number         string                 `json:"number"`
		Type           string                 `json:"type"`
		TypeName       string                 `json:"typename"`
		Logo           string                 `json:"logo"`
		List           []expressProviderEvent `json:"list"`
		DeliveryStatus int                    `json:"deliverystatus"`
		IsSign         int                    `json:"issign"`
	} `json:"result"`
}

type expressQueryResult struct {
	Status          string
	ProviderStatus  int
	ProviderMessage string
	CompanyType     string
	CompanyName     string
	Logo            string
	Route           []expressProviderEvent
	LatestTime      *time.Time
	LatestStatus    string
	DeliveredAt     *time.Time
}

var expressWorkerOnce sync.Once

func normalizeExpressDirection(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), expressDirectionOutbound) {
		return expressDirectionOutbound
	}
	return expressDirectionInbound
}

func normalizeExpressType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "auto"
	}
	return value
}

func expressConfig() expressProviderConfig {
	enabledValue := strings.ToLower(getRuntimeSetting("EXPRESS_QUERY_ENABLED", "EXPRESS_QUERY_ENABLED", "1"))
	return expressProviderConfig{
		Enabled: !map[string]bool{"0": true, "false": true, "off": true, "disabled": true}[enabledValue],
		URL: strings.TrimSpace(getRuntimeSetting(
			"EXPRESS_API_URL",
			"EXPRESS_API_URL",
			"https://jisuexpress.api.bdymkt.com/express/query",
		)),
		AuthMode:  strings.ToLower(strings.TrimSpace(getRuntimeSetting("EXPRESS_AUTH_MODE", "EXPRESS_AUTH_MODE", "appcode"))),
		AppKey:    strings.TrimSpace(getRuntimeSetting("EXPRESS_APP_KEY", "EXPRESS_APP_KEY", "")),
		AppSecret: strings.TrimSpace(getRuntimeSetting("EXPRESS_APP_SECRET", "EXPRESS_APP_SECRET", "")),
	}
}

func isExpressV1Auth(cfg expressProviderConfig) bool {
	return cfg.AuthMode == "app" || cfg.AuthMode == "app_v1" || cfg.AuthMode == "v1"
}

func queryExpressProvider(client *http.Client, cfg expressProviderConfig, expressType, number, mobile string) (expressQueryResult, error) {
	if !cfg.Enabled {
		return expressQueryResult{}, errors.New("快递查询功能未启用")
	}
	if cfg.URL == "" || cfg.AppKey == "" || (isExpressV1Auth(cfg) && cfg.AppSecret == "") {
		return expressQueryResult{}, errors.New("快递查询 API 配置未完成")
	}
	endpoint, err := url.Parse(cfg.URL)
	if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
		return expressQueryResult{}, errors.New("快递查询 API 地址无效")
	}
	query := endpoint.Query()
	// 百度极速快递接口使用 auto 自动识别承运公司。
	query.Set("type", "auto")
	query.Set("number", strings.TrimSpace(number))
	if strings.TrimSpace(mobile) != "" {
		query.Set("mobile", strings.TrimSpace(mobile))
	}
	endpoint.RawQuery = query.Encode()

	request, err := http.NewRequest(http.MethodPost, endpoint.String(), nil)
	if err != nil {
		return expressQueryResult{}, err
	}
	request.Header.Set("Content-Type", "application/json;charset=UTF-8")
	if isExpressV1Auth(cfg) {
		if err := signExpressV1Request(request, cfg.AppKey, cfg.AppSecret, time.Now().UTC()); err != nil {
			return expressQueryResult{}, err
		}
	} else {
		request.Header.Set("X-Bce-Signature", "AppCode/"+cfg.AppKey)
	}

	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	response, err := client.Do(request)
	if err != nil {
		return expressQueryResult{}, fmt.Errorf("快递查询请求失败: %w", err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message := truncateExpressText(string(body), 500)
		if message == "" {
			return expressQueryResult{}, fmt.Errorf("快递查询接口返回 HTTP %d", response.StatusCode)
		}
		return expressQueryResult{}, fmt.Errorf("快递查询接口返回 HTTP %d: %s", response.StatusCode, message)
	}

	var providerResponse expressProviderResponse
	if err := json.Unmarshal(body, &providerResponse); err != nil {
		return expressQueryResult{}, fmt.Errorf("快递查询结果解析失败: %w", err)
	}
	if providerResponse.Status != 0 {
		return expressQueryResult{}, fmt.Errorf("快递查询失败: %s", firstNonEmptyString(providerResponse.Msg, "未知错误"))
	}

	result := expressQueryResult{
		Status:          "in_transit",
		ProviderStatus:  providerResponse.Status,
		ProviderMessage: providerResponse.Msg,
		CompanyType:     providerResponse.Result.Type,
		CompanyName:     providerResponse.Result.TypeName,
		Logo:            providerResponse.Result.Logo,
		Route:           providerResponse.Result.List,
	}
	if len(providerResponse.Result.List) == 0 {
		result.Status = "pending"
	}
	if providerResponse.Result.DeliveryStatus == 0 {
		result.Status = "picked_up"
	}
	if providerResponse.Result.DeliveryStatus == 4 {
		result.Status = "exception"
	}
	if providerResponse.Result.IsSign == 1 || providerResponse.Result.DeliveryStatus == 3 {
		result.Status = "delivered"
	}

	for _, event := range providerResponse.Result.List {
		if parsed, parseErr := parseExpressEventTime(event.Time); parseErr == nil {
			if result.LatestTime == nil || parsed.After(*result.LatestTime) {
				eventTime := parsed
				result.LatestTime = &eventTime
				result.LatestStatus = event.Status
			}
		}
	}
	if result.Status == "delivered" {
		delivered := result.LatestTime
		for _, event := range providerResponse.Result.List {
			if !strings.Contains(event.Status, "签收") {
				continue
			}
			if parsed, parseErr := parseExpressEventTime(event.Time); parseErr == nil {
				deliveredTime := parsed
				delivered = &deliveredTime
				break
			}
		}
		if delivered == nil {
			now := time.Now()
			delivered = &now
		}
		result.DeliveredAt = delivered
		// 签收后不再保存或返回中间路径。
		result.Route = nil
	}
	return result, nil
}

func signExpressV1Request(request *http.Request, appKey, appSecret string, now time.Time) error {
	if request == nil || strings.TrimSpace(appKey) == "" || strings.TrimSpace(appSecret) == "" {
		return errors.New("快递查询 V1 签名配置未完成")
	}
	if request.URL == nil {
		return errors.New("快递查询请求地址无效")
	}
	host := request.Host
	if host == "" {
		host = request.URL.Host
	}
	if host == "" {
		return errors.New("快递查询请求 Host 无效")
	}
	contentType := strings.TrimSpace(request.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/json;charset=UTF-8"
		request.Header.Set("Content-Type", contentType)
	}
	date := now.UTC().Format("2006-01-02T15:04:05Z")
	authPrefix := fmt.Sprintf("bce-auth-v1/%s/%s/1800", appKey, date)
	signedHeaders := "content-type;host"
	canonicalRequest := strings.Join([]string{
		request.Method,
		bceEncodePath(request.URL.Path),
		bceCanonicalQuery(request.URL.Query()),
		"content-type:" + bceURIEncode(contentType),
		"host:" + bceURIEncode(host),
	}, "\n")
	signingKey := bceHMACSHA256([]byte(appSecret), authPrefix)
	signature := fmt.Sprintf("%x", bceHMACSHA256(signingKey, canonicalRequest))
	request.Header.Set("X-Bce-Signature", authPrefix+"/"+signedHeaders+"/"+signature)
	return nil
}

func bceHMACSHA256(key []byte, value string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}

func bceEncodePath(path string) string {
	if path == "" {
		path = "/"
	}
	return strings.ReplaceAll(bceURIEncode(path), "%2F", "/")
}

func bceCanonicalQuery(values url.Values) string {
	parts := make([]string, 0)
	for key, items := range values {
		if strings.EqualFold(key, "authorization") {
			continue
		}
		if len(items) == 0 {
			parts = append(parts, bceURIEncode(key)+"=")
			continue
		}
		for _, value := range items {
			parts = append(parts, bceURIEncode(key)+"="+bceURIEncode(value))
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, "&")
}

func bceURIEncode(value string) string {
	var builder strings.Builder
	for _, ch := range []byte(value) {
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' || ch == '.' || ch == '~' {
			builder.WriteByte(ch)
			continue
		}
		fmt.Fprintf(&builder, "%%%02X", ch)
	}
	return builder.String()
}

func parseExpressEventTime(value string) (time.Time, error) {
	location, _ := time.LoadLocation("Asia/Shanghai")
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339} {
		if parsed, err := time.ParseInLocation(layout, strings.TrimSpace(value), location); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid express event time: %s", value)
}

func refreshExpressByID(db *sql.DB, expressID int) (utils.H, error) {
	var sampleID int
	var direction, expressType, trackingNumber string
	var queryMobile, senderPhone, receiverPhone, company, currentStatus sql.NullString
	var lastQueryAt sql.NullTime
	err := db.QueryRow(`SELECT sample_id, direction, express_type, tracking_number,
		query_mobile, sender_phone, receiver_phone, express_company, status, last_query_at
		FROM detect_sample_express WHERE id = ?`, expressID).Scan(
		&sampleID, &direction, &expressType, &trackingNumber, &queryMobile, &senderPhone,
		&receiverPhone, &company, &currentStatus, &lastQueryAt)
	if err != nil {
		return nil, err
	}
	if currentStatus.String == "delivered" {
		return getExpressByID(db, expressID)
	}
	// 患者端、员工端和管理端共用同一份一小时缓存，避免重复计费查询。
	if lastQueryAt.Valid && time.Since(lastQueryAt.Time) < time.Hour {
		return getExpressByID(db, expressID)
	}
	mobile := strings.TrimSpace(queryMobile.String)
	if mobile == "" && (strings.Contains(company.String, "顺丰") || strings.EqualFold(expressType, "sfexpress")) {
		mobile = firstNonEmptyString(strings.TrimSpace(senderPhone.String), strings.TrimSpace(receiverPhone.String))
	}

	result, queryErr := queryExpressProvider(nil, expressConfig(), "auto", trackingNumber, mobile)
	if queryErr != nil {
		_, _ = db.Exec(`UPDATE detect_sample_express
			SET last_query_at = NOW(), last_query_error = ?, updated_at = NOW() WHERE id = ?`,
			truncateExpressText(queryErr.Error(), 500), expressID)
		return getExpressByID(db, expressID)
	}

	var routeJSON interface{}
	if result.Status != "delivered" && len(result.Route) > 0 {
		encoded, _ := json.Marshal(result.Route)
		routeJSON = string(encoded)
	}
	var latestTime, deliveredAt interface{}
	if result.LatestTime != nil {
		latestTime = *result.LatestTime
	}
	if result.DeliveredAt != nil {
		deliveredAt = *result.DeliveredAt
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`UPDATE detect_sample_express
		SET express_type = COALESCE(NULLIF(?, ''), express_type),
			express_company = COALESCE(NULLIF(?, ''), express_company),
			status = ?, provider_status = ?, provider_message = ?,
			route_json = ?, latest_event_time = ?, latest_event_status = ?,
			delivered_at = ?, receive_time = CASE WHEN ? = 'delivered' THEN ? ELSE receive_time END,
			last_query_at = NOW(), last_query_error = '', updated_at = NOW()
		WHERE id = ?`,
		result.CompanyType, result.CompanyName, result.Status, result.ProviderStatus,
		truncateExpressText(result.ProviderMessage, 255), routeJSON, latestTime,
		truncateExpressText(result.LatestStatus, 500), deliveredAt,
		result.Status, deliveredAt, expressID); err != nil {
		return nil, err
	}
	if result.Status == "delivered" && result.DeliveredAt != nil {
		column := "inbound_express_signed_at"
		if normalizeExpressDirection(direction) == expressDirectionOutbound {
			column = "outbound_express_signed_at"
		}
		if _, err = tx.Exec("UPDATE detect_sample SET "+column+" = ?, sample_updated_at = NOW() WHERE id = ?",
			*result.DeliveredAt, sampleID); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return getExpressByID(db, expressID)
}

func truncateExpressText(value string, limit int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return value
}

func HandleRefreshExpress(c *app.RequestContext, db *sql.DB) {
	expressID, err := strconv.Atoi(c.Param("id"))
	if err != nil || expressID <= 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "运单ID无效", Data: nil})
		return
	}
	data, err := refreshExpressByID(db, expressID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(consts.StatusNotFound, ApiResponse{Code: 404, Success: false, Message: "运单不存在", Data: nil})
			return
		}
		c.JSON(consts.StatusBadGateway, ApiResponse{Code: 502, Success: false, Message: err.Error(), Data: nil})
		return
	}
	message := "物流状态已更新"
	if lastError, _ := data["last_query_error"].(string); lastError != "" {
		message = lastError
	}
	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: message, Data: data})
}

func refreshOpenExpressShipments(db *sql.DB) {
	if db == nil || !expressConfig().Enabled {
		return
	}
	rows, err := db.Query(`SELECT id FROM detect_sample_express
		WHERE status <> 'delivered'
		  AND tracking_number <> ''
		  AND (last_query_at IS NULL OR last_query_at < DATE_SUB(NOW(), INTERVAL 60 MINUTE))
		ORDER BY COALESCE(last_query_at, created_at) ASC LIMIT 100`)
	if err != nil {
		log.Printf("Query open express shipments failed: %v", err)
		return
	}
	var ids []int
	for rows.Next() {
		var id int
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	_ = rows.Close()
	for _, id := range ids {
		if _, err := refreshExpressByID(db, id); err != nil {
			log.Printf("Refresh express %d failed: %v", id, err)
		}
		time.Sleep(150 * time.Millisecond)
	}
}

func StartExpressTrackingWorker(db *sql.DB) {
	expressWorkerOnce.Do(func() {
		go func() {
			time.Sleep(time.Minute)
			for {
				refreshOpenExpressShipments(db)
				minutes, _ := strconv.Atoi(getRuntimeSetting(
					"EXPRESS_POLL_INTERVAL_MINUTES",
					"EXPRESS_POLL_INTERVAL_MINUTES",
					"60",
				))
				if minutes < 60 {
					minutes = 60
				}
				time.Sleep(time.Duration(minutes) * time.Minute)
			}
		}()
	})
}
