//go:build linux

// flintlock-overlay-init is the init of the initrd that flintlock boots an
// overlay root volume with. See internal/overlayinit for the guest contract.
package main

import (
	"os"

	"github.com/liquidmetal-dev/flintlock/internal/overlayinit"
)

func main() {
	// The kernel connects the standard streams of /init to /dev/console, so
	// stdout is the console. Run writes one line per outcome and only returns
	// on error, with the reason already written. A non-zero exit makes the
	// kernel panic, which is the failure behaviour of the contract.
	if err := overlayinit.Run(overlayinit.NewSystem(), os.Stdout, os.Args[1:]); err != nil {
		os.Exit(1)
	}
}
