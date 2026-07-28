package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func parseSelectedGeneIDs(parameters string) []int {
	if parameters == "" {
		return nil
	}

	var paramsMap map[string]interface{}
	if err := json.Unmarshal([]byte(parameters), &paramsMap); err != nil {
		return nil
	}

	rawGenes, ok := paramsMap["selectedGenes"].([]interface{})
	if !ok {
		return nil
	}

	selectedGeneIDs := make([]int, 0, len(rawGenes))
	for _, rawGeneID := range rawGenes {
		switch v := rawGeneID.(type) {
		case float64:
			selectedGeneIDs = append(selectedGeneIDs, int(v))
		case int:
			selectedGeneIDs = append(selectedGeneIDs, v)
		}
	}

	return selectedGeneIDs
}

func loadGeneSymbolMap(db *sql.DB) map[int]string {
	geneMap := make(map[int]string)

	rows, err := db.Query(`SELECT id, gene_symbol FROM setting_gene`)
	if err != nil {
		return geneMap
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var geneSymbol string
		if err := rows.Scan(&id, &geneSymbol); err == nil && geneSymbol != "" {
			geneMap[id] = geneSymbol
		}
	}

	return geneMap
}

func extractModelGeneSymbols(parameters, formula string, geneSymbolMap map[int]string) []string {
	selectedGeneIDs := parseSelectedGeneIDs(parameters)
	if len(selectedGeneIDs) > 0 {
		geneSymbols := make([]string, 0, len(selectedGeneIDs))
		for _, geneID := range selectedGeneIDs {
			if geneSymbol, ok := geneSymbolMap[geneID]; ok && geneSymbol != "" {
				geneSymbols = append(geneSymbols, geneSymbol)
			}
		}
		if len(geneSymbols) > 0 {
			return uniqueSortedStrings(geneSymbols)
		}
	}

	return uniqueSortedStrings(extractGenesFromFormula(formula))
}

func uniqueSortedStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}

	valueSet := make(map[string]struct{})
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := valueSet[trimmed]; exists {
			continue
		}
		valueSet[trimmed] = struct{}{}
		result = append(result, trimmed)
	}

	sort.Strings(result)
	return result
}

func containsAllGenes(haystack map[string]bool, needles []string) bool {
	for _, gene := range needles {
		if !haystack[gene] {
			return false
		}
	}
	return true
}

func diffGenes(left []string, right map[string]bool) []string {
	diff := make([]string, 0)
	for _, gene := range left {
		if !right[gene] {
			diff = append(diff, gene)
		}
	}
	return uniqueSortedStrings(diff)
}

// getGeneSymbolByAnyName 根据基因名称（可能是 gene_name 或 gene_symbol）获取对应的 gene_symbol
// 先尝试匹配 gene_symbol，如果匹配不到再尝试匹配 gene_name
// 支持不区分大小写的匹配
func getGeneSymbolByAnyName(db *sql.DB, geneName string) (string, error) {
	geneName = strings.TrimSpace(geneName)
	if geneName == "" {
		return "", fmt.Errorf("gene name is empty")
	}

	// 首先尝试精确匹配 gene_symbol（不考虑 is_active，确保能找到所有基因）
	var geneSymbol string
	err := db.QueryRow("SELECT gene_symbol FROM setting_gene WHERE gene_symbol = ?", geneName).Scan(&geneSymbol)
	if err == nil && geneSymbol != "" {
		return geneSymbol, nil
	}

	// 如果精确匹配不到，尝试不区分大小写匹配 gene_symbol
	err = db.QueryRow("SELECT gene_symbol FROM setting_gene WHERE UPPER(gene_symbol) = UPPER(?)", geneName).Scan(&geneSymbol)
	if err == nil && geneSymbol != "" {
		return geneSymbol, nil
	}

	// 如果 gene_symbol 匹配不到，尝试精确匹配 gene_name
	err = db.QueryRow("SELECT gene_symbol FROM setting_gene WHERE gene_name = ?", geneName).Scan(&geneSymbol)
	if err == nil && geneSymbol != "" {
		return geneSymbol, nil
	}

	// 如果精确匹配不到，尝试不区分大小写匹配 gene_name
	err = db.QueryRow("SELECT gene_symbol FROM setting_gene WHERE UPPER(gene_name) = UPPER(?)", geneName).Scan(&geneSymbol)
	if err == nil && geneSymbol != "" {
		return geneSymbol, nil
	}

	// 如果都匹配不到，返回原始名称（用于后续判断）
	return geneName, fmt.Errorf("gene not found: %s", geneName)
}

// getGeneSymbolListByAnyNames 批量获取基因名称对应的 gene_symbol
func getGeneSymbolListByAnyNames(db *sql.DB, geneNames []string) []string {
	result := make([]string, 0, len(geneNames))
	for _, geneName := range geneNames {
		geneSymbol, _ := getGeneSymbolByAnyName(db, geneName)
		result = append(result, geneSymbol)
	}
	return result
}

func getGeneNameByAnyName(db *sql.DB, rawGeneName string) (string, error) {
	rawGeneName = strings.TrimSpace(rawGeneName)
	if rawGeneName == "" {
		return "", fmt.Errorf("gene name is empty")
	}

	var geneName string
	err := db.QueryRow("SELECT gene_name FROM setting_gene WHERE gene_name = ?", rawGeneName).Scan(&geneName)
	if err == nil && geneName != "" {
		return geneName, nil
	}

	err = db.QueryRow("SELECT gene_name FROM setting_gene WHERE UPPER(gene_name) = UPPER(?)", rawGeneName).Scan(&geneName)
	if err == nil && geneName != "" {
		return geneName, nil
	}

	err = db.QueryRow("SELECT gene_name FROM setting_gene WHERE gene_symbol = ?", rawGeneName).Scan(&geneName)
	if err == nil && geneName != "" {
		return geneName, nil
	}

	err = db.QueryRow("SELECT gene_name FROM setting_gene WHERE UPPER(gene_symbol) = UPPER(?)", rawGeneName).Scan(&geneName)
	if err == nil && geneName != "" {
		return geneName, nil
	}

	return rawGeneName, fmt.Errorf("gene not found: %s", rawGeneName)
}

func isResultMetaKey(key string) bool {
	switch key {
	case "Sample", "sample_code", "location", "Location", "Total Events", "totalEvents":
		return true
	default:
		return false
	}
}

func normalizeGeneDataKeysToSymbols(db *sql.DB, geneData map[string]interface{}) map[string]interface{} {
	normalized := make(map[string]interface{}, len(geneData))
	for key, value := range geneData {
		if isResultMetaKey(key) {
			normalized[key] = value
			continue
		}
		geneSymbol, err := getGeneSymbolByAnyName(db, key)
		if err != nil || strings.TrimSpace(geneSymbol) == "" {
			normalized[key] = value
			continue
		}
		normalized[geneSymbol] = value
	}
	return normalized
}

func normalizeGeneDataKeysToSymbolsWithMissing(db *sql.DB, geneData map[string]interface{}) (map[string]interface{}, []string) {
	normalized := make(map[string]interface{}, len(geneData))
	missing := make([]string, 0)
	for key, value := range geneData {
		if isResultMetaKey(key) {
			normalized[key] = value
			continue
		}
		geneSymbol, err := getGeneSymbolByAnyName(db, key)
		if err != nil || strings.TrimSpace(geneSymbol) == "" {
			normalized[key] = value
			missing = append(missing, key)
			continue
		}
		normalized[geneSymbol] = value
	}
	return normalized, uniqueSortedStrings(missing)
}

func expandGeneDataAliasesForFormula(db *sql.DB, geneData map[string]interface{}) map[string]interface{} {
	expanded := make(map[string]interface{}, len(geneData)*2)
	for key, value := range geneData {
		expanded[key] = value
		if isResultMetaKey(key) {
			continue
		}

		var geneName, geneSymbol string
		err := db.QueryRow(`SELECT gene_name, gene_symbol FROM setting_gene
			WHERE gene_name = ? OR UPPER(gene_name) = UPPER(?) OR gene_symbol = ? OR UPPER(gene_symbol) = UPPER(?)
			LIMIT 1`, key, key, key, key).Scan(&geneName, &geneSymbol)
		if err != nil {
			continue
		}
		if geneName != "" {
			expanded[geneName] = value
		}
		if geneSymbol != "" {
			expanded[geneSymbol] = value
		}
	}
	// 历史结果只有合并后的 HWG01。新公式优先使用平台独立变量；
	// 无法回溯平台原值时回退到旧值，避免历史报告因变量缺失而无法生成。
	if value, exists := expanded["HWG01"]; exists {
		if _, exists := expanded["HWG01_F8"]; !exists {
			expanded["HWG01_F8"] = value
		}
		if _, exists := expanded["HWG01_V5"]; !exists {
			expanded["HWG01_V5"] = value
		}
	}
	return expanded
}
