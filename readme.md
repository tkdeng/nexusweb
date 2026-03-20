# Nexus Web

A simple webserver.

## Installation

```shell
go get github.com/tkdeng/nexusweb
```

## Usage

```go

import (
  nxweb "github.com/tkdeng/nexusweb"
)

func main(){
  app, err := nxweb.New("./test", nxweb.Config{
    Port: 8080,
    ...

    Vars: nxweb.Map{ // note: these will be precompiled statically before the server runs
      "myVar": "constant var value"
      ...
    },
  })

  app.Use("/path", func(c *nxweb.Ctx) error {
    return c.Render("index", Map{
      "myVar": "dynamic var value"
      ...
    })
  })

  app.Listen()
}

```

## HTML

Files starting with `#` such as `#layout.html` are primarelly for layouts. These files will have vars precompiled, but keep embeds dynamic.

Files starting with `@` such as `@widget.html` or `@error.html` are primarelly for widgets, errors, and dynamic content. These will keep vars and embeds dynamic.

Regular files such as `body.html`, `head.html`, `custom.html` are files that can be
embedded, starting with the layout as the entry point. Your `#layout.html` file should include a `{@body}` and optionally `{@head}` embed, to embed the relative files. These files (`body.html`, etc) can optionally embed more files, such as `{@custom}` for example.

You can also embed `@widget.html` files directly, and there content and variables will remain dynamic, even while the content and variables of static files remain static.

If your `body.html` file in a higher directory/page (`about/body.html`, etc) is missing other files such as `head.html` or `custom.html` that its trying to embed, it will automatically inherit the parent files for those embeds.

The file `@error.html` automatically catches errors, sending `{status}` and `{message}` variables. You can also make separate pages for specific error statuses, in a similar way (`@404.html`, `@500.html`, etc).

```html

<body>
  {myVar} <!-- will escape html by default -->

  <!-- adding an `=` means the var will escape quotes for html args -->
  <div class="{=escapeArgsVar}">
    {#rawHtmlVar} <!-- adding `#` means it will not escape html -->
  </div>

  {$dynamicVar} <!-- adding `$` means it will be dynamic and compile at runtime -->

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
</body>

```
