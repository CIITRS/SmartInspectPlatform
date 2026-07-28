package handlers

import (
	"fmt"
	"log"
	"math/rand"
	"time"
)

// 任务类型
const (
	TaskTypeGeneratePDF = "generate_pdf"
	TaskTypeFileUpload  = "file_upload"
)

// Task 异步任务结构体
type Task struct {
	ID        string
	Type      string
	ReportID  int
	Data      map[string]interface{}
	Status    string
	Error     string
	CreatedAt int64
	UpdatedAt int64
}

// 任务状态
const (
	TaskStatusPending   = "pending"
	TaskStatusRunning   = "running"
	TaskStatusCompleted = "completed"
	TaskStatusFailed    = "failed"
)

// 任务队列
var taskQueue chan Task

// InitAsync 初始化异步处理
func InitAsync() {
	// 创建任务队列，容量为100
	taskQueue = make(chan Task, 100)

	// 启动5个worker协程处理任务
	for i := 0; i < 5; i++ {
		go worker(i, taskQueue)
	}

	log.Println("异步任务处理系统初始化成功")
}

// worker 任务处理协程
func worker(id int, tasks chan Task) {
	log.Printf("Worker %d 启动", id)

	for task := range tasks {
		log.Printf("Worker %d 开始处理任务: %s, 类型: %s", id, task.ID, task.Type)

		// 更新任务状态为运行中
		task.Status = TaskStatusRunning

		// 根据任务类型处理
		switch task.Type {
		case TaskTypeGeneratePDF:
			handleGeneratePDFTask(task)
		case TaskTypeFileUpload:
			handleFileUploadTask(task)
		default:
			log.Printf("未知任务类型: %s", task.Type)
			task.Status = TaskStatusFailed
			task.Error = "未知任务类型"
		}

		log.Printf("Worker %d 完成任务: %s, 状态: %s", id, task.ID, task.Status)
	}
}

// handleGeneratePDFTask 处理PDF生成任务
func handleGeneratePDFTask(task Task) {
	log.Printf("跳过PDF预生成任务，报告ID: %d；PDF将在用户下载时临时生成", task.ReportID)
	task.Status = TaskStatusCompleted
	log.Printf("PDF生成任务完成: %s", task.ID)
}

// handleFileUploadTask 处理文件上传任务
func handleFileUploadTask(task Task) {
	// TODO: 实现文件上传的异步处理
	log.Printf("处理文件上传任务: %s", task.ID)
	task.Status = TaskStatusCompleted
}

// AddTask 添加异步任务
func AddTask(taskType string, reportID int, data map[string]interface{}) string {
	// 生成任务ID
	taskID := generateTaskID()

	// 创建任务
	task := Task{
		ID:        taskID,
		Type:      taskType,
		ReportID:  reportID,
		Data:      data,
		Status:    TaskStatusPending,
		CreatedAt: getCurrentTimestamp(),
		UpdatedAt: getCurrentTimestamp(),
	}

	// 添加到任务队列
	taskQueue <- task

	log.Printf("添加异步任务: %s, 类型: %s, 报告ID: %d", taskID, taskType, reportID)

	return taskID
}

// generateTaskID 生成任务ID
func generateTaskID() string {
	// 使用时间戳和随机数生成任务ID
	return fmt.Sprintf("task_%d_%d", getCurrentTimestamp(), generateRandomInt(1000, 9999))
}

// getCurrentTimestamp 获取当前时间戳
func getCurrentTimestamp() int64 {
	return time.Now().Unix()
}

// generateRandomInt 生成随机整数
func generateRandomInt(min, max int) int {
	return min + rand.Intn(max-min+1)
}
