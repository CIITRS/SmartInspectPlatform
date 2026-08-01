package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// FileURLManager 管理文件临时URL
type FileURLManager struct {
	urls            map[string]*FileURL
	mutex           sync.RWMutex
	cleanupInterval time.Duration
}

// FileURL 表示一个临时文件URL
type FileURL struct {
	FilePath    string
	ExpiresAt   time.Time
	AccessCount int
	OneTimeUse  bool // 标记是否为一次性使用令牌
}

const (
	generatedReportRetentionAfterDownload = 2 * time.Minute
	generatedReportOrphanMaxAge           = 30 * time.Minute
)

// 全局文件URL管理器
var fileURLManager *FileURLManager

// 初始化文件URL管理器
func InitFileURLManager() {
	fileURLManager = &FileURLManager{
		urls:            make(map[string]*FileURL),
		cleanupInterval: time.Minute,
	}
	cleanupManagedTemporaryFiles(generatedReportOrphanMaxAge)
	// 启动清理过期URL的goroutine
	go fileURLManager.cleanupExpiredURLs()
}

// 清理过期的URL
func (m *FileURLManager) cleanupExpiredURLs() {
	ticker := time.NewTicker(m.cleanupInterval)
	defer ticker.Stop()

	for {
		<-ticker.C
		var expiredPaths []string
		m.mutex.Lock()
		now := time.Now()
		for id, url := range m.urls {
			if now.After(url.ExpiresAt) {
				expiredPaths = append(expiredPaths, url.FilePath)
				delete(m.urls, id)
			}
		}
		m.mutex.Unlock()
		for _, filePath := range expiredPaths {
			removeManagedTemporaryFile(filePath)
		}
		cleanupManagedTemporaryFiles(generatedReportOrphanMaxAge)
	}
}

func isManagedTemporaryFile(filePath string) bool {
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return false
	}
	cleanPath, err := filepath.Abs(filepath.Clean(filePath))
	if err != nil {
		return false
	}
	for _, directory := range []string{
		filepath.Join("file", "temp", "detect_report"),
		filepath.Join("file", "temp", "detect_report_preview"),
	} {
		cleanDirectory, err := filepath.Abs(directory)
		if err != nil {
			continue
		}
		if strings.HasPrefix(cleanPath, cleanDirectory+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

func removeManagedTemporaryFile(filePath string) {
	if !isManagedTemporaryFile(filePath) {
		return
	}
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		log.Printf("删除临时报告文件失败 %s: %v", filePath, err)
	}
}

func scheduleManagedTemporaryFileRemovalAfter(filePath string, retention time.Duration) {
	if !isManagedTemporaryFile(filePath) {
		return
	}
	time.AfterFunc(retention, func() {
		removeManagedTemporaryFile(filePath)
	})
}

func scheduleManagedTemporaryFileRemoval(filePath string) {
	scheduleManagedTemporaryFileRemovalAfter(filePath, generatedReportRetentionAfterDownload)
}

func cleanupManagedTemporaryFiles(maxAge time.Duration) {
	cutoff := time.Now().Add(-maxAge)
	for _, directory := range []string{
		filepath.Join("file", "temp", "detect_report"),
		filepath.Join("file", "temp", "detect_report_preview"),
	} {
		entries, err := os.ReadDir(directory)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			info, err := entry.Info()
			if err == nil && info.ModTime().Before(cutoff) {
				removeManagedTemporaryFile(filepath.Join(directory, entry.Name()))
			}
		}
	}
}

// 生成临时文件URL
func (m *FileURLManager) GenerateFileURL(filePath string, expiry time.Duration) (string, error) {
	return m.generateFileURLInternal(filePath, expiry, false)
}

// 生成一次性使用的文件URL（使用后立即失效）
func (m *FileURLManager) GenerateOneTimeFileURL(filePath string, expiry time.Duration) (string, error) {
	return m.generateFileURLInternal(filePath, expiry, true)
}

// 内部实现：生成文件URL
func (m *FileURLManager) generateFileURLInternal(filePath string, expiry time.Duration, oneTimeUse bool) (string, error) {
	// 统一路径分隔符，将所有反斜杠（包括双反斜杠）替换为正斜杠
	normalizedPath := strings.ReplaceAll(filePath, "\\", "/")
	// 再次替换，确保所有反斜杠都被替换
	normalizedPath = strings.ReplaceAll(normalizedPath, "\\", "/")

	log.Printf("处理文件路径: 原始=%s, 标准化=%s", filePath, normalizedPath)

	// 安全检查：确保文件路径在允许的范围内
	if !strings.HasPrefix(normalizedPath, "file/") {
		return "", fmt.Errorf("invalid file path")
	}

	// 检查文件是否存在，使用原始路径进行文件系统操作
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// 尝试使用标准化路径检查
		if _, err := os.Stat(normalizedPath); os.IsNotExist(err) {
			return "", fmt.Errorf("file not found")
		}
		// 如果标准化路径存在，使用标准化路径
		filePath = normalizedPath
	}

	// 生成唯一ID
	id, err := generateRandomID(16)
	if err != nil {
		return "", err
	}

	// 创建临时URL记录
	m.mutex.Lock()
	m.urls[id] = &FileURL{
		FilePath:    filePath,
		ExpiresAt:   time.Now().Add(expiry),
		AccessCount: 0,
		OneTimeUse:  oneTimeUse,
	}
	m.mutex.Unlock()

	// 返回临时URL
	if oneTimeUse {
		return fmt.Sprintf("/api/downloads/%s", id), nil
	}
	return fmt.Sprintf("/api/file/temp/%s", id), nil
}

// 获取临时URL对应的文件路径
func (m *FileURLManager) GetFileByID(id string) (string, bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	url, exists := m.urls[id]
	if !exists {
		return "", false
	}

	// 检查是否过期
	if time.Now().After(url.ExpiresAt) {
		delete(m.urls, id)
		return "", false
	}

	// 增加访问计数
	url.AccessCount++

	// 如果是一次性使用令牌，立即删除
	if url.OneTimeUse {
		delete(m.urls, id)
	}

	return url.FilePath, true
}

// 生成随机ID
func generateRandomID(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// 处理临时文件访问请求
func HandleTempFileAccess(c *app.RequestContext) {
	// 获取临时URL ID
	id := c.Param("id")
	if id == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的文件链接",
			Data:    nil,
		})
		return
	}

	// 获取文件路径
	filePath, exists := fileURLManager.GetFileByID(id)
	if !exists {
		c.JSON(consts.StatusNotFound, ApiResponse{
			Code:    404,
			Success: false,
			Message: "文件链接已过期或无效",
			Data:    nil,
		})
		return
	}

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
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=%s", filepath.Base(filePath)))
	c.Header("Cache-Control", "public, max-age=259200")

	// 提供文件
	c.File(filePath)
}

// 处理生成临时文件URL的请求
func HandleGenerateTempFileURL(c *app.RequestContext) {
	// 获取文件路径
	filePath := c.Query("path")
	if filePath == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "文件路径不能为空",
			Data:    nil,
		})
		return
	}

	// 生成3天过期的临时URL
	tempURL, err := fileURLManager.GenerateFileURL(filePath, 72*time.Hour)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "生成临时链接失败: " + err.Error(),
			Data:    nil,
		})
		return
	}

	// 返回临时URL
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "生成临时链接成功",
		Data:    utils.H{"tempUrl": tempURL},
	})
}

// 处理一次性下载URL访问的请求
func HandleOneTimeDownload(c *app.RequestContext) {
	// 获取下载token
	token := c.Param("token")
	if token == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的下载链接",
			Data:    nil,
		})
		return
	}

	// 获取文件路径
	filePath, exists := fileURLManager.GetFileByID(token)
	if !exists {
		c.JSON(consts.StatusNotFound, ApiResponse{
			Code:    404,
			Success: false,
			Message: "下载链接已过期或无效",
			Data:    nil,
		})
		return
	}

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
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(filePath)))

	fileData, err := os.ReadFile(filePath)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "读取文件失败",
			Data:    nil,
		})
		return
	}

	c.Data(consts.StatusOK, contentType, fileData)
	scheduleManagedTemporaryFileRemoval(filePath)
}
