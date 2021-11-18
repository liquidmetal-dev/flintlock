//go:build e2e
// +build e2e

package utils

import "flag"

const (
	thinpoolName = "dev-thinpool-e2e"
)

// Params groups all param.
type Params struct {
	SkipSetupThinpool  bool
	SkipTeardown       bool
	SkipDelete         bool
	ContainerdLogLevel string
	FlintlockdLogLevel string
	ThinpoolName       string
}

// NewParams returns a new Params based on provided flags.
func NewParams() *Params {
	params := Params{}

	flag.BoolVar(&params.SkipSetupThinpool, "skip.setup.thinpool", false, "Skip setting up loop-backed devicemapper thinpools. Assumes existing direct-lvm setup. Must be used with -thinpool")
	flag.StringVar(&params.ThinpoolName, "thinpool", thinpoolName, "Name of thinpool to create or of existing thinpool. When existing skip.setup.thinpool should also be set")
	flag.BoolVar(&params.SkipDelete, "skip.delete", false, "Skip running the 'delete vm' step of the tests (useful for debugging, this will also leave containerd and flintlockd running)")
	flag.BoolVar(&params.SkipTeardown, "skip.teardown", false, "Do not stop containerd or flintlockd after test exit (note: will require manual cleanup)")
	flag.StringVar(&params.ContainerdLogLevel, "level.containerd", "debug", "Set containerd's log level [trace, *debug*, info, warn, error, fatal, panic]")
	flag.StringVar(&params.FlintlockdLogLevel, "level.flintlockd", "0", "Set flintlockd's log level [A level of 2 and above is debug logging. A level of 9 and above is tracing.]")

	flag.Parse()

	return &params
}
