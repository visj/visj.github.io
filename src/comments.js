document.addEventListener('DOMContentLoaded', () => {
  const section = document.querySelector('[data-post]')
  if (!section) return

  const slug = section.dataset.post
  const form = section.querySelector('.comment-form')
  const status = section.querySelector('.form-status')
  const notice = section.querySelector('.reply-notice')
  const noticeName = notice.querySelector('.reply-to-name')
  const parentInput = form.querySelector('[name=parent_id]')

  function showNotice(name, parentId) {
    parentInput.value = parentId
    noticeName.textContent = name
    notice.style.display = 'flex'
  }

  function hideNotice() {
    parentInput.value = ''
    notice.style.display = 'none'
  }

  section.addEventListener('click', e => {
    const btn = e.target.closest('.reply-btn')
    if (!btn) return
    const comment = btn.closest('.comment')
    showNotice(comment.querySelector('.comment-name').textContent, comment.id)
    form.querySelector('[name=name]').focus()
  })

  notice.querySelector('.cancel-reply').addEventListener('click', hideNotice)

  form.addEventListener('submit', async e => {
    e.preventDefault()
    const submit = form.querySelector('button[type=submit]')
    submit.disabled = true
    status.textContent = ''

    const id = 'c-' + Math.random().toString(36).slice(2, 8)
    try {
      const turnstileInput = form.querySelector('[name="cf-turnstile-response"]')
      const r = await fetch('/api/comment', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          id,
          parent_id: parentInput.value || '',
          post: slug,
          name: form.querySelector('[name=name]').value,
          email: form.querySelector('[name=email]').value,
          comment: form.querySelector('[name=comment]').value,
          turnstileToken: turnstileInput ? turnstileInput.value : '',
        }),
      })
      if (r.ok) {
        status.textContent = 'Tack. Din kommentar granskas innan publicering.'
        form.reset()
        hideNotice()
      } else {
        status.textContent = 'Något gick fel — försök igen.'
      }
    } catch {
      status.textContent = 'Kunde inte skicka. Kontrollera anslutningen.'
    }
    submit.disabled = false
  })
})
