(function () {

    // ── Toast ─────────────────────────────────────────────────────────────────

    var toastEl = null

    function toast(msg) {
        if (!toastEl) {
            toastEl = document.createElement('div')
            toastEl.className = 'toast'
            document.body.appendChild(toastEl)
        }
        toastEl.textContent = msg
        toastEl.classList.remove('toast-hide')
        toastEl.classList.add('toast-show')
        clearTimeout(toastEl._timer)
        toastEl._timer = setTimeout(function () {
            toastEl.classList.add('toast-hide')
            toastEl.addEventListener('transitionend', function handler() {
                toastEl.classList.remove('toast-show', 'toast-hide')
                toastEl.removeEventListener('transitionend', handler)
            })
        }, 1800)
    }

    // ── Anchor links ──────────────────────────────────────────────────────────

    function slugify(text) {
        return text.trim()
            .toLowerCase()
            .replace(/[^\w\s-]/g, '')
            .replace(/\s+/g, '-')
            .replace(/-{2,}/g, '-')
    }

    document.querySelectorAll('h2, h3, h4').forEach(function (h) {
        if (!h.id) {
            h.id = slugify(h.textContent)
        }

        var icon = document.createElement('span')
        icon.className = 'anchor-icon'
        icon.textContent = '§'
        icon.setAttribute('aria-hidden', 'true')
        h.insertBefore(icon, h.firstChild)

        icon.addEventListener('click', function (e) {
            e.stopPropagation()
            if (h.classList.contains('anchor-copied')) return
            var url = location.href.split('#')[0] + '#' + h.id
            navigator.clipboard.writeText(url).then(function () {
                toast('Link copied')
                icon.textContent = '✓'
                h.classList.add('anchor-copied')
                setTimeout(function () {
                    h.classList.add('anchor-fading')
                    setTimeout(function () {
                        h.classList.remove('anchor-copied', 'anchor-fading')
                        icon.textContent = '§'
                    }, 300)
                }, 500)
            })
        })
    })

}())
