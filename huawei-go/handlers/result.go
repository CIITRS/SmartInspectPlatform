package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/tealeg/xlsx"
)

// FormulaModel 公式模型结构体
type FormulaModel struct {
	Id           int
	Name         string
	Description  string
	ModelType    string
	IsActive     int
	Parameters   string
	Version      string
	CancerTypeId int
	Formula      string
	ModelMode    string
}

// 从公式中提取基因符号
func extractGenesFromFormula(formula string) []string {
	// 用于存储提取的基因符号
	geneMap := make(map[string]bool)
	geneSymbols := []string{}

	// 简单的基因符号提取逻辑
	// 假设基因符号是由字母、数字和下划线组成的连续字符串
	// 且出现在公式的变量位置
	var currentGene strings.Builder
	inGene := false

	for _, char := range formula {
		if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' {
			if !inGene {
				inGene = true
				currentGene.Reset()
			}
			currentGene.WriteRune(char)
		} else {
			if inGene {
				gene := currentGene.String()
				// 检查是否是有效的基因符号（长度至少为2）
				if len(gene) >= 2 {
					geneMap[gene] = true
				}
				inGene = false
			}
		}
	}

	// 处理公式末尾的基因符号
	if inGene {
		gene := currentGene.String()
		if len(gene) >= 2 {
			geneMap[gene] = true
		}
	}

	// 将map转换为slice
	for gene := range geneMap {
		geneSymbols = append(geneSymbols, gene)
	}

	// 对基因符号进行排序
	sort.Strings(geneSymbols)

	return geneSymbols
}

// 处理获取结果列表请求
func HandleListResults(c *app.RequestContext, db *sql.DB) {
	// 获取查询参数
	detect_sampleIdStr := c.Query("detect_sample_id")

	// 构建查询语句
	query := `SELECT r.id, r.detect_sample_id, r.setting_cancer_type_id, r.model_id, r.result_data, r.status, r.created_at, r.updated_at,
				s.detect_sample_code as detect_sampleCode,
				ct.name as cancerTypeName,
				ms.model_name as modelName,
				ms.version as modelVersion,
				COALESCE(ts.name, '') as treatmentStageName
			FROM result r
			LEFT JOIN detect_sample s ON r.detect_sample_id = s.id
			LEFT JOIN setting_cancer_type ct ON r.setting_cancer_type_id = ct.id
			LEFT JOIN setting_model ms ON r.model_id = ms.id
			LEFT JOIN setting_treatment_stage ts ON s.setting_treatment_stage_id = ts.id
			WHERE 1=1`

	var args []interface{}

	// 如果提供了detect_sample_id参数，添加到查询条件
	if detect_sampleIdStr != "" {
		query += " AND r.detect_sample_id = ?"
		detect_sampleId, err := strconv.Atoi(detect_sampleIdStr)
		if err != nil {
			log.Printf("Invalid detect_sample_id: %v", err)
			c.JSON(consts.StatusOK, ApiResponse{
				Code:    200,
				Success: true,
				Message: "获取结果列表成功",
				Data:    utils.H{"list": []utils.H{}, "total": 0},
			})
			return
		}
		args = append(args, detect_sampleId)
	}

	query += " ORDER BY r.created_at DESC"

	// 执行查询
	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("Failed to query results: %v", err)
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "获取结果列表成功",
			Data:    utils.H{"list": []utils.H{}, "total": 0},
		})
		return
	}
	defer rows.Close()

	// 遍历查询结果
	var results []utils.H
	for rows.Next() {
		var id, detect_sampleId, cancerTypeId, modelId int
		var resultData, status, detect_sampleCode, cancerTypeName, modelName, modelVersion, treatmentStageName string
		var createdAt, updatedAt time.Time

		err := rows.Scan(&id, &detect_sampleId, &cancerTypeId, &modelId, &resultData, &status, &createdAt, &updatedAt, &detect_sampleCode, &cancerTypeName, &modelName, &modelVersion, &treatmentStageName)
		if err != nil {
			log.Printf("Failed to scan result: %v", err)
			continue
		}

		// 构建带版本的模型名称
		modelNameWithVersion := modelName
		if modelVersion != "" {
			modelNameWithVersion = fmt.Sprintf("%s [V%s]", modelName, modelVersion)
		}

		// 构建结果信息
		result := utils.H{
			"id":                 id,
			"detect_sampleId":    detect_sampleId,
			"detect_sampleCode":  detect_sampleCode,
			"cancerTypeId":       cancerTypeId,
			"cancerTypeName":     cancerTypeName,
			"modelId":            modelId,
			"modelName":          modelNameWithVersion,
			"resultData":         resultData,
			"status":             status,
			"createdAt":          createdAt.Format("2006-01-02T15:04:05+08:00"),
			"updatedAt":          updatedAt.Format("2006-01-02T15:04:05+08:00"),
			"treatmentStageName": treatmentStageName,
		}

		results = append(results, result)
	}

	// 检查遍历过程中是否有错误
	if err = rows.Err(); err != nil {
		log.Printf("Error iterating results: %v", err)
	}

	// 返回结果列表
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取结果列表成功",
		Data:    utils.H{"list": results, "total": len(results)},
	})
}

// 处理获取患者结果请求
func HandleGetPatientResults(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	param := c.Param("id")

	// 尝试将参数转换为整数（旧格式：数据库ID）
	patientId, err := strconv.Atoi(param)
	var query string
	var args []interface{}

	if err != nil {
		// 参数不是数字，尝试作为患者编号查询
		query = `SELECT r.id, r.detect_sample_id, r.setting_cancer_type_id, r.model_id, r.result_data, r.status, r.created_at, r.updated_at,
				s.detect_sample_code as detect_sampleCode,
				ct.name as cancerTypeName,
				ms.model_name as modelName,
				ms.version as modelVersion
			FROM result r
			LEFT JOIN detect_sample s ON r.detect_sample_id = s.id
			LEFT JOIN setting_cancer_type ct ON r.setting_cancer_type_id = ct.id
			LEFT JOIN setting_model ms ON r.model_id = ms.id
			LEFT JOIN detect_patient p ON s.patient_id = p.id
			WHERE p.patient_code = ?
			ORDER BY r.created_at DESC`
		args = []interface{}{param}
	} else {
		// 参数是数字，作为数据库ID查询（保持向后兼容）
		query = `SELECT r.id, r.detect_sample_id, r.setting_cancer_type_id, r.model_id, r.result_data, r.status, r.created_at, r.updated_at,
				s.detect_sample_code as detect_sampleCode,
				ct.name as cancerTypeName,
				ms.model_name as modelName,
				ms.version as modelVersion
			FROM result r
			LEFT JOIN detect_sample s ON r.detect_sample_id = s.id
			LEFT JOIN setting_cancer_type ct ON r.setting_cancer_type_id = ct.id
			LEFT JOIN setting_model ms ON r.model_id = ms.id
			WHERE s.patient_id = ?
			ORDER BY r.created_at DESC`
		args = []interface{}{patientId}
	}

	// 从数据库查询患者的结果列表
	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("Failed to query patient results: %v", err)
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "获取患者结果成功",
			Data:    utils.H{"results": []utils.H{}},
		})
		return
	}
	defer rows.Close()

	// 遍历查询结果
	var results []utils.H
	for rows.Next() {
		var id, detect_sampleId, cancerTypeId, modelId int
		var resultData, status, detect_sampleCode, cancerTypeName, modelName, modelVersion string
		var createdAt, updatedAt time.Time

		err := rows.Scan(&id, &detect_sampleId, &cancerTypeId, &modelId, &resultData, &status, &createdAt, &updatedAt, &detect_sampleCode, &cancerTypeName, &modelName, &modelVersion)
		if err != nil {
			log.Printf("Failed to scan patient result: %v", err)
			continue
		}

		// 构建结果信息
		result := utils.H{
			"id":                id,
			"detect_sampleId":   detect_sampleId,
			"detect_sampleCode": detect_sampleCode,
			"cancerTypeId":      cancerTypeId,
			"cancerTypeName":    cancerTypeName,
			"modelId":           modelId,
			"modelName":         modelName,
			"modelVersion":      modelVersion,
			"resultData":        resultData,
			"status":            status,
			"createdAt":         createdAt.Format("2006-01-02T15:04:05+08:00"),
			"updatedAt":         updatedAt.Format("2006-01-02T15:04:05+08:00"),
		}

		results = append(results, result)
	}

	// 检查遍历过程中是否有错误
	if err = rows.Err(); err != nil {
		log.Printf("Error iterating patient results: %v", err)
	}

	// 返回患者结果列表
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取患者结果成功",
		Data:    utils.H{"results": results},
	})
}

// 处理创建结果请求
func HandleCreateResult(c *app.RequestContext, db *sql.DB) {
	// 解析请求体
	var req struct {
		SampleId     int    `json:"detect_sample_id" binding:"required"`
		CancerTypeId int    `json:"setting_cancer_type_id" binding:"required"`
		ModelId      int    `json:"model_id" binding:"required"`
		ResultData   string `json:"result_data" binding:"required"`
		Status       string `json:"status" binding:"required"`
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

	// 插入结果到数据库
	result, err := db.Exec(`INSERT INTO result (detect_sample_id, setting_cancer_type_id, model_id, result_data, status, created_at, updated_at) 
					VALUES (?, ?, ?, ?, ?, NOW(), NOW())`,
		req.SampleId, req.CancerTypeId, req.ModelId, req.ResultData, req.Status)
	if err != nil {
		log.Printf("Failed to create result: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 获取插入的结果ID
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

	// 返回创建的结果ID
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "创建结果成功",
		Data:    utils.H{"id": id},
	})
}

// 处理检查现有结果请求
func HandleCheckExistingResults(c *app.RequestContext, db *sql.DB) {
	// 解析请求体
	var req struct {
		SampleCodes []string `json:"detect_sample_codes" binding:"required"`
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

	// 检查样本是否已存在结果
	var existingSamples []string

	if len(req.SampleCodes) > 0 {
		// 构建IN查询的占位符
		placeholders := make([]string, len(req.SampleCodes))
		args := make([]interface{}, len(req.SampleCodes))
		for i, detect_sampleCode := range req.SampleCodes {
			placeholders[i] = "?"
			args[i] = detect_sampleCode
		}

		// 批量查询样本ID和是否存在结果
		query := `SELECT s.detect_sample_code FROM detect_sample s 
				JOIN result r ON s.id = r.detect_sample_id 
				WHERE s.detect_sample_code IN (` + strings.Join(placeholders, ", ") + `)`

		// 执行查询
		rows, err := db.Query(query, args...)
		if err != nil {
			log.Printf("Failed to check existing results: %v", err)
		} else {
			defer rows.Close()
			for rows.Next() {
				var detect_sampleCode string
				if err := rows.Scan(&detect_sampleCode); err == nil {
					existingSamples = append(existingSamples, detect_sampleCode)
				}
			}
		}
	}

	// 返回结果
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "检查完成",
		Data:    utils.H{"existingSamples": existingSamples},
	})
}

// 处理导入结果请求
func HandleImportResults(c *app.RequestContext, db *sql.DB) {
	// 解析表单数据
	fileHeader, err := c.FormFile("file")
	if err != nil {
		log.Printf("Failed to get file: %v", err)
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
		log.Printf("Failed to open file: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "文件打开失败",
			Data:    nil,
		})
		return
	}
	defer file.Close()

	// 读取癌种ID和模型ID
	cancerTypeIdStr := c.PostForm("cancerTypeId")
	modelIdStr := c.PostForm("modelId")

	cancerTypeId, err := strconv.Atoi(cancerTypeIdStr)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的癌种ID",
			Data:    nil,
		})
		return
	}

	modelId, err := strconv.Atoi(modelIdStr)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的模型ID",
			Data:    nil,
		})
		return
	}

	// 读取Excel文件
	xlFile, err := xlsx.OpenReaderAt(file, fileHeader.Size)
	if err != nil {
		log.Printf("Failed to open Excel file: %v", err)
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "Excel文件格式错误",
			Data:    nil,
		})
		return
	}

	// 获取第一个工作表
	sheet := xlFile.Sheets[0]

	// 解析Excel数据
	var nonExistentSamples []string
	var existingResultSamples []string
	var updatedSamples []string
	var totalSamples int
	var successSamples []string
	var skippedSamples []string

	// 读取用户选择的需要覆盖的样本
	overrideSamplesStr := c.PostForm("override_detect_samples")
	var overrideSamples []string
	if overrideSamplesStr != "" {
		// 尝试解析JSON格式的覆盖样本列表
		err := json.Unmarshal([]byte(overrideSamplesStr), &overrideSamples)
		if err != nil {
			// 如果JSON解析失败，尝试使用逗号分隔解析
			overrideSamples = strings.Split(overrideSamplesStr, ",")
		}
	}

	// 收集所有样本编号
	var allSampleCodes []string
	for i, row := range sheet.Rows {
		// 跳过表头
		if i == 0 {
			continue
		}
		// 读取行数据
		if len(row.Cells) < 2 {
			continue
		}
		detect_sampleCode := row.Cells[0].String()
		allSampleCodes = append(allSampleCodes, detect_sampleCode)
	}

	// 批量查询样本ID和是否存在结果
	detect_sampleMap := make(map[string]int)   // 样本编号 -> 样本ID
	existingSampleMap := make(map[string]bool) // 样本编号 -> 是否存在结果

	if len(allSampleCodes) > 0 {
		// 构建IN查询的占位符
		placeholders := make([]string, len(allSampleCodes))
		args := make([]interface{}, len(allSampleCodes))
		for i, detect_sampleCode := range allSampleCodes {
			placeholders[i] = "?"
			args[i] = detect_sampleCode
		}

		// 批量查询样本ID
		detect_sampleQuery := `SELECT detect_sample_code, id FROM detect_sample WHERE detect_sample_code IN (` + strings.Join(placeholders, ", ") + `)`
		detect_sampleRows, err := db.Query(detect_sampleQuery, args...)
		if err == nil {
			defer detect_sampleRows.Close()
			for detect_sampleRows.Next() {
				var detect_sampleCode string
				var detect_sampleId int
				if err := detect_sampleRows.Scan(&detect_sampleCode, &detect_sampleId); err == nil {
					detect_sampleMap[detect_sampleCode] = detect_sampleId
				}
			}
		}

		// 批量查询存在结果的样本
		if len(detect_sampleMap) > 0 {
			// 提取所有样本ID
			var detect_sampleIds []int
			var detect_sampleIdToCode map[int]string = make(map[int]string)
			for code, id := range detect_sampleMap {
				detect_sampleIds = append(detect_sampleIds, id)
				detect_sampleIdToCode[id] = code
			}

			// 构建样本ID的IN查询
			idPlaceholders := make([]string, len(detect_sampleIds))
			idArgs := make([]interface{}, len(detect_sampleIds))
			for i, id := range detect_sampleIds {
				idPlaceholders[i] = "?"
				idArgs[i] = id
			}

			// 查询存在结果的样本ID
			resultQuery := `SELECT detect_sample_id FROM result WHERE detect_sample_id IN (` + strings.Join(idPlaceholders, ", ") + `) GROUP BY detect_sample_id`
			resultRows, err := db.Query(resultQuery, idArgs...)
			if err == nil {
				defer resultRows.Close()
				for resultRows.Next() {
					var detect_sampleId int
					if err := resultRows.Scan(&detect_sampleId); err == nil {
						if code, ok := detect_sampleIdToCode[detect_sampleId]; ok {
							existingSampleMap[code] = true
						}
					}
				}
			}
		}
	}

	for i, row := range sheet.Rows {
		// 跳过表头
		if i == 0 {
			continue
		}

		// 读取行数据
		if len(row.Cells) < 2 {
			continue
		}

		totalSamples++
		detect_sampleCode := row.Cells[0].String()

		// 构建包含所有基因数据的resultData
		var resultDataMap = make(map[string]interface{})
		resultDataMap["detect_sample_code"] = detect_sampleCode

		// 构建基因数据
		var geneData = make(map[string]string)
		for i := 1; i < len(row.Cells); i++ {
			// 获取基因名称（从表头获取）
			geneName := sheet.Rows[0].Cells[i].String()
			// 获取基因值
			geneValue := row.Cells[i].String()
			geneData[geneName] = geneValue
		}
		resultDataMap["gene_data"] = geneData

		// 将map转换为JSON字符串
		resultDataJSON, err := json.Marshal(resultDataMap)
		if err != nil {
			log.Printf("Failed to marshal result data: %v", err)
			continue
		}
		resultData := string(resultDataJSON)

		// 根据样本编号获取样本ID和当前状态
		detect_sampleId, exists := detect_sampleMap[detect_sampleCode]
		if !exists {
			// 样本不存在
			nonExistentSamples = append(nonExistentSamples, detect_sampleCode)
			continue
		}

		// 获取样本当前状态
		var currentStatus string
		err = db.QueryRow("SELECT status FROM detect_sample WHERE id = ?", detect_sampleId).Scan(&currentStatus)
		if err != nil {
			log.Printf("Failed to get detect_sample status for code %s: %v", detect_sampleCode, err)
			nonExistentSamples = append(nonExistentSamples, detect_sampleCode)
			continue
		}

		// 检查样本是否已存在结果
		if existingSampleMap[detect_sampleCode] {
			existingResultSamples = append(existingResultSamples, detect_sampleCode)

			// 检查用户是否选择覆盖该样本
			override := false
			for _, s := range overrideSamples {
				if s == detect_sampleCode {
					override = true
					break
				}
			}

			if !override {
				// 用户选择跳过该样本
				skippedSamples = append(skippedSamples, detect_sampleCode)
				continue
			}
		}

		// 从上下文获取当前用户ID作为检测人员
		var testOperator int
		if userID, exists := c.Get("userID"); exists {
			testOperator = userID.(int)
		}

		// 检查样本是否已完成
		if currentStatus == "completed" {
			updatedSamples = append(updatedSamples, detect_sampleCode)
		}

		// 更新样本状态为检测完成，并记录检测人员和检测完成时间
		_, err = db.Exec(`UPDATE detect_sample SET status = 'completed', test_operator = ?, test_completed_at = NOW(), updated_at = NOW() WHERE id = ?`, testOperator, detect_sampleId)
		if err != nil {
			log.Printf("Failed to update detect_sample status for detect_sample %s: %v", detect_sampleCode, err)
			continue
		}

		// 先删除与样本相关的现有结果记录
		_, err = db.Exec(`DELETE FROM result WHERE detect_sample_id = ?`, detect_sampleId)
		if err != nil {
			log.Printf("Failed to delete existing results for detect_sample %s: %v", detect_sampleCode, err)
			// 继续执行，不中断整个导入过程
		}

		// 插入结果到数据库
		_, err = db.Exec(`INSERT INTO result (detect_sample_id, setting_cancer_type_id, model_id, result_data, status, created_at, updated_at) 
			VALUES (?, ?, ?, ?, 'completed', NOW(), NOW())`,
			detect_sampleId, cancerTypeId, modelId, resultData)
		if err != nil {
			log.Printf("Failed to insert result for detect_sample %s: %v", detect_sampleCode, err)
			continue
		}

		// 记录成功导入的样本
		successSamples = append(successSamples, detect_sampleCode)
	}

	// 构建返回数据
	responseData := utils.H{
		"total_detect_samples":           totalSamples,
		"success_detect_samples":         successSamples,
		"non_existent_detect_samples":    nonExistentSamples,
		"existing_result_detect_samples": existingResultSamples,
		"skipped_detect_samples":         skippedSamples,
		"updated_detect_samples":         len(updatedSamples),
	}

	// 返回成功响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "Excel导入成功",
		Data:    responseData,
	})
}

// 处理下载模板请求
func HandleDownloadTemplate(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	cancerTypeIdStr := c.Param("cancerTypeId")
	cancerTypeId, err := strconv.Atoi(cancerTypeIdStr)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的癌种ID",
			Data:    nil,
		})
		return
	}

	// 获取查询参数中的模型ID
	modelIdStr := c.Query("modelId")
	modelId, err := strconv.Atoi(modelIdStr)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的模型ID",
			Data:    nil,
		})
		return
	}

	// 查询模型信息，获取parameters字段
	var parameters string
	err = db.QueryRow(`SELECT parameters FROM setting_model WHERE id = ?`, modelId).Scan(&parameters)
	if err != nil {
		log.Printf("Failed to query model parameters: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 解析parameters字段，获取selectedGenes
	var paramsMap map[string]interface{}
	if err := json.Unmarshal([]byte(parameters), &paramsMap); err != nil {
		log.Printf("Failed to unmarshal parameters: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 获取selectedGenes数组
	var selectedGenes []int
	if genes, ok := paramsMap["selectedGenes"].([]interface{}); ok {
		for _, gene := range genes {
			if geneId, ok := gene.(float64); ok {
				selectedGenes = append(selectedGenes, int(geneId))
			}
		}
	}

	// 从基因表中查询基因名
	geneSymbols := []string{}
	if len(selectedGenes) > 0 {
		// 构建IN查询的占位符
		placeholders := make([]string, len(selectedGenes))
		args := make([]interface{}, len(selectedGenes))
		for i, geneId := range selectedGenes {
			placeholders[i] = "?"
			args[i] = geneId
		}

		// 构建ORDER BY FIELD的占位符
		fieldPlaceholders := strings.Join(placeholders, ",")

		// 执行查询
		query := fmt.Sprintf(`SELECT gene_symbol FROM setting_gene WHERE id IN (%s) ORDER BY FIELD(id, %s)`,
			strings.Join(placeholders, ","), fieldPlaceholders)

		// 构建完整的参数列表
		fullArgs := make([]interface{}, 0, len(selectedGenes)*2)
		for _, geneId := range selectedGenes {
			fullArgs = append(fullArgs, geneId)
		}
		for _, geneId := range selectedGenes {
			fullArgs = append(fullArgs, geneId)
		}

		rows, err := db.Query(query, fullArgs...)
		if err != nil {
			log.Printf("Failed to query genes: %v", err)
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code:    500,
				Success: false,
				Message: "服务器内部错误",
				Data:    utils.H{"error": err.Error()},
			})
			return
		}
		defer rows.Close()

		for rows.Next() {
			var geneSymbol string
			if err := rows.Scan(&geneSymbol); err != nil {
				log.Printf("Failed to scan gene symbol: %v", err)
				continue
			}
			geneSymbols = append(geneSymbols, geneSymbol)
		}

		// 检查遍历过程中是否有错误
		if err = rows.Err(); err != nil {
			log.Printf("Error iterating genes: %v", err)
		}
	}

	// 如果没有selectedGenes，限制查询基因数量，避免性能问题
	if len(geneSymbols) == 0 {
		// 限制最多查询1000个基因，避免性能问题
		rows, err := db.Query(`SELECT gene_symbol FROM setting_gene ORDER BY gene_symbol ASC LIMIT 1000`)
		if err != nil {
			log.Printf("Failed to query genes: %v", err)
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code:    500,
				Success: false,
				Message: "服务器内部错误",
				Data:    utils.H{"error": err.Error()},
			})
			return
		}
		defer rows.Close()

		for rows.Next() {
			var geneSymbol string
			if err := rows.Scan(&geneSymbol); err != nil {
				log.Printf("Failed to scan gene symbol: %v", err)
				continue
			}
			geneSymbols = append(geneSymbols, geneSymbol)
		}

		// 检查遍历过程中是否有错误
		if err = rows.Err(); err != nil {
			log.Printf("Error iterating genes: %v", err)
		}
	}

	// 创建Excel文件
	xlFile := xlsx.NewFile()
	sheet, err := xlFile.AddSheet("结果导入模板")
	if err != nil {
		log.Printf("Failed to create sheet: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 创建表头行
	headerRow := sheet.AddRow()
	headerRow.AddCell().Value = "样本编号"

	// 添加基因列
	for _, geneSymbol := range geneSymbols {
		headerRow.AddCell().Value = geneSymbol
	}

	// 设置响应头
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=result_import_template_%d_%d.xlsx", cancerTypeId, modelId))

	// 写入Excel文件到响应
	err = xlFile.Write(c.Response.BodyWriter())
	if err != nil {
		log.Printf("Failed to write Excel file: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}
}

// 处理获取患者结果对比请求
func HandleGetPatientResultsCompare(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	param := c.Param("id")

	// 尝试将参数转换为整数（旧格式：数据库ID）
	patientId, err := strconv.Atoi(param)
	var query string
	var args []interface{}

	if err != nil {
		// 参数不是数字，尝试作为患者编号查询
		query = `SELECT s.id, s.result_data, s.result_updated_at, s.detect_sample_code as detect_sampleCode
				FROM detect_sample s
				JOIN detect_patient p ON s.patient_id = p.id
				WHERE p.patient_code = ?
				AND s.id IN (
					SELECT detect_sample_id FROM report WHERE status = 'approved'
				)
				ORDER BY s.result_updated_at DESC`
		args = []interface{}{param}
	} else {
		// 参数是数字，作为数据库ID查询（保持向后兼容）
		query = `SELECT s.id, s.result_data, s.result_updated_at, s.detect_sample_code as detect_sampleCode
				FROM detect_sample s
				WHERE s.patient_id = ?
				AND s.id IN (
					SELECT detect_sample_id FROM report WHERE status = 'approved'
				)
				ORDER BY s.result_updated_at DESC`
		args = []interface{}{patientId}
	}

	// 从数据库查询患者的结果列表，只包含有审核通过报告的结果，按时间倒序排序
	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("Failed to query patient results: %v", err)
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "获取患者结果对比成功",
			Data:    utils.H{"results": []utils.H{}, "trend": "", "comparison": []utils.H{}},
		})
		return
	}
	defer rows.Close()

	// 查询基因的阈值信息
	thresholdMap := make(map[string]float64)
	thresholdRows, err := db.Query(`SELECT gene_symbol, threshold FROM setting_gene`)
	if err == nil {
		defer thresholdRows.Close()
		for thresholdRows.Next() {
			var geneSymbol string
			var threshold float64
			if err := thresholdRows.Scan(&geneSymbol, &threshold); err == nil {
				thresholdMap[geneSymbol] = threshold
			}
		}
	}

	// 遍历查询结果
	var results []utils.H
	var signalValues []float64
	var testDates []string
	for rows.Next() {
		var id int
		var resultData, detect_sampleCode string
		var createdAt time.Time

		err := rows.Scan(&id, &resultData, &createdAt, &detect_sampleCode)
		if err != nil {
			log.Printf("Failed to scan patient result: %v", err)
			continue
		}

		// 解析resultData，提取信号值
		var signalValue float64
		// 尝试从resultData中提取信号值
		if len(resultData) > 0 {
			var resultMap map[string]interface{}
			if err := json.Unmarshal([]byte(resultData), &resultMap); err == nil {
				// 尝试从不同可能的字段中获取信号值
				if score, ok := resultMap["score"].(float64); ok {
					signalValue = score
				} else if signalVal, ok := resultMap["signalValue"].(float64); ok {
					signalValue = signalVal
				} else if signalVal, ok := resultMap["signal_value"].(float64); ok {
					signalValue = signalVal
				}
			}
		}

		// 构建结果信息
		result := utils.H{
			"id":                id,
			"detect_sampleCode": detect_sampleCode,
			"resultData":        resultData,
			"signalValue":       signalValue,
			"testDate":          createdAt.Format("2006-01-02"),
			"createdAt":         createdAt.Format("2006-01-02T15:04:05+08:00"),
			"thresholds":        thresholdMap,
		}

		results = append(results, result)
		signalValues = append(signalValues, signalValue)
		testDates = append(testDates, createdAt.Format("2006-01-02"))
	}

	// 检查遍历过程中是否有错误
	if err = rows.Err(); err != nil {
		log.Printf("Error iterating results: %v", err)
	}

	// 分析趋势
	trend := ""
	if len(signalValues) >= 2 {
		currentValue := signalValues[0]
		previousValue := signalValues[1]
		switch calculateReportTrend(currentValue, previousValue) {
		case "↑":
			trend = "上升"
		case "↓":
			trend = "下降"
		default:
			trend = "稳定"
		}
	}

	// 生成对比数据
	var comparison []utils.H
	for i, result := range results {
		item := utils.H{
			"testDate":    result["testDate"],
			"signalValue": result["signalValue"],
			"trend":       "",
		}

		// 计算趋势
		if i > 0 {
			currentValue := result["signalValue"].(float64)
			previousValue := results[i-1]["signalValue"].(float64)
			switch calculateReportTrend(currentValue, previousValue) {
			case "↑":
				item["trend"] = "上升"
			case "↓":
				item["trend"] = "下降"
			default:
				item["trend"] = "稳定"
			}
		}

		comparison = append(comparison, item)
	}

	// 返回患者结果对比
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取患者结果对比成功",
		Data: utils.H{
			"results":      results,
			"trend":        trend,
			"comparison":   comparison,
			"testDates":    testDates,
			"signalValues": signalValues,
			"thresholds":   thresholdMap,
		},
	})
}

// HandleGenerateResult 根据模型 ID 和检测数据生成结果
func HandleGenerateResult(c *app.RequestContext, db *sql.DB) {
	// 解析请求体
	var req struct {
		ModelId  int            `json:"model_id" binding:"required"`
		SampleId int            `json:"detect_sample_id" binding:"required"`
		Data     map[string]any `json:"data" binding:"required"`
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

	// 查询模型信息
	var model FormulaModel
	if err := db.QueryRow(`SELECT id, model_name, description, model_version, is_active, parameters, version, cancer_type_id, formula, model_mode
		FROM setting_model
		WHERE id = ?`, req.ModelId).Scan(
		&model.Id, &model.Name, &model.Description, &model.ModelType,
		&model.IsActive, &model.Parameters, &model.Version,
		&model.CancerTypeId, &model.Formula, &model.ModelMode); err != nil {
		log.Printf("Failed to query model: %v", err)
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "获取模型失败",
			Data:    nil,
		})
		return
	}

	// 根据模型模式选择不同的计算方式
	var resultData string
	if model.ModelMode != "formula" || model.Formula == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "该模型没有配置公式",
			Data:    nil,
		})
		return
	}

	// 准备变量映射和阈值映射
	variables := make(map[string]float64)

	// 从请求数据中提取基因值
	for geneSymbol, value := range req.Data {
		switch v := value.(type) {
		case float64:
			variables[geneSymbol] = v
		case int:
			variables[geneSymbol] = float64(v)
		case string:
			if floatVal, err := strconv.ParseFloat(v, 64); err == nil {
				variables[geneSymbol] = floatVal
			}
		}
	}

	thresholds, err := loadModelThresholdMap(db, req.ModelId)
	if err != nil {
		log.Printf("Failed to load model thresholds: %v", err)
		thresholds = make(map[string]float64)
	}

	// 使用支持阈值的FormulaEvaluator计算结果
	evaluator := NewFormulaEvaluator(model.Formula)
	evaluator.SetThresholds(thresholds)
	evaluator.SetVariables(variables)
	score, err := evaluator.Evaluate()
	if err != nil {
		log.Printf("Failed to evaluate formula: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "公式计算失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	resultData = fmt.Sprintf(`{"score": %.1f}`, score)

	// 生成状态
	status := "generated"

	// 尝试插入结果
	result, err := db.Exec(`INSERT INTO result (detect_sample_id, setting_cancer_type_id, model_id, result_data, status, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, NOW(), NOW())`,
		req.SampleId, model.CancerTypeId, req.ModelId, resultData, status)
	if err != nil {
		log.Printf("Failed to insert result: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 获取插入结果的ID
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

	// 返回生成结果
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "生成结果成功",
		Data: utils.H{
			"id":         id,
			"resultData": resultData,
			"status":     status,
		},
	})
}

// HandleUpdateResultSignalValue 更新结果的信号值
func HandleUpdateResultSignalValue(c *app.RequestContext, db *sql.DB) {
	// 解析请求体
	var req struct {
		ResultId    interface{} `json:"resultId" binding:"required"`
		SignalValue float64     `json:"signalValue" binding:"required"`
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

	// 处理结果ID或样本编号
	var resultId int

	// 尝试将ResultId转换为整数（旧格式：数据库ID）
	switch v := req.ResultId.(type) {
	case float64:
		resultId = int(v)
	case string:
		// 尝试将字符串转换为整数
		if id, err := strconv.Atoi(v); err == nil {
			resultId = id
		} else {
			// 尝试作为样本编号查询结果ID
			err = db.QueryRow(`SELECT r.id FROM result r LEFT JOIN detect_sample s ON r.detect_sample_id = s.id WHERE s.sample_code = ?`, v).Scan(&resultId)
			if err != nil {
				c.JSON(consts.StatusBadRequest, ApiResponse{
					Code:    400,
					Success: false,
					Message: "无效的结果ID或样本编号",
					Data:    nil,
				})
				return
			}
		}
	default:
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的结果ID格式",
			Data:    nil,
		})
		return
	}

	// 检查结果是否存在
	var resultExists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM result WHERE id = ?)", resultId).Scan(&resultExists)
	if err != nil || !resultExists {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "结果不存在",
			Data:    nil,
		})
		return
	}

	// 更新result表的signalvalue字段
	_, err = db.Exec("UPDATE result SET signalvalue = ?, updated_at = NOW() WHERE id = ?", req.SignalValue, req.ResultId)
	if err != nil {
		log.Printf("Failed to update result signal value: %v", err)
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
		Message: "信号值更新成功",
		Data:    nil,
	})
}

// HandleMatchGenes 处理基因匹配请求（按样本逐个匹配）
func HandleMatchGenes(c *app.RequestContext, db *sql.DB) {
	// 解析请求体
	var req struct {
		BatchId interface{} `json:"batchId" binding:"required"`
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

	// 处理批次ID或批次编号
	var batchId int
	var err error

	// 尝试将BatchId转换为整数（旧格式：数据库ID）
	switch v := req.BatchId.(type) {
	case float64:
		batchId = int(v)
	case string:
		// 尝试将字符串转换为整数
		if id, err := strconv.Atoi(v); err == nil {
			batchId = id
		} else {
			// 尝试作为批次编号查询批次ID
			err = db.QueryRow(`SELECT id FROM detect_batch WHERE batch_code = ?`, v).Scan(&batchId)
			if err != nil {
				c.JSON(consts.StatusBadRequest, ApiResponse{
					Code:    400,
					Success: false,
					Message: "无效的批次ID或批次编号",
					Data:    nil,
				})
				return
			}
		}
	default:
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的批次ID格式",
			Data:    nil,
		})
		return
	}

	// 查询批次中的所有样本及其检测类型
	sampleRows, err := db.Query(`
		SELECT s.sample_code, s.cancer_type_id, ct.name as cancer_type_name, ct.panel_ids
		FROM detect_sample s
		LEFT JOIN setting_cancer_type ct ON s.cancer_type_id = ct.id
		WHERE s.batch_id = ?
		ORDER BY s.sample_code`, batchId)
	if err != nil {
		log.Printf("Failed to query batch samples: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}
	defer sampleRows.Close()

	// 按样本逐个匹配Panel
	var sampleMatches []utils.H
	var allBatchGenes []string

	for sampleRows.Next() {
		var sampleCode string
		var cancerTypeID sql.NullInt32
		var cancerTypeName, panelIDsStr sql.NullString

		if err := sampleRows.Scan(&sampleCode, &cancerTypeID, &cancerTypeName, &panelIDsStr); err != nil {
			continue
		}

		// 获取该样本的基因列表
		var sampleGenes []string
		var resultData string
		err := db.QueryRow(`SELECT result_data FROM detect_sample WHERE sample_code = ? AND result_data IS NOT NULL`, sampleCode).Scan(&resultData)
		if err == nil && resultData != "" {
			var resultMap map[string]interface{}
			if err := json.Unmarshal([]byte(resultData), &resultMap); err == nil {
				if geneData, ok := resultMap["gene_data"].(map[string]interface{}); ok {
					for gene := range geneData {
						sampleGenes = append(sampleGenes, gene)
					}
				} else {
					for gene, rawValue := range resultMap {
						if gene == "Sample" || gene == "sample_code" || gene == "location" || gene == "Location" || gene == "Total Events" {
							continue
						}
						switch rawValue.(type) {
						case float64, int, string:
							sampleGenes = append(sampleGenes, gene)
						}
					}
				}
			}
		}

		sort.Strings(sampleGenes)

		// 收集所有批次基因（用于批次级匹配）
		for _, gene := range sampleGenes {
			found := false
			for _, bg := range allBatchGenes {
				if bg == gene {
					found = true
					break
				}
			}
			if !found {
				allBatchGenes = append(allBatchGenes, gene)
			}
		}

		sampleGenesMap := mapFromGenes(sampleGenes)

		// 获取该样本检测类型关联的Panel
		var panelMatches []utils.H
		if panelIDsStr.Valid && panelIDsStr.String != "" {
			panelIDs := strings.Split(panelIDsStr.String, ",")
			for _, panelID := range panelIDs {
				panelID = strings.TrimSpace(panelID)
				if panelID == "" {
					continue
				}

				var panelName, geneIDsStr string
				err := db.QueryRow(`SELECT panel_name, gene_ids FROM setting_panel WHERE id = ? AND is_active = 1`, panelID).Scan(&panelName, &geneIDsStr)
				if err != nil {
					continue
				}

				// 解析Panel的基因列表
				var panelGenes []string
				if geneIDsStr != "" {
					geneIDParts := strings.Split(geneIDsStr, ",")
					for _, gid := range geneIDParts {
						gid = strings.TrimSpace(gid)
						if gid != "" {
							panelGenes = append(panelGenes, gid)
						}
					}
				}

				// 计算匹配度
				matchCount := 0
				for _, gene := range panelGenes {
					if sampleGenesMap[gene] {
						matchCount++
					}
				}

				totalGenes := len(panelGenes)
				matchRate := 0.0
				if totalGenes > 0 {
					matchRate = float64(matchCount) / float64(totalGenes)
				}

				missingGenes := diffGenes(panelGenes, sampleGenesMap)
				extraGenes := diffGenes(sampleGenes, mapFromGenes(panelGenes))

				// 确定匹配状态
				matchStatus := "insufficient"
				matchColor := "red"
				if len(missingGenes) == 0 && len(extraGenes) == 0 {
					matchStatus = "exact"
					matchColor = "green"
				} else if len(missingGenes) == 0 {
					matchStatus = "subset"
					matchColor = "orange"
				}

				panelMatches = append(panelMatches, utils.H{
					"panelId":      panelID,
					"panelName":    panelName,
					"matchCount":   matchCount,
					"totalGenes":   totalGenes,
					"matchRate":    matchRate,
					"panelGenes":   panelGenes,
					"sampleGenes":  sampleGenes,
					"missingGenes": missingGenes,
					"extraGenes":   extraGenes,
					"matchStatus":  matchStatus,
					"matchColor":   matchColor,
					"selectable":   len(missingGenes) == 0, // 允许选择基因少于检测类型的
				})
			}
		}

		// 构建样本匹配结果
		sampleMatch := utils.H{
			"sampleCode":       sampleCode,
			"cancerTypeId":     cancerTypeID.Int32,
			"cancerTypeName":   cancerTypeName.String,
			"sampleGenes":      sampleGenes,
			"panelMatches":     panelMatches,
			"hasMatchingPanel": len(panelMatches) > 0,
			"hasExactMatch":    false,
		}

		// 检查是否有完全匹配的Panel
		for _, pm := range panelMatches {
			if status, ok := pm["matchStatus"].(string); ok && status == "exact" {
				sampleMatch["hasExactMatch"] = true
				break
			}
		}

		sampleMatches = append(sampleMatches, sampleMatch)
	}

	sort.Strings(allBatchGenes)

	// 获取所有可用的检测类型（用于用户自主选择）
	var allCancerTypes []utils.H
	cancerTypeRows, err := db.Query(`SELECT id, name, panel_ids FROM setting_cancer_type WHERE is_active = 1 ORDER BY name`)
	if err == nil {
		defer cancerTypeRows.Close()
		for cancerTypeRows.Next() {
			var id int
			var name, panelIDs string
			if err := cancerTypeRows.Scan(&id, &name, &panelIDs); err == nil {
				allCancerTypes = append(allCancerTypes, utils.H{
					"id":        id,
					"name":      name,
					"panel_ids": panelIDs,
				})
			}
		}
	}

	// 返回匹配结果
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "基因匹配成功",
		Data: utils.H{
			"sampleMatches":  sampleMatches,
			"batchGenes":     allBatchGenes,
			"allCancerTypes": allCancerTypes,
		},
	})
}

func mapFromGenes(genes []string) map[string]bool {
	geneMap := make(map[string]bool, len(genes))
	for _, gene := range genes {
		geneMap[gene] = true
	}
	return geneMap
}
