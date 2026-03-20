package compiler

import (
	"bytes"
	"os"
	"strings"

	"github.com/tkdeng/goutil"
	"github.com/tkdeng/regex"
)

func Render(buf *[]byte, root string, path string, vars map[string]string) error {
	*buf = regex.Comp(`(?s){([?!])([\w_\-]+)\s*{(.*?)}}`).RepFunc(*buf, func(b func(int) []byte) []byte {
		_, ok := vars[string(b(2))]

		if (b(1)[0] == '?' && !ok) || (b(1)[0] == '!' && ok) {
			return []byte{}
		}

		return b(3)
	})

	*buf = regex.Comp(`{@([\w_\-]+)}`).RepFunc(*buf, func(b func(int) []byte) []byte {
		ePath, err := goutil.JoinPath(string(regex.Comp(`\/[^\/]+$`).Rep([]byte(path), []byte{})), string(b(1)))
		if err != nil || !strings.HasPrefix(ePath, root+"/dist") {
			return []byte{}
		}

		if !strings.HasSuffix(ePath, ".html") {
			ePath += ".html"
		}

		eBuf, err := os.ReadFile(ePath)

		for err != nil {
			ePath = string(regex.Comp(`\/[^\/]+\/([^\/]+)$`).Rep([]byte(ePath), []byte("/$1")))

			if !strings.HasPrefix(ePath, root+"/dist") {
				return []byte{}
			}

			eBuf, err = os.ReadFile(ePath)
		}

		return eBuf
	})

	*buf = regex.Comp(`{(#|(?:[\w_\-]+|)=|)["']?([\w_\-]+)(\|.*?|)["']?}`).RepFunc(*buf, func(b func(int) []byte) []byte {
		if len(b(1)) == 0 {
			if val, ok := vars[string(b(2))]; ok {
				return goutil.HTML.Escape([]byte(val))
			} else {
				return bytes.TrimPrefix(b(3), []byte{'|'})
			}
		} else if b(1)[0] == '#' {
			if val, ok := vars[string(b(2))]; ok {
				return []byte(val)
			} else {
				return bytes.TrimPrefix(b(3), []byte{'|'})
			}
		}

		key := bytes.TrimSuffix(b(1), []byte{'='})

		if val, ok := vars[string(b(2))]; ok {
			if len(key) == 0 {
				return goutil.HTML.EscapeArgs([]byte(val))
			}
			return regex.JoinBytes(key, `="`, goutil.HTML.EscapeArgs([]byte(val), '"'), '"')
		} else if len(b(3)) != 0 {
			if len(key) == 0 {
				return bytes.TrimPrefix(b(3), []byte{'|'})
			}
			return regex.JoinBytes(key, `="`, bytes.TrimPrefix(b(3), []byte{'|'}), '"')
		}

		return []byte{}
	})

	return nil
}

func Markdown(buf *[]byte) error {
	// fmt.Println(string(*buf))

	//todo: compile markdown

	return nil
}

func Compile(path string, vars map[string]string) error {
	//todo: pre compile pages to dist
	// @pages should remain dynamic
	// #layout pages should be copied over
	// also, embed const vars when possible, otherwise keep placeholder for future vars

	//todo: add separate function for precompiled vars (and runtime var methods)
	// similar to what webx module does with {lorem}
	// may make it easier for admins to expand on these
	// may also add future extensions using {:plugin key=value} method
	// and use {:plugin key=value { content }} for simplicity of multiline content

	return nil
}
