import { build } from 'bun'

const opts = { minify: true, target: 'browser' }

await build({ ...opts, entrypoints: ['src/article.js', 'src/comments.js', 'src/nav.js'], outdir: 'assets/js' })
await build({ ...opts, entrypoints: ['src/shiki.js'], outdir: 'assets/js' })

await build({
    ...opts,
    entrypoints: [
        'src/langs/bash.js',
        'src/langs/css.js',
        'src/langs/go.js',
        'src/langs/html.js',
        'src/langs/javascript.js',
        'src/langs/json.js',
        'src/langs/rust.js',
        'src/langs/typescript.js',
    ],
    outdir: 'assets/js/langs',
})
