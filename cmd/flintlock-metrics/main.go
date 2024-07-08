package main

import (
	"os"

	"github.com/liquidmetal-dev/flintlock/internal/command/metrics"
	"github.com/sirupsen/logrus"
)

func main() {
	app := metrics.NewApp(os.Stdout)

	if err := app.Run(os.Args); err != nil {
		logrus.Error(err)
	}
}
