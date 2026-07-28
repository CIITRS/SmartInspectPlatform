package handlers

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func parseCSVFileReader(reader io.Reader) (*csv.Reader, error) {
	csvReader := csv.NewReader(reader)
	csvReader.FieldsPerRecord = -1
	return csvReader, nil
}

type PanelMatchResult struct {
	PanelID      int      `json:"panelId"`
	PanelName    string   `json:"panelName"`
	PanelCode    string   `json:"panelCode"`
	MatchRate    float64  `json:"matchRate"`
	MatchedGenes []string `json:"matchedGenes"`
	MissingGenes []string `json:"missingGenes"`
	ExtraGenes   []string `json:"extraGenes"`
	MatchStatus  string   `json:"matchStatus"`
}

type cancerTypePanelInfo struct {
	ID       int
	Name     string
	PanelIDs []int
}

func ensureBatchSampleModelIDColumn(db *sql.DB) error {
	var exists bool
	if err := db.QueryRow(`SELECT EXISTS(
		SELECT 1 FROM information_schema.columns
		WHERE table_schema = DATABASE() AND table_name = 'detect_batch_sample' AND column_name = 'model_id'
	)`).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err := db.Exec(`ALTER TABLE detect_batch_sample ADD COLUMN model_id INT NULL AFTER cancer_type_id`)
	return err
}

type samplePanelMatchCache struct {
	MatchedPanelIDs []int
	PanelMatches    []utils.H
	SampleGenes     []string
}

func parsePanelIDList(panelIDsStr string) []int {
	var panelIDs []int
	for _, part := range strings.Split(panelIDsStr, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if panelID, err := strconv.Atoi(part); err == nil {
			panelIDs = append(panelIDs, panelID)
		}
	}
	return panelIDs
}

func panelIDSet(panelIDs []int) map[int]bool {
	set := make(map[int]bool, len(panelIDs))
	for _, panelID := range panelIDs {
		set[panelID] = true
	}
	return set
}

func panelSetCovers(required []int, available map[int]bool) bool {
	if len(required) == 0 {
		return false
	}
	for _, panelID := range required {
		if !available[panelID] {
			return false
		}
	}
	return true
}

func getPanelGeneSymbols(db *sql.DB, geneIDsStr string) []string {
	var genes []string
	for _, gid := range strings.Split(geneIDsStr, ",") {
		gid = strings.TrimSpace(gid)
		if gid == "" {
			continue
		}
		var geneSymbol string
		err := db.QueryRow("SELECT gene_symbol FROM setting_gene WHERE id = ?", gid).Scan(&geneSymbol)
		if err == nil && geneSymbol != "" {
			genes = append(genes, geneSymbol)
		} else {
			genes = append(genes, gid)
		}
	}
	return genes
}

func isBatchGeneMetaColumn(key string) bool {
	switch strings.TrimSpace(key) {
	case "", "Sample", "sample_code", "location", "Location", "Total Events", "totalEvents":
		return true
	default:
		return false
	}
}

func uniqueSortedGeneSymbols(genes []string) []string {
	seen := make(map[string]string)
	for _, gene := range genes {
		gene = strings.TrimSpace(gene)
		if gene == "" {
			continue
		}
		lower := strings.ToLower(gene)
		if _, exists := seen[lower]; !exists {
			seen[lower] = gene
		}
	}
	result := make([]string, 0, len(seen))
	for _, gene := range seen {
		result = append(result, gene)
	}
	sort.Strings(result)
	return result
}

func getCancerTypeRequiredGenes(db *sql.DB, panelIDs []int) ([]string, error) {
	if len(panelIDs) == 0 {
		return []string{}, nil
	}
	genes := make([]string, 0)
	for _, panelID := range panelIDs {
		var geneIDsStr string
		if err := db.QueryRow(`SELECT COALESCE(gene_ids, '') FROM setting_panel WHERE id = ? AND is_active = 1`, panelID).Scan(&geneIDsStr); err != nil {
			if err == sql.ErrNoRows {
				continue
			}
			return nil, err
		}
		genes = append(genes, getPanelGeneSymbols(db, geneIDsStr)...)
	}
	return uniqueSortedGeneSymbols(genes), nil
}

func getBatchSampleGenes(db *sql.DB, batchID int, sampleCode string) ([]string, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	medianData, err := loadBatchMedianDataForSubmit(tx, batchID)
	if err != nil {
		return nil, err
	}
	genes := make([]string, 0)
	for _, data := range medianData {
		currentSampleCode, _ := data["Sample"].(string)
		if currentSampleCode == "" {
			currentSampleCode, _ = data["sample_code"].(string)
		}
		if strings.TrimSpace(currentSampleCode) != sampleCode {
			continue
		}
		for key := range data {
			if isBatchGeneMetaColumn(key) {
				continue
			}
			geneSymbol, _ := getGeneSymbolByAnyName(db, key)
			if geneSymbol == "" {
				geneSymbol = key
			}
			genes = append(genes, geneSymbol)
		}
		break
	}
	return uniqueSortedGeneSymbols(genes), nil
}

func missingRequiredGenes(requiredGenes []string, sampleGenes []string) []string {
	sampleGeneSet := make(map[string]bool, len(sampleGenes))
	for _, gene := range sampleGenes {
		sampleGeneSet[strings.ToLower(strings.TrimSpace(gene))] = true
	}
	missing := make([]string, 0)
	for _, gene := range requiredGenes {
		if !sampleGeneSet[strings.ToLower(strings.TrimSpace(gene))] {
			missing = append(missing, gene)
		}
	}
	return missing
}

func getActiveCancerTypePanelInfos(db *sql.DB) ([]cancerTypePanelInfo, error) {
	rows, err := db.Query(`SELECT id, name, COALESCE(panel_ids, '') FROM setting_cancer_type WHERE is_active = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cancerTypes []cancerTypePanelInfo
	for rows.Next() {
		var ct cancerTypePanelInfo
		var panelIDsStr string
		if err := rows.Scan(&ct.ID, &ct.Name, &panelIDsStr); err != nil {
			continue
		}
		ct.PanelIDs = parsePanelIDList(panelIDsStr)
		if len(ct.PanelIDs) > 0 {
			cancerTypes = append(cancerTypes, ct)
		}
	}
	return cancerTypes, rows.Err()
}

func loadSamplePanelMatchCacheForBatch(db *sql.DB, batchID int) (map[string]samplePanelMatchCache, error) {
	rows, err := db.Query(`SELECT sample_code, matched_panel_ids_json, panel_matches_json, COALESCE(sample_genes_json, '')
		FROM detect_sample_panel_match
		WHERE batch_id = ?`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cache := make(map[string]samplePanelMatchCache)
	for rows.Next() {
		var sampleCode string
		var idsJSON, matchesJSON, genesJSON sql.NullString
		if err := rows.Scan(&sampleCode, &idsJSON, &matchesJSON, &genesJSON); err != nil {
			continue
		}
		if !matchesJSON.Valid || strings.TrimSpace(matchesJSON.String) == "" {
			continue
		}
		item := samplePanelMatchCache{}
		if idsJSON.Valid && idsJSON.String != "" {
			_ = json.Unmarshal([]byte(idsJSON.String), &item.MatchedPanelIDs)
		}
		if err := json.Unmarshal([]byte(matchesJSON.String), &item.PanelMatches); err != nil {
			continue
		}
		if genesJSON.Valid && genesJSON.String != "" {
			_ = json.Unmarshal([]byte(genesJSON.String), &item.SampleGenes)
		}
		cache[sampleCode] = item
	}
	return cache, rows.Err()
}

func isSamplePanelMatchCacheComplete(db *sql.DB, batchID int) bool {
	var sampleCount, cacheCount int
	if err := db.QueryRow(`SELECT COUNT(DISTINCT sample_code) FROM detect_batch_platform_data WHERE batch_id = ? AND sample_code != 'H'`, batchID).Scan(&sampleCount); err != nil || sampleCount == 0 {
		return false
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM detect_sample_panel_match
		WHERE batch_id = ?
			AND COALESCE(NULLIF(TRIM(panel_matches_json), ''), '') <> ''
			AND COALESCE(NULLIF(TRIM(sample_genes_json), ''), '') <> ''`, batchID).Scan(&cacheCount); err != nil {
		return false
	}
	return cacheCount >= sampleCount
}

func saveSamplePanelMatchCache(db *sql.DB, batchID int, sampleCode string, matchedPanelIDs []int, panelMatches []utils.H, sampleGenes []string) {
	sort.Ints(matchedPanelIDs)
	idsJSON, _ := json.Marshal(matchedPanelIDs)
	matchesJSON, _ := json.Marshal(panelMatches)
	genesJSON, _ := json.Marshal(sampleGenes)
	if _, err := db.Exec(`INSERT INTO detect_sample_panel_match (batch_id, sample_code, matched_panel_ids_json, panel_matches_json, sample_genes_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, NOW(), NOW())
		ON DUPLICATE KEY UPDATE matched_panel_ids_json = VALUES(matched_panel_ids_json), panel_matches_json = VALUES(panel_matches_json), sample_genes_json = VALUES(sample_genes_json), updated_at = NOW()`,
		batchID, sampleCode, string(idsJSON), string(matchesJSON), string(genesJSON)); err != nil {
		log.Printf("saveSamplePanelMatchCache: failed to cache panel matches for %s: %v", sampleCode, err)
	}
}

func getSampleMatchedPanelsForBatch(db *sql.DB, batchID int, sampleCode string) ([]int, []utils.H, error) {
	var matchedPanelIDsJSON, panelMatchesJSON, sampleGenesJSON sql.NullString
	if err := db.QueryRow(`SELECT matched_panel_ids_json, panel_matches_json, COALESCE(sample_genes_json, '') FROM detect_sample_panel_match WHERE batch_id = ? AND sample_code = ?`, batchID, sampleCode).
		Scan(&matchedPanelIDsJSON, &panelMatchesJSON, &sampleGenesJSON); err == nil && panelMatchesJSON.Valid && panelMatchesJSON.String != "" && sampleGenesJSON.Valid && strings.TrimSpace(sampleGenesJSON.String) != "" {
		var cachedIDs []int
		var cachedMatches []utils.H
		if matchedPanelIDsJSON.Valid && matchedPanelIDsJSON.String != "" {
			_ = json.Unmarshal([]byte(matchedPanelIDsJSON.String), &cachedIDs)
		}
		if err := json.Unmarshal([]byte(panelMatchesJSON.String), &cachedMatches); err == nil {
			return cachedIDs, cachedMatches, nil
		}
	}

	rows, err := db.Query(`
		SELECT platform, median_data
		FROM detect_batch_platform_data
		WHERE batch_id = ? AND sample_code = ? AND sample_code != 'H'
		ORDER BY platform
	`, batchID, sampleCode)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	matchedPanelSet := make(map[int]bool)
	var matchedPanelIDs []int
	var panelMatches []utils.H
	allSampleGeneSet := make(map[string]string)

	for rows.Next() {
		var platform string
		var medianJSON sql.NullString
		if err := rows.Scan(&platform, &medianJSON); err != nil {
			continue
		}

		var median map[string]interface{}
		if medianJSON.Valid && medianJSON.String != "" {
			_ = json.Unmarshal([]byte(medianJSON.String), &median)
		}

		sampleGenes := make([]string, 0)
		sampleGeneSet := make(map[string]bool)
		sampleGeneLowerSet := make(map[string]bool)
		for key := range median {
			if key == "Sample" || key == "sample_code" || key == "location" || key == "Location" || key == "Total Events" {
				continue
			}
			geneSymbol, _ := getGeneSymbolByAnyName(db, key)
			if geneSymbol == "" {
				geneSymbol = key
			}
			sampleGenes = append(sampleGenes, geneSymbol)
			sampleGeneSet[geneSymbol] = true
			sampleGeneLowerSet[strings.ToLower(geneSymbol)] = true
			if _, exists := allSampleGeneSet[strings.ToLower(geneSymbol)]; !exists {
				allSampleGeneSet[strings.ToLower(geneSymbol)] = geneSymbol
			}
		}

		var panelID int
		var panelName, panelCode, geneIDsStr string
		err := db.QueryRow(`SELECT id, panel_name, panel_code, COALESCE(gene_ids, '') FROM setting_panel WHERE panel_code = ? AND is_active = 1`, strings.TrimSpace(platform)).
			Scan(&panelID, &panelName, &panelCode, &geneIDsStr)
		if err != nil {
			log.Printf("getSampleMatchedPanelsForBatch: failed to query panel by code %s: %v", platform, err)
			continue
		}

		panelGenes := getPanelGeneSymbols(db, geneIDsStr)
		panelGeneSet := make(map[string]bool, len(panelGenes))
		panelGeneLowerSet := make(map[string]bool, len(panelGenes))
		for _, gene := range panelGenes {
			panelGeneSet[gene] = true
			panelGeneLowerSet[strings.ToLower(gene)] = true
		}

		var missingGenes []string
		matchCount := 0
		for _, gene := range panelGenes {
			if sampleGeneSet[gene] || sampleGeneLowerSet[strings.ToLower(gene)] {
				matchCount++
			} else {
				missingGenes = append(missingGenes, gene)
			}
		}

		var extraGenes []string
		for _, gene := range sampleGenes {
			if !panelGeneSet[gene] && !panelGeneLowerSet[strings.ToLower(gene)] {
				extraGenes = append(extraGenes, gene)
			}
		}

		matchRate := 0.0
		if len(panelGenes) > 0 {
			matchRate = float64(matchCount) / float64(len(panelGenes))
		}
		matchStatus := "insufficient"
		matchColor := "red"
		selectable := len(panelGenes) > 0 && len(missingGenes) == 0
		if selectable {
			matchStatus = "exact"
			matchColor = "green"
			if !matchedPanelSet[panelID] {
				matchedPanelSet[panelID] = true
				matchedPanelIDs = append(matchedPanelIDs, panelID)
			}
		}

		panelMatches = append(panelMatches, utils.H{
			"panelId":      panelID,
			"panelName":    panelName,
			"panelCode":    panelCode,
			"matchCount":   matchCount,
			"totalGenes":   len(panelGenes),
			"matchRate":    matchRate,
			"panelGenes":   panelGenes,
			"sampleGenes":  sampleGenes,
			"missingGenes": missingGenes,
			"extraGenes":   extraGenes,
			"matchStatus":  matchStatus,
			"matchColor":   matchColor,
			"selectable":   selectable,
		})
	}

	allSampleGenes := make([]string, 0, len(allSampleGeneSet))
	for _, gene := range allSampleGeneSet {
		allSampleGenes = append(allSampleGenes, gene)
	}
	sort.Strings(allSampleGenes)
	saveSamplePanelMatchCache(db, batchID, sampleCode, matchedPanelIDs, panelMatches, allSampleGenes)
	return matchedPanelIDs, panelMatches, rows.Err()
}

func getMissingCancerTypePanels(db *sql.DB, requiredPanelIDs []int, available map[int]bool) []utils.H {
	var missing []utils.H
	for _, panelID := range requiredPanelIDs {
		if available[panelID] {
			continue
		}
		panel := utils.H{"id": panelID}
		var panelName, panelCode string
		if err := db.QueryRow(`SELECT panel_name, panel_code FROM setting_panel WHERE id = ?`, panelID).Scan(&panelName, &panelCode); err == nil {
			panel["panelName"] = panelName
			panel["panelCode"] = panelCode
		}
		missing = append(missing, panel)
	}
	return missing
}

func autoMatchBatchSamplesByPanels(db *sql.DB, batchID int64) (map[string]int, error) {
	cancerTypes, err := getActiveCancerTypePanelInfos(db)
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`
		SELECT bs.sample_code,
			COALESCE(NULLIF(bs.cancer_type_id, 0), 0) AS batch_sample_cancer_type_id,
			COALESCE((
				SELECT NULLIF(s.cancer_type_id, 0)
				FROM detect_sample s
				WHERE s.sample_code = bs.sample_code
				ORDER BY CASE WHEN s.batch_id = bs.batch_id THEN 0 ELSE 1 END, s.id DESC
				LIMIT 1
			), 0) AS registered_cancer_type_id
		FROM detect_batch_sample bs
		WHERE bs.batch_id = ? AND bs.sample_code != 'H'
	`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	updated := make(map[string]int)
	for rows.Next() {
		var sampleCode string
		var batchSampleCancerTypeID, registeredCancerTypeID int
		if err := rows.Scan(&sampleCode, &batchSampleCancerTypeID, &registeredCancerTypeID); err != nil {
			continue
		}

		matchedPanelIDs, _, err := getSampleMatchedPanelsForBatch(db, int(batchID), sampleCode)
		if err != nil {
			log.Printf("autoMatchBatchSamplesByPanels: failed to match panels for sample %s: %v", sampleCode, err)
		}

		preferredCancerTypeID := registeredCancerTypeID
		if preferredCancerTypeID == 0 {
			preferredCancerTypeID = batchSampleCancerTypeID
		}
		bestMatch, matched := chooseCancerTypeForSample(cancerTypes, matchedPanelIDs, preferredCancerTypeID)
		if !matched {
			continue
		}

		_, err = db.Exec(`UPDATE detect_batch_sample SET cancer_type_id = ? WHERE batch_id = ? AND sample_code = ?`, bestMatch.ID, batchID, sampleCode)
		if err != nil {
			log.Printf("autoMatchBatchSamplesByPanels: failed to update detect_batch_sample for sample %s: %v", sampleCode, err)
			continue
		}

		// 已登记的样本癌种是业务主数据，Panel 只用于没有癌种时的兜底推断，不能反向覆盖。
		if registeredCancerTypeID == 0 {
			_, err = db.Exec(`UPDATE detect_sample SET cancer_type_id = ?, sample_updated_at = NOW() WHERE batch_id = ? AND sample_code = ?`, bestMatch.ID, batchID, sampleCode)
			if err != nil {
				log.Printf("autoMatchBatchSamplesByPanels: failed to sync inferred cancer type for sample %s: %v", sampleCode, err)
			}
		}
		updated[sampleCode] = bestMatch.ID
		log.Printf("autoMatchBatchSamplesByPanels: sample %s matched cancer type %s (%d), preferred=%d, panels=%v", sampleCode, bestMatch.Name, bestMatch.ID, preferredCancerTypeID, matchedPanelIDs)
	}
	return updated, rows.Err()
}

func chooseCancerTypeForSample(cancerTypes []cancerTypePanelInfo, matchedPanelIDs []int, preferredCancerTypeID int) (cancerTypePanelInfo, bool) {
	if preferredCancerTypeID > 0 {
		for _, cancerType := range cancerTypes {
			if cancerType.ID == preferredCancerTypeID {
				return cancerType, true
			}
		}
	}

	matchedPanelSet := panelIDSet(matchedPanelIDs)
	var candidates []cancerTypePanelInfo
	for _, cancerType := range cancerTypes {
		if panelSetCovers(cancerType.PanelIDs, matchedPanelSet) {
			candidates = append(candidates, cancerType)
		}
	}
	if len(candidates) == 0 {
		return cancerTypePanelInfo{}, false
	}
	sort.Slice(candidates, func(i, j int) bool {
		return len(candidates[i].PanelIDs) > len(candidates[j].PanelIDs)
	})
	return candidates[0], true
}

func HandleBatchUploadMultiple(c *app.RequestContext, db *sql.DB) {
	var uploaderID int
	var uploaderName string
	uploaderIDStr := c.PostForm("uploaderId")
	if uploaderIDStr != "" {
		var err error
		uploaderID, err = strconv.Atoi(uploaderIDStr)
		if err != nil {
			uploaderID = 0
			log.Printf("Invalid uploaderId: %v", err)
		}
	}
	if uploaderID == 0 {
		if userID, exists := c.Get("userID"); exists {
			uploaderID = userID.(int)
		}
	}
	if uploaderID > 0 {
		err := db.QueryRow("SELECT real_name FROM base_manage_user WHERE id = ?", uploaderID).Scan(&uploaderName)
		if err != nil {
			log.Printf("Failed to get uploader name: %v", err)
			uploaderName = ""
		}
	}

	testerIDStr := c.PostForm("testerId")
	var testerID int
	var testerName string
	if testerIDStr != "" {
		var err error
		testerID, err = strconv.Atoi(testerIDStr)
		if err == nil {
			err = db.QueryRow("SELECT real_name FROM base_manage_user WHERE id = ?", testerID).Scan(&testerName)
			if err != nil {
				log.Printf("Failed to get tester name: %v", err)
			}
		}
	}

	panelIDStr := c.PostForm("panelId")
	var panelID int
	if panelIDStr != "" {
		var err error
		panelID, err = strconv.Atoi(panelIDStr)
		if err != nil {
			log.Printf("Invalid panelId: %v", err)
			panelID = 0
		}
	}

	form, err := c.MultipartForm()
	if err != nil {
		log.Printf("Failed to parse multipart form: %v", err)
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "解析表单数据失败",
			Data:    nil,
		})
		return
	}

	files, ok := form.File["files"]
	if !ok || len(files) == 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请选择要上传的文件",
			Data:    nil,
		})
		return
	}

	uploadDir := "./uploads/batches"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Printf("Failed to create upload directory: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "创建上传目录失败",
			Data:    nil,
		})
		return
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

	batchCode, err := generateBatchCode(db)
	if err != nil {
		log.Printf("Failed to generate batch code: %v", err)
		tx.Rollback()
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "生成批次编号失败",
			Data:    nil,
		})
		return
	}

	uploadedGenes := make(map[string]bool)
	var panelMatchResult *PanelMatchResult

	batchExtraData := make(map[string]interface{})
	if panelID > 0 {
		batchExtraData["panelId"] = panelID
	}
	batchExtraDataJSON, _ := json.Marshal(batchExtraData)

	result, err := tx.Exec(`
		INSERT INTO detect_batch (batch_code, upload_token, sample_volume, batch_start_time, batch_stop_time, sample_count, status, uploader_id, uploader_name, tester_id, tester_name, median_data, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
	`, batchCode, "", "", nil, nil, 0, "pending", uploaderID, uploaderName, testerID, testerName, string(batchExtraDataJSON))
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
	batchID, err := result.LastInsertId()
	if err != nil {
		log.Printf("Failed to get batch ID: %v", err)
		tx.Rollback()
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	allPlatforms := make(map[string]bool)
	allSampleCodes := make(map[string]bool)
	uploadedFiles := []utils.H{}
	totalSamples := 0

	for _, fileHeader := range files {
		fileName := fmt.Sprintf("%d_%s", time.Now().UnixNano(), fileHeader.Filename)
		filePath := filepath.Join(uploadDir, fileName)

		srcFile, err := fileHeader.Open()
		if err != nil {
			log.Printf("Failed to open file: %v", err)
			tx.Rollback()
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code:    500,
				Success: false,
				Message: "打开文件失败",
				Data:    nil,
			})
			return
		}
		defer srcFile.Close()

		dstFile, err := os.Create(filePath)
		if err != nil {
			log.Printf("Failed to create file: %v", err)
			tx.Rollback()
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code:    500,
				Success: false,
				Message: "保存文件失败",
				Data:    nil,
			})
			return
		}
		defer dstFile.Close()

		if _, err := io.Copy(dstFile, srcFile); err != nil {
			log.Printf("Failed to copy file: %v", err)
			tx.Rollback()
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code:    500,
				Success: false,
				Message: "保存文件失败",
				Data:    nil,
			})
			return
		}

		srcFile.Seek(0, 0)
		csvReader, _ := parseCSVFileReader(srcFile)
		baseInfo, medianData, countData, protocolName, err := parseCSVFileWithProtocol(csvReader)
		if err != nil {
			log.Printf("Failed to parse CSV: %v", err)
			tx.Rollback()
			c.JSON(consts.StatusBadRequest, ApiResponse{
				Code:    400,
				Success: false,
				Message: fmt.Sprintf("解析文件 %s 失败: %v", fileHeader.Filename, err),
				Data:    nil,
			})
			return
		}

		// 从baseInfo中提取元数据
		batchStartTime := ""
		if v, ok := baseInfo["batchStartTime"].(string); ok {
			batchStartTime = v
		}
		batchStopTime := ""
		if v, ok := baseInfo["batchStopTime"].(string); ok {
			batchStopTime = v
		}
		instrumentSn := ""
		if v, ok := baseInfo["SN"].(string); ok {
			instrumentSn = v
		}
		sampleVolume := ""
		if v, ok := baseInfo["sampleVolume"].(string); ok {
			sampleVolume = v
		}

		if len(medianData) > 0 {
			for key := range medianData[0] {
				if key != "Sample" && key != "sample_code" && key != "location" && key != "Location" && key != "Total Events" {
					uploadedGenes[key] = true
				}
			}
		}

		platform := extractPlatformFromProtocolName(protocolName)
		allPlatforms[platform] = true

		fileResult, err := tx.Exec(`
			INSERT INTO detect_batch_file (batch_id, batch_code, file_name, file_path, platform, protocol_name, uploaded_by, uploaded_by_name, batch_start_time, batch_stop_time, instrument_sn, sample_volume)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, batchID, batchCode, fileHeader.Filename, filePath, platform, protocolName, uploaderID, uploaderName, batchStartTime, batchStopTime, instrumentSn, sampleVolume)
		if err != nil {
			log.Printf("Failed to insert batch file record: %v", err)
			tx.Rollback()
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code:    500,
				Success: false,
				Message: "服务器内部错误",
				Data:    nil,
			})
			return
		}
		fileID, _ := fileResult.LastInsertId()

		sampleCountMap := make(map[string]map[string]interface{})
		for _, count := range countData {
			if sampleCode, ok := count["Sample"].(string); ok {
				sampleCode = normalizeSampleCode(sampleCode)
				count["Sample"] = sampleCode
				sampleCountMap[sampleCode] = count
			}
		}

		for _, median := range medianData {
			if sampleCode, ok := median["Sample"].(string); ok && sampleCode != "" {
				sampleCode = normalizeSampleCode(sampleCode)
				median["Sample"] = sampleCode
				// H样本（对照水）也插入到platform_data表中，但不计入样本数量
				if sampleCode != "H" {
					allSampleCodes[sampleCode] = true
					totalSamples++
				}

				sampleMedianJSON, _ := json.Marshal(median)
				sampleCount := sampleCountMap[sampleCode]
				sampleCountJSON, _ := json.Marshal(sampleCount)

				_, err := tx.Exec(`
					INSERT INTO detect_batch_platform_data (batch_id, batch_code, batch_file_id, platform, sample_code, median_data, count_data)
					VALUES (?, ?, ?, ?, ?, ?, ?)
				`, batchID, batchCode, fileID, platform, sampleCode, string(sampleMedianJSON), string(sampleCountJSON))
				if err != nil {
					log.Printf("Failed to insert platform data: %v", err)
				}
			}
		}

		uploadedFiles = append(uploadedFiles, utils.H{
			"id":           fileID,
			"fileName":     fileHeader.Filename,
			"filePath":     filePath,
			"platform":     platform,
			"protocolName": protocolName,
		})
	}

	platformsJSON, _ := json.Marshal(allPlatforms)
	var nullVal interface{}              // nil
	nullJSON, _ := json.Marshal(nullVal) // 生成 "null"
	_, err = tx.Exec(`
		UPDATE detect_batch 
		SET platforms = ?, sample_count = ?, median_data = ?, count_data = ?, updated_at = NOW()
		WHERE id = ?
	`, string(platformsJSON), len(allSampleCodes), string(nullJSON), string(nullJSON), batchID)
	if err != nil {
		log.Printf("Failed to update batch: %v", err)
		tx.Rollback()
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	for sampleCode := range allSampleCodes {
		var patientID sql.NullInt64
		var patientName sql.NullString
		matchStatus := 0
		err = tx.QueryRow(`
			SELECT s.patient_id, COALESCE(p.name, '')
			FROM detect_sample s
			LEFT JOIN detect_patient p ON s.patient_id = p.id
			WHERE s.sample_code = ?`,
			sampleCode).Scan(&patientID, &patientName)
		if err == nil {
			matchStatus = 1
			_, err = tx.Exec(`
				UPDATE detect_sample SET batch_id = ?, sample_updated_at = NOW()
				WHERE sample_code = ?`,
				batchID, sampleCode)
			if err != nil {
				log.Printf("Failed to update detect_sample for %s: %v", sampleCode, err)
			}
		} else if err != sql.ErrNoRows {
			log.Printf("Failed to check detect_sample for %s: %v", sampleCode, err)
		}

		_, err = tx.Exec(`
			INSERT INTO detect_batch_sample (batch_id, batch_code, sample_code, patient_id, patient_name, match_status)
			VALUES (?, ?, ?, ?, ?, ?)`,
			batchID, batchCode, sampleCode, patientID, patientName, matchStatus)
		if err != nil {
			log.Printf("Failed to insert batch sample: %v", err)
		}
	}

	// H样本（对照水）已经在上面的循环中处理了
	// 因为我们在插入platform_data时已经处理了H样本
	// 但detect_batch_sample表需要单独插入H样本
	// 由于medianData是循环变量，我们需要单独查询H样本
	// 为简化处理，我们可以在上传时通过查询platform_data来获取H样本
	// 这里暂时不在此处插入H样本到detect_batch_sample表
	// 后端查询时会从detect_batch_platform_data中获取H样本

	if panelID > 0 {
		panelMatchResult, err = performPanelMatch(db, panelID, uploadedGenes)
		if err != nil {
			log.Printf("Failed to perform panel match: %v", err)
		}
	} else {
		panelMatchResult, err = autoDetectPanel(db, uploadedGenes)
		if err != nil {
			log.Printf("Failed to auto detect panel: %v", err)
		}
	}

	if panelMatchResult != nil {
		panelID = panelMatchResult.PanelID
	}

	batchExtraData = make(map[string]interface{})
	if panelID > 0 {
		batchExtraData["panelId"] = panelID
	}
	if panelMatchResult != nil {
		batchExtraData["panelMatch"] = panelMatchResult
	}
	geneList := make([]string, 0, len(uploadedGenes))
	for gene := range uploadedGenes {
		geneList = append(geneList, gene)
	}
	batchExtraData["uploadedGenes"] = geneList
	batchExtraDataJSON, _ = json.Marshal(batchExtraData)

	_, err = tx.Exec(`
		UPDATE detect_batch 
		SET median_data = ?, updated_at = NOW()
		WHERE id = ?
	`, string(batchExtraDataJSON), batchID)
	if err != nil {
		log.Printf("Failed to update batch with panel match: %v", err)
	}

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

	autoMatchedSamples, matchErr := autoMatchBatchSamplesByPanels(db, batchID)
	if matchErr != nil {
		log.Printf("Failed to auto match sample cancer types for batch %d: %v", batchID, matchErr)
	}

	for _, file := range uploadedFiles {
		if filePath, ok := file["filePath"].(string); ok && filePath != "" {
			if err := os.Remove(filePath); err != nil {
				log.Printf("Failed to delete report file: %v", err)
			} else {
				log.Printf("Successfully deleted temporary file: %s", filePath)
			}
		}
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "文件上传成功",
		Data: utils.H{
			"batchId":         batchID,
			"batchCode":       batchCode,
			"files":           uploadedFiles,
			"platforms":       allPlatforms,
			"totalSamples":    totalSamples,
			"distinctSamples": len(allSampleCodes),
			"autoMatched":     autoMatchedSamples,
		},
	})
}

func HandleBatchDetailMultiPlatform(c *app.RequestContext, db *sql.DB) {
	if err := ensureBatchSampleModelIDColumn(db); err != nil {
		log.Printf("Failed to ensure detect_batch_sample.model_id: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "批次模型字段初始化失败"})
		return
	}
	idParam := c.Param("id")
	var batchID int64
	var query string
	var args []interface{}

	if id, err := strconv.ParseInt(idParam, 10, 64); err == nil {
		batchID = id
		query = `SELECT id, batch_code, COALESCE(sample_volume, ''), batch_start_time, batch_stop_time, sample_count, status, COALESCE(uploader_id, 0), COALESCE(uploader_name, ''), submitter_id, submitter_name, instrument_sn, tester_id, tester_name, platforms, median_data, count_data, merged_data, created_at, updated_at FROM detect_batch WHERE id = ?`
		args = []interface{}{batchID}
	} else {
		var dbBatchID int
		err := db.QueryRow("SELECT id FROM detect_batch WHERE batch_code = ?", idParam).Scan(&dbBatchID)
		if err != nil {
			c.JSON(consts.StatusNotFound, ApiResponse{
				Code:    404,
				Success: false,
				Message: "批次不存在",
				Data:    nil,
			})
			return
		}
		batchID = int64(dbBatchID)
		query = `SELECT id, batch_code, COALESCE(sample_volume, ''), batch_start_time, batch_stop_time, sample_count, status, COALESCE(uploader_id, 0), COALESCE(uploader_name, ''), submitter_id, submitter_name, instrument_sn, tester_id, tester_name, platforms, median_data, count_data, merged_data, created_at, updated_at FROM detect_batch WHERE id = ?`
		args = []interface{}{batchID}
	}

	var batchCode string
	var sampleVolume sql.NullString
	var status string
	var uploaderName string
	var submitterID sql.NullInt32
	var submitterName sql.NullString
	var instrumentSn sql.NullString
	var testerID sql.NullInt32
	var testerName sql.NullString
	var platforms sql.NullString
	var medianData sql.NullString
	var countData sql.NullString
	var mergedData sql.NullString
	var uploaderID int
	var sampleCount int
	var batchStartTime, batchStopTime, createdAt, updatedAt sql.NullTime

	err := db.QueryRow(query, args...).Scan(
		&batchID, &batchCode, &sampleVolume, &batchStartTime, &batchStopTime, &sampleCount, &status,
		&uploaderID, &uploaderName, &submitterID, &submitterName, &instrumentSn,
		&testerID, &testerName, &platforms, &medianData, &countData, &mergedData, &createdAt, &updatedAt,
	)
	if err != nil {
		log.Printf("Failed to query batch: %v", err)
		c.JSON(consts.StatusNotFound, ApiResponse{
			Code:    404,
			Success: false,
			Message: "批次不存在",
			Data:    nil,
		})
		return
	}

	// 移除统一配置的检测类型匹配模块相关代码

	fileRows, err := db.Query(`
		SELECT id, file_name, platform, protocol_name, uploaded_by_name, created_at, batch_start_time, batch_stop_time, instrument_sn, sample_volume 
		FROM detect_batch_file 
		WHERE batch_id = ? 
		ORDER BY platform, created_at
	`, batchID)
	if err != nil {
		log.Printf("Failed to query batch files: %v", err)
	}
	defer fileRows.Close()

	var files []utils.H
	for fileRows.Next() {
		var id int
		var fileName, platform, protocolName, uploadedByName string
		var fileCreatedAt time.Time
		var batchStartTime, batchStopTime, instrumentSn, sampleVolume sql.NullString
		err := fileRows.Scan(&id, &fileName, &platform, &protocolName, &uploadedByName, &fileCreatedAt, &batchStartTime, &batchStopTime, &instrumentSn, &sampleVolume)
		if err != nil {
			continue
		}
		files = append(files, utils.H{
			"id":           id,
			"fileName":     fileName,
			"platform":     platform,
			"protocolName": protocolName,
			"uploadedBy":   uploadedByName,
			"createdAt":    fileCreatedAt.Format("2006-01-02 15:04:05"),
			"batchStartTime": func() string {
				if batchStartTime.Valid {
					return batchStartTime.String
				} else {
					return "-"
				}
			}(),
			"batchStopTime": func() string {
				if batchStopTime.Valid {
					return batchStopTime.String
				} else {
					return "-"
				}
			}(),
			"instrumentSn": func() string {
				if instrumentSn.Valid {
					return instrumentSn.String
				} else {
					return "-"
				}
			}(),
			"sampleVolume": func() string {
				if sampleVolume.Valid {
					return sampleVolume.String
				} else {
					return "-"
				}
			}(),
		})
	}

	platformDataRows, err := db.Query(`
		SELECT id, batch_file_id, platform, sample_code, median_data, count_data
		FROM detect_batch_platform_data
		WHERE batch_id = ?
		ORDER BY sample_code, platform
	`, batchID)
	if err != nil {
		log.Printf("Failed to query platform data: %v", err)
	}
	defer platformDataRows.Close()

	sampleData := make(map[string]map[string]utils.H)
	allPlatformsMap := make(map[string]bool)
	for platformDataRows.Next() {
		var id, fileID int
		var platform, sampleCode string
		var medianJSON sql.NullString
		var countJSON sql.NullString
		err := platformDataRows.Scan(&id, &fileID, &platform, &sampleCode, &medianJSON, &countJSON)
		if err != nil {
			continue
		}

		var median, count map[string]interface{}
		if medianJSON.Valid && medianJSON.String != "" {
			json.Unmarshal([]byte(medianJSON.String), &median)
		}
		if countJSON.Valid && countJSON.String != "" {
			json.Unmarshal([]byte(countJSON.String), &count)
		}

		if sampleData[sampleCode] == nil {
			sampleData[sampleCode] = make(map[string]utils.H)
		}
		sampleData[sampleCode][platform] = utils.H{
			"id":     id,
			"fileId": fileID,
			"median": median,
			"count":  count,
		}
		allPlatformsMap[platform] = true
	}
	hasPlatformData := len(sampleData) > 0

	if _, err := db.Exec(`
		UPDATE detect_batch_sample bs
		JOIN detect_sample s ON s.sample_code = bs.sample_code
		LEFT JOIN detect_patient p ON p.id = s.patient_id
		SET bs.patient_id = s.patient_id,
			bs.patient_name = COALESCE(NULLIF(bs.patient_name, ''), p.name),
			bs.match_status = 1,
			bs.updated_at = NOW()
		WHERE bs.batch_id = ?
			AND bs.sample_code != 'H'
			AND (bs.match_status = 0 OR bs.patient_id IS NULL OR bs.patient_id = 0)
			AND s.patient_id IS NOT NULL
	`, batchID); err != nil {
		log.Printf("Failed to repair multi-platform batch sample matches for batch %d: %v", batchID, err)
	}

	// 从 detect_batch_sample 表查询样本信息，优先使用 bs.cancer_type_id；为 0 时回退到已建档样本的检测类型。
	sampleRows, err := db.Query(`
		SELECT bs.sample_code,
			COALESCE(NULLIF(bs.patient_id, 0), s.patient_id) as patient_id,
			COALESCE(p.patient_code, sp.patient_code, '') as patient_code,
			COALESCE(NULLIF(bs.patient_name, ''), p.name, sp.name, '') as patient_name,
			CASE WHEN bs.match_status = 1 OR s.patient_id IS NOT NULL THEN 1 ELSE 0 END as match_status,
			COALESCE(NULLIF(bs.cancer_type_id, 0), NULLIF(s.cancer_type_id, 0), 0) as cancer_type_id,
			COALESCE(NULLIF(bs.model_id, 0), NULLIF(s.model_id, 0), 0) as model_id,
			ct.name as cancer_type_name,
			ct.panel_ids
		FROM detect_batch_sample bs
		LEFT JOIN detect_patient p ON bs.patient_id = p.id
		LEFT JOIN detect_sample s ON s.sample_code = bs.sample_code
		LEFT JOIN detect_patient sp ON s.patient_id = sp.id
		LEFT JOIN setting_cancer_type ct ON COALESCE(NULLIF(bs.cancer_type_id, 0), NULLIF(s.cancer_type_id, 0), 0) = ct.id
		WHERE bs.batch_id = ? AND bs.sample_code != 'H'
		ORDER BY bs.sample_code
	`, batchID)
	if err != nil {
		log.Printf("Failed to query samples for panel matching: %v", err)
		sampleRows = nil
	}
	if sampleRows != nil {
		defer sampleRows.Close()
	}

	// 调试日志：记录查询结果
	log.Printf("SamplePanelMatching: Query executed, sampleRows is nil: %v", sampleRows == nil)

	panelMatchCache, err := loadSamplePanelMatchCacheForBatch(db, int(batchID))
	if err != nil {
		log.Printf("SamplePanelMatching: failed to load panel match cache for batch %d: %v", batchID, err)
		panelMatchCache = map[string]samplePanelMatchCache{}
	}
	panelMatchCacheComplete := isSamplePanelMatchCacheComplete(db, int(batchID))

	// 收集样本的基因信息和平台信息用于Panel匹配
	sampleGenesMap := make(map[string][]string)
	geneSetMap := make(map[string]map[string]bool)                 // geneSetMap[sampleCode][gene] = exists
	geneSetMapLower := make(map[string]map[string]bool)            // 用于不区分大小写的匹配
	samplePlatformsMap := make(map[string][]string)                // samplePlatformsMap[sampleCode] = ["V5", "V8"]
	samplePlatformGenesMap := make(map[string]map[string][]string) // samplePlatformGenesMap[sampleCode][platform] = genes

	if panelMatchCacheComplete {
		for sampleCode, cached := range panelMatchCache {
			sampleGenesMap[sampleCode] = cached.SampleGenes
			geneSetMap[sampleCode] = make(map[string]bool)
			geneSetMapLower[sampleCode] = make(map[string]bool)
			for _, gene := range cached.SampleGenes {
				geneSetMap[sampleCode][gene] = true
				geneSetMapLower[sampleCode][strings.ToLower(gene)] = true
			}
		}
		log.Printf("SamplePanelMatching: Using cached panel matches for batch %d (%d samples)", batchID, len(panelMatchCache))
	} else {
		for sampleCode, platformItemMap := range sampleData {
			if samplePlatformGenesMap[sampleCode] == nil {
				samplePlatformGenesMap[sampleCode] = make(map[string][]string)
			}
			for platform, platformItem := range platformItemMap {
				if median, ok := platformItem["median"].(map[string]interface{}); ok {
					// 初始化基因集合
					if _, exists := geneSetMap[sampleCode]; !exists {
						geneSetMap[sampleCode] = make(map[string]bool)
						geneSetMapLower[sampleCode] = make(map[string]bool)
					}

					// 初始化平台列表
					if _, exists := samplePlatformsMap[sampleCode]; !exists {
						samplePlatformsMap[sampleCode] = make([]string, 0)
					}
					samplePlatformsMap[sampleCode] = append(samplePlatformsMap[sampleCode], platform)

					// 收集当前平台的基因
					var platformGenes []string
					for key := range median {
						if key != "Sample" && key != "sample_code" && key != "location" && key != "Location" && key != "Total Events" {
							// 自动判断基因名称是 gene_name 还是 gene_symbol，转换为统一的 gene_symbol
							geneSymbol, _ := getGeneSymbolByAnyName(db, key)
							geneSetMap[sampleCode][geneSymbol] = true
							geneSetMapLower[sampleCode][strings.ToLower(geneSymbol)] = true
							platformGenes = append(platformGenes, geneSymbol)
						}
					}
					samplePlatformGenesMap[sampleCode][platform] = platformGenes
				}
			}
		}

		// 转换为切片并排序
		for sampleCode, geneSet := range geneSetMap {
			var genes []string
			for gene := range geneSet {
				genes = append(genes, gene)
			}
			sort.Strings(genes)
			sampleGenesMap[sampleCode] = genes

			log.Printf("SamplePanelMatching: Sample %s collected %d genes from platforms %v: %v", sampleCode, len(genes), samplePlatformsMap[sampleCode], genes)
		}
	}

	var samples []utils.H
	var samplePanelMatches []utils.H

	// 调试日志：记录样本数量
	if sampleRows != nil && panelMatchCacheComplete {
		rowCount := 0
		for sampleRows.Next() {
			rowCount++
			var sampleCode string
			var patientName sql.NullString
			var patientID sql.NullInt32
			var patientCode string
			var matchStatus int
			var cancerTypeID, modelID sql.NullInt32
			var cancerTypeName, panelIDsStr sql.NullString
			if err := sampleRows.Scan(&sampleCode, &patientID, &patientCode, &patientName, &matchStatus, &cancerTypeID, &modelID, &cancerTypeName, &panelIDsStr); err != nil {
				log.Printf("Error scanning cached sample row: %v", err)
				continue
			}

			patientIDStr := ""
			if patientID.Valid {
				patientIDStr = fmt.Sprintf("%d", patientID.Int32)
			}
			patientNameStr := ""
			if patientName.Valid {
				patientNameStr = patientName.String
			}

			cached := panelMatchCache[sampleCode]
			sampleGenes := cached.SampleGenes
			panelMatches := cached.PanelMatches
			hasExactMatch := false
			for _, pm := range panelMatches {
				if status, ok := pm["matchStatus"].(string); ok && status == "exact" {
					hasExactMatch = true
					break
				}
			}

			samplePanelMatches = append(samplePanelMatches, utils.H{
				"sampleCode":       sampleCode,
				"patientId":        patientIDStr,
				"patientCode":      patientCode,
				"patientName":      patientNameStr,
				"matchStatus":      matchStatus,
				"cancerTypeId":     cancerTypeID.Int32,
				"cancerTypeName":   cancerTypeName.String,
				"modelId":          modelID.Int32,
				"sampleGenes":      sampleGenes,
				"panelMatches":     panelMatches,
				"hasMatchingPanel": len(panelMatches) > 0,
				"hasExactMatch":    hasExactMatch,
			})

			samples = append(samples, utils.H{
				"sampleCode":     sampleCode,
				"patientId":      patientIDStr,
				"patientCode":    patientCode,
				"patientName":    patientNameStr,
				"matchStatus":    matchStatus,
				"platformData":   sampleData[sampleCode],
				"cancerTypeId":   cancerTypeID.Int32,
				"cancerTypeName": cancerTypeName.String,
				"modelId":        modelID.Int32,
				"panelMatches":   panelMatches,
				"hasExactMatch":  hasExactMatch,
			})
		}
		if err := sampleRows.Err(); err != nil {
			log.Printf("SamplePanelMatching: error iterating cached sample rows: %v", err)
		}
		log.Printf("SamplePanelMatching: Returned cached panel matches for %d samples, samplePanelMatches count: %d", rowCount, len(samplePanelMatches))
	} else if sampleRows != nil {
		log.Printf("SamplePanelMatching: Starting to process samples, sampleRows is nil: %v", sampleRows == nil)
		rowCount := 0
		for sampleRows.Next() {
			rowCount++
			var sampleCode string
			var patientName sql.NullString
			var patientID sql.NullInt32
			var patientCode string
			var matchStatus int
			var cancerTypeID, modelID sql.NullInt32
			var cancerTypeName, panelIDsStr sql.NullString
			err := sampleRows.Scan(&sampleCode, &patientID, &patientCode, &patientName, &matchStatus, &cancerTypeID, &modelID, &cancerTypeName, &panelIDsStr)
			if err != nil {
				log.Printf("Error scanning sample row: %v", err)
				continue
			}

			patientIDStr := ""
			if patientID.Valid {
				patientIDStr = fmt.Sprintf("%d", patientID.Int32)
			}
			patientNameStr := ""
			if patientName.Valid {
				patientNameStr = patientName.String
			}

			// 获取该样本的基因列表
			sampleGenes := sampleGenesMap[sampleCode]
			sampleGenesMapSet := geneSetMap[sampleCode]
			sampleGenesMapSetLower := geneSetMapLower[sampleCode]

			// 获取该样本的平台列表
			samplePlatforms := samplePlatformsMap[sampleCode]

			// 通过平台名称查找对应的Panel，并获取关联的癌种
			var panelMatches []utils.H
			if cached, ok := panelMatchCache[sampleCode]; ok && len(cached.SampleGenes) > 0 {
				panelMatches = cached.PanelMatches
				sampleGenes = cached.SampleGenes
			} else if len(samplePlatforms) > 0 {
				for _, platform := range samplePlatforms {
					platform = strings.TrimSpace(platform)
					if platform == "" {
						continue
					}

					// 通过panel_code查找Panel
					var panelID int
					var panelName, geneIDsStr string
					err := db.QueryRow(`SELECT id, panel_name, gene_ids FROM setting_panel WHERE panel_code = ? AND is_active = 1`, platform).Scan(&panelID, &panelName, &geneIDsStr)
					if err != nil {
						log.Printf("SamplePanelMatching: Failed to query panel by code %s: %v", platform, err)
						continue
					}
					log.Printf("SamplePanelMatching: Found panel %d (%s) by code %s with gene_ids: '%s'", panelID, panelName, platform, geneIDsStr)

					// 查找关联的癌种
					var cancerTypeNames []string
					ctRows, err := db.Query(`
						SELECT ct.name FROM setting_cancer_type ct 
						WHERE ct.panel_ids LIKE ? AND ct.is_active = 1`,
						"%"+fmt.Sprintf("%%%d%%", panelID))
					if err == nil {
						defer ctRows.Close()
						for ctRows.Next() {
							var name string
							ctRows.Scan(&name)
							cancerTypeNames = append(cancerTypeNames, name)
						}
					}
					log.Printf("SamplePanelMatching: Panel %s associated with cancer types: %v", panelName, cancerTypeNames)

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

					log.Printf("SamplePanelMatching: Panel %s genes: %v", panelName, panelGenes)

					// 计算匹配度（支持不区分大小写）
					matchCount := 0
					for _, gene := range panelGenes {
						if sampleGenesMapSet[gene] || sampleGenesMapSetLower[strings.ToLower(gene)] {
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
						if !sampleGenesMapSet[gene] && !sampleGenesMapSetLower[strings.ToLower(gene)] {
							missingGenes = append(missingGenes, gene)
						}
					}

					// 计算额外基因：使用该平台对应的基因列表，而不是所有平台合并的基因
					extraGenes := []string{}
					panelGenesMap := make(map[string]bool, len(panelGenes))
					panelGenesMapLower := make(map[string]bool, len(panelGenes))
					for _, gene := range panelGenes {
						panelGenesMap[gene] = true
						panelGenesMapLower[strings.ToLower(gene)] = true
					}

					// 获取该平台对应的基因列表
					var platformGenes []string
					if genes, ok := samplePlatformGenesMap[sampleCode][platform]; ok {
						platformGenes = genes
					} else {
						// 如果找不到该平台的基因，使用完整基因列表作为后备
						platformGenes = sampleGenes
					}

					for _, gene := range platformGenes {
						if !panelGenesMap[gene] && !panelGenesMapLower[strings.ToLower(gene)] {
							extraGenes = append(extraGenes, gene)
						}
					}

					log.Printf("SamplePanelMatching: Panel %s match - matched: %d/%d, missing: %v, extra: %v",
						panelName, matchCount, totalGenes, missingGenes, extraGenes)

					// 确定匹配状态
					// 当Panel所有基因都在样本中(missingGenes==0)，即为完全匹配
					// 额外基因(extraGenes)来自其他Panel，不影响本Panel的匹配完整性
					matchStatusStr := "insufficient"
					matchColor := "red"
					if len(missingGenes) == 0 {
						matchStatusStr = "exact"
						matchColor = "green"
					}

					panelMatches = append(panelMatches, utils.H{
						"panelId":      panelID,
						"panelName":    panelName,
						"panelCode":    platform,
						"cancerTypes":  cancerTypeNames,
						"matchCount":   matchCount,
						"totalGenes":   totalGenes,
						"matchRate":    matchRate,
						"panelGenes":   panelGenes,
						"sampleGenes":  sampleGenes,
						"missingGenes": missingGenes,
						"extraGenes":   extraGenes,
						"matchStatus":  matchStatusStr,
						"matchColor":   matchColor,
						"selectable":   len(missingGenes) == 0,
					})
				}
				matchedPanelIDs := make([]int, 0)
				for _, panelMatch := range panelMatches {
					if status, _ := panelMatch["matchStatus"].(string); status != "exact" {
						continue
					}
					switch value := panelMatch["panelId"].(type) {
					case int:
						matchedPanelIDs = append(matchedPanelIDs, value)
					case int32:
						matchedPanelIDs = append(matchedPanelIDs, int(value))
					case int64:
						matchedPanelIDs = append(matchedPanelIDs, int(value))
					case float64:
						matchedPanelIDs = append(matchedPanelIDs, int(value))
					}
				}
				saveSamplePanelMatchCache(db, int(batchID), sampleCode, matchedPanelIDs, panelMatches, sampleGenes)
			}

			hasExactMatch := false
			for _, pm := range panelMatches {
				if status, ok := pm["matchStatus"].(string); ok && status == "exact" {
					hasExactMatch = true
					break
				}
			}

			// 添加样本Panel匹配结果
			samplePanelMatches = append(samplePanelMatches, utils.H{
				"sampleCode":       sampleCode,
				"cancerTypeId":     cancerTypeID.Int32,
				"cancerTypeName":   cancerTypeName.String,
				"modelId":          modelID.Int32,
				"sampleGenes":      sampleGenes,
				"panelMatches":     panelMatches,
				"hasMatchingPanel": len(panelMatches) > 0,
				"hasExactMatch":    hasExactMatch,
			})

			sample := utils.H{
				"sampleCode":     sampleCode,
				"patientId":      patientIDStr,
				"patientCode":    patientCode,
				"patientName":    patientNameStr,
				"matchStatus":    matchStatus,
				"platformData":   sampleData[sampleCode],
				"cancerTypeId":   cancerTypeID.Int32,
				"cancerTypeName": cancerTypeName.String,
				"modelId":        modelID.Int32,
				"panelMatches":   panelMatches,
				"hasExactMatch":  hasExactMatch,
			}
			samples = append(samples, sample)
		}
		log.Printf("SamplePanelMatching: Processed %d samples, samplePanelMatches count: %d", rowCount, len(samplePanelMatches))

		if hasPlatformData && !panelMatchCacheComplete {
			// 自动匹配检测类型（逐个样本）
			// 第一步：查询所有Panel信息，缓存结果
			type PanelInfo struct {
				ID    int
				Name  string
				Genes []string
			}
			var allPanels []PanelInfo

			panelRows, err := db.Query(`SELECT id, panel_name, gene_ids FROM setting_panel WHERE is_active = 1`)
			if err == nil {
				defer panelRows.Close()
				for panelRows.Next() {
					var id int
					var name, geneIDsStr string
					if err := panelRows.Scan(&id, &name, &geneIDsStr); err == nil {
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
				}
			}

			// 第二步：查询所有检测类型及其关联的Panel
			type CancerTypeInfo struct {
				ID       int
				Name     string
				PanelIDs []int
			}
			var cancerTypes []CancerTypeInfo

			cancerTypeRows, err := db.Query(`SELECT id, name, panel_ids FROM setting_cancer_type WHERE is_active = 1`)
			if err == nil {
				defer cancerTypeRows.Close()
				for cancerTypeRows.Next() {
					var id int
					var name, panelIDsStr string
					if err := cancerTypeRows.Scan(&id, &name, &panelIDsStr); err == nil {
						if panelIDsStr != "" {
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
					}
				}
			}

			// 第三步：逐个样本匹配检测类型
			for _, sample := range samplePanelMatches {
				sampleCode, _ := sample["sampleCode"].(string)
				currentCancerTypeName, _ := sample["cancerTypeName"].(string)

				// 如果已经有检测类型了，跳过
				if currentCancerTypeName != "" {
					continue
				}

				sampleGenes, _ := sample["sampleGenes"].([]string)
				if len(sampleGenes) == 0 {
					continue
				}

				// 构建不区分大小写的样本基因映射
				sampleGenesMap := make(map[string]bool, len(sampleGenes))
				sampleGenesLower := make(map[string]bool, len(sampleGenes))
				for _, gene := range sampleGenes {
					sampleGenesMap[gene] = true
					sampleGenesLower[strings.ToLower(gene)] = true
				}

				// 第一步：用基因匹配Panel（找出所有完全匹配的Panel）
				var matchedPanelIDs []int
				for _, panel := range allPanels {
					if len(panel.Genes) == 0 {
						continue
					}
					// 检查Panel的所有基因是否都在样本基因中
					panelMatched := true
					for _, gene := range panel.Genes {
						if !sampleGenesMap[gene] && !sampleGenesLower[strings.ToLower(gene)] {
							panelMatched = false
							break
						}
					}
					if panelMatched {
						matchedPanelIDs = append(matchedPanelIDs, panel.ID)
					}
				}

				if len(matchedPanelIDs) == 0 {
					continue
				}

				// 第二步：用匹配到的Panel去匹配检测类型
				var matchedCancerTypes []struct {
					ID         int
					Name       string
					PanelCount int
				}
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
						matchedCancerTypes = append(matchedCancerTypes, struct {
							ID         int
							Name       string
							PanelCount int
						}{
							ID:         ct.ID,
							Name:       ct.Name,
							PanelCount: len(ct.PanelIDs),
						})
					}
				}

				// 如果找到匹配的检测类型，选择Panel数量最少的
				if len(matchedCancerTypes) > 0 {
					sort.Slice(matchedCancerTypes, func(i, j int) bool {
						return matchedCancerTypes[i].PanelCount > matchedCancerTypes[j].PanelCount
					})
					bestMatch := matchedCancerTypes[0]

					// 更新 detect_sample 表
					result, err := db.Exec(`UPDATE detect_sample SET cancer_type_id = ?, sample_updated_at = NOW() WHERE batch_id = ? AND sample_code = ?`,
						bestMatch.ID, batchID, sampleCode)
					if err != nil {
						// 如果通过 batch_id 更新失败，尝试只通过 sample_code 更新
						result, err = db.Exec(`UPDATE detect_sample SET cancer_type_id = ?, batch_id = ?, sample_updated_at = NOW() WHERE sample_code = ?`,
							bestMatch.ID, batchID, sampleCode)
					}
					if err != nil {
						log.Printf("SamplePanelMatching: Failed to update cancer type in detect_sample for sample %s: %v", sampleCode, err)
					} else {
						rowsAffected, _ := result.RowsAffected()
						log.Printf("SamplePanelMatching: Updated detect_sample for %s, rows affected: %d", sampleCode, rowsAffected)
					}

					// 更新 detect_batch_sample 表的 cancer_type_id
					_, err = db.Exec(`UPDATE detect_batch_sample SET cancer_type_id = ? WHERE batch_id = ? AND sample_code = ?`,
						bestMatch.ID, batchID, sampleCode)
					if err != nil {
						log.Printf("SamplePanelMatching: Failed to update cancer_type_id in detect_batch_sample for sample %s: %v", sampleCode, err)
					} else {
						log.Printf("SamplePanelMatching: Auto-matched sample %s to cancer type: %s (ID: %d) successfully saved",
							sampleCode, bestMatch.Name, bestMatch.ID)
					}

					// 更新返回数据中的cancerTypeName和cancerTypeId
					sample["cancerTypeName"] = bestMatch.Name
					sample["cancerTypeId"] = bestMatch.ID
					for i, s := range samples {
						if sc, ok := s["sampleCode"].(string); ok && sc == sampleCode {
							samples[i]["cancerTypeName"] = bestMatch.Name
							samples[i]["cancerTypeId"] = bestMatch.ID
							break
						}
					}
				}
			}
		}
	}

	if len(samples) == 0 {
		submittedRows, err := db.Query(`
			SELECT s.id, s.sample_code, s.patient_id, COALESCE(p.patient_code, '') as patient_code, COALESCE(p.name, '') as patient_name,
				COALESCE(s.match_status, ''), COALESCE(s.cancer_type_id, 0), COALESCE(s.model_id, 0), COALESCE(ct.name, '') as cancer_type_name
			FROM detect_sample s
			LEFT JOIN detect_patient p ON s.patient_id = p.id
			LEFT JOIN setting_cancer_type ct ON s.cancer_type_id = ct.id
			WHERE s.batch_id = ?
				AND s.sample_code != 'H'
				AND s.result_data IS NOT NULL
				AND TRIM(s.result_data) NOT IN ('', '{}', 'null')
			ORDER BY s.sample_code
		`, batchID)
		if err != nil {
			log.Printf("HandleBatchDetailMultiPlatform: failed to query submitted samples for batch %d: %v", batchID, err)
		} else {
			defer submittedRows.Close()
			for submittedRows.Next() {
				var sampleID, patientID, cancerTypeID, modelID int
				var sampleCode, patientCode, patientName, sampleMatchStatus, cancerTypeName string
				if err := submittedRows.Scan(&sampleID, &sampleCode, &patientID, &patientCode, &patientName, &sampleMatchStatus, &cancerTypeID, &modelID, &cancerTypeName); err != nil {
					log.Printf("HandleBatchDetailMultiPlatform: failed to scan submitted sample: %v", err)
					continue
				}

				var panelMatches []utils.H
				if hasPlatformData {
					_, panelMatches, err = getSampleMatchedPanelsForBatch(db, int(batchID), sampleCode)
					if err != nil {
						log.Printf("HandleBatchDetailMultiPlatform: failed to build panel matches for submitted sample %s: %v", sampleCode, err)
					}
				}
				hasExactMatch := false
				for _, pm := range panelMatches {
					if status, ok := pm["matchStatus"].(string); ok && status == "exact" {
						hasExactMatch = true
						break
					}
				}

				sampleGenes := sampleGenesMap[sampleCode]
				samplePanelMatches = append(samplePanelMatches, utils.H{
					"sampleCode":       sampleCode,
					"cancerTypeId":     cancerTypeID,
					"cancerTypeName":   cancerTypeName,
					"modelId":          modelID,
					"sampleGenes":      sampleGenes,
					"panelMatches":     panelMatches,
					"hasMatchingPanel": len(panelMatches) > 0,
					"hasExactMatch":    hasExactMatch,
				})

				sample := utils.H{
					"id":                sampleID,
					"sampleId":          sampleID,
					"sampleCode":        sampleCode,
					"patientId":         fmt.Sprintf("%d", patientID),
					"patientCode":       patientCode,
					"patientName":       patientName,
					"sampleMatchStatus": sampleMatchStatus,
					"matchStatus":       1,
					"platformData":      sampleData[sampleCode],
					"cancerTypeId":      cancerTypeID,
					"cancerTypeName":    cancerTypeName,
					"modelId":           modelID,
					"panelMatches":      panelMatches,
					"hasExactMatch":     hasExactMatch,
				}
				samples = append(samples, sample)
			}
			if err := submittedRows.Err(); err != nil {
				log.Printf("HandleBatchDetailMultiPlatform: error iterating submitted samples: %v", err)
			}
		}
	}

	// 添加H样本（对照水）到样本列表中
	// H样本在detect_batch_platform_data表中，但不在detect_batch_sample表中
	if hData, ok := sampleData["H"]; ok {
		hSample := utils.H{
			"sampleCode":   "H",
			"patientId":    "",
			"patientName":  "",
			"matchStatus":  0,
			"platformData": hData,
		}
		samples = append(samples, hSample)
	}

	var mergedDataObj []utils.H
	if mergedData.Valid && mergedData.String != "" {
		json.Unmarshal([]byte(mergedData.String), &mergedDataObj)
	}

	sampleVolumeStr := ""
	if sampleVolume.Valid {
		sampleVolumeStr = sampleVolume.String
	}
	submitterIDInt := 0
	if submitterID.Valid {
		submitterIDInt = int(submitterID.Int32)
	}
	submitterNameStr := ""
	if submitterName.Valid {
		submitterNameStr = submitterName.String
	}
	instrumentSnStr := ""
	if instrumentSn.Valid {
		instrumentSnStr = instrumentSn.String
	}
	testerIDInt := 0
	if testerID.Valid {
		testerIDInt = int(testerID.Int32)
	}
	testerNameStr := ""
	if testerName.Valid {
		testerNameStr = testerName.String
	}

	attachBatchReportIDs(db, int(batchID), samples, samplePanelMatches)
	responseData := utils.H{
		"batch": utils.H{
			"id":             batchID,
			"batchCode":      batchCode,
			"sampleVolume":   sampleVolumeStr,
			"batchStartTime": formatNullTime(batchStartTime),
			"batchStopTime":  formatNullTime(batchStopTime),
			"sampleCount":    sampleCount,
			"status":         status,
			"uploaderId":     uploaderID,
			"uploaderName":   uploaderName,
			"submitterId":    submitterIDInt,
			"submitterName":  submitterNameStr,
			"instrumentSn":   instrumentSnStr,
			"testerId":       testerIDInt,
			"testerName":     testerNameStr,
			"createdAt":      formatNullTime(createdAt),
			"updatedAt":      formatNullTime(updatedAt),
		},
		"files":              files,
		"platforms":          allPlatformsMap,
		"samples":            samples,
		"mergedData":         mergedDataObj,
		"samplePanelMatches": samplePanelMatches,
	}

	// 移除panel和panelMatch字段，因为不需要统一配置检测类型

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取成功",
		Data:    responseData,
	})
}

func HandleMergeSampleData(c *app.RequestContext, db *sql.DB) {
	var req struct {
		BatchID    int                               `json:"batchId" binding:"required"`
		SampleData map[string]map[string]interface{} `json:"sampleData" binding:"required"`
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

	mergedDataJSON, err := json.Marshal(req.SampleData)
	if err != nil {
		log.Printf("Failed to marshal merged data: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "数据格式错误",
			Data:    nil,
		})
		return
	}

	_, err = db.Exec(`UPDATE detect_batch SET merged_data = ?, updated_at = NOW() WHERE id = ?`, string(mergedDataJSON), req.BatchID)
	if err != nil {
		log.Printf("Failed to update merged data: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "保存数据失败",
			Data:    nil,
		})
		return
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "数据保存成功",
		Data:    nil,
	})
}

func HandleGetTesters(c *app.RequestContext, db *sql.DB) {
	rows, err := db.Query(`
		SELECT id, real_name, username 
		FROM base_manage_user 
		WHERE status = 1 
		ORDER BY real_name
	`)
	if err != nil {
		log.Printf("Failed to query testers: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}
	defer rows.Close()

	var testers []utils.H
	for rows.Next() {
		var id int
		var realName, username string
		err := rows.Scan(&id, &realName, &username)
		if err != nil {
			continue
		}
		testers = append(testers, utils.H{
			"id":       id,
			"name":     realName,
			"username": username,
		})
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取成功",
		Data:    utils.H{"list": testers},
	})
}

func performPanelMatch(db *sql.DB, panelID int, uploadedGenes map[string]bool) (*PanelMatchResult, error) {
	if panelID == 0 {
		return nil, nil
	}

	var panelName, panelCode, geneIDsStr string
	err := db.QueryRow("SELECT panel_name, panel_code, gene_ids FROM setting_panel WHERE id = ?", panelID).Scan(&panelName, &panelCode, &geneIDsStr)
	if err != nil {
		log.Printf("Failed to get panel info: %v", err)
		return nil, err
	}

	panelGenes := make(map[string]bool)
	if geneIDsStr != "" {
		geneIDList := strings.Split(geneIDsStr, ",")
		placeholders := make([]string, len(geneIDList))
		args := make([]interface{}, len(geneIDList))
		for i, id := range geneIDList {
			placeholders[i] = "?"
			args[i] = id
		}
		query := fmt.Sprintf("SELECT gene_symbol FROM setting_gene WHERE id IN (%s)", strings.Join(placeholders, ","))
		rows, err := db.Query(query, args...)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var geneSymbol string
				if err := rows.Scan(&geneSymbol); err == nil {
					panelGenes[geneSymbol] = true
				}
			}
		}
	}

	var matchedGenes, missingGenes, extraGenes []string
	for gene := range panelGenes {
		if uploadedGenes[gene] {
			matchedGenes = append(matchedGenes, gene)
		} else {
			missingGenes = append(missingGenes, gene)
		}
	}
	for gene := range uploadedGenes {
		if !panelGenes[gene] {
			extraGenes = append(extraGenes, gene)
		}
	}

	matchRate := 0.0
	if len(panelGenes) > 0 {
		matchRate = float64(len(matchedGenes)) / float64(len(panelGenes))
	}

	matchStatus := "insufficient"
	if len(missingGenes) == 0 && len(extraGenes) == 0 {
		matchStatus = "exact"
	} else if len(missingGenes) == 0 {
		matchStatus = "subset"
	}

	return &PanelMatchResult{
		PanelID:      panelID,
		PanelName:    panelName,
		PanelCode:    panelCode,
		MatchRate:    matchRate,
		MatchedGenes: matchedGenes,
		MissingGenes: missingGenes,
		ExtraGenes:   extraGenes,
		MatchStatus:  matchStatus,
	}, nil
}

func autoDetectPanel(db *sql.DB, uploadedGenes map[string]bool) (*PanelMatchResult, error) {
	if len(uploadedGenes) == 0 {
		return nil, fmt.Errorf("没有检测到基因数据")
	}

	rows, err := db.Query(`SELECT id, panel_name, panel_code, gene_ids FROM setting_panel WHERE is_active = 1`)
	if err != nil {
		log.Printf("autoDetectPanel: Failed to query panels: %v", err)
		return nil, err
	}
	defer rows.Close()

	var bestMatch *PanelMatchResult
	bestMatchRate := 0.0
	bestMatchLen := 0

	for rows.Next() {
		var panelID int
		var panelName, panelCode, geneIDsStr string
		if err := rows.Scan(&panelID, &panelName, &panelCode, &geneIDsStr); err != nil {
			continue
		}

		panelGenes := make(map[string]bool)

		if geneIDsStr != "" {
			geneIDList := strings.Split(geneIDsStr, ",")
			placeholders := make([]string, len(geneIDList))
			args := make([]interface{}, len(geneIDList))
			for i, id := range geneIDList {
				placeholders[i] = "?"
				args[i] = strings.TrimSpace(id)
			}
			query := fmt.Sprintf("SELECT gene_symbol FROM setting_gene WHERE id IN (%s)", strings.Join(placeholders, ","))
			geneRows, err := db.Query(query, args...)
			if err == nil {
				for geneRows.Next() {
					var geneSymbol string
					if err := geneRows.Scan(&geneSymbol); err == nil {
						panelGenes[geneSymbol] = true
					}
				}
				geneRows.Close()
			}
		}

		// 只使用 setting_panel.gene_ids 字段，不再回退到 gene_panel_relation 表

		matchedCount := 0
		for gene := range uploadedGenes {
			if panelGenes[gene] {
				matchedCount++
				continue
			}
			cleanGene := strings.TrimPrefix(gene, "Gene_")
			cleanGene = strings.TrimPrefix(cleanGene, "gene_")
			if panelGenes[cleanGene] {
				matchedCount++
				continue
			}
			if panelGenes[strings.ToLower(gene)] {
				matchedCount++
				continue
			}
			if panelGenes[strings.ToUpper(gene)] {
				matchedCount++
				continue
			}
		}

		var matchRate float64
		if len(panelGenes) > 0 {
			matchRate = float64(matchedCount) / float64(len(panelGenes))
		}

		if matchRate > bestMatchRate || (matchRate == bestMatchRate && len(panelGenes) > bestMatchLen) {
			bestMatchRate = matchRate
			bestMatchLen = len(panelGenes)

			var matchedGenes, missingGenes, extraGenes []string
			for gene := range panelGenes {
				found := false
				for uploadedGene := range uploadedGenes {
					if gene == uploadedGene ||
						gene == strings.TrimPrefix(uploadedGene, "Gene_") ||
						gene == strings.TrimPrefix(uploadedGene, "gene_") ||
						strings.ToLower(gene) == strings.ToLower(uploadedGene) {
						found = true
						break
					}
				}
				if found {
					matchedGenes = append(matchedGenes, gene)
				} else {
					missingGenes = append(missingGenes, gene)
				}
			}
			for gene := range uploadedGenes {
				found := false
				for panelGene := range panelGenes {
					if panelGene == gene ||
						panelGene == strings.TrimPrefix(gene, "Gene_") ||
						panelGene == strings.TrimPrefix(gene, "gene_") ||
						strings.ToLower(panelGene) == strings.ToLower(gene) {
						found = true
						break
					}
				}
				if !found {
					extraGenes = append(extraGenes, gene)
				}
			}

			matchStatus := "insufficient"
			if len(missingGenes) == 0 && len(extraGenes) == 0 {
				matchStatus = "exact"
			} else if len(missingGenes) == 0 {
				matchStatus = "subset"
			}

			bestMatch = &PanelMatchResult{
				PanelID:      panelID,
				PanelName:    panelName,
				PanelCode:    panelCode,
				MatchRate:    matchRate,
				MatchedGenes: matchedGenes,
				MissingGenes: missingGenes,
				ExtraGenes:   extraGenes,
				MatchStatus:  matchStatus,
			}
		}
	}

	if bestMatch == nil || bestMatch.MatchRate < 0.5 {
		return nil, fmt.Errorf("无法识别Panel，请检查上传的基因数据或联系管理员配置Panel")
	}

	return bestMatch, nil
}

// HandleUpdateSampleCancerType - 按样本设置检测类型
func HandleUpdateSampleCancerType(c *app.RequestContext, db *sql.DB) {
	if err := ensureBatchSampleModelIDColumn(db); err != nil {
		log.Printf("Failed to ensure detect_batch_sample.model_id: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "批次模型字段初始化失败"})
		return
	}
	// 解析路径参数
	batchParam := c.Param("batchId")

	// 尝试将参数转换为整数
	batchId, err := strconv.Atoi(batchParam)
	if err != nil {
		// 参数不是数字，尝试作为批次编号查询批次ID
		err = db.QueryRow(`SELECT id FROM detect_batch WHERE batch_code = ?`, batchParam).Scan(&batchId)
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
		SampleCode   string      `json:"sampleCode"`
		CancerTypeId interface{} `json:"cancerTypeId"`
		ModelId      interface{} `json:"modelId"`
	}

	// 优先使用 json.Unmarshal 解析 body 确保稳健，如果失败则尝试 c.Bind
	bodyBytes, _ := c.Body()
	if len(bodyBytes) > 0 {
		_ = json.Unmarshal(bodyBytes, &rawReq)
	}
	if rawReq.SampleCode == "" || rawReq.CancerTypeId == nil {
		if err := c.Bind(&rawReq); err != nil {
			c.JSON(consts.StatusBadRequest, ApiResponse{
				Code:    400,
				Success: false,
				Message: "请求参数错误",
				Data:    utils.H{"error": err.Error()},
			})
			return
		}
	}

	if rawReq.SampleCode == "" || rawReq.CancerTypeId == nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请求参数错误：sampleCode 和 cancerTypeId 不能为空",
			Data:    nil,
		})
		return
	}

	// 将 cancerTypeId 从 string 或 float64 转换为 int
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

	req := struct {
		SampleCode   string
		CancerTypeId int
		ModelId      int
	}{
		SampleCode:   rawReq.SampleCode,
		CancerTypeId: cancerTypeIdInt,
	}
	if rawReq.ModelId != nil {
		switch v := rawReq.ModelId.(type) {
		case float64:
			req.ModelId = int(v)
		case int:
			req.ModelId = v
		case string:
			req.ModelId, _ = strconv.Atoi(v)
		}
	}

	log.Printf("Updating sample %s in batch %d to cancer type %d", req.SampleCode, batchId, req.CancerTypeId)

	var targetCancerType cancerTypePanelInfo
	var targetPanelIDsStr string
	err = db.QueryRow(`SELECT id, name, COALESCE(panel_ids, '') FROM setting_cancer_type WHERE id = ? AND is_active = 1`, req.CancerTypeId).
		Scan(&targetCancerType.ID, &targetCancerType.Name, &targetPanelIDsStr)
	if err != nil {
		log.Printf("Failed to get target cancer type: %v", err)
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "检测类型不存在或已停用",
			Data:    nil,
		})
		return
	}
	targetCancerType.PanelIDs = parsePanelIDList(targetPanelIDsStr)
	if len(targetCancerType.PanelIDs) == 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "该检测类型未配置基因，无法匹配样本",
			Data:    nil,
		})
		return
	}

	requiredGenes, err := getCancerTypeRequiredGenes(db, targetCancerType.PanelIDs)
	if err != nil {
		log.Printf("Failed to get required genes for cancer type %d: %v", req.CancerTypeId, err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}
	if len(requiredGenes) == 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "该检测类型未配置基因，无法匹配样本",
			Data:    nil,
		})
		return
	}
	sampleGenes, err := getBatchSampleGenes(db, batchId, req.SampleCode)
	if err != nil {
		log.Printf("Failed to get sample genes for sample %s: %v", req.SampleCode, err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}
	missingGenes := missingRequiredGenes(requiredGenes, sampleGenes)
	if len(missingGenes) > 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "该检测类型所需基因未被样本基因完全覆盖，无法匹配",
			Data: utils.H{
				"sampleCode":       req.SampleCode,
				"targetCancerType": targetCancerType.Name,
				"requiredGenes":    requiredGenes,
				"sampleGenes":      sampleGenes,
				"missingGenes":     missingGenes,
			},
		})
		return
	}
	if req.ModelId > 0 {
		var modelCancerTypeID, isActive, isDeprecated int
		var modelName, parameters, formula string
		err = db.QueryRow(`SELECT cancer_type_id, is_active, COALESCE(is_deprecated, 0), model_name,
			COALESCE(parameters, ''), COALESCE(formula, '') FROM setting_model WHERE id = ?`, req.ModelId).
			Scan(&modelCancerTypeID, &isActive, &isDeprecated, &modelName, &parameters, &formula)
		if err != nil || isActive != 1 || isDeprecated == 1 || modelCancerTypeID != req.CancerTypeId {
			c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "所选模型不属于该检测类型或已停用"})
			return
		}
		modelGenes := extractModelGeneSymbols(parameters, formula, loadGeneSymbolMap(db))
		if missingModelGenes := missingRequiredGenes(modelGenes, sampleGenes); len(missingModelGenes) > 0 {
			c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "样本缺少所选模型需要的基因", Data: utils.H{"modelName": modelName, "missingGenes": missingModelGenes}})
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

	// detect_batch_sample 是上传批次的主记录，未建档样本也必须允许修改检测类型。
	_, err = tx.Exec("UPDATE detect_batch_sample SET cancer_type_id = ?, model_id = NULLIF(?, 0) WHERE sample_code = ? AND batch_id = ?", req.CancerTypeId, req.ModelId, req.SampleCode, batchId)
	if err != nil {
		tx.Rollback()
		log.Printf("Failed to update detect_batch_sample cancer_type_id: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	// 检查样本是否在detect_sample表中存在
	var sampleExists bool
	err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM detect_sample WHERE sample_code = ? AND batch_id = ?)", req.SampleCode, batchId).Scan(&sampleExists)
	if err != nil {
		tx.Rollback()
		log.Printf("Failed to check sample existence: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	if sampleExists {
		// 有真实样本档案时同步 detect_sample；未建档时不插入，避免缺少 patient_id 等必填字段。
		_, err = tx.Exec("UPDATE detect_sample SET cancer_type_id = ?, model_id = NULLIF(?, 0), sample_updated_at = NOW() WHERE sample_code = ? AND batch_id = ?", req.CancerTypeId, req.ModelId, req.SampleCode, batchId)
		if err != nil {
			tx.Rollback()
			log.Printf("Failed to update detect_sample cancer_type_id: %v", err)
			c.JSON(consts.StatusInternalServerError, ApiResponse{
				Code:    500,
				Success: false,
				Message: "服务器内部错误",
				Data:    nil,
			})
			return
		}
	}

	if _, err = tx.Exec("DELETE FROM detect_sample_panel_match WHERE batch_id = ? AND sample_code = ?", batchId, req.SampleCode); err != nil {
		tx.Rollback()
		log.Printf("Failed to invalidate sample panel match cache: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	// 提交事务
	err = tx.Commit()
	if err != nil {
		log.Printf("Failed to commit transaction: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    nil,
		})
		return
	}

	log.Printf("Successfully updated sample %s to cancer type %d", req.SampleCode, req.CancerTypeId)

	// 返回成功响应
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "样本检测类型更新成功",
		Data:    nil,
	})
}

// getCancerTypePanelCountMP - 获取检测类型的Panel数量 (Multi-Platform版本)
func getCancerTypePanelCountMP(db *sql.DB, cancerTypeId int) (int, error) {
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
