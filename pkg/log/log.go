package log

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/weaveworks/reignite/pkg/defaults"
)

const (
	// LOGGER_KEY is the key to lookup the logger in the context.
	LOGGER_KEY = "reignited.logger"
)

const (
	// LogVerbosityInfo is the verbosity level for info logging.
	LogVerbosityInfo = 0
	// LogVerbosityDebug is the verbosity level for debug logging.
	LogVerbosityDebug = 2
	// LogVerbosityTrace is the verbosity level for trace logging.
	LogVerbosityTrace = 9
)

const (
	// LogFormatText specifies a textual log format.
	LogFormatText = "text"
	// LogFormatJSON specifies a JSON log format.
	LogFormatJSON = "json"
)

// Config represents the configuration settings for a logger.
type Config struct {
	// Verbosity specifies the logging verbosity level.
	Verbosity int
	// Format specifies the logging output format.
	Format string
	// Output specifies the destination for logging. You can specify the special
	// values of 'stderr' or 'stdout' or a file path.
	Output string
}

// Configure will configure the logger from the supplied config.
func Configure(logConfig *Config) error {
	if logConfig.Format == LogFormatJSON {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logrus.SetFormatter(&logrus.TextFormatter{})
	}

	logrus.SetLevel(logrus.InfoLevel)
	if logConfig.Verbosity >= LogVerbosityDebug && logConfig.Verbosity < LogVerbosityTrace {
		logrus.SetLevel(logrus.DebugLevel)
	} else if logConfig.Verbosity >= LogVerbosityTrace {
		logrus.SetLevel(logrus.TraceLevel)
	}

	output := strings.ToLower(logConfig.Output)
	switch output {
	case "stdin":
		logrus.SetOutput(os.Stdout)
	case "stderr":
		logrus.SetOutput(os.Stderr)
	default:
		file, err := os.OpenFile(output, os.O_CREATE|os.O_APPEND, defaults.STATE_DIR_PERM.Perm())
		if err != nil {
			return fmt.Errorf("opening log file %s: %w", output, err)
		}
		logrus.SetOutput(file)
	}

	return nil

}

// WithLogger is used to attached a logger to a specific context.
func WithLogger(ctx context.Context, logger *logrus.Entry) context.Context {
	return context.WithValue(ctx, LOGGER_KEY, logger)
}

// GetLogger will get a logger from the supplied context for create a new logger.
func GetLogger(ctx context.Context) *logrus.Entry {
	logger := ctx.Value(LOGGER_KEY)

	if logger == nil {
		return logrus.NewEntry(logrus.StandardLogger())
	}

	return logger.(*logrus.Entry)
}
