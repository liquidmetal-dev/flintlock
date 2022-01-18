package metrics

import (
	"io"

	"github.com/urfave/cli/v2"
)

func NewApp(out io.Writer) *cli.App {
	app := cli.NewApp()

	if out != nil {
		app.Writer = out
	}

	app.Name = "flintlock-metrics"
	app.EnableBashCompletion = true
	app.Commands = commands()

	return app
}

func commands() []*cli.Command {
	return []*cli.Command{
		serveCommand(),
	}
}
