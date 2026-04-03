package compiler

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tkdeng/goutil"
	"github.com/tkdeng/nexusweb/compress"
	"github.com/tkdeng/nexusweb/plugins"
)

type SegHTML struct {
	t    byte
	name string
	args map[string]string
	body []byte
	seg  []SegHTML
}

var PageBuf *goutil.SyncMap[string, []SegHTML] = goutil.NewMap[string, []SegHTML]()

func Render(path string, vars map[string]string, isWidget bool) ([]byte, error) {
	if seg, ok := PageBuf.Get(path); ok {
		buf := renderSegHTML(seg, vars)

		if isWidget {
			lPath := filepath.Join(filepath.Dir(path), "/#layout")

			seg, ok := PageBuf.Get(lPath)
			for !ok {
				cPath := lPath
				lPath = filepath.Join(filepath.Dir(filepath.Dir(lPath)), "/#layout")

				if cPath == lPath {
					break
				}

				seg, ok = PageBuf.Get(lPath)
			}

			if ok {
				lBuf := renderSegHTML(seg, vars)
				buf = bytes.ReplaceAll(lBuf, []byte("{@body}"), buf)
			}
		}

		return buf, nil
	}

	return nil, os.ErrNotExist
}

/* func Render(buf *[]byte, root string, path string, vars map[string]string, isWidget bool) error {
	if isWidget {
		*buf = regex.Comp(`{@([\w_\-]+)}`).RepFunc(*buf, func(b func(int) []byte) []byte {
			ePath, err := goutil.JoinPath(filepath.Dir(path), string(b(1)))
			if err != nil || !strings.HasPrefix(ePath, root+"/dist") {
				return nil
			}

			if !strings.HasSuffix(ePath, ".html") {
				ePath += ".html"
			}

			eBuf, err := os.ReadFile(ePath)

			for err != nil {
				// ePath = regex.Comp(`\/[^\/]+\/([^\/]+)$`).RepStr(ePath, "/$1")
				ePath = filepath.Join(filepath.Dir(filepath.Dir(ePath)), filepath.Base(ePath))

				if !strings.HasPrefix(ePath, root+"/dist") {
					return nil
				}

				eBuf, err = os.ReadFile(ePath)
			}

			return eBuf
		})
	}

	*buf = regex.Comp(`(?s){([?!:#=]?)\s*\$?([\w_\-]+)\s*([^\r\n]*?)\s*(?:{%(.*?)%}|)}`).RepFunc(*buf, func(b func(int) []byte) []byte {
		var t byte
		if len(b(1)) != 0 {
			t = b(1)[0]
		}
		name := b(2)
		atts := b(3)
		cont := b(4)

		switch t {
		case '?', '!':
			val, ok := vars[string(name)]
			if !((t == '?' && (!ok || val == "")) || (t == '!' && (ok && val != ""))) {
				if len(cont) != 0 {
					if err := RenderOld(&cont, root, path, vars, false); err == nil {
						return cont
					}
					return []byte{}
				}
			}
		case ':':
			if plugin, ok := plugins.Get(string(name)); ok {
				args := map[string]string{}
				ind := 0
				regex.Comp(`([\w_\-]+)(?:\s*(=)\s*"([^"]*)"|'([^"]*)'|([\w_\-]+)|)`).RepFunc(atts, func(b func(int) []byte) []byte {
					if len(b(2)) == 0 {
						args[strconv.Itoa(ind)] = string(goutil.Clean(b(1)))
						ind++
					} else {
						args[string(goutil.Clean(b(1)))] = string(goutil.Clean(b(3)))
					}
					return nil
				})

				if len(cont) != 0 {
					if err := RenderOld(&cont, root, path, vars, false); err != nil {
						cont = []byte{}
					}
				}

				out, err := plugin.Run(args, bytes.TrimSpace(cont), false)
				if err != nil {
					PrintMsg("warn", "Warning: Plugin Error!")
					fmt.Println("  plugin:", string(b(1)))
					fmt.Println(err)
					return nil
				}

				return out
			}
		case '#':
			if val, ok := vars[string(name)]; ok && val != "" {
				return []byte(val)
			} else if len(atts) != 0 && atts[0] == '|' {
				return atts[1:]
			}
		case '=':
			if val, ok := vars[string(name)]; ok && val != "" {
				return goutil.HTML.EscapeArgs([]byte(val))
			} else if len(atts) != 0 && atts[0] == '|' {
				return atts[1:]
			}
		default:
			if len(atts) != 0 && atts[0] == '=' {
				if val, ok := vars[string(bytes.Trim(atts[1:], "\"' \t"))]; ok && val != "" {
					return regex.JoinBytes(name, `="`, goutil.HTML.EscapeArgs([]byte(val), '"'), '"')
				}
				return nil
			}

			if val, ok := vars[string(name)]; ok && val != "" {
				return goutil.HTML.Escape([]byte(val))
			} else if len(atts) != 0 && atts[0] == '|' {
				return atts[1:]
			}
		}

		return nil
	})

	return nil
} */

func renderSegHTML(seg []SegHTML, vars map[string]string) []byte {
	buf := []byte{}

	for _, s := range seg {
		switch s.t {
		case '?', '!':
			val, ok := vars[string(s.name)]
			if !((s.t == '?' && (!ok || val == "")) || (s.t == '!' && (ok && val != ""))) {
				buf = append(buf, renderSegHTML(s.seg, vars)...)
			}
		case ':':
			if plugin, ok := plugins.Get(s.name); ok {
				out, err := plugin.Run(s.args, bytes.TrimSpace(renderSegHTML(s.seg, vars)), false)
				if err != nil {
					PrintMsg("warn", "Warning: Plugin Error!")
					fmt.Println("  plugin:", s.name)
					fmt.Println(err)
				} else {
					buf = append(buf, out...)
				}
			}
		case '#', '=', ' ':
			if val, ok := vars[s.name]; ok && val != "" {
				if s.t == '=' {
					val = string(goutil.HTML.EscapeArgs([]byte(val)))
				} else if s.t == ' ' {
					val = string(goutil.HTML.Escape([]byte(val)))
				}
				buf = append(buf, []byte(val)...)
			} else {
				buf = append(buf, s.body...)
			}
		default:
			if dec, err := compress.UnZip(s.body); err == nil {
				buf = append(buf, dec...)
			} else {
				buf = append(buf, s.body...)
			}
		}
	}

	return buf
}
