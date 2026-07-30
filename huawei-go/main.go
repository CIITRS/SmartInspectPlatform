package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/joho/godotenv"

	"huawei-go/handlers"
)

// 从上下文获取用户ID的键
const UserIDKey = "userID"

func main() {
	// 显示系统信息
	fmt.Println("=============================================================")
	fmt.Println("          华微智检报告系统 (Huawei Diagnosis Report System)")
	fmt.Println("=============================================================")

	// 加载环境变量
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: No .env file found")
	}

	// 初始化AI聊天客户端
	handlers.InitAIChat()

	// 加载配置
	config := LoadConfig()
	fmt.Println("配置加载完成")

	// 初始化数据库连接
	db, err := InitDB(config)
	if err != nil {
		log.Printf("警告: 数据库连接失败: %v", err)
		log.Println("服务器将继续运行，但部分功能可能受限...")
		fmt.Printf("数据库连接状态: 失败 - %v\n", err)
	} else {
		if err := EnsureSchema(db, config.DBName); err != nil {
			log.Printf("警告: 数据库结构检查失败: %v", err)
		}
		fmt.Println("数据库连接状态: 成功")
	}

	// 生成RSA密钥对
	RSAKeyPair, err = GenerateRSAKeys()
	if err != nil {
		log.Printf("警告: RSA密钥对生成失败: %v", err)
		log.Println("服务器将继续运行，但密码加密功能可能受限...")
	} else {
		fmt.Println("RSA密钥对生成成功")
	}

	// 初始化文件URL管理器
	handlers.InitFileURLManager()
	fmt.Println("文件URL管理器初始化成功")

	// 初始化Redis连接
	if err := handlers.InitRedis(config.RedisAddr, config.RedisPassword, config.RedisDB); err != nil {
		log.Printf("警告: Redis连接失败: %v", err)
		log.Println("服务器将继续运行，但缓存功能可能受限...")
		fmt.Printf("Redis连接状态: 失败 - %v\n", err)
	} else {
		fmt.Println("Redis连接状态: 成功")
	}

	// 初始化异步处理系统
	handlers.InitAsync()
	fmt.Println("异步任务处理系统初始化成功")

	// 初始化文件存储
	if err := handlers.InitFileStorage(); err != nil {
		log.Printf("警告: 文件存储初始化失败: %v", err)
		log.Println("服务器将继续运行，但文件上传功能可能受限...")
		fmt.Printf("文件存储初始化状态: 失败 - %v\n", err)
	} else {
		fmt.Println("文件存储初始化状态: 成功")
	}

	// 设置handlers包的数据库连接
	handlers.SetDB(db)
	fmt.Println("数据库连接已传递给handlers包")
	if db != nil {
		handlers.ReloadAISettings(db)
		if err := handlers.EnsureExcelAlignedScreeningModels(db); err != nil {
			log.Printf("校准健康筛查/肠癌旧版 Excel 公式失败: %v", err)
		} else {
			fmt.Println("健康筛查/肠癌旧版 Excel 公式已校准")
		}
		if err := handlers.EnsureU5UrologyModels(db); err != nil {
			log.Printf("配置泌尿 U5 血液/尿液模型失败: %v", err)
		} else {
			fmt.Println("泌尿 U5 血液/尿液模型及阈值已配置")
		}
		if err := handlers.EnsureUrineReportPositionsMatchBlood(db); err != nil {
			log.Printf("复制尿液报告坐标失败: %v", err)
		} else {
			fmt.Println("尿液高敏/超敏报告坐标已校准")
		}
		handlers.EnsureDefaultStaff(db)
		handlers.StartSMSCodeCleanup(db)
		fmt.Println("默认员工账号与角色权限已校准")
	}

	// 设置handlers包的RSA密钥对
	handlers.SetRSAKeys(RSAKeyPair)
	fmt.Println("RSA密钥对已传递给handlers包")

	// 创建Hertz服务器
	h := server.New(
		server.WithHostPorts(":"+config.Port),
		server.WithDisablePrintRoute(true),
	)

	// 配置CORS中间件
	h.Use(func(c context.Context, ctx *app.RequestContext) {
		// 允许的来源
		ctx.Header("Access-Control-Allow-Origin", "*")
		// 允许的请求方法
		ctx.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		// 允许的请求头
		ctx.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, X-Miniapp-Session")
		// 允许携带凭证
		ctx.Header("Access-Control-Allow-Credentials", "true")
		// 预检请求缓存时间
		ctx.Header("Access-Control-Max-Age", "86400")

		// 如果是OPTIONS预检请求，直接返回200
		if string(ctx.Request.Method()) == "OPTIONS" {
			ctx.Status(consts.StatusOK)
			ctx.Abort()
			return
		}

		ctx.Next(c)
	})

	// 配置中间件
	h.Use(func(c context.Context, ctx *app.RequestContext) {
		// 日志中间件
		start := time.Now()
		ctx.Next(c)
		end := time.Now()
		log.Printf("%s %s %d %s", ctx.Request.Method(), ctx.Request.URI().Path(), ctx.Response.StatusCode(), end.Sub(start))
	})

	// Cookie验证中间件
	cookieAuth := func(c context.Context, ctx *app.RequestContext) {
		// 从Cookie获取session ID
		sessionID := ctx.Cookie("session_id")
		if string(sessionID) == "" {
			ctx.JSON(consts.StatusUnauthorized, utils.H{
				"code":    401,
				"success": false,
				"message": "未提供认证信息",
				"data":    nil,
			})
			ctx.Abort()
			return
		}

		// 从数据库验证session ID
		var userID int
		var expiry time.Time
		query := "SELECT user_id, expiry FROM base_sessions WHERE session_id = ?"
		err = db.QueryRow(query, sessionID).Scan(&userID, &expiry)
		if err != nil || time.Now().After(expiry) {
			// 删除过期的session
			if err == nil {
				db.Exec("DELETE FROM base_sessions WHERE session_id = ?", sessionID)
			}
			ctx.JSON(consts.StatusUnauthorized, utils.H{
				"code":    401,
				"success": false,
				"message": "无效的认证信息",
				"data":    nil,
			})
			ctx.Abort()
			return
		}

		// 将用户ID存储到上下文
		ctx.Set(UserIDKey, userID)
		ctx.Next(c)
	}

	// API路由组
	api := h.Group("/api")
	{
		// 健康检查
		api.GET("/health", func(c context.Context, ctx *app.RequestContext) {
			ctx.JSON(consts.StatusOK, utils.H{
				"code":    200,
				"success": true,
				"message": "OK",
				"data":    utils.H{"status": "healthy"},
			})
		})

		// 统计数据接口
		api.GET("/dashboard/stats", func(c context.Context, ctx *app.RequestContext) {
			// 获取时间筛选参数
			timeRange := string(ctx.Query("timeRange"))
			if timeRange == "" {
				timeRange = "all"
			}

			// 尝试从缓存获取统计数据
			cacheKey := "dashboard_stats_" + timeRange
			var cachedStats map[string]interface{}
			err := handlers.GetCache(cacheKey, &cachedStats)
			if err == nil {
				// 缓存命中，直接返回
				ctx.JSON(consts.StatusOK, utils.H{
					"code":    200,
					"success": true,
					"message": "获取统计数据成功",
					"data":    cachedStats,
				})
				return
			}

			// 合并查询，减少数据库连接开销
			var patientCount, sampleCount, resultCount, reportCount, pendingPatients, pendingReports, pendingReviews int

			// 根据时间范围构建查询条件
			timeCondition := ""
			switch timeRange {
			case "week":
				timeCondition = "AND created_at >= DATE_SUB(NOW(), INTERVAL 7 DAY)"
			case "day":
				timeCondition = "AND created_at >= DATE_SUB(NOW(), INTERVAL 1 DAY)"
			default:
				timeCondition = ""
			}

			// 获取患者总数
			db.QueryRow("SELECT COUNT(*) FROM detect_patient WHERE is_active = 1").Scan(&patientCount)

			// 获取样本总数
			if timeRange == "all" {
				db.QueryRow("SELECT COUNT(*) FROM detect_sample").Scan(&sampleCount)
			} else {
				db.QueryRow("SELECT COUNT(*) FROM detect_sample WHERE 1=1 " + timeCondition).Scan(&sampleCount)
			}

			// 获取结果总数（已检验的样本）
			if timeRange == "all" {
				db.QueryRow("SELECT COUNT(*) FROM detect_sample WHERE sample_status = 'tested' OR sample_status = 'completed'").Scan(&resultCount)
			} else {
				db.QueryRow("SELECT COUNT(*) FROM detect_sample WHERE (sample_status = 'tested' OR sample_status = 'completed') " + timeCondition).Scan(&resultCount)
			}

			// 获取报告总数
			if timeRange == "all" {
				db.QueryRow("SELECT COUNT(*) FROM detect_report").Scan(&reportCount)
			} else {
				db.QueryRow("SELECT COUNT(*) FROM detect_report WHERE 1=1 " + timeCondition).Scan(&reportCount)
			}

			// 获取待办任务数据
			db.QueryRow("SELECT COUNT(*) FROM detect_patient WHERE is_active = 1 AND completion_status = 0").Scan(&pendingPatients)

			if timeRange == "all" {
				db.QueryRow(`SELECT COUNT(*)
					FROM detect_sample s
					JOIN detect_batch b ON s.batch_id = b.id
					LEFT JOIN detect_report r ON r.sample_id = s.id AND r.status NOT IN ('draft', 'rejected')
					WHERE s.result_data IS NOT NULL AND b.status = 'submitted' AND r.id IS NULL`).Scan(&pendingReports)
			} else {
				db.QueryRow(`SELECT COUNT(*)
					FROM detect_sample s
					JOIN detect_batch b ON s.batch_id = b.id
					LEFT JOIN detect_report r ON r.sample_id = s.id AND r.status NOT IN ('draft', 'rejected')
					WHERE s.result_data IS NOT NULL AND b.status = 'submitted' AND r.id IS NULL ` + strings.ReplaceAll(timeCondition, "created_at", "s.created_at")).Scan(&pendingReports)
			}

			if timeRange == "all" {
				db.QueryRow("SELECT COUNT(*) FROM detect_report WHERE status IN ('pending', 'generated')").Scan(&pendingReviews)
			} else {
				db.QueryRow("SELECT COUNT(*) FROM detect_report WHERE status IN ('pending', 'generated') " + strings.ReplaceAll(timeCondition, "created_at", "created_at")).Scan(&pendingReviews)
			}

			// 构建统计数据
			stats := utils.H{
				"patients": patientCount,
				"samples":  sampleCount,
				"results":  resultCount,
				"reports":  reportCount,
				"todos": utils.H{
					"pendingPatients": pendingPatients,
					"pendingReports":  pendingReports,
					"pendingReviews":  pendingReviews,
				},
			}

			// 缓存统计数据，有效期5分钟
			handlers.SetCache(cacheKey, stats, 5*time.Minute)

			ctx.JSON(consts.StatusOK, utils.H{
				"code":    200,
				"success": true,
				"message": "获取统计数据成功",
				"data":    stats,
			})
		})

		// 获取公钥接口
		api.GET("/auth/publicKey", func(c context.Context, ctx *app.RequestContext) {
			if RSAKeyPair == nil {
				ctx.JSON(consts.StatusInternalServerError, utils.H{
					"code":    500,
					"success": false,
					"message": "RSA密钥对未初始化",
					"data":    nil,
				})
				return
			}

			publicKey, err := RSAKeyPair.GetPublicKeyPEM()
			if err != nil {
				ctx.JSON(consts.StatusInternalServerError, utils.H{
					"code":    500,
					"success": false,
					"message": "获取公钥失败",
					"data":    nil,
				})
				return
			}

			ctx.JSON(consts.StatusOK, utils.H{
				"code":    200,
				"success": true,
				"message": "获取公钥成功",
				"data":    utils.H{"publicKey": publicKey},
			})
		})

		// 认证相关路由
		auth := api.Group("/auth")
		{
			auth.POST("/login", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleLogin(ctx, db)
			})
			auth.GET("/me", cookieAuth, func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetMe(ctx, db)
			})
			auth.POST("/logout", cookieAuth, func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleLogout(ctx, db)
			})
			auth.POST("/switch-user", cookieAuth, func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleAdminSwitchUser(ctx, db)
			})
			auth.PUT("/changePassword", cookieAuth, func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleChangePassword(ctx, db)
			})
			auth.PUT("/update", cookieAuth, func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdateUser(ctx, db)
			})
			auth.PUT("/updateUsername", cookieAuth, func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdateUsername(ctx, db)
			})
			// 获取RSA公钥
			auth.GET("/public-key", func(c context.Context, ctx *app.RequestContext) {
				if RSAKeyPair == nil {
					ctx.JSON(consts.StatusInternalServerError, utils.H{
						"code":    500,
						"success": false,
						"message": "RSA密钥对未初始化",
						"data":    nil,
					})
					return
				}

				publicKeyPEM, err := RSAKeyPair.GetPublicKeyPEM()
				if err != nil {
					ctx.JSON(consts.StatusInternalServerError, utils.H{
						"code":    500,
						"success": false,
						"message": "获取公钥失败",
						"data":    nil,
					})
					return
				}

				ctx.JSON(consts.StatusOK, utils.H{
					"code":    200,
					"success": true,
					"message": "获取公钥成功",
					"data": utils.H{
						"publicKey": publicKeyPEM,
					},
				})
			})
			// 小程序登录相关路由
			auth.POST("/phone-identities", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandlePhoneIdentities(ctx, db)
			})
			auth.POST("/sms/send", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleSmsSend(ctx, db)
			})
			auth.POST("/sms/login", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleSmsLogin(ctx, db)
			})
			auth.POST("/bind-phone", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleBindPhone(ctx, db)
			})
			auth.POST("/oneclick/login", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleOneClickLogin(ctx, db)
			})
			auth.POST("/miniapp/switch-identity", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleMiniappSwitchIdentity(ctx, db)
			})
			auth.POST("/miniapp/register-patient", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleMiniappRegisterPatient(ctx, db)
			})
			auth.GET("/miniapp/check-id-card", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleMiniappCheckIdCard(ctx, db)
			})
			auth.GET("/miniapp/invite-manager", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetInviteManager(ctx, db)
			})
			auth.POST("/miniapp/invite-register", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleInviteRegister(ctx, db)
			})
		}

		// AI对话相关路由
		ai := api.Group("/ai")
		{
			ai.POST("/chat", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleAIChat(c, ctx)
			})
			ai.POST("/report-analysis", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleAIAnalyzeReport(c, ctx)
			})
		}

		// 患者管理相关路由
		patients := api.Group("/patients", cookieAuth)
		{
			patients.GET("", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleListPatients(ctx, db)
			})
			patients.GET("/trash", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleListPatientsTrash(ctx, db)
			})
			patients.GET("/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetPatientById(ctx, db)
			})
			patients.GET("/checkIdCard", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleCheckIdCard(ctx, db)
			})
			patients.POST("", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleCreatePatient(ctx, db)
			})
			patients.PUT("/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdatePatient(ctx, db)
			})
			patients.DELETE("/:id/report-files", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleDeletePatientReportFile(ctx, db)
			})
			patients.GET("/:id/report-files/preview", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetPatientReportPreviewURL(ctx, db)
			})
			patients.DELETE("/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleDeletePatient(ctx, db)
			})
			patients.PUT("/:id/restore", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleRestorePatient(ctx, db)
			})
			patients.DELETE("/:id/forceDelete", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleForceDeletePatient(ctx, db)
			})
			patients.DELETE("/:id/force-delete", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleForceDeletePatient(ctx, db)
			})
			// 文件上传路由
			patients.POST("/upload", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUploadFileToR2(ctx, db)
			})
		}

		// 样本管理相关路由
		samples := api.Group("/samples", cookieAuth)
		{
			samples.GET("", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetSamples(ctx, db)
			})
			samples.GET("/list-with-data", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetSamplesWithSystemData(ctx, db)
			})
			samples.GET("/barcode", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleSampleBarcode(ctx, db)
			})
			samples.POST("", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleCreateSample(ctx, db)
			})
			samples.POST("/allocate", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleAllocateSamples(ctx, db)
			})
			samples.PUT("/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdateSample(ctx, db)
			})
			samples.PUT("/:id/geneData", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdateGeneData(ctx, db)
			})
			samples.POST("/receive", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleSampleReceived(ctx, db)
			})
			samples.GET("/receive-preview", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleSampleReceivePreview(ctx, db)
			})
			samples.POST("/detect_batchReceive", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleBatchReceiveSamples(ctx, db)
			})
			samples.GET("/detect_batchReceive/template", func(c context.Context, ctx *app.RequestContext) {
				// 生成只包含样本编号列的Excel模板
				ctx.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
				ctx.Header("Content-Disposition", "attachment; filename=sample_detect_batch_receive_template.xlsx")

				// 这里使用简单的CSV格式作为模板，实际项目中可以使用Excel库生成真正的Excel文件
				csvContent := "样本编号\n"
				ctx.String(consts.StatusOK, csvContent)
			})
			samples.DELETE("/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleDeleteSample(ctx, db)
			})
		}

		// 结果管理相关路由
		results := api.Group("/results")
		{
			results.GET("", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleListResults(ctx, db)
			})
			results.GET("/patient/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetPatientResults(ctx, db)
			})
			results.GET("/patient/:id/compare", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetPatientResultsCompare(ctx, db)
			})
			results.POST("", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleCreateResult(ctx, db)
			})
			results.POST("/import", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleImportResults(ctx, db)
			})
			results.POST("/checkExisting", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleCheckExistingResults(ctx, db)
			})
			results.GET("/template/:cancerTypeId", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleDownloadTemplate(ctx, db)
			})
			results.POST("/generate", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGenerateResult(ctx, db)
			})
			// 箱线图数据路由
			results.GET("/boxplot", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetBoxplotData(ctx, db)
			})
			// 更新结果信号值路由
			results.POST("/update-signal", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdateResultSignalValue(ctx, db)
			})
			// 基因匹配路由
			results.POST("/match-genes", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleMatchGenes(ctx, db)
			})
		}

		// 报告管理相关路由
		reports := api.Group("/reports", cookieAuth)
		{
			reports.GET("/", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleListReports(ctx, db)
			})
			reports.GET("/samplesWithoutReports", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetSamplesWithoutReports(ctx, db)
			})
			reports.GET("/pendingReview", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetPendingReviewReports(ctx, db)
			})
			reports.GET("/patient/:patientId", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetPatientHistoricalReports(ctx, db)
			})
			reports.GET("/concise-test-pdf", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleDownloadConciseTestPdf(ctx, db)
			})
			reports.GET("/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetReportById(ctx, db)
			})
			reports.POST("/generate", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGenerateReport(ctx, db)
			})

			reports.POST("/batch-generate", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleBatchGenerateReports(ctx, db)
			})

			reports.POST("/batch-download", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleBatchDownloadReports(ctx, db)
			})

			reports.PUT("/review/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleReviewReport(ctx, db)
			})
			reports.GET("/patient/download/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleDownloadPatientReport(ctx, db)
			})
			// 获取报告PDF生成状态
			reports.GET("/pdfStatus/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetReportPdfStatus(ctx, db)
			})
			reports.GET("/:id/pdf/status", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetReportPdfStatus(ctx, db)
			})
			// 更新报告状态
			reports.PUT("/status/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdateReportStatus(ctx, db)
			})
			// 更新报告
			reports.PUT("/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdateReport(ctx, db)
			})
			// 删除报告
			reports.DELETE("/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleDeleteReport(ctx, db)
			})
			// 模板管理相关路由
			reports.GET("/templates", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetTemplates(ctx, db)
			})
			reports.POST("/templates", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleCreateTemplate(ctx, db)
			})
			reports.PUT("/templates/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdateTemplate(ctx, db)
			})
			reports.DELETE("/templates/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleDeleteTemplate(ctx, db)
			})
			// PDF预览和重新生成路由
			reports.GET("/:id/pdf/preview", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandlePreviewReportPdf(ctx, db)
			})
			reports.POST("/:id/pdf/regenerate", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleRegenerateReportPdf(ctx, db)
			})
			// 报告预览数据路由
			reports.GET("/:id/preview-data", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetReportPreviewData(ctx, db)
			})
			// PDF下载路由（GET方式，现场生成PDF）
			reports.GET("/:id/pdf/download", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleDownloadReportPdf(ctx, db)
			})
			// 一次性下载链接生成路由
			reports.POST("/:id/download", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGenerateReportDownloadURL(ctx, db)
			})
		}

		// 一次性下载路由（不需要认证）
		api.GET("/downloads/:token", func(c context.Context, ctx *app.RequestContext) {
			handlers.HandleOneTimeDownload(ctx)
		})

		// 公式计算相关路由
		formula := api.Group("/formula", cookieAuth)
		{
			formula.POST("/calculate", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleFormulaCalculate(ctx)
			})
			formula.POST("/modelCalculate", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleModelFormulaCalculate(ctx, db)
			})
		}

		// 系统设置相关路由
		system := api.Group("/system", cookieAuth)
		{
			system.GET("/bootstrap", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleSystemBootstrap(ctx, db)
			})
			// 样本类型
			system.GET("/sampleTypes", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetSampleTypes(ctx, db)
			})
			// 样本类型列表（兼容前端调用）
			system.GET("/sampleTypes/list", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetSampleTypes(ctx, db)
			})
			// 治疗阶段
			system.GET("/treatmentStages", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetTreatmentStages(ctx, db)
			})
			// 癌症类型
			system.GET("/cancerTypes/list", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleListCancerTypes(ctx, db)
			})
			system.GET("/cancerTypes/selectable/:currentCancerTypeId", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetSelectableCancerTypes(ctx, db)
			})
			system.POST("/cancerTypes", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleCreateCancerType(ctx, db)
			})
			system.PUT("/cancerTypes/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdateCancerType(ctx, db)
			})
			system.DELETE("/cancerTypes/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleDeleteCancerType(ctx, db)
			})
			// 样本类型
			system.POST("/sampleTypes", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleCreateSampleType(ctx, db)
			})
			system.PUT("/sampleTypes/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdateSampleType(ctx, db)
			})
			system.DELETE("/sampleTypes/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleDeleteSampleType(ctx, db)
			})
			// 模型设置
			system.GET("/modelSettings", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleListModels(ctx, db)
			})
			system.POST("/modelSettings", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleCreateModel(ctx, db)
			})
			system.PUT("/modelSettings/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdateModel(ctx, db)
			})
			system.GET("/modelSettings/:id/geneThresholds", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetModelGeneThresholds(ctx, db)
			})
			system.PUT("/modelSettings/:id/geneThresholds", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdateModelGeneThresholds(ctx, db)
			})
			system.DELETE("/modelSettings/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleDeleteModel(ctx, db)
			})
			// 基因设置
			system.GET("/geneSettings", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleListGenes(ctx, db)
			})
			system.POST("/geneSettings", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleCreateGene(ctx, db)
			})
			system.PUT("/geneSettings/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdateGene(ctx, db)
			})
			system.PUT("/geneSettings/:id/panels", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdateGenePanels(ctx, db)
			})
			system.DELETE("/geneSettings/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleDeleteGene(ctx, db)
			})
			// Panel设置
			system.GET("/panels", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleListPanels(ctx, db)
			})
			system.POST("/panels", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleCreatePanel(ctx, db)
			})
			system.PUT("/panels/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdatePanel(ctx, db)
			})
			system.DELETE("/panels/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleDeletePanel(ctx, db)
			})
			system.GET("/panels/:id/genes", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetPanelGenes(ctx, db)
			})
			system.PUT("/panels/:id/genes", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdatePanelGenes(ctx, db)
			})
			// 部门设置
			system.GET("/departments", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleListDepartments(ctx, db)
			})
			system.GET("/departments/tree", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleListDepartmentsTree(ctx, db)
			})
			system.POST("/departments", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleCreateDepartment(ctx, db)
			})
			system.PUT("/departments/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdateDepartment(ctx, db)
			})
			system.DELETE("/departments/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleDeleteDepartment(ctx, db)
			})
			system.PUT("/departments/:id/status", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdateDepartmentStatus(ctx, db)
			})
			// 角色设置
			system.GET("/roles", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleListRoles(ctx, db)
			})
			system.POST("/roles", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleCreateRole(ctx, db)
			})
			system.PUT("/roles/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdateRole(ctx, db)
			})
			system.DELETE("/roles/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleDeleteRole(ctx, db)
			})
			system.GET("/roles/:id/permissions", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetRolePermissions(ctx, db)
			})
			system.PUT("/roles/:id/status", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdateRoleStatus(ctx, db)
			})
			// 系统配置
			system.GET("/settings", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetSystemSettings(ctx, db)
			})
			system.PUT("/settings", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdateSystemSettings(ctx, db)
			})
			system.GET("/storage/qiniu/overview", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetQiniuStorageOverview(ctx)
			})
			system.GET("/version", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetSystemVersion(ctx)
			})
			system.POST("/version/upgrade", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpgradeSystem(ctx)
			})
			system.GET("/sms/packages", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetSMSPackages(ctx, db)
			})
			system.GET("/sms/templates", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetSMSTemplates(ctx, db)
			})
			system.GET("/sms/logs", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetSMSLogs(ctx, db)
			})
			system.GET("/help-center", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetHelpCenterSetting(ctx, db)
			})
			system.PUT("/help-center", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdateHelpCenterSetting(ctx, db)
			})
			system.GET("/report-positions", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleListReportPositions(ctx, db)
			})
			system.PUT("/report-positions/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdateReportPosition(ctx, db)
			})

			// 文件存储配置
			system.GET("/fileStorage", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetFileStorageConfig(ctx)
			})
			system.PUT("/fileStorage", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdateFileStorageConfig(ctx)
			})

			// 用户管理（兼容前端调用）
			system.GET("/users", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleListUsers(ctx, db)
			})
			system.POST("/users", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleCreateUser(ctx, db)
			})
			system.PUT("/users/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdateSystemUser(ctx, db)
			})
			system.POST("/users/:id/reset-password", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleResetUserPassword(ctx, db)
			})
			system.DELETE("/users/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleDeleteUser(ctx, db)
			})
			system.PUT("/users/:id/status", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdateUserStatus(ctx, db)
			})
			system.PUT("/users/:id/ai-allowed", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdateUserAIAccess(ctx, db)
			})
			system.GET("/users/:id/permissions", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetUserPermissions(ctx, db)
			})
			system.PUT("/users/:id/permissions", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdateUserPermissions(ctx, db)
			})
			system.DELETE("/users/:id/permissions", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleClearUserPermissions(ctx, db)
			})

			// 缓存管理
			system.POST("/cache/clear", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleClearCache(ctx)
			})

			// AI管理相关路由
			system.GET("/ai-settings", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetAISettings(ctx, db)
			})
			system.PUT("/ai-settings", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdateAISettings(ctx, db)
			})
			system.GET("/ai-usage", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetAIUsage(ctx, db)
			})
			system.GET("/ai-blacklist", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleListAIBlacklist(ctx, db)
			})
			system.POST("/ai-blacklist", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleCreateAIBlacklist(ctx, db)
			})
			system.DELETE("/ai-blacklist/:code", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleDeleteAIBlacklist(ctx, db)
			})
		}

		// 销售模组相关路由
		sales := api.Group("/sales", cookieAuth)
		{
			// 套餐管理
			sales.GET("/packages", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleListPackages(ctx, db)
			})
			sales.POST("/packages", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleCreatePackage(ctx, db)
			})
			sales.PUT("/packages", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdatePackage(ctx, db)
			})
			sales.GET("/patient-packages", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleListPatientPackages(ctx, db)
			})
			sales.POST("/patient-packages", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleBindPatientPackage(ctx, db)
			})
			sales.GET("/assignment-patients", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleListSalesAssignmentPatients(ctx, db)
			})
			sales.POST("/assign-patient", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleAssignSalesToSelfRegisteredPatient(ctx, db)
			})

			// 订单管理
			sales.GET("/orders", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleListOrders(ctx, db)
			})
			sales.POST("/orders", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleCreateOrder(ctx, db)
			})

			// 检测计划管理
			sales.GET("/detectionPlans", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleListDetectionPlans(ctx, db)
			})
			sales.PUT("/detectionPlans/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdateDetectionPlan(ctx, db)
			})

			// 销售统计
			sales.GET("/statistics", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetSalesStatistics(ctx, db)
			})
		}

		// 批次管理相关路由
		detect_batch := api.Group("/detect_batch", cookieAuth)
		{
			detect_batch.POST("/import", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleBatchImport(ctx, db)
			})
			detect_batch.GET("/list", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleBatchList(ctx, db)
			})
			detect_batch.GET("/search", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleBatchSearchByPatient(ctx, db)
			})
			detect_batch.PUT("/status/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdateBatchStatus(ctx, db)
			})
			detect_batch.DELETE("/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleDeleteBatch(ctx, db)
			})
			detect_batch.GET("/export/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleExportBatch(ctx, db)
			})
			detect_batch.POST("/match-genes", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleMatchGenes(ctx, db)
			})
			detect_batch.POST("/apply-model", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleApplyModel(ctx, db)
			})
			detect_batch.POST("/submit/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleSubmitBatch(ctx, db)
			})
			detect_batch.POST("/deleteSample", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleDeleteSampleFromBatch(ctx, db)
			})
		}

		// 快递运单管理相关路由
		express := api.Group("/express", cookieAuth)
		{
			express.POST("/create", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleCreateExpress(ctx, db)
			})
			express.GET("/:sampleId", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetExpress(ctx, db)
			})
			express.PUT("/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdateExpress(ctx, db)
			})
			express.DELETE("/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleDeleteExpress(ctx, db)
			})
		}

		// 兼容前端的批次管理路由
		batch := api.Group("/batch", cookieAuth)
		{
			batch.GET("/list", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleBatchList(ctx, db)
			})
			batch.POST("/import", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleBatchImport(ctx, db)
			})
			batch.GET("/detail/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleBatchDetail(ctx, db)
			})
			batch.GET("/samples/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleBatchSamples(ctx, db)
			})
			batch.DELETE("/submitted/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleResetSubmittedBatch(ctx, db)
			})
			batch.POST("/submitted/:id/partial-reset", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandlePartialResetSubmittedBatch(ctx, db)
			})
			batch.DELETE("/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleDeleteBatch(ctx, db)
			})
			batch.PUT("/status/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdateBatchStatus(ctx, db)
			})
			batch.GET("/export/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleExportBatch(ctx, db)
			})
			batch.POST("/submit/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleSubmitBatch(ctx, db)
			})
			batch.GET("/duplicates/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetBatchDuplicateSamples(ctx, db)
			})
			batch.POST("/duplicates/retest", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleCreateBatchRetestSamples(ctx, db)
			})
			batch.POST("/gene-matches", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleApplyBatchGeneMatches(ctx, db)
			})
			batch.POST("/deleteSample", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleDeleteSampleFromBatch(ctx, db)
			})
			// 多平台批次管理路由
			batch.POST("/multi-upload", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleBatchUploadMultiple(ctx, db)
			})
			batch.GET("/multi-detail/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleBatchDetailMultiPlatform(ctx, db)
			})
			batch.POST("/merge-data", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleMergeSampleData(ctx, db)
			})
			batch.PUT("/cancerType/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdateBatchCancerType(ctx, db)
			})
			// 按样本设置检测类型路由
			batch.PUT("/sampleCancerType/:batchId", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUpdateSampleCancerType(ctx, db)
			})
			// 自动匹配检测类型路由
			batch.POST("/auto-match-cancer-type/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleAutoMatchCancerType(ctx, db)
			})
			// 设置样本接收时间路由
			batch.POST("/set-sample-receive-date", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleSetSampleReceiveDate(ctx, db)
			})
			// 批量设置样本接收时间路由
			batch.POST("/batch-set-sample-receive-date", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleBatchSetSampleReceiveDate(ctx, db)
			})
		}

		appointments := api.Group("/appointments", cookieAuth)
		{
			appointments.GET("", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleAdminListMailAppointments(ctx, db)
			})
			appointments.PUT("/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleAdminUpdateMailAppointment(ctx, db)
			})
			appointments.POST("/tracking-upload", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleAdminUploadAppointmentTracking(ctx, db)
			})
		}

		// 小程序专用中间件 - 基于 base_miniapp_sessions 验证
		miniappAuth := func(c context.Context, ctx *app.RequestContext) {
			sessionID := ctx.Cookie("miniapp_session_id")
			// 也支持通过 header 传递
			if string(sessionID) == "" {
				sessionID = ctx.GetHeader("X-Miniapp-Session")
			}
			if string(sessionID) == "" {
				ctx.JSON(consts.StatusUnauthorized, utils.H{
					"code":    401,
					"success": false,
					"message": "未提供认证信息",
					"data":    nil,
				})
				ctx.Abort()
				return
			}

			var phone string
			var identityType string
			var userID int
			var patientID int
			query := "SELECT phone, identity_type, COALESCE(user_id, 0), COALESCE(patient_id, 0) FROM base_miniapp_sessions WHERE session_id = ? AND expiry > NOW()"
			err := db.QueryRow(query, sessionID).Scan(&phone, &identityType, &userID, &patientID)
			if err != nil {
				ctx.JSON(consts.StatusUnauthorized, utils.H{
					"code":    401,
					"success": false,
					"message": "无效的认证信息",
					"data":    nil,
				})
				ctx.Abort()
				return
			}

			ctx.Set("miniapp_phone", phone)
			ctx.Set("miniapp_identity_type", identityType)
			ctx.Set("miniapp_user_id", userID)
			ctx.Set("miniapp_patient_id", patientID)
			ctx.Next(c)
		}

		// 小程序专用路由组
		uni := api.Group("/uni", miniappAuth)
		{
			// 患者信息
			uni.GET("/patient/info", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniGetPatientInfo(ctx, db)
			})
			uni.PUT("/patient/info", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniUpdatePatientInfo(ctx, db)
			})

			// 检测计划/预约
			uni.GET("/detection-plans", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniGetDetectionPlans(ctx, db)
			})
			uni.GET("/packages", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniGetMyPackages(ctx, db)
			})
			uni.POST("/sample-box-request", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniCreateSampleBoxRequest(ctx, db)
			})
			uni.GET("/sample-box-requests", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniGetSampleBoxRequests(ctx, db)
			})
			uni.GET("/patient/manager", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniGetPatientManager(ctx, db)
			})

			// 报告
			uni.GET("/reports", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniGetReports(ctx, db)
			})
			uni.GET("/reports/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniGetReportDetail(ctx, db)
			})
			uni.GET("/reports/:id/pdf/download", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniDownloadReportPDF(ctx, db)
			})
			uni.GET("/reports/:id/preview-image", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniGetReportPreviewImage(ctx, db)
			})

			// 样本
			uni.GET("/samples", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniGetSamples(ctx, db)
			})
			uni.GET("/samples/:id/express", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniGetSampleExpress(ctx, db)
			})

			// 邮寄样本
			uni.GET("/mail-samples", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniGetMailSamples(ctx, db)
			})
			uni.POST("/mail-sample", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniCreateMailSample(ctx, db)
			})

			// 随访管理
			uni.GET("/follow-ups", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniListFollowUps(ctx, db)
			})
			uni.POST("/follow-ups", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniCreateFollowUp(ctx, db)
			})

			// 帮助中心
			uni.GET("/help-center", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniGetHelpCenter(ctx, db)
			})

			// 员工专用接口
			uni.GET("/employee/stats", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniEmployeeStats(ctx, db)
			})
			uni.GET("/employee/reports", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniEmployeeReports(ctx, db)
			})
			uni.GET("/employee/patients", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniEmployeePatients(ctx, db)
			})
			uni.GET("/employee/patient-groups", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniEmployeePatientGroups(ctx, db)
			})
			uni.POST("/employee/patient-groups", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniEmployeeCreatePatientGroup(ctx, db)
			})
			uni.DELETE("/employee/patient-groups/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniEmployeeDeletePatientGroup(ctx, db)
			})
			uni.PUT("/employee/patients/:id/group", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniEmployeeSetPatientGroup(ctx, db)
			})
			uni.GET("/employee/patients/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniEmployeePatientDetail(ctx, db)
			})
			uni.POST("/employee/patients/:id/completion", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniEmployeeCompletePatient(ctx, db)
			})
			uni.POST("/employee/patient-report/upload", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniEmployeeReportFileUpload(ctx, db)
			})
			uni.POST("/employee/patients", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniEmployeeCreatePatient(ctx, db)
			})
			uni.GET("/employee/sample-options", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniEmployeeSampleOptions(ctx, db)
			})
			uni.GET("/employee/reports/pending", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniEmployeePendingReports(ctx, db)
			})
			uni.GET("/employee/report-reviewers", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniEmployeeReportReviewers(ctx, db)
			})
			uni.PUT("/employee/reports/:id/review", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniEmployeeReviewReport(ctx, db)
			})
			uni.GET("/employee/samples/pending", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniEmployeePendingSamples(ctx, db)
			})
			uni.GET("/employee/samples", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniEmployeeSamples(ctx, db)
			})
			uni.GET("/employee/samples/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniEmployeeSampleDetail(ctx, db)
			})
			uni.DELETE("/employee/samples/:id", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniEmployeeDeleteSample(ctx, db)
			})
			uni.POST("/employee/samples/allocate", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniEmployeeAllocateSamples(ctx, db)
			})
			uni.POST("/employee/samples/receive", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniEmployeeReceiveSample(ctx, db)
			})
			uni.POST("/employee/samples/batch-receive", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniEmployeeBatchReceiveSamples(ctx, db)
			})
			uni.GET("/employee/invite-code", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUniEmployeeInviteCode(ctx, db)
			})
		}

		// 公告管理相关路由
		announcements := api.Group("/announcements")
		{
			// 获取公告列表（不需要认证）
			announcements.GET("", func(c context.Context, ctx *app.RequestContext) {
				// 查询公告列表，按发布时间倒序
				query := `
					SELECT a.id, a.title, a.content, COALESCE(a.user_id, 0), a.created_at, a.updated_at,
					       COALESCE(NULLIF(u.real_name, ''), NULLIF(u.username, ''), '管理员') AS publisher
					FROM base_announcements a
					LEFT JOIN base_manage_user u ON u.id = a.user_id
					ORDER BY a.created_at DESC`
				rows, err := db.Query(query)
				if err != nil {
					ctx.JSON(consts.StatusInternalServerError, utils.H{
						"code":    500,
						"success": false,
						"message": "获取公告列表失败",
						"data":    nil,
					})
					return
				}
				defer rows.Close()

				var announcements []utils.H
				for rows.Next() {
					var id, userID int
					var title, content string
					var createdAt, updatedAt time.Time
					var publisher sql.NullString
					err := rows.Scan(&id, &title, &content, &userID, &createdAt, &updatedAt, &publisher)
					if err != nil {
						ctx.JSON(consts.StatusInternalServerError, utils.H{
							"code":    500,
							"success": false,
							"message": "扫描公告数据失败",
							"data":    nil,
						})
						return
					}
					announcements = append(announcements, utils.H{
						"id":         id,
						"title":      title,
						"content":    content,
						"user_id":    userID,
						"user_name":  publisher.String,
						"publisher":  publisher.String,
						"created_at": createdAt,
						"updated_at": updatedAt,
					})
				}

				ctx.JSON(consts.StatusOK, utils.H{
					"code":    200,
					"success": true,
					"message": "获取公告列表成功",
					"data":    announcements,
				})
			})

			// 获取单个公告详情（不需要认证）
			announcements.GET("/:id", func(c context.Context, ctx *app.RequestContext) {
				// 获取公告ID
				id := ctx.Param("id")

				// 查询公告详情
				query := `
					SELECT a.id, a.title, a.content, COALESCE(a.user_id, 0), a.created_at, a.updated_at,
					       COALESCE(NULLIF(u.real_name, ''), NULLIF(u.username, ''), '管理员') AS publisher
					FROM base_announcements a
					LEFT JOIN base_manage_user u ON u.id = a.user_id
					WHERE a.id = ?`
				var announcement utils.H
				var announcementID, userID int
				var title, content string
				var createdAt, updatedAt time.Time
				var publisher sql.NullString
				err := db.QueryRow(query, id).Scan(&announcementID, &title, &content, &userID, &createdAt, &updatedAt, &publisher)
				if err != nil {
					ctx.JSON(consts.StatusNotFound, utils.H{
						"code":    404,
						"success": false,
						"message": "公告不存在",
						"data":    nil,
					})
					return
				}

				announcement = utils.H{
					"id":         announcementID,
					"title":      title,
					"content":    content,
					"user_id":    userID,
					"user_name":  publisher.String,
					"publisher":  publisher.String,
					"created_at": createdAt,
					"updated_at": updatedAt,
				}

				ctx.JSON(consts.StatusOK, utils.H{
					"code":    200,
					"success": true,
					"message": "获取公告详情成功",
					"data":    announcement,
				})
			})

			// 公告管理操作（需要认证）
			announcementsAuth := announcements.Group("", cookieAuth)
			{
				// 创建公告
				announcementsAuth.POST("", func(c context.Context, ctx *app.RequestContext) {
					// 获取用户ID
					userID, exists := ctx.Get(UserIDKey)
					if !exists {
						ctx.JSON(consts.StatusUnauthorized, utils.H{
							"code":    401,
							"success": false,
							"message": "未授权",
							"data":    nil,
						})
						return
					}

					// 解析请求体
					var req struct {
						Title   string `json:"title"`
						Content string `json:"content"`
					}
					if err := ctx.Bind(&req); err != nil {
						ctx.JSON(consts.StatusBadRequest, utils.H{
							"code":    400,
							"success": false,
							"message": "请求参数错误",
							"data":    nil,
						})
						return
					}

					// 创建公告
					query := "INSERT INTO base_announcements (title, content, user_id) VALUES (?, ?, ?)"
					result, err := db.Exec(query, req.Title, req.Content, userID)
					if err != nil {
						ctx.JSON(consts.StatusInternalServerError, utils.H{
							"code":    500,
							"success": false,
							"message": "创建公告失败",
							"data":    nil,
						})
						return
					}

					// 获取插入的ID
					id, err := result.LastInsertId()
					if err != nil {
						ctx.JSON(consts.StatusInternalServerError, utils.H{
							"code":    500,
							"success": false,
							"message": "获取公告ID失败",
							"data":    nil,
						})
						return
					}

					ctx.JSON(consts.StatusOK, utils.H{
						"code":    200,
						"success": true,
						"message": "创建公告成功",
						"data":    utils.H{"id": id},
					})
				})

				// 编辑公告
				announcementsAuth.PUT("/:id", func(c context.Context, ctx *app.RequestContext) {
					// 获取公告ID
					id := ctx.Param("id")

					// 解析请求体
					var req struct {
						Title   string `json:"title"`
						Content string `json:"content"`
					}
					if err := ctx.Bind(&req); err != nil {
						ctx.JSON(consts.StatusBadRequest, utils.H{
							"code":    400,
							"success": false,
							"message": "请求参数错误",
							"data":    nil,
						})
						return
					}

					// 更新公告
					query := "UPDATE base_announcements SET title = ?, content = ? WHERE id = ?"
					_, err := db.Exec(query, req.Title, req.Content, id)
					if err != nil {
						ctx.JSON(consts.StatusInternalServerError, utils.H{
							"code":    500,
							"success": false,
							"message": "更新公告失败",
							"data":    nil,
						})
						return
					}

					ctx.JSON(consts.StatusOK, utils.H{
						"code":    200,
						"success": true,
						"message": "更新公告成功",
						"data":    nil,
					})
				})

				// 删除公告
				announcementsAuth.DELETE("/:id", func(c context.Context, ctx *app.RequestContext) {
					// 获取公告ID
					id := ctx.Param("id")

					// 删除公告
					query := "DELETE FROM base_announcements WHERE id = ?"
					_, err := db.Exec(query, id)
					if err != nil {
						ctx.JSON(consts.StatusInternalServerError, utils.H{
							"code":    500,
							"success": false,
							"message": "删除公告失败",
							"data":    nil,
						})
						return
					}

					ctx.JSON(consts.StatusOK, utils.H{
						"code":    200,
						"success": true,
						"message": "删除公告成功",
						"data":    nil,
					})
				})

				// 上传公告文件
				announcementsAuth.POST("/upload", func(c context.Context, ctx *app.RequestContext) {
					handlers.HandleUploadFileToR2(ctx, db)
				})
			}
		}

		// 文件管理相关路由
		file := api.Group("/file")
		{
			// 原有的文件访问路由（需要认证）
			fileAuth := file.Group("", cookieAuth)
			{
				fileAuth.GET("/view", func(c context.Context, ctx *app.RequestContext) {
					handlers.HandleViewFile(ctx)
				})
				fileAuth.GET("/download", func(c context.Context, ctx *app.RequestContext) {
					handlers.HandleDownloadFile(ctx)
				})
				// 生成临时文件URL
				fileAuth.GET("/generateTempURL", func(c context.Context, ctx *app.RequestContext) {
					handlers.HandleGenerateTempFileURL(ctx)
				})
				// 临时文件访问
				fileAuth.GET("/temp/:id", func(c context.Context, ctx *app.RequestContext) {
					handlers.HandleTempFileAccess(ctx)
				})
			}

			// 新的安全文件访问路由（不需要认证，使用token验证）
			file.GET("/view/:path", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleSecureViewFile(ctx)
			})
			file.GET("/download/:path", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleSecureDownloadFile(ctx)
			})

		}

		// 文件上传相关路由
		upload := api.Group("/upload", cookieAuth)
		{
			upload.POST("/token", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleGetQiniuUploadToken(ctx, db)
			})
			// 图片上传
			upload.POST("/image", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUploadFileToR2(ctx, db)
			})
			// 视频上传
			upload.POST("/video", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUploadFileToR2(ctx, db)
			})
			// 附件上传
			upload.POST("/attachment", func(c context.Context, ctx *app.RequestContext) {
				handlers.HandleUploadFileToR2(ctx, db)
			})
		}
	}

	// 前端静态文件服务。优先使用可执行文件所在目录下的 static，避免 systemd/宝塔
	// 启动工作目录不同导致明明部署了文件却按另一个 ./static 查找。
	frontendDist := "./static"
	frontendStaticDirs := make([]string, 0, 2)
	addFrontendStaticDir := func(dir string) {
		absDir, absErr := filepath.Abs(dir)
		if absErr != nil {
			absDir = dir
		}
		for _, existing := range frontendStaticDirs {
			if existing == absDir {
				return
			}
		}
		if info, statErr := os.Stat(absDir); statErr == nil && info.IsDir() {
			frontendStaticDirs = append(frontendStaticDirs, absDir)
		}
	}
	if exePath, err := os.Executable(); err == nil {
		exeStatic := filepath.Join(filepath.Dir(exePath), "static")
		if _, statErr := os.Stat(exeStatic); statErr == nil {
			frontendDist = exeStatic
		}
		addFrontendStaticDir(exeStatic)
	}
	addFrontendStaticDir("./static")
	if len(frontendStaticDirs) == 0 {
		addFrontendStaticDir(frontendDist)
	}
	if len(frontendStaticDirs) > 0 {
		frontendDist = frontendStaticDirs[0]
	}
	resolveFrontendFile := func(requestPath string) (string, os.FileInfo, []string) {
		relPath := strings.TrimPrefix(requestPath, "/")
		if relPath == "" {
			relPath = "index.html"
		}
		relPath = filepath.Clean(filepath.FromSlash(relPath))
		if relPath == "." {
			relPath = "index.html"
		}
		if relPath == ".." || strings.HasPrefix(relPath, ".."+string(os.PathSeparator)) || filepath.IsAbs(relPath) {
			return "", nil, []string{fmt.Sprintf("blocked unsafe path: %s", requestPath)}
		}
		attempts := make([]string, 0, len(frontendStaticDirs))
		for _, staticDir := range frontendStaticDirs {
			filePath := filepath.Join(staticDir, relPath)
			fileInfo, err := os.Stat(filePath)
			if err == nil {
				return filePath, fileInfo, attempts
			}
			attempts = append(attempts, fmt.Sprintf("%s (%v)", filePath, err))
		}
		return "", nil, attempts
	}
	// 模板文件服务
	templateDir := "./Template"

	// 专门为PDF.js worker文件添加路由
	h.GET("/statics/pdfjs_dist/pdf.worker.min.mjs", func(c context.Context, ctx *app.RequestContext) {
		workerFilePath, _, attempts := resolveFrontendFile("/statics/pdfjs_dist/pdf.worker.min.mjs")
		if workerFilePath != "" {
			// 文件存在，直接提供
			ctx.Header("Content-Type", "application/javascript")
			ctx.File(workerFilePath)
		} else {
			log.Printf("PDF worker not found: attempts=%v", attempts)
			ctx.JSON(consts.StatusNotFound, utils.H{
				"code":    404,
				"success": false,
				"message": "PDF worker文件不存在",
				"data":    nil,
			})
		}
	})

	// 模板文件服务路由
	h.GET("/Template/*filepath", func(c context.Context, ctx *app.RequestContext) {
		filepathParam := ctx.Param("filepath")
		filePath := filepath.Join(templateDir, filepathParam)
		if _, err := os.Stat(filePath); err == nil {
			// 设置正确的Content-Type
			if strings.HasSuffix(filePath, ".pdf") {
				ctx.Header("Content-Type", "application/pdf")
			} else if strings.HasSuffix(filePath, ".png") {
				ctx.Header("Content-Type", "image/png")
			} else if strings.HasSuffix(filePath, ".jpg") || strings.HasSuffix(filePath, ".jpeg") {
				ctx.Header("Content-Type", "image/jpeg")
			} else if strings.HasSuffix(filePath, ".gif") {
				ctx.Header("Content-Type", "image/gif")
			} else if strings.HasSuffix(filePath, ".svg") {
				ctx.Header("Content-Type", "image/svg+xml")
			}
			// 提供文件
			ctx.File(filePath)
		} else {
			ctx.JSON(consts.StatusNotFound, utils.H{
				"code":    404,
				"success": false,
				"message": "模板文件不存在",
				"data":    nil,
			})
		}
	})

	h.GET("/uploads/*filepath", func(c context.Context, ctx *app.RequestContext) {
		filepathParam := ctx.Param("filepath")
		filePath := filepath.Join("uploads", filepathParam)
		if _, err := os.Stat(filePath); err == nil {
			ctx.File(filePath)
			return
		}
		ctx.JSON(consts.StatusNotFound, utils.H{
			"code":    404,
			"success": false,
			"message": "上传文件不存在",
			"data":    nil,
		})
	})

	// 检查static目录是否存在
	if _, err := os.Stat(frontendDist); os.IsNotExist(err) {
		log.Printf("前端static目录不存在: %v", err)
		fmt.Printf("前端static目录状态: 不存在 - %v\n", err)
	} else {
		fmt.Printf("前端static目录状态: 存在 - %s\n", frontendDist)
		fmt.Printf("前端static候选目录: %v\n", frontendStaticDirs)
	}

	// 检查Template目录是否存在
	if _, err := os.Stat(templateDir); os.IsNotExist(err) {
		log.Printf("Template目录不存在: %v", err)
		fmt.Printf("Template目录状态: 不存在 - %v\n", err)
	} else {
		fmt.Println("Template目录状态: 存在")
	}

	// 配置静态文件服务的缓存中间件
	isFrontendAssetRequest := func(path string) bool {
		return strings.HasPrefix(path, "/static/") ||
			strings.HasPrefix(path, "/statics/") ||
			strings.HasPrefix(path, "/Template/") ||
			strings.HasPrefix(path, "/uploads/") ||
			strings.HasSuffix(path, ".js") ||
			strings.HasSuffix(path, ".css") ||
			strings.HasSuffix(path, ".png") ||
			strings.HasSuffix(path, ".jpg") ||
			strings.HasSuffix(path, ".jpeg") ||
			strings.HasSuffix(path, ".gif") ||
			strings.HasSuffix(path, ".svg") ||
			strings.HasSuffix(path, ".ico") ||
			strings.HasSuffix(path, ".map") ||
			strings.HasSuffix(path, ".mjs")
	}

	h.Use(func(c context.Context, ctx *app.RequestContext) {
		// 为静态文件添加缓存控制头
		path := string(ctx.Request.URI().Path())
		if isFrontendAssetRequest(path) {
			// 静态资源缓存3天
			ctx.Header("Cache-Control", "public, max-age=259200")
		} else if strings.HasSuffix(path, ".html") || !strings.HasPrefix(path, "/api/") {
			// SPA入口必须每次校验，避免旧入口继续请求已替换的chunk
			ctx.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			ctx.Header("Pragma", "no-cache")
			ctx.Header("Expires", "0")
		}
		ctx.Next(c)
	})

	// 提供静态文件服务，使用文件服务器中间件
	h.NoRoute(func(c context.Context, ctx *app.RequestContext) {
		// 检查是否是API请求
		if strings.HasPrefix(string(ctx.Request.URI().Path()), "/api/") {
			ctx.JSON(consts.StatusNotFound, utils.H{
				"code":    404,
				"success": false,
				"message": "API路径不存在",
				"data":    nil,
			})
			return
		}

		// 尝试直接提供文件
		requestPath := string(ctx.Request.URI().Path())
		filePath, fileInfo, attempts := resolveFrontendFile(requestPath)
		if filePath != "" {
			// 检查是否是目录
			if fileInfo.IsDir() {
				// 如果是目录，返回index.html
				if indexPath, _, _ := resolveFrontendFile("/index.html"); indexPath != "" {
					ctx.File(indexPath)
					return
				}
				ctx.JSON(consts.StatusNotFound, utils.H{
					"code":    404,
					"success": false,
					"message": "前端入口文件不存在",
					"data":    nil,
				})
				return
			}
			// 文件存在，设置正确的Content-Type
			if strings.HasSuffix(requestPath, ".mjs") {
				ctx.Header("Content-Type", "application/javascript")
			} else if strings.HasSuffix(requestPath, ".js") {
				ctx.Header("Content-Type", "application/javascript")
			} else if strings.HasSuffix(requestPath, ".css") {
				ctx.Header("Content-Type", "text/css")
			} else if strings.HasSuffix(requestPath, ".html") {
				ctx.Header("Content-Type", "text/html")
			} else if strings.HasSuffix(requestPath, ".pdf") {
				ctx.Header("Content-Type", "application/pdf")
			} else if strings.HasSuffix(requestPath, ".png") {
				ctx.Header("Content-Type", "image/png")
			} else if strings.HasSuffix(requestPath, ".jpg") || strings.HasSuffix(requestPath, ".jpeg") {
				ctx.Header("Content-Type", "image/jpeg")
			} else if strings.HasSuffix(requestPath, ".gif") {
				ctx.Header("Content-Type", "image/gif")
			} else if strings.HasSuffix(requestPath, ".svg") {
				ctx.Header("Content-Type", "image/svg+xml")
			} else if strings.HasSuffix(requestPath, ".ico") {
				ctx.Header("Content-Type", "image/x-icon")
			}
			// 提供文件
			ctx.File(filePath)
			return
		}
		// 文件不存在，返回index.html，支持SPA路由
		if isFrontendAssetRequest(requestPath) {
			log.Printf("Static asset not found: request=%s attempts=%v frontendStaticDirs=%v cwd_static=%s frontendDist=%s",
				requestPath, attempts, frontendStaticDirs, filepath.Join(".", "static", strings.TrimPrefix(requestPath, "/")), frontendDist)
			ctx.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			ctx.Header("Pragma", "no-cache")
			ctx.Header("Expires", "0")
			ctx.JSON(consts.StatusNotFound, utils.H{
				"code":    404,
				"success": false,
				"message": "静态资源不存在，请刷新页面获取最新版本",
				"data":    nil,
			})
			return
		}
		if indexPath, _, _ := resolveFrontendFile("/index.html"); indexPath != "" {
			ctx.File(indexPath)
			return
		}
		ctx.JSON(consts.StatusNotFound, utils.H{
			"code":    404,
			"success": false,
			"message": "前端入口文件不存在",
			"data":    nil,
		})
	})

	// 启动服务器
	deploymentAddr := fmt.Sprintf("http://127.0.0.1:%s", config.Port)

	fmt.Printf("\n=======================================")
	fmt.Printf("\n系统已部署到: %s\n", deploymentAddr)
	fmt.Printf("启动时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println("=======================================")
	fmt.Printf("服务器正在启动，监听端口 %s...\n", config.Port)

	if err := h.Run(); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
