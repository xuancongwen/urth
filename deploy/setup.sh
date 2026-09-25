#!/usr/bin/env bash
# One-time host setup. Run INSIDE the LXC (Debian or Ubuntu), as root,
# which is also what the game runs as:
#
#   scp deploy/setup.sh root@<lxc>:/root/ && ssh root@<lxc> bash /root/setup.sh
#
# It is idempotent: rerunning updates the unit file and leaves the config
# and player files alone. Everything on the host is owned and run by
# root; there is no service user. Options:
#
#   --cloudflared <tunnel token>   install cloudflared and run it as a
#                                  service for this tunnel (web client only;
#                                  see docs/DEPLOY.md for why not telnet)
#
# Afterwards, deploy from the dev machine with deploy/deploy.sh.
set -euo pipefail

APP_DIR=/opt/urth
CONF_DIR=/etc/urth
TUNNEL_TOKEN=""

while [ $# -gt 0 ]; do
  case "$1" in
    --cloudflared) TUNNEL_TOKEN="${2:?--cloudflared needs a token}"; shift 2 ;;
    -h|--help) sed -n 2,14p "$0"; exit 0 ;;
    *) echo "unknown option: $1" >&2; exit 2 ;;
  esac
done

if [ "$(id -u)" -ne 0 ]; then
  echo "run as root" >&2
  exit 1
fi
if ! command -v apt-get >/dev/null; then
  echo "this script expects a Debian or Ubuntu container" >&2
  exit 1
fi

echo "==> packages"
export DEBIAN_FRONTEND=noninteractive
apt-get update -q
apt-get install -y -q --no-install-recommends rsync ca-certificates curl

echo "==> directories"
mkdir -p "$APP_DIR/bin" "$APP_DIR/data/players" "$CONF_DIR"
chmod 700 "$APP_DIR/data/players"

echo "==> config"
if [ -f "$CONF_DIR/config.yaml" ]; then
  echo "    $CONF_DIR/config.yaml exists, keeping it"
else
  cat > "$CONF_DIR/config.yaml" <<'YAML'
# Urth production configuration. Written once by deploy/setup.sh; edit in
# place, then `systemctl restart urth` (or `copyover` in game).
server:
  name: Urth
  # Telnet must reach the internet directly (port forward or relay); a
  # Cloudflare tunnel cannot carry it. See docs/DEPLOY.md.
  telnet_addr: 0.0.0.0:4000
  # The web client and WebSocket. cloudflared on this host proxies it, so
  # loopback is enough; use 0.0.0.0:4001 to reach it from the LAN too.
  websocket_addr: 127.0.0.1:4001
  # Per-listener flood protection; see config.yaml in the repo for each field.
  limits:
    max_connections: 256
    max_per_ip: 8
    input_lines_per_second: 10
    input_burst: 20
    input_flood_limit: 200

timing:
  tick_ms: 100
  round_ms: 2000
  autosave_seconds: 300

paths:
  data: /opt/urth/data

world:
  start_room: 1
  first_player_is_admin: true

log:
  level: info
  format: text
YAML
fi

echo "==> systemd unit"
cat > /etc/systemd/system/urth.service <<UNIT
[Unit]
Description=Urth MUD server
After=network-online.target
Wants=network-online.target

[Service]
WorkingDirectory=$APP_DIR
ExecStart=$APP_DIR/bin/urth -config $CONF_DIR/config.yaml
# The in-game copyover command execs the binary in place, so the main PID
# never changes and systemd sees one continuous service.
Restart=always
RestartSec=2
LimitNOFILE=65536

# The game runs as root. These keep the rest of the filesystem read-only
# to it anyway; only the data tree (player files, copyover state) is
# writable.
NoNewPrivileges=yes
ProtectSystem=strict
ReadWritePaths=$APP_DIR/data
ProtectHome=yes
PrivateTmp=yes
PrivateDevices=yes
ProtectKernelTunables=yes
ProtectControlGroups=yes
RestrictSUIDSGID=yes

[Install]
WantedBy=multi-user.target
UNIT
systemctl daemon-reload
systemctl enable urth >/dev/null

echo "==> journald cap"
mkdir -p /etc/systemd/journald.conf.d
cat > /etc/systemd/journald.conf.d/urth.conf <<'CONF'
[Journal]
SystemMaxUse=200M
CONF
systemctl restart systemd-journald

if [ -n "$TUNNEL_TOKEN" ]; then
  echo "==> cloudflared"
  if ! command -v cloudflared >/dev/null; then
    . /etc/os-release
    mkdir -p --mode=0755 /usr/share/keyrings
    curl -fsSL https://pkg.cloudflare.com/cloudflare-main.gpg -o /usr/share/keyrings/cloudflare-main.gpg
    echo "deb [signed-by=/usr/share/keyrings/cloudflare-main.gpg] https://pkg.cloudflare.com/cloudflared ${VERSION_CODENAME} main" \
      > /etc/apt/sources.list.d/cloudflared.list
    apt-get update -q
    apt-get install -y -q cloudflared
  fi
  # Idempotent: reinstalling with the same token is a no-op for the tunnel.
  cloudflared service uninstall >/dev/null 2>&1 || true
  cloudflared service install "$TUNNEL_TOKEN"
  systemctl enable --now cloudflared >/dev/null
fi

cat <<MSG

Setup complete.
  binary   $APP_DIR/bin/urth   (not present yet; deploy.sh puts it there)
  content  $APP_DIR/data/{world,scripts,banner.txt}
  players  $APP_DIR/data/players   (never touched by deploys)
  config   $CONF_DIR/config.yaml
  service  systemctl status urth ; journalctl -u urth -f

Next: on the dev machine, copy deploy/deploy.env.example to deploy/deploy.env,
set URTH_HOST, and run make deploy.
MSG
