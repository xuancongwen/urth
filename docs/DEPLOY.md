# Deploying to an LXC

How the server runs in production: one unprivileged LXC, one static
binary under systemd, content pushed from the dev machine, player files
living only on the host. Cloudflare fronts the web client; telnet needs
its own path (section 4).

## 1. Container spec

The binary idles under 20 MB of RSS and the world tick is a few percent
of one core at a hundred players. Size the container for the operating
system and cloudflared, not the game.

| Resource | Minimum | Recommended | Why |
|---|---|---|---|
| vCPU | 1 | 2 | One core runs the game; the second keeps logins (bcrypt, about 70 ms each) and journald from touching the tick |
| RAM | 256 MB | 512 MB | Game about 20 MB, cloudflared about 40 MB, the rest is page cache and headroom |
| Swap | 0 | 256 MB | Only so an OOM kill never hits the game first |
| Disk | 4 GB | 8 GB | Debian template about 1 GB, journal capped at 200 MB, player files are kilobytes |
| Template | Debian 12 or 13 | Debian 13 | The setup script assumes apt and systemd |
| Type | Unprivileged | Unprivileged | Nothing needs privileges; no nesting, no device passthrough |
| Network | DHCP reservation | Static IP | The port forward and the deploy target both need a stable address |

Nothing is built on the container: the deploy script cross-compiles on
the dev machine, so the container never needs Go or 2 GB of RAM for a
compiler. If you later fork toward the coordinate game in
`COORDINATE-PORT.md`, expect roughly one core per few hundred players
and revisit the CPU line.

## 2. First-time setup

Inside the container, as root:

```
scp deploy/setup.sh root@<lxc>:/root/
ssh root@<lxc> bash /root/setup.sh
```

This installs rsync, lays out `/opt/urth/{bin,data}` and
`/etc/urth/config.yaml`, installs and enables `urth.service`, and caps
the journal. Everything on the host is owned and run by root; there is no
service user. The unit still keeps the filesystem read-only to the game
apart from the data tree. It is safe to rerun: the unit is refreshed,
the config and players are kept.

With a Cloudflare tunnel token it also installs cloudflared as a service:

```
ssh root@<lxc> bash /root/setup.sh --cloudflared <token>
```

Then on the dev machine:

```
cp deploy/deploy.env.example deploy/deploy.env   # set URTH_HOST
make deploy
```

## 3. Deploying

`make deploy` (or `deploy/deploy.sh`) opens one ssh connection as root
(asking for a password once if no key is set up), runs the content check
(`urth check`: a world that fails to load or has unreachable rooms never
ships), vet, and tests, cross-compiles a static `linux/amd64` binary, pushes it and `data/{world,scripts,
banner.txt}` over rsync, restarts the service, and prints the last log
lines. `data/players`, `data/instances`, and the copyover state file are
excluded on both sides, so a deploy never touches a character.

Three variants:

- `make deploy DEPLOY_ARGS=--content` pushes content only and does not
  restart. Scripts hot-reload within a round on their own; areas need
  `reload world` in game.
- `make deploy DEPLOY_ARGS=--no-restart` pushes the binary too but
  leaves the old process running. Type `copyover` in game and the server
  execs the new binary at the same path with every telnet player still
  connected and every web player reconnected by token. This is the
  zero-downtime path. It works because rsync renames a new file over the
  old one and Go resolves its own path without the kernel's "(deleted)"
  suffix, so the exec picks up the new inode.
- `make deploy DEPLOY_ARGS=--skip-check` skips the content check, vet,
  and tests.

For an arm64 container set `URTH_ARCH=arm64` in `deploy.env`.

Useful on the host:

```
systemctl status urth
journalctl -u urth -f
/opt/urth/bin/urth admin -config /etc/urth/config.yaml list
```

`urth admin` edits player files directly: `promote`, `demote`, `passwd`,
`deny`, `allow`, `delete`, `show`. It is for when nobody with admin can
log in; for a character who is online, the in-game commands of the same
names are right, since the server rewrites the file at its next save.

Leave `server.builder_addr` unset on the host. The builder page has no
login and is built for the checkout on the dev machine, where content
is edited; the next deploy overwrites `data/world` on the host anyway.

## 4. Reaching the server from the internet

### The web client, through Cloudflare Tunnel

This works as-is. In the Zero Trust dashboard, give the tunnel a public
hostname (say `play.example.com`) with service `http://localhost:4001`.
The page at `/` and the socket at `/ws` both ride over HTTPS, and the
page already chooses `wss://` when loaded over `https://`. Copyover
reconnects also work through the tunnel since they are a fresh WebSocket
with a token.

### Telnet: the tunnel cannot carry it

Cloudflare Tunnel proxies HTTP and WebSocket to a public hostname. For
any other TCP protocol the *player* must run Cloudflare software: either
`cloudflared access tcp --hostname ... --url localhost:4000` or the WARP
client enrolled in your Zero Trust organisation. No MUD client (Mudlet,
TinTin++, Mudslinger, a bare telnet) can connect to a tunnel hostname on
port 4000, so this is not a public-facing option.

Cloudflare Spectrum does terminate raw TCP at the edge, but arbitrary
ports are Enterprise only; Pro and Business are limited to SSH, RDP, and
Minecraft. Not realistic for a hobby MUD.

That leaves two real options for port 4000:

1. **Port forward at the router.** Forward external 4000 to the
   container's IP. Create a DNS record `mud.example.com` on Cloudflare
   with the proxy **off** (grey cloud, DNS only) pointing at the home IP,
   and run a small dynamic DNS updater against the Cloudflare API if the
   IP is not static. Cheapest and lowest-latency. It exposes the home IP
   to anyone who resolves the name.
2. **A relay VPS.** The smallest VPS anywhere, with a WireGuard link (or
   an autossh reverse tunnel) to the container, and a forward of its
   public port 4000 across the link. Tools built for this: `rathole`,
   `frp`, or plain `iptables` DNAT over WireGuard. The home IP stays
   hidden and the VPS absorbs any abuse. Costs a few dollars a month
   and adds one hop of latency.

Either way the telnet listener must bind `0.0.0.0:4000`, which the
generated config does. The WebSocket listener can stay on loopback since
cloudflared runs on the same host.

### Limits

Both listeners cap live connections, connections per address, and input
lines per connection (`server.limits` in the config). A refused telnet
client reads "Too many connections" and is closed before it reaches the
world; a refused browser gets HTTP 503 and the page retries with backoff.
Lines over the rate are dropped; a client that keeps flooding is
disconnected with the reason `input flood` in the log.

Behind cloudflared every browser arrives from 127.0.0.1, so the web
listener keys the per-address cap by the `CF-Connecting-IP` header (or
the first `X-Forwarded-For` entry) when, and only when, the socket peer is
loopback. A direct connection from the internet is always keyed by its
real address, so the header cannot be spoofed to escape the cap.

To gate the page entirely while testing, put a Cloudflare Access policy
on the hostname; the server needs no change.

### The browser client

The page at `/` is a full client, not a terminal. The log fills the main
pane with a sticky room title; the side panel holds a map, vitals, and
what is in the room; the input bar completes command names on Tab.

- **Map.** Drawn from the `map` field of each room message: the rooms
  within six steps in the same area, positioned by the server's layout
  pass (`internal/room/layout.go`). Rooms the character has stood in are
  lit, safe rooms are green, up and down exits carry an arrow, and
  clicking a room walks there along known exits. Room files may carry
  `position: {x, y, z}` to anchor the layout where exits contradict the
  grid; the server logs every such conflict at load and reload.
- **Here.** Players, creatures, and items in the room. Clicking a
  creature or item opens a menu of commands (look, consider, kill; look,
  get, sacrifice) that send the same reference a typed command would.
- **Vitals** from the prompt's data, with a flash on the health bar when
  it drops.
- **Connect** is a button. The page does not open a socket on load;
  the player clicks Connect (or presses Enter) to start a session.
- **Reconnect** on its own after a copyover, a tunnel blip, a phone
  coming back from the background, or a server restart, with backoff up
  to 30 s; Enter retries at once. A deliberate quit does not reconnect.
  Password prompts switch the box to a password field.
- **Idle** sessions are hung up by the client: fifteen minutes without
  a command, with a warning in the log a minute before. Output arriving
  does not count. After that the Connect button is back and nothing
  reconnects unasked. The limit is `idleLimit` in `static/index.html`.
- On a phone the side panel is a drawer behind the **map** button.

## 5. Backups

Everything that matters and is not in git is `/opt/urth/data/players`.
A nightly `rsync` of that directory off the container, or a Proxmox
backup job on the whole CT, covers it.
