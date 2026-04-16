package nxweb

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	mpath "path"
	"strings"
	"sync"

	"github.com/tkdeng/goutil"
)

// Router handles path-based request multiplexing and middleware registration.
type Router struct {
	mux    *http.ServeMux // Standard library multiplexer
	Config Config         // Global framework settings

	path string // Base prefix for this specific router instance

	mu sync.RWMutex
	cb *goutil.SyncMap[string, *routeCB] // Thread-safe storage for route callbacks

	vars map[string]string // Persistent variables passed to the render engine
}

type routeCB struct {
	cb *[]func(c *Ctx) error
	mu sync.RWMutex
}

// NewRouter creates a sub-router mounted at the specified path.
//
// It provides prefix-based isolation (e.g., /api) and inherits parent
// variables, which are automatically injected into the Render engine.
//
// @handler will run before any other routes are called.
// This can be useful for initializing data, or adding a firewall to your router.
func (router *Router) NewRouter(path string, handler func(c *Ctx) error, vars ...Map) *Router {
	path = mpath.Clean("/" + strings.Trim(path, "/"))

	childRouter := &Router{
		mux:    router.mux,
		Config: router.Config,
		path:   path,
		cb:     goutil.NewMap[string, *routeCB](),
		vars:   router.vars,
	}

	if len(vars) > 0 {
		for k, v := range vars[0] {
			childRouter.vars[k] = goutil.Clean(v)
		}
	}

	router.mux.HandleFunc(path+"/", func(w http.ResponseWriter, r *http.Request) {
		// get request context (also verifies headers)
		ctx, err := childRouter.newCtx(w, r)
		if err != nil {
			if errors.Is(err, ctxInitError) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Bad Request!"))
			}
			return
		}

		// run app handler
		if router.Config.Handler != nil {
			ctx.next = false
			if err := router.Config.Handler(&ctx); err != nil {
				fmt.Println(err)

				// catch 404 error
				if err = ctx.Error(ctx.Path, 404, "Page Not Found!"); err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte("Internal Server Error!"))
				}
				return
			} else if !ctx.next {
				ctx.rendered = true
				return
			}
		}

		// run router handler
		if handler != nil {
			ctx.next = false
			if err := handler(&ctx); err != nil {
				fmt.Println(err)

				// catch 404 error
				if err = ctx.Error(ctx.Path, 404, "Page Not Found!"); err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte("Internal Server Error!"))
				}
				return
			} else if !ctx.next {
				ctx.rendered = true
				return
			}
		}

		// handle route callbacks
		cPath := strings.TrimPrefix(ctx.Path, path)
		rcb, ok := childRouter.cb.Get(cPath)
		ctx.next = true
		if ok {
			rcb.run(&ctx)
		}

		for ctx.next {
			cPath = mpath.Dir(cPath)
			cPath = mpath.Clean("/" + cPath)
			if cPath == "/" {
				break
			}
			if rcb, ok = childRouter.cb.Get(cPath); ok {
				rcb.run(&ctx)
			}
		}

		if ctx.rendered {
			return
		}

		if ctx.status != 0 && ctx.status != 200 {
			ctx.w.WriteHeader(ctx.status)
			return
		}

		// catch 404 error
		if err = ctx.Error(ctx.Path, 404, "Page Not Found!"); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error!"))
		}
	})

	return childRouter
}

// Use registers a callback for a specific path pattern.
//
// Supports static paths, dynamic segments (/:id), and optional parameters (/:id?).
// It automatically prepares the request body and form data based on the
// Content-Type header before reaching child handlers.
func (router *Router) Use(path string, cb func(c *Ctx) error) {
	paths := strings.Split(path, ":")
	for i, path := range paths {
		paths[i] = strings.TrimSuffix(path, "/")
	}

	router.mu.Lock()
	rcb, ok := router.cb.Get(paths[0])
	if !ok {
		rcb = &routeCB{
			cb: &[]func(c *Ctx) error{},
		}
		router.cb.Set(paths[0], rcb)
	}
	router.mu.Unlock()

	if router.path != "/" && router.path != "" {
		paths[0] = mpath.Join(router.path, paths[0])
	}

	rcb.mu.Lock()
	*rcb.cb = append(*rcb.cb, func(c *Ctx) error {
		if len(paths) == 1 && c.Path != paths[0] {
			return c.Next()
		}

		if len(paths) != 1 {
			u := strings.Trim(strings.Replace(c.Path, paths[0], "", 1), "/")
			if u == "" {
				optPath := false
				for i := 1; i < len(paths); i++ {
					if strings.HasPrefix(paths[i], "?") || strings.HasSuffix(paths[i], "?") {
						optPath = true
						continue
					}
					optPath = false
					break
				}

				if !optPath {
					return c.Next()
				}
			}

			uri := strings.Split(u, "/")

			if len(uri) >= len(paths) {
				return c.Next()
			}

			pathVars := map[string]string{}
			for i := 1; i < len(paths); i++ {
				if i-1 >= len(uri) {
					if strings.HasPrefix(paths[i], "?") || strings.HasSuffix(paths[i], "?") {
						break
					}
					return c.Next()
				}
				pathVars[strings.Trim(paths[i], "?")] = uri[i-1]
			}

			c.Params = pathVars
		} else {
			c.Params = map[string]string{}
		}

		if c.Method == "GET" {
			c.query = c.r.URL.Query()
		} else if c.Method == "POST" {
			if strings.Contains(c.Type, "application/json") {
				if c.body == nil {
					c.body = make(map[string]any)
					json.NewDecoder(c.r.Body).Decode(&c.body)
				}
			} else if c.form == nil {
				if err := c.r.ParseForm(); err == nil {
					c.form = c.r.PostForm
				}
			}
		}

		return cb(c)
	})
	rcb.mu.Unlock()
}

// Query returns the value of a query parameter and a boolean indicating its existence.
//
// It lazily initializes the query map from the request URL.
func (router *Router) Get(path string, cb func(c *Ctx) error) {
	router.Use(path, func(c *Ctx) error {
		if c.Method == "GET" {
			return cb(c)
		}
		return c.Next()
	})
}

// SetQuery modifies or deletes a query parameter in the current context.
//
// If a value is provided, it updates the parameter; if no value is provided,
// the key is deleted from the query map.
func (router *Router) Post(path string, cb func(c *Ctx) error) {
	router.Use(path, func(c *Ctx) error {
		if c.Method == "POST" {
			return cb(c)
		}
		return c.Next()
	})
}

func (rcb *routeCB) run(ctx *Ctx) {
	rcb.mu.RLock()
	ctx.next = true

	for _, cb := range *rcb.cb {
		if !ctx.next {
			break
		}
		ctx.next = false

		if err := cb(ctx); err != nil {
			fmt.Println(err)
			break
		} else if !ctx.next {
			ctx.rendered = true
		}
	}
	rcb.mu.RUnlock()
}
