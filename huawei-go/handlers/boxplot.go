package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"math"
	"sort"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// BoxplotData 箱线图数据结构
type BoxplotData struct {
	GeneSymbol string    `json:"GeneSymbol"`
	CancerType string    `json:"CancerType"`
	Data       []Boxplot `json:"Data"`
}

// Boxplot 单个箱线图数据结构
type Boxplot struct {
	TreatmentStage string    `json:"TreatmentStage"`
	Min            float64   `json:"Min"`
	Q1             float64   `json:"Q1"`
	Median         float64   `json:"Median"`
	Q3             float64   `json:"Q3"`
	Max            float64   `json:"Max"`
	Outliers       []float64 `json:"Outliers"`
	Count          int       `json:"Count"`
}

// CalculateQuartiles 计算四分位数
func CalculateQuartiles(values []float64) (float64, float64, float64) {
	sort.Float64s(values)
	n := len(values)

	// 处理边界情况
	if n == 0 {
		return 0, 0, 0
	}
	if n == 1 {
		return values[0], values[0], values[0]
	}

	var q1, median, q3 float64

	// 计算中位数
	if n%2 == 0 {
		median = (values[n/2-1] + values[n/2]) / 2
	} else {
		median = values[n/2]
	}

	// 计算Q1
	if n%2 == 0 {
		lowerHalf := values[:n/2]
		if len(lowerHalf)%2 == 0 {
			q1 = (lowerHalf[len(lowerHalf)/2-1] + lowerHalf[len(lowerHalf)/2]) / 2
		} else {
			q1 = lowerHalf[len(lowerHalf)/2]
		}
	} else {
		lowerHalf := values[:n/2]
		if len(lowerHalf) == 0 {
			q1 = values[0]
		} else if len(lowerHalf)%2 == 0 {
			q1 = (lowerHalf[len(lowerHalf)/2-1] + lowerHalf[len(lowerHalf)/2]) / 2
		} else {
			q1 = lowerHalf[len(lowerHalf)/2]
		}
	}

	// 计算Q3
	if n%2 == 0 {
		upperHalf := values[n/2:]
		if len(upperHalf)%2 == 0 {
			q3 = (upperHalf[len(upperHalf)/2-1] + upperHalf[len(upperHalf)/2]) / 2
		} else {
			q3 = upperHalf[len(upperHalf)/2]
		}
	} else {
		upperHalf := values[n/2+1:]
		if len(upperHalf) == 0 {
			q3 = values[n-1]
		} else if len(upperHalf)%2 == 0 {
			q3 = (upperHalf[len(upperHalf)/2-1] + upperHalf[len(upperHalf)/2]) / 2
		} else {
			q3 = upperHalf[len(upperHalf)/2]
		}
	}

	return q1, median, q3
}

// DetectOutliers 检测异常点
func DetectOutliers(values []float64, q1, q3 float64) ([]float64, float64, float64) {
	var outliers []float64
	iqr := q3 - q1
	lowerBound := q1 - 1.5*iqr
	upperBound := q3 + 1.5*iqr

	var min, max float64
	first := true

	for _, value := range values {
		if value < lowerBound || value > upperBound {
			outliers = append(outliers, value)
		} else {
			if first {
				min = value
				max = value
				first = false
			} else {
				if value < min {
					min = value
				}
				if value > max {
					max = value
				}
			}
		}
	}

	// 如果所有值都是异常点，则使用原始数据的最小最大值
	if len(outliers) == len(values) && len(values) > 0 {
		sort.Float64s(values)
		min = values[0]
		max = values[len(values)-1]
	}

	return outliers, min, max
}

// HandleGetBoxplotData 处理获取箱线图数据请求
func HandleGetBoxplotData(c *app.RequestContext, db *sql.DB) {
	// 解析查询参数
	geneSymbol := c.Query("geneSymbol")
	cancerTypeID := c.Query("cancerTypeId")

	if geneSymbol == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "基因符号不能为空",
			Data:    nil,
		})
		return
	}

	// 构建查询语句
	query := `
		SELECT 
			s.treatment_stage_id,
			r.result_data,
			ts.name as treatmentStageName,
			ct.name as cancerTypeName
		FROM 
			result r
		LEFT JOIN 
			detect_sample s ON r.detect_sample_id = s.id
		LEFT JOIN 
			treatment_stage ts ON s.treatment_stage_id = ts.id
		LEFT JOIN 
			cancer_type ct ON r.cancer_type_id = ct.id
		WHERE 
			1=1
	`

	var args []interface{}

	// 添加基因符号过滤
	query += " AND r.result_data LIKE ?"
	args = append(args, "%"+geneSymbol+"%")

	// 添加癌种过滤
	if cancerTypeID != "" {
		if ctID, err := strconv.Atoi(cancerTypeID); err == nil {
			query += " AND r.cancer_type_id = ?"
			args = append(args, ctID)
		}
	}

	// 执行查询
	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("Failed to query boxplot data: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}
	defer rows.Close()

	// 按治疗阶段分组存储数据
	dataByStage := make(map[string][]float64)
	var cancerTypeName string

	for rows.Next() {
		var treatmentStageID sql.NullInt32
		var resultData, treatmentStageName, ctName sql.NullString

		err := rows.Scan(&treatmentStageID, &resultData, &treatmentStageName, &ctName)
		if err != nil {
			log.Printf("Failed to scan boxplot data: %v", err)
			continue
		}

		// 获取癌种名称
		if ctName.Valid {
			cancerTypeName = ctName.String
		}

		// 解析结果数据，提取基因值
		if resultData.Valid && treatmentStageName.Valid {
			// 这里简化处理，实际项目中应该解析JSON格式的resultData
			// 假设resultData中包含基因值
			// 这里模拟从resultData中提取基因值
			// 实际项目中应该使用JSON解析
			value := extractGeneValue(resultData.String, geneSymbol)
			if !math.IsNaN(value) {
				stageName := treatmentStageName.String
				if stageName == "" {
					stageName = "未知"
				}
				dataByStage[stageName] = append(dataByStage[stageName], value)
			}
		}
	}

	// 检查遍历过程中是否有错误
	if err = rows.Err(); err != nil {
		log.Printf("Error iterating boxplot data: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	// 检查是否有数据
	if len(dataByStage) == 0 {
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "无历史信息",
			Data: BoxplotData{
				GeneSymbol: geneSymbol,
				CancerType: cancerTypeName,
				Data:       []Boxplot{},
			},
		})
		return
	}

	// 计算每个治疗阶段的箱线图数据
	var boxplots []Boxplot
	for stage, values := range dataByStage {
		if len(values) > 0 {
			// 计算四分位数
			q1, median, q3 := CalculateQuartiles(values)
			// 检测异常点
			outliers, min, max := DetectOutliers(values, q1, q3)

			// 添加箱线图数据
			boxplots = append(boxplots, Boxplot{
				TreatmentStage: stage,
				Min:            min,
				Q1:             q1,
				Median:         median,
				Q3:             q3,
				Max:            max,
				Outliers:       outliers,
				Count:          len(values),
			})
		}
	}

	// 返回箱线图数据
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取箱线图数据成功",
		Data: BoxplotData{
			GeneSymbol: geneSymbol,
			CancerType: cancerTypeName,
			Data:       boxplots,
		},
	})
}

// extractGeneValue 从结果数据中提取基因值
func extractGeneValue(resultData, geneSymbol string) float64 {
	// 解析JSON格式的resultData
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(resultData), &data); err == nil {
		if geneData, ok := data["gene_data"].(map[string]interface{}); ok {
			if value, ok := geneData[geneSymbol].(float64); ok {
				return value
			} else if valueStr, ok := geneData[geneSymbol].(string); ok {
				if floatVal, err := strconv.ParseFloat(valueStr, 64); err == nil {
					return floatVal
				}
			}
		}
	}
	return math.NaN()
}
