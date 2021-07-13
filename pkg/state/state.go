package state

import (
	"fmt"

	"github.com/spf13/afero"
	"github.com/weaveworks/reignite/pkg/defaults"
)

type Config struct {
	Root string
}

func NewFilesystem(cfg *Config, fs afero.Fs) StateProvider {
	return &fsProvider{
		fs:   fs,
		root: cfg.Root,
	}
}

type fsProvider struct {
	fs   afero.Fs
	root string
}

func (p *fsProvider) Get(vmid string) State {
	return &fsState{
		vmRoot: fmt.Sprintf("%s/%s", p.root, vmid),
		fs:     p.fs,
	}
}

type fsState struct {
	fs     afero.Fs
	vmRoot string
}

func (s *fsState) Ensure() error {
	if err := s.ensureDirectory(s.vmRoot); err != nil {
		return err
	}
	if err := s.ensureDirectory(s.VolumesRoot()); err != nil {
		return err
	}
	if err := s.ensureDirectory(s.LogsRoot()); err != nil {
		return err
	}
	if err := s.ensureDirectory(s.KernelRoot()); err != nil {
		return err
	}

	return nil
}

func (s *fsState) Exists() bool {
	exists, err := afero.DirExists(s.fs, s.vmRoot)
	if !exists || err != nil {
		return false
	}

	exists, err = afero.DirExists(s.fs, s.VolumesRoot())
	if !exists || err != nil {
		return false
	}

	exists, err = afero.DirExists(s.fs, s.LogsRoot())
	if !exists || err != nil {
		return false
	}

	exists, err = afero.DirExists(s.fs, s.KernelRoot())
	if !exists || err != nil {
		return false
	}

	return true
}

func (s *fsState) Root() string {
	return s.vmRoot
}

func (s *fsState) VolumesRoot() string {
	return fmt.Sprintf("%s/volumes", s.vmRoot)
}

func (s *fsState) PIDFile() string {
	return fmt.Sprintf("%s/pid", s.vmRoot)
}

func (s *fsState) LogsRoot() string {
	return fmt.Sprintf("%s/logs", s.vmRoot)
}

func (s *fsState) KernelRoot() string {
	return fmt.Sprintf("%s/kernel", s.vmRoot)
}

func (s *fsState) SocksFile() string {
	return fmt.Sprintf("%s/sock", s.vmRoot)
}

func (s *fsState) ensureDirectory(dir string) error {
	exists, err := afero.DirExists(s.fs, s.vmRoot)
	if err != nil {
		return fmt.Errorf("checking if state directory %s exists: %w", s.vmRoot, err)
	}
	if !exists {
		if err := s.fs.MkdirAll(s.vmRoot, defaults.STATE_DIR_PERM); err != nil {
			return fmt.Errorf("creating state dir %s: %w", s.vmRoot, err)
		}
	}

	return nil
}
