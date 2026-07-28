package handlers

import (
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// ApiResponse 定义统一的API响应结构
type ApiResponse struct {
	Code    int         `json:"code"`
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// JSON 发送JSON响应
func JSON(c *app.RequestContext, statusCode int, data interface{}) {
	c.JSON(statusCode, data)
}

// SuccessResponse 发送成功响应
func SuccessResponse(c *app.RequestContext, message string, data interface{}) {
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: message,
		Data:    data,
	})
}

// ErrorResponse 发送错误响应
func ErrorResponse(c *app.RequestContext, statusCode int, message string, data interface{}) {
	c.JSON(statusCode, ApiResponse{
		Code:    statusCode,
		Success: false,
		Message: message,
		Data:    data,
	})
}

// GetUserID 从上下文获取用户ID
func GetUserID(c *app.RequestContext) (int, bool) {
	userID, exists := c.Get(UserIDKey)
	if !exists {
		return 0, false
	}
	id, ok := userID.(int)
	return id, ok
}
