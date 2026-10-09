# mxui roadmap — the honest ladder

Each step ships only with real verification evidence (live runs, headless browser
proofs, green gates) and a git tag. No step is "done" because code exists.

## v0.1.0 (done, tagged)
Two Matrix users, io.mxui.component_message + component_action events, HMAC/freshness
verification, message edits on click, minimal web client rendering real buttons.
E2E proof: evidence/01,02 + e2e.js (Playwright headless).

## v0.2.0 — a real client, more components
- Composer: send plain messages from the web client; message list grouping.
- Select menus + multi-option rendering.
- Modal dialogs (io.mxui.modal_present → io.mxui.modal_submit round trip).
- Acceptance: headless run posts a message, opens a modal from a button, submits
  it, bot edits message with the submitted value.

## v0.3.0 — slash commands
- Composer parses "/" commands → io.mxui.command event; bot replies (ephemeral
  style = direct message to the user's session, rendered as a toast/inline).
- Command registry on the bot side; autocomplete hint list in the composer.
- Acceptance: headless run types /deploy 1.2.3, bot's response appears inline.

## v0.4.0 — mxui-server: the bot gateway
- Standalone Go service: bots register (token, webhook URL); POST /api/v1/messages
  to send interactive messages; interactions POSTed to the bot webhook (Discord-
  interaction-shaped JSON). SQLite state (dedupe, routing) instead of memory.
- Acceptance: approval-bot rewritten as a plain HTTP webhook bot (no Matrix code
  at all) driving the same room through mxui-server.

## v0.5.0 — Discord compatibility slice
- mxui-server exposes Discord Bot API subset: POST /api/v10/channels/:id/messages,
  interactions webhook (signature-verified like Discord's ed255 scheme), channel↔room
  mapping. HTTP-interactions Discord bots run with a base-URL change.
- Acceptance: a real off-the-shelf Discord bot library (discord.js or discord.py
  minimal script) talks to mxui-server and completes a button interaction.

## v0.6.0 — E2EE rooms (the moat)
- Bot participates in Megolm sessions (mautrix-go crypto or documented fallback);
  component events encrypted; server never holds room keys.
- Acceptance: interactive loop works in an encrypted room; homeserver DB inspection
  shows ciphertext for component events.

## v0.7.0 — packaging
- docker-compose: continuwuity + mxui-server + web client + example bot, one
  command up; README quickstart verified from scratch dir.

## v1.0.0 — honest 1.0
- Full test suite, race-clean, docs, MSC draft for the event schema, evidence pack.

Deviations get written here as they happen — the roadmap is the contract.
