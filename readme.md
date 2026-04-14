# Nexus Web 🚀

Nexus Web is a high-performance Go web framework designed for developers who want the simplicity of [Fiber](https://github.com/gofiber/fiber) but prefer the stability and compatibility of the built-in `net/http` package.

It features a unique **hybrid template engine** that pre-compiles static variables at startup while allowing flexible dynamic injection at runtime.

---

## ✨ Key Features

- **Zero Dependencies**: Built on top of standard `net/http`.
- **Static Pre-compilation**: Faster renders by resolving static variables before the server starts.
- **Smart Layouts**: Automatic inheritance of `head.html` and `body.html` across directories.
- **Shortcode Plugins**: WordPress-style plugins for reusable UI components.
- **Markdown Native**: Full GitHub-flavored markdown support with YAML frontmatter.

## 📦 Installation

```shell
go get github.com/tkdeng/nexusweb
```

## 🚀 Quick Start

```go

import (
  nxweb "github.com/tkdeng/nexusweb"
)

func main(){
  app, err := nxweb.New("./test", nxweb.Config{
    Port: 8080,
    Vars: nxweb.Map{
      "myVar": "This is a static constant" // Pre-compiled for performance
    },
  })

  app.Use("/path", func(c *nxweb.Ctx) error {
    return c.Render("index", Map{
      "myVar": "This is dynamic!" // Overrides or adds at runtime
    })
  })

  app.Listen()
}

```

## HTML

Files starting with `#` such as `#layout.html` are for layouts.

Files starting with `@` such as `@widget.html` or `@error.html` are for widgets, errors, apis, and dynamic content.

Regular files such as `body.html`, `head.html`, `custom.html` are files that can be
embedded, starting with the layout as the entry point. Your `#layout.html` file should include a `{@body}` and optionally `{@head}` embed, to embed the relative files. These files (`body.html`, etc) can optionally embed more files, such as `{@custom}` for example.

You can also embed `@widget.html` files directly.

If your `body.html` file in a higher directory/page (`about/body.html`, etc) is missing other files such as `head.html` or `custom.html` that its trying to embed, it will automatically inherit the parent files for those embeds.

The file `@error.html` automatically catches errors, sending `{status}` and `{message}` variables. You can also make separate pages for specific error statuses, in a similar way (`@404.html`, `@500.html`, etc).

```html

<body>
  {myVar} <!-- will escape html by default -->

  <!-- adding an `=` means the var will escape quotes for html args -->
  <div class="{=escapeArgsVar}">
    {#rawHtmlVar} <!-- adding `#` means it will not escape html -->
  </div>

  {$dynamicVar} <!-- adding `$` makes it dynamic and compile at runtime -->

  <!-- putting the key before the `=`, allows the whole arg to disappear when empty -->
  <div {class="myVar"}></div>

  <!-- append an `|` at the end of the var name, to set a default/fallback value -->
  <div def-arg="{=myVar|default}"></div>

  <!-- if statements (if myVar exists) -->
  {?myVar{
    <form>
      ...
    </form>
  }}

  <!-- unless statements (if myVar does not exist) -->
  {!myVar{
    <article>
      reasons `myVar` is missing or has an empty value
      ...
    </article>
  }}

  <!-- plugins/shortcodes can easily be embedded (similar to wordpress shortcodes) -->
  {:plugin arg1="value" arg2 {
    content
  }}

  <!-- this framework comes with some builtin plugins/functions -->
  {:lorem}
</body>

```

## plugins

```go

import (
  plugins "github.com/tkdeng/nexusweb/plugins"
)

func init(){
  // create new plugin
  plugins.New("button", func(args map[string]string, cont []byte, static bool) ([]byte, error) {
		return []byte("<button>"+args["name"]+"<button>"), nil
	})

  // create static plugin
  plugins.New("fastbutton", func(args map[string]string, cont []byte, static bool) ([]byte, error) {
		return []byte("<button>"+args["name"]+"<button>"), nil
	}, true) // adding true makes this plugin run at compiletime
}

```

## Markdown

```md

---
title: "Web Server"
ymlvars: "my yml vars at top of page"
---

<!-- adding yml vars overrides {$vars} to a static value for this page -->
<!-- these will be added at compiletime -->
<!-- this feature is also supported in regular html files -->

# Markdown Supported (based on github)

```
