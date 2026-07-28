package handlers

import (
	"database/sql"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// 全局数据库连接
var DB *sql.DB

// SetDB 设置数据库连接
func SetDB(db *sql.DB) {
	DB = db
}

// GetDB 从全局获取数据库连接
func GetDB() *sql.DB {
	return DB
}

// validateAndGetFile 验证文件路径并返回有效的文件路径
func validateAndGetFile(c *app.RequestContext, filePath string, isSecure bool) (string, error) {
	// 检查文件路径是否为空
	if filePath == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "文件路径不能为空",
			Data:    nil,
		})
		return "", fmt.Errorf("文件路径不能为空")
	}

	// 非安全模式下的路径检查
	if !isSecure {
		// 安全检查：确保文件路径在允许的范围内
		if !strings.HasPrefix(filePath, "file/") {
			c.JSON(403, ApiResponse{
				Code:    403,
				Success: false,
				Message: "无权访问该文件",
				Data:    nil,
			})
			return "", fmt.Errorf("无权访问该文件")
		}
	}

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(consts.StatusNotFound, ApiResponse{
			Code:    404,
			Success: false,
			Message: "文件不存在",
			Data:    nil,
		})
		return "", fmt.Errorf("文件不存在")
	}

	return filePath, nil
}

// setResponseHeaders 设置文件响应头
func setResponseHeaders(c *app.RequestContext, filePath string, disposition string) {
	// 获取文件扩展名
	ext := strings.ToLower(filepath.Ext(filePath))

	// 根据文件类型设置Content-Type
	contentType := "application/octet-stream"
	switch ext {
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".png":
		contentType = "image/png"
	case ".pdf":
		contentType = "application/pdf"
	case ".doc":
		contentType = "application/msword"
	case ".docx":
		contentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	}

	// 设置响应头
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf("%s; filename=%s", disposition, filepath.Base(filePath)))
	c.Header("Cache-Control", "public, max-age=259200")
}

// 处理文件查看请求
func HandleViewFile(c *app.RequestContext) {
	// 获取文件路径
	filePath := c.Query("path")

	// 验证文件路径
	if _, err := validateAndGetFile(c, filePath, false); err != nil {
		return
	}

	// 设置响应头
	setResponseHeaders(c, filePath, "inline")

	// 提供文件
	c.File(filePath)
}

// 处理文件下载请求
func HandleDownloadFile(c *app.RequestContext) {
	// 获取文件路径
	filePath := c.Query("path")

	// 验证文件路径
	if _, err := validateAndGetFile(c, filePath, false); err != nil {
		return
	}

	// 设置响应头（仅设置Content-Disposition，因为下载不需要Content-Type）
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(filePath)))

	// 提供文件
	c.File(filePath)
}

// 处理安全文件查看请求
func HandleSecureViewFile(c *app.RequestContext) {
	// 获取URL路径和令牌
	urlPath := c.Param("path")
	token := c.Query("token")

	// 处理URL编码的情况，当token被包含在path参数中时
	if token == "" {
		// 检查urlPath是否包含编码的查询参数
		if strings.Contains(urlPath, "%3Ftoken%3D") {
			// 解码并分离urlPath和token
			parts := strings.Split(urlPath, "%3Ftoken%3D")
			if len(parts) == 2 {
				urlPath = parts[0]
				token = parts[1]
			}
		} else if strings.Contains(urlPath, "?token=") {
			// 处理未编码的情况
			parts := strings.Split(urlPath, "?token=")
			if len(parts) == 2 {
				urlPath = parts[0]
				token = parts[1]
			}
		}
	}

	if urlPath == "" || token == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的文件链接",
			Data:    nil,
		})
		return
	}

	// 从全局DB获取数据库连接
	db := DB
	if db == nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "数据库连接失败",
			Data:    nil,
		})
		return
	}

	// 获取文件路径
	filePath, _, exists, err := GetFileBySecurePath(urlPath, token)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "获取文件信息失败",
			Data:    nil,
		})
		return
	}

	if !exists {
		c.JSON(consts.StatusNotFound, ApiResponse{
			Code:    404,
			Success: false,
			Message: "文件链接已过期或无效",
			Data:    nil,
		})
		return
	}

	// 验证文件路径
	if _, err := validateAndGetFile(c, filePath, true); err != nil {
		return
	}

	// 设置响应头
	setResponseHeaders(c, filePath, "inline")

	// 提供文件
	c.File(filePath)
}

// 处理安全文件下载请求
func HandleSecureDownloadFile(c *app.RequestContext) {
	// 获取URL路径和令牌
	urlPath := c.Param("path")
	token := c.Query("token")

	// 处理URL编码的情况，当token被包含在path参数中时
	if token == "" {
		// 检查urlPath是否包含编码的查询参数
		if strings.Contains(urlPath, "%3Ftoken%3D") {
			// 解码并分离urlPath和token
			parts := strings.Split(urlPath, "%3Ftoken%3D")
			if len(parts) == 2 {
				urlPath = parts[0]
				token = parts[1]
			}
		} else if strings.Contains(urlPath, "?token=") {
			// 处理未编码的情况
			parts := strings.Split(urlPath, "?token=")
			if len(parts) == 2 {
				urlPath = parts[0]
				token = parts[1]
			}
		}
	}

	if urlPath == "" || token == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的文件链接",
			Data:    nil,
		})
		return
	}

	// 从全局DB获取数据库连接
	db := DB
	if db == nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "数据库连接失败",
			Data:    nil,
		})
		return
	}

	// 获取文件路径
	filePath, _, exists, err := GetFileBySecurePath(urlPath, token)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "获取文件信息失败",
			Data:    nil,
		})
		return
	}

	if !exists {
		c.JSON(consts.StatusNotFound, ApiResponse{
			Code:    404,
			Success: false,
			Message: "文件链接已过期或无效",
			Data:    nil,
		})
		return
	}

	// 验证文件路径
	if _, err := validateAndGetFile(c, filePath, true); err != nil {
		return
	}

	// 设置响应头（仅设置Content-Disposition，因为下载不需要Content-Type）
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(filePath)))

	// 提供文件
	c.File(filePath)
}

// 处理公告文件上传请求
func HandleNoticeFileUpload(c *app.RequestContext, db *sql.DB) {
	// 获取上传的文件
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请选择要上传的文件",
			Data:    nil,
		})
		return
	}

	// 打开文件
	file, err := fileHeader.Open()
	if err != nil {
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
		".txt":  true,
		".zip":  true,
		".rar":  true,
	}

	ext := filepath.Ext(fileHeader.Filename)
	if !allowedExtensions[ext] {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "只允许上传JPG、PNG、PDF、DOC、DOCX、TXT、ZIP、RAR格式的文件",
			Data:    nil,
		})
		return
	}

	// 创建公告文件目录
	baseDir := "file"
	noticeDir := filepath.Join(baseDir, "notice")
	if err := os.MkdirAll(noticeDir, 0755); err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 生成唯一文件名
	timestamp := time.Now().Unix()
	randomStr := fmt.Sprintf("%d", timestamp)
	fileName := fmt.Sprintf("notice_%s_%d%s", randomStr, timestamp, ext)
	filePath := filepath.Join(noticeDir, fileName)

	// 创建目标文件
	dst, err := os.Create(filePath)
	if err != nil {
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
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 返回文件路径
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "文件上传成功",
		Data:    utils.H{"filePath": filePath, "fileName": fileHeader.Filename},
	})
}

// 通用文件上传处理函数
func handleFileUpload(c *app.RequestContext, db *sql.DB, fileType string, allowedExtensions map[string]bool, maxFileSize int64) {
	// 获取上传的文件
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请选择要上传的文件",
			Data:    nil,
		})
		return
	}

	// 检查文件大小
	if fileHeader.Size > maxFileSize {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: fmt.Sprintf("文件大小不能超过%dMB", maxFileSize/(1024*1024)),
			Data:    nil,
		})
		return
	}

	// 打开文件
	file, err := fileHeader.Open()
	if err != nil {
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
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if !allowedExtensions[ext] {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "文件类型不允许",
			Data:    nil,
		})
		return
	}

	qiniuConfig := loadQiniuStorageConfig()
	if qiniuConfig.configured() {
		objectName := buildCloudObjectName("uploads/"+fileType, fileHeader.Filename)
		if fileType == "patient-report" && db != nil {
			patientIdentifier := strings.TrimSpace(c.PostForm("patient_code"))
			patientID, patientCode, resolveErr := resolvePatientID(db, patientIdentifier, false)
			if resolveErr != nil || patientID <= 0 || !isValidPatientCode(patientCode) {
				c.JSON(consts.StatusBadRequest, ApiResponse{
					Code: 400, Success: false, Message: "患者编号无效", Data: nil,
				})
				return
			}
			objectName = buildPatientReportObjectName(patientCode, fileHeader.Filename, time.Now(), nextPatientReportSequence(db, patientID))
		}
		contentType := fileHeader.Header.Get("Content-Type")
		if contentType == "" {
			contentType = mime.TypeByExtension(ext)
		}
		fileURL, uploadErr := uploadFileToQiniu(file, objectName, fileHeader.Filename, contentType, qiniuConfig)
		if uploadErr != nil {
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code: 500, Success: false, Message: "文件上传失败",
				Data: utils.H{"error": uploadErr.Error()},
			})
			return
		}
		c.JSON(consts.StatusOK, ApiResponse{
			Code: 200, Success: true, Message: "文件上传成功",
			Data: utils.H{"url": fileURL, "path": objectName, "name": fileHeader.Filename},
		})
		return
	}

	// 创建文件目录
	baseDir := "file"
	uploadDir := filepath.Join(baseDir, fileType)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 生成唯一文件名
	timestamp := time.Now().Unix()
	randomStr := fmt.Sprintf("%d", timestamp)
	fileName := fmt.Sprintf("%s_%s_%d%s", fileType, randomStr, timestamp, ext)
	filePath := filepath.Join(uploadDir, fileName)

	// 创建目标文件
	dst, err := os.Create(filePath)
	if err != nil {
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
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 构建访问URL
	// 假设前端可以通过 /file/view?path= 访问文件
	accessUrl := "/file/view?path=" + filePath

	// 返回文件信息
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "文件上传成功",
		Data: utils.H{
			"url":  accessUrl,
			"path": filePath,
			"name": fileHeader.Filename,
		},
	})
}

// 处理图片上传请求
func HandleImageUpload(c *app.RequestContext) {
	allowedExtensions := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
		".webp": true,
	}
	maxFileSize := int64(5 * 1024 * 1024) // 5MB
	handleFileUpload(c, nil, "image", allowedExtensions, maxFileSize)
}

// 处理视频上传请求
func HandleVideoUpload(c *app.RequestContext) {
	allowedExtensions := map[string]bool{
		".mp4":  true,
		".avi":  true,
		".mov":  true,
		".wmv":  true,
		".flv":  true,
		".mkv":  true,
		".webm": true,
	}
	maxFileSize := int64(100 * 1024 * 1024) // 100MB
	handleFileUpload(c, nil, "video", allowedExtensions, maxFileSize)
}

// 处理附件上传请求
func HandleAttachmentUpload(c *app.RequestContext) {
	allowedExtensions := map[string]bool{
		".pdf":  true,
		".doc":  true,
		".docx": true,
		".txt":  true,
		".zip":  true,
		".rar":  true,
		".7z":   true,
		".xls":  true,
		".xlsx": true,
		".ppt":  true,
		".pptx": true,
		".csv":  true,
		".json": true,
		".xml":  true,
	}
	maxFileSize := int64(50 * 1024 * 1024) // 50MB
	handleFileUpload(c, nil, "attachment", allowedExtensions, maxFileSize)
}
