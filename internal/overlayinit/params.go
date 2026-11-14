package overlayinit

import (
	"fmt"
	"path"
	"strings"
)

const (
	// ContractVersion is the only guest contract version this init knows.
	ContractVersion = "1"

	// KeyContract is the kernel parameter that carries the contract version.
	KeyContract = "flintlock.root.contract"
	// KeyBase is the kernel parameter that names the read-only EROFS base device.
	KeyBase = "flintlock.root.base"
	// KeyRW is the kernel parameter that names the writable ext4 device.
	KeyRW = "flintlock.root.rw"
	// KeyInit is the kernel parameter that names the init of the image.
	KeyInit = "init"

	// DefaultInit is the init of the image when the command line has no init= parameter.
	DefaultInit = "/sbin/init"

	// BaseFSType is the filesystem type of the base device under contract version 1.
	BaseFSType = "erofs"
	// RWFSType is the filesystem type of the writable device under contract version 1.
	RWFSType = "ext4"
)

// Params holds the validated kernel parameters of the guest contract.
type Params struct {
	// Contract is the contract version.
	Contract string
	// Base is the path of the read-only EROFS base device.
	Base string
	// RW is the path of the writable ext4 device.
	RW string
	// Init is the path of the init of the image.
	Init string
}

// ParseCmdline parses a kernel command line and validates the parameters of
// the guest contract. When a key appears more than once, the last value wins.
func ParseCmdline(cmdline string) (Params, error) {
	values := map[string]string{}

	for _, token := range tokenize(cmdline) {
		key, value, found := strings.Cut(token, "=")
		if !found {
			continue
		}

		values[key] = value
	}

	params := Params{
		Contract: values[KeyContract],
		Base:     values[KeyBase],
		RW:       values[KeyRW],
		Init:     values[KeyInit],
	}

	if params.Init == "" {
		params.Init = DefaultInit
	}

	if err := params.validate(); err != nil {
		return Params{}, err
	}

	return params, nil
}

func (p Params) validate() error {
	if p.Contract == "" {
		return fmt.Errorf("%w: %s", ErrMissingParam, KeyContract)
	}

	if p.Contract != ContractVersion {
		return fmt.Errorf("%w: %s=%q, this init knows version %s",
			ErrUnknownContract, KeyContract, p.Contract, ContractVersion)
	}

	paths := []struct{ key, value string }{
		{KeyBase, p.Base},
		{KeyRW, p.RW},
		{KeyInit, p.Init},
	}

	for _, entry := range paths {
		if entry.value == "" {
			return fmt.Errorf("%w: %s", ErrMissingParam, entry.key)
		}

		if !path.IsAbs(entry.value) {
			return fmt.Errorf("%w: %s=%q is not an absolute path", ErrInvalidParam, entry.key, entry.value)
		}
	}

	if p.Base == p.RW {
		return fmt.Errorf("%w: %s and %s are the same device %q", ErrInvalidParam, KeyBase, KeyRW, p.Base)
	}

	return nil
}

// tokenize splits a kernel command line on whitespace. Double quotes group
// a value that contains spaces and are removed, as the kernel does.
func tokenize(cmdline string) []string {
	var (
		tokens  []string
		current strings.Builder
		quoted  bool
		started bool
	)

	flush := func() {
		if started {
			tokens = append(tokens, current.String())
			current.Reset()
			started = false
		}
	}

	for _, r := range cmdline {
		switch {
		case r == '"':
			quoted = !quoted
			started = true
		case !quoted && (r == ' ' || r == '\t' || r == '\n' || r == '\r'):
			flush()
		default:
			current.WriteRune(r)
			started = true
		}
	}

	flush()

	return tokens
}
