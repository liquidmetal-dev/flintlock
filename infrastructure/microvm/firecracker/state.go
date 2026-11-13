package firecracker

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/afero"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/infrastructure/microvm/shared"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
)

const apiSocketFileName = "firecracker.sock"

type State interface {
	Root() string
	SocketRoot() string

	PID() (int, error)
	PIDPath() string
	SetPid(pid int) error

	LogPath() string
	MetricsPath() string
	StdoutPath() string
	StderrPath() string
	VSockPath() string
	RelativeVSockPath() string
	SocketLinkPath() string
	legacyVSockPath() string
	APISockPath() string

	ConfigPath() string
	Config() (VmmConfig, error)
	SetConfig(cfg *VmmConfig) error

	MetadataPath() string
	Metadata() (Metadata, error)
	SetMetadata(meta *Metadata) error
}

func NewState(vmid models.VMID, stateDir, socketDir string, fs afero.Fs) State {
	return &fsState{
		stateRoot:  fmt.Sprintf("%s/%s", stateDir, vmid.String()),
		socketRoot: models.SocketRoot(socketDir, vmid),
		fs:         fs,
	}
}

type fsState struct {
	stateRoot  string
	socketRoot string
	fs         afero.Fs
}

func (s *fsState) Root() string {
	return s.stateRoot
}

func (s *fsState) SocketRoot() string {
	return s.socketRoot
}

func (s *fsState) PIDPath() string {
	return s.stateRoot + "/firecracker.pid"
}

func (s *fsState) PID() (int, error) {
	return shared.PIDReadFromFile(s.PIDPath(), s.fs)
}

func (s *fsState) LogPath() string {
	return s.stateRoot + "/firecracker.log"
}

func (s *fsState) MetricsPath() string {
	return s.stateRoot + "/firecracker.metrics"
}

func (s *fsState) StdoutPath() string {
	return s.stateRoot + "/firecracker.stdout"
}

func (s *fsState) StderrPath() string {
	return s.stateRoot + "/firecracker.stderr"
}

func (s *fsState) VSockPath() string {
	return s.socketRoot + "/" + defaults.GuestAgentVsockName
}

// socketLinkName is the name of the link in the state directory that points to
// the socket directory of the microvm.
const socketLinkName = "sock"

// SocketLinkPath returns the path of the link in the state directory that
// points to the socket directory of the microvm.
func (s *fsState) SocketLinkPath() string {
	return s.stateRoot + "/" + socketLinkName
}

// RelativeVSockPath returns the vsock socket path for the Firecracker
// configuration. It is relative to the working directory of the process (the
// state directory) and goes through the link at SocketLinkPath, so the socket
// file is still at VSockPath. Firecracker keeps this path in a snapshot, so a
// microvm restored in another state directory binds its own socket.
func (s *fsState) RelativeVSockPath() string {
	return socketLinkName + "/" + defaults.GuestAgentVsockName
}

// legacyVSockPath is where the vsock socket lived before sockets moved out of the state dir.
// Legacy fallback for #1226, to be removed in the next minor release.
func (s *fsState) legacyVSockPath() string {
	return s.stateRoot + "/" + defaults.GuestAgentVsockName
}

// APISockPath is the path of the Firecracker API socket.
func (s *fsState) APISockPath() string {
	return s.socketRoot + "/" + apiSocketFileName
}

func (s *fsState) SetPid(pid int) error {
	return shared.PIDWriteToFile(pid, s.PIDPath(), s.fs)
}

func (s *fsState) ConfigPath() string {
	return s.stateRoot + "/firecracker.cfg"
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
	err := s.writeToFileAsJSON(decodeMetadata(meta.Latest), s.MetadataPath())
	if err != nil {
		return fmt.Errorf("firecracker metadata: %w", err)
	}

	return nil
}

// decodeMetadata returns the MMDS data for the metadata of a microvm spec. It
// base64 decodes each value. A value that does not decode is kept as it is,
// in case it came without base64 encoding.
func decodeMetadata(latest map[string]string) *Metadata {
	decoded := &Metadata{
		Latest: map[string]string{},
	}

	for key, value := range latest {
		decodedValue, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			decoded.Latest[key] = value
		} else {
			decoded.Latest[key] = string(decodedValue)
		}
	}

	return decoded
}

func (s *fsState) Metadata() (Metadata, error) {
	meta := Metadata{}

	err := s.readJSONFile(&meta, s.MetadataPath())
	if err != nil {
		return Metadata{}, fmt.Errorf("firecracker metadata: %w", err)
	}

	return meta, nil
}

func (s *fsState) MetadataPath() string {
	return s.stateRoot + "/metadata.json"
}

func (s *fsState) readJSONFile(cfg interface{}, inputFile string) error {
	file, err := s.fs.Open(inputFile)
	if err != nil {
		return fmt.Errorf("opening file %s: %w", inputFile, err)
	}

	defer file.Close()

	data, err := io.ReadAll(file)
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
