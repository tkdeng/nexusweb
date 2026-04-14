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

	"github.com/tkdeng/goutil"
	"github.com/tkdeng/nexusweb/compiler"
)

// Ctx represents the request/response context.
// It provides a unified API for data retrieval, routing parameters, and response control.
type Ctx struct {
	router *Router
	w      http.ResponseWriter
	r      *http.Request

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
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		userIP = strings.Split(ip, ",")[0] // Take the first IP in the chain
	}

	if userIP == "" {
		userIP = remoteIP
	}

	ctx := Ctx{
		router: router,
		w:      w,
		r:      r,

		Host:     goutil.Clean(host),
		RemoteIP: goutil.Clean(remoteIP),
		IP:       goutil.Clean(userIP),

		Path:   "/" + strings.Trim(goutil.Clean(r.URL.Path), "/"),
		Port:   goutil.Clean(port),
		Method: goutil.Clean(r.Method),
		Type:   goutil.Clean(r.Header.Get("Content-Type")),
	}

	//todo: add methods to ensure host and origin are valid
	if err := ctx.verifyOrigin(); err != nil {
		return ctx, err
	}

	if err := ctx.redirectSSL(); err != nil {
		return ctx, err
	}

	return ctx, nil
}

func (ctx *Ctx) verifyOrigin() error {
	// Validate Host against Origins
	if len(ctx.router.Config.Origins) != 0 && !goutil.Contains(ctx.router.Config.Origins, ctx.Host) {
		msg := "Origin Not Allowed: " + ctx.Host
		http.Error(ctx.w, msg, http.StatusForbidden)
		return fmt.Errorf("%s", msg)
	}

	// Validate RemoteIP against Proxies
	if len(ctx.router.Config.Proxies) > 0 {
		if !goutil.Contains(ctx.router.Config.Proxies, ctx.RemoteIP) {
			msg := "IP Proxy Not Allowed: " + ctx.RemoteIP
			http.Error(ctx.w, msg, http.StatusForbidden)
			return fmt.Errorf("%s", msg)
		}
	}

	return nil
}

func (ctx *Ctx) redirectSSL() error {
	if ctx.router.Config.PortSSL == 0 || ctx.r.TLS != nil || ctx.r.Header.Get("X-Forwarded-Proto") == "https" {
		return nil
	}

	hostPort, _ := strconv.Atoi(ctx.Port)
	sslPort := ctx.router.Config.PortSSL
	httpPort := ctx.router.Config.Port

	if uint16(hostPort) != sslPort && hostPort != 443 {
		targetHost := ctx.Host

		if uint16(hostPort) == httpPort || httpPort == 80 {
			targetHost = fmt.Sprintf("%s:%d", ctx.Host, sslPort)
		}

		target := "https://" + targetHost + ctx.r.URL.RequestURI()

		http.Redirect(ctx.w, ctx.r, target, http.StatusMovedPermanently)
		return fmt.Errorf("redirecting to HTTPS")
	}

	return nil
}

func (ctx *Ctx) Next() error {
	ctx.next = true
	return nil
}

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
	ctx.w.Write(buf)
	return nil
}

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
