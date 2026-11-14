package network

const (
	DhcpIdentifierMac = "mac"
)

type Network struct {
	Version  int                 `yaml:"version"`
	Ethernet map[string]Ethernet `yaml:"ethernets"`
}

type Ethernet struct {
	Match          Match       `yaml:"match"`
	Addresses      []string    `yaml:"addresses,omitempty"`
	GatewayIPv4    string      `yaml:"gateway4,omitempty"`
	GatewayIPv6    string      `yaml:"gateway6,omitempty"`
	DHCP4          *bool       `yaml:"dhcp4,omitempty"`
	DHCP6          *bool       `yaml:"dhcp6,omitempty"`
	DHCPIdentifier *string     `yaml:"dhcp-identifier,omitempty"`
	Nameservers    Nameservers `yaml:"nameservers,omitempty"`
}

type Match struct {
	MACAddress string `yaml:"macaddress,omitempty"`
	Name       string `yaml:"name,omitempty"`
}

type Nameservers struct {
	Search    []string `yaml:"search,omitempty"`
	Addresses []string `yaml:"addresses,omitempty"`
}
