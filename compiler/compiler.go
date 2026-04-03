package compiler

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/html"
	"github.com/tkdeng/goutil"
	"github.com/tkdeng/nexusweb/compress"
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

//go:embed templates/about.md
var defBufAbout []byte

//go:embed templates/more.md
var defBufMore []byte

//go:embed templates/header.html
var defBufHeader []byte

//go:embed templates/@widget.html
var defBufWidget []byte

func ReadFileHTML(name string, domains []string) ([]byte, map[string]string, error) {
	var buf []byte
	var err error
	var isMD bool

	if strings.HasSuffix(name, ".html") {
		buf, err = os.ReadFile(name)

		if err != nil {
			return []byte{}, map[string]string{}, err
		}
	} else if strings.HasSuffix(name, ".md") {
		isMD = true
		buf, err = os.ReadFile(name)

		if err != nil {
			return []byte{}, map[string]string{}, err
		}
	} else {
		buf, err = os.ReadFile(name + ".html")
		if err != nil {
			isMD = true
			buf, err = os.ReadFile(name + ".md")
		}

		if err != nil {
			return []byte{}, map[string]string{}, err
		}
	}

	var ymlVars map[string]string
	if !isMD {
		ymlVars = getPageYml(&buf, name+".html")
	} else {
		ymlVars = getPageYml(&buf, name+".md")
	}

	encodeVars(&buf)

	if isMD {
		Markdown(&buf, domains)
	}

	return buf, ymlVars, nil
}

func WriteFileHTML(name string, data []byte, root ...string) error {
	name = strings.TrimSuffix(name, ".html")
	name = strings.TrimSuffix(name, ".md")

	decodeVars(&data)

	// use in memory map instead of /dist filesystem
	if len(root) != 0 && root[0] != "" {
		PageBuf.Set(strings.TrimPrefix(name, root[0]), compSegHTML(&data))
		return nil
	}

	return os.WriteFile(name+".html", data, 0755)
}

func compSegHTML(buf *[]byte) []SegHTML {
	varSeg := []SegHTML{}

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
			if len(cont) != 0 {
				s := compSegHTML(&cont)

				varSeg = append(varSeg, SegHTML{
					t:    t,
					name: string(name),
					seg:  s,
				})

				return []byte("{%SPLIT%}")
			}
		case ':':
			if _, ok := plugins.Get(string(name)); ok {
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

				var s []SegHTML
				if len(cont) != 0 {
					s = compSegHTML(&cont)
				}

				varSeg = append(varSeg, SegHTML{
					t:    t,
					name: string(name),
					args: args,
					seg:  s,
				})

				return []byte("{%SPLIT%}")
			}
		case '#', '=':
			var def []byte
			if len(atts) != 0 && atts[0] == '|' {
				def = atts[1:]
			}

			varSeg = append(varSeg, SegHTML{
				t:    t,
				name: string(name),
				body: def,
			})

			return []byte("{%SPLIT%}")
		default:
			if len(atts) != 0 && atts[0] == '=' {
				varSeg = append(varSeg, SegHTML{
					t:    '=',
					name: string(bytes.Trim(atts[1:], "\"' \t")),
				})

				return []byte("{%SPLIT%}")
			}

			var def []byte
			if len(atts) != 0 && atts[0] == '|' {
				def = atts[1:]
			}

			varSeg = append(varSeg, SegHTML{
				t:    ' ',
				name: string(name),
				body: def,
			})

			return []byte("{%SPLIT%}")
		}

		return nil
	})

	bufSeg := bytes.Split(*buf, []byte("{%SPLIT%}"))

	seg := make([]SegHTML, len(varSeg)+len(bufSeg))

	for i := 0; i < len(bufSeg)-1; i++ {
		seg = append(seg, SegHTML{
			t:    0,
			body: compress.Zip(bufSeg[i]),
		}, varSeg[i])
	}
	seg = append(seg, SegHTML{
		t:    0,
		body: compress.Zip(bufSeg[len(bufSeg)-1]),
	})

	return seg
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
		os.WriteFile(root+"/pages/#layout.html", recodeVars(defBufLayout), 0755)
		os.WriteFile(root+"/pages/@error.html", recodeVars(defBufError), 0755)
		os.WriteFile(root+"/pages/head.html", recodeVars(defBufHead), 0755)
		os.WriteFile(root+"/pages/body.md", recodeVars(defBufBody), 0755)

		os.MkdirAll(root+"/pages/about", 0755)
		os.WriteFile(root+"/pages/about/body.md", recodeVars(defBufAbout), 0755)
		os.MkdirAll(root+"/pages/about/more", 0755)
		os.WriteFile(root+"/pages/about/more/body.md", recodeVars(defBufMore), 0755)

		os.WriteFile(root+"/pages/header.html", recodeVars(defBufHeader), 0755)
		os.WriteFile(root+"/pages/@widget.html", recodeVars(defBufWidget), 0755)
	}

	/* os.RemoveAll(root + "/dist")
	os.MkdirAll(root+"/dist", 0755) */

	layoutBuf, _, err := ReadFileHTML(root+"/pages/#layout", domains)
	if err != nil {
		layoutBuf = defBufLayout
		encodeVars(&layoutBuf)
		if err = WriteFileHTML(root+"/pages/#layout", layoutBuf); err != nil {
			PrintMsg("error", "Error: Failed to write default layout page!")
			fmt.Println(err)
		}
	}

	if stat, err := os.Stat(root + "/pages/@error.html"); err != nil || stat.IsDir() {
		if stat, err := os.Stat(root + "/pages/@error.md"); err != nil || stat.IsDir() {
			buf := defBufError
			encodeVars(&buf)
			if err = WriteFileHTML(root+"/pages/@error", buf); err != nil {
				PrintMsg("error", "Error: Failed to write default @error page!")
				fmt.Println(err)
			}
		}
	}

	compVars(&layoutBuf, vars, nil)
	CompressHTML(&layoutBuf, debugMode)

	if err = WriteFileHTML(root+"/dist/#layout", compLayoutEmbed(root+"/pages", root+"/pages", root+"/pages/#layout", layoutBuf, domains, vars), root+"/dist"); err != nil {
		PrintMsg("error", "Error: Failed to write root #layout page!")
		fmt.Println(err)
	}

	buf, ymlVars := compEmbed(root+"/pages", root+"/pages", root+"/pages/#layout", layoutBuf, domains)
	compVars(&buf, vars, ymlVars)

	if err = WriteFileHTML(root+"/dist/index", buf, root+"/dist"); err != nil {
		PrintMsg("error", "Error: Failed to write home page!")
		fmt.Println(err)
	}

	if fileList, err := os.ReadDir(root + "/pages"); err == nil {
		for _, file := range fileList {
			fName := file.Name()

			if !file.IsDir() && strings.HasPrefix(fName, "@") {
				if buf, _, err := ReadFileHTML(root+"/pages/"+fName, domains); err == nil {
					fileName := regex.Comp(`\.(html|md)$`).RepLitStr(fName, "")
					buf, ymlVars := compEmbed(root+"/pages", root+"/pages", root+"/pages/"+fileName, buf, domains)
					compVars(&buf, vars, ymlVars)
					WriteFileHTML(root+"/dist/"+fileName, buf, root+"/dist")
				}
			} else if !file.IsDir() && strings.HasPrefix(fName, "#") {
				if fName == "#layout.html" || fName == "#layout.md" {
					continue
				}

				if buf, _, err := ReadFileHTML(root+"/pages/"+fName, domains); err == nil {
					compVars(&buf, vars, nil)
					CompressHTML(&buf, debugMode)

					fileName := regex.Comp(`\.(html|md)$`).RepLitStr(fName, "")
					WriteFileHTML(root+"/dist/"+fileName, compLayoutEmbed(root+"/pages", root+"/pages", root+"/pages/"+fileName, buf, domains, vars), root+"/dist")
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

	//todo: compile an rss.xml and sitemap.xml
	// might also consider using a go variable to dynamically update and return it at runtime
	// note: rss is for "Whats New" pages, sitemap is for browser indexing

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

	// os.MkdirAll(distPath, 0755)
	if err := WriteFileHTML(distPath+"/index", buf, root+"/dist"); err != nil {
		PrintMsg("error", "Error: Failed to write home page!")
		fmt.Println(err)
	}

	if fileList, err := os.ReadDir(path); err == nil {
		for _, file := range fileList {
			fName := file.Name()

			if !file.IsDir() && strings.HasPrefix(fName, "@") {
				if buf, _, err := ReadFileHTML(path+"/"+fName, domains); err == nil {
					fileName := regex.Comp(`\.(html|md)$`).RepLitStr(fName, "")
					buf, ymlVars = compEmbed(root+"/pages", path, path+"/"+fileName, buf, domains)
					compVars(&buf, vars, ymlVars)
					WriteFileHTML(distPath+"/"+fileName, buf, root+"/dist")
				}
			} else if !file.IsDir() && strings.HasPrefix(fName, "#") {
				if buf, _, err := ReadFileHTML(path+"/"+fName, domains); err == nil {
					compVars(&buf, vars, ymlVars)
					CompressHTML(&buf, debugMode)
					fileName := regex.Comp(`\.(html|md)$`).RepLitStr(fName, "")
					WriteFileHTML(distPath+"/"+fileName, compLayoutEmbed(root+"/pages", path, path+"/"+fileName, buf, domains, vars), root+"/dist")
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

func compLayoutEmbed(root string, path string, oPath string, buf []byte, domains []string, vars map[string]string) []byte {
	buf = bytes.ReplaceAll(buf, []byte("{@body}"), []byte("{!@body!}"))

	buf, ymlVars := compEmbed(root, path, oPath, buf, domains)
	compVars(&buf, vars, ymlVars)

	buf = bytes.ReplaceAll(buf, []byte("{!@body!}"), []byte("{@body}"))

	return buf
}

func compVars(buf *[]byte, vars map[string]string, ymlVars map[string]string) {
	regex.Comp(`(?s)<param data=["']?([^"'>]+)["']?>(.*?)</param>`).RepFunc(*buf, func(b func(int) []byte) []byte {
		if dec, err := compress.UnZip(b(1)); err == nil {
			data := regex.Comp(`^([?!:#=]?)(\$?)([\w_\-]+)\s*([^\r\n]*)$`).Split(dec)

			var t byte
			if len(data[1]) != 0 {
				t = data[1][0]
			}
			d := len(data[2]) != 0
			name := data[3]
			atts := data[4]
			cont := b(2)

			switch t {
			case '?', '!':
				if ymlVars != nil && len(ymlVars) != 0 {
					val, ok := ymlVars[string(name)]
					if !((t == '?' && (!ok || val == "")) || (t == '!' && (ok && val != ""))) {
						if len(cont) != 0 {
							compVars(&cont, vars, ymlVars)
							return cont
						}
						return []byte{}
					}
				}

				if d {
					break
				}

				val, ok := vars[string(name)]
				if !((t == '?' && (!ok || val == "")) || (t == '!' && (ok && val != ""))) {
					if len(cont) != 0 {
						compVars(&cont, vars, ymlVars)
						return cont
					}
					return []byte{}
				}
			case ':':
				if d {
					break
				}

				if plugin, ok := plugins.Get(string(name), true); ok {
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
						compVars(&cont, vars, ymlVars)
					}

					out, err := plugin.Run(args, bytes.TrimSpace(cont), true)
					if err != nil {
						PrintMsg("warn", "Warning: Plugin Error!")
						fmt.Println("  plugin:", string(b(1)))
						fmt.Println(err)
						break
					}

					return out
				}
			case '#':
				if ymlVars != nil && len(ymlVars) != 0 {
					if val, ok := ymlVars[string(name)]; ok && val != "" {
						return []byte(val)
					}
				}

				if d {
					break
				}

				if val, ok := vars[string(name)]; ok && val != "" {
					return []byte(val)
				}
			case '=':
				if ymlVars != nil && len(ymlVars) != 0 {
					if val, ok := ymlVars[string(name)]; ok && val != "" {
						return goutil.HTML.EscapeArgs([]byte(val))
					}
				}

				if d {
					break
				}

				if val, ok := vars[string(name)]; ok && val != "" {
					return goutil.HTML.EscapeArgs([]byte(val))
				}
			default:
				if len(atts) != 0 && atts[0] == '=' {
					if ymlVars != nil && len(ymlVars) != 0 {
						if val, ok := ymlVars[string(bytes.Trim(atts[1:], "\"' \t"))]; ok && val != "" {
							return regex.JoinBytes(name, `="`, goutil.HTML.EscapeArgs([]byte(val), '"'), '"')
						}
					}

					if d {
						break
					}

					if val, ok := vars[string(bytes.Trim(atts[1:], "\"' \t"))]; ok && val != "" {
						return regex.JoinBytes(name, `="`, goutil.HTML.EscapeArgs([]byte(val), '"'), '"')
					}

					break
				}

				if ymlVars != nil && len(ymlVars) != 0 {
					if val, ok := ymlVars[string(name)]; ok && val != "" {
						return goutil.HTML.Escape([]byte(val))
					}
				}

				if d {
					break
				}

				if val, ok := vars[string(name)]; ok && val != "" {
					return goutil.HTML.Escape([]byte(val))
				}
			}
		}

		return b(0)
	})

	*buf = regex.Comp(`{%([^{}%]+)%}`).RepFunc(*buf, func(b func(int) []byte) []byte {
		if dec, err := compress.UnZip(b(1)); err == nil {
			data := regex.Comp(`^([?!:#=]?)(\$?)([\w_\-]+)\s*([^\r\n]*)$`).Split(dec)

			var t byte
			if len(data[1]) != 0 {
				t = data[1][0]
			}
			d := len(data[2]) != 0
			name := data[3]
			atts := data[4]

			switch t {
			case '?', '!':
				// no content in this encoding
				return nil
			case ':':
				if d {
					break
				}

				if plugin, ok := plugins.Get(string(name), true); ok {
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

					out, err := plugin.Run(args, nil, true)
					if err != nil {
						PrintMsg("warn", "Warning: Plugin Error!")
						fmt.Println("  plugin:", string(b(1)))
						fmt.Println(err)
						break
					}

					return out
				}
			case '#':
				if ymlVars != nil && len(ymlVars) != 0 {
					if val, ok := ymlVars[string(name)]; ok && val != "" {
						return []byte(val)
					}
				}

				if d {
					break
				}

				if val, ok := vars[string(name)]; ok && val != "" {
					return []byte(val)
				}
			case '=':
				if ymlVars != nil && len(ymlVars) != 0 {
					if val, ok := ymlVars[string(name)]; ok && val != "" {
						return goutil.HTML.EscapeArgs([]byte(val))
					}
				}

				if d {
					break
				}

				if val, ok := vars[string(name)]; ok && val != "" {
					return goutil.HTML.EscapeArgs([]byte(val))
				}
			default:
				if len(atts) != 0 && atts[0] == '=' {
					if ymlVars != nil && len(ymlVars) != 0 {
						if val, ok := ymlVars[string(bytes.Trim(atts[1:], "\"' \t"))]; ok && val != "" {
							return regex.JoinBytes(name, `="`, goutil.HTML.EscapeArgs([]byte(val), '"'), '"')
						}
					}

					if d {
						break
					}

					if val, ok := vars[string(bytes.Trim(atts[1:], "\"' \t"))]; ok && val != "" {
						return regex.JoinBytes(name, `="`, goutil.HTML.EscapeArgs([]byte(val), '"'), '"')
					}

					break
				}

				if ymlVars != nil && len(ymlVars) != 0 {
					if val, ok := ymlVars[string(name)]; ok && val != "" {
						return goutil.HTML.Escape([]byte(val))
					}
				}

				if d {
					break
				}

				if val, ok := vars[string(name)]; ok && val != "" {
					return goutil.HTML.Escape([]byte(val))
				}
			}
		}

		return b(0)
	})
}

func getPageBuf(root string, path string, domains []string) ([]byte, map[string]string, error) {
	buf, ymlVars, err := ReadFileHTML(path, domains)
	if err != nil {
		dPath := regex.Comp(`\/([^\/]+)$`).RepStr(path, "/@$1")
		buf, ymlVars, err = ReadFileHTML(dPath, domains)
	}

	for err != nil {
		// path = regex.Comp(`\/[^\/]+\/([^\/]+)$`).RepStr(path, "/$1")
		path = filepath.Join(filepath.Dir(filepath.Dir(path)), filepath.Base(path))

		if !strings.HasPrefix(path, root) {
			return []byte{}, map[string]string{}, os.ErrNotExist
		}

		buf, ymlVars, err = ReadFileHTML(path, domains)
		if err != nil {
			dPath := regex.Comp(`\/([^\/]+)$`).RepStr(path, "/@$1")
			buf, ymlVars, err = ReadFileHTML(dPath, domains)
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
