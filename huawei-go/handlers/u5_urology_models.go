package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

var u5UrologyGeneIDs = []int{18, 8, 19, 20, 1, 21, 22}

const (
	u5UrologyBloodModelName = "泌尿-U5-血液"
	u5UrologyUrineModelName = "泌尿-U5-尿液"

	u5UrologyBloodFormula = "sqrt(pow(sum(B14_PTPRU,B37_HIST1H4F,B51_RUNX3,B55_REC8,B72_THBS1,B76_OTX1,B78_TW1ST1)/B72_THBS1,2)+pow(count_ge_threshold(B14_PTPRU,B37_HIST1H4F,B51_RUNX3,B55_REC8,B72_THBS1,B76_OTX1,B78_TW1ST1)-1,2))/sqrt(2*pow(6,2))*100*0.6"
	u5UrologyUrineFormula = "sqrt(pow(sum(B14_PTPRU,B37_HIST1H4F,B51_RUNX3,B55_REC8,B72_THBS1,B76_OTX1,B78_TW1ST1)/B72_THBS1,2)+pow(count_ge_threshold(B14_PTPRU,B37_HIST1H4F,B51_RUNX3,B55_REC8,B72_THBS1,B76_OTX1,B78_TW1ST1)-2,2))/sqrt(2*pow(5,2))*100*0.6"
)

type u5UrologyModelDefinition struct {
	Name          string
	PreviousName  string
	Description   string
	Formula       string
	Threshold     float64
	SourceFormula string
}

// EnsureU5UrologyModels installs the two U5 urology formulas and their
// model-specific thresholds. It is intentionally idempotent so an upgrade can
// safely be retried after a partial deployment.
func EnsureU5UrologyModels(db *sql.DB) error {
	if db == nil {
		return nil
	}
	if err := ensureModelGeneThresholdTable(db); err != nil {
		return err
	}

	definitions := []u5UrologyModelDefinition{
		{
			Name:          u5UrologyBloodModelName,
			PreviousName:  "泌尿-血液",
			Description:   "泌尿肿瘤 U5 版血液检测公式",
			Formula:       u5UrologyBloodFormula,
			Threshold:     100,
			SourceFormula: `=SQRT((SUM(B3:H3)/F3)^2+(COUNTIF(B3:H3,">=100")-1)^2)/SQRT(2*6^2)*100*0.6`,
		},
		{
			Name:          u5UrologyUrineModelName,
			Description:   "泌尿肿瘤 U5 版尿液检测公式",
			Formula:       u5UrologyUrineFormula,
			Threshold:     300,
			SourceFormula: `=SQRT((SUM(B4:H4)/F4)^2+(COUNTIF(B4:H4,">=300")-2)^2)/SQRT(2*5^2)*100*0.6`,
		},
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, definition := range definitions {
		modelID, err := upsertU5UrologyModel(tx, definition)
		if err != nil {
			return err
		}
		for _, geneID := range u5UrologyGeneIDs {
			if _, err := tx.Exec(`INSERT INTO setting_model_gene_threshold
				(model_id, gene_id, threshold, created_at, updated_at)
				VALUES (?, ?, ?, NOW(), NOW())
				ON DUPLICATE KEY UPDATE threshold = VALUES(threshold), updated_at = NOW()`,
				modelID, geneID, definition.Threshold); err != nil {
				return fmt.Errorf("save %s threshold for gene %d: %w", definition.Name, geneID, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	ClearCache("models:*")
	return nil
}

func upsertU5UrologyModel(tx *sql.Tx, definition u5UrologyModelDefinition) (int64, error) {
	parametersJSON, err := json.Marshal(map[string]interface{}{
		"selectedGenes":  u5UrologyGeneIDs,
		"formulaVersion": "u5-v1",
		"sourceFormula":  definition.SourceFormula,
	})
	if err != nil {
		return 0, err
	}

	var modelID int64
	err = tx.QueryRow(`SELECT id FROM setting_model
		WHERE model_name IN (?, ?) ORDER BY model_name = ? DESC, id LIMIT 1`,
		definition.Name, definition.PreviousName, definition.Name).Scan(&modelID)
	switch err {
	case nil:
		_, err = tx.Exec(`UPDATE setting_model SET model_name = ?, description = ?,
			model_version = 'weighted_equation', cancer_type_id = 13, version = 'U5',
			parameters = ?, formula = ?, model_mode = 'weighted', is_active = 1,
			is_deprecated = 0, deprecated_at = NULL, updated_at = NOW()
			WHERE id = ?`,
			definition.Name, definition.Description, string(parametersJSON),
			definition.Formula, modelID)
		if err != nil {
			return 0, fmt.Errorf("update model %s: %w", definition.Name, err)
		}
		return modelID, nil
	case sql.ErrNoRows:
		result, insertErr := tx.Exec(`INSERT INTO setting_model
			(model_name, description, model_version, cancer_type_id, version, parameters,
			 formula, model_mode, is_active, is_deprecated, deprecated_at, created_at, updated_at)
			VALUES (?, ?, 'weighted_equation', 13, 'U5', ?, ?, 'weighted', 1, 0, NULL, NOW(), NOW())`,
			definition.Name, definition.Description, string(parametersJSON), definition.Formula)
		if insertErr != nil {
			return 0, fmt.Errorf("create model %s: %w", definition.Name, insertErr)
		}
		modelID, insertErr = result.LastInsertId()
		return modelID, insertErr
	default:
		return 0, fmt.Errorf("find model %s: %w", definition.Name, err)
	}
}
