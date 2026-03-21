package compiler

import (
	"bytes"
	"net/url"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

type rawTextRenderer struct{}

func (r *rawTextRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	// Register our function for Text nodes with high priority
	reg.Register(ast.KindText, r.renderRawText)
}

func (r *rawTextRenderer) renderRawText(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	// Write the literal bytes from source (no escaping)
	n := node.(*ast.Text)
	w.Write(n.Segment.Value(source))
	return ast.WalkContinue, nil
}

type externalAutoLinkRenderer struct {
	LocalDomains []string
}

func (r *externalAutoLinkRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	// Only register for AutoLinks
	reg.Register(ast.KindAutoLink, r.renderAutoLink)
}

func (r *externalAutoLinkRenderer) renderAutoLink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.AutoLink)
	u := string(n.URL(source))

	if entering {
		isExternal := r.isExternal(u)

		_, _ = w.WriteString("<a href=\"")
		_, _ = w.Write(util.EscapeHTML(n.URL(source)))

		if isExternal {
			_, _ = w.WriteString("\" target=\"_blank\" rel=\"noopener noreferrer\"")
		} else {
			_ = w.WriteByte('"')
		}
		_ = w.WriteByte('>')
	} else {
		_, _ = w.WriteString("</a>")
	}
	return ast.WalkContinue, nil
}

func (r *externalAutoLinkRenderer) isExternal(u string) bool {
	parsed, err := url.Parse(u)
	// If it's a relative path or invalid, treat it as internal
	if err != nil || parsed.Host == "" {
		return false
	}

	host := strings.ToLower(parsed.Host)

	for _, domain := range r.LocalDomains {
		domain = strings.ToLower(domain)
		// Check if host is exactly the domain or a subdomain (e.g., .domain.com)
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return false // It matches one of our local domains
		}
	}

	// No matches found, so it's an external link
	return true
}

func Markdown(buf *[]byte, domains []string) {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Linkify,
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
			html.WithHardWraps(),
			html.WithXHTML(),
		),
	)

	md.Renderer().AddOptions(
		renderer.WithNodeRenderers(
			util.Prioritized(&rawTextRenderer{}, 100),
			util.Prioritized(&externalAutoLinkRenderer{LocalDomains: domains}, 100),
		),
	)

	var out bytes.Buffer
	if err := md.Convert(*buf, &out); err != nil {
		return
	}

	*buf = out.Bytes()
}
