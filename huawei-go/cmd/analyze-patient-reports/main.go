package main

import (
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
	limit := flag.Int("limit", 0, "最多分析多少份报告，0 表示全部")
	force := flag.Bool("force", false, "重新分析已有成功结果")
	flag.Parse()

	_ = godotenv.Load()
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true&loc=Local",
		env("DB_USER", "root"), os.Getenv("DB_PASSWORD"), env("DB_HOST", "localhost"),
		env("DB_PORT", "3306"), env("DB_NAME", "huawei_micro_diagnosis"))
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}
	handlers.SetDB(db)
	handlers.InitAIChat()
	handlers.ReloadAISettings(db)
	processed, succeeded, failed, err := handlers.BackfillPatientReportAnalyses(db, *limit, *force)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("processed=%d succeeded=%d failed=%d\n", processed, succeeded, failed)
	if failed > 0 {
		os.Exit(1)
	}
}
