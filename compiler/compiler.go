package compiler

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/html"
	"github.com/tkdeng/goutil"
	"github.com/tkdeng/nexusweb/plugins"
	"github.com/tkdeng/regex"
	"gopkg.in/yaml.v3"
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
			return nil
		}

		return b(3)
	})

	*buf = regex.Comp(`{@([\w_\-]+)}`).RepFunc(*buf, func(b func(int) []byte) []byte {
		ePath, err := goutil.JoinPath(regex.Comp(`\/[^\/]+$`).RepStr(path, ""), string(b(1)))
		if err != nil || !strings.HasPrefix(ePath, root+"/dist") {
			return nil
		}

		if !strings.HasSuffix(ePath, ".html") {
			ePath += ".html"
		}

		eBuf, err := os.ReadFile(ePath)

		for err != nil {
			ePath = regex.Comp(`\/[^\/]+\/([^\/]+)$`).RepStr(ePath, "/$1")

			if !strings.HasPrefix(ePath, root+"/dist") {
				return nil
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

		return nil
	})

	*buf = regex.Comp(`(?s){:([\w_\-]+)((?:\s+[\w_\-]+(?:\s*=\s*(?:"[^"]*"|'[^']*'|[\w_\-]+)|))+|)\s*(?:\{(.*?)\}|)}`).RepFunc(*buf, func(b func(int) []byte) []byte {
		if plugin, ok := plugins.Get(string(b(1))); ok {
			args := map[string]string{}
			ind := 0
			regex.Comp(`([\w_\-]+)(?:\s*(=)\s*"([^"]*)"|'([^"]*)'|([\w_\-]+)|)`).RepFunc(b(2), func(b func(int) []byte) []byte {
				if len(b(2)) == 0 {
					args[strconv.Itoa(ind)] = string(goutil.Clean(b(1)))
					ind++
				} else {
					args[string(goutil.Clean(b(1)))] = string(goutil.Clean(b(3)))
				}
				return nil
			})

			out, err := plugin.Run(args, bytes.TrimSpace(b(3)), false)

			if err != nil {
				PrintMsg("warn", "Warning: Plugin Error!")
				fmt.Println("  plugin:", string(b(1)))
				fmt.Println(err)
				return nil
			}

			return out
		}

		return nil
	})

	return nil
}

func CompressHTML(buf *[]byte, debugMode bool) {
	// minify HTML
	m := minify.New()
	m.AddFunc("text/html", html.Minify)

	m.Add("text/html", &html.Minifier{
		KeepQuotes:       true,
		KeepDocumentTags: true,
		KeepEndTags:      true,
		KeepWhitespace:   debugMode,
	})

	var b bytes.Buffer
	if err := m.Minify("text/html", &b, bytes.NewBuffer(*buf)); err == nil {
		*buf = b.Bytes()
	}
}

func Compile(root string, vars map[string]string, domains []string, debugMode bool) error {
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
		} else {
			Markdown(&layoutBuf, domains)
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

	compVars(&layoutBuf, vars, nil)
	CompressHTML(&layoutBuf, debugMode)
	if err = os.WriteFile(root+"/dist/#layout.html", layoutBuf, 0755); err != nil {
		PrintMsg("error", "Error: Failed to write root #layout page!")
		fmt.Println(err)
	}

	buf, ymlVars := compEmbed(root+"/pages", root+"/pages", root+"/pages/#layout", layoutBuf, domains)
	compVars(&buf, vars, ymlVars)

	if err = os.WriteFile(root+"/dist/index.html", buf, 0755); err != nil {
		PrintMsg("error", "Error: Failed to write home page!")
		fmt.Println(err)
	}

	if fileList, err := os.ReadDir(root + "/pages"); err == nil {
		for _, file := range fileList {
			fName := file.Name()

			if !file.IsDir() && strings.HasPrefix(fName, "@") {
				if buf, err := os.ReadFile(root + "/pages/" + fName); err == nil {
					if strings.HasSuffix(fName, ".md") {
						Markdown(&buf, domains)
					}

					fileName := regex.Comp(`\.(html|md)$`).RepLitStr(fName, "")
					buf, ymlVars := compEmbed(root+"/pages", root+"/pages", root+"/pages/"+fileName, buf, domains)
					compVars(&buf, vars, ymlVars)
					os.WriteFile(root+"/dist/"+fileName+".html", buf, 0755)
				}
			} else if !file.IsDir() && strings.HasPrefix(fName, "#") {
				if fName == "#layout.html" || fName == "#layout.md" {
					continue
				}

				if buf, err := os.ReadFile(root + "/pages/" + fName); err == nil {
					if strings.HasSuffix(fName, ".md") {
						Markdown(&buf, domains)
					}

					compVars(&buf, vars, nil)
					CompressHTML(&buf, debugMode)

					fileName := regex.Comp(`\.(html|md)$`).RepLitStr(fName, "")
					os.WriteFile(root+"/dist/"+fileName+".html", buf, 0755)
				}
			} else if file.IsDir() {
				if err = compPages(root, root+"/pages/"+fName, vars, domains, &layoutBuf, debugMode); err != nil {
					PrintMsg("error", "Error: Failed to compile page!")
					fmt.Println("  path:", root+"/pages/"+fName)
					fmt.Println(err)
				}
			}
		}
	}

	return nil
}

func compPages(root string, path string, vars map[string]string, domains []string, layoutBuf *[]byte, debugMode bool) error {
	var buf []byte
	var ymlVars map[string]string

	if lBuf, _, err := getPageBuf(regex.Comp(`^(%1)/pages/([^\/]+(?:\/.*|))$`, root).RepStr(path, "$1/pages/$2"), path+"/#layout", domains); err == nil {
		buf, ymlVars = compEmbed(root+"/pages", path, path+"/#layout", lBuf, domains)
	} else {
		buf, ymlVars = compEmbed(root+"/pages", path, path+"/#layout", *layoutBuf, domains)
	}

	compVars(&buf, vars, ymlVars)

	distPath := regex.Comp(`^(%1)/pages/([^\/]+(?:\/.*|))$`, root).RepStr(path, "$1/dist/$2")

	os.MkdirAll(distPath, 0755)
	if err := os.WriteFile(distPath+"/index.html", buf, 0755); err != nil {
		PrintMsg("error", "Error: Failed to write home page!")
		fmt.Println(err)
	}

	if fileList, err := os.ReadDir(path); err == nil {
		for _, file := range fileList {
			fName := file.Name()

			if !file.IsDir() && strings.HasPrefix(fName, "@") {
				if buf, err := os.ReadFile(path + "/" + fName); err == nil {
					if strings.HasSuffix(fName, ".md") {
						Markdown(&buf, domains)
					}

					fileName := regex.Comp(`\.(html|md)$`).RepLitStr(fName, "")
					buf, ymlVars = compEmbed(root+"/pages", path, path+"/"+fileName, buf, domains)
					compVars(&buf, vars, ymlVars)
					os.WriteFile(distPath+"/"+fileName+".html", buf, 0755)
				}
			} else if !file.IsDir() && strings.HasPrefix(fName, "#") {
				if buf, err := os.ReadFile(path + "/" + fName); err == nil {
					if strings.HasSuffix(fName, ".md") {
						Markdown(&buf, domains)
					}

					compVars(&buf, vars, ymlVars)
					CompressHTML(&buf, debugMode)
					fileName := regex.Comp(`\.(html|md)$`).RepLitStr(fName, "")
					os.WriteFile(distPath+"/"+fileName+".html", buf, 0755)
				}
			} else if file.IsDir() {
				if err = compPages(root, path+"/"+fName, vars, domains, layoutBuf, debugMode); err != nil {
					PrintMsg("error", "Error: Failed to compile page!")
					fmt.Println("  path:", root+"/pages/"+fName)
					fmt.Println(err)
				}
			}
		}
	}

	return nil
}

func compEmbed(root string, path string, oPath string, buf []byte, domains []string) ([]byte, map[string]string) {
	ymlVars := map[string]string{}

	buf = regex.Comp(`{@([\w_\-]+)}`).RepFunc(buf, func(b func(int) []byte) []byte {
		ePath, err := goutil.JoinPath(path, string(b(1)))
		if err != nil || ePath == oPath {
			if ePath == oPath {
				PrintMsg("warn", "Warning: Recursion Detected!")
				fmt.Println("  path:", ePath)
			}
			return b(0)
		}

		eBuf, vars, err := getPageBuf(root, ePath, domains)
		if err != nil {
			return b(0)
		}

		ymlVars = vars

		eBuf, vars = compEmbed(root, path, ePath, eBuf, domains)

		for k, v := range vars {
			if _, ok := ymlVars[k]; !ok {
				ymlVars[k] = v
			}
		}

		return eBuf
	})

	return buf, ymlVars
}

func compVars(buf *[]byte, vars map[string]string, ymlVars map[string]string) {
	if ymlVars != nil && len(ymlVars) != 0 {
		*buf = regex.Comp(`(?s){([?!])\$?([\w_\-]+)\s*{(.*?)}}`).RepFunc(*buf, func(b func(int) []byte) []byte {
			val, ok := ymlVars[string(b(2))]

			if (b(1)[0] == '?' && (!ok || val == "")) || (b(1)[0] == '!' && (ok && val != "")) {
				return nil
			}

			return b(3)
		})

		*buf = regex.Comp(`{(#|(?:[\w_\-]+|)=|)["']?\$?([\w_\-]+)(\|.*?|)["']?}`).RepFunc(*buf, func(b func(int) []byte) []byte {
			if len(b(1)) == 0 {
				if val, ok := ymlVars[string(b(2))]; ok && val != "" {
					return goutil.HTML.Escape([]byte(val))
				} else {
					return bytes.TrimPrefix(b(3), []byte{'|'})
				}
			} else if b(1)[0] == '#' {
				if val, ok := ymlVars[string(b(2))]; ok && val != "" {
					return []byte(val)
				} else {
					return bytes.TrimPrefix(b(3), []byte{'|'})
				}
			}

			key := bytes.TrimSuffix(b(1), []byte{'='})

			if val, ok := ymlVars[string(b(2))]; ok && val != "" {
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

			return nil
		})
	}

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
			ind := 0
			regex.Comp(`([\w_\-]+)(?:\s*(=)\s*"([^"]*)"|'([^"]*)'|([\w_\-]+)|)`).RepFunc(b(2), func(b func(int) []byte) []byte {
				if len(b(2)) == 0 {
					args[strconv.Itoa(ind)] = string(goutil.Clean(b(1)))
					ind++
				} else {
					args[string(goutil.Clean(b(1)))] = string(goutil.Clean(b(3)))
				}
				return nil
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

func getPageBuf(root string, path string, domains []string) ([]byte, map[string]string, error) {
	var ymlVars map[string]string

	buf, err := os.ReadFile(path + ".html")
	if err == nil {
		ymlVars = getPageYml(&buf, path+".html")
	} else if buf, err = os.ReadFile(path + ".md"); err == nil {
		ymlVars = getPageYml(&buf, path+".md")
		Markdown(&buf, domains)
	}

	if err != nil {
		dPath := regex.Comp(`\/([^\/]+)$`).RepStr(path, "/@$1")
		buf, err = os.ReadFile(dPath + ".html")
		if err == nil {
			ymlVars = getPageYml(&buf, dPath+".html")
		} else if buf, err = os.ReadFile(dPath + ".md"); err == nil {
			ymlVars = getPageYml(&buf, dPath+".md")
			Markdown(&buf, domains)
		}
	}

	for err != nil {
		path = regex.Comp(`\/[^\/]+\/([^\/]+)$`).RepStr(path, "/$1")
		if !strings.HasPrefix(path, root) {
			return []byte{}, map[string]string{}, os.ErrNotExist
		}

		buf, err = os.ReadFile(path + ".html")
		if err == nil {
			ymlVars = getPageYml(&buf, path+".html")
		} else if buf, err = os.ReadFile(path + ".md"); err == nil {
			ymlVars = getPageYml(&buf, path+".md")
			Markdown(&buf, domains)
		}

		if err != nil {
			dPath := regex.Comp(`\/([^\/]+)$`).RepStr(path, "/@$1")
			buf, err = os.ReadFile(dPath + ".html")
			if err == nil {
				ymlVars = getPageYml(&buf, dPath+".html")
			} else if buf, err = os.ReadFile(dPath + ".md"); err == nil {
				ymlVars = getPageYml(&buf, dPath+".md")
				Markdown(&buf, domains)
			}
		}
	}

	return buf, ymlVars, nil
}

func getPageYml(buf *[]byte, path string) map[string]string {
	ymlVars := make(map[string]string)

	*buf = regex.Comp(`(?s)^---(.*?)---`).RepFunc(*buf, func(b func(int) []byte) []byte {
		if err := yaml.Unmarshal(b(1), &ymlVars); err != nil {
			PrintMsg("warn", "Warning: Failed to parse yaml in file!")
			fmt.Println("  path:", path)
			fmt.Println(err)
		}

		return nil
	})

	return ymlVars
}

func Live(root string, vars map[string]string) {
	//todo: add live compiler with file change listener
	// and limit to affected subdirs for performance
	// note: parent pages may still need to update child subpages
}
