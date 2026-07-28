package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

var orderedTreatmentStageNames = []string{
	"健康体检",
	"辅助诊断",
	"术前评估",
	"术后检测",
	"残留检测",
	"复发监测",
	"化疗前",
	"化疗后",
}

var orderedTreatmentStageRank = func() map[string]int {
	ranks := make(map[string]int, len(orderedTreatmentStageNames))
	for index, name := range orderedTreatmentStageNames {
		ranks[name] = index
	}
	return ranks
}()

type ReportHistoryRow struct {
	Time   string  `json:"time"`
	Signal float64 `json:"signal"`
	Type   string  `json:"type"`
	Trend  string  `json:"trend"`
	Note   string  `json:"note"`
}

func treatmentStageRank(name string) int {
	if rank, ok := orderedTreatmentStageRank[normalizeReportTreatmentStageName(name)]; ok {
		return rank
	}
	return len(orderedTreatmentStageNames) + 100
}

func isAllowedTreatmentStage(name string) bool {
	_, ok := orderedTreatmentStageRank[normalizeReportTreatmentStageName(name)]
	return ok
}

func isPreoperativeStage(name string) bool {
	return strings.Contains(normalizeReportTreatmentStageName(name), "术前")
}

func isPostoperativeStage(name string) bool {
	return normalizeReportTreatmentStageName(name) == "术后检测"
}

func normalizeReportTrend(trend string) string {
	switch strings.TrimSpace(trend) {
	case "上升":
		return "↑"
	case "下降":
		return "↓"
	case "稳定":
		return "-"
	case "":
		return "-"
	default:
		return trend
	}
}

func calculateReportTrend(current, previous float64) string {
	if math.Abs(current-previous) < 1 {
		return "-"
	}
	if current > previous {
		return "↑"
	}
	return "↓"
}

func reportStringValue(value interface{}) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}

func reportFloatValue(value interface{}) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		f, _ := strconv.ParseFloat(v.String(), 64)
		return f
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return f
	default:
		f, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprintf("%v", v)), 64)
		return f
	}
}

func reportDateString(value interface{}) string {
	text := reportStringValue(value)
	if text == "" {
		return ""
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z07:00", "2006-01-02 15:04:05", "2006-01-02"} {
		if parsed, err := time.Parse(layout, text); err == nil {
			return parsed.Format("2006-01-02")
		}
	}
	if len(text) >= 10 {
		return text[:10]
	}
	return text
}

func reportHistoryRowsFromValue(value interface{}) []ReportHistoryRow {
	var rows []ReportHistoryRow
	switch list := value.(type) {
	case []ReportHistoryRow:
		rows = append(rows, list...)
	case []map[string]interface{}:
		for _, item := range list {
			rows = append(rows, reportHistoryRowFromMap(item))
		}
	case []interface{}:
		for _, raw := range list {
			if item, ok := raw.(map[string]interface{}); ok {
				rows = append(rows, reportHistoryRowFromMap(item))
			}
		}
	}
	return compactReportHistoryRows(rows)
}

func reportHistoryRowFromMap(item map[string]interface{}) ReportHistoryRow {
	return ReportHistoryRow{
		Time:   reportDateString(item["time"]),
		Signal: reportFloatValue(item["signal"]),
		Type:   reportStringValue(item["type"]),
		Trend:  normalizeReportTrend(reportStringValue(item["trend"])),
		Note:   reportStringValue(item["note"]),
	}
}

func compactReportHistoryRows(rows []ReportHistoryRow) []ReportHistoryRow {
	return compactReportHistoryRowsLimit(rows, 3)
}

func compactReportHistoryRowsLimit(rows []ReportHistoryRow, limit int) []ReportHistoryRow {
	cleaned := make([]ReportHistoryRow, 0, len(rows))
	for _, row := range rows {
		row.Time = reportDateString(row.Time)
		row.Type = strings.TrimSpace(row.Type)
		row.Trend = normalizeReportTrend(row.Trend)
		if row.Time == "" && row.Signal == 0 && row.Type == "" && row.Note == "" {
			continue
		}
		cleaned = append(cleaned, row)
	}
	if limit > 0 && len(cleaned) > limit {
		return cleaned[:limit]
	}
	return cleaned
}

func buildReportHistoryRowsFromFields(data map[string]interface{}) []ReportHistoryRow {
	rows := make([]ReportHistoryRow, 0, 3)
	for index := 2; index <= 4; index++ {
		rows = append(rows, ReportHistoryRow{
			Time:   reportDateString(data[fmt.Sprintf("time%d", index)]),
			Signal: reportFloatValue(data[fmt.Sprintf("signal%d", index)]),
			Trend:  normalizeReportTrend(reportStringValue(data[fmt.Sprintf("trend%d", index)])),
			Type:   reportStringValue(data[fmt.Sprintf("type%d", index)]),
			Note:   reportStringValue(data[fmt.Sprintf("note%d", index)]),
		})
	}
	return compactReportHistoryRows(rows)
}

func syncReportHistoryFields(data map[string]interface{}, current ReportHistoryRow, histories []ReportHistoryRow) {
	if data == nil {
		return
	}
	if current.Time == "" {
		current.Time = reportDateString(data["time1"])
	}
	if current.Signal == 0 {
		current.Signal = reportFloatValue(data["signal1"])
		if current.Signal == 0 {
			current.Signal = reportFloatValue(data["calculationResult"])
		}
	}
	if current.Trend == "" {
		current.Trend = normalizeReportTrend(reportStringValue(data["trend1"]))
		if current.Trend == "-" {
			current.Trend = normalizeReportTrend(reportStringValue(data["trend"]))
		}
	}
	if current.Type == "" {
		current.Type = reportStringValue(data["type1"])
		if current.Type == "" {
			current.Type = reportStringValue(data["treatmentStageName"])
		}
	}
	if current.Note == "" {
		current.Note = reportStringValue(data["note1"])
		if current.Note == "" {
			current.Note = reportStringValue(data["remarks"])
		}
	}

	if len(histories) == 0 {
		histories = reportHistoryRowsFromValue(data["selectedHistoricalReports"])
	}
	if len(histories) == 0 {
		histories = buildReportHistoryRowsFromFields(data)
	}
	histories = compactReportHistoryRows(histories)
	if len(histories) > 0 {
		latestIndex := 0
		for index := 1; index < len(histories); index++ {
			if histories[index].Time > histories[latestIndex].Time {
				latestIndex = index
			}
		}
		current.Trend = calculateReportTrend(current.Signal, histories[latestIndex].Signal)
	}

	displayRows := compactReportHistoryRowsLimit(append([]ReportHistoryRow{current}, histories...), 4)
	current = displayRows[0]
	histories = displayRows[1:]

	data["time1"] = current.Time
	data["signal1"] = current.Signal
	data["trend1"] = current.Trend
	data["type1"] = current.Type
	data["note1"] = current.Note
	data["trend"] = current.Trend
	data["selectedHistoricalReports"] = histories

	for index := 0; index < 3; index++ {
		fieldIndex := index + 2
		row := ReportHistoryRow{}
		if index < len(histories) {
			row = histories[index]
		}
		data[fmt.Sprintf("time%d", fieldIndex)] = row.Time
		data[fmt.Sprintf("signal%d", fieldIndex)] = row.Signal
		data[fmt.Sprintf("trend%d", fieldIndex)] = row.Trend
		data[fmt.Sprintf("type%d", fieldIndex)] = row.Type
		data[fmt.Sprintf("note%d", fieldIndex)] = row.Note
	}
}

func loadSameBatchPriorStageHistories(db *sql.DB, sampleID int, patientID int, currentStage string, excludeReportID int) []ReportHistoryRow {
	currentRank := treatmentStageRank(currentStage)
	if currentRank >= len(orderedTreatmentStageNames) || sampleID == 0 || patientID == 0 {
		return []ReportHistoryRow{}
	}

	rows, err := db.Query(`
		SELECT r.id, COALESCE(r.report_data, ''), COALESCE(s.receive_date, s.collection_date, s.sample_created_at), COALESCE(ts.name, ''), COALESCE(s.notes, '')
		FROM detect_report r
		JOIN detect_sample s ON r.sample_id = s.id
		JOIN detect_sample current_s ON current_s.id = ?
		LEFT JOIN setting_treatment_stage ts ON s.treatment_stage_id = ts.id
		WHERE r.patient_id = ?
			AND r.sample_id <> ?
			AND (? = 0 OR r.id <> ?)
			AND current_s.batch_id IS NOT NULL
			AND s.batch_id = current_s.batch_id
			AND r.status NOT IN ('rejected')
	`, sampleID, patientID, sampleID, excludeReportID, excludeReportID)
	if err != nil {
		log.Printf("Failed to query same-batch prior stage reports: %v", err)
		return []ReportHistoryRow{}
	}
	defer rows.Close()

	histories := []ReportHistoryRow{}
	for rows.Next() {
		var reportID int
		var reportDataJSON, stageName, note string
		var sampleDate time.Time
		if err := rows.Scan(&reportID, &reportDataJSON, &sampleDate, &stageName, &note); err != nil {
			log.Printf("Failed to scan same-batch prior stage report: %v", err)
			continue
		}
		stageRank := treatmentStageRank(stageName)
		if stageRank >= currentRank {
			continue
		}
		row := ReportHistoryRow{
			Time:  sampleDate.Format("2006-01-02"),
			Type:  stageName,
			Trend: "-",
			Note:  note,
		}
		reportData := map[string]interface{}{}
		if strings.TrimSpace(reportDataJSON) != "" {
			if err := json.Unmarshal([]byte(reportDataJSON), &reportData); err == nil {
				if value := reportDateString(reportData["time1"]); value != "" {
					row.Time = value
				}
				if value := reportFloatValue(reportData["signal1"]); value != 0 {
					row.Signal = value
				} else if value := reportFloatValue(reportData["calculationResult"]); value != 0 {
					row.Signal = value
				}
				if value := reportStringValue(reportData["type1"]); value != "" {
					row.Type = value
				} else if value := reportStringValue(reportData["treatmentStageName"]); value != "" {
					row.Type = value
				}
				if value := reportStringValue(reportData["note1"]); value != "" {
					row.Note = value
				} else if value := reportStringValue(reportData["remarks"]); value != "" {
					row.Note = value
				}
			}
		}
		histories = append(histories, row)
		_ = reportID
	}

	sort.SliceStable(histories, func(i, j int) bool {
		leftRank := treatmentStageRank(histories[i].Type)
		rightRank := treatmentStageRank(histories[j].Type)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		return histories[i].Time < histories[j].Time
	})
	return compactReportHistoryRows(histories)
}
