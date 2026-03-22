package nxweb

import (
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tkdeng/goutil"
	"github.com/tkdeng/nexusweb/compiler"
	"github.com/tkdeng/regex"
)

type App struct {
	Router
}

type Config struct {
	Title    string
	AppTitle string
	Desc     string
	Icon     string

	AssetsURI string
	PublicURI string

	Origins []string
	Proxies []string

	Vars Map

	Port    uint16
	PortSSL uint16

	DebugMode bool

	Root string

	Domains []string
}

type Map map[string]string

// New creates a new webserver
func New(root string, config ...Config) (*App, error) {
	var err error
	root, err = filepath.Abs(root)
	if err != nil {
		return &App{}, err
	}

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
	if config[0].AssetsURI == "" {
		config[0].AssetsURI = "/assets"
	}

	compVars := map[string]string{
		"title":    goutil.Clean(config[0].Title),
		"apptitle": goutil.Clean(config[0].AppTitle),
		"desc":     goutil.Clean(config[0].Desc),
		"icon":     goutil.Clean(config[0].Icon),
		"assets":   goutil.Clean(config[0].AssetsURI),
		"public":   goutil.Clean(config[0].PublicURI),
		"debug":    goutil.ToType[string](config[0].DebugMode),
	}

	// maps.Copy(compVars, config[0].Vars)
	for k, v := range config[0].Vars {
		compVars[k] = goutil.Clean(v)
	}

	err = compiler.Compile(root, compVars, config[0].Domains, config[0].DebugMode)
	if err != nil {
		return &App{}, err
	}

	os.MkdirAll(root+"/assets", 0755)

	if config[0].PublicURI != "" {
		os.MkdirAll(root+"/public", 0755)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		// w.WriteHeader(http.StatusOK)
		w.Write([]byte(`pong!`))
	})

	app := &App{
		Router{
			mux:    mux,
			Config: config[0],
			path:   "",
			routes: goutil.NewMap[string, *Router](),
			cb:     goutil.NewMap[string, *routeCB](),
		},
	}

	if config[0].PublicURI != "" && config[0].PublicURI != "/" {
		uri := config[0].PublicURI
		if !strings.HasPrefix(uri, "/") {
			uri = "/" + uri
		}
		if !strings.HasSuffix(uri, "/") {
			uri = uri + "/"
		}

		mux.Handle(uri, http.StripPrefix(uri, http.FileServer(http.Dir(root+"/public"))))
	}

	app.handler = func(w http.ResponseWriter, r *http.Request) {
		// get request context (also verifies headers)
		ctx, err := app.newCtx(w, r)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Bad Request!"))
			return
		}

		//todo: use `/.well-known/appspecific/com.chrome.devtools.json` and others to detect dev tools and add caution to non admin user IPs
		// may also do some research later on if I can extend this in any way (to make dev tools easier for developers or something)
		// also make this feature optional, or add optional plugin hooks to api

		// fmt.Println("-----")
		// fmt.Println(ctx.Path)

		//todo: add static assets handler
		// use config[0].AssetsURI and config[0].PublicURI
		// may also auto compile go wasm
		// may auto minify assets, and update compiler code to use .min files
		// or might let serveses like cloudflare handle .min files

		// handle static assets
		if strings.HasPrefix(ctx.Path, config[0].AssetsURI) {
			if path, err := goutil.JoinPath(root, "assets", strings.Replace(ctx.Path, config[0].AssetsURI, "", 1)); err == nil {
				if stat, err := os.Stat(path); err == nil && !stat.IsDir() {
					http.ServeFile(w, r, path)
					return
				}
			}
		}

		// handle route callbacks
		if cPath := ctx.Path; cPath != "/" {
			rcb, ok := app.cb.Get(cPath)
			ctx.next = true
			if ok {
				rcb.run(&ctx)
			}

			for ctx.next {
				cPath = filepath.Dir(cPath)
				if cPath == "/" {
					break
				}
				if rcb, ok = app.cb.Get(cPath); ok {
					rcb.run(&ctx)
				}
			}

			if ctx.rendered {
				return
			}
		}

		// handle static pages
		if ctx.Path == "/" || ctx.Path == "" || regex.Comp(`\/[\w_\-]+\/?$`).MatchStr(ctx.Path) {
			if err = ctx.Render(ctx.Path); err != nil {
				if err = ctx.Error(ctx.Path, 404, "Page Not Found!"); err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte("Internal Server Error!"))
				}
			}
			return
		}

		// catch 404 error
		if err = ctx.Error(ctx.Path, 404, "Page Not Found!"); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error!"))
		}
	}

	mux.HandleFunc("/", app.handler)

	return app, nil
}

// Listen for http requests
//
// default port :8080
//
// note: ports can also be set in the config when creating a new server,
// and can optionally be overwritten here.
//
//	@port (optional):
//	- 1: HTTP Port
//	- 2: SSL Port
func (app *App) Listen(port ...uint16) error {
	var portHTTP string
	if len(port) != 0 && port[0] != 0 {
		portHTTP = ":" + strconv.FormatUint(uint64(port[0]), 10)
		app.Config.Port = port[0]
	} else {
		portHTTP = ":" + strconv.FormatUint(uint64(app.Config.Port), 10)
	}

	if app.Config.PortSSL == 0 && len(port) < 2 {
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

	var portSSL string
	if len(port) >= 2 && port[1] != 0 {
		portSSL = ":" + strconv.FormatUint(uint64(port[1]), 10)
		app.Config.PortSSL = port[1]
	} else {
		portSSL = ":" + strconv.FormatUint(uint64(app.Config.PortSSL), 10)
	}

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
			// w.WriteHeader(http.StatusOK)
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
