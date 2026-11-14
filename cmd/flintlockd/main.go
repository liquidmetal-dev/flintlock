package main

import (
	"log"

	"github.com/liquidmetal-dev/flintlock/internal/command"
)

func main() {
	rootCmd, err := command.NewRootCommand()
	if err != nil {
		log.Fatalln(err)
	}

	if err := rootCmd.Execute(); err != nil {
		log.Fatalln(err)
	}
}
