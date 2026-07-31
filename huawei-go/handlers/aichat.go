package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

var (
	aiAPIKey            string
	aiAPIURL            string
	aiModel             string
	aiPrompt            string
	aiReportVisionModel string
	aiReportTextModel   string
	aiReportPrompt      string
	httpClient          *http.Client
)

const defaultReportAnalysisPrompt = `你是华微智检医疗科技的医疗报告信息录入助手。请尽最大可能准确读取患者上传报告中清晰可见的文字，不得省略可辨认的重要信息，也不得猜测模糊内容。
请先识别报告属于“检查/检验报告”“病理报告”或“无法确定”，然后提取医院、科室、患者基本信息、报告编号、检查/采样/送检/报告日期、检查项目、标本、主要所见、全部关键数值及单位、参考范围、异常标记、影像描述、结论或诊断原文。能读取的尽量完整读取；无法辨认的字段写“未识别”。
仅做客观信息整理，不提供诊断判断、治疗建议、用药建议或风险推断。
必须严格按以下格式输出，字段名单独占一行：
报告类型：检查/检验报告、病理报告或无法确定
医院：医院全称或未识别
检查时间：优先检查日期，其次采样/送检/报告日期；保留原报告时间格式或写未识别
检查项目：检查项目全称或未识别
内容摘要：
- 按原报告结构逐条列出其他所有可辨认内容，包括科室、患者信息、标本、所见、数值、单位、参考范围、异常标记、结论/诊断原文等。
不要输出“温馨提示”、免责声明或就医建议。`

// AIChatRequest AI 聊天请求结构
type AIChatRequest struct {
	Message  string        `json:"message"`
	Messages []ChatMessage `json:"messages,omitempty"`
	Stream   bool          `json:"stream,omitempty"`
}

// ChatMessage 聊天消息结构
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AIChatResponse AI 聊天响应结构
type AIChatResponse struct {
	Content string `json:"content"`
}

// OpenAIChatMessage OpenAI 聊天消息
type OpenAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIChatRequest OpenAI 聊天请求
type OpenAIChatRequest struct {
	Model    string              `json:"model"`
	Messages []OpenAIChatMessage `json:"messages"`
	Stream   bool                `json:"stream,omitempty"`
}

// OpenAIChatResponse OpenAI 聊天响应
type OpenAIChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// OpenAIStreamChunk OpenAI 流式响应块
type OpenAIStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

// InitAIChat 初始化 AI 聊天客户端
func InitAIChat() {
	aiAPIKey = os.Getenv("AI_API_KEY")
	aiAPIURL = os.Getenv("AI_API_URL")
	aiModel = os.Getenv("AI_MODEL")
	aiPrompt = os.Getenv("AI_PROMPT")
	aiReportVisionModel = firstNonEmptyString(os.Getenv("AI_REPORT_VISION_MODEL"), "ernie-4.5-turbo-vl-32k")
	aiReportTextModel = firstNonEmptyString(os.Getenv("AI_REPORT_TEXT_MODEL"), "ernie-lite-pro-128k")
	aiReportPrompt = firstNonEmptyString(os.Getenv("AI_REPORT_PROMPT"), defaultReportAnalysisPrompt)

	if aiAPIKey == "" {
		log.Println("Warning: AI_API_KEY not found in environment variables")
	}
	if aiAPIURL == "" {
		aiAPIURL = "https://qianfan.baidubce.com/v2"
		log.Println("Warning: AI_API_URL not found, using default:", aiAPIURL)
	}
	if aiModel == "" {
		aiModel = "ernie-lite-pro-128k"
		log.Println("Warning: AI_MODEL not found, using default:", aiModel)
	}

	// 初始化 HTTP 客户端
	httpClient = &http.Client{
		Timeout: 120 * time.Second,
	}

	log.Println("AI chat client initialized successfully")
}

// ReloadAISettings 从数据库加载 AI 设置。环境变量只作为数据库首次初始化的来源。
func ReloadAISettings(db *sql.DB) {
	if db == nil {
		return
	}
	ensureSystemSettingDefaults(db)
	rows, err := db.Query(`SELECT key_name, key_value, is_encrypted FROM setting_system
		WHERE key_name IN ('AI_API_KEY','AI_API_URL','AI_MODEL','AI_PROMPT',
			'AI_REPORT_VISION_MODEL','AI_REPORT_TEXT_MODEL','AI_REPORT_PROMPT')`)
	if err != nil {
		log.Printf("load AI settings from database: %v", err)
		return
	}
	defer rows.Close()
	values := map[string]string{}
	for rows.Next() {
		var key, value string
		var encrypted int
		if rows.Scan(&key, &value, &encrypted) != nil {
			continue
		}
		if encrypted == 1 {
			value = decryptConfigValue(value)
		}
		values[key] = value
	}
	aiAPIKey = values["AI_API_KEY"]
	aiAPIURL = firstNonEmptyString(values["AI_API_URL"], "https://qianfan.baidubce.com/v2")
	aiModel = firstNonEmptyString(values["AI_MODEL"], "ernie-lite-pro-128k")
	aiPrompt = values["AI_PROMPT"]
	aiReportVisionModel = firstNonEmptyString(values["AI_REPORT_VISION_MODEL"], "ernie-4.5-turbo-vl-32k")
	aiReportTextModel = firstNonEmptyString(values["AI_REPORT_TEXT_MODEL"], "ernie-lite-pro-128k")
	storedReportPrompt := strings.TrimSpace(values["AI_REPORT_PROMPT"])
	if storedReportPrompt == "" || (strings.Contains(storedReportPrompt, "结尾固定提示") && strings.Contains(storedReportPrompt, "温馨提示")) {
		storedReportPrompt = defaultReportAnalysisPrompt
		if _, updateErr := db.Exec(`UPDATE setting_system SET key_value = ?, updated_at = NOW() WHERE key_name = 'AI_REPORT_PROMPT'`, storedReportPrompt); updateErr != nil {
			log.Printf("update default report analysis prompt: %v", updateErr)
		}
	}
	aiReportPrompt = storedReportPrompt
}

// GetAIRequestUser 获取调用AI对话的用户身份和AI访问限制状态
func GetAIRequestUser(c *app.RequestContext, db *sql.DB) (userID int, patientID int, identityType string, aiAllowed bool, err error) {
	aiAllowed = true // 默认允许
	if db == nil {
		return 0, 0, "", true, nil
	}

	// 1. 尝试小程序 Token/Session 认证
	sessionID := c.Cookie("miniapp_session_id")
	if string(sessionID) == "" {
		sessionID = c.GetHeader("X-Miniapp-Session")
	}

	if string(sessionID) != "" {
		var phone string
		var uID, pID int
		query := "SELECT phone, identity_type, COALESCE(user_id, 0), COALESCE(patient_id, 0) FROM base_miniapp_sessions WHERE session_id = ? AND expiry > NOW()"
		err := db.QueryRow(query, sessionID).Scan(&phone, &identityType, &uID, &pID)
		if err == nil {
			userID = uID
			patientID = pID

			// 检查对应的表是否禁用了该用户
			if identityType == "employee" && userID > 0 {
				subjectCode, _ := getAIEmployeeSubjectCode(db, userID)
				aiAllowed = !IsAIBlacklisted(db, "employee", subjectCode)
			} else if patientID > 0 {
				subjectCode, _ := getAIPatientSubjectCode(db, patientID, phone)
				aiAllowed = !IsAIBlacklisted(db, "patient", subjectCode)
			} else if phone != "" {
				// 未绑定ID但有手机号，兜底查询患者表
				subjectCode, _ := getAIPatientSubjectCode(db, 0, phone)
				aiAllowed = !IsAIBlacklisted(db, "patient", subjectCode)
				identityType = "patient"
			}
			return userID, patientID, identityType, aiAllowed, nil
		}
	}

	// 2. 尝试后台管理系统 Cookie 认证
	webSessionID := c.Cookie("session_id")
	if string(webSessionID) != "" {
		var uID int
		var expiry time.Time
		query := "SELECT user_id, expiry FROM base_sessions WHERE session_id = ?"
		err := db.QueryRow(query, webSessionID).Scan(&uID, &expiry)
		if err == nil && time.Now().Before(expiry) {
			userID = uID
			identityType = "employee"

			subjectCode, _ := getAIEmployeeSubjectCode(db, userID)
			aiAllowed = !IsAIBlacklisted(db, "employee", subjectCode)
			return userID, 0, identityType, aiAllowed, nil
		}
	}

	return 0, 0, "", false, fmt.Errorf("unauthorized")
}

// HandleAIChat 处理 AI 聊天请求
func HandleAIChat(ctx context.Context, c *app.RequestContext) {
	if aiAPIKey == "" {
		ErrorResponse(c, consts.StatusInternalServerError, "AI API key not configured", nil)
		return
	}

	// 鉴权以及 AI 限制判断
	userID, patientID, identityType, aiAllowed, err := GetAIRequestUser(c, DB)
	if err != nil {
		ErrorResponse(c, consts.StatusUnauthorized, "未提供认证信息，请登录后重试", nil)
		return
	}

	if !aiAllowed {
		ErrorResponse(c, consts.StatusForbidden, "您的账号已被管理员限制访问AI功能，请联系客服或管理员", nil)
		return
	}

	var req AIChatRequest
	if err := c.Bind(&req); err != nil {
		ErrorResponse(c, consts.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	if req.Message == "" && len(req.Messages) == 0 {
		ErrorResponse(c, consts.StatusBadRequest, "Message is required", nil)
		return
	}

	// 将当前会话的身份存入 RequestContext，在异步/同步用量记录中使用
	c.Set("ai_user_id", userID)
	c.Set("ai_patient_id", patientID)
	c.Set("ai_identity_type", identityType)

	// 构建消息列表
	messages := buildMessages(req)

	if req.Stream {
		handleStreamChat(ctx, c, messages)
	} else {
		handleNormalChat(ctx, c, messages)
	}
}

// buildMessages 构建 AI 请求的消息列表
func buildMessages(req AIChatRequest) []OpenAIChatMessage {
	var messages []OpenAIChatMessage

	// 添加 system prompt
	if aiPrompt != "" {
		messages = append(messages, OpenAIChatMessage{
			Role:    "system",
			Content: aiPrompt,
		})
	}

	// 添加历史消息
	if len(req.Messages) > 0 {
		for _, msg := range req.Messages {
			role := "user"
			if msg.Role == "assistant" {
				role = "assistant"
			}
			messages = append(messages, OpenAIChatMessage{
				Role:    role,
				Content: msg.Content,
			})
		}
	}

	// 添加当前消息
	if req.Message != "" {
		messages = append(messages, OpenAIChatMessage{
			Role:    "user",
			Content: req.Message,
		})
	}

	return messages
}

// getChatCompletionsURL 获取聊天补全 API 端点
func getChatCompletionsURL() string {
	baseURL := strings.TrimSuffix(aiAPIURL, "/")
	if strings.HasSuffix(baseURL, "/v1") || strings.HasSuffix(baseURL, "/v2") {
		return baseURL + "/chat/completions"
	}
	return baseURL + "/v1/chat/completions"
}

func requestAICompletion(model string, messages interface{}) (string, error) {
	body, err := json.Marshal(map[string]interface{}{"model": model, "messages": messages, "stream": false})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, getChatCompletionsURL(), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+aiAPIKey)
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("AI service returned %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	var parsed OpenAIChatResponse
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("AI 服务未返回分析内容")
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}

func extractPDFText(file io.Reader) (string, error) {
	input, err := os.CreateTemp("", "patient-report-*.pdf")
	if err != nil {
		return "", err
	}
	inputPath := input.Name()
	outputPath := inputPath + ".txt"
	defer os.Remove(inputPath)
	defer os.Remove(outputPath)
	if _, err := io.Copy(input, io.LimitReader(file, 20*1024*1024+1)); err != nil {
		input.Close()
		return "", err
	}
	if err := input.Close(); err != nil {
		return "", err
	}
	command := exec.Command("pdftotext", "-layout", "-enc", "UTF-8", inputPath, outputPath)
	if output, err := command.CombinedOutput(); err != nil {
		return "", fmt.Errorf("PDF 文字提取失败，请确认服务器已安装 pdftotext: %v (%s)", err, strings.TrimSpace(string(output)))
	}
	text, err := os.ReadFile(outputPath)
	if err != nil {
		return "", err
	}
	result := strings.TrimSpace(string(text))
	if result == "" {
		return "", fmt.Errorf("PDF 中未提取到可分析文字；扫描版 PDF 请以图片方式上传")
	}
	runes := []rune(result)
	if len(runes) > 100000 {
		result = string(runes[:100000])
	}
	return result, nil
}

// AnalyzePatientReportReader 根据报告扩展名选择视觉或文本模型并返回结构化总结。
func AnalyzePatientReportReader(fileName string, file io.Reader) (string, string, error) {
	if strings.TrimSpace(aiAPIKey) == "" {
		return "", "", fmt.Errorf("AI API key not configured")
	}
	ext := strings.ToLower(filepath.Ext(strings.Split(fileName, "?")[0]))
	var model string
	var messages interface{}
	switch ext {
	case ".pdf":
		text, err := extractPDFText(file)
		if err != nil {
			return "", "", err
		}
		model = aiReportTextModel
		messages = []map[string]interface{}{
			{"role": "system", "content": aiReportPrompt},
			{"role": "user", "content": "请分析以下 PDF 报告提取文字：\n\n" + text},
		}
	case ".jpg", ".jpeg", ".png", ".webp":
		data, err := io.ReadAll(io.LimitReader(file, 20*1024*1024+1))
		if err != nil {
			return "", "", fmt.Errorf("读取图片失败: %w", err)
		}
		if len(data) > 20*1024*1024 {
			return "", "", fmt.Errorf("报告文件不能超过20MB")
		}
		mimeType := "image/" + strings.TrimPrefix(ext, ".")
		if ext == ".jpg" {
			mimeType = "image/jpeg"
		}
		model = aiReportVisionModel
		messages = []map[string]interface{}{
			{"role": "system", "content": aiReportPrompt},
			{"role": "user", "content": []map[string]interface{}{
				{"type": "text", "text": "请先识别这是检查/检验报告还是病理报告，再按规定结构客观总结。"},
				{"type": "image_url", "image_url": map[string]string{"url": "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)}},
			}},
		}
	default:
		return "", "", fmt.Errorf("仅支持 JPG、PNG、WEBP 或 PDF 报告")
	}
	content, err := requestAICompletion(model, messages)
	if err != nil {
		return "", model, err
	}
	return content, model, nil
}

// HandleAIAnalyzeReport 分析患者上传的图片或 PDF 报告。
func HandleAIAnalyzeReport(ctx context.Context, c *app.RequestContext) {
	if aiAPIKey == "" {
		ErrorResponse(c, consts.StatusInternalServerError, "AI API key not configured", nil)
		return
	}
	userID, patientID, identityType, allowed, err := GetAIRequestUser(c, DB)
	if err != nil {
		ErrorResponse(c, consts.StatusUnauthorized, "未提供认证信息，请登录后重试", nil)
		return
	}
	if !allowed {
		ErrorResponse(c, consts.StatusForbidden, "您的账号已被管理员限制访问AI功能", nil)
		return
	}
	header, err := c.FormFile("file")
	if err != nil {
		ErrorResponse(c, consts.StatusBadRequest, "请选择要分析的报告文件", nil)
		return
	}
	if header.Size > 20*1024*1024 {
		ErrorResponse(c, consts.StatusBadRequest, "报告文件不能超过20MB", nil)
		return
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	file, err := header.Open()
	if err != nil {
		ErrorResponse(c, consts.StatusBadRequest, "读取报告文件失败", nil)
		return
	}
	defer file.Close()

	switch ext {
	case ".pdf", ".jpg", ".jpeg", ".png", ".webp":
	default:
		ErrorResponse(c, consts.StatusBadRequest, "仅支持 JPG、PNG、WEBP 或 PDF 报告", nil)
		return
	}
	content, model, err := AnalyzePatientReportReader(header.Filename, file)
	if err != nil {
		log.Printf("AI report analysis failed: %v", err)
		ErrorResponse(c, consts.StatusBadGateway, "报告分析失败，请稍后重试", nil)
		return
	}
	if DB != nil {
		go RecordAIUsage(DB, estimateTokenCount(content), model, userID, patientID, identityType)
	}
	SuccessResponse(c, "分析成功", utils.H{"content": content, "model": model, "file_name": header.Filename})
}

// estimateTokenCount 估算token数量（按字符数估算，1 token ≈ 4 字符）
func estimateTokenCount(text string) int64 {
	return int64(len(text) / 4)
}

// handleNormalChat 处理普通聊天请求（非流式）
func handleNormalChat(ctx context.Context, c *app.RequestContext, messages []OpenAIChatMessage) {
	chatReq := OpenAIChatRequest{
		Model:    aiModel,
		Messages: messages,
		Stream:   false,
	}

	reqBody, err := json.Marshal(chatReq)
	if err != nil {
		log.Printf("JSON marshal error: %v", err)
		ErrorResponse(c, consts.StatusInternalServerError, "Failed to prepare request", err.Error())
		return
	}

	req, err := http.NewRequestWithContext(ctx, "POST", getChatCompletionsURL(), bytes.NewBuffer(reqBody))
	if err != nil {
		log.Printf("Create request error: %v", err)
		ErrorResponse(c, consts.StatusInternalServerError, "Failed to create request", err.Error())
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+aiAPIKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("AI chat error: %v", err)
		ErrorResponse(c, consts.StatusInternalServerError, "Failed to get AI response", err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("AI API error: %s - %s", resp.Status, string(bodyBytes))
		ErrorResponse(c, consts.StatusInternalServerError, fmt.Sprintf("AI API error: %s", resp.Status), string(bodyBytes))
		return
	}

	var chatResp OpenAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		log.Printf("Decode response error: %v", err)
		ErrorResponse(c, consts.StatusInternalServerError, "Failed to decode AI response", err.Error())
		return
	}

	if len(chatResp.Choices) > 0 {
		response := AIChatResponse{
			Content: chatResp.Choices[0].Message.Content,
		}
		SuccessResponse(c, "Success", response)

		// 记录token用量
		if DB != nil {
			tokenCount := estimateTokenCount(response.Content)
			userID, _ := c.Get("ai_user_id")
			patientID, _ := c.Get("ai_patient_id")
			identityType, _ := c.Get("ai_identity_type")
			uID, _ := userID.(int)
			pID, _ := patientID.(int)
			iType, _ := identityType.(string)
			go RecordAIUsage(DB, tokenCount, aiModel, uID, pID, iType)
		}
	} else {
		ErrorResponse(c, consts.StatusInternalServerError, "No response from AI", nil)
	}
}

// handleStreamChat 处理流式聊天请求
func handleStreamChat(ctx context.Context, c *app.RequestContext, messages []OpenAIChatMessage) {
	chatReq := OpenAIChatRequest{
		Model:    aiModel,
		Messages: messages,
		Stream:   true,
	}

	reqBody, err := json.Marshal(chatReq)
	if err != nil {
		log.Printf("JSON marshal error: %v", err)
		ErrorResponse(c, consts.StatusInternalServerError, "Failed to prepare request", err.Error())
		return
	}

	req, err := http.NewRequestWithContext(ctx, "POST", getChatCompletionsURL(), bytes.NewBuffer(reqBody))
	if err != nil {
		log.Printf("Create request error: %v", err)
		ErrorResponse(c, consts.StatusInternalServerError, "Failed to create request", err.Error())
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+aiAPIKey)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("AI chat error: %v", err)
		ErrorResponse(c, consts.StatusInternalServerError, "Failed to get AI response", err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("AI API error: %s - %s", resp.Status, string(bodyBytes))
		ErrorResponse(c, consts.StatusInternalServerError, fmt.Sprintf("AI API error: %s", resp.Status), string(bodyBytes))
		return
	}

	// 设置响应头
	c.SetContentType("text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.SetStatusCode(consts.StatusOK)

	// 读取流式响应并直接转发
	reader := resp.Body
	buf := make([]byte, 4096)
	var buffer bytes.Buffer
	var responseContent string

	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, err := reader.Read(buf)
			if n > 0 {
				buffer.Write(buf[:n])

				// 处理缓冲区中的数据
				for {
					line, err := buffer.ReadBytes('\n')
					if err != nil {
						// 将未读取的数据放回缓冲区
						buffer.Write(line)
						break
					}

					lineStr := string(line)
					lineStr = strings.TrimSpace(lineStr)

					if lineStr == "" {
						continue
					}

					if strings.HasPrefix(lineStr, "data: ") {
						dataStr := strings.TrimPrefix(lineStr, "data: ")
						dataStr = strings.TrimSpace(dataStr)

						if dataStr == "[DONE]" {
							fmt.Fprintf(c, "data: [DONE]\n\n")
							c.Flush()

							// 记录token用量
							if DB != nil && responseContent != "" {
								tokenCount := estimateTokenCount(responseContent)
								userID, _ := c.Get("ai_user_id")
								patientID, _ := c.Get("ai_patient_id")
								identityType, _ := c.Get("ai_identity_type")
								uID, _ := userID.(int)
								pID, _ := patientID.(int)
								iType, _ := identityType.(string)
								go RecordAIUsage(DB, tokenCount, aiModel, uID, pID, iType)
							}
							return
						}

						// 尝试解析并只转发内容，或者直接转发原始数据
						var chunk OpenAIStreamChunk
						if err := json.Unmarshal([]byte(dataStr), &chunk); err == nil {
							if len(chunk.Choices) > 0 {
								content := chunk.Choices[0].Delta.Content
								responseContent += content
								if content != "" {
									// 转发内容，保持 SSE 格式
									responseChunk := map[string]interface{}{
										"choices": []map[string]interface{}{
											{
												"delta": map[string]string{
													"content": content,
												},
											},
										},
									}
									chunkData, _ := json.Marshal(responseChunk)
									fmt.Fprintf(c, "data: %s\n\n", string(chunkData))
									c.Flush()
								}
							}
						} else {
							// 如果解析失败，直接转发原始数据
							fmt.Fprintf(c, "data: %s\n\n", dataStr)
							c.Flush()
						}
					}
				}
			}

			if err != nil {
				if err != io.EOF {
					log.Printf("Stream read error: %v", err)
				}
				fmt.Fprintf(c, "data: [DONE]\n\n")
				c.Flush()

				// 记录token用量
				if DB != nil && responseContent != "" {
					tokenCount := estimateTokenCount(responseContent)
					userID, _ := c.Get("ai_user_id")
					patientID, _ := c.Get("ai_patient_id")
					identityType, _ := c.Get("ai_identity_type")
					uID, _ := userID.(int)
					pID, _ := patientID.(int)
					iType, _ := identityType.(string)
					go RecordAIUsage(DB, tokenCount, aiModel, uID, pID, iType)
				}
				return
			}
		}
	}
}

// jsonEscape 转义 JSON 字符串
func jsonEscape(s string) string {
	data, _ := json.Marshal(s)
	return string(data)
}
