package compiler

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"strings"

	"github.com/tkdeng/goutil"
	"github.com/tkdeng/regex"
)

//go:embed templates/layout.html
var defLayoutBuf []byte

func Render(buf *[]byte, root string, path string, vars map[string]string) error {
	*buf = regex.Comp(`(?s){([?!])\$?([\w_\-]+)\s*{(.*?)}}`).RepFunc(*buf, func(b func(int) []byte) []byte {
		val, ok := vars[string(b(2))]

		if (b(1)[0] == '?' && (!ok || val == "")) || (b(1)[0] == '!' && (ok && val != "")) {
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

	*buf = regex.Comp(`{(#|(?:[\w_\-]+|)=|)["']?\$?([\w_\-]+)(\|.*?|)["']?}`).RepFunc(*buf, func(b func(int) []byte) []byte {
		if len(b(1)) == 0 {
			if val, ok := vars[string(b(2))]; ok && val != "" {
				return goutil.HTML.Escape([]byte(val))
			} else {
				return bytes.TrimPrefix(b(3), []byte{'|'})
			}
		} else if b(1)[0] == '#' {
			if val, ok := vars[string(b(2))]; ok && val != "" {
				return []byte(val)
			} else {
				return bytes.TrimPrefix(b(3), []byte{'|'})
			}
		}

		key := bytes.TrimSuffix(b(1), []byte{'='})

		if val, ok := vars[string(b(2))]; ok && val != "" {
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

	fmt.Println(string(*buf))

	return nil
}

func Markdown(buf *[]byte) error {
	// fmt.Println(string(*buf))

	//todo: compile markdown

	return nil
}

func CompressHTML(buf *[]byte) {
	//todo: compress html output
}

func Compile(root string, vars map[string]string) error {
	if stat, err := os.Stat(root + "/pages"); err != nil || !stat.IsDir() {
		if err != nil {
			return err
		} else {
			return fmt.Errorf("pages directory is missing")
		}
	}

	// os.RemoveAll(root + "/dist")
	// os.MkdirAll(root+"/dist", 0755)

	layoutBuf, err := os.ReadFile(root + "/pages/#layout.html")
	if err != nil {
		layoutBuf, err = os.ReadFile(root + "/pages/#layout.md")
		if err != nil {
			layoutBuf = defLayoutBuf
			os.WriteFile(root+"/pages/#layout.html", layoutBuf, 0755)
		} else if err := Markdown(&layoutBuf); err != nil {
			return err
		}
	}

	//todo: add layout const vars and logic

	compVars(&layoutBuf, vars)

	// fmt.Println(string(layoutBuf))

	return nil

	CompressHTML(&layoutBuf)
	os.WriteFile(root+"/dist/#layout.html", layoutBuf, 0755)

	buf := compEmbed(root+"/pages", root+"/pages", layoutBuf)

	// fmt.Println(string(buf))
	_ = buf

	//todo: add vars and logic

	//todo: compile sub pages and directories

	/* dirPath, err := goutil.JoinPath(root, "pages")
	if err != nil {
		return err
	}

	fileList, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	for _, file := range fileList {
		fmt.Println(file.Name())
	} */

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

func compEmbed(root string, path string, buf []byte) []byte {
	return regex.Comp(`{@([\w_\-]+)}`).RepFunc(buf, func(b func(int) []byte) []byte {
		ePath, err := goutil.JoinPath(path, string(b(1)))
		if err != nil {
			return b(0)
		}

		eBuf, err := getPageBuf(root, ePath)
		if err != nil {
			return b(0)
		}

		eBuf = compEmbed(root, path, eBuf)
		if eBuf == nil {
			return []byte{}
		}
		return eBuf
	})
}

func compVars(buf *[]byte, vars map[string]string) {
	*buf = regex.Comp(`(?s){([?!])([\w_\-]+)\s*{(.*?)}}`).RepFunc(*buf, func(b func(int) []byte) []byte {
		val, ok := vars[string(b(2))]

		if (b(1)[0] == '?' && (!ok || val == "")) || (b(1)[0] == '!' && (ok && val != "")) {
			return b(0)
		}

		return b(3)
	})

	*buf = regex.Comp(`{(#|(?:[\w_\-]+|)=|)["']?([\w_\-]+)(\|.*?|)["']?}`).RepFunc(*buf, func(b func(int) []byte) []byte {
		if len(b(1)) == 0 {
			if val, ok := vars[string(b(2))]; ok && val != "" {
				return goutil.HTML.Escape([]byte(val))
			} else {
				return bytes.TrimPrefix(b(3), []byte{'|'})
			}
		} else if b(1)[0] == '#' {
			if val, ok := vars[string(b(2))]; ok && val != "" {
				return []byte(val)
			} else {
				return bytes.TrimPrefix(b(3), []byte{'|'})
			}
		}

		key := bytes.TrimSuffix(b(1), []byte{'='})

		if val, ok := vars[string(b(2))]; ok && val != "" {
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
}

func getPageBuf(root string, path string) ([]byte, error) {
	buf, err := os.ReadFile(path + ".html")
	if err != nil {
		buf, err = os.ReadFile(path + ".md")
		if err == nil {
			if err := Markdown(&buf); err != nil {
				return []byte{}, err
			}
		}
	}

	for err != nil {
		path = string(regex.Comp(`\/[^\/]+\/([^\/]+)$`).Rep([]byte(path), []byte("/$1")))
		if !strings.HasPrefix(path, root) {
			return []byte{}, os.ErrExist
		}

		buf, err = os.ReadFile(path + ".html")
		if err != nil {
			buf, err = os.ReadFile(path + ".md")
			if err == nil {
				if err := Markdown(&buf); err != nil {
					return []byte{}, err
				}
			}
		}
	}

	return buf, nil
}
