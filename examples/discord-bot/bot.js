// discord-bot: a REAL discord.js REST client pointed at mxui-server.
// v0.5.0 acceptance: sends a component message through the Discord API shape,
// receives an interaction webhook in Discord's shape, edits the message.
const { REST, Routes } = require('discord.js');
const http = require('http');

const API = process.env.MXUI_API || 'http://127.0.0.1:8790';

async function main() {
  // register a bot with the gateway to obtain a token
  const reg = await fetch(API + '/api/v1/bots', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name: 'discord.js compat bot', webhookUrl: 'http://127.0.0.1:8793/interactions' })
  }).then(r => r.json());
  console.log('REGISTERED', reg.token.slice(0, 12) + '…');

  // The "change one base URL" promise: point discord.js REST at mxui-server.
  const rest = new REST({ version: '10', api: API, authPrefix: 'Bot' }).setToken(reg.token);

  const msg = await rest.post(Routes.channelMessages('demo'), {
    body: {
      content: 'discord.js says: deploy v3.0.0?',
      components: [{
        type: 1,
        components: [
          { type: 2, style: 3, label: 'Approve', custom_id: 'approve' },
          { type: 2, style: 4, label: 'Reject', custom_id: 'reject' }
        ]
      }]
    }
  });
  console.log('DISCORD-POST-OK id=' + msg.id);

  // webhook receiver in Discord interaction shape
  const server = http.createServer(async (req, res) => {
    if (req.url !== '/interactions') { res.writeHead(404); return res.end(); }
    let body = '';
    for await (const chunk of req) body += chunk;
    const ev = JSON.parse(body);
    console.log('INTERACTION-RECEIVED type=' + ev.type +
      ' custom_id=' + (ev.data && ev.data.custom_id) +
      ' user=' + (ev.user && ev.user.id));
    if (ev.type === 3 && ev.data.custom_id === 'approve') {
      await rest.patch(Routes.channelMessage('demo', ev.message.id), {
        body: { content: 'discord.js deploy — ✅ Approved via interaction' }
      });
      console.log('DISCORD-PATCH-OK');
    }
    res.writeHead(200);
    res.end(JSON.stringify({ type: 4, data: { content: 'ok' } }));
  });
  server.listen(8793, () => console.log('WEBHOOK-UP :8793'));
}
main().catch(e => { console.error('FAIL:', e.message); process.exit(1); });
