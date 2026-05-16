export async function onRequestPost({ request, env }) {
  let body
  try { body = await request.json() } catch { return err(400, 'Ogiltig JSON') }

  const { id = '', parent_id = '', post = '', name = '', email = '', comment = '', turnstileToken = '' } = body
  if (!id.trim() || !post.trim() || !name.trim() || !email.trim() || !comment.trim()) return err(400, 'Ogiltiga fält')
  if (!await verifyTurnstile(turnstileToken, env.TURNSTILE_SECRET)) return err(403, 'Bot-verifiering misslyckades')

  await env.DB.prepare(
    'INSERT INTO comments (id, parent_id, post, name, email, comment, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)'
  ).bind(id.trim(), parent_id.trim(), post.trim(), name.trim(), email.trim(), comment.trim(), new Date().toISOString()).run()

  return ok()
}

async function verifyTurnstile(token, secret) {
  if (!token || !secret) return false
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
