package cloudinit

type Metadata struct {
	InstanceID    string `yaml:"instance_id"`
	LocalHostname string `yaml:"local_hostname"`
	Platform      string `yaml:"platform"`
}
