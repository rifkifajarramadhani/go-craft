package main

import (
	"os"

	"github.com/rifkifajarramadhani/go-craft/internal/cli"
)

func main() {
	if err := cli.NewRootCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
