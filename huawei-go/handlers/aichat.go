package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

var (
	aiAPIKey   string
	aiAPIURL   string
	aiModel    string
	aiPrompt   string
	httpClient *http.Client
)

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
