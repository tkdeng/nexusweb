package nxweb

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/tkdeng/goutil"
)

type Router struct {
	mux    *http.ServeMux
	Config Config

	// app  *App
	path string

	routes *goutil.SyncMap[string, *Router]

	mu sync.RWMutex
	// cb []func(c *Ctx) error
	cb *goutil.SyncMap[string, *routeCB]

	handler func(w http.ResponseWriter, r *http.Request)
}

type routeCB struct {
	cb *[]func(c *Ctx) error
	mu sync.RWMutex
}

func (router *Router) newRouter(path string) *Router {
	//todo: may rebuild method to mirror similar to neweb default router
	// note: router.app.mux.HandleFunc will not run root handler,
	// so things may need to be redefined.
	// also may need to check if assets need to be rerequested,
	// if config.AssetsURI == "/"

	var childRouter *Router
	router.mu.Lock()
	if r, ok := router.routes.Get(path); ok {
		childRouter = r
	} else {
		childRouter = &Router{
			// app:    router.app,
			path:   path,
			routes: goutil.NewMap[string, *Router](),
			// cb:     []func(c *Ctx) error{},
			cb: goutil.NewMap[string, *routeCB](),
		}

		childRouter.handler = func(w http.ResponseWriter, r *http.Request) {
			ctx, err := router.newCtx(w, r)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Bad Request!"))
				return
			}

			//todo: ensure assets are handled properly

			/* ctx.next = true
			for _, cb := range childRouter.cb {
				if !ctx.next {
					break
				}
				ctx.next = false

				if err := cb(&ctx); err != nil {
					break
				}
			} */

			if err := ctx.Error(ctx.Path, 404, "Page Not Found!"); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Internal Server Error!"))
			}
		}

		router.routes.Set(path, childRouter)
		router.mux.HandleFunc(path, childRouter.handler)
	}
	router.mu.Unlock()

	return childRouter
}

//todo: may use a separate method for routers instead (to simplify)

func (router *Router) Router(path string, vars Map) *Router {
	paths := strings.Split(path, ":")

	//todo: get dynamic :var1, :var2? values from paths

	fmt.Println(paths)

	childRouter := router.newRouter(paths[0])

	// childRouter.cb = append(childRouter.cb, cb)

	return childRouter
}

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

func (router *Router) Get(path string, cb func(c *Ctx) error) {
	router.Use(path, func(c *Ctx) error {
		if c.Method == "GET" {
			return cb(c)
		}
		return c.Next()
	})
}

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
