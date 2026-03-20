package nxweb

import "testing"

func Test(t *testing.T) {
	app, err := New("./test", Config{
		Port: 3000,
	})

	if err != nil {
		t.Error(err)
	}

	app.Listen()
	_ = app
}
