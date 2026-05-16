export async function onRequestPost({ request, env }) {
  let body
  try { body = await request.json() } catch { return err(400, 'Ogiltig JSON') }

  const { name = '', email = '', message = '', turnstileToken = '' } = body
  if (!name.trim() || !message.trim()) return err(400, 'Namn och meddelande krävs')
  if (!await verifyTurnstile(turnstileToken, env.TURNSTILE_SECRET)) return err(403, 'Bot-verifiering misslyckades')

  await env.DB.prepare(
    'INSERT INTO contacts (id, name, email, message, created_at) VALUES (?, ?, ?, ?, ?)'
  ).bind(uid(), name.trim(), email.trim(), message.trim(), new Date().toISOString()).run()

  return ok()
}

async function verifyTurnstile(token, secret) {
  if (!secret) return true
  if (!token) return false
  const r = await fetch('https://challenges.cloudflare.com/turnstile/v0/siteverify', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ secret, response: token }),
  })
  return (await r.json()).success === true
}

function ok()             { return json({ ok: true }) }
function err(s, msg)      { return json({ error: msg }, s) }
function json(body, s=200) {
  return new Response(JSON.stringify(body), { status: s, headers: { 'Content-Type': 'application/json' } })
}
function uid() { return crypto.randomUUID().slice(0, 8) }
