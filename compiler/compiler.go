package compiler

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"strings"

	"github.com/tkdeng/goutil"
	"github.com/tkdeng/nexusweb/plugins"
	"github.com/tkdeng/regex"
)

//go:embed templates/#layout.html
var defBufLayout []byte

//go:embed templates/@error.html
var defBufError []byte

//go:embed templates/head.html
var defBufHead []byte

//go:embed templates/body.md
var defBufBody []byte

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

	*buf = regex.Comp(`(?s){:([\w_\-]+)((?:\s+[\w_\-]+(?:\s*=\s*(?:"[^"]*"|'[^']*'|[\w_\-]+)|))+|)\s*(?:\{(.*?)\}|)}`).RepFunc(*buf, func(b func(int) []byte) []byte {
		if plugin, ok := plugins.Get(string(b(1))); ok {
			args := map[string]string{}
			regex.Comp(`([\w_\-]+)(?:\s*=\s*"([^"]*)"|'([^"]*)'|([\w_\-]+)|)`).RepFunc(b(2), func(b func(int) []byte) []byte {
				args[string(goutil.Clean(b(1)))] = string(goutil.Clean(b(2)))
				return []byte{}
			})

			out, err := plugin.Run(args, bytes.TrimSpace(b(3)), false)

			if err != nil {
				PrintMsg("warn", "Warning: Plugin Error!")
				fmt.Println("  plugin:", string(b(1)))
				fmt.Println(err)
				return []byte{}
			}

			return out
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

func CompressHTML(buf *[]byte) {
	//todo: compress html output
}

func Compile(root string, vars map[string]string) error {
	if stat, err := os.Stat(root + "/pages"); err != nil || !stat.IsDir() {
		if stat.IsDir() {
			return fmt.Errorf("pages directory is missing")
		}

		os.MkdirAll(root+"/pages", 0755)
		os.WriteFile(root+"/pages/#layout.html", defBufLayout, 0755)
		os.WriteFile(root+"/pages/@error.html", defBufError, 0755)
		os.WriteFile(root+"/pages/head.html", defBufHead, 0755)
		os.WriteFile(root+"/pages/body.md", defBufBody, 0755)
	}

	os.RemoveAll(root + "/dist")
	os.MkdirAll(root+"/dist", 0755)

	layoutBuf, err := os.ReadFile(root + "/pages/#layout.html")
	if err != nil {
		layoutBuf, err = os.ReadFile(root + "/pages/#layout.md")
		if err != nil {
			layoutBuf = defBufLayout
			if err = os.WriteFile(root+"/pages/#layout.html", layoutBuf, 0755); err != nil {
				PrintMsg("error", "Error: Failed to write default layout page!")
				fmt.Println(err)
			}
		} else if err := Markdown(&layoutBuf); err != nil {
			return err
		}
	}

	if stat, err := os.Stat(root + "/pages/@error.html"); err != nil || stat.IsDir() {
		if stat, err := os.Stat(root + "/pages/@error.md"); err != nil || stat.IsDir() {
			if err = os.WriteFile(root+"/pages/@error.html", defBufError, 0755); err != nil {
				PrintMsg("error", "Error: Failed to write default @error page!")
				fmt.Println(err)
			}
		}
	}

	/* if stat, err := os.Stat(root + "/pages/head.html"); err != nil || stat.IsDir() {
		if stat, err := os.Stat(root + "/pages/head.md"); err != nil || stat.IsDir() {
			if err = os.WriteFile(root+"/pages/head.html", defBufHead, 0755); err != nil {
				PrintMsg("error", "Error: Failed to write default home page head!")
				fmt.Println(err)
			}
		}
	} */

	/* if stat, err := os.Stat(root + "/pages/body.html"); err != nil || stat.IsDir() {
		if stat, err := os.Stat(root + "/pages/body.md"); err != nil || stat.IsDir() {
			if err = os.WriteFile(root+"/pages/body.md", defBufBody, 0755); err != nil {
				PrintMsg("error", "Error: Failed to write default home page body!")
				fmt.Println(err)
			}
		}
	} */

	compVars(&layoutBuf, vars)
	CompressHTML(&layoutBuf)
	if err = os.WriteFile(root+"/dist/#layout.html", layoutBuf, 0755); err != nil {
		PrintMsg("error", "Error: Failed to write root #layout page!")
		fmt.Println(err)
	}

	buf := compEmbed(root+"/pages", root+"/pages", root+"/pages/#layout", layoutBuf)
	compVars(&buf, vars)

	if err = os.WriteFile(root+"/dist/index.html", buf, 0755); err != nil {
		PrintMsg("error", "Error: Failed to write home page!")
		fmt.Println(err)
	}

	fileList, err := os.ReadDir(root + "/pages")
	for _, file := range fileList {
		if !file.IsDir() && strings.HasPrefix(file.Name(), "@") {
			if buf, err := os.ReadFile(root + "/pages/" + file.Name()); err == nil {
				buf := compEmbed(root+"/pages", root+"/pages", root+"/pages/"+string(regex.Comp(`\.(html|md)$`).RepLit([]byte(file.Name()), []byte{})), buf)
				compVars(&buf, vars)
				os.WriteFile(root+"/dist/"+string(regex.Comp(`\.(html|md)$`).RepLit([]byte(file.Name()), []byte(".html"))), buf, 0755)
			}
		} else if file.IsDir() {
			if err = compPages(root, root+"/pages/"+file.Name(), vars, &layoutBuf); err != nil {
				PrintMsg("error", "Error: Failed to compile page!")
				fmt.Println("  path:", root+"/pages/"+file.Name())
				fmt.Println(err)
			}
		}
	}

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

func compPages(root string, path string, vars map[string]string, layoutBuf *[]byte) error {
	//todo: compile sub pages and directories

	var buf []byte

	if lBuf, err := getPageBuf(string(regex.Comp(`^(%1)/pages/([^\/]+)(?:\/.*|)$`, root).Rep([]byte(path), []byte("$1/pages/$2"))), path+"/#layout"); err == nil {
		buf = compEmbed(root+"/pages", path, path+"/#layout", lBuf)
	} else {
		buf = compEmbed(root+"/pages", path, path+"/#layout", *layoutBuf)
	}

	compVars(&buf, vars)

	distPath := string(regex.Comp(`^(%1)/pages/([^\/]+)(?:\/.*|)$`, root).Rep([]byte(path), []byte("$1/dist/$2")))

	os.MkdirAll(distPath, 0755)
	if err := os.WriteFile(distPath+"/index.html", buf, 0755); err != nil {
		PrintMsg("error", "Error: Failed to write home page!")
		fmt.Println(err)
	}

	// fmt.Println(string(buf))

	return nil
}

func compEmbed(root string, path string, oPath string, buf []byte) []byte {
	return regex.Comp(`{@([\w_\-]+)}`).RepFunc(buf, func(b func(int) []byte) []byte {
		ePath, err := goutil.JoinPath(path, string(b(1)))
		if err != nil || ePath == oPath {
			if ePath == oPath {
				PrintMsg("warn", "Warning: Recursion Detected!")
				fmt.Println("  path:", ePath)
			}
			return b(0)
		}

		eBuf, err := getPageBuf(root, ePath)
		if err != nil {
			return b(0)
		}

		eBuf = compEmbed(root, path, ePath, eBuf)
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
			} else if len(b(3)) != 0 {
				return bytes.TrimPrefix(b(3), []byte{'|'})
			}
			return b(0)
		} else if b(1)[0] == '#' {
			if val, ok := vars[string(b(2))]; ok && val != "" {
				return []byte(val)
			} else if len(b(3)) != 0 {
				return bytes.TrimPrefix(b(3), []byte{'|'})
			}
			return b(0)
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

		return b(0)
	})

	*buf = regex.Comp(`(?s){:([\w_\-]+)((?:\s+[\w_\-]+(?:\s*=\s*(?:"[^"]*"|'[^']*'|[\w_\-]+)|))+|)\s*(?:\{(.*?)\}|)}`).RepFunc(*buf, func(b func(int) []byte) []byte {
		if plugin, ok := plugins.Get(string(b(1)), true); ok {
			args := map[string]string{}
			regex.Comp(`([\w_\-]+)(?:\s*=\s*"([^"]*)"|'([^"]*)'|([\w_\-]+)|)`).RepFunc(b(2), func(b func(int) []byte) []byte {
				args[string(goutil.Clean(b(1)))] = string(goutil.Clean(b(2)))
				return []byte{}
			})

			out, err := plugin.Run(args, bytes.TrimSpace(b(3)), true)

			if err != nil {
				PrintMsg("warn", "Warning: Plugin Error!")
				fmt.Println("  plugin:", string(b(1)))
				fmt.Println(err)
				return b(0)
			}

			return out
		}

		return b(0)
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

	if err != nil {
		dPath := string(regex.Comp(`\/([^\/]+)$`).Rep([]byte(path), []byte("/@$1")))
		buf, err = os.ReadFile(dPath + ".html")
		if err != nil {
			buf, err = os.ReadFile(dPath + ".md")
			if err == nil {
				if err := Markdown(&buf); err != nil {
					return []byte{}, err
				}
			}
		}
	}

	for err != nil {
		path = string(regex.Comp(`\/[^\/]+\/([^\/]+)$`).Rep([]byte(path), []byte("/$1")))
		if !strings.HasPrefix(path, root) {
			return []byte{}, os.ErrNotExist
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

		if err != nil {
			dPath := string(regex.Comp(`\/([^\/]+)$`).Rep([]byte(path), []byte("/@$1")))
			buf, err = os.ReadFile(dPath + ".html")
			if err != nil {
				buf, err = os.ReadFile(dPath + ".md")
				if err == nil {
					if err := Markdown(&buf); err != nil {
						return []byte{}, err
					}
				}
			}
		}
	}

	return buf, nil
}
