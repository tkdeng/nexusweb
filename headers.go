package nxweb

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/tkdeng/goutil"
)

func (ctx *Ctx) errorDeny(reason string) error {
	msg := "Access denied. Suspicious request pattern."
	http.Error(ctx.w, msg, http.StatusForbidden)
	return fmt.Errorf("%s: %s", msg, reason)
}

func (ctx *Ctx) verifyOrigin() error {
	// Validate Host against Origins
	if len(ctx.router.Config.Origins) != 0 && !goutil.Contains(ctx.router.Config.Origins, ctx.Host) {
		return ctx.errorDeny("Origin Not Allowed: " + ctx.Host)
	}

	// Validate RemoteIP against Proxies
	if len(ctx.router.Config.Proxies) > 0 {
		if !goutil.Contains(ctx.router.Config.Proxies, ctx.RemoteIP) {
			return ctx.errorDeny("IP Proxy Not Allowed: " + ctx.RemoteIP)
		}
	}

	return nil
}

func (ctx *Ctx) verifyHeaders() error {
	ua := ctx.Header("User-Agent")
	isBot := strings.Contains(strings.ToLower(ua), "bot") || strings.Contains(strings.ToLower(ua), "spider")

	// 1. User-Agent: Keep the length check, but maybe lower it to 10.
	if len(ua) < 10 {
		return ctx.errorDeny("Invalid User-Agent")
	}

	// 2. Accept: Most bots send */* which contains the "/"
	accept := ctx.Header("Accept")
	if accept == "" || !strings.Contains(accept, "/") {
		return ctx.errorDeny("Invalid Accept header")
	}

	// 3. Encoding: Only enforce if it's NOT a bot, or if you're strictly optimizing bandwidth.
	encoding := ctx.Header("Accept-Encoding")
	if encoding == "" && !isBot {
		// We might want to allow empty encodings for simple crawlers
		// but your original code was strict:
		return ctx.errorDeny("Missing Encoding support")
	}

	return nil
}

func (ctx *Ctx) redirectSSL() error {
	if ctx.router.Config.PortSSL == 0 || ctx.r.TLS != nil || ctx.Header("X-Forwarded-Proto") == "https" {
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

// IsBot performs a strict header sanity check to identify crawlers or automated tools.
// It checks for bot-like User-Agents and missing headers typical of real browsers.
func (ctx *Ctx) IsBot() bool {
	// 1. Explicit User-Agent Bot Check
	ua := strings.ToLower(ctx.Header("User-Agent"))
	if len(ua) < 25 || strings.Contains(ua, "bot") || strings.Contains(ua, "crawler") || strings.Contains(ua, "spider") {
		return true
	}

	// 2. Accept-Language Check (Browsers almost always send this)
	lang := ctx.Header("Accept-Language")
	if lang == "" || (!strings.Contains(lang, ",") && len(lang) < 3) {
		return true
	}

	// 3. Cache-Control (Real browsers send this on navigation/refresh)
	if ctx.Header("Cache-Control") == "" {
		return true
	}

	// 4. Connection Check (Only for HTTP/1.1)
	if ctx.r.Proto == "HTTP/1.1" {
		if !strings.Contains(strings.ToLower(ctx.Header("Connection")), "keep-alive") {
			return true
		}
	}

	// 5. POST Payload Sanity (Checks for empty or suspiciously large form/JSON posts)
	if ctx.Method == "POST" {
		clStr := ctx.Header("Content-Length")
		if clStr == "" {
			return true
		}
		cl, err := strconv.Atoi(clStr)
		// Restrict to 1KB for typical forms like Login/Register
		if err != nil || cl <= 0 || cl > 1024 {
			return true
		}
	}

	return false
}

// BlockBot stops the request with a 403 if IsBot returns true.
func (ctx *Ctx) BlockBot() bool {
	if ctx.IsBot() {
		msg := "Access denied. Suspicious request pattern."
		http.Error(ctx.w, msg, http.StatusForbidden)
		return true
	}
	return false
}

// isSecure is an internal helper used for setting secure defaults (like cookies).
// It returns true if the environment is configured for SSL or if the request is encrypted.
func (ctx *Ctx) isSecure() bool {
  return ctx.router.Config.PortSSL != 0 || ctx.r.TLS != nil || ctx.Header("X-Forwarded-Proto") == "https"
}

// IsSSL returns true if the current request is encrypted.
// It checks the underlying TLS connection and the X-Forwarded-Proto header.
func (ctx *Ctx) IsSSL() bool {
  return ctx.r.TLS != nil || ctx.Header("X-Forwarded-Proto") == "https"
}
