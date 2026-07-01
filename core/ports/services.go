package ports

import (
	"context"
	"time"

	mvmv1 "github.com/liquidmetal-dev/flintlock/api/services/microvm/v1alpha1"
	"github.com/liquidmetal-dev/flintlock/core/models"
)

// MicroVMService is the port definition for a microvm service.
type MicroVMService interface {
	// Capabilities returns a list of the capabilities the provider supports.
	Capabilities() models.Capabilities

	// Create will create a new microvm.
	Create(ctx context.Context, vm *models.MicroVM) error
	// Delete will delete a VM and its runtime state.
	Delete(ctx context.Context, id string) error
	// Start will start a created microvm.
	Start(ctx context.Context, vm *models.MicroVM) error
	// State returns the state of a microvm.
	State(ctx context.Context, id string) (MicroVMState, error)
	// Metrics returns with the metrics of a microvm.
	Metrics(ctx context.Context, id models.VMID) (MachineMetrics, error)
	// Snapshot pauses a running microvm, captures a point-in-time snapshot to
	// disk and resumes it, returning the paths to the raw snapshot artifacts.
	Snapshot(ctx context.Context, input SnapshotInput) (*SnapshotResult, error)
}

// SnapshotArtifactKind identifies the role of a file produced by a snapshot.
type SnapshotArtifactKind string

const (
	// SnapshotMemory is the guest memory file.
	SnapshotMemory SnapshotArtifactKind = "memory"
	// SnapshotState is the VMM state/device file.
	SnapshotState SnapshotArtifactKind = "state"
	// SnapshotConfig is an additional VMM config file (cloud-hypervisor config.json).
	SnapshotConfig SnapshotArtifactKind = "config"
)

// SnapshotInput is the input for taking a microvm snapshot.
type SnapshotInput struct {
	// VMID identifies the microvm to snapshot.
	VMID models.VMID
}

// SnapshotArtifact is a single file produced by a snapshot.
type SnapshotArtifact struct {
	// Kind is the role of the artifact.
	Kind SnapshotArtifactKind
	// Path is the filesystem path to the artifact.
	Path string
}

// SnapshotResult is the result of taking a microvm snapshot.
type SnapshotResult struct {
	// Artifacts are the raw files produced by the snapshot.
	Artifacts []SnapshotArtifact
	// Directory is the scratch directory containing the artifacts.
	Directory string
}

// SnapshotPackager is a port for a service that packages snapshot artifacts
// into an OCI image.
type SnapshotPackager interface {
	// Build packages the snapshot artifacts and the microvm spec into an OCI
	// image at the given reference and returns the resulting image details.
	Build(ctx context.Context, input SnapshotPackageInput) (*SnapshotImage, error)
}

// SnapshotPackageInput is the input for packaging a snapshot into an OCI image.
type SnapshotPackageInput struct {
	// Reference is the full OCI image reference to package the snapshot into.
	Reference string
	// Artifacts are the raw snapshot files to include as layers.
	Artifacts []SnapshotArtifact
	// Spec is the microvm spec, included as a config blob so the image is
	// self-describing.
	Spec *models.MicroVM
}

// SnapshotImage describes a packaged snapshot OCI image.
type SnapshotImage struct {
	// Reference is the OCI image reference the snapshot was packaged into.
	Reference string
	// Digest is the manifest digest of the packaged image.
	Digest string
}

// This state represents the state of the Firecracker MVM process itself
// The state for the entire Flintlock MVM is represented in models.MicroVMState.
type MicroVMState string

// MachineMetrics is a metrics interface for providers.
type MachineMetrics interface {
	ToPrometheus() []byte
}

const (
	MicroVMStateUnknown    MicroVMState = "unknown"
	MicroVMStatePending    MicroVMState = "pending"
	MicroVMStateConfigured MicroVMState = "configured"
	MicroVMStateRunning    MicroVMState = "running"
)

// MicroVMGRPCService is a port for a microvm grpc service.
type MicroVMGRPCService interface {
	mvmv1.MicroVMServer
}

// IDService is a port for a service for working with identifiers.
type IDService interface {
	// GenerateRandom will create a random identifier.
	GenerateRandom() (string, error)
}

// EventService is a port for a service that acts as a event bus.
type EventService interface {
	// Publish will publish an event to a specific topic.
	Publish(ctx context.Context, topic string, eventToPublish interface{}) error
	// SubscribeTopic will subscribe to events on a named topic..
	SubscribeTopic(ctx context.Context, topic string) (ch <-chan *EventEnvelope, errs <-chan error)
	// SubscribeTopics will subscribe to events on a set of named topics.
	SubscribeTopics(ctx context.Context, topics []string) (ch <-chan *EventEnvelope, errs <-chan error)
	// Subscribe will subscribe to events on all topics
	Subscribe(ctx context.Context) (ch <-chan *EventEnvelope, errs <-chan error)
}

type EventEnvelope struct {
	Timestamp time.Time
	Namespace string
	Topic     string
	Event     interface{}
}

// ImageService is a port for a service that interacts with OCI images.
type ImageService interface {
	// Pull will get (i.e. pull) the image for a specific owner.
	Pull(ctx context.Context, input *ImageSpec) error
	// PullAndMount will get (i.e. pull) the image for a specific owner and then
	// make it available via a mount point.
	PullAndMount(ctx context.Context, input *ImageMountSpec) ([]models.Mount, error)
	// Exists checks if the image already exists on the machine.
	Exists(ctx context.Context, input *ImageSpec) (bool, error)
	// IsMounted checks if the image is pulled and mounted.
	IsMounted(ctx context.Context, input *ImageMountSpec) (bool, error)
}

type ImageSpec struct {
	// ImageName is the name of the image to get.
	ImageName string
	// Owner is the name of the owner of the image.
	Owner string
}

// ImageMountSpec is the declaration of an image that needs to be pulled and mounted.
type ImageMountSpec struct {
	// ImageName is the name of the image to get.
	ImageName string
	// Owner is the name of the owner of the image.
	Owner string
	// Use is an indicator of what the image will be used for.
	Use models.ImageUse
	// OwnerUsageID is an identifier from the owner.
	OwnerUsageID string
}

// NetworkService is a port for a service that interacts with the network
// stack on the host machine.
type NetworkService interface {
	// IfaceCreate will create the network interface.
	IfaceCreate(ctx context.Context, input IfaceCreateInput) (*IfaceDetails, error)
	// IfaceDelete is used to delete a network interface
	IfaceDelete(ctx context.Context, input DeleteIfaceInput) error
	// IfaceExists will check if an interface with the given name exists
	IfaceExists(ctx context.Context, name string) (bool, error)
	// IfaceDetails will get the details of the supplied network interface.
	IfaceDetails(ctx context.Context, name string) (*IfaceDetails, error)
}

type IfaceCreateInput struct {
	// DeviceName is the name of the network interface to create on the host.
	DeviceName string
	// Type is the type of network interface to create.
	Type models.IfaceType
	// MAC allows the specifying of a specific MAC address to use for the interface. If
	// not supplied a autogenerated MAC address will be used.
	MAC string
	// Attach indicates if this device should be attached to the parent bridge. Only applicable to TAP devices.
	Attach bool
	// BridgeName is the name of the bridge to attach to. Only if this is a tap device and attach is true.
	BridgeName string
}

type IfaceDetails struct {
	// DeviceName is the name of the network interface created on the host.
	DeviceName string
	// Type is the type of network interface created.
	Type models.IfaceType
	// MAC is the MAC address of the created interface.
	MAC string
	// Index is the network interface index on the host.
	Index int
}

type DeleteIfaceInput struct {
	// DeviceName is the name of the network interface to delete from the host.
	DeviceName string
}

// DiskService is a port for a service that creates disk images.
type DiskService interface {
	// Create will create a new disk.
	Create(ctx context.Context, input DiskCreateInput) error
}

// DiskType represents the type of disk.
type DiskType int

const (
	// DiskTypeFat32 is a FAT32 compatible filesystem.
	DiskTypeFat32 DiskType = iota
	// DiskTypeISO9660 is an iso filesystem.
	DiskTypeISO9660
)

// DiskCreateInput are the input options for creating a disk.
type DiskCreateInput struct {
	// Path is the filesystem path of where to create the disk.
	Path string
	// Size is how big the disk should be. It uses human readable formats
	// such as 8Mb, 10Kb.
	Size string
	// VolumeName is the name to give to the volume.
	VolumeName string
	// Type is the type of disk to create.
	Type DiskType
	// Files are the files to create in the new disk.
	Files []DiskFile
	// Overwrite specifies if the image file already exists whether
	// we should overwrite it or return an error.
	Overwrite bool
}

// DiskFile represents a file to create in a disk.
type DiskFile struct {
	// Path is the path in the disk image for the file.
	Path string
	// ContentBase64 is the content of the file encoded as base64.
	ContentBase64 string
}

// Create VirtioFSInput are the input options for creating a disk.
type VirtioFSCreateInput struct {
	Path string
}

type VirtioFSDeleteInput struct {
	Path string
}

// VirtiofsService is the port definition for a VirtioFS service.
type VirtioFSService interface {
	// Create will create a new virtiofs share.
	Create(ctx context.Context, vmid *models.VMID, input VirtioFSCreateInput) (*models.Mount, error)
	Delete(ctx context.Context, vmid *models.VMID) error
	HasVirtioFSDProcess(ctx context.Context, vmid *models.VMID) (bool, error)
}
