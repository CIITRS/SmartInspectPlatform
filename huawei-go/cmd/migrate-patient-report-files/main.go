package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"

	"huawei-go/handlers"
)

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func main() {
	apply := flag.Bool("apply", false, "正式上传并更新数据库；默认只预览")
	patientCode := flag.String("patient-code", "", "只迁移指定患者编号")
	sourceBase := flag.String("source-base", "https://bgpt.huaweibio.com.cn", "旧 /uploads 文件的来源站点")
	localRoot := flag.String("local-root", ".", "相对本地文件路径的根目录")
	limit := flag.Int("limit", 0, "最多处理多少名患者，0 表示不限制")
	flag.Parse()

	_ = godotenv.Load()
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true&loc=Local",
		env("DB_USER", "root"),
		os.Getenv("DB_PASSWORD"),
		env("DB_HOST", "localhost"),
		env("DB_PORT", "3306"),
		env("DB_NAME", "huawei_micro_diagnosis"),
	)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}
	handlers.SetDB(db)

	result, err := handlers.MigrateExistingPatientReportFiles(context.Background(), db, handlers.PatientReportMigrationOptions{
		Apply:       *apply,
		PatientCode: *patientCode,
		SourceBase:  *sourceBase,
		LocalRoot:   *localRoot,
		Limit:       *limit,
	})
	if err != nil {
		log.Fatal(err)
	}
	for _, item := range result.Items {
		fmt.Printf("[%s] patient=%s source=%s target=%s", item.Status, item.PatientCode, item.Source, item.TargetURL)
		if item.Error != "" {
			fmt.Printf(" error=%s", item.Error)
		}
		fmt.Println()
	}
	fmt.Printf("patients=%d planned=%d uploaded=%d skipped=%d failed=%d apply=%t\n",
		result.Patients, result.Planned, result.Uploaded, result.Skipped, result.Failed, *apply)
	if result.Failed > 0 {
		os.Exit(1)
	}
}
