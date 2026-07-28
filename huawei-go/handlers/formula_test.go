package handlers

import (
	"database/sql"
	"fmt"
	"math"
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

func TestV8FormulaCountGeThresholdUsesRawSum(t *testing.T) {
	formula := "sqrt(pow(sum(B14_ACV,B21_APC,B33_ADHFE1,B39_C9o,B51_SEP,B72_THBS1,B76_LOC105371031)/B72_THBS1,2)+pow(count_ge_threshold(B14_ACV,B21_APC,B33_ADHFE1,B39_C9o,B51_SEP,B72_THBS1,B76_LOC105371031),2))/sqrt(2*pow(6,2))*100*0.68"
	thresholds := map[string]float64{
		"B14_ACV":          50,
		"B21_APC":          50,
		"B33_ADHFE1":       50,
		"B39_C9o":          50,
		"B51_SEP":          50,
		"B72_THBS1":        50,
		"B76_LOC105371031": 50,
	}

	cases := []struct {
		name      string
		variables map[string]float64
		want      float64
	}{
		{
			name: "row 1",
			variables: map[string]float64{
				"B14_ACV": 860, "B21_APC": 134, "B33_ADHFE1": 13, "B39_C9o": 338.5,
				"B51_SEP": 54, "B72_THBS1": 2937, "B76_LOC105371031": 31,
			},
			want: 41.803989203797784,
		},
		{
			name: "row 2",
			variables: map[string]float64{
				"B14_ACV": 1134.5, "B21_APC": 504, "B33_ADHFE1": 10, "B39_C9o": 9,
				"B51_SEP": 11, "B72_THBS1": 2896, "B76_LOC105371031": 46,
			},
			want: 27.217158405444984,
		},
		{
			name: "row 3",
			variables: map[string]float64{
				"B14_ACV": 17, "B21_APC": 25, "B33_ADHFE1": 9, "B39_C9o": 9,
				"B51_SEP": 11, "B72_THBS1": 1328, "B76_LOC105371031": 13,
			},
			want: 11.697260017674996,
		},
		{
			name: "row 4",
			variables: map[string]float64{
				"B14_ACV": 732.5, "B21_APC": 117, "B33_ADHFE1": 337, "B39_C9o": 8,
				"B51_SEP": 192, "B72_THBS1": 3032, "B76_LOC105371031": 235,
			},
			want: 49.63145868728536,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			evaluator := NewFormulaEvaluator(formula)
			evaluator.SetThresholds(thresholds)
			evaluator.SetVariables(tc.variables)

			score, err := evaluator.Evaluate()
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}

			if math.Abs(score-tc.want) > 1e-9 {
				t.Fatalf("Evaluate() = %v, want %v", score, tc.want)
			}
		})
	}
}

func TestNestedPowFormulaWithWhitespace(t *testing.T) {
	formula := `sqrt (
		pow (sum (B14_PTPRU, B37_HIST1H4F, B51_ RUNX3, B55_ REC8, B72_ THBS1, B76_OTX1, B78_TW1ST1) / B72_THBS1, 2)
		+ pow (count_ge_threshold (B14_PTPRU, B37_HIST1H4F, B51_RUNX3, B55_REC8, B76_OTX1, B78_TW1ST1), 2)
	) / sqrt (2 * pow (6, 2)) * 100 * 0.6`
	evaluator := NewFormulaEvaluator(formula)
	evaluator.SetVariables(map[string]float64{
		"B14_PTPRU": 10, "B37_HIST1H4F": 10, "B51_RUNX3": 10, "B55_REC8": 10,
		"B72_THBS1": 20, "B76_OTX1": 10, "B78_TW1ST1": 10,
	})
	evaluator.SetThresholds(map[string]float64{
		"B14_PTPRU": 5, "B37_HIST1H4F": 5, "B51_RUNX3": 5,
		"B55_REC8": 5, "B76_OTX1": 5, "B78_TW1ST1": 5,
	})
	if _, err := evaluator.Evaluate(); err != nil {
		t.Fatalf("nested pow formula should evaluate after whitespace normalization: %v", err)
	}
}

func TestHWGTwoSegmentFormulaUsesModelThresholds(t *testing.T) {
	formula := "(sqrt(pow(sum(HWG12,HWG13,HWG10,HWG11,HWG07,HWG03,HWG14,HWG15,HWG16,HWG01,HWG17)/HWG15,2)+pow(count_ge_threshold(HWG12,HWG13,HWG10,HWG11,HWG07,HWG03,HWG14,HWG15,HWG16,HWG01,HWG17)-1,2))+sqrt(pow(sum(HWG02,HWG04,HWG08,HWG05,HWG06,HWG01,HWG09)/HWG01,2)+pow(count_ge_threshold(HWG02,HWG04,HWG08,HWG05,HWG06,HWG01,HWG09)-1,2))*0.65)/sqrt(2*pow(14,2))*100"
	variables := map[string]float64{
		"HWG01": (3285 + 3338.5) / 2, "HWG02": 1492, "HWG03": 742.5,
		"HWG04": 19, "HWG05": 18, "HWG06": 15, "HWG07": 515, "HWG08": 31,
		"HWG09": 19, "HWG10": 82, "HWG11": 14, "HWG12": 35, "HWG13": 28,
		"HWG14": 667, "HWG15": 2339, "HWG16": 12, "HWG17": 25.5,
	}
	thresholds := map[string]float64{
		"HWG01": 80, "HWG02": 80, "HWG03": 100, "HWG04": 80, "HWG05": 80,
		"HWG06": 80, "HWG07": 100, "HWG08": 80, "HWG09": 80, "HWG10": 100,
		"HWG11": 100, "HWG12": 100, "HWG13": 100, "HWG14": 100, "HWG15": 100,
		"HWG16": 100, "HWG17": 80,
	}

	evaluator := NewFormulaEvaluator(formula)
	evaluator.SetVariables(variables)
	evaluator.SetThresholds(thresholds)
	score, err := evaluator.Evaluate()
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if math.Abs(score-32.1316338174064) > 1e-9 {
		t.Fatalf("Evaluate() = %v, want %v", score, 32.1316338174064)
	}

	variables["HWG17"] = 60
	healthEvaluator := NewFormulaEvaluator(formula)
	healthEvaluator.SetVariables(variables)
	healthEvaluator.SetThresholds(thresholds)
	healthScore, err := healthEvaluator.Evaluate()
	if err != nil {
		t.Fatalf("health Evaluate() error = %v", err)
	}

	oldCRCThresholds := make(map[string]float64, len(thresholds))
	for gene, threshold := range thresholds {
		oldCRCThresholds[gene] = threshold
	}
	for _, gene := range []string{"HWG01", "HWG02", "HWG04", "HWG05", "HWG06", "HWG08", "HWG09", "HWG17"} {
		oldCRCThresholds[gene] = 50
	}
	oldCRCEvaluator := NewFormulaEvaluator(formula)
	oldCRCEvaluator.SetVariables(variables)
	oldCRCEvaluator.SetThresholds(oldCRCThresholds)
	oldCRCScore, err := oldCRCEvaluator.Evaluate()
	if err != nil {
		t.Fatalf("old CRC Evaluate() error = %v", err)
	}
	if oldCRCScore <= healthScore {
		t.Fatalf("old CRC score = %v, health score = %v; model-specific thresholds were not applied", oldCRCScore, healthScore)
	}
}

func TestExcelAlignedScreeningFormula(t *testing.T) {
	cases := []struct {
		name       string
		variables  map[string]float64
		thresholds map[string]float64
		want       float64
	}{
		{
			name: "old CRC Hou Xingde",
			variables: map[string]float64{
				"HWG01": (3392 + 2375.5) / 2, "HWG01_F8": 3392, "HWG01_V5": 2375.5,
				"HWG02": 135, "HWG03": 285, "HWG04": 105, "HWG05": 12.5, "HWG06": 11,
				"HWG07": 19, "HWG08": 17, "HWG09": 10, "HWG10": 12, "HWG11": 14,
				"HWG12": 1111, "HWG13": 27, "HWG14": 1469.5, "HWG15": 766.5,
				"HWG16": 11.5, "HWG17": 15,
			},
			thresholds: map[string]float64{
				"HWG01_F8": 100, "HWG01_V5": 100, "HWG02": 50, "HWG03": 100,
				"HWG04": 50, "HWG05": 50, "HWG06": 50, "HWG07": 100, "HWG08": 50,
				"HWG09": 50, "HWG10": 100, "HWG11": 100, "HWG12": 100,
				"HWG13": 100, "HWG14": 100, "HWG15": 100, "HWG16": 100, "HWG17": 100,
			},
			want: 31.0,
		},
		{
			name: "health screening",
			variables: map[string]float64{
				"HWG01": (3285 + 3338.5) / 2, "HWG01_F8": 3285, "HWG01_V5": 3338.5,
				"HWG02": 1492, "HWG03": 742.5, "HWG04": 19, "HWG05": 18, "HWG06": 15,
				"HWG07": 515, "HWG08": 31, "HWG09": 19, "HWG10": 82, "HWG11": 14,
				"HWG12": 35, "HWG13": 28, "HWG14": 667, "HWG15": 2339,
				"HWG16": 12, "HWG17": 25.5,
			},
			thresholds: map[string]float64{
				"HWG01_F8": 80, "HWG01_V5": 80, "HWG02": 80, "HWG03": 100,
				"HWG04": 80, "HWG05": 80, "HWG06": 80, "HWG07": 100, "HWG08": 80,
				"HWG09": 80, "HWG10": 100, "HWG11": 100, "HWG12": 100,
				"HWG13": 100, "HWG14": 100, "HWG15": 100, "HWG16": 100, "HWG17": 80,
			},
			want: 29.3,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			evaluator := NewFormulaEvaluator(excelAlignedScreeningFormula)
			evaluator.SetVariables(tc.variables)
			evaluator.SetThresholds(tc.thresholds)
			score, err := evaluator.Evaluate()
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			if rounded := math.Round(score*10) / 10; rounded != tc.want {
				t.Fatalf("Evaluate() = %v (rounded %v), want %v", score, rounded, tc.want)
			}
		})
	}
}

func TestV8ModelFromDatabaseUsesConfiguredThresholds(t *testing.T) {
	if os.Getenv("DB_INTEGRATION") != "1" {
		t.Skip("set DB_INTEGRATION=1 to run database-backed model calculation test")
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Fatalf("db.Ping() error = %v", err)
	}

	cases := []struct {
		name string
		data map[string]interface{}
	}{
		{
			name: "row 1",
			data: map[string]interface{}{
				"B14_ACV": 860, "B21_APC": 134, "B33_ADHFE1": 13, "B39_C9o": 338.5,
				"B51_SEP": 54, "B72_THBS1": 2937, "B76_LOC105371031": 31, "Total Events": 586,
			},
		},
		{
			name: "row 2",
			data: map[string]interface{}{
				"B14_ACV": 1134.5, "B21_APC": 504, "B33_ADHFE1": 10, "B39_C9o": 9,
				"B51_SEP": 11, "B72_THBS1": 2896, "B76_LOC105371031": 46, "Total Events": 531,
			},
		},
		{
			name: "row 3",
			data: map[string]interface{}{
				"B14_ACV": 17, "B21_APC": 25, "B33_ADHFE1": 9, "B39_C9o": 9,
				"B51_SEP": 11, "B72_THBS1": 1328, "B76_LOC105371031": 13, "Total Events": 498,
			},
		},
		{
			name: "row 4",
			data: map[string]interface{}{
				"B14_ACV": 732.5, "B21_APC": 117, "B33_ADHFE1": 337, "B39_C9o": 8,
				"B51_SEP": 192, "B72_THBS1": 3032, "B76_LOC105371031": 235, "Total Events": 557,
			},
		},
	}
	var formula string
	if err := db.QueryRow(`SELECT formula FROM setting_model WHERE id = 2`).Scan(&formula); err != nil {
		t.Fatalf("query V8 formula: %v", err)
	}
	thresholds, err := loadModelThresholdMap(db, 2)
	if err != nil {
		t.Fatalf("load V8 thresholds: %v", err)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			score, ok, err := calculateModelFormulaScore(db, 2, tc.data)
			if err != nil {
				t.Fatalf("calculateModelFormulaScore() error = %v", err)
			}
			if !ok {
				t.Fatalf("calculateModelFormulaScore() did not calculate a score")
			}
			evaluator := NewFormulaEvaluator(formula)
			evaluator.SetThresholds(thresholds)
			evaluator.SetVariables(convertGeneDataToFloatMap(expandGeneDataAliasesForFormula(db, tc.data)))
			rawScore, err := evaluator.Evaluate()
			if err != nil {
				t.Fatalf("configured evaluator error = %v", err)
			}
			want := math.Round(rawScore*10) / 10
			if math.Abs(score-want) > 1e-9 {
				t.Fatalf("calculateModelFormulaScore() = %v, configured threshold result = %v", score, want)
			}
		})
	}
}

func TestNewModelsFromDatabaseCalculateExpectedScores(t *testing.T) {
	if os.Getenv("DB_INTEGRATION") != "1" {
		t.Skip("set DB_INTEGRATION=1 to run database-backed model calculation test")
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer db.Close()

	var healthModelID, lungModelID, oldCRCModelID int
	if err := db.QueryRow(`SELECT id FROM setting_model WHERE model_name = '健康筛查' ORDER BY id LIMIT 1`).Scan(&healthModelID); err != nil {
		t.Fatalf("query health model id: %v", err)
	}
	if err := db.QueryRow(`SELECT id FROM setting_model WHERE model_name = '肺癌公式' ORDER BY id LIMIT 1`).Scan(&lungModelID); err != nil {
		t.Fatalf("query lung model id: %v", err)
	}
	if err := db.QueryRow(`SELECT id FROM setting_model WHERE model_name = '肠癌-旧版' ORDER BY id LIMIT 1`).Scan(&oldCRCModelID); err != nil {
		t.Fatalf("query old CRC model id: %v", err)
	}

	cases := []struct {
		name    string
		modelID int
		data    map[string]interface{}
		want    float64
	}{
		{
			name:    "lung",
			modelID: lungModelID,
			data: map[string]interface{}{
				"B55_RASSF1A": 1008, "B57_RARB": 18, "B20_SOX2OT": 18, "B39_SHOX2": 46,
				"B76_LOC105371031": 31, "B21_APC": 237, "B14_SCT": 851, "B51_LOC100130992": 21,
				"B55_PTGER4": 9, "B72_THBS1": 2983, "B78_HOXA7": 13,
			},
			want: 24.3,
		},
		{
			name:    "health",
			modelID: healthModelID,
			data: map[string]interface{}{
				"B55_RASSF1A": 35, "B57_RARB": 28, "B20_SOX2OT": 82, "B39_SHOX2": 14,
				"B76_LOC105371031": 515, "B21_APC": 742.5, "B14_SCT": 667, "B51_LOC100130992": 2339,
				"B55_PTGER4": 12, "B72_THBS1": (3285 + 3338.5) / 2, "B78_HOXA7": 25.5,
				"B14_ACV": 1492, "B33_ADHFE1": 19, "B37_HIST1H4F": 31, "B39_C9o": 18,
				"B51_SEP": 15, "B74_SDC2": 19,
			},
			want: 29.3,
		},
		{
			name:    "old CRC",
			modelID: oldCRCModelID,
			data: map[string]interface{}{
				"B55_RASSF1A": 35, "B57_RARB": 28, "B20_SOX2OT": 82, "B39_SHOX2": 14,
				"B76_LOC105371031": 515, "B21_APC": 742.5, "B14_SCT": 667, "B51_LOC100130992": 2339,
				"B55_PTGER4": 12, "B72_THBS1": (3285 + 3338.5) / 2, "B78_HOXA7": 25.5,
				"B14_ACV": 1492, "B33_ADHFE1": 19, "B37_HIST1H4F": 31, "B39_C9o": 18,
				"B51_SEP": 15, "B74_SDC2": 19,
			},
			want: 29.3,
		},
		{
			name:    "old CRC Hou Xingde Excel duplicate",
			modelID: oldCRCModelID,
			data: map[string]interface{}{
				"HWG01": (3392 + 2375.5) / 2, "HWG01_F8": 3392, "HWG01_V5": 2375.5,
				"HWG02": 135, "HWG03": 285, "HWG04": 105, "HWG05": 12.5, "HWG06": 11,
				"HWG07": 19, "HWG08": 17, "HWG09": 10, "HWG10": 12, "HWG11": 14,
				"HWG12": 1111, "HWG13": 27, "HWG14": 1469.5, "HWG15": 766.5,
				"HWG16": 11.5, "HWG17": 15,
			},
			want: 31.0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			score, ok, err := calculateModelFormulaScore(db, tc.modelID, tc.data)
			if err != nil {
				t.Fatalf("calculateModelFormulaScore() error = %v", err)
			}
			if !ok {
				t.Fatalf("calculateModelFormulaScore() did not calculate a score")
			}
			if math.Abs(score-tc.want) > 1e-9 {
				t.Fatalf("calculateModelFormulaScore() = %v, want %v", score, tc.want)
			}
		})
	}

	t.Run("health and old CRC use their own thresholds", func(t *testing.T) {
		thresholdSensitiveData := map[string]interface{}{
			"B55_RASSF1A": 60, "B57_RARB": 60, "B20_SOX2OT": 60, "B39_SHOX2": 60,
			"B76_LOC105371031": 60, "B21_APC": 60, "B14_SCT": 60, "B51_LOC100130992": 60,
			"B55_PTGER4": 60, "B72_THBS1": 60, "B78_HOXA7": 60, "B14_ACV": 60,
			"B33_ADHFE1": 60, "B37_HIST1H4F": 60, "B39_C9o": 60, "B51_SEP": 60,
			"B74_SDC2": 60,
		}
		healthScore, healthOK, err := calculateModelFormulaScore(db, healthModelID, thresholdSensitiveData)
		if err != nil || !healthOK {
			t.Fatalf("calculate health score: score=%v ok=%v err=%v", healthScore, healthOK, err)
		}
		oldCRCScore, oldCRCOK, err := calculateModelFormulaScore(db, oldCRCModelID, thresholdSensitiveData)
		if err != nil || !oldCRCOK {
			t.Fatalf("calculate old CRC score: score=%v ok=%v err=%v", oldCRCScore, oldCRCOK, err)
		}
		if healthScore == oldCRCScore {
			t.Fatalf("model-specific thresholds were ignored: both scores are %v", healthScore)
		}
	})
}
