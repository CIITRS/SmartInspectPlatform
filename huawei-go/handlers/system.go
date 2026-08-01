package handlers

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// 获取样本类型数据
func getSampleTypesData(db *sql.DB) (interface{}, error) {
	// 尝试从缓存获取
	cacheKey := "setting_sample_types"
	var cachedData []utils.H
	err := GetCache(cacheKey, &cachedData)
	if err == nil {
		// 缓存命中，直接返回
		return cachedData, nil
	}

	// 从数据库查询样本类型
	rows, err := db.Query(`SELECT id, name, description, is_active, created_at, updated_at 
			FROM setting_sample_type ORDER BY name ASC`)
	if err != nil {
		log.Printf("Failed to query sample types: %v", err)
		return []utils.H{}, nil
	}
	defer rows.Close()

	// 遍历查询结果
	var sampleTypes []utils.H
	for rows.Next() {
		var id int
		var name, description string
		var isActive int
		var createdAt, updatedAt time.Time

		err := rows.Scan(&id, &name, &description, &isActive, &createdAt, &updatedAt)
		if err != nil {
			log.Printf("Failed to scan sample type: %v", err)
			continue
		}

		// 构建样本类型信息
		sampleType := utils.H{
			"id":          id,
			"name":        name,
			"description": description,
			"is_active":   isActive,
			"created_at":  createdAt.Format("2006-01-02T15:04:05+08:00"),
			"updated_at":  updatedAt.Format("2006-01-02T15:04:05+08:00"),
		}

		sampleTypes = append(sampleTypes, sampleType)
	}

	// 检查遍历过程中是否有错误
	if err = rows.Err(); err != nil {
		log.Printf("Error iterating sample types: %v", err)
		return nil, err
	}

	// 缓存结果
	SetCache(cacheKey, sampleTypes, 24*time.Hour)

	// 返回样本类型列表，始终返回数组
	return sampleTypes, nil
}

// 获取治疗阶段数据
func getTreatmentStagesData(db *sql.DB) (interface{}, error) {
	// 尝试从缓存获取
	cacheKey := "setting_treatment_stages_allowed_v2"
	var cachedData []utils.H
	err := GetCache(cacheKey, &cachedData)
	if err == nil {
		// 缓存命中，直接返回
		return cachedData, nil
	}

	// 从数据库查询治疗阶段
	rows, err := db.Query(`SELECT id, name, description, is_active, created_at, updated_at 
				FROM setting_treatment_stage WHERE is_active = 1 ORDER BY FIELD(name, '健康体检', '辅助诊断', '术前评估', '术后检测', '残留检测', '复发监测', '化疗前', '化疗后'), id ASC`)
	if err != nil {
		log.Printf("Failed to query treatment stages: %v", err)
		return []utils.H{}, nil
	}
	defer rows.Close()

	// 遍历查询结果
	var treatmentStages []utils.H
	for rows.Next() {
		var id int
		var name, description string
		var isActive int
		var createdAt, updatedAt time.Time

		err := rows.Scan(&id, &name, &description, &isActive, &createdAt, &updatedAt)
		if err != nil {
			log.Printf("Failed to scan treatment stage: %v", err)
			continue
		}

		if !isAllowedTreatmentStage(name) {
			continue
		}

		// 构建治疗阶段信息
		treatmentStage := utils.H{
			"id":          id,
			"name":        name,
			"description": description,
			"is_active":   isActive,
			"created_at":  createdAt.Format("2006-01-02T15:04:05+08:00"),
			"updated_at":  updatedAt.Format("2006-01-02T15:04:05+08:00"),
		}

		treatmentStages = append(treatmentStages, treatmentStage)
	}

	// 检查遍历过程中是否有错误
	if err = rows.Err(); err != nil {
		log.Printf("Error iterating treatment stages: %v", err)
		return nil, err
	}

	// 缓存结果
	SetCache(cacheKey, treatmentStages, 24*time.Hour)

	// 返回治疗阶段列表，始终返回数组
	return treatmentStages, nil
}

// 获取癌症类型数据
func getCancerTypesData(db *sql.DB) (interface{}, error) {
	// 尝试从缓存获取
	cacheKey := "setting_cancer_types"
	var cachedData []utils.H
	err := GetCache(cacheKey, &cachedData)
	if err == nil {
		// 缓存命中，直接返回
		return cachedData, nil
	}

	// 从数据库查询癌症类型
	rows, err := db.Query(`SELECT id, name, description, is_active, created_at, updated_at, panel_ids 
			FROM setting_cancer_type ORDER BY name ASC`)
	if err != nil {
		log.Printf("Failed to query cancer types: %v", err)
		return []utils.H{}, nil
	}
	defer rows.Close()

	// 遍历查询结果
	var cancerTypes []utils.H
	for rows.Next() {
		var id int
		var name, description string
		var isActive int
		var createdAt, updatedAt time.Time
		var panelIDs sql.NullString

		err := rows.Scan(&id, &name, &description, &isActive, &createdAt, &updatedAt, &panelIDs)
		if err != nil {
			log.Printf("Failed to scan cancer type: %v", err)
			continue
		}

		// 查询关联的Panel信息
		var panels []utils.H
		var requiredGenes []string
		if panelIDs.Valid && panelIDs.String != "" {
			panelRows, err := db.Query(`SELECT id, panel_name, panel_code, COALESCE(gene_ids, '')
					FROM setting_panel 
					WHERE id IN (` + panelIDs.String + `) AND is_active = 1`)
			if err == nil {
				for panelRows.Next() {
					var panelID int
					var panelName, panelCode, geneIDsStr string
					if err := panelRows.Scan(&panelID, &panelName, &panelCode, &geneIDsStr); err == nil {
						geneSymbols := getPanelGeneSymbols(db, geneIDsStr)
						requiredGenes = append(requiredGenes, geneSymbols...)
						panels = append(panels, utils.H{
							"id":          panelID,
							"panelName":   panelName,
							"panelCode":   panelCode,
							"geneSymbols": geneSymbols,
						})
					}
				}
				panelRows.Close()
			}
		}

		// 构建癌症类型信息
		cancerType := utils.H{
			"id":            id,
			"name":          name,
			"description":   description,
			"is_active":     isActive,
			"created_at":    createdAt.Format("2006-01-02T15:04:05+08:00"),
			"updated_at":    updatedAt.Format("2006-01-02T15:04:05+08:00"),
			"panel_ids":     panelIDs.String,
			"panels":        panels,
			"requiredGenes": uniqueSortedGeneSymbols(requiredGenes),
		}

		cancerTypes = append(cancerTypes, cancerType)
	}

	// 检查遍历过程中是否有错误
	if err = rows.Err(); err != nil {
		log.Printf("Error iterating cancer types: %v", err)
		return nil, err
	}

	// 缓存结果
	SetCache(cacheKey, cancerTypes, 24*time.Hour)

	// 返回癌症类型列表，始终返回数组
	return cancerTypes, nil
}

// 获取用户数据
func getUsersData(db *sql.DB) (interface{}, error) {
	// 从数据库查询用户列表，使用base_manage_user表而不是user表，使用status字段而不是is_active
	rows, err := db.Query(`SELECT id, username, real_name as name, phone, department_id, role_id, status, last_login_time, created_at, updated_at, employee_id 
			FROM base_manage_user ORDER BY real_name ASC`)
	if err != nil {
		log.Printf("Failed to query users: %v", err)
		return []utils.H{}, nil
	}
	defer rows.Close()

	// 遍历查询结果，初始化为空数组而不是nil
	var users []utils.H = []utils.H{}
	for rows.Next() {
		var id, status int
		var setting_departmentId, setting_roleId sql.NullInt32
		var username, name, phone, employeeId string
		var lastLoginTime sql.NullTime
		var createdAt, updatedAt time.Time

		err := rows.Scan(&id, &username, &name, &phone, &setting_departmentId, &setting_roleId, &status, &lastLoginTime, &createdAt, &updatedAt, &employeeId)
		if err != nil {
			log.Printf("Failed to scan user: %v", err)
			continue
		}

		// 处理setting_departmentId和setting_roleId，将sql.NullInt32转换为普通int
		setting_departmentIdValue := 0
		if setting_departmentId.Valid {
			setting_departmentIdValue = int(setting_departmentId.Int32)
		}

		setting_roleIdValue := 0
		if setting_roleId.Valid {
			setting_roleIdValue = int(setting_roleId.Int32)
		}

		// 处理last_login_time - 格式化为中文格式"2026年06月01日 22:22:22"
		lastLoginTimeStr := "从未登录"
		if lastLoginTime.Valid {
			lastLoginTimeStr = lastLoginTime.Time.Format("2006年01月02日 15:04:05")
		}

		// 查询部门名称
		departmentName := ""
		if setting_departmentIdValue > 0 {
			var deptName string
			err := db.QueryRow("SELECT name FROM setting_department WHERE id = ?", setting_departmentIdValue).Scan(&deptName)
			if err == nil {
				departmentName = deptName
			}
		}

		// 查询角色名称
		roleName := ""
		if setting_roleIdValue > 0 {
			var rName string
			err := db.QueryRow("SELECT name FROM setting_role WHERE id = ?", setting_roleIdValue).Scan(&rName)
			if err == nil {
				roleName = rName
			}
		}
		roles, roleIDs, roleNames := getUserRoles(db, id, setting_roleIdValue)
		if len(roleNames) > 0 {
			roleName = strings.Join(roleNames, "、")
		}

		// 检查用户是否绑定小程序(查询uni_patient表是否有对应的手机号)
		bindMiniProgram := "否"
		if phone != "" {
			var count int
			err := db.QueryRow("SELECT COUNT(*) FROM uni_patient WHERE phone = ?", phone).Scan(&count)
			if err == nil && count > 0 {
				bindMiniProgram = "是"
			}
		}

		// 构建用户信息，使用前端期望的字段名
		user := utils.H{
			"id":                id,
			"username":          username,
			"real_name":         name,
			"name":              name, // 同时返回name字段保持兼容性
			"email":             "",   // base_manage_user表中没有email字段，使用空字符串
			"phone":             phone,
			"department_id":     setting_departmentIdValue,
			"department_name":   departmentName, // 部门名称 - 使用正确的字段名
			"role_id":           setting_roleIdValue,
			"role_ids":          roleIDs,
			"role_name":         roleName, // 角色名称 - 使用正确的字段名
			"role_names":        roleNames,
			"roles":             roles,
			"status":            status, // 直接返回status字段，前端使用status判断
			"isActive":          status, // 保持isActive映射，兼容其他组件
			"createdAt":         createdAt.Format("2006-01-02T15:04:05+08:00"),
			"updatedAt":         updatedAt.Format("2006-01-02T15:04:05+08:00"),
			"last_login_time":   lastLoginTimeStr, // 格式化为中文格式
			"employee_id":       employeeId,       // 工号字段
			"bind_mini_program": bindMiniProgram,  // 是否绑定小程序
		}

		users = append(users, user)
	}

	// 检查遍历过程中是否有错误
	if err = rows.Err(); err != nil {
		log.Printf("Error iterating users: %v", err)
		return nil, err
	}

	// 返回用户列表，使用前端期望的格式
	return utils.H{
		"list":  users,
		"total": len(users),
	}, nil
}

// 系统设置相关处理函数
func HandleGetSampleTypes(c *app.RequestContext, db *sql.DB) {
	// 尝试从缓存获取
	cacheKey := "setting_sample_types"
	var cachedData []utils.H
	err := GetCache(cacheKey, &cachedData)
	if err == nil {
		// 缓存命中，直接返回
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "获取样本类型成功",
			Data:    cachedData,
		})
		return
	}

	// 从数据库查询样本类型
	rows, err := db.Query(`SELECT id, name, description, is_active, created_at, updated_at 
			FROM setting_sample_type ORDER BY name ASC`)
	if err != nil {
		log.Printf("Failed to query sample types: %v", err)
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "获取样本类型成功",
			Data:    []utils.H{},
		})
		return
	}
	defer rows.Close()

	// 遍历查询结果
	var sampleTypes []utils.H
	for rows.Next() {
		var id int
		var name, description string
		var isActive int
		var createdAt, updatedAt time.Time

		err := rows.Scan(&id, &name, &description, &isActive, &createdAt, &updatedAt)
		if err != nil {
			log.Printf("Failed to scan sample type: %v", err)
			continue
		}

		// 构建样本类型信息
		sampleType := utils.H{
			"id":          id,
			"name":        name,
			"description": description,
			"is_active":   isActive,
			"created_at":  createdAt.Format("2006-01-02T15:04:05+08:00"),
			"updated_at":  updatedAt.Format("2006-01-02T15:04:05+08:00"),
		}

		sampleTypes = append(sampleTypes, sampleType)
	}

	// 检查遍历过程中是否有错误
	if err = rows.Err(); err != nil {
		log.Printf("Error iterating sample types: %v", err)
	}

	// 缓存结果
	SetCache(cacheKey, sampleTypes, 24*time.Hour)

	// 返回样本类型列表，始终返回数组
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取样本类型成功",
		Data:    sampleTypes,
	})
}

func HandleGetTreatmentStages(c *app.RequestContext, db *sql.DB) {
	// 尝试从缓存获取
	cacheKey := "setting_treatment_stages_allowed_v2"
	var cachedData []utils.H
	err := GetCache(cacheKey, &cachedData)
	if err == nil {
		// 缓存命中，直接返回
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "获取治疗阶段成功",
			Data:    cachedData,
		})
		return
	}

	// 从数据库查询治疗阶段
	rows, err := db.Query(`SELECT id, name, description, is_active, created_at, updated_at 
				FROM setting_treatment_stage WHERE is_active = 1 ORDER BY FIELD(name, '健康体检', '辅助诊断', '术前评估', '术后检测', '残留检测', '复发监测', '化疗前', '化疗后'), id ASC`)
	if err != nil {
		log.Printf("Failed to query treatment stages: %v", err)
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "获取治疗阶段成功",
			Data:    []utils.H{},
		})
		return
	}
	defer rows.Close()

	// 遍历查询结果
	var treatmentStages []utils.H
	for rows.Next() {
		var id int
		var name, description string
		var isActive int
		var createdAt, updatedAt time.Time

		err := rows.Scan(&id, &name, &description, &isActive, &createdAt, &updatedAt)
		if err != nil {
			log.Printf("Failed to scan treatment stage: %v", err)
			continue
		}

		if !isAllowedTreatmentStage(name) {
			continue
		}

		// 构建治疗阶段信息
		treatmentStage := utils.H{
			"id":          id,
			"name":        name,
			"description": description,
			"is_active":   isActive,
			"created_at":  createdAt.Format("2006-01-02T15:04:05+08:00"),
			"updated_at":  updatedAt.Format("2006-01-02T15:04:05+08:00"),
		}

		treatmentStages = append(treatmentStages, treatmentStage)
	}

	// 检查遍历过程中是否有错误
	if err = rows.Err(); err != nil {
		log.Printf("Error iterating treatment stages: %v", err)
	}

	// 缓存结果
	SetCache(cacheKey, treatmentStages, 24*time.Hour)

	// 返回治疗阶段列表，始终返回数组
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取治疗阶段成功",
		Data:    treatmentStages,
	})
}

func getModelsData(db *sql.DB, activeOnly bool, includeDeprecated bool) (interface{}, error) {
	cacheKey := fmt.Sprintf("models:activeOnly=%t:includeDeprecated=%t", activeOnly, includeDeprecated)
	var cachedData []utils.H
	if err := GetCache(cacheKey, &cachedData); err == nil {
		return cachedData, nil
	}

	geneSymbolMap := loadGeneSymbolMap(db)
	query := `SELECT ms.id, ms.model_name as name, ms.description, ms.model_version as modelType, ms.is_active, ms.parameters, ms.version, ms.cancer_type_id, ct.name as cancerTypeName, ms.formula, ms.model_mode, COALESCE(ms.is_deprecated, 0) as is_deprecated, ms.deprecated_at
			FROM setting_model ms
			LEFT JOIN setting_cancer_type ct ON ms.cancer_type_id = ct.id`
	conditions := make([]string, 0, 2)
	if activeOnly {
		conditions = append(conditions, "ms.is_active = 1")
	}
	if !includeDeprecated {
		conditions = append(conditions, "COALESCE(ms.is_deprecated, 0) = 0")
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY COALESCE(ms.is_deprecated, 0) ASC, ms.model_name ASC"

	rows, err := db.Query(query)
	if err != nil {
		return []utils.H{}, err
	}
	defer rows.Close()

	models := []utils.H{}
	for rows.Next() {
		var id, isActive, cancerTypeId, isDeprecated int
		var name, description, modelType, parameters, version, cancerTypeName, formula, modelMode sql.NullString
		var deprecatedAt sql.NullTime

		if err := rows.Scan(&id, &name, &description, &modelType, &isActive, &parameters, &version, &cancerTypeId, &cancerTypeName, &formula, &modelMode, &isDeprecated, &deprecatedAt); err != nil {
			log.Printf("Failed to scan model: %v", err)
			continue
		}

		model := utils.H{
			"id":             id,
			"name":           name.String,
			"modelName":      name.String,
			"description":    description.String,
			"modelType":      modelType.String,
			"isActive":       isActive,
			"is_active":      isActive,
			"version":        version.String,
			"parameters":     parameters.String,
			"cancerTypeId":   cancerTypeId,
			"cancerTypeName": cancerTypeName.String,
			"isDeprecated":   isDeprecated,
			"is_deprecated":  isDeprecated,
		}

		var paramsMap map[string]interface{}
		if parameters.Valid && parameters.String != "" {
			_ = json.Unmarshal([]byte(parameters.String), &paramsMap)
		}

		selectedGeneIDs := parseSelectedGeneIDs(parameters.String)
		if len(selectedGeneIDs) > 0 {
			model["selectedGenes"] = selectedGeneIDs
		}
		if formula.Valid {
			model["formula"] = formula.String
		}
		if modelMode.Valid {
			model["modelMode"] = modelMode.String
		}
		if deprecatedAt.Valid {
			model["deprecatedAt"] = deprecatedAt.Time.Format("2006-01-02T15:04:05+08:00")
		}

		geneSymbols := extractModelGeneSymbols(parameters.String, formula.String, geneSymbolMap)
		model["geneSymbols"] = geneSymbols
		model["genes"] = strings.Join(geneSymbols, ",")
		model["geneCount"] = len(geneSymbols)
		model["selectableInBatch"] = isActive == 1 && isDeprecated == 0

		applicableItems := []string{}
		if paramsMap != nil {
			if rawItems, ok := paramsMap["applicableItems"].([]interface{}); ok {
				for _, rawItem := range rawItems {
					if item, ok := rawItem.(string); ok && strings.TrimSpace(item) != "" {
						applicableItems = append(applicableItems, strings.TrimSpace(item))
					}
				}
			}
		}
		if len(applicableItems) == 0 && cancerTypeName.String != "" {
			applicableItems = []string{cancerTypeName.String}
		}
		model["applicableItems"] = uniqueSortedStrings(applicableItems)
		models = append(models, model)
	}
	if err := rows.Err(); err != nil {
		return models, err
	}
	SetCache(cacheKey, models, 24*time.Hour)
	return models, nil
}

func HandleSystemBootstrap(c *app.RequestContext, db *sql.DB) {
	resourceText := strings.TrimSpace(c.Query("resources"))
	if resourceText == "" {
		resourceText = "sampleTypes,treatmentStages,cancerTypes,users"
	}
	resources := strings.Split(resourceText, ",")
	data := utils.H{}
	errors := utils.H{}

	add := func(name string, value interface{}, err error) {
		if err != nil {
			errors[name] = err.Error()
		}
		data[name] = value
	}

	for _, resource := range resources {
		switch strings.TrimSpace(resource) {
		case "sampleTypes":
			value, err := getSampleTypesData(db)
			add("sampleTypes", value, err)
		case "treatmentStages":
			value, err := getTreatmentStagesData(db)
			add("treatmentStages", value, err)
		case "cancerTypes":
			value, err := getCancerTypesData(db)
			add("cancerTypes", value, err)
		case "users":
			value, err := getUsersData(db)
			add("users", value, err)
		case "models":
			activeOnly := c.Query("activeOnly") == "1" || strings.EqualFold(c.Query("activeOnly"), "true")
			includeDeprecated := true
			if c.Query("includeDeprecated") == "0" || strings.EqualFold(c.Query("includeDeprecated"), "false") {
				includeDeprecated = false
			}
			value, err := getModelsData(db, activeOnly, includeDeprecated)
			add("models", value, err)
		case "templates":
			value, err := getReportTemplatesData(db, map[string]string{})
			add("templates", value, err)
		case "reportPositions":
			value, err := getReportPositionsData(db)
			add("reportPositions", value, err)
		}
	}
	if len(errors) > 0 {
		data["errors"] = errors
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取页面初始化数据成功",
		Data:    data,
	})
}

func HandleListCancerTypes(c *app.RequestContext, db *sql.DB) {
	// 尝试从缓存获取
	cacheKey := "setting_cancer_types"
	var cachedData []utils.H
	err := GetCache(cacheKey, &cachedData)
	if err == nil {
		// 缓存命中，直接返回
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "获取检测癌种成功",
			Data:    cachedData,
		})
		return
	}

	// 从数据库查询癌症类型
	rows, err := db.Query(`SELECT id, name, description, is_active, created_at, updated_at, panel_ids 
			FROM setting_cancer_type ORDER BY name ASC`)
	if err != nil {
		log.Printf("Failed to query cancer types: %v", err)
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "获取检测癌种成功",
			Data:    []utils.H{},
		})
		return
	}
	defer rows.Close()

	// 遍历查询结果
	var cancerTypes []utils.H
	for rows.Next() {
		var id int
		var name, description string
		var isActive int
		var createdAt, updatedAt time.Time
		var panelIDs sql.NullString

		err := rows.Scan(&id, &name, &description, &isActive, &createdAt, &updatedAt, &panelIDs)
		if err != nil {
			log.Printf("Failed to scan cancer type: %v", err)
			continue
		}

		// 查询关联的Panel信息
		var panels []utils.H
		var requiredGenes []string
		if panelIDs.Valid && panelIDs.String != "" {
			panelRows, err := db.Query(`SELECT id, panel_name, panel_code, COALESCE(gene_ids, '')
					FROM setting_panel 
					WHERE id IN (` + panelIDs.String + `) AND is_active = 1`)
			if err == nil {
				for panelRows.Next() {
					var panelID int
					var panelName, panelCode, geneIDsStr string
					if err := panelRows.Scan(&panelID, &panelName, &panelCode, &geneIDsStr); err == nil {
						geneSymbols := getPanelGeneSymbols(db, geneIDsStr)
						requiredGenes = append(requiredGenes, geneSymbols...)
						panels = append(panels, utils.H{
							"id":          panelID,
							"panelName":   panelName,
							"panelCode":   panelCode,
							"geneSymbols": geneSymbols,
						})
					}
				}
				panelRows.Close()
			}
		}

		// 构建癌症类型信息
		cancerType := utils.H{
			"id":            id,
			"name":          name,
			"description":   description,
			"is_active":     isActive,
			"created_at":    createdAt.Format("2006-01-02T15:04:05+08:00"),
			"updated_at":    updatedAt.Format("2006-01-02T15:04:05+08:00"),
			"panel_ids":     panelIDs.String,
			"panels":        panels,
			"requiredGenes": uniqueSortedGeneSymbols(requiredGenes),
		}

		cancerTypes = append(cancerTypes, cancerType)
	}

	// 检查遍历过程中是否有错误
	if err = rows.Err(); err != nil {
		log.Printf("Error iterating cancer types: %v", err)
	}

	// 缓存结果
	SetCache(cacheKey, cancerTypes, 24*time.Hour)

	// 返回癌症类型列表，始终返回数组
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取检测癌种成功",
		Data:    cancerTypes,
	})
}

// HandleGetSelectableCancerTypes - 获取可选择的检测类型列表（Panel数量 <= 当前检测类型）
func HandleGetSelectableCancerTypes(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	currentCancerTypeIdParam := c.Param("currentCancerTypeId")

	var currentCancerTypeId int
	var err error

	if currentCancerTypeIdParam != "" && currentCancerTypeIdParam != "0" {
		currentCancerTypeId, err = strconv.Atoi(currentCancerTypeIdParam)
		if err != nil {
			c.JSON(consts.StatusBadRequest, ApiResponse{
				Code:    400,
				Success: false,
				Message: "无效的检测癌种ID",
				Data:    nil,
			})
			return
		}
	}

	// 如果当前没有检测类型，返回所有可用的检测类型
	if currentCancerTypeId <= 0 {
		rows, err := db.Query(`SELECT id, name, description, panel_ids 
			FROM setting_cancer_type WHERE is_active = 1 ORDER BY name ASC`)
		if err != nil {
			log.Printf("Failed to query cancer types: %v", err)
			c.JSON(consts.StatusOK, ApiResponse{
				Code:    200,
				Success: true,
				Message: "获取可选择检测癌种成功",
				Data:    []utils.H{},
			})
			return
		}
		defer rows.Close()

		var cancerTypes []utils.H
		for rows.Next() {
			var id int
			var name, description, panelIDsStr string

			if err := rows.Scan(&id, &name, &description, &panelIDsStr); err != nil {
				continue
			}

			// 计算Panel数量
			panelCount := 0
			if panelIDsStr != "" {
				panelIDs := strings.Split(panelIDsStr, ",")
				for _, panelID := range panelIDs {
					if strings.TrimSpace(panelID) != "" {
						panelCount++
					}
				}
			}

			cancerTypes = append(cancerTypes, utils.H{
				"id":          id,
				"name":        name,
				"description": description,
				"panelCount":  panelCount,
			})
		}

		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "获取可选择检测癌种成功",
			Data:    cancerTypes,
		})
		return
	}

	// 正确获取当前检测类型的Panel数量
	var currentPanelIDsStr string
	err = db.QueryRow(`SELECT panel_ids FROM setting_cancer_type WHERE id = ? AND is_active = 1`, currentCancerTypeId).Scan(&currentPanelIDsStr)
	if err != nil {
		log.Printf("Failed to get current cancer type panel IDs: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	currentPanelCount := 0
	if currentPanelIDsStr != "" {
		panelIDs := strings.Split(currentPanelIDsStr, ",")
		for _, panelID := range panelIDs {
			if strings.TrimSpace(panelID) != "" {
				currentPanelCount++
			}
		}
	}

	// 查询所有可用的检测类型
	rows, err := db.Query(`SELECT id, name, description, panel_ids 
		FROM setting_cancer_type WHERE is_active = 1 ORDER BY name ASC`)
	if err != nil {
		log.Printf("Failed to query cancer types: %v", err)
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "获取可选择检测癌种成功",
			Data:    []utils.H{},
		})
		return
	}
	defer rows.Close()

	// 筛选出Panel数量小于或等于当前检测类型的检测类型
	var selectableCancerTypes []utils.H
	for rows.Next() {
		var id int
		var name, description, panelIDsStr string

		if err := rows.Scan(&id, &name, &description, &panelIDsStr); err != nil {
			continue
		}

		// 计算Panel数量
		panelCount := 0
		if panelIDsStr != "" {
			panelIDs := strings.Split(panelIDsStr, ",")
			for _, panelID := range panelIDs {
				if strings.TrimSpace(panelID) != "" {
					panelCount++
				}
			}
		}

		// 只有Panel数量小于或等于当前检测类型的检测类型才能被选择
		if panelCount <= currentPanelCount {
			selectableCancerTypes = append(selectableCancerTypes, utils.H{
				"id":          id,
				"name":        name,
				"description": description,
				"panelCount":  panelCount,
			})
		}
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取可选择检测癌种成功",
		Data:    selectableCancerTypes,
	})
}

// 处理获取模型列表请求
func HandleListModels(c *app.RequestContext, db *sql.DB) {
	activeOnly := c.Query("activeOnly") == "1" || strings.EqualFold(c.Query("activeOnly"), "true")
	includeDeprecated := true
	if c.Query("includeDeprecated") == "0" || strings.EqualFold(c.Query("includeDeprecated"), "false") {
		includeDeprecated = false
	}

	// 尝试从缓存获取
	cacheKey := fmt.Sprintf("models:activeOnly=%t:includeDeprecated=%t", activeOnly, includeDeprecated)
	var cachedData []utils.H
	err := GetCache(cacheKey, &cachedData)
	if err == nil {
		// 缓存命中，直接返回
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "获取模型列表成功",
			Data:    cachedData,
		})
		return
	}

	geneSymbolMap := loadGeneSymbolMap(db)

	// 从数据库查询模型列表，关联setting_cancer_type表获取癌症类型名称
	query := `SELECT ms.id, ms.model_name as name, ms.description, ms.model_version as modelType, ms.is_active, ms.parameters, ms.version, ms.cancer_type_id, ct.name as cancerTypeName, ms.formula, ms.model_mode, COALESCE(ms.is_deprecated, 0) as is_deprecated, ms.deprecated_at
			FROM setting_model ms
			LEFT JOIN setting_cancer_type ct ON ms.cancer_type_id = ct.id`
	conditions := make([]string, 0, 2)
	if activeOnly {
		conditions = append(conditions, "ms.is_active = 1")
	}
	if !includeDeprecated {
		conditions = append(conditions, "COALESCE(ms.is_deprecated, 0) = 0")
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY COALESCE(ms.is_deprecated, 0) ASC, ms.model_name ASC"

	rows, err := db.Query(query)
	if err != nil {
		log.Printf("Failed to query models: %v", err)
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "获取模型列表成功",
			Data:    []utils.H{},
		})
		return
	}
	defer rows.Close()

	// 遍历查询结果
	var models []utils.H = []utils.H{} // 初始化为空切片，而不是nil切片，避免JSON序列化时变为null
	for rows.Next() {
		var id, isActive, cancerTypeId, isDeprecated int
		var name, description, modelType, parameters, version, cancerTypeName, formula, modelMode sql.NullString
		var deprecatedAt sql.NullTime

		err := rows.Scan(&id, &name, &description, &modelType, &isActive, &parameters, &version, &cancerTypeId, &cancerTypeName, &formula, &modelMode, &isDeprecated, &deprecatedAt)
		if err != nil {
			log.Printf("Failed to scan model: %v", err)
			continue
		}

		// 构建模型信息
		model := utils.H{
			"id":             id,
			"name":           name.String,
			"modelName":      name.String,
			"description":    description.String,
			"modelType":      modelType.String,
			"isActive":       isActive,
			"is_active":      isActive,
			"version":        version.String,
			"parameters":     parameters.String,
			"cancerTypeId":   cancerTypeId,
			"cancerTypeName": cancerTypeName.String,
			"isDeprecated":   isDeprecated,
			"is_deprecated":  isDeprecated,
		}

		var paramsMap map[string]interface{}
		if parameters.Valid && parameters.String != "" {
			_ = json.Unmarshal([]byte(parameters.String), &paramsMap)
		}

		selectedGeneIDs := parseSelectedGeneIDs(parameters.String)
		if len(selectedGeneIDs) > 0 {
			model["selectedGenes"] = selectedGeneIDs
		}

		// 添加公式相关字段
		if formula.Valid {
			model["formula"] = formula.String
		}
		if modelMode.Valid {
			model["modelMode"] = modelMode.String
		}
		if deprecatedAt.Valid {
			model["deprecatedAt"] = deprecatedAt.Time.Format("2006-01-02T15:04:05+08:00")
		}

		geneSymbols := extractModelGeneSymbols(parameters.String, formula.String, geneSymbolMap)
		model["geneSymbols"] = geneSymbols
		model["genes"] = strings.Join(geneSymbols, ",")
		model["geneCount"] = len(geneSymbols)
		model["selectableInBatch"] = isActive == 1 && isDeprecated == 0

		applicableItems := []string{}
		if rawItems, ok := paramsMap["applicableItems"].([]interface{}); ok {
			for _, rawItem := range rawItems {
				if item, ok := rawItem.(string); ok && strings.TrimSpace(item) != "" {
					applicableItems = append(applicableItems, strings.TrimSpace(item))
				}
			}
		}
		if len(applicableItems) == 0 && cancerTypeName.String != "" {
			applicableItems = []string{cancerTypeName.String}
		}
		model["applicableItems"] = uniqueSortedStrings(applicableItems)

		models = append(models, model)
	}

	// 检查遍历过程中是否有错误
	if err = rows.Err(); err != nil {
		log.Printf("Error iterating models: %v", err)
	}

	// 缓存结果
	SetCache(cacheKey, models, 24*time.Hour)

	// 返回模型列表，始终返回数组
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取模型列表成功",
		Data:    models,
	})
}

// 处理创建模型请求
func HandleCreateModel(c *app.RequestContext, db *sql.DB) {
	// 解析请求体
	var req struct {
		Name          string                 `json:"name" binding:"required"`
		Description   string                 `json:"description"`
		ModelType     string                 `json:"model_type" binding:"required"`
		CancerTypeId  int                    `json:"cancer_type_id"`
		Version       string                 `json:"version"`
		SelectedGenes []int                  `json:"selected_genes"`
		GeneWeights   map[string]interface{} `json:"gene_weights"` // 使用interface{}类型，以便处理字符串和数字类型的权重值
		Parameters    string                 `json:"parameters"`
		Formula       string                 `json:"formula"`
		ModelMode     string                 `json:"model_mode"`
		IsActive      int                    `json:"is_active"`
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

	// 处理parameters字段，确保它是有效的JSON
	parameters := req.Parameters
	if parameters == "" {
		parameters = "{}" // 空对象是有效的JSON
	}

	// 将SelectedGenes存储到parameters中
	if len(req.SelectedGenes) > 0 {
		// 解析现有的parameters
		var paramsMap map[string]interface{}
		if err := json.Unmarshal([]byte(parameters), &paramsMap); err != nil {
			paramsMap = make(map[string]interface{})
		}

		// 更新parameters
		paramsMap["selectedGenes"] = req.SelectedGenes

		// 重新编码为JSON
		if updatedParams, err := json.Marshal(paramsMap); err == nil {
			parameters = string(updatedParams)
		}
	}

	// 处理modelMode字段
	modelMode := req.ModelMode

	// 处理is_active字段，默认启用
	isActive := req.IsActive
	if isActive == 0 {
		isActive = 1
	}

	formula := normalizeFormulaText(req.Formula)

	// 插入模型到数据库
	result, err := db.Exec(`INSERT INTO setting_model (model_name, description, model_version, cancer_type_id, version, parameters, formula, model_mode, is_active, is_deprecated, deprecated_at, created_at, updated_at) 
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, NULL, NOW(), NOW())`,
		req.Name, req.Description, req.ModelType, req.CancerTypeId, req.Version, parameters, formula, modelMode, isActive)
	if err != nil {
		log.Printf("Failed to create model: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 获取插入的模型ID
	modelID, err := result.LastInsertId()
	if err != nil {
		log.Printf("Failed to get last insert ID: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 清理缓存
	ClearCache("models:*")

	// 返回创建的模型ID
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "创建模型成功",
		Data: utils.H{
			"id": modelID,
		},
	})
}

// 处理更新模型请求
func HandleUpdateModel(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	idParam := c.Param("id")
	var id int
	n, err := fmt.Sscanf(idParam, "%d", &id)
	if err != nil || n == 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的模型ID",
			Data:    nil,
		})
		return
	}

	// 解析请求体
	var req struct {
		Name          string                 `json:"name"`
		Description   string                 `json:"description"`
		ModelType     string                 `json:"model_type"`
		CancerTypeId  int                    `json:"cancer_type_id"`
		Version       string                 `json:"version"`
		SelectedGenes []int                  `json:"selected_genes"`
		GeneWeights   map[string]interface{} `json:"gene_weights"` // 使用interface{}类型，以便处理字符串和数字类型的权重值
		Parameters    string                 `json:"parameters"`
		IsActive      int                    `json:"is_active"`
		Formula       string                 `json:"formula"`
		ModelMode     string                 `json:"model_mode"`
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

	// 处理parameters字段，确保它是有效的JSON
	parameters := req.Parameters
	if parameters == "" {
		parameters = "{}" // 空对象是有效的JSON
	}

	// 将SelectedGenes存储到parameters中
	if len(req.SelectedGenes) > 0 {
		// 解析现有的parameters
		var paramsMap map[string]interface{}
		if err := json.Unmarshal([]byte(parameters), &paramsMap); err != nil {
			paramsMap = make(map[string]interface{})
		}

		// 更新parameters
		paramsMap["selectedGenes"] = req.SelectedGenes

		// 重新编码为JSON
		if updatedParams, err := json.Marshal(paramsMap); err == nil {
			parameters = string(updatedParams)
		}
	}

	// 处理modelMode字段
	modelMode := req.ModelMode

	formula := normalizeFormulaText(req.Formula)

	// 更新模型基本信息到数据库
	_, err = db.Exec(`UPDATE setting_model SET model_name = ?, description = ?, model_version = ?, cancer_type_id = ?, version = ?, parameters = ?, formula = ?, model_mode = ?, is_active = ?, updated_at = NOW() 
			WHERE id = ?`,
		req.Name, req.Description, req.ModelType, req.CancerTypeId, req.Version, parameters, formula, modelMode, req.IsActive, id)
	if err != nil {
		log.Printf("Failed to update model: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 清理缓存
	ClearCache("models:*")

	// 返回更新成功响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "更新模型成功",
		Data:    nil,
	})
}

// 处理删除模型请求
func HandleDeleteModel(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	idParam := c.Param("id")
	var id int
	_, err := fmt.Sscanf(idParam, "%d", &id)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的模型ID",
			Data:    nil,
		})
		return
	}

	// 改为弃用模型，保留历史报告中的模型引用
	_, err = db.Exec("UPDATE setting_model SET is_deprecated = 1, is_active = 0, deprecated_at = NOW(), updated_at = NOW() WHERE id = ?", id)
	if err != nil {
		log.Printf("Failed to delete model: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 清理缓存
	ClearCache("models:*")

	// 返回删除成功响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "模型已弃用",
		Data:    nil,
	})
}

// 处理获取模型基因阈值请求
func HandleGetModelGeneThresholds(c *app.RequestContext, db *sql.DB) {
	idParam := c.Param("id")
	modelID, err := strconv.Atoi(idParam)
	if err != nil || modelID <= 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的模型ID",
			Data:    nil,
		})
		return
	}

	rows, err := loadModelGeneThresholdRows(db, modelID)
	if err != nil {
		log.Printf("Failed to load model gene thresholds: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "获取模型基因阈值失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取模型基因阈值成功",
		Data:    rows,
	})
}

// 处理更新模型基因阈值请求
func HandleUpdateModelGeneThresholds(c *app.RequestContext, db *sql.DB) {
	idParam := c.Param("id")
	modelID, err := strconv.Atoi(idParam)
	if err != nil || modelID <= 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的模型ID",
			Data:    nil,
		})
		return
	}

	var req struct {
		Thresholds []struct {
			GeneID    int     `json:"geneId"`
			Threshold float64 `json:"threshold"`
		} `json:"thresholds"`
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

	if err := ensureModelGeneThresholdTable(db); err != nil {
		log.Printf("Failed to ensure model gene threshold table: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "保存模型基因阈值失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	tx, err := db.Begin()
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "保存模型基因阈值失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}
	defer tx.Rollback()

	for _, item := range req.Thresholds {
		if item.GeneID <= 0 {
			continue
		}
		if _, err := tx.Exec(`INSERT INTO setting_model_gene_threshold (model_id, gene_id, threshold, created_at, updated_at)
			VALUES (?, ?, ?, NOW(), NOW())
			ON DUPLICATE KEY UPDATE threshold = VALUES(threshold), updated_at = NOW()`,
			modelID, item.GeneID, item.Threshold); err != nil {
			log.Printf("Failed to upsert model gene threshold: %v", err)
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code:    500,
				Success: false,
				Message: "保存模型基因阈值失败",
				Data:    utils.H{"error": err.Error()},
			})
			return
		}
	}

	if err := tx.Commit(); err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "保存模型基因阈值失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	ClearCache("models:*")
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "保存模型基因阈值成功",
		Data:    nil,
	})
}

// 处理获取基因列表请求
func HandleListGenes(c *app.RequestContext, db *sql.DB) {
	activeOnly := c.Query("activeOnly") == "1" || c.Query("activeOnly") == "true"
	search := c.Query("search")
	skipCache := c.Query("skipCache") == "1" || c.Query("skipCache") == "true"
	var cacheKey string
	var err error

	// 尝试从缓存获取（搜索时或skipCache=true时不使用缓存）
	var cachedData []utils.H
	var useCache bool = false
	if search == "" && !skipCache {
		cacheKey = "genes"
		if activeOnly {
			cacheKey = "genes_active"
		}
		err = GetCache(cacheKey, &cachedData)
		if err == nil && cachedData != nil {
			useCache = true
		}
	}

	if useCache {
		// 缓存命中，直接返回
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "获取基因列表成功",
			Data:    cachedData,
		})
		return
	}

	// 从数据库查询基因列表
	query := `SELECT id, gene_name, gene_symbol, description, threshold 
			FROM setting_gene`

	// 处理搜索参数（search已在前面声明）
	if search != "" {
		query += " WHERE gene_name LIKE ? OR gene_symbol LIKE ?"
	}

	query += " ORDER BY gene_symbol ASC"

	var rows *sql.Rows
	if search != "" {
		rows, err = db.Query(query, "%"+search+"%", "%"+search+"%")
	} else {
		rows, err = db.Query(query)
	}
	if err != nil {
		log.Printf("Failed to query genes: %v", err)
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "获取基因列表成功",
			Data:    []utils.H{},
		})
		return
	}
	defer rows.Close()

	// 遍历查询结果
	var genes []utils.H
	for rows.Next() {
		var id int
		var geneName, geneSymbol, description string
		var threshold float64

		err := rows.Scan(&id, &geneName, &geneSymbol, &description, &threshold)
		if err != nil {
			log.Printf("Failed to scan gene: %v", err)
			continue
		}

		// 查询该基因关联的Panel（同时支持新的gene_ids字段和旧的关联表）
		var panels []utils.H

		// 先尝试从新的 gene_ids 字段查询（使用REPLACE去除可能的空格）
		panelRows, err := db.Query(`SELECT p.id, p.panel_name, p.panel_code 
				FROM setting_panel p
				WHERE p.is_active = 1 AND FIND_IN_SET(?, REPLACE(p.gene_ids, ' ', '')) > 0`, id)
		if err == nil {
			for panelRows.Next() {
				var panelID int
				var panelName, panelCode string
				if err := panelRows.Scan(&panelID, &panelName, &panelCode); err == nil {
					panels = append(panels, utils.H{
						"id":        panelID,
						"panelName": panelName,
						"panelCode": panelCode,
					})
				}
			}
			panelRows.Close()
		}

		// 构建基因信息
		gene := utils.H{
			"id":          id,
			"name":        geneName,
			"geneSymbol":  geneSymbol,
			"description": description,
			"threshold":   threshold,
			"panels":      panels,
		}

		genes = append(genes, gene)
	}

	// 检查遍历过程中是否有错误
	if err = rows.Err(); err != nil {
		log.Printf("Error iterating genes: %v", err)
	}

	// 缓存结果
	SetCache(cacheKey, genes, 24*time.Hour)

	// 返回基因列表，始终返回数组
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取基因列表成功",
		Data:    genes,
	})
}

// 处理创建基因请求
func HandleCreateGene(c *app.RequestContext, db *sql.DB) {
	// 解析请求体
	var req struct {
		Name        string  `json:"name" binding:"required"`
		GeneSymbol  string  `json:"geneSymbol" binding:"required"`
		Description string  `json:"description" binding:"required"`
		IsActive    int     `json:"isActive"`
		Threshold   float64 `json:"threshold"`
		PanelIDs    []int   `json:"panelIds"`
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

	// 如果没有提供isActive，则默认为1
	if req.IsActive == 0 {
		req.IsActive = 1
	}

	// 插入基因到数据库
	result, err := db.Exec(`INSERT INTO setting_gene (gene_name, gene_symbol, description, threshold, created_at, updated_at) 
				VALUES (?, ?, ?, ?, NOW(), NOW())`,
		req.Name, req.GeneSymbol, req.Description, req.Threshold)
	if err != nil {
		log.Printf("Failed to create gene: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 获取插入的基因ID
	id, err := result.LastInsertId()
	if err != nil {
		log.Printf("Failed to get last insert ID: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 清理缓存
	DeleteCache("genes")

	// 返回创建的基因ID
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "创建基因成功",
		Data: utils.H{
			"id": id,
		},
	})
}

// 处理更新基因请求
func HandleUpdateGene(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	idParam := c.Param("id")
	var id int
	_, err := fmt.Sscanf(idParam, "%d", &id)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的基因ID",
			Data:    nil,
		})
		return
	}

	// 解析请求体
	var req struct {
		Name        string  `json:"name"`
		GeneSymbol  string  `json:"geneSymbol"`
		Description string  `json:"description"`
		Threshold   float64 `json:"threshold"`
		IsActive    int     `json:"isActive"`
		PanelIDs    []int   `json:"panelIds"`
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

	// 获取现有数据
	var existingName, existingGeneSymbol, existingDescription string
	var existingThreshold float64
	var existingIsActive int
	err = db.QueryRow("SELECT gene_name, gene_symbol, description, threshold, is_active FROM setting_gene WHERE id = ?", id).Scan(&existingName, &existingGeneSymbol, &existingDescription, &existingThreshold, &existingIsActive)
	if err != nil {
		log.Printf("Failed to get existing gene: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 使用现有值或新值
	if req.Name == "" {
		req.Name = existingName
	}
	if req.GeneSymbol == "" {
		req.GeneSymbol = existingGeneSymbol
	}
	if req.Description == "" {
		req.Description = existingDescription
	}

	// 使用现有阈值或新阈值
	thresholdValue := req.Threshold
	if thresholdValue == 0 {
		thresholdValue = existingThreshold
	}

	// 更新基因信息到数据库（包括 is_active）
	log.Printf("is_active从%d变更为%d (id=%d)", existingIsActive, req.IsActive, id)
	_, err = db.Exec(`UPDATE setting_gene SET gene_name = ?, gene_symbol = ?, description = ?, threshold = ?, is_active = ?, updated_at = NOW() 
			WHERE id = ?`,
		req.Name, req.GeneSymbol, req.Description, thresholdValue, req.IsActive, id)
	if err != nil {
		log.Printf("Failed to update gene: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 清理缓存
	DeleteCache("genes")

	// 返回更新成功响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "更新基因成功",
		Data:    nil,
	})
}

// 处理更新单个基因Panel关系请求
func HandleUpdateGenePanels(c *app.RequestContext, db *sql.DB) {
	idParam := c.Param("id")
	geneID, err := strconv.Atoi(idParam)
	if err != nil || geneID <= 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的基因ID",
			Data:    nil,
		})
		return
	}

	var req struct {
		PanelIDs []int `json:"panelIds"`
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

	DeleteCache("genes")
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "更新基因Panel成功",
		Data:    nil,
	})
}

// 处理删除基因请求
func HandleDeleteGene(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	idParam := c.Param("id")
	var id int
	_, err := fmt.Sscanf(idParam, "%d", &id)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的基因ID",
			Data:    nil,
		})
		return
	}

	// 执行删除
	_, err = db.Exec("DELETE FROM setting_gene WHERE id = ?", id)
	if err != nil {
		log.Printf("Failed to delete gene: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 清理缓存
	DeleteCache("genes")

	// 返回删除成功响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "删除基因成功",
		Data:    nil,
	})
}

// 处理获取部门列表请求
func HandleListDepartments(c *app.RequestContext, db *sql.DB) {
	// 从数据库查询部门列表，使用status字段而不是is_active
	rows, err := db.Query(`SELECT id, name, parent_id, description, status, created_at, updated_at 
			FROM setting_department ORDER BY name ASC`)
	if err != nil {
		log.Printf("Failed to query setting_departments: %v", err)
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "获取部门列表成功",
			Data:    []utils.H{},
		})
		return
	}
	defer rows.Close()

	// 遍历查询结果，初始化为空数组而不是nil
	var setting_departments []utils.H = []utils.H{}
	for rows.Next() {
		var id, status int
		var parentId sql.NullInt32
		var name string
		var description sql.NullString
		var createdAt, updatedAt time.Time

		err := rows.Scan(&id, &name, &parentId, &description, &status, &createdAt, &updatedAt)
		if err != nil {
			log.Printf("Failed to scan setting_department: %v", err)
			continue
		}

		// 处理parentId，将sql.NullInt32转换为普通int
		parentIdValue := 0
		if parentId.Valid {
			parentIdValue = int(parentId.Int32)
		}

		// 处理description，将sql.NullString转换为普通string
		descriptionValue := ""
		if description.Valid {
			descriptionValue = description.String
		}

		// 构建部门信息，将status映射为isActive
		setting_department := utils.H{
			"id":          id,
			"name":        name,
			"parentId":    parentIdValue,
			"description": descriptionValue,
			"isActive":    status,
			"createdAt":   createdAt.Format("2006-01-02T15:04:05+08:00"),
			"updatedAt":   updatedAt.Format("2006-01-02T15:04:05+08:00"),
		}

		setting_departments = append(setting_departments, setting_department)
	}

	// 检查遍历过程中是否有错误
	if err = rows.Err(); err != nil {
		log.Printf("Error iterating setting_departments: %v", err)
	}

	// 返回部门列表，使用前端期望的格式
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取部门列表成功",
		Data: utils.H{
			"list":  setting_departments,
			"total": len(setting_departments),
		},
	})
}

// 处理获取部门树形结构请求
func HandleListDepartmentsTree(c *app.RequestContext, db *sql.DB) {
	// 从数据库查询部门列表，使用status字段而不是is_active
	rows, err := db.Query(`SELECT id, name, parent_id, description, status, created_at, updated_at 
			FROM setting_department ORDER BY name ASC`)
	if err != nil {
		log.Printf("Failed to query setting_departments: %v", err)
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "获取部门树形结构成功",
			Data:    []utils.H{},
		})
		return
	}
	defer rows.Close()

	// 遍历查询结果，初始化为空数组而不是nil
	var setting_departmentMap = make(map[int]utils.H)
	for rows.Next() {
		var id, status int
		var parentId sql.NullInt32
		var name string
		var description sql.NullString
		var createdAt, updatedAt time.Time

		err := rows.Scan(&id, &name, &parentId, &description, &status, &createdAt, &updatedAt)
		if err != nil {
			log.Printf("Failed to scan setting_department: %v", err)
			continue
		}

		// 处理parentId，将sql.NullInt32转换为普通int
		parentIdValue := 0
		if parentId.Valid {
			parentIdValue = int(parentId.Int32)
		}

		// 处理description，将sql.NullString转换为普通string
		descriptionValue := ""
		if description.Valid {
			descriptionValue = description.String
		}

		// 构建部门信息，将status映射为isActive
		setting_department := utils.H{
			"id":          id,
			"name":        name,
			"parentId":    parentIdValue,
			"description": descriptionValue,
			"isActive":    status,
			"createdAt":   createdAt.Format("2006-01-02T15:04:05+08:00"),
			"updatedAt":   updatedAt.Format("2006-01-02T15:04:05+08:00"),
			"children":    []utils.H{},
		}

		setting_departmentMap[id] = setting_department
	}

	// 检查遍历过程中是否有错误
	if err = rows.Err(); err != nil {
		log.Printf("Error iterating setting_departments: %v", err)
	}

	// 构建树形结构
	var rootDepartments []utils.H
	for _, dept := range setting_departmentMap {
		parentId := int(dept["parentId"].(int))
		if parentId == 0 {
			// 根部门
			rootDepartments = append(rootDepartments, dept)
		} else {
			// 子部门，添加到父部门的children数组中
			if parent, exists := setting_departmentMap[parentId]; exists {
				children := parent["children"].([]utils.H)
				children = append(children, dept)
				parent["children"] = children
				setting_departmentMap[parentId] = parent
			}
		}
	}

	// 返回部门树形结构，使用前端期望的格式
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取部门树形结构成功",
		Data:    rootDepartments,
	})
}

// 处理获取用户列表请求
func HandleListUsers(c *app.RequestContext, db *sql.DB) {
	// 从数据库查询用户列表，使用base_manage_user表而不是user表，使用status字段而不是is_active
	rows, err := db.Query(`SELECT id, username, real_name as name, phone, department_id, role_id, status, last_login_time, created_at, updated_at, employee_id, COALESCE(ai_allowed, 1) 
			FROM base_manage_user ORDER BY real_name ASC`)
	if err != nil {
		log.Printf("Failed to query users: %v", err)
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "获取用户列表成功",
			Data:    []utils.H{},
		})
		return
	}
	defer rows.Close()

	// 遍历查询结果，初始化为空数组而不是nil
	var users []utils.H = []utils.H{}
	for rows.Next() {
		var id, status, aiAllowed int
		var setting_departmentId, setting_roleId sql.NullInt32
		var username, name, phone, employeeId string
		var lastLoginTime sql.NullTime
		var createdAt, updatedAt time.Time

		err := rows.Scan(&id, &username, &name, &phone, &setting_departmentId, &setting_roleId, &status, &lastLoginTime, &createdAt, &updatedAt, &employeeId, &aiAllowed)
		if err != nil {
			log.Printf("Failed to scan user: %v", err)
			continue
		}

		// 处理setting_departmentId和setting_roleId，将sql.NullInt32转换为普通int
		setting_departmentIdValue := 0
		if setting_departmentId.Valid {
			setting_departmentIdValue = int(setting_departmentId.Int32)
		}

		setting_roleIdValue := 0
		if setting_roleId.Valid {
			setting_roleIdValue = int(setting_roleId.Int32)
		}

		// 处理last_login_time - 格式化为中文格式"2026年06月01日 22:22:22"
		lastLoginTimeStr := "从未登录"
		if lastLoginTime.Valid {
			lastLoginTimeStr = lastLoginTime.Time.Format("2006年01月02日 15:04:05")
		}

		// 查询部门名称
		departmentName := ""
		if setting_departmentIdValue > 0 {
			var deptName string
			err := db.QueryRow("SELECT name FROM setting_department WHERE id = ?", setting_departmentIdValue).Scan(&deptName)
			if err == nil {
				departmentName = deptName
			}
		}

		// 查询角色名称
		roleName := ""
		if setting_roleIdValue > 0 {
			var rName string
			err := db.QueryRow("SELECT name FROM setting_role WHERE id = ?", setting_roleIdValue).Scan(&rName)
			if err == nil {
				roleName = rName
			}
		}
		roles, roleIDs, roleNames := getUserRoles(db, id, setting_roleIdValue)
		if len(roleNames) > 0 {
			roleName = strings.Join(roleNames, "、")
		}

		// 检查用户是否绑定小程序(查询uni_patient表是否有对应的手机号)
		bindMiniProgram := "否"
		if phone != "" {
			var count int
			err := db.QueryRow("SELECT COUNT(*) FROM uni_patient WHERE phone = ?", phone).Scan(&count)
			if err == nil && count > 0 {
				bindMiniProgram = "是"
			}
		}

		// 构建用户信息，使用前端期望的字段名
		user := utils.H{
			"id":                id,
			"username":          username,
			"real_name":         name,
			"name":              name, // 同时返回name字段保持兼容性
			"email":             "",   // base_manage_user表中没有email字段，使用空字符串
			"phone":             phone,
			"department_id":     setting_departmentIdValue,
			"department_name":   departmentName, // 部门名称 - 使用正确的字段名
			"role_id":           setting_roleIdValue,
			"role_ids":          roleIDs,
			"role_name":         roleName, // 角色名称 - 使用正确的字段名
			"role_names":        roleNames,
			"roles":             roles,
			"status":            status, // 直接返回status字段，前端使用status判断
			"isActive":          status, // 保持isActive映射，兼容其他组件
			"createdAt":         createdAt.Format("2006-01-02T15:04:05+08:00"),
			"updatedAt":         updatedAt.Format("2006-01-02T15:04:05+08:00"),
			"last_login_time":   lastLoginTimeStr, // 格式化为中文格式
			"employee_id":       employeeId,       // 工号字段
			"bind_mini_program": bindMiniProgram,  // 是否绑定小程序
			"ai_allowed":        aiAllowed,        // AI 访问权限
		}

		users = append(users, user)
	}

	// 检查遍历过程中是否有错误
	if err = rows.Err(); err != nil {
		log.Printf("Error iterating users: %v", err)
	}

	// 返回用户列表，使用前端期望的格式
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取用户列表成功",
		Data: utils.H{
			"list":  users,
			"total": len(users),
		},
	})
}

// 处理获取角色列表请求
func HandleListRoles(c *app.RequestContext, db *sql.DB) {
	// 从数据库查询角色列表，使用status字段而不是is_active
	rows, err := db.Query(`SELECT id, name, description, status, created_at, updated_at 
			FROM setting_role ORDER BY name ASC`)
	if err != nil {
		log.Printf("Failed to query setting_roles: %v", err)
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "获取角色列表成功",
			Data:    []utils.H{},
		})
		return
	}
	defer rows.Close()

	// 遍历查询结果，初始化为空数组而不是nil
	var setting_roles []utils.H = []utils.H{}
	for rows.Next() {
		var id, status int
		var name, description string
		var createdAt, updatedAt time.Time

		err := rows.Scan(&id, &name, &description, &status, &createdAt, &updatedAt)
		if err != nil {
			log.Printf("Failed to scan setting_role: %v", err)
			continue
		}

		// 构建角色信息，将status映射为isActive
		setting_role := utils.H{
			"id":          id,
			"name":        name,
			"description": description,
			"isActive":    status,
			"createdAt":   createdAt.Format("2006-01-02T15:04:05+08:00"),
			"updatedAt":   updatedAt.Format("2006-01-02T15:04:05+08:00"),
		}

		setting_roles = append(setting_roles, setting_role)
	}

	// 检查遍历过程中是否有错误
	if err = rows.Err(); err != nil {
		log.Printf("Error iterating setting_roles: %v", err)
	}

	// 返回角色列表，使用前端期望的格式
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取角色列表成功",
		Data: utils.H{
			"list":  setting_roles,
			"total": len(setting_roles),
		},
	})
}

// 处理创建部门请求
func HandleCreateDepartment(c *app.RequestContext, db *sql.DB) {
	// 解析请求体
	var req struct {
		Name        string `json:"name" binding:"required"`
		ParentId    *int   `json:"parent_id"`
		Description string `json:"description"`
		Status      int    `json:"status"`
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

	// 处理parentId，将nil转换为NULL
	var parentIdValue interface{} = nil
	if req.ParentId != nil {
		parentIdValue = *req.ParentId
	}

	// 插入部门到数据库
	result, err := db.Exec(`INSERT INTO setting_department (name, parent_id, description, status, created_at, updated_at) 
				VALUES (?, ?, ?, ?, NOW(), NOW())`,
		req.Name, parentIdValue, req.Description, req.Status)
	if err != nil {
		log.Printf("Failed to create setting_department: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 获取插入的部门ID
	id, err := result.LastInsertId()
	if err != nil {
		log.Printf("Failed to get last insert ID: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 返回创建的部门ID
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "创建部门成功",
		Data: utils.H{
			"id": id,
		},
	})
}

// 处理更新部门请求
func HandleUpdateDepartment(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	idParam := c.Param("id")
	var id int
	_, err := fmt.Sscanf(idParam, "%d", &id)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的部门ID",
			Data:    nil,
		})
		return
	}

	// 解析请求体
	var req struct {
		Name        string `json:"name" binding:"required"`
		ParentId    *int   `json:"parent_id"`
		Description string `json:"description"`
		Status      int    `json:"status"`
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

	// 处理parentId，将nil转换为NULL
	var parentIdValue interface{} = nil
	if req.ParentId != nil {
		parentIdValue = *req.ParentId
	}

	// 更新部门信息到数据库
	_, err = db.Exec(`UPDATE setting_department SET name = ?, parent_id = ?, description = ?, status = ?, updated_at = NOW() 
			WHERE id = ?`,
		req.Name, parentIdValue, req.Description, req.Status, id)
	if err != nil {
		log.Printf("Failed to update setting_department: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}
	// 返回更新成功响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "更新部门成功",
		Data:    nil,
	})
}

// 处理删除部门请求
func HandleDeleteDepartment(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	idParam := c.Param("id")
	var id int
	_, err := fmt.Sscanf(idParam, "%d", &id)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的部门ID",
			Data:    nil,
		})
		return
	}

	// 删除部门
	_, err = db.Exec("DELETE FROM setting_department WHERE id = ?", id)
	if err != nil {
		log.Printf("Failed to delete setting_department: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 返回删除成功响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "删除部门成功",
		Data:    nil,
	})
}

// 处理创建角色请求
func HandleCreateRole(c *app.RequestContext, db *sql.DB) {
	// 解析请求体
	var req struct {
		Name        string        `json:"name" binding:"required"`
		Description string        `json:"description"`
		Status      int           `json:"status"`
		Permissions []interface{} `json:"permissions"`
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

	// 插入角色到数据库
	result, err := db.Exec(`INSERT INTO setting_role (name, description, status, created_at, updated_at) 
			VALUES (?, ?, ?, NOW(), NOW())`,
		req.Name, req.Description, req.Status)
	if err != nil {
		log.Printf("Failed to create setting_role: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 获取插入的角色ID
	id, err := result.LastInsertId()
	if err != nil {
		log.Printf("Failed to get last insert ID: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 处理页面权限数据
	if len(req.Permissions) > 0 {
		tx, err := db.Begin()
		if err != nil {
			log.Printf("Failed to begin transaction: %v", err)
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code:    500,
				Success: false,
				Message: "服务器内部错误",
				Data:    utils.H{"error": err.Error()},
			})
			return
		}

		// 递归处理权限树
		var processPermissions func(permissions []interface{}, parentPageID string)
		processPermissions = func(permissions []interface{}, parentPageID string) {
			for _, p := range permissions {
				if perm, ok := p.(map[string]interface{}); ok {
					pageID := perm["id"].(string)
					// 尝试从title获取pageName，如果不存在则使用id
					pageName := pageID
					if title, ok := perm["title"].(string); ok {
						pageName = title
					}
					checked := true
					if c, ok := perm["checked"].(bool); ok {
						checked = c
					}

					// 插入权限记录
					_, err := tx.Exec(`INSERT INTO setting_role_permission (role_id, page_id, page_name, parent_page_id, checked, created_at, updated_at) 
							VALUES (?, ?, ?, ?, ?, NOW(), NOW())`,
						id, pageID, pageName, parentPageID, checked)
					if err != nil {
						log.Printf("Failed to insert setting_role permission: %v", err)
						return
					}

					// 处理子权限
					if children, ok := perm["children"].([]interface{}); ok && len(children) > 0 {
						processPermissions(children, pageID)
					}
				}
			}
		}

		// 处理权限数据
		processPermissions(req.Permissions, "")

		// 提交事务
		if err := tx.Commit(); err != nil {
			log.Printf("Failed to commit transaction: %v", err)
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code:    500,
				Success: false,
				Message: "服务器内部错误",
				Data:    utils.H{"error": err.Error()},
			})
			return
		}
	}

	// 返回创建的角色ID
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "创建角色成功",
		Data: utils.H{
			"id": id,
		},
	})
}

// 处理更新角色请求
func HandleUpdateRole(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	idParam := c.Param("id")
	var id int
	_, err := fmt.Sscanf(idParam, "%d", &id)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的角色ID",
			Data:    nil,
		})
		return
	}

	// 解析请求体
	var req struct {
		Name        string        `json:"name" binding:"required"`
		Description string        `json:"description"`
		Status      int           `json:"status"`
		Permissions []interface{} `json:"permissions"`
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

	// 更新角色信息到数据库
	_, err = db.Exec(`UPDATE setting_role SET name = ?, description = ?, status = ?, updated_at = NOW() 
			WHERE id = ?`,
		req.Name, req.Description, req.Status, id)
	if err != nil {
		log.Printf("Failed to update setting_role: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 处理页面权限数据
	if len(req.Permissions) > 0 {
		// 开始事务
		tx, err := db.Begin()
		if err != nil {
			log.Printf("Failed to begin transaction: %v", err)
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code:    500,
				Success: false,
				Message: "服务器内部错误",
				Data:    utils.H{"error": err.Error()},
			})
			return
		}

		// 删除旧的权限记录
		_, err = tx.Exec("DELETE FROM setting_role_permission WHERE role_id = ?", id)
		if err != nil {
			log.Printf("Failed to delete old setting_role permissions: %v", err)
			tx.Rollback()
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code:    500,
				Success: false,
				Message: "服务器内部错误",
				Data:    utils.H{"error": err.Error()},
			})
			return
		}

		// 递归处理权限树
		var processPermissions func(permissions []interface{}, parentPageID string)
		processPermissions = func(permissions []interface{}, parentPageID string) {
			for _, p := range permissions {
				if perm, ok := p.(map[string]interface{}); ok {
					pageID := perm["id"].(string)
					// 尝试从title获取pageName，如果不存在则使用id
					pageName := pageID
					if title, ok := perm["title"].(string); ok {
						pageName = title
					}
					checked := true
					if c, ok := perm["checked"].(bool); ok {
						checked = c
					}

					// 插入权限记录
					_, err := tx.Exec(`INSERT INTO setting_role_permission (role_id, page_id, page_name, parent_page_id, checked, created_at, updated_at) 
							VALUES (?, ?, ?, ?, ?, NOW(), NOW())`,
						id, pageID, pageName, parentPageID, checked)
					if err != nil {
						log.Printf("Failed to insert setting_role permission: %v", err)
						return
					}

					// 处理子权限
					if children, ok := perm["children"].([]interface{}); ok && len(children) > 0 {
						processPermissions(children, pageID)
					}
				}
			}
		}

		// 处理权限数据
		processPermissions(req.Permissions, "")

		// 提交事务
		if err := tx.Commit(); err != nil {
			log.Printf("Failed to commit transaction: %v", err)
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code:    500,
				Success: false,
				Message: "服务器内部错误",
				Data:    utils.H{"error": err.Error()},
			})
			return
		}
	}

	// 返回更新成功响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "更新角色成功",
		Data:    nil,
	})
}

// 处理删除角色请求
func HandleDeleteRole(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	idParam := c.Param("id")
	var id int
	_, err := fmt.Sscanf(idParam, "%d", &id)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的角色ID",
			Data:    nil,
		})
		return
	}

	// 删除角色
	_, err = db.Exec("DELETE FROM setting_role WHERE id = ?", id)
	if err != nil {
		log.Printf("Failed to delete setting_role: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 返回删除成功响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "删除角色成功",
		Data:    nil,
	})
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func simpleNamePinyin(name string) string {
	pinyinMap := map[rune]string{
		'赵': "zhao", '辉': "hui", '史': "shi", '策': "ce", '航': "hang", '迟': "chi", '宝': "bao", '华': "hua",
		'纯': "chun", '玉': "yu", '田': "tian", '晓': "xiao", '雨': "yu", '王': "wang", '猛': "meng",
		'毛': "mao", '欣': "xin", '李': "li", '忠': "zhong", '琳': "lin", '于': "yu", '雁': "yan",
		'柏': "bai", '立': "li", '婧': "jing", '煌': "huang", '车': "che", '敬': "jing", '昌': "chang",
		'朱': "zhu", '亮': "liang", '陈': "chen", '晨': "chen", '刘': "liu", '宇': "yu", '桐': "tong",
		'哈': "ha", '尔': "er", '滨': "bin", '实': "shi", '验': "yan", '室': "shi", '姚': "yao",
		'喻': "yu", '翔': "xiang",
	}
	var builder strings.Builder
	for _, r := range strings.TrimSpace(name) {
		if value, ok := pinyinMap[r]; ok {
			builder.WriteString(value)
			continue
		}
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
		}
	}
	return strings.ToLower(builder.String())
}

func defaultUserPassword(realName string) string {
	pinyin := simpleNamePinyin(realName)
	if pinyin == "" {
		return "Hw123456"
	}
	return "Hw123456" + pinyin
}

type defaultStaffUser struct {
	EmployeeID string
	Name       string
	RoleName   string
	Phone      string
}

var defaultStaffUsers = []defaultStaffUser{
	{EmployeeID: "23001", Name: "赵辉", RoleName: "管理员", Phone: "15846390499"},
	{EmployeeID: "23002", Name: "史策", RoleName: "销售"},
	{EmployeeID: "23003", Name: "史航", RoleName: "销售"},
	{EmployeeID: "23004", Name: "迟宝华", RoleName: "销售"},
	{EmployeeID: "23005", Name: "赵纯玉", RoleName: "销售"},
	{EmployeeID: "23006", Name: "田晓雨", RoleName: "销售"},
	{EmployeeID: "23007", Name: "王猛", RoleName: "销售"},
	{EmployeeID: "23008", Name: "毛欣欣", RoleName: "销售"},
	{EmployeeID: "11001", Name: "李忠琳", RoleName: "销售"},
	{EmployeeID: "23010", Name: "于宝雁", RoleName: "销售"},
	{EmployeeID: "23011", Name: "毛欣欣", RoleName: "销售"},
	{EmployeeID: "23012", Name: "柏立婧", RoleName: "销售"},
	{EmployeeID: "23013", Name: "王煊", RoleName: "销售"},
	{EmployeeID: "21001", Name: "车敬昌", RoleName: "销售"},
	{EmployeeID: "65001", Name: "朱亮", RoleName: "销售"},
	{EmployeeID: "S00101", Name: "陈晨", RoleName: "管理员"},
	{EmployeeID: "S00102", Name: "刘宇桐", RoleName: "检验"},
	{EmployeeID: "S001", Name: "哈尔滨实验室", RoleName: "实验室账号"},
	{EmployeeID: "IT001", Name: "姚喻翔", RoleName: "IT管理员", Phone: "16601122566"},
}

type defaultPermission struct {
	ID     string
	Name   string
	Parent string
}

var rolePermissionSeeds = map[string][]defaultPermission{
	"管理员": {
		{"dashboard", "首页", ""}, {"patient", "患者中心", ""}, {"sample", "样本中心", ""},
		{"result", "结果中心", ""}, {"report", "报告中心", ""}, {"sales", "销售中心", ""},
		{"system", "系统设置", ""}, {"users", "用户管理", ""},
		{"announcement", "公告管理", ""}, {"ai-management", "AI管理", ""},
	},
	"IT管理员": {
		{"dashboard", "首页", ""}, {"patient", "患者中心", ""}, {"sample", "样本中心", ""},
		{"result", "结果中心", ""}, {"report", "报告中心", ""}, {"sales", "销售中心", ""},
		{"system", "系统设置", ""}, {"users", "用户管理", ""},
		{"announcement", "公告管理", ""}, {"ai-management", "AI管理", ""},
	},
	"销售": {
		{"dashboard", "首页", ""}, {"patient", "患者中心", ""}, {"sales", "销售中心", ""}, {"report", "报告中心", ""}, {"appointment", "物流中心", ""},
	},
	"检验": {
		{"dashboard", "首页", ""}, {"patient", "患者中心", ""}, {"sample", "样本中心", ""}, {"result", "结果中心", ""}, {"report", "报告中心", ""},
	},
	"实验室账号": {
		{"dashboard", "首页", ""}, {"patient", "患者中心", ""}, {"sample", "样本中心", ""}, {"result", "结果中心", ""}, {"report", "报告中心", ""},
	},
}

func ensureDefaultRole(db *sql.DB, name string) (int, error) {
	var id int
	if err := db.QueryRow("SELECT id FROM setting_role WHERE name = ? LIMIT 1", name).Scan(&id); err == nil {
		return id, nil
	} else if err != sql.ErrNoRows {
		return 0, err
	}
	result, err := db.Exec(`INSERT INTO setting_role (name, description, status, created_at, updated_at)
		VALUES (?, ?, 1, NOW(), NOW())`, name, name+"默认权限")
	if err != nil {
		return 0, err
	}
	lastID, err := result.LastInsertId()
	return int(lastID), err
}

func ensureDefaultRolePermissions(db *sql.DB, roleID int, roleName string) error {
	var existing int
	if err := db.QueryRow("SELECT COUNT(*) FROM setting_role_permission WHERE role_id = ?", roleID).Scan(&existing); err != nil {
		return err
	}
	if existing > 0 {
		return nil
	}
	for _, perm := range rolePermissionSeeds[roleName] {
		if _, err := db.Exec(`INSERT INTO setting_role_permission
			(role_id, page_id, page_name, parent_page_id, checked, created_at, updated_at)
			VALUES (?, ?, ?, ?, 1, NOW(), NOW())`, roleID, perm.ID, perm.Name, perm.Parent); err != nil {
			return err
		}
	}
	return nil
}

func ensureDefaultStaffUser(db *sql.DB, staff defaultStaffUser, roleID int) error {
	username := staff.EmployeeID
	var existingID int
	var existingPhone string
	err := db.QueryRow(`SELECT id, phone FROM base_manage_user
		WHERE employee_id = ? OR username = ? LIMIT 1`, staff.EmployeeID, username).Scan(&existingID, &existingPhone)
	if err == nil {
		phone := strings.TrimSpace(staff.Phone)
		if phone == "" && strings.TrimSpace(existingPhone) != "" && staff.EmployeeID != "S001" {
			phone = existingPhone
		}
		_, err = db.Exec(`UPDATE base_manage_user SET username = ?, real_name = ?, phone = ?, role_id = ?,
			status = 1, employee_id = ?, updated_at = NOW() WHERE id = ?`,
			username, staff.Name, phone, roleID, staff.EmployeeID, existingID)
		if err != nil {
			return err
		}
		tx, txErr := db.Begin()
		if txErr != nil {
			return txErr
		}
		defer tx.Rollback()
		if err := syncUserRoles(tx, existingID, roleID, []int{roleID}); err != nil {
			return err
		}
		return tx.Commit()
	}
	if err != sql.ErrNoRows {
		return err
	}
	salt := generateSalt(16)
	passwordMd5 := md5Hash(defaultUserPassword(staff.Name))
	hashedPassword := md5HashWithSalt(passwordMd5, salt)
	_, err = db.Exec(`INSERT INTO base_manage_user
		(username, password, salt, real_name, phone, department_id, role_id, status, created_at, updated_at, employee_id)
		VALUES (?, ?, ?, ?, ?, NULL, ?, 1, NOW(), NOW(), ?)`,
		username, hashedPassword, salt, staff.Name, strings.TrimSpace(staff.Phone), roleID, staff.EmployeeID)
	if err != nil {
		return err
	}
	if err := db.QueryRow(`SELECT id FROM base_manage_user WHERE employee_id = ? OR username = ? LIMIT 1`, staff.EmployeeID, username).Scan(&existingID); err != nil {
		return err
	}
	tx, txErr := db.Begin()
	if txErr != nil {
		return txErr
	}
	defer tx.Rollback()
	if err := syncUserRoles(tx, existingID, roleID, []int{roleID}); err != nil {
		return err
	}
	return tx.Commit()
}

// EnsureDefaultStaff 校准默认角色、权限和员工账号。不会重置已有账号密码。
func EnsureDefaultStaff(db *sql.DB) {
	if db == nil {
		return
	}
	if _, err := db.Exec("DELETE FROM setting_role_permission WHERE page_id = 'certificate'"); err != nil {
		log.Printf("Remove certificate role permission error: %v", err)
	}
	if _, err := db.Exec("DELETE FROM base_manage_user_permission WHERE page_id = 'certificate'"); err != nil {
		log.Printf("Remove certificate user permission error: %v", err)
	}
	roleIDs := map[string]int{}
	for roleName := range rolePermissionSeeds {
		roleID, err := ensureDefaultRole(db, roleName)
		if err != nil {
			log.Printf("Ensure default role %s error: %v", roleName, err)
			continue
		}
		roleIDs[roleName] = roleID
		if err := ensureDefaultRolePermissions(db, roleID, roleName); err != nil {
			log.Printf("Ensure default role permissions %s error: %v", roleName, err)
		}
	}
	for _, staff := range defaultStaffUsers {
		roleID := roleIDs[staff.RoleName]
		if roleID == 0 {
			continue
		}
		if err := ensureDefaultStaffUser(db, staff, roleID); err != nil {
			log.Printf("Ensure default staff %s/%s error: %v", staff.EmployeeID, staff.Name, err)
		}
	}
}

func ensureSystemSettingDefaults(db *sql.DB) {
	defaults := []struct {
		Key         string
		Value       string
		Type        string
		IsEncrypted int
		Description string
	}{
		{"AI_API_KEY", os.Getenv("AI_API_KEY"), "password", 1, "百度千帆 AI API Key"},
		{"AI_API_URL", firstNonEmptyString(os.Getenv("AI_API_URL"), "https://qianfan.baidubce.com/v2"), "text", 0, "百度千帆 OpenAI 兼容接口 Base URL"},
		{"AI_MODEL", firstNonEmptyString(os.Getenv("AI_MODEL"), "ernie-lite-pro-128k"), "text", 0, "AI 客服文本模型"},
		{"AI_PROMPT", os.Getenv("AI_PROMPT"), "textarea", 0, "AI 客服系统提示词"},
		{"AI_REPORT_VISION_MODEL", firstNonEmptyString(os.Getenv("AI_REPORT_VISION_MODEL"), "ernie-4.5-turbo-vl-32k"), "text", 0, "图片报告视觉分析模型"},
		{"AI_REPORT_TEXT_MODEL", firstNonEmptyString(os.Getenv("AI_REPORT_TEXT_MODEL"), "ernie-lite-pro-128k"), "text", 0, "PDF 报告文本分析模型"},
		{"AI_REPORT_PROMPT", firstNonEmptyString(os.Getenv("AI_REPORT_PROMPT"), defaultReportAnalysisPrompt), "textarea", 0, "患者上传报告分析提示词"},
		{"EXPRESS_QUERY_ENABLED", firstNonEmptyString(os.Getenv("EXPRESS_QUERY_ENABLED"), "1"), "switch", 0, "启用百度快递查询"},
		{"EXPRESS_API_URL", firstNonEmptyString(os.Getenv("EXPRESS_API_URL"), "https://jisuexpress.api.bdymkt.com/express/query"), "text", 0, "快递查询 API 地址"},
		{"EXPRESS_AUTH_MODE", firstNonEmptyString(os.Getenv("EXPRESS_AUTH_MODE"), "appcode"), "text", 0, "鉴权方式：appcode（云市场）或 app_v1（AppKey/AppSecret 签名）"},
		{"EXPRESS_APP_KEY", os.Getenv("EXPRESS_APP_KEY"), "password", 1, "百度快递 AppCode 或 AppKey"},
		{"EXPRESS_APP_SECRET", os.Getenv("EXPRESS_APP_SECRET"), "password", 1, "普通 APP V1 鉴权的 AppSecret"},
		{"EXPRESS_POLL_INTERVAL_MINUTES", firstNonEmptyString(os.Getenv("EXPRESS_POLL_INTERVAL_MINUTES"), "60"), "number", 0, "未签收运单自动刷新间隔（最短60分钟）"},
		{"SMS_BAIDU_ACCESS_KEY", os.Getenv("SMS_BAIDU_ACCESS_KEY"), "text", 0, "百度智能云 SMS Access Key"},
		{"SMS_BAIDU_SECRET_KEY", os.Getenv("SMS_BAIDU_SECRET_KEY"), "password", 1, "百度智能云 SMS Secret Key"},
		{"SMS_BAIDU_ENDPOINT", firstNonEmptyString(os.Getenv("SMS_BAIDU_ENDPOINT"), "http://sms.bj.baidubce.com"), "text", 0, "百度智能云 SMS 北京区域服务域名"},
		{"SMS_BAIDU_CERTIFICATE_ID", firstNonEmptyString(os.Getenv("SMS_BAIDU_CERTIFICATE_ID"), "sms-cert-RtzbqZ63190"), "text", 0, "百度短信资质 ID"},
		{"SMS_BAIDU_SIGNATURE_ID", firstNonEmptyString(os.Getenv("SMS_BAIDU_SIGNATURE_ID"), "sms-sign-ShdJQl83240"), "text", 0, "百度短信签名 ID"},
		{"SMS_BAIDU_SIGNATURE_CONTENT", firstNonEmptyString(os.Getenv("SMS_BAIDU_SIGNATURE_CONTENT"), "华微智检"), "text", 0, "百度短信签名内容"},
		{"SMS_BAIDU_LOGIN_TEMPLATE_ID", firstNonEmptyString(os.Getenv("SMS_BAIDU_LOGIN_TEMPLATE_ID"), "sms-tmpl-BhdTpq84685"), "text", 0, "百度短信登录验证码模板 ID"},
		{"SMS_BAIDU_REPORT_TEMPLATE_ID", firstNonEmptyString(os.Getenv("SMS_BAIDU_REPORT_TEMPLATE_ID"), "sms-tmpl-iVHgkc58950"), "text", 0, "百度短信报告出具通知模板 ID"},
		{"SMS_BAIDU_USER_ID", os.Getenv("SMS_BAIDU_USER_ID"), "text", 0, "百度智能云用户 ID，用于查询短信量包"},
		{"SMS_BAIDU_TEMPLATE_IDS", firstNonEmptyString(os.Getenv("SMS_BAIDU_TEMPLATE_IDS"), "sms-tmpl-BhdTpq84685,sms-tmpl-iVHgkc58950"), "text", 0, "百度短信候选模板 ID，多个用逗号分隔"},
		{"SMS_ADMIN_LOGIN_ENABLED", "1", "switch", 0, "管理后台登录验证码短信"},
		{"SMS_MINIAPP_LOGIN_ENABLED", "1", "switch", 0, "小程序登录验证码短信"},
		{"SMS_ADMIN_BIND_PHONE_ENABLED", "1", "switch", 0, "管理后台绑定手机短信"},
		{"SMS_MINIAPP_BIND_PHONE_ENABLED", "1", "switch", 0, "小程序绑定手机短信"},
		{"SMS_INVITE_REGISTER_ENABLED", "1", "switch", 0, "邀请注册验证码短信"},
		{"SMS_REPORT_READY_ENABLED", "1", "switch", 0, "报告出具通知短信"},
		{"QINIU_ENABLED", firstNonEmptyString(os.Getenv("QINIU_ENABLED"), "1"), "switch", 0, "启用七牛云对象存储"},
		{"QINIU_ACCESS_KEY", os.Getenv("QINIU_ACCESS_KEY"), "text", 0, "七牛云 AccessKey"},
		{"QINIU_SECRET_KEY", os.Getenv("QINIU_SECRET_KEY"), "password", 1, "七牛云 SecretKey"},
		{"QINIU_BUCKET", firstNonEmptyString(os.Getenv("QINIU_BUCKET"), "bucket01-bgpt-huaweibio-com-cn"), "text", 0, "七牛云存储空间名称"},
		{"QINIU_DOMAIN", firstNonEmptyString(os.Getenv("QINIU_DOMAIN"), "https://bucket01.huaweibio.com.cn"), "text", 0, "七牛云自定义访问域名"},
		{"QINIU_UPLOAD_URL", firstNonEmptyString(os.Getenv("QINIU_UPLOAD_URL"), "https://upload.qiniup.com"), "text", 0, "七牛云表单上传地址"},
		{"QINIU_TOKEN_TTL_SECONDS", firstNonEmptyString(os.Getenv("QINIU_TOKEN_TTL_SECONDS"), "3600"), "number", 0, "七牛云上传凭证有效期（秒）"},
		{"WECHAT_APP_ID", firstNonEmptyString(os.Getenv("WECHAT_APP_ID"), "wxac666c112df0f8f9"), "text", 0, "微信小程序 AppID"},
		{"WECHAT_APP_SECRET", os.Getenv("WECHAT_APP_SECRET"), "password", 1, "微信小程序 AppSecret，用于获取 access_token 和手机号"},
		{"WECHAT_QRCODE_ENV_VERSION", os.Getenv("WECHAT_QRCODE_ENV_VERSION"), "text", 0, "小程序码环境版本：release、trial 或 develop"},
		{"WECHAT_CERT_PATH", getCartPath("开放平台证书.cer"), "text", 0, "微信开放平台证书路径"},
		{"WECHAT_RSA_PRIVATE_KEY_PATH", getCartPath("RSA PRIVATE KEY.txt"), "text", 0, "RSA 私钥文件路径"},
		{"WECHAT_RSA_PUBLIC_KEY_PATH", getCartPath("PUBLIC KEY.txt"), "text", 0, "RSA 公钥文件路径"},
		{"WECHAT_AES_KEY_PATH", getCartPath("对称密钥.txt"), "text", 0, "对称密钥文件路径"},
		{"MINIAPP_HELP_CENTER_JSON", `{"categories":[{"name":"报告查看","items":[{"question":"什么时候可以查看报告？","answer":"样本完成检测并通过审核后，可在小程序“查看结果”中查看和下载报告。"},{"question":"为什么看不到待审核报告？","answer":"待审核报告属于内部处理状态，审核完成前不会展示给用户。"}]},{"name":"样本服务","items":[{"question":"如何邮寄样本？","answer":"进入“样本邮寄”，填写寄件人信息和快递单号后提交。"},{"question":"如何查询样本进度？","answer":"进入“进度查询”，可查看所有样本从创建到出报告的时间线。"}]}]}`, "textarea", 0, "小程序帮助中心 JSON，支持按 categories 分类配置常见问题"},
	}
	removeLegacySettings := []string{
		"SMS_ALIYUN_ACCESS_KEY",
		"SMS_ALIYUN_ACCESS_SECRET",
		"SMS_ALIYUN_SIGN_NAME",
		"SMS_TENCENT_SECRET_ID",
		"SMS_TENCENT_SECRET_KEY",
		"SMS_TENCENT_SIGN_NAME",
		"SMS_BAIDU_CERTIFICATE_NAME",
		"SMS_BAIDU_CERTIFICATE_USE",
		"SMS_BAIDU_AGENT_NAME",
		"SMS_BAIDU_AGENT_PHONE",
		"SMS_BAIDU_CERTIFICATE_STATUS",
		"SMS_BAIDU_CERTIFICATE_UPDATED_AT",
	}
	for _, key := range removeLegacySettings {
		if _, err := db.Exec("DELETE FROM setting_system WHERE key_name = ?", key); err != nil {
			log.Printf("Failed to remove legacy system setting %s: %v", key, err)
		}
	}
	for _, item := range defaults {
		value := item.Value
		if item.IsEncrypted == 1 && value != "" {
			value = encryptConfigValue(value)
		}
		_, err := db.Exec(`INSERT INTO setting_system (key_name, key_value, key_type, is_encrypted, description, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, NOW(), NOW())
			ON DUPLICATE KEY UPDATE key_name = key_name`,
			item.Key, value, item.Type, item.IsEncrypted, item.Description)
		if err != nil {
			log.Printf("Failed to ensure system setting %s: %v", item.Key, err)
		}
		if (item.Key == "EXPRESS_APP_KEY" || item.Key == "EXPRESS_APP_SECRET") && item.Value != "" {
			if _, updateErr := db.Exec(`UPDATE setting_system SET key_value = ?, key_type = ?, is_encrypted = ?,
				description = ?, updated_at = NOW() WHERE key_name = ? AND COALESCE(TRIM(key_value), '') = ''`,
				value, item.Type, item.IsEncrypted, item.Description, item.Key); updateErr != nil {
				log.Printf("Initialize empty express setting %s error: %v", item.Key, updateErr)
			}
		}
	}
}

// 处理获取系统配置请求
func HandleGetSystemSettings(c *app.RequestContext, db *sql.DB) {
	ensureSystemSettingDefaults(db)
	// 从数据库查询系统配置
	rows, err := db.Query(`SELECT key_name, key_value, key_type, is_encrypted, description 
			FROM setting_system ORDER BY key_name ASC`)
	if err != nil {
		log.Printf("Failed to query system settings: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}
	defer rows.Close()

	// 遍历查询结果
	var settings []utils.H
	for rows.Next() {
		var keyName, keyValue, keyType, description string
		var isEncrypted int

		err := rows.Scan(&keyName, &keyValue, &keyType, &isEncrypted, &description)
		if err != nil {
			log.Printf("Failed to scan system setting: %v", err)
			continue
		}

		// 如果是加密字段，需要解密
		if isEncrypted == 1 {
			// 解密值
			keyValue = decryptConfigValue(keyValue)
		}
		if keyName == "AI_API_KEY" {
			keyValue = maskAPIKey(keyValue)
		}

		// 构建配置信息
		setting := utils.H{
			"key_name":     keyName,
			"key_value":    keyValue,
			"key_type":     keyType,
			"is_encrypted": isEncrypted,
			"description":  description,
		}

		settings = append(settings, setting)
	}

	// 检查遍历过程中是否有错误
	if err = rows.Err(); err != nil {
		log.Printf("Error iterating system settings: %v", err)
	}

	// 返回系统配置列表
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取系统配置成功",
		Data:    settings,
	})
}

// 处理更新系统配置请求
func HandleUpdateSystemSettings(c *app.RequestContext, db *sql.DB) {
	// 解析请求体
	var req struct {
		Settings []struct {
			KeyName     string `json:"key_name" binding:"required"`
			KeyValue    string `json:"key_value"`
			IsEncrypted int    `json:"is_encrypted"`
		}
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

	// 开始事务
	tx, err := db.Begin()
	if err != nil {
		log.Printf("Failed to begin transaction: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 遍历更新配置
	for _, setting := range req.Settings {
		// 如果是加密字段，需要加密
		value := setting.KeyValue
		if setting.KeyName == "AI_API_KEY" && strings.Contains(value, "******") {
			var existing string
			if err := tx.QueryRow(`SELECT key_value FROM setting_system WHERE key_name = ?`, setting.KeyName).Scan(&existing); err == nil {
				value = decryptConfigValue(existing)
			}
		}
		if setting.IsEncrypted == 1 {
			// 加密值
			value = encryptConfigValue(value)
		}

		// 更新配置
		_, err := tx.Exec(`INSERT INTO setting_system (key_name, key_value, key_type, is_encrypted, description, created_at, updated_at)
			VALUES (?, ?, 'text', ?, '', NOW(), NOW())
			ON DUPLICATE KEY UPDATE key_value = VALUES(key_value), is_encrypted = VALUES(is_encrypted), updated_at = NOW()`,
			setting.KeyName, value, setting.IsEncrypted)
		if err != nil {
			log.Printf("Failed to update system setting: %v", err)
			tx.Rollback()
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code:    500,
				Success: false,
				Message: "服务器内部错误",
				Data:    utils.H{"error": err.Error()},
			})
			return
		}
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		log.Printf("Failed to commit transaction: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}
	ReloadAISettings(db)

	// 返回更新成功响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "更新系统配置成功",
		Data:    nil,
	})
}

func defaultHelpCenterJSON() string {
	return `{"categories":[{"name":"报告查看","items":[{"question":"什么时候可以查看报告？","answer":"样本完成检测并通过审核后，可在小程序“查看结果”中查看和下载报告。"},{"question":"为什么看不到待审核报告？","answer":"待审核报告属于内部处理状态，审核完成前不会展示给用户。"}]},{"name":"样本服务","items":[{"question":"如何邮寄样本？","answer":"进入“样本邮寄”，填写寄件人信息和快递单号后提交。"},{"question":"如何查询样本进度？","answer":"进入“进度查询”，可查看所有样本从创建到出报告的时间线。"}]}]}`
}

func parseHelpCenterJSON(raw string) (utils.H, error) {
	if strings.TrimSpace(raw) == "" {
		raw = defaultHelpCenterJSON()
	}
	var payload utils.H
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, err
	}
	if _, ok := payload["categories"]; !ok {
		payload["categories"] = []interface{}{}
	}
	return payload, nil
}

func HandleGetHelpCenterSetting(c *app.RequestContext, db *sql.DB) {
	ensureSystemSettingDefaults(db)
	raw := ""
	err := db.QueryRow(`SELECT key_value FROM setting_system WHERE key_name = 'MINIAPP_HELP_CENTER_JSON' LIMIT 1`).Scan(&raw)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Failed to query help center setting: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "获取帮助中心失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	payload, parseErr := parseHelpCenterJSON(raw)
	if parseErr != nil {
		log.Printf("Failed to parse help center setting: %v", parseErr)
		payload, _ = parseHelpCenterJSON(defaultHelpCenterJSON())
	}
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取帮助中心成功",
		Data:    payload,
	})
}

func HandleUpdateHelpCenterSetting(c *app.RequestContext, db *sql.DB) {
	var payload utils.H
	if err := c.Bind(&payload); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请求参数错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}
	if _, ok := payload["categories"]; !ok {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "帮助中心至少需要 categories 字段",
			Data:    nil,
		})
		return
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "帮助中心内容格式错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	_, err = db.Exec(`INSERT INTO setting_system (key_name, key_value, key_type, is_encrypted, description, created_at, updated_at)
		VALUES ('MINIAPP_HELP_CENTER_JSON', ?, 'textarea', 0, '小程序帮助中心 JSON，支持按 categories 分类配置常见问题', NOW(), NOW())
		ON DUPLICATE KEY UPDATE key_value = VALUES(key_value), key_type = 'textarea', is_encrypted = 0, updated_at = NOW()`, string(raw))
	if err != nil {
		log.Printf("Failed to update help center setting: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "保存帮助中心失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "保存帮助中心成功",
		Data:    payload,
	})
}

// 处理获取角色权限请求
func HandleGetRolePermissions(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	idParam := c.Param("id")
	var id int
	_, err := fmt.Sscanf(idParam, "%d", &id)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的角色ID",
			Data:    nil,
		})
		return
	}

	// 从数据库查询角色权限
	rows, err := db.Query(`SELECT page_id, page_name, parent_page_id, checked 
			FROM setting_role_permission WHERE role_id = ? ORDER BY page_id ASC`, id)
	if err != nil {
		log.Printf("Failed to query setting_role permissions: %v", err)
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "获取角色权限成功",
			Data:    []utils.H{},
		})
		return
	}
	defer rows.Close()

	// 构建权限树结构
	permissionsMap := make(map[string]utils.H)
	var permissions []utils.H

	// 遍历查询结果
	for rows.Next() {
		var pageID, pageName, parentPageID string
		var checked bool

		err := rows.Scan(&pageID, &pageName, &parentPageID, &checked)
		if err != nil {
			log.Printf("Failed to scan setting_role permission: %v", err)
			continue
		}

		// 构建权限节点
		permission := utils.H{
			"id":       pageID,
			"title":    pageName,
			"key":      pageID,
			"checked":  checked,
			"children": []utils.H{},
		}

		permissionsMap[pageID] = permission

		// 如果是根节点，直接添加到权限列表
		if parentPageID == "" {
			permissions = append(permissions, permission)
		} else {
			// 如果是子节点，添加到父节点的children中
			if parent, exists := permissionsMap[parentPageID]; exists {
				if children, ok := parent["children"].([]utils.H); ok {
					children = append(children, permission)
					parent["children"] = children
				}
			}
		}
	}

	// 检查遍历过程中是否有错误
	if err = rows.Err(); err != nil {
		log.Printf("Error iterating setting_role permissions: %v", err)
	}

	// 返回角色权限，使用前端期望的格式
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取角色权限成功",
		Data:    permissions,
	})
}

func scanPermissionTree(rows *sql.Rows) ([]utils.H, error) {
	permissionsMap := make(map[string]utils.H)
	var permissions []utils.H

	for rows.Next() {
		var pageID, pageName, parentPageID string
		var checked bool
		if err := rows.Scan(&pageID, &pageName, &parentPageID, &checked); err != nil {
			return permissions, err
		}

		permission := utils.H{
			"id":       pageID,
			"title":    pageName,
			"key":      pageID,
			"checked":  checked,
			"children": []utils.H{},
		}
		permissionsMap[pageID] = permission

		if parentPageID == "" {
			permissions = append(permissions, permission)
			continue
		}
		if parent, exists := permissionsMap[parentPageID]; exists {
			if children, ok := parent["children"].([]utils.H); ok {
				parent["children"] = append(children, permission)
			}
		}
	}
	return permissions, rows.Err()
}

func normalizeRoleIDs(primaryRoleID int, roleIDs []int) []int {
	seen := map[int]bool{}
	result := []int{}
	if primaryRoleID > 0 {
		seen[primaryRoleID] = true
		result = append(result, primaryRoleID)
	}
	for _, roleID := range roleIDs {
		if roleID <= 0 || seen[roleID] {
			continue
		}
		seen[roleID] = true
		result = append(result, roleID)
	}
	return result
}

func getUserRoles(db *sql.DB, userID int, primaryRoleID int) ([]utils.H, []int, []string) {
	roleIDs := []int{}
	rows, err := db.Query(`SELECT ur.role_id FROM base_manage_user_role ur
		INNER JOIN setting_role r ON ur.role_id = r.id
		WHERE ur.user_id = ? AND r.status = 1
		ORDER BY ur.id ASC`, userID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var roleID int
			if scanErr := rows.Scan(&roleID); scanErr == nil {
				roleIDs = append(roleIDs, roleID)
			}
		}
	}
	roleIDs = normalizeRoleIDs(primaryRoleID, roleIDs)

	roles := []utils.H{}
	names := []string{}
	for _, roleID := range roleIDs {
		var name, description string
		if err := db.QueryRow("SELECT name, description FROM setting_role WHERE id = ? AND status = 1", roleID).Scan(&name, &description); err != nil {
			continue
		}
		roles = append(roles, utils.H{"id": roleID, "name": name, "description": description})
		names = append(names, name)
	}
	return roles, roleIDs, names
}

func getUserRoleNames(db *sql.DB, userID int) []string {
	var primaryRoleID sql.NullInt32
	_ = db.QueryRow("SELECT role_id FROM base_manage_user WHERE id = ?", userID).Scan(&primaryRoleID)
	_, _, names := getUserRoles(db, userID, int(primaryRoleID.Int32))
	return names
}

func hasRoleName(roleNames []string, keywords ...string) bool {
	for _, roleName := range roleNames {
		for _, keyword := range keywords {
			if strings.Contains(roleName, keyword) {
				return true
			}
		}
	}
	return false
}

func isValidReportReviewer(db *sql.DB, userID int) bool {
	var employeeID, username string
	if err := db.QueryRow("SELECT COALESCE(employee_id, ''), COALESCE(username, '') FROM base_manage_user WHERE id = ? AND status = 1", userID).Scan(&employeeID, &username); err != nil {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(employeeID), "admin") || strings.EqualFold(strings.TrimSpace(username), "admin") {
		return false
	}
	return hasRoleName(getUserRoleNames(db, userID), "管理员", "管理", "IT")
}

func mustSelectRealReportReviewer(db *sql.DB, userID int) bool {
	var employeeID, username string
	if err := db.QueryRow("SELECT COALESCE(employee_id, ''), COALESCE(username, '') FROM base_manage_user WHERE id = ? AND status = 1", userID).Scan(&employeeID, &username); err != nil {
		return true
	}
	employeeID = strings.TrimSpace(employeeID)
	username = strings.TrimSpace(username)
	if strings.EqualFold(employeeID, "admin") || strings.EqualFold(username, "admin") {
		return true
	}
	roleNames := getUserRoleNames(db, userID)
	return hasRoleName(roleNames, "实验室")
}

func syncUserRoles(tx *sql.Tx, userID int, primaryRoleID int, roleIDs []int) error {
	roleIDs = normalizeRoleIDs(primaryRoleID, roleIDs)
	if len(roleIDs) == 0 {
		return fmt.Errorf("至少选择一个角色")
	}
	if _, err := tx.Exec("DELETE FROM base_manage_user_role WHERE user_id = ?", userID); err != nil {
		return err
	}
	for _, roleID := range roleIDs {
		if _, err := tx.Exec(`INSERT INTO base_manage_user_role (user_id, role_id, created_at, updated_at)
			VALUES (?, ?, NOW(), NOW())
			ON DUPLICATE KEY UPDATE updated_at = NOW()`, userID, roleID); err != nil {
			return err
		}
	}
	return nil
}

func queryRolePermissionsForUser(db *sql.DB, userID int, primaryRoleID int) ([]utils.H, error) {
	_, roleIDs, _ := getUserRoles(db, userID, primaryRoleID)
	if len(roleIDs) == 0 {
		return []utils.H{}, nil
	}
	placeholders := make([]string, 0, len(roleIDs))
	args := make([]interface{}, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		placeholders = append(placeholders, "?")
		args = append(args, roleID)
	}
	rows, err := db.Query(fmt.Sprintf(`SELECT page_id, MAX(page_name), MAX(parent_page_id), MAX(checked)
		FROM setting_role_permission
		WHERE role_id IN (%s)
		GROUP BY page_id
		ORDER BY page_id ASC`, strings.Join(placeholders, ",")), args...)
	if err != nil {
		return []utils.H{}, err
	}
	defer rows.Close()
	return scanPermissionTree(rows)
}

func writePermissionTree(tx *sql.Tx, tableName, ownerColumn string, ownerID int, permissions []interface{}) error {
	var process func(items []interface{}, parentPageID string) error
	process = func(items []interface{}, parentPageID string) error {
		for _, item := range items {
			perm, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			pageID := fmt.Sprint(perm["id"])
			if pageID == "" || pageID == "<nil>" {
				pageID = fmt.Sprint(perm["key"])
			}
			if pageID == "" || pageID == "<nil>" {
				continue
			}
			pageName := pageID
			if title := strings.TrimSpace(fmt.Sprint(perm["title"])); title != "" && title != "<nil>" {
				pageName = title
			} else if label := strings.TrimSpace(fmt.Sprint(perm["label"])); label != "" && label != "<nil>" {
				pageName = label
			}
			checked := true
			if value, ok := perm["checked"].(bool); ok {
				checked = value
			}

			query := fmt.Sprintf(`INSERT INTO %s (%s, page_id, page_name, parent_page_id, checked, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, NOW(), NOW())`, tableName, ownerColumn)
			if _, err := tx.Exec(query, ownerID, pageID, pageName, parentPageID, checked); err != nil {
				return err
			}
			if children, ok := perm["children"].([]interface{}); ok && len(children) > 0 {
				if err := process(children, pageID); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return process(permissions, "")
}

// HandleGetUserPermissions 获取某个用户的独立权限。若用户未设置独立权限，则返回其角色权限。
func HandleGetUserPermissions(c *app.RequestContext, db *sql.DB) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "无效的用户ID", Data: nil})
		return
	}

	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM base_manage_user_permission WHERE user_id = ?", id).Scan(&count)
	query := `SELECT page_id, page_name, parent_page_id, checked FROM base_manage_user_permission WHERE user_id = ? ORDER BY page_id ASC`
	args := []interface{}{id}
	source := "user"
	if count == 0 {
		var roleID sql.NullInt32
		if err := db.QueryRow("SELECT role_id FROM base_manage_user WHERE id = ?", id).Scan(&roleID); err != nil {
			c.JSON(consts.StatusNotFound, ApiResponse{Code: 404, Success: false, Message: "用户不存在", Data: nil})
			return
		}
		permissions, err := queryRolePermissionsForUser(db, id, int(roleID.Int32))
		if err != nil {
			log.Printf("Failed to query role permissions for user: %v", err)
		}
		c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "获取用户权限成功", Data: utils.H{"source": "role", "permissions": permissions}})
		return
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("Failed to query user permissions: %v", err)
		c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "获取用户权限成功", Data: []utils.H{}})
		return
	}
	defer rows.Close()

	permissions, err := scanPermissionTree(rows)
	if err != nil {
		log.Printf("Failed to scan user permissions: %v", err)
	}
	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "获取用户权限成功", Data: utils.H{"source": source, "permissions": permissions}})
}

// HandleUpdateUserPermissions 为某个用户保存独立权限。
func HandleUpdateUserPermissions(c *app.RequestContext, db *sql.DB) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "无效的用户ID", Data: nil})
		return
	}
	var req struct {
		Permissions []interface{} `json:"permissions"`
	}
	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "请求参数错误", Data: utils.H{"error": err.Error()}})
		return
	}

	tx, err := db.Begin()
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "服务器内部错误", Data: nil})
		return
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM base_manage_user_permission WHERE user_id = ?", id); err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "保存权限失败", Data: nil})
		return
	}
	if len(req.Permissions) > 0 {
		if err := writePermissionTree(tx, "base_manage_user_permission", "user_id", id, req.Permissions); err != nil {
			log.Printf("Failed to write user permissions: %v", err)
			c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "保存权限失败", Data: nil})
			return
		}
	}
	if err := tx.Commit(); err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "保存权限失败", Data: nil})
		return
	}
	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "用户权限已保存", Data: nil})
}

func HandleClearUserPermissions(c *app.RequestContext, db *sql.DB) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "无效的用户ID", Data: nil})
		return
	}
	if _, err := db.Exec("DELETE FROM base_manage_user_permission WHERE user_id = ?", id); err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "清除权限失败", Data: nil})
		return
	}
	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "已恢复角色默认权限", Data: nil})
}

// 处理更新部门状态请求
func HandleUpdateDepartmentStatus(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	idParam := c.Param("id")
	var id int
	_, err := fmt.Sscanf(idParam, "%d", &id)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的部门ID",
			Data:    nil,
		})
		return
	}

	// 解析请求体
	var req struct {
		Status int `json:"status"`
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

	// 检查Status字段是否有值
	if req.Status != 0 && req.Status != 1 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "状态值无效",
			Data:    nil,
		})
		return
	}

	// 更新部门状态到数据库
	_, err = db.Exec(`UPDATE setting_department SET status = ?, updated_at = NOW() WHERE id = ?`,
		req.Status, id)
	if err != nil {
		log.Printf("Failed to update setting_department status: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 返回更新成功响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "状态更新成功",
		Data:    nil,
	})
}

// 生成随机盐值
func generateSalt(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// 使用MD5加密密码（带盐值）
func md5HashWithSalt(password, salt string) string {
	hasher := md5.New()
	hasher.Write([]byte(password + salt))
	return hex.EncodeToString(hasher.Sum(nil))
}

// 加密配置值
func encryptConfigValue(value string) string {
	key := []byte("huawei_config_secret_key")
	block, err := aes.NewCipher(key)
	if err != nil {
		log.Printf("Error creating cipher: %v", err)
		return ""
	}

	iv := make([]byte, aes.BlockSize)
	stream := cipher.NewCFBEncrypter(block, iv)

	encrypted := make([]byte, len(value))
	stream.XORKeyStream(encrypted, []byte(value))

	return base64.StdEncoding.EncodeToString(encrypted)
}

// 解密配置值
func decryptConfigValue(encryptedValue string) string {
	if encryptedValue == "" {
		return ""
	}

	key := []byte("huawei_config_secret_key")
	block, err := aes.NewCipher(key)
	if err != nil {
		log.Printf("Error creating cipher: %v", err)
		return ""
	}

	iv := make([]byte, aes.BlockSize)
	stream := cipher.NewCFBDecrypter(block, iv)

	encrypted, err := base64.StdEncoding.DecodeString(encryptedValue)
	if err != nil {
		log.Printf("Error decoding base64: %v", err)
		return ""
	}

	decrypted := make([]byte, len(encrypted))
	stream.XORKeyStream(decrypted, encrypted)

	return string(decrypted)
}

// 处理创建用户请求
func HandleCreateUser(c *app.RequestContext, db *sql.DB) {
	// 解析请求体
	var req struct {
		Username     string `json:"username" binding:"required"`
		Password     string `json:"password"`
		RealName     string `json:"real_name" binding:"required"`
		EmployeeId   string `json:"employee_id"`
		Phone        string `json:"phone" binding:"required"`
		RoleId       *int   `json:"role_id" binding:"required"`
		RoleIds      []int  `json:"role_ids"`
		DepartmentId *int   `json:"department_id" binding:"required"`
		Status       int    `json:"status"`
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

	// 处理setting_roleId和setting_departmentId，将nil转换为NULL
	var setting_roleIdValue interface{} = nil
	if req.RoleId != nil {
		setting_roleIdValue = *req.RoleId
	}
	roleIDs := normalizeRoleIDs(0, req.RoleIds)
	if req.RoleId != nil {
		roleIDs = normalizeRoleIDs(*req.RoleId, roleIDs)
	}
	if len(roleIDs) == 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "请选择角色", Data: nil})
		return
	}
	setting_roleIdValue = roleIDs[0]

	var setting_departmentIdValue interface{} = nil
	if req.DepartmentId != nil {
		setting_departmentIdValue = *req.DepartmentId
	}

	// 生成随机盐值
	salt := generateSalt(16)
	if strings.TrimSpace(req.Password) == "" {
		req.Password = defaultUserPassword(req.RealName)
	}
	// 与前端保持一致：先计算密码的MD5，再与盐值拼接后计算MD5
	passwordMd5 := md5Hash(req.Password)
	hashedPassword := md5HashWithSalt(passwordMd5, salt)

	tx, err := db.Begin()
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "服务器内部错误", Data: nil})
		return
	}
	defer tx.Rollback()

	// 插入用户到数据库
	result, err := tx.Exec(`INSERT INTO base_manage_user (username, password, salt, real_name, phone, department_id, role_id, status, created_at, updated_at, employee_id) 
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW(), ?)`,
		req.Username, hashedPassword, salt, req.RealName, req.Phone, setting_departmentIdValue, setting_roleIdValue, req.Status, req.EmployeeId)
	if err != nil {
		log.Printf("Failed to create user: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 获取插入的用户ID
	id, err := result.LastInsertId()
	if err != nil {
		log.Printf("Failed to get last insert ID: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}
	if err := syncUserRoles(tx, int(id), roleIDs[0], roleIDs); err != nil {
		log.Printf("Failed to sync user roles: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "保存角色失败", Data: utils.H{"error": err.Error()}})
		return
	}
	if err := tx.Commit(); err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "服务器内部错误", Data: nil})
		return
	}

	// 返回创建的用户ID
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "创建用户成功",
		Data: utils.H{
			"id": id,
		},
	})
}

// 处理更新用户请求
func HandleUpdateSystemUser(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	idParam := c.Param("id")
	var id int
	_, err := fmt.Sscanf(idParam, "%d", &id)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的用户ID",
			Data:    nil,
		})
		return
	}

	// 解析请求体
	var req struct {
		RealName     string `json:"real_name" binding:"required"`
		EmployeeId   string `json:"employee_id"`
		Phone        string `json:"phone" binding:"required"`
		RoleId       *int   `json:"role_id" binding:"required"`
		RoleIds      []int  `json:"role_ids"`
		DepartmentId *int   `json:"department_id" binding:"required"`
		Status       int    `json:"status"`
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

	// 处理setting_roleId和setting_departmentId，将nil转换为NULL
	var setting_roleIdValue interface{} = nil
	if req.RoleId != nil {
		setting_roleIdValue = *req.RoleId
	}
	roleIDs := normalizeRoleIDs(0, req.RoleIds)
	if req.RoleId != nil {
		roleIDs = normalizeRoleIDs(*req.RoleId, roleIDs)
	}
	if len(roleIDs) == 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "请选择角色", Data: nil})
		return
	}
	setting_roleIdValue = roleIDs[0]

	var setting_departmentIdValue interface{} = nil
	if req.DepartmentId != nil {
		setting_departmentIdValue = *req.DepartmentId
	}

	tx, err := db.Begin()
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "服务器内部错误", Data: nil})
		return
	}
	defer tx.Rollback()

	// 更新用户信息到数据库
	_, err = tx.Exec(`UPDATE base_manage_user SET real_name = ?, phone = ?, department_id = ?, role_id = ?, status = ?, updated_at = NOW(), employee_id = ? 
			WHERE id = ?`,
		req.RealName, req.Phone, setting_departmentIdValue, setting_roleIdValue, req.Status, req.EmployeeId, id)
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
	if err := syncUserRoles(tx, id, roleIDs[0], roleIDs); err != nil {
		log.Printf("Failed to sync user roles: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "保存角色失败", Data: utils.H{"error": err.Error()}})
		return
	}
	if err := tx.Commit(); err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "服务器内部错误", Data: nil})
		return
	}

	// 返回更新成功响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "更新用户成功",
		Data:    nil,
	})
}

func isCurrentSystemAdmin(c *app.RequestContext, db *sql.DB) bool {
	userID, ok := GetUserID(c)
	if !ok {
		return false
	}
	return hasRoleName(getUserRoleNames(db, userID), "管理员", "IT")
}

// HandleResetUserPassword 管理员将指定账号密码重置为 Hw123456。
func HandleResetUserPassword(c *app.RequestContext, db *sql.DB) {
	if !isCurrentSystemAdmin(c, db) {
		c.JSON(consts.StatusForbidden, ApiResponse{
			Code:    403,
			Success: false,
			Message: "仅管理员可重置密码",
			Data:    nil,
		})
		return
	}

	idParam := c.Param("id")
	var id int
	if _, err := fmt.Sscanf(idParam, "%d", &id); err != nil || id <= 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的用户ID",
			Data:    nil,
		})
		return
	}

	var username string
	if err := db.QueryRow("SELECT username FROM base_manage_user WHERE id = ?", id).Scan(&username); err != nil {
		c.JSON(consts.StatusNotFound, ApiResponse{
			Code:    404,
			Success: false,
			Message: "用户不存在",
			Data:    nil,
		})
		return
	}

	salt := generateSalt(16)
	hashedPassword := hashManageUserPassword("Hw123456", salt)
	_, err := db.Exec("UPDATE base_manage_user SET password = ?, salt = ?, updated_at = NOW() WHERE id = ?", hashedPassword, salt, id)
	if err != nil {
		log.Printf("Reset user password error: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "重置密码失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "密码已重置为 Hw123456",
		Data:    utils.H{"username": username},
	})
}

// 处理删除用户请求
func HandleDeleteUser(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	idParam := c.Param("id")
	var id int
	_, err := fmt.Sscanf(idParam, "%d", &id)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的用户ID",
			Data:    nil,
		})
		return
	}

	// 删除用户
	_, err = db.Exec("DELETE FROM base_manage_user WHERE id = ?", id)
	if err != nil {
		log.Printf("Failed to delete user: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 返回删除成功响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "删除用户成功",
		Data:    nil,
	})
}

// 处理更新用户状态请求
func HandleUpdateUserStatus(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	idParam := c.Param("id")
	var id int
	_, err := fmt.Sscanf(idParam, "%d", &id)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的用户ID",
			Data:    nil,
		})
		return
	}

	// 解析请求体
	var req struct {
		Status int `json:"status"`
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

	// 检查Status字段是否有值
	if req.Status != 0 && req.Status != 1 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "状态值无效",
			Data:    nil,
		})
		return
	}

	// 更新用户状态到数据库
	_, err = db.Exec(`UPDATE base_manage_user SET status = ?, updated_at = NOW() WHERE id = ?`,
		req.Status, id)
	if err != nil {
		log.Printf("Failed to update user status: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 返回更新成功响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "状态更新成功",
		Data:    nil,
	})
}

// 处理更新角色状态请求
func HandleUpdateRoleStatus(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	idParam := c.Param("id")
	var id int
	_, err := fmt.Sscanf(idParam, "%d", &id)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的角色ID",
			Data:    nil,
		})
		return
	}

	// 解析请求体
	var req struct {
		Status int `json:"status"`
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

	// 检查Status字段是否有值
	if req.Status != 0 && req.Status != 1 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "状态值无效",
			Data:    nil,
		})
		return
	}

	// 更新角色状态到数据库
	_, err = db.Exec(`UPDATE setting_role SET status = ?, updated_at = NOW() WHERE id = ?`,
		req.Status, id)
	if err != nil {
		log.Printf("Failed to update setting_role status: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 返回更新成功响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "状态更新成功",
		Data:    nil,
	})
}

// 处理创建癌症类型请求
func HandleCreateCancerType(c *app.RequestContext, db *sql.DB) {
	// 解析请求体
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		IsActive    int    `json:"is_active"`
		PanelIDs    string `json:"panel_ids"`
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

	// 设置默认值：新创建的检测类型默认启用
	if req.IsActive == 0 {
		req.IsActive = 1
	}

	// 插入癌症类型到数据库
	result, err := db.Exec(`INSERT INTO setting_cancer_type (name, description, is_active, panel_ids, created_at, updated_at) 
			VALUES (?, ?, ?, ?, NOW(), NOW())`,
		req.Name, req.Description, req.IsActive, req.PanelIDs)
	if err != nil {
		log.Printf("Failed to create cancer type: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 清理缓存
	DeleteCache("setting_cancer_types")

	// 获取插入的癌症类型ID
	id, err := result.LastInsertId()
	if err != nil {
		log.Printf("Failed to get last insert ID: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 返回创建的癌症类型ID
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "创建检测癌种成功",
		Data: utils.H{
			"id": id,
		},
	})
}

// 处理更新癌症类型请求
func HandleUpdateCancerType(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	idParam := c.Param("id")
	var id int
	_, err := fmt.Sscanf(idParam, "%d", &id)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的检测癌种ID",
			Data:    nil,
		})
		return
	}

	// 解析请求体
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		IsActive    int    `json:"is_active"`
		PanelIDs    string `json:"panel_ids"`
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

	// 获取现有数据
	var existingName, existingDescription, existingPanelIDs string
	var existingIsActive int
	err = db.QueryRow("SELECT name, description, is_active, COALESCE(panel_ids, '') FROM setting_cancer_type WHERE id = ?", id).Scan(&existingName, &existingDescription, &existingIsActive, &existingPanelIDs)
	if err != nil {
		log.Printf("Failed to get existing cancer type: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 使用现有值或新值
	if req.Name == "" {
		req.Name = existingName
	}
	if req.Description == "" {
		req.Description = existingDescription
	}
	if req.PanelIDs == "" {
		req.PanelIDs = existingPanelIDs
	}
	// 直接使用前端传来的 is_active 值
	log.Printf("is_active从%d变更为%d (id=%d)", existingIsActive, req.IsActive, id)

	// 更新癌症类型信息到数据库（is_active字段直接使用前端发送的值）
	_, err = db.Exec(`UPDATE setting_cancer_type SET name = ?, description = ?, is_active = ?, panel_ids = ?, updated_at = NOW()
			WHERE id = ?`,
		req.Name, req.Description, req.IsActive, req.PanelIDs, id)
	if err != nil {
		log.Printf("Failed to update cancer type: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 清理缓存
	DeleteCache("setting_cancer_types")

	// 返回更新成功响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "更新检测癌种成功",
		Data:    nil,
	})
}

// 处理删除癌症类型请求
func HandleDeleteCancerType(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	idParam := c.Param("id")
	var id int
	_, err := fmt.Sscanf(idParam, "%d", &id)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的检测癌种ID",
			Data:    nil,
		})
		return
	}

	// 执行软删除：将is_active字段设置为0
	_, err = db.Exec("UPDATE setting_cancer_type SET is_active = 0, updated_at = NOW() WHERE id = ?", id)
	if err != nil {
		log.Printf("Failed to delete cancer type: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 清理缓存
	DeleteCache("setting_cancer_types")

	// 返回删除成功响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "删除检测癌种成功",
		Data:    nil,
	})
}

// 处理创建样本类型请求
func HandleCreateSampleType(c *app.RequestContext, db *sql.DB) {
	// 解析请求体
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		IsActive    int    `json:"is_active"`
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

	// 插入样本类型到数据库
	result, err := db.Exec(`INSERT INTO setting_sample_type (name, description, is_active, created_at, updated_at) 
				VALUES (?, ?, ?, NOW(), NOW())`,
		req.Name, req.Description, req.IsActive)
	if err != nil {
		log.Printf("Failed to create sample type: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 获取插入的样本类型ID
	id, err := result.LastInsertId()
	if err != nil {
		log.Printf("Failed to get last insert ID: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 返回创建的样本类型ID
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "创建样本类型成功",
		Data: utils.H{
			"id": id,
		},
	})
}

// 处理更新样本类型请求
func HandleUpdateSampleType(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	idParam := c.Param("id")
	var id int
	_, err := fmt.Sscanf(idParam, "%d", &id)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的样本类型ID",
			Data:    nil,
		})
		return
	}

	// 解析请求体
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		IsActive    int    `json:"is_active"`
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

	// 获取现有数据
	var existingName, existingDescription string
	var existingIsActive int
	err = db.QueryRow("SELECT name, description, is_active FROM setting_sample_type WHERE id = ?", id).Scan(&existingName, &existingDescription, &existingIsActive)
	if err != nil {
		log.Printf("Failed to get existing sample type: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 使用现有值或新值
	if req.Name == "" {
		req.Name = existingName
	}
	if req.Description == "" {
		req.Description = existingDescription
	}

	// 更新样本类型信息到数据库
	_, err = db.Exec(`UPDATE setting_sample_type SET name = ?, description = ?, is_active = ?, updated_at = NOW() 
			WHERE id = ?`,
		req.Name, req.Description, req.IsActive, id)
	if err != nil {
		log.Printf("Failed to update sample type: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 清理缓存
	DeleteCache("setting_sample_types")

	// 返回更新成功响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "更新样本类型成功",
		Data:    nil,
	})
}

// 处理删除样本类型请求
func HandleDeleteSampleType(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	idParam := c.Param("id")
	var id int
	_, err := fmt.Sscanf(idParam, "%d", &id)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的样本类型ID",
			Data:    nil,
		})
		return
	}

	// 执行软删除：将is_active字段设置为0
	_, err = db.Exec("UPDATE setting_sample_type SET is_active = 0, updated_at = NOW() WHERE id = ?", id)
	if err != nil {
		log.Printf("Failed to delete sample type: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 返回删除成功响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "删除样本类型成功",
		Data:    nil,
	})
}

// 处理清除缓存请求
func HandleClearCache(c *app.RequestContext) {
	// 解析请求体
	var req struct {
		CacheType string `json:"cacheType"`
	}

	if err := c.Bind(&req); err != nil {
		// 如果解析失败，默认清除所有缓存
		req.CacheType = "all"
	}

	// 根据CacheType清除不同类型的缓存
	switch req.CacheType {
	case "genes":
		// 清除基因相关缓存
		DeleteCache("genes")
		DeleteCache("genes_active")
	case "panels":
		// 清除Panel相关缓存
		DeleteCache("genes")
		DeleteCache("genes_active")
	case "models":
		// 清除模型相关缓存
		ClearCache("models:*")
	case "cancerTypes":
		// 清除癌症类型缓存
		DeleteCache("setting_cancer_types")
	case "sampleTypes":
		// 清除样本类型缓存
		DeleteCache("setting_sample_types")
	case "treatmentStages":
		// 清除治疗阶段缓存
		DeleteCache("setting_treatment_stages")
		DeleteCache("setting_treatment_stages_allowed_v1")
		DeleteCache("setting_treatment_stages_allowed_v2")
	case "dashboard":
		// 清除仪表板缓存
		DeleteCache("dashboard_stats_all")
		DeleteCache("dashboard_stats_week")
		DeleteCache("dashboard_stats_day")
	default:
		// 清除所有系统缓存
		DeleteCache("genes")
		DeleteCache("genes_active")
		ClearCache("models:*")
		DeleteCache("setting_cancer_types")
		DeleteCache("setting_sample_types")
		DeleteCache("setting_treatment_stages")
		DeleteCache("setting_treatment_stages_allowed_v1")
		DeleteCache("setting_treatment_stages_allowed_v2")
		DeleteCache("dashboard_stats_all")
		DeleteCache("dashboard_stats_week")
		DeleteCache("dashboard_stats_day")
	}

	log.Println("Cache cleared successfully for type:", req.CacheType)

	// 返回清除成功响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "缓存清除成功",
		Data:    nil,
	})
}

// maskAPIKey 对API Key进行脱敏处理，只显示前8位和后4位
func maskAPIKey(apiKey string) string {
	if len(apiKey) <= 12 {
		return "******"
	}
	return apiKey[:8] + "******" + apiKey[len(apiKey)-4:]
}

// HandleGetAISettings 获取AI配置信息
func HandleGetAISettings(c *app.RequestContext, db *sql.DB) {
	ReloadAISettings(db)
	currentAPIKey := aiAPIKey
	currentURL := aiAPIURL
	currentModel := aiModel
	currentPrompt := aiPrompt

	// 脱敏处理API Key
	maskedAPIKey := maskAPIKey(currentAPIKey)

	// 返回AI配置信息
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取AI配置信息成功",
		Data: utils.H{
			"api_key":             maskedAPIKey,
			"api_url":             currentURL,
			"model":               currentModel,
			"prompt":              currentPrompt,
			"report_vision_model": aiReportVisionModel,
			"report_text_model":   aiReportTextModel,
			"report_prompt":       aiReportPrompt,
			"configured":          currentAPIKey != "",
		},
	})
}

// HandleUpdateAISettings 将 AI 配置加密保存到数据库并立即生效。
func HandleUpdateAISettings(c *app.RequestContext, db *sql.DB) {
	var req struct {
		APIKey            string `json:"api_key"`
		APIURL            string `json:"api_url"`
		Model             string `json:"model"`
		Prompt            string `json:"prompt"`
		ReportVisionModel string `json:"report_vision_model"`
		ReportTextModel   string `json:"report_text_model"`
		ReportPrompt      string `json:"report_prompt"`
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

	// 如果 API Key 包含脱敏星号，表明用户没有编辑过它，依然使用原密钥
	finalAPIKey := req.APIKey
	if strings.Contains(req.APIKey, "******") {
		finalAPIKey = aiAPIKey
	}
	req.APIURL = strings.TrimSpace(req.APIURL)
	req.Model = strings.TrimSpace(req.Model)
	req.ReportVisionModel = strings.TrimSpace(req.ReportVisionModel)
	req.ReportTextModel = strings.TrimSpace(req.ReportTextModel)
	if finalAPIKey == "" || req.APIURL == "" || req.Model == "" || req.ReportVisionModel == "" || req.ReportTextModel == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "请完整填写 AI 接口、密钥及模型", Data: nil})
		return
	}
	if strings.TrimSpace(req.ReportPrompt) == "" {
		req.ReportPrompt = defaultReportAnalysisPrompt
	}
	values := []struct {
		key, value, keyType, description string
		encrypted                        int
	}{
		{"AI_API_KEY", finalAPIKey, "password", "百度千帆 AI API Key", 1},
		{"AI_API_URL", req.APIURL, "text", "百度千帆 OpenAI 兼容接口 Base URL", 0},
		{"AI_MODEL", req.Model, "text", "AI 客服文本模型", 0},
		{"AI_PROMPT", req.Prompt, "textarea", "AI 客服系统提示词", 0},
		{"AI_REPORT_VISION_MODEL", req.ReportVisionModel, "text", "图片报告视觉分析模型", 0},
		{"AI_REPORT_TEXT_MODEL", req.ReportTextModel, "text", "PDF 报告文本分析模型", 0},
		{"AI_REPORT_PROMPT", req.ReportPrompt, "textarea", "患者上传报告分析提示词", 0},
	}
	tx, err := db.Begin()
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "保存配置失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}
	defer tx.Rollback()
	for _, item := range values {
		value := item.value
		if item.encrypted == 1 {
			value = encryptConfigValue(value)
		}
		if _, err := tx.Exec(`INSERT INTO setting_system
			(key_name, key_value, key_type, is_encrypted, description, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, NOW(), NOW())
			ON DUPLICATE KEY UPDATE key_value = VALUES(key_value), key_type = VALUES(key_type),
				is_encrypted = VALUES(is_encrypted), description = VALUES(description), updated_at = NOW()`,
			item.key, value, item.keyType, item.encrypted, item.description); err != nil {
			c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "保存配置失败", Data: utils.H{"error": err.Error()}})
			return
		}
	}
	if err := tx.Commit(); err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "保存配置失败", Data: utils.H{"error": err.Error()}})
		return
	}
	ReloadAISettings(db)

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "修改并保存AI配置成功",
		Data: utils.H{
			"api_key":             maskAPIKey(finalAPIKey),
			"api_url":             req.APIURL,
			"model":               req.Model,
			"prompt":              req.Prompt,
			"report_vision_model": req.ReportVisionModel,
			"report_text_model":   req.ReportTextModel,
			"report_prompt":       req.ReportPrompt,
			"configured":          finalAPIKey != "",
		},
	})
}

// HandleGetAIUsage 获取AI使用统计信息
func HandleGetAIUsage(c *app.RequestContext, db *sql.DB) {
	var todayUsage, monthUsage, totalUsage int64

	// 查询今日用量
	err := db.QueryRow(`SELECT COALESCE(SUM(token_count), 0) FROM setting_ai_usage_log WHERE DATE(created_at) = CURDATE()`).Scan(&todayUsage)
	if err != nil {
		log.Printf("Failed to query today AI usage: %v", err)
	}

	// 查询本月用量
	err = db.QueryRow(`SELECT COALESCE(SUM(token_count), 0) FROM setting_ai_usage_log WHERE YEAR(created_at) = YEAR(CURDATE()) AND MONTH(created_at) = MONTH(CURDATE())`).Scan(&monthUsage)
	if err != nil {
		log.Printf("Failed to query month AI usage: %v", err)
	}

	// 查询总用量
	err = db.QueryRow(`SELECT COALESCE(SUM(token_count), 0) FROM setting_ai_usage_log`).Scan(&totalUsage)
	if err != nil {
		log.Printf("Failed to query total AI usage: %v", err)
	}

	// 查询历史趋势（最近7天）
	var history []utils.H
	rows, err := db.Query(`SELECT DATE(created_at) as date, COALESCE(SUM(token_count), 0) as count 
		FROM setting_ai_usage_log 
		WHERE created_at >= DATE_SUB(CURDATE(), INTERVAL 6 DAY)
		GROUP BY DATE(created_at)
		ORDER BY date ASC`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var date string
			var count int64
			if err := rows.Scan(&date, &count); err == nil {
				history = append(history, utils.H{
					"date":  date,
					"count": count,
				})
			}
		}
	}

	// 返回统计信息
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取AI使用统计成功",
		Data: utils.H{
			"today_usage": todayUsage,
			"month_usage": monthUsage,
			"total_usage": totalUsage,
			"history":     history,
		},
	})
}

// RecordAIUsage 记录AI使用量（已扩展保存UserID/PatientID/IdentityType）
func RecordAIUsage(db *sql.DB, tokenCount int64, model string, userID int, patientID int, identityType string) {
	_, err := db.Exec(`INSERT INTO setting_ai_usage_log (token_count, model, user_id, patient_id, identity_type, created_at) VALUES (?, ?, ?, ?, ?, NOW())`, tokenCount, model, userID, patientID, identityType)
	if err != nil {
		log.Printf("Failed to record AI usage: %v", err)
	}
}

// HandleUpdateUserAIAccess 修改系统管理用户的AI访问权限
func HandleUpdateUserAIAccess(c *app.RequestContext, db *sql.DB) {
	idParam := c.Param("id")
	var id int
	_, err := fmt.Sscanf(idParam, "%d", &id)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的用户ID",
			Data:    nil,
		})
		return
	}

	var req struct {
		Allowed bool `json:"allowed"`
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

	aiAllowed := 0
	if req.Allowed {
		aiAllowed = 1
	}

	_, err = db.Exec(`UPDATE base_manage_user SET ai_allowed = ?, updated_at = NOW() WHERE id = ?`, aiAllowed, id)
	if err != nil {
		log.Printf("Failed to update user AI access: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "更新用户AI权限成功",
		Data:    nil,
	})
}

func getAIEmployeeSubjectCode(db *sql.DB, userID int) (string, string) {
	var employeeID, username, realName sql.NullString
	_ = db.QueryRow(`SELECT employee_id, username, real_name FROM base_manage_user WHERE id = ? LIMIT 1`, userID).Scan(&employeeID, &username, &realName)
	code := ""
	if employeeID.Valid && strings.TrimSpace(employeeID.String) != "" {
		code = strings.TrimSpace(employeeID.String)
	} else if username.Valid {
		code = strings.TrimSpace(username.String)
	}
	name := ""
	if realName.Valid && strings.TrimSpace(realName.String) != "" {
		name = realName.String
	} else if username.Valid {
		name = username.String
	}
	return code, name
}

func getAIPatientSubjectCode(db *sql.DB, patientID int, phone string) (string, string) {
	var patientCode, name sql.NullString
	if patientID > 0 {
		_ = db.QueryRow(`SELECT patient_code, name FROM detect_patient WHERE id = ? LIMIT 1`, patientID).Scan(&patientCode, &name)
	} else if strings.TrimSpace(phone) != "" {
		_ = db.QueryRow(`SELECT patient_code, name FROM detect_patient WHERE phone = ? AND is_active = 1 LIMIT 1`, phone).Scan(&patientCode, &name)
	}
	return strings.TrimSpace(patientCode.String), name.String
}

func IsAIBlacklisted(db *sql.DB, subjectType, subjectCode string) bool {
	if db == nil || strings.TrimSpace(subjectCode) == "" {
		return false
	}
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM setting_ai_blacklist WHERE subject_type = ? AND subject_code = ?`, subjectType, subjectCode).Scan(&count)
	return err == nil && count > 0
}

func HandleListAIBlacklist(c *app.RequestContext, db *sql.DB) {
	subjectType := c.Query("subject_type")
	query := `SELECT id, subject_type, subject_code, subject_name, reason, created_by, created_at, updated_at
		FROM setting_ai_blacklist`
	args := []interface{}{}
	if subjectType != "" {
		query += " WHERE subject_type = ?"
		args = append(args, subjectType)
	}
	query += " ORDER BY created_at DESC"

	rows, err := db.Query(query, args...)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "获取AI黑名单失败", Data: utils.H{"error": err.Error()}})
		return
	}
	defer rows.Close()

	list := []utils.H{}
	for rows.Next() {
		var id, createdBy int
		var subjectType, subjectCode, subjectName, reason string
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &subjectType, &subjectCode, &subjectName, &reason, &createdBy, &createdAt, &updatedAt); err != nil {
			continue
		}
		list = append(list, utils.H{
			"id":           id,
			"subject_type": subjectType,
			"subject_code": subjectCode,
			"subject_name": subjectName,
			"reason":       reason,
			"created_by":   createdBy,
			"created_at":   createdAt.Format("2006-01-02T15:04:05+08:00"),
			"updated_at":   updatedAt.Format("2006-01-02T15:04:05+08:00"),
		})
	}

	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "获取AI黑名单成功", Data: utils.H{"list": list, "total": len(list)}})
}

func HandleCreateAIBlacklist(c *app.RequestContext, db *sql.DB) {
	var req struct {
		SubjectType string `json:"subject_type"`
		SubjectCode string `json:"subject_code"`
		Reason      string `json:"reason"`
	}
	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "请求参数错误", Data: utils.H{"error": err.Error()}})
		return
	}
	req.SubjectType = strings.TrimSpace(req.SubjectType)
	req.SubjectCode = strings.TrimSpace(req.SubjectCode)
	if req.SubjectType != "patient" && req.SubjectType != "employee" {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "黑名单类型必须为患者或员工", Data: nil})
		return
	}
	if req.SubjectCode == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "请输入自编编号", Data: nil})
		return
	}

	subjectName := ""
	if req.SubjectType == "patient" {
		var name sql.NullString
		if err := db.QueryRow(`SELECT name FROM detect_patient WHERE patient_code = ? LIMIT 1`, req.SubjectCode).Scan(&name); err != nil {
			c.JSON(consts.StatusNotFound, ApiResponse{Code: 404, Success: false, Message: "患者编号不存在", Data: nil})
			return
		}
		subjectName = name.String
	} else {
		var name sql.NullString
		err := db.QueryRow(`SELECT COALESCE(NULLIF(real_name, ''), username) FROM base_manage_user WHERE employee_id = ? OR username = ? LIMIT 1`, req.SubjectCode, req.SubjectCode).Scan(&name)
		if err != nil {
			c.JSON(consts.StatusNotFound, ApiResponse{Code: 404, Success: false, Message: "员工编号不存在", Data: nil})
			return
		}
		subjectName = name.String
	}

	createdBy := 0
	if userID, exists := c.Get(UserIDKey); exists {
		if id, ok := userID.(int); ok {
			createdBy = id
		}
	}

	_, err := db.Exec(`INSERT INTO setting_ai_blacklist (subject_type, subject_code, subject_name, reason, created_by)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE subject_name = VALUES(subject_name), reason = VALUES(reason), updated_at = NOW()`,
		req.SubjectType, req.SubjectCode, subjectName, req.Reason, createdBy)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "保存AI黑名单失败", Data: utils.H{"error": err.Error()}})
		return
	}

	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "保存AI黑名单成功", Data: nil})
}

func HandleDeleteAIBlacklist(c *app.RequestContext, db *sql.DB) {
	subjectCode := strings.TrimSpace(c.Param("code"))
	if subjectCode == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "自编编号不能为空", Data: nil})
		return
	}
	result, err := db.Exec(`DELETE FROM setting_ai_blacklist WHERE subject_code = ?`, subjectCode)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "移出AI黑名单失败", Data: utils.H{"error": err.Error()}})
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(consts.StatusNotFound, ApiResponse{Code: 404, Success: false, Message: "黑名单记录不存在", Data: nil})
		return
	}
	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "移出AI黑名单成功", Data: nil})
}
