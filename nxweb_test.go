package nxweb

import (
	"fmt"
	"testing"
)

func Test(t *testing.T) {
	app, err := New("./test", Config{
		Port:      3000,
		DebugMode: true,
		Domains: []string{
			"localhost",
		},
		AssetsURI: "/assets",
		PublicURI: "/public",
	})

	app.Use("/test/:var1/:var2", func(c *Ctx) error {
		fmt.Println("--- r1 ^ ---")
		return c.Next()
	})

	app.Use("/test/:var1/:var2", func(c *Ctx) error {
		fmt.Println("--- r2 ^ ---")
		return nil
	})

	if err != nil {
		t.Error(err)
	}

	app.Listen()
	_ = app
}
