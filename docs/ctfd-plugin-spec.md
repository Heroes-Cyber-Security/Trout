# CTFd Plugin Specification

Trout requires a CTFd plugin to synchronize flag submissions when using open-source CTFd (which lacks built-in webhooks).

## Plugin Requirements

A minimal CTFd plugin that:

1. Hooks into CTFd's submission lifecycle
2. POSTs submission events to Trout's `/api/v1/submissions` endpoint
3. Signs requests with a shared HMAC-SHA256 secret

## API Contract

### POST /api/v1/submissions

**Headers:**
```
Content-Type: application/json
X-Trout-Signature: t={unix_ts},v1={hmac_hex}
```

**Body:**
```json
{
  "user_id": "42",
  "user_name": "player1",
  "challenge_id": "web-xss-easy",
  "challenge_name": "XSS Challenge",
  "flag": "CTF{...}",
  "status": "correct"
}
```

**Signature Algorithm:**
```
HMAC-SHA256(secret, "{timestamp}.{request_body}")
```

**Response:**
```json
{"success": true}
```

## Example Plugin Structure

```
CTFd/plugins/trout/
├── __init__.py
├── config.json
└── README.md
```

## Configuration

The plugin reads from CTFd's admin config panel:
- `trout_url`: Trout server URL (e.g. `http://trout:8080`)
- `trout_secret`: Shared HMAC secret

## Event Hooks

The plugin should hook into CTFd's `on_submission` event (via `CTFd.events`) and POST to Trout when a correct submission is detected.

## Implementation Notes

- Plugin development is tracked separately from Trout itself
- The plugin should be installed manually by CTFd admins
- After installation, set `Plugin Installed = true` in Trout's CTFd settings page
