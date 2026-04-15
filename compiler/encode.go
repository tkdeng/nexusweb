package compiler

import (
	"strconv"
	"strings"

	"github.com/tkdeng/nexusweb/compress"
	"github.com/tkdeng/regex"
)

func encodeVars(buf *[]byte) {
	ind := 0
	iMax := 0
	*buf = regex.Comp(`(?s)(\\?[{}]|"(?:\\[\\"]|.)*?"|'(?:\\[\\']|.)*?')`).RepFunc(*buf, func(b func(int) []byte) []byte {
		if s := string(b(0)); s == "{" || s == "}" {
			if s == "{" {
				r := regex.JoinBytes('{', ind, '%')
				ind++
				if ind > iMax {
					iMax = ind
				}
				return r
			} else {
				ind--
				return regex.JoinBytes('%', ind, '}')
			}
		}
		return b(0)
	})

	for i := iMax - 1; i >= 0; i-- {
		n := strconv.Itoa(i)
		n1 := strconv.Itoa(i + 1)
		*buf = regex.Comp(`(?s){`+n+`\%([?!:#=]?)\s*(\$?)([\w_\-]+)(\s*[^\r\n]*?)\s*(?:{`+n1+`\%(.*?)\%`+n1+`}|)\%`+n+`}`).RepFunc(*buf, func(b func(int) []byte) []byte {
			t := b(1)
			d := b(2)
			name := b(3)
			atts := b(4)
			cont := b(5)

			enc := compress.Zip(regex.JoinBytes(t, d, name, atts), true)

			if len(cont) == 0 {
				return regex.JoinBytes(`{%`, enc, `%}`)
			}

			return regex.JoinBytes(`<param data="`, enc, `">`, cont, `</param>`)
		})
	}

	*buf = regex.Comp(`(?s)(\\?(?:{[0-9]+%|%[0-9]+})|"(?:\\[\\"]|.)*?"|'(?:\\[\\']|.)*?')`).RepFunc(*buf, func(b func(int) []byte) []byte {
		if s := string(b(0)); strings.HasPrefix(s, "{") || strings.HasSuffix(s, "}") {
			if strings.HasPrefix(s, "{") {
				return []byte{'{'}
			} else {
				return []byte{'}'}
			}
		}
		return b(0)
	})

	//! old method below (new method handles nested var content better)
	/* *buf = regex.Comp(`(?s){([?!:#=]?)\s*(\$?)([\w_\-]+)(\s*[^\r\n]*?)\s*(?:{(.*?)}|)}`).RepFunc(*buf, func(b func(int) []byte) []byte {
		t := b(1)
		d := b(2)
		name := b(3)
		atts := b(4)
		cont := b(5)

		enc := compress.Zip(regex.JoinBytes(t, d, name, atts), true)

		if len(cont) == 0 {
			return regex.JoinBytes(`{%`, enc, `%}`)
		}

		return regex.JoinBytes(`<param data="`, enc, `">`, cont, `</param>`)
	}) */
}

func decodeVars(buf *[]byte) {
	*buf = regex.Comp(`{%([^{}%]*)%}`).RepFunc(*buf, func(b func(int) []byte) []byte {
		if dec, err := compress.UnZip(b(1)); err == nil {
			return regex.JoinBytes('{', dec, '}')
		}
		return b(0)
	})

	*buf = regex.Comp(`(?s)<param\s+data=["']?([^"'>]*)["']?\s*/?>(.*?)(</param>)`).RepFunc(*buf, func(b func(int) []byte) []byte {
		if dec, err := compress.UnZip(b(1)); err == nil {
			return regex.JoinBytes('{', dec, `{%`, b(2), `%}`, '}')
		}
		return b(0)
	})
}

func recodeVars(buf []byte) []byte {
	encodeVars(&buf)
	decodeVars(&buf)
	return buf
}
