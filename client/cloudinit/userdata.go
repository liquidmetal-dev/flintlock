package cloudinit

type UserData struct {
	HostName        string      `yaml:"hostname,omitempty"`
	Fqdn            string      `yaml:"fqdn,omitempty"`
	Users           []User      `yaml:"users,omitempty"`
	SSHPasswordAuth *bool       `yaml:"ssh_pwauth,omitempty"`
	DisableRoot     *bool       `yaml:"disable_root,omitempty"`
	PackageUpdate   *bool       `yaml:"package_update,omitempty"`
	FinalMessage    string      `yaml:"final_message,omitempty"`
	WriteFiles      []WriteFile `yaml:"write_files,omitempty"`
	RunCommands     []string    `yaml:"runcmd,omitempty"`
}

type User struct {
	Name              string   `yaml:"name"`
	Sudo              string   `yaml:"sudo,omitempty"`
	Groups            string   `yaml:"groups,omitempty"`
	Home              string   `yaml:"home,omitempty"`
	Shell             string   `yaml:"shell,omitempty"`
	LockPasswd        *bool    `yaml:"lock_passwd,omitempty"`
	SSHAuthorizedKeys []string `yaml:"ssh_authorized_keys,omitempty"`
}

type WriteFile struct {
	Encoding    string `yaml:"encoding"`
	Content     string `yaml:"content"`
	Path        string `yaml:"path"`
	Permissions string `yaml:"permissions"`
}
