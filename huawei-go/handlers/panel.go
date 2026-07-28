package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func HandleListPanels(c *app.RequestContext, db *sql.DB) {
	activeOnly := c.Query("activeOnly") == "1" || c.Query("activeOnly") == "true"

	query := `SELECT id, panel_name, panel_code, description, is_active, created_at, updated_at, gene_ids 
			FROM setting_panel`
	if activeOnly {
		query += " WHERE is_active = 1"
	}
	query += " ORDER BY panel_name ASC"

	rows, err := db.Query(query)
	if err != nil {
		log.Printf("Failed to query panels: %v", err)
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "获取Panel列表成功",
			Data:    []utils.H{},
		})
		return
	}
	defer rows.Close()

	var panels []utils.H
	for rows.Next() {
		var id, isActive int
		var panelName, panelCode, description string
		var createdAt, updatedAt time.Time
		var geneIDs sql.NullString

		err := rows.Scan(&id, &panelName, &panelCode, &description, &isActive, &createdAt, &updatedAt, &geneIDs)
		if err != nil {
			log.Printf("Failed to scan panel: %v", err)
			continue
		}

		panel := utils.H{
			"id":           id,
			"panelName":     panelName,
			"panelCode":     panelCode,
			"description":   description,
			"isActive":      isActive,
			"is_active":     isActive,
			"createdAt":     createdAt.Format("2006-01-02T15:04:05+08:00"),
			"updatedAt":     updatedAt.Format("2006-01-02T15:04:05+08:00"),
			"geneNames":     "",
		}

		// 处理 gene_ids 字段并查询基因名称
		if geneIDs.Valid && geneIDs.String != "" {
			panel["gene_ids"] = geneIDs.String
			
			// 查询关联的基因名称
			geneRows, geneErr := db.Query(`SELECT gene_symbol FROM setting_gene WHERE FIND_IN_SET(id, ?) > 0`, geneIDs.String)
			if geneErr == nil {
				var geneNames []string
				for geneRows.Next() {
					var geneSymbol string
					if err := geneRows.Scan(&geneSymbol); err == nil {
						geneNames = append(geneNames, geneSymbol)
					}
				}
				geneRows.Close()
				panel["geneNames"] = strings.Join(geneNames, ", ")
			}
		}

		panels = append(panels, panel)
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取Panel列表成功",
		Data:    panels,
	})
}

func HandleCreatePanel(c *app.RequestContext, db *sql.DB) {
	var req struct {
		PanelName   string `json:"panel_name" form:"panel_name"`
		PanelCode   string `json:"panel_code" form:"panel_code"`
		Description string `json:"description" form:"description"`
		IsActive    int    `json:"is_active" form:"is_active"`
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

	if req.IsActive == 0 {
		req.IsActive = 1
	}

	result, err := db.Exec(`INSERT INTO setting_panel (panel_name, panel_code, description, is_active, created_at, updated_at) 
			VALUES (?, ?, ?, ?, NOW(), NOW())`,
		req.PanelName, req.PanelCode, req.Description, req.IsActive)
	if err != nil {
		log.Printf("Failed to create panel: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "创建Panel失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	panelID, err := result.LastInsertId()
	if err != nil {
		log.Printf("Failed to get last insert ID: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 创建成功后清除基因缓存，因为基因关联可能改变
	DeleteCache("genes")
	DeleteCache("genes_active")

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "创建Panel成功",
		Data: utils.H{
			"id": panelID,
		},
	})
}

func HandleUpdatePanel(c *app.RequestContext, db *sql.DB) {
	idParam := c.Param("id")
	var id int
	_, err := fmt.Sscanf(idParam, "%d", &id)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的Panel ID",
			Data:    nil,
		})
		return
	}

	var req struct {
		PanelName   string `json:"panel_name" form:"panel_name"`
		PanelCode   string `json:"panel_code" form:"panel_code"`
		Description string `json:"description" form:"description"`
		IsActive    int    `json:"is_active" form:"is_active"`
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

	_, err = db.Exec(`UPDATE setting_panel SET panel_name = ?, panel_code = ?, description = ?, is_active = ?, updated_at = NOW() 
			WHERE id = ?`,
		req.PanelName, req.PanelCode, req.Description, req.IsActive, id)
	if err != nil {
		log.Printf("Failed to update panel: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "更新Panel失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 更新成功后清除基因缓存，因为基因关联可能改变
	DeleteCache("genes")
	DeleteCache("genes_active")

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "更新Panel成功",
		Data:    nil,
	})
}

func HandleDeletePanel(c *app.RequestContext, db *sql.DB) {
	idParam := c.Param("id")
	var id int
	_, err := fmt.Sscanf(idParam, "%d", &id)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的Panel ID",
			Data:    nil,
		})
		return
	}

	_, err = db.Exec("UPDATE setting_panel SET is_active = 0, updated_at = NOW() WHERE id = ?", id)
	if err != nil {
		log.Printf("Failed to delete panel: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "删除Panel失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 删除成功后清除基因缓存，因为基因关联可能改变
	DeleteCache("genes")
	DeleteCache("genes_active")

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "删除Panel成功",
		Data:    nil,
	})
}

func HandleGetPanelGenes(c *app.RequestContext, db *sql.DB) {
	idParam := c.Param("id")
	var panelID int
	_, err := fmt.Sscanf(idParam, "%d", &panelID)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的Panel ID",
			Data:    nil,
		})
		return
	}

	// 首先尝试从 gene_ids 字段读取
	var geneIDsStr sql.NullString
	err = db.QueryRow("SELECT gene_ids FROM setting_panel WHERE id = ?", panelID).Scan(&geneIDsStr)
	if err == nil && geneIDsStr.Valid && geneIDsStr.String != "" {
		// 解析 gene_ids 字符串（假设是逗号分隔的ID）
		genes := getGenesFromIDs(db, geneIDsStr.String)
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "获取Panel基因列表成功",
			Data:    genes,
		})
		return
	}

	// gene_ids 为空，返回空数组
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取Panel基因列表成功",
		Data:    []utils.H{},
	})
}

// 辅助函数：根据 gene_ids 字符串获取基因列表
func getGenesFromIDs(db *sql.DB, geneIDsStr string) []utils.H {
	var genes []utils.H
	rows, err := db.Query(`SELECT id, gene_name, gene_symbol, description 
		FROM setting_gene 
		ORDER BY gene_symbol ASC`)
	if err != nil {
		log.Printf("Failed to query genes: %v", err)
		return []utils.H{}
	}
	defer rows.Close()

	geneIDMap := make(map[int]utils.H)
	for rows.Next() {
		var id int
		var geneName, geneSymbol, description string
		err := rows.Scan(&id, &geneName, &geneSymbol, &description)
		if err != nil {
			continue
		}
		geneIDMap[id] = utils.H{
			"id":          id,
			"name":        geneName,
			"geneSymbol":  geneSymbol,
			"description": description,
		}
	}

	// 根据 geneIDsStr 过滤和排序
	parts := strings.Split(geneIDsStr, ",")
	for _, part := range parts {
		var id int
		_, err := fmt.Sscanf(part, "%d", &id)
		if err == nil {
			if gene, ok := geneIDMap[id]; ok {
				genes = append(genes, gene)
			}
		}
	}

	return genes
}

func HandleUpdatePanelGenes(c *app.RequestContext, db *sql.DB) {
	idParam := c.Param("id")
	var panelID int
	_, err := fmt.Sscanf(idParam, "%d", &panelID)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的Panel ID",
			Data:    nil,
		})
		return
	}

	var req struct {
		GeneIDs []int `json:"geneIds"`
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

	// 构建 gene_ids 字符串
	geneIDStrs := make([]string, len(req.GeneIDs))
	for i, id := range req.GeneIDs {
		geneIDStrs[i] = fmt.Sprintf("%d", id)
	}
	geneIDsStr := strings.Join(geneIDStrs, ",")

	// 更新 setting_panel 表的 gene_ids 字段
	_, err = db.Exec(`UPDATE setting_panel SET gene_ids = ?, updated_at = NOW() WHERE id = ?`, geneIDsStr, panelID)
	if err != nil {
		log.Printf("Failed to update panel gene_ids: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "更新Panel基因关联失败",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 更新成功后清除基因缓存，因为基因关联已改变
	DeleteCache("genes")
	DeleteCache("genes_active")

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "更新Panel基因关联成功",
		Data:    nil,
	})
}
