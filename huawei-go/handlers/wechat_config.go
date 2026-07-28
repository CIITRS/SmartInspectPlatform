package handlers

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
)

func getSystemSetting(db *sql.DB, key string) string {
	if db == nil || key == "" {
		return ""
	}
	var value string
	var encrypted int
	if err := db.QueryRow("SELECT key_value, is_encrypted FROM setting_system WHERE key_name = ? LIMIT 1", key).Scan(&value, &encrypted); err != nil {
		return ""
	}
	if encrypted == 1 {
		value = decryptConfigValue(value)
	}
	return strings.TrimSpace(value)
}

func getRuntimeSetting(key, envKey, fallback string) string {
	if value := getSystemSetting(GetDB(), key); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv(envKey)); value != "" {
		return value
	}
	return fallback
}

func getCartPath(fileName string) string {
	return filepath.Join("cart", fileName)
}
