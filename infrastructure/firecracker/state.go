package firecracker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/spf13/afero"

	"github.com/weaveworks/reignite/core/models"
	"github.com/weaveworks/reignite/pkg/defaults"
)

type State interface {
	Root() string

	PID() (int, error)
	PIDPath() string
	SetPid(pid int) error

	LogPath() string
	MetricsPath() string
	StdoutPath() string
	StderrPath() string
	SockPath() string

	ConfigPath() string
	Config() (*VmmConfig, error)
	SetConfig(cfg *VmmConfig) error
}

func NewState(vmid models.VMID, stateDir string, fs afero.Fs) State {
	return &fsState{
		stateRoot: fmt.Sprintf("%s/%s", stateDir, vmid),
		fs:        fs,
	}
}

type fsState struct {
	stateRoot string
	fs        afero.Fs
}

func (s *fsState) Root() string {
	return s.stateRoot
}

func (s *fsState) PIDPath() string {
	return fmt.Sprintf("%s/firecracker.pid", s.stateRoot)
}

func (s *fsState) PID() (int, error) {
	return s.pidReadFromFile(s.PIDPath())
}

func (s *fsState) LogPath() string {
	return fmt.Sprintf("%s/firecracker.log", s.stateRoot)
}

func (s *fsState) MetricsPath() string {
	return fmt.Sprintf("%s/firecracker.metrics", s.stateRoot)
}

func (s *fsState) StdoutPath() string {
	return fmt.Sprintf("%s/firecracker.stdout", s.stateRoot)
}

func (s *fsState) StderrPath() string {
	return fmt.Sprintf("%s/firecracker.stderr", s.stateRoot)
}

func (s *fsState) SockPath() string {
	return fmt.Sprintf("%s/firecracker.sock", s.stateRoot)
}

func (s *fsState) SetPid(pid int) error {
	return s.pidWriteToFile(pid, s.PIDPath())
}

func (s *fsState) ConfigPath() string {
	return fmt.Sprintf("%s/firecracker.cfg", s.stateRoot)
}

func (s *fsState) Config() (*VmmConfig, error) {
	return s.cfgReadFromFile(s.ConfigPath())
}

func (s *fsState) SetConfig(cfg *VmmConfig) error {
	return s.cfgWriteToFile(cfg, s.ConfigPath())
}

func (s *fsState) pidReadFromFile(pidFile string) (int, error) {
	file, err := s.fs.Open(pidFile)
	if err != nil {
		return -1, fmt.Errorf("opening pid file %s: %w", pidFile, err)
	}

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return -1, fmt.Errorf("reading pid file %s: %w", pidFile, err)
	}

	pid, err := strconv.Atoi(string(bytes.TrimSpace(data)))
	if err != nil {
		return -1, fmt.Errorf("converting data to int: %w", err)
	}

	return pid, nil
}

func (s *fsState) pidWriteToFile(pid int, pidFile string) error {
	file, err := s.fs.OpenFile(pidFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, defaults.DataFilePerm)
	defer file.Close() //nolint: staticcheck
	if err != nil {
		return fmt.Errorf("opening pid file %s: %w", pidFile, err)
	}

	_, err = fmt.Fprintf(file, "%d", pid)
	if err != nil {
		return fmt.Errorf("writing pid %d to file %s: %w", pid, pidFile, err)
	}

	return nil
}

func (s *fsState) cfgReadFromFile(cfgFile string) (*VmmConfig, error) {
	file, err := s.fs.Open(cfgFile)
	if err != nil {
		return nil, fmt.Errorf("opening firecracker config file %s: %w", cfgFile, err)
	}

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("reading firecracker config file %s: %w", cfgFile, err)
	}

	cfg := &VmmConfig{}
	err = json.Unmarshal(data, cfg)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling firecracker config: %w", err)
	}

	return cfg, nil
}

func (s *fsState) cfgWriteToFile(cfg *VmmConfig, cfgFile string) error {
	data, err := json.MarshalIndent(cfg, "", " ")
	if err != nil {
		return fmt.Errorf("marshalling firecracker config: %w", err)
	}

	file, err := s.fs.OpenFile(cfgFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, defaults.DataFilePerm)
	defer file.Close() //nolint: staticcheck
	if err != nil {
		return fmt.Errorf("opening firecracker config file %s: %w", cfgFile, err)
	}

	_, err = file.Write(data)
	if err != nil {
		return fmt.Errorf("writing firecracker cfg to file %s: %w", cfgFile, err)
	}

	return nil
}
