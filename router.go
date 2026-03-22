package nxweb

import (
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

	//todo: get dynamic :var1, :var2? values from paths

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
		//todo: verify correct url path
		//todo: handle :var1, :var2? values from paths[1:]
		return cb(c)
	})
	rcb.mu.Unlock()
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
