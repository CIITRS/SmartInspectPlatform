package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"unicode"
)

const excelAlignedScreeningFormula = "(sqrt(pow(sum(HWG12,HWG13,HWG10,HWG11,HWG07,HWG03,HWG14,HWG15,HWG16,HWG01_F8,HWG17)/HWG01_F8,2)+pow(count_ge_threshold(HWG12,HWG13,HWG10,HWG11,HWG07,HWG03,HWG14,HWG15,HWG16,HWG01_F8,HWG17)-1,2))+sqrt(pow(sum(HWG02,HWG04,HWG08,HWG05,HWG06,HWG01_V5,HWG09)/HWG01_V5,2)+pow(count_ge_threshold(HWG02,HWG04,HWG08,HWG05,HWG06,HWG01_V5,HWG09)-1,2))*0.65)/sqrt(2*pow(14,2))*100"

func configuredDuplicateAverageGenes(db *sql.DB, modelID int) map[string]bool {
	result := make(map[string]bool)
	if db == nil || modelID <= 0 {
		return result
	}
	var parameters sql.NullString
	if err := db.QueryRow(`SELECT parameters FROM setting_model WHERE id = ?`, modelID).Scan(&parameters); err != nil ||
		!parameters.Valid || strings.TrimSpace(parameters.String) == "" {
		return result
	}
	var config struct {
		DuplicateAverageGenes []string `json:"duplicateAverageGenes"`
	}
	if err := json.Unmarshal([]byte(parameters.String), &config); err != nil {
		return result
	}
	for _, configuredGene := range config.DuplicateAverageGenes {
		configuredGene = strings.TrimSpace(configuredGene)
		if configuredGene == "" {
			continue
		}
		result[configuredGene] = true
		if symbol, err := getGeneSymbolByAnyName(db, configuredGene); err == nil && strings.TrimSpace(symbol) != "" {
			result[symbol] = true
		}
	}
	return result
}

func formulaNumericValue(value interface{}) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func formulaPlatformSuffix(platform string) string {
	var builder strings.Builder
	for _, char := range strings.ToUpper(strings.TrimSpace(platform)) {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			builder.WriteRune(char)
		} else {
			builder.WriteRune('_')
		}
	}
	return strings.Trim(builder.String(), "_")
}

// enrichExcelDuplicateGeneVariables keeps the two Panel-specific internal-reference values
// separate. Each Panel's sum and threshold count use and remove its own internal reference.
func enrichExcelDuplicateGeneVariables(db *sql.DB, batchID int, sampleCode string, modelID int, geneData map[string]interface{}) (bool, error) {
	configuredGenes := configuredDuplicateAverageGenes(db, modelID)
	if len(configuredGenes) == 0 || batchID <= 0 || strings.TrimSpace(sampleCode) == "" {
		return false, nil
	}
	rows, err := db.Query(`SELECT platform, median_data
		FROM detect_batch_platform_data
		WHERE batch_id = ? AND sample_code = ?
		ORDER BY platform, id`, batchID, sampleCode)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	changed := false
	for rows.Next() {
		var platform string
		var medianJSON sql.NullString
		if err := rows.Scan(&platform, &medianJSON); err != nil {
			return false, err
		}
		if !medianJSON.Valid || strings.TrimSpace(medianJSON.String) == "" {
			continue
		}
		var median map[string]interface{}
		if err := json.Unmarshal([]byte(medianJSON.String), &median); err != nil {
			return false, err
		}
		for rawGene, rawValue := range median {
			if isResultMetaKey(rawGene) {
				continue
			}
			geneSymbol, geneErr := getGeneSymbolByAnyName(db, rawGene)
			if geneErr != nil || !configuredGenes[geneSymbol] {
				continue
			}
			value, ok := formulaNumericValue(rawValue)
			if !ok {
				continue
			}
			suffix := formulaPlatformSuffix(platform)
			if suffix != "" {
				geneData[geneSymbol+"_"+suffix] = value
				changed = true
			}
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return changed, nil
}

func enrichExcelDuplicateGeneVariablesForSample(db *sql.DB, sampleID, modelID int, geneData map[string]interface{}) (bool, error) {
	if db == nil || sampleID <= 0 || modelID <= 0 {
		return false, nil
	}
	var batchID int
	var sampleCode string
	if err := db.QueryRow(`SELECT COALESCE(batch_id, 0), sample_code
		FROM detect_sample WHERE id = ?`, sampleID).Scan(&batchID, &sampleCode); err != nil {
		return false, err
	}
	return enrichExcelDuplicateGeneVariables(db, batchID, sampleCode, modelID, geneData)
}

// EnsureExcelAlignedScreeningModels updates stored formulas and backfills platform-specific
// duplicate variables for existing health-screening and old-CRC sample results.
func EnsureExcelAlignedScreeningModels(db *sql.DB) error {
	if db == nil {
		return nil
	}
	rows, err := db.Query(`SELECT id, COALESCE(parameters, '{}')
		FROM setting_model WHERE model_name IN ('健康筛查', '肠癌-旧版')`)
	if err != nil {
		return err
	}
	type modelConfig struct {
		ID         int
		Parameters string
	}
	models := make([]modelConfig, 0, 2)
	for rows.Next() {
		var item modelConfig
		if err := rows.Scan(&item.ID, &item.Parameters); err != nil {
			rows.Close()
			return err
		}
		models = append(models, item)
	}
	if err := rows.Close(); err != nil {
		return err
	}

	for _, model := range models {
		parameters := map[string]interface{}{}
		_ = json.Unmarshal([]byte(model.Parameters), &parameters)
		parameters["formulaVersion"] = "panel-own-internal-reference-v2"
		parameters["duplicatePlatformVariables"] = map[string]interface{}{
			"HWG01": map[string]string{"F8": "HWG01_F8", "V5": "HWG01_V5"},
		}
		parametersJSON, _ := json.Marshal(parameters)
		if _, err := db.Exec(`UPDATE setting_model
			SET formula = ?, parameters = ?, updated_at = NOW()
			WHERE id = ?`, excelAlignedScreeningFormula, string(parametersJSON), model.ID); err != nil {
			return fmt.Errorf("update model %d Excel formula: %w", model.ID, err)
		}
	}

	sampleRows, err := db.Query(`SELECT s.id, s.sample_code, COALESCE(s.batch_id, 0),
			COALESCE(s.model_id, 0), COALESCE(s.result_data, '')
		FROM detect_sample s
		JOIN setting_model m ON m.id = s.model_id
		WHERE m.model_name IN ('健康筛查', '肠癌-旧版')
			AND s.batch_id IS NOT NULL AND s.result_data IS NOT NULL
			AND TRIM(s.result_data) NOT IN ('', '{}', 'null')`)
	if err != nil {
		return err
	}
	defer sampleRows.Close()
	for sampleRows.Next() {
		var sampleID, batchID, modelID int
		var sampleCode, resultDataJSON string
		if err := sampleRows.Scan(&sampleID, &sampleCode, &batchID, &modelID, &resultDataJSON); err != nil {
			return err
		}
		resultData := map[string]interface{}{}
		if err := json.Unmarshal([]byte(resultDataJSON), &resultData); err != nil {
			continue
		}
		geneData, ok := resultData["gene_data"].(map[string]interface{})
		if !ok {
			continue
		}
		changed, enrichErr := enrichExcelDuplicateGeneVariables(db, batchID, sampleCode, modelID, geneData)
		if enrichErr != nil {
			log.Printf("Backfill Excel duplicate variables for sample %s failed: %v", sampleCode, enrichErr)
			continue
		}
		if !changed {
			continue
		}
		resultData["gene_data"] = geneData
		delete(resultData, "score")
		delete(resultData, "signalValue")
		delete(resultData, "calculationResult")
		updatedJSON, _ := json.Marshal(resultData)
		if _, err := db.Exec(`UPDATE detect_sample SET result_data = ?, signalvalue = NULL,
			result_updated_at = NOW(), updated_at = NOW() WHERE id = ?`, string(updatedJSON), sampleID); err != nil {
			return err
		}
	}
	return sampleRows.Err()
}
