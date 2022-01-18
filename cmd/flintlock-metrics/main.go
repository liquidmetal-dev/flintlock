package main

import (
	"os"

	"github.com/sirupsen/logrus"

	"github.com/weaveworks/flintlock/internal/command/metrics"
)

func main() {
	app := metrics.NewApp(os.Stdout)

	if err := app.Run(os.Args); err != nil {
		logrus.Error(err)
	}
}
