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

    // ── References ────────────────────────────────────────────────────────────

    var refPopup = null
    var activeRef = null

    function buildRefMap() {
        var map = {}
        var counter = 1
        document.querySelectorAll('sup.ref[data-ref]').forEach(function (sup) {
            var key = sup.dataset.ref
            if (!(key in map)) {
                map[key] = counter++
            }
            var n = map[key]
            sup.textContent = n
            sup.setAttribute('data-ref-n', n)
        })
        return map
    }

    function showRefPopup(sup) {
        var key = sup.dataset.ref
        var li = document.getElementById(key)
        if (!li) return

        if (!refPopup) {
            refPopup = document.createElement('div')
            refPopup.className = 'ref-popup'
            document.body.appendChild(refPopup)
            refPopup.addEventListener('click', function (e) {
                e.stopPropagation()
            })
        }

        // Toggle off if clicking the same ref again
        if (activeRef === sup && refPopup.classList.contains('ref-popup-visible')) {
            hideRefPopup()
            return
        }
        activeRef = sup

        refPopup.innerHTML = ''

        var body = document.createElement('div')
        body.className = 'ref-popup-body'
        body.innerHTML = li.innerHTML
        refPopup.appendChild(body)

        var footer = document.createElement('div')
        footer.className = 'ref-popup-footer'
        var jump = document.createElement('a')
        jump.href = '#references'
        jump.textContent = '→ See all references'
        jump.addEventListener('click', hideRefPopup)
        footer.appendChild(jump)
        refPopup.appendChild(footer)

        // Position: measure first pass off-screen
        refPopup.style.visibility = 'hidden'
        refPopup.style.display = 'block'
        refPopup.classList.add('ref-popup-visible')

        var rect = sup.getBoundingClientRect()
        var pw = refPopup.offsetWidth
        var margin = 8

        var left = rect.left + window.scrollX
        var top = rect.bottom + window.scrollY + margin

        // Clamp horizontally
        var maxLeft = window.innerWidth - pw - 16
        if (left > maxLeft) left = maxLeft
        if (left < 16) left = 16

        refPopup.style.left = left + 'px'
        refPopup.style.top = top + 'px'
        refPopup.style.visibility = ''
    }

    function hideRefPopup() {
        if (refPopup) refPopup.classList.remove('ref-popup-visible')
        activeRef = null
    }

    function initReferences() {
        var sups = document.querySelectorAll('sup.ref[data-ref]')
        if (!sups.length) return

        buildRefMap()

        sups.forEach(function (sup) {
            sup.addEventListener('click', function (e) {
                e.stopPropagation()
                showRefPopup(sup)
            })
        })

        document.addEventListener('click', hideRefPopup)
        document.addEventListener('keydown', function (e) {
            if (e.key === 'Escape') hideRefPopup()
        })
    }

    initReferences()

}())
