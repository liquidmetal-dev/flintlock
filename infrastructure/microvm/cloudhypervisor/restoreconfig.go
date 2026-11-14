package cloudhypervisor

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
)

const (
	consoleModeFile   = "File"
	consoleModeSocket = "Socket"
)

var errSnapshotDeviceMismatch = errors.New("snapshot devices do not match the microvm")

// restoreConfigPaths are the host paths of the devices of a restored microvm.
type restoreConfigPaths struct {
	// Disks are the paths of the drives, in the order the guest sees them.
	Disks []string
	// Taps are the TAP devices of the network interfaces, in the order of the
	// spec.
	Taps []string
	// FSSockets are the virtiofs sockets, in the order of the spec.
	FSSockets []string
	// VSock is the vsock socket, or empty if the microvm has no vsock.
	VSock string
	// StateRoot is the state directory of the microvm.
	StateRoot string
	// SocketRoot is the socket directory of the microvm.
	SocketRoot string
}

type jsonObject map[string]json.RawMessage

// rewriteRestoreConfig returns the config.json of a cloud-hypervisor snapshot
// with the host paths of its devices changed to the ones of the restored
// microvm. Every other field is kept as it is, so the config stays valid for
// any version of cloud-hypervisor.
func rewriteRestoreConfig(data []byte, paths restoreConfigPaths) ([]byte, error) {
	config := jsonObject{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if err := rewriteList(config, "disks", "path", paths.Disks); err != nil {
		return nil, err
	}

	if err := rewriteList(config, "net", "tap", paths.Taps); err != nil {
		return nil, err
	}

	if err := rewriteList(config, "fs", "socket", paths.FSSockets); err != nil {
		return nil, err
	}

	if err := rewriteVSock(config, paths.VSock); err != nil {
		return nil, err
	}

	for _, key := range []string{"serial", "console", "debug_console"} {
		if err := rewriteConsole(config, key, paths); err != nil {
			return nil, err
		}
	}

	return json.Marshal(config)
}

// rewriteList sets field of each object in the list at key to the value at
// the same index of values.
func rewriteList(config jsonObject, key, field string, values []string) error {
	var items []jsonObject
	if err := getField(config, key, &items); err != nil {
		return err
	}

	if len(items) != len(values) {
		return fmt.Errorf("%w: %d %s in the snapshot, %d in the microvm",
			errSnapshotDeviceMismatch, len(items), key, len(values))
	}

	if len(items) == 0 {
		return nil
	}

	for i := range items {
		if err := setField(items[i], field, values[i]); err != nil {
			return err
		}
	}

	return setField(config, key, items)
}

func rewriteVSock(config jsonObject, socket string) error {
	var vsock jsonObject
	if err := getField(config, "vsock", &vsock); err != nil {
		return err
	}

	if (vsock != nil) != (socket != "") {
		return fmt.Errorf("%w: vsock in the snapshot is %t, in the microvm is %t",
			errSnapshotDeviceMismatch, vsock != nil, socket != "")
	}

	if vsock == nil {
		return nil
	}

	if err := setField(vsock, "socket", socket); err != nil {
		return err
	}

	return setField(config, "vsock", vsock)
}

// rewriteConsole moves the output file or socket of a console device to the
// directories of the restored microvm, keeping its name.
func rewriteConsole(config jsonObject, key string, paths restoreConfigPaths) error {
	var console jsonObject
	if err := getField(config, key, &console); err != nil {
		return err
	}

	if console == nil {
		return nil
	}

	var mode string
	if err := getField(console, "mode", &mode); err != nil {
		return err
	}

	var field, dir string

	switch mode {
	case consoleModeFile:
		field, dir = "file", paths.StateRoot
	case consoleModeSocket:
		field, dir = "socket", paths.SocketRoot
	default:
		return nil
	}

	var path string
	if err := getField(console, field, &path); err != nil {
		return err
	}

	if path == "" {
		return nil
	}

	if err := setField(console, field, filepath.Join(dir, filepath.Base(path))); err != nil {
		return err
	}

	return setField(config, key, console)
}

// getField decodes the field key of obj into dest. A missing or null field
// leaves dest unchanged.
func getField(obj jsonObject, key string, dest any) error {
	raw, ok := obj[key]
	if !ok {
		return nil
	}

	if err := json.Unmarshal(raw, dest); err != nil {
		return fmt.Errorf("parsing %s: %w", key, err)
	}

	return nil
}

func setField(obj jsonObject, key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encoding %s: %w", key, err)
	}

	obj[key] = raw

	return nil
}
