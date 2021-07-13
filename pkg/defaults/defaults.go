package defaults

import (
	"io/fs"
)

const (
	DOMAIN = "works.weave.reignited"

	REIGNITED_CONF_DIR = "/etc/opt/reignited"

	STATE_DIR = "/var/lib/reignite"

	CONTAINERD_SNAPSHOTTER = "devmapper"

	KERNAL_NAME = "kernel"

	// rw-r--r--
	STATE_FILE_PERM = 0644

	STATE_DIR_PERM = fs.ModePerm

	FIRECRACKER_BIN = "firecracker"

	API_PORT = 9876

	CONTAINERD_NAMESPACE = "reignite"
)
