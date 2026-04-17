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
	// ctx.Header("Vary", "User-Agent")

	// Set Secure Headers
	ctx.Header("X-Content-Type-Options", "nosniff")
	ctx.Header("Strict-Transport-Security", "max-age=63072000")
	ctx.Header("Referrer-Policy", "strict-origin-when-cross-origin")

	// disable browser: camera, microphone, geolocation, interest-cohort
	ctx.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=(), interest-cohort=()")

	ua := ctx.Header("User-Agent")
	isBot := strings.Contains(strings.ToLower(ua), "bot") || strings.Contains(strings.ToLower(ua), "spider")

	// User-Agent: Keep the length check, but maybe lower it to 10.
	if len(ua) < 10 {
		return ctx.errorDeny("Invalid User-Agent")
	}

	// Accept: Most bots send */* which contains the "/"
	if accept := ctx.Header("Accept"); accept == "" || !strings.Contains(accept, "/") {
		return ctx.errorDeny("Invalid Accept header")
	}

	// Encoding: Only enforce if it's NOT a bot, or if you're strictly optimizing bandwidth.
	if ctx.Header("Accept-Encoding") == "" && !isBot {
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
	// Explicit User-Agent Bot Check
	ua := strings.ToLower(ctx.Header("User-Agent"))
	if len(ua) < 25 || strings.Contains(ua, "bot") || strings.Contains(ua, "crawler") || strings.Contains(ua, "spider") {
		return true
	}

	// Sec-CH-UA Check
	if ctx.Header("Sec-CH-UA") == "" && len(ua) < 40 {
		return true
	}

	// Accept Header Check
	accept := ctx.Header("Accept")
	if ctx.Method == "GET" && !strings.Contains(accept, "text/html") {
		return true
	}

	// Accept-Language Check (Browsers almost always send this)
	lang := ctx.Header("Accept-Language")
	if lang == "" || (!strings.Contains(lang, ",") && len(lang) < 3) {
		return true
	}

	// Cache-Control (Real browsers send this on navigation/refresh)
	if ctx.Method != "GET" && ctx.Header("Cache-Control") == "" {
		return true
	}

	// Connection Check (Only for HTTP/1.1)
	if ctx.r.Proto == "HTTP/1.1" {
		if !strings.Contains(strings.ToLower(ctx.Header("Connection")), "keep-alive") {
			return true
		}
	}

	// POST Payload Sanity (Checks for empty or suspiciously large form/JSON posts)
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

		// Check for common automated tool Content-Types
		if ctx.Header("Content-Type") == "" {
			return true
		}
	}

	return false
}

// BotProtect hardens the page against clickjacking and verifies the client is not a bot.
// Returns TRUE if the request is safe to proceed.
// Returns FALSE if the request was blocked and a response was already sent.
func (ctx *Ctx) BotProtect(useErr418 bool) bool {
	// ctx.Header("Vary", "User-Agent, Sec-Fetch-Dest, Sec-Fetch-Site")
	ctx.Header("Vary", "Sec-Fetch-Dest, Sec-Fetch-Site")

	// Prevent Clickjacking
	ctx.Header("Content-Security-Policy", "frame-ancestors 'none';")
	ctx.Header("X-Frame-Options", "DENY")
	ctx.Header("Cross-Origin-Opener-Policy", "same-origin")
	ctx.Header("Cross-Origin-Embedder-Policy", "require-corp")
	ctx.Header("Cross-Origin-Resource-Policy", "same-origin")

	ctx.Header("X-Robots-Tag", "noindex, nofollow")

	// ctx.Header("X-Content-Type-Options", "nosniff")
	// ctx.Header("Strict-Transport-Security", "max-age=63072000")
	// ctx.Header("Referrer-Policy", "strict-origin-when-cross-origin")
	// ctx.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=(), interest-cohort=()")

	status := http.StatusForbidden // Default 403
	msg := "Access denied. Suspicious request pattern."

	if useErr418 {
		status = 418
		msg = "I'm a Teapot"
	}

	// Perform Client Fingerprinting
	if ctx.IsBot() {
		if err := ctx.Error("@error", status, msg); err != nil {
			ctx.Status(status).Write([]byte("<h1>Error " + strconv.Itoa(status) + "</h1><h2>" + msg + "</h2>"))
		}
		return false
	}

	// Check Sec-Fetch-Dest header
	if dest := ctx.Header("Sec-Fetch-Dest"); (ctx.Method != "POST" && dest != "document") || (ctx.Method == "POST" && dest != "document" && dest != "empty") {
		if err := ctx.Error("@error", status, msg); err != nil {
			ctx.Status(status).Write([]byte("<h1>Error " + strconv.Itoa(status) + "</h1><h2>" + msg + "</h2>"))
		}
		return false
	}

	// Cheak Sec-Fetch-Site header
	if site := ctx.Header("Sec-Fetch-Site"); ctx.Method != "GET" && site != "same-origin" && site != "same-site" {
		if err := ctx.Error("@error", status, msg); err != nil {
			ctx.Status(status).Write([]byte("<h1>Error " + strconv.Itoa(status) + "</h1><h2>" + msg + "</h2>"))
		}
		return false
	}

	return true
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
