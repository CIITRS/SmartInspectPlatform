package handlers

import (
	"database/sql"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// Express 快递运单结构体
type Express struct {
	ID               int            `json:"id"`
	SampleID         int            `json:"sample_id"`
	SampleCode       string         `json:"sample_code"`
	ExpressCompany   sql.NullString `json:"express_company"`
	TrackingNumber   string         `json:"tracking_number"`
	SenderName       sql.NullString `json:"sender_name"`
	SenderPhone      sql.NullString `json:"sender_phone"`
	SenderAddress    sql.NullString `json:"sender_address"`
	ReceiverName     sql.NullString `json:"receiver_name"`
	ReceiverPhone    sql.NullString `json:"receiver_phone"`
	ReceiverAddress  sql.NullString `json:"receiver_address"`
	SendTime         sql.NullTime   `json:"send_time"`
	ReceiveTime      sql.NullTime   `json:"receive_time"`
	Status           string         `json:"status"`
	Notes            sql.NullString `json:"notes"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

// HandleCreateExpress 创建快递运单
func HandleCreateExpress(c *app.RequestContext, db *sql.DB) {
	// 解析请求体
	var req struct {
		SampleID        int    `json:"sample_id" binding:"required"`
		SampleCode      string `json:"sample_code" binding:"required"`
		ExpressCompany  string `json:"express_company"`
		TrackingNumber  string `json:"tracking_number" binding:"required"`
		SenderName      string `json:"sender_name"`
		SenderPhone     string `json:"sender_phone"`
		SenderAddress   string `json:"sender_address"`
		ReceiverName    string `json:"receiver_name"`
		ReceiverPhone   string `json:"receiver_phone"`
		ReceiverAddress string `json:"receiver_address"`
		SendTime        string `json:"send_time"`
		ReceiveTime     string `json:"receive_time"`
		Status          string `json:"status"`
		Notes           string `json:"notes"`
	}

	if err := c.Bind(&req); err != nil {
		log.Printf("Failed to bind express creation request: %v", err)
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请求参数错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 验证必填字段
	if req.SampleID <= 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "样本ID不能为空",
			Data:    nil,
		})
		return
	}

	if req.SampleCode == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "样本编号不能为空",
			Data:    nil,
		})
		return
	}

	if req.TrackingNumber == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "快递单号不能为空",
			Data:    nil,
		})
		return
	}

	// 设置默认状态
	status := "pending"
	if req.Status != "" {
		status = req.Status
	}

	// 解析时间字段
	var sendTime, receiveTime interface{}
	if req.SendTime != "" {
		if t, err := time.Parse("2006-01-02T15:04:05Z", req.SendTime); err == nil {
			sendTime = t
		} else if t, err := time.Parse("2006-01-02 15:04:05", req.SendTime); err == nil {
			sendTime = t
		}
	}
	if req.ReceiveTime != "" {
		if t, err := time.Parse("2006-01-02T15:04:05Z", req.ReceiveTime); err == nil {
			receiveTime = t
		} else if t, err := time.Parse("2006-01-02 15:04:05", req.ReceiveTime); err == nil {
			receiveTime = t
		}
	}

	// 插入数据库
	query := `INSERT INTO detect_sample_express 
		(sample_id, sample_code, express_company, tracking_number, sender_name, sender_phone, 
		sender_address, receiver_name, receiver_phone, receiver_address, send_time, receive_time, status, notes) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	result, err := db.Exec(query, req.SampleID, req.SampleCode, req.ExpressCompany, req.TrackingNumber,
		req.SenderName, req.SenderPhone, req.SenderAddress, req.ReceiverName, req.ReceiverPhone,
		req.ReceiverAddress, sendTime, receiveTime, status, req.Notes)

	if err != nil {
		log.Printf("Failed to create express: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 获取插入的ID
	id, err := result.LastInsertId()
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

	// 查询刚创建的快递运单
	express, err := getExpressByID(db, int(id))
	if err != nil {
		log.Printf("Failed to query created express: %v", err)
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "创建快递运单成功",
			Data:    utils.H{"id": id},
		})
		return
	}

	// 返回创建的快递运单信息
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "创建快递运单成功",
		Data:    express,
	})
}

// HandleGetExpress 根据样本ID获取快递运单
func HandleGetExpress(c *app.RequestContext, db *sql.DB) {
	// 获取样本ID参数
	sampleIDStr := c.Param("sampleId")
	if sampleIDStr == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请求参数错误，缺少样本ID",
			Data:    nil,
		})
		return
	}

	sampleID, err := strconv.Atoi(sampleIDStr)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的样本ID",
			Data:    nil,
		})
		return
	}

	// 查询快递运单
	query := `SELECT id, sample_id, sample_code, express_company, tracking_number, 
		sender_name, sender_phone, sender_address, receiver_name, receiver_phone, 
		receiver_address, send_time, receive_time, status, notes, created_at, updated_at 
		FROM detect_sample_express WHERE sample_id = ? ORDER BY created_at DESC`

	rows, err := db.Query(query, sampleID)
	if err != nil {
		log.Printf("Failed to query express: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}
	defer rows.Close()

	var expressList []utils.H
	for rows.Next() {
		express := scanExpressRow(rows)
		if express != nil {
			expressList = append(expressList, express)
		}
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error iterating express rows: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 返回快递运单列表
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "获取快递运单成功",
		Data: utils.H{
			"list":  expressList,
			"total": len(expressList),
		},
	})
}

// HandleUpdateExpress 更新快递运单信息
func HandleUpdateExpress(c *app.RequestContext, db *sql.DB) {
	// 获取快递运单ID
	idStr := c.Param("id")
	if idStr == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请求参数错误，缺少运单ID",
			Data:    nil,
		})
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的运单ID",
			Data:    nil,
		})
		return
	}

	// 解析请求体
	var req struct {
		SampleID        int    `json:"sample_id"`
		SampleCode      string `json:"sample_code"`
		ExpressCompany  string `json:"express_company"`
		TrackingNumber  string `json:"tracking_number"`
		SenderName      string `json:"sender_name"`
		SenderPhone     string `json:"sender_phone"`
		SenderAddress   string `json:"sender_address"`
		ReceiverName    string `json:"receiver_name"`
		ReceiverPhone   string `json:"receiver_phone"`
		ReceiverAddress string `json:"receiver_address"`
		SendTime        string `json:"send_time"`
		ReceiveTime     string `json:"receive_time"`
		Status          string `json:"status"`
		Notes           string `json:"notes"`
	}

	if err := c.Bind(&req); err != nil {
		log.Printf("Failed to bind express update request: %v", err)
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请求参数错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 构建动态更新查询，只更新非空字段
	var setClauses []string
	var args []interface{}

	if req.SampleID > 0 {
		setClauses = append(setClauses, "sample_id = ?")
		args = append(args, req.SampleID)
	}
	if req.SampleCode != "" {
		setClauses = append(setClauses, "sample_code = ?")
		args = append(args, req.SampleCode)
	}
	if req.ExpressCompany != "" {
		setClauses = append(setClauses, "express_company = ?")
		args = append(args, req.ExpressCompany)
	}
	if req.TrackingNumber != "" {
		setClauses = append(setClauses, "tracking_number = ?")
		args = append(args, req.TrackingNumber)
	}
	if req.SenderName != "" {
		setClauses = append(setClauses, "sender_name = ?")
		args = append(args, req.SenderName)
	}
	if req.SenderPhone != "" {
		setClauses = append(setClauses, "sender_phone = ?")
		args = append(args, req.SenderPhone)
	}
	if req.SenderAddress != "" {
		setClauses = append(setClauses, "sender_address = ?")
		args = append(args, req.SenderAddress)
	}
	if req.ReceiverName != "" {
		setClauses = append(setClauses, "receiver_name = ?")
		args = append(args, req.ReceiverName)
	}
	if req.ReceiverPhone != "" {
		setClauses = append(setClauses, "receiver_phone = ?")
		args = append(args, req.ReceiverPhone)
	}
	if req.ReceiverAddress != "" {
		setClauses = append(setClauses, "receiver_address = ?")
		args = append(args, req.ReceiverAddress)
	}
	if req.SendTime != "" {
		if t, err := time.Parse("2006-01-02T15:04:05Z", req.SendTime); err == nil {
			setClauses = append(setClauses, "send_time = ?")
			args = append(args, t)
		} else if t, err := time.Parse("2006-01-02 15:04:05", req.SendTime); err == nil {
			setClauses = append(setClauses, "send_time = ?")
			args = append(args, t)
		}
	}
	if req.ReceiveTime != "" {
		if t, err := time.Parse("2006-01-02T15:04:05Z", req.ReceiveTime); err == nil {
			setClauses = append(setClauses, "receive_time = ?")
			args = append(args, t)
		} else if t, err := time.Parse("2006-01-02 15:04:05", req.ReceiveTime); err == nil {
			setClauses = append(setClauses, "receive_time = ?")
			args = append(args, t)
		}
	}
	if req.Status != "" {
		setClauses = append(setClauses, "status = ?")
		args = append(args, req.Status)
	}
	if req.Notes != "" {
		setClauses = append(setClauses, "notes = ?")
		args = append(args, req.Notes)
	}

	// 如果没有要更新的字段
	if len(setClauses) == 0 {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "没有要更新的字段",
			Data:    nil,
		})
		return
	}

	// 添加更新时间
	setClauses = append(setClauses, "updated_at = NOW()")

	// 构建完整的查询
	query := "UPDATE detect_sample_express SET " + strings.Join(setClauses, ", ") + " WHERE id = ?"
	args = append(args, id)

	// 执行更新
	_, err = db.Exec(query, args...)
	if err != nil {
		log.Printf("Failed to update express: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 查询更新后的快递运单
	express, err := getExpressByID(db, id)
	if err != nil {
		log.Printf("Failed to query updated express: %v", err)
		c.JSON(consts.StatusOK, ApiResponse{
			Code:    200,
			Success: true,
			Message: "更新快递运单成功",
			Data:    nil,
		})
		return
	}

	// 返回更新后的快递运单信息
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "更新快递运单成功",
		Data:    express,
	})
}

// HandleDeleteExpress 删除快递运单
func HandleDeleteExpress(c *app.RequestContext, db *sql.DB) {
	// 获取快递运单ID
	idStr := c.Param("id")
	if idStr == "" {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请求参数错误，缺少运单ID",
			Data:    nil,
		})
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "无效的运单ID",
			Data:    nil,
		})
		return
	}

	// 删除快递运单
	_, err = db.Exec("DELETE FROM detect_sample_express WHERE id = ?", id)
	if err != nil {
		log.Printf("Failed to delete express: %v", err)
		c.JSON(consts.StatusInternalServerError, ApiResponse{
			Code:    500,
			Success: false,
			Message: "服务器内部错误",
			Data:    utils.H{"error": err.Error()},
		})
		return
	}

	// 返回删除成功
	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "删除快递运单成功",
		Data:    nil,
	})
}

// getExpressByID 根据ID获取快递运单
func getExpressByID(db *sql.DB, id int) (utils.H, error) {
	query := `SELECT id, sample_id, sample_code, express_company, tracking_number, 
		sender_name, sender_phone, sender_address, receiver_name, receiver_phone, 
		receiver_address, send_time, receive_time, status, notes, created_at, updated_at 
		FROM detect_sample_express WHERE id = ?`

	var express Express
	err := db.QueryRow(query, id).Scan(
		&express.ID, &express.SampleID, &express.SampleCode, &express.ExpressCompany,
		&express.TrackingNumber, &express.SenderName, &express.SenderPhone, &express.SenderAddress,
		&express.ReceiverName, &express.ReceiverPhone, &express.ReceiverAddress,
		&express.SendTime, &express.ReceiveTime, &express.Status, &express.Notes,
		&express.CreatedAt, &express.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return scanExpress(&express), nil
}

// scanExpressRow 扫描一行快递运单数据
func scanExpressRow(rows *sql.Rows) utils.H {
	var express Express
	err := rows.Scan(
		&express.ID, &express.SampleID, &express.SampleCode, &express.ExpressCompany,
		&express.TrackingNumber, &express.SenderName, &express.SenderPhone, &express.SenderAddress,
		&express.ReceiverName, &express.ReceiverPhone, &express.ReceiverAddress,
		&express.SendTime, &express.ReceiveTime, &express.Status, &express.Notes,
		&express.CreatedAt, &express.UpdatedAt,
	)
	if err != nil {
		log.Printf("Failed to scan express row: %v", err)
		return nil
	}

	return scanExpress(&express)
}

// scanExpress 将Express结构体转换为响应格式
func scanExpress(e *Express) utils.H {
	result := utils.H{
		"id":             e.ID,
		"sample_id":      e.SampleID,
		"sample_code":    e.SampleCode,
		"tracking_number": e.TrackingNumber,
		"status":        e.Status,
		"created_at":    e.CreatedAt.Format("2006-01-02T15:04:05+08:00"),
		"updated_at":    e.UpdatedAt.Format("2006-01-02T15:04:05+08:00"),
	}

	if e.ExpressCompany.Valid {
		result["express_company"] = e.ExpressCompany.String
	}
	if e.SenderName.Valid {
		result["sender_name"] = e.SenderName.String
	}
	if e.SenderPhone.Valid {
		result["sender_phone"] = e.SenderPhone.String
	}
	if e.SenderAddress.Valid {
		result["sender_address"] = e.SenderAddress.String
	}
	if e.ReceiverName.Valid {
		result["receiver_name"] = e.ReceiverName.String
	}
	if e.ReceiverPhone.Valid {
		result["receiver_phone"] = e.ReceiverPhone.String
	}
	if e.ReceiverAddress.Valid {
		result["receiver_address"] = e.ReceiverAddress.String
	}
	if e.SendTime.Valid {
		result["send_time"] = e.SendTime.Time.Format("2006-01-02T15:04:05+08:00")
	}
	if e.ReceiveTime.Valid {
		result["receive_time"] = e.ReceiveTime.Time.Format("2006-01-02T15:04:05+08:00")
	}
	if e.Notes.Valid {
		result["notes"] = e.Notes.String
	}

	return result
}
