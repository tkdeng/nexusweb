package nxweb

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tkdeng/goutil"
	"github.com/tkdeng/nexusweb/compiler"
)

type App struct {
	mux    *http.ServeMux
	Config Config

	router *Router
}

type Config struct {
	Title    string
	AppTitle string
	Desc     string
	Icon     string

	PublicURI string

	Origins []string
	Proxies []string

	Vars Map

	Port    uint16
	PortSSL uint16

	DebugMode bool

	Root string
}

type Map map[string]string

func New(root string, config ...Config) (*App, error) {
	os.MkdirAll(root, 0755)

	if len(config) == 0 {
		config = append(config, Config{})
	}

	config[0].Root = root
	if config[0].Title == "" {
		config[0].Title = "Web Server"
	}
	if config[0].AppTitle == "" {
		config[0].AppTitle = "WebServer"
	}
	if config[0].Desc == "" {
		config[0].Desc = "A Web Server."
	}
	if config[0].Port == 0 {
		config[0].Port = 8080
	}

	var portVar string
	if config[0].PortSSL == 0 {
		portVar = strconv.FormatUint(uint64(config[0].Port), 10)
	} else {
		portVar = strconv.FormatUint(uint64(config[0].PortSSL), 10)
	}

	compVars := map[string]string{
		"root":     goutil.Clean(root),
		"title":    goutil.Clean(config[0].Title),
		"apptitle": goutil.Clean(config[0].AppTitle),
		"desc":     goutil.Clean(config[0].Desc),

		"public-uri": goutil.Clean(config[0].PublicURI),
		"debug-mode": goutil.ToType[string](config[0].DebugMode),

		"port":      portVar,
		"port-http": strconv.FormatUint(uint64(config[0].Port), 10),
		"port-ssl":  strconv.FormatUint(uint64(config[0].PortSSL), 10),
	}

	// maps.Copy(compVars, config[0].Vars)
	for k, v := range config[0].Vars {
		compVars[k] = goutil.Clean(v)
	}

	err := compiler.Compile(config[0].Root, compVars)
	if err != nil {
		return &App{}, err
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`pong!`))
	})

	//todo: trigger compile and catch 404
	// add a custom app.Use() method to add custom routes

	app := &App{
		mux:    mux,
		Config: config[0],
		router: &Router{
			path:   "/",
			routes: goutil.NewMap[string, *Router](),
			cb:     []func(c *Ctx) error{},
		},
	}

	app.router.app = app

	app.router.handler = func(w http.ResponseWriter, r *http.Request) {
		ctx, err := app.router.newCtx(w, r)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Bad Request!"))
			return
		}

		if strings.HasPrefix(ctx.Path, "/.well-known/") || strings.HasPrefix(ctx.Path, "/favicon.ico") {
			return
		}

		fmt.Println("-----")
		fmt.Println(ctx.Path)

		//todo: add static assets handler

		if err = ctx.Render(ctx.Path); err != nil {
			if err = ctx.Error(ctx.Path, 404, "Page Not Found!"); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Internal Server Error!"))
			}
		}
	}

	mux.HandleFunc("/", app.router.handler)

	return app, nil
}

func (app *App) Listen() error {
	//todo: may allow listen method to optionally override port
	// make sure config can be updated with it
	// also make sure static config vars are modified (or just make port dynamic)

	portHTTP := ":" + strconv.FormatUint(uint64(app.Config.Port), 10)

	if app.Config.PortSSL == 0 {
		server := &http.Server{
			Addr:              portHTTP,
			Handler:           app.mux,
			ReadHeaderTimeout: 3 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      15 * time.Second,
			IdleTimeout:       60 * time.Second,
		}

		PrintMsg("confirm", "Server starting on \x1b[1;35m"+portHTTP, 50, true)
		if err := server.ListenAndServe(); err != nil {
			PrintMsg("error", "error: Failed to start server \x1b[1;35m"+portHTTP, 50, true)
			return err
		}

		return nil
	}

	portSSL := ":" + strconv.FormatUint(uint64(app.Config.PortSSL), 10)

	server := &http.Server{
		Addr:              portSSL,
		Handler:           app.mux,
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,

		TLSConfig: &tls.Config{
			MinVersion:               tls.VersionTLS12,
			PreferServerCipherSuites: true,
			CurvePreferences:         []tls.CurveID{tls.CurveP256, tls.X25519},
		},
	}

	crtFile, keyFile, err := app.autoTLS(app.Config.Root + "/db/ssl/auto_ssl")
	if err != nil {
		return err
	}

	go func() {
		mux := http.NewServeMux()

		mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`pong! (insecure)`))
		})

		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			host, _, err := net.SplitHostPort(r.Host)
			if err != nil {
				host = r.Host
			}

			target := "https://" + host + portSSL + r.URL.Path
			if len(r.URL.RawQuery) > 0 {
				target += "?" + r.URL.RawQuery
			}

			http.Redirect(w, r, target, http.StatusMovedPermanently)
		})

		httpServer := &http.Server{
			Addr:              portHTTP,
			Handler:           mux,
			ReadHeaderTimeout: 2 * time.Second,
			ReadTimeout:       3 * time.Second,
			WriteTimeout:      3 * time.Second,
			IdleTimeout:       30 * time.Second,
		}

		PrintMsg("confirm", "HTTP Server starting on \x1b[1;35m"+portHTTP, 50, true)
		if err := httpServer.ListenAndServe(); err != nil {
			PrintMsg("error", "error: Failed to start HTTP server \x1b[1;35m"+portHTTP, 50, true)
		}
	}()

	PrintMsg("confirm", "Secure Server starting on \x1b[1;35m"+portSSL, 50, true)
	if err = server.ListenAndServeTLS(crtFile, keyFile); err != nil {
		PrintMsg("error", "error: Failed to start SSL server \x1b[1;35m"+portSSL, 50, true)
		return err
	}

	return nil
}
