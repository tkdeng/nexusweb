package compiler

import (
	"fmt"
)

// PrintMsg prints to console and auto inserts spaces
func PrintMsg(color string, msg string) {
	if color == "none" {
		color = "0"
	} else if color == "error" {
		color = "0;31"
	} else if color == "confirm" {
		color = "0;32"
	} else if color == "warn" {
		color = "0;33"
	} else if color == "info" {
		color = "0;34"
	} else if color == "value" {
		color = "0;35"
	}

	fmt.Println("\r\x1b[" + color + "m" + msg + "\x1b[0m")
}
