# mxui — Discord-shaped interactive components over native Matrix

**Thesis:** the missing piece of Matrix is rich interactions (buttons, modals, slash
commands) — every platform that has them gives up privacy (Discord/Slack), and every
private platform lacks them (Matrix E2EE rooms, Telegram secret chats). mxui defines a
custom-event interaction layer that rides native Matrix rooms, including encrypted ones,
and ships a bot-side SDK so any agent/bot can use it today.

## Architecture (MVP)

```
mxui-server (Go)
├── Bot SDK surface: POST /api/v1/messages        → send message + components
│                      POST /api/v1/interactions  → webhook callbacks to bot URL
├── Interaction event schema: io.mxui.component_action (custom event type)
│   { interactionId, messageId, componentId (actionId), value, user, room, ts,
│     triggerId (single-use, HMAC, TTL 60s — anti-spoof, UIKit triggerId semantics) }
├── Render fallback: m.room.message with org.matrix.custom.html fallback markup
│   (plain clients show "[Approve] [Reject]" links w/ mxui:// deep-links)
├── Message edits on interaction: m.room.message edits referencing source event
└── E2EE path (v2): bot is a normal room member joining Megolm sessions via
    vodozemac (matrix-nio equivalent in Go: matrix-nio-e2e has no Go port; use
    embedded Python sidecar or Rust FFI vodozemac) — server NEVER holds room keys.
```

## What the MVP proves

1. Button click → `io.mxui.component_action` event → bot webhook → bot edits message.
   Full Discord-style round trip over native Matrix, zero bridges.
2. The interaction survives federation semantics (custom events replicate like any event).
3. Fallback rendering means stock clients (Element) show usable text, not broken blobs.
4. Non-encrypted rooms in v1 (server-to-server TLS); E2EE rooms are v2 with client-held
   keys — the commercial moat ("even your agents can't read rooms they aren't keyed into").

## Repo layout

- `cmd/mxui-server/` — HTTP API for bots + event emission/sync
- `internal/mxevent/` — the interaction event schema + triggerId HMAC
- `internal/bot/` — client API, message-with-components composer, edit helpers
- `examples/approval-bot/` — demo: "Deploy v1.2.3? [Approve] [Reject]"
- `web/` — minimal client page that renders components from custom events (demo client)

## Not in MVP (deliberately)

- Modals/contextual bars (need a real client; v2)
- Slash commands (needs client composer integration; v2)
- The Discord-bot-API compat layer (consequence of the interaction layer, later)
- MSC proposal writing (after the schema survives dogfooding)

## Homeserver notes (this machine)

- continuwuity container `continuwuity`, port 6167, server_name `macbook.tailfddf66.ts.net`
- registration open, encryption enabled, federation disabled (fine for MVP)
