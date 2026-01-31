package main

import (
	"os"

	"github.com/fairy-pitta/portree/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
