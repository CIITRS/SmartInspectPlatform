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

type ReportPositionValue struct {
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Width    float64 `json:"width"`
	Height   float64 `json:"height"`
	FontSize float64 `json:"fontSize,omitempty"`
	Align    string  `json:"align,omitempty"`
}

type ReportPositionConfig struct {
	ID             int                            `json:"id"`
	PositionKey    string                         `json:"positionKey"`
	PositionName   string                         `json:"positionName"`
	SampleTypeID   int                            `json:"sampleTypeId"`
	ReportType     string                         `json:"reportType"`
	PageNumber     int                            `json:"pageNumber"`
	BackgroundPath string                         `json:"backgroundPath"`
	Positions      map[string]ReportPositionValue `json:"positions"`
	IsActive       int                            `json:"isActive"`
}

var bloodNormalTableColumns = []struct {
	key      string
	x, width float64
}{
	{key: "Time", x: 61.06, width: 25.86},
	{key: "Signal", x: 86.92, width: 19.40},
	{key: "Trend", x: 106.32, width: 19.72},
	{key: "Type", x: 126.04, width: 30.67},
	{key: "Note", x: 156.71, width: 40.79},
}

var bloodNormalTableRows = []float64{90.04, 97.36, 104.77, 112.19}

func applyBloodNormalTableCalibration(positions map[string]ReportPositionValue) {
	for row, y := range bloodNormalTableRows {
		for _, column := range bloodNormalTableColumns {
			positions[fmt.Sprintf("%s%d", column.key, row+1)] = ReportPositionValue{
				X: column.x, Y: y, Width: column.width, Height: 6, FontSize: 10, Align: "center",
			}
		}
	}
}

func nearPosition(actual, expected, tolerance float64) bool {
	return math.Abs(actual-expected) <= tolerance
}

// Only migrate the known legacy defaults and the roughly aligned Blood_Normal
// coordinates. This keeps later manual fine tuning in the position editor intact.
func needsBloodNormalTableCalibration(positions map[string]ReportPositionValue) bool {
	time1, hasTime := positions["Time1"]
	note1, hasNote := positions["Note1"]
	if !hasTime || !hasNote {
		return true
	}
	legacy := nearPosition(time1.X, 61.5, 0.1) && nearPosition(note1.X, 126.6, 0.1)
	rough := nearPosition(time1.X, 62.0, 0.25) && nearPosition(note1.X, 157.5, 0.35)
	return legacy || rough
}

// defaultReportPositions mirrors the original fillPage coordinates in pdf.go,
// with the Blood_Normal trend table measured from the actual report artwork.
func defaultReportPositions() map[string]ReportPositionValue {
	positions := map[string]ReportPositionValue{
		"NameP2":             {X: 30, Y: 72.5, Width: 28, Height: 6, FontSize: 10},
		"SexP2":              {X: 62, Y: 72.5, Width: 18, Height: 6, FontSize: 10},
		"AgeP2":              {X: 92, Y: 72.5, Width: 18, Height: 6, FontSize: 10},
		"SampleType":         {X: 149, Y: 71.5, Width: 42, Height: 6, FontSize: 10},
		"SampleTime":         {X: 149, Y: 81, Width: 42, Height: 6, FontSize: 10},
		"Project":            {X: 30, Y: 87.3, Width: 50, Height: 6, FontSize: 10},
		"NumberID":           {X: 92, Y: 87.3, Width: 48, Height: 6, FontSize: 10},
		"Organization":       {X: 149, Y: 90, Width: 42, Height: 6, FontSize: 10},
		"Inspector":          {X: 32, Y: 251.2, Width: 50, Height: 6, FontSize: 10},
		"Reviewer":           {X: 105, Y: 251.2, Width: 50, Height: 6, FontSize: 10},
		"ReportTime":         {X: 175.5, Y: 251.2, Width: 28, Height: 6, FontSize: 10},
		"SignalInstructions": {X: 42.5, Y: 136.5, Width: 150, Height: 15, FontSize: 10},
		"ResultInstructions": {X: 42.5, Y: 154, Width: 150, Height: 28, FontSize: 10},
	}
	applyBloodNormalTableCalibration(positions)
	return positions
}

func ensureDefaultReportPositions(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("database is not available")
	}
	positionsJSON, err := json.Marshal(defaultReportPositions())
	if err != nil {
		return err
	}
	defaults := []struct {
		key, name              string
		sampleID               int
		reportType, background string
	}{
		{"blood_normal", "血液正常报告", 1, "normal", "Template/Template_Report/Blood_Normal.jpg"},
		{"blood_high", "血液高敏报告", 1, "high", "Template/Template_Report/Blood_Sensitivity.jpg"},
		{"urine_normal", "尿液正常报告", 2, "normal", "Template/Template_Report/Urine_Normal.jpg"},
		{"urine_high", "尿液高敏报告", 2, "high", "Template/Template_Report/Urine_Sensitivity.jpg"},
		{"physical_examination", "早筛报告", 0, "screening", "Template/Template_Report/Physical_examination.jpg"},
	}
	for _, item := range defaults {
		_, err := db.Exec(`INSERT INTO setting_report_position
			(position_key, position_name, sample_type_id, report_type, page_number, background_path, positions_json, is_active)
			VALUES (?, ?, ?, ?, 3, ?, ?, 1)
			ON DUPLICATE KEY UPDATE position_name = VALUES(position_name)`,
			item.key, item.name, item.sampleID, item.reportType, item.background, string(positionsJSON))
		if err != nil {
			return err
		}
		if item.key == "blood_normal" {
			var currentJSON string
			if err := db.QueryRow(`SELECT positions_json FROM setting_report_position WHERE position_key = ?`, item.key).Scan(&currentJSON); err != nil {
				return err
			}
			current := map[string]ReportPositionValue{}
			if err := json.Unmarshal([]byte(currentJSON), &current); err == nil && needsBloodNormalTableCalibration(current) {
				applyBloodNormalTableCalibration(current)
				calibratedJSON, marshalErr := json.Marshal(current)
				if marshalErr != nil {
					return marshalErr
				}
				if _, err := db.Exec(`UPDATE setting_report_position SET positions_json = ?, updated_at = NOW() WHERE position_key = ?`, string(calibratedJSON), item.key); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func normalizeAssignedReportType(reportType string) string {
	switch strings.ToLower(strings.TrimSpace(reportType)) {
	case "high", "super", "supersensitive", "超敏", "超敏报告":
		return "high"
	case "screening", "physical", "体检", "健康筛查", "早筛", "早筛检查":
		return "screening"
	case "normal", "standard", "detailed", "普通", "普通报告", "高敏", "高敏报告", "":
		return "normal"
	default:
		return "normal"
	}
}

func reportTypeDisplayLabel(reportType string) string {
	switch normalizeAssignedReportType(reportType) {
	case "high":
		return "超敏"
	case "screening":
		return "健康筛查"
	default:
		return "高敏"
	}
}

func reportTypeAssayLabel(reportType string) string {
	switch normalizeAssignedReportType(reportType) {
	case "high":
		return "MePlex超敏180CpG"
	case "screening":
		return "早筛检查"
	default:
		return "MePlex高敏98CpG"
	}
}

func reportTypeFullLabel(reportType string) string {
	return fmt.Sprintf("%s（%s）", reportTypeDisplayLabel(reportType), reportTypeAssayLabel(reportType))
}

func scanReportPosition(scanner interface{ Scan(...interface{}) error }) (ReportPositionConfig, error) {
	var config ReportPositionConfig
	var positionsJSON string
	err := scanner.Scan(&config.ID, &config.PositionKey, &config.PositionName, &config.SampleTypeID,
		&config.ReportType, &config.PageNumber, &config.BackgroundPath, &positionsJSON, &config.IsActive)
	if err != nil {
		return config, err
	}
	config.Positions = defaultReportPositions()
	if positionsJSON != "" {
		if err := json.Unmarshal([]byte(positionsJSON), &config.Positions); err != nil {
			log.Printf("解析报告定位失败 position=%s: %v", config.PositionKey, err)
			config.Positions = defaultReportPositions()
		}
	}
	if normalizeAssignedReportType(config.ReportType) == "high" {
		delete(config.Positions, "Organization")
	}
	return config, nil
}

func resolveReportPosition(db *sql.DB, sampleTypeID int, assignedReportType string) ReportPositionConfig {
	_ = ensureDefaultReportPositions(db)
	reportType := normalizeAssignedReportType(assignedReportType)
	query := `SELECT id, position_key, position_name, sample_type_id, report_type, page_number,
		background_path, positions_json, is_active
		FROM setting_report_position
		WHERE is_active = 1 AND report_type = ? AND sample_type_id IN (?, 0)
		ORDER BY CASE WHEN sample_type_id = ? THEN 0 ELSE 1 END LIMIT 1`
	config, err := scanReportPosition(db.QueryRow(query, reportType, sampleTypeID, sampleTypeID))
	if err == nil {
		return config
	}
	return ReportPositionConfig{
		PositionKey: "blood_normal", PositionName: "血液正常报告", SampleTypeID: 1,
		ReportType: "normal", PageNumber: 3, BackgroundPath: "Template/Template_Report/Blood_Normal.jpg",
		Positions: defaultReportPositions(), IsActive: 1,
	}
}

func getReportPositionsData(db *sql.DB) (utils.H, error) {
	if err := ensureDefaultReportPositions(db); err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT id, position_key, position_name, sample_type_id, report_type, page_number,
		background_path, positions_json, is_active FROM setting_report_position ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []ReportPositionConfig{}
	for rows.Next() {
		item, err := scanReportPosition(rows)
		if err == nil {
			list = append(list, item)
		}
	}
	return utils.H{"list": list, "total": len(list)}, rows.Err()
}

func HandleListReportPositions(c *app.RequestContext, db *sql.DB) {
	data, err := getReportPositionsData(db)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: err.Error()})
		return
	}
	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "获取报告定位成功", Data: data})
}

func HandleUpdateReportPosition(c *app.RequestContext, db *sql.DB) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "无效的定位ID"})
		return
	}
	var req struct {
		PositionName   string                         `json:"positionName"`
		BackgroundPath string                         `json:"backgroundPath"`
		Positions      map[string]ReportPositionValue `json:"positions"`
		IsActive       int                            `json:"isActive"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "请求参数错误"})
		return
	}
	var reportType string
	if err := db.QueryRow("SELECT report_type FROM setting_report_position WHERE id = ?", id).Scan(&reportType); err != nil {
		c.JSON(consts.StatusNotFound, ApiResponse{Code: 404, Success: false, Message: "报告定位不存在"})
		return
	}
	if normalizeAssignedReportType(reportType) == "high" {
		delete(req.Positions, "Organization")
	}
	positionsJSON, err := json.Marshal(req.Positions)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "坐标格式错误"})
		return
	}
	result, err := db.Exec(`UPDATE setting_report_position SET position_name = ?, background_path = ?,
		positions_json = ?, is_active = ?, updated_at = NOW() WHERE id = ?`,
		req.PositionName, req.BackgroundPath, string(positionsJSON), req.IsActive, id)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: err.Error()})
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(consts.StatusNotFound, ApiResponse{Code: 404, Success: false, Message: "报告定位不存在"})
		return
	}
	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "保存报告定位成功"})
}
