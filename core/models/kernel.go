package models

// Kernel is the specification of the kernel and its arguments.
type Kernel struct {
	// Image is the container image to use for the kernel.
	Image ContainerImage `json:"image" validate:"required,imageURI"`
	// Filename is the name of the kernel filename in the container.
	Filename string `validate:"required"`
	// CmdLine are the args to use for the kernel cmdline.
	CmdLine map[string]string `json:"cmdline,omitempty"`
	// AddNetworkConfig if set to true indicates that the network-config kernel argument should be generated.
	AddNetworkConfig bool `json:"add_network_config"`
}
