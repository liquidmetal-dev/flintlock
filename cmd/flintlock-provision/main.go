package main

import (
	"log"

	"github.com/liquidmetal-dev/flintlock/internal/command/provision"
)

func main() {
	rootCmd, err := provision.NewRootCommand()
	if err != nil {
		log.Fatalln(err)
	}

	if err := rootCmd.Execute(); err != nil {
		log.Fatalln(err)
	}
}
