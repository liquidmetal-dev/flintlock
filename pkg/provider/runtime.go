package provider

import "go.uber.org/zap"

// Runetime represents the provider runtime environment.
type Runtime struct {
	Logger    *zap.SugaredLogger
	StatePath string
}
