package firecracker

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/infrastructure/microvm/shared"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
	"github.com/spf13/afero"
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
	Metadata() (Metadata, error)
	SetMetadata(meta *Metadata) error
}

func NewState(vmid models.VMID, stateDir string, fs afero.Fs) State {
	return &fsState{
		stateRoot: fmt.Sprintf("%s/%s", stateDir, vmid.String()),
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
	return shared.PIDReadFromFile(s.PIDPath(), s.fs)
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

func (s *fsState) SetPid(pid int) error {
	return shared.PIDWriteToFile(pid, s.PIDPath(), s.fs)
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
