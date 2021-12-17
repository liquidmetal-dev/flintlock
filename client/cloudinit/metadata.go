package cloudinit

type Metadata struct {
	InstanceID    string `yaml:"instance_id",json:"instance_id"`
	LocalHostname string `yaml:"local_hostname",json:"local_hostname"`
	Platform      string `yaml:"platform",json:"platform"`
	ClusterName   string `yaml:"cluster_name",json:"cluster_name"`
}
