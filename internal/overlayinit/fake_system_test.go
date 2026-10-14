package overlayinit

import (
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"
)

// fakeSystem records every call in order and answers from its tables.
type fakeSystem struct {
	calls []string
	// files holds the contents ReadFile returns, by path.
	files map[string]string
	// blockDevices holds the time at which each block device appears.
	blockDevices map[string]time.Time
	// charDevices holds paths that exist as character devices.
	charDevices map[string]bool
	// fail maps a call prefix, as recorded in calls, to the error to return.
	fail map[string]error
	env  []string
	now  time.Time
}

func newFakeSystem(cmdline string) *fakeSystem {
	start := time.Date(2026, time.October, 14, 12, 0, 0, 0, time.UTC)

	return &fakeSystem{
		files: map[string]string{cmdlinePath: cmdline},
		blockDevices: map[string]time.Time{
			"/dev/vda": start,
			"/dev/vdb": start,
		},
		charDevices: map[string]bool{},
		fail:        map[string]error{},
		env:         []string{"HOME=/", "TERM=linux"},
		now:         start,
	}
}

func (f *fakeSystem) record(format string, args ...any) error {
	call := fmt.Sprintf(format, args...)
	f.calls = append(f.calls, call)

	for prefix, err := range f.fail {
		if strings.HasPrefix(call, prefix) {
			return err
		}
	}

	return nil
}

func (f *fakeSystem) ReadFile(path string) ([]byte, error) {
	if err := f.record("readfile %s", path); err != nil {
		return nil, err
	}

	contents, ok := f.files[path]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: path, Err: fs.ErrNotExist}
	}

	return []byte(contents), nil
}

func (f *fakeSystem) Stat(path string) (os.FileInfo, error) {
	if err := f.record("stat %s", path); err != nil {
		return nil, err
	}

	if appears, ok := f.blockDevices[path]; ok && !f.now.Before(appears) {
		return fakeFileInfo{name: path, mode: os.ModeDevice}, nil
	}

	if f.charDevices[path] {
		return fakeFileInfo{name: path, mode: os.ModeDevice | os.ModeCharDevice}, nil
	}

	return nil, &fs.PathError{Op: "stat", Path: path, Err: fs.ErrNotExist}
}

func (f *fakeSystem) MkdirAll(path string, perm os.FileMode) error {
	return f.record("mkdir %s %o", path, perm)
}

func (f *fakeSystem) Mount(source, target, fstype string, flags uintptr, data string) error {
	return f.record("mount %s %s %s %#x %q", source, target, fstype, flags, data)
}

func (f *fakeSystem) Chdir(path string) error {
	return f.record("chdir %s", path)
}

func (f *fakeSystem) Chroot(path string) error {
	return f.record("chroot %s", path)
}

func (f *fakeSystem) RemoveContents(dir string) error {
	return f.record("removecontents %s", dir)
}

func (f *fakeSystem) Environ() []string {
	return f.env
}

func (f *fakeSystem) Exec(path string, argv, env []string) error {
	return f.record("exec %s %v %v", path, argv, env)
}

func (f *fakeSystem) Now() time.Time {
	return f.now
}

func (f *fakeSystem) Sleep(d time.Duration) {
	_ = f.record("sleep %s", d)
	f.now = f.now.Add(d)
}

type fakeFileInfo struct {
	name string
	mode os.FileMode
}

func (i fakeFileInfo) Name() string       { return i.name }
func (i fakeFileInfo) Size() int64        { return 0 }
func (i fakeFileInfo) Mode() os.FileMode  { return i.mode }
func (i fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (i fakeFileInfo) IsDir() bool        { return false }
func (i fakeFileInfo) Sys() any           { return nil }
