package overlayinit

import (
	"os"
	"time"
)

// Mount flags of the Linux mount(2) ABI. They are declared here, and not
// taken from golang.org/x/sys/unix, so that the logic of this package and
// its tests compile on every platform.
const (
	msRdonly uintptr = 0x1
	msNosuid uintptr = 0x2
	msNodev  uintptr = 0x4
	msNoexec uintptr = 0x8
	msMove   uintptr = 0x2000
)

// System is the set of syscalls the init needs. The Linux implementation is
// NewSystem. The tests use a fake, so the sequence and the failure paths are
// tested without a boot.
type System interface {
	// ReadFile returns the contents of the file at path.
	ReadFile(path string) ([]byte, error)
	// Stat returns the information of the file at path, following symlinks.
	Stat(path string) (os.FileInfo, error)
	// MkdirAll creates the directory at path and its parents if they are absent.
	MkdirAll(path string, perm os.FileMode) error
	// Mount calls mount(2).
	Mount(source, target, fstype string, flags uintptr, data string) error
	// Chdir changes the working directory of the process.
	Chdir(path string) error
	// Chroot changes the root directory of the process.
	Chroot(path string) error
	// RemoveContents deletes everything under dir without crossing a mount
	// point. It leaves dir itself in place.
	RemoveContents(dir string) error
	// Environ returns the environment of the process.
	Environ() []string
	// Exec replaces the process with the program at path. It only returns on error.
	Exec(path string, argv, env []string) error
	// Now returns the current time.
	Now() time.Time
	// Sleep pauses the process for d.
	Sleep(d time.Duration)
}
