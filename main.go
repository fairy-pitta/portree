package main

import (
	"os"

	"github.com/shuna/gws/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
