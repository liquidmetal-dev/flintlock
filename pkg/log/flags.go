package log

import (
	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"
)

// AddFlagsToApp will add the logging flags to the supplied app and bind to the provided config.
func AddFlagsToApp(app *cli.App, config *Config) {
	// A level of 2 and above is debug logging. A level of 9 and above is tracing.
	verbosityFlag := altsrc.NewIntFlag(&cli.IntFlag{
		Name:    "verbosity",
		Aliases: []string{"v"},
		Usage: "The verbosity level of the logging. A level of 2 and above is debug logging. " +
			"A level of 9 and above is tracing.",
		Value:       config.Verbosity,
		Destination: &config.Verbosity,
	})

	formatFlag := altsrc.NewStringFlag(&cli.StringFlag{
		Name:        "format",
		Usage:       "The format to use for logging. Valid values are 'text' and 'json'",
		DefaultText: "text",
		Value:       LogFormatText,
		Destination: &config.Format,
	})

	outputFlag := altsrc.NewStringFlag(&cli.StringFlag{
		Name:        "output",
		Usage:       "Logging output",
		DefaultText: "stderr",
		Value:       "stderr",
		Destination: &config.Output,
	})

	app.Flags = append(app.Flags, verbosityFlag, formatFlag, outputFlag)
}
