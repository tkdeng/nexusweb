package plugins

import (
	"fmt"

	"github.com/tkdeng/goutil"
)

type Plugin struct {
	cb     func(args map[string]string, cont []byte, static bool) ([]byte, error)
	static bool
}

var plugins *goutil.SyncMap[string, *Plugin] = goutil.NewMap[string, *Plugin]()

func New(name string, cb func(args map[string]string, cont []byte, static bool) ([]byte, error), static ...bool) error {
	if plugins.Has(name) {
		return fmt.Errorf("Plugin '%s' already exists", name)
	}

	s := false
	if len(static) != 0 && static[0] == true {
		s = true
	}

	plugins.Set(name, &Plugin{
		cb:     cb,
		static: s,
	})

	return nil
}

func Get(name string, static ...bool) (*Plugin, bool) {
	if plugin, ok := plugins.Get(name); ok {
		if len(static) == 0 {
			return plugin, true
		} else if static[0] == plugin.static {
			return plugin, true
		}
	}

	return &Plugin{}, false
}

func Has(name string, static ...bool) bool {
	if len(static) != 0 {
		if plugin, ok := plugins.Get(name); ok && static[0] == plugin.static {
			return true
		}
		return false
	}

	return plugins.Has(name)
}

func Remove(name string) {
	plugins.Del(name)
}

func (plugin *Plugin) Run(args map[string]string, cont []byte, static bool) ([]byte, error) {
	return plugin.cb(args, cont, static)
}

func (plugin *Plugin) Static() bool {
	return plugin.static
}
