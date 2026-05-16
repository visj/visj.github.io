(function () {
    var nav = document.querySelector('nav')
    var ticking = false
    window.addEventListener('scroll', function () {
        if (!ticking) {
            requestAnimationFrame(function () {
                nav.classList.toggle('nav-scrolled', window.scrollY > 0)
                ticking = false
            })
            ticking = true
        }
    }, { passive: true })
})()
