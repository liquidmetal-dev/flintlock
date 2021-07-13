package main

import (
	"log"

	"github.com/weaveworks/reignite/internal/command"
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
