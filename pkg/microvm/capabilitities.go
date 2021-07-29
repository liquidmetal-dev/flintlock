package microvm

// Capabaility represents a capability of a provider.
type Capability string

const (
	// MetadataServiceCapability is a capability that indicates the microvm provider
	// has a metadata service.
	MetadataServiceCapability = Capability("metadata-service")
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
