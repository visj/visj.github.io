# visj.github.io

Personal site built with Go and deployed to Cloudflare Pages.

---

## Writing posts in HTML

Posts live in `views/posts/<slug>/index.html`. Each file has two sections: a `<head>` with metadata and a `<body>` with the content.

### Head metadata

```html
<head>
    <title>Post title</title>
    <meta name="date" content="2026-05-14">
    <meta name="author" content="Vilhelm Sjölund">
    <meta name="tags" content="skrivande, språk">
    <meta name="description" content="Short summary shown in previews.">
</head>
```

### Body structure

```html
<body>
    <h1>Main title</h1>
    <p class="byline">Vilhelm Sjölund &nbsp;|&nbsp; 14 maj 2026</p>

    <p>Opening paragraph...</p>

    <h2>Section heading</h2>
    <h3>Sub-section heading</h3>
</body>
```

---

## Common elements

### Blockquote

```html
<blockquote>
    <p>The quoted text goes here.</p>
    <cite>— Author Name, <em>Book Title</em></cite>
</blockquote>
```

The `<cite>` line is optional. Omit it if there is no clear attribution.

### Inline reference marker

Place a superscript next to the sentence being cited. The `data-ref` attribute must match the `id` of the corresponding list item in the references section.

```html
<p>Some claim supported by research.<sup class="ref" data-ref="mueller2014"></sup></p>
```

### Reference list

At the bottom of the post, list all sources in an ordered list. Each `<li>` gets an `id` that matches the `data-ref` values used above.

```html
<h2>Referenser</h2>
<ol class="references" id="references">
    <li id="mueller2014">Mueller, P. A. (2014). Title. <em>Journal</em>, 25(6), 1159–1168.
        <a href="https://doi.org/...">doi:...</a>
    </li>
    <li id="fadiman1998">Fadiman, A. (1998). <em>Ex Libris</em>. Publisher.
        <a href="https://...">Förlagets sida</a>
    </li>
</ol>
```

The JavaScript in `layout.html` (or a post script) reads the `data-ref` attributes and automatically numbers the superscripts to match their position in this list.

### Emphasis and strong

```html
<em>italic text</em>
<strong>bold text</strong>
```

### Unordered list

```html
<ul>
    <li>First point</li>
    <li>Second point</li>
</ul>
```

---

## Layout

`layout.html` is the shared shell. It wraps every page with the `<nav>`, `<main>`, and a small scroll-detection script. Two placeholders are replaced at build time:

- `{{HEAD}}` — injected from the post's `<head>` block
- `{{BODY}}` — injected from the post's `<body>` block
- `{{GLOBALCSS}}` — the hashed stylesheet `<link>` tag

Do not put full `<html>`, `<head>`, or `<body>` tags in post files — only their contents.
