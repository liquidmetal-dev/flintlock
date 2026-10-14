// Package overlayinit implements the logic of flintlock-overlay-init, the
// init of the initrd that flintlock boots an overlay root volume with.
//
// The init assembles an overlayfs root from a read-only EROFS base device
// and a writable ext4 device, and then hands over to the init of the image.
// The interface between flintlockd and the init is the guest contract of
// docs/proposals/26-09-25-snapshot-restore/0204-overlay-boot-design.md,
// section 4. In short, for contract version 1:
//
//   - The kernel command line carries flintlock.root.contract=1,
//     flintlock.root.base=<device> (the EROFS base) and
//     flintlock.root.rw=<device> (the ext4 writable disk). An optional init=
//     names the init of the image; the default is /sbin/init. The init reads
//     the parameters from /proc/cmdline, because the kernel does not pass
//     keys that contain a dot to the environment of init.
//   - The init mounts devtmpfs on /dev and proc on /proc, waits up to
//     DeviceTimeout for both devices, mounts the base read-only, mounts the
//     writable disk and creates upper/ and work/ on it, mounts the overlay,
//     writes one console line that names the contract version, both devices
//     and their filesystem types, and then does the switch_root sequence:
//     move /dev and /proc into the overlay, delete the contents of the
//     initramfs, move the overlay to /, chroot and exec the init of the
//     image. It uses switch_root and not pivot_root, because the initramfs
//     rootfs cannot be unmounted on the 5.10 and 6.1 guest kernels.
//   - On any error the init writes one line with the reason to the console
//     and exits with a non-zero status. The kernel then panics. There is no
//     rescue shell.
//
// The console is standard output. The kernel opens /dev/console as the
// standard streams of /init before it runs, so the lines reach the serial
// console of the VMM and the init of the image inherits the same streams.
//
// Every syscall sits behind the System interface, so that the sequence, the
// parameter parsing and the failure paths are unit tested without a boot.
package overlayinit
