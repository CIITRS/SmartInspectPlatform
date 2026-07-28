package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// 生成随机的10位英文数字组合
func generateRandomPath() (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const length = 10
	result := make([]byte, length)
	charsetLength := big.NewInt(int64(len(charset)))

	for i := range result {
		num, err := rand.Int(rand.Reader, charsetLength)
		if err != nil {
			return "", err
		}
		result[i] = charset[num.Int64()]
	}

	return string(result), nil
}

// 生成32位访问令牌
func generateAccessToken() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// 生成UUID
func generateUUID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:]), nil
}

// 根据文件路径确定使用哪个表
func getTableNameByPath(filePath string) string {
	normalizedPath := strings.ToLower(filePath)
	if strings.Contains(normalizedPath, "file/report") {
		return "base_files_report"
	}
	return "base_files_patient"
}

// 检查url_path是否已存在
func isURLPathExists(db *sql.DB, urlPath string, tableName string) (bool, error) {
	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE url_path = ?", tableName)
	err := db.QueryRow(query, urlPath).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// 生成安全的文件访问URL
func GenerateSecureFileURL(filePath string, originalName string, expiry time.Duration) (string, error) {
	// 使用全局DB变量
	db := DB
	if db == nil {
		return "", fmt.Errorf("database connection not initialized")
	}

	// 统一路径分隔符
	normalizedPath := strings.ReplaceAll(filePath, "\\", "/")
	// 处理可能的多余斜杠
	normalizedPath = strings.ReplaceAll(normalizedPath, "//", "/")

	log.Printf("处理文件路径: 原始=%s, 标准化=%s", filePath, normalizedPath)

	// 安全检查
	if !strings.HasPrefix(normalizedPath, "file/") {
		// 尝试添加file/前缀
		tempPath := "file/" + normalizedPath
		if strings.HasPrefix(tempPath, "file/file/") {
			tempPath = strings.TrimPrefix(tempPath, "file/")
		}
		if !strings.HasPrefix(tempPath, "file/") {
			return "", fmt.Errorf("invalid file path")
		}
		normalizedPath = tempPath
		log.Printf("修正后文件路径: %s", normalizedPath)
	}

	// 检查文件是否存在
	var validPath string
	if _, err := os.Stat(filePath); err == nil {
		validPath = filePath
	} else if _, err := os.Stat(normalizedPath); err == nil {
		validPath = normalizedPath
	} else {
		// 尝试不同的路径格式
		altPath1 := strings.ReplaceAll(filePath, "/", "\\")
		altPath2 := strings.ReplaceAll(normalizedPath, "/", "\\")

		if _, err := os.Stat(altPath1); err == nil {
			validPath = altPath1
		} else if _, err := os.Stat(altPath2); err == nil {
			validPath = altPath2
		} else {
			log.Printf("文件不存在: %s, %s, %s, %s", filePath, normalizedPath, altPath1, altPath2)
			return "", fmt.Errorf("file not found")
		}
	}
	filePath = validPath
	log.Printf("使用有效文件路径: %s", filePath)

	// 根据文件路径确定使用哪个表
	tableName := getTableNameByPath(filePath)
	log.Printf("使用数据库表: %s", tableName)

	// 首先根据原始文件名查询数据库，看是否存在未过期的链接
	var existingURLPath string
	var existingToken string
	var existingExpiresAt sql.NullTime
	var existingAccessCount int

	log.Printf("根据原始文件名查询数据库: originalName=%s", originalName)

	query := fmt.Sprintf("SELECT url_path, access_token, expires_at, access_count FROM %s WHERE original_name = ? AND expires_at > NOW() LIMIT 1", tableName)
	err := db.QueryRow(query, originalName).Scan(&existingURLPath, &existingToken, &existingExpiresAt, &existingAccessCount)

	if err == nil {
		// 找到未过期的链接，返回之前的链接
		log.Printf("找到未过期的链接: originalName=%s, urlPath=%s", originalName, existingURLPath)

		// 删除同样文件名的其他链接
		log.Printf("删除同样文件名的其他链接: originalName=%s,保留urlPath=%s", originalName, existingURLPath)
		updateQuery := fmt.Sprintf("UPDATE %s SET expires_at = NOW() WHERE original_name = ? AND url_path != ? AND expires_at > NOW()", tableName)
		_, err := db.Exec(updateQuery, originalName, existingURLPath)
		if err != nil {
			log.Printf("删除其他链接失败: %v", err)
			// 继续执行，不中断
		} else {
			log.Printf("删除其他链接成功: originalName=%s", originalName)
		}

		// 返回之前的链接
		return fmt.Sprintf("/api/file/view/%s?token=%s", existingURLPath, existingToken), nil
	} else if err != sql.ErrNoRows {
		// 发生其他错误
		log.Printf("查询数据库失败: %v", err)
		// 继续执行，生成新链接
	}

	// 没有找到未过期的链接，生成新链接

	// 使同一文件的旧链接失效
	log.Printf("使同一文件的旧链接失效: storage_path=%s", filePath)
	updateQuery := fmt.Sprintf("UPDATE %s SET expires_at = NOW() WHERE storage_path = ? AND expires_at > NOW()", tableName)
	_, err = db.Exec(updateQuery, filePath)
	if err != nil {
		log.Printf("使旧链接失效失败: %v", err)
		// 继续执行，不中断
	}

	// 生成唯一的url_path
	var urlPath string
	for i := 0; i < 10; i++ {
		urlPath, err = generateRandomPath()
		if err != nil {
			continue
		}
		exists, err := isURLPathExists(db, urlPath, tableName)
		if err != nil {
			continue
		}
		if !exists {
			break
		}
	}
	if urlPath == "" {
		return "", fmt.Errorf("failed to generate unique url path")
	}

	// 生成访问令牌
	token, err := generateAccessToken()
	if err != nil {
		return "", err
	}

	// 生成UUID作为文件ID
	fileID, err := generateUUID()
	if err != nil {
		log.Printf("生成UUID失败: %v", err)
		return "", err
	}
	log.Printf("生成UUID成功: %s", fileID)

	// 获取文件信息
	fileInfo, err := os.Stat(filePath)
	var fileSize int64
	if err == nil {
		fileSize = fileInfo.Size()
	}

	// 获取MIME类型
	mimeType := "application/octet-stream"
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".jpg", ".jpeg":
		mimeType = "image/jpeg"
	case ".png":
		mimeType = "image/png"
	case ".pdf":
		mimeType = "application/pdf"
	case ".doc":
		mimeType = "application/msword"
	case ".docx":
		mimeType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	}

	// 计算过期时间 - 30分钟
	expTime := time.Now().Add(30 * time.Minute)
	expiresAt := &expTime

	// 删除任何具有相同original_name的现有记录
	log.Printf("删除现有记录: originalName=%s", originalName)
	deleteQuery := fmt.Sprintf("DELETE FROM %s WHERE original_name = ?", tableName)
	_, err = db.Exec(deleteQuery, originalName)
	if err != nil {
		log.Printf("删除现有记录失败: %v", err)
		// 继续执行，不中断
	} else {
		log.Printf("删除现有记录成功: originalName=%s", originalName)
	}

	// 插入数据库
	log.Printf("准备插入数据库: fileID=%s, originalName=%s, filePath=%s, mimeType=%s, fileSize=%d, token=%s, expiresAt=%v, urlPath=%s",
		fileID, originalName, filePath, mimeType, fileSize, token, expiresAt, urlPath)

	insertQuery := fmt.Sprintf("INSERT INTO %s (id, original_name, storage_path, mime_type, size, access_token, access_count, expires_at, is_public, url_path) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", tableName)
	_, err = db.Exec(insertQuery, fileID, originalName, filePath, mimeType, fileSize, token, 0, expiresAt, false, urlPath)
	if err != nil {
		log.Printf("插入数据库失败: %v", err)
		return "", err
	}
	log.Printf("插入数据库成功: fileID=%s, urlPath=%s", fileID, urlPath)

	// 返回安全的访问URL
	return fmt.Sprintf("/api/file/view/%s?token=%s", urlPath, token), nil
}

// 根据url_path和token获取文件信息
func GetFileBySecurePath(urlPath string, token string) (string, string, bool, error) {
	// 使用全局DB变量
	db := DB
	if db == nil {
		log.Printf("数据库连接未初始化")
		return "", "", false, fmt.Errorf("database connection not initialized")
	}

	var storagePath string
	var expiresAt sql.NullTime
	var isPublic bool
	var accessCount int
	var foundTable string

	log.Printf("查询文件信息: urlPath=%s, token=%s", urlPath, token)

	// 先在 base_files_report 表中查找
	query := "SELECT storage_path, expires_at, is_public, access_count FROM base_files_report WHERE url_path = ? AND access_token = ?"
	err := db.QueryRow(query, urlPath, token).Scan(&storagePath, &expiresAt, &isPublic, &accessCount)

	if err == nil {
		foundTable = "base_files_report"
		log.Printf("在 base_files_report 表中找到文件")
	} else if err == sql.ErrNoRows {
		// 在 base_files_patient 表中查找
		query = "SELECT storage_path, expires_at, is_public, access_count FROM base_files_patient WHERE url_path = ? AND access_token = ?"
		err = db.QueryRow(query, urlPath, token).Scan(&storagePath, &expiresAt, &isPublic, &accessCount)
		if err == nil {
			foundTable = "base_files_patient"
			log.Printf("在 base_files_patient 表中找到文件")
		}
	}

	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("文件记录不存在: urlPath=%s, token=%s", urlPath, token)
			return "", "", false, nil
		}
		log.Printf("查询文件信息失败: %v", err)
		return "", "", false, err
	}

	log.Printf("文件信息查询成功: storagePath=%s, expiresAt=%v, isPublic=%v, accessCount=%d", storagePath, expiresAt, isPublic, accessCount)

	// 检查是否过期
	if !isPublic && expiresAt.Valid {
		if time.Now().After(expiresAt.Time) {
			log.Printf("文件链接已过期: urlPath=%s, expiresAt=%v", urlPath, expiresAt.Time)
			return "", "", false, nil
		}
	}

	// 检查访问次数是否超过限制（5次）
	if accessCount >= 5 {
		log.Printf("文件链接访问次数已达上限: urlPath=%s, accessCount=%d", urlPath, accessCount)
		// 设置链接过期
		updateQuery := fmt.Sprintf("UPDATE %s SET expires_at = NOW() WHERE url_path = ? AND access_token = ?", foundTable)
		_, err := db.Exec(updateQuery, urlPath, token)
		if err != nil {
			log.Printf("更新文件链接过期状态失败: %v", err)
		}
		return "", "", false, nil
	}

	// 增加访问次数
	updateQuery := fmt.Sprintf("UPDATE %s SET access_count = access_count + 1 WHERE url_path = ? AND access_token = ?", foundTable)
	_, err = db.Exec(updateQuery, urlPath, token)
	if err != nil {
		log.Printf("更新访问次数失败: %v", err)
		// 继续执行，不中断
	}

	log.Printf("文件链接访问成功: urlPath=%s, 过期时间=%v, 剩余访问次数=%d", urlPath, expiresAt, 5 - (accessCount + 1))

	return storagePath, "", true, nil
}
