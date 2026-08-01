#!/usr/bin/env bash
set -Eeuo pipefail

TAG="${1:-}"
CHECK_ONLY="${2:-}"
if [[ ! "$TAG" =~ ^v[0-9]+(\.[0-9]+){1,2}(-[0-9A-Za-z.-]+)?$ ]]; then
  echo "Usage: $0 vX.Y[.Z] [--check]" >&2
  exit 2
fi
if [[ -n "$CHECK_ONLY" && "$CHECK_ONLY" != "--check" ]]; then
  echo "Unknown option: $CHECK_ONLY" >&2
  exit 2
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
REPOSITORY="${GITHUB_REPOSITORY:-CIITRS/SmartInspectPlatform}"
UPGRADE_ROOT="${SMART_INSPECT_UPGRADE_ROOT:-$APP_DIR/.upgrade}"
RUN_ID="$(date +%Y%m%d%H%M%S)-$$"
RUN_DIR="$UPGRADE_ROOT/run-$RUN_ID"
DOWNLOAD_DIR="$RUN_DIR/download"
STAGE_DIR="$RUN_DIR/stage"
BACKUP_DIR="$UPGRADE_ROOT/backups/$RUN_ID-$TAG"
LOCK_FILE="$UPGRADE_ROOT/upgrade.lock"
UPGRADE_APPLIED=0
STATIC_PREVIOUS=""
BINARY_TARGET=""
SERVICE_MANAGER=""
SERVICE_TARGET=""
SUPERVISORCTL=""
SUPERVISOR_CONFIG="${SMART_INSPECT_SUPERVISOR_CONFIG:-/etc/supervisor/supervisord.conf}"
UPGRADE_STATUS_FILE="${SMART_INSPECT_UPGRADE_STATUS_FILE:-$APP_DIR/logs/upgrade-status.json}"
UPGRADE_STARTED_AT="${SMART_INSPECT_UPGRADE_STARTED_AT:-$(date -u '+%Y-%m-%dT%H:%M:%SZ')}"
UPGRADE_STEP=1
DOWNLOAD_NAME=""
DOWNLOAD_BYTES=0
DOWNLOAD_TOTAL_BYTES=0
DOWNLOAD_SPEED_BPS=0
DOWNLOAD_PERCENT=0

log() {
  printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"
}

write_status() {
  local step="$1" state="$2" message="$3" progress updated_at temporary
  UPGRADE_STEP="$step"
  progress=$((step * 100 / 7))
  if [[ "$step" == "2" && "$DOWNLOAD_TOTAL_BYTES" -gt 0 ]]; then
    progress=$((14 + DOWNLOAD_PERCENT * 14 / 100))
  fi
  [[ "$state" == "completed" ]] && progress=100
  updated_at="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
  mkdir -p "$(dirname "$UPGRADE_STATUS_FILE")"
  temporary="$UPGRADE_STATUS_FILE.tmp.$$"
  printf '{"version":"%s","state":"%s","current_step":%d,"total_steps":7,"progress":%d,"message":"%s","started_at":"%s","updated_at":"%s","download_name":"%s","download_bytes":%d,"download_total_bytes":%d,"download_speed_bps":%d,"download_percent":%d}\n' \
    "$TAG" "$state" "$step" "$progress" "$message" "$UPGRADE_STARTED_AT" "$updated_at" \
    "$DOWNLOAD_NAME" "$DOWNLOAD_BYTES" "$DOWNLOAD_TOTAL_BYTES" "$DOWNLOAD_SPEED_BPS" "$DOWNLOAD_PERCENT" >"$temporary"
  mv -f "$temporary" "$UPGRADE_STATUS_FILE"
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "Required command not found: $1" >&2
    exit 1
  }
}

find_python() {
  local candidate
  for candidate in \
    "$(command -v python3 2>/dev/null || true)" \
    /www/server/panel/pyenv/bin/python3 \
    /www/server/panel/pyenv/bin/python; do
    if [[ -n "$candidate" && -x "$candidate" ]]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done
  echo "Python 3 is required to parse GitHub Release metadata." >&2
  return 1
}

github_curl() {
  local headers=(
    --header "X-GitHub-Api-Version: 2022-11-28"
    --header "User-Agent: SmartInspectPlatform-Upgrader"
  )
  if [[ -n "${GITHUB_TOKEN:-}" ]]; then
    headers+=(--header "Authorization: Bearer $GITHUB_TOKEN")
  fi
  curl --http1.1 --fail --location --retry 3 --retry-all-errors --connect-timeout 15 \
    --silent --show-error "${headers[@]}" "$@"
}

monitor_download() {
  local curl_pid="$1" archive="$2" started_at now elapsed
  started_at="$(date +%s)"
  while kill -0 "$curl_pid" 2>/dev/null; do
    if [[ -f "$archive" ]]; then
      DOWNLOAD_BYTES="$(stat -c %s "$archive" 2>/dev/null || printf '0')"
    fi
    now="$(date +%s)"
    elapsed=$((now - started_at))
    [[ "$elapsed" -gt 0 ]] && DOWNLOAD_SPEED_BPS=$((DOWNLOAD_BYTES / elapsed))
    if [[ "$DOWNLOAD_TOTAL_BYTES" -gt 0 ]]; then
      DOWNLOAD_PERCENT=$((DOWNLOAD_BYTES * 100 / DOWNLOAD_TOTAL_BYTES))
      [[ "$DOWNLOAD_PERCENT" -gt 100 ]] && DOWNLOAD_PERCENT=100
    fi
    write_status 2 "running" "正在下载 GitHub 发布包"
    sleep 1
  done
}

download_asset() {
  local asset_id="$1" archive="$2" curl_pid monitor_pid curl_status
  github_curl \
    --continue-at - \
    --header "Accept: application/octet-stream" \
    "https://api.github.com/repos/$REPOSITORY/releases/assets/$asset_id" \
    --output "$archive" &
  curl_pid=$!
  monitor_download "$curl_pid" "$archive" &
  monitor_pid=$!
  curl_status=0
  wait "$curl_pid" || curl_status=$?
  wait "$monitor_pid" || true
  [[ "$curl_status" == "0" ]] || return "$curl_status"
  DOWNLOAD_BYTES="$(stat -c %s "$archive")"
  DOWNLOAD_PERCENT=100
  write_status 2 "running" "发布包下载完成，正在校验完整性"
}

detect_binary() {
  if [[ -n "${SMART_INSPECT_BINARY:-}" ]]; then
    BINARY_TARGET="$SMART_INSPECT_BINARY"
  elif [[ -f "$APP_DIR/huawei-go" ]]; then
    BINARY_TARGET="$APP_DIR/huawei-go"
  else
    BINARY_TARGET="$APP_DIR/server"
  fi
}

detect_supervisor() {
  local candidate program profile command_path
  for candidate in \
    "${SMART_INSPECT_SUPERVISORCTL:-}" \
    /www/server/panel/pyenv/bin/supervisorctl \
    "$(command -v supervisorctl 2>/dev/null || true)"; do
    if [[ -n "$candidate" && -x "$candidate" ]]; then
      SUPERVISORCTL="$candidate"
      break
    fi
  done
  [[ -n "$SUPERVISORCTL" && -f "$SUPERVISOR_CONFIG" ]] || return 1

  if [[ -n "${SMART_INSPECT_SERVICE:-}" && "${SMART_INSPECT_SERVICE}" != "auto" ]]; then
    SERVICE_TARGET="${SMART_INSPECT_SERVICE}"
    [[ "$SERVICE_TARGET" == *:* ]] || SERVICE_TARGET="${SERVICE_TARGET}:*"
    return 0
  fi

  for profile in /www/server/panel/plugin/supervisor/profile/*.ini /etc/supervisor/conf.d/*.conf; do
    [[ -f "$profile" ]] || continue
    command_path="$(awk -F= '/^[[:space:]]*command[[:space:]]*=/{sub(/^[^=]*=/, ""); gsub(/^[[:space:]]+|[[:space:]]+$/, ""); print; exit}' "$profile")"
    command_path="${command_path%% *}"
    if [[ "$command_path" == "$BINARY_TARGET" ]]; then
      program="$(sed -n 's/^[[:space:]]*\[program:\([^]]*\)\].*/\1/p' "$profile" | head -1)"
      if [[ -n "$program" ]]; then
        SERVICE_TARGET="${program}:*"
        return 0
      fi
    fi
  done
  return 1
}

detect_service_manager() {
  local requested="${SMART_INSPECT_SERVICE_MANAGER:-auto}"
  if [[ "$requested" == "auto" || "$requested" == "supervisor" ]]; then
    if detect_supervisor; then
      SERVICE_MANAGER="supervisor"
      return
    fi
    [[ "$requested" == "auto" ]] || {
      echo "Supervisor was requested but supervisorctl/config/service could not be detected." >&2
      exit 1
    }
  fi
  if [[ "$requested" == "auto" || "$requested" == "systemd" ]]; then
    require_command systemctl
    SERVICE_MANAGER="systemd"
    SERVICE_TARGET="${SMART_INSPECT_SERVICE:-huawei-go}"
    return
  fi
  echo "Unsupported service manager: $requested" >&2
  exit 1
}

restart_service() {
  if [[ "$SERVICE_MANAGER" == "supervisor" ]]; then
    "$SUPERVISORCTL" -c "$SUPERVISOR_CONFIG" restart "$SERVICE_TARGET"
    "$SUPERVISORCTL" -c "$SUPERVISOR_CONFIG" status "$SERVICE_TARGET" | grep -q RUNNING
  else
    systemctl restart "$SERVICE_TARGET"
    systemctl is-active --quiet "$SERVICE_TARGET"
  fi
}

health_check() {
  local port health_url attempt
  port="$(awk -F= '/^[[:space:]]*PORT=/{gsub(/[[:space:]\r]/, "", $2); print $2; exit}' "$APP_DIR/.env" 2>/dev/null || true)"
  port="${port:-3001}"
  health_url="${SMART_INSPECT_HEALTH_URL:-http://127.0.0.1:$port/}"
  for attempt in $(seq 1 20); do
    if curl --fail --silent --show-error --max-time 5 --output /dev/null "$health_url"; then
      return 0
    fi
    sleep 1
  done
  echo "Health check failed: $health_url" >&2
  return 1
}

cleanup() {
  rm -rf -- "$RUN_DIR"
}

rollback() {
  local exit_code=$?
  trap - ERR
  if [[ "$UPGRADE_APPLIED" == "1" ]]; then
    log "Upgrade failed; restoring previous application files."
    if [[ -f "$BACKUP_DIR/$(basename "$BINARY_TARGET")" ]]; then
      install -m 0755 "$BACKUP_DIR/$(basename "$BINARY_TARGET")" "$BINARY_TARGET"
    fi
    if [[ -n "$STATIC_PREVIOUS" && -d "$STATIC_PREVIOUS" ]]; then
      rm -rf -- "$APP_DIR/static"
      mv "$STATIC_PREVIOUS" "$APP_DIR/static"
    elif [[ -d "$BACKUP_DIR/static" ]]; then
      rm -rf -- "$APP_DIR/static"
      cp -a "$BACKUP_DIR/static" "$APP_DIR/static"
    fi
    restart_service || true
  fi
  write_status "$UPGRADE_STEP" "failed" "升级失败，已尝试恢复上一版本"
  exit "$exit_code"
}

download_release() {
  local asset archive package_root release_json python_bin asset_id asset_digest asset_size expected_digest actual_size
  asset="SmartInspectPlatform-$TAG-linux-amd64.tar.gz"
  archive="$DOWNLOAD_DIR/$asset"
  release_json="$DOWNLOAD_DIR/release.json"
  mkdir -p "$DOWNLOAD_DIR" "$STAGE_DIR"
  python_bin="$(find_python)"
  log "Reading GitHub Release metadata for $TAG."
  github_curl \
    --header "Accept: application/vnd.github+json" \
    "https://api.github.com/repos/$REPOSITORY/releases/tags/$TAG" \
    --output "$release_json"
  read -r asset_id asset_digest asset_size < <(
    "$python_bin" -c \
      'import json,sys; data=json.load(open(sys.argv[1],encoding="utf-8")); asset=next((item for item in data.get("assets",[]) if item.get("name")==sys.argv[2]),None); asset or sys.exit("Release asset not found: "+sys.argv[2]); print(asset["id"],asset.get("digest",""),asset["size"])' \
      "$release_json" "$asset"
  )
  [[ "$asset_id" =~ ^[0-9]+$ && "$asset_digest" =~ ^sha256:[0-9a-fA-F]{64}$ && "$asset_size" =~ ^[0-9]+$ ]] || {
    echo "GitHub Release asset metadata is incomplete or invalid." >&2
    exit 1
  }
  log "Downloading verified GitHub Release asset $asset."
  DOWNLOAD_NAME="$asset"
  DOWNLOAD_TOTAL_BYTES="$asset_size"
  DOWNLOAD_BYTES=0
  DOWNLOAD_SPEED_BPS=0
  DOWNLOAD_PERCENT=0
  write_status 2 "running" "正在下载 GitHub 发布包"
  download_asset "$asset_id" "$archive"
  actual_size="$(stat -c %s "$archive")"
  [[ "$actual_size" == "$asset_size" ]] || {
    echo "Release asset size mismatch: got $actual_size, expected $asset_size." >&2
    exit 1
  }
  expected_digest="${asset_digest#sha256:}"
  printf '%s  %s\n' "$expected_digest" "$archive" | sha256sum --check -
  tar -xzf "$archive" -C "$DOWNLOAD_DIR"
  package_root="$DOWNLOAD_DIR/SmartInspectPlatform-$TAG"
  [[ -x "$package_root/huawei-go/server" ]] || {
    echo "Release package does not contain huawei-go/server." >&2
    exit 1
  }
  cp "$package_root/huawei-go/server" "$STAGE_DIR/application"
  cp -a "$package_root/huawei-go/static" "$STAGE_DIR/static"
  if [[ -f "$package_root/huawei-go/scripts/upgrade.sh" ]]; then
    cp "$package_root/huawei-go/scripts/upgrade.sh" "$STAGE_DIR/upgrade.sh"
  fi
}

build_from_source() {
  local root_dir worktree_dir build_commit build_date
  root_dir="$(cd "$APP_DIR/.." && pwd)"
  worktree_dir="$RUN_DIR/worktree"
  require_command git
  require_command go
  require_command npm
  cd "$root_dir"
  git fetch --tags origin
  git rev-parse --verify "${TAG}^{commit}" >/dev/null
  git worktree add --detach "$worktree_dir" "$TAG"
  (
    cd "$worktree_dir/huawei-ui"
    npm ci
    npm run build
  )
  mkdir -p "$worktree_dir/huawei-go/static"
  cp -a "$worktree_dir/huawei-ui/dist/." "$worktree_dir/huawei-go/static/"
  build_commit="$(git -C "$worktree_dir" rev-parse --short=12 HEAD)"
  build_date="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  (
    cd "$worktree_dir/huawei-go"
    go test ./...
    go build -trimpath \
      -ldflags "-s -w -X huawei-go/internal/appversion.CurrentVersion=$TAG -X huawei-go/internal/appversion.BuildCommit=$build_commit -X huawei-go/internal/appversion.BuildDate=$build_date" \
      -o "$STAGE_DIR/application" .
  )
  cp -a "$worktree_dir/huawei-go/static" "$STAGE_DIR/static"
  cp "$worktree_dir/huawei-go/scripts/upgrade.sh" "$STAGE_DIR/upgrade.sh"
  git -C "$root_dir" worktree remove --force "$worktree_dir"
}

require_command curl
require_command sha256sum
require_command tar
require_command flock
mkdir -p "$UPGRADE_ROOT" "$UPGRADE_ROOT/backups"
exec 9>"$LOCK_FILE"
flock -n 9 || {
  echo "Another system upgrade is already running." >&2
  exit 1
}
trap cleanup EXIT
trap rollback ERR

detect_binary
detect_service_manager
write_status 1 "running" "正在检查运行环境和服务配置"
log "Application directory: $APP_DIR"
log "Binary target: $BINARY_TARGET"
log "Service manager: $SERVICE_MANAGER ($SERVICE_TARGET)"

write_status 2 "running" "正在获取并校验 GitHub 发布包"
if [[ -f "$APP_DIR/go.mod" && -d "$APP_DIR/../huawei-ui" ]] &&
  git -C "$APP_DIR/.." rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  log "Upgrade mode: source build"
  build_from_source
else
  log "Upgrade mode: verified GitHub Release"
  download_release
fi
write_status 2 "running" "发布包已获取并通过校验"

if [[ "$CHECK_ONLY" == "--check" ]]; then
	write_status 7 "completed" "升级检查完成，未修改运行文件"
  log "Check completed. Release, checksum, layout and service control are valid; no files were changed."
  exit 0
fi

write_status 3 "running" "正在备份当前版本"
mkdir -p "$BACKUP_DIR"
if [[ -f "$BINARY_TARGET" ]]; then
  cp -a "$BINARY_TARGET" "$BACKUP_DIR/$(basename "$BINARY_TARGET")"
fi
if [[ -d "$APP_DIR/static" ]]; then
  cp -a "$APP_DIR/static" "$BACKUP_DIR/static"
fi

write_status 4 "running" "正在替换后端程序和管理端静态文件"
UPGRADE_APPLIED=1
install -m 0755 "$STAGE_DIR/application" "$BINARY_TARGET.new"
mv -f "$BINARY_TARGET.new" "$BINARY_TARGET"

STATIC_PREVIOUS="$APP_DIR/static.previous-$RUN_ID"
rm -rf -- "$APP_DIR/static.new-$RUN_ID"
cp -a "$STAGE_DIR/static" "$APP_DIR/static.new-$RUN_ID"
if [[ -d "$APP_DIR/static" ]]; then
  mv "$APP_DIR/static" "$STATIC_PREVIOUS"
fi
mv "$APP_DIR/static.new-$RUN_ID" "$APP_DIR/static"

if [[ -f "$STAGE_DIR/upgrade.sh" ]]; then
  install -m 0755 "$STAGE_DIR/upgrade.sh" "$APP_DIR/scripts/upgrade.sh.new"
  mv -f "$APP_DIR/scripts/upgrade.sh.new" "$APP_DIR/scripts/upgrade.sh"
fi

write_status 5 "running" "正在重启系统服务"
restart_service
write_status 6 "running" "服务已重启，正在执行健康检查"
health_check
printf '%s\n' "$TAG" >"$APP_DIR/VERSION"
UPGRADE_APPLIED=0
rm -rf -- "$STATIC_PREVIOUS"
write_status 7 "completed" "升级完成，系统运行正常"
log "Upgrade to $TAG completed successfully. Backup: $BACKUP_DIR"
