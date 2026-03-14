package nxweb

import (
	"net/http"
	"strings"
	"sync"

	"github.com/tkdeng/goutil"
)

type Router struct {
	app  *App
	path string

	routes *goutil.SyncMap[string, *Router]

	mu sync.RWMutex
	cb []func(c *Ctx) error

	handler func(w http.ResponseWriter, r *http.Request)
}

func (router *Router) newRouter(path string) *Router {
	var childRouter *Router
	router.mu.Lock()
	if r, ok := router.routes.Get(path); ok {
		childRouter = r
	} else {
		childRouter = &Router{
			app:    router.app,
			path:   path,
			routes: goutil.NewMap[string, *Router](),
		}

		childRouter.handler = func(w http.ResponseWriter, r *http.Request) {

		}

		router.routes.Set(path, childRouter)
	}
	router.mu.Unlock()

	return childRouter
}

func (router *Router) Use(path string, cb func(c *Ctx) error) *Router {
	paths := strings.Split(path, ":")

	//todo: get dynamic :var1, :var2? values from paths

	childRouter := router.newRouter(paths[0])

	return childRouter
}
