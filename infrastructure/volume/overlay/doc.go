// Package overlay provisions the drives of an overlay volume: a read-only
// EROFS base shared by every microvm of one image, and a writable ext4
// disk per microvm. Service implements ports.VolumeService over them.
//
// A base is one EROFS file built from the layers of an OCI image. The
// build never unpacks the image. Each layer is converted to an EROFS file
// from its tar stream, and the layer files are merged into one image. The
// build is deterministic: two builds of one image with one profile give
// files with the same sha256.
//
// A base is keyed by the rootfs chain ID of the image and the name of the
// profile that fixes the build inputs. A writable disk is a reflink of a
// template formatted once per size. The on-host layout under the state
// root is:
//
//	bases/<sha256>.img                                 mode 0400
//	bases/by-source/<chain-id>/<profile> -> ../../<sha256>.img
//	bases/.locks/<key>.lock                            one flock per key
//	templates/rw-<size-mb>.img                         ext4, mode 0600
//	templates/.locks/rw-<size-mb>.lock                 one flock per size
//	scratch/                                           build temp
//	vm/<ns>/<name>/<uid>/volumes/root.base -> ../../../../../bases/<sha256>.img
//	vm/<ns>/<name>/<uid>/volumes/root.rw               reflink of a template
//
// The modification time of a by-source link is the last use of its base.
// The sweep removes a base that no microvm links to once that time is
// older than the retention period. Every lock is a flock, not a mutex,
// because the daemon runs more than one set of ports.
//
// The package imports no containerd code. The layers come from a
// LayerSource, which infrastructure/containerd implements.
package overlay
