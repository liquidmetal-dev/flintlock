// Package overlay builds the read-only EROFS base of an overlay volume.
//
// A base is one EROFS file built from the layers of an OCI image. The
// build never unpacks the image. Each layer is converted to an EROFS file
// from its tar stream, and the layer files are merged into one image. The
// build is deterministic: two builds of one image with one profile give
// files with the same sha256.
//
// A base is keyed by the rootfs chain ID of the image and the name of the
// profile that fixes the build inputs. The on-host layout under the state
// root is:
//
//	bases/<sha256>.img                                 mode 0400
//	bases/by-source/<chain-id>/<profile> -> ../../<sha256>.img
//	bases/.locks/<key>.lock                            one flock per key
//	scratch/base-<random>/                             build temp
//	vm/<ns>/<name>/<uid>/volumes/root.base -> ../../../../../bases/<sha256>.img
//
// The package imports no containerd code. The layers come from a
// LayerSource, which infrastructure/containerd implements.
package overlay
