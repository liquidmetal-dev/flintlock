//go:build e2e
// +build e2e

package utils

import "flag"

const (
	thinpoolName     = "dev-thinpool-e2e"
	defaultProviders = "firecracker,cloudhypervisor"
)

// Params groups all param.
type Params struct {
	SkipSetupThinpool  bool
	SkipTeardown       bool
	SkipDelete         bool
	ContainerdLogLevel string
	FlintlockdLogLevel string
	ThinpoolName       string
	// ArtefactsDir is where the files of the state directory of each microvm
	// are saved to before the microvm is deleted. Nothing is saved if it is
	// empty.
	ArtefactsDir string
	// Providers are the microvm providers to run the tests with. The first one
	// is the default provider of flintlockd.
	Providers []Provider
}

// NewParams returns a new Params based on provided flags.
func NewParams() *Params {
	params := Params{}

	// The default is a constant of the tests, it can only fail to parse if the
	// constant is wrong.
	providers, err := ParseProviders(defaultProviders)
	if err != nil {
		panic(err)
	}

	params.Providers = providers

	providersUsage := "Comma separated list of the microvm providers to run the tests with " +
		"[firecracker, cloudhypervisor]. The VMM of each of them must be on the PATH " +
		"(default \"" + defaultProviders + "\")"

	flag.Func("providers", providersUsage, func(value string) error {
		providers, err := ParseProviders(value)
		if err != nil {
			return err
		}

		params.Providers = providers

		return nil
	})

	flag.BoolVar(&params.SkipSetupThinpool, "skip.setup.thinpool", false, "Skip setting up loop-backed devicemapper thinpools. Assumes existing direct-lvm setup. Must be used with -thinpool")
	flag.StringVar(&params.ThinpoolName, "thinpool", thinpoolName, "Name of thinpool to create or of existing thinpool. When existing skip.setup.thinpool should also be set")
	flag.BoolVar(&params.SkipDelete, "skip.delete", false, "Skip running the 'delete vm' step of the tests (useful for debugging, this will also leave containerd and flintlockd running)")
	flag.BoolVar(&params.SkipTeardown, "skip.teardown", false, "Do not stop containerd or flintlockd after test exit (note: will require manual cleanup)")
	flag.StringVar(&params.ContainerdLogLevel, "level.containerd", "debug", "Set containerd's log level [trace, *debug*, info, warn, error, fatal, panic]")
	flag.StringVar(&params.FlintlockdLogLevel, "level.flintlockd", "0", "Set flintlockd's log level [A level of 2 and above is debug logging. A level of 9 and above is tracing.]")

	artefactsUsage := "Directory to save the files of the state directory of each MicroVM to, " +
		"before the MicroVM is deleted. Nothing is saved if it is not set"

	flag.StringVar(&params.ArtefactsDir, "artefacts.dir", "", artefactsUsage)

	flag.Parse()

	return &params
}
