package handlers

import (
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func HandleUniEmployeePatientGroups(c *app.RequestContext, db *sql.DB) {
	employeeID, ok := requireMiniappEmployee(c, db)
	if !ok {
		return
	}
	rows, err := db.Query(`SELECT g.id, g.name, g.color, g.sort_order, COUNT(m.patient_id)
		FROM sale_patient_group g
		LEFT JOIN sale_patient_group_member m ON m.group_id = g.id
		WHERE g.sales_user_id = ?
		GROUP BY g.id, g.name, g.color, g.sort_order
		ORDER BY g.sort_order, g.id`, employeeID)
	if err != nil {
		ErrorResponse(c, consts.StatusInternalServerError, "获取分组失败", nil)
		return
	}
	defer rows.Close()
	list := []utils.H{}
	for rows.Next() {
		var id, sortOrder, patientCount int
		var name, color string
		if rows.Scan(&id, &name, &color, &sortOrder, &patientCount) == nil {
			list = append(list, utils.H{"id": id, "name": name, "color": color, "sort_order": sortOrder, "patient_count": patientCount})
		}
	}
	SuccessResponse(c, "获取成功", utils.H{"list": list})
}

func HandleUniEmployeeCreatePatientGroup(c *app.RequestContext, db *sql.DB) {
	employeeID, ok := requireMiniappEmployee(c, db)
	if !ok {
		return
	}
	var req struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	body, _ := c.Body()
	if json.Unmarshal(body, &req) != nil || strings.TrimSpace(req.Name) == "" {
		ErrorResponse(c, consts.StatusBadRequest, "请输入分组名称", nil)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if len([]rune(req.Name)) > 30 {
		ErrorResponse(c, consts.StatusBadRequest, "分组名称不能超过30个字", nil)
		return
	}
	if strings.TrimSpace(req.Color) == "" {
		req.Color = "#1677ff"
	}
	result, err := db.Exec(`INSERT INTO sale_patient_group (sales_user_id, name, color, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW())`, employeeID, req.Name, req.Color)
	if err != nil {
		ErrorResponse(c, consts.StatusBadRequest, "分组名称已存在", nil)
		return
	}
	id, _ := result.LastInsertId()
	SuccessResponse(c, "分组创建成功", utils.H{"id": id, "name": req.Name, "color": req.Color})
}

func HandleUniEmployeeDeletePatientGroup(c *app.RequestContext, db *sql.DB) {
	employeeID, ok := requireMiniappEmployee(c, db)
	if !ok {
		return
	}
	groupID, _ := strconv.Atoi(c.Param("id"))
	tx, err := db.Begin()
	if err != nil {
		ErrorResponse(c, consts.StatusInternalServerError, "删除分组失败", nil)
		return
	}
	defer tx.Rollback()
	var owned int
	if tx.QueryRow(`SELECT COUNT(*) FROM sale_patient_group WHERE id = ? AND sales_user_id = ?`, groupID, employeeID).Scan(&owned) != nil || owned == 0 {
		ErrorResponse(c, consts.StatusForbidden, "无权操作该分组", nil)
		return
	}
	if _, err = tx.Exec(`DELETE FROM sale_patient_group_member WHERE group_id = ?`, groupID); err != nil {
		ErrorResponse(c, consts.StatusInternalServerError, "删除分组失败", nil)
		return
	}
	if _, err = tx.Exec(`DELETE FROM sale_patient_group WHERE id = ? AND sales_user_id = ?`, groupID, employeeID); err != nil {
		ErrorResponse(c, consts.StatusInternalServerError, "删除分组失败", nil)
		return
	}
	if err = tx.Commit(); err != nil {
		ErrorResponse(c, consts.StatusInternalServerError, "删除分组失败", nil)
		return
	}
	SuccessResponse(c, "分组已删除", nil)
}

func HandleUniEmployeeSetPatientGroup(c *app.RequestContext, db *sql.DB) {
	employeeID, ok := requireMiniappEmployee(c, db)
	if !ok {
		return
	}
	patientID, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		GroupID int `json:"group_id"`
	}
	body, _ := c.Body()
	if json.Unmarshal(body, &req) != nil {
		ErrorResponse(c, consts.StatusBadRequest, "请求参数错误", nil)
		return
	}
	query := `SELECT COUNT(*) FROM detect_patient WHERE id = ? AND is_active = 1`
	args := []interface{}{patientID}
	if !miniappEmployeeCanManageAllPatients(db, employeeID) {
		query += " AND sales_person = ?"
		args = append(args, getMiniappEmployeeCode(db, employeeID))
	}
	var accessible int
	if db.QueryRow(query, args...).Scan(&accessible) != nil || accessible == 0 {
		ErrorResponse(c, consts.StatusForbidden, "无权操作该患者", nil)
		return
	}
	if req.GroupID > 0 {
		var owned int
		if db.QueryRow(`SELECT COUNT(*) FROM sale_patient_group WHERE id = ? AND sales_user_id = ?`, req.GroupID, employeeID).Scan(&owned) != nil || owned == 0 {
			ErrorResponse(c, consts.StatusForbidden, "无权操作该分组", nil)
			return
		}
	}
	tx, err := db.Begin()
	if err != nil {
		ErrorResponse(c, consts.StatusInternalServerError, "设置分组失败", nil)
		return
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`DELETE m FROM sale_patient_group_member m
		JOIN sale_patient_group g ON g.id = m.group_id
		WHERE m.patient_id = ? AND g.sales_user_id = ?`, patientID, employeeID); err != nil {
		ErrorResponse(c, consts.StatusInternalServerError, "设置分组失败", nil)
		return
	}
	if req.GroupID > 0 {
		if _, err = tx.Exec(`INSERT INTO sale_patient_group_member (group_id, patient_id, created_at) VALUES (?, ?, NOW())`, req.GroupID, patientID); err != nil {
			ErrorResponse(c, consts.StatusInternalServerError, "设置分组失败", nil)
			return
		}
	}
	if err = tx.Commit(); err != nil {
		ErrorResponse(c, consts.StatusInternalServerError, "设置分组失败", nil)
		return
	}
	SuccessResponse(c, "患者分组已更新", nil)
}
