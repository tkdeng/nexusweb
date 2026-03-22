package nxweb

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tkdeng/goutil"
	"github.com/tkdeng/nexusweb/compiler"
	"github.com/tkdeng/regex"
)

type Ctx struct {
	router *Router
	w      http.ResponseWriter
	r      *http.Request

	Host string
	Port string
	Path string
	IP   string
}

func (router *Router) newCtx(w http.ResponseWriter, r *http.Request) (*Ctx, error) {
	host, port, err := net.SplitHostPort(r.Host)
	if err != nil {
		return nil, err
	}

	// get ip address or remote host
	remoteIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remoteIP = r.RemoteAddr
	}

	if remoteIP == "" {
		return nil, fmt.Errorf("unable to detect remote ip address")
	}

	return &Ctx{
		router: router,
		w:      w,
		r:      r,

		Host: goutil.Clean(host),
		Port: goutil.Clean(port),
		Path: "/" + strings.Trim(goutil.Clean(r.URL.Path), "/"),
		IP:   goutil.Clean(remoteIP),
	}, nil
}

func (ctx *Ctx) getLayout(path string) ([]byte, error) {
	// fmt.Println(path, filepath.Base(path))

	if !regex.Comp(`\/@([\w_\-\.]+)$`).MatchStr(path) {
		return nil, fmt.Errorf("layout not found")
	}

	lPath := regex.Comp(`\/@([\w_\-\.]+)$`).RepLitStr(path, "/#layout.html")

	lFilePath, err := goutil.JoinPath(ctx.router.app.Config.Root, "dist", lPath)
	if err != nil {
		return nil, err
	}

	lbuf, err := os.ReadFile(lFilePath)
	for err != nil && regex.Comp(`\/[\w_\-\.]+(\/#[\w_\-\.]+)$`).MatchStr(lPath) {
		lPath = regex.Comp(`\/[\w_\-\.]+(\/#[\w_\-\.]+)$`).RepStr(lPath, "$1")

		if lFilePath, err = goutil.JoinPath(ctx.router.app.Config.Root, "dist", lPath); err != nil {
			return nil, err
		}

		lbuf, err = os.ReadFile(lFilePath)
	}

	if err != nil {
		return nil, err
	}

	return lbuf, nil
}

func (ctx *Ctx) Render(path string, vars ...Map) error {
	if path == "/" || path == "" {
		path = "index"
	} else if path[0] != '/' {
		path = "/" + strings.TrimSuffix(path, ".html")
	} else {
		path = strings.TrimSuffix(path, ".html")
	}

	filePath, err := goutil.JoinPath(ctx.router.app.Config.Root, "dist", path)
	if err != nil {
		return err
	}

	isWidget := false
	if filename := filepath.Base(filePath); filename[0] == '@' || filename == "index" {
		filePath += ".html"
		if filename[0] == '@' {
			isWidget = true
		}
	} else {
		filePath += "/index.html"
	}

	buf, err := os.ReadFile(filePath)

	if err != nil && strings.HasSuffix(filePath, "/index.html") {
		filePath = strings.TrimSuffix(filePath, "/index.html")
		buf, err = os.ReadFile(filePath)
	}

	if err != nil {
		return err
	}

	if isWidget {
		//todo: optimize performance for ctx.getLayout method
		if lBuf, err := ctx.getLayout(path); err == nil && lBuf != nil {
			// buf = regex.Comp(`{@body}`).Rep(lBuf, buf)
			buf = bytes.ReplaceAll(lBuf, []byte("{@body}"), buf)
		}
	}

	varList := map[string]string{
		"title":    goutil.Clean(ctx.router.app.Config.Title),
		"apptitle": goutil.Clean(ctx.router.app.Config.AppTitle),
		"desc":     goutil.Clean(ctx.router.app.Config.Desc),
		"icon":     goutil.Clean(ctx.router.app.Config.Icon),
		"public":   goutil.Clean(ctx.router.app.Config.PublicURI),
		"debug":    goutil.ToType[string](ctx.router.app.Config.DebugMode),
	}

	for k, v := range ctx.router.app.Config.Vars {
		varList[k] = goutil.Clean(v)
	}

	//todo: set other dynamic vars
	// may also allow routers to store separate additional vars (on creation only)

	for _, m := range vars {
		// maps.Copy(varList, m)
		for k, v := range m {
			varList[k] = goutil.Clean(v)
		}
	}

	if err = compiler.Render(&buf, ctx.router.app.Config.Root, filePath, varList, isWidget); err != nil {
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
		"status": strconv.Itoa(status),
		"msg":    msg,
	})

	if err != nil {
		err = ctx.Render(path+"/@error", Map{
			"status": strconv.Itoa(status),
			"msg":    msg,
		})
	}

	if err != nil {
		err = ctx.Render("@"+strconv.Itoa(status), Map{
			"status": strconv.Itoa(status),
			"msg":    msg,
		})
	}

	if err != nil {
		err = ctx.Render("/@error", Map{
			"status": strconv.Itoa(status),
			"msg":    msg,
		})
	}

	if err != nil {
		ctx.w.WriteHeader(status)
		ctx.w.Write([]byte(msg))
	}

	return nil
}
