package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"log"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"github.com/jung-kurt/gofpdf"
)

type reportPDFMode string

const (
	reportPDFModeFull    reportPDFMode = "full"
	reportPDFModeConcise reportPDFMode = "concise"
)

func formatReportProjectName(project, reportType string) string {
	project = strings.TrimSpace(project)
	if project == "" || (strings.Contains(strings.ToLower(project), "meplex") && strings.Contains(strings.ToLower(project), "cpg")) {
		return project
	}

	isUltraSensitive := strings.Contains(project, "超敏") || normalizeAssignedReportType(reportType) == "high"
	if isUltraSensitive {
		return project + "(MePlex超敏180CpG)"
	}
	return project + "(MePlex高敏98CpG)"
}

// 从身份证号提取生日并计算年龄函数
func calculateAge(idCard string) int {
	if idCard == "" {
		return 0
	}

	var birthDateStr string
	// 根据身份证号长度提取生日
	if len(idCard) == 18 {
		// 18位身份证：第7-14位是出生日期（YYYYMMDD）
		birthDateStr = idCard[6:14]
	} else if len(idCard) == 15 {
		// 15位身份证：第7-12位是出生日期（YYMMDD），需要补全为YYYYMMDD
		birthDateStr = "19" + idCard[6:12]
	} else {
		// 身份证号长度不正确
		return 0
	}

	// 解析生日日期
	birthDate, err := time.Parse("20060102", birthDateStr)
	if err != nil {
		return 0
	}

	// 计算年龄
	now := time.Now()
	age := now.Year() - birthDate.Year()

	// 调整年龄（如果生日还没过）
	if now.Month() < birthDate.Month() || (now.Month() == birthDate.Month() && now.Day() < birthDate.Day()) {
		age--
	}

	return age
}

// 生成PDF报告
func generatePDFReport(db *sql.DB, detect_reportId int) (string, error) {
	return generatePDFReportWithMode(db, detect_reportId, reportPDFModeFull)
}

func generateConcisePDFReport(db *sql.DB, detect_reportId int) (string, error) {
	return generatePDFReportWithMode(db, detect_reportId, reportPDFModeConcise)
}

func generatePDFReportWithMode(db *sql.DB, detect_reportId int, mode reportPDFMode) (string, error) {
	log.Printf("开始生成PDF报告，报告ID: %d", detect_reportId)
	// 从数据库查询报告信息，包括样本编号和癌种信息
	var detect_patientId, detect_sampleId, cancerTypeId, sampleTypeID int
	var detect_patientName, gender, organization, detect_sampleCode, detect_patientIdCard, cancerTypeName, assignedReportType string
	var createdAt time.Time
	var detect_reportData string

	err := db.QueryRow(`SELECT r.patient_id, r.report_data, 
		p.name as detect_patientName, p.gender, p.id_card, r.created_at, 
		s.sample_code as detect_sampleCode, 
		s.id as detect_sampleId,
		s.cancer_type_id, COALESCE(s.sample_type_id, 0), r.report_type,
		ct.name as cancerTypeName
	FROM detect_report r
	LEFT JOIN detect_patient p ON r.patient_id = p.id
	LEFT JOIN detect_sample s ON r.sample_id = s.id
	LEFT JOIN setting_cancer_type ct ON s.cancer_type_id = ct.id
	WHERE r.id = ?`, detect_reportId).Scan(
		&detect_patientId, &detect_reportData, &detect_patientName, &gender, &detect_patientIdCard, &createdAt, &detect_sampleCode, &detect_sampleId, &cancerTypeId, &sampleTypeID, &assignedReportType, &cancerTypeName)
	if err != nil {
		log.Printf("查询报告信息失败: %v", err)
		return "", fmt.Errorf("failed to query detect_report info: %v", err)
	}

	log.Printf("报告信息查询成功: 样本编号=%s, 患者姓名=%s, 癌种=%s", detect_sampleCode, detect_patientName, cancerTypeName)

	// 计算年龄
	age := calculateAge(detect_patientIdCard)

	// 解析报告数据获取其他字段
	var detect_reportDataMap map[string]interface{}
	if detect_reportData != "" {
		if err := json.Unmarshal([]byte(detect_reportData), &detect_reportDataMap); err != nil {
			log.Printf("解析报告数据失败: %v", err)
			detect_reportDataMap = make(map[string]interface{})
		}
	} else {
		detect_reportDataMap = make(map[string]interface{})
	}

	// 从样本表和样本类型表查询样本类型、组织信息、治疗阶段和采集时间
	var detect_sampleTypeFromDB, treatmentStageName string
	var detect_sampleCollectedAt time.Time
	err = db.QueryRow(`
		SELECT 
			COALESCE(st.name, ''), 
		COALESCE(s.organization, ''),
		COALESCE(ts.name, ''),
		COALESCE(s.receive_date, s.collection_date, s.sample_created_at)
	FROM detect_sample s 
	LEFT JOIN setting_sample_type st ON s.sample_type_id = st.id
	LEFT JOIN setting_treatment_stage ts ON s.treatment_stage_id = ts.id
		WHERE s.id = ?
	`, detect_sampleId).Scan(&detect_sampleTypeFromDB, &organization, &treatmentStageName, &detect_sampleCollectedAt)
	if err != nil {
		log.Printf("查询样本信息失败: %v", err)
		// 尝试从detect_reportDataMap获取样本信息
		if detect_reportDataMap != nil {
			if detect_sampleType, ok := detect_reportDataMap["detect_sampleType"].(string); ok && detect_sampleType != "" {
				detect_sampleTypeFromDB = detect_sampleType
				log.Printf("从detect_reportDataMap获取样本类型: %s", detect_sampleTypeFromDB)
			}
			if org, ok := detect_reportDataMap["organization"].(string); ok && org != "" {
				organization = org
				log.Printf("从detect_reportDataMap获取组织: %s", organization)
			}
			if treatmentStage, ok := detect_reportDataMap["treatmentStageName"].(string); ok && treatmentStage != "" {
				treatmentStageName = treatmentStage
				log.Printf("从detect_reportDataMap获取治疗阶段: %s", treatmentStageName)
			}
			// 尝试从detect_reportDataMap获取采集时间
			// 尝试多个可能的字段名
			var foundTime bool
			possibleFields := []string{"SampleTimedata", "detect_sampleCollectedAt", "collectedDate", "time"}
			for _, field := range possibleFields {
				if detect_sampleTimedata, ok := detect_reportDataMap[field].(string); ok && detect_sampleTimedata != "" {
					if t, err := time.Parse("2006-01-02", detect_sampleTimedata); err == nil {
						detect_sampleCollectedAt = t
						log.Printf("从detect_reportDataMap获取采集时间: %v", detect_sampleCollectedAt)
						foundTime = true
						break
					} else if t, err := time.Parse(time.RFC3339, detect_sampleTimedata); err == nil {
						detect_sampleCollectedAt = t
						log.Printf("从detect_reportDataMap获取采集时间: %v", detect_sampleCollectedAt)
						foundTime = true
						break
					} else {
						log.Printf("解析%s失败: %v", field, err)
					}
				}
			}
			// 尝试从selectedHistoricalReports中获取时间
			if !foundTime {
				if selectedHistoricalReports, ok := detect_reportDataMap["selectedHistoricalReports"].([]interface{}); ok && len(selectedHistoricalReports) > 0 {
					if firstReport, ok := selectedHistoricalReports[0].(map[string]interface{}); ok {
						if timeStr, ok := firstReport["time"].(string); ok && timeStr != "" {
							if t, err := time.Parse("2006-01-02", timeStr); err == nil {
								detect_sampleCollectedAt = t
								log.Printf("从selectedHistoricalReports获取采集时间: %v", detect_sampleCollectedAt)
								foundTime = true
							} else if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
								detect_sampleCollectedAt = t
								log.Printf("从selectedHistoricalReports获取采集时间: %v", detect_sampleCollectedAt)
								foundTime = true
							}
						}
					}
				}
			}
			if !foundTime {
				log.Printf("无法从detect_reportDataMap获取采集时间，使用空时间值")
				// 使用零值时间，不使用当前时间
				detect_sampleCollectedAt = time.Time{}
			}
		} else {
			log.Printf("无法从数据库获取样本信息且detect_reportDataMap为nil，使用空时间值")
			// 使用零值时间，不使用当前时间
			detect_sampleCollectedAt = time.Time{}
		}
		// 继续执行，使用默认值
	} else {
		log.Printf("从数据库获取样本类型: %s, 组织: %s, 治疗阶段: %s, 采集时间: %v", detect_sampleTypeFromDB, organization, treatmentStageName, detect_sampleCollectedAt)
	}

	currentReportRow := ReportHistoryRow{
		Time:   reportDateString(detect_reportDataMap["time1"]),
		Signal: reportFloatValue(detect_reportDataMap["signal1"]),
		Trend:  normalizeReportTrend(reportStringValue(detect_reportDataMap["trend1"])),
		Type:   reportStringValue(detect_reportDataMap["type1"]),
		Note:   reportStringValue(detect_reportDataMap["note1"]),
	}
	if currentReportRow.Time == "" && !detect_sampleCollectedAt.IsZero() {
		currentReportRow.Time = detect_sampleCollectedAt.Format("2006-01-02")
	}
	if currentReportRow.Signal == 0 {
		currentReportRow.Signal = reportFloatValue(detect_reportDataMap["calculationResult"])
	}
	if currentReportRow.Type == "" {
		currentReportRow.Type = treatmentStageName
	}
	if currentReportRow.Note == "" {
		currentReportRow.Note = reportStringValue(detect_reportDataMap["remarks"])
	}
	syncReportHistoryFields(detect_reportDataMap, currentReportRow, nil)

	// 查询样本上传者（检验者）和审核者信息
	var inspectorName, reviewerName string
	// 从样本表查询检验者
	err = db.QueryRow(`SELECT COALESCE(au.real_name, '') FROM detect_sample s
		LEFT JOIN detect_batch b ON s.batch_id = b.id
		LEFT JOIN base_manage_user au ON COALESCE(NULLIF(s.test_operator, 0), NULLIF(b.tester_id, 0)) = au.id
		WHERE s.id = ?`, detect_sampleId).Scan(&inspectorName)
	if err != nil {
		log.Printf("查询样本上传者失败: %v", err)
		inspectorName = ""
	}

	// 从报告表查询审核者
	err = db.QueryRow(`SELECT COALESCE(au.real_name, '') FROM detect_report r 
		LEFT JOIN base_manage_user au ON r.reviewed_by = au.id 
		WHERE r.id = ?`, detect_reportId).Scan(&reviewerName)
	if err != nil {
		log.Printf("查询报告审核者失败: %v", err)
		reviewerName = ""
	}

	// 查询患者之前的检测结果，按时间顺序，包含治疗阶段
	rows, err := db.Query(`SELECT r.report_no, r.created_at, r.report_data, 
			COALESCE(s.treatment_stage_id, 0) as treatment_stage_id,
			COALESCE(ts.name, '') as treatment_stage_name
			FROM detect_report r
			LEFT JOIN detect_sample s ON r.sample_id = s.id
			LEFT JOIN setting_treatment_stage ts ON s.treatment_stage_id = ts.id
			WHERE r.patient_id = ? AND r.id != ? AND r.status = 'reviewed'
			ORDER BY r.created_at DESC
			LIMIT 3`, detect_patientId, detect_reportId)
	if err != nil {
		log.Printf("查询历史检测结果失败: %v", err)
		// 继续执行，不中断PDF生成
	} else {
		defer rows.Close()
	}

	// 收集历史检测结果
	var previousResults []utils.H
	if rows != nil {
		for rows.Next() {
			var prevReportNo string
			var prevCreatedAt time.Time
			var prevReportData string
			var prevTreatmentStageId int
			var prevTreatmentStageName string

			err := rows.Scan(&prevReportNo, &prevCreatedAt, &prevReportData, &prevTreatmentStageId, &prevTreatmentStageName)
			if err != nil {
				log.Printf("扫描历史检测结果失败: %v", err)
				continue
			}

			// 解析报告数据
			var detect_reportMap map[string]interface{}
			if err := json.Unmarshal([]byte(prevReportData), &detect_reportMap); err == nil {
				// 提取信号值和类型
				var signalValue float64
				var resultType string

				// 尝试从不同可能的字段中获取信号值
				if score, ok := detect_reportMap["calculationResult"].(float64); ok {
					signalValue = score
				}

				// 计算趋势
				trend := ""
				if len(previousResults) > 0 {
					if prevSignalValue, ok := previousResults[0]["signalValue"].(float64); ok {
						switch calculateReportTrend(signalValue, prevSignalValue) {
						case "↑":
							trend = "上升"
						case "↓":
							trend = "下降"
						default:
							trend = "稳定"
						}
					}
				}

				// 确保organization是字符串类型
				var orgValue string
				if org, ok := detect_reportMap["organization"].(string); ok {
					orgValue = org
				} else {
					orgValue = ""
				}

				// 构建结果对象，包含治疗阶段
				resultObj := utils.H{
					"detect_reportNo":    prevReportNo,
					"createdAt":          prevCreatedAt,
					"signalValue":        signalValue,
					"resultType":         resultType,
					"trend":              trend,
					"organization":       orgValue,
					"treatmentStageName": prevTreatmentStageName,
				}

				previousResults = append(previousResults, resultObj)
			}
		}
	}

	// PDF只作为下载时的临时文件，不持久化到数据库或对象存储。
	detect_reportDir := filepath.Join("file", "temp", "detect_report")
	if err := os.MkdirAll(detect_reportDir, 0755); err != nil {
		log.Printf("创建报告目录失败: %v", err)
		return "", fmt.Errorf("failed to create detect_report directory: %v", err)
	}

	// 生成PDF文件路径，使用样本编号-姓名.pdf命名
	pdfFileName := fmt.Sprintf("%s-%s-%d.pdf", detect_sampleCode, detect_patientName, time.Now().UnixNano())
	pdfPath := filepath.Join(detect_reportDir, pdfFileName)
	log.Printf("PDF文件路径: %s", pdfPath)

	// 获取当前报告的信号值
	var currentSignalValue float64
	if score, ok := detect_reportDataMap["calculationResult"].(float64); ok {
		currentSignalValue = score
	}

	// 获取结果说明和信号值说明
	resultExplanation := ""
	signalValueExplanation := ""
	if re, ok := detect_reportDataMap["resultExplanation"].(string); ok {
		resultExplanation = re
	}
	if sve, ok := detect_reportDataMap["signalValueExplanation"].(string); ok {
		signalValueExplanation = sve
	}

	// 处理采集时间，当时间为空时显示为空白
	var detect_sampleTimeStr string
	if !detect_sampleCollectedAt.IsZero() {
		detect_sampleTimeStr = detect_sampleCollectedAt.Format("2006-01-02")
	} else {
		detect_sampleTimeStr = ""
	}

	// 填充PDF表单 - 管理端完整版拼接通用页，小程序简洁版只生成报告页。
	err = FillPDFFormFixed(db, sampleTypeID, assignedReportType, pdfPath, mode, detect_patientName, gender, age, organization,
		createdAt, previousResults, detect_sampleTypeFromDB, detect_sampleTimeStr, inspectorName, reviewerName, currentSignalValue, resultExplanation, signalValueExplanation, detect_sampleId, detect_sampleCode, cancerTypeName, treatmentStageName, detect_reportDataMap)
	if err != nil {
		log.Printf("填充PDF表单失败: %v", err)
		return "", fmt.Errorf("failed to fill PDF form: %v", err)
	}
	log.Printf("填充PDF表单成功，路径: %s", pdfPath)

	log.Printf("PDF报告生成完成，路径: %s", pdfPath)
	return pdfPath, nil
}

// 复制字体文件
func copyFontFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = dstFile.ReadFrom(srcFile)
	return err
}

// 复制文件
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = dstFile.ReadFrom(srcFile)
	return err
}

// 使用pdfcpu stamp添加条形码到PDF
func addBarcodeWithPDFCPU(inputPath string, detect_sampleCode string) (string, error) {
	log.Printf("使用pdfcpu stamp添加条形码，输入路径: %s, 样本编号: %s", inputPath, detect_sampleCode)

	// 创建输出路径
	outputPath := inputPath + ".embedded.pdf"

	// 确保temp目录存在
	tempDir := filepath.Join("file", "temp")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		log.Printf("创建临时目录失败: %v", err)
		return inputPath, nil
	}

	// 生成CODE128格式的条形码
	log.Printf("生成条形码...")
	code128Code, err := code128.Encode(detect_sampleCode)
	if err != nil {
		log.Printf("生成条形码失败: %v", err)
		return inputPath, nil
	}

	// 调整条形码大小（增大尺寸，使其更清晰）
	code128Barcode, err := barcode.Scale(code128Code, 300, 100) // 增大为300x100
	if err != nil {
		log.Printf("调整条形码大小失败: %v", err)
		return inputPath, nil
	}

	// 将条形码转换为8位深度的RGBA图像，并在下方添加样本编号
	bounds := code128Barcode.Bounds()

	// 创建更大的图像，为文本留出空间
	textHeight := 40 // 文本高度（增大以适应更大的字体）
	combinedBounds := image.Rect(0, 0, bounds.Dx(), bounds.Dy()+textHeight)
	rgba := image.NewRGBA(combinedBounds)

	// 填充白色背景
	for x := 0; x < combinedBounds.Dx(); x++ {
		for y := 0; y < combinedBounds.Dy(); y++ {
			rgba.Set(x, y, color.White)
		}
	}

	// 绘制条形码
	for x := 0; x < bounds.Dx(); x++ {
		for y := 0; y < bounds.Dy(); y++ {
			c := code128Barcode.At(x, y)
			r, g, b, a := c.RGBA()
			// 检查是否是黑色（条形码的条）
			if r == 0 && g == 0 && b == 0 && a == 0xffff {
				// 黑色条
				rgba.Set(x, y, color.Black)
			} else {
				// 白色背景
				rgba.Set(x, y, color.White)
			}
		}
	}

	// 在条形码下方添加样本编号文本
	// 使用freetype库加载字体文件并绘制清晰的文本
	textY := bounds.Dy() + 10 // 条形码下方15像素（增加空间以适应更大的字体）
	// 尝试加载字体文件 - 优先使用宋体
	// 首先检查是否存在宋体字体文件
	fontPath := "C:\\Windows\\Fonts\\simsun.ttc" // 宋体
	if _, err := os.Stat(fontPath); os.IsNotExist(err) {
		// 如果宋体不存在，尝试其他常见字体路径
		fontPath = "C:\\Windows\\Fonts\\arial.ttf" // Windows系统默认字体路径
		if _, err := os.Stat(fontPath); os.IsNotExist(err) {
			log.Printf("未找到字体文件，使用备用方法绘制文本")
			// 备用方法：使用黑色矩形
			textWidth := len(detect_sampleCode) * 10
			textX := (combinedBounds.Dx() - textWidth) / 2
			for i := 0; i < len(detect_sampleCode); i++ {
				charX := textX + i*10
				for dx := 0; dx < 8; dx++ {
					for dy := 0; dy < 12; dy++ {
						rgba.Set(charX+dx, textY+dy, color.Black)
					}
				}
			}
			// 继续执行，不返回
		}
	}

	// 加载字体文件
	fontBytes, err := os.ReadFile(fontPath)
	if err != nil {
		log.Printf("加载字体文件失败: %v", err)
		// 备用方法
		textWidth := len(detect_sampleCode) * 10
		textX := (combinedBounds.Dx() - textWidth) / 2
		for i := 0; i < len(detect_sampleCode); i++ {
			charX := textX + i*10
			for dx := 0; dx < 8; dx++ {
				for dy := 0; dy < 12; dy++ {
					rgba.Set(charX+dx, textY+dy, color.Black)
				}
			}
		}
		// 继续执行，不返回
	}
	// 解析字体
	font, err := truetype.Parse(fontBytes)
	if err != nil {
		log.Printf("解析字体失败: %v", err)
		// 备用方法
		textWidth := len(detect_sampleCode) * 10
		textX := (combinedBounds.Dx() - textWidth) / 2
		for i := 0; i < len(detect_sampleCode); i++ {
			charX := textX + i*10
			for dx := 0; dx < 8; dx++ {
				for dy := 0; dy < 12; dy++ {
					rgba.Set(charX+dx, textY+dy, color.Black)
				}
			}
		}
		// 继续执行，不返回
	}
	// 创建freetype上下文
	c := freetype.NewContext()
	c.SetDPI(96)
	c.SetFont(font)
	c.SetFontSize(20) // 增大字体大小为16，使其更清晰
	c.SetClip(combinedBounds)
	c.SetDst(rgba)
	c.SetSrc(image.NewUniform(color.Black))
	// 计算文本位置（居中显示）
	textWidth := float64(len(detect_sampleCode)) * 12 // 估算文本宽度（增大以适应更大的字体）
	textX := (float64(combinedBounds.Dx()) - textWidth) / 2
	// 绘制文本
	pt := freetype.Pt(int(textX), textY+16) // 调整Y位置以适应更大的字体
	_, err = c.DrawString(detect_sampleCode, pt)
	if err != nil {
		log.Printf("绘制文本失败: %v", err)
	}
	// 将条形码保存为临时文件
	barcodePath := filepath.Join(tempDir, detect_sampleCode+"_barcode.png")
	f, err := os.Create(barcodePath)
	if err != nil {
		log.Printf("创建条形码文件失败: %v", err)
		return inputPath, nil
	}
	// 编码为PNG格式
	err = png.Encode(f, rgba)
	f.Close() // 立即关闭文件
	if err != nil {
		log.Printf("保存条形码失败: %v", err)
		os.Remove(barcodePath)
		return inputPath, nil
	}
	// 构建配置字符串
	// 位置：右上角(tr)，不旋转，偏移量20 20，缩放0.3
	stampConfig := "position:tr, rot:0, offset:-20 -20, scalef:0.3 abs"

	log.Printf("使用pdfcpu添加条形码到PDF...")
	cmd := exec.Command("pdfcpu",
		"stamp", "add",
		"-p", "1", // 第一页
		"-m", "image", // 模式：图片
		"--",        // 分隔符
		barcodePath, // 图片文件（string|file参数）
		stampConfig, // 描述配置（description参数）
		inputPath,   // 输入文件
		outputPath)  // 输出文件（可选）

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("pdfcpu添加条形码失败: %v, 输出: %s", err, string(output))
	}

	// 删除临时条形码文件
	os.Remove(barcodePath)

	log.Printf("PDF处理成功，输出路径: %s", outputPath)
	return outputPath, nil
}

// ReportData 存储PDF填写所需的所有数据
type ReportData struct {
	PatientName            string
	Gender                 string
	Age                    int
	Organization           string
	CreatedAt              time.Time
	PreviousResults        []utils.H
	SampleType             string
	SampleTime             string
	Inspector              string
	Reviewer               string
	CurrentSignalValue     float64
	ResultExplanation      string
	SignalValueExplanation string
	SampleId               int
	SampleCode             string
	CancerTypeName         string
	TreatmentStageName     string
	ReportDataMap          map[string]interface{}
}

// fillPage 函数用于填写指定页面的内容
func fillPage(pdf *gofpdf.Fpdf, pageNum int, data ReportData) {
	switch pageNum {
	case 1:
		// 第1页：填写NameP1, SexP1, AgeP1
		pdf.SetFont("NotoSansSC", "", 16)
		pdf.SetXY(50.0, 138.0)
		pdf.Cell(0, 0, data.PatientName)

		pdf.SetXY(90.0, 138.0)
		pdf.Cell(0, 0, data.Gender)

		pdf.SetXY(120.0, 138.0)
		pdf.Cell(0, 0, fmt.Sprintf("%d", data.Age))

		// 第1页：添加条形码
		if err := addBarcodeWithGofpdf(pdf, data.SampleCode); err != nil {
			log.Printf("添加条形码失败: %v", err)
		}
	case 3:
		// 第3页：填写NameP2, SexP2, AgeP2, Organization, ReportTime, SampleType, SampleTime, Inspector, Reviewer, Time01, Single1, NumberID, Project
		pdf.SetFont("NotoSansSC", "", 10)
		// NameP2
		pdf.SetXY(30, 72.5)
		pdf.Cell(0, 0, data.PatientName)
		// SexP2
		pdf.SetXY(62, 72.5)
		pdf.Cell(0, 0, data.Gender)
		// AgeP2
		pdf.SetXY(92, 72.5)
		pdf.Cell(0, 0, fmt.Sprintf("%d", data.Age))

		// SampleType
		pdf.SetXY(149.0, 71.5)
		pdf.Cell(0, 0, data.SampleType)
		// SampleTime
		pdf.SetXY(149.0, 81.0)
		pdf.Cell(0, 0, data.SampleTime)
		// Project
		pdf.SetXY(30.0, 87.3)
		pdf.Cell(0, 0, data.CancerTypeName)
		// NumberID
		pdf.SetXY(92, 87.3)
		pdf.Cell(0, 0, data.SampleCode)
		// Organization
		pdf.SetXY(149.0, 90.0)
		pdf.Cell(0, 0, data.Organization)

		// Inspector
		pdf.SetXY(32, 251.2)
		pdf.Cell(0, 0, data.Inspector)
		// Reviewer
		pdf.SetXY(105, 251.2)
		pdf.Cell(0, 0, data.Reviewer)
		// ReportTime
		pdf.SetXY(175.5, 251.2)
		pdf.Cell(0, 0, data.CreatedAt.Format("2006-01-02"))

		// 填写Time1, Signal1, Tend1, Type1, Note1和Time2, 3, 4
		pdf.SetFont("NotoSansSC", "", 10)

		// 从报告数据中获取TIME, SIGNAL, TREND, TYPE, NOTE字段
		getReportDataString := func(key string) string {
			if data.ReportDataMap != nil {
				// 尝试获取值
				if value, ok := data.ReportDataMap[key]; ok {
					// 根据值的类型进行转换
					switch v := value.(type) {
					case string:
						if v == "" && key == "time1" {
							return data.CreatedAt.Format("2006-01-02")
						}
						if v == "" && key == "trend1" {
							return "-"
						}
						// 检查是否是时间字段（key包含"time"）且值是ISO 8601格式
						if strings.Contains(key, "time") {
							// 尝试解析ISO 8601格式的时间
							if t, err := time.Parse(time.RFC3339, v); err == nil {
								// 转换为YYYY-MM-DD格式
								return t.Format("2006-01-02")
							}
							// 尝试解析其他常见的时间格式
							if t, err := time.Parse("2006-01-02T15:04:05Z07:00", v); err == nil {
								return t.Format("2006-01-02")
							}
						}
						return v
					case int:
						return fmt.Sprintf("%d", v)
					case float64:
						return fmt.Sprintf("%f", v)
					case bool:
						if v {
							return "是"
						} else {
							return "否"
						}
					default:
						return fmt.Sprintf("%v", v)
					}
				}

				// 处理特殊字段
				if key == "time1" {
					// 尝试使用报告时间
					if value, ok := data.ReportDataMap["detect_reportTime"]; ok {
						switch v := value.(type) {
						case string:
							if t, err := time.Parse(time.RFC3339, v); err == nil {
								return t.Format("2006-01-02")
							}
							if t, err := time.Parse("2006-01-02T15:04:05Z07:00", v); err == nil {
								return t.Format("2006-01-02")
							}
							return v
						}
					}
					return data.CreatedAt.Format("2006-01-02")
				} else if key == "trend1" {
					// 尝试使用trend字段
					if value, ok := data.ReportDataMap["trend"]; ok {
						switch v := value.(type) {
						case string:
							return v
						default:
							return fmt.Sprintf("%v", v)
						}
					}
					return "-"
				} else if key == "type1" {
					// 尝试使用treatmentStageName字段
					if value, ok := data.ReportDataMap["treatmentStageName"]; ok {
						switch v := value.(type) {
						case string:
							return v
						default:
							return fmt.Sprintf("%v", v)
						}
					}
					return ""
				} else if key == "time2" || key == "signal2" || key == "trend2" || key == "type2" || key == "note2" ||
					key == "time3" || key == "signal3" || key == "trend3" || key == "type3" || key == "note3" ||
					key == "time4" || key == "signal4" || key == "trend4" || key == "type4" || key == "note4" {
					// 尝试从selectedHistoricalReports数组获取历史检测数据
					if selectedHistoricalReports, ok := data.ReportDataMap["selectedHistoricalReports"].([]interface{}); ok {
						// 确定数组索引（time2对应索引0，time3对应索引1，time4对应索引2）
						index := -1
						switch key {
						case "time2", "signal2", "trend2", "type2", "note2":
							index = 0
						case "time3", "signal3", "trend3", "type3", "note3":
							index = 1
						case "time4", "signal4", "trend4", "type4", "note4":
							index = 2
						}

						// 检查索引是否有效
						if index >= 0 && index < len(selectedHistoricalReports) {
							if detect_report, ok := selectedHistoricalReports[index].(map[string]interface{}); ok {
								// 确定要获取的字段名
								fieldName := ""
								switch key {
								case "time2", "time3", "time4":
									fieldName = "time"
								case "signal2", "signal3", "signal4":
									fieldName = "signal"
								case "trend2", "trend3", "trend4":
									fieldName = "trend"
								case "type2", "type3", "type4":
									fieldName = "type"
								case "note2", "note3", "note4":
									fieldName = "note"
								}

								// 获取字段值
								if value, ok := detect_report[fieldName]; ok {
									switch v := value.(type) {
									case string:
										return v
									case float64:
										return fmt.Sprintf("%.1f", v)
									case int:
										return fmt.Sprintf("%d", v)
									default:
										return fmt.Sprintf("%v", v)
									}
								}
							}
						}
					}

					// 如果是signal字段，返回0
					if strings.Contains(key, "signal") {
						return "0"
					}
					return ""
				}
			}
			return ""
		}

		getReportDataFloat := func(key string) float64 {
			if data.ReportDataMap != nil {
				// 尝试获取值
				if value, ok := data.ReportDataMap[key]; ok {
					// 根据值的类型进行转换
					switch v := value.(type) {
					case float64:
						return v
					case int:
						return float64(v)
					case string:
						// 尝试将字符串转换为float64
						if f, err := strconv.ParseFloat(v, 64); err == nil {
							return f
						}
					}
				}

				// 处理特殊字段
				if key == "signal1" {
					// 尝试使用calculationResult
					if value, ok := data.ReportDataMap["calculationResult"]; ok {
						switch v := value.(type) {
						case float64:
							return v
						case int:
							return float64(v)
						case string:
							if f, err := strconv.ParseFloat(v, 64); err == nil {
								return f
							}
						}
					}
				} else if key == "signal2" || key == "signal3" || key == "signal4" {
					// 尝试从selectedHistoricalReports数组获取历史检测数据
					if selectedHistoricalReports, ok := data.ReportDataMap["selectedHistoricalReports"].([]interface{}); ok {
						// 确定数组索引（signal2对应索引0，signal3对应索引1，signal4对应索引2）
						index := -1
						switch key {
						case "signal2":
							index = 0
						case "signal3":
							index = 1
						case "signal4":
							index = 2
						}

						// 检查索引是否有效
						if index >= 0 && index < len(selectedHistoricalReports) {
							if detect_report, ok := selectedHistoricalReports[index].(map[string]interface{}); ok {
								// 获取signal字段值
								if value, ok := detect_report["signal"]; ok {
									switch v := value.(type) {
									case float64:
										return v
									case int:
										return float64(v)
									case string:
										if f, err := strconv.ParseFloat(v, 64); err == nil {
											return f
										}
									}
								}
							}
						}
					}
				}
			}
			return 0
		}

		// 检查是否有数据的函数
		hasData := func(timeKey string, signalKey string) bool {
			// 对于本次结果（time1, signal1），总是返回true，确保它被绘制
			if timeKey == "time1" && signalKey == "signal1" {
				return true
			}

			timeValue := getReportDataString(timeKey)
			signalValue := getReportDataFloat(signalKey)
			return timeValue != "" || signalValue > 0
		}

		// 填写报告数据，只显示有数据的行
		yOffset := 107.3
		rowHeight := 7.25

		// 报告1
		if hasData("time1", "signal1") {
			// Time1 (检测时间1) - 居中
			time1Text := getReportDataString("time1")
			time1X := 61.5
			time1Width := 27.35
			time1TextWidth := pdf.GetStringWidth(time1Text)
			pdf.SetXY(time1X+(time1Width-time1TextWidth)/2, yOffset+3.6) // 加1mm调整垂直位置
			pdf.Cell(0, 0, time1Text)

			// Signal1 (信号值1) - 居中
			signal1Value := getReportDataFloat("signal1")
			signal1Text := fmt.Sprintf("%.1f", signal1Value)
			signal1X := 88.0
			signal1Width := 17.61
			signal1TextWidth := pdf.GetStringWidth(signal1Text)
			pdf.SetXY(signal1X+(signal1Width-signal1TextWidth)/2, yOffset+3.6)
			pdf.Cell(0, 0, signal1Text)

			// Tend1 (趋势1) - 居中
			tend1Text := getReportDataString("trend1")
			tend1X := 100.8
			tend1Width := 30.48
			tend1TextWidth := pdf.GetStringWidth(tend1Text)
			pdf.SetXY(tend1X+(tend1Width-tend1TextWidth)/2, yOffset+3.6)
			pdf.Cell(0, 0, tend1Text)

			// Type1 (检测类型1) - 居中
			type1Text := getReportDataString("type1")
			type1X := 129.2
			type1Width := 19.6
			type1TextWidth := pdf.GetStringWidth(type1Text)
			pdf.SetXY(type1X+(type1Width-type1TextWidth)/2, yOffset+3.6)
			pdf.Cell(0, 0, type1Text)

			// Note1 (备注1) - 居中
			note1Text := getReportDataString("note1")
			note1X := 126.6
			note1Width := 40.0
			note1TextWidth := pdf.GetStringWidth(note1Text)
			pdf.SetXY(note1X+(note1Width-note1TextWidth)/2, yOffset+3.6)
			pdf.Cell(0, 0, note1Text)

			yOffset += rowHeight
		}

		// 报告2
		if hasData("time2", "signal2") {
			// Time2 (检测时间2) - 居中
			time2Text := getReportDataString("time2")
			time2X := 61.5
			time2Width := 27.35
			time2TextWidth := pdf.GetStringWidth(time2Text)
			pdf.SetXY(time2X+(time2Width-time2TextWidth)/2, yOffset+3.6)
			pdf.Cell(0, 0, time2Text)

			// Signal2 (信号值2) - 居中
			signal2Value := getReportDataFloat("signal2")
			signal2Text := fmt.Sprintf("%.1f", signal2Value)
			signal2X := 88.0
			signal2Width := 17.61
			signal2TextWidth := pdf.GetStringWidth(signal2Text)
			pdf.SetXY(signal2X+(signal2Width-signal2TextWidth)/2, yOffset+3.6)
			pdf.Cell(0, 0, signal2Text)

			// Tend2 (趋势2) - 居中
			tend2Text := getReportDataString("trend2")
			tend2X := 100.8
			tend2Width := 30.48
			tend2TextWidth := pdf.GetStringWidth(tend2Text)
			pdf.SetXY(tend2X+(tend2Width-tend2TextWidth)/2, yOffset+3.6)
			pdf.Cell(0, 0, tend2Text)

			// Type2 (检测类型2) - 居中
			type2Text := getReportDataString("type2")
			type2X := 129.2
			type2Width := 19.6
			type2TextWidth := pdf.GetStringWidth(type2Text)
			pdf.SetXY(type2X+(type2Width-type2TextWidth)/2, yOffset+3.6)
			pdf.Cell(0, 0, type2Text)

			// Note2 (备注2) - 居中
			note2Text := getReportDataString("note2")
			note2X := 126.6
			note2Width := 40.0
			note2TextWidth := pdf.GetStringWidth(note2Text)
			pdf.SetXY(note2X+(note2Width-note2TextWidth)/2, yOffset+3.6)
			pdf.Cell(0, 0, note2Text)

			yOffset += rowHeight
		}

		// 报告3
		if hasData("time3", "signal3") {
			// Time3 (检测时间3) - 居中
			time3Text := getReportDataString("time3")
			time3X := 61.5
			time3Width := 27.35
			time3TextWidth := pdf.GetStringWidth(time3Text)
			pdf.SetXY(time3X+(time3Width-time3TextWidth)/2, yOffset+3.6)
			pdf.Cell(0, 0, time3Text)

			// Signal3 (信号值3) - 居中
			signal3Value := getReportDataFloat("signal3")
			signal3Text := fmt.Sprintf("%.1f", signal3Value)
			signal3X := 88.0
			signal3Width := 17.61
			signal3TextWidth := pdf.GetStringWidth(signal3Text)
			pdf.SetXY(signal3X+(signal3Width-signal3TextWidth)/2, yOffset+3.6)
			pdf.Cell(0, 0, signal3Text)

			// Tend3 (趋势3) - 居中
			tend3Text := getReportDataString("trend3")
			tend3X := 100.8
			tend3Width := 30.48
			tend3TextWidth := pdf.GetStringWidth(tend3Text)
			pdf.SetXY(tend3X+(tend3Width-tend3TextWidth)/2, yOffset+3.6)
			pdf.Cell(0, 0, tend3Text)

			// Type3 (检测类型3) - 居中
			type3Text := getReportDataString("type3")
			type3X := 129.2
			type3Width := 19.6
			type3TextWidth := pdf.GetStringWidth(type3Text)
			pdf.SetXY(type3X+(type3Width-type3TextWidth)/2, yOffset+3.6)
			pdf.Cell(0, 0, type3Text)

			// Note3 (备注3) - 居中
			note3Text := getReportDataString("note3")
			note3X := 126.6
			note3Width := 40.0
			note3TextWidth := pdf.GetStringWidth(note3Text)
			pdf.SetXY(note3X+(note3Width-note3TextWidth)/2, yOffset+3.6)
			pdf.Cell(0, 0, note3Text)

			yOffset += rowHeight
		}

		// 报告4
		if hasData("time4", "signal4") {
			// Time4 (检测时间4) - 居中
			time4Text := getReportDataString("time4")
			time4X := 61.5
			time4Width := 27.35
			time4TextWidth := pdf.GetStringWidth(time4Text)
			pdf.SetXY(time4X+(time4Width-time4TextWidth)/2, yOffset+3.6)
			pdf.Cell(0, 0, time4Text)

			// Signal4 (信号值4) - 居中
			signal4Value := getReportDataFloat("signal4")
			signal4Text := fmt.Sprintf("%.1f", signal4Value)
			signal4X := 88.0
			signal4Width := 17.61
			signal4TextWidth := pdf.GetStringWidth(signal4Text)
			pdf.SetXY(signal4X+(signal4Width-signal4TextWidth)/2, yOffset+3.6)
			pdf.Cell(0, 0, signal4Text)

			// Tend4 (趋势4) - 居中
			tend4Text := getReportDataString("trend4")
			tend4X := 100.8
			tend4Width := 30.48
			tend4TextWidth := pdf.GetStringWidth(tend4Text)
			pdf.SetXY(tend4X+(tend4Width-tend4TextWidth)/2, yOffset+3.6)
			pdf.Cell(0, 0, tend4Text)

			// Type4 (检测类型4) - 居中
			type4Text := getReportDataString("type4")
			type4X := 129.2
			type4Width := 19.6
			type4TextWidth := pdf.GetStringWidth(type4Text)
			pdf.SetXY(type4X+(type4Width-type4TextWidth)/2, yOffset+3.6)
			pdf.Cell(0, 0, type4Text)

			// Note4 (备注4) - 居中
			note4Text := getReportDataString("note4")
			note4X := 126.6
			note4Width := 40.0
			note4TextWidth := pdf.GetStringWidth(note4Text)
			pdf.SetXY(note4X+(note4Width-note4TextWidth)/2, yOffset+3.6)
			pdf.Cell(0, 0, note4Text)

			yOffset += rowHeight
		}

		// SignalInstructions - 多行文本
		pdf.SetXY(42.5, 136.5) // 根据模板调整位置
		drawPDFMultilineText(pdf, 42.5, 136.5, safePDFTextBoxWidth(150), 5, data.SignalValueExplanation)
		// ResultInstructions - 多行文本
		pdf.SetXY(42.5, 154.0) // 根据模板调整位置
		drawPDFMultilineText(pdf, 42.5, 154.0, safePDFTextBoxWidth(150), 5, data.ResultExplanation)
	}
}

// 修复后的填充PDF表单函数 - 使用gofpdf
func FillPDFFormFixed(db *sql.DB, sampleTypeID int, assignedReportType, outputPath string, mode reportPDFMode, detect_patientName, gender string, age int, organization string,
	createdAt time.Time, previousResults []utils.H, detect_sampleType, detect_sampleTime string, inspector, reviewer string, currentSignalValue float64, resultExplanation, signalValueExplanation string, detect_sampleId int, detect_sampleCode string, cancerTypeName string, treatmentStageName string, detect_reportDataMap map[string]interface{}) error {

	// 创建gofpdf实例，使用横向（L）作为默认页面方向，因为第一页是横向的
	pdf := gofpdf.New("P", "mm", "A4", "")
	var err error

	// 使用NotoSansSC字体
	fontPath := "fonts/NotoSansSC.ttf"
	if _, err := os.Stat(fontPath); err != nil {
		log.Printf("NotoSansSC.ttf字体文件不存在: %v", err)
		return fmt.Errorf("font file not found: %s", fontPath)
	}

	// 设置字体位置并添加字体
	pdf.SetFontLocation("fonts")
	// 使用AddUTF8Font函数添加字体，不需要json文件
	pdf.AddUTF8Font("NotoSansSC", "", "NotoSansSC.ttf")
	log.Printf("使用NotoSansSC字体")
	pdf.SetFont("NotoSansSC", "", 12)

	// 构建ReportData
	data := ReportData{
		PatientName:            detect_patientName,
		Gender:                 gender,
		Age:                    age,
		Organization:           organization,
		CreatedAt:              createdAt,
		PreviousResults:        previousResults,
		SampleType:             detect_sampleType,
		SampleTime:             detect_sampleTime,
		Inspector:              inspector,
		Reviewer:               reviewer,
		CurrentSignalValue:     currentSignalValue,
		ResultExplanation:      resultExplanation,
		SignalValueExplanation: signalValueExplanation,
		SampleId:               detect_sampleId,
		SampleCode:             detect_sampleCode,
		CancerTypeName:         formatReportProjectName(cancerTypeName, assignedReportType),
		TreatmentStageName:     treatmentStageName,
		ReportDataMap:          detect_reportDataMap,
	}
	positionConfig := resolveReportPosition(db, sampleTypeID, assignedReportType)
	if normalizeAssignedReportType(assignedReportType) == "high" {
		data.Organization = ""
	}
	log.Printf("报告第三页定位: %s (%s), sample_type_id=%d, report_type=%s",
		positionConfig.PositionName, positionConfig.BackgroundPath, sampleTypeID, assignedReportType)
	renderPages := generatePageRenderData(data)
	renderPageMap := make(map[int][]utils.H)
	for _, page := range renderPages {
		pageNumber, _ := page["pageNumber"].(int)
		elements, _ := page["elements"].([]utils.H)
		renderPageMap[pageNumber] = elements
	}

	if mode == reportPDFModeFull {
		// 完整版报告：通用页来自 Template_Universal，报告正文页来自 Template_Report。
		for i := 1; i <= 13; i++ {
			jpgPath := filepath.Join("Template", "Template_Universal", fmt.Sprintf("Template_01_%02d.jpg", i))
			if i == positionConfig.PageNumber {
				jpgPath = filepath.FromSlash(positionConfig.BackgroundPath)
			}
			if _, err := os.Stat(jpgPath); err != nil {
				if i == positionConfig.PageNumber {
					return fmt.Errorf("background image not found: %s", jpgPath)
				}
				log.Printf("Skip missing universal template page %d: %s", i, jpgPath)
				continue
			}
			if i == positionConfig.PageNumber {
				pdf.AddPageFormat("", gofpdf.SizeType{Wd: 210, Ht: 297})
				pdf.ImageOptions(jpgPath, 0, 0, 210, 297, false, gofpdf.ImageOptions{}, 0, "")
			} else {
				pdf.AddPageFormat("", gofpdf.SizeType{Wd: 297, Ht: 210})
				pdf.ImageOptions(jpgPath, 0, 0, 297, 210, false, gofpdf.ImageOptions{}, 0, "")
			}
			drawConfiguredPage(pdf, i, data, renderPageMap[i], positionConfig)
		}
	} else {
		reportPage := positionConfig.PageNumber
		jpgPath := filepath.FromSlash(positionConfig.BackgroundPath)
		if _, err := os.Stat(jpgPath); err != nil {
			return fmt.Errorf("background image not found: %s", jpgPath)
		}
		pdf.AddPageFormat("", gofpdf.SizeType{Wd: 210, Ht: 297})
		pdf.ImageOptions(jpgPath, 0, 0, 210, 297, false, gofpdf.ImageOptions{}, 0, "")
		drawConfiguredPage(pdf, reportPage, data, renderPageMap[reportPage], positionConfig)
	}

	// 保存PDF
	err = pdf.OutputFileAndClose(outputPath)
	if err != nil {
		return fmt.Errorf("failed to save PDF: %v", err)
	}

	return nil
}

// drawConfiguredPage 使用与前端画框相同的毫米坐标渲染内容。
func drawConfiguredPage(pdf *gofpdf.Fpdf, pageNumber int, data ReportData, elements []utils.H, config ReportPositionConfig) {
	for _, element := range elements {
		key := fmt.Sprint(element["key"])
		x, _ := element["x"].(float64)
		y, _ := element["y"].(float64)
		width := 80.0
		height := 6.0
		fontSize := 10.0
		if pageNumber == config.PageNumber {
			if position, ok := config.Positions[key]; ok {
				// Report position settings are absolute millimetre coordinates.
				// Adding the old element/default delta here moves calibrated rows twice.
				x, y = position.X, position.Y
				if position.Width > 0 {
					width = position.Width
				}
				if position.Height > 0 {
					height = position.Height
				}
				if position.FontSize > 0 {
					fontSize = position.FontSize
				}
			}
		}
		if pageNumber == 1 {
			fontSize = 16
		}
		pdf.SetFont("NotoSansSC", "", fontSize)
		content := fmt.Sprint(element["content"])
		if position, ok := config.Positions[key]; ok && position.Align == "center" {
			x += (width - pdf.GetStringWidth(content)) / 2
		}
		pdf.SetXY(x, y)
		if element["type"] == "multilineText" {
			lineHeight := 5.0
			if height > 0 && lineHeight > height {
				lineHeight = height
			}
			drawPDFMultilineText(pdf, x, y, safePDFTextBoxWidth(width), lineHeight, content)
		} else {
			// x/y 与旧模板一致，表示文字基线起点；宽高用于前端定位框和多行文本。
			pdf.Cell(0, 0, content)
		}
	}
	if pageNumber == 1 {
		if err := addBarcodeWithGofpdf(pdf, data.SampleCode); err != nil {
			log.Printf("添加条形码失败: %v", err)
		}
	}
}

func applyAbsoluteReportPosition(element utils.H, position ReportPositionValue) {
	element["x"] = position.X
	element["y"] = position.Y
	element["width"] = position.Width
	element["height"] = position.Height
	element["fontSize"] = position.FontSize
}

func safePDFTextBoxWidth(width float64) float64 {
	if width > 4 {
		return width - 3
	}
	return width
}

func drawPDFMultilineText(pdf *gofpdf.Fpdf, x, y, width, lineHeight float64, content string) {
	if width <= 0 {
		width = 80
	}
	if lineHeight <= 0 {
		lineHeight = 5
	}
	lines := splitPDFTextLines(pdf, content, width)
	for index, line := range lines {
		pdf.SetXY(x, y+float64(index)*lineHeight)
		pdf.CellFormat(width, lineHeight, line, "", 0, "L", false, 0, "")
	}
}

func splitPDFTextLines(pdf *gofpdf.Fpdf, content string, width float64) []string {
	rawLines := strings.Split(content, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, raw := range rawLines {
		if raw == "" {
			lines = append(lines, "")
			continue
		}
		current := ""
		for _, r := range []rune(raw) {
			next := current + string(r)
			if current != "" && pdf.GetStringWidth(next) > width {
				lines = append(lines, current)
				current = string(r)
				continue
			}
			current = next
		}
		lines = append(lines, current)
	}
	return lines
}

// 使用gofpdf添加条形码到PDF
func addBarcodeWithGofpdf(pdf *gofpdf.Fpdf, detect_sampleCode string) error {
	// 生成CODE128格式的条形码
	code128Code, err := code128.Encode(detect_sampleCode)
	if err != nil {
		return fmt.Errorf("failed to encode barcode: %v", err)
	}

	// 调整条形码大小（减小尺寸）
	code128Barcode, err := barcode.Scale(code128Code, 200, 60)
	if err != nil {
		return fmt.Errorf("failed to scale barcode: %v", err)
	}

	// 将条形码转换为灰度图像
	bounds := code128Barcode.Bounds()
	// 创建灰度图像
	grayImage := image.NewGray(bounds)

	// 填充白色背景并绘制条形码
	for x := 0; x < bounds.Dx(); x++ {
		for y := 0; y < bounds.Dy(); y++ {
			c := code128Barcode.At(x, y)
			// 将颜色转换为灰度
			gray := color.GrayModel.Convert(c)
			grayImage.Set(x, y, gray)
		}
	}

	// 保存条形码为临时PNG文件，使用样本编号作为前缀，防止冲突
	barcodePath := detect_sampleCode + "_barcode.png"
	f, err := os.Create(barcodePath)
	if err != nil {
		return fmt.Errorf("failed to create barcode file: %v", err)
	}

	// 使用png.Encode保存为灰度PNG
	err = png.Encode(f, grayImage)
	f.Close()
	if err != nil {
		return fmt.Errorf("failed to encode barcode to PNG: %v", err)
	}

	// 获取页面尺寸
	pageWidth, _ := pdf.GetPageSize()

	// 计算条形码尺寸
	barcodeWidth := 200.0 / 96 * 25.4 // 200px转换为mm
	barcodeHeight := 60.0 / 96 * 25.4 // 60px转换为mm

	// 计算右上角位置（距右边界20mm，距上边界20mm）
	barcodeX := pageWidth - 20 - barcodeWidth
	barcodeY := 20.0

	// 添加条形码到第1页的右上角
	pdf.ImageOptions(barcodePath, barcodeX, barcodeY, barcodeWidth, barcodeHeight, false, gofpdf.ImageOptions{}, 0, "")

	// 在条形码下方添加样本编号
	pdf.SetFont("NotoSansSC", "", 11)
	pdf.SetTextColor(0, 0, 0) // 黑色文本
	// 计算文本位置（条形码下方5mm，居中对齐）
	textWidth := pdf.GetStringWidth(detect_sampleCode)
	textX := barcodeX + (barcodeWidth-textWidth)/2
	textY := barcodeY + barcodeHeight + 5.0
	pdf.SetXY(textX, textY)
	pdf.Cell(0, 0, detect_sampleCode)

	// 清理临时文件
	os.Remove(barcodePath)

	return nil
}

// 生成随机密码
func generateRandomPassword(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
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

// 检查文件是否存在
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// ExtractReportPreviewData 提取报告预览数据
func ExtractReportPreviewData(db *sql.DB, detect_reportId int) (utils.H, error) {
	log.Printf("开始提取报告预览数据，报告ID: %d", detect_reportId)

	// 从数据库查询报告信息，包括样本编号和癌种信息
	var detect_patientId, detect_sampleId, cancerTypeId, sampleTypeID int
	var detect_patientName, gender, organization, detect_sampleCode, detect_patientIdCard, cancerTypeName, assignedReportType string
	var createdAt time.Time
	var detect_reportData string

	err := db.QueryRow(`SELECT r.patient_id, r.report_data, 
		p.name as detect_patientName, p.gender, p.id_card, r.created_at, 
		s.sample_code as detect_sampleCode, 
		s.id as detect_sampleId,
		s.cancer_type_id, COALESCE(s.sample_type_id, 0), r.report_type,
		ct.name as cancerTypeName
	FROM detect_report r
	LEFT JOIN detect_patient p ON r.patient_id = p.id
	LEFT JOIN detect_sample s ON r.sample_id = s.id
	LEFT JOIN setting_cancer_type ct ON s.cancer_type_id = ct.id
	WHERE r.id = ?`, detect_reportId).Scan(
		&detect_patientId, &detect_reportData, &detect_patientName, &gender, &detect_patientIdCard, &createdAt, &detect_sampleCode, &detect_sampleId, &cancerTypeId, &sampleTypeID, &assignedReportType, &cancerTypeName)
	if err != nil {
		log.Printf("查询报告信息失败: %v", err)
		return nil, fmt.Errorf("failed to query detect_report info: %v", err)
	}

	log.Printf("报告信息查询成功: 样本编号=%s, 患者姓名=%s, 癌种=%s", detect_sampleCode, detect_patientName, cancerTypeName)

	// 计算年龄
	age := calculateAge(detect_patientIdCard)

	// 解析报告数据获取其他字段
	var detect_reportDataMap map[string]interface{}
	if detect_reportData != "" {
		if err := json.Unmarshal([]byte(detect_reportData), &detect_reportDataMap); err != nil {
			log.Printf("解析报告数据失败: %v", err)
			detect_reportDataMap = make(map[string]interface{})
		}
	} else {
		detect_reportDataMap = make(map[string]interface{})
	}

	// 从样本表和样本类型表查询样本类型、组织信息、治疗阶段和采集时间
	var detect_sampleTypeFromDB, treatmentStageName string
	var detect_sampleCollectedAt time.Time
	err = db.QueryRow(`
		SELECT 
			COALESCE(st.name, ''), 
		COALESCE(s.organization, ''),
		COALESCE(ts.name, ''),
		COALESCE(s.receive_date, s.collection_date, s.sample_created_at)
	FROM detect_sample s 
	LEFT JOIN setting_sample_type st ON s.sample_type_id = st.id
	LEFT JOIN setting_treatment_stage ts ON s.treatment_stage_id = ts.id
		WHERE s.id = ?`, detect_sampleId).Scan(&detect_sampleTypeFromDB, &organization, &treatmentStageName, &detect_sampleCollectedAt)
	if err != nil {
		log.Printf("查询样本信息失败: %v", err)
		// 尝试从detect_reportDataMap获取样本信息
		if detect_reportDataMap != nil {
			if detect_sampleType, ok := detect_reportDataMap["sampleType"].(string); ok && detect_sampleType != "" {
				detect_sampleTypeFromDB = detect_sampleType
				log.Printf("从detect_reportDataMap获取样本类型: %s", detect_sampleTypeFromDB)
			}
			if org, ok := detect_reportDataMap["organization"].(string); ok && org != "" {
				organization = org
				log.Printf("从detect_reportDataMap获取组织: %s", organization)
			}
			if treatmentStage, ok := detect_reportDataMap["treatmentStageName"].(string); ok && treatmentStage != "" {
				treatmentStageName = treatmentStage
				log.Printf("从detect_reportDataMap获取治疗阶段: %s", treatmentStageName)
			}
			// 尝试从detect_reportDataMap获取采集时间
			var foundTime bool
			possibleFields := []string{"SampleTimedata", "detect_sampleCollectedAt", "collectedDate", "time"}
			for _, field := range possibleFields {
				if detect_sampleTimedata, ok := detect_reportDataMap[field].(string); ok && detect_sampleTimedata != "" {
					if t, err := time.Parse("2006-01-02", detect_sampleTimedata); err == nil {
						detect_sampleCollectedAt = t
						log.Printf("从detect_reportDataMap获取采集时间: %v", detect_sampleCollectedAt)
						foundTime = true
						break
					} else if t, err := time.Parse(time.RFC3339, detect_sampleTimedata); err == nil {
						detect_sampleCollectedAt = t
						log.Printf("从detect_reportDataMap获取采集时间: %v", detect_sampleCollectedAt)
						foundTime = true
						break
					}
				}
			}
			// 尝试从selectedHistoricalReports中获取时间
			if !foundTime {
				if selectedHistoricalReports, ok := detect_reportDataMap["selectedHistoricalReports"].([]interface{}); ok && len(selectedHistoricalReports) > 0 {
					if firstReport, ok := selectedHistoricalReports[0].(map[string]interface{}); ok {
						if timeStr, ok := firstReport["time"].(string); ok && timeStr != "" {
							if t, err := time.Parse("2006-01-02", timeStr); err == nil {
								detect_sampleCollectedAt = t
								log.Printf("从selectedHistoricalReports获取采集时间: %v", detect_sampleCollectedAt)
								foundTime = true
							} else if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
								detect_sampleCollectedAt = t
								log.Printf("从selectedHistoricalReports获取采集时间: %v", detect_sampleCollectedAt)
								foundTime = true
							}
						}
					}
				}
			}
		}
	} else {
		log.Printf("从数据库获取样本类型: %s, 组织: %s, 治疗阶段: %s, 采集时间: %v", detect_sampleTypeFromDB, organization, treatmentStageName, detect_sampleCollectedAt)
	}

	// 查询样本上传者（检验者）和审核者信息
	var inspectorName, reviewerName string
	// 从样本表查询检验者
	err = db.QueryRow(`SELECT COALESCE(au.real_name, '') FROM detect_sample s
		LEFT JOIN detect_batch b ON s.batch_id = b.id
		LEFT JOIN base_manage_user au ON COALESCE(NULLIF(s.test_operator, 0), NULLIF(b.tester_id, 0)) = au.id
		WHERE s.id = ?`, detect_sampleId).Scan(&inspectorName)
	if err != nil {
		log.Printf("查询样本上传者失败: %v", err)
		inspectorName = ""
	}

	// 从报告表查询审核者
	err = db.QueryRow(`SELECT COALESCE(au.real_name, '') FROM detect_report r 
		LEFT JOIN base_manage_user au ON r.reviewed_by = au.id 
		WHERE r.id = ?`, detect_reportId).Scan(&reviewerName)
	if err != nil {
		log.Printf("查询报告审核者失败: %v", err)
		reviewerName = ""
	}

	// 获取当前报告的信号值
	var currentSignalValue float64
	if score, ok := detect_reportDataMap["calculationResult"].(float64); ok {
		currentSignalValue = score
	}

	// 获取结果说明和信号值说明
	resultExplanation := ""
	signalValueExplanation := ""
	if re, ok := detect_reportDataMap["resultExplanation"].(string); ok {
		resultExplanation = re
	}
	if sve, ok := detect_reportDataMap["signalValueExplanation"].(string); ok {
		signalValueExplanation = sve
	}

	// 处理采集时间，当时间为空时显示为空白
	var detect_sampleTimeStr string
	if !detect_sampleCollectedAt.IsZero() {
		detect_sampleTimeStr = detect_sampleCollectedAt.Format("2006-01-02")
	} else {
		detect_sampleTimeStr = ""
	}

	// 构建报告数据对象
	reportData := ReportData{
		PatientName:            detect_patientName,
		Gender:                 gender,
		Age:                    age,
		Organization:           organization,
		CreatedAt:              createdAt,
		PreviousResults:        []utils.H{},
		SampleType:             detect_sampleTypeFromDB,
		SampleTime:             detect_sampleTimeStr,
		Inspector:              inspectorName,
		Reviewer:               reviewerName,
		CurrentSignalValue:     currentSignalValue,
		ResultExplanation:      resultExplanation,
		SignalValueExplanation: signalValueExplanation,
		SampleId:               detect_sampleId,
		SampleCode:             detect_sampleCode,
		CancerTypeName:         formatReportProjectName(cancerTypeName, assignedReportType),
		TreatmentStageName:     treatmentStageName,
		ReportDataMap:          detect_reportDataMap,
	}
	if normalizeAssignedReportType(assignedReportType) == "high" {
		reportData.Organization = ""
	}
	// 生成各页面的渲染数据，并应用与 PDF 生成相同的模板坐标。
	pages := generatePageRenderData(reportData)
	if normalizeAssignedReportType(assignedReportType) == "high" {
		removePreviewElementByKey(pages, "Organization")
	}
	positionConfig := resolveReportPosition(db, sampleTypeID, assignedReportType)
	for _, page := range pages {
		pageNumber, _ := page["pageNumber"].(int)
		if pageNumber != positionConfig.PageNumber {
			continue
		}
		page["backgroundPath"] = "/" + filepath.ToSlash(positionConfig.BackgroundPath)
		page["pageWidth"] = 210.0
		page["pageHeight"] = 297.0
		page["coordinateUnit"] = "mm"
		page["webAdjust"] = utils.H{
			"x": 0.0,
			"y": -2.0,
		}
		elements, _ := page["elements"].([]utils.H)
		for _, element := range elements {
			key := fmt.Sprint(element["key"])
			if key == "SignalInstructions" || key == "ResultInstructions" {
				element["webAdjustY"] = -1.5
			}
			if position, ok := positionConfig.Positions[key]; ok {
				applyAbsoluteReportPosition(element, position)
			}
		}
	}

	// 返回预览数据
	previewData := utils.H{
		"reportId":    detect_reportId,
		"sampleCode":  detect_sampleCode,
		"patientName": detect_patientName,
		"pages":       pages,
	}

	log.Printf("报告预览数据提取成功")
	return previewData, nil
}

// removePreviewElementByKey removes fields that are intentionally absent from
// a report template. Keeping an empty element would let the web preview render
// a placeholder even though the generated PDF correctly draws nothing.
func removePreviewElementByKey(pages []utils.H, key string) {
	for _, page := range pages {
		elements, _ := page["elements"].([]utils.H)
		filtered := make([]utils.H, 0, len(elements))
		for _, element := range elements {
			if fmt.Sprint(element["key"]) == key {
				continue
			}
			filtered = append(filtered, element)
		}
		page["elements"] = filtered
	}
}

func generateReportPreviewImage(db *sql.DB, detect_reportId int) (string, error) {
	previewData, err := ExtractReportPreviewData(db, detect_reportId)
	if err != nil {
		return "", err
	}

	pages, ok := previewData["pages"].([]utils.H)
	if !ok || len(pages) == 0 {
		return "", fmt.Errorf("report preview pages not found")
	}

	var page utils.H
	for _, item := range pages {
		if fmt.Sprint(item["backgroundPath"]) != "" {
			page = item
			break
		}
	}
	if page == nil {
		return "", fmt.Errorf("report preview background not found")
	}

	backgroundPath := strings.TrimPrefix(fmt.Sprint(page["backgroundPath"]), "/")
	backgroundPath = filepath.FromSlash(backgroundPath)
	file, err := os.Open(backgroundPath)
	if err != nil {
		return "", fmt.Errorf("open preview background failed: %w", err)
	}
	defer file.Close()

	background, _, err := image.Decode(file)
	if err != nil {
		return "", fmt.Errorf("decode preview background failed: %w", err)
	}
	bounds := background.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, background, bounds.Min, draw.Src)

	fontBytes, err := os.ReadFile("fonts/NotoSansSC.ttf")
	if err != nil {
		return "", fmt.Errorf("read preview font failed: %w", err)
	}
	font, err := truetype.Parse(fontBytes)
	if err != nil {
		return "", fmt.Errorf("parse preview font failed: %w", err)
	}

	elements, _ := page["elements"].([]utils.H)
	for _, element := range elements {
		elementType := fmt.Sprint(element["type"])
		isMultiline := elementType == "multiline" || elementType == "multilineText"
		if elementType != "text" && !isMultiline {
			continue
		}
		content := fmt.Sprint(element["content"])
		if strings.TrimSpace(content) == "" {
			continue
		}
		xMM := previewFloat(element["x"])
		yMM := previewFloat(element["y"])
		widthMM := previewFloat(element["width"])
		fontSize := previewFloat(element["fontSize"])
		if fontSize <= 0 {
			fontSize = 10
		}

		x := xMM / 210.0 * float64(bounds.Dx())
		y := yMM / 297.0 * float64(bounds.Dy())
		width := widthMM / 210.0 * float64(bounds.Dx())
		scale := float64(bounds.Dx()) / 595.0
		drawPreviewText(rgba, font, content, x, y, width, fontSize*scale, fmt.Sprint(element["align"]), isMultiline)
	}

	outputDir := filepath.Join("file", "temp", "detect_report_preview")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", err
	}
	outputPath := filepath.Join(outputDir, fmt.Sprintf("report_preview_%d_%d.png", detect_reportId, time.Now().UnixNano()))
	output, err := os.Create(outputPath)
	if err != nil {
		return "", err
	}
	defer output.Close()
	if err := png.Encode(output, rgba); err != nil {
		return "", err
	}
	return outputPath, nil
}

func previewFloat(value interface{}) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		parsed, _ := strconv.ParseFloat(v, 64)
		return parsed
	default:
		return 0
	}
}

func drawPreviewText(dst *image.RGBA, font *truetype.Font, content string, x, y, width, fontSize float64, align string, multiline bool) {
	if width <= 0 {
		width = 600
	}
	lines := splitPreviewLines(content, width, fontSize, multiline)
	lineHeight := fontSize * 1.25
	for index, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lineX := x
		if align == "center" {
			lineX = x + (width-previewTextWidth(line, fontSize))/2
		}
		ctx := freetype.NewContext()
		ctx.SetDPI(72)
		ctx.SetFont(font)
		ctx.SetFontSize(fontSize)
		ctx.SetClip(dst.Bounds())
		ctx.SetDst(dst)
		ctx.SetSrc(image.NewUniform(color.Black))
		_, _ = ctx.DrawString(line, freetype.Pt(int(lineX), int(y+fontSize+float64(index)*lineHeight)))
	}
}

func splitPreviewLines(content string, width, fontSize float64, multiline bool) []string {
	rawLines := strings.Split(content, "\n")
	if !multiline {
		return rawLines
	}
	lines := []string{}
	for _, raw := range rawLines {
		current := ""
		for _, r := range []rune(raw) {
			next := current + string(r)
			if current != "" && previewTextWidth(next, fontSize) > width {
				lines = append(lines, current)
				current = string(r)
			} else {
				current = next
			}
		}
		if current != "" {
			lines = append(lines, current)
		}
	}
	return lines
}

func previewTextWidth(text string, fontSize float64) float64 {
	width := 0.0
	for _, r := range []rune(text) {
		if r < 128 {
			width += fontSize * 0.55
		} else {
			width += fontSize
		}
	}
	return width
}

// 生成各页面的渲染数据
func generatePageRenderData(data ReportData) []utils.H {
	var pages []utils.H

	// 第1页
	page1 := utils.H{
		"pageNumber": 1,
		"elements": []utils.H{
			{
				"type":    "text",
				"key":     "NameP1",
				"content": data.PatientName,
				"x":       50.0,
				"y":       138.0,
			},
			{
				"type":    "text",
				"key":     "SexP1",
				"content": data.Gender,
				"x":       90.0,
				"y":       138.0,
			},
			{
				"type":    "text",
				"key":     "AgeP1",
				"content": fmt.Sprintf("%d", data.Age),
				"x":       120.0,
				"y":       138.0,
			},
		},
	}
	pages = append(pages, page1)

	// 第3页
	page3Elements := []utils.H{
		{
			"type":    "text",
			"key":     "NameP2",
			"content": data.PatientName,
			"x":       30.0,
			"y":       72.5,
		},
		{
			"type":    "text",
			"key":     "SexP2",
			"content": data.Gender,
			"x":       62.0,
			"y":       72.5,
		},
		{
			"type":    "text",
			"key":     "AgeP2",
			"content": fmt.Sprintf("%d", data.Age),
			"x":       92.0,
			"y":       72.5,
		},
		{
			"type":    "text",
			"key":     "SampleType",
			"content": data.SampleType,
			"x":       149.0,
			"y":       71.5,
		},
		{
			"type":    "text",
			"key":     "SampleTime",
			"content": data.SampleTime,
			"x":       149.0,
			"y":       81.0,
		},
		{
			"type":    "text",
			"key":     "Project",
			"content": data.CancerTypeName,
			"x":       30.0,
			"y":       87.3,
		},
		{
			"type":    "text",
			"key":     "NumberID",
			"content": data.SampleCode,
			"x":       92.0,
			"y":       87.3,
		},
		{
			"type":    "text",
			"key":     "Organization",
			"content": data.Organization,
			"x":       149.0,
			"y":       90.0,
		},
		{
			"type":    "text",
			"key":     "Inspector",
			"content": data.Inspector,
			"x":       32.0,
			"y":       251.2,
		},
		{
			"type":    "text",
			"key":     "Reviewer",
			"content": data.Reviewer,
			"x":       105.0,
			"y":       251.2,
		},
		{
			"type":    "text",
			"key":     "ReportTime",
			"content": data.CreatedAt.Format("2006-01-02"),
			"x":       175.5,
			"y":       251.2,
		},
	}

	// 添加报告表格数据
	yOffset := 107.3
	rowHeight := 7.25

	// 辅助函数：获取报告数据字符串
	getReportDataString := func(key string) string {
		if data.ReportDataMap != nil {
			if value, ok := data.ReportDataMap[key]; ok {
				switch v := value.(type) {
				case string:
					if v == "" && key == "time1" {
						return data.CreatedAt.Format("2006-01-02")
					}
					if v == "" && key == "trend1" {
						return "-"
					}
					if strings.Contains(key, "time") {
						if t, err := time.Parse(time.RFC3339, v); err == nil {
							return t.Format("2006-01-02")
						}
						if t, err := time.Parse("2006-01-02T15:04:05Z07:00", v); err == nil {
							return t.Format("2006-01-02")
						}
					}
					return v
				case int:
					return fmt.Sprintf("%d", v)
				case float64:
					return fmt.Sprintf("%f", v)
				case bool:
					if v {
						return "是"
					} else {
						return "否"
					}
				default:
					return fmt.Sprintf("%v", v)
				}
			}
			if key == "time1" {
				if value, ok := data.ReportDataMap["detect_reportTime"]; ok {
					switch v := value.(type) {
					case string:
						if t, err := time.Parse(time.RFC3339, v); err == nil {
							return t.Format("2006-01-02")
						}
						if t, err := time.Parse("2006-01-02T15:04:05Z07:00", v); err == nil {
							return t.Format("2006-01-02")
						}
						return v
					}
				}
				return data.CreatedAt.Format("2006-01-02")
			} else if key == "trend1" {
				if value, ok := data.ReportDataMap["trend"]; ok {
					switch v := value.(type) {
					case string:
						return v
					default:
						return fmt.Sprintf("%v", v)
					}
				}
				return "-"
			} else if key == "type1" {
				if value, ok := data.ReportDataMap["treatmentStageName"]; ok {
					switch v := value.(type) {
					case string:
						return v
					default:
						return fmt.Sprintf("%v", v)
					}
				}
				return ""
			} else if key == "time2" || key == "signal2" || key == "trend2" || key == "type2" || key == "note2" ||
				key == "time3" || key == "signal3" || key == "trend3" || key == "type3" || key == "note3" ||
				key == "time4" || key == "signal4" || key == "trend4" || key == "type4" || key == "note4" {
				if selectedHistoricalReports, ok := data.ReportDataMap["selectedHistoricalReports"].([]interface{}); ok {
					index := -1
					switch key {
					case "time2", "signal2", "trend2", "type2", "note2":
						index = 0
					case "time3", "signal3", "trend3", "type3", "note3":
						index = 1
					case "time4", "signal4", "trend4", "type4", "note4":
						index = 2
					}
					if index >= 0 && index < len(selectedHistoricalReports) {
						if detect_report, ok := selectedHistoricalReports[index].(map[string]interface{}); ok {
							fieldName := ""
							switch key {
							case "time2", "time3", "time4":
								fieldName = "time"
							case "signal2", "signal3", "signal4":
								fieldName = "signal"
							case "trend2", "trend3", "trend4":
								fieldName = "trend"
							case "type2", "type3", "type4":
								fieldName = "type"
							case "note2", "note3", "note4":
								fieldName = "note"
							}
							if value, ok := detect_report[fieldName]; ok {
								switch v := value.(type) {
								case string:
									return v
								case float64:
									return fmt.Sprintf("%.1f", v)
								case int:
									return fmt.Sprintf("%d", v)
								default:
									return fmt.Sprintf("%v", v)
								}
							}
						}
					}
				}
				if strings.Contains(key, "signal") {
					return "0"
				}
				return ""
			}
		}
		return ""
	}

	// 辅助函数：获取报告数据浮点数
	getReportDataFloat := func(key string) float64 {
		if data.ReportDataMap != nil {
			if value, ok := data.ReportDataMap[key]; ok {
				switch v := value.(type) {
				case float64:
					return v
				case int:
					return float64(v)
				case string:
					if f, err := strconv.ParseFloat(v, 64); err == nil {
						return f
					}
				}
			}
			if key == "signal1" {
				if value, ok := data.ReportDataMap["calculationResult"]; ok {
					switch v := value.(type) {
					case float64:
						return v
					case int:
						return float64(v)
					case string:
						if f, err := strconv.ParseFloat(v, 64); err == nil {
							return f
						}
					}
				}
			} else if key == "signal2" || key == "signal3" || key == "signal4" {
				if selectedHistoricalReports, ok := data.ReportDataMap["selectedHistoricalReports"].([]interface{}); ok {
					index := -1
					switch key {
					case "signal2":
						index = 0
					case "signal3":
						index = 1
					case "signal4":
						index = 2
					}
					if index >= 0 && index < len(selectedHistoricalReports) {
						if detect_report, ok := selectedHistoricalReports[index].(map[string]interface{}); ok {
							if value, ok := detect_report["signal"]; ok {
								switch v := value.(type) {
								case float64:
									return v
								case int:
									return float64(v)
								case string:
									if f, err := strconv.ParseFloat(v, 64); err == nil {
										return f
									}
								}
							}
						}
					}
				}
			}
		}
		return 0
	}

	// 检查是否有数据的函数
	hasData := func(timeKey string, signalKey string) bool {
		if timeKey == "time1" && signalKey == "signal1" {
			return true
		}
		timeValue := getReportDataString(timeKey)
		signalValue := getReportDataFloat(signalKey)
		return timeValue != "" || signalValue > 0
	}

	// 报告1
	if hasData("time1", "signal1") {
		page3Elements = append(page3Elements,
			utils.H{
				"type":    "text",
				"key":     "Time1",
				"content": getReportDataString("time1"),
				"x":       61.5,
				"y":       yOffset + 3.6,
			},
			utils.H{
				"type":    "text",
				"key":     "Signal1",
				"content": fmt.Sprintf("%.1f", getReportDataFloat("signal1")),
				"x":       88.0,
				"y":       yOffset + 3.6,
			},
			utils.H{
				"type":    "text",
				"key":     "Trend1",
				"content": getReportDataString("trend1"),
				"x":       100.8,
				"y":       yOffset + 3.6,
			},
			utils.H{
				"type":    "text",
				"key":     "Type1",
				"content": getReportDataString("type1"),
				"x":       129.2,
				"y":       yOffset + 3.6,
			},
			utils.H{
				"type":    "text",
				"key":     "Note1",
				"content": getReportDataString("note1"),
				"x":       126.6,
				"y":       yOffset + 3.6,
			},
		)
		yOffset += rowHeight
	}

	// 报告2
	if hasData("time2", "signal2") {
		page3Elements = append(page3Elements,
			utils.H{
				"type":    "text",
				"key":     "Time2",
				"content": getReportDataString("time2"),
				"x":       61.5,
				"y":       yOffset + 3.6,
			},
			utils.H{
				"type":    "text",
				"key":     "Signal2",
				"content": fmt.Sprintf("%.1f", getReportDataFloat("signal2")),
				"x":       88.0,
				"y":       yOffset + 3.6,
			},
			utils.H{
				"type":    "text",
				"key":     "Trend2",
				"content": getReportDataString("trend2"),
				"x":       100.8,
				"y":       yOffset + 3.6,
			},
			utils.H{
				"type":    "text",
				"key":     "Type2",
				"content": getReportDataString("type2"),
				"x":       129.2,
				"y":       yOffset + 3.6,
			},
			utils.H{
				"type":    "text",
				"key":     "Note2",
				"content": getReportDataString("note2"),
				"x":       126.6,
				"y":       yOffset + 3.6,
			},
		)
		yOffset += rowHeight
	}

	// 报告3
	if hasData("time3", "signal3") {
		page3Elements = append(page3Elements,
			utils.H{
				"type":    "text",
				"key":     "Time3",
				"content": getReportDataString("time3"),
				"x":       61.5,
				"y":       yOffset + 3.6,
			},
			utils.H{
				"type":    "text",
				"key":     "Signal3",
				"content": fmt.Sprintf("%.1f", getReportDataFloat("signal3")),
				"x":       88.0,
				"y":       yOffset + 3.6,
			},
			utils.H{
				"type":    "text",
				"key":     "Trend3",
				"content": getReportDataString("trend3"),
				"x":       100.8,
				"y":       yOffset + 3.6,
			},
			utils.H{
				"type":    "text",
				"key":     "Type3",
				"content": getReportDataString("type3"),
				"x":       129.2,
				"y":       yOffset + 3.6,
			},
			utils.H{
				"type":    "text",
				"key":     "Note3",
				"content": getReportDataString("note3"),
				"x":       126.6,
				"y":       yOffset + 3.6,
			},
		)
		yOffset += rowHeight
	}

	// 报告4
	if hasData("time4", "signal4") {
		page3Elements = append(page3Elements,
			utils.H{
				"type":    "text",
				"key":     "Time4",
				"content": getReportDataString("time4"),
				"x":       61.5,
				"y":       yOffset + 3.6,
			},
			utils.H{
				"type":    "text",
				"key":     "Signal4",
				"content": fmt.Sprintf("%.1f", getReportDataFloat("signal4")),
				"x":       88.0,
				"y":       yOffset + 3.6,
			},
			utils.H{
				"type":    "text",
				"key":     "Trend4",
				"content": getReportDataString("trend4"),
				"x":       100.8,
				"y":       yOffset + 3.6,
			},
			utils.H{
				"type":    "text",
				"key":     "Type4",
				"content": getReportDataString("type4"),
				"x":       129.2,
				"y":       yOffset + 3.6,
			},
			utils.H{
				"type":    "text",
				"key":     "Note4",
				"content": getReportDataString("note4"),
				"x":       126.6,
				"y":       yOffset + 3.6,
			},
		)
		yOffset += rowHeight
	}

	// 添加信号值说明和结果说明
	page3Elements = append(page3Elements,
		utils.H{
			"type":    "multilineText",
			"key":     "SignalInstructions",
			"content": data.SignalValueExplanation,
			"x":       42.5,
			"y":       136.5,
		},
		utils.H{
			"type":    "multilineText",
			"key":     "ResultInstructions",
			"content": data.ResultExplanation,
			"x":       42.5,
			"y":       154.0,
		},
	)

	page3 := utils.H{
		"pageNumber": 3,
		"elements":   page3Elements,
	}
	pages = append(pages, page3)

	return pages
}
