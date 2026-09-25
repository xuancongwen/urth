#!/usr/bin/env bash
# Deploy to the LXC from the dev machine. Cross-compiles a static Linux
# binary, pushes it and the content tree over ssh/rsync, and restarts the
# service. Player files on the host are never touched.
#
#   deploy/deploy.sh [options]
#
#   --content       push world, scripts, and banner only; no binary, no
#                   restart. Scripts hot-reload on their own; type
#                   `reload world` in game for areas.
#   --no-restart    push everything but do not restart. Type `copyover` in
#                   game to switch to the new binary with nobody dropped.
#   --skip-check    do not run vet and tests before building.
#
# Connection settings come from deploy/deploy.env (see deploy.env.example)
# or the environment: URTH_HOST (required), URTH_SSH_PORT, URTH_ARCH,
# URTH_APP_DIR. Deploys log in as root. One ssh connection is opened at
# the start and shared by every rsync and ssh call, so a password is
# asked for once at most.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if [ -f deploy/deploy.env ]; then
  # shellcheck disable=SC1091
  . deploy/deploy.env
fi
HOST="${URTH_HOST:?set URTH_HOST in deploy/deploy.env or the environment}"
SSH_PORT="${URTH_SSH_PORT:-22}"
ARCH="${URTH_ARCH:-amd64}"
APP_DIR="${URTH_APP_DIR:-/opt/urth}"

CONTENT_ONLY=0
RESTART=1
CHECK=1
while [ $# -gt 0 ]; do
  case "$1" in
    --content) CONTENT_ONLY=1; RESTART=0 ;;
    --no-restart) RESTART=0 ;;
    --skip-check) CHECK=0 ;;
    -h|--help) sed -n 2,18p "$0"; exit 0 ;;
    *) echo "unknown option: $1" >&2; exit 2 ;;
  esac
  shift
done

TARGET="root@$HOST"
CTL="${TMPDIR:-/tmp}/urth-deploy-$$.sock"
SSH_OPTS=(-p "$SSH_PORT" -o ControlPath="$CTL")
RSYNC_RSH="ssh -p $SSH_PORT -o ControlPath=$CTL"

remote() { ssh "${SSH_OPTS[@]}" "$TARGET" "$@"; }

echo "==> connecting to $TARGET"
ssh "${SSH_OPTS[@]}" -o ControlMaster=yes -o ControlPersist=120 -fN "$TARGET"
trap 'ssh "${SSH_OPTS[@]}" -O exit "$TARGET" 2>/dev/null || true' EXIT

echo "==> target $TARGET:$APP_DIR ($ARCH)"
if ! remote test -d "$APP_DIR/data"; then
  echo "$APP_DIR/data is missing on $HOST; run deploy/setup.sh there first" >&2
  exit 1
fi

if [ "$CONTENT_ONLY" -eq 0 ]; then
  if [ "$CHECK" -eq 1 ]; then
    echo "==> vet and test"
    go vet ./...
    go test ./... >/dev/null
  fi
  echo "==> build linux/$ARCH"
  GOOS=linux GOARCH="$ARCH" make -s build
  echo "    $(GOOS=linux GOARCH="$ARCH" go version -m bin/urth 2>/dev/null | head -1 || true)"

  echo "==> push binary"
  # rsync writes to a temp file and renames it over the target, so the
  # running server keeps its old inode and a later copyover execs the new
  # file at the same path.
  rsync -e "$RSYNC_RSH" --chmod=0755 bin/urth "$TARGET:$APP_DIR/bin/urth"
fi

echo "==> push content"
rsync -e "$RSYNC_RSH" -rlt --delete \
  --exclude=/players --exclude=/instances --exclude=/copyover.json \
  data/ "$TARGET:$APP_DIR/data/"

if [ "$RESTART" -eq 1 ]; then
  echo "==> restart"
  remote systemctl restart urth
  sleep 1
  if remote systemctl is-active --quiet urth; then
    echo "    urth is active"
    remote "journalctl -u urth -n 5 --no-pager -o cat"
  else
    echo "    urth failed to start:" >&2
    remote "journalctl -u urth -n 30 --no-pager -o cat" >&2
    exit 1
  fi
else
  if remote systemctl is-active --quiet urth; then
    if [ "$CONTENT_ONLY" -eq 1 ]; then
      echo "==> content pushed; scripts reload on their own, type 'reload world' in game for areas"
    else
      echo "==> pushed without restart; type 'copyover' in game to switch binaries"
    fi
  else
    echo "==> pushed; urth is not running (systemctl start urth)"
  fi
fi
