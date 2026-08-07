# Quick Notes

A note-taking tool for people who think in the terminal - write it down in seconds, find it on any device.

Built by [@bkrisa12](https://x.com/bkrisa12).

## What is this?

Quick Notes is a tiny CLI note-taking app. You open it, jot down whatever's on your mind - a quick thought, a shopping list, a snippet - and it's instantly saved locally and quietly synced to your own server in the background. No cloud account, no third-party service: just your own VPS, reachable only through your private [Tailscale](https://tailscale.com) network.

## Installation

### Requirements

- [Go](https://go.dev/dl/) 1.25 or later
- [Tailscale](https://tailscale.com) installed and connected on every device you want to sync (laptop, VPS, phone)

### Linux (client)

```bash
git clone https://github.com/bkrisa/notes.git
cd notes
go build -o note main.go
sudo mv note /usr/local/bin/
```

### VPS (server)

Clone and build the same way as above, then set it up as a background service.

Create `/etc/systemd/system/note.service`:

```ini
[Unit]
Description=Note App Server
After=network.target tailscaled.service

[Service]
Type=simple
User=note-user
WorkingDirectory=/path/to/notes
Environment=QNOTE_LISTEN=<your-vps-tailscale-ip>:8080
ExecStart=/path/to/notes/note --server
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Then:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now note
```

Run a dedicated, unprivileged user for this (not root) - see [Security](#security) below.

### Termux (Android)

```bash
pkg install golang git
git clone https://github.com/bkrisa/notes.git
cd notes
go build -o note main.go
cp note $PREFIX/bin/
```

> Termux's `golang` package can lag behind the latest Go release. If `tailscale.com` refuses to build due to a Go version mismatch, try a slightly older `tailscale.com` version: `go get tailscale.com@v1.100.0 && go mod tidy`.

### Connect a client to your server

On each client device, point it at your VPS:

```bash
echo 'export QNOTE_SERVER="http://<your-vps-tailscale-ip>:8080"' >> ~/.bashrc
source ~/.bashrc
```

## Usage

Run `note` with no arguments to start a new note:

```
note> milk
note> eggs
note> bread
note> :wq
```

**While writing:**
| Command | Action |
|---|---|
| `:w` | Save and keep adding lines |
| `:wq` | Save and quit |
| `:q` | Quit without saving |
| `:u` | Undo the last line |

**From the command line:**
| Command | Action |
|---|---|
| `note :ls` | List all notes |
| `note :find <term>` | Search by note ID or content |
| `note :date <query>` | Search by creation date (e.g. `2026-08-06`) |
| `note :edit <id>` | Edit a note line by line (Enter keeps a line, typing replaces it, then continue writing new lines) |
| `note :d <id>` | Delete a note |

Notes save locally instantly and sync to your server automatically and silently in the background — no manual sync command needed. If you're offline, the note just stays local until the next save attempt succeeds.

## Security

- The server only listens on its Tailscale IP - it's never exposed to the public internet, even if the VPS also hosts other public sites.
- Requests from `127.0.0.1` (loopback) are explicitly rejected, closing a local-bypass loophole.
- Every request is verified against Tailscale's `WhoIs`, so only recognized, authenticated devices on your Tailnet can read, write, or delete notes.

## License

[MIT](LICENSE)
