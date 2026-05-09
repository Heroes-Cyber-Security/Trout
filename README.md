# Trout

Infrastructure-agnostic and DX-first CTF dynamic flag server for Jeopardy and AnD formats.

## Features

- **Dynamic Leetspeak Flags** — deterministic per-user flag generation via leetspeak character transformation (no hash suffixes)
- **Netcat QnA Server** — per-challenge TCP listeners with sequential Q&A flow
- **CTFd Integration** — token-based identity assignment and webhook event subscription
- **Discord Notifications** — webhook relay for flag generation and solve events
- **Admin Web UI** — server-rendered dashboard with challenge CRUD, CTFd config, and Discord settings
- **Internal HTTP API** — challenge authors fetch user-specific flags on the internal network
- **Custom CTF Server API** — standardized `/api/v1/submissions` endpoint for non-CTFd platforms
- **SQLite Config** — all configuration stored in a single SQLite file

## Quick Start

```bash
# Build
go build -o trout .

# Run
TROUT_ADMIN_PASSWORD=changeme ./trout --db trout.db
```

## CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--db` | `trout.db` | Path to SQLite database |
| `--http-addr` | `:8080` | Address for admin UI and webhooks |
| `--internal-addr` | `127.0.0.1:9125` | Address for internal flag API |
| `--admin-password` | `$TROUT_ADMIN_PASSWORD` | Admin UI password |

## Architecture

```
External Network          Internal Network
───────────────           ────────────────
Admin Browser             CTFd / Custom CTF Server
    │                              │
    ▼                              ▼
:8080/admin/*              :8080/ctfd/webhook
(Basic Auth)               :8080/api/v1/submissions

                           Challenge Containers
                                  │
                                  ▼
                            :9125/internal/flag

Players → :{port}/tcp (netcat QnA → flag)
```

## Netcat Flow

1. Connect to challenge port
2. Answer sequential questions
3. Provide CTFd token (or skip for anonymous)
4. Receive deterministic leetspeak-transformed flag

## CTFd Integration

- **Enterprise**: CTFd sends webhooks to `/ctfd/webhook` endpoint
- **Open-Source**: Requires installing the Trout plugin (see `docs/ctfd-plugin-spec.md`)
- **No CTFd**: Custom CTF servers can POST to `/api/v1/submissions`

## Internal API

Challenge authors fetch flags for users on the internal network:

```bash
curl http://127.0.0.1:9125/internal/flag?user_id=42\&challenge_id=web-xss
```
