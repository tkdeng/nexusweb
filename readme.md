# Nexus Web 🚀

Nexus Web is a high-performance Go web framework designed for developers who want the simplicity of [Fiber](https://github.com/gofiber/fiber) but prefer the stability and compatibility of the built-in `net/http` package.

---

## ✨ Key Features

- **Built on net/http**: Standard library stability with a Fiber-like DX.
- **Markdown Native**: Full GitHub-Flavored Markdown (GFM) support. Any .md file is automatically processed as a page.
- **Cascading Templates**: Automatic inheritance—missing files are inherited from the nearest parent directory.
- **Shortcode Plugins**: WordPress-style logic for reusable UI components.

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
  // Initialize the app with the template directory and configuration
  app, err := nxweb.New("./app", nxweb.Config{
    Port: 8080,
    Vars: nxweb.Map{
      "myVar": "This is a static constant", // Compiled once at startup
    },
  })

  // GET Route: Rendering a template with dynamic variables
  app.Get("/path", func(c *nxweb.Ctx) error {
    return c.Render("index", nxweb.Map{
      "dynamicVar": "This is injected at runtime!",
    })
  })

  // POST Route: Demonstrates dynamic parameters and automatic sanitization
  app.Post("/api/user/:id", func(c *nxweb.Ctx) error {
    // c.Params contains URI segments extracted by the router
    userID := c.Params["id"]

    // c.Body(key) returns (any, bool) for JSON or Form data
    val, ok := c.Body("name")
    if !ok {
      return c.Json(nxweb.JSON{"error": "Name is required"})
    }

    // ToType[T] converts the value to the requested type AND 
    // automatically ensures valid UTF-8 for string, []byte, and byte.
    userName := nxweb.ToType[string](val)

    return c.Json(nxweb.JSON{
      "status":  "success",
      "message": "Profile updated",
      "data": nxweb.JSON{
        "id":   userID,
        "name": userName,
      },
    })
  })

  // Start the server
  log.Fatal(app.Listen())
}

```

## 🛠 Template Syntax

| Feature | Syntax | Description |
| :--- | :--- | :--- |
| **Embed** | `{@file}` | Embeds a file with cascading inheritance. |
| **Static** | `{var}` | Pre-compiled at startup. HTML-escaped. |
| **Dynamic** | `{$var}` | Runtime variable via `c.Render`. |
| **Raw** | `{#var}` | Renders without HTML escaping. |
| **Escaped Arg** | `{=var}` | Safely escapes variable for HTML attributes. |
| **Attr Guard** | `{class="var"}` | Renders the attribute only if `var` is not empty. |
| **Default** | `{var\|def}` | Provides a fallback value if `var` is empty. |
| **If** | `{?var{...}}` | Renders content if `var` is present/true. |
| **Unless** | `{!var{...}}` | Renders content if `var` is missing/false. |
| **Plugin** | `{:name}` | Executes a custom shortcode/plugin (supports optional `{content}`). |

## 📂 Cascading Inheritance

Nexus Web uses a recursive search for any embedded file (`{@filename}`). This applies to **all** file types.

- `#layout.html`: The entry point for the engine.
- `@widget.html`: Reserved for widgets, logic, and error handling (e.g., `@404.html`).
- `*.html / *.md`: Components embedded via `{@filename}`.

**Example:**
If `/blog/index.html` calls `{@sidebar}`, but `/blog/sidebar.html` is missing, the engine automatically "climbs" the directory tree to use the root `/sidebar.html`.

## 🔌 Plugins

```go
import "github.com/tkdeng/nexusweb/plugins"

func init() {
  // Runs every render
  plugins.New("button", func(args map[string]string, cont []byte, static bool) ([]byte, error) {
    return []byte("<button>"+args["text"]+"</button>"), nil
  })

  // Runs once at compile-time (Static)
  plugins.New("fast", func(args map[string]string, cont []byte, static bool) ([]byte, error) {
    return []byte("<div>Optimized</div>"), nil
  }, true)
}
```
