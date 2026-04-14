package nxweb

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tkdeng/goutil"
	"github.com/tkdeng/nexusweb/compiler"
)

// Ctx represents the request/response context.
// It provides a unified API for data retrieval, routing parameters, and response control.
type Ctx struct {
	router *Router
	w      http.ResponseWriter
	r      *http.Request
	status int

	next     bool // Internal flag to signal execution of the next handler
	rendered bool // Internal flag to prevent double-rendering

	// Public Request Metadata
	Host     string // The hostname requested (e.g., example.com)
	RemoteIP string // The physical IP address of the immediate connection (extracted from RemoteAddr)
	IP       string // The End-User IP (extracted from X-Forwarded-For)

	Path   string // The sanitized request path
	Port   string // The port the request arrived on
	Method string // HTTP method (GET, POST, etc.)
	Type   string // Content-Type header of the request

	// Params contains dynamic route segments (e.g., :id)
	Params map[string]string

	query url.Values     // Cached URL search parameters
	form  url.Values     // Cached POST form data
	body  map[string]any // Cached JSON payload

	locals map[string]any
}

var ctxInitError error = errors.New("CTX Init Error!")

func (router *Router) newCtx(w http.ResponseWriter, r *http.Request) (Ctx, error) {
	host, port, err := net.SplitHostPort(r.Host)
	if err != nil {
		return Ctx{}, errors.Join(ctxInitError, err)
	}

	// get ip address or remote host
	remoteIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remoteIP = r.RemoteAddr
	}

	if remoteIP == "" {
		return Ctx{}, errors.Join(ctxInitError, fmt.Errorf("unable to detect remote ip address"))
	}

	var userIP string
	if ip := goutil.Clean(r.Header.Get("X-Forwarded-For")); ip != "" {
		userIP = strings.Split(ip, ",")[0] // Take the first IP in the chain
	}

	if userIP == "" {
		userIP = remoteIP
	}

	ctx := Ctx{
		router: router,
		w:      w,
		r:      r,
		status: 200,

		Host:     goutil.Clean(host),
		RemoteIP: goutil.Clean(remoteIP),
		IP:       goutil.Clean(userIP),

		Path:   "/" + strings.Trim(goutil.Clean(r.URL.Path), "/"),
		Port:   goutil.Clean(port),
		Method: goutil.Clean(r.Method),
		Type:   goutil.Clean(r.Header.Get("Content-Type")),

		locals: make(map[string]any),
	}

	// ensure headers are valid

	if err := ctx.verifyOrigin(); err != nil {
		return ctx, err
	}

	if err := ctx.verifyHeaders(); err != nil {
		return ctx, err
	}

	if err := ctx.redirectSSL(); err != nil {
		return ctx, err
	}

	return ctx, nil
}

// Next flags the context to continue execution to the next handler in the stack.
// This is typically used in middleware to allow the request to reach the main handler.
func (ctx *Ctx) Next() error {
	ctx.next = true
	return nil
}

// Render processes a template file with a set of variables and writes the result to the response.
// It automatically handles path normalization (e.g., index files, suffix stripping) and
// identifies "Widgets" (paths starting with @). It merges global config variables,
// router-level variables, and local variables passed into the call.
func (ctx *Ctx) Render(path string, vars ...Map) error {
	if path == "/" || path == "" {
		path = "/index"
	} else if path[0] != '/' {
		path = "/" + strings.TrimSuffix(strings.TrimSuffix(path, ".html"), ".md")
	} else {
		path = strings.TrimSuffix(strings.TrimSuffix(path, ".html"), ".md")
	}
	path = strings.TrimRight(path, "/")

	isWidget := false
	if filename := filepath.Base(path); len(filename) != 0 && (filename[0] == '@' || filename != "index") {
		if filename[0] == '@' {
			isWidget = true
		} else {
			path += "/index"
		}
	}

	varList := map[string]string{
		"title":    goutil.Clean(ctx.router.Config.Title),
		"apptitle": goutil.Clean(ctx.router.Config.AppTitle),
		"desc":     goutil.Clean(ctx.router.Config.Desc),
		"icon":     goutil.Clean(ctx.router.Config.Icon),
		"public":   goutil.Clean(ctx.router.Config.PublicURI),
		"devmode":  goutil.ToType[string](ctx.router.Config.DevMode),
	}

	for k, v := range ctx.router.Config.Vars {
		varList[k] = goutil.Clean(v)
	}

	for k, v := range ctx.router.vars {
		varList[k] = goutil.Clean(v)
	}

	for _, m := range vars {
		// maps.Copy(varList, m)
		for k, v := range m {
			varList[k] = goutil.Clean(v)
		}
	}

	buf, err := compiler.Render(path, varList, isWidget)
	if err != nil {
		return err
	}

	if status, ok := varList["status"]; ok {
		if i, e := strconv.Atoi(status); e == nil {
			ctx.w.WriteHeader(i)
			ctx.w.Write(buf)
			return nil
		}
	}

	// ctx.w.WriteHeader(http.StatusOK)
	if ctx.status != 0 && ctx.status != 200 {
		ctx.w.WriteHeader(ctx.status)
	}
	ctx.w.Write(buf)
	return nil
}

// Error attempts to render a beautiful error page using a hierarchical fallback system.
// It searches for templates in the following order:
// 1. {path}/@{status} (e.g., /users/@404)
// 2. {path}/@error    (e.g., /users/@error)
// 3. @{status}        (e.g., @404)
// 4. @error           (Global error widget)
// 5. Plain text/HTML fallback if no templates are found.
func (ctx *Ctx) Error(path string, status int, msg string) error {
	path = strings.TrimRight(path, "/")

	err := ctx.Render(path+"/@"+strconv.Itoa(status), Map{
		"status":  strconv.Itoa(status),
		"message": msg,
	})

	if err != nil {
		err = ctx.Render(path+"/@error", Map{
			"status":  strconv.Itoa(status),
			"message": msg,
		})
	}

	if err != nil {
		err = ctx.Render("@"+strconv.Itoa(status), Map{
			"status":  strconv.Itoa(status),
			"message": msg,
		})
	}

	if err != nil {
		err = ctx.Render("@error", Map{
			"status":  strconv.Itoa(status),
			"message": msg,
		})
	}

	if err != nil {
		ctx.w.WriteHeader(status)
		ctx.w.Write([]byte("<h1>Error " + strconv.Itoa(status) + "</h1><h2>" + msg + "</h2>"))
	}

	return nil
}

// Write sends raw bytes to the response body.
func (ctx *Ctx) Write(buf []byte) error {
	if ctx.status != 0 && ctx.status != 200 {
		ctx.w.WriteHeader(ctx.status)
	}

	if _, err := ctx.w.Write(buf); err != nil {
		return err
	}
	return nil
}

// Json marshals the provided value into a JSON string and writes it to the response.
// It sets the 'Content-Type: application/json' header. An optional indent (number of spaces)
// can be provided for pretty-printing the output.
func (ctx *Ctx) Json(val any, indent ...int) error {
	ctx.Header("Content-Type", "application/json")

	var buf []byte
	var err error

	if len(indent) != 0 && indent[0] != 0 {
		buf, err = json.MarshalIndent(val, "", strings.Repeat(" ", indent[0]))
	} else {
		buf, err = json.Marshal(val)
	}

	if err != nil {
		return err
	}

	if ctx.status != 0 && ctx.status != 200 {
		ctx.w.WriteHeader(ctx.status)
	}

	if _, err := ctx.w.Write(buf); err != nil {
		return err
	}
	return nil
}

// Status sets the HTTP response status code.
// This is buffered within the Ctx and only sent to the client when a
// "Write" method (like JSON, String, etc.) is called. This allows headers
// to be modified even after the status is set.
func (ctx *Ctx) Status(status int) *Ctx {
	ctx.status = status
	return ctx
}

// Query returns the value of a query parameter and a boolean indicating if it exists
func (ctx *Ctx) Query(key string) (value string, ok bool) {
	if ctx.query == nil {
		ctx.query = ctx.r.URL.Query()
	}

	val, ok := ctx.query[key]
	if !ok {
		return "", false
	}

	if len(val) == 0 {
		return "", true
	}

	return goutil.Clean(val[0]), true
}

// SetQuery sets a query parameter
//
// If no value is provided, the key is deleted
func (ctx *Ctx) SetQuery(key string, value ...string) {
	if ctx.query == nil {
		ctx.query = ctx.r.URL.Query()
	}

	if len(value) == 0 {
		ctx.query.Del(key)
	} else {
		ctx.query.Set(key, value[0])
	}
}

// Body returns a value from the request body (JSON or Form) and a boolean for existence.
//
// If the Content-Type is application/json, it looks up the key in the JSON body.
// Otherwise, it looks up the key in the POST form data.
func (ctx *Ctx) Body(key string) (value any, ok bool) {
	if strings.Contains(ctx.Type, "application/json") {
		if ctx.body == nil {
			ctx.body = make(map[string]any)
			json.NewDecoder(ctx.r.Body).Decode(&ctx.body)
		}

		val, ok := ctx.body[key]
		if !ok {
			return nil, false
		}

		if v, ok := val.(string); ok {
			val = goutil.Clean(v)
		} else if v, ok := val.([]byte); ok {
			val = goutil.Clean(v)
		}

		return val, true
	}

	if ctx.form == nil {
		if err := ctx.r.ParseForm(); err != nil {
			return nil, false
		}
		ctx.form = ctx.r.PostForm
	}

	val, ok := ctx.form[key]
	if !ok {
		return nil, false
	}

	if len(val) == 0 {
		return nil, true
	}

	return goutil.Clean(val[0]), true
}

// SetBody sets a post body parameter
//
// If no value is provided, the key is deleted
func (ctx *Ctx) SetBody(key string, value ...string) {
	if strings.Contains(ctx.Type, "application/json") {
		if ctx.body == nil {
			ctx.body = make(map[string]any)
			json.NewDecoder(ctx.r.Body).Decode(&ctx.body)
		}

		if len(value) == 0 {
			delete(ctx.body, key)
		} else {
			ctx.body[key] = value[0]
		}
		return
	}

	if ctx.form == nil {
		if err := ctx.r.ParseForm(); err != nil {
			return
		}
		ctx.form = ctx.r.PostForm
	}

	if len(value) == 0 {
		ctx.form.Del(key)
	} else {
		ctx.form.Set(key, value[0])
	}
}

// Header is a dual-purpose method for request and response headers.
// If a value is provided, it sets the response header and returns that value.
// If no value is provided, it returns the sanitized value of the existing response header.
func (ctx *Ctx) Header(key string, value ...string) string {
	if len(value) != 0 {
		val := goutil.Clean(value[0])
		ctx.w.Header().Set(goutil.Clean(key), val)
		return val
	}

	return goutil.Clean(ctx.r.Header.Get(key))
}

// AddHeader appends a value to a response header without overwriting existing values.
func (ctx *Ctx) AddHeader(key string, value string) {
	ctx.w.Header().Add(goutil.Clean(key), goutil.Clean(value))
}

// DelHeader removes a specific header from the response.
func (ctx *Ctx) DelHeader(key string) {
	ctx.w.Header().Del(goutil.Clean(key))
}

// Cookie is a dual-purpose method for getting and setting cookies.
// If a value is provided, it sets a secure-by-default, same-site-only cookie that expires in 30 days.
// Defaults are optimized for security: HttpOnly, Strict SameSite, and Domain-locked.
func (ctx *Ctx) Cookie(name string, value ...string) string {
	if len(value) != 0 {
		// Set a new cookie with secure defaults
		http.SetCookie(ctx.w, &http.Cookie{
			Name:     goutil.Clean(name),
			Value:    goutil.Clean(value[0]),
			Path:     "/",
			HttpOnly: true,
			Secure:   ctx.isSecure(),
			SameSite: http.SameSiteStrictMode,
			Domain:   ctx.Host,
			Expires:  time.Now().AddDate(0, 0, 30),
		})
		return value[0]
	}

	// Get a cookie from the request
	cookie, err := ctx.r.Cookie(goutil.Clean(name))
	if err != nil {
		return ""
	}
	return cookie.Value
}

// GetCookie retrieves the full native http.Cookie object from the request.
// Use this when you need to inspect cookie metadata beyond just the value, 
// such as Expiry or Domain attributes.
func (ctx *Ctx) GetCookie(name string) (*http.Cookie, error) {
	return ctx.r.Cookie(goutil.Clean(name))
}

// SetCookie adds a Set-Cookie header to the response
func (ctx *Ctx) SetCookie(cookie *http.Cookie) {
	http.SetCookie(ctx.w, cookie)
}

// DelCookie expires a cookie by name, effectively deleting it from the browser.
// It mirrors the secure defaults of the Cookie() method to ensure the browser 
// correctly identifies and overwrites the intended cookie.
func (ctx *Ctx) DelCookie(name string) {
	http.SetCookie(ctx.w, &http.Cookie{
		Name:     goutil.Clean(name),
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   ctx.isSecure(),
		Domain:   ctx.Host,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
}

// Locals stores or retrieves request-scoped variables
func (ctx *Ctx) Locals(key string, value ...any) any {
	if len(value) != 0 {
		ctx.locals[key] = value[0]
		return value[0]
	}
	return ctx.locals[key]
}

// Request returns the underlying *http.Request
func (ctx *Ctx) Request() *http.Request {
	return ctx.r
}

// Response returns the underlying http.ResponseWriter
func (ctx *Ctx) Response() http.ResponseWriter {
	return ctx.w
}
