package models

// Capabaility represents a capability of a provider.
type Capability string

const (
	// MetadataServiceCapability is a capability that indicates the microvm provider
	// has a metadata service.
	MetadataServiceCapability Capability = "metadata-service"

	// AutoStartCapability is a capability of the microvm provider where the vm is automatically started
	// as part of the creation process. If a provider doesn't have this capability then its assumed the
	// microvm will be started via a call to the start implementation of the provider.
	AutoStartCapability Capability = "auto-start"
)

// Capabilities represents a list of capabilities.
type Capabilities []Capability

// Has is used to test if this set of capabilities has a specific capability.
func (cp Capabilities) Has(hasCap Capability) bool {
	for _, c := range cp {
		if c == hasCap {
			return true
		}
	}

	return false
}
