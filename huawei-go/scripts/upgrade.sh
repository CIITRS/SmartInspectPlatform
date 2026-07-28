#!/usr/bin/env bash
set -euo pipefail

TAG="${1:-}"
if [[ ! "$TAG" =~ ^v[0-9]+(\.[0-9]+){1,2}(-[0-9A-Za-z.-]+)?$ ]]; then
  echo "Invalid release tag: $TAG" >&2
  exit 2
fi

BACKEND_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ROOT_DIR="$(cd "$BACKEND_DIR/.." && pwd)"
SERVICE_NAME="${SMART_INSPECT_SERVICE:-huawei-go}"
UPGRADE_ROOT="$ROOT_DIR/.upgrade"
WORKTREE_DIR="$UPGRADE_ROOT/worktree-${TAG//\//-}"
STAGE_DIR="$UPGRADE_ROOT/stage-${TAG//\//-}"
BACKUP_DIR="$UPGRADE_ROOT/backups/$(date +%Y%m%d%H%M%S)-${TAG//\//-}"
UPGRADE_APPLIED=0

cleanup() {
  git -C "$ROOT_DIR" worktree remove --force "$WORKTREE_DIR" >/dev/null 2>&1 || true
}

rollback() {
  local exit_code=$?
  trap - ERR
  if [[ "$UPGRADE_APPLIED" == "1" ]]; then
    echo "Upgrade failed; restoring previous application files." >&2
    if [[ -f "$BACKUP_DIR/server" ]]; then
      install -m 0755 "$BACKUP_DIR/server" "$BACKEND_DIR/server"
    fi
    if [[ -d "$BACKUP_DIR/static" ]]; then
      rm -rf "$BACKEND_DIR/static"
      cp -a "$BACKUP_DIR/static" "$BACKEND_DIR/static"
    fi
    systemctl restart "$SERVICE_NAME" || true
  fi
  exit "$exit_code"
}

trap cleanup EXIT
trap rollback ERR

mkdir -p "$UPGRADE_ROOT/backups"
cd "$ROOT_DIR"
git fetch --tags origin
git rev-parse --verify "${TAG}^{commit}" >/dev/null

if [[ -d "$WORKTREE_DIR" ]]; then
  git worktree remove --force "$WORKTREE_DIR"
fi
git worktree add --detach "$WORKTREE_DIR" "$TAG"

cd "$WORKTREE_DIR/huawei-ui"
npm ci
npm run build

mkdir -p "$WORKTREE_DIR/huawei-go/static"
find "$WORKTREE_DIR/huawei-go/static" -mindepth 1 -maxdepth 1 -exec rm -rf -- {} +
cp -a "$WORKTREE_DIR/huawei-ui/dist/." "$WORKTREE_DIR/huawei-go/static/"

cd "$WORKTREE_DIR/huawei-go"
BUILD_COMMIT="$(git -C "$WORKTREE_DIR" rev-parse --short=12 HEAD)"
BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
go build -trimpath \
  -ldflags "-s -w -X huawei-go/internal/appversion.CurrentVersion=$TAG -X huawei-go/internal/appversion.BuildCommit=$BUILD_COMMIT -X huawei-go/internal/appversion.BuildDate=$BUILD_DATE" \
  -o server .

mkdir -p "$STAGE_DIR" "$BACKUP_DIR"
cp "$WORKTREE_DIR/huawei-go/server" "$STAGE_DIR/server"
cp -a "$WORKTREE_DIR/huawei-go/static" "$STAGE_DIR/static"
if [[ -f "$BACKEND_DIR/server" ]]; then cp "$BACKEND_DIR/server" "$BACKUP_DIR/server"; fi
if [[ -d "$BACKEND_DIR/static" ]]; then cp -a "$BACKEND_DIR/static" "$BACKUP_DIR/static"; fi

install -m 0755 "$STAGE_DIR/server" "$BACKEND_DIR/server.new"
mv -f "$BACKEND_DIR/server.new" "$BACKEND_DIR/server"
mkdir -p "$BACKEND_DIR/static"
find "$BACKEND_DIR/static" -mindepth 1 -maxdepth 1 -exec rm -rf -- {} +
cp -a "$STAGE_DIR/static/." "$BACKEND_DIR/static/"

UPGRADE_APPLIED=1
systemctl restart "$SERVICE_NAME"
echo "$TAG" > "$ROOT_DIR/VERSION"
UPGRADE_APPLIED=0
echo "Upgrade to $TAG completed successfully."
