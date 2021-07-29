package log

import "github.com/spf13/pflag"

// AddFlags will add the logging flags to the supplied flagset and bind to the provided config.
func AddFlags(fs *pflag.FlagSet, config *Config) {
	fs.IntVarP(&config.Verbosity, "verbosity", "v", LogVerbosityInfo, "The verbosity level of the logging. A level of 2 and above is debug logging. A level of 9 and above is tracing.") //nolint:lll
	fs.StringVar(&config.Format, "log-format", LogFormatText, "The format of the logging output. Can be 'text' or 'json'.")
	fs.StringVar(&config.Output, "log-output", "stderr", "The output for logging. Supply a file path or one of the special values of 'stdout' and 'stderr'.") //nolint:lll
}
