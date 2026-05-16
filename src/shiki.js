import { createHighlighterCore } from 'shiki/core'
import { createJavaScriptRegexEngine } from 'shiki/engine/javascript'
import themeLight from 'shiki/themes/light-plus.mjs'

createHighlighterCore({
    themes: [themeLight],
    langs: window.__shikiLangs || [],
    engine: createJavaScriptRegexEngine()
}).then(function (hl) {
    document.querySelectorAll('pre code[class*="language-"]').forEach(function (el) {
        var match = el.className.match(/language-(\w+)/)
        var lang = match ? match[1] : 'text'
        var html = hl.codeToHtml(el.textContent, { lang: lang, theme: 'light-plus' })
        var pre = el.closest('pre')
        if (pre) {
            pre.outerHTML = html
        }
    })
})
