package handlers

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/des"
	"crypto/md5"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

const reportSubscribeTemplateID = "etRGY-LJcMas11zwBIpayTEF1THGdUG_sAoNb2XQoro"

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

// RSA密钥接口
type RSAKeysInterface interface {
	DecryptRSA(encryptedData []byte) ([]byte, error)
}

// 认证相关请求结构
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

type UpdateUserRequest struct {
	RealName     string `json:"real_name" binding:"required"`
	Phone        string `json:"phone" binding:"required"`
	Email        string `json:"email"`
	EmployeeId   string `json:"employee_id"`
	RoleId       int    `json:"role_id"`
	DepartmentId int    `json:"department_id"`
	Status       int    `json:"status"`
}

// 从上下文获取用户ID的键
const UserIDKey = "userID"

// 全局RSA密钥对
var RSAKeyPair RSAKeysInterface

func createPhoneBindToken(db *sql.DB, userID int, client string) (string, error) {
	token := fmt.Sprintf("bind_%d_%s", time.Now().UnixNano(), generateRandomString(32))
	_, err := db.Exec(`INSERT INTO base_phone_bind_tokens (token, user_id, client, used, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, 0, ?, NOW(), NOW())`, token, userID, client, time.Now().Add(10*time.Minute))
	if err != nil {
		return "", err
	}
	return token, nil
}

func getPhoneBindTokenUser(db *sql.DB, token, client string) (int, error) {
	var userID int
	var expiresAt time.Time
	var used int
	err := db.QueryRow(`SELECT user_id, expires_at, used FROM base_phone_bind_tokens
		WHERE token = ? AND client = ? LIMIT 1`, token, client).Scan(&userID, &expiresAt, &used)
	if err != nil {
		return 0, err
	}
	if used == 1 {
		return 0, fmt.Errorf("绑定令牌已使用")
	}
	if time.Now().After(expiresAt) {
		return 0, fmt.Errorf("绑定令牌已过期")
	}
	return userID, nil
}

func markPhoneBindTokenUsed(db *sql.DB, token string) {
	_, _ = db.Exec("UPDATE base_phone_bind_tokens SET used = 1, updated_at = NOW() WHERE token = ?", token)
}

// SetRSAKeys 设置RSA密钥对
func SetRSAKeys(keys RSAKeysInterface) {
	RSAKeyPair = keys
}

// 认证相关处理函数
func HandleLogin(c *app.RequestContext, db *sql.DB) {
	// 检查数据库连接是否可用
	if db == nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "数据库连接失败，请联系管理员",
			Data:    nil,
		})
		return
	}

	// 解析请求体
	var req struct {
		Username       string `json:"username" binding:"required"`
		Password       string `json:"password" binding:"required"`
		Timestamp      string `json:"timestamp" binding:"required"`
		AutoLogin      bool   `json:"autoLogin"`
		IsRsaEncrypted bool   `json:"isRsaEncrypted"` // 是否使用RSA加密
	}

	// 手动读取和解析请求体，以便查看详细错误
	body, err := c.Body()
	if err != nil {
		log.Printf("Failed to read request body: %v", err)
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "读取请求体失败",
			Data:    nil,
		})
		return
	}

	if err := json.Unmarshal(body, &req); err != nil {
		log.Printf("Failed to unmarshal request body: %v", err)
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "解析请求体失败 " + err.Error(),
			Data:    nil,
		})
		return
	}

	// 验证必要字段
	if req.Username == "" || req.Password == "" || req.Timestamp == "" {
		log.Printf("Missing required fields for login")
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "用户名、密码和时间戳不能为空",
			Data:    nil,
		})
		return
	}

	// 检查是否使用RSA加密
	if !req.IsRsaEncrypted || RSAKeyPair == nil {
		log.Printf("Login attempt without RSA encryption for user %s", req.Username)
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请使用RSA加密方式登录",
			Data:    nil,
		})
		return
	}

	// 解密RSA加密的密码
	encryptedBytes, err := base64.StdEncoding.DecodeString(req.Password)
	if err != nil {
		log.Printf("RSA decode error: %v", err)
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "密码格式错误",
			Data:    nil,
		})
		return
	}

	decryptedBytes, err := RSAKeyPair.DecryptRSA(encryptedBytes)
	if err != nil {
		log.Printf("RSA decrypt error: %v", err)
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "密码解密失败",
			Data:    nil,
		})
		return
	}

	req.Password = string(decryptedBytes)
	log.Printf("RSA decryption successful for user %s", req.Username)

	// 从数据库查询用户
	var userID int
	var username string
	var realName string
	var phone string
	var password string
	var salt string
	var roleID sql.NullInt32
	var status int
	var lastLoginTime sql.NullTime

	query := "SELECT id, username, real_name, phone, password, salt, role_id, status, last_login_time FROM base_manage_user WHERE username = ?"
	log.Printf("Executing query: %s with username: %s", query, req.Username)
	err = db.QueryRow(query, req.Username).Scan(&userID, &username, &realName, &phone, &password, &salt, &roleID, &status, &lastLoginTime)
	if err != nil {
		log.Printf("Query error: %v", err)
		if err == sql.ErrNoRows {
			c.JSON(consts.StatusUnauthorized, ApiResponse{
				Code:    401,
				Success: false,
				Message: "用户名或密码错误",
				Data:    nil,
			})
			return
		}
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 将sql.NullInt32转换为int
	roleIDInt := 0
	if roleID.Valid {
		roleIDInt = int(roleID.Int32)
	}

	log.Printf("Query succeeded, found user ID: %d, role ID: %d, status: %d", userID, roleIDInt, status)

	// 验证用户状态
	if status == 0 {
		c.JSON(401, ApiResponse{
			Code:    401,
			Success: false,
			Message: "账号已被禁用",
			Data:    nil,
		})
		return
	}

	// 验证时间戳
	log.Printf("Verifying timestamp for user %d", userID)
	// 解析前端传递的时间戳
	timestampInt, err := strconv.ParseInt(req.Timestamp, 10, 64)
	if err != nil {
		log.Printf("Timestamp parsing failed: %v", err)
		c.JSON(consts.StatusUnauthorized, ApiResponse{
			Code:    401,
			Success: false,
			Message: "非法请求：无效的时间戳",
			Data:    nil,
		})
		return
	}

	// 计算时间戳与服务器时间的差距（秒）
	timestampTime := time.Unix(timestampInt/1000, 0) // 前端传递的是毫秒时间戳，转换为秒
	serverTime := time.Now()
	timeDiff := int(math.Abs(float64(serverTime.Unix() - timestampTime.Unix())))

	// 检查时间差是否超过1分钟（60秒）
	if timeDiff > 60 {
		log.Printf("Timestamp verification failed: time difference %d seconds exceeds 1 minute", timeDiff)
		c.JSON(consts.StatusUnauthorized, ApiResponse{
			Code:    401,
			Success: false,
			Message: "非法请求，请检查系统时间是否与北京时间同步",
			Data:    nil,
		})
		return
	}

	// 验证密码
	log.Printf("Verifying password for user %d", userID)

	// RSA加密路径：验证解密后的密码
	// 数据库中存储的是MD5哈希值，需要计算原始密码的MD5进行比较
	decryptedPassword := req.Password
	passwordValid := matchManageUserPassword(password, decryptedPassword, salt)

	log.Printf("Password validation result: %t", passwordValid)

	if !passwordValid {
		log.Printf("Password verification failed for user %d", userID)
		c.JSON(consts.StatusUnauthorized, ApiResponse{
			Code:    401,
			Success: false,
			Message: "用户名或密码错误",
			Data:    nil,
		})
		return
	}

	// 验证通过后，记录加密密码用于安全审计
	log.Printf("Password verification succeeded for user %d", userID)

	if strings.TrimSpace(phone) == "" {
		bindToken, err := createPhoneBindToken(db, userID, "admin")
		if err != nil {
			log.Printf("Failed to create admin phone bind token: %v", err)
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code:    500,
				Success: false,
				Message: "服务器内部错误",
				Data:    nil,
			})
			return
		}
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "首次密码登录需要绑定手机号",
			Data: utils.H{
				"need_bind_phone": true,
				"bind_token":      bindToken,
				"user_id":         userID,
			},
		})
		return
	}

	// 更新最后登录时间
	updateQuery := "UPDATE base_manage_user SET last_login_time = ? WHERE id = ?"
	_, err = db.Exec(updateQuery, time.Now(), userID)
	if err != nil {
		// 仅记录日志，不影响登录流程
		log.Printf("Failed to update last_login_time: %v", err)
	}

	// 生成session ID
	sessionID := fmt.Sprintf("%d_%d_%s", userID, time.Now().UnixNano(), generateRandomString(32))

	// 设置session过期时间
	var expiresIn time.Duration
	var cookieMaxAge int

	if req.AutoLogin {
		expiresIn = 7 * 24 * time.Hour // 7天
		cookieMaxAge = int(expiresIn.Seconds())
	} else {
		// 如果不是自动登录，数据库Session保存24小时（防止意外关闭等），但Cookie设置为会话级
		expiresIn = 24 * time.Hour
		cookieMaxAge = 0 // 0表示会话Cookie，关闭浏览器时清除
		// 注意：Go的SetCookie如果maxAge < 0会删除cookie。如果 == 0，有些浏览器行为是会话，有些可能会误解。
		// 在Go中，0意味着没有Max-Age属性，通常被视为会话cookie。
		// 但为了保险，我们明确设置为会话行为。
	}
	expiry := time.Now().Add(expiresIn)

	// 删除用户原来的所有base_sessions，使它们失效
	_, err = db.Exec("DELETE FROM base_sessions WHERE user_id = ?", userID)
	if err != nil {
		log.Printf("Failed to delete old base_sessions: %v", err)
		// 继续执行，不中断登录流程
	}

	// 存储session到数据库
	_, err = db.Exec(
		"INSERT INTO base_sessions (session_id, user_id, expiry, created_at, updated_at) VALUES (?, ?, ?, NOW(), NOW())",
		sessionID, userID, expiry,
	)
	if err != nil {
		log.Printf("Failed to store session: %v", err)
		c.JSON(500, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	// 设置cookie
	// path, domain, sameSite, secure, httpOnly
	// secure: false (for dev/http), httpOnly: true
	c.SetCookie(
		"session_id",
		sessionID,
		cookieMaxAge,
		"/",
		"",
		protocol.CookieSameSiteLaxMode,
		false,
		true,
	)

	// 查询角色信息
	var roleName string
	var roleDescription string

	roleQuery := "SELECT name, description FROM setting_role WHERE id = ?"
	log.Printf("Executing role query: %s with role ID: %d", roleQuery, roleIDInt)
	roleErr := db.QueryRow(roleQuery, roleIDInt).Scan(&roleName, &roleDescription)

	role := utils.H{}
	if roleErr == nil {
		role = utils.H{
			"id":          roleIDInt,
			"name":        roleName,
			"description": roleDescription,
		}
	} else {
		log.Printf("Role query error: %v", roleErr)
	}
	roles, roleIDs, roleNames := getUserRoles(db, userID, roleIDInt)
	if len(roles) > 0 {
		role = roles[0]
		roleIDInt = roleIDs[0]
		roleName = roleNames[0]
	}

	// 构建用户信息
	user := utils.H{
		"id":        userID,
		"username":  username,
		"real_name": realName,
		"phone":     phone,
		"role_id":   roleIDInt,
		"role":      role,
		"status":    status,
	}

	// 只在 lastLoginTime 有效时添加到用户信息中
	if lastLoginTime.Valid {
		user["last_login_time"] = lastLoginTime.Time.Format("2006-01-02T15:04:05+08:00")
	} else {
		user["last_login_time"] = nil
	}

	// 返回用户信息
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "登录成功",
		Data: utils.H{
			"user": user,
		},
	})
}

// 生成随机字符串
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func miniappNeedPatientRegister(c *app.RequestContext, phone string) {
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "首次登录需要填写患者基本信息",
		Data: utils.H{
			"need_register": true,
			"phone":         phone,
			"message":       "首次登录需要填写患者基本信息",
		},
	})
}

func buildPatientRegisterInfo(idCard string) (string, interface{}) {
	if len(idCard) != 18 {
		return "", nil
	}

	birthdayPart := idCard[6:14]
	birthday, err := time.Parse("20060102", birthdayPart)
	if err != nil {
		return "", nil
	}

	gender := ""
	if seq, err := strconv.Atoi(idCard[16:17]); err == nil {
		if seq%2 == 0 {
			gender = "女"
		} else {
			gender = "男"
		}
	}

	return gender, birthday.Format("2006-01-02")
}

func HandleMiniappCheckIdCard(c *app.RequestContext, db *sql.DB) {
	idCard := strings.TrimSpace(c.Query("id_card"))
	documentType := normalizePatientDocumentType(c.Query("id_document_type"))
	documentNo := strings.TrimSpace(c.Query("id_document_no"))
	if idCard == "" {
		var req struct {
			IDCard         string `json:"id_card"`
			IDDocumentType string `json:"id_document_type"`
			IDDocumentNo   string `json:"id_document_no"`
		}
		body, _ := c.Body()
		_ = json.Unmarshal(body, &req)
		idCard = strings.TrimSpace(req.IDCard)
		if strings.TrimSpace(req.IDDocumentType) != "" {
			documentType = normalizePatientDocumentType(req.IDDocumentType)
		}
		if strings.TrimSpace(req.IDDocumentNo) != "" {
			documentNo = strings.TrimSpace(req.IDDocumentNo)
		}
	}
	if documentNo == "" {
		documentNo = idCard
	}
	documentNo = normalizePatientDocumentNo(documentNo)
	if documentNo == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "身份证件号不能为空", Data: nil})
		return
	}
	if isResidentIDCard(documentType) && len(documentNo) != 18 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "居民身份证号格式不正确",
			Data:    nil,
		})
		return
	}

	gender, birthday := "", ""
	if isResidentIDCard(documentType) {
		gender, birthday, _ = parseResidentIDCardInfo(documentNo)
	}
	var patientID int
	var patientName, patientPhone string
	err := db.QueryRow(`SELECT id, COALESCE(name, ''), COALESCE(phone, '')
		FROM detect_patient
		WHERE `+patientDocumentWhereClause("?")+` AND is_active = 1
		ORDER BY id DESC LIMIT 1`, documentNo, documentNo).Scan(&patientID, &patientName, &patientPhone)
	exists := err == nil
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Miniapp check id card error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "身份证件号校验失败",
			Data:    nil,
		})
		return
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "身份证件号校验成功",
		Data: utils.H{
			"exists":           exists,
			"bindable":         exists && strings.TrimSpace(patientPhone) == "",
			"gender":           gender,
			"birthday":         birthday,
			"id_document_type": documentType,
			"id_document_no":   documentNo,
			"patient": utils.H{
				"id":    patientID,
				"name":  patientName,
				"phone": patientPhone,
			},
		},
	})
}

func createMiniappPatientSession(c *app.RequestContext, db *sql.DB, phone string, patientID int, patientName string, patientCode string) {
	expiresIn := 24 * time.Hour
	sessionID := fmt.Sprintf("p_%d_%s", time.Now().UnixNano(), generateRandomString(32))
	expiry := time.Now().Add(expiresIn)
	var patientSource, salesPerson string
	_ = db.QueryRow(`SELECT COALESCE(patient_source, ''), COALESCE(sales_person, '') FROM detect_patient WHERE id = ? LIMIT 1`, patientID).
		Scan(&patientSource, &salesPerson)

	_, err := db.Exec(
		"INSERT INTO base_miniapp_sessions (session_id, user_id, phone, identity_type, patient_id, expiry, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, NOW(), NOW())",
		sessionID, 0, phone, "patient", patientID, expiry,
	)
	if err != nil {
		log.Printf("Failed to create miniapp patient session: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	c.SetCookie(
		"miniapp_session_id",
		sessionID,
		int(expiresIn.Seconds()),
		"/",
		"",
		protocol.CookieSameSiteLaxMode,
		false,
		true,
	)

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "建档并登录成功",
		Data: utils.H{
			"session_id":  sessionID,
			"need_select": false,
			"user_info": utils.H{
				"patient_id":      patientID,
				"name":            patientName,
				"patient_code":    patientCode,
				"phone":           phone,
				"patient_source":  patientSource,
				"patientSource":   patientSource,
				"sales_person":    salesPerson,
				"salesPersonCode": salesPerson,
			},
		},
	})
}

func HandleMiniappRegisterPatient(c *app.RequestContext, db *sql.DB) {
	if db == nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "数据库连接失败，请联系管理员",
			Data:    nil,
		})
		return
	}

	var req struct {
		Name                    string `json:"name"`
		IdCard                  string `json:"id_card"`
		Phone                   string `json:"phone"`
		JsCode                  string `json:"jsCode"`
		ReportSubscribeAccepted bool   `json:"report_subscribe_accepted"`
	}

	body, err := c.Body()
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "读取请求体失败",
			Data:    nil,
		})
		return
	}

	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "解析请求体失败",
			Data:    nil,
		})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.IdCard = strings.TrimSpace(req.IdCard)
	req.Phone = strings.TrimSpace(req.Phone)

	if req.Name == "" || req.IdCard == "" || req.Phone == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "姓名、身份证件号和电话不能为空",
			Data:    nil,
		})
		return
	}
	if len(req.IdCard) != 18 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "身份证号格式不正确",
			Data:    nil,
		})
		return
	}

	var employeeCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM base_manage_user WHERE phone = ?", req.Phone).Scan(&employeeCount); err != nil {
		log.Printf("Check employee phone error: %v", err)
	}
	if employeeCount > 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "该手机号已绑定员工账号，请直接登录",
			Data:    nil,
		})
		return
	}

	var patientCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM detect_patient WHERE phone = ? AND is_active = 1", req.Phone).Scan(&patientCount); err != nil {
		log.Printf("Check patient phone error: %v", err)
	}
	if patientCount > 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "该手机号已存在患者信息，请重新登录",
			Data:    nil,
		})
		return
	}

	gender, birthday := buildPatientRegisterInfo(req.IdCard)
	var existingPatient struct {
		ID          int
		PatientCode string
		Name        string
		Phone       string
	}
	err = db.QueryRow(`SELECT id, patient_code, COALESCE(name, ''), COALESCE(phone, '')
		FROM detect_patient WHERE id_card = ? AND is_active = 1 LIMIT 1`, req.IdCard).
		Scan(&existingPatient.ID, &existingPatient.PatientCode, &existingPatient.Name, &existingPatient.Phone)
	if err == nil {
		if strings.TrimSpace(existingPatient.Phone) != "" {
			c.JSON(consts.StatusBadRequest, ApiResponse{
				Code:    400,
				Success: false,
				Message: "该身份证件号已存在患者信息",
				Data:    nil,
			})
			return
		}
		openID, openIDErr := getWechatOpenID(req.JsCode)
		if openIDErr != nil {
			log.Printf("Get register patient openid error: %v", openIDErr)
		}
		subscribeTemplateID := ""
		if req.ReportSubscribeAccepted {
			subscribeTemplateID = reportSubscribeTemplateID
		}
		_, err = db.Exec(`UPDATE detect_patient SET phone = ?, name = COALESCE(NULLIF(name, ''), ?),
			id_document_type = '居民身份证', id_document_no = ?, gender = ?, birthday = ?, wechat_openid = COALESCE(NULLIF(wechat_openid, ''), ?),
			report_subscribe_enabled = ?, report_subscribe_template_id = ?, updated_at = NOW()
			WHERE id = ?`,
			req.Phone, req.Name, req.IdCard, gender, birthday, openID, boolToInt(req.ReportSubscribeAccepted), subscribeTemplateID, existingPatient.ID)
		if err != nil {
			log.Printf("Bind existing miniapp patient error: %v", err)
			c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "绑定患者失败", Data: nil})
			return
		}
		name := existingPatient.Name
		if strings.TrimSpace(name) == "" {
			name = req.Name
		}
		createMiniappPatientSession(c, db, req.Phone, existingPatient.ID, name, existingPatient.PatientCode)
		return
	}
	if err != sql.ErrNoRows {
		log.Printf("Check patient id card error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "身份证件号校验失败", Data: nil})
		return
	}

	patientCode := generatePatientCode(db)
	openID, openIDErr := getWechatOpenID(req.JsCode)
	if openIDErr != nil {
		log.Printf("Get register patient openid error: %v", openIDErr)
	}
	subscribeTemplateID := ""
	if req.ReportSubscribeAccepted {
		subscribeTemplateID = reportSubscribeTemplateID
	}

	result, err := db.Exec(`INSERT INTO detect_patient
		(patient_code, name, gender, id_document_type, id_document_no, id_card, phone, birthday, patient_source, wechat_openid, report_subscribe_enabled, report_subscribe_template_id, is_active, completion_status, patient_status, created_at, updated_at)
		VALUES (?, ?, ?, '居民身份证', ?, ?, ?, ?, 'miniapp_self', ?, ?, ?, 1, 1, 1, NOW(), NOW())`,
		patientCode, req.Name, gender, req.IdCard, req.IdCard, req.Phone, birthday, openID, boolToInt(req.ReportSubscribeAccepted), subscribeTemplateID,
	)
	if err != nil {
		log.Printf("Create miniapp patient error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "患者建档失败",
			Data:    nil,
		})
		return
	}

	patientID, err := result.LastInsertId()
	if err != nil {
		log.Printf("Get miniapp patient id error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "患者建档失败",
			Data:    nil,
		})
		return
	}

	createMiniappPatientSession(c, db, req.Phone, int(patientID), req.Name, patientCode)
}

// 使用MD5加密并返回字节数组
func md5HashBytes(text string) []byte {
	hasher := md5.New()
	hasher.Write([]byte(text))
	return hasher.Sum(nil)
}

// 使用MD5加密并返回十六进制字符串
func md5Hash(text string) string {
	return hex.EncodeToString(md5HashBytes(text))
}

func hashManageUserPassword(rawPassword, salt string) string {
	return md5Hash(md5Hash(rawPassword) + salt)
}

func matchManageUserPassword(storedHash, rawPassword, salt string) bool {
	return storedHash == hashManageUserPassword(rawPassword, salt) || storedHash == md5Hash(rawPassword+salt)
}

// 使用SHA-256加密并返回十六进制字符串
func sha256Hash(text string) string {
	hasher := sha256.New()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))
}

// DES解密函数
func desDecrypt(ciphertext []byte, key []byte) ([]byte, error) {
	block, err := des.NewCipher(key)
	if err != nil {
		return nil, err
	}
	iv := make([]byte, des.BlockSize)
	stream := cipher.NewCBCDecrypter(block, iv)
	stream.CryptBlocks(ciphertext, ciphertext)
	// 移除PKCS#7填充
	length := len(ciphertext)
	if length == 0 {
		return ciphertext, nil
	}
	unpadding := int(ciphertext[length-1])
	// 验证填充值是否有效
	if unpadding <= 0 || unpadding > length {
		return ciphertext, nil
	}
	return ciphertext[:length-unpadding], nil
}

// 解密登录密码
func decryptPassword(encryptedPassword string, timestamp string) (string, error) {
	// 由于前端使用的是简化的加密逻辑，我们不需要解密
	// 我们需要在HandleLogin函数中直接验证
	// 这里我们直接返回encryptedPassword，让HandleLogin函数处理验证逻辑
	return encryptedPassword, nil
}

func HandleLogout(c *app.RequestContext, db *sql.DB) {
	sessionID := c.Cookie("session_id")
	if string(sessionID) != "" {
		_, err := db.Exec("DELETE FROM base_sessions WHERE session_id = ?", sessionID)
		if err != nil {
			log.Printf("Failed to delete session: %v", err)
		}
	}

	c.SetCookie(
		"session_id",
		"",
		-1,
		"/",
		"",
		protocol.CookieSameSiteLaxMode,
		false,
		true,
	)

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "退出登录成功",
		Data:    nil,
	})
}

func HandleAdminSwitchUser(c *app.RequestContext, db *sql.DB) {
	currentUserID, ok := GetUserID(c)
	if !ok {
		c.JSON(consts.StatusUnauthorized, ApiResponse{Code: 401, Success: false, Message: "未认证", Data: nil})
		return
	}
	var req struct {
		UserID int `json:"user_id"`
	}
	if err := c.Bind(&req); err != nil || req.UserID <= 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "请选择要切换的账号", Data: nil})
		return
	}

	var currentPhone string
	if err := db.QueryRow("SELECT phone FROM base_manage_user WHERE id = ? AND status = 1", currentUserID).Scan(&currentPhone); err != nil {
		c.JSON(consts.StatusUnauthorized, ApiResponse{Code: 401, Success: false, Message: "当前账号不可用", Data: nil})
		return
	}
	var targetCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM base_manage_user WHERE id = ? AND phone = ? AND status = 1", req.UserID, currentPhone).Scan(&targetCount); err != nil || targetCount == 0 {
		c.JSON(consts.StatusForbidden, ApiResponse{Code: 403, Success: false, Message: "只能切换同手机号账号", Data: nil})
		return
	}

	sessionID := fmt.Sprintf("%d_%d_%s", req.UserID, time.Now().UnixNano(), generateRandomString(32))
	expiry := time.Now().Add(24 * time.Hour)
	oldSessionID := c.Cookie("session_id")
	if string(oldSessionID) != "" {
		_, _ = db.Exec("DELETE FROM base_sessions WHERE session_id = ?", oldSessionID)
	}
	if _, err := db.Exec("INSERT INTO base_sessions (session_id, user_id, expiry, created_at, updated_at) VALUES (?, ?, ?, NOW(), NOW())", sessionID, req.UserID, expiry); err != nil {
		log.Printf("Admin switch user session error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "切换失败", Data: nil})
		return
	}
	c.SetCookie("session_id", sessionID, 86400, "/", "", protocol.CookieSameSiteLaxMode, false, true)
	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "切换成功", Data: utils.H{"session_id": sessionID}})
}

func HandleGetMe(c *app.RequestContext, db *sql.DB) {
	// 从上下文获取用户ID
	userID, exists := c.Get(UserIDKey)
	if !exists {
		c.JSON(consts.StatusUnauthorized, ApiResponse{
			Code:    401,
			Success: false,
			Message: "未认证",
			Data:    nil,
		})
		return
	}

	// 从数据库查询用户信息
	var id int
	var username string
	var realName string
	var phone string
	var employeeID string
	var roleID sql.NullInt32
	var status int
	var lastLoginTime sql.NullTime
	var departmentID sql.NullInt32

	// 查询用户基本信息，包括部门ID
	query := `SELECT u.id, u.username, u.real_name, u.phone, COALESCE(u.employee_id, ''), u.role_id, u.status, u.last_login_time, u.department_id
		 FROM base_manage_user u WHERE u.id = ?`
	log.Printf("Executing get user info query: %s with user ID: %v", query, userID)
	err := db.QueryRow(query, userID).Scan(&id, &username, &realName, &phone, &employeeID, &roleID, &status, &lastLoginTime, &departmentID)
	if err != nil {
		log.Printf("Get user info query error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 构建部门信息
	department := ""
	if departmentID.Valid {
		// 通过部门ID获取部门名称
		var deptName string
		deptErr := db.QueryRow("SELECT name FROM setting_department WHERE id = ?", departmentID.Int32).Scan(&deptName)
		if deptErr == nil {
			department = deptName
		}
	}
	log.Printf("Get user info succeeded: id=%d, username=%s, realName=%s, phone=%s, roleID=%v, status=%d, department=%s",
		id, username, realName, phone, roleID, status, department)

	// 查询角色信息
	var roleName string
	var roleDescription string

	// 将sql.NullInt32转换为int
	roleIDInt := 0
	if roleID.Valid {
		roleIDInt = int(roleID.Int32)
	}

	roleQuery := "SELECT name, description FROM setting_role WHERE id = ?"
	log.Printf("Executing role query: %s with role ID: %d", roleQuery, roleIDInt)
	roleErr := db.QueryRow(roleQuery, roleIDInt).Scan(&roleName, &roleDescription)

	role := utils.H{}
	if roleErr == nil {
		role = utils.H{
			"id":          roleIDInt,
			"name":        roleName,
			"description": roleDescription,
		}
	} else {
		log.Printf("Role query error: %v", roleErr)
	}
	roles, roleIDs, roleNames := getUserRoles(db, id, roleIDInt)
	if len(roles) > 0 {
		role = roles[0]
		roleIDInt = roleIDs[0]
		roleName = roleNames[0]
	}

	// 获取用户可访问的页面权限
	var permissions []utils.H
	var userPermissionCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM base_manage_user_permission WHERE user_id = ?", id).Scan(&userPermissionCount)
	if userPermissionCount > 0 {
		rows, err := db.Query(`SELECT page_id, page_name, parent_page_id, checked
			FROM base_manage_user_permission WHERE user_id = ? ORDER BY page_id ASC`, id)
		if err == nil {
			defer rows.Close()
			if tree, scanErr := scanPermissionTree(rows); scanErr == nil {
				permissions = tree
			} else {
				log.Printf("Failed to scan user permission override: %v", scanErr)
			}
		}
	} else if roleIDInt > 0 {
		tree, err := queryRolePermissionsForUser(db, id, roleIDInt)
		if err != nil {
			log.Printf("Failed to query merged role permissions: %v", err)
		}
		permissions = tree
	}

	// 构建响应数据
	switchIdentities := []utils.H{}
	switchRows, switchErr := db.Query(`SELECT u.id, u.username, u.real_name, COALESCE(u.employee_id, ''), COALESCE(r.name, '')
		FROM base_manage_user u
		LEFT JOIN setting_role r ON u.role_id = r.id
		WHERE u.phone = ? AND u.status = 1
		ORDER BY u.id ASC`, phone)
	if switchErr == nil {
		defer switchRows.Close()
		for switchRows.Next() {
			var switchID int
			var switchUsername, switchRealName, switchEmployeeID, switchRoleName string
			if err := switchRows.Scan(&switchID, &switchUsername, &switchRealName, &switchEmployeeID, &switchRoleName); err == nil {
				var switchPrimaryRoleID sql.NullInt32
				_ = db.QueryRow("SELECT role_id FROM base_manage_user WHERE id = ?", switchID).Scan(&switchPrimaryRoleID)
				_, _, switchRoleNames := getUserRoles(db, switchID, int(switchPrimaryRoleID.Int32))
				if len(switchRoleNames) > 0 {
					switchRoleName = strings.Join(switchRoleNames, "、")
				}
				switchIdentities = append(switchIdentities, utils.H{
					"user_id":     switchID,
					"username":    switchUsername,
					"real_name":   switchRealName,
					"employee_id": switchEmployeeID,
					"role_name":   switchRoleName,
					"current":     switchID == id,
				})
			}
		}
	}
	responseData := utils.H{
		"id":                id,
		"username":          username,
		"real_name":         realName,
		"employee_id":       employeeID,
		"phone":             phone,
		"role":              role,
		"roles":             roles,
		"role_ids":          roleIDs,
		"role_names":        roleNames,
		"status":            status,
		"department":        department,
		"permissions":       permissions,
		"switch_identities": switchIdentities,
	}

	// 只在 lastLoginTime 有效时添加到响应数据中
	if lastLoginTime.Valid {
		responseData["last_login_time"] = lastLoginTime.Time.Format("2006-01-02T15:04:05+08:00")
	} else {
		responseData["last_login_time"] = nil
	}

	// 返回用户信息
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取用户信息成功",
		Data:    responseData,
	})
}

func buildMiniappIdentityListByPhone(db *sql.DB, phone string) ([]utils.H, error) {
	identityList := []utils.H{}

	userRows, userErr := db.Query("SELECT id, username, real_name, phone, role_id, status FROM base_manage_user WHERE phone = ? AND status = 1 ORDER BY id ASC", phone)
	if userErr == nil {
		defer userRows.Close()
		for userRows.Next() {
			var userID, status int
			var username, realName, userPhone string
			var roleID sql.NullInt32
			if err := userRows.Scan(&userID, &username, &realName, &userPhone, &roleID, &status); err != nil {
				log.Printf("Failed to scan miniapp employee identity: %v", err)
				continue
			}
			roleIDInt := 0
			if roleID.Valid {
				roleIDInt = int(roleID.Int32)
			}
			role := utils.H{}
			var roleName, roleDescription string
			if roleErr := db.QueryRow("SELECT name, description FROM setting_role WHERE id = ?", roleIDInt).Scan(&roleName, &roleDescription); roleErr == nil {
				role = utils.H{"id": roleIDInt, "name": roleName, "description": roleDescription}
			}
			roles, roleIDs, roleNames := getUserRoles(db, userID, roleIDInt)
			if len(roles) > 0 {
				role = roles[0]
			}
			employeeInfo := utils.H{
				"user_id":    userID,
				"username":   username,
				"real_name":  realName,
				"phone":      userPhone,
				"role":       role,
				"roles":      roles,
				"role_ids":   roleIDs,
				"role_names": roleNames,
				"status":     status,
			}
			identityList = append(identityList, utils.H{
				"identity_type": "employee",
				"user_id":       userID,
				"username":      username,
				"real_name":     realName,
				"info":          employeeInfo,
			})
		}
	} else {
		return nil, userErr
	}

	rows, err := db.Query(`SELECT id, patient_code, name, gender, id_card, phone, birthday,
		address, diagnosis, cancer_diameter, smoking_status, sales_person,
		is_active, completion_status, patient_status
		FROM detect_patient WHERE phone = ? AND is_active = 1`, phone)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id, isActive, completionStatus, patientStatus int
		var patientCode, name, gender, idCard, patientPhone string
		var birthday sql.NullTime
		var address, diagnosis, cancerDiameter, smokingStatus, salesPerson sql.NullString
		if err := rows.Scan(&id, &patientCode, &name, &gender, &idCard, &patientPhone, &birthday,
			&address, &diagnosis, &cancerDiameter, &smokingStatus, &salesPerson,
			&isActive, &completionStatus, &patientStatus); err != nil {
			log.Printf("Failed to scan miniapp switch patient identity: %v", err)
			continue
		}
		patientInfo := utils.H{
			"patient_id":        id,
			"patient_code":      patientCode,
			"name":              name,
			"gender":            gender,
			"id_card":           idCard,
			"phone":             patientPhone,
			"is_active":         isActive,
			"completion_status": completionStatus,
			"patient_status":    patientStatus,
		}
		if birthday.Valid {
			patientInfo["birthday"] = birthday.Time.Format("2006-01-02")
		}
		if address.Valid {
			patientInfo["address"] = address.String
		}
		if diagnosis.Valid {
			patientInfo["diagnosis"] = diagnosis.String
		}
		if cancerDiameter.Valid {
			patientInfo["cancer_diameter"] = cancerDiameter.String
		}
		if smokingStatus.Valid {
			patientInfo["smoking_status"] = smokingStatus.String
		}
		if salesPerson.Valid {
			patientInfo["sales_person"] = salesPerson.String
		}
		identityList = append(identityList, utils.H{
			"identity_type": "patient",
			"patient_id":    id,
			"name":          name,
			"patient_code":  patientCode,
			"info":          patientInfo,
		})
	}

	return identityList, nil
}

func intFromInterface(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		i, _ := strconv.Atoi(v.String())
		return i
	case string:
		i, _ := strconv.Atoi(strings.TrimSpace(v))
		return i
	default:
		return 0
	}
}

func HandleMiniappSwitchIdentity(c *app.RequestContext, db *sql.DB) {
	sessionID := strings.TrimSpace(string(c.GetHeader("X-Miniapp-Session")))
	if sessionID == "" {
		sessionID = strings.TrimSpace(string(c.Cookie("miniapp_session_id")))
	}
	if sessionID == "" {
		c.JSON(consts.StatusUnauthorized, ApiResponse{Code: 401, Success: false, Message: "请先登录", Data: nil})
		return
	}

	var req struct {
		IdentityType string `json:"identity_type"`
		UserID       int    `json:"user_id"`
		PatientID    int    `json:"patient_id"`
	}
	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "请求参数错误", Data: nil})
		return
	}
	req.IdentityType = strings.TrimSpace(req.IdentityType)

	var phone string
	var expiry time.Time
	if err := db.QueryRow("SELECT phone, expiry FROM base_miniapp_sessions WHERE session_id = ? AND expiry > NOW()", sessionID).Scan(&phone, &expiry); err != nil {
		c.JSON(consts.StatusUnauthorized, ApiResponse{Code: 401, Success: false, Message: "登录已失效，请重新登录", Data: nil})
		return
	}

	identityList, err := buildMiniappIdentityListByPhone(db, phone)
	if err != nil {
		log.Printf("Build miniapp identity list error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "切换身份失败", Data: nil})
		return
	}

	var selected utils.H
	var userIDArg interface{}
	var patientIDArg interface{}
	for _, identity := range identityList {
		if reportStringValue(identity["identity_type"]) != req.IdentityType {
			continue
		}
		info, _ := identity["info"].(utils.H)
		if info == nil {
			if rawInfo, ok := identity["info"].(map[string]interface{}); ok {
				info = utils.H(rawInfo)
			}
		}
		if req.IdentityType == "employee" {
			candidateUserID := intFromInterface(identity["user_id"])
			if candidateUserID == 0 && info != nil {
				candidateUserID = intFromInterface(info["user_id"])
			}
			if req.UserID > 0 && candidateUserID != req.UserID {
				continue
			}
			selected = identity
			userIDArg = candidateUserID
			patientIDArg = nil
			break
		}
		if req.IdentityType == "patient" {
			candidatePatientID := intFromInterface(identity["patient_id"])
			if candidatePatientID == 0 && info != nil {
				candidatePatientID = intFromInterface(info["patient_id"])
			}
			if req.PatientID > 0 && candidatePatientID != req.PatientID {
				continue
			}
			selected = identity
			userIDArg = nil
			patientIDArg = candidatePatientID
			break
		}
	}

	if selected == nil {
		c.JSON(consts.StatusForbidden, ApiResponse{Code: 403, Success: false, Message: "该手机号无此身份", Data: nil})
		return
	}

	if _, err := db.Exec(`UPDATE base_miniapp_sessions
		SET identity_type = ?, user_id = ?, patient_id = ?, updated_at = NOW()
		WHERE session_id = ?`, req.IdentityType, userIDArg, patientIDArg, sessionID); err != nil {
		log.Printf("Switch miniapp identity error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "切换身份失败", Data: nil})
		return
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "切换成功",
		Data: utils.H{
			"session_id":    sessionID,
			"user_info":     selected,
			"identity_list": identityList,
			"expiry":        expiry.Format("2006-01-02T15:04:05+08:00"),
		},
	})
}

func HandleOneClickLogin(c *app.RequestContext, db *sql.DB) {
	if db == nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "数据库连接失败，请联系管理员",
			Data:    nil,
		})
		return
	}

	var rawReq struct {
		Phone         string `json:"phone"`
		AutoLogin     bool   `json:"autoLogin"`
		Code          string `json:"code"`
		PhoneCode     string `json:"phoneCode"`
		JsCode        string `json:"jsCode"`
		EncryptedData string `json:"encryptedData"`
		Iv            string `json:"iv"`
	}

	body, err := c.Body()
	if err != nil {
		log.Printf("Failed to read request body: %v", err)
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "读取请求体失败",
			Data:    nil,
		})
		return
	}

	if err := json.Unmarshal(body, &rawReq); err != nil {
		log.Printf("Failed to unmarshal request body: %v", err)
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "解析请求体失败 " + err.Error(),
			Data:    nil,
		})
		return
	}

	phone := strings.TrimSpace(rawReq.Phone)
	var phoneResolveErr error

	phoneCode := strings.TrimSpace(rawReq.PhoneCode)
	if phoneCode == "" {
		phoneCode = strings.TrimSpace(rawReq.Code)
	}

	if phone == "" && phoneCode != "" {
		wechatPhone, err := getPhoneNumberFromWechat(phoneCode)
		if err != nil {
			phoneResolveErr = err
			log.Printf("Failed to get phone from Wechat phone code: %v", err)
		} else {
			phone = strings.TrimSpace(wechatPhone)
			log.Printf("Successfully got phone from Wechat: %s", phone)
		}
	}

	if phone == "" && rawReq.EncryptedData != "" && rawReq.Iv != "" && isWechatConfigured() {
		jsCode := strings.TrimSpace(rawReq.JsCode)
		if jsCode == "" && rawReq.PhoneCode == "" {
			jsCode = strings.TrimSpace(rawReq.Code)
		}
		wechatPhone, err := getPhoneNumberFromEncryptedData(jsCode, rawReq.EncryptedData, rawReq.Iv)
		if err != nil {
			phoneResolveErr = err
			log.Printf("Failed to decrypt Wechat phone data: %v", err)
		} else {
			phone = strings.TrimSpace(wechatPhone)
			log.Printf("Successfully decrypted phone from Wechat encryptedData: %s", phone)
		}
	}

	if phone == "" {
		message := "手机号不能为空"
		if rawReq.Code != "" || rawReq.PhoneCode != "" || rawReq.EncryptedData != "" || rawReq.Iv != "" {
			if !isWechatConfigured() {
				message = "微信小程序配置未完成，无法获取手机号"
			} else if phoneResolveErr != nil {
				message = "微信手机号获取失败：" + phoneResolveErr.Error()
			} else {
				message = "微信未返回手机号，请重新授权"
			}
		}
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: message,
			Data:    nil,
		})
		return
	}

	hasEmployee := false
	hasPatient := false
	var employeeInfo map[string]interface{}
	var patientInfoList []map[string]interface{}

	var userID int
	var username, realName string
	var roleID sql.NullInt32

	employeeQuery := "SELECT id, username, real_name, role_id FROM base_manage_user WHERE phone = ?"
	err = db.QueryRow(employeeQuery, phone).Scan(&userID, &username, &realName, &roleID)
	if err == nil {
		hasEmployee = true
		roleIDInt := 0
		if roleID.Valid {
			roleIDInt = int(roleID.Int32)
		}
		employeeInfo = map[string]interface{}{
			"user_id":   userID,
			"username":  username,
			"real_name": realName,
			"role_id":   roleIDInt,
		}
	} else if err != sql.ErrNoRows {
		log.Printf("Query employee error: %v", err)
	}

	patientQuery := `SELECT id, patient_code, name, gender, id_card, phone, birthday,
		address, diagnosis, cancer_diameter, smoking_status, sales_person,
		is_active, completion_status, patient_status
		FROM detect_patient WHERE phone = ?`
	rows, err := db.Query(patientQuery, phone)
	if err != nil {
		log.Printf("Query patient error: %v", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var id, isActive, completionStatus, patientStatus int
			var patientCode, name, gender, idCard, phone string
			var birthday sql.NullTime
			var address, diagnosis, cancerDiameter, smokingStatus sql.NullString
			var salesPerson sql.NullString

			err := rows.Scan(&id, &patientCode, &name, &gender, &idCard, &phone, &birthday,
				&address, &diagnosis, &cancerDiameter, &smokingStatus, &salesPerson,
				&isActive, &completionStatus, &patientStatus)
			if err != nil {
				log.Printf("Failed to scan patient: %v", err)
				continue
			}

			hasPatient = true
			patientInfo := map[string]interface{}{
				"patient_id":        id,
				"patient_code":      patientCode,
				"name":              name,
				"gender":            gender,
				"id_card":           idCard,
				"phone":             phone,
				"is_active":         isActive,
				"completion_status": completionStatus,
				"patient_status":    patientStatus,
			}

			if birthday.Valid {
				patientInfo["birthday"] = birthday.Time.Format("2006-01-02")
			}
			if address.Valid {
				patientInfo["address"] = address.String
			}
			if diagnosis.Valid {
				patientInfo["diagnosis"] = diagnosis.String
			}
			if cancerDiameter.Valid {
				patientInfo["cancer_diameter"] = cancerDiameter.String
			}
			if smokingStatus.Valid {
				patientInfo["smoking_status"] = smokingStatus.String
			}
			if salesPerson.Valid {
				patientInfo["sales_person"] = salesPerson.String
			}

			patientInfoList = append(patientInfoList, patientInfo)
		}
	}

	if !hasEmployee && !hasPatient {
		miniappNeedPatientRegister(c, phone)
		return
	}

	var expiresIn time.Duration
	if rawReq.AutoLogin {
		expiresIn = 7 * 24 * time.Hour
	} else {
		expiresIn = 24 * time.Hour
	}
	expiry := time.Now().Add(expiresIn)

	var sessionID string
	if hasEmployee {
		sessionID = fmt.Sprintf("%d_%d_%s", userID, time.Now().UnixNano(), generateRandomString(32))
	} else {
		sessionID = fmt.Sprintf("p_%d_%s", time.Now().UnixNano(), generateRandomString(32))
	}

	var defaultIdentityType string
	var defaultPatientID interface{}
	var defaultUserID interface{}

	if hasEmployee && !hasPatient {
		defaultIdentityType = "employee"
		defaultUserID = userID
	} else if !hasEmployee && hasPatient {
		defaultIdentityType = "patient"
		defaultUserID = nil
		if len(patientInfoList) > 0 {
			defaultPatientID = patientInfoList[0]["patient_id"]
		}
	}

	_, err = db.Exec(
		"INSERT INTO base_miniapp_sessions (session_id, user_id, phone, identity_type, patient_id, expiry, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, NOW(), NOW())",
		sessionID, defaultUserID, phone, defaultIdentityType, defaultPatientID, expiry,
	)
	if err != nil {
		log.Printf("Failed to create miniapp session: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	c.SetCookie(
		"miniapp_session_id",
		sessionID,
		int(expiresIn.Seconds()),
		"/",
		"",
		protocol.CookieSameSiteLaxMode,
		false,
		true,
	)

	if (hasEmployee && hasPatient) || (hasEmployee && len(patientInfoList) > 1) {
		identityList := []utils.H{}

		if hasEmployee {
			identityList = append(identityList, utils.H{
				"identity_type": "employee",
				"user_id":       userID,
				"username":      username,
				"real_name":     realName,
				"info":          employeeInfo,
			})
		}

		for _, patientInfo := range patientInfoList {
			identityList = append(identityList, utils.H{
				"identity_type": "patient",
				"patient_id":    patientInfo["patient_id"],
				"name":          patientInfo["name"],
				"patient_code":  patientInfo["patient_code"],
				"info":          patientInfo,
			})
		}

		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "登录成功，请选择身份",
			Data: utils.H{
				"session_id":    sessionID,
				"need_select":   true,
				"identity_list": identityList,
			},
		})
		return
	}

	var userInfo utils.H
	if hasEmployee {
		userInfo = utils.H{
			"user_id":   userID,
			"username":  username,
			"real_name": realName,
			"role_id": func() int {
				if roleID.Valid {
					return int(roleID.Int32)
				}
				return 0
			}(),
		}
	} else {
		userInfo = utils.H{
			"patient_id":   patientInfoList[0]["patient_id"],
			"name":         patientInfoList[0]["name"],
			"patient_code": patientInfoList[0]["patient_code"],
		}
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "登录成功",
		Data: utils.H{
			"session_id":  sessionID,
			"need_select": false,
			"user_info":   userInfo,
		},
	})
}

func HandlePhoneIdentities(c *app.RequestContext, db *sql.DB) {
	if db == nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "数据库连接失败，请联系管理员",
			Data:    nil,
		})
		return
	}

	var req struct {
		Phone string `json:"phone" binding:"required"`
	}

	body, err := c.Body()
	if err != nil {
		log.Printf("Failed to read request body: %v", err)
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "读取请求体失败",
			Data:    nil,
		})
		return
	}

	if err := json.Unmarshal(body, &req); err != nil {
		log.Printf("Failed to unmarshal request body: %v", err)
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "解析请求体失败 " + err.Error(),
			Data:    nil,
		})
		return
	}

	if req.Phone == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "手机号不能为空",
			Data:    nil,
		})
		return
	}

	hasEmployee := false
	hasPatient := false
	var employeeInfo map[string]interface{}
	var patientInfoList []map[string]interface{}

	var userID int
	var username, realName string
	var roleID sql.NullInt32

	employeeQuery := "SELECT id, username, real_name, role_id FROM base_manage_user WHERE phone = ?"
	err = db.QueryRow(employeeQuery, req.Phone).Scan(&userID, &username, &realName, &roleID)
	if err == nil {
		hasEmployee = true
		roleIDInt := 0
		if roleID.Valid {
			roleIDInt = int(roleID.Int32)
		}
		employeeInfo = map[string]interface{}{
			"user_id":   userID,
			"username":  username,
			"real_name": realName,
			"role_id":   roleIDInt,
		}
	} else if err != sql.ErrNoRows {
		log.Printf("Query employee error: %v", err)
	}

	patientQuery := `SELECT id, patient_code, name, gender, id_card, phone, birthday,
		address, diagnosis, cancer_diameter, smoking_status, sales_person,
		is_active, completion_status, patient_status
		FROM detect_patient WHERE phone = ?`
	rows, err := db.Query(patientQuery, req.Phone)
	if err != nil {
		log.Printf("Query patient error: %v", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var id, isActive, completionStatus, patientStatus int
			var patientCode, name, gender, idCard, phone string
			var birthday time.Time
			var address, diagnosis, cancerDiameter, smokingStatus sql.NullString
			var salesPerson sql.NullString

			err := rows.Scan(&id, &patientCode, &name, &gender, &idCard, &phone, &birthday,
				&address, &diagnosis, &cancerDiameter, &smokingStatus, &salesPerson,
				&isActive, &completionStatus, &patientStatus)
			if err != nil {
				log.Printf("Failed to scan patient: %v", err)
				continue
			}

			hasPatient = true
			patientInfo := map[string]interface{}{
				"patient_id":        id,
				"patient_code":      patientCode,
				"name":              name,
				"gender":            gender,
				"id_card":           idCard,
				"phone":             phone,
				"is_active":         isActive,
				"completion_status": completionStatus,
				"patient_status":    patientStatus,
			}

			if birthday.Year() > 1 {
				patientInfo["birthday"] = birthday.Format("2006-01-02")
			}
			if address.Valid {
				patientInfo["address"] = address.String
			}
			if diagnosis.Valid {
				patientInfo["diagnosis"] = diagnosis.String
			}
			if cancerDiameter.Valid {
				patientInfo["cancer_diameter"] = cancerDiameter.String
			}
			if smokingStatus.Valid {
				patientInfo["smoking_status"] = smokingStatus.String
			}
			if salesPerson.Valid {
				patientInfo["sales_person"] = salesPerson.String
			}

			patientInfoList = append(patientInfoList, patientInfo)
		}
	}

	responseData := utils.H{
		"hasEmployee":  hasEmployee,
		"hasPatient":   hasPatient,
		"employeeInfo": nil,
		"patientInfo":  nil,
	}

	if hasEmployee {
		responseData["employeeInfo"] = employeeInfo
	}
	if hasPatient {
		responseData["patientInfo"] = patientInfoList
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "查询成功",
		Data:    responseData,
	})
}

func HandleTestLogin(c *app.RequestContext, db *sql.DB) {
	if db == nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "数据库连接失败，请联系管理员",
			Data:    nil,
		})
		return
	}

	var req struct {
		Phone string `json:"phone" binding:"required"`
	}

	body, err := c.Body()
	if err != nil {
		log.Printf("Failed to read request body: %v", err)
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "读取请求体失败",
			Data:    nil,
		})
		return
	}

	if err := json.Unmarshal(body, &req); err != nil {
		log.Printf("Failed to unmarshal request body: %v", err)
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "解析请求体失败 " + err.Error(),
			Data:    nil,
		})
		return
	}

	if req.Phone == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "手机号不能为空",
			Data:    nil,
		})
		return
	}

	hasEmployee := false
	hasPatient := false
	var employeeInfo map[string]interface{}
	var patientInfoList []map[string]interface{}

	var userID int
	var username, realName string
	var roleID sql.NullInt32

	employeeQuery := "SELECT id, username, real_name, role_id FROM base_manage_user WHERE phone = ?"
	err = db.QueryRow(employeeQuery, req.Phone).Scan(&userID, &username, &realName, &roleID)
	if err == nil {
		hasEmployee = true
		roleIDInt := 0
		if roleID.Valid {
			roleIDInt = int(roleID.Int32)
		}
		employeeInfo = map[string]interface{}{
			"user_id":   userID,
			"username":  username,
			"real_name": realName,
			"role_id":   roleIDInt,
		}
	} else if err != sql.ErrNoRows {
		log.Printf("Query employee error: %v", err)
	}

	patientQuery := `SELECT id, patient_code, name, gender, id_card, phone, birthday,
		address, diagnosis, cancer_diameter, smoking_status, sales_person,
		is_active, completion_status, patient_status
		FROM detect_patient WHERE phone = ?`
	rows, err := db.Query(patientQuery, req.Phone)
	if err != nil {
		log.Printf("Query patient error: %v", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var id, isActive, completionStatus, patientStatus int
			var patientCode, name, gender, idCard, phone string
			var birthday sql.NullTime
			var address, diagnosis, cancerDiameter, smokingStatus sql.NullString
			var salesPerson sql.NullString

			err := rows.Scan(&id, &patientCode, &name, &gender, &idCard, &phone, &birthday,
				&address, &diagnosis, &cancerDiameter, &smokingStatus, &salesPerson,
				&isActive, &completionStatus, &patientStatus)
			if err != nil {
				log.Printf("Failed to scan patient: %v", err)
				continue
			}

			hasPatient = true
			patientInfo := map[string]interface{}{
				"patient_id":        id,
				"patient_code":      patientCode,
				"name":              name,
				"gender":            gender,
				"id_card":           idCard,
				"phone":             phone,
				"is_active":         isActive,
				"completion_status": completionStatus,
				"patient_status":    patientStatus,
			}

			if birthday.Valid {
				patientInfo["birthday"] = birthday.Time.Format("2006-01-02")
			}
			if address.Valid {
				patientInfo["address"] = address.String
			}
			if diagnosis.Valid {
				patientInfo["diagnosis"] = diagnosis.String
			}
			if cancerDiameter.Valid {
				patientInfo["cancer_diameter"] = cancerDiameter.String
			}
			if smokingStatus.Valid {
				patientInfo["smoking_status"] = smokingStatus.String
			}
			if salesPerson.Valid {
				patientInfo["sales_person"] = salesPerson.String
			}

			patientInfoList = append(patientInfoList, patientInfo)
		}
	}

	if !hasEmployee && !hasPatient {
		miniappNeedPatientRegister(c, req.Phone)
		return
	}

	expiresIn := 24 * time.Hour
	sessionID := fmt.Sprintf("%d_%d_%s", userID, time.Now().UnixNano(), generateRandomString(32))
	expiry := time.Now().Add(expiresIn)

	var defaultUserID int
	var defaultPatientID int
	var defaultIdentityType string

	if hasEmployee {
		defaultUserID = userID
		defaultIdentityType = "employee"
	} else {
		defaultUserID = 0
		defaultIdentityType = "patient"
		if len(patientInfoList) > 0 {
			defaultPatientID = patientInfoList[0]["patient_id"].(int)
		}
	}

	_, err = db.Exec(
		"INSERT INTO base_miniapp_sessions (session_id, user_id, phone, identity_type, patient_id, expiry, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, NOW(), NOW())",
		sessionID, defaultUserID, req.Phone, defaultIdentityType, defaultPatientID, expiry,
	)
	if err != nil {
		log.Printf("Failed to create miniapp session: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	c.SetCookie(
		"miniapp_session_id",
		sessionID,
		int(expiresIn.Seconds()),
		"/",
		"",
		protocol.CookieSameSiteLaxMode,
		false,
		true,
	)

	if (hasEmployee && hasPatient) || (hasEmployee && len(patientInfoList) > 1) || (!hasEmployee && len(patientInfoList) > 1) {
		identityList := []utils.H{}

		if hasEmployee {
			identityList = append(identityList, utils.H{
				"identity_type": "employee",
				"user_id":       userID,
				"username":      username,
				"real_name":     realName,
				"info":          employeeInfo,
			})
		}

		for _, patientInfo := range patientInfoList {
			identityList = append(identityList, utils.H{
				"identity_type": "patient",
				"patient_id":    patientInfo["patient_id"],
				"name":          patientInfo["name"],
				"patient_code":  patientInfo["patient_code"],
				"info":          patientInfo,
			})
		}

		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "登录成功，请选择身份",
			Data: utils.H{
				"session_id":    sessionID,
				"need_select":   true,
				"identity_list": identityList,
			},
		})
		return
	}

	var userInfo utils.H
	if hasEmployee {
		userInfo = utils.H{
			"user_id":   userID,
			"username":  username,
			"real_name": realName,
			"role_id": func() int {
				if roleID.Valid {
					return int(roleID.Int32)
				}
				return 0
			}(),
		}
	} else {
		userInfo = utils.H{
			"patient_id":   patientInfoList[0]["patient_id"],
			"name":         patientInfoList[0]["name"],
			"patient_code": patientInfoList[0]["patient_code"],
		}
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "登录成功",
		Data: utils.H{
			"session_id":  sessionID,
			"need_select": false,
			"user_info":   userInfo,
		},
	})
}

func HandleUpdateUsername(c *app.RequestContext, db *sql.DB) {
	// 从上下文获取用户ID
	userID, exists := c.Get(UserIDKey)
	if !exists {
		c.JSON(consts.StatusUnauthorized, ApiResponse{
			Code:    401,
			Success: false,
			Message: "未认证",
			Data:    nil,
		})
		return
	}

	// 解析请求体
	var req struct {
		Username string `json:"username" binding:"required,min=3,max=20"`
	}

	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请求参数错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 检查用户名是否已存在
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM base_manage_user WHERE username = ? AND id != ?", req.Username, userID).Scan(&count)
	if err != nil {
		log.Printf("Check username existence error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	if count > 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "用户名已存在",
			Data:    nil,
		})
		return
	}

	// 更新用户名
	_, err = db.Exec("UPDATE base_manage_user SET username = ? WHERE id = ?", req.Username, userID)
	if err != nil {
		log.Printf("Update username error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 返回成功响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "用户名修改成功",
		Data:    nil,
	})
}

func HandleChangePassword(c *app.RequestContext, db *sql.DB) {
	// 解析请求体
	var req ChangePasswordRequest
	// 读取并打印请求体
	body, err := c.Body()
	if err != nil {
		log.Printf("Failed to read request body: %v", err)
	} else {
		log.Printf("ChangePassword request body: %s", string(body))
	}
	if err := c.Bind(&req); err != nil {
		log.Printf("Failed to bind request body: %v", err)
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "旧密码和新密码不能为空",
			Data:    nil,
		})
		return
	}

	// 从上下文获取用户ID
	userID, exists := c.Get(UserIDKey)
	if !exists {
		c.JSON(consts.StatusUnauthorized, ApiResponse{
			Code:    401,
			Success: false,
			Message: "未认证",
			Data:    nil,
		})
		return
	}

	// 查询当前用户密码和盐值
	var currentPassword string
	var salt string
	query := "SELECT password, salt FROM base_manage_user WHERE id = ?"
	err = db.QueryRow(query, userID).Scan(&currentPassword, &salt)
	if err != nil {
		log.Printf("Failed to get current password: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	// 验证旧密码（使用MD5 + 盐值）
	log.Printf("Current password from database: %s", currentPassword)
	log.Printf("Salt from database: %s", salt)
	log.Printf("Old password from request: %s", req.OldPassword)
	expectedPassword := hashManageUserPassword(req.OldPassword, salt)
	log.Printf("Expected password: %s", expectedPassword)
	if !matchManageUserPassword(currentPassword, req.OldPassword, salt) {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "旧密码错误",
			Data:    nil,
		})
		return
	}

	// 对新密码进行MD5 + 盐值加密
	newPassword := hashManageUserPassword(req.NewPassword, salt)

	// 更新密码
	updateQuery := "UPDATE base_manage_user SET password = ?, updated_at = NOW() WHERE id = ?"
	_, err = db.Exec(updateQuery, newPassword, userID)
	if err != nil {
		log.Printf("Failed to update password: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	// 返回成功响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "密码修改成功",
		Data:    nil,
	})
}

func HandleUpdateUser(c *app.RequestContext, db *sql.DB) {
	// 从上下文获取用户ID
	userID, exists := c.Get(UserIDKey)
	if !exists {
		c.JSON(consts.StatusUnauthorized, ApiResponse{
			Code:    401,
			Success: false,
			Message: "未认证",
			Data:    nil,
		})
		return
	}

	// 解析请求体
	var req UpdateUserRequest
	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "真实姓名和联系电话不能为空",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 更新用户信息到数据库，只更新实际存在的字段
	_, err := db.Exec(`UPDATE base_manage_user SET real_name = ?, phone = ?, updated_at = NOW() 
		WHERE id = ?`, req.RealName, req.Phone, userID)
	if err != nil {
		log.Printf("Failed to update user: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 返回成功响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "个人信息更新成功",
		Data:    nil,
	})
}

func normalizeSMSPurpose(purpose, client string) string {
	purpose = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(purpose), "-", "_"))
	client = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(client), "-", "_"))
	if purpose != "" {
		switch purpose {
		case "admin", "backend", "manage", "manager", "admin_login", "backend_login", "manage_login", "manager_login":
			return "admin_login"
		case "miniapp", "mini_app", "wechat", "wx", "patient", "miniapp_login", "mini_app_login", "wechat_login", "wx_login", "patient_login":
			return "miniapp_login"
		case "login", "sms_login", "phone_login", "captcha_login", "verification_login":
			if client == "admin" || client == "backend" || client == "manage" || client == "manager" {
				return "admin_login"
			}
			return "miniapp_login"
		case "admin_bind_phone", "backend_bind_phone", "manage_bind_phone", "manager_bind_phone":
			return "admin_bind_phone"
		case "miniapp_bind_phone", "mini_app_bind_phone", "wechat_bind_phone", "wx_bind_phone", "patient_bind_phone":
			return "miniapp_bind_phone"
		case "bind_phone", "phone_bind":
			if client == "miniapp" || client == "mini_app" || client == "wechat" || client == "wx" || client == "patient" {
				return "miniapp_bind_phone"
			}
			return "admin_bind_phone"
		case "invite", "invite_register":
			return "invite_register"
		default:
			if client == "admin" || client == "backend" || client == "manage" || client == "manager" {
				return "admin_login"
			}
			if client == "miniapp" || client == "mini_app" || client == "wechat" || client == "wx" || client == "patient" {
				return "miniapp_login"
			}
			return purpose
		}
	}
	if client == "admin" || client == "backend" || client == "manage" || client == "manager" {
		return "admin_login"
	}
	if client == "miniapp" || client == "mini_app" || client == "wechat" || client == "wx" || client == "patient" {
		return "miniapp_login"
	}
	return "miniapp_login"
}

func isAdminSMSPurpose(purpose string) bool {
	return purpose == "admin_login" || purpose == "backend_login"
}

func isPhoneBindSMSPurpose(purpose string) bool {
	return purpose == "admin_bind_phone" || purpose == "miniapp_bind_phone"
}

func isInviteRegisterSMSPurpose(purpose string) bool {
	return purpose == "invite_register"
}

const smsCodeTTL = 5 * time.Minute

func StartSMSCodeCleanup(db *sql.DB) {
	if db == nil {
		return
	}
	deleteExpiredSMSCodes(db)
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			deleteExpiredSMSCodes(db)
		}
	}()
}

func deleteExpiredSMSCodes(db *sql.DB) {
	if db == nil {
		return
	}
	if _, err := db.Exec("DELETE FROM base_sms_codes WHERE expires_at <= NOW()"); err != nil {
		log.Printf("Failed to delete expired SMS codes: %v", err)
	}
}

func verifyAndUseSMSCode(db *sql.DB, phone, purpose, code string) error {
	deleteExpiredSMSCodes(db)

	var smsID int
	var smsCode string
	err := db.QueryRow(`SELECT id, code FROM base_sms_codes
		WHERE phone = ? AND purpose = ? AND used = 0 AND expires_at > NOW()
		ORDER BY created_at DESC LIMIT 1`, phone, purpose).Scan(&smsID, &smsCode)
	if err != nil {
		return fmt.Errorf("验证码无效或已过期")
	}
	if smsCode != strings.TrimSpace(code) {
		return fmt.Errorf("验证码错误")
	}
	if _, err := db.Exec("UPDATE base_sms_codes SET used = 1 WHERE id = ?", smsID); err != nil {
		log.Printf("Failed to mark SMS code as used: %v", err)
	}
	deleteExpiredSMSCodes(db)
	return nil
}

func buildLoginSMSContentVar(code string) map[string]interface{} {
	return map[string]interface{}{
		"SMSvCode": code,
		"code":     code,
		"minute":   "5",
	}
}

func validateAdminSMSPhone(c *app.RequestContext, db *sql.DB, phone string) bool {
	var employeeCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM base_manage_user WHERE phone = ? AND status = 1", phone).Scan(&employeeCount); err != nil {
		log.Printf("Check admin sms employee phone error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return false
	}
	if employeeCount > 0 {
		return true
	}

	var patientCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM detect_patient WHERE phone = ? AND is_active = 1", phone).Scan(&patientCount); err != nil {
		log.Printf("Check admin sms patient phone error: %v", err)
	}
	if patientCount > 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "该平台只适用于管理员，患者请微信搜索华微智慧小程序。",
			Data:    nil,
		})
		return false
	}

	c.JSON(consts.StatusBadRequest, ApiResponse{
		Code:    400,
		Success: false,
		Message: "该手机号未绑定后台账号",
		Data:    nil,
	})
	return false
}

func createAdminSMSLoginSession(c *app.RequestContext, db *sql.DB, phone string) {
	var userID int
	var username string
	var realName string
	var userPhone string
	var roleID sql.NullInt32
	var status int
	var lastLoginTime sql.NullTime

	err := db.QueryRow(`SELECT id, username, real_name, phone, role_id, status, last_login_time
		FROM base_manage_user WHERE phone = ?`, phone).Scan(&userID, &username, &realName, &userPhone, &roleID, &status, &lastLoginTime)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(consts.StatusUnauthorized, ApiResponse{
				Code:    401,
				Success: false,
				Message: "该手机号未绑定后台账号",
				Data:    nil,
			})
			return
		}
		log.Printf("Admin SMS login user query error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	if status == 0 {
		c.JSON(consts.StatusUnauthorized, ApiResponse{
			Code:    401,
			Success: false,
			Message: "账号已被禁用",
			Data:    nil,
		})
		return
	}

	roleIDInt := 0
	if roleID.Valid {
		roleIDInt = int(roleID.Int32)
	}

	sessionID := fmt.Sprintf("%d_%d_%s", userID, time.Now().UnixNano(), generateRandomString(32))
	expiry := time.Now().Add(24 * time.Hour)

	_, err = db.Exec("DELETE FROM base_sessions WHERE user_id = ?", userID)
	if err != nil {
		log.Printf("Failed to delete old admin sms sessions: %v", err)
	}
	_, err = db.Exec(
		"INSERT INTO base_sessions (session_id, user_id, expiry, created_at, updated_at) VALUES (?, ?, ?, NOW(), NOW())",
		sessionID, userID, expiry,
	)
	if err != nil {
		log.Printf("Failed to store admin sms session: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	c.SetCookie("session_id", sessionID, 0, "/", "", protocol.CookieSameSiteLaxMode, false, true)
	_, _ = db.Exec("UPDATE base_manage_user SET last_login_time = ? WHERE id = ?", time.Now(), userID)

	var roleName string
	var roleDescription string
	role := utils.H{}
	if err := db.QueryRow("SELECT name, description FROM setting_role WHERE id = ?", roleIDInt).Scan(&roleName, &roleDescription); err == nil {
		role = utils.H{
			"id":          roleIDInt,
			"name":        roleName,
			"description": roleDescription,
		}
	}

	user := utils.H{
		"id":        userID,
		"username":  username,
		"real_name": realName,
		"phone":     userPhone,
		"role_id":   roleIDInt,
		"role":      role,
		"status":    status,
	}
	if lastLoginTime.Valid {
		user["last_login_time"] = lastLoginTime.Time.Format("2006-01-02T15:04:05+08:00")
	} else {
		user["last_login_time"] = nil
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "登录成功",
		Data: utils.H{
			"user": user,
		},
	})
}

func HandleSmsSend(c *app.RequestContext, db *sql.DB) {
	if db == nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "数据库连接失败",
			Data:    nil,
		})
		return
	}

	var req struct {
		Phone     string `json:"phone" binding:"required"`
		Purpose   string `json:"purpose"`
		Client    string `json:"client"`
		BindToken string `json:"bind_token"`
	}

	body, err := c.Body()
	if err != nil {
		log.Printf("Failed to read request body: %v", err)
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "读取请求体失败",
			Data:    nil,
		})
		return
	}

	if err := json.Unmarshal(body, &req); err != nil {
		log.Printf("Failed to unmarshal request body: %v", err)
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "解析请求体失败",
			Data:    nil,
		})
		return
	}

	req.Phone = strings.TrimSpace(req.Phone)
	if req.Phone == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "手机号不能为空",
			Data:    nil,
		})
		return
	}

	rawPurpose := req.Purpose
	rawClient := req.Client
	req.Purpose = normalizeSMSPurpose(req.Purpose, req.Client)
	deleteExpiredSMSCodes(db)
	if !isSMSPurposeEnabled(req.Purpose) {
		writeSMSSendLog(db, req.Purpose, req.Phone, getBaiduSMSConfig().LoginTemplateID, "skipped", "DISABLED", "该类短信已关闭")
		c.JSON(consts.StatusForbidden, ApiResponse{
			Code:    403,
			Success: false,
			Message: fmt.Sprintf("%s已关闭", smsPurposeDisplayName(req.Purpose)),
			Data:    utils.H{"purpose": req.Purpose},
		})
		return
	}
	if isPhoneBindSMSPurpose(req.Purpose) {
		client := "admin"
		if req.Purpose == "miniapp_bind_phone" || req.Client == "miniapp" {
			client = "miniapp"
		}
		if _, err := getPhoneBindTokenUser(db, strings.TrimSpace(req.BindToken), client); err != nil {
			message := "绑定验证已失效，请重新密码登录"
			if client == "miniapp" {
				message = "绑定验证已失效，请重新登录"
			}
			c.JSON(consts.StatusBadRequest, ApiResponse{
				Code:    400,
				Success: false,
				Message: message,
				Data:    nil,
			})
			return
		}
	} else if isAdminSMSPurpose(req.Purpose) {
		if !validateAdminSMSPhone(c, db, req.Phone) {
			return
		}
	} else if !isInviteRegisterSMSPurpose(req.Purpose) && req.Purpose != "miniapp_login" {
		log.Printf("Invalid SMS purpose: rawPurpose=%q rawClient=%q normalizedPurpose=%q phone=%s", rawPurpose, rawClient, req.Purpose, req.Phone)
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "短信用途无效",
			Data:    nil,
		})
		return
	}

	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	code := fmt.Sprintf("%06d", seededRand.Intn(1000000))
	expiresAt := time.Now().Add(smsCodeTTL)

	result, err := db.Exec(`INSERT INTO base_sms_codes (phone, code, purpose, expires_at, used, created_at) VALUES (?, ?, ?, ?, 0, NOW())`,
		req.Phone, code, req.Purpose, expiresAt)
	if err != nil {
		log.Printf("Failed to store SMS code: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}
	smsCodeID, _ := result.LastInsertId()

	if err := sendBaiduSMS(db, req.Phone, getBaiduSMSConfig().LoginTemplateID, req.Purpose, buildLoginSMSContentVar(code)); err != nil {
		if smsCodeID > 0 {
			if _, cleanupErr := db.Exec("DELETE FROM base_sms_codes WHERE id = ?", smsCodeID); cleanupErr != nil {
				log.Printf("Failed to delete unsent SMS code %d: %v", smsCodeID, cleanupErr)
			}
		}
		log.Printf("Failed to send Baidu SMS code to %s: %v", req.Phone, err)
		message := fmt.Sprintf("短信发送失败：%s", err.Error())
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: message,
			Data: utils.H{
				"purpose": req.Purpose,
			},
		})
		return
	}

	log.Printf("SMS code generated for phone %s (purpose: %s)", req.Phone, req.Purpose)

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "验证码已发送",
		Data:    nil,
	})
}

func HandleSmsLogin(c *app.RequestContext, db *sql.DB) {
	if db == nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "数据库连接失败",
			Data:    nil,
		})
		return
	}

	var req struct {
		Phone   string `json:"phone" binding:"required"`
		Code    string `json:"code" binding:"required"`
		Purpose string `json:"purpose"`
		Client  string `json:"client"`
	}

	body, err := c.Body()
	if err != nil {
		log.Printf("Failed to read request body: %v", err)
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "读取请求体失败",
			Data:    nil,
		})
		return
	}

	if err := json.Unmarshal(body, &req); err != nil {
		log.Printf("Failed to unmarshal request body: %v", err)
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "解析请求体失败",
			Data:    nil,
		})
		return
	}

	req.Phone = strings.TrimSpace(req.Phone)
	req.Code = strings.TrimSpace(req.Code)
	if req.Phone == "" || req.Code == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "手机号和验证码不能为空",
			Data:    nil,
		})
		return
	}

	req.Purpose = normalizeSMSPurpose(req.Purpose, req.Client)
	if isAdminSMSPurpose(req.Purpose) && !validateAdminSMSPhone(c, db, req.Phone) {
		return
	}

	if err := verifyAndUseSMSCode(db, req.Phone, req.Purpose, req.Code); err != nil {
		c.JSON(consts.StatusUnauthorized, ApiResponse{
			Code:    401,
			Success: false,
			Message: err.Error(),
			Data:    nil,
		})
		return
	}

	if isAdminSMSPurpose(req.Purpose) {
		createAdminSMSLoginSession(c, db, req.Phone)
		return
	}

	hasEmployee := false
	hasPatient := false
	var patientInfoList []map[string]interface{}

	var userID int
	var username, realName, phone string
	var roleID sql.NullInt32
	var status int

	userQuery := "SELECT id, username, real_name, phone, role_id, status FROM base_manage_user WHERE phone = ?"
	userErr := db.QueryRow(userQuery, req.Phone).Scan(&userID, &username, &realName, &phone, &roleID, &status)
	if userErr == nil {
		hasEmployee = true
	} else if userErr != sql.ErrNoRows {
		log.Printf("SMS login employee query error: %v", userErr)
	}

	patientQuery := `SELECT id, patient_code, name, gender, id_card, phone, birthday,
		address, diagnosis, cancer_diameter, smoking_status, sales_person,
		is_active, completion_status, patient_status
		FROM detect_patient WHERE phone = ?`
	rows, patientErr := db.Query(patientQuery, req.Phone)
	if patientErr != nil {
		log.Printf("SMS login patient query error: %v", patientErr)
	} else {
		defer rows.Close()

		for rows.Next() {
			var id, isActive, completionStatus, patientStatus int
			var patientCode, name, gender, idCard, patientPhone string
			var birthday sql.NullTime
			var address, diagnosis, cancerDiameter, smokingStatus sql.NullString
			var salesPerson sql.NullString

			scanErr := rows.Scan(&id, &patientCode, &name, &gender, &idCard, &patientPhone, &birthday,
				&address, &diagnosis, &cancerDiameter, &smokingStatus, &salesPerson,
				&isActive, &completionStatus, &patientStatus)
			if scanErr != nil {
				log.Printf("Failed to scan SMS login patient: %v", scanErr)
				continue
			}

			hasPatient = true
			patientInfo := map[string]interface{}{
				"patient_id":        id,
				"patient_code":      patientCode,
				"name":              name,
				"gender":            gender,
				"id_card":           idCard,
				"phone":             patientPhone,
				"is_active":         isActive,
				"completion_status": completionStatus,
				"patient_status":    patientStatus,
			}

			if birthday.Valid {
				patientInfo["birthday"] = birthday.Time.Format("2006-01-02")
			}
			if address.Valid {
				patientInfo["address"] = address.String
			}
			if diagnosis.Valid {
				patientInfo["diagnosis"] = diagnosis.String
			}
			if cancerDiameter.Valid {
				patientInfo["cancer_diameter"] = cancerDiameter.String
			}
			if smokingStatus.Valid {
				patientInfo["smoking_status"] = smokingStatus.String
			}
			if salesPerson.Valid {
				patientInfo["sales_person"] = salesPerson.String
			}

			patientInfoList = append(patientInfoList, patientInfo)
		}
	}

	if !hasEmployee && !hasPatient {
		miniappNeedPatientRegister(c, req.Phone)
		return
	}

	var sessionID string
	expiry := time.Now().Add(24 * time.Hour)
	defaultIdentityType := ""
	defaultUserID := 0
	defaultPatientID := 0

	if hasEmployee && !hasPatient {
		defaultIdentityType = "employee"
		defaultUserID = userID
	} else if !hasEmployee && hasPatient {
		defaultIdentityType = "patient"
		if len(patientInfoList) > 0 {
			if patientID, ok := patientInfoList[0]["patient_id"].(int); ok {
				defaultPatientID = patientID
			}
		}
	}

	sessionID = fmt.Sprintf("%d_%d_%s", time.Now().Unix(), time.Now().UnixNano(), generateRandomString(32))

	_, err = db.Exec(
		"INSERT INTO base_miniapp_sessions (session_id, user_id, phone, identity_type, patient_id, expiry, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, NOW(), NOW())",
		sessionID, defaultUserID, req.Phone, defaultIdentityType, defaultPatientID, expiry,
	)
	if err != nil {
		log.Printf("Failed to create session: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	c.SetCookie(
		"miniapp_session_id",
		sessionID,
		86400, // 24小时
		"/",
		"",
		protocol.CookieSameSiteLaxMode,
		false,
		true,
	)

	identityList, identityErr := buildMiniappIdentityListByPhone(db, req.Phone)
	if identityErr != nil {
		log.Printf("Build SMS login identity list error: %v", identityErr)
	}
	if len(identityList) > 1 {
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "登录成功，请选择身份",
			Data: utils.H{
				"session_id":    sessionID,
				"need_select":   true,
				"identity_list": identityList,
			},
		})
		return
	}

	var userInfo utils.H
	if hasEmployee {
		userInfo = utils.H{
			"user_id":   userID,
			"username":  username,
			"real_name": realName,
			"role_id": func() int {
				if roleID.Valid {
					return int(roleID.Int32)
				}
				return 0
			}(),
		}
	} else {
		userInfo = utils.H{
			"patient_id":   patientInfoList[0]["patient_id"],
			"name":         patientInfoList[0]["name"],
			"patient_code": patientInfoList[0]["patient_code"],
		}
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "登录成功",
		Data: utils.H{
			"session_id":  sessionID,
			"need_select": false,
			"user_info":   userInfo,
		},
	})
}

func HandleBindPhone(c *app.RequestContext, db *sql.DB) {
	if db == nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "数据库连接失败", Data: nil})
		return
	}

	var req struct {
		Phone     string `json:"phone"`
		Code      string `json:"code"`
		BindToken string `json:"bind_token"`
		Client    string `json:"client"`
	}
	body, err := c.Body()
	if err != nil || json.Unmarshal(body, &req) != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "请求参数错误", Data: nil})
		return
	}
	req.Phone = strings.TrimSpace(req.Phone)
	req.Code = strings.TrimSpace(req.Code)
	req.Client = strings.TrimSpace(req.Client)
	if req.Client == "" {
		req.Client = "admin"
	}
	if req.Phone == "" || req.Code == "" || strings.TrimSpace(req.BindToken) == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "手机号、验证码和绑定令牌不能为空", Data: nil})
		return
	}

	userID, err := getPhoneBindTokenUser(db, strings.TrimSpace(req.BindToken), req.Client)
	if err != nil {
		message := "绑定验证已失效，请重新密码登录"
		if req.Client == "miniapp" {
			message = "绑定验证已失效，请重新登录"
		}
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: message, Data: nil})
		return
	}

	purpose := "admin_bind_phone"
	if req.Client == "miniapp" {
		purpose = "miniapp_bind_phone"
	}

	var existing int
	if err := db.QueryRow("SELECT COUNT(*) FROM base_manage_user WHERE phone = ? AND id != ?", req.Phone, userID).Scan(&existing); err == nil && existing > 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "该手机号已绑定其他后台用户", Data: nil})
		return
	}

	if err := verifyAndUseSMSCode(db, req.Phone, purpose, req.Code); err != nil {
		c.JSON(consts.StatusUnauthorized, ApiResponse{Code: 401, Success: false, Message: "验证码无效或已过期", Data: nil})
		return
	}

	_, err = db.Exec("UPDATE base_manage_user SET phone = ?, updated_at = NOW() WHERE id = ?", req.Phone, userID)
	if err != nil {
		log.Printf("Bind phone update user error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "服务器内部错误", Data: nil})
		return
	}
	markPhoneBindTokenUsed(db, req.BindToken)

	if req.Client == "miniapp" {
		createMiniappEmployeePasswordSession(c, db, userID, req.Phone)
		return
	}
	createAdminSMSLoginSession(c, db, req.Phone)
}

func verifyManageUserPassword(db *sql.DB, username, password string) (int, string, string, string, sql.NullInt32, int, bool, error) {
	var userID int
	var dbUsername, realName, phone, hashedPassword, salt string
	var roleID sql.NullInt32
	var status int
	err := db.QueryRow(`SELECT id, username, real_name, phone, password, salt, role_id, status
		FROM base_manage_user WHERE username = ?`, username).Scan(&userID, &dbUsername, &realName, &phone, &hashedPassword, &salt, &roleID, &status)
	if err != nil {
		return 0, "", "", "", roleID, 0, false, err
	}
	passwordValid := matchManageUserPassword(hashedPassword, password, salt)
	return userID, dbUsername, realName, phone, roleID, status, passwordValid, nil
}

func createMiniappEmployeePasswordSession(c *app.RequestContext, db *sql.DB, userID int, phone string) {
	var username, realName string
	var roleID sql.NullInt32
	if err := db.QueryRow("SELECT username, real_name, role_id FROM base_manage_user WHERE id = ?", userID).Scan(&username, &realName, &roleID); err != nil {
		log.Printf("Miniapp employee session user query error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "服务器内部错误", Data: nil})
		return
	}

	sessionID := fmt.Sprintf("%d_%d_%s", userID, time.Now().UnixNano(), generateRandomString(32))
	expiry := time.Now().Add(24 * time.Hour)
	_, err := db.Exec(`INSERT INTO base_miniapp_sessions (session_id, user_id, phone, identity_type, patient_id, expiry, created_at, updated_at)
		VALUES (?, ?, ?, 'employee', 0, ?, NOW(), NOW())`, sessionID, userID, phone, expiry)
	if err != nil {
		log.Printf("Miniapp employee session create error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "服务器内部错误", Data: nil})
		return
	}
	c.SetCookie("miniapp_session_id", sessionID, 86400, "/", "", protocol.CookieSameSiteLaxMode, false, true)
	roleIDInt := 0
	if roleID.Valid {
		roleIDInt = int(roleID.Int32)
	}
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "登录成功",
		Data: utils.H{
			"session_id":  sessionID,
			"need_select": false,
			"user_info": utils.H{
				"user_id":   userID,
				"username":  username,
				"real_name": realName,
				"role_id":   roleIDInt,
				"phone":     phone,
			},
		},
	})
}

func getWechatAppID() string {
	if db := GetDB(); db != nil {
		ensureSystemSettingDefaults(db)
	}
	return getRuntimeSetting("WECHAT_APP_ID", "WECHAT_APP_ID", "wxac666c112df0f8f9")
}

func getWechatAppSecret() string {
	if db := GetDB(); db != nil {
		ensureSystemSettingDefaults(db)
	}
	return getRuntimeSetting("WECHAT_APP_SECRET", "WECHAT_APP_SECRET", "")
}

func isWechatConfigured() bool {
	appID := getWechatAppID()
	appSecret := getWechatAppSecret()
	return appID != "" && appSecret != ""
}

func getWechatAccessToken() (string, error) {
	if !isWechatConfigured() {
		return "", fmt.Errorf("微信配置未完成")
	}

	appID := getWechatAppID()
	appSecret := getWechatAppSecret()

	url := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s", appID, appSecret)

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("获取access_token失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取access_token响应失败: %v", err)
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		Errcode     int    `json:"errcode"`
		Errmsg      string `json:"errmsg"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析access_token响应失败: %v", err)
	}

	if result.Errcode != 0 {
		return "", fmt.Errorf("微信API错误: %d - %s", result.Errcode, result.Errmsg)
	}

	return result.AccessToken, nil
}

func getWechatSessionKey(code string) (string, error) {
	if !isWechatConfigured() {
		return "", fmt.Errorf("微信配置未完成")
	}
	if strings.TrimSpace(code) == "" {
		return "", fmt.Errorf("缺少微信登录code")
	}

	appID := getWechatAppID()
	appSecret := getWechatAppSecret()
	url := fmt.Sprintf("https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code", appID, appSecret, code)

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("请求jscode2session失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取jscode2session响应失败: %v", err)
	}

	var result struct {
		SessionKey string `json:"session_key"`
		Errcode    int    `json:"errcode"`
		Errmsg     string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析jscode2session响应失败: %v", err)
	}
	if result.Errcode != 0 {
		return "", fmt.Errorf("jscode2session错误: %d - %s", result.Errcode, result.Errmsg)
	}
	if result.SessionKey == "" {
		return "", fmt.Errorf("微信未返回session_key")
	}

	return result.SessionKey, nil
}

func getWechatOpenID(code string) (string, error) {
	if !isWechatConfigured() {
		return "", fmt.Errorf("微信配置未完成")
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return "", nil
	}
	appID := getWechatAppID()
	appSecret := getWechatAppSecret()
	url := fmt.Sprintf("https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code", appID, appSecret, code)
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("请求jscode2session失败: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取jscode2session响应失败: %v", err)
	}
	var result struct {
		OpenID  string `json:"openid"`
		Errcode int    `json:"errcode"`
		Errmsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析jscode2session响应失败: %v", err)
	}
	if result.Errcode != 0 {
		return "", fmt.Errorf("jscode2session错误: %d - %s", result.Errcode, result.Errmsg)
	}
	return strings.TrimSpace(result.OpenID), nil
}

func getPhoneNumberFromEncryptedData(jsCode, encryptedData, iv string) (string, error) {
	sessionKey, err := getWechatSessionKey(jsCode)
	if err != nil {
		return "", err
	}

	plainText, err := decryptWechatEncryptedData(sessionKey, encryptedData, iv)
	if err != nil {
		return "", err
	}

	var result struct {
		PhoneNumber     string `json:"phoneNumber"`
		PurePhoneNumber string `json:"purePhoneNumber"`
	}
	if err := json.Unmarshal(plainText, &result); err != nil {
		return "", fmt.Errorf("解析手机号解密结果失败: %v", err)
	}
	if result.PhoneNumber != "" {
		return result.PhoneNumber, nil
	}
	if result.PurePhoneNumber != "" {
		return result.PurePhoneNumber, nil
	}

	return "", fmt.Errorf("微信解密结果中没有手机号")
}

func decryptWechatEncryptedData(sessionKey, encryptedData, iv string) ([]byte, error) {
	keyBytes, err := base64.StdEncoding.DecodeString(sessionKey)
	if err != nil {
		return nil, fmt.Errorf("session_key格式错误: %v", err)
	}
	cipherBytes, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return nil, fmt.Errorf("encryptedData格式错误: %v", err)
	}
	ivBytes, err := base64.StdEncoding.DecodeString(iv)
	if err != nil {
		return nil, fmt.Errorf("iv格式错误: %v", err)
	}
	if len(cipherBytes) == 0 || len(cipherBytes)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("encryptedData长度不正确")
	}
	if len(ivBytes) != aes.BlockSize {
		return nil, fmt.Errorf("iv长度不正确")
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("创建AES解密器失败: %v", err)
	}

	plainText := make([]byte, len(cipherBytes))
	mode := cipher.NewCBCDecrypter(block, ivBytes)
	mode.CryptBlocks(plainText, cipherBytes)

	plainText, err = pkcs7Unpad(plainText, aes.BlockSize)
	if err != nil {
		return nil, err
	}

	return plainText, nil
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, fmt.Errorf("PKCS7数据长度不正确")
	}
	padding := int(data[len(data)-1])
	if padding == 0 || padding > blockSize || padding > len(data) {
		return nil, fmt.Errorf("PKCS7填充不正确")
	}
	for _, value := range data[len(data)-padding:] {
		if int(value) != padding {
			return nil, fmt.Errorf("PKCS7填充不正确")
		}
	}
	return data[:len(data)-padding], nil
}

func getPhoneNumberFromWechat(code string) (string, error) {
	if !isWechatConfigured() {
		return "", fmt.Errorf("微信配置未完成")
	}

	accessToken, err := getWechatAccessToken()
	if err != nil {
		return "", fmt.Errorf("获取access_token失败: %v", err)
	}

	url := fmt.Sprintf("https://api.weixin.qq.com/wxa/business/getuserphonenumber?access_token=%s", accessToken)

	reqBody := map[string]string{"code": code}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("构建请求体失败: %v", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("请求微信API失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %v", err)
	}

	var result struct {
		Errcode   int    `json:"errcode"`
		Errmsg    string `json:"errmsg"`
		PhoneInfo struct {
			PhoneNumber     string `json:"phoneNumber"`
			PurePhoneNumber string `json:"purePhoneNumber"`
		} `json:"phone_info"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析微信响应失败: %v", err)
	}

	if result.Errcode != 0 {
		return "", fmt.Errorf("获取手机号失败: %d - %s", result.Errcode, result.Errmsg)
	}

	if result.PhoneInfo.PhoneNumber != "" {
		return result.PhoneInfo.PhoneNumber, nil
	}

	return result.PhoneInfo.PurePhoneNumber, nil
}
