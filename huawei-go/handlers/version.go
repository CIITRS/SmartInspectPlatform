package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	"huawei-go/internal/appversion"
)

type githubRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
}

type systemUpgradeStatus struct {
	Version     string `json:"version"`
	State       string `json:"state"`
	CurrentStep int    `json:"current_step"`
	TotalSteps  int    `json:"total_steps"`
	Progress    int    `json:"progress"`
	Message     string `json:"message"`
	StartedAt   string `json:"started_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

const systemUpgradeTotalSteps = 7

var (
	releaseTagPattern = regexp.MustCompile(`^v\d+(?:\.\d+){1,2}(?:-[0-9A-Za-z.-]+)?$`)
	upgradeRunning    atomic.Bool
	upgradeLogMu      sync.Mutex
)

func githubRepository() string {
	if value := strings.TrimSpace(os.Getenv("GITHUB_REPOSITORY")); value != "" {
		return value
	}
	return appversion.GitHubRepository
}

func fetchLatestGitHubRelease() (githubRelease, error) {
	var release githubRelease
	repository := githubRepository()
	request, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/"+repository+"/releases/latest", nil)
	if err != nil {
		return release, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "SmartInspectPlatform/"+appversion.CurrentVersion)
	if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := (&http.Client{Timeout: 15 * time.Second}).Do(request)
	if err != nil {
		return release, err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if response.StatusCode == http.StatusNotFound {
		return release, nil
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return release, fmt.Errorf("GitHub Release 查询失败（HTTP %d）", response.StatusCode)
	}
	if err := json.Unmarshal(body, &release); err != nil {
		return release, err
	}
	return release, nil
}

func numericVersionParts(version string) []int {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	if index := strings.Index(version, "-"); index >= 0 {
		version = version[:index]
	}
	parts := strings.Split(version, ".")
	result := make([]int, 3)
	for index := 0; index < len(parts) && index < len(result); index++ {
		result[index], _ = strconv.Atoi(parts[index])
	}
	return result
}

func compareVersions(left, right string) int {
	leftParts := numericVersionParts(left)
	rightParts := numericVersionParts(right)
	for index := range leftParts {
		if leftParts[index] < rightParts[index] {
			return -1
		}
		if leftParts[index] > rightParts[index] {
			return 1
		}
	}
	return 0
}

func systemUpgradeEnabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("SYSTEM_UPGRADE_ENABLED")))
	return value == "" || value == "1" || value == "true" || value == "on" || value == "enabled"
}

func systemUpgradeStatusPath() string {
	if value := strings.TrimSpace(os.Getenv("SYSTEM_UPGRADE_STATUS_FILE")); value != "" {
		return value
	}
	return filepath.Join("logs", "upgrade-status.json")
}

func readSystemUpgradeStatus() systemUpgradeStatus {
	status := systemUpgradeStatus{State: "idle", TotalSteps: systemUpgradeTotalSteps, Message: "尚未开始升级"}
	data, err := os.ReadFile(systemUpgradeStatusPath())
	if err != nil || json.Unmarshal(data, &status) != nil {
		return status
	}
	if status.TotalSteps <= 0 {
		status.TotalSteps = systemUpgradeTotalSteps
	}
	if status.Progress < 0 {
		status.Progress = 0
	}
	if status.Progress > 100 {
		status.Progress = 100
	}
	return status
}

func writeSystemUpgradeStatus(status systemUpgradeStatus) error {
	path := systemUpgradeStatusPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	status.TotalSteps = systemUpgradeTotalSteps
	status.UpdatedAt = time.Now().Format(time.RFC3339)
	data, err := json.Marshal(status)
	if err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, append(data, '\n'), 0644); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func readUpgradeLogTail(maxLines int) []string {
	data, err := os.ReadFile(filepath.Join("logs", "upgrade.log"))
	if err != nil {
		return []string{}
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return lines
}

func versionResponse(release githubRelease) utils.H {
	updateAvailable := release.TagName != "" && compareVersions(appversion.CurrentVersion, release.TagName) < 0
	upgradeStatus := readSystemUpgradeStatus()
	return utils.H{
		"current_version":     appversion.CurrentVersion,
		"release_date":        appversion.ReleaseDate,
		"build_commit":        appversion.BuildCommit,
		"build_date":          appversion.BuildDate,
		"repository":          githubRepository(),
		"latest_version":      release.TagName,
		"latest_name":         release.Name,
		"latest_notes":        release.Body,
		"latest_url":          release.HTMLURL,
		"latest_published_at": release.PublishedAt,
		"update_available":    updateAvailable,
		"upgrade_running":     upgradeRunning.Load() || upgradeStatus.State == "running",
		"upgrade_supported":   runtime.GOOS != "windows" && systemUpgradeEnabled(),
		"upgrade_status":      upgradeStatus,
	}
}

func HandleGetSystemVersion(c *app.RequestContext) {
	release, err := fetchLatestGitHubRelease()
	if err != nil {
		c.JSON(consts.StatusBadGateway, ApiResponse{
			Code: 502, Success: false, Message: err.Error(),
			Data: versionResponse(githubRelease{}),
		})
		return
	}
	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "版本信息读取成功", Data: versionResponse(release)})
}

func HandleGetSystemUpgradeStatus(c *app.RequestContext) {
	status := readSystemUpgradeStatus()
	if upgradeRunning.Load() && status.State != "running" {
		status.State = "running"
		status.Message = "升级任务正在运行"
	}
	c.JSON(consts.StatusOK, ApiResponse{Code: 200, Success: true, Message: "升级状态读取成功", Data: utils.H{
		"status":   status,
		"log_tail": readUpgradeLogTail(30),
	}})
}

func HandleUpgradeSystem(c *app.RequestContext) {
	if runtime.GOOS == "windows" || !systemUpgradeEnabled() {
		c.JSON(consts.StatusNotImplemented, ApiResponse{Code: 501, Success: false, Message: "当前运行环境未启用自动升级", Data: nil})
		return
	}
	if !upgradeRunning.CompareAndSwap(false, true) {
		c.JSON(consts.StatusConflict, ApiResponse{Code: 409, Success: false, Message: "系统升级正在进行中", Data: nil})
		return
	}

	var request struct {
		Version string `json:"version"`
	}
	body, _ := c.Body()
	if json.Unmarshal(body, &request) != nil {
		upgradeRunning.Store(false)
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "升级版本不能为空", Data: nil})
		return
	}
	release, err := fetchLatestGitHubRelease()
	if err != nil || release.TagName == "" || request.Version != release.TagName || !releaseTagPattern.MatchString(release.TagName) {
		upgradeRunning.Store(false)
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "升级版本与 GitHub 最新 Release 不一致", Data: nil})
		return
	}
	if compareVersions(appversion.CurrentVersion, release.TagName) >= 0 {
		upgradeRunning.Store(false)
		c.JSON(consts.StatusBadRequest, ApiResponse{Code: 400, Success: false, Message: "当前已是最新版本", Data: nil})
		return
	}
	startedAt := time.Now().Format(time.RFC3339)
	if err := writeSystemUpgradeStatus(systemUpgradeStatus{
		Version: release.TagName, State: "running", CurrentStep: 1, Progress: 5,
		Message: "正在启动升级任务", StartedAt: startedAt,
	}); err != nil {
		upgradeRunning.Store(false)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "无法创建升级进度状态", Data: nil})
		return
	}

	scriptPath := strings.TrimSpace(os.Getenv("SYSTEM_UPGRADE_SCRIPT"))
	if scriptPath == "" {
		scriptPath = filepath.Join("scripts", "upgrade.sh")
	}
	absoluteScript, err := filepath.Abs(scriptPath)
	if err != nil {
		upgradeRunning.Store(false)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "升级脚本路径无效", Data: nil})
		return
	}
	if _, err := os.Stat(absoluteScript); err != nil {
		upgradeRunning.Store(false)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "升级脚本不存在", Data: nil})
		return
	}
	if err := os.MkdirAll("logs", 0755); err != nil {
		upgradeRunning.Store(false)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "无法创建升级日志目录", Data: nil})
		return
	}
	logFile, err := os.OpenFile(filepath.Join("logs", "upgrade.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		upgradeRunning.Store(false)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "无法打开升级日志", Data: nil})
		return
	}
	workingDirectory, _ := os.Getwd()
	command := exec.Command("bash", absoluteScript, release.TagName)
	command.Dir = workingDirectory
	command.Stdout = logFile
	command.Stderr = logFile
	command.Env = append(os.Environ(),
		"SMART_INSPECT_CURRENT_VERSION="+appversion.CurrentVersion,
		"SMART_INSPECT_BUILD_COMMIT="+appversion.BuildCommit,
		"SMART_INSPECT_UPGRADE_STATUS_FILE="+systemUpgradeStatusPath(),
		"SMART_INSPECT_UPGRADE_STARTED_AT="+startedAt,
	)
	if err := command.Start(); err != nil {
		_ = logFile.Close()
		upgradeRunning.Store(false)
		c.JSON(consts.StatusInternalServerError, ApiResponse{Code: 500, Success: false, Message: "启动升级失败", Data: nil})
		return
	}
	go func() {
		err := command.Wait()
		_ = logFile.Close()
		upgradeLogMu.Lock()
		if err != nil {
			log.Printf("System upgrade to %s failed: %v", release.TagName, err)
			status := readSystemUpgradeStatus()
			if status.State == "running" || status.State == "idle" {
				status.Version = release.TagName
				status.State = "failed"
				status.Message = "升级失败，请查看日志后重试"
				_ = writeSystemUpgradeStatus(status)
			}
		}
		upgradeLogMu.Unlock()
		upgradeRunning.Store(false)
	}()
	c.JSON(consts.StatusAccepted, ApiResponse{
		Code: 202, Success: true, Message: "升级任务已启动，完成后服务将自动重启",
		Data: utils.H{"version": release.TagName, "log": "logs/upgrade.log"},
	})
}
