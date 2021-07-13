package microvm

// Capabaility represents a capability of a provider.
type Capability string

const (
	// MetadataServiceCapability is a caoability that indicates the microvm provider
	// has a metadata service.
	MetadataServiceCapability = Capability("metadata-service")
)

// Capability represents a list of capabilities.
type Capabilities []Capability

// Has is used to test if this set of capabilities has a specific capability.
func (cp Capabilities) Has(cap Capability) bool {
	for _, c := range cp {
		if c == cap {
			return true
		}
	}

	return false
}
