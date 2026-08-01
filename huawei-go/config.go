package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"fmt"
	"log"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

// Config 系统配置结构体
type Config struct {
	DBHost        string
	DBPort        string
	DBUser        string
	DBPassword    string
	DBName        string
	Port          string
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	AIAPIKey      string
	AIAPIURL      string
	AIModel       string
	AIPrompt      string
}

// RSA密钥对
type RSAKeys struct {
	PrivateKey *rsa.PrivateKey
	PublicKey  *rsa.PublicKey
}

// 全局RSA密钥对
var RSAKeyPair *RSAKeys

// LoadConfig 加载配置
func LoadConfig() *Config {
	// 加载环境变量
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: No .env file found")
	}

	// 构建配置
	config := &Config{
		DBHost:        getEnv("DB_HOST", "localhost"),
		DBPort:        getEnv("DB_PORT", "3306"),
		DBUser:        getEnv("DB_USER", "root"),
		DBPassword:    getEnv("DB_PASSWORD", ""),
		DBName:        getEnv("DB_NAME", "huawei_micro_diagnosis"),
		Port:          getEnv("PORT", "3001"),
		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       0,
		AIAPIKey:      getEnv("AI_API_KEY", ""),
		AIAPIURL:      getEnv("AI_API_URL", "https://qianfan.baidubce.com/v2/chat/completions"),
		AIModel:       getEnv("AI_MODEL", "ernie-lite-pro-128k"),
		AIPrompt:      getEnv("AI_PROMPT", ""),
	}

	return config
}

// GenerateRSAKeys 生成RSA密钥对
func GenerateRSAKeys() (*RSAKeys, error) {
	keyFile := "private.pem"

	// 尝试从文件读取私钥
	if _, err := os.Stat(keyFile); err == nil {
		keyBytes, err := os.ReadFile(keyFile)
		if err == nil {
			block, _ := pem.Decode(keyBytes)
			if block != nil {
				privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
				if err == nil {
					log.Println("已加载现有的RSA密钥对")
					return &RSAKeys{
						PrivateKey: privateKey,
						PublicKey:  &privateKey.PublicKey,
					}, nil
				}
			}
		}
		log.Printf("读取或解析密钥文件失败，将重新生成: %v", err)
	}

	// 生成2048位RSA密钥对
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	// 保存私钥到文件
	keyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	pemBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyBytes,
	}

	file, err := os.Create(keyFile)
	if err != nil {
		log.Printf("无法保存私钥文件: %v", err)
	} else {
		defer file.Close()
		if err := pem.Encode(file, pemBlock); err != nil {
			log.Printf("无法写入私钥内容: %v", err)
		} else {
			log.Println("已生成并保存新的RSA密钥对")
		}
	}

	return &RSAKeys{
		PrivateKey: privateKey,
		PublicKey:  &privateKey.PublicKey,
	}, nil
}

// GetPublicKeyPEM 获取公钥的PEM格式字符串
func (keys *RSAKeys) GetPublicKeyPEM() (string, error) {
	// 编码公钥为PEM格式
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(keys.PublicKey)
	if err != nil {
		return "", err
	}

	pemBlock := &pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: publicKeyBytes,
	}

	return string(pem.EncodeToMemory(pemBlock)), nil
}

// DecryptRSA 解密RSA加密的数据
func (keys *RSAKeys) DecryptRSA(encryptedData []byte) ([]byte, error) {
	// 解密数据
	return rsa.DecryptPKCS1v15(rand.Reader, keys.PrivateKey, encryptedData)
}

// getEnv 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// InitDB 初始化数据库连接
func InitDB(config *Config) (*sql.DB, error) {
	// 构建DSN (Data Source Name)
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		config.DBUser,
		config.DBPassword,
		config.DBHost,
		config.DBPort,
		config.DBName,
	)

	// 打开数据库连接
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	// 测试数据库连接
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

// EnsureSchema 确保数据库表结构存在
func EnsureSchema(db *sql.DB, dbName string) error {
	// 创建必要的表结构
	createTablesSQL := []string{
		`CREATE TABLE IF NOT EXISTS base_sessions (
			id INT AUTO_INCREMENT PRIMARY KEY,
			session_id VARCHAR(255) NOT NULL UNIQUE,
			user_id INT NOT NULL,
			expiry DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE IF NOT EXISTS base_miniapp_sessions (
			id INT AUTO_INCREMENT PRIMARY KEY,
			session_id VARCHAR(255) NOT NULL UNIQUE,
			user_id INT DEFAULT 0,
			phone VARCHAR(20) NOT NULL,
			identity_type VARCHAR(20) NOT NULL,
			patient_id INT DEFAULT 0,
			expiry DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE IF NOT EXISTS base_sms_codes (
			id INT AUTO_INCREMENT PRIMARY KEY,
			phone VARCHAR(20) NOT NULL,
			code VARCHAR(10) NOT NULL,
			purpose VARCHAR(50) NOT NULL DEFAULT 'miniapp_login',
			expires_at DATETIME NOT NULL,
			used TINYINT(1) DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			KEY idx_sms_codes_phone_purpose (phone, purpose),
			KEY idx_sms_codes_expires_at (expires_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE IF NOT EXISTS base_sms_send_log (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			purpose VARCHAR(50) NOT NULL DEFAULT '',
			mobile VARCHAR(30) NOT NULL DEFAULT '',
			template_id VARCHAR(100) NOT NULL DEFAULT '',
			status VARCHAR(20) NOT NULL DEFAULT 'sending',
			provider_code VARCHAR(50) NOT NULL DEFAULT '',
			provider_message VARCHAR(500) NOT NULL DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			KEY idx_sms_send_log_purpose (purpose),
			KEY idx_sms_send_log_status (status),
			KEY idx_sms_send_log_mobile (mobile),
			KEY idx_sms_send_log_created_at (created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE IF NOT EXISTS base_phone_bind_tokens (
			id INT AUTO_INCREMENT PRIMARY KEY,
			token VARCHAR(100) NOT NULL UNIQUE,
			user_id INT NOT NULL,
			client VARCHAR(20) NOT NULL DEFAULT 'admin',
			used TINYINT(1) DEFAULT 0,
			expires_at DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			KEY idx_phone_bind_token_user (user_id),
			KEY idx_phone_bind_token_expiry (expires_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE IF NOT EXISTS setting_role (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(100) NOT NULL UNIQUE,
			description VARCHAR(255) DEFAULT '',
			status TINYINT(1) DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE IF NOT EXISTS setting_role_permission (
			id INT AUTO_INCREMENT PRIMARY KEY,
			role_id INT NOT NULL,
			page_id VARCHAR(100) NOT NULL,
			page_name VARCHAR(100) DEFAULT '',
			parent_page_id VARCHAR(100) DEFAULT '',
			checked TINYINT(1) DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			KEY idx_role_permission_role (role_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE IF NOT EXISTS base_manage_user_permission (
			id INT AUTO_INCREMENT PRIMARY KEY,
			user_id INT NOT NULL,
			page_id VARCHAR(100) NOT NULL,
			page_name VARCHAR(100) DEFAULT '',
			parent_page_id VARCHAR(100) DEFAULT '',
			checked TINYINT(1) DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uk_user_permission (user_id, page_id),
			KEY idx_user_permission_user (user_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE IF NOT EXISTS base_manage_user_role (
			id INT AUTO_INCREMENT PRIMARY KEY,
			user_id INT NOT NULL,
			role_id INT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uk_manage_user_role (user_id, role_id),
			KEY idx_manage_user_role_user (user_id),
			KEY idx_manage_user_role_role (role_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE IF NOT EXISTS base_manage_user (
			id INT AUTO_INCREMENT PRIMARY KEY,
			username VARCHAR(100) NOT NULL UNIQUE,
			password VARCHAR(255) NOT NULL,
			salt VARCHAR(64) NOT NULL,
			real_name VARCHAR(100) NOT NULL,
			phone VARCHAR(20) DEFAULT '',
			department_id INT DEFAULT NULL,
			role_id INT DEFAULT NULL,
			status TINYINT(1) DEFAULT 1,
			last_login_time DATETIME DEFAULT NULL,
			employee_id VARCHAR(50) DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			KEY idx_manage_user_employee (employee_id),
			KEY idx_manage_user_phone (phone)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE IF NOT EXISTS base_announcements (
			id INT AUTO_INCREMENT PRIMARY KEY,
			title VARCHAR(255) NOT NULL,
			content TEXT,
			user_id INT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE IF NOT EXISTS detect_patient (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			id_document_type VARCHAR(50) DEFAULT '居民身份证',
			id_document_no VARCHAR(100) DEFAULT '',
			id_card VARCHAR(18) UNIQUE,
			phone VARCHAR(20) NOT NULL,
			gender VARCHAR(10),
			birth_date DATE,
			is_active TINYINT(1) DEFAULT 1,
			completion_status TINYINT(1) DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE IF NOT EXISTS detect_sample (
			id INT AUTO_INCREMENT PRIMARY KEY,
			patient_id INT,
			sample_code VARCHAR(100) UNIQUE NOT NULL,
			sample_type_id INT DEFAULT NULL,
			cancer_type_id INT DEFAULT NULL,
			treatment_stage_id INT DEFAULT NULL,
			collection_date DATETIME DEFAULT NULL,
			collection_operator INT DEFAULT NULL,
			receive_date DATETIME DEFAULT NULL,
			receive_operator INT DEFAULT NULL,
			sample_status VARCHAR(20) DEFAULT 'pending',
			report_type VARCHAR(50) DEFAULT 'normal',
			notes TEXT,
			organization VARCHAR(255) DEFAULT '',
			sample_created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			sample_updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			FOREIGN KEY (patient_id) REFERENCES detect_patient(id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE IF NOT EXISTS detect_sample_code_pool (
			id INT AUTO_INCREMENT PRIMARY KEY,
			sample_code VARCHAR(100) NOT NULL UNIQUE,
			prefix VARCHAR(100) NOT NULL,
			sequence_no INT NOT NULL,
			recycled_by INT DEFAULT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'available',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			used_at DATETIME DEFAULT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			KEY idx_sample_code_pool_available (prefix, status, sequence_no)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		// Panel管理表
		`CREATE TABLE IF NOT EXISTS setting_panel (
			id INT AUTO_INCREMENT PRIMARY KEY,
			panel_name VARCHAR(100) NOT NULL,
			panel_code VARCHAR(50) UNIQUE NOT NULL,
			description TEXT,
			is_active TINYINT(1) DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		// 基因与Panel关联表（多对多关系）
		// CREATE TABLE IF NOT EXISTS gene_panel_relation (
		// 	id INT AUTO_INCREMENT PRIMARY KEY,
		// 	gene_id INT NOT NULL,
		// 	panel_id INT NOT NULL,
		// 	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		// 	UNIQUE KEY unique_gene_panel (gene_id, panel_id),
		// 	FOREIGN KEY (gene_id) REFERENCES setting_gene(id) ON DELETE CASCADE,
		// 	FOREIGN KEY (panel_id) REFERENCES setting_panel(id) ON DELETE CASCADE
		// ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE IF NOT EXISTS setting_model_gene_threshold (
			id INT AUTO_INCREMENT PRIMARY KEY,
			model_id INT NOT NULL,
			gene_id INT NOT NULL,
			threshold DECIMAL(12,4) NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY unique_model_gene_threshold (model_id, gene_id),
			KEY idx_model_gene_threshold_model (model_id),
			KEY idx_model_gene_threshold_gene (gene_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		// AI 使用量记录表
		`CREATE TABLE IF NOT EXISTS setting_ai_usage_log (
			id INT AUTO_INCREMENT PRIMARY KEY,
			token_count INT NOT NULL,
			model VARCHAR(100) NOT NULL,
			user_id INT DEFAULT 0,
			patient_id INT DEFAULT 0,
			identity_type VARCHAR(20) DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE IF NOT EXISTS setting_ai_blacklist (
			id INT AUTO_INCREMENT PRIMARY KEY,
			subject_type VARCHAR(20) NOT NULL,
			subject_code VARCHAR(100) NOT NULL,
			subject_name VARCHAR(100) DEFAULT '',
			reason VARCHAR(255) DEFAULT '',
			created_by INT DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uk_ai_blacklist_subject (subject_type, subject_code),
			KEY idx_ai_blacklist_subject_code (subject_code)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE IF NOT EXISTS patient_report_analysis (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			patient_id INT NOT NULL,
			file_key CHAR(64) NOT NULL,
			file_url TEXT NOT NULL,
			file_name VARCHAR(255) DEFAULT '',
			file_type VARCHAR(20) DEFAULT '',
			report_type VARCHAR(50) DEFAULT '',
			hospital VARCHAR(255) DEFAULT '',
			examination_time VARCHAR(100) DEFAULT '',
			examination_item VARCHAR(255) DEFAULT '',
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			analysis_text LONGTEXT,
			model VARCHAR(100) DEFAULT '',
			error_message VARCHAR(500) DEFAULT '',
			edited_by INT DEFAULT 0,
			edited_at DATETIME DEFAULT NULL,
			analyzed_at DATETIME DEFAULT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uk_patient_report_analysis (patient_id, file_key),
			KEY idx_patient_report_analysis_status (status),
			KEY idx_patient_report_analysis_patient (patient_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE IF NOT EXISTS setting_system (
			id INT AUTO_INCREMENT PRIMARY KEY,
			key_name VARCHAR(100) NOT NULL UNIQUE,
			key_value TEXT,
			key_type VARCHAR(50) DEFAULT 'text',
			is_encrypted TINYINT(1) DEFAULT 0,
			description VARCHAR(255) DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE IF NOT EXISTS sale_patient_group (
			id INT AUTO_INCREMENT PRIMARY KEY,
			sales_user_id INT NOT NULL,
			name VARCHAR(100) NOT NULL,
			color VARCHAR(20) DEFAULT '#1677ff',
			sort_order INT DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uk_sale_patient_group_owner_name (sales_user_id, name),
			KEY idx_sale_patient_group_owner (sales_user_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE IF NOT EXISTS sale_patient_group_member (
			id INT AUTO_INCREMENT PRIMARY KEY,
			group_id INT NOT NULL,
			patient_id INT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE KEY uk_sale_patient_group_member (group_id, patient_id),
			KEY idx_sale_patient_group_member_patient (patient_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE IF NOT EXISTS patient_informed_consent (
			id INT AUTO_INCREMENT PRIMARY KEY,
			patient_id INT NOT NULL,
			consent_version VARCHAR(30) NOT NULL DEFAULT 'v1',
			consent_text LONGTEXT NOT NULL,
			signature_data LONGTEXT NOT NULL,
			signed_name VARCHAR(100) DEFAULT '',
			signed_by_user_id INT DEFAULT 0,
			signed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uk_patient_consent_once (patient_id),
			KEY idx_patient_consent_signed_at (signed_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE IF NOT EXISTS setting_report_position (
			id INT AUTO_INCREMENT PRIMARY KEY,
			position_key VARCHAR(50) NOT NULL UNIQUE,
			position_name VARCHAR(100) NOT NULL,
			sample_type_id INT NOT NULL DEFAULT 0,
			report_type VARCHAR(50) NOT NULL,
			page_number INT NOT NULL DEFAULT 3,
			background_path VARCHAR(255) NOT NULL,
			positions_json LONGTEXT NOT NULL,
			is_active TINYINT(1) DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uk_report_position_assignment (sample_type_id, report_type),
			KEY idx_report_position_match (sample_type_id, report_type, is_active)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE IF NOT EXISTS detect_sample_panel_match (
			id INT AUTO_INCREMENT PRIMARY KEY,
			batch_id INT NOT NULL,
			sample_code VARCHAR(100) NOT NULL,
			matched_panel_ids_json LONGTEXT,
			panel_matches_json LONGTEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uk_sample_panel_match (batch_id, sample_code),
			KEY idx_sample_panel_match_batch (batch_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE IF NOT EXISTS detect_sample_express (
			id INT AUTO_INCREMENT PRIMARY KEY,
			sample_id INT NOT NULL,
			sample_code VARCHAR(100) NOT NULL,
			direction VARCHAR(20) NOT NULL DEFAULT 'inbound',
			express_type VARCHAR(50) NOT NULL DEFAULT 'auto',
			express_company VARCHAR(100),
			tracking_number VARCHAR(100) NOT NULL,
			query_mobile VARCHAR(20),
			sender_name VARCHAR(100),
			sender_phone VARCHAR(20),
			sender_address VARCHAR(255),
			receiver_name VARCHAR(100),
			receiver_phone VARCHAR(20),
			receiver_address VARCHAR(255),
			send_time DATETIME,
			receive_time DATETIME,
			delivered_at DATETIME,
			status VARCHAR(30) DEFAULT 'pending',
			provider_status INT DEFAULT NULL,
			provider_message VARCHAR(255) DEFAULT '',
			route_json LONGTEXT,
			latest_event_time DATETIME,
			latest_event_status VARCHAR(500) DEFAULT '',
			last_query_at DATETIME,
			last_query_error VARCHAR(500) DEFAULT '',
			notes TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uk_sample_express_current (sample_id, direction),
			KEY idx_sample_express_sample (sample_id),
			KEY idx_sample_express_tracking (tracking_number)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE IF NOT EXISTS patient_follow_up (
			id INT AUTO_INCREMENT PRIMARY KEY,
			patient_id INT NOT NULL,
			phone VARCHAR(20) NOT NULL DEFAULT '',
			diagnosis_info TEXT,
			report_notes TEXT,
			images_json LONGTEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			KEY idx_follow_up_patient (patient_id),
			KEY idx_follow_up_phone (phone),
			KEY idx_follow_up_created (created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE IF NOT EXISTS sale_package (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			detection_count INT NOT NULL DEFAULT 1,
			interval_days INT NOT NULL DEFAULT 90,
			price DECIMAL(10,2) NOT NULL DEFAULT 0,
			description TEXT,
			status VARCHAR(20) DEFAULT 'active',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			KEY idx_sale_package_status (status)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE IF NOT EXISTS sale_order (
			id INT AUTO_INCREMENT PRIMARY KEY,
			sale_order_no VARCHAR(64) UNIQUE NOT NULL,
			detect_patient_id INT NOT NULL,
			detect_patient_id_card VARCHAR(18) DEFAULT '',
			sale_package_id INT NOT NULL,
			setting_cancer_type_id INT DEFAULT NULL,
			first_detection_date DATE DEFAULT NULL,
			payment_method VARCHAR(50) DEFAULT '',
			payment_status VARCHAR(30) DEFAULT 'pending',
			sales_person_id INT DEFAULT NULL,
			total_amount DECIMAL(10,2) DEFAULT 0,
			status VARCHAR(30) DEFAULT 'pending',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			KEY idx_sale_order_patient (detect_patient_id),
			KEY idx_sale_order_package (sale_package_id),
			KEY idx_sale_order_sales (sales_person_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE IF NOT EXISTS sale_detection_plan (
			id INT AUTO_INCREMENT PRIMARY KEY,
			sale_order_id INT NOT NULL,
			detect_patient_id INT NOT NULL,
			detection_date DATE DEFAULT NULL,
			detection_number INT NOT NULL DEFAULT 1,
			status VARCHAR(30) DEFAULT 'scheduled',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			KEY idx_detection_plan_order (sale_order_id),
			KEY idx_detection_plan_patient (detect_patient_id),
			KEY idx_detection_plan_date (detection_date)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE IF NOT EXISTS sale_sample_box_request (
			id INT AUTO_INCREMENT PRIMARY KEY,
			sale_order_id INT DEFAULT NULL,
			detection_plan_id INT DEFAULT NULL,
			detect_patient_id INT NOT NULL,
			receiver_name VARCHAR(100) DEFAULT '',
			receiver_phone VARCHAR(20) DEFAULT '',
			receiver_address VARCHAR(255) DEFAULT '',
			expected_send_date DATE DEFAULT NULL,
			status VARCHAR(30) DEFAULT 'requested',
			notes TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			KEY idx_box_request_patient (detect_patient_id),
			KEY idx_box_request_order (sale_order_id),
			KEY idx_box_request_plan (detection_plan_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE IF NOT EXISTS mail_address (
			id INT AUTO_INCREMENT PRIMARY KEY,
			detect_patient_id INT NOT NULL,
			sale_order_id INT DEFAULT NULL,
			detection_plan_id INT DEFAULT NULL,
			receiver_name VARCHAR(100) DEFAULT '',
			receiver_phone VARCHAR(20) DEFAULT '',
			province VARCHAR(50) DEFAULT '',
			city VARCHAR(50) DEFAULT '',
			district VARCHAR(50) DEFAULT '',
			detail_address VARCHAR(255) DEFAULT '',
			full_address VARCHAR(500) DEFAULT '',
			express_company VARCHAR(100) DEFAULT '',
			tracking_number VARCHAR(100) DEFAULT '',
			express_status VARCHAR(30) DEFAULT 'pending',
			express_route_json LONGTEXT,
			express_delivered_at DATETIME DEFAULT NULL,
			express_last_query_at DATETIME DEFAULT NULL,
			express_last_query_error VARCHAR(500) DEFAULT '',
			status VARCHAR(30) DEFAULT 'requested',
			notes TEXT,
			shipped_at DATETIME DEFAULT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			KEY idx_mail_address_patient (detect_patient_id),
			KEY idx_mail_address_order (sale_order_id),
			KEY idx_mail_address_plan (detection_plan_id),
			KEY idx_mail_address_status (status)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE IF NOT EXISTS base_files_patient (
			id CHAR(36) PRIMARY KEY,
			patient_id INT DEFAULT NULL,
			patient_code VARCHAR(100) DEFAULT '',
			file_name VARCHAR(255) NOT NULL,
			file_path VARCHAR(500) NOT NULL,
			file_url TEXT,
			storage_path VARCHAR(500) DEFAULT '',
			url_path VARCHAR(255) DEFAULT '',
			access_token VARCHAR(64) DEFAULT '',
			expires_at DATETIME DEFAULT NULL,
			is_public TINYINT(1) DEFAULT 0,
			access_count INT DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			KEY idx_base_files_patient_code (patient_code),
			KEY idx_base_files_patient_id (patient_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE IF NOT EXISTS base_files_report (
			id CHAR(36) PRIMARY KEY,
			file_name VARCHAR(255) NOT NULL,
			file_path VARCHAR(500) NOT NULL,
			file_url TEXT,
			storage_path VARCHAR(500) DEFAULT '',
			url_path VARCHAR(255) DEFAULT '',
			access_token VARCHAR(64) DEFAULT '',
			expires_at DATETIME DEFAULT NULL,
			is_public TINYINT(1) DEFAULT 0,
			access_count INT DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
	}

	for _, sql := range createTablesSQL {
		_, err := db.Exec(sql)
		if err != nil {
			return fmt.Errorf("failed to create table: %v", err)
		}
	}

	if err := ensureColumn(db, dbName, "base_miniapp_sessions", "user_id", "INT DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureColumn(db, dbName, "base_miniapp_sessions", "patient_id", "INT DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureColumn(db, dbName, "base_manage_user", "ai_allowed", "TINYINT(1) DEFAULT 1"); err != nil {
		return err
	}
	userPermissionColumns := map[string]string{
		"user_id":        "INT NOT NULL DEFAULT 0",
		"page_id":        "VARCHAR(100) NOT NULL DEFAULT ''",
		"page_name":      "VARCHAR(100) DEFAULT ''",
		"parent_page_id": "VARCHAR(100) DEFAULT ''",
		"checked":        "TINYINT(1) DEFAULT 1",
	}
	for column, definition := range userPermissionColumns {
		if err := ensureColumn(db, dbName, "base_manage_user_permission", column, definition); err != nil {
			return err
		}
	}
	if err := ensureColumn(db, dbName, "detect_patient", "ai_allowed", "TINYINT(1) DEFAULT 1"); err != nil {
		return err
	}
	reportAnalysisColumns := map[string]string{
		"report_type":      "VARCHAR(50) DEFAULT ''",
		"hospital":         "VARCHAR(255) DEFAULT ''",
		"examination_time": "VARCHAR(100) DEFAULT ''",
		"examination_item": "VARCHAR(255) DEFAULT ''",
		"edited_by":        "INT DEFAULT 0",
		"edited_at":        "DATETIME DEFAULT NULL",
	}
	for column, definition := range reportAnalysisColumns {
		if err := ensureColumn(db, dbName, "patient_report_analysis", column, definition); err != nil {
			return err
		}
	}
	patientColumns := map[string]string{
		"patient_code":                 "VARCHAR(100) UNIQUE",
		"id_document_type":             "VARCHAR(50) DEFAULT '居民身份证'",
		"id_document_no":               "VARCHAR(100) DEFAULT ''",
		"birthday":                     "DATE",
		"address":                      "VARCHAR(255) DEFAULT ''",
		"diagnosis":                    "TEXT",
		"cancer_diameter":              "VARCHAR(100) DEFAULT ''",
		"smoking_status":               "VARCHAR(50) DEFAULT ''",
		"detection_mode":               "VARCHAR(50) DEFAULT ''",
		"sales_person":                 "VARCHAR(100) DEFAULT NULL",
		"patient_source":               "VARCHAR(50) DEFAULT ''",
		"wechat_openid":                "VARCHAR(128) DEFAULT ''",
		"report_subscribe_enabled":     "TINYINT(1) DEFAULT 0",
		"report_subscribe_template_id": "VARCHAR(128) DEFAULT ''",
		"created_by":                   "INT DEFAULT NULL",
		"patient_status":               "TINYINT(1) DEFAULT 1",
		"other_info":                   "TEXT",
		"report_files":                 "TEXT",
		"cancer_pathology":             "TEXT",
		"prognosis_info":               "TEXT",
		"surgery_date":                 "DATE DEFAULT NULL",
		"chemo_start_date":             "DATE DEFAULT NULL",
	}
	for column, definition := range patientColumns {
		if err := ensureColumn(db, dbName, "detect_patient", column, definition); err != nil {
			return err
		}
	}
	if _, err := db.Exec(`UPDATE detect_patient
		SET id_document_type = '居民身份证', id_document_no = id_card
		WHERE COALESCE(NULLIF(TRIM(id_card), ''), '') <> ''
			AND (COALESCE(NULLIF(TRIM(id_document_no), ''), '') = '' OR COALESCE(NULLIF(TRIM(id_document_type), ''), '') = '')`); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE detect_patient
		SET id_document_type = '自编号', id_document_no = CAST(id AS CHAR)
		WHERE COALESCE(NULLIF(TRIM(id_card), ''), '') = ''
			AND (COALESCE(NULLIF(TRIM(id_document_no), ''), '') = '' OR COALESCE(NULLIF(TRIM(id_document_type), ''), '') = '')`); err != nil {
		return err
	}
	if err := ensureColumnType(db, "detect_patient", "sales_person", "VARCHAR(100) DEFAULT NULL"); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE detect_patient
		SET patient_source = CASE
			WHEN COALESCE(NULLIF(TRIM(sales_person), ''), '') <> '' THEN 'sales_invite'
			ELSE 'miniapp_self'
		END
		WHERE COALESCE(NULLIF(TRIM(patient_source), ''), '') = ''`); err != nil {
		return err
	}
	if _, err := db.Exec(`INSERT IGNORE INTO base_manage_user_role (user_id, role_id, created_at, updated_at)
		SELECT id, role_id, NOW(), NOW()
		FROM base_manage_user
		WHERE role_id IS NOT NULL AND role_id > 0`); err != nil {
		return err
	}
	var treatmentStageTableCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = ? AND table_name = 'setting_treatment_stage'`, dbName).Scan(&treatmentStageTableCount); err != nil {
		return err
	}
	if treatmentStageTableCount > 0 {
		for _, name := range []string{"化疗前", "化疗后"} {
			if _, err := db.Exec(`INSERT INTO setting_treatment_stage (name, description, is_active, created_at, updated_at)
				SELECT ?, ?, 1, NOW(), NOW()
				WHERE NOT EXISTS (SELECT 1 FROM setting_treatment_stage WHERE name = ?)`,
				name, name, name); err != nil {
				return err
			}
		}
	}
	sampleColumns := map[string]string{
		"sample_code":                "VARCHAR(100) UNIQUE",
		"sample_type_id":             "INT DEFAULT NULL",
		"cancer_type_id":             "INT DEFAULT NULL",
		"treatment_stage_id":         "INT DEFAULT NULL",
		"collection_date":            "DATETIME DEFAULT NULL",
		"collection_operator":        "INT DEFAULT NULL",
		"receive_date":               "DATETIME DEFAULT NULL",
		"receive_operator":           "INT DEFAULT NULL",
		"test_operator":              "INT DEFAULT NULL",
		"test_completed_at":          "DATETIME DEFAULT NULL",
		"report_type":                "VARCHAR(50) DEFAULT 'normal'",
		"notes":                      "TEXT",
		"organization":               "VARCHAR(255) DEFAULT ''",
		"sample_created_at":          "DATETIME DEFAULT CURRENT_TIMESTAMP",
		"sample_updated_at":          "DATETIME DEFAULT CURRENT_TIMESTAMP",
		"result_data":                "LONGTEXT",
		"result_status":              "VARCHAR(50) DEFAULT ''",
		"signalvalue":                "DOUBLE DEFAULT NULL",
		"batch_id":                   "INT DEFAULT NULL",
		"model_id":                   "INT DEFAULT NULL",
		"service_mode":               "VARCHAR(20) NOT NULL DEFAULT 'single'",
		"sale_package_id":            "INT DEFAULT NULL",
		"inbound_express_signed_at":  "DATETIME DEFAULT NULL",
		"outbound_express_signed_at": "DATETIME DEFAULT NULL",
	}
	for column, definition := range sampleColumns {
		if err := ensureColumn(db, dbName, "detect_sample", column, definition); err != nil {
			return err
		}
	}
	expressColumns := map[string]string{
		"direction":           "VARCHAR(20) NOT NULL DEFAULT 'inbound'",
		"express_type":        "VARCHAR(50) NOT NULL DEFAULT 'auto'",
		"query_mobile":        "VARCHAR(20) DEFAULT NULL",
		"delivered_at":        "DATETIME DEFAULT NULL",
		"provider_status":     "INT DEFAULT NULL",
		"provider_message":    "VARCHAR(255) DEFAULT ''",
		"route_json":          "LONGTEXT",
		"latest_event_time":   "DATETIME DEFAULT NULL",
		"latest_event_status": "VARCHAR(500) DEFAULT ''",
		"last_query_at":       "DATETIME DEFAULT NULL",
		"last_query_error":    "VARCHAR(500) DEFAULT ''",
	}
	for column, definition := range expressColumns {
		if err := ensureColumn(db, dbName, "detect_sample_express", column, definition); err != nil {
			return err
		}
	}
	if _, err := db.Exec(`DELETE older FROM detect_sample_express older
		JOIN detect_sample_express newer
		  ON newer.sample_id = older.sample_id
		 AND newer.direction = older.direction
		 AND newer.id > older.id`); err != nil {
		return err
	}
	if err := ensureCompositeUniqueIndex(db, dbName, "detect_sample_express",
		"uk_sample_express_current", []string{"sample_id", "direction"}); err != nil {
		return err
	}
	panelMatchColumns := map[string]string{
		"matched_panel_ids_json": "LONGTEXT",
		"panel_matches_json":     "LONGTEXT",
		"sample_genes_json":      "LONGTEXT",
	}
	for column, definition := range panelMatchColumns {
		if err := ensureColumn(db, dbName, "detect_sample_panel_match", column, definition); err != nil {
			return err
		}
	}
	salePackageColumns := map[string]string{
		"detection_count": "INT NOT NULL DEFAULT 1",
		"interval_days":   "INT NOT NULL DEFAULT 90",
		"price":           "DECIMAL(10,2) NOT NULL DEFAULT 0",
		"description":     "TEXT",
		"status":          "VARCHAR(20) DEFAULT 'active'",
	}
	for column, definition := range salePackageColumns {
		if err := ensureColumn(db, dbName, "sale_package", column, definition); err != nil {
			return err
		}
	}
	saleOrderColumns := map[string]string{
		"sale_order_no":          "VARCHAR(64) UNIQUE",
		"detect_patient_id":      "INT DEFAULT NULL",
		"detect_patient_id_card": "VARCHAR(18) DEFAULT ''",
		"sale_package_id":        "INT DEFAULT NULL",
		"setting_cancer_type_id": "INT DEFAULT NULL",
		"first_detection_date":   "DATE DEFAULT NULL",
		"payment_method":         "VARCHAR(50) DEFAULT ''",
		"payment_status":         "VARCHAR(30) DEFAULT 'pending'",
		"sales_person_id":        "INT DEFAULT NULL",
		"total_amount":           "DECIMAL(10,2) DEFAULT 0",
		"status":                 "VARCHAR(30) DEFAULT 'pending'",
	}
	for column, definition := range saleOrderColumns {
		if err := ensureColumn(db, dbName, "sale_order", column, definition); err != nil {
			return err
		}
	}
	saleDetectionPlanColumns := map[string]string{
		"sale_order_id":     "INT DEFAULT NULL",
		"detect_patient_id": "INT DEFAULT NULL",
		"detection_date":    "DATE DEFAULT NULL",
		"detection_number":  "INT NOT NULL DEFAULT 1",
		"status":            "VARCHAR(30) DEFAULT 'scheduled'",
	}
	for column, definition := range saleDetectionPlanColumns {
		if err := ensureColumn(db, dbName, "sale_detection_plan", column, definition); err != nil {
			return err
		}
	}
	reportColumns := map[string]string{
		"rejected_reason":    "TEXT",
		"parent_report_id":   "INT DEFAULT NULL",
		"report_role":        "VARCHAR(20) DEFAULT 'single'",
		"patient_viewed_at":  "DATETIME DEFAULT NULL",
		"patient_view_count": "INT NOT NULL DEFAULT 0",
	}
	for column, definition := range reportColumns {
		if err := ensureColumn(db, dbName, "detect_report", column, definition); err != nil {
			return err
		}
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS detect_report_change_log (
		id INT AUTO_INCREMENT PRIMARY KEY,
		report_id INT NOT NULL,
		field_name VARCHAR(100) NOT NULL,
		old_value TEXT,
		new_value TEXT,
		changed_by INT DEFAULT NULL,
		changed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_report_change_log_report_id (report_id),
		INDEX idx_report_change_log_changed_at (changed_at)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`); err != nil {
		return err
	}
	mailAddressColumns := map[string]string{
		"sale_order_id":            "INT DEFAULT NULL",
		"detection_plan_id":        "INT DEFAULT NULL",
		"province":                 "VARCHAR(50) DEFAULT ''",
		"city":                     "VARCHAR(50) DEFAULT ''",
		"district":                 "VARCHAR(50) DEFAULT ''",
		"detail_address":           "VARCHAR(255) DEFAULT ''",
		"full_address":             "VARCHAR(500) DEFAULT ''",
		"express_company":          "VARCHAR(100) DEFAULT ''",
		"tracking_number":          "VARCHAR(100) DEFAULT ''",
		"express_status":           "VARCHAR(30) DEFAULT 'pending'",
		"express_route_json":       "LONGTEXT",
		"express_delivered_at":     "DATETIME DEFAULT NULL",
		"express_last_query_at":    "DATETIME DEFAULT NULL",
		"express_last_query_error": "VARCHAR(500) DEFAULT ''",
		"status":                   "VARCHAR(30) DEFAULT 'requested'",
		"notes":                    "TEXT",
		"shipped_at":               "DATETIME DEFAULT NULL",
	}
	for column, definition := range mailAddressColumns {
		if err := ensureColumn(db, dbName, "mail_address", column, definition); err != nil {
			return err
		}
	}
	if err := ensureSingleColumnUniqueIndex(db, dbName, "detect_batch", "id", "uk_detect_batch_id"); err != nil {
		return err
	}
	filePatientColumns := map[string]string{
		"patient_id":   "INT DEFAULT NULL",
		"patient_code": "VARCHAR(100) DEFAULT ''",
		"file_name":    "VARCHAR(255) NOT NULL DEFAULT ''",
		"file_path":    "VARCHAR(500) NOT NULL DEFAULT ''",
		"file_url":     "TEXT",
		"storage_path": "VARCHAR(500) DEFAULT ''",
		"url_path":     "VARCHAR(255) DEFAULT ''",
		"access_token": "VARCHAR(64) DEFAULT ''",
		"expires_at":   "DATETIME DEFAULT NULL",
		"is_public":    "TINYINT(1) DEFAULT 0",
		"access_count": "INT DEFAULT 0",
	}
	for column, definition := range filePatientColumns {
		if err := ensureColumn(db, dbName, "base_files_patient", column, definition); err != nil {
			return err
		}
	}
	fileReportColumns := map[string]string{
		"file_name":    "VARCHAR(255) NOT NULL DEFAULT ''",
		"file_path":    "VARCHAR(500) NOT NULL DEFAULT ''",
		"file_url":     "TEXT",
		"storage_path": "VARCHAR(500) DEFAULT ''",
		"url_path":     "VARCHAR(255) DEFAULT ''",
		"access_token": "VARCHAR(64) DEFAULT ''",
		"expires_at":   "DATETIME DEFAULT NULL",
		"is_public":    "TINYINT(1) DEFAULT 0",
		"access_count": "INT DEFAULT 0",
	}
	for column, definition := range fileReportColumns {
		if err := ensureColumn(db, dbName, "base_files_report", column, definition); err != nil {
			return err
		}
	}

	log.Println("数据库表结构检查完成")
	return nil
}

func ensureColumn(db *sql.DB, dbName, tableName, columnName, definition string) error {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND COLUMN_NAME = ?`, dbName, tableName, columnName).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to inspect column %s.%s: %v", tableName, columnName, err)
	}
	if count > 0 {
		return nil
	}
	if _, err := db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", tableName, columnName, definition)); err != nil {
		return fmt.Errorf("failed to add column %s.%s: %v", tableName, columnName, err)
	}
	return nil
}

func ensureColumnType(db *sql.DB, tableName, columnName, definition string) error {
	if _, err := db.Exec(fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s %s", tableName, columnName, definition)); err != nil {
		return fmt.Errorf("failed to modify column %s.%s: %v", tableName, columnName, err)
	}
	return nil
}

func ensureSingleColumnUniqueIndex(db *sql.DB, dbName, tableName, columnName, indexName string) error {
	var tableCount int
	err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?`, dbName, tableName).Scan(&tableCount)
	if err != nil {
		return fmt.Errorf("failed to inspect table %s: %v", tableName, err)
	}
	if tableCount == 0 {
		return nil
	}

	var indexCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM information_schema.STATISTICS s
		WHERE s.TABLE_SCHEMA = ? AND s.TABLE_NAME = ? AND s.NON_UNIQUE = 0
			AND s.COLUMN_NAME = ? AND s.SEQ_IN_INDEX = 1
			AND NOT EXISTS (
				SELECT 1 FROM information_schema.STATISTICS s2
				WHERE s2.TABLE_SCHEMA = s.TABLE_SCHEMA
					AND s2.TABLE_NAME = s.TABLE_NAME
					AND s2.INDEX_NAME = s.INDEX_NAME
					AND s2.SEQ_IN_INDEX > 1
			)`, dbName, tableName, columnName).Scan(&indexCount)
	if err != nil {
		return fmt.Errorf("failed to inspect unique index %s.%s: %v", tableName, columnName, err)
	}
	if indexCount > 0 {
		return nil
	}

	if _, err := db.Exec(fmt.Sprintf("ALTER TABLE %s ADD UNIQUE KEY %s (%s)", tableName, indexName, columnName)); err != nil {
		return fmt.Errorf("failed to add unique index %s on %s.%s: %v", indexName, tableName, columnName, err)
	}
	return nil
}

func ensureCompositeUniqueIndex(db *sql.DB, dbName, tableName, indexName string, columns []string) error {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND INDEX_NAME = ? AND NON_UNIQUE = 0`,
		dbName, tableName, indexName).Scan(&count); err != nil {
		return fmt.Errorf("failed to inspect unique index %s.%s: %v", tableName, indexName, err)
	}
	if count == len(columns) {
		return nil
	}
	if count > 0 {
		if _, err := db.Exec(fmt.Sprintf("ALTER TABLE %s DROP INDEX %s", tableName, indexName)); err != nil {
			return fmt.Errorf("failed to replace unique index %s.%s: %v", tableName, indexName, err)
		}
	}
	if _, err := db.Exec(fmt.Sprintf("ALTER TABLE %s ADD UNIQUE KEY %s (%s)",
		tableName, indexName, strings.Join(columns, ", "))); err != nil {
		return fmt.Errorf("failed to add unique index %s on %s: %v", indexName, tableName, err)
	}
	return nil
}
