package nxweb

import (
	"strconv"

	lorem "github.com/drhodes/golorem"
	"github.com/tkdeng/nexusweb/plugins"
)

func init() {
	plugins.New("lorem", func(args map[string]string, cont []byte, static bool) ([]byte, error) {
		t := byte('p')
		min := 3
		max := 5

		if val, ok := args["0"]; ok {
			t = val[0]
		}

		if val, ok := args["1"]; ok {
			if s, e := strconv.Atoi(val); e == nil && s > 0 {
				min = s
			}
		}

		if val, ok := args["2"]; ok {
			if s, e := strconv.Atoi(val); e == nil && s > 0 {
				max = s
			}
		} else if _, ok := args["1"]; ok {
			max = min
		}

		if min > max {
			min, max = max, min
		}

		switch t {
		case 'p':
			return []byte(lorem.Paragraph(min, max)), nil
		case 's':
			return []byte(lorem.Sentence(min, max)), nil
		case 'w':
			return []byte(lorem.Word(min, max)), nil
		case 'e':
			return []byte(lorem.Email()), nil
		case 'h':
			return []byte(lorem.Host()), nil
		case 'u':
			return []byte(lorem.Url()), nil
		default:
			return []byte(lorem.Paragraph(min, max)), nil
		}
	}, true)

	plugins.New("lorem-d", func(args map[string]string, cont []byte, static bool) ([]byte, error) {
		t := byte('p')
		min := 3
		max := 5

		if val, ok := args["0"]; ok {
			t = val[0]
		}

		if val, ok := args["1"]; ok {
			if s, e := strconv.Atoi(val); e == nil && s > 0 {
				min = s
			}
		}

		if val, ok := args["2"]; ok {
			if s, e := strconv.Atoi(val); e == nil && s > 0 {
				max = s
			}
		} else if _, ok := args["1"]; ok {
			max = min
		}

		if min > max {
			min, max = max, min
		}

		switch t {
		case 'p':
			return []byte(lorem.Paragraph(min, max)), nil
		case 's':
			return []byte(lorem.Sentence(min, max)), nil
		case 'w':
			return []byte(lorem.Word(min, max)), nil
		case 'e':
			return []byte(lorem.Email()), nil
		case 'h':
			return []byte(lorem.Host()), nil
		case 'u':
			return []byte(lorem.Url()), nil
		default:
			return []byte(lorem.Paragraph(min, max)), nil
		}
	})

	plugins.New("embed", func(args map[string]string, cont []byte, static bool) ([]byte, error) {
		//todo: smart embed youtube videos
		return []byte(""), nil
	}, true)
}
