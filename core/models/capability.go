package models

// Capabaility represents a capability of a provider.
type Capability string

const (
	// MetadataServiceCapability is a capability that indicates the microvm provider
	// has a metadata service.
	MetadataServiceCapability Capability = "metadata-service"

	// StartCapability is a capability that the microvm provider must be started separately from creation.
	// If a provider doesn't have this capability then its assumed the microvm will be started at creation.
	StartCapability Capability = "start"
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
