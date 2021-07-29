package provider

import (
	"github.com/sirupsen/logrus"
)

// Runetime represents the provider runtime environment.
type Runtime struct {
	Logger    *logrus.Entry
	StatePath string
}
