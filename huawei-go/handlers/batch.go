package handlers

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/tealeg/xlsx"
)

// 批次结构体
type Batch struct {
	ID             int            `json:"id"`
	BatchCode      string         `json:"batchCode"`
	UploadToken    string         `json:"uploadToken"`
	SampleVolume   string         `json:"sampleVolume"`
	BatchStartTime sql.NullTime   `json:"batchStartTime"`
	BatchStopTime  sql.NullTime   `json:"batchStopTime"`
	SampleCount    int            `json:"sampleCount"`
	Status         string         `json:"status"`
	UploaderID     int            `json:"uploaderId"`
	UploaderName   string         `json:"uploaderName"`
	SubmitterID    sql.NullInt32  `json:"submitterId"`
	SubmitterName  sql.NullString `json:"submitterName"`
	InstrumentSn   sql.NullString `json:"instrumentSn"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
	Samples        sql.NullString `json:"samples"`
}

type duplicateNumericValue struct {
	sum   float64
	count int
}

func normalizeCSVGeneHeader(header string) string {
	header = strings.TrimSpace(header)
	switch header {
	case "B76_LOC10537103":
		return "B76_LOC105371031"
	default:
		return header
	}
}

func normalizeSampleCode(sampleCode string) string {
	return strings.TrimSpace(strings.TrimPrefix(sampleCode, "\ufeff"))
}

func setMedianDataCell(data map[string]interface{}, numericValues map[string]*duplicateNumericValue, header string, value string) {
	header = normalizeCSVGeneHeader(header)
	if header == "" || header == "Total Events" {
		return
	}
	value = strings.TrimSpace(value)

	if parsed, err := strconv.ParseFloat(value, 64); err == nil {
		stats, exists := numericValues[header]
		if !exists {
			numericValues[header] = &duplicateNumericValue{sum: parsed, count: 1}
			data[header] = value
			return
		}
		stats.sum += parsed
		stats.count++
		data[header] = strconv.FormatFloat(stats.sum/float64(stats.count), 'f', -1, 64)
		return
	}

	if _, exists := data[header]; !exists {
		data[header] = value
	}
}

// 生成批次编号
func generateBatchCode(db *sql.DB) (string, error) {
	var detect_batchCode string
	err := db.QueryRow("CALL generate_detect_batch_code(@detect_batchCode); SELECT @detect_batchCode").Scan(&detect_batchCode)
	if err != nil {
		// 如果存储过程调用失败，手动生成批次编号
		today := time.Now()
		dateStr := today.Format("20060102")
		prefix := "HWB" + dateStr

		// 查找今天的最大序号
		var seq int
		err := db.QueryRow("SELECT COALESCE(MAX(CAST(SUBSTRING(batch_code, LENGTH(?)+1) AS UNSIGNED)), 0) FROM detect_batch WHERE batch_code LIKE CONCAT(?,'%')", prefix, prefix).Scan(&seq)
		if err != nil {
			seq = 0
		}

		seq++
		detect_batchCode = fmt.Sprintf("%s%03d", prefix, seq)
	}
	return detect_batchCode, nil
}

// 解析CSV文件
func parseCSVFile(file *csv.Reader) (map[string]interface{}, []map[string]interface{}, []map[string]interface{}, error) {
	// 存储基础信息
	baseInfo := make(map[string]interface{})

	// 存储Median数据
	medianData := []map[string]interface{}{}

	// 存储Count数据
	countData := []map[string]interface{}{}

	// 状态变量
	var inMedianData bool
	var inCountData bool
	var medianHeader []string
	var countHeader []string

	// 逐行处理CSV数据
	lineNum := 1
	for {
		row, err := file.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, nil, fmt.Errorf("error reading CSV at line %d: %v", lineNum, err)
		}
		lineNum++

		if len(row) == 0 {
			continue
		}

		// 检查是否开始DataType部分
		if row[0] == "DataType:" {
			// 只有Median和Count需要处理
			if len(row) > 1 && row[1] == "Median" {
				inMedianData = true
				inCountData = false
				medianHeader = []string{}
			} else if len(row) > 1 && row[1] == "Count" {
				inMedianData = false
				inCountData = true
				countHeader = []string{}
			} else {
				// 其他DataType，跳过处理
				inMedianData = false
				inCountData = false
			}
			continue
		}

		// 处理Median数据
		if inMedianData {
			if len(medianHeader) == 0 {
				// 读取表头
				medianHeader = row
			} else {
				// 读取数据行
				data := make(map[string]interface{})
				numericValues := make(map[string]*duplicateNumericValue)
				// 标记是否有非空值
				hasNonEmptyValue := false
				for k, header := range medianHeader {
					if k < len(row) {
						// 跳过空表头列和Total Events列
						normalizedHeader := normalizeCSVGeneHeader(header)
						if normalizedHeader != "" && normalizedHeader != "Total Events" {
							value := row[k]
							setMedianDataCell(data, numericValues, normalizedHeader, value)
							if value != "" {
								hasNonEmptyValue = true
							}
						}
					}
				}
				// 只有当行中有非空值时才添加到medianData
				if hasNonEmptyValue {
					medianData = append(medianData, data)
				}
			}
			continue
		}

		// 处理Count数据
		if inCountData {
			if len(countHeader) == 0 {
				// 读取表头
				countHeader = row
			} else {
				// 读取数据行
				data := make(map[string]interface{})
				for k, header := range countHeader {
					if k < len(row) {
						// 跳过空表头列
						if header != "" {
							data[header] = row[k]
						}
					}
				}
				// 过滤掉空样本数据
				if sampleCode, ok := data["Sample"].(string); ok && sampleCode != "" {
					countData = append(countData, data)
				}
			}
			continue
		}

		// 解析基础信息
		if len(row) > 1 {
			switch strings.TrimSpace(row[0]) {
			case "SampleVolume":
				baseInfo["sampleVolume"] = strings.TrimSpace(row[1])
			case "BatchStartTime":
				baseInfo["batchStartTime"] = strings.TrimSpace(row[1])
			case "BatchStopTime":
				baseInfo["batchStopTime"] = strings.TrimSpace(row[1])

			case "Samples":
				if len(row) > 1 {
					sampleCount, err := strconv.Atoi(strings.TrimSpace(row[1]))
					if err == nil {
						baseInfo["sampleCount"] = sampleCount
					}
				}

			case "SN":
				baseInfo["SN"] = strings.TrimSpace(row[1])
			}
		}
	}

	return baseInfo, medianData, countData, nil
}

// 处理批次导入请求
func HandleBatchImport(c *app.RequestContext, db *sql.DB) {
	// 从上下文获取当前用户ID和姓名
	var uploaderID int
	var uploaderName string
	if userID, exists := c.Get("userID"); exists {
		uploaderID = userID.(int)
		// 查询用户姓名
		err := db.QueryRow("SELECT real_name FROM base_manage_user WHERE id = ?", uploaderID).Scan(&uploaderName)
		if err != nil {
			log.Printf("Failed to get uploader name: %v", err)
			uploaderName = ""
		}
	}

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

	// 读取文件内容
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // 允许可变字段数

	// 解析CSV数据
	baseInfo, medianData, countData, err := parseCSVFile(reader)
	if err != nil {
		log.Printf("Failed to parse CSV file: %v", err)
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "CSV文件解析失败",
			Data:    nil,
		})
		return
	}

	// 构建样本数据映射，用于快速查找Count数据
	sampleCountMap := make(map[string]map[string]interface{})
	for _, count := range countData {
		if sampleCode, ok := count["Sample"].(string); ok {
			sampleCode = normalizeSampleCode(sampleCode)
			count["Sample"] = sampleCode
			sampleCountMap[sampleCode] = count
		}
	}

	// 生成批次编号
	detect_batchCode, err := generateBatchCode(db)
	if err != nil {
		log.Printf("Failed to generate detect_batch code: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "生成批次编号失败",
			Data:    nil,
		})
		return
	}

	// 生成上传token
	uploadToken := fmt.Sprintf("%s_%d", detect_batchCode, time.Now().UnixNano())

	// 解析时间
	var batchStartTime, batchStopTime time.Time
	if startTimeStr, ok := baseInfo["batchStartTime"].(string); ok {
		batchStartTime, _ = time.Parse("1/2/2006 3:04:05 PM", startTimeStr)
	}
	if stopTimeStr, ok := baseInfo["batchStopTime"].(string); ok {
		batchStopTime, _ = time.Parse("1/2/2006 3:04:05 PM", stopTimeStr)
	}

	// 获取样本数量
	sampleCount := 0
	if count, ok := baseInfo["sampleCount"].(int); ok {
		sampleCount = count
	}

	// 检查detect_batch_sample表是否有cancer_type_id字段
	var exists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'detect_batch_sample' AND column_name = 'cancer_type_id')").Scan(&exists)
	if err != nil {
		log.Printf("Failed to check column existence: %v", err)
	}

	// 如果字段不存在,先添加
	if !exists {
		_, err = db.Exec("ALTER TABLE detect_batch_sample ADD COLUMN cancer_type_id INT DEFAULT 0 AFTER match_status")
		if err != nil {
			log.Printf("Failed to add cancer_type_id column: %v", err)
		}
	}

	// 开始事务
	tx, err := db.Begin()
	if err != nil {
		log.Printf("Failed to begin transaction: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	// 转换Median和Count数据为JSON
	medianDataJSON, err := json.Marshal(medianData)
	if err != nil {
		log.Printf("Failed to marshal median data: %v", err)
	}
	countDataJSON, err := json.Marshal(countData)
	if err != nil {
		log.Printf("Failed to marshal count data: %v", err)
	}

	// 插入批次信息
	result, err := tx.Exec(`INSERT INTO detect_batch (
		batch_code, upload_token, sample_volume, batch_start_time, batch_stop_time, 
		sample_count, status, uploader_id, uploader_name, instrument_sn, 
		median_data, count_data
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		detect_batchCode, uploadToken, baseInfo["sampleVolume"], batchStartTime, batchStopTime,
		sampleCount, "pending", uploaderID, uploaderName, baseInfo["SN"],
		string(medianDataJSON), string(countDataJSON))
	if err != nil {
		log.Printf("Failed to insert detect_batch: %v", err)
		tx.Rollback()
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	// 获取批次ID
	detect_batchId, err := result.LastInsertId()
	if err != nil {
		log.Printf("Failed to get detect_batch ID: %v", err)
		tx.Rollback()
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	// 处理样本数据
	successCount := 0

	// 提取所有样本号并去重
	sampleCodeMap := make(map[string]bool)
	var sampleCodes []string

	for _, data := range medianData {
		sampleCode, ok := data["Sample"].(string)
		if ok {
			sampleCode = normalizeSampleCode(sampleCode)
			data["Sample"] = sampleCode
		}
		if !ok || sampleCode == "" || sampleCode == "H" {
			continue
		}
		if !sampleCodeMap[sampleCode] {
			sampleCodeMap[sampleCode] = true
			sampleCodes = append(sampleCodes, sampleCode)
		}
	}

	// 批量插入 detect_batch_sample 表
	if len(sampleCodes) > 0 {
		// 构建批量插入语句
		query := "INSERT INTO detect_batch_sample (batch_id, batch_code, sample_code, match_status) VALUES "
		var args []interface{}
		var placeholders []string

		for _, sampleCode := range sampleCodes {
			placeholders = append(placeholders, "(?, ?, ?, ?)")
			args = append(args, detect_batchId, detect_batchCode, sampleCode, 0)
		}

		query += strings.Join(placeholders, ", ")
		_, err = tx.Exec(query, args...)
		if err != nil {
			log.Printf("Failed to insert into detect_batch_sample: %v", err)
		}
	}

	// 检查样本是否有接收时间
	// 先收集所有需要检查的样本编号
	var samplesWithoutReceiveDate []string
	for _, data := range medianData {
		sampleCode, ok := data["Sample"].(string)
		if !ok {
			continue
		}
		sampleCode = normalizeSampleCode(sampleCode)
		data["Sample"] = sampleCode

		// 排除H对照水样本
		if sampleCode == "H" {
			continue
		}

		// 查找样本并检查是否有接收时间
		var sampleId int
		var hasReceiveDate bool
		err := tx.QueryRow("SELECT s.id, s.receive_date IS NOT NULL FROM detect_sample s WHERE s.sample_code = ?", sampleCode).Scan(&sampleId, &hasReceiveDate)
		if err != nil {
			// 样本不存在，跳过（会在后续匹配检查中处理）
			continue
		}

		// 如果样本没有接收时间，记录下来
		if !hasReceiveDate {
			samplesWithoutReceiveDate = append(samplesWithoutReceiveDate, sampleCode)
		}
	}

	// 如果存在没有接收时间的样本，返回提示信息
	if len(samplesWithoutReceiveDate) > 0 {
		tx.Rollback()
		c.JSON(consts.StatusUnprocessableEntity, ApiResponse{
			Code:    422,
			Success: false,
			Message: "样本缺少接收时间，请人工选择接收时间",
			Data: utils.H{
				"samplesWithoutReceiveDate": samplesWithoutReceiveDate,
				"batchStartTime":            batchStartTime.Format("2006-01-02 15:04:05"),
				"batchStopTime":             batchStopTime.Format("2006-01-02 15:04:05"),
				"batchCode":                 detect_batchCode,
				"batchId":                   detect_batchId,
			},
		})
		return
	}

	// 处理样本匹配
	for _, data := range medianData {
		sampleCode, ok := data["Sample"].(string)
		if !ok {
			continue
		}
		sampleCode = normalizeSampleCode(sampleCode)
		data["Sample"] = sampleCode

		// 排除H对照水样本
		if sampleCode == "H" {
			continue
		}

		// 查找样本ID
		var sampleId int
		var patientId int
		var patientName string
		err := tx.QueryRow("SELECT s.id, p.id, p.name FROM detect_sample s LEFT JOIN detect_patient p ON s.patient_id = p.id WHERE s.sample_code = ?", sampleCode).Scan(&sampleId, &patientId, &patientName)
		if err != nil {
			// 样本不存在，跳过
			continue
		}

		// 更新 detect_batch_sample 表
		_, err = tx.Exec("UPDATE detect_batch_sample SET patient_id = ?, patient_name = ?, match_status = 1 WHERE batch_id = ? AND sample_code = ?",
			patientId, patientName, detect_batchId, sampleCode)
		if err != nil {
			log.Printf("Failed to update detect_batch_sample: %v", err)
		}

		// 获取对应样本的Count数据
		countData := sampleCountMap[sampleCode]

		// 构建result_data
		resultData := make(map[string]interface{})
		resultData["sample_code"] = sampleCode
		resultData["median"] = data
		resultData["count"] = countData

		// 转换为JSON字符串
		resultDataJSON, err := json.Marshal(resultData)
		if err != nil {
			continue
		}

		// 更新样本结果
		_, err = tx.Exec(`UPDATE detect_sample SET batch_id = ?, result_data = ?, sample_status = 'pending', result_updated_at = NOW() WHERE id = ?`,
			detect_batchId, string(resultDataJSON), sampleId)
		if err == nil {
			successCount++
		}
	}

	// 确定批次状态 - 所有上传的批次默认为待处理状态
	detect_batchStatus := "pending"

	// 更新批次状态
	_, err = tx.Exec("UPDATE detect_batch SET status = ? WHERE id = ?", detect_batchStatus, detect_batchId)
	if err != nil {
		log.Printf("Failed to update detect_batch status: %v", err)
	}

	// 提交事务
	if err = tx.Commit(); err != nil {
		log.Printf("Failed to commit transaction: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	// 检查是否存在未匹配的样本
	var hasUnmatchedSamples bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM detect_batch_sample WHERE batch_id = ? AND match_status = 0)", detect_batchId).Scan(&hasUnmatchedSamples)
	if err != nil {
		log.Printf("Failed to check for unmatched samples: %v", err)
		hasUnmatchedSamples = false
	}

	// 检查对照样本的基因表达量
	hasHighExpression := false
	for _, data := range medianData {
		sampleCode, ok := data["Sample"].(string)
		if ok && sampleCode == "H" {
			// 检查所有基因的表达量
			for key, value := range data {
				if key != "Sample" && key != "sample_code" && key != "location" && key != "Location" {
					if strValue, ok := value.(string); ok {
						if floatValue, err := strconv.ParseFloat(strValue, 64); err == nil && floatValue > 100 {
							hasHighExpression = true
							break
						}
					}
				}
			}
			if hasHighExpression {
				break
			}
		}
	}

	// 检查样本的磁珠数
	hasLowBeadCount := false
	for _, data := range countData {
		sampleCode, ok := data["Sample"].(string)
		if ok && sampleCode != "H" {
			// 检查所有基因的磁珠数
			for key, value := range data {
				if key != "Sample" && key != "sample_code" && key != "location" && key != "Location" && key != "Total Events" {
					if strValue, ok := value.(string); ok {
						if floatValue, err := strconv.ParseFloat(strValue, 64); err == nil && floatValue < 10 {
							hasLowBeadCount = true
							break
						}
					}
				}
			}
			if hasLowBeadCount {
				break
			}
		}
	}

	// 自动匹配检测类型并保存到数据库（按样本逐个匹配）
	autoMatchedCancerTypeName := autoMatchAndSaveCancerType(db, detect_batchId, medianData)

	// 自动按样本匹配检测类型中的Panel（必须在检测类型保存后调用）
	samplePanelMatches := matchSamplePanels(db, detect_batchId, medianData)

	// 构建校验结果
	validationResult := utils.H{
		"hasUnmatchedSamples":   hasUnmatchedSamples,
		"hasHighExpression":     hasHighExpression,
		"hasLowBeadCount":       hasLowBeadCount,
		"status":                "pending",
		"autoMatchedCancerType": autoMatchedCancerTypeName,
	}

	// 确定校验状态
	if hasUnmatchedSamples {
		validationResult["status"] = "error"
	} else if hasHighExpression || hasLowBeadCount {
		validationResult["status"] = "warning"
	} else {
		validationResult["status"] = "success"
	}

	// 返回结果
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "批次导入成功",
		Data: utils.H{
			"batchCode":        detect_batchCode,
			"batchId":          detect_batchId,
			"successCount":     successCount,
			"totalCount":       len(medianData),
			"sampleCount":      sampleCount,
			"batchStartTime":   batchStartTime.Format("2006-01-02 15:04:05"),
			"batchStopTime":    batchStopTime.Format("2006-01-02 15:04:05"),
			"status":           detect_batchStatus,
			"validation":       validationResult,
			"samplePanelMatch": samplePanelMatches,
		},
	})
}

// autoMatchAndSaveCancerType 按样本逐个匹配检测类型并保存到数据库
func autoMatchAndSaveCancerType(db *sql.DB, batchId int64, medianData []map[string]interface{}) string {
	// 第一步：查询所有Panel信息，缓存结果
	type PanelInfo struct {
		ID    int
		Name  string
		Genes []string
	}
	var allPanels []PanelInfo

	panelRows, err := db.Query(`SELECT id, panel_name, gene_ids FROM setting_panel WHERE is_active = 1`)
	if err != nil {
		log.Printf("autoMatchAndSaveCancerType: Failed to query panels: %v", err)
		return ""
	}
	defer panelRows.Close()

	for panelRows.Next() {
		var id int
		var name, geneIDsStr string
		if err := panelRows.Scan(&id, &name, &geneIDsStr); err != nil {
			continue
		}

		var genes []string
		if geneIDsStr != "" {
			geneIDParts := strings.Split(geneIDsStr, ",")
			for _, gid := range geneIDParts {
				gid = strings.TrimSpace(gid)
				if gid != "" {
					var geneSymbol string
					err := db.QueryRow("SELECT gene_symbol FROM setting_gene WHERE id = ?", gid).Scan(&geneSymbol)
					if err == nil && geneSymbol != "" {
						genes = append(genes, geneSymbol)
					} else {
						genes = append(genes, gid)
					}
				}
			}
		}

		allPanels = append(allPanels, PanelInfo{
			ID:    id,
			Name:  name,
			Genes: genes,
		})
	}

	if len(allPanels) == 0 {
		log.Printf("autoMatchAndSaveCancerType: No active panels found")
		return ""
	}

	// 第二步：查询所有检测类型及其关联的Panel
	type CancerTypeInfo struct {
		ID       int
		Name     string
		PanelIDs []int
	}
	var cancerTypes []CancerTypeInfo

	cancerTypeRows, err := db.Query(`SELECT id, name, panel_ids FROM setting_cancer_type WHERE is_active = 1`)
	if err != nil {
		log.Printf("autoMatchAndSaveCancerType: Failed to query cancer types: %v", err)
		return ""
	}
	defer cancerTypeRows.Close()

	for cancerTypeRows.Next() {
		var id int
		var name, panelIDsStr string
		if err := cancerTypeRows.Scan(&id, &name, &panelIDsStr); err != nil {
			continue
		}

		if panelIDsStr == "" {
			continue
		}

		panelIDParts := strings.Split(panelIDsStr, ",")
		var panelIDs []int
		for _, pid := range panelIDParts {
			pid = strings.TrimSpace(pid)
			if pid != "" {
				if pID, err := strconv.Atoi(pid); err == nil {
					panelIDs = append(panelIDs, pID)
				}
			}
		}

		cancerTypes = append(cancerTypes, CancerTypeInfo{
			ID:       id,
			Name:     name,
			PanelIDs: panelIDs,
		})
	}

	if len(cancerTypes) == 0 {
		log.Printf("autoMatchAndSaveCancerType: No active cancer types found")
		return ""
	}

	// 统计每个检测类型被匹配到的次数
	cancerTypeMatchCount := make(map[int]int)
	var mostMatchedCancerTypeID int
	var mostMatchedCount int

	// 按样本逐个匹配检测类型
	for _, data := range medianData {
		sampleCode, ok := data["Sample"].(string)
		if !ok || sampleCode == "" || sampleCode == "H" {
			continue
		}

		// 获取该样本的基因列表
		sampleGenes := make(map[string]bool)
		for gene, rawValue := range data {
			if gene == "Sample" || gene == "sample_code" || gene == "location" || gene == "Location" || gene == "Total Events" {
				continue
			}
			switch rawValue.(type) {
			case float64, int, string:
				geneSymbol, _ := getGeneSymbolByAnyName(db, gene)
				if geneSymbol != "" {
					sampleGenes[geneSymbol] = true
				}
			}
		}

		if len(sampleGenes) == 0 {
			continue
		}

		// 构建不区分大小写的样本基因映射
		sampleGenesLower := make(map[string]bool)
		for gene := range sampleGenes {
			sampleGenesLower[strings.ToLower(gene)] = true
		}

		// 第一步：用基因匹配Panel（找出所有完全匹配的Panel）
		var matchedPanelIDs []int
		var matchedPanelNames []string
		for _, panel := range allPanels {
			if len(panel.Genes) == 0 {
				continue
			}

			// 检查Panel的所有基因是否都在样本基因中
			panelMatched := true
			for _, gene := range panel.Genes {
				if !sampleGenes[gene] && !sampleGenesLower[strings.ToLower(gene)] {
					panelMatched = false
					break
				}
			}

			if panelMatched {
				matchedPanelIDs = append(matchedPanelIDs, panel.ID)
				matchedPanelNames = append(matchedPanelNames, panel.Name)
				log.Printf("autoMatchAndSaveCancerType: Sample %s matched Panel %d (%s)", sampleCode, panel.ID, panel.Name)
			}
		}

		if len(matchedPanelIDs) == 0 {
			log.Printf("autoMatchAndSaveCancerType: Sample %s has no matched panels", sampleCode)
			continue
		}

		// 第二步：用匹配到的Panel去匹配检测类型
		var matchedCancerTypes []utils.H
		for _, ct := range cancerTypes {
			// 检查检测类型的所有Panel是否都被样本匹配到了
			allPanelsMatched := true
			for _, ctPanelID := range ct.PanelIDs {
				found := false
				for _, matchedPanelID := range matchedPanelIDs {
					if ctPanelID == matchedPanelID {
						found = true
						break
					}
				}
				if !found {
					allPanelsMatched = false
					break
				}
			}

			if allPanelsMatched {
				matchedCancerTypes = append(matchedCancerTypes, utils.H{
					"id":         ct.ID,
					"name":       ct.Name,
					"panelCount": len(ct.PanelIDs),
				})
				log.Printf("autoMatchAndSaveCancerType: Sample %s matched cancer type %d (%s)", sampleCode, ct.ID, ct.Name)
			}
		}

		// 如果找到匹配的检测类型，选择最佳匹配（Panel数量最少的）
		if len(matchedCancerTypes) > 0 {
			// 排序：优先选择Panel数量最多的检测类型，避免多Panel样本被较小检测类型提前截获。
			sort.Slice(matchedCancerTypes, func(i, j int) bool {
				return matchedCancerTypes[i]["panelCount"].(int) > matchedCancerTypes[j]["panelCount"].(int)
			})

			bestMatch := matchedCancerTypes[0]
			ctID := bestMatch["id"].(int)
			ctName := bestMatch["name"].(string)
			cancerTypeMatchCount[ctID]++

			// 更新该样本的检测类型 - detect_sample表
			_, err := db.Exec(`UPDATE detect_sample SET cancer_type_id = ? WHERE batch_id = ? AND sample_code = ?`, ctID, batchId, sampleCode)
			if err != nil {
				log.Printf("autoMatchAndSaveCancerType: Failed to update cancer type for sample %s (detect_sample): %v", sampleCode, err)
			}

			// 更新该样本的检测类型 - detect_batch_sample表
			_, err = db.Exec(`UPDATE detect_batch_sample SET cancer_type_id = ? WHERE batch_id = ? AND sample_code = ?`, ctID, batchId, sampleCode)
			if err != nil {
				log.Printf("autoMatchAndSaveCancerType: Failed to update cancer type for sample %s (detect_batch_sample): %v", sampleCode, err)
			} else {
				log.Printf("autoMatchAndSaveCancerType: Sample %s -> Cancer Type: %s (ID: %d), matched Panels: %v", sampleCode, ctName, ctID, matchedPanelNames)
			}
		} else {
			log.Printf("autoMatchAndSaveCancerType: Sample %s has no matched cancer types, matched Panels: %v", sampleCode, matchedPanelNames)
		}
	}

	// 找出匹配次数最多的检测类型作为批次级别的匹配结果
	for ctID, count := range cancerTypeMatchCount {
		if count > mostMatchedCount {
			mostMatchedCount = count
			mostMatchedCancerTypeID = ctID
		}
	}

	// 获取匹配次数最多的检测类型名称
	var matchedCancerTypeName string
	for _, ct := range cancerTypes {
		if ct.ID == mostMatchedCancerTypeID {
			matchedCancerTypeName = ct.Name
			break
		}
	}

	log.Printf("autoMatchAndSaveCancerType: Batch %d auto-matched, most frequent cancer type: %s (ID: %d, matched %d samples)",
		batchId, matchedCancerTypeName, mostMatchedCancerTypeID, mostMatchedCount)

	return matchedCancerTypeName
}

// determinedCancerType 存储确定的检测类型信息
type determinedCancerType struct {
	ID         int
	Name       string
	PanelCount int
}

// determineCancerTypeFromPanelMatches 根据Panel匹配结果确定检测类型
// 优先选择完全匹配的Panel对应的检测类型，如果有多个则选择Panel数量最少的
func determineCancerTypeFromPanelMatches(panelMatches []utils.H) determinedCancerType {
	if len(panelMatches) == 0 {
		return determinedCancerType{}
	}

	// 先收集所有完全匹配的Panel
	exactMatches := []utils.H{}
	for _, pm := range panelMatches {
		if status, ok := pm["matchStatus"].(string); ok && status == "exact" {
			exactMatches = append(exactMatches, pm)
		}
	}

	var candidates []utils.H
	if len(exactMatches) > 0 {
		candidates = exactMatches
	} else {
		// 如果没有完全匹配，选择匹配率最高的Panel
		maxMatchRate := 0.0
		for _, pm := range panelMatches {
			if rate, ok := pm["matchRate"].(float64); ok && rate > maxMatchRate {
				maxMatchRate = rate
			}
		}
		for _, pm := range panelMatches {
			if rate, ok := pm["matchRate"].(float64); ok && rate >= maxMatchRate {
				candidates = append(candidates, pm)
			}
		}
	}

	if len(candidates) == 0 {
		return determinedCancerType{}
	}

	// 从候选中选择Panel数量最少的检测类型
	// 注意：Panel匹配结果中没有直接的检测类型信息，我们需要从panel名称中推断
	// 或者返回匹配最好的Panel所属的检测类型

	// 这里我们简单处理：返回第一个候选的Panel对应的检测类型信息
	firstMatch := candidates[0]
	panelName, _ := firstMatch["panelName"].(string)

	// 从Panel名称中提取检测类型信息（简单实现）
	return determinedCancerType{
		ID:         0,
		Name:       extractCancerTypeNameFromPanelName(panelName),
		PanelCount: 1,
	}
}

// extractCancerTypeNameFromPanelName 从Panel名称中提取检测类型名称
func extractCancerTypeNameFromPanelName(panelName string) string {
	// Panel名称格式示例：F8F8早筛检查智朗-肺癌
	// 提取检测类型名称部分
	if strings.Contains(panelName, "早筛检查") {
		if strings.Contains(panelName, "肺癌") {
			return "早筛检查-肺癌"
		} else if strings.Contains(panelName, "肠癌") {
			return "早筛检查-肠癌"
		}
		return "早筛检查"
	}
	if strings.Contains(panelName, "肠癌") {
		return "肠癌"
	}
	if strings.Contains(panelName, "肺癌") {
		return "肺癌"
	}
	return panelName
}

// matchSamplePanels 按样本逐个匹配检测类型中的Panel
func matchSamplePanels(db *sql.DB, batchId int64, medianData []map[string]interface{}) []utils.H {
	var sampleMatches []utils.H

	// 查询批次中的所有样本及其检测类型
	rows, err := db.Query(`
		SELECT s.sample_code, s.cancer_type_id, ct.name as cancer_type_name, ct.panel_ids
		FROM detect_sample s
		LEFT JOIN setting_cancer_type ct ON s.cancer_type_id = ct.id
		WHERE s.batch_id = ?
		ORDER BY s.sample_code`, batchId)
	if err != nil {
		log.Printf("Failed to query batch samples for panel matching: %v", err)
		return sampleMatches
	}
	defer rows.Close()

	for rows.Next() {
		var sampleCode string
		var cancerTypeID sql.NullInt32
		var cancerTypeName, panelIDsStr sql.NullString

		if err := rows.Scan(&sampleCode, &cancerTypeID, &cancerTypeName, &panelIDsStr); err != nil {
			continue
		}

		// 获取该样本的基因列表
		var sampleGenes []string
		for _, data := range medianData {
			if sCode, ok := data["Sample"].(string); ok && sCode == sampleCode {
				for gene, rawValue := range data {
					if gene == "Sample" || gene == "sample_code" || gene == "location" || gene == "Location" || gene == "Total Events" {
						continue
					}
					switch rawValue.(type) {
					case float64, int, string:
						// 自动判断基因名称是 gene_name 还是 gene_symbol，转换为统一的 gene_symbol
						geneSymbol, _ := getGeneSymbolByAnyName(db, gene)
						sampleGenes = append(sampleGenes, geneSymbol)
					}
				}
				break
			}
		}

		sort.Strings(sampleGenes)
		// 构建不区分大小写的样本基因映射
		sampleGenesMap := make(map[string]bool, len(sampleGenes))
		sampleGenesLowerMap := make(map[string]bool, len(sampleGenes))
		for _, gene := range sampleGenes {
			sampleGenesMap[gene] = true
			sampleGenesLowerMap[strings.ToLower(gene)] = true
		}

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
							// 将基因ID转换为基因名称
							var geneSymbol string
							err := db.QueryRow("SELECT gene_symbol FROM setting_gene WHERE id = ?", gid).Scan(&geneSymbol)
							if err == nil && geneSymbol != "" {
								panelGenes = append(panelGenes, geneSymbol)
							} else {
								panelGenes = append(panelGenes, gid) // 如果找不到对应的基因名称，保留原ID
							}
						}
					}
				}

				// 计算匹配度（支持不区分大小写）
				matchCount := 0
				for _, gene := range panelGenes {
					if sampleGenesMap[gene] || sampleGenesLowerMap[strings.ToLower(gene)] {
						matchCount++
					}
				}

				totalGenes := len(panelGenes)
				matchRate := 0.0
				if totalGenes > 0 {
					matchRate = float64(matchCount) / float64(totalGenes)
				}

				// 计算缺失和额外基因（支持不区分大小写）
				missingGenes := []string{}
				for _, gene := range panelGenes {
					if !sampleGenesMap[gene] && !sampleGenesLowerMap[strings.ToLower(gene)] {
						missingGenes = append(missingGenes, gene)
					}
				}

				extraGenes := []string{}
				panelGenesMap := make(map[string]bool, len(panelGenes))
				panelGenesLowerMap := make(map[string]bool, len(panelGenes))
				for _, gene := range panelGenes {
					panelGenesMap[gene] = true
					panelGenesLowerMap[strings.ToLower(gene)] = true
				}
				for _, gene := range sampleGenes {
					if !panelGenesMap[gene] && !panelGenesLowerMap[strings.ToLower(gene)] {
						extraGenes = append(extraGenes, gene)
					}
				}

				// 确定匹配状态
				matchStatus := "insufficient"
				matchColor := "red"
				if totalGenes == 0 {
					// Panel没有定义基因，视为完全匹配
					matchStatus = "exact"
					matchColor = "green"
				} else if matchRate >= 1.0 {
					// 匹配率100%，视为完全匹配
					matchStatus = "exact"
					matchColor = "green"
				} else if len(missingGenes) == 0 && matchRate > 0 {
					// 没有缺失基因但匹配率不是100%
					matchStatus = "exact"
					matchColor = "green"
				} else if len(missingGenes) == 0 {
					// 没有缺失基因但有额外基因（样本基因包含Panel基因）
					matchStatus = "exact"
					matchColor = "green"
				}

				// 查询该Panel所属的检测类型
				var cancerTypes []string
				ctRows, err := db.Query(`SELECT name FROM setting_cancer_type WHERE panel_ids LIKE ? AND is_active = 1`, "%"+panelID+"%")
				if err == nil {
					for ctRows.Next() {
						var ctName string
						if err := ctRows.Scan(&ctName); err == nil {
							cancerTypes = append(cancerTypes, ctName)
						}
					}
					ctRows.Close()
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
					"selectable":   len(missingGenes) == 0,
					"cancerTypes":  cancerTypes,
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

		// 根据Panel匹配结果确定检测类型
		determinedCancerType := determineCancerTypeFromPanelMatches(panelMatches)
		sampleMatch["determinedCancerTypeId"] = determinedCancerType.ID
		sampleMatch["determinedCancerTypeName"] = determinedCancerType.Name
		sampleMatch["determinedCancerTypePanelCount"] = determinedCancerType.PanelCount

		sampleMatches = append(sampleMatches, sampleMatch)
	}

	return sampleMatches
}

// 处理获取批次列表请求
func HandleBatchList(c *app.RequestContext, db *sql.DB) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", c.DefaultQuery("page_size", "10")))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize
	batchCode := strings.TrimSpace(c.Query("batchCode"))
	if batchCode == "" {
		batchCode = strings.TrimSpace(c.Query("batch_code"))
	}
	patientName := strings.TrimSpace(c.Query("patientName"))
	if patientName == "" {
		patientName = strings.TrimSpace(c.Query("patient_name"))
	}
	sampleKeyword := strings.TrimSpace(c.Query("sampleKeyword"))
	if sampleKeyword == "" {
		sampleKeyword = strings.TrimSpace(c.Query("sample_keyword"))
	}
	startDate := strings.TrimSpace(c.Query("startDate"))
	if startDate == "" {
		startDate = strings.TrimSpace(c.Query("start_date"))
	}
	endDate := strings.TrimSpace(c.Query("endDate"))
	if endDate == "" {
		endDate = strings.TrimSpace(c.Query("end_date"))
	}
	where := []string{"1=1"}
	args := []interface{}{}
	joinPatient := ""
	if batchCode != "" {
		where = append(where, "b.batch_code LIKE ?")
		args = append(args, "%"+batchCode+"%")
	}
	if patientName != "" || sampleKeyword != "" {
		joinPatient = " LEFT JOIN detect_sample s ON b.id = s.batch_id LEFT JOIN detect_patient p ON s.patient_id = p.id "
	}
	if patientName != "" {
		where = append(where, "p.name LIKE ?")
		args = append(args, "%"+patientName+"%")
	}
	if sampleKeyword != "" {
		where = append(where, "s.sample_code LIKE ?")
		args = append(args, "%"+sampleKeyword+"%")
	}
	if startDate != "" {
		where = append(where, "COALESCE(b.batch_start_time, b.created_at) >= ?")
		args = append(args, startDate+" 00:00:00")
	}
	if endDate != "" {
		where = append(where, "COALESCE(b.batch_stop_time, b.batch_start_time, b.created_at) <= ?")
		args = append(args, endDate+" 23:59:59")
	}
	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := db.QueryRow("SELECT COUNT(DISTINCT b.id) FROM detect_batch b "+joinPatient+" WHERE "+whereSQL, args...).Scan(&total); err != nil {
		log.Printf("Failed to count detect_batches: %v", err)
		total = 0
	}
	queryArgs := append([]interface{}{}, args...)
	queryArgs = append(queryArgs, pageSize, offset)
	// 构建查询语句
	query := `SELECT DISTINCT b.id, b.batch_code, b.sample_volume, b.batch_start_time, b.batch_stop_time, 
		b.sample_count, b.status, b.uploader_name, b.tester_name, b.created_at 
	FROM detect_batch b ` + joinPatient + ` 
	WHERE ` + whereSQL + `
	ORDER BY b.created_at DESC
	LIMIT ? OFFSET ?`

	// 执行查询
	rows, err := db.Query(query, queryArgs...)
	if err != nil {
		log.Printf("Failed to query detect_batches: %v", err)
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "获取批次列表成功",
			Data:    utils.H{"list": []utils.H{}, "total": 0},
		})
		return
	}
	defer rows.Close()

	// 遍历查询结果
	var detect_batches []utils.H
	for rows.Next() {
		var id, sampleCount int
		var detect_batchCode, sampleVolume, status, uploaderName string
		var detect_batchStartTime, detect_batchStopTime, createdAt sql.NullTime
		var testerName sql.NullString

		err := rows.Scan(&id, &detect_batchCode, &sampleVolume, &detect_batchStartTime, &detect_batchStopTime, &sampleCount, &status, &uploaderName, &testerName, &createdAt)
		if err != nil {
			log.Printf("Failed to scan detect_batch: %v", err)
			continue
		}

		// 构建批次信息
		testerNameStr := ""
		if testerName.Valid {
			testerNameStr = testerName.String
		}
		detect_batch := utils.H{
			"id":             id,
			"batchCode":      detect_batchCode,
			"sampleVolume":   sampleVolume,
			"batchStartTime": formatNullTime(detect_batchStartTime),
			"batchStopTime":  formatNullTime(detect_batchStopTime),
			"sampleCount":    sampleCount,
			"status":         status,
			"uploaderName":   uploaderName,
			"testerName":     testerNameStr,
			"createdAt":      formatNullTime(createdAt),
		}

		detect_batches = append(detect_batches, detect_batch)
	}

	// 检查遍历过程中是否有错误
	if err = rows.Err(); err != nil {
		log.Printf("Error iterating detect_batches: %v", err)
	}

	// 返回批次列表
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取批次列表成功",
		Data:    utils.H{"list": detect_batches, "total": total},
	})
}

// 处理获取批次详情请求
func HandleBatchDetail(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	idParam := c.Param("id")
	detect_batchId, err := strconv.Atoi(idParam)
	var query string
	var args []interface{}

	if err != nil {
		// 如果不是数字，则尝试作为batch_code查询
		batchCode := idParam
		query = `SELECT id, batch_code, COALESCE(upload_token, ''), COALESCE(sample_volume, ''), batch_start_time, batch_stop_time, 
					sample_count, status, COALESCE(uploader_id, 0), COALESCE(uploader_name, ''), submitter_id, submitter_name, instrument_sn, 
					created_at, updated_at, median_data, count_data, samples 
				FROM detect_batch WHERE batch_code = ?`
		args = []interface{}{batchCode}
	} else {
		// 如果是数字，则作为ID查询（保持向后兼容）
		query = `SELECT id, batch_code, COALESCE(upload_token, ''), COALESCE(sample_volume, ''), batch_start_time, batch_stop_time, 
					sample_count, status, COALESCE(uploader_id, 0), COALESCE(uploader_name, ''), submitter_id, submitter_name, instrument_sn, 
					created_at, updated_at, median_data, count_data, samples 
				FROM detect_batch WHERE id = ?`
		args = []interface{}{detect_batchId}
	}

	// 查询批次信息
	var detect_batch Batch
	var medianDataJSON, countDataJSON sql.NullString
	var samplesJSON sql.NullString
	err = db.QueryRow(query, args...).Scan(
		&detect_batch.ID, &detect_batch.BatchCode, &detect_batch.UploadToken, &detect_batch.SampleVolume, &detect_batch.BatchStartTime, &detect_batch.BatchStopTime,
		&detect_batch.SampleCount, &detect_batch.Status, &detect_batch.UploaderID, &detect_batch.UploaderName, &detect_batch.SubmitterID, &detect_batch.SubmitterName, &detect_batch.InstrumentSn,
		&detect_batch.CreatedAt, &detect_batch.UpdatedAt, &medianDataJSON, &countDataJSON, &samplesJSON)
	if err != nil {
		log.Printf("Failed to query detect_batch: %v, idParam: %s", err, idParam)
		c.JSON(consts.StatusNotFound, ApiResponse{
			Code:    404,
			Success: false,
			Message: "批次不存在",
			Data:    nil,
		})
		return
	}

	// 查询批次中的样本结果
	var results []utils.H
	var submittedMedianData []map[string]interface{}
	submittedGeneColumns := make(map[string]bool)
	unmatchedGeneSet := make(map[string]bool)

	// 批次提交后 detect_batch_sample 会被清空，提交后的样本应从 detect_sample.batch_id 读取。
	submittedRows, submittedErr := db.Query(`
		SELECT s.id, s.sample_code, s.patient_id, COALESCE(p.patient_code, '') as patient_code, COALESCE(p.name, '') as patient_name,
			COALESCE(p.gender, '') as gender, COALESCE(p.id_card, '') as id_card,
			COALESCE(s.match_status, ''), COALESCE(s.cancer_type_id, 0), COALESCE(ct.name, ''), COALESCE(s.sample_type_id, 0), COALESCE(st.name, ''),
			COALESCE(s.treatment_stage_id, 0), COALESCE(ts.name, ''), COALESCE(s.organization, ''), COALESCE(s.report_type, 'normal'), COALESCE(s.model_id, 0), COALESCE(s.result_data, ''),
			COALESCE(s.receive_date, s.collection_date, s.sample_created_at)
		FROM detect_sample s
		LEFT JOIN detect_patient p ON s.patient_id = p.id
		LEFT JOIN setting_cancer_type ct ON s.cancer_type_id = ct.id
		LEFT JOIN setting_sample_type st ON s.sample_type_id = st.id
		LEFT JOIN setting_treatment_stage ts ON s.treatment_stage_id = ts.id
		WHERE s.batch_id = ?
			AND s.result_data IS NOT NULL
			AND TRIM(s.result_data) NOT IN ('', '{}', 'null')
		ORDER BY s.sample_code
	`, detect_batch.ID)
	if submittedErr == nil {
		defer submittedRows.Close()
		for submittedRows.Next() {
			var sampleID, patientID, cancerTypeID, sampleTypeID, treatmentStageID, modelID int
			var sampleCode, patientCode, patientName, gender, idCard, matchStatus, cancerTypeName, sampleTypeName, treatmentStageName, organization, reportType, resultData string
			var sampleCollectedAt sql.NullTime
			if err := submittedRows.Scan(&sampleID, &sampleCode, &patientID, &patientCode, &patientName, &gender, &idCard, &matchStatus, &cancerTypeID, &cancerTypeName, &sampleTypeID, &sampleTypeName, &treatmentStageID, &treatmentStageName, &organization, &reportType, &modelID, &resultData, &sampleCollectedAt); err != nil {
				log.Printf("Failed to scan submitted sample: %v", err)
				continue
			}
			if strings.TrimSpace(resultData) == "" {
				continue
			}

			result := utils.H{
				"id":                 sampleID,
				"sampleId":           sampleID,
				"sampleCode":         sampleCode,
				"patientId":          patientID,
				"patientCode":        patientCode,
				"patientName":        patientName,
				"gender":             gender,
				"patientAge":         calculateAge(idCard),
				"matchStatus":        1,
				"cancerTypeId":       cancerTypeID,
				"cancerTypeName":     cancerTypeName,
				"sampleTypeId":       sampleTypeID,
				"sampleType":         sampleTypeName,
				"treatmentStageId":   treatmentStageID,
				"treatmentStageName": treatmentStageName,
				"organization":       organization,
				"reportType":         normalizeSampleReportType(reportType),
				"reportTypeLabel":    reportTypeFullLabel(reportType),
				"modelId":            modelID,
				"selectedModelId":    modelID,
			}
			if sampleCollectedAt.Valid {
				result["sampleCollectedAt"] = sampleCollectedAt.Time.Format("2006-01-02")
			}
			if matchStatus != "" {
				result["sampleMatchStatus"] = matchStatus
			}
			results = append(results, result)

			var resultDataMap map[string]interface{}
			if err := json.Unmarshal([]byte(resultData), &resultDataMap); err != nil {
				log.Printf("Failed to parse submitted result data for sample %s: %v", sampleCode, err)
				continue
			}
			geneData, ok := resultDataMap["gene_data"].(map[string]interface{})
			if !ok {
				continue
			}
			var unmatchedGenes []string
			geneData, unmatchedGenes = normalizeGeneDataKeysToSymbolsWithMissing(db, geneData)
			for _, gene := range unmatchedGenes {
				unmatchedGeneSet[gene] = true
			}
			median := map[string]interface{}{"Sample": sampleCode}
			for gene, value := range geneData {
				median[gene] = value
				submittedGeneColumns[gene] = true
			}
			submittedMedianData = append(submittedMedianData, median)
		}
		if err := submittedRows.Err(); err != nil {
			log.Printf("Error iterating submitted samples: %v", err)
		}
	} else {
		log.Printf("Failed to query submitted samples: %v", submittedErr)
	}

	// 检查批次状态，如果是已完成状态，优先使用samples字段
	if len(results) == 0 && samplesJSON.Valid && samplesJSON.String != "" {
		// 解析samples JSON
		var samples []map[string]interface{}
		err := json.Unmarshal([]byte(samplesJSON.String), &samples)
		if err != nil {
			log.Printf("Failed to parse samples JSON: %v", err)
		} else {
			// 构建结果信息
			for i, sample := range samples {
				result := utils.H{
					"id":          i + 1, // 生成临时ID
					"sampleCode":  sample["sample_code"],
					"patientId":   sample["patient_id"],
					"patientCode": sample["patient_code"],
					"patientName": sample["patient_name"],
					"matchStatus": 1, // 已完成批次的样本都已匹配
				}
				results = append(results, result)
			}
		}
	}

	if len(results) == 0 {
		// 对于未完成的批次，查询detect_batch_sample表
		query = `SELECT s.id, s.sample_code, s.patient_id, COALESCE(p.patient_code, '') as patient_code, COALESCE(s.patient_name, '') as patient_name, s.match_status
		FROM detect_batch_sample s
		LEFT JOIN detect_patient p ON s.patient_id = p.id
		WHERE s.batch_id = ? 
		ORDER BY s.sample_code`

		rows, err := db.Query(query, detect_batch.ID)
		if err != nil {
			log.Printf("Failed to query detect_batch_sample: %v", err)
			// 如果表不存在或查询失败，返回空结果列表
			results = []utils.H{}
		} else {
			defer rows.Close()

			// 遍历查询结果
			for rows.Next() {
				var id, matchStatus int
				var patientId sql.NullInt32
				var sampleCode, patientCode, patientName string

				err := rows.Scan(&id, &sampleCode, &patientId, &patientCode, &patientName, &matchStatus)
				if err != nil {
					log.Printf("Failed to scan detect_batch_sample: %v", err)
					continue
				}

				// 处理patientId的NULL值
				patientIdValue := 0
				if patientId.Valid {
					patientIdValue = int(patientId.Int32)
				}

				// 构建结果信息
				result := utils.H{
					"id":          id,
					"sampleCode":  sampleCode,
					"patientId":   patientIdValue,
					"patientCode": patientCode,
					"patientName": patientName,
					"matchStatus": matchStatus,
				}

				results = append(results, result)
			}

			// 检查遍历过程中是否有错误
			if err = rows.Err(); err != nil {
				log.Printf("Error iterating detect_batch_sample: %v", err)
			}
		}
	}

	// 提取基因列名
	var geneColumns []string
	if len(submittedGeneColumns) > 0 {
		for gene := range submittedGeneColumns {
			geneColumns = append(geneColumns, gene)
		}
		sort.Strings(geneColumns)
	}

	// 处理instrumentSn字段
	instrumentSn := ""
	if detect_batch.InstrumentSn.Valid {
		instrumentSn = detect_batch.InstrumentSn.String
	}

	// 解析Median和Count数据
	var medianData []map[string]interface{}
	var countData []map[string]interface{}

	if len(submittedMedianData) > 0 {
		medianData = submittedMedianData
	} else if medianDataJSON.Valid && medianDataJSON.String != "" {
		err := json.Unmarshal([]byte(medianDataJSON.String), &medianData)
		if err != nil {
			log.Printf("Failed to parse median data: %v", err)
		}
	}

	for i, data := range medianData {
		var unmatchedGenes []string
		medianData[i], unmatchedGenes = normalizeGeneDataKeysToSymbolsWithMissing(db, data)
		for _, gene := range unmatchedGenes {
			unmatchedGeneSet[gene] = true
		}
	}

	if len(geneColumns) == 0 && len(medianData) > 0 {
		geneSet := make(map[string]bool)
		for _, data := range medianData {
			for key := range data {
				if key != "Sample" && key != "sample_code" && key != "location" && key != "Location" && key != "Total Events" {
					geneSet[key] = true
				}
			}
		}
		for gene := range geneSet {
			geneColumns = append(geneColumns, gene)
		}
		sort.Strings(geneColumns)
	}

	if countDataJSON.Valid && countDataJSON.String != "" {
		err := json.Unmarshal([]byte(countDataJSON.String), &countData)
		if err != nil {
			log.Printf("Failed to parse count data: %v", err)
		}
	}

	// 处理提交人信息
	submitterId := 0
	if detect_batch.SubmitterID.Valid {
		submitterId = int(detect_batch.SubmitterID.Int32)
	}
	submitterName := ""
	if detect_batch.SubmitterName.Valid {
		submitterName = detect_batch.SubmitterName.String
	}

	// 提取未匹配的样本
	missingSamples := []string{}

	// 对于已完成的批次，不应该有未匹配的样本
	if detect_batch.Status != "completed" {
		// 首先，从results中获取已匹配的样本
		matchedSamples := make(map[string]bool)
		for _, result := range results {
			if sampleCode, ok := result["sampleCode"].(string); ok {
				matchedSamples[sampleCode] = true
			}
			// 同时检查是否有matchStatus为0的样本
			if matchStatus, ok := result["matchStatus"].(int); ok && matchStatus == 0 {
				if sampleCode, ok := result["sampleCode"].(string); ok {
					missingSamples = append(missingSamples, sampleCode)
				}
			}
		}

		// 然后，从medianData中提取所有样本，检查哪些样本未匹配
		for _, data := range medianData {
			if sampleCode, ok := data["Sample"].(string); ok && sampleCode != "" && sampleCode != "H" {
				if !matchedSamples[sampleCode] {
					// 检查该样本是否已经在missingSamples中
					found := false
					for _, ms := range missingSamples {
						if ms == sampleCode {
							found = true
							break
						}
					}
					if !found {
						missingSamples = append(missingSamples, sampleCode)
					}
				}
			}
		}
	}

	// 提取对照水数据（H样本）
	controlWaterData := make(map[string]interface{})
	for _, data := range medianData {
		if sampleCode, ok := data["Sample"].(string); ok && sampleCode == "H" {
			controlWaterData["median"] = data
			break
		}
	}
	for _, data := range countData {
		if sampleCode, ok := data["Sample"].(string); ok && sampleCode == "H" {
			controlWaterData["count"] = data
			break
		}
	}

	effectiveSampleCount := detect_batch.SampleCount
	resultSampleCodes := make(map[string]bool)
	for _, result := range results {
		if sampleCode, ok := result["sampleCode"].(string); ok {
			if normalized := normalizeBatchSampleCode(sampleCode); normalized != "" {
				resultSampleCodes[normalized] = true
			}
		}
	}
	if len(resultSampleCodes) > 0 {
		effectiveSampleCount = len(resultSampleCodes)
	} else if len(medianData) > 0 {
		medianSampleCodes := make(map[string]bool)
		for _, data := range medianData {
			if normalized := normalizeBatchSampleCode(sampleCodeFromExportRow(data)); normalized != "" {
				medianSampleCodes[normalized] = true
			}
		}
		if len(medianSampleCodes) > 0 {
			effectiveSampleCount = len(medianSampleCodes)
		}
	}

	// 构建返回数据
	attachBatchReportIDs(db, detect_batch.ID, results)
	unmatchedGenes := make([]string, 0, len(unmatchedGeneSet))
	for gene := range unmatchedGeneSet {
		unmatchedGenes = append(unmatchedGenes, gene)
	}
	sort.Strings(unmatchedGenes)

	responseData := utils.H{
		"batch": utils.H{
			"id":             detect_batch.ID,
			"batchCode":      detect_batch.BatchCode,
			"uploadToken":    detect_batch.UploadToken,
			"sampleVolume":   detect_batch.SampleVolume,
			"batchStartTime": formatNullTime(detect_batch.BatchStartTime),
			"batchStopTime":  formatNullTime(detect_batch.BatchStopTime),
			"sampleCount":    effectiveSampleCount,
			"status":         detect_batch.Status,
			"uploaderId":     detect_batch.UploaderID,
			"uploaderName":   detect_batch.UploaderName,
			"submitterId":    submitterId,
			"submitterName":  submitterName,
			"instrumentSn":   instrumentSn,
			"createdAt":      detect_batch.CreatedAt.Format("2006-01-02 15:04:05"),
			"updatedAt":      detect_batch.UpdatedAt.Format("2006-01-02 15:04:05"),
		},
		"results":        results,
		"missingSamples": missingSamples,
		"geneColumns":    geneColumns,
		"unmatchedGenes": unmatchedGenes,
		"medianData":     medianData,
		"countData":      countData,
		"controlWater":   controlWaterData,
	}

	// 返回批次详情
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取批次详情成功",
		Data:    responseData,
	})
}

// attachBatchReportIDs marks only samples that already have an active report.
// The UI link itself deliberately uses sampleCode so report URLs remain stable.
func attachBatchReportIDs(db *sql.DB, batchID int, lists ...[]utils.H) {
	rows, err := db.Query(`SELECT s.sample_code, r.id
		FROM detect_sample s
		JOIN detect_report r ON r.sample_id = s.id
		WHERE s.batch_id = ? AND r.status <> 'rejected'
		ORDER BY s.sample_code,
			CASE WHEN COALESCE(r.parent_report_id, 0) = 0 THEN 0 ELSE 1 END,
			r.created_at DESC, r.id DESC`, batchID)
	if err != nil {
		log.Printf("Failed to query batch report links for batch %d: %v", batchID, err)
		return
	}
	defer rows.Close()
	reportIDs := map[string]int{}
	for rows.Next() {
		var sampleCode string
		var reportID int
		if err := rows.Scan(&sampleCode, &reportID); err != nil {
			continue
		}
		if _, exists := reportIDs[sampleCode]; !exists {
			reportIDs[sampleCode] = reportID
		}
	}
	for _, list := range lists {
		for _, sample := range list {
			sampleCode, _ := sample["sampleCode"].(string)
			if reportID := reportIDs[sampleCode]; reportID > 0 {
				sample["reportId"] = reportID
				sample["hasReport"] = true
			}
		}
	}
}

// 处理按患者搜索批次请求
func HandleBatchSearchByPatient(c *app.RequestContext, db *sql.DB) {
	// 获取查询参数
	patientName := c.Query("patientName")
	patientIdStr := c.Query("patientId")

	// 构建查询语句
	query := `SELECT DISTINCT b.id, b.batch_code, b.sample_volume, b.batch_start_time, b.batch_stop_time, 
		b.sample_count, b.status, b.uploader_name, b.tester_name, b.created_at 
	FROM detect_batch b 
	JOIN detect_sample s ON b.id = s.batch_id 
	JOIN detect_patient p ON s.patient_id = p.id 
	WHERE 1=1`

	var args []interface{}

	// 添加查询条件
	if patientName != "" {
		query += " AND p.name LIKE ?"
		args = append(args, "%"+patientName+"%")
	}

	if patientIdStr != "" {
		patientId, err := strconv.Atoi(patientIdStr)
		if err == nil {
			query += " AND p.id = ?"
			args = append(args, patientId)
		}
	}

	query += " ORDER BY b.created_at DESC"

	// 执行查询
	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("Failed to search detect_batches: %v", err)
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "搜索批次成功",
			Data:    utils.H{"list": []utils.H{}, "total": 0},
		})
		return
	}
	defer rows.Close()

	// 遍历查询结果
	var detect_batches []utils.H
	for rows.Next() {
		var id, sampleCount int
		var detect_batchCode, sampleVolume, status, uploaderName string
		var detect_batchStartTime, detect_batchStopTime, createdAt sql.NullTime
		var testerName sql.NullString

		err := rows.Scan(&id, &detect_batchCode, &sampleVolume, &detect_batchStartTime, &detect_batchStopTime, &sampleCount, &status, &uploaderName, &testerName, &createdAt)
		if err != nil {
			log.Printf("Failed to scan detect_batch: %v", err)
			continue
		}

		// 构建批次信息
		testerNameStr := ""
		if testerName.Valid {
			testerNameStr = testerName.String
		}
		detect_batch := utils.H{
			"id":             id,
			"batchCode":      detect_batchCode,
			"sampleVolume":   sampleVolume,
			"batchStartTime": formatNullTime(detect_batchStartTime),
			"batchStopTime":  formatNullTime(detect_batchStopTime),
			"sampleCount":    sampleCount,
			"status":         status,
			"uploaderName":   uploaderName,
			"testerName":     testerNameStr,
			"createdAt":      formatNullTime(createdAt),
		}

		detect_batches = append(detect_batches, detect_batch)
	}

	// 检查遍历过程中是否有错误
	if err = rows.Err(); err != nil {
		log.Printf("Error iterating detect_batches: %v", err)
	}

	// 返回搜索结果
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "搜索批次成功",
		Data:    utils.H{"list": detect_batches, "total": len(detect_batches)},
	})
}

// 处理更新批次状态请求
func HandleUpdateBatchStatus(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	param := c.Param("id")

	// 尝试将参数转换为整数（旧格式：数据库ID）
	detect_batchId, err := strconv.Atoi(param)
	if err != nil {
		// 参数不是数字，尝试作为批次编号查询批次ID
		err = db.QueryRow(`SELECT id FROM detect_batch WHERE batch_code = ?`, param).Scan(&detect_batchId)
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

	// 解析请求体
	var req struct {
		Status string `json:"status" binding:"required"`
	}

	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请求参数错误",
			Data:    nil,
		})
		return
	}

	// 更新批次状态
	_, err = db.Exec("UPDATE detect_batch SET status = ?, updated_at = NOW() WHERE id = ?", req.Status, detect_batchId)
	if err != nil {
		log.Printf("Failed to update detect_batch status: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	// 更新批次中所有样本的状态
	_, err = db.Exec("UPDATE detect_sample SET sample_status = ?, result_updated_at = NOW() WHERE batch_id = ?", req.Status, detect_batchId)
	if err != nil {
		log.Printf("Failed to update sample statuses: %v", err)
		// 继续执行，不中断整个流程
	}

	// 返回成功响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "批次状态更新成功",
		Data:    nil,
	})
}

// 处理更新批次检测类型请求
func HandleUpdateBatchCancerType(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	param := c.Param("id")

	// 尝试将参数转换为整数（旧格式：数据库ID）
	detect_batchId, err := strconv.Atoi(param)
	if err != nil {
		// 参数不是数字，尝试作为批次编号查询批次ID
		err = db.QueryRow(`SELECT id FROM detect_batch WHERE batch_code = ?`, param).Scan(&detect_batchId)
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

	// 解析请求体（兼容前端传来的字符串或数字类型 cancerTypeId）
	var rawReq struct {
		CancerTypeId interface{} `json:"cancerTypeId"`
	}

	// 优先使用 json.Unmarshal 解析 body 确保稳健，如果失败则尝试 c.Bind
	bodyBytes, _ := c.Body()
	if len(bodyBytes) > 0 {
		_ = json.Unmarshal(bodyBytes, &rawReq)
	}
	if rawReq.CancerTypeId == nil {
		if err := c.Bind(&rawReq); err != nil || rawReq.CancerTypeId == nil {
			c.JSON(consts.StatusBadRequest, ApiResponse{
				Code:    400,
				Success: false,
				Message: "请求参数错误：cancerTypeId 不能为空",
				Data:    nil,
			})
			return
		}
	}

	// 将 cancerTypeId 从 string 或 float64（JSON 默认数字类型）或 int 转换为 int
	var cancerTypeIdInt int
	switch v := rawReq.CancerTypeId.(type) {
	case float64:
		cancerTypeIdInt = int(v)
	case int:
		cancerTypeIdInt = v
	case string:
		parsed, parseErr := strconv.Atoi(v)
		if parseErr != nil {
			c.JSON(consts.StatusBadRequest, ApiResponse{
				Code:    400,
				Success: false,
				Message: "cancerTypeId 格式错误",
				Data:    nil,
			})
			return
		}
		cancerTypeIdInt = parsed
	default:
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "cancerTypeId 类型不支持",
			Data:    nil,
		})
		return
	}

	if cancerTypeIdInt <= 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "cancerTypeId 必须为正整数",
			Data:    nil,
		})
		return
	}

	req := struct{ CancerTypeId int }{CancerTypeId: cancerTypeIdInt}

	// 获取当前批次的检测类型ID
	var currentCancerTypeId int
	err = db.QueryRow(`SELECT cancer_type_id FROM detect_batch WHERE id = ?`, detect_batchId).Scan(&currentCancerTypeId)
	if err != nil {
		log.Printf("Failed to get current cancer type: %v", err)
	}

	// 如果当前有检测类型，检查是否允许调整
	if currentCancerTypeId > 0 && currentCancerTypeId != req.CancerTypeId {
		// 获取当前检测类型的Panel数量
		currentPanelCount, err := getCancerTypePanelCount(db, currentCancerTypeId)
		if err != nil {
			log.Printf("Failed to get current cancer type panel count: %v", err)
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code:    500,
				Success: false,
				Message: "服务器内部错误",
				Data:    nil,
			})
			return
		}

		// 获取目标检测类型的Panel数量
		targetPanelCount, err := getCancerTypePanelCount(db, req.CancerTypeId)
		if err != nil {
			log.Printf("Failed to get target cancer type panel count: %v", err)
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code:    500,
				Success: false,
				Message: "服务器内部错误",
				Data:    nil,
			})
			return
		}

		// 检查是否允许调整：只能调整到Panel数量小于或等于当前检测类型的检测类型
		if targetPanelCount > currentPanelCount {
			c.JSON(consts.StatusBadRequest, ApiResponse{
				Code:    400,
				Success: false,
				Message: "不允许从当前检测类型调整到Panel数量更多的检测类型",
				Data:    utils.H{"currentPanelCount": currentPanelCount, "targetPanelCount": targetPanelCount},
			})
			return
		}
	}

	// 首先检查detect_batch表是否有cancer_type_id字段
	var exists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'detect_batch' AND column_name = 'cancer_type_id')").Scan(&exists)
	if err != nil {
		log.Printf("Failed to check column existence: %v", err)
	}

	// 如果字段不存在，先添加
	if !exists {
		_, err = db.Exec("ALTER TABLE detect_batch ADD COLUMN cancer_type_id INT DEFAULT 0 AFTER instrument_sn")
		if err != nil {
			log.Printf("Failed to add cancer_type_id column: %v", err)
		}
	}

	// 批次、上传批次样本和已建档样本必须保持一致，避免详情页刷新后又读到旧值。
	tx, err := db.Begin()
	if err != nil {
		log.Printf("Failed to begin update cancer type transaction: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}
	defer tx.Rollback()

	// 更新批次检测类型
	_, err = tx.Exec("UPDATE detect_batch SET cancer_type_id = ?, updated_at = NOW() WHERE id = ?", req.CancerTypeId, detect_batchId)
	if err != nil {
		log.Printf("Failed to update detect_batch cancer_type_id: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	// 上传批次样本是批次详情的主要数据源，未建档样本也需要同步。
	_, err = tx.Exec("UPDATE detect_batch_sample SET cancer_type_id = ? WHERE batch_id = ?", req.CancerTypeId, detect_batchId)
	if err != nil {
		log.Printf("Failed to update detect_batch_sample cancer_type_id: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	// 同时更新已建档样本的检测类型。
	_, err = tx.Exec("UPDATE detect_sample SET cancer_type_id = ?, sample_updated_at = NOW() WHERE batch_id = ?", req.CancerTypeId, detect_batchId)
	if err != nil {
		log.Printf("Failed to update detect_sample cancer_type_id: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	if err = tx.Commit(); err != nil {
		log.Printf("Failed to commit update cancer type transaction: %v", err)
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
		Message: "检测类型更新成功",
		Data:    nil,
	})
}

// getCancerTypePanelCount - 获取检测类型的Panel数量
func getCancerTypePanelCount(db *sql.DB, cancerTypeId int) (int, error) {
	var panelIDsStr string
	err := db.QueryRow(`SELECT panel_ids FROM setting_cancer_type WHERE id = ? AND is_active = 1`, cancerTypeId).Scan(&panelIDsStr)
	if err != nil {
		return 0, err
	}

	if panelIDsStr == "" {
		return 0, nil
	}

	panelIDs := strings.Split(panelIDsStr, ",")
	count := 0
	for _, panelID := range panelIDs {
		if strings.TrimSpace(panelID) != "" {
			count++
		}
	}
	return count, nil
}

// 处理删除批次请求
func HandleDeleteBatch(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	param := c.Param("id")

	// 尝试将参数转换为整数（旧格式：数据库ID）
	detect_batchId, err := strconv.Atoi(param)
	if err != nil {
		// 参数不是数字，尝试作为批次编号查询批次ID
		err = db.QueryRow(`SELECT id FROM detect_batch WHERE batch_code = ?`, param).Scan(&detect_batchId)
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

	// 开始事务
	tx, err := db.Begin()
	if err != nil {
		log.Printf("Failed to begin transaction: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	// 查询批次中的样本及其报告状态
	// 检查是否有已发布的报告
	rows, err := tx.Query(`
		SELECT s.id, s.sample_code, r.status
		FROM detect_sample s
		LEFT JOIN detect_report r ON s.id = r.sample_id
		WHERE s.batch_id = ?
	`, detect_batchId)
	if err != nil {
		log.Printf("Failed to query sample report status for batch: %v", err)
		tx.Rollback()
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	// 收集样本信息和检查已发布报告
	var sampleIDs []int
	var sampleCodes []string
	publishedReportSamples := []string{}
	for rows.Next() {
		var sampleID int
		var sampleCode string
		var reportStatus sql.NullString
		if err := rows.Scan(&sampleID, &sampleCode, &reportStatus); err == nil {
			sampleIDs = append(sampleIDs, sampleID)
			sampleCodes = append(sampleCodes, sampleCode)
			// 检查是否有已发布的报告
			if reportStatus.Valid && reportStatus.String == "published" {
				publishedReportSamples = append(publishedReportSamples, sampleCode)
			}
		}
	}
	rows.Close()

	// 如果有已发布的报告，阻止删除操作
	if len(publishedReportSamples) > 0 {
		tx.Rollback()
		errorMessage := fmt.Sprintf("%s报告已发布，请退回后再删除", publishedReportSamples[0])
		if len(publishedReportSamples) > 1 {
			errorMessage = fmt.Sprintf("%s等报告已发布，请退回后再删除", strings.Join(publishedReportSamples, "、"))
		}
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: errorMessage,
			Data:    utils.H{"publishedReportSamples": publishedReportSamples},
		})
		return
	}

	// 获取关联报告的文件路径（用于后续删除文件，只获取未发布报告的文件）
	var filePaths []string
	if len(sampleIDs) > 0 {
		query := "SELECT file_path FROM detect_report WHERE sample_id IN ("
		var args []interface{}
		for i, id := range sampleIDs {
			if i > 0 {
				query += ", "
			}
			query += "?"
			args = append(args, id)
		}
		query += ") AND (status IS NULL OR status != 'published')"
		rows, err = tx.Query(query, args...)
		if err == nil {
			for rows.Next() {
				var filePath string
				if err := rows.Scan(&filePath); err == nil && filePath != "" {
					filePaths = append(filePaths, filePath)
				}
			}
			rows.Close()
		}
	}

	// 获取 detect_batch_file 表中的关联文件路径（在删除操作前执行）
	batchFileRows, err := tx.Query("SELECT file_path FROM detect_batch_file WHERE batch_id = ?", detect_batchId)
	if err != nil {
		log.Printf("Failed to query batch files: %v", err)
	} else {
		for batchFileRows.Next() {
			var filePath string
			if err := batchFileRows.Scan(&filePath); err == nil && filePath != "" {
				filePaths = append(filePaths, filePath)
			}
		}
		batchFileRows.Close()
	}

	// 删除未发布的报告记录
	if len(sampleIDs) > 0 {
		query := "DELETE FROM detect_report WHERE sample_id IN ("
		var args []interface{}
		for i, id := range sampleIDs {
			if i > 0 {
				query += ", "
			}
			query += "?"
			args = append(args, id)
		}
		query += ") AND (status IS NULL OR status != 'published')"
		_, err = tx.Exec(query, args...)
		if err != nil {
			log.Printf("Failed to delete unpublished reports: %v", err)
			tx.Rollback()
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code:    500,
				Success: false,
				Message: "服务器内部错误",
				Data:    nil,
			})
			return
		}
	}

	// 删除 detect_batch_platform_data 表记录
	_, err = tx.Exec("DELETE FROM detect_batch_platform_data WHERE batch_id = ?", detect_batchId)
	if err != nil {
		log.Printf("Failed to delete platform data: %v", err)
		tx.Rollback()
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	// 删除 detect_batch_sample 表中的关联记录
	_, err = tx.Exec("DELETE FROM detect_batch_sample WHERE batch_id = ?", detect_batchId)
	if err != nil {
		log.Printf("Failed to delete related batch samples: %v", err)
		tx.Rollback()
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	// 删除 detect_batch_file 表记录（在获取文件路径之后执行）
	_, err = tx.Exec("DELETE FROM detect_batch_file WHERE batch_id = ?", detect_batchId)
	if err != nil {
		log.Printf("Failed to delete batch files: %v", err)
		tx.Rollback()
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	// 不删除样本记录，而是更新样本状态
	// 将样本状态更新为 'received'，清空 batch_id，保留原有的 receive_date
	if len(sampleIDs) > 0 {
		query := "UPDATE detect_sample SET sample_status = 'received', batch_id = NULL, result_data = NULL, sample_updated_at = NOW() WHERE id IN ("
		var args []interface{}
		for i, id := range sampleIDs {
			if i > 0 {
				query += ", "
			}
			query += "?"
			args = append(args, id)
		}
		query += ")"
		_, err = tx.Exec(query, args...)
		if err != nil {
			log.Printf("Failed to update sample status: %v", err)
			tx.Rollback()
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code:    500,
				Success: false,
				Message: "服务器内部错误",
				Data:    nil,
			})
			return
		}
	}

	// 所有关联数据清理完成后，彻底删除批次记录
	_, err = tx.Exec("DELETE FROM detect_batch WHERE id = ?", detect_batchId)
	if err != nil {
		log.Printf("Failed to permanently delete batch: %v", err)
		tx.Rollback()
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	// 提交事务
	if err = tx.Commit(); err != nil {
		log.Printf("Failed to commit transaction: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	// 删除关联文件（在事务提交成功后执行，包括批次CSV文件和未发布报告文件）
	for _, filePath := range filePaths {
		if err := os.Remove(filePath); err != nil {
			log.Printf("Failed to delete batch/report file: %v", err)
		} else {
			log.Printf("Deleted batch/report file: %s", filePath)
		}
	}

	// 返回删除成功
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "批次已彻底删除",
		Data: utils.H{
			"sampleCount": len(sampleIDs),
			"batchId":     detect_batchId,
		},
	})
}

func HandleResetSubmittedBatch(c *app.RequestContext, db *sql.DB) {
	param := c.Param("id")
	force := c.Query("force") == "1" || strings.EqualFold(c.Query("force"), "true")
	resetBatchSamples(c, db, param, nil, force)
}

func HandlePartialResetSubmittedBatch(c *app.RequestContext, db *sql.DB) {
	param := c.Param("id")
	force := c.Query("force") == "1" || strings.EqualFold(c.Query("force"), "true")
	var req struct {
		SampleCodes []string `json:"sampleCodes"`
		Force       bool     `json:"force"`
	}
	if err := json.Unmarshal(c.Request.Body(), &req); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请求参数格式错误",
			Data:    nil,
		})
		return
	}
	resetBatchSamples(c, db, param, req.SampleCodes, force || req.Force)
}

func resetBatchSamples(c *app.RequestContext, db *sql.DB, param string, requestedSampleCodes []string, force bool) {
	detectBatchID, err := strconv.Atoi(param)
	if err != nil {
		err = db.QueryRow(`SELECT id FROM detect_batch WHERE batch_code = ?`, param).Scan(&detectBatchID)
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

	tx, err := db.Begin()
	if err != nil {
		log.Printf("Failed to begin transaction: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}
	defer tx.Rollback()

	var status string
	if err := tx.QueryRow("SELECT status FROM detect_batch WHERE id = ?", detectBatchID).Scan(&status); err != nil {
		c.JSON(consts.StatusNotFound, ApiResponse{
			Code:    404,
			Success: false,
			Message: "批次不存在",
			Data:    nil,
		})
		return
	}
	resettableStatuses := map[string]bool{
		"submitted":        true,
		"completed":        true,
		"forced_completed": true,
	}
	if !resettableStatuses[strings.TrimSpace(strings.ToLower(status))] {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "仅已提交或已生成报告的批次可执行此操作",
			Data:    nil,
		})
		return
	}

	sampleQuery := "SELECT id FROM detect_sample WHERE batch_id = ?"
	sampleArgs := []interface{}{detectBatchID}
	if len(requestedSampleCodes) > 0 {
		placeholders := make([]string, 0, len(requestedSampleCodes))
		for _, sampleCode := range requestedSampleCodes {
			sampleCode = strings.TrimSpace(sampleCode)
			if sampleCode == "" {
				continue
			}
			placeholders = append(placeholders, "?")
			sampleArgs = append(sampleArgs, sampleCode)
		}
		if len(placeholders) == 0 {
			c.JSON(consts.StatusBadRequest, ApiResponse{
				Code:    400,
				Success: false,
				Message: "请选择需要退回的样本",
				Data:    nil,
			})
			return
		}
		sampleQuery += " AND sample_code IN (" + strings.Join(placeholders, ",") + ")"
	}
	sampleRows, err := tx.Query(sampleQuery, sampleArgs...)
	if err != nil {
		log.Printf("Failed to query sample IDs for submitted batch reset: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	sampleIDs := make([]int, 0)
	for sampleRows.Next() {
		var sampleID int
		if err := sampleRows.Scan(&sampleID); err == nil {
			sampleIDs = append(sampleIDs, sampleID)
		}
	}
	sampleRows.Close()
	if len(requestedSampleCodes) > 0 && len(sampleIDs) == 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "未找到需要退回的样本",
			Data:    nil,
		})
		return
	}

	if len(sampleIDs) > 0 {
		placeholders := make([]string, 0, len(sampleIDs))
		args := make([]interface{}, 0, len(sampleIDs))
		for _, sampleID := range sampleIDs {
			placeholders = append(placeholders, "?")
			args = append(args, sampleID)
		}

		reviewedReports, err := queryReviewedReportsForSamples(tx, placeholders, args)
		if err != nil {
			log.Printf("Failed to query reviewed reports for submitted batch reset: %v", err)
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code:    500,
				Success: false,
				Message: "检查已审核报告失败",
				Data:    nil,
			})
			return
		}
		if len(reviewedReports) > 0 && !force {
			firstSampleCode := fmt.Sprint(reviewedReports[0]["sampleCode"])
			message := fmt.Sprintf("%s报告已经审核通过，是否退回该报告", firstSampleCode)
			if len(reviewedReports) > 1 {
				message = fmt.Sprintf("%s等%d份报告已经审核通过，是否退回这些报告", firstSampleCode, len(reviewedReports))
			}
			c.JSON(consts.StatusOK, ApiResponse{
				Code:    409,
				Success: false,
				Message: message,
				Data: utils.H{
					"requiresConfirmation": true,
					"reviewedReports":      reviewedReports,
				},
			})
			return
		}

		deleteReportsSQL := fmt.Sprintf("DELETE FROM detect_report WHERE sample_id IN (%s)", strings.Join(placeholders, ","))
		if _, err := tx.Exec(deleteReportsSQL, args...); err != nil {
			log.Printf("Failed to delete reports for submitted batch reset: %v", err)
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code:    500,
				Success: false,
				Message: "删除报告失败",
				Data:    nil,
			})
			return
		}

		resetSamplesSQL := fmt.Sprintf(`UPDATE detect_sample
			SET result_data = NULL,
				sample_status = CASE
					WHEN receive_date IS NOT NULL OR receive_operator IS NOT NULL THEN 'received'
					ELSE 'created'
				END,
				match_status = 'pending',
				test_operator = NULL,
				test_completed_at = NULL,
				result_updated_at = NULL,
				sample_updated_at = NOW()
			WHERE id IN (%s)`, strings.Join(placeholders, ","))
		if _, err := tx.Exec(resetSamplesSQL, args...); err != nil {
			log.Printf("Failed to reset sample report data for submitted batch: %v", err)
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code:    500,
				Success: false,
				Message: "重置样本报告数据失败",
				Data:    nil,
			})
			return
		}
	}

	if len(requestedSampleCodes) == 0 {
		if _, err := tx.Exec("UPDATE detect_batch SET status = 'pending', submitter_id = NULL, submitter_name = NULL, updated_at = NOW() WHERE id = ?", detectBatchID); err != nil {
			log.Printf("Failed to reset submitted batch status: %v", err)
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code:    500,
				Success: false,
				Message: "重置批次状态失败",
				Data:    nil,
			})
			return
		}
	} else if _, err := tx.Exec("UPDATE detect_batch SET updated_at = NOW() WHERE id = ?", detectBatchID); err != nil {
		log.Printf("Failed to touch batch after partial reset: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "更新批次失败",
			Data:    nil,
		})
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Failed to commit submitted batch reset: %v", err)
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
		Message: func() string {
			if len(requestedSampleCodes) > 0 {
				return "所选样本已退回，关联报告已清除"
			}
			return "批次已退回，批次状态和关联报告已清除"
		}(),
		Data: utils.H{
			"sampleCount": len(sampleIDs),
		},
	})
}

func queryReviewedReportsForSamples(tx *sql.Tx, placeholders []string, args []interface{}) ([]utils.H, error) {
	query := fmt.Sprintf(`
		SELECT r.id, COALESCE(s.sample_code, ''), COALESCE(r.status, ''), COALESCE(r.report_no, '')
		FROM detect_report r
		LEFT JOIN detect_sample s ON r.sample_id = s.id
		WHERE r.sample_id IN (%s)
			AND r.status IN ('reviewed', 'published')
		ORDER BY s.sample_code, r.id`, strings.Join(placeholders, ","))
	rows, err := tx.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reports := []utils.H{}
	for rows.Next() {
		var id int
		var sampleCode, status, reportNo string
		if err := rows.Scan(&id, &sampleCode, &status, &reportNo); err != nil {
			return nil, err
		}
		reports = append(reports, utils.H{
			"id":         id,
			"sampleCode": sampleCode,
			"status":     status,
			"reportNo":   reportNo,
		})
	}
	return reports, rows.Err()
}

func parseBatchExportRows(raw string) []map[string]interface{} {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	var rows []map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &rows); err == nil {
		return rows
	}
	var object map[string]map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &object); err == nil {
		sampleCodes := make([]string, 0, len(object))
		for sampleCode := range object {
			sampleCodes = append(sampleCodes, sampleCode)
		}
		sort.Strings(sampleCodes)
		rows := make([]map[string]interface{}, 0, len(sampleCodes))
		for _, sampleCode := range sampleCodes {
			row := object[sampleCode]
			if row == nil {
				row = map[string]interface{}{}
			}
			if _, exists := row["Sample"]; !exists {
				row["Sample"] = sampleCode
			}
			rows = append(rows, row)
		}
		return rows
	}
	return nil
}

func sampleCodeFromExportRow(row map[string]interface{}) string {
	for _, key := range []string{"Sample", "sample_code", "sampleCode"} {
		if value, ok := row[key]; ok {
			return strings.TrimSpace(fmt.Sprint(value))
		}
	}
	return ""
}

func normalizeBatchSampleCode(sampleCode string) string {
	sampleCode = strings.TrimSpace(sampleCode)
	if sampleCode == "" || strings.EqualFold(sampleCode, "H") {
		return ""
	}
	return sampleCode
}

func sampleCodeSliceFromSet(codes map[string]bool) []string {
	list := make([]string, 0, len(codes))
	for sampleCode := range codes {
		if normalized := normalizeBatchSampleCode(sampleCode); normalized != "" {
			list = append(list, normalized)
		}
	}
	sort.Strings(list)
	return list
}

func queryCurrentBatchSampleCodesTx(tx *sql.Tx, batchID int) (map[string]bool, error) {
	codes := map[string]bool{}
	addCode := func(sampleCode string) {
		if normalized := normalizeBatchSampleCode(sampleCode); normalized != "" {
			codes[normalized] = true
		}
	}

	rows, err := tx.Query("SELECT sample_code FROM detect_batch_sample WHERE batch_id = ?", batchID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var sampleCode string
		if err := rows.Scan(&sampleCode); err != nil {
			rows.Close()
			return nil, err
		}
		addCode(sampleCode)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if len(codes) > 0 {
		return codes, nil
	}

	rows, err = tx.Query("SELECT DISTINCT sample_code FROM detect_batch_platform_data WHERE batch_id = ?", batchID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var sampleCode string
		if err := rows.Scan(&sampleCode); err != nil {
			rows.Close()
			return nil, err
		}
		addCode(sampleCode)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if len(codes) > 0 {
		return codes, nil
	}

	var medianDataJSON sql.NullString
	if err := tx.QueryRow("SELECT median_data FROM detect_batch WHERE id = ?", batchID).Scan(&medianDataJSON); err != nil {
		return nil, err
	}
	if medianDataJSON.Valid {
		for _, row := range parseBatchExportRows(medianDataJSON.String) {
			addCode(sampleCodeFromExportRow(row))
		}
	}
	if len(codes) > 0 {
		return codes, nil
	}

	rows, err = tx.Query(`SELECT sample_code FROM detect_sample
		WHERE batch_id = ?
			AND result_data IS NOT NULL
			AND TRIM(result_data) NOT IN ('', '{}', 'null')`, batchID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var sampleCode string
		if err := rows.Scan(&sampleCode); err != nil {
			rows.Close()
			return nil, err
		}
		addCode(sampleCode)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return codes, nil
}

func queryAllBatchSampleCodesTx(tx *sql.Tx, batchID int) (map[string]bool, error) {
	codes := map[string]bool{}
	addCode := func(sampleCode string) {
		if normalized := normalizeBatchSampleCode(sampleCode); normalized != "" {
			codes[normalized] = true
		}
	}

	for _, query := range []string{
		"SELECT sample_code FROM detect_batch_sample WHERE batch_id = ?",
		"SELECT DISTINCT sample_code FROM detect_batch_platform_data WHERE batch_id = ?",
		"SELECT sample_code FROM detect_sample WHERE batch_id = ?",
	} {
		rows, err := tx.Query(query, batchID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var sampleCode string
			if err := rows.Scan(&sampleCode); err != nil {
				rows.Close()
				return nil, err
			}
			addCode(sampleCode)
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
	}

	var medianDataJSON, countDataJSON, mergedDataJSON sql.NullString
	if err := tx.QueryRow("SELECT median_data, count_data, merged_data FROM detect_batch WHERE id = ?", batchID).Scan(&medianDataJSON, &countDataJSON, &mergedDataJSON); err != nil {
		return nil, err
	}
	for _, raw := range []sql.NullString{medianDataJSON, countDataJSON, mergedDataJSON} {
		if !raw.Valid {
			continue
		}
		for _, row := range parseBatchExportRows(raw.String) {
			addCode(sampleCodeFromExportRow(row))
		}
	}
	return codes, nil
}

func queryCurrentBatchSampleCodes(db *sql.DB, batchID int) map[string]bool {
	codes := map[string]bool{}
	rows, err := db.Query("SELECT sample_code FROM detect_batch_sample WHERE batch_id = ?", batchID)
	if err == nil {
		for rows.Next() {
			var sampleCode string
			if err := rows.Scan(&sampleCode); err == nil {
				if normalized := normalizeBatchSampleCode(sampleCode); normalized != "" {
					codes[normalized] = true
				}
			}
		}
		rows.Close()
	}
	if len(codes) > 0 {
		return codes
	}
	rows, err = db.Query("SELECT DISTINCT sample_code FROM detect_batch_platform_data WHERE batch_id = ?", batchID)
	if err == nil {
		for rows.Next() {
			var sampleCode string
			if err := rows.Scan(&sampleCode); err == nil {
				if normalized := normalizeBatchSampleCode(sampleCode); normalized != "" {
					codes[normalized] = true
				}
			}
		}
		rows.Close()
	}
	if len(codes) > 0 {
		return codes
	}
	var medianDataJSON sql.NullString
	if err := db.QueryRow("SELECT median_data FROM detect_batch WHERE id = ?", batchID).Scan(&medianDataJSON); err == nil && medianDataJSON.Valid {
		for _, row := range parseBatchExportRows(medianDataJSON.String) {
			if normalized := normalizeBatchSampleCode(sampleCodeFromExportRow(row)); normalized != "" {
				codes[normalized] = true
			}
		}
	}
	if len(codes) > 0 {
		return codes
	}
	rows, err = db.Query(`SELECT sample_code FROM detect_sample
		WHERE batch_id = ?
			AND result_data IS NOT NULL
			AND TRIM(result_data) NOT IN ('', '{}', 'null')`, batchID)
	if err == nil {
		for rows.Next() {
			var sampleCode string
			if err := rows.Scan(&sampleCode); err == nil {
				if normalized := normalizeBatchSampleCode(sampleCode); normalized != "" {
					codes[normalized] = true
				}
			}
		}
		rows.Close()
	}
	return codes
}

func writeXLSXRow(sheet *xlsx.Sheet, values ...interface{}) {
	row := sheet.AddRow()
	for _, value := range values {
		cell := row.AddCell()
		switch v := value.(type) {
		case time.Time:
			if v.IsZero() {
				cell.Value = ""
			} else {
				cell.Value = v.Format("2006-01-02 15:04:05")
			}
		case sql.NullTime:
			if v.Valid {
				cell.Value = v.Time.Format("2006-01-02 15:04:05")
			}
		case sql.NullString:
			if v.Valid {
				cell.Value = v.String
			}
		default:
			if value == nil {
				cell.Value = ""
			} else {
				cell.Value = fmt.Sprint(value)
			}
		}
	}
}

func batchStatusText(status string) string {
	switch status {
	case "pending":
		return "待处理"
	case "submitted":
		return "已提交"
	case "completed", "forced_completed":
		return "已完成"
	case "rejected", "withdrawn":
		return "已退回"
	default:
		return status
	}
}

// 处理导出批次结果请求
func HandleExportBatch(c *app.RequestContext, db *sql.DB) {
	param := c.Param("id")

	detect_batchId, err := strconv.Atoi(param)
	if err != nil {
		err = db.QueryRow(`SELECT id FROM detect_batch WHERE batch_code = ?`, param).Scan(&detect_batchId)
		if err != nil {
			c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "无效的批次ID或批次编号", Data: nil})
			return
		}
	}

	var batchCode, uploadToken, sampleVolume, status, uploaderName, testerName string
	var submitterName, instrumentSn, medianDataJSON, countDataJSON, mergedDataJSON sql.NullString
	var batchStartTime, batchStopTime sql.NullTime
	var sampleCount int
	var createdAt, updatedAt time.Time
	err = db.QueryRow(`SELECT batch_code, COALESCE(upload_token, ''), COALESCE(sample_volume, ''),
		batch_start_time, batch_stop_time, sample_count, status, COALESCE(uploader_name, ''),
		COALESCE(tester_name, ''), submitter_name, instrument_sn, created_at, updated_at,
		median_data, count_data, merged_data
		FROM detect_batch WHERE id = ?`, detect_batchId).Scan(
		&batchCode, &uploadToken, &sampleVolume, &batchStartTime, &batchStopTime, &sampleCount,
		&status, &uploaderName, &testerName, &submitterName, &instrumentSn, &createdAt, &updatedAt,
		&medianDataJSON, &countDataJSON, &mergedDataJSON)
	if err != nil {
		c.JSON(consts.StatusNotFound, ApiResponse{
			Code:    404,
			Success: false,
			Message: "批次不存在",
			Data:    nil,
		})
		return
	}

	dataRows := []map[string]interface{}{}
	if mergedDataJSON.Valid {
		dataRows = parseBatchExportRows(mergedDataJSON.String)
	}
	if len(dataRows) == 0 && medianDataJSON.Valid {
		dataRows = parseBatchExportRows(medianDataJSON.String)
	}
	countRows := []map[string]interface{}{}
	if countDataJSON.Valid {
		countRows = parseBatchExportRows(countDataJSON.String)
	}
	countBySample := map[string]map[string]interface{}{}
	for _, row := range countRows {
		if sampleCode := sampleCodeFromExportRow(row); sampleCode != "" {
			countBySample[sampleCode] = row
		}
	}

	currentSampleCodes := queryCurrentBatchSampleCodes(db, detect_batchId)
	filteredRows := make([]map[string]interface{}, 0, len(dataRows))
	for _, row := range dataRows {
		sampleCode := sampleCodeFromExportRow(row)
		if sampleCode == "" || sampleCode == "H" {
			continue
		}
		if len(currentSampleCodes) > 0 && !currentSampleCodes[sampleCode] {
			continue
		}
		filteredRows = append(filteredRows, row)
		currentSampleCodes[sampleCode] = true
	}

	sampleInfo := map[string]utils.H{}
	if len(currentSampleCodes) > 0 {
		placeholders := make([]string, 0, len(currentSampleCodes))
		args := make([]interface{}, 0, len(currentSampleCodes))
		for sampleCode := range currentSampleCodes {
			placeholders = append(placeholders, "?")
			args = append(args, sampleCode)
		}
		rows, err := db.Query(`SELECT s.sample_code, COALESCE(p.patient_code, ''), COALESCE(p.name, ''),
			COALESCE(st.name, ''), COALESCE(ct.name, ''), COALESCE(ts.name, ''), COALESCE(s.sample_status, '')
			FROM detect_sample s
			LEFT JOIN detect_patient p ON s.patient_id = p.id
			LEFT JOIN setting_sample_type st ON s.sample_type_id = st.id
			LEFT JOIN setting_cancer_type ct ON s.cancer_type_id = ct.id
			LEFT JOIN setting_treatment_stage ts ON s.treatment_stage_id = ts.id
			WHERE s.sample_code IN (`+strings.Join(placeholders, ",")+`)`, args...)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var sampleCode, patientCode, patientName, sampleType, cancerType, treatmentStage, sampleStatus string
				if err := rows.Scan(&sampleCode, &patientCode, &patientName, &sampleType, &cancerType, &treatmentStage, &sampleStatus); err == nil {
					sampleInfo[sampleCode] = utils.H{
						"patientCode":    patientCode,
						"patientName":    patientName,
						"sampleType":     sampleType,
						"cancerType":     cancerType,
						"treatmentStage": treatmentStage,
						"sampleStatus":   sampleStatus,
					}
				}
			}
		}
	}

	geneSet := map[string]bool{}
	countGeneSet := map[string]bool{}
	metaKeys := map[string]bool{"Sample": true, "sample_code": true, "sampleCode": true, "location": true, "Location": true, "Total Events": true, "totalEvents": true}
	for _, row := range filteredRows {
		for key := range row {
			if !metaKeys[key] {
				geneSet[key] = true
			}
		}
	}
	for _, row := range countBySample {
		for key := range row {
			if !metaKeys[key] {
				countGeneSet[key] = true
			}
		}
	}
	genes := make([]string, 0, len(geneSet))
	for gene := range geneSet {
		genes = append(genes, gene)
	}
	sort.Strings(genes)
	countGenes := make([]string, 0, len(countGeneSet))
	for gene := range countGeneSet {
		countGenes = append(countGenes, gene)
	}
	sort.Strings(countGenes)

	xlFile := xlsx.NewFile()
	infoSheet, err := xlFile.AddSheet("批次信息")
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "创建导出文件失败", Data: nil})
		return
	}
	writeXLSXRow(infoSheet, "字段", "内容")
	writeXLSXRow(infoSheet, "批次编号", batchCode)
	writeXLSXRow(infoSheet, "状态", batchStatusText(status))
	writeXLSXRow(infoSheet, "样本数量", sampleCount)
	writeXLSXRow(infoSheet, "样本体积", sampleVolume)
	writeXLSXRow(infoSheet, "仪器编号", instrumentSn)
	writeXLSXRow(infoSheet, "上传人", uploaderName)
	writeXLSXRow(infoSheet, "检测人员", testerName)
	writeXLSXRow(infoSheet, "提交人", submitterName)
	writeXLSXRow(infoSheet, "批次开始时间", batchStartTime)
	writeXLSXRow(infoSheet, "批次结束时间", batchStopTime)
	writeXLSXRow(infoSheet, "创建时间", createdAt)
	writeXLSXRow(infoSheet, "更新时间", updatedAt)
	writeXLSXRow(infoSheet, "上传Token", uploadToken)

	resultSheet, err := xlFile.AddSheet("样本结果")
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "创建导出文件失败", Data: nil})
		return
	}
	header := []interface{}{"样本编号", "患者编号", "患者姓名", "样本类型", "检测类型", "治疗阶段", "样本状态"}
	for _, gene := range genes {
		header = append(header, gene)
	}
	for _, gene := range countGenes {
		header = append(header, "Count_"+gene)
	}
	writeXLSXRow(resultSheet, header...)
	for _, row := range filteredRows {
		sampleCode := sampleCodeFromExportRow(row)
		info := sampleInfo[sampleCode]
		values := []interface{}{
			sampleCode,
			info["patientCode"],
			info["patientName"],
			info["sampleType"],
			info["cancerType"],
			info["treatmentStage"],
			info["sampleStatus"],
		}
		for _, gene := range genes {
			values = append(values, row[gene])
		}
		countRow := countBySample[sampleCode]
		for _, gene := range countGenes {
			if countRow == nil {
				values = append(values, "")
			} else {
				values = append(values, countRow[gene])
			}
		}
		writeXLSXRow(resultSheet, values...)
	}

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=detect_batch_%s_results.xlsx", batchCode))
	if err := xlFile.Write(c.Response.BodyWriter()); err != nil {
		log.Printf("Failed to write batch export Excel: %v", err)
	}
}

// 处理应用模型请求
func HandleApplyModel(c *app.RequestContext, db *sql.DB) {
	// 解析请求体
	var req struct {
		BatchId int `json:"batchId" binding:"required"`
		ModelId int `json:"modelId" binding:"required"`
	}

	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请求参数错误",
			Data:    nil,
		})
		return
	}

	// 这里应该实现应用模型逻辑
	// 暂时返回成功
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "模型应用成功",
		Data:    nil,
	})
}

// 处理查询批次样本请求
func HandleBatchSamples(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	param := c.Param("id")

	// 尝试将参数转换为整数（旧格式：数据库ID）
	batchId, err := strconv.Atoi(param)
	if err != nil {
		// 参数不是数字，尝试作为批次编号查询批次ID
		err = db.QueryRow(`SELECT id FROM detect_batch WHERE batch_code = ?`, param).Scan(&batchId)
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

	// 查询 detect_batch_sample 表
	query := `SELECT id, batch_id, batch_code, sample_code, patient_id, patient_name, match_status, created_at, updated_at 
			FROM detect_batch_sample 
			WHERE batch_id = ? 
			ORDER BY sample_code`

	rows, err := db.Query(query, batchId)
	if err != nil {
		log.Printf("Failed to query detect_batch_sample: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}
	defer rows.Close()

	// 遍历查询结果
	var samples []utils.H
	for rows.Next() {
		var id, batchId, patientId int
		var batchCode, sampleCode, patientName string
		var matchStatus int
		var createdAt, updatedAt time.Time

		err := rows.Scan(&id, &batchId, &batchCode, &sampleCode, &patientId, &patientName, &matchStatus, &createdAt, &updatedAt)
		if err != nil {
			log.Printf("Failed to scan detect_batch_sample: %v", err)
			continue
		}

		// 构建样本信息
		sample := utils.H{
			"id":          id,
			"batchId":     batchId,
			"batchCode":   batchCode,
			"sampleCode":  sampleCode,
			"patientId":   patientId,
			"patientName": patientName,
			"matchStatus": matchStatus,
			"createdAt":   createdAt.Format("2006-01-02T15:04:05+08:00"),
			"updatedAt":   updatedAt.Format("2006-01-02T15:04:05+08:00"),
		}

		samples = append(samples, sample)
	}

	// 检查遍历过程中是否有错误
	if err = rows.Err(); err != nil {
		log.Printf("Error iterating detect_batch_sample: %v", err)
	}

	// 返回样本列表
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取批次样本成功",
		Data: utils.H{
			"list":  samples,
			"total": len(samples),
		},
	})
}

// 处理添加样本到批次请求
func HandleAddSampleToBatch(c *app.RequestContext, db *sql.DB) {
	// 解析请求体
	var req struct {
		BatchId        int    `json:"batchId" binding:"required"`
		SampleCode     string `json:"sampleCode" binding:"required"`
		PatientId      int    `json:"patientId" binding:"required"`
		SampleType     int    `json:"sampleType" binding:"required"`
		CancerType     int    `json:"cancerType" binding:"required"`
		TreatmentStage int    `json:"treatmentStage" binding:"required"`
	}

	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请求参数错误",
			Data:    nil,
		})
		return
	}

	// 从上下文获取当前用户ID
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

	// 开始事务
	tx, err := db.Begin()
	if err != nil {
		log.Printf("Failed to begin transaction: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	// 检查批次是否存在
	var batchExists bool
	err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM detect_batch WHERE id = ?)", req.BatchId).Scan(&batchExists)
	if err != nil || !batchExists {
		log.Printf("Batch not found: %v", err)
		tx.Rollback()
		c.JSON(consts.StatusNotFound, ApiResponse{
			Code:    404,
			Success: false,
			Message: "批次不存在",
			Data:    nil,
		})
		return
	}

	// 检查样本是否已存在
	var sampleExists bool
	err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM detect_sample WHERE sample_code = ?)", req.SampleCode).Scan(&sampleExists)
	if err != nil {
		log.Printf("Failed to check sample existence: %v", err)
		tx.Rollback()
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	if sampleExists {
		tx.Rollback()
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "样本编号已存在",
			Data:    nil,
		})
		return
	}

	// 检查患者是否存在
	var patientExists bool
	err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM detect_patient WHERE id = ? AND is_active = 1)", req.PatientId).Scan(&patientExists)
	if err != nil || !patientExists {
		log.Printf("Patient not found: %v", err)
		tx.Rollback()
		c.JSON(consts.StatusNotFound, ApiResponse{
			Code:    404,
			Success: false,
			Message: "患者不存在",
			Data:    nil,
		})
		return
	}

	// 创建新样本
	result, err := tx.Exec(`INSERT INTO detect_sample (
		sample_code, patient_id, sample_type_id, cancer_type_id, treatment_stage_id, 
		collection_date, collection_operator, sample_status, batch_id, match_status, 
		sample_created_at, sample_updated_at
	) VALUES (?, ?, ?, ?, ?, NOW(), ?, 'created', ?, 'pending', NOW(), NOW())`,
		req.SampleCode, req.PatientId, req.SampleType, req.CancerType, req.TreatmentStage,
		userID, req.BatchId)

	if err != nil {
		log.Printf("Failed to create sample: %v", err)
		tx.Rollback()
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	// 获取新样本ID
	sampleId, err := result.LastInsertId()
	if err != nil {
		log.Printf("Failed to get sample ID: %v", err)
		tx.Rollback()
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	// 样本添加成功，无需处理missing_samples（字段已删除）

	// 提交事务
	if err = tx.Commit(); err != nil {
		log.Printf("Failed to commit transaction: %v", err)
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
		Message: "样本添加成功",
		Data: utils.H{
			"sampleId":   sampleId,
			"sampleCode": req.SampleCode,
			"batchId":    req.BatchId,
		},
	})
}

func loadBatchMedianDataForSubmit(tx *sql.Tx, batchID int) ([]map[string]interface{}, error) {
	var mergedDataJSON sql.NullString
	if err := tx.QueryRow("SELECT merged_data FROM detect_batch WHERE id = ?", batchID).Scan(&mergedDataJSON); err != nil {
		return nil, err
	}
	if mergedDataJSON.Valid && strings.TrimSpace(mergedDataJSON.String) != "" {
		var mergedMap map[string]map[string]interface{}
		if err := json.Unmarshal([]byte(mergedDataJSON.String), &mergedMap); err == nil && len(mergedMap) > 0 {
			sampleCodes := make([]string, 0, len(mergedMap))
			for sampleCode := range mergedMap {
				sampleCodes = append(sampleCodes, sampleCode)
			}
			sort.Strings(sampleCodes)

			medianData := make([]map[string]interface{}, 0, len(sampleCodes))
			for _, sampleCode := range sampleCodes {
				data := mergedMap[sampleCode]
				if data == nil {
					data = make(map[string]interface{})
				}
				data["Sample"] = sampleCode
				medianData = append(medianData, data)
			}
			return medianData, nil
		}

		var mergedList []map[string]interface{}
		if err := json.Unmarshal([]byte(mergedDataJSON.String), &mergedList); err == nil {
			return mergedList, nil
		}
	}

	rows, err := tx.Query(`
		SELECT sample_code, platform, median_data
		FROM detect_batch_platform_data
		WHERE batch_id = ? AND sample_code != 'H'
		ORDER BY sample_code, platform
	`, batchID)
	if err == nil {
		defer rows.Close()
		sampleData := make(map[string]map[string]interface{})
		for rows.Next() {
			var sampleCode, platform string
			var medianJSON sql.NullString
			if err := rows.Scan(&sampleCode, &platform, &medianJSON); err != nil {
				continue
			}
			if !medianJSON.Valid || strings.TrimSpace(medianJSON.String) == "" {
				continue
			}

			var median map[string]interface{}
			if err := json.Unmarshal([]byte(medianJSON.String), &median); err != nil {
				return nil, err
			}

			if _, exists := sampleData[sampleCode]; !exists {
				sampleData[sampleCode] = map[string]interface{}{"Sample": sampleCode}
			}
			for key, value := range median {
				if key == "Sample" || key == "sample_code" {
					continue
				}
				sampleData[sampleCode][key] = value
			}
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		if len(sampleData) > 0 {
			sampleCodes := make([]string, 0, len(sampleData))
			for sampleCode := range sampleData {
				sampleCodes = append(sampleCodes, sampleCode)
			}
			sort.Strings(sampleCodes)

			medianData := make([]map[string]interface{}, 0, len(sampleCodes))
			for _, sampleCode := range sampleCodes {
				medianData = append(medianData, sampleData[sampleCode])
			}
			return medianData, nil
		}
	} else {
		log.Printf("Failed to query multi-platform median data: %v", err)
	}

	var medianDataJSON sql.NullString
	if err := tx.QueryRow("SELECT median_data FROM detect_batch WHERE id = ?", batchID).Scan(&medianDataJSON); err != nil {
		return nil, err
	}
	if !medianDataJSON.Valid || strings.TrimSpace(medianDataJSON.String) == "" {
		return []map[string]interface{}{}, nil
	}

	var medianData []map[string]interface{}
	if err := json.Unmarshal([]byte(medianDataJSON.String), &medianData); err != nil {
		var metadata map[string]interface{}
		if objectErr := json.Unmarshal([]byte(medianDataJSON.String), &metadata); objectErr == nil {
			return []map[string]interface{}{}, nil
		}
		return nil, err
	}
	return medianData, nil
}

func rewriteBatchGeneDataKeys(rawJSON sql.NullString, mapping map[string]string) (string, bool, error) {
	if !rawJSON.Valid || strings.TrimSpace(rawJSON.String) == "" {
		return rawJSON.String, false, nil
	}

	var rows []map[string]interface{}
	if err := json.Unmarshal([]byte(rawJSON.String), &rows); err != nil {
		return rawJSON.String, false, err
	}

	changed := false
	for i, row := range rows {
		rewritten := make(map[string]interface{}, len(row))
		for key, value := range row {
			if target, ok := mapping[key]; ok && target != "" {
				rewritten[target] = value
				changed = true
				continue
			}
			rewritten[key] = value
		}
		rows[i] = rewritten
	}

	if !changed {
		return rawJSON.String, false, nil
	}

	updated, err := json.Marshal(rows)
	if err != nil {
		return rawJSON.String, false, err
	}
	return string(updated), true, nil
}

func HandleApplyBatchGeneMatches(c *app.RequestContext, db *sql.DB) {
	type GeneMatch struct {
		Source string `json:"source"`
		Target string `json:"target"`
	}

	var req struct {
		BatchId interface{} `json:"batchId"`
		Matches []GeneMatch `json:"matches"`
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

	var batchID int
	switch v := req.BatchId.(type) {
	case float64:
		batchID = int(v)
	case string:
		if id, err := strconv.Atoi(v); err == nil {
			batchID = id
		} else if err := db.QueryRow(`SELECT id FROM detect_batch WHERE batch_code = ?`, v).Scan(&batchID); err != nil {
			c.JSON(consts.StatusBadRequest, ApiResponse{
				Code:    400,
				Success: false,
				Message: "无效的批次ID或批次编号",
				Data:    nil,
			})
			return
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

	geneMapping := make(map[string]string)
	for _, match := range req.Matches {
		source := strings.TrimSpace(match.Source)
		target := strings.TrimSpace(match.Target)
		if source == "" || target == "" {
			continue
		}
		geneSymbol, err := getGeneSymbolByAnyName(db, target)
		if err != nil || strings.TrimSpace(geneSymbol) == "" {
			c.JSON(consts.StatusBadRequest, ApiResponse{
				Code:    400,
				Success: false,
				Message: fmt.Sprintf("目标基因不存在：%s", target),
				Data:    nil,
			})
			return
		}
		geneMapping[source] = geneSymbol
	}

	if len(geneMapping) == 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "没有可保存的基因匹配",
			Data:    nil,
		})
		return
	}

	tx, err := db.Begin()
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "服务器内部错误", Data: nil})
		return
	}
	defer tx.Rollback()

	var medianJSON, countJSON sql.NullString
	if err := tx.QueryRow("SELECT median_data, count_data FROM detect_batch WHERE id = ?", batchID).Scan(&medianJSON, &countJSON); err != nil {
		c.JSON(consts.StatusNotFound, ApiResponse{Code: 404, Success: false, Message: "批次不存在", Data: nil})
		return
	}

	updatedMedian, medianChanged, err := rewriteBatchGeneDataKeys(medianJSON, geneMapping)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "Median数据格式错误", Data: nil})
		return
	}
	updatedCount, countChanged, err := rewriteBatchGeneDataKeys(countJSON, geneMapping)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "Count数据格式错误", Data: nil})
		return
	}

	if medianChanged || countChanged {
		if _, err := tx.Exec("UPDATE detect_batch SET median_data = ?, count_data = ?, updated_at = NOW() WHERE id = ?", updatedMedian, updatedCount, batchID); err != nil {
			c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "保存基因匹配失败", Data: nil})
			return
		}
	}

	if err := tx.Commit(); err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "服务器内部错误", Data: nil})
		return
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "基因匹配保存成功",
		Data:    utils.H{"updated": medianChanged || countChanged},
	})
}

// 处理提交批次请求，将Median值传输到样本表
func HandleSubmitBatch(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	param := c.Param("id")
	forceOverwrite := c.Query("force") == "1" || strings.EqualFold(c.Query("force"), "true")
	if !forceOverwrite && len(c.Request.Body()) > 0 {
		var req struct {
			Force          bool `json:"force"`
			ForceOverwrite bool `json:"forceOverwrite"`
		}
		if err := json.Unmarshal(c.Request.Body(), &req); err == nil {
			forceOverwrite = req.Force || req.ForceOverwrite
		}
	}

	// 尝试将参数转换为整数（旧格式：数据库ID）
	detect_batchId, err := strconv.Atoi(param)
	if err != nil {
		// 参数不是数字，尝试作为批次编号查询批次ID
		err = db.QueryRow(`SELECT id FROM detect_batch WHERE batch_code = ?`, param).Scan(&detect_batchId)
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
	if err := ensureBatchSampleModelIDColumn(db); err != nil {
		log.Printf("Failed to ensure detect_batch_sample.model_id: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "批次模型字段初始化失败"})
		return
	}

	// 从上下文获取当前用户ID和姓名
	var submitterID int
	var submitterName string
	if userID, exists := c.Get("userID"); exists {
		submitterID = userID.(int)
		// 查询用户姓名
		err := db.QueryRow("SELECT real_name FROM base_manage_user WHERE id = ?", submitterID).Scan(&submitterName)
		if err != nil {
			log.Printf("Failed to get submitter name: %v", err)
			submitterName = ""
		}
	}
	testOperatorID := submitterID
	var selectedTesterID sql.NullInt32
	var selectedTesterName sql.NullString
	if err := db.QueryRow("SELECT tester_id, tester_name FROM detect_batch WHERE id = ?", detect_batchId).Scan(&selectedTesterID, &selectedTesterName); err == nil {
		if selectedTesterID.Valid && selectedTesterID.Int32 > 0 {
			testOperatorID = int(selectedTesterID.Int32)
		}
	} else {
		log.Printf("Failed to query batch tester: %v", err)
	}

	// 开始事务
	tx, err := db.Begin()
	if err != nil {
		log.Printf("Failed to begin transaction: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	// 检查是否存在未匹配的样本（match_status=0）
	var hasUnmatchedSamples bool
	err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM detect_batch_sample WHERE batch_id = ? AND match_status = 0)", detect_batchId).Scan(&hasUnmatchedSamples)
	if err != nil {
		log.Printf("Failed to check for unmatched samples: %v", err)
		tx.Rollback()
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	if hasUnmatchedSamples {
		tx.Rollback()
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无法提交，存在系统中不存在的样本",
			Data:    nil,
		})
		return
	}

	// 检查是否存在已有检测结果或报告完成的样本
	var completedSamples []string
	rows, err := tx.Query("SELECT s.sample_code FROM detect_sample s JOIN detect_batch_sample bs ON s.sample_code = bs.sample_code WHERE bs.batch_id = ? AND s.sample_status IN ('tested', 'completed')", detect_batchId)
	if err != nil {
		log.Printf("Failed to check completed samples: %v", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var sampleCode string
			if err := rows.Scan(&sampleCode); err == nil {
				completedSamples = append(completedSamples, sampleCode)
			}
		}
	}

	// 如果存在已完成的样本，默认返回提示信息；用户明确选择覆盖时允许继续提交。
	if len(completedSamples) > 0 && !forceOverwrite {
		tx.Rollback()
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "存在已检测或已完成的样本，提交后将覆盖这些样本的信息",
			Data:    map[string]interface{}{"completedSamples": completedSamples},
		})
		return
	}

	medianData, err := loadBatchMedianDataForSubmit(tx, detect_batchId)
	if err != nil {
		log.Printf("Failed to load median data for submit: %v", err)
		tx.Rollback()
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	remainingSampleCodes, err := queryCurrentBatchSampleCodesTx(tx, detect_batchId)
	if err != nil {
		log.Printf("Failed to query current batch samples: %v", err)
		tx.Rollback()
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}
	if len(remainingSampleCodes) == 0 {
		tx.Rollback()
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "批次中没有可提交的样本",
			Data:    nil,
		})
		return
	}

	// 处理每个样本的Median数据
	processedSampleCodes := make(map[string]bool)
	for _, data := range medianData {
		sampleCode, ok := data["Sample"].(string)
		if !ok {
			continue
		}
		sampleCode = strings.TrimSpace(sampleCode)

		// 排除H对照水样本
		if sampleCode == "H" {
			continue
		}
		if !remainingSampleCodes[sampleCode] {
			continue
		}

		// 查找样本ID。Panel/患者匹配会提前把 match_status 更新为 matched，
		// 提交资格应由检测状态决定，不能再用 match_status=pending 过滤。
		var sampleId int
		if forceOverwrite {
			err = tx.QueryRow(`SELECT id FROM detect_sample WHERE sample_code = ? LIMIT 1`, sampleCode).Scan(&sampleId)
		} else {
			err = tx.QueryRow(`SELECT id FROM detect_sample
				WHERE sample_code = ? AND COALESCE(sample_status, '') NOT IN ('tested', 'completed')
				LIMIT 1`, sampleCode).Scan(&sampleId)
		}
		if err != nil {
			// 样本不存在或已经处理过，跳过
			continue
		}

		// 构建gene_data
		geneData := make(map[string]interface{})
		for key, value := range data {
			if key != "Sample" && key != "sample_code" && key != "location" && key != "Location" {
				geneSymbol, geneErr := getGeneSymbolByAnyName(db, key)
				if geneErr != nil || strings.TrimSpace(geneSymbol) == "" {
					log.Printf("Skip unmatched gene %s for sample %s", key, sampleCode)
					continue
				}
				// 尝试将值转换为浮点数
				if strValue, ok := value.(string); ok {
					if floatValue, err := strconv.ParseFloat(strValue, 64); err == nil {
						geneData[geneSymbol] = floatValue
					} else {
						geneData[geneSymbol] = value
					}
				} else {
					geneData[geneSymbol] = value
				}
			}
		}

		var selectedModelID sql.NullInt32
		if modelErr := tx.QueryRow(`SELECT model_id FROM detect_batch_sample WHERE batch_id = ? AND sample_code = ?`, detect_batchId, sampleCode).Scan(&selectedModelID); modelErr != nil && modelErr != sql.ErrNoRows {
			log.Printf("Failed to get selected model for sample %s: %v", sampleCode, modelErr)
		}
		if selectedModelID.Valid && selectedModelID.Int32 > 0 {
			if _, enrichErr := enrichExcelDuplicateGeneVariables(db, detect_batchId, sampleCode, int(selectedModelID.Int32), geneData); enrichErr != nil {
				log.Printf("Failed to preserve platform duplicate genes for sample %s: %v", sampleCode, enrichErr)
			}
		}

		// 构建result_data
		resultData := map[string]interface{}{
			"gene_data":   geneData,
			"sample_code": sampleCode,
		}

		// 转换为JSON字符串
		resultDataJSON, err := json.Marshal(resultData)
		if err != nil {
			continue
		}

		// 获取样本当前状态
		var currentStatus string
		err = tx.QueryRow("SELECT sample_status FROM detect_sample WHERE id = ?", sampleId).Scan(&currentStatus)
		if err != nil {
			log.Printf("Failed to get sample status: %v", err)
			continue
		}
		// 构建更新语句
		setClauses := []string{"batch_id = ?", "result_data = ?", "sample_status = 'tested'", "match_status = 'matched'", "test_operator = ?", "result_updated_at = NOW()"}
		args := []interface{}{detect_batchId, string(resultDataJSON), testOperatorID}

		// 只对未接收的样本更新接收人信息
		if currentStatus != "received" {
			setClauses = append(setClauses, "receive_operator = ?")
			args = append(args, submitterID)
		}
		if selectedModelID.Valid && selectedModelID.Int32 > 0 {
			setClauses = append(setClauses, "model_id = ?")
			args = append(args, selectedModelID.Int32)
		}
		args = append(args, sampleId)

		// 构建完整的更新语句
		query := "UPDATE detect_sample SET " + strings.Join(setClauses, ", ") + " WHERE id = ?"

		// 更新样本记录
		_, err = tx.Exec(query, args...)
		if err != nil {
			log.Printf("Failed to save result: %v", err)
			continue
		}
		processedSampleCodes[sampleCode] = true
	}

	if len(processedSampleCodes) != len(remainingSampleCodes) {
		missingProcessed := sampleCodeSliceFromSet(remainingSampleCodes)
		filteredMissing := make([]string, 0, len(missingProcessed))
		for _, sampleCode := range missingProcessed {
			if !processedSampleCodes[sampleCode] {
				filteredMissing = append(filteredMissing, sampleCode)
			}
		}
		tx.Rollback()
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "批次人数校验失败，部分剩余样本没有可提交的检测结果",
			Data:    utils.H{"missingSamples": filteredMissing},
		})
		return
	}

	// 查询该批次的所有样本信息
	remainingSampleList := sampleCodeSliceFromSet(remainingSampleCodes)
	placeholders := make([]string, 0, len(remainingSampleList))
	args := make([]interface{}, 0, len(remainingSampleList))
	for _, sampleCode := range remainingSampleList {
		placeholders = append(placeholders, "?")
		args = append(args, sampleCode)
	}
	rows, err = tx.Query(fmt.Sprintf(`
		SELECT s.sample_code, s.patient_id, COALESCE(p.patient_code, '') as patient_code, COALESCE(p.name, '') as patient_name
		FROM detect_sample s
		LEFT JOIN detect_patient p ON s.patient_id = p.id
		WHERE s.sample_code IN (%s)
		ORDER BY s.sample_code
	`, strings.Join(placeholders, ",")), args...)
	if err != nil {
		log.Printf("Failed to query submitted sample records: %v", err)
	} else {
		defer rows.Close()

		// 构建样本信息数组
		var samples []map[string]interface{}
		for rows.Next() {
			var sampleCode string
			var patientId sql.NullInt32
			var patientCode string
			var patientName sql.NullString

			err := rows.Scan(&sampleCode, &patientId, &patientCode, &patientName)
			if err != nil {
				log.Printf("Failed to scan submitted sample: %v", err)
				continue
			}

			samples = append(samples, map[string]interface{}{
				"sample_code":  sampleCode,
				"patient_id":   patientId.Int32,
				"patient_code": patientCode,
				"patient_name": patientName.String,
			})
		}

		// 将样本信息转换为JSON
		if len(samples) > 0 {
			samplesJSON, err := json.Marshal(samples)
			if err != nil {
				log.Printf("Failed to marshal samples to JSON: %v", err)
			} else {
				// 更新 detect_batch.samples 字段
				_, err = tx.Exec("UPDATE detect_batch SET samples = ? WHERE id = ?", string(samplesJSON), detect_batchId)
				if err != nil {
					log.Printf("Failed to update detect_batch.samples: %v", err)
				}
			}
		}
	}

	// 更新批次状态和提交人信息
	_, err = tx.Exec("UPDATE detect_batch SET status = 'submitted', sample_count = ?, submitter_id = ?, submitter_name = ?, updated_at = NOW() WHERE id = ?", len(remainingSampleCodes), submitterID, submitterName, detect_batchId)
	if err != nil {
		log.Printf("Failed to update detect_batch status: %v", err)
	}

	// 删除 detect_batch_sample 表中该批次的记录
	_, err = tx.Exec("DELETE FROM detect_batch_sample WHERE batch_id = ?", detect_batchId)
	if err != nil {
		log.Printf("Failed to delete detect_batch_sample records: %v", err)
	}

	// 提交事务
	if err = tx.Commit(); err != nil {
		log.Printf("Failed to commit transaction: %v", err)
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
		Message: "批次提交成功，Median值已传输到样本表",
		Data:    nil,
	})
}

func resolveBatchID(db *sql.DB, param string) (int, string, error) {
	batchID, err := strconv.Atoi(strings.TrimSpace(param))
	if err == nil && batchID > 0 {
		var batchCode string
		if scanErr := db.QueryRow(`SELECT batch_code FROM detect_batch WHERE id = ?`, batchID).Scan(&batchCode); scanErr != nil {
			return 0, "", scanErr
		}
		return batchID, batchCode, nil
	}
	var batchCode string
	err = db.QueryRow(`SELECT id, batch_code FROM detect_batch WHERE batch_code = ?`, strings.TrimSpace(param)).Scan(&batchID, &batchCode)
	return batchID, batchCode, err
}

func queryDuplicateCompletedSamples(db *sql.DB, batchID int) ([]utils.H, error) {
	rows, err := db.Query(`
		SELECT DISTINCT s.id, s.sample_code, COALESCE(p.name, ''), COALESCE(p.patient_code, ''), COALESCE(s.sample_status, '')
		FROM detect_batch_sample bs
		JOIN detect_sample s ON s.sample_code = bs.sample_code
		LEFT JOIN detect_patient p ON p.id = s.patient_id
		WHERE bs.batch_id = ? AND s.sample_status IN ('tested', 'completed')
		ORDER BY s.sample_code
	`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	duplicates := []utils.H{}
	for rows.Next() {
		var sampleID int
		var sampleCode, patientName, patientCode, status string
		if err := rows.Scan(&sampleID, &sampleCode, &patientName, &patientCode, &status); err != nil {
			return nil, err
		}
		duplicates = append(duplicates, utils.H{
			"sampleId":    sampleID,
			"sampleCode":  sampleCode,
			"patientName": patientName,
			"patientCode": patientCode,
			"status":      status,
		})
	}
	return duplicates, rows.Err()
}

func HandleGetBatchDuplicateSamples(c *app.RequestContext, db *sql.DB) {
	batchID, batchCode, err := resolveBatchID(db, c.Param("id"))
	if err != nil {
		c.JSON(consts.StatusNotFound, ApiResponse{Code: 404, Success: false, Message: "批次不存在", Data: nil})
		return
	}
	duplicates, err := queryDuplicateCompletedSamples(db, batchID)
	if err != nil {
		log.Printf("Failed to query duplicate samples: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "服务器内部错误", Data: nil})
		return
	}
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取重复样本成功",
		Data: utils.H{
			"batchId":          batchID,
			"batchCode":        batchCode,
			"duplicateSamples": duplicates,
		},
	})
}

func nextRetestSampleCodeTx(tx *sql.Tx, baseSampleCode string) (string, error) {
	for i := 1; i <= 99; i++ {
		candidate := fmt.Sprintf("%s-FJ%02d", strings.TrimSpace(baseSampleCode), i)
		var exists bool
		if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM detect_sample WHERE sample_code = ?)`, candidate).Scan(&exists); err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("样本 %s 的复检编号已超过FJ99", baseSampleCode)
}

func renameSampleCodeInBatchJSON(raw string, oldCode string, newCode string) (string, bool, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "null" {
		return raw, false, nil
	}
	var rows []map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &rows); err != nil {
		return raw, false, nil
	}
	changed := false
	for _, row := range rows {
		for _, key := range []string{"Sample", "sample_code", "sampleCode"} {
			if strings.TrimSpace(fmt.Sprint(row[key])) == oldCode {
				row[key] = newCode
				changed = true
			}
		}
	}
	if !changed {
		return raw, false, nil
	}
	updated, err := json.Marshal(rows)
	if err != nil {
		return raw, false, err
	}
	return string(updated), true, nil
}

func renameSampleCodeInBatchDataTx(tx *sql.Tx, batchID int, oldCode string, newCode string) error {
	_, err := tx.Exec(`UPDATE detect_batch_sample SET sample_code = ? WHERE batch_id = ? AND sample_code = ?`, newCode, batchID, oldCode)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE detect_batch_platform_data SET sample_code = ? WHERE batch_id = ? AND sample_code = ?`, newCode, batchID, oldCode)
	if err != nil {
		return err
	}

	var medianData, countData, mergedData sql.NullString
	if err := tx.QueryRow(`SELECT median_data, count_data, merged_data FROM detect_batch WHERE id = ?`, batchID).Scan(&medianData, &countData, &mergedData); err != nil {
		return err
	}
	updates := map[string]sql.NullString{
		"median_data": medianData,
		"count_data":  countData,
		"merged_data": mergedData,
	}
	for column, value := range updates {
		if !value.Valid {
			continue
		}
		updated, changed, err := renameSampleCodeInBatchJSON(value.String, oldCode, newCode)
		if err != nil {
			return err
		}
		if changed {
			if _, err := tx.Exec(fmt.Sprintf(`UPDATE detect_batch SET %s = ? WHERE id = ?`, column), updated, batchID); err != nil {
				return err
			}
		}
	}
	return nil
}

func createRetestSampleForBatchTx(tx *sql.Tx, batchID int, oldCode string) (string, error) {
	newCode, err := nextRetestSampleCodeTx(tx, oldCode)
	if err != nil {
		return "", err
	}
	_, err = tx.Exec(`
		INSERT INTO detect_sample (
			patient_id, sample_code, sample_type_id, cancer_type_id, treatment_stage_id,
			collection_date, collection_operator, receive_date, receive_operator,
			sample_status, report_type, notes, organization, batch_id, match_status,
			sample_created_at, sample_updated_at, created_at, updated_at
		)
		SELECT patient_id, ?, sample_type_id, cancer_type_id, treatment_stage_id,
			collection_date, collection_operator, receive_date, receive_operator,
			'received', report_type, CONCAT(COALESCE(notes, ''), CASE WHEN COALESCE(notes, '') = '' THEN '' ELSE '\n' END, '复检样本，来源：', sample_code),
			organization, ?, 'pending', NOW(), NOW(), NOW(), NOW()
		FROM detect_sample
		WHERE sample_code = ?
		LIMIT 1
	`, newCode, batchID, oldCode)
	if err != nil {
		return "", err
	}
	if err := renameSampleCodeInBatchDataTx(tx, batchID, oldCode, newCode); err != nil {
		return "", err
	}
	return newCode, nil
}

func HandleCreateBatchRetestSamples(c *app.RequestContext, db *sql.DB) {
	var req struct {
		BatchID     int      `json:"batchId"`
		BatchCode   string   `json:"batchCode"`
		SampleCode  string   `json:"sampleCode"`
		SampleCodes []string `json:"sampleCodes"`
	}
	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "请求参数错误", Data: nil})
		return
	}
	if req.BatchID <= 0 {
		batchID, _, err := resolveBatchID(db, req.BatchCode)
		if err != nil {
			c.JSON(consts.StatusNotFound, ApiResponse{Code: 404, Success: false, Message: "批次不存在", Data: nil})
			return
		}
		req.BatchID = batchID
	}
	sampleSet := map[string]bool{}
	if normalized := normalizeBatchSampleCode(req.SampleCode); normalized != "" {
		sampleSet[normalized] = true
	}
	for _, sampleCode := range req.SampleCodes {
		if normalized := normalizeBatchSampleCode(sampleCode); normalized != "" {
			sampleSet[normalized] = true
		}
	}
	if len(sampleSet) == 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "请选择复检样本", Data: nil})
		return
	}
	sampleCodes := sampleCodeSliceFromSet(sampleSet)

	tx, err := db.Begin()
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "服务器内部错误", Data: nil})
		return
	}
	created := []utils.H{}
	for _, sampleCode := range sampleCodes {
		newCode, err := createRetestSampleForBatchTx(tx, req.BatchID, sampleCode)
		if err != nil {
			tx.Rollback()
			log.Printf("Failed to create retest sample for %s: %v", sampleCode, err)
			c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "创建复检样本失败", Data: nil})
			return
		}
		created = append(created, utils.H{"sourceSampleCode": sampleCode, "sampleCode": newCode})
	}
	if err := tx.Commit(); err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "服务器内部错误", Data: nil})
		return
	}
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "复检样本已创建",
		Data:    utils.H{"batchId": req.BatchID, "samples": created},
	})
}

// 处理批次结果删除请求
func HandleDeleteSampleFromBatch(c *app.RequestContext, db *sql.DB) {
	// 解析请求体
	var req struct {
		BatchId     int      `json:"batchId"`
		BatchCode   string   `json:"batchCode"`
		SampleCode  string   `json:"sampleCode"`
		SampleCodes []string `json:"sampleCodes"`
	}

	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请求参数错误",
			Data:    nil,
		})
		return
	}

	sampleCodeSet := make(map[string]bool)
	if strings.TrimSpace(req.SampleCode) != "" {
		sampleCodeSet[strings.TrimSpace(req.SampleCode)] = true
	}
	for _, sampleCode := range req.SampleCodes {
		if strings.TrimSpace(sampleCode) != "" {
			sampleCodeSet[strings.TrimSpace(sampleCode)] = true
		}
	}
	if len(sampleCodeSet) == 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请选择要删除结果的样本",
			Data:    nil,
		})
		return
	}
	sampleCodes := make([]string, 0, len(sampleCodeSet))
	for sampleCode := range sampleCodeSet {
		sampleCodes = append(sampleCodes, sampleCode)
	}
	sort.Strings(sampleCodes)

	if req.BatchId <= 0 && strings.TrimSpace(req.BatchCode) != "" {
		err := db.QueryRow(`SELECT id FROM detect_batch WHERE batch_code = ?`, strings.TrimSpace(req.BatchCode)).Scan(&req.BatchId)
		if err != nil {
			c.JSON(consts.StatusNotFound, ApiResponse{
				Code:    404,
				Success: false,
				Message: "批次不存在",
				Data:    nil,
			})
			return
		}
	}
	if req.BatchId <= 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "批次参数错误",
			Data:    nil,
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
			Data:    nil,
		})
		return
	}

	// 检查批次是否存在
	var batchExists bool
	err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM detect_batch WHERE id = ?)", req.BatchId).Scan(&batchExists)
	if err != nil || !batchExists {
		log.Printf("Batch not found: %v", err)
		tx.Rollback()
		c.JSON(consts.StatusNotFound, ApiResponse{
			Code:    404,
			Success: false,
			Message: "批次不存在",
			Data:    nil,
		})
		return
	}

	placeholders := make([]string, 0, len(sampleCodes))
	args := []interface{}{req.BatchId}
	for _, sampleCode := range sampleCodes {
		placeholders = append(placeholders, "?")
		args = append(args, sampleCode)
	}

	// 检查样本是否仍存在于批次任一数据源中。异常批次可能只残留在平台明细或批次JSON里。
	existingSampleCodes, err := queryAllBatchSampleCodesTx(tx, req.BatchId)
	if err != nil {
		log.Printf("Failed to query batch sample sources: %v", err)
		tx.Rollback()
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}
	existingCount := 0
	for _, sampleCode := range sampleCodes {
		if existingSampleCodes[normalizeBatchSampleCode(sampleCode)] {
			existingCount++
		}
	}
	if existingCount == 0 {
		log.Printf("Sample not found in batch: %v", err)
		tx.Rollback()
		c.JSON(consts.StatusNotFound, ApiResponse{
			Code:    404,
			Success: false,
			Message: "结果不存在于该批次中",
			Data:    nil,
		})
		return
	}

	// 获取批次的median_data和count_data
	var medianDataJSON, countDataJSON sql.NullString
	err = tx.QueryRow("SELECT median_data, count_data FROM detect_batch WHERE id = ?", req.BatchId).Scan(&medianDataJSON, &countDataJSON)
	if err != nil {
		log.Printf("Failed to get batch data: %v", err)
		tx.Rollback()
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	// 处理median_data。单平台批次为数组，多平台批次可能是对象元信息（如 uploadedGenes），对象无需按样本过滤。
	if medianDataJSON.Valid && medianDataJSON.String != "" {
		updatedMedianDataJSON, changed, err := filterBatchDataJSONBySampleCodes(medianDataJSON.String, sampleCodeSet)
		if err != nil {
			log.Printf("Failed to parse median data: %v", err)
			tx.Rollback()
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code:    500,
				Success: false,
				Message: "服务器内部错误",
				Data:    nil,
			})
			return
		}
		if changed {
			_, err = tx.Exec("UPDATE detect_batch SET median_data = ? WHERE id = ?", updatedMedianDataJSON, req.BatchId)
			if err != nil {
				log.Printf("Failed to update median data: %v", err)
				tx.Rollback()
				c.JSON(consts.StatusInternalServerError, ApiResponse{
					Code:    500,
					Success: false,
					Message: "服务器内部错误",
					Data:    nil,
				})
				return
			}
		}
	}

	// 处理count_data
	if countDataJSON.Valid && countDataJSON.String != "" {
		updatedCountDataJSON, changed, err := filterBatchDataJSONBySampleCodes(countDataJSON.String, sampleCodeSet)
		if err != nil {
			log.Printf("Failed to parse count data: %v", err)
			tx.Rollback()
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code:    500,
				Success: false,
				Message: "服务器内部错误",
				Data:    nil,
			})
			return
		}
		if changed {
			_, err = tx.Exec("UPDATE detect_batch SET count_data = ? WHERE id = ?", updatedCountDataJSON, req.BatchId)
			if err != nil {
				log.Printf("Failed to update count data: %v", err)
				tx.Rollback()
				c.JSON(consts.StatusInternalServerError, ApiResponse{
					Code:    500,
					Success: false,
					Message: "服务器内部错误",
					Data:    nil,
				})
				return
			}
		}
	}

	var mergedDataJSON sql.NullString
	if err := tx.QueryRow("SELECT merged_data FROM detect_batch WHERE id = ?", req.BatchId).Scan(&mergedDataJSON); err != nil {
		log.Printf("Failed to get merged batch data: %v", err)
		tx.Rollback()
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}
	if mergedDataJSON.Valid && strings.TrimSpace(mergedDataJSON.String) != "" {
		updatedMergedDataJSON, changed, err := filterBatchDataJSONBySampleCodes(mergedDataJSON.String, sampleCodeSet)
		if err != nil {
			log.Printf("Failed to parse merged data: %v", err)
			tx.Rollback()
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code:    500,
				Success: false,
				Message: "服务器内部错误",
				Data:    nil,
			})
			return
		}
		if changed {
			_, err = tx.Exec("UPDATE detect_batch SET merged_data = ? WHERE id = ?", updatedMergedDataJSON, req.BatchId)
			if err != nil {
				log.Printf("Failed to update merged data: %v", err)
				tx.Rollback()
				c.JSON(consts.StatusInternalServerError, ApiResponse{
					Code:    500,
					Success: false,
					Message: "服务器内部错误",
					Data:    nil,
				})
				return
			}
		}
	}

	// 从多平台明细表中删除该样本的所有平台数据
	_, err = tx.Exec("DELETE FROM detect_batch_platform_data WHERE batch_id = ? AND sample_code IN ("+strings.Join(placeholders, ",")+")", args...)
	if err != nil {
		log.Printf("Failed to delete batch platform sample data: %v", err)
		tx.Rollback()
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	// 从detect_batch_sample表中删除相关记录
	_, err = tx.Exec("DELETE FROM detect_batch_sample WHERE batch_id = ? AND sample_code IN ("+strings.Join(placeholders, ",")+")", args...)
	if err != nil {
		log.Printf("Failed to delete batch sample record: %v", err)
		tx.Rollback()
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	// 已提交批次的样本保留在 detect_sample 中。这里仅清理该批次产生的结果链路，
	// 不解绑样本、不重置患者/治疗阶段/匹配状态等样本档案信息。
	selectedSampleIDs := make([]int, 0, len(sampleCodes))
	selectedSampleCodeByID := map[int]string{}
	sampleIDRows, err := tx.Query("SELECT id, sample_code FROM detect_sample WHERE batch_id = ? AND sample_code IN ("+strings.Join(placeholders, ",")+")", args...)
	if err != nil {
		log.Printf("Failed to query selected samples from detect_sample: %v", err)
		tx.Rollback()
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "查询样本失败",
			Data:    nil,
		})
		return
	}
	for sampleIDRows.Next() {
		var sampleID int
		var sampleCode string
		if err := sampleIDRows.Scan(&sampleID, &sampleCode); err != nil {
			sampleIDRows.Close()
			log.Printf("Failed to scan selected sample id: %v", err)
			tx.Rollback()
			c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "读取样本失败", Data: nil})
			return
		}
		selectedSampleIDs = append(selectedSampleIDs, sampleID)
		selectedSampleCodeByID[sampleID] = sampleCode
	}
	if err := sampleIDRows.Close(); err != nil {
		log.Printf("Failed to close selected sample id rows: %v", err)
		tx.Rollback()
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "读取样本失败", Data: nil})
		return
	}

	if len(selectedSampleIDs) > 0 {
		idPlaceholders := make([]string, 0, len(selectedSampleIDs))
		idArgs := make([]interface{}, 0, len(selectedSampleIDs))
		for _, sampleID := range selectedSampleIDs {
			idPlaceholders = append(idPlaceholders, "?")
			idArgs = append(idArgs, sampleID)
		}

		reviewedRows, err := tx.Query(`SELECT r.id, r.sample_id, COALESCE(r.status, '')
			FROM detect_report r
			WHERE r.sample_id IN (`+strings.Join(idPlaceholders, ",")+`) AND r.status IN ('reviewed', 'published')`, idArgs...)
		if err != nil {
			log.Printf("Failed to check reviewed reports before batch sample delete: %v", err)
			tx.Rollback()
			c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "检查关联报告失败", Data: nil})
			return
		}
		blockedSamples := []string{}
		for reviewedRows.Next() {
			var reportID, sampleID int
			var status string
			if err := reviewedRows.Scan(&reportID, &sampleID, &status); err == nil {
				if sampleCode := selectedSampleCodeByID[sampleID]; sampleCode != "" {
					blockedSamples = append(blockedSamples, sampleCode)
				}
			}
			_ = reportID
			_ = status
		}
		reviewedRows.Close()
		if len(blockedSamples) > 0 {
			tx.Rollback()
			sort.Strings(blockedSamples)
			c.JSON(consts.StatusBadRequest, ApiResponse{
				Code:    400,
				Success: false,
				Message: fmt.Sprintf("%s报告已经审核通过，请先退回报告后再删除结果", blockedSamples[0]),
				Data:    utils.H{"sampleCodes": blockedSamples},
			})
			return
		}

		reportIDs := []int{}
		reportArgs := append([]interface{}{}, idArgs...)
		reportArgs = append(reportArgs, idArgs...)
		reportRows, err := tx.Query(`SELECT id FROM detect_report
			WHERE sample_id IN (`+strings.Join(idPlaceholders, ",")+`)
				OR parent_report_id IN (SELECT id FROM detect_report WHERE sample_id IN (`+strings.Join(idPlaceholders, ",")+`))`, reportArgs...)
		if err != nil {
			log.Printf("Failed to query reports before batch sample delete: %v", err)
			tx.Rollback()
			c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "查询关联报告失败", Data: nil})
			return
		}
		for reportRows.Next() {
			var reportID int
			if err := reportRows.Scan(&reportID); err == nil {
				reportIDs = append(reportIDs, reportID)
			}
		}
		reportRows.Close()
		if len(reportIDs) > 0 {
			reportPlaceholders := make([]string, 0, len(reportIDs))
			reportDeleteArgs := make([]interface{}, 0, len(reportIDs))
			for _, reportID := range reportIDs {
				reportPlaceholders = append(reportPlaceholders, "?")
				reportDeleteArgs = append(reportDeleteArgs, reportID)
			}
			if mysqlTableExists(tx, "detect_report_change_log") {
				if _, err := tx.Exec(`DELETE FROM detect_report_change_log WHERE report_id IN (`+strings.Join(reportPlaceholders, ",")+`)`, reportDeleteArgs...); err != nil {
					log.Printf("Failed to delete report change logs before batch sample delete: %v", err)
					tx.Rollback()
					c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "删除报告变更记录失败", Data: nil})
					return
				}
			}
			if _, err := tx.Exec(`DELETE FROM detect_report WHERE id IN (`+strings.Join(reportPlaceholders, ",")+`)`, reportDeleteArgs...); err != nil {
				log.Printf("Failed to delete reports before batch sample delete: %v", err)
				tx.Rollback()
				c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "删除关联报告失败", Data: nil})
				return
			}
		}

		if mysqlTableExists(tx, "detect_sample_panel_match") {
			if _, err := tx.Exec("DELETE FROM detect_sample_panel_match WHERE batch_id = ? AND sample_code IN ("+strings.Join(placeholders, ",")+")", args...); err != nil {
				log.Printf("Failed to delete sample panel matches before batch sample delete: %v", err)
				tx.Rollback()
				c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "删除结果Panel匹配缓存失败", Data: nil})
				return
			}
		}

		if _, err := tx.Exec(`UPDATE detect_sample
			SET result_data = NULL,
				result_status = NULL,
				signalvalue = NULL,
				sample_status = CASE
					WHEN receive_date IS NOT NULL OR receive_operator IS NOT NULL THEN 'received'
					ELSE 'created'
				END,
				match_status = 'pending',
				test_operator = NULL,
				test_completed_at = NULL,
				result_updated_at = NULL,
				sample_updated_at = NOW()
			WHERE batch_id = ? AND sample_code IN (`+strings.Join(placeholders, ",")+`)`, args...); err != nil {
			log.Printf("Failed to clear batch sample results: %v", err)
			tx.Rollback()
			c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "清除批次结果失败", Data: nil})
			return
		}
	}

	// 计算并更新当前仍保留结果的样本数。
	remainingSampleCodes, err := queryCurrentBatchSampleCodesTx(tx, req.BatchId)
	if err != nil {
		log.Printf("Failed to count remaining samples: %v", err)
		tx.Rollback()
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}
	if len(remainingSampleCodes) == 0 {
		batchFilePaths := []string{}
		batchFileRows, fileErr := tx.Query("SELECT file_path FROM detect_batch_file WHERE batch_id = ?", req.BatchId)
		if fileErr == nil {
			for batchFileRows.Next() {
				var filePath string
				if scanErr := batchFileRows.Scan(&filePath); scanErr == nil && strings.TrimSpace(filePath) != "" {
					batchFilePaths = append(batchFilePaths, filePath)
				}
			}
			batchFileRows.Close()
		} else {
			log.Printf("Failed to query empty batch files: %v", fileErr)
		}

		cleanupStatements := []string{
			"DELETE FROM detect_batch_platform_data WHERE batch_id = ?",
			"DELETE FROM detect_batch_sample WHERE batch_id = ?",
			"DELETE FROM detect_sample_panel_match WHERE batch_id = ?",
			"DELETE FROM detect_batch_file WHERE batch_id = ?",
		}
		for _, statement := range cleanupStatements {
			if _, err := tx.Exec(statement, req.BatchId); err != nil {
				log.Printf("Failed to clean empty batch data: %v", err)
				tx.Rollback()
				c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "删除空批次失败", Data: nil})
				return
			}
		}
		if _, err := tx.Exec(`UPDATE detect_sample SET batch_id = NULL, sample_updated_at = NOW() WHERE batch_id = ?`, req.BatchId); err != nil {
			log.Printf("Failed to detach samples from empty batch: %v", err)
			tx.Rollback()
			c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "解除样本批次关联失败", Data: nil})
			return
		}
		if _, err := tx.Exec("DELETE FROM detect_batch WHERE id = ?", req.BatchId); err != nil {
			log.Printf("Failed to delete empty batch: %v", err)
			tx.Rollback()
			c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "删除空批次失败", Data: nil})
			return
		}
		if err := tx.Commit(); err != nil {
			log.Printf("Failed to commit empty batch delete: %v", err)
			c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "服务器内部错误", Data: nil})
			return
		}
		for _, filePath := range batchFilePaths {
			if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
				log.Printf("Failed to delete empty batch file %s: %v", filePath, err)
			}
		}
		c.JSON(consts.StatusOK, ApiResponse{
			Code: 200, Success: true, Message: "样本结果已删除，该样本是最后一个样本，批次已同时删除",
			Data: utils.H{"batchId": req.BatchId, "sampleCodes": sampleCodes, "batchDeleted": true},
		})
		return
	}

	// 更新批次样本数
	_, err = tx.Exec("UPDATE detect_batch SET sample_count = ? WHERE id = ?", len(remainingSampleCodes), req.BatchId)
	if err != nil {
		log.Printf("Failed to update batch sample count: %v", err)
		tx.Rollback()
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	// 提交事务
	if err = tx.Commit(); err != nil {
		log.Printf("Failed to commit transaction: %v", err)
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
		Message: "结果删除成功",
		Data: utils.H{
			"batchId":     req.BatchId,
			"sampleCodes": sampleCodes,
		},
	})
}

func filterBatchDataJSONBySampleCodes(raw string, sampleCodeSet map[string]bool) (string, bool, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "null" {
		return raw, false, nil
	}

	var rows []map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &rows); err == nil {
		filtered := make([]map[string]interface{}, 0, len(rows))
		changed := false
		for _, row := range rows {
			sampleCode := ""
			if value, ok := row["Sample"].(string); ok {
				sampleCode = strings.TrimSpace(value)
			}
			if sampleCode == "" {
				if value, ok := row["sample_code"].(string); ok {
					sampleCode = strings.TrimSpace(value)
				}
			}
			if sampleCode != "" && sampleCodeSet[sampleCode] {
				changed = true
				continue
			}
			filtered = append(filtered, row)
		}
		if !changed {
			return raw, false, nil
		}
		data, err := json.Marshal(filtered)
		if err != nil {
			return "", false, err
		}
		return string(data), true, nil
	}

	var object map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &object); err == nil {
		filtered := make(map[string]interface{}, len(object))
		changed := false
		for key, value := range object {
			sampleCode := strings.TrimSpace(key)
			if sampleCode != "" && sampleCodeSet[sampleCode] {
				changed = true
				continue
			}
			if row, ok := value.(map[string]interface{}); ok {
				rowSampleCode := ""
				if value, ok := row["Sample"].(string); ok {
					rowSampleCode = strings.TrimSpace(value)
				}
				if rowSampleCode == "" {
					if value, ok := row["sample_code"].(string); ok {
						rowSampleCode = strings.TrimSpace(value)
					}
				}
				if rowSampleCode != "" && sampleCodeSet[rowSampleCode] {
					changed = true
					continue
				}
			}
			filtered[key] = value
		}
		if !changed {
			return raw, false, nil
		}
		data, err := json.Marshal(filtered)
		if err != nil {
			return "", false, err
		}
		return string(data), true, nil
	}

	return "", false, fmt.Errorf("unsupported batch data JSON")
}

// 从ProtocolName提取平台标识（如V8、V9）
func extractPlatformFromProtocolName(protocolName string) string {
	re := regexp.MustCompile(`Panel([A-Z0-9]+)`)
	match := re.FindStringSubmatch(protocolName)
	if len(match) >= 2 {
		return match[1]
	}
	// 如果没有匹配到，尝试其他常见模式
	re = regexp.MustCompile(`([A-Z][0-9])`)
	match = re.FindStringSubmatch(protocolName)
	if len(match) >= 2 {
		return match[1]
	}
	return "UNKNOWN"
}

// 解析单个CSV文件，返回基础信息、Median数据、Count数据和ProtocolName
func parseCSVFileWithProtocol(file *csv.Reader) (map[string]interface{}, []map[string]interface{}, []map[string]interface{}, string, error) {
	baseInfo := make(map[string]interface{})
	medianData := []map[string]interface{}{}
	countData := []map[string]interface{}{}
	protocolName := ""

	var inMedianData bool
	var inCountData bool
	var medianHeader []string
	var countHeader []string

	lineNum := 1
	for {
		row, err := file.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, nil, "", fmt.Errorf("error reading CSV at line %d: %w", lineNum, err)
		}
		lineNum++

		if len(row) == 0 {
			continue
		}

		// 提取ProtocolName
		if strings.TrimSpace(row[0]) == "ProtocolName" && len(row) > 1 {
			protocolName = strings.TrimSpace(row[1])
		}

		// 检查是否开始DataType部分
		if row[0] == "DataType:" {
			if len(row) > 1 && row[1] == "Median" {
				inMedianData = true
				inCountData = false
				medianHeader = []string{}
			} else if len(row) > 1 && row[1] == "Count" {
				inMedianData = false
				inCountData = true
				countHeader = []string{}
			} else {
				inMedianData = false
				inCountData = false
			}
			continue
		}

		// 处理Median数据
		if inMedianData {
			if len(medianHeader) == 0 {
				medianHeader = row
			} else {
				data := make(map[string]interface{})
				numericValues := make(map[string]*duplicateNumericValue)
				hasNonEmptyValue := false
				for k, header := range medianHeader {
					if k < len(row) {
						normalizedHeader := normalizeCSVGeneHeader(header)
						if normalizedHeader != "" && normalizedHeader != "Total Events" {
							value := row[k]
							setMedianDataCell(data, numericValues, normalizedHeader, value)
							if value != "" {
								hasNonEmptyValue = true
							}
						}
					}
				}
				if hasNonEmptyValue {
					medianData = append(medianData, data)
				}
			}
			continue
		}

		// 处理Count数据
		if inCountData {
			if len(countHeader) == 0 {
				countHeader = row
			} else {
				data := make(map[string]interface{})
				for k, header := range countHeader {
					if k < len(row) {
						if header != "" {
							data[header] = row[k]
						}
					}
				}
				if sampleCode, ok := data["Sample"].(string); ok && sampleCode != "" {
					countData = append(countData, data)
				}
			}
			continue
		}

		// 解析基础信息
		if len(row) > 1 {
			switch strings.TrimSpace(row[0]) {
			case "SampleVolume":
				baseInfo["sampleVolume"] = strings.TrimSpace(row[1])
			case "BatchStartTime":
				baseInfo["batchStartTime"] = strings.TrimSpace(row[1])
			case "BatchStopTime":
				baseInfo["batchStopTime"] = strings.TrimSpace(row[1])
			case "Samples":
				if len(row) > 1 {
					sampleCount, err := strconv.Atoi(strings.TrimSpace(row[1]))
					if err == nil {
						baseInfo["sampleCount"] = sampleCount
					}
				}
			case "SN":
				baseInfo["SN"] = strings.TrimSpace(row[1])
			}
		}
	}

	return baseInfo, medianData, countData, protocolName, nil
}

func formatNullTime(t sql.NullTime) string {
	if t.Valid {
		return t.Time.Format("2006-01-02 15:04:05")
	}
	return ""
}

// HandleAutoMatchCancerType - 自动匹配检测类型
func HandleAutoMatchCancerType(c *app.RequestContext, db *sql.DB) {
	// 解析路径参数
	param := c.Param("id")

	// 尝试将参数转换为整数（旧格式：数据库ID）
	batchId, err := strconv.Atoi(param)
	if err != nil {
		// 参数不是数字，尝试作为批次编号查询批次ID
		err = db.QueryRow(`SELECT id FROM detect_batch WHERE batch_code = ?`, param).Scan(&batchId)
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

	// 收集所有样本的基因集合（同时支持单平台和多平台批次）
	allGenes := make(map[string]bool)

	// 方式1：尝试从单平台 median_data 字段读取基因（单平台批次）
	var medianDataJSON sql.NullString
	_ = db.QueryRow(`SELECT median_data FROM detect_batch WHERE id = ?`, batchId).Scan(&medianDataJSON)
	if medianDataJSON.Valid && medianDataJSON.String != "" {
		var medianData []map[string]interface{}
		if jsonErr := json.Unmarshal([]byte(medianDataJSON.String), &medianData); jsonErr == nil {
			for _, data := range medianData {
				sampleCode, ok := data["Sample"].(string)
				if !ok || sampleCode == "" || sampleCode == "H" {
					continue
				}
				for gene, rawValue := range data {
					if gene == "Sample" || gene == "sample_code" || gene == "location" || gene == "Location" || gene == "Total Events" {
						continue
					}
					switch rawValue.(type) {
					case float64, int, string:
						geneSymbol, _ := getGeneSymbolByAnyName(db, gene)
						if geneSymbol != "" {
							allGenes[geneSymbol] = true
						}
					}
				}
			}
		}
	}

	// 方式2：若单平台数据为空，从多平台 detect_batch_platform_data 表读取基因
	if len(allGenes) == 0 {
		log.Printf("HandleAutoMatchCancerType: batch %d has no single-platform median_data, trying multi-platform data", batchId)
		platRows, platErr := db.Query(`
			SELECT sample_code, median_data 
			FROM detect_batch_platform_data 
			WHERE batch_id = ? AND sample_code != 'H'
		`, batchId)
		if platErr == nil {
			defer platRows.Close()
			for platRows.Next() {
				var sampleCode string
				var platMedianJSON sql.NullString
				if scanErr := platRows.Scan(&sampleCode, &platMedianJSON); scanErr != nil {
					continue
				}
				if !platMedianJSON.Valid || platMedianJSON.String == "" {
					continue
				}
				var median map[string]interface{}
				if jsonErr := json.Unmarshal([]byte(platMedianJSON.String), &median); jsonErr != nil {
					continue
				}
				for gene := range median {
					if gene == "Sample" || gene == "sample_code" || gene == "location" || gene == "Location" || gene == "Total Events" {
						continue
					}
					geneSymbol, _ := getGeneSymbolByAnyName(db, gene)
					if geneSymbol != "" {
						allGenes[geneSymbol] = true
					}
				}
			}
		} else {
			log.Printf("HandleAutoMatchCancerType: failed to query platform data for batch %d: %v", batchId, platErr)
		}
	}

	log.Printf("HandleAutoMatchCancerType: batch %d collected %d genes", batchId, len(allGenes))

	if len(allGenes) == 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "未找到样本基因数据",
			Data:    nil,
		})
		return
	}

	// 查询所有检测类型及其关联的Panel
	cancerTypeRows, err := db.Query(`
		SELECT ct.id, ct.name, ct.panel_ids
		FROM setting_cancer_type ct
		WHERE ct.is_active = 1`)
	if err != nil {
		log.Printf("Failed to query cancer types: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}
	defer cancerTypeRows.Close()

	// 查找匹配的检测类型
	var matchedCancerTypes []utils.H
	for cancerTypeRows.Next() {
		var id int
		var name, panelIDsStr string
		if err := cancerTypeRows.Scan(&id, &name, &panelIDsStr); err != nil {
			continue
		}

		if panelIDsStr == "" {
			continue
		}

		// 解析Panel IDs
		panelIDs := strings.Split(panelIDsStr, ",")
		panelCount := len(panelIDs)

		// 获取该检测类型所有Panel的基因
		var allPanelGenes []string
		for _, panelID := range panelIDs {
			panelID = strings.TrimSpace(panelID)
			if panelID == "" {
				continue
			}

			var geneIDsStr string
			err := db.QueryRow(`SELECT gene_ids FROM setting_panel WHERE id = ? AND is_active = 1`, panelID).Scan(&geneIDsStr)
			if err != nil {
				continue
			}

			if geneIDsStr != "" {
				geneIDParts := strings.Split(geneIDsStr, ",")
				for _, gid := range geneIDParts {
					gid = strings.TrimSpace(gid)
					if gid != "" {
						var geneSymbol string
						err := db.QueryRow("SELECT gene_symbol FROM setting_gene WHERE id = ?", gid).Scan(&geneSymbol)
						if err == nil && geneSymbol != "" {
							allPanelGenes = append(allPanelGenes, geneSymbol)
						} else {
							allPanelGenes = append(allPanelGenes, gid)
						}
					}
				}
			}
		}

		// 计算匹配度：检查Panel基因与样本基因的匹配情况
		totalPanelGenes := len(allPanelGenes)
		matchCount := 0

		// 构建不区分大小写的样本基因映射
		allGenesLower := make(map[string]bool)
		for gene := range allGenes {
			allGenesLower[strings.ToLower(gene)] = true
		}

		for _, gene := range allPanelGenes {
			if allGenes[gene] || allGenesLower[strings.ToLower(gene)] {
				matchCount++
			}
		}

		// 计算覆盖率：匹配的Panel基因占Panel总基因的比例
		coverageRate := 0.0
		if totalPanelGenes > 0 {
			coverageRate = float64(matchCount) / float64(totalPanelGenes)
		}

		// 只有当Panel所有基因都能在样本中找到时才视为匹配
		if coverageRate >= 1.0 {
			// 计算样本基因与Panel基因的重叠率（样本基因中有多少比例在Panel中）
			sampleCoverageRate := 0.0
			sampleGeneCount := len(allGenes)
			if sampleGeneCount > 0 && totalPanelGenes > 0 {
				// 计算样本基因在Panel中的数量
				sampleMatchCount := 0
				panelGenesLower := make(map[string]bool)
				for _, gene := range allPanelGenes {
					panelGenesLower[strings.ToLower(gene)] = true
				}
				for gene := range allGenes {
					if panelGenesLower[strings.ToLower(gene)] {
						sampleMatchCount++
					}
				}
				sampleCoverageRate = float64(sampleMatchCount) / float64(sampleGeneCount)
			}

			matchedCancerTypes = append(matchedCancerTypes, utils.H{
				"id":                 id,
				"name":               name,
				"panelCount":         panelCount,
				"panelGenes":         allPanelGenes,
				"totalGenes":         totalPanelGenes,
				"coverageRate":       coverageRate,
				"sampleCoverageRate": sampleCoverageRate,
			})
		}
	}

	// 如果没有匹配的检测类型
	if len(matchedCancerTypes) == 0 {
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "未找到匹配的检测类型",
			Data: utils.H{
				"matchedCancerTypes":    []utils.H{},
				"recommendedCancerType": nil,
			},
		})
		return
	}

	// 优化排序逻辑：优先选择样本覆盖率最高的检测类型，其次选择Panel数量最少的
	sort.Slice(matchedCancerTypes, func(i, j int) bool {
		// 首先按样本覆盖率降序排序
		coverageI := matchedCancerTypes[i]["sampleCoverageRate"].(float64)
		coverageJ := matchedCancerTypes[j]["sampleCoverageRate"].(float64)
		if coverageI != coverageJ {
			return coverageI > coverageJ
		}
		// 如果覆盖率相同，选择Panel数量最少的（更精确匹配）
		return matchedCancerTypes[i]["panelCount"].(int) < matchedCancerTypes[j]["panelCount"].(int)
	})

	recommendedCancerType := matchedCancerTypes[0]

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "检测类型匹配成功",
		Data: utils.H{
			"matchedCancerTypes":    matchedCancerTypes,
			"recommendedCancerType": recommendedCancerType,
		},
	})
}

// HandleSetSampleReceiveDate - 为样本设置接收时间
// 接收参数：batch_id 或 batch_code，sample_code，receive_date
// 提供"同检测时间"选项，允许将 receive_date 设置为 batch_start_time 或 batch_stop_time
// 更新样本的 receive_date 和 receive_operator（当前用户）
// 更新样本状态为 'received'
func HandleSetSampleReceiveDate(c *app.RequestContext, db *sql.DB) {
	// 解析请求体
	var req struct {
		BatchId      int    `json:"batchId"`
		BatchCode    string `json:"batchCode"`
		SampleCode   string `json:"sampleCode" binding:"required"`
		ReceiveDate  string `json:"receiveDate"`  // 用户指定的接收时间
		UseBatchTime string `json:"useBatchTime"` // "start" 或 "stop"，表示使用批次的检测时间
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

	// 从上下文获取当前用户ID
	var receiveOperator int
	var receiveOperatorName string
	if userID, exists := c.Get("userID"); exists {
		receiveOperator = userID.(int)
		// 查询用户姓名
		err := db.QueryRow("SELECT real_name FROM base_manage_user WHERE id = ?", receiveOperator).Scan(&receiveOperatorName)
		if err != nil {
			log.Printf("Failed to get receive operator name: %v", err)
			receiveOperatorName = ""
		}
	}

	// 获取批次ID
	var batchId int
	if req.BatchId > 0 {
		batchId = req.BatchId
	} else if req.BatchCode != "" {
		err := db.QueryRow("SELECT id FROM detect_batch WHERE batch_code = ?", req.BatchCode).Scan(&batchId)
		if err != nil {
			c.JSON(consts.StatusNotFound, ApiResponse{
				Code:    404,
				Success: false,
				Message: "批次不存在",
				Data:    nil,
			})
			return
		}
	} else {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请提供批次ID或批次编号",
			Data:    nil,
		})
		return
	}

	// 查询批次信息，获取检测时间
	var batchStartTime, batchStopTime sql.NullTime
	err := db.QueryRow("SELECT batch_start_time, batch_stop_time FROM detect_batch WHERE id = ?", batchId).Scan(&batchStartTime, &batchStopTime)
	if err != nil {
		c.JSON(consts.StatusNotFound, ApiResponse{
			Code:    404,
			Success: false,
			Message: "批次不存在",
			Data:    nil,
		})
		return
	}

	// 确定接收时间
	var receiveDate time.Time
	if req.UseBatchTime == "start" && batchStartTime.Valid {
		receiveDate = batchStartTime.Time
	} else if req.UseBatchTime == "stop" && batchStopTime.Valid {
		receiveDate = batchStopTime.Time
	} else if req.ReceiveDate != "" {
		// 解析用户指定的接收时间
		parsedDate, err := time.Parse("2006-01-02 15:04:05", req.ReceiveDate)
		if err != nil {
			// 尝试其他格式
			parsedDate, err = time.Parse("2006-01-02T15:04:05", req.ReceiveDate)
			if err != nil {
				c.JSON(consts.StatusBadRequest, ApiResponse{
					Code:    400,
					Success: false,
					Message: "接收时间格式错误，请使用格式：2006-01-02 15:04:05",
					Data:    nil,
				})
				return
			}
		}
		receiveDate = parsedDate
	} else {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请提供接收时间或选择使用批次检测时间",
			Data: utils.H{
				"batchStartTime": formatNullTime(batchStartTime),
				"batchStopTime":  formatNullTime(batchStopTime),
			},
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
			Data:    nil,
		})
		return
	}

	// 检查样本是否存在
	var sampleId int
	var currentStatus string
	err = tx.QueryRow("SELECT id, sample_status FROM detect_sample WHERE sample_code = ?", req.SampleCode).Scan(&sampleId, &currentStatus)
	if err != nil {
		tx.Rollback()
		c.JSON(consts.StatusNotFound, ApiResponse{
			Code:    404,
			Success: false,
			Message: "样本不存在",
			Data:    nil,
		})
		return
	}

	// 更新样本的接收时间和接收操作员
	_, err = tx.Exec(`UPDATE detect_sample SET receive_date = ?, receive_operator = ?, sample_status = 'received', sample_updated_at = NOW() WHERE id = ?`,
		receiveDate, receiveOperator, sampleId)
	if err != nil {
		log.Printf("Failed to update sample receive date: %v", err)
		tx.Rollback()
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	// 提交事务
	if err = tx.Commit(); err != nil {
		log.Printf("Failed to commit transaction: %v", err)
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
		Message: "样本接收时间设置成功",
		Data: utils.H{
			"sampleCode":          req.SampleCode,
			"sampleId":            sampleId,
			"receiveDate":         receiveDate.Format("2006-01-02 15:04:05"),
			"receiveOperator":     receiveOperator,
			"receiveOperatorName": receiveOperatorName,
			"batchId":             batchId,
		},
	})
}

// HandleBatchSetSampleReceiveDate - 批量为多个样本设置接收时间
// 接收参数：batch_id 或 batch_code，sample_codes（数组），receive_date 或 useBatchTime
func HandleBatchSetSampleReceiveDate(c *app.RequestContext, db *sql.DB) {
	// 解析请求体
	var req struct {
		BatchId      int      `json:"batchId"`
		BatchCode    string   `json:"batchCode"`
		SampleCodes  []string `json:"sampleCodes" binding:"required"`
		ReceiveDate  string   `json:"receiveDate"`  // 用户指定的接收时间
		UseBatchTime string   `json:"useBatchTime"` // "start" 或 "stop"，表示使用批次的检测时间
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

	if len(req.SampleCodes) == 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请提供样本编号列表",
			Data:    nil,
		})
		return
	}

	// 从上下文获取当前用户ID
	var receiveOperator int
	var receiveOperatorName string
	if userID, exists := c.Get("userID"); exists {
		receiveOperator = userID.(int)
		// 查询用户姓名
		err := db.QueryRow("SELECT real_name FROM base_manage_user WHERE id = ?", receiveOperator).Scan(&receiveOperatorName)
		if err != nil {
			log.Printf("Failed to get receive operator name: %v", err)
			receiveOperatorName = ""
		}
	}

	// 获取批次ID
	var batchId int
	if req.BatchId > 0 {
		batchId = req.BatchId
	} else if req.BatchCode != "" {
		err := db.QueryRow("SELECT id FROM detect_batch WHERE batch_code = ?", req.BatchCode).Scan(&batchId)
		if err != nil {
			c.JSON(consts.StatusNotFound, ApiResponse{
				Code:    404,
				Success: false,
				Message: "批次不存在",
				Data:    nil,
			})
			return
		}
	} else {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请提供批次ID或批次编号",
			Data:    nil,
		})
		return
	}

	// 查询批次信息，获取检测时间
	var batchStartTime, batchStopTime sql.NullTime
	err := db.QueryRow("SELECT batch_start_time, batch_stop_time FROM detect_batch WHERE id = ?", batchId).Scan(&batchStartTime, &batchStopTime)
	if err != nil {
		c.JSON(consts.StatusNotFound, ApiResponse{
			Code:    404,
			Success: false,
			Message: "批次不存在",
			Data:    nil,
		})
		return
	}

	// 确定接收时间
	var receiveDate time.Time
	if req.UseBatchTime == "start" && batchStartTime.Valid {
		receiveDate = batchStartTime.Time
	} else if req.UseBatchTime == "stop" && batchStopTime.Valid {
		receiveDate = batchStopTime.Time
	} else if req.ReceiveDate != "" {
		// 解析用户指定的接收时间
		parsedDate, err := time.Parse("2006-01-02 15:04:05", req.ReceiveDate)
		if err != nil {
			// 尝试其他格式
			parsedDate, err = time.Parse("2006-01-02T15:04:05", req.ReceiveDate)
			if err != nil {
				c.JSON(consts.StatusBadRequest, ApiResponse{
					Code:    400,
					Success: false,
					Message: "接收时间格式错误，请使用格式：2006-01-02 15:04:05",
					Data:    nil,
				})
				return
			}
		}
		receiveDate = parsedDate
	} else {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请提供接收时间或选择使用批次检测时间",
			Data: utils.H{
				"batchStartTime": formatNullTime(batchStartTime),
				"batchStopTime":  formatNullTime(batchStopTime),
			},
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
			Data:    nil,
		})
		return
	}

	// 批量更新样本
	updatedCount := 0
	var updatedSamples []string
	for _, sampleCode := range req.SampleCodes {
		// 检查样本是否存在
		var sampleId int
		err = tx.QueryRow("SELECT id FROM detect_sample WHERE sample_code = ?", sampleCode).Scan(&sampleId)
		if err != nil {
			// 样本不存在，跳过
			continue
		}

		// 更新样本的接收时间和接收操作员
		_, err = tx.Exec(`UPDATE detect_sample SET receive_date = ?, receive_operator = ?, sample_status = 'received', sample_updated_at = NOW() WHERE id = ?`,
			receiveDate, receiveOperator, sampleId)
		if err != nil {
			log.Printf("Failed to update sample %s receive date: %v", sampleCode, err)
			continue
		}

		updatedCount++
		updatedSamples = append(updatedSamples, sampleCode)
	}

	// 提交事务
	if err = tx.Commit(); err != nil {
		log.Printf("Failed to commit transaction: %v", err)
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
		Message: "批量设置样本接收时间成功",
		Data: utils.H{
			"total":               len(req.SampleCodes),
			"updated":             updatedCount,
			"updatedSamples":      updatedSamples,
			"receiveDate":         receiveDate.Format("2006-01-02 15:04:05"),
			"receiveOperator":     receiveOperator,
			"receiveOperatorName": receiveOperatorName,
			"batchId":             batchId,
		},
	})
}
