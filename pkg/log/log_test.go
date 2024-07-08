package log_test

import (
	"context"
	"io/ioutil"
	"os"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	"github.com/liquidmetal-dev/flintlock/pkg/log"
)

func TestLogger_Configure(t *testing.T) {
	RegisterTestingT(t)

	tempLogFile, err := ioutil.TempFile(os.TempDir(), "log")
	tempLogFileName := tempLogFile.Name()
	tempLogFile.Close()
	os.Remove(tempLogFile.Name())

	Expect(err).NotTo(HaveOccurred())

	testCases := []struct {
		name        string
		config      *log.Config
		expected    func(*logrus.Logger)
		expectError bool
	}{
		{
			name: "json formatter",
			config: &log.Config{
				Format: log.LogFormatJSON,
				Output: "stdout",
			},
			expected: func(l *logrus.Logger) {
				jf, ok := l.Formatter.(*logrus.JSONFormatter)
				Expect(ok).To(BeTrue())
				Expect(jf).NotTo(BeNil())
			},
			expectError: false,
		},
		{
			name: "text formatter",
			config: &log.Config{
				Format: log.LogFormatText,
				Output: "stdout",
			},
			expected: func(l *logrus.Logger) {
				jf, ok := l.Formatter.(*logrus.TextFormatter)
				Expect(ok).To(BeTrue())
				Expect(jf).NotTo(BeNil())
			},
			expectError: false,
		},
		{
			name: "invalid formatter",
			config: &log.Config{
				Format: "invalidformatter",
			},
			expected: func(l *logrus.Logger) {
			},
			expectError: true,
		},
		{
			name: "use stdout (test lowercase as well)",
			config: &log.Config{
				Format: log.LogFormatText,
				Output: "STDOUT",
			},
			expected: func(l *logrus.Logger) {
				// this isn't really a great test
				Expect(l.Out).To(BeEquivalentTo(os.Stdout))
			},
			expectError: false,
		},
		{
			name: "use stderr",
			config: &log.Config{
				Format: log.LogFormatText,
				Output: "stderr",
			},
			expected: func(l *logrus.Logger) {
				// this isn't really a great test
				Expect(l.Out).To(BeEquivalentTo(os.Stderr))
			},
			expectError: false,
		},
		{
			name: "no output",
			config: &log.Config{
				Format: log.LogFormatText,
				Output: "",
			},
			expected: func(l *logrus.Logger) {
			},
			expectError: true,
		},
		{
			name: "file output",
			config: &log.Config{
				Format: log.LogFormatText,
				Output: tempLogFile.Name(),
			},
			expected: func(l *logrus.Logger) {
				fs, err := os.Stat(tempLogFileName)
				Expect(err).NotTo(HaveOccurred())
				Expect(fs).NotTo(BeNil())
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			err := log.Configure(tc.config)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
				return
			}

			Expect(err).NotTo(HaveOccurred())

			logger := logrus.StandardLogger()
			tc.expected(logger)
		})
	}
}

func TestLogger_ConfigureVerbosity(t *testing.T) {
	RegisterTestingT(t)

	inputVerbosity := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	expectedLevels := []logrus.Level{
		logrus.InfoLevel,
		logrus.InfoLevel,
		logrus.DebugLevel,
		logrus.DebugLevel,
		logrus.DebugLevel,
		logrus.DebugLevel,
		logrus.DebugLevel,
		logrus.DebugLevel,
		logrus.DebugLevel,
		logrus.TraceLevel,
		logrus.TraceLevel,
	}

	for i, verbosity := range inputVerbosity {
		err := log.Configure(&log.Config{
			Verbosity: verbosity,
			Format:    "json",
			Output:    "stderr",
		})
		Expect(err).NotTo(HaveOccurred())

		logger := logrus.StandardLogger()

		expectedLevel := expectedLevels[i]
		Expect(expectedLevel).To(Equal(logger.Level))
	}
}

func TestLogger_Context(t *testing.T) {
	RegisterTestingT(t)

	ctx := context.Background()
	cfg := &log.Config{
		Verbosity: 1,
		Format:    "json",
		Output:    "stderr",
	}
	err := log.Configure(cfg)
	Expect(err).NotTo(HaveOccurred())

	logger := log.GetLogger(ctx)
	Expect(logger).NotTo(BeNil())

	ctx = log.WithLogger(ctx, log.GetLogger(ctx).WithField("vmid", "1234"))
	logger = log.GetLogger(ctx)
	Expect(logger).NotTo(BeNil())
	Expect(logger.Data["vmid"]).To(Equal("1234"))
}
