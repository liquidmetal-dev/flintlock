package firecracker

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/spf13/afero"
	"github.com/weaveworks-liquidmetal/flintlock/pkg/defaults"
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

	ConfigPath() string
	Config() (VmmConfig, error)
	SetConfig(cfg *VmmConfig) error

	MetadataPath() string
	CloudInitImage() string
	AdditionalMetadata() string

	Metadata() (Metadata, error)
	SetMetadata(meta *Metadata) error

	SocketPath() string
}

func NewState(stateDir string, fs afero.Fs) State {
	return &fsState{
		stateRoot: stateDir,
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

func (s *fsState) SocketPath() string {
	return fmt.Sprintf("%s/firecracker.sock", s.stateRoot)
}

func (s *fsState) SetPid(pid int) error {
	return s.pidWriteToFile(pid, s.PIDPath())
}

func (s *fsState) ConfigPath() string {
	return fmt.Sprintf("%s/firecracker.cfg", s.stateRoot)
}

func (s *fsState) Config() (VmmConfig, error) {
	cfg := VmmConfig{}

	err := s.readJSONFile(&cfg, s.ConfigPath())
	if err != nil {
		return VmmConfig{}, fmt.Errorf("firecracker config: %w", err)
	}

	return cfg, nil
}

func (s *fsState) SetConfig(cfg *VmmConfig) error {
	err := s.writeToFileAsJSON(cfg, s.ConfigPath())
	if err != nil {
		return fmt.Errorf("firecracker config: %w", err)
	}

	return nil
}

func (s *fsState) SetMetadata(meta *Metadata) error {
	decoded := &Metadata{
		Latest: map[string]string{},
	}

	// Try to base64 decode values, if we can't decode, use them at they are,
	// in case we got them without base64 encoding.
	for key, value := range meta.Latest {
		decodedValue, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			decoded.Latest[key] = value
		} else {
			decoded.Latest[key] = string(decodedValue)
		}
	}

	err := s.writeToFileAsJSON(decoded, s.MetadataPath())
	if err != nil {
		return fmt.Errorf("firecracker metadata: %w", err)
	}

	return nil
}

func (s *fsState) Metadata() (Metadata, error) {
	meta := Metadata{}

	err := s.readJSONFile(&meta, s.ConfigPath())
	if err != nil {
		return Metadata{}, fmt.Errorf("firecracker metadata: %w", err)
	}

	return meta, nil
}

func (s *fsState) MetadataPath() string {
	return fmt.Sprintf("%s/metadata.json", s.stateRoot)
}

func (s *fsState) CloudInitImage() string {
	return fmt.Sprintf("%s/cloud-init.img", s.stateRoot)
}

func (s *fsState) AdditionalMetadata() string {
	return fmt.Sprintf("%s/data.img", s.stateRoot)
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
	if err != nil {
		return fmt.Errorf("opening pid file %s: %w", pidFile, err)
	}

	defer file.Close()

	_, err = fmt.Fprintf(file, "%d", pid)
	if err != nil {
		return fmt.Errorf("writing pid %d to file %s: %w", pid, pidFile, err)
	}

	return nil
}

func (s *fsState) readJSONFile(cfg interface{}, inputFile string) error {
	file, err := s.fs.Open(inputFile)
	if err != nil {
		return fmt.Errorf("opening file %s: %w", inputFile, err)
	}

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return fmt.Errorf("reading file %s: %w", inputFile, err)
	}

	err = json.Unmarshal(data, cfg)
	if err != nil {
		return fmt.Errorf("unmarshalling: %w", err)
	}

	return nil
}

func (s *fsState) writeToFileAsJSON(cfg interface{}, outputFilePath string) error {
	data, err := json.MarshalIndent(cfg, "", " ")
	if err != nil {
		return fmt.Errorf("marshalling: %w", err)
	}

	file, err := s.fs.OpenFile(outputFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, defaults.DataFilePerm)
	if err != nil {
		return fmt.Errorf("opening output file %s: %w", outputFilePath, err)
	}

	defer file.Close()

	_, err = file.Write(data)
	if err != nil {
		return fmt.Errorf("writing output file %s: %w", outputFilePath, err)
	}

	return nil
}
