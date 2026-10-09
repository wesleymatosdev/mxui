# mxui — Discord-shaped interactive components over native Matrix

**Thesis:** the missing piece of Matrix is rich interactions (buttons, selects,
modals, slash commands). Every platform that has them gives up privacy; every
private platform lacks them. mxui is an interaction layer over native Matrix
rooms — custom events any client can render — plus a bot gateway and a Discord
Bot API compatibility slice, so existing Discord bots and agent frameworks run
against a self-hosted Matrix backend by changing one base URL.

## Status (2026-10-09)

Tagged and e2e-verified through **v0.5.0**. See ROADMAP.md for the ladder.

- **v0.1.0** — `io.mxui.component_message`/`component_action` events, HMAC
  trigger + freshness + dedupe verification, message edits, web client with
  real buttons. [evidence/01,02]
- **v0.2.0** — composer, select menus, modals (`modal_present`/`modal_submit`),
  slash commands with ephemeral replies. [evidence/v02-*]
- **v0.4.0** — `mxui-server` gateway: bots register token+webhook, send via
  REST (`POST /api/v1/messages`), receive interactions as webhooks. The
  webhook-bot example has zero Matrix code. [evidence/v04-*]
- **v0.5.0** — Discord Bot API slice: `POST/PATCH /v10/channels/:id/messages`,
  `Bot`-prefixed auth, Discord component JSON → mxui blocks, Discord-shaped
  interaction webhooks. **A real discord.js REST client completed a button
  interaction round trip** (post → render → click → interaction webhook →
  PATCH edit) with only `api:` pointed at mxui-server. [evidence/v05-*]

## Quickstart (this machine)

```bash
# homeserver: continuwuity docker on :6167 (registration token in compose)
cd ~/projects/personal/mxui
MXUI_REG_TOKEN=mxui-mvp-dev-token go run ./cmd/mxui-server   # gateway :8790
cd web && python3 -m http.server 8791                         # client
open http://localhost:8791                                    # login mxui-clicker / clicker-pass-1

# a Discord bot, unmodified except the base URL:
cd examples/discord-bot && node bot.js   # needs discord.js in NODE_PATH
```

## Event schema (io.mxui.*)

- `io.mxui.component_message` — persistent blocks, `m.in_reply_to` the
  human-readable fallback message, `expiry` unix ts.
- `io.mxui.component_action` — {interactionId, messageEvent, componentId,
  value, userId, roomId, ts}. v1 verification: freshness ±120s, expiry,
  single-use per (message, component, user). v0 HMAC triggerId still honored.
- `io.mxui.modal_present` / `io.mxui.modal_submit` — targeted dialogs
  (client opens when action.userId matches).
- `io.mxui.command` — {command, args, userId} from the composer.

Trust model (v1) = Discord components: any room member may click; the gateway
dedupes, enforces TTL, and forwards. Per-user signing + E2EE rooms are v0.6.

## Repo layout

- `cmd/mxui-server/` — bot gateway + Discord compat routes (Go, SQLite)
- `internal/mxevent/` — event schema, trigger verification (tested)
- `internal/bot/` — thin Matrix CS-API client
- `web/` — single-file client (no build, no deps)
- `examples/` — approval-bot (native), demo-bot (full surface),
  webhook-bot (zero Matrix code), discord-bot (real discord.js)
- `evidence/` — e2e scripts + screenshots for every tagged version

## Not yet (v0.6+)

E2EE rooms (Megolm; the moat — server never holds room keys), ed25519 webhook
signatures, modals over the Discord slice, MSC draft, multi-room routing,
docker packaging.

## Homeserver notes (this machine)

- continuwuity container `continuwuity`, port 6167, server_name `macbook.tailfddf66.ts.net`
- registration token `mxui-mvp-dev-token` (compose), encryption enabled, federation disabled
