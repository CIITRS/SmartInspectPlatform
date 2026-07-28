package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func ensureModelGeneThresholdTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS setting_model_gene_threshold (
		id INT AUTO_INCREMENT PRIMARY KEY,
		model_id INT NOT NULL,
		gene_id INT NOT NULL,
		threshold DECIMAL(12,4) NOT NULL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		UNIQUE KEY unique_model_gene_threshold (model_id, gene_id),
		KEY idx_model_gene_threshold_model (model_id),
		KEY idx_model_gene_threshold_gene (gene_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`)
	return err
}

func selectedModelGeneIDs(db *sql.DB, modelID int) []int {
	var parameters, formula sql.NullString
	if err := db.QueryRow(`SELECT parameters, formula FROM setting_model WHERE id = ?`, modelID).Scan(&parameters, &formula); err != nil {
		return nil
	}

	geneIDs := parseSelectedGeneIDs(parameters.String)
	if len(geneIDs) > 0 {
		return geneIDs
	}

	formulaGenes := extractGenesFromFormula(formula.String)
	if len(formulaGenes) == 0 {
		return nil
	}

	ids := make([]int, 0, len(formulaGenes))
	for _, geneName := range formulaGenes {
		var id int
		err := db.QueryRow(`SELECT id FROM setting_gene WHERE gene_name = ? OR gene_symbol = ? LIMIT 1`, geneName, geneName).Scan(&id)
		if err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

func loadModelGeneThresholdRows(db *sql.DB, modelID int) ([]utils.H, error) {
	if err := ensureModelGeneThresholdTable(db); err != nil {
		return nil, err
	}

	geneIDs := selectedModelGeneIDs(db, modelID)
	args := make([]interface{}, 0, len(geneIDs)+1)
	query := `SELECT g.id, g.gene_name, g.gene_symbol, COALESCE(g.description, ''), COALESCE(mgt.threshold, g.threshold, 0)
		FROM setting_gene g
		LEFT JOIN setting_model_gene_threshold mgt ON mgt.gene_id = g.id AND mgt.model_id = ?`
	args = append(args, modelID)
	if len(geneIDs) > 0 {
		placeholders := make([]string, len(geneIDs))
		for i, id := range geneIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		query += " WHERE g.id IN (" + strings.Join(placeholders, ",") + ")"
	}
	query += " ORDER BY g.gene_symbol ASC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	thresholds := make([]utils.H, 0)
	for rows.Next() {
		var id int
		var geneName, geneSymbol, description string
		var threshold float64
		if err := rows.Scan(&id, &geneName, &geneSymbol, &description, &threshold); err != nil {
			return nil, err
		}
		thresholds = append(thresholds, utils.H{
			"id":          id,
			"geneId":      id,
			"name":        geneName,
			"geneName":    geneName,
			"geneSymbol":  geneSymbol,
			"description": description,
			"threshold":   threshold,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return thresholds, nil
}

func loadModelThresholdMap(db *sql.DB, modelID int) (map[string]float64, error) {
	rows, err := loadModelGeneThresholdRows(db, modelID)
	if err != nil {
		return nil, err
	}

	thresholds := make(map[string]float64)
	for _, row := range rows {
		geneName, _ := row["geneName"].(string)
		geneSymbol, _ := row["geneSymbol"].(string)
		threshold, _ := row["threshold"].(float64)
		if geneName != "" {
			thresholds[geneName] = threshold
		}
		if geneSymbol != "" {
			thresholds[geneSymbol] = threshold
		}
	}
	for geneName := range configuredDuplicateAverageGenes(db, modelID) {
		threshold, exists := thresholds[geneName]
		if !exists {
			continue
		}
		thresholds[geneName+"_F8"] = threshold
		thresholds[geneName+"_V5"] = threshold
	}
	return thresholds, nil
}

func convertGeneDataToFloatMap(geneData map[string]interface{}) map[string]float64 {
	variables := make(map[string]float64)
	for geneName, rawValue := range geneData {
		switch value := rawValue.(type) {
		case float64:
			variables[geneName] = value
		case float32:
			variables[geneName] = float64(value)
		case int:
			variables[geneName] = float64(value)
		case int64:
			variables[geneName] = float64(value)
		case json.Number:
			if parsed, err := value.Float64(); err == nil {
				variables[geneName] = parsed
			}
		case string:
			if parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
				variables[geneName] = parsed
			}
		}
	}
	return variables
}

func firstNonEmpty(values ...interface{}) interface{} {
	for _, value := range values {
		if strings.TrimSpace(fmt.Sprint(value)) != "" && fmt.Sprint(value) != "<nil>" {
			return value
		}
	}
	return ""
}

func calculateModelFormulaScore(db *sql.DB, modelID int, geneData map[string]interface{}) (float64, bool, error) {
	if modelID == 0 || len(geneData) == 0 {
		return 0, false, nil
	}

	var formula sql.NullString
	if err := db.QueryRow(`SELECT formula FROM setting_model WHERE id = ?`, modelID).Scan(&formula); err != nil {
		return 0, false, err
	}
	if !formula.Valid || strings.TrimSpace(formula.String) == "" {
		return 0, false, nil
	}

	thresholds, err := loadModelThresholdMap(db, modelID)
	if err != nil {
		return 0, false, err
	}

	evaluator := NewFormulaEvaluator(formula.String)
	evaluator.SetThresholds(thresholds)
	evaluator.SetVariables(convertGeneDataToFloatMap(expandGeneDataAliasesForFormula(db, geneData)))
	score, err := evaluator.Evaluate()
	if err != nil {
		return 0, false, fmt.Errorf("model %d formula failed: %w", modelID, err)
	}
	return math.Round(score*10) / 10, true, nil
}

func logModelScoreError(sampleID, modelID int, err error) {
	if err != nil {
		log.Printf("Failed to calculate formula score for sample %d with model %d: %v", sampleID, modelID, err)
	}
}

func HandleModelFormulaCalculate(c *app.RequestContext, db *sql.DB) {
	var req struct {
		ModelID  int                      `json:"modelId"`
		GeneData map[string]interface{}   `json:"geneData"`
		Rows     []map[string]interface{} `json:"rows"`
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
	if req.ModelID <= 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "缺少模型ID",
			Data:    nil,
		})
		return
	}

	if len(req.Rows) == 0 && req.GeneData != nil {
		req.Rows = []map[string]interface{}{req.GeneData}
	}

	results := make([]utils.H, 0, len(req.Rows))
	for index, row := range req.Rows {
		score, ok, err := calculateModelFormulaScore(db, req.ModelID, row)
		if err != nil {
			c.JSON(consts.StatusBadRequest, ApiResponse{
				Code:    400,
				Success: false,
				Message: "模型计算失败",
				Data:    utils.H{"error": err.Error(), "rowIndex": index},
			})
			return
		}
		results = append(results, utils.H{
			"index":      index,
			"sampleCode": firstNonEmpty(row["Sample"], row["sample_code"]),
			"score":      score,
			"calculated": ok,
		})
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "模型计算成功",
		Data:    utils.H{"results": results},
	})
}
