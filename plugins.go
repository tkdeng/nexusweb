package nxweb

import "github.com/tkdeng/nexusweb/plugins"

func init(){
	plugins.New("lorem", func(args map[string]string, cont []byte, static bool) ([]byte, error) {
		return []byte("Lorem ipsum dolor sit amet consectetur adipisicing elit. Voluptates ipsa ipsum nihil molestias. Aspernatur, enim rerum quis non facere architecto! Accusantium nam dolore necessitatibus iure, cum dolores ipsam atque voluptas."), nil
	}, true)
}
